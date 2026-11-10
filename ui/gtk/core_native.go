//go:build linux && cgo

package gtk

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unsafe"
)

// nativeView holds the state of a GameKit component. GTK owns the widgets;
// the qdata handle keeps this Go record alive for exactly their lifetime.
type nativeView struct {
	outer, control, body, label *gtkWidget
	window                      uintptr
	kind                        int
	handle, action              uintptr
	suppress                    int

	x, y, width, height                           int
	visible                                       bool
	cssClass                                      string
	accessibleName, accessibleID, accessibleValue string
	dragSource, dropTarget                        *gtkEventController

	prefix, suffix, scroll, end                                      *gtkWidget
	checkTitle, checkDetail                                          *gtkWidget
	placeholder, url, text                                           string
	hasText                                                          bool
	buttons                                                          []*gtkWidget
	enabled                                                          []bool
	selected, popupWidth, visibleRows, preferredWidth, maximumLength int
	textLimitConnected                                               bool

	running, vertical, inverted bool
	value                       float64
	progressTick                cUint

	texture            *gdkTexture
	scaling, alignment int
	nearest            bool
	sourceScale        float64

	button  *nativeButton
	section *nativeSection
	canvas  *nativeCanvas
}

var viewKey = gQuarkFromString(stringPointer("gamekit-view"))

func viewState(pointer unsafe.Pointer) *nativeView {
	if pointer == nil {
		return nil
	}
	data := gObjectGetQdata((*gObject)(pointer), viewKey)
	if data == nil {
		return nil
	}
	return nativeCallback(data).(*nativeView)
}
func createNativeView(window uintptr, outer, control *gtkWidget, kind int, action uintptr) *nativeView {
	if control == nil {
		control = outer
	}
	view := &nativeView{outer: outer, control: control, kind: kind, window: window, action: action, selected: -1, visible: true, sourceScale: 1}
	gObjectRefSink(unsafe.Pointer(outer))
	gObjectSetQdataFull((*gObject)(unsafe.Pointer(outer)), viewKey, newNativeCallback(view), gtkDestroyCallback())
	if win := nativeWinForHandle(window); win != nil {
		putNativeLayout(win.root, outer)
	}
	return view
}
func (view *nativeView) releaseNative() {
	if view.texture != nil {
		gObjectUnref(unsafe.Pointer(view.texture))
	}
	if view.cssClass != "" {
		removeNativeCSSClass(view.cssClass)
	}
}
func (view *nativeView) send(event gtkEvent) {
	if view.suppress == 0 {
		dispatchNativeEvent(view.handle, event)
	}
}
func (view *nativeView) activate(event int) {
	if view.suppress == 0 {
		gkGTKUIGoEvent(cUintptr(view.action), cInt(event))
	}
}
func (view *nativeView) container() *gtkWidget {
	if view.body != nil {
		return view.body
	}
	return view.outer
}
func viewFromDescendant(widget *gtkWidget) *nativeView {
	for widget != nil {
		if view := viewState(unsafe.Pointer(widget)); view != nil {
			return view
		}
		widget = gtkWidgetGetParent(widget)
	}
	return nil
}
func isNative(object unsafe.Pointer, kind gType) bool {
	return object != nil && gTypeCheckInstanceIsA((*gTypeInstance)(object), kind) != 0
}

// GTK's native GtkBox owns child disposal. The custom class only supplies
// GameKit's manual frames, measurement, and clipping callbacks.
type nativeFrame struct{ x, y, width, height int }
type nativeLayout struct {
	minimumWidth, minimumHeight int
	zoom                        float64
	allocating                  bool
	allocate                    func(int, int)
}

var layoutKey = gQuarkFromString(stringPointer("gamekit-layout"))
var frameKey = gQuarkFromString(stringPointer("gamekit-frame"))
var nativeLayoutType gType

