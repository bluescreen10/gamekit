//go:build windows

package win32

import (
	"errors"
	stdimage "image"
	"image/draw"
	"math"
	"sync"
	"sync/atomic"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/ui"
)

type nativeWidget interface {
	ui.Widget
	base() *widgetBase
}

type widgetBase struct {
	window *Window
	native *nativeView

	mu       sync.RWMutex
	bounds   ui.Rect
	parent   nativeWidget
	visible  bool
	enabled  bool
	closed   bool
	contract widgetContract
}

func newWidgetBase(window *Window, native *nativeView, role ui.AccessibilityRole) *widgetBase {
	base := &widgetBase{
		window:  window,
		native:  native,
		visible: true,
		enabled: true,
	}
	base.attachContract(role)
	return base
}

func (w *widgetBase) base() *widgetBase {
	return w
}

func (w *widgetBase) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	native := w.native
	w.native = nil
	w.closed = true
	w.mu.Unlock()

	w.detachContract(native)
	native.clearAction()
	native.destroy()
}

// closeComponent closes the component that owns the widget, so that the
// component also releases its own native objects.
func (w *widgetBase) closeComponent() {
	if component, ok := w.contract.owner.(resource); ok {
		component.Close()
	}
	w.Close()
}

func (w *widgetBase) Bounds() ui.Rect {
	if w == nil {
		return ui.Rect{}
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.bounds
}

func (w *widgetBase) SetBounds(bounds ui.Rect) {
	if w == nil {
		return
	}
	bounds.Size.Width = max(0, bounds.Size.Width)
	bounds.Size.Height = max(0, bounds.Size.Height)

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	previous := w.bounds.Size
	w.bounds = bounds
	native := w.native
	w.mu.Unlock()

	native.setFrame(bounds.Position.X,
		bounds.Position.Y,
		bounds.Size.Width,
		bounds.Size.Height,
	)
	if bounds.Size != previous {
		w.dispatchResize(bounds.Size)
	}
}

func (w *widgetBase) PreferredSize() ui.Size {
	native := w.nativeView()
	if native == nil {
		return ui.Size{}
	}
	width, height := native.preferredSize()
	return ui.Size{Width: int(width), Height: int(height)}
}

func (w *widgetBase) IsVisible() bool {
	if w == nil {
		return false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.visible && !w.closed
}

func (w *widgetBase) SetVisible(visible bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	changed := w.visible != visible
	w.visible = visible
	native := w.native
	w.mu.Unlock()

	native.setVisible(visible)
	if changed {
		// A hidden widget measures as empty, so the parent must lay out
		// again when the widget shows.
		w.relayoutParent()
	}
}

func (w *widgetBase) IsEnabled() bool {
	if w == nil {
		return false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.enabled && !w.closed
}

func (w *widgetBase) SetEnabled(enabled bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.enabled = enabled
	native := w.native
	w.mu.Unlock()

	native.setEnabled(enabled)
}

func (w *widgetBase) nativeView() *nativeView {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.native
}

type listeners[T any] struct {
	mu       sync.RWMutex
	nextID   uint64
	handlers map[uint64]T
}

func (l *listeners[T]) add(handler T) ui.Unsubscribe {
	l.mu.Lock()
	l.nextID++
	id := l.nextID
	if l.handlers == nil {
		l.handlers = make(map[uint64]T)
	}
	l.handlers[id] = handler
	l.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.handlers, id)
			l.mu.Unlock()
		})
	}
}

func (l *listeners[T]) snapshot() []T {
	l.mu.RLock()
	defer l.mu.RUnlock()

	handlers := make([]T, 0, len(l.handlers))
	for _, handler := range l.handlers {
		handlers = append(handlers, handler)
	}
	return handlers
}

type layout struct {
	*widgetBase

	mu          sync.RWMutex
	spec        ui.LayoutSpec
	arrangement arrangement
}

// maximumArrangementPasses limits how often a container arranges its children
// again in one arrangement.
const maximumArrangementPasses = 4

// arrangement tracks a running arrangement of a container's children. A child
// whose preferred size changes when it gets its bounds, such as a group box
// whose subtitle wraps at the new width, asks its parent to arrange again
// while the parent arranges. The positions that the running pass computed are
// then out of date, so the pass runs again when it ends, instead of nesting.
type arrangement struct {
	running bool
	again   bool
}

// run calls arrange, and calls it again while children ask for it.
func (a *arrangement) run(arrange func()) {
	if a.running {
		a.again = true
		return
	}
	a.running = true
	// The windows move without drawing until the outermost pass ends, and
	// then draw once.
	beginBatch()
	defer func() {
		a.running = false
		endBatch()
	}()
	for range maximumArrangementPasses {
		a.again = false
		arrange()
		if !a.again {
			return
		}
	}
}

