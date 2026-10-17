//go:build windows && cgo

package win32

/*
#include "window_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

func createNativeWindow(title string, width, height int, hints WindowOptions, owner unsafe.Pointer) (unsafe.Pointer, error) {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	var flags C.uint32_t
	if hints.Resizable {
		flags |= C.GK_WINDOW_RESIZABLE
	}
	if hints.Hidden {
		flags |= C.GK_WINDOW_HIDDEN
	}
	if hints.Borderless {
		flags |= C.GK_WINDOW_BORDERLESS
	}
	if hints.AlwaysOnTop {
		flags |= C.GK_WINDOW_ALWAYS_ON_TOP
	}
	if hints.TransparentFramebuffer {
		flags |= C.GK_WINDOW_TRANSPARENT
	}
	if hints.Kind == ui.WindowUtility {
		flags |= C.GK_WINDOW_UTILITY
	}
	var message [1024]C.char
	native := C.gkWin32WindowCreate(ctitle, C.int(width), C.int(height), flags, owner,
		C.int(hints.Position.X), C.int(hints.Position.Y), &message[0], C.size_t(len(message)))
	if native == nil {
		detail := C.GoString(&message[0])
		if detail == "" {
			detail = "native window creation failed"
		}
		return nil, fmt.Errorf("ui/win32: %s", detail)
	}
	return native, nil
}

func destroyNativeWindow(native unsafe.Pointer) {
	C.gkWin32WindowDestroy(native)
}
func pollNativeWindow(native unsafe.Pointer) {
	C.gkWin32WindowPoll(native)
}
func nativeWindowShouldClose(native unsafe.Pointer) bool {
	return C.gkWin32WindowShouldClose(native) != 0
}
func setNativeWindowShouldClose(native unsafe.Pointer, close bool) {
	value := C.int(0)
	if close {
		value = 1
	}
	C.gkWin32WindowSetShouldClose(native, value)
}
func nativeWindowSize(native unsafe.Pointer) (int, int) {
	var width, height C.int
	C.gkWin32WindowSize(native, &width, &height)
	return int(width), int(height)
}
func nativeWindowFramebufferSize(native unsafe.Pointer) (int, int) {
	var width, height C.int
	C.gkWin32WindowFramebufferSize(native, &width, &height)
	return int(width), int(height)
}
func setNativeWindowSize(native unsafe.Pointer, width, height int) {
	C.gkWin32WindowSetSize(native, C.int(width), C.int(height))
}
func nativeWindowPosition(native unsafe.Pointer) (int, int) {
	var x, y C.int
	C.gkWin32WindowPosition(native, &x, &y)
	return int(x), int(y)
}
func setNativeWindowPosition(native unsafe.Pointer, x, y int) {
	C.gkWin32WindowSetPosition(native, C.int(x), C.int(y))
}
func nativeWindowVisible(native unsafe.Pointer) bool {
	return C.gkWin32WindowVisible(native) != 0
}
func setNativeWindowVisible(native unsafe.Pointer, visible bool) {
	value := C.int(0)
	if visible {
		value = 1
	}
	C.gkWin32WindowSetVisible(native, value)
}
func bringNativeWindowToFront(native unsafe.Pointer) {
	C.gkWin32WindowBringToFront(native)
}
func setNativeWindowTitle(native unsafe.Pointer, title string) {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	C.gkWin32WindowSetTitle(native, ctitle)
}
func setNativeWindowTheme(native unsafe.Pointer, theme ui.Theme) {
	C.gkWin32WindowSetTheme(native, C.int(theme))
}
func nativeWindowHandle(native unsafe.Pointer) uintptr {
	return uintptr(C.gkWin32WindowNativeHandle(native))
}
func nativeWindowDisplay(native unsafe.Pointer) unsafe.Pointer {
	return C.gkWin32WindowNativeDisplay(native)
}

func setNativeWindowHandle(native unsafe.Pointer, handle uintptr) {
	C.gkWin32WindowSetHandle(native, C.uintptr_t(handle))
}
