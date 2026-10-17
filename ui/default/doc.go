// Package defaultui registers the default native UI backend for the target
// platform. Import this package for its side effect.
//
// The package selects Cocoa on macOS, Win32 on Windows, and GTK 4 on Linux.
// The Windows default requires cgo, a Clang compiler for MinGW, and the
// Windows App Runtime. The Linux default requires cgo and GTK 4 development
// files.
package defaultui
