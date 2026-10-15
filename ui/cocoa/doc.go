// Package cocoa provides an AppKit implementation of ui.Backend for macOS.
//
// An import of this package registers the backend as "cocoa". macOS builds
// require cgo. The zero value of Backend is ready for use.
//
// The backend implements GameKit's native widget set with AppKit controls.
// Window and Canvas expose NSWindow and NSView surfaces that can be passed
// directly to the Metal and Vulkan GPU backends.
package cocoa
