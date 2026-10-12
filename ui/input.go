package ui

import (
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

// KeyEvent is one physical-key transition. Scancode is platform-specific and
// is useful for keys without a portable identity.
type KeyEvent struct {
	Key       keyboard.Key
	Scancode  int
	Action    keyboard.KeyAction
	Modifiers keyboard.ModifierKey
}

// PointerButtonEvent is one pointing-device button transition.
type PointerButtonEvent struct {
	Button    pointer.Button
	Action    pointer.ButtonAction
	Modifiers keyboard.ModifierKey
}

// ScrollEvent is one high-resolution scroll event.
type ScrollEvent struct {
	X float64
	Y float64
}

// KeyHandler receives physical-key transitions.
type KeyHandler func(event KeyEvent)

// TextHandler receives text after the operating system applies the active
// keyboard layout, modifiers, and dead keys.
type TextHandler func(char rune)

// PointerButtonHandler receives pointing-device button transitions.
type PointerButtonHandler func(event PointerButtonEvent)

// ScrollHandler receives high-resolution scrolling on both axes.
type ScrollHandler func(event ScrollEvent)

// Unsubscribe removes an event handler. It is safe to call more than once.
type Unsubscribe func()

// PointerInput provides polling and event callbacks for a pointing device over
// one surface: a whole window, or a single canvas within it. PointerPosition is
// in logical pixels from the surface's top-left corner. ScrollOffset is a
// running total, so several consumers can read it without consuming one
// another's input.
//
// A canvas sees only what the platform delivers to it: a press that begins over
// it, until that button is released, and scrolling while the pointer is over it.
// A press that begins on another control is not the canvas's, even if the
// pointer is then dragged across it. The owning window also sees canvas events;
// its handlers run before the canvas handlers.
type PointerInput interface {
	PointerButtonState(button pointer.Button) pointer.ButtonAction
	PointerPosition() (x, y float64)
	ScrollOffset() (x, y float64)

	OnPointerButton(handler PointerButtonHandler) Unsubscribe
	OnScroll(handler ScrollHandler) Unsubscribe
}

// Input provides polling and event callbacks for keyboard and pointing
// devices across a whole window.
type Input interface {
	PointerInput

	KeyState(key keyboard.Key) keyboard.KeyAction

	PointerMode() pointer.Mode
	SetPointerMode(mode pointer.Mode)

	OnKey(handler KeyHandler) Unsubscribe
	OnText(handler TextHandler) Unsubscribe
}
