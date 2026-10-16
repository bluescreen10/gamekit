package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

func inputCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Canvas", "Click the canvas to focus it, then press keys. Hold a button to capture the pointer.", g.canvasDemo),
		g.card("Window input", "Input events that the whole window receives.", g.windowInputDemo),
		g.card("Pointer mode", "Hide the pointer, or lock it to the window as a first-person camera does. Press Escape to return to normal.", g.pointerModeDemo),
	}
}

func (g *gallery) canvasDemo() ui.Widget {
	canvas := must(g.window.CreateCanvas())
	canvas.SetMinimumSize(ui.Size{Width: 320, Height: 260})
	g.attempt("Cursor", func() error {
		return canvas.SetCursor(ui.CursorCrosshair)
	})

	surface := g.value("Surface: waiting")
	buttons := g.value("Buttons: none")
	motion := g.value("Pointer: outside")
	scroll := g.value("Scroll: 0, 0")
	keys := g.value("Keys: click the canvas first")
	frames := g.value("Frames: 0")

	canvas.OnResize(func(size ui.Size) {
		pixels := canvas.FramebufferSize()
		surface.SetText(fmt.Sprintf("Surface: %d × %d points, %d × %d pixels, color space %d",
			size.Width, size.Height, pixels.Width, pixels.Height, canvas.ColorSpace()))
	})
	g.attempt("Surface events", func() error {
		canvas.OnSurfaceChange(func(event ui.SurfaceEvent) {
			g.log("Canvas surface event %d: %d × %d at scale %.2f", event.Kind, event.FramebufferSize.Width,
				event.FramebufferSize.Height, event.ScaleFactor)
		})
		canvas.OnOcclusionChange(func(occluded bool) {
			g.log("Canvas occluded: %t", occluded)
		})
		return nil
	})
	canvas.OnPointerButton(func(event ui.PointerButtonEvent) {
		x, y := canvas.PointerPosition()
		buttons.SetText(fmt.Sprintf("Buttons: button %d %s at %.0f, %.0f, modifiers %d",
			event.Button, describeButtonAction(event.Action), x, y, event.Modifiers))
		if event.Action == pointer.Pressed {
			g.attempt("Focus", canvas.Focus)
			g.attempt("Capture", canvas.CapturePointer)
		} else {
			canvas.ReleasePointer()
		}
		canvas.RequestRedraw()
	})
	g.attempt("Pointer motion", func() error {
		canvas.OnPointerMove(func(event ui.PointerMoveEvent) {
			motion.SetText(fmt.Sprintf("Pointer: %.0f, %.0f (moved %.1f, %.1f), captured %t",
				event.X, event.Y, event.DeltaX, event.DeltaY, canvas.HasPointerCapture()))
		})
		canvas.OnPointerEnter(func(event ui.PointerCrossingEvent) {
			g.log("Pointer entered the canvas at %.0f, %.0f", event.X, event.Y)
		})
		canvas.OnPointerLeave(func(event ui.PointerCrossingEvent) {
			motion.SetText("Pointer: outside")
		})
		return nil
	})
	canvas.OnScroll(func(event ui.ScrollEvent) {
		x, y := canvas.ScrollOffset()
		scroll.SetText(fmt.Sprintf("Scroll: %.2f, %.2f; total %.2f, %.2f", event.X, event.Y, x, y))
	})
	canvas.OnKey(func(event ui.KeyEvent) {
		keys.SetText(fmt.Sprintf("Keys: key %d scancode %d %s, modifiers %d, repeat %t",
			event.Key, event.Scancode, describeKeyAction(event.Action), event.Modifiers, event.IsRepeat))
	})
	canvas.OnText(func(char rune) {
		g.log("Canvas text: %q", char)
	})

	frameCount := 0
	g.attempt("Frame requests", func() error {
		canvas.OnFrameRequest(func(request ui.FrameRequest) {
			frameCount++
			frames.SetText(fmt.Sprintf("Frames: %d (interval %s)", frameCount, request.Interval))
		})
		return nil
	})
	redraw := g.button("Request redraw", func() {
		canvas.RequestRedraw()
		g.log("Redraw pending: %t", canvas.TakeRedrawRequest())
	})

	return g.column(canvas, surface, buttons, motion, scroll, keys, frames, g.row(redraw))
}

func describeButtonAction(action pointer.ButtonAction) string {
	if action == pointer.Pressed {
		return "pressed"
	}
	return "released"
}