func layoutState(widget *gtkWidget) *nativeLayout {
	if widget == nil {
		return nil
	}
	data := gObjectGetQdata((*gObject)(unsafe.Pointer(widget)), layoutKey)
	if data == nil {
		return nil
	}
	return nativeCallback(data).(*nativeLayout)
}
func frameState(widget *gtkWidget) *nativeFrame {
	data := gObjectGetQdata((*gObject)(unsafe.Pointer(widget)), frameKey)
	if data == nil {
		return nil
	}
	return nativeCallback(data).(*nativeFrame)
}
func createNativeLayout(role gtkAccessibleRole) *gtkWidget {
	if nativeLayoutType == 0 {
		nativeLayoutType = registerNativeLayoutType()
	}
	widget := widgetWithRole(nativeLayoutType, role)
	gtkWidgetSetLayoutManager(widget, nil)
	gtkWidgetSetOverflow(widget, gtkOverflowVisible)
	gObjectSetQdataFull((*gObject)(unsafe.Pointer(widget)), layoutKey, newNativeCallback(&nativeLayout{zoom: 1}), gtkDestroyCallback())
	return widget
}
func measureNativeLayout(widget *gtkWidget, orientation gtkOrientation) (int, int) {
	layout := layoutState(widget)
	if layout == nil {
		return 0, 0
	}
	size := layout.minimumWidth
	if orientation == gtkOrientationVertical {
		size = layout.minimumHeight
	}
	value := int(math.Ceil(float64(size) * layout.zoom))
	return value, value
}
func allocateNativeLayout(widget *gtkWidget, width, height int) {
	layout := layoutState(widget)
	if layout == nil {
		return
	}
	layout.allocating = true
	defer func() { layout.allocating = false }()
	if layout.allocate != nil {
		layout.allocate(width, height)
	}
	for child := gtkWidgetGetFirstChild(widget); child != nil; child = gtkWidgetGetNextSibling(child) {
		if gtkWidgetShouldLayout(child) == 0 {
			continue
		}
		frame := frameState(child)
		if frame == nil {
			frame = &nativeFrame{}
		}
		var minimum cInt
		gtkWidgetMeasure(child, gtkOrientationHorizontal, -1, &minimum, nil, nil, nil)
		w := max(frame.width, int(minimum))
		gtkWidgetMeasure(child, gtkOrientationVertical, cInt(w), &minimum, nil, nil, nil)
		h := max(frame.height, int(minimum))
		origin := graphenePoint{x: cFloat(float64(frame.x) * layout.zoom), y: cFloat(float64(frame.y) * layout.zoom)}
		transform := gskTransformTranslate(nil, &origin)
		if layout.zoom != 1 {
			transform = gskTransformScale(transform, cFloat(layout.zoom), cFloat(layout.zoom))
		}
		gtkWidgetAllocate(child, cInt(w), cInt(h), -1, transform)
	}
}
func snapshotNativeLayout(widget *gtkWidget, snapshot *gtkSnapshot) {
	clip := nativeRect(-8, -8, float64(gtkWidgetGetWidth(widget))+16, float64(gtkWidgetGetHeight(widget))+16)
	gtkSnapshotPushClip(snapshot, &clip)
	for child := gtkWidgetGetFirstChild(widget); child != nil; child = gtkWidgetGetNextSibling(child) {
		gtkWidgetSnapshotChild(widget, child, snapshot)
	}
	gtkSnapshotPop(snapshot)
}
func nativeRect(x, y, width, height float64) grapheneRect {
	return grapheneRect{origin: graphenePoint{x: cFloat(x), y: cFloat(y)}, size: grapheneSize{width: cFloat(width), height: cFloat(height)}}
}
func putNativeLayout(parent, child *gtkWidget) {
	if parent == nil || child == nil {
		return
	}
	removeNativeWidget(child)
	frame := frameState(child)
	gtkWidgetSetChildVisible(child, nativeBool(frame != nil && frame.width > 0 && frame.height > 0))
	gtkWidgetSetParent(child, parent)
}
func moveNativeWidget(widget *gtkWidget, x, y, width, height int) {
	frame := nativeFrame{x, y, max(0, width), max(0, height)}
	previous := frameState(widget)
	if previous != nil && *previous == frame {
		return
	}
	if previous == nil {
		previous = &nativeFrame{}
		gObjectSetQdataFull((*gObject)(unsafe.Pointer(widget)), frameKey, newNativeCallback(previous), gtkDestroyCallback())
	}
	*previous = frame
	gtkWidgetSetChildVisible(widget, nativeBool(frame.width > 0 && frame.height > 0))
	if parent := gtkWidgetGetParent(widget); parent != nil {
		if layout := layoutState(parent); layout == nil || !layout.allocating {
			gtkWidgetQueueAllocate(parent)
		}
	}
}
func setNativeLayoutMinimum(widget *gtkWidget, width, height int) {
	if layout := layoutState(widget); layout != nil {
		layout.minimumWidth, layout.minimumHeight = max(0, width), max(0, height)
		gtkWidgetQueueResize(widget)
	}
}
func setNativeLayoutZoom(widget *gtkWidget, zoom float64) {
	if layout := layoutState(widget); layout != nil && zoom > 0 {
		layout.zoom = zoom
		gtkWidgetQueueResize(widget)
	}
}
func nativeLayoutZoom(widget *gtkWidget) float64 {
	if layout := layoutState(widget); layout != nil {
		return layout.zoom
	}
	return 1
}
func removeNativeWidget(widget *gtkWidget) {
	if widget == nil {
		return
	}
	parent := gtkWidgetGetParent(widget)
	if parent == nil {
		return
	}
	switch {
	case layoutState(parent) != nil:
		gtkWidgetUnparent(widget)
	case isNative(unsafe.Pointer(parent), gtkBoxGetType()):
		gtkBoxRemove((*gtkBox)(unsafe.Pointer(parent)), widget)
	case isNative(unsafe.Pointer(parent), gtkOverlayGetType()):
		overlay := (*gtkOverlay)(unsafe.Pointer(parent))
		if gtkOverlayGetChild(overlay) == widget {
			gtkOverlaySetChild(overlay, nil)
		} else {
			gtkOverlayRemoveOverlay(overlay, widget)
		}
	case isNative(unsafe.Pointer(parent), gtkScrolledWindowGetType()):
		gtkScrolledWindowSetChild((*gtkScrolledWindow)(unsafe.Pointer(parent)), nil)
	case isNative(unsafe.Pointer(parent), gtkViewportGetType()):
		gtkViewportSetChild((*gtkViewport)(unsafe.Pointer(parent)), nil)
	case isNative(unsafe.Pointer(parent), gtkFrameGetType()):
		gtkFrameSetChild((*gtkFrame)(unsafe.Pointer(parent)), nil)
	case isNative(unsafe.Pointer(parent), gtkPopoverGetType()):
		gtkPopoverSetChild((*gtkPopover)(unsafe.Pointer(parent)), nil)
	case isNative(unsafe.Pointer(parent), gtkRevealerGetType()):
		gtkRevealerSetChild((*gtkRevealer)(unsafe.Pointer(parent)), nil)
	case isNative(unsafe.Pointer(parent), gtkWindowGetType()):
		gtkWindowSetChild((*gtkWindow)(unsafe.Pointer(parent)), nil)
	case isNative(unsafe.Pointer(parent), gtkGridGetType()):
		gtkGridRemove((*gtkGrid)(unsafe.Pointer(parent)), widget)
	case isNative(unsafe.Pointer(parent), gtkStackGetType()):
		gtkStackRemove((*gtkStack)(unsafe.Pointer(parent)), widget)
	case isNative(unsafe.Pointer(parent), gtkPanedGetType()):
		pane := (*gtkPaned)(unsafe.Pointer(parent))
		if gtkPanedGetStartChild(pane) == widget {
			gtkPanedSetStartChild(pane, nil)
		} else {
			gtkPanedSetEndChild(pane, nil)
		}
	case isNative(unsafe.Pointer(parent), gtkFlowBoxChildGetType()):
		gtkFlowBoxChildSetChild((*gtkFlowBoxChild)(unsafe.Pointer(parent)), nil)
	default:
		gtkWidgetUnparent(widget)
	}
}

