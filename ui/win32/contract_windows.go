package win32

import (
	"fmt"
	stdimage "image"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Accessibility ----
//
// Native controls report their role and state to assistive technology through
// Windows. The backend adds the name and the description that the application
// sets with dynamic annotation, which overrides what the control reports.

var (
	accPropServices      comObject
	accPropServicesTried bool
)

func accProps() comObject {
	if !accPropServicesTried {
		accPropServicesTried = true
		win32CoCreateInstance(clsidAccPropServices, 0, clsctxInprocServer, iidAccPropServices, &accPropServices)
	}
	return accPropServices
}

func annotate(h hwnd, property *guid, value string) {
	services := accProps()
	if services == 0 || h == 0 {
		return
	}
	if value == "" {
		win32AccPropServicesClearHwndProps(services, h, objidClient, childidSelf, property, 1)
		return
	}
	win32AccPropServicesSetHwndPropStr(services, h, objidClient, childidSelf, property, value)
}

func (view *nativeView) setAccessibility(identifier string, name string, description string, value string, required bool, hidden bool, invalid bool, describedBy []*nativeView) {
	if !view.live() {
		return
	}
	view.accessibleID = identifier
	view.accessibleName = name
	view.accessibleDescription = description
	view.accessibleValue = value
	view.accessibleRequired = required
	view.accessibleHidden = hidden
	view.accessibleInvalid = invalid
	described := description
	for _, related := range describedBy {
		if !related.live() {
			continue
		}
		text := related.accessibleName
		if text == "" {
			text = related.text
		}
		if text != "" {
			if described != "" {
				described += " "
			}
			described += text
		}
	}
	target := view.target()
	annotate(target, propidAccName, view.accessibleName)
	annotate(target, propidAccDescription, described)
	if view.accessibleValue != "" {
		annotate(target, propidAccValue, view.accessibleValue)
	}
}

// ---- Accessibility tree ----
//
// The tree comes from the views of the window: each view reports the UI
// Automation control type of its control and the name of its component, which
// the Go layer maps to roles.

func controlType(view *nativeView) string {
	switch view.kind {
	case kindLabel, kindValidation:
		return "Text"
	case kindButton, kindToggle, kindColor, kindFont, kindKeyRecorder:
		return "Button"
	case kindTextField, kindTextArea, kindPassword, kindSearch:
		return "Edit"
	case kindNumber, kindStepper:
		return "Spinner"
	case kindCheckBox, kindSwitch:
		return "CheckBox"
	case kindSelect, kindCombo, kindDate:
		return "ComboBox"
	case kindSlider, kindDial, kindRange:
		return "Slider"
	case kindProgress, kindActivity:
		return "ProgressBar"
	case kindSeparator:
		return "Separator"
	case kindImage, kindIcon, kindVector, kindAnimation:
		return "Image"
	case kindLink:
		return "Hyperlink"
	case kindTabs:
		return "Tab"
	case kindTable:
		return "Table"
	case kindOutline:
		return "Tree"
	case kindCollection:
		return "List"
	case kindStatusBar:
		return "StatusBar"
	case kindCommandBar, kindToolbar:
		return "ToolBar"
	case kindDocument:
		return "Document"
	case kindCalendar:
		return "Calendar"
	default:
		return "Group"
	}
}

func componentName(view *nativeView) string {
	switch view.kind {
	case kindPassword:
		return "password"
	case kindTextArea:
		return "textarea"
	case kindActivity:
		return "busy"
	case kindSection:
		return "disclosure"
	case kindScroll:
		return "scroll"
	case kindCanvas:
		return "canvas"
	case kindDate:
		return "date"
	case kindCalendar:
		return "calendar"
	case kindColor:
		return "color"
	case kindFont:
		return "font"
	case kindSplit:
		return "split"
	case kindWeb:
		return "web"
	case kindVideo:
		return "video"
	case kindDocument:
		return "document"
	case kindStatusBar, kindBadge:
		return "status"
	case kindSwitch:
		return "switch"
	case kindToggle:
		return "toggle"
	case kindSearch:
		return "search"
	case kindNumber, kindStepper:
		return "spin"
	case kindRadioGroup, kindSegmented:
		return "radiogroup"
	case kindDial, kindRange:
		return "slider"
	case kindSeparator:
		return "separator"
	case kindTabs:
		return "tablist"
	case kindBreadcrumbs, kindPagination:
		return "navigation"
	case kindInfoBar:
		return "alert"
	case kindLayout, kindClip, kindGroupBox:
		return "group"
	default:
		return ""
	}
}

// treeText replaces the separators of the tree in a value.
func treeText(value string) string {
	return strings.Map(func(r rune) rune {
		if r == 0x1e || r == 0x1f {
			return ' '
		}
		return r
	}, value)
}

func viewValueText(view *nativeView) string {
	if view.accessibleValue != "" {
		return view.accessibleValue
	}
	switch view.kind {
	case kindTextField, kindTextArea, kindSearch, kindCombo, kindSelect:
		return view.currentText()
	case kindCheckBox, kindToggle, kindSwitch:
		if view.isChecked() {
			return "1"
		}
		return "0"
	case kindSlider, kindProgress, kindNumber, kindStepper, kindDial:
		return strconv.FormatFloat(view.value, 'g', 6, 64)
	default:
		return ""
	}
}

func viewName(view *nativeView) string {
	if view.accessibleName != "" {
		return view.accessibleName
	}
	switch view.kind {
	case kindLabel, kindButton, kindToggle, kindCheckBox, kindSwitch, kindLink, kindSection, kindGroupBox,
		kindStatusBar, kindBadge, kindValidation:
		return view.text
	default:
		return ""
	}
}

type treeWriter struct {
	out   strings.Builder
	count int
	root  hwnd
	scale float64
}

// accessibleItem is a part of a view that is an item for assistive technology,
// such as a button of a radio group.
type accessibleItem struct {
	controlType string
	name        string
}

func viewItems(view *nativeView) []accessibleItem {
	switch view.kind {
	case kindSegmented:
		return segmentItems(view)
	case kindTabs:
		return tabItems(view)
	case kindBreadcrumbs:
		return breadcrumbItems(view)
	case kindCommandBar, kindToolbar:
		return toolbarItems(view)
	}
	var items []accessibleItem
	for _, button := range radioButtons(view) {
		items = append(items, accessibleItem{"RadioButton", windowText(button)})
	}
	return items
}

func (w *treeWriter) record(depth int, kind, component, id, name, value string, bounds rect) {
	w.count++
	fmt.Fprintf(&w.out, "%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s", depth, treeText(kind), treeText(component),
		treeText(id), treeText(name), treeText(value))
	fmt.Fprintf(&w.out, "\x1f%.0f\x1f%.0f\x1f%.0f\x1f%.0f\x1e", float64(bounds.left)/w.scale,
		float64(bounds.top)/w.scale, float64(bounds.right-bounds.left)/w.scale,
		float64(bounds.bottom-bounds.top)/w.scale)
}

func (w *treeWriter) children(parent hwnd, depth int) {
	var children []hwnd
	for child := win32GetWindow(parent, gwChild); child != 0; child = win32GetWindow(child, gwHwndNext) {
		children = append(children, child)
	}
	// Reading order: top to bottom, then leading to trailing.
	slices.SortStableFunc(children, func(a, b hwnd) int {
		var ra, rb rect
		win32GetWindowRect(a, &ra)
		win32GetWindowRect(b, &rb)
		if ra.top != rb.top {
			return int(ra.top - rb.top)
		}
		return int(ra.left - rb.left)
	})
	for _, child := range children {
		w.window(child, depth)
	}
}

func (w *treeWriter) window(h hwnd, depth int) {
	if depth > 64 || w.count > 20000 {
		return
	}
	view := viewOf(h)
	if view != nil && view.owner != nil {
		return
	}
	if win32GetWindowLongPtr(h, gwlStyle)&wsVisible == 0 {
		return
	}
	if view == nil || view.outer != h {
		// A layout or part without a view of its own: its children belong to
		// the nearest view.
		if view == nil {
			w.children(h, depth)
		}
		return
	}
	if view.accessibleHidden {
		return
	}
	var bounds rect
	win32GetWindowRect(h, &bounds)
	mapWindowRect(0, w.root, &bounds)
	w.record(depth, controlType(view), componentName(view), view.accessibleID, viewName(view), viewValueText(view),
		bounds)
	for _, item := range viewItems(view) {
		empty := rect{left: bounds.left, top: bounds.top, right: bounds.left, bottom: bounds.top}
		w.record(depth+1, item.controlType, "", "", item.name, "", empty)
	}
	// The children of a container are in its body, or are its own child
	// windows, such as the panes of a split view. The parts of a native
	// control have no views, so they add nothing.
	w.children(view.container(), depth+1)
}

// accessibilityTree returns the accessibility tree of a window: records
// separated by 0x1e of fields separated by 0x1f.
func (win *nativeWindow) accessibilityTree() string {
	if win == nil || win.hwnd == 0 {
		return ""
	}
	writer := treeWriter{root: win.root, scale: win.scale()}
	width, height := win.size()
	fmt.Fprintf(&writer.out, "0\x1fWindow\x1f\x1f\x1f%s\x1f\x1f0\x1f0\x1f%d\x1f%d\x1e", treeText(windowText(win.hwnd)),
		width, height)
	for _, bar := range win.chrome {
		if bar.view != nil && bar.top {
			writer.window(bar.view.outer, 1)
		}
	}
	writer.children(win.root, 1)
	for _, bar := range win.chrome {
		if bar.view != nil && !bar.top {
			writer.window(bar.view.outer, 1)
		}
	}
	return writer.out.String()
}

// ---- Focus ----

func focusable(h hwnd) bool {
	return win32GetWindowLongPtr(h, gwlStyle)&wsTabStop != 0 && win32IsWindowEnabled(h) && win32IsWindowVisible(h)
}

func firstFocusable(parent hwnd) hwnd {
	for child := win32GetWindow(parent, gwChild); child != 0; child = win32GetWindow(child, gwHwndNext) {
		if focusable(child) {
			return child
		}
		if inner := firstFocusable(child); inner != 0 {
			return inner
		}
	}
	return 0
}

func (view *nativeView) focus() bool {
	if !view.live() {
		return false
	}
	target := view.target()
	if target != 0 && win32GetWindowLongPtr(target, gwlStyle)&wsTabStop == 0 {
		target = firstFocusable(view.outer)
	}
	if target == 0 || !win32IsWindowEnabled(target) {
		return false
	}
	root := win32GetAncestor(target, gaRoot)
	if !win32IsWindowVisible(root) || !win32IsWindowVisible(target) {
		return false
	}
	win32SetFocus(target)
	return win32GetFocus() == target
}

func (view *nativeView) isFocused() bool {
	if !view.live() {
		return false
	}
	focus := win32GetFocus()
	return focus != 0 && (focus == view.outer || win32IsChild(view.outer, focus))
}

// ---- Tooltips ----

var tooltipControl hwnd

// tooltipWindow returns the tooltip control of the application, which shows
// the tooltips of every view.
func tooltipWindow() hwnd {
	if tooltipControl == 0 || !win32IsWindow(tooltipControl) {
		cookie := activate()
		tooltipControl = win32CreateWindowEx(wsExTopmost|wsExTransparent, tooltipsClass, "", wsPopup|ttsAlwaysTip|ttsNoPrefix,
			cwUseDefault, cwUseDefault, cwUseDefault, cwUseDefault, 0, 0, instance(), nil)
		deactivate(cookie)
		win32SendMessage(tooltipControl, ttmSetMaxTipWidth, 0, 400)
	}
	return tooltipControl
}

func tooltipFor(h hwnd, text string) {
	if h == 0 {
		return
	}
	tooltip := tooltipWindow()
	info := toolInfo{uFlags: ttfIdIsHwnd | ttfSubclass, hwnd: win32GetParent(h), uId: uintptr(h)}
	info.cbSize = uint32(unsafe.Sizeof(info))
	win32SendMessage(tooltip, ttmDelTool, 0, uintptr(unsafe.Pointer(&info)))
	if text == "" {
		return
	}
	info.lpszText = wide(text)
	win32SendMessage(tooltip, ttmAddTool, 0, uintptr(unsafe.Pointer(&info)))
}

func (view *nativeView) setTooltip(text string) {
	if !view.live() {
		return
	}
	view.tooltip = text
	tooltipFor(view.outer, text)
	if view.control != 0 && view.control != view.outer {
		tooltipFor(view.control, text)
	}
}

// ---- Cursors ----

// cursorShapes are the cursors of ui.Cursor. Windows has no grab cursors; a hand
// is the Windows cursor for an object that the pointer can take and move.
var cursorShapes = []uintptr{idcArrow, idcHand, idcIBeam, idcCross, idcSizeAll, idcSizeWE, idcSizeNS, idcNo,
	idcWait, idcHand, idcSizeAll}

// setCursorShape sets the cursor of the pointer.
func setCursorShape(cursor int) {
	if cursor < 0 || cursor >= len(cursorShapes) {
		cursor = 0
	}
	win32SetCursor(win32LoadCursor(0, cursorShapes[cursor]))
}

// cursorSubclass sets the cursor of a view over its windows. data is the view's
// id.
func cursorSubclass() uintptr {
	return callback("cursorSubclass", func(h hwnd, message uint32, wparam, lparam, id, data uintptr) uintptr {
		if message == wmSetCursor && loword(lparam) == htClient {
			if view := views[data]; view != nil && view.cursor > 0 {
				setCursorShape(view.cursor)
				return 1
			}
		}
		if message == wmNCDestroy {
			win32RemoveWindowSubclass(h, cursorSubclass(), 2)
		}
		return win32DefSubclassProc(h, message, wparam, lparam)
	})
}

func (view *nativeView) setCursor(cursor int) {
	if !view.live() {
		return
	}
	view.cursor = cursor
	for _, h := range []hwnd{view.outer, view.control} {
		if h != 0 {
			win32SetWindowSubclass(h, cursorSubclass(), 2, view.id)
		}
	}
}

// ---- Style ----

// resolveStyle applies the colors of a view's style in the theme of its
// window.
func (v *nativeView) resolveStyle() {
	style := &v.style
	pal := v.palette()
	if style.foregroundRole != 0 {
		style.foreground, style.hasForeground = semanticColor(pal, style.foregroundRole)
	}
	if style.backgroundRole != 0 {
		style.background, style.hasBackground = semanticColor(pal, style.backgroundRole)
	}
	if v.kind == kindLayout || v.kind == kindClip {
		if style.hasBackground {
			color := mix(backgroundOf(win32GetParent(v.outer)), style.background)
			setLayoutBackground(v.container(), &color)
		} else {
			setLayoutBackground(v.container(), nil)
		}
	}
	if v.background != 0 {
		win32DeleteObject(hgdiobj(v.background))
		v.background = 0
	}
}

// setStyle sets the style of a view. A nil color uses the semantic color
// of its role. It reports false for a font family that is not installed.
func (view *nativeView) setStyle(family string, size float64, weight int, italic bool, foreground int, background int, foregroundRGBA *[4]float64, backgroundRGBA *[4]float64, cornerRadius float64) bool {
	if !view.live() {
		return false
	}
	if family != "" && family != "System" && family != "Monospace" && !fontFamilyInstalled(family) {
		return false
	}
	style := &view.style
	switch family {
	case "Monospace":
		style.family = "Consolas"
	case "System":
		style.family = ""
	default:
		style.family = family
	}
	style.size = max(size, 0)
	style.weight = max(weight, 0)
	style.italic = italic
	style.cornerRadius = max(cornerRadius, 0)
	style.hasForeground = false
	style.foregroundRole = 0
	if foregroundRGBA != nil {
		style.hasForeground = true
		style.foreground = themeColor{float32(foregroundRGBA[0]), float32(foregroundRGBA[1]),
			float32(foregroundRGBA[2]), float32(foregroundRGBA[3])}
	} else {
		style.foregroundRole = foreground
	}
	style.hasBackground = false
	style.backgroundRole = 0
	if backgroundRGBA != nil {
		style.hasBackground = true
		style.background = themeColor{float32(backgroundRGBA[0]), float32(backgroundRGBA[1]),
			float32(backgroundRGBA[2]), float32(backgroundRGBA[3])}
	} else {
		style.backgroundRole = background
	}
	view.resolveStyle()
	view.applyFont()
	if view.ext != nil {
		view.ext.themeChanged(view)
	}
	view.invalidate()
	return true
}

// ---- Context menus ----

// menuSubclass shows the context menu of a view. data is the view's id.
func menuSubclass() uintptr {
	return callback("menuSubclass", func(h hwnd, message uint32, wparam, lparam, id, data uintptr) uintptr {
		if message == wmContextMenu {
			if view := views[data]; view != nil && view.contextMenu != nil {
				at := point{x: int32(signedLoword(lparam)), y: int32(signedHiword(lparam))}
				if at.x == -1 && at.y == -1 {
					var bounds rect
					win32GetWindowRect(h, &bounds)
					at = point{x: bounds.left + 8, y: bounds.top + 8}
				}
				menuPopupAt(view.contextMenu, h, at)
				return 0
			}
		}
		if message == wmNCDestroy {
			win32RemoveWindowSubclass(h, menuSubclass(), 3)
		}
		return win32DefSubclassProc(h, message, wparam, lparam)
	})
}

func (view *nativeView) setMenu(menu *nativeMenu) {
	if !view.live() {
		return
	}
	view.contextMenu = menu
	for _, h := range []hwnd{view.outer, view.control} {
		if h != 0 {
			win32SetWindowSubclass(h, menuSubclass(), 3, view.id)
		}
	}
}

// ---- Data objects ----

// dataItem is data in a clipboard format.
type dataItem struct {
	format formatEtc
	memory hglobal
}

// dataObject holds data in clipboard formats for drag and drop. It implements
// IDataObject.
type dataObject struct {
	items []dataItem
}

var dataObjectTable []uintptr

func newDataObject() (*comServer, *dataObject) {
	if dataObjectTable == nil {
		dataObjectTable = methodTable(
			callback("IDataObject.GetData", serveDataGet),
			callback("IDataObject.GetDataHere", func(this, format, medium uintptr) uintptr { return eNotImpl }),
			callback("IDataObject.QueryGetData", serveDataQueryGet),
			callback("IDataObject.GetCanonicalFormatEtc", func(this, format, out uintptr) uintptr {
				(*formatEtc)(memoryAt(out)).ptd = 0
				return eNotImpl
			}),
			callback("IDataObject.SetData", func(this, format, medium, release uintptr) uintptr { return eNotImpl }),
			callback("IDataObject.EnumFormatEtc", serveDataEnumFormatEtc),
			callback("IDataObject.DAdvise", func(this, format, flags, sink, connection uintptr) uintptr {
				return oleEAdviseNotSupported
			}),
			callback("IDataObject.DUnadvise", func(this, connection uintptr) uintptr { return oleEAdviseNotSupported }),
			callback("IDataObject.EnumDAdvise", func(this, result uintptr) uintptr { return oleEAdviseNotSupported }),
		)
	}
	data := &dataObject{}
	server := newCOMServer(dataObjectTable, []*guid{iidDataObject}, data)
	server.release = func() {
		for _, item := range data.items {
			win32GlobalFree(item.memory)
		}
	}
	return server, data
}

// add adds data in a format.
func (d *dataObject) add(format uint16, data []byte) {
	memory := win32GlobalAlloc(gmemMoveable, uintptr(max(len(data), 1)))
	if memory == 0 {
		return
	}
	if len(data) > 0 {
		target := win32GlobalLock(memory)
		copy(unsafe.Slice((*byte)(memoryAt(target)), len(data)), data)
		win32GlobalUnlock(memory)
	}
	d.items = append(d.items, dataItem{
		format: formatEtc{cfFormat: format, dwAspect: dvaspectContent, lindex: -1, tymed: tymedHGlobal},
		memory: memory,
	})
}

func (d *dataObject) find(format *formatEtc) *dataItem {
	for i := range d.items {
		if d.items[i].format.cfFormat == format.cfFormat && format.tymed&tymedHGlobal != 0 {
			return &d.items[i]
		}
	}
	return nil
}

func serveDataGet(this, format, medium uintptr) uintptr {
	data, ok := serverObject[*dataObject](this)
	if !ok {
		return eUnexpected
	}
	item := data.find((*formatEtc)(memoryAt(format)))
	if item == nil {
		return dvEFormatEtc
	}
	size := win32GlobalSize(item.memory)
	copied := win32GlobalAlloc(gmemMoveable, size)
	source := win32GlobalLock(item.memory)
	target := win32GlobalLock(copied)
	copy(unsafe.Slice((*byte)(memoryAt(target)), size), unsafe.Slice((*byte)(memoryAt(source)), size))
	win32GlobalUnlock(copied)
	win32GlobalUnlock(item.memory)
	out := (*stgMedium)(memoryAt(medium))
	out.tymed = tymedHGlobal
	out.handle = uintptr(copied)
	out.pUnkForRelease = 0
	return sOK
}

func serveDataQueryGet(this, format uintptr) uintptr {
	data, ok := serverObject[*dataObject](this)
	if !ok || data.find((*formatEtc)(memoryAt(format))) == nil {
		return dvEFormatEtc
	}
	return sOK
}

func serveDataEnumFormatEtc(this, direction, result uintptr) uintptr {
	data, ok := serverObject[*dataObject](this)
	if !ok {
		return eUnexpected
	}
	if direction != datadirGet {
		return eNotImpl
	}
	formats := make([]formatEtc, len(data.items))
	for i, item := range data.items {
		formats[i] = item.format
	}
	var first *formatEtc
	if len(formats) > 0 {
		first = &formats[0]
	}
	return hresultValue(win32SHCreateStdEnumFmtEtc(uint32(len(formats)), first, (*comObject)(memoryAt(result))))
}

// customFormat returns the clipboard format for a MIME type.
func customFormat(mimeType string) uint16 {
	return uint16(win32RegisterClipboardFormat("GameKit " + mimeType))
}

// fileList returns a DROPFILES block for paths.
func fileList(paths []string) []byte {
	var list []uint16
	for _, path := range paths {
		list = append(list, utf16.Encode([]rune(path))...)
		list = append(list, 0)
	}
	list = append(list, 0)
	header := dropFiles{pFiles: uint32(unsafe.Sizeof(dropFiles{})), fWide: 1}
	block := make([]byte, int(unsafe.Sizeof(header))+len(list)*2)
	copy(block, unsafe.Slice((*byte)(unsafe.Pointer(&header)), unsafe.Sizeof(header)))
	copy(block[unsafe.Sizeof(header):], unsafe.Slice((*byte)(unsafe.Pointer(&list[0])), len(list)*2))
	return block
}

// filesFromDrop returns the paths of an HDROP.
func filesFromDrop(drop uintptr) []string {
	count := win32DragQueryFile(drop, 0xffffffff, nil)
	paths := make([]string, 0, count)
	for i := range count {
		length := win32DragQueryFile(drop, i, nil)
		path := make([]uint16, length+1)
		win32DragQueryFile(drop, i, path)
		paths = append(paths, goString(path))
	}
	return paths
}

// dib returns a CF_DIB block for RGBA pixels.
func dib(pixels []byte, width, height int) []byte {
	header := bitmapInfoHeader{
		biWidth:       int32(width),
		biHeight:      int32(height),
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}
	header.biSize = uint32(unsafe.Sizeof(header))
	size := int(header.biSize)
	block := make([]byte, size+width*height*4)
	copy(block, unsafe.Slice((*byte)(unsafe.Pointer(&header)), size))
	out := block[size:]
	for y := range height {
		row := pixels[(height-1-y)*width*4:]
		for x := range width {
			o := out[(y*width+x)*4:]
			o[0] = row[x*4+2]
			o[1] = row[x*4+1]
			o[2] = row[x*4+0]
			o[3] = row[x*4+3]
		}
	}
	return block
}

// pixelsFromDIB reads RGBA pixels from a CF_DIB block.
func pixelsFromDIB(block []byte) (pixels []byte, width, height int, ok bool) {
	var header bitmapInfoHeader
	if len(block) < int(unsafe.Sizeof(header)) {
		return nil, 0, 0, false
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&header)), unsafe.Sizeof(header)), block)
	w, h := int(header.biWidth), int(header.biHeight)
	bottomUp := h > 0
	h = abs(h)
	bits := int(header.biBitCount)
	if w <= 0 || h <= 0 || (bits != 32 && bits != 24) {
		return nil, 0, 0, false
	}
	offset := int(header.biSize) + int(header.biClrUsed)*4
	if header.biCompression == biBitfields && header.biSize == uint32(unsafe.Sizeof(header)) {
		offset += 12
	}
	stride := (w*bits/8 + 3) &^ 3
	if len(block) < offset+stride*h {
		return nil, 0, 0, false
	}
	hasAlpha := bits == 32 && dibHasAlpha(block, header, w, h, offset, stride)
	pixels = make([]byte, w*h*4)
	for y := range h {
		source := y
		if bottomUp {
			source = h - 1 - y
		}
		row := block[offset+stride*source:]
		for x := range w {
			out := pixels[(y*w+x)*4:]
			p := row[x*(bits/8):]
			out[0], out[1], out[2], out[3] = p[2], p[1], p[0], 255
			if hasAlpha {
				out[3] = p[3]
			}
		}
	}
	return pixels, w, h, true
}

