//go:build linux && cgo

package gtk

import "unsafe"

type nativeCanvas struct {
	view            *nativeView
	xdisplay        *display
	xwindow         xWindow
	mapped          bool
	tick            cUint
	frames, buttons int
	lastX, lastY    float64
	haveLast        bool
}

var nativeCapture *gtkWidget

func (canvas *nativeCanvas) send(event gtkEvent) { dispatchNativeEvent(canvas.view.action, event) }
func (canvas *nativeCanvas) syncSurface() {
	if canvas.xwindow == 0 {
		return
	}
	widget := canvas.view.outer
	native := gtkWidgetGetNative(widget)
	if native == nil {
		return
	}
	surface := gtkNativeGetSurface(native)
	if !isNative(unsafe.Pointer(surface), gdkX11SurfaceGetType()) {
		return
	}
	x, y, width, height := 0, 0, 1, 1
	scale := float64(gtkWidgetGetScaleFactor(widget))
	var bounds grapheneRect
	if gtkWidgetComputeBounds(widget, (*gtkWidget)(unsafe.Pointer(native)), &bounds) != 0 {
		var ox, oy cDouble
		gtkNativeGetSurfaceTransform(native, &ox, &oy)
		x, y = int((float64(bounds.origin.x)+float64(ox))*scale), int((float64(bounds.origin.y)+float64(oy))*scale)
		width, height = max(1, int(float64(bounds.size.width)*scale)), max(1, int(float64(bounds.size.height)*scale))
	}
	xMoveResizeWindow(canvas.xdisplay, canvas.xwindow, cInt(x), cInt(y), cUint(width), cUint(height))
	mapped := gtkWidgetGetMapped(widget) != 0 && gtkWidgetGetWidth(widget) > 0
	if mapped && !canvas.mapped {
		xMapWindow(canvas.xdisplay, canvas.xwindow)
	} else if !mapped && canvas.mapped {
		xUnmapWindow(canvas.xdisplay, canvas.xwindow)
	}
	canvas.mapped = mapped
	xFlush(canvas.xdisplay)
}
func (canvas *nativeCanvas) surfaceWindow() xWindow {
	if canvas.xwindow == 0 {
		win := nativeWinFromWidget(canvas.view.outer)
		if win == nil {
			win = nativeWinForHandle(canvas.view.window)
		}
		if win == nil {
			return 0
		}
		display, parent := win.xdisplay(), win.xwindow()
		if display == nil || parent == 0 {
			return 0
		}
		canvas.xdisplay = display
		canvas.xwindow = xCreateSimpleWindow(display, parent, 0, 0, 1, 1, 0, 0, 0)
		xSelectInput(display, canvas.xwindow, 0)
		canvas.syncSurface()
	}
	return canvas.xwindow
}
func (canvas *nativeCanvas) close() {
	if canvas.tick != 0 {
		gtkWidgetRemoveTickCallback(canvas.view.outer, canvas.tick)
		canvas.tick = 0
	}
	if canvas.xwindow != 0 && canvas.xdisplay != nil {
		xDestroyWindow(canvas.xdisplay, canvas.xwindow)
		xFlush(canvas.xdisplay)
		canvas.xwindow = 0
	}
}
func (canvas *nativeCanvas) reportSurface(kind int) {
	scale := int(gtkWidgetGetScaleFactor(canvas.view.outer))
	canvas.send(gtkEvent{kind: gkEventSurface, a: kind, b: 1, x: float64(canvas.view.width * scale), y: float64(canvas.view.height * scale), z: float64(scale)})
}
func (canvas *nativeCanvas) button(button uint32, action int, state gdkModifierType) {
	index := nativePointerButton(button)
	if canvas.view.action == 0 || index < 0 || index >= 8 {
		return
	}
	mask := 1 << index
	if action == 1 {
		canvas.buttons |= mask
	} else {
		if canvas.buttons&mask == 0 {
			return
		}
		canvas.buttons &^= mask
	}
	gkGTKUIGoCanvasPointerButton(cUintptr(canvas.view.action), cInt(index), cInt(action), cInt(nativeModifiers(state)))
}
func (canvas *nativeCanvas) cancelButtons() {
	for button := 0; button < 8; button++ {
		if canvas.buttons&(1<<button) != 0 {
			gkGTKUIGoCanvasPointerButton(cUintptr(canvas.view.action), cInt(button), 0, 0)
		}
	}
	canvas.buttons = 0
}
func (canvas *nativeCanvas) motion(x, y float64, state gdkModifierType) {
	dx, dy := 0.0, 0.0
	if canvas.haveLast {
		dx, dy = x-canvas.lastX, y-canvas.lastY
	}
	canvas.lastX, canvas.lastY, canvas.haveLast = x, y, true
	canvas.send(gtkEvent{kind: gkEventPointerMove, a: nativeModifiers(state), x: x, y: y, z: dx, w: dy})
}
func gkGTKUICreateCanvas(window, handle cUintptr) unsafe.Pointer {
	area := createNativeLayout(gtkAccessibleRoleApplication)
	gtkWidgetSetFocusable(area, 1)
	view := createNativeView(uintptr(window), area, nil, kindCanvas, uintptr(handle))
	canvas := &nativeCanvas{view: view}
	view.canvas = canvas
	click := gtkGestureClickNew()
	gtkGestureSingleSetButton((*gtkGestureSingle)(unsafe.Pointer(click)), 0)
	for _, signal := range []struct {
		name   string
		action int
	}{{"pressed", 1}, {"released", 0}} {
		connectNativeArgument(unsafe.Pointer(click), signal.name, gtkPressedCallback(), func(x, y float64) {
			if signal.action == 1 && gtkWidgetGetFocusable(area) != 0 {
				gtkWidgetGrabFocus(area)
			}
			canvas.button(uint32(gtkGestureSingleGetCurrentButton((*gtkGestureSingle)(unsafe.Pointer(click)))), signal.action, gtkEventControllerGetCurrentEventState((*gtkEventController)(unsafe.Pointer(click))))
		})
	}
	connectNativeArgument(unsafe.Pointer(click), "cancel", gtkObjectSignalCallback(), func(_ unsafe.Pointer) { canvas.cancelButtons() })
	gtkWidgetAddController(area, (*gtkEventController)(unsafe.Pointer(click)))
	scroll := gtkEventControllerScrollNew(gtkEventControllerScrollBothAxes)
	connectNativeArgument(unsafe.Pointer(scroll), "scroll", gtkScrollSignalCallback(), func(x, y float64) bool {
		gkGTKUIGoCanvasScroll(cUintptr(view.action), cDouble(-x), cDouble(-y))
		return true
	})
	gtkWidgetAddController(area, scroll)
	motion := gtkEventControllerMotionNew()
	connectNativeArgument(unsafe.Pointer(motion), "motion", gtkXYSignalCallback(), func(x, y float64) { canvas.motion(x, y, gtkEventControllerGetCurrentEventState(motion)) })
	connectNativeArgument(unsafe.Pointer(motion), "enter", gtkXYSignalCallback(), func(x, y float64) { canvas.send(gtkEvent{kind: gkEventPointerEnter, x: x, y: y}) })
	connectNativeSignal(unsafe.Pointer(motion), "leave", func() {
		x, y := nativePointerPosition(area)
		canvas.haveLast = false
		canvas.send(gtkEvent{kind: gkEventPointerLeave, x: x, y: y})
	})
	gtkWidgetAddController(area, motion)
	keys := gtkEventControllerKeyNew()
	connectNativeArgument(unsafe.Pointer(keys), "key-pressed", gtkKeySignalCallback(), func(value, code uint32, state gdkModifierType) bool {
		if key := physicalKeyCode(code); key >= 0 {
			canvas.send(gtkEvent{kind: gkEventKey, a: key, b: int(code), c: 1, x: float64(nativeModifiers(state))})
		}
		r := rune(gdkKeyvalToUnicode(cUint(value)))
		if r >= 0x20 && r != 0x7f && state&(gdkControlMask|gdkAltMask|gdkSuperMask) == 0 {
			canvas.send(gtkEvent{kind: gkEventText, a: int(r)})
		}
		return true
	})
	connectNativeArgument(unsafe.Pointer(keys), "key-released", gtkKeyReleasedCallback(), func(value, code uint32, state gdkModifierType) {
		if key := physicalKeyCode(code); key >= 0 {
			canvas.send(gtkEvent{kind: gkEventKey, a: key, b: int(code), c: 0, x: float64(nativeModifiers(state))})
		}
	})
	gtkWidgetAddController(area, keys)
	canvas.tick = gtkWidgetAddTickCallback(area, gtkTickCallbackAddress(), newNativeCallback(func(clock *gdkFrameClock) bool {
		canvas.syncSurface()
		if canvas.frames&1 != 0 {
			now := gGetMonotonicTime()
			var interval, presentation cInt64
			gdkFrameClockGetRefreshInfo(clock, gdkFrameClockGetFrameTime(clock), &interval, &presentation)
			until := 0.0
			if presentation > now {
				until = float64(presentation-now) / 1e6
			}
			period := 1.0 / 60
			if interval > 0 {
				period = float64(interval) / 1e6
			}
			canvas.send(gtkEvent{kind: gkEventFrame, x: until, y: period})
			gtkWidgetQueueDraw(area)
		}
		return true
	}), gtkDestroyCallback())
	connectNativeNotify(unsafe.Pointer(area), "notify::scale-factor", func() { canvas.reportSurface(1) })
	connectNativeSignal(unsafe.Pointer(area), "map", func() { canvas.syncSurface(); canvas.send(gtkEvent{kind: gkEventWindowOcclusion}) })
	connectNativeSignal(unsafe.Pointer(area), "unmap", func() { canvas.syncSurface(); canvas.send(gtkEvent{kind: gkEventWindowOcclusion, a: 1}) })
	return unsafe.Pointer(area)
}
func gkGTKUIControlHandle(pointer unsafe.Pointer) cUintptr {
	view := viewState(pointer)
	if view == nil {
		return 0
	}
	if view.canvas != nil {
		return cUintptr(view.canvas.surfaceWindow())
	}
	return cUintptr(uintptr(pointer))
}
func gkGTKUICanvasXDisplay(pointer unsafe.Pointer) cUintptr {
	view := viewState(pointer)
	if view == nil || view.canvas == nil {
		return 0
	}
	view.canvas.surfaceWindow()
	return cUintptr(uintptr(unsafe.Pointer(view.canvas.xdisplay)))
}
func gkGTKUICanvasPointerPosition(pointer unsafe.Pointer, x, y *cDouble) {
	a, b := nativePointerPosition((*gtkWidget)(pointer))
	*x, *y = cDouble(a), cDouble(b)
}
func gkGTKCanvasStartFrames(pointer unsafe.Pointer, start cInt) cInt {
	view := viewState(pointer)
	if view == nil || view.canvas == nil {
		return 0
	}
	if start != 0 {
		view.canvas.frames |= 1
		gtkWidgetQueueDraw(view.outer)
	} else {
		view.canvas.frames &^= 1
	}
	return 1
}

