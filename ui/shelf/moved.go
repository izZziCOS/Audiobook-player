package shelf

import (
    "os"

    "github.com/izzzicos/audiobook-player/library"
)

func exists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}

func (s *Shelf) movedBooks() []*shelfEntry {
    var moved []*shelfEntry
    for _, entry := range s.entries {
        if entry.fingerprint != nil && !exists(entry.path) {
            moved = append(moved, entry)
        }
    }

    return moved
}

func matchMoved(candidates []*shelfEntry, path string) *shelfEntry {
    fingerprint, err := library.FingerprintOf(path)
    if err != nil {
        return nil
    }

    for _, entry := range candidates {
        if entry.fingerprint.Matches(fingerprint) {
            return entry
        }
    }

    return nil
}

func locateMoved(path string, known *library.Fingerprint) string {
    if known == nil {
        return ""
    }

    for _, candidate := range library.OtherDrives(path) {
        if !exists(candidate) {
            continue
        }

        if fingerprint, err := library.FingerprintOf(candidate); err == nil && known.Matches(fingerprint) {
            return candidate
        }
    }

    return ""
}
