package win32

import (
	"math"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// Severity: 0 information, 1 success, 2 warning, 3 error.

func severityGlyph(severity int) string {
	switch severity {
	case 1:
		return ""
	case 2:
		return ""
	case 3:
		return ""
	default:
		return ""
	}
}

// severityRole returns the ui.SemanticColor of a severity.
func severityRole(severity int) int {
	switch severity {
	case 1:
		return 6
	case 2:
		return 7
	case 3:
		return 8
	default:
		return 9
	}
}

func severityColor(pal *palette, severity int) themeColor {
	switch severity {
	case 1:
		return pal.success
	case 2:
		return pal.caution
	case 3:
		return pal.critical
	default:
		return pal.information
	}
}

func severityBackground(pal *palette, severity int) themeColor {
	switch severity {
	case 1:
		return pal.successBackground
	case 2:
		return pal.cautionBackground
	case 3:
		return pal.criticalBackground
	default:
		return pal.informationBackground
	}
}

// glyphLabel creates a label that shows a glyph of the symbol font.
func glyphLabel(parent hwnd, glyph string, size float32) hwnd {
	label := makeNative(wcStatic, glyph, ssCenter|ssCenterImage|ssNoPrefix, 0, parent)
	symbol := fontSpec{family: symbolFamily(), size: size, weight: 400}
	win32SendMessage(label, wmSetFont, uintptr(fontHandle(symbol, win32GetDpiForWindow(label))), 0)
	return label
}

// actionButton is the button of an action.
type actionButton struct {
	hwnd hwnd
	id   string
}

// makeActionButtons creates a button for each action.
func makeActionButtons(parent hwnd, owner *nativeView, actions []ui.Action) []actionButton {
	buttons := make([]actionButton, len(actions))
	for i, action := range actions {
		button := makeNative(wcButton, action.Text, bsPushButton|wsTabStop, 0, parent)
		if owner != nil {
			setViewOf(button, owner)
		}
		win32SendMessage(button, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(button))), 0)
		win32EnableWindow(button, action.Enabled)
		buttons[i] = actionButton{hwnd: button, id: action.ID}
	}
	return buttons
}

// clickedAction returns the action of the button that sent a command.
func clickedAction(buttons []actionButton, from hwnd) (string, bool) {
	for _, button := range buttons {
		if button.hwnd == from {
			return button.id, true
		}
	}
	return "", false
}

func buttonWidth(button hwnd) float64 {
	return max(72, float64(measureGDI(windowText(button), bodyFont(), 0, 0).cx+24))
}

// ---- Announcements ----
//
// An announcement is the text of a hidden label of the window that assistive
// technology reads as a live region.

func announcer(top hwnd) hwnd {
	if label := hwnd(win32GetProp(top, "GKAnnouncer")); label != 0 && win32IsWindow(label) {
		return label
	}
	label := makeNative(wcStatic, "", ssLeft|ssNoPrefix, 0, top)
	win32SetWindowPos(label, hwndBottom, -10000, -10000, 1, 1, swpNoActivate)
	win32SetProp(top, "GKAnnouncer", uintptr(label))
	return label
}

// announceText reports text to assistive technology.
func announceText(h hwnd, text string) {
	if h == 0 || text == "" {
		return
	}
	top := win32GetAncestor(h, gaRoot)
	if top == 0 {
		top = h
	}
	label := announcer(top)
	win32SetWindowText(label, text)
	var services comObject
	if win32CoCreateInstance(clsidAccPropServices, 0, clsctxInprocServer, iidAccPropServices, &services).succeeded() {
		polite := variant{vt: vtI4}
		polite.data[0] = 1
		win32AccPropServicesSetHwndProp(services, label, objidClient, childidSelf, liveSettingProperty, &polite)
		win32AccPropServicesSetHwndPropStr(services, label, objidClient, childidSelf, propidAccName, text)
		win32UnknownRelease(services)
	}
	win32NotifyWinEvent(eventObjectLiveRegionChanged, label, objidClient, childidSelf)
}

func (view *nativeView) announce(text string) {
	if view.live() {
		announceText(view.outer, text)
	}
}

// ---- Info bar ----
//
// An info bar is a band in the color of its severity with an icon, a title, a
// message, its actions, and a close button.

type infoBarExtension struct {
	baseExtension
	severity                    int
	icon, title, detail, closer hwnd
	buttons                     []actionButton
}

