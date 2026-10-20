package userdata

import (
    "os"
    "path/filepath"
    "slices"
    "strings"
    "testing"
)

func TestCollections(t *testing.T) {
    file := FileIn(t.TempDir())
    data := New(file)

    classics, err := data.CreateCollection("  Classics ")
    if err != nil || classics != "Classics" {
        t.Fatalf("creating a collection gave %q, %v", classics, err)
    }
    data.CreateCollection("For bedtime")

    for name, problem := range map[string]string{
        "":          "enter a name",
        "   ":       "enter a name",
        "classics":  "already a collection called “Classics”",
        "All Books": "whole library",
        strings.Repeat("x", 61): "60 characters",
    } {
        if _, err := data.CreateCollection(name); err == nil || !strings.Contains(err.Error(), problem) {
            t.Errorf("creating %q gave %v, want an error about %q", name, err, problem)
        }
    }

    data.SetInCollection("Classics", "/books/Dracula", true)
    data.SetInCollection("Classics", "/books/Dracula", true)
    data.SetInCollection("For bedtime", "/books/Dracula", true)
    data.SetInCollection("Classics", "/books/Emma", true)
    if !data.InCollection("Classics", "/books/Dracula") || data.InCollection("For bedtime", "/books/Emma") {
        t.Fatal("a book can be in several collections, and only in the ones it was added to")
    }

    if _, err := data.RenameCollection("Classics", "CLASSICS"); err != nil {
        t.Fatalf("changing only the case of a name should work: %v", err)
    }
    if _, err := data.RenameCollection("CLASSICS", "for bedtime"); err == nil {
        t.Fatal("renaming to another collection's name should fail")
    }

    data.SetInCollection("CLASSICS", "/books/Emma", false)
    data.DeleteCollection("For bedtime")

    restarted := New(file)
    restarted.Load(testPreferences(t))
    if names := restarted.Collections(); !slices.Equal(names, []string{"CLASSICS"}) {
        t.Fatalf("after a restart the collections are %v", names)
    }
    if !restarted.InCollection("CLASSICS", "/books/Dracula") || restarted.InCollection("CLASSICS", "/books/Emma") {
        t.Fatal("a collection's books should be saved")
    }
}

func TestCollectionsFollowMovedBooks(t *testing.T) {
    data := New("")
    data.CreateCollection("Classics")
    old, moved := filepath.Join("books", "Dracula"), filepath.Join("horror", "Dracula")
    data.SetInCollection("Classics", old, true)

    data.Move(old, moved)

    if data.InCollection("Classics", old) || !data.InCollection("Classics", moved) {
        t.Fatal("a moved book should stay in its collections")
    }
}

func TestDeletedBooksLeaveTheirCollections(t *testing.T) {
    root := t.TempDir()
    deleted := filepath.Join(root, "Books", "Deleted")
    kept := filepath.Join(root, "Books", "Kept")
    writeFile(t, filepath.Join(deleted, "1.mp3"), "audio")
    writeFile(t, filepath.Join(kept, "1.mp3"), "audio")

    data := New("")
    data.Library = []SavedBook{{Path: kept}}
    data.CreateCollection("Classics")
    data.SetInCollection("Classics", deleted, true)
    data.SetInCollection("Classics", kept, true)
    os.RemoveAll(deleted)

    data.Forget(data.PlanTidy().Find())

    if data.InCollection("Classics", deleted) || !data.InCollection("Classics", kept) {
        t.Fatal("only books deleted from the disk should leave their collections")
    }
}
