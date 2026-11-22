package shelf

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	cardWidth         = 172
	cardProgressGap   = 6
	cardProgressThick = 4
)

type bookCard struct {
	widget.BaseWidget

	Title     string
	Author    string
	Status    string
	Rating    string
	Progress  float64
	Thumbnail image.Image
	OnTapped  func()
	Menu      func() *fyne.Menu
	OnFocused func()

	hovered bool
	focused bool
}

func newBookCard(onTapped func(), menu func() *fyne.Menu) *bookCard {
	card := &bookCard{OnTapped: onTapped, Menu: menu}
	card.ExtendBaseWidget(card)

	return card
}

func (c *bookCard) Tapped(*fyne.PointEvent) {
	if c.OnTapped != nil {
		c.OnTapped()
	}
}

func (c *bookCard) TappedSecondary(event *fyne.PointEvent) {
	c.showMenuAt(event.AbsolutePosition)
}

func (c *bookCard) showMenuAt(position fyne.Position) {
	if c.Menu == nil {
		return
	}

	canvas := fyne.CurrentApp().Driver().CanvasForObject(c)
	if canvas == nil {
		return
	}

	widget.ShowPopUpMenuAtPosition(c.Menu(), canvas, position)
}

func (c *bookCard) FocusGained() {
	c.focused = true
	c.Refresh()
	if c.OnFocused != nil {
		c.OnFocused()
	}
}

func (c *bookCard) FocusLost() {
	c.focused = false
	c.Refresh()
}

func (c *bookCard) TypedRune(rune) {}

func (c *bookCard) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		c.Tapped(nil)
	}
}

func (c *bookCard) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (c *bookCard) MouseIn(*desktop.MouseEvent) {
	c.hovered = true
	c.Refresh()
}

func (c *bookCard) MouseMoved(*desktop.MouseEvent) {}

func (c *bookCard) MouseOut() {
	c.hovered = false
	c.Refresh()
}

func (c *bookCard) CreateRenderer() fyne.WidgetRenderer {
	title := widget.NewLabel("")
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Wrapping = fyne.TextWrapWord
	title.Truncation = fyne.TextTruncateEllipsis

	author := widget.NewLabel("")
	author.Truncation = fyne.TextTruncateEllipsis
	author.SizeName = theme.SizeNameCaptionText

	status := widget.NewLabel("")
	status.Truncation = fyne.TextTruncateEllipsis
	status.SizeName = theme.SizeNameCaptionText
	status.Importance = widget.LowImportance

	rating := widget.NewLabel("")
	rating.SizeName = theme.SizeNameCaptionText
	rating.Importance = widget.LowImportance

	cover := canvas.NewImageFromImage(nil)
	cover.FillMode = canvas.ImageFillContain
	cover.CornerRadius = 12

	placeholderBackground := canvas.NewRectangle(color.Transparent)
	placeholderBackground.CornerRadius = 12

	background := canvas.NewRectangle(color.Transparent)
	background.CornerRadius = 12
	background.StrokeWidth = 2

	var more *widget.Button
	more = widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() {
		position := fyne.CurrentApp().Driver().AbsolutePositionForObject(more)
		c.showMenuAt(position.AddXY(0, more.Size().Height))
	})
	more.Importance = widget.LowImportance

	renderer := &bookCardRenderer{
		card:                  c,
		background:            background,
		placeholderBackground: placeholderBackground,
		placeholderIcon: canvas.NewImageFromResource(
			theme.NewColoredResource(theme.MediaMusicIcon(), theme.ColorNamePlaceHolder),
		),
		cover:  cover,
		title:  title,
		author: author,
		status: status,
		rating: rating,
		more:   more,
		track:  canvas.NewRectangle(color.Transparent),
		fill:   canvas.NewRectangle(color.Transparent),
	}
	renderer.track.CornerRadius = cardProgressThick / 2
	renderer.fill.CornerRadius = cardProgressThick / 2
	renderer.Refresh()

	return renderer
}

