package main

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
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

// isImageFile checks if the file has an image extension
func isImageFile(filename string) bool {
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp"}
	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return true
		}
	}
	return false
}

func findImage() (string, error) {
	dir, err := os.Getwd()
	imageName := ""
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return "", err
	}

	// Read files in the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return "", err
	}

	// Loop through files and find images
	for _, file := range files {
		if !file.IsDir() && isImageFile(file.Name()) {
			fmt.Println("Found image:", file.Name())
			imageName = file.Name()
		}
	}
	return imageName, err
}

var (
	ap            *audioPanel // Audio panel for controlling playback
	currentFileURI fyne.URI   // Save current file location
)

func main() {
	// Use a unique ID for the app
	a := app.NewWithID("com.example.audiobookplayer")
	w := a.NewWindow("Audiobook player")
	w.Resize(fyne.NewSize(800, 800))

	label := widget.NewLabel("Select an MP3 file...")
	playBtn := widget.NewButton("Play", nil)
	pauseBtn := widget.NewButton("Pause", nil)

	line := canvas.NewLine(color.White)
	line.StrokeWidth = 5

	speedIncBtn := widget.NewButton("Increase speed", nil)
	speedDecBtn := widget.NewButton("Decrease speed", nil)

	volumeIncBtn := widget.NewButton("Increase volume", nil)
	volumeDecBtn := widget.NewButton("Decrease volume", nil)

	forwardBtn := widget.NewButton("Skip forward", nil)
	backwardBtn := widget.NewButton("Skip backward", nil)

	gridPlayControls := container.New(layout.NewGridLayout(2), playBtn, pauseBtn)
	gridVolumeControls := container.New(layout.NewGridLayout(2), volumeIncBtn, volumeDecBtn)
	gridSpeedControls := container.New(layout.NewGridLayout(2), speedIncBtn, speedDecBtn)
	gridSkipControls := container.New(layout.NewGridLayout(2), forwardBtn, backwardBtn)

	imageName, err := findImage()
	if err != nil {
		fmt.Println("Error reading directory:", err)
	}
	fmt.Println(imageName)
	image := canvas.NewImageFromURI(storage.NewFileURI(imageName))
	image.FillMode = canvas.ImageFillOriginal

	// Disable buttons initially
	playBtn.Disable()
	pauseBtn.Disable()
	speedIncBtn.Disable()
	speedDecBtn.Disable()
	volumeDecBtn.Disable()
	volumeIncBtn.Disable()
	forwardBtn.Disable()
	backwardBtn.Disable()

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
			label.SetText("Selected: " + uri.Name())
			fmt.Println(uri)
			image = canvas.NewImageFromURI(uri)

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

			// Initialize the speaker with the sample rate of the MP3 file
			speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

			// Create the audio panel
			ap = newAudioPanel(format.SampleRate, streamer)

			// Enable Play button
			playBtn.Enable()
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

	w.SetContent(container.NewVBox(
		btn,
		label,
		image,
		gridPlayControls, gridVolumeControls, gridSpeedControls, gridSkipControls,
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

	// After setting up UI elements but before w.ShowAndRun()
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

					currentFileURI = uri
					label.SetText("Resumed: " + uri.Name())

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