func (l *layout) base() *widgetBase {
	return l.widgetBase
}

func (l *layout) Close() {
	if l == nil {
		return
	}
	for _, child := range l.Children() {
		child.Close()
	}
	l.widgetBase.Close()
}

func (l *layout) relayout() {
	l.applyLayout(l.Bounds().Size)
}

func (l *layout) SetBounds(bounds ui.Rect) {
	l.widgetBase.SetBounds(bounds)
	l.applyLayout(bounds.Size)
}

func (l *layout) PreferredSize() ui.Size {
	l.mu.RLock()
	spec := copyLayoutSpec(l.spec)
	l.mu.RUnlock()

	size, err := spec.PreferredSize()
	if err != nil {
		return ui.Size{}
	}
	return size
}

func (l *layout) Spec() ui.LayoutSpec {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return copyLayoutSpec(l.spec)
}

func (l *layout) SetSpec(spec ui.LayoutSpec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	children, err := l.validateChildren(spec)
	if err != nil {
		return err
	}

	l.mu.Lock()
	previous := layoutChildren(l.spec)
	l.spec = copyLayoutSpec(spec)
	l.mu.Unlock()

	for _, child := range previous {
		if containsWidget(children, child) {
			continue
		}
		base := child.base()
		base.parent = nil
		l.window.nativeWindow().attachToWindow(base.nativeView())
	}
	for _, child := range children {
		base := child.base()
		base.parent = l
		base.nativeView().setParent(l.nativeView())
	}

	l.applyLayout(l.Bounds().Size)
	return nil
}

func (l *layout) Children() []ui.Widget {
	l.mu.RLock()
	defer l.mu.RUnlock()

	children := layoutChildren(l.spec)
	result := make([]ui.Widget, len(children))
	for i, child := range children {
		result[i] = child
	}
	return result
}

func (l *layout) validateChildren(spec ui.LayoutSpec) ([]nativeWidget, error) {
	children := make([]nativeWidget, 0, len(spec.Items))
	for _, item := range spec.Items {
		if item.Widget == nil {
			continue
		}
		child, ok := item.Widget.(nativeWidget)
		if !ok {
			return nil, errors.New("ui/win32: layout child belongs to another backend")
		}
		children = append(children, child)
	}
	seen := make(map[*widgetBase]struct{}, len(children))
	for _, child := range children {
		base := child.base()
		if base.window != l.window {
			return nil, errors.New("ui/win32: layout child belongs to another window")
		}
		if base.closed {
			return nil, ui.ErrClosed
		}
		if base.parent != nil && base.parent.base() != l.widgetBase {
			return nil, errors.New("ui/win32: layout child already has a parent")
		}
		if l.window.isContent(base) {
			return nil, errors.New("ui/win32: layout child is the window content")
		}
		if _, exists := seen[base]; exists {
			return nil, errors.New("ui/win32: layout contains the same child more than once")
		}
		seen[base] = struct{}{}
		for ancestor := nativeWidget(l); ancestor != nil; ancestor = ancestor.base().parent {
			if base == ancestor.base() {
				return nil, errors.New("ui/win32: layout cycle")
			}
		}
	}
	return children, nil
}

func (l *layout) applyLayout(size ui.Size) {
	l.arrangement.run(func() {
		l.mu.RLock()
		spec := copyLayoutSpec(l.spec)
		l.mu.RUnlock()

		positions, err := spec.Resolve(size)
		if err != nil {
			return
		}
		for i, item := range spec.Items {
			if item.Widget != nil {
				item.Widget.SetBounds(positions[i])
			}
		}
	})
}

func layoutChildren(spec ui.LayoutSpec) []nativeWidget {
	children := make([]nativeWidget, 0, len(spec.Items))
	for _, item := range spec.Items {
		if item.Widget == nil {
			continue
		}
		child, ok := item.Widget.(nativeWidget)
		if !ok {
			continue
		}
		children = append(children, child)
	}
	return children
}

func containsWidget(widgets []nativeWidget, target nativeWidget) bool {
	for _, widget := range widgets {
		if widget.base() == target.base() {
			return true
		}
	}
	return false
}

func copyLayoutSpec(spec ui.LayoutSpec) ui.LayoutSpec {
	spec.Items = append([]ui.LayoutItem(nil), spec.Items...)
	return spec
}

type label struct {
	*widgetBase
	state labelState
}

