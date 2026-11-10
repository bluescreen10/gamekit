//go:build linux && cgo

package gtk

import (
	"fmt"
	"math"
	"strings"
	"unsafe"
)

func setOptionalAccessibleText(widget *gtkWidget, property gtkAccessibleProperty, text string) {
	if text == "" {
		gtkAccessibleResetProperty((*gtkAccessible)(unsafe.Pointer(widget)), property)
	} else {
		setAccessibleText(widget, property, text)
	}
}
func setAccessibleRelation(widget *gtkWidget, relation gtkAccessibleRelation, widgets []*gtkWidget) {
	element := (*gtkAccessible)(unsafe.Pointer(widget))
	if len(widgets) == 0 {
		gtkAccessibleResetRelation(element, relation)
		return
	}
	var list *gList
	for i := len(widgets) - 1; i >= 0; i-- {
		list = gListPrepend(list, unsafe.Pointer(widgets[i]))
	}
	var value gValue
	gtkAccessibleRelationInitValue(relation, &value)
	gValueSetPointer(&value, unsafe.Pointer(list))
	gtkAccessibleUpdateRelationValue(element, 1, &relation, &value)
	gValueUnset(&value)
}
func gkGTKSetAccessibility(pointer unsafe.Pointer, identifier, name, description, value *cChar, required, hidden, invalid cInt, labeledBy unsafe.Pointer, describedBy *unsafe.Pointer, describedByCount cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	view.accessibleID, view.accessibleName, view.accessibleValue = goString(identifier), goString(name), goString(value)
	setOptionalAccessibleText(view.control, gtkAccessiblePropertyLabel, view.accessibleName)
	setOptionalAccessibleText(view.control, gtkAccessiblePropertyDescription, goString(description))
	setOptionalAccessibleText(view.control, gtkAccessiblePropertyValueText, view.accessibleValue)
	var propertyValue gValue
	gValueInit(&propertyValue, gTypeBoolean)
	gValueSetBoolean(&propertyValue, nativeBool(required != 0))
	property := gtkAccessiblePropertyRequired
	gtkAccessibleUpdatePropertyValue((*gtkAccessible)(unsafe.Pointer(view.control)), 1, &property, &propertyValue)
	gValueUnset(&propertyValue)
	setAccessibleBoolState(view.control, gtkAccessibleStateHidden, hidden != 0)
	// The value API reads invalid-state values as integers.
	state := gtkAccessibleStateInvalid
	gValueInit(&propertyValue, gTypeInt)
	v := cInt(gtkAccessibleInvalidFalse)
	if invalid != 0 {
		v = gtkAccessibleInvalidTrue
	}
	gValueSetInt(&propertyValue, v)
	gtkAccessibleUpdateStateValue((*gtkAccessible)(unsafe.Pointer(view.control)), 1, &state, &propertyValue)
	gValueUnset(&propertyValue)
	var labels, descriptions []*gtkWidget
	if label := viewState(labeledBy); label != nil {
		labels = append(labels, label.control)
	}
	for _, pointer := range unsafe.Slice(describedBy, int(describedByCount)) {
		if related := viewState(pointer); related != nil {
			descriptions = append(descriptions, related.control)
		}
	}
	setAccessibleRelation(view.control, gtkAccessibleRelationLabelledBy, labels)
	setAccessibleRelation(view.control, gtkAccessibleRelationDescribedBy, descriptions)
}
func gkGTKAnnounceText(widget *gtkWidget, text *cChar, priority cInt) {
	if widget == nil || goString(text) == "" {
		return
	}
	level := gtkAccessibleAnnouncementPriorityMedium
	if priority >= 2 {
		level = gtkAccessibleAnnouncementPriorityHigh
	}
	gtkAccessibleAnnounce((*gtkAccessible)(unsafe.Pointer(widget)), text, level)
}
func gkGTKAnnounce(pointer unsafe.Pointer, text *cChar, priority cInt) {
	widget := (*gtkWidget)(pointer)
	if view := viewState(pointer); view != nil {
		widget = view.control
	}
	gkGTKAnnounceText(widget, text, priority)
}
func accessibleView(widget *gtkWidget) *nativeView {
	if view := viewState(unsafe.Pointer(widget)); view != nil {
		return view
	}
	if parent := gtkWidgetGetParent(widget); parent != nil {
		if view := viewState(unsafe.Pointer(parent)); view != nil && view.control == widget {
			return view
		}
	}
	return nil
}
func nativeSubrole(widget *gtkWidget) string {
	view := accessibleView(widget)
	if view == nil {
		return ""
	}
	return map[int]string{
		kindPassword:  "password",
		kindTextArea:  "textarea",
		kindActivity:  "busy",
		kindSection:   "disclosure",
		kindScroll:    "scroll",
		kindCanvas:    "canvas",
		kindDate:      "date",
		kindCalendar:  "calendar",
		kindColor:     "color",
		kindFont:      "font",
		kindSplit:     "split",
		kindWeb:       "web",
		kindVideo:     "video",
		kindDocument:  "document",
		kindStatusBar: "status",
	}[view.kind]
}
func nativeAccessibleName(widget *gtkWidget) string {
	if view := accessibleView(widget); view != nil {
		if view.accessibleName != "" {
			return view.accessibleName
		}
		if view.section != nil {
			return goString(gtkLabelGetText((*gtkLabel)(unsafe.Pointer(view.section.title))))
		}
	}
	pointer := unsafe.Pointer(widget)
	switch {
	case isNative(pointer, gtkLabelGetType()):
		return goString(gtkLabelGetText((*gtkLabel)(pointer)))
	case isNative(pointer, gtkButtonGetType()):
		return goString(gtkButtonGetLabel((*gtkButton)(pointer)))
	case isNative(pointer, gtkCheckButtonGetType()):
		return goString(gtkCheckButtonGetLabel((*gtkCheckButton)(pointer)))
	case isNative(pointer, gtkWindowGetType()):
		return goString(gtkWindowGetTitle((*gtkWindow)(pointer)))
	}
	return ""
}
func nativeAccessibleValue(widget *gtkWidget) string {
	if view := viewState(unsafe.Pointer(widget)); view != nil && view.accessibleValue != "" {
		return view.accessibleValue
	}
	pointer := unsafe.Pointer(widget)
	switch {
	case isNative(pointer, gtkEditableGetType()) && !isNative(pointer, gtkPasswordEntryGetType()):
		return goString(gtkEditableGetText((*gtkEditable)(pointer)))
	case isNative(pointer, gtkRangeGetType()):
		return fmt.Sprintf("%g", gtkRangeGetValue((*gtkRange)(pointer)))
	case isNative(pointer, gtkCheckButtonGetType()):
		if gtkCheckButtonGetActive((*gtkCheckButton)(pointer)) != 0 {
			return "1"
		}
		return "0"
	}
	return ""
}
func gkGTKAccessibilityTree(pointer unsafe.Pointer, size *cSize) *cChar {
	win := nativeWin(pointer)
	if win == nil {
		*size = 0
		return nil
	}
	var output strings.Builder
	count := 0
	clean := strings.NewReplacer("\x1e", " ", "\x1f", " ")
	roles := (*gEnumClass)(gTypeClassRef(gtkAccessibleRoleGetType()))
	defer gTypeClassUnref(unsafe.Pointer(roles))
	var appendNode func(*gtkAccessible, int)
	appendNode = func(element *gtkAccessible, depth int) {
		if element == nil || depth > 64 || count > 20000 {
			return
		}
		var widget *gtkWidget
		if isNative(unsafe.Pointer(element), gtkWidgetGetType()) {
			widget = (*gtkWidget)(unsafe.Pointer(element))
			if depth > 0 && (gtkWidgetGetVisible(widget) == 0 || gtkWidgetGetChildVisible(widget) == 0) {
				return
			}
		}
		count++
		identifier, name, value, subrole := "", "", "", ""
		bounds := nativeRect(0, 0, 0, 0)
		if widget != nil {
			if view := viewState(unsafe.Pointer(widget)); view != nil {
				identifier = view.accessibleID
			}
			name, value, subrole = nativeAccessibleName(widget), nativeAccessibleValue(widget), nativeSubrole(widget)
			if widget == win.root {
				bounds = nativeRect(0, 0, float64(gtkWidgetGetWidth(widget)), float64(gtkWidgetGetHeight(widget)))
			} else {
				gtkWidgetComputeBounds(widget, win.root, &bounds)
			}
		}
		role := ""
		if enum := gEnumGetValue(roles, cInt(gtkAccessibleGetAccessibleRole(element))); enum != nil {
			role = goString(enum.value_nick)
		}
		fmt.Fprintf(&output, "%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%.0f\x1f%.0f\x1f%.0f\x1f%.0f\x1e", depth, clean.Replace(role), clean.Replace(subrole), clean.Replace(identifier), clean.Replace(name), clean.Replace(value), bounds.origin.x, bounds.origin.y, bounds.size.width, bounds.size.height)
		for child := gtkAccessibleGetFirstAccessibleChild(element); child != nil; {
			appendNode(child, depth+1)
			next := gtkAccessibleGetNextAccessibleSibling(child)
			gObjectUnref(unsafe.Pointer(child))
			child = next
		}
	}
	appendNode((*gtkAccessible)(unsafe.Pointer(win.widget)), 0)
	*size = cSize(output.Len())
	return gStrdup(stringPointer(output.String()))
}
func gkGTKFocus(pointer unsafe.Pointer) cInt {
	view := viewState(pointer)
	if view == nil || gtkWidgetGetRoot(view.outer) == nil {
		return 0
	}
	target := view.control
	if gtkWidgetGetFocusable(target) == 0 && !isNative(unsafe.Pointer(target), gtkEditableGetType()) && !isNative(unsafe.Pointer(target), gtkButtonGetType()) && !isNative(unsafe.Pointer(target), gtkCheckButtonGetType()) {
		if gtkWidgetGetFocusable(view.outer) == 0 {
			return 0
		}
		target = view.outer
	}
	gtkWidgetSetCanFocus(target, 1)
	return cInt(gtkWidgetGrabFocus(target))
}
func gkGTKIsFocused(pointer unsafe.Pointer) cInt {
	view := viewState(pointer)
	if view == nil {
		return 0
	}
	root := gtkWidgetGetRoot(view.outer)
	if root == nil {
		return 0
	}
	focus := gtkRootGetFocus(root)
	return cInt(nativeBool(focus != nil && (focus == view.outer || gtkWidgetIsAncestor(focus, view.outer) != 0)))
}
func gkGTKAfterPoll() {
	for _, win := range nativeWindows {
		win.trackFocus()
	}
}

