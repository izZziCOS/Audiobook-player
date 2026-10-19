package ui

import (
    "image/color"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/driver/desktop"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"
)

var (
    skipBackIcon = fyne.NewStaticResource(
        "skip-back.svg",
        []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z"/></svg>`),
    )
    skipForwardIcon = fyne.NewStaticResource(
        "skip-forward.svg",
        []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 5V1l5 5-5 5V7c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6h2c0 4.42-3.58 8-8 8s-8-3.58-8-8 3.58-8 8-8z"/></svg>`),
    )
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
        r.background.FillColor = blend(th.Color(theme.ColorNamePrimary, variant), color.White, 0.15)
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

func blend(base, overlay color.Color, amount float64) color.Color {
    br, bg, bb, ba := base.RGBA()
    or, og, ob, _ := overlay.RGBA()
    mix := func(from, to uint32) uint8 {
        return uint8((float64(from)*(1-amount) + float64(to)*amount) / 257)
    }

    return color.NRGBA{
        R: mix(br, or),
        G: mix(bg, og),
        B: mix(bb, ob),
        A: uint8(ba / 257),
    }
}

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
