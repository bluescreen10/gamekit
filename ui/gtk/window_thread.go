//go:build linux && cgo

package gtk

import "runtime"

// GTK requires window creation and event dispatch on the thread that
// initialized it.
func init() {
	runtime.LockOSThread()
}
