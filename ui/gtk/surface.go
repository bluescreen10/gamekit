//go:build linux && cgo

package gtk

/*
#include "window_bridge.h"
*/
import "C"

import "github.com/bluescreen10/gamekit/gpu"

// NativeSurface describes the X11 window of this window to a GPU backend.
func (w *Window) NativeSurface() gpu.NativeSurface {
	native := w.nativePointer()
	if native == nil {
		return gpu.NativeSurface{}
	}
	handle := uintptr(C.gkGTKWindowNativeHandle(native))
	display := uintptr(C.gkGTKWindowNativeDisplayHandle(native))
	if handle == 0 || display == 0 {
		return gpu.NativeSurface{}
	}
	return gpu.NativeSurface{
		Kind:    gpu.NativeSurfaceXlibWindow,
		Handle:  handle,
		Display: display,
	}
}
