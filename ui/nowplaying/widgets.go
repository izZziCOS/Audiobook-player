package nowplaying

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/ui/kit"
)

type roundButton struct {
	widget.BaseWidget

	icon     fyne.Resource
	label    string
	filled   bool
	diameter float32
	onTapped func()
	hovered  bool
}

func newRoundButton(
	icon fyne.Resource,
	label string,
	filled bool,
	diameter float32,
	onTapped func(),
) *roundButton {
	button := &roundButton{
		icon:     icon,
		label:    label,
		filled:   filled,
		diameter: diameter,
		onTapped: onTapped,
	}
	button.ExtendBaseWidget(button)

	return button
}

func (b *roundButton) SetIcon(icon fyne.Resource) {
	b.icon = icon
	b.Refresh()
}

func (b *roundButton) Tapped(*fyne.PointEvent) {
	if b.onTapped != nil {
		b.onTapped()
	}
}

func (b *roundButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (b *roundButton) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}

func (b *roundButton) MouseMoved(*desktop.MouseEvent) {}

func (b *roundButton) MouseOut() {
	b.hovered = false
	b.Refresh()
}

func (b *roundButton) CreateRenderer() fyne.WidgetRenderer {
	text := canvas.NewText(b.label, color.Transparent)
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Bold: true}

	renderer := &roundButtonRenderer{
		button:     b,
		background: canvas.NewCircle(color.Transparent),
		image:      canvas.NewImageFromResource(nil),
		text:       text,
	}
	renderer.image.FillMode = canvas.ImageFillContain
	renderer.Refresh()

	return renderer
}

type roundButtonRenderer struct {
	button     *roundButton
	background *canvas.Circle
	image      *canvas.Image
	text       *canvas.Text
}

func (r *roundButtonRenderer) Layout(size fyne.Size) {
	side := min(size.Width, size.Height)
	origin := fyne.NewPos((size.Width-side)/2, (size.Height-side)/2)

	r.background.Resize(fyne.NewSquareSize(side))
	r.background.Move(origin)

	iconSide := side * 0.85
	if r.button.filled {
		iconSide = side * 0.5
	}
	iconOrigin := origin.AddXY((side-iconSide)/2, (side-iconSide)/2)
	r.image.Resize(fyne.NewSquareSize(iconSide))
	r.image.Move(iconOrigin)

	r.text.TextSize = iconSide * 0.27
	textSize := r.text.MinSize()
	r.text.Resize(textSize)
	r.text.Move(iconOrigin.AddXY(
		(iconSide-textSize.Width)/2,
		iconSide*13/24-textSize.Height/2,
	))
}

func (r *roundButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSquareSize(r.button.diameter)
}

func (r *roundButtonRenderer) Refresh() {
	th := r.button.Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()

	switch {
	case r.button.filled && r.button.hovered:
		r.background.FillColor = kit.Blend(th.Color(theme.ColorNamePrimary, variant), color.White, 0.15)
	case r.button.filled:
		r.background.FillColor = th.Color(theme.ColorNamePrimary, variant)
	case r.button.hovered:
		r.background.FillColor = th.Color(theme.ColorNameHover, variant)
	default:
		r.background.FillColor = color.Transparent
	}

	if r.button.filled {
		r.image.Resource = theme.NewColoredResource(r.button.icon, theme.ColorNameForegroundOnPrimary)
	} else {
		r.image.Resource = theme.NewThemedResource(r.button.icon)
	}

	r.text.Text = r.button.label
	r.text.Color = th.Color(theme.ColorNameForeground, variant)

	r.Layout(r.button.Size())
	r.background.Refresh()
	r.image.Refresh()
	r.text.Refresh()
}

func (r *roundButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.image, r.text}
}

func (r *roundButtonRenderer) Destroy() {}

type squareLayout struct{}

func (squareLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	side := min(size.Width, size.Height)
	origin := fyne.NewPos((size.Width-side)/2, (size.Height-side)/2)

	for _, object := range objects {
		object.Resize(fyne.NewSquareSize(side))
		object.Move(origin)
	}
}

func (squareLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	side := float32(0)
	for _, object := range objects {
		minSize := object.MinSize()
		side = max(side, minSize.Width, minSize.Height)
	}

	return fyne.NewSquareSize(side)
}

func newCaptionStyle(alignment fyne.TextAlign) widget.RichTextStyle {
	return widget.RichTextStyle{
		Alignment: alignment,
		ColorName: theme.ColorNamePlaceHolder,
		SizeName:  theme.SizeNameCaptionText,
		TextStyle: fyne.TextStyle{Monospace: true},
	}
}

func newRichText(text string, style widget.RichTextStyle) *widget.RichText {
	richText := widget.NewRichText(&widget.TextSegment{
		Text:  text,
		Style: style,
	})
	richText.Truncation = fyne.TextTruncateEllipsis

	return richText
}

func setRichText(richText *widget.RichText, text string) {
	segment := richText.Segments[0].(*widget.TextSegment)
	if segment.Text == text {
		return
	}

	segment.Text = text
	richText.Refresh()
}
