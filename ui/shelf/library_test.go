package shelf

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
)

func TestAddingAFolderOfBooks(t *testing.T) {
	s, host := newTestShelf(t)
	root, hobbit := testbooks.Library(t)

	s.AddFrom([]string{root})

	titles := shelfTitles(s)
	slices.Sort(titles)
	if want := []string{"Dune", "Dune Messiah", "The Hobbit"}; !slices.Equal(titles, want) {
		t.Fatalf("library holds %v, want %v", titles, want)
	}
	if host.shown == 0 || s.empty.Visible() {
		t.Fatal("the library should be showing its books")
	}

	for _, entry := range s.entries {
		if !strings.HasPrefix(entry.card.Status, "Not started · ") {
			t.Errorf("%s: status %q, want the length of an unstarted book", entry.title, entry.card.Status)
		}
		if entry.card.Thumbnail != nil {
			t.Errorf("%s: unexpected cover art", entry.title)
		}
	}

	hobbitEntry := s.find(hobbit)
	if hobbitEntry == nil || hobbitEntry.card.Author != "2 parts" {
		t.Fatalf("the Hobbit should be one book of 2 parts, got %+v", hobbitEntry)
	}

	saved := pathsOnDisk(t, s)
	if len(saved) != 3 {
		t.Fatalf("saved library %v", saved)
	}

	s.AddFrom([]string{root})
	if len(s.entries) != 3 {
		t.Fatalf("adding the same folder twice gave %d books", len(s.entries))
	}
}

func TestFinishedAndMissingBooks(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	hobbitEntry := s.find(hobbit)
	s.data.Finish(hobbit)

	s.Refresh()
	if hobbitEntry.card.Status != "Finished" || hobbitEntry.card.Progress != 1 {
		t.Fatalf("a finished book shows %.2f, %q", hobbitEntry.card.Progress, hobbitEntry.card.Status)
	}

	missing := filepath.Join(root, "Gone.m4b")
	s.Add([]string{missing})
	if entry := s.find(missing); entry == nil || entry.card.Status != "Not found" {
		t.Fatalf("a missing book should say so, got %+v", entry)
	}

	s.remove(hobbitEntry)
	if s.find(hobbit) != nil || slices.Contains(pathsOnDisk(t, s), hobbit) {
		t.Fatal("removing a book should take it out of the library")
	}
}

func TestOpeningAMissingBook(t *testing.T) {
	s, host := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	if err := os.RemoveAll(hobbit); err != nil {
		t.Fatal(err)
	}
	s.open(s.find(hobbit))

	if len(host.opened) != 1 || host.opened[0] != hobbit {
		t.Fatalf("clicking a book should ask for it to be played, got %v", host.opened)
	}
	if card, _ := s.Card(hobbit); card.Status != "Not found" {
		t.Fatalf("a book that can't be opened should say so, got %q", card.Status)
	}
}
