package win32

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

func (view *nativeView) disconnect() {
	if view.live() {
		view.receiver = nil
	}
}

// viewNative creates a view with an extension for a native control.
func viewNative(window *nativeWindow, class, text string, style uint32, kind int, ext extension) *nativeView {
	h := makeNative(class, text, style, 0, 0)
	if h == 0 {
		return nil
	}
	view := adoptView(window, h, 0, kind)
	view.ext = ext
	return view
}

func buttonCheck(checked bool) uintptr {
	if checked {
		return bstChecked
	}
	return bstUnchecked
}

// ---- Toggle button ----

type toggleExtension struct {
	baseExtension
}

func (toggleExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	view.report(changeEvent{})
	return true
}

func (toggleExtension) themeChanged(view *nativeView) {
	buttonThemeChanged(view)
}

func (toggleExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return buttonPreferred(view), true
}

func (win *nativeWindow) newToggleButton(text string, icon *iconImage, checked bool, toolbar bool) *nativeView {
	view := viewNative(win, wcButton, text, bsAutoCheckBox|bsPushLike|wsTabStop, kindToggle, toggleExtension{})
	if view == nil {
		return nil
	}
	view.text = text
	win32SendMessage(view.outer, bmSetCheck, buttonCheck(checked), 0)
	if icon != nil {
		view.configureButton(icon, 0, 0, "", 0)
	}
	return view
}

// ---- Switch ----
//
// Windows has no switch control in its classic controls, so a switch is a check
// box, which assistive technology reports as a switch.

type switchExtension struct {
	baseExtension
	on, off string
	// The track of the switch, in logical units, for its control size:
	// regular, mini, small, or large.
	size                         int
	trackWidth, trackHeight, gap float32
}

func (s *switchExtension) setSize(value int) {
	s.size = value
	widths := [4]float32{40, 28, 34, 48}
	heights := [4]float32{20, 14, 17, 24}
	index := value
	if index < 0 || index >= 4 {
		index = 0
	}
	s.trackWidth = widths[index]
	s.trackHeight = heights[index]
	switch index {
	case 1:
		s.gap = 6
	case 2:
		s.gap = 8
	default:
		s.gap = 10
	}
}

func (s *switchExtension) font(view *nativeView) fontSpec {
	base := bodyFont()
	sizes := [4]float32{0, 11, 12, 16}
	if s.size > 0 && s.size < 4 {
		base.size = sizes[s.size]
	}
	return view.fontOr(base)
}

func (s *switchExtension) checked(view *nativeView) bool {
	return win32SendMessage(view.outer, bmGetCheck, 0, 0) == bstChecked
}

func (s *switchExtension) stateLabel(checked bool) string {
	if checked {
		return s.on
	}
	return s.off
}

func (s *switchExtension) label(view *nativeView) string {
	if view.text != "" {
		return view.text
	}
	return s.stateLabel(s.checked(view))
}

func (s *switchExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	view.accessibleValue = s.stateLabel(s.checked(view))
	win32InvalidateRect(view.outer, nil, false)
	view.report(changeEvent{})
	return true
}

// notify draws the switch over the check box, which keeps the keyboard
// behavior, the focus, and the accessibility of a check box: Windows has no
// switch among its common controls.
func (s *switchExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.code != nmCustomDraw || header.hwndFrom != view.outer {
		return false
	}
	draw := (*nmCustomDrawInfo)(unsafe.Pointer(header))
	if draw.dwDrawStage != cddsPrePaint {
		*result = cdrfDoDefault
		return true
	}
	pal := view.palette()
	checked := s.checked(view)
	enabled := win32IsWindowEnabled(view.outer)
	hot := draw.uItemState&cdisHot != 0
	pressed := draw.uItemState&cdisSelected != 0
	focused := draw.uItemState&cdisFocus != 0 &&
		win32SendMessage(view.outer, wmQueryUIState, 0, 0)&uisfHideFocus == 0
	text := s.label(view)
	textFont := s.font(view)
	paintCanvas(view.outer, draw.hdc, &draw.rc, func(canvas *painter) {
		y, x := (canvas.height-s.trackHeight)/2, float32(3)
		radius := s.trackHeight / 2
		if checked {
			fill := pal.accent
			switch {
			case !enabled:
				fill = pal.accentDisabled
			case pressed:
				fill = pal.accentPressed
			case hot:
				fill = pal.accentHover
			}
			canvas.fillRound(x, y, s.trackWidth, s.trackHeight, radius, fill)
		} else {
			fill := pal.control
			if hot && enabled {
				fill = pal.controlHover
			}
			canvas.fillRound(x, y, s.trackWidth, s.trackHeight, radius, fill)
			stroke := pal.controlStrongDisabled
			if enabled {
				stroke = pal.controlStrong
			}
			canvas.strokeRound(x+0.5, y+0.5, s.trackWidth-1, s.trackHeight-1, radius-0.5, stroke, 1)
		}
		// The thumb grows under the pointer and stretches while pressed, as in
		// Windows.
		unit := s.trackHeight / 20
		thumb := 12 * unit
		if pressed || hot {
			thumb = 14 * unit
		}
		var stretch float32
		if pressed {
			stretch = 3 * unit
		}
		cy := y + s.trackHeight/2
		cx := x + s.trackHeight/2 + stretch/2
		if checked {
			cx = x + s.trackWidth - s.trackHeight/2 - stretch/2
		}
		knob := pal.textDisabled
		switch {
		case checked:
			knob = pal.textOnAccent
		case enabled:
			knob = pal.textSecondary
		}
		canvas.fillRound(cx-thumb/2-stretch/2, cy-thumb/2, thumb+stretch, thumb, thumb/2, knob)
		if focused {
			canvas.strokeRound(x-2, y-2, s.trackWidth+4, s.trackHeight+4, radius+2, pal.focus, 1.5)
		}
		if text != "" {
			tx := x + s.trackWidth + s.gap
			color := pal.textDisabled
			if enabled {
				color = view.textColorOr(pal.text)
			}
			canvas.text(text, textFont, tx, 0, canvas.width-tx, canvas.height, color, 0, 1, false, true)
		}
	})
	*result = cdrfSkipDefault
	return true
}

func (s *switchExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	textFont := s.font(view)
	text := view.text
	if text == "" {
		text = s.off
		if len(s.on) > len(s.off) {
			text = s.on
		}
	}
	width := s.trackWidth + 6
	if text != "" {
		width += s.gap + float32(measureGDI(text, textFont, 0, 0).cx) + 2
	}
	height := max(s.trackHeight+6, float32(measureGDI("Ag", textFont, 0, 0).cy+4))
	return d2d1SizeF{width: width, height: height}, true
}

func (win *nativeWindow) newSwitch(text string, onLabel string, offLabel string, checked bool, size int) *nativeView {
	state := &switchExtension{on: onLabel, off: offLabel}
	state.setSize(size)
	view := viewNative(win, wcButton, text, bsAutoCheckBox|wsTabStop, kindSwitch, state)
	if view == nil {
		return nil
	}
	view.text = text
	win32SendMessage(view.outer, bmSetCheck, buttonCheck(checked), 0)
	view.accessibleValue = state.stateLabel(checked)
	return view
}

func (view *nativeView) isSwitchOn() bool {
	return view.live() && view.kind == kindSwitch && win32SendMessage(view.outer, bmGetCheck, 0, 0) == bstChecked
}

func (view *nativeView) configureSwitch(text string, checked bool) {
	if !view.live() || view.kind != kindSwitch {
		return
	}
	view.text = text
	win32SetWindowText(view.outer, text)
	view.suppress++
	win32SendMessage(view.outer, bmSetCheck, buttonCheck(checked), 0)
	view.suppress--
	if state, ok := view.ext.(*switchExtension); ok {
		view.accessibleValue = state.stateLabel(checked)
	}
}

// ---- Password field ----

const passwordCharacter = 0x25cf

// passwordExtension is a password text box with a button that shows the
// password while it is pressed down.
type passwordExtension struct {
	fieldExtension
	reveal   hwnd
	revealed bool
	images   himagelist
}

func (p *passwordExtension) free() {
	if p.images != 0 {
		win32ImageListDestroy(p.images)
	}
}

func (p *passwordExtension) layout(view *nativeView) {
	p.fieldExtension.layout(view)
	if p.reveal == 0 {
		return
	}
	var client rect
	win32GetClientRect(view.outer, &client)
	scale := view.scale()
	width, border := int32(math.Round(28*scale)), int32(math.Round(scale))
	win32SetWindowPos(p.reveal, 0, client.right-border-width, border, width, client.bottom-2*border,
		swpNoZOrder|swpNoActivate)
	// The edit control stops before the button.
	var edit rect
	win32GetWindowRect(p.edit, &edit)
	mapWindowRect(0, view.outer, &edit)
	win32SetWindowPos(p.edit, 0, 0, 0, client.right-border-width-edit.left, edit.bottom-edit.top,
		swpNoMove|swpNoZOrder|swpNoActivate)
}

