package win32

import (
	"errors"
	"math"
	"slices"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Toolbar controls ----
//
// A toolbar and a command bar are toolbar controls of Windows with a button for
// each action. A flexible space is a separator whose width takes the room that
// the buttons leave.

// Kinds of toolbar items.
const (
	toolAction = iota
	toolSeparator
	toolSpace
	toolFlexibleSpace
)

type toolItem struct {
	kind    int
	id      string
	label   string
	icon    *iconImage
	enabled bool
	// image is the index of the icon in the image list of the toolbar.
	image int32
}

// toolItems returns the items of a toolbar, with the actions that they name.
// An action without an icon shows a circle.
func toolItems(items []ui.ToolbarItem, actions []ui.Action) ([]toolItem, error) {
	byID := map[string]ui.Action{}
	for _, action := range actions {
		byID[action.ID] = action
	}
	tools := make([]toolItem, len(items))
	for i, item := range items {
		switch {
		case item.Separator:
			tools[i] = toolItem{kind: toolSeparator}
		case item.Space:
			tools[i] = toolItem{kind: toolSpace}
		case item.FlexibleSpace:
			tools[i] = toolItem{kind: toolFlexibleSpace}
		default:
			action, ok := byID[item.ActionID]
			if !ok {
				return nil, errors.New("ui: toolbar item names an unknown action " + item.ActionID)
			}
			icon := action.Icon
			if icon.IsZero() {
				icon = ui.IconSource{SystemName: "circle"}
			}
			tools[i] = actionTool(action)
			tools[i].icon = newIcon(icon, 0)
		}
	}
	return tools, nil
}

// actionTool returns the item of an action.
func actionTool(action ui.Action) toolItem {
	return toolItem{
		kind:    toolAction,
		id:      action.ID,
		label:   action.Text,
		icon:    newIcon(action.Icon, 0),
		enabled: action.Enabled,
	}
}

type toolbarExtension struct {
	baseExtension
	bar         hwnd
	items       []toolItem
	images      himagelist
	showsLabels bool
	// overflow hides the buttons that do not fit behind a button.
	overflow  bool
	receiver  eventReceiver
	hidden    int
	fullWidth int32
	// labelsBelow shows labels below the icons, as the toolbars of windows do
	// on other platforms; a command bar shows them beside the icons.
	labelsBelow bool
	// labels holds the text of the buttons, which the toolbar points at.
	labels [][]uint16
}

func (t *toolbarExtension) below() bool {
	return t.labelsBelow && t.showsLabels
}

func (t *toolbarExtension) free() {
	if t.images != 0 {
		win32ImageListDestroy(t.images)
	}
}

// metrics gives the buttons the room of the command bars of Windows: padding
// around the icon and the label, and a gap between buttons.
func (t *toolbarExtension) metrics(view *nativeView) {
	scale := view.scale()
	spacing := 4.0
	if t.labelsBelow {
		spacing = 8
	}
	metrics := tbMetrics{
		dwMask:          tbmfPad | tbmfButtonSpacing,
		cxPad:           int32(math.Round(12 * scale)),
		cyPad:           int32(math.Round(10 * scale)),
		cxButtonSpacing: int32(math.Round(spacing * scale)),
	}
	metrics.cbSize = uint32(unsafe.Sizeof(metrics))
	win32SendMessage(t.bar, tbSetMetrics, 0, uintptr(unsafe.Pointer(&metrics)))
	win32SendMessage(t.bar, tbSetPadding, 0, makeWParam(int(metrics.cxPad), int(metrics.cyPad)))
}

func (t *toolbarExtension) commandOf(index int) uintptr {
	return uintptr(100 + index)
}

// build makes the buttons of the items.
func (t *toolbarExtension) build(view *nativeView) {
	win32SendMessage(t.bar, wmSetRedraw, 0, 0)
	style := win32SendMessage(t.bar, tbGetStyle, 0, 0)
	if t.below() {
		style &^= tbstyleList
	} else {
		style |= tbstyleList
	}
	win32SendMessage(t.bar, tbSetStyle, 0, style)
	t.metrics(view)
	for win32SendMessage(t.bar, tbButtonCount, 0, 0) > 0 {
		win32SendMessage(t.bar, tbDeleteButton, 0, 0)
	}
	if t.images != 0 {
		win32SendMessage(t.bar, tbSetImageList, 0, 0)
		win32ImageListDestroy(t.images)
		t.images = 0
	}
	side := int32(math.Round(16 * view.scale()))
	for i := range t.items {
		t.items[i].image = -1
	}
	if slices.ContainsFunc(t.items, func(item toolItem) bool { return item.icon != nil }) {
		t.images = win32ImageListCreate(side, side, ilcColor32, int32(len(t.items)), 1)
		for i, item := range t.items {
			if item.icon != nil {
				bitmap := iconBitmap(item.icon, int(side), view.palette().text)
				t.items[i].image = win32ImageListAdd(t.images, bitmap, 0)
				win32DeleteObject(hgdiobj(bitmap))
			}
		}
		win32SendMessage(t.bar, tbSetImageList, 0, uintptr(t.images))
	} else {
		win32SendMessage(t.bar, tbSetBitmapSize, 0, makeWParam(0, int(side)))
	}
	t.labels = nil
	var buttons []tbButton
	for i, item := range t.items {
		button := tbButton{idCommand: int32(t.commandOf(i))}
		if item.kind == toolAction {
			button.iBitmap = iImageNone
			if item.image >= 0 {
				button.iBitmap = item.image
			}
			if item.enabled {
				button.fsState = tbstateEnabled
			}
			button.fsStyle = btnsButton | btnsAutoSize
			if (t.showsLabels || item.image < 0) && !t.below() {
				button.fsStyle |= btnsShowText
			}
			label := wideSlice(item.label)
			t.labels = append(t.labels, label)
			button.iString = uintptr(unsafe.Pointer(&label[0]))
		} else {
			// A space is a separator whose line the toolbar paints over; see
			// message.
			switch item.kind {
			case toolSeparator:
				button.iBitmap = 6
			case toolSpace:
				button.iBitmap = int32(math.Round(12 * view.scale()))
			}
			button.fsStyle = btnsSep
		}
		buttons = append(buttons, button)
	}
	if len(buttons) > 0 {
		win32SendMessage(t.bar, tbAddButtons, uintptr(len(buttons)), uintptr(unsafe.Pointer(&buttons[0])))
	}
	win32SendMessage(t.bar, tbAutoSize, 0, 0)
	// The full width, with every button shown, is the preferred width; fit
	// later hides the buttons that do not fit.
	var size sizeInfo
	win32SendMessage(t.bar, tbGetMaxSize, 0, uintptr(unsafe.Pointer(&size)))
	t.fullWidth = size.cx
	win32SendMessage(t.bar, wmSetRedraw, 1, 0)
	win32InvalidateRect(t.bar, nil, true)
}

func (t *toolbarExtension) buttonHeight() int {
	return int(hiword(win32SendMessage(t.bar, tbGetButtonSize, 0, 0)))
}

func (t *toolbarExtension) itemRect(index int) rect {
	var area rect
	win32SendMessage(t.bar, tbGetItemRect, uintptr(index), uintptr(unsafe.Pointer(&area)))
	return area
}

func (t *toolbarExtension) setWidth(index int, width int) {
	info := tbButtonInfo{dwMask: tbifSize, cx: uint16(width)}
	info.cbSize = uint32(unsafe.Sizeof(info))
	win32SendMessage(t.bar, tbSetButtonInfo, t.commandOf(index), uintptr(unsafe.Pointer(&info)))
}

// fit sets the width of the flexible spaces and hides the buttons that do not
// fit, in logical units.
func (t *toolbarExtension) fit(view *nativeView) {
	scale := view.scale()
	var client rect
	win32GetClientRect(t.bar, &client)
	if client.right <= 0 {
		return
	}
	// The buttons take their width and the spacing between them; the flexible
	// spaces share what is left.
	flexible := 0
	for i, item := range t.items {
		win32SendMessage(t.bar, tbHideButton, t.commandOf(i), 0)
		if item.kind == toolFlexibleSpace {
			flexible++
			t.setWidth(i, 0)
		}
	}
	var fixed int32
	if len(t.items) > 0 {
		fixed = t.itemRect(len(t.items) - 1).right
	}
	// The overflow button takes room only when the buttons do not fit.
	var reserve int32
	if t.overflow && fixed > client.right {
		reserve = int32(math.Round(28 * scale))
	}
	extra := max(0, client.right-reserve-fixed)
	spread := func(room int32) {
		for i, item := range t.items {
			if item.kind == toolFlexibleSpace {
				t.setWidth(i, int(max(0, room))/flexible)
			}
		}
	}
	if flexible > 0 {
		spread(extra)
		// A separator can take more room than its width; the spaces take the
		// difference between where the last button ends and the end of the
		// bar.
		for range 2 {
			difference := client.right - reserve - t.itemRect(len(t.items)-1).right
			if difference == 0 {
				break
			}
			extra = max(0, extra+difference)
			spread(extra)
		}
	}
	t.hidden = 0
	if !t.overflow {
		return
	}
	limit := client.right - reserve
	for i := range t.items {
		if t.itemRect(i).right > limit {
			win32SendMessage(t.bar, tbHideButton, t.commandOf(i), 1)
			t.hidden++
		}
	}
}

func (t *toolbarExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if h == t.bar && message == wmPaint {
		// A space or a flexible space is blank: the line that the toolbar draws
		// for a separator is painted over.
		*result = win32DefSubclassProc(h, message, wparam, lparam)
		dc := win32GetDC(t.bar)
		brush := win32CreateSolidBrush(backgroundOf(t.bar).colorRef())
		for i, item := range t.items {
			if item.kind == toolSpace || item.kind == toolFlexibleSpace {
				var area rect
				if win32SendMessage(t.bar, tbGetItemRect, uintptr(i), uintptr(unsafe.Pointer(&area))) != 0 {
					win32FillRect(dc, &area, brush)
				}
			}
		}
		win32DeleteObject(hgdiobj(brush))
		win32ReleaseDC(t.bar, dc)
		return true
	}
	if message == wmCommand && hwnd(lparam) == t.bar {
		if index := int(loword(wparam)) - 100; index >= 0 && index < len(t.items) {
			send(t.receiver, actionEvent{id: t.items[index].id})
		}
		*result = 0
		return true
	}
	if message == wmSize && h == view.outer {
		t.layout(view)
	}
	return false
}

func (t *toolbarExtension) layout(view *nativeView) {
	var client rect
	win32GetClientRect(view.outer, &client)
	height := int32(t.buttonHeight())
	scale := view.scale()
	// A window toolbar keeps a margin from the edges of the window.
	var inset int32
	if t.labelsBelow {
		inset = int32(math.Round(4 * scale))
	}
	win32SetWindowPos(t.bar, 0, inset, max(0, (client.bottom-height)/2), client.right-2*inset, height,
		swpNoZOrder|swpNoActivate)
	t.fit(view)
	if more := hwnd(win32GetProp(view.outer, "GKMore")); more != 0 {
		side := min(height, int32(math.Round(28*scale)))
		win32SetWindowPos(more, hwndTop, client.right-int32(math.Round(26*scale)), (client.bottom-side)/2,
			int32(math.Round(24*scale)), side, swpNoActivate)
		if t.hidden > 0 {
			win32ShowWindow(more, swShowNA)
		} else {
			win32ShowWindow(more, swHide)
		}
	}
}

func (t *toolbarExtension) themeChanged(view *nativeView) {
	applyThemeTo(t.bar, view.theme(), "")
	win32SendMessage(t.bar, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(t.bar))), 1)
	// The glyphs of the icons take the text color of the theme.
	if len(t.items) > 0 {
		t.build(view)
		t.layout(view)
	}
}

