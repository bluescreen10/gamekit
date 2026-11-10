//go:build linux && cgo

package gtk

import (
	"html"
	"sync"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

var severityStylesOnce sync.Once

func severityClass(severity int) string {
	switch severity {
	case 1:
		return "gk-success"
	case 2:
		return "gk-warning"
	case 3:
		return "gk-error"
	default:
		return "gk-info"
	}
}
func severityStyles() {
	severityStylesOnce.Do(func() {
		setComponentCSS(".gk-info-bar", "border-radius: 6px; padding: 6px 8px; border: 1px solid;")
		for i, color := range []string{"@theme_selected_bg_color", "@success_color", "@warning_color", "@error_color"} {
			class := severityClass(i)
			setComponentCSS(".gk-info-bar."+class, "background-color: alpha("+color+", 0.12); border-color: alpha("+color+", 0.45);")
			setComponentCSS("image."+class+", label."+class, "color: "+color+";")
			setComponentCSS("label.gk-pill."+class, "background-color: "+color+"; color: white;")
		}
		setComponentCSS("label.gk-pill", "border-radius: 9px; padding: 0 6px; min-width: 6px; min-height: 18px; font-size: smaller; font-weight: bold; color: white;")
		setComponentCSS(".gk-toast", "border-radius: 10px; padding: 6px 12px;")
	})
}
func setSeverityClass(widget *gtkWidget, severity int) {
	for i := range 4 {
		gtkWidgetRemoveCSSClass(widget, stringPointer(severityClass(i)))
	}
	gtkWidgetAddCSSClass(widget, stringPointer(severityClass(severity)))
}
func appendActionButtons(box *gtkWidget, ids *cChar, idsSize cSize, titles *cChar, titlesSize cSize, enabled *cInt, count cInt, activate func(string)) {
	identifiers, names := splitNul(unsafe.Pointer(ids), idsSize), splitNul(unsafe.Pointer(titles), titlesSize)
	flags := unsafe.Slice(enabled, int(count))
	for i := 0; i < int(count) && i < len(identifiers) && i < len(names); i++ {
		identifier := identifiers[i]
		button := gtkButtonNewWithLabel(stringPointer(names[i]))
		gtkWidgetSetValign(button, gtkAlignCenter)
		gtkWidgetSetSensitive(button, nativeBool(flags[i] != 0))
		connectNativeSignal(unsafe.Pointer(button), "clicked", func() { activate(identifier) })
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), button)
	}
}

type nativeInfoBar struct{ icon, message *gtkWidget }

func gkGTKInfoBarCreate(window cUintptr, closable cInt, ids *cChar, idsSize cSize, titles *cChar, titlesSize cSize, enabled *cInt, count cInt) unsafe.Pointer {
	severityStyles()
	box := boxWithRole(gtkOrientationHorizontal, 8, gtkAccessibleRoleAlert)
	gtkWidgetAddCSSClass(box, stringPointer("gk-info-bar"))
	bar := &nativeInfoBar{icon: gtkImageNew(), message: gtkLabelNew(stringPointer(""))}
	gtkWidgetSetValign(bar.icon, gtkAlignCenter)
	gtkLabelSetWrap((*gtkLabel)(unsafe.Pointer(bar.message)), 1)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(bar.message)), 0)
	gtkWidgetSetHexpand(bar.message, 1)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), bar.icon)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), bar.message)
	registerNativeComponent(uintptr(window), box, bar.message, kindInfoBar)
	storeComponentState(box, bar)
	appendActionButtons(box, ids, idsSize, titles, titlesSize, enabled, count, func(id string) { sendComponentEvent(box, gtkEvent{kind: gkEventAction, text: id}) })
	if closable != 0 {
		button := gtkButtonNewFromIconName(stringPointer("window-close-symbolic"))
		gtkWidgetAddCSSClass(button, stringPointer("flat"))
		gtkWidgetSetValign(button, gtkAlignCenter)
		setAccessibleText(button, gtkAccessiblePropertyLabel, "Close")
		connectNativeSignal(unsafe.Pointer(button), "clicked", func() { sendComponentEvent(box, gtkEvent{kind: gkEventClose}) })
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), button)
	}
	return unsafe.Pointer(box)
}
func gkGTKInfoBarSet(pointer unsafe.Pointer, severity cInt, title, detail *cChar) {
	bar := componentState[nativeInfoBar](pointer)
	if bar == nil {
		return
	}
	widget := (*gtkWidget)(pointer)
	gtkImageSetFromIconName((*gtkImage)(unsafe.Pointer(bar.icon)), stringPointer(alertIconName(ui.Severity(severity))))
	setSeverityClass(bar.icon, int(severity))
	setSeverityClass(widget, int(severity))
	markup := "<b>" + html.EscapeString(goString(title)) + "</b>"
	if text := goString(detail); text != "" {
		markup += "  " + html.EscapeString(text)
	}
	gtkLabelSetMarkup((*gtkLabel)(unsafe.Pointer(bar.message)), stringPointer(markup))
	text := goString(title)
	if goString(detail) != "" {
		text += " " + goString(detail)
	}
	setAccessibleText(widget, gtkAccessiblePropertyLabel, text)
}
func gkGTKBadgeCreate(window cUintptr) unsafe.Pointer {
	severityStyles()
	label := widgetWithRole(gtkLabelGetType(), gtkAccessibleRoleStatus)
	gtkWidgetAddCSSClass(label, stringPointer("gk-pill"))
	gtkWidgetSetValign(label, gtkAlignCenter)
	gtkWidgetSetHalign(label, gtkAlignCenter)
	registerNativeComponent(uintptr(window), label, nil, kindBadge)
	return unsafe.Pointer(label)
}
func gkGTKBadgeSet(pointer unsafe.Pointer, text *cChar, severity cInt, accessible *cChar) {
	widget := (*gtkWidget)(pointer)
	gtkLabelSetText((*gtkLabel)(pointer), text)
	setSeverityClass(widget, int(severity))
	value := goString(accessible)
	if value == "" {
		value = goString(text)
	}
	setAccessibleText(widget, gtkAccessiblePropertyValueText, value)
}

