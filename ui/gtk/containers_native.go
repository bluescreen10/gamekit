//go:build linux && cgo

package gtk

import (
	"math"
	"unsafe"
)

type nativeGroupBox struct{ widget, title, subtitle, body *gtkWidget }

func gkGTKGroupBoxCreate(window cUintptr, title, subtitle *cChar) unsafe.Pointer {
	box := &nativeGroupBox{
		widget:   widgetWithRole(gtkFrameGetType(), gtkAccessibleRoleGroup),
		title:    gtkLabelNew(title),
		subtitle: gtkLabelNew(subtitle),
		body:     newComponentLayout(),
	}
	gtkWidgetAddCSSClass(box.title, stringPointer("heading"))
	gtkFrameSetLabelWidget((*gtkFrame)(unsafe.Pointer(box.widget)), box.title)
	label := (*gtkLabel)(unsafe.Pointer(box.subtitle))
	gtkLabelSetWrap(label, 1)
	gtkLabelSetXalign(label, 0)
	gtkWidgetAddCSSClass(box.subtitle, stringPointer("dim-label"))
	gtkWidgetAddCSSClass(box.subtitle, stringPointer("caption"))
	gtkWidgetSetParent(box.subtitle, box.body)
	gtkWidgetSetVisible(box.subtitle, nativeBool(goString(subtitle) != ""))
	gtkFrameSetChild((*gtkFrame)(unsafe.Pointer(box.widget)), box.body)
	registerNativeComponent(uintptr(window), box.widget, nil, kindGroupBox)
	componentBody(box.widget, box.body)
	storeComponentState(box.widget, box)
	onComponentAllocate(box.body, func(width, _ int) {
		if gtkWidgetGetVisible(box.subtitle) != 0 {
			width = max(0, width-8)
			moveComponent(box.subtitle, 4, 2, width, measureWidget(box.subtitle, gtkOrientationVertical, width))
		}
	})
	setAccessibleText(box.widget, gtkAccessiblePropertyLabel, goString(title))
	return unsafe.Pointer(box.widget)
}
func gkGTKGroupBoxSet(pointer unsafe.Pointer, title, subtitle *cChar) {
	if box := componentState[nativeGroupBox](pointer); box != nil {
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(box.title)), title)
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(box.subtitle)), subtitle)
		gtkWidgetSetVisible(box.subtitle, nativeBool(goString(subtitle) != ""))
		setAccessibleText(box.widget, gtkAccessiblePropertyLabel, goString(title))
		gtkWidgetQueueAllocate(box.body)
	}
}
func gkGTKGroupBoxChrome(pointer unsafe.Pointer, width, height *cInt) {
	*width, *height = 2, 2
	if box := componentState[nativeGroupBox](pointer); box != nil {
		var frame cInt
		gtkWidgetMeasure(box.widget, gtkOrientationVertical, -1, &frame, nil, nil, nil)
		*height = cInt(max(int(frame), measureWidget(box.title, gtkOrientationVertical, -1)+2))
	}
}
func gkGTKGroupBoxSubtitleHeight(pointer unsafe.Pointer, width cInt) cInt {
	if box := componentState[nativeGroupBox](pointer); box != nil && gtkWidgetGetVisible(box.subtitle) != 0 {
		return cInt(measureWidget(box.subtitle, gtkOrientationVertical, max(0, int(width)-8)) + 6)
	}
	return 0
}
func gkGTKGroupBoxBody(pointer unsafe.Pointer, width, height, top *cInt) {
	*width, *height, *top = 0, 0, 0
	if box := componentState[nativeGroupBox](pointer); box != nil {
		var chromeWidth, chromeHeight cInt
		gkGTKGroupBoxChrome(pointer, &chromeWidth, &chromeHeight)
		w, h := componentFrameSize(box.widget)
		*width, *height = max(0, cInt(w)-chromeWidth), max(0, cInt(h)-chromeHeight)
		*top = gkGTKGroupBoxSubtitleHeight(pointer, *width)
	}
}

type nativeSplit struct {
	widget              *gtkWidget
	panes, dividers     []*gtkWidget
	positions, minimums []float64
	vertical, updating  bool
}

