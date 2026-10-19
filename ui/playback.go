package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"

	"github.com/izzzicos/audiobook-player/audio"
)

func (ui *UI) loadAudio(path string) bool {
    file, err := os.Open(path)
    if err != nil {
        dialog.ShowError(err, ui.Window)
        return false
    }

    streamer, format, err := audio.Decode(file)
    if err != nil {
        file.Close()
        dialog.ShowError(err, ui.Window)
        return false
    }

    if streamer.Len() <= 0 {
        streamer.Close()
        file.Close()
        dialog.ShowError(
            fmt.Errorf("the selected audio file is empty"),
            ui.Window,
        )
        return false
    }

    if ui.Player != nil {
        _ = ui.Player.Close()
    }

    ui.Player = audio.NewPlayer(
        format.SampleRate,
        streamer,
    )
    ui.Player.OnEnd = ui.handlePartEnd

    ui.updateCover(path)

    total := ui.durationSeconds()
    ui.Progress.Max = total
    ui.Progress.Value = 0
    ui.Progress.Refresh()

    ui.setPlaying(false)
    ui.Player.SetVolume(ui.Volume.Value)
    ui.changeSpeed(ui.Speed.Selected)
    ui.updateTimes(0)

    return true
}

func (ui *UI) togglePlay() {
    if ui.Player == nil {
        return
    }

    if ui.Playing {
        ui.Player.Pause()
    } else {
        ui.Player.Play()
    }

    ui.setPlaying(!ui.Playing)
}

func (ui *UI) setPlaying(playing bool) {
    ui.Playing = playing

    if playing {
        ui.PlayButton.SetIcon(theme.MediaPauseIcon())
    } else {
        ui.PlayButton.SetIcon(theme.MediaPlayIcon())
    }
}

func (ui *UI) skip(seconds int) {
    if ui.Player == nil {
        return
    }

    if err := ui.Player.Skip(seconds); err != nil {
        dialog.ShowError(err, ui.Window)
        return
    }

    ui.refreshProgress()
}

func (ui *UI) changeSpeed(value string) {
    speed, err := strconv.ParseFloat(strings.TrimSuffix(value, "x"), 64)
    if err != nil || speed <= 0 {
        speed = 1.0
    }

    ui.speed = speed

    if ui.Player == nil {
        return
    }

    ui.Player.SetSpeed(speed)
    ui.refreshProgress()
}

func (ui *UI) changeVolume(delta float64) {
    ui.Volume.SetValue(min(max(ui.Volume.Value+delta, 0), 1))
}

func (ui *UI) previewSeek(value float64) {
    if ui.Player == nil {
        return
    }

    ui.seeking.Store(true)
    ui.updateTimes(value)
}

func (ui *UI) finishSeek(value float64) {
    if ui.Player != nil {
        _ = ui.Player.SeekTo(value)
    }

    ui.releaseFocus()

    ui.seeking.Store(false)
}

func (ui *UI) durationSeconds() float64 {
    if ui.Player == nil {
        return 0
    }

    length := ui.Player.Length()
    if length <= 0 {
        return 0
    }

    return float64(length) /
        float64(ui.Player.SampleRate)
}

func (ui *UI) updateProgress() {
    ticker := time.NewTicker(250 * time.Millisecond)
    defer ticker.Stop()

    for range ticker.C {
        ui.refreshProgress()
    }
}

func (ui *UI) refreshProgress() {
    player := ui.Player
    if player == nil || ui.seeking.Load() {
        return
    }

    currentSeconds := float64(player.Position()) /
        float64(player.SampleRate)

    ui.Progress.Value = currentSeconds
    ui.Progress.Refresh()
    ui.updateTimes(currentSeconds)
}

func (ui *UI) updateTimes(currentSeconds float64) {
    remaining := max(ui.Progress.Max-currentSeconds, 0) / ui.speed

    setRichText(ui.Current, formatTime(currentSeconds))
    setRichText(ui.Remaining, "−"+formatTime(remaining))
}