type nativeValidation struct{ icon, label *gtkWidget }

func gkGTKInlineValidationCreate(window cUintptr) unsafe.Pointer {
	severityStyles()
	box := gtkBoxNew(gtkOrientationHorizontal, 4)
	state := &nativeValidation{icon: gtkImageNew(), label: gtkLabelNew(stringPointer(""))}
	gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(state.icon)), 14)
	gtkWidgetSetValign(state.icon, gtkAlignStart)
	gtkLabelSetWrap((*gtkLabel)(unsafe.Pointer(state.label)), 1)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(state.label)), 0)
	gtkWidgetAddCSSClass(state.label, stringPointer("caption"))
	gtkWidgetSetHexpand(state.label, 1)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), state.icon)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), state.label)
	registerNativeComponent(uintptr(window), box, state.label, kindValidation)
	storeComponentState(box, state)
	return unsafe.Pointer(box)
}
func gkGTKInlineValidationSet(pointer unsafe.Pointer, severity cInt, text *cChar) {
	state := componentState[nativeValidation](pointer)
	if state == nil {
		return
	}
	gtkImageSetFromIconName((*gtkImage)(unsafe.Pointer(state.icon)), stringPointer(alertIconName(ui.Severity(severity))))
	setSeverityClass(state.icon, int(severity))
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.label)), text)
	for i := range 4 {
		gtkWidgetRemoveCSSClass(state.label, stringPointer(severityClass(i)))
	}
	if severity >= 2 {
		gtkWidgetAddCSSClass(state.label, stringPointer(severityClass(int(severity))))
		gtkWidgetRemoveCSSClass(state.label, stringPointer("dim-label"))
	} else {
		gtkWidgetAddCSSClass(state.label, stringPointer("dim-label"))
	}
}

type nativeEmptyState struct{ title, detail *gtkWidget }

func gkGTKEmptyStateCreate(window cUintptr, icon unsafe.Pointer, ids *cChar, idsSize cSize, titles *cChar, titlesSize cSize, enabled *cInt, count cInt) unsafe.Pointer {
	box := boxWithRole(gtkOrientationVertical, 8, gtkAccessibleRoleGroup)
	state := &nativeEmptyState{title: gtkLabelNew(stringPointer("")), detail: gtkLabelNew(stringPointer(""))}
	if icon != nil {
		image := gtkImageNewFromPaintable((*gdkPaintable)(icon))
		gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(image)), 48)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), image)
	}
	gtkWidgetAddCSSClass(state.title, stringPointer("title-2"))
	gtkLabelSetWrap((*gtkLabel)(unsafe.Pointer(state.title)), 1)
	gtkLabelSetWrap((*gtkLabel)(unsafe.Pointer(state.detail)), 1)
	gtkWidgetAddCSSClass(state.detail, stringPointer("dim-label"))
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), state.title)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), state.detail)
	registerNativeComponent(uintptr(window), box, nil, kindEmptyState)
	storeComponentState(box, state)
	if count > 0 {
		buttons := gtkBoxNew(gtkOrientationHorizontal, 8)
		gtkWidgetSetHalign(buttons, gtkAlignCenter)
		gtkWidgetSetMarginTop(buttons, 6)
		appendActionButtons(buttons, ids, idsSize, titles, titlesSize, enabled, count, func(id string) { sendComponentEvent(box, gtkEvent{kind: gkEventAction, text: id}) })
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), buttons)
	}
	return unsafe.Pointer(box)
}
func gkGTKEmptyStateSet(pointer unsafe.Pointer, title, detail *cChar) {
	state := componentState[nativeEmptyState](pointer)
	if state == nil {
		return
	}
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.title)), title)
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.detail)), detail)
	gtkWidgetSetVisible(state.detail, nativeBool(goString(detail) != ""))
	setAccessibleText((*gtkWidget)(pointer), gtkAccessiblePropertyLabel, goString(title))
}

