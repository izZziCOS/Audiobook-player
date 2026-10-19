package audio

import (
    "encoding/binary"
    "math"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/faiface/beep"
)

func decodeFile(t *testing.T, path string) (beep.StreamSeekCloser, beep.Format) {
    t.Helper()

    file, err := os.Open(path)
    if err != nil {
        t.Fatal(err)
    }

    streamer, format, err := Decode(file)
    if err != nil {
        file.Close()
        t.Fatalf("decoding %s: %v", path, err)
    }
    t.Cleanup(func() { streamer.Close() })

    return streamer, format
}

func readAll(streamer beep.Streamer) [][2]float64 {
    var samples [][2]float64
    buffer := make([][2]float64, 1000)

    for {
        n, ok := streamer.Stream(buffer)
        samples = append(samples, buffer[:n]...)
        if !ok {
            return samples
        }
    }
}

func writeWAV(t *testing.T, formatType, bits, channels int, values []int32) string {
    t.Helper()

    bytesPerSample := bits / 8
    var data []byte
    for _, value := range values {
        for i := 0; i < bytesPerSample; i++ {
            data = append(data, byte(value>>(8*i)))
        }
    }

    const sampleRate = 8000
    header := make([]byte, 44)
    copy(header[0:], "RIFF")
    binary.LittleEndian.PutUint32(header[4:], uint32(36+len(data)))
    copy(header[8:], "WAVE")
    copy(header[12:], "fmt ")
    binary.LittleEndian.PutUint32(header[16:], 16)
    binary.LittleEndian.PutUint16(header[20:], uint16(formatType))
    binary.LittleEndian.PutUint16(header[22:], uint16(channels))
    binary.LittleEndian.PutUint32(header[24:], sampleRate)
    binary.LittleEndian.PutUint32(header[28:], uint32(sampleRate*channels*bytesPerSample))
    binary.LittleEndian.PutUint16(header[32:], uint16(channels*bytesPerSample))
    binary.LittleEndian.PutUint16(header[34:], uint16(bits))
    copy(header[36:], "data")
    binary.LittleEndian.PutUint32(header[40:], uint32(len(data)))

    path := filepath.Join(t.TempDir(), "test.wav")
    if err := os.WriteFile(path, append(header, data...), 0o644); err != nil {
        t.Fatal(err)
    }

    return path
}

func TestIsSupported(t *testing.T) {
    for name, want := range map[string]bool{
        "book.mp3":  true,
        "BOOK.MP3":  true,
        "part.wav":  true,
        "part.flac": true,
        "part.ogg":  true,
        "part.oga":  true,
        "part.m4b":  false,
        "notes.txt": false,
        "mp3":       false,
    } {
        if got := IsSupported(name); got != want {
            t.Errorf("IsSupported(%q) = %v, want %v", name, got, want)
        }
    }
}

func TestWAVIsDecodedAtFullScale(t *testing.T) {
    tests := []struct {
        bits  int
        value int32
    }{
        {16, 16384},
        {16, -32768},
        {24, 4194304},
        {24, -8388608},
    }

    for _, test := range tests {
        path := writeWAV(t, 1, test.bits, 1, []int32{test.value, test.value})
        streamer, _ := decodeFile(t, path)
        samples := readAll(streamer)

        want := float64(test.value) / float64(int64(1)<<(test.bits-1))
        if len(samples) != 2 || math.Abs(samples[0][0]-want) > 1e-9 || samples[0][0] != samples[0][1] {
            t.Errorf("%d-bit value %d decoded as %v, want %v", test.bits, test.value, samples, want)
        }
    }
}

func TestFloatWAVIsRejectedWithClearMessage(t *testing.T) {
    path := writeWAV(t, 3, 32, 1, []int32{0, 0})

    file, err := os.Open(path)
    if err != nil {
        t.Fatal(err)
    }
    defer file.Close()

    _, _, err = Decode(file)
    if err == nil || !strings.Contains(err.Error(), "not supported") {
        t.Fatalf("expected a not supported error, got %v", err)
    }
}

