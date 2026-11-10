package win32

import (
	"math"
	"strings"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui"
)

// ---- Creation helpers ----

// nativeControl creates a view for a native control.
func nativeControl(window *nativeWindow, class, text string, style, exStyle uint32, kind int) *nativeView {
	h := makeNative(class, text, style, exStyle, 0)
	if h == 0 {
		return nil
	}
	return adoptView(window, h, 0, kind)
}

// hostView creates a view whose window is a layout that holds native controls.
func hostView(window *nativeWindow, kind int, ext extension) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kind, window)
	view.ext = ext
	return view
}

func clamp(value, minimum, maximum float64) float64 {
	return max(minimum, min(value, maximum))
}

// textHeight returns the height of a line of the body font.
func (v *nativeView) textHeight() int {
	return measureGDI("Ag", v.fontOr(bodyFont()), 0, 0).cy
}

// windowText returns the text of a window.
func windowText(h hwnd) string {
	if h == 0 {
		return ""
	}
	length := win32GetWindowTextLength(h)
	if length <= 0 {
		return ""
	}
	text := make([]uint16, length+1)
	win32GetWindowText(h, &text[0], length+1)
	return goString(text)
}

// controlText converts text to the line breaks of an edit control.
func controlText(text string) string {
	text = strings.ReplaceAll(text, "\r", "")
	return strings.ReplaceAll(text, "\n", "\r\n")
}

// ---- Layouts ----

func (win *nativeWindow) newLayoutView() *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindLayout, win)
	view.body = host
	return view
}

// ---- Labels ----

func (win *nativeWindow) newLabel(text string) *nativeView {
	view := nativeControl(win, wcStatic, text, ssLeft|ssNoPrefix|ssCenterImage|ssEndEllipsis|ssNotify, 0,
		kindLabel)
	if view == nil {
		return nil
	}
	view.text = text
	view.ext = newLabelExtension()
	return view
}

// ---- Buttons ----

type buttonExtension struct {
	baseExtension
}

func (buttonExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	// A busy button ignores clicks.
	if !buttonBusy(view) {
		view.report(clickEvent{})
	}
	return true
}

func (buttonExtension) layout(view *nativeView) {
	buttonLayout(view)
}

func (buttonExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	return buttonCustomDraw(view, header, result)
}

func (buttonExtension) themeChanged(view *nativeView) {
	buttonThemeChanged(view)
}

func (buttonExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	// The badge follows the button.
	if h == view.outer && (message == wmWindowPosChanged || message == wmShowWindow) {
		buttonLayout(view)
	}
	return false
}

func (buttonExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return buttonPreferred(view), true
}

// buttonPreferred returns the preferred size of a push button or a toggle
// button with its text and icon.
func buttonPreferred(view *nativeView) d2d1SizeF {
	text := measureGDI(view.text, view.fontOr(bodyFont()), 0, 0)
	line := float32(view.textHeight())
	width := float32(text.cx) + 24
	height := line + 8
	// A button centers its text in its whole width, so an icon before or after
	// the text has the room of the icon on both sides.
	const icon, edge, gap = 16, 8, 4
	placement := buttonIconPlacement(view)
	switch placement {
	case 0, 1:
		width = float32(text.cx) + 2*(edge+icon+gap)
		height = max(height, icon+12)
	case 2, 3:
		width = max(float32(text.cx), icon) + 24
		height = line + icon + gap + 12
	case 4:
		width = icon + 24
		height = max(height, icon+12)
	}
	if width <= 64 && placement != 4 {
		width = 64
	}
	return d2d1SizeF{width: width, height: height}
}

func (win *nativeWindow) newButton(text string) *nativeView {
	view := nativeControl(win, wcButton, text, bsPushButton|wsTabStop, 0, kindButton)
	if view == nil {
		return nil
	}
	view.text = text
	view.ext = buttonExtension{}
	return view
}

// ---- Text boxes ----

// fieldHooks are the steps of a text box that a component built on it
// replaces.
type fieldHooks interface {
	// changed runs after the text changes.
	changed(view *nativeView)
	// submitted runs after Return.
	submitted(view *nativeView)
	// focusLost runs when the edit control loses the focus.
	focusLost(view *nativeView)
}

// fieldOwner is the extension of a text box: a fieldExtension, or an
// extension that embeds one.
type fieldOwner interface {
	extension
	fieldHooks
	field() *fieldExtension
}

// fieldExtension is a text box: an edit control in a frame that the backend
// draws with the visual style of Windows, with optional labels before and
// after the text.
type fieldExtension struct {
	baseExtension
	edit      hwnd
	prefix    hwnd
	suffix    hwnd
	updown    hwnd
	multiline bool
	hover     bool
	// hooks is the extension that owns the field, whose steps the field
	// runs.
	hooks fieldHooks
}

func (f *fieldExtension) field() *fieldExtension {
	return f
}

// fieldOf returns the text box of a view, or nil.
func fieldOf(view *nativeView) *fieldExtension {
	if view == nil {
		return nil
	}
	if owner, ok := view.ext.(fieldOwner); ok {
		return owner.field()
	}
	return nil
}

const fieldPadding = 6

func (f *fieldExtension) layout(view *nativeView) {
	var client rect
	win32GetClientRect(view.outer, &client)
	scale := view.scale()
	scaled := func(value float64) int32 {
		return int32(math.Round(value * scale))
	}
	pad := scaled(fieldPadding)
	border := scaled(1)
	left, right := pad, client.right-pad
	font := view.fontOr(bodyFont())
	if f.prefix != 0 && win32IsWindowVisible(f.prefix) {
		width := scaled(float64(measureGDI(windowText(f.prefix), font, 0, 0).cx))
		win32SetWindowPos(f.prefix, 0, left, border, width, client.bottom-2*border, swpNoZOrder|swpNoActivate)
		left += width + scaled(4)
	}
	if f.updown != 0 {
		width := scaled(18)
		win32SetWindowPos(f.updown, 0, client.right-border-width, border, width, client.bottom-2*border,
			swpNoZOrder|swpNoActivate)
		right = client.right - border - width - scaled(2)
	}
	if f.suffix != 0 && win32IsWindowVisible(f.suffix) {
		width := scaled(float64(measureGDI(windowText(f.suffix), font, 0, 0).cx))
		right -= width
		win32SetWindowPos(f.suffix, 0, right, border, width, client.bottom-2*border, swpNoZOrder|swpNoActivate)
		right -= scaled(4)
	}
	if f.multiline {
		win32SetWindowPos(f.edit, 0, border+scaled(2), border+scaled(2), client.right-2*border-scaled(2),
			client.bottom-2*border-scaled(3), swpNoZOrder|swpNoActivate)
		return
	}
	line := scaled(float64(measureGDI("Ag", font, 0, 0).cy))
	top := (client.bottom - line) / 2
	win32SetWindowPos(f.edit, 0, left, max(top, border), max(right-left, 1), line, swpNoZOrder|swpNoActivate)
}

func (f *fieldExtension) paintGDI(view *nativeView, h hwnd, dc hdc, client *rect) bool {
	focused := win32GetFocus() == f.edit
	theme := view.theme()
	enabled := win32IsWindowEnabled(f.edit)
	pal := paletteFor(theme)
	fill := rgbColor(0xffffff)
	if pal.dark {
		fill = pal.inputActive
	}
	if view.readOnly && !f.multiline {
		fill = rgbColor(0xf9f9f9)
		if pal.dark {
			fill = pal.layer
		}
	}
	if view.style.hasBackground {
		fill = mix(fill, view.style.background)
	}
	fillRectColor(dc, client, fill)
	var data htheme
	if theme != 2 {
		data = win32OpenThemeData(h, "Edit")
	}
	if data != 0 {
		state := int32(epsnNormal)
		switch {
		case !enabled:
			state = epsnDisabled
		case focused:
			state = epsnFocused
		case f.hover:
			state = epsnHot
		}
		win32DrawThemeBackground(data, dc, epEditBorderNoScroll, state, client, nil)
		win32CloseThemeData(data)
	} else {
		color := pal.controlStroke
		if focused {
			color = pal.accent
		} else if f.hover {
			color = pal.controlStrong
		}
		frame := win32CreateSolidBrush(color.colorRef())
		win32FrameRect(dc, client, frame)
		win32DeleteObject(hgdiobj(frame))
	}
	return true
}

func (f *fieldExtension) refresh(view *nativeView) {
	if view != nil && view.outer != 0 {
		win32InvalidateRect(view.outer, nil, false)
	}
}

