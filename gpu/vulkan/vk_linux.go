//go:build linux && cgo

package vulkan

/*
#define VK_USE_PLATFORM_XLIB_KHR
#include <vulkan/vulkan.h>
*/
import "C"

// Platform declarations; see vk.go for the naming convention.

type (
	cWindow                    = C.Window
	vkXlibSurfaceCreateInfoKHR = C.VkXlibSurfaceCreateInfoKHR
)

const (
	vkStructureTypeXlibSurfaceCreateInfoKHR = C.VK_STRUCTURE_TYPE_XLIB_SURFACE_CREATE_INFO_KHR
)

func vkCreateXlibSurfaceKHR(instance vkInstance, pCreateInfo *vkXlibSurfaceCreateInfoKHR, pAllocator *vkAllocationCallbacks, pSurface *vkSurfaceKHR) vkResult {
	return C.vkCreateXlibSurfaceKHR(instance, pCreateInfo, pAllocator, pSurface)
}
