package nowplaying

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

type slowServer struct {
	waiting []func()
}

func (s *slowServer) async(work func()) {
	s.waiting = append(s.waiting, work)
}

func (s *slowServer) answer(t *testing.T) {
	t.Helper()

	if len(s.waiting) == 0 {
		t.Fatal("nothing was waiting for the server")
	}
	work := s.waiting[0]
	s.waiting = s.waiting[1:]
	work()
}

func newSlowPlayer(t *testing.T) (*Player, *slowServer) {
	t.Helper()

	app := test.NewApp()
	t.Cleanup(app.Quit)

	server := &slowServer{}
	window := app.NewWindow("Audiobook Player")
	p := New(window, app.Preferences(), userdata.New(userdata.FileIn(t.TempDir())), &kit.Picker{}, server.async)
	testbooks.ExpectFilesClosed(t)
	t.Cleanup(p.Close)
	window.SetContent(p.Content())

	return p, server
}

func TestOpeningASlowBookDoesntBlock(t *testing.T) {
	p, server := newSlowPlayer(t)
	folder, _ := testbooks.Book(t)
	book, _ := library.OpenFolder(folder)

	p.OpenBookAndPlay(book)

	if !p.Opening() || p.Book() != nil || p.subtitle.String() != openingText {
		t.Fatalf("while the book opens the player should say so, subtitle %q", p.subtitle.String())
	}

	p.TogglePlay()
	p.TogglePlay()
	server.answer(t)
	for len(server.waiting) > 0 {
		server.answer(t)
	}

	if p.Opening() || p.Book() != book || !p.Playing() {
		t.Fatalf("once open the book should play, opening %v, playing %v", p.Opening(), p.Playing())
	}
}

func TestPausingWhileOpeningIsRemembered(t *testing.T) {
	p, server := newSlowPlayer(t)
	folder, _ := testbooks.Book(t)
	book, _ := library.OpenFolder(folder)

	p.OpenBookAndPlay(book)
	p.TogglePlay()
	server.answer(t)

	if p.Playing() {
		t.Fatal("pausing while the book opened should keep it paused")
	}
}

func TestANewerChoiceWins(t *testing.T) {
	p, server := newSlowPlayer(t)
	first, _ := testbooks.Book(t)
	second, _ := testbooks.Book(t)
	firstBook, _ := library.OpenFolder(first)
	secondBook, _ := library.OpenFolder(second)

	p.OpenBook(firstBook)
	p.OpenBook(secondBook)
	server.answer(t)
	server.answer(t)
	for len(server.waiting) > 0 {
		server.answer(t)
	}

	if p.Book() != secondBook {
		t.Fatalf("the book chosen last should be open, got %+v", p.Book())
	}
}

func TestTheNextPartIsReadyBeforeItsNeeded(t *testing.T) {
	p, server := newSlowPlayer(t)
	folder, parts := testbooks.Book(t)
	book, _ := library.OpenFolder(folder)

	p.OpenBook(book)
	server.answer(t)
	for len(server.waiting) > 0 {
		server.answer(t)
	}

	p.TogglePlay()
	p.audio.SeekTo(p.durationSeconds() - 1)
	p.prefetchNextPart()
	if len(server.waiting) != 1 {
		t.Fatalf("near the end of a part the next one should start opening, %d jobs waiting", len(server.waiting))
	}
	server.answer(t)
	if p.prefetched == nil || p.prefetched.path != parts[1] {
		t.Fatal("the next part should be ready")
	}

	p.handlePartEnd()
	if len(server.waiting) != 0 || p.Opening() || p.Book().Index != 1 || !p.Playing() {
		t.Fatalf("moving on should be instant, opening %v, part %d", p.Opening(), p.Book().Index+1)
	}
}