func (l *label) Text() string {
	return nativeText(l.nativeView())
}

func (l *label) SetText(text string) {
	setNativeText(l.nativeView(), text)
	l.announceText(text)
}

type button struct {
	*widgetBase
	state  buttonState
	clicks listeners[ui.ClickHandler]
}

func (b *button) Text() string {
	return nativeText(b.nativeView())
}

func (b *button) SetText(text string) {
	setNativeText(b.nativeView(), text)
	b.reapplyStyle()
	b.announceText(text)
}

func (b *button) OnClick(handler ui.ClickHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return b.clicks.add(handler)
}

func (b *button) receiveEvent(event any) {
	if _, ok := event.(clickEvent); !ok || b.state.busy {
		return
	}
	for _, handler := range b.clicks.snapshot() {
		handler()
	}
}

type textField struct {
	*widgetBase
	state   textFieldState
	changes listeners[ui.TextChangeHandler]
}

type textArea struct {
	*widgetBase
	state   textAreaState
	changes listeners[ui.TextChangeHandler]
}

func (t *textArea) Text() string {
	return nativeText(t.nativeView())
}

func (t *textArea) SetText(text string) {
	setNativeText(t.nativeView(), text)
}

func (t *textArea) Placeholder() string {
	native := t.nativeView()
	if native == nil {
		return ""
	}
	return native.placeholder
}

func (t *textArea) SetPlaceholder(placeholder string) {
	native := t.nativeView()
	if native == nil {
		return
	}
	native.setPlaceholder(placeholder)
}

func (t *textArea) IsReadOnly() bool {
	native := t.nativeView()
	return native != nil && native.readOnly
}

func (t *textArea) SetReadOnly(readOnly bool) {
	if native := t.nativeView(); native != nil {
		native.setReadOnly(readOnly)
	}
}

func (t *textArea) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}

func (t *textArea) receiveEvent(event any) {
	if _, ok := event.(textChangeEvent); !ok {
		return
	}
	text := t.Text()
	for _, handler := range t.changes.snapshot() {
		handler(text)
	}
}

type checkBox struct {
	*widgetBase
	state   checkBoxState
	changes listeners[ui.ToggleHandler]
}

func (c *checkBox) Text() string {
	return nativeText(c.nativeView())
}

func (c *checkBox) SetText(text string) {
	setNativeText(c.nativeView(), text)
	c.announceText(text)
}

func (c *checkBox) IsChecked() bool {
	native := c.nativeView()
	return native != nil && native.isChecked()
}

func (c *checkBox) SetChecked(checked bool) {
	if native := c.nativeView(); native != nil {
		native.setChecked(checked)
	}
}

func (c *checkBox) OnChange(handler ui.ToggleHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return c.changes.add(handler)
}

func (c *checkBox) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	checked := c.IsChecked()
	for _, handler := range c.changes.snapshot() {
		handler(checked)
	}
}

type selection struct {
	*widgetBase
	items   []string
	choices []ui.Choice
	changes listeners[ui.SelectionChangeHandler]
}

func (s *selection) Items() []string {
	return append([]string(nil), s.items...)
}

func (s *selection) SelectedIndex() int {
	native := s.nativeView()
	if native == nil {
		return -1
	}
	return int(native.selectionIndex())
}

func (s *selection) SetSelectedIndex(index int) error {
	if index < 0 || index >= len(s.items) {
		return errors.New("ui: selection is out of range")
	}
	native := s.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setSelectionIndex(index)
	return nil
}

func (s *selection) OnChange(handler ui.SelectionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}

func (s *selection) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	index := s.SelectedIndex()
	for _, handler := range s.changes.snapshot() {
		handler(index)
	}
}

type radioGroup struct {
	*selection
	state radioGroupState
}

type selectControl struct {
	*selection
	state selectState
}

func (t *textField) Text() string {
	return nativeText(t.nativeView())
}

func (t *textField) SetText(text string) {
	setNativeText(t.nativeView(), text)
}

func (t *textField) Placeholder() string {
	native := t.nativeView()
	if native == nil {
		return ""
	}
	return native.placeholder
}

func (t *textField) SetPlaceholder(placeholder string) {
	native := t.nativeView()
	if native == nil {
		return
	}
	native.setPlaceholder(placeholder)
}

func (t *textField) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}

func (t *textField) receiveEvent(event any) {
	switch event.(type) {
	case submitEvent:
		text := t.Text()
		each(&t.state.submits, func(handler ui.TextChangeHandler) {
			handler(text)
		})
	case textChangeEvent:
		t.enforceMaximumLength()
		text := t.Text()
		for _, handler := range t.changes.snapshot() {
			handler(text)
		}
	}
}