func (p *passwordExtension) show(visible bool) {
	character := uintptr(passwordCharacter)
	if visible {
		character = 0
	}
	win32SendMessage(p.edit, emSetPasswordChar, character, 0)
	win32InvalidateRect(p.edit, nil, true)
}

func (p *passwordExtension) command(view *nativeView, from hwnd, code int) bool {
	if from == p.reveal && code == bnClicked {
		visible := win32SendMessage(p.reveal, tbIsButtonChecked, 1, 0) != 0
		p.show(visible || p.revealed)
		return true
	}
	return p.fieldExtension.command(view, from, code)
}

func (p *passwordExtension) icons(view *nativeView) {
	eye := &iconImage{glyph: ""}
	scale := view.scale()
	size := int32(math.Round(16 * scale))
	list := win32ImageListCreate(size, size, ilcColor32, 1, 0)
	bitmap := iconBitmap(eye, int(size), view.palette().text)
	win32ImageListAdd(list, bitmap, 0)
	win32DeleteObject(hgdiobj(bitmap))
	win32SendMessage(p.reveal, tbSetImageList, 0, uintptr(list))
	if p.images != 0 {
		win32ImageListDestroy(p.images)
	}
	p.images = list
	win32SendMessage(p.reveal, tbSetButtonSize, 0, makeWParam(int(size)+int(math.Round(10*scale)), int(size)))
}

func (p *passwordExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.hwndFrom != p.reveal || header.code != nmCustomDraw {
		return false
	}
	draw := (*nmCustomDrawInfo)(unsafe.Pointer(header))
	if draw.dwDrawStage == cddsPreErase {
		// The toolbar lies inside the text box, so it has the fill of the text
		// box behind its button.
		pal := view.palette()
		fill := rgbColor(0xffffff)
		if pal.dark {
			fill = pal.inputActive
		}
		var client rect
		win32GetClientRect(p.reveal, &client)
		fillRectColor(draw.hdc, &client, fill)
		*result = cdrfSkipDefault
		return true
	}
	*result = cdrfDoDefault
	return true
}

func (p *passwordExtension) themeChanged(view *nativeView) {
	p.fieldExtension.themeChanged(view)
	if p.reveal != 0 {
		applyThemeTo(p.reveal, view.theme(), "")
		p.icons(view)
	}
}

func (win *nativeWindow) newPasswordField(placeholder string, reveal bool) *nativeView {
	field := &passwordExtension{}
	view := createField(win, kindPassword, esAutoHScroll|esPassword, field)
	win32SendMessage(field.edit, emSetPasswordChar, passwordCharacter, 0)
	view.placeholder = placeholder
	win32SendMessage(field.edit, emSetCueBanner, 1, uintptr(unsafe.Pointer(wide(placeholder))))
	if reveal {
		view.value = 1
		// The reveal button is a flat toolbar button, which has no border until
		// the pointer is over it, like the reveal button of Windows.
		field.reveal = makeNative(toolbarClassName, "",
			tbstyleFlat|tbstyleCustomErase|tbstyleTooltips|ccsNoResize|ccsNoDivider|ccsNoParentAlign, 0, view.outer)
		setViewOf(field.reveal, view)
		win32SendMessage(field.reveal, tbButtonStructSize, unsafe.Sizeof(tbButton{}), 0)
		field.icons(view)
		button := tbButton{iBitmap: 0, idCommand: 1, fsState: tbstateEnabled, fsStyle: btnsCheck}
		win32SendMessage(field.reveal, tbAddButtons, 1, uintptr(unsafe.Pointer(&button)))
		applyThemeTo(field.reveal, view.theme(), "")
		field.layout(view)
	}
	return view
}

func (view *nativeView) passwordText() string {
	if field := fieldOf(view); field != nil {
		return windowText(field.edit)
	}
	return ""
}

func (view *nativeView) setPassword(text string, placeholder string, revealed bool) {
	if !view.live() {
		return
	}
	field, ok := view.ext.(*passwordExtension)
	if !ok {
		return
	}
	view.suppress++
	if windowText(field.edit) != text {
		win32SetWindowText(field.edit, text)
	}
	view.suppress--
	view.placeholder = placeholder
	win32SendMessage(field.edit, emSetCueBanner, 1, uintptr(unsafe.Pointer(wide(placeholder))))
	field.revealed = revealed
	field.show(revealed)
}

// ---- Search field ----
//
// The suggestions of a search field appear in the autocomplete list of Windows,
// which the shell provides for edit controls.

// suggestions implements IEnumString over the suggestions of a search field.
type suggestions struct {
	items []string
	next  int
}

var suggestionsTable []uintptr

func newSuggestions(items []string, next int) *comServer {
	if suggestionsTable == nil {
		suggestionsTable = methodTable(
			callback("IEnumString.Next", func(this, count, out, fetched uintptr) uintptr {
				list, ok := serverObject[*suggestions](this)
				if !ok {
					return eUnexpected
				}
				done := 0
				results := unsafe.Slice((*uintptr)(memoryAt(out)), count)
				for done < int(count) && list.next < len(list.items) {
					text := utf16.Encode([]rune(list.items[list.next]))
					text = append(text, 0)
					memory := win32CoTaskMemAlloc(uintptr(len(text) * 2))
					copy(unsafe.Slice((*uint16)(memoryAt(memory)), len(text)), text)
					results[done] = memory
					done++
					list.next++
				}
				if fetched != 0 {
					*(*uint32)(memoryAt(fetched)) = uint32(done)
				}
				if done == int(count) {
					return sOK
				}
				return sFalse
			}),
			callback("IEnumString.Skip", func(this, count uintptr) uintptr {
				list, ok := serverObject[*suggestions](this)
				if !ok {
					return eUnexpected
				}
				list.next += int(count)
				if list.next <= len(list.items) {
					return sOK
				}
				return sFalse
			}),
			callback("IEnumString.Reset", func(this uintptr) uintptr {
				if list, ok := serverObject[*suggestions](this); ok {
					list.next = 0
				}
				return sOK
			}),
			callback("IEnumString.Clone", func(this, out uintptr) uintptr {
				list, ok := serverObject[*suggestions](this)
				if !ok {
					return eUnexpected
				}
				clone := newSuggestions(slices.Clone(list.items), list.next)
				*(*uintptr)(memoryAt(out)) = clone.address()
				return sOK
			}),
		)
	}
	return newCOMServer(suggestionsTable, []*guid{iidEnumString}, &suggestions{items: items, next: next})
}

type searchExtension struct {
	fieldExtension
	suggestions *comServer
	complete    comObject
}

func (s *searchExtension) free() {
	win32UnknownRelease(s.complete)
	if s.suggestions != nil {
		win32UnknownRelease(s.suggestions.com())
	}
}

func (win *nativeWindow) newSearchField(text string, placeholder string) *nativeView {
	field := &searchExtension{}
	view := createField(win, kindSearch, esAutoHScroll, field)
	view.placeholder = placeholder
	win32SendMessage(field.edit, emSetCueBanner, 1, uintptr(unsafe.Pointer(wide(placeholder))))
	view.suppress++
	win32SetWindowText(field.edit, text)
	view.suppress--
	field.suggestions = newSuggestions(nil, 0)
	if win32CoCreateInstance(clsidAutoComplete, 0, clsctxInprocServer, iidAutoComplete2, &field.complete).succeeded() {
		win32AutoCompleteInit(field.complete, field.edit, field.suggestions.com(), "", "")
		win32AutoComplete2SetOptions(field.complete, acoAutoSuggest|acoUpDownKeyDropsList)
	}
	return view
}

func (view *nativeView) setSuggestions(items []string) {
	if !view.live() {
		return
	}
	field, ok := view.ext.(*searchExtension)
	if !ok {
		return
	}
	list := field.suggestions.object.(*suggestions)
	list.items = slices.Clone(items)
	list.next = 0
}

// ---- Number field ----

type numberExtension struct {
	fieldExtension
	minimum, maximum, step float64
	decimals               int
}

func (n *numberExtension) format(value float64) string {
	return strconv.FormatFloat(value, 'f', n.decimals, 64)
}

func (n *numberExtension) show(view *nativeView) {
	view.suppress++
	win32SetWindowText(n.edit, n.format(view.value))
	view.suppress--
}

