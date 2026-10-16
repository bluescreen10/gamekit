package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func windowCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Title and theme", "Edit the title of the window and change its theme.", g.titleThemeDemo),
		g.card("Geometry", "Size and position readouts, with buttons that move and resize the window.", g.geometryDemo),
		g.card("Window states", "Minimize restores the window after two seconds.", g.windowStatesDemo),
		g.card("Window chrome", "Resizable, borderless, and always-on-top windows, and size limits.", g.windowChromeDemo),
		g.card("Close requests", "Ask before the window closes.", g.closeRequestDemo),
		g.card("Other windows", "A utility window that its owner keeps on top, and a second normal window.", g.otherWindowsDemo),
		g.card("Screen and preferences", "The screen of the window and the system settings that affect the UI.", g.screenDemo),
		g.card("Focus and activation", "Window focus and application activation events.", g.activationDemo),
		g.card("File drop", "Drag files from the file manager onto this window.", g.fileDropDemo),
	}
}

func (g *gallery) titleThemeDemo() ui.Widget {
	title := must(g.window.CreateTextField())
	title.SetText(g.window.Title())
	title.OnChange(func(text string) {
		g.window.SetTitle(text)
	})
	selected := 0
	for i, entry := range themes {
		if entry.theme == g.window.Theme() {
			selected = i
		}
	}
	names := make([]string, len(themes))
	for i, entry := range themes {
		names[i] = entry.name
	}
	theme := g.choice(names, selected, func(index int) {
		if err := g.setTheme(themes[index].theme); err != nil {
			g.log("Theme: %v", err)
		}
	})
	g.window.OnThemeChange(func(theme ui.Theme) {
		g.log("Theme changed: %d", theme)
	})
	return g.column(g.labeledWide("Title", title), g.labeled("Theme", theme))
}

func (g *gallery) geometryDemo() ui.Widget {
	size := g.value("")
	position := g.value("")
	framebuffer := g.value("")
	update := func() {
		current := g.window.Size()
		size.SetText(fmt.Sprintf("%d × %d", current.Width, current.Height))
		origin := g.window.Position()
		position.SetText(fmt.Sprintf("%d, %d", origin.X, origin.Y))
		pixels := g.window.FramebufferSize()
		framebuffer.SetText(fmt.Sprintf("%d × %d", pixels.Width, pixels.Height))
	}
	update()
	g.window.OnResize(func(ui.Size) {
		update()
	})
	g.window.OnMove(func(ui.Point) {
		update()
	})

	resize := func(width, height int) func() {
		return func() {
			if err := g.window.SetSize(ui.Size{Width: width, Height: height}); err != nil {
				g.log("Size: %v", err)
			}
		}
	}
	center := g.button("Center on screen", func() {
		area := g.window.Screen().WorkArea
		current := g.window.Size()
		g.window.SetPosition(ui.Point{
			X: area.Position.X + (area.Size.Width-current.Width)/2,
			Y: area.Position.Y + (area.Size.Height-current.Height)/2,
		})
	})
	corner := g.button("Top-left corner", func() {
		g.window.SetPosition(g.window.Screen().WorkArea.Position)
	})
	return g.column(
		g.labeled("Content size", size),
		g.labeled("Position", position),
		g.labeled("Framebuffer", framebuffer),
		g.row(g.button("900 × 640", resize(900, 640)), g.button("1120 × 780", resize(1120, 780)),
			g.button("1400 × 900", resize(1400, 900)), center, corner),
	)
}

func (g *gallery) windowStatesDemo() ui.Widget {
	state := g.value("")
	update := func() {
		state.SetText(fmt.Sprintf("minimized %t, maximized %t, fullscreen %t",
			g.window.IsMinimized(), g.window.IsMaximized(), g.window.IsFullscreen()))
	}
	update()
	g.window.OnFullscreenChange(func(fullscreen bool) {
		update()
		g.log("Fullscreen: %t", fullscreen)
	})
	g.window.OnResize(func(ui.Size) {
		update()
	})
	minimize := g.button("Minimize", func() {
		if err := g.window.Minimize(); err != nil {
			g.log("Minimize: %v", err)
			return
		}
		g.after(2*time.Second, func() {
			if err := g.window.Restore(); err != nil {
				g.log("Restore: %v", err)
			}
			g.window.BringToFront()
			update()
		})
	})
	maximize := g.button("Maximize", func() {
		if err := g.window.Maximize(); err != nil {
			g.log("Maximize: %v", err)
		}
		update()
	})
	restore := g.button("Restore", func() {
		if err := g.window.Restore(); err != nil {
			g.log("Restore: %v", err)
		}
		update()
	})
	fullscreen := g.button("Toggle fullscreen", func() {
		if err := g.window.SetFullscreen(!g.window.IsFullscreen()); err != nil {
			g.log("Fullscreen: %v", err)
		}
	})
	return g.column(g.row(minimize, maximize, restore, fullscreen), state)
}

