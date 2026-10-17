// Package window provides GameKit's built-in X11 backend for ui.
//
// The backend provides windows and input on Linux only. It does not have
// native widgets. Applications that need native widgets should use package
// gtk on Linux, package win32 on Windows, and package cocoa on macOS.
// Importing this package on Linux registers the backend as "x11".
// The zero value of Backend is ready for use.
package window
