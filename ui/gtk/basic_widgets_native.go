//go:build linux && cgo

package gtk

import (
	"math"
	"unsafe"
)

var nativeImageType gType

func measureNativeImage(widget *gtkWidget, orientation gtkOrientation) (int, int) {
	view := viewState(unsafe.Pointer(widget))
	if view == nil || view.texture == nil {
		return 0, 0
	}
	size := gdkTextureGetWidth(view.texture)
	if orientation == gtkOrientationVertical {
		size = gdkTextureGetHeight(view.texture)
	}
	return 0, int(math.Ceil(float64(size) / view.sourceScale))
}
func snapshotNativeImage(widget *gtkWidget, snapshot *gtkSnapshot) {
	view := viewState(unsafe.Pointer(widget))
	if view == nil || view.texture == nil {
		return
	}
	width, height := float64(gtkWidgetGetWidth(widget)), float64(gtkWidgetGetHeight(widget))
	sw, sh := float64(gdkTextureGetWidth(view.texture)), float64(gdkTextureGetHeight(view.texture))
	destination := nativeRect(0, 0, width, height)
	if view.scaling != 2 && sw > 0 && sh > 0 {
		scale := math.Min(width/sw, height/sh)
		if view.scaling == 1 {
			scale = math.Max(width/sw, height/sh)
		}
		w, h := sw*scale, sh*scale
		horizontal := [...]float64{0.5, 0, 0.5, 1, 0, 1, 0, 0.5, 1}
		vertical := [...]float64{0.5, 0, 0, 0, 0.5, 0.5, 1, 1, 1}
		anchor := view.alignment
		if anchor < 0 || anchor >= len(horizontal) {
			anchor = 0
		}
		destination = nativeRect((width-w)*horizontal[anchor], (height-h)*vertical[anchor], w, h)
	}
	clip := nativeRect(0, 0, width, height)
	gtkSnapshotPushClip(snapshot, &clip)
	filter := gskScalingFilterTrilinear
	if view.nearest {
		filter = gskScalingFilterNearest
	}
	gtkSnapshotAppendScaledTexture(snapshot, view.texture, filter, &destination)
	gtkSnapshotPop(snapshot)
}
func gkGTKUICreateLayout(window cUintptr) unsafe.Pointer {
	widget := createNativeLayout(gtkAccessibleRoleGroup)
	view := createNativeView(uintptr(window), widget, nil, kindLayout, 0)
	view.body = widget
	return unsafe.Pointer(widget)
}
func gkGTKUICreateLabel(window cUintptr, text *cChar) unsafe.Pointer {
	widget := gtkLabelNew(text)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(widget)), 0)
	createNativeView(uintptr(window), widget, nil, kindLabel, 0)
	return unsafe.Pointer(widget)
}
func gkGTKUICreateButton(window cUintptr, text *cChar, handle cUintptr) unsafe.Pointer {
	widget := gtkButtonNewWithLabel(text)
	view := createNativeView(uintptr(window), widget, nil, kindButton, uintptr(handle))
	connectNativeSignal(unsafe.Pointer(widget), "clicked", func() { view.activate(uiEventClick) })
	return unsafe.Pointer(widget)
}
func createNativeAffixBox(entry *gtkWidget) (box, prefix, suffix *gtkWidget) {
	box = gtkBoxNew(gtkOrientationHorizontal, 4)
	prefix, suffix = gtkLabelNew(stringPointer("")), gtkLabelNew(stringPointer(""))
	for _, label := range []*gtkWidget{prefix, suffix} {
		gtkWidgetAddCSSClass(label, stringPointer("dim-label"))
		gtkWidgetSetVisible(label, 0)
	}
	gtkWidgetSetHexpand(entry, 1)
	gtkWidgetSetValign(entry, gtkAlignCenter)
	for _, child := range []*gtkWidget{prefix, entry, suffix} {
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), child)
	}
	return
}
func gkGTKUICreateTextField(window, handle cUintptr) unsafe.Pointer {
	entry := gtkEntryNew()
	box, prefix, suffix := createNativeAffixBox(entry)
	view := createNativeView(uintptr(window), box, entry, kindTextField, uintptr(handle))
	view.prefix, view.suffix = prefix, suffix
	connectNativeSignal(unsafe.Pointer(entry), "changed", func() { view.activate(uiEventTextChange) })
	connectNativeSignal(unsafe.Pointer(entry), "activate", func() { view.activate(uiEventSubmit) })
	return unsafe.Pointer(box)
}
func (view *nativeView) updatePlaceholder() {
	if view.label == nil {
		return
	}
	buffer := gtkTextViewGetBuffer((*gtkTextView)(unsafe.Pointer(view.control)))
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.label)), stringPointer(view.placeholder))
	gtkWidgetSetVisible(view.label, nativeBool(view.placeholder != "" && gtkTextBufferGetCharCount(buffer) == 0))
}
func gkGTKUICreateTextArea(window cUintptr, text, placeholder *cChar, readOnly, wrap cInt, handle cUintptr) unsafe.Pointer {
	outer, scroll, editor := gtkOverlayNew(), gtkScrolledWindowNew(), gtkTextViewNew()
	gtkScrolledWindowSetHasFrame((*gtkScrolledWindow)(unsafe.Pointer(scroll)), 1)
	policy := gtkPolicyAutomatic
	if wrap != 0 {
		policy = gtkPolicyNever
	}
	gtkScrolledWindowSetPolicy((*gtkScrolledWindow)(unsafe.Pointer(scroll)), policy, gtkPolicyAutomatic)
	mode := gtkWrapNone
	if wrap != 0 {
		mode = gtkWrapWordChar
	}
	textView := (*gtkTextView)(unsafe.Pointer(editor))
	gtkTextViewSetWrapMode(textView, mode)
	gtkTextViewSetEditable(textView, nativeBool(readOnly == 0))
	gtkTextViewSetCursorVisible(textView, nativeBool(readOnly == 0))
	gtkTextViewSetLeftMargin(textView, 4)
	gtkTextViewSetTopMargin(textView, 4)
	gtkScrolledWindowSetChild((*gtkScrolledWindow)(unsafe.Pointer(scroll)), editor)
	gtkOverlaySetChild((*gtkOverlay)(unsafe.Pointer(outer)), scroll)
	hint := gtkLabelNew(stringPointer(""))
	gtkWidgetAddCSSClass(hint, stringPointer("dim-label"))
	gtkWidgetSetHalign(hint, gtkAlignStart)
	gtkWidgetSetValign(hint, gtkAlignStart)
	gtkWidgetSetMarginStart(hint, 6)
	gtkWidgetSetMarginTop(hint, 5)
	gtkWidgetSetCanTarget(hint, 0)
	gtkOverlayAddOverlay((*gtkOverlay)(unsafe.Pointer(outer)), hint)
	view := createNativeView(uintptr(window), outer, editor, kindTextArea, uintptr(handle))
	view.label, view.scroll, view.placeholder = hint, scroll, goString(placeholder)
	buffer := gtkTextViewGetBuffer(textView)
	gtkTextBufferSetText(buffer, text, -1)
	connectNativeSignal(unsafe.Pointer(buffer), "changed", func() { view.updatePlaceholder(); view.activate(uiEventTextChange) })
	view.updatePlaceholder()
	return unsafe.Pointer(outer)
}
func gkGTKUICreateCheckBox(window cUintptr, text *cChar, checked cInt, handle cUintptr) unsafe.Pointer {
	widget := gtkCheckButtonNewWithLabel(text)
	button := (*gtkCheckButton)(unsafe.Pointer(widget))
	gtkCheckButtonSetActive(button, nativeBool(checked != 0))
	view := createNativeView(uintptr(window), widget, nil, kindCheckBox, uintptr(handle))
	connectNativeSignal(unsafe.Pointer(widget), "toggled", func() {
		if view.suppress == 0 {
			gtkCheckButtonSetInconsistent(button, 0)
		}
		view.activate(uiEventToggleChange)
	})
	return unsafe.Pointer(widget)
}
func (view *nativeView) addRadio(title string, enabled bool) {
	widget := gtkCheckButtonNewWithLabel(stringPointer(title))
	button := (*gtkCheckButton)(unsafe.Pointer(widget))
	if len(view.buttons) > 0 {
		gtkCheckButtonSetGroup(button, (*gtkCheckButton)(unsafe.Pointer(view.buttons[0])))
	}
	gtkWidgetSetSensitive(widget, nativeBool(enabled))
	index := len(view.buttons)
	view.buttons = append(view.buttons, widget)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(view.control)), widget)
	connectNativeSignal(unsafe.Pointer(widget), "toggled", func() {
		if view.suppress == 0 && gtkCheckButtonGetActive(button) != 0 {
			view.selected = index
			view.activate(uiEventSelectionChange)
		}
	})
}
func (view *nativeView) selectRadio(index int) {
	view.suppress++
	defer func() { view.suppress-- }()
	view.selected = index
	for i, widget := range view.buttons {
		gtkCheckButtonSetActive((*gtkCheckButton)(unsafe.Pointer(widget)), nativeBool(i == index))
	}
}
func gkGTKUICreateRadioGroup(window, handle cUintptr) unsafe.Pointer {
	outer := boxWithRole(gtkOrientationVertical, 4, gtkAccessibleRoleRadioGroup)
	title, buttons := gtkLabelNew(stringPointer("")), gtkBoxNew(gtkOrientationVertical, 2)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(title)), 0)
	gtkWidgetSetVisible(title, 0)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(outer)), title)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(outer)), buttons)
	view := createNativeView(uintptr(window), outer, buttons, kindRadioGroup, uintptr(handle))
	view.label = title
	return unsafe.Pointer(outer)
}
func gkGTKUICreateSelect(window, handle cUintptr) unsafe.Pointer {
	model := gtkStringListNew(nil)
	widget := gtkDropDownNew((*gListModel)(unsafe.Pointer(model)), nil)
	dropdown := (*gtkDropDown)(unsafe.Pointer(widget))
	gtkDropDownSetSelected(dropdown, ^cUint(0))
	view := createNativeView(uintptr(window), widget, nil, kindSelect, uintptr(handle))
	factory := gtkSignalListItemFactoryNew()
	connectNativeArgument(unsafe.Pointer(factory), "setup", gtkListItemCallback(), func(item *gtkListItem) {
		label := gtkLabelNew(stringPointer(""))
		gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(label)), 0)
		gtkListItemSetChild(item, label)
	})
	connectNativeArgument(unsafe.Pointer(factory), "bind", gtkListItemCallback(), func(item *gtkListItem) {
		object := (*gtkStringObject)(gtkListItemGetItem(item))
		label := gtkListItemGetChild(item)
		text := ""
		if object != nil {
			text = goString(gtkStringObjectGetString(object))
		}
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(label)), stringPointer(text))
		position := int(gtkListItemGetPosition(item))
		enabled := position >= len(view.enabled) || view.enabled[position]
		gtkWidgetSetSensitive(label, nativeBool(enabled))
		gtkListItemSetSelectable(item, nativeBool(enabled))
		gtkListItemSetActivatable(item, nativeBool(enabled))
	})
	gtkDropDownSetListFactory(dropdown, factory)
	gObjectUnref(unsafe.Pointer(factory))
	connectNativeArgument(unsafe.Pointer(widget), "notify::selected", gtkNotifyCallback(), func() { view.activate(uiEventSelectionChange) })
	return unsafe.Pointer(widget)
}
func gkGTKUISelectionAddItem(pointer unsafe.Pointer, text *cChar) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	if view.kind == kindRadioGroup {
		view.addRadio(goString(text), true)
	} else if view.kind == kindSelect {
		dropdown := (*gtkDropDown)(unsafe.Pointer(view.control))
		view.suppress++
		selected := gtkDropDownGetSelected(dropdown)
		gtkStringListAppend((*gtkStringList)(unsafe.Pointer(gtkDropDownGetModel(dropdown))), text)
		gtkDropDownSetSelected(dropdown, selected)
		view.suppress--
	}
}
func gkGTKUISelectionIndex(pointer unsafe.Pointer) cInt {
	view := viewState(pointer)
	if view == nil {
		return -1
	}
	if view.kind == kindRadioGroup {
		return cInt(view.selected)
	}
	if view.kind == kindSelect {
		return cInt(gtkDropDownGetSelected((*gtkDropDown)(unsafe.Pointer(view.control))))
	}
	return -1
}
func gkGTKUISetSelectionIndex(pointer unsafe.Pointer, index cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	if view.kind == kindRadioGroup {
		view.selectRadio(int(index))
	} else if view.kind == kindSelect {
		view.suppress++
		gtkDropDownSetSelected((*gtkDropDown)(unsafe.Pointer(view.control)), cUint(index))
		view.suppress--
	}
}
func gkGTKUICreateSlider(window cUintptr, minimum, maximum, value, step cDouble, handle cUintptr) unsafe.Pointer {
	increment := step
	if increment <= 0 {
		increment = (maximum - minimum) / 100
	}
	adjustment := gtkAdjustmentNew(value, minimum, maximum, increment, 0, 0)
	widget := gtkScaleNew(gtkOrientationHorizontal, adjustment)
	gtkScaleSetDrawValue((*gtkScale)(unsafe.Pointer(widget)), 0)
	gtkWidgetSetSizeRequest(widget, 96, -1)
	view := createNativeView(uintptr(window), widget, nil, kindSlider, uintptr(handle))
	view.value = float64(step)
	connectNativeSignal(unsafe.Pointer(widget), "value-changed", func() { view.activate(uiEventValueChange) })
	connectNativeArgument(unsafe.Pointer(widget), "change-value", gtkRangeValueCallback(), func(value float64) bool {
		if view.value <= 0 {
			return false
		}
		low, high := float64(gtkAdjustmentGetLower(adjustment)), float64(gtkAdjustmentGetUpper(adjustment))
		snapped := low + math.Round((value-low)/view.value)*view.value
		gtkRangeSetValue((*gtkRange)(unsafe.Pointer(widget)), cDouble(max(low, min(high, snapped))))
		return true
	})
	return unsafe.Pointer(widget)
}
func gkGTKUICreateProgressBar(window cUintptr, value cDouble) unsafe.Pointer {
	widget := gtkProgressBarNew()
	value = max(0, min(1, value))
	gtkProgressBarSetFraction((*gtkProgressBar)(unsafe.Pointer(widget)), value)
	gtkWidgetSetValign(widget, gtkAlignCenter)
	createNativeView(uintptr(window), widget, nil, kindProgress, 0).value = float64(value)
	return unsafe.Pointer(widget)
}
func gkGTKUICreateActivityIndicator(window cUintptr, running cInt) unsafe.Pointer {
	widget := gtkSpinnerNew()
	gtkSpinnerSetSpinning((*gtkSpinner)(unsafe.Pointer(widget)), nativeBool(running != 0))
	gtkWidgetSetSizeRequest(widget, 16, 16)
	createNativeView(uintptr(window), widget, nil, kindActivity, 0).running = running != 0
	return unsafe.Pointer(widget)
}
func gkGTKUICreateSeparator(window cUintptr, direction cInt) unsafe.Pointer {
	orientation := gtkOrientationHorizontal
	if direction == 1 {
		orientation = gtkOrientationVertical
	}
	widget := boxWithRole(orientation, 6, gtkAccessibleRoleSeparator)
	view := createNativeView(uintptr(window), widget, gtkSeparatorNew(orientation), kindSeparator, 0)
	view.label, view.end = gtkLabelNew(stringPointer("")), gtkSeparatorNew(orientation)
	gtkWidgetAddCSSClass(view.label, stringPointer("dim-label"))
	gtkWidgetAddCSSClass(view.label, stringPointer("caption"))
	gtkWidgetSetVisible(view.label, 0)
	gtkWidgetSetVisible(view.end, 0)
	for _, child := range []*gtkWidget{view.control, view.label, view.end} {
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(widget)), child)
	}
	gkGTKSeparatorConfigure(unsafe.Pointer(widget), direction, nil, 0, 1)
	return unsafe.Pointer(widget)
}
func gkGTKUICreateImage(window cUintptr, scaling cInt) unsafe.Pointer {
	if nativeImageType == 0 {
		nativeImageType = registerNativeImageType()
	}
	widget := widgetWithRole(nativeImageType, gtkAccessibleRoleImg)
	createNativeView(uintptr(window), widget, nil, kindImage, 0).scaling = int(scaling)
	return unsafe.Pointer(widget)
}
func gkGTKUIImageSet(pointer unsafe.Pointer, pixels *cUchar, width, height, stride cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	if view.texture != nil {
		gObjectUnref(unsafe.Pointer(view.texture))
		view.texture = nil
	}
	if pixels != nil && width > 0 && height > 0 {
		if stride <= 0 {
			stride = width * 4
		}
		bytes := gBytesNew(unsafe.Pointer(pixels), gSize(stride)*gSize(height))
		view.texture = gdkMemoryTextureNew(width, height, gdkMemoryR8G8B8A8, bytes, gSize(stride))
		gBytesUnref(bytes)
	}
	gtkWidgetQueueResize(view.outer)
	gtkWidgetQueueDraw(view.outer)
}
func gkGTKUICreateLink(window cUintptr, text, uri *cChar, handle cUintptr) unsafe.Pointer {
	address := goString(uri)
	if address == "" {
		address = "about:blank"
	}
	widget := gtkLinkButtonNewWithLabel(stringPointer(address), text)
	gtkWidgetSetHalign(widget, gtkAlignStart)
	view := createNativeView(uintptr(window), widget, nil, kindLink, uintptr(handle))
	view.url = goString(uri)
	connectNativeArgument(unsafe.Pointer(widget), "activate-link", gtkCloseRequestCallback(), func() bool { view.activate(uiEventClick); return true })
	return unsafe.Pointer(widget)
}
func gkGTKUILinkURL(pointer unsafe.Pointer) *cChar {
	if view := viewState(pointer); view != nil {
		return gStrdup(stringPointer(view.url))
	}
	return gStrdup(stringPointer(""))
}
func gkGTKUISetLinkURL(pointer unsafe.Pointer, uri *cChar) {
	if view := viewState(pointer); view != nil {
		view.url = goString(uri)
		address := view.url
		if address == "" {
			address = "about:blank"
		}
		gtkLinkButtonSetURI((*gtkLinkButton)(unsafe.Pointer(view.control)), stringPointer(address))
	}
}
func gkGTKUICreateTabs(window, handle cUintptr) unsafe.Pointer {
	widget := gtkNotebookNew()
	view := createNativeView(uintptr(window), widget, nil, kindTabs, uintptr(handle))
	data := newNativeCallback(func() { view.activate(uiEventTabChange) })
	gSignalConnectData(unsafe.Pointer(widget), stringPointer("switch-page"), gtkTabSwitchCallback(), data, gtkCallbackFree(), gConnectAfter)
	return unsafe.Pointer(widget)
}
func gkGTKUITabsAdd(pointer unsafe.Pointer, title *cChar, content unsafe.Pointer) cInt {
	view := viewState(pointer)
	if view == nil || content == nil {
		return 0
	}
	page := createNativeLayout(gtkAccessibleRoleGroup)
	putNativeLayout(page, (*gtkWidget)(content))
	view.suppress++
	index := gtkNotebookAppendPage((*gtkNotebook)(unsafe.Pointer(view.outer)), page, gtkLabelNew(title))
	view.suppress--
	return cInt(nativeBool(index >= 0))
}
func gkGTKUITabsSelected(pointer unsafe.Pointer) cInt {
	if view := viewState(pointer); view != nil {
		return gtkNotebookGetCurrentPage((*gtkNotebook)(unsafe.Pointer(view.outer)))
	}
	return -1
}
func gkGTKUITabsSetSelected(pointer unsafe.Pointer, index cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	notebook := (*gtkNotebook)(unsafe.Pointer(view.outer))
	if index >= 0 && index < gtkNotebookGetNPages(notebook) {
		view.suppress++
		gtkNotebookSetCurrentPage(notebook, index)
		view.suppress--
	}
}
func nativeChromeSize(widget *gtkWidget) (int, int) {
	var width, height cInt
	gtkWidgetMeasure(widget, gtkOrientationHorizontal, -1, &width, nil, nil, nil)
	gtkWidgetMeasure(widget, gtkOrientationVertical, -1, &height, nil, nil, nil)
	return int(width), int(height)
}
func gkGTKUITabsContentSize(pointer unsafe.Pointer, width, height *cInt) {
	*width, *height = 0, 0
	view := viewState(pointer)
	if view == nil {
		return
	}
	notebook := (*gtkNotebook)(unsafe.Pointer(view.outer))
	index := gtkNotebookGetCurrentPage(notebook)
	var page *gtkWidget
	if index >= 0 {
		page = gtkNotebookGetNthPage(notebook, index)
	}
	if page != nil && int(gtkWidgetGetWidth(view.outer)) == view.width && int(gtkWidgetGetHeight(view.outer)) == view.height && gtkWidgetGetWidth(page) > 0 {
		*width, *height = gtkWidgetGetWidth(page), gtkWidgetGetHeight(page)
		return
	}
	w, h := nativeChromeSize(view.outer)
	position := gtkNotebookGetTabPos(notebook)
	if position == gtkPosTop || position == gtkPosBottom {
		w = 2
	} else {
		h = 2
	}
	*width, *height = cInt(max(0, view.width-w)), cInt(max(0, view.height-h))
}

func gkGTKUIImageSetScaling(pointer unsafe.Pointer, scaling cInt) {
	if view := viewState(pointer); view != nil {
		view.scaling = int(scaling)
		gtkWidgetQueueDraw(view.outer)
	}
}
