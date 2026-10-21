package userdata

import (
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "fyne.io/fyne/v2"

    "github.com/izzzicos/audiobook-player/library"
)

const coversFolder = "covers"

type BookEdit struct {
    Title  string `json:"title,omitempty"`
    Author string `json:"author,omitempty"`
    Cover  string `json:"cover,omitempty"`
    ASIN      string `json:"asin,omitempty"`
    Store     string `json:"store,omitempty"`
    NoAudible bool   `json:"noAudible,omitempty"`
}

func (d *Data) Edit(book string) BookEdit {
    return d.edits[book]
}

func (d *Data) SetEdit(book string, edit BookEdit) {
    edit.Title = library.CleanText(edit.Title)
    edit.Author = library.CleanText(edit.Author)

    previous := d.edits[book]
    if edit == (BookEdit{}) {
        delete(d.edits, book)
    } else {
        d.edits[book] = edit
    }

    if previous.Cover != edit.Cover {
        d.releaseCover(previous.Cover)
    }
    d.Save()
}

func (d *Data) coversFolder() string {
    if d.file == "" {
        return ""
    }

    return filepath.Join(filepath.Dir(d.file), coversFolder)
}

func (d *Data) CoverFile(book string) string {
    name := d.edits[book].Cover
    if name == "" || d.file == "" {
        return ""
    }

    return filepath.Join(d.coversFolder(), name)
}

func ReadCustomCover(file string) []byte {
    if file == "" {
        return nil
    }

    data, err := os.ReadFile(file)
    if err != nil {
        fyne.LogError("Reading a custom cover failed", err)
        return nil
    }

    return data
}

func (d *Data) SaveCover(data []byte) (string, error) {
    folder := d.coversFolder()
    if folder == "" {
        return "", errors.New("covers can only be saved once the library has a file")
    }

    sum := sha256.Sum256(data)
    name := hex.EncodeToString(sum[:12]) + ".jpg"
    file := filepath.Join(folder, name)
    if _, err := os.Stat(file); err == nil {
        return name, nil
    }

    if err := os.MkdirAll(folder, 0o755); err != nil {
        return "", fmt.Errorf("making the covers folder: %w", err)
    }

    temporary, err := os.CreateTemp(folder, name+".*.tmp")
    if err != nil {
        return "", fmt.Errorf("saving the cover: %w", err)
    }
    _, writeErr := temporary.Write(data)
    closeErr := temporary.Close()
    if err := errors.Join(writeErr, closeErr); err != nil {
        os.Remove(temporary.Name())
        return "", fmt.Errorf("saving the cover: %w", err)
    }
    if err := os.Rename(temporary.Name(), file); err != nil {
        os.Remove(temporary.Name())
        return "", fmt.Errorf("saving the cover: %w", err)
    }

    return name, nil
}

func (d *Data) coverInUse(name string) bool {
    for _, edit := range d.edits {
        if edit.Cover == name {
            return true
        }
    }

    return false
}

func (d *Data) releaseCover(name string) {
    if name == "" || d.file == "" || d.coverInUse(name) {
        return
    }

    if err := os.Remove(filepath.Join(d.coversFolder(), name)); err != nil && !errors.Is(err, os.ErrNotExist) {
        fyne.LogError("Removing an unused cover failed", err)
    }
}

func (d *Data) moveEdit(old, newPath string) {
    edit, found := d.edits[old]
    if !found {
        return
    }

    delete(d.edits, old)
    if _, taken := d.edits[newPath]; taken {
        d.releaseCover(edit.Cover)
        return
    }

    d.edits[newPath] = edit
}

func (d *Data) forgetEdit(book string) bool {
    edit, found := d.edits[book]
    if !found {
        return false
    }

    delete(d.edits, book)
    d.releaseCover(edit.Cover)
    return true
}

func unusedCovers(folder string, used map[string]bool) []string {
    if folder == "" {
        return nil
    }

    entries, err := os.ReadDir(folder)
    if err != nil {
        return nil
    }

    var unused []string
    for _, entry := range entries {
        name := entry.Name()
        if !entry.IsDir() && !used[name] && !strings.HasPrefix(name, ".") {
            unused = append(unused, name)
        }
    }

    return unused
}