func (b *infoBarExtension) colors(view *nativeView) {
	background := severityBackground(view.palette(), b.severity)
	setLayoutBackground(view.outer, &background)
	setTextRole(b.icon, severityRole(b.severity))
	win32InvalidateRect(view.outer, nil, true)
}

func (b *infoBarExtension) layout(view *nativeView) {
	height := min(float64(view.height), 40)
	x := 12.0
	placeWindow(b.icon, x, 0, 20, height)
	x += 28
	right := float64(view.width) - 8
	if b.closer != 0 {
		right -= 28
		placeWindow(b.closer, right, (height-28)/2, 28, 28)
		right -= 8
	}
	for i := len(b.buttons) - 1; i >= 0; i-- {
		width := buttonWidth(b.buttons[i].hwnd)
		right -= width
		placeWindow(b.buttons[i].hwnd, right, (height-26)/2, width, 26)
		right -= 8
	}
	titleWidth := float64(measureGDI(view.text, typeRamp(2), 0, 0).cx)
	placeWindow(b.title, x, 0, min(titleWidth+2, max(0, right-x)), height)
	x += titleWidth + 12
	placeWindow(b.detail, x, 0, max(0, right-x), height)
}

func (b *infoBarExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	if from == b.closer {
		send(view.receiver, closeEvent{})
		return true
	}
	if id, ok := clickedAction(b.buttons, from); ok {
		send(view.receiver, actionEvent{id: id})
		return true
	}
	return false
}

func (b *infoBarExtension) themeChanged(view *nativeView) {
	b.colors(view)
}

func (b *infoBarExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	width := 48 + float64(measureGDI(view.text, typeRamp(2), 0, 0).cx) + 12 +
		float64(measureGDI(view.placeholder, bodyFont(), 0, 0).cx) + 12
	for _, button := range b.buttons {
		width += buttonWidth(button.hwnd) + 8
	}
	if b.closer != 0 {
		width += 36
	}
	return d2d1SizeF{width: float32(width), height: 40}, true
}

func (win *nativeWindow) newInfoBar(closable bool, actions []ui.Action) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindInfoBar, win)
	bar := &infoBarExtension{}
	view.ext = bar
	bar.icon = glyphLabel(host, severityGlyph(0), 14)
	bar.title = makeNative(wcStatic, "", ssLeft|ssCenterImage|ssNoPrefix|ssEndEllipsis, 0, host)
	bar.detail = makeNative(wcStatic, "", ssLeft|ssCenterImage|ssNoPrefix|ssEndEllipsis, 0, host)
	win32SendMessage(bar.title, wmSetFont, uintptr(fontHandle(typeRamp(2), win32GetDpiForWindow(host))), 0)
	win32SendMessage(bar.detail, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(host))), 0)
	bar.buttons = makeActionButtons(host, view, actions)
	if closable {
		bar.closer = makeNative(wcButton, "", bsPushButton|wsTabStop, 0, host)
		setViewOf(bar.closer, view)
		symbol := fontSpec{family: symbolFamily(), size: 10, weight: 400}
		win32SendMessage(bar.closer, wmSetFont, uintptr(fontHandle(symbol, win32GetDpiForWindow(host))), 0)
	}
	bar.colors(view)
	return view
}

func (view *nativeView) setInfoBar(severity int, title string, detail string) {
	if !view.live() || view.kind != kindInfoBar {
		return
	}
	bar := view.ext.(*infoBarExtension)
	bar.severity = severity
	if severity < 0 || severity >= 4 {
		bar.severity = 0
	}
	view.text = title
	view.placeholder = detail
	win32SetWindowText(bar.icon, severityGlyph(bar.severity))
	win32SetWindowText(bar.title, title)
	win32SetWindowText(bar.detail, detail)
	text := title
	if detail != "" {
		text += " " + detail
	}
	if view.accessibleName == "" || view.accessibleName == text {
		view.accessibleName = text
	}
	bar.colors(view)
	bar.layout(view)
}

// ---- Tooltip ----
//
// A tooltip that the application shows is a tracking tooltip of Windows.

type nativeTooltip struct {
	receiver eventReceiver
	tip      hwnd
	text     string
	anchor   hwnd
	shown    bool
	timer    uintptr
}

func (t *nativeTooltip) destroy() {
	cancelTimer(t.timer)
	if t.tip != 0 {
		win32DestroyWindow(t.tip)
	}
}

