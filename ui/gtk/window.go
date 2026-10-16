//go:build linux && cgo

package gtk

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

// WindowOptions controls optional native-window behavior. Its zero value creates
// a visible, fixed-size, decorated window, like SDL_CreateWindow without flags.
type WindowOptions struct {
	// Kind is the role of the window. The zero value is ui.WindowNormal.
	Kind ui.WindowKind
	// Owner is the window that owns a utility window.
	Owner *Window
	// Position is the initial position. For a utility window, it is relative
	// to the content area of the owner.
	Position ui.Point
	// Resizable lets the user change the size of the window.
	Resizable bool
	// Hidden creates the window without showing it.
	Hidden bool
	// Borderless removes the title bar and the frame.
	Borderless bool
	// Maximized makes the window fill the work area of its screen.
	Maximized bool
	// AlwaysOnTop keeps the window above the normal windows.
	AlwaysOnTop bool
	// TransparentFramebuffer lets the alpha channel of the framebuffer show
	// the content behind the window.
	TransparentFramebuffer bool
	// Theme is the appearance of the window and its controls.
	Theme ui.Theme
	// Fullscreen requests the native fullscreen mode. A hidden window enters
	// it when it is first shown.
	Fullscreen bool
	// MinimumSize is the smallest content size. A zero dimension has no
	// limit.
	MinimumSize ui.Size
	// MaximumSize is the largest content size. A zero dimension has no
	// limit.
	MaximumSize ui.Size
}

// Window owns a native operating-system window. Window and event methods must
// be called from the goroutine that called CreateWindow, which must be locked
// to its operating-system thread. GTK runs on the thread that creates the
// first window.
type Window struct {
	mu        sync.RWMutex
	native    unsafe.Pointer
	title     string
	kind      ui.WindowKind
	owner     *Window
	children  map[*Window]struct{}
	destroyed bool

	// handle identifies this window to the C layer, which cannot hold a Go pointer.
	// Released by Destroy; see callbacks.go.
	handle uintptr

	// Event callbacks, all guarded by mu. See callbacks.go.
	keyCallback           KeyCallback
	charCallback          CharCallback
	scrollCallback        ScrollCallback
	pointerButtonCallback PointerButtonCallback

	// Level-triggered input state. Native event dispatch updates these before calling
	// user callbacks, so callbacks and polling always observe the same state.
	keys    [keyboard.KeyMenu + 1]keyboard.KeyAction
	buttons [pointer.ButtonCount]pointer.ButtonAction

	content ui.Widget
	theme   ui.Theme
	widgets map[*widgetBase]struct{}
	// resources are the native objects other than widgets that the window
	// created, such as menus and popovers. Destroy closes them.
	resources map[resource]struct{}
	flags     windowFlags
	events    windowEvents
	menuBar   *menuBar

	handlersMu sync.RWMutex
	nextID     uint64

	keyHandlers           map[uint64]ui.KeyHandler
	textHandlers          map[uint64]ui.TextHandler
	pointerButtonHandlers map[uint64]ui.PointerButtonHandler
	scrollHandlers        map[uint64]ui.ScrollHandler
	moveHandlers          map[uint64]ui.MoveHandler
}

var (
	windowsMu sync.RWMutex
	windows   = map[*Window]struct{}{}
)

// CreateWindow creates a native window without an OpenGL context. Its native
// handle is exposed through NativeSurface for GPU presentation.
func CreateWindow(title string, width, height int, opts *WindowOptions) (*Window, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("ui/gtk: window width and height must be positive")
	}
	var options WindowOptions
	if opts != nil {
		options = *opts
	}
	if err := options.Theme.Validate(); err != nil {
		return nil, err
	}
	if err := options.Kind.Validate(); err != nil {
		return nil, err
	}
	if options.Kind == ui.WindowUtility && options.Owner == nil {
		return nil, errors.New("ui/gtk: utility window requires an owner")
	}
	if options.Kind == ui.WindowNormal && options.Owner != nil {
		return nil, errors.New("ui/gtk: normal window cannot have an owner")
	}
	var ownerNative unsafe.Pointer
	if options.Owner != nil {
		ownerNative = options.Owner.nativePointer()
		if ownerNative == nil {
			return nil, ui.ErrClosed
		}
	}
	native, err := createNativeWindow(title, width, height, options, ownerNative)
	if err != nil {
		return nil, err
	}
	w := &Window{
		native:   native,
		title:    title,
		kind:     options.Kind,
		owner:    options.Owner,
		children: make(map[*Window]struct{}),
		theme:    options.Theme,
	}
	windowsMu.Lock()
	windows[w] = struct{}{}
	windowsMu.Unlock()
	if w.owner != nil {
		w.owner.mu.Lock()
		w.owner.children[w] = struct{}{}
		w.owner.mu.Unlock()
	}
	// Give the C layer a way to name this window when it dispatches an event.
	w.handle = newWindowHandle(w)
	setNativeWindowHandle(native, w.handle)
	setNativeWindowTheme(native, options.Theme)
	w.connectInput()
	if err := w.attachExtensions(options); err != nil {
		w.Destroy()
		return nil, err
	}
	return w, nil
}

