package win32

import (
	"math"
	"strconv"
	"unicode/utf16"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Labels ----

type labelView struct {
	alignment    int
	wrap         bool
	truncation   int
	selectable   bool
	style        int
	maximumLines int
}

var labelStates = map[*nativeView]*labelView{}

// labelStateOf returns the state of a label, creating it.
func labelStateOf(view *nativeView) *labelView {
	state := labelStates[view]
	if state == nil {
		state = &labelView{truncation: 3}
		labelStates[view] = state
	}
	return state
}

func labelFont(view *nativeView, state *labelView) fontSpec {
	var font fontSpec
	if state.style == 4 {
		font = typeRamp(0)
		font.family = "Consolas"
		if fontFamilyInstalled("Cascadia Mono") {
			font.family = "Cascadia Mono"
		}
	} else {
		font = typeRamp(state.style)
	}
	return view.fontOr(font)
}

// labelShown returns the text that a label shows: its text shortened at the
// start or in the middle, which a static control cannot do itself, so that it
// fits the width of the label.
func labelShown(view *nativeView, state *labelView) string {
	text := view.text
	if (state.truncation != 1 && state.truncation != 2) || view.width <= 0 {
		return text
	}
	font := labelFont(view, state)
	// Wrapped text fits in its lines: the maximum number of lines, or the lines
	// that the height of the label holds.
	line := measureGDI("Ag", font, 0, 0).cy
	held := max(1, view.height/max(1, line))
	lines := held
	if state.maximumLines > 0 {
		lines = state.maximumLines
	}
	if state.wrap && view.height > 0 {
		lines = min(lines, held)
	}
	fits := func(value string) bool {
		if !state.wrap {
			return measureGDI(value, font, 0, 0).cx <= view.width
		}
		return measureGDI(value, font, view.width, 0).cy <= line*lines
	}
	if fits(text) {
		return text
	}
	units := utf16.Encode([]rune(text))
	part := func(from, to int) string {
		return string(utf16.Decode(units[from:to]))
	}
	low, high := 0, len(units)
	best := "…"
	for low <= high && high > 0 {
		keep := (low + high) / 2
		var candidate string
		if state.truncation == 1 {
			candidate = "…" + part(len(units)-keep, len(units))
		} else {
			head := (keep + 1) / 2
			candidate = part(0, head) + "…" + part(len(units)-(keep-head), len(units))
		}
		if fits(candidate) {
			best = candidate
			low = keep + 1
		} else {
			if keep == 0 {
				break
			}
			high = keep - 1
		}
	}
	return best
}

func applyLabel(view *nativeView) {
	state := labelStateOf(view)
	shown := labelShown(view, state)
	if state.selectable {
		shown = controlText(shown)
	}
	win32SetWindowText(view.outer, shown)
}

// setLabelText sets the text of a label.
func setLabelText(view *nativeView, text string) {
	view.text = text
	applyLabel(view)
}

type labelExtension struct {
	baseExtension
}

// newLabelExtension returns the extension of a label.
func newLabelExtension() extension {
	return labelExtension{}
}

func (labelExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	state := labelStateOf(view)
	font := labelFont(view, state)
	line := measureGDI("Ag", font, 0, 0).cy
	if state.wrap && view.preferredWidth > 0 {
		size := measureGDI(view.text, font, view.preferredWidth, 0)
		if state.maximumLines > 0 && size.cy > line*state.maximumLines {
			size.cy = line * state.maximumLines
		}
		return d2d1SizeF{width: float32(size.cx), height: float32(max(size.cy, line))}, true
	}
	if state.wrap {
		size := measureGDI(view.text, font, 100000, 0)
		return d2d1SizeF{width: float32(size.cx), height: float32(max(size.cy, line))}, true
	}
	size := measureGDI(view.text, font, 0, 0)
	return d2d1SizeF{width: float32(size.cx), height: float32(line)}, true
}

func (labelExtension) layout(view *nativeView) {
	if state := labelStateOf(view); state.truncation == 1 || state.truncation == 2 {
		applyLabel(view)
	}
}

func (labelExtension) themeChanged(view *nativeView) {
	view.font = fontHandle(labelFont(view, labelStateOf(view)), win32GetDpiForWindow(view.outer))
	win32SendMessage(view.outer, wmSetFont, uintptr(view.font), 1)
}

func (view *nativeView) configureLabel(alignment int, wrap bool, truncation int, selectable bool, style int, maximumLines int) {
	if !view.live() || view.kind != kindLabel {
		return
	}
	state := labelStateOf(view)
	wasSelectable := state.selectable
	state.alignment = alignment
	state.wrap = wrap
	state.truncation = truncation
	state.selectable = selectable
	state.style = style
	state.maximumLines = maximumLines
	if _, ok := view.ext.(labelExtension); !ok {
		view.ext = labelExtension{}
	}
	if selectable {
		// A static control cannot select its text, so a selectable label is a
		// read-only edit control without a border.
		editStyle := uint32(esReadOnly | esMultiline | wsTabStop)
		switch alignment {
		case 1:
			editStyle |= esCenter
		case 2:
			editStyle |= esRight
		default:
			editStyle |= esLeft
		}
		if !wrap {
			editStyle |= esAutoHScroll
		}
		edit := makeNative(wcEdit, "", editStyle, 0, 0)
		view.replaceOuter(edit)
		subclass(edit)
		applyThemeTo(edit, view.theme(), "")
	} else {
		staticStyle := uint32(ssNoPrefix | ssNotify)
		switch alignment {
		case 1:
			staticStyle |= ssCenter
		case 2:
			staticStyle |= ssRight
		default:
			staticStyle |= ssLeft
		}
		if !wrap {
			staticStyle |= ssCenterImage
			if truncation == 3 {
				staticStyle |= ssEndEllipsis
			}
		} else if truncation == 3 {
			staticStyle |= ssEditControl | ssEndEllipsis
		}
		if wasSelectable {
			label := makeNative(wcStatic, "", staticStyle, 0, 0)
			view.replaceOuter(label)
			subclass(label)
		} else {
			current := win32GetWindowLongPtr(view.outer, gwlStyle)
			current &^= ssTypeMask | ssCenterImage | ssEllipsisMask | ssEditControl
			win32SetWindowLongPtr(view.outer, gwlStyle, current|uintptr(staticStyle))
		}
	}
	view.ext.themeChanged(view)
	applyLabel(view)
	win32InvalidateRect(view.outer, nil, true)
}

func (view *nativeView) labelPreferredWidth(width int) {
	if view.live() {
		view.preferredWidth = max(width, 0)
	}
}

func forgetLabel(view *nativeView) {
	delete(labelStates, view)
}

// ---- Accelerators ----
//
// A button can have a shortcut key. The window looks for a shortcut before it
// sends a key to the control with the focus.

type accelerator struct {
	key       uint32
	modifiers int
}

var accelerators = map[*nativeView]accelerator{}

var namedKeys = map[string]uint32{
	"Up": vkUp, "Down": vkDown, "Left": vkLeft, "Right": vkRight, "Delete": vkDelete, "Home": vkHome,
	"End": vkEnd, "PageUp": vkPrior, "PageDown": vkNext, "Backspace": vkBack, "Return": vkReturn,
	"Enter": vkReturn, "Escape": vkEscape, "Tab": vkTab, "Space": vkSpace, ",": vkOEMComma, ".": vkOEMPeriod,
	"-": vkOEMMinus, "=": vkOEMPlus, "+": vkOEMPlus, ";": vkOEM1, "/": vkOEM2, "`": vkOEM3, "[": vkOEM4,
	"\\": vkOEM5, "]": vkOEM6, "'": vkOEM7,
}

// virtualKey parses the key name of a shortcut that the Go layer makes.
func virtualKey(name string) uint32 {
	if name == "" {
		return 0
	}
	if key, ok := namedKeys[name]; ok {
		return key
	}
	if len(name) >= 2 && name[0] == 'F' && name[1] >= '1' && name[1] <= '9' {
		if number, err := strconv.Atoi(name[1:]); err == nil && number >= 1 && number <= 24 {
			return vkF1 + uint32(number) - 1
		}
	}
	if len(name) == 1 && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= '0' && name[0] <= '9')) {
		return uint32(name[0])
	}
	return 0
}