// dibHasAlpha reports whether a 32-bit DIB has an alpha channel: a V5 header
// says so with its alpha mask; another header has one when an alpha byte is
// not zero.
func dibHasAlpha(block []byte, header bitmapInfoHeader, width, height, offset, stride int) bool {
	if header.biSize >= uint32(unsafe.Sizeof(bitmapV5Header{})) {
		return (*bitmapV5Header)(unsafe.Pointer(&block[0])).bV5AlphaMask != 0
	}
	for y := range height {
		row := block[offset+stride*y:]
		for x := range width {
			if row[x*4+3] != 0 {
				return true
			}
		}
	}
	return false
}

// ---- Drag sources ----

var (
	dragSources = map[*nativeView]ui.DragSource{}
	dragView    *nativeView
	dragStart   point
)

var dropSourceTable []uintptr

// newDropSource returns an IDropSource for DoDragDrop.
func newDropSource() *comServer {
	if dropSourceTable == nil {
		dropSourceTable = methodTable(
			callback("IDropSource.QueryContinueDrag", func(this, escape, keys uintptr) uintptr {
				if escape != 0 {
					return dragdropSCancel
				}
				if keys&(mkLButton|mkRButton) == 0 {
					return dragdropSDrop
				}
				return sOK
			}),
			callback("IDropSource.GiveFeedback", func(this, effect uintptr) uintptr {
				return dragdropSUseDefaultCursors
			}),
		)
	}
	return newCOMServer(dropSourceTable, []*guid{iidDropSource}, nil)
}

