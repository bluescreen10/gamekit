//go:build windows && cgo

package win32

/*
#include "widget_bridge.h"
*/
import "C"

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
// the canvas is closed (gkWin32UIClearAction zeroes the view's copy first).
func canvasFromHandle(handle C.uintptr_t) *canvas {
	if handle == 0 {
		return nil
	}
	result, _ := cgo.Handle(handle).Value().(*canvas)
	return result
}

//export gkWin32UIGoCanvasPointerButton
func gkWin32UIGoCanvasPointerButton(handle C.uintptr_t, button, action, mods C.int) {
	if c := canvasFromHandle(handle); c != nil {
		c.DispatchPointerButton(ui.PointerButtonEvent{
			Button:    pointer.Button(button),
			Action:    pointer.ButtonAction(action),
			Modifiers: keyboard.ModifierKey(mods),
		})
	}
}

//export gkWin32UIGoCanvasScroll
func gkWin32UIGoCanvasScroll(handle C.uintptr_t, x, y C.double) {
	if c := canvasFromHandle(handle); c != nil {
		c.DispatchScroll(ui.ScrollEvent{X: float64(x), Y: float64(y)})
	}
}
