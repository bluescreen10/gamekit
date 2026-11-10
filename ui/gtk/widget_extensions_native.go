//go:build linux && cgo

package gtk

import (
	"fmt"
	"math"
	"unicode/utf8"
	"unsafe"
)

func gkGTKLabelConfigure(pointer unsafe.Pointer, alignment, wrap, truncation, selectable, style, maximumLines cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	label := (*gtkLabel)(unsafe.Pointer(view.outer))
	alignments := [...]cFloat{0, 0.5, 1, 0}
	index := int(alignment)
	if index < 0 || index >= 4 {
		index = 0
	}
	gtkLabelSetXalign(label, alignments[index])
	gtkLabelSetJustify(label, gtkJustification(index))
	gtkLabelSetSelectable(label, nativeBool(selectable != 0))
	gtkLabelSetWrap(label, nativeBool(wrap != 0))
	mode := pangoWrapWordChar
	if wrap == 2 {
		mode = pangoWrapChar
	}
	gtkLabelSetWrapMode(label, mode)
	ellipsize := pangoEllipsizeNone
	if truncation >= 1 && truncation <= 3 {
		ellipsize = pangoEllipsizeMode(truncation)
	}
	gtkLabelSetEllipsize(label, ellipsize)
	lines := cInt(-1)
	if wrap != 0 && maximumLines > 0 {
		lines = maximumLines
	}
	gtkLabelSetLines(label, lines)
	gtkLabelSetSingleLineMode(label, nativeBool(wrap == 0))
	styles := []string{"body", "caption", "heading", "title-2", "monospace"}
	for i, class := range styles {
		gtkWidgetRemoveCSSClass(view.outer, stringPointer(class))
		if int(style) == i {
			gtkWidgetAddCSSClass(view.outer, stringPointer(class))
		}
	}
}
func gkGTKLabelPreferredWidth(pointer unsafe.Pointer, width cInt) {
	if view := viewState(pointer); view != nil {
		view.preferredWidth = max(0, int(width))
		gtkLabelSetMaxWidthChars((*gtkLabel)(unsafe.Pointer(view.outer)), -1)
	}
}

type nativeButton struct {
	box, icon, label, spinner, badge *gtkWidget
	shortcuts                        *gtkEventController
}

