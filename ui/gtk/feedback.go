//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"math"
	"runtime/cgo"
	"strconv"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// actionButtons converts actions to the buffers the GTK layer expects.
type actionButtons struct {
	ids, titles []byte
	enabled     []C.int
	count       int
}

func newActionButtons(actions []ui.Action) actionButtons {
	ids := make([]string, len(actions))
	titles := make([]string, len(actions))
	enabled := make([]C.int, max(1, len(actions)))
	for i, action := range actions {
		ids[i], titles[i], enabled[i] = action.ID, action.Text, boolInt(action.Enabled)
	}
	return actionButtons{ids: joinNul(ids), titles: joinNul(titles), enabled: enabled, count: len(actions)}
}

// ---- Info bar ----

type infoBar struct {
	*widgetBase
	severity ui.Severity
	title    string
	detail   string
	live     ui.LiveRegion
	actions  listeners[ui.ActionHandler]
	closes   listeners[ui.FocusHandler]
}

// CreateInfoBar implements ui.Window.CreateInfoBar.
func (w *Window) CreateInfoBar(options ui.InfoBarOptions) (ui.InfoBar, error) {
	if err := options.Severity.Validate(); err != nil {
		return nil, err
	}
	if err := (ui.MenuOptions{Actions: options.Actions}).Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	buttons := newActionButtons(options.Actions)
	native := C.gkGTKInfoBarCreate(window, boolInt(options.Closable), bytesPointer(buttons.ids), C.size_t(len(buttons.ids)),
		bytesPointer(buttons.titles), C.size_t(len(buttons.titles)), &buttons.enabled[0], C.int(buttons.count))
	result := &infoBar{severity: options.Severity, title: options.Title, detail: options.Detail, live: options.Live}
	if result.widgetBase, err = w.newComponent(native, ui.RoleAlert, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (i *infoBar) apply() {
	native := i.nativePointer()
	if native == nil {
		return
	}
	title, detail := cString(i.title), cString(i.detail)
	defer freeCString(title)
	defer freeCString(detail)
	C.gkGTKInfoBarSet(native, C.int(i.severity), title, detail)
	if i.live != ui.LiveOff {
		message := cString(i.title + ". " + i.detail)
		defer freeCString(message)
		C.gkGTKAnnounce(native, message, C.int(i.live))
	}
	i.relayoutParent()
}

func (i *infoBar) gtkEvent(event gtkEvent) {
	switch event.kind {
	case C.GK_EVENT_ACTION:
		id := event.text
		each(&i.actions, func(handler ui.ActionHandler) {
			handler(id)
		})
	case C.GK_EVENT_CLOSE:
		i.SetVisible(false)
		each(&i.closes, func(handler ui.FocusHandler) {
			handler()
		})
	}
}

func (i *infoBar) Severity() ui.Severity {
	return i.severity
}
func (i *infoBar) SetSeverity(severity ui.Severity) {
	i.severity = severity
	i.apply()
}
func (i *infoBar) Title() string {
	return i.title
}
func (i *infoBar) SetTitle(title string) {
	i.title = title
	i.apply()
}
func (i *infoBar) Detail() string {
	return i.detail
}
func (i *infoBar) SetDetail(detail string) {
	i.detail = detail
	i.apply()
}
func (i *infoBar) OnAction(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&i.actions, handler, handler == nil)
}
func (i *infoBar) OnClose(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&i.closes, handler, handler == nil)
}

// ---- Tooltip ----

type tooltip struct {
	window    *Window
	native    unsafe.Pointer
	text      string
	delay     time.Duration
	placement ui.Edge
}

// CreateTooltip implements ui.Window.CreateTooltip.
func (w *Window) CreateTooltip(options ui.TooltipOptions) (ui.Tooltip, error) {
	if options.Delay < 0 {
		return nil, errors.New("ui: tooltip delay must be non-negative")
	}
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}
	result := &tooltip{window: w, native: C.gkGTKTooltipCreate(), delay: options.Delay, placement: options.Placement}
	result.SetText(options.Text)
	w.adoptResource(result)
	return result, nil
}

func (t *tooltip) Close() {
	if t.native != nil {
		C.gkGTKRelease(t.native)
		t.native = nil
		t.window.releaseResource(t)
	}
}
func (t *tooltip) Text() string {
	return t.text
}
func (t *tooltip) SetText(text string) {
	t.text = text
	if t.native != nil {
		value := cString(text)
		defer freeCString(value)
		C.gkGTKTooltipSetText(t.native, value)
	}
}
func (t *tooltip) Show(anchor ui.Widget) error {
	if t.native == nil {
		return ui.ErrClosed
	}
	owner, ok := anchor.(nativeWidget)
	if !ok {
		return errors.New("ui/gtk: tooltip anchor belongs to another backend")
	}
	view := owner.base().nativePointer()
	if view == nil {
		return ui.ErrClosed
	}
	if C.gkGTKTooltipShow(t.native, view, C.int(t.placement), C.double(t.delay.Seconds())) == 0 {
		return errors.New("ui/gtk: a tooltip's anchor must be in a visible window")
	}
	return nil
}
func (t *tooltip) Hide() {
	if t.native != nil {
		C.gkGTKTooltipHide(t.native)
	}
}
func (t *tooltip) IsShown() bool {
	return t.native != nil && C.gkGTKTooltipShown(t.native) != 0
}

