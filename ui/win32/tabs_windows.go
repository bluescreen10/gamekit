package win32

import (
	"math"
	"unsafe"
)

// ---- Tabs ----
//
// Tabs at the top or the bottom are the tab control of Windows. The tab control
// shows its tabs only at the top or the bottom with the visual styles of
// Windows, so tabs at the left or the right are a list beside the pages. A page
// is the content view of a tab in the content area, which shows the page of the
// selected tab.

type tabsExtension struct {
	baseExtension
	placement int
	overflow  int
	selected  int
	titles    []string
	pages     []*nativeView
	// strip is the tab control or the list.
	strip hwnd
	// content is the layout that holds the pages.
	content hwnd
	// menu is the button that lists every tab.
	menu hwnd
}

func (t *tabsExtension) vertical() bool {
	return t.placement == 2 || t.placement == 3
}

// stripWidth is the width of a list of tabs at the side.
func (t *tabsExtension) stripWidth(view *nativeView) float64 {
	font := view.fontOr(bodyFont())
	widest := 80
	for _, title := range t.titles {
		widest = max(widest, measureGDI(title, font, 0, 0).cx+24)
	}
	return float64(min(widest, 240))
}

// contentFrame returns the frame of the pages in logical units.
func (t *tabsExtension) contentFrame(view *nativeView) (x, y, width, height float64) {
	if t.vertical() {
		side := t.stripWidth(view)
		if t.placement == 2 {
			x = side
		}
		return x, 0, max(0, float64(view.width)-side), float64(view.height)
	}
	scale := view.scale()
	area := rect{right: int32(math.Round(float64(view.width) * scale)), bottom: int32(math.Round(float64(view.height) * scale))}
	win32SendMessage(t.strip, tcmAdjustRect, 0, uintptr(unsafe.Pointer(&area)))
	return float64(area.left) / scale, float64(area.top) / scale, max(0, float64(area.right-area.left)/scale),
		max(0, float64(area.bottom-area.top)/scale)
}

