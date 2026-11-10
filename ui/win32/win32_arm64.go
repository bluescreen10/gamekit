//go:build windows

package win32

// floatThunk calls the function of a floatFrame, whose address it takes in R0,
// with the frame's integer and floating-point register arguments, and stores
// the function's results in the frame. It is called through syscall with the
// native calling convention, so it has no Go declaration to call it directly.
func floatThunk()

// floatThunkAddress returns the address of floatThunk.
func floatThunkAddress() uintptr