func describeKeyAction(action keyboard.KeyAction) string {
	switch action {
	case keyboard.KeyPressed:
		return "pressed"
	case keyboard.KeyRepeat:
		return "repeated"
	default:
		return "released"
	}
}

func (g *gallery) windowInputDemo() ui.Widget {
	buttons := g.value("Buttons: click anywhere in the window")
	scroll := g.value("Scroll: none")
	keys := g.value("Keys: none")
	text := g.value("Text: none")
	other := g.value("Other: none")
	other.SetWrapMode(ui.WrapWords)

	g.window.OnPointerButton(func(event ui.PointerButtonEvent) {
		x, y := g.window.PointerPosition()
		buttons.SetText(fmt.Sprintf("Buttons: button %d %s at %.0f, %.0f", event.Button, describeButtonAction(event.Action), x, y))
	})
	g.window.OnScroll(func(event ui.ScrollEvent) {
		x, y := g.window.ScrollOffset()
		scroll.SetText(fmt.Sprintf("Scroll: %.2f, %.2f; total %.2f, %.2f", event.X, event.Y, x, y))
	})
	g.window.OnKey(func(event ui.KeyEvent) {
		keys.SetText(fmt.Sprintf("Keys: key %d %s, modifiers %d; Shift is %s", event.Key,
			describeKeyAction(event.Action), event.Modifiers, describeKeyAction(g.window.KeyState(keyboard.KeyLeftShift))))
	})
	typed := []rune{}
	g.window.OnText(func(char rune) {
		typed = append(typed, char)
		if len(typed) > 40 {
			typed = typed[len(typed)-40:]
		}
		text.SetText("Text: " + string(typed))
	})

	subscriptions := []struct {
		name      string
		subscribe func()
	}{
		{"IME", func() {
			g.window.OnIME(func(event ui.IMEEvent) {
				other.SetText(fmt.Sprintf("Other: IME event %d, text %q", event.Kind, event.Text))
			})
		}},
		{"Touch", func() {
			g.window.OnTouch(func(event ui.TouchEvent) {
				other.SetText(fmt.Sprintf("Other: touch %d phase %d at %.0f, %.0f", event.ID, event.Phase, event.X, event.Y))
			})
		}},
		{"Pen", func() {
			g.window.OnPen(func(event ui.PenEvent) {
				other.SetText(fmt.Sprintf("Other: pen phase %d pressure %.2f tilt %.1f, %.1f eraser %t",
					event.Phase, event.Pressure, event.TiltX, event.TiltY, event.Eraser))
			})
		}},
		{"Gesture", func() {
			g.window.OnGesture(func(event ui.GestureEvent) {
				other.SetText(fmt.Sprintf("Other: gesture %d phase %d, magnification %.2f, rotation %.1f",
					event.Kind, event.Phase, event.Magnification, event.Rotation))
			})
		}},
		{"Device", func() {
			g.window.OnDeviceChange(func(event ui.DeviceChangeEvent) {
				g.log("Device %q (kind %d) connected: %t", event.Name, event.Kind, event.Connected)
			})
		}},
	}
	available := []string{}
	for _, entry := range subscriptions {
		g.attempt(entry.name+" events", func() error {
			entry.subscribe()
			available = append(available, entry.name)
			return nil
		})
	}
	return g.column(buttons, scroll, keys, text, other,
		g.caption("Subscribed to: "+strings.Join(available, ", ")))
}

func (g *gallery) pointerModeDemo() ui.Widget {
	modes := []pointer.Mode{pointer.Normal, pointer.Hidden, pointer.Disabled}
	var control ui.Select
	control = g.choice([]string{"Normal", "Hidden", "Disabled (locked)"}, 0, func(index int) {
		g.window.SetPointerMode(modes[index])
		g.log("Pointer mode: %d", g.window.PointerMode())
	})
	g.window.OnKey(func(event ui.KeyEvent) {
		if event.Key != keyboard.KeyEscape || event.Action != keyboard.KeyPressed {
			return
		}
		if g.window.PointerMode() == pointer.Normal {
			return
		}
		g.window.SetPointerMode(pointer.Normal)
		g.attempt("Pointer mode", func() error {
			return control.SetSelectedIndex(0)
		})
		g.log("Pointer mode reset by Escape")
	})
	restore := g.button("Normal after 5 seconds", func() {
		g.after(5*time.Second, func() {
			g.window.SetPointerMode(pointer.Normal)
			g.attempt("Pointer mode", func() error {
				return control.SetSelectedIndex(0)
			})
		})
	})
	return g.row(control, restore)
}
