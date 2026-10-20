package audio

import (
    "encoding/binary"
    "errors"
    "fmt"
    "io"
    "math"
    "os"
    "path/filepath"
    "strings"

    "github.com/faiface/beep"
    "github.com/faiface/beep/mp3"
    "github.com/faiface/beep/wav"
    "github.com/jfreymuth/oggvorbis"
    mewkizflac "github.com/mewkiz/flac"
    aacpcm "github.com/tphakala/go-aac/pcm"
    "github.com/tphakala/go-m4a/aacm4a"
)

var SupportedExtensions = []string{".mp3", ".m4b", ".m4a", ".wav", ".flac", ".ogg", ".oga"}

func IsSupported(name string) bool {
    extension := strings.ToLower(filepath.Ext(name))

    for _, supported := range SupportedExtensions {
        if extension == supported {
            return true
        }
    }

    return false
}

func Decode(file *os.File) (beep.StreamSeekCloser, beep.Format, error) {
    switch extension := strings.ToLower(filepath.Ext(file.Name())); extension {
    case ".mp3":
        return mp3.Decode(file)
    case ".wav":
        return decodeWAV(file)
    case ".flac":
        return decodeFLAC(file)
    case ".ogg", ".oga":
        return decodeVorbis(file)
    case ".m4b", ".m4a":
        return decodeM4A(file)
    default:
        return nil, beep.Format{}, fmt.Errorf("unsupported audio format %q", extension)
    }
}

func Duration(path string) (float64, error) {
    file, err := os.Open(path)
    if err != nil {
        return 0, err
    }

    streamer, format, err := Decode(file)
    if err != nil {
        file.Close()
        return 0, err
    }
    defer streamer.Close()

    return float64(streamer.Len()) / float64(format.SampleRate), nil
}

type scaledStreamer struct {
    beep.StreamSeekCloser
    factor float64
}

func (s scaledStreamer) Stream(samples [][2]float64) (int, bool) {
    n, ok := s.StreamSeekCloser.Stream(samples)
    for i := 0; i < n; i++ {
        samples[i][0] *= s.factor
        samples[i][1] *= s.factor
    }

    return n, ok
}

func decodeWAV(file *os.File) (beep.StreamSeekCloser, beep.Format, error) {
    streamer, format, err := wav.Decode(file)
    if err != nil && strings.Contains(err.Error(), "unsupported") {
        return nil, beep.Format{}, fmt.Errorf(
            "this WAV file's encoding is not supported, only 8, 16 and 24-bit PCM WAV files are (%w)",
            err,
        )
    }
    if err != nil {
        return nil, beep.Format{}, err
    }

    if format.Precision < 2 {
        return streamer, format, nil
    }

    bits := format.Precision * 8
    fullScale := float64(int64(1)<<bits - 1)
    halfScale := float64(int64(1) << (bits - 1))

    return scaledStreamer{streamer, fullScale / halfScale}, format, nil
}

type flacDecoder struct {
    file     io.Closer
    stream   *mewkizflac.Stream
    scale    float64
    buffer   [][2]float64
    position int
    skip     int
    ended    bool
    err      error
}

func decodeFLAC(file *os.File) (beep.StreamSeekCloser, beep.Format, error) {
    stream, err := mewkizflac.NewSeek(file)
    if err != nil {
        return nil, beep.Format{}, fmt.Errorf("flac: %w", err)
    }

    bitsPerSample := int(stream.Info.BitsPerSample)
    format := beep.Format{
        SampleRate:  beep.SampleRate(stream.Info.SampleRate),
        NumChannels: min(int(stream.Info.NChannels), 2),
        Precision:   (bitsPerSample + 7) / 8,
    }

    return &flacDecoder{
        file:   file,
        stream: stream,
        scale:  1 / float64(int64(1)<<(bitsPerSample-1)),
    }, format, nil
}