func (g *gallery) windowChromeDemo() ui.Widget {
	resizable := g.checkBox("Resizable", g.window.IsResizable(), func(checked bool) {
		g.window.SetResizable(checked)
		g.log("Resizable: %t", g.window.IsResizable())
	})
	borderless := g.checkBox("Borderless", g.window.IsBorderless(), func(checked bool) {
		g.window.SetBorderless(checked)
		g.log("Borderless: %t", g.window.IsBorderless())
	})
	onTop := g.checkBox("Always on top", g.window.IsAlwaysOnTop(), func(checked bool) {
		g.window.SetAlwaysOnTop(checked)
		g.log("Always on top: %t", g.window.IsAlwaysOnTop())
	})
	limits := g.checkBox("Limit size to 800 × 560 – 1300 × 900", false, func(checked bool) {
		minimum, maximum := ui.Size{Width: 760, Height: 520}, ui.Size{}
		if checked {
			minimum, maximum = ui.Size{Width: 800, Height: 560}, ui.Size{Width: 1300, Height: 900}
		}
		if err := g.window.SetMaximumSize(maximum); err != nil {
			g.log("Maximum size: %v", err)
		}
		if err := g.window.SetMinimumSize(minimum); err != nil {
			g.log("Minimum size: %v", err)
		}
		g.log("Size limits: %+v to %+v", g.window.MinimumSize(), g.window.MaximumSize())
	})
	return g.column(g.row(resizable, borderless, onTop), limits)
}

func (g *gallery) closeRequestDemo() ui.Widget {
	confirm := false
	g.window.OnCloseRequest(func() bool {
		if !confirm {
			return true
		}
		g.log("Close blocked. Clear “Keep the window open” to allow it.")
		return false
	})
	keepOpen := g.checkBox("Keep the window open", false, func(checked bool) {
		confirm = checked
	})
	return g.column(g.caption("Click the close button of the window, or press ⌘W or Ctrl+W, while the box is checked."), keepOpen)
}

func (g *gallery) otherWindowsDemo() ui.Widget {
	inspector := must(g.backend.CreateWindow(ui.WindowOptions{
		Title:    "Inspector",
		Size:     ui.Size{Width: 280, Height: 140},
		Kind:     ui.WindowUtility,
		Owner:    g.window,
		Position: ui.Point{X: 40, Y: 80},
		Hidden:   true,
		Theme:    g.window.Theme(),
	}))
	body := must(inspector.CreateLayout(ui.Column(
		ui.Item(must(inspector.CreateLabel("A utility window. Drag it by its title bar."))),
		ui.Space(),
	).WithPadding(ui.UniformInsets(10))))
	if err := inspector.SetContent(body); err != nil {
		panic(err)
	}
	inspector.OnMove(func(position ui.Point) {
		g.log("Inspector position: %d, %d", position.X, position.Y)
	})
	g.window.OnThemeChange(func(theme ui.Theme) {
		g.attempt("Inspector theme", func() error {
			return inspector.SetTheme(theme)
		})
	})
	show := g.button("Show inspector", func() {
		inspector.SetShouldClose(false)
		inspector.SetVisible(true)
		inspector.BringToFront()
	})
	hide := g.button("Hide inspector", func() {
		inspector.SetVisible(false)
	})
	second := g.button("Open a second window", func() {
		if err := g.openSecondWindow(); err != nil {
			g.log("Second window: %v", err)
		}
	})
	return g.row(show, hide, second)
}