// trackTool returns the tool of a tracking tooltip.
func trackTool() toolInfo {
	info := toolInfo{uId: 1}
	info.cbSize = uint32(unsafe.Sizeof(info))
	return info
}

func newTooltip() *nativeTooltip {
	tooltip := &nativeTooltip{}
	cookie := activate()
	tooltip.tip = win32CreateWindowEx(wsExTopmost|wsExTransparent, tooltipsClass, "", wsPopup|ttsNoPrefix|ttsAlwaysTip,
		cwUseDefault, cwUseDefault, cwUseDefault, cwUseDefault, 0, 0, instance(), nil)
	deactivate(cookie)
	win32SendMessage(tooltip.tip, ttmSetMaxTipWidth, 0, 400)
	info := trackTool()
	info.uFlags = ttfTrack | ttfAbsolute
	info.lpszText = wide("")
	win32SendMessage(tooltip.tip, ttmAddTool, 0, uintptr(unsafe.Pointer(&info)))
	return tooltip
}

func (tooltip *nativeTooltip) setText(text string) {
	if tooltip == nil {
		return
	}
	tooltip.text = text
	info := trackTool()
	info.lpszText = wide(text)
	win32SendMessage(tooltip.tip, ttmUpdateTipText, 0, uintptr(unsafe.Pointer(&info)))
}

func (t *nativeTooltip) open(edge int) {
	if t.anchor == 0 || !win32IsWindow(t.anchor) || !win32IsWindowVisible(t.anchor) {
		return
	}
	var anchor rect
	win32GetWindowRect(t.anchor, &anchor)
	info := trackTool()
	win32SendMessage(t.tip, ttmTrackActivate, 1, uintptr(unsafe.Pointer(&info)))
	var tip rect
	win32GetWindowRect(t.tip, &tip)
	width, height := tip.right-tip.left, tip.bottom-tip.top
	const gap = 4
	x, y := (anchor.left+anchor.right-width)/2, anchor.bottom+gap
	switch edge {
	case 0:
		y = anchor.top - height - gap
	case 2:
		x = anchor.left - width - gap
		y = (anchor.top + anchor.bottom - height) / 2
	case 3:
		x = anchor.right + gap
		y = (anchor.top + anchor.bottom - height) / 2
	}
	win32SendMessage(t.tip, ttmTrackPosition, 0, makeWParam(int(x), int(y)))
	t.shown = true
}

func (tooltip *nativeTooltip) show(anchor *nativeView, edge int, delay float64) bool {
	var win *nativeWindow
	if anchor.live() {
		win = windowOf(anchor.outer)
	}
	if tooltip == nil || !anchor.live() || win == nil || !win32IsWindowVisible(win.hwnd) ||
		!win32IsWindowVisible(anchor.outer) {
		return false
	}
	cancelTimer(tooltip.timer)
	tooltip.timer = 0
	tooltip.anchor = anchor.outer
	if delay <= 0 {
		tooltip.open(edge)
		return true
	}
	tooltip.shown = true
	tooltip.timer = startTimer(uint32(math.Round(delay*1000)), func() bool {
		tooltip.timer = 0
		tooltip.open(edge)
		return false
	})
	return true
}

func (tooltip *nativeTooltip) hide() {
	if tooltip == nil {
		return
	}
	cancelTimer(tooltip.timer)
	tooltip.timer = 0
	info := trackTool()
	win32SendMessage(tooltip.tip, ttmTrackActivate, 0, uintptr(unsafe.Pointer(&info)))
	tooltip.shown = false
}

// ---- Toast ----
//
// A toast is a message that floats above the bottom of the content of a window
// for a while: a dark band with the message and its actions, in the style of
// the notifications of Windows.

type nativeToast struct {
	receiver eventReceiver
	view     *nativeView
	message  hwnd
	buttons  []actionButton
	timeout  float64
	timer    uintptr
	win      *nativeWindow
}

func (t *nativeToast) dismiss() {
	cancelTimer(t.timer)
	t.timer = 0
	if t.view != nil && t.win != nil {
		win32ShowWindow(t.view.outer, swHide)
		reparent(t.view.outer, parking())
	}
	t.win = nil
}

func (t *nativeToast) destroy() {
	t.dismiss()
	if t.view != nil {
		t.view.destroy()
	}
}

type toastBandExtension struct {
	baseExtension
	toast *nativeToast
}