func (d *flacDecoder) Stream(samples [][2]float64) (n int, ok bool) {
    for n < len(samples) && d.err == nil && !d.ended {
        if len(d.buffer) == 0 {
            d.refill()
            continue
        }

        copied := copy(samples[n:], d.buffer)
        d.buffer = d.buffer[copied:]
        d.position += copied
        n += copied
    }

    return n, n > 0
}

func (d *flacDecoder) refill() {
    frame, err := d.stream.ParseNext()
    if err == io.EOF {
        d.ended = true
        return
    }
    if err != nil {
        d.err = fmt.Errorf("flac: %w", err)
        return
    }

    left := frame.Subframes[0].Samples
    right := left
    if len(frame.Subframes) > 1 {
        right = frame.Subframes[1].Samples
    }

    d.buffer = d.buffer[:0]
    for i := range left {
        d.buffer = append(d.buffer, [2]float64{
            float64(left[i]) * d.scale,
            float64(right[i]) * d.scale,
        })
    }

    skipped := min(d.skip, len(d.buffer))
    d.buffer = d.buffer[skipped:]
    d.skip -= skipped
}

func (d *flacDecoder) Err() error {
    return d.err
}

func (d *flacDecoder) Len() int {
    return int(d.stream.Info.NSamples)
}

func (d *flacDecoder) Position() int {
    return d.position
}

func (d *flacDecoder) Seek(position int) error {
    blockSize := max(int(d.stream.Info.BlockSizeMax), 1)

    var frameStart uint64
    var err error
    for attempt := 0; attempt < 4; attempt++ {
        target := max(position-attempt*blockSize, 0)
        if frameStart, err = d.stream.Seek(uint64(target)); err == nil {
            break
        }
    }
    if err != nil {
        return fmt.Errorf("flac: %w", err)
    }

    d.buffer = d.buffer[:0]
    d.position = position
    d.skip = position - int(frameStart)
    d.ended = false
    d.err = nil

    return nil
}

func (d *flacDecoder) Close() error {
    return d.file.Close()
}

type vorbisDecoder struct {
    file     io.Closer
    reader   *oggvorbis.Reader
    channels int
    right    int
    buffer   []float32
    err      error
}

func decodeVorbis(file *os.File) (beep.StreamSeekCloser, beep.Format, error) {
    reader, err := oggvorbis.NewReader(file)
    if err != nil {
        return nil, beep.Format{}, fmt.Errorf("ogg/vorbis: %w", err)
    }

    channels := reader.Channels()
    right := min(1, channels-1)
    if channels == 3 || channels >= 5 {
        right = 2
    }

    format := beep.Format{
        SampleRate:  beep.SampleRate(reader.SampleRate()),
        NumChannels: min(channels, 2),
        Precision:   2,
    }

    return &vorbisDecoder{
        file:     file,
        reader:   reader,
        channels: channels,
        right:    right,
    }, format, nil
}

func (d *vorbisDecoder) Stream(samples [][2]float64) (n int, ok bool) {
    if d.err != nil {
        return 0, false
    }

    wanted := len(samples) * d.channels
    if cap(d.buffer) < wanted {
        d.buffer = make([]float32, wanted)
    }

    for n < len(samples) {
        read, err := d.reader.Read(d.buffer[:(len(samples)-n)*d.channels])

        for i := 0; i < read; i += d.channels {
            samples[n][0] = float64(d.buffer[i])
            samples[n][1] = float64(d.buffer[i+d.right])
            n++
        }

        if err == io.EOF {
            break
        }
        if err != nil {
            d.err = fmt.Errorf("ogg/vorbis: %w", err)
            break
        }
        if read == 0 {
            break
        }
    }

    return n, n > 0
}

func (d *vorbisDecoder) Err() error {
    return d.err
}

func (d *vorbisDecoder) Len() int {
    return int(d.reader.Length())
}

