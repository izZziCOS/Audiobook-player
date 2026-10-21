package titlebar

import (
    "image/color"
    "testing"
    "unsafe"

    "golang.org/x/sys/windows"
)

func testWindow(t *testing.T) uintptr {
    t.Helper()

    procCreateWindowExW := user32.NewProc("CreateWindowExW")
    procDestroyWindow := user32.NewProc("DestroyWindow")

    const overlappedWindow = 0x00CF0000
    window, _, err := procCreateWindowExW.Call(
        0,
        uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("STATIC"))),
        uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Title bar test"))),
        overlappedWindow,
        0, 0, 300, 200, 0, 0, 0, 0,
    )
    if window == 0 {
        t.Fatalf("creating a window: %v", err)
    }
    t.Cleanup(func() {
        procDestroyWindow.Call(window)
        delete(applied, window)
    })

    return window
}

func isDark(window uintptr) bool {
    dark, _ := darkMode(window)
    return dark
}

func countRedraws(t *testing.T) *int {
    t.Helper()

    count := 0
    original := redrawFrame
    redrawFrame = func(uintptr) { count++ }
    t.Cleanup(func() { redrawFrame = original })

    return &count
}

var (
    light = state{dark: false, background: 0x00f0f4f7, foreground: 0x00191b1d}
    dark  = state{dark: true, background: 0x00151313, foreground: 0x00e7ebee}
)

func TestTitleBarFollowsTheChoice(t *testing.T) {
    window := testWindow(t)

    if err := apply(window, dark); err != nil {
        t.Fatal(err)
    }
    if !isDark(window) {
        t.Skip("this Windows has no dark title bars")
    }

    if err := apply(window, light); err != nil {
        t.Fatal(err)
    }
    if isDark(window) {
        t.Fatal("the title bar should be light again")
    }
}

func TestMatchingAgainDoesntRedrawTheFrame(t *testing.T) {
    window := testWindow(t)
    redraws := countRedraws(t)

    for range 3 {
        if err := apply(window, dark); err != nil {
            t.Fatal(err)
        }
    }
    if *redraws != 1 {
        t.Fatalf("the frame was redrawn %d times, want only when the appearance changes", *redraws)
    }
    if !isDark(window) {
        t.Skip("this Windows has no dark title bars")
    }

    setAttribute(window, darkModeAttribute, 0)
    if err := apply(window, dark); err != nil {
        t.Fatal(err)
    }
    if !isDark(window) || *redraws != 1 {
        t.Fatalf("a reset by something else should be put right quietly, redraws %d", *redraws)
    }
}

func TestChangingTheAppearanceRedrawsOnce(t *testing.T) {
    window := testWindow(t)
    redraws := countRedraws(t)

    apply(window, light)
    apply(window, dark)
    apply(window, dark)
    apply(window, light)

    if *redraws != 3 {
        t.Fatalf("the frame was redrawn %d times, want once per change", *redraws)
    }
}

func TestColorRef(t *testing.T) {
    if got := colorRef(color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}); got != 0x00563412 {
        t.Fatalf("got %#08x, want Windows' blue-green-red order", got)
    }
}
