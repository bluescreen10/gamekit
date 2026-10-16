package main

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/bluescreen10/gamekit/ui"
)

func behaviorCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Semantic colors", "Text colors that follow the theme of the system.", g.semanticColorsDemo),
		g.card("Custom styles", "Fonts, explicit colors, and rounded corners.", g.customStylesDemo),
		g.card("Cursors", "Move the pointer over each label.", g.cursorsDemo),
		g.card("Tooltips", "Hover over the controls.", g.tooltipsDemo),
		g.card("Focus", "Focus a field from code, and watch the focus and blur events.", g.focusDemo),
		g.card("Drag and drop", "Drag the source onto the target, or drop text or files from other applications.", g.dragDropDemo),
		g.card("Accessibility", "Metadata for assistive technologies, and the accessibility tree of the window.", g.accessibilityDemo),
	}
}

func (g *gallery) semanticColorsDemo() ui.Widget {
	colors := []struct {
		name  string
		color ui.SemanticColor
	}{
		{"Primary", ui.SemanticColorPrimary},
		{"Secondary", ui.SemanticColorSecondary},
		{"Tertiary", ui.SemanticColorTertiary},
		{"Accent", ui.SemanticColorAccent},
		{"Success", ui.SemanticColorSuccess},
		{"Warning", ui.SemanticColorWarning},
		{"Error", ui.SemanticColorError},
		{"Link", ui.SemanticColorLink},
	}
	labels := make([]ui.Widget, len(colors))
	for i, entry := range colors {
		label := g.label(entry.name)
		g.attempt("Style", func() error {
			return label.SetStyle(ui.WidgetStyle{Foreground: entry.color})
		})
		labels[i] = label
	}
	surface := g.label("  Surface background  ")
	g.attempt("Style", func() error {
		return surface.SetStyle(ui.WidgetStyle{Background: ui.SemanticColorSurface, CornerRadius: 4})
	})
	return g.column(g.row(labels...), g.row(surface))
}

func (g *gallery) customStylesDemo() ui.Widget {
	filled := g.button("Filled button", func() {
		g.log("Filled button clicked")
	})
	g.attempt("Filled style", func() error {
		return filled.SetStyle(ui.WidgetStyle{
			Font:            ui.Font{Size: 14, Weight: ui.FontWeightBold},
			ForegroundColor: color.White,
			BackgroundColor: color.RGBA{R: 0x2f, G: 0x80, B: 0xed, A: 0xff},
			CornerRadius:    8,
		})
	})
	danger := g.button("Danger", func() {
		g.log("Danger clicked")
	})
	g.attempt("Danger style", func() error {
		return danger.SetStyle(ui.WidgetStyle{
			ForegroundColor: color.White,
			BackgroundColor: color.RGBA{R: 0xd9, G: 0x3b, B: 0x3b, A: 0xff},
			CornerRadius:    16,
		})
	})
	serif := g.label("Large serif text")
	g.attempt("Serif style", func() error {
		return serif.SetStyle(ui.WidgetStyle{Font: ui.Font{Family: "Georgia", Size: 22, Italic: true}})
	})
	colored := g.label("Explicit foreground color")
	g.attempt("Color style", func() error {
		return colored.SetStyle(ui.WidgetStyle{ForegroundColor: color.RGBA{R: 0x8e, G: 0x44, B: 0xad, A: 0xff}})
	})
	report := g.button("Report style", func() {
		g.log("Filled button style: %+v", filled.Style())
	})
	return g.column(g.row(filled, danger, report), g.row(serif, colored))
}

