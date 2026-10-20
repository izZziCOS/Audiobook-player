//go:build windows

package media

import (
    "errors"
    "fmt"
    "runtime"
    "strings"
    "sync"
    "sync/atomic"
    "syscall"
    "unsafe"

    "golang.org/x/sys/windows"
)

var (
    combase                       = windows.NewLazySystemDLL("combase.dll")
    procRoInitialize              = combase.NewProc("RoInitialize")
    procRoUninitialize            = combase.NewProc("RoUninitialize")
    procRoGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
    procWindowsCreateString       = combase.NewProc("WindowsCreateString")
    procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
    procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")

    shcore                                 = windows.NewLazySystemDLL("shcore.dll")
    procCreateRandomAccessStreamOverStream = shcore.NewProc("CreateRandomAccessStreamOverStream")

    shlwapi               = windows.NewLazySystemDLL("shlwapi.dll")
    procSHCreateMemStream = shlwapi.NewProc("SHCreateMemStream")

    user32               = windows.NewLazySystemDLL("user32.dll")
    procRegisterClassExW = user32.NewProc("RegisterClassExW")
    procCreateWindowExW  = user32.NewProc("CreateWindowExW")
    procDestroyWindow    = user32.NewProc("DestroyWindow")
    procDefWindowProcW   = user32.NewProc("DefWindowProcW")
    procGetMessageW      = user32.NewProc("GetMessageW")
    procDispatchMessageW = user32.NewProc("DispatchMessageW")
    procPostMessageW     = user32.NewProc("PostMessageW")
    procPostQuitMessage  = user32.NewProc("PostQuitMessage")
)

const (
    controlsPlaybackStatus          = 6
    controlsSetPlaybackStatus       = 7
    controlsDisplayUpdater          = 8
    controlsSetIsEnabled            = 11
    controlsSetIsPlayEnabled        = 13
    controlsSetIsPauseEnabled       = 17
    controlsSetIsFastForwardEnabled = 21
    controlsSetIsRewindEnabled      = 23
    controlsSetIsPreviousEnabled    = 25
    controlsSetIsNextEnabled        = 27
    controlsAddButtonPressed        = 32
    controlsRemoveButtonPressed     = 33

    updaterSetType         = 7
    updaterSetThumbnail    = 11
    updaterMusicProperties = 12
    updaterUpdate          = 17

    musicTitle     = 6
    musicSetTitle  = 7
    musicArtist    = 10
    musicSetArtist = 11

    buttonEventButton = 6

    streamReferenceFromStream = 8

    interopGetForWindow = 6
)

const (
    statusPlaying = 3
    statusPaused  = 4

    playbackTypeMusic = 1

    eNoInterface         = 0x80004002
    classAlreadyExists   = 1410
    roInitSingleThreaded = 0
    wmRunWork            = 0x8000 + 1
)

var (
    errClosed     = errors.New("media controls are closed")
    className     = windows.StringToUTF16Ptr("AudiobookPlayerMediaControls")
    windowTitle   = windows.StringToUTF16Ptr("Audiobook Player")
    registerClass sync.Once
    classErr      error
)

type object struct {
    vtable *[64]uintptr
}

type callError struct {
    method int
    code   uint32
}

func (e callError) Error() string {
    return fmt.Sprintf("Windows media controls: call %d failed with %#08x", e.method, e.code)
}

func failed(result uintptr) bool {
    return int32(result) < 0
}

//go:uintptrescapes
func (o *object) call(method int, args ...uintptr) error {
    result, _, _ := syscall.SyscallN(o.vtable[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
    if failed(result) {
        return callError{method: method, code: uint32(result)}
    }

    return nil
}

func (o *object) release() {
    if o != nil {
        syscall.SyscallN(o.vtable[2], uintptr(unsafe.Pointer(o)))
    }
}

func asObject(pointer uintptr) *object {
    return *(**object)(unsafe.Pointer(&pointer))
}

func newString(text string) (uintptr, error) {
    characters, err := windows.UTF16FromString(strings.ReplaceAll(text, "\x00", ""))
    if err != nil || len(characters) <= 1 {
        return 0, err
    }

    var handle uintptr
    result, _, _ := procWindowsCreateString.Call(
        uintptr(unsafe.Pointer(&characters[0])),
        uintptr(len(characters)-1),
        uintptr(unsafe.Pointer(&handle)),
    )
    if failed(result) {
        return 0, fmt.Errorf("creating a Windows string failed with %#08x", uint32(result))
    }

    return handle, nil
}

func deleteString(handle uintptr) {
    if handle != 0 {
        procWindowsDeleteString.Call(handle)
    }
}

func readString(handle uintptr) string {
    if handle == 0 {
        return ""
    }

    var length uint32
    buffer, _, _ := procWindowsGetStringRawBuffer.Call(handle, uintptr(unsafe.Pointer(&length)))
    return windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&buffer)))
}

