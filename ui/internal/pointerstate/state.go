// Package pointerstate keeps the platform-independent state behind PointerInput.
package pointerstate

import (
	"sync"

	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

// State tracks held buttons, cumulative scrolling, and pointer event handlers.
type State struct {
	mu               sync.RWMutex
	buttons          [pointer.ButtonCount]pointer.ButtonAction
	scrollX, scrollY float64
	nextHandlerID    uint64
	buttonHandlers   map[uint64]ui.PointerButtonHandler
	scrollHandlers   map[uint64]ui.ScrollHandler
}

// PointerButtonState reports the current state of button.
func (s *State) PointerButtonState(button pointer.Button) pointer.ButtonAction {
	if button < 0 || button >= pointer.ButtonCount {
		return pointer.Released
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buttons[button]
}

// ScrollOffset returns the cumulative scrolling dispatched to the state.
func (s *State) ScrollOffset() (x, y float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scrollX, s.scrollY
}

// OnPointerButton registers a button handler.
func (s *State) OnPointerButton(handler ui.PointerButtonHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}

	s.mu.Lock()
	s.nextHandlerID++
	id := s.nextHandlerID
	if s.buttonHandlers == nil {
		s.buttonHandlers = make(map[uint64]ui.PointerButtonHandler)
	}
	s.buttonHandlers[id] = handler
	s.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.buttonHandlers, id)
			s.mu.Unlock()
		})
	}
}

// OnScroll registers a scroll handler.
func (s *State) OnScroll(handler ui.ScrollHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}

	s.mu.Lock()
	s.nextHandlerID++
	id := s.nextHandlerID
	if s.scrollHandlers == nil {
		s.scrollHandlers = make(map[uint64]ui.ScrollHandler)
	}
	s.scrollHandlers[id] = handler
	s.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.scrollHandlers, id)
			s.mu.Unlock()
		})
	}
}

// DispatchPointerButton updates button state and invokes a snapshot of its handlers.
func (s *State) DispatchPointerButton(event ui.PointerButtonEvent) {
	s.mu.Lock()
	if event.Button >= 0 && event.Button < pointer.ButtonCount {
		s.buttons[event.Button] = event.Action
	}
	handlers := make([]ui.PointerButtonHandler, 0, len(s.buttonHandlers))
	for _, handler := range s.buttonHandlers {
		handlers = append(handlers, handler)
	}
	s.mu.Unlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// DispatchScroll adds event to the cumulative offset and invokes a snapshot of
// its handlers.
func (s *State) DispatchScroll(event ui.ScrollEvent) {
	s.mu.Lock()
	s.scrollX += event.X
	s.scrollY += event.Y
	handlers := make([]ui.ScrollHandler, 0, len(s.scrollHandlers))
	for _, handler := range s.scrollHandlers {
		handlers = append(handlers, handler)
	}
	s.mu.Unlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// Reset clears state and handlers without invoking them.
func (s *State) Reset() {
	s.mu.Lock()
	s.buttons = [pointer.ButtonCount]pointer.ButtonAction{}
	s.scrollX = 0
	s.scrollY = 0
	s.buttonHandlers = nil
	s.scrollHandlers = nil
	s.mu.Unlock()
}

func noOpUnsubscribe() {
}
