package library

import (
    "fmt"
    "path/filepath"
)

type Book struct {
    Folder string
    Parts  []string
    Index  int
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

func (b *Book) Title() string {
    if b.IsMultiPart() {
        return CleanName(filepath.Base(b.Folder))
    }

    return DisplayTitle(b.Current())
}

func (b *Book) Subtitle() string {
    if b.IsMultiPart() {
        return fmt.Sprintf(
            "Part %d of %d · %s",
            b.Index+1,
            len(b.Parts),
            DisplayTitle(b.Current()),
        )
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
    return filepath.Clean(filepath.FromSlash(path))
}
