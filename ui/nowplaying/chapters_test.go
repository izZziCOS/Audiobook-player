package nowplaying

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
)

func openChapterBook(t *testing.T) *Player {
	t.Helper()

	p := newTestPlayer(t)
	path := filepath.Join(t.TempDir(), "Story.m4b")
	testbooks.CopyFile(t, "speech-chapters.m4b", path)
	p.OpenFile(path)

	if p.book == nil {
		t.Fatal("the book did not open")
	}

	return p
}

func TestChaptersInThePlayer(t *testing.T) {
	p := openChapterBook(t)

	if !p.hasChapters() || len(p.chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %+v", p.chapters)
	}
	if !p.previousButton.Visible() || !p.nextButton.Visible() || !p.partsButton.Visible() || p.partsButton.Text != "Chapters" {
		t.Fatal("a book with chapters should show the chapter buttons")
	}

	subtitle := func() string {
		return p.subtitle.String()
	}
	at := func(want float64) bool {
		return math.Abs(p.positionSeconds()-want) < 0.01
	}

	if subtitle() != "Opening · 1 of 3" {
		t.Fatalf("subtitle %q", subtitle())
	}

	p.next()
	if !at(0.25) || subtitle() != "The Middle · 2 of 3" {
		t.Fatalf("after next: %.3fs, %q", p.positionSeconds(), subtitle())
	}

	p.next()
	p.next()
	if !at(0.5) || subtitle() != "Ending · 3 of 3" {
		t.Fatalf("next on the last chapter should stay there: %.3fs, %q", p.positionSeconds(), subtitle())
	}

	p.previous()
	if !at(0.25) {
		t.Fatalf("previous should go back a chapter, at %.3fs", p.positionSeconds())
	}

	p.previous()
	p.previous()
	if !at(0) || subtitle() != "Opening · 1 of 3" {
		t.Fatalf("previous on the first chapter should go to its start: %.3fs, %q", p.positionSeconds(), subtitle())
	}

	if err := p.audio.SeekTo(0.4); err != nil {
		t.Fatal(err)
	}
	p.refreshProgress()
	if subtitle() != "The Middle · 2 of 3" {
		t.Fatalf("the subtitle should follow playback, got %q", subtitle())
	}

	p.showParts()
}

func TestBooksWithoutChaptersKeepParts(t *testing.T) {
	p := newTestPlayer(t)
	folder, _ := testbooks.Book(t)
	p.OpenFolder(folder)

	if p.hasChapters() || p.partsButton.Text != "Parts" || !p.partsButton.Visible() {
		t.Fatal("a multi-part book without chapters should list its parts")
	}

	p.next()
	if p.book.Index != 1 {
		t.Fatalf("next should go to the next part, got part %d", p.book.Index+1)
	}

	p.showParts()
}

func TestPreviousChapterStart(t *testing.T) {
	chapters := []library.Chapter{
		{Title: "One", Start: 0},
		{Title: "Two", Start: 100},
		{Title: "Three", Start: 200},
	}

	tests := []struct {
		position        float64
		hasPreviousPart bool
		want            float64
		ok              bool
	}{
		{150, false, 100, true},
		{101, false, 0, true},
		{99.97, false, 0, true},
		{2, false, 0, true},
		{2, true, 0, false},
		{10, true, 0, true},
		{250, false, 200, true},
	}

	for _, test := range tests {
		got, ok := previousChapterStart(chapters, test.position, test.hasPreviousPart)
		if got != test.want || ok != test.ok {
			t.Errorf("previous from %v (previous part %v) = %v, %v, want %v, %v",
				test.position, test.hasPreviousPart, got, ok, test.want, test.ok)
		}
	}
}

func mp3WithChapters(t *testing.T) string {
	t.Helper()

	frame := func(id string, data []byte) []byte {
		out := binary.BigEndian.AppendUint32([]byte(id), uint32(len(data)))
		return append(append(out, 0, 0), data...)
	}
	chapter := func(id string, startMs uint32, title string) []byte {
		data := append([]byte(id), 0)
		data = binary.BigEndian.AppendUint32(data, startMs)
		data = binary.BigEndian.AppendUint32(data, startMs+100)
		data = append(data, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
		return frame("CHAP", append(data, frame("TIT2", append([]byte{3}, title...))...))
	}

	body := append(chapter("a", 0, "Opening"), chapter("b", 300, "Second thoughts")...)
	size := len(body)
	tag := append([]byte{'I', 'D', '3', 3, 0, 0, byte(size >> 21 & 0x7f), byte(size >> 14 & 0x7f), byte(size >> 7 & 0x7f), byte(size & 0x7f)}, body...)

	audio, err := os.ReadFile(filepath.Join("..", "..", "audio", "testdata", "speech-cbr.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if string(audio[:3]) == "ID3" {
		skip := int(audio[6]&0x7f)<<21 | int(audio[7]&0x7f)<<14 | int(audio[8]&0x7f)<<7 | int(audio[9]&0x7f)
		audio = audio[10+skip:]
	}

	path := filepath.Join(t.TempDir(), "Story.mp3")
	if err := os.WriteFile(path, append(tag, audio...), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChaptersInAnMP3Book(t *testing.T) {
	p := newTestPlayer(t)
	p.OpenFile(mp3WithChapters(t))

	if !p.hasChapters() || len(p.chapters) != 2 || p.chapters[1].Title != "Second thoughts" || p.partsButton.Text != "Chapters" {
		t.Fatalf("an MP3 with chapters should show them, got %+v", p.chapters)
	}

	p.next()
	if !closeTo(p.positionSeconds(), 0.3) || p.currentChapter() != 1 {
		t.Fatalf("next should go to the second chapter, at %v", p.positionSeconds())
	}
	if !strings.HasPrefix(p.subtitle.String(), "Second thoughts") {
		t.Fatalf("the subtitle should name the chapter, got %q", p.subtitle.String())
	}
}
