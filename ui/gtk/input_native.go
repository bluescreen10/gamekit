//go:build linux && cgo

package gtk

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
	"unsafe"
)

func nativeSurfacePoint(widget *gtkWidget, x, y float64) (float64, float64) {
	native := gtkWidgetGetNative(widget)
	if native == nil {
		return 0, 0
	}
	var ox, oy cDouble
	gtkNativeGetSurfaceTransform(native, &ox, &oy)
	source := graphenePoint{x: cFloat(x - float64(ox)), y: cFloat(y - float64(oy))}
	destination := source
	gtkWidgetComputePoint((*gtkWidget)(unsafe.Pointer(native)), widget, &source, &destination)
	return float64(destination.x), float64(destination.y)
}
func nativePointerPosition(widget *gtkWidget) (float64, float64) {
	native := gtkWidgetGetNative(widget)
	if native == nil {
		return 0, 0
	}
	surface := gtkNativeGetSurface(native)
	if surface == nil {
		return 0, 0
	}
	seat := gdkDisplayGetDefaultSeat(gdkSurfaceGetDisplay(surface))
	if seat == nil {
		return 0, 0
	}
	device := gdkSeatGetPointer(seat)
	var x, y cDouble
	if device == nil || gdkSurfaceGetDevicePosition(surface, device, &x, &y, nil) == 0 {
		return 0, 0
	}
	return nativeSurfacePoint(widget, float64(x), float64(y))
}
func (win *nativeWinState) eventPosition(event *gdkEvent) (float64, float64) {
	var x, y cDouble
	gdkEventGetPosition(event, &x, &y)
	return nativeSurfacePoint(win.root, float64(x), float64(y))
}
func (win *nativeWinState) focusEditable() bool {
	for widget := gtkRootGetFocus((*gtkRoot)(unsafe.Pointer(win.widget))); widget != nil; widget = gtkWidgetGetParent(widget) {
		if isNative(unsafe.Pointer(widget), gtkEditableGetType()) || isNative(unsafe.Pointer(widget), gtkTextViewGetType()) {
			return true
		}
	}
	return false
}
func (win *nativeWinState) emitText(text string) {
	for _, r := range text {
		if r >= 0x20 && r != 0x7f {
			gkGTKGoCharEvent(cUintptr(win.handle), cUint(r))
		}
	}
}
func (win *nativeWinState) imeEvent(kind int, text string, start, length int) {
	win.notify(gtkEvent{kind: gkEventIme, a: kind, b: start, c: length, text: text})
}
func (win *nativeWinState) installIME() {
	win.im = gtkIMMulticontextNew()
	gtkIMContextSetClientWidget(win.im, win.root)
	connectNativeArgument(unsafe.Pointer(win.im), "commit", gtkStringSignalCallback(), func(text string) {
		if win.composing {
			win.composing = false
			win.imeEvent(2, text, len(utf16.Encode([]rune(text))), 0)
		}
		win.emitText(text)
	})
	connectNativeSignal(unsafe.Pointer(win.im), "preedit-start", func() {
		if !win.composing {
			win.composing = true
			win.imeEvent(0, "", 0, 0)
		}
	})
	connectNativeSignal(unsafe.Pointer(win.im), "preedit-changed", func() {
		var text *cChar
		var cursor cInt
		gtkIMContextGetPreeditString(win.im, &text, nil, &cursor)
		value := takeString(text)
		if value != "" {
			if !win.composing {
				win.composing = true
				win.imeEvent(0, "", 0, 0)
			}
			win.imeEvent(1, value, nativeUTF16Offset(value, int(cursor)), 0)
		}
	})
	connectNativeSignal(unsafe.Pointer(win.im), "preedit-end", func() {
		if win.composing {
			win.composing = false
			win.imeEvent(3, "", 0, 0)
		}
	})
}
func (win *nativeWinState) installFileDrop() {
	drop := gtkDropTargetNew(gdkFileListGetType(), gdkActionCopy)
	for _, signal := range []string{"enter", "motion"} {
		connectNativeArgument(unsafe.Pointer(drop), signal, gtkDropEnterCallback(), func(x, y float64) gdkDragAction {
			if win.acceptFiles {
				return gdkActionCopy
			}
			return 0
		})
	}
	connectNativeArgument(unsafe.Pointer(drop), "drop", gtkValueDropCallback(), func(value *gValue, x, y float64) bool {
		if !win.acceptFiles {
			return false
		}
		files := gdkFileListGetFiles((*gdkFileList)(gValueGetBoxed(value)))
		defer gSListFree(files)
		var paths []string
		for item := files; item != nil; item = item.next {
			if path := gFileGetPath((*gFile)(item.data)); path != nil {
				paths = append(paths, takeString(path))
			}
		}
		if len(paths) > 0 {
			win.notify(gtkEvent{kind: gkEventWindowFileDrop, a: len(paths), text: strings.Join(paths, "\x1e")})
		}
		return len(paths) > 0
	})
	gtkWidgetAddController(win.root, (*gtkEventController)(unsafe.Pointer(drop)))
}
func (win *nativeWinState) warpCenter() {
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return
	}
	source := graphenePoint{x: cFloat(gtkWidgetGetWidth(win.root) / 2), y: cFloat(gtkWidgetGetHeight(win.root) / 2)}
	destination := source
	gtkWidgetComputePoint(win.root, win.widget, &source, &destination)
	scale := cFloat(gtkWidgetGetScaleFactor(win.widget))
	xWarpPointer(display, 0, window, 0, 0, 0, 0, cInt(destination.x*scale), cInt(destination.y*scale))
	xFlush(display)
	win.warping = true
}
func nativePenEvent(event *gdkEvent) bool {
	device := gdkEventGetDevice(event)
	return device != nil && gdkDeviceGetSource(device) == gdkSourcePen
}
func (win *nativeWinState) sendPen(event *gdkEvent, phase int) {
	x, y := win.eventPosition(event)
	var pressure, tiltX, tiltY, rotation cDouble
	gdkEventGetAxis(event, gdkAxisPressure, &pressure)
	gdkEventGetAxis(event, gdkAxisXtilt, &tiltX)
	gdkEventGetAxis(event, gdkAxisYtilt, &tiltY)
	gdkEventGetAxis(event, gdkAxisRotation, &rotation)
	flags := 0
	if tool := gdkEventGetDeviceTool(event); tool != nil && gdkDeviceToolGetToolType(tool) == gdkDeviceToolTypeEraser {
		flags |= 1
	}
	if gdkEventGetModifierState(event)&gdkButton2Mask != 0 {
		flags |= 2
	}
	win.notify(gtkEvent{
		kind: gkEventPen,
		a:    phase,
		b:    int(uintptr(unsafe.Pointer(gdkEventGetDevice(event))) & 0x7fffffff),
		c:    flags,
		x:    x,
		y:    y,
		z:    float64(pressure),
		w:    float64(rotation) * 2 * math.Pi,
		text: fmt.Sprintf("%f %f", float64(tiltX)*math.Pi/2, float64(tiltY)*math.Pi/2),
	})
}
func nativeScrollDeltas(event *gdkEvent) (float64, float64) {
	switch gdkScrollEventGetDirection(event) {
	case gdkScrollUp:
		return 0, -1
	case gdkScrollDown:
		return 0, 1
	case gdkScrollLeft:
		return -1, 0
	case gdkScrollRight:
		return 1, 0
	}
	var x, y cDouble
	gdkScrollEventGetDeltas(event, &x, &y)
	return float64(x), float64(y)
}
func (win *nativeWinState) inputEvent(event *gdkEvent) bool {
	if win.destroyed {
		return false
	}
	kind := gdkEventGetEventType(event)
	state := gdkEventGetModifierState(event)
	mods := nativeModifiers(state)
	switch kind {
	case gdkKeyPress, gdkKeyRelease:
		code := gdkKeyEventGetKeycode(event)
		key := physicalKeyCode(uint32(code))
		pressed := kind == gdkKeyPress
		if key >= 0 {
			gkGTKGoKeyEvent(cUintptr(win.handle), cInt(key), cInt(code), cInt(nativeBool(pressed)), cInt(mods))
		}
		editable := win.focusEditable()
		if !editable && win.im != nil && gtkIMContextFilterKeypress(win.im, event) != 0 {
			return false
		}
		if pressed && state&(gdkControlMask|gdkAltMask|gdkSuperMask) == 0 {
			win.emitText(string(rune(gdkKeyvalToUnicode(gdkKeyEventGetKeyval(event)))))
		}
		return false
	case gdkButtonPress, gdkButtonRelease:
		button := nativePointerButton(uint32(gdkButtonEventGetButton(event)))
		pressed := kind == gdkButtonPress
		if button >= 0 && button < 8 {
			mask := uint32(1) << button
			if pressed {
				win.buttons |= mask
			} else {
				win.buttons &^= mask
			}
			gkGTKGoPointerButtonEvent(cUintptr(win.handle), cInt(button), cInt(nativeBool(pressed)), cInt(mods))
		}
		if nativePenEvent(event) && button == 0 {
			phase := 2
			if pressed {
				phase = 0
			}
			win.sendPen(event, phase)
		}
	case gdkMotionNotify:
		x, y := win.eventPosition(event)
		dx, dy := 0.0, 0.0
		if win.haveLast {
			dx, dy = x-win.lastX, y-win.lastY
		}
		if win.cursorMode == 2 {
			cx, cy := float64(gtkWidgetGetWidth(win.root)/2), float64(gtkWidgetGetHeight(win.root)/2)
			if win.warping && math.Abs(x-cx) < 1 && math.Abs(y-cy) < 1 {
				win.warping = false
				win.lastX, win.lastY, win.haveLast = x, y, true
				break
			}
			dx, dy = x-cx, y-cy
			win.virtualX += dx
			win.virtualY += dy
			win.warpCenter()
			x, y = win.virtualX, win.virtualY
		} else {
			win.lastX, win.lastY, win.haveLast = x, y, true
		}
		win.notify(gtkEvent{kind: gkEventPointerMove, a: mods, x: x, y: y, z: dx, w: dy})
		if nativePenEvent(event) {
			win.sendPen(event, 1)
		}
	case gdkEnterNotify, gdkLeaveNotify:
		if gdkCrossingEventGetDetail(event) == gdkNotifyInferior {
			break
		}
		x, y := win.eventPosition(event)
		report := gkEventPointerEnter
		if kind == gdkLeaveNotify {
			report = gkEventPointerLeave
			win.haveLast = false
		}
		win.notify(gtkEvent{kind: report, x: x, y: y})
	case gdkScroll:
		dx, dy := nativeScrollDeltas(event)
		win.scrollX -= dx
		win.scrollY -= dy
		if dx != 0 || dy != 0 {
			gkGTKGoScrollEvent(cUintptr(win.handle), cDouble(-dx), cDouble(-dy))
		}
	case gdkTouchBegin, gdkTouchUpdate, gdkTouchEnd, gdkTouchCancel:
		phase := 0
		switch kind {
		case gdkTouchUpdate:
			phase = 1
		case gdkTouchEnd:
			phase = 2
		case gdkTouchCancel:
			phase = 3
		}
		x, y := win.eventPosition(event)
		win.notify(gtkEvent{kind: gkEventTouch, a: int(uintptr(unsafe.Pointer(gdkEventGetEventSequence(event))) & 0x7fffffff), b: phase, x: x, y: y, z: 1})
	case gdkTouchpadPinch, gdkTouchpadSwipe:
		phase := 1
		switch gdkTouchpadEventGetGesturePhase(event) {
		case gdkTouchpadGesturePhaseBegin:
			phase = 0
		case gdkTouchpadGesturePhaseEnd:
			phase = 2
		case gdkTouchpadGesturePhaseCancel:
			phase = 3
		}
		x, y := win.eventPosition(event)
		report := gtkEvent{kind: gkEventGesture, b: phase, x: x, y: y}
		if kind == gdkTouchpadSwipe {
			var dx, dy cDouble
			gdkTouchpadEventGetDeltas(event, &dx, &dy)
			report.a, report.z, report.w = 2, float64(dx), float64(dy)
			win.notify(report)
			break
		}
		scale := float64(gdkTouchpadEventGetPinchScale(event))
		if phase == 0 {
			win.pinchScale = 1
		}
		report.z = scale - win.pinchScale
		win.pinchScale = scale
		win.notify(report)
		if angle := gdkTouchpadEventGetPinchAngleDelta(event); angle != 0 {
			report.a, report.z = 1, float64(angle)
			win.notify(report)
		}
	}
	if nativeCapture != nil && nativeWinFromWidget(nativeCapture) == win {
		return deliverNativeCapture(nativeCapture, event)
	}
	return false
}
func gkGTKWindowCursorPosition(pointer unsafe.Pointer, x, y *cDouble) {
	px, py := 0.0, 0.0
	if win := nativeWin(pointer); win != nil {
		if win.cursorMode == 2 {
			px, py = win.virtualX, win.virtualY
		} else {
			px, py = nativePointerPosition(win.root)
		}
	}
	if x != nil {
		*x = cDouble(px)
	}
	if y != nil {
		*y = cDouble(py)
	}
}
func gkGTKWindowGetScroll(pointer unsafe.Pointer, x, y *cDouble) {
	*x, *y = 0, 0
	if win := nativeWin(pointer); win != nil {
		*x, *y = cDouble(win.scrollX), cDouble(win.scrollY)
	}
}
func gkGTKWindowSetCursorMode(pointer unsafe.Pointer, mode cInt) {
	win := nativeWin(pointer)
	if win == nil || win.cursorMode == int(mode) {
		return
	}
	display, window := win.xdisplay(), win.xwindow()
	if mode == 2 {
		win.virtualX, win.virtualY = nativePointerPosition(win.root)
		gtkWindowPresent(win.window())
	}
	if win.cursorMode == 2 && display != nil {
		xUngrabPointer(display, 0)
		xFlush(display)
	}
	win.cursorMode = int(mode)
	var cursorName *cChar
	if mode != 0 {
		cursorName = stringPointer("none")
	}
	gtkWidgetSetCursorFromName(win.widget, cursorName)
	if mode == 2 && display != nil && window != 0 && gtkWidgetGetMapped(win.widget) != 0 {
		xGrabPointer(display, window, 1, xButtonPressMask|xButtonReleaseMask|xPointerMotionMask, xGrabModeAsync, xGrabModeAsync, window, 0, 0)
		win.warpCenter()
	}
}
func gkGTKWindowGetCursorMode(pointer unsafe.Pointer) cInt {
	if win := nativeWin(pointer); win != nil {
		return cInt(win.cursorMode)
	}
	return 0
}