func gkGTKDisconnect(pointer unsafe.Pointer) {
	if view := viewState(pointer); view != nil {
		view.handle = 0
	}
}
func gkGTKSetTooltip(pointer unsafe.Pointer, text *cChar) {
	if view := viewState(pointer); view != nil {
		gtkWidgetSetTooltipText(view.outer, text)
	}
}
func gkGTKSetCursor(pointer unsafe.Pointer, cursor cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	names := []string{"", "pointer", "text", "crosshair", "move", "ew-resize", "ns-resize", "not-allowed", "wait", "grab", "grabbing"}
	var name *cChar
	if cursor >= 0 && int(cursor) < len(names) && names[cursor] != "" {
		name = stringPointer(names[cursor])
	}
	gtkWidgetSetCursorFromName(view.outer, name)
	if view.control != view.outer {
		gtkWidgetSetCursorFromName(view.control, name)
	}
}
func nativeColorCSS(property string, semantic int, rgba *cDouble) string {
	if rgba != nil {
		color := unsafe.Slice(rgba, 4)
		return fmt.Sprintf("%s: rgba(%d, %d, %d, %g); ", property, int(math.Round(float64(color[0])*255)), int(math.Round(float64(color[1])*255)), int(math.Round(float64(color[2])*255)), color[3])
	}
	if color := semanticCSSColor(semantic); color != "" {
		return property + ": " + color + "; "
	}
	return ""
}
func gkGTKSetStyle(pointer unsafe.Pointer, family *cChar, size cDouble, weight, italic, foreground, background cInt, foregroundRGBA, backgroundRGBA *cDouble, cornerRadius cDouble) cInt {
	view := viewState(pointer)
	if view == nil || !nativeFontFamilyInstalled(goString(family)) {
		return 0
	}
	var css strings.Builder
	font := goString(family)
	if font != "" && font != "System" {
		if font == "Monospace" {
			css.WriteString("font-family: monospace; ")
		} else {
			fmt.Fprintf(&css, "font-family: %q; ", font)
		}
	}
	if size > 0 {
		fmt.Fprintf(&css, "font-size: %gpx; ", size)
	}
	if weight > 0 {
		fmt.Fprintf(&css, "font-weight: %d; ", weight)
	}
	if italic != 0 {
		css.WriteString("font-style: italic; ")
	}
	color := nativeColorCSS("color", int(foreground), foregroundRGBA)
	css.WriteString(color)
	hasBackground := backgroundRGBA != nil || semanticCSSColor(int(background)) != ""
	button := isNative(unsafe.Pointer(view.control), gtkButtonGetType())
	if hasBackground {
		css.WriteString(nativeColorCSS("background-color", int(background), backgroundRGBA))
		css.WriteString("background-image: none; ")
		if button {
			css.WriteString("border: none; box-shadow: none; ")
		}
	}
	if cornerRadius > 0 {
		fmt.Fprintf(&css, "border-radius: %gpx; ", cornerRadius)
	}
	selector := "." + nativeWidgetCSSClass(view)
	setNativeCSS(selector, css.String())
	active := ""
	if hasBackground && button {
		active = "filter: brightness(1.3);"
	}
	setNativeCSS(selector+":active", active)
	if hasBackground && isNative(unsafe.Pointer(view.control), gtkTextViewGetType()) {
		color += nativeColorCSS("background-color", int(background), backgroundRGBA)
	}
	setNativeCSS(selector+" text", color)
	return 1
}
