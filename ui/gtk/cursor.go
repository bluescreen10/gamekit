//go:build linux && cgo

package gtk

/*
#include "window_bridge.h"
*/
import "C"

// GetPointerPos returns the cursor in logical pixels relative to the top-left
// corner of the window's content area.
func (w *Window) GetPointerPos() (x, y float64) {
	if native := w.nativePointer(); native != nil {
		var nativeX, nativeY C.double
		C.gkGTKWindowCursorPosition(native, &nativeX, &nativeY)
		return float64(nativeX), float64(nativeY)
	}
	return 0, 0
}