// commit reads the value from the text, as a number box does when it loses
// the focus or the user presses Return.
func (n *numberExtension) commit(view *nativeView) {
	value, ok := leadingNumber(windowText(n.edit))
	if !ok {
		n.show(view)
		return
	}
	n.set(view, value)
}

// leadingNumber parses the number at the start of text, as wcstod does.
func leadingNumber(text string) (float64, bool) {
	text = strings.TrimLeft(text, " \t\r\n")
	for end := len(text); end > 0; end-- {
		if value, err := strconv.ParseFloat(text[:end], 64); err == nil {
			return value, true
		}
	}
	return 0, false
}

func (n *numberExtension) set(view *nativeView, value float64) {
	factor := math.Pow(10, float64(n.decimals))
	value = math.Round(clamp(value, n.minimum, n.maximum)*factor) / factor
	changed := value != view.value
	view.value = value
	n.show(view)
	if changed {
		view.report(changeEvent{})
	}
}

func (n *numberExtension) changed(view *nativeView) {
}

func (n *numberExtension) submitted(view *nativeView) {
	n.commit(view)
}

func (n *numberExtension) focusLost(view *nativeView) {
	n.commit(view)
}

func (n *numberExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.hwndFrom == n.updown && header.code == udnDeltaPos {
		change := (*nmUpDown)(unsafe.Pointer(header))
		n.commit(view)
		n.set(view, view.value-float64(change.iDelta)*n.step)
		*result = 1
		return true
	}
	return false
}

func (n *numberExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if h == n.edit && message == wmKeyDown && (wparam == vkUp || wparam == vkDown) {
		n.commit(view)
		if wparam == vkUp {
			n.set(view, view.value+n.step)
		} else {
			n.set(view, view.value-n.step)
		}
		*result = 0
		return true
	}
	return n.fieldExtension.message(view, h, message, wparam, lparam, result)
}

func (win *nativeWindow) newNumberField(minimum float64, maximum float64, value float64, step float64, decimals int) *nativeView {
	field := &numberExtension{minimum: minimum, maximum: max(maximum, minimum), step: step, decimals: max(decimals, 0)}
	if step <= 0 {
		field.step = 1
	}
	view := createField(win, kindNumber, esAutoHScroll|esRight, field)
	field.updown = makeNative(updownClass, "", udsArrowKeys|udsHotTrack, 0, view.outer)
	setViewOf(field.updown, view)
	win32SendMessage(field.updown, udmSetRange32, signExtend(0xfff0bdc0), 1000000)
	applyThemeTo(field.updown, view.theme(), "")
	view.value = clamp(value, field.minimum, field.maximum)
	view.minimum = field.minimum
	view.maximum = field.maximum
	field.show(view)
	field.layout(view)
	return view
}

func (view *nativeView) numberFieldValue() float64 {
	if view.live() && view.kind == kindNumber {
		return view.value
	}
	return 0
}

func (view *nativeView) setNumberFieldValue(value float64) {
	if !view.live() {
		return
	}
	if field, ok := view.ext.(*numberExtension); ok {
		factor := math.Pow(10, float64(field.decimals))
		view.value = math.Round(clamp(value, field.minimum, field.maximum)*factor) / factor
		field.show(view)
	}
}

// ---- Editable combo ----

func comboFill(combo hwnd, items []string) {
	win32SendMessage(combo, cbResetContent, 0, 0)
	for _, item := range items {
		win32SendMessage(combo, cbAddString, 0, uintptr(unsafe.Pointer(wide(item))))
	}
}

func (win *nativeWindow) newComboBox(text string, items []string) *nativeView {
	view := newSelectView(win, kindCombo, cbsDropDown|cbsAutoHScroll)
	combo := selectCombo(view)
	view.suppress++
	comboFill(combo, items)
	win32SetWindowText(combo, text)
	view.suppress--
	return view
}

func (view *nativeView) setComboItems(items []string) {
	if !view.live() || view.kind != kindCombo {
		return
	}
	combo := selectCombo(view)
	if combo == 0 {
		return
	}
	text := windowText(combo)
	view.suppress++
	comboFill(combo, items)
	win32SetWindowText(combo, text)
	view.suppress--
}

// ---- Segmented control ----
//
// A segmented control is a row of buttons. Mode 0 is momentary buttons, mode 1
// selects one segment like radio buttons, and mode 2 selects any set of
// segments like check boxes.

// segment is a button of a segmented control.
type segment struct {
	button hwnd
	title  string
	icon   *iconImage
	// images is the image list of the icon of the button.
	images himagelist
}

type segmentsExtension struct {
	baseExtension
	mode     ui.SelectionMode
	clicked  int
	segments []segment
}

func (s *segmentsExtension) free() {
	for _, item := range s.segments {
		if item.images != 0 {
			win32ImageListDestroy(item.images)
		}
	}
}

// applyIcon gives a segment its icon in the text color of the theme, with the
// padding of a button from the leading border.
func (s *segmentsExtension) applyIcon(view *nativeView, item *segment) {
	if item.images != 0 {
		win32ImageListDestroy(item.images)
		item.images = 0
	}
	if item.icon == nil {
		return
	}
	scale := scaleOf(item.button)
	side := int32(math.Round(16 * scale))
	item.images = win32ImageListCreate(side, side, ilcColor32, 1, 0)
	bitmap := iconBitmap(item.icon, int(side), view.palette().text)
	win32ImageListAdd(item.images, bitmap, 0)
	win32DeleteObject(hgdiobj(bitmap))
	list := buttonImageList{himl: item.images, uAlign: buttonImageListAlignCenter}
	if item.title != "" {
		list.uAlign = buttonImageListAlignLeft
		list.margin = rect{left: int32(math.Round(8 * scale)), right: int32(math.Round(2 * scale))}
	}
	win32SendMessage(item.button, bcmSetImageList, 0, uintptr(unsafe.Pointer(&list)))
	win32InvalidateRect(item.button, nil, true)
}

func (s *segmentsExtension) themeChanged(view *nativeView) {
	for i := range s.segments {
		applyThemeTo(s.segments[i].button, view.theme(), "")
		s.applyIcon(view, &s.segments[i])
	}
}

func (s *segmentsExtension) layout(view *nativeView) {
	count := len(s.segments)
	if count == 0 {
		return
	}
	width := float64(view.width) / float64(count)
	for i, item := range s.segments {
		x := math.Floor(float64(i) * width)
		placeWindow(item.button, x, 0, math.Floor(float64(i+1)*width)-x, float64(view.height))
	}
}

func (s *segmentsExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	for i, item := range s.segments {
		if item.button != from {
			continue
		}
		s.clicked = i
		if s.mode == ui.SelectionSingle {
			for other, otherItem := range s.segments {
				win32SendMessage(otherItem.button, bmSetCheck, buttonCheck(other == i), 0)
			}
		}
		view.report(changeEvent{})
		return true
	}
	return false
}

func (s *segmentsExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	font := view.fontOr(bodyFont())
	widest := float32(40)
	for _, item := range s.segments {
		// A button centers its text in its whole width, so an icon before the
		// text has its room on both sides.
		text := float32(measureGDI(item.title, font, 0, 0).cx)
		width := text + 24
		if item.icon != nil {
			width = text + 2*(8+16+4)
			if item.title == "" {
				width = 16 + 24
			}
		}
		widest = max(widest, width)
	}
	return d2d1SizeF{width: widest * float32(len(s.segments)), height: float32(measureGDI("Ag", font, 0, 0).cy + 8)},
		true
}

func segmentsOf(view *nativeView) *segmentsExtension {
	if view == nil || view.kind != kindSegmented {
		return nil
	}
	segments, _ := view.ext.(*segmentsExtension)
	return segments
}

func (win *nativeWindow) newSegmentedControl(items []ui.Segment, mode ui.SelectionMode) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindSegmented, win)
	segments := &segmentsExtension{mode: mode, clicked: -1}
	view.ext = segments
	for i, item := range items {
		style := uint32(wsTabStop)
		if i == 0 {
			style |= wsGroup
		}
		switch mode {
		case ui.SelectionNone:
			style |= bsPushButton
		case ui.SelectionSingle:
			style |= bsRadioButton | bsPushLike
		default:
			style |= bsAutoCheckBox | bsPushLike
		}
		button := makeNative(wcButton, item.Text, style, 0, host)
		setViewOf(button, view)
		win32SendMessage(button, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(button))), 0)
		applyThemeTo(button, view.theme(), "")
		win32EnableWindow(button, item.Enabled)
		segments.segments = append(segments.segments, segment{button: button, title: item.Text,
			icon: newIcon(item.Icon, 0)})
		segments.applyIcon(view, &segments.segments[i])
	}
	return view
}