func setString(target *object, method int, text string) error {
    handle, err := newString(text)
    if err != nil {
        return err
    }
    defer deleteString(handle)

    return target.call(method, handle)
}

func getString(target *object, method int) (string, error) {
    var handle uintptr
    if err := target.call(method, uintptr(unsafe.Pointer(&handle))); err != nil {
        return "", err
    }
    defer deleteString(handle)

    return readString(handle), nil
}

func activationFactory(class string, iid *guid) (*object, error) {
    name, err := newString(class)
    if err != nil {
        return nil, err
    }
    defer deleteString(name)

    var factory *object
    result, _, _ := procRoGetActivationFactory.Call(name, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&factory)))
    if failed(result) {
        return nil, fmt.Errorf("%s is not available (%#08x)", class, uint32(result))
    }

    return factory, nil
}

type buttonHandler struct {
    vtable *buttonHandlerVtable
    refs   atomic.Int32
    handle func(Event)
}

type buttonHandlerVtable struct {
    queryInterface uintptr
    addRef         uintptr
    release        uintptr
    invoke         uintptr
}

var handlerVtable = &buttonHandlerVtable{
    queryInterface: syscall.NewCallback(func(this *buttonHandler, iid *guid, result **buttonHandler) uintptr {
        if *iid != iidUnknown && *iid != iidAgileObject && *iid != iidButtonPressedHandler {
            *result = nil
            return eNoInterface
        }

        this.refs.Add(1)
        *result = this
        return 0
    }),
    addRef: syscall.NewCallback(func(this *buttonHandler) uintptr {
        return uintptr(this.refs.Add(1))
    }),
    release: syscall.NewCallback(func(this *buttonHandler) uintptr {
        return uintptr(this.refs.Add(-1))
    }),
    invoke: syscall.NewCallback(func(this *buttonHandler, sender, args *object) uintptr {
        var button transportButton
        if args.call(buttonEventButton, uintptr(unsafe.Pointer(&button))) != nil {
            return 0
        }

        if command, ok := commandForButton(button); ok {
            this.handle(Event{Command: command})
        }
        return 0
    }),
}

type message struct {
    window   uintptr
    message  uint32
    wParam   uintptr
    lParam   uintptr
    time     uint32
    pointX   int32
    pointY   int32
    lPrivate uint32
}

type windowClass struct {
    size        uint32
    style       uint32
    windowProc  uintptr
    classExtra  int32
    windowExtra int32
    instance    windows.Handle
    icon        uintptr
    cursor      uintptr
    background  uintptr
    menuName    *uint16
    className   *uint16
    smallIcon   uintptr
}

type Controls struct {
    work      chan func()
    done      chan struct{}
    closeOnce sync.Once
    window    uintptr

    controls *object
    streams  *object
    handler  *buttonHandler
    token    int64
    enabled  bool
}

func Start(handle func(Event)) (*Controls, error) {
    c := &Controls{
        work:    make(chan func(), 16),
        done:    make(chan struct{}),
        handler: &buttonHandler{vtable: handlerVtable, handle: handle},
    }

    started := make(chan error, 1)
    go c.run(started)
    if err := <-started; err != nil {
        return nil, err
    }

    return c, nil
}

func (c *Controls) run(started chan<- error) {
    runtime.LockOSThread()
    defer close(c.done)

    procRoInitialize.Call(roInitSingleThreaded)
    defer procRoUninitialize.Call()

    err := c.open()
    started <- err
    if err == nil {
        c.pump()
    }

    c.shutDown()
}

func (c *Controls) open() error {
    window, err := createWindow()
    if err != nil {
        return err
    }
    c.window = window

    interop, err := activationFactory("Windows.Media.SystemMediaTransportControls", &iidTransportControlsInterop)
    if err != nil {
        return err
    }
    defer interop.release()

    if err := interop.call(interopGetForWindow, c.window, uintptr(unsafe.Pointer(&iidTransportControls)), uintptr(unsafe.Pointer(&c.controls))); err != nil {
        return err
    }

    c.streams, err = activationFactory("Windows.Storage.Streams.RandomAccessStreamReference", &iidRandomAccessStreamReferences)
    if err != nil {
        return err
    }

    for _, method := range []int{
        controlsSetIsPlayEnabled,
        controlsSetIsPauseEnabled,
        controlsSetIsNextEnabled,
        controlsSetIsPreviousEnabled,
        controlsSetIsFastForwardEnabled,
        controlsSetIsRewindEnabled,
    } {
        if err := c.controls.call(method, 1); err != nil {
            return err
        }
    }

    return c.controls.call(controlsAddButtonPressed, uintptr(unsafe.Pointer(c.handler)), uintptr(unsafe.Pointer(&c.token)))
}

