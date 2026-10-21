package userdata

import (
    "path/filepath"
    "strings"

    "github.com/izzzicos/audiobook-player/library"
)

func MovedPath(path, from, to string) string {
    if path == from {
        return to
    }

    if strings.HasPrefix(path, from+string(filepath.Separator)) {
        return to + path[len(from):]
    }

    return path
}

func (d *Data) Move(old, newPath string) {
    if old == newPath {
        return
    }

    if position, found := d.Positions[old]; found {
        position.Part = MovedPath(position.Part, old, newPath)
        if existing, taken := d.Positions[newPath]; !taken || existing.Listened < position.Listened {
            d.Positions[newPath] = position
        }
        delete(d.Positions, old)
    }

    d.Stats.Rekey(old, newPath)

    moved := map[string]PartDuration{}
    for part, duration := range d.Durations {
        if target := MovedPath(part, old, newPath); target != part {
            moved[target] = duration
            delete(d.Durations, part)
        }
    }
    for part, duration := range moved {
        d.Durations[part] = duration
    }

    if d.Ignored[old] {
        delete(d.Ignored, old)
        d.Ignored[newPath] = true
    }

    d.moveInCollections(old, newPath)
    d.moveEdit(old, newPath)
    d.moveDownload(old, newPath)

    d.LastBook.Folder = MovedPath(d.LastBook.Folder, old, newPath)
    d.LastBook.File = MovedPath(d.LastBook.File, old, newPath)
}

func (d *Data) Merge(part, book string) {
    if part == book {
        return
    }

    if position, found := d.Positions[part]; found {
        position.Finished = false
        if existing, taken := d.Positions[book]; !taken || existing.Listened < position.Listened {
            d.Positions[book] = position
        }
        delete(d.Positions, part)
    }

    d.Stats.Rekey(part, book)
    d.moveInCollections(part, book)
    d.moveEdit(part, book)

    if d.LastBook.Folder == "" && d.LastBook.File == part {
        d.LastBook.Folder = book
    }
}

func (d *Data) MoveIgnored(from, to string) {
    for path := range d.Ignored {
        if relative, inside := library.Within(path, from); inside {
            delete(d.Ignored, path)
            d.Ignored[filepath.Join(to, relative)] = true
        }
    }
}