// ---- Toast ----

type toast struct {
	window  *Window
	native  unsafe.Pointer
	handle  cgo.Handle
	actions listeners[ui.ActionHandler]
}

// CreateToast implements ui.Window.CreateToast.
func (w *Window) CreateToast(options ui.ToastOptions) (ui.Toast, error) {
	if options.Message == "" {
		return nil, errors.New("ui: toast requires a message")
	}
	if err := (ui.MenuOptions{Actions: options.Actions}).Validate(); err != nil {
		return nil, err
	}
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	result := &toast{window: w}
	result.handle = cgo.NewHandle(result)
	message := cString(options.Message)
	defer freeCString(message)
	buttons := newActionButtons(options.Actions)
	result.native = C.gkGTKToastCreate(C.uintptr_t(result.handle), message, bytesPointer(buttons.ids),
		C.size_t(len(buttons.ids)), bytesPointer(buttons.titles), C.size_t(len(buttons.titles)),
		&buttons.enabled[0], C.int(buttons.count), C.double(timeout.Seconds()))
	w.adoptResource(result)
	return result, nil
}

func (t *toast) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_ACTION {
		id := event.text
		each(&t.actions, func(handler ui.ActionHandler) {
			handler(id)
		})
	}
}
func (t *toast) Close() {
	if t.native == nil {
		return
	}
	native := t.native
	t.native = nil
	C.gkGTKToastRelease(native)
	t.handle.Delete()
	t.window.releaseResource(t)
}
func (t *toast) Show() error {
	if t.native == nil {
		return ui.ErrClosed
	}
	window := t.window.nativeWindowPointer()
	if window == nil || C.gkGTKToastShow(t.native, window) == 0 {
		return ui.ErrClosed
	}
	return nil
}
func (t *toast) Dismiss() {
	if t.native != nil {
		C.gkGTKToastDismiss(t.native)
	}
}
func (t *toast) IsShown() bool {
	return t.native != nil && C.gkGTKToastShown(t.native) != 0
}
func (t *toast) OnAction(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&t.actions, handler, handler == nil)
}

// ---- Badge ----

type badge struct {
	*widgetBase
	options ui.BadgeOptions
}

// CreateBadge implements ui.Window.CreateBadge.
func (w *Window) CreateBadge(options ui.BadgeOptions) (ui.Badge, error) {
	if err := options.Severity.Validate(); err != nil {
		return nil, err
	}
	if options.Count < 0 || options.Maximum < 0 {
		return nil, errors.New("ui: badge counts must be non-negative")
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &badge{options: options}
	if result.widgetBase, err = w.newComponent(C.gkGTKBadgeCreate(window), ui.RoleStatus, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (b *badge) gtkEvent(gtkEvent) {
	// A badge sends no native events.
}

func (b *badge) apply() {
	if native := b.nativePointer(); native != nil {
		text, accessible := cString(b.DisplayText()), cString(b.options.AccessibleValue)
		defer freeCString(text)
		defer freeCString(accessible)
		C.gkGTKBadgeSet(native, text, C.int(b.options.Severity), accessible)
		b.relayoutParent()
	}
}

func (b *badge) Count() int {
	return b.options.Count
}
func (b *badge) SetCount(count int) {
	b.options.Count = max(0, count)
	b.apply()
}
func (b *badge) Text() string {
	return b.options.Text
}
func (b *badge) SetText(text string) {
	b.options.Text = text
	b.apply()
	b.announceText(text)
}

// DisplayText is Text when set, otherwise the count capped at Maximum.
func (b *badge) DisplayText() string {
	if b.options.Text != "" {
		return b.options.Text
	}
	if b.options.Maximum > 0 && b.options.Count > b.options.Maximum {
		return strconv.Itoa(b.options.Maximum) + "+"
	}
	return strconv.Itoa(b.options.Count)
}

// ---- Progress dialog ----

type progressDialog struct {
	window  *Window
	native  unsafe.Pointer
	handle  cgo.Handle
	value   float64
	message string
	cancels listeners[ui.FocusHandler]
}

// CreateProgressDialog implements ui.Window.CreateProgressDialog.
func (w *Window) CreateProgressDialog(options ui.ProgressDialogOptions) (ui.ProgressDialog, error) {
	if math.IsNaN(options.Value) || options.Value > 1 {
		return nil, errors.New("ui: progress must be at most one, or negative for indeterminate")
	}
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}
	result := &progressDialog{window: w, value: options.Value, message: options.Message}
	result.handle = cgo.NewHandle(result)
	title := cString(options.Title)
	defer freeCString(title)
	result.native = C.gkGTKProgressDialogCreate(C.uintptr_t(result.handle), title, boolInt(options.Cancellable))
	result.apply()
	return result, nil
}

func (p *progressDialog) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_CLOSE {
		each(&p.cancels, func(handler ui.FocusHandler) {
			handler()
		})
	}
}
func (p *progressDialog) apply() {
	if p.native != nil {
		message := cString(p.message)
		defer freeCString(message)
		C.gkGTKProgressDialogSet(p.native, message, C.double(p.value))
	}
}
func (p *progressDialog) Close() {
	if p.native == nil {
		return
	}
	native := p.native
	p.native = nil
	C.gkGTKProgressDialogRelease(native)
	p.handle.Delete()
	p.window.releaseResource(p)
}
func (p *progressDialog) Show() error {
	if p.native == nil {
		return ui.ErrClosed
	}
	C.gkGTKProgressDialogShow(p.native, p.window.nativeWindowPointer())
	return nil
}
func (p *progressDialog) Value() float64 {
	return p.value
}
func (p *progressDialog) SetValue(value float64) {
	if math.IsNaN(value) {
		return
	}
	p.value = min(value, 1)
	p.apply()
}
func (p *progressDialog) Message() string {
	return p.message
}
func (p *progressDialog) SetMessage(message string) {
	p.message = message
	p.apply()
}
func (p *progressDialog) OnCancel(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&p.cancels, handler, handler == nil)
}