// Destroy closes the native window. It is safe to call more than once.
func (w *Window) Destroy() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.destroyed {
		w.mu.Unlock()
		return
	}
	native := w.native
	handle := w.handle
	content := w.content
	resources := make([]resource, 0, len(w.resources))
	for owned := range w.resources {
		resources = append(resources, owned)
	}
	children := make([]*Window, 0, len(w.children))
	for child := range w.children {
		children = append(children, child)
	}
	owner := w.owner
	w.native = nil
	w.handle = 0
	w.content = nil
	w.resources = nil
	w.children = nil
	w.owner = nil
	w.destroyed = true
	w.mu.Unlock()

	if content != nil {
		content.Close()
	}
	for _, owned := range resources {
		owned.Close()
	}
	w.closeWidgets()
	for _, child := range children {
		child.Close()
	}
	if owner != nil {
		owner.mu.Lock()
		delete(owner.children, w)
		owner.mu.Unlock()
	}

	windowsMu.Lock()
	delete(windows, w)
	windowsMu.Unlock()
	// Clear the handle before the native window goes away, so an event still in flight
	// resolves to nothing rather than to a freed window.
	detachNativeWindow(native)
	setNativeWindowHandle(native, 0)
	deleteWindowHandle(handle)
	destroyNativeWindow(native)
}

// Close is equivalent to Destroy.
func (w *Window) Close() {
	w.Destroy()
}

// PollEvents processes pending operating-system events for this window.
func (w *Window) PollEvents() {
	if native := w.nativePointer(); native != nil {
		pollNativeWindow(native)
	}
	pollClipboard()
	if w.content != nil {
		bounds := ui.Rect{Size: w.Size()}
		if w.content.Bounds() != bounds {
			w.content.SetBounds(bounds)
		}
	}
}

// PollEvents processes events for every live GameKit window.
func PollEvents() {
	windowsMu.RLock()
	snapshot := make([]*Window, 0, len(windows))
	for w := range windows {
		snapshot = append(snapshot, w)
	}
	windowsMu.RUnlock()
	for _, w := range snapshot {
		w.PollEvents()
	}
}

// ShouldClose reports whether the user requested that the window close.
func (w *Window) ShouldClose() bool {
	if native := w.nativePointer(); native != nil {
		return nativeWindowShouldClose(native)
	}
	return true
}

// SetShouldClose changes the close-request flag without destroying the window.
func (w *Window) SetShouldClose(close bool) {
	if native := w.nativePointer(); native != nil {
		setNativeWindowShouldClose(native, close)
	}
}

// GetSize returns the native window's drawable client size in logical pixels.
func (w *Window) GetSize() (width, height int) {
	if native := w.nativePointer(); native != nil {
		return nativeWindowSize(native)
	}
	return 0, 0
}

// GetFramebufferSize returns the drawable size in physical pixels.
func (w *Window) GetFramebufferSize() (width, height int) {
	if native := w.nativePointer(); native != nil {
		return nativeWindowFramebufferSize(native)
	}
	return 0, 0
}

// Size returns the client size in logical pixels.
func (w *Window) Size() ui.Size {
	width, height := w.GetSize()
	return ui.Size{Width: width, Height: height}
}

// FramebufferSize returns the drawable size in physical pixels.
func (w *Window) FramebufferSize() ui.Size {
	width, height := w.GetFramebufferSize()
	return ui.Size{Width: width, Height: height}
}

