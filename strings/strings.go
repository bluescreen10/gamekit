// Package strings converts between Go strings and the NUL-terminated strings of C,
// in Go memory, for packages that call C.
//
// A C type is a different type in each package that uses cgo, so these functions take
// and return *byte, which a caller converts to its own *C.char.
package strings

import "unsafe"

// CString returns s as a NUL-terminated C string in Go memory. Passed directly to a C
// call, it needs no pinning. Pin it before storing it in memory that is passed to C,
// and do not let C keep it past the call.
func CString(s string) *byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0]
}

// GoString returns a copy of the NUL-terminated C string at p, or "" if p is nil.
func GoString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
