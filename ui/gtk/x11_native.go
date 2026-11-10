//go:build linux && cgo

package gtk

import "unsafe"

func (win *nativeWinState) atom(name string) atom {
	return xInternAtom(win.xdisplay(), stringPointer(name), 0)
}
func (win *nativeWinState) property(name string, kind atom, limit int) []cUlong {
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return nil
	}
	var actual atom
	var format cInt
	var count, remaining cUlong
	var data *cUchar
	status := xGetWindowProperty(display, window, xInternAtom(display, stringPointer(name), 0), 0, cLong(limit), 0, kind, &actual, &format, &count, &remaining, &data)
	if data == nil {
		return nil
	}
	defer xFree(unsafe.Pointer(data))
	if status != 0 || format != 32 {
		return nil
	}
	return append([]cUlong(nil), unsafe.Slice((*cUlong)(unsafe.Pointer(data)), int(count))...)
}
func (win *nativeWinState) frameExtents() (left, right, top, bottom int) {
	if win.borderless {
		return
	}
	if values := win.property("_NET_FRAME_EXTENTS", xACardinal, 4); len(values) == 4 {
		return int(values[0]), int(values[1]), int(values[2]), int(values[3])
	}
	return
}
func (win *nativeWinState) clientOrigin() (int, int) {
	var x, y cInt
	display, window := win.xdisplay(), win.xwindow()
	if display != nil && window != 0 {
		var child xWindow
		xTranslateCoordinates(display, window, xDefaultRootWindow(display), 0, 0, &x, &y, &child)
	}
	return int(x), int(y)
}
func (win *nativeWinState) contentOrigin() (int, int) {
	x, y := win.clientOrigin()
	if gtkWidgetGetMapped(win.root) != 0 {
		source, destination := graphenePoint{}, graphenePoint{}
		if gtkWidgetComputePoint(win.root, win.widget, &source, &destination) != 0 {
			x += int(destination.x)
			y += int(destination.y)
		}
	}
	return x, y
}
func (win *nativeWinState) position() (int, int) {
	if gtkWidgetGetMapped(win.widget) == 0 {
		return win.x, win.y
	}
	x, y := win.clientOrigin()
	left, _, top, _ := win.frameExtents()
	x -= left
	y -= top
	if win.owner != nil {
		ox, oy := win.owner.contentOrigin()
		x -= ox
		y -= oy
	}
	return x, y
}
func (win *nativeWinState) message(name string, values [5]cLong) {
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return
	}
	var event xEvent
	message := (*xClientMessageEvent)(unsafe.Pointer(&event))
	message._type = xClientMessage
	message.window = window
	message.message_type = xInternAtom(display, stringPointer(name), 0)
	message.format = 32
	*(*[5]cLong)(unsafe.Pointer(&message.data)) = values
	xSendEvent(display, xDefaultRootWindow(display), 0, xSubstructureRedirectMask|xSubstructureNotifyMask, &event)
	xFlush(display)
}
func (win *nativeWinState) sendState(add bool, first, second string) {
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return
	}
	a, b := win.atom(first), atom(0)
	if second != "" {
		b = win.atom(second)
	}
	if gtkWidgetGetMapped(win.widget) == 0 {
		var values []atom
		for _, value := range win.property("_NET_WM_STATE", xAAtom, 8) {
			if atom(value) != a && atom(value) != b {
				values = append(values, atom(value))
			}
		}
		if add {
			values = append(values, a)
			if b != 0 {
				values = append(values, b)
			}
		}
		xChangeProperty(display, window, win.atom("_NET_WM_STATE"), xAAtom, 32, xPropModeReplace, (*cUchar)(unsafe.Pointer(unsafe.SliceData(values))), cInt(len(values)))
		xFlush(display)
		return
	}
	win.message("_NET_WM_STATE", [5]cLong{cLong(nativeBool(add)), cLong(a), cLong(b), 1, 0})
}
func (win *nativeWinState) applyHints() {
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return
	}
	hints := xAllocSizeHints()
	if hints == nil {
		return
	}
	defer xFree(unsafe.Pointer(hints))
	var supplied cLong
	xGetWMNormalHints(display, window, hints, &supplied)
	hints.flags &^= xPMaxSize
	switch {
	case win.fullscreen || win.fullscreenTarget:
	case !win.resizable:
		w, h := win.width, win.height
		if win.reportedWidth > 0 {
			w = win.reportedWidth
		}
		if win.reportedHeight > 0 {
			h = win.reportedHeight
		}
		hints.flags |= xPMinSize | xPMaxSize
		hints.min_width, hints.max_width = cInt(w), cInt(w)
		hints.min_height, hints.max_height = cInt(h), cInt(h)
	case win.maxWidth > 0 || win.maxHeight > 0:
		w, h := win.maxWidth, win.maxHeight
		if w <= 0 {
			w = 32767
		}
		if h <= 0 {
			h = 32767
		}
		hints.flags |= xPMaxSize
		hints.max_width, hints.max_height = cInt(w), cInt(h)
	}
	if win.havePosition {
		hints.flags |= xUSPosition
		hints.x, hints.y = cInt(win.x), cInt(win.y)
	}
	xSetWMNormalHints(display, window, hints)
	xFlush(display)
}
func (win *nativeWinState) moveFrame(x, y int) {
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return
	}
	if win.owner != nil {
		ox, oy := win.owner.contentOrigin()
		x += ox
		y += oy
	}
	xMoveWindow(display, window, cInt(x), cInt(y))
	xFlush(display)
}
func (win *nativeWinState) setUtilityType() {
	if !win.utility {
		return
	}
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return
	}
	utility := win.atom("_NET_WM_WINDOW_TYPE_UTILITY")
	xChangeProperty(display, window, win.atom("_NET_WM_WINDOW_TYPE"), xAAtom, 32, xPropModeReplace, (*cUchar)(unsafe.Pointer(&utility)), 1)
	win.sendState(true, "_NET_WM_STATE_SKIP_TASKBAR", "_NET_WM_STATE_SKIP_PAGER")
}

const (
	nativeStateMinimized  = 1
	nativeStateMaximized  = 2
	nativeStateFullscreen = 4
)

func (win *nativeWinState) state() int {
	if gtkWidgetGetMapped(win.widget) == 0 {
		return 0
	}
	state := 0
	if values := win.property("WM_STATE", win.atom("WM_STATE"), 2); len(values) > 0 && values[0] == xIconicState {
		state |= nativeStateMinimized
	}
	hidden, vertical, horizontal, fullscreen := win.atom("_NET_WM_STATE_HIDDEN"), win.atom("_NET_WM_STATE_MAXIMIZED_VERT"), win.atom("_NET_WM_STATE_MAXIMIZED_HORZ"), win.atom("_NET_WM_STATE_FULLSCREEN")
	maximized := 0
	for _, value := range win.property("_NET_WM_STATE", xAAtom, 32) {
		switch atom(value) {
		case hidden:
			state |= nativeStateMinimized
		case vertical, horizontal:
			maximized++
		case fullscreen:
			state |= nativeStateFullscreen
		}
	}
	if maximized == 2 {
		state |= nativeStateMaximized
	}
	return state
}