type bookCardRenderer struct {
	card                  *bookCard
	background            *canvas.Rectangle
	placeholderBackground *canvas.Rectangle
	placeholderIcon       *canvas.Image
	cover                 *canvas.Image
	title                 *widget.Label
	author                *widget.Label
	status                *widget.Label
	rating                *widget.Label
	more                  *widget.Button
	track                 *canvas.Rectangle
	fill                  *canvas.Rectangle
}

func (r *bookCardRenderer) Layout(size fyne.Size) {
	padding := theme.Padding()
	innerPadding := theme.InnerPadding()
	side := size.Width - 2*padding
	coverOrigin := fyne.NewPos(padding, padding)
	labelX := padding - innerPadding/2
	labelWidth := size.Width - 2*padding + innerPadding

	r.background.Resize(size)

	r.placeholderBackground.Move(coverOrigin)
	r.placeholderBackground.Resize(fyne.NewSquareSize(side))
	iconSide := side * 0.4
	r.placeholderIcon.Move(coverOrigin.AddXY((side-iconSide)/2, (side-iconSide)/2))
	r.placeholderIcon.Resize(fyne.NewSquareSize(iconSide))

	r.cover.Move(coverOrigin)
	r.cover.Resize(fyne.NewSquareSize(side))

	// The title takes one or two lines, and the author follows it. The
	// progress bar and status sit at the bottom, so they line up across a row.
	y := padding + side
	titleHeight := r.titleLineHeight()
	if r.titleWraps(labelWidth) {
		titleHeight = twoLineHeight(titleHeight)
	}
	r.title.Move(fyne.NewPos(labelX, y))
	r.title.Resize(fyne.NewSize(labelWidth, titleHeight))
	y += titleHeight - innerPadding/2

	r.author.Move(fyne.NewPos(labelX, y))
	r.author.Resize(fyne.NewSize(labelWidth, r.author.MinSize().Height))

	rowHeight := r.status.MinSize().Height
	y = size.Height - padding - rowHeight - cardProgressThick
	barWidth := side - innerPadding
	r.track.Move(fyne.NewPos(padding+innerPadding/2, y))
	r.track.Resize(fyne.NewSize(barWidth, cardProgressThick))
	r.fill.Move(fyne.NewPos(padding+innerPadding/2, y))
	r.fill.Resize(fyne.NewSize(barWidth*float32(min(max(r.card.Progress, 0), 1)), cardProgressThick))
	y += cardProgressThick

	moreWidth := float32(0)
	if r.more.Visible() {
		moreWidth = rowHeight
		r.more.Move(fyne.NewPos(size.Width-padding-moreWidth, y))
		r.more.Resize(fyne.NewSquareSize(moreWidth))
	}

	rowWidth := labelWidth - moreWidth
	ratingWidth := float32(0)
	if r.card.Rating != "" {
		ratingWidth = min(r.rating.MinSize().Width, rowWidth/2)
	}
	r.status.Move(fyne.NewPos(labelX, y))
	r.status.Resize(fyne.NewSize(rowWidth-ratingWidth, rowHeight))
	r.rating.Move(fyne.NewPos(labelX+rowWidth-ratingWidth, y))
	r.rating.Resize(fyne.NewSize(ratingWidth, rowHeight))
}

// titleWraps reports whether the title needs a second line at this width.
func (r *bookCardRenderer) titleWraps(labelWidth float32) bool {
	textWidth := fyne.MeasureText(r.card.Title, theme.TextSize(), r.title.TextStyle).Width
	return textWidth > labelWidth-2*theme.InnerPadding()
}

// titleLineHeight is the height of a one-line title, worked out from the font
// because a wrapping label's own MinSize depends on how it was last laid out.
func (r *bookCardRenderer) titleLineHeight() float32 {
	return fyne.MeasureText("M", theme.TextSize(), r.title.TextStyle).Height + 2*theme.InnerPadding()
}

