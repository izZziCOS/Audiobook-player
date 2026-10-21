package library

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func boxBytes(kind string, parts ...[]byte) []byte {
	body := bytes.Join(parts, nil)
	return append(binary.BigEndian.AppendUint32(nil, uint32(8+len(body))), append([]byte(kind), body...)...)
}

func fullBoxBytes(kind string, version byte, parts ...[]byte) []byte {
	return boxBytes(kind, append([]byte{version, 0, 0, 0}, bytes.Join(parts, nil)...))
}

func be32(values ...uint32) []byte {
	var out []byte
	for _, value := range values {
		out = binary.BigEndian.AppendUint32(out, value)
	}
	return out
}

func be64(value uint64) []byte {
	return binary.BigEndian.AppendUint64(nil, value)
}

func utf8Sample(text string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(text))), text...)
}

func utf16Sample(text string) []byte {
	encoded := []byte{0xfe, 0xff}
	for _, unit := range utf16.Encode([]rune(text)) {
		encoded = binary.BigEndian.AppendUint16(encoded, unit)
	}
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(encoded))), encoded...)
}

type testTrack struct {
	id       uint32
	handler  string
	chapters []uint32
	version  byte
}

type chapterFile struct {
	samples     [][]byte
	deltas      []uint32
	perChunk    []uint32
	use64       bool
	nero        []Chapter
	movieLast   bool
	references  []uint32
	extraTracks []testTrack
}

func (f chapterFile) build() []byte {
	ftyp := boxBytes("ftyp", []byte("M4B "), be32(0), []byte("M4A mp42isom"))

	var sizes []uint32
	for _, sample := range f.samples {
		sizes = append(sizes, uint32(len(sample)))
	}

	var chunks [][]byte
	remaining := f.samples
	for _, count := range f.perChunk {
		chunks = append(chunks, bytes.Join(remaining[:count], nil))
		remaining = remaining[count:]
	}

	buildMovie := func(dataStart uint64) []byte {
		var offsets []byte
		position := dataStart
		for _, chunk := range chunks {
			if f.use64 {
				offsets = append(offsets, be64(position)...)
			} else {
				offsets = append(offsets, be32(uint32(position))...)
			}
			position += uint64(len(chunk))
		}

		offsetBox := fullBoxBytes("stco", 0, be32(uint32(len(chunks))), offsets)
		if f.use64 {
			offsetBox = fullBoxBytes("co64", 0, be32(uint32(len(chunks))), offsets)
		}

		var times []byte
		for _, delta := range f.deltas {
			times = append(times, be32(1, delta)...)
		}

		var layout []byte
		for i, count := range f.perChunk {
			layout = append(layout, be32(uint32(i+1), count, 1)...)
		}

		track := func(t testTrack, table []byte) []byte {
			header := fullBoxBytes("tkhd", 0, be32(0, 0, t.id))
			if t.version == 1 {
				header = fullBoxBytes("tkhd", 1, be64(0), be64(0), be32(t.id))
			}

			var references []byte
			if len(t.chapters) > 0 {
				references = boxBytes("tref", boxBytes("chap", be32(t.chapters...)))
			}

			return boxBytes("trak",
				header,
				references,
				boxBytes("mdia",
					fullBoxBytes("mdhd", 0, be32(0, 0, 1000, 0)),
					fullBoxBytes("hdlr", 0, be32(0), []byte(t.handler), make([]byte, 13)),
					boxBytes("minf", table),
				),
			)
		}

		table := boxBytes("stbl",
			fullBoxBytes("stts", 0, be32(uint32(len(f.deltas))), times),
			fullBoxBytes("stsc", 0, be32(uint32(len(f.perChunk))), layout),
			fullBoxBytes("stsz", 0, be32(0, uint32(len(sizes))), be32(sizes...)),
			offsetBox,
		)

		parts := [][]byte{
			track(testTrack{id: 1, handler: "soun", chapters: f.references}, nil),
			track(testTrack{id: 2, handler: "text", version: 1}, table),
		}
		for _, extra := range f.extraTracks {
			parts = append(parts, track(extra, nil))
		}

		if f.nero != nil {
			var entries []byte
			for _, chapter := range f.nero {
				entries = append(entries, be64(uint64(chapter.Start*1e7))...)
				entries = append(entries, byte(len(chapter.Title)))
				entries = append(entries, chapter.Title...)
			}
			parts = append(parts, boxBytes("udta", fullBoxBytes("chpl", 1, be32(0), []byte{byte(len(f.nero))}, entries)))
		}

		return boxBytes("moov", parts...)
	}

	media := bytes.Join(chunks, nil)
	if f.movieLast {
		return bytes.Join([][]byte{ftyp, boxBytes("mdat", media), buildMovie(uint64(len(ftyp) + 8))}, nil)
	}

	movieLength := len(buildMovie(0))
	return bytes.Join([][]byte{ftyp, buildMovie(uint64(len(ftyp) + movieLength + 8)), boxBytes("mdat", media)}, nil)
}

