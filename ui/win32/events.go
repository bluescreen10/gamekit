//go:build windows && cgo

package win32

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE -D_WIN32_WINNT=0x0A00 -DWINVER=0x0A00 -DNTDDI_VERSION=0x0A00000B
#cgo LDFLAGS: -static-libstdc++ -static-libgcc -luser32 -lgdi32 -lcomdlg32 -lshell32 -lshlwapi -lole32 -loleaut32 -luuid -ldwmapi -luxtheme -loleacc -ladvapi32 -ld2d1 -ldwrite -lwindowscodecs -limm32 -lwinmm -lruntimeobject -lmfplay -lpropsys
#include "win32_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

// win32Event is an event reported by the native layer; see win32_bridge.h.
type win32Event struct {
	kind       int
	a, b, c    int
	x, y, z, w float64
	text       string
	text2      string
	object     unsafe.Pointer
}

// win32Receiver receives events for the Go value a handle names.
type win32Receiver interface {
	win32Event(event win32Event)
}

// win32Querier answers decisions the native layer asks for.
type win32Querier interface {
	win32Query(event win32Event) bool
}

func convertEvent(event *C.GKWin32Event) win32Event {
	result := win32Event{
		kind:   int(event.kind),
		a:      int(event.a),
		b:      int(event.b),
		c:      int(event.c),
		x:      float64(event.x),
		y:      float64(event.y),
		z:      float64(event.z),
		w:      float64(event.w),
		object: event.object,
	}
	if event.text != nil {
		result.text = C.GoString(event.text)
	}
	if event.text2 != nil {
		result.text2 = C.GoString(event.text2)
	}
	return result
}

//export gkWin32GoEvent
func gkWin32GoEvent(handle C.uintptr_t, event *C.GKWin32Event) {
	if handle == 0 || event == nil {
		return
	}
	if receiver, ok := cgo.Handle(handle).Value().(win32Receiver); ok {
		receiver.win32Event(convertEvent(event))
	}
}

//export gkWin32GoQuery
func gkWin32GoQuery(handle C.uintptr_t, event *C.GKWin32Event) C.int {
	if handle == 0 || event == nil {
		return 1
	}
	if querier, ok := cgo.Handle(handle).Value().(win32Querier); ok {
		return boolInt(querier.win32Query(convertEvent(event)))
	}
	return 1
}

// cString returns a C copy of value that the caller must free.
func cString(value string) *C.char {
	return C.CString(value)
}

func freeCString(value *C.char) {
	C.free(unsafe.Pointer(value))
}

// goStringFree converts a malloc'd C string and frees it.
func goStringFree(value *C.char) string {
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}

// joinNul joins values into a NUL-separated buffer, as the native layer
// expects for string lists.
func joinNul(values []string) []byte {
	var buffer []byte
	for _, value := range values {
		buffer = append(buffer, value...)
		buffer = append(buffer, 0)
	}
	return buffer
}

// splitNul splits a NUL-separated buffer of size bytes.
func splitNul(buffer unsafe.Pointer, size C.size_t) []string {
	if buffer == nil || size == 0 {
		return nil
	}
	data := C.GoBytes(buffer, C.int(size))
	var values []string
	start := 0
	for i, b := range data {
		if b == 0 {
			values = append(values, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		values = append(values, string(data[start:]))
	}
	return values
}

// bytesPointer returns a C pointer to the first byte of buffer, or nil.
func bytesPointer(buffer []byte) *C.char {
	if len(buffer) == 0 {
		return nil
	}
	return (*C.char)(unsafe.Pointer(&buffer[0]))
}
