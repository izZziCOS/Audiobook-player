package autoimport

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/store"
	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/shelf"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

func newTestImporter(t *testing.T) (*Importer, *shelf.Shelf) {
	t.Helper()

	app := test.NewApp()
	t.Cleanup(app.Quit)

	window := app.NewWindow("Settings")
	data := userdata.New(userdata.FileIn(t.TempDir()))
	picker := &kit.Picker{}
	inline := func(work func()) {
		work()
	}

	books := shelf.New(window, app.Preferences(), data, picker, inline)
	im := New(window, app.Preferences(), data, books, picker, inline)
	window.SetContent(im.Section())

	return im, books
}

func ignoredOnDisk(t *testing.T, im *Importer) []string {
	t.Helper()

	var saved struct {
		Ignored []string `json:"ignored"`
	}
	if _, err := store.Load(im.data.File(), &saved); err != nil {
		t.Fatal(err)
	}

	return saved.Ignored
}

func TestAutoImportFromLibraryFolder(t *testing.T) {
	im, books := newTestImporter(t)
	root, hobbit := testbooks.Library(t)
	testbooks.Backdate(t, root)
	dune := filepath.Join(root, "Herbert", "Dune.m4b")

	im.SetFolder(root)

	if len(books.Cards()) != 3 || !strings.Contains(im.status.Text, "3 new books") {
		t.Fatalf("expected 3 imported books, got %d and %q", len(books.Cards()), im.status.Text)
	}
	if im.prefs.String(preferenceWatchedFolder) != root || im.folderLabel.Text != root || !im.folderActions.Visible() {
		t.Fatal("the folder should be saved and shown")
	}

	fresh := filepath.Join(root, "New", "Fresh.m4b")
	testbooks.CopyFile(t, "speech-aac-mono.m4b", fresh)
	im.Scan()
	if books.Contains(fresh) || !strings.Contains(im.status.Text, "1 book is still being copied") {
		t.Fatalf("a book that is still being copied should wait, status %q", im.status.Text)
	}

	testbooks.Backdate(t, filepath.Dir(fresh))
	im.Scan()
	if !books.Contains(fresh) || !strings.Contains(im.status.Text, "1 new book") {
		t.Fatalf("the copied book should be imported once it settles, status %q", im.status.Text)
	}

	books.Remove(dune)
	im.Scan()
	if books.Contains(dune) {
		t.Fatal("a book removed by the user should not be imported again")
	}
	if !slices.Contains(ignoredOnDisk(t, im), dune) {
		t.Fatal("removed books should be remembered")
	}

	books.Add([]string{dune})
	if !books.Contains(dune) || im.data.Ignored[dune] {
		t.Fatal("adding a removed book by hand should bring it back")
	}

	testbooks.CopyFile(t, "speech-16bit-mono.wav", filepath.Join(hobbit, "Part 3.wav"))
	testbooks.Backdate(t, hobbit)
	im.Scan()
	if card, _ := books.Card(hobbit); card.Author != "3 parts" {
		t.Fatalf("a book that gained a part should be refreshed, it shows %q", card.Author)
	}

	im.SetFolder("")
	if im.folderActions.Visible() || im.folderLabel.Text != "No folder chosen" || im.prefs.String(preferenceWatchedFolder) != "" {
		t.Fatal("stopping should clear the folder")
	}

	another := filepath.Join(root, "Later", "Later.m4b")
	testbooks.CopyFile(t, "speech-aac-mono.m4b", another)
	testbooks.Backdate(t, filepath.Dir(another))
	im.Scan()
	if books.Contains(another) {
		t.Fatal("nothing should be imported once importing has stopped")
	}
}

func TestCheckingOnlyWhenStale(t *testing.T) {
	im, books := newTestImporter(t)
	root, _ := testbooks.Library(t)
	testbooks.Backdate(t, root)
	im.SetFolder(root)

	added := filepath.Join(root, "Added", "Added.m4b")
	testbooks.CopyFile(t, "speech-aac-mono.m4b", added)
	testbooks.Backdate(t, filepath.Dir(added))

	im.ScanIfStale()
	if books.Contains(added) {
		t.Fatal("the folder was just checked, so it should not be checked again yet")
	}

	im.lastImport = time.Now().Add(-time.Minute)
	im.ScanIfStale()
	if !books.Contains(added) {
		t.Fatal("a stale folder should be checked again")
	}
}

