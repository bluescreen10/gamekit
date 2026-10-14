// Package defaultui registers the default native UI backend for the target
// platform. Import this package for its side effect.
//
// The package selects Cocoa on macOS, Win32 on Windows, and GTK 4 on Linux.
// The Linux default requires cgo and GTK 4 development files.
package defaultui
