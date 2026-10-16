package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func containerCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Group box", "Every card on these pages is a group box. This one edits its own title.", g.groupBoxDemo),
		g.card("Sections", "Collapsible sections with a subtitle, an icon, and a badge.", g.sectionsDemo),
		g.card("Accordions", "Only one section of the first accordion opens at a time.", g.accordionsDemo),
		g.card("Tabs", "Change where the tabs sit and how they overflow.", g.tabsDemo),
		g.card("Split views", "Drag the dividers. The side panes collapse.", g.splitViewsDemo),
		g.card("Scroll view", "A view that scrolls in both directions, with scroll bar, behavior, and zoom options.", g.scrollViewDemo),
		g.card("Drawers", "Panels that slide from the left and right edges.", g.drawersDemo),
		g.card("Popover", "A transient popover that points to its anchor.", g.popoverDemo),
		g.card("Docking layout", "Move the inspector between dock areas and hide the console.", g.dockingDemo),
	}
}

func (g *gallery) groupBoxDemo() ui.Widget {
	title := must(g.window.CreateTextField())
	title.SetText("Editable title")
	subtitle := must(g.window.CreateTextField())
	subtitle.SetText("And an editable subtitle")
	box := must(g.window.CreateGroupBox(ui.GroupBoxOptions{
		Title:    title.Text(),
		Subtitle: subtitle.Text(),
		Content:  g.layout(ui.Column(ui.Item(g.labeledWide("Title", title)), ui.Item(g.labeledWide("Subtitle", subtitle))).WithGap(8).WithPadding(ui.UniformInsets(8))),
	}))
	title.OnChange(func(text string) {
		box.SetTitle(text)
	})
	subtitle.OnChange(func(text string) {
		box.SetSubtitle(text)
	})
	return box
}

func (g *gallery) sectionsDemo() ui.Widget {
	content := func(text string) ui.Widget {
		return g.layout(ui.Column(ui.Item(g.caption(text))).WithPadding(ui.Insets{Top: 4, Left: 18, Bottom: 4}))
	}
	expanded := must(g.window.CreateSection(ui.SectionOptions{
		Title:    "Rendering",
		Content:  content("Resolution, frame rate, and vertical sync settings would go here."),
		Expanded: true,
	}))
	decorated := must(g.window.CreateSection(ui.SectionOptions{
		Title:   "Downloads",
		Content: content("Three downloads are in progress."),
	}))
	g.attempt("Section decorations", func() error {
		decorated.SetSubtitle("Collapsed at start")
		decorated.SetIcon(ui.IconSource{SystemName: "download"})
		decorated.SetBadge("3")
		return nil
	})
	for _, section := range []ui.Section{expanded, decorated} {
		section.OnChange(func(isExpanded bool) {
			g.log("%s expanded: %t", section.Title(), isExpanded)
		})
	}
	animated := g.checkBox("Animate", false, func(checked bool) {
		expanded.SetAnimationEnabled(checked)
		decorated.SetAnimationEnabled(checked)
		g.log("Section animation: %t", expanded.IsAnimationEnabled())
	})
	toggle := g.button("Toggle both", func() {
		expanded.SetExpanded(!expanded.IsExpanded())
		decorated.SetExpanded(!decorated.IsExpanded())
	})
	return g.column(expanded, decorated, g.row(toggle, animated, g.enabledToggle(expanded, decorated)))
}

