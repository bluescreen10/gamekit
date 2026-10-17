//go:build windows && cgo

package win32

/*
#include "window_bridge.h"
*/
import "C"

import "github.com/bluescreen10/gamekit/gpu"

// NativeSurface describes the window to a GPU backend as a Win32 window.
func (w *Window) NativeSurface() gpu.NativeSurface {
	native := w.nativePointer()
	if native == nil {
		return gpu.NativeSurface{}
	}
	handle := uintptr(C.gkWin32WindowNativeHandle(native))
	if handle == 0 {
		return gpu.NativeSurface{}
	}
	return gpu.NativeSurface{
		Kind:   gpu.NativeSurfaceWin32Window,
		Handle: handle,
	}
}