// effects returns the drop effects of drag operations.
func effects(operations ui.DragOperation) uint32 {
	var result uint32
	if operations&ui.DragCopy != 0 {
		result |= dropEffectCopy
	}
	if operations&ui.DragMove != 0 {
		result |= dropEffectMove
	}
	if operations&ui.DragLink != 0 {
		result |= dropEffectLink
	}
	return result
}

// operationOf returns the drag operation of a drop effect.
func operationOf(effect uint32) ui.DragOperation {
	switch {
	case effect&dropEffectMove != 0:
		return ui.DragMove
	case effect&dropEffectCopy != 0:
		return ui.DragCopy
	case effect&dropEffectLink != 0:
		return ui.DragLink
	default:
		return 0
	}
}

// startDrag drags the data of a source until the user drops it. A source
// without operations copies.
func startDrag(source ui.DragSource) {
	server, data := newDataObject()
	if source.Text != "" {
		text := utf16.Encode([]rune(source.Text + "\x00"))
		data.add(cfUnicodeText, unsafe.Slice((*byte)(unsafe.Pointer(&text[0])), len(text)*2))
	}
	if len(source.Files) > 0 {
		data.add(cfHDrop, fileList(source.Files))
	}
	if source.Image != nil && !source.Image.Bounds().Empty() {
		data.add(cfDIB, dib(rgbaPixels(source.Image)))
	}
	for mime, value := range source.Custom {
		data.add(customFormat(mime), value)
	}
	drop := newDropSource()
	operations := source.Operations
	if operations == 0 {
		operations = ui.DragCopy
	}
	var effect uint32
	win32DoDragDrop(server.com(), drop.com(), effects(operations), &effect)
	win32UnknownRelease(drop.com())
	win32UnknownRelease(server.com())
}

