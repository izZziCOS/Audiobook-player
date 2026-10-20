//go:build windows

package titlebar

import (
    "fmt"
    "unsafe"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/driver"
    "golang.org/x/sys/windows"
)

const (
    darkModeAttribute           = 20
    darkModeAttributeBefore20H1 = 19
    invalidArgument             = 0x80070057

    redrawFrame = 0x0001 | 0x0002 | 0x0004 | 0x0010 | 0x0020
)

var (
    dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
    procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

    user32           = windows.NewLazySystemDLL("user32.dll")
    procSetWindowPos = user32.NewProc("SetWindowPos")
)

func SetDark(window fyne.Window, dark bool) error {
    native, ok := window.(driver.NativeWindow)
    if !ok {
        return nil
    }

    var err error
    native.RunNative(func(context any) {
        if windowsContext, ok := context.(driver.WindowsWindowContext); ok && windowsContext.HWND != 0 {
            err = setDark(windowsContext.HWND, dark)
        }
    })

    return err
}

func setDark(window uintptr, dark bool) error {
    value := int32(0)
    if dark {
        value = 1
    }

    result := setAttribute(window, darkModeAttribute, value)
    if result == invalidArgument {
        result = setAttribute(window, darkModeAttributeBefore20H1, value)
    }

    switch result {
    case 0:
        procSetWindowPos.Call(window, 0, 0, 0, 0, 0, redrawFrame)
        return nil
    case invalidArgument:
        return nil
    default:
        return fmt.Errorf("Windows didn't change the title bar colour (%#08x)", uint32(result))
    }
}

func setAttribute(window uintptr, attribute uintptr, value int32) uint32 {
    result, _, _ := procDwmSetWindowAttribute.Call(window, attribute, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
    return uint32(result)
}
