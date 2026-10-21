package shelf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

func overlayText(s *Shelf, part string) bool {
	return overlayObject(s, func(label *widget.Label) bool { return strings.Contains(label.Text, part) }) != nil
}

func TestOverlappingFoldersDontAddBooksTwice(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	s.AddFrom([]string{filepath.Join(root, "Tolkien")})
	s.AddFrom([]string{hobbit})
	s.Add([]string{filepath.Join(hobbit, "Part 2.flac")})

	link := filepath.Join(t.TempDir(), "Audiobooks")
	if err := os.Symlink(root, link); err == nil {
		s.AddFrom([]string{link})
	} else {
		t.Logf("symbolic links aren't available here: %v", err)
	}

	if len(s.entries) != 3 {
		t.Fatalf("the library holds %v, want each book once", shelfTitles(s))
	}
	if s.duplicatesButton.Visible() || s.window.Canvas().Overlays().Top() != nil {
		t.Fatal("the same files reached another way aren't duplicates to report")
	}
}

func TestCopiesAreSkippedAndReported(t *testing.T) {
	s, _ := newTestShelf(t)
	root, _ := testbooks.Library(t)
	s.AddFrom([]string{root})
	original := filepath.Join(root, "Herbert", "Dune.m4b")

	elsewhere := t.TempDir()
	testbooks.CopyFile(t, "speech-aac-mono.m4b", filepath.Join(elsewhere, "Downloads", "Dune.m4b"))
	testbooks.CopyFile(t, "speech-16bit-mono.wav", filepath.Join(elsewhere, "Downloads", "Other", "Other.wav"))

	s.data.CreateCollection("Sci-fi")
	s.ShowCollection("Sci-fi")
	s.AddFrom([]string{elsewhere})

	if len(s.entries) != 4 || s.Contains(filepath.Join(elsewhere, "Downloads", "Dune.m4b")) {
		t.Fatalf("only the new book should be added, the library holds %v", shelfTitles(s))
	}
	if !overlayText(s, "already in your library from "+original) {
		t.Fatal("the skipped copy should be explained")
	}
	if !s.data.InCollection("Sci-fi", original) || !s.data.InCollection("Sci-fi", filepath.Join(elsewhere, "Downloads", "Other")) {
		t.Fatal("the book already in the library should join the open collection instead of its copy")
	}
}

func TestALonePartJoinsItsFolderBook(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	part := filepath.Join(hobbit, "Part 2.flac")

	s.Add([]string{part})
	s.data.Positions[part] = userdata.Position{Part: part, Position: 0.5, Listened: time.Now().Unix()}
	s.data.CreateCollection("Classics")
	s.data.SetInCollection("Classics", part, true)

	s.AddFrom([]string{root})

	if s.Contains(part) || !s.Contains(hobbit) || len(s.entries) != 3 {
		t.Fatalf("the part should be folded into its book, the library holds %v", shelfTitles(s))
	}
	if position := s.data.Positions[hobbit]; position.Part != part || position.Position != 0.5 {
		t.Fatalf("the book should carry on from the part, got %+v", position)
	}
	if !s.data.InCollection("Classics", hobbit) {
		t.Fatal("the book should take the part's place in its collections")
	}
	for _, path := range pathsOnDisk(t, s) {
		if path == part {
			t.Fatal("the part shouldn't stay in the saved library")
		}
	}
}

func TestReviewingDuplicates(t *testing.T) {
	s, _ := newTestShelf(t)
	root, _ := testbooks.Library(t)
	s.AddFrom([]string{root})

	opened := filepath.Join(t.TempDir(), "Dune.m4b")
	testbooks.CopyFile(t, "speech-aac-mono.m4b", opened)
	s.Add([]string{opened})

	if len(s.entries) != 4 {
		t.Fatal("a copy the user opens should still be added")
	}
	if !s.duplicatesButton.Visible() || s.duplicatesButton.Text != "1 duplicate" {
		t.Fatalf("the duplicate should be pointed out, button %q", s.duplicatesButton.Text)
	}

	test.Tap(s.duplicatesButton)
	if !overlayText(s, opened) || !overlayText(s, filepath.Join(root, "Herbert", "Dune.m4b")) {
		t.Fatal("the review should list both copies")
	}

	removeCopy := overlayObject(s, func(button *widget.Button) bool { return button.Text == "Remove" })
	test.Tap(removeCopy)
	if len(s.entries) != 4 {
		t.Fatal("removing from the review should ask first")
	}
	test.Tap(overlayObject(s, func(button *widget.Button) bool { return button.Text == "Remove" }))

	if len(s.entries) != 3 || s.duplicatesButton.Visible() {
		t.Fatalf("one copy should be left, the library holds %v", shelfTitles(s))
	}
}
