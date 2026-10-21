package audio

import (
	"math"
	"testing"

	"github.com/faiface/beep"
)

type sliceStreamer struct {
	samples  [][2]float64
	position int
}

func (s *sliceStreamer) Stream(samples [][2]float64) (int, bool) {
	n := copy(samples, s.samples[s.position:])
	s.position += n
	return n, n > 0
}

func (s *sliceStreamer) Err() error {
	return nil
}

func sine(sampleRate beep.SampleRate, frequency, seconds float64) [][2]float64 {
	samples := make([][2]float64, int(float64(sampleRate)*seconds))
	for i := range samples {
		value := 0.5 * math.Sin(2*math.Pi*frequency*float64(i)/float64(sampleRate))
		samples[i] = [2]float64{value, value}
	}

	return samples
}

func zeroCrossingFrequency(samples [][2]float64, sampleRate beep.SampleRate) float64 {
	crossings := 0
	for i := 1; i < len(samples); i++ {
		if (samples[i-1][0] < 0) != (samples[i][0] < 0) {
			crossings++
		}
	}

	return float64(crossings) / 2 / (float64(len(samples)) / float64(sampleRate))
}

func TestStretcherAtNormalSpeedLeavesAudioUnchanged(t *testing.T) {
	sampleRate := beep.SampleRate(44100)
	input := sine(sampleRate, 440, 1)

	stretcher := NewStretcher(sampleRate, &sliceStreamer{samples: input})
	output := readAll(stretcher)

	for i := stretcher.hop; i < len(input)-stretcher.frame; i++ {
		if math.Abs(output[i][0]-input[i][0]) > 1e-9 {
			t.Fatalf("sample %d changed from %v to %v", i, input[i][0], output[i][0])
		}
	}
}

func TestStretcherChangesSpeedButNotPitch(t *testing.T) {
	for _, sampleRate := range []beep.SampleRate{22050, 44100, 48000} {
		for _, tempo := range []float64{0.5, 0.75, 1.1, 1.5, 2.0} {
			input := sine(sampleRate, 220, 3)

			stretcher := NewStretcher(sampleRate, &sliceStreamer{samples: input})
			stretcher.SetTempo(tempo)
			output := readAll(stretcher)

			speedup := float64(len(input)) / float64(len(output))
			if math.Abs(speedup-tempo)/tempo > 0.03 {
				t.Errorf("%d Hz at %.2fx: output is %.3fx shorter", sampleRate, tempo, speedup)
			}

			middle := output[len(output)/10 : len(output)*9/10]
			if pitch := zeroCrossingFrequency(middle, sampleRate); math.Abs(pitch-220) > 3 {
				t.Errorf("%d Hz at %.2fx: pitch is %.1f Hz, want 220 Hz", sampleRate, tempo, pitch)
			}

			largestStep := 0.0
			for i := 1; i < len(middle); i++ {
				largestStep = math.Max(largestStep, math.Abs(middle[i][0]-middle[i-1][0]))
			}
			smoothLimit := 0.5 * 2 * math.Pi * 220 / float64(sampleRate) * 1.3
			if largestStep > smoothLimit {
				t.Errorf("%d Hz at %.2fx: click detected (step %.4f)", sampleRate, tempo, largestStep)
			}
		}
	}
}

func TestStretcherResetStartsCleanAfterSeek(t *testing.T) {
	sampleRate := beep.SampleRate(44100)
	source := &sliceStreamer{samples: sine(sampleRate, 300, 1)}

	stretcher := NewStretcher(sampleRate, source)
	stretcher.SetTempo(1.5)
	readAll(stretcher)

	source.position = 0
	stretcher.Reset()

	if output := readAll(stretcher); len(output) == 0 {
		t.Fatal("no audio after reset")
	}
}
