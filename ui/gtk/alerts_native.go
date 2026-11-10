//go:build linux && cgo

package gtk

import (
	"context"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

type nativeAlert struct {
	window   *gtkWindow
	entry    *gtkWidget
	response int
}

func newAlert(severity ui.Severity, title, message, details string, buttons []string, cancel int,
	destructive []bool, prompt *ui.PromptOptions) *nativeAlert {
	var role gValue
	gValueInit(&role, gtkAccessibleRoleGetType())
	gValueSetEnum(&role, gtkAccessibleRoleAlertDialog)
	properties, free := nativeStringVector([]string{"accessible-role"})
	object := gObjectNewWithProperties(gtkWindowGetType(), 1, properties, &role)
	free()
	gValueUnset(&role)
	alert := &nativeAlert{window: (*gtkWindow)(unsafe.Pointer(object)), response: -1}
	gObjectRefSink(unsafe.Pointer(object))
	gtkWindowSetModal(alert.window, 1)
	gtkWindowSetResizable(alert.window, 0)
	gtkWindowSetTitle(alert.window, stringPointer(title))
	connectNativeCloseRequest(alert.window, func() bool {
		alert.response = max(cancel, 0)
		return true
	})
	box := gtkBoxNew(gtkOrientationVertical, 10)
	gtkWidgetSetMarginStart(box, 24)
	gtkWidgetSetMarginEnd(box, 24)
	gtkWidgetSetMarginTop(box, 20)
	gtkWidgetSetMarginBottom(box, 16)
	gtkWidgetSetSizeRequest(box, 320, -1)
	appendChild := func(child *gtkWidget) { gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), child) }
	icon := gtkImageNewFromIconName(stringPointer(alertIconName(severity)))
	gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(icon)), 48)
	appendChild(icon)
	heading := gtkLabelNew(stringPointer(title))
	gtkWidgetAddCSSClass(heading, stringPointer("title-3"))
	gtkLabelSetWrap((*gtkLabel)(unsafe.Pointer(heading)), 1)
	gtkLabelSetMaxWidthChars((*gtkLabel)(unsafe.Pointer(heading)), 40)
	appendChild(heading)
	if message != "" {
		body := gtkLabelNew(stringPointer(message))
		gtkLabelSetWrap((*gtkLabel)(unsafe.Pointer(body)), 1)
		gtkLabelSetMaxWidthChars((*gtkLabel)(unsafe.Pointer(body)), 50)
		gtkLabelSetJustify((*gtkLabel)(unsafe.Pointer(body)), gtkJustifyCenter)
		appendChild(body)
		var description gValue
		gValueInit(&description, gTypeString)
		gValueSetString(&description, stringPointer(message))
		property := gtkAccessibleProperty(gtkAccessiblePropertyDescription)
		gtkAccessibleUpdatePropertyValue((*gtkAccessible)(unsafe.Pointer(alert.window)), 1, &property, &description)
		gValueUnset(&description)
	}
	if details != "" {
		expander := gtkExpanderNew(stringPointer("Details"))
		label := gtkLabelNew(stringPointer(details))
		gtkLabelSetWrap((*gtkLabel)(unsafe.Pointer(label)), 1)
		gtkLabelSetSelectable((*gtkLabel)(unsafe.Pointer(label)), 1)
		gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(label)), 0)
		gtkWidgetAddCSSClass(label, stringPointer("dim-label"))
		gtkExpanderSetChild((*gtkExpander)(unsafe.Pointer(expander)), label)
		appendChild(expander)
	}
	if prompt != nil {
		if prompt.Secure {
			alert.entry = gtkPasswordEntryNew()
		} else {
			alert.entry = gtkEntryNew()
		}
		gtkEditableSetText((*gtkEditable)(unsafe.Pointer(alert.entry)), stringPointer(prompt.Text))
		var placeholder gValue
		gValueInit(&placeholder, gTypeString)
		gValueSetString(&placeholder, stringPointer(prompt.Placeholder))
		gObjectSetProperty((*gObject)(unsafe.Pointer(alert.entry)), stringPointer("placeholder-text"), &placeholder)
		gValueUnset(&placeholder)
		connectNativeSignal(unsafe.Pointer(alert.entry), "activate", func() { alert.response = 0 })
		appendChild(alert.entry)
	}
	row := gtkBoxNew(gtkOrientationHorizontal, 8)
	gtkWidgetSetHalign(row, gtkAlignEnd)
	gtkWidgetSetMarginTop(row, 6)
	for i := len(buttons) - 1; i >= 0; i-- {
		button := gtkButtonNewWithLabel(stringPointer(buttons[i]))
		if i < len(destructive) && destructive[i] {
			gtkWidgetAddCSSClass(button, stringPointer("destructive-action"))
		} else if i == 0 {
			gtkWidgetAddCSSClass(button, stringPointer("suggested-action"))
		}
		// Each button retains its own index until GTK destroys its signal.
		index := i
		connectNativeSignal(unsafe.Pointer(button), "clicked", func() { alert.response = index })
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), button)
		if i == 0 {
			gtkWindowSetDefaultWidget(alert.window, button)
		}
	}
	appendChild(row)
	gtkWindowSetChild(alert.window, box)
	if cancel >= 0 {
		controller := gtkShortcutControllerNew()
		shortcuts := (*gtkShortcutController)(unsafe.Pointer(controller))
		gtkShortcutControllerSetScope(shortcuts, gtkShortcutScopeGlobal)
		gtkShortcutControllerAddShortcut(shortcuts, gtkShortcutNew(gtkKeyvalTriggerNew(gdkKeyEscape, 0), gtkNamedActionNew(stringPointer("window.close"))))
		gtkWidgetAddController((*gtkWidget)(unsafe.Pointer(alert.window)), controller)
	}
	return alert
}

func alertIconName(severity ui.Severity) string {
	switch severity {
	case ui.SeveritySuccess:
		return "object-select-symbolic"
	case ui.SeverityWarning:
		return "dialog-warning-symbolic"
	case ui.SeverityError:
		return "dialog-error-symbolic"
	default:
		return "dialog-information-symbolic"
	}
}

func (alert *nativeAlert) close() {
	gtkWindowDestroy(alert.window)
	gObjectUnref(unsafe.Pointer(alert.window))
}

// runAlert keeps the GTK main loop responsive while the modal dialog is open.
func (w *Window) runAlert(ctx context.Context, alert *nativeAlert) (int, error) {
	if parent := w.nativeGTKWindow(); parent != nil && gtkWidgetGetVisible((*gtkWidget)(unsafe.Pointer(parent))) != 0 {
		gtkWindowSetTransientFor(alert.window, parent)
	}
	gtkWindowPresent(alert.window)
	defer gtkWidgetSetVisible((*gtkWidget)(unsafe.Pointer(alert.window)), 0)
	if alert.entry != nil {
		gtkWidgetGrabFocus(alert.entry)
	}
	for {
		runNativePending()
		if alert.response >= 0 {
			return alert.response, nil
		}
		if err := ctx.Err(); err != nil {
			return -1, err
		}
		time.Sleep(time.Millisecond)
	}
}
