package nowplaying

import (
    "fmt"
        "math"
        "time"

        "fyne.io/fyne/v2"
        "fyne.io/fyne/v2/theme"
        "fyne.io/fyne/v2/widget"

        "github.com/izzzicos/audiobook-player/ui/kit"
    )

const sleepFade = 10 * time.Second

var sleepOptions = []time.Duration{
    5 * time.Minute,
    10 * time.Minute,
    15 * time.Minute,
    30 * time.Minute,
    45 * time.Minute,
    60 * time.Minute,
    90 * time.Minute,
}

func (p *Player) buildSleepButton() *widget.Button {
    p.sleepButton = widget.NewButtonWithIcon("", theme.NewThemedResource(kit.SleepIcon), p.showSleepMenu)

    return p.sleepButton
}

func (p *Player) sleepMenu() *fyne.Menu {
    var items []*fyne.MenuItem

    if p.sleepRemaining > 0 {
        items = append(items,
            fyne.NewMenuItem("Turn off sleep timer", func() {
                p.setSleepTimer(0)
            }),
            fyne.NewMenuItemSeparator(),
        )
    }

    for _, duration := range sleepOptions {
        items = append(items, fyne.NewMenuItem(
            fmt.Sprintf("Stop after %d minutes", int(duration.Minutes())),
            func() {
                p.setSleepTimer(duration)
            },
        ))
    }

    return fyne.NewMenu("Sleep timer", items...)
}

func (p *Player) showSleepMenu() {
    kit.ShowMenuAbove(p.window, p.sleepButton, p.sleepMenu())
}

func (p *Player) setSleepTimer(duration time.Duration) {
    p.sleepRemaining = duration
    p.sleepLastTick = time.Now()
    p.restoreVolume()
    p.updateSleepButton()
}

func (p *Player) tickSleepTimer(now time.Time) {
    elapsed := now.Sub(p.sleepLastTick)
    p.sleepLastTick = now

    if p.sleepRemaining <= 0 {
        return
    }

    if p.playing {
        p.sleepRemaining -= elapsed
    }

    if p.sleepRemaining <= 0 {
        p.fallAsleep()
        return
    }

    if p.playing && p.sleepRemaining < sleepFade && p.audio != nil {
        p.audio.SetVolume(p.volume.Value * p.sleepRemaining.Seconds() / sleepFade.Seconds())
    }

    p.updateSleepButton()
}

func (p *Player) fallAsleep() {
    p.sleepRemaining = 0

    if p.playing {
        p.TogglePlay()
    }

    p.restoreVolume()
    p.updateSleepButton()
}

func (p *Player) restoreVolume() {
    if p.audio != nil {
        p.audio.SetVolume(p.volume.Value)
    }
}

func (p *Player) updateSleepButton() {
    text := ""
    if p.sleepRemaining > 0 {
        text = kit.FormatTime(math.Ceil(p.sleepRemaining.Seconds()))
    }

    if p.sleepButton.Text == text {
        return
    }

    resized := len(p.sleepButton.Text) != len(text)
    p.sleepButton.SetText(text)
    if resized {
        kit.RefreshContent(p.window)
    }
}
