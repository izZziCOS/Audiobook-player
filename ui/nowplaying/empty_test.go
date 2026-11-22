package nowplaying

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
)

func TestNoBookShowsWaysToFindOne(t *testing.T) {
	p := newTestPlayer(t)

	if p.controls.Visible() || !p.emptyActions.Visible() {
		t.Fatal("with no book, the controls can't do anything, so ways to find a book should show instead")
	}

	shown := 0
	p.ShowLibrary = func() { shown++ }
	row := p.emptyActions.(*fyne.Container).Objects[0].(*fyne.Container)
	library := row.Objects[0].(*widget.Button)
	test.Tap(library)
	if library.Text != "Go to Library" || shown != 1 {
		t.Fatalf("%q should take you to the Library, shown %d times", library.Text, shown)
	}
}

func TestOpeningABookSpinsThePlayButton(t *testing.T) {
	p := newTestPlayer(t)
	var held []func()
	p.async = func(work func()) {
		held = append(held, work)
	}

	folder, _ := testbooks.Book(t)
	p.OpenFolder(folder)
	if !p.playButton.busy || !p.controls.Visible() {
		t.Fatal("while the first book opens, the player should show it's loading")
	}

	for len(held) > 0 {
		work := held[0]
		held = held[1:]
		work()
	}
	if p.playButton.busy || !p.controls.Visible() || p.emptyActions.Visible() {
		t.Fatal("once the book is open the play button should stop spinning and the controls stay")
	}
	if p.author.Visible() != (p.metadata.Artist != "") {
		t.Fatalf("the author line should show exactly when there's an author, got %q", p.metadata.Artist)
	}
}
