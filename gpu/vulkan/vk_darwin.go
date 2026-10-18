//go:build darwin && cgo && gamekit_vulkan

package vulkan

/*
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#define VK_USE_PLATFORM_METAL_EXT
#include <vulkan/vulkan.h>
*/
import "C"

// Platform declarations; see vk.go for the naming convention.

type (
	vkMetalSurfaceCreateInfoEXT = C.VkMetalSurfaceCreateInfoEXT
)

const (
	vkStructureTypeMetalSurfaceCreateInfoEXT = C.VK_STRUCTURE_TYPE_METAL_SURFACE_CREATE_INFO_EXT
)

func vkCreateMetalSurfaceEXT(instance vkInstance, pCreateInfo *vkMetalSurfaceCreateInfoEXT, pAllocator *vkAllocationCallbacks, pSurface *vkSurfaceKHR) vkResult {
	return C.vkCreateMetalSurfaceEXT(instance, pCreateInfo, pAllocator, pSurface)
}
