package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

// maximumLoggedEvents is the number of events that the event log keeps.
const maximumLoggedEvents = 300

// sidebarWidth is the first width of the sidebar, in logical pixels.
const sidebarWidth = 220

// gallery owns the main window, the page navigation, and the event log.
type gallery struct {
	backend ui.Backend
	window  ui.Window

	groups      []pageGroup
	currentPage *page
	switching   bool
	pageHost    ui.Layout
	sidebar     ui.NavigationList
	statusBar   ui.StatusBar
	menuBar     ui.MenuBar

	events       []string
	eventLog     ui.TextArea
	printsEvents bool

	unavailableCount int

	timers []*timer
}

// pageGroup is a section of the sidebar.
type pageGroup struct {
	id    string
	title string
	pages []*page
}

// page is one entry of the sidebar. Its widget is built when the user first
// selects it.
type page struct {
	id     string
	title  string
	detail string
	icon   string
	cards  func(g *gallery) []ui.Widget
	widget ui.Widget
}

// timer calls tick on every iteration of the main loop until tick returns
// false.
type timer struct {
	tick func(now time.Time) bool
}

func newGallery(backend ui.Backend, window ui.Window) *gallery {
	g := &gallery{
		backend: backend,
		window:  window,
	}
	g.groups = galleryPages()
	return g
}

// galleryPages lists the sidebar sections and their pages in display order.
func galleryPages() []pageGroup {
	return []pageGroup{
		{id: "controls", title: "Controls", pages: []*page{
			{id: "buttons", title: "Buttons", icon: "play", cards: buttonCards,
				detail: "Push buttons, button roles, icons, toggle buttons, and links."},
			{id: "toggles", title: "Toggles & choices", icon: "visible", cards: toggleCards,
				detail: "Check boxes, switches, radio groups, and segmented controls."},
			{id: "text-input", title: "Text input", icon: "edit", cards: textInputCards,
				detail: "Text fields, password and search fields, text areas, combo boxes, and shortcut recorders."},
			{id: "values", title: "Values & ranges", icon: "settings", cards: valueCards,
				detail: "Sliders, number fields, steppers, dials, progress bars, and activity indicators."},
			{id: "pickers", title: "Pickers", icon: "calendar", cards: pickerCards,
				detail: "Pop-up selects, date and time pickers, calendars, and color and font pickers."},
		}},
		{id: "content", title: "Content", pages: []*page{
			{id: "text", title: "Text display", icon: "document", cards: textDisplayCards,
				detail: "Labels, semantic text styles, wrapping and truncation, rich text, and Markdown."},
			{id: "media", title: "Images & media", icon: "image", cards: mediaCards,
				detail: "Image views, icons, SVG documents, animated images, video, and web content."},
			{id: "feedback", title: "Feedback", icon: "notification", cards: feedbackCards,
				detail: "Info bars, inline validation, badges, toasts, tooltips, progress dialogs, and empty states."},
		}},
		{id: "layout", title: "Layout", pages: []*page{
			{id: "layouts", title: "Layouts", icon: "menu", cards: layoutCards,
				detail: "Rows and columns, grids, flow, overlay and form layouts, separators, margins, and size limits."},
			{id: "containers", title: "Containers", icon: "folder", cards: containerCards,
				detail: "Group boxes, sections, accordions, tabs, split views, scroll views, drawers, popovers, and docking layouts."},
		}},
		{id: "data", title: "Data", pages: []*page{
			{id: "collections", title: "Lists & tables", icon: "document", cards: collectionCards,
				detail: "List, table, tree, and collection views, and the property inspector."},
			{id: "navigation", title: "Navigation", icon: "home", cards: navigationCards,
				detail: "Navigation lists, breadcrumbs, pagination, and the command palette."},
		}},
		{id: "commands", title: "Commands", pages: []*page{
			{id: "menus", title: "Menus & toolbars", icon: "more", cards: commandCards,
				detail: "The menu bar, toolbar, command bar, context menus, title bar accessories, and the status bar."},
			{id: "dialogs", title: "Dialogs", icon: "open", cards: dialogCards,
				detail: "File dialogs, alerts, confirmations, prompts, the about panel, and dialog cancellation."},
		}},
		{id: "platform", title: "Platform", pages: []*page{
			{id: "window", title: "Window", icon: "home", cards: windowCards,
				detail: "Title, theme, geometry, window states, close requests, other windows, the screen, and system preferences."},
			{id: "behavior", title: "Widget behavior", icon: "user", cards: behaviorCards,
				detail: "Styles, cursors, tooltips, focus, drag and drop, and accessibility metadata."},
			{id: "input", title: "Input & canvas", icon: "game-controller", cards: inputCards,
				detail: "The GPU canvas and its input streams, window input, and pointer modes."},
			{id: "services", title: "Services", icon: "share", cards: serviceCards,
				detail: "Clipboard, notifications, the system tray, recent documents, URLs, printing, and sharing."},
			{id: "events", title: "Event log", icon: "clock", cards: eventLogCards,
				detail: "Every event and result that the gallery reports, newest last."},
		}},
	}
}

