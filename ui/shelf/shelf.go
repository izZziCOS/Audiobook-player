package shelf

import (
    "errors"
    "fmt"
    "path/filepath"
    "sync/atomic"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/widget"

    "github.com/ncruces/zenity"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/about"
    "github.com/izzzicos/audiobook-player/ui/details"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/offline"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const preferenceLibrarySort = "librarySort"

type Shelf struct {
    Open         func(path string) error
    Show         func()
    Speed        func() float64
    SyncProgress func()
    StartFolder  func() string
    EditDetails  func(path string, original details.Original)
    ShowAbout    func(path string, book about.Book)
    Offline      *offline.Store

    window fyne.Window
    prefs  fyne.Preferences
    data   *userdata.Data
    picker *kit.Picker
    async  func(func())
    cache  *shelfCache

    content       fyne.CanvasObject
    grid          *fyne.Container
    empty         fyne.CanvasObject
    search        *widget.Entry
    noMatches     *widget.Label
    count         *widget.Label
    header        *fyne.Container
    filterButtons    map[string]*widget.Button
    sortButton       *widget.Button
    collectionButton *widget.Button
    duplicatesButton *widget.Button

    entries        []*shelfEntry
    rechecking     atomic.Bool
    order          string
    filter         string
    collection     string
    arrangePending bool
}

type Card struct {
    Path     string
    Title    string
    Author   string
    Status   string
    Rating   string
    Progress float64
}

func New(window fyne.Window, prefs fyne.Preferences, data *userdata.Data, picker *kit.Picker, async func(func())) *Shelf {
    s := &Shelf{
        window: window,
        prefs:  prefs,
        data:   data,
        picker: picker,
        async:  async,
    }
    if file := data.File(); file != "" {
        s.cache = newShelfCache(filepath.Dir(file))
    }
    s.content = s.build()

    return s
}

func (s *Shelf) Content() fyne.CanvasObject {
    return s.content
}

func (s *Shelf) Load() {
    for _, saved := range s.data.Library {
        if s.find(saved.Path) != nil {
            continue
        }

        entry := s.newShelfEntry(saved.Path)
        entry.fingerprint = saved.Fingerprint
        s.entries = append(s.entries, entry)
    }
    s.publish()

    s.Refresh()
    s.loadCache()
    s.loadShelfInfo(s.entries)
}

func (s *Shelf) loadCache() {
    s.cache.load()

    books := map[string]bool{}
    covers := map[string]bool{}
    for _, entry := range s.entries {
        books[entry.path] = true
        if file := s.data.CoverFile(entry.path); file != "" {
            covers[filepath.Base(file)] = true
        }
    }

    cache := s.cache
    s.async(func() {
        cache.prune(books, covers)
    })
}

func (s *Shelf) publish() {
    books := make([]userdata.SavedBook, len(s.entries))
    for i, entry := range s.entries {
        books[i] = userdata.SavedBook{Path: entry.path, Fingerprint: entry.fingerprint}
    }

    s.data.Library = books
}

func (s *Shelf) save() {
    s.publish()
    s.data.Save()
}

func (s *Shelf) markDirty() {
    s.publish()
    s.data.MarkDirty()
}

func (s *Shelf) show() {
    if s.Show != nil {
        s.Show()
    }
}

func (s *Shelf) speed() float64 {
    if s.Speed == nil {
        return 1
    }

    return s.Speed()
}

func (s *Shelf) Cards() []Card {
    cards := make([]Card, len(s.entries))
    for i, entry := range s.entries {
        cards[i] = entry.view()
    }

    return cards
}

func (s *Shelf) Card(path string) (Card, bool) {
    if entry := s.find(path); entry != nil {
        return entry.view(), true
    }

    return Card{}, false
}

func (entry *shelfEntry) view() Card {
    return Card{
        Path:     entry.path,
        Title:    entry.title,
        Author:   entry.card.Author,
        Status:   entry.card.Status,
        Rating:   entry.card.Rating,
        Progress: entry.card.Progress,
    }
}

func (s *Shelf) Title(path string) (string, bool) {
    if entry := s.find(path); entry != nil {
        return entry.title, true
    }

    return "", false
}

func (s *Shelf) Contains(path string) bool {
    return s.find(path) != nil
}

func (s *Shelf) Stale(path string, parts int) bool {
    entry := s.find(path)
    return entry != nil && (entry.missing || entry.book != nil && len(entry.book.Parts) != parts)
}

func (s *Shelf) Reload(path string) {
    if entry := s.find(path); entry != nil {
        s.loadShelfInfo([]*shelfEntry{entry})
    }
}

func (s *Shelf) Remove(path string) {
    if entry := s.find(path); entry != nil {
        s.remove(entry)
    }
}

func (s *Shelf) FocusSearch() {
    if s.header.Visible() {
        s.window.Canvas().Focus(s.search)
    }
}

func (s *Shelf) open(entry *shelfEntry) {
    if s.Open == nil {
        return
    }

    path := entry.path
    if s.Offline.State(path) == offline.Downloaded {
        if err := s.Open(path); err != nil {
            dialog.ShowError(openError(entry.title, err, false), s.window)
        }
        return
    }

    s.async(func() {
        err := reachable(path, library.ResponseTimeout)
        unavailable := errors.Is(err, library.ErrNotResponding) || err != nil && library.Unavailable(path)

        fyne.Do(func() {
            if !errors.Is(err, library.ErrNotResponding) {
                if err = s.Open(path); err == nil {
                    return
                }
            }

            entry.missing = true
            entry.unavailable = unavailable
            s.updateCard(entry)
            dialog.ShowError(openError(entry.title, err, unavailable), s.window)
        })
    })
}

func openError(title string, err error, unavailable bool) error {
    switch {
    case errors.Is(err, library.ErrNotResponding):
        return fmt.Errorf("%q is on a drive or server that isn't responding. Check the connection, for example your VPN, then try again", title)
    case unavailable:
        return fmt.Errorf("%q is on a drive or server that isn't connected. Connect it, then try again. Your place in the book is kept", title)
    default:
        return fmt.Errorf("%q can't be found. It may have been moved or renamed", title)
    }
}

func (s *Shelf) RecheckMissing() {
    var missing []*shelfEntry
    for _, entry := range s.entries {
        if entry.missing {
            missing = append(missing, entry)
        }
    }
    if len(missing) == 0 || !s.rechecking.CompareAndSwap(false, true) {
        return
    }

    paths := make([]string, len(missing))
    for i, entry := range missing {
        paths[i] = entry.path
    }

    s.async(func() {
        var back []string
        for _, path := range paths {
            if reachable(path, library.ResponseTimeout) == nil {
                back = append(back, path)
            }
        }

        fyne.Do(func() {
            s.rechecking.Store(false)
            var found []*shelfEntry
            for _, path := range back {
                if entry := s.find(path); entry != nil {
                    found = append(found, entry)
                }
            }
            s.loadShelfInfo(found)
        })
    })
}

func (s *Shelf) editDetails(entry *shelfEntry) {
    if s.EditDetails == nil {
        return
    }

    original := entry.original
    if original.Title == "" {
        original.Title = library.CleanName(filepath.Base(entry.path))
    }
    s.EditDetails(entry.path, original)
}

func (s *Shelf) showAbout(entry *shelfEntry) {
    if s.ShowAbout != nil {
        s.ShowAbout(entry.path, about.Book{Title: entry.title, Author: entry.author, Cover: entry.card.Thumbnail})
    }
}

func (s *Shelf) relink(entry *shelfEntry, newPath string) {
    if entry.path == newPath {
        return
    }

    s.data.Move(entry.path, newPath)
    entry.path = newPath
    entry.missing = false
}

func (s *Shelf) MoveFolder(from, to string) {
    var relinked []*shelfEntry
    for _, entry := range s.entries {
        if exists(entry.path) {
            continue
        }

        relative, inside := library.Within(entry.path, from)
        if !inside {
            continue
        }

        if candidate := filepath.Join(to, relative); exists(candidate) {
            s.relink(entry, candidate)
            relinked = append(relinked, entry)
        }
    }

    s.data.MoveIgnored(from, to)

    if len(relinked) > 0 {
        s.save()
        s.loadShelfInfo(relinked)
    }
}

func (s *Shelf) startFolder() string {
    if s.StartFolder != nil {
        return s.StartFolder()
    }

    return kit.HomeFolder()
}

func (s *Shelf) chooseFolders() {
    kit.Browse(s.picker, s.window,
        func() ([]string, error) {
            return zenity.SelectFileMultiple(
                zenity.Title("Add audiobooks to the library"),
                zenity.Filename(kit.WithTrailingSeparator(s.startFolder())),
                zenity.Directory(),
            )
        },
        s.AddFrom,
        func() {
            kit.ShowFolderDialog(s.window, s.startFolder(), func(path string) {
                s.AddFrom([]string{path})
            })
        },
    )
}
