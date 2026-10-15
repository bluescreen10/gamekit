//go:build darwin && cgo

package cocoa

/*
#include "cocoa_bridge.h"
#include "window_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"runtime/cgo"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui"
)

// windowEvents holds the subscribers for the extended window contract.
type windowEvents struct {
	resizes       listeners[ui.ResizeHandler]
	fullscreens   listeners[ui.FullscreenHandler]
	closeRequests listeners[ui.CloseRequestHandler]
	focuses       listeners[ui.FocusHandler]
	blurs         listeners[ui.FocusHandler]
	themes        listeners[ui.ThemeChangeHandler]
	fileDrops     listeners[ui.FileDropHandler]
	preferences   listeners[ui.SystemPreferencesHandler]
	pointerMoves  listeners[ui.PointerMoveHandler]
	enters        listeners[ui.PointerCrossingHandler]
	leaves        listeners[ui.PointerCrossingHandler]
	imes          listeners[ui.IMEHandler]
	touches       listeners[ui.TouchHandler]
	pens          listeners[ui.PenHandler]
	gestures      listeners[ui.GestureHandler]
	devices       listeners[ui.DeviceChangeHandler]

	activated     listeners[ui.FocusHandler]
	deactivated   listeners[ui.FocusHandler]
	reopens       listeners[ui.FocusHandler]
	quitRequests  listeners[ui.CloseRequestHandler]
	clipboard     listeners[ui.FocusHandler]
	notifications listeners[ui.NotificationActivationHandler]
}

// windowFlags are the window attributes that can change at run time.
type windowFlags struct {
	minimum     ui.Size
	maximum     ui.Size
	resizable   bool
	borderless  bool
	alwaysOnTop bool
}

// attachExtensions connects the extended window contract after creation.
func (w *Window) attachExtensions(options WindowOptions) error {
	window := w.nativeWindowPointer()
	C.gkCocoaWindowAttach(window, C.uintptr_t(w.handle))
	attachApplication()
	w.flags = windowFlags{
		resizable:   options.Resizable,
		borderless:  options.Borderless,
		alwaysOnTop: options.AlwaysOnTop,
	}
	if options.MinimumSize != (ui.Size{}) || options.MaximumSize != (ui.Size{}) {
		if err := w.SetMinimumSize(options.MinimumSize); err != nil {
			return err
		}
		if err := w.SetMaximumSize(options.MaximumSize); err != nil {
			return err
		}
	}
	if options.Maximized {
		if err := w.Maximize(); err != nil {
			return err
		}
	}
	if options.Fullscreen {
		if err := w.SetFullscreen(true); err != nil && !errors.Is(err, ui.ErrUnavailable) {
			return err
		}
	}
	return nil
}

// detachNativeWindow disconnects the extended contract before destruction.
func detachNativeWindow(native unsafe.Pointer) {
	if window := C.gkWindowNativePointer(native); window != nil {
		C.gkCocoaWindowDetach(window)
	}
}

// MinimumSize implements ui.Window.MinimumSize.
func (w *Window) MinimumSize() ui.Size {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.flags.minimum
}

// SetMinimumSize implements ui.Window.SetMinimumSize.
func (w *Window) SetMinimumSize(size ui.Size) error {
	w.mu.RLock()
	maximum := w.flags.maximum
	w.mu.RUnlock()
	return w.setSizeLimits(size, maximum)
}

// MaximumSize implements ui.Window.MaximumSize.
func (w *Window) MaximumSize() ui.Size {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.flags.maximum
}

// SetMaximumSize implements ui.Window.SetMaximumSize.
func (w *Window) SetMaximumSize(size ui.Size) error {
	w.mu.RLock()
	minimum := w.flags.minimum
	w.mu.RUnlock()
	return w.setSizeLimits(minimum, size)
}

func (w *Window) setSizeLimits(minimum, maximum ui.Size) error {
	if err := ui.ValidateSizeConstraints(minimum, maximum); err != nil {
		return err
	}
	window := w.nativeWindowPointer()
	if window == nil {
		return ui.ErrClosed
	}
	C.gkCocoaWindowSetSizeLimits(window, C.int(minimum.Width), C.int(minimum.Height),
		C.int(maximum.Width), C.int(maximum.Height))
	w.mu.Lock()
	w.flags.minimum = minimum
	w.flags.maximum = maximum
	w.mu.Unlock()
	return nil
}

// IsResizable implements ui.Window.IsResizable.
func (w *Window) IsResizable() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.flags.resizable
}

// SetResizable implements ui.Window.SetResizable.
func (w *Window) SetResizable(resizable bool) {
	w.mu.Lock()
	w.flags.resizable = resizable
	flags := w.flags
	w.mu.Unlock()
	w.applyStyle(flags)
}

// IsBorderless implements ui.Window.IsBorderless.
func (w *Window) IsBorderless() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.flags.borderless
}

// SetBorderless implements ui.Window.SetBorderless.
func (w *Window) SetBorderless(borderless bool) {
	w.mu.Lock()
	w.flags.borderless = borderless
	flags := w.flags
	w.mu.Unlock()
	w.applyStyle(flags)
}

func (w *Window) applyStyle(flags windowFlags) {
	if window := w.nativeWindowPointer(); window != nil {
		C.gkCocoaWindowSetStyle(window, boolInt(flags.resizable), boolInt(flags.borderless))
	}
}

// IsAlwaysOnTop implements ui.Window.IsAlwaysOnTop.
func (w *Window) IsAlwaysOnTop() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.flags.alwaysOnTop
}

// SetAlwaysOnTop implements ui.Window.SetAlwaysOnTop.
func (w *Window) SetAlwaysOnTop(alwaysOnTop bool) {
	window := w.nativeWindowPointer()
	if window == nil {
		return
	}
	C.gkCocoaWindowSetAlwaysOnTop(window, boolInt(alwaysOnTop))
	w.mu.Lock()
	w.flags.alwaysOnTop = alwaysOnTop
	w.mu.Unlock()
}

func (w *Window) windowCall(call func(unsafe.Pointer) C.int, failure string) error {
	window := w.nativeWindowPointer()
	if window == nil {
		return ui.ErrClosed
	}
	if call(window) == 0 {
		return errors.New("ui/cocoa: " + failure)
	}
	return nil
}

// Minimize implements ui.Window.Minimize. It returns an error for a fullscreen
// window, because macOS does not minimize fullscreen windows.
func (w *Window) Minimize() error {
	return w.windowCall(func(window unsafe.Pointer) C.int {
		return C.gkCocoaWindowMinimize(window)
	},
		"the window could not be minimized")
}

// Maximize implements ui.Window.Maximize.
func (w *Window) Maximize() error {
	return w.windowCall(func(window unsafe.Pointer) C.int {
		return C.gkCocoaWindowMaximize(window)
	},
		"the window could not be maximized")
}

// Restore implements ui.Window.Restore. Leaving fullscreen completes
// asynchronously.
func (w *Window) Restore() error {
	return w.windowCall(func(window unsafe.Pointer) C.int {
		return C.gkCocoaWindowRestore(window)
	},
		"the window could not be restored")
}

// IsMinimized implements ui.Window.IsMinimized.
func (w *Window) IsMinimized() bool {
	window := w.nativeWindowPointer()
	return window != nil && C.gkCocoaWindowIsMinimized(window) != 0
}

// IsMaximized implements ui.Window.IsMaximized.
func (w *Window) IsMaximized() bool {
	window := w.nativeWindowPointer()
	return window != nil && C.gkCocoaWindowIsMaximized(window) != 0
}

// IsFullscreen implements ui.Window.IsFullscreen. It reports a completed
// transition into the fullscreen Space of the window.
func (w *Window) IsFullscreen() bool {
	window := w.nativeWindowPointer()
	return window != nil && C.gkCocoaWindowIsFullscreen(window) != 0
}

// OnFullscreenChange reports completed fullscreen transitions.
func (w *Window) OnFullscreenChange(handler ui.FullscreenHandler) ui.Unsubscribe {
	return subscribe(&w.events.fullscreens, handler, handler == nil)
}

// SetFullscreen requests macOS's native fullscreen, which moves the window to
// its own Space. The animated transition completes asynchronously.
func (w *Window) SetFullscreen(fullscreen bool) error {
	if w.Kind() == ui.WindowUtility && fullscreen {
		return errors.New("ui: utility window cannot be fullscreen")
	}
	window := w.nativeWindowPointer()
	if window == nil {
		return ui.ErrClosed
	}
	if C.gkCocoaWindowSetFullscreen(window, boolInt(fullscreen)) == 0 {
		return fmt.Errorf("ui/cocoa: fullscreen needs an unlocked, awake display: %w", ui.ErrUnavailable)
	}
	return nil
}

// canBecomeKey reports whether AppKit lets the window take keyboard focus.
func (w *Window) canBecomeKey() bool {
	window := w.nativeWindowPointer()
	return window != nil && C.gkCocoaWindowCanBecomeKey(window) != 0
}

// Screen implements ui.Window.Screen.
func (w *Window) Screen() ui.Screen {
	window := w.nativeWindowPointer()
	if window == nil {
		return ui.Screen{}
	}
	var values [11]C.double
	var name *C.char
	C.gkCocoaWindowScreen(window, &values[0], &name)
	rect := func(offset int) ui.Rect {
		return ui.Rect{
			Position: ui.Point{X: int(values[offset]), Y: int(values[offset+1])},
			Size:     ui.Size{Width: int(values[offset+2]), Height: int(values[offset+3])},
		}
	}
	return ui.Screen{
		Name:        goStringFree(name),
		Bounds:      rect(0),
		WorkArea:    rect(4),
		ScaleFactor: float64(values[8]),
		DPI:         float64(values[9]),
		RefreshRate: float64(values[10]),
	}
}

// SystemPreferences implements ui.Window.SystemPreferences.
func (w *Window) SystemPreferences() ui.SystemPreferences {
	return systemPreferences(w.nativeWindowPointer())
}

func systemPreferences(window unsafe.Pointer) ui.SystemPreferences {
	var values [10]C.double
	var locale *C.char
	C.gkCocoaSystemPreferences(window, &values[0], &locale)
	channel := func(value C.double) uint8 {
		return uint8(math.Round(math.Max(0, math.Min(1, float64(value))) * 255))
	}
	return ui.SystemPreferences{
		ResolvedTheme:       ui.Theme(values[0]),
		TextScale:           float64(values[1]),
		HighContrast:        values[2] != 0,
		ReducedMotion:       values[3] != 0,
		ReducedTransparency: values[4] != 0,
		AccentColor:         color.RGBA{R: channel(values[5]), G: channel(values[6]), B: channel(values[7]), A: channel(values[8])},
		Locale:              goStringFree(locale),
		RightToLeft:         values[9] != 0,
	}
}

func subscribe[T any](list *listeners[T], handler T, nilHandler bool) ui.Unsubscribe {
	if nilHandler {
		return noOpUnsubscribe
	}
	return list.add(handler)
}

// OnResize implements ui.Window.OnResize.
func (w *Window) OnResize(handler ui.ResizeHandler) ui.Unsubscribe {
	return subscribe(&w.events.resizes, handler, handler == nil)
}

// OnCloseRequest implements ui.Window.OnCloseRequest.
func (w *Window) OnCloseRequest(handler ui.CloseRequestHandler) ui.Unsubscribe {
	return subscribe(&w.events.closeRequests, handler, handler == nil)
}

// OnFocus implements ui.Window.OnFocus.
func (w *Window) OnFocus(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&w.events.focuses, handler, handler == nil)
}

// OnBlur implements ui.Window.OnBlur.
func (w *Window) OnBlur(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&w.events.blurs, handler, handler == nil)
}

// OnThemeChange implements ui.Window.OnThemeChange.
func (w *Window) OnThemeChange(handler ui.ThemeChangeHandler) ui.Unsubscribe {
	return subscribe(&w.events.themes, handler, handler == nil)
}

// OnFileDrop implements ui.Window.OnFileDrop.
func (w *Window) OnFileDrop(handler ui.FileDropHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	unsubscribe := w.events.fileDrops.add(handler)
	w.updateFileDrops()
	var once sync.Once
	return func() {
		once.Do(func() {
			unsubscribe()
			w.updateFileDrops()
		})
	}
}

// updateFileDrops registers the window for file drops while it has handlers.
func (w *Window) updateFileDrops() {
	if window := w.nativeWindowPointer(); window != nil {
		C.gkCocoaWindowAcceptFiles(window, boolInt(len(w.events.fileDrops.snapshot()) > 0))
	}
}

// OnSystemPreferencesChange implements ui.Window.OnSystemPreferencesChange.
func (w *Window) OnSystemPreferencesChange(handler ui.SystemPreferencesHandler) ui.Unsubscribe {
	return subscribe(&w.events.preferences, handler, handler == nil)
}

// OnPointerMove implements ui.Window.OnPointerMove.
func (w *Window) OnPointerMove(handler ui.PointerMoveHandler) ui.Unsubscribe {
	return subscribe(&w.events.pointerMoves, handler, handler == nil)
}

// OnPointerEnter implements ui.Window.OnPointerEnter.
func (w *Window) OnPointerEnter(handler ui.PointerCrossingHandler) ui.Unsubscribe {
	return subscribe(&w.events.enters, handler, handler == nil)
}

// OnPointerLeave implements ui.Window.OnPointerLeave.
func (w *Window) OnPointerLeave(handler ui.PointerCrossingHandler) ui.Unsubscribe {
	return subscribe(&w.events.leaves, handler, handler == nil)
}

// OnIME implements ui.Window.OnIME.
func (w *Window) OnIME(handler ui.IMEHandler) ui.Unsubscribe {
	return subscribe(&w.events.imes, handler, handler == nil)
}

// OnTouch implements ui.Window.OnTouch.
func (w *Window) OnTouch(handler ui.TouchHandler) ui.Unsubscribe {
	return subscribe(&w.events.touches, handler, handler == nil)
}

// OnPen implements ui.Window.OnPen.
func (w *Window) OnPen(handler ui.PenHandler) ui.Unsubscribe {
	return subscribe(&w.events.pens, handler, handler == nil)
}

// OnGesture implements ui.Window.OnGesture.
func (w *Window) OnGesture(handler ui.GestureHandler) ui.Unsubscribe {
	return subscribe(&w.events.gestures, handler, handler == nil)
}

// OnDeviceChange implements ui.Window.OnDeviceChange.
func (w *Window) OnDeviceChange(handler ui.DeviceChangeHandler) ui.Unsubscribe {
	return subscribe(&w.events.devices, handler, handler == nil)
}

// OnActivated implements ui.Window.OnActivated.
func (w *Window) OnActivated(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&w.events.activated, handler, handler == nil)
}

// OnDeactivated implements ui.Window.OnDeactivated.
func (w *Window) OnDeactivated(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&w.events.deactivated, handler, handler == nil)
}

// OnReopen implements ui.Window.OnReopen. macOS reports a reopen when the user
// clicks the Dock icon of the running application.
func (w *Window) OnReopen(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&w.events.reopens, handler, handler == nil)
}

// OnQuitRequest implements ui.Window.OnQuitRequest.
func (w *Window) OnQuitRequest(handler ui.CloseRequestHandler) ui.Unsubscribe {
	return subscribe(&w.events.quitRequests, handler, handler == nil)
}

func each[T any](list *listeners[T], call func(T)) {
	for _, handler := range list.snapshot() {
		call(handler)
	}
}

// cocoaEvent receives window events from the AppKit layer.
func (w *Window) cocoaEvent(event cocoaEvent) {
	switch event.kind {
	case C.GK_EVENT_WINDOW_RESIZE:
		size := ui.Size{Width: int(event.x), Height: int(event.y)}
		each(&w.events.resizes, func(handler ui.ResizeHandler) {
			handler(size)
		})
	case C.GK_EVENT_WINDOW_FULLSCREEN:
		fullscreen := event.a != 0
		each(&w.events.fullscreens, func(handler ui.FullscreenHandler) {
			handler(fullscreen)
		})
	case C.GK_EVENT_WINDOW_FOCUS:
		w.mu.RLock()
		bar := w.menuBar
		w.mu.RUnlock()
		if bar != nil {
			bar.install()
		}
		each(&w.events.focuses, func(handler ui.FocusHandler) {
			handler()
		})
	case C.GK_EVENT_WINDOW_BLUR:
		each(&w.events.blurs, func(handler ui.FocusHandler) {
			handler()
		})
	case C.GK_EVENT_WINDOW_THEME:
		theme := ui.Theme(event.a)
		each(&w.events.themes, func(handler ui.ThemeChangeHandler) {
			handler(theme)
		})
		w.dispatchPreferences()
	case C.GK_EVENT_WINDOW_FILE_DROP:
		paths := strings.Split(event.text, "\x1e")
		each(&w.events.fileDrops, func(handler ui.FileDropHandler) {
			handler(append([]string(nil), paths...))
		})
	case C.GK_EVENT_POINTER_MOVE:
		move := ui.PointerMoveEvent{
			X: event.x, Y: event.y, DeltaX: event.z, DeltaY: event.w,
			Modifiers: keyboard.ModifierKey(event.a),
		}
		each(&w.events.pointerMoves, func(handler ui.PointerMoveHandler) {
			handler(move)
		})
	case C.GK_EVENT_POINTER_ENTER:
		crossing := ui.PointerCrossingEvent{X: event.x, Y: event.y}
		each(&w.events.enters, func(handler ui.PointerCrossingHandler) {
			handler(crossing)
		})
	case C.GK_EVENT_POINTER_LEAVE:
		crossing := ui.PointerCrossingEvent{X: event.x, Y: event.y}
		each(&w.events.leaves, func(handler ui.PointerCrossingHandler) {
			handler(crossing)
		})
	case C.GK_EVENT_IME:
		ime := ui.IMEEvent{
			Kind:      ui.IMEEventKind(event.a),
			Text:      event.text,
			Selection: utf16Range(event.text, event.b, event.c),
		}
		each(&w.events.imes, func(handler ui.IMEHandler) {
			handler(ime)
		})
	case C.GK_EVENT_TOUCH:
		touch := ui.TouchEvent{ID: event.a, Phase: ui.InputPhase(event.b), X: event.x, Y: event.y, Pressure: event.z}
		each(&w.events.touches, func(handler ui.TouchHandler) {
			handler(touch)
		})
	case C.GK_EVENT_PEN:
		pen := ui.PenEvent{
			Phase: ui.InputPhase(event.a), X: event.x, Y: event.y, Pressure: event.z, Rotation: event.w,
			Eraser: event.c&1 != 0, BarrelButton: event.c&2 != 0, DeviceID: event.b,
		}
		if fields := strings.Fields(event.text); len(fields) == 2 {
			pen.TiltX, _ = strconv.ParseFloat(fields[0], 64)
			pen.TiltY, _ = strconv.ParseFloat(fields[1], 64)
		}
		each(&w.events.pens, func(handler ui.PenHandler) {
			handler(pen)
		})
	case C.GK_EVENT_GESTURE:
		gesture := ui.GestureEvent{Kind: ui.GestureKind(event.a), Phase: ui.InputPhase(event.b), X: event.x, Y: event.y}
		switch gesture.Kind {
		case ui.GestureMagnify:
			gesture.Magnification = event.z
		case ui.GestureRotate:
			gesture.Rotation = event.z
		case ui.GestureSwipe:
			gesture.DeltaX, gesture.DeltaY = event.z, event.w
		}
		each(&w.events.gestures, func(handler ui.GestureHandler) {
			handler(gesture)
		})
	case C.GK_EVENT_DEVICE:
		w.dispatchDevice(deviceEvent(event))
	}
}

func deviceEvent(event cocoaEvent) ui.DeviceChangeEvent {
	return ui.DeviceChangeEvent{
		Kind:      ui.DeviceKind(event.a),
		DeviceID:  event.b,
		Connected: event.c != 0,
		Name:      event.text,
	}
}

func (w *Window) dispatchDevice(device ui.DeviceChangeEvent) {
	each(&w.events.devices, func(handler ui.DeviceChangeHandler) {
		handler(device)
	})
}

func (w *Window) dispatchPreferences() {
	if len(w.events.preferences.snapshot()) == 0 {
		return
	}
	preferences := w.SystemPreferences()
	each(&w.events.preferences, func(handler ui.SystemPreferencesHandler) {
		handler(preferences)
	})
}

// cocoaQuery answers whether a user close request may proceed.
func (w *Window) cocoaQuery(event cocoaEvent) bool {
	if event.kind != C.GK_EVENT_WINDOW_CLOSE_REQUEST {
		return true
	}
	allow := true
	each(&w.events.closeRequests, func(handler ui.CloseRequestHandler) {
		if !handler() {
			allow = false
		}
	})
	return allow
}

// utf16Range converts a UTF-16 range in text to a rune range.
func utf16Range(text string, start, length int) ui.TextRange {
	runeIndex := func(units int) int {
		count := 0
		for i, r := range []rune(text) {
			if count >= units {
				return i
			}
			if r >= 0x10000 {
				count += 2
			} else {
				count++
			}
		}
		return len([]rune(text))
	}
	return ui.TextRange{Start: runeIndex(start), End: runeIndex(start + length)}
}

// application receives application-wide events and forwards them to every
// window.
type application struct{}

var (
	applicationOnce   sync.Once
	applicationHandle cgo.Handle
)

func attachApplication() {
	applicationOnce.Do(func() {
		applicationHandle = cgo.NewHandle(&application{})
		C.gkCocoaAppAttach(C.uintptr_t(applicationHandle))
	})
}

func liveWindows() []*Window {
	windowsMu.RLock()
	defer windowsMu.RUnlock()
	result := make([]*Window, 0, len(windows))
	for window := range windows {
		result = append(result, window)
	}
	return result
}

func (*application) cocoaEvent(event cocoaEvent) {
	for _, window := range liveWindows() {
		switch event.kind {
		case C.GK_EVENT_APP_ACTIVE:
			each(&window.events.activated, func(handler ui.FocusHandler) {
				handler()
			})
		case C.GK_EVENT_APP_INACTIVE:
			each(&window.events.deactivated, func(handler ui.FocusHandler) {
				handler()
			})
		case C.GK_EVENT_APP_REOPEN:
			each(&window.events.reopens, func(handler ui.FocusHandler) {
				handler()
			})
		case C.GK_EVENT_PREFERENCES:
			window.dispatchPreferences()
		case C.GK_EVENT_DEVICE:
			window.dispatchDevice(deviceEvent(event))
		}
	}
}

func (*application) cocoaQuery(event cocoaEvent) bool {
	if event.kind != C.GK_EVENT_APP_QUIT_REQUEST {
		return true
	}
	allow := true
	for _, window := range liveWindows() {
		each(&window.events.quitRequests, func(handler ui.CloseRequestHandler) {
			if !handler() {
				allow = false
			}
		})
	}
	return allow
}