// build creates the main window content: the sidebar, the page area, and the
// status bar.
func (g *gallery) build(startPage string) error {
	statusBar, err := g.window.CreateStatusBar(ui.StatusBarOptions{Text: "Ready"})
	if err != nil {
		return err
	}
	g.statusBar = statusBar

	sidebar, err := g.createSidebar(startPage)
	if err != nil {
		return err
	}
	pageHost, err := g.window.CreateLayout(ui.Column())
	if err != nil {
		return err
	}
	g.pageHost = pageHost

	// The user can drag the divider to make the sidebar wider.
	body, err := g.window.CreateSplitView(ui.SplitViewOptions{
		Direction: ui.Horizontal,
		Panes: []ui.SplitPane{
			{Content: sidebar, Minimum: 160},
			{Content: pageHost, Minimum: 480},
		},
		Dividers: []int{sidebarWidth},
	})
	if err != nil {
		return err
	}
	// The split view has its final size after the window is shown.
	g.after(0, func() {
		g.attempt("Sidebar width", func() error {
			return body.SetDividers([]int{sidebarWidth})
		})
	})
	root, err := g.window.CreateLayout(ui.Column(
		ui.Item(body).Grow(1),
		ui.Item(statusBar),
	))
	if err != nil {
		return err
	}
	if err := g.window.SetContent(root); err != nil {
		return err
	}

	g.installMenuBar()
	if g.findPage(startPage) == nil {
		return fmt.Errorf("no page has the ID %q", startPage)
	}
	g.showPage(startPage)
	return nil
}

func (g *gallery) createSidebar(startPage string) (ui.NavigationList, error) {
	sections := make([]ui.NavigationSection, len(g.groups))
	for i, group := range g.groups {
		items := make([]ui.NavigationItem, len(group.pages))
		for j, entry := range group.pages {
			items[j] = ui.NavigationItem{
				ID:   entry.id,
				Text: entry.title,
				Icon: ui.IconSource{SystemName: entry.icon},
			}
		}
		sections[i] = ui.NavigationSection{ID: group.id, Title: group.title, Items: items}
	}
	sidebar, err := g.window.CreateNavigationList(ui.NavigationListOptions{
		Sections:   sections,
		SelectedID: startPage,
	})
	if err != nil {
		return nil, err
	}
	g.sidebar = sidebar
	sidebar.OnChange(func(id string) {
		g.showPage(id)
	})
	return sidebar, nil
}

