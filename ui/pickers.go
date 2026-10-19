package ui

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/storage"

    "github.com/ncruces/zenity"

    "github.com/izzzicos/audiobook-player/audio"
)

func (ui *UI) chooseFile() {
    if !ui.browsing.CompareAndSwap(false, true) {
        return
    }

    go func() {
        defer ui.browsing.Store(false)

        path, err := zenity.SelectFile(
            zenity.Title("Open audiobook"),
            zenity.Filename(withTrailingSeparator(ui.browseFolder())),
            zenity.FileFilters{
                {Name: "Audiobooks (MP3, WAV, FLAC, Ogg Vorbis)", Patterns: audioPatterns(), CaseFold: true},
            },
        )
        ui.Window.RequestFocus()

        switch {
        case errors.Is(err, zenity.ErrCanceled):
        case err != nil:
            ui.showFileDialog()
        default:
            ui.openFile(path)
        }
    }()
}

func (ui *UI) chooseFolder() {
    if !ui.browsing.CompareAndSwap(false, true) {
        return
    }

    go func() {
        defer ui.browsing.Store(false)

        path, err := zenity.SelectFile(
            zenity.Title("Open audiobook folder"),
            zenity.Filename(withTrailingSeparator(ui.libraryFolder())),
            zenity.Directory(),
        )
        ui.Window.RequestFocus()

        switch {
        case errors.Is(err, zenity.ErrCanceled):
        case err != nil:
            ui.showFolderDialog()
        default:
            ui.openFolder(path)
        }
    }()
}

func (ui *UI) browseFolder() string {
    if ui.Book != nil {
        return ui.Book.BrowseFolder()
    }

    return homeFolder()
}

func (ui *UI) libraryFolder() string {
    if ui.Book != nil {
        return ui.Book.LibraryFolder()
    }

    return homeFolder()
}

func homeFolder() string {
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }

    return home
}

func withTrailingSeparator(folder string) string {
    if folder == "" || strings.HasSuffix(folder, string(filepath.Separator)) {
        return folder
    }

    return folder + string(filepath.Separator)
}

func audioPatterns() []string {
    patterns := make([]string, len(audio.SupportedExtensions))
    for i, extension := range audio.SupportedExtensions {
        patterns[i] = "*" + extension
    }

    return patterns
}

func audioExtensionsAnyCase() []string {
    var extensions []string
    for _, extension := range audio.SupportedExtensions {
        extensions = append(extensions, extension, strings.ToUpper(extension))
    }

    return extensions
}

func (ui *UI) showFileDialog() {
    fileDialog := dialog.NewFileOpen(
        func(reader fyne.URIReadCloser, err error) {
            if err != nil {
                dialog.ShowError(err, ui.Window)
                return
            }

            if reader == nil {
                return
            }
            defer reader.Close()

            ui.openFile(reader.URI().Path())
        },
        ui.Window,
    )

    fileDialog.SetFilter(storage.NewExtensionFileFilter(audioExtensionsAnyCase()))
    ui.showFallbackDialog(fileDialog, ui.browseFolder())
}

func (ui *UI) showFolderDialog() {
    folderDialog := dialog.NewFolderOpen(
        func(lister fyne.ListableURI, err error) {
            if err != nil {
                dialog.ShowError(err, ui.Window)
                return
            }

            if lister == nil {
                return
            }

            ui.openFolder(lister.Path())
        },
        ui.Window,
    )

    ui.showFallbackDialog(folderDialog, ui.libraryFolder())
}

func (ui *UI) showFallbackDialog(fileDialog *dialog.FileDialog, folder string) {
    if location, err := storage.ListerForURI(storage.NewFileURI(folder)); err == nil {
        fileDialog.SetLocation(location)
    }

    fileDialog.Resize(ui.Window.Canvas().Size().Subtract(fyne.NewSquareSize(40)))
    fileDialog.Show()
}

func (ui *UI) handleDrop(_ fyne.Position, uris []fyne.URI) {
    for _, uri := range uris {
        if info, err := os.Stat(uri.Path()); err == nil && info.IsDir() {
            ui.openFolder(uri.Path())
            return
        }

        if audio.IsSupported(uri.Name()) {
            ui.openFile(uri.Path())
            return
        }
    }

    dialog.ShowError(
        fmt.Errorf("only MP3, WAV, FLAC and Ogg Vorbis files, or folders of them, can be opened"),
        ui.Window,
    )
}
