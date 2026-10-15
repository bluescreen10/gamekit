package ui

import (
	"errors"
	"fmt"
)

// Shortcut is a keyboard shortcut. Key is a key name such as "S", "F5", or
// "Backspace". Command is the platform's primary command modifier: Command on
// macOS and Control elsewhere.
type Shortcut struct {
	// Key is the name of the key.
	Key string

	// Command adds the primary command modifier of the platform.
	Command bool

	// Control adds the Control modifier. On platforms other than macOS,
	// Command already uses Control.
	Control bool

	// Alt adds the Alt modifier, which is Option on macOS.
	Alt bool

	// Shift adds the Shift modifier.
	Shift bool
}

// IsZero reports whether the shortcut has no key.
func (shortcut Shortcut) IsZero() bool {
	return shortcut.Key == ""
}

// Action is a command that menus, toolbars, and command bars present. IDs are
// stable and unique within one options value.
type Action struct {
	// ID identifies the action in handlers and in item references.
	ID string

	// Text is the label of the action.
	Text string

	// Icon is the image of the action. The zero value shows no image.
	Icon IconSource

	// Tooltip is the help text of the action.
	Tooltip string

	// Enabled lets the user activate the action.
	Enabled bool

	// Checked shows a check mark next to the action.
	Checked bool

	// Shortcut is the keyboard shortcut of the action.
	Shortcut Shortcut
}

// ActionHandler receives the ID of an activated action.
type ActionHandler func(id string)

func validateActions(actions []Action) error {
	seen := make(map[string]struct{}, len(actions))
	for i, action := range actions {
		if action.ID == "" {
			return fmt.Errorf("ui: action %d has no ID", i)
		}
		if _, exists := seen[action.ID]; exists {
			return fmt.Errorf("ui: duplicate action ID %q", action.ID)
		}
		seen[action.ID] = struct{}{}
	}
	return nil
}

// MenuItem is one entry in a menu. It references an Action by ID, separates
// groups, or opens a submenu with Title and Items.
type MenuItem struct {
	// ActionID is the ID of the action that the item activates.
	ActionID string

	// Separator makes the item a line between groups.
	Separator bool

	// Title is the label of a submenu.
	Title string

	// Items are the entries of a submenu.
	Items []MenuItem
}

func validateMenuItems(items []MenuItem) error {
	for i, item := range items {
		kinds := 0
		if item.ActionID != "" {
			kinds++
		}
		if item.Separator {
			kinds++
		}
		if item.Title != "" || len(item.Items) > 0 {
			kinds++
		}
		if kinds != 1 {
			return fmt.Errorf("ui: menu item %d must be an action, separator, or submenu", i)
		}
		if err := validateMenuItems(item.Items); err != nil {
			return err
		}
	}
	return nil
}

// MenuOptions describes a native menu. Actions may be empty when the menu is
// placed in a MenuBar that supplies them.
type MenuOptions struct {
	// Actions are the actions that the items reference.
	Actions []Action

	// Items are the entries of the menu in order.
	Items []MenuItem
}

// Validate checks the menu's actions and item structure.
func (options MenuOptions) Validate() error {
	if err := validateActions(options.Actions); err != nil {
		return err
	}
	return validateMenuItems(options.Items)
}

// Menu is a native menu that can be placed in a menu bar, a system tray, or a
// submenu. OnAction receives activations of its items.
type Menu interface {
	// Close releases the menu. After Close, the methods that return an
	// error return ErrClosed.
	Close()

	// Items returns the entries of the menu.
	Items() []MenuItem

	// SetItems replaces the entries of the menu.
	SetItems(items []MenuItem) error

	// Actions returns the actions of the menu.
	Actions() []Action

	// SetActions replaces the actions of the menu.
	SetActions(actions []Action) error

	// OnAction receives the ID of each action that the user activates.
	OnAction(handler ActionHandler) Unsubscribe
}

// NamedMenu is one top-level menu in a menu bar.
type NamedMenu struct {
	// Title is the label of the menu in the menu bar.
	Title string

	// Menu is the menu that opens from the title.
	Menu Menu
}

// MenuBarOptions describes the window's or application's menu bar. On macOS
// it becomes the application main menu while the window is key.
type MenuBarOptions struct {
	// Actions are the actions that the menus reference. The menus use them
	// in addition to their own actions.
	Actions []Action

	// Menus are the top-level menus in order.
	Menus []NamedMenu
}

// Validate checks the menu bar's actions and menus.
func (options MenuBarOptions) Validate() error {
	if err := validateActions(options.Actions); err != nil {
		return err
	}
	for i, menu := range options.Menus {
		if menu.Title == "" || menu.Menu == nil {
			return fmt.Errorf("ui: menu bar menu %d requires a title and a menu", i)
		}
	}
	return nil
}

