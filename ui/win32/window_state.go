//go:build windows

package win32

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"sync"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/internal/unsupported"
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
	clipboard     listeners[ui.FocusHandler]
	notifications listeners[ui.NotificationActivationHandler]
	quitRequests  listeners[ui.CloseRequestHandler]
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
	window := w.nativeWindow()
	if window == nil {
		return ui.ErrClosed
	}
	window.setSizeLimits(minimum.Width, minimum.Height,
		maximum.Width, maximum.Height)
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
	if window := w.nativeWindow(); window != nil {
		window.setStyle(flags.resizable, flags.borderless)
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
	window := w.nativeWindow()
	if window == nil {
		return
	}
	window.setAlwaysOnTop(alwaysOnTop)
	w.mu.Lock()
	w.flags.alwaysOnTop = alwaysOnTop
	w.mu.Unlock()
}

func (w *Window) windowCall(call func(*nativeWindow) bool, failure string) error {
	window := w.nativeWindow()
	if window == nil {
		return ui.ErrClosed
	}
	if !call(window) {
		return errors.New("ui/win32: " + failure)
	}
	return nil
}

// Minimize implements ui.Window.Minimize. It returns an error for a fullscreen
// window, which the window manager does not minimize. A hidden window is
// shown first, because a window manager minimizes only mapped windows.
func (w *Window) Minimize() error {
	return w.windowCall(func(window *nativeWindow) bool {
		return window.minimize()
	},
		"the window could not be minimized")
}

// Maximize implements ui.Window.Maximize.
func (w *Window) Maximize() error {
	return w.windowCall(func(window *nativeWindow) bool {
		return window.maximize()
	},
		"the window could not be maximized")
}

// Restore implements ui.Window.Restore. Leaving fullscreen completes
// asynchronously.
func (w *Window) Restore() error {
	return w.windowCall(func(window *nativeWindow) bool {
		return window.restore()
	},
		"the window could not be restored")
}

// IsMinimized implements ui.Window.IsMinimized.
func (w *Window) IsMinimized() bool {
	window := w.nativeWindow()
	return window != nil && window.isMinimized()
}

// IsMaximized implements ui.Window.IsMaximized.
func (w *Window) IsMaximized() bool {
	window := w.nativeWindow()
	return window != nil && window.isMaximized()
}

// IsFullscreen implements ui.Window.IsFullscreen. It reports a completed
// transition into the fullscreen Space of the window.
func (w *Window) IsFullscreen() bool {
	window := w.nativeWindow()
	return window != nil && window.isFullscreen()
}

// OnFullscreenChange reports completed fullscreen transitions.
func (w *Window) OnFullscreenChange(handler ui.FullscreenHandler) ui.Unsubscribe {
	return subscribe(&w.events.fullscreens, handler, handler == nil)
}

// SetFullscreen asks the window manager for fullscreen. The transition
// completes asynchronously: when the window manager reports the state and the
// content has its new size. It returns ui.ErrUnavailable while the session
// is locked, because the window manager does not show windows then.
func (w *Window) SetFullscreen(fullscreen bool) error {
	if w.Kind() == ui.WindowUtility && fullscreen {
		return errors.New("ui: utility window cannot be fullscreen")
	}
	window := w.nativeWindow()
	if window == nil {
		return ui.ErrClosed
	}
	if !window.setFullscreen(fullscreen) {
		return fmt.Errorf("ui/win32: fullscreen needs an unlocked, awake display: %w", ui.ErrUnavailable)
	}
	return nil
}

// canBecomeKey reports whether Windows lets the window take keyboard focus.
func (w *Window) canBecomeKey() bool {
	window := w.nativeWindow()
	return window != nil && window.canBecomeKey()
}

// Screen implements ui.Window.Screen.
func (w *Window) Screen() ui.Screen {
	window := w.nativeWindow()
	if window == nil {
		return ui.Screen{}
	}
	screen := window.screen()
	return ui.Screen{
		Name: screen.name,
		Bounds: ui.Rect{
			Position: ui.Point{X: int(screen.x), Y: int(screen.y)},
			Size:     ui.Size{Width: int(screen.width), Height: int(screen.height)},
		},
		WorkArea: ui.Rect{
			Position: ui.Point{X: int(screen.workX), Y: int(screen.workY)},
			Size:     ui.Size{Width: int(screen.workWidth), Height: int(screen.workHeight)},
		},
		ScaleFactor: screen.scale,
		DPI:         screen.dpi,
		RefreshRate: screen.refresh,
	}
}

