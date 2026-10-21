package offline

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/userdata"
	"github.com/izzzicos/audiobook-player/vfs"
)

const (
	folderName          = "offline"
	preferenceAutomatic = "keepCurrentDownloaded"
	retryAfter          = 2 * time.Minute
	copyChunk           = 1 << 20
	spaceMargin         = 200 << 20
	progressEvery       = 250 * time.Millisecond
)

var (
	errCancelled = errors.New("the download was cancelled")
	reachable    = library.Reachable
)

type job struct {
	cancelled atomic.Bool
	copied    atomic.Int64
	total     atomic.Int64
	automatic bool
}

type Store struct {
	OnChanged func(book string)
	Title     func(book string) (string, bool)

	window fyne.Window
	prefs  fyne.Preferences
	data   *userdata.Data
	async  func(func())
	folder string

	jobs     map[string]*job
	failures map[string]time.Time
	problems map[string]string

	section   fyne.CanvasObject
	usage     *widget.Label
	automatic *widget.Check
	removeAll *widget.Button
}

func New(window fyne.Window, prefs fyne.Preferences, data *userdata.Data, async func(func())) *Store {
	s := &Store{
		window:   window,
		prefs:    prefs,
		data:     data,
		async:    async,
		jobs:     map[string]*job{},
		failures: map[string]time.Time{},
		problems: map[string]string{},
	}
	if file := data.File(); file != "" {
		s.folder = filepath.Join(filepath.Dir(file), folderName)
	}
	s.build()

	return s
}

func (s *Store) Section() fyne.CanvasObject {
	return s.section
}

func (s *Store) build() {
	s.usage = widget.NewLabel("")
	s.usage.Wrapping = fyne.TextWrapWord

	s.automatic = widget.NewCheck("Keep the book I'm listening to downloaded", func(on bool) {
		s.prefs.SetBool(preferenceAutomatic, on)
	})
	s.automatic.SetChecked(s.prefs.Bool(preferenceAutomatic))

	s.removeAll = widget.NewButtonWithIcon("Remove all downloads", theme.DeleteIcon(), func() {
		dialog.ShowConfirm(
			"Remove all downloads?",
			"The copies on this computer are deleted. The books stay in your library and on your drive or server.",
			func(confirmed bool) {
				if confirmed {
					s.RemoveAll()
				}
			},
			s.window,
		)
	})
	s.removeAll.Importance = widget.LowImportance

	s.updateUsage()

	s.section = kit.Section(
		"Offline downloads",
		"Right-click a book on a drive or server and choose Download for offline listening to keep a copy on this computer. "+
			"A downloaded book always plays from that copy, so it works without the network or your VPN.",
		s.usage,
		s.automatic,
		container.NewHBox(s.removeAll),
	)
}

func (s *Store) updateUsage() {
	count, bytes := 0, int64(0)
	for _, download := range s.data.Downloads() {
		bytes += download.Bytes()
		if download.Complete {
			count++
		}
	}

	if count == 0 && bytes == 0 {
		s.usage.SetText("No books are downloaded.")
		s.removeAll.Hide()
		return
	}

	s.usage.SetText(fmt.Sprintf("%s · %s on this computer.", kit.CountPhrase(count, "book downloaded", "books downloaded", "No books downloaded yet"), Size(bytes)))
	s.removeAll.Show()
}

func Size(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%d MB", bytes>>20)
	default:
		return fmt.Sprintf("%d KB", max(bytes>>10, 1))
	}
}

func (s *Store) changed(book string) {
	s.updateUsage()
	if s.OnChanged != nil {
		s.OnChanged(book)
	}
}

type State int

const (
	NotDownloaded State = iota
	Downloading
	Paused
	Downloaded
)

func (s *Store) State(book string) State {
	if s == nil {
		return NotDownloaded
	}
	if s.jobs[book] != nil {
		return Downloading
	}

	download, found := s.data.Download(book)
	switch {
	case !found:
		return NotDownloaded
	case download.Complete:
		return Downloaded
	default:
		return Paused
	}
}

func (s *Store) Progress(book string) float64 {
	if s == nil {
		return 0
	}
	if job := s.jobs[book]; job != nil && job.total.Load() > 0 {
		return float64(job.copied.Load()) / float64(job.total.Load())
	}

	return 0
}

func (s *Store) Problem(book string) string {
	if s == nil {
		return ""
	}

	return s.problems[book]
}

func (s *Store) localPath(download userdata.Download, part userdata.DownloadPart) string {
	return filepath.Join(s.folder, download.Folder, part.File)
}

