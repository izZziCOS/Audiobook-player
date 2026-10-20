package nowplaying

import (
    "time"

        "fyne.io/fyne/v2"

        "github.com/izzzicos/audiobook-player/media"
    )

const (
    skipSeconds = int(media.SkipInterval / time.Second)
    mediaDrift  = 2 * time.Second
)

type mediaControls interface {
    SetTrack(media.Track) error
    SetStatus(media.Status) error
    Close()
}

type publishedStatus struct {
    status media.Status
    at     time.Time
}

func (p publishedStatus) differs(status media.Status, now time.Time) bool {
    if p.at.IsZero() || p.status.Playing != status.Playing || p.status.Rate != status.Rate {
        return true
    }

    if !p.status.Playing {
        return status.Position != p.status.Position
    }

    expected := p.status.Position + time.Duration(float64(now.Sub(p.at))*p.status.Rate)
    return (status.Position - expected).Abs() > mediaDrift
}

func secondsDuration(seconds float64) time.Duration {
    return time.Duration(seconds * float64(time.Second))
}

func (p *Player) StartMediaControls() {
    controls, err := media.Start(func(event media.Event) {
        fyne.Do(func() {
            p.handleMediaEvent(event)
        })
    })
    if err != nil {
        fyne.LogError("Media keys and the system's media controls are not available", err)
        return
    }

    p.media = controls
}

func (p *Player) StopMediaControls() {
    if p.media != nil {
        p.media.Close()
        p.media = nil
    }
}

func (p *Player) handleMediaEvent(event media.Event) {
    switch event.Command {
    case media.Play:
        if !p.playing {
            p.TogglePlay()
        }
    case media.Pause:
        if p.playing {
            p.TogglePlay()
        }
    case media.TogglePlay:
        p.TogglePlay()
    case media.Next, media.SkipForward:
        p.Skip(skipSeconds)
    case media.Previous, media.SkipBackward:
        p.Skip(-skipSeconds)
    case media.Seek:
        p.jumpTo(event.Position.Seconds())
    }
}

func (p *Player) publishTrack() {
    if p.media == nil || p.book == nil {
        return
    }

    track := media.Track{
        Title:    p.book.Title(p.metadata),
        Artist:   p.metadata.Artist,
        Duration: secondsDuration(p.totalSeconds()),
    }
    if p.cover.Resource != nil {
        track.Cover = p.cover.Resource.Content()
    }

    if err := p.media.SetTrack(track); err != nil {
        fyne.LogError("Couldn't show the book in the system's media controls", err)
    }

    p.mediaStatus = publishedStatus{}
    p.publishStatus(time.Now())
}

func (p *Player) publishStatus(now time.Time) {
    if p.media == nil || p.audio == nil {
        return
    }

    status := media.Status{
        Playing:  p.playing,
        Position: secondsDuration(p.Elapsed()),
        Rate:     p.speed,
    }
    if !p.mediaStatus.differs(status, now) {
        return
    }

    p.mediaStatus = publishedStatus{status: status, at: now}
    if err := p.media.SetStatus(status); err != nil {
        fyne.LogError("Couldn't update the system's media controls", err)
    }
}
