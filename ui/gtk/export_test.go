//go:build linux && cgo

package gtk

import (
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// CanBecomeKey reports whether GTK lets window take keyboard focus.
func CanBecomeKey(window ui.Window) bool {
	native, ok := window.(*Window)
	return ok && native.canBecomeKey()
}

// ExportPrint exercises the print callbacks without opening a print dialog or
// requiring a printer. The caller must run on GTK's main thread.
func ExportPrint(options ui.PrintOptions, path string) (int, error) {
	job := &printJob{options: options}
	operation := gtkPrintOperationNew()
	defer gObjectUnref(unsafe.Pointer(operation))
	gtkPrintOperationSetUnit(operation, gtkUnitPoints)
	gtkPrintOperationSetExportFilename(operation, stringPointer(path))
	pages := executeNativePrint(job, operation, gtkPrintOperationActionExport, nil)
	return pages, job.err
}

// ScheduleNativeResponse clicks a real GTK button after a dialog or popover
// becomes visible. The caller must run it and its cleanup on GTK's main thread.
func ScheduleNativeResponse(title, button, text string, closeWindow bool) func() {
	var id cUint
	deadline := time.Now().Add(2 * time.Second)
	id = gIdleAddFull(200, gtkSourceCallback(), newNativeCallback(func() bool {
		windows := gtkWindowListToplevels()
		defer gListFree(windows)
		for item := windows; item != nil; item = item.next {
			window := (*gtkWindow)(item.data)
			widget := (*gtkWidget)(unsafe.Pointer(window))
			if goString(gtkWindowGetTitle(window)) != title || gtkWidgetGetVisible(widget) == 0 {
				continue
			}
			if closeWindow {
				gtkWindowClose(window)
				id = 0
				return false
			}
			var visit func(*gtkWidget) *gtkWidget
			visit = func(widget *gtkWidget) *gtkWidget {
				name := goString(gtkWidgetGetName(widget))
				if name == "GtkEntry" || name == "GtkPasswordEntry" {
					gtkEditableSetText((*gtkEditable)(unsafe.Pointer(widget)), stringPointer(text))
				}
				if name == "GtkLabel" && goString(gtkLabelGetText((*gtkLabel)(unsafe.Pointer(widget)))) == button {
					return widget
				}
				for child := gtkWidgetGetFirstChild(widget); child != nil; child = gtkWidgetGetNextSibling(child) {
					if found := visit(child); found != nil {
						return found
					}
				}
				return nil
			}
			if label := visit(widget); label != nil {
				// A label can be wrapped in the share button's icon row.
				for parent := gtkWidgetGetParent(label); parent != nil; parent = gtkWidgetGetParent(parent) {
					if goString(gtkWidgetGetName(parent)) == "GtkButton" {
						gtkWidgetActivate(parent)
						id = 0
						return false
					}
				}
			}
		}
		if time.Now().After(deadline) {
			id = 0
			return false
		}
		return true
	}), gtkDestroyCallback())
	return func() {
		if id != 0 {
			gSourceRemove(id)
		}
	}
}
