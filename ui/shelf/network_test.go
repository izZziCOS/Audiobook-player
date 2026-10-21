package shelf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
	"github.com/izzzicos/audiobook-player/ui/offline"
)

func disconnectedBook(t *testing.T) string {
	t.Helper()

	switch runtime.GOOS {
	case "darwin":
		return "/Volumes/audiobook-player-test-share/Herbert/Dune.m4b"
	case "linux":
		return "/mnt/audiobook-player-test-share/Herbert/Dune.m4b"
	case "windows":
		for letter := 'Z'; letter >= 'D'; letter-- {
			if _, err := os.Stat(string(letter) + `:\`); err != nil {
				return string(letter) + `:\Herbert\Dune.m4b`
			}
		}
	}

	t.Skip("no drive letter or mount point is free for this test")
	return ""
}

func notRespondingFor(t *testing.T, paths ...string) {
	t.Helper()

	original := reachable
	reachable = func(path string, timeout time.Duration) error {
		for _, stalled := range paths {
			if path == stalled {
				return library.ErrNotResponding
			}
		}
		return original(path, timeout)
	}
	t.Cleanup(func() { reachable = original })
}

func dialogText(s *Shelf) string {
	var text []string
	top := s.window.Canvas().Overlays().Top()
	if top == nil {
		return ""
	}
	for _, object := range test.LaidOutObjects(top) {
		if label, ok := object.(*widget.Label); ok {
			text = append(text, label.Text)
		}
	}

	return strings.Join(text, " ")
}

func TestBooksOnADisconnectedDriveSayNotConnected(t *testing.T) {
	s, host := newTestShelf(t)
	path := disconnectedBook(t)

	s.Add([]string{path})
	if card, _ := s.Card(path); card.Status != "Not connected" {
		t.Fatalf("a book on a drive that isn't there should say so, got %q", card.Status)
	}

	s.open(s.find(path))
	if len(host.opened) != 1 || !strings.Contains(dialogText(s), "isn't connected") || !strings.Contains(dialogText(s), "place in the book is kept") {
		t.Fatalf("opening it should explain, got %q", dialogText(s))
	}
}

func TestAStalledServerDoesntOpenTheBook(t *testing.T) {
	s, host := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})
	notRespondingFor(t, hobbit)

	s.open(s.find(hobbit))

	if len(host.opened) != 0 {
		t.Fatal("a book on a server that doesn't respond shouldn't be opened, which could freeze the window")
	}
	if card, _ := s.Card(hobbit); card.Status != "Not connected" || !strings.Contains(dialogText(s), "isn't responding") {
		t.Fatalf("the card says %q and the message %q", card.Status, dialogText(s))
	}
}

func TestMissingBooksAreFoundAgainWhenTheyComeBack(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	away := filepath.Join(t.TempDir(), "away")
	if err := os.Rename(hobbit, away); err != nil {
		t.Fatal(err)
	}
	s.Reload(hobbit)
	if card, _ := s.Card(hobbit); card.Status != "Not found" {
		t.Fatalf("got %q", card.Status)
	}

	if err := os.Rename(away, hobbit); err != nil {
		t.Fatal(err)
	}
	s.RecheckMissing()
	if card, _ := s.Card(hobbit); !strings.HasPrefix(card.Status, "Not started") {
		t.Fatalf("a book that's back should show its progress again, got %q", card.Status)
	}
}

func TestDownloadMenu(t *testing.T) {
	s, _ := newTestShelf(t)
	s.Offline = offline.New(s.window, s.prefs, s.data, s.async)
	s.Offline.OnChanged = s.DownloadChanged
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})
	entry := s.find(hobbit)

	if items := s.downloadItems(entry); len(items) != 0 {
		t.Fatal("books on this computer's own disk don't need downloading")
	}

	entry.network = true
	menu := s.cardMenu(entry)
	menuItem(t, menu, "Download for offline listening").Action()
	if card, _ := s.Card(hobbit); !strings.HasSuffix(card.Status, " · Downloaded") {
		t.Fatalf("the card shows %q", card.Status)
	}

	menuItem(t, s.cardMenu(entry), "Remove download").Action()
	if card, _ := s.Card(hobbit); strings.Contains(card.Status, "Download") {
		t.Fatalf("after removing the download the card shows %q", card.Status)
	}
}
