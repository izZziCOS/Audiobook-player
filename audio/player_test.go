package audio

import (
	"errors"
	"math"
	"testing"

	"github.com/faiface/beep"
)

type fakeStream struct {
	sliceStreamer
	err error
}

func (s *fakeStream) Err() error {
	return s.err
}

func (s *fakeStream) Len() int {
	return len(s.samples)
}

func (s *fakeStream) Position() int {
	return s.position
}

func (s *fakeStream) Seek(position int) error {
	s.position = position
	return nil
}

func (s *fakeStream) Close() error {
	return nil
}

func newTestPlayer() (*Player, *fakeStream) {
	sampleRate := beep.SampleRate(44100)
	source := &fakeStream{sliceStreamer: sliceStreamer{samples: sine(sampleRate, 220, 1)}}

	return NewPlayer(sampleRate, source), source
}

func peakAtVolume(value float64) float64 {
	player, _ := newTestPlayer()
	player.SetVolume(value)

	buffer := make([][2]float64, 4410)
	player.Volume.Stream(buffer)

	peak := 0.0
	for _, sample := range buffer {
		peak = math.Max(peak, math.Abs(sample[0]))
	}

	return peak
}

func TestVolumeIsSilentAtZeroAndRisesSteadily(t *testing.T) {
	if peak := peakAtVolume(0); peak != 0 {
		t.Errorf("volume 0: peak %.4f, want silence", peak)
	}

	previous := 0.0
	for _, value := range []float64{0.05, 0.1, 0.5, 1} {
		peak := peakAtVolume(value)
		if peak <= previous {
			t.Errorf("volume %.2f: peak %.4f, want louder than %.4f", value, peak, previous)
		}

		previous = peak
	}
}

func TestPlayerReportsDecoderErrors(t *testing.T) {
	player, source := newTestPlayer()

	if err := player.Err(); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	source.err = errors.New("corrupt frame")
	if err := player.Err(); !errors.Is(err, source.err) {
		t.Fatalf("got %v, want %v", err, source.err)
	}

	player.Close()
	if err := player.Err(); err != nil {
		t.Fatalf("a closed player should report no error, got %v", err)
	}
}
