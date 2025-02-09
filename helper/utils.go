package helper

import (
	"fmt"
	"os"
	"strings"
)

// formatTime converts seconds into a "hh:mm:ss" string
func FormatTime(seconds float64) string {
	totalMinutes := int(seconds) / 60
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	secondsRemaining := int(seconds) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secondsRemaining)
}

func FindImage() (string, error) {
	dir, err := os.Getwd()
	imageName := ""
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return "", err
	}

	// Read files in the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return "", err
	}

	// Loop through files and find images
	for _, file := range files {
		if !file.IsDir() && isImageFile(file.Name()) {
			fmt.Println("Found image:", file.Name())
			imageName = file.Name()
		}
	}
	return imageName, err
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