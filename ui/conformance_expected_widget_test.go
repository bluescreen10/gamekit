package ui_test

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/bluescreen10/gamekit/ui"
)

func checkExpectedWidgetContract(r *recorder, window ui.Window) {
	label, err := window.CreateLabel("Account")
	r.check("widget/complete/create_label", func() error {
		return err
	})
	if label == nil {
		return
	}
	defer label.Close()

	field, err := window.CreateTextField()
	r.check("widget/complete/create_field", func() error {
		return err
	})
	if field == nil {
		return
	}
	defer field.Close()

	checkExpectedAccessibility(r, label, field)
	checkExpectedFocus(r, window, field)
	checkExpectedWidgetGeometry(r, label)
	checkExpectedWidgetPresentation(r, label)
	checkExpectedWidgetInteraction(r, window, label)
	checkExpectedAccessibilityTree(r, window, label, field)
}

func checkExpectedAccessibility(r *recorder, label ui.Label, field ui.TextField) {
	r.check("widget/accessibility/metadata", func() error {
		want := ui.Accessibility{
			ID:          "account-name",
			Name:        "Account name",
			Description: "The name shown to other players",
			ValueText:   "Empty",
			LabeledBy:   []ui.Widget{label},
			DescribedBy: []ui.Widget{label},
			IsRequired:  true,
			IsInvalid:   true,
			Live:        ui.LivePolite,
		}
		if err := field.SetAccessibility(want); err != nil {
			return err
		}
		if got := field.Accessibility(); !reflect.DeepEqual(got, want) {
			return fmt.Errorf("Accessibility() = %#v, want %#v", got, want)
		}
		return nil
	})

	r.check("widget/accessibility/hidden", func() error {
		metadata := label.Accessibility()
		metadata.IsHidden = true
		if err := label.SetAccessibility(metadata); err != nil {
			return err
		}
		if !label.Accessibility().IsHidden {
			return errors.New("accessibility-hidden state was not preserved")
		}
		return nil
	})
}

func checkExpectedFocus(r *recorder, window ui.Window, widget ui.Widget) {
	r.check("widget/focus/subscriptions", func() error {
		focus := widget.OnFocus(func() {
		})
		blur := widget.OnBlur(func() {
		})
		if focus == nil || blur == nil {
			return errors.New("focus subscription returned nil")
		}
		focus()
		focus()
		blur()
		blur()
		return nil
	})

	r.check("widget/focus/request", func() error {
		window.SetVisible(true)
		defer window.SetVisible(false)
		if err := widget.Focus(); err != nil {
			return err
		}
		window.PollEvents()
		if !widget.IsFocused() {
			return errors.New("Focus did not focus the widget")
		}
		return nil
	})
}

func checkExpectedWidgetGeometry(r *recorder, widget ui.Widget) {
	r.check("widget/geometry/constraints", func() error {
		minimum := ui.Size{Width: 80, Height: 24}
		maximum := ui.Size{Width: 400, Height: 96}
		widget.SetMinimumSize(minimum)
		widget.SetMaximumSize(maximum)
		if widget.MinimumSize() != minimum || widget.MaximumSize() != maximum {
			return errors.New("widget size constraints were not preserved")
		}

		margins := ui.Insets{Top: 1, Right: 2, Bottom: 3, Left: 4}
		widget.SetMargins(margins)
		if widget.Margins() != margins {
			return fmt.Errorf("Margins() = %#v, want %#v", widget.Margins(), margins)
		}
		return nil
	})

	r.check("widget/geometry/resize_event", func() error {
		var sizes []ui.Size
		unsubscribe := widget.OnResize(func(size ui.Size) {
			sizes = append(sizes, size)
		})
		widget.SetBounds(ui.Rect{Size: ui.Size{Width: 180, Height: 40}})
		unsubscribe()
		if len(sizes) != 1 || sizes[0] != (ui.Size{Width: 180, Height: 40}) {
			return fmt.Errorf("resize events = %#v", sizes)
		}
		return nil
	})
}

func checkExpectedWidgetPresentation(r *recorder, widget ui.Widget) {
	r.check("widget/presentation/tooltip", func() error {
		widget.SetTooltip("More information")
		return wantEqual(widget.Tooltip(), "More information")
	})
	r.check("widget/presentation/cursor", func() error {
		if err := widget.SetCursor(ui.CursorPointer); err != nil {
			return err
		}
		return wantEqual(widget.Cursor(), ui.CursorPointer)
	})
	r.check("widget/presentation/style", func() error {
		want := ui.WidgetStyle{
			Font:       ui.Font{Family: "System", Size: 14},
			Foreground: ui.SemanticColorPrimary,
			Background: ui.SemanticColorSurface,
		}
		if err := widget.SetStyle(want); err != nil {
			return err
		}
		if got := widget.Style(); got != want {
			return fmt.Errorf("Style() = %#v, want %#v", got, want)
		}
		return nil
	})
}

func checkExpectedWidgetInteraction(r *recorder, window ui.Window, widget ui.Widget) {
	r.check("widget/interaction/context_menu", func() error {
		menu, err := window.CreateContextMenu(ui.ContextMenuOptions{
			Actions: []ui.Action{{ID: "copy", Text: "Copy", Enabled: true}},
			Items:   []ui.MenuItem{{ActionID: "copy"}},
		})
		if err != nil {
			return err
		}
		defer menu.Close()
		return widget.SetContextMenu(menu)
	})

	r.check("widget/interaction/drag_drop", func() error {
		if err := widget.SetDragSource(ui.DragSource{
			Operations: ui.DragCopy,
			Text:       "GameKit",
		}); err != nil {
			return err
		}
		if err := widget.SetDropTarget(ui.DropTarget{
			Operations: ui.DragCopy | ui.DragMove,
			Formats:    []string{"text/plain"},
		}); err != nil {
			return err
		}
		unsubscribe := widget.OnDrop(func(ui.DropEvent) {
		})
		if unsubscribe == nil {
			return errors.New("drop subscription returned nil")
		}
		unsubscribe()
		return nil
	})
}

func checkExpectedAccessibilityTree(r *recorder, window ui.Window, label ui.Label, field ui.TextField) {
	layout, err := window.CreateLayout(ui.Column(ui.Item(label), ui.Item(field)))
	r.check("accessibility/tree/create_content", func() error {
		return err
	})
	if layout == nil {
		return
	}
	defer layout.Close()

	r.check("accessibility/tree", func() error {
		if err := window.SetContent(layout); err != nil {
			return err
		}
		tree := window.AccessibilityTree()
		if tree.Role != ui.RoleWindow {
			return fmt.Errorf("root accessibility role = %v", tree.Role)
		}
		if tree.FindByID("account-name") == nil {
			return errors.New("accessibility tree cannot find the text field")
		}
		return nil
	})
}
