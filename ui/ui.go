package ui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"

	"github.com/izzzicos/audiobook-player/audio"
	"github.com/izzzicos/audiobook-player/helper"
)

type UI struct {
    App            fyne.App
    Window         fyne.Window
    AudioPanel     *audio.AudioPanel
    CurrentFileURI fyne.URI

    Label      *widget.Label
    Progress   *widget.Slider
    Current    *widget.Label
    Total      *widget.Label
    Cover      *canvas.Image
    Volume     *widget.Slider
    Speed      *widget.Select
    Playing    bool
}

func NewUI() *UI {
    a := app.NewWithID("com.example.audiobookplayer")
    w := a.NewWindow("Audiobook Player")
    w.Resize(fyne.NewSize(600, 720))

    return &UI{
        App:    a,
        Window: w,
    }
}

func (ui *UI) SetupUI() {
    ui.Label = widget.NewLabel("Select an audiobook")
    ui.Current = widget.NewLabel("00:00:00")
    ui.Total = widget.NewLabel("00:00:00")

    ui.Progress = widget.NewSlider(0, 1)
    ui.Progress.Step = 0.001

    ui.Volume = widget.NewSlider(0, 1)
    ui.Volume.SetValue(0.7)

    ui.Speed = widget.NewSelect(
        []string{
            "0.5x",
            "0.75x",
            "1.0x",
            "1.25x",
            "1.5x",
            "1.75x",
            "2.0x",
        },
        ui.changeSpeed,
    )
    ui.Speed.SetSelected("1.0x")

    ui.Cover = canvas.NewImageFromFile("")
    ui.Cover.FillMode = canvas.ImageFillContain
    ui.Cover.SetMinSize(fyne.NewSize(350, 350))

    openButton := widget.NewButtonWithIcon(
        "Open audiobook",
        theme.FolderOpenIcon(),
        ui.openMP3FileDialog,
    )

    playButton := widget.NewButtonWithIcon(
        "",
        theme.MediaPlayIcon(),
        ui.play,
    )
    pauseButton := widget.NewButtonWithIcon(
        "",
        theme.MediaPauseIcon(),
        ui.pause,
    )

    backButton := widget.NewButtonWithIcon(
        "30 sec",
        theme.MediaFastRewindIcon(),
        func() {
            ui.skip(-30)
        },
    )

    forwardButton := widget.NewButtonWithIcon(
        "30 sec",
        theme.MediaFastForwardIcon(),
        func() {
            ui.skip(30)
        },
    )

    volumeLabel := widget.NewLabel("Volume")
    speedLabel := widget.NewLabel("Playback speed")

    ui.Progress.OnChanged = func(value float64) {
        if ui.AudioPanel == nil {
            return
        }

        seconds := value * ui.durationSeconds()
        _ = ui.AudioPanel.SeekTo(seconds)
    }

    ui.Volume.OnChanged = func(value float64) {
        if ui.AudioPanel != nil {
            ui.AudioPanel.SetVolume(value)
        }
    }

    controls := container.New(
        layout.NewGridLayout(4),
        backButton,
        playButton,
        pauseButton,
        forwardButton,
    )

    timeLabels := container.NewBorder(
        nil,
        nil,
        ui.Current,
        ui.Total,
    )

    content := container.NewVBox(
        openButton,
        container.NewCenter(ui.Label),
        container.NewCenter(ui.Cover),
        ui.Progress,
        timeLabels,
        controls,
        volumeLabel,
        ui.Volume,
        speedLabel,
        ui.Speed,
    )

    ui.Window.SetContent(content)
    ui.Window.SetOnClosed(ui.savePreferences)

    ui.loadPreferences()

    go ui.updateProgress()

    ui.Window.ShowAndRun()
}

func (ui *UI) openMP3FileDialog() {
    filter := storage.NewExtensionFileFilter(
        []string{".mp3"},
    )

    home, err := os.UserHomeDir()
    if err != nil {
        dialog.ShowError(err, ui.Window)
        return
    }

    location, err := storage.ListerForURI(
        storage.NewFileURI(home),
    )
    if err != nil {
        dialog.ShowError(err, ui.Window)
        return
    }

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

            ui.openFile(reader.URI())
        },
        ui.Window,
    )

    fileDialog.SetFilter(filter)
    fileDialog.SetLocation(location)
    fileDialog.Show()
}

