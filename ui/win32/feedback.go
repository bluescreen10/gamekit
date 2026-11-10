//go:build windows

package win32

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

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
	native := window.newInfoBar(options.Closable, options.Actions)
	result := &infoBar{severity: options.Severity, title: options.Title, detail: options.Detail, live: options.Live}
	if result.widgetBase, err = w.newComponent(native, ui.RoleAlert, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (i *infoBar) apply() {
	native := i.nativeView()
	if native == nil {
		return
	}
	native.setInfoBar(int(i.severity), i.title, i.detail)
	if i.live != ui.LiveOff {
		native.announce(i.title + ". " + i.detail)
	}
	i.relayoutParent()
}

func (i *infoBar) receiveEvent(event any) {
	switch event := event.(type) {
	case actionEvent:
		id := event.id
		each(&i.actions, func(handler ui.ActionHandler) {
			handler(id)
		})
	case closeEvent:
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
	native    *nativeTooltip
	text      string
	delay     time.Duration
	placement ui.Edge
}

// CreateTooltip implements ui.Window.CreateTooltip.
func (w *Window) CreateTooltip(options ui.TooltipOptions) (ui.Tooltip, error) {
	if options.Delay < 0 {
		return nil, errors.New("ui: tooltip delay must be non-negative")
	}
	if w.nativeWindow() == nil {
		return nil, ui.ErrClosed
	}
	result := &tooltip{window: w, native: newTooltip(), delay: options.Delay, placement: options.Placement}
	result.SetText(options.Text)
	w.adoptResource(result)
	return result, nil
}

func (t *tooltip) Close() {
	if t.native != nil {
		t.native.destroy()
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
		t.native.setText(text)
	}
}

func (t *tooltip) Show(anchor ui.Widget) error {
	if t.native == nil {
		return ui.ErrClosed
	}
	owner, ok := anchor.(nativeWidget)
	if !ok {
		return errors.New("ui/win32: tooltip anchor belongs to another backend")
	}
	view := owner.base().nativeView()
	if view == nil {
		return ui.ErrClosed
	}
	if !t.native.show(view, int(t.placement), t.delay.Seconds()) {
		return errors.New("ui/win32: a tooltip's anchor must be in a visible window")
	}
	return nil
}

func (t *tooltip) Hide() {
	if t.native != nil {
		t.native.hide()
	}
}

func (t *tooltip) IsShown() bool {
	return t.native != nil && t.native.shown
}

// ---- Toast ----

type toast struct {
	window  *Window
	native  *nativeToast
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
	if w.nativeWindow() == nil {
		return nil, ui.ErrClosed
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	result := &toast{window: w}
	result.native = newToast(result, options.Message, options.Actions,
		timeout.Seconds())
	w.adoptResource(result)
	return result, nil
}

func (t *toast) receiveEvent(event any) {
	if action, ok := event.(actionEvent); ok {
		id := action.id
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
	native.release()
	t.window.releaseResource(t)
}

func (t *toast) Show() error {
	if t.native == nil {
		return ui.ErrClosed
	}
	window := t.window.nativeWindow()
	if window == nil || !t.native.show(window) {
		return ui.ErrClosed
	}
	return nil
}

func (t *toast) Dismiss() {
	if t.native != nil {
		t.native.dismiss()
	}
}

func (t *toast) IsShown() bool {
	return t.native != nil && t.native.isShown()
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
	if result.widgetBase, err = w.newComponent(window.newBadge(), ui.RoleStatus, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (b *badge) receiveEvent(event any) {
	// A badge sends no native events.
}

func (b *badge) apply() {
	if native := b.nativeView(); native != nil {
		native.setBadgeValue(b.DisplayText(), int(b.options.Severity), b.options.AccessibleValue)
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
	native  *nativeProgressDialog
	value   float64
	message string
	cancels listeners[ui.FocusHandler]
}

// CreateProgressDialog implements ui.Window.CreateProgressDialog.
func (w *Window) CreateProgressDialog(options ui.ProgressDialogOptions) (ui.ProgressDialog, error) {
	if math.IsNaN(options.Value) || options.Value > 1 {
		return nil, errors.New("ui: progress must be at most one, or negative for indeterminate")
	}
	if w.nativeWindow() == nil {
		return nil, ui.ErrClosed
	}
	result := &progressDialog{window: w, value: options.Value, message: options.Message}
	result.native = newProgressDialog(result, options.Title, options.Cancellable)
	result.apply()
	return result, nil
}

func (p *progressDialog) receiveEvent(event any) {
	if _, ok := event.(closeEvent); ok {
		each(&p.cancels, func(handler ui.FocusHandler) {
			handler()
		})
	}
}

func (p *progressDialog) apply() {
	if p.native != nil {
		p.native.set(p.message, p.value)
	}
}

func (p *progressDialog) Close() {
	if p.native == nil {
		return
	}
	native := p.native
	p.native = nil
	native.release()
	p.window.releaseResource(p)
}

func (p *progressDialog) Show() error {
	if p.native == nil {
		return ui.ErrClosed
	}
	p.native.show(p.window.nativeWindow())
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
	icon := newIcon(options.Icon, 0)
	native := window.newEmptyState(icon, options.Actions)
	result := &emptyState{title: options.Title, detail: options.Detail}
	if result.widgetBase, err = w.newComponent(native, ui.RoleGroup, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (e *emptyState) apply() {
	if native := e.nativeView(); native != nil {
		native.setEmptyState(e.title, e.detail)
	}
}

func (e *emptyState) receiveEvent(event any) {
	if action, ok := event.(actionEvent); ok {
		id := action.id
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
	if result.widgetBase, err = w.newComponent(window.newInlineValidation(), ui.RoleLabel, result); err != nil {
		return nil, err
	}
	result.apply(false)
	return result, nil
}

func (v *inlineValidation) receiveEvent(event any) {
	// An inline validation message sends no native events.
}

// apply updates the message; changes are announced politely so a screen
// reader hears new validation errors.
func (v *inlineValidation) apply(announce bool) {
	if native := v.nativeView(); native != nil {
		native.setValidation(int(v.severity), v.text)
		if announce {
			native.announce(v.text)
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