type slider struct {
	*widgetBase
	state                  sliderState
	minimum, maximum, step float64
	changes                listeners[ui.ValueChangeHandler]
}

func (s *slider) Minimum() float64 {
	return s.minimum
}

func (s *slider) Maximum() float64 {
	return s.maximum
}

func (s *slider) Step() float64 {
	return s.step
}

type progressBar struct {
	*widgetBase
	state progressState
}

func (p *progressBar) Value() float64 {
	native := p.nativeView()
	if native == nil {
		return 0
	}
	return float64(native.progressFraction())
}

func (p *progressBar) SetValue(value float64) {
	if native := p.nativeView(); native != nil {
		native.setProgressValue(value)
	}
}

type activityIndicator struct {
	*widgetBase
	state activityState
}

func (a *activityIndicator) IsRunning() bool {
	native := a.nativeView()
	return native != nil && native.isActivityRunning()
}

func (a *activityIndicator) SetRunning(running bool) {
	if native := a.nativeView(); native != nil {
		native.setActivityRunning(running)
	}
}

type separator struct {
	*widgetBase
	state     separatorState
	direction ui.LayoutDirection
}

func (s *separator) Direction() ui.LayoutDirection {
	return s.direction
}

type imageView struct {
	*widgetBase
	state   imageState
	scaling ui.ImageScaling
}

func (i *imageView) SetImage(source stdimage.Image) error {
	if source == nil || source.Bounds().Empty() {
		return errors.New("ui: image source is required and must not be empty")
	}
	native := i.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	bounds := source.Bounds()
	rgba := stdimage.NewRGBA(stdimage.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), source, bounds.Min, draw.Src)
	native.setImagePixels(rgba.Pix,
		rgba.Rect.Dx(), rgba.Rect.Dy(), rgba.Stride)
	// The size of the source changes the preferred size.
	i.relayoutParent()
	return nil
}

func (i *imageView) Scaling() ui.ImageScaling {
	return i.scaling
}

func (i *imageView) SetScaling(scaling ui.ImageScaling) error {
	if scaling > ui.ImageScaleStretch {
		return errors.New("ui: invalid image scaling")
	}
	native := i.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setImageScaling(int(scaling))
	i.scaling = scaling
	return nil
}

type link struct {
	*widgetBase
	state  linkState
	clicks listeners[ui.ClickHandler]
}

func (l *link) Text() string {
	return nativeText(l.nativeView())
}

func (l *link) SetText(text string) {
	setNativeText(l.nativeView(), text)
	l.announceText(text)
}

func (l *link) URL() string {
	native := l.nativeView()
	if native == nil {
		return ""
	}
	return native.linkURL()
}

func (l *link) SetURL(url string) {
	native := l.nativeView()
	if native == nil {
		return
	}
	native.setLinkURL(url)
}

func (l *link) OnClick(handler ui.ClickHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return l.clicks.add(handler)
}

func (l *link) receiveEvent(event any) {
	if _, ok := event.(clickEvent); !ok {
		return
	}
	for _, handler := range l.clicks.snapshot() {
		handler()
	}
	l.opened()
}

type tabs struct {
	*widgetBase
	state   tabsState
	stateMu sync.RWMutex
	items   []ui.TabItem
	changes listeners[ui.SelectionChangeHandler]
}

type scrollView struct {
	*widgetBase
	state   scrollState
	content nativeWidget
	axes    ui.ScrollAxes
}

func (s *scrollView) Close() {
	if s == nil {
		return
	}
	if s.content != nil {
		s.content.Close()
		s.content = nil
	}
	s.widgetBase.Close()
}

func (s *scrollView) SetBounds(bounds ui.Rect) {
	s.widgetBase.SetBounds(bounds)
	s.layoutContent()
}

func (s *scrollView) PreferredSize() ui.Size {
	return ui.Size{Width: 320, Height: 240}
}

func (s *scrollView) Content() ui.Widget {
	if s == nil || s.content == nil {
		return nil
	}
	return s.content
}

func (s *scrollView) ScrollOffset() ui.Point {
	native := s.nativeView()
	if native == nil {
		return ui.Point{}
	}
	x, y := native.scrollOffset()
	return ui.Point{X: int(x), Y: int(y)}
}

func (s *scrollView) SetScrollOffset(offset ui.Point) {
	switch s.axes {
	case ui.ScrollVertical:
		offset.X = 0
	case ui.ScrollHorizontal:
		offset.Y = 0
	}
	if native := s.nativeView(); native != nil {
		smooth := s.ScrollBehavior() == ui.ScrollSmooth
		native.setScrollPosition(max(0, offset.X), max(0, offset.Y), smooth)
	}
}

