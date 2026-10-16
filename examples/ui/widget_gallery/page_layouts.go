package main

import (
	"fmt"

	"github.com/bluescreen10/gamekit/ui"
)

func layoutCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Row growth", "Grow weights of 1, 2, and 3 share the extra width. Resize the window to watch.", g.rowGrowthDemo),
		g.card("Cross-axis alignment", "Stretch, start, center, and end in a tall row.", g.alignmentDemo),
		g.card("Grid layout", "Fixed and growing tracks, with items that span rows and columns.", g.gridDemo),
		g.card("Flow layout", "Children wrap onto new lines when the row is full.", g.flowDemo),
		g.card("Overlay layout", "Children stacked over an image, at anchors with offsets.", g.overlayDemo),
		g.card("Form layout", "Labeled rows, required fields, help text, and row errors.", g.formDemo),
		g.card("Separators", "Full and inset separators, a labeled separator, thickness, and a vertical separator.", g.separatorsDemo),
		g.card("Margins and size limits", "Margins push a widget away from its neighbors. Size limits bound it.", g.marginsDemo),
	}
}

func (g *gallery) rowGrowthDemo() ui.Widget {
	items := make([]ui.LayoutItem, 3)
	for i := range items {
		weight := float32(i + 1)
		button := g.button(fmt.Sprintf("Grow %.0f", weight), func() {
			g.log("Grow %.0f clicked", weight)
		})
		items[i] = ui.Item(button).Grow(weight)
	}
	growing := g.layout(ui.Row(items...).WithGap(8))

	left := g.button("Start", func() {
		g.log("Start clicked")
	})
	right := g.button("End", func() {
		g.log("End clicked")
	})
	spaced := g.layout(ui.Row(ui.Item(left), ui.Space(), ui.Item(right)).WithGap(8))
	return g.column(growing, g.caption("A spacer between two buttons:"), spaced)
}

func (g *gallery) alignmentDemo() ui.Widget {
	alignments := []struct {
		name      string
		alignment ui.Alignment
	}{
		{"Stretch", ui.AlignStretch},
		{"Start", ui.AlignStart},
		{"Center", ui.AlignCenter},
		{"End", ui.AlignEnd},
	}
	items := make([]ui.LayoutItem, len(alignments))
	for i, entry := range alignments {
		button := g.button(entry.name, func() {
			g.log("%s clicked", entry.name)
		})
		items[i] = ui.Item(button).Grow(1).Align(entry.alignment)
	}
	row := g.layout(ui.Row(items...).WithGap(8).WithPadding(ui.UniformInsets(4)))
	row.SetMinimumSize(ui.Size{Height: 90})
	return row
}

func (g *gallery) gridDemo() ui.Widget {
	cell := func(text string) ui.Widget {
		return g.button(text, func() {
			g.log("Grid cell: %s", text)
		})
	}
	grid := must(g.window.CreateGridLayout(ui.GridLayoutOptions{
		Columns: []ui.GridTrack{{Size: 180}, {Grow: 1}, {Grow: 1}},
		Rows:    []ui.GridTrack{{Size: 40}, {Size: 40}, {Size: 40}},
		Items: []ui.GridItem{
			{Content: cell("Spans three rows"), Column: 0, Row: 0, RowSpan: 3},
			{Content: cell("Spans two columns"), Column: 1, Row: 0, ColumnSpan: 2},
			{Content: cell("Row 1, column 1"), Column: 1, Row: 1},
			{Content: cell("Row 1, column 2"), Column: 2, Row: 1},
			{Content: cell("Centered"), Column: 1, Row: 2, Alignment: ui.AlignCenter},
			{Content: cell("End"), Column: 2, Row: 2, Alignment: ui.AlignEnd},
		},
		RowGap:    6,
		ColumnGap: 6,
	}))
	wider := g.checkBox("Wide gaps", false, func(checked bool) {
		options := grid.Options()
		options.RowGap, options.ColumnGap = 6, 6
		if checked {
			options.RowGap, options.ColumnGap = 16, 24
		}
		if err := grid.SetOptions(options); err != nil {
			g.log("Grid: %v", err)
		}
	})
	return g.column(grid, g.row(wider))
}

func (g *gallery) flowDemo() ui.Widget {
	names := []string{"Physics", "Audio", "Rendering", "Networking", "Input", "Scripting", "Animation",
		"Particles", "Terrain", "Lighting", "Shadows", "Post-processing", "UI", "Localization"}
	chips := make([]ui.Widget, len(names))
	for i, name := range names {
		chips[i] = must(g.window.CreateToggleButton(ui.ToggleButtonOptions{Text: name, Checked: i%3 == 0}))
	}
	flow := must(g.window.CreateFlowLayout(ui.FlowLayoutOptions{
		Direction: ui.Horizontal,
		Wrap:      true,
		RowGap:    6,
		ColumnGap: 6,
		Children:  chips,
	}))
	added := 0
	add := g.button("Add a tag", func() {
		added++
		chip, err := g.window.CreateToggleButton(ui.ToggleButtonOptions{Text: fmt.Sprintf("Tag %d", added)})
		if err != nil {
			g.log("Tag: %v", err)
			return
		}
		if err := flow.SetChildren(append(flow.Children(), chip)); err != nil {
			g.log("Flow: %v", err)
		}
	})
	return g.column(flow, g.row(add))
}

