package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var speedOptions = []string{
    "0.5x",
    "0.6x",
    "0.7x",
    "0.75x",
    "0.8x",
    "0.9x",
    "1.0x",
    "1.1x",
    "1.2x",
    "1.25x",
    "1.3x",
    "1.4x",
    "1.5x",
    "1.6x",
    "1.7x",
    "1.75x",
    "1.8x",
    "1.9x",
    "2.0x",
}

func (ui *UI) buildContent() fyne.CanvasObject {
    titleStyle := widget.RichTextStyleSubHeading
    titleStyle.Alignment = fyne.TextAlignCenter
    ui.Title = newRichText("Open or drop an audiobook", titleStyle)

    subtitleStyle := newCaptionStyle(fyne.TextAlignCenter)
    subtitleStyle.TextStyle = fyne.TextStyle{}
    ui.Subtitle = newRichText("", subtitleStyle)
    ui.Subtitle.Hide()

    ui.Current = newRichText(formatTime(0), newCaptionStyle(fyne.TextAlignLeading))
    ui.Current.Truncation = fyne.TextTruncateOff
    ui.Remaining = newRichText("−"+formatTime(0), newCaptionStyle(fyne.TextAlignTrailing))
    ui.Remaining.Truncation = fyne.TextTruncateOff

    ui.Progress = widget.NewSlider(0, 1)
    ui.Progress.OnChanged = ui.previewSeek
    ui.Progress.OnChangeEnded = ui.finishSeek

    ui.Volume = widget.NewSlider(0, 1)
    ui.Volume.Step = 0.01
    ui.Volume.SetValue(0.7)
    ui.Volume.OnChanged = func(value float64) {
        if ui.Player != nil {
            ui.Player.SetVolume(value)
        }
    }
    ui.Volume.OnChangeEnded = func(float64) {
        ui.releaseFocus()
    }

    ui.Speed = widget.NewSelect(
        speedOptions,
        func(value string) {
            ui.changeSpeed(value)
            ui.releaseFocus()
        },
    )
    ui.Speed.PlaceHolder = "1.0x"
    ui.Speed.SetSelected("1.0x")

    content := container.NewBorder(
        ui.buildTopBar(),
        ui.buildNowPlaying(),
        nil,
        nil,
        container.NewPadded(ui.buildCover()),
    )

    return container.NewPadded(content)
}

func (ui *UI) buildTopBar() fyne.CanvasObject {
    openFileButton := widget.NewButtonWithIcon(
        "Open file",
        theme.FileAudioIcon(),
        ui.chooseFile,
    )

    openFolderButton := widget.NewButtonWithIcon(
        "Open folder",
        theme.FolderOpenIcon(),
        ui.chooseFolder,
    )

    ui.PartsButton = widget.NewButtonWithIcon(
        "Parts",
        theme.ListIcon(),
        ui.showParts,
    )
    ui.PartsButton.Hide()

    return container.NewBorder(
        nil,
        nil,
        nil,
        ui.PartsButton,
        container.NewGridWithColumns(2, openFileButton, openFolderButton),
    )
}

func (ui *UI) buildCover() fyne.CanvasObject {
    ui.Cover = canvas.NewImageFromFile("")
    ui.Cover.FillMode = canvas.ImageFillContain
    ui.Cover.SetMinSize(fyne.NewSquareSize(200))
    ui.Cover.Hide()

    placeholderBackground := canvas.NewRectangle(
        theme.Color(theme.ColorNameInputBackground),
    )
    placeholderBackground.CornerRadius = 16
    ui.followTheme(func() {
        placeholderBackground.FillColor = theme.Color(theme.ColorNameInputBackground)
        placeholderBackground.Refresh()
    })

    placeholderIcon := canvas.NewImageFromResource(
        theme.NewColoredResource(theme.MediaMusicIcon(), theme.ColorNamePlaceHolder),
    )
    placeholderIcon.SetMinSize(fyne.NewSquareSize(96))

    ui.CoverPlaceholder = container.NewStack(
        placeholderBackground,
        container.NewCenter(placeholderIcon),
    )

    return container.New(
        squareLayout{},
        ui.CoverPlaceholder,
        ui.Cover,
    )
}

func (ui *UI) buildNowPlaying() fyne.CanvasObject {
    progressRow := container.NewBorder(
        nil,
        nil,
        ui.Current,
        ui.Remaining,
        ui.Progress,
    )

    volumeRow := container.NewBorder(
        nil,
        nil,
        widget.NewIcon(theme.VolumeDownIcon()),
        container.NewHBox(
            widget.NewIcon(theme.VolumeUpIcon()),
            ui.Speed,
        ),
        ui.Volume,
    )

    return container.NewVBox(
        ui.Title,
        ui.Subtitle,
        progressRow,
        ui.buildTransport(),
        volumeRow,
    )
}

func (ui *UI) buildTransport() fyne.CanvasObject {
    ui.PreviousButton = newRoundButton(
        theme.MediaSkipPreviousIcon(),
        "",
        false,
        40,
        func() {
            ui.goToRelativePart(-1)
        },
    )
    ui.PreviousButton.Hide()

    ui.NextButton = newRoundButton(
        theme.MediaSkipNextIcon(),
        "",
        false,
        40,
        func() {
            ui.goToRelativePart(1)
        },
    )
    ui.NextButton.Hide()

    ui.PlayButton = newRoundButton(
        theme.MediaPlayIcon(),
        "",
        true,
        64,
        ui.togglePlay,
    )

    backButton := newRoundButton(
        skipBackIcon,
        "30",
        false,
        48,
        func() {
            ui.skip(-30)
        },
    )

    forwardButton := newRoundButton(
        skipForwardIcon,
        "30",
        false,
        48,
        func() {
            ui.skip(30)
        },
    )

    return container.NewCenter(
        container.NewHBox(
            ui.PreviousButton,
            backButton,
            layout.NewSpacer(),
            ui.PlayButton,
            layout.NewSpacer(),
            forwardButton,
            ui.NextButton,
        ),
    )
}

func (ui *UI) followTheme(apply func()) {
    changes := make(chan fyne.Settings)
    ui.App.Settings().AddChangeListener(changes)

    go func() {
        for range changes {
            apply()
        }
    }()
}

func (ui *UI) refreshLayout() {
    if content := ui.Window.Content(); content != nil {
        content.Refresh()
    }
}
