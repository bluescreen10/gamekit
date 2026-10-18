//go:build windows && cgo

package vulkan

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// CreateSurface creates a Vulkan Win32 surface for a native target.
func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("vulkan: nil surface target")
	}
	native := target.NativeSurface()
	if native.Kind != gpu.NativeSurfaceWin32Window {
		return 0, fmt.Errorf("vulkan: unsupported native surface kind %d", native.Kind)
	}
	if native.Handle == 0 {
		return 0, errors.New("vulkan: surface target is closed or invalid")
	}
	ci := vkWin32SurfaceCreateInfoKHR{
		sType:     vkStructureTypeWIN32SurfaceCreateInfoKHR,
		hinstance: getModuleHandleW(nil),
	}
	// The window is an HWND, never Go memory; storing its bits keeps vet from flagging a
	// uintptr turned pointer.
	*(*uintptr)(unsafe.Pointer(&ci.hwnd)) = native.Handle
	var surface vkSurfaceKHR
	if r := vkCreateWin32SurfaceKHR(b.instance, &ci, nil, &surface); r != vkSuccess {
		return 0, fmt.Errorf("vulkan: vkCreateWin32SurfaceKHR failed (%d)", int(r))
	}
	return surfaceHandle(surface), nil
}
