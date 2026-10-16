//go:build linux && cgo

package gtk

/*
#cgo pkg-config: gtk4 x11
#cgo LDFLAGS: -lm
#include "gtk_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

// gtkEvent is an event reported by the GTK layer; see gtk_bridge.h.
type gtkEvent struct {
	kind       int
	a, b, c    int
	x, y, z, w float64
	text       string
	text2      string
	object     unsafe.Pointer
}

// gtkReceiver receives events for the Go value a handle names.
type gtkReceiver interface {
	gtkEvent(event gtkEvent)
}

// gtkQuerier answers decisions the GTK layer asks for.
type gtkQuerier interface {
	gtkQuery(event gtkEvent) bool
}

func convertEvent(event *C.GKGTKEvent) gtkEvent {
	result := gtkEvent{
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

//export gkGTKGoEvent
func gkGTKGoEvent(handle C.uintptr_t, event *C.GKGTKEvent) {
	if handle == 0 || event == nil {
		return
	}
	if receiver, ok := cgo.Handle(handle).Value().(gtkReceiver); ok {
		receiver.gtkEvent(convertEvent(event))
	}
}

//export gkGTKGoQuery
func gkGTKGoQuery(handle C.uintptr_t, event *C.GKGTKEvent) C.int {
	if handle == 0 || event == nil {
		return 1
	}
	if querier, ok := cgo.Handle(handle).Value().(gtkQuerier); ok {
		return boolInt(querier.gtkQuery(convertEvent(event)))
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

// joinNul joins values into a NUL-separated buffer, as the GTK layer
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
