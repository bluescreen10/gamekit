//go:build windows

package win32

import (
	"sync"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

func (w *Window) connectInput() {
	w.SetKeyCallback(func(_ *Window, key keyboard.Key, scancode int, action keyboard.KeyAction, modifiers keyboard.ModifierKey) {
		w.dispatchKeyHandlers(ui.KeyEvent{
			Key:       key,
			Scancode:  scancode,
			Action:    action,
			Modifiers: modifiers,
			IsRepeat:  action == keyboard.KeyRepeat,
		})
	})
	w.SetCharCallback(func(_ *Window, text rune) {
		w.dispatchTextHandlers(text)
	})
	w.SetPointerButtonCallback(func(_ *Window, button pointer.Button, action pointer.ButtonAction, modifiers keyboard.ModifierKey) {
		w.dispatchPointerButtonHandlers(ui.PointerButtonEvent{
			Button:    button,
			Action:    action,
			Modifiers: modifiers,
		})
	})
	w.SetScrollCallback(func(_ *Window, x, y float64) {
		w.dispatchScrollHandlers(ui.ScrollEvent{X: x, Y: y})
	})
}

// KeyState returns the current state of key.
func (w *Window) KeyState(key keyboard.Key) keyboard.KeyAction {
	if w == nil {
		return keyboard.KeyReleased
	}
	return w.GetKey(key)
}

// PointerButtonState returns the current state of button.
func (w *Window) PointerButtonState(button pointer.Button) pointer.ButtonAction {
	if w == nil {
		return pointer.Released
	}
	return w.GetPointerButton(button)
}

// PointerPosition returns the pointer in logical pixels from the top-left of
// the window's content area.
func (w *Window) PointerPosition() (x, y float64) {
	if w == nil {
		return 0, 0
	}
	return w.GetPointerPos()
}

// ScrollOffset returns the scroll offsets accumulated since window creation.
func (w *Window) ScrollOffset() (x, y float64) {
	if w == nil {
		return 0, 0
	}
	return w.GetScroll()
}

// PointerMode returns the window's current pointer mode.
func (w *Window) PointerMode() pointer.Mode {
	if w == nil {
		return pointer.Normal
	}
	return w.GetPointerMode()
}

// OnKey registers a physical-key handler and returns a function that removes it.
func (w *Window) OnKey(handler ui.KeyHandler) ui.Unsubscribe {
	if w == nil || handler == nil {
		return noOpUnsubscribe
	}
	w.handlersMu.Lock()
	id := w.nextHandlerID()
	if w.keyHandlers == nil {
		w.keyHandlers = make(map[uint64]ui.KeyHandler)
	}
	w.keyHandlers[id] = handler
	w.handlersMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			w.handlersMu.Lock()
			delete(w.keyHandlers, id)
			w.handlersMu.Unlock()
		})
	}
}

// OnText registers a text-input handler and returns a function that removes it.
func (w *Window) OnText(handler ui.TextHandler) ui.Unsubscribe {
	if w == nil || handler == nil {
		return noOpUnsubscribe
	}
	w.handlersMu.Lock()
	id := w.nextHandlerID()
	if w.textHandlers == nil {
		w.textHandlers = make(map[uint64]ui.TextHandler)
	}
	w.textHandlers[id] = handler
	w.handlersMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			w.handlersMu.Lock()
			delete(w.textHandlers, id)
			w.handlersMu.Unlock()
		})
	}
}

// OnPointerButton registers a pointer-button handler and returns a function
// that removes it.
func (w *Window) OnPointerButton(handler ui.PointerButtonHandler) ui.Unsubscribe {
	if w == nil || handler == nil {
		return noOpUnsubscribe
	}
	w.handlersMu.Lock()
	id := w.nextHandlerID()
	if w.pointerButtonHandlers == nil {
		w.pointerButtonHandlers = make(map[uint64]ui.PointerButtonHandler)
	}
	w.pointerButtonHandlers[id] = handler
	w.handlersMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			w.handlersMu.Lock()
			delete(w.pointerButtonHandlers, id)
			w.handlersMu.Unlock()
		})
	}
}

// OnScroll registers a scroll handler and returns a function that removes it.
func (w *Window) OnScroll(handler ui.ScrollHandler) ui.Unsubscribe {
	if w == nil || handler == nil {
		return noOpUnsubscribe
	}
	w.handlersMu.Lock()
	id := w.nextHandlerID()
	if w.scrollHandlers == nil {
		w.scrollHandlers = make(map[uint64]ui.ScrollHandler)
	}
	w.scrollHandlers[id] = handler
	w.handlersMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			w.handlersMu.Lock()
			delete(w.scrollHandlers, id)
			w.handlersMu.Unlock()
		})
	}
}

func (w *Window) nextHandlerID() uint64 {
	w.nextID++
	return w.nextID
}

func noOpUnsubscribe() {
}