func (t *tabsExtension) layout(view *nativeView) {
	menuSize := 0.0
	if t.menu != 0 {
		menuSize = 24
	}
	if t.vertical() {
		side := t.stripWidth(view)
		x := float64(view.width) - side
		if t.placement == 2 {
			x = 0
		}
		if t.menu != 0 {
			placeWindow(t.menu, x, 0, side, menuSize)
		}
		placeWindow(t.strip, x, menuSize, side, max(0, float64(view.height)-menuSize))
	} else {
		placeWindow(t.strip, 0, 0, float64(view.width), float64(view.height))
		if t.menu != 0 {
			var item rect
			height := menuSize
			if win32SendMessage(t.strip, tcmGetItemRect, 0, uintptr(unsafe.Pointer(&item))) != 0 {
				height = float64(item.bottom-item.top) / view.scale()
			}
			y := 2.0
			if t.placement == 1 {
				y = float64(view.height) - height - 2
			}
			placeWindow(t.menu, float64(view.width)-menuSize-2, y, menuSize, height-2)
			win32SetWindowPos(t.menu, hwndTop, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		}
	}
	x, y, width, height := t.contentFrame(view)
	placeWindow(t.content, x, y, width, height)
	win32SetWindowPos(t.content, hwndTop, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
}

func (t *tabsExtension) showPage() {
	for i, page := range t.pages {
		if !page.live() {
			continue
		}
		if i == t.selected {
			win32ShowWindow(page.outer, swShowNA)
		} else {
			win32ShowWindow(page.outer, swHide)
		}
	}
}

func (t *tabsExtension) selectTab(view *nativeView, index int, report bool) {
	if index == t.selected {
		return
	}
	t.selected = index
	view.suppress++
	if t.vertical() {
		win32SendMessage(t.strip, lbSetCurSel, uintptr(index), 0)
	} else {
		win32SendMessage(t.strip, tcmSetCurSel, uintptr(index), 0)
	}
	view.suppress--
	t.showPage()
	if report {
		view.report(changeEvent{})
	}
}

func (t *tabsExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.hwndFrom == t.strip && header.code == tcnSelChange {
		t.selectTab(view, int(int32(win32SendMessage(t.strip, tcmGetCurSel, 0, 0))), view.suppress == 0)
		*result = 0
		return true
	}
	return false
}

func (t *tabsExtension) command(view *nativeView, from hwnd, code int) bool {
	if from == t.strip && code == lbnSelChange {
		t.selectTab(view, int(int32(win32SendMessage(t.strip, lbGetCurSel, 0, 0))), view.suppress == 0)
		return true
	}
	if from == t.menu && code == bnClicked {
		popup := win32CreatePopupMenu()
		for i, title := range t.titles {
			flags := uint32(mfString)
			if i == t.selected {
				flags |= mfChecked
			}
			win32AppendMenu(popup, flags, uintptr(i+1), title)
		}
		var bounds rect
		win32GetWindowRect(t.menu, &bounds)
		command := int(win32TrackPopupMenuEx(popup, tpmReturnCmd|tpmRightAlign, bounds.right, bounds.bottom,
			win32GetAncestor(t.menu, gaRoot), nil))
		win32DestroyMenu(popup)
		if command >= 1 && command <= len(t.titles) {
			t.selectTab(view, command-1, true)
		}
		return true
	}
	return false
}

func (t *tabsExtension) themeChanged(view *nativeView) {
	font := fontHandle(view.fontOr(bodyFont()), win32GetDpiForWindow(t.strip))
	win32SendMessage(t.strip, wmSetFont, uintptr(font), 1)
	applyThemeTo(t.strip, view.theme(), "")
}

func tabsOf(view *nativeView) *tabsExtension {
	if view == nil || view.kind != kindTabs {
		return nil
	}
	tabs, _ := view.ext.(*tabsExtension)
	return tabs
}

// buildTabs makes the strip of the tabs for their placement and overflow
// policy.
func buildTabs(view *nativeView) {
	tabs := tabsOf(view)
	if tabs.strip != 0 {
		win32DestroyWindow(tabs.strip)
	}
	if tabs.menu != 0 {
		win32DestroyWindow(tabs.menu)
		tabs.menu = 0
	}
	if tabs.vertical() {
		tabs.strip = makeNative(wcListBox, "", lbsNotify|lbsNoIntegralHeight|wsVScroll|wsTabStop, wsExClientEdge,
			view.outer)
		for _, title := range tabs.titles {
			win32SendMessage(tabs.strip, lbAddString, 0, uintptr(unsafe.Pointer(wide(title))))
		}
		win32SendMessage(tabs.strip, lbSetCurSel, uintptr(tabs.selected), 0)
	} else {
		style := uint32(wsTabStop | wsClipSiblings)
		if tabs.placement == 1 {
			style |= tcsBottom
		}
		tabs.strip = makeNative(wcTabControl, "", style, 0, view.outer)
		for i, title := range tabs.titles {
			insertTab(tabs.strip, i, title)
		}
		win32SendMessage(tabs.strip, tcmSetCurSel, uintptr(tabs.selected), 0)
	}
	setViewOf(tabs.strip, view)
	subclass(tabs.strip)
	if tabs.overflow == 0 {
		tabs.menu = makeNative(wcButton, "", bsPushButton, 0, view.outer)
		setViewOf(tabs.menu, view)
		symbol := fontSpec{family: symbolFamily(), size: 10, weight: 400}
		win32SendMessage(tabs.menu, wmSetFont, uintptr(fontHandle(symbol, win32GetDpiForWindow(tabs.menu))), 0)
		applyThemeTo(tabs.menu, view.theme(), "")
	}
	view.control = tabs.strip
	tabs.themeChanged(view)
	win32SetWindowPos(tabs.strip, hwndBottom, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	tabs.layout(view)
}

// insertTab inserts a tab in a tab control.
func insertTab(strip hwnd, index int, title string) {
	item := tcItem{mask: tcifText, pszText: wide(title)}
	win32SendMessage(strip, tcmInsertItem, uintptr(index), uintptr(unsafe.Pointer(&item)))
}

func (win *nativeWindow) newTabs() *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindTabs, win)
	tabs := &tabsExtension{overflow: 1, selected: -1}
	view.ext = tabs
	tabs.content = newLayout(host)
	view.body = tabs.content
	buildTabs(view)
	return view
}

func (view *nativeView) addTab(title string, page *nativeView) bool {
	tabs := tabsOf(view)
	if tabs == nil || !page.live() {
		return false
	}
	tabs.titles = append(tabs.titles, title)
	tabs.pages = append(tabs.pages, page)
	reparent(page.outer, tabs.content)
	view.suppress++
	if tabs.vertical() {
		win32SendMessage(tabs.strip, lbAddString, 0, uintptr(unsafe.Pointer(wide(title))))
	} else {
		insertTab(tabs.strip, len(tabs.titles)-1, title)
	}
	view.suppress--
	if tabs.selected < 0 {
		tabs.selectTab(view, 0, false)
	}
	tabs.showPage()
	tabs.layout(view)
	return true
}

func (view *nativeView) selectedTab() int {
	if tabs := tabsOf(view); tabs != nil {
		return tabs.selected
	}
	return -1
}

func (view *nativeView) selectTab(index int) {
	if tabs := tabsOf(view); tabs != nil && index >= 0 && index < len(tabs.titles) {
		tabs.selectTab(view, index, false)
	}
}

func (view *nativeView) tabContentSize() (int, int) {
	tabs := tabsOf(view)
	if tabs == nil {
		return 0, 0
	}
	_, _, w, h := tabs.contentFrame(view)
	return int(math.Floor(w)), int(math.Floor(h))
}

func (view *nativeView) configureTabs(placement int) {
	if tabs := tabsOf(view); tabs != nil && tabs.placement != placement {
		tabs.placement = placement
		buildTabs(view)
	}
}

func (view *nativeView) setTabOverflow(policy int) {
	if tabs := tabsOf(view); tabs != nil && tabs.overflow != policy {
		tabs.overflow = policy
		buildTabs(view)
	}
}

func tabItems(view *nativeView) []accessibleItem {
	var items []accessibleItem
	if tabs := tabsOf(view); tabs != nil {
		for _, title := range tabs.titles {
			items = append(items, accessibleItem{"TabItem", title})
		}
	}
	return items
}
