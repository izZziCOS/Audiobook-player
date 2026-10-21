package audio

import (
	"sync"
	"time"

	"github.com/faiface/beep"
)

const (
	bufferAhead   = 20 * time.Second
	bufferChunk   = 4096
	firstFillWait = 500 * time.Millisecond
	waitingAfter  = time.Second
	closeWait     = 250 * time.Millisecond
)

type Buffered struct {
	source beep.StreamSeekCloser
	length int
	ahead  int

	mu         sync.Mutex
	wake       *sync.Cond
	samples    [][2]float64
	position   int
	generation int
	seekTo     int
	seeking    bool
	ended      bool
	err        error
	closed     bool
	dryFrom    time.Time
	finished   chan struct{}
}

func NewBuffered(source beep.StreamSeekCloser, sampleRate beep.SampleRate) *Buffered {
	b := &Buffered{
		source:   source,
		length:   source.Len(),
		ahead:    sampleRate.N(bufferAhead),
		position: source.Position(),
		finished: make(chan struct{}),
	}
	b.wake = sync.NewCond(&b.mu)

	go b.fill()
	b.waitForFirstFill()

	return b
}

func (b *Buffered) waitForFirstFill() {
	deadline := time.Now().Add(firstFillWait)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		ready := len(b.samples) > 0 || b.ended
		b.mu.Unlock()
		if ready {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (b *Buffered) fill() {
	chunk := make([][2]float64, bufferChunk)

	for {
		b.mu.Lock()
		for !b.closed && !b.seeking && (b.ended || len(b.samples) >= b.ahead) {
			b.wake.Wait()
		}
		if b.closed {
			b.mu.Unlock()
			b.source.Close()
			close(b.finished)
			return
		}
		generation := b.generation
		seeking, target := b.seeking, b.seekTo
		b.seeking = false
		b.mu.Unlock()

		if seeking {
			if err := b.source.Seek(target); err != nil {
				b.finish(generation, err)
				continue
			}
		}

		n, ok := b.source.Stream(chunk)
		err := b.source.Err()

		b.mu.Lock()
		if generation == b.generation {
			b.samples = append(b.samples, chunk[:n]...)
			if !ok {
				b.ended = true
				b.err = err
			}
			b.wake.Broadcast()
		}
		b.mu.Unlock()
	}
}

func (b *Buffered) finish(generation int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if generation == b.generation {
		b.ended = true
		b.err = err
	}
}

func (b *Buffered) Stream(samples [][2]float64) (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := copy(samples, b.samples)
	b.samples = b.samples[n:]
	b.position += n
	b.wake.Broadcast()

	if n == len(samples) {
		b.dryFrom = time.Time{}
		return n, true
	}
	if b.ended {
		return n, n > 0
	}

	if b.dryFrom.IsZero() {
		b.dryFrom = time.Now()
	}
	clear(samples[n:])
	return len(samples), true
}

func (b *Buffered) Waiting() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return !b.dryFrom.IsZero() && time.Since(b.dryFrom) >= waitingAfter
}

func (b *Buffered) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.err
}

func (b *Buffered) Len() int {
	return b.length
}

func (b *Buffered) Position() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.position
}

func (b *Buffered) Seek(position int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.generation++
	b.samples = b.samples[:0]
	b.position = position
	b.seekTo = position
	b.seeking = true
	b.ended = false
	b.err = nil
	b.dryFrom = time.Time{}
	b.wake.Broadcast()

	return nil
}

func (b *Buffered) Close() error {
	b.mu.Lock()
	b.closed = true
	b.wake.Broadcast()
	b.mu.Unlock()

	select {
	case <-b.finished:
	case <-time.After(closeWait):
	}

	return nil
}
