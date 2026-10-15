//go:build darwin && cgo

package cocoa

/*
#include "cocoa_bridge.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"sync"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// systemTray is an NSStatusItem in the menu bar.
type systemTray struct {
	mu        sync.RWMutex
	native    unsafe.Pointer
	handle    cgo.Handle
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
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}
	tray := &systemTray{
		icon:    options.Icon,
		title:   options.Title,
		tooltip: options.Tooltip,
		visible: true,
	}
	tray.handle = cgo.NewHandle(tray)
	tray.native = C.gkCocoaTrayCreate(C.uintptr_t(tray.handle))
	if tray.native == nil {
		tray.handle.Delete()
		return nil, errors.New("ui/cocoa: the menu bar could not create a status item")
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

func (t *systemTray) cocoaEvent(event cocoaEvent) {
	if event.kind == C.GK_EVENT_ACTIVATE {
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
	image := nativeIcon(icon, 15)
	defer C.gkCocoaRelease(image)
	titleValue := cString(title)
	tooltipValue := cString(tooltip)
	defer freeCString(titleValue)
	defer freeCString(tooltipValue)
	C.gkCocoaTrayConfigure(native, image, boolInt(icon.SystemName != ""), titleValue, tooltipValue)
}

func (t *systemTray) nativeTray() unsafe.Pointer {
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
	C.gkCocoaTrayRemove(native)
	t.handle.Delete()
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
	var nativeMenu unsafe.Pointer
	if menu != nil {
		owned, err := ownedMenu(menu)
		if err != nil {
			return err
		}
		if nativeMenu = owned.nativeMenu(); nativeMenu == nil {
			return ui.ErrClosed
		}
	}
	C.gkCocoaTraySetMenu(native, nativeMenu)
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
	C.gkCocoaTraySetVisible(native, boolInt(visible))
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
	C.gkCocoaTraySetAttention(native, boolInt(attention))
	t.mu.Lock()
	t.attention = attention
	t.mu.Unlock()
}

func (t *systemTray) Bounds() (ui.Rect, error) {
	native := t.nativeTray()
	if native == nil {
		return ui.Rect{}, ui.ErrClosed
	}
	var x, y, width, height C.double
	if C.gkCocoaTrayBounds(native, &x, &y, &width, &height) == 0 {
		return ui.Rect{}, errors.New("ui/cocoa: the status item is not shown in the menu bar")
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