// dragTrack starts a drag when the pointer moves far enough with the button
// down over a view that has a drag source. It returns true when it consumed
// the message.
func dragTrack(message *msg) bool {
	switch message.message {
	case wmLButtonDown:
		dragView = nil
		if view := viewFromDescendant(message.hwnd); view != nil && hasDragSource(view) {
			dragView = view
			win32GetCursorPos(&dragStart)
		}
		return false
	case wmLButtonUp:
		dragView = nil
		return false
	}
	if message.message != wmMouseMove || dragView == nil || message.wParam&mkLButton == 0 {
		return false
	}
	var cursor point
	win32GetCursorPos(&cursor)
	if abs(int(cursor.x-dragStart.x)) < int(win32GetSystemMetrics(smCXDrag)) &&
		abs(int(cursor.y-dragStart.y)) < int(win32GetSystemMetrics(smCYDrag)) {
		return false
	}
	view := dragView
	dragView = nil
	source, ok := dragSources[view]
	if !view.live() || !ok {
		return false
	}
	if win32GetCapture() != 0 {
		win32ReleaseCapture()
	}
	startDrag(source)
	return true
}

// setDragSource makes a view the source of a drag. A source without data
// stops it.
func (view *nativeView) setDragSource(source ui.DragSource) {
	if !view.live() {
		return
	}
	if source.Operations == 0 && source.Text == "" && len(source.Files) == 0 && source.Image == nil &&
		len(source.Custom) == 0 {
		delete(dragSources, view)
		return
	}
	dragSources[view] = source
}

