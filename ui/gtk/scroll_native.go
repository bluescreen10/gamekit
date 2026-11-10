//go:build linux && cgo

package gtk

import (
	"math"
	"unsafe"
)

func gkGTKUICreateScrollView(window cUintptr, axes cInt) unsafe.Pointer {
	widget, body := gtkScrolledWindowNew(), createNativeLayout(gtkAccessibleRoleGroup)
	scroll := (*gtkScrolledWindow)(unsafe.Pointer(widget))
	gtkScrolledWindowSetHasFrame(scroll, 1)
	horizontal, vertical := gtkPolicyNever, gtkPolicyNever
	if axes == 1 || axes == 2 {
		horizontal = gtkPolicyAutomatic
	}
	if axes == 0 || axes == 2 {
		vertical = gtkPolicyAutomatic
	}
	gtkScrolledWindowSetPolicy(scroll, horizontal, vertical)
	gtkScrolledWindowSetChild(scroll, body)
	createNativeView(uintptr(window), widget, nil, kindScroll, 0).body = body
	return unsafe.Pointer(widget)
}
func gkGTKUIScrollViewportSize(pointer unsafe.Pointer, width, height *cInt) {
	*width, *height = 0, 0
	if view := viewState(pointer); view != nil {
		*width, *height = cInt(max(0, view.width-2)), cInt(max(0, view.height-2))
	}
}
func configureNativeScroll(adjustment *gtkAdjustment, content, page int) {
	value := float64(gtkAdjustmentGetValue(adjustment))
	gtkAdjustmentConfigure(adjustment, cDouble(max(0, min(value, float64(content-page)))), 0, cDouble(max(content, page)), cDouble(page)*0.1, cDouble(page)*0.9, cDouble(page))
}
func gkGTKUIScrollSetContentSize(pointer unsafe.Pointer, width, height cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	setNativeLayoutMinimum(view.body, int(width), int(height))
	var vw, vh cInt
	gkGTKUIScrollViewportSize(pointer, &vw, &vh)
	scroll := (*gtkScrolledWindow)(unsafe.Pointer(view.outer))
	configureNativeScroll(gtkScrolledWindowGetHadjustment(scroll), int(width), int(vw))
	configureNativeScroll(gtkScrolledWindowGetVadjustment(scroll), int(height), int(vh))
}
func nativeScroll(view *nativeView) *gtkScrolledWindow {
	if view == nil {
		return nil
	}
	if isNative(unsafe.Pointer(view.outer), gtkScrolledWindowGetType()) {
		return (*gtkScrolledWindow)(unsafe.Pointer(view.outer))
	}
	return (*gtkScrolledWindow)(unsafe.Pointer(view.scroll))
}
func gkGTKScrollPosition(pointer unsafe.Pointer, x, y *cInt) {
	*x, *y = 0, 0
	if scroll := nativeScroll(viewState(pointer)); scroll != nil {
		*x, *y = cInt(math.Floor(float64(gtkAdjustmentGetValue(gtkScrolledWindowGetHadjustment(scroll))))), cInt(math.Floor(float64(gtkAdjustmentGetValue(gtkScrolledWindowGetVadjustment(scroll)))))
	}
}
func gkGTKUIScrollOffset(pointer unsafe.Pointer, x, y *cInt) { gkGTKScrollPosition(pointer, x, y) }
func adjustNativeScroll(adjustment *gtkAdjustment, value float64) {
	page := gtkAdjustmentGetPageSize(adjustment)
	if cDouble(value) > gtkAdjustmentGetUpper(adjustment)-page {
		gtkAdjustmentConfigure(adjustment, cDouble(value), gtkAdjustmentGetLower(adjustment), cDouble(value)+page, gtkAdjustmentGetStepIncrement(adjustment), gtkAdjustmentGetPageIncrement(adjustment), page)
	}
	gtkAdjustmentSetValue(adjustment, cDouble(value))
}
func scrollNativeTo(scroll *gtkScrolledWindow, x, y float64, animated bool) {
	horizontal, vertical := gtkScrolledWindowGetHadjustment(scroll), gtkScrolledWindowGetVadjustment(scroll)
	x, y = max(0, x), max(0, y)
	widget := (*gtkWidget)(unsafe.Pointer(scroll))
	if !animated || gtkWidgetGetMapped(widget) == 0 {
		adjustNativeScroll(horizontal, x)
		adjustNativeScroll(vertical, y)
		return
	}
	x = min(x, max(0, float64(gtkAdjustmentGetUpper(horizontal)-gtkAdjustmentGetPageSize(horizontal))))
	y = min(y, max(0, float64(gtkAdjustmentGetUpper(vertical)-gtkAdjustmentGetPageSize(vertical))))
	fromX, fromY := float64(gtkAdjustmentGetValue(horizontal)), float64(gtkAdjustmentGetValue(vertical))
	start := int64(gGetMonotonicTime())
	if clock := gtkWidgetGetFrameClock(widget); clock != nil {
		start = int64(gdkFrameClockGetFrameTime(clock))
	}
	gtkWidgetAddTickCallback(widget, gtkTickCallbackAddress(), newNativeCallback(func(clock *gdkFrameClock) bool {
		t := float64(int64(gdkFrameClockGetFrameTime(clock))-start) / 200000
		if t >= 1 {
			gtkAdjustmentSetValue(horizontal, cDouble(x))
			gtkAdjustmentSetValue(vertical, cDouble(y))
			return false
		}
		eased := 1 - math.Pow(1-max(0, t), 3)
		gtkAdjustmentSetValue(horizontal, cDouble(fromX+(x-fromX)*eased))
		gtkAdjustmentSetValue(vertical, cDouble(fromY+(y-fromY)*eased))
		return true
	}), gtkDestroyCallback())
}
func gkGTKSetScrollPosition(pointer unsafe.Pointer, x, y, animated cInt) {
	if scroll := nativeScroll(viewState(pointer)); scroll != nil {
		scrollNativeTo(scroll, float64(x), float64(y), animated != 0)
	}
}
func gkGTKScrollConfigure(pointer unsafe.Pointer, axes, policy, top, right, bottom, left, overscroll cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	scroll := (*gtkScrolledWindow)(unsafe.Pointer(view.outer))
	shown := gtkPolicyAutomatic
	if policy == 1 {
		shown = gtkPolicyAlways
	} else if policy == 2 {
		shown = gtkPolicyExternal
	}
	horizontal, vertical := gtkPolicyNever, gtkPolicyNever
	if axes == 1 || axes == 2 {
		horizontal = shown
	}
	if axes == 0 || axes == 2 {
		vertical = shown
	}
	gtkScrolledWindowSetPolicy(scroll, horizontal, vertical)
	gtkScrolledWindowSetOverlayScrolling(scroll, nativeBool(policy == 0))
	gtkScrolledWindowSetKineticScrolling(scroll, nativeBool(overscroll == 0))
	gtkWidgetSetMarginTop(view.body, top)
	gtkWidgetSetMarginEnd(view.body, right)
	gtkWidgetSetMarginBottom(view.body, bottom)
	gtkWidgetSetMarginStart(view.body, left)
}
func gkGTKScrollSetZoom(pointer unsafe.Pointer, zoom cDouble) {
	if view := viewState(pointer); view != nil {
		setNativeLayoutZoom(view.body, float64(zoom))
	}
}
func gkGTKScrollZoom(pointer unsafe.Pointer) cDouble {
	if view := viewState(pointer); view != nil {
		return cDouble(nativeLayoutZoom(view.body))
	}
	return 1
}
func gkGTKScrollViewport(pointer unsafe.Pointer, width, height *cInt) {
	view := viewState(pointer)
	if view != nil {
		viewport := gtkScrolledWindowGetChild((*gtkScrolledWindow)(unsafe.Pointer(view.outer)))
		if viewport != nil && gtkWidgetGetWidth(viewport) > 1 && int(gtkWidgetGetWidth(view.outer)) == view.width {
			*width, *height = gtkWidgetGetWidth(viewport), gtkWidgetGetHeight(viewport)
			return
		}
	}
	gkGTKUIScrollViewportSize(pointer, width, height)
}
func gkGTKScrollToView(pointer, targetPointer unsafe.Pointer, animated cInt) cInt {
	scroll, target := viewState(pointer), viewState(targetPointer)
	if scroll == nil || target == nil || scroll.kind != kindScroll || gtkWidgetIsAncestor(target.outer, scroll.body) == 0 {
		return 0
	}
	x, y := 0, 0
	for widget := target.outer; widget != nil && widget != scroll.body; widget = gtkWidgetGetParent(widget) {
		if view := viewState(unsafe.Pointer(widget)); view != nil {
			x += view.x
			y += view.y
		}
	}
	zoom := nativeLayoutZoom(scroll.body)
	tx, ty := float64(x)*zoom, float64(y)*zoom
	window := (*gtkScrolledWindow)(unsafe.Pointer(scroll.outer))
	ox, oy := float64(gtkAdjustmentGetValue(gtkScrolledWindowGetHadjustment(window))), float64(gtkAdjustmentGetValue(gtkScrolledWindowGetVadjustment(window)))
	var vw, vh cInt
	gkGTKScrollViewport(pointer, &vw, &vh)
	if tx+float64(target.width)*zoom > ox+float64(vw) {
		ox = tx + float64(target.width)*zoom - float64(vw)
	}
	ox = min(ox, tx)
	if ty+float64(target.height)*zoom > oy+float64(vh) {
		oy = ty + float64(target.height)*zoom - float64(vh)
	}
	oy = min(oy, ty)
	scrollNativeTo(window, ox, oy, animated != 0)
	return 1
}
