package syncing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

type idlePlayer struct{}

func (idlePlayer) Book() *library.Book         { return nil }
func (idlePlayer) Playing() bool               { return false }
func (idlePlayer) MoveTo(string, float64) bool { return false }

func newTestSyncer(t *testing.T) *Syncer {
	t.Helper()

	app := test.NewApp()
	t.Cleanup(app.Quit)

	return New(app.NewWindow("Settings"), app.Preferences(), userdata.New(userdata.FileIn(t.TempDir())), &kit.Picker{}, func(work func()) {
		work()
	}, idlePlayer{})
}

func TestAnUnreachableSyncFolderKeepsYourPlace(t *testing.T) {
	s := newTestSyncer(t)
	original := reachable
	reachable = func(string, time.Duration) error { return library.ErrNotResponding }
	t.Cleanup(func() { reachable = original })

	s.SetFolder(t.TempDir())

	if !strings.Contains(s.status.Text, "isn't responding") || !strings.Contains(s.status.Text, "kept here") {
		t.Fatalf("status %q", s.status.Text)
	}
	if s.writing.Load() || s.pulling.Load() {
		t.Fatal("the next sync should still be able to run")
	}
}

func TestOtherComputersFilesAreReadSafely(t *testing.T) {
	s := newTestSyncer(t)
	folder := t.TempDir()
	s.SetFolder(folder)

	directory := filepath.Join(folder, syncFolderName)
	own := filepath.Join(directory, s.deviceID()+".json")
	if _, err := os.Stat(own); err != nil {
		t.Fatalf("choosing a folder should save this computer's file: %v", err)
	}
	os.WriteFile(filepath.Join(directory, "broken.json"), []byte("{not json"), 0o644)
	os.WriteFile(filepath.Join(directory, "other.json"), []byte(`{"version": 1, "id": "other", "name": "Windows PC", "books": []}`), 0o644)
	os.WriteFile(filepath.Join(directory, "future.json"), []byte(`{"version": 99, "id": "future", "name": "Newer app"}`), 0o644)

	devices, err := s.read(folder, s.deviceID())
	if err != nil || len(devices) != 1 || devices[0].Name != "Windows PC" {
		t.Fatalf("got %+v, %v, want only the other computer's readable file", devices, err)
	}

	s.Pull(nil)
	if !strings.Contains(s.status.Text, "with Windows PC") {
		t.Fatalf("status %q", s.status.Text)
	}

	s.SetFolder("")
	if s.Enabled() || !strings.Contains(s.status.Text, "only kept on this computer") {
		t.Fatal("stopping should say progress is only kept here")
	}
}

func TestAgo(t *testing.T) {
	now := time.Date(2026, 10, 21, 18, 0, 0, 0, time.Local)
	for listened, want := range map[time.Time]string{
		now.Add(-20 * time.Second): "just now",
		now.Add(-5 * time.Minute):  "5 minutes ago",
		now.Add(-1 * time.Hour):    "1 hour ago",
		now.Add(-72 * time.Hour):   "on 18 October",
	} {
		if got := ago(listened.Unix(), now); got != want {
			t.Errorf("ago(%v) = %q, want %q", listened, got, want)
		}
	}
}
