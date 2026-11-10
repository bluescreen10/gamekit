//go:build linux && cgo

package gtk

import (
	"runtime/cgo"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/internal/pointerstate"
)

// canvasPointer is the pointer as one canvas sees it: only the presses and scrolling
// the platform delivered to the canvas itself. The window's own pointer state sees
// every event in the window, including those aimed at the controls beside the canvas.
type canvasPointer = pointerstate.State

// canvasFromHandle resolves the handle a native canvas view reports with, or nil once
// the canvas is closed (gkGTKUIClearAction zeroes the view's copy first).
func canvasFromHandle(handle cUintptr) *canvas {
	if handle == 0 {
		return nil
	}
	result, _ := cgo.Handle(handle).Value().(*canvas)
	return result
}

func gkGTKUIGoCanvasPointerButton(handle cUintptr, button, action, mods cInt) {
	if c := canvasFromHandle(handle); c != nil {
		c.DispatchPointerButton(ui.PointerButtonEvent{
			Button:    pointer.Button(button),
			Action:    pointer.ButtonAction(action),
			Modifiers: keyboard.ModifierKey(mods),
		})
	}
}

func gkGTKUIGoCanvasScroll(handle cUintptr, x, y cDouble) {
	if c := canvasFromHandle(handle); c != nil {
		c.DispatchScroll(ui.ScrollEvent{X: float64(x), Y: float64(y)})
	}
}