func (t *toolbarExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	scale := view.scale()
	width := float64(t.fullWidth) / scale
	if t.overflow {
		width += 28
	}
	return d2d1SizeF{width: float32(width), height: float32(float64(t.buttonHeight())/scale + 4)}, true
}

// moreSubclass shows the actions that do not fit in a menu. data is the id of
// the view of the toolbar.
func moreSubclass() uintptr {
	return callback("moreSubclass", func(h hwnd, message uint32, wparam, lparam, id, data uintptr) uintptr {
		if message == wmLButtonUp {
			if view := views[data]; view != nil {
				if toolbar, ok := view.ext.(*toolbarExtension); ok {
					toolbar.showMore(h)
				}
			}
		}
		if message == wmNCDestroy {
			win32RemoveWindowSubclass(h, moreSubclass(), 4)
		}
		return win32DefSubclassProc(h, message, wparam, lparam)
	})
}

func (t *toolbarExtension) showMore(button hwnd) {
	popup := win32CreatePopupMenu()
	for i, item := range t.items {
		info := tbButtonInfo{dwMask: tbifState}
		info.cbSize = uint32(unsafe.Sizeof(info))
		win32SendMessage(t.bar, tbGetButtonInfo, t.commandOf(i), uintptr(unsafe.Pointer(&info)))
		if item.kind == toolAction && info.fsState&tbstateHidden != 0 {
			flags := uint32(mfString)
			if !item.enabled {
				flags |= mfGrayed
			}
			win32AppendMenu(popup, flags, uintptr(i+1), item.label)
		}
	}
	var bounds rect
	win32GetWindowRect(button, &bounds)
	command := int(win32TrackPopupMenuEx(popup, tpmReturnCmd|tpmRightAlign, bounds.right, bounds.bottom,
		win32GetAncestor(button, gaRoot), nil))
	win32DestroyMenu(popup)
	if command >= 1 && command <= len(t.items) {
		send(t.receiver, actionEvent{id: t.items[command-1].id})
	}
}

