package syncing

import (
	"path/filepath"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

const deviceVersion = 1

type Device struct {
	Version     int                `json:"version"`
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Written     int64              `json:"written"`
	Books       []Record           `json:"books"`
	Edits       []EditRecord       `json:"edits,omitempty"`
	Collections []CollectionRecord `json:"collections,omitempty"`
	Deleted     map[string]int64   `json:"deletedCollections,omitempty"`
	Stats       *StatsRecord       `json:"stats,omitempty"`

	covers []string
}

type Record struct {
	Title       string              `json:"title,omitempty"`
	Fingerprint library.Fingerprint `json:"fingerprint"`
	Part        string              `json:"part"`
	PartIndex   int                 `json:"partIndex"`
	Position    float64             `json:"position"`
	Elapsed     float64             `json:"elapsed,omitempty"`
	Listened    int64               `json:"listened"`
	Played      int64               `json:"played,omitempty"`
	Finished    bool                `json:"finished,omitempty"`
	Last        bool                `json:"last,omitempty"`
}

func (r Record) PlayedAt() int64 {
	if r.Played > 0 {
		return r.Played
	}

	return r.Listened
}

type Change struct {
	Book     userdata.SavedBook
	Position userdata.Position
	Record   Record
	Device   string
}

func (c Change) LastBook() userdata.LastBook {
	paths := partPaths(c.Book)
	if len(paths) == 1 && paths[0] == c.Book.Path {
		return userdata.LastBook{File: c.Position.Part}
	}

	return userdata.LastBook{Folder: c.Book.Path, File: c.Position.Part}
}

func partPaths(book userdata.SavedBook) []string {
	parts := book.Fingerprint.Parts
	if len(parts) == 1 && parts[0].Name == filepath.Base(book.Path) {
		return []string{book.Path}
	}

	paths := make([]string, len(parts))
	for i, part := range parts {
		paths[i] = filepath.Join(book.Path, part.Name)
	}

	return paths
}

func partLengths(paths []string, durations map[string]userdata.PartDuration) ([]float64, bool) {
	lengths := make([]float64, len(paths))
	for i, path := range paths {
		cached, found := durations[path]
		if !found {
			return nil, false
		}
		lengths[i] = cached.Seconds
	}

	return lengths, true
}

func recordFor(book userdata.SavedBook, title string, position userdata.Position, durations map[string]userdata.PartDuration) (Record, bool) {
	if book.Fingerprint == nil || len(book.Fingerprint.Parts) == 0 {
		return Record{}, false
	}

	paths := partPaths(book)
	index := -1
	for i, path := range paths {
		if path == position.Part {
			index = i
		}
	}
	if index < 0 {
		return Record{}, false
	}

	record := Record{
		Title:       title,
		Fingerprint: *book.Fingerprint,
		Part:        filepath.Base(position.Part),
		PartIndex:   index,
		Position:    position.Position,
		Listened:    position.Listened,
		Played:      position.PlayedAt(),
		Finished:    position.Finished,
	}
	if lengths, ok := partLengths(paths, durations); ok {
		for _, length := range lengths[:index] {
			record.Elapsed += length
		}
		record.Elapsed += position.Position
	}

	return record, true
}

func matchBook(books []userdata.SavedBook, fingerprint library.Fingerprint) (userdata.SavedBook, bool) {
	for _, book := range books {
		if book.Fingerprint != nil && book.Fingerprint.SameContents(fingerprint) {
			return book, true
		}
	}

	for _, book := range books {
		if book.Fingerprint != nil && (book.Fingerprint.Matches(fingerprint) || fingerprint.Matches(*book.Fingerprint)) {
			return book, true
		}
	}

	return userdata.SavedBook{}, false
}

func place(book userdata.SavedBook, record Record, durations map[string]userdata.PartDuration) (userdata.Position, bool) {
	paths := partPaths(book)
	position := userdata.Position{Listened: record.Listened, Played: record.PlayedAt(), Finished: record.Finished}

	for _, path := range paths {
		if filepath.Base(path) == record.Part {
			position.Part, position.Position = path, record.Position
			return position, true
		}
	}

	lengths, ok := partLengths(paths, durations)
	if !ok || record.Elapsed <= 0 {
		return userdata.Position{}, false
	}

	remaining := record.Elapsed
	for i, length := range lengths {
		if remaining < length || i == len(lengths)-1 {
			position.Part, position.Position = paths[i], min(max(remaining, 0), length)
			return position, true
		}
		remaining -= length
	}

	return userdata.Position{}, false
}

func newerChanges(books []userdata.SavedBook, local map[string]userdata.Position, durations map[string]userdata.PartDuration, devices []Device) ([]Change, *Change) {
	best := map[string]Change{}
	var last *Change

	for _, device := range devices {
		for _, record := range device.Books {
			book, found := matchBook(books, record.Fingerprint)
			if !found {
				continue
			}

			position, ok := place(book, record, durations)
			if !ok {
				continue
			}

			change := Change{Book: book, Position: position, Record: record, Device: device.Name}
			if record.PlayedAt() > local[book.Path].PlayedAt() && record.PlayedAt() > best[book.Path].Record.PlayedAt() {
				best[book.Path] = change
			}
			if record.Last && (last == nil || record.PlayedAt() > last.Record.PlayedAt()) {
				last = &change
			}
		}
	}

	changes := make([]Change, 0, len(best))
	for _, book := range books {
		if change, found := best[book.Path]; found {
			changes = append(changes, change)
		}
	}

	return changes, last
}
