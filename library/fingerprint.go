package library

import (
    "os"
    "path/filepath"
    "runtime"
    "strings"
)

type PartPrint struct {
    Name string `json:"name"`
    Size int64  `json:"size"`
}

type Fingerprint struct {
    Name  string      `json:"name"`
    Parts []PartPrint `json:"parts"`
}

func FingerprintOf(path string) (Fingerprint, error) {
    book, err := OpenBook(path)
    if err != nil {
        return Fingerprint{}, err
    }

    fingerprint := Fingerprint{Name: filepath.Base(book.Key())}
    for _, part := range book.Parts {
        info, err := os.Stat(part)
        if err != nil {
            return Fingerprint{}, err
        }

        fingerprint.Parts = append(fingerprint.Parts, PartPrint{Name: filepath.Base(part), Size: info.Size()})
    }

    return fingerprint, nil
}

func (f Fingerprint) Matches(candidate Fingerprint) bool {
    if len(f.Parts) == 0 {
        return false
    }

    available := make(map[PartPrint]bool, len(candidate.Parts))
    for _, part := range candidate.Parts {
        available[part] = true
    }

    for _, part := range f.Parts {
        if !available[part] {
            return false
        }
    }

    return f.Name == candidate.Name || len(f.Parts) == len(candidate.Parts)
}

func Normalize(path string) string {
    return canonical(filepath.Clean(filepath.FromSlash(path)), runtime.GOOS == "windows")
}

func canonical(path string, windows bool) string {
    if windows && len(path) >= 2 && path[1] == ':' {
        return strings.ToUpper(path[:1]) + path[1:]
    }

    return path
}

func OtherDrives(path string) []string {
    return otherDrives(path, runtime.GOOS == "windows")
}

func otherDrives(path string, windows bool) []string {
    if !windows || len(path) < 2 || path[1] != ':' {
        return nil
    }

    current := strings.ToUpper(path[:1])
    var candidates []string
    for letter := 'C'; letter <= 'Z'; letter++ {
        if string(letter) != current {
            candidates = append(candidates, string(letter)+path[1:])
        }
    }

    return candidates
}

func Within(path, root string) (string, bool) {
    relative, err := filepath.Rel(root, path)
    if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
        return "", false
    }

    return relative, true
}
