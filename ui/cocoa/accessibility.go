//go:build darwin && cgo

package cocoa

/*
#include "cocoa_bridge.h"
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
		C.gkCocoaForgetFocus(window, C.uintptr_t(handle))
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
		C.gkCocoaTrackFocus(window)
	}
}

// nativeWindowPointer returns the NSWindow.
func (w *Window) nativeWindowPointer() unsafe.Pointer {
	native := w.nativePointer()
	if native == nil {
		return nil
	}
	return C.gkWindowNativePointer(native)
}

// axRoles maps AppKit accessibility roles to ui roles.
var axRoles = map[string]ui.AccessibilityRole{
	"AXWindow":             ui.RoleWindow,
	"AXSheet":              ui.RoleDialog,
	"AXDialog":             ui.RoleDialog,
	"AXGroup":              ui.RoleGroup,
	"AXSplitGroup":         ui.RoleSplitView,
	"AXStaticText":         ui.RoleLabel,
	"AXButton":             ui.RoleButton,
	"AXMenuButton":         ui.RoleButton,
	"AXPopUpButton":        ui.RoleComboBox,
	"AXComboBox":           ui.RoleComboBox,
	"AXTextField":          ui.RoleTextField,
	"AXTextArea":           ui.RoleTextArea,
	"AXCheckBox":           ui.RoleCheckBox,
	"AXRadioGroup":         ui.RoleRadioGroup,
	"AXRadioButton":        ui.RoleRadioButton,
	"AXSlider":             ui.RoleSlider,
	"AXIncrementor":        ui.RoleSpinButton,
	"AXProgressIndicator":  ui.RoleProgressBar,
	"AXBusyIndicator":      ui.RoleBusyIndicator,
	"AXSplitter":           ui.RoleSeparator,
	"AXImage":              ui.RoleImage,
	"AXLink":               ui.RoleLink,
	"AXTabGroup":           ui.RoleTabList,
	"AXScrollArea":         ui.RoleScrollView,
	"AXDisclosureTriangle": ui.RoleDisclosure,
	"AXList":               ui.RoleList,
	"AXTable":              ui.RoleTable,
	"AXRow":                ui.RoleListItem,
	"AXOutline":            ui.RoleTree,
	"AXGrid":               ui.RoleGrid,
	"AXMenu":               ui.RoleMenu,
	"AXMenuBar":            ui.RoleMenuBar,
	"AXMenuItem":           ui.RoleMenuItem,
	"AXToolbar":            ui.RoleToolbar,
	"AXDateField":          ui.RoleDatePicker,
	"AXColorWell":          ui.RoleColorPicker,
	"AXWebArea":            ui.RoleWebView,
	"AXLayoutArea":         ui.RoleCanvas,
	"AXHelpTag":            ui.RoleTooltip,
}

var axSubroles = map[string]ui.AccessibilityRole{
	"AXSecureTextField": ui.RolePasswordField,
	"AXSearchField":     ui.RoleSearchField,
	"AXSwitch":          ui.RoleSwitch,
	"AXToggle":          ui.RoleToggleButton,
	"AXTabButton":       ui.RoleTab,
	"AXOutlineRow":      ui.RoleListItem,
}

func accessibilityRole(role, subrole string) ui.AccessibilityRole {
	if mapped, ok := axSubroles[subrole]; ok {
		return mapped
	}
	return axRoles[role]
}

// AccessibilityTree returns the window's accessibility hierarchy as AppKit
// exposes it to assistive technologies. Bounds are in content coordinates.
func (w *Window) AccessibilityTree() ui.AccessibilityNode {
	window := w.nativeWindowPointer()
	if window == nil {
		return ui.AccessibilityNode{Role: ui.RoleWindow}
	}
	var size C.size_t
	buffer := C.gkCocoaAccessibilityTree(window, &size)
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
