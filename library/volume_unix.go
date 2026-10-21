//go:build darwin || linux

package library

import (
	"os"
	"path/filepath"
	"syscall"
)

func isMountPoint(root string, info os.FileInfo) bool {
	parent, err := os.Stat(filepath.Dir(root))
	if err != nil {
		return true
	}

	rootStat, rootOK := info.Sys().(*syscall.Stat_t)
	parentStat, parentOK := parent.Sys().(*syscall.Stat_t)
	if !rootOK || !parentOK {
		return true
	}

	return rootStat.Dev != parentStat.Dev
}

func FreeSpace(path string) (int64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, false
	}

	return int64(stat.Bavail) * int64(stat.Bsize), true
}
