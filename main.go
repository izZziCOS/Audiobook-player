package main

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
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/izzzicos/audiobook-player/helper"
)

type audioPanel struct {
	sampleRate beep.SampleRate
	streamer   beep.StreamSeeker
	ctrl       *beep.Ctrl
	resampler  *beep.Resampler
	volume     *effects.Volume
}

func newAudioPanel(sampleRate beep.SampleRate, streamer beep.StreamSeeker) *audioPanel {
	ctrl := &beep.Ctrl{Streamer: beep.Loop(-1, streamer)}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	return &audioPanel{sampleRate, streamer, ctrl, resampler, volume}
}

func (ap *audioPanel) play() {
	speaker.Play(ap.volume)
}

func (ap *audioPanel) pause() {
	speaker.Lock()
	ap.ctrl.Paused = true
	speaker.Unlock()
}

func (ap *audioPanel) setVolume(vol float64) {
	speaker.Lock()
	ap.volume.Volume += vol
	speaker.Unlock()
}

func (ap *audioPanel) setSpeed(ratio float64) {
	speaker.Lock()
	ap.resampler.SetRatio(ratio)
	speaker.Unlock()
}

func (ap *audioPanel) skip(seconds int) {
	speaker.Lock()
	newPos := ap.streamer.Position() + ap.sampleRate.N(time.Duration(seconds)*time.Second)
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= ap.streamer.Len() {
		newPos = ap.streamer.Len() - 1
	}
	_ = ap.streamer.Seek(newPos)
	speaker.Unlock()
}

func tidyUp() {
	fmt.Println("Exited")
}

var (
	ap            *audioPanel // Audio panel for controlling playback
	currentFileURI fyne.URI   // Save current file location
)

// trackedStreamer wraps a StreamSeeker to track the total length and current position
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

