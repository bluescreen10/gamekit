//go:build windows

package win32

import (
	"errors"
	"sync"
	"time"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/internal/pointerstate"
)

// canvasEvents holds subscribers for canvas input and presentation events.
type canvasEvents struct {
	keys       listeners[ui.KeyHandler]
	texts      listeners[ui.TextHandler]
	moves      listeners[ui.PointerMoveHandler]
	enters     listeners[ui.PointerCrossingHandler]
	leaves     listeners[ui.PointerCrossingHandler]
	frames     listeners[ui.FrameRequestHandler]
	occlusions listeners[ui.OcclusionHandler]
	surfaces   listeners[ui.SurfaceHandler]
	framesMu   sync.Mutex
}

func (c *canvas) OnKey(handler ui.KeyHandler) ui.Unsubscribe {
	return subscribe(&c.events.keys, handler, handler == nil)
}

func (c *canvas) OnText(handler ui.TextHandler) ui.Unsubscribe {
	return subscribe(&c.events.texts, handler, handler == nil)
}

func (c *canvas) OnPointerMove(handler ui.PointerMoveHandler) ui.Unsubscribe {
	return subscribe(&c.events.moves, handler, handler == nil)
}

func (c *canvas) OnPointerEnter(handler ui.PointerCrossingHandler) ui.Unsubscribe {
	return subscribe(&c.events.enters, handler, handler == nil)
}

func (c *canvas) OnPointerLeave(handler ui.PointerCrossingHandler) ui.Unsubscribe {
	return subscribe(&c.events.leaves, handler, handler == nil)
}

func (c *canvas) CapturePointer() error {
	native := c.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	if !native.captureCanvasPointer(true) {
		return errors.New("ui/win32: a canvas must be in a window to capture the pointer")
	}
	return nil
}

func (c *canvas) ReleasePointer() {
	if native := c.nativeView(); native != nil {
		native.captureCanvasPointer(false)
	}
}

func (c *canvas) HasPointerCapture() bool {
	native := c.nativeView()
	return native != nil && native.hasCapture()
}

// OnFrameRequest delivers display-synchronized frame requests while at least
// one handler is subscribed.
func (c *canvas) OnFrameRequest(handler ui.FrameRequestHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	unsubscribe := c.events.frames.add(handler)
	c.updateFrames()
	var once sync.Once
	return func() {
		once.Do(func() {
			unsubscribe()
			c.updateFrames()
		})
	}
}

func (c *canvas) updateFrames() {
	c.events.framesMu.Lock()
	defer c.events.framesMu.Unlock()
	native := c.nativeView()
	if native == nil {
		return
	}
	native.setCanvasFrames(len(c.events.frames.snapshot()) > 0)
}

func (c *canvas) OnOcclusionChange(handler ui.OcclusionHandler) ui.Unsubscribe {
	return subscribe(&c.events.occlusions, handler, handler == nil)
}

func (c *canvas) OnSurfaceChange(handler ui.SurfaceHandler) ui.Unsubscribe {
	return subscribe(&c.events.surfaces, handler, handler == nil)
}

func (c *canvas) ColorSpace() ui.ColorSpace {
	if c.nativeView() == nil {
		return ui.ColorSpaceUnknown
	}
	return ui.ColorSpaceSRGB
}

func (c *canvas) receiveEvent(event any) {
	switch event := event.(type) {
	case ui.KeyEvent:
		each(&c.events.keys, func(handler ui.KeyHandler) {
			handler(event)
		})
	case charEvent:
		each(&c.events.texts, func(handler ui.TextHandler) {
			handler(event.char)
		})
	case ui.PointerMoveEvent:
		each(&c.events.moves, func(handler ui.PointerMoveHandler) {
			handler(event)
		})
	case pointerEnterEvent:
		each(&c.events.enters, func(handler ui.PointerCrossingHandler) {
			handler(event.crossing)
		})
	case pointerLeaveEvent:
		each(&c.events.leaves, func(handler ui.PointerCrossingHandler) {
			handler(event.crossing)
		})
	case frameEvent:
		request := ui.FrameRequest{Time: time.Now(), Interval: event.interval}
		each(&c.events.frames, func(handler ui.FrameRequestHandler) {
			handler(request)
		})
	case occlusionEvent:
		each(&c.events.occlusions, func(handler ui.OcclusionHandler) {
			handler(event.occluded)
		})
	case ui.SurfaceEvent:
		each(&c.events.surfaces, func(handler ui.SurfaceHandler) {
			handler(event)
		})
	}
}

// canvasPointer is the pointer as one canvas sees it: only the presses and scrolling
// the platform delivered to the canvas itself. The window's own pointer state sees
// every event in the window, including those aimed at the controls beside the canvas.
type canvasPointer = pointerstate.State

// dispatchNativeButton reports a button of the pointer that the native canvas
// received.
func (c *canvas) dispatchNativeButton(button, action, mods int) {
	c.DispatchPointerButton(ui.PointerButtonEvent{
		Button:    pointer.Button(button),
		Action:    pointer.ButtonAction(action),
		Modifiers: keyboard.ModifierKey(mods),
	})
}

// dispatchNativeScroll reports scrolling that the native canvas received.
func (c *canvas) dispatchNativeScroll(x, y float64) {
	c.DispatchScroll(ui.ScrollEvent{X: x, Y: y})
}
