//go:build linux && cgo

package gtk

import "unsafe"

type nativeSection struct {
	header, revealer, arrow, title, subtitle, badge, icon *gtkWidget
	expanded, animated                                    bool
}

func (section *nativeSection) update() {
	icon := "pan-end-symbolic"
	if section.expanded {
		icon = "pan-down-symbolic"
	}
	gtkImageSetFromIconName((*gtkImage)(unsafe.Pointer(section.arrow)), stringPointer(icon))
	duration := cUint(0)
	if section.animated {
		duration = 200
	}
	gtkRevealerSetTransitionDuration((*gtkRevealer)(unsafe.Pointer(section.revealer)), duration)
	gtkRevealerSetRevealChild((*gtkRevealer)(unsafe.Pointer(section.revealer)), nativeBool(section.expanded))
	setAccessibleBoolState(section.header, gtkAccessibleStateExpanded, section.expanded)
}
func gkGTKUICreateSection(window cUintptr, title *cChar, expanded cInt, handle cUintptr) unsafe.Pointer {
	outer := createNativeLayout(gtkAccessibleRoleGroup)
	state := &nativeSection{
		header:   gtkButtonNew(),
		arrow:    gtkImageNew(),
		icon:     gtkImageNew(),
		title:    gtkLabelNew(title),
		subtitle: gtkLabelNew(stringPointer("")),
		badge:    gtkLabelNew(stringPointer("")),
		revealer: gtkRevealerNew(),
		expanded: expanded != 0,
	}
	gtkWidgetAddCSSClass(state.header, stringPointer("flat"))
	row := gtkBoxNew(gtkOrientationHorizontal, 6)
	for _, widget := range []*gtkWidget{state.arrow, state.icon, state.title, state.subtitle, state.badge} {
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), widget)
	}
	for _, widget := range []*gtkWidget{state.icon, state.subtitle, state.badge} {
		gtkWidgetSetVisible(widget, 0)
	}
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(state.title)), 0)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(state.subtitle)), 0)
	gtkWidgetSetHexpand(state.subtitle, 1)
	gtkWidgetAddCSSClass(state.subtitle, stringPointer("dim-label"))
	gtkWidgetAddCSSClass(state.badge, stringPointer("dim-label"))
	gtkButtonSetChild((*gtkButton)(unsafe.Pointer(state.header)), row)
	putNativeLayout(outer, state.header)
	moveNativeWidget(state.header, 0, 0, 160, 28)
	body := createNativeLayout(gtkAccessibleRoleGroup)
	gtkRevealerSetTransitionType((*gtkRevealer)(unsafe.Pointer(state.revealer)), gtkRevealerTransitionTypeSlideDown)
	gtkRevealerSetChild((*gtkRevealer)(unsafe.Pointer(state.revealer)), body)
	putNativeLayout(outer, state.revealer)
	view := createNativeView(uintptr(window), outer, state.header, kindSection, uintptr(handle))
	view.body, view.section = body, state
	layoutState(outer).allocate = func(width, height int) {
		moveNativeWidget(state.header, 0, 0, width, 28)
		moveNativeWidget(state.revealer, 0, 28, width, max(0, height-28))
	}
	connectNativeSignal(unsafe.Pointer(state.header), "clicked", func() { state.expanded = !state.expanded; state.update(); view.activate(uiEventSectionChange) })
	state.update()
	return unsafe.Pointer(outer)
}
func gkGTKUISectionExpanded(pointer unsafe.Pointer) cInt {
	if view := viewState(pointer); view != nil && view.section != nil {
		return cInt(nativeBool(view.section.expanded))
	}
	return 0
}
func gkGTKUISectionSetExpanded(pointer unsafe.Pointer, expanded cInt) {
	if view := viewState(pointer); view != nil && view.section != nil {
		view.section.expanded = expanded != 0
		view.section.update()
	}
}
func gkGTKSectionSetAnimated(pointer unsafe.Pointer, animated cInt) {
	if view := viewState(pointer); view != nil && view.section != nil {
		view.section.animated = animated != 0
		view.section.update()
	}
}
func gkGTKSectionConfigure(pointer unsafe.Pointer, subtitle *cChar, icon unsafe.Pointer, badge *cChar) {
	view := viewState(pointer)
	if view == nil || view.section == nil {
		return
	}
	state := view.section
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.subtitle)), subtitle)
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.badge)), badge)
	gtkImageSetFromPaintable((*gtkImage)(unsafe.Pointer(state.icon)), (*gdkPaintable)(icon))
	gtkWidgetSetVisible(state.subtitle, nativeBool(goString(subtitle) != ""))
	gtkWidgetSetVisible(state.badge, nativeBool(goString(badge) != ""))
	gtkWidgetSetVisible(state.icon, nativeBool(icon != nil))
}