var nativeCSS struct {
	provider *gtkCssProvider
	rules    map[string]string
	next     uint64
}

func applyNativeCSS() {
	if nativeCSS.provider == nil {
		nativeCSS.provider = gtkCSSProviderNew()
		if display := gdkDisplayGetDefault(); display != nil {
			gtkStyleContextAddProviderForDisplay(display, (*gtkStyleProvider)(unsafe.Pointer(nativeCSS.provider)), 601)
		}
	}
	var source strings.Builder
	keys := make([]string, 0, len(nativeCSS.rules))
	for selector := range nativeCSS.rules {
		keys = append(keys, selector)
	}
	sort.Strings(keys)
	for _, selector := range keys {
		fmt.Fprintf(&source, "%s { %s }\n", selector, nativeCSS.rules[selector])
	}
	gtkCSSProviderLoadFromString(nativeCSS.provider, stringPointer(source.String()))
}
func setNativeCSS(selector, rule string) {
	if nativeCSS.rules == nil {
		nativeCSS.rules = make(map[string]string)
	}
	if rule == "" {
		delete(nativeCSS.rules, selector)
	} else {
		nativeCSS.rules[selector] = rule
	}
	applyNativeCSS()
}
func removeNativeCSSClass(class string) {
	for selector := range nativeCSS.rules {
		if selectorHasNativeClass(selector, class) {
			delete(nativeCSS.rules, selector)
		}
	}
	applyNativeCSS()
}
func nativeWidgetCSSClass(view *nativeView) string {
	if view == nil {
		return ""
	}
	if view.cssClass == "" {
		nativeCSS.next++
		view.cssClass = fmt.Sprintf("gk-w%d", nativeCSS.next)
		gtkWidgetAddCSSClass(view.outer, stringPointer(view.cssClass))
		if view.control != view.outer {
			gtkWidgetAddCSSClass(view.control, stringPointer(view.cssClass))
		}
	}
	return view.cssClass
}