func (b *toastBandExtension) layout(view *nativeView) {
	x, height := 14.0, float64(view.height)
	width := float64(measureGDI(view.text, bodyFont(), 0, 0).cx + 2)
	placeWindow(b.toast.message, x, 0, width, height)
	x += width + 12
	for _, button := range b.toast.buttons {
		w := buttonWidth(button.hwnd)
		placeWindow(button.hwnd, x, (height-26)/2, w, 26)
		x += w + 8
	}
}

func (b *toastBandExtension) width(view *nativeView) float64 {
	total := 28 + float64(measureGDI(view.text, bodyFont(), 0, 0).cx+2)
	for _, button := range b.toast.buttons {
		total += buttonWidth(button.hwnd) + 8 + 4
	}
	return min(total, 560)
}

func (b *toastBandExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	if id, ok := clickedAction(b.toast.buttons, from); ok {
		send(b.toast.receiver, actionEvent{id: id})
		b.toast.dismiss()
		return true
	}
	return false
}

func newToast(receiver eventReceiver, message string, actions []ui.Action, timeout float64) *nativeToast {
	toast := &nativeToast{receiver: receiver, timeout: timeout}
	host := newLayout(parking())
	toast.view = newView(host, 0, kindGeneric, nil)
	toast.view.ext = &toastBandExtension{toast: toast}
	toast.view.text = message
	toast.message = makeNative(wcStatic, message, ssLeft|ssCenterImage|ssNoPrefix, 0, host)
	setTextRole(toast.message, textRoleInverted)
	win32SendMessage(toast.message, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(host))), 0)
	toast.buttons = makeActionButtons(host, toast.view, actions)
	return toast
}

