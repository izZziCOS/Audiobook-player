package library

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

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