func (g *gallery) accordionsDemo() ui.Widget {
	sections := func(prefix string) []ui.AccordionSection {
		names := []string{"General", "Graphics", "Audio"}
		result := make([]ui.AccordionSection, len(names))
		for i, name := range names {
			result[i] = ui.AccordionSection{
				ID:       strings.ToLower(name),
				Title:    name,
				Content:  g.layout(ui.Column(ui.Item(g.caption(fmt.Sprintf("%s settings for %s.", name, prefix)))).WithPadding(ui.UniformInsets(8))),
				Expanded: i == 0,
			}
		}
		return result
	}
	single := must(g.window.CreateAccordion(ui.AccordionOptions{Sections: sections("the single accordion"), Expansion: ui.AccordionSingle}))
	single.OnChange(func(ids []string) {
		g.log("Single accordion: [%s]", strings.Join(ids, ", "))
	})
	multiple := must(g.window.CreateAccordion(ui.AccordionOptions{Sections: sections("the multiple accordion"), Expansion: ui.AccordionMultiple}))
	multiple.OnChange(func(ids []string) {
		g.log("Multiple accordion: [%s]", strings.Join(ids, ", "))
	})
	expandAll := g.button("Expand all", func() {
		for _, id := range []string{"general", "graphics", "audio"} {
			g.attempt("Expand", func() error {
				return multiple.SetExpanded(id, true)
			})
		}
	})
	return g.layout(ui.Row(
		ui.Item(g.column(g.label("Single"), single)).Grow(1),
		ui.Item(g.column(g.label("Multiple"), multiple, g.row(expandAll))).Grow(1),
	).WithGap(16))
}

func (g *gallery) tabsDemo() ui.Widget {
	names := []string{"Scene", "Assets", "Console", "Profiler", "Network", "Settings"}
	items := make([]ui.TabItem, len(names))
	for i, name := range names {
		items[i] = ui.TabItem{
			Title:   name,
			Content: g.layout(ui.Column(ui.Item(g.caption(fmt.Sprintf("Content of the %s tab.", name))), ui.Space()).WithPadding(ui.UniformInsets(12))),
		}
	}
	tabs := must(g.window.CreateTabs(ui.TabsOptions{Items: items, SelectedIndex: 0}))
	tabs.SetMinimumSize(ui.Size{Height: 160})
	tabs.OnChange(func(index int) {
		g.log("Tab: %s", tabs.Items()[index].Title)
	})
	placements := []ui.TabPlacement{ui.TabPlacementTop, ui.TabPlacementBottom, ui.TabPlacementLeft, ui.TabPlacementRight}
	placement := g.choice([]string{"Top", "Bottom", "Left", "Right"}, 0, func(index int) {
		tabs.SetPlacement(placements[index])
	})
	overflow := g.choice([]string{"Overflow menu", "Scroll on overflow"}, 0, func(index int) {
		tabs.SetOverflowPolicy([]ui.TabOverflowPolicy{ui.TabOverflowMenu, ui.TabOverflowScroll}[index])
	})
	last := g.button("Select last tab", func() {
		if err := tabs.SetSelectedIndex(len(names) - 1); err != nil {
			g.log("Tabs: %v", err)
		}
	})
	return g.column(tabs, g.row(placement, overflow, last, g.enabledToggle(tabs)))
}

func (g *gallery) splitViewsDemo() ui.Widget {
	pane := func(text string) ui.Widget {
		return g.layout(ui.Column(ui.Item(g.caption(text)), ui.Space()).WithPadding(ui.UniformInsets(8)))
	}
	horizontal := must(g.window.CreateSplitView(ui.SplitViewOptions{
		Direction: ui.Horizontal,
		Panes: []ui.SplitPane{
			{Content: pane("Sidebar (collapsible, at least 80 pixels)"), Minimum: 80, Collapsible: true},
			{Content: pane("Main area (at least 160 pixels)"), Minimum: 160},
			{Content: pane("Inspector (collapsible)"), Minimum: 80, Collapsible: true},
		},
	}))
	horizontal.SetMinimumSize(ui.Size{Height: 140})
	vertical := must(g.window.CreateSplitView(ui.SplitViewOptions{
		Direction: ui.Vertical,
		Panes: []ui.SplitPane{
			{Content: pane("Editor"), Minimum: 40},
			{Content: pane("Terminal"), Minimum: 40, Collapsible: true},
		},
	}))
	vertical.SetMinimumSize(ui.Size{Height: 180})
	g.logDividerChanges("Horizontal dividers", horizontal)
	g.logDividerChanges("Vertical divider", vertical)

	reset := func() {
		width := horizontal.Bounds().Size.Width
		g.attempt("Dividers", func() error {
			return horizontal.SetDividers([]int{width / 4, width * 3 / 4})
		})
		g.attempt("Dividers", func() error {
			return vertical.SetDividers([]int{90})
		})
	}
	// The split views have their final size after the page is shown.
	g.after(0, reset)
	return g.column(horizontal, vertical, g.row(g.button("Reset dividers", reset)))
}

