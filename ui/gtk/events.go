//go:build linux && cgo

package gtk

import (
	"runtime/cgo"
	"unsafe"
)

// gtkEvent carries a native event to its owning Go component.
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

// cString returns a C copy of value that the caller must free.
func cString(value string) *cChar {
	return gStrdup(stringPointer(value))
}

func freeCString(value *cChar) {
	gFree(unsafe.Pointer(value))
}

// goStringFree converts a malloc'd C string and frees it.
func goStringFree(value *cChar) string {
	if value == nil {
		return ""
	}
	defer gFree(unsafe.Pointer(value))
	return goString(value)
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
func splitNul(buffer unsafe.Pointer, size cSize) []string {
	if buffer == nil || size == 0 {
		return nil
	}
	data := goBytes(buffer, cInt(size))
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
func bytesPointer(buffer []byte) *cChar {
	if len(buffer) == 0 {
		return nil
	}
	return (*cChar)(unsafe.Pointer(&buffer[0]))
}

func dispatchNativeEvent(handle uintptr, event gtkEvent) {
	if handle != 0 {
		if receiver, ok := cgo.Handle(handle).Value().(gtkReceiver); ok {
			receiver.gtkEvent(event)
		}
	}
}

func queryNativeEvent(handle uintptr, event gtkEvent) bool {
	if handle == 0 {
		return false
	}
	if receiver, ok := cgo.Handle(handle).Value().(gtkQuerier); ok {
		return receiver.gtkQuery(event)
	}
	return true
}
