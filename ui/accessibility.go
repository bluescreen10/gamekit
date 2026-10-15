package ui

import (
	"errors"
	"fmt"
)

// AccessibilityRole is the platform-neutral semantic role that a backend maps
// to its native accessibility role.
type AccessibilityRole uint8

const (
	// RoleUnknown has no known role.
	RoleUnknown AccessibilityRole = iota
	// RoleWindow is a window.
	RoleWindow
	// RoleGroup is a group of related elements.
	RoleGroup
	// RoleLabel is a label.
	RoleLabel
	// RoleButton is a button.
	RoleButton
	// RoleToggleButton is a toggle button.
	RoleToggleButton
	// RoleSwitch is a switch.
	RoleSwitch
	// RoleTextField is a text field.
	RoleTextField
	// RolePasswordField is a password field.
	RolePasswordField
	// RoleSearchField is a search field.
	RoleSearchField
	// RoleTextArea is a text area.
	RoleTextArea
	// RoleCheckBox is a check box.
	RoleCheckBox
	// RoleRadioGroup is a radio group.
	RoleRadioGroup
	// RoleRadioButton is a radio button.
	RoleRadioButton
	// RoleComboBox is a list of choices that opens from a button or field.
	RoleComboBox
	// RoleSlider is a slider.
	RoleSlider
	// RoleSpinButton is a field with buttons that step its value.
	RoleSpinButton
	// RoleProgressBar is a progress bar.
	RoleProgressBar
	// RoleBusyIndicator is an indicator of activity without known progress.
	RoleBusyIndicator
	// RoleSeparator is a separator.
	RoleSeparator
	// RoleImage is an image.
	RoleImage
	// RoleLink is a link.
	RoleLink
	// RoleTabList is a set of tabs.
	RoleTabList
	// RoleTab is a tab.
	RoleTab
	// RoleScrollView is a scroll view.
	RoleScrollView
	// RoleCanvas is an area that the application draws.
	RoleCanvas
	// RoleDisclosure is a section that the user can expand and collapse.
	RoleDisclosure
	// RoleList is a list.
	RoleList
	// RoleListItem is a list item.
	RoleListItem
	// RoleTable is a table.
	RoleTable
	// RoleTree is a tree.
	RoleTree
	// RoleGrid is a grid of items.
	RoleGrid
	// RoleMenu is a menu.
	RoleMenu
	// RoleMenuBar is a menu bar.
	RoleMenuBar
	// RoleMenuItem is a menu item.
	RoleMenuItem
	// RoleToolbar is a toolbar.
	RoleToolbar
	// RoleStatus is an area that shows status information.
	RoleStatus
	// RoleAlert is an important message.
	RoleAlert
	// RoleDialog is a dialog.
	RoleDialog
	// RoleDatePicker is a date picker.
	RoleDatePicker
	// RoleTimePicker is a time picker.
	RoleTimePicker
	// RoleColorPicker is a color picker.
	RoleColorPicker
	// RoleFontPicker is a font picker.
	RoleFontPicker
	// RoleCalendar is a calendar.
	RoleCalendar
	// RoleSplitView is a set of panes with dividers.
	RoleSplitView
	// RoleNavigation is a list of navigation destinations.
	RoleNavigation
	// RoleTooltip is a tooltip.
	RoleTooltip
	// RoleDocument is a view of a document.
	RoleDocument
	// RoleWebView is a view of web content.
	RoleWebView
	// RoleVideo is a video player.
	RoleVideo
)

var roleNames = [...]string{
	"unknown", "window", "group", "label", "button", "toggle button", "switch",
	"text field", "password field", "search field", "text area", "check box",
	"radio group", "radio button", "combo box", "slider", "spin button",
	"progress bar", "busy indicator", "separator", "image", "link", "tab list",
	"tab", "scroll view", "canvas", "disclosure", "list", "list item", "table",
	"tree", "grid", "menu", "menu bar", "menu item", "toolbar", "status",
	"alert", "dialog", "date picker", "time picker", "color picker",
	"font picker", "calendar", "split view", "navigation", "tooltip",
	"document", "web view", "video",
}

// String returns a human-readable role name.
func (role AccessibilityRole) String() string {
	if int(role) < len(roleNames) {
		return roleNames[role]
	}
	return fmt.Sprintf("AccessibilityRole(%d)", role)
}

// LiveRegion controls how assistive technologies announce changes.
type LiveRegion uint8

const (
	// LiveOff does not announce changes.
	LiveOff LiveRegion = iota
	// LivePolite announces changes when the user is idle.
	LivePolite
	// LiveAssertive interrupts the user to announce changes.
	LiveAssertive
)

// Accessibility is the metadata a widget exposes to assistive technologies in
// addition to its native role and value. ID is a stable automation identifier.
// LabeledBy and DescribedBy must contain widgets from the same window.
type Accessibility struct {
	// ID is the identifier that automation and UI tests use.
	ID string
	// Name is the text that assistive technologies read for the widget.
	Name string
	// Description is help text that assistive technologies read after the
	// name.
	Description string
	// ValueText describes the value in words, for example "3 of 10".
	ValueText string
	// LabeledBy are the widgets that name this widget.
	LabeledBy []Widget
	// DescribedBy are the widgets that describe this widget.
	DescribedBy []Widget
	// IsRequired marks a field that the user must fill.
	IsRequired bool
	// IsInvalid marks a field whose value is not accepted.
	IsInvalid bool
	// IsHidden removes the widget from the accessibility tree.
	IsHidden bool
	// Live makes assistive technologies announce changes to the text of
	// the widget.
	Live LiveRegion
}

// Validate checks that the metadata uses recognized values.
func (metadata Accessibility) Validate() error {
	if metadata.Live > LiveAssertive {
		return errors.New("ui: invalid live-region priority")
	}
	for _, widget := range metadata.LabeledBy {
		if widget == nil {
			return errors.New("ui: accessibility LabeledBy contains a nil widget")
		}
	}
	for _, widget := range metadata.DescribedBy {
		if widget == nil {
			return errors.New("ui: accessibility DescribedBy contains a nil widget")
		}
	}
	return nil
}

// AccessibilityNode is one element of a window's accessibility tree, as the
// backend exposes it to the platform.
type AccessibilityNode struct {
	// Role is the semantic role of the element.
	Role AccessibilityRole
	// ID is the automation identifier of the element.
	ID string
	// Name is the text that assistive technologies read for the element.
	Name string
	// Value is the value of the element as text.
	Value string
	// Bounds is the area of the element in window coordinates.
	Bounds Rect
	// Hidden is set when assistive technologies do not show the element.
	Hidden bool
	// Children are the elements inside this element.
	Children []AccessibilityNode
}

// FindByID returns the first node, in depth-first order, whose ID matches id.
func (node *AccessibilityNode) FindByID(id string) *AccessibilityNode {
	if node == nil || id == "" {
		return nil
	}
	if node.ID == id {
		return node
	}
	for i := range node.Children {
		if found := node.Children[i].FindByID(id); found != nil {
			return found
		}
	}
	return nil
}