func (toast *nativeToast) show(win *nativeWindow) bool {
	if toast == nil || win == nil || win.root == 0 {
		return false
	}
	toast.dismiss()
	band := rgbColor(0x2b2b2b)
	if paletteFor(win.themeOf()).dark {
		band = rgbColor(0xf3f3f3)
	}
	setLayoutBackground(toast.view.outer, &band)
	reparent(toast.view.outer, win.root)
	content := clientSize(win.root)
	data := toast.view.ext.(*toastBandExtension)
	width, height := data.width(toast.view), 40.0
	toast.view.x = int(math.Round((float64(content.width) - width) / 2))
	toast.view.y = int(math.Round(float64(content.height) - height - 20))
	toast.view.width = int(math.Round(width))
	toast.view.height = int(height)
	toast.view.place()
	data.layout(toast.view)
	// The band has rounded corners.
	scale := scaleOf(win.root)
	radius := int32(math.Round(8 * scale))
	region := win32CreateRoundRectRgn(0, 0, int32(math.Round(width*scale))+1, int32(math.Round(height*scale))+1, radius,
		radius)
	win32SetWindowRgn(toast.view.outer, region, true)
	win32SetWindowPos(toast.view.outer, hwndTop, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
	toast.win = win
	if toast.timeout > 0 {
		toast.timer = startTimer(uint32(math.Round(toast.timeout*1000)), func() bool {
			toast.timer = 0
			toast.dismiss()
			return false
		})
	}
	announceText(win.hwnd, toast.view.text)
	return true
}

func (toast *nativeToast) isShown() bool {
	return toast != nil && toast.win != nil
}

func (toast *nativeToast) release() {
	if toast != nil {
		toast.receiver = nil
		toast.destroy()
	}
}

// ---- Badge ----
//
// A badge is a count or a short text in a pill in the color of its severity,
// which Windows has no control for.

type badgeExtension struct {
	baseExtension
	severity int
}

func badgeFont() fontSpec {
	font := typeRamp(1)
	font.weight = 700
	return font
}

func (b *badgeExtension) paint(view *nativeView, canvas *painter) {
	pal := view.palette()
	font := badgeFont()
	text := measureText(view.text, font, 0, 0)
	height := min(canvas.height, text.height+6)
	width := min(canvas.width, max(height, text.width+18))
	x, y := (canvas.width-width)/2, (canvas.height-height)/2
	canvas.fillRound(x, y, width, height, height/2, severityColor(pal, b.severity))
	ink := rgbColor(0xffffff)
	if pal.dark {
		ink = rgbColor(0x000000)
	}
	canvas.centeredText(view.text, font, x, y, width, height, ink)
}

func (b *badgeExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	text := measureText(view.text, badgeFont(), 0, 0)
	height := text.height + 6
	return d2d1SizeF{width: max(height, text.width+18), height: height}, true
}

func (win *nativeWindow) newBadge() *nativeView {
	return createDrawn(win, kindBadge, &badgeExtension{}, false)
}

func (view *nativeView) setBadgeValue(text string, severity int, accessible string) {
	if !view.live() || view.kind != kindBadge {
		return
	}
	view.ext.(*badgeExtension).severity = severity
	view.text = text
	value := accessible
	if value == "" {
		value = text
	}
	view.accessibleValue = value
	if view.accessibleName == "" {
		view.accessibleName = value
	}
	win32InvalidateRect(view.outer, nil, false)
}

// ---- Progress dialog ----
//
// A progress dialog is the progress dialog of the shell of Windows, which runs
// without blocking the application.

type nativeProgressDialog struct {
	receiver    eventReceiver
	dialog      comObject
	shown       bool
	cancellable bool
	reported    bool
	timer       uintptr
	title       string
}

func (p *nativeProgressDialog) destroy() {
	cancelTimer(p.timer)
	if p.dialog != 0 {
		if p.shown {
			win32ProgressDialogStopProgressDialog(p.dialog)
		}
		win32UnknownRelease(p.dialog)
	}
}

func newProgressDialog(receiver eventReceiver, title string, cancellable bool) *nativeProgressDialog {
	progress := &nativeProgressDialog{receiver: receiver, cancellable: cancellable, title: title}
	if win32CoCreateInstance(clsidProgressDialog, 0, clsctxInprocServer, iidProgressDialog, &progress.dialog).failed() {
		return nil
	}
	win32ProgressDialogSetTitle(progress.dialog, title)
	if cancellable {
		win32ProgressDialogSetCancelMsg(progress.dialog, "Cancelling…")
	}
	return progress
}

// set sets the message of a progress dialog and its
// progress from 0 to 1, or keeps the progress for a negative value.
func (progress *nativeProgressDialog) set(message string, value float64) {
	if progress == nil {
		return
	}
	win32ProgressDialogSetLine(progress.dialog, 1, message, false)
	if value >= 0 {
		win32ProgressDialogSetProgress64(progress.dialog, uint64(math.Round(min(value, 1)*10000)), 10000)
	}
}

func (progress *nativeProgressDialog) show(win *nativeWindow) bool {
	if progress == nil {
		return false
	}
	var owner hwnd
	if win != nil && win.hwnd != 0 && win32IsWindowVisible(win.hwnd) {
		owner = win.hwnd
	}
	flags := uint32(progdlgNoMinimize | progdlgNoTime)
	if !progress.cancellable {
		flags |= progdlgNoCancel
	}
	if win32ProgressDialogStartProgressDialog(progress.dialog, owner, flags).failed() {
		return false
	}
	progress.shown = true
	if progress.cancellable {
		progress.timer = startTimer(100, func() bool {
			if win32ProgressDialogHasUserCancelled(progress.dialog) && !progress.reported {
				// The Go layer closes the dialog; a cancel request only
				// reports.
				progress.reported = true
				send(progress.receiver, closeEvent{})
			}
			return true
		})
	}
	return true
}

func (progress *nativeProgressDialog) release() {
	if progress != nil {
		progress.receiver = nil
		progress.destroy()
	}
}

// ---- Empty state ----

type emptyStateExtension struct {
	baseExtension
	icon          hwnd
	iconHandle    hicon
	title, detail hwnd
	buttons       []actionButton
}

func (e *emptyStateExtension) free() {
	if e.iconHandle != 0 {
		win32DestroyIcon(e.iconHandle)
	}
}

func (e *emptyStateExtension) heights(view *nativeView, width int) (title, detail int) {
	title = measureGDI(view.text, typeRamp(3), width, 0).cy
	if view.placeholder != "" {
		detail = measureGDI(view.placeholder, bodyFont(), width, 0).cy
	}
	return title, detail
}

func (e *emptyStateExtension) total(view *nativeView, title, detail int) float64 {
	total := float64(title + 6 + detail)
	if e.icon != 0 {
		total += 56
	}
	if len(e.buttons) > 0 {
		total += 38
	}
	return total
}

func (e *emptyStateExtension) layout(view *nativeView) {
	width := min(360, float64(view.width))
	titleHeight, detailHeight := e.heights(view, int(width))
	total := e.total(view, titleHeight, detailHeight)
	y := max(0, (float64(view.height)-total)/2)
	x := (float64(view.width) - width) / 2
	if e.icon != 0 {
		placeWindow(e.icon, (float64(view.width)-48)/2, y, 48, 48)
		y += 56
	}
	placeWindow(e.title, x, y, width, float64(titleHeight))
	y += float64(titleHeight + 6)
	placeWindow(e.detail, x, y, width, float64(detailHeight))
	y += float64(detailHeight + 12)
	row := 0.0
	for _, button := range e.buttons {
		row += buttonWidth(button.hwnd) + 8
	}
	bx := (float64(view.width) - row + 8) / 2
	for _, button := range e.buttons {
		w := buttonWidth(button.hwnd)
		placeWindow(button.hwnd, bx, y, w, 26)
		bx += w + 8
	}
}

func (e *emptyStateExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	if id, ok := clickedAction(e.buttons, from); ok {
		send(view.receiver, actionEvent{id: id})
		return true
	}
	return false
}

func (e *emptyStateExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	titleHeight, detailHeight := e.heights(view, 360)
	return d2d1SizeF{width: 360, height: float32(e.total(view, titleHeight, detailHeight))}, true
}

func (win *nativeWindow) newEmptyState(icon *iconImage, actions []ui.Action) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindEmptyState, win)
	state := &emptyStateExtension{}
	view.ext = state
	if image := icon; image != nil {
		state.icon = makeNative(wcStatic, "", ssIcon|ssCenterImage|ssRealSizeControl, 0, host)
		side := int(math.Round(48 * scaleOf(host)))
		state.iconHandle = iconHandle(image, side, view.palette().textTertiary)
		win32SendMessage(state.icon, stmSetIcon, uintptr(state.iconHandle), 0)
	}
	state.title = makeNative(wcStatic, "", ssCenter|ssNoPrefix, 0, host)
	state.detail = makeNative(wcStatic, "", ssCenter|ssNoPrefix, 0, host)
	setTextRole(state.detail, 2)
	win32SendMessage(state.title, wmSetFont, uintptr(fontHandle(typeRamp(3), win32GetDpiForWindow(host))), 0)
	win32SendMessage(state.detail, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(host))), 0)
	state.buttons = makeActionButtons(host, view, actions)
	return view
}

