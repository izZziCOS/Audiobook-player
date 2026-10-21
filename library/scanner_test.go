package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/izzzicos/audiobook-player/vfs"
)

type countingFolder struct {
	root     string
	listings atomic.Int64
	checks   atomic.Int64
}

func (c *countingFolder) Open(name string) (vfs.File, error) {
	return os.Open(filepath.Join(c.root, filepath.FromSlash(name)))
}

func (c *countingFolder) Stat(name string) (fs.FileInfo, error) {
	c.checks.Add(1)
	return os.Stat(filepath.Join(c.root, filepath.FromSlash(name)))
}

func (c *countingFolder) ReadDir(name string) ([]fs.DirEntry, error) {
	c.listings.Add(1)
	return os.ReadDir(filepath.Join(c.root, filepath.FromSlash(name)))
}

func (c *countingFolder) WriteAtomic(string, []byte) error { return nil }
func (c *countingFolder) Connected() bool                  { return true }

func (c *countingFolder) reset() {
	c.listings.Store(0)
	c.checks.Store(0)
}

func setAge(t *testing.T, root string, age time.Duration) {
	t.Helper()

	when := time.Now().Add(-age)
	filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err == nil {
			os.Chtimes(path, when, when)
		}
		return nil
	})
}

func scannedLibrary(t *testing.T) (string, *countingFolder) {
	t.Helper()

	local := t.TempDir()
	for _, folder := range []string{"Tolkien/The Hobbit", "Herbert", "Austen/Emma/Extras", "Empty"} {
		os.MkdirAll(filepath.Join(local, folder), 0o755)
	}
	createFiles(t, filepath.Join(local, "Tolkien", "The Hobbit"), "Part 1.mp3", "Part 2.mp3")
	createFiles(t, filepath.Join(local, "Herbert"), "Dune.m4b", "Dune Messiah.m4b")
	createFiles(t, filepath.Join(local, "Austen", "Emma"), "1.mp3", "2.mp3", "3.mp3")
	setAge(t, local, time.Hour)

	root := filepath.Join(t.TempDir(), "Server")
	counting := &countingFolder{root: local}
	vfs.Mount(root, counting)
	t.Cleanup(func() { vfs.Unmount(root) })

	return root, counting
}

func foundPaths(found []Found) []string {
	var paths []string
	for _, book := range found {
		paths = append(paths, book.Path)
	}
	sort.Strings(paths)
	return paths
}

func TestScannerFindsWhatFindBooksFinds(t *testing.T) {
	root, _ := scannedLibrary(t)

	expected, err := FindBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(expected)

	found, err := NewScanner().Scan(root)
	if err != nil || !slices.Equal(foundPaths(found), expected) {
		t.Fatalf("got %v, %v, want %v", foundPaths(found), err, expected)
	}
	for _, book := range found {
		if parts, _, _ := Inspect(book.Path); parts != book.Parts {
			t.Errorf("%s: %d parts, want %d", book.Path, book.Parts, parts)
		}
	}
}

func TestAnUnchangedLibraryIsntReadAgain(t *testing.T) {
	root, counting := scannedLibrary(t)
	scanner := NewScanner()

	scanner.Scan(root)
	if counting.listings.Load() != 8 {
		t.Fatalf("the first check should list every folder once, listed %d", counting.listings.Load())
	}

	counting.reset()
	found, _ := scanner.Scan(root)
	if counting.listings.Load() != 0 || counting.checks.Load() != 8 || len(found) != 4 {
		t.Fatalf("an unchanged library should only be checked, not listed: %d listings, %d checks, %d books", counting.listings.Load(), counting.checks.Load(), len(found))
	}

	local := counting.root
	os.MkdirAll(filepath.Join(local, "Austen", "Persuasion"), 0o755)
	createFiles(t, filepath.Join(local, "Austen", "Persuasion"), "1.mp3")
	setAge(t, filepath.Join(local, "Austen", "Persuasion"), time.Hour)
	os.Chtimes(filepath.Join(local, "Austen"), time.Now(), time.Now())
	os.Remove(filepath.Join(local, "Herbert", "Dune Messiah.m4b"))

	counting.reset()
	found, _ = scanner.Scan(root)
	if counting.listings.Load() != 3 {
		t.Fatalf("only the changed folders and the new one should be listed, listed %d", counting.listings.Load())
	}
	paths := foundPaths(found)
	if !slices.Contains(paths, filepath.Join(root, "Austen", "Persuasion")) || slices.Contains(paths, filepath.Join(root, "Herbert", "Dune Messiah.m4b")) {
		t.Fatalf("new and removed books should be noticed, got %v", paths)
	}
}

func TestABookBeingCopiedIsWatched(t *testing.T) {
	root, counting := scannedLibrary(t)
	local := counting.root
	scanner := NewScanner()
	scanner.Scan(root)

	os.MkdirAll(filepath.Join(local, "Copying"), 0o755)
	createFiles(t, filepath.Join(local, "Copying"), "1.mp3", "2.mp3")
	found, _ := scanner.Scan(root)
	copying := findBook(found, filepath.Join(root, "Copying"))
	if copying == nil || time.Since(copying.Modified) > time.Minute {
		t.Fatalf("a book being copied should look recently changed, got %+v", copying)
	}

	later := time.Now().Add(-time.Hour)
	for _, name := range []string{"1.mp3", "2.mp3"} {
		os.Chtimes(filepath.Join(local, "Copying", name), later, later)
	}
	found, _ = scanner.Scan(root)
	if copying := findBook(found, filepath.Join(root, "Copying")); copying == nil || !copying.Modified.Equal(later) {
		t.Fatalf("once copying stops the book should be seen as settled, got %+v", copying)
	}
}

func findBook(found []Found, path string) *Found {
	for i := range found {
		if found[i].Path == path {
			return &found[i]
		}
	}
	return nil
}

func TestTheScanIsRememberedBetweenStarts(t *testing.T) {
	root, counting := scannedLibrary(t)
	file := filepath.Join(t.TempDir(), "scan-cache.json")

	first := SavedScanner(file)
	first.Scan(root)
	if err := first.Save(); err != nil {
		t.Fatal(err)
	}

	counting.reset()
	found, err := SavedScanner(file).Scan(root)
	if err != nil || len(found) != 4 || counting.listings.Load() != 0 {
		t.Fatalf("after a restart an unchanged library shouldn't be listed again: %d listings, %d books, %v", counting.listings.Load(), len(found), err)
	}

	os.WriteFile(file, []byte("{broken"), 0o644)
	counting.reset()
	if found, _ := SavedScanner(file).Scan(root); len(found) != 4 || counting.listings.Load() != 8 {
		t.Fatalf("a damaged file should just mean reading everything again, %d listings", counting.listings.Load())
	}

	counting.reset()
	forgotten := SavedScanner(file).Forget()
	forgotten.Scan(root)
	if counting.listings.Load() != 8 {
		t.Fatal("Check now should read everything again")
	}
}
