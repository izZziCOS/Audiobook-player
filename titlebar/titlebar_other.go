//go:build !windows

package titlebar

import "fyne.io/fyne/v2"

func SetDark(window fyne.Window, dark bool) error {
    return nil
}
