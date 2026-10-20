package audio

import (
    "encoding/binary"
    "errors"
    "math"
    "os"
    "path/filepath"
    "testing"

    "github.com/faiface/beep"
    aacpcm "github.com/tphakala/go-aac/pcm"
    "github.com/tphakala/go-m4a/aacm4a"
)

func peak(samples [][2]float64) float64 {
    largest := 0.0
    for _, sample := range samples {
        largest = math.Max(largest, math.Max(math.Abs(sample[0]), math.Abs(sample[1])))
    }

    return largest
}

func TestM4AFromAppleEncoder(t *testing.T) {
    tests := []struct {
        name       string
        sampleRate beep.SampleRate
        channels   int
    }{
        {"speech-aac-mono.m4b", 22050, 1},
        {"speech-aac-stereo.m4a", 44100, 2},
    }

    for _, test := range tests {
        streamer, format := decodeFile(t, filepath.Join("testdata", test.name))

        if format.SampleRate != test.sampleRate || format.NumChannels != test.channels {
            t.Errorf("%s: got %d Hz, %d channels", test.name, format.SampleRate, format.NumChannels)
        }

        seconds := float64(streamer.Len()) / float64(format.SampleRate)
        if math.Abs(seconds-0.6815) > 0.15 {
            t.Errorf("%s: length %.3fs, want about 0.68s", test.name, seconds)
        }

        samples := readAll(streamer)
        if len(samples) != streamer.Len() || streamer.Position() != streamer.Len() {
            t.Errorf("%s: streamed %d samples, position %d, length %d", test.name, len(samples), streamer.Position(), streamer.Len())
        }
        if level := peak(samples); level < 0.3 || level > 1 {
            t.Errorf("%s: peak level %.3f, want speech at a normal level", test.name, level)
        }
        if err := streamer.Err(); err != nil {
            t.Errorf("%s: %v", test.name, err)
        }
    }
}

func TestM4ASeekMatchesLinearDecode(t *testing.T) {
    streamer, _ := decodeFile(t, filepath.Join("testdata", "speech-aac-stereo.m4a"))
    linear := readAll(streamer)

    for _, target := range []int{0, 1000, 1024, 5000, 20000, len(linear) - 100} {
        if err := streamer.Seek(target); err != nil {
            t.Fatalf("seeking to %d: %v", target, err)
        }
        if streamer.Position() != target {
            t.Fatalf("position after seeking to %d is %d", target, streamer.Position())
        }

        buffer := make([][2]float64, min(500, len(linear)-target))
        n, _ := streamer.Stream(buffer)
        if n != len(buffer) {
            t.Fatalf("after seeking to %d: read %d samples, want %d", target, n, len(buffer))
        }

        for i := 0; i < n; i++ {
            if math.Abs(buffer[i][0]-linear[target+i][0]) > 0.01 {
                t.Fatalf("after seeking to %d, sample %d is %v, want %v", target, i, buffer[i][0], linear[target+i][0])
            }
        }
    }
}

func TestM4ATrimsEncoderDelay(t *testing.T) {
    const sampleRate = 44100
    const length = 30000

    source := make([]byte, 0, length*2)
    for i := 0; i < length; i++ {
        value := int16(12000 * math.Sin(2*math.Pi*440*float64(i)/sampleRate))
        source = binary.LittleEndian.AppendUint16(source, uint16(value))
    }

    path := filepath.Join(t.TempDir(), "gapless.m4a")
    file, err := os.Create(path)
    if err != nil {
        t.Fatal(err)
    }
    config := aacpcm.Config{SampleRate: sampleRate, BitDepth: 16, Channels: 1, Bitrate: 128000}
    if err := aacm4a.EncodeInterleaved(file, config, source); err != nil {
        t.Fatal(err)
    }
    file.Close()

    streamer, _ := decodeFile(t, path)
    if streamer.Len() != length {
        t.Fatalf("length %d, want exactly the %d encoded samples", streamer.Len(), length)
    }

    samples := readAll(streamer)
    var cross, energy float64
    for i := 2000; i < length-2000; i++ {
        original := float64(int16(binary.LittleEndian.Uint16(source[i*2:]))) / 32768
        cross += original * samples[i][0]
        energy += original * original
    }
    if ratio := cross / energy; ratio < 0.9 || ratio > 1.1 {
        t.Fatalf("decoded audio does not line up with the source (ratio %.3f), so the encoder delay was not trimmed", ratio)
    }
}

func TestHEAACIsRejectedClearly(t *testing.T) {
    file, err := os.Open(filepath.Join("testdata", "speech-heaac-stereo.m4a"))
    if err != nil {
        t.Fatal(err)
    }
    defer file.Close()

    if _, _, err := Decode(file); !errors.Is(err, errHEAAC) {
        t.Fatalf("got %v, want the HE-AAC error", err)
    }
}

func TestM4ADuration(t *testing.T) {
    seconds, err := Duration(filepath.Join("testdata", "speech-aac-mono.m4b"))
    if err != nil || math.Abs(seconds-0.6815) > 0.15 {
        t.Fatalf("got %v, %v, want about 0.68s", seconds, err)
    }
}