func gkGTKCanvasCapture(pointer unsafe.Pointer, capture cInt) cInt {
	widget := (*gtkWidget)(pointer)
	if capture != 0 {
		if nativeWinFromWidget(widget) == nil {
			return 0
		}
		nativeCapture = widget
	} else {
		releaseNativeCapture(widget)
	}
	return 1
}
func gkGTKCanvasHasCapture(pointer unsafe.Pointer) cInt {
	return cInt(nativeBool(pointer != nil && unsafe.Pointer(nativeCapture) == pointer))
}

func releaseNativeCapture(widget *gtkWidget) {
	if nativeCapture == widget {
		nativeCapture = nil
	}
}

func deliverNativeCapture(widget *gtkWidget, event *gdkEvent) bool {
	view := viewState(unsafe.Pointer(widget))
	if view == nil || view.canvas == nil || view.action == 0 {
		return false
	}
	var sx, sy cDouble
	gdkEventGetPosition(event, &sx, &sy)
	x, y := nativeSurfacePoint(widget, float64(sx), float64(sy))
	inside := x >= 0 && y >= 0 && x < float64(gtkWidgetGetWidth(widget)) && y < float64(gtkWidgetGetHeight(widget))
	if inside {
		return false
	}
	state := gdkEventGetModifierState(event)
	switch gdkEventGetEventType(event) {
	case gdkButtonPress:
		view.canvas.button(uint32(gdkButtonEventGetButton(event)), 1, state)
	case gdkButtonRelease:
		view.canvas.button(uint32(gdkButtonEventGetButton(event)), 0, state)
	case gdkMotionNotify:
		view.canvas.send(gtkEvent{kind: gkEventPointerMove, a: nativeModifiers(state), x: x, y: y})
	case gdkScroll:
		var dx, dy cDouble
		gdkScrollEventGetDeltas(event, &dx, &dy)
		gkGTKUIGoCanvasScroll(cUintptr(view.action), -dx, -dy)
	default:
		return false
	}
	return true
}
