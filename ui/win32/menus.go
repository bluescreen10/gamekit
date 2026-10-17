//go:build windows && cgo

package win32

/*
#include "win32_bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime/cgo"
	"sync"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// menu is a GMenu built from action IDs. Actions come from the menu's
// own options or, when it is placed in a menu bar, from the bar.
type menu struct {
	window  *Window
	mu      sync.RWMutex
	native  unsafe.Pointer
	handle  cgo.Handle
	actions []ui.Action
	items   []ui.MenuItem
	// inherited are actions supplied by the container that shows the menu.
	inherited []ui.Action
	closed    bool
	listeners listeners[ui.ActionHandler]
}

func newMenu(options ui.MenuOptions) (*menu, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &menu{
		actions: append([]ui.Action(nil), options.Actions...),
		items:   cloneMenuItems(options.Items),
	}
	result.handle = cgo.NewHandle(result)
	result.native = C.gkWin32MenuCreate(C.uintptr_t(result.handle), nil)
	if result.native == nil {
		result.handle.Delete()
		return nil, errors.New("ui/win32: native backend could not create a menu")
	}
	result.rebuild()
	return result, nil
}

func cloneMenuItems(items []ui.MenuItem) []ui.MenuItem {
	if items == nil {
		return nil
	}
	result := make([]ui.MenuItem, len(items))
	for i, item := range items {
		item.Items = cloneMenuItems(item.Items)
		result[i] = item
	}
	return result
}

func (m *menu) win32Event(event win32Event) {
	if event.kind != C.GK_EVENT_ACTION {
		return
	}
	for _, handler := range m.listeners.snapshot() {
		handler(event.text)
	}
}

func (m *menu) nativeMenu() unsafe.Pointer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil
	}
	return m.native
}

func (m *menu) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	native := m.native
	handle := m.handle
	m.native = nil
	m.mu.Unlock()
	C.gkWin32MenuSetHandle(native, 0)
	C.gkWin32Release(native)
	handle.Delete()
	m.window.releaseResource(m)
}

func (m *menu) Items() []ui.MenuItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneMenuItems(m.items)
}

func (m *menu) SetItems(items []ui.MenuItem) error {
	if err := (ui.MenuOptions{Items: items}).Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ui.ErrClosed
	}
	m.items = cloneMenuItems(items)
	m.mu.Unlock()
	m.rebuild()
	return nil
}

func (m *menu) Actions() []ui.Action {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]ui.Action(nil), m.actions...)
}

func (m *menu) SetActions(actions []ui.Action) error {
	if err := (ui.MenuOptions{Actions: actions}).Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ui.ErrClosed
	}
	m.actions = append([]ui.Action(nil), actions...)
	m.mu.Unlock()
	m.rebuild()
	return nil
}

func (m *menu) OnAction(handler ui.ActionHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return m.listeners.add(handler)
}

// inherit supplies actions from a menu bar or other container.
func (m *menu) inherit(actions []ui.Action) {
	m.mu.Lock()
	m.inherited = append([]ui.Action(nil), actions...)
	m.mu.Unlock()
	m.rebuild()
}

func (m *menu) lookup(id string) (ui.Action, bool) {
	for _, action := range m.actions {
		if action.ID == id {
			return action, true
		}
	}
	for _, action := range m.inherited {
		if action.ID == id {
			return action, true
		}
	}
	return ui.Action{}, false
}

func (m *menu) rebuild() {
	m.mu.RLock()
	native := m.native
	items := cloneMenuItems(m.items)
	closed := m.closed
	m.mu.RUnlock()
	if closed || native == nil {
		return
	}
	C.gkWin32MenuClear(native)
	m.build(native, items)
}

func (m *menu) build(native unsafe.Pointer, items []ui.MenuItem) {
	for _, item := range items {
		switch {
		case item.Separator:
			C.gkWin32MenuAddSeparator(native)
		case item.ActionID != "":
			action, ok := m.lookup(item.ActionID)
			if !ok {
				// The containing bar may supply the action later.
				action = ui.Action{ID: item.ActionID, Text: item.ActionID}
			}
			addMenuAction(native, action)
		default:
			title := cString(item.Title)
			submenu := C.gkWin32MenuAddSubmenu(native, title)
			freeCString(title)
			m.build(submenu, item.Items)
		}
	}
}

func addMenuAction(native unsafe.Pointer, action ui.Action) {
	id := cString(action.ID)
	text := cString(action.Text)
	tooltip := cString(action.Tooltip)
	key, modifiers := keyEquivalent(action.Shortcut)
	keyValue := cString(key)
	defer freeCString(id)
	defer freeCString(text)
	defer freeCString(tooltip)
	defer freeCString(keyValue)
	icon := nativeIcon(action.Icon, 16)
	C.gkWin32MenuAddItem(native, id, text, boolInt(action.Enabled), boolInt(action.Checked),
		keyValue, C.int(modifiers), icon, tooltip)
	C.gkWin32Release(icon)
}

// setActionState updates every item for id; -1 leaves a field unchanged.
func (m *menu) setActionState(id string, enabled, checked int) {
	if native := m.nativeMenu(); native != nil {
		value := cString(id)
		defer freeCString(value)
		C.gkWin32MenuSetActionState(native, value, C.int(enabled), C.int(checked))
	}
}

// contextMenu is a menu that a widget shows for a secondary click.
type contextMenu struct {
	*menu
}

func (c *contextMenu) Popup(widget ui.Widget, position ui.Point) error {
	owner, ok := widget.(nativeWidget)
	if !ok {
		return errors.New("ui/win32: widget belongs to another backend")
	}
	view := owner.base().nativePointer()
	native := c.nativeMenu()
	if view == nil || native == nil {
		return ui.ErrClosed
	}
	C.gkWin32MenuPopup(native, view, C.double(position.X), C.double(position.Y))
	return nil
}

// CreateMenu creates a native menu.
func (w *Window) CreateMenu(options ui.MenuOptions) (ui.Menu, error) {
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}
	result, err := newMenu(options)
	if err != nil {
		return nil, err
	}
	result.window = w
	w.adoptResource(result)
	return result, nil
}

// CreateContextMenu creates a native context menu.
func (w *Window) CreateContextMenu(options ui.ContextMenuOptions) (ui.ContextMenu, error) {
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}
	result, err := newMenu(ui.MenuOptions(options))
	if err != nil {
		return nil, err
	}
	result.window = w
	w.adoptResource(result)
	return &contextMenu{menu: result}, nil
}

// ownedMenu returns the native menu behind a ui.Menu.
func ownedMenu(value ui.Menu) (*menu, error) {
	switch owned := value.(type) {
	case *menu:
		return owned, nil
	case *contextMenu:
		return owned.menu, nil
	default:
		return nil, fmt.Errorf("ui/win32: menu %T belongs to another backend", value)
	}
}

var _ ui.Menu = (*menu)(nil)
var _ ui.ContextMenu = (*contextMenu)(nil)
