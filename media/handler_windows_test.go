package media

import (
    "syscall"
    "unsafe"
)

func callHandler(method uintptr, args ...unsafe.Pointer) uintptr {
    values := make([]uintptr, len(args))
    for index, arg := range args {
        values[index] = uintptr(arg)
    }

    result, _, _ := syscall.SyscallN(method, values...)
    return result
}

type fakeArgs struct {
    vtable *[64]uintptr
    button transportButton
}

var fakeArgsVtable = func() *[64]uintptr {
    var table [64]uintptr
    table[buttonEventButton] = syscall.NewCallback(func(this *fakeArgs, button *transportButton) uintptr {
        *button = this.button
        return 0
    })
    return &table
}()

func fakeButtonArgs(button transportButton) *fakeArgs {
    return &fakeArgs{vtable: fakeArgsVtable, button: button}
}
