package main

import (
	"context"
	"errors"

	"github.com/bluescreen10/gamekit/ui"
)

// themes lists the themes that the gallery offers, with their menu action IDs.
var themes = []struct {
	id    string
	name  string
	theme ui.Theme
}{
	{"theme-system", "System", ui.ThemeSystem},
	{"theme-light", "Light", ui.ThemeLight},
	{"theme-dark", "Dark", ui.ThemeDark},
}

func commandCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Menu bar", "The gallery installs a menu bar with File, View, and Help menus. These controls change its actions.", g.menuBarDemo),
		g.card("Toolbar", "Install a window toolbar, with or without labels.", g.toolbarDemo),
		g.card("Command bars", "A bar of action buttons, in the full and compact presentations.", g.commandBarsDemo),
		g.card("Context menu", "Right-click the area, or open the menu from the button.", g.contextMenuDemo),
		g.card("Title bar accessory", "Put a widget in the title bar of the window.", g.titleBarAccessoryDemo),
		g.card("Status bar", "The bar at the bottom of this window shows the last event.", g.statusBarDemo),
	}
}

// installMenuBar creates the menu bar of the gallery window.
func (g *gallery) installMenuBar() {
	g.attempt("Menu bar", func() error {
		menuBar, err := g.createMenuBar()
		if err != nil {
			return err
		}
		g.menuBar = menuBar
		return nil
	})
}

func (g *gallery) createMenuBar() (ui.MenuBar, error) {
	actions := []ui.Action{
		{ID: "new", Text: "New Window", Shortcut: ui.Shortcut{Key: "N", Command: true}, Enabled: true},
		{ID: "open", Text: "Open…", Shortcut: ui.Shortcut{Key: "O", Command: true}, Enabled: true},
		{ID: "save", Text: "Save", Shortcut: ui.Shortcut{Key: "S", Command: true}, Enabled: true, Tooltip: "Save the document"},
		{ID: "close", Text: "Close Window", Shortcut: ui.Shortcut{Key: "W", Command: true}, Enabled: true},
		{ID: "status", Text: "Show Status Bar", Checked: true, Enabled: true},
		{ID: "event-log", Text: "Event Log", Shortcut: ui.Shortcut{Key: "L", Command: true, Shift: true}, Enabled: true},
		{ID: "about", Text: "About the Gallery", Enabled: true},
	}
	for _, entry := range themes {
		actions = append(actions, ui.Action{ID: entry.id, Text: entry.name, Enabled: true, Checked: entry.theme == g.window.Theme()})
	}

	file, err := g.window.CreateMenu(ui.MenuOptions{Actions: actions, Items: []ui.MenuItem{
		{ActionID: "new"}, {ActionID: "open"}, {ActionID: "save"}, {Separator: true}, {ActionID: "close"},
	}})
	if err != nil {
		return nil, err
	}
	view, err := g.window.CreateMenu(ui.MenuOptions{Actions: actions, Items: []ui.MenuItem{
		{Title: "Theme", Items: []ui.MenuItem{{ActionID: "theme-system"}, {ActionID: "theme-light"}, {ActionID: "theme-dark"}}},
		{Separator: true},
		{ActionID: "status"},
		{ActionID: "event-log"},
	}})
	if err != nil {
		return nil, err
	}
	help, err := g.window.CreateMenu(ui.MenuOptions{Actions: actions, Items: []ui.MenuItem{{ActionID: "about"}}})
	if err != nil {
		return nil, err
	}
	menuBar, err := g.window.CreateMenuBar(ui.MenuBarOptions{
		Actions: actions,
		Menus: []ui.NamedMenu{
			{Title: "File", Menu: file},
			{Title: "View", Menu: view},
			{Title: "Help", Menu: help},
		},
	})
	if err != nil {
		return nil, err
	}
	statusShown := true
	menuBar.OnAction(func(id string) {
		g.attempt("Menu "+id, func() error {
			if id == "status" {
				statusShown = !statusShown
				g.statusBar.SetVisible(statusShown)
				return menuBar.SetActionChecked("status", statusShown)
			}
			return g.runMenuAction(id)
		})
	})
	return menuBar, nil
}

