//go:build darwin && cgo

package metal

import (
	"errors"
	"fmt"

	"github.com/bluescreen10/gamekit/gpu"
)

// CocoaViewMetalLayer installs and returns a CAMetalLayer on a Cocoa view.
// It is exported for the Vulkan backend's VK_EXT_metal_surface path.
func CocoaViewMetalLayer(view uintptr) uintptr {
	if view == 0 {
		return 0
	}
	pool := pushPool()
	defer pool.drain()
	v := id(view)
	layer := viewLayer(v)
	if !objectIsKindOfClass(layer, class("CAMetalLayer")) {
		viewSetWantsLayer(v, true)
		layer = metalLayerLayer()
		viewSetLayer(v, layer)
	}
	scale := float64(1)
	if window := viewWindow(v); window != 0 {
		scale = windowBackingScaleFactor(window)
	}
	metalLayerSetContentsScale(layer, scale)
	_, _, width, height := viewBounds(v)
	metalLayerSetDrawableSize(layer, width*scale, height*scale)
	return uintptr(layer)
}

// CocoaWindowMetalLayer installs and returns a CAMetalLayer on a Cocoa window's
// content view. It is exported for the Vulkan backend's Metal surface path.
func CocoaWindowMetalLayer(window uintptr) uintptr {
	if window == 0 {
		return 0
	}
	return CocoaViewMetalLayer(uintptr(windowContentView(id(window))))
}

// CreateSurface creates a CAMetalLayer presentation surface for a native target.
func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("metal: nil surface target")
	}
	native := target.NativeSurface()
	var layer uintptr
	switch native.Kind {
	case gpu.NativeSurfaceCocoaWindow:
		layer = CocoaWindowMetalLayer(native.Handle)
	case gpu.NativeSurfaceCocoaView:
		layer = CocoaViewMetalLayer(native.Handle)
	default:
		return 0, fmt.Errorf("metal: unsupported native surface kind %d", native.Kind)
	}
	if layer == 0 {
		return 0, errors.New("metal: surface target is closed or invalid")
	}
	return gpu.Surface(layer), nil
}
