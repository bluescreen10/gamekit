package main

import (
	"strings"

	"github.com/bluescreen10/gamekit/ui"
)

func toggleCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Check boxes", "The parent box shows the mixed state when only some children are checked.", g.checkBoxesDemo),
		g.card("Check box presentation", "A label before the control, and a description below the label.", g.checkBoxPresentationDemo),
		g.card("Switches", "Every control size, and on and off labels.", g.switchesDemo),
		g.card("Radio groups", "Choices with descriptions and a disabled choice. Change the orientation.", g.radioGroupsDemo),
		g.card("Segmented controls", "Single selection with icons, multiple selection, and a disabled segment.", g.segmentedControlsDemo),
	}
}

func (g *gallery) checkBoxesDemo() ui.Widget {
	parent := must(g.window.CreateCheckBox("All toppings", false))
	g.attempt("Mixed state", func() error {
		parent.SetAllowsMixedState(true)
		return nil
	})
	names := []string{"Cheese", "Mushrooms", "Olives"}
	children := make([]ui.CheckBox, len(names))
	updating := false
	updateParent := func() {
		checked := 0
		for _, child := range children {
			if child.IsChecked() {
				checked++
			}
		}
		state := ui.CheckMixed
		switch checked {
		case 0:
			state = ui.CheckOff
		case len(children):
			state = ui.CheckOn
		}
		updating = true
		g.attempt("Check state", func() error {
			parent.SetState(state)
			return nil
		})
		updating = false
	}
	childWidgets := make([]ui.Widget, len(names))
	for i, name := range names {
		child := g.checkBox(name, i == 0, func(checked bool) {
			g.log("%s: %t", name, checked)
			updateParent()
		})
		child.SetMargins(ui.Insets{Left: 22})
		children[i] = child
		childWidgets[i] = child
	}
	parent.OnChange(func(checked bool) {
		if updating {
			return
		}
		for _, child := range children {
			child.SetChecked(checked)
		}
		g.log("All toppings: %t", checked)
	})
	updateParent()

	items := append([]ui.Widget{parent}, childWidgets...)
	return g.column(append(items, g.states(items...))...)
}

func (g *gallery) checkBoxPresentationDemo() ui.Widget {
	before := g.checkBox("Label before the box", true, func(checked bool) {
		g.log("Label before: %t", checked)
	})
	g.attempt("Label placement", func() error {
		before.SetLabelPlacement(ui.LabelBeforeControl)
		return nil
	})
	described := g.checkBox("Send usage statistics", false, func(checked bool) {
		g.log("Usage statistics: %t", checked)
	})
	g.attempt("Description", func() error {
		described.SetDescription("Helps us improve GameKit. No personal data is sent.")
		return nil
	})
	disabledOn := must(g.window.CreateCheckBox("Disabled and checked", true))
	disabledOn.SetEnabled(false)
	disabledOff := must(g.window.CreateCheckBox("Disabled and unchecked", false))
	disabledOff.SetEnabled(false)
	return g.column(before, described, g.row(disabledOn, disabledOff))
}

func (g *gallery) switchesDemo() ui.Widget {
	sizes := []struct {
		text string
		size ui.ControlSize
	}{
		{"Mini", ui.ControlSizeMini},
		{"Small", ui.ControlSizeSmall},
		{"Regular", ui.ControlSizeRegular},
		{"Large", ui.ControlSizeLarge},
	}
	switches := make([]ui.Widget, 0, len(sizes)+1)
	for i, entry := range sizes {
		control := g.optional(func() ui.Widget {
			control := must(g.window.CreateSwitch(ui.SwitchOptions{
				Text:    entry.text,
				Checked: i%2 == 0,
				Size:    entry.size,
			}))
			control.OnChange(func(checked bool) {
				g.log("%s switch: %t", entry.text, checked)
			})
			return control
		})
		switches = append(switches, control)
	}
	labeled := must(g.window.CreateSwitch(ui.SwitchOptions{
		Text:     "Wi-Fi",
		OnLabel:  "On",
		OffLabel: "Off",
		Checked:  true,
	}))
	labeled.OnChange(func(checked bool) {
		g.log("Wi-Fi: %t", checked)
	})
	return g.column(
		g.row(switches...),
		g.labeled("With state labels", labeled),
		g.states(append(switches, labeled)...),
	)
}

