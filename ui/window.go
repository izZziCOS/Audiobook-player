package ui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"

	"github.com/izzzicos/audiobook-player/audio"
	"github.com/izzzicos/audiobook-player/ui/autoimport"
	"github.com/izzzicos/audiobook-player/ui/kit"
)

const (
	progressInterval = 250 * time.Millisecond
	saveInterval     = 5 * time.Second
)

func (ui *UI) buildTabs() fyne.CanvasObject {
	ui.playerTab = container.NewTabItemWithIcon("Now Playing", theme.MediaPlayIcon(), ui.player.Content())
	ui.libraryTab = container.NewTabItemWithIcon("Library", theme.GridIcon(), ui.shelf.Content())
	ui.statsTab = container.NewTabItemWithIcon("Statistics", theme.NewThemedResource(kit.StatisticsIcon), ui.statistics.Content())
	ui.settingsTab = container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), ui.settings.Content())

	ui.tabs = container.NewAppTabs(ui.playerTab, ui.libraryTab, ui.statsTab, ui.settingsTab)
	ui.tabs.OnSelected = func(tab *container.TabItem) {
		ui.player.ShowBackdrop(tab == ui.playerTab)

		if tab == ui.libraryTab {
			ui.shelf.Refresh()
			ui.shelf.RecheckMissing()
			ui.importer.ScanIfStale()
		}
		if tab == ui.statsTab {
			ui.statistics.Refresh()
		}
		ui.Window.Canvas().Unfocus()
	}

	return container.NewStack(ui.player.Backdrop(), container.NewPadded(ui.tabs))
}

func (ui *UI) showLibrary() {
	ui.tabs.Select(ui.libraryTab)
}

func (ui *UI) playFromShelf(path string) error {
	if current := ui.player.Book(); current != nil && current.Key() == path {
		ui.tabs.Select(ui.playerTab)
		ui.player.Play()
		return nil
	}

	book, err := ui.offline.Open(path)
	if err != nil {
		return err
	}

	ui.tabs.Select(ui.playerTab)
	ui.player.OpenBookAndPlay(book)
	return nil
}

func (ui *UI) addShortcuts() {
	ui.Window.Canvas().AddShortcut(
		&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierShortcutDefault},
		func(fyne.Shortcut) {
			ui.focusLibrarySearch()
		},
	)
}

func (ui *UI) focusLibrarySearch() {
	ui.showLibrary()
	ui.shelf.FocusSearch()
}

func (ui *UI) handleDrop(_ fyne.Position, uris []fyne.URI) {
	if ui.tabs.Selected() == ui.libraryTab {
		paths := make([]string, len(uris))
		for i, uri := range uris {
			paths[i] = uri.Path()
		}

		ui.shelf.AddFrom(paths)
		return
	}

	for _, uri := range uris {
		if info, err := os.Stat(uri.Path()); err == nil && info.IsDir() {
			ui.player.OpenFolder(uri.Path())
			return
		}

		if audio.IsSupported(uri.Name()) {
			ui.player.OpenFile(uri.Path())
			return
		}
	}

	dialog.ShowError(
		fmt.Errorf("only MP3, M4B, M4A, WAV, FLAC and Ogg Vorbis files, or folders of them, can be opened"),
		ui.Window,
	)
}

func (ui *UI) currentKey() string {
	if book := ui.player.Book(); book != nil {
		return book.Key()
	}

	return ""
}

func (ui *UI) everyProgressTick(now time.Time) {
	ui.player.Tick(now)
	ui.statistics.Record(now, ui.player.Playing(), ui.currentKey(), ui.player.Speed())
}

func (ui *UI) everySaveTick() {
	if ui.player.Playing() {
		ui.player.SavePosition()
		if ui.tabs.Selected() == ui.statsTab {
			ui.statistics.Refresh()
		}
	}

	ui.data.Flush()
	ui.syncer.Tick(time.Now())
	ui.offline.Tick(time.Now())
	ui.about.MatchAutomatically(ui.shelf.AudibleCandidates())
}

func (ui *UI) tick() {
	progressTicker := time.NewTicker(progressInterval)
	defer progressTicker.Stop()

	saveTicker := time.NewTicker(saveInterval)
	defer saveTicker.Stop()

	importTicker := time.NewTicker(autoimport.Interval)
	defer importTicker.Stop()

	for {
		select {
		case now := <-progressTicker.C:
			fyne.Do(func() {
				ui.everyProgressTick(now)
			})
		case <-saveTicker.C:
			fyne.Do(ui.everySaveTick)
		case <-importTicker.C:
			fyne.Do(ui.importer.ScanIfDue)
		}
	}
}
