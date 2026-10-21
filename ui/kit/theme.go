package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type palette map[fyne.ThemeColorName]color.Color

const (
	ColorNameChartMark  fyne.ThemeColorName = "chartMark"
	ColorNameChartEmpty fyne.ThemeColorName = "chartEmpty"
)

var ColorNameChartHeat = []fyne.ThemeColorName{"chartHeat1", "chartHeat2", "chartHeat3", "chartHeat4"}

func hex(value uint32) color.NRGBA {
	return color.NRGBA{R: uint8(value >> 24), G: uint8(value >> 16), B: uint8(value >> 8), A: uint8(value)}
}

var (
	LightPalette = palette{
		theme.ColorNameBackground:          hex(0xf7f4f0ff),
		theme.ColorNameForeground:          hex(0x1d1b19ff),
		theme.ColorNamePrimary:             hex(0xd9613aff),
		theme.ColorNameForegroundOnPrimary: hex(0xffffffff),
		theme.ColorNameFocus:               hex(0xd9613a80),
		theme.ColorNameSelection:           hex(0xd9613a33),
		theme.ColorNameButton:              hex(0x5a3a2014),
		theme.ColorNameDisabledButton:      hex(0x5a3a200a),
		theme.ColorNameInputBackground:     hex(0x5a3a2014),
		theme.ColorNameInputBorder:         hex(0x00000000),
		theme.ColorNameHover:               hex(0x1d1b190d),
		theme.ColorNamePressed:             hex(0x1d1b191a),
		theme.ColorNameSeparator:           hex(0xe4ded7ff),
		theme.ColorNamePlaceHolder:         hex(0x8a837bff),
		theme.ColorNameDisabled:            hex(0x857d75ff),
		theme.ColorNameScrollBar:           hex(0x1d1b1940),
		theme.ColorNameShadow:              hex(0x1d1b1926),
		theme.ColorNameMenuBackground:      hex(0xfffdfbff),
		theme.ColorNameOverlayBackground:   hex(0xfffdfbff),
		theme.ColorNameHeaderBackground:    hex(0xefebe6ff),
		ColorNameChartMark:                 hex(0xd9613aff),
		ColorNameChartEmpty:                hex(0xe4ded7ff),
		"chartHeat1":                       hex(0xef9275ff),
		"chartHeat2":                       hex(0xdd6d49ff),
		"chartHeat3":                       hex(0xc24d25ff),
		"chartHeat4":                       hex(0xa0340aff),
	}

	DarkPalette = palette{
		theme.ColorNameBackground:          hex(0x131315ff),
		theme.ColorNameForeground:          hex(0xeeebe7ff),
		theme.ColorNamePrimary:             hex(0xf07c52ff),
		theme.ColorNameForegroundOnPrimary: hex(0x1a0f0bff),
		theme.ColorNameFocus:               hex(0xf07c5280),
		theme.ColorNameSelection:           hex(0xf07c5240),
		theme.ColorNameButton:              hex(0xffffff14),
		theme.ColorNameDisabledButton:      hex(0xffffff0a),
		theme.ColorNameInputBackground:     hex(0xffffff12),
		theme.ColorNameInputBorder:         hex(0x00000000),
		theme.ColorNameHover:               hex(0xffffff12),
		theme.ColorNamePressed:             hex(0xffffff21),
		theme.ColorNameSeparator:           hex(0x2a2a2eff),
		theme.ColorNamePlaceHolder:         hex(0x8e8b93ff),
		theme.ColorNameDisabled:            hex(0x7f7c86ff),
		theme.ColorNameScrollBar:           hex(0xffffff40),
		theme.ColorNameShadow:              hex(0x00000080),
		theme.ColorNameMenuBackground:      hex(0x1d1d20ff),
		theme.ColorNameOverlayBackground:   hex(0x1d1d20ff),
		theme.ColorNameHeaderBackground:    hex(0x1b1b1eff),
		ColorNameChartMark:                 hex(0xd7663cff),
		ColorNameChartEmpty:                hex(0x2a2a2eff),
		"chartHeat1":                       hex(0x833f27ff),
		"chartHeat2":                       hex(0xaf5331ff),
		"chartHeat3":                       hex(0xd96a42ff),
		"chartHeat4":                       hex(0xfb906aff),
	}
)

type Theme struct {
	Forced  bool
	Variant fyne.ThemeVariant
}

func (t Theme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.Forced {
		variant = t.Variant
	}

	colors := LightPalette
	if variant == theme.VariantDark {
		colors = DarkPalette
	}

	if value, found := colors[name]; found {
		return value
	}

	return theme.DefaultTheme().Color(name, variant)
}

func (Theme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (Theme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (Theme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius:
		return 12
	case theme.SizeNameSelectionRadius:
		return 10
	case theme.SizeNameScrollBarRadius:
		return 4
	case theme.SizeNameHeadingText:
		return 26
	case theme.SizeNameSubHeadingText:
		return 20
	case theme.SizeNameSeparatorThickness:
		return 1
	}

	return theme.DefaultTheme().Size(name)
}

func Blend(base, overlay color.Color, amount float64) color.Color {
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
