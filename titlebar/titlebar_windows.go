//go:build windows

package titlebar

import (
	"fmt"
	"image/color"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

const (
	darkModeAttribute           = 20
	darkModeAttributeBefore20H1 = 19
	borderColorAttribute        = 34
	captionColorAttribute       = 35
	textColorAttribute          = 36
	invalidArgument             = 0x80070057

	frameChanged = 0x0001 | 0x0002 | 0x0004 | 0x0010 | 0x0020
)

var (
	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")

	user32           = windows.NewLazySystemDLL("user32.dll")
	procSetWindowPos = user32.NewProc("SetWindowPos")

	redrawFrame = func(window uintptr) {
		procSetWindowPos.Call(window, 0, 0, 0, 0, 0, frameChanged)
	}
)

type state struct {
	dark       bool
	background uint32
	foreground uint32
}

var applied = map[uintptr]state{}

func Match(window fyne.Window, look Look) error {
	native, ok := window.(driver.NativeWindow)
	if !ok {
		return nil
	}

	var err error
	native.RunNative(func(context any) {
		if windowsContext, ok := context.(driver.WindowsWindowContext); ok && windowsContext.HWND != 0 {
			err = apply(windowsContext.HWND, state{
				dark:       look.Dark,
				background: colorRef(look.Background),
				foreground: colorRef(look.Foreground),
			})
		}
	})

	return err
}

func apply(window uintptr, want state) error {
	previous, known := applied[window]

	if current, ok := darkMode(window); !ok || current != want.dark {
		if err := setDarkMode(window, want.dark); err != nil {
			return err
		}
	}

	if !known || previous.background != want.background || previous.foreground != want.foreground {
		setAttribute(window, captionColorAttribute, want.background)
		setAttribute(window, borderColorAttribute, want.background)
		setAttribute(window, textColorAttribute, want.foreground)
	}

	if !known || previous.dark != want.dark {
		redrawFrame(window)
	}

	applied[window] = want
	return nil
}

func darkMode(window uintptr) (bool, bool) {
	for _, attribute := range []uintptr{darkModeAttribute, darkModeAttributeBefore20H1} {
		var value uint32
		result, _, _ := procDwmGetWindowAttribute.Call(window, attribute, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
		if result == 0 {
			return value != 0, true
		}
	}

	return false, false
}

func setDarkMode(window uintptr, dark bool) error {
	value := uint32(0)
	if dark {
		value = 1
	}

	result := setAttribute(window, darkModeAttribute, value)
	if result == invalidArgument {
		result = setAttribute(window, darkModeAttributeBefore20H1, value)
	}

	switch result {
	case 0, invalidArgument:
		return nil
	default:
		return fmt.Errorf("Windows didn't change the title bar colour (%#08x)", result)
	}
}

func setAttribute(window uintptr, attribute uintptr, value uint32) uint32 {
	result, _, _ := procDwmSetWindowAttribute.Call(window, attribute, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
	return uint32(result)
}

func colorRef(value color.Color) uint32 {
	if value == nil {
		return 0
	}

	red, green, blue, _ := value.RGBA()
	return red>>8 | (green>>8)<<8 | (blue>>8)<<16
}
