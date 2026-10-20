package nowplaying

import (
    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "path/filepath"
    "testing"
    "time"

    "github.com/izzzicos/audiobook-player/audio"
    "github.com/izzzicos/audiobook-player/media"
)

type fakeMedia struct {
    tracks   []media.Track
    statuses []media.Status
    closed   bool
}

func (f *fakeMedia) SetTrack(track media.Track) error {
    f.tracks = append(f.tracks, track)
    return nil
}

func (f *fakeMedia) SetStatus(status media.Status) error {
    f.statuses = append(f.statuses, status)
    return nil
}

func (f *fakeMedia) Close() {
    f.closed = true
}

func (f *fakeMedia) status() media.Status {
    return f.statuses[len(f.statuses)-1]
}

func TestMediaControlsFollowThePlayer(t *testing.T) {
    p := newTestPlayer(t)
    controls := &fakeMedia{}
    p.media = controls

    folder, parts := testbooks.Book(t)
    for _, part := range parts {
        seconds, err := audio.Duration(part)
        if err != nil {
            t.Fatal(err)
        }
        p.data.CacheDuration(part, seconds)
    }
    testbooks.WriteCover(t, filepath.Join(folder, "cover.png"))

    p.OpenFolder(folder)

    if len(controls.tracks) == 0 {
        t.Fatal("opening a book should show it in the system's media controls")
    }
    track := controls.tracks[len(controls.tracks)-1]
    if track.Title != "The Hobbit" || len(track.Cover) == 0 || !closeTo(track.Duration.Seconds(), p.totalSeconds()) {
        t.Fatalf("the system was shown %q, %d bytes of cover, %v long", track.Title, len(track.Cover), track.Duration)
    }

    p.handleMediaEvent(media.Event{Command: media.Play})
    p.handleMediaEvent(media.Event{Command: media.Play})
    if !p.playing || !controls.status().Playing || controls.status().Rate != 1 {
        t.Fatal("the play button should start playing, and pressing it again should keep playing")
    }

    p.handleMediaEvent(media.Event{Command: media.Pause})
    if p.playing || controls.status().Playing {
        t.Fatal("the pause button should pause")
    }

    p.handleMediaEvent(media.Event{Command: media.TogglePlay})
    p.handleMediaEvent(media.Event{Command: media.TogglePlay})
    if p.playing {
        t.Fatal("play/pause pressed twice should leave the book paused")
    }

    published := len(controls.statuses)
    p.refreshProgress()
    p.refreshProgress()
    if len(controls.statuses) != published {
        t.Fatal("nothing changed, so the system shouldn't be told again")
    }

    target := p.book.Durations[0] + 0.3
    p.handleMediaEvent(media.Event{Command: media.Seek, Position: secondsDuration(target)})
    if p.book.Index != 1 || !closeTo(p.Elapsed(), target) || !closeTo(controls.status().Position.Seconds(), target) {
        t.Fatalf("seeking from the system's controls went to part %d at %.2f", p.book.Index+1, p.Elapsed())
    }

    p.handleMediaEvent(media.Event{Command: media.Previous})
    if p.book.Index != 0 || p.Elapsed() != 0 {
        t.Fatal("previous should go back 30 seconds")
    }

    p.handleMediaEvent(media.Event{Command: media.SkipForward})
    if p.book.Index != 2 {
        t.Fatal("skipping forward should go ahead 30 seconds")
    }

    p.StopMediaControls()
    if !controls.closed || p.media != nil {
        t.Fatal("the controls should be closed when the app stops")
    }
}

func TestMediaStatusOnlyChangesWhenItMust(t *testing.T) {
    start := time.Now()
    playing := publishedStatus{status: media.Status{Playing: true, Position: time.Minute, Rate: 1.5}, at: start}

    for name, check := range map[string]struct {
        status media.Status
        at     time.Time
        want   bool
    }{
        "keeps playing":   {media.Status{Playing: true, Position: time.Minute + 15*time.Second, Rate: 1.5}, start.Add(10 * time.Second), false},
        "jumped forward":  {media.Status{Playing: true, Position: 2 * time.Minute, Rate: 1.5}, start.Add(10 * time.Second), true},
        "jumped back":     {media.Status{Playing: true, Position: 30 * time.Second, Rate: 1.5}, start.Add(time.Second), true},
        "paused":          {media.Status{Playing: false, Position: time.Minute, Rate: 1.5}, start, true},
        "speed changed":   {media.Status{Playing: true, Position: time.Minute, Rate: 2}, start, true},
    } {
        if got := playing.differs(check.status, check.at); got != check.want {
            t.Errorf("%s: got %v, want %v", name, got, check.want)
        }
    }

    paused := publishedStatus{status: media.Status{Position: time.Minute, Rate: 1}, at: start}
    if paused.differs(media.Status{Position: time.Minute, Rate: 1}, start.Add(time.Hour)) {
        t.Error("a paused book that hasn't moved shouldn't be sent again")
    }
    if !paused.differs(media.Status{Position: time.Minute + time.Second/2, Rate: 1}, start) {
        t.Error("any move while paused should be sent")
    }

    if !(publishedStatus{}).differs(media.Status{}, start) {
        t.Error("the first status should always be sent")
    }
}