func gkGTKRelease(object unsafe.Pointer) {
	if object != nil {
		gObjectUnref(object)
	}
}
func gkGTKRetain(object unsafe.Pointer) {
	if object != nil {
		gObjectRef(object)
	}
}

func gkGTKSetHandle(object unsafe.Pointer, handle cUintptr) {
	if view := viewState(object); view != nil {
		view.handle = uintptr(handle)
	}
}

func nativeModifiers(state gdkModifierType) int {
	result := 0
	for _, mask := range []struct {
		native gdkModifierType
		bit    int
	}{{gdkShiftMask, 1}, {gdkControlMask, 2}, {gdkAltMask, 4}, {gdkSuperMask | gdkMetaMask, 8}, {gdkLockMask, 16}} {
		if state&mask.native != 0 {
			result |= mask.bit
		}
	}
	return result
}
func nativePointerButton(button uint32) int {
	switch button {
	case 1:
		return 0
	case 2:
		return 2
	case 3:
		return 1
	}
	if button >= 8 && button < 13 {
		return int(button) - 5
	}
	return -1
}

// Linux evdev positions map to the portable physical key codes.
var nativePhysicalKeys = [...]int{
	1: 256, 2: 49, 3: 50, 4: 51, 5: 52, 6: 53, 7: 54, 8: 55,
	9: 56, 10: 57, 11: 48, 12: 45, 13: 61, 14: 259, 15: 258, 16: 81,
	17: 87, 18: 69, 19: 82, 20: 84, 21: 89, 22: 85, 23: 73, 24: 79,
	25: 80, 26: 91, 27: 93, 28: 257, 29: 341, 30: 65, 31: 83, 32: 68,
	33: 70, 34: 71, 35: 72, 36: 74, 37: 75, 38: 76, 39: 59, 40: 39,
	41: 96, 42: 340, 43: 92, 44: 90, 45: 88, 46: 67, 47: 86, 48: 66,
	49: 78, 50: 77, 51: 44, 52: 46, 53: 47, 54: 344, 55: 332, 56: 342,
	57: 32, 58: 280, 59: 290, 60: 291, 61: 292, 62: 293, 63: 294, 64: 295,
	65: 296, 66: 297, 67: 298, 68: 299, 69: 282, 70: 281, 71: 327, 72: 328,
	73: 329, 74: 333, 75: 324, 76: 325, 77: 326, 78: 334, 79: 321, 80: 322,
	81: 323, 82: 320, 83: 330, 86: 161, 87: 300, 88: 301, 96: 335, 97: 345,
	98: 331, 99: 283, 100: 346, 102: 268, 103: 265, 104: 266, 105: 263, 106: 262,
	107: 269, 108: 264, 109: 267, 110: 260, 111: 261, 117: 336, 119: 284, 125: 343,
	126: 347, 127: 348, 183: 302, 184: 303, 185: 304, 186: 305, 187: 306, 188: 307,
	189: 308, 190: 309, 191: 310, 192: 311, 193: 312, 194: 313,
}

func physicalKeyCode(code uint32) int {
	if code < 8 || int(code-8) >= len(nativePhysicalKeys) {
		return -1
	}
	key := nativePhysicalKeys[code-8]
	if key == 0 {
		return -1
	}
	return key
}

func registerNativeLayoutType() gType {
	return gTypeRegisterStaticSimple(gtkBoxGetType(), stringPointer("GameKitLayout"), gtkBoxClassSize, gtkLayoutClassCallback(), gtkBoxSize, nil, 0)
}
func registerNativeImageType() gType {
	return gTypeRegisterStaticSimple(gtkWidgetGetType(), stringPointer("GameKitImage"), gtkWidgetClassSize, gtkImageClassCallback(), gtkWidgetSize, nil, 0)
}
func initializeNativeLayoutClass(widget *gtkWidgetClass) {
	widget.measure = gtkLayoutMeasureAddress()
	widget.size_allocate = gtkLayoutAllocateAddress()
	widget.snapshot = gtkLayoutSnapshotAddress()
}
func initializeNativeImageClass(widget *gtkWidgetClass) {
	widget.measure = gtkImageMeasureAddress()
	widget.snapshot = gtkImageSnapshotAddress()
	gtkWidgetClassSetAccessibleRole(widget, gtkAccessibleRoleImg)
}

func selectorHasNativeClass(selector, class string) bool {
	token := "." + class
	for offset := 0; offset < len(selector); {
		index := strings.Index(selector[offset:], token)
		if index < 0 {
			return false
		}
		end := offset + index + len(token)
		if end == len(selector) {
			return true
		}
		next := selector[end]
		if !(next >= 'a' && next <= 'z' || next >= 'A' && next <= 'Z' || next >= '0' && next <= '9' || next == '_' || next == '-') {
			return true
		}
		offset = end
	}
	return false
}
