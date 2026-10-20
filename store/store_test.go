package store

import (
    "errors"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"
)

type record struct {
    Name  string `json:"name"`
    Count int    `json:"count"`
}

func TestSaveAndLoad(t *testing.T) {
    path := filepath.Join(t.TempDir(), "nested", "library.json")

    if err := Save(path, record{Name: "first", Count: 1}); err != nil {
        t.Fatal(err)
    }
    if err := Save(path, record{Name: "second", Count: 2}); err != nil {
        t.Fatal(err)
    }

    var loaded record
    recovered, err := Load(path, &loaded)
    if err != nil || recovered != "" || loaded.Name != "second" {
        t.Fatalf("got %+v, recovered %q, %v", loaded, recovered, err)
    }

    leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp"))
    if len(leftovers) != 0 {
        t.Fatalf("temporary files left behind: %v", leftovers)
    }
}

func TestSaveKeepsTheOldFileWhenWritingFails(t *testing.T) {
    path := filepath.Join(t.TempDir(), "library.json")
    if err := Save(path, record{Name: "good"}); err != nil {
        t.Fatal(err)
    }

    if err := Save(path, func() {}); err == nil {
        t.Fatal("saving a value that can't be encoded should fail")
    }

    var loaded record
    if _, err := Load(path, &loaded); err != nil || loaded.Name != "good" {
        t.Fatalf("the previous save should be untouched, got %+v, %v", loaded, err)
    }
}

func TestLoadMissingFile(t *testing.T) {
    var loaded record
    if _, err := Load(filepath.Join(t.TempDir(), "library.json"), &loaded); !errors.Is(err, os.ErrNotExist) {
        t.Fatalf("got %v, want a not-exist error", err)
    }
}

func TestDailyBackupsAndRecovery(t *testing.T) {
    path := filepath.Join(t.TempDir(), "library.json")
    day := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.Local)

    for i := 0; i < 10; i++ {
        if err := Save(path, record{Name: "day", Count: i}); err != nil {
            t.Fatal(err)
        }
        if err := Backup(path, day.AddDate(0, 0, i), 7); err != nil {
            t.Fatal(err)
        }
        if err := Backup(path, day.AddDate(0, 0, i), 7); err != nil {
            t.Fatal(err)
        }
    }

    backups := Backups(path)
    if len(backups) != 7 || !strings.HasSuffix(backups[0], "library-2026-10-10.json") || !strings.HasSuffix(backups[6], "library-2026-10-04.json") {
        t.Fatalf("backups %v", backups)
    }

    if err := os.WriteFile(path, []byte(`{"name": "trunc`), 0o600); err != nil {
        t.Fatal(err)
    }

    var loaded record
    recovered, err := Load(path, &loaded)
    if err != nil || !strings.HasSuffix(recovered, "library-2026-10-10.json") || loaded.Count != 9 {
        t.Fatalf("a damaged file should be recovered from the newest backup, got %+v from %q, %v", loaded, recovered, err)
    }

    os.WriteFile(backups[0], []byte("garbage"), 0o600)
    loaded = record{}
    recovered, err = Load(path, &loaded)
    if err != nil || !strings.HasSuffix(recovered, "library-2026-10-09.json") || loaded.Count != 8 {
        t.Fatalf("a damaged backup should be skipped, got %+v from %q, %v", loaded, recovered, err)
    }
}

func TestSetAside(t *testing.T) {
    path := filepath.Join(t.TempDir(), "library.json")
    os.WriteFile(path, []byte("garbage"), 0o600)

    moved, err := SetAside(path, time.Date(2026, time.October, 20, 9, 30, 0, 0, time.Local))
    if err != nil || filepath.Base(moved) != "library.damaged-20261020-093000.json" {
        t.Fatalf("got %q, %v", moved, err)
    }
    if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
        t.Fatal("the damaged file should have been moved")
    }
}
