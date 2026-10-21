package library

import (
    "bytes"
    "encoding/binary"
    "image"
    "image/png"
    "os"
    "path/filepath"
    "sort"
    "strings"
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

    want := []string{"Part 1.MP3", "Part 2.flac", "Part 3.ogg", "Part 5.m4b", "Part 10.mp3"}
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

func pngData(t *testing.T, side int) []byte {
    t.Helper()

    var buffer bytes.Buffer
    if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, side, side))); err != nil {
        t.Fatal(err)
    }

    return buffer.Bytes()
}

func writeFile(t *testing.T, path string, data []byte) {
    t.Helper()

    if err := os.WriteFile(path, data, 0o644); err != nil {
        t.Fatal(err)
    }
}

func coverName(cover *Cover) string {
    if cover == nil {
        return "no cover"
    }

    return filepath.Base(cover.Name)
}

func TestFindCover(t *testing.T) {
    directory := t.TempDir()
    audioPath := filepath.Join(directory, "book.mp3")
    createFiles(t, directory, "book.mp3", "cover.jpg")

    if cover, err := FindCover(audioPath, nil); err != nil || cover != nil {
        t.Fatalf("an unreadable cover.jpg should be ignored, got %s, %v", coverName(cover), err)
    }

    writeFile(t, filepath.Join(directory, "artwork.png"), pngData(t, 4))

    cover, err := FindCover(audioPath, nil)
    if err != nil || coverName(cover) != "artwork.png" {
        t.Fatalf("got %s, %v, want artwork.png", coverName(cover), err)
    }
}

func TestFindCoverPriority(t *testing.T) {
    directory := t.TempDir()
    audioPath := filepath.Join(directory, "book.mp3")
    embedded := pngData(t, 2)
    writeFile(t, filepath.Join(directory, "cover.png"), pngData(t, 3))

    if cover, _ := FindCover(audioPath, []byte("not an image")); coverName(cover) != "cover.png" {
        t.Errorf("unreadable embedded artwork should be skipped, got %s", coverName(cover))
    }

    if cover, _ := FindCover(audioPath, embedded); cover == nil || !bytes.Equal(cover.Data, embedded) {
        t.Errorf("embedded artwork should come before cover.png, got %s", coverName(cover))
    }

    writeFile(t, filepath.Join(directory, "book.png"), pngData(t, 4))
    if cover, _ := FindCover(audioPath, embedded); coverName(cover) != "book.png" {
        t.Errorf("an image named like the audio file should come first, got %s", coverName(cover))
    }
}

func id3Frame(id string, data []byte) []byte {
    frame := []byte(id)
    frame = binary.BigEndian.AppendUint32(frame, uint32(len(data)))
    frame = append(frame, 0, 0)

    return append(frame, data...)
}

func id3TextFrame(id, text string) []byte {
    return id3Frame(id, append([]byte{0}, text...))
}

func id3Tag(frames ...[]byte) []byte {
    body := bytes.Join(frames, nil)
    size := len(body)
    header := []byte{
        'I', 'D', '3', 3, 0, 0,
        byte(size >> 21 & 0x7f), byte(size >> 14 & 0x7f), byte(size >> 7 & 0x7f), byte(size & 0x7f),
    }

    return append(header, body...)
}

func TestReadMetadata(t *testing.T) {
    picture := pngData(t, 4)
    attachedPicture := append([]byte{0}, "image/png\x00"...)
    attachedPicture = append(attachedPicture, 3, 0)
    attachedPicture = append(attachedPicture, picture...)

    path := filepath.Join(t.TempDir(), "part.mp3")
    writeFile(t, path, id3Tag(
        id3TextFrame("TIT2", "An Unexpected Party"),
        id3TextFrame("TALB", "The Hobbit"),
        id3TextFrame("TPE1", "J. R. R. Tolkien"),
        id3Frame("APIC", attachedPicture),
    ))

    metadata := ReadMetadata(path)
    if metadata.Title != "An Unexpected Party" || metadata.Album != "The Hobbit" || metadata.Artist != "J. R. R. Tolkien" {
        t.Errorf("got title %q, album %q, artist %q", metadata.Title, metadata.Album, metadata.Artist)
    }
    if !bytes.Equal(metadata.Picture, picture) {
        t.Errorf("embedded picture has %d bytes, want %d", len(metadata.Picture), len(picture))
    }
}

func TestReadMetadataWithoutTags(t *testing.T) {
    directory := t.TempDir()
    createFiles(t, directory, "plain.mp3")

    if metadata := ReadMetadata(filepath.Join(directory, "plain.mp3")); metadata.Title != "" || metadata.Picture != nil {
        t.Errorf("expected no metadata, got %+v", metadata)
    }

    if metadata := ReadMetadata(filepath.Join(directory, "missing.mp3")); metadata.Album != "" {
        t.Errorf("expected no metadata for a missing file, got %+v", metadata)
    }
}

