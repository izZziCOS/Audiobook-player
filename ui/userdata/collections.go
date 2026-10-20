package userdata

import (
    "errors"
    "fmt"
    "slices"
    "strings"
    "unicode/utf8"
)

const (
    AllBooks          = "All books"
    maxCollectionName = 60
)

type Collection struct {
    Name  string   `json:"name"`
    Books []string `json:"books,omitempty"`
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
    d.collections = append(d.collections, Collection{Name: name})
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
    d.Save()

    return collection.Name, nil
}

func (d *Data) DeleteCollection(name string) {
    d.collections = slices.DeleteFunc(d.collections, func(collection Collection) bool {
        return collection.Name == name
    })
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
    d.Save()
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