func (g *gallery) cursorsDemo() ui.Widget {
	cursors := []struct {
		name   string
		cursor ui.Cursor
	}{
		{"Default", ui.CursorDefault}, {"Pointer", ui.CursorPointer}, {"Text", ui.CursorText},
		{"Crosshair", ui.CursorCrosshair}, {"Move", ui.CursorMove}, {"Resize ↔", ui.CursorResizeHorizontal},
		{"Resize ↕", ui.CursorResizeVertical}, {"Not allowed", ui.CursorNotAllowed}, {"Wait", ui.CursorWait},
		{"Grab", ui.CursorGrab}, {"Grabbing", ui.CursorGrabbing},
	}
	items := make([]ui.GridItem, len(cursors))
	for i, entry := range cursors {
		label := g.label(entry.name)
		label.SetTextAlignment(ui.TextAlignCenter)
		g.attempt("Cursor", func() error {
			return label.SetCursor(entry.cursor)
		})
		g.attempt("Style", func() error {
			return label.SetStyle(ui.WidgetStyle{Background: ui.SemanticColorSurface, CornerRadius: 4})
		})
		items[i] = ui.GridItem{Content: label, Column: i % 4, Row: i / 4}
	}
	return must(g.window.CreateGridLayout(ui.GridLayoutOptions{
		Columns:   []ui.GridTrack{{Grow: 1}, {Grow: 1}, {Grow: 1}, {Grow: 1}},
		Rows:      []ui.GridTrack{{Size: 36}, {Size: 36}, {Size: 36}},
		Items:     items,
		RowGap:    6,
		ColumnGap: 6,
	}))
}

func (g *gallery) tooltipsDemo() ui.Widget {
	button := g.button("Button", func() {
		g.log("Button clicked")
	})
	button.SetTooltip("Buttons show tooltips.")
	field := must(g.window.CreateTextField())
	field.SetTooltip("So do text fields.")
	check := must(g.window.CreateCheckBox("Check box", false))
	check.SetTooltip("And check boxes.")
	disabled := g.button("Disabled", func() {
		g.log("Disabled clicked")
	})
	disabled.SetEnabled(false)
	disabled.SetTooltip("Disabled controls keep their tooltips.")
	return g.row(button, fixed(field, ui.Size{Width: 160}), check, disabled)
}

func (g *gallery) focusDemo() ui.Widget {
	first := must(g.window.CreateTextField())
	first.SetPlaceholder("First")
	second := must(g.window.CreateTextField())
	second.SetPlaceholder("Second")
	state := g.value("")
	report := func() {
		state.SetText(fmt.Sprintf("first focused %t, second focused %t", first.IsFocused(), second.IsFocused()))
	}
	for i, field := range []ui.TextField{first, second} {
		name := []string{"First", "Second"}[i]
		field.OnFocus(func() {
			g.log("%s field focused", name)
			report()
		})
		field.OnBlur(func() {
			g.log("%s field lost focus", name)
			report()
		})
	}
	focus := func(field ui.TextField) func() {
		return func() {
			if err := field.Focus(); err != nil {
				g.log("Focus: %v", err)
			}
			report()
		}
	}
	report()
	return g.column(
		g.row(fixed(first, ui.Size{Width: 160}), fixed(second, ui.Size{Width: 160})),
		g.row(g.button("Focus first", focus(first)), g.button("Focus second", focus(second)), state),
	)
}

