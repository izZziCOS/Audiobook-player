package audio

import (
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
)

type AudioPanel struct {
	SampleRate beep.SampleRate
	Streamer   beep.StreamSeeker
	Ctrl       *beep.Ctrl
	Resampler  *beep.Resampler
	Volume     *effects.Volume
}

func NewAudioPanel(sampleRate beep.SampleRate, streamer beep.StreamSeeker) *AudioPanel {
	ctrl := &beep.Ctrl{Streamer: beep.Loop(-1, streamer)}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	return &AudioPanel{sampleRate, streamer, ctrl, resampler, volume}
}

func (ap *AudioPanel) Play() {
	speaker.Play(ap.Volume)
}

func (ap *AudioPanel) Pause() {
	speaker.Lock()
	ap.Ctrl.Paused = true
	speaker.Unlock()
}

func (ap *AudioPanel) SetVolume(vol float64) {
	speaker.Lock()
	ap.Volume.Volume += vol
	speaker.Unlock()
}

func (ap *AudioPanel) SetSpeed(ratio float64) {
	speaker.Lock()
	ap.Resampler.SetRatio(ratio)
	speaker.Unlock()
}

func (ap *AudioPanel) Skip(seconds int) {
	speaker.Lock()
	newPos := ap.Streamer.Position() + ap.SampleRate.N(time.Duration(seconds)*time.Second)
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= ap.Streamer.Len() {
		newPos = ap.Streamer.Len() - 1
	}
	_ = ap.Streamer.Seek(newPos)
	speaker.Unlock()
}