// MenuBar is a native menu bar.
type MenuBar interface {
	// Close removes the menu bar. The menus stay open.
	Close()

	// Menus returns the top-level menus.
	Menus() []NamedMenu

	// SetActionEnabled enables or disables a menu bar action.
	SetActionEnabled(id string, enabled bool) error

	// SetActionChecked shows or removes the check mark of a menu bar action.
	SetActionChecked(id string, checked bool) error

	// OnAction receives the ID of each action that the user activates in
	// any menu of the bar.
	OnAction(handler ActionHandler) Unsubscribe
}

// ContextMenuOptions describes a native context menu.
type ContextMenuOptions struct {
	// Actions are the actions that the items reference.
	Actions []Action

	// Items are the entries of the menu in order.
	Items []MenuItem
}

// Validate checks the menu's actions and item structure.
func (options ContextMenuOptions) Validate() error {
	return MenuOptions(options).Validate()
}

// ContextMenu is a native menu shown for a secondary click on a widget.
type ContextMenu interface {
	Menu
	// Popup shows the menu at position in widget coordinates.
	Popup(widget Widget, position Point) error
}

// ToolbarItem is one toolbar entry.
type ToolbarItem struct {
	// ActionID is the ID of the action that the item activates.
	ActionID string

	// Separator makes the item a line between groups.
	Separator bool

	// Space makes the item a fixed space.
	Space bool

	// FlexibleSpace makes the item a space that fills the free width.
	FlexibleSpace bool
}

// ToolbarOptions describes a native window toolbar.
type ToolbarOptions struct {
	// Actions are the actions that the items reference.
	Actions []Action

	// Items are the entries of the toolbar in order.
	Items []ToolbarItem

	// ShowsLabels shows the text of each action below its icon.
	ShowsLabels bool
}

// Validate checks the toolbar's actions and items.
func (options ToolbarOptions) Validate() error {
	if err := validateActions(options.Actions); err != nil {
		return err
	}
	for i, item := range options.Items {
		kinds := 0
		for _, set := range []bool{item.ActionID != "", item.Separator, item.Space, item.FlexibleSpace} {
			if set {
				kinds++
			}
		}
		if kinds != 1 {
			return fmt.Errorf("ui: toolbar item %d must have exactly one kind", i)
		}
	}
	return nil
}

// Toolbar is a native window toolbar.
type Toolbar interface {
	// Close removes the toolbar from the window.
	Close()

	// Items returns the entries of the toolbar.
	Items() []ToolbarItem

	// SetActionEnabled enables or disables the items of an action.
	SetActionEnabled(id string, enabled bool) error

	// OnAction receives the ID of each action that the user activates.
	OnAction(handler ActionHandler) Unsubscribe
}

// CommandBarOptions describes an in-content command bar.
type CommandBarOptions struct {
	// Actions are the buttons of the bar in order.
	Actions []Action

	// Compact shows icons without text.
	Compact bool
}

// Validate checks the command bar's actions.
func (options CommandBarOptions) Validate() error {
	if len(options.Actions) == 0 {
		return errors.New("ui: command bar requires at least one action")
	}
	return validateActions(options.Actions)
}

// CommandBar is a row of action buttons with overflow.
type CommandBar interface {
	Widget
	// Actions returns the actions of the bar.
	Actions() []Action

	// SetActions replaces the actions of the bar.
	SetActions(actions []Action) error

	// OnAction receives the ID of each action that the user activates.
	OnAction(handler ActionHandler) Unsubscribe
}

// Command is one entry in a command palette.
type Command struct {
	// ID identifies the command in OnActivate handlers.
	ID string

	// Text is the name of the command that the search matches.
	Text string

	// Detail is the secondary text of the command.
	Detail string

	// Shortcut is the keyboard shortcut that the palette shows for the
	// command.
	Shortcut Shortcut

	// Enabled lets the user activate the command.
	Enabled bool
}

// CommandPaletteOptions describes a searchable command list.
type CommandPaletteOptions struct {
	// Commands are the entries of the palette.
	Commands []Command

	// Placeholder is the text that the empty search field shows.
	Placeholder string
}

// Validate checks that command IDs are present and unique.
func (options CommandPaletteOptions) Validate() error {
	seen := make(map[string]struct{}, len(options.Commands))
	for i, command := range options.Commands {
		if command.ID == "" {
			return fmt.Errorf("ui: command %d has no ID", i)
		}
		if _, exists := seen[command.ID]; exists {
			return fmt.Errorf("ui: duplicate command ID %q", command.ID)
		}
		seen[command.ID] = struct{}{}
	}
	return nil
}

// CommandPalette is a searchable command list. OnActivate receives the chosen
// command ID.
type CommandPalette interface {
	Widget

	// Commands returns the entries of the palette.
	Commands() []Command

	// SetCommands replaces the entries of the palette.
	SetCommands(commands []Command) error

	// Query returns the text in the search field.
	Query() string

	// SetQuery changes the text in the search field and filters the
	// commands.
	SetQuery(query string)

	// OnActivate receives the ID of each command that the user activates.
	OnActivate(handler ActionHandler) Unsubscribe
}