func (s *scrollView) layoutContent() {
	native := s.nativeView()
	if native == nil || s.content == nil {
		return
	}
	width, height := native.scrollViewportSize()
	viewport := ui.Size{Width: max(0, int(width)), Height: max(0, int(height))}
	if viewport.Width <= 1 {
		viewport.Width = s.Bounds().Size.Width
	}
	if viewport.Height <= 1 {
		viewport.Height = s.Bounds().Size.Height
	}
	preferred := s.content.PreferredSize()
	contentSize := viewport
	switch s.axes {
	case ui.ScrollVertical:
		contentSize.Height = max(viewport.Height, preferred.Height)
	case ui.ScrollHorizontal:
		contentSize.Width = max(viewport.Width, preferred.Width)
	case ui.ScrollBoth:
		contentSize.Width = max(viewport.Width, preferred.Width)
		contentSize.Height = max(viewport.Height, preferred.Height)
	}
	native.setScrollContentSize(contentSize.Width, contentSize.Height)
	s.content.SetBounds(ui.Rect{Size: contentSize})
}

func (t *tabs) Close() {
	if t == nil {
		return
	}
	for _, item := range t.Items() {
		item.Content.Close()
	}
	t.widgetBase.Close()
}

func (t *tabs) SetBounds(bounds ui.Rect) {
	t.widgetBase.SetBounds(bounds)
	t.layoutContent()
}

func (t *tabs) PreferredSize() ui.Size {
	t.stateMu.RLock()
	items := append([]ui.TabItem(nil), t.items...)
	t.stateMu.RUnlock()
	size := ui.Size{Width: 160, Height: 100}
	for _, item := range items {
		preferred := item.Content.PreferredSize()
		size.Width = max(size.Width, preferred.Width+16)
		size.Height = max(size.Height, preferred.Height+40)
	}
	return size
}

func (t *tabs) Items() []ui.TabItem {
	t.stateMu.RLock()
	defer t.stateMu.RUnlock()
	return append([]ui.TabItem(nil), t.items...)
}

func (t *tabs) SelectedIndex() int {
	native := t.nativeView()
	if native == nil {
		return -1
	}
	return int(native.selectedTab())
}

func (t *tabs) SetSelectedIndex(index int) error {
	t.stateMu.RLock()
	count := len(t.items)
	t.stateMu.RUnlock()
	if index < 0 || index >= count {
		return errors.New("ui: tab selection is out of range")
	}
	native := t.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.selectTab(index)
	t.layoutContent()
	return nil
}

func (t *tabs) OnChange(handler ui.SelectionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}

func (t *tabs) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	index := t.SelectedIndex()
	t.layoutContent()
	for _, handler := range t.changes.snapshot() {
		handler(index)
	}
}

func (t *tabs) layoutContent() {
	native := t.nativeView()
	if native == nil {
		return
	}
	width, height := native.tabContentSize()
	selected := t.SelectedIndex()
	for index, item := range t.Items() {
		item.Content.SetVisible(index == selected)
		if index == selected {
			item.Content.SetBounds(ui.Rect{Size: ui.Size{Width: int(width), Height: int(height)}})
		}
	}
}

func (s *slider) Value() float64 {
	native := s.nativeView()
	if native == nil {
		return 0
	}
	return float64(native.sliderValue())
}

func (s *slider) SetValue(value float64) error {
	if math.IsNaN(value) || value < s.minimum || value > s.maximum {
		return errors.New("ui: slider value must be within its range")
	}
	native := s.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setSliderValue(value)
	return nil
}

func (s *slider) OnChange(handler ui.ValueChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}

func (s *slider) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	value := s.Value()
	for _, handler := range s.changes.snapshot() {
		handler(value)
	}
}

type canvas struct {
	*widgetBase
	canvasPointer
	redraw atomic.Bool
	events canvasEvents
}

func (c *canvas) Close() {
	if c == nil {
		return
	}
	c.Reset()
	c.widgetBase.Close()
}

// PointerPosition returns the pointer in logical pixels from the canvas's top-left
// corner. It is outside the canvas's bounds when the pointer is.
func (c *canvas) PointerPosition() (x, y float64) {
	native := c.nativeView()
	if native == nil {
		return 0, 0
	}
	nativeX, nativeY := native.canvasPointerPosition()
	return float64(nativeX), float64(nativeY)
}

