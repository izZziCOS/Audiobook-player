package userdata

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
)

const dataFolderName = "Data"

var earlierFiles = []string{
	fileName,
	"backups",
	"covers",
	"thumbnails",
	"offline",
	"online.json",
	"shelf-cache.json",
}

func Folder(root string) string {
	folder := filepath.Join(root, dataFolderName)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		fyne.LogError("Making the app's data folder failed, so the app's main folder is used instead", err)
		return root
	}

	names := earlierFiles
	if damaged, err := filepath.Glob(filepath.Join(root, "library.damaged-*.json")); err == nil {
		for _, path := range damaged {
			names = append(names, filepath.Base(path))
		}
	}

	for _, name := range names {
		from, to := filepath.Join(root, name), filepath.Join(folder, name)
		if _, err := os.Lstat(from); err != nil {
			continue
		}
		if _, err := os.Lstat(to); err == nil {
			continue
		}
		if err := os.Rename(from, to); err != nil {
			fyne.LogError("Moving "+name+" into the app's data folder failed", err)
		}
	}

	return folder
}