// runMenuAction runs a command of the gallery menu bar.
func (g *gallery) runMenuAction(id string) error {
	g.log("Menu action: %s", id)
	for _, entry := range themes {
		if entry.id == id {
			return g.setTheme(entry.theme)
		}
	}
	switch id {
	case "new":
		return g.openSecondWindow()
	case "open":
		path, err := g.window.OpenFile(context.Background(), ui.OpenFileOptions{Title: "Open"})
		if errors.Is(err, ui.ErrCancelled) {
			g.log("Open cancelled")
			return nil
		}
		if err != nil {
			return err
		}
		g.log("Opened %s", path)
	case "save":
		g.log("Saved (simulated)")
	case "close":
		g.window.SetShouldClose(true)
	case "event-log":
		g.showPage("events")
	case "about":
		return g.showAbout()
	}
	return nil
}

// setTheme changes the theme of the gallery window and checks its menu item.
func (g *gallery) setTheme(theme ui.Theme) error {
	if err := g.window.SetTheme(theme); err != nil {
		return err
	}
	if g.menuBar != nil {
		for _, entry := range themes {
			if err := g.menuBar.SetActionChecked(entry.id, entry.theme == theme); err != nil {
				return err
			}
		}
	}
	g.log("Theme: %d", theme)
	return nil
}

func (g *gallery) menuBarDemo() ui.Widget {
	if g.menuBar == nil {
		return g.unavailable("the menu bar could not be created")
	}
	save := g.checkBox("Save enabled", true, func(checked bool) {
		if err := g.menuBar.SetActionEnabled("save", checked); err != nil {
			g.log("Menu: %v", err)
		}
	})
	dark := g.button("Check Dark theme", func() {
		g.attempt("Theme", func() error {
			return g.setTheme(ui.ThemeDark)
		})
	})
	system := g.button("Check System theme", func() {
		g.attempt("Theme", func() error {
			return g.setTheme(ui.ThemeSystem)
		})
	})
	return g.column(
		g.caption("Open the View menu to see the checked theme. ⌘S or Ctrl+S runs Save when it is enabled."),
		g.row(save, dark, system),
	)
}

func (g *gallery) toolbarDemo() ui.Widget {
	var toolbar ui.Toolbar
	labels := false
	install := func() {
		if toolbar != nil {
			toolbar.Close()
			toolbar = nil
		}
		next, err := g.window.CreateToolbar(ui.ToolbarOptions{
			Actions: []ui.Action{
				{ID: "back", Text: "Back", Icon: ui.IconSource{SystemName: "arrow-left"}, Tooltip: "Go back", Enabled: true},
				{ID: "forward", Text: "Forward", Icon: ui.IconSource{SystemName: "arrow-right"}, Tooltip: "Go forward", Enabled: false},
				{ID: "refresh", Text: "Refresh", Icon: ui.IconSource{SystemName: "refresh"}, Enabled: true},
				{ID: "share", Text: "Share", Icon: ui.IconSource{SystemName: "share"}, Enabled: true},
				{ID: "settings", Text: "Settings", Icon: ui.IconSource{SystemName: "gear"}, Enabled: true},
			},
			Items: []ui.ToolbarItem{
				{ActionID: "back"}, {ActionID: "forward"}, {Separator: true}, {ActionID: "refresh"},
				{FlexibleSpace: true}, {ActionID: "share"}, {Space: true}, {ActionID: "settings"},
			},
			ShowsLabels: labels,
		})
		if err != nil {
			g.log("Toolbar: %v", err)
			return
		}
		next.OnAction(func(id string) {
			g.log("Toolbar action: %s", id)
		})
		toolbar = next
		g.log("Toolbar installed")
	}
	installButton := g.button("Install toolbar", install)
	remove := g.button("Remove toolbar", func() {
		if toolbar != nil {
			toolbar.Close()
			toolbar = nil
			g.log("Toolbar removed")
		}
	})
	showLabels := g.checkBox("Show labels", false, func(checked bool) {
		labels = checked
		if toolbar != nil {
			install()
		}
	})
	forward := g.checkBox("Forward enabled", false, func(checked bool) {
		if toolbar == nil {
			return
		}
		if err := toolbar.SetActionEnabled("forward", checked); err != nil {
			g.log("Toolbar: %v", err)
		}
	})
	return g.row(installButton, remove, showLabels, forward)
}