func (c *canvas) NativeSurface() gpu.NativeSurface {
	native := c.nativeView()
	if native == nil {
		return gpu.NativeSurface{}
	}
	handle := uintptr(native.outer)
	if handle == 0 {
		return gpu.NativeSurface{}
	}
	return gpu.NativeSurface{
		Kind:   gpu.NativeSurfaceWin32Window,
		Handle: handle,
	}
}

func (c *canvas) FramebufferSize() ui.Size {
	native := c.nativeView()
	if native == nil {
		return ui.Size{}
	}
	width, height := native.framebufferSize()
	return ui.Size{Width: int(width), Height: int(height)}
}

func (c *canvas) RequestRedraw() {
	c.redraw.Store(true)
}

func (c *canvas) TakeRedrawRequest() bool {
	return c.redraw.Swap(false)
}

type section struct {
	*widgetBase
	state sectionState

	stateMu  sync.RWMutex
	content  nativeWidget
	expanded bool
	changes  listeners[ui.ExpansionChangeHandler]
}

func (s *section) Close() {
	if s == nil {
		return
	}
	s.stateMu.Lock()
	content := s.content
	s.content = nil
	s.stateMu.Unlock()
	if content != nil {
		content.Close()
	}
	s.widgetBase.Close()
}

func (s *section) SetBounds(bounds ui.Rect) {
	s.widgetBase.SetBounds(bounds)
	s.layoutContent()
}

func (s *section) PreferredSize() ui.Size {
	s.stateMu.RLock()
	content := s.content
	expanded := s.expanded
	s.stateMu.RUnlock()
	if content == nil || !expanded {
		return ui.Size{Width: 120, Height: sectionHeaderHeight}
	}
	size := content.PreferredSize()
	return ui.Size{
		Width:  max(120, size.Width),
		Height: sectionHeaderHeight + size.Height,
	}
}

func (s *section) Title() string {
	return nativeText(s.nativeView())
}

func (s *section) SetTitle(title string) {
	setNativeText(s.nativeView(), title)
}

func (s *section) Content() ui.Widget {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if s.content == nil {
		return nil
	}
	return s.content
}

func (s *section) IsExpanded() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.expanded
}

func (s *section) SetExpanded(expanded bool) {
	s.stateMu.Lock()
	if s.expanded == expanded {
		s.stateMu.Unlock()
		return
	}
	s.expanded = expanded
	s.stateMu.Unlock()
	if native := s.nativeView(); native != nil {
		native.setSectionExpanded(expanded)
	}
	s.relayoutParent()
}

func (s *section) OnChange(handler ui.ExpansionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}

func (s *section) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	native := s.nativeView()
	if native == nil {
		return
	}
	expanded := native.isSectionExpanded()
	s.stateMu.Lock()
	s.expanded = expanded
	s.stateMu.Unlock()
	s.relayoutParent()
	for _, handler := range s.changes.snapshot() {
		handler(expanded)
	}
}

func (s *section) layoutContent() {
	s.stateMu.RLock()
	content := s.content
	expanded := s.expanded
	s.stateMu.RUnlock()
	if content == nil || !expanded {
		return
	}
	size := s.Bounds().Size
	content.SetBounds(ui.Rect{Size: ui.Size{
		Width:  size.Width,
		Height: max(0, size.Height-sectionHeaderHeight),
	}})
}

func (s *section) relayoutParent() {
	parent := s.base().parent
	if layout, ok := parent.(*layout); ok {
		layout.applyLayout(layout.Bounds().Size)
	}
	s.layoutContent()
}

func validateContainerChild(container nativeWidget, content ui.Widget) (nativeWidget, error) {
	child, ok := content.(nativeWidget)
	if !ok {
		return nil, errors.New("ui/win32: container content belongs to another backend")
	}
	base := child.base()
	owner := container.base()
	if base.window != owner.window {
		return nil, errors.New("ui/win32: container content belongs to another window")
	}
	if base.closed {
		return nil, ui.ErrClosed
	}
	if base.parent != nil && base.parent.base() != owner {
		return nil, errors.New("ui/win32: container content already has a parent")
	}
	if owner.window.isContent(base) {
		return nil, errors.New("ui/win32: container content is the window content")
	}
	for ancestor := container; ancestor != nil; ancestor = ancestor.base().parent {
		if base == ancestor.base() {
			return nil, errors.New("ui/win32: container cycle")
		}
	}
	return child, nil
}

// isContent reports whether base is the window's root content.
func (w *Window) isContent(base *widgetBase) bool {
	content, ok := w.content.(nativeWidget)
	return ok && content.base() == base
}

