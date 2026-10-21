package shelf

import (
    "bytes"
    "image"
    "image/png"
    "os"
    "path/filepath"
    "testing"
    "time"

    "fyne.io/fyne/v2/test"

    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func openShelfAt(t *testing.T, file string) *Shelf {
    t.Helper()

    app := test.NewApp()
    t.Cleanup(app.Quit)

    data := userdata.New(file)
    data.Load(app.Preferences())
    s := New(app.NewWindow("Library"), app.Preferences(), data, &kit.Picker{}, func(work func()) {
        work()
    })
    s.Load()

    return s
}

func writeImage(t *testing.T, path string, side int) {
    t.Helper()

    var encoded bytes.Buffer
    if err := png.Encode(&encoded, image.NewGray(image.Rect(0, 0, side, side))); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
        t.Fatal(err)
    }
}

func touchLater(t *testing.T, path string) {
    t.Helper()

    later := time.Now().Add(time.Hour)
    if err := os.Chtimes(path, later, later); err != nil {
        t.Fatal(err)
    }
}

func thumbnailFiles(t *testing.T, s *Shelf) []string {
    t.Helper()

    entries, _ := os.ReadDir(s.cache.folder)
    var names []string
    for _, entry := range entries {
        names = append(names, entry.Name())
    }

    return names
}

func cardFor(t *testing.T, s *Shelf, path string) *shelfEntry {
    t.Helper()

    entry := s.find(path)
    if entry == nil {
        t.Fatalf("%s isn't in the library", path)
    }

    return entry
}

func TestCoversAreCachedBetweenRestarts(t *testing.T) {
    file := userdata.FileIn(t.TempDir())
    root, hobbit := testbooks.Library(t)
    writeImage(t, filepath.Join(hobbit, "cover.png"), 600)

    first := openShelfAt(t, file)
    first.AddFrom([]string{root})
    if cardFor(t, first, hobbit).card.Thumbnail == nil || len(thumbnailFiles(t, first)) != 1 {
        t.Fatalf("the cover should be shown and cached, thumbnails %v", thumbnailFiles(t, first))
    }

    cached := first.cache.books[hobbit]
    cached.Title = "From the cache"
    first.cache.books[hobbit] = cached
    first.cache.dirty = true
    first.cache.save()

    restarted := openShelfAt(t, file)
    entry := cardFor(t, restarted, hobbit)
    if entry.title != "From the cache" || entry.card.Thumbnail == nil || entry.card.Thumbnail.Bounds().Dx() != thumbnailSide {
        t.Fatalf("an unchanged book should come from the cache, got %q", entry.title)
    }
    if len(restarted.entries) != 3 || restarted.find(filepath.Join(root, "Herbert", "Dune.m4b")).title != "Dune" {
        t.Fatal("books without covers should be cached too")
    }
}

func TestChangedCoversAreReadAgain(t *testing.T) {
    file := userdata.FileIn(t.TempDir())
    root, hobbit := testbooks.Library(t)
    cover := filepath.Join(hobbit, "cover.png")
    writeImage(t, cover, 600)

    first := openShelfAt(t, file)
    first.AddFrom([]string{root})
    previous := thumbnailFiles(t, first)

    writeImage(t, cover, 200)
    touchLater(t, cover)
    restarted := openShelfAt(t, file)
    if thumbnail := cardFor(t, restarted, hobbit).card.Thumbnail; thumbnail == nil || thumbnail.Bounds().Dx() != 200 {
        t.Fatal("a replaced cover should be read again")
    }
    if now := thumbnailFiles(t, restarted); len(now) != 1 || now[0] == previous[0] {
        t.Fatalf("the old thumbnail should be replaced, thumbnails %v", now)
    }

    writeImage(t, filepath.Join(hobbit, "Part 1.png"), 300)
    touchLater(t, hobbit)
    restarted = openShelfAt(t, file)
    if thumbnail := cardFor(t, restarted, hobbit).card.Thumbnail; thumbnail == nil || thumbnail.Bounds().Dx() != 300 {
        t.Fatal("a new image that takes priority should be noticed")
    }
}

func TestCacheForgetsRemovedBooksAndSurvivesDamage(t *testing.T) {
    file := userdata.FileIn(t.TempDir())
    root, hobbit := testbooks.Library(t)
    writeImage(t, filepath.Join(hobbit, "cover.png"), 600)

    first := openShelfAt(t, file)
    first.AddFrom([]string{root})
    first.Remove(hobbit)

    restarted := openShelfAt(t, file)
    if _, found := restarted.cache.books[hobbit]; found || len(thumbnailFiles(t, restarted)) != 0 {
        t.Fatalf("a removed book's cached cover should be deleted, thumbnails %v", thumbnailFiles(t, restarted))
    }

    restarted.Add([]string{hobbit})
    if err := os.WriteFile(restarted.cache.file, []byte("{not json"), 0o644); err != nil {
        t.Fatal(err)
    }
    damaged := openShelfAt(t, file)
    if entry := cardFor(t, damaged, hobbit); entry.title != "The Hobbit" || entry.card.Thumbnail == nil {
        t.Fatal("a damaged cache should only mean reading the books again")
    }
}

func TestCustomCoversAreCached(t *testing.T) {
    file := userdata.FileIn(t.TempDir())
    root, hobbit := testbooks.Library(t)

    first := openShelfAt(t, file)
    first.AddFrom([]string{root})

    var encoded bytes.Buffer
    png.Encode(&encoded, image.NewGray(image.Rect(0, 0, 900, 900)))
    name, err := first.data.SaveCover(encoded.Bytes())
    if err != nil {
        t.Fatal(err)
    }
    first.data.SetEdit(hobbit, userdata.BookEdit{Cover: name})
    first.Reload(hobbit)
    if len(first.cache.covers) != 1 || cardFor(t, first, hobbit).card.Thumbnail == nil {
        t.Fatal("the custom cover should be shown and cached")
    }

    first.data.SetEdit(hobbit, userdata.BookEdit{})
    restarted := openShelfAt(t, file)
    if len(restarted.cache.covers) != 0 || len(thumbnailFiles(t, restarted)) != 0 {
        t.Fatalf("an unused custom cover's thumbnail should be deleted, thumbnails %v", thumbnailFiles(t, restarted))
    }
}
