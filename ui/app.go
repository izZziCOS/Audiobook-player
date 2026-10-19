package ui

import (
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/audio"
	"github.com/izzzicos/audiobook-player/library"
)

const AppID = "com.izzzicos.audiobookplayer"

type UI struct {
    App    fyne.App
    Window fyne.Window
    Player *audio.Player
    Book   *library.Book

    Title            *widget.RichText
    Subtitle         *widget.RichText
    Progress         *widget.Slider
    Current          *widget.RichText
    Remaining        *widget.RichText
    Cover            *canvas.Image
    CoverPlaceholder fyne.CanvasObject
    Volume           *widget.Slider
    Speed            *widget.Select
    PlayButton       *roundButton
    PreviousButton   *roundButton
    NextButton       *roundButton
    PartsButton      *widget.Button
    Playing          bool

    speed float64

    seeking  atomic.Bool
    browsing atomic.Bool
}

func NewUI(icon fyne.Resource) *UI {
    a := app.NewWithID(AppID)
    if icon != nil {
        a.SetIcon(icon)
    }

    w := a.NewWindow("Audiobook Player")
    w.Resize(fyne.NewSize(600, 720))

    return &UI{
        App:    a,
        Window: w,
        speed:  1.0,
    }
}

func (ui *UI) Run() {
    ui.setup()
    ui.Window.ShowAndRun()
}

func (ui *UI) setup() {
    ui.Window.SetContent(ui.buildContent())
    ui.App.Lifecycle().SetOnStopped(ui.savePreferences)
    ui.Window.Canvas().SetOnTypedKey(ui.handleKey)
    ui.Window.SetOnDropped(ui.handleDrop)

    if err := audio.InitSpeaker(); err != nil {
        dialog.ShowError(err, ui.Window)
    }

    ui.loadPreferences()

    go ui.updateProgress()
}
