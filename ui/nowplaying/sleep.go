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

const (
	sleepFade       = 10 * time.Second
	chapterSlack    = 3.0
	noSleepPosition = -1.0
)

var sleepOptions = []time.Duration{
	5 * time.Minute,
	10 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
	45 * time.Minute,
	60 * time.Minute,
	90 * time.Minute,
}

type sleepMode int

const (
	sleepOff sleepMode = iota
	sleepAfter
	sleepAtChapterEnd
	sleepAtPartEnd
)

type sleepState struct {
	mode      sleepMode
	remaining time.Duration
	lastTick  time.Time

	chapterEnd   float64
	chapterPart  int
	lastPosition float64
}

func (p *Player) buildSleepButton() *widget.Button {
	p.sleepButton = widget.NewButtonWithIcon("", theme.NewThemedResource(kit.SleepIcon), p.showSleepMenu)

	return p.sleepButton
}

func (p *Player) sleepMenu() *fyne.Menu {
	var items []*fyne.MenuItem

	if p.sleep.mode != sleepOff {
		items = append(items,
			fyne.NewMenuItem("Turn off sleep timer", func() {
				p.setSleepTimer(0)
			}),
			fyne.NewMenuItemSeparator(),
		)
	}

	var ends []*fyne.MenuItem
	if p.hasChapters() {
		ends = append(ends, p.sleepEndItem("End of chapter", sleepAtChapterEnd))
	}
	if p.book != nil && p.book.IsMultiPart() {
		ends = append(ends, p.sleepEndItem("End of this part", sleepAtPartEnd))
	}
	if len(ends) > 0 {
		items = append(append(items, ends...), fyne.NewMenuItemSeparator())
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

func (p *Player) sleepEndItem(label string, mode sleepMode) *fyne.MenuItem {
	item := fyne.NewMenuItem(label, func() {
		p.sleepAtEnd(mode)
	})
	item.Checked = p.sleep.mode == mode

	return item
}

func (p *Player) showSleepMenu() {
	kit.ShowMenuAbove(p.window, p.sleepButton, p.sleepMenu())
}

func (p *Player) setSleepTimer(duration time.Duration) {
	p.sleep = sleepState{
		mode:         sleepAfter,
		remaining:    duration,
		lastTick:     time.Now(),
		lastPosition: noSleepPosition,
	}
	if duration <= 0 {
		p.sleep.mode = sleepOff
	}

	p.restoreVolume()
	p.updateSleepButton()
}

func (p *Player) sleepAtEnd(mode sleepMode) {
	p.sleep = sleepState{
		mode:         mode,
		lastTick:     time.Now(),
		lastPosition: noSleepPosition,
	}
	p.aimAtChapterEnd()

	p.restoreVolume()
	p.updateSleepButton()
}

func (p *Player) forgetSleepPosition() {
	p.sleep.lastPosition = noSleepPosition
}

func (p *Player) nextChapterStart(position float64) (float64, bool) {
	if !p.hasChapters() {
		return 0, false
	}

	for _, chapter := range p.chapters {
		if chapter.Start > position {
			return chapter.Start, true
		}
	}

	return 0, false
}

func (p *Player) aimAtChapterEnd() {
	p.sleep.chapterEnd = noSleepPosition
	if p.book == nil || p.audio == nil {
		return
	}

	p.sleep.chapterPart = p.book.Index
	if end, found := p.nextChapterStart(p.positionSeconds() + chapterTolerance); found {
		p.sleep.chapterEnd = end
	}
}

func (p *Player) passedChapterEnd() (float64, bool) {
	if p.book == nil || p.audio == nil {
		return 0, false
	}

	position := p.positionSeconds()
	end, previous := p.sleep.chapterEnd, p.sleep.lastPosition
	p.sleep.lastPosition = position

	if previous == noSleepPosition || p.sleep.chapterPart != p.book.Index {
		p.aimAtChapterEnd()
		return 0, false
	}
	if end == noSleepPosition || position < end {
		return 0, false
	}
	if previous < end && position-end < chapterSlack {
		return end, true
	}

	p.aimAtChapterEnd()
	return 0, false
}

func (p *Player) sleepLeft() time.Duration {
	switch p.sleep.mode {
	case sleepAfter:
		return p.sleep.remaining
	case sleepAtChapterEnd, sleepAtPartEnd:
		if p.audio == nil {
			return 0
		}

		end := p.durationSeconds()
		if p.sleep.mode == sleepAtChapterEnd && p.sleep.chapterEnd != noSleepPosition {
			end = p.sleep.chapterEnd
		}

		seconds := max(end-p.positionSeconds(), 0) / p.speed
		return time.Duration(seconds * float64(time.Second))
	default:
		return 0
	}
}

func (p *Player) tickSleepTimer(now time.Time) {
	elapsed := now.Sub(p.sleep.lastTick)
	p.sleep.lastTick = now

	switch p.sleep.mode {
	case sleepOff:
		return
	case sleepAfter:
		if p.playing {
			p.sleep.remaining -= elapsed
		}
		if p.sleep.remaining <= 0 {
			p.fallAsleep()
			return
		}
	case sleepAtChapterEnd:
		if end, passed := p.passedChapterEnd(); passed && p.playing {
			p.fallAsleep()
			p.seekInPart(end)
			return
		}
	}

	if left := p.sleepLeft(); p.playing && left < sleepFade && p.audio != nil {
		p.audio.SetVolume(p.volume.Value * left.Seconds() / sleepFade.Seconds())
	}

	p.updateSleepButton()
}

func (p *Player) sleepsAtPartEnd() bool {
	return p.sleep.mode == sleepAtChapterEnd || p.sleep.mode == sleepAtPartEnd
}

func (p *Player) fallAsleep() {
	p.sleep = sleepState{lastPosition: noSleepPosition}

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
	if p.sleep.mode != sleepOff {
		text = kit.FormatTime(math.Ceil(p.sleepLeft().Seconds()))
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
