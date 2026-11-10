//go:build linux && cgo

package gtk

import (
	"math"
	"unicode"
	"unsafe"
)

type nativeControl struct {
	widget, control, label, popover, list, button *gtkWidget
	on, off, display                              string
	buttons                                       []*gtkWidget
	mode, clicked                                 int
	suppress, recording, systemFont               bool
}

func newNativeControl(window cUintptr, widget, control *gtkWidget, kind int) *nativeControl {
	if control == nil {
		control = widget
	}
	state := &nativeControl{widget: widget, control: control, clicked: -1}
	registerNativeComponent(uintptr(window), widget, control, kind)
	storeComponentState(widget, state)
	return state
}
func (state *nativeControl) report(event gtkEvent) {
	if !state.suppress {
		sendComponentEvent(state.widget, event)
	}
}
func gkGTKToggleButtonCreate(window cUintptr, text *cChar, icon unsafe.Pointer, checked, toolbar cInt) unsafe.Pointer {
	widget := gtkToggleButtonNewWithLabel(text)
	if toolbar != 0 {
		gtkWidgetAddCSSClass(widget, stringPointer("flat"))
	}
	gtkToggleButtonSetActive((*gtkToggleButton)(unsafe.Pointer(widget)), nativeBool(checked != 0))
	state := newNativeControl(window, widget, nil, kindToggle)
	if icon != nil {
		configureComponentButton(widget, (*gdkPaintable)(icon))
	}
	connectNativeSignal(unsafe.Pointer(widget), "toggled", func() { state.report(gtkEvent{kind: gkEventValue}) })
	return unsafe.Pointer(widget)
}
func (state *nativeControl) updateSwitch() {
	text := state.off
	if gtkSwitchGetActive((*gtkSwitch)(unsafe.Pointer(state.control))) != 0 {
		text = state.on
	}
	if text != "" {
		setAccessibleText(state.control, gtkAccessiblePropertyValueText, text)
	} else {
		gtkAccessibleResetProperty((*gtkAccessible)(unsafe.Pointer(state.control)), gtkAccessiblePropertyValueText)
	}
}
func gkGTKSwitchCreate(window cUintptr, text, on, off *cChar, checked, size cInt) unsafe.Pointer {
	box := gtkBoxNew(gtkOrientationHorizontal, 8)
	label := gtkLabelNew(text)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(label)), 0)
	gtkWidgetSetHexpand(label, 1)
	gtkWidgetSetVisible(label, nativeBool(goString(text) != ""))
	toggle := gtkSwitchNew()
	gtkWidgetSetValign(toggle, gtkAlignCenter)
	gtkSwitchSetActive((*gtkSwitch)(unsafe.Pointer(toggle)), nativeBool(checked != 0))
	if size == 1 || size == 2 {
		gtkWidgetAddCSSClass(toggle, stringPointer("gk-small-switch"))
		setComponentCSS("switch.gk-small-switch", "transform: scale(0.8);")
	}
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), label)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), toggle)
	state := newNativeControl(window, box, toggle, kindSwitch)
	state.label, state.on, state.off = label, goString(on), goString(off)
	setComponentLabel(box, label)
	setAccessibleText(toggle, gtkAccessiblePropertyLabel, goString(text))
	setAccessibleRelation(toggle, gtkAccessibleRelationLabelledBy, []*gtkWidget{label})
	state.updateSwitch()
	connectNativeNotify(unsafe.Pointer(toggle), "notify::active", func() { state.updateSwitch(); state.report(gtkEvent{kind: gkEventValue}) })
	return unsafe.Pointer(box)
}
func gkGTKSwitchChecked(pointer unsafe.Pointer) cInt {
	if state := componentState[nativeControl](pointer); state != nil && gtkSwitchGetActive((*gtkSwitch)(unsafe.Pointer(state.control))) != 0 {
		return 1
	}
	return 0
}
func gkGTKSwitchSet(pointer unsafe.Pointer, text *cChar, checked cInt) {
	if state := componentState[nativeControl](pointer); state != nil {
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.label)), text)
		gtkWidgetSetVisible(state.label, nativeBool(goString(text) != ""))
		setAccessibleText(state.control, gtkAccessiblePropertyLabel, goString(text))
		state.suppress = true
		gtkSwitchSetActive((*gtkSwitch)(unsafe.Pointer(state.control)), nativeBool(checked != 0))
		state.suppress = false
		state.updateSwitch()
	}
}
func (state *nativeControl) textSignals() {
	connectNativeSignal(unsafe.Pointer(state.control), "changed", func() { state.report(gtkEvent{kind: gkEventTextChange}) })
	connectNativeSignal(unsafe.Pointer(state.control), "activate", func() { state.report(gtkEvent{kind: gkEventSubmit}) })
}
func gkGTKPasswordCreate(window cUintptr, placeholder *cChar, reveal cInt) unsafe.Pointer {
	widget := gtkPasswordEntryNew()
	gtkPasswordEntrySetShowPeekIcon((*gtkPasswordEntry)(unsafe.Pointer(widget)), nativeBool(reveal != 0))
	setNativeProperty(unsafe.Pointer(widget), "placeholder-text", goString(placeholder))
	setNativeProperty(unsafe.Pointer(widget), "activates-default", true)
	state := newNativeControl(window, widget, nil, kindPassword)
	state.textSignals()
	if text := nativeDescendant(widget, gtkTextGetType()); text != nil {
		connectNativeNotify(unsafe.Pointer(text), "notify::visibility", func() {
			visible := 0
			if gtkTextGetVisibility((*gtkText)(unsafe.Pointer(text))) != 0 {
				visible = 1
			}
			state.report(gtkEvent{kind: gkEventExpand, a: visible})
		})
	}
	return unsafe.Pointer(widget)
}
func gkGTKPasswordText(pointer unsafe.Pointer) *cChar {
	return gStrdup(gtkEditableGetText((*gtkEditable)(unsafe.Pointer(componentControl((*gtkWidget)(pointer))))))
}
func gkGTKPasswordSet(pointer unsafe.Pointer, text, placeholder *cChar, revealed cInt) {
	state := componentState[nativeControl](pointer)
	if state == nil {
		return
	}
	state.suppress = true
	defer func() { state.suppress = false }()
	if goString(gtkEditableGetText((*gtkEditable)(unsafe.Pointer(state.control)))) != goString(text) {
		gtkEditableSetText((*gtkEditable)(unsafe.Pointer(state.control)), text)
	}
	setNativeProperty(unsafe.Pointer(state.control), "placeholder-text", goString(placeholder))
	if field := nativeDescendant(state.control, gtkTextGetType()); field != nil {
		gtkTextSetVisibility((*gtkText)(unsafe.Pointer(field)), nativeBool(revealed != 0))
	}
}
func gkGTKNumberCreate(window cUintptr, minimum, maximum, value, step cDouble, decimals cInt) unsafe.Pointer {
	if step <= 0 {
		step = 1
	}
	widget := gtkSpinButtonNewWithRange(minimum, maximum, step)
	spin := (*gtkSpinButton)(unsafe.Pointer(widget))
	gtkSpinButtonSetDigits(spin, cUint(max(0, decimals)))
	gtkSpinButtonSetNumeric(spin, 1)
	gtkSpinButtonSetValue(spin, value)
	state := newNativeControl(window, widget, nil, kindNumber)
	connectNativeSignal(unsafe.Pointer(widget), "value-changed", func() { state.report(gtkEvent{kind: gkEventValue, x: float64(gtkSpinButtonGetValue(spin))}) })
	return unsafe.Pointer(widget)
}
func gkGTKNumberValue(pointer unsafe.Pointer) cDouble {
	return gtkSpinButtonGetValue((*gtkSpinButton)(unsafe.Pointer(componentControl((*gtkWidget)(pointer)))))
}
func gkGTKNumberSetValue(pointer unsafe.Pointer, value cDouble) {
	if state := componentState[nativeControl](pointer); state != nil {
		state.suppress = true
		gtkSpinButtonSetValue((*gtkSpinButton)(unsafe.Pointer(state.control)), value)
		state.suppress = false
	}
}
func (state *nativeControl) fillSuggestions(items *cChar, size cSize) {
	gtkListBoxRemoveAll((*gtkListBox)(unsafe.Pointer(state.list)))
	values := splitNul(unsafe.Pointer(items), size)
	for _, value := range values {
		label := gtkLabelNew(stringPointer(value))
		gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(label)), 0)
		gtkListBoxAppend((*gtkListBox)(unsafe.Pointer(state.list)), label)
	}
	if state.button != nil {
		gtkWidgetSetSensitive(state.button, nativeBool(len(values) > 0))
	}
}
func (state *nativeControl) suggestionSignals(search bool) {
	connectNativeArgument(unsafe.Pointer(state.list), "row-activated", gtkObjectSignalCallback(), func(object unsafe.Pointer) {
		label := gtkListBoxRowGetChild((*gtkListBoxRow)(object))
		text := goString(gtkLabelGetText((*gtkLabel)(unsafe.Pointer(label))))
		state.suppress = true
		gtkEditableSetText((*gtkEditable)(unsafe.Pointer(state.control)), stringPointer(text))
		state.suppress = false
		if search {
			gtkPopoverPopdown((*gtkPopover)(unsafe.Pointer(state.popover)))
		} else {
			gtkMenuButtonPopdown((*gtkMenuButton)(unsafe.Pointer(state.button)))
		}
		state.report(gtkEvent{kind: gkEventTextChange, text: text})
		if search {
			state.report(gtkEvent{kind: gkEventSubmit})
		}
	})
}
func gkGTKSearchCreate(window cUintptr, text, placeholder *cChar) unsafe.Pointer {
	widget := gtkSearchEntryNew()
	gtkEditableSetText((*gtkEditable)(unsafe.Pointer(widget)), text)
	gtkSearchEntrySetPlaceholderText((*gtkSearchEntry)(unsafe.Pointer(widget)), placeholder)
	state := newNativeControl(window, widget, nil, kindSearch)
	state.popover, state.list = gtkPopoverNew(), gtkListBoxNew()
	gtkPopoverSetHasArrow((*gtkPopover)(unsafe.Pointer(state.popover)), 0)
	gtkWidgetSetParent(state.popover, widget)
	gtkPopoverSetChild((*gtkPopover)(unsafe.Pointer(state.popover)), state.list)
	state.suggestionSignals(true)
	connectNativeSignal(unsafe.Pointer(widget), "search-changed", func() {
		if !state.suppress && gtkWidgetGetFirstChild(state.list) != nil {
			if field := nativeDescendant(widget, gtkTextGetType()); field != nil && gtkWidgetHasFocus(field) != 0 {
				gtkPopoverPopup((*gtkPopover)(unsafe.Pointer(state.popover)))
			}
		}
		state.report(gtkEvent{kind: gkEventTextChange})
	})
	connectNativeSignal(unsafe.Pointer(widget), "activate", func() { state.report(gtkEvent{kind: gkEventSubmit}) })
	return unsafe.Pointer(widget)
}
func gkGTKSearchSetSuggestions(pointer unsafe.Pointer, items *cChar, size cSize) {
	if state := componentState[nativeControl](pointer); state != nil {
		state.fillSuggestions(items, size)
	}
}
func gkGTKComboCreate(window cUintptr, text, items *cChar, size cSize) unsafe.Pointer {
	box := boxWithRole(gtkOrientationHorizontal, 0, gtkAccessibleRoleComboBox)
	gtkWidgetAddCSSClass(box, stringPointer("linked"))
	entry, button := gtkEntryNew(), gtkMenuButtonNew()
	gtkWidgetSetHexpand(entry, 1)
	gtkEditableSetText((*gtkEditable)(unsafe.Pointer(entry)), text)
	state := newNativeControl(window, box, entry, kindCombo)
	state.button, state.popover, state.list = button, gtkPopoverNew(), gtkListBoxNew()
	scroll := gtkScrolledWindowNew()
	gtkScrolledWindowSetPolicy((*gtkScrolledWindow)(unsafe.Pointer(scroll)), gtkPolicyNever, gtkPolicyAutomatic)
	gtkScrolledWindowSetPropagateNaturalHeight((*gtkScrolledWindow)(unsafe.Pointer(scroll)), 1)
	gtkScrolledWindowSetMaxContentHeight((*gtkScrolledWindow)(unsafe.Pointer(scroll)), 300)
	gtkScrolledWindowSetChild((*gtkScrolledWindow)(unsafe.Pointer(scroll)), state.list)
	gtkPopoverSetChild((*gtkPopover)(unsafe.Pointer(state.popover)), scroll)
	gtkMenuButtonSetPopover((*gtkMenuButton)(unsafe.Pointer(button)), state.popover)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), entry)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), button)
	state.fillSuggestions(items, size)
	state.suggestionSignals(false)
	state.textSignals()
	focus := gtkEventControllerFocusNew()
	connectNativeSignal(unsafe.Pointer(focus), "leave", func() { state.report(gtkEvent{kind: gkEventSubmit}) })
	gtkWidgetAddController(entry, focus)
	return unsafe.Pointer(box)
}
func gkGTKComboSetItems(pointer unsafe.Pointer, items *cChar, size cSize) {
	if state := componentState[nativeControl](pointer); state != nil {
		state.fillSuggestions(items, size)
	}
}

