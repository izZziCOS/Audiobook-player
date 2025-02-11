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
	"github.com/faiface/beep"
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
	Label          *widget.Label
	Controls       *PlayerControls
	Progress       *ProgressBar
	ImageContainer *ImageHandler
}

type PlayerControls struct {
	PlayBtn, PauseBtn, SpeedIncBtn, SpeedDecBtn, VolumeIncBtn, VolumeDecBtn, ForwardBtn, BackwardBtn *widget.Button
}

type ProgressBar struct {
	Bar              *widget.ProgressBar
	CurrentTimeLabel *widget.Label
	TotalTimeLabel   *widget.Label
}

type ImageHandler struct {
	Image         *canvas.Image
	Container     *fyne.Container
	UpdateImageFn func(uri fyne.URI)
}


func NewUI() *UI {
	a := app.NewWithID("com.example.audiobookplayer")
	w := a.NewWindow("Audiobook player")
	w.Resize(fyne.NewSize(600, 700))

	return &UI{
		App:    a,
		Window: w,
	}
}

func (ui *UI) SetupUI() {
	ui.Label = widget.NewLabel("Select an MP3 file...")
	ui.Controls = ui.createPlayerControls()
	ui.Progress = ui.createProgressBar()
	ui.ImageContainer = ui.createImageContainer()

	btn := widget.NewButton("Open MP3 File", ui.openMP3FileDialog)

	ui.Window.SetContent(container.NewVBox(
		btn,
		container.NewCenter(ui.Label),
		container.NewCenter(ui.ImageContainer.Container),
		ui.Progress.Bar,
		container.NewBorder(nil, nil, ui.Progress.CurrentTimeLabel, ui.Progress.TotalTimeLabel),
		ui.Controls.createControlGrid(),
		ui.Controls.createSpeedGrid(),
	))

	ui.Window.SetOnClosed(ui.savePreferences)
	ui.loadPreferences()

	ui.Window.ShowAndRun()
}

func (ui *UI) createPlayerControls() *PlayerControls {
	controls := &PlayerControls{
		PlayBtn:      widget.NewButtonWithIcon("", theme.MediaPlayIcon(), ui.play),
		PauseBtn:     widget.NewButtonWithIcon("", theme.MediaPauseIcon(), ui.pause),
		VolumeIncBtn: widget.NewButtonWithIcon("", theme.VolumeUpIcon(), ui.increaseVolume),
		VolumeDecBtn: widget.NewButtonWithIcon("", theme.VolumeDownIcon(), ui.decreaseVolume),
		ForwardBtn:   widget.NewButtonWithIcon("", theme.MediaFastForwardIcon(), ui.skipForward),
		BackwardBtn:  widget.NewButtonWithIcon("", theme.MediaFastRewindIcon(), ui.skipBackward),
		SpeedIncBtn:  widget.NewButton("Increase speed", ui.increaseSpeed),
		SpeedDecBtn:  widget.NewButton("Decrease speed", ui.decreaseSpeed),
	}

	controls.disableAll()
	return controls
}


func (pc *PlayerControls) createControlGrid() *fyne.Container {
	return container.New(layout.NewGridLayout(6),
		pc.VolumeDecBtn, pc.BackwardBtn, pc.PlayBtn, pc.PauseBtn, pc.ForwardBtn, pc.VolumeIncBtn)
}

func (pc *PlayerControls) createSpeedGrid() *fyne.Container {
	return container.New(layout.NewGridLayout(2),
		pc.SpeedDecBtn, pc.SpeedIncBtn)
}

func (pc *PlayerControls) disableAll() {
	pc.PlayBtn.Disable()
	pc.PauseBtn.Disable()
	pc.SpeedIncBtn.Disable()
	pc.SpeedDecBtn.Disable()
	pc.VolumeDecBtn.Disable()
	pc.VolumeIncBtn.Disable()
	pc.ForwardBtn.Disable()
	pc.BackwardBtn.Disable()
}

func (ui *UI) createProgressBar() *ProgressBar {
	progressBar := widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 1

	currentTimeLabel := widget.NewLabel("00:00:00")
	totalTimeLabel := widget.NewLabel("00:00:00")

	go ui.updateProgress(progressBar, currentTimeLabel, totalTimeLabel)

	return &ProgressBar{
		Bar:              progressBar,
		CurrentTimeLabel: currentTimeLabel,
		TotalTimeLabel:   totalTimeLabel,
	}
}

func (ui *UI) createImageContainer() *ImageHandler {
	image := canvas.NewImageFromFile("")
	image.FillMode = canvas.ImageFillOriginal
	imageContainer := container.NewGridWrap(fyne.NewSize(500, 500), image)

	return &ImageHandler{
		Image:     image,
		Container: imageContainer,
		UpdateImageFn: func(uri fyne.URI) {
			imagePath, err := helper.FindImage(uri)
			if err != nil {
				fmt.Println("Error finding image:", err)
				return
			}

			if imagePath != "" {
				image.File = imagePath
			} else {
				image.File = ""
			}
			image.Refresh()
			imageContainer.Refresh()
		},
	}
}

