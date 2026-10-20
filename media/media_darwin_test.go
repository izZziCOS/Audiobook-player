package media

import (
    "testing"
    "time"
)

func TestNowPlayingOnMac(t *testing.T) {
    var received []Event
    controls, err := Start(func(event Event) {
        received = append(received, event)
    })
    if err != nil {
        t.Fatal(err)
    }
    defer controls.Close()

    controls.SetStatus(Status{Playing: true, Position: 42 * time.Second, Rate: 1.5})
    controls.SetTrack(Track{Title: "The Hobbit", Artist: "J. R. R. Tolkien", Cover: testCover(t), Duration: time.Hour})

    shown := nowPlaying()
    if shown.title != "The Hobbit" || !shown.playing || shown.elapsed != 42*time.Second || !shown.artwork {
        t.Fatalf("Now Playing shows %+v", shown)
    }

    controls.SetStatus(Status{Playing: false, Position: 50 * time.Second, Rate: 1.5})
    if shown := nowPlaying(); shown.playing || shown.elapsed != 50*time.Second || shown.title != "The Hobbit" {
        t.Fatalf("after pausing, Now Playing shows %+v", shown)
    }

    mediaCommand(2, 0)
    mediaCommand(7, 12.5)
    mediaCommand(99, 0)
    if len(received) != 2 || received[0].Command != TogglePlay || received[1] != (Event{Command: Seek, Position: 12500 * time.Millisecond}) {
        t.Fatalf("received %+v", received)
    }

    controls.Close()
    mediaCommand(0, 0)
    if len(received) != 2 || nowPlaying().title != "" {
        t.Fatal("closed controls should stop showing and stop reacting")
    }
}
