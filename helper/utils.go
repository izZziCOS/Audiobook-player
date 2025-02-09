package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
)

// FormatTime converts seconds into a "hh:mm:ss" string
func FormatTime(seconds float64) string {
	totalMinutes := int(seconds) / 60
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	secondsRemaining := int(seconds) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secondsRemaining)
}

// FindImage searches for an image file in the same directory as the MP3 file
func FindImage(mp3URI fyne.URI) (string, error) {
	mp3Path := mp3URI.Path()
	mp3Dir := filepath.Dir(mp3Path)

	files, err := os.ReadDir(mp3Dir)
	if err != nil {
		return "", fmt.Errorf("error reading directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && isImageFile(file.Name()) {
			fmt.Println("Found image:", file.Name())
			return filepath.Join(mp3Dir, file.Name()), nil
		}
	}

	return "", nil
}

// isImageFile checks if the file has an image extension
func isImageFile(filename string) bool {
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp"}
	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return true
		}
	}
	return false
}