func (split *nativeSplit) length() int {
	w, h := componentFrameSize(split.widget)
	if split.vertical {
		return w
	}
	return h
}
func (split *nativeSplit) defaults() {
	for i, position := range split.positions {
		if position < 0 {
			split.positions[i] = float64(split.length()-len(split.positions))*float64(i+1)/float64(len(split.panes)) + float64(i)
		}
	}
}
func (split *nativeSplit) apply() {
	split.updating = true
	defer func() { split.updating = false }()
	origin := 0.0
	for i, divider := range split.dividers {
		gtkPanedSetPosition((*gtkPaned)(unsafe.Pointer(divider)), cInt(math.Round(max(0, split.positions[i]-origin))))
		origin = split.positions[i] + 1
	}
}
func gkGTKSplitViewCreate(window cUintptr, vertical, count cInt) unsafe.Pointer {
	n := max(1, int(count))
	split := &nativeSplit{panes: make([]*gtkWidget, n), positions: make([]float64, max(0, n-1)), minimums: make([]float64, n), vertical: vertical != 0}
	for i := range split.panes {
		split.panes[i] = newComponentLayout()
	}
	for i := range split.positions {
		split.positions[i] = -1
	}
	orientation := gtkOrientation(gtkOrientationVertical)
	if split.vertical {
		orientation = gtkOrientationHorizontal
	}
	if n == 1 {
		split.widget = newComponentLayout()
		gtkWidgetSetParent(split.panes[0], split.widget)
		moveComponent(split.panes[0], 0, 0, 1, 1)
	} else {
		split.dividers = make([]*gtkWidget, n-1)
		tail := split.panes[n-1]
		for i := n - 2; i >= 0; i-- {
			divider := gtkPanedNew(orientation)
			paned := (*gtkPaned)(unsafe.Pointer(divider))
			gtkPanedSetStartChild(paned, split.panes[i])
			gtkPanedSetEndChild(paned, tail)
			gtkPanedSetResizeStartChild(paned, 0)
			gtkPanedSetShrinkStartChild(paned, 0)
			gtkPanedSetShrinkEndChild(paned, 0)
			split.dividers[i], tail = divider, divider
			connectNativeNotify(unsafe.Pointer(divider), "notify::position", func() {
				if split.updating || gtkWidgetGetMapped(split.widget) == 0 {
					return
				}
				origin := 0.0
				for i, divider := range split.dividers {
					split.positions[i] = origin + float64(gtkPanedGetPosition((*gtkPaned)(unsafe.Pointer(divider))))
					origin = split.positions[i] + 1
				}
				sendComponentEvent(split.widget, gtkEvent{kind: gkEventResize})
			})
		}
		split.widget = tail
	}
	registerNativeComponent(uintptr(window), split.widget, nil, kindSplit)
	storeComponentState(split.widget, split)
	return unsafe.Pointer(split.widget)
}
func gkGTKSplitViewPane(pointer unsafe.Pointer, index cInt) unsafe.Pointer {
	split := componentState[nativeSplit](pointer)
	if split == nil || index < 0 || int(index) >= len(split.panes) {
		return nil
	}
	pane := split.panes[index]
	registerComponentPane(split.widget, pane)
	return unsafe.Pointer(pane)
}
func gkGTKSplitViewSetPane(pointer unsafe.Pointer, index cInt, minimum cDouble, collapsible cInt) {
	split := componentState[nativeSplit](pointer)
	if split == nil || index < 0 || int(index) >= len(split.panes) {
		return
	}
	split.minimums[index] = max(0, float64(minimum))
	if split.vertical {
		minimumComponent(split.panes[index], int(minimum), 0)
	} else {
		minimumComponent(split.panes[index], 0, int(minimum))
	}
	if int(index) < len(split.dividers) {
		gtkPanedSetShrinkStartChild((*gtkPaned)(unsafe.Pointer(split.dividers[index])), nativeBool(collapsible != 0))
	} else if len(split.dividers) > 0 {
		gtkPanedSetShrinkEndChild((*gtkPaned)(unsafe.Pointer(split.dividers[len(split.dividers)-1])), nativeBool(collapsible != 0))
	}
}
func gkGTKSplitViewPaneFrame(pointer unsafe.Pointer, index cInt, x, y, width, height *cInt) {
	*x, *y, *width, *height = 0, 0, 0, 0
	split := componentState[nativeSplit](pointer)
	if split == nil || index < 0 || int(index) >= len(split.panes) {
		return
	}
	split.defaults()
	start, end := 0.0, float64(split.length())
	if index > 0 {
		start = split.positions[index-1] + 1
	}
	if int(index) < len(split.positions) {
		end = split.positions[index]
	}
	w, h := componentFrameSize(split.widget)
	length := cInt(math.Floor(max(0, end-start)))
	if split.vertical {
		*x, *width, *height = cInt(start), length, cInt(h)
	} else {
		*y, *width, *height = cInt(start), cInt(w), length
	}
}
func gkGTKSplitViewDivider(pointer unsafe.Pointer, index cInt) cDouble {
	split := componentState[nativeSplit](pointer)
	if split == nil || index < 0 || int(index) >= len(split.positions) {
		return 0
	}
	split.defaults()
	return cDouble(split.positions[index])
}
func gkGTKSplitViewSetDivider(pointer unsafe.Pointer, index cInt, position cDouble) {
	split := componentState[nativeSplit](pointer)
	if split == nil || index < 0 || int(index) >= len(split.positions) {
		return
	}
	split.defaults()
	lower, upper := split.minimums[index], float64(split.length())
	if index > 0 {
		lower += split.positions[index-1] + 1
	}
	if int(index)+1 < len(split.positions) {
		upper = split.positions[index+1]
	}
	upper -= 1 + split.minimums[index+1]
	split.positions[index] = max(lower, min(upper, float64(position)))
	split.apply()
}

