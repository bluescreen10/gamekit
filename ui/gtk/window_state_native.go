//go:build linux && cgo

package gtk

import (
	"time"
	"unsafe"
)

type nativeWinState struct {
	widget, overlay, box, top, root, bottom *gtkWidget
	owner                                   *nativeWinState
	handle                                  uintptr
	shouldClose, destroyed                  bool

	utility, borderless, resizable bool
	alwaysOnTop, transparent       bool
	theme, resolvedTheme           int

	x, y, width, height                      int
	reportedWidth, reportedHeight            int
	minWidth, minHeight, maxWidth, maxHeight int
	havePosition                             bool

	focus                            *gtkWidget
	im                               *gtkIMContext
	acceptFiles, composing, occluded bool
	cursorMode                       int
	virtualX, virtualY, lastX, lastY float64
	scrollX, scrollY, pinchScale     float64
	haveLast, warping                bool
	buttons                          uint32

	fullscreen, fullscreenTarget, fullscreenState bool
	windowedWidth, windowedHeight                 int
	restoreWidth, restoreHeight, restoreTries     int
	fullscreenTimer, restoreTimer                 cUint
}

var nativeWindows []*nativeWinState

func nativeWin(pointer unsafe.Pointer) *nativeWinState {
	for _, win := range nativeWindows {
		if unsafe.Pointer(win.widget) == pointer {
			return win
		}
	}
	return nil
}
func nativeWinForHandle(handle uintptr) *nativeWinState {
	for _, win := range nativeWindows {
		if uintptr(unsafe.Pointer(win.widget)) == handle {
			return win
		}
	}
	return nil
}
func nativeWinFromWidget(widget *gtkWidget) *nativeWinState {
	if widget == nil {
		return nil
	}
	return nativeWin(unsafe.Pointer(gtkWidgetGetRoot(widget)))
}
func (win *nativeWinState) window() *gtkWindow { return (*gtkWindow)(unsafe.Pointer(win.widget)) }
func (win *nativeWinState) notify(event gtkEvent) {
	if win != nil && !win.destroyed {
		dispatchNativeEvent(win.handle, event)
	}
}
func (win *nativeWinState) surface() *gdkSurface {
	if win == nil || win.destroyed {
		return nil
	}
	gtkWidgetRealize(win.widget)
	native := gtkWidgetGetNative(win.widget)
	if native == nil {
		return nil
	}
	return gtkNativeGetSurface(native)
}
func (win *nativeWinState) xdisplay() *display {
	surface := win.surface()
	if !isNative(unsafe.Pointer(surface), gdkX11SurfaceGetType()) {
		return nil
	}
	return gdkX11DisplayGetXdisplay(gdkSurfaceGetDisplay(surface))
}
func (win *nativeWinState) xwindow() xWindow {
	surface := win.surface()
	if !isNative(unsafe.Pointer(surface), gdkX11SurfaceGetType()) {
		return 0
	}
	return gdkX11SurfaceGetXid(surface)
}
func pumpNativeUntil(done func() bool, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for !done() && time.Now().Before(deadline) {
		if gMainContextIteration(nil, 0) == 0 {
			time.Sleep(2 * time.Millisecond)
		}
	}
}
func (win *nativeWinState) chromeHeight() int {
	return measureWidget(win.top, gtkOrientationVertical, -1) + measureWidget(win.bottom, gtkOrientationVertical, -1)
}
func (win *nativeWinState) size() (int, int) {
	width, height := win.width, win.height
	if gtkWidgetGetMapped(win.widget) != 0 {
		w, h := int(gtkWidgetGetWidth(win.root)), int(gtkWidgetGetHeight(win.root))
		if w > 0 && h > 0 {
			width, height = w, h
		}
	}
	return width, height
}
func (win *nativeWinState) rootAllocated(width, height int) {
	if win.fullscreenState != win.fullscreen && win.fullscreenSettled(width, height) {
		win.restoreWidth, win.restoreHeight = 0, 0
		win.fullscreenChanged(win.fullscreenState)
	}
	if width == win.reportedWidth && height == win.reportedHeight {
		return
	}
	win.reportedWidth, win.reportedHeight, win.width, win.height = width, height, width, height
	win.notify(gtkEvent{kind: gkEventWindowResize, x: float64(width), y: float64(height)})
}
func (win *nativeWinState) trackFocus() {
	if win == nil || win.destroyed {
		return
	}
	focus := gtkRootGetFocus((*gtkRoot)(unsafe.Pointer(win.widget)))
	view := viewFromDescendant(focus)
	var current *gtkWidget
	if view != nil {
		current = view.outer
	}
	if current == win.focus {
		return
	}
	previous := win.focus
	win.focus = current
	if old := viewState(unsafe.Pointer(previous)); old != nil {
		old.send(gtkEvent{kind: gkEventBlur})
	}
	if view != nil {
		view.send(gtkEvent{kind: gkEventFocus})
	}
}
func gkGTKTrackFocus(pointer unsafe.Pointer) {
	if win := nativeWin(pointer); win != nil {
		win.trackFocus()
	}
}
func gkGTKForgetFocus(pointer unsafe.Pointer, handle cUintptr) {
	if win := nativeWin(pointer); win != nil {
		view := viewState(unsafe.Pointer(win.focus))
		if view == nil || view.handle == 0 || view.handle == uintptr(handle) {
			win.focus = nil
		}
	}
}
func (win *nativeWinState) mapped() bool {
	if gtkWidgetGetMapped(win.widget) == 0 {
		return false
	}
	display, window := win.xdisplay(), win.xwindow()
	if display == nil || window == 0 {
		return true
	}
	var attributes xWindowAttributes
	return xGetWindowAttributes(display, window, &attributes) != 0 && attributes.map_state == xIsViewable
}
func (win *nativeWinState) applyMapped() {
	win.setUtilityType()
	win.applyHints()
	if win.havePosition {
		win.moveFrame(win.x, win.y)
	}
	if win.alwaysOnTop {
		win.sendState(true, "_NET_WM_STATE_ABOVE", "")
	}
}
func (win *nativeWinState) surfaceStateChanged() {
	if win.destroyed {
		return
	}
	win.pollState()
	occluded := gdkToplevelGetState((*gdkToplevel)(unsafe.Pointer(win.surface())))&gdkToplevelStateMinimized != 0
	if occluded != win.occluded {
		win.occluded = occluded
		win.notify(gtkEvent{kind: gkEventWindowOcclusion, a: int(nativeBool(occluded))})
	}
}
func (win *nativeWinState) activeChanged() {
	active := gtkWindowIsActive(win.window()) != 0
	if win.im != nil {
		if active {
			gtkIMContextFocusIn(win.im)
		} else {
			gtkIMContextFocusOut(win.im)
		}
	}
	if !active {
		for button := 0; button < 8; button++ {
			if win.buttons&(1<<button) != 0 {
				gkGTKGoPointerButtonEvent(cUintptr(win.handle), cInt(button), 0, 0)
			}
		}
		win.buttons = 0
	}
	kind := gkEventWindowBlur
	if active {
		kind = gkEventWindowFocus
	}
	win.notify(gtkEvent{kind: kind})
	gIdleAddFull(0, gtkSourceCallback(), newNativeCallback(func() bool { updateNativeAppActive(); return false }), gtkDestroyCallback())
}
func gkGTKWindowCreate(title *cChar, width, height cInt, flags cUint32, owner unsafe.Pointer, x, y cInt, errorBuffer *cChar, errorSize cSize) unsafe.Pointer {
	if !initializeNativeGTK() {
		if errorBuffer != nil && errorSize > 0 {
			buffer := unsafe.Slice((*byte)(unsafe.Pointer(errorBuffer)), int(errorSize))
			n := copy(buffer, "GTK 4 could not open an X11 display")
			buffer[min(n, len(buffer)-1)] = 0
		}
		return nil
	}
	win := &nativeWinState{
		owner:        nativeWin(owner),
		width:        max(1, int(width)),
		height:       max(1, int(height)),
		x:            int(x),
		y:            int(y),
		havePosition: owner != nil,
		resizable:    flags&gkWindowResizable != 0,
		borderless:   flags&gkWindowBorderless != 0,
		alwaysOnTop:  flags&gkWindowAlwaysOnTop != 0,
		transparent:  flags&gkWindowTransparent != 0,
		utility:      flags&gkWindowUtility != 0,
		widget:       gtkWindowNew(),
	}
	gObjectRefSink(unsafe.Pointer(win.widget))
	win.overlay, win.box, win.top, win.bottom, win.root = gtkOverlayNew(), gtkBoxNew(gtkOrientationVertical, 0), gtkBoxNew(gtkOrientationVertical, 0), gtkBoxNew(gtkOrientationVertical, 0), createNativeLayout(gtkAccessibleRoleGroup)
	gtkWidgetSetVexpand(win.root, 1)
	gtkWidgetSetHexpand(win.root, 1)
	gtkWidgetSetFocusable(win.root, 1)
	layoutState(win.root).allocate = win.rootAllocated
	for _, child := range []*gtkWidget{win.top, win.root, win.bottom} {
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(win.box)), child)
	}
	gtkOverlaySetChild((*gtkOverlay)(unsafe.Pointer(win.overlay)), win.box)
	gtkWindowSetChild(win.window(), win.overlay)
	gtkWindowSetTitle(win.window(), title)
	gtkWindowSetDefaultSize(win.window(), cInt(win.width), cInt(win.height))
	gtkWindowSetResizable(win.window(), nativeBool(win.resizable))
	gtkWindowSetDecorated(win.window(), nativeBool(!win.borderless))
	if win.transparent {
		gtkWidgetAddCSSClass(win.widget, stringPointer("gk-transparent"))
		setNativeCSS("window.gk-transparent, window.gk-transparent > overlay", "background: transparent;")
	}
	if win.owner != nil {
		gtkWindowSetTransientFor(win.window(), win.owner.window())
	}
	nativeWindows = append(nativeWindows, win)
	connectNativeCloseRequest(win.window(), func() bool {
		if win.handle != 0 && !queryNativeEvent(win.handle, gtkEvent{kind: gkEventWindowCloseRequest}) {
			return true
		}
		win.shouldClose = true
		if win.utility {
			gtkWidgetSetVisible(win.widget, 0)
		}
		return true
	})
	connectNativeNotify(unsafe.Pointer(win.widget), "notify::is-active", win.activeChanged)
	connectNativeNotify(unsafe.Pointer(win.widget), "notify::focus-widget", win.trackFocus)
	connectNativeSignal(unsafe.Pointer(win.widget), "map", win.applyMapped)
	controller := gtkEventControllerLegacyNew()
	gtkEventControllerSetPropagationPhase(controller, gtkPhaseCapture)
	connectNativeArgument(unsafe.Pointer(controller), "event", gtkLegacyEventCallback(), win.inputEvent)
	gtkWidgetAddController(win.widget, controller)
	win.installFileDrop()
	win.installIME()
	gtkWidgetRealize(win.widget)
	if surface := win.surface(); surface != nil {
		connectNativeNotify(unsafe.Pointer(surface), "notify::state", win.surfaceStateChanged)
	}
	win.setUtilityType()
	if win.alwaysOnTop {
		win.sendState(true, "_NET_WM_STATE_ABOVE", "")
	}
	if win.owner != nil {
		win.applyHints()
		win.moveFrame(win.x, win.y)
	}
	if flags&gkWindowMaximized != 0 {
		gtkWindowMaximize(win.window())
	}
	win.resolvedTheme = resolvedNativeTheme()
	if flags&gkWindowHidden == 0 {
		gtkWindowPresent(win.window())
	}
	return unsafe.Pointer(win.widget)
}
func gkGTKWindowDestroy(pointer unsafe.Pointer) {
	win := nativeWin(pointer)
	if win == nil {
		return
	}
	win.handle = 0
	if win.fullscreenTimer != 0 {
		gSourceRemove(win.fullscreenTimer)
	}
	if win.restoreTimer != 0 {
		gSourceRemove(win.restoreTimer)
	}
	if nativeWinFromWidget(nativeCapture) == win {
		nativeCapture = nil
	}
	if win.cursorMode != 0 {
		if display := win.xdisplay(); display != nil {
			xUngrabPointer(display, 0)
		}
	}
	for i, value := range nativeWindows {
		if value == win {
			nativeWindows = append(nativeWindows[:i], nativeWindows[i+1:]...)
			break
		}
	}
	win.destroyed = true
	if win.im != nil {
		gtkIMContextSetClientWidget(win.im, nil)
		gObjectUnref(unsafe.Pointer(win.im))
		win.im = nil
	}
	gtkWindowDestroy(win.window())
	gObjectUnref(pointer)
	applyNativeTheme()
}
func gkGTKWindowPoll(pointer unsafe.Pointer) {
	runNativePending()
	win := nativeWin(pointer)
	if win != nil && gtkWidgetGetMapped(win.widget) != 0 {
		win.pollState()
		if !win.destroyed && win.handle != 0 {
			x, y := win.position()
			if !win.havePosition || x != win.x || y != win.y {
				moved := win.havePosition
				win.x, win.y, win.havePosition = x, y, true
				if moved {
					gkGTKGoWindowMove(cUintptr(win.handle), cInt(x), cInt(y))
				}
			}
		}
	}
	gkGTKAfterPoll()
}
func gkGTKWindowShouldClose(pointer unsafe.Pointer) cInt {
	win := nativeWin(pointer)
	return cInt(nativeBool(win == nil || win.shouldClose))
}
func gkGTKWindowSetShouldClose(pointer unsafe.Pointer, close cInt) {
	if win := nativeWin(pointer); win != nil {
		win.shouldClose = close != 0
	}
}
func gkGTKWindowSize(pointer unsafe.Pointer, width, height *cInt) {
	*width, *height = 0, 0
	if win := nativeWin(pointer); win != nil {
		w, h := win.size()
		*width, *height = cInt(w), cInt(h)
	}
}
func gkGTKWindowFramebufferSize(pointer unsafe.Pointer, width, height *cInt) {
	gkGTKWindowSize(pointer, width, height)
	if win := nativeWin(pointer); win != nil {
		scale := gtkWidgetGetScaleFactor(win.widget)
		*width *= scale
		*height *= scale
	}
}
func (win *nativeWinState) clampSize(width, height int) (int, int) {
	width, height = max(1, width, win.minWidth), max(1, height, win.minHeight)
	if win.maxWidth > 0 {
		width = min(width, win.maxWidth)
	}
	if win.maxHeight > 0 {
		height = min(height, win.maxHeight)
	}
	return width, height
}
func gkGTKWindowSetSize(pointer unsafe.Pointer, width, height cInt) {
	win := nativeWin(pointer)
	if win == nil {
		return
	}
	w, h := win.clampSize(int(width), int(height))
	win.width, win.height = w, h
	gtkWindowSetDefaultSize(win.window(), cInt(w), cInt(h+win.chromeHeight()))
	if gtkWidgetGetMapped(win.widget) == 0 {
		return
	}
	if !win.resizable {
		win.reportedWidth, win.reportedHeight = w, h
		win.applyHints()
	}
	if display, window := win.xdisplay(), win.xwindow(); display != nil && window != 0 {
		scale := int(gtkWidgetGetScaleFactor(win.widget))
		xResizeWindow(display, window, cUint(w*scale), cUint((h+win.chromeHeight())*scale))
		xFlush(display)
	}
	pumpNativeUntil(func() bool { return int(gtkWidgetGetWidth(win.root)) == w && int(gtkWidgetGetHeight(win.root)) == h }, time.Second)
}
func gkGTKWindowPosition(pointer unsafe.Pointer, x, y *cInt) {
	*x, *y = 0, 0
	if win := nativeWin(pointer); win != nil {
		a, b := win.position()
		*x, *y = cInt(a), cInt(b)
	}
}
func gkGTKWindowSetPosition(pointer unsafe.Pointer, x, y cInt) {
	win := nativeWin(pointer)
	if win == nil {
		return
	}
	beforeX, beforeY := win.position()
	win.havePosition = true
	if gtkWidgetGetMapped(win.widget) == 0 {
		win.x, win.y = int(x), int(y)
		win.applyHints()
		return
	}
	win.moveFrame(int(x), int(y))
	pumpNativeUntil(func() bool { a, b := win.position(); return a == int(x) && b == int(y) }, time.Second)
	win.x, win.y = win.position()
	if win.x != beforeX || win.y != beforeY {
		gkGTKGoWindowMove(cUintptr(win.handle), cInt(win.x), cInt(win.y))
	}
}
func gkGTKWindowVisible(pointer unsafe.Pointer) cInt {
	if win := nativeWin(pointer); win != nil {
		return cInt(gtkWidgetGetVisible(win.widget))
	}
	return 0
}
func gkGTKWindowSetVisible(pointer unsafe.Pointer, visible cInt) {
	win := nativeWin(pointer)
	if win == nil {
		return
	}
	if visible != 0 {
		if win.fullscreenTarget {
			gtkWindowFullscreen(win.window())
		}
		gtkWindowPresent(win.window())
		pumpNativeUntil(win.mapped, time.Second)
	} else {
		gtkWidgetSetVisible(win.widget, 0)
		pumpNativeUntil(func() bool { return gtkWidgetGetMapped(win.widget) == 0 }, 200*time.Millisecond)
	}
}
func gkGTKWindowBringToFront(pointer unsafe.Pointer) {
	if win := nativeWin(pointer); win != nil && gtkWidgetGetVisible(win.widget) != 0 {
		if display, window := win.xdisplay(), win.xwindow(); display != nil && window != 0 {
			xRaiseWindow(display, window)
			xFlush(display)
		}
	}
}
func gkGTKWindowSetTitle(pointer unsafe.Pointer, title *cChar) {
	if win := nativeWin(pointer); win != nil {
		gtkWindowSetTitle(win.window(), title)
	}
}
func gkGTKWindowNativeHandle(pointer unsafe.Pointer) cUintptr {
	if win := nativeWin(pointer); win != nil {
		return cUintptr(win.xwindow())
	}
	return 0
}
func gkGTKWindowNativeDisplayHandle(pointer unsafe.Pointer) cUintptr {
	if win := nativeWin(pointer); win != nil {
		return cUintptr(uintptr(unsafe.Pointer(win.xdisplay())))
	}
	return 0
}
func gkGTKWindowNativePointer(pointer unsafe.Pointer) unsafe.Pointer { return pointer }
func gkGTKWindowNativeDisplay(pointer unsafe.Pointer) unsafe.Pointer {
	if win := nativeWin(pointer); win != nil {
		return unsafe.Pointer(win.xdisplay())
	}
	return nil
}
func gkGTKWindowSetHandle(pointer unsafe.Pointer, handle cUintptr) {
	if win := nativeWin(pointer); win != nil {
		win.handle = uintptr(handle)
	}
}
func gkGTKWindowAttach(pointer unsafe.Pointer, handle cUintptr) {
	gkGTKWindowSetHandle(pointer, handle)
}
func gkGTKWindowDetach(pointer unsafe.Pointer) { gkGTKWindowSetHandle(pointer, 0) }

