//go:build windows

package win32

import (
	"errors"
	stdimage "image"
	"image/color"
	"image/draw"

	"github.com/bluescreen10/gamekit/ui"
)

// widgetContract is the state behind the common widget contract. Native
// changes are applied immediately; the Go copy answers getters exactly.
type widgetContract struct {
	// owner is the widget that receives the events of the native view, other
	// than focus events.
	owner         eventReceiver
	role          ui.AccessibilityRole
	accessibility ui.Accessibility
	minimum       ui.Size
	maximum       ui.Size
	margins       ui.Insets
	tooltip       string
	cursor        ui.Cursor
	style         ui.WidgetStyle
	contextMenu   ui.ContextMenu
	dragSource    ui.DragSource
	dropTarget    ui.DropTarget

	focuses listeners[ui.FocusHandler]
	blurs   listeners[ui.FocusHandler]
	resizes listeners[ui.ResizeHandler]
	drops   listeners[ui.DropHandler]
}

// attachContract makes the widget the receiver of the focus and drop events of
// its native view.
func (w *widgetBase) attachContract(role ui.AccessibilityRole) {
	w.contract.role = role
	w.native.receiver = w
	if w.window != nil {
		w.window.registerWidget(w)
	}
}

// detachContract stops the events of the native view before it is destroyed.
func (w *widgetBase) detachContract(native *nativeView) {
	if native.receiver == nil {
		return
	}
	native.disconnect()
	native.receiver = nil
	if w.window != nil {
		w.window.unregisterWidget(w)
	}
}

func (w *widgetBase) receiveEvent(event any) {
	switch event.(type) {
	case focusEvent:
		for _, handler := range w.contract.focuses.snapshot() {
			handler()
		}
	case blurEvent:
		for _, handler := range w.contract.blurs.snapshot() {
			handler()
		}
	default:
		send(w.contract.owner, event)
	}
}

// answerQuery forwards questions to the widget that owns the view.
func (w *widgetBase) answerQuery(query any) bool {
	return ask(w.contract.owner, query)
}

func (w *widgetBase) Accessibility() ui.Accessibility {
	w.mu.RLock()
	defer w.mu.RUnlock()
	metadata := w.contract.accessibility
	metadata.LabeledBy = cloneWidgets(metadata.LabeledBy)
	metadata.DescribedBy = cloneWidgets(metadata.DescribedBy)
	return metadata
}

// announceText asks assistive technologies to read text when the widget is a
// live region. Text changes become UI Automation notifications, as in an
// ARIA live region.
func (w *widgetBase) announceText(text string) {
	w.mu.RLock()
	live := w.contract.accessibility.Live
	w.mu.RUnlock()
	native := w.nativeView()
	if live == ui.LiveOff || native == nil {
		return
	}
	native.announce(text)
}

func cloneWidgets(widgets []ui.Widget) []ui.Widget {
	if widgets == nil {
		return nil
	}
	return append([]ui.Widget(nil), widgets...)
}

func (w *widgetBase) SetAccessibility(metadata ui.Accessibility) error {
	if err := metadata.Validate(); err != nil {
		return err
	}
	native := w.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	// Windows names a control by its own label, so the labels are only
	// checked.
	for _, widget := range metadata.LabeledBy {
		if _, err := w.relatedView(widget); err != nil {
			return err
		}
	}
	describedBy := make([]*nativeView, len(metadata.DescribedBy))
	for i, widget := range metadata.DescribedBy {
		view, err := w.relatedView(widget)
		if err != nil {
			return err
		}
		describedBy[i] = view
	}
	native.setAccessibility(metadata.ID, metadata.Name, metadata.Description, metadata.ValueText,
		metadata.IsRequired, metadata.IsHidden, metadata.IsInvalid, describedBy)

	metadata.LabeledBy = cloneWidgets(metadata.LabeledBy)
	metadata.DescribedBy = cloneWidgets(metadata.DescribedBy)
	w.mu.Lock()
	w.contract.accessibility = metadata
	w.mu.Unlock()
	return nil
}

// relatedView returns the native view of a widget named in a relationship.
func (w *widgetBase) relatedView(widget ui.Widget) (*nativeView, error) {
	related, ok := widget.(nativeWidget)
	if !ok || related.base().window != w.window {
		return nil, errors.New("ui/win32: accessibility relationship names a widget from another window or backend")
	}
	view := related.base().nativeView()
	if view == nil {
		return nil, ui.ErrClosed
	}
	return view, nil
}

func (w *widgetBase) AccessibilityRole() ui.AccessibilityRole {
	return w.contract.role
}

func (w *widgetBase) Focus() error {
	native := w.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	if !w.IsVisible() || !w.IsEnabled() {
		return errors.New("ui/win32: hidden or disabled widgets cannot take focus")
	}
	if !native.focus() {
		return errors.New("ui/win32: widget cannot take keyboard focus")
	}
	if w.window != nil {
		w.window.trackFocus()
	}
	return nil
}

func (w *widgetBase) IsFocused() bool {
	native := w.nativeView()
	return native != nil && native.isFocused()
}

func (w *widgetBase) OnFocus(handler ui.FocusHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return w.contract.focuses.add(handler)
}

func (w *widgetBase) OnBlur(handler ui.FocusHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return w.contract.blurs.add(handler)
}

func (w *widgetBase) MinimumSize() ui.Size {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.contract.minimum
}

func (w *widgetBase) SetMinimumSize(size ui.Size) {
	w.mu.Lock()
	w.contract.minimum = ui.Size{Width: max(0, size.Width), Height: max(0, size.Height)}
	w.mu.Unlock()
	w.relayoutParent()
}

func (w *widgetBase) MaximumSize() ui.Size {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.contract.maximum
}

