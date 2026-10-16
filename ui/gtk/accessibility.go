//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
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
		C.gkGTKForgetFocus(window, C.uintptr_t(handle))
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
		C.gkGTKTrackFocus(window)
	}
}

// nativeWindowPointer returns the native window record of the GTK layer.
func (w *Window) nativeWindowPointer() unsafe.Pointer {
	native := w.nativePointer()
	if native == nil {
		return nil
	}
	return C.gkGTKWindowNativePointer(native)
}

// gtkRoles maps the nicks of GtkAccessibleRole values to ui roles.
var gtkRoles = map[string]ui.AccessibilityRole{
	"window":             ui.RoleWindow,
	"application":        ui.RoleCanvas,
	"dialog":             ui.RoleDialog,
	"alert-dialog":       ui.RoleDialog,
	"alert":              ui.RoleAlert,
	"group":              ui.RoleGroup,
	"generic":            ui.RoleGroup,
	"label":              ui.RoleLabel,
	"button":             ui.RoleButton,
	"toggle-button":      ui.RoleToggleButton,
	"switch":             ui.RoleSwitch,
	"text-box":           ui.RoleTextField,
	"search-box":         ui.RoleSearchField,
	"checkbox":           ui.RoleCheckBox,
	"radio-group":        ui.RoleRadioGroup,
	"radio":              ui.RoleRadioButton,
	"combo-box":          ui.RoleComboBox,
	"slider":             ui.RoleSlider,
	"spin-button":        ui.RoleSpinButton,
	"progress-bar":       ui.RoleProgressBar,
	"meter":              ui.RoleProgressBar,
	"separator":          ui.RoleSeparator,
	"img":                ui.RoleImage,
	"link":               ui.RoleLink,
	"tab-list":           ui.RoleTabList,
	"tab":                ui.RoleTab,
	"list":               ui.RoleList,
	"list-box":           ui.RoleList,
	"list-item":          ui.RoleListItem,
	"row":                ui.RoleListItem,
	"tree-item":          ui.RoleListItem,
	"table":              ui.RoleTable,
	"tree":               ui.RoleTree,
	"tree-grid":          ui.RoleTree,
	"grid":               ui.RoleGrid,
	"menu":               ui.RoleMenu,
	"menu-bar":           ui.RoleMenuBar,
	"menu-item":          ui.RoleMenuItem,
	"menu-item-checkbox": ui.RoleMenuItem,
	"menu-item-radio":    ui.RoleMenuItem,
	"toolbar":            ui.RoleToolbar,
	"status":             ui.RoleStatus,
	"navigation":         ui.RoleNavigation,
	"tooltip":            ui.RoleTooltip,
	"document":           ui.RoleDocument,
}

// gtkSubroles maps the component names that the GTK layer reports for
// widgets whose GTK role is too general.
var gtkSubroles = map[string]ui.AccessibilityRole{
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
}

func accessibilityRole(role, subrole string) ui.AccessibilityRole {
	if mapped, ok := gtkSubroles[subrole]; ok {
		return mapped
	}
	return gtkRoles[role]
}

// AccessibilityTree returns the window's accessibility hierarchy as GTK
// exposes it to assistive technologies. Bounds are in content coordinates.
func (w *Window) AccessibilityTree() ui.AccessibilityNode {
	window := w.nativeWindowPointer()
	if window == nil {
		return ui.AccessibilityNode{Role: ui.RoleWindow}
	}
	var size C.size_t
	buffer := C.gkGTKAccessibilityTree(window, &size)
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
