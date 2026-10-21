package userdata

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/izzzicos/audiobook-player/library"
)

const (
	AllBooks          = "All books"
	maxCollectionName = 60
)

type Collection struct {
	ID        string                `json:"id,omitempty"`
	Name      string                `json:"name"`
	Books     []string              `json:"books,omitempty"`
	Elsewhere []library.Fingerprint `json:"elsewhere,omitempty"`
	Changed   int64                 `json:"changed,omitempty"`
}

func newCollectionID() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (d *Data) changed(collection *Collection) {
	collection.Changed = time.Now().UnixNano()
	d.revision++
}

func (d *Data) Collections() []string {
	names := make([]string, len(d.collections))
	for i, collection := range d.collections {
		names[i] = collection.Name
	}

	return names
}

func (d *Data) collection(name string) *Collection {
	for i := range d.collections {
		if d.collections[i].Name == name {
			return &d.collections[i]
		}
	}

	return nil
}

func (d *Data) CheckCollectionName(name, renaming string) error {
	name = strings.TrimSpace(name)

	switch {
	case name == "":
		return errors.New("enter a name for the collection")
	case utf8.RuneCountInString(name) > maxCollectionName:
		return fmt.Errorf("use a name of %d characters or fewer", maxCollectionName)
	case strings.EqualFold(name, AllBooks):
		return fmt.Errorf("“%s” is already used for the whole library", AllBooks)
	}

	for _, collection := range d.collections {
		if collection.Name != renaming && strings.EqualFold(collection.Name, name) {
			return fmt.Errorf("there's already a collection called “%s”", collection.Name)
		}
	}

	return nil
}

func (d *Data) CreateCollection(name string) (string, error) {
	if err := d.CheckCollectionName(name, ""); err != nil {
		return "", err
	}

	name = strings.TrimSpace(name)
	d.collections = append(d.collections, Collection{ID: newCollectionID(), Name: name})
	d.changed(&d.collections[len(d.collections)-1])
	d.Save()

	return name, nil
}

func (d *Data) RenameCollection(from, to string) (string, error) {
	collection := d.collection(from)
	if collection == nil {
		return "", fmt.Errorf("there's no collection called “%s”", from)
	}
	if err := d.CheckCollectionName(to, from); err != nil {
		return "", err
	}

	collection.Name = strings.TrimSpace(to)
	d.changed(collection)
	d.Save()

	return collection.Name, nil
}

func (d *Data) DeleteCollection(name string) {
	d.collections = slices.DeleteFunc(d.collections, func(collection Collection) bool {
		if collection.Name == name {
			d.deletedCollections[collection.ID] = time.Now().UnixNano()
			return true
		}
		return false
	})
	d.revision++
	d.Save()
}

func (d *Data) InCollection(name, book string) bool {
	collection := d.collection(name)
	return collection != nil && slices.Contains(collection.Books, book)
}

func (d *Data) SetInCollection(name, book string, member bool) {
	collection := d.collection(name)
	if collection == nil || slices.Contains(collection.Books, book) == member {
		return
	}

	if member {
		collection.Books = append(collection.Books, book)
	} else {
		collection.Books = slices.DeleteFunc(collection.Books, func(other string) bool {
			return other == book
		})
	}
	d.changed(collection)
	d.Save()
}

func (d *Data) SyncedCollections() ([]Collection, map[string]int64) {
	collections := make([]Collection, len(d.collections))
	for i, collection := range d.collections {
		collections[i] = collection
		collections[i].Books = slices.Clone(collection.Books)
		collections[i].Elsewhere = slices.Clone(collection.Elsewhere)
	}

	deleted := make(map[string]int64, len(d.deletedCollections))
	for id, when := range d.deletedCollections {
		deleted[id] = when
	}

	return collections, deleted
}

func (d *Data) ApplyCollection(incoming Collection) bool {
	if incoming.ID == "" || d.deletedCollections[incoming.ID] >= incoming.Changed {
		return false
	}

	for i := range d.collections {
		if d.collections[i].ID != incoming.ID {
			continue
		}
		if d.collections[i].Changed >= incoming.Changed {
			return false
		}
		d.collections[i] = incoming
		d.mergeSameNames()
		return true
	}

	d.collections = append(d.collections, incoming)
	d.mergeSameNames()
	return true
}

func (d *Data) ApplyDeletion(id string, when int64) bool {
	if when <= d.deletedCollections[id] {
		return false
	}
	d.deletedCollections[id] = when

	before := len(d.collections)
	d.collections = slices.DeleteFunc(d.collections, func(collection Collection) bool {
		return collection.ID == id && collection.Changed < when
	})

	return len(d.collections) != before
}

func (d *Data) mergeSameNames() {
	for i := 0; i < len(d.collections); i++ {
		for j := i + 1; j < len(d.collections); j++ {
			a, b := &d.collections[i], &d.collections[j]
			if !strings.EqualFold(a.Name, b.Name) {
				continue
			}

			keep, drop := a, b
			if b.ID < a.ID {
				keep, drop = b, a
			}
			for _, book := range drop.Books {
				if !slices.Contains(keep.Books, book) {
					keep.Books = append(keep.Books, book)
				}
			}
			keep.Elsewhere = append(keep.Elsewhere, drop.Elsewhere...)
			keep.Changed = max(keep.Changed, drop.Changed)
			d.deletedCollections[drop.ID] = keep.Changed
			dropped := drop.ID

			d.collections = slices.DeleteFunc(d.collections, func(collection Collection) bool {
				return collection.ID == dropped
			})
			j = i
		}
	}
}

func (d *Data) moveInCollections(old, newPath string) {
	for i := range d.collections {
		books := d.collections[i].Books
		if !slices.Contains(books, old) {
			continue
		}

		books = slices.DeleteFunc(books, func(book string) bool {
			return book == old
		})
		if !slices.Contains(books, newPath) {
			books = append(books, newPath)
		}
		d.collections[i].Books = books
	}
}

func (d *Data) forgetInCollections(book string) bool {
	changed := false
	for i := range d.collections {
		before := len(d.collections[i].Books)
		d.collections[i].Books = slices.DeleteFunc(d.collections[i].Books, func(other string) bool {
			return other == book
		})
		changed = changed || len(d.collections[i].Books) != before
	}

	return changed
}