func twoLineHeight(oneLine float32) float32 {
	lineHeight := oneLine - 2*theme.InnerPadding()
	return oneLine + lineHeight + theme.LineSpacing()
}

func (r *bookCardRenderer) MinSize() fyne.Size {
	padding := theme.Padding()
	innerPadding := theme.InnerPadding()
	side := float32(cardWidth) - 2*padding

	height := padding + side +
		twoLineHeight(r.titleLineHeight()) - innerPadding/2 +
		r.author.MinSize().Height - innerPadding/2 +
		cardProgressGap + cardProgressThick +
		r.status.MinSize().Height + padding

	return fyne.NewSize(cardWidth, height)
}

func (r *bookCardRenderer) Refresh() {
	th := r.card.Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()

	r.background.FillColor = color.Transparent
	if r.card.hovered {
		r.background.FillColor = th.Color(theme.ColorNameHover, variant)
	}
	r.background.StrokeColor = color.Transparent
	if r.card.focused {
		r.background.StrokeColor = th.Color(theme.ColorNameFocus, variant)
	}
	r.placeholderBackground.FillColor = th.Color(theme.ColorNameInputBackground, variant)
	r.track.FillColor = th.Color(theme.ColorNameInputBackground, variant)
	r.fill.FillColor = th.Color(theme.ColorNamePrimary, variant)

	r.cover.Image = r.card.Thumbnail
	if r.card.Thumbnail == nil {
		r.cover.Hide()
		r.placeholderBackground.Show()
		r.placeholderIcon.Show()
	} else {
		r.cover.Show()
		r.placeholderBackground.Hide()
		r.placeholderIcon.Hide()
	}

	r.title.SetText(r.card.Title)
	r.author.SetText(r.card.Author)
	r.status.SetText(r.card.Status)
	r.rating.SetText(r.card.Rating)
	if r.card.Rating == "" {
		r.rating.Hide()
	} else {
		r.rating.Show()
	}
	if r.card.Menu == nil {
		r.more.Hide()
	} else {
		r.more.Show()
	}

	r.Layout(r.card.Size())
	for _, object := range []fyne.CanvasObject{r.background, r.placeholderBackground, r.placeholderIcon, r.cover, r.track, r.fill} {
		object.Refresh()
	}
}

func (r *bookCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		r.background,
		r.placeholderBackground,
		r.placeholderIcon,
		r.cover,
		r.title,
		r.author,
		r.track,
		r.fill,
		r.status,
		r.rating,
		r.more,
	}
}

func (r *bookCardRenderer) Destroy() {}

type shelfLayout struct {
	minCell fyne.Size
	cell    fyne.Size
	columns int
}

func newShelfLayout(minCell fyne.Size) *shelfLayout {
	return &shelfLayout{minCell: minCell, cell: minCell, columns: 1}
}

func (l *shelfLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	gap := theme.Padding() * 2
	l.columns = max(int((size.Width+gap)/(l.minCell.Width+gap)), 1)

	width := (size.Width - gap*float32(l.columns-1)) / float32(l.columns)
	l.cell = fyne.NewSize(width, l.minCell.Height+width-l.minCell.Width)

	for i, object := range objects {
		row, column := i/l.columns, i%l.columns
		object.Move(fyne.NewPos(float32(column)*(l.cell.Width+gap), float32(row)*(l.cell.Height+gap)))
		object.Resize(l.cell)
	}
}

func (l *shelfLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	rows := (len(objects) + l.columns - 1) / l.columns
	if rows == 0 {
		return fyne.NewSize(l.minCell.Width, 0)
	}

	gap := theme.Padding() * 2
	return fyne.NewSize(l.minCell.Width, float32(rows)*l.cell.Height+gap*float32(rows-1))
}
