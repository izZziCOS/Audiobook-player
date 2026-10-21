package nowplaying

import (
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/ui/kit"
)

const preferenceRewind = "rewindAfterPause"

func rewindFor(paused time.Duration) float64 {
    switch {
    case paused < 10*time.Second:
        return 0
    case paused < time.Minute:
        return 2
    case paused < 5*time.Minute:
        return 5
    case paused < time.Hour:
        return 10
    case paused < 24*time.Hour:
        return 20
    default:
        return 30
    }
}

func (p *Player) rewindsAfterPause() bool {
    return p.prefs.BoolWithFallback(preferenceRewind, true)
}

func (p *Player) rewindAfterPause() {
    pausedAt := p.pausedAt
    p.pausedAt = time.Time{}
    if pausedAt.IsZero() || p.audio == nil || !p.rewindsAfterPause() {
        return
    }

    position := p.positionSeconds()
    target := max(position-rewindFor(p.clock().Sub(pausedAt)), 0)
    if target >= position {
        return
    }

    _ = p.audio.SeekTo(target)
    p.refreshProgress()
}

func (p *Player) Section() fyne.CanvasObject {
    rewind := widget.NewCheck("Go back a little when I carry on after a pause", func(on bool) {
        p.prefs.SetBool(preferenceRewind, on)
    })
    rewind.SetChecked(p.rewindsAfterPause())

    return kit.Section(
        "Playback",
        "After a pause, playback starts slightly earlier so you can pick up the thread: "+
            "2 seconds after a minute's pause, 5 after five minutes, 10 after an hour, 20 after a few hours and 30 after a day or more. "+
            "It doesn't go back if you moved somewhere yourself.",
        rewind,
    )
}
