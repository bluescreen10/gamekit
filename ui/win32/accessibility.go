//go:build windows && cgo

package win32

/*
#include "win32_bridge.h"
#include "window_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"strconv"
	"strings"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// registerWidget and unregisterWidget track live widgets so the window can
// clean up native state that refers to them.
func (w *Window) registerWidget(widget *widgetBase) {
	w.mu.Lock()
	if w.widgets == nil {
		w.widgets = make(map[*widgetBase]struct{})
	}
	w.widgets[widget] = struct{}{}
	w.mu.Unlock()
}

func (w *Window) unregisterWidget(widget *widgetBase, handle uintptr) {
	w.mu.Lock()
	delete(w.widgets, widget)
	w.mu.Unlock()
	if window := w.nativeWindowPointer(); window != nil {
		C.gkWin32ForgetFocus(window, C.uintptr_t(handle))
	}
}

// resource is a native object that a window creates and closes with itself.
type resource interface {
	Close()
}

// adoptResource makes the window close owned when the window closes.
func (w *Window) adoptResource(owned resource) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.resources == nil {
		w.resources = make(map[resource]struct{})
	}
	w.resources[owned] = struct{}{}
}

// releaseResource removes a closed resource from the window.
func (w *Window) releaseResource(owned resource) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.resources, owned)
}

// closeWidgets closes every widget that the window created and that is still
// open. A widget that is not in the content tree, or that a closed container
// did not close, would otherwise keep its native view and its handles.
func (w *Window) closeWidgets() {
	for {
		w.mu.Lock()
		var next *widgetBase
		for widget := range w.widgets {
			next = widget
			break
		}
		w.mu.Unlock()
		if next == nil {
			return
		}
		next.closeComponent()
	}
}

// trackFocus reports focus changes since the last check.
func (w *Window) trackFocus() {
	if window := w.nativeWindowPointer(); window != nil {
		C.gkWin32TrackFocus(window)
	}
}

// nativeWindowPointer returns the native window record of the native layer.
func (w *Window) nativeWindowPointer() unsafe.Pointer {
	native := w.nativePointer()
	if native == nil {
		return nil
	}
	return C.gkWin32WindowNativePointer(native)
}

// controlTypes maps the names of UI Automation control types to ui roles.
var controlTypes = map[string]ui.AccessibilityRole{
	"Window":      ui.RoleWindow,
	"Pane":        ui.RoleGroup,
	"Group":       ui.RoleGroup,
	"Custom":      ui.RoleGroup,
	"Text":        ui.RoleLabel,
	"Button":      ui.RoleButton,
	"SplitButton": ui.RoleButton,
	"Edit":        ui.RoleTextField,
	"CheckBox":    ui.RoleCheckBox,
	"RadioButton": ui.RoleRadioButton,
	"ComboBox":    ui.RoleComboBox,
	"Slider":      ui.RoleSlider,
	"Spinner":     ui.RoleSpinButton,
	"ProgressBar": ui.RoleProgressBar,
	"Separator":   ui.RoleSeparator,
	"Image":       ui.RoleImage,
	"Hyperlink":   ui.RoleLink,
	"Tab":         ui.RoleTabList,
	"TabItem":     ui.RoleTab,
	"List":        ui.RoleList,
	"ListItem":    ui.RoleListItem,
	"DataItem":    ui.RoleListItem,
	"TreeItem":    ui.RoleListItem,
	"Table":       ui.RoleTable,
	"DataGrid":    ui.RoleGrid,
	"Tree":        ui.RoleTree,
	"Menu":        ui.RoleMenu,
	"MenuBar":     ui.RoleMenuBar,
	"MenuItem":    ui.RoleMenuItem,
	"ToolBar":     ui.RoleToolbar,
	"AppBar":      ui.RoleToolbar,
	"StatusBar":   ui.RoleStatus,
	"ToolTip":     ui.RoleTooltip,
	"Document":    ui.RoleDocument,
	"Calendar":    ui.RoleCalendar,
	"ScrollBar":   ui.RoleScrollView,
}

// componentRoles maps the component names that the native layer reports for
// elements whose control type is too general.
var componentRoles = map[string]ui.AccessibilityRole{
	"password":   ui.RolePasswordField,
	"textarea":   ui.RoleTextArea,
	"busy":       ui.RoleBusyIndicator,
	"disclosure": ui.RoleDisclosure,
	"scroll":     ui.RoleScrollView,
	"canvas":     ui.RoleCanvas,
	"date":       ui.RoleDatePicker,
	"calendar":   ui.RoleCalendar,
	"color":      ui.RoleColorPicker,
	"font":       ui.RoleFontPicker,
	"split":      ui.RoleSplitView,
	"web":        ui.RoleWebView,
	"video":      ui.RoleVideo,
	"document":   ui.RoleDocument,
	"status":     ui.RoleStatus,
	"switch":     ui.RoleSwitch,
	"toggle":     ui.RoleToggleButton,
	"search":     ui.RoleSearchField,
	"spin":       ui.RoleSpinButton,
	"radiogroup": ui.RoleRadioGroup,
	"slider":     ui.RoleSlider,
	"separator":  ui.RoleSeparator,
	"tablist":    ui.RoleTabList,
	"navigation": ui.RoleNavigation,
	"alert":      ui.RoleAlert,
	"group":      ui.RoleGroup,
}

func accessibilityRole(controlType, component string) ui.AccessibilityRole {
	if mapped, ok := componentRoles[component]; ok {
		return mapped
	}
	return controlTypes[controlType]
}

// AccessibilityTree returns the window's accessibility hierarchy as UI
// Automation exposes it to assistive technologies. Bounds are in content
// coordinates.
func (w *Window) AccessibilityTree() ui.AccessibilityNode {
	window := w.nativeWindowPointer()
	if window == nil {
		return ui.AccessibilityNode{Role: ui.RoleWindow}
	}
	var size C.size_t
	buffer := C.gkWin32AccessibilityTree(window, &size)
	if buffer == nil {
		return ui.AccessibilityNode{Role: ui.RoleWindow}
	}
	defer C.free(unsafe.Pointer(buffer))
	return parseAccessibilityTree(C.GoStringN(buffer, C.int(size)))
}

// parseAccessibilityTree builds nodes from depth-first records separated by
// U+001E, with fields separated by U+001F.
func parseAccessibilityTree(data string) ui.AccessibilityNode {
	type frame struct {
		depth int
		node  *ui.AccessibilityNode
	}
	var root ui.AccessibilityNode
	var stack []frame
	for _, record := range strings.Split(data, "\x1e") {
		fields := strings.Split(record, "\x1f")
		if len(fields) < 10 {
			continue
		}
		depth, _ := strconv.Atoi(fields[0])
		number := func(index int) int {
			value, _ := strconv.ParseFloat(fields[index], 64)
			return int(value)
		}
		node := ui.AccessibilityNode{
			Role:  accessibilityRole(fields[1], fields[2]),
			ID:    fields[3],
			Name:  fields[4],
			Value: fields[5],
			Bounds: ui.Rect{
				Position: ui.Point{X: number(6), Y: number(7)},
				Size:     ui.Size{Width: number(8), Height: number(9)},
			},
		}
		if depth == 0 || len(stack) == 0 {
			root = node
			stack = []frame{{depth: 0, node: &root}}
			continue
		}
		for len(stack) > 1 && stack[len(stack)-1].depth >= depth {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1].node
		parent.Children = append(parent.Children, node)
		stack = append(stack, frame{depth: depth, node: &parent.Children[len(parent.Children)-1]})
	}
	return root
}