// Kind reports the window's application role.
func (w *Window) Kind() ui.WindowKind {
	if w == nil {
		return ui.WindowNormal
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.kind
}

// SetSize changes the client-area size in logical pixels.
func (w *Window) SetSize(size ui.Size) error {
	if !size.IsValid() {
		return errors.New("ui/gtk: window size must be positive")
	}
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	setNativeWindowSize(native, size.Width, size.Height)
	return nil
}

// Position reports the window's top-left position. A utility window's position
// is relative to its owner's content area.
func (w *Window) Position() ui.Point {
	native := w.nativePointer()
	if native == nil {
		return ui.Point{}
	}
	x, y := nativeWindowPosition(native)
	return ui.Point{X: x, Y: y}
}

// SetPosition moves the window. Utility-window coordinates are relative to the
// owner's content area.
func (w *Window) SetPosition(position ui.Point) {
	native := w.nativePointer()
	if native == nil {
		return
	}
	setNativeWindowPosition(native, position.X, position.Y)
}

// IsVisible reports whether the native window is currently shown.
func (w *Window) IsVisible() bool {
	native := w.nativePointer()
	return native != nil && nativeWindowVisible(native)
}

// SetVisible shows or hides the native window.
func (w *Window) SetVisible(visible bool) {
	native := w.nativePointer()
	if native == nil {
		return
	}
	setNativeWindowVisible(native, visible)
}

// BringToFront raises a visible window above other windows.
func (w *Window) BringToFront() {
	if native := w.nativePointer(); native != nil {
		bringNativeWindowToFront(native)
	}
}

// OnMove registers a handler for user-initiated window movement.
func (w *Window) OnMove(handler ui.MoveHandler) ui.Unsubscribe {
	if w == nil || handler == nil {
		return noOpUnsubscribe
	}
	w.handlersMu.Lock()
	if w.moveHandlers == nil {
		w.moveHandlers = make(map[uint64]ui.MoveHandler)
	}
	id := w.nextHandlerID()
	w.moveHandlers[id] = handler
	w.handlersMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			w.handlersMu.Lock()
			delete(w.moveHandlers, id)
			w.handlersMu.Unlock()
		})
	}
}

func (w *Window) dispatchMove(position ui.Point) {
	w.handlersMu.RLock()
	handlers := make([]ui.MoveHandler, 0, len(w.moveHandlers))
	for _, handler := range w.moveHandlers {
		handlers = append(handlers, handler)
	}
	w.handlersMu.RUnlock()
	for _, handler := range handlers {
		handler(position)
	}
}

// SetTitle changes the native window title.
func (w *Window) SetTitle(title string) {
	if native := w.nativePointer(); native != nil {
		setNativeWindowTitle(native, title)
		w.mu.Lock()
		w.title = title
		w.mu.Unlock()
	}
}

// GetTitle returns the last title assigned through GameKit.
func (w *Window) GetTitle() string {
	if w == nil {
		return ""
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.title
}

// Title returns the last title assigned through GameKit.
func (w *Window) Title() string {
	return w.GetTitle()
}

// Theme returns the window's requested native appearance.
func (w *Window) Theme() ui.Theme {
	if w == nil {
		return ui.ThemeSystem
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.theme
}

// SetTheme changes the native appearance of the window and its controls.
func (w *Window) SetTheme(theme ui.Theme) error {
	if err := theme.Validate(); err != nil {
		return err
	}
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	setNativeWindowTheme(native, theme)
	w.mu.Lock()
	w.theme = theme
	w.mu.Unlock()
	return nil
}

// GetNativeHandle returns the X11 Window of the window.
// Prefer passing the window to gpu.Backend.CreateSurface unless integrating
// another native API.
func (w *Window) GetNativeHandle() uintptr {
	if native := w.nativePointer(); native != nil {
		return nativeWindowHandle(native)
	}
	return 0
}

// GetNativeDisplay returns the Xlib Display of the window.
func (w *Window) GetNativeDisplay() unsafe.Pointer {
	if native := w.nativePointer(); native != nil {
		return nativeWindowDisplay(native)
	}
	return nil
}

func (w *Window) nativePointer() unsafe.Pointer {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.native
}
