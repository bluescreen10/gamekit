// Package window provides GameKit's built-in Win32 and X11 backends for ui.
//
// It uses Win32 on Windows and X11 on Linux. Win32 implements the native
// widget set; the X11 path provides windows and input only. Applications that
// need native Linux widgets should use package gtk, and macOS applications
// should use package cocoa. Win32 is being replaced by package winui.
// Importing this package registers "win32" or "x11" for the target platform.
// The zero value of Backend is ready for use.
package window
