//go:build !darwin && !linux && !windows

package library

import "os"

func isMountPoint(string, os.FileInfo) bool {
	return true
}

func isNetworkVolume(string) bool {
	return false
}

func FreeSpace(string) (int64, bool) {
	return 0, false
}