func (view *nativeView) setEmptyState(title string, detail string) {
	if !view.live() || view.kind != kindEmptyState {
		return
	}
	state := view.ext.(*emptyStateExtension)
	view.text = title
	view.placeholder = detail
	win32SetWindowText(state.title, title)
	win32SetWindowText(state.detail, detail)
	if detail == "" {
		win32ShowWindow(state.detail, swHide)
	} else {
		win32ShowWindow(state.detail, swShowNA)
	}
	if view.accessibleName == "" || view.accessibleName == title {
		view.accessibleName = title
	}
	state.layout(view)
}

// ---- Inline validation ----

type validationExtension struct {
	baseExtension
	icon, label hwnd
	severity    int
}

func (v *validationExtension) layout(view *nativeView) {
	placeWindow(v.icon, 0, 1, 16, 16)
	placeWindow(v.label, 20, 0, float64(max(0, view.width-20)), float64(view.height))
}

func (v *validationExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	width := 0
	if view.preferredWidth > 20 {
		width = view.preferredWidth - 20
	}
	size := measureGDI(view.text, typeRamp(1), width, 0)
	return d2d1SizeF{width: float32(size.cx + 20), height: float32(max(size.cy, 16))}, true
}

func (win *nativeWindow) newInlineValidation() *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindValidation, win)
	validation := &validationExtension{}
	view.ext = validation
	validation.icon = glyphLabel(host, severityGlyph(0), 12)
	validation.label = makeNative(wcStatic, "", ssLeft|ssNoPrefix, 0, host)
	win32SendMessage(validation.label, wmSetFont, uintptr(fontHandle(typeRamp(1), win32GetDpiForWindow(host))), 0)
	return view
}

func (view *nativeView) setValidation(severity int, text string) {
	if !view.live() || view.kind != kindValidation {
		return
	}
	validation := view.ext.(*validationExtension)
	validation.severity = severity
	view.text = text
	win32SetWindowText(validation.icon, severityGlyph(severity))
	win32SetWindowText(validation.label, text)
	setTextRole(validation.icon, severityRole(severity))
	if severity >= 2 {
		setTextRole(validation.label, severityRole(severity))
	} else {
		setTextRole(validation.label, 2)
	}
	win32InvalidateRect(view.outer, nil, true)
	announceText(view.outer, text)
}
