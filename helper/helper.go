package helper

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "fyne.io/fyne/v2"
)

func FormatTime(seconds float64) string {
    if seconds < 0 {
        seconds = 0
    }

    totalSeconds := int(seconds)
    hours := totalSeconds / 3600
    minutes := (totalSeconds % 3600) / 60
    remainingSeconds := totalSeconds % 60

    return fmt.Sprintf(
        "%02d:%02d:%02d",
        hours,
        minutes,
        remainingSeconds,
    )
}

func FindImage(mp3URI fyne.URI) (string, error) {
    if mp3URI == nil {
        return "", fmt.Errorf("no audio file selected")
    }

    mp3Path := mp3URI.Path()
    directory := filepath.Dir(mp3Path)
    audioName := strings.TrimSuffix(
        filepath.Base(mp3Path),
        filepath.Ext(mp3Path),
    )

    entries, err := os.ReadDir(directory)
    if err != nil {
        return "", fmt.Errorf("reading audio directory: %w", err)
    }

    // Prefer artwork that matches the audio filename.
    preferred := []string{
        audioName + ".jpg",
        audioName + ".jpeg",
        audioName + ".png",
        "cover.jpg",
        "cover.png",
        "folder.jpg",
        "folder.png",
        "front.jpg",
        "front.png",
    }

    for _, name := range preferred {
        path := filepath.Join(directory, name)

        info, err := os.Stat(path)
        if err == nil && !info.IsDir() {
            return path, nil
        }
    }

    // Otherwise, use the first supported image.
    for _, entry := range entries {
        if !entry.IsDir() && isImageFile(entry.Name()) {
            return filepath.Join(directory, entry.Name()), nil
        }
    }

    return "", nil
}

func isImageFile(filename string) bool {
    extensions := []string{
        ".jpg",
        ".jpeg",
        ".png",
        ".gif",
        ".bmp",
        ".tiff",
        ".webp",
    }

    extension := strings.ToLower(filepath.Ext(filename))

    for _, ext := range extensions {
        if extension == ext {
            return true
        }
    }

    return false
}