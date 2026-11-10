//go:build linux && cgo

package gtk

import "unsafe"

func registerNativeComponent(window uintptr, outer, control *gtkWidget, kind int) *gtkWidget {
	createNativeView(window, outer, control, kind, 0)
	return outer
}
func newComponentLayout() *gtkWidget                               { return createNativeLayout(gtkAccessibleRoleGroup) }
func newComponentLayoutWithRole(role gtkAccessibleRole) *gtkWidget { return createNativeLayout(role) }
func componentBody(widget, body *gtkWidget) {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		view.body = body
	}
}
func componentControl(widget *gtkWidget) *gtkWidget {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		return view.control
	}
	return widget
}
func sendComponentEvent(widget *gtkWidget, event gtkEvent) {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		view.send(event)
	}
}
func onComponentAllocate(widget *gtkWidget, allocate func(int, int)) {
	if layout := layoutState(widget); layout != nil {
		layout.allocate = allocate
	}
}
func moveComponent(widget *gtkWidget, x, y, width, height int) {
	moveNativeWidget(widget, x, y, width, height)
}
func minimumComponent(widget *gtkWidget, width, height int) {
	setNativeLayoutMinimum(widget, width, height)
}
func putComponent(parent, child *gtkWidget) { putNativeLayout(parent, child) }
func setComponentCSS(selector, rule string) { setNativeCSS(selector, rule) }
func announceComponent(widget *gtkWidget, text string, priority int) {
	gkGTKAnnounceText(widget, stringPointer(text), cInt(priority))
}
func componentFrameSize(widget *gtkWidget) (int, int) {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		return view.width, view.height
	}
	return int(gtkWidgetGetWidth(widget)), int(gtkWidgetGetHeight(widget))
}
func componentContainer(widget *gtkWidget) *gtkWidget {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		return view.container()
	}
	return nil
}
func registerComponentPane(owner, pane *gtkWidget) {
	if viewState(unsafe.Pointer(pane)) != nil {
		return
	}
	var window uintptr
	if parent := viewState(unsafe.Pointer(owner)); parent != nil {
		window = parent.window
	}
	view := createNativeView(0, pane, nil, kindClip, 0)
	view.window = window
	view.body = pane
	gObjectUnref(unsafe.Pointer(pane))
}
func setComponentWindow(widget, anchor *gtkWidget) {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		if win := nativeWinFromWidget(anchor); win != nil {
			view.window = uintptr(unsafe.Pointer(win.widget))
		}
	}
}
func removeNativeComponent(widget *gtkWidget) { removeNativeWidget(widget) }
func setComponentLabel(widget, label *gtkWidget) {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		view.label = label
	}
}
func configureComponentButton(widget *gtkWidget, icon *gdkPaintable) {
	gkGTKButtonConfigure(unsafe.Pointer(widget), unsafe.Pointer(icon), 0, 0, nil, 0)
}
func componentKeyCode(code uint32) int { return physicalKeyCode(code) }
func componentCSSClass(widget *gtkWidget) string {
	return nativeWidgetCSSClass(viewState(unsafe.Pointer(widget)))
}
func queryComponent(widget *gtkWidget, event gtkEvent) bool {
	view := viewState(unsafe.Pointer(widget))
	return view != nil && queryNativeEvent(view.handle, event)
}