// openSecondWindow opens another normal window, which runs its own event
// processing until the user closes it.
func (g *gallery) openSecondWindow() error {
	window, err := g.backend.CreateWindow(ui.WindowOptions{
		Title:     "Second window",
		Size:      ui.Size{Width: 360, Height: 200},
		Resizable: true,
		Theme:     g.window.Theme(),
	})
	if err != nil {
		return err
	}
	label, err := window.CreateLabel("A second top-level window.")
	if err != nil {
		window.Close()
		return err
	}
	closeButton, err := window.CreateButton("Close")
	if err != nil {
		window.Close()
		return err
	}
	closeButton.OnClick(func() {
		window.SetShouldClose(true)
	})
	body, err := window.CreateLayout(ui.Column(ui.Item(label), ui.Space(), ui.Item(closeButton).Align(ui.AlignEnd)).
		WithPadding(ui.UniformInsets(14)).WithGap(10))
	if err != nil {
		window.Close()
		return err
	}
	if err := window.SetContent(body); err != nil {
		window.Close()
		return err
	}
	g.every(func(time.Time) bool {
		window.PollEvents()
		if !window.ShouldClose() {
			return true
		}
		window.Close()
		g.log("Second window closed")
		return false
	})
	g.log("Second window opened")
	return nil
}

func (g *gallery) screenDemo() ui.Widget {
	screen := g.value("")
	preferences := g.value("")
	update := func() {
		current := g.window.Screen()
		screen.SetText(fmt.Sprintf("%s: %d × %d at %d, %d; scale %.2f, %.0f DPI, %.0f Hz",
			current.Name, current.Bounds.Size.Width, current.Bounds.Size.Height,
			current.Bounds.Position.X, current.Bounds.Position.Y, current.ScaleFactor, current.DPI, current.RefreshRate))
		settings := g.window.SystemPreferences()
		preferences.SetText(fmt.Sprintf("theme %d, text scale %.2f, high contrast %t, reduced motion %t,\n"+
			"reduced transparency %t, accent %s, locale %q, right to left %t",
			settings.ResolvedTheme, settings.TextScale, settings.HighContrast, settings.ReducedMotion,
			settings.ReducedTransparency, formatColor(settings.AccentColor), settings.Locale, settings.RightToLeft))
	}
	update()
	g.window.OnSystemPreferencesChange(func(ui.SystemPreferences) {
		update()
		g.log("System preferences changed")
	})
	g.window.OnMove(func(ui.Point) {
		update()
	})
	preferences.SetWrapMode(ui.WrapWords)
	return g.column(g.labeledWide("Screen", screen), g.labeledWide("Preferences", preferences), g.row(g.button("Refresh", update)))
}

func (g *gallery) activationDemo() ui.Widget {
	events := []struct {
		name      string
		subscribe func(handler func()) ui.Unsubscribe
	}{
		{"Window focused", func(handler func()) ui.Unsubscribe {
			return g.window.OnFocus(handler)
		}},
		{"Window lost focus", func(handler func()) ui.Unsubscribe {
			return g.window.OnBlur(handler)
		}},
		{"Application activated", func(handler func()) ui.Unsubscribe {
			return g.window.OnActivated(handler)
		}},
		{"Application deactivated", func(handler func()) ui.Unsubscribe {
			return g.window.OnDeactivated(handler)
		}},
		{"Application reopened", func(handler func()) ui.Unsubscribe {
			return g.window.OnReopen(handler)
		}},
	}
	lines := []string{}
	for _, event := range events {
		g.attempt(event.name, func() error {
			event.subscribe(func() {
				g.log("%s", event.name)
			})
			lines = append(lines, event.name+": subscribed")
			return nil
		})
	}
	quitConfirm := false
	g.attempt("Quit request", func() error {
		g.window.OnQuitRequest(func() bool {
			if quitConfirm {
				g.log("Quit blocked")
				return false
			}
			return true
		})
		lines = append(lines, "Quit request: subscribed")
		return nil
	})
	blockQuit := g.checkBox("Block quit requests", false, func(checked bool) {
		quitConfirm = checked
	})
	return g.column(g.caption(strings.Join(lines, " · ")), blockQuit)
}

func (g *gallery) fileDropDemo() ui.Widget {
	dropped := g.value("Nothing dropped yet")
	dropped.SetWrapMode(ui.WrapCharacters)
	g.window.OnFileDrop(func(paths []string) {
		dropped.SetText(strings.Join(paths, "\n"))
		g.log("Dropped %d files", len(paths))
	})
	return dropped
}
