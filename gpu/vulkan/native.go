//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"runtime"

	"github.com/bluescreen10/gamekit/strings"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// The package calls Vulkan directly through the declarations in vk.go, and builds the
// structs Vulkan takes in Go. Two rules follow from cgo's pointer rules.
// A struct passed by pointer may point at other Go memory only while that memory is
// pinned, so the structs a call chains together are pinned for the call (pinned), or
// live in the backend's scratch, which is pinned for the backend's life. And no C
// helper allocates: strings and arrays are Go memory, pinned the same way.

// noQueueFamily and noMemoryType mark a search that found nothing.
const (
	noQueueFamily = ^uint32(0)
	noMemoryType  = ^uint32(0)
)

// pinned pins v for the life of pin and returns it, so a struct can be built and
// linked into a chain in one expression.
func pinned[T any](pin *runtime.Pinner, v *T) *T {
	pin.Pin(v)
	return v
}

// pinnedCString returns s as a C string pinned by pin, for a struct passed to C.
func pinnedCString(pin *runtime.Pinner, s string) *cChar {
	p := strings.CString(s)
	pin.Pin(p)
	return (*cChar)(unsafe.Pointer(p))
}

// cStrings returns names as an array of C strings, the array and every string pinned.
// It is nil when names is empty.
func cStrings(pin *runtime.Pinner, names []string) **cChar {
	if len(names) == 0 {
		return nil
	}
	ptrs := make([]*cChar, len(names))
	for i, n := range names {
		ptrs[i] = pinnedCString(pin, n)
	}
	pin.Pin(&ptrs[0])
	return &ptrs[0]
}

// bool32 is a Go bool as a VkBool32.
func bool32(v bool) vkBool32 {
	if v {
		return vkTrue
	}
	return vkFalse
}

// surfaceFromHandle and surfaceHandle convert between a gpu.Surface and the
// VkSurfaceKHR it holds. They reinterpret the bits rather than convert, which keeps
// vet from flagging a uintptr turned pointer; the value is a Vulkan handle, never Go
// memory.
func surfaceFromHandle(h gpu.Surface) vkSurfaceKHR {
	return *(*vkSurfaceKHR)(unsafe.Pointer(&h))
}

func surfaceHandle(s vkSurfaceKHR) gpu.Surface {
	return *(*gpu.Surface)(unsafe.Pointer(&s))
}

// scratch holds the structs that recording and submission chain together. The backend
// pins one for its whole life, so building them costs nothing per command. Command
// recording is single-threaded, as the backend's registries are.
type scratch struct {
	colors     [maxColorAttachments]vkRenderingAttachmentInfo
	depth      vkRenderingAttachmentInfo
	rendering  vkRenderingInfo
	image      vkImageMemoryBarrier2
	memory     vkMemoryBarrier2
	dependency vkDependencyInfo
	sets       [1]vkDescriptorSet

	cmdInfo    vkCommandBufferSubmitInfo
	wait       vkSemaphoreSubmitInfo
	signal     vkSemaphoreSubmitInfo
	submit     vkSubmitInfo2
	present    vkPresentInfoKHR
	swapchain  vkSwapchainKHR
	imageIndex cUint32
	renderDone vkSemaphore
}
