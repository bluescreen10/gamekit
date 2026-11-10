//go:build windows

package win32

import (
	"errors"
	"fmt"
	"sync"

	"github.com/bluescreen10/gamekit/ui"
)

// systemTray is an icon in the notification area of the Windows taskbar.
type systemTray struct {
	mu        sync.RWMutex
	native    *nativeTray
	icon      ui.IconSource
	title     string
	tooltip   string
	menu      ui.Menu
	visible   bool
	attention bool
	activates listeners[ui.FocusHandler]
}

// CreateSystemTray adds a status item to the menu bar.
func (w *Window) CreateSystemTray(options ui.SystemTrayOptions) (ui.SystemTray, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if w.nativeWindow() == nil {
		return nil, ui.ErrClosed
	}
	tray := &systemTray{
		icon:    options.Icon,
		title:   options.Title,
		tooltip: options.Tooltip,
		visible: true,
	}
	tray.native = newTray(tray)
	if tray.native == nil {
		return nil, errors.New("ui/win32: the menu bar could not create a status item")
	}
	tray.apply()
	if options.Menu != nil {
		if err := tray.SetMenu(options.Menu); err != nil {
			tray.Close()
			return nil, err
		}
	}
	return tray, nil
}

func (t *systemTray) receiveEvent(event any) {
	if _, ok := event.(activateEvent); ok {
		each(&t.activates, func(handler ui.FocusHandler) {
			handler()
		})
	}
}

// apply sends the icon, title, and tooltip to the status item. Symbols become
// template images so they follow the menu bar's appearance.
func (t *systemTray) apply() {
	t.mu.RLock()
	native, icon, title, tooltip := t.native, t.icon, t.title, t.tooltip
	t.mu.RUnlock()
	if native == nil {
		return
	}
	image := newIcon(icon, 15)
	native.configure(image, icon.SystemName != "", title, tooltip)
}

func (t *systemTray) nativeTray() *nativeTray {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.native
}

func (t *systemTray) Close() {
	t.mu.Lock()
	native := t.native
	t.native = nil
	t.mu.Unlock()
	if native == nil {
		return
	}
	native.destroy()
}

func (t *systemTray) Icon() ui.IconSource {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.icon
}

func (t *systemTray) SetIcon(icon ui.IconSource) error {
	if t.nativeTray() == nil {
		return ui.ErrClosed
	}
	t.mu.Lock()
	if icon.IsZero() && t.title == "" {
		t.mu.Unlock()
		return errors.New("ui: system tray item requires an icon or title")
	}
	t.icon = icon
	t.mu.Unlock()
	t.apply()
	return nil
}

func (t *systemTray) Title() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.title
}

// SetTitle changes the text beside the icon. An item without an icon keeps
// its previous title rather than becoming empty.
func (t *systemTray) SetTitle(title string) {
	t.mu.Lock()
	if title == "" && t.icon.IsZero() {
		t.mu.Unlock()
		return
	}
	t.title = title
	t.mu.Unlock()
	t.apply()
}

func (t *systemTray) Tooltip() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tooltip
}

func (t *systemTray) SetTooltip(tooltip string) {
	t.mu.Lock()
	t.tooltip = tooltip
	t.mu.Unlock()
	t.apply()
}

func (t *systemTray) Menu() ui.Menu {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.menu
}

func (t *systemTray) SetMenu(menu ui.Menu) error {
	native := t.nativeTray()
	if native == nil {
		return ui.ErrClosed
	}
	var nativeMenu *nativeMenu
	if menu != nil {
		owned, err := ownedMenu(menu)
		if err != nil {
			return err
		}
		if nativeMenu = owned.nativeMenu(); nativeMenu == nil {
			return ui.ErrClosed
		}
	}
	native.setMenu(nativeMenu)
	t.mu.Lock()
	t.menu = menu
	t.mu.Unlock()
	return nil
}

func (t *systemTray) IsVisible() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.visible && t.native != nil
}

func (t *systemTray) SetVisible(visible bool) {
	native := t.nativeTray()
	if native == nil {
		return
	}
	native.setVisible(visible)
	t.mu.Lock()
	t.visible = visible
	t.mu.Unlock()
}

func (t *systemTray) IsAttention() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.attention
}

func (t *systemTray) SetAttention(attention bool) {
	native := t.nativeTray()
	if native == nil {
		return
	}
	native.setAttention(attention)
	t.mu.Lock()
	t.attention = attention
	t.mu.Unlock()
}

func (t *systemTray) Bounds() (ui.Rect, error) {
	native := t.nativeTray()
	if native == nil {
		return ui.Rect{}, ui.ErrClosed
	}
	x, y, width, height, ok := native.bounds()
	if !ok {
		return ui.Rect{}, fmt.Errorf("ui/win32: the item is not shown in the notification area: %w",
			ui.ErrNotSupported)
	}
	return ui.Rect{
		Position: ui.Point{X: int(x), Y: int(y)},
		Size:     ui.Size{Width: int(width), Height: int(height)},
	}, nil
}

func (t *systemTray) OnActivate(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&t.activates, handler, handler == nil)
}

var _ ui.SystemTray = (*systemTray)(nil)
