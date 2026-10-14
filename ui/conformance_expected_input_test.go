package ui_test

import (
	"errors"

	"github.com/bluescreen10/gamekit/ui"
)

func checkExpectedInput(r *recorder, window ui.Window) {
	r.check("input/complete/subscriptions", func() error {
		unsubscribes := []ui.Unsubscribe{
			window.OnKey(func(event ui.KeyEvent) {
				_ = event.IsRepeat
				_ = event.DeviceID
			}),
			window.OnPointerMove(func(ui.PointerMoveEvent) {
			}),
			window.OnPointerEnter(func(ui.PointerCrossingEvent) {
			}),
			window.OnPointerLeave(func(ui.PointerCrossingEvent) {
			}),
			window.OnIME(func(ui.IMEEvent) {
			}),
			window.OnTouch(func(ui.TouchEvent) {
			}),
			window.OnPen(func(ui.PenEvent) {
			}),
			window.OnGesture(func(ui.GestureEvent) {
			}),
			window.OnDeviceChange(func(ui.DeviceChangeEvent) {
			}),
		}
		for _, unsubscribe := range unsubscribes {
			if unsubscribe == nil {
				return errors.New("input subscription returned nil")
			}
			unsubscribe()
			unsubscribe()
		}
		return nil
	})

	canvas, err := window.CreateCanvas()
	r.check("canvas/complete/create", func() error {
		return err
	})
	if canvas == nil {
		return
	}
	defer canvas.Close()

	r.check("canvas/complete/input", func() error {
		unsubscribes := []ui.Unsubscribe{
			canvas.OnKey(func(ui.KeyEvent) {
			}),
			canvas.OnText(func(rune) {
			}),
			canvas.OnPointerMove(func(ui.PointerMoveEvent) {
			}),
			canvas.OnPointerEnter(func(ui.PointerCrossingEvent) {
			}),
			canvas.OnPointerLeave(func(ui.PointerCrossingEvent) {
			}),
		}
		for _, unsubscribe := range unsubscribes {
			unsubscribe()
		}
		if err := canvas.CapturePointer(); err != nil {
			return err
		}
		if !canvas.HasPointerCapture() {
			return errors.New("canvas did not capture the pointer")
		}
		canvas.ReleasePointer()
		if canvas.HasPointerCapture() {
			return errors.New("canvas retained pointer capture")
		}
		return nil
	})
}