func TestUnreadableLibraryFolder(t *testing.T) {
	im, books := newTestImporter(t)

	im.SetFolder(filepath.Join(t.TempDir(), "missing"))

	if !strings.HasPrefix(im.status.Text, "The library folder isn't available") || len(books.Cards()) != 0 {
		t.Fatalf("status %q", im.status.Text)
	}
}

func TestRememberedFolderIsScanned(t *testing.T) {
	im, books := newTestImporter(t)
	root, _ := testbooks.Library(t)
	testbooks.Backdate(t, root)
	im.prefs.SetString(preferenceWatchedFolder, root)

	im.Load()

	if im.Folder() != root || len(books.Cards()) != 3 {
		t.Fatalf("the remembered folder should be checked on start, got %q with %d books", im.Folder(), len(books.Cards()))
	}
}

func TestLibraryFolderSkipsCopiesOfBooksAlreadyAdded(t *testing.T) {
	im, books := newTestImporter(t)
	elsewhere := filepath.Join(t.TempDir(), "Downloads", "Dune.m4b")
	testbooks.CopyFile(t, "speech-aac-mono.m4b", elsewhere)
	books.Add([]string{elsewhere})

	root, _ := testbooks.Library(t)
	testbooks.Backdate(t, root)
	im.SetFolder(root)

	if len(books.Cards()) != 3 || books.Contains(filepath.Join(root, "Herbert", "Dune.m4b")) {
		t.Fatalf("the copy of Dune shouldn't be added, the library has %d books", len(books.Cards()))
	}
	if !strings.Contains(im.status.Text, "2 new books") || !strings.Contains(im.status.Text, "1 copy of a book already in your library was skipped") {
		t.Fatalf("status %q", im.status.Text)
	}
}

func TestNetworkFoldersAreCheckedLessOften(t *testing.T) {
	im, _ := newTestImporter(t)
	root, _ := testbooks.Library(t)
	testbooks.Backdate(t, root)
	im.SetFolder(root)

	scans := 0
	original := reachable
	reachable = func(path string, timeout time.Duration) error {
		scans++
		return original(path, timeout)
	}
	t.Cleanup(func() { reachable = original })

	im.lastImport = time.Now().Add(-Interval)
	im.ScanIfDue()
	if scans != 1 {
		t.Fatal("a local folder should be checked every couple of minutes")
	}

	im.network.Store(true)
	im.lastImport = time.Now().Add(-Interval)
	im.ScanIfDue()
	im.lastImport = time.Now().Add(-libraryTabRecheck)
	im.ScanIfStale()
	if scans != 1 {
		t.Fatal("a folder on a server should be checked less often")
	}

	im.lastImport = time.Now().Add(-networkInterval)
	im.ScanIfDue()
	if scans != 2 {
		t.Fatal("a folder on a server should still be checked in the end")
	}
}

func TestAStalledServerIsReported(t *testing.T) {
	im, books := newTestImporter(t)
	root, _ := testbooks.Library(t)
	original := reachable
	reachable = func(string, time.Duration) error { return library.ErrNotResponding }
	t.Cleanup(func() { reachable = original })

	im.SetFolder(root)

	if !strings.Contains(im.status.Text, "isn't responding") || !strings.Contains(im.status.Text, "VPN") || len(books.Cards()) != 0 {
		t.Fatalf("status %q", im.status.Text)
	}
	if im.importing.Load() {
		t.Fatal("the next check should still be able to run")
	}
}

func TestTheLibraryFolderScanIsSaved(t *testing.T) {
	im, _ := newTestImporter(t)
	root, _ := testbooks.Library(t)
	testbooks.Backdate(t, root)

	im.SetFolder(root)

	saved := filepath.Join(filepath.Dir(im.data.File()), scanCacheName)
	data, err := os.ReadFile(saved)
	if err != nil || !strings.Contains(string(data), "The_Hobbit") {
		t.Fatalf("what the folder holds should be saved for next time: %v", err)
	}
}
