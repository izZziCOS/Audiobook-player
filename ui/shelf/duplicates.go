package shelf

import (
    "cmp"
    "fmt"
    "os"
    "path/filepath"
    "slices"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/vfs"
)

type Copy struct {
    Path     string
    Original string
}

type ImportResult struct {
    Added  int
    Copies []Copy

    paths map[string]string
}

func (r ImportResult) InLibrary(path string) string {
    return r.paths[library.Normalize(path)]
}

type knownBook struct {
    path        string
    info        os.FileInfo
    fingerprint *library.Fingerprint
    merged      bool
}

type importPlan struct {
    added   []string
    relinks map[string]string
    merges  map[string]string
    copies  []Copy
    paths   map[string]string
}

func (s *Shelf) knownBooks() []knownBook {
    books := make([]knownBook, len(s.entries))
    for i, entry := range s.entries {
        books[i] = knownBook{path: entry.path, fingerprint: entry.fingerprint}
    }

    return books
}

func (book *knownBook) inspect() {
    info, err := vfs.Stat(book.path)
    if err != nil {
        return
    }

    book.info = info
    if book.fingerprint == nil {
        if fingerprint, err := library.FingerprintOf(book.path); err == nil {
            book.fingerprint = &fingerprint
        }
    }
}

func (book *knownBook) present() bool {
    return book.info != nil && !book.merged
}

func planImport(books []knownBook, paths []string, keepCopies bool) importPlan {
    plan := importPlan{
        relinks: map[string]string{},
        merges:  map[string]string{},
        paths:   map[string]string{},
    }

    for i := range books {
        books[i].inspect()
    }

    exact := map[string]bool{}
    for _, book := range books {
        exact[book.path] = true
    }

    for _, path := range paths {
        path = library.Normalize(path)
        if plan.paths[path] != "" {
            continue
        }
        if exact[path] {
            plan.paths[path] = path
            continue
        }

        info, err := vfs.Stat(path)
        if err != nil {
            plan.added = append(plan.added, path)
            plan.paths[path] = path
            exact[path] = true
            continue
        }

        if owner := coveringBook(books, path, info); owner != "" {
            plan.paths[path] = owner
            continue
        }

        var fingerprint *library.Fingerprint
        if found, err := library.FingerprintOf(path); err == nil {
            fingerprint = &found
        }

        if moved := movedBook(books, fingerprint); moved != nil {
            plan.relinks[moved.path] = path
            moved.path, moved.info = path, info
            plan.paths[path] = path
            exact[path] = true
            continue
        }

        if original := copiedBook(books, fingerprint); original != nil && !keepCopies {
            plan.copies = append(plan.copies, Copy{Path: path, Original: original.path})
            plan.paths[path] = original.path
            continue
        }

        if info.IsDir() && fingerprint != nil && !wholeBooksOnly(*fingerprint) {
            for i := range books {
                book := &books[i]
                if book.present() && !book.info.IsDir() && library.Normalize(filepath.Dir(book.path)) == path {
                    book.merged = true
                    plan.merges[book.path] = path
                }
            }
        }

        plan.added = append(plan.added, path)
        plan.paths[path] = path
        exact[path] = true
        books = append(books, knownBook{path: path, info: info, fingerprint: fingerprint})
    }

    for part, book := range plan.merges {
        for requested, inLibrary := range plan.paths {
            if inLibrary == part {
                plan.paths[requested] = book
            }
        }
    }

    return plan
}

func coveringBook(books []knownBook, path string, info os.FileInfo) string {
    for _, book := range books {
        if book.present() && os.SameFile(book.info, info) {
            return book.path
        }
    }

    if info.IsDir() {
        return ""
    }

    folder := library.Normalize(filepath.Dir(path))
    folderInfo, err := vfs.Stat(folder)
    if err != nil {
        return ""
    }

    for _, book := range books {
        if book.present() && book.info.IsDir() && (book.path == folder || os.SameFile(book.info, folderInfo)) {
            return book.path
        }
    }

    return ""
}

func movedBook(books []knownBook, fingerprint *library.Fingerprint) *knownBook {
    if fingerprint == nil {
        return nil
    }

    for i := range books {
        book := &books[i]
        if book.info == nil && !book.merged && book.fingerprint != nil && book.fingerprint.Matches(*fingerprint) {
            return book
        }
    }

    return nil
}

func copiedBook(books []knownBook, fingerprint *library.Fingerprint) *knownBook {
    if fingerprint == nil {
        return nil
    }

    for i := range books {
        book := &books[i]
        if book.present() && book.fingerprint != nil && book.fingerprint.SameContents(*fingerprint) {
            return book
        }
    }

    return nil
}

func wholeBooksOnly(fingerprint library.Fingerprint) bool {
    names := make([]string, len(fingerprint.Parts))
    for i, part := range fingerprint.Parts {
        names[i] = part.Name
    }

    return library.WholeBooksOnly(names)
}

func (s *Shelf) importBooks(paths []string, keepCopies bool, done func(ImportResult)) {
    if len(paths) == 0 {
        if done != nil {
            done(ImportResult{})
        }
        return
    }

    books := s.knownBooks()
    paths = slices.Clone(paths)

    s.async(func() {
        plan := planImport(books, paths, keepCopies)

        fyne.Do(func() {
            result := s.applyImport(plan)
            if done != nil {
                done(result)
            }
        })
    })
}