func writeChapterFile(t *testing.T, f chapterFile) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "book.m4b")
	writeFile(t, path, f.build())
	return path
}

func sameChapters(got, want []Chapter) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i].Title != want[i].Title || got[i].Start != want[i].Start {
			return false
		}
	}

	return true
}

func standardChapterFile() chapterFile {
	return chapterFile{
		samples: [][]byte{
			utf8Sample("Prologue"),
			utf16Sample("Chapter Één"),
			utf8Sample(""),
			utf8Sample("  The End  "),
		},
		deltas:     []uint32{61500, 120000, 30000, 5000},
		perChunk:   []uint32{2, 1, 1},
		references: []uint32{2},
	}
}

func TestQuickTimeChapters(t *testing.T) {
	want := []Chapter{
		{"Prologue", 0},
		{"Chapter Één", 61.5},
		{"Chapter 3", 181.5},
		{"The End", 211.5},
	}

	for name, f := range map[string]chapterFile{
		"movie first": standardChapterFile(),
		"movie last": func() chapterFile {
			f := standardChapterFile()
			f.movieLast = true
			return f
		}(),
		"64-bit offsets": func() chapterFile {
			f := standardChapterFile()
			f.use64 = true
			return f
		}(),
	} {
		if got := ReadChapters(writeChapterFile(t, f)); !sameChapters(got, want) {
			t.Errorf("%s: got %+v, want %+v", name, got, want)
		}
	}
}

func TestChapterTrackReferencesSkipArtwork(t *testing.T) {
	f := standardChapterFile()
	f.references = []uint32{3, 2}
	f.extraTracks = []testTrack{{id: 3, handler: "vide"}}

	if got := ReadChapters(writeChapterFile(t, f)); len(got) != 4 || got[0].Title != "Prologue" {
		t.Fatalf("the artwork track should be skipped, got %+v", got)
	}
}

func TestNeroChapters(t *testing.T) {
	nero := []Chapter{{"One", 0}, {"Two", 12.5}, {"Three", 3600}}

	f := standardChapterFile()
	f.references = nil
	f.nero = nero
	if got := ReadChapters(writeChapterFile(t, f)); !sameChapters(got, nero) {
		t.Fatalf("without a chapter track: got %+v, want %+v", got, nero)
	}

	f.references = []uint32{2}
	if got := ReadChapters(writeChapterFile(t, f)); len(got) != 4 || got[0].Title != "Prologue" {
		t.Fatalf("the chapter track should be preferred, got %+v", got)
	}
}

func TestChaptersOfRealFile(t *testing.T) {
	want := []Chapter{{"Opening", 0}, {"The Middle", 0.25}, {"Ending", 0.5}}
	if got := ReadChapters("../audio/testdata/speech-chapters.m4b"); !sameChapters(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if got := ReadChapters("../audio/testdata/speech-aac-mono.m4b"); len(got) != 0 {
		t.Fatalf("a file without chapters gave %+v", got)
	}
}

func TestChaptersOfOtherFiles(t *testing.T) {
	directory := t.TempDir()
	createFiles(t, directory, "book.mp3")

	for _, path := range []string{
		filepath.Join(directory, "book.mp3"),
		filepath.Join(directory, "missing.m4b"),
		"../audio/testdata/speech-16bit-mono.flac",
	} {
		if got := ReadChapters(path); got != nil {
			t.Errorf("%s: got %+v", path, got)
		}
	}
}

func TestTruncatedChapterFilesDoNotPanic(t *testing.T) {
	f := standardChapterFile()
	f.nero = []Chapter{{"One", 0}, {"Two", 12.5}}
	data := f.build()
	path := filepath.Join(t.TempDir(), "book.m4b")

	for length := 0; length < len(data); length++ {
		writeFile(t, path, data[:length])
		ReadChapters(path)
	}
}