func gkGTKSegmentedCreate(window cUintptr, labels *cChar, size cSize, icons *unsafe.Pointer, enabled *cInt, count, mode cInt) unsafe.Pointer {
	box := boxWithRole(gtkOrientationHorizontal, 0, gtkAccessibleRoleRadioGroup)
	gtkWidgetAddCSSClass(box, stringPointer("linked"))
	state := newNativeControl(window, box, nil, kindSegmented)
	state.mode = int(mode)
	titles := splitNul(unsafe.Pointer(labels), size)
	paintables := borrowedPaintables(icons, int(count))
	flags := unsafe.Slice(enabled, int(count))
	for i := range int(count) {
		var button *gtkWidget
		if mode == 0 {
			button = gtkButtonNew()
		} else {
			button = gtkToggleButtonNew()
		}
		title := ""
		if i < len(titles) {
			title = titles[i]
		}
		if paintables[i] != nil {
			row := gtkBoxNew(gtkOrientationHorizontal, 4)
			image := gtkImageNewFromPaintable(paintables[i])
			gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(image)), 16)
			gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), image)
			if title != "" {
				gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), gtkLabelNew(stringPointer(title)))
			}
			gtkButtonSetChild((*gtkButton)(unsafe.Pointer(button)), row)
		} else {
			gtkButtonSetLabel((*gtkButton)(unsafe.Pointer(button)), stringPointer(title))
		}
		if mode == 1 && len(state.buttons) > 0 {
			gtkToggleButtonSetGroup((*gtkToggleButton)(unsafe.Pointer(button)), (*gtkToggleButton)(unsafe.Pointer(state.buttons[0])))
		}
		gtkWidgetSetSensitive(button, nativeBool(flags[i] != 0))
		gtkWidgetSetHexpand(button, 1)
		state.buttons = append(state.buttons, button)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), button)
		connectNativeSignal(unsafe.Pointer(button), "clicked", func() {
			state.clicked = i
			if mode == 0 {
				state.report(gtkEvent{kind: gkEventSelection})
			}
		})
		if mode != 0 {
			connectNativeSignal(unsafe.Pointer(button), "toggled", func() {
				if mode != 1 || gtkToggleButtonGetActive((*gtkToggleButton)(unsafe.Pointer(button))) != 0 {
					state.report(gtkEvent{kind: gkEventSelection})
				}
			})
		}
	}
	return unsafe.Pointer(box)
}
func gkGTKSegmentSelected(pointer unsafe.Pointer, index cInt) cInt {
	state := componentState[nativeControl](pointer)
	if state == nil || state.mode == 0 || index < 0 || int(index) >= len(state.buttons) {
		return 0
	}
	if gtkToggleButtonGetActive((*gtkToggleButton)(unsafe.Pointer(state.buttons[index]))) != 0 {
		return 1
	}
	return 0
}
func gkGTKSegmentClicked(pointer unsafe.Pointer) cInt {
	if state := componentState[nativeControl](pointer); state != nil {
		return cInt(state.clicked)
	}
	return -1
}
func gkGTKSegmentSetSelected(pointer unsafe.Pointer, index, selected cInt) {
	state := componentState[nativeControl](pointer)
	if state == nil || state.mode == 0 || index < 0 || int(index) >= len(state.buttons) {
		return
	}
	state.suppress = true
	gtkToggleButtonSetActive((*gtkToggleButton)(unsafe.Pointer(state.buttons[index])), nativeBool(selected != 0))
	state.suppress = false
}