// acceleratorModifiers returns the shortcut modifier bits that are down:
// Control 1, Alt 2, Shift 4.
func acceleratorModifiers() int {
	modifiers := 0
	if keyDown(vkControl) {
		modifiers |= 1
	}
	if keyDown(vkMenu) {
		modifiers |= 2
	}
	if keyDown(vkShift) {
		modifiers |= 4
	}
	return modifiers
}

// acceleratorPress clicks the button of a window whose shortcut is the key
// that the message presses.
func acceleratorPress(win *nativeWindow, message *msg) bool {
	if message.message != wmKeyDown && message.message != wmSysKeyDown {
		return false
	}
	key := uint32(message.wParam)
	modifiers := acceleratorModifiers()
	for view, shortcut := range accelerators {
		if shortcut.key != key || shortcut.modifiers != modifiers || !view.live() {
			continue
		}
		target := view.target()
		if windowOf(target) != win || !win32IsWindowVisible(target) || !win32IsWindowEnabled(target) {
			continue
		}
		if key == vkReturn || key == vkEscape {
			// A control that handles Return or Escape itself keeps it.
			code := win32SendMessage(message.hwnd, wmGetDlgCode, message.wParam, uintptr(unsafe.Pointer(message)))
			name := className(message.hwnd)
			multiline := name == wcEdit && win32GetWindowLongPtr(message.hwnd, gwlStyle)&esMultiline != 0
			if code&dlgcWantAllKeys != 0 || multiline || name == wcComboBox {
				continue
			}
		}
		win32SendMessage(target, bmClick, 0, 0)
		return true
	}
	return false
}

func forgetAccelerator(view *nativeView) {
	delete(accelerators, view)
}

// ---- Buttons ----

// buttonParts holds what a button shows besides its text: an icon, a badge,
// and a spinner while it is busy.
type buttonParts struct {
	icon      *iconImage
	placement int
	images    himagelist
	spinner   *nativeView
	badge     *nativeView
	badgeText string
	busy      bool
}

var buttonPartsOf = map[*nativeView]*buttonParts{}

// partsOf returns the parts of a button, creating them.
func partsOf(view *nativeView) *buttonParts {
	parts := buttonPartsOf[view]
	if parts == nil {
		parts = &buttonParts{}
		buttonPartsOf[view] = parts
	}
	return parts
}

