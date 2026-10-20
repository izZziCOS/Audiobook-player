package userdata

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/izzzicos/audiobook-player/stats"
)

func writeFile(t *testing.T, path string, content string) {
    t.Helper()

    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
        t.Fatal(err)
    }
}

func cache(t *testing.T, data *Data, path string) {
    t.Helper()

    data.CacheDuration(path, 600)
    if _, found := data.Durations[path]; !found {
        t.Fatalf("%s should be cached", path)
    }
}

func TestTidyForgetsOnlyWhatIsGone(t *testing.T) {
    root := t.TempDir()
    disk := filepath.Join(root, "Books")
    unplugged := filepath.Join(root, "USB drive", "Books")

    kept := filepath.Join(disk, "Kept")
    keptPart, deletedPart, changedPart := filepath.Join(kept, "1.mp3"), filepath.Join(kept, "2.mp3"), filepath.Join(kept, "3.mp3")
    offline := filepath.Join(unplugged, "Offline")
    offlinePart := filepath.Join(offline, "1.mp3")
    deletedBook := filepath.Join(disk, "Deleted")
    removedBook := filepath.Join(disk, "Removed")
    offlineRemoved := filepath.Join(unplugged, "Removed offline")
    stray := filepath.Join(root, "Somewhere", "stray.mp3")

    data := New("")
    for _, part := range []string{keptPart, deletedPart, changedPart, offlinePart, filepath.Join(deletedBook, "1.mp3"), filepath.Join(removedBook, "1.mp3"), stray} {
        writeFile(t, part, "audio")
        cache(t, data, part)
    }
    data.Library = []SavedBook{{Path: kept}, {Path: offline}}

    for _, book := range []string{kept, offline, deletedBook, removedBook, offlineRemoved} {
        data.Positions[book] = Position{Position: 12}
    }
    for _, book := range []string{deletedBook, removedBook, offlineRemoved} {
        data.Ignore(book)
    }

    os.Remove(deletedPart)
    writeFile(t, changedPart, "a longer recording")
    os.RemoveAll(deletedBook)
    os.RemoveAll(filepath.Join(root, "USB drive"))

    data.Forget(data.PlanTidy().Find())

    for part, want := range map[string]bool{
        keptPart:    true,
        deletedPart: false,
        changedPart: false,
        offlinePart: true,
        filepath.Join(deletedBook, "1.mp3"): false,
        filepath.Join(removedBook, "1.mp3"): false,
        stray:       false,
    } {
        if _, found := data.Durations[part]; found != want {
            t.Errorf("part %s: kept %v, want %v", part, found, want)
        }
    }

    for book, want := range map[string]bool{
        kept:           true,
        offline:        true,
        deletedBook:    false,
        removedBook:    true,
        offlineRemoved: true,
    } {
        if _, found := data.Positions[book]; found != want {
            t.Errorf("position for %s: kept %v, want %v", book, found, want)
        }
    }
    if data.Ignored[deletedBook] || !data.Ignored[removedBook] || !data.Ignored[offlineRemoved] {
        t.Fatalf("only removed books that were deleted should be forgotten, got %v", data.Ignored)
    }
    if !data.Dirty() {
        t.Fatal("the tidied data should be saved")
    }
}

func TestTidyLeavesWhatChangedMeanwhile(t *testing.T) {
    root := t.TempDir()
    book := filepath.Join(root, "Books", "Deleted")
    part := filepath.Join(book, "1.mp3")
    writeFile(t, part, "audio")

    data := New("")
    cache(t, data, part)
    data.Positions[book] = Position{Position: 12}
    os.RemoveAll(book)

    leftovers := data.PlanTidy().Find()

    writeFile(t, part, "audio again")
    cache(t, data, part)
    data.Library = []SavedBook{{Path: book}}

    data.Forget(leftovers)

    if _, found := data.Durations[part]; !found {
        t.Fatal("a part measured again after the check should stay")
    }
    if _, found := data.Positions[book]; !found {
        t.Fatal("a book added back after the check should keep its place")
    }
    if data.Dirty() {
        t.Fatal("nothing was forgotten, so nothing needs saving")
    }
}

func TestLoadingCompactsOldStatistics(t *testing.T) {
    file := FileIn(t.TempDir())
    data := New(file)
    now := time.Now()
    for daysAgo := range 800 {
        data.Stats.Days[now.AddDate(0, 0, -daysAgo).Format("2006-01-02")] = stats.Day{Listened: 600, Content: 600}
    }
    total := data.Stats.Total()
    data.Save()

    restarted := New(file)
    restarted.Load(testPreferences(t))

    if len(restarted.Stats.Days) > stats.KeepDays || restarted.Stats.Total() != total || !restarted.Dirty() {
        t.Fatalf("%d days kept with %v in total, want at most %d with %v", len(restarted.Stats.Days), restarted.Stats.Total(), stats.KeepDays, total)
    }
    if _, longest := restarted.Stats.Streaks(now); longest != 800 {
        t.Fatalf("the longest streak became %d days", longest)
    }
}
