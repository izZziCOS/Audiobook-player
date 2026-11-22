package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// NewAdaptiveRow puts leading and trailing side by side when the row is wide
// enough for both, and stacks trailing under leading, at full width, when it
// isn't. Its height depends on the width it was last given, so onChange is
// called after it switches between the two, to lay out its parent again.
func NewAdaptiveRow(onChange func(), leading, trailing fyne.CanvasObject) *fyne.Container {
	return container.New(&adaptiveRow{onChange: onChange}, leading, trailing)
}

type adaptiveRow struct {
	stacked  bool
	onChange func()
}

func (l *adaptiveRow) fits(objects []fyne.CanvasObject, width float32) bool {
	return objects[0].MinSize().Width+theme.Padding()+objects[1].MinSize().Width <= width
}

func (l *adaptiveRow) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	leading, trailing := objects[0], objects[1]
	lead, trail := leading.MinSize(), trailing.MinSize()

	stacked := !l.fits(objects, size.Width)
	if stacked {
		leading.Move(fyne.NewPos(0, 0))
		leading.Resize(fyne.NewSize(size.Width, lead.Height))
		trailing.Move(fyne.NewPos(0, lead.Height+theme.Padding()))
		trailing.Resize(fyne.NewSize(size.Width, trail.Height))
	} else {
		height := max(lead.Height, trail.Height)
		leading.Move(fyne.NewPos(0, (height-lead.Height)/2))
		leading.Resize(fyne.NewSize(size.Width-trail.Width-theme.Padding(), lead.Height))
		trailing.Move(fyne.NewPos(size.Width-trail.Width, (height-trail.Height)/2))
		trailing.Resize(trail)
	}

	if stacked != l.stacked {
		l.stacked = stacked
		if l.onChange != nil {
			fyne.Do(l.onChange)
		}
	}
}

func (l *adaptiveRow) MinSize(objects []fyne.CanvasObject) fyne.Size {
	lead, trail := objects[0].MinSize(), objects[1].MinSize()
	width := max(lead.Width, trail.Width)
	if l.stacked {
		return fyne.NewSize(width, lead.Height+theme.Padding()+trail.Height)
	}

	return fyne.NewSize(width, max(lead.Height, trail.Height))
}