func applyButtonIcon(view *nativeView) {
	parts := buttonPartsOf[view]
	if parts == nil {
		return
	}
	button := view.target()
	if parts.images != 0 {
		win32ImageListDestroy(parts.images)
		parts.images = 0
	}
	list := buttonImageList{}
	if parts.icon != nil {
		scale := view.scale()
		size := int32(math.Round(16 * scale))
		parts.images = win32ImageListCreate(size, size, ilcColor32, 1, 0)
		bitmap := iconBitmap(parts.icon, int(size), view.palette().text)
		win32ImageListAdd(parts.images, bitmap, 0)
		win32DeleteObject(hgdiobj(bitmap))
		list.himl = parts.images
		alignments := []uint32{buttonImageListAlignLeft, buttonImageListAlignRight, buttonImageListAlignTop,
			buttonImageListAlignBottom, buttonImageListAlignCenter}
		placement := parts.placement
		if placement < 0 || placement >= len(alignments) {
			placement = 0
		}
		list.uAlign = alignments[placement]
		// The icon keeps the padding of the button from the border that it is
		// next to, and a gap from the label.
		edge, gap := int32(math.Round(8*scale)), int32(math.Round(2*scale))
		switch parts.placement {
		case 0:
			list.margin = rect{left: edge, right: gap}
		case 1:
			list.margin = rect{left: gap, right: edge}
		case 2:
			list.margin = rect{top: edge / 2, bottom: gap}
		case 3:
			list.margin = rect{top: gap, bottom: edge / 2}
		}
	}
	win32SendMessage(button, bcmSetImageList, 0, uintptr(unsafe.Pointer(&list)))
	iconOnly := parts.icon != nil && parts.placement == 4
	if iconOnly || parts.busy {
		win32SetWindowText(button, "")
	} else {
		win32SetWindowText(button, view.text)
	}
	win32InvalidateRect(button, nil, true)
}

func luminance(color themeColor) float32 {
	return 0.2126*color.r + 0.7152*color.g + 0.0722*color.b
}

// shade moves a color toward white or black, away from its own lightness, by
// amount.
func shade(color themeColor, amount float32) themeColor {
	var target float32
	if luminance(color) < 0.5 {
		target = 1
	}
	color.r += (target - color.r) * amount
	color.g += (target - color.g) * amount
	color.b += (target - color.b) * amount
	return color
}

// buttonCustomDraw draws a push button whose style sets a background color or
// a corner radius, which the visual style of a button cannot show.
func buttonCustomDraw(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.code != nmCustomDraw || header.hwndFrom != view.outer ||
		(!view.style.hasBackground && view.style.cornerRadius <= 0) {
		return false
	}
	draw := (*nmCustomDrawInfo)(unsafe.Pointer(header))
	if draw.dwDrawStage != cddsPrePaint {
		*result = cdrfDoDefault
		return true
	}
	pal := view.palette()
	enabled := win32IsWindowEnabled(view.outer)
	pressed := draw.uItemState&cdisSelected != 0
	hot := draw.uItemState&cdisHot != 0
	focused := draw.uItemState&cdisFocus != 0 && win32SendMessage(view.outer, wmQueryUIState, 0, 0)&uisfHideFocus == 0
	fill := pal.control
	if view.style.hasBackground {
		fill = view.style.background
	}
	if pressed {
		fill = shade(fill, 0.2)
	} else if hot {
		fill = shade(fill, 0.1)
	}
	ink := view.textColorOr(pal.text)
	font := view.fontOr(bodyFont())
	var icon *iconImage
	busy := false
	if parts := buttonPartsOf[view]; parts != nil {
		icon = parts.icon
		busy = parts.busy
	}
	text := view.text
	paintCanvas(view.outer, draw.hdc, &draw.rc, func(canvas *painter) {
		radius := float32(view.style.cornerRadius)
		opacity := float32(0.5)
		if enabled {
			opacity = 1
		}
		face := fill
		face.a *= opacity
		canvas.fillRound(0, 0, canvas.width, canvas.height, radius, face)
		if !view.style.hasBackground {
			canvas.strokeRound(0.5, 0.5, canvas.width-1, canvas.height-1, radius, pal.controlStroke, 1)
		}
		if focused {
			canvas.strokeRound(1.5, 1.5, canvas.width-3, canvas.height-3, max(radius-1.5, 0), pal.focus, 2)
		}
		if busy {
			return
		}
		color := ink
		color.a *= opacity
		var iconSize, gap, textWidth float32
		if icon != nil {
			iconSize = 16
			if text != "" {
				gap = 6
			}
		}
		if text != "" {
			textWidth = float32(measureGDI(text, font, 0, 0).cx + 2)
		}
		x := (canvas.width - (iconSize + gap + textWidth)) / 2
		if icon != nil {
			drawIcon(canvas, icon, x, (canvas.height-iconSize)/2, iconSize, color)
		}
		if text != "" {
			canvas.text(text, font, x+iconSize+gap, 0, textWidth, canvas.height, color, 1, 1, false, true)
		}
	})
	*result = cdrfSkipDefault
	return true
}

// buttonThemeChanged updates the icon and the badge of a button for a new
// theme.
func buttonThemeChanged(view *nativeView) {
	parts := buttonPartsOf[view]
	if parts == nil {
		return
	}
	// The glyph of the icon takes the text color of the theme.
	if parts.icon != nil {
		applyButtonIcon(view)
	}
	if parts.badge != nil {
		win32InvalidateRect(parts.badge.outer, nil, false)
	}
}

// buttonIconPlacement returns the placement of the icon of a button: 0 before,
// 1 after, 2 above, 3 below, 4 alone, or -1 without an icon.
func buttonIconPlacement(view *nativeView) int {
	parts := buttonPartsOf[view]
	if parts == nil || parts.icon == nil {
		return -1
	}
	return parts.placement
}

