package win32

import (
	"math"
	"slices"
	"unsafe"
)

func (parent *nativeView) addChild(view *nativeView) {
	if !parent.live() || !view.live() {
		return
	}
	if target := parent.container(); target != 0 {
		reparent(view.outer, target)
		if view.visible {
			win32ShowWindow(view.outer, swShowNA)
		}
	}
}

func (win *nativeWindow) newClipView() *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindClip, win)
	view.body = host
	return view
}

// ---- Group box ----
//
// A group box is the group box of Windows: a frame with its title. The content
// and the subtitle are inside the frame.

const groupInset = 9

type groupBoxExtension struct {
	baseExtension
	frame        hwnd
	subtitle     hwnd
	subtitleText string
}

func (g *groupBoxExtension) titleHeight(view *nativeView) int {
	return measureGDI("Ag", view.fontOr(bodyFont()), 0, 0).cy
}

func (g *groupBoxExtension) subtitleHeight(view *nativeView, width int) int {
	if g.subtitleText == "" {
		return 0
	}
	return measureGDI(g.subtitleText, typeRamp(1), max(width, 1), 0).cy + 6
}

func (g *groupBoxExtension) layout(view *nativeView) {
	placeWindow(g.frame, 0, 0, float64(view.width), float64(view.height))
	top := g.titleHeight(view) + 4
	width := max(float64(view.width-2*groupInset), 0)
	height := max(float64(view.height-top-groupInset), 0)
	placeWindow(view.body, groupInset, float64(top), width, height)
	sub := g.subtitleHeight(view, int(width))
	if g.subtitle != 0 {
		placeWindow(g.subtitle, 0, 0, width, float64(max(sub-6, 0)))
		if g.subtitleText == "" {
			win32ShowWindow(g.subtitle, swHide)
		} else {
			win32ShowWindow(g.subtitle, swShowNA)
		}
	}
}

// paintDark draws the frame in the dark theme, which the visual styles of
// Windows do not have for group boxes: a line in the color of control borders,
// and the title over it in the text color.
func (g *groupBoxExtension) paintDark(view *nativeView) {
	var paint paintStruct
	dc := win32BeginPaint(g.frame, &paint)
	defer win32EndPaint(g.frame, &paint)
	var client rect
	win32GetClientRect(g.frame, &client)
	pal := view.palette()
	background := win32CreateSolidBrush(backgroundOf(g.frame).colorRef())
	defer win32DeleteObject(hgdiobj(background))
	win32FillRect(dc, &client, background)
	font := hgdiobj(win32SendMessage(g.frame, wmGetFont, 0, 0))
	previous := win32SelectObject(dc, font)
	defer win32SelectObject(dc, previous)
	var metrics textMetric
	win32GetTextMetrics(dc, &metrics)
	middle := metrics.tmHeight / 2
	line := win32CreateSolidBrush(pal.controlStroke.colorRef())
	defer win32DeleteObject(hgdiobj(line))
	box := rect{left: client.left, top: client.top + middle, right: client.right, bottom: client.bottom}
	win32FrameRect(dc, &box, line)
	if view.text == "" {
		return
	}
	pad := int32(math.Round(4 * scaleOf(g.frame)))
	var size sizeInfo
	text := wideSlice(view.text)
	win32GetTextExtentPoint32(dc, &text[0], int32(len(text)-1), &size)
	title := rect{left: 2 * pad, right: min(2*pad+size.cx+2*pad, client.right), bottom: metrics.tmHeight}
	win32FillRect(dc, &title, background)
	win32SetBkMode(dc, transparent)
	win32SetTextColor(dc, pal.text.colorRef())
	area := rect{left: title.left + pad, right: title.right - pad, bottom: title.bottom}
	win32DrawText(dc, view.text, &area, dtLeft|dtTop|dtSingleLine|dtNoPrefix|dtEndEllipsis)
}

