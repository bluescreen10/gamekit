// Package gtk provides a GTK 4 implementation of ui.Backend for Linux.
//
// An import of this package registers the backend as "gtk". Linux builds
// require cgo and GTK 4 development files.
//
// The backend implements GameKit's native widget set and targets GTK's X11
// implementation so Window and Canvas can be passed directly to the Vulkan
// backend as Xlib surface targets.
package gtk
