package shelf

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/ui/internal/testbooks"
)

func menuItem(t *testing.T, menu *fyne.Menu, label string) *fyne.MenuItem {
	t.Helper()

	for _, item := range menu.Items {
		if item.Label == label {
			return item
		}
	}

	t.Fatalf("no %q in the menu %v", label, labels(menu))
	return nil
}

func labels(menu *fyne.Menu) []string {
	var names []string
	for _, item := range menu.Items {
		if !item.IsSeparator {
			names = append(names, item.Label)
		}
	}

	return names
}

func overlayObject[T fyne.CanvasObject](s *Shelf, match func(T) bool) T {
	var none T
	top := s.window.Canvas().Overlays().Top()
	if top == nil {
		return none
	}

	for _, object := range test.LaidOutObjects(top) {
		if found, ok := object.(T); ok && match(found) {
			return found
		}
	}

	return none
}

func TestCollectionsInTheLibrary(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	created, err := s.data.CreateCollection("Classics")
	if err != nil {
		t.Fatal(err)
	}
	s.ShowCollection(created)

	expectShown(t, s)
	if s.collectionButton.Text != "Classics" || !s.noMatches.Visible() || !strings.HasPrefix(s.noMatches.Text, "“Classics” is empty") {
		t.Fatalf("an empty collection should say so, got %q", s.noMatches.Text)
	}

	s.ShowCollection("")
	menu := s.cardMenu(s.find(hobbit))
	if want := []string{"Play", "About this book…", "Edit details…", "Add to collection", "Remove from library…"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("in All books a card offers %v, want %v", labels(menu), want)
	}
	choice := menuItem(t, menuItem(t, menu, "Add to collection").ChildMenu, "Classics")
	if choice.Checked {
		t.Fatal("the book isn't in Classics yet")
	}
	choice.Action()

	s.ShowCollection("Classics")
	expectShown(t, s, "The Hobbit")
	if s.count.Text != "1 book" || s.noMatches.Visible() {
		t.Fatalf("count %q", s.count.Text)
	}
	if !menuItem(t, menuItem(t, s.cardMenu(s.find(hobbit)), "Add to collection").ChildMenu, "Classics").Checked {
		t.Fatal("the collections a book is in should be ticked")
	}

	s.setShelfFilter(showFinished)
	expectShown(t, s)
	if s.noMatches.Text != "No finished books yet" {
		t.Fatalf("filters should work inside a collection, got %q", s.noMatches.Text)
	}
	s.setShelfFilter(showAll)
	s.search.SetText("dune")
	expectShown(t, s)
	s.search.SetText("")

	menuItem(t, s.cardMenu(s.find(hobbit)), "Remove from “Classics”").Action()
	expectShown(t, s)
	if !s.Contains(hobbit) {
		t.Fatal("removing a book from a collection should keep it in the library")
	}

	s.ShowCollection("")
	if len(shownTitles(s)) != 3 || s.collectionButton.Text != "All books" {
		t.Fatal("All books should show the whole library again")
	}
}

func TestAddingBooksWhileACollectionIsOpen(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.data.CreateCollection("For bedtime")
	s.ShowCollection("For bedtime")

	s.AddFrom([]string{hobbit})
	expectShown(t, s, "The Hobbit")

	s.ShowCollection("")
	s.AddFrom([]string{root})
	s.ShowCollection("For bedtime")
	expectShown(t, s, "The Hobbit")
	if len(s.entries) != 3 {
		t.Fatal("books added from All books should only join the library")
	}
}

func TestCollectionsMenu(t *testing.T) {
	s, _ := newTestShelf(t)
	for _, name := range []string{"Zebra", "apple", "Book 10", "Book 2"} {
		s.data.CreateCollection(name)
	}

	menu := s.collectionsMenu()
	if want := []string{"All books", "apple", "Book 2", "Book 10", "Zebra", "New collection…"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("the menu shows %v, want %v", labels(menu), want)
	}
	if !menu.Items[0].Checked {
		t.Fatal("All books should be ticked")
	}

	menuItem(t, menu, "Zebra").Action()
	menu = s.collectionsMenu()
	if !menuItem(t, menu, "Zebra").Checked || menuItem(t, menu, "Delete “Zebra”…") == nil || menuItem(t, menu, "Rename “Zebra”…") == nil {
		t.Fatalf("an open collection can be renamed and deleted, menu %v", labels(menu))
	}

	s.data.DeleteCollection("Zebra")
	s.ShowCollection("Zebra")
	if s.collection != "" || s.collectionButton.Text != "All books" {
		t.Fatal("a collection that no longer exists should fall back to All books")
	}

	long := "Books my grandmother recommended for long winter evenings"
	s.data.CreateCollection(long)
	s.ShowCollection(long)
	if text := s.collectionButton.Text; len([]rune(text)) != collectionButtonLength || !strings.HasSuffix(text, "…") {
		t.Fatalf("a long name should be shortened on the button, got %q", text)
	}
}

