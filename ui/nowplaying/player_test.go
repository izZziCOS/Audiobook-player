package nowplaying

import (
    "math"
    "testing"
    "time"

    "fyne.io/fyne/v2/test"

    "github.com/izzzicos/audiobook-player/audio"
    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func newTestPlayer(t *testing.T) *Player {
    t.Helper()

    app := test.NewApp()
    t.Cleanup(app.Quit)

    window := app.NewWindow("Audiobook Player")
    p := New(window, app.Preferences(), userdata.New(userdata.FileIn(t.TempDir())), &kit.Picker{}, func(work func()) {
        work()
    })
    window.SetContent(p.Content())

    return p
}

func closeTo(a, b float64) bool {
    return math.Abs(a-b) < 0.01
}

func TestWholeBookProgress(t *testing.T) {
    p := newTestPlayer(t)
    folder, parts := testbooks.Book(t)

    var durations []float64
    for _, part := range parts {
        seconds, err := audio.Duration(part)
        if err != nil {
            t.Fatal(err)
        }

        durations = append(durations, seconds)
        p.data.CacheDuration(part, seconds)
    }
    total := durations[0] + durations[1] + durations[2]

    p.OpenFolder(folder)

    if p.book == nil || !p.book.HasDurations() {
        t.Fatal("expected the book's durations to be loaded from the cache")
    }
    if !closeTo(p.progress.Max, total) {
        t.Fatalf("slider max %v, want the whole book %v", p.progress.Max, total)
    }

    if err := p.Seek(durations[0] + 0.25); err != nil {
        t.Fatal(err)
    }
    if p.book.Index != 1 || !closeTo(p.positionSeconds(), 0.25) {
        t.Fatalf("after seeking into part 2: part %d at %v", p.book.Index+1, p.positionSeconds())
    }
    if !closeTo(p.Elapsed(), durations[0]+0.25) {
        t.Fatalf("elapsed %v, want %v", p.Elapsed(), durations[0]+0.25)
    }

    p.Skip(-30)
    if p.book.Index != 0 || p.positionSeconds() != 0 {
        t.Fatalf("skipping back past the start: part %d at %v", p.book.Index+1, p.positionSeconds())
    }

    p.Skip(30)
    if p.book.Index != 2 {
        t.Fatalf("skipping forward past the end should stay in the last part, got part %d", p.book.Index+1)
    }
    if !closeTo(p.progress.Max, total) {
        t.Fatalf("slider max changed to %v after changing parts", p.progress.Max)
    }
}

func TestSinglePartProgressUsesTheFile(t *testing.T) {
    p := newTestPlayer(t)
    _, parts := testbooks.Book(t)

    p.OpenFile(parts[1])

    if !closeTo(p.progress.Max, p.durationSeconds()) || p.durationSeconds() <= 0 {
        t.Fatalf("slider max %v, want the file's length %v", p.progress.Max, p.durationSeconds())
    }
    if got := p.title.String(); got != "Part 2" {
        t.Errorf("title %q, want the file name", got)
    }
}

func TestRestartingTheOpenBook(t *testing.T) {
    p := newTestPlayer(t)
    folder, parts := testbooks.Book(t)
    p.OpenFolder(folder)
    p.MoveTo(parts[1], 0.25)

    p.Restart(folder)

    if p.book.Index != 0 || p.positionSeconds() != 0 || p.data.Positions[folder].Part != parts[0] {
        t.Fatalf("the open book should go back to the start, got part %d at %v", p.book.Index+1, p.positionSeconds())
    }
}

func TestEachBookKeepsItsOwnSpeed(t *testing.T) {
    p := newTestPlayer(t)
    first, _ := testbooks.Book(t)
    second, _ := testbooks.Book(t)

    p.OpenFolder(first)
    p.chooseSpeed("2.0x")
    p.OpenFolder(second)
    if p.speedChoice != "2.0x" {
        t.Fatalf("a book without its own speed should use the last speed chosen, got %s", p.speedChoice)
    }
    p.chooseSpeed("1.25x")

    p.OpenFolder(first)
    if p.speedChoice != "2.0x" || p.speed != 2.0 {
        t.Fatalf("the first book should play at its own speed, got %s", p.speedChoice)
    }
    p.OpenFolder(second)
    if p.speedChoice != "1.25x" || p.SpeedFor(first) != 2.0 {
        t.Fatalf("got %s and %v", p.speedChoice, p.SpeedFor(first))
    }
}

func TestRewindTable(t *testing.T) {
    for paused, want := range map[time.Duration]float64{
        5 * time.Second:  0,
        30 * time.Second: 2,
        3 * time.Minute:  5,
        20 * time.Minute: 10,
        6 * time.Hour:    20,
        72 * time.Hour:   30,
    } {
        if got := rewindFor(paused); got != want {
            t.Errorf("after %v got %v seconds, want %v", paused, got, want)
        }
    }
}

func TestGoingBackAfterAPause(t *testing.T) {
    p := newTestPlayer(t)
    _, parts := testbooks.Book(t)
    now := time.Now()
    p.clock = func() time.Time { return now }

    p.OpenFile(parts[0])
    middle := p.durationSeconds() / 2
    resumeAfter := func(pause time.Duration) float64 {
        p.audio.SeekTo(middle)
        p.TogglePlay()
        p.TogglePlay()
        now = now.Add(pause)
        p.TogglePlay()
        position := p.positionSeconds()
        p.TogglePlay()
        return position
    }

    if position := resumeAfter(5 * time.Second); !closeTo(position, middle) {
        t.Fatalf("a short pause shouldn't go back, at %v", position)
    }
    if position := resumeAfter(2 * time.Minute); position != 0 {
        t.Fatalf("after two minutes it should go back 5 seconds, to the start here, at %v", position)
    }

    p.audio.SeekTo(middle)
    p.TogglePlay()
    p.TogglePlay()
    now = now.Add(time.Hour)
    p.seekInPart(middle / 2)
    p.TogglePlay()
    if !closeTo(p.positionSeconds(), middle/2) {
        t.Fatal("after moving yourself, it shouldn't go back")
    }
    p.TogglePlay()

    p.prefs.SetBool(preferenceRewind, false)
    if position := resumeAfter(time.Hour); !closeTo(position, middle) {
        t.Fatal("with the setting off it shouldn't go back")
    }
}

func TestReopeningABookLaterGoesBack(t *testing.T) {
    p := newTestPlayer(t)
    folder, parts := testbooks.Book(t)
    p.data.Positions[folder] = userdata.Position{Part: parts[0], Position: 0.3, Listened: time.Now().Add(-48 * time.Hour).Unix()}

    p.OpenFolder(folder)
    if !closeTo(p.positionSeconds(), 0.3) {
        t.Fatalf("setup: the book should open where it was left, at %v", p.positionSeconds())
    }
    p.TogglePlay()
    if p.positionSeconds() != 0 {
        t.Fatalf("two days later it should go back 30 seconds, to the start here, at %v", p.positionSeconds())
    }
}
