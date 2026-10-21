package userdata

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/izzzicos/audiobook-player/store"
)

func TestEarlierDataMovesIntoTheDataFolder(t *testing.T) {
    root := t.TempDir()
    writeFile(t, filepath.Join(root, "library.json"), `{"version": 1}`)
    writeFile(t, filepath.Join(root, "backups", "library-2026-10-20.json"), "{}")
    writeFile(t, filepath.Join(root, "offline", "abc", "Part 1.mp3"), "audio")
    writeFile(t, filepath.Join(root, "library.damaged-20261020-101010.json"), "broken")
    writeFile(t, filepath.Join(root, "preferences.json"), "{}")

    folder := Folder(root)

    for _, name := range []string{"library.json", filepath.Join("backups", "library-2026-10-20.json"), filepath.Join("offline", "abc", "Part 1.mp3"), "library.damaged-20261020-101010.json"} {
        if _, err := os.Stat(filepath.Join(folder, name)); err != nil {
            t.Errorf("%s should have moved into the data folder: %v", name, err)
        }
        if _, err := os.Stat(filepath.Join(root, name)); err == nil {
            t.Errorf("%s should no longer be in the main folder", name)
        }
    }
    if _, err := os.Stat(filepath.Join(root, "preferences.json")); err != nil {
        t.Fatal("Fyne's own preferences should stay where Fyne keeps them")
    }

    writeFile(t, filepath.Join(root, "library.json"), "stale")
    if Folder(root) != folder {
        t.Fatal("the data folder should stay the same")
    }
    if data, _ := os.ReadFile(filepath.Join(folder, "library.json")); string(data) != `{"version": 1}` {
        t.Fatal("a file already in the data folder must never be replaced")
    }
}

func TestSavingDoesntTouchTheMainFolder(t *testing.T) {
    root := t.TempDir()
    folder := Folder(root)
    old := time.Now().Add(-time.Hour)
    os.Chtimes(root, old, old)
    before, _ := os.Stat(root)

    data := New(FileIn(folder))
    for range 3 {
        data.Positions["/books/Dune"] = Position{Part: "/books/Dune/1.mp3", Listened: time.Now().Unix()}
        data.Save()
    }
    store.Backup(data.File(), time.Now(), dailyBackups)

    after, _ := os.Stat(root)
    if !after.ModTime().Equal(before.ModTime()) {
        t.Fatal("saving the library shouldn't change the main folder, which Fyne watches for its settings")
    }
}