func TestCreatingACollectionInTheDialog(t *testing.T) {
	s, _ := newTestShelf(t)
	s.window.Resize(fyne.NewSize(620, 780))
	s.data.CreateCollection("Classics")

	menuItem(t, s.collectionsMenu(), "New collection…").Action()

	entry := overlayObject(s, func(*widget.Entry) bool { return true })
	create := overlayObject(s, func(button *widget.Button) bool { return button.Text == "Create" })
	if entry == nil || create == nil {
		t.Fatal("New collection should ask for a name")
	}

	message := overlayObject(s, func(text *canvas.Text) bool { return text.Text == collectionTip })
	if !create.Disabled() || message == nil {
		t.Fatal("an empty name can't be created, and the tip should show")
	}

	test.Type(entry, "classics")
	if !create.Disabled() || message.Text != "There's already a collection called “Classics”" || message.Color != theme.Color(theme.ColorNameError) {
		t.Fatalf("a name that is taken should be refused straight away, the message says %q", message.Text)
	}

	entry.SetText("")
	test.Type(entry, "For bedtime")
	if create.Disabled() || message.Text != collectionTip {
		t.Fatal("a new name should be accepted")
	}
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if s.collection != "For bedtime" || !slices.Contains(s.data.Collections(), "For bedtime") {
		t.Fatalf("pressing Return should create and open the collection, showing %q", s.collection)
	}
	if top := s.window.Canvas().Overlays().Top(); top != nil && top.Visible() {
		t.Fatal("the dialog should close")
	}
}

func TestNewCollectionFromABook(t *testing.T) {
	s, _ := newTestShelf(t)
	root, _ := testbooks.Library(t)
	s.AddFrom([]string{root})
	dune := filepath.Join(root, "Herbert", "Dune.m4b")

	menuItem(t, menuItem(t, s.cardMenu(s.find(dune)), "Add to collection").ChildMenu, "New collection…").Action()
	entry := overlayObject(s, func(*widget.Entry) bool { return true })
	test.Type(entry, "Science fiction")
	test.Tap(overlayObject(s, func(button *widget.Button) bool { return button.Text == "Create" }))

	if !s.data.InCollection("Science fiction", dune) || s.collection != "" {
		t.Fatal("a collection made from a book's menu should contain that book, and leave the view as it was")
	}
}

func TestHeaderButtonsFitTheirText(t *testing.T) {
	s, _ := newTestShelf(t)
	s.window.Resize(fyne.NewSize(620, 780))
	root, _ := testbooks.Library(t)
	s.AddFrom([]string{root})
	s.data.CreateCollection("For bedtime")

	fits := func(button *widget.Button) bool {
		return button.Size().Width >= button.MinSize().Width
	}

	s.setShelfSort(sortTitle)
	s.setShelfSort(sortRecent)
	s.ShowCollection("For bedtime")
	if !fits(s.sortButton) || !fits(s.collectionButton) {
		t.Fatalf("a longer label should widen its button: sort %v of %v, collection %v of %v",
			s.sortButton.Size().Width, s.sortButton.MinSize().Width, s.collectionButton.Size().Width, s.collectionButton.MinSize().Width)
	}
}

func TestRemovingABookAsksFirst(t *testing.T) {
	s, _ := newTestShelf(t)
	root, hobbit := testbooks.Library(t)
	s.AddFrom([]string{root})

	button := func(label string) *widget.Button {
		return overlayObject(s, func(button *widget.Button) bool { return button.Text == label })
	}

	menuItem(t, s.cardMenu(s.find(hobbit)), "Remove from library…").Action()
	if !s.Contains(hobbit) {
		t.Fatal("the book should stay until the removal is confirmed")
	}
	cancel := button("Cancel")
	if cancel == nil || button("Remove") == nil {
		t.Fatal("expected a dialog with Cancel and Remove")
	}
	test.Tap(cancel)
	if !s.Contains(hobbit) || !slices.Contains(pathsOnDisk(t, s), hobbit) {
		t.Fatal("Cancel should keep the book")
	}

	menuItem(t, s.cardMenu(s.find(hobbit)), "Remove from library…").Action()
	test.Tap(button("Remove"))
	if s.Contains(hobbit) || slices.Contains(pathsOnDisk(t, s), hobbit) {
		t.Fatal("Remove should take the book out of the library")
	}
}
