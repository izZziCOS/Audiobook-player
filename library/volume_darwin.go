package library

import "golang.org/x/sys/unix"

var networkFileSystems = map[string]bool{
	"smbfs":  true,
	"nfs":    true,
	"afpfs":  true,
	"webdav": true,
	"cifs":   true,
}

func isNetworkVolume(path string) bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return false
	}

	return networkFileSystems[unix.ByteSliceToString(stat.Fstypename[:])]
}
