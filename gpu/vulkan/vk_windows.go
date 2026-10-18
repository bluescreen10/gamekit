//go:build windows && cgo

package vulkan

/*
#define VK_USE_PLATFORM_WIN32_KHR
#include <vulkan/vulkan.h>
*/
import "C"

// Platform declarations; see vk.go for the naming convention.

type (
	cHmodule                    = C.HMODULE
	cWchar                      = C.WCHAR
	vkWin32SurfaceCreateInfoKHR = C.VkWin32SurfaceCreateInfoKHR
)

const (
	vkStructureTypeWIN32SurfaceCreateInfoKHR = C.VK_STRUCTURE_TYPE_WIN32_SURFACE_CREATE_INFO_KHR
)

func vkCreateWin32SurfaceKHR(instance vkInstance, pCreateInfo *vkWin32SurfaceCreateInfoKHR, pAllocator *vkAllocationCallbacks, pSurface *vkSurfaceKHR) vkResult {
	return C.vkCreateWin32SurfaceKHR(instance, pCreateInfo, pAllocator, pSurface)
}

// getModuleHandleW is the Win32 GetModuleHandleW, for the surface's instance handle.
func getModuleHandleW(lpModuleName *cWchar) cHmodule {
	return C.GetModuleHandleW(lpModuleName)
}
