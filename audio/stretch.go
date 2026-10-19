package audio

import (
    "math"
    "time"

    "github.com/faiface/beep"
)

const (
    stretchFrameDuration     = 40 * time.Millisecond
    stretchToleranceDuration = 12 * time.Millisecond
    stretchCoarseStep        = 4
    stretchReadChunk         = 1024
)

type Stretcher struct {
    source    beep.Streamer
    frame     int
    hop       int
    tolerance int
    window    []float64
    tempo     float64

    input      [][2]float64
    inputStart int
    drained    bool
    started    bool
    finished   bool
    analysis   float64
    previous   int

    overlap    [][2]float64
    pending    [][2]float64
    pendingPos int

    template  []float64
    candidate []float64
    readBuf   [][2]float64
}

func NewStretcher(sampleRate beep.SampleRate, source beep.Streamer) *Stretcher {
    frame := sampleRate.N(stretchFrameDuration)
    frame -= frame % 2
    hop := frame / 2

    window := make([]float64, frame)
    for i := range window {
        window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(frame))
    }

    return &Stretcher{
        source:    source,
        frame:     frame,
        hop:       hop,
        tolerance: sampleRate.N(stretchToleranceDuration),
        window:    window,
        tempo:     1.0,
        overlap:   make([][2]float64, frame),
        template:  make([]float64, hop),
        readBuf:   make([][2]float64, stretchReadChunk),
    }
}

func (s *Stretcher) SetTempo(tempo float64) {
    s.tempo = tempo
}

func (s *Stretcher) Reset() {
    s.input = s.input[:0]
    s.inputStart = 0
    s.drained = false
    s.started = false
    s.finished = false
    s.analysis = 0
    s.previous = 0
    s.pending = s.pending[:0]
    s.pendingPos = 0

    for i := range s.overlap {
        s.overlap[i] = [2]float64{}
    }
}

func (s *Stretcher) Stream(samples [][2]float64) (n int, ok bool) {
    for n < len(samples) {
        if s.pendingPos == len(s.pending) {
            if s.finished {
                break
            }

            s.pending = s.pending[:0]
            s.pendingPos = 0
            s.produce()
            continue
        }

        copied := copy(samples[n:], s.pending[s.pendingPos:])
        s.pendingPos += copied
        n += copied
    }

    return n, n > 0
}

func (s *Stretcher) Err() error {
    return s.source.Err()
}

func (s *Stretcher) produce() {
    nominal := int(math.Round(s.analysis))
    s.fill(nominal + s.tolerance + s.frame)

    if s.drained && nominal >= s.inputEnd() {
        s.pending = append(s.pending, s.overlap[:s.hop]...)
        s.finished = true
        return
    }

    position := nominal
    if s.started && s.tempo != 1.0 {
        position = s.bestPosition(nominal)
    }

    for i := 0; i < s.frame; i++ {
        sample := s.at(position + i)
        weight := s.window[i]
        s.overlap[i][0] += weight * sample[0]
        s.overlap[i][1] += weight * sample[1]
    }

    s.pending = append(s.pending, s.overlap[:s.hop]...)
    copy(s.overlap, s.overlap[s.hop:])
    for i := s.frame - s.hop; i < s.frame; i++ {
        s.overlap[i] = [2]float64{}
    }

    s.started = true
    s.previous = position

    if s.tempo == 1.0 {
        s.analysis = float64(position + s.hop)
    } else {
        s.analysis += s.tempo * float64(s.hop)
    }

    s.discardBefore(min(int(s.analysis)-s.tolerance, position+s.hop))
}

func (s *Stretcher) bestPosition(nominal int) int {
    low := max(nominal-s.tolerance, s.inputStart)
    high := max(nominal+s.tolerance, low)

    templateStart := s.previous + s.hop
    for i := range s.template {
        s.template[i] = s.monoAt(templateStart + i)
    }

    span := high - low + s.hop
    if cap(s.candidate) < span {
        s.candidate = make([]float64, span)
    }
    s.candidate = s.candidate[:span]
    for i := range s.candidate {
        s.candidate[i] = s.monoAt(low + i)
    }

    best := min(max(nominal, low), high)
    bestScore := s.similarity(best-low, 2)

    for offset := 0; offset <= high-low; offset += stretchCoarseStep {
        if score := s.similarity(offset, 2); score > bestScore {
            best, bestScore = low+offset, score
        }
    }

    bestScore = s.similarity(best-low, 1)
    coarseBest := best
    for position := coarseBest - stretchCoarseStep + 1; position < coarseBest+stretchCoarseStep; position++ {
        if position < low || position > high || position == coarseBest {
            continue
        }

        if score := s.similarity(position-low, 1); score > bestScore {
            best, bestScore = position, score
        }
    }

    return best
}

func (s *Stretcher) similarity(offset, stride int) float64 {
    var cross, energy float64

    for i := 0; i < s.hop; i += stride {
        value := s.candidate[offset+i]
        cross += s.template[i] * value
        energy += value * value
    }

    return cross / math.Sqrt(energy+1e-12)
}

func (s *Stretcher) fill(end int) {
    for !s.drained && s.inputEnd() < end {
        n, ok := s.source.Stream(s.readBuf)
        s.input = append(s.input, s.readBuf[:n]...)

        if !ok || n == 0 {
            s.drained = true
        }
    }
}

func (s *Stretcher) discardBefore(position int) {
    drop := position - s.inputStart
    if drop <= 0 {
        return
    }

    drop = min(drop, len(s.input))
    s.input = append(s.input[:0], s.input[drop:]...)
    s.inputStart += drop
}

func (s *Stretcher) inputEnd() int {
    return s.inputStart + len(s.input)
}

func (s *Stretcher) at(position int) [2]float64 {
    index := position - s.inputStart
    if index < 0 || index >= len(s.input) {
        return [2]float64{}
    }

    return s.input[index]
}

func (s *Stretcher) monoAt(position int) float64 {
    sample := s.at(position)
    return (sample[0] + sample[1]) / 2
}
