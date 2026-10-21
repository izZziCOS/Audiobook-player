package nowplaying

import (
	"maps"

	"fyne.io/fyne/v2"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

func (p *Player) loadDurations(book *library.Book) {
	if !book.IsMultiPart() {
		return
	}

	durations := make([]float64, len(book.Parts))
	var missing []int

	for index, part := range book.Parts {
		if seconds, found := p.data.CachedDurationOf(part, book.PlayablePart(index)); found {
			durations[index] = seconds
		} else if index == book.Index {
			durations[index] = p.durationSeconds()
			p.data.CacheDurationOf(part, book.PlayablePart(index), durations[index])
		} else {
			missing = append(missing, index)
		}
	}

	if len(missing) == 0 {
		p.applyDurations(book, durations)
		return
	}

	p.async(func() {
		measured := map[string]userdata.PartDuration{}
		for _, index := range missing {
			entry, err := userdata.MeasureDuration(book.PlayablePart(index))
			if err != nil {
				return
			}

			durations[index] = entry.Seconds
			measured[book.Parts[index]] = entry
		}

		fyne.Do(func() {
			maps.Copy(p.data.Durations, measured)
			p.applyDurations(book, durations)
		})
	})
}

func (p *Player) applyDurations(book *library.Book, durations []float64) {
	p.data.MarkDirty()

	if p.book != book {
		return
	}

	book.Durations = durations
	p.resetProgress()
	p.publishTrack()
}
