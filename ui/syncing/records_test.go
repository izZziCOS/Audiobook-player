package syncing

import (
    "path/filepath"
    "testing"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func hobbitAt(root string) userdata.SavedBook {
    return userdata.SavedBook{
        Path: filepath.Join(root, "Tolkien", "The Hobbit"),
        Fingerprint: &library.Fingerprint{Name: "The Hobbit", Parts: []library.PartPrint{
            {Name: "Part 1.mp3", Size: 100}, {Name: "Part 2.mp3", Size: 200}, {Name: "Part 3.mp3", Size: 300},
        }},
    }
}

func lengths(book userdata.SavedBook, seconds ...float64) map[string]userdata.PartDuration {
    durations := map[string]userdata.PartDuration{}
    for i, path := range partPaths(book) {
        durations[path] = userdata.PartDuration{Seconds: seconds[i]}
    }

    return durations
}

func TestAPlaceMovesBetweenComputersWithDifferentFolders(t *testing.T) {
    laptop := hobbitAt(filepath.Join("Volumes", "audiobooks"))
    desktop := hobbitAt(filepath.Join("Z:", "Books"))
    desktop.Fingerprint.Name = "The_Hobbit"

    record, ok := recordFor(laptop, "The Hobbit", userdata.Position{Part: partPaths(laptop)[1], Position: 42, Listened: 1000}, lengths(laptop, 600, 700, 800))
    if !ok || record.Part != "Part 2.mp3" || record.PartIndex != 1 || record.Elapsed != 642 {
        t.Fatalf("got %+v", record)
    }

    position, ok := place(desktop, record, nil)
    if !ok || position.Part != filepath.Join(desktop.Path, "Part 2.mp3") || position.Position != 42 || position.Listened != 1000 {
        t.Fatalf("on the other computer the place is %+v", position)
    }
}

func TestRenamedPartsFallBackToTheTimeIntoTheBook(t *testing.T) {
    laptop := hobbitAt("books")
    reencoded := userdata.SavedBook{
        Path: filepath.Join("other", "The Hobbit"),
        Fingerprint: &library.Fingerprint{Name: "The Hobbit", Parts: []library.PartPrint{
            {Name: "01.m4a", Size: 1}, {Name: "02.m4a", Size: 2},
        }},
    }
    record := Record{Fingerprint: *laptop.Fingerprint, Part: "Part 3.mp3", Position: 10, Elapsed: 1310}

    if _, ok := place(reencoded, record, nil); ok {
        t.Fatal("without the parts' lengths the place can't be worked out")
    }
    position, ok := place(reencoded, record, lengths(reencoded, 1000, 1000))
    if !ok || position.Part != filepath.Join(reencoded.Path, "02.m4a") || position.Position != 310 {
        t.Fatalf("got %+v", position)
    }
}

func TestSingleFileBooks(t *testing.T) {
    dune := userdata.SavedBook{
        Path:        filepath.Join("books", "Dune.m4b"),
        Fingerprint: &library.Fingerprint{Name: "Dune.m4b", Parts: []library.PartPrint{{Name: "Dune.m4b", Size: 5}}},
    }
    if paths := partPaths(dune); len(paths) != 1 || paths[0] != dune.Path {
        t.Fatalf("a single file is its own part, got %v", paths)
    }

    record, ok := recordFor(dune, "Dune", userdata.Position{Part: dune.Path, Position: 7, Listened: 5, Finished: true}, nil)
    if !ok || !record.Finished {
        t.Fatalf("got %+v", record)
    }
}

func TestTheNewestListenWins(t *testing.T) {
    here := hobbitAt("here")
    books := []userdata.SavedBook{here}
    local := map[string]userdata.Position{here.Path: {Part: partPaths(here)[0], Position: 5, Listened: 2000}}

    older := Record{Fingerprint: *here.Fingerprint, Part: "Part 1.mp3", Position: 1, Listened: 1500, Last: true}
    newer := Record{Fingerprint: *here.Fingerprint, Part: "Part 3.mp3", Position: 9, Listened: 3000, Last: true}
    newest := Record{Fingerprint: *here.Fingerprint, Part: "Part 2.mp3", Position: 20, Listened: 4000}
    stranger := Record{Fingerprint: library.Fingerprint{Parts: []library.PartPrint{{Name: "x.mp3", Size: 9}}}, Part: "x.mp3", Listened: 9999}

    changes, last := newerChanges(books, local, nil, []Device{
        {Name: "MacBook", Books: []Record{older, stranger}},
        {Name: "Windows PC", Books: []Record{newer}},
        {Name: "Old laptop", Books: []Record{newest}},
    })

    if len(changes) != 1 || changes[0].Device != "Old laptop" || changes[0].Position.Position != 20 {
        t.Fatalf("got %+v, want the newest listen from any computer", changes)
    }
    if last == nil || last.Device != "Windows PC" {
        t.Fatalf("the book another computer was on most recently should be known, got %+v", last)
    }

    if changes, _ := newerChanges(books, map[string]userdata.Position{here.Path: {Listened: 5000}}, nil, []Device{{Books: []Record{newest}}}); len(changes) != 0 {
        t.Fatal("a place older than this computer's shouldn't replace it")
    }
}