func hasDragSource(view *nativeView) bool {
	_, ok := dragSources[view]
	return ok
}

func forgetDragSource(view *nativeView) {
	delete(dragSources, view)
	if dragView == view {
		dragView = nil
	}
}

// ---- Drop targets ----

// dropTarget implements IDropTarget for a view.
type dropTarget struct {
	view       *nativeView
	operations ui.DragOperation
	// formats are the MIME types that the target accepts; none accepts all.
	formats  []string
	accepted bool
}

var dropTargets = map[*nativeView]*comServer{}

func (t *dropTarget) wants(mimeType string) bool {
	return len(t.formats) == 0 || slices.Contains(t.formats, mimeType)
}

func offersFormat(data comObject, format uint16) bool {
	request := formatEtc{cfFormat: format, dwAspect: dvaspectContent, lindex: -1, tymed: tymedHGlobal}
	return win32DataObjectQueryGetData(data, &request) == sOK
}

func (t *dropTarget) acceptable(data comObject) bool {
	if (t.wants("text/plain") || t.wants("public.utf8-plain-text")) && offersFormat(data, cfUnicodeText) {
		return true
	}
	if (t.wants("text/uri-list") || t.wants("public.file-url") || t.wants("files")) && offersFormat(data, cfHDrop) {
		return true
	}
	if (t.wants("image/png") || t.wants("image")) && offersFormat(data, cfDIB) {
		return true
	}
	for _, mimeType := range t.formats {
		if offersFormat(data, customFormat(mimeType)) {
			return true
		}
	}
	return false
}

