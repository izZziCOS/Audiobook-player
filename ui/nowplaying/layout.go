package nowplaying

import (
	"image/color"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/ui/kit"
)

const backdropAlpha = 0x66

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

func (p *Player) buildTopBar() fyne.CanvasObject {
	var openButton *widget.Button
	openButton = widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), func() {
		kit.ShowMenuBelow(p.window, openButton, fyne.NewMenu("",
			fyne.NewMenuItem("Open file…", p.chooseFile),
			fyne.NewMenuItem("Open folder…", p.chooseFolder),
		))
	})
	openButton.Importance = widget.LowImportance

	p.partsButton = widget.NewButtonWithIcon(
		"Parts",
		theme.ListIcon(),
		p.showParts,
	)
	p.partsButton.Importance = widget.LowImportance
	p.partsButton.Hide()

	p.editButton = widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), p.editDetails)
	p.editButton.Importance = widget.LowImportance
	p.editButton.Hide()

	p.aboutButton = widget.NewButtonWithIcon("About", theme.InfoIcon(), p.showAbout)
	p.aboutButton.Importance = widget.LowImportance
	p.aboutButton.Hide()

	return container.NewBorder(nil, nil, openButton, container.NewHBox(p.aboutButton, p.editButton, p.partsButton))
}

func (p *Player) buildCover() fyne.CanvasObject {
	p.cover = canvas.NewImageFromFile("")
	p.cover.FillMode = canvas.ImageFillContain
	p.cover.CornerRadius = 18
	p.cover.SetMinSize(fyne.NewSquareSize(200))
	p.cover.Hide()

	placeholderBackground := canvas.NewRectangle(
		theme.Color(theme.ColorNameInputBackground),
	)
	placeholderBackground.CornerRadius = 18
	kit.FollowTheme(func() {
		placeholderBackground.FillColor = theme.Color(theme.ColorNameInputBackground)
		placeholderBackground.Refresh()
	})

	placeholderIcon := canvas.NewImageFromResource(
		theme.NewColoredResource(theme.MediaMusicIcon(), theme.ColorNamePlaceHolder),
	)
	placeholderIcon.SetMinSize(fyne.NewSquareSize(96))

	p.coverPlaceholder = container.NewStack(
		placeholderBackground,
		container.NewCenter(placeholderIcon),
	)

	return container.New(
		squareLayout{},
		p.coverPlaceholder,
		p.cover,
	)
}

func (p *Player) buildNowPlaying() fyne.CanvasObject {
	progress := container.New(
		layout.NewCustomPaddedVBoxLayout(-theme.Padding()),
		p.progress,
		container.NewBorder(nil, nil, p.current, p.remaining),
	)

	controls := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(p.speedButton, p.buildSleepButton()),
		container.NewBorder(
			nil,
			nil,
			widget.NewIcon(theme.VolumeDownIcon()),
			widget.NewIcon(theme.VolumeUpIcon()),
			p.volume,
		),
	)

	return container.NewVBox(
		p.title,
		p.subtitle,
		layout.NewSpacer(),
		progress,
		p.buildTransport(),
		controls,
	)
}

func (p *Player) buildTransport() fyne.CanvasObject {
	p.previousButton = newRoundButton(
		theme.MediaSkipPreviousIcon(),
		"",
		false,
		44,
		func() {
			p.previous()
		},
	)
	p.previousButton.Hide()

	p.nextButton = newRoundButton(
		theme.MediaSkipNextIcon(),
		"",
		false,
		44,
		func() {
			p.next()
		},
	)
	p.nextButton.Hide()

	p.playButton = newRoundButton(
		theme.MediaPlayIcon(),
		"",
		true,
		72,
		p.TogglePlay,
	)

	backButton := newRoundButton(
		kit.SkipBackIcon,
		strconv.Itoa(skipSeconds),
		false,
		52,
		func() {
			p.Skip(-skipSeconds)
		},
	)

	forwardButton := newRoundButton(
		kit.SkipForwardIcon,
		strconv.Itoa(skipSeconds),
		false,
		52,
		func() {
			p.Skip(skipSeconds)
		},
	)

	return container.NewCenter(
		container.New(
			layout.NewCustomPaddedHBoxLayout(theme.Padding()*3),
			container.NewCenter(p.previousButton),
			container.NewCenter(backButton),
			p.playButton,
			container.NewCenter(forwardButton),
			container.NewCenter(p.nextButton),
		),
	)
}

func (p *Player) setBackdropTint(tint *color.NRGBA) {
	p.backdropTint = tint
	p.applyBackdrop()
}

func (p *Player) applyBackdrop() {
	tint := color.NRGBAModel.Convert(theme.Color(theme.ColorNamePrimary)).(color.NRGBA)
	if p.backdropTint != nil {
		tint = *p.backdropTint
	}

	start := tint
	start.A = backdropAlpha
	end := tint
	end.A = 0

	p.backdrop.StartColor = start
	p.backdrop.EndColor = end
	p.backdrop.Refresh()
}

func (p *Player) chooseSpeed(value string) {
	p.selectSpeed(value)
	p.prefs.SetString(preferenceSpeed, p.speedChoice)

	if p.book != nil {
		edit := p.data.Edit(p.book.Key())
		edit.Speed = p.speedChoice
		p.data.SetEdit(p.book.Key(), edit)
		if p.OnSpeedChanged != nil {
			p.OnSpeedChanged(p.book.Key())
		}
	}
}

func (p *Player) bookSpeed(key string) string {
	if speed := p.data.Edit(key).Speed; slices.Contains(speedOptions, speed) {
		return speed
	}

	return p.prefs.StringWithFallback(preferenceSpeed, "1.0x")
}

func (p *Player) SpeedFor(key string) float64 {
	return speedValue(p.bookSpeed(key))
}

func (p *Player) selectSpeed(value string) {
	if !slices.Contains(speedOptions, value) {
		value = "1.0x"
	}

	p.speedChoice = value
	p.speedButton.SetText(strings.TrimSuffix(value, "x") + "×")
	p.changeSpeed(value)
}

func (p *Player) showSpeedMenu() {
	items := make([]*fyne.MenuItem, len(speedOptions))
	for i, option := range speedOptions {
		items[i] = fyne.NewMenuItem(strings.TrimSuffix(option, "x")+"×", func() {
			p.chooseSpeed(option)
		})
		items[i].Checked = option == p.speedChoice
	}

	kit.ShowMenuAbove(p.window, p.speedButton, fyne.NewMenu("Speed", items...))
}
