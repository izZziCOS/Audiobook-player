package audio

import (
	"fmt"
	"math"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
)

type AudioPanel struct {
    SampleRate beep.SampleRate
    Streamer   beep.StreamSeekCloser
    Ctrl       *beep.Ctrl
    Resampler  *beep.Resampler
    Volume     *effects.Volume

    Speed  float64
    Closed bool
}

func NewAudioPanel(
    sampleRate beep.SampleRate,
    streamer beep.StreamSeekCloser,
) *AudioPanel {
    ctrl := &beep.Ctrl{Streamer: streamer}
    resampler := beep.ResampleRatio(4, 1, ctrl)

    volume := &effects.Volume{
        Streamer: resampler,
        Base:     2,
        Volume:   0,
    }

    return &AudioPanel{
        SampleRate: sampleRate,
        Streamer:   streamer,
        Ctrl:       ctrl,
        Resampler:  resampler,
        Volume:     volume,
        Speed:      1.0,
    }
}

func (ap *AudioPanel) Play() {
    if ap == nil || ap.Closed {
        return
    }

    speaker.Lock()
    ap.Ctrl.Paused = false
    speaker.Unlock()

    speaker.Play(ap.Volume)
}

func (ap *AudioPanel) Pause() {
    if ap == nil || ap.Closed {
        return
    }

    speaker.Lock()
    defer speaker.Unlock()

    ap.Ctrl.Paused = true
}

func (ap *AudioPanel) IsPaused() bool {
    if ap == nil || ap.Closed {
        return true
    }

    speaker.Lock()
    defer speaker.Unlock()

    return ap.Ctrl.Paused
}

func (ap *AudioPanel) SetVolume(value float64) {
    if ap == nil || ap.Closed {
        return
    }

    if value < 0 {
        value = 0
    }
    if value > 1 {
        value = 1
    }

    volume := -5.0
    if value > 0 {
        volume = 2 * log2(value)
    }

    speaker.Lock()
    ap.Volume.Volume = volume
    speaker.Unlock()
}

func (ap *AudioPanel) SetSpeed(ratio float64) {
    if ap == nil || ap.Closed {
        return
    }

    if ratio < 0.5 {
        ratio = 0.5
    }
    if ratio > 2.0 {
        ratio = 2.0
    }

    speaker.Lock()
    ap.Speed = ratio
    ap.Resampler.SetRatio(ratio)
    speaker.Unlock()
}

func (ap *AudioPanel) Skip(seconds int) error {
    if ap == nil || ap.Closed {
        return fmt.Errorf("no active audio stream")
    }

    speaker.Lock()
    defer speaker.Unlock()

    length := ap.Streamer.Len()
    if length <= 0 {
        return fmt.Errorf("audio stream is empty")
    }

    position := ap.Streamer.Position()
    target := position + ap.SampleRate.N(
        time.Duration(seconds)*time.Second,
    )

    if target < 0 {
        target = 0
    }
    if target >= length {
        target = length - 1
    }

    return ap.Streamer.Seek(target)
}

func (ap *AudioPanel) SeekTo(seconds float64) error {
    if ap == nil || ap.Closed {
        return fmt.Errorf("no active audio stream")
    }

    speaker.Lock()
    defer speaker.Unlock()

    target := ap.SampleRate.N(
        time.Duration(seconds * float64(time.Second)),
    )

    if target < 0 {
        target = 0
    }

    length := ap.Streamer.Len()
    if length <= 0 {
        return fmt.Errorf("audio stream is empty")
    }

    if target >= length {
        target = length - 1
    }

    return ap.Streamer.Seek(target)
}

func (ap *AudioPanel) Position() int {
    if ap == nil || ap.Closed {
        return 0
    }

    speaker.Lock()
    defer speaker.Unlock()

    return ap.Streamer.Position()
}

func (ap *AudioPanel) Length() int {
    if ap == nil || ap.Closed {
        return 0
    }

    speaker.Lock()
    defer speaker.Unlock()

    return ap.Streamer.Len()
}

func (ap *AudioPanel) Close() error {
    if ap == nil || ap.Closed {
        return nil
    }

    ap.Pause()
    ap.Closed = true

    return ap.Streamer.Close()
}

func log2(value float64) float64 {
    // log2(x) = ln(x) / ln(2)
    return math.Log(value) / math.Log(2)
}