// toolbarView makes the view of a toolbar control.
func toolbarView(window *nativeWindow, kind int, receiver eventReceiver, overflow bool) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kind, window)
	toolbar := &toolbarExtension{receiver: receiver, overflow: overflow, showsLabels: true}
	view.ext = toolbar
	toolbar.bar = makeNative(toolbarClassName, "",
		tbstyleFlat|tbstyleList|tbstyleTooltips|ccsNoDivider|ccsNoResize|ccsNoParentAlign|wsTabStop, 0, host)
	win32SendMessage(toolbar.bar, tbButtonStructSize, unsafe.Sizeof(tbButton{}), 0)
	win32SendMessage(toolbar.bar, tbSetExtendedStyle, 0, tbstyleExMixedButtons|tbstyleExDoubleBuffer)
	setViewOf(toolbar.bar, view)
	subclass(toolbar.bar)
	view.control = toolbar.bar
	if overflow {
		more := makeNative(wcButton, "", bsPushButton, 0, host)
		symbol := fontSpec{family: symbolFamily(), size: 12, weight: 400}
		win32SendMessage(more, wmSetFont, uintptr(fontHandle(symbol, win32GetDpiForWindow(more))), 0)
		win32SetProp(host, "GKMore", uintptr(more))
		win32SetWindowSubclass(more, moreSubclass(), 4, view.id)
		win32ShowWindow(more, swHide)
	}
	toolbar.themeChanged(view)
	return view
}

