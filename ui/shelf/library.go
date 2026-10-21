package shelf

import (
    "cmp"
    "errors"
    "fmt"
    "image"
    "maps"
    "path/filepath"
    "slices"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/details"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const thumbnailSide = 320

type shelfEntry struct {
    path        string
    book        *library.Book
    title       string
    author      string
    original    details.Original
    missing     bool
    unavailable bool
    offline     bool
    network     bool
    outOfDate   bool
    fingerprint *library.Fingerprint
    card        *bookCard
}

type shelfInfo struct {
    book        *library.Book
    title       string
    author      string
    thumbnail   image.Image
    original    details.Original
    missing     bool
    unavailable bool
    offline     bool
    network     bool
    fingerprint *library.Fingerprint
    movedTo     string
}

func (s *Shelf) build() fyne.CanvasObject {
    addButton := widget.NewButtonWithIcon(
        "Add books",
        theme.ContentAddIcon(),
        s.chooseFolders,
    )
    addButton.Importance = widget.HighImportance

    clearSearch := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
        s.search.SetText("")
    })
    clearSearch.Importance = widget.LowImportance
    clearSearch.Hide()

    s.search = widget.NewEntry()
    s.search.PlaceHolder = "Search by title, author or file name"
    s.search.ActionItem = clearSearch
    s.search.OnChanged = func(query string) {
        if query == "" {
            clearSearch.Hide()
        } else {
            clearSearch.Show()
        }
        s.showShelf()
    }

    s.count = widget.NewLabel("")
    s.count.SizeName = theme.SizeNameCaptionText
    s.count.TextStyle = fyne.TextStyle{Bold: true}

    s.duplicatesButton = widget.NewButtonWithIcon("", theme.WarningIcon(), s.showDuplicates)
    s.duplicatesButton.Importance = widget.WarningImportance
    s.duplicatesButton.Hide()

    hint := widget.NewLabel("Click a book to play it · Right-click for more")
    hint.SizeName = theme.SizeNameCaptionText
    hint.Importance = widget.LowImportance
    hint.Alignment = fyne.TextAlignTrailing
    hint.Truncation = fyne.TextTruncateEllipsis

    template := newBookCard(nil, nil)
    s.grid = container.New(newShelfLayout(template.MinSize()))

    emptyTitle := widget.NewLabel("Your library is empty")
    emptyTitle.TextStyle = fyne.TextStyle{Bold: true}
    emptyTitle.SizeName = theme.SizeNameSubHeadingText
    emptyTitle.Alignment = fyne.TextAlignCenter
    emptyHint := widget.NewLabel("Add a folder of audiobooks, open a book, or choose a library folder in Settings to import books automatically.")
    emptyHint.Alignment = fyne.TextAlignCenter
    emptyHint.Wrapping = fyne.TextWrapWord
    emptyHint.Importance = widget.LowImportance
    emptyButton := widget.NewButtonWithIcon("Add books", theme.ContentAddIcon(), s.chooseFolders)
    emptyButton.Importance = widget.HighImportance
    s.empty = container.NewVBox(
        layout.NewSpacer(),
        container.NewCenter(canvas.NewImageFromResource(
            theme.NewColoredResource(theme.GridIcon(), theme.ColorNamePlaceHolder),
        )),
        emptyTitle,
        emptyHint,
        container.NewCenter(emptyButton),
        layout.NewSpacer(),
    )

    s.noMatches = widget.NewLabel("")
    s.noMatches.Alignment = fyne.TextAlignCenter
    s.noMatches.Wrapping = fyne.TextWrapWord
    s.noMatches.Importance = widget.LowImportance
    s.noMatches.Hide()

    filters, sorting := s.buildShelfControls()

    s.header = container.NewVBox(
        container.NewBorder(nil, nil, s.buildCollectionButton(), addButton, s.search),
        filters,
        container.NewBorder(nil, nil, container.NewHBox(s.count, s.duplicatesButton), sorting, hint),
    )

    return container.New(
        layout.NewCustomPaddedLayout(4, 0, 4, 4),
        container.NewBorder(
            s.header,
            nil,
            nil,
            nil,
            container.NewStack(
                container.NewVScroll(container.New(layout.NewCustomPaddedLayout(8, 16, 0, 0), s.grid)),
                s.empty,
                s.noMatches,
            ),
        ),
    )
}

func (s *Shelf) newShelfEntry(path string) *shelfEntry {
    entry := &shelfEntry{
        path:  path,
        title: library.CleanName(filepath.Base(path)),
    }
    if edit := s.data.Edit(path); edit.Title != "" {
        entry.title = edit.Title
    }
    entry.card = newBookCard(
        func() {
            s.open(entry)
        },
        func() *fyne.Menu {
            return s.cardMenu(entry)
        },
    )
    entry.card.Title = entry.title

    return entry
}

func (s *Shelf) find(path string) *shelfEntry {
    for _, entry := range s.entries {
        if entry.path == path {
            return entry
        }
    }

    return nil
}

func (s *Shelf) Add(paths []string) {
    s.data.Unignore(cleanPaths(paths))
    s.importBooks(paths, true, nil)
}