func (g *groupBoxExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	// A group box draws only its line and its title. Its parent does not paint
	// under it, so it fills its background, or a moved group box keeps what was
	// under it before.
	if h == g.frame && message == wmPaint && view.palette().dark {
		g.paintDark(view)
		*result = 0
		return true
	}
	if h == g.frame && message == wmEraseBkgnd {
		var client rect
		win32GetClientRect(g.frame, &client)
		fillRectColor(hdc(wparam), &client, backgroundOf(g.frame))
		*result = 1
		return true
	}
	return false
}

func (g *groupBoxExtension) themeChanged(view *nativeView) {
	win32SendMessage(g.frame, wmSetFont, uintptr(fontHandle(view.fontOr(bodyFont()), win32GetDpiForWindow(g.frame))), 1)
	if g.subtitle != 0 {
		win32SendMessage(g.subtitle, wmSetFont, uintptr(fontHandle(typeRamp(1), win32GetDpiForWindow(g.subtitle))), 1)
	}
}

func groupOf(view *nativeView) *groupBoxExtension {
	if view == nil || view.kind != kindGroupBox {
		return nil
	}
	box, _ := view.ext.(*groupBoxExtension)
	return box
}

func (win *nativeWindow) newGroupBox(title string, subtitle string) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindGroupBox, win)
	box := &groupBoxExtension{subtitleText: subtitle}
	view.ext = box
	view.text = title
	box.frame = makeNative(wcButton, title, bsGroupBox|wsClipSiblings, 0, host)
	view.body = newLayout(host)
	// A child win that is created first is at the top of the z-order, and
	// the body clips the siblings above it, so the frame goes below the body.
	win32SetWindowPos(box.frame, hwndBottom, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	setViewOf(box.frame, view)
	subclass(box.frame)
	box.subtitle = makeNative(wcStatic, subtitle, ssLeft|ssNoPrefix, 0, view.body)
	applyThemeTo(box.frame, view.theme(), "")
	box.themeChanged(view)
	return view
}

func (view *nativeView) setGroupBoxTitle(title string, subtitle string) {
	box := groupOf(view)
	if box == nil {
		return
	}
	view.text = title
	win32SetWindowText(box.frame, title)
	box.subtitleText = subtitle
	win32SetWindowText(box.subtitle, subtitle)
	box.layout(view)
}

func (view *nativeView) groupBoxChrome() (int, int) {
	box := groupOf(view)
	if box == nil {
		return 0, 0
	}
	return 2 * groupInset, box.titleHeight(view) + 4 + groupInset
}

func (view *nativeView) groupBoxSubtitleHeight(width int) int {
	if box := groupOf(view); box != nil {
		return box.subtitleHeight(view, width)
	}
	return 0
}

func (view *nativeView) groupBoxBody() (width, height, top int) {
	if groupOf(view) == nil {
		return 0, 0, 0
	}
	chromeWidth, chromeHeight := view.groupBoxChrome()
	width = max(0, view.width-chromeWidth)
	height = max(0, view.height-chromeHeight)
	return width, height, view.groupBoxSubtitleHeight(width)
}

// ---- Split view ----
//
// The panes of a split view are layouts with a divider between each pair, which
// the user drags. Windows has no split view control, so the backend draws the
// dividers.

const splitGrip = 6

type splitExtension struct {
	baseExtension
	vertical    bool
	count       int
	panes       []hwnd
	dividers    []*nativeView
	positions   []float64
	minimums    []float64
	collapsible []bool
	records     []*nativeView
}

func (s *splitExtension) free() {
	for _, record := range s.records {
		if record != nil {
			record.outer = 0
			record.destroy()
		}
	}
	for _, divider := range s.dividers {
		divider.destroy()
	}
}

func (s *splitExtension) length(view *nativeView) float64 {
	if s.vertical {
		return float64(view.width)
	}
	return float64(view.height)
}

func (s *splitExtension) cross(view *nativeView) float64 {
	if s.vertical {
		return float64(view.height)
	}
	return float64(view.width)
}

func (s *splitExtension) defaults(view *nativeView) {
	total := s.length(view)
	for i := 0; i < s.count-1; i++ {
		if s.positions[i] < 0 {
			s.positions[i] = (total-float64(s.count-1))*float64(i+1)/float64(s.count) + float64(i)
		}
	}
}