// SetContent makes content fill the native window's content area.
func (w *Window) SetContent(content ui.Widget) error {
	widget, ok := content.(nativeWidget)
	if !ok || widget.base().window != w {
		return errors.New("ui/win32: content belongs to another backend or window")
	}
	if widget.base().parent != nil {
		return errors.New("ui/win32: content already belongs to a layout")
	}
	if w.content != nil && w.content != content {
		w.content.SetVisible(false)
	}

	w.nativeWindow().attachToWindow(widget.base().nativeView())
	w.content = content
	content.SetVisible(true)
	content.SetBounds(ui.Rect{Size: w.Size()})
	return nil
}

// CreateLayout creates a native layout owned by the window.
func (w *Window) CreateLayout(spec ui.LayoutSpec) (ui.Layout, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	native := w.nativeWindow().newLayoutView()
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a layout")
	}
	result := &layout{widgetBase: newWidgetBase(w, native, ui.RoleGroup)}
	if err := result.SetSpec(spec); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

// CreateLabel creates a native label owned by the window.
func (w *Window) CreateLabel(text string) (ui.Label, error) {
	native := w.nativeWindow().newLabel(text)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a label")
	}
	return &label{widgetBase: newWidgetBase(w, native, ui.RoleLabel)}, nil
}

// CreateButton creates a native push button owned by the window.
func (w *Window) CreateButton(text string) (ui.Button, error) {
	result := &button{}
	native := w.nativeWindow().newButton(text)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a button")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleButton)
	result.contract.owner = result
	return result, nil
}

// CreateTextField creates a native single-line text editor owned by the window.
func (w *Window) CreateTextField() (ui.TextField, error) {
	result := &textField{}
	native := w.nativeWindow().newTextField()
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a text field")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleTextField)
	result.contract.owner = result
	return result, nil
}

// CreateTextArea creates a native multiline text editor owned by the window.
func (w *Window) CreateTextArea(options ui.TextAreaOptions) (ui.TextArea, error) {
	result := &textArea{}
	native := w.nativeWindow().newTextArea(options.Text, options.Placeholder,
		options.ReadOnly, options.Wrap)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a text area")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleTextArea)
	result.contract.owner = result
	return result, nil
}

// CreateCheckBox creates a native check box owned by the window.
func (w *Window) CreateCheckBox(text string, checked bool) (ui.CheckBox, error) {
	result := &checkBox{}
	native := w.nativeWindow().newCheckBox(text,
		checked)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a check box")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleCheckBox)
	result.contract.owner = result
	return result, nil
}

// CreateRadioGroup creates a native group of mutually exclusive choices.
func (w *Window) CreateRadioGroup(options ui.RadioGroupOptions) (ui.RadioGroup, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &radioGroup{selection: &selection{items: append([]string(nil), options.Items...), choices: choicesFromItems(options.Items)}}
	native := w.nativeWindow().newRadioGroup()
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a radio group")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleRadioGroup)
	result.contract.owner = result
	addSelectionItems(native, options.Items)
	native.setSelectionIndex(options.SelectedIndex)
	return result, nil
}

// CreateSelect creates a native drop-down selection control.
func (w *Window) CreateSelect(options ui.SelectOptions) (ui.Select, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &selectControl{selection: &selection{items: append([]string(nil), options.Items...), choices: choicesFromItems(options.Items)}}
	native := w.nativeWindow().newSelect()
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a select control")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleComboBox)
	result.contract.owner = result
	addSelectionItems(native, options.Items)
	native.setSelectionIndex(options.SelectedIndex)
	return result, nil
}

func addSelectionItems(native *nativeView, items []string) {
	for _, item := range items {
		native.addSelectionItem(item)
	}
}

// CreateSlider creates a native slider owned by the window.
func (w *Window) CreateSlider(options ui.SliderOptions) (ui.Slider, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &slider{minimum: options.Minimum, maximum: options.Maximum, step: options.Step}
	native := w.nativeWindow().newSlider(options.Minimum,
		options.Maximum,
		options.Value,
		options.Step,
	)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a slider")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleSlider)
	result.contract.owner = result
	return result, nil
}

// CreateProgressBar creates a native determinate progress indicator.
func (w *Window) CreateProgressBar(value float64) (ui.ProgressBar, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return nil, errors.New("ui: progress value must be between zero and one")
	}
	native := w.nativeWindow().newProgressBar(value)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a progress bar")
	}
	return &progressBar{widgetBase: newWidgetBase(w, native, ui.RoleProgressBar)}, nil
}