func (ui *UI) openFile(uri fyne.URI) {
    file, err := os.Open(uri.Path())
    if err != nil {
        dialog.ShowError(err, ui.Window)
        return
    }

    streamer, format, err := mp3.Decode(file)
    if err != nil {
        file.Close()
        dialog.ShowError(err, ui.Window)
        return
    }

    if streamer.Len() <= 0 {
        streamer.Close()
        file.Close()
        dialog.ShowError(
            fmt.Errorf("the selected audio file is empty"),
            ui.Window,
        )
        return
    }

    if ui.AudioPanel != nil {
        _ = ui.AudioPanel.Close()
    }

    speaker.Init(
        format.SampleRate,
        format.SampleRate.N(time.Second/10),
    )

    ui.AudioPanel = audio.NewAudioPanel(
        format.SampleRate,
        streamer,
    )

    ui.CurrentFileURI = uri
    ui.Label.SetText(uri.Name())

    ui.updateCover(uri)

    ui.Progress.SetValue(0)
    ui.Total.SetText(
        helper.FormatTime(
            float64(streamer.Len()) /
                float64(format.SampleRate),
        ),
    )

    ui.Playing = false
    ui.AudioPanel.SetVolume(ui.Volume.Value)
}

func (ui *UI) play() {
    if ui.AudioPanel == nil {
        return
    }

    if ui.Playing {
        return
    }

    ui.AudioPanel.Play()
    ui.Playing = true
}

func (ui *UI) pause() {
    if ui.AudioPanel == nil {
        return
    }

    ui.AudioPanel.Pause()
    ui.Playing = false
}

func (ui *UI) skip(seconds int) {
    if ui.AudioPanel == nil {
        return
    }

    if err := ui.AudioPanel.Skip(seconds); err != nil {
        dialog.ShowError(err, ui.Window)
    }
}

func (ui *UI) changeSpeed(value string) {
    if ui.AudioPanel == nil {
        return
    }

    var speed float64

    switch value {
    case "0.5x":
        speed = 0.5
    case "0.75x":
        speed = 0.75
    case "1.0x":
        speed = 1.0
    case "1.25x":
        speed = 1.25
    case "1.5x":
        speed = 1.5
    case "1.75x":
        speed = 1.75
    case "2.0x":
        speed = 2.0
    default:
        speed = 1.0
    }

    ui.AudioPanel.SetSpeed(speed)
}

func (ui *UI) durationSeconds() float64 {
    if ui.AudioPanel == nil {
        return 0
    }

    length := ui.AudioPanel.Length()
    if length <= 0 {
        return 0
    }

    return float64(length) /
        float64(ui.AudioPanel.SampleRate)
}

func (ui *UI) updateProgress() {
    ticker := time.NewTicker(250 * time.Millisecond)
    defer ticker.Stop()

    for range ticker.C {
        if ui.AudioPanel == nil {
            continue
        }

        position := ui.AudioPanel.Position()
        length := ui.AudioPanel.Length()

        if length <= 0 {
            continue
        }

        progress := float64(position) / float64(length)
        if progress < 0 {
            progress = 0
        }
        if progress > 1 {
            progress = 1
        }

        currentSeconds := float64(position) /
            float64(ui.AudioPanel.SampleRate)

        totalSeconds := float64(length) /
            float64(ui.AudioPanel.SampleRate)

        ui.Progress.SetValue(progress)
        ui.Current.SetText(helper.FormatTime(currentSeconds))
        ui.Total.SetText(helper.FormatTime(totalSeconds))
    }
}

func (ui *UI) updateCover(uri fyne.URI) {
    path, err := helper.FindImage(uri)
    if err != nil {
        dialog.ShowError(err, ui.Window)
        return
    }

    ui.Cover.File = path
    ui.Cover.Refresh()
}

func (ui *UI) savePreferences() {
    if ui.CurrentFileURI == nil {
        return
    }

    prefs := ui.App.Preferences()
    prefs.SetString("lastFile", ui.CurrentFileURI.String())

    if ui.AudioPanel != nil {
        position := float64(ui.AudioPanel.Position()) /
            float64(ui.AudioPanel.SampleRate)

        prefs.SetFloat("lastPosition", position)
        prefs.SetBool("wasPlaying", ui.Playing)
        prefs.SetFloat("volume", ui.Volume.Value)
        prefs.SetString("speed", ui.Speed.Selected)
    }
}

func (ui *UI) loadPreferences() {
    prefs := ui.App.Preferences()

    volume := prefs.FloatWithFallback("volume", 0.7)
    ui.Volume.SetValue(volume)
}