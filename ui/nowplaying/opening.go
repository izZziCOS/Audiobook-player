package nowplaying

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/faiface/beep"

	"github.com/izzzicos/audiobook-player/audio"
	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/vfs"
)

const (
	openingText   = "Opening…"
	prefetchAhead = 90.0
)

type openedAudio struct {
	path     string
	streamer beep.StreamSeekCloser
	format   beep.Format
	tags     library.Metadata
	chapters []library.Chapter
	cover    *library.Cover
}

func (o *openedAudio) close() {
	if o != nil && o.streamer != nil {
		o.streamer.Close()
	}
}

func openAudio(path string) (*openedAudio, error) {
	file, err := vfs.Open(path)
	if err != nil {
		return nil, err
	}

	streamer, format, err := audio.Decode(file)
	if err != nil {
		file.Close()
		return nil, err
	}

	if streamer.Len() <= 0 {
		streamer.Close()
		file.Close()
		return nil, errors.New("the selected audio file is empty")
	}

	opened := &openedAudio{
		path:     path,
		streamer: audio.NewBuffered(streamer, format.SampleRate),
		format:   format,
		tags:     library.ReadMetadata(path),
		chapters: library.ReadChapters(path),
	}
	opened.cover, _ = library.FindCover(path, opened.tags.Picture)

	return opened, nil
}

func (p *Player) install(key string, opened *openedAudio) {
	if p.audio != nil {
		_ = p.audio.Close()
	}

	player := audio.NewPlayer(opened.format.SampleRate, opened.streamer)
	player.OnEnd = func() {
		fyne.Do(func() {
			if p.audio == player {
				p.handlePartEnd()
			}
		})
	}
	p.audio = player
	p.forgetSleepPosition()

	p.tags = opened.tags
	p.chapters = opened.chapters
	p.foundCover = opened.cover
	p.applyEdit(key)

	p.setPlaying(false)
	p.audio.SetVolume(p.volume.Value)
	p.changeSpeed(p.speedChoice)
}

func (p *Player) Close() {
	p.generation++
	p.opening = false
	p.forgetPrefetched()

	if p.audio != nil {
		_ = p.audio.Close()
		p.audio = nil
	}
	p.playing = false
}

func (p *Player) Opening() bool {
	return p.opening
}

func (p *Player) load(key, path string, done func(ok bool)) {
	p.generation++
	generation := p.generation

	if prefetched := p.takePrefetched(path); prefetched != nil {
		p.opening = false
		p.install(key, prefetched)
		done(true)
		return
	}

	if p.audio != nil && p.playing {
		p.audio.Pause()
		p.setPlaying(false)
	}
	p.opening = true
	p.setSubtitle(openingText)
	p.showPlayButton(p.playAfterOpen)

	p.async(func() {
		opened, err := openAudio(path)

		fyne.Do(func() {
			if generation != p.generation {
				opened.close()
				return
			}

			p.opening = false
			if err != nil {
				p.playAfterOpen = false
				if p.book != nil {
					p.setSubtitle(p.subtitleText())
				} else {
					p.setSubtitle("")
				}
				dialog.ShowError(err, p.window)
				done(false)
				return
			}

			p.install(key, opened)
			done(true)
		})
	})
}

func (p *Player) takePrefetched(path string) *openedAudio {
	prefetched := p.prefetched
	p.prefetched = nil
	if prefetched != nil && prefetched.path == path {
		return prefetched
	}

	prefetched.close()
	return nil
}

func (p *Player) forgetPrefetched() {
	p.prefetchGeneration++
	p.prefetching = ""
	p.prefetched.close()
	p.prefetched = nil
}

func (p *Player) prefetchNextPart() {
	if p.book == nil || p.audio == nil || !p.playing || p.opening || !p.book.HasPart(p.book.Index+1) {
		return
	}

	remaining := p.durationSeconds() - p.positionSeconds()
	if remaining <= 0 || remaining > prefetchAhead*p.speed {
		return
	}

	next := p.book.PlayablePart(p.book.Index + 1)
	if p.prefetching == next || p.prefetched != nil && p.prefetched.path == next {
		return
	}

	p.forgetPrefetched()
	p.prefetching = next
	generation := p.prefetchGeneration

	p.async(func() {
		opened, err := openAudio(next)

		fyne.Do(func() {
			if generation != p.prefetchGeneration || err != nil {
				opened.close()
				return
			}
			p.prefetching = ""
			p.prefetched = opened
		})
	})
}
