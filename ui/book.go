package ui

import (
    "fmt"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/library"
)

func (ui *UI) openFile(path string) {
    ui.openBook(library.SingleFile(path))
}

func (ui *UI) openFolder(folder string) {
    book, err := library.OpenFolder(folder)
    if err != nil {
        dialog.ShowError(err, ui.Window)
        return
    }

    ui.openBook(book)
}

func (ui *UI) resumeBook(folder, path string) bool {
    if folder != "" {
        if book, err := library.OpenFolder(folder); err == nil {
            if index, found := book.Find(path); found {
                book.Index = index
                return ui.openBook(book)
            }
        }
    }

    return ui.openBook(library.SingleFile(path))
}

func (ui *UI) openBook(book *library.Book) bool {
    if !ui.loadAudio(book.Current()) {
        return false
    }

    ui.Book = book
    ui.updateBookInfo()

    return true
}

func (ui *UI) goToRelativePart(offset int) {
    if ui.Book == nil {
        return
    }

    ui.goToPart(ui.Book.Index+offset, ui.Playing)
}

func (ui *UI) goToPart(index int, play bool) {
    if ui.Book == nil || !ui.Book.HasPart(index) {
        return
    }

    if !ui.loadAudio(ui.Book.Parts[index]) {
        return
    }

    ui.Book.Index = index
    ui.updateBookInfo()

    if play {
        ui.Player.Play()
        ui.setPlaying(true)
    }
}

func (ui *UI) handlePartEnd() {
    if ui.Book != nil && ui.Book.HasPart(ui.Book.Index+1) {
        ui.goToPart(ui.Book.Index+1, true)
        return
    }

    ui.setPlaying(false)
}

func (ui *UI) updateBookInfo() {
    setRichText(ui.Title, ui.Book.Title())
    ui.setSubtitle(ui.Book.Subtitle())

    for _, object := range []fyne.CanvasObject{ui.PreviousButton, ui.NextButton, ui.PartsButton} {
        if ui.Book.IsMultiPart() {
            object.Show()
        } else {
            object.Hide()
        }
    }

    ui.refreshLayout()
}

func (ui *UI) setSubtitle(text string) {
    setRichText(ui.Subtitle, text)

    if text == "" {
        ui.Subtitle.Hide()
    } else {
        ui.Subtitle.Show()
    }

    ui.refreshLayout()
}

func (ui *UI) updateCover(audioPath string) {
    path, err := library.FindCover(audioPath)
    if err != nil {
        path = ""
    }

    ui.Cover.File = path
    ui.Cover.Refresh()

    if path == "" {
        ui.Cover.Hide()
        ui.CoverPlaceholder.Show()
    } else {
        ui.CoverPlaceholder.Hide()
        ui.Cover.Show()
    }
}

func (ui *UI) showParts() {
    if ui.Book == nil {
        return
    }

    var partsDialog dialog.Dialog

    list := widget.NewList(
        func() int {
            return len(ui.Book.Parts)
        },
        func() fyne.CanvasObject {
            label := widget.NewLabel("")
            label.Truncation = fyne.TextTruncateEllipsis
            return label
        },
        func(id widget.ListItemID, object fyne.CanvasObject) {
            label := object.(*widget.Label)
            label.TextStyle.Bold = id == ui.Book.Index
            label.SetText(fmt.Sprintf("%d. %s", id+1, library.DisplayTitle(ui.Book.Parts[id])))
        },
    )
    list.OnSelected = func(id widget.ListItemID) {
        partsDialog.Hide()
        ui.goToPart(id, ui.Playing)
    }

    partsDialog = dialog.NewCustom("Parts", "Close", list, ui.Window)

    size := ui.Window.Canvas().Size()
    partsDialog.Resize(fyne.NewSize(min(size.Width-40, 460), size.Height*0.8))
    partsDialog.Show()
    list.ScrollTo(ui.Book.Index)
}
