//go:build windows

package win32

import (
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

// Event callbacks, in the GLFW shape: register one per window, and it is invoked from
// PollEvents on the goroutine that called it. A nil callback removes the previous one.
//
// Callbacks report things polling cannot: the characters a keystroke actually produced
// (which depend on layout and dead keys), auto-repeat at the rate the OS is configured
// for, scroll deltas, and events that begin and end inside a single frame.
type (
	// KeyCallback receives physical key transitions. scancode is the raw
	// platform-specific code, for keys with no portable identity.
	KeyCallback func(w *Window, key keyboard.Key, scancode int, action keyboard.KeyAction, mods keyboard.ModifierKey)

	// CharCallback receives text, already mapped through the user's keyboard layout.
	// This cannot be reconstructed from KeyCallback: layout, shift states and dead
	// keys all sit between a keystroke and the character it produces.
	CharCallback func(w *Window, char rune)

	// ScrollCallback receives scroll deltas. A trackpad reports fractional values and
	// both axes; a wheel mouse typically reports whole steps on y only.
	ScrollCallback func(w *Window, x, y float64)

	// PointerButtonCallback receives pointing-device button transitions.
	PointerButtonCallback func(w *Window, button pointer.Button, action pointer.ButtonAction, mods keyboard.ModifierKey)
)

// SetKeyCallback registers cb for physical key transitions, replacing any previous
// callback, and returns the one it replaced.
func (w *Window) SetKeyCallback(cb KeyCallback) KeyCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.keyCallback
	w.keyCallback = cb
	return previous
}

// SetCharCallback registers cb for typed text, replacing any previous callback, and
// returns the one it replaced.
func (w *Window) SetCharCallback(cb CharCallback) CharCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.charCallback
	w.charCallback = cb
	return previous
}

// SetScrollCallback registers cb for scroll deltas, replacing any previous callback,
// and returns the one it replaced.
func (w *Window) SetScrollCallback(cb ScrollCallback) ScrollCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.scrollCallback
	w.scrollCallback = cb
	return previous
}

// SetPointerButtonCallback registers cb for pointing-device buttons, replacing any
// previous callback, and returns the one it replaced.
func (w *Window) SetPointerButtonCallback(cb PointerButtonCallback) PointerButtonCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.pointerButtonCallback
	w.pointerButtonCallback = cb
	return previous
}

// The native window calls these for every event, whether or not a callback is
// registered. Key and button dispatch update the polling state of the window
// first. Every path releases the lock before it calls user code, so callbacks
// can call back into the window.

// dispatchNativeKey reports a key that the native window received.
func (w *Window) dispatchNativeKey(key, scancode, action, mods int) {
	w.dispatchKey(keyboard.Key(key), scancode, keyboard.KeyAction(action), keyboard.ModifierKey(mods))
}

// dispatchNativeButton reports a button of the pointer that the native window
// received.
func (w *Window) dispatchNativeButton(button, action, mods int) {
	w.dispatchPointerButton(pointer.Button(button), pointer.ButtonAction(action), keyboard.ModifierKey(mods))
}

// dispatchChar reports a character that the keyboard layout produced.
func (w *Window) dispatchChar(char rune) {
	w.mu.RLock()
	cb := w.charCallback
	w.mu.RUnlock()
	if cb != nil {
		cb(w, char)
	}
}

// dispatchScroll reports scrolling.
func (w *Window) dispatchScroll(x, y float64) {
	w.mu.RLock()
	cb := w.scrollCallback
	w.mu.RUnlock()
	if cb != nil {
		cb(w, x, y)
	}
}

// dispatchNativeMove reports a new position of the native window.
func (w *Window) dispatchNativeMove(x, y int) {
	w.dispatchMove(ui.Point{X: x, Y: y})
}