func (t *dropTarget) choose(keys, allowed uint32) uint32 {
	wanted := effects(t.operations) & allowed
	switch {
	case keys&mkControl != 0 && wanted&dropEffectCopy != 0:
		return dropEffectCopy
	case keys&mkShift != 0 && wanted&dropEffectMove != 0:
		return dropEffectMove
	case wanted&dropEffectCopy != 0:
		return dropEffectCopy
	case wanted&dropEffectMove != 0:
		return dropEffectMove
	default:
		return wanted & dropEffectLink
	}
}

// readData returns the data of an IDataObject in a format.
func readData(data comObject, format uint16) ([]byte, bool) {
	request := formatEtc{cfFormat: format, dwAspect: dvaspectContent, lindex: -1, tymed: tymedHGlobal}
	var medium stgMedium
	if win32DataObjectGetData(data, &request, &medium) != sOK {
		return nil, false
	}
	var block []byte
	if medium.tymed == tymedHGlobal && medium.handle != 0 {
		memory := hglobal(medium.handle)
		size := win32GlobalSize(memory)
		block = goBytesAt(win32GlobalLock(memory), int(size))
		win32GlobalUnlock(memory)
	}
	win32ReleaseStgMedium(&medium)
	return block, true
}

func (t *dropTarget) drop(data comObject, keys uint32, at point, effect *uint32) {
	chosen := uint32(dropEffectNone)
	if t.accepted {
		chosen = t.choose(keys, *effect)
	}
	*effect = chosen
	t.accepted = false
	view := t.view
	if chosen == 0 || !view.live() || view.receiver == nil {
		return
	}
	client := at
	win32ScreenToClient(view.outer, &client)
	scale := view.scale()
	var text string
	if block, ok := readData(data, cfUnicodeText); ok && len(block) >= 2 {
		units := unsafe.Slice((*uint16)(unsafe.Pointer(&block[0])), len(block)/2)
		text = goString(units)
	}
	var paths []string
	if block, ok := readData(data, cfHDrop); ok && len(block) >= int(unsafe.Sizeof(dropFiles{})) {
		memory := win32GlobalAlloc(gmemMoveable, uintptr(len(block)))
		copy(unsafe.Slice((*byte)(memoryAt(win32GlobalLock(memory))), len(block)), block)
		win32GlobalUnlock(memory)
		paths = filesFromDrop(uintptr(memory))
		win32GlobalFree(memory)
	}
	event := ui.DropEvent{
		Operation: operationOf(chosen),
		Position:  ui.Point{X: int(float64(client.x) / scale), Y: int(float64(client.y) / scale)},
		Text:      text,
		Files:     paths,
	}
	if block, ok := readData(data, cfDIB); ok {
		if pixels, width, height, ok := pixelsFromDIB(block); ok {
			image := stdimage.NewNRGBA(stdimage.Rect(0, 0, width, height))
			copy(image.Pix, pixels)
			event.Image = image
		}
	}
	for _, mimeType := range t.formats {
		if block, ok := readData(data, customFormat(mimeType)); ok {
			if event.Custom == nil {
				event.Custom = map[string][]byte{}
			}
			event.Custom[mimeType] = block
		}
	}
	if widget, ok := view.receiver.(*widgetBase); ok {
		widget.dispatchDrop(event)
	}
}