// buttonLayout places the spinner and the badge of a button.
func buttonLayout(view *nativeView) {
	parts := buttonPartsOf[view]
	if parts == nil {
		return
	}
	if parts.spinner != nil {
		const side = 16
		placeWindow(parts.spinner.outer, (float64(view.width)-side)/2, (float64(view.height)-side)/2, side, side)
		if parts.busy {
			win32ShowWindow(parts.spinner.outer, swShowNA)
		} else {
			win32ShowWindow(parts.spinner.outer, swHide)
		}
	}
	if parts.badge == nil {
		return
	}
	// The badge sits on the top-right corner of the button, over its edge, so
	// it is a sibling above the button, which follows it.
	button := view.outer
	badge := parts.badge.outer
	scale := view.scale()
	text := measureText(parts.badgeText, typeRamp(1), 0, 0)
	height := int32(math.Round(16 * scale))
	width := int32(math.Round(max(16, float64(text.width)+10) * scale))
	// The badge goes in the nearest enclosing layout with room above the
	// button, so that the layout does not clip it.
	isLayout := func(h hwnd) bool {
		return className(h) == layoutClass
	}
	parent := win32GetParent(button)
	var frame rect
	for {
		win32GetWindowRect(button, &frame)
		mapWindowRect(0, parent, &frame)
		above := win32GetParent(parent)
		if frame.top-height/2 >= 0 || above == 0 || !isLayout(above) || !isLayout(parent) || win32GetParent(above) == 0 {
			break
		}
		parent = above
	}
	if win32GetParent(badge) != parent {
		win32SetParent(badge, parent)
	}
	var client rect
	win32GetClientRect(parent, &client)
	x := min(frame.right-width*2/3, client.right-width)
	// The top edge of the button runs through the middle of the badge.
	y := max(frame.top-height/2, 0)
	flags := uint32(swpNoActivate | swpHideWindow)
	if parts.badgeText != "" && win32IsWindowVisible(button) {
		flags = swpNoActivate | swpShowWindow
	}
	win32SetWindowPos(badge, hwndTop, x, y, width, height, flags)
	win32SetWindowRgn(badge, win32CreateRoundRectRgn(0, 0, width+1, height+1, height, height), true)
}

// badgePainterExtension draws the count badge on the corner of a button, which
// Windows has no control for.
type badgePainterExtension struct {
	baseExtension
	button *nativeView
}

func (b *badgePainterExtension) paint(view *nativeView, canvas *painter) {
	parts := buttonPartsOf[b.button]
	if parts == nil {
		return
	}
	// Windows draws count badges in the same red in both themes.
	canvas.fillRound(0, 0, canvas.width, canvas.height, canvas.height/2, rgbColor(0xc42b1c))
	font := typeRamp(1)
	font.weight = 700
	canvas.centeredText(parts.badgeText, font, 0, 0, canvas.width, canvas.height, rgbColor(0xffffff))
}

func (b *badgePainterExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	// The pointer reaches the button under the badge.
	if message == wmNCHitTest {
		*result = htTransparent
		return true
	}
	return false
}

func (view *nativeView) configureButton(icon *iconImage, placement int, role int, key string, modifiers int) {
	if !view.live() || (view.kind != kindButton && view.kind != kindToggle) {
		return
	}
	parts := partsOf(view)
	parts.icon = icon
	parts.placement = placement
	applyButtonIcon(view)
	button := view.target()
	if view.kind == kindButton {
		style := win32GetWindowLongPtr(button, gwlStyle) &^ (bsDefPushButton | bsPushButton)
		if role == 1 {
			style |= bsDefPushButton
		} else {
			style |= bsPushButton
		}
		win32SetWindowLongPtr(button, gwlStyle, style)
		win32InvalidateRect(button, nil, true)
	}
	var shortcut accelerator
	switch {
	case key != "":
		shortcut.key = virtualKey(key)
		shortcut.modifiers = modifiers
	case role == 1:
		shortcut.key = vkReturn
	case role == 2:
		shortcut.key = vkEscape
	}
	if shortcut.key != 0 {
		accelerators[view] = shortcut
	} else {
		delete(accelerators, view)
	}
}

func (view *nativeView) setButtonBusy(busy bool) {
	if !view.live() || (view.kind != kindButton && view.kind != kindToggle) {
		return
	}
	parts := partsOf(view)
	parts.busy = busy
	if busy && parts.spinner == nil {
		parts.spinner = newActivityView(view.target())
		parts.spinner.owner = view
		// The button does not draw over its spinner.
		win32SetWindowLongPtr(view.outer, gwlStyle, win32GetWindowLongPtr(view.outer, gwlStyle)|wsClipChildren)
	}
	if parts.spinner != nil {
		parts.spinner.setActivityRunning(busy)
	}
	applyButtonIcon(view)
	buttonLayout(view)
	view.accessibleValue = ""
	if busy {
		view.accessibleValue = "busy"
	}
}

// buttonBusy reports whether a button is busy.
func buttonBusy(view *nativeView) bool {
	parts := buttonPartsOf[view]
	return parts != nil && parts.busy
}

func (view *nativeView) setButtonBadge(text string) {
	if !view.live() || (view.kind != kindButton && view.kind != kindToggle) {
		return
	}
	parts := partsOf(view)
	parts.badgeText = text
	if parts.badge == nil && text != "" {
		parts.badge = createDrawn(nil, kindGeneric, &badgePainterExtension{button: view}, false)
		parts.badge.owner = view
		// The button does not draw over the badge, a sibling above it.
		win32SetWindowLongPtr(view.outer, gwlStyle, win32GetWindowLongPtr(view.outer, gwlStyle)|wsClipSiblings)
	}
	if parts.badge != nil {
		win32InvalidateRect(parts.badge.outer, nil, false)
	}
	buttonLayout(view)
	view.accessibleDescription = text
}

// buttonBadge returns the badge of a button.
func buttonBadge(view *nativeView) string {
	if parts := buttonPartsOf[view]; parts != nil {
		return parts.badgeText
	}
	return ""
}

