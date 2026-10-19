package ui

import (
    "os"

    "fyne.io/fyne/v2/storage"
)

const (
    preferenceVolume       = "volume"
    preferenceSpeed        = "speed"
    preferenceBookFolder   = "bookFolder"
    preferenceLastFile     = "lastFile"
    preferenceLastPosition = "lastPosition"
    preferenceWasPlaying   = "wasPlaying"
)

func (ui *UI) savePreferences() {
    prefs := ui.App.Preferences()
    prefs.SetFloat(preferenceVolume, ui.Volume.Value)
    prefs.SetString(preferenceSpeed, ui.Speed.Selected)

    if ui.Book == nil || ui.Player == nil {
        return
    }

    position := float64(ui.Player.Position()) /
        float64(ui.Player.SampleRate)

    prefs.SetString(preferenceBookFolder, ui.Book.Folder)
    prefs.SetString(preferenceLastFile, storage.NewFileURI(ui.Book.Current()).String())
    prefs.SetFloat(preferenceLastPosition, position)
    prefs.SetBool(preferenceWasPlaying, ui.Playing)
}

func (ui *UI) loadPreferences() {
    prefs := ui.App.Preferences()

    ui.Volume.SetValue(prefs.FloatWithFallback(preferenceVolume, 0.7))
    ui.Speed.SetSelected(prefs.StringWithFallback(preferenceSpeed, "1.0x"))

    lastFile := prefs.String(preferenceLastFile)
    if lastFile == "" {
        return
    }

    uri, err := storage.ParseURI(lastFile)
    if err != nil {
        return
    }

    if _, err := os.Stat(uri.Path()); err != nil {
        return
    }

    if !ui.resumeBook(prefs.String(preferenceBookFolder), uri.Path()) {
        return
    }

    _ = ui.Player.SeekTo(prefs.Float(preferenceLastPosition))
    ui.refreshProgress()
}
