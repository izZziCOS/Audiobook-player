package shelf

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/store"
	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

type testHost struct {
	shown  int
	opened []string
}

func newTestShelf(t *testing.T) (*Shelf, *testHost) {
	t.Helper()

	app := test.NewApp()
	t.Cleanup(app.Quit)
	testbooks.ExpectFilesClosed(t)

	window := app.NewWindow("Library")
	data := userdata.New(userdata.FileIn(t.TempDir()))
	s := New(window, app.Preferences(), data, &kit.Picker{}, func(work func()) {
		work()
	})

	host := &testHost{}
	s.Show = func() {
		host.shown++
	}
	s.Open = func(path string) error {
		host.opened = append(host.opened, path)
		_, err := library.OpenBook(path)
		return err
	}
	window.SetContent(s.Content())

	return s, host
}

func pathsOnDisk(t *testing.T, s *Shelf) []string {
	t.Helper()

	var saved struct {
		Library []userdata.SavedBook `json:"library"`
	}
	if _, err := store.Load(s.data.File(), &saved); err != nil {
		t.Fatalf("reading the saved library: %v", err)
	}

	var paths []string
	for _, book := range saved.Library {
		paths = append(paths, book.Path)
	}

	return paths
}

func shelfTitles(s *Shelf) []string {
	var titles []string
	for _, entry := range s.entries {
		titles = append(titles, entry.card.Title)
	}

	return titles
}
