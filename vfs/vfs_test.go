package vfs

import (
    "errors"
    "io/fs"
    "os"
    "path/filepath"
    "testing"
    "testing/fstest"
)

type memory struct {
    files     fstest.MapFS
    connected bool
    written   map[string]string
}

func (m *memory) Open(name string) (File, error) {
    if !m.connected {
        return nil, ErrNotConnected
    }
    if _, err := m.files.Stat(name); err != nil {
        return nil, err
    }
    temporary := filepath.Join(os.TempDir(), "vfs-test-"+filepath.Base(name))
    os.WriteFile(temporary, m.files[name].Data, 0o644)
    return os.Open(temporary)
}

func (m *memory) Stat(name string) (fs.FileInfo, error) {
    if !m.connected {
        return nil, ErrNotConnected
    }
    if name == "" {
        name = "."
    }
    return m.files.Stat(name)
}

func (m *memory) ReadDir(name string) ([]fs.DirEntry, error) {
    if !m.connected {
        return nil, ErrNotConnected
    }
    if name == "" {
        name = "."
    }
    return m.files.ReadDir(name)
}

func (m *memory) WriteAtomic(name string, data []byte) error {
    m.written[name] = string(data)
    return nil
}

func (m *memory) Connected() bool { return m.connected }

func TestPathsUnderAMountGoToTheServer(t *testing.T) {
    root := filepath.Join(t.TempDir(), "Server")
    server := &memory{
        files:     fstest.MapFS{"Tolkien/The Hobbit/Part 1.mp3": {Data: []byte("audio")}},
        connected: true,
        written:   map[string]string{},
    }
    Mount(root, server)
    t.Cleanup(func() { Unmount(root) })

    hobbit := filepath.Join(root, "Tolkien", "The Hobbit")
    if !IsRemote(hobbit) || IsRemote(t.TempDir()) || IsRemote(root+"-other") {
        t.Fatal("only paths inside the mount are on the server")
    }

    entries, err := ReadDir(hobbit)
    if err != nil || len(entries) != 1 || entries[0].Name() != "Part 1.mp3" {
        t.Fatalf("got %v, %v", entries, err)
    }

    file, err := Open(filepath.Join(hobbit, "Part 1.mp3"))
    if err != nil || file.Name() != filepath.Join(hobbit, "Part 1.mp3") {
        t.Fatalf("a file from the server should keep its full path as its name, got %v", err)
    }
    file.Close()

    WriteAtomic(filepath.Join(root, ".sync", "laptop.json"), []byte("{}"))
    if server.written[".sync/laptop.json"] != "{}" {
        t.Fatalf("writes should reach the server with its own slash paths, got %v", server.written)
    }

    server.connected = false
    if _, err := Stat(hobbit); !errors.Is(err, ErrNotConnected) || Connected(hobbit) {
        t.Fatalf("a disconnected server should say so, got %v", err)
    }
    if _, err := Stat(filepath.Join(hobbit, "missing")); errors.Is(err, fs.ErrNotExist) {
        t.Fatal("while disconnected, nothing should look deleted")
    }
}

func TestLocalPathsAreUnchanged(t *testing.T) {
    path := filepath.Join(t.TempDir(), "book.mp3")
    WriteAtomic(path, []byte("local"))

    data, err := ReadFile(path)
    if err != nil || string(data) != "local" || !Connected(path) {
        t.Fatalf("got %q, %v", data, err)
    }
}