func (view *nativeView) buttonContent() *nativeButton {
	if view.button != nil {
		return view.button
	}
	text := goString(gtkButtonGetLabel((*gtkButton)(unsafe.Pointer(view.control))))
	parts := &nativeButton{
		box:     gtkBoxNew(gtkOrientationHorizontal, 6),
		icon:    gtkImageNew(),
		label:   gtkLabelNew(stringPointer(text)),
		spinner: gtkSpinnerNew(),
		badge:   gtkLabelNew(stringPointer("")),
	}
	view.button = parts
	view.text, view.hasText = text, true
	overlay := gtkOverlayNew()
	gtkWidgetSetHalign(parts.box, gtkAlignCenter)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(parts.box)), parts.icon)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(parts.box)), parts.label)
	gtkOverlaySetChild((*gtkOverlay)(unsafe.Pointer(overlay)), parts.box)
	gtkWidgetSetHalign(parts.spinner, gtkAlignCenter)
	gtkWidgetSetValign(parts.spinner, gtkAlignCenter)
	gtkOverlayAddOverlay((*gtkOverlay)(unsafe.Pointer(overlay)), parts.spinner)
	gtkWidgetAddCSSClass(parts.badge, stringPointer("gk-badge"))
	gtkWidgetSetHalign(parts.badge, gtkAlignEnd)
	gtkWidgetSetValign(parts.badge, gtkAlignStart)
	gtkWidgetSetCanTarget(parts.badge, 0)
	gtkOverlayAddOverlay((*gtkOverlay)(unsafe.Pointer(overlay)), parts.badge)
	for _, widget := range []*gtkWidget{parts.icon, parts.spinner, parts.badge} {
		gtkWidgetSetVisible(widget, 0)
	}
	connectNativeArgument(unsafe.Pointer(overlay), "get-child-position", gtkOverlayPositionCallback(), func(child *gtkWidget, allocation *gdkRectangle) bool {
		if child != parts.badge {
			return false
		}
		w := measureWidget(child, gtkOrientationHorizontal, -1)
		h := measureWidget(child, gtkOrientationVertical, w)
		bounds := nativeRect(0, 0, float64(gtkWidgetGetWidth(overlay)), float64(gtkWidgetGetHeight(overlay)))
		gtkWidgetComputeBounds(view.control, overlay, &bounds)
		x := int(bounds.origin.x+bounds.size.width) - w/2 - 4
		if gtkWidgetGetDirection(overlay) == gtkTextDirRTL {
			x = int(bounds.origin.x) - w/2 + 4
		}
		*allocation = gdkRectangle{x: cInt(x), y: cInt(int(bounds.origin.y) - h/2 + 2), width: cInt(w), height: cInt(h)}
		return true
	})
	gtkWidgetSetOverflow(overlay, gtkOverflowVisible)
	gtkWidgetSetOverflow(view.control, gtkOverflowVisible)
	setNativeCSS("label.gk-badge", "background-color: @error_color; color: white; border-radius: 8px; padding: 0 5px; font-size: smaller; font-weight: bold; min-width: 6px; min-height: 16px;")
	gtkButtonSetChild((*gtkButton)(unsafe.Pointer(view.control)), overlay)
	setAccessibleText(view.control, gtkAccessiblePropertyLabel, text)
	return parts
}
func (view *nativeView) setButtonText(text string) {
	view.text, view.hasText = text, true
	if view.button != nil {
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.button.label)), stringPointer(text))
		if view.accessibleName == "" {
			setAccessibleText(view.control, gtkAccessiblePropertyLabel, text)
		}
	}
}
func installNativeActivationShortcut(widget *gtkWidget, accelerator string) *gtkEventController {
	if accelerator == "" {
		return nil
	}
	trigger := gtkShortcutTriggerParseString(stringPointer(accelerator))
	if trigger == nil {
		return nil
	}
	controller := gtkShortcutControllerNew()
	gtkShortcutControllerSetScope((*gtkShortcutController)(unsafe.Pointer(controller)), gtkShortcutScopeManaged)
	action := gtkActivateActionGet()
	gObjectRef(unsafe.Pointer(action))
	gtkShortcutControllerAddShortcut((*gtkShortcutController)(unsafe.Pointer(controller)), gtkShortcutNew(trigger, (*gtkShortcutAction)(unsafe.Pointer(action))))
	gtkWidgetAddController(widget, controller)
	return controller
}
func gkGTKButtonConfigure(pointer, icon unsafe.Pointer, placement, role cInt, key *cChar, modifiers cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	parts := view.buttonContent()
	gtkImageSetFromPaintable((*gtkImage)(unsafe.Pointer(parts.icon)), (*gdkPaintable)(icon))
	gtkWidgetSetVisible(parts.icon, nativeBool(icon != nil))
	gtkWidgetSetVisible(parts.label, nativeBool(icon == nil || placement != 4))
	orientation := gtkOrientationHorizontal
	if placement == 2 || placement == 3 {
		orientation = gtkOrientationVertical
	}
	gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(parts.box)), orientation)
	var after *gtkWidget
	if placement == 1 || placement == 3 {
		after = parts.label
	}
	gtkBoxReorderChildAfter((*gtkBox)(unsafe.Pointer(parts.box)), parts.icon, after)
	for _, class := range []string{"suggested-action", "destructive-action"} {
		gtkWidgetRemoveCSSClass(view.control, stringPointer(class))
	}
	accelerator := goString(key)
	if accelerator == "" {
		if role == 1 {
			accelerator = "Return"
		} else if role == 2 {
			accelerator = "Escape"
		}
	}
	if role == 1 {
		gtkWidgetAddCSSClass(view.control, stringPointer("suggested-action"))
	} else if role == 3 {
		gtkWidgetAddCSSClass(view.control, stringPointer("destructive-action"))
	}
	if parts.shortcuts != nil {
		gtkWidgetRemoveController(view.control, parts.shortcuts)
	}
	parts.shortcuts = installNativeActivationShortcut(view.control, accelerator)
}
func gkGTKButtonSetBusy(pointer unsafe.Pointer, busy cInt) {
	if view := viewState(pointer); view != nil {
		parts := view.buttonContent()
		gtkWidgetSetVisible(parts.spinner, nativeBool(busy != 0))
		gtkSpinnerSetSpinning((*gtkSpinner)(unsafe.Pointer(parts.spinner)), nativeBool(busy != 0))
		opacity := cDouble(1)
		if busy != 0 {
			opacity = 0
		}
		gtkWidgetSetOpacity(parts.box, opacity)
		setAccessibleBoolState(view.control, gtkAccessibleStateBusy, busy != 0)
	}
}
func gkGTKButtonSetBadge(pointer unsafe.Pointer, text *cChar) {
	if view := viewState(pointer); view != nil {
		parts := view.buttonContent()
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(parts.badge)), text)
		gtkWidgetSetVisible(parts.badge, nativeBool(goString(text) != ""))
		setOptionalAccessibleText(view.control, gtkAccessiblePropertyDescription, goString(text))
	}
}
func gkGTKTextFieldConfigure(pointer unsafe.Pointer, readOnly, maximumLength, purpose, alignment cInt, prefix, suffix *cChar, spellCheck cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	editable := (*gtkEditable)(unsafe.Pointer(view.control))
	gtkEditableSetEditable(editable, nativeBool(readOnly == 0))
	if isNative(unsafe.Pointer(view.control), gtkEntryGetType()) {
		entry := (*gtkEntry)(unsafe.Pointer(view.control))
		gtkEntrySetMaxLength(entry, max(0, maximumLength))
		purposes := []gtkInputPurpose{gtkInputPurposeFreeForm, gtkInputPurposeName, gtkInputPurposeFreeForm, gtkInputPurposeEmail, gtkInputPurposeURL, gtkInputPurposePhone}
		index := int(purpose)
		if index < 0 || index >= len(purposes) {
			index = 0
		}
		gtkEntrySetInputPurpose(entry, purposes[index])
		gtkEntrySetInputHints(entry, nativeSpellingHints(spellCheck != 0))
	}
	alignments := [...]cFloat{0, 0.5, 1, 0}
	index := int(alignment)
	if index < 0 || index >= 4 {
		index = 0
	}
	gtkEditableSetAlignment(editable, alignments[index])
	for _, part := range []struct {
		widget *gtkWidget
		text   *cChar
	}{{view.prefix, prefix}, {view.suffix, suffix}} {
		if part.widget != nil {
			gtkLabelSetText((*gtkLabel)(unsafe.Pointer(part.widget)), part.text)
			gtkWidgetSetVisible(part.widget, nativeBool(goString(part.text) != ""))
		}
	}
}
func nativeSpellingHints(enabled bool) gtkInputHints {
	if enabled {
		return gtkInputHintSpellcheck
	}
	return gtkInputHintNoSpellcheck
}
func nativeUTF16Offset(text string, characters int) int {
	units := 0
	for _, r := range text {
		if characters <= 0 {
			break
		}
		units++
		if r > 0xffff {
			units++
		}
		characters--
	}
	return units
}
func nativeCharacterOffset(text string, units int) int {
	characters := 0
	for _, r := range text {
		if units <= 0 {
			break
		}
		units--
		if r > 0xffff {
			units--
		}
		characters++
	}
	return characters
}
func nativeSelectionText(view *nativeView) (string, int, int) {
	var start, end cInt
	if isNative(unsafe.Pointer(view.control), gtkTextViewGetType()) {
		buffer := gtkTextViewGetBuffer((*gtkTextView)(unsafe.Pointer(view.control)))
		var first, last, begin, finish gtkTextIter
		gtkTextBufferGetSelectionBounds(buffer, &first, &last)
		start, end = gtkTextIterGetOffset(&first), gtkTextIterGetOffset(&last)
		gtkTextBufferGetBounds(buffer, &begin, &finish)
		return takeString(gtkTextBufferGetText(buffer, &begin, &finish, 0)), int(start), int(end)
	}
	editable := (*gtkEditable)(unsafe.Pointer(view.control))
	gtkEditableGetSelectionBounds(editable, &start, &end)
	if start == end {
		start = gtkEditableGetPosition(editable)
		end = start
	}
	return goString(gtkEditableGetText(editable)), int(start), int(end)
}
func gkGTKTextSelection(pointer unsafe.Pointer, start, length *cInt) {
	*start, *length = 0, 0
	if view := viewState(pointer); view != nil {
		text, a, b := nativeSelectionText(view)
		from, to := nativeUTF16Offset(text, min(a, b)), nativeUTF16Offset(text, max(a, b))
		*start, *length = cInt(from), cInt(to-from)
	}
}
func gkGTKSetTextSelection(pointer unsafe.Pointer, start, length cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	text, _, _ := nativeSelectionText(view)
	from, to := cInt(nativeCharacterOffset(text, max(0, int(start)))), cInt(nativeCharacterOffset(text, max(0, int(start))+max(0, int(length))))
	if isNative(unsafe.Pointer(view.control), gtkTextViewGetType()) {
		buffer := gtkTextViewGetBuffer((*gtkTextView)(unsafe.Pointer(view.control)))
		var first, last gtkTextIter
		gtkTextBufferGetIterAtOffset(buffer, &first, from)
		gtkTextBufferGetIterAtOffset(buffer, &last, to)
		gtkTextBufferSelectRange(buffer, &last, &first)
	} else {
		gtkEditableSelectRegion((*gtkEditable)(unsafe.Pointer(view.control)), from, to)
	}
}
func gkGTKTextAreaConfigure(pointer unsafe.Pointer, wrap, maximumLength, tabMovesFocus, spellCheck cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	editor := (*gtkTextView)(unsafe.Pointer(view.control))
	mode := gtkWrapWordChar
	if wrap == 0 {
		mode = gtkWrapNone
	} else if wrap == 2 {
		mode = gtkWrapChar
	}
	gtkTextViewSetWrapMode(editor, mode)
	if view.scroll != nil {
		horizontal := gtkPolicyAutomatic
		if wrap != 0 {
			horizontal = gtkPolicyNever
		}
		gtkScrolledWindowSetPolicy((*gtkScrolledWindow)(unsafe.Pointer(view.scroll)), horizontal, gtkPolicyAutomatic)
	}
	if !view.textLimitConnected {
		view.textLimitConnected = true
		buffer := gtkTextViewGetBuffer(editor)
		connectNativeArgument(unsafe.Pointer(buffer), "insert-text", gtkTextInsertCallback(), func(text string) {
			if view.maximumLength > 0 && int(gtkTextBufferGetCharCount(buffer))+utf8.RuneCountInString(text) > view.maximumLength {
				gSignalStopEmissionByName(unsafe.Pointer(buffer), stringPointer("insert-text"))
			}
		})
	}
	view.maximumLength = max(0, int(maximumLength))
	gtkTextViewSetAcceptsTab(editor, nativeBool(tabMovesFocus == 0))
	gtkTextViewSetInputHints(editor, nativeSpellingHints(spellCheck != 0))
}
func (view *nativeView) setCheckText(text string) {
	view.text, view.hasText = text, true
	if view.checkTitle != nil {
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.checkTitle)), stringPointer(text))
	} else {
		gtkCheckButtonSetLabel((*gtkCheckButton)(unsafe.Pointer(view.control)), stringPointer(text))
	}
}
func gkGTKCheckBoxConfigure(pointer unsafe.Pointer, allowsMixed, state, labelBefore cInt, text, description *cChar) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	button := (*gtkCheckButton)(unsafe.Pointer(view.control))
	view.suppress++
	gtkCheckButtonSetActive(button, nativeBool(state == 1))
	gtkCheckButtonSetInconsistent(button, nativeBool(state == 2 && allowsMixed != 0))
	view.suppress--
	direction := gtkTextDirNone
	if labelBefore != 0 {
		direction = gtkTextDirRTL
	}
	gtkWidgetSetDirection(view.control, direction)
	if goString(description) != "" {
		if view.checkTitle == nil {
			box := gtkBoxNew(gtkOrientationVertical, 0)
			view.checkTitle, view.checkDetail = gtkLabelNew(stringPointer("")), gtkLabelNew(stringPointer(""))
			for _, label := range []*gtkWidget{view.checkTitle, view.checkDetail} {
				gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(label)), 0)
				gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), label)
			}
			gtkWidgetAddCSSClass(view.checkDetail, stringPointer("dim-label"))
			gtkWidgetAddCSSClass(view.checkDetail, stringPointer("caption"))
			gtkCheckButtonSetChild(button, box)
		}
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.checkDetail)), description)
		view.setCheckText(goString(text))
		setAccessibleText(view.control, gtkAccessiblePropertyLabel, goString(text))
		setAccessibleText(view.control, gtkAccessiblePropertyDescription, goString(description))
	} else {
		if view.checkTitle != nil {
			gtkCheckButtonSetChild(button, nil)
			view.checkTitle, view.checkDetail = nil, nil
		}
		view.setCheckText(goString(text))
		gtkAccessibleResetProperty((*gtkAccessible)(unsafe.Pointer(view.control)), gtkAccessiblePropertyDescription)
	}
}
func gkGTKCheckBoxState(pointer unsafe.Pointer) cInt {
	if view := viewState(pointer); view != nil {
		button := (*gtkCheckButton)(unsafe.Pointer(view.control))
		if gtkCheckButtonGetInconsistent(button) != 0 {
			return 2
		}
		return cInt(gtkCheckButtonGetActive(button))
	}
	return 0
}
func gkGTKSetChoices(pointer unsafe.Pointer, titles *cChar, titlesSize cSize, enabled *cInt, count cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	values := splitNul(unsafe.Pointer(titles), titlesSize)
	flags := unsafe.Slice(enabled, int(count))
	view.enabled = make([]bool, len(values))
	for i := range values {
		view.enabled[i] = i >= len(flags) || flags[i] != 0
	}
	view.suppress++
	defer func() { view.suppress-- }()
	if view.kind == kindRadioGroup {
		for _, button := range view.buttons {
			gtkBoxRemove((*gtkBox)(unsafe.Pointer(view.control)), button)
		}
		view.buttons = nil
		for i, title := range values {
			view.addRadio(title, view.enabled[i])
		}
		index := view.selected
		if index >= len(values) {
			index = -1
		}
		view.selectRadio(index)
	} else {
		dropdown := (*gtkDropDown)(unsafe.Pointer(view.control))
		selected := gtkDropDownGetSelected(dropdown)
		strings, free := nativeStringVector(values)
		model := gtkStringListNew(strings)
		free()
		gtkDropDownSetModel(dropdown, (*gListModel)(unsafe.Pointer(model)))
		if int(selected) >= len(values) {
			selected = ^cUint(0)
		}
		gtkDropDownSetSelected(dropdown, selected)
		gObjectUnref(unsafe.Pointer(model))
	}
}
func gkGTKRadioGroupConfigure(pointer unsafe.Pointer, horizontal cInt, label *cChar) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	orientation, spacing := gtkOrientationVertical, cInt(2)
	if horizontal != 0 {
		orientation, spacing = gtkOrientationHorizontal, 12
	}
	gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(view.control)), orientation)
	gtkBoxSetSpacing((*gtkBox)(unsafe.Pointer(view.control)), spacing)
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.label)), label)
	gtkWidgetAddCSSClass(view.label, stringPointer("heading"))
	gtkWidgetSetVisible(view.label, nativeBool(goString(label) != ""))
	setOptionalAccessibleText(view.outer, gtkAccessiblePropertyLabel, goString(label))
}

