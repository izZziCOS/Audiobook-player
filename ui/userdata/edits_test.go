package userdata

import (
    "os"
    "path/filepath"
    "testing"
)

func coverExists(data *Data, name string) bool {
    _, err := os.Stat(filepath.Join(data.coversFolder(), name))
    return err == nil
}

func TestEditsSurviveARestart(t *testing.T) {
    file := FileIn(t.TempDir())
    data := New(file)

    cover, err := data.SaveCover([]byte("cover"))
    if err != nil {
        t.Fatal(err)
    }
    data.SetEdit("/books/Hobbit", BookEdit{Title: "  The   Hobbit ", Author: "J. R. R. Tolkien", Cover: cover})

    restarted := New(file)
    restarted.Load(testPreferences(t))
    edit := restarted.Edit("/books/Hobbit")
    if edit.Title != "The Hobbit" || edit.Author != "J. R. R. Tolkien" || edit.Cover != cover {
        t.Fatalf("after a restart the edit is %+v", edit)
    }
    if got, _ := os.ReadFile(restarted.CoverFile("/books/Hobbit")); string(got) != "cover" {
        t.Fatalf("the custom cover holds %q", got)
    }

    restarted.SetEdit("/books/Hobbit", BookEdit{})
    if restarted.Edit("/books/Hobbit") != (BookEdit{}) || restarted.CoverFile("/books/Hobbit") != "" {
        t.Fatal("an empty edit should go back to the files' own details")
    }
    if coverExists(restarted, cover) {
        t.Fatal("a cover nothing uses any more should be deleted")
    }
}

func TestSharedCoversAreKept(t *testing.T) {
    data := New(FileIn(t.TempDir()))

    first, _ := data.SaveCover([]byte("same picture"))
    second, _ := data.SaveCover([]byte("same picture"))
    if first != second {
        t.Fatalf("the same picture was saved twice, as %q and %q", first, second)
    }

    data.SetEdit("/books/Dune", BookEdit{Cover: first})
    data.SetEdit("/books/Dune Messiah", BookEdit{Cover: second})
    data.SetEdit("/books/Dune", BookEdit{Title: "Dune"})

    if !coverExists(data, first) {
        t.Fatal("a cover another book still uses should be kept")
    }
}

func TestEditsFollowMovedBooks(t *testing.T) {
    data := New("")
    old, moved := filepath.Join("books", "Dracula"), filepath.Join("horror", "Dracula")
    data.SetEdit(old, BookEdit{Author: "Bram Stoker"})

    data.Move(old, moved)

    if data.Edit(old) != (BookEdit{}) || data.Edit(moved).Author != "Bram Stoker" {
        t.Fatal("a moved book should keep its corrected details")
    }
}

func TestDeletedBooksLoseTheirEdits(t *testing.T) {
    root := t.TempDir()
    deleted := filepath.Join(root, "Books", "Deleted")
    kept := filepath.Join(root, "Books", "Kept")
    writeFile(t, filepath.Join(deleted, "1.mp3"), "audio")
    writeFile(t, filepath.Join(kept, "1.mp3"), "audio")

    data := New(FileIn(t.TempDir()))
    data.Library = []SavedBook{{Path: kept}}
    deletedCover, _ := data.SaveCover([]byte("deleted"))
    keptCover, _ := data.SaveCover([]byte("kept"))
    leftover, _ := data.SaveCover([]byte("never used"))
    data.SetEdit(deleted, BookEdit{Title: "Gone", Cover: deletedCover})
    data.SetEdit(kept, BookEdit{Title: "Here", Cover: keptCover})
    os.RemoveAll(deleted)

    data.Forget(data.PlanTidy().Find())

    if data.Edit(deleted) != (BookEdit{}) || data.Edit(kept).Title != "Here" {
        t.Fatal("only books deleted from the disk should lose their edits")
    }
    if coverExists(data, deletedCover) || coverExists(data, leftover) || !coverExists(data, keptCover) {
        t.Fatal("only covers that are still used should be kept")
    }
}
