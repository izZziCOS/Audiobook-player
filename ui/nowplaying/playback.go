package nowplaying

import (
    "fmt"
        "os"
        "strconv"
        "strings"
        "time"

        "fyne.io/fyne/v2"
        "fyne.io/fyne/v2/dialog"
        "fyne.io/fyne/v2/theme"

        "github.com/izzzicos/audiobook-player/audio"
        "github.com/izzzicos/audiobook-player/library"
        "github.com/izzzicos/audiobook-player/ui/kit"
    )

func (p *Player) loadAudio(path string) bool {
    file, err := os.Open(path)
    if err != nil {
        dialog.ShowError(err, p.window)
        return false
    }

    streamer, format, err := audio.Decode(file)
    if err != nil {
        file.Close()
        dialog.ShowError(err, p.window)
        return false
    }

    if streamer.Len() <= 0 {
        streamer.Close()
        file.Close()
        dialog.ShowError(
            fmt.Errorf("the selected audio file is empty"),
            p.window,
        )
        return false
    }

    if p.audio != nil {
        _ = p.audio.Close()
    }

    player := audio.NewPlayer(
        format.SampleRate,
        streamer,
    )
    player.OnEnd = func() {
        fyne.Do(func() {
            if p.audio == player {
                p.handlePartEnd()
            }
        })
    }
    p.audio = player

    p.metadata = library.ReadMetadata(path)
    p.chapters = library.ReadChapters(path)
    p.updateCover(path, p.metadata.Picture)

    p.setPlaying(false)
    p.audio.SetVolume(p.volume.Value)
    p.changeSpeed(p.speedChoice)

    return true
}

func (p *Player) TogglePlay() {
    if p.audio == nil {
        return
    }

    if p.playing {
        p.audio.Pause()
        p.SavePosition()
    } else {
        p.audio.Play()
    }

    p.setPlaying(!p.playing)
}

func (p *Player) setPlaying(playing bool) {
    p.playing = playing

    if playing {
        p.playButton.SetIcon(theme.MediaPauseIcon())
    } else {
        p.playButton.SetIcon(theme.MediaPlayIcon())
    }

    p.publishStatus(time.Now())
}

func (p *Player) Skip(seconds int) {
    p.jumpTo(p.Elapsed() + float64(seconds))
}

func (p *Player) jumpTo(seconds float64) {
    if p.audio == nil {
        return
    }

    if err := p.Seek(seconds); err != nil {
        dialog.ShowError(err, p.window)
        return
    }

    p.refreshProgress()
    p.SavePosition()
}

func (p *Player) Seek(seconds float64) error {
    if p.book == nil || !p.book.HasDurations() {
        return p.audio.SeekTo(seconds)
    }

    index, position := p.book.Locate(seconds)
    if index == p.book.Index {
        return p.audio.SeekTo(position)
    }

    playing := p.playing
    p.goToPart(index, false)
    if p.book.Index != index {
        return nil
    }

    err := p.audio.SeekTo(position)
    if playing {
        p.audio.Play()
        p.setPlaying(true)
    }

    return err
}

func (p *Player) changeSpeed(value string) {
    speed, err := strconv.ParseFloat(strings.TrimSuffix(value, "x"), 64)
    if err != nil || speed <= 0 {
        speed = 1.0
    }

    p.speed = speed

    if p.audio == nil {
        return
    }

    p.audio.SetSpeed(speed)
    p.refreshProgress()
}

func (p *Player) changeVolume(delta float64) {
    p.volume.SetValue(min(max(p.volume.Value+delta, 0), 1))
}

func (p *Player) previewSeek(value float64) {
    if p.audio == nil {
        return
    }

    p.seeking.Store(true)
    p.updateTimes(value)
}

func (p *Player) finishSeek(value float64) {
    if p.audio != nil {
        _ = p.Seek(value)
    }

    p.releaseFocus()

    p.seeking.Store(false)
    p.refreshProgress()
    p.SavePosition()
}

func (p *Player) durationSeconds() float64 {
    if p.audio == nil {
        return 0
    }

    length := p.audio.Length()
    if length <= 0 {
        return 0
    }

    return float64(length) /
        float64(p.audio.SampleRate)
}

func (p *Player) positionSeconds() float64 {
    if p.audio == nil {
        return 0
    }

    return float64(p.audio.Position()) /
        float64(p.audio.SampleRate)
}

func (p *Player) totalSeconds() float64 {
    if p.book != nil && p.book.HasDurations() {
        return p.book.TotalDuration()
    }

    return p.durationSeconds()
}

func (p *Player) Elapsed() float64 {
    if p.book != nil && p.book.HasDurations() {
        return p.book.Offset() + p.positionSeconds()
    }

    return p.positionSeconds()
}

func (p *Player) resetProgress() {
    p.progress.Max = p.totalSeconds()
    p.progress.Refresh()
    p.refreshProgress()
}

func (p *Player) refreshProgress() {
    if p.audio == nil || p.seeking.Load() {
        return
    }

    currentSeconds := p.Elapsed()

    p.progress.Value = currentSeconds
    p.progress.Refresh()
    p.updateTimes(currentSeconds)
    p.updateChapter()
    p.publishStatus(time.Now())
}

func (p *Player) updateTimes(currentSeconds float64) {
    remaining := max(p.progress.Max-currentSeconds, 0) / p.speed

    setRichText(p.current, kit.FormatTime(currentSeconds))
    setRichText(p.remaining, "−"+kit.FormatTime(remaining))
}