func cleanPaths(paths []string) []string {
    cleaned := make([]string, len(paths))
    for i, path := range paths {
        cleaned[i] = library.Normalize(path)
    }

    return cleaned
}

func (s *Shelf) Import(paths []string, done func(ImportResult)) {
    s.importBooks(paths, false, done)
}

func (s *Shelf) remove(entry *shelfEntry) {
    s.entries = slices.DeleteFunc(s.entries, func(other *shelfEntry) bool {
        return other == entry
    })
    s.data.Ignore(entry.path)
    s.Offline.Remove(entry.path)

    s.save()
    s.Refresh()
}

func (s *Shelf) Refresh() {
    if s.SyncProgress != nil {
        s.SyncProgress()
    }

    s.sortShelf()

    for _, entry := range s.entries {
        s.updateCard(entry)
    }

    s.showShelf()
}

func (s *Shelf) showShelf() {
    query := s.search.Text

    var objects []fyne.CanvasObject
    total := 0
    for _, entry := range s.entries {
        if !s.inCollection(entry) {
            continue
        }

        total++
        if s.passesFilter(entry) && entry.matches(query) {
            objects = append(objects, entry.card)
        }
    }

    s.grid.Objects = objects
    s.grid.Refresh()

    if len(s.entries) == 0 {
        s.empty.Show()
        s.header.Hide()
    } else {
        s.empty.Hide()
        s.header.Show()
    }

    s.count.SetText(bookCount(len(objects), total))
    s.updateDuplicates()

    switch {
    case len(s.entries) > 0 && total == 0 && s.collection != "":
        s.noMatches.SetText(emptyCollectionText(s.collection))
        s.noMatches.Show()
    case len(s.entries) > 0 && len(objects) == 0:
        s.noMatches.SetText(noMatchesText(s.filter, query))
        s.noMatches.Show()
    default:
        s.noMatches.Hide()
    }
}

func bookCount(shown, total int) string {
    noun := "books"
    if total == 1 {
        noun = "book"
    }

    if shown == total {
        return fmt.Sprintf("%d %s", total, noun)
    }

    return fmt.Sprintf("%d of %d %s", shown, total, noun)
}

func (s *Shelf) updateCard(entry *shelfEntry) {
    entry.card.Title = entry.title
    entry.card.Author = entry.author
    if entry.author == "" && entry.book != nil && entry.book.IsMultiPart() {
        entry.card.Author = fmt.Sprintf("%d parts", len(entry.book.Parts))
    }
    entry.card.Progress, entry.card.Status = s.shelfProgress(entry)
    entry.card.Status = s.withDownload(entry, entry.card.Status)
    entry.card.Rating = ""
    if online, found := s.data.Online(entry.path); found {
        entry.card.Rating = online.Edition.Stars()
    }
    entry.card.Refresh()
}

func (s *Shelf) shelfProgress(entry *shelfEntry) (float64, string) {
    if entry.missing && entry.unavailable {
        return 0, "Not connected"
    }
    if entry.missing {
        return 0, "Not found"
    }

    saved, started := s.data.Positions[entry.path]
    if saved.Finished {
        return 1, "Finished"
    }

    book := entry.book
    if book == nil || !book.HasDurations() {
        if s.readingState(entry) == stateInProgress {
            return 0, "In progress"
        }

        return 0, "Not started"
    }

    total := book.TotalDuration()
    elapsed := 0.0
    if started {
        index, found := book.Find(saved.Part)
        if found {
            elapsed = book.OffsetOf(index) + saved.Position
        }
    }

    if elapsed <= 0 || total <= 0 {
        return 0, "Not started · " + kit.FormatLength(total)
    }

    fraction := min(elapsed/total, 1)
    remaining := max(total-elapsed, 0) / s.speed()

    return fraction, fmt.Sprintf("%d%% · %s left", int(fraction*100), kit.FormatLength(remaining))
}

func (s *Shelf) loadShelfInfo(entries []*shelfEntry) {
    if len(entries) == 0 {
        return
    }

    entries = slices.Clone(entries)
    paths := make([]string, len(entries))
    fingerprints := make([]*library.Fingerprint, len(entries))
    edits := make([]userdata.BookEdit, len(entries))
    coverFiles := make([]string, len(entries))
    locals := make([]*library.Book, len(entries))
    for i, entry := range entries {
        paths[i], fingerprints[i] = entry.path, entry.fingerprint
        edits[i], coverFiles[i] = s.data.Edit(entry.path), s.data.CoverFile(entry.path)
        locals[i] = s.Offline.Local(entry.path)
    }
    cache := maps.Clone(s.data.Durations)

    s.async(func() {
        books := make([]*library.Book, len(entries))
        for i, entry := range entries {
            info := readShelfInfo(paths[i], fingerprints[i], s.cache, locals[i])
            info.applyEdit(edits[i], coverFiles[i], s.cache)
            books[i] = info.book

            fyne.Do(func() {
                s.applyShelfInfo(entry, info)
            })
        }
        s.cache.save()

        for i, book := range books {
            if book == nil {
                continue
            }

            durations, measured, ok := measureBook(book, cache)
            if !ok {
                continue
            }

            entry := entries[i]
            fyne.Do(func() {
                s.applyShelfDurations(entry, book, durations, measured)
            })
        }
    })
}

