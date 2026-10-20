package userdata

import (
    "errors"
    "maps"
    "os"
    "path/filepath"
)

type TidyPlan struct {
    library map[string]bool
    parts   map[string]PartDuration
    removed []string
}

type Leftovers struct {
    parts map[string]PartDuration
    books []string
}

func (d *Data) libraryPaths() map[string]bool {
    paths := map[string]bool{}
    for _, book := range d.Library {
        paths[book.Path] = true
    }

    return paths
}

func (d *Data) PlanTidy() TidyPlan {
    plan := TidyPlan{
        library: d.libraryPaths(),
        parts:   maps.Clone(d.Durations),
    }

    seen := map[string]bool{}
    for book := range d.Positions {
        seen[book] = true
    }
    for book := range d.Ignored {
        seen[book] = true
    }
    for _, collection := range d.collections {
        for _, book := range collection.Books {
            seen[book] = true
        }
    }
    for book := range seen {
        if !plan.library[book] {
            plan.removed = append(plan.removed, book)
        }
    }

    return plan
}

func (p TidyPlan) Find() Leftovers {
    leftovers := Leftovers{parts: map[string]PartDuration{}}
    present := map[string]bool{}

    for part, cached := range p.parts {
        book, found := owner(part, p.library)
        if !found {
            leftovers.parts[part] = cached
            continue
        }

        if _, checked := present[book]; !checked {
            _, err := os.Stat(book)
            present[book] = err == nil
        }
        if !present[book] {
            continue
        }

        info, err := os.Stat(part)
        if err != nil || info.Size() != cached.Size || info.ModTime().UnixNano() != cached.Modified {
            leftovers.parts[part] = cached
        }
    }

    for _, book := range p.removed {
        if gone(book) {
            leftovers.books = append(leftovers.books, book)
        }
    }

    return leftovers
}

func owner(part string, library map[string]bool) (string, bool) {
    for path := part; ; {
        if library[path] {
            return path, true
        }

        parent := filepath.Dir(path)
        if parent == path {
            return "", false
        }
        path = parent
    }
}

func gone(path string) bool {
    if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
        return false
    }

    _, err := os.Stat(filepath.Dir(path))
    return err == nil
}

func (d *Data) Forget(leftovers Leftovers) {
    library := d.libraryPaths()
    changed := false

    for part, planned := range leftovers.parts {
        if current, found := d.Durations[part]; found && current == planned {
            delete(d.Durations, part)
            changed = true
        }
    }

    for _, book := range leftovers.books {
        if library[book] {
            continue
        }

        if _, found := d.Positions[book]; found {
            delete(d.Positions, book)
            changed = true
        }
        if d.Ignored[book] {
            delete(d.Ignored, book)
            changed = true
        }
        if d.forgetInCollections(book) {
            changed = true
        }
    }

    if changed {
        d.MarkDirty()
    }
}
