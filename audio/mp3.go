package audio

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	gomp3 "github.com/hajimehoshi/go-mp3"

	"github.com/izzzicos/audiobook-player/vfs"
)

const (
	probeSize      = 16 << 10
	resyncSize     = 16 << 10
	framesChecked  = 8
	framesBackedUp = 2
	pcmChunk       = 16 << 10
)

var (
	pcmFormat   = beep.Format{NumChannels: 2, Precision: 2}
	errNoIndex  = errors.New("mp3: variable bitrate without an index")
	mpeg1Rates  = [15]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	mpeg2Rates  = [15]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
	sampleRates = [4][3]int{{11025, 12000, 8000}, {}, {22050, 24000, 16000}, {44100, 48000, 32000}}
)

type mp3Header struct {
	version    int
	bitrate    int
	sampleRate int
	padding    int
	mono       bool
}

func parseMP3Header(b []byte) (mp3Header, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return mp3Header{}, false
	}

	version := int(b[1]>>3) & 3
	layer := int(b[1]>>1) & 3
	rateIndex := int(b[2]>>4) & 15
	frequency := int(b[2]>>2) & 3
	if version == 1 || layer != 1 || rateIndex == 0 || rateIndex == 15 || frequency == 3 {
		return mp3Header{}, false
	}

	rates := mpeg2Rates
	if version == 3 {
		rates = mpeg1Rates
	}

	return mp3Header{
		version:    version,
		bitrate:    rates[rateIndex] * 1000,
		sampleRate: sampleRates[version][frequency],
		padding:    int(b[2]>>1) & 1,
		mono:       b[3]>>6 == 3,
	}, true
}

func (h mp3Header) samplesPerFrame() int {
	if h.version == 3 {
		return 1152
	}
	return 576
}

func (h mp3Header) frameSize() int {
	return h.samplesPerFrame()/8*h.bitrate/h.sampleRate + h.padding
}

func (h mp3Header) averageFrameSize() float64 {
	return float64(h.samplesPerFrame()) / 8 * float64(h.bitrate) / float64(h.sampleRate)
}

func (h mp3Header) sideInfoSize() int {
	switch {
	case h.version == 3 && h.mono:
		return 17
	case h.version == 3:
		return 32
	case h.mono:
		return 9
	default:
		return 17
	}
}

func (h mp3Header) matches(other mp3Header) bool {
	return h.version == other.version && h.sampleRate == other.sampleRate
}

type seekPoint struct {
	sample int64
	offset int64
}

type mp3Info struct {
	audioStart int64
	audioEnd   int64
	first      mp3Header
	cbr        bool
	samples    int64
	points     []seekPoint
}

func readAt(file vfs.File, offset int64, size int) []byte {
	buffer := make([]byte, size)
	n, _ := file.ReadAt(buffer, offset)
	return buffer[:n]
}

func id3Size(b []byte) int64 {
	if len(b) < 10 || string(b[:3]) != "ID3" {
		return 0
	}

	size := int64(b[6]&0x7F)<<21 | int64(b[7]&0x7F)<<14 | int64(b[8]&0x7F)<<7 | int64(b[9]&0x7F)
	size += 10
	if b[5]&0x10 != 0 {
		size += 10
	}

	return size
}

func firstFrame(data []byte) (int, mp3Header, bool) {
	for i := 0; i+4 <= len(data); i++ {
		header, ok := parseMP3Header(data[i:])
		if !ok {
			continue
		}
		next := i + header.frameSize()
		if next+4 > len(data) {
			return i, header, true
		}
		if following, ok := parseMP3Header(data[next:]); ok && following.matches(header) {
			return i, header, true
		}
	}

	return 0, mp3Header{}, false
}

func probeMP3(file vfs.File, size int64) (mp3Info, error) {
	start := readAt(file, 0, 10)
	audioStart := id3Size(start)

	data := readAt(file, audioStart, probeSize)
	at, header, found := firstFrame(data)
	if !found {
		return mp3Info{}, errors.New("mp3: no audio frames found")
	}

	info := mp3Info{audioStart: audioStart + int64(at), audioEnd: size, first: header}
	if tail := readAt(file, size-128, 3); string(tail) == "TAG" {
		info.audioEnd = size - 128
	}
	audioBytes := info.audioEnd - info.audioStart
	frame := data[at:]
	spf := int64(header.samplesPerFrame())

	if tag := at + 4 + header.sideInfoSize(); tag+8 <= len(data) && (string(data[tag:tag+4]) == "Xing" || string(data[tag:tag+4]) == "Info") {
		flags := binary.BigEndian.Uint32(data[tag+4:])
		position := tag + 8
		frames, bytes := int64(0), audioBytes
		if flags&1 != 0 && position+4 <= len(data) {
			frames = int64(binary.BigEndian.Uint32(data[position:]))
			position += 4
		}
		if flags&2 != 0 && position+4 <= len(data) {
			if declared := int64(binary.BigEndian.Uint32(data[position:])); declared > 0 {
				bytes = declared
			}
			position += 4
		}
		if frames > 0 {
			info.samples = (frames + 1) * spf
			info.cbr = string(data[tag:tag+4]) == "Info"
			if flags&4 != 0 && position+100 <= len(data) {
				for percent := 0; percent < 100; percent++ {
					info.points = append(info.points, seekPoint{
						sample: info.samples * int64(percent) / 100,
						offset: info.audioStart + bytes*int64(data[position+percent])/256,
					})
				}
			}
			if info.cbr || len(info.points) > 0 {
				return info, nil
			}
		}
	}

	if vbri := at + 36; vbri+26 <= len(data) && string(data[vbri:vbri+4]) == "VBRI" {
		frames := int64(binary.BigEndian.Uint32(data[vbri+14:]))
		entries := int(binary.BigEndian.Uint16(data[vbri+18:]))
		scale := int64(binary.BigEndian.Uint16(data[vbri+20:]))
		entrySize := int(binary.BigEndian.Uint16(data[vbri+22:]))
		perEntry := int64(binary.BigEndian.Uint16(data[vbri+24:]))
		table := vbri + 26
		if frames > 0 && entrySize >= 1 && entrySize <= 4 && table+entries*entrySize <= len(data) {
			info.samples = (frames + 1) * spf
			offset := info.audioStart
			info.points = append(info.points, seekPoint{sample: 0, offset: offset})
			for i := 0; i < entries; i++ {
				value := int64(0)
				for _, b := range data[table+i*entrySize : table+(i+1)*entrySize] {
					value = value<<8 | int64(b)
				}
				offset += value * scale
				info.points = append(info.points, seekPoint{sample: int64(i+1) * perEntry * spf, offset: offset})
			}
			return info, nil
		}
	}

	position := 0
	for checked := 0; checked < framesChecked && position+4 <= len(frame); checked++ {
		next, ok := parseMP3Header(frame[position:])
		if !ok || !next.matches(header) {
			break
		}
		if next.bitrate != header.bitrate {
			return mp3Info{}, errNoIndex
		}
		position += next.frameSize()
	}

	info.cbr = true
	info.samples = int64(math.Round(float64(audioBytes)/header.averageFrameSize())) * spf
	return info, nil
}

