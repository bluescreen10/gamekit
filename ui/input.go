package ui

import (
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

// KeyEvent is one physical-key transition. Scancode is platform-specific and
// is useful for keys without a portable identity.
type KeyEvent struct {
	// Key is the portable identity of the physical key.
	Key keyboard.Key

	// Scancode is the platform code of the physical key.
	Scancode int

	// Action is the press or release.
	Action keyboard.KeyAction

	// Modifiers are the modifier keys that are down.
	Modifiers keyboard.ModifierKey

	// IsRepeat reports an auto-repeat press generated while the key is held.
	IsRepeat bool

	// DeviceID identifies the keyboard when the platform distinguishes devices.
	DeviceID int
}

// PointerButtonEvent is one pointing-device button transition.
type PointerButtonEvent struct {
	// Button is the button that changed.
	Button pointer.Button

	// Action is the press or release.
	Action pointer.ButtonAction

	// Modifiers are the modifier keys that are down.
	Modifiers keyboard.ModifierKey
}

// ScrollEvent is one high-resolution scroll event.
type ScrollEvent struct {
	// X is the horizontal scroll amount, with the sign that the platform
	// reports. A trackpad reports logical pixels; a wheel reports about one
	// unit for each detent.
	X float64

	// Y is the vertical scroll amount, in the same unit as X.
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
	// PointerButtonState returns the last action of button over the
	// surface.
	PointerButtonState(button pointer.Button) pointer.ButtonAction

	// PointerPosition returns the last position of the pointer.
	PointerPosition() (x, y float64)

	// ScrollOffset returns the total of all scroll events.
	ScrollOffset() (x, y float64)

	// OnPointerButton receives button presses and releases.
	OnPointerButton(handler PointerButtonHandler) Unsubscribe

	// OnScroll receives scroll events.
	OnScroll(handler ScrollHandler) Unsubscribe

	// OnPointerMove receives pointer motion.
	OnPointerMove(handler PointerMoveHandler) Unsubscribe

	// OnPointerEnter receives the pointer entering the surface.
	OnPointerEnter(handler PointerCrossingHandler) Unsubscribe

	// OnPointerLeave receives the pointer leaving the surface.
	OnPointerLeave(handler PointerCrossingHandler) Unsubscribe
}

// KeyInput provides keyboard event callbacks for a focused surface.
type KeyInput interface {
	// OnKey receives physical-key presses and releases.
	OnKey(handler KeyHandler) Unsubscribe

	// OnText receives the characters that the keys produce.
	OnText(handler TextHandler) Unsubscribe
}

// Input provides polling and event callbacks for keyboard and pointing
// devices across a whole window.
type Input interface {
	PointerInput

	// KeyState returns the last action of key while the window had focus.
	KeyState(key keyboard.Key) keyboard.KeyAction

	// PointerMode returns how the pointer behaves over the window.
	PointerMode() pointer.Mode

	// SetPointerMode changes how the pointer behaves over the window, for
	// example to hide it or to capture it for camera control.
	SetPointerMode(mode pointer.Mode)

	KeyInput

	// OnIME receives input-method composition events.
	OnIME(handler IMEHandler) Unsubscribe

	// OnTouch receives touch contacts.
	OnTouch(handler TouchHandler) Unsubscribe

	// OnPen receives stylus input.
	OnPen(handler PenHandler) Unsubscribe

	// OnGesture receives trackpad and touch gestures.
	OnGesture(handler GestureHandler) Unsubscribe

	// OnDeviceChange receives input devices that connect or disconnect.
	OnDeviceChange(handler DeviceChangeHandler) Unsubscribe
}

// PointerMoveEvent reports pointer motion in logical pixels from the surface's
// top-left corner. DeltaX and DeltaY are unaccelerated relative motion when
// the platform reports it, which is the only motion in disabled pointer mode.
type PointerMoveEvent struct {
	// X and Y are the position of the pointer.
	X, Y float64

	// DeltaX and DeltaY are the motion since the previous event.
	DeltaX, DeltaY float64

	// Modifiers are the modifier keys that are down.
	Modifiers keyboard.ModifierKey

	// DeviceID identifies the pointing device when the platform
	// distinguishes devices.
	DeviceID int
}

// PointerCrossingEvent reports the pointer entering or leaving a surface.
type PointerCrossingEvent struct {
	// X and Y are the position where the pointer crossed the edge.
	X, Y float64
}

// IMEEventKind identifies a stage of input-method composition.
type IMEEventKind uint8

const (
	// IMECompositionStart begins a composition.
	IMECompositionStart IMEEventKind = iota

	// IMECompositionUpdate changes the marked text.
	IMECompositionUpdate

	// IMECompositionCommit ends a composition with final text.
	IMECompositionCommit

	// IMECompositionCancel ends a composition without text.
	IMECompositionCancel
)

// IMEEvent reports input-method composition. Text is the marked text for an
// update and the committed text for a commit. Selection is relative to Text.
type IMEEvent struct {
	// Kind is the stage of the composition.
	Kind IMEEventKind

	// Text is the marked or committed text.
	Text string

	// Selection is the selected part of Text.
	Selection TextRange
}

// InputPhase is the lifecycle stage of a touch, pen, or gesture.
type InputPhase uint8

const (
	// PhaseBegan starts the input.
	PhaseBegan InputPhase = iota

	// PhaseMoved changes the input.
	PhaseMoved

	// PhaseEnded finishes the input normally.
	PhaseEnded

	// PhaseCancelled stops the input. The system took it for another use.
	PhaseCancelled
)

// TouchEvent reports one direct or trackpad touch contact.
type TouchEvent struct {
	// ID identifies the contact from PhaseBegan to its end.
	ID int

	// Phase is the stage of the contact.
	Phase InputPhase

	// X and Y are the position of the contact.
	X, Y float64

	// Pressure is the force of the contact from zero to one. It is zero when
	// the device does not measure force.
	Pressure float64

	// DeviceID identifies the touch device.
	DeviceID int
}

// PenEvent reports stylus input. Pressure is normalized to zero through one;
// tilt is in radians from vertical.
type PenEvent struct {
	// Phase is the stage of the stroke.
	Phase InputPhase

	// X and Y are the position of the pen tip.
	X, Y float64

	// Pressure is the force of the tip from zero to one.
	Pressure float64

	// TiltX and TiltY are the angles of the pen from vertical.
	TiltX, TiltY float64

	// Rotation is the rotation of the pen around its axis in radians.
	Rotation float64

	// Eraser is set when the eraser end touches the surface.
	Eraser bool

	// BarrelButton is set while the button on the pen is down.
	BarrelButton bool

	// DeviceID identifies the pen.
	DeviceID int
}

// GestureKind identifies a platform trackpad or touch gesture.
type GestureKind uint8

const (
	// GestureMagnify is a pinch that zooms.
	GestureMagnify GestureKind = iota

	// GestureRotate is a two-finger rotation.
	GestureRotate

	// GestureSwipe is a quick swipe with more than one finger.
	GestureSwipe

	// GestureSmartMagnify is a two-finger double tap that zooms to a part
	// of the content.
	GestureSmartMagnify

	// GesturePan is a drag with more than one finger.
	GesturePan
)

// GestureEvent reports a recognized gesture. Magnification is the relative
// scale change, Rotation is in radians, and DeltaX/DeltaY carry swipe or pan
// motion.
type GestureEvent struct {
	// Kind is the gesture.
	Kind GestureKind

	// Phase is the stage of the gesture.
	Phase InputPhase

	// X and Y are the position of the gesture.
	X, Y float64

	// Magnification is the scale change of a magnify gesture.
	Magnification float64

	// Rotation is the angle change of a rotate gesture.
	Rotation float64

	// DeltaX and DeltaY are the motion of a swipe or pan gesture.
	DeltaX, DeltaY float64
}

// DeviceKind classifies an input device.
type DeviceKind uint8

const (
	// DeviceKeyboard is a keyboard.
	DeviceKeyboard DeviceKind = iota

	// DevicePointer is a mouse or a trackpad.
	DevicePointer

	// DeviceTouch is a touch screen.
	DeviceTouch

	// DevicePen is a stylus.
	DevicePen

	// DeviceGamepad is a game controller.
	DeviceGamepad
)

// DeviceChangeEvent reports an input device connecting or disconnecting.
type DeviceChangeEvent struct {
	// Kind is the class of the device.
	Kind DeviceKind

	// DeviceID identifies the device in other events.
	DeviceID int

	// Name is the name of the device that the system reports.
	Name string

	// Connected is set when the device connects and clear when it
	// disconnects.
	Connected bool
}

// PointerMoveHandler receives pointer motion.
type PointerMoveHandler func(event PointerMoveEvent)

// PointerCrossingHandler receives pointer enter and leave events.
type PointerCrossingHandler func(event PointerCrossingEvent)

// IMEHandler receives input-method composition events.
type IMEHandler func(event IMEEvent)

// TouchHandler receives touch contacts.
type TouchHandler func(event TouchEvent)

// PenHandler receives stylus input.
type PenHandler func(event PenEvent)

// GestureHandler receives recognized gestures.
type GestureHandler func(event GestureEvent)

// DeviceChangeHandler receives input-device changes.
type DeviceChangeHandler func(event DeviceChangeEvent)