// logDividerChanges logs the divider positions of view when they change.
func (g *gallery) logDividerChanges(name string, view ui.SplitView) {
	previous := ""
	view.OnChange(func(positions []int) {
		current := fmt.Sprint(positions)
		if current == previous {
			return
		}
		previous = current
		g.log("%s: %s", name, current)
	})
}

func (g *gallery) scrollViewDemo() ui.Widget {
	rows := make([]ui.LayoutItem, 30)
	var target ui.Widget
	for i := range rows {
		cells := make([]ui.LayoutItem, 12)
		for j := range cells {
			cell := g.label(fmt.Sprintf("R%02d C%02d", i+1, j+1))
			cell.SetMinimumSize(ui.Size{Width: 70})
			if i == 24 && j == 10 {
				target = cell
				cell.SetText("Target")
			}
			cells[j] = ui.Item(cell)
		}
		rows[i] = ui.Item(g.layout(ui.Row(cells...).WithGap(8)))
	}
	content := g.layout(ui.Column(rows...).WithGap(8).WithPadding(ui.UniformInsets(8)))
	scroll := must(g.window.CreateScrollView(ui.ScrollViewOptions{Content: content, Axes: ui.ScrollBoth}))
	scroll.SetMinimumSize(ui.Size{Height: 220})
	offset := g.value("Offset 0, 0")
	g.every(func(time.Time) bool {
		position := scroll.ScrollOffset()
		viewport := scroll.ViewportSize()
		offset.SetText(fmt.Sprintf("Offset %d, %d; viewport %d × %d", position.X, position.Y, viewport.Width, viewport.Height))
		return true
	})

	policy := g.choice([]string{"Automatic scroll bars", "Always show", "Never show"}, 0, func(index int) {
		scroll.SetScrollbarPolicy([]ui.ScrollbarPolicy{ui.ScrollbarsAutomatic, ui.ScrollbarsAlways, ui.ScrollbarsNever}[index])
	})
	smooth := g.checkBox("Smooth scrolling", false, func(checked bool) {
		behavior := ui.ScrollInstant
		if checked {
			behavior = ui.ScrollSmooth
		}
		scroll.SetScrollBehavior(behavior)
	})
	bounce := g.checkBox("Overscroll", true, func(checked bool) {
		policy := ui.OverscrollDisabled
		if checked {
			policy = ui.OverscrollAutomatic
		}
		scroll.SetOverscrollPolicy(policy)
	})
	zooms := []float64{0.5, 1, 1.5, 2}
	zoom := g.choice([]string{"50%", "100%", "150%", "200%"}, 1, func(index int) {
		if err := scroll.SetZoom(zooms[index]); err != nil {
			g.log("Zoom: %v", err)
		}
	})
	insets := g.checkBox("Content insets", false, func(checked bool) {
		value := ui.Insets{}
		if checked {
			value = ui.UniformInsets(24)
		}
		scroll.SetContentInsets(value)
	})
	top := g.button("Top left", func() {
		scroll.SetScrollOffset(ui.Point{})
	})
	reveal := g.button("Scroll to target", func() {
		if err := scroll.ScrollToWidget(target); err != nil {
			g.log("Scroll to target: %v", err)
		}
	})
	return g.column(
		scroll,
		offset,
		g.row(policy, zoom, smooth, bounce, insets),
		g.row(top, reveal),
	)
}