func readShelfInfo(path string, known *library.Fingerprint, cache *shelfCache, local *library.Book) shelfInfo {
    book, err := library.OpenBook(path)
    offline := false
    switch {
    case err == nil:
    case local != nil:
        book, offline = local, true
    default:
        if moved := locateMoved(path, known); moved != "" {
            info := readShelfInfo(moved, nil, cache, nil)
            info.movedTo = moved
            return info
        }
        return shelfInfo{missing: true, unavailable: library.Unavailable(path)}
    }

    info := shelfInfo{book: book, offline: offline, network: offline || library.IsNetwork(path)}
    if original, found := cache.bookInfo(path, book); found {
        info.original = original
    } else {
        original, coverFile := readOriginal(book)
        info.original = original
        cache.rememberBook(path, book, original, coverFile)
    }

    if !offline {
        if fingerprint, err := library.FingerprintOfBook(book); err == nil {
            info.fingerprint = &fingerprint
        }
    }

    return info
}

func readOriginal(book *library.Book) (details.Original, string) {
    first := book.PlayablePart(0)
    metadata := library.ReadMetadata(first)
    original := details.Original{
        Title:  book.Title(metadata),
        Author: metadata.Artist,
    }

    cover, _ := library.FindCover(first, metadata.Picture)
    if cover == nil {
        return original, ""
    }

    original.Cover, _ = library.Thumbnail(cover.Data, thumbnailSide)
    return original, cover.File()
}

func (info *shelfInfo) applyEdit(edit userdata.BookEdit, coverFile string, cache *shelfCache) {
    info.title = cmp.Or(edit.Title, info.original.Title)
    info.author = cmp.Or(edit.Author, info.original.Author)
    info.thumbnail = info.original.Cover

    if custom := cache.customThumbnail(coverFile); custom != nil {
        info.thumbnail = custom
    }
}

func measureBook(book *library.Book, cache map[string]userdata.PartDuration) ([]float64, map[string]userdata.PartDuration, bool) {
    durations := make([]float64, len(book.Parts))
    measured := map[string]userdata.PartDuration{}

    for i, part := range book.Parts {
        if seconds, found := userdata.LookupDurationOf(cache, part, book.PlayablePart(i)); found {
            durations[i] = seconds
            continue
        }

        entry, err := userdata.MeasureDuration(book.PlayablePart(i))
        if err != nil {
            return nil, nil, false
        }

        durations[i] = entry.Seconds
        measured[part] = entry
    }

    return durations, measured, true
}

func (s *Shelf) applyShelfInfo(entry *shelfEntry, info shelfInfo) {
    if info.movedTo != "" && s.find(info.movedTo) == nil {
        s.relink(entry, info.movedTo)
        s.save()
    }

    if info.fingerprint != nil && !info.missing {
        entry.fingerprint = info.fingerprint
        s.markDirty()
    }

    entry.book = info.book
    entry.missing = info.missing
    entry.unavailable = info.unavailable
    entry.offline = info.offline
    entry.network = info.network
    entry.outOfDate = !info.offline && s.Offline.OutOfDate(entry.path, info.fingerprint)
    if info.title != "" {
        entry.title = info.title
    }
    entry.author = info.author
    entry.original = info.original
    entry.card.Thumbnail = info.thumbnail

    s.updateCard(entry)
    s.arrangeShelfSoon()
}

func (s *Shelf) applyShelfDurations(
    entry *shelfEntry,
    book *library.Book,
    durations []float64,
    measured map[string]userdata.PartDuration,
) {
    if len(measured) > 0 {
        maps.Copy(s.data.Durations, measured)
        s.data.MarkDirty()
    }

    if entry.book != book {
        return
    }

    book.Durations = durations
    s.updateCard(entry)
    s.arrangeShelfSoon()
}

func (s *Shelf) AddFrom(roots []string) {
    collection := s.collection

    s.async(func() {
        var found []string
        var failures []error
        for _, root := range roots {
            books, err := library.FindBooks(root)
            if err != nil {
                failures = append(failures, err)
                continue
            }

            found = append(found, books...)
        }

        fyne.Do(func() {
            s.show()

            if len(found) == 0 {
                err := errors.Join(failures...)
                if err == nil {
                    err = fmt.Errorf("no audiobooks were found in the selected folders")
                }
                dialog.ShowError(err, s.window)
                return
            }

            s.data.Unignore(cleanPaths(found))
            s.importBooks(found, false, func(result ImportResult) {
                var inLibrary []string
                for _, path := range found {
                    if book := result.InLibrary(path); book != "" {
                        inLibrary = append(inLibrary, book)
                    }
                }
                s.addToCollection(collection, inLibrary)
                s.showShelf()

                if len(result.Copies) > 0 {
                    dialog.ShowInformation("Already in your library", copiesMessage(result.Copies), s.window)
                }
            })
        })
    })
}