func segmentItems(view *nativeView) []accessibleItem {
	var items []accessibleItem
	if segments := segmentsOf(view); segments != nil {
		kind := "RadioButton"
		if segments.mode == ui.SelectionNone {
			kind = "Button"
		}
		for _, item := range segments.segments {
			items = append(items, accessibleItem{kind, item.title})
		}
	}
	return items
}

func (view *nativeView) isSegmentSelected(index int) bool {
	segments := segmentsOf(view)
	if segments == nil || index < 0 || index >= len(segments.segments) || segments.mode == ui.SelectionNone {
		return false
	}
	return win32SendMessage(segments.segments[index].button, bmGetCheck, 0, 0) == bstChecked
}

func (view *nativeView) clickedSegment() int {
	if segments := segmentsOf(view); segments != nil {
		return segments.clicked
	}
	return -1
}

func (view *nativeView) setSegmentSelected(index int, selected bool) {
	segments := segmentsOf(view)
	if segments == nil || index < 0 || index >= len(segments.segments) || segments.mode == ui.SelectionNone {
		return
	}
	view.suppress++
	win32SendMessage(segments.segments[index].button, bmSetCheck, buttonCheck(selected), 0)
	view.suppress--
}

// ---- Value controls ----
//
// A stepper, a dial, and a range slider keep their values in valueState.

type valueState struct {
	minimum, maximum, value, upper, step float64
	wraps                                bool
}

func (s *valueState) values() *valueState {
	return s
}

// valueOwner is the extension of a value control.
type valueOwner interface {
	values() *valueState
}

func (s *valueState) snap(next float64) float64 {
	if s.step > 0 {
		next = s.minimum + math.Round((next-s.minimum)/s.step)*s.step
	}
	return clamp(next, s.minimum, s.maximum)
}

func (s *valueState) report(view *nativeView) {
	view.value = s.value
	if view.kind == kindRange {
		view.accessibleValue = fmt.Sprintf("%g to %g", s.value, s.upper)
	} else {
		view.accessibleValue = fmt.Sprintf("%g", s.value)
	}
	win32InvalidateRect(view.outer, nil, false)
	if view.kind == kindRange {
		view.report(rangeChangeEvent{lower: s.value, upper: s.upper})
	} else {
		view.report(changeEvent{})
	}
}

func valueOf(view *nativeView) *valueState {
	if view == nil {
		return nil
	}
	if owner, ok := view.ext.(valueOwner); ok {
		return owner.values()
	}
	return nil
}

// stepperExtension is an up-down control without a text box.
type stepperExtension struct {
	baseExtension
	valueState
}

func (s *stepperExtension) stepBy(view *nativeView, direction float64) {
	amount := s.step
	if amount <= 0 {
		amount = 1
	}
	next := s.value + direction*amount
	if next > s.maximum {
		next = s.maximum
		if s.wraps {
			next = s.minimum
		}
	} else if next < s.minimum {
		next = s.minimum
		if s.wraps {
			next = s.maximum
		}
	}
	if next != s.value {
		s.value = next
		s.report(view)
	}
}

func (s *stepperExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.code != udnDeltaPos {
		return false
	}
	change := (*nmUpDown)(unsafe.Pointer(header))
	if change.iDelta > 0 {
		s.stepBy(view, -1)
	} else {
		s.stepBy(view, 1)
	}
	*result = 1
	return true
}

func (s *stepperExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: 18, height: float32(measureGDI("Ag", bodyFont(), 0, 0).cy + 8)}, true
}

func (win *nativeWindow) newStepper(minimum float64, maximum float64, value float64, step float64, wraps bool) *nativeView {
	state := &stepperExtension{}
	state.minimum = minimum
	state.maximum = max(maximum, minimum)
	state.value = clamp(value, state.minimum, state.maximum)
	state.step = step
	state.wraps = wraps
	view := viewNative(win, updownClass, "", udsArrowKeys|udsHotTrack|wsTabStop, kindStepper, state)
	if view == nil {
		return nil
	}
	win32SendMessage(view.outer, udmSetRange32, signExtend(0xfff0bdc0), 1000000)
	view.value = state.value
	return view
}

// dialExtension is a knob that Windows has no control for. A dial is at its
// minimum at the lower left and turns clockwise through 300 degrees to the
// lower right. A dial that wraps is at its minimum at the top and turns through
// a full circle.
type dialExtension struct {
	baseExtension
	valueState
	start    float64
	sweep    float64
	dragging bool
}

func (d *dialExtension) paint(view *nativeView, canvas *painter) {
	pal := view.palette()
	radius := min(canvas.width, canvas.height)/2 - 3
	if radius <= 0 {
		return
	}
	cx, cy := canvas.width/2, canvas.height/2
	canvas.strokeEllipse(cx, cy, radius, radius, pal.controlStroke, 2)
	span := d.maximum - d.minimum
	fraction := 0.0
	if span > 0 {
		fraction = (d.value - d.minimum) / span
	}
	angle := d.start + fraction*d.sweep
	accent, needle := pal.accentDisabled, pal.textDisabled
	if view.enabled {
		accent, needle = pal.accent, pal.text
	}
	canvas.arc(cx, cy, radius, float32(d.start), float32(fraction*d.sweep), accent, 2)
	canvas.line(cx, cy, cx+float32(math.Cos(angle))*radius*0.8, cy+float32(math.Sin(angle))*radius*0.8, needle, 2.5)
	if view.focused && view.keyboardFocus {
		canvas.strokeEllipse(cx, cy, radius+2, radius+2, pal.focus, 1.5)
	}
}

func (d *dialExtension) point(view *nativeView, x, y int) {
	scale := view.scale()
	size := clientSize(view.outer)
	angle := math.Atan2(float64(y)/scale-float64(size.height)/2, float64(x)/scale-float64(size.width)/2)
	offset := math.Mod(angle-d.start+4*math.Pi, 2*math.Pi)
	if offset > d.sweep {
		if offset-d.sweep < (2*math.Pi-d.sweep)/2 {
			offset = d.sweep
		} else {
			offset = 0
		}
	}
	next := d.snap(d.minimum + offset/d.sweep*(d.maximum-d.minimum))
	if d.wraps && next >= d.maximum {
		// The top of a full circle is the minimum.
		next = d.minimum
	}
	if next != d.value {
		d.value = next
		d.report(view)
	}
}

func (d *dialExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	switch message {
	case wmLButtonDown:
		win32SetFocus(view.outer)
		win32SetCapture(view.outer)
		d.dragging = true
		d.point(view, signedLoword(lparam), signedHiword(lparam))
		*result = 0
		return true
	case wmMouseMove:
		if d.dragging {
			d.point(view, signedLoword(lparam), signedHiword(lparam))
		}
	case wmLButtonUp, wmCaptureChanged:
		if d.dragging {
			d.dragging = false
			if message == wmLButtonUp {
				win32ReleaseCapture()
			}
		}
	case wmKeyDown:
		amount := d.step
		if amount <= 0 {
			amount = (d.maximum - d.minimum) / 100
		}
		next := d.value
		switch wparam {
		case vkRight, vkUp:
			next += amount
		case vkLeft, vkDown:
			next -= amount
		case vkHome:
			next = d.minimum
		case vkEnd:
			next = d.maximum
		default:
			return false
		}
		if d.wraps && next > d.maximum {
			next = d.minimum + (next - d.maximum)
		} else if d.wraps && next < d.minimum {
			next = d.maximum - (d.minimum - next)
		}
		next = d.snap(next)
		if next != d.value {
			d.value = next
			d.report(view)
		}
		*result = 0
		return true
	}
	return false
}

func (d *dialExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: 48, height: 48}, true
}

func (win *nativeWindow) newDial(minimum float64, maximum float64, value float64, step float64, wraps bool) *nativeView {
	dial := &dialExtension{start: 120 * math.Pi / 180, sweep: 300 * math.Pi / 180}
	dial.minimum = minimum
	dial.maximum = max(maximum, minimum)
	dial.value = clamp(value, dial.minimum, dial.maximum)
	dial.step = step
	dial.wraps = wraps
	if wraps {
		dial.start = -math.Pi / 2
		dial.sweep = 2 * math.Pi
	}
	view := createDrawn(win, kindDial, dial, true)
	view.value = dial.value
	return view
}

func (view *nativeView) controlValue() float64 {
	if state := valueOf(view); state != nil {
		return state.value
	}
	return 0
}

func (view *nativeView) setControlValue(value float64) {
	state := valueOf(view)
	if state == nil {
		return
	}
	state.value = clamp(value, state.minimum, state.maximum)
	view.value = state.value
	win32InvalidateRect(view.outer, nil, false)
}

// rangeExtension is a slider with two thumbs, which Windows has no control
// for. It draws the track and the thumbs of the trackbar of Windows.
type rangeExtension struct {
	baseExtension
	valueState
	dragging    int
	focusedKnob int
}

