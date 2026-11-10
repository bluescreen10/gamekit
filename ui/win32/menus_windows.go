package win32

import (
	"math"
	"slices"
	"unsafe"
)

// A menu is a model of items, separators, and submenus. The backend builds a
// native menu from it for each place that shows it: a context menu, a pop-up,
// or the menu bar of a window. The root menu keeps the state of each action,
// which the items of every place share.

type menuEntryType int

const (
	menuItem menuEntryType = iota
	menuSeparator
	menuSubmenu
)

type menuEntry struct {
	typ       menuEntryType
	id        string
	title     string
	key       string
	modifiers int
	tooltip   string
	icon      *iconImage
	submenu   *nativeMenu
}

type actionState struct {
	enabled   bool
	checkable bool
	checked   bool
}

type nativeMenu struct {
	receiver eventReceiver
	root     *nativeMenu
	title    string
	entries  []menuEntry
	actions  map[string]*actionState
	inBar    *nativeMenuBar
}

func (m *nativeMenu) rootMenu() *nativeMenu {
	if m != nil && m.root != nil {
		return m.root
	}
	return m
}

// action returns the state of an action of a root menu, creating it.
func (m *nativeMenu) action(id string) *actionState {
	state := m.actions[id]
	if state == nil {
		state = &actionState{enabled: true}
		m.actions[id] = state
	}
	return state
}

// shortcutText returns the text of a shortcut in a menu, such as
// "Ctrl+Shift+K".
func menuShortcutText(key string, modifiers int) string {
	if key == "" {
		return ""
	}
	var text string
	if modifiers&1 != 0 {
		text += "Ctrl+"
	}
	if modifiers&2 != 0 {
		text += "Alt+"
	}
	if modifiers&4 != 0 {
		text += "Shift+"
	}
	switch key {
	case "Return":
		return text + "Enter"
	case "Escape":
		return text + "Esc"
	case "Delete":
		return text + "Del"
	default:
		return text + key
	}
}

// menuCommand is the action that a command of a native menu performs.
type menuCommandEntry struct {
	root *nativeMenu
	id   string
}

// build appends the entries of a menu to a native menu. The command of an item
// is first plus its index in commands.
func (m *nativeMenu) build(target hmenu, commands *[]menuCommandEntry, first uint32, bitmaps *[]hbitmap) {
	root := m.rootMenu()
	size := int(math.Round(16 * float64(win32GetDpiForSystem()) / 96))
	for _, entry := range m.entries {
		switch entry.typ {
		case menuSeparator:
			win32AppendMenu(target, mfSeparator, 0, "")
		case menuSubmenu:
			sub := win32CreatePopupMenu()
			entry.submenu.build(sub, commands, first, bitmaps)
			win32AppendMenu(target, mfPopup, uintptr(sub), entry.title)
		default:
			state := root.action(entry.id)
			*commands = append(*commands, menuCommandEntry{root, entry.id})
			text := entry.title
			if shortcut := menuShortcutText(entry.key, entry.modifiers); shortcut != "" {
				text += "\t" + shortcut
			}
			info := menuItemInfo{
				fMask:      miimID | miimString | miimState | miimFType,
				fType:      mftString,
				wID:        first + uint32(len(*commands)) - 1,
				dwTypeData: wide(text),
				fState:     mfsEnabled,
			}
			info.cbSize = uint32(unsafe.Sizeof(info))
			if !state.enabled {
				info.fState = mfsDisabled
			}
			if state.checkable && state.checked {
				info.fState |= mfsChecked
			}
			if entry.icon != nil {
				if bitmap := iconBitmap(entry.icon, size, paletteFor(resolvedTheme()).text); bitmap != 0 {
					info.fMask |= miimBitmap
					info.hbmpItem = bitmap
					*bitmaps = append(*bitmaps, bitmap)
				}
			}
			win32InsertMenuItem(target, uint32(win32GetMenuItemCount(target)), true, &info)
		}
	}
}

// menuInvoke performs an action of a menu as a click on its item does.
func menuInvoke(menu *nativeMenu, id string) {
	root := menu.rootMenu()
	if root == nil {
		return
	}
	state := root.action(id)
	if !state.enabled {
		return
	}
	if state.checkable {
		state.checked = !state.checked
	}
	send(root.receiver, actionEvent{id: id})
}