func (g *gallery) overlayDemo() ui.Widget {
	backdrop := must(g.window.CreateImage(ui.ImageOptions{Source: gradientImage(320, 160), Scaling: ui.ImageScaleFill}))
	label := func(text string) ui.Widget {
		result := g.label(text)
		g.attempt("Style", func() error {
			return result.SetStyle(ui.WidgetStyle{
				Background:   ui.SemanticColorSurface,
				CornerRadius: 4,
			})
		})
		return result
	}
	badge := g.optional(func() ui.Widget {
		return must(g.window.CreateBadge(ui.BadgeOptions{Text: "Live", Severity: ui.SeverityError}))
	})
	overlay := must(g.window.CreateOverlayLayout(ui.OverlayLayoutOptions{
		Items: []ui.OverlayItem{
			{Content: backdrop, Anchor: ui.AnchorCenter},
			{Content: label("Top left"), Anchor: ui.AnchorTopLeft, Offset: ui.Point{X: 8, Y: 8}},
			{Content: badge, Anchor: ui.AnchorTopRight, Offset: ui.Point{X: -8, Y: 8}},
			{Content: label("Centered"), Anchor: ui.AnchorCenter},
			{Content: label("Bottom right"), Anchor: ui.AnchorBottomRight, Offset: ui.Point{X: -8, Y: -8}},
		},
	}))
	overlay.SetMinimumSize(ui.Size{Height: 180})
	return overlay
}

func (g *gallery) formDemo() ui.Widget {
	name := must(g.window.CreateTextField())
	name.SetPlaceholder("Ada Lovelace")
	email := must(g.window.CreateTextField())
	email.SetPlaceholder("ada@example.com")
	age := must(g.window.CreateNumberField(ui.NumberFieldOptions{Minimum: 0, Maximum: 150, Value: 36, Step: 1}))
	plan := must(g.window.CreateSelect(ui.SelectOptions{Items: []string{"Free", "Pro", "Team"}, SelectedIndex: 0}))
	newsletter := must(g.window.CreateSwitch(ui.SwitchOptions{Text: "Send me the newsletter"}))

	form := must(g.window.CreateFormLayout(ui.FormLayoutOptions{Rows: []ui.FormRow{
		{ID: "name", Label: "Name", Editor: name, Required: true},
		{ID: "email", Label: "E-mail", Editor: email, Required: true, Help: "We never share your address."},
		{ID: "age", Label: "Age", Editor: age},
		{ID: "plan", Label: "Plan", Editor: plan, Help: "You can change your plan later."},
		{ID: "newsletter", Label: "Newsletter", Editor: newsletter},
	}}))
	validate := g.button("Validate", func() {
		messages := map[string]string{"name": "", "email": ""}
		if name.Text() == "" {
			messages["name"] = "Enter your name."
		}
		if email.Text() == "" {
			messages["email"] = "Enter your e-mail address."
		}
		for id, message := range messages {
			if err := form.SetError(id, message); err != nil {
				g.log("Form: %v", err)
			}
		}
		g.log("Form validated")
	})
	return g.column(form, g.row(validate, g.enabledToggle(name, email, age, plan, newsletter)))
}

func (g *gallery) separatorsDemo() ui.Widget {
	full := must(g.window.CreateSeparator(ui.Horizontal))
	inset := must(g.window.CreateSeparator(ui.Horizontal))
	g.attempt("Inset separator", func() error {
		inset.SetPresentation(ui.SeparatorInset)
		return nil
	})
	labeled := must(g.window.CreateSeparator(ui.Horizontal))
	g.attempt("Separator label", func() error {
		labeled.SetLabel("Advanced")
		return nil
	})
	thick := must(g.window.CreateSeparator(ui.Horizontal))
	g.attempt("Separator thickness", func() error {
		thick.SetThickness(4)
		return nil
	})
	vertical := must(g.window.CreateSeparator(ui.Vertical))
	columns := g.layout(ui.Row(
		ui.Item(g.label("Left")).Align(ui.AlignCenter),
		ui.Item(vertical).MinimumSize(ui.Size{Width: 1, Height: 32}),
		ui.Item(g.label("Right")).Align(ui.AlignCenter),
		ui.Space(),
	).WithGap(12))
	return g.column(
		g.caption("Full"), full,
		g.caption("Inset"), inset,
		g.caption("Labeled"), labeled,
		g.caption("Four pixels thick"), thick,
		columns,
	)
}

func (g *gallery) marginsDemo() ui.Widget {
	plain := g.button("No margins", func() {
		g.log("No margins clicked")
	})
	spaced := g.button("Margins 24 left and right", func() {
		g.log("Margins clicked")
	})
	spaced.SetMargins(ui.Insets{Left: 24, Right: 24})
	margined := g.layout(ui.Row(ui.Item(plain), ui.Item(spaced), ui.Space()))

	bounded := g.button("Grows to at most 300 pixels", func() {
		g.log("Bounded button clicked")
	})
	bounded.SetMinimumSize(ui.Size{Width: 160})
	bounded.SetMaximumSize(ui.Size{Width: 300})
	growing := g.layout(ui.Row(ui.Item(bounded).Grow(1)))

	report := g.button("Report bounds", func() {
		g.log("Bounded button: %+v, preferred %+v", bounded.Bounds(), bounded.PreferredSize())
	})
	return g.column(margined, growing, g.row(report))
}