func (s *Shelf) applyImport(plan importPlan) ImportResult {
    result := ImportResult{Copies: plan.copies, paths: plan.paths}

    var changed []*shelfEntry
    for old, newPath := range plan.relinks {
        if entry := s.find(old); entry != nil && s.find(newPath) == nil {
            s.relink(entry, newPath)
            changed = append(changed, entry)
        }
    }

    for _, path := range plan.added {
        if s.find(path) != nil {
            continue
        }

        entry := s.newShelfEntry(path)
        s.entries = append(s.entries, entry)
        changed = append(changed, entry)
        result.Added++
    }

    merged := false
    for part, book := range plan.merges {
        entry := s.find(part)
        if entry == nil || s.find(book) == nil {
            continue
        }

        s.data.Merge(part, book)
        s.entries = slices.DeleteFunc(s.entries, func(other *shelfEntry) bool {
            return other == entry
        })
        merged = true
    }

    if len(changed) == 0 && !merged {
        return result
    }

    s.save()
    s.Refresh()
    s.loadShelfInfo(changed)

    return result
}

func (s *Shelf) FolderBookOf(file string) string {
    folder := library.Normalize(filepath.Dir(file))
    entry := s.find(folder)
    if entry == nil || entry.missing {
        return ""
    }

    if info, err := vfs.Stat(folder); err != nil || !info.IsDir() {
        return ""
    }

    return folder
}

func (s *Shelf) duplicateGroups() [][]*shelfEntry {
    folders := map[string]*shelfEntry{}
    for _, entry := range s.entries {
        if !entry.missing && entry.book != nil && entry.book.Folder != "" {
            folders[entry.path] = entry
        }
    }

    contents := func(entry *shelfEntry) string {
        if entry.missing || entry.fingerprint == nil || len(entry.fingerprint.Parts) == 0 {
            return ""
        }
        if entry.book != nil && entry.book.Folder == "" {
            if folder := folders[library.Normalize(filepath.Dir(entry.path))]; folder != nil && folder.fingerprint != nil {
                return folder.fingerprint.Contents()
            }
        }

        return entry.fingerprint.Contents()
    }

    grouped := map[string][]*shelfEntry{}
    var keys []string
    for _, entry := range s.entries {
        key := contents(entry)
        if key == "" {
            continue
        }
        if _, seen := grouped[key]; !seen {
            keys = append(keys, key)
        }
        grouped[key] = append(grouped[key], entry)
    }

    var groups [][]*shelfEntry
    for _, key := range keys {
        if group := grouped[key]; len(group) > 1 {
            slices.SortFunc(group, func(a, b *shelfEntry) int {
                return cmp.Compare(a.path, b.path)
            })
            groups = append(groups, group)
        }
    }
    slices.SortFunc(groups, func(a, b []*shelfEntry) int {
        return compareNames(a[0].title, b[0].title)
    })

    return groups
}

func (s *Shelf) updateDuplicates() {
    count := len(s.duplicateGroups())
    if count == 0 {
        s.duplicatesButton.Hide()
        return
    }

    s.duplicatesButton.SetText(kit.CountPhrase(count, "duplicate", "duplicates", ""))
    s.duplicatesButton.Show()
}

func (s *Shelf) showDuplicates() {
    var popUp dialog.Dialog

    intro := widget.NewLabel("These books are in your library more than once. Remove the ones you don't need; their files stay where they are.")
    intro.Wrapping = fyne.TextWrapWord

    list := container.NewVBox()
    var fill func()
    fill = func() {
        groups := s.duplicateGroups()
        if len(groups) == 0 {
            popUp.Hide()
            return
        }

        list.RemoveAll()
        for i, group := range groups {
            if i > 0 {
                list.Add(widget.NewSeparator())
            }

            heading := widget.NewLabel(group[0].title)
            heading.TextStyle = fyne.TextStyle{Bold: true}
            heading.Truncation = fyne.TextTruncateEllipsis
            list.Add(heading)

            for _, entry := range group {
                list.Add(s.duplicateRow(entry, fill))
            }
        }
        list.Refresh()
    }

    size := s.window.Canvas().Size()
    popUp = dialog.NewCustom(
        "Duplicate books",
        "Close",
        container.NewBorder(intro, nil, nil, nil, container.NewVScroll(list)),
        s.window,
    )
    fill()
    popUp.Resize(fyne.NewSize(min(size.Width-40, 560), size.Height*0.8))
    popUp.Show()
}

func (s *Shelf) duplicateRow(entry *shelfEntry, removed func()) fyne.CanvasObject {
    location := widget.NewLabel(vfs.Display(entry.path))
    location.Wrapping = fyne.TextWrapBreak
    location.SizeName = theme.SizeNameCaptionText

    status := widget.NewLabel(entry.card.Status)
    status.SizeName = theme.SizeNameCaptionText
    status.Importance = widget.LowImportance

    remove := widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), func() {
        s.confirmRemove(entry, removed)
    })
    remove.Importance = widget.LowImportance

    return container.NewBorder(nil, nil, nil, container.NewCenter(remove), container.NewVBox(location, status))
}

func copiesMessage(copies []Copy) string {
    if len(copies) == 1 {
        return fmt.Sprintf(
            "%q is already in your library from %s, so this copy wasn't added.",
            filepath.Base(copies[0].Path),
            copies[0].Original,
        )
    }

    return fmt.Sprintf(
        "%d books are already in your library from other folders, so these copies weren't added.",
        len(copies),
    )
}
