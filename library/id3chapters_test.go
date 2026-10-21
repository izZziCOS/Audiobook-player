package library

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func syncsafeBytes(size int) []byte {
	return []byte{byte(size >> 21 & 0x7f), byte(size >> 14 & 0x7f), byte(size >> 7 & 0x7f), byte(size & 0x7f)}
}

func chapterFrame(frame func(string, []byte) []byte, id string, startMs uint32, subframes ...[]byte) []byte {
	data := append([]byte(id), 0)
	data = binary.BigEndian.AppendUint32(data, startMs)
	data = binary.BigEndian.AppendUint32(data, startMs+1000)
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	data = append(data, bytes.Join(subframes, nil)...)

	return frame("CHAP", data)
}

func utf16Title(text string) []byte {
	data := []byte{1, 0xFF, 0xFE}
	for _, unit := range utf16.Encode([]rune(text)) {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}

	return id3Frame("TIT2", append(data, 0, 0))
}

func id3v4Frame(id string, data []byte) []byte {
	frame := append([]byte(id), syncsafeBytes(len(data))...)
	frame = append(frame, 0, 0)

	return append(frame, data...)
}

func id3v4Tag(frames ...[]byte) []byte {
	body := bytes.Join(frames, nil)
	return append(append([]byte{'I', 'D', '3', 4, 0, 0}, syncsafeBytes(len(body))...), body...)
}

func mp3WithTag(t *testing.T, tag []byte) string {
	t.Helper()

	audio, err := os.ReadFile(filepath.Join("..", "audio", "testdata", "speech-cbr.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(audio, []byte("ID3")) {
		audio = audio[10+syncsafe(audio[6:10]):]
	}

	path := filepath.Join(t.TempDir(), "Chapters.mp3")
	writeFile(t, path, append(tag, audio...))
	return path
}

func TestChaptersInMP3s(t *testing.T) {
	path := mp3WithTag(t, id3Tag(
		id3TextFrame("TALB", "The Hobbit"),
		id3Frame("CTOC", []byte("toc\x00\x03\x03chp0\x00chp1\x00chp2\x00")),
		chapterFrame(id3Frame, "chp2", 400, id3TextFrame("TIT2", "Riddles in the Dark")),
		chapterFrame(id3Frame, "chp0", 0, utf16Title("An Unexpected Party – Bilbo")),
		chapterFrame(id3Frame, "chp1", 200),
		id3Frame("APIC", append([]byte{0}, "image/png\x00\x03\x00not really a picture"...)),
	))

	want := []Chapter{
		{Title: "An Unexpected Party – Bilbo", Start: 0},
		{Title: "Chapter 2", Start: 0.2},
		{Title: "Riddles in the Dark", Start: 0.4},
	}
	if got := ReadChapters(path); !sameChapters(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestChaptersInID3Version4(t *testing.T) {
	lengthIndicated := append([]byte{0, 0, 0, 20}, append([]byte{3}, "Kapitel Zwei"...)...)
	path := mp3WithTag(t, id3v4Tag(
		chapterFrame(id3v4Frame, "a", 0, id3v4Frame("TIT2", append([]byte{3}, "Erstes Kapitel"...))),
		chapterFrame(id3v4Frame, "b", 1500, append(append([]byte("TIT2"), syncsafeBytes(len(lengthIndicated))...), append([]byte{0, 1}, lengthIndicated...)...)),
	))

	want := []Chapter{{Title: "Erstes Kapitel", Start: 0}, {Title: "Kapitel Zwei", Start: 1.5}}
	if got := ReadChapters(path); !sameChapters(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestUnsynchronisedTag(t *testing.T) {
	tag := id3Tag(chapterFrame(id3Frame, "a", 0x00FF00, id3TextFrame("TIT2", "Late")))
	body := bytes.ReplaceAll(tag[10:], []byte{0xFF}, []byte{0xFF, 0x00})
	unsynchronised := append([]byte{'I', 'D', '3', 3, 0, 0x80}, syncsafeBytes(len(body))...)

	path := mp3WithTag(t, append(unsynchronised, body...))
	if got := ReadChapters(path); len(got) != 1 || got[0].Title != "Late" || got[0].Start != float64(0x00FF00)/1000 {
		t.Fatalf("got %+v", got)
	}
}

func TestBrokenTagsHaveNoChapters(t *testing.T) {
	for _, tag := range [][]byte{
		[]byte("ID3\x03\x00\x00\x00\x00\x10\x00CHAP"),
		id3Tag(id3Frame("CHAP", []byte("no terminator"))),
		id3Tag(id3Frame("CHAP", []byte("a\x00short"))),
		append([]byte("ID3\x02\x00\x00"), syncsafeBytes(4)...),
	} {
		if got := ReadChapters(mp3WithTag(t, tag)); len(got) != 0 {
			t.Errorf("a broken tag gave %+v", got)
		}
	}
}
