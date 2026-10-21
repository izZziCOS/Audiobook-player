package settings

import (
    "slices"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/titlebar"
    "github.com/izzzicos/audiobook-player/ui/kit"
)

const preferenceAppearance = "appearance"

const (
    appearanceSystem = "System"
    appearanceLight  = "Light"
    appearanceDark   = "Dark"
)

var appearanceOptions = []string{appearanceSystem, appearanceLight, appearanceDark}

type Settings struct {
    app     fyne.App
    window  fyne.Window
    content fyne.CanvasObject

    appearance string
    buttons    map[string]*widget.Button
}

func SavedTheme(prefs fyne.Preferences) fyne.Theme {
    return themeFor(prefs.StringWithFallback(preferenceAppearance, appearanceSystem))
}

func themeFor(appearance string) fyne.Theme {
    switch appearance {
    case appearanceLight:
        return kit.Theme{Forced: true, Variant: theme.VariantLight}
    case appearanceDark:
        return kit.Theme{Forced: true, Variant: theme.VariantDark}
    default:
        return kit.Theme{}
    }
}

func New(app fyne.App, window fyne.Window, sections ...fyne.CanvasObject) *Settings {
    s := &Settings{
        app:        app,
        window:     window,
        appearance: app.Preferences().StringWithFallback(preferenceAppearance, appearanceSystem),
    }
    s.content = s.build(sections)

    return s
}

func (s *Settings) Content() fyne.CanvasObject {
    return s.content
}

func (s *Settings) build(sections []fyne.CanvasObject) fyne.CanvasObject {
    icons := map[string]fyne.Resource{
        appearanceSystem: theme.ComputerIcon(),
        appearanceLight:  theme.NewThemedResource(kit.LightModeIcon),
        appearanceDark:   theme.NewThemedResource(kit.SleepIcon),
    }

    s.buttons = map[string]*widget.Button{}
    var choices []fyne.CanvasObject
    for _, option := range appearanceOptions {
        button := widget.NewButtonWithIcon(option, icons[option], func() {
            s.setAppearance(option)
        })
        s.buttons[option] = button
        choices = append(choices, button)
    }
    kit.HighlightChoice(s.buttons, s.appearance)

    rows := []fyne.CanvasObject{
        kit.Section(
            "Appearance",
            "Choose a light or dark look, or follow your system's setting.",
            container.NewGridWithColumns(len(choices), choices...),
        ),
    }
    for _, section := range sections {
        rows = append(rows, kit.Gap(), widget.NewSeparator(), kit.Gap(), section)
    }

    return container.NewVScroll(container.New(
        layout.NewCustomPaddedLayout(12, 24, 16, 16),
        container.NewVBox(rows...),
    ))
}

func (s *Settings) setAppearance(appearance string) {
    if !slices.Contains(appearanceOptions, appearance) {
        appearance = appearanceSystem
    }

    s.appearance = appearance
    s.app.Preferences().SetString(preferenceAppearance, appearance)
    s.app.Settings().SetTheme(themeFor(appearance))
    kit.HighlightChoice(s.buttons, appearance)
}

func (s *Settings) usesDarkColors() bool {
    switch s.appearance {
    case appearanceDark:
        return true
    case appearanceLight:
        return false
    default:
        return s.app.Settings().ThemeVariant() == theme.VariantDark
    }
}

func (s *Settings) titleBarLook() titlebar.Look {
    dark := s.usesDarkColors()
    variant := theme.VariantLight
    if dark {
        variant = theme.VariantDark
    }

    colors := themeFor(s.appearance)
    return titlebar.Look{
        Dark:       dark,
        Background: colors.Color(theme.ColorNameBackground, variant),
        Foreground: colors.Color(theme.ColorNameForeground, variant),
    }
}

func (s *Settings) MatchTitleBar() {
    if err := titlebar.Match(s.window, s.titleBarLook()); err != nil {
        fyne.LogError("Couldn't match the title bar to the app's appearance", err)
    }
}
