package ui

import (
	"fmt"
	"image/color"
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
}

func NewUI() *UI {
	a := app.NewWithID("com.example.audiobookplayer")
	w := a.NewWindow("Audiobook player")
	w.Resize(fyne.NewSize(600, 700))
	w.SetFixedSize(true)

	return &UI{
		App:    a,
		Window: w,
	}
}

func (ui *UI) SetupUI() {
	label := widget.NewLabel("Select an MP3 file...")
	playBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil)
	pauseBtn := widget.NewButtonWithIcon("", theme.MediaPauseIcon(), nil)

	line := canvas.NewLine(color.White)
	line.StrokeWidth = 5

	speedIncBtn := widget.NewButton("Increase speed", nil)
	speedDecBtn := widget.NewButton("Decrease speed", nil)

	volumeIncBtn := widget.NewButtonWithIcon("", theme.VolumeUpIcon(), nil)
	volumeDecBtn := widget.NewButtonWithIcon("", theme.VolumeDownIcon(), nil)

	forwardBtn := widget.NewButtonWithIcon("", theme.MediaFastForwardIcon(), nil)
	backwardBtn := widget.NewButtonWithIcon("", theme.MediaFastRewindIcon(), nil)

	gridPlayControls := container.New(layout.NewGridLayout(6), volumeDecBtn, backwardBtn, playBtn, pauseBtn, forwardBtn, volumeIncBtn)
	gridSpeedControls := container.New(layout.NewGridLayout(2), speedDecBtn, speedIncBtn)

	image := canvas.NewImageFromFile("")
	image.FillMode = canvas.ImageFillOriginal
	imageContainer := container.NewGridWrap(fyne.NewSize(500, 500), image)
	updateImageAndContainer := func(uri fyne.URI) {
		updateImage(image, uri)
		image.Refresh()
		imageContainer.Refresh()
	}
	image.Resize(fyne.NewSize(500, 500))
	image.Refresh()

	progressBar := widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 1

	currentTimeLabel := widget.NewLabel("00:00:00")
	totalTimeLabel := widget.NewLabel("00:00:00")

	playBtn.Disable()
	pauseBtn.Disable()
	speedIncBtn.Disable()
	speedDecBtn.Disable()
	volumeDecBtn.Disable()
	volumeIncBtn.Disable()
	forwardBtn.Disable()
	backwardBtn.Disable()

	updateProgress := func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if ui.AudioPanel != nil && ui.AudioPanel.Streamer != nil {
				currentPos := ui.AudioPanel.Streamer.Position()
				totalLen := ui.AudioPanel.Streamer.Len()

				progress := float64(currentPos) / float64(totalLen)
				if totalLen > 0 {
					progressBar.SetValue(progress)
				}

				currentSeconds := float64(currentPos) / float64(ui.AudioPanel.SampleRate)
				currentTimeLabel.SetText(helper.FormatTime(currentSeconds))
				totalTime := (float64(totalLen)) / float64(ui.AudioPanel.SampleRate)
				totalTimeLabel.SetText(helper.FormatTime(totalTime))
			}
		}
	}

	go updateProgress()

	btn := widget.NewButton("Open MP3 File", func() {
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
			label.TextStyle.Bold = true
			label.SetText("Playing: " + uri.Name())
			updateImageAndContainer(reader.URI())

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

			playBtn.Enable()

			totalSeconds := float64(tracked.Len()) / float64(format.SampleRate)
			totalTimeLabel.SetText(helper.FormatTime(totalSeconds))
		}, ui.Window)

		fileDialog.SetFilter(filter)
		fileDialog.SetLocation(startDir)
		fileDialog.Show()
	})

	playBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			if ui.AudioPanel.Ctrl.Paused {
				ui.AudioPanel.Ctrl.Paused = false
			} else {
				ui.AudioPanel.Play()
			}
			playBtn.Disable()
			pauseBtn.Enable()
			speedIncBtn.Enable()
			speedDecBtn.Enable()
			volumeDecBtn.Enable()
			volumeIncBtn.Enable()
			forwardBtn.Enable()
			backwardBtn.Enable()
		}
	}

	pauseBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			ui.AudioPanel.Pause()
			playBtn.Enable()
			pauseBtn.Disable()
			speedDecBtn.Disable()
			speedIncBtn.Disable()
			volumeDecBtn.Disable()
			volumeIncBtn.Disable()
			forwardBtn.Disable()
			backwardBtn.Disable()
		}
	}

	speedIncBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			ui.AudioPanel.SetSpeed(ui.AudioPanel.Resampler.Ratio() * 16 / 15)
		}
	}

	speedDecBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			ui.AudioPanel.SetSpeed(ui.AudioPanel.Resampler.Ratio() * 15 / 16)
		}
	}

	volumeDecBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			ui.AudioPanel.SetVolume(-0.1)
		}
	}

	volumeIncBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			ui.AudioPanel.SetVolume(0.1)
		}
	}

	forwardBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			ui.AudioPanel.Skip(30)
		}
	}

	backwardBtn.OnTapped = func() {
		if ui.AudioPanel != nil {
			ui.AudioPanel.Skip(-30)
		}
	}

	timeContent := container.New(layout.NewBorderLayout(nil, nil, currentTimeLabel, totalTimeLabel), currentTimeLabel, totalTimeLabel)

	centeredProgressContainer := container.NewCenter(container.NewGridWrap(fyne.NewSize(600, 20), progressBar))
	centeredImageContainer := container.NewCenter(imageContainer)

	ui.Window.SetContent(container.NewVBox(
		btn,
		container.NewCenter(label),
		centeredImageContainer,
		centeredProgressContainer,
		timeContent,
		gridPlayControls, gridSpeedControls,
	))

	ui.Window.SetOnClosed(func() {
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
	})

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
				// Show the resume dialog
				resumeDialog := dialog.NewConfirm("Resume Playback", "Resume playback from "+uri.Name()+"?", func(resume bool) {
					if !resume {
						prefs.SetString("lastFile", "")
						prefs.SetFloat("lastPosition", 0)
						prefs.SetBool("wasPlaying", false)
						return
					}

					// Load the file and resume playback
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

					label.TextStyle.Bold = true
					ui.CurrentFileURI = uri
					label.SetText(uri.Name())

					updateImageAndContainer(uri)

					if wasPlaying {
						ui.AudioPanel.Play()
						playBtn.Disable()
						pauseBtn.Enable()
						speedIncBtn.Enable()
						speedDecBtn.Enable()
						volumeDecBtn.Enable()
						volumeIncBtn.Enable()
						forwardBtn.Enable()
						backwardBtn.Enable()
					} else {
						playBtn.Enable()
						pauseBtn.Disable()
					}
				}, ui.Window)
				resumeDialog.SetConfirmText("Resume")
				resumeDialog.SetDismissText("Start Over")
				resumeDialog.Show()
			}
		}
	}

	ui.Window.ShowAndRun()
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