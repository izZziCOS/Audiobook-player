package audio

import (
	"fmt"
	"math"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
)

const OutputSampleRate = beep.SampleRate(44100)

func InitSpeaker() error {
	return speaker.Init(
		OutputSampleRate,
		OutputSampleRate.N(time.Second/10),
	)
}

type Player struct {
	SampleRate beep.SampleRate
	Streamer   beep.StreamSeekCloser
	Stretcher  *Stretcher
	Resampler  *beep.Resampler
	Ctrl       *beep.Ctrl
	Volume     *effects.Volume

	Speed  float64
	Closed bool
	OnEnd  func()

	queued bool
}

func NewPlayer(
	sampleRate beep.SampleRate,
	streamer beep.StreamSeekCloser,
) *Player {
	stretcher := NewStretcher(sampleRate, streamer)
	resampler := beep.Resample(4, sampleRate, OutputSampleRate, stretcher)
	ctrl := &beep.Ctrl{Streamer: resampler}

	volume := &effects.Volume{
		Streamer: ctrl,
		Base:     2,
		Volume:   0,
	}

	return &Player{
		SampleRate: sampleRate,
		Streamer:   streamer,
		Stretcher:  stretcher,
		Resampler:  resampler,
		Ctrl:       ctrl,
		Volume:     volume,
		Speed:      1.0,
	}
}

func (p *Player) Play() {
	if p == nil || p.Closed {
		return
	}

	speaker.Lock()
	p.Ctrl.Paused = false
	if p.Streamer.Position() >= p.Streamer.Len() {
		_ = p.seek(0)
	}
	alreadyQueued := p.queued
	p.queued = true
	speaker.Unlock()

	if !alreadyQueued {
		speaker.Play(beep.Seq(p.Volume, beep.Callback(p.finish)))
	}
}

func (p *Player) finish() {
	p.queued = false

	if p.OnEnd != nil && !p.Closed {
		go p.OnEnd()
	}
}

func (p *Player) Pause() {
	if p == nil || p.Closed {
		return
	}

	speaker.Lock()
	defer speaker.Unlock()

	p.Ctrl.Paused = true
}

func (p *Player) IsPaused() bool {
	if p == nil || p.Closed {
		return true
	}

	speaker.Lock()
	defer speaker.Unlock()

	return p.Ctrl.Paused
}

func (p *Player) SetVolume(value float64) {
	if p == nil || p.Closed {
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
	p.Volume.Volume = volume
	p.Volume.Silent = value == 0
	speaker.Unlock()
}

func (p *Player) SetSpeed(ratio float64) {
	if p == nil || p.Closed {
		return
	}

	if ratio < 0.5 {
		ratio = 0.5
	}
	if ratio > 2.0 {
		ratio = 2.0
	}

	speaker.Lock()
	p.Speed = ratio
	p.Stretcher.SetTempo(ratio)
	speaker.Unlock()
}

func (p *Player) SeekTo(seconds float64) error {
	if p == nil || p.Closed {
		return fmt.Errorf("no active audio stream")
	}

	speaker.Lock()
	defer speaker.Unlock()

	target := p.SampleRate.N(
		time.Duration(seconds * float64(time.Second)),
	)

	if target < 0 {
		target = 0
	}

	length := p.Streamer.Len()
	if length <= 0 {
		return fmt.Errorf("audio stream is empty")
	}

	if target >= length {
		target = length - 1
	}

	return p.seek(target)
}

func (p *Player) seek(target int) error {
	if err := p.Streamer.Seek(target); err != nil {
		return err
	}

	p.Stretcher.Reset()
	return nil
}

func (p *Player) Position() int {
	if p == nil || p.Closed {
		return 0
	}

	speaker.Lock()
	defer speaker.Unlock()

	return p.Streamer.Position()
}

func (p *Player) Length() int {
	if p == nil || p.Closed {
		return 0
	}

	speaker.Lock()
	defer speaker.Unlock()

	return p.Streamer.Len()
}

func (p *Player) Err() error {
	if p == nil || p.Closed {
		return nil
	}

	speaker.Lock()
	defer speaker.Unlock()

	return p.Streamer.Err()
}

func (p *Player) Waiting() bool {
	if p == nil || p.Closed {
		return false
	}

	waiter, ok := p.Streamer.(interface{ Waiting() bool })
	return ok && waiter.Waiting()
}

func (p *Player) Close() error {
	if p == nil || p.Closed {
		return nil
	}

	speaker.Lock()
	p.Ctrl.Paused = true
	p.Ctrl.Streamer = nil
	p.Closed = true
	speaker.Unlock()

	return p.Streamer.Close()
}

func log2(value float64) float64 {
	// log2(x) = ln(x) / ln(2)
	return math.Log(value) / math.Log(2)
}