// UpdateImage updates the image based on the selected MP3 file
func updateImage(image *canvas.Image, uri fyne.URI) {
	// Find an image file in the same directory as the MP3 file
	imagePath, err := helper.FindImage(uri)
	if err != nil {
		fmt.Println("Error finding image:", err)
		return
	}

	// Set the image source
	if imagePath != "" {
		image.File = imagePath
		image.Refresh() // Refresh the image to display the new file
	} else {
		// If no image is found, use a placeholder or clear the image
		image.File = ""
		image.Refresh()
	}
}
func main() {
	a := app.NewWithID("com.example.audiobookplayer")
	w := a.NewWindow("Audiobook player")
	w.Resize(fyne.NewSize(600, 700))
	w.SetFixedSize(true)

	label := widget.NewLabel("Select an MP3 file...")
	playBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil)
	pauseBtn := widget.NewButtonWithIcon("", theme.MediaPauseIcon(), nil)

	line := canvas.NewLine(color.White)
	line.StrokeWidth = 5

	speedIncBtn := widget.NewButton("Increase speed", nil)
	speedDecBtn := widget.NewButton("Decrease speed", nil)

	volumeIncBtn := widget.NewButtonWithIcon("",theme.VolumeUpIcon(), nil)
	volumeDecBtn := widget.NewButtonWithIcon("",theme.VolumeDownIcon(), nil)

	forwardBtn := widget.NewButtonWithIcon("",theme.MediaFastForwardIcon(), nil)
	backwardBtn := widget.NewButtonWithIcon("", theme.MediaFastRewindIcon(), nil)

	gridPlayControls := container.New(layout.NewGridLayout(6), volumeDecBtn, backwardBtn, playBtn, pauseBtn, forwardBtn, volumeIncBtn)
	gridSpeedControls := container.New(layout.NewGridLayout(2), speedDecBtn, speedIncBtn)

	// Display Audiobook image
	image := canvas.NewImageFromFile("") // Initially empty
	image.FillMode = canvas.ImageFillOriginal
	imageContainer := container.NewGridWrap(fyne.NewSize(500, 500), image)
	updateImageAndContainer := func(uri fyne.URI) {
    // Update the image based on the selected MP3 file
    updateImage(image, uri)

    // Refresh the image and container
    image.Refresh()
    imageContainer.Refresh()
}
	image.Resize(fyne.NewSize(500, 500))
	image.Refresh() // Force the image to refresh

	// Progress bar
	progressBar := widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 1

	// Time labels
	currentTimeLabel := widget.NewLabel("00:00:00")
	totalTimeLabel := widget.NewLabel("00:00:00")

	// Disable buttons initially
	playBtn.Disable()
	pauseBtn.Disable()
	speedIncBtn.Disable()
	speedDecBtn.Disable()
	volumeDecBtn.Disable()
	volumeIncBtn.Disable()
	forwardBtn.Disable()
	backwardBtn.Disable()

	// Function to update progress bar and time labels
	updateProgress := func() {
		for {
			time.Sleep(100 * time.Millisecond) // Update every 100ms
			if ap != nil && ap.streamer != nil {
				currentPos := ap.streamer.Position()
				totalLen := ap.streamer.Len()

				// Update progress bar
				progress := float64(currentPos) / float64(totalLen)
				if totalLen > 0 {
					progressBar.SetValue(progress)
				}

				// Update current time label
				currentSeconds := float64(currentPos) / float64(ap.sampleRate)
				currentTimeLabel.SetText(helper.FormatTime(currentSeconds))
				// Set Total time label
				totalTime := (float64(totalLen)) / float64(ap.sampleRate)
				totalTimeLabel.SetText(helper.FormatTime(totalTime))
			}
		}
	}

	// Start the progress updater
	go updateProgress()

	btn := widget.NewButton("Open MP3 File", func() {
		// Create a file filter for .mp3 files only
		filter := storage.NewExtensionFileFilter([]string{".mp3"})

		// Set a valid starting directory (e.g., user's home directory)
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Error getting home directory:", err)
			return
		}

		// Convert the home directory to a ListableURI
		startDir, err := storage.ListerForURI(storage.NewFileURI(homeDir))
		if err != nil {
			fmt.Println("Error creating listable URI:", err)
			return
		}

		// Create the file dialog with the filter and starting directory
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			uri := reader.URI()
			currentFileURI = uri
			label.TextStyle.Bold = true
			label.SetText("Playing: " + uri.Name())
			updateImageAndContainer(reader.URI())

			// Load and play the MP3 file
			file, err := os.Open(reader.URI().Path())
			if err != nil {
				fmt.Println("Error opening file:", err)
				return
			}

			// Decode the MP3 file
			streamer, format, err := mp3.Decode(file)
			if err != nil {
				fmt.Println("Error decoding MP3:", err)
				return
			}

			// Verify streamer length
			if streamer.Len() == 0 {
				fmt.Println("Error: Streamer length is 0")
				return
			}

			// Initialize the speaker with the sample rate of the MP3 file
			speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

			// Wrap the streamer with the trackedStreamer
			tracked := &trackedStreamer{
				StreamSeeker: streamer,
				totalSamples: streamer.Len(),
			}

			// Create the audio panel
			ap = newAudioPanel(format.SampleRate, tracked)

			// Enable Play button
			playBtn.Enable()

			// Set total time label
			totalSeconds := float64(tracked.Len()) / float64(format.SampleRate)
			totalTimeLabel.SetText(helper.FormatTime(totalSeconds))
		}, w)

		// Set the filter and starting directory for the file dialog
		fileDialog.SetFilter(filter)
		fileDialog.SetLocation(startDir)

		// Show the file dialog
		fileDialog.Show()
	})

	// Play button action
	playBtn.OnTapped = func() {
		if ap != nil {
			if ap.ctrl.Paused {
				ap.ctrl.Paused = false
			} else {
				ap.play()
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

	// Pause button action
	pauseBtn.OnTapped = func() {
		if ap != nil {
			ap.pause()
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

	// Speed increase button action
	speedIncBtn.OnTapped = func() {
		if ap != nil {
			ap.setSpeed(ap.resampler.Ratio() * 16 / 15)
		}
	}

	// Speed decrease button action
	speedDecBtn.OnTapped = func() {
		if ap != nil {
			ap.setSpeed(ap.resampler.Ratio() * 15 / 16)
		}
	}

	// Volume decrease button action
	volumeDecBtn.OnTapped = func() {
		if ap != nil {
			ap.setVolume(-0.1)
		}
	}

	// Volume increase button action
	volumeIncBtn.OnTapped = func() {
		if ap != nil {
			ap.setVolume(0.1)
		}
	}

	// Skip forward button action
	forwardBtn.OnTapped = func() {
		if ap != nil {
			ap.skip(30)
		}
	}

	// Skip backward button action
	backwardBtn.OnTapped = func() {
		if ap != nil {
			ap.skip(-30)
		}
	}

	timeContent := container.New(layout.NewBorderLayout(nil, nil, currentTimeLabel, totalTimeLabel), currentTimeLabel, totalTimeLabel)

	// Center the progress container
	centeredProgressContainer := container.NewCenter(container.NewGridWrap(fyne.NewSize(600, 20), progressBar))
	centeredImageContainer := container.NewCenter(imageContainer)

	w.SetContent(container.NewVBox(
		btn,
		container.NewCenter(label),
		centeredImageContainer,
		centeredProgressContainer,
		timeContent,
		gridPlayControls, gridSpeedControls,
	))

	// Save data when closed
	w.SetOnClosed(func() {
		if ap != nil && ap.streamer != nil && currentFileURI != nil {
			prefs := a.Preferences()
			currentPos := ap.streamer.Position()
			positionInSeconds := float64(currentPos) / float64(ap.sampleRate)
			isPlaying := !ap.ctrl.Paused

			fmt.Printf("Saving preferences: file=%s, position=%.2f, playing=%v\n", currentFileURI.String(), positionInSeconds, isPlaying)

			prefs.SetString("lastFile", currentFileURI.String())
			prefs.SetFloat("lastPosition", positionInSeconds)
			prefs.SetBool("wasPlaying", isPlaying)
		}
	})

	// Load preferences
	prefs := a.Preferences()
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
						dialog.ShowError(err, w)
						return
					}
					streamer, format, err := mp3.Decode(file)
					if err != nil {
						dialog.ShowError(err, w)
						return
					}
					speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
					ap = newAudioPanel(format.SampleRate, streamer)

					samples := int(float64(format.SampleRate) * savedPosition)
					if samples < 0 {
						samples = 0
					} else if samples >= streamer.Len() {
						samples = streamer.Len() - 1
					}
					if err := ap.streamer.Seek(samples); err != nil {
						dialog.ShowError(fmt.Errorf("failed to seek to position: %v", err), w)
						return
					}

					label.TextStyle.Bold = true
					currentFileURI = uri
					label.SetText(uri.Name())
					
					updateImageAndContainer(uri)

					if wasPlaying {
						ap.play()
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
				}, w)
				resumeDialog.SetConfirmText("Resume")
				resumeDialog.SetDismissText("Start Over")
				resumeDialog.Show()
			}
		}
	}

	w.ShowAndRun()
	tidyUp()
}