func (s *Store) Local(book string) *library.Book {
	if s == nil || s.folder == "" {
		return nil
	}

	download, found := s.data.Download(book)
	if !found || !download.Complete || len(download.Parts) == 0 {
		return nil
	}

	result := &library.Book{}
	if !download.Single {
		result.Folder = book
	}
	for _, part := range download.Parts {
		local := s.localPath(download, part)
		if info, err := os.Stat(local); err != nil || info.Size() != part.Size {
			return nil
		}
		result.Parts = append(result.Parts, part.Source)
		result.Local = append(result.Local, local)
	}

	return result
}

func (s *Store) Open(path string) (*library.Book, error) {
	if local := s.Local(library.Normalize(path)); local != nil {
		return local, nil
	}

	return library.OpenBook(path)
}

func (s *Store) Prepare(book *library.Book) {
	if s == nil || s.folder == "" {
		return
	}

	download, found := s.data.Download(book.Key())
	if !found {
		return
	}

	local := make([]string, len(book.Parts))
	any := false
	for i, source := range book.Parts {
		for _, part := range download.Parts {
			if part.Source != source || !part.Done {
				continue
			}
			path := s.localPath(download, part)
			if info, err := os.Stat(path); err == nil && info.Size() == part.Size {
				local[i] = path
				any = true
			}
		}
	}

	if any {
		book.Local = local
	}
}

func (s *Store) OutOfDate(book string, fingerprint *library.Fingerprint) bool {
	if s == nil || fingerprint == nil {
		return false
	}

	download, found := s.data.Download(book)
	if !found || !download.Complete {
		return false
	}

	if len(download.Parts) != len(fingerprint.Parts) {
		return true
	}
	for i, part := range fingerprint.Parts {
		if download.Parts[i].File != part.Name || download.Parts[i].Size != part.Size {
			return true
		}
	}

	return false
}

func (s *Store) Start(book string) {
	s.start(book, false, false)
}

func (s *Store) start(book string, automatic, onlyFromNetwork bool) {
	if s == nil || s.folder == "" || s.jobs[book] != nil {
		return
	}

	existing, _ := s.data.Download(book)
	current := &job{automatic: automatic}
	s.jobs[book] = current
	delete(s.problems, book)
	s.changed(book)

	s.async(func() {
		err := s.download(book, existing, current, onlyFromNetwork)

		fyne.Do(func() {
			delete(s.jobs, book)
			switch {
			case current.cancelled.Load():
				s.discard(book)
			case errors.Is(err, errNotOnNetwork):
			case err != nil:
				s.failures[book] = time.Now()
				s.problems[book] = problemText(err)
				fyne.LogError("Downloading a book for offline listening stopped", err)
				if !current.automatic && !connectionProblem(err) {
					dialog.ShowError(fmt.Errorf("%q couldn't be downloaded: %w", s.title(book), err), s.window)
				}
			default:
				delete(s.failures, book)
			}
			s.data.Save()
			s.changed(book)
		})
	})
}

var errNotOnNetwork = errors.New("the book is on this computer already")

func connectionProblem(err error) bool {
	return errors.Is(err, library.ErrNotResponding) || errors.Is(err, os.ErrNotExist)
}

func problemText(err error) string {
	if connectionProblem(err) {
		return "the drive or server isn't connected, so it will carry on once it's back"
	}

	return err.Error()
}

func (s *Store) title(book string) string {
	if s.Title != nil {
		if title, found := s.Title(book); found {
			return title
		}
	}

	return filepath.Base(book)
}

func (s *Store) download(book string, existing userdata.Download, current *job, onlyFromNetwork bool) error {
	if err := reachable(book, library.ResponseTimeout); err != nil {
		return err
	}
	if onlyFromNetwork && !library.IsNetwork(book) {
		return errNotOnNetwork
	}

	source, err := library.OpenBook(book)
	if err != nil {
		return err
	}

	download := userdata.Download{
		Folder:    existing.Folder,
		Single:    source.Folder == "",
		Automatic: current.automatic,
	}
	if download.Folder == "" {
		sum := sha256.Sum256([]byte(book))
		download.Folder = hex.EncodeToString(sum[:10])
	}

	var needed int64
	for _, path := range source.Parts {
		info, err := vfs.Stat(path)
		if err != nil {
			return err
		}

		part := userdata.DownloadPart{
			Source:   path,
			File:     filepath.Base(path),
			Size:     info.Size(),
			Modified: info.ModTime().UnixNano(),
		}
		part.Done = s.alreadyCopied(download, part)
		current.total.Add(part.Size)
		if part.Done {
			current.copied.Add(part.Size)
		} else {
			needed += part.Size
		}
		download.Parts = append(download.Parts, part)
	}

	directory := filepath.Join(s.folder, download.Folder)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	if free, known := library.FreeSpace(directory); known && free < needed+spaceMargin {
		return fmt.Errorf("there isn't enough space on this computer: it needs %s more", Size(needed+spaceMargin-free))
	}

	s.record(book, download)

	last := time.Now()
	report := func() {
		if time.Since(last) >= progressEvery {
			last = time.Now()
			fyne.Do(func() {
				s.changed(book)
			})
		}
	}

	for i, part := range download.Parts {
		if part.Done {
			continue
		}
		if err := copyPart(part, filepath.Join(directory, part.File), current, report); err != nil {
			return err
		}
		download.Parts[i].Done = true
		s.record(book, download)
	}

	download.Cover = copyCover(source, directory)
	download.Complete = true
	s.record(book, download)

	return nil
}

