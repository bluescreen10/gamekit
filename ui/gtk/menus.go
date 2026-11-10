//go:build linux && cgo

package gtk

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
	native  *nativeMenu
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
	result.native = newNativeMenu(uintptr(result.handle))
	if result.native == nil {
		result.handle.Delete()
		return nil, errors.New("ui/gtk: native backend could not create a menu")
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

func (m *menu) gtkEvent(event gtkEvent) {
	if event.kind != gkEventAction {
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
	return unsafe.Pointer(m.native.model)
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
	native.handle = 0
	gObjectUnref(unsafe.Pointer(native.model))
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
	native.clear()
	m.build(native, native.model, items)
	native.items = items
	native.definitions = make(map[string]ui.Action)
	var collect func([]ui.MenuItem)
	collect = func(items []ui.MenuItem) {
		for _, item := range items {
			if item.ActionID != "" {
				action, ok := m.lookup(item.ActionID)
				if !ok {
					action = ui.Action{ID: item.ActionID, Text: item.ActionID}
				}
				native.definitions[item.ActionID] = action
			}
			collect(item.Items)
		}
	}
	collect(items)
	for _, changed := range native.changes.snapshot() {
		changed()
	}
}

func (m *menu) build(root *nativeMenu, model *gMenu, items []ui.MenuItem) {
	var section *gMenu
	newSection := func() {
		section = gMenuNew()
		gMenuAppendSection(model, nil, (*gMenuModel)(unsafe.Pointer(section)))
		gObjectUnref(unsafe.Pointer(section))
	}
	for _, item := range items {
		if item.Separator {
			newSection()
			continue
		}
		if section == nil {
			newSection()
		}
		if item.ActionID != "" {
			action, ok := m.lookup(item.ActionID)
			if !ok {
				action = ui.Action{ID: item.ActionID, Text: item.ActionID}
			}
			root.appendAction(section, action)
		} else {
			submenu := gMenuNew()
			gMenuAppendSubmenu(section, stringPointer(item.Title), (*gMenuModel)(unsafe.Pointer(submenu)))
			m.build(root, submenu, item.Items)
			gObjectUnref(unsafe.Pointer(submenu))
		}
	}
}

func (m *menu) setActionState(id string, enabled, checked int) {
	m.mu.RLock()
	native := m.native
	m.mu.RUnlock()
	if native != nil {
		native.setActionState(id, enabled, checked)
		for _, changed := range native.changes.snapshot() {
			changed()
		}
	}
}

// contextMenu is a menu that a widget shows for a secondary click.
type contextMenu struct {
	*menu
}

func (c *contextMenu) Popup(widget ui.Widget, position ui.Point) error {
	owner, ok := widget.(nativeWidget)
	if !ok {
		return errors.New("ui/gtk: widget belongs to another backend")
	}
	view := owner.base().nativePointer()
	native := c.nativeMenu()
	if view == nil || native == nil {
		return ui.ErrClosed
	}
	menuRecord(native).popup((*gtkWidget)(view), position)
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

// ownedMenu returns the GTK menu behind a ui.Menu.
func ownedMenu(value ui.Menu) (*menu, error) {
	switch owned := value.(type) {
	case *menu:
		return owned, nil
	case *contextMenu:
		return owned.menu, nil
	default:
		return nil, fmt.Errorf("ui/gtk: menu %T belongs to another backend", value)
	}
}

var _ ui.Menu = (*menu)(nil)
var _ ui.ContextMenu = (*contextMenu)(nil)
