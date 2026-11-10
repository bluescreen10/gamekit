//go:build linux && cgo

package gtk

import (
	"unsafe"

	cstrings "github.com/bluescreen10/gamekit/strings"
)

// stringPointer uses Go storage. The native call must copy the string before
// returning; native APIs that retain pointers need separately owned storage.
func stringPointer(value string) *cChar {
	return (*cChar)(unsafe.Pointer(cstrings.CString(value)))
}

func goString(value *cChar) string {
	return cstrings.GoString((*byte)(unsafe.Pointer(value)))
}

func takeString(value *cChar) string {
	defer gFree(unsafe.Pointer(value))
	return goString(value)
}

// Native string vectors are used by APIs that accept a char**. Allocating both
// the vector and strings here avoids passing Go pointers in native storage.
func nativeStringVector(values []string) (**cChar, func()) {
	data := gMalloc(gSize((len(values) + 1) * int(unsafe.Sizeof(uintptr(0)))))
	vector := unsafe.Slice((**cChar)(data), len(values)+1)
	for i, value := range values {
		vector[i] = gStrdup(stringPointer(value))
	}
	vector[len(values)] = nil
	return (**cChar)(data), func() {
		for _, value := range vector[:len(values)] {
			gFree(unsafe.Pointer(value))
		}
		gFree(data)
	}
}

func goBytes(pointer unsafe.Pointer, size cInt) []byte {
	if pointer == nil || size <= 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(pointer), int(size))...)
}
func goStringN(pointer *cChar, size cInt) string {
	if pointer == nil || size <= 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(pointer)), int(size)))
}
