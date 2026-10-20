package nowplaying

import (
    "strings"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/storage"

    "github.com/ncruces/zenity"

    "github.com/izzzicos/audiobook-player/audio"
    "github.com/izzzicos/audiobook-player/ui/kit"
)

func (p *Player) chooseFile() {
    kit.Browse(p.picker, p.window,
        func() (string, error) {
            return zenity.SelectFile(
                zenity.Title("Open audiobook"),
                zenity.Filename(kit.WithTrailingSeparator(p.browseFolder())),
                zenity.FileFilters{
                    {Name: "Audiobooks (MP3, M4B, M4A, WAV, FLAC, Ogg Vorbis)", Patterns: audioPatterns(), CaseFold: true},
                },
            )
        },
        p.OpenFile,
        p.showFileDialog,
    )
}

func (p *Player) chooseFolder() {
    kit.Browse(p.picker, p.window,
        func() (string, error) {
            return zenity.SelectFile(
                zenity.Title("Open audiobook folder"),
                zenity.Filename(kit.WithTrailingSeparator(p.LibraryFolder())),
                zenity.Directory(),
            )
        },
        p.OpenFolder,
        func() {
            kit.ShowFolderDialog(p.window, p.LibraryFolder(), p.OpenFolder)
        },
    )
}

func (p *Player) browseFolder() string {
    if p.book != nil {
        return p.book.BrowseFolder()
    }

    return kit.HomeFolder()
}

func (p *Player) LibraryFolder() string {
    if p.book != nil {
        return p.book.LibraryFolder()
    }

    return kit.HomeFolder()
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

func (p *Player) showFileDialog() {
    fileDialog := dialog.NewFileOpen(
        func(reader fyne.URIReadCloser, err error) {
            if err != nil {
                dialog.ShowError(err, p.window)
                return
            }

            if reader == nil {
                return
            }
            defer reader.Close()

            p.OpenFile(reader.URI().Path())
        },
        p.window,
    )

    fileDialog.SetFilter(storage.NewExtensionFileFilter(audioExtensionsAnyCase()))
    kit.ShowFallbackDialog(p.window, fileDialog, p.browseFolder())
}