// paneSpan returns the start and the length of a pane.
func (s *splitExtension) paneSpan(view *nativeView, i int) (float64, float64) {
	start := 0.0
	if i > 0 {
		start = s.positions[i-1] + 1
	}
	end := s.length(view)
	if i < s.count-1 {
		end = s.positions[i]
	}
	return start, max(math.Floor(end-start), 0)
}

func (s *splitExtension) arrange(view *nativeView) {
	s.defaults(view)
	across := s.cross(view)
	for i := range s.count {
		start, size := s.paneSpan(view, i)
		if s.vertical {
			placeWindow(s.panes[i], start, 0, size, across)
		} else {
			placeWindow(s.panes[i], 0, start, across, size)
		}
		if i < s.count-1 {
			grip := s.positions[i] + 0.5 - splitGrip/2
			if s.vertical {
				placeWindow(s.dividers[i].outer, grip, 0, splitGrip, across)
			} else {
				placeWindow(s.dividers[i].outer, 0, grip, across, splitGrip)
			}
		}
	}
}

func (s *splitExtension) clamp(view *nativeView, index int, position float64) {
	lower := 0.0
	if index > 0 {
		lower = s.positions[index-1] + 1
	}
	if !s.collapsible[index] {
		lower += s.minimums[index]
	}
	upper := s.length(view)
	if index+1 < s.count-1 {
		upper = s.positions[index+1]
	}
	upper--
	if !s.collapsible[index+1] {
		upper -= s.minimums[index+1]
	}
	s.positions[index] = max(lower, min(position, upper))
}

func (s *splitExtension) layout(view *nativeView) {
	s.arrange(view)
}

func splitOf(view *nativeView) *splitExtension {
	if view == nil || view.kind != kindSplit {
		return nil
	}
	split, _ := view.ext.(*splitExtension)
	return split
}

type dividerExtension struct {
	baseExtension
	split    *nativeView
	index    int
	dragging bool
	offset   float64
}

func (d *dividerExtension) paint(view *nativeView, canvas *painter) {
	data := splitOf(d.split)
	if data == nil {
		return
	}
	pal := d.split.palette()
	color := pal.divider
	if view.hover || d.dragging {
		color = pal.accent
	}
	if data.vertical {
		canvas.fillRect(float32(math.Floor(float64(canvas.width/2))), 0, 1, canvas.height, color)
	} else {
		canvas.fillRect(0, float32(math.Floor(float64(canvas.height/2))), canvas.width, 1, color)
	}
}

// along returns the position of the pointer of a mouse message along the split
// view, in logical units.
func (d *dividerExtension) along(view *nativeView, data *splitExtension, lparam uintptr) float64 {
	at := point{x: int32(signedLoword(lparam)), y: int32(signedHiword(lparam))}
	win32MapWindowPoints(view.outer, d.split.outer, &at, 1)
	scale := d.split.scale()
	if data.vertical {
		return float64(at.x) / scale
	}
	return float64(at.y) / scale
}

func (d *dividerExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	data := splitOf(d.split)
	if data == nil {
		return false
	}
	switch message {
	case wmSetCursor:
		cursor := uintptr(idcSizeNS)
		if data.vertical {
			cursor = idcSizeWE
		}
		win32SetCursor(win32LoadCursor(0, cursor))
		*result = 1
		return true
	case wmLButtonDown:
		d.offset = d.along(view, data, lparam) - data.positions[d.index]
		d.dragging = true
		win32SetCapture(view.outer)
		*result = 0
		return true
	case wmMouseMove:
		if d.dragging {
			data.clamp(d.split, d.index, d.along(view, data, lparam)-d.offset)
			data.arrange(d.split)
			send(d.split.receiver, layoutChangeEvent{})
		}
	case wmLButtonUp, wmCaptureChanged:
		if d.dragging {
			d.dragging = false
			if message == wmLButtonUp {
				win32ReleaseCapture()
			}
			win32InvalidateRect(view.outer, nil, false)
		}
	}
	return false
}

