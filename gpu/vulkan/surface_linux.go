//go:build linux && cgo

package vulkan

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// CreateSurface creates a Vulkan Xlib surface for a native target.
func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("vulkan: nil surface target")
	}
	native := target.NativeSurface()
	if native.Kind != gpu.NativeSurfaceXlibWindow {
		return 0, fmt.Errorf("vulkan: unsupported native surface kind %d", native.Kind)
	}
	if native.Display == 0 || native.Handle == 0 {
		return 0, errors.New("vulkan: surface target is closed or invalid")
	}
	ci := vkXlibSurfaceCreateInfoKHR{
		sType:  vkStructureTypeXlibSurfaceCreateInfoKHR,
		window: cWindow(native.Handle),
	}
	// The display is an Xlib pointer, never Go memory; storing its bits keeps vet from
	// flagging a uintptr turned pointer.
	*(*uintptr)(unsafe.Pointer(&ci.dpy)) = native.Display
	var surface vkSurfaceKHR
	if r := vkCreateXlibSurfaceKHR(b.instance, &ci, nil, &surface); r != vkSuccess {
		return 0, fmt.Errorf("vulkan: vkCreateXlibSurfaceKHR failed (%d)", int(r))
	}
	return surfaceHandle(surface), nil
}