func createWindow() (uintptr, error) {
    var instance windows.Handle
    if err := windows.GetModuleHandleEx(0, nil, &instance); err != nil {
        return 0, err
    }

    registerClass.Do(func() {
        class := windowClass{
            windowProc: procDefWindowProcW.Addr(),
            instance:   instance,
            className:  className,
        }
        class.size = uint32(unsafe.Sizeof(class))

        if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class))); atom == 0 && err != syscall.Errno(classAlreadyExists) {
            classErr = fmt.Errorf("registering the media controls window: %w", err)
        }
    })
    if classErr != nil {
        return 0, classErr
    }

    window, _, err := procCreateWindowExW.Call(
        0,
        uintptr(unsafe.Pointer(className)),
        uintptr(unsafe.Pointer(windowTitle)),
        0, 0, 0, 0, 0, 0, 0,
        uintptr(instance),
        0,
    )
    if window == 0 {
        return 0, fmt.Errorf("creating the media controls window: %w", err)
    }

    return window, nil
}

func (c *Controls) pump() {
    var next message
    for {
        result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&next)), 0, 0, 0)
        if int32(result) <= 0 {
            return
        }

        if next.message == wmRunWork {
            c.runWork()
            continue
        }

        procDispatchMessageW.Call(uintptr(unsafe.Pointer(&next)))
    }
}

func (c *Controls) runWork() {
    for {
        select {
        case work := <-c.work:
            work()
        default:
            return
        }
    }
}

func (c *Controls) shutDown() {
    if c.controls != nil {
        if c.token != 0 {
            c.controls.call(controlsRemoveButtonPressed, uintptr(c.token))
        }
        c.controls.call(controlsSetIsEnabled, 0)
        c.controls.release()
        c.controls = nil
    }

    c.streams.release()
    c.streams = nil

    if c.window != 0 {
        procDestroyWindow.Call(c.window)
    }
}

func (c *Controls) do(work func() error) error {
    result := make(chan error, 1)

    select {
    case c.work <- func() { result <- work() }:
    case <-c.done:
        return errClosed
    }

    procPostMessageW.Call(c.window, wmRunWork, 0, 0)

    select {
    case err := <-result:
        return err
    case <-c.done:
        return errClosed
    }
}

func (c *Controls) SetTrack(track Track) error {
    return c.do(func() error {
        return c.showTrack(track)
    })
}

func (c *Controls) showTrack(track Track) error {
    var updater *object
    if err := c.controls.call(controlsDisplayUpdater, uintptr(unsafe.Pointer(&updater))); err != nil {
        return err
    }
    defer updater.release()

    if err := updater.call(updaterSetType, playbackTypeMusic); err != nil {
        return err
    }

    var music *object
    if err := updater.call(updaterMusicProperties, uintptr(unsafe.Pointer(&music))); err != nil {
        return err
    }
    defer music.release()

    if err := setString(music, musicSetTitle, track.Title); err != nil {
        return err
    }
    if err := setString(music, musicSetArtist, track.Artist); err != nil {
        return err
    }

    thumbnail, thumbnailErr := c.thumbnail(track.Cover)
    defer thumbnail.release()
    if err := updater.call(updaterSetThumbnail, uintptr(unsafe.Pointer(thumbnail))); err != nil {
        return err
    }

    if err := updater.call(updaterUpdate); err != nil {
        return err
    }

    if !c.enabled {
        if err := c.controls.call(controlsSetIsEnabled, 1); err != nil {
            return err
        }
        c.enabled = true
    }

    return thumbnailErr
}

func (c *Controls) thumbnail(cover []byte) (*object, error) {
    if len(cover) == 0 {
        return nil, nil
    }

    pointer, _, _ := procSHCreateMemStream.Call(uintptr(unsafe.Pointer(&cover[0])), uintptr(len(cover)))
    if pointer == 0 {
        return nil, errors.New("Windows media controls: couldn't copy the cover art")
    }
    memory := asObject(pointer)
    defer memory.release()

    var stream *object
    result, _, _ := procCreateRandomAccessStreamOverStream.Call(
        uintptr(unsafe.Pointer(memory)),
        0,
        uintptr(unsafe.Pointer(&iidRandomAccessStream)),
        uintptr(unsafe.Pointer(&stream)),
    )
    if failed(result) {
        return nil, fmt.Errorf("Windows media controls: couldn't read the cover art (%#08x)", uint32(result))
    }
    defer stream.release()

    var reference *object
    if err := c.streams.call(streamReferenceFromStream, uintptr(unsafe.Pointer(stream)), uintptr(unsafe.Pointer(&reference))); err != nil {
        return nil, err
    }

    return reference, nil
}

func (c *Controls) SetStatus(status Status) error {
    return c.do(func() error {
        playback := statusPaused
        if status.Playing {
            playback = statusPlaying
        }

        return c.controls.call(controlsSetPlaybackStatus, uintptr(playback))
    })
}

func (c *Controls) Close() {
    c.closeOnce.Do(func() {
        c.do(func() error {
            procPostQuitMessage.Call(0)
            return nil
        })
        <-c.done
    })
}
