package main

import (
	"fmt"

	"github.com/bluescreen10/gamekit/ui"
)

// closer is an object that a build step creates and must release when the
// step fails.
type closer interface {
	Close()
}

// building collects the objects that the current build step created, so that
// buildSafely can release them when the step fails. buildDepth counts the
// build steps that run.
var (
	building   []closer
	buildDepth int
)

// must returns value, or panics with err. The gallery builds its cards with
// must, and buildSafely turns the panic into a message in place of the card
// content, so that one component that a backend does not support does not
// stop the others.
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	if object, ok := any(value).(closer); ok && buildDepth > 0 {
		building = append(building, object)
	}
	return value
}

// buildSafely runs build. If build panics, buildSafely closes the objects
// that build created and returns a label that shows the reason.
func (g *gallery) buildSafely(build func() ui.Widget) (result ui.Widget) {
	outer := building
	building = nil
	buildDepth++
	defer func() {
		recovered := recover()
		created := building
		building = outer
		buildDepth--
		if recovered == nil {
			if buildDepth > 0 {
				building = append(building, created...)
			}
			return
		}
		for i := len(created) - 1; i >= 0; i-- {
			created[i].Close()
		}
		result = g.unavailable(recovered)
	}()
	return build()
}

// unavailable returns a label that tells why a component could not be shown.
func (g *gallery) unavailable(reason any) ui.Widget {
	text := fmt.Sprintf("Not available on this backend: %v", reason)
	g.unavailableCount++
	g.log("%s", text)
	label := must(g.window.CreateLabel(text))
	label.SetWrapMode(ui.WrapWords)
	g.attempt("Style", func() error {
		return label.SetStyle(ui.WidgetStyle{Foreground: ui.SemanticColorWarning})
	})
	return label
}

// card puts the result of build in a group box with a title and a short
// description of what to try.
func (g *gallery) card(title, detail string, build func() ui.Widget) ui.Widget {
	content := g.buildSafely(build)
	return must(g.window.CreateGroupBox(ui.GroupBoxOptions{
		Title:    title,
		Subtitle: detail,
		Content:  g.layout(ui.Column(ui.Item(content)).WithPadding(ui.UniformInsets(8))),
	}))
}

// optional returns the result of build, or a label with the reason when the
// backend does not support it.
func (g *gallery) optional(build func() ui.Widget) ui.Widget {
	return g.buildSafely(build)
}

func (g *gallery) layout(spec ui.LayoutSpec) ui.Layout {
	return must(g.window.CreateLayout(spec))
}

// row places widgets from the start of a row, centered vertically.
func (g *gallery) row(widgets ...ui.Widget) ui.Layout {
	items := make([]ui.LayoutItem, 0, len(widgets)+1)
	for _, widget := range widgets {
		items = append(items, ui.Item(widget).Align(ui.AlignCenter))
	}
	items = append(items, ui.Space())
	return g.layout(ui.Row(items...).WithGap(8))
}

// column stacks widgets and stretches them to its width.
func (g *gallery) column(widgets ...ui.Widget) ui.Layout {
	items := make([]ui.LayoutItem, len(widgets))
	for i, widget := range widgets {
		items[i] = ui.Item(widget)
	}
	return g.layout(ui.Column(items...).WithGap(10))
}

// labeled places a fixed-width label before widgets.
func (g *gallery) labeled(text string, widgets ...ui.Widget) ui.Layout {
	label := g.label(text)
	items := []ui.LayoutItem{ui.Item(label).Align(ui.AlignCenter).MinimumSize(ui.Size{Width: 150})}
	for _, widget := range widgets {
		items = append(items, ui.Item(widget).Align(ui.AlignCenter))
	}
	items = append(items, ui.Space())
	return g.layout(ui.Row(items...).WithGap(8))
}

// labeledWide places a fixed-width label before a widget that fills the rest
// of the row.
func (g *gallery) labeledWide(text string, widget ui.Widget) ui.Layout {
	label := g.label(text)
	return g.layout(ui.Row(
		ui.Item(label).Align(ui.AlignCenter).MinimumSize(ui.Size{Width: 150}),
		ui.Item(widget).Grow(1).Align(ui.AlignCenter),
	).WithGap(8))
}

func (g *gallery) label(text string) ui.Label {
	return must(g.window.CreateLabel(text))
}

// caption returns a wrapping label in the caption style.
func (g *gallery) caption(text string) ui.Label {
	label := g.label(text)
	label.SetSemanticStyle(ui.TextStyleCaption)
	label.SetWrapMode(ui.WrapWords)
	return label
}

// value returns a label for a value that changes, in the monospace style.
func (g *gallery) value(text string) ui.Label {
	label := g.label(text)
	label.SetSemanticStyle(ui.TextStyleMonospace)
	return label
}

func (g *gallery) button(text string, click func()) ui.Button {
	button := must(g.window.CreateButton(text))
	button.OnClick(func() {
		g.attempt(text, func() error {
			click()
			return nil
		})
	})
	return button
}

func (g *gallery) checkBox(text string, checked bool, change func(checked bool)) ui.CheckBox {
	check := must(g.window.CreateCheckBox(text, checked))
	check.OnChange(func(checked bool) {
		g.attempt(text, func() error {
			change(checked)
			return nil
		})
	})
	return check
}

// choice returns a pop-up select that calls change with the selected index.
func (g *gallery) choice(items []string, selected int, change func(index int)) ui.Select {
	control := must(g.window.CreateSelect(ui.SelectOptions{Items: items, SelectedIndex: selected}))
	control.OnChange(func(index int) {
		g.attempt(items[index], func() error {
			change(index)
			return nil
		})
	})
	return control
}

// enabledToggle returns a check box that enables or disables widgets.
func (g *gallery) enabledToggle(widgets ...ui.Widget) ui.CheckBox {
	return g.checkBox("Enabled", true, func(enabled bool) {
		for _, widget := range widgets {
			widget.SetEnabled(enabled)
		}
		g.log("Enabled: %t", enabled)
	})
}

// visibleToggle returns a check box that shows or hides widgets.
func (g *gallery) visibleToggle(widgets ...ui.Widget) ui.CheckBox {
	return g.checkBox("Visible", true, func(visible bool) {
		for _, widget := range widgets {
			widget.SetVisible(visible)
		}
		g.log("Visible: %t", visible)
	})
}

// states returns a row of check boxes that enable and show widgets.
func (g *gallery) states(widgets ...ui.Widget) ui.Layout {
	return g.row(g.enabledToggle(widgets...), g.visibleToggle(widgets...))
}

// fixed limits widget to size and returns it.
func fixed[T ui.Widget](widget T, size ui.Size) T {
	widget.SetMinimumSize(size)
	widget.SetMaximumSize(size)
	return widget
}

// tall gives widget a minimum height and returns it.
func tall[T ui.Widget](widget T, height int) T {
	widget.SetMinimumSize(ui.Size{Height: height})
	return widget
}
