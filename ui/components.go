package ui

import (
	"errors"
	"fmt"
	stdimage "image"
	"image/color"
	"math"
	"time"
)

func validateRange(name string, minimum, maximum float64, values ...float64) error {
	for _, value := range append([]float64{minimum, maximum}, values...) {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("ui: %s values must be finite", name)
		}
	}
	if minimum >= maximum {
		return fmt.Errorf("ui: %s minimum must be less than maximum", name)
	}
	for _, value := range values {
		if value < minimum || value > maximum {
			return fmt.Errorf("ui: %s value must be within its range", name)
		}
	}
	return nil
}

func validateStep(name string, step float64) error {
	if math.IsNaN(step) || math.IsInf(step, 0) || step < 0 {
		return fmt.Errorf("ui: %s step must be finite and non-negative", name)
	}
	return nil
}

func validateIDs(kind string, ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for i, id := range ids {
		if id == "" {
			return fmt.Errorf("ui: %s %d has no ID", kind, i)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("ui: duplicate %s ID %q", kind, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

// ---- Value and choice controls ----

// ToggleButtonPresentation selects a toggle button's native style.
type ToggleButtonPresentation uint8

const (
	// ToggleButtonStandard is a normal push button.
	ToggleButtonStandard ToggleButtonPresentation = iota

	// ToggleButtonToolbar is a borderless button for toolbars.
	ToggleButtonToolbar
)

// ToggleButtonOptions describes a push button that stays pressed. Buttons in
// the same window with the same non-empty GroupID are mutually exclusive.
type ToggleButtonOptions struct {
	// Text is the label of the button.
	Text string

	// Icon is the image of the button.
	Icon IconSource

	// Checked presses the button initially.
	Checked bool

	// GroupID names the group of mutually exclusive buttons. An empty GroupID
	// makes the button independent.
	GroupID string

	// Presentation is the style of the button.
	Presentation ToggleButtonPresentation
}

// ToggleButton is a push button with an on/off state.
type ToggleButton interface {
	Widget

	// Text returns the label of the button.
	Text() string

	// SetText changes the label of the button.
	SetText(text string)

	// IsChecked reports whether the button is pressed.
	IsChecked() bool

	// SetChecked presses or releases the button. Pressing a button releases the
	// other buttons of its group.
	SetChecked(checked bool)

	// GroupID returns the group of the button.
	GroupID() string

	// OnChange runs handler after the user presses or releases the button.
	OnChange(handler ToggleHandler) Unsubscribe
}

// SwitchOptions describes an on/off switch. OnLabel and OffLabel describe the
// states for assistive technologies and platforms that show them.
type SwitchOptions struct {
	// Text is the label of the switch.
	Text string

	// OnLabel describes the on state.
	OnLabel string

	// OffLabel describes the off state.
	OffLabel string

	// Checked turns the switch on initially.
	Checked bool

	// Size is the size of the switch.
	Size ControlSize
}

// Switch is a native on/off switch.
type Switch interface {
	Widget

	// Text returns the label of the switch.
	Text() string

	// SetText changes the label of the switch.
	SetText(text string)

	// IsChecked reports whether the switch is on.
	IsChecked() bool

	// SetChecked turns the switch on or off.
	SetChecked(checked bool)

	// OnChange runs handler after the user turns the switch on or off.
	OnChange(handler ToggleHandler) Unsubscribe
}

// PasswordReveal controls whether the user can show a password's text.
type PasswordReveal uint8

const (
	// PasswordRevealNone never shows the text.
	PasswordRevealNone PasswordReveal = iota

	// PasswordRevealButton shows a button that shows and hides the text.
	PasswordRevealButton
)

// PasswordFieldOptions describes a secure text field. AllowCopy permits
// copying the secret text.
type PasswordFieldOptions struct {
	// Placeholder is the text that the empty field shows.
	Placeholder string

	// Reveal controls whether the user can show the text.
	Reveal PasswordReveal

	// AllowCopy lets the user copy the text.
	AllowCopy bool
}

// PasswordField is a native secure text field.
type PasswordField interface {
	Widget

	// Text returns the secret text.
	Text() string

	// SetText changes the secret text.
	SetText(text string)

	// Placeholder returns the text that the empty field shows.
	Placeholder() string

	// SetPlaceholder changes the text that the empty field shows.
	SetPlaceholder(placeholder string)

	// IsRevealed reports whether the field shows the text.
	IsRevealed() bool

	// SetRevealed returns ErrNotSupported when Reveal is PasswordRevealNone.
	SetRevealed(revealed bool) error

	// OnChange runs handler after each edit by the user.
	OnChange(handler TextChangeHandler) Unsubscribe

	// OnSubmit runs handler when the user presses Return.
	OnSubmit(handler TextChangeHandler) Unsubscribe
}

// SearchFieldOptions describes a native search field.
type SearchFieldOptions struct {
	// Text is the initial search text.
	Text string

	// Placeholder is the text that the empty field shows.
	Placeholder string

	// Suggestions are the completions that the field offers.
	Suggestions []string
}

// SearchField is a native search field. OnSubmit receives the query when the
// user presses Return.
type SearchField interface {
	Widget

	// Text returns the search text.
	Text() string

	// SetText changes the search text.
	SetText(text string)

	// Placeholder returns the text that the empty field shows.
	Placeholder() string

	// SetPlaceholder changes the text that the empty field shows.
	SetPlaceholder(placeholder string)

	// Suggestions returns the completions that the field offers.
	Suggestions() []string

	// SetSuggestions replaces the completions.
	SetSuggestions(suggestions []string)

	// Clear removes the search text.
	Clear()

	// OnChange runs handler after each edit by the user.
	OnChange(handler TextChangeHandler) Unsubscribe

	// OnSubmit runs handler when the user presses Return.
	OnSubmit(handler TextChangeHandler) Unsubscribe
}

// NumberFieldOptions describes a numeric text field. Decimals is the number of
// displayed fractional digits.
type NumberFieldOptions struct {
	// Minimum is the smallest value.
	Minimum float64

	// Maximum is the largest value.
	Maximum float64

	// Value is the initial value.
	Value float64

	// Step is the change for each step.
	Step float64

	// Decimals is the number of fractional digits that the field shows.
	Decimals int
}

// Validate checks the range, value, step, and precision.
func (options NumberFieldOptions) Validate() error {
	if err := validateRange("number field", options.Minimum, options.Maximum, options.Value); err != nil {
		return err
	}
	if options.Decimals < 0 || options.Decimals > 15 {
		return errors.New("ui: number field decimals must be between 0 and 15")
	}
	return validateStep("number field", options.Step)
}

// NumberField is a numeric text field with stepping.
type NumberField interface {
	Widget

	// Value returns the current value.
	Value() float64

	// SetValue changes the value. It returns an error when value is outside the
	// range.
	SetValue(value float64) error

	// Minimum returns the smallest value.
	Minimum() float64

	// Maximum returns the largest value.
	Maximum() float64

	// Step returns the change for each step.
	Step() float64

	// Decimals returns the number of fractional digits.
	Decimals() int

	// OnChange runs handler after the user changes the value.
	OnChange(handler ValueChangeHandler) Unsubscribe
}

// EditableComboOptions describes a text field with a list of suggestions.
type EditableComboOptions struct {
	// Text is the initial text.
	Text string

	// Items are the suggestions in the list.
	Items []string

	// AllowFreeText lets the user enter text that is not in Items.
	AllowFreeText bool
}

// EditableCombo is a native combo box. SelectedIndex is the index of the item
// equal to Text, or -1.
type EditableCombo interface {
	Widget

	// Text returns the text in the field.
	Text() string

	// SetText changes the text in the field.
	SetText(text string)

	// Items returns the suggestions.
	Items() []string

	// SetItems replaces the suggestions.
	SetItems(items []string)

	// SelectedIndex returns the index of the item that equals the text, or -1.
	SelectedIndex() int

	// OnChange runs handler after the user changes the text.
	OnChange(handler TextChangeHandler) Unsubscribe
}

// Segment is one segment of a segmented control.
type Segment struct {
	// ID identifies the segment.
	ID string

	// Text is the label of the segment.
	Text string

	// Icon is the image of the segment.
	Icon IconSource

	// Enabled lets the user select the segment.
	Enabled bool
}

// SegmentedControlOptions describes a native segmented control.
type SegmentedControlOptions struct {
	// Items are the segments in order.
	Items []Segment

	// SelectedIDs are the IDs of the initially selected segments.
	SelectedIDs []string

	// Mode is how many segments the user can select.
	Mode SelectionMode
}

// Validate checks segment IDs and the initial selection.
func (options SegmentedControlOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: segmented control requires at least one segment")
	}
	ids := make([]string, len(options.Items))
	for i, item := range options.Items {
		ids[i] = item.ID
	}
	if err := validateIDs("segment", ids); err != nil {
		return err
	}
	if options.Mode > SelectionMultiple {
		return errors.New("ui: invalid selection mode")
	}
	return validateSelectedIDs(ids, options.SelectedIDs, options.Mode)
}

func validateSelectedIDs(ids, selected []string, mode SelectionMode) error {
	if mode == SelectionNone && len(selected) > 0 || mode == SelectionSingle && len(selected) > 1 {
		return errors.New("ui: selection has too many items for its mode")
	}
	known := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		known[id] = struct{}{}
	}
	for _, id := range selected {
		if _, exists := known[id]; !exists {
			return fmt.Errorf("ui: selected ID %q does not exist", id)
		}
	}
	return nil
}

// ValidateSelectedIDs checks that selected names known IDs and fits mode.
func ValidateSelectedIDs(ids, selected []string, mode SelectionMode) error {
	return validateSelectedIDs(ids, selected, mode)
}

// SelectionIDsHandler receives the selected IDs after a user change.
type SelectionIDsHandler func(ids []string)

// SegmentedControl is a native segmented control.
type SegmentedControl interface {
	Widget

	// Items returns the segments.
	Items() []Segment

	// SelectedIDs returns the IDs of the selected segments.
	SelectedIDs() []string

	// SetSelectedIDs selects the segments with ids.
	SetSelectedIDs(ids []string) error

	// OnChange runs handler after the user changes the selection.
	OnChange(handler SelectionIDsHandler) Unsubscribe
}

// StepperOptions describes up/down stepping. Wrap continues from the other end
// of the range.
type StepperOptions struct {
	// Minimum is the smallest value.
	Minimum float64

	// Maximum is the largest value.
	Maximum float64

	// Value is the initial value.
	Value float64

	// Step is the change for each step.
	Step float64

	// Wrap continues from the other end of the range.
	Wrap bool
}

// Validate checks the range, value, and step.
func (options StepperOptions) Validate() error {
	if err := validateRange("stepper", options.Minimum, options.Maximum, options.Value); err != nil {
		return err
	}
	return validateStep("stepper", options.Step)
}

// Stepper is a native stepper control.
type Stepper interface {
	Widget

	// Value returns the current value.
	Value() float64

	// SetValue changes the value. It returns an error when value is outside the
	// range.
	SetValue(value float64) error

	// Increment adds one step to the value.
	Increment()

	// Decrement subtracts one step from the value.
	Decrement()

	// OnChange runs handler after the user changes the value.
	OnChange(handler ValueChangeHandler) Unsubscribe
}

// RangeSliderOptions describes a two-thumb slider.
type RangeSliderOptions struct {
	// Minimum is the smallest value.
	Minimum float64

	// Maximum is the largest value.
	Maximum float64

	// LowerValue is the initial start of the range.
	LowerValue float64

	// UpperValue is the initial end of the range.
	UpperValue float64

	// Step is the distance between the values that the user can select. Zero
	// allows all values.
	Step float64
}

// Validate checks the range and ordered values.
func (options RangeSliderOptions) Validate() error {
	if err := validateRange("range slider", options.Minimum, options.Maximum, options.LowerValue, options.UpperValue); err != nil {
		return err
	}
	if options.LowerValue > options.UpperValue {
		return errors.New("ui: range slider lower value exceeds upper value")
	}
	return validateStep("range slider", options.Step)
}

// RangeChangeHandler receives a range after a user change.
type RangeChangeHandler func(lower, upper float64)

// RangeSlider selects a sub-range.
type RangeSlider interface {
	Widget

	// LowerValue returns the start of the range.
	LowerValue() float64

	// UpperValue returns the end of the range.
	UpperValue() float64

	// SetValues changes the range. It returns an error when lower is larger than
	// upper or a value is outside the limits.
	SetValues(lower, upper float64) error

	// OnChange runs handler after the user changes the range.
	OnChange(handler RangeChangeHandler) Unsubscribe
}

// DatePickerOptions describes a date picker. Zero Minimum or Maximum leaves
// that bound open. Values are calendar dates; the time of day is ignored.
type DatePickerOptions struct {
	// Value is the initial date.
	Value time.Time

	// Minimum is the earliest date.
	Minimum time.Time

	// Maximum is the latest date.
	Maximum time.Time
}

// DateChangeHandler receives a date after a user change.
type DateChangeHandler func(date time.Time)

// DatePicker is a native date picker.
type DatePicker interface {
	Widget

	// Value returns the selected date.
	Value() time.Time

	// SetValue selects date. It returns an error when date is outside the limits.
	SetValue(date time.Time) error

	// OnChange runs handler after the user selects a date.
	OnChange(handler DateChangeHandler) Unsubscribe
}

// ClockFormat selects 12- or 24-hour display.
type ClockFormat uint8

const (
	// ClockSystem follows the setting of the user.
	ClockSystem ClockFormat = iota

	// Clock12Hour shows hours from 1 to 12 with AM and PM.
	Clock12Hour

	// Clock24Hour shows hours from 0 to 23.
	Clock24Hour
)

// TimePickerOptions describes a time-of-day picker. Value is the time since
// midnight.
type TimePickerOptions struct {
	// Value is the initial time since midnight.
	Value time.Duration

	// ShowSeconds shows and edits seconds.
	ShowSeconds bool

	// Clock is the hour format.
	Clock ClockFormat
}

// Validate checks that the value is within one day.
func (options TimePickerOptions) Validate() error {
	if options.Value < 0 || options.Value >= 24*time.Hour {
		return errors.New("ui: time picker value must be within one day")
	}
	return nil
}

// TimeChangeHandler receives a time of day after a user change.
type TimeChangeHandler func(value time.Duration)

// TimePicker is a native time-of-day picker.
type TimePicker interface {
	Widget

	// Value returns the selected time since midnight.
	Value() time.Duration

	// SetValue changes the time. It returns an error when value is not within one
	// day.
	SetValue(value time.Duration) error

	// OnChange runs handler after the user changes the time.
	OnChange(handler TimeChangeHandler) Unsubscribe
}

// ColorPickerOptions describes a color well.
type ColorPickerOptions struct {
	// Color is the initial color.
	Color color.Color

	// AllowAlpha lets the user select the opacity.
	AllowAlpha bool
}

// ColorChangeHandler receives a color after a user change.
type ColorChangeHandler func(value color.RGBA)

// ColorPicker is a native color well that opens the platform color panel.
type ColorPicker interface {
	Widget

	// Color returns the selected color.
	Color() color.RGBA

	// SetColor changes the color.
	SetColor(value color.Color)

	// AllowsAlpha reports whether the user can select the opacity.
	AllowsAlpha() bool

	// OnChange runs handler after the user selects a color.
	OnChange(handler ColorChangeHandler) Unsubscribe
}

// FontPickerOptions describes a font chooser.
type FontPickerOptions struct {
	// Font is the initial font.
	Font Font
}

// FontChangeHandler receives a font after a user change.
type FontChangeHandler func(font Font)

// FontPicker shows the current font and opens the platform font panel.
type FontPicker interface {
	Widget

	// Font returns the selected font.
	Font() Font

	// SetFont changes the font.
	SetFont(font Font) error

	// OnChange runs handler after the user selects a font.
	OnChange(handler FontChangeHandler) Unsubscribe
}

// CalendarSelection controls how many dates a calendar selects.
type CalendarSelection uint8

const (
	// CalendarSingle selects one date.
	CalendarSingle CalendarSelection = iota

	// CalendarMultiple selects dates that need not be adjacent.
	CalendarMultiple

	// CalendarRange selects a contiguous range given by its first and last
	// dates.
	CalendarRange
)

// CalendarOptions describes a month calendar.
type CalendarOptions struct {
	// VisibleMonth is the month that the calendar shows first. The zero value
	// shows the month of the first selected date.
	VisibleMonth time.Time

	// SelectedDates are the initially selected dates.
	SelectedDates []time.Time

	// Selection is how many dates the user can select.
	Selection CalendarSelection
}

// DatesChangeHandler receives dates after a user change.
type DatesChangeHandler func(dates []time.Time)

// Calendar is a native month calendar.
type Calendar interface {
	Widget

	// VisibleMonth returns the month that the calendar shows.
	VisibleMonth() time.Time

	// SetVisibleMonth shows month without a change to the selection.
	SetVisibleMonth(month time.Time)

	// SelectedDates returns the selected dates.
	SelectedDates() []time.Time

	// SetSelectedDates selects dates.
	SetSelectedDates(dates []time.Time) error

	// OnChange runs handler after the user changes the selection.
	OnChange(handler DatesChangeHandler) Unsubscribe
}

// KeyRecorderOptions describes a shortcut recorder.
type KeyRecorderOptions struct {
	// Shortcut is the initial shortcut.
	Shortcut Shortcut
}

// ShortcutChangeHandler receives a shortcut after the user records one.
type ShortcutChangeHandler func(shortcut Shortcut)

// KeyRecorder records a keyboard shortcut.
type KeyRecorder interface {
	Widget

	// Shortcut returns the recorded shortcut.
	Shortcut() Shortcut

	// SetShortcut changes the shortcut.
	SetShortcut(shortcut Shortcut)

	// OnChange runs handler after the user records a shortcut.
	OnChange(handler ShortcutChangeHandler) Unsubscribe
}

// DialOptions describes a circular value control.
type DialOptions struct {
	// Minimum is the smallest value.
	Minimum float64

	// Maximum is the largest value.
	Maximum float64

	// Value is the initial value.
	Value float64

	// Step is the distance between the values that the user can select. Zero
	// allows all values.
	Step float64

	// Wrap continues from the other end of the range after a full turn.
	Wrap bool
}

// Validate checks the range, value, and step.
func (options DialOptions) Validate() error {
	if err := validateRange("dial", options.Minimum, options.Maximum, options.Value); err != nil {
		return err
	}
	return validateStep("dial", options.Step)
}

// Dial is a circular value control.
type Dial interface {
	Widget

	// Value returns the current value.
	Value() float64

	// SetValue changes the value. It returns an error when value is outside the
	// range.
	SetValue(value float64) error

	// OnChange runs handler after the user changes the value.
	OnChange(handler ValueChangeHandler) Unsubscribe
}

// ---- Containers and layouts ----

// AccordionExpansion controls how many accordion sections can be open.
type AccordionExpansion uint8

const (
	// AccordionMultiple lets many sections be open at the same time.
	AccordionMultiple AccordionExpansion = iota

	// AccordionSingle closes the open section when the user opens another.
	AccordionSingle
)

// AccordionSection is one section of an accordion.
type AccordionSection struct {
	// ID identifies the section.
	ID string

	// Title is the text of the header.
	Title string

	// Content is the widget that the section shows when open.
	Content Widget

	// Expanded opens the section initially.
	Expanded bool
}

// AccordionOptions describes a stack of collapsible sections.
type AccordionOptions struct {
	// Sections are the sections in order.
	Sections []AccordionSection

	// Expansion is how many sections can be open.
	Expansion AccordionExpansion
}

// Validate checks section IDs, content, and expansion.
func (options AccordionOptions) Validate() error {
	if len(options.Sections) == 0 {
		return errors.New("ui: accordion requires at least one section")
	}
	ids := make([]string, len(options.Sections))
	expanded := 0
	for i, section := range options.Sections {
		ids[i] = section.ID
		if section.Content == nil {
			return fmt.Errorf("ui: accordion section %d has no content", i)
		}
		if section.Expanded {
			expanded++
		}
	}
	if options.Expansion == AccordionSingle && expanded > 1 {
		return errors.New("ui: single-expansion accordion has several expanded sections")
	}
	return validateIDs("accordion section", ids)
}

// Accordion is a stack of collapsible sections.
type Accordion interface {
	Widget

	// Sections returns the sections.
	Sections() []AccordionSection

	// ExpandedIDs returns the IDs of the open sections.
	ExpandedIDs() []string

	// SetExpanded opens or closes the section with id.
	SetExpanded(id string, expanded bool) error

	// OnChange runs handler with the open section IDs after the user opens or
	// closes a section.
	OnChange(handler SelectionIDsHandler) Unsubscribe
}

// SplitPane is one pane of a split view.
type SplitPane struct {
	// Content is the widget in the pane.
	Content Widget

	// Minimum is the smallest size of the pane along the split direction.
	Minimum int

	// Collapsible lets the user collapse the pane.
	Collapsible bool
}

// SplitViewOptions describes resizable panes. Dividers are the positions of
// the dividers between panes, in logical pixels along Direction.
type SplitViewOptions struct {
	// Direction is the direction in which the panes follow each other.
	Direction LayoutDirection

	// Panes are the panes in order.
	Panes []SplitPane

	// Dividers are the initial divider positions. Empty Dividers share the space
	// equally.
	Dividers []int
}

// Validate checks panes and divider positions.
func (options SplitViewOptions) Validate() error {
	if len(options.Panes) < 2 {
		return errors.New("ui: split view requires at least two panes")
	}
	for i, pane := range options.Panes {
		if pane.Content == nil {
			return fmt.Errorf("ui: split pane %d has no content", i)
		}
	}
	if len(options.Dividers) != 0 && len(options.Dividers) != len(options.Panes)-1 {
		return errors.New("ui: split view needs one divider position per adjacent pane pair")
	}
	return nil
}

// SplitView arranges panes with user-resizable dividers. When the split view
// changes size, the last pane gets the difference and the other panes keep
// their size.
type SplitView interface {
	Widget

	// Panes returns the panes.
	Panes() []SplitPane

	// Dividers returns the divider positions.
	Dividers() []int

	// SetDividers moves the dividers. There is one position for each pair of
	// adjacent panes.
	SetDividers(positions []int) error

	// OnChange runs handler with the divider positions after they change.
	OnChange(handler func(positions []int)) Unsubscribe
}

// GroupBoxOptions describes a titled box around content.
type GroupBoxOptions struct {
	// Title is the title of the frame.
	Title string

	// Subtitle is the secondary text below the title.
	Subtitle string

	// Content is the widget in the frame.
	Content Widget
}

// GroupBox is a native titled box.
type GroupBox interface {
	Widget
	// Title returns the title of the frame.
	Title() string

	// SetTitle changes the title of the frame.
	SetTitle(title string)

	// Subtitle returns the secondary text.
	Subtitle() string

	// SetSubtitle changes the secondary text.
	SetSubtitle(subtitle string)

	// Content returns the widget in the frame.
	Content() Widget
}

// PopoverOptions describes transient content anchored to a widget.
type PopoverOptions struct {
	// Anchor is the widget that the popover points to.
	Anchor Widget

	// Content is the widget in the popover.
	Content Widget

	// PreferredEdge is the side of the anchor where the popover opens when there
	// is space.
	PreferredEdge Edge

	// Transient closes the popover when the user interacts elsewhere.
	Transient bool
}

// Popover shows content in a native popover. It is not part of the window's
// widget tree.
type Popover interface {
	// Close releases the popover and its content.
	Close()

	// Show opens the popover at its anchor. The anchor must be in a visible
	// window.
	Show() error

	// Hide closes the popover. It does not release it.
	Hide()

	// IsShown reports whether the popover is open.
	IsShown() bool

	// Content returns the widget in the popover.
	Content() Widget

	// OnHide runs handler after the popover closes.
	OnHide(handler FocusHandler) Unsubscribe
}

// DrawerOptions describes a panel that slides from a window edge.
type DrawerOptions struct {
	// Edge is the side of the window where the drawer opens.
	Edge Edge

	// Content is the widget in the drawer.
	Content Widget

	// Open opens the drawer initially.
	Open bool

	// Width is the size of the drawer across its edge in logical pixels.
	Width int
}

// Drawer is a sliding side panel.
type Drawer interface {
	Widget

	// Content returns the widget in the drawer.
	Content() Widget

	// IsOpen reports whether the drawer is open.
	IsOpen() bool

	// SetOpen opens or closes the drawer.
	SetOpen(open bool)

	// OnChange runs handler after the drawer opens or closes.
	OnChange(handler ExpansionChangeHandler) Unsubscribe
}

// GridTrack sizes one grid row or column: a fixed Size, a Grow weight, or the
// content's preferred size when both are zero.
type GridTrack struct {
	// Size is the fixed size of the track in logical pixels.
	Size int

	// Grow is the share of the remaining space that the track receives.
	Grow float32
}

// GridItem places content in a grid cell. Zero spans are one.
type GridItem struct {
	// Content is the widget in the cell.
	Content Widget

	// Column is the index of the first column of the cell.
	Column int

	// Row is the index of the first row of the cell.
	Row int

	// ColumnSpan is the number of columns of the cell.
	ColumnSpan int

	// RowSpan is the number of rows of the cell.
	RowSpan int

	// Alignment places the content in the cell.
	Alignment Alignment
}

// GridLayoutOptions describes a two-dimensional layout.
type GridLayoutOptions struct {
	// Columns are the columns in order.
	Columns []GridTrack

	// Rows are the rows in order.
	Rows []GridTrack

	// Items are the cells.
	Items []GridItem

	// RowGap is the space between rows.
	RowGap int

	// ColumnGap is the space between columns.
	ColumnGap int

	// Padding is the space between the edges and the cells.
	Padding Insets
}

// Validate checks tracks and item placement.
func (options GridLayoutOptions) Validate() error {
	if len(options.Columns) == 0 || len(options.Rows) == 0 {
		return errors.New("ui: grid requires at least one row and one column")
	}
	if options.RowGap < 0 || options.ColumnGap < 0 {
		return errors.New("ui: grid gaps must be non-negative")
	}
	for i, item := range options.Items {
		columnSpan, rowSpan := max(1, item.ColumnSpan), max(1, item.RowSpan)
		if item.Content == nil {
			return fmt.Errorf("ui: grid item %d has no content", i)
		}
		if item.Column < 0 || item.Row < 0 || item.ColumnSpan < 0 || item.RowSpan < 0 ||
			item.Column+columnSpan > len(options.Columns) || item.Row+rowSpan > len(options.Rows) {
			return fmt.Errorf("ui: grid item %d is outside the grid", i)
		}
	}
	return nil
}

// GridLayout arranges children in rows and columns.
type GridLayout interface {
	Widget
	// Options returns the arrangement of the grid.
	Options() GridLayoutOptions

	// SetOptions replaces the arrangement and the children.
	SetOptions(options GridLayoutOptions) error

	// Children returns the widgets in the grid.
	Children() []Widget
}

// FlowLayoutOptions describes a layout that places children in lines.
type FlowLayoutOptions struct {
	// Direction is the direction of the lines.
	Direction LayoutDirection

	// Wrap starts a new line when a child does not fit.
	Wrap bool

	// RowGap is the space between lines.
	RowGap int

	// ColumnGap is the space between children on a line.
	ColumnGap int

	// Children are the widgets in order.
	Children []Widget
}

// FlowLayout places children in lines and wraps them when they do not fit.
type FlowLayout interface {
	Widget

	// Children returns the widgets in the layout.
	Children() []Widget

	// SetChildren replaces the widgets in the layout.
	SetChildren(children []Widget) error
}

// OverlayItem places content over others at an anchor.
type OverlayItem struct {
	// Content is the widget.
	Content Widget

	// Anchor is the position of the widget in the layout.
	Anchor Anchor

	// Offset moves the widget from its anchor.
	Offset Point
}

// OverlayLayoutOptions describes stacked content; later items draw on top.
type OverlayLayoutOptions struct {
	// Items are the widgets from back to front.
	Items []OverlayItem
}

// OverlayLayout stacks children.
type OverlayLayout interface {
	Widget

	// Items returns the widgets from back to front.
	Items() []OverlayItem

	// SetItems replaces the widgets.
	SetItems(items []OverlayItem) error
}

// FormRow is one labeled form field.
type FormRow struct {
	// ID identifies the row.
	ID string

	// Label is the text before the editor.
	Label string

	// Editor is the widget that edits the value.
	Editor Widget

	// Required marks a field that the user must fill.
	Required bool

	// Help is the text below the editor.
	Help string
}

// FormLayoutOptions describes a labeled form.
type FormLayoutOptions struct {
	// Rows are the rows in order.
	Rows []FormRow
}

// Validate checks row IDs and editors.
func (options FormLayoutOptions) Validate() error {
	ids := make([]string, len(options.Rows))
	for i, row := range options.Rows {
		ids[i] = row.ID
		if row.Editor == nil {
			return fmt.Errorf("ui: form row %d has no editor", i)
		}
	}
	return validateIDs("form row", ids)
}

// FormLayout aligns labels and editors using platform form conventions.
type FormLayout interface {
	Widget

	// Rows returns the rows.
	Rows() []FormRow

	// SetRows replaces the rows.
	SetRows(rows []FormRow) error

	// SetError shows an inline error under a row; an empty message clears it.
	SetError(id, message string) error
}

// DockArea is the region a docked pane occupies.
type DockArea uint8

const (
	// DockCenter is the main area.
	DockCenter DockArea = iota

	// DockLeft is the area at the left edge.
	DockLeft

	// DockRight is the area at the right edge.
	DockRight

	// DockTop is the area at the top edge.
	DockTop

	// DockBottom is the area at the bottom edge.
	DockBottom
)

// DockPane is one dockable pane.
type DockPane struct {
	// ID identifies the pane.
	ID string

	// Title is the text of the header.
	Title string

	// Content is the widget in the pane.
	Content Widget

	// Area is the area of the pane.
	Area DockArea
}

// DockingLayoutOptions describes dockable panes.
type DockingLayoutOptions struct {
	// Panes are the panes in order.
	Panes []DockPane
}

// Validate checks pane IDs and content.
func (options DockingLayoutOptions) Validate() error {
	if len(options.Panes) == 0 {
		return errors.New("ui: docking layout requires at least one pane")
	}
	ids := make([]string, len(options.Panes))
	for i, pane := range options.Panes {
		ids[i] = pane.ID
		if pane.Content == nil {
			return fmt.Errorf("ui: dock pane %d has no content", i)
		}
	}
	return validateIDs("dock pane", ids)
}

// DockingLayout arranges panes that the user can move between areas.
type DockingLayout interface {
	Widget

	// Panes returns the panes.
	Panes() []DockPane

	// MovePane moves the pane with id to area.
	MovePane(id string, area DockArea) error

	// SetPaneVisible shows or hides the pane with id.
	SetPaneVisible(id string, visible bool) error
}

// ---- Data views ----

// ListItem is one row of a list view.
type ListItem struct {
	// ID identifies the item.
	ID string

	// Text is the main text.
	Text string

	// Detail is the secondary text.
	Detail string

	// Icon is the image of the item.
	Icon IconSource
}

// ListViewOptions describes a single-column list.
type ListViewOptions struct {
	// Items are the items in order.
	Items []ListItem

	// Selection is how many items the user can select.
	Selection SelectionMode

	// SelectedIDs are the IDs of the initially selected items.
	SelectedIDs []string
}

// Validate checks item IDs and the initial selection.
func (options ListViewOptions) Validate() error {
	ids := make([]string, len(options.Items))
	for i, item := range options.Items {
		ids[i] = item.ID
	}
	if err := validateIDs("list item", ids); err != nil {
		return err
	}
	return validateSelectedIDs(ids, options.SelectedIDs, options.Selection)
}

// ListView is a native list. OnActivate receives double-click or Return.
type ListView interface {
	Widget
	// Items returns the items.
	Items() []ListItem

	// SetItems replaces the items.
	SetItems(items []ListItem) error

	// SelectedIDs returns the IDs of the selected items.
	SelectedIDs() []string

	// SetSelectedIDs selects the items with ids.
	SetSelectedIDs(ids []string) error

	// ScrollToItem scrolls until the item with id is visible.
	ScrollToItem(id string) error

	// OnSelectionChange runs handler after the user changes the selection.
	OnSelectionChange(handler SelectionIDsHandler) Unsubscribe

	// OnActivate runs handler with the ID of the item that the user activates.
	OnActivate(handler ActionHandler) Unsubscribe
}

// TableColumn is one table column. Width zero uses the content width.
type TableColumn struct {
	// ID identifies the column in TableRow.Values.
	ID string

	// Title is the text of the header.
	Title string

	// Width is the width of the column in logical pixels.
	Width int

	// Sortable lets the user sort by the column.
	Sortable bool
}

// TableRow is one table row. Values are keyed by column ID.
type TableRow struct {
	// ID identifies the row.
	ID string

	// Values maps column IDs to the text in the cells.
	Values map[string]string
}

// TableViewOptions describes a multi-column table.
type TableViewOptions struct {
	// Columns are the columns in order.
	Columns []TableColumn

	// Rows are the rows in order.
	Rows []TableRow
	// Selection is how many rows the user can select.
	Selection SelectionMode

	// SelectedIDs are the IDs of the initially selected rows.
	SelectedIDs []string
}

// Validate checks column and row IDs and the selection.
func (options TableViewOptions) Validate() error {
	if len(options.Columns) == 0 {
		return errors.New("ui: table requires at least one column")
	}
	columns := make([]string, len(options.Columns))
	for i, column := range options.Columns {
		columns[i] = column.ID
	}
	if err := validateIDs("table column", columns); err != nil {
		return err
	}
	rows := make([]string, len(options.Rows))
	for i, row := range options.Rows {
		rows[i] = row.ID
	}
	if err := validateIDs("table row", rows); err != nil {
		return err
	}
	return validateSelectedIDs(rows, options.SelectedIDs, options.Selection)
}

// SortDescriptor describes a table sort.
type SortDescriptor struct {
	// ColumnID is the ID of the sort column. It is empty when the table is not
	// sorted.
	ColumnID string

	// Ascending sorts from the smallest value to the largest.
	Ascending bool
}

// TableView is a native multi-column table. The application sorts rows in
// response to OnSortChange.
type TableView interface {
	Widget

	// Columns returns the columns.
	Columns() []TableColumn

	// Rows returns the rows.
	Rows() []TableRow

	// SetRows replaces the rows. The application sorts them.
	SetRows(rows []TableRow) error

	// SelectedIDs returns the IDs of the selected rows.
	SelectedIDs() []string

	// SetSelectedIDs selects the rows with ids.
	SetSelectedIDs(ids []string) error

	// Sort returns the sort that the user selected.
	Sort() SortDescriptor

	// OnSortChange runs handler after the user selects a sort. The application
	// then sorts the rows.
	OnSortChange(handler func(sort SortDescriptor)) Unsubscribe

	// OnSelectionChange runs handler after the user changes the selection.
	OnSelectionChange(handler SelectionIDsHandler) Unsubscribe

	// OnActivate runs handler with the ID of the row that the user activates.
	OnActivate(handler ActionHandler) Unsubscribe
}

// TreeNode is one tree node. HasChildren shows a disclosure triangle even when
// Children is empty, so children can be loaded on expansion.
type TreeNode struct {
	// ID identifies the node.
	ID string

	// Text is the label of the node.
	Text string

	// Icon is the image of the node.
	Icon IconSource

	// HasChildren shows the node as expandable before its children are known.
	HasChildren bool

	// Children are the nodes below this node.
	Children []TreeNode
}

// TreeViewOptions describes a hierarchical list.
type TreeViewOptions struct {
	// Roots are the top-level nodes.
	Roots []TreeNode

	// Selection is how many nodes the user can select.
	Selection SelectionMode
}

// TreeView is a native outline. OnExpand lets the application supply children
// with SetChildren.
type TreeView interface {
	Widget

	// Roots returns the top-level nodes.
	Roots() []TreeNode

	// SetRoots replaces all nodes.
	SetRoots(roots []TreeNode) error

	// SetChildren replaces the children of the node with id.
	SetChildren(id string, children []TreeNode) error

	// IsExpanded reports whether the node with id shows its children.
	IsExpanded(id string) bool

	// SetExpanded expands or collapses the node with id.
	SetExpanded(id string, expanded bool) error

	// SelectedIDs returns the IDs of the selected nodes.
	SelectedIDs() []string

	// SetSelectedIDs selects the nodes with ids.
	SetSelectedIDs(ids []string) error

	// OnExpand runs handler with the ID of each node that the user expands.
	OnExpand(handler ActionHandler) Unsubscribe

	// OnSelectionChange runs handler after the user changes the selection.
	OnSelectionChange(handler SelectionIDsHandler) Unsubscribe
}

// CollectionItem is one cell in a collection view.
type CollectionItem struct {
	// ID identifies the item.
	ID string

	// Text is the label of the item.
	Text string

	// Image is the image of the item.
	Image stdimage.Image
}

// CollectionViewOptions describes a grid of items. Zero ItemSize uses a
// platform default.
type CollectionViewOptions struct {
	// Items are the items in order.
	Items []CollectionItem

	// ItemSize is the size of each item.
	ItemSize Size

	// Selection is how many items the user can select.
	Selection SelectionMode
}

// CollectionView is a native item grid.
type CollectionView interface {
	Widget

	// Items returns the items.
	Items() []CollectionItem

	// SetItems replaces the items.
	SetItems(items []CollectionItem) error

	// SelectedIDs returns the IDs of the selected items.
	SelectedIDs() []string

	// SetSelectedIDs selects the items with ids.
	SetSelectedIDs(ids []string) error

	// OnSelectionChange runs handler after the user changes the selection.
	OnSelectionChange(handler SelectionIDsHandler) Unsubscribe

	// OnActivate runs handler with the ID of the item that the user activates.
	OnActivate(handler ActionHandler) Unsubscribe
}

// NavigationItem is one sidebar destination.
type NavigationItem struct {
	// ID identifies the destination.
	ID string

	// Text is the label of the destination.
	Text string

	// Icon is the image of the destination.
	Icon IconSource

	// Badge is the badge text. An empty text shows no badge.
	Badge string
}

// NavigationSection groups sidebar destinations.
type NavigationSection struct {
	// ID identifies the section.
	ID string

	// Title is the header of the section.
	Title string

	// Items are the destinations in order.
	Items []NavigationItem
}

// NavigationListOptions describes a source list or sidebar.
type NavigationListOptions struct {
	// Sections are the sections in order.
	Sections []NavigationSection

	// SelectedID is the ID of the initially selected destination.
	SelectedID string
}

// NavigationList is a sidebar of destinations.
type NavigationList interface {
	Widget

	// Sections returns the sections.
	Sections() []NavigationSection

	// SetSections replaces the sections.
	SetSections(sections []NavigationSection) error

	// SelectedID returns the ID of the selected destination.
	SelectedID() string

	// SetSelectedID selects the destination with id.
	SetSelectedID(id string) error

	// OnChange runs handler with the ID of the destination that the user selects.
	OnChange(handler ActionHandler) Unsubscribe
}

// BreadcrumbItem is one path element.
type BreadcrumbItem struct {
	// ID identifies the location.
	ID string

	// Text is the label of the location.
	Text string
}

// BreadcrumbsOptions describes a path control.
type BreadcrumbsOptions struct {
	// Items are the locations from the root to the current location.
	Items []BreadcrumbItem
}

// Breadcrumbs is a native path control. OnActivate receives the clicked ID.
type Breadcrumbs interface {
	Widget

	// Items returns the locations.
	Items() []BreadcrumbItem

	// SetItems replaces the locations.
	SetItems(items []BreadcrumbItem) error

	// OnActivate runs handler with the ID of the location that the user clicks.
	OnActivate(handler ActionHandler) Unsubscribe
}

// PaginationOptions describes a page selector. Pages are zero-based.
type PaginationOptions struct {
	// PageCount is the number of pages.
	PageCount int

	// SelectedPage is the index of the initially selected page.
	SelectedPage int

	// Compact shows a smaller control.
	Compact bool
}

// Validate checks the page count and selection.
func (options PaginationOptions) Validate() error {
	if options.PageCount <= 0 {
		return errors.New("ui: pagination requires at least one page")
	}
	if options.SelectedPage < 0 || options.SelectedPage >= options.PageCount {
		return errors.New("ui: pagination selection is out of range")
	}
	return nil
}

// Pagination selects a page.
type Pagination interface {
	Widget

	// PageCount returns the number of pages.
	PageCount() int

	// SetPageCount changes the number of pages.
	SetPageCount(count int) error

	// SelectedPage returns the index of the selected page.
	SelectedPage() int

	// SetSelectedPage selects the page at index page.
	SetSelectedPage(page int) error

	// OnChange runs handler with the page that the user selects.
	OnChange(handler SelectionChangeHandler) Unsubscribe
}

// PropertyRow is one property with an editor widget.
type PropertyRow struct {
	// ID identifies the row.
	ID string

	// Label is the name of the property.
	Label string

	// Editor is the widget that edits the property.
	Editor Widget
}

// PropertyGroup is a collapsible group of properties.
type PropertyGroup struct {
	// ID identifies the group.
	ID string

	// Title is the header of the group.
	Title string

	// Rows are the properties in order.
	Rows []PropertyRow
}

// PropertyInspectorOptions describes an inspector of grouped properties.
type PropertyInspectorOptions struct {
	// Groups are the groups in order.
	Groups []PropertyGroup
}

// PropertyInspector shows grouped label/editor rows.
type PropertyInspector interface {
	Widget

	// Groups returns the groups.
	Groups() []PropertyGroup

	// SetGroupExpanded expands or collapses the group with id.
	SetGroupExpanded(id string, expanded bool) error
}

// ---- Window chrome ----

// StatusBarOptions describes a window status bar.
type StatusBarOptions struct {
	// Text is the initial status text.
	Text string
}

// StatusBar shows status text along the bottom of a window.
type StatusBar interface {
	Widget

	// Text returns the status text.
	Text() string

	// SetText changes the status text.
	SetText(text string)
}

// TitleBarAccessoryOptions places content in the window title bar.
type TitleBarAccessoryOptions struct {
	// Content is the widget in the title bar.
	Content Widget

	// Alignment puts the content at the start or the end of the title bar.
	Alignment Alignment
}

// TitleBarAccessory is content hosted in the title bar.
type TitleBarAccessory interface {
	// Close removes the accessory and releases its content.
	Close()

	// Content returns the widget in the title bar.
	Content() Widget
}

// ---- Feedback ----

// InfoBarOptions describes an in-content message bar.
type InfoBarOptions struct {
	// Severity selects the icon and color of the bar.
	Severity Severity

	// Title is the main text.
	Title string

	// Detail is the secondary text.
	Detail string

	// Actions are the buttons of the bar.
	Actions []Action

	// Closable shows a close button.
	Closable bool

	// Live makes assistive technologies announce the message.
	Live LiveRegion
}

// InfoBar is an in-content message bar.
type InfoBar interface {
	Widget

	// Severity returns the severity of the message.
	Severity() Severity

	// SetSeverity changes the severity of the message.
	SetSeverity(severity Severity)

	// Title returns the main text.
	Title() string

	// SetTitle changes the main text.
	SetTitle(title string)

	// Detail returns the secondary text.
	Detail() string

	// SetDetail changes the secondary text.
	SetDetail(detail string)

	// OnAction runs handler with the ID of the action that the user activates.
	OnAction(handler ActionHandler) Unsubscribe

	// OnClose runs handler when the user clicks the close button.
	OnClose(handler FocusHandler) Unsubscribe
}

// TooltipOptions describes a standalone tooltip shown on demand.
type TooltipOptions struct {
	// Text is the help text.
	Text string

	// Delay is the time before the tooltip shows.
	Delay time.Duration

	// Placement is the side of the anchor where the tooltip shows.
	Placement Edge
}

// Tooltip is a tooltip that the application shows next to a widget.
type Tooltip interface {
	// Close releases the tooltip.
	Close()

	// Text returns the help text.
	Text() string

	// SetText changes the help text.
	SetText(text string)

	// Show shows the tooltip at anchor after the delay. The anchor must be in a
	// visible window.
	Show(anchor Widget) error

	// Hide hides the tooltip.
	Hide()

	// IsShown reports whether the tooltip shows.
	IsShown() bool
}

// ToastOptions describes a transient message. Zero Timeout uses a platform
// default.
type ToastOptions struct {
	// Message is the text of the toast.
	Message string

	// Actions are the buttons of the toast.
	Actions []Action

	// Timeout is the time before the toast hides.
	Timeout time.Duration
}

// Toast is a transient message shown over a window.
type Toast interface {
	// Close releases the toast.
	Close()

	// Show shows the toast over its window.
	Show() error

	// Dismiss hides the toast before the timeout.
	Dismiss()

	// IsShown reports whether the toast shows.
	IsShown() bool

	// OnAction runs handler with the ID of the action that the user activates.
	OnAction(handler ActionHandler) Unsubscribe
}

// BadgeOptions describes a count or text badge. Counts above a positive
// Maximum display as "Maximum+".
type BadgeOptions struct {
	// Count is the number that the badge shows.
	Count int

	// Maximum is the largest number that the badge shows exactly.
	Maximum int

	// Text is the text that the badge shows in place of a count.
	Text string

	// Severity selects the color of the badge.
	Severity Severity

	// AccessibleValue is the text that assistive technologies read.
	AccessibleValue string
}

// Badge displays a small count or label.
type Badge interface {
	Widget

	// Count returns the number of the badge.
	Count() int

	// SetCount changes the number of the badge.
	SetCount(count int)

	// Text returns the text of the badge.
	Text() string

	// SetText changes the text of the badge.
	SetText(text string)

	// DisplayText returns the text that the badge shows, for example "99+".
	DisplayText() string
}

// ProgressDialogOptions describes a modal progress sheet. Value is in [0, 1];
// a negative Value is indeterminate.
type ProgressDialogOptions struct {
	// Title is the title of the dialog.
	Title string

	// Message is the text that describes the operation.
	Message string

	// Value is the initial progress.
	Value float64

	// Cancellable shows a cancel button.
	Cancellable bool
}

// ProgressDialog is a window-modal progress sheet.
type ProgressDialog interface {
	// Close closes the dialog and releases it.
	Close()

	// Show shows the dialog over its window.
	Show() error

	// Value returns the progress.
	Value() float64

	// SetValue changes the progress.
	SetValue(value float64)

	// Message returns the text that describes the operation.
	Message() string

	// SetMessage changes the text that describes the operation.
	SetMessage(message string)

	// OnCancel runs handler when the user clicks the cancel button.
	OnCancel(handler FocusHandler) Unsubscribe
}

// EmptyStateOptions describes a placeholder for empty content.
type EmptyStateOptions struct {
	// Icon is the image above the title.
	Icon IconSource

	// Title is the main text.
	Title string

	// Detail is the secondary text.
	Detail string

	// Actions are the buttons below the text.
	Actions []Action
}

// EmptyState is a placeholder shown in place of empty content.
type EmptyState interface {
	Widget

	// Title returns the main text.
	Title() string

	// SetTitle changes the main text.
	SetTitle(title string)

	// Detail returns the secondary text.
	Detail() string

	// SetDetail changes the secondary text.
	SetDetail(detail string)

	// OnAction runs handler with the ID of the action that the user activates.
	OnAction(handler ActionHandler) Unsubscribe
}

// InlineValidationOptions describes a field-level message.
type InlineValidationOptions struct {
	// Severity selects the icon and color of the message.
	Severity Severity

	// Text is the message.
	Text string
}

// InlineValidation shows a message next to a form field. Link it to the field
// with Accessibility.DescribedBy.
type InlineValidation interface {
	Widget

	// Severity returns the severity of the message.
	Severity() Severity

	// SetSeverity changes the severity of the message.
	SetSeverity(severity Severity)

	// Text returns the message.
	Text() string

	// SetText changes the message.
	SetText(text string)
}

// ---- Content ----

// IconOptions describes a symbol or image icon.
type IconOptions struct {
	// SystemName is the backend-neutral name of a symbol.
	SystemName string

	// Image is the image to show when there is no symbol.
	Image stdimage.Image

	// Size is the size of the icon.
	Size Size

	// Tint is the color of a symbol.
	Tint SemanticColor

	// AccessibilityName is the text that assistive technologies read. An empty
	// name hides the icon from them.
	AccessibilityName string
}

// Validate checks that the icon has a source and a usable size.
func (options IconOptions) Validate() error {
	if options.SystemName == "" && options.Image == nil {
		return errors.New("ui: icon requires a system name or image")
	}
	if options.Size.Width < 0 || options.Size.Height < 0 {
		return errors.New("ui: icon size must be non-negative")
	}
	return nil
}

// Icon displays a symbol or image icon.
type Icon interface {
	Widget

	// Source returns the symbol or image.
	Source() IconSource

	// SetSource changes the symbol or image.
	SetSource(source IconSource) error
}

// RichTextOptions describes a styled text view. Text is plain text; RTF, when
// set, takes precedence.
type RichTextOptions struct {
	// Text is the initial plain text.
	Text string

	// RTF is the initial text in Rich Text Format.
	RTF []byte

	// Editable lets the user edit the text.
	Editable bool
}

// RichText is a native styled text view.
type RichText interface {
	Widget

	// Text returns the text without styles.
	Text() string

	// SetText replaces the content with plain text.
	SetText(text string)

	// RTF returns the content in Rich Text Format.
	RTF() ([]byte, error)

	// SetRTF replaces the content with text in Rich Text Format.
	SetRTF(rtf []byte) error

	// IsEditable reports whether the user can edit the text.
	IsEditable() bool

	// SetEditable lets the user edit the text, or stops the user.
	SetEditable(editable bool)

	// OnChange runs handler after each edit by the user.
	OnChange(handler FocusHandler) Unsubscribe
}

// MarkdownOptions describes a read-only Markdown view.
type MarkdownOptions struct {
	// Source is the CommonMark text.
	Source string
}

// Markdown renders CommonMark text with native text views.
type Markdown interface {
	Widget

	// Source returns the CommonMark text.
	Source() string

	// SetSource replaces the CommonMark text.
	SetSource(source string) error

	// OnLinkActivate runs handler with the URL of each link that the user clicks.
	OnLinkActivate(handler func(url string)) Unsubscribe
}

// SVGOptions describes a vector image.
type SVGOptions struct {
	// Source is the SVG document.
	Source []byte

	// AccessibilityText is the description of the image for assistive
	// technologies.
	AccessibilityText string
}

// SVG displays an SVG image.
type SVG interface {
	Widget

	// SetSource replaces the SVG document. It returns an error when the document
	// is not valid.
	SetSource(source []byte) error

	// IntrinsicSize returns the size that the document declares.
	IntrinsicSize() Size
}

// AnimationFrame is one animated-image frame.
type AnimationFrame struct {
	// Image is the image of the frame.
	Image stdimage.Image

	// Duration is the time that the frame shows.
	Duration time.Duration
}

// AnimatedImageOptions describes a frame animation. Playback pauses when the
// user prefers reduced motion.
type AnimatedImageOptions struct {
	// Frames are the frames in order.
	Frames []AnimationFrame

	// Repeat starts again after the last frame.
	Repeat bool
}

// Validate checks that there are frames with images and positive durations.
func (options AnimatedImageOptions) Validate() error {
	if len(options.Frames) == 0 {
		return errors.New("ui: animated image requires at least one frame")
	}
	for i, frame := range options.Frames {
		if frame.Image == nil || frame.Image.Bounds().Empty() {
			return fmt.Errorf("ui: animation frame %d has no image", i)
		}
		if frame.Duration <= 0 {
			return fmt.Errorf("ui: animation frame %d has no duration", i)
		}
	}
	return nil
}

// AnimatedImage plays frames.
type AnimatedImage interface {
	Widget

	// Play starts the animation.
	Play()

	// Pause stops the animation at the current frame.
	Pause()

	// IsPlaying reports whether the animation runs.
	IsPlaying() bool
}

// VideoOptions describes a video player. Source is a file path or URL.
type VideoOptions struct {
	// Source is the video to play.
	Source string

	// ShowControls shows the playback controls of the platform.
	ShowControls bool

	// Autoplay starts the playback when the video loads.
	Autoplay bool

	// Muted plays the video without sound.
	Muted bool
}

// Video plays media with native playback.
type Video interface {
	Widget

	// Play starts the playback.
	Play()

	// Pause stops the playback at the current position.
	Pause()

	// IsPlaying reports whether the video plays.
	IsPlaying() bool

	// Position returns the playback position.
	Position() time.Duration

	// Seek moves the playback to position.
	Seek(position time.Duration) error

	// Duration returns the length of the video. It is zero until the video loads.
	Duration() time.Duration
}

// WebViewOptions describes embedded web content. HTML, when set, is loaded
// instead of URL.
type WebViewOptions struct {
	// URL is the page to load.
	URL string

	// HTML is the HTML document to show.
	HTML string

	// AllowScripts lets the page run JavaScript.
	AllowScripts bool
}

// WebView hosts native web content.
type WebView interface {
	Widget

	// URL returns the URL of the current page.
	URL() string

	// Load loads the page at url.
	Load(url string) error

	// LoadHTML shows the HTML document.
	LoadHTML(html string) error

	// OnNavigate runs handler with the URL of each page that the view loads.
	OnNavigate(handler func(url string)) Unsubscribe
}
