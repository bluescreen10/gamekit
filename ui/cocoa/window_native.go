//go:build darwin && cgo

package cocoa

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
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
	native := C.gkWindowCreate(ctitle, C.int(width), C.int(height), flags, owner,
		C.int(hints.Position.X), C.int(hints.Position.Y), &message[0], C.size_t(len(message)))
	if native == nil {
		detail := C.GoString(&message[0])
		if detail == "" {
			detail = "native window creation failed"
		}
		return nil, fmt.Errorf("ui/cocoa: %s", detail)
	}
	return native, nil
}

func destroyNativeWindow(native unsafe.Pointer) {
	C.gkWindowDestroy(native)
}
func pollNativeWindow(native unsafe.Pointer) {
	C.gkWindowPoll(native)
}
func nativeWindowShouldClose(native unsafe.Pointer) bool {
	return C.gkWindowShouldClose(native) != 0
}
func setNativeWindowShouldClose(native unsafe.Pointer, close bool) {
	value := C.int(0)
	if close {
		value = 1
	}
	C.gkWindowSetShouldClose(native, value)
}
func nativeWindowSize(native unsafe.Pointer) (int, int) {
	var width, height C.int
	C.gkWindowSize(native, &width, &height)
	return int(width), int(height)
}
func nativeWindowFramebufferSize(native unsafe.Pointer) (int, int) {
	var width, height C.int
	C.gkWindowFramebufferSize(native, &width, &height)
	return int(width), int(height)
}
func setNativeWindowSize(native unsafe.Pointer, width, height int) {
	C.gkWindowSetSize(native, C.int(width), C.int(height))
}
func nativeWindowPosition(native unsafe.Pointer) (int, int) {
	var x, y C.int
	C.gkWindowPosition(native, &x, &y)
	return int(x), int(y)
}
func setNativeWindowPosition(native unsafe.Pointer, x, y int) {
	C.gkWindowSetPosition(native, C.int(x), C.int(y))
}
func nativeWindowVisible(native unsafe.Pointer) bool {
	return C.gkWindowVisible(native) != 0
}
func setNativeWindowVisible(native unsafe.Pointer, visible bool) {
	value := C.int(0)
	if visible {
		value = 1
	}
	C.gkWindowSetVisible(native, value)
}
func bringNativeWindowToFront(native unsafe.Pointer) {
	C.gkWindowBringToFront(native)
}
func setNativeWindowTitle(native unsafe.Pointer, title string) {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	C.gkWindowSetTitle(native, ctitle)
}
func setNativeWindowTheme(native unsafe.Pointer, theme ui.Theme) {
	C.gkWindowSetTheme(native, C.int(theme))
}
func nativeWindowHandle(native unsafe.Pointer) uintptr {
	return uintptr(C.gkWindowNativeHandle(native))
}
func nativeWindowDisplay(native unsafe.Pointer) unsafe.Pointer {
	return C.gkWindowNativeDisplay(native)
}

func setNativeWindowHandle(native unsafe.Pointer, handle uintptr) {
	C.gkWindowSetHandle(native, C.uintptr_t(handle))
}
