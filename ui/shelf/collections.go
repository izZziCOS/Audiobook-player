package shelf

import (
    "errors"
    "fmt"
    "slices"
    "strings"
    "unicode"
    "unicode/utf8"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/audio"
    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/offline"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const (
    collectionButtonLength  = 24
    collectionMessageLength = 64
    removeTitleLength       = 48
    collectionTip           = "A book can be in as many collections as you like."
)

func (s *Shelf) buildCollectionButton() *widget.Button {
    s.collectionButton = widget.NewButtonWithIcon(userdata.AllBooks, theme.FolderOpenIcon(), nil)
    s.collectionButton.OnTapped = func() {
        kit.ShowMenuBelow(s.window, s.collectionButton, s.collectionsMenu())
    }

    return s.collectionButton
}

func (s *Shelf) collectionNames() []string {
    names := s.data.Collections()
    slices.SortFunc(names, compareNames)
    return names
}

func (s *Shelf) collectionsMenu() *fyne.Menu {
    all := fyne.NewMenuItem(userdata.AllBooks, func() {
        s.ShowCollection("")
    })
    all.Checked = s.collection == ""
    items := []*fyne.MenuItem{all}

    if names := s.collectionNames(); len(names) > 0 {
        items = append(items, fyne.NewMenuItemSeparator())
        for _, name := range names {
            item := fyne.NewMenuItem(name, func() {
                s.ShowCollection(name)
            })
            item.Checked = s.collection == name
            items = append(items, item)
        }
    }

    items = append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("New collection…", func() {
        s.askForName("New collection", "", "Create", func(name string) error {
            created, err := s.data.CreateCollection(name)
            if err == nil {
                s.ShowCollection(created)
            }
            return err
        })
    }))

    if current := s.collection; current != "" {
        items = append(items,
            fyne.NewMenuItem(fmt.Sprintf("Rename “%s”…", current), func() {
                s.askForName("Rename collection", current, "Rename", func(name string) error {
                    renamed, err := s.data.RenameCollection(current, name)
                    if err == nil {
                        s.ShowCollection(renamed)
                    }
                    return err
                })
            }),
            fyne.NewMenuItem(fmt.Sprintf("Delete “%s”…", current), func() {
                s.confirmDelete(current)
            }),
        )
    }

    return fyne.NewMenu("Collections", items...)
}

func (s *Shelf) ShowCollection(name string) {
    if name != "" && !slices.Contains(s.data.Collections(), name) {
        name = ""
    }

    s.collection = name
    label := name
    if label == "" {
        label = userdata.AllBooks
    }
    s.collectionButton.SetText(shortened(label, collectionButtonLength))
    s.header.Refresh()
    s.showShelf()
}

func shortened(text string, length int) string {
    if utf8.RuneCountInString(text) <= length {
        return text
    }

    return strings.TrimSpace(string([]rune(text)[:length-1])) + "…"
}

func (s *Shelf) inCollection(entry *shelfEntry) bool {
    return s.collection == "" || s.data.InCollection(s.collection, entry.path)
}

func (s *Shelf) cardMenu(entry *shelfEntry) *fyne.Menu {
    var choices []*fyne.MenuItem
    for _, name := range s.collectionNames() {
        member := s.data.InCollection(name, entry.path)
        choice := fyne.NewMenuItem(name, func() {
            s.data.SetInCollection(name, entry.path, !member)
            s.showShelf()
        })
        choice.Checked = member
        choices = append(choices, choice)
    }
    if len(choices) > 0 {
        choices = append(choices, fyne.NewMenuItemSeparator())
    }
    choices = append(choices, fyne.NewMenuItem("New collection…", func() {
        s.askForName("New collection", "", "Create", func(name string) error {
            created, err := s.data.CreateCollection(name)
            if err == nil {
                s.data.SetInCollection(created, entry.path, true)
                s.showShelf()
            }
            return err
        })
    }))

    collections := fyne.NewMenuItem("Add to collection", nil)
    collections.ChildMenu = fyne.NewMenu("", choices...)

    items := []*fyne.MenuItem{
        fyne.NewMenuItem("Play", func() {
            s.open(entry)
        }),
        fyne.NewMenuItem("About this book…", func() {
            s.showAbout(entry)
        }),
        fyne.NewMenuItem("Edit details…", func() {
            s.editDetails(entry)
        }),
        collections,
    }
    items = append(items, s.downloadItems(entry)...)
    if s.readingState(entry) != stateNotStarted {
        items = append(items, fyne.NewMenuItem("Reset progress…", func() {
            s.confirmRestart(entry)
        }))
    }
    if current := s.collection; current != "" {
        items = append(items, fyne.NewMenuItem(fmt.Sprintf("Remove from “%s”", current), func() {
            s.data.SetInCollection(current, entry.path, false)
            s.showShelf()
        }))
    }
    items = append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Remove from library…", func() {
        s.confirmRemove(entry, nil)
    }))

    return fyne.NewMenu("", items...)
}

