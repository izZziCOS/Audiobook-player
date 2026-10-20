package library

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

    "github.com/izzzicos/audiobook-player/audio"
)

func ListAudioFiles(directory string) ([]string, error) {
    entries, err := os.ReadDir(directory)
    if err != nil {
        return nil, fmt.Errorf("reading folder: %w", err)
    }

    var files []string
    for _, entry := range entries {
        name := entry.Name()
        if entry.IsDir() || strings.HasPrefix(name, ".") {
            continue
        }

        if audio.IsSupported(name) {
            files = append(files, filepath.Join(directory, name))
        }
    }

    sort.Slice(files, func(i, j int) bool {
        return NaturalLess(filepath.Base(files[i]), filepath.Base(files[j]))
    })

    return files, nil
}

func NaturalLess(a, b string) bool {
    a, b = strings.ToLower(a), strings.ToLower(b)

    for a != "" && b != "" {
        aChunk, aRest := nextChunk(a)
        bChunk, bRest := nextChunk(b)

        if aChunk != bChunk {
            if isDigit(aChunk[0]) && isDigit(bChunk[0]) {
                aNumber := strings.TrimLeft(aChunk, "0")
                bNumber := strings.TrimLeft(bChunk, "0")

                if len(aNumber) != len(bNumber) {
                    return len(aNumber) < len(bNumber)
                }
                if aNumber != bNumber {
                    return aNumber < bNumber
                }

                return len(aChunk) < len(bChunk)
            }

            return aChunk < bChunk
        }

        a, b = aRest, bRest
    }

    return len(a) < len(b)
}

func nextChunk(text string) (string, string) {
    digits := isDigit(text[0])

    end := 1
    for end < len(text) && isDigit(text[end]) == digits {
        end++
    }

    return text[:end], text[end:]
}

func isDigit(character byte) bool {
    return character >= '0' && character <= '9'
}

const maxScanDepth = 6

func FindBooks(root string) ([]string, error) {
    root = normalize(root)

    info, err := os.Stat(root)
    if err != nil {
        return nil, err
    }

    if !info.IsDir() {
        if !audio.IsSupported(root) {
            return nil, fmt.Errorf("%q is not a supported audio file", filepath.Base(root))
        }

        return []string{root}, nil
    }

    var books []string
    if err := findBooks(root, 0, &books); err != nil {
        return nil, err
    }

    return books, nil
}

func findBooks(directory string, depth int, books *[]string) error {
    files, err := ListAudioFiles(directory)
    if err != nil {
        return err
    }

    switch {
    case len(files) == 0:
    case allWholeBooks(files):
        *books = append(*books, files...)
    default:
        *books = append(*books, directory)
    }

    if depth >= maxScanDepth {
        return nil
    }

    entries, err := os.ReadDir(directory)
    if err != nil {
        return err
    }

    for _, entry := range entries {
        if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
            _ = findBooks(filepath.Join(directory, entry.Name()), depth+1, books)
        }
    }

    return nil
}

func allWholeBooks(files []string) bool {
    for _, file := range files {
        if !strings.EqualFold(filepath.Ext(file), ".m4b") {
            return false
        }
    }

    return true
}

func Inspect(path string) (int, time.Time, error) {
    info, err := os.Stat(path)
    if err != nil {
        return 0, time.Time{}, err
    }

    if !info.IsDir() {
        return 1, info.ModTime(), nil
    }

    files, err := ListAudioFiles(path)
    if err != nil {
        return 0, time.Time{}, err
    }

    var modified time.Time
    for _, file := range files {
        if fileInfo, err := os.Stat(file); err == nil && fileInfo.ModTime().After(modified) {
            modified = fileInfo.ModTime()
        }
    }

    return len(files), modified, nil
}
