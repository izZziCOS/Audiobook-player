package autoimport

import (
    "fmt"
    "os"
    "sync/atomic"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/ncruces/zenity"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/shelf"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const preferenceWatchedFolder = "libraryFolder"

type Importer struct {
    StartFolder func() string

    window fyne.Window
    prefs  fyne.Preferences
    data   *userdata.Data
    shelf  *shelf.Shelf
    picker *kit.Picker
    async  func(func())

    section       fyne.CanvasObject
    folderLabel   *widget.Label
    folderActions *fyne.Container
    status        *widget.Label

    folder     string
    lastImport time.Time
    importing  atomic.Bool
}

func New(window fyne.Window, prefs fyne.Preferences, data *userdata.Data, books *shelf.Shelf, picker *kit.Picker, async func(func())) *Importer {
    im := &Importer{
        window: window,
        prefs:  prefs,
        data:   data,
        shelf:  books,
        picker: picker,
        async:  async,
    }
    im.build()

    return im
}

func (im *Importer) build() {
    im.folderLabel = widget.NewLabel("")
    im.folderLabel.Truncation = fyne.TextTruncateEllipsis

    folderBackground := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
    folderBackground.CornerRadius = theme.InputRadiusSize()
    kit.FollowTheme(func() {
        folderBackground.FillColor = theme.Color(theme.ColorNameInputBackground)
        folderBackground.CornerRadius = theme.InputRadiusSize()
        folderBackground.Refresh()
    })

    folderRow := container.NewStack(
        folderBackground,
        container.NewBorder(nil, nil, container.NewPadded(widget.NewIcon(theme.FolderIcon())), nil, im.folderLabel),
    )

    chooseButton := widget.NewButtonWithIcon("Choose folder…", theme.FolderOpenIcon(), im.chooseFolder)
    checkButton := widget.NewButtonWithIcon("Check now", theme.ViewRefreshIcon(), im.Scan)
    stopButton := widget.NewButtonWithIcon("Stop importing", theme.ContentClearIcon(), func() {
        im.SetFolder("")
    })
    stopButton.Importance = widget.LowImportance
    im.folderActions = container.NewHBox(checkButton, stopButton)

    im.status = widget.NewLabel("")
    im.status.SizeName = theme.SizeNameCaptionText
    im.status.Importance = widget.LowImportance
    im.status.Wrapping = fyne.TextWrapWord

    im.updateFolder()

    im.section = kit.Section(
        "Library folder",
        "Audiobooks in this folder and its subfolders are added to your library automatically, "+
            "including books you copy into it later. A folder of audio files is one book; "+
            "each .m4b file is a book of its own.",
        folderRow,
        container.NewBorder(nil, nil, chooseButton, nil, im.folderActions),
        im.status,
    )
}

func (im *Importer) Section() fyne.CanvasObject {
    return im.section
}

func (im *Importer) Folder() string {
    return im.folder
}

func exists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}

func (im *Importer) chooseFolder() {
    start := im.folder
    if start == "" && im.StartFolder != nil {
        start = im.StartFolder()
    }
    if start == "" {
        start = kit.HomeFolder()
    }

    kit.Browse(im.picker, im.window,
        func() (string, error) {
            return zenity.SelectFile(
                zenity.Title("Choose your audiobook folder"),
                zenity.Filename(kit.WithTrailingSeparator(start)),
                zenity.Directory(),
            )
        },
        im.SetFolder,
        func() {
            kit.ShowFolderDialog(im.window, start, im.SetFolder)
        },
    )
}

const (
    Interval          = 2 * time.Minute
    autoImportSettle  = 30 * time.Second
    libraryTabRecheck = 30 * time.Second
)

type foundBook struct {
    path     string
    parts    int
    modified time.Time
}

func (im *Importer) SetFolder(folder string) {
    if folder != "" {
        folder = library.Normalize(folder)
    }

    if previous := im.folder; previous != "" && folder != "" && previous != folder {
        im.shelf.MoveFolder(previous, folder)
    }

    im.folder = folder
    im.lastImport = time.Time{}
    im.prefs.SetString(preferenceWatchedFolder, folder)
    im.updateFolder()
    im.Scan()
}

func (im *Importer) Scan() {
    folder := im.folder
    if folder == "" {
        return
    }

    if !exists(folder) {
        for _, candidate := range library.OtherDrives(folder) {
            if exists(candidate) {
                im.SetFolder(candidate)
                return
            }
        }
    }

    if !im.importing.CompareAndSwap(false, true) {
        return
    }

    im.setStatus("Looking for new books…")

    im.async(func() {
        found, err := findImportableBooks(folder)

        fyne.Do(func() {
            im.importing.Store(false)
            im.applyImport(folder, found, err)
        })
    })
}

func (im *Importer) ScanIfStale() {
    if time.Since(im.lastImport) >= libraryTabRecheck {
        im.Scan()
    }
}

func findImportableBooks(folder string) ([]foundBook, error) {
    paths, err := library.FindBooks(folder)
    if err != nil {
        return nil, err
    }

    books := make([]foundBook, 0, len(paths))
    for _, path := range paths {
        parts, modified, err := library.Inspect(path)
        if err != nil {
            continue
        }

        books = append(books, foundBook{path: path, parts: parts, modified: modified})
    }

    return books, nil
}

func (im *Importer) applyImport(folder string, found []foundBook, err error) {
    if folder != im.folder {
        return
    }

    im.lastImport = time.Now()
    checked := im.lastImport.Format("15:04")

    if err != nil {
        im.setStatus(fmt.Sprintf("Couldn't read the folder at %s: %v", checked, err))
        return
    }

    var added []string
    copying := 0
    for _, book := range found {
        if im.data.Ignored[book.path] {
            continue
        }

        if time.Since(book.modified) < autoImportSettle {
            copying++
            continue
        }

        switch {
        case !im.shelf.Contains(book.path):
            added = append(added, book.path)
        case im.shelf.Stale(book.path, book.parts):
            im.shelf.Reload(book.path)
        }
    }

    im.shelf.Import(added)

    status := fmt.Sprintf("Last checked at %s · %s", checked, kit.CountPhrase(len(added), "new book", "new books", "no new books"))
    if copying > 0 {
        status += " · " + kit.CountPhrase(copying, "book is still being copied", "books are still being copied", "")
    }
    im.setStatus(status)
}

func (im *Importer) Load() {
    im.folder = im.prefs.String(preferenceWatchedFolder)
    im.updateFolder()
    im.Scan()
}

func (im *Importer) updateFolder() {
    if im.folder == "" {
        im.folderLabel.SetText("No folder chosen")
        im.folderLabel.Importance = widget.LowImportance
        im.folderActions.Hide()
        im.status.SetText("Books are only added when you add or open them.")
    } else {
        im.folderLabel.SetText(im.folder)
        im.folderLabel.Importance = widget.MediumImportance
        im.folderActions.Show()
    }

    im.folderLabel.Refresh()
}

func (im *Importer) setStatus(text string) {
    im.status.SetText(text)
}
