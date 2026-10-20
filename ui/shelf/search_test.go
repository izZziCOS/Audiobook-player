package shelf

import (
    "path/filepath"
    "testing"

    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
)

func TestMatchesSearch(t *testing.T) {
    fields := []string{"Wuthering Heights", "Emily Brontë", "wuthering_heights_unabridged.m4b", "Classics"}

    for query, want := range map[string]bool{
        "":                   true,
        "   ":                true,
        "wuthering":          true,
        "HEIGHTS":            true,
        "bronte":             true,
        "Brontë":             true,
        "emily heights":      true,
        "unabridged":         true,
        "classics":           true,
        "heights austen":     false,
        "jane":               false,
        "wuthering_heights":  true,
    } {
        if got := matchesSearch(query, fields...); got != want {
            t.Errorf("matchesSearch(%q) = %v, want %v", query, got, want)
        }
    }
}

func TestSearchingTheLibrary(t *testing.T) {
    s, _ := newTestShelf(t)
    root, hobbit := testbooks.Library(t)
    s.AddFrom([]string{root})

    search := func(query string) []string {
        s.search.SetText(query)

        var titles []string
        for _, object := range s.grid.Objects {
            titles = append(titles, object.(*bookCard).Title)
        }
        return titles
    }

    if titles := search("dune"); len(titles) != 2 {
        t.Fatalf("searching for dune found %v", titles)
    }
    if titles := search("messiah"); len(titles) != 1 || titles[0] != "Dune Messiah" {
        t.Fatalf("searching for messiah found %v", titles)
    }
    if titles := search("tolkien"); len(titles) != 1 || titles[0] != "The Hobbit" {
        t.Fatalf("searching by the parent folder found %v", titles)
    }

    if titles := search("austen"); len(titles) != 0 || !s.noMatches.Visible() || s.noMatches.Text != "No books match “austen”" {
        t.Fatalf("a search with no results showed %v and %q", titles, s.noMatches.Text)
    }

    s.Refresh()
    if len(s.grid.Objects) != 0 {
        t.Fatal("the search should still apply after switching tabs")
    }

    if titles := search(""); len(titles) != 3 || s.noMatches.Visible() {
        t.Fatalf("clearing the search showed %v", titles)
    }

    s.remove(s.find(hobbit))
    s.remove(s.find(filepath.Join(root, "Herbert", "Dune.m4b")))
    s.remove(s.find(filepath.Join(root, "Herbert", "Dune Messiah.m4b")))
    if s.header.Visible() || !s.empty.Visible() {
        t.Fatal("an empty library should hide the search box and show the empty message")
    }
}
