package library

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/izzzicos/audiobook-player/audio"
)

type Book struct {
    Folder    string
    Parts     []string
    Index     int
    Durations []float64
}

func SingleFile(path string) *Book {
    return &Book{Parts: []string{normalize(path)}}
}

func OpenFolder(folder string) (*Book, error) {
    folder = normalize(folder)

    parts, err := ListAudioFiles(folder)
    if err != nil {
        return nil, err
    }

    if len(parts) == 0 {
        return nil, fmt.Errorf("no audio files found in %q", filepath.Base(folder))
    }

    return &Book{Folder: folder, Parts: parts}, nil
}

func OpenBook(path string) (*Book, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }

    if info.IsDir() {
        return OpenFolder(path)
    }

    if !audio.IsSupported(path) {
        return nil, fmt.Errorf("%q is not a supported audio file", filepath.Base(path))
    }

    return SingleFile(path), nil
}

func (b *Book) Key() string {
    if b.Folder != "" {
        return b.Folder
    }

    return b.Parts[0]
}

func (b *Book) Current() string {
    return b.Parts[b.Index]
}

func (b *Book) IsMultiPart() bool {
    return len(b.Parts) > 1
}

func (b *Book) HasPart(index int) bool {
    return index >= 0 && index < len(b.Parts)
}

func (b *Book) Find(path string) (int, bool) {
    path = normalize(path)

    for index, part := range b.Parts {
        if part == path {
            return index, true
        }
    }

    return 0, false
}

func (b *Book) HasDurations() bool {
    return len(b.Durations) == len(b.Parts)
}

func (b *Book) TotalDuration() float64 {
    total := 0.0
    for _, duration := range b.Durations {
        total += duration
    }

    return total
}

func (b *Book) Offset() float64 {
    return b.OffsetOf(b.Index)
}

func (b *Book) OffsetOf(index int) float64 {
    if !b.HasDurations() || !b.HasPart(index) {
        return 0
    }

    offset := 0.0
    for _, duration := range b.Durations[:index] {
        offset += duration
    }

    return offset
}

func (b *Book) Locate(seconds float64) (int, float64) {
    for index, duration := range b.Durations {
        if seconds < duration || index == len(b.Durations)-1 {
            return index, max(seconds, 0)
        }

        seconds -= duration
    }

    return 0, 0
}

func (b *Book) Title(tags Metadata) string {
    if tags.Album != "" {
        return tags.Album
    }

    if b.IsMultiPart() {
        return CleanName(filepath.Base(b.Folder))
    }

    if tags.Title != "" {
        return tags.Title
    }

    return DisplayTitle(b.Current())
}

func (b *Book) Subtitle(tags Metadata) string {
    if b.IsMultiPart() {
        partName := tags.Title
        if partName == "" {
            partName = DisplayTitle(b.Current())
        }

        return fmt.Sprintf(
            "Part %d of %d · %s",
            b.Index+1,
            len(b.Parts),
            partName,
        )
    }

    if tags.Artist != "" {
        return tags.Artist
    }

    return CleanName(FolderName(b.Current()))
}

func (b *Book) BrowseFolder() string {
    if b.Folder != "" {
        return b.Folder
    }

    return filepath.Dir(b.Current())
}

func (b *Book) LibraryFolder() string {
    if b.Folder != "" {
        return filepath.Dir(b.Folder)
    }

    return b.BrowseFolder()
}

func normalize(path string) string {
    return Normalize(path)
}