func (g *gallery) radioGroupsDemo() ui.Widget {
	simple := must(g.window.CreateRadioGroup(ui.RadioGroupOptions{
		Items:         []string{"Wireframe", "Shaded", "Rendered"},
		SelectedIndex: 1,
	}))
	g.attempt("Radio label", func() error {
		simple.SetLabel("Viewport shading")
		return nil
	})
	simple.OnChange(func(index int) {
		g.log("Shading: %s", simple.Items()[index])
	})

	detailed := must(g.window.CreateRadioGroup(ui.RadioGroupOptions{
		Items:         []string{"Low", "Medium", "High"},
		SelectedIndex: 0,
	}))
	g.attempt("Radio choices", func() error {
		return detailed.SetChoices([]ui.Choice{
			{ID: "low", Text: "Low", Description: "Fast, for older hardware.", Enabled: true},
			{ID: "medium", Text: "Medium", Description: "Balanced quality and speed.", Enabled: true},
			{ID: "ultra", Text: "Ultra", Description: "Needs a discrete GPU.", Enabled: false},
		})
	})
	detailed.OnChange(func(index int) {
		g.log("Quality: %s", detailed.Items()[index])
	})

	orientation := g.choice([]string{"Vertical", "Horizontal"}, 0, func(index int) {
		direction := ui.Vertical
		if index == 1 {
			direction = ui.Horizontal
		}
		simple.SetOrientation(direction)
		detailed.SetOrientation(direction)
		g.log("Radio orientation: %s", []string{"vertical", "horizontal"}[index])
	})
	g.attempt("Radio orientation", func() error {
		simple.SetOrientation(ui.Vertical)
		detailed.SetOrientation(ui.Vertical)
		return nil
	})
	return g.column(
		g.layout(ui.Row(ui.Item(simple), ui.Item(detailed), ui.Space()).WithGap(32)),
		g.row(g.label("Orientation"), orientation, g.enabledToggle(simple, detailed)),
	)
}

func (g *gallery) segmentedControlsDemo() ui.Widget {
	single := must(g.window.CreateSegmentedControl(ui.SegmentedControlOptions{
		Items: []ui.Segment{
			{ID: "list", Text: "List", Icon: ui.IconSource{SystemName: "menu"}, Enabled: true},
			{ID: "grid", Text: "Grid", Icon: ui.IconSource{SystemName: "image"}, Enabled: true},
			{ID: "columns", Text: "Columns", Icon: ui.IconSource{SystemName: "document"}, Enabled: true},
		},
		SelectedIDs: []string{"list"},
		Mode:        ui.SelectionSingle,
	}))
	single.OnChange(func(ids []string) {
		g.log("View mode: %s", strings.Join(ids, ", "))
	})

	multiple := must(g.window.CreateSegmentedControl(ui.SegmentedControlOptions{
		Items: []ui.Segment{
			{ID: "bold", Text: "Bold", Enabled: true},
			{ID: "italic", Text: "Italic", Enabled: true},
			{ID: "underline", Text: "Underline", Enabled: true},
			{ID: "strike", Text: "Strike", Enabled: false},
		},
		SelectedIDs: []string{"bold", "underline"},
		Mode:        ui.SelectionMultiple,
	}))
	multiple.OnChange(func(ids []string) {
		g.log("Text styles: [%s]", strings.Join(ids, ", "))
	})
	reset := g.button("Select italic only", func() {
		if err := multiple.SetSelectedIDs([]string{"italic"}); err != nil {
			g.log("Select italic: %v", err)
		}
	})
	return g.column(
		g.labeled("Single", single),
		g.labeled("Multiple", multiple, reset),
		g.states(single, multiple),
	)
}
