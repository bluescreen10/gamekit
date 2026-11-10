//go:build linux && cgo

package gtk

import "unsafe"

var componentStateKey = gQuarkFromString(stringPointer("gamekit-component"))

func storeComponentState(widget *gtkWidget, state any) {
	gObjectSetQdataFull((*gObject)(unsafe.Pointer(widget)), componentStateKey, newNativeCallback(state), gtkDestroyCallback())
}
func componentState[T any](pointer unsafe.Pointer) *T {
	if pointer == nil {
		return nil
	}
	data := gObjectGetQdata((*gObject)(pointer), componentStateKey)
	if data == nil {
		return nil
	}
	state, _ := nativeCallback(data).(*T)
	return state
}
func widgetWithRole(kind gType, role gtkAccessibleRole) *gtkWidget {
	var value gValue
	gValueInit(&value, gtkAccessibleRoleGetType())
	gValueSetEnum(&value, cInt(role))
	names, free := nativeStringVector([]string{"accessible-role"})
	object := gObjectNewWithProperties(kind, 1, names, &value)
	free()
	gValueUnset(&value)
	return (*gtkWidget)(unsafe.Pointer(object))
}
func boxWithRole(orientation gtkOrientation, spacing int, role gtkAccessibleRole) *gtkWidget {
	box := widgetWithRole(gtkBoxGetType(), role)
	gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(box)), orientation)
	gtkBoxSetSpacing((*gtkBox)(unsafe.Pointer(box)), cInt(spacing))
	return box
}
func setAccessibleText(widget *gtkWidget, property gtkAccessibleProperty, text string) {
	var value gValue
	gValueInit(&value, gTypeString)
	gValueSetString(&value, stringPointer(text))
	gtkAccessibleUpdatePropertyValue((*gtkAccessible)(unsafe.Pointer(widget)), 1, &property, &value)
	gValueUnset(&value)
}
func measureWidget(widget *gtkWidget, orientation gtkOrientation, forSize int) int {
	var natural cInt
	gtkWidgetMeasure(widget, orientation, cInt(forSize), nil, &natural, nil, nil)
	return int(natural)
}
func preferredWidgetSize(widget *gtkWidget) (int, int) {
	return measureWidget(widget, gtkOrientationHorizontal, -1), measureWidget(widget, gtkOrientationVertical, -1)
}
func positionForEdge(edge int) gtkPositionType {
	switch edge {
	case 0:
		return gtkPosTop
	case 2:
		return gtkPosLeft
	case 3:
		return gtkPosRight
	default:
		return gtkPosBottom
	}
}
func anchorPopover(popover, anchor *gtkWidget, edge int) bool {
	root := gtkWidgetGetRoot(anchor)
	if root == nil || gtkWidgetGetVisible((*gtkWidget)(unsafe.Pointer(root))) == 0 || gtkWidgetGetMapped(anchor) == 0 {
		return false
	}
	if gtkWidgetGetParent(popover) != anchor {
		if gtkWidgetGetParent(popover) != nil {
			gtkWidgetUnparent(popover)
		}
		gtkWidgetSetParent(popover, anchor)
	}
	gtkPopoverSetPosition((*gtkPopover)(unsafe.Pointer(popover)), positionForEdge(edge))
	return true
}
func closeNativePopover(widget *gtkWidget) {
	gtkPopoverPopdown((*gtkPopover)(unsafe.Pointer(widget)))
	if gtkWidgetGetParent(widget) != nil {
		gtkWidgetUnparent(widget)
	}
	gObjectUnref(unsafe.Pointer(widget))
}

func setAccessibleBoolState(widget *gtkWidget, state gtkAccessibleState, value bool) {
	var native gValue
	gtkAccessibleStateInitValue(state, &native)
	switch gTypeFundamental(native.g_type) {
	case gTypeBoolean:
		gValueSetBoolean(&native, nativeBool(value))
	case gTypeEnum:
		gValueSetEnum(&native, cInt(nativeBool(value)))
	default:
		gValueSetInt(&native, cInt(nativeBool(value)))
	}
	gtkAccessibleUpdateStateValue((*gtkAccessible)(unsafe.Pointer(widget)), 1, &state, &native)
	gValueUnset(&native)
}

func setNativeProperty(object unsafe.Pointer, name string, value any) {
	var native gValue
	switch value := value.(type) {
	case int:
		gValueInit(&native, gTypeInt)
		gValueSetInt(&native, cInt(value))
	case float64:
		gValueInit(&native, gTypeDouble)
		gValueSetDouble(&native, cDouble(value))
	case gdkRGBA:
		gValueInit(&native, gdkRgbaGetType())
		gValueSetBoxed(&native, unsafe.Pointer(&value))
	case string:
		gValueInit(&native, gTypeString)
		gValueSetString(&native, stringPointer(value))
	case bool:
		gValueInit(&native, gTypeBoolean)
		gValueSetBoolean(&native, nativeBool(value))
	}
	gObjectSetProperty((*gObject)(object), stringPointer(name), &native)
	gValueUnset(&native)
}
func getNativeStringProperty(object unsafe.Pointer, name string) string {
	var value gValue
	gValueInit(&value, gTypeString)
	gObjectGetProperty((*gObject)(object), stringPointer(name), &value)
	defer gValueUnset(&value)
	return goString(gValueGetString(&value))
}
func nativeDescendant(widget *gtkWidget, kind gType) *gtkWidget {
	for child := gtkWidgetGetFirstChild(widget); child != nil; child = gtkWidgetGetNextSibling(child) {
		if gTypeCheckInstanceIsA((*gTypeInstance)(unsafe.Pointer(child)), kind) != 0 {
			return child
		}
		if found := nativeDescendant(child, kind); found != nil {
			return found
		}
	}
	return nil
}
func setAccessibleNumber(widget *gtkWidget, property gtkAccessibleProperty, number float64) {
	var value gValue
	gValueInit(&value, gTypeDouble)
	gValueSetDouble(&value, cDouble(number))
	gtkAccessibleUpdatePropertyValue((*gtkAccessible)(unsafe.Pointer(widget)), 1, &property, &value)
	gValueUnset(&value)
}

func semanticCSSColor(color int) string {
	colors := [...]string{
		"",
		"@theme_fg_color",
		"alpha(@theme_fg_color, 0.7)",
		"alpha(@theme_fg_color, 0.5)",
		"@theme_base_color",
		"@theme_selected_bg_color",
		"@success_color",
		"@warning_color",
		"@error_color",
		"@accent_color",
	}
	if color < 0 || color >= len(colors) {
		return ""
	}
	return colors[color]
}