const rangeInset = 8

func (r *rangeExtension) position(width, value float64) float64 {
	span := r.maximum - r.minimum
	fraction := 0.0
	if span > 0 {
		fraction = (value - r.minimum) / span
	}
	return rangeInset + fraction*max(width-rangeInset*2, 0)
}

func (r *rangeExtension) valueAt(width, x float64) float64 {
	track := max(width-rangeInset*2, 1)
	offset := clamp(x-rangeInset, 0, track)
	return r.snap(r.minimum + offset/track*(r.maximum-r.minimum))
}

func (r *rangeExtension) paintGDI(view *nativeView, h hwnd, dc hdc, client *rect) bool {
	scale := view.scale()
	width := float64(client.right) / scale
	middle := client.bottom / 2
	scaled := func(value float64) int32 {
		return int32(math.Round(value * scale))
	}
	lower, higher := scaled(r.position(width, r.value)), scaled(r.position(width, r.upper))
	pad := scaled(rangeInset)
	track := rect{left: pad, top: middle - scaled(2), right: client.right - pad, bottom: middle + scaled(2)}
	theme := view.theme()
	pal := paletteFor(theme)
	var data htheme
	if theme != 2 {
		data = win32OpenThemeData(h, "Trackbar")
	}
	if data != 0 {
		win32DrawThemeBackground(data, dc, tkpTrack, trsNormal, &track, nil)
	} else {
		fillRectColor(dc, &track, pal.controlStrong)
	}
	span := rect{left: lower, top: track.top + 1, right: higher, bottom: track.bottom - 1}
	if view.enabled {
		fillRectColor(dc, &span, pal.accent)
	} else {
		fillRectColor(dc, &span, pal.accentDisabled)
	}
	thumbWidth, thumbHeight := scaled(11), scaled(20)
	for knob, x := range []int32{lower, higher} {
		thumb := rect{left: x - thumbWidth/2, top: middle - thumbHeight/2}
		thumb.right = thumb.left + thumbWidth
		thumb.bottom = thumb.top + thumbHeight
		state := int32(tusNormal)
		switch {
		case !view.enabled:
			state = tusDisabled
		case r.dragging == knob:
			state = tusPressed
		case view.focused && r.focusedKnob == knob:
			state = tusFocused
		}
		if data != 0 {
			win32DrawThemeBackground(data, dc, tkpThumb, state, &thumb, nil)
		} else if state == tusPressed {
			fillRectColor(dc, &thumb, pal.accentPressed)
		} else {
			fillRectColor(dc, &thumb, pal.accent)
		}
		if view.focused && r.focusedKnob == knob && view.keyboardFocus {
			focus := thumb
			win32InflateRect(&focus, scaled(2), scaled(2))
			win32DrawFocusRect(dc, &focus)
		}
	}
	if data != 0 {
		win32CloseThemeData(data)
	}
	return true
}

func (r *rangeExtension) move(view *nativeView, x int) {
	width := float64(clientSize(view.outer).width)
	next := r.valueAt(width, float64(x)/view.scale())
	if r.dragging == 0 {
		next = min(next, r.upper)
		if next == r.value {
			return
		}
		r.value = next
	} else {
		next = max(next, r.value)
		if next == r.upper {
			return
		}
		r.upper = next
	}
	r.report(view)
}

func (r *rangeExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	switch message {
	case wmLButtonDown:
		win32SetFocus(view.outer)
		win32SetCapture(view.outer)
		width := float64(clientSize(view.outer).width)
		x := float64(signedLoword(lparam)) / view.scale()
		lowerX, upperX := r.position(width, r.value), r.position(width, r.upper)
		r.dragging = 1
		if math.Abs(x-lowerX) <= math.Abs(x-upperX) && !(x > upperX && lowerX == upperX) {
			r.dragging = 0
		}
		r.focusedKnob = r.dragging
		r.move(view, signedLoword(lparam))
		win32InvalidateRect(view.outer, nil, false)
		*result = 0
		return true
	case wmMouseMove:
		if r.dragging >= 0 {
			r.move(view, signedLoword(lparam))
		}
	case wmLButtonUp, wmCaptureChanged:
		if r.dragging >= 0 {
			r.dragging = -1
			if message == wmLButtonUp {
				win32ReleaseCapture()
			}
			win32InvalidateRect(view.outer, nil, false)
		}
	case wmGetDlgCode:
		*result = dlgcWantArrows
		return true
	case wmKeyDown:
		amount := r.step
		if amount <= 0 {
			amount = (r.maximum - r.minimum) / 100
		}
		var delta float64
		switch wparam {
		case vkTab:
			return false
		case vkSpace:
			r.focusedKnob = 1 - r.focusedKnob
			win32InvalidateRect(view.outer, nil, false)
			*result = 0
			return true
		case vkRight, vkUp:
			delta = amount
		case vkLeft, vkDown:
			delta = -amount
		default:
			return false
		}
		if r.focusedKnob == 0 {
			r.value = min(r.snap(r.value+delta), r.upper)
		} else {
			r.upper = max(r.snap(r.upper+delta), r.value)
		}
		r.report(view)
		*result = 0
		return true
	}
	return false
}

func (r *rangeExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: 160, height: 26}, true
}

func (win *nativeWindow) newRangeSlider(minimum float64, maximum float64, lower float64, upper float64, step float64) *nativeView {
	r := &rangeExtension{dragging: -1}
	r.minimum = minimum
	r.maximum = max(maximum, minimum)
	r.step = step
	r.value = clamp(lower, r.minimum, r.maximum)
	r.upper = clamp(upper, r.value, r.maximum)
	return createDrawn(win, kindRange, r, true)
}

func (view *nativeView) rangeValues() (float64, float64) {
	if state := valueOf(view); state != nil {
		return state.value, state.upper
	}
	return 0, 0
}

func (view *nativeView) setRange(lower float64, upper float64) {
	state := valueOf(view)
	if state == nil {
		return
	}
	state.value = lower
	state.upper = upper
	win32InvalidateRect(view.outer, nil, false)
}

// ---- Dates and times ----
//
// Dates are exchanged as seconds since 1970 in UTC, and pickers show them as
// the same calendar date, so a date never shifts with the time zone.

const (
	elementsSeconds = 0x0002
	elementsDate    = 0x00e0
)

func systemDate(seconds float64) systemTime {
	date := time.Unix(int64(math.Floor(seconds)), 0).UTC()
	return systemTime{
		wYear:      uint16(date.Year()),
		wMonth:     uint16(date.Month()),
		wDay:       uint16(date.Day()),
		wDayOfWeek: uint16(date.Weekday()),
		wHour:      12,
	}
}

func secondsOf(date systemTime) float64 {
	return float64(time.Date(int(date.wYear), time.Month(date.wMonth), int(date.wDay), 0, 0, 0, 0, time.UTC).Unix())
}

func dayNumber(seconds float64) int {
	return int(math.Floor(seconds / 86400))
}

func localeUses12Hour() bool {
	var format [80]uint16
	win32GetLocaleInfoEx("", localeSTimeFormat, &format[0], int32(len(format)))
	return strings.ContainsRune(goString(format[:]), 'h')
}

type dateExtension struct {
	baseExtension
	elements         int
	value            float64
	minimum, maximum float64
	clock            int
	calendar         bool
}

func (d *dateExtension) clamp(value float64) float64 {
	if d.minimum != 0 && value < d.minimum {
		value = d.minimum
	}
	if d.maximum != 0 && value > d.maximum {
		value = d.maximum
	}
	return value
}

func (d *dateExtension) timeOnly() bool {
	return d.elements&elementsDate == 0
}

func (d *dateExtension) show(view *nativeView) {
	view.suppress++
	defer func() { view.suppress-- }()
	switch {
	case d.timeOnly():
		seconds := int(math.Mod(math.Mod(math.Floor(d.value), 86400)+86400, 86400))
		var now systemTime
		win32GetLocalTime(&now)
		now.wHour = uint16(seconds / 3600)
		now.wMinute = uint16((seconds / 60) % 60)
		now.wSecond = uint16(seconds % 60)
		now.wMilliseconds = 0
		win32SendMessage(view.outer, dtmSetSystemTime, gdtValid, uintptr(unsafe.Pointer(&now)))
	case d.calendar:
		date := systemDate(d.value)
		win32SendMessage(view.outer, mcmSetCurSel, 0, uintptr(unsafe.Pointer(&date)))
	default:
		date := systemDate(d.value)
		win32SendMessage(view.outer, dtmSetSystemTime, gdtValid, uintptr(unsafe.Pointer(&date)))
	}
}