func (d *vorbisDecoder) Position() int {
    return int(d.reader.Position())
}

func (d *vorbisDecoder) Seek(position int) error {
    return d.reader.SetPosition(int64(position))
}

func (d *vorbisDecoder) Close() error {
    return d.file.Close()
}

const aacFrameSize = 1024

var errHEAAC = errors.New("this file is HE-AAC encoded, which is not supported, only AAC-LC M4B and M4A files are")

type m4aDecoder struct {
    file     io.Closer
    decoder  *aacm4a.Decoder
    channels int
    delay    int
    length   int
    position int
    skip     int
    buffer   []byte
    ended    bool
    err      error
}

func decodeM4A(file *os.File) (beep.StreamSeekCloser, beep.Format, error) {
    decoder, info, err := aacm4a.NewSeekableDecoder(file)
    if err != nil {
        return nil, beep.Format{}, m4aError(err)
    }

    if info.Channels < 1 || info.Channels > 2 {
        return nil, beep.Format{}, fmt.Errorf("m4a: %d audio channels are not supported, only mono and stereo are", info.Channels)
    }

    d := &m4aDecoder{
        file:     file,
        decoder:  decoder,
        channels: info.Channels,
        delay:    int(info.EncoderDelay),
        length:   int(math.Ceil(info.Duration.Seconds() * float64(info.SampleRate))),
    }

    if err := d.Seek(0); err != nil {
        return nil, beep.Format{}, err
    }
    d.read(1)
    if d.err != nil {
        return nil, beep.Format{}, d.err
    }
    if err := d.Seek(0); err != nil {
        return nil, beep.Format{}, err
    }

    format := beep.Format{
        SampleRate:  beep.SampleRate(info.SampleRate),
        NumChannels: info.Channels,
        Precision:   2,
    }

    return d, format, nil
}

func m4aError(err error) error {
    if errors.Is(err, aacpcm.ErrUnsupportedSBR) {
        return errHEAAC
    }

    return fmt.Errorf("m4a: %w", err)
}

func (d *m4aDecoder) read(frames int) int {
    size := frames * d.channels * 2
    if cap(d.buffer) < size {
        d.buffer = make([]byte, size)
    }
    d.buffer = d.buffer[:size]

    read, err := io.ReadFull(d.decoder, d.buffer)
    switch {
    case err == io.EOF || err == io.ErrUnexpectedEOF:
        d.ended = true
    case err != nil:
        d.err = m4aError(err)
    }

    return read / (d.channels * 2)
}

func (d *m4aDecoder) Stream(samples [][2]float64) (n int, ok bool) {
    for d.skip > 0 && !d.ended && d.err == nil {
        d.skip -= d.read(min(d.skip, aacFrameSize))
    }

    for n < len(samples) && d.position < d.length && !d.ended && d.err == nil {
        read := d.read(min(len(samples)-n, d.length-d.position))

        for i := 0; i < read; i++ {
            frame := d.buffer[i*d.channels*2:]
            left := float64(int16(binary.LittleEndian.Uint16(frame))) / 32768
            right := left
            if d.channels == 2 {
                right = float64(int16(binary.LittleEndian.Uint16(frame[2:]))) / 32768
            }

            samples[n+i] = [2]float64{left, right}
        }

        n += read
        d.position += read
    }

    return n, n > 0
}

func (d *m4aDecoder) Err() error {
    return d.err
}

func (d *m4aDecoder) Len() int {
    return d.length
}

func (d *m4aDecoder) Position() int {
    return d.position
}

func (d *m4aDecoder) Seek(position int) error {
    linear := d.delay + position
    if err := d.decoder.SeekSample(linear / aacFrameSize); err != nil {
        return m4aError(err)
    }

    d.position = position
    d.skip = linear % aacFrameSize
    d.ended = false
    d.err = nil

    return nil
}

func (d *m4aDecoder) Close() error {
    return d.file.Close()
}
