//go:build darwin

package media

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework MediaPlayer
#include <stdlib.h>
#include "remote_darwin.h"
*/
import "C"

import (
    "sync"
    "time"
    "unsafe"
)

var (
    handlerLock sync.Mutex
    handler     func(Event)

    remoteCommands = map[C.int]Command{
        C.remotePlay:         Play,
        C.remotePause:        Pause,
        C.remoteToggle:       TogglePlay,
        C.remoteNext:         Next,
        C.remotePrevious:     Previous,
        C.remoteSkipForward:  SkipForward,
        C.remoteSkipBackward: SkipBackward,
        C.remoteSeek:         Seek,
    }
)

//export mediaCommand
func mediaCommand(code C.int, position C.double) {
    handlerLock.Lock()
    handle := handler
    handlerLock.Unlock()

    if command, ok := remoteCommands[code]; ok && handle != nil {
        handle(Event{Command: command, Position: time.Duration(float64(position) * float64(time.Second))})
    }
}

type Controls struct {
    closeOnce sync.Once
}

func Start(handle func(Event)) (*Controls, error) {
    handlerLock.Lock()
    handler = handle
    handlerLock.Unlock()

    C.startRemote(C.double(SkipInterval.Seconds()))
    return &Controls{}, nil
}

func (c *Controls) SetTrack(track Track) error {
    title := C.CString(track.Title)
    defer C.free(unsafe.Pointer(title))
    artist := C.CString(track.Artist)
    defer C.free(unsafe.Pointer(artist))

    var cover unsafe.Pointer
    if len(track.Cover) > 0 {
        cover = C.CBytes(track.Cover)
        defer C.free(cover)
    }

    C.setRemoteTrack(title, artist, cover, C.int(len(track.Cover)), C.double(track.Duration.Seconds()))
    return nil
}

func (c *Controls) SetStatus(status Status) error {
    playing := C.int(0)
    if status.Playing {
        playing = 1
    }

    C.setRemoteStatus(playing, C.double(status.Position.Seconds()), C.double(status.Rate))
    return nil
}

func (c *Controls) Close() {
    c.closeOnce.Do(func() {
        handlerLock.Lock()
        handler = nil
        handlerLock.Unlock()

        C.stopRemote()
    })
}

type shown struct {
    title   string
    playing bool
    elapsed time.Duration
    artwork bool
}

func nowPlaying() shown {
    title := C.remoteTitle()
    defer C.free(unsafe.Pointer(title))

    return shown{
        title:   C.GoString(title),
        playing: C.remoteIsPlaying() != 0,
        elapsed: time.Duration(float64(C.remoteElapsed()) * float64(time.Second)),
        artwork: C.remoteHasArtwork() != 0,
    }
}