// ---- Toolbar ----

type nativeToolbar struct {
	win  *nativeWindow
	view *nativeView
}

func (t *nativeToolbar) destroy() {
	t.view.ext.(*toolbarExtension).receiver = nil
	if windowLive(t.win) {
		t.win.removeChrome(t.view)
	}
	t.view.destroy()
}

// newToolbar creates the toolbar of a window.
func newToolbar(receiver eventReceiver, win *nativeWindow, items []toolItem, showsLabels bool) *nativeToolbar {
	if win == nil {
		return nil
	}
	object := &nativeToolbar{win: win}
	object.view = toolbarView(win, kindToolbar, receiver, true)
	toolbar := object.view.ext.(*toolbarExtension)
	toolbar.showsLabels = showsLabels
	toolbar.labelsBelow = true
	toolbar.items = items
	toolbar.build(object.view)
	object.view.accessibleName = "Toolbar"
	win.addChrome(object.view, true, int(math.Round(float64(toolbar.buttonHeight())/win.scale()))+6)
	return object
}

func (object *nativeToolbar) setEnabled(identifier string, enabled bool) {
	if object == nil {
		return
	}
	toolbar := object.view.ext.(*toolbarExtension)
	for i := range toolbar.items {
		if toolbar.items[i].id == identifier {
			toolbar.items[i].enabled = enabled
			win32SendMessage(toolbar.bar, tbEnableButton, toolbar.commandOf(i), boolArg(enabled))
		}
	}
}

// ---- Status bar ----

type statusBarExtension struct {
	baseExtension
}

func (statusBarExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	size := measureGDI(view.text, view.fontOr(bodyFont()), 0, 0)
	return d2d1SizeF{width: float32(size.cx + 16), height: float32(size.cy + 8)}, true
}

func (win *nativeWindow) newStatusBar(text string) *nativeView {
	bar := makeNative(statusClassName, "", ccsNoParentAlign|ccsNoResize|ccsNoDivider|sbarsTooltips, 0, 0)
	view := adoptView(win, bar, 0, kindStatusBar)
	view.ext = statusBarExtension{}
	view.text = text
	win32SendMessage(bar, sbSetText, sbtNoBorders, uintptr(unsafe.Pointer(wide(text))))
	return view
}

func (view *nativeView) setStatus(text string) {
	if view.live() && view.kind == kindStatusBar {
		view.text = text
		win32SendMessage(view.outer, sbSetText, sbtNoBorders, uintptr(unsafe.Pointer(wide(text))))
	}
}

// ---- Command bar ----

func (win *nativeWindow) newCommandBar() *nativeView {
	return toolbarView(win, kindCommandBar, nil, true)
}

func (view *nativeView) setCommandBarActions(actions []ui.Action, compact bool) {
	if !view.live() || view.kind != kindCommandBar {
		return
	}
	toolbar := view.ext.(*toolbarExtension)
	toolbar.receiver = view.receiver
	toolbar.showsLabels = !compact
	toolbar.items = make([]toolItem, len(actions))
	for i, action := range actions {
		toolbar.items[i] = actionTool(action)
	}
	toolbar.build(view)
	toolbar.layout(view)
}

func toolbarItems(view *nativeView) []accessibleItem {
	var items []accessibleItem
	if view != nil && (view.kind == kindCommandBar || view.kind == kindToolbar) {
		for _, item := range view.ext.(*toolbarExtension).items {
			if item.kind == toolAction {
				items = append(items, accessibleItem{"Button", item.label})
			}
		}
	}
	return items
}