func (w *Window) dispatchKeyHandlers(event ui.KeyEvent) {
	w.handlersMu.RLock()
	handlers := make([]ui.KeyHandler, 0, len(w.keyHandlers))
	for _, handler := range w.keyHandlers {
		handlers = append(handlers, handler)
	}
	w.handlersMu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

func (w *Window) dispatchTextHandlers(text rune) {
	w.handlersMu.RLock()
	handlers := make([]ui.TextHandler, 0, len(w.textHandlers))
	for _, handler := range w.textHandlers {
		handlers = append(handlers, handler)
	}
	w.handlersMu.RUnlock()

	for _, handler := range handlers {
		handler(text)
	}
}

func (w *Window) dispatchPointerButtonHandlers(event ui.PointerButtonEvent) {
	w.handlersMu.RLock()
	handlers := make([]ui.PointerButtonHandler, 0, len(w.pointerButtonHandlers))
	for _, handler := range w.pointerButtonHandlers {
		handlers = append(handlers, handler)
	}
	w.handlersMu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

func (w *Window) dispatchScrollHandlers(event ui.ScrollEvent) {
	w.handlersMu.RLock()
	handlers := make([]ui.ScrollHandler, 0, len(w.scrollHandlers))
	for _, handler := range w.scrollHandlers {
		handlers = append(handlers, handler)
	}
	w.handlersMu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// GetKey reports whether key is currently held. Repeat is an event-stream detail;
// polling continues to report KeyPressed until the release event arrives.
func (w *Window) GetKey(key keyboard.Key) keyboard.KeyAction {
	if w == nil || key < 0 || key >= keyboard.Key(keyboard.KeyMenu+1) {
		return keyboard.KeyReleased
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.destroyed {
		return keyboard.KeyReleased
	}
	return w.keys[key]
}

// GetPointerButton reports whether button is currently held.
func (w *Window) GetPointerButton(button pointer.Button) pointer.ButtonAction {
	if w == nil || button < 0 || button >= pointer.ButtonCount {
		return pointer.Released
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.destroyed {
		return pointer.Released
	}
	return w.buttons[button]
}

func (w *Window) dispatchKey(key keyboard.Key, scancode int, action keyboard.KeyAction, mods keyboard.ModifierKey) {
	w.mu.Lock()
	if key >= 0 && key < keyboard.Key(keyboard.KeyMenu+1) {
		if action == keyboard.KeyReleased {
			w.keys[key] = keyboard.KeyReleased
		} else {
			// A repeated key-down remains pressed for polling. If a backend reports
			// repeated downs as ordinary presses, normalize subsequent ones here.
			if action == keyboard.KeyPressed && w.keys[key] == keyboard.KeyPressed {
				action = keyboard.KeyRepeat
			}
			w.keys[key] = keyboard.KeyPressed
		}
	}
	cb := w.keyCallback
	w.mu.Unlock()
	if cb != nil {
		cb(w, key, scancode, action, mods)
	}
}

func (w *Window) dispatchPointerButton(button pointer.Button, action pointer.ButtonAction, mods keyboard.ModifierKey) {
	w.mu.Lock()
	if button >= 0 && button < pointer.ButtonCount {
		w.buttons[button] = action
	}
	cb := w.pointerButtonCallback
	w.mu.Unlock()
	if cb != nil {
		cb(w, button, action, mods)
	}
}

// GetScroll returns the scroll offsets accumulated since the window was created.
//
// This is a running total rather than a per-frame delta, matching GLFW: a caller that
// wants the delta remembers the previous value and subtracts. Totals compose safely
// when several consumers read the same window; a delta would be consumed by whichever
// one read it first.
func (w *Window) GetScroll() (x, y float64) {
	if native := w.nativeWindow(); native != nil {
		return native.scrollTotal()
	}
	return 0, 0
}

// SetPointerMode selects how the cursor behaves over this window.
func (w *Window) SetPointerMode(mode pointer.Mode) {
	if native := w.nativeWindow(); native != nil {
		native.setCursorMode(mode)
	}
}

// GetPointerMode returns the window's current cursor mode.
func (w *Window) GetPointerMode() pointer.Mode {
	if native := w.nativeWindow(); native != nil {
		return native.cursorMode
	}
	return pointer.Normal
}

// DisablePointer hides the cursor and locks it to the window, so GetPointerPos reports
// unbounded virtual motion instead of a position inside the window.
//
// This is what a first-person camera needs: the pointer never reaches the edge of the
// display, so there is no limit on how far the view can turn. Shorthand for
// SetPointerMode(pointer.Disabled).
func (w *Window) DisablePointer() {
	w.SetPointerMode(pointer.Disabled)
}

// EnablePointer restores the normal visible, free-moving cursor. Shorthand for
// SetPointerMode(pointer.Normal).
func (w *Window) EnablePointer() {
	w.SetPointerMode(pointer.Normal)
}

// GetPointerPos returns the cursor in logical pixels relative to the top-left
// corner of the window's content area.
func (w *Window) GetPointerPos() (x, y float64) {
	if native := w.nativeWindow(); native != nil {
		return native.cursorPosition()
	}
	return 0, 0
}
