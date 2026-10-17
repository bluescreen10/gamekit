// Package gtk provides a GTK 4 implementation of ui.Backend for Linux.
//
// An import of this package registers the backend as "gtk". Linux builds
// require cgo and the development files of GTK 4.14 or later.
//
// The backend implements the complete ui contract with GTK widgets. It
// targets GTK's X11 implementation, so Window and Canvas can be passed
// directly to the Vulkan backend as Xlib surface targets; under Wayland, GTK
// runs through Xwayland.
//
// Some features use other parts of the Linux desktop:
//
//   - Notifications use the freedesktop notification service, and the badge
//     uses the Unity launcher API.
//   - The system tray item is a StatusNotifierItem. It shows only while a
//     tray host runs, such as the KDE panel or the AppIndicator extension of
//     GNOME Shell.
//   - Web views load WebKitGTK 6 when the first one is created, so WebKitGTK
//     is a run-time dependency, not a build dependency.
//   - Video uses the GStreamer media backend of GTK.
//
// Notifications and system tray items communicate over the per-user D-Bus
// session bus. A normal graphical desktop starts this bus automatically.
// Headless environments must provide one, for example by launching the
// application with dbus-run-session. Notifications additionally require an
// org.freedesktop.Notifications service; D-Bus alone does not display them.
// Basic windows, widgets, and Vulkan rendering do not require D-Bus.
//
// GTK must run on one thread: the thread that creates the first window. The
// package locks the main goroutine to the main thread when it initializes.
package gtk
