// Package defaultgpu registers the default GPU backend for the target platform.
// Import this package for its side effect:
//
//	import _ "github.com/bluescreen10/gamekit/gpu/default"
//
// The default is Metal on macOS and Vulkan on Linux and Windows. Both require cgo.
// Without cgo, or on other platforms, this package registers no backend.
//
// On macOS, the gamekit_vulkan build tag also registers Vulkan. Its higher
// registry priority makes it the default in that build. Explicit selection can
// still use either backend. Normal macOS builds need no Vulkan SDK.
// Import does not initialize a device. Call Init on the backend returned by gpu.Instance.
package defaultgpu