// SystemPreferences implements ui.Window.SystemPreferences.
func (w *Window) SystemPreferences() ui.SystemPreferences {
	return systemPreferences(w.nativeWindow())
}

func systemPreferences(window *nativeWindow) ui.SystemPreferences {
	values := systemPreferenceValuesOf(window)
	channel := func(value float32) uint8 {
		return uint8(math.Round(math.Max(0, math.Min(1, float64(value))) * 255))
	}
	accent := values.accent
	return ui.SystemPreferences{
		ResolvedTheme:       ui.Theme(values.theme),
		TextScale:           values.textScale,
		HighContrast:        values.highContrast,
		ReducedMotion:       values.reduceMotion,
		ReducedTransparency: values.reduceTransparency,
		AccentColor:         color.RGBA{R: channel(accent.r), G: channel(accent.g), B: channel(accent.b), A: channel(accent.a)},
		Locale:              values.locale,
		RightToLeft:         values.rightToLeft,
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
	if window := w.nativeWindow(); window != nil {
		window.setAcceptFiles(len(w.events.fileDrops.snapshot()) > 0)
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

// OnReopen panics with an error that wraps ui.ErrNotSupported. Windows
// starts a new process when the user opens a running application again, so
// the running application gets no reopen request.
func (w *Window) OnReopen(handler ui.FocusHandler) ui.Unsubscribe {
	unsupported.Panic("ui/win32: Window.OnReopen")
	return noOpUnsubscribe
}

// OnQuitRequest implements ui.Window.OnQuitRequest. Windows asks the
// application to quit when the user signs out or shuts down; a handler that
// returns false asks Windows to wait.
func (w *Window) OnQuitRequest(handler ui.CloseRequestHandler) ui.Unsubscribe {
	attachApplication()
	return subscribe(&w.events.quitRequests, handler, handler == nil)
}

func each[T any](list *listeners[T], call func(T)) {
	for _, handler := range list.snapshot() {
		call(handler)
	}
}

// receiveEvent receives the events of the native window.
func (w *Window) receiveEvent(event any) {
	switch event := event.(type) {
	case windowResizeEvent:
		each(&w.events.resizes, func(handler ui.ResizeHandler) {
			handler(event.size)
		})
	case fullscreenEvent:
		each(&w.events.fullscreens, func(handler ui.FullscreenHandler) {
			handler(event.fullscreen)
		})
	case focusEvent:
		w.mu.RLock()
		bar := w.menuBar
		w.mu.RUnlock()
		if bar != nil {
			bar.install()
		}
		each(&w.events.focuses, func(handler ui.FocusHandler) {
			handler()
		})
	case blurEvent:
		each(&w.events.blurs, func(handler ui.FocusHandler) {
			handler()
		})
	case themeEvent:
		each(&w.events.themes, func(handler ui.ThemeChangeHandler) {
			handler(event.theme)
		})
		w.dispatchPreferences()
	case fileDropEvent:
		each(&w.events.fileDrops, func(handler ui.FileDropHandler) {
			handler(append([]string(nil), event.paths...))
		})
	case ui.PointerMoveEvent:
		each(&w.events.pointerMoves, func(handler ui.PointerMoveHandler) {
			handler(event)
		})
	case pointerEnterEvent:
		each(&w.events.enters, func(handler ui.PointerCrossingHandler) {
			handler(event.crossing)
		})
	case pointerLeaveEvent:
		each(&w.events.leaves, func(handler ui.PointerCrossingHandler) {
			handler(event.crossing)
		})
	}
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

// answerQuery asks the close request handlers whether the window can close.
func (w *Window) answerQuery(query any) bool {
	if _, ok := query.(closeRequest); !ok {
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

var applicationOnce sync.Once

func attachApplication() {
	applicationOnce.Do(func() {
		setAppReceiver(&application{})
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

// answerQuery asks the quit request handlers of every window whether the
// application can quit.
func (*application) answerQuery(query any) bool {
	if _, ok := query.(quitRequest); !ok {
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

func (*application) receiveEvent(event any) {
	activation, ok := event.(appActivationEvent)
	if !ok {
		return
	}
	for _, window := range liveWindows() {
		if activation.active {
			each(&window.events.activated, func(handler ui.FocusHandler) {
				handler()
			})
		} else {
			each(&window.events.deactivated, func(handler ui.FocusHandler) {
				handler()
			})
		}
	}
}
