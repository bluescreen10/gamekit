//go:build darwin && cgo

package cocoa

/*
#cgo LDFLAGS: -framework UserNotifications -framework UniformTypeIdentifiers -framework AVKit -framework AVFoundation -framework CoreMedia -framework CoreImage -framework WebKit -framework GameController
#include "cocoa_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

// cocoaEvent is an event reported by the AppKit layer; see cocoa_bridge.h.
type cocoaEvent struct {
	kind       int
	a, b, c    int
	x, y, z, w float64
	text       string
	text2      string
	object     unsafe.Pointer
}

// cocoaReceiver receives events for the Go value a handle names.
type cocoaReceiver interface {
	cocoaEvent(event cocoaEvent)
}

// cocoaQuerier answers decisions the AppKit layer asks for.
type cocoaQuerier interface {
	cocoaQuery(event cocoaEvent) bool
}

func convertEvent(event *C.GKCocoaEvent) cocoaEvent {
	result := cocoaEvent{
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

//export gkCocoaGoEvent
func gkCocoaGoEvent(handle C.uintptr_t, event *C.GKCocoaEvent) {
	if handle == 0 || event == nil {
		return
	}
	if receiver, ok := cgo.Handle(handle).Value().(cocoaReceiver); ok {
		receiver.cocoaEvent(convertEvent(event))
	}
}

//export gkCocoaGoQuery
func gkCocoaGoQuery(handle C.uintptr_t, event *C.GKCocoaEvent) C.int {
	if handle == 0 || event == nil {
		return 1
	}
	if querier, ok := cgo.Handle(handle).Value().(cocoaQuerier); ok {
		return boolInt(querier.cocoaQuery(convertEvent(event)))
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

// joinNul joins values into a NUL-separated buffer, as the AppKit layer
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
