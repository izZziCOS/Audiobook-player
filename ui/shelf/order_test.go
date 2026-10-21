package shelf

import (
    "path/filepath"
    "slices"
    "strings"
    "testing"

    "fyne.io/fyne/v2/test"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/online"
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

func TestRatingsOnCardsAndSortingByRating(t *testing.T) {
    s, _ := newTestShelf(t)
    root, hobbit := testbooks.Library(t)
    s.AddFrom([]string{root})
    dune := filepath.Join(root, "Herbert", "Dune.m4b")
    messiah := filepath.Join(root, "Herbert", "Dune Messiah.m4b")

    s.data.LinkOnline(hobbit, "us", online.Edition{ASIN: "B0099SMFVQ", Title: "The Hobbit", Rating: 4.8, Ratings: 41000})
    s.data.LinkOnline(dune, "us", online.Edition{ASIN: "B002V1OF70", Title: "Dune", Rating: 4.6, Ratings: 150000})
    s.data.LinkOnline(messiah, "us", online.Edition{ASIN: "B002V5D7RU", Title: "Dune Messiah", Rating: 5, Ratings: 12})
    s.Refresh()

    if card, _ := s.Card(dune); card.Rating != "★ 4.6 · 150k" {
        t.Fatalf("the card shows %q", card.Rating)
    }

    s.setShelfSort(sortRating)
    if titles := shelfTitles(s); !slices.Equal(titles, []string{"The Hobbit", "Dune", "Dune Messiah"}) {
        t.Fatalf("sorted by rating: %v, want well-rated books before a 5.0 from 12 people", titles)
    }

    s.data.UnlinkOnline(dune)
    s.Refresh()
    if card, _ := s.Card(dune); card.Rating != "" {
        t.Fatalf("an unlinked book shouldn't show a rating, got %q", card.Rating)
    }
    if titles := shelfTitles(s); titles[len(titles)-1] != "Dune" {
        t.Fatalf("books without a rating should come last, got %v", titles)
    }
}

func TestResettingProgress(t *testing.T) {
    s, _ := newTestShelf(t)
    root, hobbit := testbooks.Library(t)
    s.AddFrom([]string{root})
    var restarted []string
    s.OnRestart = func(path string) { restarted = append(restarted, path) }

    if slices.Contains(labels(s.cardMenu(s.find(hobbit))), "Reset progress…") {
        t.Fatal("a book that hasn't been started has no progress to reset")
    }

    s.data.Positions[hobbit] = userdata.Position{Part: filepath.Join(hobbit, "Part 2.flac"), Position: 3, Listened: 100, Finished: true}
    s.Refresh()
    menuItem(t, s.cardMenu(s.find(hobbit)), "Reset progress…").Action()
    if s.data.Positions[hobbit].Finished == false {
        t.Fatal("nothing should change until the reset is confirmed")
    }
    test.Tap(overlayObject(s, func(button *widget.Button) bool { return button.Text == "Reset progress" }))

    position := s.data.Positions[hobbit]
    if position.Part != filepath.Join(hobbit, "Part 1.wav") || position.Position != 0 || position.Finished || position.Listened <= 100 {
        t.Fatalf("the book should be back at the start, got %+v", position)
    }
    if card, _ := s.Card(hobbit); !strings.HasPrefix(card.Status, "Not started") || len(restarted) != 1 {
        t.Fatalf("the card shows %q", card.Status)
    }
}

func TestSortingByLengthAndTimeLeft(t *testing.T) {
    s, _ := newTestShelf(t)
    root, hobbit := testbooks.Library(t)
    s.AddFrom([]string{root})
    dune := filepath.Join(root, "Herbert", "Dune.m4b")
    messiah := filepath.Join(root, "Herbert", "Dune Messiah.m4b")

    hobbitLength, _, _ := s.listened(s.find(hobbit))
    duneLength, _, known := s.listened(s.find(dune))
    if !known || hobbitLength == duneLength {
        t.Fatal("setup: the books' lengths should be known and differ")
    }

    s.setShelfSort(sortLength)
    titles := shelfTitles(s)
    if hobbitLength < duneLength && titles[0] != "The Hobbit" || hobbitLength > duneLength && titles[2] != "The Hobbit" {
        t.Fatalf("sorted by length: %v (Hobbit %v, Dune %v)", titles, hobbitLength, duneLength)
    }

    s.data.Positions[dune] = userdata.Position{Part: dune, Position: duneLength * 0.9, Listened: 1}
    s.data.Positions[messiah] = userdata.Position{Part: messiah, Listened: 2, Finished: true}
    s.setShelfSort(sortTimeLeft)
    if titles := shelfTitles(s); titles[0] != "Dune" || titles[2] != "Dune Messiah" {
        t.Fatalf("sorted by time left: %v, want the nearly finished book first and the finished one last", titles)
    }
}

func TestTimeLeftUsesEachBooksSpeed(t *testing.T) {
    s, _ := newTestShelf(t)
    root, hobbit := testbooks.Library(t)
    s.AddFrom([]string{root})
    dune := filepath.Join(root, "Herbert", "Dune.m4b")

    normal, _ := s.timeLeft(s.find(dune))
    s.SpeedOf = func(path string) float64 {
        if path == dune {
            return 2
        }
        return 1
    }
    fast, _ := s.timeLeft(s.find(dune))
    if !closeEnough(fast, normal/2) {
        t.Fatalf("at 2× the time left should halve, got %v from %v", fast, normal)
    }
    if same, _ := s.timeLeft(s.find(hobbit)); same == 0 {
        t.Fatal("other books keep their own speed")
    }
}

func closeEnough(a, b float64) bool {
    return a-b < 0.01 && b-a < 0.01
}
