package syncing

import (
	"maps"
	"path/filepath"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/stats"
	"github.com/izzzicos/audiobook-player/ui/userdata"
	"github.com/izzzicos/audiobook-player/vfs"
)

const coversFolder = "covers"

type EditRecord struct {
	Fingerprint library.Fingerprint     `json:"fingerprint"`
	Changed     int64                   `json:"changed"`
	Edit        userdata.BookEdit       `json:"edit"`
	Online      *userdata.OnlineDetails `json:"online,omitempty"`

	coverData []byte
}

type CollectionRecord struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	Changed int64                 `json:"changed"`
	Books   []library.Fingerprint `json:"books,omitempty"`
}

type BookStat struct {
	Fingerprint *library.Fingerprint `json:"fingerprint,omitempty"`
	Title       string               `json:"title"`
	Seconds     float64              `json:"seconds,omitempty"`
	Finished    string               `json:"finished,omitempty"`
}

type StatsRecord struct {
	Days    map[string]stats.Day `json:"days,omitempty"`
	Books   []BookStat           `json:"books,omitempty"`
	Earlier *stats.Summary       `json:"earlier,omitempty"`
}

type shared struct {
	edits       []EditRecord
	collections []CollectionRecord
	deleted     map[string]int64
	stats       *StatsRecord
	covers      []string
}

func (s *Syncer) fingerprints() map[string]*library.Fingerprint {
	prints := map[string]*library.Fingerprint{}
	for _, book := range s.data.Library {
		if book.Fingerprint != nil {
			prints[book.Path] = book.Fingerprint
		}
	}

	return prints
}

func (s *Syncer) title(path string) string {
	if s.Title != nil {
		if title, found := s.Title(path); found {
			return title
		}
	}

	return library.CleanName(filepath.Base(path))
}

func (s *Syncer) sharedSnapshot() shared {
	prints := s.fingerprints()
	var snapshot shared

	for _, book := range s.data.EditedBooks() {
		print := prints[book]
		if print == nil {
			continue
		}

		edit := s.data.Edit(book)
		record := EditRecord{Fingerprint: *print, Changed: s.data.EditChanged(book), Edit: edit}
		if edit.ASIN != "" {
			if details, found := s.data.OnlineDetailsFor(edit.Store, edit.ASIN); found {
				record.Online = &details
			}
		}
		if edit.Cover != "" {
			snapshot.covers = append(snapshot.covers, edit.Cover)
		}
		snapshot.edits = append(snapshot.edits, record)
	}

	collections, deleted := s.data.SyncedCollections()
	snapshot.deleted = deleted
	for _, collection := range collections {
		record := CollectionRecord{ID: collection.ID, Name: collection.Name, Changed: collection.Changed}
		for _, book := range collection.Books {
			if print := prints[book]; print != nil {
				record.Books = append(record.Books, *print)
			}
		}
		record.Books = append(record.Books, collection.Elsewhere...)
		snapshot.collections = append(snapshot.collections, record)
	}

	log := s.data.Stats
	record := &StatsRecord{Days: maps.Clone(log.Days)}
	if log.Earlier != nil {
		earlier := *log.Earlier
		record.Earlier = &earlier
	}
	books := map[string]bool{}
	for book := range log.Books {
		books[book] = true
	}
	for book := range log.Completed {
		books[book] = true
	}
	for book := range books {
		record.Books = append(record.Books, BookStat{
			Fingerprint: prints[book],
			Title:       s.title(book),
			Seconds:     log.Books[book],
			Finished:    log.Completed[book],
		})
	}
	snapshot.stats = record

	return snapshot
}

func (s *Syncer) uploadCovers(folder string, names []string) {
	for _, name := range names {
		if s.uploaded[name] {
			continue
		}

		target := filepath.Join(s.syncDirectory(folder), coversFolder, name)
		if _, err := vfs.Stat(target); err == nil {
			s.uploaded[name] = true
			continue
		}

		data := s.data.CoverData(name)
		if data == nil {
			continue
		}
		if vfs.WriteAtomic(target, data) == nil {
			s.uploaded[name] = true
		}
	}
}

func (s *Syncer) downloadCovers(folder string, devices []Device) {
	for d := range devices {
		for e := range devices[d].Edits {
			record := &devices[d].Edits[e]
			name := record.Edit.Cover
			if name == "" || s.data.HasCover(name) {
				continue
			}
			if data, err := vfs.ReadFile(filepath.Join(s.syncDirectory(folder), coversFolder, name)); err == nil {
				record.coverData = data
			}
		}
	}
}

type applied struct {
	books       []string
	collections bool
	stats       bool
}

func (s *Syncer) applyShared(devices []Device) applied {
	var result applied
	books := s.data.Library

	newest := map[string]EditRecord{}
	for _, device := range devices {
		for _, record := range device.Edits {
			book, found := matchBook(books, record.Fingerprint)
			if found && record.Changed > newest[book.Path].Changed {
				newest[book.Path] = record
			}
		}
	}
	for path, record := range newest {
		if record.Changed <= s.data.EditChanged(path) {
			continue
		}

		edit := record.Edit
		if edit.Cover != "" && !s.data.HasCover(edit.Cover) {
			if record.coverData == nil {
				continue
			}
			name, err := s.data.SaveCover(record.coverData)
			if err != nil {
				continue
			}
			edit.Cover = name
		}
		if record.Online != nil {
			s.data.ApplyOnline(*record.Online)
		}
		if s.data.ApplyEdit(path, edit, record.Changed) {
			result.books = append(result.books, path)
		}
	}

	latest := map[string]CollectionRecord{}
	for _, device := range devices {
		for id, when := range device.Deleted {
			if s.data.ApplyDeletion(id, when) {
				result.collections = true
			}
		}
		for _, record := range device.Collections {
			if record.Changed > latest[record.ID].Changed {
				latest[record.ID] = record
			}
		}
	}
	for _, record := range latest {
		collection := userdata.Collection{ID: record.ID, Name: record.Name, Changed: record.Changed}
		for _, print := range record.Books {
			if book, found := matchBook(books, print); found {
				collection.Books = append(collection.Books, book.Path)
			} else {
				collection.Elsewhere = append(collection.Elsewhere, print)
			}
		}
		if s.data.ApplyCollection(collection) {
			result.collections = true
		}
	}

	for _, device := range devices {
		if device.Stats == nil {
			continue
		}

		log := stats.New()
		maps.Copy(log.Days, device.Stats.Days)
		log.Earlier = device.Stats.Earlier
		titles := map[string]string{}
		for _, book := range device.Stats.Books {
			key := "elsewhere:" + book.Title
			if book.Fingerprint != nil {
				if local, found := matchBook(books, *book.Fingerprint); found {
					key = local.Path
				} else {
					key = "elsewhere:" + book.Fingerprint.Contents()
				}
			}
			if book.Seconds > 0 {
				log.Books[key] += book.Seconds
			}
			if book.Finished != "" {
				log.Completed[key] = book.Finished
			}
			titles[key] = book.Title
		}
		s.data.SetOtherStats(device.ID, log, titles)
		result.stats = true
	}

	if len(result.books) > 0 || result.collections {
		s.data.MarkDirty()
	}

	return result
}
