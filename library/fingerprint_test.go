package library

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFingerprintFollowsAMovedBook(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "E", "Audiobooks", "The Hobbit")
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(original, "Part 1.mp3"), []byte("first part"))
	writeFile(t, filepath.Join(original, "Part 2.mp3"), []byte("second"))

	before, err := FingerprintOf(original)
	if err != nil || len(before.Parts) != 2 || before.Name != "The Hobbit" {
		t.Fatalf("got %+v, %v", before, err)
	}

	moved := filepath.Join(root, "F", "Books", "The Hobbit")
	os.MkdirAll(filepath.Dir(moved), 0o755)
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}

	after, err := FingerprintOf(moved)
	if err != nil || !before.Matches(after) {
		t.Fatalf("a moved book should match: %+v vs %+v, %v", before, after, err)
	}

	writeFile(t, filepath.Join(moved, "Part 3.mp3"), []byte("third"))
	grown, _ := FingerprintOf(moved)
	if !before.Matches(grown) {
		t.Fatal("a book that gained a part should still match")
	}

	renamed := filepath.Join(root, "F", "Books", "Hobbit (Unabridged)")
	os.Rename(moved, renamed)
	os.Remove(filepath.Join(renamed, "Part 3.mp3"))
	if after, _ := FingerprintOf(renamed); !before.Matches(after) {
		t.Fatal("a renamed folder with the same parts should match")
	}

	writeFile(t, filepath.Join(renamed, "Part 2.mp3"), []byte("re-encoded and longer"))
	if after, _ := FingerprintOf(renamed); before.Matches(after) {
		t.Fatal("a part with a different size is a different book")
	}

	if (Fingerprint{Name: "Empty"}).Matches(Fingerprint{Name: "Empty"}) {
		t.Fatal("an empty fingerprint should never match")
	}
}

func TestFingerprintOfSingleFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Dune.m4b")
	writeFile(t, path, []byte("audio"))

	fingerprint, err := FingerprintOf(path)
	if err != nil || fingerprint.Name != "Dune.m4b" || len(fingerprint.Parts) != 1 || fingerprint.Parts[0].Size != 5 {
		t.Fatalf("got %+v, %v", fingerprint, err)
	}

	other := filepath.Join(directory, "Other.m4b")
	writeFile(t, other, []byte("audio"))
	if otherPrint, _ := FingerprintOf(other); fingerprint.Matches(otherPrint) {
		t.Fatal("two different files of the same size should not match")
	}
}

func TestWindowsDriveLetters(t *testing.T) {
	if got := canonical(`e:\Audiobooks\Dune.m4b`, true); got != `E:\Audiobooks\Dune.m4b` {
		t.Errorf("canonical = %q", got)
	}
	if got := canonical("/books/e:/odd", true); got != "/books/e:/odd" {
		t.Errorf("only a leading drive letter should change, got %q", got)
	}
	if got := canonical(`e:\Audiobooks`, false); got != `e:\Audiobooks` {
		t.Errorf("other systems should be left alone, got %q", got)
	}

	drives := otherDrives(`E:\Audiobooks\Dune.m4b`, true)
	if len(drives) != 23 || slices.Contains(drives, `E:\Audiobooks\Dune.m4b`) || drives[0] != `C:\Audiobooks\Dune.m4b` {
		t.Fatalf("other drives %v", drives)
	}
	if otherDrives("/Volumes/Books/Dune.m4b", false) != nil {
		t.Fatal("there are no drive letters outside Windows")
	}
}

func TestWithin(t *testing.T) {
	root := filepath.Join("books", "library")

	if relative, ok := Within(filepath.Join(root, "Herbert", "Dune.m4b"), root); !ok || relative != filepath.Join("Herbert", "Dune.m4b") {
		t.Errorf("got %q, %v", relative, ok)
	}
	for _, outside := range []string{root, filepath.Join("books", "other"), filepath.Join("books", "library2", "x")} {
		if _, ok := Within(outside, root); ok {
			t.Errorf("%q should not be inside %q", outside, root)
		}
	}
}

func TestSameContents(t *testing.T) {
	hobbit := Fingerprint{Name: "The_Hobbit", Parts: []PartPrint{{"1.mp3", 10}, {"2.mp3", 20}}}
	copied := Fingerprint{Name: "Hobbit (copy)", Parts: []PartPrint{{"2.mp3", 20}, {"1.mp3", 10}}}
	trimmed := Fingerprint{Name: "The_Hobbit", Parts: []PartPrint{{"1.mp3", 10}}}
	resized := Fingerprint{Name: "The_Hobbit", Parts: []PartPrint{{"1.mp3", 10}, {"2.mp3", 21}}}

	if !hobbit.SameContents(copied) {
		t.Fatal("a renamed copy with the same files should match")
	}
	if hobbit.SameContents(trimmed) || hobbit.SameContents(resized) {
		t.Fatal("books with different files shouldn't match")
	}
	if (Fingerprint{}).SameContents(Fingerprint{}) {
		t.Fatal("empty fingerprints never match")
	}
}