func (s *Store) alreadyCopied(download userdata.Download, part userdata.DownloadPart) bool {
	info, err := os.Stat(filepath.Join(s.folder, download.Folder, part.File))
	return err == nil && info.Size() == part.Size && info.ModTime().UnixNano() == part.Modified
}

func (s *Store) record(book string, download userdata.Download) {
	snapshot := download
	snapshot.Parts = append([]userdata.DownloadPart(nil), download.Parts...)
	fyne.Do(func() {
		s.data.SetDownload(book, snapshot)
	})
}

func copyPart(part userdata.DownloadPart, destination string, current *job, report func()) error {
	in, err := vfs.Open(part.Source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.CreateTemp(filepath.Dir(destination), filepath.Base(destination)+".*.part")
	if err != nil {
		return err
	}
	temporary := out.Name()
	defer os.Remove(temporary)

	buffer := make([]byte, copyChunk)
	for {
		if current.cancelled.Load() {
			out.Close()
			return errCancelled
		}

		n, readErr := in.Read(buffer)
		if n > 0 {
			if _, err := out.Write(buffer[:n]); err != nil {
				out.Close()
				return err
			}
			current.copied.Add(int64(n))
			report()
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			out.Close()
			return readErr
		}
	}

	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	modified := time.Unix(0, part.Modified)
	if err := os.Chtimes(temporary, modified, modified); err != nil {
		return err
	}

	return os.Rename(temporary, destination)
}

func copyCover(source *library.Book, directory string) string {
	metadata := library.ReadMetadata(source.Parts[0])
	cover, _ := library.FindCover(source.Parts[0], metadata.Picture)
	if cover == nil || cover.File() == "" {
		return ""
	}

	name := filepath.Base(cover.File())
	if err := os.WriteFile(filepath.Join(directory, name), cover.Data, 0o644); err != nil {
		fyne.LogError("Copying a book's cover for offline listening failed", err)
		return ""
	}

	return name
}

func (s *Store) Cancel(book string) {
	if current := s.jobs[book]; current != nil {
		current.cancelled.Store(true)
	}
}

func (s *Store) Remove(book string) {
	if s == nil {
		return
	}
	if current := s.jobs[book]; current != nil {
		current.cancelled.Store(true)
		return
	}

	s.discard(book)
	s.data.Save()
	s.changed(book)
}

func (s *Store) discard(book string) {
	download, found := s.data.Download(book)
	if !found {
		return
	}

	if download.Folder != "" {
		if err := os.RemoveAll(filepath.Join(s.folder, download.Folder)); err != nil {
			fyne.LogError("Removing a downloaded book failed", err)
		}
	}
	s.data.ForgetDownload(book)
	delete(s.failures, book)
	delete(s.problems, book)
}

func (s *Store) RemoveAll() {
	for book := range s.data.Downloads() {
		s.Remove(book)
	}
}

func (s *Store) Load() {
	if s == nil || s.folder == "" {
		return
	}

	used := map[string]bool{}
	for _, download := range s.data.Downloads() {
		used[download.Folder] = true
	}
	folder := s.folder

	s.async(func() {
		entries, err := os.ReadDir(folder)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() && !used[entry.Name()] {
				os.RemoveAll(filepath.Join(folder, entry.Name()))
			}
		}
	})

	s.updateUsage()
}

func (s *Store) Tick(now time.Time) {
	if s == nil {
		return
	}

	for book, download := range s.data.Downloads() {
		if download.Complete || s.jobs[book] != nil || now.Sub(s.failures[book]) < retryAfter {
			continue
		}
		s.start(book, download.Automatic, false)
	}
}

func (s *Store) BookOpened(book string) {
	if s == nil || !s.prefs.Bool(preferenceAutomatic) {
		return
	}

	for other, download := range s.data.Downloads() {
		if other != book && download.Automatic {
			s.Remove(other)
		}
	}

	if s.State(book) == NotDownloaded {
		s.start(book, true, true)
	}
}
