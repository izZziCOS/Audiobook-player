package vfs

import (
    "errors"
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "strings"
    "sync"
)

var ErrNotConnected = errors.New("the media server isn't connected")

type File interface {
    io.Reader
    io.ReaderAt
    io.Seeker
    io.Closer
    Name() string
    Stat() (fs.FileInfo, error)
}

type FS interface {
    Open(name string) (File, error)
    Stat(name string) (fs.FileInfo, error)
    ReadDir(name string) ([]fs.DirEntry, error)
    WriteAtomic(name string, data []byte) error
    Connected() bool
}

type mount struct {
    root string
    fsys FS
}

var (
    mu     sync.RWMutex
    mounts []mount
)

func Mount(root string, fsys FS) {
    root = filepath.Clean(root)

    mu.Lock()
    defer mu.Unlock()

    for i := range mounts {
        if mounts[i].root == root {
            mounts[i].fsys = fsys
            return
        }
    }
    mounts = append(mounts, mount{root: root, fsys: fsys})
}

func Unmount(root string) {
    root = filepath.Clean(root)

    mu.Lock()
    defer mu.Unlock()

    for i := range mounts {
        if mounts[i].root == root {
            mounts = append(mounts[:i], mounts[i+1:]...)
            return
        }
    }
}

func resolve(path string) (FS, string, bool) {
    mu.RLock()
    defer mu.RUnlock()

    path = filepath.Clean(path)
    for _, m := range mounts {
        if path == m.root {
            return m.fsys, "", true
        }
        if strings.HasPrefix(path, m.root+string(filepath.Separator)) {
            return m.fsys, filepath.ToSlash(path[len(m.root)+1:]), true
        }
    }

    return nil, "", false
}

func IsRemote(path string) bool {
    _, _, remote := resolve(path)
    return remote
}

func Connected(path string) bool {
    fsys, _, remote := resolve(path)
    return !remote || fsys.Connected()
}

func Open(path string) (File, error) {
    if fsys, name, remote := resolve(path); remote {
        file, err := fsys.Open(name)
        if err != nil {
            return nil, pathError("open", path, err)
        }
        return named{File: file, name: path}, nil
    }

    return os.Open(path)
}

func Stat(path string) (fs.FileInfo, error) {
    if fsys, name, remote := resolve(path); remote {
        info, err := fsys.Stat(name)
        if err != nil {
            return nil, pathError("stat", path, err)
        }
        return info, nil
    }

    return os.Stat(path)
}

func ReadDir(path string) ([]fs.DirEntry, error) {
    if fsys, name, remote := resolve(path); remote {
        entries, err := fsys.ReadDir(name)
        if err != nil {
            return nil, pathError("readdir", path, err)
        }
        return entries, nil
    }

    return os.ReadDir(path)
}

func ReadFile(path string) ([]byte, error) {
    file, err := Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    return io.ReadAll(file)
}

func WriteAtomic(path string, data []byte) error {
    if fsys, name, remote := resolve(path); remote {
        if err := fsys.WriteAtomic(name, data); err != nil {
            return pathError("write", path, err)
        }
        return nil
    }

    return writeLocal(path, data)
}

func writeLocal(path string, data []byte) error {
    directory := filepath.Dir(path)
    if err := os.MkdirAll(directory, 0o700); err != nil {
        return err
    }

    temporary, err := os.CreateTemp(directory, filepath.Base(path)+".*.tmp")
    if err != nil {
        return err
    }
    defer os.Remove(temporary.Name())

    if _, err := temporary.Write(data); err != nil {
        temporary.Close()
        return err
    }
    if err := temporary.Sync(); err != nil {
        temporary.Close()
        return err
    }
    if err := temporary.Close(); err != nil {
        return err
    }

    return os.Rename(temporary.Name(), path)
}

func pathError(op, path string, err error) error {
    var existing *fs.PathError
    if errors.As(err, &existing) {
        return &fs.PathError{Op: op, Path: path, Err: existing.Err}
    }

    return &fs.PathError{Op: op, Path: path, Err: err}
}

type named struct {
    File
    name string
}

func (n named) Name() string {
    return n.name
}

func Display(path string) string {
    mu.RLock()
    defer mu.RUnlock()

    path = filepath.Clean(path)
    for _, m := range mounts {
        name := filepath.Base(m.root)
        if path == m.root {
            return name
        }
        if strings.HasPrefix(path, m.root+string(filepath.Separator)) {
            return name + " / " + filepath.ToSlash(path[len(m.root)+1:])
        }
    }

    return path
}