// menuNative builds a pop-up menu of a menu. Command n performs commands[n-1].
func menuNative(menu *nativeMenu) (hmenu, []menuCommandEntry, []hbitmap) {
	popup := win32CreatePopupMenu()
	var commands []menuCommandEntry
	var bitmaps []hbitmap
	menu.build(popup, &commands, 1, &bitmaps)
	return popup, commands, bitmaps
}

// menuPopupAt shows a menu at a point on the screen.
func menuPopupAt(menu *nativeMenu, owner hwnd, screen point) {
	if menu == nil {
		return
	}
	popup, commands, bitmaps := menuNative(menu)
	top := owner
	if owner != 0 {
		top = win32GetAncestor(owner, gaRoot)
	}
	command := uint32(win32TrackPopupMenuEx(popup, tpmReturnCmd|tpmRightButton|tpmNoNotify, screen.x, screen.y, top,
		nil))
	win32DestroyMenu(popup)
	for _, bitmap := range bitmaps {
		win32DeleteObject(hgdiobj(bitmap))
	}
	if command >= 1 && int(command) <= len(commands) {
		entry := commands[command-1]
		menuInvoke(entry.root, entry.id)
	}
}

func (m *nativeMenu) changed() {
	if root := m.rootMenu(); root != nil && root.inBar != nil {
		root.inBar.rebuild()
	}
}

func newNativeMenu(receiver eventReceiver, title string) *nativeMenu {
	return &nativeMenu{receiver: receiver, title: title, actions: map[string]*actionState{}}
}

func (menu *nativeMenu) setReceiver(receiver eventReceiver) {
	if root := menu.rootMenu(); root != nil {
		root.receiver = receiver
	}
}

func (menu *nativeMenu) clear() {
	if menu == nil {
		return
	}
	menu.entries = nil
	if menu.root == nil {
		// A root menu drops its actions; submenus are rebuilt with it.
		menu.actions = map[string]*actionState{}
	}
	menu.changed()
}

func (menu *nativeMenu) addItem(actionID string, title string, enabled bool, checked bool, key string, modifiers int, icon *iconImage, tooltip string) {
	if menu == nil {
		return
	}
	entry := menuEntry{id: actionID, title: title, key: key, modifiers: modifiers, tooltip: tooltip}
	state := menu.rootMenu().action(actionID)
	state.enabled = enabled
	if checked {
		state.checkable = true
		state.checked = true
	}
	menu.entries = append(menu.entries, entry)
	menu.changed()
}

func (menu *nativeMenu) addSeparator() {
	if menu != nil {
		menu.entries = append(menu.entries, menuEntry{typ: menuSeparator})
		menu.changed()
	}
}

// addSubmenu adds a submenu, which the parent menu owns.
func (menu *nativeMenu) addSubmenu(title string) *nativeMenu {
	if menu == nil {
		return nil
	}
	submenu := &nativeMenu{root: menu.rootMenu(), title: title}
	menu.entries = append(menu.entries, menuEntry{typ: menuSubmenu, title: title, submenu: submenu})
	menu.changed()
	return submenu
}

// setActionState sets the state of an action; -1 keeps a value.
func (menu *nativeMenu) setActionState(actionID string, enabled int, checked int) {
	root := menu.rootMenu()
	if root == nil {
		return
	}
	state := root.actions[actionID]
	if state == nil {
		return
	}
	if enabled >= 0 {
		state.enabled = enabled != 0
	}
	if checked > 0 {
		state.checkable = true
	}
	if checked >= 0 && state.checkable {
		state.checked = checked != 0
	}
	root.changed()
}

func (menu *nativeMenu) popup(view *nativeView, x float64, y float64) {
	if !view.live() || view.outer == 0 {
		return
	}
	scale := view.scale()
	at := point{x: int32(math.Round(x * scale)), y: int32(math.Round(y * scale))}
	win32ClientToScreen(view.outer, &at)
	menuPopupAt(menu, view.outer, at)
}

// ---- Menu bar ----

const menuBarFirst = 0x1000

type nativeMenuBar struct {
	receiver  eventReceiver
	win       *nativeWindow
	bar       hmenu
	installed bool
	menus     []barMenu
	// commands holds the action of command n at n - menuBarFirst.
	commands []menuCommandEntry
	bitmaps  []hbitmap
}

func (b *nativeMenuBar) destroy() {
	menuBars = slices.DeleteFunc(menuBars, func(other *nativeMenuBar) bool { return other == b })
	if b.installed && b.win != nil && !b.win.destroyed && b.win.menubar == b.bar {
		setMenuBar(b.win, 0)
	}
	for _, item := range b.menus {
		item.menu.inBar = nil
	}
	if b.bar != 0 {
		win32DestroyMenu(b.bar)
	}
	for _, bitmap := range b.bitmaps {
		win32DeleteObject(hgdiobj(bitmap))
	}
}

