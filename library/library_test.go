package library

import (
    "image"
    "image/png"
    "os"
    "path/filepath"
    "sort"
    "testing"
)

func createFiles(t *testing.T, directory string, names ...string) {
    t.Helper()

    for _, name := range names {
        if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o644); err != nil {
            t.Fatal(err)
        }
    }
}

func TestNaturalLess(t *testing.T) {
    ordered := [][2]string{
        {"Part 2", "Part 10"},
        {"part 2", "Part 3"},
        {"Ch01", "Ch2"},
        {"a", "a1"},
        {"Disc 1 Track 9", "Disc 1 Track 10"},
        {"Disc 1 Track 10", "Disc 2 Track 1"},
        {"1", "01"},
    }

    for _, pair := range ordered {
        if !NaturalLess(pair[0], pair[1]) || NaturalLess(pair[1], pair[0]) {
            t.Errorf("expected %q before %q", pair[0], pair[1])
        }
    }
}

func TestListAudioFiles(t *testing.T) {
    directory := t.TempDir()
    createFiles(t, directory,
        "Part 10.mp3", "Part 2.flac", "Part 1.MP3", "Part 3.ogg",
        "._Part 4.mp3", "notes.txt", "cover.jpg", "Part 5.m4b",
    )
    if err := os.Mkdir(filepath.Join(directory, "Extras.mp3"), 0o755); err != nil {
        t.Fatal(err)
    }

    files, err := ListAudioFiles(directory)
    if err != nil {
        t.Fatal(err)
    }

    var names []string
    for _, file := range files {
        names = append(names, filepath.Base(file))
    }

    want := []string{"Part 1.MP3", "Part 2.flac", "Part 3.ogg", "Part 10.mp3"}
    if len(names) != len(want) {
        t.Fatalf("got %v, want %v", names, want)
    }
    for i := range want {
        if names[i] != want[i] {
            t.Fatalf("got %v, want %v", names, want)
        }
    }
}

func TestNames(t *testing.T) {
    if got := DisplayTitle("/books/The_Hobbit_-_Part__01.mp3"); got != "The Hobbit - Part 01" {
        t.Errorf("DisplayTitle = %q", got)
    }

    if got := DisplayTitle("/books/Ch. 1 Mr. Bilbo.mp3"); got != "Ch. 1 Mr. Bilbo" {
        t.Errorf("DisplayTitle should keep dots, got %q", got)
    }

    if got := FolderName(filepath.Join("books", "Dune", "part.mp3")); got != "Dune" {
        t.Errorf("FolderName = %q", got)
    }
}

func TestFindCover(t *testing.T) {
    directory := t.TempDir()
    audioPath := filepath.Join(directory, "book.mp3")
    createFiles(t, directory, "book.mp3", "cover.jpg")

    if cover, err := FindCover(audioPath); err != nil || cover != "" {
        t.Fatalf("an unreadable cover.jpg should be ignored, got %q, %v", cover, err)
    }

    artwork, err := os.Create(filepath.Join(directory, "artwork.png"))
    if err != nil {
        t.Fatal(err)
    }
    if err := png.Encode(artwork, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
        t.Fatal(err)
    }
    artwork.Close()

    cover, err := FindCover(audioPath)
    if err != nil || filepath.Base(cover) != "artwork.png" {
        t.Fatalf("got %q, %v, want artwork.png", cover, err)
    }
}

func TestBookFromFolder(t *testing.T) {
    root := t.TempDir()
    folder := filepath.Join(root, "The_Hobbit")
    if err := os.Mkdir(folder, 0o755); err != nil {
        t.Fatal(err)
    }
    createFiles(t, folder, "Part 1.mp3", "Part 2.mp3", "Part 10.mp3")

    book, err := OpenFolder(folder)
    if err != nil {
        t.Fatal(err)
    }

    if !book.IsMultiPart() || len(book.Parts) != 3 {
        t.Fatalf("expected 3 parts, got %v", book.Parts)
    }
    if book.Title() != "The Hobbit" || book.Subtitle() != "Part 1 of 3 · Part 1" {
        t.Errorf("title %q, subtitle %q", book.Title(), book.Subtitle())
    }

    book.Index = 2
    if book.Subtitle() != "Part 3 of 3 · Part 10" {
        t.Errorf("subtitle %q", book.Subtitle())
    }
    if book.HasPart(3) || book.HasPart(-1) || !book.HasPart(0) {
        t.Error("HasPart is wrong at the edges")
    }

    index, found := book.Find(filepath.ToSlash(filepath.Join(folder, "Part 2.mp3")))
    if !found || index != 1 {
        t.Errorf("Find with forward slashes: %d, %v", index, found)
    }

    if book.BrowseFolder() != folder || book.LibraryFolder() != root {
        t.Errorf("browse %q, library %q", book.BrowseFolder(), book.LibraryFolder())
    }
}

func TestSingleFileBook(t *testing.T) {
    path := filepath.Join(t.TempDir(), "Dune", "Dune_Unabridged.mp3")
    book := SingleFile(path)

    if book.IsMultiPart() || book.Folder != "" {
        t.Fatal("a single file should not be a multi-part book")
    }
    if book.Title() != "Dune Unabridged" || book.Subtitle() != "Dune" {
        t.Errorf("title %q, subtitle %q", book.Title(), book.Subtitle())
    }
    if book.BrowseFolder() != filepath.Dir(path) {
        t.Errorf("browse folder %q", book.BrowseFolder())
    }
}

func TestOpenFolderWithoutAudio(t *testing.T) {
    directory := t.TempDir()
    createFiles(t, directory, "notes.txt")

    if _, err := OpenFolder(directory); err == nil {
        t.Fatal("expected an error for a folder without audio files")
    }
}

func TestListAudioFilesIsSorted(t *testing.T) {
    directory := t.TempDir()
    createFiles(t, directory, "b.mp3", "a.mp3", "c.mp3")

    files, err := ListAudioFiles(directory)
    if err != nil {
        t.Fatal(err)
    }

    if !sort.SliceIsSorted(files, func(i, j int) bool { return NaturalLess(files[i], files[j]) }) {
        t.Fatalf("not sorted: %v", files)
    }
}
