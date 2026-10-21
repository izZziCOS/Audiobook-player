package shelf

import (
	"fmt"

	"fyne.io/fyne/v2"

	"github.com/izzzicos/audiobook-player/ui/offline"
)

func (s *Shelf) withDownload(entry *shelfEntry, status string) string {
	switch s.Offline.State(entry.path) {
	case offline.Downloading:
		return fmt.Sprintf("Downloading %d%%", int(s.Offline.Progress(entry.path)*100))
	case offline.Paused:
		return status + " · Download paused"
	case offline.Downloaded:
		if entry.outOfDate {
			return status + " · Download out of date"
		}
		return status + " · Downloaded"
	default:
		return status
	}
}

func (s *Shelf) downloadItems(entry *shelfEntry) []*fyne.MenuItem {
	if s.Offline == nil {
		return nil
	}

	path := entry.path
	download := fyne.NewMenuItem("Download for offline listening", func() {
		s.Offline.Start(path)
	})
	remove := fyne.NewMenuItem("Remove download", func() {
		s.Offline.Remove(path)
	})

	switch s.Offline.State(path) {
	case offline.Downloading:
		return []*fyne.MenuItem{fyne.NewMenuItem("Cancel download", func() {
			s.Offline.Cancel(path)
		})}
	case offline.Paused:
		download.Label = "Resume download"
		return []*fyne.MenuItem{download, remove}
	case offline.Downloaded:
		if entry.outOfDate {
			download.Label = "Update download"
			return []*fyne.MenuItem{download, remove}
		}
		return []*fyne.MenuItem{remove}
	default:
		if !entry.network {
			return nil
		}
		return []*fyne.MenuItem{download}
	}
}

func (s *Shelf) DownloadChanged(path string) {
	entry := s.find(path)
	if entry == nil {
		return
	}

	if s.Offline.State(path) == offline.Downloaded {
		entry.outOfDate = !entry.offline && s.Offline.OutOfDate(path, entry.fingerprint)
	}
	s.updateCard(entry)
}