type nativeValueControl struct {
	widget                               *gtkWidget
	minimum, maximum, value, upper, step float64
	wraps, isRange                       bool
	dragging, focused                    int
	originX, originY                     float64
}

func (state *nativeValueControl) snap(value float64) float64 {
	if state.step > 0 {
		value = state.minimum + math.Round((value-state.minimum)/state.step)*state.step
	}
	return max(state.minimum, min(state.maximum, value))
}
func (state *nativeValueControl) accessible() {
	setAccessibleNumber(state.widget, gtkAccessiblePropertyValueMin, state.minimum)
	setAccessibleNumber(state.widget, gtkAccessiblePropertyValueMax, state.maximum)
	setAccessibleNumber(state.widget, gtkAccessiblePropertyValueNow, state.value)
}
func (state *nativeValueControl) changed() {
	state.accessible()
	gtkWidgetQueueDraw(state.widget)
	sendComponentEvent(state.widget, gtkEvent{kind: gkEventValue, x: state.value, y: state.upper})
}
func (state *nativeValueControl) stepBy(direction float64) {
	step := state.step
	if step <= 0 {
		step = 1
	}
	value := state.value + direction*step
	if value > state.maximum {
		value = state.maximum
		if state.wraps {
			value = state.minimum
		}
	} else if value < state.minimum {
		value = state.minimum
		if state.wraps {
			value = state.maximum
		}
	}
	if value != state.value {
		state.value = value
		state.changed()
	}
}
func gkGTKStepperCreate(window cUintptr, minimum, maximum, value, step cDouble, wraps cInt) unsafe.Pointer {
	box := boxWithRole(gtkOrientationHorizontal, 0, gtkAccessibleRoleSpinButton)
	gtkWidgetAddCSSClass(box, stringPointer("linked"))
	down, up := gtkButtonNewFromIconName(stringPointer("list-remove-symbolic")), gtkButtonNewFromIconName(stringPointer("list-add-symbolic"))
	setAccessibleText(down, gtkAccessiblePropertyLabel, "Decrease")
	setAccessibleText(up, gtkAccessiblePropertyLabel, "Increase")
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), down)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), up)
	state := &nativeValueControl{
		widget:  box,
		minimum: float64(minimum),
		maximum: float64(maximum),
		value:   float64(max(minimum, min(maximum, value))),
		step:    float64(step),
		wraps:   wraps != 0,
	}
	registerNativeComponent(uintptr(window), box, nil, kindStepper)
	storeComponentState(box, state)
	state.accessible()
	connectNativeSignal(unsafe.Pointer(up), "clicked", func() { state.stepBy(1) })
	connectNativeSignal(unsafe.Pointer(down), "clicked", func() { state.stepBy(-1) })
	return unsafe.Pointer(box)
}
func (state *nativeValueControl) dialSweep() float64 {
	if state.wraps {
		return 2 * math.Pi
	}
	return 300 * math.Pi / 180
}
func (state *nativeValueControl) dialStart() float64 {
	if state.wraps {
		return -math.Pi / 2
	}
	return 120 * math.Pi / 180
}
func (state *nativeValueControl) fraction(value float64) float64 {
	if state.maximum <= state.minimum {
		return 0
	}
	return (value - state.minimum) / (state.maximum - state.minimum)
}
func (state *nativeValueControl) dialPoint(x, y float64) {
	width, height := float64(gtkWidgetGetWidth(state.widget)), float64(gtkWidgetGetHeight(state.widget))
	angle := math.Atan2(y-height/2, x-width/2)
	sweep := state.dialSweep()
	offset := math.Mod(angle-state.dialStart()+4*math.Pi, 2*math.Pi)
	if offset > sweep {
		if offset-sweep < (2*math.Pi-sweep)/2 {
			offset = sweep
		} else {
			offset = 0
		}
	}
	value := state.snap(state.minimum + offset/sweep*(state.maximum-state.minimum))
	if state.wraps && value >= state.maximum {
		value = state.minimum
	}
	if value != state.value {
		state.value = value
		state.changed()
	}
}
func (state *nativeValueControl) drawDial(context *cairoContext, width, height int) {
	var color gdkRGBA
	gtkWidgetGetColor(state.widget, &color)
	radius := float64(min(width, height))/2 - 3
	cx, cy := float64(width)/2, float64(height)/2
	start := state.dialStart()
	angle := start + state.fraction(state.value)*state.dialSweep()
	cairoSetLineWidth(context, 2)
	cairoSetSourceRgba(context, cDouble(color.red), cDouble(color.green), cDouble(color.blue), 0.25)
	cairoArc(context, cDouble(cx), cDouble(cy), cDouble(radius), 0, 2*math.Pi)
	cairoStroke(context)
	alpha := cDouble(1)
	if gtkWidgetIsSensitive(state.widget) == 0 {
		alpha = 0.4
	}
	cairoSetSourceRgba(context, cDouble(color.red), cDouble(color.green), cDouble(color.blue), alpha)
	cairoArc(context, cDouble(cx), cDouble(cy), cDouble(radius), cDouble(start), cDouble(angle))
	cairoStroke(context)
	cairoMoveTo(context, cDouble(cx), cDouble(cy))
	cairoLineTo(context, cDouble(cx+math.Cos(angle)*radius*0.8), cDouble(cy+math.Sin(angle)*radius*0.8))
	cairoSetLineWidth(context, 3)
	cairoStroke(context)
}
func (state *nativeValueControl) rangePosition(width int, value float64) float64 {
	return 9 + state.fraction(value)*float64(max(1, width-18))
}
func (state *nativeValueControl) rangeMove(x float64) {
	width := int(gtkWidgetGetWidth(state.widget))
	value := state.snap(state.minimum + (x-9)/float64(max(1, width-18))*(state.maximum-state.minimum))
	if state.dragging == 0 {
		state.value = min(value, state.upper)
	} else {
		state.upper = max(value, state.value)
	}
	state.changed()
}
func (state *nativeValueControl) drawRange(context *cairoContext, width, height int) {
	var color gdkRGBA
	gtkWidgetGetColor(state.widget, &color)
	mid := float64(height) / 2
	lower, upper := state.rangePosition(width, state.value), state.rangePosition(width, state.upper)
	cairoSetLineWidth(context, 4)
	cairoSetSourceRgba(context, cDouble(color.red), cDouble(color.green), cDouble(color.blue), 0.2)
	cairoMoveTo(context, 9, cDouble(mid))
	cairoLineTo(context, cDouble(width-9), cDouble(mid))
	cairoStroke(context)
	cairoSetSourceRgba(context, cDouble(color.red), cDouble(color.green), cDouble(color.blue), 0.8)
	cairoMoveTo(context, cDouble(lower), cDouble(mid))
	cairoLineTo(context, cDouble(upper), cDouble(mid))
	cairoStroke(context)
	for knob, x := range []float64{lower, upper} {
		cairoArc(context, cDouble(x), cDouble(mid), 8, 0, 2*math.Pi)
		cairoSetSourceRgba(context, 1, 1, 1, 1)
		cairoFillPreserve(context)
		focused := gtkWidgetHasFocus(state.widget) != 0 && state.focused == knob
		alpha, line := cDouble(0.3), cDouble(1)
		if focused {
			alpha, line = 0.9, 2
		}
		cairoSetSourceRgba(context, cDouble(color.red), cDouble(color.green), cDouble(color.blue), alpha)
		cairoSetLineWidth(context, line)
		cairoStroke(context)
	}
}
func newDrawingValue(window cUintptr, minimum, maximum, value, step cDouble, isRange bool) *nativeValueControl {
	widget := widgetWithRole(gtkDrawingAreaGetType(), gtkAccessibleRoleSlider)
	state := &nativeValueControl{
		widget:  widget,
		minimum: float64(minimum),
		maximum: float64(maximum),
		value:   float64(max(minimum, min(maximum, value))),
		step:    float64(step),
		isRange: isRange,
	}
	width, height, kind := 32, 32, kindDial
	if isRange {
		width, height, kind = 160, 22, kindRange
	}
	gtkDrawingAreaSetContentWidth((*gtkDrawingArea)(unsafe.Pointer(widget)), cInt(width))
	gtkDrawingAreaSetContentHeight((*gtkDrawingArea)(unsafe.Pointer(widget)), cInt(height))
	registerNativeComponent(uintptr(window), widget, nil, kind)
	storeComponentState(widget, state)
	draw := state.drawDial
	if isRange {
		draw = state.drawRange
	}
	gtkDrawingAreaSetDrawFunc((*gtkDrawingArea)(unsafe.Pointer(widget)), gtkDrawCallback(), newNativeCallback(draw), gtkDestroyCallback())
	gtkWidgetSetFocusable(widget, 1)
	keys := gtkEventControllerKeyNew()
	connectNativeArgument(unsafe.Pointer(keys), "key-pressed", gtkKeySignalCallback(), func(keyval, keycode uint32, modifiers gdkModifierType) bool {
		step := state.step
		if step <= 0 {
			step = (state.maximum - state.minimum) / 100
		}
		delta := 0.0
		switch keyval {
		case gdkKeyLeft, gdkKeyDown:
			delta = -step
		case gdkKeyRight, gdkKeyUp:
			delta = step
		case gdkKeyTab:
			if state.isRange && state.focused == 0 {
				state.focused = 1
				gtkWidgetQueueDraw(widget)
				return true
			}
			return false
		default:
			return false
		}
		if state.isRange {
			if state.focused == 0 {
				state.value = max(state.minimum, min(state.upper, state.value+delta))
			} else {
				state.upper = min(state.maximum, max(state.value, state.upper+delta))
			}
		} else {
			state.value = max(state.minimum, min(state.maximum, state.value+delta))
		}
		state.changed()
		return true
	})
	gtkWidgetAddController(widget, keys)
	drag := gtkGestureDragNew()
	connectNativeArgument(unsafe.Pointer(drag), "drag-begin", gtkXYSignalCallback(), func(x, y float64) {
		state.originX, state.originY = x, y
		gtkWidgetGrabFocus(widget)
		if state.isRange {
			width := int(gtkWidgetGetWidth(widget))
			lower, upper := state.rangePosition(width, state.value), state.rangePosition(width, state.upper)
			state.dragging = 0
			if math.Abs(x-upper) < math.Abs(x-lower) || lower == upper && x >= lower {
				state.dragging = 1
			}
			state.focused = state.dragging
			state.rangeMove(x)
		} else {
			state.dialPoint(x, y)
		}
	})
	connectNativeArgument(unsafe.Pointer(drag), "drag-update", gtkXYSignalCallback(), func(x, y float64) {
		if state.isRange {
			state.rangeMove(state.originX + x)
		} else {
			state.dialPoint(state.originX+x, state.originY+y)
		}
	})
	gtkWidgetAddController(widget, (*gtkEventController)(unsafe.Pointer(drag)))
	return state
}
func gkGTKDialCreate(window cUintptr, minimum, maximum, value, step cDouble, wraps cInt) unsafe.Pointer {
	state := newDrawingValue(window, minimum, maximum, value, step, false)
	state.wraps = wraps != 0
	state.accessible()
	return unsafe.Pointer(state.widget)
}
func gkGTKControlValue(pointer unsafe.Pointer) cDouble {
	if state := componentState[nativeValueControl](pointer); state != nil {
		return cDouble(state.value)
	}
	return 0
}
func gkGTKControlSetValue(pointer unsafe.Pointer, value cDouble) {
	if state := componentState[nativeValueControl](pointer); state != nil {
		state.value = float64(value)
		state.accessible()
		gtkWidgetQueueDraw(state.widget)
	}
}
func gkGTKRangeSliderCreate(window cUintptr, minimum, maximum, lower, upper, step cDouble) unsafe.Pointer {
	state := newDrawingValue(window, minimum, maximum, lower, step, true)
	state.upper = max(state.value, min(float64(maximum), float64(upper)))
	state.accessible()
	return unsafe.Pointer(state.widget)
}
func gkGTKRangeSliderValues(pointer unsafe.Pointer, lower, upper *cDouble) {
	*lower, *upper = 0, 0
	if state := componentState[nativeValueControl](pointer); state != nil {
		*lower, *upper = cDouble(state.value), cDouble(state.upper)
	}
}
func gkGTKRangeSliderSet(pointer unsafe.Pointer, lower, upper cDouble) {
	if state := componentState[nativeValueControl](pointer); state != nil {
		state.value, state.upper = float64(lower), float64(upper)
		state.accessible()
		gtkWidgetQueueDraw(state.widget)
	}
}

