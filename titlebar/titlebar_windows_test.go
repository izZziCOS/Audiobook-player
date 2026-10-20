package titlebar

import (
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
    })

    return window
}

func isDark(window uintptr) bool {
    var value int32
    dwmapi.NewProc("DwmGetWindowAttribute").Call(window, darkModeAttribute, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
    return value != 0
}

func TestTitleBarFollowsTheChoice(t *testing.T) {
    window := testWindow(t)

    if err := setDark(window, true); err != nil {
        t.Fatal(err)
    }
    if !isDark(window) {
        t.Skip("this Windows has no dark title bars")
    }

    if err := setDark(window, false); err != nil {
        t.Fatal(err)
    }
    if isDark(window) {
        t.Fatal("the title bar should be light again")
    }
}
