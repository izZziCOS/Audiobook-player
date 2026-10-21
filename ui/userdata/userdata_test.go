package userdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/store"
)

func testPreferences(t *testing.T) fyne.Preferences {
	t.Helper()

	app := test.NewApp()
	t.Cleanup(app.Quit)
	return app.Preferences()
}

func savedLibrary(paths ...string) []SavedBook {
	var books []SavedBook
	for _, path := range paths {
		books = append(books, SavedBook{Path: path})
	}

	return books
}

func TestDataSurvivesARestart(t *testing.T) {
	file := FileIn(t.TempDir())
	data := New(file)
	book := library.SingleFile(filepath.Join(t.TempDir(), "Dune.m4b"))

	data.Library = savedLibrary(book.Key())
	data.Remember(book, 42, true)
	data.Ignore("/books/removed")
	data.Finish(book.Key())

	restarted := New(file)
	if notice := restarted.Load(testPreferences(t)); notice != "" {
		t.Fatalf("unexpected notice %q", notice)
	}

	position := restarted.Positions[book.Key()]
	if position.Position != 42 || !position.Finished || len(restarted.Library) != 1 || restarted.LastBook.File != book.Current() {
		t.Fatalf("after a restart the data is %+v", restarted)
	}
	if !restarted.Ignored[library.Normalize("/books/removed")] || restarted.Stats.CompletedCount() != 1 {
		t.Fatal("removed books and finished books should come back")
	}
}

func TestDamagedLibraryIsRestoredFromBackup(t *testing.T) {
	file := FileIn(t.TempDir())
	data := New(file)
	data.Library = savedLibrary("a", "b", "c")
	data.Save()

	if len(store.Backups(file)) != 1 {
		t.Fatal("saving should make today's backup")
	}

	if err := os.WriteFile(file, []byte(`{"version":1,"library":[{"pa`), 0o600); err != nil {
		t.Fatal(err)
	}

	restarted := New(file)
	notice := restarted.Load(testPreferences(t))

	if !strings.Contains(notice, "restored from the backup made on") || len(restarted.Library) != 3 {
		t.Fatalf("expected a restore from backup, got %q with %d books", notice, len(restarted.Library))
	}
}

func TestDamagedLibraryWithoutBackupIsKept(t *testing.T) {
	file := FileIn(t.TempDir())
	if err := os.WriteFile(file, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	notice := New(file).Load(testPreferences(t))
	if !strings.Contains(notice, "empty library") || !strings.Contains(notice, "damaged file was kept") {
		t.Fatalf("notice %q", notice)
	}

	kept, _ := filepath.Glob(filepath.Join(filepath.Dir(file), "library.damaged-*.json"))
	if len(kept) != 1 {
		t.Fatal("the damaged file should be set aside, not overwritten")
	}
}

func TestSaveFailureIsReported(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-folder")
	os.WriteFile(blocker, []byte("x"), 0o600)
	data := New(filepath.Join(blocker, fileName))

	reported := 0
	data.OnSaveError = func(error) {
		reported++
	}

	data.Save()
	data.Save()

	if reported != 1 || !data.Dirty() {
		t.Fatalf("a failed save should be reported once and retried later, reported %d times", reported)
	}
}

func TestDataMovesOutOfPreferences(t *testing.T) {
	prefs := testPreferences(t)
	folder := filepath.Join(t.TempDir(), "The_Hobbit")
	part := filepath.Join(folder, "Part 2.flac")
	removed := filepath.Join(t.TempDir(), "removed")

	prefs.SetString("bookPositions", `{"`+strings.ReplaceAll(folder, `\`, `\\`)+`":{"part":"`+strings.ReplaceAll(part, `\`, `\\`)+`","position":0.2}}`)
	prefs.SetStringList("library", []string{folder})
	prefs.SetStringList("libraryIgnored", []string{removed})
	prefs.SetString("listeningStats", `{"days":{"2026-10-01":{"listened":120,"content":120}}}`)
	prefs.SetString("bookFolder", folder)
	prefs.SetString("lastFile", storage.NewFileURI(part).String())

	file := FileIn(t.TempDir())
	data := New(file)
	data.Load(prefs)

	if len(data.Library) != 1 || data.Positions[folder].Position != 0.2 || data.LastBook != (LastBook{Folder: folder, File: part}) {
		t.Fatalf("the old library and position should be used, got %+v", data)
	}
	if !data.Ignored[removed] || data.Stats.Total() != 120 {
		t.Fatal("removed books and statistics should move over")
	}

	restarted := New(file)
	restarted.Load(testPreferences(t))
	if len(restarted.Library) != 1 || restarted.Stats.Total() != 120 {
		t.Fatalf("the data file should hold the moved data, got %+v", restarted)
	}
	for _, key := range migratedPreferences {
		if prefs.String(key) != "" || len(prefs.StringList(key)) != 0 {
			t.Fatalf("%s should be removed from the preferences", key)
		}
	}
}

func TestOldestUpgradeKeepsTheLastPosition(t *testing.T) {
	prefs := testPreferences(t)
	part := filepath.Join(t.TempDir(), "Part 3.ogg")
	prefs.SetString("lastFile", storage.NewFileURI(part).String())
	prefs.SetFloat("lastPosition", 0.4)

	data := New(FileIn(t.TempDir()))
	data.Load(prefs)

	if data.Positions[part].Position != 0.4 {
		t.Fatal("upgrading from the first version should keep the single saved position")
	}
}

func TestMovingABooksData(t *testing.T) {
	data := New("")
	old, moved := filepath.Join("books", "Hobbit"), filepath.Join("fantasy", "Hobbit")
	part := filepath.Join(old, "Part 1.mp3")

	data.Positions[old] = Position{Part: part, Position: 3}
	data.Durations[part] = PartDuration{Seconds: 60}
	data.Stats.Record(time.Now(), 2, 1, old)
	data.Ignored[old] = true
	data.LastBook = LastBook{Folder: old, File: part}

	data.Move(old, moved)

	movedPart := filepath.Join(moved, "Part 1.mp3")
	if data.Positions[moved].Part != movedPart || data.Durations[movedPart].Seconds != 60 || !data.Ignored[moved] {
		t.Fatalf("the book's data should follow it, got %+v", data)
	}
	if data.Stats.Books[moved] != 2 {
		t.Fatal("listening time should follow the book")
	}
	if _, left := data.Positions[old]; left || data.LastBook != (LastBook{Folder: moved, File: movedPart}) {
		t.Fatal("nothing should be left under the old path")
	}
}
