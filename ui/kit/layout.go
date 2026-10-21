package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func Gap() fyne.CanvasObject {
	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(0, theme.Padding()*2))
	return gap
}

func Heading(title string) *widget.Label {
	heading := widget.NewLabel(title)
	heading.TextStyle = fyne.TextStyle{Bold: true}
	heading.SizeName = theme.SizeNameSubHeadingText
	return heading
}

func Section(title, description string, content ...fyne.CanvasObject) fyne.CanvasObject {
	text := widget.NewLabel(description)
	text.Wrapping = fyne.TextWrapWord
	text.Importance = widget.LowImportance

	return container.NewVBox(append([]fyne.CanvasObject{Heading(title), text}, content...)...)
}

func FollowTheme(apply func()) {
	fyne.CurrentApp().Settings().AddListener(func(fyne.Settings) {
		apply()
	})
}

func RefreshContent(window fyne.Window) {
	if content := window.Content(); content != nil {
		content.Refresh()
	}
}

func ShowMenuAbove(window fyne.Window, anchor fyne.CanvasObject, menu *fyne.Menu) {
	popUp := widget.NewPopUpMenu(menu, window.Canvas())
	position := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	size := popUp.MinSize()

	popUp.ShowAtPosition(fyne.NewPos(
		max(position.X+anchor.Size().Width-size.Width, 0),
		max(position.Y-size.Height, 0),
	))
}

func ShowMenuBelow(window fyne.Window, anchor fyne.CanvasObject, menu *fyne.Menu) {
	popUp := widget.NewPopUpMenu(menu, window.Canvas())
	position := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)

	popUp.ShowAtPosition(position.AddXY(0, anchor.Size().Height))
}

func HighlightChoice(buttons map[string]*widget.Button, chosen string) {
	for option, button := range buttons {
		if option == chosen {
			button.Importance = widget.HighImportance
		} else {
			button.Importance = widget.MediumImportance
		}
		button.Refresh()
	}
}