func gkGTKAddChild(container, child unsafe.Pointer) {
	if target := componentContainer((*gtkWidget)(container)); target != nil {
		putComponent(target, (*gtkWidget)(child))
	}
}
func gkGTKClipViewCreate(window cUintptr) unsafe.Pointer {
	widget := newComponentLayout()
	registerNativeComponent(uintptr(window), widget, nil, kindClip)
	componentBody(widget, widget)
	return unsafe.Pointer(widget)
}

type nativePopover struct {
	widget, body *gtkWidget
	handle       uintptr
	closed       bool
}

func newNativePopover(handle uintptr, transient bool) *nativePopover {
	popover := &nativePopover{widget: gtkPopoverNew(), body: newComponentLayout(), handle: handle}
	gObjectRefSink(unsafe.Pointer(popover.widget))
	gtkPopoverSetAutohide((*gtkPopover)(unsafe.Pointer(popover.widget)), nativeBool(transient))
	registerNativeComponent(0, popover.body, nil, kindClip)
	componentBody(popover.body, popover.body)
	gtkPopoverSetChild((*gtkPopover)(unsafe.Pointer(popover.widget)), popover.body)
	gObjectUnref(unsafe.Pointer(popover.body))
	connectNativeSignal(unsafe.Pointer(popover.widget), "closed", func() {
		if !popover.closed {
			dispatchNativeEvent(popover.handle, gtkEvent{kind: gkEventClose})
		}
	})
	return popover
}
func (popover *nativePopover) show(anchor *gtkWidget, edge, width, height int) bool {
	if !anchorPopover(popover.widget, anchor, edge) {
		return false
	}
	setComponentWindow(popover.body, anchor)
	gtkWidgetSetSizeRequest(popover.body, cInt(max(1, width)), cInt(max(1, height)))
	gtkPopoverPopup((*gtkPopover)(unsafe.Pointer(popover.widget)))
	return true
}
func (popover *nativePopover) close() { popover.closed = true; closeNativePopover(popover.widget) }

var accessoryKey = gQuarkFromString(stringPointer("gamekit-accessories"))

type nativeAccessory struct{ bar, view *gtkWidget }

func accessoryBar(window *gtkWindow) *gtkWidget {
	object := (*gObject)(unsafe.Pointer(window))
	if data := gObjectGetQdata(object, accessoryKey); data != nil {
		return (*gtkWidget)(data)
	}
	bar := gtkCenterBoxNew()
	gtkWidgetAddCSSClass(bar, stringPointer("toolbar"))
	gtkCenterBoxSetStartWidget((*gtkCenterBox)(unsafe.Pointer(bar)), gtkBoxNew(gtkOrientationHorizontal, 6))
	gtkCenterBoxSetCenterWidget((*gtkCenterBox)(unsafe.Pointer(bar)), gtkBoxNew(gtkOrientationHorizontal, 6))
	gtkCenterBoxSetEndWidget((*gtkCenterBox)(unsafe.Pointer(bar)), gtkBoxNew(gtkOrientationHorizontal, 6))
	gtkBoxAppend(windowChrome(window, true), bar)
	gtkWidgetSetVisible(bar, 0)
	gObjectSetQdataFull(object, accessoryKey, unsafe.Pointer(bar), nil)
	return bar
}
func updateAccessoryBar(bar *gtkWidget) {
	box := (*gtkCenterBox)(unsafe.Pointer(bar))
	visible := gtkWidgetGetFirstChild(gtkCenterBoxGetStartWidget(box)) != nil || gtkWidgetGetFirstChild(gtkCenterBoxGetCenterWidget(box)) != nil || gtkWidgetGetFirstChild(gtkCenterBoxGetEndWidget(box)) != nil
	gtkWidgetSetVisible(bar, nativeBool(visible))
}
func addNativeAccessory(window *Window, view *gtkWidget, alignment, width, height int) *nativeAccessory {
	if window.IsBorderless() {
		return nil
	}
	bar := accessoryBar(window.nativeGTKWindow())
	box := (*gtkCenterBox)(unsafe.Pointer(bar))
	part := gtkCenterBoxGetCenterWidget(box)
	if alignment == 1 {
		part = gtkCenterBoxGetStartWidget(box)
	} else if alignment == 3 {
		part = gtkCenterBoxGetEndWidget(box)
	}
	removeNativeComponent(view)
	gtkWidgetSetSizeRequest(view, cInt(max(1, width)), cInt(max(1, height)))
	gtkWidgetSetChildVisible(view, 1)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(part)), view)
	updateAccessoryBar(bar)
	gObjectRef(unsafe.Pointer(view))
	return &nativeAccessory{bar: bar, view: view}
}
func (accessory *nativeAccessory) close() {
	removeNativeComponent(accessory.view)
	updateAccessoryBar(accessory.bar)
	gObjectUnref(unsafe.Pointer(accessory.view))
}