type nativeTooltip struct {
	widget, label *gtkWidget
	timer         cUint
}

func newNativeTooltip() *nativeTooltip {
	state := &nativeTooltip{widget: widgetWithRole(gtkPopoverGetType(), gtkAccessibleRoleTooltip), label: gtkLabelNew(stringPointer(""))}
	gObjectRefSink(unsafe.Pointer(state.widget))
	gtkPopoverSetAutohide((*gtkPopover)(unsafe.Pointer(state.widget)), 0)
	gtkPopoverSetHasArrow((*gtkPopover)(unsafe.Pointer(state.widget)), 0)
	gtkWidgetSetCanTarget(state.widget, 0)
	gtkWidgetSetCanFocus(state.widget, 0)
	gtkWidgetAddCSSClass(state.widget, stringPointer("tooltip"))
	gtkPopoverSetChild((*gtkPopover)(unsafe.Pointer(state.widget)), state.label)
	return state
}
func (tooltip *nativeTooltip) hide() {
	if tooltip.timer != 0 {
		gSourceRemove(tooltip.timer)
		tooltip.timer = 0
	}
	gtkPopoverPopdown((*gtkPopover)(unsafe.Pointer(tooltip.widget)))
}
func (tooltip *nativeTooltip) close() { tooltip.hide(); closeNativePopover(tooltip.widget) }
func (tooltip *nativeTooltip) show(anchor *gtkWidget, edge int, delay time.Duration) bool {
	if !anchorPopover(tooltip.widget, anchor, edge) {
		return false
	}
	tooltip.hide()
	if delay <= 0 {
		gtkPopoverPopup((*gtkPopover)(unsafe.Pointer(tooltip.widget)))
	} else {
		tooltip.timer = gTimeoutAddFull(0, cUint(delay.Milliseconds()), gtkSourceCallback(), newNativeCallback(func() bool {
			tooltip.timer = 0
			gtkPopoverPopup((*gtkPopover)(unsafe.Pointer(tooltip.widget)))
			return false
		}), gtkDestroyCallback())
	}
	return true
}

type nativeToast struct {
	widget, message, overlay *gtkWidget
	timer                    cUint
	timeout                  time.Duration
	handle                   uintptr
	closed                   bool
}

func newNativeToast(handle uintptr, message string, actions []ui.Action, timeout time.Duration) *nativeToast {
	severityStyles()
	toast := &nativeToast{
		widget:  boxWithRole(gtkOrientationHorizontal, 8, gtkAccessibleRoleStatus),
		message: gtkLabelNew(stringPointer(message)),
		handle:  handle,
		timeout: timeout,
	}
	gObjectRefSink(unsafe.Pointer(toast.widget))
	gtkWidgetAddCSSClass(toast.widget, stringPointer("osd"))
	gtkWidgetAddCSSClass(toast.widget, stringPointer("gk-toast"))
	gtkWidgetSetHalign(toast.widget, gtkAlignCenter)
	gtkWidgetSetValign(toast.widget, gtkAlignEnd)
	gtkWidgetSetMarginBottom(toast.widget, 20)
	gtkLabelSetEllipsize((*gtkLabel)(unsafe.Pointer(toast.message)), pangoEllipsizeEnd)
	gtkLabelSetMaxWidthChars((*gtkLabel)(unsafe.Pointer(toast.message)), 60)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(toast.widget)), toast.message)
	for _, action := range actions {
		button := gtkButtonNewWithLabel(stringPointer(action.Text))
		gtkWidgetSetSensitive(button, nativeBool(action.Enabled))
		connectNativeSignal(unsafe.Pointer(button), "clicked", func() {
			if !toast.closed {
				dispatchNativeEvent(toast.handle, gtkEvent{kind: gkEventAction, text: action.ID})
				toast.dismiss()
			}
		})
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(toast.widget)), button)
	}
	setAccessibleText(toast.widget, gtkAccessiblePropertyLabel, message)
	return toast
}
func (toast *nativeToast) dismiss() {
	if toast.timer != 0 {
		gSourceRemove(toast.timer)
		toast.timer = 0
	}
	if toast.overlay != nil {
		gtkOverlayRemoveOverlay((*gtkOverlay)(unsafe.Pointer(toast.overlay)), toast.widget)
		toast.overlay = nil
	}
}
func (toast *nativeToast) close() {
	toast.closed = true
	toast.dismiss()
	gObjectUnref(unsafe.Pointer(toast.widget))
}
func (toast *nativeToast) show(window *gtkWindow) {
	toast.dismiss()
	toast.overlay = gtkWindowGetChild(window)
	gtkOverlayAddOverlay((*gtkOverlay)(unsafe.Pointer(toast.overlay)), toast.widget)
	if toast.timeout > 0 {
		toast.timer = gTimeoutAddFull(0, cUint(toast.timeout.Milliseconds()), gtkSourceCallback(), newNativeCallback(func() bool { toast.timer = 0; toast.dismiss(); return false }), gtkDestroyCallback())
	}
	announceComponent(toast.widget, goString(gtkLabelGetText((*gtkLabel)(unsafe.Pointer(toast.message)))), 1)
}

