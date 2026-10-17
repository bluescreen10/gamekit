//go:build windows && cgo

package win32

import "runtime"

// Windows delivers the messages of a window to the thread that created it,
// so window creation and event dispatch stay on the main thread.
func init() {
	runtime.LockOSThread()
}
