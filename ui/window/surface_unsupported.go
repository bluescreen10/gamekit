//go:build !cgo || !linux

package window

import "github.com/bluescreen10/gamekit/gpu"

func (w *Window) NativeSurface() gpu.NativeSurface {
	return gpu.NativeSurface{}
}
