//go:build windows && cgo

package win32

import "github.com/bluescreen10/gamekit/ui"

// CanBecomeKey reports whether Windows lets window take keyboard focus.
func CanBecomeKey(window ui.Window) bool {
	native, ok := window.(*Window)
	return ok && native.canBecomeKey()
}
