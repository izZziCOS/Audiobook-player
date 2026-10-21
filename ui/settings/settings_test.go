package settings

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/ui/kit"
)

func newTestSettings(t *testing.T) *Settings {
	t.Helper()

	app := test.NewApp()
	t.Cleanup(app.Quit)

	window := app.NewWindow("Settings")
	s := New(app, window,
		Group{Name: "General", Sections: []fyne.CanvasObject{widget.NewLabel("Playback")}},
		Group{Name: "Library", Sections: []fyne.CanvasObject{widget.NewLabel("Library folder"), widget.NewLabel("Offline downloads")}},
	)
	window.SetContent(s.Content())

	return s
}

func TestAppearance(t *testing.T) {
	s := newTestSettings(t)
	background := func(variant string) any {
		if variant == "dark" {
			return s.app.Settings().Theme().Color(theme.ColorNameBackground, theme.VariantDark)
		}
		return s.app.Settings().Theme().Color(theme.ColorNameBackground, theme.VariantLight)
	}

	s.setAppearance(appearanceDark)
	if background("light") != kit.DarkPalette[theme.ColorNameBackground] {
		t.Fatal("dark mode should be used even when the system is light")
	}
	if s.app.Preferences().String(preferenceAppearance) != appearanceDark {
		t.Fatal("the choice should be saved")
	}
	if s.buttons[appearanceDark].Importance != widget.HighImportance ||
		s.buttons[appearanceLight].Importance != widget.MediumImportance {
		t.Fatal("the chosen option should be highlighted")
	}

	s.setAppearance(appearanceLight)
	if background("dark") != kit.LightPalette[theme.ColorNameBackground] {
		t.Fatal("light mode should be used even when the system is dark")
	}

	s.setAppearance(appearanceSystem)
	if background("dark") != kit.DarkPalette[theme.ColorNameBackground] || background("light") != kit.LightPalette[theme.ColorNameBackground] {
		t.Fatal("system mode should follow the system")
	}

	s.setAppearance("Purple")
	if s.appearance != appearanceSystem {
		t.Fatalf("an unknown choice should fall back to System, got %q", s.appearance)
	}

	if SavedTheme(s.app.Preferences()) != (kit.Theme{}) {
		t.Fatal("the saved choice should be used when the app starts")
	}
}

func TestTitleBarFollowsTheAppearance(t *testing.T) {
	s := newTestSettings(t)
	systemIsDark := s.app.Settings().ThemeVariant() == theme.VariantDark

	for appearance, want := range map[string]bool{
		appearanceDark:   true,
		appearanceLight:  false,
		appearanceSystem: systemIsDark,
	} {
		s.setAppearance(appearance)
		if s.usesDarkColors() != want {
			t.Errorf("%s: dark title bar is %v, want %v", appearance, !want, want)
		}
		s.MatchTitleBar()
	}
}

func TestSettingsAreGrouped(t *testing.T) {
	s := newTestSettings(t)

	if s.group != "General" || !s.groups["General"].Visible() || s.groups["Library"].Visible() {
		t.Fatal("Settings should open on the first group")
	}

	test.Tap(s.groupButtons["Library"])
	if !s.groups["Library"].Visible() || s.groups["General"].Visible() || s.groupButtons["Library"].Importance != widget.HighImportance {
		t.Fatal("choosing a group should show it and highlight its button")
	}

	again := New(s.app, s.window, Group{Name: "General"}, Group{Name: "Library"})
	if again.group != "Library" {
		t.Fatal("the group chosen last should be shown next time")
	}

	missing := New(s.app, s.window, Group{Name: "Only"})
	if missing.group != "Only" {
		t.Fatal("a remembered group that no longer exists should fall back to the first")
	}
}