func (s *Shelf) addToCollection(name string, paths []string) {
    if name == "" {
        return
    }

    for _, path := range paths {
        s.data.SetInCollection(name, library.Normalize(path), true)
    }
}

func capitalized(err error) error {
    if err == nil {
        return nil
    }

    text := []rune(err.Error())
    text[0] = unicode.ToUpper(text[0])
    return errors.New(string(text))
}

func (s *Shelf) askForName(title, current, confirm string, apply func(name string) error) {
    var popUp dialog.Dialog

    name := widget.NewEntry()
    name.SetText(current)
    name.PlaceHolder = "For example Classics or For bedtime"

    message := canvas.NewText(collectionTip, theme.Color(theme.ColorNamePlaceHolder))
    message.TextSize = theme.CaptionTextSize()

    create := widget.NewButtonWithIcon(confirm, theme.ConfirmIcon(), nil)
    create.Importance = widget.HighImportance
    cancel := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
        popUp.Hide()
    })

    check := func(text string) bool {
        err := s.data.CheckCollectionName(text, current)
        shown := err != nil && strings.TrimSpace(text) != ""

        if err == nil {
            create.Enable()
        } else {
            create.Disable()
        }

        if shown {
            message.Text = shortened(capitalized(err).Error(), collectionMessageLength)
            message.Color = theme.Color(theme.ColorNameError)
        } else {
            message.Text = collectionTip
            message.Color = theme.Color(theme.ColorNamePlaceHolder)
        }
        message.Refresh()

        return err == nil
    }

    submit := func() {
        if !check(name.Text) {
            return
        }
        if err := apply(name.Text); err != nil {
            dialog.ShowError(err, s.window)
            return
        }
        popUp.Hide()
    }

    name.OnChanged = func(text string) {
        check(text)
    }
    name.OnSubmitted = func(string) {
        submit()
    }
    create.OnTapped = submit
    check(current)

    popUp = dialog.NewCustomWithoutButtons(title, container.NewVBox(
        name,
        container.NewPadded(message),
        container.NewGridWithColumns(2, cancel, create),
    ), s.window)
    popUp.Resize(fyne.NewSize(min(s.window.Canvas().Size().Width-40, 440), popUp.MinSize().Height))
    popUp.Show()
    s.window.Canvas().Focus(name)
}

func (s *Shelf) confirmDelete(name string) {
    dialog.ShowConfirm(
        fmt.Sprintf("Delete “%s”?", name),
        "The books in it stay in your library.",
        func(confirmed bool) {
            if confirmed {
                s.data.DeleteCollection(name)
                s.ShowCollection("")
            }
        },
        s.window,
    )
}

func (s *Shelf) confirmRemove(entry *shelfEntry, removed func()) {
    message := "Your files aren't deleted. Use Add books to bring the book back where you left off."
    if s.Offline.State(entry.path) != offline.NotDownloaded {
        message = "Its offline copy on this computer is deleted, but the files on your drive or server aren't. Use Add books to bring the book back where you left off."
    }
    confirm := dialog.NewConfirm(
        fmt.Sprintf("Remove “%s” from your library?", shortened(entry.title, removeTitleLength)),
        message,
        func(confirmed bool) {
            if confirmed {
                s.remove(entry)
                if removed != nil {
                    removed()
                }
            }
        },
        s.window,
    )
    confirm.SetConfirmText("Remove")
    confirm.SetDismissText("Cancel")
    confirm.SetConfirmImportance(widget.DangerImportance)
    confirm.Show()
}

func (s *Shelf) confirmRestart(entry *shelfEntry) {
    confirm := dialog.NewConfirm(
        fmt.Sprintf("Start “%s” from the beginning?", shortened(entry.title, removeTitleLength)),
        "Your place in the book and its finished mark are cleared, on your other computers too if you sync. Your listening statistics are kept.",
        func(confirmed bool) {
            if confirmed {
                s.restart(entry)
            }
        },
        s.window,
    )
    confirm.SetConfirmText("Reset progress")
    confirm.SetDismissText("Cancel")
    confirm.SetConfirmImportance(widget.DangerImportance)
    confirm.Show()
}

func (s *Shelf) restart(entry *shelfEntry) {
    first := ""
    switch {
    case entry.book != nil:
        first = entry.book.Parts[0]
    case audio.IsSupported(entry.path):
        first = entry.path
    }

    if first == "" {
        delete(s.data.Positions, entry.path)
        s.data.Save()
    } else {
        s.data.Restart(entry.path, first)
    }

    if s.OnRestart != nil {
        s.OnRestart(entry.path)
    }
    s.Refresh()
}

func emptyCollectionText(name string) string {
    return fmt.Sprintf("“%s” is empty. Right-click a book in %s and choose Add to collection, or use Add books while this collection is open.", name, userdata.AllBooks)
}
