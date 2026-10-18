//go:build darwin && cgo && gamekit_vulkan

package vulkan

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/metal"
)

// MetalSurfaceExtensions are the instance extensions needed for a Metal surface;
// pass them to New when creating an on-screen backend on macOS.
var MetalSurfaceExtensions = []string{"VK_KHR_surface", "VK_EXT_metal_surface"}

// CreateSurface creates a Vulkan Metal surface for a Cocoa window or view.
// The Vulkan instance must have been created with MetalSurfaceExtensions.
func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("vulkan: nil surface target")
	}
	native := target.NativeSurface()
	var layer uintptr
	switch native.Kind {
	case gpu.NativeSurfaceCocoaWindow:
		layer = metal.CocoaWindowMetalLayer(native.Handle)
	case gpu.NativeSurfaceCocoaView:
		layer = metal.CocoaViewMetalLayer(native.Handle)
	default:
		return 0, fmt.Errorf("vulkan: unsupported native surface kind %d", native.Kind)
	}
	if layer == 0 {
		return 0, errors.New("vulkan: surface target is closed or invalid")
	}
	ci := vkMetalSurfaceCreateInfoEXT{sType: vkStructureTypeMetalSurfaceCreateInfoEXT}
	// The layer is a CAMetalLayer pointer, never Go memory; storing its bits keeps vet
	// from flagging a uintptr turned pointer.
	*(*uintptr)(unsafe.Pointer(&ci.pLayer)) = layer
	var surface vkSurfaceKHR
	if r := vkCreateMetalSurfaceEXT(b.instance, &ci, nil, &surface); r != vkSuccess {
		return 0, fmt.Errorf("vulkan: vkCreateMetalSurfaceEXT failed (%d)", int(r))
	}
	return surfaceHandle(surface), nil
}
