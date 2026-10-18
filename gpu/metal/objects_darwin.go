//go:build darwin && cgo

package metal

import (
	"unsafe"

	"github.com/bluescreen10/gamekit/strings"
)

// Helpers over the messages in objc_darwin.go.

func class(name string) id {
	return objcGetClass(name)
}

// newObject returns [[name alloc] init], owned by the caller.
func newObject(name string) id {
	return objectNew(class(name))
}

func release(x id) {
	if x != 0 {
		objectRelease(x)
	}
}

func retain(x id) id {
	if x != 0 {
		objectRetain(x)
	}
	return x
}

// nsString returns s as a new NSString, which the caller releases.
func nsString(s string) id {
	str := objectAlloc(class("NSString"))
	if len(s) == 0 {
		return objectInit(str)
	}
	b := []byte(s)
	return stringInitWithBytes(str, unsafe.Pointer(&b[0]), uint(len(b)), nsUTF8StringEncoding)
}

// stringOf returns an NSString's contents.
func stringOf(s id) string {
	if s == 0 {
		return ""
	}
	return strings.GoString((*byte)(stringUTF8String(s)))
}

// errorText returns an NSError's description, or fallback when there is no error.
func errorText(err id, fallback string) string {
	if err == 0 {
		return fallback
	}
	return stringOf(errorLocalizedDescription(err))
}

// setLabel names a Metal object for tools like the Metal debugger.
func setLabel(x id, label string) {
	if label == "" {
		return
	}
	s := nsString(label)
	objectSetLabel(x, s)
	release(s)
}

// unsafeBytes is n bytes of native memory at p+off.
func unsafeBytes(p unsafe.Pointer, off uint64, n int) []byte {
	return unsafe.Slice((*byte)(unsafe.Add(p, off)), n)
}
