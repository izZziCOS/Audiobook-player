package nowplaying

import (
    "time"
    "fmt"

        "fyne.io/fyne/v2/dialog"

        "github.com/izzzicos/audiobook-player/library"
    )

const (
        restartChapterAfter = 3.0
        chapterTolerance    = 0.05
    )

func (p *Player) hasChapters() bool {
    return len(p.chapters) > 1
}

func chapterAt(chapters []library.Chapter, position float64) int {
    index := 0
    for i, chapter := range chapters {
        if chapter.Start <= position+chapterTolerance {
            index = i
        }
    }

    return index
}

func previousChapterStart(chapters []library.Chapter, position float64, hasPreviousPart bool) (float64, bool) {
    index := chapterAt(chapters, position)

    switch {
    case position-chapters[index].Start > restartChapterAfter:
        return chapters[index].Start, true
    case index > 0:
        return chapters[index-1].Start, true
    case !hasPreviousPart:
        return chapters[0].Start, true
    default:
        return 0, false
    }
}

func (p *Player) currentChapter() int {
    return chapterAt(p.chapters, p.positionSeconds())
}

func (p *Player) subtitleText() string {
    if !p.hasChapters() {
        return p.book.Subtitle(p.metadata)
    }

    index := p.currentChapter()
    text := fmt.Sprintf("%s · %d of %d", p.chapters[index].Title, index+1, len(p.chapters))
    if p.book.IsMultiPart() {
        text = fmt.Sprintf("Part %d of %d · %s", p.book.Index+1, len(p.book.Parts), text)
    }

    return text
}

func (p *Player) updateChapter() {
    if p.book == nil || !p.hasChapters() {
        return
    }

    if index := p.currentChapter(); index != p.chapterIndex {
        p.chapterIndex = index
        p.setSubtitle(p.subtitleText())
    }
}

func (p *Player) previous() {
    if p.audio == nil || p.book == nil {
        return
    }

    if p.hasChapters() {
        if start, ok := previousChapterStart(p.chapters, p.positionSeconds(), p.book.HasPart(p.book.Index-1)); ok {
            p.seekInPart(start)
            return
        }
    }

    p.goToRelativePart(-1)
}

func (p *Player) next() {
    if p.audio == nil || p.book == nil {
        return
    }

    if p.hasChapters() {
        if index := p.currentChapter(); index+1 < len(p.chapters) {
            p.seekInPart(p.chapters[index+1].Start)
            return
        }
    }

    p.goToRelativePart(1)
}

func (p *Player) Restart(key string) {
    if p.book == nil || p.audio == nil || p.book.Key() != key {
        return
    }

    if p.playing {
        p.TogglePlay()
    }
    p.MoveTo(p.book.Parts[0], 0)
}

func (p *Player) MoveTo(part string, seconds float64) bool {
    if p.book == nil || p.audio == nil {
        return false
    }

    index, found := p.book.Find(part)
    if !found {
        return false
    }

    if index != p.book.Index {
        p.goToPart(index, p.playing, func() {
            p.seekInPart(seconds)
        })
        return true
    }
    p.seekInPart(seconds)

    return true
}

func (p *Player) seekInPart(seconds float64) {
    p.forgetSleepPosition()
    p.pausedAt = time.Time{}

    if err := p.audio.SeekTo(seconds); err != nil {
        dialog.ShowError(err, p.window)
        return
    }

    p.refreshProgress()
    p.SavePosition()
}
