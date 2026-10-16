//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
*/
import "C"

import (
	"errors"
	"sync"
	"time"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui"
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
	native := c.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	if C.gkGTKCanvasCapture(native, 1) == 0 {
		return errors.New("ui/gtk: a canvas must be in a window to capture the pointer")
	}
	return nil
}

func (c *canvas) ReleasePointer() {
	if native := c.nativePointer(); native != nil {
		C.gkGTKCanvasCapture(native, 0)
	}
}

func (c *canvas) HasPointerCapture() bool {
	native := c.nativePointer()
	return native != nil && C.gkGTKCanvasHasCapture(native) != 0
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
	native := c.nativePointer()
	if native == nil {
		return
	}
	C.gkGTKCanvasStartFrames(native, boolInt(len(c.events.frames.snapshot()) > 0))
}

func (c *canvas) OnOcclusionChange(handler ui.OcclusionHandler) ui.Unsubscribe {
	return subscribe(&c.events.occlusions, handler, handler == nil)
}

func (c *canvas) OnSurfaceChange(handler ui.SurfaceHandler) ui.Unsubscribe {
	return subscribe(&c.events.surfaces, handler, handler == nil)
}

func (c *canvas) ColorSpace() ui.ColorSpace {
	native := c.nativePointer()
	if native == nil {
		return ui.ColorSpaceUnknown
	}
	return ui.ColorSpace(C.gkGTKViewColorSpace(native))
}

func (c *canvas) gtkEvent(event gtkEvent) {
	switch event.kind {
	case C.GK_EVENT_KEY:
		key := ui.KeyEvent{
			Key:       keyboard.Key(event.a),
			Scancode:  event.b,
			Action:    keyboard.KeyAction(event.c),
			Modifiers: keyboard.ModifierKey(int(event.x)),
			IsRepeat:  keyboard.KeyAction(event.c) == keyboard.KeyRepeat,
		}
		each(&c.events.keys, func(handler ui.KeyHandler) {
			handler(key)
		})
	case C.GK_EVENT_TEXT:
		text := rune(event.a)
		each(&c.events.texts, func(handler ui.TextHandler) {
			handler(text)
		})
	case C.GK_EVENT_POINTER_MOVE:
		move := ui.PointerMoveEvent{
			X: event.x, Y: event.y, DeltaX: event.z, DeltaY: event.w,
			Modifiers: keyboard.ModifierKey(event.a),
		}
		each(&c.events.moves, func(handler ui.PointerMoveHandler) {
			handler(move)
		})
	case C.GK_EVENT_POINTER_ENTER:
		crossing := ui.PointerCrossingEvent{X: event.x, Y: event.y}
		each(&c.events.enters, func(handler ui.PointerCrossingHandler) {
			handler(crossing)
		})
	case C.GK_EVENT_POINTER_LEAVE:
		crossing := ui.PointerCrossingEvent{X: event.x, Y: event.y}
		each(&c.events.leaves, func(handler ui.PointerCrossingHandler) {
			handler(crossing)
		})
	case C.GK_EVENT_FRAME:
		request := ui.FrameRequest{
			Time:     time.Now().Add(time.Duration(event.x * float64(time.Second))),
			Interval: time.Duration(event.y * float64(time.Second)),
		}
		each(&c.events.frames, func(handler ui.FrameRequestHandler) {
			handler(request)
		})
	case C.GK_EVENT_WINDOW_OCCLUSION:
		occluded := event.a != 0
		each(&c.events.occlusions, func(handler ui.OcclusionHandler) {
			handler(occluded)
		})
	case C.GK_EVENT_SURFACE:
		kinds := [...]ui.SurfaceEventKind{ui.SurfaceResized, ui.SurfaceScaleChanged, ui.SurfaceColorSpaceChanged}
		surface := ui.SurfaceEvent{
			Kind:            ui.SurfaceResized,
			FramebufferSize: ui.Size{Width: int(event.x), Height: int(event.y)},
			ScaleFactor:     event.z,
			ColorSpace:      ui.ColorSpace(event.b),
		}
		if event.a >= 0 && event.a < len(kinds) {
			surface.Kind = kinds[event.a]
		}
		each(&c.events.surfaces, func(handler ui.SurfaceHandler) {
			handler(surface)
		})
	}
}
