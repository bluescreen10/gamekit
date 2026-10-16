package main

import (
	"fmt"
	"strconv"

	"github.com/bluescreen10/gamekit/ui"
)

func buttonCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Push buttons", "Each role has its own native look. Return activates the default button and Escape the cancel button.", g.pushButtonsDemo),
		g.card("Icons", "System icons in every placement.", g.buttonIconsDemo),
		g.card("Busy, badge, and shortcut", "A busy button ignores clicks. The shortcut button responds to ⌘K or Ctrl+K.", g.buttonStatesDemo),
		g.card("Toggle buttons", "An independent toggle, and a group of mutually exclusive toggles in the toolbar style.", g.toggleButtonsDemo),
		g.card("Links", "A link that reports its activation, a link that opens the browser, and a link with an icon.", g.linksDemo),
	}
}

func (g *gallery) pushButtonsDemo() ui.Widget {
	roles := []struct {
		text string
		role ui.ButtonRole
	}{
		{"Normal", ui.ButtonNormal},
		{"Default", ui.ButtonDefault},
		{"Cancel", ui.ButtonCancel},
		{"Delete", ui.ButtonDestructive},
	}
	buttons := make([]ui.Widget, len(roles))
	for i, entry := range roles {
		button := g.button(entry.text, func() {
			g.log("Clicked %s button", entry.text)
		})
		g.attempt("Button role", func() error {
			button.SetRole(entry.role)
			return nil
		})
		buttons[i] = button
	}
	return g.column(g.row(buttons...), g.states(buttons...))
}

func (g *gallery) buttonIconsDemo() ui.Widget {
	placements := []struct {
		text      string
		icon      string
		placement ui.IconPlacement
	}{
		{"Before", "star", ui.IconBeforeText},
		{"After", "arrow-right", ui.IconAfterText},
		{"Above", "download", ui.IconAboveText},
		{"Below", "upload", ui.IconBelowText},
		{"Settings", "gear", ui.IconOnly},
	}
	buttons := make([]ui.Widget, len(placements))
	for i, entry := range placements {
		button := g.button(entry.text, func() {
			g.log("Clicked icon button: %s", entry.text)
		})
		g.attempt("Button icon", func() error {
			button.SetIcon(ui.IconSource{SystemName: entry.icon}, entry.placement)
			return nil
		})
		if entry.placement == ui.IconOnly {
			button.SetTooltip("Icon-only button. Its text is the accessible name.")
		}
		buttons[i] = button
	}
	return g.column(g.row(buttons...), g.states(buttons...))
}

func (g *gallery) buttonStatesDemo() ui.Widget {
	busy := g.button("Upload", func() {
		g.log("Upload clicked")
	})
	busyToggle := g.checkBox("Busy", false, func(checked bool) {
		busy.SetBusy(checked)
		g.log("Upload busy: %t", busy.IsBusy())
	})

	count := 3
	inbox := g.button("Inbox", func() {
		g.log("Inbox clicked")
	})
	setCount := func(value int) {
		count = max(0, value)
		badge := ""
		if count > 0 {
			badge = strconv.Itoa(count)
		}
		inbox.SetBadge(badge)
		g.log("Inbox badge: %q", inbox.Badge())
	}
	g.attempt("Badge", func() error {
		setCount(count)
		return nil
	})
	more := g.button("+1", func() {
		setCount(count + 1)
	})
	fewer := g.button("−1", func() {
		setCount(count - 1)
	})

	shortcut := g.button("Search", func() {
		g.log("Search activated")
	})
	g.attempt("Shortcut", func() error {
		shortcut.SetShortcut(ui.Shortcut{Key: "K", Command: true})
		return nil
	})
	shortcutLabel := g.caption(fmt.Sprintf("Shortcut: %s", describeShortcut(shortcut.Shortcut())))

	return g.column(
		g.labeled("Busy state", busy, busyToggle),
		g.labeled("Badge", inbox, more, fewer),
		g.labeled("Shortcut", shortcut, shortcutLabel),
	)
}

func (g *gallery) toggleButtonsDemo() ui.Widget {
	bold := must(g.window.CreateToggleButton(ui.ToggleButtonOptions{
		Text: "Favorite",
		Icon: ui.IconSource{SystemName: "heart"},
	}))
	bold.OnChange(func(checked bool) {
		g.log("Favorite: %t", checked)
	})

	alignments := []string{"Left", "Center", "Right"}
	group := make([]ui.Widget, len(alignments))
	for i, text := range alignments {
		toggle := must(g.window.CreateToggleButton(ui.ToggleButtonOptions{
			Text:         text,
			Checked:      i == 0,
			GroupID:      "alignment",
			Presentation: ui.ToggleButtonToolbar,
		}))
		toggle.OnChange(func(checked bool) {
			g.log("Align %s: %t", text, checked)
		})
		group[i] = toggle
	}

	all := append([]ui.Widget{bold}, group...)
	return g.column(
		g.labeled("Independent", bold),
		g.labeled("Exclusive group", group...),
		g.states(all...),
	)
}

func (g *gallery) linksDemo() ui.Widget {
	reported := must(g.window.CreateLink("Report activation", "https://example.com/report"))
	reported.OnClick(func() {
		g.log("Link activated: %s (visited: %t)", reported.URL(), reported.IsVisited())
	})

	browser := must(g.window.CreateLink("Open GameKit on GitHub", "https://github.com/bluescreen10/gamekit"))
	g.attempt("Link policy", func() error {
		browser.SetOpenPolicy(ui.LinkOpenPlatform)
		return nil
	})
	browser.OnClick(func() {
		g.log("Opening %s", browser.URL())
	})

	withIcon := must(g.window.CreateLink("Help", "https://example.com/help"))
	g.attempt("Link icon", func() error {
		withIcon.SetIcon(ui.IconSource{SystemName: "help"})
		return nil
	})
	withIcon.OnClick(func() {
		g.log("Help link activated")
	})

	visited := g.checkBox("Visited", false, func(checked bool) {
		reported.SetVisited(checked)
		browser.SetVisited(checked)
		withIcon.SetVisited(checked)
	})
	return g.column(
		g.row(reported, browser, withIcon),
		g.row(visited, g.enabledToggle(reported, browser, withIcon)),
	)
}

// describeShortcut returns a shortcut in the form "Command+Shift+K".
func describeShortcut(shortcut ui.Shortcut) string {
	if shortcut.IsZero() {
		return "none"
	}
	text := ""
	if shortcut.Command {
		text += "Command+"
	}
	if shortcut.Control {
		text += "Control+"
	}
	if shortcut.Alt {
		text += "Alt+"
	}
	if shortcut.Shift {
		text += "Shift+"
	}
	return text + shortcut.Key
}
