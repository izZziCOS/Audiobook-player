package shelf

import (
    "cmp"
    "fmt"
    "slices"
    "strings"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/kit"
)

const (
    sortRecent   = "Recently played"
    sortTitle    = "Title"
    sortAuthor   = "Author"
    sortProgress = "Progress"
)

var sortOptions = []string{sortRecent, sortTitle, sortAuthor, sortProgress}

const (
    showAll        = "All"
    showInProgress = "In progress"
    showNotStarted = "Not started"
    showFinished   = "Finished"
)

var showOptions = []string{showAll, showInProgress, showNotStarted, showFinished}

type readingState int

const (
    stateInProgress readingState = iota
    stateNotStarted
    stateFinished
)

func (s *Shelf) readingState(entry *shelfEntry) readingState {
    saved, started := s.data.Positions[entry.path]
    if saved.Finished {
        return stateFinished
    }
    if !started {
        return stateNotStarted
    }
    if saved.Position > 0 {
        return stateInProgress
    }

    if entry.book != nil {
        if index, found := entry.book.Find(saved.Part); found && index > 0 {
            return stateInProgress
        }
    }

    return stateNotStarted
}

func (s *Shelf) passesFilter(entry *shelfEntry) bool {
    switch s.filter {
    case showInProgress:
        return s.readingState(entry) == stateInProgress
    case showNotStarted:
        return s.readingState(entry) == stateNotStarted
    case showFinished:
        return s.readingState(entry) == stateFinished
    default:
        return true
    }
}

func compareNames(a, b string) int {
    a, b = foldForSearch(a), foldForSearch(b)
    switch {
    case a == b:
        return 0
    case library.NaturalLess(a, b):
        return -1
    default:
        return 1
    }
}

func compareAuthors(a, b string) int {
    if (a == "") != (b == "") {
        if a == "" {
            return 1
        }
        return -1
    }

    return compareNames(a, b)
}

func byTitle(a, b *shelfEntry) int {
    return compareNames(a.title, b.title)
}

func (s *Shelf) sortShelf() {
    var order func(a, b *shelfEntry) int

    switch s.order {
    case sortTitle:
        order = byTitle
    case sortAuthor:
        order = func(a, b *shelfEntry) int {
            return cmp.Or(compareAuthors(a.author, b.author), byTitle(a, b))
        }
    case sortProgress:
        states := map[*shelfEntry]readingState{}
        fractions := map[*shelfEntry]float64{}
        for _, entry := range s.entries {
            states[entry] = s.readingState(entry)
            fractions[entry], _ = s.shelfProgress(entry)
        }
        order = func(a, b *shelfEntry) int {
            return cmp.Or(
                cmp.Compare(states[a], states[b]),
                cmp.Compare(fractions[b], fractions[a]),
                byTitle(a, b),
            )
        }
    default:
        order = func(a, b *shelfEntry) int {
            return cmp.Or(
                cmp.Compare(s.data.Positions[b.path].Listened, s.data.Positions[a.path].Listened),
                byTitle(a, b),
            )
        }
    }

    slices.SortStableFunc(s.entries, order)
}

func (s *Shelf) arrangeShelf() {
    s.sortShelf()
    s.showShelf()
}

func (s *Shelf) arrangeShelfSoon() {
    if s.arrangePending {
        return
    }

    s.arrangePending = true
    fyne.Do(func() {
        s.arrangePending = false
        s.arrangeShelf()
    })
}

func (s *Shelf) setShelfSort(option string) {
    if !slices.Contains(sortOptions, option) {
        option = sortRecent
    }

    s.order = option
    s.prefs.SetString(preferenceLibrarySort, option)
    s.sortButton.SetText("Sort: " + option)
    s.arrangeShelf()
}

func (s *Shelf) setShelfFilter(option string) {
    if !slices.Contains(showOptions, option) {
        option = showAll
    }

    s.filter = option
    kit.HighlightChoice(s.filterButtons, s.filter)
    s.showShelf()
}

func (s *Shelf) buildShelfControls() (filters, sorting fyne.CanvasObject) {
    s.filterButtons = map[string]*widget.Button{}
    var choices []fyne.CanvasObject
    for _, option := range showOptions {
        button := widget.NewButton(option, func() {
            s.setShelfFilter(option)
        })
        s.filterButtons[option] = button
        choices = append(choices, button)
    }
    s.filter = showAll
    kit.HighlightChoice(s.filterButtons, s.filter)

    s.sortButton = widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), nil)
    s.sortButton.IconPlacement = widget.ButtonIconTrailingText
    s.sortButton.Importance = widget.LowImportance
    s.sortButton.OnTapped = func() {
        items := make([]*fyne.MenuItem, len(sortOptions))
        for i, option := range sortOptions {
            items[i] = fyne.NewMenuItem(option, func() {
                s.setShelfSort(option)
            })
            items[i].Checked = option == s.order
        }
        kit.ShowMenuBelow(s.window, s.sortButton, fyne.NewMenu("", items...))
    }

    s.order = s.prefs.StringWithFallback(preferenceLibrarySort, sortRecent)
    if !slices.Contains(sortOptions, s.order) {
        s.order = sortRecent
    }
    s.sortButton.SetText("Sort: " + s.order)

    return container.NewHBox(choices...), s.sortButton
}

func noMatchesText(filter, query string) string {
    query = strings.TrimSpace(query)

    books := map[string]string{
        showInProgress: "books in progress",
        showNotStarted: "unstarted books",
        showFinished:   "finished books",
    }[filter]

    switch {
    case query == "" && filter == showNotStarted:
        return "You've started every book"
    case query == "":
        return fmt.Sprintf("No %s yet", books)
    case books == "":
        return fmt.Sprintf("No books match “%s”", query)
    default:
        return fmt.Sprintf("No %s match “%s”", books, query)
    }
}