func (f *fieldExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if h == f.edit {
		switch message {
		case wmSetFocus:
			f.refresh(view)
		case wmKillFocus:
			f.refresh(view)
			f.hooks.focusLost(view)
		case wmMouseMove:
			if !f.hover {
				f.hover = true
				trackMouseLeave(f.edit)
				f.refresh(view)
			}
		case wmMouseLeave:
			var cursor point
			win32GetCursorPos(&cursor)
			if win32WindowFromPoint(cursor) != view.outer {
				f.hover = false
				f.refresh(view)
			}
		case wmGetDlgCode:
			if f.multiline && lparam != 0 && (*msg)(memoryAt(lparam)).wParam == vkTab && textAreaTabMovesFocus(view) {
				*result = win32DefSubclassProc(h, message, wparam, lparam) &^ (dlgcWantAllKeys | dlgcWantTab)
				return true
			}
		case wmKeyDown:
			if wparam == vkReturn && !f.multiline {
				f.hooks.submitted(view)
				*result = 0
				return true
			}
		case wmChar:
			if (wparam == '\r' || wparam == '\n') && !f.multiline {
				*result = 0
				return true
			}
			if wparam == 1 && keyDown(vkControl) {
				// Control+A selects all, which edit controls do not do.
				win32SendMessage(f.edit, emSetSel, 0, ^uintptr(0))
				*result = 0
				return true
			}
		case wmPaint:
			if f.multiline && view.placeholder != "" && win32GetWindowTextLength(f.edit) == 0 && win32GetFocus() != f.edit {
				*result = win32DefSubclassProc(h, message, wparam, lparam)
				dc := win32GetDC(f.edit)
				var area rect
				win32SendMessage(f.edit, emGetRect, 0, uintptr(unsafe.Pointer(&area)))
				previous := win32SelectObject(dc, hgdiobj(view.font))
				win32SetTextColor(dc, view.palette().textTertiary.colorRef())
				win32SetBkMode(dc, transparent)
				win32DrawText(dc, view.placeholder, &area, dtWordBreak|dtNoPrefix|dtEditControl)
				win32SelectObject(dc, previous)
				win32ReleaseDC(f.edit, dc)
				return true
			}
		}
		return false
	}
	if h == view.outer {
		switch message {
		case wmSize:
			view.ext.layout(view)
		case wmLButtonDown:
			win32SetFocus(f.edit)
			*result = 0
			return true
		case wmMouseMove:
			if !f.hover {
				f.hover = true
				trackMouseLeave(view.outer)
				f.refresh(view)
			}
		case wmMouseLeave:
			var cursor point
			win32GetCursorPos(&cursor)
			if win32WindowFromPoint(cursor) != f.edit {
				f.hover = false
				f.refresh(view)
			}
		case wmSetCursor:
			win32SetCursor(win32LoadCursor(0, idcIBeam))
			*result = 1
			return true
		}
	}
	return false
}

func (f *fieldExtension) changed(view *nativeView) {
	view.report(textChangeEvent{})
}

func (f *fieldExtension) submitted(view *nativeView) {
	view.report(submitEvent{})
}

func (f *fieldExtension) focusLost(view *nativeView) {
}

func (f *fieldExtension) command(view *nativeView, from hwnd, code int) bool {
	if from == f.edit && code == enChange {
		f.hooks.changed(view)
		return true
	}
	return false
}

func (f *fieldExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	if f.multiline {
		return d2d1SizeF{width: 200, height: 96}, true
	}
	width := float32(160)
	if f.updown != 0 {
		width = 100
	}
	return d2d1SizeF{width: width, height: float32(view.textHeight() + 8)}, true
}

func (f *fieldExtension) themeChanged(view *nativeView) {
	applyThemeTo(f.edit, view.theme(), "")
	if f.updown != 0 {
		applyThemeTo(f.updown, view.theme(), "")
	}
	f.refresh(view)
}

// createField creates a text box with an edit control of a style. owner is the
// extension of the text box, or nil for a plain one.
func createField(window *nativeWindow, kind int, editStyle uint32, owner fieldOwner) *nativeView {
	if owner == nil {
		owner = &fieldExtension{}
	}
	field := owner.field()
	field.hooks = owner
	field.multiline = editStyle&esMultiline != 0
	view := createDrawn(window, kind, owner, false)
	field.edit = makeNative(wcEdit, "", editStyle|wsTabStop, 0, view.outer)
	view.control = field.edit
	setViewOf(field.edit, view)
	subclass(field.edit)
	view.applyFont()
	applyThemeTo(field.edit, view.theme(), "")
	win32SendMessage(field.edit, emSetMargins, ecLeftMargin|ecRightMargin, 0)
	return view
}

func (win *nativeWindow) newTextField() *nativeView {
	return createField(win, kindTextField, esAutoHScroll|esLeft, nil)
}

func (win *nativeWindow) newTextArea(text string, placeholder string, readOnly bool, wrap bool) *nativeView {
	style := uint32(esMultiline | esWantReturn | esAutoVScroll | wsVScroll)
	if !wrap {
		style |= esAutoHScroll | wsHScroll
	}
	if readOnly {
		style |= esReadOnly
	}
	view := createField(win, kindTextArea, style, nil)
	field := fieldOf(view)
	view.readOnly = readOnly
	view.placeholder = placeholder
	view.suppress++
	win32SetWindowText(field.edit, controlText(text))
	view.suppress--
	return view
}

// ---- Check boxes ----

type checkBoxExtension struct {
	baseExtension
}

func (checkBoxExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	view.report(changeEvent{})
	return true
}

func (checkBoxExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	font := view.fontOr(bodyFont())
	text := measureGDI(view.text, font, 0, 0)
	height := view.textHeight()
	width := float32(text.cx + 22)
	total := float32(21)
	if height > 17 {
		total = float32(height + 4)
	}
	// A description is a second line below the label.
	if description := checkBoxDescription(view); description != "" {
		size := measureGDI(description, font, 0, 0)
		width = max(width, float32(size.cx+22))
		total += float32(size.cy + 2)
	}
	return d2d1SizeF{width: width, height: total}, true
}

func (c checkBoxExtension) layout(view *nativeView) {
	// A check box with its label before the box draws the box at its trailing
	// edge, so it keeps its own width for the box to follow the label.
	if checkBoxLabelBefore(view) && view.width > 0 {
		size, _ := c.preferred(view)
		placeWindow(view.outer, float64(view.x), float64(view.y), min(float64(view.width), float64(size.width)+4),
			float64(view.height))
	}
}

func (win *nativeWindow) newCheckBox(text string, checked bool) *nativeView {
	view := nativeControl(win, wcButton, text, bsAutoCheckBox|wsTabStop, 0, kindCheckBox)
	if view == nil {
		return nil
	}
	view.text = text
	view.ext = checkBoxExtension{}
	state := uintptr(bstUnchecked)
	if checked {
		state = bstChecked
	}
	win32SendMessage(view.outer, bmSetCheck, state, 0)
	return view
}

// ---- Radio groups ----

// radioGroupExtension holds a radio button for each choice in a layout.
type radioGroupExtension struct {
	baseExtension
	buttons    []hwnd
	horizontal bool
	label      string
	title      hwnd
}

func (g *radioGroupExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	for i, button := range g.buttons {
		if button != from {
			continue
		}
		if view.selected != i {
			view.selected = i
			for j, other := range g.buttons {
				state := uintptr(bstUnchecked)
				if j == i {
					state = bstChecked
				}
				win32SendMessage(other, bmSetCheck, state, 0)
			}
			view.report(changeEvent{})
		}
		return true
	}
	return false
}

func radioSize(view *nativeView, button hwnd) gdiSize {
	text := measureGDI(windowText(button), view.fontOr(bodyFont()), 0, 0)
	height := view.textHeight()
	size := gdiSize{cx: text.cx + 22, cy: 21}
	if height > 17 {
		size.cy = height + 4
	}
	return size
}

func (g *radioGroupExtension) layout(view *nativeView) {
	x, y := 0.0, 0.0
	if g.title != 0 && win32IsWindowVisible(g.title) {
		height := float64(view.textHeight())
		placeWindow(g.title, 0, 0, float64(view.width), height)
		y = height + 4
	}
	for _, button := range g.buttons {
		size := radioSize(view, button)
		if g.horizontal {
			placeWindow(button, x, y, float64(size.cx), float64(size.cy))
			x += float64(size.cx + 12)
		} else {
			placeWindow(button, 0, y, float64(max(view.width, size.cx)), float64(size.cy))
			y += float64(size.cy + 4)
		}
	}
}

func (g *radioGroupExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	width, height := 0, 0
	if g.title != 0 && win32IsWindowVisible(g.title) {
		size := measureGDI(g.label, view.fontOr(bodyFont()), 0, 0)
		width = size.cx
		height = size.cy + 4
	}
	rowWidth, rowHeight := 0, 0
	for i, button := range g.buttons {
		size := radioSize(view, button)
		if g.horizontal {
			rowWidth += size.cx
			if i > 0 {
				rowWidth += 12
			}
			rowHeight = max(rowHeight, size.cy)
		} else {
			width = max(width, size.cx)
			height += size.cy
			if i > 0 {
				height += 4
			}
		}
	}
	if g.horizontal {
		width = max(width, rowWidth)
		height += rowHeight
	}
	return d2d1SizeF{width: float32(width), height: float32(height)}, true
}

func (win *nativeWindow) newRadioGroup() *nativeView {
	view := hostView(win, kindRadioGroup, &radioGroupExtension{})
	view.selected = -1
	return view
}

func radioGroupOf(view *nativeView) *radioGroupExtension {
	if view == nil || view.kind != kindRadioGroup {
		return nil
	}
	group, _ := view.ext.(*radioGroupExtension)
	return group
}

func setRadioChoices(view *nativeView, choices []ui.Choice) {
	group := radioGroupOf(view)
	if group == nil {
		return
	}
	for _, button := range group.buttons {
		win32DestroyWindow(button)
	}
	group.buttons = nil
	for i, choice := range choices {
		style := uint32(bsRadioButton | wsTabStop)
		if i == 0 {
			style |= wsGroup
		}
		button := makeNative(wcButton, choice.Text, style, 0, view.outer)
		setViewOf(button, view)
		win32SendMessage(button, wmSetFont, uintptr(fontHandle(view.fontOr(bodyFont()), win32GetDpiForWindow(button))), 0)
		applyThemeTo(button, view.theme(), "")
		win32EnableWindow(button, choice.Enabled && view.enabled)
		group.buttons = append(group.buttons, button)
	}
	if view.selected >= len(group.buttons) {
		view.selected = -1
	}
	for i, button := range group.buttons {
		state := uintptr(bstUnchecked)
		if i == view.selected {
			state = bstChecked
		}
		win32SendMessage(button, bmSetCheck, state, 0)
	}
	group.layout(view)
}

