//go:build linux && cgo

package gtk

import (
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

func showNativeAbout(options ui.AboutOptions) {
	widget := gtkAboutDialogNew()
	dialog := (*gtkAboutDialog)(unsafe.Pointer(widget))
	name := gGetApplicationName()
	if options.Name != "" {
		name = stringPointer(options.Name)
	}
	gtkAboutDialogSetProgramName(dialog, name)
	if options.Version != "" {
		gtkAboutDialogSetVersion(dialog, stringPointer(options.Version))
	}
	if options.Copyright != "" {
		gtkAboutDialogSetCopyright(dialog, stringPointer(options.Copyright))
	}
	if options.Website != "" {
		gtkAboutDialogSetWebsite(dialog, stringPointer(options.Website))
	}
	if options.Credits != "" {
		gtkAboutDialogSetComments(dialog, stringPointer(options.Credits))
	}
	if icon := nativeIcon(options.Icon, 64); icon != nil {
		gtkAboutDialogSetLogo(dialog, icon)
		gObjectUnref(unsafe.Pointer(icon))
	}
	// GTK owns the top-level window until it is closed and destroyed.
	window := (*gtkWindow)(unsafe.Pointer(widget))
	gtkWindowSetHideOnClose(window, 0)
	gtkWindowPresent(window)
}