var menuBars []*nativeMenuBar

// setMenuBar shows a menu on a window, or none, and keeps the size of the
// content of the window.
func setMenuBar(win *nativeWindow, menu hmenu) {
	width, height := win.size()
	win.menubar = menu
	if !win.fullscreen {
		win32SetMenu(win.hwnd, menu)
		if !win32IsZoomed(win.hwnd) && width > 0 && height > 0 {
			win.setSize(width, height)
		}
	}
	win32DrawMenuBar(win.hwnd)
}

func (b *nativeMenuBar) rebuild() {
	fresh := win32CreateMenu()
	var commands []menuCommandEntry
	var bitmaps []hbitmap
	for _, item := range b.menus {
		popup := win32CreatePopupMenu()
		item.menu.build(popup, &commands, menuBarFirst, &bitmaps)
		win32AppendMenu(fresh, mfPopup, uintptr(popup), item.title)
	}
	old := b.bar
	b.bar = fresh
	b.commands = commands
	for _, bitmap := range b.bitmaps {
		win32DeleteObject(hgdiobj(bitmap))
	}
	b.bitmaps = bitmaps
	if b.installed && b.win != nil && !b.win.destroyed {
		b.win.menubar = fresh
		if !b.win.fullscreen {
			win32SetMenu(b.win.hwnd, fresh)
		}
		win32DrawMenuBar(b.win.hwnd)
	}
	if old != 0 {
		win32DestroyMenu(old)
	}
}

// barMenu is a menu of a menu bar, with its title.
type barMenu struct {
	title string
	menu  *nativeMenu
}

// newMenuBar creates the menu bar of a window, or returns nil when a menu is
// in another menu bar.
func (win *nativeWindow) newMenuBar(menus []barMenu) *nativeMenuBar {
	if win == nil {
		return nil
	}
	for _, item := range menus {
		if item.menu == nil || item.menu.inBar != nil {
			return nil
		}
	}
	bar := &nativeMenuBar{win: win, menus: menus}
	for _, item := range menus {
		item.menu.inBar = bar
	}
	bar.rebuild()
	menuBars = append(menuBars, bar)
	return bar
}

func (bar *nativeMenuBar) install() {
	if bar == nil || bar.win == nil {
		return
	}
	bar.installed = true
	setMenuBar(bar.win, bar.bar)
}

// menuCommand performs the item of a menu bar that a WM_COMMAND names.
func menuCommand(h hwnd, id int) bool {
	for _, bar := range menuBars {
		if !bar.installed || bar.win == nil || bar.win.hwnd != h {
			continue
		}
		index := id - menuBarFirst
		if index >= 0 && index < len(bar.commands) {
			entry := bar.commands[index]
			menuInvoke(entry.root, entry.id)
			bar.rebuild()
			return true
		}
	}
	return false
}

// shortcut finds the item of a menu whose shortcut is a key.
func (m *nativeMenu) shortcut(key uint32, modifiers int) (*nativeMenu, string, bool) {
	for _, entry := range m.entries {
		switch {
		case entry.typ == menuSubmenu:
			if owner, id, ok := entry.submenu.shortcut(key, modifiers); ok {
				return owner, id, true
			}
		case entry.typ == menuItem && entry.key != "" && virtualKey(entry.key) == key && entry.modifiers == modifiers:
			return m.rootMenu(), entry.id, true
		}
	}
	return nil, "", false
}

// menuAccelerator performs the item of the menu bar of a window whose shortcut
// is the key that a message presses.
func menuAccelerator(win *nativeWindow, message *msg) bool {
	if message.message != wmKeyDown && message.message != wmSysKeyDown {
		return false
	}
	modifiers := acceleratorModifiers()
	if modifiers&3 == 0 && !(message.wParam >= vkF1 && message.wParam <= vkF24) {
		// A shortcut without Control or Alt is typing, except for function
		// keys.
		return false
	}
	for _, bar := range menuBars {
		if !bar.installed || bar.win != win {
			continue
		}
		for _, item := range bar.menus {
			if owner, id, ok := item.menu.shortcut(uint32(message.wParam), modifiers); ok {
				menuInvoke(owner, id)
				bar.rebuild()
				return true
			}
		}
	}
	return false
}
