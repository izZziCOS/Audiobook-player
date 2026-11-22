package shelf

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
)

// holdWork makes the shelf's background work wait until run is called, so a
// test can see what the window shows in the meantime.
func holdWork(s *Shelf) (run func()) {
	var queue []func()
	s.async = func(work func()) {
		queue = append(queue, work)
	}

	return func() {
		for len(queue) > 0 {
			work := queue[0]
			queue = queue[1:]
			work()
		}
	}
}

func dialogButton(s *Shelf, text string) *widget.Button {
	top := s.window.Canvas().Overlays().Top()
	if top == nil {
		return nil
	}
	for _, object := range test.LaidOutObjects(top) {
		if button, ok := object.(*widget.Button); ok && button.Text == text {
			return button
		}
	}

	return nil
}

func TestOpeningABookShowsOnItsCard(t *testing.T) {
	s, host := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})
	run := holdWork(s)

	s.open(s.find(hobbit))
	if card, _ := s.Card(hobbit); card.Status != openingStatus {
		t.Fatalf("while the book is being reached the card should say so, got %q", card.Status)
	}

	s.open(s.find(hobbit))
	run()

	if len(host.opened) != 1 {
		t.Fatalf("a second click while opening shouldn't open the book again, opened %v", host.opened)
	}
	if card, _ := s.Card(hobbit); card.Status == openingStatus {
		t.Fatal("once the book is open the card should show its progress again")
	}
}

func TestTryingAgainOpensTheBookOnceItsBack(t *testing.T) {
	s, host := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	original := reachable
	stalled := true
	reachable = func(path string, timeout time.Duration) error {
		if stalled {
			return library.ErrNotResponding
		}
		return original(path, timeout)
	}
	t.Cleanup(func() { reachable = original })

	s.open(s.find(hobbit))
	retry := dialogButton(s, "Try again")
	if retry == nil {
		t.Fatalf("the message should offer to try again, got %q", dialogText(s))
	}

	stalled = false
	test.Tap(retry)
	if len(host.opened) != 1 || host.opened[0] != hobbit {
		t.Fatalf("trying again once the server answers should open the book, opened %v", host.opened)
	}
}

func TestAddBooksShowsItsScanning(t *testing.T) {
	s, _ := newTestShelf(t)
	root, _ := testbooks.Library(t)
	run := holdWork(s)

	s.AddFrom([]string{root})
	for _, button := range s.addButtons {
		if !button.Disabled() || button.Text != "Adding books…" {
			t.Fatalf("while folders are scanned Add books should say so, got %q disabled %v", button.Text, button.Disabled())
		}
	}

	run()
	for _, button := range s.addButtons {
		if button.Disabled() || button.Text != "Add books" {
			t.Fatalf("after scanning Add books should be back, got %q disabled %v", button.Text, button.Disabled())
		}
	}
}

func TestAnEmptyScanStillEndsScanning(t *testing.T) {
	s, _ := newTestShelf(t)

	s.AddFrom([]string{t.TempDir()})
	if s.scans != 0 || s.addButtons[0].Disabled() {
		t.Fatal("a folder with no books in it should still leave Add books usable")
	}
}

func TestCardsHaveAMenuButton(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})
	s.window.Resize(fyne.NewSize(600, 800))

	card := s.find(hobbit).card
	more := test.WidgetRenderer(card).(*bookCardRenderer).more
	if !more.Visible() {
		t.Fatal("each card should have a button for its menu, not only a right-click")
	}

	test.Tap(more)
	if top := s.window.Canvas().Overlays().Top(); top == nil {
		t.Fatal("the button should open the book's menu")
	}
}

func TestCardsOpenFromTheKeyboard(t *testing.T) {
	s, host := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	card := s.find(hobbit).card
	s.window.Canvas().Focus(card)
	if !card.focused {
		t.Fatal("a card should take the keyboard focus")
	}

	card.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if len(host.opened) != 1 {
		t.Fatalf("Return on a focused card should open its book, opened %v", host.opened)
	}
}
