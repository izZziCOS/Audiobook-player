package audio

import (
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
)

type countingFile struct {
	*os.File
	read atomic.Int64
}

func (c *countingFile) Read(p []byte) (int, error) {
	n, err := c.File.Read(p)
	c.read.Add(int64(n))
	return n, err
}

func (c *countingFile) ReadAt(p []byte, offset int64) (int, error) {
	n, err := c.File.ReadAt(p, offset)
	c.read.Add(int64(n))
	return n, err
}

func openCounting(t *testing.T, name string) *countingFile {
	t.Helper()

	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}

	return &countingFile{File: file}
}

func oldDecoder(t *testing.T, name string) beep.StreamSeekCloser {
	t.Helper()

	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	streamer, _, err := mp3.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { streamer.Close() })

	return streamer
}

func newDecoder(t *testing.T, name string) (beep.StreamSeekCloser, *countingFile) {
	t.Helper()

	file := openCounting(t, name)
	streamer, _, err := decodeMP3(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { streamer.Close() })

	return streamer, file
}

func take(streamer beep.Streamer, count int) [][2]float64 {
	out := make([][2]float64, 0, count)
	chunk := make([][2]float64, 1024)
	for len(out) < count {
		n, ok := streamer.Stream(chunk[:min(len(chunk), count-len(out))])
		out = append(out, chunk[:n]...)
		if !ok {
			break
		}
	}

	return out
}

func TestMP3sAreOpenedWithoutReadingThemWhole(t *testing.T) {
	for _, test := range []struct {
		name     string
		streamed bool
	}{
		{"speech-cbr.mp3", true},
		{"speech-cbr-plain.mp3", true},
		{"speech-vbr-stereo.mp3", true},
		{"long-cbr.mp3", true},
		{"long-vbr.mp3", true},
		{"speech-vbr-no-index.mp3", false},
	} {
		streamer, _ := newDecoder(t, test.name)
		if _, streamed := streamer.(*mp3Stream); streamed != test.streamed {
			t.Errorf("%s: streamed %v, want %v", test.name, streamed, test.streamed)
			continue
		}

		old := oldDecoder(t, test.name)
		tolerance := 0
		if test.name == "speech-cbr-plain.mp3" || test.name == "long-cbr.mp3" {
			tolerance = 1152
		}
		if diff := streamer.Len() - old.Len(); diff < -tolerance || diff > tolerance {
			t.Errorf("%s: length %d, the full scan says %d", test.name, streamer.Len(), old.Len())
		}

		ours, theirs := take(streamer, old.Len()+5000), take(old, old.Len()+5000)
		if len(ours) != len(theirs) {
			t.Errorf("%s: decoded %d samples, the old decoder %d", test.name, len(ours), len(theirs))
		}
		for i := range min(len(ours), len(theirs)) {
			if ours[i] != theirs[i] {
				t.Errorf("%s: sample %d differs", test.name, i)
				break
			}
		}
	}
}

func TestOpeningALongMP3ReadsLittle(t *testing.T) {
	for _, name := range []string{"long-cbr.mp3", "long-vbr.mp3"} {
		streamer, file := newDecoder(t, name)
		size, _ := file.Stat()
		take(streamer, 44100)

		if read := file.read.Load(); read > 64<<10 || read*3 > size.Size() {
			t.Errorf("%s: read %d of %d bytes to start playing", name, read, size.Size())
		}
	}
}

func alignment(ours, theirs [][2]float64, around int) int {
	best, bestScore := 0, math.Inf(1)
	for shift := -around; shift <= around; shift++ {
		score := 0.0
		for i := range 2048 {
			j := i + around + shift
			score += math.Abs(ours[i][0] - theirs[j][0])
		}
		if score < bestScore {
			best, bestScore = shift, score
		}
	}

	return best
}

func TestSeekingAnMP3LandsInTheRightPlace(t *testing.T) {
	for _, test := range []struct {
		name     string
		accuracy int
	}{
		{"long-cbr.mp3", 1152 * 2},
		{"long-vbr.mp3", 44100},
	} {
		file := openCounting(t, test.name)
		streamer, format, err := decodeMP3(file)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { streamer.Close() })
		old := oldDecoder(t, test.name)
		rate := int(format.SampleRate)

		for _, seconds := range []float64{5, 31.3, 55} {
			target := int(seconds * float64(rate))
			before := file.read.Load()
			if err := streamer.Seek(target); err != nil || streamer.Position() != target {
				t.Fatalf("%s: seek to %vs gave position %d, %v", test.name, seconds, streamer.Position(), err)
			}
			ours := take(streamer, 2048)

			around := test.accuracy
			old.Seek(max(target-around, 0))
			theirs := take(old, 2048+2*around)
			if len(ours) < 2048 || len(theirs) < 2048+2*around {
				t.Fatalf("%s: too little audio after seeking to %vs", test.name, seconds)
			}

			shift := alignment(ours, theirs, around)
			if shift < -around+1 || shift > around-1 {
				t.Errorf("%s: at %vs the audio is %d samples away, more than %d", test.name, seconds, shift, around)
			}
			t.Logf("%s: seeking to %vs landed %d samples (%.3fs) off and read %d bytes", test.name, seconds, shift, float64(shift)/float64(rate), file.read.Load()-before)
		}
	}
}