// setButtonText sets the text of a push button or a toggle button.
func setButtonText(view *nativeView, text string) {
	view.text = text
	if buttonPartsOf[view] != nil {
		applyButtonIcon(view)
	} else {
		win32SetWindowText(view.target(), text)
	}
}

func forgetButton(view *nativeView) {
	parts := buttonPartsOf[view]
	if parts == nil {
		return
	}
	delete(buttonPartsOf, view)
	if parts.images != 0 {
		win32ImageListDestroy(parts.images)
	}
	if parts.spinner != nil {
		parts.spinner.destroy()
	}
	if parts.badge != nil {
		parts.badge.destroy()
	}
}

// ---- Text ----

// textControl returns the edit control of a view, or 0.
func textControl(view *nativeView) hwnd {
	if view == nil {
		return 0
	}
	if field := fieldOf(view); field != nil {
		return field.edit
	}
	if combo := selectCombo(view); combo != 0 {
		info := comboBoxInfo{}
		info.cbSize = uint32(unsafe.Sizeof(info))
		if win32GetComboBoxInfo(combo, &info) && info.hwndItem != 0 {
			return info.hwndItem
		}
	}
	if name := className(view.target()); name == wcEdit || name == "RICHEDIT50W" {
		return view.target()
	}
	return 0
}

func affixLabel(view *nativeView, field *fieldExtension, suffix bool) hwnd {
	label := &field.prefix
	style := uint32(ssNoPrefix | ssCenterImage | ssLeft)
	if suffix {
		label = &field.suffix
		style = ssNoPrefix | ssCenterImage | ssRight
	}
	if *label == 0 {
		*label = makeNative(wcStatic, "", style, 0, view.outer)
		win32SendMessage(*label, wmSetFont, uintptr(view.font), 0)
	}
	return *label
}

func (view *nativeView) configureTextField(readOnly bool, maximumLength int, purpose int, alignment int, prefix string, suffix string, spellCheck bool) {
	field := fieldOf(view)
	if field == nil {
		return
	}
	view.readOnly = readOnly
	win32SendMessage(field.edit, emSetReadOnly, boolArg(readOnly), 0)
	win32SendMessage(field.edit, emSetLimitText, uintptr(max(maximumLength, 0)), 0)
	style := win32GetWindowLongPtr(field.edit, gwlStyle) &^ (esLeft | esCenter | esRight)
	switch alignment {
	case 1:
		style |= esCenter
	case 2:
		style |= esRight
	default:
		style |= esLeft
	}
	win32SetWindowLongPtr(field.edit, gwlStyle, style)
	for _, affix := range []struct {
		text   string
		window hwnd
		suffix bool
	}{{prefix, field.prefix, false}, {suffix, field.suffix, true}} {
		if affix.text == "" && affix.window == 0 {
			continue
		}
		label := affixLabel(view, field, affix.suffix)
		win32SetWindowText(label, affix.text)
		if affix.text == "" {
			win32ShowWindow(label, swHide)
		} else {
			win32ShowWindow(label, swShowNA)
		}
	}
	view.ext.layout(view)
	win32InvalidateRect(view.outer, nil, true)
}

// editText returns the text of an edit control as UTF-16.
func editText(edit hwnd) []uint16 {
	length := win32GetWindowTextLength(edit)
	if length <= 0 {
		return nil
	}
	text := make([]uint16, length+1)
	win32GetWindowText(edit, &text[0], length+1)
	return text[:length]
}

// textSelection returns the selection of a text view in the code units
// of the Go layer, which count a line break as one unit where an edit control
// counts two.
func (view *nativeView) textSelection() (start, length int) {
	edit := textControl(view)
	if edit == 0 {
		return 0, 0
	}
	var from, to uint32
	win32SendMessage(edit, emGetSel, uintptr(unsafe.Pointer(&from)), uintptr(unsafe.Pointer(&to)))
	text := editText(edit)
	logical := func(position uint32) int {
		result := 0
		for i := 0; i < int(position) && i < len(text); i++ {
			if text[i] != '\r' {
				result++
			}
		}
		return result
	}
	start = logical(from)
	return start, logical(to) - start
}

func (view *nativeView) setTextSelection(start int, length int) {
	edit := textControl(view)
	if edit == 0 {
		return
	}
	text := editText(edit)
	physical := func(position int) int {
		count, i := 0, 0
		for ; i < len(text) && count < position; i++ {
			if text[i] != '\r' {
				count++
			}
		}
		return i
	}
	logicalSize := 0
	for _, unit := range text {
		if unit != '\r' {
			logicalSize++
		}
	}
	start = max(0, min(start, logicalSize))
	length = max(0, min(length, logicalSize-start))
	win32SendMessage(edit, emSetSel, uintptr(physical(start)), uintptr(physical(start+length)))
	win32SendMessage(edit, emScrollCaret, 0, 0)
}

// textAreaView holds the options of a text area that the edit control does
// not keep.
type textAreaView struct {
	wrap          bool
	tabMovesFocus bool
}

var textAreaStates = map[*nativeView]*textAreaView{}

// textAreaWraps reports whether a text area wraps its lines.
func textAreaWraps(view *nativeView) bool {
	if state := textAreaStates[view]; state != nil {
		return state.wrap
	}
	return true
}

// textAreaTabMovesFocus reports whether Tab moves the focus out of a text
// area.
func textAreaTabMovesFocus(view *nativeView) bool {
	if state := textAreaStates[view]; state != nil {
		return state.tabMovesFocus
	}
	return true
}