func TestFLACMatchesWAV(t *testing.T) {
    for _, name := range []string{"speech-16bit-mono", "speech-24bit-stereo"} {
        flacStreamer, flacFormat := decodeFile(t, filepath.Join("testdata", name+".flac"))
        wavStreamer, wavFormat := decodeFile(t, filepath.Join("testdata", name+".wav"))

        if flacFormat.SampleRate != wavFormat.SampleRate {
            t.Fatalf("%s: sample rate %d, want %d", name, flacFormat.SampleRate, wavFormat.SampleRate)
        }

        flacSamples := readAll(flacStreamer)
        wavSamples := readAll(wavStreamer)
        if len(flacSamples) != len(wavSamples) || len(flacSamples) != flacStreamer.Len() {
            t.Fatalf("%s: %d FLAC samples, %d WAV samples, Len %d", name, len(flacSamples), len(wavSamples), flacStreamer.Len())
        }

        for i := range wavSamples {
            if math.Abs(flacSamples[i][0]-wavSamples[i][0]) > 1e-9 || math.Abs(flacSamples[i][1]-wavSamples[i][1]) > 1e-9 {
                t.Fatalf("%s: sample %d is %v in FLAC and %v in WAV", name, i, flacSamples[i], wavSamples[i])
            }
        }
    }
}

func TestFLACSeekIsExactEvenAfterTheEnd(t *testing.T) {
    for _, name := range []string{"speech-16bit-mono", "speech-24bit-stereo"} {
        reference, _ := decodeFile(t, filepath.Join("testdata", name+".flac"))
        expected := readAll(reference)

        streamer, _ := decodeFile(t, filepath.Join("testdata", name+".flac"))
        readAll(streamer)

        buffer := make([][2]float64, 64)
        targets := []int{0, 1, 4095, 4096, len(expected) / 2, len(expected) - 64, len(expected) - 1}
        for _, target := range targets {
            if err := streamer.Seek(target); err != nil {
                t.Fatalf("%s: seek to %d: %v", name, target, err)
            }

            n, _ := streamer.Stream(buffer)
            if n == 0 || n > len(expected)-target {
                t.Fatalf("%s: seek to %d returned %d samples", name, target, n)
            }

            for i := 0; i < n; i++ {
                if buffer[i] != expected[target+i] {
                    t.Fatalf("%s: after seeking to %d, sample %d is wrong", name, target, i)
                }
            }

            if streamer.Position() != target+n {
                t.Fatalf("%s: position %d after seeking to %d and reading %d", name, streamer.Position(), target, n)
            }
        }
    }
}

func TestMonoVorbisDecodesAtTheRightSpeed(t *testing.T) {
    streamer, format := decodeFile(t, filepath.Join("testdata", "hello-mono.ogg"))

    samples := readAll(streamer)
    if len(samples) != streamer.Len() {
        t.Fatalf("streamed %d samples, Len is %d", len(samples), streamer.Len())
    }

    duration := float64(len(samples)) / float64(format.SampleRate)
    if math.Abs(duration-0.4876) > 0.01 {
        t.Fatalf("duration %.3fs, want about 0.488s", duration)
    }

    for _, sample := range samples {
        if sample[0] != sample[1] {
            t.Fatal("mono audio should be identical in both channels")
        }
    }

    target := len(samples) / 2
    if err := streamer.Seek(target); err != nil {
        t.Fatal(err)
    }

    buffer := make([][2]float64, 32)
    n, _ := streamer.Stream(buffer)
    for i := 0; i < n; i++ {
        if math.Abs(buffer[i][0]-samples[target+i][0]) > 1e-6 {
            t.Fatalf("after seeking to %d, sample %d is wrong", target, i)
        }
    }
}
