package offline

import (
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
    "time"

    "fyne.io/fyne/v2/test"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func newTestStore(t *testing.T) *Store {
    t.Helper()

    app := test.NewApp()
    t.Cleanup(app.Quit)

    return New(app.NewWindow("Library"), app.Preferences(), userdata.New(userdata.FileIn(t.TempDir())), func(work func()) {
        work()
    })
}

func TestDownloadingABook(t *testing.T) {
    s := newTestStore(t)
    folder, parts := testbooks.Book(t)
    testbooks.WriteCover(t, filepath.Join(folder, "cover.png"))

    var changes int
    s.OnChanged = func(string) { changes++ }
    s.Start(folder)

    download, found := s.data.Download(folder)
    if !found || !download.Complete || len(download.Parts) != 3 || download.Cover != "cover.png" || changes == 0 {
        t.Fatalf("got %+v", download)
    }
    if s.State(folder) != Downloaded || !strings.HasPrefix(s.usage.Text, "1 book downloaded") {
        t.Fatalf("state %v, usage %q", s.State(folder), s.usage.Text)
    }

    for i, part := range download.Parts {
        local := filepath.Join(s.folder, download.Folder, part.File)
        original, _ := os.Stat(parts[i])
        copied, err := os.Stat(local)
        if err != nil || copied.Size() != original.Size() || !copied.ModTime().Equal(original.ModTime()) {
            t.Fatalf("part %d wasn't copied exactly: %v", i+1, err)
        }
    }

    leftovers, _ := filepath.Glob(filepath.Join(s.folder, download.Folder, "*.part"))
    if len(leftovers) != 0 {
        t.Fatalf("temporary files were left behind: %v", leftovers)
    }
}

func TestADownloadedBookPlaysWithoutTheOriginal(t *testing.T) {
    s := newTestStore(t)
    folder, parts := testbooks.Book(t)
    s.Start(folder)

    away := folder + " (away)"
    if err := os.Rename(folder, away); err != nil {
        t.Fatal(err)
    }

    book, err := s.Open(folder)
    if err != nil || book.Key() != folder || len(book.Local) != 3 {
        t.Fatalf("got %+v, %v", book, err)
    }
    if book.Parts[1] != parts[1] || !strings.HasPrefix(book.Local[1], s.folder) {
        t.Fatal("the book should keep its own parts as its identity and play the copies")
    }
    if _, err := os.Stat(book.PlayablePart(1)); err != nil {
        t.Fatalf("the copy should be playable: %v", err)
    }

    os.Rename(away, folder)
    fresh, _ := library.OpenBook(folder)
    s.Prepare(fresh)
    if fresh.Playable() != book.Local[0] {
        t.Fatal("even with the original there, the copy should be played")
    }
}

func TestAnInterruptedDownloadCarriesOn(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("file permissions can't make a file unreadable here")
    }

    s := newTestStore(t)
    folder, parts := testbooks.Book(t)
    os.Chmod(parts[2], 0)
    t.Cleanup(func() { os.Chmod(parts[2], 0o644) })

    s.Start(folder)
    download, _ := s.data.Download(folder)
    if download.Complete || !download.Parts[0].Done || !download.Parts[1].Done || download.Parts[2].Done || s.State(folder) != Paused {
        t.Fatalf("the download should stop at the part it couldn't read, got %+v", download)
    }
    first := filepath.Join(s.folder, download.Folder, download.Parts[0].File)
    before, _ := os.Stat(first)

    os.Chmod(parts[2], 0o644)
    s.Tick(time.Now())
    if s.State(folder) != Paused {
        t.Fatal("it shouldn't retry straight away")
    }

    s.Tick(time.Now().Add(retryAfter))
    after, _ := os.Stat(first)
    if s.State(folder) != Downloaded || !after.ModTime().Equal(before.ModTime()) {
        t.Fatal("the retry should finish the download without copying the finished parts again")
    }
}

func TestRemovingAndUpdatingDownloads(t *testing.T) {
    s := newTestStore(t)
    folder, _ := testbooks.Book(t)
    s.Start(folder)
    download, _ := s.data.Download(folder)

    fingerprint, _ := library.FingerprintOf(folder)
    if s.OutOfDate(folder, &fingerprint) {
        t.Fatal("a fresh download isn't out of date")
    }
    testbooks.CopyFile(t, "speech-16bit-mono.wav", filepath.Join(folder, "Part 4.wav"))
    fingerprint, _ = library.FingerprintOf(folder)
    if !s.OutOfDate(folder, &fingerprint) {
        t.Fatal("a book with a new part on the server should say its download is out of date")
    }

    s.Remove(folder)
    if _, err := os.Stat(filepath.Join(s.folder, download.Folder)); !os.IsNotExist(err) || s.State(folder) != NotDownloaded {
        t.Fatal("removing should delete the copy")
    }
    if _, err := os.Stat(folder); err != nil {
        t.Fatal("removing a download must never touch the original")
    }
}

func TestCancellingStopsTheCopy(t *testing.T) {
    _, parts := testbooks.Book(t)
    info, _ := os.Stat(parts[0])
    destination := filepath.Join(t.TempDir(), "Part 1.wav")
    current := &job{}
    current.cancelled.Store(true)

    err := copyPart(userdata.DownloadPart{Source: parts[0], Size: info.Size(), Modified: info.ModTime().UnixNano()}, destination, current, func() {})
    if err != errCancelled {
        t.Fatalf("got %v", err)
    }
    if entries, _ := os.ReadDir(filepath.Dir(destination)); len(entries) != 0 {
        t.Fatalf("a cancelled copy should leave nothing behind, found %d files", len(entries))
    }
}

func TestKeepingTheCurrentBookDownloaded(t *testing.T) {
    s := newTestStore(t)
    first, _ := testbooks.Book(t)
    second, _ := testbooks.Book(t)

    s.start(first, true, false)
    if s.State(first) != Downloaded {
        t.Fatal("setup: the first book should be downloaded")
    }

    s.BookOpened(second)
    if s.State(first) != Downloaded {
        t.Fatal("with the setting off, nothing should change")
    }

    s.automatic.SetChecked(true)
    s.BookOpened(second)
    if s.State(first) != NotDownloaded {
        t.Fatal("the previous automatic download should be removed")
    }
    if s.State(second) != NotDownloaded || s.Problem(second) != "" {
        t.Fatal("a book already on this computer isn't downloaded again, and that isn't a problem")
    }
}
