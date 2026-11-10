//go:build windows && !arm64

package win32

// floatThunkAddress is only called on arm64; see win32_arm64.s.
func floatThunkAddress() uintptr {
	panic("ui/win32: floatThunk is only used on arm64")
}
