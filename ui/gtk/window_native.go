//go:build linux && cgo

package gtk

import (
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

func createNativeWindow(title string, width, height int, hints WindowOptions, owner unsafe.Pointer) (unsafe.Pointer, error) {
	ctitle := cString(title)
	defer gFree(unsafe.Pointer(ctitle))
	var flags cUint32
	if hints.Resizable {
		flags |= gkWindowResizable
	}
	if hints.Hidden {
		flags |= gkWindowHidden
	}
	if hints.Borderless {
		flags |= gkWindowBorderless
	}
	if hints.AlwaysOnTop {
		flags |= gkWindowAlwaysOnTop
	}
	if hints.TransparentFramebuffer {
		flags |= gkWindowTransparent
	}
	if hints.Kind == ui.WindowUtility {
		flags |= gkWindowUtility
	}
	var message [1024]cChar
	native := gkGTKWindowCreate(ctitle, cInt(width), cInt(height), flags, owner,
		cInt(hints.Position.X), cInt(hints.Position.Y), &message[0], cSize(len(message)))
	if native == nil {
		detail := goString(&message[0])
		if detail == "" {
			detail = "native window creation failed"
		}
		return nil, fmt.Errorf("ui/gtk: %s", detail)
	}
	return native, nil
}

func destroyNativeWindow(native unsafe.Pointer) {
	gkGTKWindowDestroy(native)
}
func pollNativeWindow(native unsafe.Pointer) {
	gkGTKWindowPoll(native)
}
func nativeWindowShouldClose(native unsafe.Pointer) bool {
	return gkGTKWindowShouldClose(native) != 0
}
func setNativeWindowShouldClose(native unsafe.Pointer, close bool) {
	value := cInt(0)
	if close {
		value = 1
	}
	gkGTKWindowSetShouldClose(native, value)
}
func nativeWindowSize(native unsafe.Pointer) (int, int) {
	var width, height cInt
	gkGTKWindowSize(native, &width, &height)
	return int(width), int(height)
}
func nativeWindowFramebufferSize(native unsafe.Pointer) (int, int) {
	var width, height cInt
	gkGTKWindowFramebufferSize(native, &width, &height)
	return int(width), int(height)
}
func setNativeWindowSize(native unsafe.Pointer, width, height int) {
	gkGTKWindowSetSize(native, cInt(width), cInt(height))
}
func nativeWindowPosition(native unsafe.Pointer) (int, int) {
	var x, y cInt
	gkGTKWindowPosition(native, &x, &y)
	return int(x), int(y)
}
func setNativeWindowPosition(native unsafe.Pointer, x, y int) {
	gkGTKWindowSetPosition(native, cInt(x), cInt(y))
}
func nativeWindowVisible(native unsafe.Pointer) bool {
	return gkGTKWindowVisible(native) != 0
}
func setNativeWindowVisible(native unsafe.Pointer, visible bool) {
	value := cInt(0)
	if visible {
		value = 1
	}
	gkGTKWindowSetVisible(native, value)
}
func bringNativeWindowToFront(native unsafe.Pointer) {
	gkGTKWindowBringToFront(native)
}
func setNativeWindowTitle(native unsafe.Pointer, title string) {
	ctitle := cString(title)
	defer gFree(unsafe.Pointer(ctitle))
	gkGTKWindowSetTitle(native, ctitle)
}
func setNativeWindowTheme(native unsafe.Pointer, theme ui.Theme) {
	gkGTKWindowSetTheme(native, cInt(theme))
}
func nativeWindowHandle(native unsafe.Pointer) uintptr {
	return uintptr(gkGTKWindowNativeHandle(native))
}
func nativeWindowDisplay(native unsafe.Pointer) unsafe.Pointer {
	return gkGTKWindowNativeDisplay(native)
}

func setNativeWindowHandle(native unsafe.Pointer, handle uintptr) {
	gkGTKWindowSetHandle(native, cUintptr(handle))
}