func (view *nativeView) configureTextArea(wrap bool, maximumLength int, tabMovesFocus bool, spellCheck bool) {
	if !view.live() || view.kind != kindTextArea {
		return
	}
	field := fieldOf(view)
	if field == nil {
		return
	}
	state := textAreaStates[view]
	if state == nil {
		state = &textAreaView{wrap: true, tabMovesFocus: true}
		textAreaStates[view] = state
	}
	state.tabMovesFocus = tabMovesFocus
	win32SendMessage(field.edit, emSetLimitText, uintptr(max(maximumLength, 0)), 0)
	if wrap == state.wrap {
		return
	}
	// An edit control wraps or scrolls from its creation, so a change makes a
	// new control with the same text and selection.
	state.wrap = wrap
	text := windowText(field.edit)
	var from, to uint32
	win32SendMessage(field.edit, emGetSel, uintptr(unsafe.Pointer(&from)), uintptr(unsafe.Pointer(&to)))
	style := uint32(esMultiline | esWantReturn | esAutoVScroll | wsVScroll | wsTabStop)
	if !wrap {
		style |= esAutoHScroll | wsHScroll
	}
	if view.readOnly {
		style |= esReadOnly
	}
	old := field.edit
	focused := win32GetFocus() == old
	edit := makeNative(wcEdit, "", style, 0, view.outer)
	setViewOf(edit, view)
	field.edit = edit
	view.control = edit
	subclass(edit)
	view.applyFont()
	applyThemeTo(edit, view.theme(), "")
	win32SendMessage(edit, emSetMargins, ecLeftMargin|ecRightMargin, 0)
	win32SendMessage(edit, emSetLimitText, uintptr(max(maximumLength, 0)), 0)
	view.suppress++
	win32SetWindowText(edit, text)
	view.suppress--
	win32SendMessage(edit, emSetSel, uintptr(from), uintptr(to))
	clearViewOf(old)
	win32DestroyWindow(old)
	if focused {
		win32SetFocus(edit)
	}
	view.ext.layout(view)
}

func forgetTextArea(view *nativeView) {
	delete(textAreaStates, view)
}

// ---- Scrolling ----

func scrollingWindow(view *nativeView) hwnd {
	if view == nil {
		return 0
	}
	if edit := textControl(view); edit != 0 {
		return edit
	}
	if name := className(view.target()); name == wcListView || name == wcTreeView || name == "RICHEDIT50W" {
		return view.target()
	}
	return 0
}

func (view *nativeView) scrollPosition() (int, int) {
	if !view.live() {
		return 0, 0
	}
	if view.kind == kindScroll {
		ox, oy := scrollOffsetOf(view)
		return int(math.Floor(ox)), int(math.Floor(oy))
	}
	target := scrollingWindow(view)
	if target == 0 {
		return 0, 0
	}
	scale := view.scale()
	var x, y int
	info := scrollInfo{fMask: sifPos}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if win32GetScrollInfo(target, sbHorz, &info) {
		x = int(math.Round(float64(info.nPos) / scale))
	}
	info.nPos = 0
	if win32GetScrollInfo(target, sbVert, &info) {
		y = int(math.Round(float64(info.nPos) / scale))
	}
	return x, y
}

func makeWParam(low, high int) uintptr {
	return uintptr(uint16(low)) | uintptr(uint16(high))<<16
}

func (view *nativeView) setScrollPosition(x int, y int, animated bool) {
	if !view.live() {
		return
	}
	if view.kind == kindScroll {
		setScrollOffset(view, float64(max(x, 0)), float64(max(y, 0)))
		return
	}
	target := scrollingWindow(view)
	if target == 0 {
		return
	}
	scale := view.scale()
	info := scrollInfo{fMask: sifAll}
	info.cbSize = uint32(unsafe.Sizeof(info))
	name := className(target)
	if name == wcListView {
		var origin point
		win32SendMessage(target, lvmGetOrigin, 0, uintptr(unsafe.Pointer(&origin)))
		item := rect{left: lvirBounds}
		rowHeight := int32(1)
		if win32SendMessage(target, lvmGetItemRect, 0, uintptr(unsafe.Pointer(&item))) != 0 {
			rowHeight = item.bottom - item.top
		}
		vertical := scrollInfo{fMask: sifPos}
		vertical.cbSize = uint32(unsafe.Sizeof(vertical))
		win32GetScrollInfo(target, sbVert, &vertical)
		currentY := vertical.nPos * max(rowHeight, 1)
		dx := int32(math.Round(float64(x)*scale)) - origin.x
		dy := int32(math.Round(float64(y)*scale)) - currentY
		win32SendMessage(target, lvmScroll, uintptr(dx), uintptr(dy))
		return
	}
	if win32GetScrollInfo(target, sbVert, &info) {
		position := int(math.Round(float64(y) * scale))
		if name == wcEdit {
			// An edit control scrolls by lines.
			line := measureGDI("Ag", view.fontOr(bodyFont()), 0, 0).cy
			first := int(win32SendMessage(target, emGetFirstVisibleLine, 0, 0))
			win32SendMessage(target, emLineScroll, 0, uintptr(y/max(line, 1)-first))
		} else {
			win32SendMessage(target, wmVScroll, makeWParam(sbThumbPosition, position), 0)
		}
	}
	if x > 0 && win32GetScrollInfo(target, sbHorz, &info) {
		win32SendMessage(target, wmHScroll, makeWParam(sbThumbPosition, int(math.Round(float64(x)*scale))), 0)
	}
}

// ---- Check boxes ----

type checkState struct {
	allowsMixed bool
	labelBefore bool
	description string
}

var checkStates = map[*nativeView]*checkState{}

