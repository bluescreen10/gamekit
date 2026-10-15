//go:build darwin && cgo

package cocoa

import "github.com/bluescreen10/gamekit/ui"

// CanBecomeKey reports whether AppKit lets window take keyboard focus.
func CanBecomeKey(window ui.Window) bool {
	native, ok := window.(*Window)
	return ok && native.canBecomeKey()
}
