package library

import (
    "path/filepath"
    "strings"
)

func DisplayTitle(path string) string {
    return CleanName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func CleanName(name string) string {
    name = strings.ReplaceAll(name, "_", " ")

    return strings.Join(strings.Fields(name), " ")
}

func FolderName(path string) string {
    folder := filepath.Base(filepath.Dir(path))
    if folder == "." || folder == string(filepath.Separator) {
        return ""
    }

    return folder
}
