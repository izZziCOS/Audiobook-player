package shelf

import (
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "image"
    "image/jpeg"
    "image/png"
    "maps"
    "os"
    "path/filepath"
    "strings"
    "sync"

    "fyne.io/fyne/v2"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/store"
    "github.com/izzzicos/audiobook-player/ui/details"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const (
    cacheFileName   = "shelf-cache.json"
    thumbnailFolder = "thumbnails"
    cacheVersion    = 1
)

type fileStamp struct {
    Size     int64 `json:"size"`
    Modified int64 `json:"modified"`
}

func stampOf(path string) (fileStamp, bool) {
    info, err := os.Stat(path)
    if err != nil {
        return fileStamp{}, false
    }

    return fileStamp{Size: info.Size(), Modified: info.ModTime().UnixNano()}, true
}

type cachedBook struct {
    FirstPart string    `json:"firstPart"`
    Audio     fileStamp `json:"audio"`
    Folder    fileStamp `json:"folder"`
    CoverFile string    `json:"coverFile,omitempty"`
    Cover     fileStamp `json:"cover"`
    Title     string    `json:"title"`
    Author    string    `json:"author,omitempty"`
    Thumbnail string    `json:"thumbnail,omitempty"`
}

type savedCache struct {
    Version int                   `json:"version"`
    Books   map[string]cachedBook `json:"books"`
    Covers  map[string]string     `json:"covers,omitempty"`
}

type shelfCache struct {
    file   string
    folder string

    mu     sync.Mutex
    books  map[string]cachedBook
    covers map[string]string
    dirty  bool
}

func newShelfCache(appFolder string) *shelfCache {
    if appFolder == "" {
        return nil
    }

    return &shelfCache{
        file:   filepath.Join(appFolder, cacheFileName),
        folder: filepath.Join(appFolder, thumbnailFolder),
        books:  map[string]cachedBook{},
        covers: map[string]string{},
    }
}

func (c *shelfCache) load() {
    if c == nil {
        return
    }

    var saved savedCache
    if _, err := store.Load(c.file, &saved); err != nil && !errors.Is(err, os.ErrNotExist) {
        fyne.LogError("The library's cover cache couldn't be read, so covers will be loaded again", err)
    }

    c.mu.Lock()
    defer c.mu.Unlock()

    if saved.Version != cacheVersion {
        return
    }
    if saved.Books != nil {
        c.books = saved.Books
    }
    if saved.Covers != nil {
        c.covers = saved.Covers
    }
}

func (c *shelfCache) save() {
    if c == nil {
        return
    }

    c.mu.Lock()
    if !c.dirty {
        c.mu.Unlock()
        return
    }
    snapshot := savedCache{Version: cacheVersion, Books: maps.Clone(c.books), Covers: maps.Clone(c.covers)}
    c.dirty = false
    c.mu.Unlock()

    if err := store.Save(c.file, snapshot); err != nil {
        fyne.LogError("Saving the library's cover cache failed", err)
    }
}

func (c *shelfCache) bookInfo(path string, book *library.Book) (details.Original, bool) {
    if c == nil {
        return details.Original{}, false
    }

    c.mu.Lock()
    cached, found := c.books[path]
    c.mu.Unlock()
    if !found || cached.FirstPart != book.Parts[0] || !cached.fresh() {
        return details.Original{}, false
    }

    original := details.Original{Title: cached.Title, Author: cached.Author}
    if cached.Thumbnail != "" {
        original.Cover = c.readThumbnail(cached.Thumbnail)
        if original.Cover == nil {
            return details.Original{}, false
        }
    }

    return original, true
}

func (cached cachedBook) fresh() bool {
    if audio, ok := stampOf(cached.FirstPart); !ok || audio != cached.Audio {
        return false
    }
    if folder, ok := stampOf(filepath.Dir(cached.FirstPart)); !ok || folder != cached.Folder {
        return false
    }
    if cached.CoverFile != "" {
        if cover, ok := stampOf(cached.CoverFile); !ok || cover != cached.Cover {
            return false
        }
    }

    return true
}

func (c *shelfCache) rememberBook(path string, book *library.Book, original details.Original, coverFile string) {
    if c == nil {
        return
    }

    firstPart := book.Parts[0]
    cached := cachedBook{FirstPart: firstPart, CoverFile: coverFile, Title: original.Title, Author: original.Author}
    var ok bool
    if cached.Audio, ok = stampOf(firstPart); !ok {
        return
    }
    if cached.Folder, ok = stampOf(filepath.Dir(firstPart)); !ok {
        return
    }
    if coverFile != "" {
        if cached.Cover, ok = stampOf(coverFile); !ok {
            return
        }
    }

    var encoded []byte
    var extension string
    if original.Cover != nil {
        var err error
        if encoded, extension, err = encodeThumbnail(original.Cover); err != nil {
            fyne.LogError("Caching a cover failed", err)
            return
        }
        cached.Thumbnail = thumbnailName(fmt.Sprintf("%s\x00%+v\x00%+v", path, cached.Audio, cached.Cover)) + extension
    }

    c.mu.Lock()
    defer c.mu.Unlock()

    if encoded != nil && !c.writeThumbnail(cached.Thumbnail, encoded) {
        return
    }
    if previous := c.books[path].Thumbnail; previous != "" && previous != cached.Thumbnail {
        c.removeThumbnail(previous)
    }
    c.books[path] = cached
    c.dirty = true
}

func (c *shelfCache) customThumbnail(coverFile string) image.Image {
    if coverFile == "" {
        return nil
    }

    name := filepath.Base(coverFile)
    if c != nil {
        c.mu.Lock()
        cached := c.covers[name]
        c.mu.Unlock()
        if cached != "" {
            if thumbnail := c.readThumbnail(cached); thumbnail != nil {
                return thumbnail
            }
        }
    }

    custom := userdata.ReadCustomCover(coverFile)
    if custom == nil {
        return nil
    }
    thumbnail, err := library.Thumbnail(custom, thumbnailSide)
    if err != nil {
        return nil
    }

    if c != nil {
        if encoded, extension, err := encodeThumbnail(thumbnail); err == nil {
            cached := "custom-" + strings.TrimSuffix(name, filepath.Ext(name)) + extension
            c.mu.Lock()
            if c.writeThumbnail(cached, encoded) {
                c.covers[name] = cached
                c.dirty = true
            }
            c.mu.Unlock()
        }
    }

    return thumbnail
}

func (c *shelfCache) prune(books, covers map[string]bool) {
    if c == nil {
        return
    }

    c.mu.Lock()
    defer c.mu.Unlock()

    used := map[string]bool{}
    for path, cached := range c.books {
        if !books[path] {
            delete(c.books, path)
            c.dirty = true
            continue
        }
        used[cached.Thumbnail] = true
    }
    for name, cached := range c.covers {
        if !covers[name] {
            delete(c.covers, name)
            c.dirty = true
            continue
        }
        used[cached] = true
    }

    entries, err := os.ReadDir(c.folder)
    if err != nil {
        return
    }
    for _, entry := range entries {
        if !entry.IsDir() && !used[entry.Name()] {
            c.removeThumbnail(entry.Name())
        }
    }
}

func (c *shelfCache) readThumbnail(name string) image.Image {
    file, err := os.Open(filepath.Join(c.folder, name))
    if err != nil {
        return nil
    }
    defer file.Close()

    thumbnail, _, err := image.Decode(file)
    if err != nil {
        return nil
    }

    return thumbnail
}

func (c *shelfCache) writeThumbnail(name string, encoded []byte) bool {
    if err := os.MkdirAll(c.folder, 0o755); err != nil {
        fyne.LogError("Caching a cover failed", err)
        return false
    }

    file := filepath.Join(c.folder, name)
    temporary, err := os.CreateTemp(c.folder, name+".*.tmp")
    if err != nil {
        fyne.LogError("Caching a cover failed", err)
        return false
    }
    _, writeErr := temporary.Write(encoded)
    closeErr := temporary.Close()
    if err := errors.Join(writeErr, closeErr); err != nil {
        os.Remove(temporary.Name())
        fyne.LogError("Caching a cover failed", err)
        return false
    }
    if err := os.Rename(temporary.Name(), file); err != nil {
        os.Remove(temporary.Name())
        fyne.LogError("Caching a cover failed", err)
        return false
    }

    return true
}

func (c *shelfCache) removeThumbnail(name string) {
    if err := os.Remove(filepath.Join(c.folder, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
        fyne.LogError("Removing an unused cached cover failed", err)
    }
}

func thumbnailName(source string) string {
    sum := sha256.Sum256([]byte(source))
    return hex.EncodeToString(sum[:12])
}

func encodeThumbnail(thumbnail image.Image) ([]byte, string, error) {
    var encoded bytes.Buffer

    if opaque, ok := thumbnail.(interface{ Opaque() bool }); ok && !opaque.Opaque() {
        err := png.Encode(&encoded, thumbnail)
        return encoded.Bytes(), ".png", err
    }

    err := jpeg.Encode(&encoded, thumbnail, &jpeg.Options{Quality: 88})
    return encoded.Bytes(), ".jpg", err
}
