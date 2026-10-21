package library

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/izzzicos/audiobook-player/vfs"
)

const ResponseTimeout = 4 * time.Second

var ErrNotResponding = errors.New("the drive or server isn't responding")

var systemFolders = map[string]bool{
	"#recycle":                  true,
	"#snapshot":                 true,
	"$recycle.bin":              true,
	"system volume information": true,
	"lost+found":                true,
}

func SkipFolder(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") || systemFolders[strings.ToLower(name)]
}

func Reachable(path string, timeout time.Duration) error {
	result := make(chan error, 1)
	go func() {
		_, err := vfs.Stat(path)
		result <- err
	}()

	select {
	case err := <-result:
		if errors.Is(err, vfs.ErrNotConnected) {
			return ErrNotResponding
		}
		return err
	case <-time.After(timeout):
		return ErrNotResponding
	}
}

func Unavailable(path string) bool {
	if vfs.IsRemote(path) {
		return !vfs.Connected(path)
	}

	root := volumeRoot(path, runtime.GOOS)
	if root == "" {
		return false
	}

	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return false
	}

	info, err := os.Stat(root)
	if err != nil {
		return true
	}

	return !isMountPoint(root, info)
}

func volumeRoot(path, goos string) string {
	if goos == "windows" {
		volume := windowsVolume(path)
		if volume == "" {
			return ""
		}
		return volume + `\`
	}

	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	switch {
	case goos == "darwin" && len(parts) > 2 && parts[1] == "Volumes":
		return "/" + strings.Join(parts[1:3], "/")
	case len(parts) > 2 && parts[1] == "mnt":
		return "/" + strings.Join(parts[1:3], "/")
	case len(parts) > 3 && parts[1] == "media":
		return "/" + strings.Join(parts[1:4], "/")
	case len(parts) > 4 && parts[1] == "run" && parts[2] == "media":
		return "/" + strings.Join(parts[1:5], "/")
	default:
		return ""
	}
}

func windowsVolume(path string) string {
	path = strings.ReplaceAll(path, "/", `\`)

	if len(path) >= 2 && path[1] == ':' {
		return path[:2]
	}

	if strings.HasPrefix(path, `\\`) {
		parts := strings.SplitN(path[2:], `\`, 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return `\\` + parts[0] + `\` + parts[1]
		}
	}

	return ""
}

func IsNetwork(path string) bool {
	if vfs.IsRemote(path) {
		return true
	}
	if runtime.GOOS == "windows" && strings.HasPrefix(strings.ReplaceAll(path, "/", `\`), `\\`) {
		return true
	}

	return isNetworkVolume(path)
}
