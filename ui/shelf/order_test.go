package shelf

import (
    "path/filepath"
    "slices"
    "testing"

    "fyne.io/fyne/v2/test"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func shownTitles(s *Shelf) []string {
    var titles []string
    for _, object := range s.grid.Objects {
        for _, entry := range s.entries {
            if entry.card == object {
                titles = append(titles, entry.title)
            }
        }
    }

    return titles
}

func newSortableLibrary(t *testing.T) (*Shelf, map[string]*shelfEntry) {
    t.Helper()

    s, _ := newTestShelf(t)
    root, hobbit := testbooks.Library(t)
    s.AddFrom([]string{root})

    books := map[string]*shelfEntry{
        "hobbit":  s.find(hobbit),
        "dune":    s.find(filepath.Join(root, "Herbert", "Dune.m4b")),
        "messiah": s.find(filepath.Join(root, "Herbert", "Dune Messiah.m4b")),
    }

    messiah := books["messiah"].path
    s.data.Positions[hobbit] = userdata.Position{Part: filepath.Join(hobbit, "Part 2.flac"), Position: 0.3, Listened: 100}
    s.data.Positions[messiah] = userdata.Position{Part: messiah, Listened: 200}
    s.data.Finish(messiah)

    s.Refresh()
    return s, books
}

func expectShown(t *testing.T, s *Shelf, want ...string) {
    t.Helper()

    if got := shownTitles(s); !slices.Equal(got, want) {
        t.Fatalf("the library shows %v, want %v", got, want)
    }
}

func TestSortingTheLibrary(t *testing.T) {
    s, books := newSortableLibrary(t)

    expectShown(t, s, "Dune Messiah", "The Hobbit", "Dune")

    s.setShelfSort(sortTitle)
    expectShown(t, s, "Dune", "Dune Messiah", "The Hobbit")
    if s.sortButton.Text != "Sort: Title" {
        t.Fatalf("the sort button says %q", s.sortButton.Text)
    }

    s.setShelfSort(sortProgress)
    expectShown(t, s, "The Hobbit", "Dune", "Dune Messiah")

    books["messiah"].author = "Frank Herbert"
    books["hobbit"].author = "J. R. R. Tolkien"
    books["dune"].author = ""
    s.setShelfSort(sortAuthor)
    expectShown(t, s, "Dune Messiah", "The Hobbit", "Dune")

    books["hobbit"].author = ""
    books["dune"].author = "Frank Herbert"
    s.Refresh()
    expectShown(t, s, "Dune", "Dune Messiah", "The Hobbit")
    if books["hobbit"].card.Author != "2 parts" {
        t.Fatalf("a book without an author should show its parts, got %q", books["hobbit"].card.Author)
    }

    if s.prefs.String(preferenceLibrarySort) != sortAuthor {
        t.Fatal("the chosen order should be remembered")
    }

    s.setShelfSort("Shoe size")
    if s.order != sortRecent {
        t.Fatalf("an unknown order should fall back to recently played, got %q", s.order)
    }
}

func TestRememberedSortIsUsed(t *testing.T) {
    app := test.NewApp()
    t.Cleanup(app.Quit)
    app.Preferences().SetString(preferenceLibrarySort, sortProgress)
    s := New(app.NewWindow("Library"), app.Preferences(), userdata.New(""), &kit.Picker{}, func(work func()) {
        work()
    })

    if s.order != sortProgress || s.sortButton.Text != "Sort: Progress" {
        t.Fatalf("the library should open sorted by %q, got %q", sortProgress, s.order)
    }
}

func TestFilteringTheLibrary(t *testing.T) {
    s, _ := newSortableLibrary(t)

    s.setShelfFilter(showInProgress)
    expectShown(t, s, "The Hobbit")
    if s.count.Text != "1 of 3 books" || s.noMatches.Visible() {
        t.Fatalf("count %q", s.count.Text)
    }

    s.setShelfFilter(showNotStarted)
    expectShown(t, s, "Dune")

    s.setShelfFilter(showFinished)
    expectShown(t, s, "Dune Messiah")
    if s.filterButtons[showFinished].Importance != widget.HighImportance || s.filterButtons[showAll].Importance != widget.MediumImportance {
        t.Fatal("the chosen filter should be highlighted")
    }

    s.search.SetText("hobbit")
    expectShown(t, s)
    if !s.noMatches.Visible() || s.noMatches.Text != "No finished books match “hobbit”" {
        t.Fatalf("no matches says %q", s.noMatches.Text)
    }

    s.setShelfFilter(showAll)
    expectShown(t, s, "The Hobbit")
}

func TestNothingMatchesTheFilter(t *testing.T) {
    for _, check := range []struct{ filter, query, want string }{
        {showAll, " dune ", "No books match “dune”"},
        {showInProgress, "", "No books in progress yet"},
        {showFinished, "", "No finished books yet"},
        {showNotStarted, "", "You've started every book"},
        {showNotStarted, "dune", "No unstarted books match “dune”"},
    } {
        if got := noMatchesText(check.filter, check.query); got != check.want {
            t.Errorf("%s / %q: got %q, want %q", check.filter, check.query, got, check.want)
        }
    }
}
