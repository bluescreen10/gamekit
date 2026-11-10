//go:build windows

package win32

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

// newComponent wraps a native view created for the window and routes its
// events to owner.
func (w *Window) newComponent(native *nativeView, role ui.AccessibilityRole, owner eventReceiver) (*widgetBase, error) {
	if native == nil {
		return nil, fmt.Errorf("ui/win32: native backend could not create a %s", role)
	}
	base := newWidgetBase(w, native, role)
	base.contract.owner = owner
	return base, nil
}

func (w *Window) windowHandle() (*nativeWindow, error) {
	if w.nativeWindow() == nil {
		return nil, ui.ErrClosed
	}
	return w.nativeWindow(), nil
}

// ---- Toggle button ----

type toggleButton struct {
	*widgetBase
	groupID string
	changes listeners[ui.ToggleHandler]
}

// CreateToggleButton implements ui.Window.CreateToggleButton.
func (w *Window) CreateToggleButton(options ui.ToggleButtonOptions) (ui.ToggleButton, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	icon := newIcon(options.Icon, 0)
	result := &toggleButton{groupID: options.GroupID}
	native := window.newToggleButton(options.Text, icon, options.Checked,
		options.Presentation == ui.ToggleButtonToolbar)
	if result.widgetBase, err = w.newComponent(native, ui.RoleToggleButton, result); err != nil {
		return nil, err
	}
	if options.Checked {
		result.uncheckGroup()
	}
	return result, nil
}

func (t *toggleButton) Text() string {
	return nativeText(t.nativeView())
}

func (t *toggleButton) SetText(text string) {
	setNativeText(t.nativeView(), text)
	t.announceText(text)
}

func (t *toggleButton) IsChecked() bool {
	native := t.nativeView()
	return native != nil && native.isChecked()
}

func (t *toggleButton) SetChecked(checked bool) {
	if native := t.nativeView(); native != nil {
		native.setChecked(checked)
		if checked {
			t.uncheckGroup()
		}
	}
}

func (t *toggleButton) GroupID() string {
	return t.groupID
}

func (t *toggleButton) OnChange(handler ui.ToggleHandler) ui.Unsubscribe {
	return subscribe(&t.changes, handler, handler == nil)
}

// uncheckGroup keeps toggle buttons with the same group mutually exclusive.
func (t *toggleButton) uncheckGroup() {
	if t.groupID == "" {
		return
	}
	for _, widget := range t.window.liveWidgets() {
		other, ok := widget.contract.owner.(*toggleButton)
		if ok && other != t && other.groupID == t.groupID && other.IsChecked() {
			other.nativeView().setChecked(false)
			checked := false
			each(&other.changes, func(handler ui.ToggleHandler) {
				handler(checked)
			})
		}
	}
}

func (t *toggleButton) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	checked := t.IsChecked()
	if checked {
		t.uncheckGroup()
	}
	each(&t.changes, func(handler ui.ToggleHandler) {
		handler(checked)
	})
}

// liveWidgets returns the window's open widgets.
func (w *Window) liveWidgets() []*widgetBase {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]*widgetBase, 0, len(w.widgets))
	for widget := range w.widgets {
		result = append(result, widget)
	}
	return result
}

// ---- Switch ----

type switchControl struct {
	*widgetBase
	text    string
	changes listeners[ui.ToggleHandler]
}

