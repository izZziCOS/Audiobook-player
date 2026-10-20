package shelf

import (
    "fmt"
    "path/filepath"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/widget"

    "github.com/ncruces/zenity"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const preferenceLibrarySort = "librarySort"

type Shelf struct {
    Open         func(path string) error
    Show         func()
    Speed        func() float64
    SyncProgress func()
    StartFolder  func() string

    window fyne.Window
    prefs  fyne.Preferences
    data   *userdata.Data
    picker *kit.Picker
    async  func(func())

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

    entries        []*shelfEntry
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
    s.loadShelfInfo(s.entries)
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

    if err := s.Open(entry.path); err != nil {
        entry.missing = true
        s.updateCard(entry)
        dialog.ShowError(
            fmt.Errorf("%q can't be found. It may have been moved or renamed, or be on a drive that isn't connected", entry.title),
            s.window,
        )
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
