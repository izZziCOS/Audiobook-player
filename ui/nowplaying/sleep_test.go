package nowplaying

import (
    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "math"
    "testing"
    "time"

    "fyne.io/fyne/v2/test"

    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func openPlayingBook(t *testing.T) *Player {
    t.Helper()

    p := newTestPlayer(t)
    _, parts := testbooks.Book(t)
    p.OpenFile(parts[0])
    p.TogglePlay()

    if !p.playing {
        t.Fatal("the book should be playing")
    }

    return p
}

func fullVolume(p *Player) float64 {
    return 2 * math.Log2(p.volume.Value)
}

func TestSleepTimerCountsDownWhilePlaying(t *testing.T) {
    p := openPlayingBook(t)
    advance := func(duration time.Duration) {
        p.tickSleepTimer(p.sleepLastTick.Add(duration))
    }

    p.setSleepTimer(15 * time.Minute)
    if p.sleepButton.Text != "15:00" {
        t.Fatalf("button shows %q", p.sleepButton.Text)
    }

    advance(10 * time.Minute)
    if p.sleepButton.Text != "5:00" || !p.playing {
        t.Fatalf("after 10 minutes: %q, playing %v", p.sleepButton.Text, p.playing)
    }

    p.TogglePlay()
    advance(30 * time.Minute)
    if p.sleepButton.Text != "5:00" {
        t.Fatalf("the timer should not run while paused, shows %q", p.sleepButton.Text)
    }
    p.TogglePlay()

    advance(5*time.Minute - 5*time.Second)
    if p.sleepButton.Text != "0:05" || !p.playing {
        t.Fatalf("5 seconds before the end: %q, playing %v", p.sleepButton.Text, p.playing)
    }
    if volume := p.audio.Volume.Volume; volume >= fullVolume(p) {
        t.Fatalf("the volume should be fading, it is %.2f of %.2f", volume, fullVolume(p))
    }

    advance(5 * time.Second)
    if p.playing || p.sleepRemaining != 0 || p.sleepButton.Text != "" {
        t.Fatalf("the timer should have paused playback: playing %v, %q", p.playing, p.sleepButton.Text)
    }
    if p.audio.Volume.Volume != fullVolume(p) {
        t.Fatal("the volume should be restored after the timer stops playback")
    }

    advance(time.Hour)
    if p.playing {
        t.Fatal("an expired timer should do nothing")
    }
}

func TestTurningTheSleepTimerOff(t *testing.T) {
    p := openPlayingBook(t)

    p.setSleepTimer(5 * time.Minute)
    p.tickSleepTimer(p.sleepLastTick.Add(5*time.Minute - 2*time.Second))
    p.setSleepTimer(0)

    if p.sleepButton.Text != "" || p.audio.Volume.Volume != fullVolume(p) {
        t.Fatal("turning the timer off should clear it and restore the volume")
    }

    p.tickSleepTimer(p.sleepLastTick.Add(time.Hour))
    if !p.playing {
        t.Fatal("playback should continue once the timer is off")
    }
}

func TestSleepMenu(t *testing.T) {
    p := openPlayingBook(t)

    if menu := p.sleepMenu(); len(menu.Items) != len(sleepOptions) || menu.Items[0].Label != "Stop after 5 minutes" {
        t.Fatalf("menu without a timer: %d items, first %q", len(menu.Items), menu.Items[0].Label)
    }

    p.sleepMenu().Items[3].Action()
    if p.sleepRemaining != 30*time.Minute {
        t.Fatalf("choosing 30 minutes set %v", p.sleepRemaining)
    }

    menu := p.sleepMenu()
    if menu.Items[0].Label != "Turn off sleep timer" || !menu.Items[1].IsSeparator {
        t.Fatalf("menu with a timer should start with turning it off, got %q", menu.Items[0].Label)
    }

    menu.Items[0].Action()
    if p.sleepRemaining != 0 {
        t.Fatal("turning the timer off from the menu did not work")
    }

    p.showSleepMenu()
}

func TestFinishingABook(t *testing.T) {
    p := openPlayingBook(t)
    key := p.book.Key()

    p.handlePartEnd()
    if p.data.Stats.CompletedCount() != 1 || !p.data.Positions[key].Finished || p.playing {
        t.Fatalf("finishing the book should stop it and count it, got %d finished", p.data.Stats.CompletedCount())
    }

    app := test.NewApp()
    t.Cleanup(app.Quit)
    restarted := userdata.New(p.data.File())
    restarted.Load(app.Preferences())
    if restarted.Stats.CompletedCount() != 1 || !restarted.Positions[key].Finished {
        t.Fatal("a finished book should still be finished after a restart")
    }
}