func gkGTKWindowAcceptFiles(pointer unsafe.Pointer, accept cInt) {
	if win := nativeWin(pointer); win != nil {
		win.acceptFiles = accept != 0
	}
}

func gkGTKWindowCanBecomeKey(pointer unsafe.Pointer) cInt {
	if win := nativeWin(pointer); win != nil {
		return cInt(gtkWidgetGetCanFocus(win.widget))
	}
	return 0
}

func gkGTKWindowSetSizeLimits(pointer unsafe.Pointer, minWidth, minHeight, maxWidth, maxHeight cInt) {
	if win := nativeWin(pointer); win != nil {
		win.minWidth, win.minHeight, win.maxWidth, win.maxHeight = max(0, int(minWidth)), max(0, int(minHeight)), max(0, int(maxWidth)), max(0, int(maxHeight))
		setNativeLayoutMinimum(win.root, win.minWidth, win.minHeight)
		win.applyHints()
		w, h := win.clampSize(win.width, win.height)
		if w != win.width || h != win.height {
			gkGTKWindowSetSize(pointer, cInt(w), cInt(h))
		}
	}
}
func gkGTKWindowSetStyle(pointer unsafe.Pointer, resizable, borderless cInt) {
	if win := nativeWin(pointer); win != nil {
		win.resizable, win.borderless = resizable != 0, borderless != 0
		gtkWindowSetResizable(win.window(), nativeBool(win.resizable))
		gtkWindowSetDecorated(win.window(), nativeBool(!win.borderless))
		win.applyHints()
	}
}
func gkGTKWindowSetAlwaysOnTop(pointer unsafe.Pointer, alwaysOnTop cInt) {
	if win := nativeWin(pointer); win != nil {
		win.alwaysOnTop = alwaysOnTop != 0
		win.sendState(win.alwaysOnTop, "_NET_WM_STATE_ABOVE", "")
	}
}
