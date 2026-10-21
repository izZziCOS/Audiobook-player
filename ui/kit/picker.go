package kit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"github.com/ncruces/zenity"
)

type Picker struct {
	busy atomic.Bool
}

func Browse[T any](picker *Picker, window fyne.Window, choose func() (T, error), chosen func(T), fallback func()) {
	if !picker.busy.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer picker.busy.Store(false)

		value, err := choose()

		fyne.Do(func() {
			window.RequestFocus()

			switch {
			case errors.Is(err, zenity.ErrCanceled):
			case err != nil:
				fallback()
			default:
				chosen(value)
			}
		})
	}()
}

func HomeFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}

func WithTrailingSeparator(folder string) string {
	if folder == "" || strings.HasSuffix(folder, string(filepath.Separator)) {
		return folder
	}

	return folder + string(filepath.Separator)
}

func ShowFallbackDialog(window fyne.Window, fileDialog *dialog.FileDialog, folder string) {
	if location, err := storage.ListerForURI(storage.NewFileURI(folder)); err == nil {
		fileDialog.SetLocation(location)
	}

	fileDialog.Resize(window.Canvas().Size().Subtract(fyne.NewSquareSize(40)))
	fileDialog.Show()
}

func ShowFolderDialog(window fyne.Window, folder string, chosen func(string)) {
	folderDialog := dialog.NewFolderOpen(
		func(lister fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, window)
				return
			}

			if lister != nil {
				chosen(lister.Path())
			}
		},
		window,
	)

	ShowFallbackDialog(window, folderDialog, folder)
}