func (view *nativeView) sizeSelectPopup() {
	popover := nativeDescendant(view.control, gtkPopoverGetType())
	if popover == nil {
		return
	}
	width := view.popupWidth
	if width <= 0 {
		width = -1
	}
	gtkWidgetSetSizeRequest(popover, cInt(width), -1)
	if scroll := nativeDescendant(popover, gtkScrolledWindowGetType()); scroll != nil {
		height := 400
		if view.visibleRows > 0 {
			height = view.visibleRows * 34
		}
		gtkScrolledWindowSetMaxContentHeight((*gtkScrolledWindow)(unsafe.Pointer(scroll)), cInt(height))
		gtkScrolledWindowSetPropagateNaturalHeight((*gtkScrolledWindow)(unsafe.Pointer(scroll)), 1)
	}
}
func gkGTKSelectConfigure(pointer unsafe.Pointer, placeholder *cChar, popupWidth cInt) {
	if view := viewState(pointer); view != nil {
		setOptionalAccessibleText(view.control, gtkAccessiblePropertyPlaceholder, goString(placeholder))
		if goString(placeholder) != "" {
			gtkWidgetSetTooltipText(view.control, placeholder)
		}
		view.popupWidth = max(0, int(popupWidth))
		view.sizeSelectPopup()
	}
}
func gkGTKSelectSetVisibleRows(pointer unsafe.Pointer, rows cInt) {
	if view := viewState(pointer); view != nil {
		view.visibleRows = max(0, int(rows))
		view.sizeSelectPopup()
	}
}
func gkGTKSliderConfigure(pointer unsafe.Pointer, vertical cInt, minimum, maximum, frequency cDouble, snap cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	view.vertical = vertical != 0
	orientation := gtkOrientationHorizontal
	w, h := cInt(96), cInt(-1)
	position := gtkPosBottom
	if view.vertical {
		orientation, w, h, position = gtkOrientationVertical, -1, 96, gtkPosRight
	}
	gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(view.control)), orientation)
	gtkWidgetSetSizeRequest(view.control, w, h)
	gtkRangeSetInverted((*gtkRange)(unsafe.Pointer(view.control)), nativeBool(view.vertical != view.inverted))
	scale := (*gtkScale)(unsafe.Pointer(view.control))
	gtkScaleClearMarks(scale)
	if frequency > 0 && maximum > minimum {
		count := min(1000, int(math.Floor(float64((maximum-minimum)/frequency)))+1)
		for i := 0; i < count; i++ {
			gtkScaleAddMark(scale, minimum+cDouble(i)*frequency, position, nil)
		}
	}
	view.value = 0
	if frequency > 0 && snap != 0 {
		view.value = float64(frequency)
	}
}
func gkGTKSliderSetInverted(pointer unsafe.Pointer, inverted cInt) {
	if view := viewState(pointer); view != nil {
		view.inverted = inverted != 0
		gtkRangeSetInverted((*gtkRange)(unsafe.Pointer(view.control)), nativeBool(view.vertical != view.inverted))
	}
}
func gkGTKProgressConfigure(pointer unsafe.Pointer, indeterminate, vertical cInt, text *cChar, state cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	progress := (*gtkProgressBar)(unsafe.Pointer(view.control))
	view.running = indeterminate != 0
	if view.running && view.progressTick == 0 {
		gtkProgressBarSetPulseStep(progress, 0.1)
		var last int64
		view.progressTick = gtkWidgetAddTickCallback(view.control, gtkTickCallbackAddress(), newNativeCallback(func(clock *gdkFrameClock) bool {
			if !view.running {
				view.progressTick = 0
				return false
			}
			now := int64(gdkFrameClockGetFrameTime(clock))
			if now-last > 100000 {
				last = now
				gtkProgressBarPulse(progress)
			}
			return true
		}), gtkDestroyCallback())
	} else if !view.running {
		gtkProgressBarSetFraction(progress, cDouble(view.value))
	}
	orientation := gtkOrientationHorizontal
	valign, halign := gtkAlignCenter, gtkAlignFill
	if vertical != 0 {
		orientation, valign, halign = gtkOrientationVertical, gtkAlignFill, gtkAlignCenter
	}
	gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(view.control)), orientation)
	gtkProgressBarSetInverted(progress, nativeBool(vertical != 0))
	gtkWidgetSetValign(view.control, valign)
	gtkWidgetSetHalign(view.control, halign)
	gtkProgressBarSetText(progress, text)
	gtkProgressBarSetShowText(progress, nativeBool(goString(text) != ""))
	setOptionalAccessibleText(view.control, gtkAccessiblePropertyValueText, goString(text))
	classes := []string{"", "gk-paused", "gk-warning", "gk-error"}
	colors := []string{"", "alpha(@theme_fg_color, 0.4)", "@warning_color", "@error_color"}
	for i := 1; i < len(classes); i++ {
		gtkWidgetRemoveCSSClass(view.control, stringPointer(classes[i]))
		if int(state) == i {
			gtkWidgetAddCSSClass(view.control, stringPointer(classes[i]))
		}
		setNativeCSS("progressbar."+classes[i]+" > trough > progress", "background-color: "+colors[i]+";")
	}
}
func gkGTKActivityConfigure(pointer unsafe.Pointer, size cInt, label *cChar) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	sizes := [...]cInt{24, 12, 16, 48}
	index := int(size)
	if index < 0 || index >= len(sizes) {
		index = 0
	}
	gtkWidgetSetSizeRequest(view.control, sizes[index], sizes[index])
	gtkWidgetSetTooltipText(view.control, label)
	setOptionalAccessibleText(view.control, gtkAccessiblePropertyLabel, goString(label))
}
func gkGTKSeparatorConfigure(pointer unsafe.Pointer, vertical cInt, label *cChar, inset, thickness cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	orientation := gtkOrientationHorizontal
	view.vertical = vertical != 0
	if view.vertical {
		orientation = gtkOrientationVertical
	}
	gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(view.outer)), orientation)
	for _, line := range []*gtkWidget{view.control, view.end} {
		gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(line)), orientation)
		gtkWidgetSetHexpand(line, nativeBool(!view.vertical))
		gtkWidgetSetVexpand(line, nativeBool(view.vertical))
		valign, halign := gtkAlignCenter, gtkAlignFill
		if view.vertical {
			valign, halign = gtkAlignFill, gtkAlignCenter
		}
		gtkWidgetSetValign(line, valign)
		gtkWidgetSetHalign(line, halign)
	}
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(view.label)), label)
	gtkWidgetSetVisible(view.label, nativeBool(goString(label) != ""))
	gtkWidgetSetVisible(view.end, nativeBool(goString(label) != ""))
	if goString(label) != "" {
		setAccessibleText(view.outer, gtkAccessiblePropertyLabel, goString(label))
	}
	margin := cInt(0)
	if inset != 0 {
		margin = 16
	}
	top, side := cInt(0), margin
	dimension := "min-height"
	if view.vertical {
		top, side = margin, 0
		dimension = "min-width"
	}
	gtkWidgetSetMarginTop(view.outer, top)
	gtkWidgetSetMarginBottom(view.outer, top)
	gtkWidgetSetMarginStart(view.outer, side)
	gtkWidgetSetMarginEnd(view.outer, side)
	setNativeCSS("."+nativeWidgetCSSClass(view)+" > separator", fmt.Sprintf("%s: %dpx;", dimension, max(1, thickness)))
}
func gkGTKImageConfigure(pointer unsafe.Pointer, alignment, nearest cInt, alternative *cChar, scale cDouble) {
	if view := viewState(pointer); view != nil {
		view.alignment, view.nearest = int(alignment), nearest != 0
		view.sourceScale = float64(scale)
		if scale <= 0 {
			view.sourceScale = 1
		}
		setOptionalAccessibleText(view.outer, gtkAccessiblePropertyLabel, goString(alternative))
		setAccessibleBoolState(view.outer, gtkAccessibleStateHidden, goString(alternative) == "")
		gtkWidgetQueueResize(view.outer)
		gtkWidgetQueueDraw(view.outer)
	}
}
func gkGTKLinkConfigure(pointer unsafe.Pointer, visited cInt, icon unsafe.Pointer) {
	if view := viewState(pointer); view != nil {
		gtkLinkButtonSetVisited((*gtkLinkButton)(unsafe.Pointer(view.control)), nativeBool(visited != 0))
		parts := view.buttonContent()
		gtkWidgetSetHalign(parts.box, gtkAlignStart)
		gtkImageSetFromPaintable((*gtkImage)(unsafe.Pointer(parts.icon)), (*gdkPaintable)(icon))
		gtkWidgetSetVisible(parts.icon, nativeBool(icon != nil))
	}
}
func gkGTKTabsConfigure(pointer unsafe.Pointer, placement cInt) {
	if view := viewState(pointer); view != nil {
		positions := []gtkPositionType{gtkPosTop, gtkPosBottom, gtkPosLeft, gtkPosRight}
		index := int(placement)
		if index < 0 || index >= len(positions) {
			index = 0
		}
		gtkNotebookSetTabPos((*gtkNotebook)(unsafe.Pointer(view.outer)), positions[index])
	}
}
func gkGTKTabsSetOverflow(pointer unsafe.Pointer, policy cInt) {
	if view := viewState(pointer); view != nil {
		notebook := (*gtkNotebook)(unsafe.Pointer(view.outer))
		gtkNotebookSetScrollable(notebook, 1)
		if policy == 0 {
			gtkNotebookPopupEnable(notebook)
		} else {
			gtkNotebookPopupDisable(notebook)
		}
	}
}