func (g *gallery) commandBarsDemo() ui.Widget {
	actions := []ui.Action{
		{ID: "add", Text: "Add", Icon: ui.IconSource{SystemName: "add"}, Enabled: true},
		{ID: "edit", Text: "Edit", Icon: ui.IconSource{SystemName: "edit"}, Enabled: true},
		{ID: "copy", Text: "Copy", Icon: ui.IconSource{SystemName: "copy"}, Enabled: true},
		{ID: "delete", Text: "Delete", Icon: ui.IconSource{SystemName: "delete"}, Enabled: false, Tooltip: "Select an item first"},
		{ID: "pin", Text: "Pin", Icon: ui.IconSource{SystemName: "star"}, Enabled: true, Checked: true},
	}
	full := must(g.window.CreateCommandBar(ui.CommandBarOptions{Actions: actions}))
	compact := must(g.window.CreateCommandBar(ui.CommandBarOptions{Actions: actions, Compact: true}))
	for _, bar := range []ui.CommandBar{full, compact} {
		bar.OnAction(func(id string) {
			g.log("Command bar action: %s", id)
		})
	}
	enableDelete := g.checkBox("Delete enabled", false, func(checked bool) {
		for _, bar := range []ui.CommandBar{full, compact} {
			updated := bar.Actions()
			for i := range updated {
				if updated[i].ID == "delete" {
					updated[i].Enabled = checked
				}
			}
			if err := bar.SetActions(updated); err != nil {
				g.log("Command bar: %v", err)
			}
		}
	})
	return g.column(g.labeled("Full", full), g.labeled("Compact", compact), g.row(enableDelete))
}

func (g *gallery) contextMenuDemo() ui.Widget {
	menu := must(g.window.CreateContextMenu(ui.ContextMenuOptions{
		Actions: []ui.Action{
			{ID: "cut", Text: "Cut", Shortcut: ui.Shortcut{Key: "X", Command: true}, Enabled: true},
			{ID: "copy", Text: "Copy", Shortcut: ui.Shortcut{Key: "C", Command: true}, Enabled: true},
			{ID: "paste", Text: "Paste", Shortcut: ui.Shortcut{Key: "V", Command: true}, Enabled: false},
			{ID: "grid", Text: "Show Grid", Checked: true, Enabled: true},
			{ID: "front", Text: "Bring to Front", Enabled: true},
			{ID: "back", Text: "Send to Back", Enabled: true},
		},
		Items: []ui.MenuItem{
			{ActionID: "cut"}, {ActionID: "copy"}, {ActionID: "paste"}, {Separator: true},
			{ActionID: "grid"},
			{Title: "Arrange", Items: []ui.MenuItem{{ActionID: "front"}, {ActionID: "back"}}},
		},
	}))
	menu.OnAction(func(id string) {
		g.log("Context menu action: %s", id)
	})
	area := g.label("Right-click this area")
	area.SetTextAlignment(ui.TextAlignCenter)
	area.SetMinimumSize(ui.Size{Height: 70})
	g.attempt("Area style", func() error {
		return area.SetStyle(ui.WidgetStyle{Background: ui.SemanticColorSurface, CornerRadius: 6})
	})
	if err := area.SetContextMenu(menu); err != nil {
		panic(err)
	}
	var open ui.Button
	open = g.button("Open menu", func() {
		if err := menu.Popup(open, ui.Point{Y: open.Bounds().Size.Height}); err != nil {
			g.log("Context menu: %v", err)
		}
	})
	return g.column(area, g.row(open))
}

func (g *gallery) titleBarAccessoryDemo() ui.Widget {
	var accessory ui.TitleBarAccessory
	alignment := ui.AlignEnd
	install := g.button("Add accessory", func() {
		if accessory != nil {
			return
		}
		content := g.layout(ui.Row(
			ui.Item(g.button("Sync", func() {
				g.log("Title bar button clicked")
			})).Align(ui.AlignCenter),
		).WithPadding(ui.Insets{Left: 6, Right: 6}))
		next, err := g.window.CreateTitleBarAccessory(ui.TitleBarAccessoryOptions{Content: content, Alignment: alignment})
		if err != nil {
			content.Close()
			g.log("Title bar accessory: %v", err)
			return
		}
		accessory = next
		g.log("Title bar accessory added")
	})
	remove := g.button("Remove accessory", func() {
		if accessory != nil {
			accessory.Close()
			accessory = nil
			g.log("Title bar accessory removed")
		}
	})
	side := g.choice([]string{"Trailing", "Leading"}, 0, func(index int) {
		alignment = []ui.Alignment{ui.AlignEnd, ui.AlignStart}[index]
		g.log("The alignment applies to the next accessory")
	})
	return g.row(install, remove, side)
}

func (g *gallery) statusBarDemo() ui.Widget {
	text := must(g.window.CreateTextField())
	text.SetPlaceholder("Status text")
	text.OnSubmit(func(value string) {
		g.statusBar.SetText(value)
	})
	set := g.button("Set status", func() {
		g.statusBar.SetText(text.Text())
	})
	return g.labeled("Text", fixed(text, ui.Size{Width: 260}), set)
}
