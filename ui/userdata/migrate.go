package userdata

import (
    "encoding/json"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/storage"

    "github.com/izzzicos/audiobook-player/stats"
)

const (
    oldPositions    = "bookPositions"
    oldDurations    = "partDurations"
    oldLibrary      = "library"
    oldIgnored      = "libraryIgnored"
    oldStats        = "listeningStats"
    oldBookFolder   = "bookFolder"
    oldLastFile     = "lastFile"
    oldLastPosition = "lastPosition"
)

var migratedPreferences = []string{
    oldPositions,
    oldDurations,
    oldLibrary,
    oldIgnored,
    oldStats,
    oldBookFolder,
    oldLastFile,
    oldLastPosition,
}

func migratePreferences(prefs fyne.Preferences) saved {
    data := saved{
        Version:   version,
        Positions: map[string]Position{},
        Durations: map[string]PartDuration{},
        Stats:     stats.New(),
    }

    _ = json.Unmarshal([]byte(prefs.String(oldPositions)), &data.Positions)
    _ = json.Unmarshal([]byte(prefs.String(oldDurations)), &data.Durations)
    _ = json.Unmarshal([]byte(prefs.String(oldStats)), data.Stats)

    for _, path := range prefs.StringList(oldLibrary) {
        data.Library = append(data.Library, SavedBook{Path: path})
    }
    data.Ignored = prefs.StringList(oldIgnored)

    if uri, err := storage.ParseURI(prefs.String(oldLastFile)); err == nil && prefs.String(oldLastFile) != "" {
        data.LastBook = LastBook{Folder: prefs.String(oldBookFolder), File: uri.Path()}

        key := data.LastBook.Folder
        if key == "" {
            key = data.LastBook.File
        }
        if _, found := data.Positions[key]; !found && prefs.Float(oldLastPosition) > 0 {
            data.Positions[key] = Position{Part: data.LastBook.File, Position: prefs.Float(oldLastPosition)}
        }
    }

    return data
}

func removeMigratedPreferences(prefs fyne.Preferences) {
    for _, key := range migratedPreferences {
        prefs.RemoveValue(key)
    }
}
