package pointerstate_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/internal/pointerstate"
)

func TestStateDispatchesPointerInput(t *testing.T) {
	var state pointerstate.State
	var buttons []ui.PointerButtonEvent
	var scrolling []ui.ScrollEvent
	removeButton := state.OnPointerButton(func(event ui.PointerButtonEvent) {
		buttons = append(buttons, event)
	})
	removeScroll := state.OnScroll(func(event ui.ScrollEvent) {
		scrolling = append(scrolling, event)
	})

	buttonEvent := ui.PointerButtonEvent{Button: pointer.ButtonLeft, Action: pointer.Pressed}
	state.DispatchPointerButton(buttonEvent)
	state.DispatchScroll(ui.ScrollEvent{X: 1.5, Y: -2})
	state.DispatchScroll(ui.ScrollEvent{X: 0.5, Y: 3})

	if state.PointerButtonState(pointer.ButtonLeft) != pointer.Pressed {
		t.Fatal("PointerButtonState(ButtonLeft) = Released, want Pressed")
	}
	if x, y := state.ScrollOffset(); x != 2 || y != 1 {
		t.Fatalf("ScrollOffset() = (%v, %v), want (2, 1)", x, y)
	}
	if len(buttons) != 1 || buttons[0] != buttonEvent {
		t.Fatalf("button events = %#v, want %#v", buttons, []ui.PointerButtonEvent{buttonEvent})
	}
	if len(scrolling) != 2 {
		t.Fatalf("scroll events = %#v, want two events", scrolling)
	}

	removeButton()
	removeButton()
	removeScroll()
	removeScroll()
	state.DispatchPointerButton(ui.PointerButtonEvent{Button: pointer.ButtonLeft, Action: pointer.Released})
	state.DispatchScroll(ui.ScrollEvent{Y: 1})
	if len(buttons) != 1 || len(scrolling) != 2 {
		t.Fatal("unsubscribed handlers received events")
	}
}

func TestStateReset(t *testing.T) {
	var state pointerstate.State
	called := false
	state.OnPointerButton(func(event ui.PointerButtonEvent) {
		called = true
	})
	state.DispatchPointerButton(ui.PointerButtonEvent{Button: pointer.ButtonRight, Action: pointer.Pressed})
	state.DispatchScroll(ui.ScrollEvent{X: 4, Y: 5})
	called = false

	state.Reset()

	if state.PointerButtonState(pointer.ButtonRight) != pointer.Released {
		t.Fatal("PointerButtonState(ButtonRight) = Pressed after Reset")
	}
	if x, y := state.ScrollOffset(); x != 0 || y != 0 {
		t.Fatalf("ScrollOffset() = (%v, %v) after Reset, want (0, 0)", x, y)
	}
	state.DispatchPointerButton(ui.PointerButtonEvent{Button: pointer.ButtonRight, Action: pointer.Released})
	if called {
		t.Fatal("Reset left a pointer handler registered")
	}
}
