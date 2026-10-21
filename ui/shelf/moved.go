package shelf

import (
    "os"

    "github.com/izzzicos/audiobook-player/library"
)

var reachable = library.Reachable

func exists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
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
