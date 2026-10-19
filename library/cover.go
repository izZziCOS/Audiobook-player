package library

import (
    "fmt"
    "image"
    _ "image/gif"
    _ "image/jpeg"
    _ "image/png"
    "os"
    "path/filepath"
    "strings"

    _ "golang.org/x/image/bmp"
    _ "golang.org/x/image/webp"
)

func FindCover(audioPath string) (string, error) {
    directory := filepath.Dir(audioPath)
    audioName := strings.TrimSuffix(
        filepath.Base(audioPath),
        filepath.Ext(audioPath),
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
        if err == nil && !info.IsDir() && isReadableImage(path) {
            return path, nil
        }
    }

    // Otherwise, use the first supported image.
    for _, entry := range entries {
        path := filepath.Join(directory, entry.Name())
        if !entry.IsDir() && isImageFile(entry.Name()) && isReadableImage(path) {
            return path, nil
        }
    }

    return "", nil
}

func isReadableImage(path string) bool {
    file, err := os.Open(path)
    if err != nil {
        return false
    }
    defer file.Close()

    _, _, err = image.DecodeConfig(file)
    return err == nil
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