// ---- Empty state ----

type emptyState struct {
	*widgetBase
	title, detail string
	actions       listeners[ui.ActionHandler]
}

// CreateEmptyState implements ui.Window.CreateEmptyState.
func (w *Window) CreateEmptyState(options ui.EmptyStateOptions) (ui.EmptyState, error) {
	if err := (ui.MenuOptions{Actions: options.Actions}).Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	icon := nativeIcon(options.Icon, 0)
	defer C.gkGTKRelease(icon)
	buttons := newActionButtons(options.Actions)
	native := C.gkGTKEmptyStateCreate(window, icon, bytesPointer(buttons.ids), C.size_t(len(buttons.ids)),
		bytesPointer(buttons.titles), C.size_t(len(buttons.titles)), &buttons.enabled[0], C.int(buttons.count))
	result := &emptyState{title: options.Title, detail: options.Detail}
	if result.widgetBase, err = w.newComponent(native, ui.RoleGroup, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (e *emptyState) apply() {
	if native := e.nativePointer(); native != nil {
		title, detail := cString(e.title), cString(e.detail)
		defer freeCString(title)
		defer freeCString(detail)
		C.gkGTKEmptyStateSet(native, title, detail)
	}
}
func (e *emptyState) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_ACTION {
		id := event.text
		each(&e.actions, func(handler ui.ActionHandler) {
			handler(id)
		})
	}
}
func (e *emptyState) Title() string {
	return e.title
}
func (e *emptyState) SetTitle(title string) {
	e.title = title
	e.apply()
}
func (e *emptyState) Detail() string {
	return e.detail
}
func (e *emptyState) SetDetail(detail string) {
	e.detail = detail
	e.apply()
}
func (e *emptyState) OnAction(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&e.actions, handler, handler == nil)
}

// ---- Inline validation ----

type inlineValidation struct {
	*widgetBase
	severity ui.Severity
	text     string
}

// CreateInlineValidation implements ui.Window.CreateInlineValidation.
func (w *Window) CreateInlineValidation(options ui.InlineValidationOptions) (ui.InlineValidation, error) {
	if err := options.Severity.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &inlineValidation{severity: options.Severity, text: options.Text}
	if result.widgetBase, err = w.newComponent(C.gkGTKInlineValidationCreate(window), ui.RoleLabel, result); err != nil {
		return nil, err
	}
	result.apply(false)
	return result, nil
}

func (v *inlineValidation) gtkEvent(gtkEvent) {
	// An inline validation message sends no native events.
}

// apply updates the message; changes are announced politely so a screen
// reader hears new validation errors.
func (v *inlineValidation) apply(announce bool) {
	if native := v.nativePointer(); native != nil {
		text := cString(v.text)
		defer freeCString(text)
		C.gkGTKInlineValidationSet(native, C.int(v.severity), text)
		if announce {
			C.gkGTKAnnounce(native, text, C.int(ui.LivePolite))
		}
		v.relayoutParent()
	}
}
func (v *inlineValidation) Severity() ui.Severity {
	return v.severity
}
func (v *inlineValidation) SetSeverity(severity ui.Severity) {
	v.severity = severity
	v.apply(false)
}
func (v *inlineValidation) Text() string {
	return v.text
}
func (v *inlineValidation) SetText(text string) {
	changed := text != v.text
	v.text = text
	v.apply(changed)
	v.announceText(text)
}

var (
	_ ui.InfoBar          = (*infoBar)(nil)
	_ ui.Tooltip          = (*tooltip)(nil)
	_ ui.Toast            = (*toast)(nil)
	_ ui.Badge            = (*badge)(nil)
	_ ui.ProgressDialog   = (*progressDialog)(nil)
	_ ui.EmptyState       = (*emptyState)(nil)
	_ ui.InlineValidation = (*inlineValidation)(nil)
)
