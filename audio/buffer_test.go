package audio

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/faiface/beep"
)

type fakeSource struct {
	mu       sync.Mutex
	length   int
	position int
	gate     chan struct{}
	failAt   int
	err      error
	closed   chan struct{}
}

func newFakeSource(length int) *fakeSource {
	return &fakeSource{length: length, failAt: -1, closed: make(chan struct{})}
}

func (f *fakeSource) stall() {
	f.mu.Lock()
	f.gate = make(chan struct{})
	f.mu.Unlock()
}

func (f *fakeSource) resume() {
	f.mu.Lock()
	gate := f.gate
	f.gate = nil
	f.mu.Unlock()
	if gate != nil {
		close(gate)
	}
}

func (f *fakeSource) Stream(samples [][2]float64) (int, bool) {
	f.mu.Lock()
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0
	for n < len(samples) && f.position < f.length {
		if f.position == f.failAt {
			f.err = errors.New("the server went away")
			return n, false
		}
		samples[n] = [2]float64{float64(f.position), -float64(f.position)}
		f.position++
		n++
	}

	return n, n > 0
}

func (f *fakeSource) Err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

func (f *fakeSource) Len() int {
	return f.length
}

func (f *fakeSource) Position() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.position
}

func (f *fakeSource) Seek(position int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.position = position
	return nil
}

func (f *fakeSource) Close() error {
	close(f.closed)
	return nil
}

const testRate = beep.SampleRate(1000)

func drainUntilDry(b *Buffered) {
	chunk := make([][2]float64, 997)
	still := 0
	for still < 20 {
		before := b.Position()
		b.Stream(chunk)
		if b.Position() == before {
			still++
			time.Sleep(time.Millisecond)
		} else {
			still = 0
		}
	}
}

func drain(t *testing.T, b *Buffered, want int) []float64 {
	t.Helper()

	var values []float64
	chunk := make([][2]float64, 97)
	deadline := time.Now().Add(5 * time.Second)
	for len(values) < want && time.Now().Before(deadline) {
		before := b.Position()
		n, ok := b.Stream(chunk)
		for i := 0; i < b.Position()-before; i++ {
			values = append(values, chunk[i][0])
		}
		if !ok {
			break
		}
		_ = n
		time.Sleep(time.Millisecond)
	}

	return values
}

func TestBufferedPlaysEverySampleInOrder(t *testing.T) {
	source := newFakeSource(50_000)
	b := NewBuffered(source, testRate)
	defer b.Close()

	values := drain(t, b, 50_000)
	if len(values) != 50_000 {
		t.Fatalf("got %d samples, want 50000", len(values))
	}
	for i, value := range values {
		if value != float64(i) {
			t.Fatalf("sample %d is %v", i, value)
		}
	}

	if n, ok := b.Stream(make([][2]float64, 10)); n != 0 || ok {
		t.Fatalf("after the end Stream gave %d, %v", n, ok)
	}
}

func TestBufferedSeeksWithoutWaiting(t *testing.T) {
	source := newFakeSource(100_000)
	b := NewBuffered(source, testRate)
	defer b.Close()

	b.Seek(70_000)
	if b.Position() != 70_000 {
		t.Fatalf("position %d straight after seeking", b.Position())
	}

	values := drain(t, b, 10)
	if len(values) < 10 || values[0] != 70_000 || values[9] != 70_009 {
		t.Fatalf("after seeking got %v", values[:min(len(values), 10)])
	}
}

func TestAStalledSourceNeverBlocksPlayback(t *testing.T) {
	source := newFakeSource(1_000_000)
	b := NewBuffered(source, testRate)
	defer b.Close()

	source.stall()
	drainUntilDry(b)

	chunk := make([][2]float64, 100)
	chunk[0] = [2]float64{1, 1}
	started := time.Now()
	position := b.Position()
	n, ok := b.Stream(chunk)
	if time.Since(started) > 100*time.Millisecond || n != len(chunk) || !ok {
		t.Fatalf("a stalled source should give silence at once, took %v", time.Since(started))
	}
	if chunk[0] != [2]float64{} || b.Position() != position {
		t.Fatal("silence while waiting shouldn't move the position")
	}

	time.Sleep(waitingAfter + 50*time.Millisecond)
	b.Stream(chunk)
	if !b.Waiting() {
		t.Fatal("a long wait should be reported")
	}

	b.Seek(10)
	if b.Position() != 10 || b.Waiting() {
		t.Fatal("seeking while the source is stalled shouldn't wait for it")
	}

	source.resume()
	if values := drain(t, b, 5); len(values) < 5 || values[0] != 10 {
		t.Fatalf("after the source came back got %v", values)
	}
}

func TestBufferedReportsReadErrors(t *testing.T) {
	source := newFakeSource(10_000)
	source.failAt = 6_000
	b := NewBuffered(source, testRate)
	defer b.Close()

	values := drain(t, b, 10_000)
	if len(values) != 6_000 || b.Err() == nil {
		t.Fatalf("got %d samples and error %v, want playback to stop at the error", len(values), b.Err())
	}
}

func TestClosingWhileStalledDoesntWait(t *testing.T) {
	source := newFakeSource(1_000_000)
	b := NewBuffered(source, testRate)
	source.stall()
	drainUntilDry(b)

	started := time.Now()
	b.Close()
	if time.Since(started) > closeWait+200*time.Millisecond {
		t.Fatalf("closing shouldn't wait long for a stalled source, took %v", time.Since(started))
	}

	source.resume()
	select {
	case <-source.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("the source should be closed once it answers")
	}
}

func TestBufferedMatchesTheDecoder(t *testing.T) {
	for _, name := range []string{"speech-16bit-mono.wav", "speech-24bit-stereo.flac", "hello-mono.ogg", "speech-aac-mono.m4b", "speech-16bit-mono.flac"} {
		direct := decodeAll(t, name, false)
		buffered := decodeAll(t, name, true)
		if len(direct) == 0 || len(direct) != len(buffered) {
			t.Fatalf("%s: %d samples directly, %d through the buffer", name, len(direct), len(buffered))
		}
		for i := range direct {
			if direct[i] != buffered[i] {
				t.Fatalf("%s: sample %d differs", name, i)
			}
		}
	}
}

func decodeAll(t *testing.T, name string, buffered bool) [][2]float64 {
	t.Helper()

	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	streamer, format, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if buffered {
		streamer = NewBuffered(streamer, format.SampleRate)
	}
	defer streamer.Close()

	var all [][2]float64
	chunk := make([][2]float64, 1024)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		before := streamer.Position()
		_, ok := streamer.Stream(chunk)
		all = append(all, chunk[:streamer.Position()-before]...)
		if !ok {
			break
		}
	}

	return all
}

func TestClosingClosesTheFile(t *testing.T) {
	source := newFakeSource(1_000)
	b := NewBuffered(source, testRate)

	b.Close()
	select {
	case <-source.closed:
	default:
		t.Fatal("when Close returns, the file should be closed")
	}
}
