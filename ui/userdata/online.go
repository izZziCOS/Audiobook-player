package userdata

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"

	"github.com/izzzicos/audiobook-player/online"
	"github.com/izzzicos/audiobook-player/store"
)

const (
	onlineFileName = "online.json"
	onlineVersion  = 1
	RefreshOnline  = 30 * 24 * time.Hour
)

type OnlineDetails struct {
	Store   string         `json:"store"`
	Edition online.Edition `json:"edition"`
	Fetched int64          `json:"fetched"`
}

func (o OnlineDetails) Stale(now time.Time) bool {
	return now.Sub(time.Unix(o.Fetched, 0)) > RefreshOnline
}

type savedOnline struct {
	Version  int                      `json:"version"`
	Editions map[string]OnlineDetails `json:"editions"`
	Searched map[string]Searched      `json:"searched,omitempty"`
}

type Searched struct {
	Query string `json:"query"`
	At    int64  `json:"at"`
}

func (d *Data) Searched(book string) Searched {
	return d.searched[book]
}

func (d *Data) SetSearched(book, query string) {
	d.searched[book] = Searched{Query: query, At: time.Now().Unix()}
	d.saveOnline()
}

func onlineKey(store, asin string) string {
	return store + ":" + asin
}

func (d *Data) onlineFile() string {
	if d.file == "" {
		return ""
	}

	return filepath.Join(filepath.Dir(d.file), onlineFileName)
}

func (d *Data) Online(book string) (OnlineDetails, bool) {
	edit := d.edits[book]
	if edit.ASIN == "" {
		return OnlineDetails{}, false
	}

	details, found := d.online[onlineKey(edit.Store, edit.ASIN)]
	return details, found
}

func (d *Data) LinkOnline(book, storeCode string, edition online.Edition) {
	d.UpdateOnline(storeCode, edition)

	previous := d.edits[book]
	edit := previous
	edit.ASIN, edit.Store, edit.NoAudible = edition.ASIN, storeCode, false
	d.SetEdit(book, edit)

	if previous.ASIN != "" && onlineKey(previous.Store, previous.ASIN) != onlineKey(storeCode, edition.ASIN) {
		d.pruneOnline()
	}
}

func (d *Data) UpdateOnline(storeCode string, edition online.Edition) {
	d.online[onlineKey(storeCode, edition.ASIN)] = OnlineDetails{
		Store:   storeCode,
		Edition: edition,
		Fetched: time.Now().Unix(),
	}
	d.saveOnline()
}

func (d *Data) ApplyOnline(details OnlineDetails) {
	key := onlineKey(details.Store, details.Edition.ASIN)
	if current, found := d.online[key]; found && current.Fetched >= details.Fetched {
		return
	}

	d.online[key] = details
	d.saveOnline()
}

func (d *Data) OnlineDetailsFor(store, asin string) (OnlineDetails, bool) {
	details, found := d.online[onlineKey(store, asin)]
	return details, found
}

func (d *Data) UnlinkOnline(book string) {
	edit := d.edits[book]
	if edit.ASIN == "" {
		return
	}

	edit.ASIN, edit.Store, edit.NoAudible = "", "", true
	d.SetEdit(book, edit)
	d.pruneOnline()
}

func (d *Data) pruneOnline() {
	used := map[string]bool{}
	for _, edit := range d.edits {
		if edit.ASIN != "" {
			used[onlineKey(edit.Store, edit.ASIN)] = true
		}
	}

	changed := false
	for key := range d.online {
		if !used[key] {
			delete(d.online, key)
			changed = true
		}
	}

	inLibrary := d.libraryPaths()
	for book := range d.searched {
		if !inLibrary[book] {
			delete(d.searched, book)
			changed = true
		}
	}

	if changed {
		d.saveOnline()
	}
}

func (d *Data) loadOnline() {
	file := d.onlineFile()
	if file == "" {
		return
	}

	var saved savedOnline
	if _, err := store.Load(file, &saved); err != nil && !errors.Is(err, os.ErrNotExist) {
		fyne.LogError("The saved Audible details couldn't be read, so books will be looked up again when opened", err)
	}
	if saved.Version == onlineVersion && saved.Editions != nil {
		d.online = saved.Editions
	}
	if saved.Version == onlineVersion && saved.Searched != nil {
		d.searched = saved.Searched
	}

	d.pruneOnline()
}

func (d *Data) saveOnline() {
	file := d.onlineFile()
	if file == "" {
		return
	}

	if err := store.Save(file, savedOnline{Version: onlineVersion, Editions: d.online, Searched: d.searched}); err != nil {
		fyne.LogError("Saving the Audible details failed", err)
	}
}