func (ui *UI) updateProgress(progressBar *widget.ProgressBar, currentTimeLabel, totalTimeLabel *widget.Label) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if ui.AudioPanel != nil && ui.AudioPanel.Streamer != nil {
			currentPos := ui.AudioPanel.Streamer.Position()
			totalLen := ui.AudioPanel.Streamer.Len()

			progress := float64(currentPos) / float64(totalLen)
			if totalLen > 0 {
				progressBar.SetValue(progress)
			}

			currentSeconds := float64(currentPos) / float64(ui.AudioPanel.SampleRate)
			currentTimeLabel.SetText(helper.FormatTime(currentSeconds))
			totalTime := float64(totalLen) / float64(ui.AudioPanel.SampleRate)
			totalTimeLabel.SetText(helper.FormatTime(totalTime))
		}
	}
}

func (ui *UI) openMP3FileDialog() {
	filter := storage.NewExtensionFileFilter([]string{".mp3"})

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}

	startDir, err := storage.ListerForURI(storage.NewFileURI(homeDir))
	if err != nil {
		fmt.Println("Error creating listable URI:", err)
		return
	}

	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		uri := reader.URI()
		ui.CurrentFileURI = uri
		ui.Label.TextStyle.Bold = true
		ui.Label.SetText("Playing: " + uri.Name())
		ui.ImageContainer.UpdateImageFn(uri)

		file, err := os.Open(reader.URI().Path())
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}

		streamer, format, err := mp3.Decode(file)
		if err != nil {
			fmt.Println("Error decoding MP3:", err)
			return
		}

		if streamer.Len() == 0 {
			fmt.Println("Error: Streamer length is 0")
			return
		}

		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

		tracked := &trackedStreamer{
			StreamSeeker: streamer,
			totalSamples: streamer.Len(),
		}

		ui.AudioPanel = audio.NewAudioPanel(format.SampleRate, tracked)
		ui.Controls.PlayBtn.Enable()

		totalSeconds := float64(tracked.Len()) / float64(format.SampleRate)
		ui.Progress.TotalTimeLabel.SetText(helper.FormatTime(totalSeconds))
	}, ui.Window)

	fileDialog.SetFilter(filter)
	fileDialog.SetLocation(startDir)
	fileDialog.Show()
}

func (ui *UI) play() {
	if ui.AudioPanel != nil {
		if ui.AudioPanel.Ctrl.Paused {
			ui.AudioPanel.Ctrl.Paused = false
		} else {
			ui.AudioPanel.Play()
		}
		ui.Controls.PlayBtn.Disable()
		ui.Controls.PauseBtn.Enable()
		ui.Controls.SpeedIncBtn.Enable()
		ui.Controls.SpeedDecBtn.Enable()
		ui.Controls.VolumeDecBtn.Enable()
		ui.Controls.VolumeIncBtn.Enable()
		ui.Controls.ForwardBtn.Enable()
		ui.Controls.BackwardBtn.Enable()
	}
}

func (ui *UI) pause() {
	if ui.AudioPanel != nil {
		ui.AudioPanel.Pause()
		ui.Controls.PlayBtn.Enable()
		ui.Controls.PauseBtn.Disable()
		ui.Controls.SpeedDecBtn.Disable()
		ui.Controls.SpeedIncBtn.Disable()
		ui.Controls.VolumeDecBtn.Disable()
		ui.Controls.VolumeIncBtn.Disable()
		ui.Controls.ForwardBtn.Disable()
		ui.Controls.BackwardBtn.Disable()
	}
}

func (ui *UI) savePreferences() {
	if ui.AudioPanel != nil && ui.AudioPanel.Streamer != nil && ui.CurrentFileURI != nil {
		prefs := ui.App.Preferences()
		currentPos := ui.AudioPanel.Streamer.Position()
		positionInSeconds := float64(currentPos) / float64(ui.AudioPanel.SampleRate)
		isPlaying := !ui.AudioPanel.Ctrl.Paused

		fmt.Printf("Saving preferences: file=%s, position=%.2f, playing=%v\n", ui.CurrentFileURI.String(), positionInSeconds, isPlaying)

		prefs.SetString("lastFile", ui.CurrentFileURI.String())
		prefs.SetFloat("lastPosition", positionInSeconds)
		prefs.SetBool("wasPlaying", isPlaying)
	}
}