// showPage puts the page with id in the page area. It builds the page the
// first time.
func (g *gallery) showPage(id string) {
	next := g.findPage(id)
	if next == nil || next == g.currentPage || g.switching {
		return
	}
	// Some backend calls run the platform event loop, which can deliver a
	// sidebar selection while a page is being built.
	g.switching = true
	defer func() {
		g.switching = false
	}()
	if next.widget == nil {
		next.widget = g.buildSafely(func() ui.Widget {
			return g.buildPage(next)
		})
	}
	if g.currentPage != nil {
		g.currentPage.widget.SetVisible(false)
	}
	if err := g.pageHost.SetSpec(ui.Column(ui.Item(next.widget).Grow(1))); err != nil {
		g.log("Show page %s: %v", next.title, err)
		return
	}
	next.widget.SetVisible(true)
	g.currentPage = next
	if g.sidebar != nil && g.sidebar.SelectedID() != id {
		g.attempt("Sidebar", func() error {
			return g.sidebar.SetSelectedID(id)
		})
	}
	g.log("Page: %s", next.title)
}

func (g *gallery) findPage(id string) *page {
	for _, group := range g.groups {
		for _, entry := range group.pages {
			if entry.id == id {
				return entry
			}
		}
	}
	return nil
}

// buildPage creates the header of a page and a scroll view with its cards.
func (g *gallery) buildPage(entry *page) ui.Widget {
	title := g.label(entry.title)
	title.SetSemanticStyle(ui.TextStyleTitle)
	detail := g.caption(entry.detail)
	divider := must(g.window.CreateSeparator(ui.Horizontal))

	cards := entry.cards(g)
	items := make([]ui.LayoutItem, len(cards))
	for i, card := range cards {
		items[i] = ui.Item(card)
	}
	content := g.layout(ui.Column(items...).WithPadding(ui.UniformInsets(16)).WithGap(16))
	scroll := must(g.window.CreateScrollView(ui.ScrollViewOptions{
		Content: content,
		Axes:    ui.ScrollVertical,
	}))

	header := g.layout(ui.Column(ui.Item(title), ui.Item(detail)).
		WithPadding(ui.Insets{Top: 14, Right: 16, Bottom: 10, Left: 16}).WithGap(4))
	return g.layout(ui.Column(
		ui.Item(header),
		ui.Item(divider).MinimumSize(ui.Size{Height: 1}),
		ui.Item(scroll).Grow(1),
	))
}

// log shows an event in the status bar and adds it to the event log.
func (g *gallery) log(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	g.statusBar.SetText(message)

	entry := time.Now().Format("15:04:05.000") + "  " + message
	if g.printsEvents {
		fmt.Println(entry)
	}
	g.events = append(g.events, entry)
	if len(g.events) > maximumLoggedEvents {
		g.events = g.events[len(g.events)-maximumLoggedEvents:]
	}
	if g.eventLog != nil {
		g.eventLog.SetText(strings.Join(g.events, "\n"))
		g.eventLog.SetScrollPosition(ui.Point{Y: 1 << 30})
	}
}

// attempt runs step and logs the error that it returns or the panic that it
// causes. Backends panic with ui.ErrNotSupported from setters that the
// platform cannot provide.
func (g *gallery) attempt(name string, step func() error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			g.log("%s: %v", name, recovered)
		}
	}()
	if err := step(); err != nil {
		g.log("%s: %v", name, err)
	}
}

// after calls action once, when delay has passed.
func (g *gallery) after(delay time.Duration, action func()) {
	deadline := time.Now().Add(delay)
	g.every(func(now time.Time) bool {
		if now.Before(deadline) {
			return true
		}
		action()
		return false
	})
}

// every calls tick on each iteration of the main loop until it returns
// false.
func (g *gallery) every(tick func(now time.Time) bool) {
	g.timers = append(g.timers, &timer{tick: tick})
}

func (g *gallery) runTimers(now time.Time) {
	active := g.timers
	g.timers = nil
	for _, t := range active {
		keep := true
		g.attempt("Timer", func() error {
			keep = t.tick(now)
			return nil
		})
		if keep {
			g.timers = append(g.timers, t)
		}
	}
}