func (d *dateExtension) changed(view *nativeView, value systemTime) {
	if view.suppress != 0 {
		return
	}
	var next float64
	if d.timeOnly() {
		next = float64(int(value.wHour)*3600 + int(value.wMinute)*60)
		if d.elements&elementsSeconds != 0 {
			next += float64(value.wSecond)
		}
	} else {
		next = d.clamp(secondsOf(value))
	}
	d.value = next
	view.report(changeEvent{})
}

func (d *dateExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	switch header.code {
	case dtnDateTimeChange:
		change := (*nmDateTimeChange)(unsafe.Pointer(header))
		if change.dwFlags == gdtValid {
			d.changed(view, change.st)
		}
		*result = 0
		return true
	case mcnSelChange:
		change := (*nmSelChange)(unsafe.Pointer(header))
		d.changed(view, change.stSelStart)
		*result = 0
		return true
	}
	return false
}

func (d *dateExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	if d.calendar {
		return calendarMinimum(view), true
	}
	width := float32(130)
	if d.timeOnly() {
		width = 110
	}
	return d2d1SizeF{width: width, height: float32(measureGDI("Ag", bodyFont(), 0, 0).cy + 8)}, true
}

// calendarMinimum returns the smallest size of a month calendar in logical
// units.
func calendarMinimum(view *nativeView) d2d1SizeF {
	var area rect
	win32SendMessage(view.outer, mcmGetMinReqRect, 0, uintptr(unsafe.Pointer(&area)))
	scale := view.scale()
	return d2d1SizeF{width: float32(float64(area.right) / scale), height: float32(float64(area.bottom) / scale)}
}

func dateOf(view *nativeView) *dateExtension {
	if view == nil || view.kind != kindDate {
		return nil
	}
	date, _ := view.ext.(*dateExtension)
	return date
}

func (win *nativeWindow) newDatePicker(style int, elements int, value float64, minimum float64, maximum float64, rangeMode int, interval float64, clock int) *nativeView {
	date := &dateExtension{elements: elements, value: value, minimum: minimum, maximum: maximum, clock: clock}
	var view *nativeView
	switch {
	case elements&elementsDate != 0 && style == 1:
		date.calendar = true
		view = viewNative(win, monthCalClass, "", wsTabStop|mcsNoTodayCircle, kindDate, date)
	case elements&elementsDate != 0:
		view = viewNative(win, dateTimePickClass, "", wsTabStop|dtsShortDateCenturyFormat, kindDate, date)
	default:
		view = viewNative(win, dateTimePickClass, "", wsTabStop|dtsTimeFormat|dtsUpDown, kindDate, date)
		if view != nil {
			twelve := clock == 1 || (clock == 0 && localeUses12Hour())
			format := "HH:mm"
			switch {
			case elements&elementsSeconds != 0 && twelve:
				format = "h:mm:ss tt"
			case elements&elementsSeconds != 0:
				format = "HH:mm:ss"
			case twelve:
				format = "h:mm tt"
			}
			win32SendMessage(view.outer, dtmSetFormat, 0, uintptr(unsafe.Pointer(wide(format))))
		}
	}
	if view == nil {
		return nil
	}
	if !date.timeOnly() && (minimum != 0 || maximum != 0) {
		limits := [2]systemTime{systemDate(minimum), systemDate(maximum)}
		var flags uintptr
		if minimum != 0 {
			flags |= gdtrMin
		}
		if maximum != 0 {
			flags |= gdtrMax
		}
		message := uint32(dtmSetRange)
		if date.calendar {
			message = mcmSetRange
		}
		win32SendMessage(view.outer, message, flags, uintptr(unsafe.Pointer(&limits[0])))
	}
	date.show(view)
	return view
}

func (view *nativeView) pickedDate() (value, interval float64) {
	if date := dateOf(view); date != nil {
		return date.value, 0
	}
	return 0, 0
}

func (view *nativeView) setPickedDate(value float64, interval float64) {
	if date := dateOf(view); date != nil {
		date.value = value
		date.show(view)
	}
}

// ---- Calendar ----
//
// The month calendar of Windows selects one date or a range. A set of dates is
// shown in bold, and a click adds or removes a date.

type calendarExtension struct {
	baseExtension
	selection int
	dates     []float64
	// dayStates holds the bold days that MCN_GETDAYSTATE hands to the
	// control, which reads them after the notification returns.
	dayStates [16]uint32
}

func (c *calendarExtension) apply(view *nativeView) {
	view.suppress++
	switch {
	case c.selection == 2 && len(c.dates) > 0:
		last := c.dates[0]
		if len(c.dates) > 1 {
			last = c.dates[1]
		}
		selection := [2]systemTime{systemDate(c.dates[0]), systemDate(last)}
		win32SendMessage(view.outer, mcmSetSelRange, 0, uintptr(unsafe.Pointer(&selection[0])))
	case c.selection == 0 && len(c.dates) > 0:
		date := systemDate(c.dates[0])
		win32SendMessage(view.outer, mcmSetCurSel, 0, uintptr(unsafe.Pointer(&date)))
	}
	win32InvalidateRect(view.outer, nil, true)
	view.suppress--
}

// boldDays fills the bold days of the months from start.
func (c *calendarExtension) boldDays(start systemTime, months []uint32) {
	month := start
	for i := range months {
		months[i] = 0
		for _, date := range c.dates {
			day := systemDate(date)
			if day.wYear == month.wYear && day.wMonth == month.wMonth {
				months[i] |= 1 << (day.wDay - 1)
			}
		}
		month.wMonth++
		if month.wMonth > 12 {
			month.wMonth = 1
			month.wYear++
		}
	}
}

func (c *calendarExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.code == mcnGetDayState && c.selection == 1 {
		state := (*nmDayState)(unsafe.Pointer(header))
		count := min(int(state.cDayState), len(c.dayStates))
		c.boldDays(state.stStart, c.dayStates[:count])
		state.prgDayState = &c.dayStates[0]
		*result = 0
		return true
	}
	if header.code == mcnSelect && view.suppress == 0 {
		change := (*nmSelChange)(unsafe.Pointer(header))
		seconds := secondsOf(change.stSelStart)
		switch c.selection {
		case 0:
			c.dates = []float64{seconds}
		case 2:
			last := secondsOf(change.stSelEnd)
			c.dates = []float64{seconds}
			if dayNumber(last) != dayNumber(seconds) {
				c.dates = append(c.dates, last)
			}
		default:
			found := slices.IndexFunc(c.dates, func(date float64) bool { return dayNumber(date) == dayNumber(seconds) })
			if found < 0 {
				c.dates = append(c.dates, seconds)
				slices.Sort(c.dates)
			} else {
				c.dates = slices.Delete(c.dates, found, found+1)
			}
			c.refreshBold(view)
		}
		view.report(changeEvent{})
		*result = 0
		return true
	}
	return false
}

func (c *calendarExtension) refreshBold(view *nativeView) {
	if c.selection != 1 {
		return
	}
	var visible [2]systemTime
	count := int(int32(win32SendMessage(view.outer, mcmGetMonthRange, gmrDayState, uintptr(unsafe.Pointer(&visible[0])))))
	months := make([]uint32, max(count, 1))
	c.boldDays(visible[0], months[:max(count, 0)])
	win32SendMessage(view.outer, mcmSetDayState, uintptr(max(count, 0)), uintptr(unsafe.Pointer(&months[0])))
}

func (c *calendarExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return calendarMinimum(view), true
}

func calendarOf(view *nativeView) *calendarExtension {
	if view == nil || view.kind != kindCalendar {
		return nil
	}
	calendar, _ := view.ext.(*calendarExtension)
	return calendar
}

func monthStart(seconds float64) float64 {
	date := time.Unix(int64(math.Floor(seconds)), 0).UTC()
	return float64(time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC).Unix())
}

// calendarVisible returns the first month that a calendar shows.
func calendarVisible(view *nativeView) float64 {
	var visible [2]systemTime
	win32SendMessage(view.outer, mcmGetMonthRange, gmrVisible, uintptr(unsafe.Pointer(&visible[0])))
	visible[0].wDay = 1
	return secondsOf(visible[0])
}

// calendarShowMonth turns the pages of a calendar to a month with its next and
// previous buttons, which do not change the selection.
func calendarShowMonth(view *nativeView, seconds float64) {
	target := monthStart(seconds)
	for range 2400 {
		visible := calendarVisible(view)
		if dayNumber(visible) == dayNumber(target) {
			break
		}
		info := mcGridInfo{dwFlags: mcgifRect, dwPart: mcgipPrev}
		info.cbSize = uint32(unsafe.Sizeof(info))
		if visible < target {
			info.dwPart = mcgipNext
		}
		if win32SendMessage(view.outer, mcmGetCalendarGridInfo, 0, uintptr(unsafe.Pointer(&info))) == 0 {
			break
		}
		at := makeWParam(int((info.rc.left+info.rc.right)/2), int((info.rc.top+info.rc.bottom)/2))
		view.suppress++
		win32SendMessage(view.outer, wmLButtonDown, mkLButton, at)
		win32SendMessage(view.outer, wmLButtonUp, 0, at)
		view.suppress--
		if dayNumber(calendarVisible(view)) == dayNumber(visible) {
			break
		}
	}
}