var dropTargetTable []uintptr

// pointlOf returns the POINTL that a method received by value, which takes one
// argument slot on 64-bit Windows.
func pointlOf(value uintptr) point {
	return point{x: int32(uint32(value)), y: int32(uint32(uint64(value) >> 32))}
}

func dropTargetMethods() []uintptr {
	enter := func(this, data, keys uintptr, at point, effect uintptr) uintptr {
		target, ok := serverObject[*dropTarget](this)
		if !ok {
			return eUnexpected
		}
		out := (*uint32)(memoryAt(effect))
		target.accepted = target.acceptable(comObject(data))
		if target.accepted {
			*out = target.choose(uint32(keys), *out)
		} else {
			*out = dropEffectNone
		}
		return sOK
	}
	over := func(this, keys uintptr, at point, effect uintptr) uintptr {
		target, ok := serverObject[*dropTarget](this)
		if !ok {
			return eUnexpected
		}
		out := (*uint32)(memoryAt(effect))
		if target.accepted {
			*out = target.choose(uint32(keys), *out)
		} else {
			*out = dropEffectNone
		}
		return sOK
	}
	leave := callback("IDropTarget.DragLeave", func(this uintptr) uintptr {
		if target, ok := serverObject[*dropTarget](this); ok {
			target.accepted = false
		}
		return sOK
	})
	drop := func(this, data, keys uintptr, at point, effect uintptr) uintptr {
		target, ok := serverObject[*dropTarget](this)
		if !ok {
			return eUnexpected
		}
		target.drop(comObject(data), uint32(keys), at, (*uint32)(memoryAt(effect)))
		return sOK
	}
	// A POINTL takes one argument slot on 64-bit Windows and two on 32-bit
	// Windows.
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return []uintptr{
			callback("IDropTarget.DragEnter", func(this, data, keys, at, effect uintptr) uintptr {
				return enter(this, data, keys, pointlOf(at), effect)
			}),
			callback("IDropTarget.DragOver", func(this, keys, at, effect uintptr) uintptr {
				return over(this, keys, pointlOf(at), effect)
			}),
			leave,
			callback("IDropTarget.Drop", func(this, data, keys, at, effect uintptr) uintptr {
				return drop(this, data, keys, pointlOf(at), effect)
			}),
		}
	}
	return []uintptr{
		callback("IDropTarget.DragEnter", func(this, data, keys, x, y, effect uintptr) uintptr {
			return enter(this, data, keys, point{x: int32(x), y: int32(y)}, effect)
		}),
		callback("IDropTarget.DragOver", func(this, keys, x, y, effect uintptr) uintptr {
			return over(this, keys, point{x: int32(x), y: int32(y)}, effect)
		}),
		leave,
		callback("IDropTarget.Drop", func(this, data, keys, x, y, effect uintptr) uintptr {
			return drop(this, data, keys, point{x: int32(x), y: int32(y)}, effect)
		}),
	}
}

func (view *nativeView) setDropTarget(target ui.DropTarget) {
	if !view.live() || view.outer == 0 {
		return
	}
	forgetDropTarget(view)
	if target.Operations == 0 {
		return
	}
	if dropTargetTable == nil {
		dropTargetTable = methodTable(dropTargetMethods()...)
	}
	drop := &dropTarget{view: view, operations: target.Operations, formats: target.Formats}
	server := newCOMServer(dropTargetTable, []*guid{iidDropTarget}, drop)
	if win32RegisterDragDrop(view.outer, server.com()).succeeded() {
		dropTargets[view] = server
	} else {
		win32UnknownRelease(server.com())
	}
}

func forgetDropTarget(view *nativeView) {
	server := dropTargets[view]
	if server == nil {
		return
	}
	if view.outer != 0 && win32IsWindow(view.outer) {
		win32RevokeDragDrop(view.outer)
	}
	if target, ok := server.object.(*dropTarget); ok {
		target.view = nil
	}
	delete(dropTargets, view)
	win32UnknownRelease(server.com())
}