func (win *nativeWindow) newSplitView(vertical bool, panes int) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindSplit, win)
	split := &splitExtension{vertical: vertical, count: max(panes, 1)}
	view.ext = split
	for range split.count {
		split.panes = append(split.panes, newLayout(host))
		split.positions = append(split.positions, -1)
		split.minimums = append(split.minimums, 0)
		split.collapsible = append(split.collapsible, false)
		split.records = append(split.records, nil)
	}
	for i := 0; i < split.count-1; i++ {
		record := createDrawn(nil, kindGeneric, &dividerExtension{split: view, index: i}, false)
		record.owner = view
		reparent(record.outer, host)
		split.dividers = append(split.dividers, record)
	}
	return view
}

func (view *nativeView) paneView(index int) *nativeView {
	split := splitOf(view)
	if split == nil || index < 0 || index >= split.count {
		return nil
	}
	if split.records[index] == nil {
		record := newView(0, 0, kindClip, view.window)
		record.outer = split.panes[index]
		record.body = split.panes[index]
		split.records[index] = record
	}
	return split.records[index]
}

func (view *nativeView) configurePane(index int, minimum float64, collapsible bool) {
	split := splitOf(view)
	if split == nil || index < 0 || index >= split.count {
		return
	}
	split.minimums[index] = max(minimum, 0)
	split.collapsible[index] = collapsible
}

func (view *nativeView) paneFrame(index int) (x, y, width, height int) {
	split := splitOf(view)
	if split == nil || index < 0 || index >= split.count {
		return 0, 0, 0, 0
	}
	split.defaults(view)
	start, length := split.paneSpan(view, index)
	cross := int(split.cross(view))
	if split.vertical {
		return int(start), 0, int(length), cross
	}
	return 0, int(start), cross, int(length)
}

func (view *nativeView) dividerPosition(index int) float64 {
	split := splitOf(view)
	if split == nil || index < 0 || index >= split.count-1 {
		return 0
	}
	split.defaults(view)
	return split.positions[index]
}

func (view *nativeView) setDividerPosition(index int, position float64) {
	split := splitOf(view)
	if split == nil || index < 0 || index >= split.count-1 {
		return
	}
	split.defaults(view)
	split.clamp(view, index, position)
	split.arrange(view)
}

// ---- Popover ----
//
// A popover is a pop-up window with a shadow next to its anchor. A transient
// popover closes when another window becomes active, and its owner keeps the
// look of the active window while it shows.

type nativePopover struct {
	receiver  eventReceiver
	popup     hwnd
	owner     hwnd
	record    *nativeView
	transient bool
	shown     bool
}

const popoverClass = "GameKitPopover"

var (
	popovers           []*nativePopover
	popoversByHWND     = map[hwnd]*nativePopover{}
	popoverClassLoaded bool
)

func (p *nativePopover) close(report bool) {
	if !p.shown {
		return
	}
	p.shown = false
	win32ShowWindow(p.popup, swHide)
	if p.owner != 0 && win32IsWindow(p.owner) && win32GetForegroundWindow() == 0 {
		win32SetForegroundWindow(p.owner)
	}
	if report {
		send(p.receiver, closeEvent{})
	}
}

func (p *nativePopover) destroy() {
	p.receiver = nil
	popovers = slices.DeleteFunc(popovers, func(other *nativePopover) bool { return other == p })
	if p.record != nil {
		p.record.outer = 0
		p.record.destroy()
	}
	if p.popup != 0 && win32IsWindow(p.popup) {
		delete(popoversByHWND, p.popup)
		win32DestroyWindow(p.popup)
	}
}

func popoverProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	popover := popoversByHWND[h]
	var result uintptr
	if forwardNotification(message, wparam, lparam, &result) {
		return result
	}
	switch message {
	case wmActivate:
		if popover != nil && loword(wparam) == waInactive && popover.transient {
			next := hwnd(lparam)
			// A dialog that the popover opens keeps it open.
			if next == 0 || win32GetWindow(next, gwOwner) != h {
				later(func() {
					if slices.Contains(popovers, popover) && popover.popup == h && win32GetActiveWindow() != h {
						popover.close(true)
					}
				})
			}
		}
	case wmKeyDown:
		if popover != nil && wparam == vkEscape {
			popover.close(true)
			return 0
		}
	case wmClose:
		if popover != nil {
			popover.close(true)
		}
		return 0
	case wmEraseBkgnd:
		var client rect
		win32GetClientRect(h, &client)
		fillRectColor(hdc(wparam), &client, paletteFor(resolvedTheme()).flyout)
		return 1
	}
	return win32DefWindowProc(h, message, wparam, lparam)
}