func radioGroupConfigure(view *nativeView, horizontal bool, label string) {
	group := radioGroupOf(view)
	if group == nil {
		return
	}
	group.horizontal = horizontal
	group.label = label
	if group.title == 0 {
		group.title = makeNative(wcStatic, "", ssLeft|ssNoPrefix, 0, view.outer)
		win32SendMessage(group.title, wmSetFont, uintptr(fontHandle(view.fontOr(bodyFont()), win32GetDpiForWindow(group.title))), 0)
	}
	win32SetWindowText(group.title, label)
	if label == "" {
		win32ShowWindow(group.title, swHide)
	} else {
		win32ShowWindow(group.title, swShowNA)
	}
	group.layout(view)
}

// radioButtons returns the buttons of a radio group, or nil.
func radioButtons(view *nativeView) []hwnd {
	if group := radioGroupOf(view); group != nil {
		return group.buttons
	}
	return nil
}

// ---- Selects ----

// selectExtension is a drop-down list in a layout, which centers it
// vertically, because the height of a combo box window includes its list.
type selectExtension struct {
	baseExtension
	combo       hwnd
	visibleRows int
	popupWidth  int
	placeholder string
}

// comboItem returns the text of an item of a combo box.
func comboItem(combo hwnd, index int) string {
	length := int(win32SendMessage(combo, cbGetLBTextLen, uintptr(index), 0))
	if length <= 0 {
		return ""
	}
	item := make([]uint16, length+1)
	win32SendMessage(combo, cbGetLBText, uintptr(index), uintptr(unsafe.Pointer(&item[0])))
	return goString(item)
}

func (s *selectExtension) command(view *nativeView, from hwnd, code int) bool {
	if from != s.combo {
		return false
	}
	if view.kind == kindCombo {
		// An editable combo box reports its text.
		switch code {
		case cbnSelChange:
			index := int(int32(win32SendMessage(s.combo, cbGetCurSel, 0, 0)))
			text := ""
			if index >= 0 {
				text = comboItem(s.combo, index)
			}
			view.report(textChangeEvent{text: text})
		case cbnEditChange:
			view.report(textChangeEvent{})
		case cbnKillFocus:
			view.report(submitEvent{})
		}
		return true
	}
	if code == cbnSelChange {
		view.selected = int(int32(win32SendMessage(s.combo, cbGetCurSel, 0, 0)))
		view.report(changeEvent{})
		return true
	}
	return false
}

func (s *selectExtension) layout(view *nativeView) {
	var client rect
	win32GetClientRect(view.outer, &client)
	height := int32(win32SendMessage(s.combo, cbGetItemHeight, ^uintptr(0), 0)) + 8
	scale := view.scale()
	list := int32(math.Round(float64(s.visibleRows*(view.textHeight()+4)) * scale))
	top := (client.bottom - height) / 2
	win32SetWindowPos(s.combo, 0, 0, max(top, 0), client.right, height+list, swpNoZOrder|swpNoActivate)
	if s.popupWidth > 0 {
		win32SendMessage(s.combo, cbSetDroppedWidth, uintptr(math.Round(float64(s.popupWidth)*scale)), 0)
	}
}

func (s *selectExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if h == view.outer && message == wmSize {
		s.layout(view)
	}
	if h != view.outer && h != s.combo && message == wmKeyDown && wparam == vkReturn && view.kind == kindCombo {
		view.report(submitEvent{})
		*result = 0
		return true
	}
	if h != view.outer && h != s.combo && message == wmChar && wparam == '\r' {
		*result = 0
		return true
	}
	return false
}

func (s *selectExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	font := view.fontOr(bodyFont())
	width := measureGDI(s.placeholder, font, 0, 0).cx
	count := int(int32(win32SendMessage(s.combo, cbGetCount, 0, 0)))
	for i := range count {
		width = max(width, measureGDI(comboItem(s.combo, i), font, 0, 0).cx)
	}
	return d2d1SizeF{width: float32(max(width+34, 96)), height: float32(view.textHeight() + 8)}, true
}

func (s *selectExtension) themeChanged(view *nativeView) {
	applyThemeTo(s.combo, view.theme(), "")
}

