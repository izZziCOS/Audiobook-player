package userdata

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "slices"
    "strings"
    "time"

    "fyne.io/fyne/v2"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/stats"
    "github.com/izzzicos/audiobook-player/store"
)

const (
    fileName     = "library.json"
    version      = 1
    dailyBackups = 7
)

type Position struct {
    Part     string  `json:"part"`
    Position float64 `json:"position"`
    Listened int64   `json:"listened,omitempty"`
    Finished bool    `json:"finished,omitempty"`
}

type SavedBook struct {
    Path        string               `json:"path"`
    Fingerprint *library.Fingerprint `json:"fingerprint,omitempty"`
}

type LastBook struct {
    Folder string `json:"folder,omitempty"`
    File   string `json:"file,omitempty"`
}

type saved struct {
    Version   int                     `json:"version"`
    Library   []SavedBook             `json:"library"`
    Ignored   []string                `json:"ignored,omitempty"`
    Positions map[string]Position     `json:"positions"`
    Durations map[string]PartDuration `json:"durations"`
    Stats       *stats.Log              `json:"stats"`
    LastBook    LastBook                `json:"lastBook"`
    Collections []Collection            `json:"collections,omitempty"`
}

type Data struct {
    Positions map[string]Position
    Durations map[string]PartDuration
    Stats     *stats.Log
    Ignored   map[string]bool
    LastBook  LastBook
    Library   []SavedBook

    OnSaveError func(error)

    collections    []Collection
    file           string
    dirty          bool
    saveErrorShown bool
}

func FileIn(directory string) string {
    return filepath.Join(directory, fileName)
}

func New(file string) *Data {
    data := &Data{file: file}
    data.apply(saved{})
    return data
}

func (d *Data) File() string {
    return d.file
}

func (d *Data) Dirty() bool {
    return d.dirty
}

func (d *Data) MarkDirty() {
    d.dirty = true
}

func (d *Data) Flush() {
    if d.dirty {
        d.Save()
    }
}

func (d *Data) Save() {
    d.dirty = false
    if d.file == "" {
        return
    }

    if err := store.Save(d.file, d.collect()); err != nil {
        d.dirty = true
        fyne.LogError("Saving the library failed", err)
        if !d.saveErrorShown {
            d.saveErrorShown = true
            if d.OnSaveError != nil {
                d.OnSaveError(err)
            }
        }
        return
    }

    d.saveErrorShown = false
    if err := store.Backup(d.file, time.Now(), dailyBackups); err != nil {
        fyne.LogError("Backing up the library failed", err)
    }
}

func (d *Data) collect() saved {
    data := saved{
        Version:   version,
        Library:   d.Library,
        Positions: d.Positions,
        Durations: d.Durations,
        Stats:       d.Stats,
        LastBook:    d.LastBook,
        Collections: d.collections,
    }

    for path := range d.Ignored {
        data.Ignored = append(data.Ignored, path)
    }
    slices.Sort(data.Ignored)

    return data
}

func (d *Data) Load(prefs fyne.Preferences) string {
    var data saved
    recovered, err := store.Load(d.file, &data)

    notice := ""
    switch {
    case err == nil && recovered != "":
        notice = fmt.Sprintf(
            "Your library file was damaged, so it was restored from the backup made on %s. Anything that changed after that backup was lost.",
            backupDate(recovered),
        )
    case errors.Is(err, os.ErrNotExist):
        data = migratePreferences(prefs)
    case err != nil:
        moved, moveErr := store.SetAside(d.file, time.Now())
        notice = "Your library file couldn't be read and there was no backup to restore, so the app started with an empty library."
        if moveErr == nil {
            notice += " The damaged file was kept as " + moved + "."
        }
        data = saved{}
    }

    d.apply(normalize(data))
    if d.Stats.Compact(time.Now(), stats.KeepDays) {
        d.MarkDirty()
    }
    if errors.Is(err, os.ErrNotExist) {
        d.Save()
        if !d.dirty {
            removeMigratedPreferences(prefs)
        }
    }

    return notice
}

func backupDate(path string) string {
    name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
    date, err := time.Parse("2006-01-02", name[max(len(name)-len("2006-01-02"), 0):])
    if err != nil {
        return filepath.Base(path)
    }

    return date.Format("2 January 2006")
}

func (d *Data) apply(data saved) {
    d.Positions = data.Positions
    if d.Positions == nil {
        d.Positions = map[string]Position{}
    }

    d.Durations = data.Durations
    if d.Durations == nil {
        d.Durations = map[string]PartDuration{}
    }

    d.Stats = data.Stats
    if d.Stats == nil {
        d.Stats = stats.New()
    }

    d.Ignored = map[string]bool{}
    for _, path := range data.Ignored {
        d.Ignored[path] = true
    }

    d.LastBook = data.LastBook
    d.Library = data.Library
    d.collections = data.Collections
}

func normalize(data saved) saved {
    positions := map[string]Position{}
    for key, position := range data.Positions {
        position.Part = library.Normalize(position.Part)
        positions[library.Normalize(key)] = position
    }
    data.Positions = positions

    durations := map[string]PartDuration{}
    for key, duration := range data.Durations {
        durations[library.Normalize(key)] = duration
    }
    data.Durations = durations

    if data.Stats != nil {
        var keys []string
        for key := range data.Stats.Books {
            keys = append(keys, key)
        }
        for key := range data.Stats.Completed {
            keys = append(keys, key)
        }
        for _, key := range keys {
            if normalized := library.Normalize(key); normalized != key {
                data.Stats.Rekey(key, normalized)
            }
        }
    }

    for i := range data.Library {
        data.Library[i].Path = library.Normalize(data.Library[i].Path)
    }
    for i := range data.Ignored {
        data.Ignored[i] = library.Normalize(data.Ignored[i])
    }

    for i := range data.Collections {
        for j := range data.Collections[i].Books {
            data.Collections[i].Books[j] = library.Normalize(data.Collections[i].Books[j])
        }
    }

    if data.LastBook.Folder != "" {
        data.LastBook.Folder = library.Normalize(data.LastBook.Folder)
    }
    if data.LastBook.File != "" {
        data.LastBook.File = library.Normalize(data.LastBook.File)
    }

    return data
}

func (d *Data) Remember(book *library.Book, position float64) {
    key := book.Key()
    d.Positions[key] = Position{
        Part:     book.Current(),
        Position: position,
        Listened: time.Now().Unix(),
        Finished: d.Positions[key].Finished && book.Index == 0 && position < 1,
    }

    d.LastBook = LastBook{Folder: book.Folder, File: book.Current()}
    d.MarkDirty()
}

func (d *Data) Finish(key string) {
    position := d.Positions[key]
    position.Finished = true
    d.Positions[key] = position
    d.Stats.Complete(time.Now(), key)
    d.Save()
}

func (d *Data) Ignore(path string) {
    d.Ignored[path] = true
}

func (d *Data) Unignore(paths []string) {
    changed := false
    for _, path := range paths {
        if d.Ignored[path] {
            delete(d.Ignored, path)
            changed = true
        }
    }

    if changed {
        d.MarkDirty()
    }
}
