package library

import (
    "errors"
    "os"
    "path/filepath"
    "runtime"
    "testing"
    "time"
)

func TestSkipFolder(t *testing.T) {
    for _, name := range []string{".hidden", "@eaDir", "@tmp", "#recycle", "#snapshot", "$RECYCLE.BIN", "System Volume Information", "lost+found"} {
        if !SkipFolder(name) {
            t.Errorf("%q should be skipped", name)
        }
    }
    for _, name := range []string{"Tolkien", "#1 Bestsellers", "Recycle"} {
        if SkipFolder(name) {
            t.Errorf("%q is an ordinary folder", name)
        }
    }
}

func TestSynologyFoldersAreNotScanned(t *testing.T) {
    root := t.TempDir()
    for _, folder := range []string{"Tolkien/The Hobbit", "#recycle/Deleted Book", "#snapshot/GMT+00_2026.10.20/Tolkien/The Hobbit", "Tolkien/The Hobbit/@eaDir/Part 1.mp3"} {
        if err := os.MkdirAll(filepath.Join(root, folder), 0o755); err != nil {
            t.Fatal(err)
        }
    }
    createFiles(t, filepath.Join(root, "Tolkien", "The Hobbit"), "Part 1.mp3", "Part 2.mp3")
    createFiles(t, filepath.Join(root, "#recycle", "Deleted Book"), "1.mp3")
    createFiles(t, filepath.Join(root, "#snapshot", "GMT+00_2026.10.20", "Tolkien", "The Hobbit"), "Part 1.mp3")
    createFiles(t, filepath.Join(root, "Tolkien", "The Hobbit", "@eaDir", "Part 1.mp3"), "SYNOINDEX_MEDIA_INFO.mp3")

    books, err := FindBooks(root)
    if err != nil || len(books) != 1 || books[0] != filepath.Join(root, "Tolkien", "The Hobbit") {
        t.Fatalf("got %v, %v, want only The Hobbit", books, err)
    }
}

func TestVolumeRoot(t *testing.T) {
    for _, test := range []struct{ path, goos, want string }{
        {"/Volumes/audiobooks/Tolkien/The Hobbit", "darwin", "/Volumes/audiobooks"},
        {"/Volumes/audiobooks", "darwin", "/Volumes/audiobooks"},
        {"/Volumes", "darwin", ""},
        {"/Users/me/Books/Dune.m4b", "darwin", ""},
        {"/mnt/nas/Books/Dune.m4b", "linux", "/mnt/nas"},
        {"/media/me/USB/Dune.m4b", "linux", "/media/me/USB"},
        {"/run/media/me/USB/Dune.m4b", "linux", "/run/media/me/USB"},
        {"/home/me/Books/Dune.m4b", "linux", ""},
        {`Z:\Tolkien\The Hobbit`, "windows", `Z:\`},
        {`\\nas\audiobooks\Tolkien`, "windows", `\\nas\audiobooks\`},
        {`\\nas`, "windows", ""},
    } {
        if got := volumeRoot(test.path, test.goos); got != test.want {
            t.Errorf("volumeRoot(%q, %s) = %q, want %q", test.path, test.goos, got, test.want)
        }
    }
}

func TestUnavailable(t *testing.T) {
    home, _ := os.UserHomeDir()
    if Unavailable(filepath.Join(home, "Books", "Dune.m4b")) {
        t.Fatal("a book in the home folder is never on a disconnected drive")
    }

    switch runtime.GOOS {
    case "darwin":
        if !Unavailable("/Volumes/no-such-share-for-tests/Dune.m4b") {
            t.Fatal("a book on a share that isn't mounted should be unavailable")
        }
    case "windows":
        if !Unavailable(`\\no-such-server-for-tests\share\Dune.m4b`) {
            t.Fatal("a book on a server that isn't there should be unavailable")
        }
    }
}

func TestReachable(t *testing.T) {
    if err := Reachable(t.TempDir(), time.Second); err != nil {
        t.Fatalf("an existing folder should be reachable, got %v", err)
    }
    if err := Reachable(filepath.Join(t.TempDir(), "missing"), time.Second); !errors.Is(err, os.ErrNotExist) {
        t.Fatalf("got %v, want not found", err)
    }
    if err := Reachable(t.TempDir(), 0); err != nil && !errors.Is(err, ErrNotResponding) {
        t.Fatalf("got %v", err)
    }
    if IsNetwork(t.TempDir()) {
        t.Fatal("a temporary folder isn't on a network drive")
    }
}
