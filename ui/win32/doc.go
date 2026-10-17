// Package win32 provides a Win32 implementation of ui.Backend for Windows.
//
// An import of this package registers the backend as "win32". Windows builds
// require cgo and a Clang C++ compiler that targets MinGW, such as
// llvm-mingw, because the native layer is written in C++.
//
// The backend implements the complete ui contract with the native controls
// of Windows: the common controls version 6 with visual styles, in the light
// or dark theme of the system. Windows has no control for a few views, such
// as the dial, the activity spinner, and badges, so the backend draws them
// with Direct2D in the style of Windows. Window and Canvas are Win32 windows
// (HWND), so they can be passed directly to the Vulkan backend as Win32
// surface targets.
//
// The backend uses only components that are part of Windows, so an
// application is a single executable that needs nothing installed:
//
//   - File dialogs are the common item dialogs of Windows, alerts are task
//     dialogs, and printing uses the print dialog and GDI.
//   - The clipboard is the Win32 clipboard, and images are encoded and
//     decoded with the Windows Imaging Component.
//   - Rich text and Markdown are rich edit controls, SVG images are drawn by
//     Direct2D, and videos play with Media Foundation.
//   - Notifications are toast notifications. The backend registers an
//     application user model ID for the executable under
//     HKEY_CURRENT_USER\Software\Classes\AppUserModelId when an application
//     first uses notifications. The badge is an overlay on the taskbar
//     buttons of the windows.
//   - The system tray item is an icon in the notification area of the
//     taskbar.
//   - Sharing shows the share sheet of Windows.
//   - Web views use the WebView2 Runtime that Windows 11 and current
//     versions of Windows 10 include. The backend loads it from where Windows
//     installs it, so the application does not ship the WebView2 loader. On
//     a system without the runtime, a web view stays empty.
//
// Windows delivers the messages of a window to the thread that created it.
// The package locks the main goroutine to the main thread when it
// initializes.
package win32