func gkGTKColorWellCreate(window cUintptr, r, g, b, a cDouble, allowsAlpha cInt) unsafe.Pointer {
	dialog := gtkColorDialogNew()
	gtkColorDialogSetWithAlpha(dialog, nativeBool(allowsAlpha != 0))
	widget := gtkColorDialogButtonNew(dialog)
	color := gdkRGBA{red: cFloat(r), green: cFloat(g), blue: cFloat(b), alpha: cFloat(a)}
	gtkColorDialogButtonSetRgba((*gtkColorDialogButton)(unsafe.Pointer(widget)), &color)
	state := newNativeControl(window, widget, nil, kindColor)
	connectNativeNotify(unsafe.Pointer(widget), "notify::rgba", func() { state.report(gtkEvent{kind: gkEventValue}) })
	return unsafe.Pointer(widget)
}
func gkGTKColorWellColor(pointer unsafe.Pointer, rgba *cDouble) {
	color := gtkColorDialogButtonGetRgba((*gtkColorDialogButton)(unsafe.Pointer(componentControl((*gtkWidget)(pointer)))))
	target := unsafe.Slice(rgba, 4)
	target[0], target[1], target[2], target[3] = cDouble(color.red), cDouble(color.green), cDouble(color.blue), cDouble(color.alpha)
}
func gkGTKColorWellSet(pointer unsafe.Pointer, r, g, b, a cDouble) {
	if state := componentState[nativeControl](pointer); state != nil {
		state.suppress = true
		color := gdkRGBA{red: cFloat(r), green: cFloat(g), blue: cFloat(b), alpha: cFloat(a)}
		gtkColorDialogButtonSetRgba((*gtkColorDialogButton)(unsafe.Pointer(state.control)), &color)
		state.suppress = false
	}
}
func nativeSystemFamily() string {
	name := "Sans 11"
	if settings := gtkSettingsGetDefault(); settings != nil {
		if value := getNativeStringProperty(unsafe.Pointer(settings), "gtk-font-name"); value != "" {
			name = value
		}
	}
	description := pangoFontDescriptionFromString(stringPointer(name))
	defer pangoFontDescriptionFree(description)
	family := goString(pangoFontDescriptionGetFamily(description))
	if family == "" {
		family = "Sans"
	}
	return family
}
func nativeFontDescription(family string, size float64, weight int, italic bool) *pangoFontDescription {
	if family == "" || family == "System" {
		family = nativeSystemFamily()
	}
	if size <= 0 {
		size = 11
	}
	if weight <= 0 {
		weight = pangoWeightNormal
	}
	description := pangoFontDescriptionNew()
	pangoFontDescriptionSetFamily(description, stringPointer(family))
	pangoFontDescriptionSetSize(description, cInt(size*pangoScale))
	pangoFontDescriptionSetWeight(description, pangoWeight(weight))
	style := pangoStyle(pangoStyleNormal)
	if italic {
		style = pangoStyleItalic
	}
	pangoFontDescriptionSetStyle(description, style)
	return description
}
func gkGTKFontPickerCreate(window cUintptr, family *cChar, size cDouble, weight, italic cInt) unsafe.Pointer {
	widget := gtkFontDialogButtonNew(gtkFontDialogNew())
	gtkFontDialogButtonSetLevel((*gtkFontDialogButton)(unsafe.Pointer(widget)), gtkFontLevelFont)
	state := newNativeControl(window, widget, nil, kindFont)
	gkGTKFontPickerSet(unsafe.Pointer(widget), family, size, weight, italic)
	connectNativeNotify(unsafe.Pointer(widget), "notify::font-desc", func() {
		if !state.suppress {
			state.systemFont = false
		}
		state.report(gtkEvent{kind: gkEventValue})
	})
	return unsafe.Pointer(widget)
}
func gkGTKFontPickerSet(pointer unsafe.Pointer, family *cChar, size cDouble, weight, italic cInt) {
	state := componentState[nativeControl](pointer)
	if state == nil || !nativeFontFamilyInstalled(goString(family)) {
		return
	}
	description := nativeFontDescription(goString(family), float64(size), int(weight), italic != 0)
	state.suppress = true
	gtkFontDialogButtonSetFontDesc((*gtkFontDialogButton)(unsafe.Pointer(state.control)), description)
	state.suppress = false
	pangoFontDescriptionFree(description)
	state.systemFont = goString(family) == "" || goString(family) == "System"
}
func gkGTKFontPickerFont(pointer unsafe.Pointer, size *cDouble, weight, italic *cInt) *cChar {
	state := componentState[nativeControl](pointer)
	if state == nil {
		*size, *weight, *italic = 0, 400, 0
		return gStrdup(stringPointer("System"))
	}
	description := gtkFontDialogButtonGetFontDesc((*gtkFontDialogButton)(unsafe.Pointer(state.control)))
	*size = cDouble(pangoFontDescriptionGetSize(description)) / pangoScale
	*weight = cInt(pangoFontDescriptionGetWeight(description))
	*italic = 0
	if pangoFontDescriptionGetStyle(description) != pangoStyleNormal {
		*italic = 1
	}
	family := goString(pangoFontDescriptionGetFamily(description))
	if state.systemFont {
		family = "System"
	}
	return gStrdup(stringPointer(family))
}
func (state *nativeControl) updateRecorder() {
	text := state.display
	if state.recording {
		text = "Type shortcut…"
	} else if text == "" {
		text = "Record Shortcut"
	}
	gtkButtonSetLabel((*gtkButton)(unsafe.Pointer(state.control)), stringPointer(text))
	setAccessibleText(state.control, gtkAccessiblePropertyLabel, "Shortcut")
	setAccessibleText(state.control, gtkAccessiblePropertyValueText, state.display)
}
func gkGTKKeyRecorderCreate(window cUintptr) unsafe.Pointer {
	widget := gtkButtonNew()
	state := newNativeControl(window, widget, nil, kindKeyRecorder)
	state.updateRecorder()
	connectNativeSignal(unsafe.Pointer(widget), "clicked", func() { state.recording = true; gtkWidgetGrabFocus(widget); state.updateRecorder() })
	keys := gtkEventControllerKeyNew()
	gtkEventControllerSetPropagationPhase(keys, gtkPhaseCapture)
	connectNativeArgument(unsafe.Pointer(keys), "key-pressed", gtkKeySignalCallback(), func(keyval, keycode uint32, modifiers gdkModifierType) bool {
		if !state.recording {
			return false
		}
		switch keyval {
		case gdkKeyShiftL, gdkKeyShiftR, gdkKeyControlL, gdkKeyControlR, gdkKeyAltL, gdkKeyAltR, gdkKeySuperL, gdkKeySuperR, gdkKeyMetaL, gdkKeyMetaR:
			return true
		}
		modifiers &= gdkControlMask | gdkAltMask | gdkShiftMask | gdkSuperMask
		state.recording = false
		if keyval == gdkKeyEscape && modifiers&(gdkControlMask|gdkAltMask) == 0 {
			state.updateRecorder()
			return true
		}
		if (keyval == gdkKeyBackSpace || keyval == gdkKeyDelete) && modifiers == 0 {
			state.updateRecorder()
			state.report(gtkEvent{kind: gkEventValue})
			return true
		}
		bits := 0
		if modifiers&gdkControlMask != 0 {
			bits |= 1
		}
		if modifiers&gdkSuperMask != 0 {
			bits |= 2
		}
		if modifiers&gdkAltMask != 0 {
			bits |= 4
		}
		if modifiers&gdkShiftMask != 0 {
			bits |= 8
		}
		character := rune(gdkKeyvalToUnicode(gdkKeyvalToLower(cUint(keyval))))
		text := ""
		if character >= 0x20 {
			text = string(unicode.ToUpper(character))
		}
		state.report(gtkEvent{kind: gkEventValue, a: bits, b: componentKeyCode(keycode), text: text})
		state.updateRecorder()
		return true
	})
	gtkWidgetAddController(widget, keys)
	focus := gtkEventControllerFocusNew()
	connectNativeSignal(unsafe.Pointer(focus), "leave", func() { state.recording = false; state.updateRecorder() })
	gtkWidgetAddController(widget, focus)
	return unsafe.Pointer(widget)
}
func gkGTKKeyRecorderSet(pointer unsafe.Pointer, display *cChar) {
	if state := componentState[nativeControl](pointer); state != nil {
		state.display = goString(display)
		state.updateRecorder()
	}
}

func (state *nativeControl) disposeNative() {
	if state.popover != nil && gtkWidgetGetParent(state.popover) == state.widget {
		gtkWidgetUnparent(state.popover)
		state.popover = nil
	}
}
