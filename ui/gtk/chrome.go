//go:build linux && cgo

package gtk

import (
	"errors"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Menu bar ----

// menuBar is a GtkPopoverMenuBar at the top of its window. Each GTK window
// shows its own menu bar.
type menuBar struct {
	window        *Window
	native        *nativeMenuBar
	menus         []ui.NamedMenu
	owned         []*menu
	actions       []ui.Action
	unsubscribers []ui.Unsubscribe
	listeners     listeners[ui.ActionHandler]
}

// CreateMenuBar implements ui.Window.CreateMenuBar. The bar shows above the
// content of the window, and the shortcuts of its actions work while the
// window has keyboard focus.
func (w *Window) CreateMenuBar(options ui.MenuBarOptions) (ui.MenuBar, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}
	result := &menuBar{window: w, menus: append([]ui.NamedMenu(nil), options.Menus...), actions: append([]ui.Action(nil), options.Actions...)}
	for _, named := range options.Menus {
		owned, err := ownedMenu(named.Menu)
		if err != nil {
			return nil, err
		}
		result.owned = append(result.owned, owned)
	}
	var err error
	result.native, err = newNativeMenuBar(w, options.Menus)
	if err != nil {
		return nil, err
	}
	for _, owned := range result.owned {
		owned.inherit(options.Actions)
		result.unsubscribers = append(result.unsubscribers, owned.OnAction(func(id string) {
			each(&result.listeners, func(handler ui.ActionHandler) {
				handler(id)
			})
		}))
	}
	w.mu.Lock()
	w.menuBar = result
	w.mu.Unlock()
	w.adoptResource(result)
	result.install()
	return result, nil
}

// install shows the bar in its window.
func (m *menuBar) install() {
	if m.native != nil {
		gtkWidgetSetVisible(m.native.widget, 1)
	}
}

func (m *menuBar) Close() {
	if m.native == nil {
		return
	}
	native := m.native
	m.native = nil
	for _, unsubscribe := range m.unsubscribers {
		unsubscribe()
	}
	for _, owned := range m.owned {
		owned.inherit(nil)
	}
	m.window.mu.Lock()
	if m.window.menuBar == m {
		m.window.menuBar = nil
	}
	m.window.mu.Unlock()
	m.window.releaseResource(m)
	native.close()
}

func (m *menuBar) Menus() []ui.NamedMenu {
	return append([]ui.NamedMenu(nil), m.menus...)
}

func (m *menuBar) setAction(id string, update func(*ui.Action)) error {
	for i := range m.actions {
		if m.actions[i].ID == id {
			update(&m.actions[i])
			for _, owned := range m.owned {
				owned.inherit(m.actions)
			}
			return nil
		}
	}
	return errors.New("ui: menu bar action does not exist")
}

func (m *menuBar) SetActionEnabled(id string, enabled bool) error {
	return m.setAction(id, func(action *ui.Action) {
		action.Enabled = enabled
	})
}

func (m *menuBar) SetActionChecked(id string, checked bool) error {
	return m.setAction(id, func(action *ui.Action) {
		action.Checked = checked
	})
}

func (m *menuBar) OnAction(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&m.listeners, handler, handler == nil)
}

// ---- Toolbar ----

type toolbar struct {
	window    *Window
	native    *nativeToolbar
	items     []ui.ToolbarItem
	listeners listeners[ui.ActionHandler]
}

// CreateToolbar implements ui.Window.CreateToolbar.
func (w *Window) CreateToolbar(options ui.ToolbarOptions) (ui.Toolbar, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window := w.nativeWindowPointer()
	if window == nil {
		return nil, ui.ErrClosed
	}
	result := &toolbar{window: w, items: append([]ui.ToolbarItem(nil), options.Items...)}
	var err error
	result.native, err = newNativeToolbar(w, options, func(id string) {
		each(&result.listeners, func(handler ui.ActionHandler) { handler(id) })
	})
	if err != nil {
		return nil, err
	}
	w.adoptResource(result)
	return result, nil
}

