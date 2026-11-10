//go:build linux && cgo

package gtk

import "unsafe"

func gkGTKUIDestroy(pointer unsafe.Pointer) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	if state := gObjectGetQdata((*gObject)(pointer), componentStateKey); state != nil {
		if owner, ok := nativeCallback(state).(interface{ disposeNative() }); ok {
			owner.disposeNative()
		}
	}
	if view.canvas != nil {
		view.canvas.close()
	}
	releaseNativeCapture(view.outer)
	removeNativeWidget(view.outer)
	gObjectUnref(pointer)
}
func gkGTKUIClearAction(pointer unsafe.Pointer) {
	if view := viewState(pointer); view != nil {
		view.action = 0
		if view.canvas != nil {
			view.canvas.frames = 0
			releaseNativeCapture(view.outer)
		}
	}
}
func gkGTKUIAttachToWindow(window cUintptr, pointer unsafe.Pointer) {
	if win := nativeWinForHandle(uintptr(window)); win != nil && pointer != nil {
		putNativeLayout(win.root, (*gtkWidget)(pointer))
	}
}
func gkGTKUISetParent(pointer, parent unsafe.Pointer) {
	view, container := viewState(pointer), viewState(parent)
	if view == nil || container == nil {
		return
	}
	if target := container.container(); layoutState(target) != nil {
		putNativeLayout(target, view.outer)
	}
}
func gkGTKUIBringToFront(pointer unsafe.Pointer) {
	if view := viewState(pointer); view != nil {
		if parent := gtkWidgetGetParent(view.outer); parent != nil {
			gtkWidgetInsertBefore(view.outer, parent, nil)
		}
	}
}
func gkGTKUISetFrame(pointer unsafe.Pointer, x, y, width, height cInt) {
	if view := viewState(pointer); view != nil {
		view.x, view.y, view.width, view.height = int(x), int(y), max(0, int(width)), max(0, int(height))
		moveNativeWidget(view.outer, view.x, view.y, view.width, view.height)
		if view.canvas != nil {
			view.canvas.syncSurface()
			view.canvas.reportSurface(0)
		}
	}
}
func gkGTKUISetVisible(pointer unsafe.Pointer, visible cInt) {
	if view := viewState(pointer); view != nil {
		view.visible = visible != 0
		gtkWidgetSetVisible(view.outer, nativeBool(view.visible))
		if view.canvas != nil {
			view.canvas.syncSurface()
		}
	}
}
func gkGTKUISetEnabled(pointer unsafe.Pointer, enabled cInt) {
	if view := viewState(pointer); view != nil {
		gtkWidgetSetSensitive(view.outer, nativeBool(enabled != 0))
	}
}
func gkGTKUIPreferredSize(pointer unsafe.Pointer, width, height *cInt) {
	*width, *height = 0, 0
	view := viewState(pointer)
	if view == nil {
		return
	}
	w := measureWidget(view.outer, gtkOrientationHorizontal, -1)
	if view.kind == kindLabel && view.preferredWidth > 0 && gtkLabelGetWrap((*gtkLabel)(unsafe.Pointer(view.outer))) != 0 {
		var minimum cInt
		gtkWidgetMeasure(view.outer, gtkOrientationHorizontal, -1, &minimum, nil, nil, nil)
		w = max(int(minimum), min(w, view.preferredWidth))
	}
	h := measureWidget(view.outer, gtkOrientationVertical, w)
	switch view.kind {
	case kindTextArea:
		w, h = 200, 96
	case kindRadioGroup:
		w, h = max(120, w), max(24, h)
	case kindTabs, kindScroll:
		w, h = 320, 240
	}
	*width, *height = cInt(w), cInt(h)
}
func gkGTKUIFramebufferSize(pointer unsafe.Pointer, width, height *cInt) {
	*width, *height = 0, 0
	if view := viewState(pointer); view != nil {
		scale := gtkWidgetGetScaleFactor(view.outer)
		*width, *height = cInt(view.width)*scale, cInt(view.height)*scale
	}
}
func gkGTKUIText(pointer unsafe.Pointer) *cChar {
	view := viewState(pointer)
	if view == nil {
		return gStrdup(stringPointer(""))
	}
	switch view.kind {
	case kindLabel:
		return gStrdup(gtkLabelGetText((*gtkLabel)(unsafe.Pointer(view.outer))))
	case kindButton, kindLink, kindToggle, kindCheckBox:
		if view.hasText {
			return gStrdup(stringPointer(view.text))
		}
		if view.kind == kindCheckBox {
			return gStrdup(gtkCheckButtonGetLabel((*gtkCheckButton)(unsafe.Pointer(view.control))))
		}
		return gStrdup(gtkButtonGetLabel((*gtkButton)(unsafe.Pointer(view.control))))
	case kindTextArea:
		buffer := gtkTextViewGetBuffer((*gtkTextView)(unsafe.Pointer(view.control)))
		var start, end gtkTextIter
		gtkTextBufferGetBounds(buffer, &start, &end)
		return gtkTextBufferGetText(buffer, &start, &end, 0)
	case kindSection:
		return gStrdup(gtkLabelGetText((*gtkLabel)(unsafe.Pointer(view.section.title))))
	}
	if isNative(unsafe.Pointer(view.control), gtkLabelGetType()) {
		return gStrdup(gtkLabelGetText((*gtkLabel)(unsafe.Pointer(view.control))))
	}
	if isNative(unsafe.Pointer(view.control), gtkEditableGetType()) {
		return gStrdup(gtkEditableGetText((*gtkEditable)(unsafe.Pointer(view.control))))
	}
	return gStrdup(stringPointer(""))
}
func gkGTKUISetText(pointer unsafe.Pointer, text *cChar) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	view.suppress++
	defer func() { view.suppress-- }()
	switch view.kind {
	case kindLabel:
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.outer)), text)
	case kindButton, kindLink, kindToggle:
		if view.button != nil {
			view.setButtonText(goString(text))
		} else {
			gtkButtonSetLabel((*gtkButton)(unsafe.Pointer(view.control)), text)
		}
	case kindCheckBox:
		view.setCheckText(goString(text))
	case kindTextArea:
		gtkTextBufferSetText(gtkTextViewGetBuffer((*gtkTextView)(unsafe.Pointer(view.control))), text, -1)
	case kindSection:
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.section.title)), text)
	default:
		if isNative(unsafe.Pointer(view.control), gtkLabelGetType()) {
			gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.control)), text)
		} else if isNative(unsafe.Pointer(view.control), gtkEditableGetType()) {
			gtkEditableSetText((*gtkEditable)(unsafe.Pointer(view.control)), text)
		}
	}
}
func gkGTKUISetPlaceholder(pointer unsafe.Pointer, text *cChar) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	if view.kind == kindTextArea {
		view.placeholder = goString(text)
		view.updatePlaceholder()
	} else if isNative(unsafe.Pointer(view.control), gtkEntryGetType()) {
		gtkEntrySetPlaceholderText((*gtkEntry)(unsafe.Pointer(view.control)), text)
	} else if isNative(unsafe.Pointer(view.control), gtkSearchEntryGetType()) {
		gtkSearchEntrySetPlaceholderText((*gtkSearchEntry)(unsafe.Pointer(view.control)), text)
	} else {
		setNativeProperty(unsafe.Pointer(view.control), "placeholder-text", goString(text))
	}
}
func gkGTKUIPlaceholder(pointer unsafe.Pointer) *cChar {
	view := viewState(pointer)
	if view == nil {
		return gStrdup(stringPointer(""))
	}
	if view.kind == kindTextArea {
		return gStrdup(stringPointer(view.placeholder))
	}
	if isNative(unsafe.Pointer(view.control), gtkEntryGetType()) {
		return gStrdup(gtkEntryGetPlaceholderText((*gtkEntry)(unsafe.Pointer(view.control))))
	}
	if isNative(unsafe.Pointer(view.control), gtkSearchEntryGetType()) {
		return gStrdup(gtkSearchEntryGetPlaceholderText((*gtkSearchEntry)(unsafe.Pointer(view.control))))
	}
	return gStrdup(stringPointer(getNativeStringProperty(unsafe.Pointer(view.control), "placeholder-text")))
}
func gkGTKUIChecked(pointer unsafe.Pointer) cInt {
	view := viewState(pointer)
	if view == nil {
		return 0
	}
	widget := unsafe.Pointer(view.control)
	switch {
	case isNative(widget, gtkCheckButtonGetType()):
		return cInt(gtkCheckButtonGetActive((*gtkCheckButton)(widget)))
	case isNative(widget, gtkToggleButtonGetType()):
		return cInt(gtkToggleButtonGetActive((*gtkToggleButton)(widget)))
	case isNative(widget, gtkSwitchGetType()):
		return cInt(gtkSwitchGetActive((*gtkSwitch)(widget)))
	}
	return 0
}
func gkGTKUISetChecked(pointer unsafe.Pointer, checked cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	view.suppress++
	defer func() { view.suppress-- }()
	widget := unsafe.Pointer(view.control)
	switch {
	case isNative(widget, gtkCheckButtonGetType()):
		gtkCheckButtonSetInconsistent((*gtkCheckButton)(widget), 0)
		gtkCheckButtonSetActive((*gtkCheckButton)(widget), nativeBool(checked != 0))
	case isNative(widget, gtkToggleButtonGetType()):
		gtkToggleButtonSetActive((*gtkToggleButton)(widget), nativeBool(checked != 0))
	case isNative(widget, gtkSwitchGetType()):
		gtkSwitchSetActive((*gtkSwitch)(widget), nativeBool(checked != 0))
	}
}
func gkGTKUIReadOnly(pointer unsafe.Pointer) cInt {
	if view := viewState(pointer); view != nil && view.kind == kindTextArea {
		return cInt(nativeBool(gtkTextViewGetEditable((*gtkTextView)(unsafe.Pointer(view.control))) == 0))
	}
	return 0
}
func gkGTKUISetReadOnly(pointer unsafe.Pointer, readOnly cInt) {
	if view := viewState(pointer); view != nil && view.kind == kindTextArea {
		gtkTextViewSetEditable((*gtkTextView)(unsafe.Pointer(view.control)), nativeBool(readOnly == 0))
		gtkTextViewSetCursorVisible((*gtkTextView)(unsafe.Pointer(view.control)), nativeBool(readOnly == 0))
	}
}
func gkGTKUISliderValue(pointer unsafe.Pointer) cDouble {
	if view := viewState(pointer); view != nil {
		return gtkRangeGetValue((*gtkRange)(unsafe.Pointer(view.control)))
	}
	return 0
}
func gkGTKUISetSliderValue(pointer unsafe.Pointer, value cDouble) {
	if view := viewState(pointer); view != nil {
		view.suppress++
		gtkRangeSetValue((*gtkRange)(unsafe.Pointer(view.control)), value)
		view.suppress--
	}
}
func gkGTKUIProgressValue(pointer unsafe.Pointer) cDouble {
	if view := viewState(pointer); view != nil {
		return cDouble(view.value)
	}
	return 0
}
func gkGTKUISetProgressValue(pointer unsafe.Pointer, value cDouble) {
	if view := viewState(pointer); view != nil {
		view.value = float64(max(0, min(1, value)))
		if isNative(unsafe.Pointer(view.control), gtkProgressBarGetType()) {
			gtkProgressBarSetFraction((*gtkProgressBar)(unsafe.Pointer(view.control)), cDouble(view.value))
		} else if isNative(unsafe.Pointer(view.control), gtkLevelBarGetType()) {
			gtkLevelBarSetValue((*gtkLevelBar)(unsafe.Pointer(view.control)), cDouble(view.value))
		}
	}
}
func gkGTKUIActivityRunning(pointer unsafe.Pointer) cInt {
	if view := viewState(pointer); view != nil {
		return cInt(nativeBool(view.running))
	}
	return 0
}
func gkGTKUISetActivityRunning(pointer unsafe.Pointer, running cInt) {
	if view := viewState(pointer); view != nil {
		view.running = running != 0
		gtkSpinnerSetSpinning((*gtkSpinner)(unsafe.Pointer(view.control)), nativeBool(view.running))
	}
}
