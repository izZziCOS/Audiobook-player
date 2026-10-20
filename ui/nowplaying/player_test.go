package nowplaying

import (
    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "math"
    "testing"

    "fyne.io/fyne/v2/test"

    "github.com/izzzicos/audiobook-player/audio"
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
