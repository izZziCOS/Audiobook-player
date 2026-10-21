//go:build !windows

package titlebar

import "fyne.io/fyne/v2"

func Match(fyne.Window, Look) error {
	return nil
}
