package testbooks

import (
    "io/fs"
    "os"
    "path/filepath"
    "sync/atomic"
    "testing"

    "github.com/izzzicos/audiobook-player/vfs"
)

type Server struct {
    folder    string
    connected atomic.Bool
}

func (s *Server) Disconnect() { s.connected.Store(false) }
func (s *Server) Reconnect()  { s.connected.Store(true) }

func (s *Server) Connected() bool {
    return s.connected.Load()
}

func (s *Server) path(name string) (string, error) {
    if !s.connected.Load() {
        return "", vfs.ErrNotConnected
    }

    return filepath.Join(s.folder, filepath.FromSlash(name)), nil
}

func (s *Server) Open(name string) (vfs.File, error) {
    path, err := s.path(name)
    if err != nil {
        return nil, err
    }

    return os.Open(path)
}

func (s *Server) Stat(name string) (fs.FileInfo, error) {
    path, err := s.path(name)
    if err != nil {
        return nil, err
    }

    return os.Stat(path)
}

func (s *Server) ReadDir(name string) ([]fs.DirEntry, error) {
    path, err := s.path(name)
    if err != nil {
        return nil, err
    }

    return os.ReadDir(path)
}

func (s *Server) WriteAtomic(name string, data []byte) error {
    path, err := s.path(name)
    if err != nil {
        return err
    }
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }

    return os.WriteFile(path, data, 0o644)
}

func RemoteLibrary(t *testing.T) (root, hobbit string, server *Server) {
    t.Helper()

    local, localHobbit := Library(t)
    root = filepath.Join(t.TempDir(), "not-on-disk", "Media server")
    relative, _ := filepath.Rel(local, localHobbit)

    server = &Server{folder: local}
    server.Reconnect()
    vfs.Mount(root, server)
    t.Cleanup(func() { vfs.Unmount(root) })

    return root, filepath.Join(root, relative), server
}

func BackdateRemote(t *testing.T, server *Server) {
    t.Helper()

    Backdate(t, server.folder)
}
