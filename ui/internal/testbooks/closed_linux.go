package testbooks

import (
	"os"
	"path/filepath"
)

func openFiles() []string {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return nil
	}

	var paths []string
	for _, entry := range entries {
		if target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name())); err == nil {
			paths = append(paths, target)
		}
	}

	return paths
}
