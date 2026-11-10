package win32

import (
	"errors"
	"math"
	"unsafe"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

// Key actions, numbered as in GLFW.
const (
	keyReleased = 0
	keyPressed  = 1
	keyRepeat   = 2
	keyCount    = 349
)

// Pointing-device buttons and their state. Numbering matches GLFW.
const (
	pointerReleased    = 0
	pointerPressed     = 1
	pointerButtonLeft  = 0
	pointerButtonRight = 1
	pointerButtonMid   = 2
	pointerButtonCount = 8
)

// chromeBar is a bar above or below the content of a window: a toolbar, a
// status bar, or an info bar.
type chromeBar struct {
	view   *nativeView
	top    bool
	height int
}

// nativeWindow is a top-level window.
type nativeWindow struct {
	// window is the Go window that receives the events, until it is destroyed.
	window *Window
	hwnd   hwnd
	// root is the layout that holds the content.
	root    hwnd
	owner   *nativeWindow
	focus   *nativeView
	menubar hmenu
	chrome  []chromeBar

	shouldClose                              bool
	utility, borderless, resizable           bool
	alwaysOnTop, transparent                 bool
	havePosition                             bool
	x, y                                     int
	minWidth, minHeight, maxWidth, maxHeight int
	cursorMode                               pointer.Mode
	virtualX, virtualY                       float64
	lastX, lastY                             int
	haveLast                                 bool
	warping                                  bool
	scrollX, scrollY                         float64
	fullscreen                               bool
	windowedRect                             rect
	windowedStyle                            uintptr
	shown                                    bool
	// pendingFullscreen is a fullscreen request of a window that has not been
	// shown, which it enters when it is shown.
	pendingFullscreen bool
	acceptFiles       bool
	theme             int
	resolvedTheme     int
	destroyed         bool
	active            bool
	// tracking reports that the window tracks the pointer leaving it.
	tracking                      bool
	inside                        bool
	buttons                       uint32
	highSurrogate                 uint32
	minimized                     bool
	reportedWidth, reportedHeight int
}

func (w *nativeWindow) destroy() {
}

var (
	windowList  []*nativeWindow
	appReceiver eventReceiver
	appActive   = -1
	captureView *nativeView
)

const windowClass = "GameKitWindow"

func windowLive(win *nativeWindow) bool {
	if win == nil {
		return false
	}
	for _, live := range windowList {
		if live == win {
			return true
		}
	}
	return false
}

// windowOf returns the window that holds a window of Windows.
func windowOf(h hwnd) *nativeWindow {
	if h == 0 {
		return nil
	}
	root := win32GetAncestor(h, gaRoot)
	for _, win := range windowList {
		if win.hwnd == root {
			return win
		}
	}
	return nil
}

// notify reports an event to the Go window, if it has not been destroyed.
func (w *nativeWindow) notify(event any) {
	if w != nil && w.window != nil {
		w.window.receiveEvent(event)
	}
}

// scale returns the number of physical pixels in one logical unit.
func (w *nativeWindow) scale() float64 {
	if w != nil && w.hwnd != 0 {
		return scaleOf(w.hwnd)
	}
	return float64(win32GetDpiForSystem()) / 96
}

// ---- Theme ----

func requestedTheme() int {
	theme := 0
	for _, win := range windowList {
		if win.theme != 0 {
			theme = win.theme
		}
	}
	return theme
}

// resolvedTheme returns the theme of the application: 1 light, 2 dark.
func resolvedTheme() int {
	if theme := requestedTheme(); theme != 0 {
		return theme
	}
	if systemDark() {
		return 2
	}
	return 1
}

// themeOf returns the theme of a window: 1 light, 2 dark.
func (w *nativeWindow) themeOf() int {
	if w != nil && w.theme != 0 {
		return w.theme
	}
	return resolvedTheme()
}

// themeChild gives a native control the visual style of a theme and redraws
// it.
func themeChild(child hwnd, theme int) {
	view := viewOf(child)
	if view != nil && view.outer == child {
		view.resolveStyle()
	}
	if view != nil && view.ext != nil {
		view.ext.themeChanged(view)
	}
	if name := className(child); name != layoutClass && name != controlClass {
		applyThemeTo(child, theme, "")
	}
	win32InvalidateRect(child, nil, true)
}

func themeChildCallback() uintptr {
	return callback("themeChild", func(child hwnd, theme uintptr) uintptr {
		themeChild(child, int(theme))
		return 1
	})
}

func (w *nativeWindow) applyTheme() {
	if w == nil || w.hwnd == 0 {
		return
	}
	theme := w.themeOf()
	dark := int32(0)
	if theme == 2 {
		dark = 1
	}
	win32DwmSetWindowAttribute(w.hwnd, dwmwaUseImmersiveDarkMode, unsafe.Pointer(&dark), 4)
	win32EnumChildWindows(w.hwnd, themeChildCallback(), uintptr(theme))
	// The views of the window that are not in it now, such as the content of
	// another tab or page, wait in the parking window.
	for child := win32GetWindow(parking(), gwChild); child != 0; child = win32GetWindow(child, gwHwndNext) {
		if view := viewOf(child); view != nil && view.window == w {
			themeChild(child, theme)
			win32EnumChildWindows(child, themeChildCallback(), uintptr(theme))
		}
	}
	win32RedrawWindow(w.hwnd, nil, 0, rdwInvalidate|rdwErase|rdwAllChildren|rdwFrame)
}

func notifyThemes() {
	for _, win := range windowList {
		resolved := win.themeOf()
		if resolved != win.resolvedTheme {
			win.resolvedTheme = resolved
			win.applyTheme()
			win.notify(themeEvent{theme: ui.Theme(resolved)})
		}
	}
}

// applyThemes applies a change of the system theme to the windows.
func applyThemes() {
	notifyThemes()
}

// ---- Content area ----

func (w *nativeWindow) chromeTop() int {
	height := 0
	for _, bar := range w.chrome {
		if bar.top && bar.view != nil && bar.view.visible {
			height += bar.height
		}
	}
	return height
}

func (w *nativeWindow) chromeBottom() int {
	height := 0
	for _, bar := range w.chrome {
		if !bar.top && bar.view != nil && bar.view.visible {
			height += bar.height
		}
	}
	return height
}

// layout positions the chrome and the content of a window.
func (w *nativeWindow) layout() {
	if w == nil || w.hwnd == 0 || w.root == 0 {
		return
	}
	client := clientSize(w.hwnd)
	top := 0.0
	for _, bar := range w.chrome {
		if bar.top && bar.view != nil && bar.view.visible {
			bar.view.x = 0
			bar.view.y = int(top)
			bar.view.width = int(math.Round(float64(client.width)))
			bar.view.height = bar.height
			bar.view.place()
			if bar.view.ext != nil {
				bar.view.ext.layout(bar.view)
			}
			top += float64(bar.height)
		}
	}
	bottom := float64(client.height)
	for i := len(w.chrome) - 1; i >= 0; i-- {
		bar := w.chrome[i]
		if !bar.top && bar.view != nil && bar.view.visible {
			bottom -= float64(bar.height)
			bar.view.x = 0
			bar.view.y = int(math.Round(bottom))
			bar.view.width = int(math.Round(float64(client.width)))
			bar.view.height = bar.height
			bar.view.place()
			if bar.view.ext != nil {
				bar.view.ext.layout(bar.view)
			}
		}
	}
	placeWindow(w.root, 0, top, float64(client.width), max(bottom-top, 0))
}

// addChrome puts a view above (top) or below the content.
func (w *nativeWindow) addChrome(view *nativeView, top bool, height int) {
	if w == nil || view == nil {
		return
	}
	for i := range w.chrome {
		if w.chrome[i].view == view {
			w.chrome[i].top = top
			w.chrome[i].height = height
			w.layout()
			return
		}
	}
	w.chrome = append(w.chrome, chromeBar{view: view, top: top, height: height})
	reparent(view.outer, w.hwnd)
	win32ShowWindow(view.outer, swShowNA)
	w.layout()
}

func (w *nativeWindow) removeChrome(view *nativeView) {
	if w == nil {
		return
	}
	for i, bar := range w.chrome {
		if bar.view == view {
			w.chrome = append(w.chrome[:i], w.chrome[i+1:]...)
			if view.outer != 0 && win32IsWindow(view.outer) {
				win32ShowWindow(view.outer, swHide)
				reparent(view.outer, parking())
			}
			w.layout()
			return
		}
	}
}

// contentOrigin returns the screen position of the content area in physical
// pixels.
func (w *nativeWindow) contentOrigin() point {
	var origin point
	if w == nil || w.hwnd == 0 {
		return origin
	}
	if w.root != 0 {
		win32ClientToScreen(w.root, &origin)
	} else {
		win32ClientToScreen(w.hwnd, &origin)
	}
	return origin
}

// contentSize returns the size of the content area in logical units.
func (w *nativeWindow) contentSize() (float64, float64) {
	var size d2d1SizeF
	if w.root != 0 {
		size = clientSize(w.root)
	} else {
		size = clientSize(w.hwnd)
	}
	return float64(size.width), float64(size.height)
}

// position returns the position of the window frame in logical units,
// relative to the content of the owner for an owned window.
func (w *nativeWindow) position() (int, int) {
	if w == nil || w.hwnd == 0 {
		return 0, 0
	}
	if !w.shown {
		return w.x, w.y
	}
	var frame rect
	win32GetWindowRect(w.hwnd, &frame)
	scale := w.scale()
	px, py := float64(frame.left), float64(frame.top)
	if w.owner != nil {
		origin := w.owner.contentOrigin()
		px -= float64(origin.x)
		py -= float64(origin.y)
	}
	return int(math.Round(px / scale)), int(math.Round(py / scale))
}

func (w *nativeWindow) moveFrame(x, y int) {
	if w == nil || w.hwnd == 0 {
		return
	}
	scale := w.scale()
	px, py := float64(x)*scale, float64(y)*scale
	if w.owner != nil {
		origin := w.owner.contentOrigin()
		px += float64(origin.x)
		py += float64(origin.y)
	}
	win32SetWindowPos(w.hwnd, 0, int32(math.Round(px)), int32(math.Round(py)), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate|swpNoOwnerZOrder)
}

// outerSize returns the size of the window frame for a content size in
// logical units.
func (w *nativeWindow) outerSize(width, height float64) (int32, int32) {
	scale := w.scale()
	area := rect{
		right:  int32(math.Round(width * scale)),
		bottom: int32(math.Round((height + float64(w.chromeTop()+w.chromeBottom())) * scale)),
	}
	dpi := win32GetDpiForWindow(w.hwnd)
	if dpi == 0 {
		dpi = 96
	}
	style := uint32(win32GetWindowLongPtr(w.hwnd, gwlStyle))
	extended := uint32(win32GetWindowLongPtr(w.hwnd, gwlExStyle))
	win32AdjustWindowRectExForDpi(&area, style, w.menubar != 0, extended, dpi)
	return area.right - area.left, area.bottom - area.top
}

func (w *nativeWindow) resizeClient(width, height int) {
	cx, cy := w.outerSize(float64(width), float64(height))
	win32SetWindowPos(w.hwnd, 0, 0, 0, cx, cy, swpNoMove|swpNoZOrder|swpNoActivate|swpNoOwnerZOrder)
	// A menu bar can wrap to more lines at a narrow width, which changes the
	// client area, so the size is checked once more.
	_, h := w.contentSize()
	if int(math.Round(h)) != height && !win32IsZoomed(w.hwnd) && !w.fullscreen {
		var frame rect
		win32GetWindowRect(w.hwnd, &frame)
		extra := int32(math.Round((float64(height) - h) * w.scale()))
		win32SetWindowPos(w.hwnd, 0, 0, 0, frame.right-frame.left, frame.bottom-frame.top+extra,
			swpNoMove|swpNoZOrder|swpNoActivate|swpNoOwnerZOrder)
	}
}

// ---- Cursor ----

func (w *nativeWindow) center() (float64, float64) {
	width, height := w.contentSize()
	return math.Floor(width / 2), math.Floor(height / 2)
}

func (w *nativeWindow) warpToCenter() {
	x, y := w.center()
	origin := w.contentOrigin()
	scale := w.scale()
	win32SetCursorPos(origin.x+int32(math.Round(x*scale)), origin.y+int32(math.Round(y*scale)))
	w.warping = true
}

func (w *nativeWindow) clipCursor(clip bool) {
	if !clip || w.hwnd == 0 {
		win32ClipCursor(nil)
		return
	}
	content := w.root
	if content == 0 {
		content = w.hwnd
	}
	var area rect
	win32GetClientRect(content, &area)
	mapWindowRect(content, 0, &area)
	win32ClipCursor(&area)
}

// pointerHidden reports whether the pointer is hidden over a window.
func (w *nativeWindow) pointerHidden() bool {
	return w != nil && w.cursorMode != pointer.Normal && w.active
}

// ---- Input ----

func (w *nativeWindow) releaseButtons() {
	if w.window == nil {
		w.buttons = 0
		return
	}
	for button := range pointerButtonCount {
		if w.buttons&(1<<button) != 0 {
			w.window.dispatchNativeButton(button, pointerReleased, modifiers())
		}
	}
	w.buttons = 0
}

// buttonOf returns the button of a mouse message, or -1, and whether the
// message presses it.
func buttonOf(message uint32, wparam uintptr) (button int, pressed bool) {
	switch message {
	case wmLButtonDown, wmRButtonDown, wmMButtonDown, wmXButtonDown,
		wmLButtonDblClk, wmRButtonDblClk, wmMButtonDblClk, wmXButtonDblClk:
		pressed = true
	}
	switch message {
	case wmLButtonDown, wmLButtonUp, wmLButtonDblClk:
		return pointerButtonLeft, pressed
	case wmRButtonDown, wmRButtonUp, wmRButtonDblClk:
		return pointerButtonRight, pressed
	case wmMButtonDown, wmMButtonUp, wmMButtonDblClk:
		return pointerButtonMid, pressed
	case wmXButtonDown, wmXButtonUp, wmXButtonDblClk:
		if hiword(wparam) == xbutton1 {
			return 3, pressed
		}
		return 4, pressed
	default:
		return -1, false
	}
}

// pointerPoint returns the position of the pointer in the content of a
// window in logical units.
func (w *nativeWindow) pointerPoint() (float64, float64) {
	var cursor point
	win32GetCursorPos(&cursor)
	origin := w.contentOrigin()
	scale := w.scale()
	return float64(cursor.x-origin.x) / scale, float64(cursor.y-origin.y) / scale
}

func (w *nativeWindow) pointerMoved(target hwnd) {
	x, y := w.pointerPoint()
	if !w.inside {
		w.inside = true
		w.notify(pointerEnterEvent{crossing: ui.PointerCrossingEvent{X: x, Y: y}})
	}
	trackMouseLeave(target)
	var dx, dy float64
	if w.haveLast {
		dx = x - float64(w.lastX)
		dy = y - float64(w.lastY)
	}
	if w.cursorMode == pointer.Disabled && w.active {
		cx, cy := w.center()
		if w.warping && math.Abs(x-cx) < 1 && math.Abs(y-cy) < 1 {
			w.warping = false
			return
		}
		dx = x - cx
		dy = y - cy
		w.virtualX += dx
		w.virtualY += dy
		w.warpToCenter()
		x = w.virtualX
		y = w.virtualY
	} else {
		w.lastX = int(math.Round(x))
		w.lastY = int(math.Round(y))
		w.haveLast = true
	}
	w.notify(ui.PointerMoveEvent{X: x, Y: y, DeltaX: dx, DeltaY: dy, Modifiers: keyboard.ModifierKey(modifiers())})
}

// left reports that the pointer left a window when the window that the pointer
// was over reports it left, unless the pointer is over another window of the
// same top-level window.
func (w *nativeWindow) left() {
	var cursor point
	win32GetCursorPos(&cursor)
	under := win32WindowFromPoint(cursor)
	if under != 0 && win32GetAncestor(under, gaRoot) == w.hwnd {
		return
	}
	if w.inside {
		w.inside = false
		w.haveLast = false
		x, y := w.pointerPoint()
		w.notify(pointerLeaveEvent{crossing: ui.PointerCrossingEvent{X: x, Y: y}})
	}
}

// tab moves the focus to the next or previous control.
func (w *nativeWindow) tab(focus hwnd, backward bool) bool {
	next := win32GetNextDlgTabItem(w.hwnd, focus, backward)
	if next == 0 || next == focus {
		return false
	}
	win32SetFocus(next)
	if view := viewFromDescendant(next); view != nil {
		view.keyboardFocus = true
		view.invalidate()
	}
	if className(next) == wcEdit {
		win32SendMessage(next, emSetSel, 0, ^uintptr(0))
	}
	return true
}

// windowInput observes a message before it is dispatched. The window gets the
// keyboard and pointer input of all its controls, as one stream, while the
// control under the input still receives it. It returns true when it consumed
// the message.
func windowInput(message *msg) bool {
	kind := message.message
	if dragTrack(message) {
		return true
	}
	keyboard := kind >= wmKeyFirst && kind <= wmKeyLast
	mouse := (kind >= wmMouseFirst && kind <= wmMouseLast) || kind == wmMouseLeave
	if !keyboard && !mouse {
		return false
	}
	win := windowOf(message.hwnd)
	if win == nil || win.destroyed {
		return false
	}
	switch kind {
	case wmKeyDown, wmSysKeyDown, wmKeyUp, wmSysKeyUp:
		if menuAccelerator(win, message) || acceleratorPress(win, message) {
			return true
		}
		key := keyFromVirtualKey(message.wParam, message.lParam)
		if key >= 0 && key < keyCount && win.window != nil {
			action := keyPressed
			if kind == wmKeyUp || kind == wmSysKeyUp {
				action = keyReleased
			} else if message.lParam&(1<<30) != 0 {
				action = keyRepeat
			}
			win.window.dispatchNativeKey(key, int((message.lParam>>16)&0xff), action, modifiers())
		}
		if kind == wmKeyDown && message.wParam == vkTab && !keyDown(vkControl) {
			code := win32SendMessage(message.hwnd, wmGetDlgCode, message.wParam, uintptr(unsafe.Pointer(message)))
			if code&(dlgcWantTab|dlgcWantAllKeys) == 0 {
				return win.tab(message.hwnd, keyDown(vkShift))
			}
		}
		return false
	case wmChar, wmSysChar:
		unit := uint32(message.wParam)
		if unit >= 0xd800 && unit <= 0xdbff {
			win.highSurrogate = unit
			return false
		}
		codepoint := unit
		if unit >= 0xdc00 && unit <= 0xdfff && win.highSurrogate != 0 {
			codepoint = 0x10000 + ((win.highSurrogate - 0xd800) << 10) + (unit - 0xdc00)
		}
		win.highSurrogate = 0
		if codepoint >= 0x20 && codepoint != 0x7f && win.window != nil && !keyDown(vkControl) {
			win.window.dispatchChar(rune(codepoint))
		}
		return false
	case wmMouseMove:
		win.pointerMoved(message.hwnd)
		return false
	case wmMouseLeave:
		win.left()
		return false
	case wmMouseWheel, wmMouseHWheel:
		delta := float64(int16(hiword(message.wParam))) / wheelDelta
		var dx, dy float64
		if kind == wmMouseHWheel {
			dx = -delta
		} else {
			dy = delta
		}
		win.scrollX += dx
		win.scrollY += dy
		if win.window != nil {
			win.window.dispatchScroll(dx, dy)
		}
		// Windows sends the wheel to the window with the keyboard focus. The
		// window under the pointer gets it instead, as on macOS and Linux.
		under := win32WindowFromPoint(point{x: int32(signedLoword(message.lParam)), y: int32(signedHiword(message.lParam))})
		if under != 0 && under != message.hwnd && win32GetWindowThreadProcessID(under, nil) == win32GetCurrentThreadID() {
			win32SendMessage(under, kind, message.wParam, message.lParam)
			return true
		}
		return false
	default:
		button, pressed := buttonOf(kind, message.wParam)
		if button < 0 || button >= pointerButtonCount {
			return false
		}
		mask := uint32(1) << button
		if pressed {
			win.buttons |= mask
			if win.window != nil {
				win.window.dispatchNativeButton(button, pointerPressed, modifiers())
			}
		} else if win.buttons&mask != 0 {
			win.buttons &^= mask
			if win.window != nil {
				win.window.dispatchNativeButton(button, pointerReleased, modifiers())
			}
		}
		return false
	}
}

// ---- Application ----

func updateApp() {
	active := 0
	for _, win := range windowList {
		if win.active {
			active = 1
		}
	}
	if active != appActive {
		appActive = active
		send(appReceiver, appActivationEvent{active: active != 0})
	}
}

// setAppReceiver sets the receiver of the events of the application.
func setAppReceiver(receiver eventReceiver) {
	appReceiver = receiver
}

// ---- Window state ----

func (w *nativeWindow) reportSize() {
	width, height := w.contentSize()
	wi, he := int(math.Round(width)), int(math.Round(height))
	if wi == w.reportedWidth && he == w.reportedHeight {
		return
	}
	w.reportedWidth = wi
	w.reportedHeight = he
	w.notify(windowResizeEvent{size: ui.Size{Width: wi, Height: he}})
}

func (w *nativeWindow) stateChanged() {
	minimized := win32IsIconic(w.hwnd) || !win32IsWindowVisible(w.hwnd)
	if minimized != w.minimized {
		w.minimized = minimized
	}
}

func (w *nativeWindow) dropFiles(drop uintptr) {
	count := win32DragQueryFile(drop, 0xffffffff, nil)
	paths := make([]string, 0, count)
	for i := range count {
		length := win32DragQueryFile(drop, i, nil)
		path := make([]uint16, length+1)
		win32DragQueryFile(drop, i, path)
		paths = append(paths, goString(path))
	}
	win32DragFinish(drop)
	if count != 0 && w.acceptFiles {
		w.notify(fileDropEvent{paths: paths})
	}
}

// windowsByHWND maps the windows of the class to their state.
var windowsByHWND = map[hwnd]*nativeWindow{}

func refontChildCallback() uintptr {
	return callback("refontChild", func(child hwnd, data uintptr) uintptr {
		view := viewOf(child)
		if view != nil && (view.outer == child || view.control == child) {
			view.applyFont()
			if view.ext != nil {
				view.ext.themeChanged(view)
			}
		}
		return 1
	})
}

// ---- Dark menu bar ----
//
// Windows draws the menu bar in light colors in the dark theme. Like other
// Win32 applications with a dark theme, the window draws the bar and its items
// in dark colors through the messages that the menu bar sends to draw itself,
// which the Windows headers do not declare.

const (
	uahDrawMenu     = 0x0091
	uahDrawMenuItem = 0x0092
)

type uahMenu struct {
	menu  hmenu
	hdc   hdc
	flags uint32
}

type uahMenuItem struct {
	position     int32
	metrics      [8]uint32
	popupMetrics [5]uint32
}

type uahDrawMenuItemInfo struct {
	draw drawItemStruct
	menu uahMenu
	item uahMenuItem
}

// menuBarRect returns the rectangle of the menu bar in window coordinates.
func menuBarRect(h hwnd) rect {
	info := menuBarInfo{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if !win32GetMenuBarInfo(h, objidMenu, 0, &info) {
		return rect{}
	}
	var window rect
	win32GetWindowRect(h, &window)
	bar := info.rcBar
	bar.left -= window.left
	bar.right -= window.left
	bar.top -= window.top
	bar.bottom -= window.top
	return bar
}

// darkMenuBarLine paints over the light line that Windows draws below the
// menu bar.
func darkMenuBarLine(h hwnd, pal *palette) {
	bar := menuBarRect(h)
	if bar.bottom <= bar.top {
		return
	}
	line := rect{left: bar.left, top: bar.bottom, right: bar.right, bottom: bar.bottom + 1}
	dc := win32GetWindowDC(h)
	fillRectColor(dc, &line, pal.window)
	win32ReleaseDC(h, dc)
}

func darkMenuBar(win *nativeWindow, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if win == nil || win.menubar == 0 || win.themeOf() != 2 {
		return false
	}
	pal := paletteFor(2)
	switch message {
	case uahDrawMenu:
		menu := (*uahMenu)(memoryAt(lparam))
		bar := menuBarRect(h)
		fillRectColor(menu.hdc, &bar, pal.window)
		*result = 1
		return true
	case uahDrawMenuItem:
		item := (*uahDrawMenuItemInfo)(memoryAt(lparam))
		var text [256]uint16
		info := menuItemInfo{fMask: miimString, dwTypeData: &text[0], cch: 255}
		info.cbSize = uint32(unsafe.Sizeof(info))
		win32GetMenuItemInfo(item.menu.menu, uint32(item.item.position), true, &info)
		state := item.draw.itemState
		fill := pal.window
		if state&(odsSelected|odsHotLight) != 0 {
			fill = pal.subtle
			if state&odsSelected != 0 {
				fill = pal.subtlePressed
			}
		}
		fillRectColor(item.menu.hdc, &item.draw.rcItem, fill)
		disabled := state&(odsInactive|odsDisabled|odsGrayed) != 0
		win32SetBkMode(item.menu.hdc, transparent)
		textColor := pal.text
		if disabled {
			textColor = pal.textDisabled
		}
		win32SetTextColor(item.menu.hdc, textColor.colorRef())
		flags := uint32(dtCenter | dtSingleLine | dtVCenter)
		if state&odsNoAccel != 0 {
			flags |= dtHidePrefix
		}
		win32DrawText(item.menu.hdc, goString(text[:]), &item.draw.rcItem, flags)
		*result = 1
		return true
	case wmNCPaint, wmNCActivate:
		*result = win32DefWindowProc(h, message, wparam, lparam)
		darkMenuBarLine(h, pal)
		return true
	default:
		return false
	}
}

func windowProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	win := windowsByHWND[h]
	var result uintptr
	if message != wmDrawItem && forwardNotification(message, wparam, lparam, &result) {
		return result
	}
	if message != wmNCActivate && darkMenuBar(win, h, message, wparam, lparam, &result) {
		return result
	}
	switch message {
	case wmNCCreate:
		if creating != nil {
			win = creating
			win.hwnd = h
			windowsByHWND[h] = win
		}
		win32EnableNonClientDpiScaling(h)
	case wmClose:
		if win != nil {
			if win.window != nil && !win.window.answerQuery(closeRequest{}) {
				return 0
			}
			win.shouldClose = true
			if win.utility {
				win32ShowWindow(h, swHide)
			}
		}
		return 0
	case wmEraseBkgnd:
		var client rect
		win32GetClientRect(h, &client)
		fillRectColor(hdc(wparam), &client, paletteFor(win.themeOf()).window)
		return 1
	case wmSize:
		if win != nil {
			win.layout()
			if wparam != sizeMinimized {
				win.reportSize()
			}
			win.stateChanged()
		}
	case wmShowWindow:
		if win != nil && wparam == 0 {
			later(func() {
				if windowLive(win) {
					win.stateChanged()
				}
			})
		}
	case wmGetMinMaxInfo:
		if win != nil && win.hwnd != 0 {
			info := (*minMaxInfo)(memoryAt(lparam))
			if win.minWidth > 0 || win.minHeight > 0 {
				cx, cy := win.outerSize(float64(win.minWidth), float64(win.minHeight))
				if win.minWidth > 0 {
					info.ptMinTrackSize.x = cx
				}
				if win.minHeight > 0 {
					info.ptMinTrackSize.y = cy
				}
			}
			if (win.maxWidth > 0 || win.maxHeight > 0) && !win.fullscreen {
				width, height := float64(win.maxWidth), float64(win.maxHeight)
				if width <= 0 {
					width = 100000
				}
				if height <= 0 {
					height = 100000
				}
				cx, cy := win.outerSize(width, height)
				if win.maxWidth > 0 {
					info.ptMaxTrackSize.x = cx
				}
				if win.maxHeight > 0 {
					info.ptMaxTrackSize.y = cy
				}
			}
			return 0
		}
	case wmActivate:
		if win != nil {
			active := loword(wparam) != waInactive
			if active != win.active {
				win.active = active
				if !active {
					win.releaseButtons()
					if win.cursorMode == pointer.Disabled {
						win.clipCursor(false)
					}
				} else if win.cursorMode == pointer.Disabled {
					win.clipCursor(true)
				}
				if active {
					win.notify(focusEvent{})
				} else {
					win.notify(blurEvent{})
				}
				updateApp()
			}
		}
	case wmNCActivate:
		// The window keeps the look of the active window while a popover that
		// it owns is active.
		kept := wparam == 0 && popoverOwns(h)
		if kept {
			wparam = 1
		}
		if darkMenuBar(win, h, message, wparam, lparam, &result) {
			return result
		}
		if kept {
			return win32DefWindowProc(h, message, wparam, lparam)
		}
	case wmSetFocus:
		if win != nil && win.focus.live() {
			win32SetFocus(win.focus.target())
			return 0
		}
	case wmSetCursor:
		if win != nil && win.pointerHidden() && loword(lparam) == htClient {
			win32SetCursor(0)
			return 1
		}
	case wmDpiChanged:
		suggested := (*rect)(memoryAt(lparam))
		win32SetWindowPos(h, 0, suggested.left, suggested.top, suggested.right-suggested.left,
			suggested.bottom-suggested.top, swpNoZOrder|swpNoActivate)
		win32EnumChildWindows(h, refontChildCallback(), 0)
		if win != nil {
			win.layout()
		}
		return 0
	case wmSettingChange, wmThemeChanged:
		applyThemes()
	case wmDropFiles:
		if win != nil {
			win.dropFiles(wparam)
		}
		return 0
	case wmCommand:
		if lparam == 0 && hiword(wparam) == 0 && menuCommand(h, int(loword(wparam))) {
			return 0
		}
	case wmQueryEndSession:
		if appReceiver != nil {
			if !ask(appReceiver, quitRequest{}) {
				win32ShutdownBlockReasonCreate(h, "The application has unsaved changes.")
				return 0
			}
			win32ShutdownBlockReasonDestroy(h)
		}
		return 1
	case wmCancelMode:
		if win != nil {
			win.releaseButtons()
		}
	case wmNCDestroy:
		delete(windowsByHWND, h)
	}
	return win32DefWindowProc(h, message, wparam, lparam)
}

// ---- Creation ----

func (w *nativeWindow) style() uint32 {
	if w.fullscreen || w.borderless {
		style := uint32(wsPopup | wsClipChildren)
		if w.borderless && !w.fullscreen {
			style |= wsMinimizeBox
		}
		return style
	}
	style := uint32(wsOverlapped | wsCaption | wsSysMenu | wsClipChildren)
	if !w.utility {
		style |= wsMinimizeBox
	}
	if w.resizable {
		style |= wsThickFrame
		if !w.utility {
			style |= wsMaximizeBox
		}
	}
	return style
}

func (w *nativeWindow) exStyle() uint32 {
	style := uint32(wsExControlParent)
	if w.alwaysOnTop {
		style |= wsExTopmost
	}
	if w.utility {
		style |= wsExToolWindow
	}
	if w.transparent {
		style |= wsExLayered
	}
	if w.acceptFiles {
		style |= wsExAcceptFiles
	}
	return style
}

var windowClassRegistered bool

func registerWindowClass() {
	if windowClassRegistered {
		return
	}
	windowClassRegistered = true
	class := wndClassEx{
		style:       csDblClks,
		lpfnWndProc: callback("window", windowProc),
		hInstance:   instance(),
		hCursor:     win32LoadCursor(0, idcArrow),
		hIcon:       win32LoadIcon(win32GetModuleHandle(""), 1),
	}
	if class.hIcon == 0 {
		class.hIcon = win32LoadIcon(0, idiApplication)
	}
	registerClass(&class, windowClass)
}

// creating is the window whose creation is in progress, which its window
// procedure adopts in WM_NCCREATE.
var creating *nativeWindow

// newNativeWindow creates a window with a content size. The position of a
// utility window is relative to the content of its owner.
func newNativeWindow(title string, width, height int, options WindowOptions, owner *nativeWindow) (*nativeWindow, error) {
	if err := initialize(); err != nil {
		return nil, err
	}
	registerWindowClass()
	cookie := activate()
	defer deactivate(cookie)
	win := &nativeWindow{
		owner:          owner,
		utility:        options.Kind == ui.WindowUtility,
		resizable:      options.Resizable,
		borderless:     options.Borderless,
		alwaysOnTop:    options.AlwaysOnTop,
		transparent:    options.TransparentFramebuffer,
		x:              options.Position.X,
		y:              options.Position.Y,
		havePosition:   owner != nil,
		cursorMode:     pointer.Normal,
		reportedWidth:  -1,
		reportedHeight: -1,
	}
	var parent hwnd
	if win.owner != nil {
		parent = win.owner.hwnd
	}
	creating = win
	h := win32CreateWindowEx(win.exStyle(), windowClass, title, win.style(), cwUseDefault, cwUseDefault, 400, 300,
		parent, 0, instance(), nil)
	creating = nil
	if h == 0 {
		return nil, errors.New("ui/win32: Windows could not create a window")
	}
	if win.transparent {
		win32SetLayeredWindowAttributes(h, 0, 255, lwaAlpha)
	}
	win.root = newLayout(h)
	windowList = append(windowList, win)
	win.resolvedTheme = resolvedTheme()
	win.applyTheme()
	win.resizeClient(max(width, 1), max(height, 1))
	if win.owner != nil {
		win.moveFrame(options.Position.X, options.Position.Y)
	}
	win.layout()
	if !options.Hidden {
		win.setVisible(true)
	}
	return win, nil
}

func (win *nativeWindow) close() {
	if !windowLive(win) {
		return
	}
	win.destroyed = true
	win.window = nil
	if captureView != nil && windowOf(captureView.outer) == win {
		captureView = nil
	}
	if win.cursorMode == pointer.Disabled {
		win32ClipCursor(nil)
	}
	for i, live := range windowList {
		if live == win {
			windowList = append(windowList[:i], windowList[i+1:]...)
			break
		}
	}
	win.focus = nil
	for _, bar := range win.chrome {
		if bar.view != nil && bar.view.outer != 0 && win32IsWindow(bar.view.outer) {
			win32ShowWindow(bar.view.outer, swHide)
			win32SetParent(bar.view.outer, parking())
		}
	}
	win.chrome = nil
	// The views of the window belong to the Go layer, which releases them;
	// they move to the parking window so destroying the window does not
	// destroy their windows.
	for child := win32GetWindow(win.root, gwChild); child != 0; {
		next := win32GetWindow(child, gwHwndNext)
		if viewOf(child) != nil {
			win32ShowWindow(child, swHide)
			win32SetParent(child, parking())
		}
		child = next
	}
	if win.menubar != 0 {
		win32SetMenu(win.hwnd, 0)
	}
	if win.owner != nil && win32IsWindowVisible(win.hwnd) && win32GetActiveWindow() == win.hwnd {
		win32SetActiveWindow(win.owner.hwnd)
	}
	win32DestroyWindow(win.hwnd)
	win.destroy()
	updateApp()
	notifyThemes()
}

// ---- Polling ----

func (win *nativeWindow) poll() {
	pumpOnce()
	if windowLive(win) && win.shown {
		win.stateChanged()
		if win.window != nil && win32IsWindowVisible(win.hwnd) {
			x, y := win.position()
			if !win.havePosition || x != win.x || y != win.y {
				moved := win.havePosition
				win.x = x
				win.y = y
				win.havePosition = true
				if moved {
					win.window.dispatchNativeMove(x, y)
				}
			}
		}
	}
	afterPoll()
}

// ---- Geometry ----

func (win *nativeWindow) size() (int, int) {
	if win == nil {
		return 0, 0
	}
	w, h := win.contentSize()
	return int(math.Round(w)), int(math.Round(h))
}

func (win *nativeWindow) framebufferSize() (int, int) {
	var client rect
	if win != nil && win.root != 0 {
		win32GetClientRect(win.root, &client)
	}
	return int(client.right), int(client.bottom)
}

func (win *nativeWindow) setSize(width int, height int) {
	if win == nil {
		return
	}
	width = max(width, 1)
	height = max(height, 1)
	if win.minWidth > 0 && width < win.minWidth {
		width = win.minWidth
	}
	if win.minHeight > 0 && height < win.minHeight {
		height = win.minHeight
	}
	if win.maxWidth > 0 && width > win.maxWidth {
		width = win.maxWidth
	}
	if win.maxHeight > 0 && height > win.maxHeight {
		height = win.maxHeight
	}
	if win.fullscreen {
		win.windowedRect.right = win.windowedRect.left + int32(math.Round(float64(width)*win.scale()))
		win.windowedRect.bottom = win.windowedRect.top + int32(math.Round(float64(height)*win.scale()))
		return
	}
	if win32IsZoomed(win.hwnd) {
		win32ShowWindow(win.hwnd, swRestore)
	}
	win.resizeClient(width, height)
}

func (win *nativeWindow) setPosition(x int, y int) {
	if win == nil {
		return
	}
	beforeX, beforeY := win.position()
	win.havePosition = true
	win.moveFrame(x, y)
	if !win.shown {
		win.x = x
		win.y = y
		return
	}
	afterX, afterY := win.position()
	win.x = afterX
	win.y = afterY
	if (afterX != beforeX || afterY != beforeY) && win.window != nil {
		win.window.dispatchNativeMove(afterX, afterY)
	}
}

// ---- Visibility ----

func (win *nativeWindow) visible() bool {
	return win != nil && win.hwnd != 0 && win32IsWindowVisible(win.hwnd)
}

func (win *nativeWindow) setVisible(visible bool) {
	if win == nil || win.hwnd == 0 {
		return
	}
	if visible {
		first := !win.shown
		win.shown = true
		if first && win.owner != nil {
			win.moveFrame(win.x, win.y)
		}
		placement := windowPlacement{}
		placement.length = uint32(unsafe.Sizeof(placement))
		win32GetWindowPlacement(win.hwnd, &placement)
		command := int32(swShow)
		switch {
		case placement.flags&wpfRestoreToMaximized != 0 && !win32IsWindowVisible(win.hwnd):
			command = swShowMaximized
		case win.utility:
			command = swShowNoActivate
		}
		if win32IsIconic(win.hwnd) {
			command = swRestore
		}
		win32ShowWindow(win.hwnd, command)
		if !win.utility {
			win32SetForegroundWindow(win.hwnd)
		}
		if win.pendingFullscreen {
			win.pendingFullscreen = false
			win.setFullscreen(true)
		}
		win32UpdateWindow(win.hwnd)
		win.layout()
		win.reportSize()
	} else {
		win32ShowWindow(win.hwnd, swHide)
	}
	win.stateChanged()
}

func (win *nativeWindow) bringToFront() {
	if win == nil || !win32IsWindowVisible(win.hwnd) {
		return
	}
	win32SetWindowPos(win.hwnd, hwndTop, 0, 0, 0, 0, swpNoMove|swpNoSize)
	win32SetForegroundWindow(win.hwnd)
}

func (win *nativeWindow) setTitle(title string) {
	if win != nil && win.hwnd != 0 {
		win32SetWindowText(win.hwnd, title)
	}
}

func (win *nativeWindow) setTheme(theme int) {
	if win == nil {
		return
	}
	win.theme = theme
	notifyThemes()
}

// ---- Native handles ----

func (win *nativeWindow) nativeHandle() uintptr {
	if win != nil {
		return uintptr(win.hwnd)
	}
	return 0
}

// moduleAddress returns the module of the program, which is the address where
// Windows loaded it, outside the memory of Go.
func moduleAddress() unsafe.Pointer {
	module := win32GetModuleHandle("")
	return *(*unsafe.Pointer)(unsafe.Pointer(&module))
}

// ---- Pointer ----

func (win *nativeWindow) cursorPosition() (float64, float64) {
	switch {
	case win != nil && win.cursorMode == pointer.Disabled:
		return win.virtualX, win.virtualY
	case win != nil && win.hwnd != 0:
		return win.pointerPoint()
	default:
		return 0, 0
	}
}

func (win *nativeWindow) scrollTotal() (float64, float64) {
	if win != nil {
		return win.scrollX, win.scrollY
	}
	return 0, 0
}

func (w *nativeWindow) applyCursorMode(previous pointer.Mode) {
	if previous == pointer.Disabled {
		w.clipCursor(false)
	}
	var cursor point
	win32GetCursorPos(&cursor)
	win32SetCursorPos(cursor.x, cursor.y)
	if w.cursorMode == pointer.Disabled && win32IsWindowVisible(w.hwnd) {
		w.clipCursor(w.active)
		w.warpToCenter()
	}
}

var uiThread uint32

func (win *nativeWindow) setCursorMode(mode pointer.Mode) {
	if win == nil || win.cursorMode == mode {
		return
	}
	previous := win.cursorMode
	if mode == pointer.Disabled {
		win.virtualX, win.virtualY = win.cursorPosition()
	}
	win.cursorMode = mode
	if win32GetCurrentThreadID() == uiThread {
		win.applyCursorMode(previous)
		return
	}
	later(func() {
		if windowLive(win) {
			win.applyCursorMode(previous)
		}
	})
}

// ---- Extended window contract ----

func (win *nativeWindow) setAcceptFiles(accept bool) {
	if win != nil && win.hwnd != 0 {
		win.acceptFiles = accept
		win32DragAcceptFiles(win.hwnd, accept)
	}
}

func (win *nativeWindow) canBecomeKey() bool {
	return win != nil && win.hwnd != 0 && win32IsWindowEnabled(win.hwnd)
}

func (win *nativeWindow) setSizeLimits(minWidth int, minHeight int, maxWidth int, maxHeight int) {
	if win == nil {
		return
	}
	win.minWidth = max(minWidth, 0)
	win.minHeight = max(minHeight, 0)
	win.maxWidth = max(maxWidth, 0)
	win.maxHeight = max(maxHeight, 0)
	width, height := win.size()
	clampedWidth, clampedHeight := width, height
	if win.minWidth > 0 && clampedWidth < win.minWidth {
		clampedWidth = win.minWidth
	}
	if win.minHeight > 0 && clampedHeight < win.minHeight {
		clampedHeight = win.minHeight
	}
	if win.maxWidth > 0 && clampedWidth > win.maxWidth {
		clampedWidth = win.maxWidth
	}
	if win.maxHeight > 0 && clampedHeight > win.maxHeight {
		clampedHeight = win.maxHeight
	}
	if clampedWidth != width || clampedHeight != height {
		win.setSize(clampedWidth, clampedHeight)
	}
}

// restyle applies the style of the window after a setting changes, keeping its
// content size.
func (w *nativeWindow) restyle() {
	width, height := w.size()
	visible := win32GetWindowLongPtr(w.hwnd, gwlStyle) & wsVisible
	win32SetWindowLongPtr(w.hwnd, gwlStyle, uintptr(w.style())|visible)
	win32SetWindowLongPtr(w.hwnd, gwlExStyle, uintptr(w.exStyle()))
	after := hwndNoTopmost
	if w.alwaysOnTop {
		after = hwndTopmost
	}
	win32SetWindowPos(w.hwnd, after, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpFrameChanged)
	if !w.fullscreen && !win32IsZoomed(w.hwnd) {
		w.resizeClient(width, height)
	}
}

func (win *nativeWindow) setStyle(resizable bool, borderless bool) {
	if win == nil {
		return
	}
	win.resizable = resizable
	win.borderless = borderless
	win.restyle()
}

func (win *nativeWindow) setAlwaysOnTop(alwaysOnTop bool) {
	if win == nil {
		return
	}
	win.alwaysOnTop = alwaysOnTop
	after := hwndNoTopmost
	if alwaysOnTop {
		after = hwndTopmost
	}
	win32SetWindowPos(win.hwnd, after, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
}

func (win *nativeWindow) isMinimized() bool {
	return win != nil && win.shown && win32IsIconic(win.hwnd)
}

func (win *nativeWindow) minimize() bool {
	if win == nil {
		return false
	}
	if win32IsIconic(win.hwnd) {
		return true
	}
	if win.fullscreen {
		return false
	}
	win.shown = true
	win32ShowWindow(win.hwnd, swMinimize)
	win.stateChanged()
	return win32IsIconic(win.hwnd)
}

func (win *nativeWindow) isMaximized() bool {
	if win == nil || win.fullscreen {
		return false
	}
	if !win32IsWindowVisible(win.hwnd) {
		placement := windowPlacement{}
		placement.length = uint32(unsafe.Sizeof(placement))
		win32GetWindowPlacement(win.hwnd, &placement)
		return placement.flags&wpfRestoreToMaximized != 0
	}
	return win32IsZoomed(win.hwnd)
}

func (win *nativeWindow) maximize() bool {
	if win == nil || win.fullscreen {
		return false
	}
	if !win.shown || !win32IsWindowVisible(win.hwnd) {
		placement := windowPlacement{}
		placement.length = uint32(unsafe.Sizeof(placement))
		win32GetWindowPlacement(win.hwnd, &placement)
		placement.flags |= wpfRestoreToMaximized
		placement.showCmd = swHide
		if win32IsWindowVisible(win.hwnd) {
			placement.showCmd = swShowMaximized
		}
		win32SetWindowPlacement(win.hwnd, &placement)
		return true
	}
	win32ShowWindow(win.hwnd, swMaximize)
	win.stateChanged()
	return true
}

func (win *nativeWindow) restore() bool {
	if win == nil {
		return false
	}
	if win.fullscreen {
		win.setFullscreen(false)
	}
	placement := windowPlacement{}
	placement.length = uint32(unsafe.Sizeof(placement))
	win32GetWindowPlacement(win.hwnd, &placement)
	if !win32IsWindowVisible(win.hwnd) && placement.flags&wpfRestoreToMaximized != 0 {
		placement.flags &^= wpfRestoreToMaximized
		win32SetWindowPlacement(win.hwnd, &placement)
	}
	if win32IsIconic(win.hwnd) || win32IsZoomed(win.hwnd) {
		win32ShowWindow(win.hwnd, swRestore)
		if win32IsZoomed(win.hwnd) {
			win32ShowWindow(win.hwnd, swRestore)
		}
	}
	win.stateChanged()
	return !win32IsIconic(win.hwnd)
}

func (win *nativeWindow) isFullscreen() bool {
	return win != nil && win.fullscreen
}

func (win *nativeWindow) setFullscreen(fullscreen bool) bool {
	if win == nil || win.hwnd == 0 {
		return false
	}
	if !win.shown && !win32IsWindowVisible(win.hwnd) {
		win.pendingFullscreen = fullscreen && !win.fullscreen
		if !fullscreen && win.fullscreen {
			win.fullscreen = false
		} else {
			return true
		}
	}
	if fullscreen == win.fullscreen {
		return true
	}
	if fullscreen {
		if win32IsZoomed(win.hwnd) {
			win32ShowWindow(win.hwnd, swRestore)
		}
		win32GetWindowRect(win.hwnd, &win.windowedRect)
		win.fullscreen = true
		monitor := win32MonitorFromWindow(win.hwnd, monitorDefaultToNearest)
		info := monitorInfo{}
		info.cbSize = uint32(unsafe.Sizeof(info))
		win32GetMonitorInfo(monitor, unsafe.Pointer(&info))
		visible := win32GetWindowLongPtr(win.hwnd, gwlStyle) & wsVisible
		win32SetWindowLongPtr(win.hwnd, gwlStyle, uintptr(win.style())|visible)
		if win.menubar != 0 {
			win32SetMenu(win.hwnd, 0)
		}
		bounds := info.rcMonitor
		win32SetWindowPos(win.hwnd, hwndTop, bounds.left, bounds.top, bounds.right-bounds.left, bounds.bottom-bounds.top,
			swpFrameChanged|swpNoOwnerZOrder)
		if win.shown && !win32IsWindowVisible(win.hwnd) {
			win32ShowWindow(win.hwnd, swShow)
		}
	} else {
		win.fullscreen = false
		visible := win32GetWindowLongPtr(win.hwnd, gwlStyle) & wsVisible
		win32SetWindowLongPtr(win.hwnd, gwlStyle, uintptr(win.style())|visible)
		if win.menubar != 0 {
			win32SetMenu(win.hwnd, win.menubar)
		}
		windowed := win.windowedRect
		after := hwndNoTopmost
		if win.alwaysOnTop {
			after = hwndTopmost
		}
		win32SetWindowPos(win.hwnd, after, windowed.left, windowed.top, windowed.right-windowed.left,
			windowed.bottom-windowed.top, swpFrameChanged|swpNoOwnerZOrder)
	}
	win.layout()
	win.reportSize()
	win.notify(fullscreenEvent{fullscreen: fullscreen})
	return true
}

// ---- Screens ----

// screenInfo describes the screen of a window in logical units.
type screenInfo struct {
	x, y, width, height                 float64
	workX, workY, workWidth, workHeight float64
	scale, dpi, refresh                 float64
	name                                string
}

func (win *nativeWindow) screen() screenInfo {
	var monitor uintptr
	if win != nil && win.hwnd != 0 {
		monitor = win32MonitorFromWindow(win.hwnd, monitorDefaultToNearest)
	} else {
		monitor = win32MonitorFromPoint(point{}, monitorDefaultToPrimary)
	}
	info := monitorInfoEx{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	win32GetMonitorInfo(monitor, unsafe.Pointer(&info))
	dpiX, dpiY := uint32(96), uint32(96)
	if procGetDpiForMonitor.find() != 0 {
		win32GetDpiForMonitor(monitor, 0, &dpiX, &dpiY)
	}
	scale := float64(dpiX) / 96
	mode := devMode{}
	mode.dmSize = uint16(unsafe.Sizeof(mode))
	refresh := 60.0
	if win32EnumDisplaySettings(&info.szDevice[0], enumCurrentSettings, &mode) && mode.dmDisplayFrequency > 1 {
		refresh = float64(mode.dmDisplayFrequency)
	}
	bounds, work := info.rcMonitor, info.rcWork
	screen := screenInfo{
		x:          float64(bounds.left) / scale,
		y:          float64(bounds.top) / scale,
		width:      float64(bounds.right-bounds.left) / scale,
		height:     float64(bounds.bottom-bounds.top) / scale,
		workX:      float64(work.left) / scale,
		workY:      float64(work.top) / scale,
		workWidth:  float64(work.right-work.left) / scale,
		workHeight: float64(work.bottom-work.top) / scale,
		scale:      scale,
		dpi:        float64(dpiX),
		refresh:    refresh,
		name:       goString(info.szDevice[:]),
	}
	if scale <= 0 {
		screen.scale = 1
	}
	device := displayDevice{}
	device.cb = uint32(unsafe.Sizeof(device))
	if win32EnumDisplayDevices(&info.szDevice[0], 0, &device, 0) && device.DeviceString[0] != 0 {
		screen.name = goString(device.DeviceString[:])
	}
	return screen
}

// systemPreferenceValues are the preferences of the system that the Go layer
// reports.
type systemPreferenceValues struct {
	theme              int
	textScale          float64
	highContrast       bool
	reduceMotion       bool
	reduceTransparency bool
	accent             themeColor
	rightToLeft        bool
	locale             string
}

// systemPreferences returns the preferences of the system, with the theme of a
// window, or of the system for nil.
func systemPreferenceValuesOf(win *nativeWindow) systemPreferenceValues {
	values := systemPreferenceValues{theme: win.themeOf()}
	textScale := uint32(100)
	size := uint32(4)
	if win32RegGetValue(hkeyCurrentUser, `Software\Microsoft\Accessibility`, "TextScaleFactor", rrfRtRegDword, nil,
		unsafe.Pointer(&textScale), &size) != errorSuccess {
		textScale = 100
	}
	animations := int32(1)
	win32SystemParametersInfo(spiGetClientAreaAnimation, 0, unsafe.Pointer(&animations), 0)
	transparency := uint32(1)
	size = 4
	if win32RegGetValue(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		"EnableTransparency", rrfRtRegDword, nil, unsafe.Pointer(&transparency), &size) != errorSuccess {
		transparency = 1
	}
	contrast := highContrast{}
	contrast.cbSize = uint32(unsafe.Sizeof(contrast))
	high := win32SystemParametersInfo(spiGetHighContrast, contrast.cbSize, unsafe.Pointer(&contrast), 0) &&
		contrast.dwFlags&hcfHighContrastOn != 0
	values.textScale = 1
	if textScale > 0 {
		values.textScale = float64(textScale) / 100
	}
	values.highContrast = high
	values.reduceMotion = animations == 0
	values.reduceTransparency = transparency == 0
	values.accent = accentColor()
	var name [localeNameMaxLength]uint16
	locale := "en-US"
	if win32GetUserDefaultLocaleName(&name[0], localeNameMaxLength) != 0 && name[0] != 0 {
		locale = goString(name[:])
	}
	var layout uint32
	values.rightToLeft = win32GetLocaleInfoEx(locale, localeIReadingLayout|localeReturnNumber,
		(*uint16)(unsafe.Pointer(&layout)), 2) != 0 && layout == 1
	values.locale = locale
	return values
}

// ---- Focus tracking ----

func trackFocus(win *nativeWindow) {
	if !windowLive(win) || win.destroyed {
		return
	}
	focus := win32GetFocus()
	var view *nativeView
	if focus != 0 && win32GetAncestor(focus, gaRoot) == win.hwnd {
		view = viewFromDescendant(focus)
	} else if !win.active {
		// An inactive window keeps the view that has its focus.
		return
	}
	if view == win.focus {
		return
	}
	previous := win.focus
	win.focus = view
	if previous.live() {
		send(previous.receiver, blurEvent{})
	}
	if view != nil {
		send(view.receiver, focusEvent{})
	}
}

// afterPoll runs after each event-loop pass.
func afterPoll() {
	for _, win := range append([]*nativeWindow(nil), windowList...) {
		trackFocus(win)
	}
}

func (win *nativeWindow) forgetFocus(receiver eventReceiver) {
	if !windowLive(win) {
		return
	}
	view := win.focus
	if !view.live() || view.receiver == receiver || view.receiver == nil {
		win.focus = nil
	}
}

// ---- Canvas capture and scale ----

func (view *nativeView) captureCanvasPointer(capture bool) bool {
	if capture {
		if !view.live() || windowOf(view.outer) == nil {
			return false
		}
		captureView = view
		win32SetCapture(view.outer)
	} else if captureView == view {
		captureView = nil
		if view.live() && win32GetCapture() == view.outer {
			win32ReleaseCapture()
		}
	}
	return true
}

func (view *nativeView) hasCapture() bool {
	return view != nil && captureView == view
}

// releaseCapture forgets the capture of the pointer by a view.
func releaseCapture(view *nativeView) {
	if captureView == view {
		captureView = nil
	}
}