func (win *nativeWindow) newCalendar(selection int, dates []float64, visible float64) *nativeView {
	state := &calendarExtension{selection: selection}
	style := uint32(wsTabStop | mcsNoTodayCircle)
	switch selection {
	case 2:
		style |= mcsMultiSelect
	case 1:
		style |= mcsDayState
	}
	view := viewNative(win, monthCalClass, "", style, kindCalendar, state)
	if view == nil {
		return nil
	}
	if selection == 2 {
		win32SendMessage(view.outer, mcmSetMaxSelCount, 3660, 0)
	}
	view.setCalendarDates(dates)
	if visible != 0 {
		view.setVisibleMonth(visible)
	}
	return view
}

func (view *nativeView) calendarDates() []float64 {
	if state := calendarOf(view); state != nil {
		return slices.Clone(state.dates)
	}
	return nil
}

func (view *nativeView) setCalendarDates(dates []float64) {
	state := calendarOf(view)
	if state == nil {
		return
	}
	state.dates = slices.Clone(dates)
	slices.Sort(state.dates)
	state.apply(view)
	state.refreshBold(view)
	if len(dates) > 0 {
		calendarShowMonth(view, dates[0])
	}
}

func (view *nativeView) visibleMonth() float64 {
	if calendarOf(view) == nil {
		return 0
	}
	return calendarVisible(view)
}

func (view *nativeView) setVisibleMonth(month float64) {
	if state := calendarOf(view); state != nil {
		calendarShowMonth(view, month)
		state.refreshBold(view)
	}
}

// ---- Color ----
//
// A color well is a button that shows the color and opens the color dialog of
// Windows. The dialog has no opacity, so for a color well that allows it, the
// dialog gets an opacity row: a trackbar and its value.

type opacityRow struct {
	alpha  float64
	slider hwnd
	value  hwnd
}

// opacityRows holds the opacity rows of the color dialogs that show, by the
// identifier that the dialog receives.
var (
	opacityRows    = map[uintptr]*opacityRow{}
	nextOpacityRow uintptr
)

func (o *opacityRow) show() {
	win32SetWindowText(o.value, fmt.Sprintf("%d%%", int(math.Round(o.alpha*100))))
}

func opacityHook(dialog hwnd, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case wmInitDialog:
		choose := (*chooseColor)(memoryAt(lparam))
		opacity := opacityRows[choose.lCustData]
		if opacity == nil {
			return 0
		}
		win32SetProp(dialog, "GKOpacity", choose.lCustData)
		font := win32SendMessage(dialog, wmGetFont, 0, 0)
		var client rect
		win32GetClientRect(dialog, &client)
		// The row goes below the controls of the dialog, which grows to hold
		// it, with the margins of the dialog.
		units := rect{left: 7, top: 4, right: 40, bottom: 14}
		win32MapDialogRect(dialog, &units)
		margin, gap, labelWidth, height := units.left, units.top, units.right, units.bottom
		row := height + gap
		var frame rect
		win32GetWindowRect(dialog, &frame)
		win32SetWindowPos(dialog, 0, 0, 0, frame.right-frame.left, frame.bottom-frame.top+row+gap,
			swpNoMove|swpNoZOrder|swpNoActivate)
		top := client.bottom + gap/2
		label := win32CreateWindowEx(0, wcStatic, "&Opacity:", wsChild|wsVisible|ssLeft|ssCenterImage, margin, top,
			labelWidth, height, dialog, 0, instance(), nil)
		valueWidth := labelWidth
		opacity.slider = win32CreateWindowEx(0, trackbarClass, "Opacity", wsChild|wsVisible|wsTabStop|tbsHorz|tbsNoTicks,
			margin+labelWidth, top, client.right-2*margin-labelWidth-valueWidth, height, dialog, 0, instance(), nil)
		opacity.value = win32CreateWindowEx(0, wcStatic, "", wsChild|wsVisible|ssRight|ssCenterImage,
			client.right-margin-valueWidth, top, valueWidth, height, dialog, 0, instance(), nil)
		win32SendMessage(label, wmSetFont, font, 0)
		win32SendMessage(opacity.value, wmSetFont, font, 0)
		win32SendMessage(opacity.slider, tbmSetRange, 0, makeWParam(0, 100))
		win32SendMessage(opacity.slider, tbmSetPageSize, 0, 10)
		win32SendMessage(opacity.slider, tbmSetPos, 1, uintptr(math.Round(opacity.alpha*100)))
		opacity.show()
		return 0
	case wmHScroll:
		opacity := opacityRows[win32GetProp(dialog, "GKOpacity")]
		if opacity != nil && hwnd(lparam) == opacity.slider {
			opacity.alpha = float64(win32SendMessage(opacity.slider, tbmGetPos, 0, 0)) / 100
			opacity.show()
			return 1
		}
	case wmDestroy:
		win32RemoveProp(dialog, "GKOpacity")
	}
	return 0
}

type colorWellExtension struct {
	baseExtension
	rgba        [4]float64
	allowsAlpha bool
	images      himagelist
	custom      [16]uint32
}

func (c *colorWellExtension) free() {
	if c.images != 0 {
		win32ImageListDestroy(c.images)
	}
}

func (c *colorWellExtension) show(view *nativeView) {
	scale := view.scale()
	width, height := int(math.Round(32*scale)), int(math.Round(14*scale))
	if c.images != 0 {
		win32ImageListDestroy(c.images)
	}
	c.images = win32ImageListCreate(int32(width), int32(height), ilcColor32, 1, 0)
	border := view.palette().controlStrong
	pixels := make([]byte, width*height*4)
	for y := range height {
		for x := range width {
			p := pixels[(y*width+x)*4:]
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				p[0] = byte(border.b * 255)
				p[1] = byte(border.g * 255)
				p[2] = byte(border.r * 255)
				p[3] = 255
				continue
			}
			// A translucent color shows over a checkerboard.
			checker := 1.0
			if (x/4+y/4)%2 != 0 {
				checker = 0.8
			}
			a := c.rgba[3]
			p[0] = byte(math.Round((c.rgba[2]*a + checker*(1-a)) * 255))
			p[1] = byte(math.Round((c.rgba[1]*a + checker*(1-a)) * 255))
			p[2] = byte(math.Round((c.rgba[0]*a + checker*(1-a)) * 255))
			p[3] = 255
		}
	}
	info := bitmapInfo{bmiHeader: bitmapInfoHeader{biWidth: int32(width), biHeight: -int32(height), biPlanes: 1,
		biBitCount: 32}}
	info.bmiHeader.biSize = uint32(unsafe.Sizeof(info.bmiHeader))
	var bits uintptr
	screen := win32GetDC(0)
	bitmap := win32CreateDIBSection(screen, unsafe.Pointer(&info), dibRGBColors, &bits, 0, 0)
	win32ReleaseDC(0, screen)
	if bitmap != 0 {
		copy(unsafe.Slice((*byte)(memoryAt(bits)), len(pixels)), pixels)
		win32ImageListAdd(c.images, bitmap, 0)
		win32DeleteObject(hgdiobj(bitmap))
	}
	list := buttonImageList{himl: c.images, uAlign: buttonImageListAlignCenter}
	win32SendMessage(view.outer, bcmSetImageList, 0, uintptr(unsafe.Pointer(&list)))
	view.accessibleValue = fmt.Sprintf("#%02X%02X%02X%02X", int(math.Round(c.rgba[0]*255)),
		int(math.Round(c.rgba[1]*255)), int(math.Round(c.rgba[2]*255)), int(math.Round(c.rgba[3]*255)))
	win32InvalidateRect(view.outer, nil, true)
}

