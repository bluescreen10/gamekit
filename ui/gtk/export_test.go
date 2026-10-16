//go:build linux && cgo

package gtk

import "github.com/bluescreen10/gamekit/ui"

// CanBecomeKey reports whether GTK lets window take keyboard focus.
func CanBecomeKey(window ui.Window) bool {
	native, ok := window.(*Window)
	return ok && native.canBecomeKey()
}
