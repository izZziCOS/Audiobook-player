package library

import (
	"os"

	"golang.org/x/sys/windows"
)

func isMountPoint(string, os.FileInfo) bool {
	return true
}

func isNetworkVolume(path string) bool {
	volume := windowsVolume(path)
	if volume == "" {
		return false
	}

	root, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return false
	}

	return windows.GetDriveType(root) == windows.DRIVE_REMOTE
}

func FreeSpace(path string) (int64, bool) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}

	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(name, &available, &total, &free); err != nil {
		return 0, false
	}

	return int64(available), true
}