func (g *gallery) drawersDemo() ui.Widget {
	drawerContent := func(text string) ui.Widget {
		return g.layout(ui.Column(ui.Item(g.label(text)), ui.Item(g.caption("Drawer content")), ui.Space()).
			WithPadding(ui.UniformInsets(10)).WithGap(4))
	}
	left := must(g.window.CreateDrawer(ui.DrawerOptions{Edge: ui.EdgeLeft, Content: drawerContent("Left drawer"), Open: true, Width: 160}))
	right := must(g.window.CreateDrawer(ui.DrawerOptions{Edge: ui.EdgeRight, Content: drawerContent("Right drawer"), Width: 180}))
	for _, drawer := range []ui.Drawer{left, right} {
		drawer.OnChange(func(open bool) {
			g.log("Drawer open: %t", open)
		})
	}
	toggleLeft := g.button("Toggle left", func() {
		left.SetOpen(!left.IsOpen())
	})
	toggleRight := g.button("Toggle right", func() {
		right.SetOpen(!right.IsOpen())
	})
	main := g.layout(ui.Column(
		ui.Item(g.caption("Main content. The drawers take space from it when they open.")),
		ui.Item(g.row(toggleLeft, toggleRight)),
		ui.Space(),
	).WithPadding(ui.UniformInsets(10)).WithGap(8))
	area := g.layout(ui.Row(ui.Item(left), ui.Item(main).Grow(1), ui.Item(right)))
	area.SetMinimumSize(ui.Size{Height: 150})
	return area
}

func (g *gallery) popoverDemo() ui.Widget {
	edges := []struct {
		text string
		edge ui.Edge
	}{
		{"Below", ui.EdgeBottom},
		{"Above", ui.EdgeTop},
		{"Left", ui.EdgeLeft},
		{"Right", ui.EdgeRight},
	}
	anchors := make([]ui.Widget, len(edges))
	for i, entry := range edges {
		anchors[i] = g.popoverButton(entry.text, entry.edge)
	}
	return g.row(anchors...)
}

// popoverButton returns a button that opens a popover at edge.
func (g *gallery) popoverButton(text string, edge ui.Edge) ui.Widget {
	volume := g.value("Volume 70%")
	slider := must(g.window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 0.7}))
	slider.OnChange(func(value float64) {
		volume.SetText(fmt.Sprintf("Volume %.0f%%", value*100))
	})
	content := g.layout(ui.Column(ui.Item(g.label("Popover "+strings.ToLower(text))), ui.Item(slider), ui.Item(volume)).
		WithPadding(ui.UniformInsets(12)).WithGap(8))
	content.SetMinimumSize(ui.Size{Width: 220})

	var popover ui.Popover
	anchor := g.button(text, func() {
		if popover.IsShown() {
			popover.Hide()
			return
		}
		if err := popover.Show(); err != nil {
			g.log("Popover: %v", err)
		}
	})
	popover = must(g.window.CreatePopover(ui.PopoverOptions{
		Anchor:        anchor,
		Content:       content,
		PreferredEdge: edge,
		Transient:     true,
	}))
	popover.OnHide(func() {
		g.log("Popover %s hidden", strings.ToLower(text))
	})
	return anchor
}

func (g *gallery) dockingDemo() ui.Widget {
	pane := func(text string) ui.Widget {
		return g.layout(ui.Column(ui.Item(g.caption(text)), ui.Space()).WithPadding(ui.UniformInsets(8)))
	}
	dock := must(g.window.CreateDockingLayout(ui.DockingLayoutOptions{Panes: []ui.DockPane{
		{ID: "explorer", Title: "Explorer", Content: pane("Project files"), Area: ui.DockLeft},
		{ID: "editor", Title: "Editor", Content: pane("The center pane fills the remaining space."), Area: ui.DockCenter},
		{ID: "inspector", Title: "Inspector", Content: pane("Properties"), Area: ui.DockRight},
		{ID: "console", Title: "Console", Content: pane("Log output"), Area: ui.DockBottom},
	}}))
	dock.SetMinimumSize(ui.Size{Height: 260})
	areas := []ui.DockArea{ui.DockRight, ui.DockLeft, ui.DockTop, ui.DockBottom}
	area := g.choice([]string{"Right", "Left", "Top", "Bottom"}, 0, func(index int) {
		if err := dock.MovePane("inspector", areas[index]); err != nil {
			g.log("Move inspector: %v", err)
		}
	})
	console := g.checkBox("Console visible", true, func(checked bool) {
		if err := dock.SetPaneVisible("console", checked); err != nil {
			g.log("Console: %v", err)
		}
	})
	return g.column(dock, g.row(g.label("Inspector area"), area, console))
}