func TestTitlesFromTags(t *testing.T) {
    folder := filepath.Join(t.TempDir(), "hobbit_rip")
    book := &Book{
        Folder: folder,
        Parts:  []string{filepath.Join(folder, "01.mp3"), filepath.Join(folder, "02.mp3")},
        Index:  1,
    }
    tags := Metadata{Title: "An Unexpected Party", Album: "The Hobbit", Artist: "J. R. R. Tolkien"}

    if got := book.Title(tags); got != "The Hobbit" {
        t.Errorf("title %q", got)
    }
    if got := book.Subtitle(tags); got != "Part 2 of 2 · An Unexpected Party" {
        t.Errorf("subtitle %q", got)
    }

    single := SingleFile(filepath.Join(folder, "dune_final.mp3"))
    if got := single.Title(Metadata{Title: "Dune"}); got != "Dune" {
        t.Errorf("single-file title %q", got)
    }
    if got := single.Subtitle(Metadata{Artist: "Frank Herbert"}); got != "Frank Herbert" {
        t.Errorf("single-file subtitle %q", got)
    }
}

func TestBookDurations(t *testing.T) {
    book := &Book{Parts: []string{"a.mp3", "b.mp3", "c.mp3"}, Index: 2}
    if book.HasDurations() || book.Offset() != 0 {
        t.Fatal("a book without durations should report none")
    }

    book.Durations = []float64{100, 200, 300}
    if !book.HasDurations() || book.TotalDuration() != 600 || book.Offset() != 300 {
        t.Fatalf("total %v, offset %v", book.TotalDuration(), book.Offset())
    }

    cases := []struct {
        seconds  float64
        index    int
        position float64
    }{
        {-5, 0, 0},
        {0, 0, 0},
        {99.5, 0, 99.5},
        {100, 1, 0},
        {350, 2, 50},
        {700, 2, 400},
    }

    for _, c := range cases {
        index, position := book.Locate(c.seconds)
        if index != c.index || position != c.position {
            t.Errorf("Locate(%v) = %d, %v, want %d, %v", c.seconds, index, position, c.index, c.position)
        }
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
    if book.Title(Metadata{}) != "The Hobbit" || book.Subtitle(Metadata{}) != "Part 1 of 3 · Part 1" {
        t.Errorf("title %q, subtitle %q", book.Title(Metadata{}), book.Subtitle(Metadata{}))
    }

    book.Index = 2
    if book.Subtitle(Metadata{}) != "Part 3 of 3 · Part 10" {
        t.Errorf("subtitle %q", book.Subtitle(Metadata{}))
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

    if book.Key() != folder {
        t.Errorf("key %q, want the folder %q", book.Key(), folder)
    }
}

func TestSingleFileBook(t *testing.T) {
    path := filepath.Join(t.TempDir(), "Dune", "Dune_Unabridged.mp3")
    book := SingleFile(path)

    if book.IsMultiPart() || book.Folder != "" {
        t.Fatal("a single file should not be a multi-part book")
    }
    if book.Title(Metadata{}) != "Dune Unabridged" || book.Subtitle(Metadata{}) != "Dune" {
        t.Errorf("title %q, subtitle %q", book.Title(Metadata{}), book.Subtitle(Metadata{}))
    }
    if book.BrowseFolder() != filepath.Dir(path) {
        t.Errorf("browse folder %q", book.BrowseFolder())
    }
    if book.Key() != path {
        t.Errorf("key %q, want the file %q", book.Key(), path)
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

func TestFindBooks(t *testing.T) {
    root := t.TempDir()
    for _, path := range []string{
        filepath.Join(root, "Tolkien", "The Hobbit"),
        filepath.Join(root, "Herbert"),
        filepath.Join(root, ".hidden", "Secret"),
        filepath.Join(root, "Empty"),
    } {
        if err := os.MkdirAll(path, 0o755); err != nil {
            t.Fatal(err)
        }
    }
    createFiles(t, filepath.Join(root, "Tolkien", "The Hobbit"), "Part 1.mp3", "Part 2.mp3", "cover.jpg")
    createFiles(t, filepath.Join(root, "Herbert"), "Dune.m4b", "Dune Messiah.M4B")
    createFiles(t, filepath.Join(root, ".hidden", "Secret"), "a.mp3")
    createFiles(t, root, "Loose.m4b")

    books, err := FindBooks(root)
    if err != nil {
        t.Fatal(err)
    }

    var names []string
    for _, book := range books {
        relative, _ := filepath.Rel(root, book)
        names = append(names, filepath.ToSlash(relative))
    }
    sort.Strings(names)

    want := []string{"Herbert/Dune Messiah.M4B", "Herbert/Dune.m4b", "Loose.m4b", "Tolkien/The Hobbit"}
    if strings.Join(names, "|") != strings.Join(want, "|") {
        t.Fatalf("got %v, want %v", names, want)
    }

    single, err := FindBooks(filepath.Join(root, "Loose.m4b"))
    if err != nil || len(single) != 1 {
        t.Fatalf("a single file should be one book, got %v, %v", single, err)
    }
}

func TestOpenBook(t *testing.T) {
    root := t.TempDir()
    createFiles(t, root, "Part 1.mp3", "Part 2.mp3", "notes.txt")

    if book, err := OpenBook(root); err != nil || len(book.Parts) != 2 {
        t.Fatalf("folder: %v, %v", book, err)
    }
    if book, err := OpenBook(filepath.Join(root, "Part 2.mp3")); err != nil || book.IsMultiPart() {
        t.Fatalf("file: %v, %v", book, err)
    }
    if _, err := OpenBook(filepath.Join(root, "notes.txt")); err == nil {
        t.Fatal("a text file is not a book")
    }
    if _, err := OpenBook(filepath.Join(root, "missing")); err == nil {
        t.Fatal("a missing path is not a book")
    }
}

func TestThumbnail(t *testing.T) {
    large, err := Thumbnail(pngData(t, 1000), 320)
    if err != nil || large.Bounds().Dx() != 320 || large.Bounds().Dy() != 320 {
        t.Fatalf("got %v, %v, want a 320px thumbnail", large.Bounds(), err)
    }

    small, err := Thumbnail(pngData(t, 100), 320)
    if err != nil || small.Bounds().Dx() != 100 {
        t.Fatalf("small images should be kept as they are, got %v, %v", small.Bounds(), err)
    }

    if _, err := Thumbnail([]byte("not an image"), 320); err == nil {
        t.Fatal("expected an error for data that is not an image")
    }
}

func TestOffsetOf(t *testing.T) {
    book := &Book{Parts: []string{"a", "b", "c"}, Durations: []float64{10, 20, 30}}
    for index, want := range map[int]float64{0: 0, 1: 10, 2: 30, 3: 0, -1: 0} {
        if got := book.OffsetOf(index); got != want {
            t.Errorf("OffsetOf(%d) = %v, want %v", index, got, want)
        }
    }
}

func TestPrepareCover(t *testing.T) {
    prepared, err := PrepareCover(pngData(t, 2000))
    if err != nil {
        t.Fatal(err)
    }

    config, format, err := image.DecodeConfig(bytes.NewReader(prepared))
    if err != nil || format != "jpeg" || config.Width != customCoverSide || config.Height != customCoverSide {
        t.Fatalf("got a %dx%d %s image (%v), want a %dpx JPEG", config.Width, config.Height, format, err, customCoverSide)
    }

    if _, err := PrepareCover([]byte("not an image")); err == nil {
        t.Fatal("expected an error for data that is not an image")
    }
}

func TestOverride(t *testing.T) {
    tags := Metadata{Title: "Chapter 1", Album: "The Hobit", Artist: "Tolkein"}

    if kept := tags.Override("", ""); kept.Album != "The Hobit" || kept.Artist != "Tolkein" {
        t.Fatalf("empty corrections should keep the tags, got %+v", kept)
    }

    corrected := tags.Override("The Hobbit", "J. R. R. Tolkien")
    book := SingleFile(filepath.Join(t.TempDir(), "track01.mp3"))
    if book.Title(corrected) != "The Hobbit" || corrected.Artist != "J. R. R. Tolkien" || corrected.Title != "Chapter 1" {
        t.Fatalf("got %q by %q, part %q", book.Title(corrected), corrected.Artist, corrected.Title)
    }

    folder := &Book{Folder: "/books/the_hobbit", Parts: []string{"/books/the_hobbit/1.mp3", "/books/the_hobbit/2.mp3"}}
    if title := folder.Title(Metadata{}.Override("The Hobbit", "")); title != "The Hobbit" {
        t.Fatalf("a corrected title should win over the folder name, got %q", title)
    }
}

func TestReadASIN(t *testing.T) {
    path := filepath.Join(t.TempDir(), "part.mp3")
    writeFile(t, path, id3Tag(
        id3TextFrame("TALB", "Frankenstein"),
        id3TextFrame("TXXX", "ASIN\x00b00fqrcm9o"),
    ))
    if asin := ReadMetadata(path).ASIN; asin != "B00FQRCM9O" {
        t.Fatalf("got ASIN %q from the ID3 tag", asin)
    }

    if asin := readASIN(map[string]interface{}{"asin": "not an asin"}); asin != "" {
        t.Fatalf("got ASIN %q from a value that isn't one", asin)
    }
    if asin := readASIN(map[string]interface{}{"ASIN": "B071V2RNK3"}); asin != "B071V2RNK3" {
        t.Fatalf("got ASIN %q from an MP4 or Vorbis field", asin)
    }
}
