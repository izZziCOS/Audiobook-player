package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/izzzicos/audiobook-player/audio"
	"github.com/izzzicos/audiobook-player/vfs"
)

const (
	recheckFor  = 10 * time.Minute
	scanVersion = 1
)

type Found struct {
	Path     string
	Parts    int
	Modified time.Time
}

type listedFile struct {
	path     string
	modified time.Time
}

type folderListing struct {
	modified   time.Time
	subfolders []string
	files      []listedFile
	newest     time.Time
}

type Scanner struct {
	mu      sync.Mutex
	folders map[string]folderListing
	now     func() time.Time
	file    string
	dirty   bool
}

func NewScanner() *Scanner {
	return &Scanner{folders: map[string]folderListing{}, now: time.Now}
}

type savedFile struct {
	Path     string    `json:"path"`
	Modified time.Time `json:"modified"`
}

type savedFolder struct {
	Modified   time.Time   `json:"modified"`
	Subfolders []string    `json:"subfolders,omitempty"`
	Files      []savedFile `json:"files,omitempty"`
}

type savedScan struct {
	Version int                    `json:"version"`
	Folders map[string]savedFolder `json:"folders"`
}

func SavedScanner(file string) *Scanner {
	scanner := NewScanner()
	scanner.file = file

	data, err := os.ReadFile(file)
	if err != nil {
		return scanner
	}

	var saved savedScan
	if json.Unmarshal(data, &saved) != nil || saved.Version != scanVersion {
		return scanner
	}

	for path, folder := range saved.Folders {
		listing := folderListing{modified: folder.Modified, subfolders: folder.Subfolders}
		for _, file := range folder.Files {
			listing.files = append(listing.files, listedFile{path: file.Path, modified: file.Modified})
		}
		listing.newest = newestOf(listing.files)
		scanner.folders[path] = listing
	}

	return scanner
}

func (s *Scanner) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == "" || !s.dirty {
		return nil
	}

	saved := savedScan{Version: scanVersion, Folders: map[string]savedFolder{}}
	for path, listing := range s.folders {
		folder := savedFolder{Modified: listing.modified, Subfolders: listing.subfolders}
		for _, file := range listing.files {
			folder.Files = append(folder.Files, savedFile{Path: file.path, Modified: file.modified})
		}
		saved.Folders[path] = folder
	}

	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if err := vfs.WriteAtomic(s.file, data); err != nil {
		return err
	}

	s.dirty = false
	return nil
}

func (s *Scanner) Forget() *Scanner {
	fresh := NewScanner()
	fresh.file = s.file
	fresh.dirty = true

	return fresh
}

func (s *Scanner) Scan(root string) ([]Found, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	root = normalize(root)
	info, err := vfs.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !audio.IsSupported(root) {
			return nil, errors.New("not a supported audio file")
		}
		return []Found{{Path: root, Parts: 1, Modified: info.ModTime()}}, nil
	}

	seen := map[string]bool{}
	var found []Found
	if err := s.walk(root, info.ModTime(), 0, seen, &found); err != nil {
		return nil, err
	}

	for folder := range s.folders {
		if !seen[folder] {
			delete(s.folders, folder)
			s.dirty = true
		}
	}

	return found, nil
}

func (s *Scanner) walk(folder string, modified time.Time, depth int, seen map[string]bool, found *[]Found) error {
	seen[folder] = true
	listing, err := s.listing(folder, modified)
	if err != nil {
		return err
	}

	names := make([]string, len(listing.files))
	for i, file := range listing.files {
		names[i] = file.path
	}

	switch {
	case len(listing.files) == 0:
	case WholeBooksOnly(names):
		for _, file := range listing.files {
			*found = append(*found, Found{Path: file.path, Parts: 1, Modified: file.modified})
		}
	default:
		*found = append(*found, Found{Path: folder, Parts: len(listing.files), Modified: listing.newest})
	}

	if depth >= maxScanDepth {
		return nil
	}

	for _, subfolder := range listing.subfolders {
		info, err := vfs.Stat(subfolder)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = s.walk(subfolder, info.ModTime(), depth+1, seen, found)
	}

	return nil
}

func (s *Scanner) listing(folder string, modified time.Time) (folderListing, error) {
	if cached, found := s.folders[folder]; found && cached.modified.Equal(modified) {
		if s.now().Sub(cached.newest) < recheckFor {
			restamped := restamp(cached)
			if !restamped.newest.Equal(cached.newest) {
				s.dirty = true
			}
			s.folders[folder] = restamped
			return restamped, nil
		}
		return cached, nil
	}

	entries, err := vfs.ReadDir(folder)
	if err != nil {
		return folderListing{}, err
	}

	listing := folderListing{modified: modified}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(folder, name)

		if entry.IsDir() {
			if !SkipFolder(name) {
				listing.subfolders = append(listing.subfolders, path)
			}
			continue
		}
		if strings.HasPrefix(name, ".") || !audio.IsSupported(name) {
			continue
		}

		file := listedFile{path: path}
		if info, err := entry.Info(); err == nil {
			file.modified = info.ModTime()
		}
		listing.files = append(listing.files, file)
	}

	sort.Slice(listing.files, func(i, j int) bool {
		return NaturalLess(filepath.Base(listing.files[i].path), filepath.Base(listing.files[j].path))
	})
	listing.newest = newestOf(listing.files)
	s.folders[folder] = listing
	s.dirty = true

	return listing, nil
}

func restamp(listing folderListing) folderListing {
	files := make([]listedFile, len(listing.files))
	for i, file := range listing.files {
		files[i] = file
		if info, err := vfs.Stat(file.path); err == nil {
			files[i].modified = info.ModTime()
		}
	}

	listing.files = files
	listing.newest = newestOf(files)
	return listing
}

func newestOf(files []listedFile) time.Time {
	var newest time.Time
	for _, file := range files {
		if file.modified.After(newest) {
			newest = file.modified
		}
	}

	return newest
}