// CreateActivityIndicator creates a native indeterminate activity indicator.
func (w *Window) CreateActivityIndicator(running bool) (ui.ActivityIndicator, error) {
	native := w.nativeWindow().newActivityIndicator(running)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create an activity indicator")
	}
	return &activityIndicator{widgetBase: newWidgetBase(w, native, ui.RoleBusyIndicator)}, nil
}

// CreateSeparator creates a native visual divider.
func (w *Window) CreateSeparator(direction ui.LayoutDirection) (ui.Separator, error) {
	if direction != ui.Horizontal && direction != ui.Vertical {
		return nil, errors.New("ui: invalid separator direction")
	}
	native := w.nativeWindow().newSeparator(int(direction))
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a separator")
	}
	return &separator{widgetBase: newWidgetBase(w, native, ui.RoleSeparator), direction: direction}, nil
}

// CreateImage creates a native image view.
func (w *Window) CreateImage(options ui.ImageOptions) (ui.ImageView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	native := w.nativeWindow().newImageView(int(options.Scaling))
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create an image")
	}
	result := &imageView{widgetBase: newWidgetBase(w, native, ui.RoleImage), scaling: options.Scaling}
	if err := result.SetImage(options.Source); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

// CreateLink creates a native hyperlink control.
func (w *Window) CreateLink(text, url string) (ui.Link, error) {
	result := &link{}
	native := w.nativeWindow().newLink(text, url)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a link")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleLink)
	result.contract.owner = result
	return result, nil
}

// CreateTabs creates a native tab view.
func (w *Window) CreateTabs(options ui.TabsOptions) (ui.Tabs, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &tabs{}
	native := w.nativeWindow().newTabs()
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create tabs")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleTabList)
	result.contract.owner = result
	contents := make([]ui.Widget, len(options.Items))
	for i, item := range options.Items {
		contents[i] = item.Content
	}
	children, err := validateContainerChildren(result, contents)
	if err != nil {
		// Close only the tab view: the caller still owns the contents.
		result.widgetBase.Close()
		return nil, err
	}
	for i, item := range options.Items {
		child := children[i]
		child.base().parent = result
		added := native.addTab(item.Title, child.base().nativeView())
		if !added {
			for _, added := range children[:i+1] {
				orphan(added)
			}
			result.widgetBase.Close()
			return nil, errors.New("ui/win32: native backend could not add a tab")
		}
		result.items = append(result.items, item)
	}
	native.selectTab(options.SelectedIndex)
	result.layoutContent()
	return result, nil
}

// CreateScrollView creates a native scrolling viewport owned by the window.
func (w *Window) CreateScrollView(options ui.ScrollViewOptions) (ui.ScrollView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &scrollView{axes: options.Axes}
	native := w.nativeWindow().newScrollView(int(options.Axes))
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a scroll view")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleScrollView)
	content, err := validateContainerChild(result, options.Content)
	if err != nil {
		result.Close()
		return nil, err
	}
	result.content = content
	content.base().parent = result
	(content.base().nativeView()).setParent(native)
	result.layoutContent()
	return result, nil
}

// CreateCanvas creates a native control that a GPU backend can present to. The
// canvas receives the input of its view.
func (w *Window) CreateCanvas() (ui.Canvas, error) {
	result := &canvas{}
	native := w.nativeWindow().newCanvas(result)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a canvas")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleCanvas)
	return result, nil
}

// CreateSection creates a collapsible native section owned by the window.
func (w *Window) CreateSection(options ui.SectionOptions) (ui.Section, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	result := &section{expanded: options.Expanded}
	native := w.nativeWindow().newSection(options.Title,
		options.Expanded,
	)
	if native == nil {
		return nil, errors.New("ui/win32: native backend could not create a section")
	}
	result.widgetBase = newWidgetBase(w, native, ui.RoleDisclosure)
	result.contract.owner = result
	child, err := validateContainerChild(result, options.Content)
	if err != nil {
		result.Close()
		return nil, err
	}
	result.content = child
	child.base().parent = result
	(child.base().nativeView()).setParent(result.nativeView())
	return result, nil
}

func nativeText(native *nativeView) string {
	if native == nil {
		return ""
	}
	return native.currentText()
}

func setNativeText(native *nativeView, text string) {
	if native == nil {
		return
	}
	native.setText(text)
}

var _ ui.Layout = (*layout)(nil)
var _ ui.Label = (*label)(nil)
var _ ui.Button = (*button)(nil)
var _ ui.TextField = (*textField)(nil)
var _ ui.Slider = (*slider)(nil)
var _ ui.Canvas = (*canvas)(nil)
var _ ui.Section = (*section)(nil)
