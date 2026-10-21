package library

import "golang.org/x/sys/unix"

var networkFileSystems = map[int64]bool{
    0x6969:     true,
    0x517b:     true,
    0xff534d42: true,
    0xfe534d42: true,
}

func isNetworkVolume(path string) bool {
    var stat unix.Statfs_t
    if err := unix.Statfs(path, &stat); err != nil {
        return false
    }

    return networkFileSystems[int64(stat.Type)]
}