// CreateSwitch implements ui.Window.CreateSwitch.
func (w *Window) CreateSwitch(options ui.SwitchOptions) (ui.Switch, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &switchControl{text: options.Text}
	native := window.newSwitch(options.Text, options.OnLabel, options.OffLabel, options.Checked, int(options.Size))
	if result.widgetBase, err = w.newComponent(native, ui.RoleSwitch, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *switchControl) Text() string {
	return s.text
}

func (s *switchControl) SetText(text string) {
	s.text = text
	s.set(s.IsChecked())
	s.announceText(text)
}

func (s *switchControl) IsChecked() bool {
	native := s.nativeView()
	return native != nil && native.isSwitchOn()
}

func (s *switchControl) SetChecked(checked bool) {
	s.set(checked)
}

func (s *switchControl) set(checked bool) {
	if native := s.nativeView(); native != nil {
		native.configureSwitch(s.text, checked)
	}
}

func (s *switchControl) OnChange(handler ui.ToggleHandler) ui.Unsubscribe {
	return subscribe(&s.changes, handler, handler == nil)
}

func (s *switchControl) receiveEvent(event any) {
	if _, ok := event.(changeEvent); ok {
		checked := s.IsChecked()
		s.set(checked)
		each(&s.changes, func(handler ui.ToggleHandler) {
			handler(checked)
		})
	}
}

// ---- Password field ----

type passwordField struct {
	*widgetBase
	placeholder string
	reveal      ui.PasswordReveal
	revealed    bool
	changes     listeners[ui.TextChangeHandler]
	submits     listeners[ui.TextChangeHandler]
}

// CreatePasswordField implements ui.Window.CreatePasswordField.
func (w *Window) CreatePasswordField(options ui.PasswordFieldOptions) (ui.PasswordField, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &passwordField{placeholder: options.Placeholder, reveal: options.Reveal}
	native := window.newPasswordField(options.Placeholder, options.Reveal == ui.PasswordRevealButton)
	if result.widgetBase, err = w.newComponent(native, ui.RolePasswordField, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *passwordField) Text() string {
	native := p.nativeView()
	if native == nil {
		return ""
	}
	return native.passwordText()
}

func (p *passwordField) SetText(text string) {
	p.set(text)
}

func (p *passwordField) set(text string) {
	if native := p.nativeView(); native != nil {
		native.setPassword(text, p.placeholder, p.revealed)
	}
}

func (p *passwordField) Placeholder() string {
	return p.placeholder
}

func (p *passwordField) SetPlaceholder(placeholder string) {
	p.placeholder = placeholder
	p.set(p.Text())
}

func (p *passwordField) IsRevealed() bool {
	return p.revealed
}

func (p *passwordField) SetRevealed(revealed bool) error {
	if p.nativeView() == nil {
		return ui.ErrClosed
	}
	if revealed && p.reveal == ui.PasswordRevealNone {
		return fmt.Errorf("ui/win32: password field does not allow revealing: %w", ui.ErrNotSupported)
	}
	text := p.Text()
	p.revealed = revealed
	p.set(text)
	return nil
}

func (p *passwordField) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	return subscribe(&p.changes, handler, handler == nil)
}

func (p *passwordField) OnSubmit(handler ui.TextChangeHandler) ui.Unsubscribe {
	return subscribe(&p.submits, handler, handler == nil)
}

func (p *passwordField) receiveEvent(event any) {
	switch event := event.(type) {
	case textChangeEvent:
		text := p.Text()
		each(&p.changes, func(handler ui.TextChangeHandler) {
			handler(text)
		})
	case submitEvent:
		text := p.Text()
		each(&p.submits, func(handler ui.TextChangeHandler) {
			handler(text)
		})
	case expandEvent:
		p.revealed = event.expanded
	}
}

// ---- Search field ----

type searchField struct {
	*widgetBase
	suggestions []string
	changes     listeners[ui.TextChangeHandler]
	submits     listeners[ui.TextChangeHandler]
}

// CreateSearchField implements ui.Window.CreateSearchField.
func (w *Window) CreateSearchField(options ui.SearchFieldOptions) (ui.SearchField, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &searchField{}
	native := window.newSearchField(options.Text, options.Placeholder)
	if result.widgetBase, err = w.newComponent(native, ui.RoleSearchField, result); err != nil {
		return nil, err
	}
	result.SetSuggestions(options.Suggestions)
	return result, nil
}

func (s *searchField) Text() string {
	return nativeText(s.nativeView())
}

func (s *searchField) SetText(text string) {
	setNativeText(s.nativeView(), text)
}

func (s *searchField) Placeholder() string {
	native := s.nativeView()
	if native == nil {
		return ""
	}
	return native.placeholder
}

func (s *searchField) SetPlaceholder(placeholder string) {
	if native := s.nativeView(); native != nil {
		native.setPlaceholder(placeholder)
	}
}

func (s *searchField) Suggestions() []string {
	return append([]string(nil), s.suggestions...)
}

func (s *searchField) SetSuggestions(suggestions []string) {
	s.suggestions = append([]string(nil), suggestions...)
	if native := s.nativeView(); native != nil {
		native.setSuggestions(suggestions)
	}
}

func (s *searchField) Clear() {
	s.SetText("")
}

func (s *searchField) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	return subscribe(&s.changes, handler, handler == nil)
}

func (s *searchField) OnSubmit(handler ui.TextChangeHandler) ui.Unsubscribe {
	return subscribe(&s.submits, handler, handler == nil)
}