func (ui *UI) loadPreferences() {
	prefs := ui.App.Preferences()
	savedURIStr := prefs.String("lastFile")
	savedPosition := prefs.Float("lastPosition")
	wasPlaying := prefs.Bool("wasPlaying")

	fmt.Printf("Loading preferences: file=%s, position=%.2f, playing=%v\n", savedURIStr, savedPosition, wasPlaying)

	if savedURIStr != "" && savedPosition > 0 {
		uri, err := storage.ParseURI(savedURIStr)
		if err != nil {
			fmt.Println("Error parsing saved URI:", err)
			prefs.SetString("lastFile", "") // Clear invalid URI
		} else {
			filePath := uri.Path()
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				fmt.Println("Saved file no longer exists:", filePath)
				prefs.SetString("lastFile", "") // Clear invalid file
				prefs.SetFloat("lastPosition", 0)
				prefs.SetBool("wasPlaying", false)
			} else {
				resumeDialog := dialog.NewConfirm("Resume Playback", "Resume playback from "+uri.Name()+"?", func(resume bool) {
					if !resume {
						prefs.SetString("lastFile", "")
						prefs.SetFloat("lastPosition", 0)
						prefs.SetBool("wasPlaying", false)
						return
					}

					file, err := os.Open(filePath)
					if err != nil {
						dialog.ShowError(err, ui.Window)
						return
					}

					streamer, format, err := mp3.Decode(file)
					if err != nil {
						dialog.ShowError(err, ui.Window)
						return
					}

					speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
					ui.AudioPanel = audio.NewAudioPanel(format.SampleRate, streamer)

					samples := int(float64(format.SampleRate) * savedPosition)
					if samples < 0 {
						samples = 0
					} else if samples >= streamer.Len() {
						samples = streamer.Len() - 1
					}
					if err := ui.AudioPanel.Streamer.Seek(samples); err != nil {
						dialog.ShowError(fmt.Errorf("failed to seek to position: %v", err), ui.Window)
						return
					}

					ui.CurrentFileURI = uri
					ui.Label.TextStyle.Bold = true
					ui.Label.SetText(uri.Name())
					ui.ImageContainer.UpdateImageFn(uri)

					if wasPlaying {
						ui.AudioPanel.Play()
						ui.Controls.PlayBtn.Disable()
						ui.Controls.PauseBtn.Enable()
						ui.Controls.SpeedIncBtn.Enable()
						ui.Controls.SpeedDecBtn.Enable()
						ui.Controls.VolumeDecBtn.Enable()
						ui.Controls.VolumeIncBtn.Enable()
						ui.Controls.ForwardBtn.Enable()
						ui.Controls.BackwardBtn.Enable()
					} else {
						ui.Controls.PlayBtn.Enable()
						ui.Controls.PauseBtn.Disable()
					}
				}, ui.Window)
				resumeDialog.SetConfirmText("Resume")
				resumeDialog.SetDismissText("Start Over")
				resumeDialog.Show()
			}
		}
	}
}

func updateImage(image *canvas.Image, uri fyne.URI) {
	imagePath, err := helper.FindImage(uri)
	if err != nil {
		fmt.Println("Error finding image:", err)
		return
	}

	if imagePath != "" {
		image.File = imagePath
		image.Refresh()
	} else {
		image.File = ""
		image.Refresh()
	}
}

func (ui *UI) increaseSpeed() {
	if ui.AudioPanel != nil {
		ui.AudioPanel.SetSpeed(ui.AudioPanel.Resampler.Ratio() * 16 / 15)
	}
}

func (ui *UI) decreaseSpeed() {
	if ui.AudioPanel != nil {
		ui.AudioPanel.SetSpeed(ui.AudioPanel.Resampler.Ratio() * 15 / 16)
	}
}

func (ui *UI) increaseVolume() {
	if ui.AudioPanel != nil {
		ui.AudioPanel.SetVolume(0.1)
	}
}

func (ui *UI) decreaseVolume() {
	if ui.AudioPanel != nil {
		ui.AudioPanel.SetVolume(-0.1)
	}
}

func (ui *UI) skipForward() {
	if ui.AudioPanel != nil {
		ui.AudioPanel.Skip(30)
	}
}

func (ui *UI) skipBackward() {
	if ui.AudioPanel != nil {
		ui.AudioPanel.Skip(-30)
	}
}

type trackedStreamer struct {
	beep.StreamSeeker
	totalSamples int
	currentPos   int
}

func (ts *trackedStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = ts.StreamSeeker.Stream(samples)
	ts.currentPos += n
	return n, ok
}

func (ts *trackedStreamer) Seek(p int) error {
	err := ts.StreamSeeker.Seek(p)
	if err == nil {
		ts.currentPos = p
	}
	return err
}

func (ts *trackedStreamer) Len() int {
	return ts.totalSamples
}

func (ts *trackedStreamer) Position() int {
	return ts.currentPos
}