func (t *toolbar) gtkEvent(event gtkEvent) {
	if event.kind == gkEventAction {
		id := event.text
		each(&t.listeners, func(handler ui.ActionHandler) {
			handler(id)
		})
	}
}

func (t *toolbar) Close() {
	if t.native == nil {
		return
	}
	native := t.native
	t.native = nil
	native.close()
	t.window.releaseResource(t)
}

func (t *toolbar) Items() []ui.ToolbarItem {
	return append([]ui.ToolbarItem(nil), t.items...)
}

func (t *toolbar) SetActionEnabled(id string, enabled bool) error {
	if t.native == nil {
		return ui.ErrClosed
	}
	for _, item := range t.items {
		if item.ActionID == id {
			if button := t.native.buttons[id]; button != nil {
				gtkWidgetSetSensitive(button, nativeBool(enabled))
			}
			return nil
		}
	}
	return errors.New("ui: toolbar action does not exist")
}

func (t *toolbar) OnAction(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&t.listeners, handler, handler == nil)
}

// ---- Status bar ----

type statusBar struct {
	*widgetBase
	text string
}

// CreateStatusBar implements ui.Window.CreateStatusBar.
func (w *Window) CreateStatusBar(options ui.StatusBarOptions) (ui.StatusBar, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	text := cString(options.Text)
	defer freeCString(text)
	result := &statusBar{text: options.Text}
	if result.widgetBase, err = w.newComponent(gkGTKStatusBarCreate(window, text), ui.RoleStatus, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *statusBar) gtkEvent(gtkEvent) {
	// A status bar sends no native events.
}
func (s *statusBar) Text() string {
	return s.text
}
func (s *statusBar) SetText(text string) {
	s.text = text
	if native := s.nativePointer(); native != nil {
		value := cString(text)
		defer freeCString(value)
		gkGTKStatusBarSet(native, value)
	}
	s.announceText(text)
}

// ---- Command bar ----

type commandBar struct {
	*widgetBase
	actions   []ui.Action
	compact   bool
	listeners listeners[ui.ActionHandler]
}

// CreateCommandBar implements ui.Window.CreateCommandBar.
func (w *Window) CreateCommandBar(options ui.CommandBarOptions) (ui.CommandBar, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &commandBar{compact: options.Compact}
	if result.widgetBase, err = w.newComponent(gkGTKCommandBarCreate(window), ui.RoleToolbar, result); err != nil {
		return nil, err
	}
	if err := result.SetActions(options.Actions); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (c *commandBar) Actions() []ui.Action {
	return append([]ui.Action(nil), c.actions...)
}

func (c *commandBar) SetActions(actions []ui.Action) error {
	if err := (ui.CommandBarOptions{Actions: actions}).Validate(); err != nil {
		return err
	}
	native := c.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	ids := make([]string, len(actions))
	titles := make([]string, len(actions))
	icons := make([]ui.IconSource, len(actions))
	enabled := make([]cInt, len(actions))
	for i, action := range actions {
		ids[i], titles[i], icons[i], enabled[i] = action.ID, action.Text, action.Icon, boolInt(action.Enabled)
	}
	array := newIconArray(icons, 0)
	defer array.free()
	idBuffer, titleBuffer := joinNul(ids), joinNul(titles)
	gkGTKCommandBarSetActions(native, bytesPointer(idBuffer), cSize(len(idBuffer)), bytesPointer(titleBuffer),
		cSize(len(titleBuffer)), array.native(), &enabled[0], cInt(len(actions)), boolInt(c.compact))
	c.actions = append([]ui.Action(nil), actions...)
	return nil
}

func (c *commandBar) OnAction(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&c.listeners, handler, handler == nil)
}

func (c *commandBar) gtkEvent(event gtkEvent) {
	if event.kind == gkEventAction {
		id := event.text
		each(&c.listeners, func(handler ui.ActionHandler) {
			handler(id)
		})
	}
}

var (
	_ ui.MenuBar    = (*menuBar)(nil)
	_ ui.Toolbar    = (*toolbar)(nil)
	_ ui.StatusBar  = (*statusBar)(nil)
	_ ui.CommandBar = (*commandBar)(nil)
)
