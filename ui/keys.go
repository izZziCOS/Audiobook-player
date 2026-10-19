package ui

import "fyne.io/fyne/v2"

func (ui *UI) handleKey(event *fyne.KeyEvent) {
    switch event.Name {
    case fyne.KeySpace:
        ui.togglePlay()
    case fyne.KeyLeft:
        ui.skip(-30)
    case fyne.KeyRight:
        ui.skip(30)
    case fyne.KeyUp:
        ui.changeVolume(0.05)
    case fyne.KeyDown:
        ui.changeVolume(-0.05)
    }
}

func (ui *UI) releaseFocus() {
    ui.Window.Canvas().Unfocus()
}