func (view *nativeView) configureCheckBox(allowsMixed bool, state int, labelBefore bool, text string, description string) {
	if !view.live() || view.kind != kindCheckBox {
		return
	}
	check := &checkState{allowsMixed: allowsMixed, labelBefore: labelBefore, description: description}
	checkStates[view] = check
	box := view.outer
	style := win32GetWindowLongPtr(box, gwlStyle) &^ (bsTypeMask | bsLeftText | bsMultiline)
	if allowsMixed {
		style |= bsAuto3State
	} else {
		style |= bsAutoCheckBox
	}
	if labelBefore {
		style |= bsLeftText
	}
	if description != "" {
		style |= bsMultiline
	}
	win32SetWindowLongPtr(box, gwlStyle, style)
	setCheckBoxText(view, text)
	view.suppress++
	checked := uintptr(bstUnchecked)
	switch {
	case state == 2 && allowsMixed:
		checked = bstIndeterminate
	case state == 1:
		checked = bstChecked
	}
	win32SendMessage(box, bmSetCheck, checked, 0)
	view.suppress--
	view.accessibleDescription = description
	win32InvalidateRect(box, nil, true)
}

func (view *nativeView) checkBoxState() int {
	if !view.live() || view.kind != kindCheckBox {
		return 0
	}
	switch win32SendMessage(view.outer, bmGetCheck, 0, 0) {
	case bstIndeterminate:
		return 2
	case bstChecked:
		return 1
	default:
		return 0
	}
}

// setCheckBoxText sets the label of a check box, which shows its description
// below it.
func setCheckBoxText(view *nativeView, text string) {
	view.text = text
	shown := text
	if check := checkStates[view]; check != nil && check.description != "" {
		shown += "\n" + check.description
	}
	win32SetWindowText(view.outer, shown)
}

// checkBoxLabelBefore reports whether the label of a check box is before the
// box.
func checkBoxLabelBefore(view *nativeView) bool {
	check := checkStates[view]
	return check != nil && check.labelBefore
}

// checkBoxDescription returns the description of a check box.
func checkBoxDescription(view *nativeView) string {
	if check := checkStates[view]; check != nil {
		return check.description
	}
	return ""
}

func forgetCheckBox(view *nativeView) {
	delete(checkStates, view)
}

// ---- Choices ----

func (view *nativeView) setChoices(choices []ui.Choice) {
	if !view.live() {
		return
	}
	view.suppress++
	if view.kind == kindRadioGroup {
		setRadioChoices(view, choices)
	} else if combo := selectCombo(view); combo != 0 {
		selected := int(int32(win32SendMessage(combo, cbGetCurSel, 0, 0)))
		win32SendMessage(combo, cbResetContent, 0, 0)
		for _, choice := range choices {
			win32SendMessage(combo, cbAddString, 0, uintptr(unsafe.Pointer(wide(choice.Text))))
		}
		if selected >= len(choices) {
			selected = -1
		}
		win32SendMessage(combo, cbSetCurSel, uintptr(selected), 0)
		view.selected = selected
	}
	view.suppress--
}

func (view *nativeView) configureRadioGroup(horizontal bool, label string) {
	if view.live() && view.kind == kindRadioGroup {
		radioGroupConfigure(view, horizontal, label)
	}
}

func (view *nativeView) configureSelect(placeholder string, popupWidth int) {
	if view.live() && selectCombo(view) != 0 {
		view.placeholder = placeholder
		selectConfigure(view, placeholder, popupWidth)
	}
}

func (view *nativeView) setVisibleRows(rows int) {
	selection := selectOf(view)
	if selection == nil {
		return
	}
	selection.visibleRows = rows
	if rows <= 0 {
		selection.visibleRows = 12
	}
	win32SendMessage(selection.combo, cbSetMinVisible, uintptr(selection.visibleRows), 0)
	selection.layout(view)
}

// ---- Value controls ----

func (view *nativeView) configureSlider(vertical bool, minimum float64, maximum float64, frequency float64, snap bool) {
	slider := sliderOf(view)
	if slider == nil {
		return
	}
	view.suppress++
	view.vertical = vertical
	view.minimum = minimum
	view.maximum = max(maximum, minimum)
	view.value = clamp(view.value, view.minimum, view.maximum)
	slider.frequency = frequency
	slider.snap = snap
	style := win32GetWindowLongPtr(view.outer, gwlStyle)
	if vertical {
		style = (style &^ tbsHorz) | tbsVert
	} else {
		style = (style &^ tbsVert) | tbsHorz
	}
	win32SetWindowLongPtr(view.outer, gwlStyle, style)
	slider.configure(view)
	view.suppress--
}

func (view *nativeView) setSliderInverted(inverted bool) {
	if slider := sliderOf(view); slider != nil {
		view.inverted = inverted
		view.suppress++
		slider.configure(view)
		view.suppress--
	}
}

type progressView struct {
	text  string
	state int
}

var progressStates = map[*nativeView]*progressView{}

func (view *nativeView) configureProgress(indeterminate bool, vertical bool, text string, state int) {
	if !view.live() || view.kind != kindProgress {
		return
	}
	bar := view.outer
	progressStates[view] = &progressView{text: text, state: state}
	view.running = indeterminate
	view.vertical = vertical
	style := win32GetWindowLongPtr(bar, gwlStyle) &^ (pbsMarquee | pbsVertical)
	if indeterminate {
		style |= pbsMarquee
	}
	if vertical {
		style |= pbsVertical
	}
	win32SetWindowLongPtr(bar, gwlStyle, style)
	win32SendMessage(bar, pbmSetMarquee, boolArg(indeterminate), 30)
	if !indeterminate {
		win32SendMessage(bar, pbmSetPos, uintptr(math.Round(view.value*10000)), 0)
	}
	states := []uintptr{pbstNormal, pbstPaused, pbstPaused, pbstError}
	if state < 0 || state >= len(states) {
		state = 0
	}
	win32SendMessage(bar, pbmSetState, states[state], 0)
	view.accessibleValue = text
	win32InvalidateRect(bar, nil, true)
}