func (g *gallery) dragDropDemo() ui.Widget {
	source := g.label("Drag me: “GameKit”")
	source.SetMinimumSize(ui.Size{Width: 180, Height: 60})
	source.SetTextAlignment(ui.TextAlignCenter)
	g.attempt("Style", func() error {
		return source.SetStyle(ui.WidgetStyle{Background: ui.SemanticColorAccent, Foreground: ui.SemanticColorSurface, CornerRadius: 8})
	})
	g.attempt("Cursor", func() error {
		return source.SetCursor(ui.CursorGrab)
	})
	if err := source.SetDragSource(ui.DragSource{Operations: ui.DragCopy, Text: "GameKit"}); err != nil {
		panic(err)
	}

	imageSource := must(g.window.CreateImage(ui.ImageOptions{Source: swatchImage(48, 0.55), Scaling: ui.ImageScaleFit}))
	fixed(imageSource, ui.Size{Width: 60, Height: 60})
	g.attempt("Image drag", func() error {
		return imageSource.SetDragSource(ui.DragSource{Operations: ui.DragCopy, Image: swatchImage(48, 0.55)})
	})

	target := g.label("Drop here")
	target.SetMinimumSize(ui.Size{Width: 260, Height: 60})
	target.SetTextAlignment(ui.TextAlignCenter)
	target.SetWrapMode(ui.WrapCharacters)
	g.attempt("Style", func() error {
		return target.SetStyle(ui.WidgetStyle{Background: ui.SemanticColorSurface, CornerRadius: 8})
	})
	if err := target.SetDropTarget(ui.DropTarget{
		Operations: ui.DragCopy | ui.DragMove,
		Formats:    []string{"text/plain", "text/uri-list", "image/png"},
	}); err != nil {
		panic(err)
	}
	target.OnDrop(func(event ui.DropEvent) {
		parts := []string{}
		if event.Text != "" {
			parts = append(parts, fmt.Sprintf("text %q", event.Text))
		}
		if len(event.Files) > 0 {
			parts = append(parts, fmt.Sprintf("%d files", len(event.Files)))
		}
		if event.Image != nil {
			size := event.Image.Bounds().Size()
			parts = append(parts, fmt.Sprintf("image %d × %d", size.X, size.Y))
		}
		summary := strings.Join(parts, ", ")
		target.SetText("Dropped " + summary)
		g.log("Drop at %d, %d (operation %d): %s", event.Position.X, event.Position.Y, event.Operation, summary)
	})
	return g.row(source, imageSource, target)
}

func (g *gallery) accessibilityDemo() ui.Widget {
	play := g.button("▶", func() {
		g.log("Play clicked")
	})
	g.attempt("Accessibility", func() error {
		return play.SetAccessibility(ui.Accessibility{
			ID:          "gallery-play",
			Name:        "Play",
			Description: "Starts the preview of the level.",
		})
	})
	volume := must(g.window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 0.4}))
	volumeLabel := g.label("Volume")
	g.attempt("Accessibility", func() error {
		return volume.SetAccessibility(ui.Accessibility{
			ID:        "gallery-volume",
			LabeledBy: []ui.Widget{volumeLabel},
			ValueText: "40 percent",
		})
	})
	volume.OnChange(func(value float64) {
		g.attempt("Accessibility", func() error {
			metadata := volume.Accessibility()
			metadata.ValueText = fmt.Sprintf("%.0f percent", value*100)
			return volume.SetAccessibility(metadata)
		})
	})
	decorative := g.label("✦ ✦ ✦")
	g.attempt("Accessibility", func() error {
		return decorative.SetAccessibility(ui.Accessibility{IsHidden: true})
	})

	tree := must(g.window.CreateTextArea(ui.TextAreaOptions{ReadOnly: true}))
	tall(tree, 220)
	g.attempt("Style", func() error {
		return tree.SetStyle(ui.WidgetStyle{Font: ui.Font{Size: 11}})
	})
	dump := g.button("Show accessibility tree", func() {
		root := g.window.AccessibilityTree()
		lines := []string{}
		appendAccessibilityNode(&lines, root, 0)
		tree.SetText(strings.Join(lines, "\n"))
		if node := root.FindByID("gallery-play"); node != nil {
			g.log("Found %q with role %s", node.Name, node.Role)
		}
	})
	roles := g.caption(fmt.Sprintf("Roles: button %s, slider %s, label %s",
		play.AccessibilityRole(), volume.AccessibilityRole(), decorative.AccessibilityRole()))
	return g.column(
		g.row(play, volumeLabel, fixed(volume, ui.Size{Width: 160}), decorative, dump),
		roles,
		tree,
	)
}

// appendAccessibilityNode adds a line for node and each of its descendants.
func appendAccessibilityNode(lines *[]string, node ui.AccessibilityNode, depth int) {
	line := strings.Repeat("  ", depth) + node.Role.String()
	if node.Name != "" {
		line += fmt.Sprintf(" %q", node.Name)
	}
	if node.Value != "" {
		line += fmt.Sprintf(" = %q", node.Value)
	}
	if node.ID != "" {
		line += " #" + node.ID
	}
	if node.Hidden {
		line += " (hidden)"
	}
	*lines = append(*lines, line)
	for _, child := range node.Children {
		appendAccessibilityNode(lines, child, depth+1)
	}
}