// popoverOwns reports whether a window owns a popover that shows.
func popoverOwns(owner hwnd) bool {
	for _, popover := range popovers {
		if popover.shown && popover.owner == owner {
			return true
		}
	}
	return false
}

func newPopover(receiver eventReceiver, transient bool) *nativePopover {
	if !popoverClassLoaded {
		popoverClassLoaded = true
		class := wndClassEx{
			style:       csDropShadow,
			lpfnWndProc: callback("popover", popoverProc),
			hInstance:   instance(),
			hCursor:     win32LoadCursor(0, idcArrow),
		}
		registerClass(&class, popoverClass)
	}
	popover := &nativePopover{receiver: receiver, transient: transient}
	cookie := activate()
	popover.popup = win32CreateWindowEx(wsExToolWindow|wsExControlParent, popoverClass, "",
		wsPopup|wsBorder|wsClipChildren, 0, 0, 1, 1, 0, 0, instance(), nil)
	deactivate(cookie)
	popoversByHWND[popover.popup] = popover
	body := newLayout(popover.popup)
	flyout := paletteFor(resolvedTheme()).flyout
	setLayoutBackground(body, &flyout)
	popover.record = newView(0, 0, kindClip, nil)
	popover.record.outer = body
	popover.record.body = body
	popovers = append(popovers, popover)
	return popover
}

func (popover *nativePopover) body() *nativeView {
	if popover != nil {
		return popover.record
	}
	return nil
}

func (popover *nativePopover) show(anchor *nativeView, edge int, width int, height int) bool {
	var win *nativeWindow
	if anchor.live() {
		win = windowOf(anchor.outer)
	}
	if popover == nil || !anchor.live() || win == nil || !win32IsWindowVisible(win.hwnd) ||
		!win32IsWindowVisible(anchor.outer) {
		return false
	}
	popover.owner = win.hwnd
	popover.record.window = win
	win32SetWindowLongPtr(popover.popup, gwlpHwndParent, uintptr(win.hwnd))
	width = max(width, 1)
	height = max(height, 1)
	scale := scaleOf(win.hwnd)
	flyout := paletteFor(win.themeOf()).flyout
	setLayoutBackground(popover.record.outer, &flyout)
	var anchorRect rect
	win32GetWindowRect(anchor.outer, &anchorRect)
	frame := rect{right: int32(math.Round(float64(width) * scale)), bottom: int32(math.Round(float64(height) * scale))}
	win32AdjustWindowRectExForDpi(&frame, wsPopup|wsBorder, false, wsExToolWindow, win32GetDpiForWindow(win.hwnd))
	frameWidth, frameHeight := frame.right-frame.left, frame.bottom-frame.top
	gap := int32(math.Round(4 * scale))
	anchorWidth, anchorHeight := anchorRect.right-anchorRect.left, anchorRect.bottom-anchorRect.top
	x, y := anchorRect.left+(anchorWidth-frameWidth)/2, anchorRect.bottom+gap
	switch edge {
	case 0:
		y = anchorRect.top - frameHeight - gap
	case 2:
		x = anchorRect.left - frameWidth - gap
		y = anchorRect.top + (anchorHeight-frameHeight)/2
	case 3:
		x = anchorRect.right + gap
		y = anchorRect.top + (anchorHeight-frameHeight)/2
	}
	// The popover stays on the screen of its anchor.
	monitor := monitorInfo{}
	monitor.cbSize = uint32(unsafe.Sizeof(monitor))
	win32GetMonitorInfo(win32MonitorFromWindow(win.hwnd, monitorDefaultToNearest), unsafe.Pointer(&monitor))
	x = max(monitor.rcWork.left, min(x, monitor.rcWork.right-frameWidth))
	y = max(monitor.rcWork.top, min(y, monitor.rcWork.bottom-frameHeight))
	win32SetWindowPos(popover.popup, hwndTop, x, y, frameWidth, frameHeight, swpNoActivate)
	placeWindow(popover.record.outer, 0, 0, float64(width), float64(height))
	popover.shown = true
	win32ShowWindow(popover.popup, swShow)
	win32SetForegroundWindow(popover.popup)
	return true
}