func (c *colorWellExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	choose := chooseColor{
		hwndOwner:    win32GetAncestor(view.outer, gaRoot),
		rgbResult:    themeColor{float32(c.rgba[0]), float32(c.rgba[1]), float32(c.rgba[2]), 1}.colorRef(),
		lpCustColors: &c.custom[0],
		Flags:        ccFullOpen | ccRGBInit | ccAnyColor,
	}
	choose.lStructSize = uint32(unsafe.Sizeof(choose))
	opacity := &opacityRow{alpha: c.rgba[3]}
	if c.allowsAlpha {
		nextOpacityRow++
		id := nextOpacityRow
		opacityRows[id] = opacity
		defer delete(opacityRows, id)
		choose.Flags |= ccEnableHook
		choose.lpfnHook = callback("opacityHook", opacityHook)
		choose.lCustData = id
	}
	cookie := activate()
	chosen := win32ChooseColor(&choose)
	deactivate(cookie)
	if chosen {
		c.rgba[0] = float64(choose.rgbResult&0xff) / 255
		c.rgba[1] = float64((choose.rgbResult>>8)&0xff) / 255
		c.rgba[2] = float64((choose.rgbResult>>16)&0xff) / 255
		if c.allowsAlpha {
			c.rgba[3] = opacity.alpha
		}
		c.show(view)
		view.report(changeEvent{})
	}
	return true
}

func (c *colorWellExtension) themeChanged(view *nativeView) {
	c.show(view)
}

func (c *colorWellExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: 56, height: float32(measureGDI("Ag", bodyFont(), 0, 0).cy + 8)}, true
}

func colorWellOf(view *nativeView) *colorWellExtension {
	if view == nil || view.kind != kindColor {
		return nil
	}
	well, _ := view.ext.(*colorWellExtension)
	return well
}

func (win *nativeWindow) newColorWell(r float64, g float64, b float64, a float64, allowsAlpha bool) *nativeView {
	well := &colorWellExtension{rgba: [4]float64{r, g, b, 1}, allowsAlpha: allowsAlpha}
	if allowsAlpha {
		well.rgba[3] = a
	}
	view := viewNative(win, wcButton, "", bsPushButton|wsTabStop, kindColor, well)
	if view == nil {
		return nil
	}
	view.text = "Color"
	well.show(view)
	return view
}

func (view *nativeView) colorWellColor() [4]float64 {
	if well := colorWellOf(view); well != nil {
		return well.rgba
	}
	return [4]float64{0, 0, 0, 1}
}

func (view *nativeView) setColorWellColor(r float64, g float64, b float64, a float64) {
	well := colorWellOf(view)
	if well == nil {
		return
	}
	well.rgba = [4]float64{r, g, b, 1}
	if well.allowsAlpha {
		well.rgba[3] = a
	}
	well.show(view)
}

// ---- Font ----
//
// A font picker is a button that shows the font and opens the font dialog of
// Windows.

type fontPickerExtension struct {
	baseExtension
	family string
	system bool
	size   float64
	weight int
	italic bool
}

func (f *fontPickerExtension) show(view *nativeView) {
	name := f.family
	if f.system {
		name = "System"
	}
	text := fmt.Sprintf("%s %g", name, f.size)
	view.text = text
	win32SetWindowText(view.outer, text)
	view.accessibleValue = text
}

func (f *fontPickerExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	var font logFont
	screen := win32GetDC(0)
	font.lfHeight = -win32MulDiv(int32(math.Round(f.size)), win32GetDeviceCaps(screen, logPixelsY), 72)
	win32ReleaseDC(0, screen)
	font.lfWeight = int32(f.weight)
	if f.italic {
		font.lfItalic = 1
	}
	name := f.family
	if f.system {
		name = bodyFont().family
	}
	face := utf16.Encode([]rune(name))
	copy(font.lfFaceName[:len(font.lfFaceName)-1], face)
	choose := chooseFont{
		hwndOwner: win32GetAncestor(view.outer, gaRoot),
		lpLogFont: &font,
		Flags:     cfScreenFonts | cfInitToLogFontStruct | cfNoVertFonts | cfNoScriptSel,
	}
	choose.lStructSize = uint32(unsafe.Sizeof(choose))
	if win32ChooseFont(&choose) {
		f.family = goString(font.lfFaceName[:])
		f.system = false
		f.size = float64(choose.iPointSize) / 10
		f.weight = int(font.lfWeight)
		if f.weight == 0 {
			f.weight = 400
		}
		f.italic = font.lfItalic != 0
		f.show(view)
		view.report(changeEvent{})
	}
	return true
}

func (f *fontPickerExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	text := measureGDI(view.text, bodyFont(), 0, 0)
	return d2d1SizeF{width: float32(text.cx + 24), height: float32(measureGDI("Ag", bodyFont(), 0, 0).cy + 8)}, true
}

func (win *nativeWindow) newFontPicker(family string, size float64, weight int, italic bool) *nativeView {
	view := viewNative(win, wcButton, "", bsPushButton|wsTabStop, kindFont, &fontPickerExtension{})
	if view == nil {
		return nil
	}
	view.setPickedFont(family, size, weight, italic)
	return view
}

func (view *nativeView) setPickedFont(family string, size float64, weight int, italic bool) {
	if !view.live() || view.kind != kindFont {
		return
	}
	if family != "" && family != "System" && family != "Monospace" && !fontFamilyInstalled(family) {
		return
	}
	state := view.ext.(*fontPickerExtension)
	state.system = family == "" || family == "System"
	switch {
	case state.system:
		state.family = bodyFont().family
	case family == "Monospace":
		state.family = "Consolas"
	default:
		state.family = family
	}
	state.size = size
	if size <= 0 {
		state.size = 12
	}
	state.weight = weight
	if weight <= 0 {
		state.weight = 400
	}
	state.italic = italic
	state.show(view)
}

// pickedFont returns the font of a font picker: its family,
// "System" for the system font, and its size, weight, and slant.
func (view *nativeView) pickedFont() (family string, size float64, weight int, italic bool) {
	if !view.live() || view.kind != kindFont {
		return "System", 0, 400, false
	}
	state := view.ext.(*fontPickerExtension)
	family = state.family
	if state.system {
		family = "System"
	}
	return family, state.size, state.weight, state.italic
}

// ---- Key recorder ----
//
// A key recorder is a button that records the next shortcut that the user
// types after clicking it.

type recorderExtension struct {
	baseExtension
	display   string
	recording bool
}

func (r *recorderExtension) update(view *nativeView) {
	text := "Record Shortcut"
	switch {
	case r.recording:
		text = "Type shortcut…"
	case r.display != "":
		text = r.display
	}
	view.text = text
	win32SetWindowText(view.outer, text)
	if view.accessibleName == "" {
		view.accessibleName = "Shortcut"
	}
	view.accessibleValue = r.display
}

func (r *recorderExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	r.recording = true
	win32SetFocus(view.outer)
	r.update(view)
	return true
}

func (r *recorderExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if message == wmKillFocus && r.recording {
		r.recording = false
		r.update(view)
		return false
	}
	if message == wmGetDlgCode && r.recording {
		*result = dlgcWantAllKeys
		return true
	}
	if (message != wmKeyDown && message != wmSysKeyDown) || !r.recording {
		return message == wmChar && r.recording
	}
	key := wparam
	*result = 0
	if key == vkShift || key == vkControl || key == vkMenu || key == vkLWin || key == vkRWin {
		return true
	}
	mods := modifiers()
	r.recording = false
	if key == vkEscape && mods&(modControl|modAlt) == 0 {
		r.update(view)
		return true
	}
	if (key == vkBack || key == vkDelete) && mods&(modControl|modAlt|modShift|modSuper) == 0 {
		r.update(view)
		send(view.receiver, changeEvent{})
		return true
	}
	bits := 0
	if mods&modControl != 0 {
		bits |= 1
	}
	if mods&modSuper != 0 {
		bits |= 2
	}
	if mods&modAlt != 0 {
		bits |= 4
	}
	if mods&modShift != 0 {
		bits |= 8
	}
	var text string
	if character := win32MapVirtualKey(uint32(key), mapvkVkToChar) & 0x7fffffff; character >= 0x20 {
		text = string(unicode.ToUpper(rune(character)))
	}
	send(view.receiver, keyRecordEvent{modifiers: bits, key: keyFromVirtualKey(wparam, lparam), label: text})
	r.update(view)
	return true
}

func (r *recorderExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	text := measureGDI("Record Shortcut", bodyFont(), 0, 0)
	return d2d1SizeF{width: float32(text.cx + 24), height: float32(measureGDI("Ag", bodyFont(), 0, 0).cy + 8)}, true
}

func (win *nativeWindow) newKeyRecorder() *nativeView {
	recorder := &recorderExtension{}
	view := viewNative(win, wcButton, "", bsPushButton|wsTabStop, kindKeyRecorder, recorder)
	if view == nil {
		return nil
	}
	recorder.update(view)
	return view
}

func (view *nativeView) setRecordedKey(display string) {
	if !view.live() || view.kind != kindKeyRecorder {
		return
	}
	recorder := view.ext.(*recorderExtension)
	recorder.display = display
	recorder.update(view)
}