func newSelectView(window *nativeWindow, kind int, style uint32) *nativeView {
	selection := &selectExtension{visibleRows: 12}
	view := hostView(window, kind, selection)
	selection.combo = makeNative(wcComboBox, "", style|wsTabStop|wsVScroll, 0, view.outer)
	view.control = selection.combo
	setViewOf(selection.combo, view)
	subclass(selection.combo)
	info := comboBoxInfo{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if win32GetComboBoxInfo(selection.combo, &info) && info.hwndItem != 0 {
		setViewOf(info.hwndItem, view)
		subclass(info.hwndItem)
	}
	view.applyFont()
	applyThemeTo(selection.combo, view.theme(), "")
	view.selected = -1
	return view
}

func selectOf(view *nativeView) *selectExtension {
	if view == nil {
		return nil
	}
	selection, _ := view.ext.(*selectExtension)
	return selection
}

// selectCombo returns the combo box of a select, or 0.
func selectCombo(view *nativeView) hwnd {
	if selection := selectOf(view); selection != nil {
		return selection.combo
	}
	return 0
}

func selectConfigure(view *nativeView, placeholder string, popupWidth int) {
	selection := selectOf(view)
	if selection == nil {
		return
	}
	selection.placeholder = placeholder
	selection.popupWidth = popupWidth
	win32SendMessage(selection.combo, cbSetCueBanner, 0, uintptr(unsafe.Pointer(wide(placeholder))))
	selection.layout(view)
}

func (win *nativeWindow) newSelect() *nativeView {
	return newSelectView(win, kindSelect, cbsDropDownList)
}

func (view *nativeView) addSelectionItem(text string) {
	if combo := selectCombo(view); combo != 0 {
		win32SendMessage(combo, cbAddString, 0, uintptr(unsafe.Pointer(wide(text))))
	}
}

func (view *nativeView) selectionIndex() int {
	if !view.live() {
		return -1
	}
	if combo := selectCombo(view); combo != 0 {
		return int(int32(win32SendMessage(combo, cbGetCurSel, 0, 0)))
	}
	return view.selected
}

func (view *nativeView) setSelectionIndex(index int) {
	if !view.live() {
		return
	}
	view.suppress++
	view.selected = index
	if combo := selectCombo(view); combo != 0 {
		win32SendMessage(combo, cbSetCurSel, uintptr(index), 0)
	} else {
		for i, button := range radioButtons(view) {
			state := uintptr(bstUnchecked)
			if i == index {
				state = bstChecked
			}
			win32SendMessage(button, bmSetCheck, state, 0)
		}
	}
	view.suppress--
}

// ---- Sliders ----

// sliderExtension is a trackbar. The trackbar holds integer positions, so a
// value is a position times the step, or one of 10000 positions without a
// step.
type sliderExtension struct {
	baseExtension
	positions int
	frequency float64
	snap      bool
}

func (s *sliderExtension) position(view *nativeView, value float64) int {
	span := view.maximum - view.minimum
	if span <= 0 {
		return 0
	}
	fraction := (value - view.minimum) / span
	if view.inverted {
		fraction = 1 - fraction
	}
	if view.vertical {
		fraction = 1 - fraction
	}
	return int(math.Round(fraction * float64(s.positions)))
}

func (s *sliderExtension) value(view *nativeView, position int) float64 {
	fraction := 0.0
	if s.positions > 0 {
		fraction = float64(position) / float64(s.positions)
	}
	if view.vertical {
		fraction = 1 - fraction
	}
	if view.inverted {
		fraction = 1 - fraction
	}
	result := view.minimum + fraction*(view.maximum-view.minimum)
	if view.step > 0 {
		result = view.minimum + math.Round((result-view.minimum)/view.step)*view.step
	}
	return clamp(result, view.minimum, view.maximum)
}

func (s *sliderExtension) configure(view *nativeView) {
	span := view.maximum - view.minimum
	s.positions = 10000
	if view.step > 0 && span > 0 {
		s.positions = int(math.Round(span / view.step))
	}
	if s.positions <= 0 {
		s.positions = 1
	}
	bar := view.outer
	win32SendMessage(bar, tbmSetRangeMin, 0, 0)
	win32SendMessage(bar, tbmSetRangeMax, 0, uintptr(s.positions))
	line := s.positions / 100
	if view.step > 0 {
		line = 1
	}
	win32SendMessage(bar, tbmSetLineSize, 0, uintptr(line))
	win32SendMessage(bar, tbmSetPageSize, 0, uintptr(max(s.positions/10, 1)))
	win32SendMessage(bar, tbmClearTics, 1, 0)
	style := win32GetWindowLongPtr(bar, gwlStyle)
	if s.frequency > 0 && span > 0 {
		ticks := int(math.Round(span / s.frequency))
		if ticks > 0 && ticks <= 200 {
			win32SetWindowLongPtr(bar, gwlStyle, (style&^tbsNoTicks)|tbsAutoTicks)
			win32SendMessage(bar, tbmSetTicFreq, uintptr(math.Round(float64(s.positions)/float64(ticks))), 0)
		}
	} else {
		win32SetWindowLongPtr(bar, gwlStyle, (style&^tbsAutoTicks)|tbsNoTicks)
	}
	win32SendMessage(bar, tbmSetPos, 1, uintptr(s.position(view, view.value)))
	win32InvalidateRect(bar, nil, true)
}

func (s *sliderExtension) scroll(view *nativeView, from hwnd, code int) bool {
	if code == tbEndTrack {
		return true
	}
	position := int(int32(win32SendMessage(view.outer, tbmGetPos, 0, 0)))
	next := s.value(view, position)
	if s.snap && s.frequency > 0 {
		next = view.minimum + math.Round((next-view.minimum)/s.frequency)*s.frequency
		next = clamp(next, view.minimum, view.maximum)
		win32SendMessage(view.outer, tbmSetPos, 1, uintptr(s.position(view, next)))
	}
	if next != view.value {
		view.value = next
		view.report(changeEvent{})
	}
	return true
}

func (s *sliderExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	thickness := float32(26)
	if s.frequency > 0 {
		thickness = 32
	}
	if view.vertical {
		return d2d1SizeF{width: thickness, height: 160}, true
	}
	return d2d1SizeF{width: 160, height: thickness}, true
}

func sliderOf(view *nativeView) *sliderExtension {
	if view == nil {
		return nil
	}
	slider, _ := view.ext.(*sliderExtension)
	return slider
}

func (win *nativeWindow) newSlider(minimum float64, maximum float64, value float64, step float64) *nativeView {
	view := nativeControl(win, trackbarClass, "", tbsHorz|tbsNoTicks|tbsBoth|wsTabStop, 0, kindSlider)
	if view == nil {
		return nil
	}
	slider := &sliderExtension{}
	view.ext = slider
	view.minimum = minimum
	view.maximum = max(maximum, minimum)
	view.step = max(step, 0)
	view.value = clamp(value, view.minimum, view.maximum)
	view.suppress++
	slider.configure(view)
	view.suppress--
	return view
}

func (view *nativeView) sliderValue() float64 {
	if view.live() {
		return view.value
	}
	return 0
}

func (view *nativeView) setSliderValue(value float64) {
	if !view.live() {
		return
	}
	view.value = clamp(value, view.minimum, view.maximum)
	if slider := sliderOf(view); slider != nil {
		view.suppress++
		win32SendMessage(view.outer, tbmSetPos, 1, uintptr(slider.position(view, view.value)))
		view.suppress--
	}
}

// ---- Progress ----

type progressExtension struct {
	baseExtension
	indeterminate bool
}

func (progressExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	if view.vertical {
		return d2d1SizeF{width: 6, height: 160}, true
	}
	return d2d1SizeF{width: 160, height: 6}, true
}

func (win *nativeWindow) newProgressBar(value float64) *nativeView {
	view := nativeControl(win, progressClass, "", 0, 0, kindProgress)
	if view == nil {
		return nil
	}
	view.ext = &progressExtension{}
	win32SendMessage(view.outer, pbmSetRange32, 0, 10000)
	view.value = clamp(value, 0, 1)
	win32SendMessage(view.outer, pbmSetPos, uintptr(math.Round(view.value*10000)), 0)
	return view
}

func (view *nativeView) progressFraction() float64 {
	if view.live() && view.kind == kindProgress {
		return view.value
	}
	return 0
}

func (view *nativeView) setProgressValue(value float64) {
	if view.live() && view.kind == kindProgress {
		view.value = clamp(value, 0, 1)
		win32SendMessage(view.target(), pbmSetPos, uintptr(math.Round(view.value*10000)), 0)
	}
}

// ---- Activity indicators ----
//
// Windows has no control for an activity indicator, so the backend draws a
// rotating arc like the one of Windows.

type activityExtension struct {
	baseExtension
	timer uintptr
	angle float32
	size  int
}

func (a *activityExtension) free() {
	cancelTimer(a.timer)
}

func (a *activityExtension) paint(view *nativeView, canvas *painter) {
	if !view.running {
		return
	}
	diameter := min(canvas.width, canvas.height)
	thickness := max(diameter/10, 1.5)
	radius := diameter/2 - thickness
	cx, cy := canvas.width/2, canvas.height/2
	sweep := float32(math.Pi * (0.6 + 0.4*math.Sin(float64(a.angle)*0.7)))
	canvas.arc(cx, cy, radius, a.angle, sweep, view.palette().accent, thickness)
}

func (a *activityExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	side := float32(20)
	switch a.size {
	case 0:
		side = 16
	case 2:
		side = 32
	}
	return d2d1SizeF{width: side, height: side}, true
}

func updateActivity(view *nativeView) {
	activity, ok := view.ext.(*activityExtension)
	if !ok {
		return
	}
	if view.running && activity.timer == 0 {
		activity.timer = startTimer(16, func() bool {
			if !view.live() {
				return false
			}
			activity.angle += 0.12
			if win32IsWindowVisible(view.outer) {
				win32InvalidateRect(view.outer, nil, false)
			}
			return true
		})
	} else if !view.running && activity.timer != 0 {
		cancelTimer(activity.timer)
		activity.timer = 0
	}
	win32InvalidateRect(view.outer, nil, false)
}

func setActivitySize(view *nativeView, size int) {
	if view != nil && view.kind == kindActivity {
		if activity, ok := view.ext.(*activityExtension); ok {
			activity.size = size
		}
	}
}

// newActivityView creates a small activity indicator in a window.
func newActivityView(parent hwnd) *nativeView {
	view := createDrawn(nil, kindActivity, &activityExtension{size: 0}, false)
	reparent(view.outer, parent)
	win32EnableWindow(view.outer, false)
	return view
}

func (win *nativeWindow) newActivityIndicator(running bool) *nativeView {
	view := createDrawn(win, kindActivity, &activityExtension{size: 1}, false)
	view.running = running
	updateActivity(view)
	return view
}

func (view *nativeView) isActivityRunning() bool {
	return view.live() && view.kind == kindActivity && view.running
}

func (view *nativeView) setActivityRunning(running bool) {
	if view.live() && view.kind == kindActivity {
		view.running = running
		updateActivity(view)
	}
}

// ---- Separators ----

type separatorExtension struct {
	baseExtension
	label     string
	inset     int
	thickness int
}

func (s *separatorExtension) paint(view *nativeView, canvas *painter) {
	pal := view.palette()
	t := float32(s.thickness)
	inset := float32(s.inset)
	if s.label == "" {
		if view.vertical {
			x := float32(math.Floor(float64(canvas.width / 2)))
			canvas.fillRect(x, inset, t, canvas.height-2*inset, pal.divider)
		} else {
			y := float32(math.Floor(float64(canvas.height / 2)))
			canvas.fillRect(inset, y, canvas.width-2*inset, t, pal.divider)
		}
		return
	}
	font := typeRamp(1)
	text := measureText(s.label, font, 0, 0)
	if view.vertical {
		x := float32(math.Floor(float64(canvas.width / 2)))
		middle := canvas.height / 2
		canvas.fillRect(x, inset, t, middle-text.height/2-4-inset, pal.divider)
		canvas.fillRect(x, middle+text.height/2+4, t, middle-text.height/2-4-inset, pal.divider)
		canvas.text(s.label, font, 0, middle-text.height/2, canvas.width, text.height, pal.textSecondary, 1, 1,
			false, true)
	} else {
		y := float32(math.Floor(float64(canvas.height / 2)))
		middle := canvas.width / 2
		canvas.fillRect(inset, y, middle-text.width/2-6-inset, t, pal.divider)
		canvas.fillRect(middle+text.width/2+6, y, middle-text.width/2-6-inset, t, pal.divider)
		canvas.text(s.label, font, middle-text.width/2-1, 0, text.width+2, canvas.height, pal.textSecondary, 1, 1,
			false, true)
	}
}

func (s *separatorExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	t := float32(s.thickness)
	if s.label != "" {
		text := measureText(s.label, typeRamp(1), 0, 0)
		if view.vertical {
			return d2d1SizeF{width: text.width, height: text.height + 24}, true
		}
		return d2d1SizeF{width: text.width + 24, height: text.height}, true
	}
	if view.vertical {
		return d2d1SizeF{width: t}, true
	}
	return d2d1SizeF{height: t}, true
}

func separatorOf(view *nativeView) *separatorExtension {
	if view == nil || view.kind != kindSeparator {
		return nil
	}
	separator, _ := view.ext.(*separatorExtension)
	return separator
}

func separatorConfigure(view *nativeView, vertical bool, label string, inset, thickness int) {
	separator := separatorOf(view)
	if separator == nil {
		return
	}
	view.vertical = vertical
	separator.label = label
	separator.inset = max(inset, 0)
	separator.thickness = thickness
	if thickness <= 0 {
		separator.thickness = 1
	}
	win32InvalidateRect(view.outer, nil, true)
}

// separatorLabel returns the label of a separator.
func separatorLabel(view *nativeView) string {
	if separator := separatorOf(view); separator != nil {
		return separator.label
	}
	return ""
}

func (win *nativeWindow) newSeparator(direction int) *nativeView {
	view := createDrawn(win, kindSeparator, &separatorExtension{thickness: 1}, false)
	view.vertical = direction == 1
	return view
}

// ---- Images ----

type imageExtension struct {
	baseExtension
	pixels        []byte
	width, height int
	bitmap        comObject
	owner         comObject
}

func (i *imageExtension) free() {
	win32UnknownRelease(i.bitmap)
	i.bitmap = 0
}

// Anchor order matches ui.Anchor: center, then clockwise from top-left.
var (
	anchorHorizontal = [9]float64{0.5, 0, 0.5, 1, 0, 1, 0, 0.5, 1}
	anchorVertical   = [9]float64{0.5, 0, 0, 0, 0.5, 0.5, 1, 1, 1}
)

func (i *imageExtension) paint(view *nativeView, canvas *painter) {
	if len(i.pixels) == 0 || i.width <= 0 || i.height <= 0 {
		return
	}
	if i.bitmap == 0 || i.owner != canvas.target {
		win32UnknownRelease(i.bitmap)
		i.bitmap = bitmapFromRGBA(canvas.target, i.pixels, i.width, i.height, i.width*4)
		i.owner = canvas.target
	}
	sourceScale := view.sourceScale
	if sourceScale <= 0 {
		sourceScale = 1
	}
	sourceWidth, sourceHeight := float64(i.width)/sourceScale, float64(i.height)/sourceScale
	w, h, x, y := float64(canvas.width), float64(canvas.height), 0.0, 0.0
	if view.scaling != 2 {
		sx, sy := float64(canvas.width)/sourceWidth, float64(canvas.height)/sourceHeight
		scale := min(sx, sy)
		if view.scaling == 1 {
			scale = max(sx, sy)
		}
		w = sourceWidth * scale
		h = sourceHeight * scale
		anchor := view.alignment
		if anchor < 0 || anchor >= 9 {
			anchor = 0
		}
		x = (float64(canvas.width) - w) * anchorHorizontal[anchor]
		y = (float64(canvas.height) - h) * anchorVertical[anchor]
	}
	canvas.clip(0, 0, canvas.width, canvas.height)
	canvas.bitmap(i.bitmap, float32(x), float32(y), float32(w), float32(h), 1, view.nearest)
	canvas.unclip()
}

func (i *imageExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	scale := view.sourceScale
	if scale <= 0 {
		scale = 1
	}
	return d2d1SizeF{width: float32(float64(i.width) / scale), height: float32(float64(i.height) / scale)}, true
}

func (win *nativeWindow) newImageView(scaling int) *nativeView {
	view := createDrawn(win, kindImage, &imageExtension{}, false)
	view.scaling = scaling
	return view
}

func (view *nativeView) setImagePixels(pixels []byte, width int, height int, stride int) {
	if !view.live() || view.kind != kindImage || len(pixels) == 0 || width <= 0 || height <= 0 {
		return
	}
	image, ok := view.ext.(*imageExtension)
	if !ok {
		return
	}
	image.width = width
	image.height = height
	image.pixels = make([]byte, width*height*4)
	for y := range height {
		copy(image.pixels[y*width*4:(y+1)*width*4], pixels[y*stride:])
	}
	win32UnknownRelease(image.bitmap)
	image.bitmap = 0
	win32InvalidateRect(view.outer, nil, false)
}

func (view *nativeView) setImageScaling(scaling int) {
	if view.live() && view.kind == kindImage {
		view.scaling = scaling
		win32InvalidateRect(view.outer, nil, false)
	}
}

// ---- Links ----

// linkMarkup returns text as the markup of a SysLink control.
func linkMarkup(text string) string {
	replacer := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	return "<a>" + replacer.Replace(text) + "</a>"
}

type linkExtension struct {
	baseExtension
}

func (linkExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.code == nmClick || header.code == nmReturn {
		view.report(clickEvent{})
		*result = 0
		return true
	}
	return false
}

func (linkExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	text := measureGDI(view.text, view.fontOr(bodyFont()), 0, 0)
	return d2d1SizeF{width: float32(text.cx + 2), height: float32(text.cy + 2)}, true
}

func (win *nativeWindow) newLink(text string, url string) *nativeView {
	view := nativeControl(win, wcLink, linkMarkup(text), wsTabStop|lwsTransparent, 0, kindLink)
	if view == nil {
		return nil
	}
	view.text = text
	view.url = url
	view.ext = linkExtension{}
	return view
}

func (view *nativeView) linkURL() string {
	if view.live() {
		return view.url
	}
	return ""
}

func (view *nativeView) setLinkURL(url string) {
	if view.live() {
		view.url = url
	}
}

// ---- Scroll views ----
//
// A scroll view is a layout with scroll bars. Its content is a layout that
// moves under it as it scrolls.

type scrollExtension struct {
	baseExtension
	contentWidth, contentHeight float64
	top, right, bottom, left    float64
	x, y                        float64
	zoom                        float64
	policy                      int
	// The wheel glides to a target offset, as scrolling in Windows does.
	targetX, targetY float64
	glide            uintptr
}

func (s *scrollExtension) free() {
	s.stopGlide()
}

func (s *scrollExtension) stopGlide() {
	if s.glide != 0 {
		cancelTimer(s.glide)
		s.glide = 0
	}
}

func (s *scrollExtension) glideTo(view *nativeView, nx, ny float64) {
	width, height := s.viewport(view)
	maxX := s.contentWidth*s.zoom + s.left + s.right - width
	maxY := s.contentHeight*s.zoom + s.top + s.bottom - height
	s.targetX = clamp(nx, 0, max(maxX, 0))
	s.targetY = clamp(ny, 0, max(maxY, 0))
	if s.glide != 0 {
		return
	}
	animations := int32(1)
	win32SystemParametersInfo(spiGetClientAreaAnimation, 0, unsafe.Pointer(&animations), 0)
	if animations == 0 {
		s.scrollTo(view, s.targetX, s.targetY)
		return
	}
	s.glide = startTimer(10, func() bool {
		if !view.live() {
			return false
		}
		dx, dy := s.targetX-s.x, s.targetY-s.y
		done := math.Abs(dx) < 0.5 && math.Abs(dy) < 0.5
		if done {
			s.x, s.y = s.targetX, s.targetY
		} else {
			s.x += dx * 0.3
			s.y += dy * 0.3
		}
		s.update(view)
		if done {
			s.glide = 0
		}
		return !done
	})
}

func (s *scrollExtension) viewport(view *nativeView) (float64, float64) {
	size := clientSize(view.outer)
	return float64(size.width), float64(size.height)
}

func (s *scrollExtension) clampOffset(view *nativeView) {
	width, height := s.viewport(view)
	maxX := s.contentWidth*s.zoom + s.left + s.right - width
	maxY := s.contentHeight*s.zoom + s.top + s.bottom - height
	s.x = clamp(s.x, 0, max(maxX, 0))
	s.y = clamp(s.y, 0, max(maxY, 0))
}

func (s *scrollExtension) update(view *nativeView) {
	width, height := s.viewport(view)
	horizontal := view.axes == 1 || view.axes == 2
	vertical := view.axes == 0 || view.axes == 2
	scale := view.scale()
	s.clampOffset(view)
	info := scrollInfo{fMask: sifRange | sifPage | sifPos}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if s.policy == 2 {
		info.fMask |= sifDisableNoScroll
	}
	if horizontal {
		info.nMax = int32(math.Round((s.contentWidth*s.zoom+s.left+s.right)*scale)) - 1
		info.nPage = uint32(math.Round(width * scale))
	}
	info.nPos = int32(math.Round(s.x * scale))
	if s.policy != 1 {
		win32SetScrollInfo(view.outer, sbHorz, &info, true)
	}
	info.nMax, info.nPage = 0, 0
	if vertical {
		info.nMax = int32(math.Round((s.contentHeight*s.zoom+s.top+s.bottom)*scale)) - 1
		info.nPage = uint32(math.Round(height * scale))
	}
	info.nPos = int32(math.Round(s.y * scale))
	if s.policy != 1 {
		win32SetScrollInfo(view.outer, sbVert, &info, true)
	} else {
		win32ShowScrollBar(view.outer, sbBoth, false)
	}
	placeWindow(view.body, s.left-s.x, s.top-s.y, s.contentWidth*s.zoom, s.contentHeight*s.zoom)
	setLayoutZoom(view.body, s.zoom)
}

func (s *scrollExtension) scrollTo(view *nativeView, nx, ny float64) {
	s.stopGlide()
	s.x = nx
	s.y = ny
	s.update(view)
}

func (s *scrollExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if h != view.outer {
		return false
	}
	switch message {
	case wmSize:
		s.update(view)
		return false
	case wmHScroll, wmVScroll:
		if lparam != 0 {
			return false
		}
		bar := int32(sbVert)
		if message == wmHScroll {
			bar = sbHorz
		}
		info := scrollInfo{fMask: sifAll}
		info.cbSize = uint32(unsafe.Sizeof(info))
		win32GetScrollInfo(view.outer, bar, &info)
		scale := view.scale()
		position := s.y * scale
		if bar == sbHorz {
			position = s.x * scale
		}
		line := 40 * scale
		switch loword(wparam) {
		case sbLineUp:
			position -= line
		case sbLineDown:
			position += line
		case sbPageUp:
			position -= float64(info.nPage)
		case sbPageDown:
			position += float64(info.nPage)
		case sbThumbTrack, sbThumbPosition:
			position = float64(info.nTrackPos)
		case sbTop:
			position = 0
		case sbBottom:
			position = float64(info.nMax)
		}
		s.stopGlide()
		if bar == sbHorz {
			s.x = position / scale
		} else {
			s.y = position / scale
		}
		s.update(view)
		*result = 0
		return true
	case wmMouseWheel, wmMouseHWheel:
		delta := float64(int16(hiword(wparam))) / wheelDelta
		lines := uint32(3)
		win32SystemParametersInfo(spiGetWheelScrollLines, 0, unsafe.Pointer(&lines), 0)
		if lines == 0 {
			lines = 3
		}
		distance := delta * float64(lines) * 16
		horizontal := message == wmMouseHWheel || view.axes == 1 || keyDown(vkShift)
		nx, ny := s.x, s.y
		if s.glide != 0 {
			nx, ny = s.targetX, s.targetY
		}
		if horizontal {
			if message == wmMouseHWheel {
				nx += distance
			} else {
				nx -= distance
			}
		} else {
			ny -= distance
		}
		s.glideTo(view, nx, ny)
		*result = 0
		return true
	default:
		return false
	}
}

func scrollOf(view *nativeView) *scrollExtension {
	if view == nil || view.kind != kindScroll {
		return nil
	}
	scroll, _ := view.ext.(*scrollExtension)
	return scroll
}

func scrollOffsetOf(view *nativeView) (float64, float64) {
	if scroll := scrollOf(view); scroll != nil {
		return scroll.x, scroll.y
	}
	return 0, 0
}

func setScrollOffset(view *nativeView, x, y float64) {
	if scroll := scrollOf(view); scroll != nil {
		scroll.scrollTo(view, x, y)
	}
}

func viewportSizeOf(view *nativeView) (float64, float64) {
	if scroll := scrollOf(view); scroll != nil {
		return scroll.viewport(view)
	}
	return 0, 0
}

func setScrollInsets(view *nativeView, top, right, bottom, left int) {
	if scroll := scrollOf(view); scroll != nil {
		scroll.top = float64(max(top, 0))
		scroll.right = float64(max(right, 0))
		scroll.bottom = float64(max(bottom, 0))
		scroll.left = float64(max(left, 0))
		scroll.update(view)
	}
}

func setScrollPolicy(view *nativeView, policy int) {
	if scroll := scrollOf(view); scroll != nil {
		scroll.policy = policy
		scroll.update(view)
	}
}

func (win *nativeWindow) newScrollView(axes int) *nativeView {
	scroll := &scrollExtension{zoom: 1}
	host := newLayout(parking())
	style := win32GetWindowLongPtr(host, gwlStyle)
	if axes == 1 || axes == 2 {
		style |= wsHScroll
	}
	if axes == 0 || axes == 2 {
		style |= wsVScroll
	}
	win32SetWindowLongPtr(host, gwlStyle, style)
	view := newView(host, 0, kindScroll, win)
	view.ext = scroll
	view.axes = axes
	view.body = newLayout(host)
	applyThemeTo(host, view.theme(), "")
	scroll.update(view)
	return view
}

func (view *nativeView) scrollViewportSize() (int, int) {
	w, h := viewportSizeOf(view)
	return int(math.Floor(w)), int(math.Floor(h))
}

func (view *nativeView) setScrollContentSize(width int, height int) {
	if scroll := scrollOf(view); scroll != nil {
		scroll.contentWidth = float64(max(width, 0))
		scroll.contentHeight = float64(max(height, 0))
		scroll.update(view)
	}
}

func (view *nativeView) scrollOffset() (int, int) {
	x, y := scrollOffsetOf(view)
	return int(math.Round(x)), int(math.Round(y))
}

// ---- Canvas ----

type canvasExtension struct {
	baseExtension
	// owner receives the events of the canvas until the Go layer closes it.
	owner                         *canvas
	lastX, lastY                  float64
	haveLast                      bool
	inside                        bool
	shown                         bool
	reportedWidth, reportedHeight int32
	reportedScale                 float64
	frames                        uintptr
	highSurrogate                 uint32
}

func (c *canvasExtension) free() {
	cancelTimer(c.frames)
}

func canvasOf(view *nativeView) *canvasExtension {
	if view == nil || view.kind != kindCanvas {
		return nil
	}
	canvas, _ := view.ext.(*canvasExtension)
	return canvas
}

func canvasSend(view *nativeView, event any) {
	if c := canvasOf(view); c != nil && c.owner != nil {
		c.owner.receiveEvent(event)
	}
}

func canvasReportSurface(view *nativeView, kind int) {
	data := canvasOf(view)
	var client rect
	win32GetClientRect(view.outer, &client)
	scale := view.scale()
	if data != nil && kind == 0 && client.right == data.reportedWidth && client.bottom == data.reportedHeight &&
		scale == data.reportedScale {
		return
	}
	if data != nil {
		data.reportedWidth = client.right
		data.reportedHeight = client.bottom
		data.reportedScale = scale
	}
	kinds := [...]ui.SurfaceEventKind{ui.SurfaceResized, ui.SurfaceScaleChanged, ui.SurfaceColorSpaceChanged}
	canvasSend(view, ui.SurfaceEvent{
		Kind:            kinds[kind],
		FramebufferSize: ui.Size{Width: int(client.right), Height: int(client.bottom)},
		ScaleFactor:     scale,
		ColorSpace:      ui.ColorSpaceSRGB,
	})
}

func canvasButton(view *nativeView, button, action, mods int) {
	c := canvasOf(view)
	if c == nil || c.owner == nil || button < 0 || button >= pointerButtonCount {
		return
	}
	mask := 1 << button
	if action == pointerPressed {
		view.canvasButtons |= mask
	} else {
		if view.canvasButtons&mask == 0 {
			return
		}
		view.canvasButtons &^= mask
	}
	c.owner.dispatchNativeButton(button, action, mods)
}

func canvasShown(view *nativeView) {
	data := canvasOf(view)
	if data == nil {
		return
	}
	shown := win32IsWindowVisible(view.outer) && windowOf(view.outer) != nil
	if shown != data.shown {
		data.shown = shown
		canvasSend(view, occlusionEvent{occluded: !shown})
	}
}

// canvasButtonOf returns the button of a mouse message.
func canvasButtonOf(message uint32, wparam uintptr) int {
	switch message {
	case wmLButtonDown, wmLButtonDblClk, wmLButtonUp:
		return pointerButtonLeft
	case wmRButtonDown, wmRButtonDblClk, wmRButtonUp:
		return pointerButtonRight
	case wmMButtonDown, wmMButtonDblClk, wmMButtonUp:
		return pointerButtonMid
	}
	if hiword(wparam) == xbutton1 {
		return 3
	}
	return 4
}

func (c *canvasExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	switch message {
	case wmEraseBkgnd:
		*result = 1
		return true
	case wmPaint:
		var paint paintStruct
		win32BeginPaint(view.outer, &paint)
		win32EndPaint(view.outer, &paint)
		*result = 0
		return true
	case wmSize:
		canvasReportSurface(view, 0)
		return false
	case wmShowWindow:
		later(func() {
			if view.live() {
				canvasShown(view)
			}
		})
		return false
	case wmDpiChangedAfterParent:
		canvasReportSurface(view, 1)
		return false
	case wmGetDlgCode:
		*result = dlgcWantAllKeys | dlgcWantChars | dlgcWantArrows | dlgcWantTab
		return true
	case wmLButtonDown, wmRButtonDown, wmMButtonDown, wmXButtonDown,
		wmLButtonDblClk, wmRButtonDblClk, wmMButtonDblClk, wmXButtonDblClk:
		win32SetFocus(view.outer)
		win32SetCapture(view.outer)
		canvasButton(view, canvasButtonOf(message, wparam), pointerPressed, modifiers())
		*result = 0
		if message == wmXButtonDown || message == wmXButtonDblClk {
			*result = 1
		}
		return true
	case wmLButtonUp, wmRButtonUp, wmMButtonUp, wmXButtonUp:
		canvasButton(view, canvasButtonOf(message, wparam), pointerReleased, modifiers())
		if view.canvasButtons == 0 && win32GetCapture() == view.outer && !view.hasCapture() {
			win32ReleaseCapture()
		}
		*result = 0
		if message == wmXButtonUp {
			*result = 1
		}
		return true
	case wmCaptureChanged:
		if hwnd(lparam) != view.outer {
			for button := range pointerButtonCount {
				if view.canvasButtons&(1<<button) != 0 && c.owner != nil {
					c.owner.dispatchNativeButton(button, pointerReleased, 0)
				}
			}
			view.canvasButtons = 0
		}
		return false
	case wmMouseMove:
		scale := view.scale()
		x, y := float64(signedLoword(lparam))/scale, float64(signedHiword(lparam))/scale
		if !c.inside {
			c.inside = true
			trackMouseLeave(view.outer)
			canvasSend(view, pointerEnterEvent{crossing: ui.PointerCrossingEvent{X: x, Y: y}})
		}
		var dx, dy float64
		if c.haveLast {
			dx, dy = x-c.lastX, y-c.lastY
		}
		c.lastX, c.lastY = x, y
		c.haveLast = true
		canvasSend(view, ui.PointerMoveEvent{
			X:         x,
			Y:         y,
			DeltaX:    dx,
			DeltaY:    dy,
			Modifiers: keyboard.ModifierKey(modifiers()),
		})
		return false
	case wmMouseLeave:
		c.inside = false
		c.haveLast = false
		canvasSend(view, pointerLeaveEvent{crossing: ui.PointerCrossingEvent{X: c.lastX, Y: c.lastY}})
		return false
	case wmMouseWheel, wmMouseHWheel:
		delta := float64(int16(hiword(wparam))) / wheelDelta
		if c.owner != nil {
			if message == wmMouseHWheel {
				c.owner.dispatchNativeScroll(-delta, 0)
			} else {
				c.owner.dispatchNativeScroll(0, delta)
			}
		}
		*result = 0
		return true
	case wmKeyDown, wmSysKeyDown, wmKeyUp, wmSysKeyUp:
		if key := keyFromVirtualKey(wparam, lparam); key >= 0 {
			action := keyPressed
			if message == wmKeyUp || message == wmSysKeyUp {
				action = keyReleased
			} else if lparam&(1<<30) != 0 {
				action = keyRepeat
			}
			canvasSend(view, ui.KeyEvent{
				Key:       keyboard.Key(key),
				Scancode:  int((lparam >> 16) & 0xff),
				Action:    keyboard.KeyAction(action),
				Modifiers: keyboard.ModifierKey(modifiers()),
				IsRepeat:  action == keyRepeat,
			})
		}
		return message == wmKeyDown || message == wmKeyUp
	case wmChar:
		unit := uint32(wparam)
		if unit >= 0xd800 && unit <= 0xdbff {
			c.highSurrogate = unit
			return true
		}
		codepoint := unit
		if unit >= 0xdc00 && unit <= 0xdfff && c.highSurrogate != 0 {
			codepoint = 0x10000 + ((c.highSurrogate - 0xd800) << 10) + (unit - 0xdc00)
		}
		c.highSurrogate = 0
		if codepoint >= 0x20 && codepoint != 0x7f && modifiers()&(modControl|modAlt|modSuper) == 0 {
			canvasSend(view, charEvent{char: rune(codepoint)})
		}
		*result = 0
		return true
	default:
		return false
	}
}

func canvasFrames(view *nativeView) {
	data := canvasOf(view)
	if data == nil {
		return
	}
	wanted := view.frames && data.owner != nil
	if wanted && data.frames == 0 {
		data.frames = startTimer(1, func() bool {
			if !view.live() || !view.frames {
				return false
			}
			period := 1.0 / 60
			monitor := win32MonitorFromWindow(view.outer, monitorDefaultToNearest)
			info := monitorInfoEx{}
			info.cbSize = uint32(unsafe.Sizeof(info))
			mode := devMode{}
			mode.dmSize = uint16(unsafe.Sizeof(mode))
			if win32GetMonitorInfo(monitor, unsafe.Pointer(&info)) &&
				win32EnumDisplaySettings(&info.szDevice[0], enumCurrentSettings, &mode) && mode.dmDisplayFrequency > 1 {
				period = 1 / float64(mode.dmDisplayFrequency)
			}
			canvasSend(view, frameEvent{interval: time.Duration(period * float64(time.Second))})
			return true
		})
	} else if !wanted && data.frames != 0 {
		cancelTimer(data.frames)
		data.frames = 0
	}
}

func (win *nativeWindow) newCanvas(owner *canvas) *nativeView {
	return createDrawn(win, kindCanvas, &canvasExtension{owner: owner, reportedWidth: -1, reportedHeight: -1},
		true)
}

func (view *nativeView) canvasPointerPosition() (float64, float64) {
	if !view.live() || view.outer == 0 {
		return 0, 0
	}
	var cursor point
	if !win32GetCursorPos(&cursor) {
		return 0, 0
	}
	win32ScreenToClient(view.outer, &cursor)
	scale := view.scale()
	return float64(cursor.x) / scale, float64(cursor.y) / scale
}

func (view *nativeView) setCanvasFrames(start bool) bool {
	if !view.live() || view.kind != kindCanvas {
		return false
	}
	view.frames = start
	canvasFrames(view)
	return true
}

// ---- Sections ----

const sectionHeaderHeight = 28

// sectionExtension is a section: a header that Windows has no control for,
// and a body.
type sectionExtension struct {
	baseExtension
	header     hwnd
	headerView *nativeView
	subtitle   string
	badge      string
	icon       *iconImage
	expanded   bool
}

func (s *sectionExtension) free() {
	if s.icon != nil {
		s.icon = nil
	}
}

func (s *sectionExtension) layout(view *nativeView) {
	placeWindow(s.header, 0, 0, float64(view.width), sectionHeaderHeight)
	placeWindow(view.body, 0, sectionHeaderHeight, float64(view.width), max(float64(view.height)-sectionHeaderHeight, 0))
	if s.expanded {
		win32ShowWindow(view.body, swShowNA)
	} else {
		win32ShowWindow(view.body, swHide)
	}
}

// sectionHeaderExtension draws the header of a section: a chevron, an icon, a
// title, a subtitle, and a badge.
type sectionHeaderExtension struct {
	baseExtension
	section *nativeView
}

func (h *sectionHeaderExtension) data() *sectionExtension {
	data, _ := h.section.ext.(*sectionExtension)
	return data
}

func (h *sectionHeaderExtension) paint(view *nativeView, canvas *painter) {
	data := h.data()
	if data == nil {
		return
	}
	section := h.section
	pal := section.palette()
	if view.hover || view.pressed {
		fill := pal.subtle
		if view.pressed {
			fill = pal.subtlePressed
		}
		canvas.fillRound(0, 0, canvas.width, canvas.height, 4, fill)
	}
	x := float32(6)
	chevron := ""
	if data.expanded {
		chevron = ""
	}
	canvas.glyph(chevron, 10, x, 0, 12, canvas.height, pal.text)
	x += 18
	if data.icon != nil {
		drawIcon(canvas, data.icon, x, (canvas.height-16)/2, 16, pal.text)
		x += 22
	}
	font := section.fontOr(typeRamp(2))
	title := measureText(section.text, font, 0, 0)
	textColor := pal.text
	if !section.enabled {
		textColor = pal.textDisabled
	}
	canvas.text(section.text, font, x, 0, canvas.width-x, canvas.height, section.textColorOr(textColor), 0, 1,
		false, true)
	x += title.width + 8
	if data.subtitle != "" {
		caption := typeRamp(1)
		size := measureText(data.subtitle, caption, 0, 0)
		canvas.text(data.subtitle, caption, x, 0, canvas.width-x, canvas.height, pal.textSecondary, 0, 1, false, true)
		x += size.width + 8
	}
	if data.badge != "" {
		caption := typeRamp(1)
		size := measureText(data.badge, caption, 0, 0)
		right := max(canvas.width-8-size.width, x)
		canvas.text(data.badge, caption, right, 0, size.width+2, canvas.height, pal.textSecondary, 0, 1, false, true)
	}
	if view.focused && view.keyboardFocus {
		canvas.strokeRound(1, 1, canvas.width-2, canvas.height-2, 4, pal.focus, 1.5)
	}
}

func toggleSection(view *nativeView, expanded, report bool) {
	data, ok := view.ext.(*sectionExtension)
	if !ok {
		return
	}
	data.expanded = expanded
	data.layout(view)
	win32InvalidateRect(data.header, nil, false)
	if report {
		view.report(changeEvent{})
	}
}

func (h *sectionHeaderExtension) message(view *nativeView, window hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	data := h.data()
	if data == nil {
		return false
	}
	switch message {
	case wmLButtonDown:
		view.pressed = true
		win32SetCapture(view.outer)
		win32InvalidateRect(view.outer, nil, false)
		return false
	case wmLButtonUp:
		if view.pressed {
			view.pressed = false
			win32ReleaseCapture()
			win32InvalidateRect(view.outer, nil, false)
			if view.hover && h.section.enabled {
				toggleSection(h.section, !data.expanded, true)
			}
		}
		*result = 0
		return true
	case wmKeyDown:
		switch wparam {
		case vkSpace, vkReturn:
			toggleSection(h.section, !data.expanded, true)
			*result = 0
			return true
		case vkLeft, vkRight:
			expand := wparam == vkRight
			if expand != data.expanded {
				toggleSection(h.section, expand, true)
			}
			*result = 0
			return true
		}
	}
	return false
}

func (win *nativeWindow) newSection(title string, expanded bool) *nativeView {
	data := &sectionExtension{expanded: expanded}
	view := hostView(win, kindSection, data)
	view.text = title
	header := createDrawn(win, kindGeneric, &sectionHeaderExtension{section: view}, true)
	reparent(header.outer, view.outer)
	// The header belongs to the section, which reports its focus and its
	// accessibility.
	header.owner = view
	data.header = header.outer
	data.headerView = header
	view.control = header.outer
	view.body = newLayout(view.outer)
	data.layout(view)
	return view
}

func sectionOf(view *nativeView) *sectionExtension {
	if view == nil || view.kind != kindSection {
		return nil
	}
	section, _ := view.ext.(*sectionExtension)
	return section
}

func (view *nativeView) isSectionExpanded() bool {
	section := sectionOf(view)
	return section != nil && section.expanded
}

func (view *nativeView) setSectionAnimated(animated bool) {
	if view.live() && view.kind == kindSection {
		view.animated = animated
	}
}

func (view *nativeView) configureSection(subtitle string, icon *iconImage, badge string) {
	section := sectionOf(view)
	if section == nil {
		return
	}
	section.subtitle = subtitle
	section.badge = badge
	section.icon = icon
	win32InvalidateRect(section.header, nil, false)
}

func (view *nativeView) setSectionExpanded(expanded bool) {
	if view.live() && view.kind == kindSection {
		toggleSection(view, expanded, false)
	}
}

// ---- Generic view functions ----

func (view *nativeView) clearAction() {
	if !view.live() {
		return
	}
	if c := canvasOf(view); c != nil {
		c.owner = nil
		view.frames = false
		canvasFrames(view)
		releaseCapture(view)
	}
}

func (win *nativeWindow) attachToWindow(view *nativeView) {
	if !view.live() || win == nil || win.root == 0 {
		return
	}
	reparent(view.outer, win.root)
	if view.visible {
		win32ShowWindow(view.outer, swShowNA)
	}
	if view.kind == kindCanvas {
		canvasShown(view)
	}
}

func (view *nativeView) setParent(container *nativeView) {
	if !view.live() || !container.live() {
		return
	}
	if target := container.container(); target != 0 {
		reparent(view.outer, target)
		if view.visible {
			win32ShowWindow(view.outer, swShowNA)
		}
		if view.kind == kindCanvas {
			canvasShown(view)
		}
	}
}

func (view *nativeView) bringToFront() {
	if view.live() && view.outer != 0 {
		win32SetWindowPos(view.outer, hwndTop, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
}

func (view *nativeView) setFrame(x int, y int, width int, height int) {
	if !view.live() {
		return
	}
	width = max(width, 0)
	height = max(height, 0)
	// The layout code sets the frames of every view on each pass; moving a
	// window to where it already is costs a message round.
	if view.placed && view.x == x && view.y == y && view.width == width && view.height == height {
		return
	}
	view.placed = true
	view.x, view.y, view.width, view.height = x, y, width, height
	view.place()
	if view.ext != nil {
		view.ext.layout(view)
	}
}

func (view *nativeView) setVisible(visible bool) {
	if !view.live() {
		return
	}
	view.visible = visible
	if visible {
		win32ShowWindow(view.outer, swShowNA)
	} else {
		win32ShowWindow(view.outer, swHide)
	}
	if view.kind == kindCanvas {
		canvasShown(view)
	}
}

func enableChildCallback() uintptr {
	return callback("enableChild", func(child hwnd, enabled uintptr) uintptr {
		if view := viewOf(child); view == nil || view == viewOf(win32GetParent(child)) {
			win32EnableWindow(child, enabled != 0)
			win32InvalidateRect(child, nil, true)
		}
		return 1
	})
}

func (view *nativeView) setEnabled(enabled bool) {
	if !view.live() {
		return
	}
	view.enabled = enabled
	win32EnableWindow(view.outer, enabled)
	if view.control != 0 {
		win32EnableWindow(view.control, enabled)
	}
	if view.kind != kindLayout && view.kind != kindScroll {
		win32EnumChildWindows(view.outer, enableChildCallback(), boolArg(enabled))
	}
	view.invalidate()
}

func (view *nativeView) preferredSize() (int, int) {
	if !view.live() {
		return 0, 0
	}
	switch view.kind {
	case kindTabs, kindScroll:
		return 320, 240
	case kindLayout, kindCanvas:
		return 0, 0
	}
	if view.ext != nil {
		if size, ok := view.ext.preferred(view); ok {
			return int(math.Ceil(float64(size.width))), int(math.Ceil(float64(size.height)))
		}
	}
	return 0, 0
}

func (view *nativeView) framebufferSize() (int, int) {
	var client rect
	if view.live() && view.outer != 0 {
		win32GetClientRect(view.outer, &client)
	}
	return int(client.right), int(client.bottom)
}

// ---- Text ----

// textTarget returns the window that holds the text of a view.
func textTarget(view *nativeView) hwnd {
	if field := fieldOf(view); field != nil {
		return field.edit
	}
	if combo := selectCombo(view); combo != 0 {
		return combo
	}
	return view.target()
}

func (view *nativeView) currentText() string {
	if !view.live() {
		return ""
	}
	switch view.kind {
	case kindLabel, kindButton, kindLink, kindToggle, kindCheckBox, kindSection:
		return view.text
	}
	return strings.ReplaceAll(windowText(textTarget(view)), "\r", "")
}

func (view *nativeView) setText(text string) {
	if !view.live() {
		return
	}
	view.suppress++
	switch view.kind {
	case kindLabel:
		setLabelText(view, text)
	case kindButton, kindToggle:
		setButtonText(view, text)
	case kindCheckBox:
		setCheckBoxText(view, text)
	case kindLink:
		view.text = text
		win32SetWindowText(view.outer, linkMarkup(text))
	case kindSection:
		view.text = text
		view.invalidate()
	default:
		win32SetWindowText(textTarget(view), controlText(text))
	}
	view.suppress--
}

func (view *nativeView) setPlaceholder(text string) {
	if !view.live() {
		return
	}
	view.placeholder = text
	if field := fieldOf(view); field != nil {
		if !field.multiline {
			win32SendMessage(field.edit, emSetCueBanner, 1, uintptr(unsafe.Pointer(wide(text))))
		}
		win32InvalidateRect(field.edit, nil, true)
	} else if combo := selectCombo(view); combo != 0 {
		win32SendMessage(combo, cbSetCueBanner, 0, uintptr(unsafe.Pointer(wide(text))))
	}
}

// ---- Values ----

func (view *nativeView) isChecked() bool {
	if !view.live() {
		return false
	}
	if view.kind == kindCheckBox || view.kind == kindToggle {
		return win32SendMessage(view.target(), bmGetCheck, 0, 0) == bstChecked
	}
	return view.checked
}

func (view *nativeView) setChecked(checked bool) {
	if !view.live() {
		return
	}
	view.suppress++
	view.checked = checked
	if view.kind == kindCheckBox || view.kind == kindToggle {
		state := uintptr(bstUnchecked)
		if checked {
			state = bstChecked
		}
		win32SendMessage(view.target(), bmSetCheck, state, 0)
	} else {
		view.invalidate()
	}
	view.suppress--
}

func (view *nativeView) setReadOnly(readOnly bool) {
	if !view.live() || view.kind != kindTextArea {
		return
	}
	if field := fieldOf(view); field != nil {
		view.readOnly = readOnly
		win32SendMessage(field.edit, emSetReadOnly, boolArg(readOnly), 0)
		win32InvalidateRect(view.outer, nil, false)
	}
}

// ---- File dialogs ----

func dialogOwner(window *nativeWindow) hwnd {
	if win := window; win != nil && win.hwnd != 0 && win32IsWindowVisible(win.hwnd) {
		return win.hwnd
	}
	return 0
}

func dialogFilters(dialog comObject, extensions string) {
	var patterns []string
	for _, part := range strings.Split(extensions, ",") {
		part = strings.TrimLeft(part, " .")
		part = strings.TrimRight(part, " ")
		if part != "" {
			patterns = append(patterns, "*."+part)
		}
	}
	if len(patterns) == 0 {
		return
	}
	spec := comdlgFilterSpec{
		pszName: wide(strings.Join(patterns, ", ")),
		pszSpec: wide(strings.Join(patterns, ";")),
	}
	win32FileDialogSetFileTypes(dialog, 1, &spec)
	win32FileDialogSetFileTypeIndex(dialog, 1)
}

func dialogFolder(dialog comObject, directory string) {
	if directory == "" {
		return
	}
	var folder comObject
	if win32SHCreateItemFromParsingName(directory, 0, iidShellItem, &folder).succeeded() {
		win32FileDialogSetFolder(dialog, folder)
		win32UnknownRelease(folder)
	}
}

// itemPath returns the file system path of a shell item.
func itemPath(item comObject) string {
	var path uintptr
	if win32ShellItemGetDisplayName(item, sigdnFileSysPath, &path).failed() || path == 0 {
		return ""
	}
	defer win32CoTaskMemFree(path)
	return goStringAt(path)
}

func (win *nativeWindow) showOpenPanel(title string, directory string, filename string, extensions string, multiple bool, directories bool, hidden bool) (paths []string, cancelled bool) {
	defer deactivate(activate())
	var dialog comObject
	if win32CoCreateInstance(clsidFileOpenDialog, 0, clsctxInprocServer, iidFileOpenDialog, &dialog).failed() {
		return nil, true
	}
	defer win32UnknownRelease(dialog)
	var options uint32
	win32FileDialogGetOptions(dialog, &options)
	options |= fosForceFileSystem | fosPathMustExist
	if multiple {
		options |= fosAllowMultiSelect
	}
	if directories {
		options |= fosPickFolders
	}
	if hidden {
		options |= fosForceShowHidden
	}
	win32FileDialogSetOptions(dialog, options)
	if title != "" {
		win32FileDialogSetTitle(dialog, title)
	}
	if filename != "" {
		win32FileDialogSetFileName(dialog, filename)
	}
	if !directories {
		dialogFilters(dialog, extensions)
	}
	dialogFolder(dialog, directory)
	if win32ModalWindowShow(dialog, dialogOwner(win)).failed() {
		return nil, true
	}
	var items comObject
	if win32FileOpenDialogGetResults(dialog, &items).failed() || items == 0 {
		return nil, true
	}
	defer win32UnknownRelease(items)
	var count uint32
	win32ShellItemArrayGetCount(items, &count)
	for i := range count {
		var item comObject
		if win32ShellItemArrayGetItemAt(items, i, &item).succeeded() && item != 0 {
			if path := itemPath(item); path != "" {
				paths = append(paths, path)
			}
			win32UnknownRelease(item)
		}
	}
	return paths, false
}

func (win *nativeWindow) showSavePanel(title string, directory string, filename string, extensions string, hidden bool, defaultExtension string) (paths []string, cancelled bool) {
	defer deactivate(activate())
	var dialog comObject
	if win32CoCreateInstance(clsidFileSaveDialog, 0, clsctxInprocServer, iidFileSaveDialog, &dialog).failed() {
		return nil, true
	}
	defer win32UnknownRelease(dialog)
	var options uint32
	win32FileDialogGetOptions(dialog, &options)
	options |= fosForceFileSystem | fosOverwritePrompt
	if hidden {
		options |= fosForceShowHidden
	}
	win32FileDialogSetOptions(dialog, options)
	if title != "" {
		win32FileDialogSetTitle(dialog, title)
	}
	if filename != "" {
		win32FileDialogSetFileName(dialog, filename)
	}
	if defaultExtension != "" {
		win32FileDialogSetDefaultExtension(dialog, strings.TrimPrefix(defaultExtension, "."))
	}
	dialogFilters(dialog, extensions)
	dialogFolder(dialog, directory)
	if win32ModalWindowShow(dialog, dialogOwner(win)).failed() {
		return nil, true
	}
	var item comObject
	if win32FileDialogGetResult(dialog, &item).failed() || item == 0 {
		return nil, true
	}
	defer win32UnknownRelease(item)
	if path := itemPath(item); path != "" {
		paths = append(paths, path)
	}
	return paths, false
}
