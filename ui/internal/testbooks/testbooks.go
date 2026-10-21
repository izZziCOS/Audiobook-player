package testbooks

import (
	"bytes"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func sampleFolder() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "audio", "testdata")
}

func CopyFile(t *testing.T, source, destination string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(sampleFolder(), source))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func Library(t *testing.T) (root, hobbit string) {
	t.Helper()

	root = t.TempDir()
	hobbit = filepath.Join(root, "Tolkien", "The_Hobbit")
	CopyFile(t, "speech-16bit-mono.wav", filepath.Join(hobbit, "Part 1.wav"))
	CopyFile(t, "speech-24bit-stereo.flac", filepath.Join(hobbit, "Part 2.flac"))
	CopyFile(t, "speech-aac-mono.m4b", filepath.Join(root, "Herbert", "Dune.m4b"))
	CopyFile(t, "speech-aac-mono.m4b", filepath.Join(root, "Herbert", "Dune Messiah.m4b"))

	return root, hobbit
}

func Book(t *testing.T) (folder string, parts []string) {
	t.Helper()

	folder = filepath.Join(t.TempDir(), "The_Hobbit")
	sources := map[string]string{
		"Part 1.wav":  "speech-16bit-mono.wav",
		"Part 2.flac": "speech-24bit-stereo.flac",
		"Part 3.ogg":  "hello-mono.ogg",
	}

	for _, name := range []string{"Part 1.wav", "Part 2.flac", "Part 3.ogg"} {
		path := filepath.Join(folder, name)
		CopyFile(t, sources[name], path)
		parts = append(parts, path)
	}

	return folder, parts
}

func Backdate(t *testing.T, root string) {
	t.Helper()

	old := time.Now().Add(-time.Hour)
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, old, old)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func WriteCover(t *testing.T, path string) {
	t.Helper()

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewGray(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
