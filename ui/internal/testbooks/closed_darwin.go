package testbooks

import (
	"bytes"
	"syscall"
	"unsafe"
)

const getPath = 50

func openFiles() []string {
	var paths []string
	buffer := make([]byte, 1024)

	for fd := 0; fd < 4096; fd++ {
		clear(buffer)
		_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), getPath, uintptr(unsafe.Pointer(&buffer[0])))
		if errno == 0 {
			if end := bytes.IndexByte(buffer, 0); end > 0 {
				paths = append(paths, string(buffer[:end]))
			}
		}
	}

	return paths
}
