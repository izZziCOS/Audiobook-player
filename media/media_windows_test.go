package media

import (
	"strings"
	"testing"
	"time"
	"unsafe"
)

func shownOnWindows(t *testing.T, c *Controls) (title, artist string, status int32) {
	t.Helper()

	err := c.do(func() error {
		if err := c.controls.call(controlsPlaybackStatus, uintptr(unsafe.Pointer(&status))); err != nil {
			return err
		}

		var updater *object
		if err := c.controls.call(controlsDisplayUpdater, uintptr(unsafe.Pointer(&updater))); err != nil {
			return err
		}
		defer updater.release()

		var music *object
		if err := updater.call(updaterMusicProperties, uintptr(unsafe.Pointer(&music))); err != nil {
			return err
		}
		defer music.release()

		var err error
		if title, err = getString(music, musicTitle); err != nil {
			return err
		}
		artist, err = getString(music, musicArtist)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	return title, artist, status
}

func startOrSkip(t *testing.T, handle func(Event)) *Controls {
	t.Helper()

	controls, err := Start(handle)
	if err != nil && strings.Contains(err.Error(), "is not available") {
		t.Skipf("this Windows has no media controls: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}

	return controls
}

func TestMediaControlsOnWindows(t *testing.T) {
	controls := startOrSkip(t, func(Event) {})

	if err := controls.SetTrack(Track{Title: "The Hobbit", Artist: "J. R. R. Tolkien", Cover: testCover(t), Duration: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if err := controls.SetStatus(Status{Playing: true, Position: time.Minute, Rate: 1}); err != nil {
		t.Fatal(err)
	}

	title, artist, status := shownOnWindows(t, controls)
	if title != "The Hobbit" || artist != "J. R. R. Tolkien" || status != statusPlaying {
		t.Fatalf("Windows shows %q by %q with status %d", title, artist, status)
	}

	if err := controls.SetTrack(Track{Title: "Untitled\x00", Cover: []byte("not a picture")}); err != nil {
		t.Logf("a broken cover is reported: %v", err)
	}
	if err := controls.SetStatus(Status{}); err != nil {
		t.Fatal(err)
	}
	if title, artist, status := shownOnWindows(t, controls); title != "Untitled" || artist != "" || status != statusPaused {
		t.Fatalf("Windows shows %q by %q with status %d", title, artist, status)
	}

	controls.Close()
	controls.Close()
	if err := controls.SetStatus(Status{Playing: true}); err != errClosed {
		t.Fatalf("closed controls returned %v", err)
	}

	startOrSkip(t, func(Event) {}).Close()
}

func TestButtonPressesReachTheApp(t *testing.T) {
	var received []Event
	handler := &buttonHandler{vtable: handlerVtable, handle: func(event Event) {
		received = append(received, event)
	}}

	var result *buttonHandler
	if code := callHandler(handler.vtable.queryInterface, unsafe.Pointer(handler), unsafe.Pointer(&iidButtonPressedHandler), unsafe.Pointer(&result)); code != 0 || result != handler {
		t.Fatalf("asking for the handler's interface gave %#x", code)
	}
	if code := callHandler(handler.vtable.queryInterface, unsafe.Pointer(handler), unsafe.Pointer(&iidTransportControls), unsafe.Pointer(&result)); code != eNoInterface || result != nil {
		t.Fatalf("asking for another interface gave %#x", code)
	}

	args := fakeButtonArgs(buttonNext)
	callHandler(handler.vtable.invoke, unsafe.Pointer(handler), nil, unsafe.Pointer(args))
	if len(received) != 1 || received[0].Command != Next {
		t.Fatalf("received %+v", received)
	}
}