func (s *searchField) receiveEvent(event any) {
	text := s.Text()
	switch event.(type) {
	case textChangeEvent:
		each(&s.changes, func(handler ui.TextChangeHandler) {
			handler(text)
		})
	case submitEvent:
		each(&s.submits, func(handler ui.TextChangeHandler) {
			handler(text)
		})
	}
}

// ---- Number field ----

type numberField struct {
	*widgetBase
	options ui.NumberFieldOptions
	changes listeners[ui.ValueChangeHandler]
}

// CreateNumberField implements ui.Window.CreateNumberField.
func (w *Window) CreateNumberField(options ui.NumberFieldOptions) (ui.NumberField, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &numberField{options: options}
	native := window.newNumberField(options.Minimum, options.Maximum,
		options.Value, options.Step, options.Decimals)
	if result.widgetBase, err = w.newComponent(native, ui.RoleSpinButton, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (n *numberField) Value() float64 {
	native := n.nativeView()
	if native == nil {
		return 0
	}
	return float64(native.numberFieldValue())
}

func (n *numberField) SetValue(value float64) error {
	if math.IsNaN(value) || value < n.options.Minimum || value > n.options.Maximum {
		return errors.New("ui: number field value must be within its range")
	}
	native := n.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setNumberFieldValue(value)
	return nil
}

func (n *numberField) Minimum() float64 {
	return n.options.Minimum
}

func (n *numberField) Maximum() float64 {
	return n.options.Maximum
}

func (n *numberField) Step() float64 {
	return n.options.Step
}

func (n *numberField) Decimals() int {
	return n.options.Decimals
}

func (n *numberField) OnChange(handler ui.ValueChangeHandler) ui.Unsubscribe {
	return subscribe(&n.changes, handler, handler == nil)
}

func (n *numberField) receiveEvent(event any) {
	if _, ok := event.(changeEvent); ok {
		value := n.Value()
		each(&n.changes, func(handler ui.ValueChangeHandler) {
			handler(value)
		})
	}
}

// ---- Editable combo ----

type editableCombo struct {
	*widgetBase
	items     []string
	freeText  bool
	lastValid string
	changes   listeners[ui.TextChangeHandler]
}

// CreateEditableCombo implements ui.Window.CreateEditableCombo.
func (w *Window) CreateEditableCombo(options ui.EditableComboOptions) (ui.EditableCombo, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	if !options.AllowFreeText && options.Text != "" && !slices.Contains(options.Items, options.Text) {
		return nil, errors.New("ui: combo text must be one of its items when free text is not allowed")
	}
	result := &editableCombo{items: append([]string(nil), options.Items...), freeText: options.AllowFreeText, lastValid: options.Text}
	native := window.newComboBox(options.Text, options.Items)
	if result.widgetBase, err = w.newComponent(native, ui.RoleComboBox, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *editableCombo) Text() string {
	return nativeText(c.nativeView())
}

func (c *editableCombo) SetText(text string) {
	setNativeText(c.nativeView(), text)
	if c.freeText || slices.Contains(c.items, text) {
		c.lastValid = text
	}
}

func (c *editableCombo) Items() []string {
	return append([]string(nil), c.items...)
}

func (c *editableCombo) SetItems(items []string) {
	c.items = append([]string(nil), items...)
	if native := c.nativeView(); native != nil {
		native.setComboItems(items)
	}
}

func (c *editableCombo) SelectedIndex() int {
	return slices.Index(c.items, c.Text())
}

func (c *editableCombo) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	return subscribe(&c.changes, handler, handler == nil)
}

func (c *editableCombo) receiveEvent(event any) {
	text := c.Text()
	switch event := event.(type) {
	case textChangeEvent:
		if event.text != "" {
			text = event.text
		}
	case submitEvent:
		// Return or leaving the field commits the text; without free text an
		// unknown entry reverts to the last valid one.
		if !c.freeText && !slices.Contains(c.items, text) {
			setNativeText(c.nativeView(), c.lastValid)
			return
		}
	default:
		return
	}
	if c.freeText || slices.Contains(c.items, text) {
		c.lastValid = text
	}
	each(&c.changes, func(handler ui.TextChangeHandler) {
		handler(text)
	})
}

// ---- Segmented control ----

type segmentedControl struct {
	*widgetBase
	items   []ui.Segment
	mode    ui.SelectionMode
	changes listeners[ui.SelectionIDsHandler]
}

// CreateSegmentedControl implements ui.Window.CreateSegmentedControl.
func (w *Window) CreateSegmentedControl(options ui.SegmentedControlOptions) (ui.SegmentedControl, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &segmentedControl{items: append([]ui.Segment(nil), options.Items...), mode: options.Mode}
	native := window.newSegmentedControl(options.Items, options.Mode)
	if result.widgetBase, err = w.newComponent(native, ui.RoleRadioGroup, result); err != nil {
		return nil, err
	}
	if err := result.SetSelectedIDs(options.SelectedIDs); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (s *segmentedControl) Items() []ui.Segment {
	return append([]ui.Segment(nil), s.items...)
}

func (s *segmentedControl) ids() []string {
	ids := make([]string, len(s.items))
	for i, item := range s.items {
		ids[i] = item.ID
	}
	return ids
}

func (s *segmentedControl) SelectedIDs() []string {
	native := s.nativeView()
	if native == nil || s.mode == ui.SelectionNone {
		return nil
	}
	var selected []string
	for i, item := range s.items {
		if native.isSegmentSelected(i) {
			selected = append(selected, item.ID)
		}
	}
	return selected
}

func (s *segmentedControl) SetSelectedIDs(ids []string) error {
	if err := ui.ValidateSelectedIDs(s.ids(), ids, s.mode); err != nil {
		return err
	}
	native := s.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	for i, item := range s.items {
		native.setSegmentSelected(i, slices.Contains(ids, item.ID))
	}
	return nil
}

func (s *segmentedControl) OnChange(handler ui.SelectionIDsHandler) ui.Unsubscribe {
	return subscribe(&s.changes, handler, handler == nil)
}

func (s *segmentedControl) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	selected := s.SelectedIDs()
	if s.mode == ui.SelectionNone {
		// A momentary control reports the clicked segment.
		if native := s.nativeView(); native != nil {
			index := int(native.clickedSegment())
			if index >= 0 && index < len(s.items) {
				selected = []string{s.items[index].ID}
			}
		}
	}
	each(&s.changes, func(handler ui.SelectionIDsHandler) {
		handler(append([]string(nil), selected...))
	})
}

// ---- Stepper and dial ----

// rangedControl is a native control with a scalar value in a range.
type rangedControl struct {
	*widgetBase
	minimum, maximum, step float64
	wraps                  bool
	changes                listeners[ui.ValueChangeHandler]
}

func (r *rangedControl) Value() float64 {
	native := r.nativeView()
	if native == nil {
		return 0
	}
	return float64(native.controlValue())
}

func (r *rangedControl) SetValue(value float64) error {
	if math.IsNaN(value) || value < r.minimum || value > r.maximum {
		return errors.New("ui: value must be within the control's range")
	}
	native := r.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setControlValue(value)
	return nil
}

func (r *rangedControl) OnChange(handler ui.ValueChangeHandler) ui.Unsubscribe {
	return subscribe(&r.changes, handler, handler == nil)
}

func (r *rangedControl) receiveEvent(event any) {
	if _, ok := event.(changeEvent); ok {
		value := r.Value()
		each(&r.changes, func(handler ui.ValueChangeHandler) {
			handler(value)
		})
	}
}

// stepBy changes the value programmatically, wrapping when enabled.
func (r *rangedControl) stepBy(delta float64) {
	value := r.Value() + delta
	switch {
	case value > r.maximum && r.wraps:
		value = r.minimum
	case value < r.minimum && r.wraps:
		value = r.maximum
	default:
		value = max(r.minimum, min(r.maximum, value))
	}
	_ = r.SetValue(value)
}

type stepper struct{ rangedControl }

func (s *stepper) Increment() {
	s.stepBy(max(s.step, 1))
}

func (s *stepper) Decrement() {
	s.stepBy(-max(s.step, 1))
}

// CreateStepper implements ui.Window.CreateStepper.
func (w *Window) CreateStepper(options ui.StepperOptions) (ui.Stepper, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &stepper{rangedControl{minimum: options.Minimum, maximum: options.Maximum, step: options.Step, wraps: options.Wrap}}
	native := window.newStepper(options.Minimum, options.Maximum,
		options.Value, options.Step, options.Wrap)
	if result.widgetBase, err = w.newComponent(native, ui.RoleSpinButton, result); err != nil {
		return nil, err
	}
	return result, nil
}

type dial struct{ rangedControl }

// CreateDial implements ui.Window.CreateDial.
func (w *Window) CreateDial(options ui.DialOptions) (ui.Dial, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &dial{rangedControl{minimum: options.Minimum, maximum: options.Maximum, step: options.Step, wraps: options.Wrap}}
	native := window.newDial(options.Minimum, options.Maximum,
		options.Value, options.Step, options.Wrap)
	if result.widgetBase, err = w.newComponent(native, ui.RoleSlider, result); err != nil {
		return nil, err
	}
	return result, nil
}

// ---- Range slider ----

type rangeSlider struct {
	*widgetBase
	options ui.RangeSliderOptions
	changes listeners[ui.RangeChangeHandler]
}

// CreateRangeSlider implements ui.Window.CreateRangeSlider.
func (w *Window) CreateRangeSlider(options ui.RangeSliderOptions) (ui.RangeSlider, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &rangeSlider{options: options}
	native := window.newRangeSlider(options.Minimum, options.Maximum,
		options.LowerValue, options.UpperValue, options.Step)
	if result.widgetBase, err = w.newComponent(native, ui.RoleSlider, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *rangeSlider) values() (float64, float64) {
	native := r.nativeView()
	if native == nil {
		return 0, 0
	}
	lower, upper := native.rangeValues()
	return float64(lower), float64(upper)
}

func (r *rangeSlider) LowerValue() float64 {
	lower, _ := r.values()
	return lower
}

func (r *rangeSlider) UpperValue() float64 {
	_, upper := r.values()
	return upper
}

func (r *rangeSlider) SetValues(lower, upper float64) error {
	options := r.options
	options.LowerValue, options.UpperValue = lower, upper
	if err := options.Validate(); err != nil {
		return err
	}
	native := r.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setRange(lower, upper)
	return nil
}

func (r *rangeSlider) OnChange(handler ui.RangeChangeHandler) ui.Unsubscribe {
	return subscribe(&r.changes, handler, handler == nil)
}

func (r *rangeSlider) receiveEvent(event any) {
	if change, ok := event.(rangeChangeEvent); ok {
		lower, upper := change.lower, change.upper
		each(&r.changes, func(handler ui.RangeChangeHandler) {
			handler(lower, upper)
		})
	}
}

// ---- Dates and times ----

// Date-picker element flags of the native layer.
const (
	datePickerHourMinute       = 0x000c
	datePickerHourMinuteSecond = 0x000e
	datePickerYearMonthDay     = 0x00e0
)

func unixSeconds(value time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	return float64(value.UnixNano()) / float64(time.Second)
}

// calendarDate converts seconds since 1970 to a UTC calendar date.
func calendarDate(seconds float64) time.Time {
	value := time.Unix(0, int64(seconds*float64(time.Second))).UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

// dateOnly keeps the calendar date of value in its own location as a UTC date.
func dateOnly(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

type datePicker struct {
	*widgetBase
	minimum, maximum time.Time
	changes          listeners[ui.DateChangeHandler]
}

// CreateDatePicker implements ui.Window.CreateDatePicker.
func (w *Window) CreateDatePicker(options ui.DatePickerOptions) (ui.DatePicker, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	value := dateOnly(options.Value)
	if value.IsZero() {
		value = dateOnly(time.Now())
	}
	minimum, maximum := dateOnly(options.Minimum), dateOnly(options.Maximum)
	if !minimum.IsZero() && !maximum.IsZero() && minimum.After(maximum) {
		return nil, errors.New("ui: date picker minimum is after its maximum")
	}
	result := &datePicker{minimum: minimum, maximum: maximum}
	native := window.newDatePicker(0, datePickerYearMonthDay, unixSeconds(value),
		unixSeconds(minimum), unixSeconds(maximum), 0, 0, 0)
	if result.widgetBase, err = w.newComponent(native, ui.RoleDatePicker, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *datePicker) Value() time.Time {
	native := d.nativeView()
	if native == nil {
		return time.Time{}
	}
	value, _ := native.pickedDate()
	return calendarDate(value)
}

func (d *datePicker) SetValue(date time.Time) error {
	date = dateOnly(date)
	if date.IsZero() || (!d.minimum.IsZero() && date.Before(d.minimum)) || (!d.maximum.IsZero() && date.After(d.maximum)) {
		return errors.New("ui: date is outside the picker's range")
	}
	native := d.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setPickedDate(unixSeconds(date), 0)
	return nil
}

func (d *datePicker) OnChange(handler ui.DateChangeHandler) ui.Unsubscribe {
	return subscribe(&d.changes, handler, handler == nil)
}

func (d *datePicker) receiveEvent(event any) {
	if _, ok := event.(changeEvent); ok {
		value := d.Value()
		each(&d.changes, func(handler ui.DateChangeHandler) {
			handler(value)
		})
	}
}

type timePicker struct {
	*widgetBase
	changes listeners[ui.TimeChangeHandler]
}

// CreateTimePicker implements ui.Window.CreateTimePicker.
func (w *Window) CreateTimePicker(options ui.TimePickerOptions) (ui.TimePicker, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	elements := datePickerHourMinute
	if options.ShowSeconds {
		elements = datePickerHourMinuteSecond
	}
	result := &timePicker{}
	native := window.newDatePicker(0, elements, options.Value.Seconds(), 0, 0, 0, 0,
		int(options.Clock))
	if result.widgetBase, err = w.newComponent(native, ui.RoleTimePicker, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *timePicker) Value() time.Duration {
	native := t.nativeView()
	if native == nil {
		return 0
	}
	value, _ := native.pickedDate()
	seconds := math.Mod(value, 24*60*60)
	if seconds < 0 {
		seconds += 24 * 60 * 60
	}
	return time.Duration(math.Round(seconds)) * time.Second
}

func (t *timePicker) SetValue(value time.Duration) error {
	if err := (ui.TimePickerOptions{Value: value}).Validate(); err != nil {
		return err
	}
	native := t.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setPickedDate(value.Seconds(), 0)
	return nil
}

func (t *timePicker) OnChange(handler ui.TimeChangeHandler) ui.Unsubscribe {
	return subscribe(&t.changes, handler, handler == nil)
}

func (t *timePicker) receiveEvent(event any) {
	if _, ok := event.(changeEvent); ok {
		value := t.Value()
		each(&t.changes, func(handler ui.TimeChangeHandler) {
			handler(value)
		})
	}
}

// calendar is a CalendarView. A range shows as a contiguous set of selected
// days.
type calendar struct {
	*widgetBase
	selection ui.CalendarSelection
	changes   listeners[ui.DatesChangeHandler]
}

// CreateCalendar implements ui.Window.CreateCalendar.
func (w *Window) CreateCalendar(options ui.CalendarOptions) (ui.Calendar, error) {
	if options.Selection > ui.CalendarRange {
		return nil, errors.New("ui: invalid calendar selection")
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	dates, err := calendarDates(options.Selection, options.SelectedDates)
	if err != nil {
		return nil, err
	}
	if len(dates) == 0 && options.Selection != ui.CalendarMultiple {
		dates = []time.Time{dateOnly(time.Now())}
	}
	seconds := calendarSeconds(dates)
	result := &calendar{selection: options.Selection}
	native := window.newCalendar(int(options.Selection), seconds, unixSeconds(monthOf(options.VisibleMonth)))
	if result.widgetBase, err = w.newComponent(native, ui.RoleCalendar, result); err != nil {
		return nil, err
	}
	return result, nil
}

func monthOf(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// calendarDates validates selected dates for selection and returns them as
// sorted UTC dates. A range keeps only its first and last dates.
func calendarDates(selection ui.CalendarSelection, dates []time.Time) ([]time.Time, error) {
	if selection == ui.CalendarSingle && len(dates) > 1 {
		return nil, errors.New("ui: a single-selection calendar accepts one date")
	}
	result := make([]time.Time, 0, len(dates))
	for _, date := range dates {
		date = dateOnly(date)
		if !slices.ContainsFunc(result, date.Equal) {
			result = append(result, date)
		}
	}
	slices.SortFunc(result, func(a, b time.Time) int {
		return a.Compare(b)
	})
	if selection == ui.CalendarRange && len(result) > 2 {
		result = []time.Time{result[0], result[len(result)-1]}
	}
	return result, nil
}

func calendarSeconds(dates []time.Time) []float64 {
	seconds := make([]float64, len(dates))
	for i, date := range dates {
		seconds[i] = unixSeconds(date)
	}
	return seconds
}

// VisibleMonth returns the first day of the month that the calendar shows.
func (c *calendar) VisibleMonth() time.Time {
	native := c.nativeView()
	if native == nil {
		return time.Time{}
	}
	return calendarDate(float64(native.visibleMonth()))
}

// SetVisibleMonth shows the month of month without changing the selection.
func (c *calendar) SetVisibleMonth(month time.Time) {
	if native := c.nativeView(); native != nil && !month.IsZero() {
		native.setVisibleMonth(unixSeconds(monthOf(month)))
	}
}

// SelectedDates returns the selected dates in ascending order.
func (c *calendar) SelectedDates() []time.Time {
	native := c.nativeView()
	if native == nil {
		return nil
	}
	seconds := native.calendarDates()
	if len(seconds) == 0 {
		return nil
	}
	dates := make([]time.Time, len(seconds))
	for i, value := range seconds {
		dates[i] = calendarDate(value)
	}
	return dates
}

// SetSelectedDates replaces the selection and shows the month of the first
// date.
func (c *calendar) SetSelectedDates(dates []time.Time) error {
	selected, err := calendarDates(c.selection, dates)
	if err != nil {
		return err
	}
	if len(selected) == 0 && c.selection != ui.CalendarMultiple {
		return errors.New("ui: a calendar needs at least one selected date")
	}
	native := c.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	seconds := calendarSeconds(selected)
	native.setCalendarDates(seconds)
	return nil
}

func (c *calendar) OnChange(handler ui.DatesChangeHandler) ui.Unsubscribe {
	return subscribe(&c.changes, handler, handler == nil)
}

func (c *calendar) receiveEvent(event any) {
	if _, ok := event.(changeEvent); ok {
		dates := c.SelectedDates()
		each(&c.changes, func(handler ui.DatesChangeHandler) {
			handler(append([]time.Time(nil), dates...))
		})
	}
}

// ---- Color and font ----

type colorPicker struct {
	*widgetBase
	allowsAlpha bool
	changes     listeners[ui.ColorChangeHandler]
}

func colorComponents(value color.Color) [4]float64 {
	if value == nil {
		value = color.Black
	}
	rgba := color.NRGBAModel.Convert(value).(color.NRGBA)
	return [4]float64{float64(rgba.R) / 255, float64(rgba.G) / 255, float64(rgba.B) / 255, float64(rgba.A) / 255}
}

// CreateColorPicker implements ui.Window.CreateColorPicker.
func (w *Window) CreateColorPicker(options ui.ColorPickerOptions) (ui.ColorPicker, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	c := colorComponents(options.Color)
	if !options.AllowAlpha {
		c[3] = 1
	}
	result := &colorPicker{allowsAlpha: options.AllowAlpha}
	native := window.newColorWell(c[0], c[1], c[2], c[3], options.AllowAlpha)
	if result.widgetBase, err = w.newComponent(native, ui.RoleColorPicker, result); err != nil {
		return nil, err
	}
	return result, nil
}

// Color returns the color as non-premultiplied sRGB stored in color.RGBA.
func (c *colorPicker) Color() color.RGBA {
	native := c.nativeView()
	if native == nil {
		return color.RGBA{}
	}
	rgba := native.colorWellColor()
	channel := func(value float64) uint8 {
		return uint8(math.Round(math.Max(0, math.Min(1, value)) * 255))
	}
	return color.RGBA{R: channel(rgba[0]), G: channel(rgba[1]), B: channel(rgba[2]), A: channel(rgba[3])}
}

func (c *colorPicker) SetColor(value color.Color) {
	if native := c.nativeView(); native != nil {
		components := colorComponents(value)
		if !c.allowsAlpha {
			components[3] = 1
		}
		native.setColorWellColor(components[0], components[1], components[2], components[3])
	}
}

func (c *colorPicker) AllowsAlpha() bool {
	return c.allowsAlpha
}

func (c *colorPicker) OnChange(handler ui.ColorChangeHandler) ui.Unsubscribe {
	return subscribe(&c.changes, handler, handler == nil)
}

func (c *colorPicker) receiveEvent(event any) {
	if _, ok := event.(changeEvent); ok {
		value := c.Color()
		each(&c.changes, func(handler ui.ColorChangeHandler) {
			handler(value)
		})
	}
}

type fontPicker struct {
	*widgetBase
	font    ui.Font
	changes listeners[ui.FontChangeHandler]
}

// CreateFontPicker implements ui.Window.CreateFontPicker.
func (w *Window) CreateFontPicker(options ui.FontPickerOptions) (ui.FontPicker, error) {
	if err := options.Font.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &fontPicker{font: options.Font}
	native := window.newFontPicker(options.Font.Family, options.Font.Size, int(options.Font.Weight),
		options.Font.Italic)
	if result.widgetBase, err = w.newComponent(native, ui.RoleFontPicker, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (f *fontPicker) Font() ui.Font {
	return f.font
}

func (f *fontPicker) SetFont(font ui.Font) error {
	if err := font.Validate(); err != nil {
		return err
	}
	native := f.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setPickedFont(font.Family, font.Size, int(font.Weight), font.Italic)
	f.font = font
	return nil
}

func (f *fontPicker) OnChange(handler ui.FontChangeHandler) ui.Unsubscribe {
	return subscribe(&f.changes, handler, handler == nil)
}

func (f *fontPicker) receiveEvent(event any) {
	if _, ok := event.(changeEvent); !ok {
		return
	}
	native := f.nativeView()
	if native == nil {
		return
	}
	family, size, weight, italic := native.pickedFont()
	f.font = ui.Font{Family: family, Size: size, Weight: ui.FontWeight(weight), Italic: italic}
	font := f.font
	each(&f.changes, func(handler ui.FontChangeHandler) {
		handler(font)
	})
}

// ---- Key recorder ----

type keyRecorder struct {
	*widgetBase
	shortcut ui.Shortcut
	changes  listeners[ui.ShortcutChangeHandler]
}

// CreateKeyRecorder implements ui.Window.CreateKeyRecorder.
func (w *Window) CreateKeyRecorder(options ui.KeyRecorderOptions) (ui.KeyRecorder, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &keyRecorder{}
	native := window.newKeyRecorder()
	if result.widgetBase, err = w.newComponent(native, ui.RoleButton, result); err != nil {
		return nil, err
	}
	result.SetShortcut(options.Shortcut)
	return result, nil
}

func (k *keyRecorder) Shortcut() ui.Shortcut {
	return k.shortcut
}

func (k *keyRecorder) SetShortcut(shortcut ui.Shortcut) {
	k.shortcut = shortcut
	if native := k.nativeView(); native != nil {
		native.setRecordedKey(shortcutText(shortcut))
	}
}

func (k *keyRecorder) OnChange(handler ui.ShortcutChangeHandler) ui.Unsubscribe {
	return subscribe(&k.changes, handler, handler == nil)
}

// keyNames names keys whose characters are not printable.
var keyNames = map[int]string{
	257: "Return", 258: "Tab", 259: "Backspace", 256: "Escape", 32: "Space", 261: "Delete",
	262: "Right", 263: "Left", 264: "Down", 265: "Up", 266: "PageUp", 267: "PageDown", 268: "Home", 269: "End",
}

func (k *keyRecorder) receiveEvent(event any) {
	record, ok := event.(keyRecordEvent)
	if !ok {
		return
	}
	shortcut := ui.Shortcut{}
	if record.label != "" || record.key != 0 {
		key := strings.ToUpper(record.label)
		if name, ok := keyNames[record.key]; ok {
			key = name
		} else if record.key >= 290 && record.key <= 314 {
			key = fmt.Sprintf("F%d", record.key-289)
		}
		shortcut = ui.Shortcut{
			Key:     key,
			Command: record.modifiers&1 != 0,
			Control: record.modifiers&2 != 0,
			Alt:     record.modifiers&4 != 0,
			Shift:   record.modifiers&8 != 0,
		}
	}
	k.SetShortcut(shortcut)
	each(&k.changes, func(handler ui.ShortcutChangeHandler) {
		handler(shortcut)
	})
}

var (
	_ ui.ToggleButton     = (*toggleButton)(nil)
	_ ui.Switch           = (*switchControl)(nil)
	_ ui.PasswordField    = (*passwordField)(nil)
	_ ui.SearchField      = (*searchField)(nil)
	_ ui.NumberField      = (*numberField)(nil)
	_ ui.EditableCombo    = (*editableCombo)(nil)
	_ ui.SegmentedControl = (*segmentedControl)(nil)
	_ ui.Stepper          = (*stepper)(nil)
	_ ui.Dial             = (*dial)(nil)
	_ ui.RangeSlider      = (*rangeSlider)(nil)
	_ ui.DatePicker       = (*datePicker)(nil)
	_ ui.TimePicker       = (*timePicker)(nil)
	_ ui.Calendar         = (*calendar)(nil)
	_ ui.ColorPicker      = (*colorPicker)(nil)
	_ ui.FontPicker       = (*fontPicker)(nil)
	_ ui.KeyRecorder      = (*keyRecorder)(nil)
)
