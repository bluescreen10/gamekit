//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	stdimage "image"
	"image/color"
	"image/draw"
	"runtime/cgo"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// widgetContract is the state behind the common widget contract. Native
// changes are applied immediately; the Go copy answers getters exactly.
type widgetContract struct {
	// owner receives component events that GTK reports through the
	// widget's handle.
	owner         gtkReceiver
	role          ui.AccessibilityRole
	eventHandle   cgo.Handle
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

// attachContract gives the native view a handle so that GTK can report
// focus and drop events for the widget.
func (w *widgetBase) attachContract(role ui.AccessibilityRole) {
	w.contract.role = role
	w.contract.eventHandle = cgo.NewHandle(w)
	C.gkGTKSetHandle(w.native, C.uintptr_t(w.contract.eventHandle))
	if w.window != nil {
		w.window.registerWidget(w)
	}
}

// detachContract releases the handle before the native view is destroyed.
func (w *widgetBase) detachContract(native unsafe.Pointer) {
	if w.contract.eventHandle == 0 {
		return
	}
	handle := w.contract.eventHandle
	w.contract.eventHandle = 0
	C.gkGTKDisconnect(native)
	C.gkGTKSetHandle(native, 0)
	if w.window != nil {
		w.window.unregisterWidget(w, uintptr(handle))
	}
	handle.Delete()
}

func (w *widgetBase) gtkEvent(event gtkEvent) {
	switch event.kind {
	case C.GK_EVENT_FOCUS:
		for _, handler := range w.contract.focuses.snapshot() {
			handler()
		}
	case C.GK_EVENT_BLUR:
		for _, handler := range w.contract.blurs.snapshot() {
			handler()
		}
	default:
		if w.contract.owner != nil {
			w.contract.owner.gtkEvent(event)
		}
	}
}

// gtkQuery forwards decisions to the owning component.
func (w *widgetBase) gtkQuery(event gtkEvent) bool {
	if querier, ok := w.contract.owner.(gtkQuerier); ok {
		return querier.gtkQuery(event)
	}
	return true
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
// live region. GTK has no live-region property, so text changes become
// announcements, as in an ARIA live region.
func (w *widgetBase) announceText(text string) {
	w.mu.RLock()
	live := w.contract.accessibility.Live
	w.mu.RUnlock()
	native := w.nativePointer()
	if live == ui.LiveOff || native == nil {
		return
	}
	value := cString(text)
	defer freeCString(value)
	C.gkGTKAnnounce(native, value, C.int(live))
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
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	var labeledBy unsafe.Pointer
	if len(metadata.LabeledBy) > 0 {
		view, err := w.relatedView(metadata.LabeledBy[0])
		if err != nil {
			return err
		}
		labeledBy = view
	}
	if len(metadata.LabeledBy) > 1 {
		for _, widget := range metadata.LabeledBy[1:] {
			if _, err := w.relatedView(widget); err != nil {
				return err
			}
		}
	}
	describedBy := C.malloc(C.size_t(max(1, len(metadata.DescribedBy))) * C.size_t(unsafe.Sizeof(uintptr(0))))
	defer C.free(describedBy)
	views := unsafe.Slice((*unsafe.Pointer)(describedBy), max(1, len(metadata.DescribedBy)))
	for i, widget := range metadata.DescribedBy {
		view, err := w.relatedView(widget)
		if err != nil {
			return err
		}
		views[i] = view
	}

	identifier := cString(metadata.ID)
	name := cString(metadata.Name)
	description := cString(metadata.Description)
	value := cString(metadata.ValueText)
	defer freeCString(identifier)
	defer freeCString(name)
	defer freeCString(description)
	defer freeCString(value)
	C.gkGTKSetAccessibility(native, identifier, name, description, value,
		boolInt(metadata.IsRequired), boolInt(metadata.IsHidden), boolInt(metadata.IsInvalid),
		labeledBy, (*unsafe.Pointer)(describedBy), C.int(len(metadata.DescribedBy)))

	metadata.LabeledBy = cloneWidgets(metadata.LabeledBy)
	metadata.DescribedBy = cloneWidgets(metadata.DescribedBy)
	w.mu.Lock()
	w.contract.accessibility = metadata
	w.mu.Unlock()
	return nil
}

// relatedView returns the native view of a widget named in a relationship.
func (w *widgetBase) relatedView(widget ui.Widget) (unsafe.Pointer, error) {
	related, ok := widget.(nativeWidget)
	if !ok || related.base().window != w.window {
		return nil, errors.New("ui/gtk: accessibility relationship names a widget from another window or backend")
	}
	view := related.base().nativePointer()
	if view == nil {
		return nil, ui.ErrClosed
	}
	return view, nil
}

func (w *widgetBase) AccessibilityRole() ui.AccessibilityRole {
	return w.contract.role
}

func (w *widgetBase) Focus() error {
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	if !w.IsVisible() || !w.IsEnabled() {
		return errors.New("ui/gtk: hidden or disabled widgets cannot take focus")
	}
	if C.gkGTKFocus(native) == 0 {
		return errors.New("ui/gtk: widget cannot take keyboard focus")
	}
	if w.window != nil {
		w.window.trackFocus()
	}
	return nil
}

func (w *widgetBase) IsFocused() bool {
	native := w.nativePointer()
	return native != nil && C.gkGTKIsFocused(native) != 0
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
	native := w.nativePointer()
	if native == nil {
		return
	}
	value := cString(text)
	defer freeCString(value)
	C.gkGTKSetTooltip(native, value)
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
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkGTKSetCursor(native, C.int(cursor))
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
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	family := cString(style.Font.Family)
	defer freeCString(family)
	foreground, hasForeground := rgba(style.ForegroundColor)
	background, hasBackground := rgba(style.BackgroundColor)
	var foregroundPointer, backgroundPointer *C.double
	if hasForeground {
		foregroundPointer = &foreground[0]
	}
	if hasBackground {
		backgroundPointer = &background[0]
	}
	if C.gkGTKSetStyle(native, family, C.double(style.Font.Size), C.int(style.Font.Weight),
		boolInt(style.Font.Italic), C.int(style.Foreground), C.int(style.Background),
		foregroundPointer, backgroundPointer, C.double(style.CornerRadius)) == 0 {
		return errors.New("ui/gtk: font family is not installed")
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
func rgba(value color.Color) ([4]C.double, bool) {
	if value == nil {
		return [4]C.double{}, false
	}
	red, green, blue, alpha := value.RGBA()
	if alpha == 0 {
		return [4]C.double{}, true
	}
	// RGBA returns premultiplied 16-bit components; CSS expects straight
	// alpha.
	return [4]C.double{
		C.double(red) / C.double(alpha),
		C.double(green) / C.double(alpha),
		C.double(blue) / C.double(alpha),
		C.double(alpha) / 0xffff,
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
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	var nativeMenu unsafe.Pointer
	if menu != nil {
		owned, ok := menu.(*contextMenu)
		if !ok {
			return errors.New("ui/gtk: context menu belongs to another backend")
		}
		nativeMenu = owned.nativeMenu()
		if nativeMenu == nil {
			return ui.ErrClosed
		}
	}
	C.gkGTKSetMenu(native, nativeMenu)
	w.mu.Lock()
	w.contract.contextMenu = menu
	w.mu.Unlock()
	return nil
}

func (w *widgetBase) SetDragSource(source ui.DragSource) error {
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	if source.Operations&^(ui.DragCopy|ui.DragMove|ui.DragLink) != 0 {
		return errors.New("ui: invalid drag operations")
	}
	text := cString(source.Text)
	defer freeCString(text)
	files := joinNul(source.Files)
	var pixels []byte
	var width, height int
	if source.Image != nil && !source.Image.Bounds().Empty() {
		pixels, width, height = rgbaPixels(source.Image)
	}
	types := make([]string, 0, len(source.Custom))
	var data []byte
	var sizes []C.size_t
	for mime, value := range source.Custom {
		types = append(types, mime)
		data = append(data, value...)
		sizes = append(sizes, C.size_t(len(value)))
	}
	typeBuffer := joinNul(types)
	var sizesPointer *C.size_t
	if len(sizes) > 0 {
		sizesPointer = &sizes[0]
	}
	C.gkGTKSetDragSource(native, C.int(source.Operations), text,
		bytesPointer(files), C.size_t(len(files)),
		(*C.uchar)(unsafe.Pointer(bytesPointer(pixels))), C.int(width), C.int(height),
		bytesPointer(typeBuffer), C.size_t(len(typeBuffer)),
		(*C.uchar)(unsafe.Pointer(bytesPointer(data))), sizesPointer, C.int(len(sizes)))
	w.mu.Lock()
	w.contract.dragSource = source
	w.mu.Unlock()
	return nil
}

func (w *widgetBase) SetDropTarget(target ui.DropTarget) error {
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	if target.Operations&^(ui.DragCopy|ui.DragMove|ui.DragLink) != 0 {
		return errors.New("ui: invalid drop operations")
	}
	types := joinNul(target.Formats)
	C.gkGTKSetDropTarget(native, C.int(target.Operations), bytesPointer(types), C.size_t(len(types)))
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

//export gkGTKGoDrop
func gkGTKGoDrop(handle C.uintptr_t, operation C.int, x, y C.double, text *C.char, files *C.char,
	filesSize C.size_t, pixels *C.uchar, width, height C.int, types *C.char, typesSize C.size_t,
	data *C.uchar, sizes *C.size_t, count C.int) {
	if handle == 0 {
		return
	}
	widget, ok := cgo.Handle(handle).Value().(*widgetBase)
	if !ok {
		return
	}
	event := ui.DropEvent{
		Operation: ui.DragOperation(operation),
		Position:  ui.Point{X: int(x), Y: int(y)},
		Files:     splitNul(unsafe.Pointer(files), filesSize),
	}
	if text != nil {
		event.Text = C.GoString(text)
	}
	if pixels != nil && width > 0 && height > 0 {
		image := stdimage.NewNRGBA(stdimage.Rect(0, 0, int(width), int(height)))
		copy(image.Pix, unsafe.Slice((*byte)(unsafe.Pointer(pixels)), int(width)*int(height)*4))
		event.Image = image
	}
	if count > 0 {
		mimes := splitNul(unsafe.Pointer(types), typesSize)
		lengths := unsafe.Slice(sizes, int(count))
		offset := 0
		event.Custom = make(map[string][]byte, len(mimes))
		for i, mime := range mimes {
			if i >= len(lengths) {
				break
			}
			length := int(lengths[i])
			event.Custom[mime] = C.GoBytes(unsafe.Add(unsafe.Pointer(data), offset), C.int(length))
			offset += length
		}
	}
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