func (w *widgetBase) SetMaximumSize(size ui.Size) {
	w.mu.Lock()
	w.contract.maximum = ui.Size{Width: max(0, size.Width), Height: max(0, size.Height)}
	w.mu.Unlock()
	w.relayoutParent()
}

func (w *widgetBase) Margins() ui.Insets {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.contract.margins
}

func (w *widgetBase) SetMargins(margins ui.Insets) {
	w.mu.Lock()
	w.contract.margins = margins
	w.mu.Unlock()
	w.relayoutParent()
}

// relayoutParent reapplies the parent layout after a size hint changes.
func (w *widgetBase) relayoutParent() {
	if relayout, ok := w.parent.(interface{ relayout() }); ok {
		relayout.relayout()
	}
}

func (w *widgetBase) OnResize(handler ui.ResizeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return w.contract.resizes.add(handler)
}

func (w *widgetBase) dispatchResize(size ui.Size) {
	for _, handler := range w.contract.resizes.snapshot() {
		handler(size)
	}
}

func (w *widgetBase) Tooltip() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.contract.tooltip
}

func (w *widgetBase) SetTooltip(text string) {
	native := w.nativeView()
	if native == nil {
		return
	}
	native.setTooltip(text)
	w.mu.Lock()
	w.contract.tooltip = text
	w.mu.Unlock()
}

func (w *widgetBase) Cursor() ui.Cursor {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.contract.cursor
}

func (w *widgetBase) SetCursor(cursor ui.Cursor) error {
	if err := cursor.Validate(); err != nil {
		return err
	}
	native := w.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setCursor(int(cursor))
	w.mu.Lock()
	w.contract.cursor = cursor
	w.mu.Unlock()
	return nil
}

func (w *widgetBase) Style() ui.WidgetStyle {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.contract.style
}

func (w *widgetBase) SetStyle(style ui.WidgetStyle) error {
	if err := style.Validate(); err != nil {
		return err
	}
	native := w.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	foreground, hasForeground := rgba(style.ForegroundColor)
	background, hasBackground := rgba(style.BackgroundColor)
	var foregroundRGBA, backgroundRGBA *[4]float64
	if hasForeground {
		foregroundRGBA = &foreground
	}
	if hasBackground {
		backgroundRGBA = &background
	}
	if !native.setStyle(style.Font.Family, style.Font.Size, int(style.Font.Weight), style.Font.Italic,
		int(style.Foreground), int(style.Background), foregroundRGBA, backgroundRGBA,
		float64(style.CornerRadius)) {
		return errors.New("ui/win32: font family is not installed")
	}
	w.mu.Lock()
	w.contract.style = style
	w.mu.Unlock()
	// A font or a fill changes the preferred size.
	w.relayoutParent()
	return nil
}

// rgba converts value to red, green, blue, and alpha from zero to one. It
// reports false for a nil color.
func rgba(value color.Color) ([4]float64, bool) {
	if value == nil {
		return [4]float64{}, false
	}
	red, green, blue, alpha := value.RGBA()
	if alpha == 0 {
		return [4]float64{}, true
	}
	// RGBA returns premultiplied 16-bit components; CSS expects straight
	// alpha.
	return [4]float64{
		float64(red) / float64(alpha),
		float64(green) / float64(alpha),
		float64(blue) / float64(alpha),
		float64(alpha) / 0xffff,
	}, true
}

// reapplyStyle applies the style again after a change that resets native
// attributes, such as a new button title.
func (w *widgetBase) reapplyStyle() {
	style := w.Style()
	if style.ForegroundColor != nil || style.BackgroundColor != nil {
		_ = w.SetStyle(style)
	}
}

func (w *widgetBase) SetContextMenu(menu ui.ContextMenu) error {
	native := w.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	var nativeMenu *nativeMenu
	if menu != nil {
		owned, ok := menu.(*contextMenu)
		if !ok {
			return errors.New("ui/win32: context menu belongs to another backend")
		}
		nativeMenu = owned.nativeMenu()
		if nativeMenu == nil {
			return ui.ErrClosed
		}
	}
	native.setMenu(nativeMenu)
	w.mu.Lock()
	w.contract.contextMenu = menu
	w.mu.Unlock()
	return nil
}

func (w *widgetBase) SetDragSource(source ui.DragSource) error {
	native := w.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	if source.Operations&^(ui.DragCopy|ui.DragMove|ui.DragLink) != 0 {
		return errors.New("ui: invalid drag operations")
	}
	native.setDragSource(source)
	w.mu.Lock()
	w.contract.dragSource = source
	w.mu.Unlock()
	return nil
}

func (w *widgetBase) SetDropTarget(target ui.DropTarget) error {
	native := w.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	if target.Operations&^(ui.DragCopy|ui.DragMove|ui.DragLink) != 0 {
		return errors.New("ui: invalid drop operations")
	}
	native.setDropTarget(target)
	target.Formats = append([]string(nil), target.Formats...)
	w.mu.Lock()
	w.contract.dropTarget = target
	w.mu.Unlock()
	return nil
}

func (w *widgetBase) OnDrop(handler ui.DropHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return w.contract.drops.add(handler)
}

// dispatchDrop reports data that the user dropped on the widget.
func (widget *widgetBase) dispatchDrop(event ui.DropEvent) {
	for _, handler := range widget.contract.drops.snapshot() {
		handler(event)
	}
}

// rgbaPixels converts an image to tightly packed non-premultiplied RGBA.
func rgbaPixels(source stdimage.Image) ([]byte, int, int) {
	bounds := source.Bounds()
	image := stdimage.NewNRGBA(stdimage.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(image, image.Bounds(), source, bounds.Min, draw.Src)
	return image.Pix, bounds.Dx(), bounds.Dy()
}