func (popover *nativePopover) hide() {
	if popover != nil {
		popover.close(true)
	}
}

// ---- Title bar accessories ----
//
// A window of Windows has no room for controls in its title bar, so accessories
// share a bar at the top of the content, below the title bar.

type accessoryItem struct {
	view          *nativeView
	alignment     int
	width, height int
}

type accessoryBarExtension struct {
	baseExtension
	items []accessoryItem
}

func (a *accessoryBarExtension) layout(view *nativeView) {
	left, right := 8.0, float64(view.width)-8
	total := 0.0
	for _, item := range a.items {
		if item.alignment == 2 {
			total += float64(item.width) + 6
		}
	}
	center := (float64(view.width) - total) / 2
	for _, item := range a.items {
		w, h := float64(item.width), float64(item.height)
		y := (float64(view.height) - h) / 2
		var x float64
		switch item.alignment {
		case 1:
			x = left
			left += w + 6
		case 3:
			right -= w
			x = right
			right -= 6
		default:
			x = center
			center += w + 6
		}
		placeWindow(item.view.outer, x, y, w, h)
	}
}

type nativeAccessory struct {
	receiver eventReceiver
	win      *nativeWindow
	bar      *nativeView
	view     *nativeView
}

var accessoryBars = map[*nativeWindow]*nativeView{}

func accessoryBarUpdate(win *nativeWindow, bar *nativeView) {
	data := bar.ext.(*accessoryBarExtension)
	height := 0
	for _, item := range data.items {
		height = max(height, item.height)
	}
	if len(data.items) == 0 {
		win.removeChrome(bar)
		return
	}
	win.addChrome(bar, true, height+8)
	data.layout(bar)
}

// addTitleAccessory adds a view to the accessory bar of a window and
// returns the controller that removes it.
func (win *nativeWindow) addTitleAccessory(view *nativeView, alignment int, width int, height int) *nativeAccessory {
	if win == nil || !view.live() || win.borderless {
		return nil
	}
	bar := accessoryBars[win]
	if bar == nil {
		bar = newView(newLayout(parking()), 0, kindLayout, win)
		bar.ext = &accessoryBarExtension{}
		accessoryBars[win] = bar
	}
	data := bar.ext.(*accessoryBarExtension)
	if alignment != 1 && alignment != 3 {
		alignment = 2
	}
	data.items = append(data.items, accessoryItem{view: view, alignment: alignment, width: max(width, 1),
		height: max(height, 1)})
	reparent(view.outer, bar.outer)
	win32ShowWindow(view.outer, swShowNA)
	accessoryBarUpdate(win, bar)
	return &nativeAccessory{win: win, bar: bar, view: view}
}

// layout places the accessories of the bar again, after the Go layer has
// given one its size.
func (accessory *nativeAccessory) layout() {
	if accessory != nil && accessory.bar != nil && accessory.bar.ext != nil {
		accessory.bar.ext.layout(accessory.bar)
	}
}

func (accessory *nativeAccessory) remove() {
	if accessory == nil {
		return
	}
	if windowLive(accessory.win) {
		data := accessory.bar.ext.(*accessoryBarExtension)
		data.items = slices.DeleteFunc(data.items, func(item accessoryItem) bool { return item.view == accessory.view })
		if accessory.view.live() {
			win32ShowWindow(accessory.view.outer, swHide)
			reparent(accessory.view.outer, accessory.win.root)
		}
		accessoryBarUpdate(accessory.win, accessory.bar)
	}
}
