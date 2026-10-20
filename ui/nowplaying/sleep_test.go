package nowplaying

import (
    "math"
    "slices"
    "testing"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/test"

    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
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
        p.tickSleepTimer(p.sleep.lastTick.Add(duration))
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
    if p.playing || p.sleep.remaining != 0 || p.sleepButton.Text != "" {
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
    p.tickSleepTimer(p.sleep.lastTick.Add(5*time.Minute - 2*time.Second))
    p.setSleepTimer(0)

    if p.sleepButton.Text != "" || p.audio.Volume.Volume != fullVolume(p) {
        t.Fatal("turning the timer off should clear it and restore the volume")
    }

    p.tickSleepTimer(p.sleep.lastTick.Add(time.Hour))
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
    if p.sleep.remaining != 30*time.Minute {
        t.Fatalf("choosing 30 minutes set %v", p.sleep.remaining)
    }

    menu := p.sleepMenu()
    if menu.Items[0].Label != "Turn off sleep timer" || !menu.Items[1].IsSeparator {
        t.Fatalf("menu with a timer should start with turning it off, got %q", menu.Items[0].Label)
    }

    menu.Items[0].Action()
    if p.sleep.remaining != 0 {
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

func tick(p *Player) {
    p.tickSleepTimer(p.sleep.lastTick.Add(250 * time.Millisecond))
}

func playChapterBookAt(t *testing.T, seconds float64) *Player {
    t.Helper()

    p := openChapterBook(t)
    p.TogglePlay()
    if err := p.audio.SeekTo(seconds); err != nil {
        t.Fatal(err)
    }

    return p
}

func menuLabels(menu *fyne.Menu) []string {
    var labels []string
    for _, item := range menu.Items {
        labels = append(labels, item.Label)
    }

    return labels
}

func TestSleepAtTheEndOfAChapter(t *testing.T) {
    p := playChapterBookAt(t, 0.1)

    p.sleepAtEnd(sleepAtChapterEnd)
    tick(p)
    if p.sleep.chapterEnd != 0.25 || p.sleepButton.Text != "0:01" {
        t.Fatalf("the timer should aim at the end of the first chapter, got %v and %q", p.sleep.chapterEnd, p.sleepButton.Text)
    }
    if volume := p.audio.Volume.Volume; volume >= fullVolume(p) {
        t.Fatal("the last seconds of the chapter should fade out")
    }

    if err := p.audio.SeekTo(0.24); err != nil {
        t.Fatal(err)
    }
    tick(p)
    if !p.playing || p.sleep.chapterEnd != 0.25 {
        t.Fatal("a moment before the end of the chapter, it should still be aiming at that end")
    }

    if err := p.audio.SeekTo(0.3); err != nil {
        t.Fatal(err)
    }
    tick(p)

    if p.playing || p.sleep.mode != sleepOff || p.sleepButton.Text != "" {
        t.Fatal("playback should stop once the chapter has ended")
    }
    if !closeTo(p.positionSeconds(), 0.25) || p.subtitle.String() != "The Middle · 2 of 3" {
        t.Fatalf("listening should carry on from the start of the next chapter, at %.3fs", p.positionSeconds())
    }
    if p.audio.Volume.Volume != fullVolume(p) {
        t.Fatal("the volume should be restored")
    }
}

func TestSkippingPastAChapterEndKeepsPlaying(t *testing.T) {
    p := playChapterBookAt(t, 0.1)
    p.sleepAtEnd(sleepAtChapterEnd)
    tick(p)

    p.next()
    tick(p)
    if !p.playing || p.sleep.mode != sleepAtChapterEnd || p.sleep.chapterEnd != 0.5 {
        t.Fatalf("jumping to the next chapter should move the stop to its end, got %v", p.sleep.chapterEnd)
    }

    p.Skip(-skipSeconds)
    tick(p)
    if !p.playing || p.sleep.chapterEnd != 0.25 {
        t.Fatal("skipping back should aim at the end of the chapter skipped into")
    }

    if err := p.audio.SeekTo(0.26); err != nil {
        t.Fatal(err)
    }
    tick(p)
    if p.playing || !closeTo(p.positionSeconds(), 0.25) {
        t.Fatal("playback should stop at the end of that chapter")
    }
}

func TestChapterSleepInTheLastChapterStopsAtTheEnd(t *testing.T) {
    p := playChapterBookAt(t, 0.6)
    p.sleepAtEnd(sleepAtChapterEnd)
    tick(p)

    if p.sleep.chapterEnd != noSleepPosition || p.sleepLeft() <= 0 {
        t.Fatal("in the last chapter the timer should count down to the end of the file")
    }

    p.handlePartEnd()
    if p.playing || p.sleep.mode != sleepOff || !p.data.Positions[p.book.Key()].Finished {
        t.Fatal("the book should finish and the timer turn off")
    }
}

func TestSleepAtTheEndOfThePart(t *testing.T) {
    p := newTestPlayer(t)
    folder, _ := testbooks.Book(t)
    p.OpenFolder(folder)
    p.TogglePlay()

    p.sleepAtEnd(sleepAtPartEnd)
    if p.sleepButton.Text == "" {
        t.Fatal("the button should count down to the end of the part")
    }

    p.handlePartEnd()
    if p.playing || p.book.Index != 1 || p.positionSeconds() != 0 || p.sleep.mode != sleepOff {
        t.Fatalf("playback should stop with the next part ready, got part %d, playing %v", p.book.Index+1, p.playing)
    }
    if p.data.Positions[folder].Part != p.book.Parts[1] {
        t.Fatal("the next part should be remembered, so listening resumes there")
    }

    p.TogglePlay()
    p.handlePartEnd()
    if !p.playing || p.book.Index != 2 {
        t.Fatal("without a sleep timer the next part should play straight away")
    }
}

func TestSleepMenuOffersTheEnds(t *testing.T) {
    chapters := menuLabels(openChapterBook(t).sleepMenu())
    if !slices.Contains(chapters, "End of chapter") || slices.Contains(chapters, "End of this part") {
        t.Fatalf("a single-file book with chapters offers %v", chapters)
    }

    p := newTestPlayer(t)
    folder, _ := testbooks.Book(t)
    p.OpenFolder(folder)
    parts := menuLabels(p.sleepMenu())
    if slices.Contains(parts, "End of chapter") || !slices.Contains(parts, "End of this part") {
        t.Fatalf("a book of several parts offers %v", parts)
    }

    menu := p.sleepMenu()
    menu.Items[0].Action()
    menu = p.sleepMenu()
    if menu.Items[0].Label != "Turn off sleep timer" || !menu.Items[2].Checked || menu.Items[2].Label != "End of this part" {
        t.Fatalf("the chosen end should be ticked, got %v", menuLabels(menu))
    }

    p.setSleepTimer(10 * time.Minute)
    if p.sleepMenu().Items[2].Checked {
        t.Fatal("choosing a time should replace the end of the part")
    }
}