type nativeProgressDialog struct {
	window            *gtkWindow
	message, progress *gtkWidget
	indeterminate     bool
	last              int64
	handle            uintptr
	closed            bool
}

func newNativeProgressDialog(handle uintptr, title string, cancellable bool) *nativeProgressDialog {
	state := &nativeProgressDialog{window: (*gtkWindow)(unsafe.Pointer(gtkWindowNew())), message: gtkLabelNew(stringPointer("")), progress: gtkProgressBarNew(), handle: handle}
	gObjectRefSink(unsafe.Pointer(state.window))
	gtkWindowSetTitle(state.window, stringPointer(title))
	gtkWindowSetModal(state.window, 1)
	gtkWindowSetResizable(state.window, 0)
	gtkWindowSetDefaultSize(state.window, 360, -1)
	gtkWindowSetDeletable(state.window, 0)
	connectNativeCloseRequest(state.window, func() bool { return true })
	box := gtkBoxNew(gtkOrientationVertical, 8)
	gtkWidgetSetMarginStart(box, 20)
	gtkWidgetSetMarginEnd(box, 20)
	gtkWidgetSetMarginTop(box, 16)
	gtkWidgetSetMarginBottom(box, 12)
	heading := gtkLabelNew(stringPointer(title))
	gtkWidgetAddCSSClass(heading, stringPointer("heading"))
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(heading)), 0)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(state.message)), 0)
	gtkLabelSetEllipsize((*gtkLabel)(unsafe.Pointer(state.message)), pangoEllipsizeEnd)
	for _, child := range []*gtkWidget{heading, state.message, state.progress} {
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), child)
	}
	if cancellable {
		button := gtkButtonNewWithLabel(stringPointer("Cancel"))
		installNativeActivationShortcut(button, "Escape")
		gtkWidgetSetHalign(button, gtkAlignEnd)
		connectNativeSignal(unsafe.Pointer(button), "clicked", func() {
			if !state.closed {
				dispatchNativeEvent(state.handle, gtkEvent{kind: gkEventClose})
			}
		})
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), button)
	}
	gtkWindowSetChild(state.window, box)
	gtkWidgetAddTickCallback(state.progress, gtkTickCallbackAddress(), newNativeCallback(func(clock *gdkFrameClock) bool {
		now := int64(gdkFrameClockGetFrameTime(clock))
		if state.indeterminate && now-state.last > 100000 {
			state.last = now
			gtkProgressBarPulse((*gtkProgressBar)(unsafe.Pointer(state.progress)))
		}
		return true
	}), gtkDestroyCallback())
	return state
}
func (state *nativeProgressDialog) set(message string, value float64) {
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.message)), stringPointer(message))
	state.indeterminate = value < 0
	if value >= 0 {
		gtkProgressBarSetFraction((*gtkProgressBar)(unsafe.Pointer(state.progress)), cDouble(min(1, value)))
	}
}
func (state *nativeProgressDialog) show(parent *gtkWindow) {
	if parent != nil && gtkWidgetGetVisible((*gtkWidget)(unsafe.Pointer(parent))) != 0 {
		gtkWindowSetTransientFor(state.window, parent)
	}
	gtkWindowPresent(state.window)
}
func (state *nativeProgressDialog) close() {
	state.closed = true
	gtkWindowDestroy(state.window)
	gObjectUnref(unsafe.Pointer(state.window))
}
