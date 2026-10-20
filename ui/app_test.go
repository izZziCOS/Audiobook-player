package ui

import (
    "math"
    "os"
    "path/filepath"
    "slices"
    "strings"
    "testing"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/storage"
    "fyne.io/fyne/v2/test"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/store"
    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func newTestUIAt(t *testing.T, file string) *UI {
    t.Helper()

    a := test.NewApp()
    t.Cleanup(a.Quit)

    ui := &UI{
        App:    a,
        Window: a.NewWindow("Audiobook Player"),
        data:   userdata.New(file),
        async: func(work func()) {
            work()
        },
    }
    ui.build()
    ui.Window.SetContent(ui.content)

    return ui
}

func newTestUI(t *testing.T) *UI {
    t.Helper()

    return newTestUIAt(t, userdata.FileIn(t.TempDir()))
}

func restart(t *testing.T, previous *UI) *UI {
    t.Helper()

    ui := newTestUIAt(t, previous.data.File())
    ui.load()
    return ui
}

func closeTo(a, b float64) bool {
    return math.Abs(a-b) < 0.01
}

func partPosition(ui *UI) float64 {
    return ui.player.Elapsed() - ui.player.Book().Offset()
}

func listenTo(ui *UI, path string) {
    ui.playFromShelf(path)
    ui.player.Skip(0)
    start := time.Now()
    ui.statistics.Record(start, true, path, 1)
    ui.statistics.Record(start.Add(time.Second), true, path, 1)
    ui.data.Finish(path)
}

func pathsOnDisk(t *testing.T, ui *UI) []string {
    t.Helper()

    var saved struct {
        Library []userdata.SavedBook `json:"library"`
    }
    if _, err := store.Load(ui.data.File(), &saved); err != nil {
        t.Fatalf("reading the saved library: %v", err)
    }

    var paths []string
    for _, book := range saved.Library {
        paths = append(paths, book.Path)
    }

    return paths
}

func TestTabOrder(t *testing.T) {
    ui := newTestUI(t)

    var titles []string
    for _, item := range ui.tabs.Items {
        titles = append(titles, item.Text)
    }

    if want := []string{"Now Playing", "Library", "Statistics", "Settings"}; !slices.Equal(titles, want) {
        t.Fatalf("tabs %v, want %v", titles, want)
    }

    if ui.tabs.Selected() != ui.playerTab || !ui.player.Backdrop().Visible() {
        t.Fatal("Now Playing should open first, with its backdrop")
    }

    ui.tabs.Select(ui.libraryTab)
    if ui.player.Backdrop().Visible() {
        t.Fatal("the cover's colours should only show behind Now Playing")
    }
}

func TestSwitchingBooksFromTheLibrary(t *testing.T) {
    ui := newTestUI(t)
    root, hobbit := testbooks.Library(t)
    ui.shelf.AddFrom([]string{root})

    if err := ui.playFromShelf(hobbit); err != nil {
        t.Fatal(err)
    }
    if book := ui.player.Book(); book == nil || book.Key() != hobbit || !ui.player.Playing() || ui.tabs.Selected() != ui.playerTab {
        t.Fatal("clicking a book should open it, start playing and show the player")
    }

    if err := ui.player.Seek(ui.player.Book().Durations[0] + 0.3); err != nil {
        t.Fatal(err)
    }

    dune := filepath.Join(root, "Herbert", "Dune.m4b")
    ui.playFromShelf(dune)
    if ui.player.Book().Key() != dune {
        t.Fatalf("expected Dune to be open, got %s", ui.player.Book().Key())
    }

    ui.tabs.Select(ui.libraryTab)

    cards := ui.shelf.Cards()
    if cards[0].Path != dune || cards[1].Path != hobbit {
        t.Fatalf("most recently listened books should come first, got %v", cards)
    }
    if hobbitCard := cards[1]; hobbitCard.Progress <= 0.3 || hobbitCard.Progress >= 1 || !strings.Contains(hobbitCard.Status, "% · ") {
        t.Fatalf("the Hobbit shows %.2f, %q", hobbitCard.Progress, hobbitCard.Status)
    }

    ui.playFromShelf(hobbit)
    if ui.player.Book().Index != 1 || partPosition(ui) < 0.25 {
        t.Fatalf("the Hobbit should resume in part 2, got part %d at %.2f", ui.player.Book().Index+1, partPosition(ui))
    }

    ui.tabs.Select(ui.libraryTab)
    ui.playFromShelf(hobbit)
    if ui.tabs.Selected() != ui.playerTab || !ui.player.Playing() {
        t.Fatal("clicking the book that is already open should show the player and keep playing")
    }
}

func TestLibraryAndProgressSurviveARestart(t *testing.T) {
    ui := newTestUI(t)
    root, hobbit := testbooks.Library(t)
    ui.shelf.AddFrom([]string{root})
    ui.playFromShelf(hobbit)
    if err := ui.player.Seek(ui.player.Book().Durations[0] + 0.3); err != nil {
        t.Fatal(err)
    }
    start := time.Now()
    ui.statistics.Record(start, true, hobbit, 1)
    ui.statistics.Record(start.Add(time.Second), true, hobbit, 1)
    ui.save()

    restarted := restart(t, ui)

    if len(restarted.shelf.Cards()) != 3 {
        t.Fatalf("the library should come back, got %v", restarted.shelf.Cards())
    }
    if book := restarted.player.Book(); book == nil || book.Key() != hobbit || book.Index != 1 || !closeTo(partPosition(restarted), 0.3) {
        t.Fatal("the last book should resume where it was left")
    }
    if restarted.data.Stats.Total() != 1 {
        t.Fatalf("listening statistics should come back, got %v", restarted.data.Stats.Total())
    }

    for _, saved := range restarted.data.Library {
        if saved.Path == hobbit && (saved.Fingerprint == nil || len(saved.Fingerprint.Parts) != 2) {
            t.Fatal("each book's fingerprint should be saved")
        }
    }
}

func TestUpgradeFromPreferences(t *testing.T) {
    ui := newTestUI(t)
    folder, parts := testbooks.Book(t)

    prefs := ui.App.Preferences()
    prefs.SetString("bookPositions", `{"`+strings.ReplaceAll(folder, `\`, `\\`)+`":{"part":"`+strings.ReplaceAll(parts[1], `\`, `\\`)+`","position":0.2}}`)
    prefs.SetStringList("library", []string{folder})
    prefs.SetString("bookFolder", folder)
    prefs.SetString("lastFile", storage.NewFileURI(parts[1]).String())

    ui.load()

    if len(ui.shelf.Cards()) != 1 || ui.player.Book() == nil || ui.player.Book().Index != 1 || !closeTo(partPosition(ui), 0.2) {
        t.Fatal("the old library and position should be used")
    }
}

func TestOldestUpgradeKeepsTheLastPosition(t *testing.T) {
    ui := newTestUI(t)
    _, parts := testbooks.Book(t)

    prefs := ui.App.Preferences()
    prefs.SetString("lastFile", storage.NewFileURI(parts[2]).String())
    prefs.SetFloat("lastPosition", 0.4)

    ui.load()

    if ui.player.Book() == nil || !closeTo(partPosition(ui), 0.4) {
        t.Fatal("upgrading from the first version should keep the single saved position")
    }
}

func TestSaveFailureIsShown(t *testing.T) {
    blocker := filepath.Join(t.TempDir(), "not-a-folder")
    os.WriteFile(blocker, []byte("x"), 0o600)
    ui := newTestUIAt(t, filepath.Join(blocker, "library.json"))

    ui.data.Save()

    if ui.Window.Canvas().Overlays().Top() == nil {
        t.Fatal("a failed save should be shown to the user")
    }
}

func TestMovedBookIsFoundAgain(t *testing.T) {
    ui := newTestUI(t)
    root, hobbit := testbooks.Library(t)
    testbooks.Backdate(t, root)
    ui.importer.SetFolder(root)
    listenTo(ui, hobbit)
    ui.playFromShelf(filepath.Join(root, "Herbert", "Dune.m4b"))

    moved := filepath.Join(root, "Fantasy", "The_Hobbit")
    os.MkdirAll(filepath.Dir(moved), 0o755)
    if err := os.Rename(hobbit, moved); err != nil {
        t.Fatal(err)
    }

    ui.importer.Scan()

    if len(ui.shelf.Cards()) != 3 || ui.shelf.Contains(hobbit) || !ui.shelf.Contains(moved) {
        t.Fatalf("the moved book should be relinked, not added again: %v", ui.shelf.Cards())
    }
    if _, found := ui.data.Positions[moved]; !found || ui.data.Stats.Books[moved] != 1 || ui.data.Stats.Completed[moved] == "" {
        t.Fatal("progress, listening time and finished status should follow the book")
    }
    if _, found := ui.data.Positions[hobbit]; found || ui.data.Stats.Books[hobbit] != 0 {
        t.Fatal("nothing should be left under the old path")
    }
    if !slices.Contains(pathsOnDisk(t, ui), moved) {
        t.Fatal("the new location should be saved")
    }
}

func TestMovedLibraryFolder(t *testing.T) {
    ui := newTestUI(t)
    root, hobbit := testbooks.Library(t)
    testbooks.Backdate(t, root)
    ui.importer.SetFolder(root)
    listenTo(ui, hobbit)
    ui.playFromShelf(filepath.Join(root, "Herbert", "Dune.m4b"))
    ui.shelf.Remove(filepath.Join(ui.importer.Folder(), "Herbert", "Dune Messiah.m4b"))

    newRoot := filepath.Join(t.TempDir(), "Audiobooks")
    if err := os.Rename(root, newRoot); err != nil {
        t.Fatal(err)
    }

    ui.importer.SetFolder(newRoot)

    newHobbit := filepath.Join(newRoot, "Tolkien", "The_Hobbit")
    if len(ui.shelf.Cards()) != 2 || !ui.shelf.Contains(newHobbit) || ui.data.Stats.Books[newHobbit] != 1 {
        t.Fatalf("books should follow the library folder: %v", ui.shelf.Cards())
    }
    if !ui.data.Ignored[filepath.Join(newRoot, "Herbert", "Dune Messiah.m4b")] {
        t.Fatal("a removed book should stay removed after the folder moves")
    }
}

func TestMovedBookAddedByHand(t *testing.T) {
    ui := newTestUI(t)
    _, hobbit := testbooks.Library(t)
    ui.shelf.AddFrom([]string{hobbit})
    listenTo(ui, hobbit)

    elsewhere := filepath.Join(t.TempDir(), "The_Hobbit")
    if err := os.Rename(hobbit, elsewhere); err != nil {
        t.Fatal(err)
    }

    ui.shelf.AddFrom([]string{filepath.Dir(elsewhere)})

    if len(ui.shelf.Cards()) != 1 || !ui.shelf.Contains(elsewhere) || ui.data.Stats.CompletedCount() != 1 {
        t.Fatalf("adding the moved book should relink it: %v", ui.shelf.Cards())
    }
}

func TestOpeningFilesFromTheCommandLine(t *testing.T) {
    ui := newTestUI(t)
    folder, parts := testbooks.Book(t)

    ui.openArguments([]string{"-psn_0_12345", filepath.Join(folder, "missing.mp3"), parts[1]})
    if ui.player.Book() == nil || ui.player.Book().Current() != parts[1] || ui.tabs.Selected() != ui.playerTab {
        t.Fatal("the first audio file given should open in the player")
    }

    ui.openArguments([]string{folder})
    if ui.player.Book() == nil || ui.player.Book().Key() != folder {
        t.Fatal("a folder given on the command line should open as a book")
    }

    ui.tabs.Select(ui.libraryTab)
    ui.openArguments(nil)
    if ui.tabs.Selected() != ui.libraryTab {
        t.Fatal("no arguments should leave the window as it is")
    }
}

func TestOpenedBooksJoinTheLibrary(t *testing.T) {
    ui := newTestUI(t)
    folder, _ := testbooks.Book(t)

    ui.player.OpenFolder(folder)

    if !ui.shelf.Contains(folder) {
        t.Fatal("a book opened in the player should be added to the library")
    }
}

func TestSearchShortcutFocusesTheSearchBox(t *testing.T) {
    ui := newTestUI(t)
    root, _ := testbooks.Library(t)
    ui.shelf.AddFrom([]string{root})
    ui.tabs.Select(ui.playerTab)

    ui.focusLibrarySearch()

    entry, focused := ui.Window.Canvas().Focused().(*widget.Entry)
    if ui.tabs.Selected() != ui.libraryTab || !focused || !strings.HasPrefix(entry.PlaceHolder, "Search") {
        t.Fatal("the shortcut should show the library and focus the search box")
    }
}

func TestDroppingFilesOnTheWindow(t *testing.T) {
    ui := newTestUI(t)
    folder, parts := testbooks.Book(t)
    root, _ := testbooks.Library(t)

    ui.handleDrop(fyne.Position{}, []fyne.URI{storage.NewFileURI(parts[0])})
    if ui.player.Book() == nil || ui.player.Book().Current() != parts[0] {
        t.Fatal("a file dropped on the player should open")
    }

    ui.handleDrop(fyne.Position{}, []fyne.URI{storage.NewFileURI(folder)})
    if ui.player.Book().Key() != folder {
        t.Fatal("a folder dropped on the player should open as a book")
    }

    ui.tabs.Select(ui.libraryTab)
    ui.handleDrop(fyne.Position{}, []fyne.URI{storage.NewFileURI(root)})
    for _, book := range []string{filepath.Join(root, "Herbert", "Dune.m4b"), filepath.Join(root, "Tolkien", "The_Hobbit")} {
        if !ui.shelf.Contains(book) {
            t.Fatalf("a folder dropped on the library should add its books, got %v", ui.shelf.Cards())
        }
    }

    ui.tabs.Select(ui.playerTab)
    ui.handleDrop(fyne.Position{}, []fyne.URI{storage.NewFileURI(filepath.Join(root, "notes.txt"))})
    if ui.Window.Canvas().Overlays().Top() == nil {
        t.Fatal("dropping something that can't be played should explain why")
    }
}

func TestPlayingIsSavedAndCounted(t *testing.T) {
    ui := newTestUI(t)
    _, parts := testbooks.Book(t)
    ui.player.OpenFile(parts[0])
    ui.player.TogglePlay()

    start := time.Now()
    ui.everyProgressTick(start)
    ui.everyProgressTick(start.Add(time.Second))
    ui.everySaveTick()

    if total := ui.data.Stats.Total(); !closeTo(total, 1) || ui.data.Dirty() {
        t.Fatalf("the ticks should count listening time and save it, got %vs", total)
    }
}

func TestStartingTidiesDeletedBooks(t *testing.T) {
    ui := newTestUI(t)
    root, hobbit := testbooks.Library(t)
    dune := filepath.Join(root, "Herbert", "Dune.m4b")
    ui.shelf.AddFrom([]string{root})
    ui.playFromShelf(hobbit)
    start := time.Now()
    ui.statistics.Record(start, true, hobbit, 1)
    ui.statistics.Record(start.Add(time.Second), true, hobbit, 1)
    ui.playFromShelf(dune)

    hobbitPart := filepath.Join(hobbit, "Part 1.wav")
    if ui.data.Durations[hobbitPart].Seconds == 0 {
        t.Fatal("the Hobbit's parts should have been measured")
    }

    ui.shelf.Remove(hobbit)
    if err := os.RemoveAll(hobbit); err != nil {
        t.Fatal(err)
    }
    ui.save()

    restarted := restart(t, ui)

    if _, found := restarted.data.Durations[hobbitPart]; found {
        t.Fatal("the part lengths of a deleted book should be forgotten")
    }
    if _, found := restarted.data.Positions[hobbit]; found || restarted.data.Ignored[hobbit] {
        t.Fatal("the place in a deleted book should be forgotten")
    }
    if _, found := restarted.data.Positions[dune]; !found || restarted.data.Stats.Books[hobbit] != 1 || restarted.data.Stats.Total() != 1 {
        t.Fatal("other books and listening statistics should be kept")
    }
}