type mp3Stream struct {
	file   vfs.File
	info   mp3Info
	length int

	decoder  *gomp3.Decoder
	pcm      []byte
	pending  []byte
	position int
	skip     int
	err      error
}

type streamOnly struct {
	io.Reader
}

func decodeMP3(file vfs.File) (beep.StreamSeekCloser, beep.Format, error) {
	info, statErr := file.Stat()
	if statErr == nil {
		if probed, err := probeMP3(file, info.Size()); err == nil {
			stream := &mp3Stream{file: file, info: probed, length: int(probed.samples), pcm: make([]byte, pcmChunk)}
			if err := stream.start(probed.audioStart, 0, 0); err == nil {
				return stream, beep.Format{SampleRate: beep.SampleRate(probed.first.sampleRate), NumChannels: pcmFormat.NumChannels, Precision: pcmFormat.Precision}, nil
			}
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, beep.Format{}, err
	}
	return mp3.Decode(file)
}

func (s *mp3Stream) start(offset int64, base, skip int) error {
	decoder, err := gomp3.NewDecoder(streamOnly{io.NewSectionReader(s.file, offset, s.info.audioEnd-offset)})
	if err != nil {
		return err
	}

	s.decoder = decoder
	s.pending = nil
	s.position = base + skip
	s.skip = skip
	s.err = nil

	return nil
}

func (s *mp3Stream) Stream(samples [][2]float64) (int, bool) {
	if s.err != nil || s.decoder == nil {
		return 0, false
	}

	filled := 0
	for filled < len(samples) {
		if len(s.pending) < 4 {
			n, err := s.decoder.Read(s.pcm)
			s.pending = s.pcm[:n-n%4]
			if n == 0 {
				if !errors.Is(err, io.EOF) && err != nil {
					s.err = err
				}
				if s.position < s.length {
					s.length = s.position
				}
				return filled, filled > 0
			}
		}

		for len(s.pending) >= 4 && filled < len(samples) {
			sample, _ := pcmFormat.DecodeSigned(s.pending[:4])
			s.pending = s.pending[4:]

			if s.skip > 0 {
				s.skip--
				continue
			}
			samples[filled] = sample
			filled++
			s.position++
		}
	}

	return filled, true
}

func (s *mp3Stream) Err() error {
	return s.err
}

func (s *mp3Stream) Len() int {
	return max(s.length, s.position)
}

func (s *mp3Stream) Position() int {
	return s.position
}

func (s *mp3Stream) Seek(target int) error {
	target = min(max(target, 0), s.Len())
	spf := s.info.first.samplesPerFrame()
	backed := int64(target - framesBackedUp*spf)

	if backed <= 0 {
		return s.start(s.info.audioStart, 0, target)
	}

	var offset, base int64
	if s.info.cbr {
		frame := backed / int64(spf)
		offset = s.info.audioStart + int64(float64(frame)*s.info.first.averageFrameSize())
		base = frame * int64(spf)
	} else {
		point := seekPoint{offset: s.info.audioStart}
		for _, candidate := range s.info.points {
			if candidate.sample <= backed && candidate.offset >= point.offset {
				point = candidate
			}
		}
		offset, base = point.offset, point.sample
	}

	aligned, ok := s.resync(offset)
	if !ok {
		return s.start(s.info.audioStart, 0, target)
	}
	if s.info.cbr {
		base = int64(math.Round(float64(aligned-s.info.audioStart)/s.info.first.averageFrameSize())) * int64(spf)
	}

	return s.start(aligned, int(base), target-int(base))
}

func (s *mp3Stream) resync(offset int64) (int64, bool) {
	data := readAt(s.file, offset, resyncSize)
	for i := 0; i+4 <= len(data); i++ {
		header, ok := parseMP3Header(data[i:])
		if !ok || !header.matches(s.info.first) {
			continue
		}

		confirmed := true
		position := i
		for range 3 {
			next := position + header.frameSize()
			if next+4 > len(data) {
				break
			}
			following, ok := parseMP3Header(data[next:])
			if !ok || !following.matches(s.info.first) {
				confirmed = false
				break
			}
			position, header = next, following
		}
		if confirmed {
			return offset + int64(i), true
		}
	}

	return 0, false
}

func (s *mp3Stream) Close() error {
	return s.file.Close()
}