// progressStateOf returns the state of a progress bar.
func progressStateOf(view *nativeView) int {
	if state := progressStates[view]; state != nil {
		return state.state
	}
	return 0
}

func forgetProgress(view *nativeView) {
	delete(progressStates, view)
}

func (view *nativeView) configureActivity(size int, label string) {
	if !view.live() || view.kind != kindActivity {
		return
	}
	setActivitySize(view, size)
	view.setTooltip(label)
	if view.accessibleName == "" || view.accessibleName == view.text {
		view.text = label
	}
}

func (view *nativeView) configureSeparator(vertical bool, label string, inset bool, thickness int) {
	if !view.live() || view.kind != kindSeparator {
		return
	}
	insetWidth := 0
	if inset {
		insetWidth = 16
	}
	separatorConfigure(view, vertical, label, insetWidth, thickness)
	view.text = label
}

func (view *nativeView) configureImage(alignment int, nearest bool, alternative string, scale float64) {
	if !view.live() || view.kind != kindImage {
		return
	}
	view.alignment = alignment
	view.nearest = nearest
	view.sourceScale = scale
	if scale <= 0 {
		view.sourceScale = 1
	}
	view.text = alternative
	// An image without a description is decorative.
	view.accessibleHidden = alternative == "" && view.accessibleName == ""
	win32InvalidateRect(view.outer, nil, false)
}

// ---- Links ----

func (view *nativeView) configureLink(visited bool, icon *iconImage) {
	if !view.live() || view.kind != kindLink {
		return
	}
	item := lItem{mask: lifItemIndex | lifState, stateMask: lisVisited}
	if visited {
		item.state = lisVisited
	}
	win32SendMessage(view.outer, lmSetItem, 0, uintptr(unsafe.Pointer(&item)))
	view.accessibleValue = ""
	if visited {
		view.accessibleValue = "visited"
	}
	win32InvalidateRect(view.outer, nil, true)
}

// openURL opens a URL with the default program.
func openURL(target string, background bool) bool {
	if target == "" {
		return false
	}
	info := shellExecuteInfo{
		fMask:  seeMaskNoAsync | seeMaskFlagNoUI,
		lpVerb: wide("open"),
		lpFile: wide(target),
		nShow:  swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	var foreground hwnd
	if background {
		info.nShow = swShowNoActivate
		foreground = win32GetForegroundWindow()
	}
	if !win32ShellExecuteEx(&info) {
		return false
	}
	if foreground != 0 {
		// The application that opens the URL may still activate itself, so the
		// window that was in front takes the foreground back.
		win32SetForegroundWindow(foreground)
		later(func() {
			if win32IsWindow(foreground) {
				win32SetForegroundWindow(foreground)
			}
		})
	}
	return true
}

// ---- Scroll views ----

func (view *nativeView) configureScroll(axes int, policy int, top int, right int, bottom int, left int, overscroll bool) {
	if !view.live() || view.kind != kindScroll {
		return
	}
	view.axes = axes
	style := win32GetWindowLongPtr(view.outer, gwlStyle) &^ (wsHScroll | wsVScroll)
	if policy != 2 {
		if axes == 1 || axes == 2 {
			style |= wsHScroll
		}
		if axes == 0 || axes == 2 {
			style |= wsVScroll
		}
	}
	win32SetWindowLongPtr(view.outer, gwlStyle, style)
	win32SetWindowPos(view.outer, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
	setScrollInsets(view, top, right, bottom, left)
	switch policy {
	case 1:
		setScrollPolicy(view, 2)
	case 2:
		setScrollPolicy(view, 1)
	default:
		setScrollPolicy(view, 0)
	}
}

func (view *nativeView) setZoom(zoom float64) {
	if scroll := scrollOf(view); scroll != nil {
		scroll.zoom = zoom
		if zoom <= 0 {
			scroll.zoom = 1
		}
		scroll.update(view)
	}
}

func (view *nativeView) zoomFactor() float64 {
	if scroll := scrollOf(view); scroll != nil {
		return scroll.zoom
	}
	return 1
}

func (scrollView *nativeView) scrollToView(target *nativeView, animated bool) bool {
	if !scrollView.live() || !target.live() || scrollView.kind != kindScroll || scrollView.body == 0 {
		return false
	}
	if !win32IsChild(scrollView.body, target.outer) {
		return false
	}
	// The position of the view in the content of the scroll view, in logical
	// units.
	var bounds rect
	win32GetWindowRect(target.outer, &bounds)
	mapWindowRect(0, scrollView.body, &bounds)
	scale := scrollView.scale()
	x, y := float64(bounds.left)/scale, float64(bounds.top)/scale
	width, height := float64(bounds.right-bounds.left)/scale, float64(bounds.bottom-bounds.top)/scale
	ox, oy := scrollOffsetOf(scrollView)
	viewportWidth, viewportHeight := viewportSizeOf(scrollView)
	if x+width > ox+viewportWidth {
		ox = x + width - viewportWidth
	}
	if x < ox {
		ox = x
	}
	if y+height > oy+viewportHeight {
		oy = y + height - viewportHeight
	}
	if y < oy {
		oy = y
	}
	setScrollOffset(scrollView, ox, oy)
	return true
}
