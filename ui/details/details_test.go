package details

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

const book = "/books/The_Hobbit"

func newTestEditor(t *testing.T) (*Editor, *[]string) {
	t.Helper()

	app := test.NewApp()
	t.Cleanup(app.Quit)

	window := app.NewWindow("Audiobook Player")
	editor := New(window, &kit.Picker{}, userdata.New(userdata.FileIn(t.TempDir())), func(work func()) {
		work()
	})

	var saved []string
	editor.OnSaved = func(key string) {
		saved = append(saved, key)
	}

	return editor, &saved
}

func writeImage(t *testing.T) string {
	t.Helper()

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewGray(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

var original = Original{Title: "The Hobit", Author: "Tolkein"}

func TestCorrectingTheTitleAndAuthor(t *testing.T) {
	editor, saved := newTestEditor(t)

	form := editor.open(book, original)
	if form.title.Text != "The Hobit" || form.author.Text != "Tolkein" {
		t.Fatalf("the form starts with %q by %q, want the files' details", form.title.Text, form.author.Text)
	}

	form.title.SetText("The Hobbit")
	form.author.SetText(" J. R. R.  Tolkien ")
	test.Tap(form.save)

	edit := editor.data.Edit(book)
	if edit.Title != "The Hobbit" || edit.Author != "J. R. R. Tolkien" {
		t.Fatalf("saved %+v", edit)
	}
	if len(*saved) != 1 || (*saved)[0] != book {
		t.Fatalf("OnSaved was called with %v", *saved)
	}

	form = editor.open(book, original)
	if form.title.Text != "The Hobbit" || form.author.Text != "J. R. R. Tolkien" {
		t.Fatalf("editing again starts with %q by %q, want the corrections", form.title.Text, form.author.Text)
	}
}

func TestClearingOrMatchingTheFilesUndoesACorrection(t *testing.T) {
	editor, _ := newTestEditor(t)
	editor.data.SetEdit(book, userdata.BookEdit{Title: "The Hobbit", Author: "J. R. R. Tolkien"})

	form := editor.open(book, original)
	form.title.SetText("")
	form.author.SetText("Tolkein")
	form.submit()

	if edit := editor.data.Edit(book); edit != (userdata.BookEdit{}) {
		t.Fatalf("got %+v, want no correction left", edit)
	}
}

func TestChoosingAndRemovingACustomCover(t *testing.T) {
	editor, _ := newTestEditor(t)

	form := editor.open(book, original)
	if !form.useOriginal.Disabled() || form.preview.Visible() {
		t.Fatal("a book without a cover should show the placeholder and nothing to undo")
	}

	form.useImage(writeImage(t))
	if form.useOriginal.Disabled() || !form.preview.Visible() {
		t.Fatal("a chosen image should be previewed")
	}
	if editor.data.Edit(book).Cover != "" {
		t.Fatal("the cover shouldn't be saved until Save is pressed")
	}
	form.submit()

	file := editor.data.CoverFile(book)
	if data, err := os.ReadFile(file); err != nil || len(data) == 0 {
		t.Fatalf("the custom cover wasn't saved: %v", err)
	}

	form = editor.open(book, original)
	if form.useOriginal.Disabled() || !form.preview.Visible() {
		t.Fatal("editing again should show the custom cover")
	}
	test.Tap(form.useOriginal)
	form.submit()

	if editor.data.CoverFile(book) != "" {
		t.Fatal("Use original should remove the custom cover")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("the unused cover file should be deleted")
	}
}

func TestChoosingSomethingThatIsNotAnImage(t *testing.T) {
	editor, _ := newTestEditor(t)
	path := filepath.Join(t.TempDir(), "notes.png")
	os.WriteFile(path, []byte("not an image"), 0o644)

	form := editor.open(book, original)
	form.useImage(path)

	if form.chosen != nil || !form.useOriginal.Disabled() {
		t.Fatal("a file that isn't an image shouldn't be used")
	}
}
