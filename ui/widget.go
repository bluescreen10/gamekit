package ui

import (
	"errors"
	"fmt"
	stdimage "image"
	"math"
	"time"
)

// ClickHandler receives a native button activation.
type ClickHandler func()

// TextChangeHandler receives the current text after a text field changes.
type TextChangeHandler func(text string)

// ValueChangeHandler receives the current value after a value control changes.
type ValueChangeHandler func(value float64)

// ResizeHandler receives a drawable's new size in logical pixels.
type ResizeHandler func(size Size)

// MoveHandler receives a window's new position in logical pixels.
type MoveHandler func(position Point)

// ExpansionChangeHandler receives a section's expanded state after it changes.
type ExpansionChangeHandler func(expanded bool)

// ToggleHandler receives the current state after a toggle control changes.
type ToggleHandler func(checked bool)

// SelectionChangeHandler receives the selected item index after it changes.
type SelectionChangeHandler func(index int)

// FocusHandler receives a focus or activation change.
type FocusHandler func()

// DropHandler receives data dropped onto a widget.
type DropHandler func(event DropEvent)

// Widget is a native control owned by a window. Bounds use logical pixels in
// the coordinate system of the widget's parent.
//
// MinimumSize, MaximumSize, and Margins are layout hints that containers
// respect; a zero MaximumSize means unbounded. OnResize handlers run after
// SetBounds or a parent layout changes the widget's size.
type Widget interface {
	// Close releases the widget and the widgets in it. After Close, the methods
	// that return an error return ErrClosed.
	Close()

	// Bounds returns the position and size of the widget in its parent.
	Bounds() Rect

	// SetBounds moves and resizes the widget. A container calls it for its
	// children.
	SetBounds(bounds Rect)

	// PreferredSize returns the size that shows the content of the widget without
	// clipping.
	PreferredSize() Size

	// IsVisible reports whether the widget is shown.
	IsVisible() bool

	// SetVisible shows or hides the widget.
	SetVisible(visible bool)

	// IsEnabled reports whether the user can interact with the widget.
	IsEnabled() bool

	// SetEnabled lets the user interact with the widget, or stops the user.
	SetEnabled(enabled bool)

	// Accessibility returns the metadata that the widget gives to assistive
	// technologies.
	Accessibility() Accessibility

	// SetAccessibility validates metadata and gives it to assistive technologies.
	// It returns an error that wraps ErrNotSupported when the platform cannot show
	// a part of metadata.
	SetAccessibility(metadata Accessibility) error

	// AccessibilityRole returns the semantic role of the widget.
	AccessibilityRole() AccessibilityRole

	// Focus gives the keyboard focus to the widget. It returns an error when the
	// widget is hidden, is disabled, or cannot take focus.
	Focus() error

	// IsFocused reports whether the widget has the keyboard focus.
	IsFocused() bool

	// OnFocus runs handler when the widget gets the keyboard focus.
	OnFocus(handler FocusHandler) Unsubscribe

	// OnBlur runs handler when the widget loses the keyboard focus.
	OnBlur(handler FocusHandler) Unsubscribe

	// MinimumSize returns the smallest size that containers give the widget.
	MinimumSize() Size

	// SetMinimumSize changes the smallest size that containers give the widget.
	SetMinimumSize(size Size)

	// MaximumSize returns the largest size that containers give the widget. A zero
	// dimension has no limit.
	MaximumSize() Size

	// SetMaximumSize changes the largest size that containers give the widget.
	SetMaximumSize(size Size)

	// Margins returns the space that containers keep around the widget.
	Margins() Insets

	// SetMargins changes the space that containers keep around the widget.
	SetMargins(margins Insets)

	// OnResize runs handler after the size of the widget changes.
	OnResize(handler ResizeHandler) Unsubscribe

	// Tooltip returns the help text that shows when the pointer stays over the
	// widget.
	Tooltip() string

	// SetTooltip changes the help text. An empty text removes the tooltip.
	SetTooltip(text string)

	// Cursor returns the pointer shape over the widget.
	Cursor() Cursor

	// SetCursor changes the pointer shape over the widget.
	SetCursor(cursor Cursor) error

	// Style returns the font and colors of the widget.
	Style() WidgetStyle

	// SetStyle changes the font and colors of the widget.
	SetStyle(style WidgetStyle) error

	// SetContextMenu sets the menu that a secondary click opens. A nil menu
	// removes it.
	SetContextMenu(menu ContextMenu) error

	// SetDragSource lets the user drag data from the widget.
	SetDragSource(source DragSource) error

	// SetDropTarget lets the user drop data on the widget.
	SetDropTarget(target DropTarget) error

	// OnDrop runs handler for each drop on the widget.
	OnDrop(handler DropHandler) Unsubscribe
}

// DragOperation is a set of permitted drag-and-drop operations.
type DragOperation uint8

const (
	// DragCopy copies the data.
	DragCopy DragOperation = 1 << iota
	// DragMove moves the data.
	DragMove
	// DragLink creates a link to the data.
	DragLink

	// DragNone permits no operation.
	DragNone DragOperation = 0
)

// DragSource makes a widget start a native drag with the given data. A zero
// Operations value disables dragging.
type DragSource struct {
	// Operations are the operations that the destination can do.
	Operations DragOperation
	// Text is the text to drag.
	Text string
	// Files are the paths of the files to drag.
	Files []string
	// Image is the image to drag.
	Image stdimage.Image
	// Custom maps MIME types to data in that type.
	Custom map[string][]byte
}

// DropTarget makes a widget accept native drops. Formats are MIME types such
// as "text/plain", "text/uri-list", "image/png", or custom types. A zero
// Operations value disables dropping.
type DropTarget struct {
	// Operations are the operations that the widget accepts.
	Operations DragOperation
	// Formats are the MIME types that the widget accepts.
	Formats []string
}

// DropEvent is data dropped onto a widget. Position is in widget coordinates.
type DropEvent struct {
	// Operation is the operation that the user chose.
	Operation DragOperation
	// Position is the drop position.
	Position Point
	// Text is the dropped text.
	Text string
	// Files are the paths of the dropped files.
	Files []string
	// Image is the dropped image.
	Image stdimage.Image
	// Custom maps MIME types to the dropped data in that type.
	Custom map[string][]byte
}

// Layout is a native container whose LayoutSpec arranges its children. A child
// can belong to only one layout at a time.
type Layout interface {
	Widget
	// Spec returns the arrangement of the layout.
	Spec() LayoutSpec
	// SetSpec replaces the arrangement and the children. Children that are not in
	// spec leave the layout and stay open.
	SetSpec(spec LayoutSpec) error
	// Children returns the widgets in the layout in order.
	Children() []Widget
}

// Label displays non-editable text.
type Label interface {
	Widget
	// Text returns the text of the label.
	Text() string
	// SetText changes the text of the label.
	SetText(text string)
	// TextAlignment returns the horizontal alignment of the text.
	TextAlignment() TextAlignment
	// SetTextAlignment changes the horizontal alignment of the text.
	SetTextAlignment(alignment TextAlignment)
	// WrapMode returns how the text wraps.
	WrapMode() WrapMode
	// SetWrapMode changes how the text wraps.
	SetWrapMode(mode WrapMode)
	// Truncation returns where the label shortens text that does not fit.
	Truncation() Truncation
	// SetTruncation changes where the label shortens text that does not fit.
	SetTruncation(truncation Truncation)
	// IsSelectable reports whether the user can select and copy the text.
	IsSelectable() bool
	// SetSelectable lets the user select and copy the text, or stops the user.
	SetSelectable(selectable bool)
	// SemanticStyle returns the text style of the label.
	SemanticStyle() TextStyle
	// SetSemanticStyle changes the text style of the label.
	SetSemanticStyle(style TextStyle)
	// MaximumLines limits wrapped text; zero means unlimited.
	MaximumLines() int
	// SetMaximumLines changes the line limit.
	SetMaximumLines(lines int)
}

// ButtonRole describes a button's purpose, which can change its native style
// and keyboard behavior.
type ButtonRole uint8

const (
	// ButtonNormal is a button without a special purpose.
	ButtonNormal ButtonRole = iota
	// ButtonDefault is activated by Return.
	ButtonDefault
	// ButtonCancel is activated by Escape.
	ButtonCancel
	// ButtonDestructive is a button whose action deletes data or cannot be
	// undone. The platform can show it in a warning style.
	ButtonDestructive
)

// Button is a native push button.
type Button interface {
	Widget
	// Text returns the text of the button.
	Text() string
	// SetText changes the text of the button.
	SetText(text string)
	// OnClick runs handler when the user activates the button.
	OnClick(handler ClickHandler) Unsubscribe
	// Icon returns the image of the button and its placement.
	Icon() (IconSource, IconPlacement)
	// SetIcon changes the image of the button and its placement.
	SetIcon(icon IconSource, placement IconPlacement)
	// Role returns the purpose of the button.
	Role() ButtonRole
	// SetRole changes the purpose of the button.
	SetRole(role ButtonRole)
	// Shortcut returns the keyboard shortcut that activates the button.
	Shortcut() Shortcut
	// SetShortcut changes the keyboard shortcut that activates the button.
	SetShortcut(shortcut Shortcut)
	// IsBusy reports a pending operation; a busy button shows progress and
	// ignores activation.
	IsBusy() bool
	// SetBusy starts or stops the busy state.
	SetBusy(busy bool)
	// Badge returns the text of the small badge on the button.
	Badge() string
	// SetBadge changes the text of the badge. An empty text removes the badge.
	SetBadge(badge string)
}

// InputPurpose hints at the expected content of a text field so the platform
// can pick a keyboard, autofill, and spelling behavior.
type InputPurpose uint8

const (
	// InputPurposeText is general text.
	InputPurposeText InputPurpose = iota
	// InputPurposeName is the name of a person.
	InputPurposeName
	// InputPurposeUsername is the name of an account.
	InputPurposeUsername
	// InputPurposeEmail is an email address.
	InputPurposeEmail
	// InputPurposeURL is a web address.
	InputPurposeURL
	// InputPurposePhone is a telephone number.
	InputPurposePhone
	// InputPurposeNumber is a number.
	InputPurposeNumber
)

// TabPolicy controls whether Tab inserts a tab character or moves focus.
type TabPolicy uint8

const (
	// TabMovesFocus moves the focus to the next control.
	TabMovesFocus TabPolicy = iota
	// TabInsert inserts a tab character.
	TabInsert
)

// TextField is a native single-line text editor. OnChange handlers receive user
// edits; SetText does not invoke them.
type TextField interface {
	Widget
	// Text returns the text in the field.
	Text() string
	// SetText changes the text in the field.
	SetText(text string)
	// Placeholder returns the text that the empty field shows.
	Placeholder() string
	// SetPlaceholder changes the text that the empty field shows.
	SetPlaceholder(placeholder string)
	// OnChange runs handler after each edit by the user.
	OnChange(handler TextChangeHandler) Unsubscribe
	// OnSubmit receives the text when the user presses Return.
	OnSubmit(handler TextChangeHandler) Unsubscribe
	// IsReadOnly reports whether the user cannot edit the text.
	IsReadOnly() bool
	// SetReadOnly stops the user from editing the text, or lets the user edit it.
	SetReadOnly(readOnly bool)
	// Selection returns the selected text range.
	Selection() TextRange
	// SetSelection selects a text range.
	SetSelection(selection TextRange)
	// MaximumLength limits the text in runes; zero means unlimited.
	MaximumLength() int
	// SetMaximumLength changes the length limit.
	SetMaximumLength(length int)
	// InputPurpose returns the expected content of the field.
	InputPurpose() InputPurpose
	// SetInputPurpose changes the expected content of the field.
	SetInputPurpose(purpose InputPurpose)
	// TextAlignment returns the horizontal alignment of the text.
	TextAlignment() TextAlignment
	// SetTextAlignment changes the horizontal alignment of the text.
	SetTextAlignment(alignment TextAlignment)
	// Prefix returns the fixed text before the editable text.
	Prefix() string
	// SetPrefix changes the fixed text before the editable text.
	SetPrefix(prefix string)
	// Suffix returns the fixed text after the editable text.
	Suffix() string
	// SetSuffix changes the fixed text after the editable text.
	SetSuffix(suffix string)
	// IsSpellCheckEnabled reports whether the field marks spelling mistakes.
	IsSpellCheckEnabled() bool
	// SetSpellCheckEnabled starts or stops the spelling check.
	SetSpellCheckEnabled(enabled bool)
}

// TextAreaOptions describes a native multiline text editor.
type TextAreaOptions struct {
	// Text is the initial text.
	Text string
	// Placeholder is the text that the empty area shows.
	Placeholder string
	// ReadOnly stops the user from editing the text.
	ReadOnly bool
	// Wrap wraps lines at word boundaries.
	Wrap bool
}

// TextArea is a native multiline text editor. OnChange handlers receive user
// edits; SetText does not invoke them.
type TextArea interface {
	Widget
	// Text returns the text in the area.
	Text() string
	// SetText changes the text in the area.
	SetText(text string)
	// Placeholder returns the text that the empty area shows.
	Placeholder() string
	// SetPlaceholder changes the text that the empty area shows.
	SetPlaceholder(placeholder string)
	// IsReadOnly reports whether the user cannot edit the text.
	IsReadOnly() bool
	// SetReadOnly stops the user from editing the text, or lets the user edit it.
	SetReadOnly(readOnly bool)
	// OnChange runs handler after each edit by the user.
	OnChange(handler TextChangeHandler) Unsubscribe
	// WrapMode returns how the lines wrap.
	WrapMode() WrapMode
	// SetWrapMode changes how the lines wrap.
	SetWrapMode(mode WrapMode)
	// Selection returns the selected text range.
	Selection() TextRange
	// SetSelection selects a text range.
	SetSelection(selection TextRange)
	// ScrollPosition returns the position of the visible text in logical pixels.
	ScrollPosition() Point
	// SetScrollPosition scrolls the text to position.
	SetScrollPosition(position Point)
	// MaximumLength returns the limit of the text in runes. Zero means no limit.
	MaximumLength() int
	// SetMaximumLength changes the length limit.
	SetMaximumLength(length int)
	// TabPolicy returns what the Tab key does.
	TabPolicy() TabPolicy
	// SetTabPolicy changes what the Tab key does.
	SetTabPolicy(policy TabPolicy)
	// IsSpellCheckEnabled reports whether the area marks spelling mistakes.
	IsSpellCheckEnabled() bool
	// SetSpellCheckEnabled starts or stops the spelling check.
	SetSpellCheckEnabled(enabled bool)
}

// CheckState is the state of a check box that may allow a mixed state.
type CheckState uint8

const (
	// CheckOff is not checked.
	CheckOff CheckState = iota
	// CheckOn is checked.
	CheckOn
	// CheckMixed shows that the check box controls items with different states.
	CheckMixed
)

// LabelPlacement places a control's label relative to the control.
type LabelPlacement uint8

const (
	// LabelAfterControl puts the label after the control.
	LabelAfterControl LabelPlacement = iota
	// LabelBeforeControl puts the label before the control.
	LabelBeforeControl
)

// Choice is one selectable option with a stable ID.
type Choice struct {
	// ID identifies the choice.
	ID string
	// Text is the label of the choice.
	Text string
	// Description is secondary text for the choice.
	Description string
	// Enabled lets the user select the choice.
	Enabled bool
}

func validateChoices(choices []Choice) error {
	if len(choices) == 0 {
		return errors.New("ui: at least one choice is required")
	}
	seen := make(map[string]struct{}, len(choices))
	for i, choice := range choices {
		if choice.ID == "" {
			return fmt.Errorf("ui: choice %d has no ID", i)
		}
		if _, exists := seen[choice.ID]; exists {
			return fmt.Errorf("ui: duplicate choice ID %q", choice.ID)
		}
		seen[choice.ID] = struct{}{}
	}
	return nil
}

// ValidateChoices checks that choices are present and have unique IDs.
func ValidateChoices(choices []Choice) error {
	return validateChoices(choices)
}

// CheckBox is a native independently checked control.
type CheckBox interface {
	Widget
	// Text returns the label of the check box.
	Text() string
	// SetText changes the label of the check box.
	SetText(text string)
	// IsChecked reports whether the check box is on.
	IsChecked() bool
	// SetChecked turns the check box on or off.
	SetChecked(checked bool)
	// OnChange runs handler after the user changes the check box.
	OnChange(handler ToggleHandler) Unsubscribe
	// State reports CheckMixed only when AllowsMixedState is true.
	State() CheckState
	// SetState changes the state. CheckMixed requires AllowsMixedState.
	SetState(state CheckState)
	// AllowsMixedState reports whether the check box can show CheckMixed.
	AllowsMixedState() bool
	// SetAllowsMixedState lets the check box show CheckMixed, or stops it.
	SetAllowsMixedState(allow bool)
	// LabelPlacement returns the side of the label.
	LabelPlacement() LabelPlacement
	// SetLabelPlacement changes the side of the label.
	SetLabelPlacement(placement LabelPlacement)
	// Description returns the secondary text below the label.
	Description() string
	// SetDescription changes the secondary text below the label.
	SetDescription(description string)
}

// RadioGroupOptions describes a native group of mutually exclusive choices.
type RadioGroupOptions struct {
	// Items are the labels of the choices.
	Items []string
	// SelectedIndex is the index of the initially selected choice.
	SelectedIndex int
}

// Validate checks that the group has choices and a valid selection.
func (options RadioGroupOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: radio group requires at least one item")
	}
	if options.SelectedIndex < 0 || options.SelectedIndex >= len(options.Items) {
		return errors.New("ui: radio group selection is out of range")
	}
	return nil
}

// RadioGroup is a native group of mutually exclusive choices.
type RadioGroup interface {
	Widget
	// Items returns the labels of the choices.
	Items() []string
	// SelectedIndex returns the index of the selected choice.
	SelectedIndex() int
	// SetSelectedIndex selects the choice at index.
	SetSelectedIndex(index int) error
	// OnChange runs handler after the user selects a choice.
	OnChange(handler SelectionChangeHandler) Unsubscribe
	// Orientation returns the direction of the choices.
	Orientation() LayoutDirection
	// SetOrientation changes the direction of the choices.
	SetOrientation(orientation LayoutDirection)
	// Label returns the title of the group.
	Label() string
	// SetLabel changes the title of the group.
	SetLabel(label string)
	// Choices replace Items; SetChoices keeps the selection by ID when possible.
	Choices() []Choice
	// SetChoices replaces the choices.
	SetChoices(choices []Choice) error
}

// SelectOptions describes a native drop-down selection control.
type SelectOptions struct {
	// Items are the labels of the choices.
	Items []string
	// SelectedIndex is the index of the initially selected choice.
	SelectedIndex int
}

// Validate checks that the drop-down has choices and a valid selection.
func (options SelectOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: select requires at least one item")
	}
	if options.SelectedIndex < 0 || options.SelectedIndex >= len(options.Items) {
		return errors.New("ui: select selection is out of range")
	}
	return nil
}

// Select is a native drop-down selection control.
type Select interface {
	Widget
	// Items returns the labels of the choices.
	Items() []string
	// SelectedIndex returns the index of the selected choice.
	SelectedIndex() int
	// SetSelectedIndex selects the choice at index.
	SetSelectedIndex(index int) error
	// OnChange runs handler after the user selects a choice.
	OnChange(handler SelectionChangeHandler) Unsubscribe
	// Placeholder returns the text that shows when there is no selection.
	Placeholder() string
	// SetPlaceholder changes the text that shows when there is no selection.
	SetPlaceholder(placeholder string)
	// Choices returns the choices.
	Choices() []Choice
	// SetChoices replaces the choices. It keeps the selection by ID when possible.
	SetChoices(choices []Choice) error
	// PopupWidth is the minimum pop-up width; zero matches the control.
	PopupWidth() int
	// SetPopupWidth changes the minimum pop-up width.
	SetPopupWidth(width int)
	// VisibleRowCount returns the number of rows that the pop-up shows before it
	// scrolls. Zero lets the platform decide.
	VisibleRowCount() int
	// SetVisibleRowCount changes the number of visible rows.
	SetVisibleRowCount(rows int)
}

// SliderOptions describes a native slider. Minimum must be less than Maximum,
// and Value must fall within that range.
type SliderOptions struct {
	// Minimum is the smallest value.
	Minimum float64
	// Maximum is the largest value.
	Maximum float64
	// Value is the initial value.
	Value float64
	// Step is the distance between the values that the user can select. Zero
	// allows all values.
	Step float64
}

// Validate checks that the slider range and initial value are usable.
func (options SliderOptions) Validate() error {
	if math.IsNaN(options.Minimum) || math.IsInf(options.Minimum, 0) ||
		math.IsNaN(options.Maximum) || math.IsInf(options.Maximum, 0) {
		return errors.New("ui: slider bounds must be finite")
	}
	if options.Minimum >= options.Maximum {
		return errors.New("ui: slider minimum must be less than maximum")
	}
	if math.IsNaN(options.Value) || math.IsInf(options.Value, 0) ||
		options.Value < options.Minimum || options.Value > options.Maximum {
		return errors.New("ui: slider value must be within its range")
	}
	if math.IsNaN(options.Step) || math.IsInf(options.Step, 0) || options.Step < 0 {
		return errors.New("ui: slider step must be finite and non-negative")
	}
	return nil
}

// Slider is a native scalar-value control. OnChange handlers receive user
// changes; SetValue does not invoke them.
type Slider interface {
	Widget
	// Value returns the current value.
	Value() float64
	// SetValue rejects values outside the slider's range.
	SetValue(value float64) error
	// OnChange runs handler after the user changes the value.
	OnChange(handler ValueChangeHandler) Unsubscribe
	// Minimum returns the smallest value.
	Minimum() float64
	// Maximum returns the largest value.
	Maximum() float64
	// Step returns the distance between the values that the user can select.
	Step() float64
	// Orientation returns the direction of the slider.
	Orientation() LayoutDirection
	// SetOrientation changes the direction of the slider.
	SetOrientation(orientation LayoutDirection)
	// IsInverted reports whether the maximum is at the start of the slider.
	IsInverted() bool
	// SetInverted puts the maximum at the start of the slider, or at the end.
	SetInverted(inverted bool)
	// Ticks returns the tick marks.
	Ticks() SliderTicks
	// SetTicks changes the tick marks.
	SetTicks(ticks SliderTicks)
}

// SliderTicks describes slider tick marks. Frequency is the value distance
// between ticks; zero hides them.
type SliderTicks struct {
	// Frequency is the value distance between ticks.
	Frequency float64
	// ShowLabels shows the value at each tick.
	ShowLabels bool
	// SnapToTicks limits the value to the ticks.
	SnapToTicks bool
}

// ProgressMode selects determinate or indeterminate progress.
type ProgressMode uint8

const (
	// ProgressDeterminate shows a known fraction.
	ProgressDeterminate ProgressMode = iota
	// ProgressIndeterminate shows activity without a known fraction.
	ProgressIndeterminate
)

// ProgressState colors a progress bar to show its condition.
type ProgressState uint8

const (
	// ProgressNormal is progress without a problem.
	ProgressNormal ProgressState = iota
	// ProgressPaused is progress that stopped for a time.
	ProgressPaused
	// ProgressWarning is progress with a possible problem.
	ProgressWarning
	// ProgressError is progress that failed.
	ProgressError
)

// ProgressBar displays completion in the inclusive range zero through one.
type ProgressBar interface {
	Widget
	// Value returns the completed fraction.
	Value() float64
	// SetValue changes the completed fraction.
	SetValue(value float64)
	// Mode returns whether the bar shows a known fraction.
	Mode() ProgressMode
	// SetMode changes whether the bar shows a known fraction.
	SetMode(mode ProgressMode)
	// Orientation returns the direction of the bar.
	Orientation() LayoutDirection
	// SetOrientation changes the direction of the bar.
	SetOrientation(orientation LayoutDirection)
	// Text returns the text that describes the progress.
	Text() string
	// SetText changes the text that describes the progress.
	SetText(text string)
	// State returns the condition that the color of the bar shows.
	State() ProgressState
	// SetState changes the condition that the color of the bar shows.
	SetState(state ProgressState)
}

// ActivityIndicator displays indeterminate ongoing activity.
type ActivityIndicator interface {
	Widget
	// IsRunning reports whether the indicator moves.
	IsRunning() bool
	// SetRunning starts or stops the indicator.
	SetRunning(running bool)
	// ControlSize returns the size of the indicator.
	ControlSize() ControlSize
	// SetControlSize changes the size of the indicator.
	SetControlSize(size ControlSize)
	// Label names the activity for assistive technologies and is shown when
	// the platform style has room for it.
	Label() string
	// SetLabel changes the name of the activity.
	SetLabel(label string)
}

// SeparatorPresentation controls how far a separator extends.
type SeparatorPresentation uint8

const (
	// SeparatorFull extends the separator across its bounds.
	SeparatorFull SeparatorPresentation = iota
	// SeparatorInset leaves space at both ends of the separator.
	SeparatorInset
)

// Separator is a native horizontal or vertical visual divider.
type Separator interface {
	Widget
	// Direction returns the direction of the line.
	Direction() LayoutDirection
	// SetDirection changes the direction of the line.
	SetDirection(direction LayoutDirection)
	// Label returns the text in the middle of the line.
	Label() string
	// SetLabel changes the text in the middle of the line.
	SetLabel(label string)
	// Presentation returns how far the line extends.
	Presentation() SeparatorPresentation
	// SetPresentation changes how far the line extends.
	SetPresentation(presentation SeparatorPresentation)
	// Thickness returns the width of the line in logical pixels.
	Thickness() int
	// SetThickness changes the width of the line.
	SetThickness(thickness int)
}

// ImageScaling controls how an image is fitted into its widget bounds.
type ImageScaling uint8

const (
	// ImageScaleFit preserves aspect ratio and shows the entire image.
	ImageScaleFit ImageScaling = iota
	// ImageScaleFill preserves aspect ratio and fills the bounds, cropping if needed.
	ImageScaleFill
	// ImageScaleStretch fills the bounds without preserving aspect ratio.
	ImageScaleStretch
)

// ImageOptions describes a native image view.
type ImageOptions struct {
	// Source is the image to show.
	Source stdimage.Image
	// Scaling is how the image fits the bounds.
	Scaling ImageScaling
}

// Validate checks that the image and scaling mode are usable.
func (options ImageOptions) Validate() error {
	if options.Source == nil {
		return errors.New("ui: image source is required")
	}
	if options.Source.Bounds().Empty() {
		return errors.New("ui: image source is empty")
	}
	if options.Scaling > ImageScaleStretch {
		return fmt.Errorf("ui: invalid image scaling %d", options.Scaling)
	}
	return nil
}

// ImageView displays an image using native drawing and scaling.
type ImageView interface {
	Widget
	// SetImage replaces the image.
	SetImage(source stdimage.Image) error
	// Scaling returns how the image fits the bounds.
	Scaling() ImageScaling
	// SetScaling changes how the image fits the bounds.
	SetScaling(scaling ImageScaling) error
	// Alignment returns the position of the image when it does not fill the
	// bounds.
	Alignment() Anchor
	// SetAlignment changes the position of the image.
	SetAlignment(alignment Anchor)
	// Interpolation returns how the view resamples the image.
	Interpolation() Interpolation
	// SetInterpolation changes how the view resamples the image.
	SetInterpolation(interpolation Interpolation)
	// AlternativeText returns the description of the image for assistive
	// technologies.
	AlternativeText() string
	// SetAlternativeText changes the description of the image.
	SetAlternativeText(text string)
	// SourceScale is the number of source pixels per logical pixel.
	SourceScale() float64
	// SetSourceScale changes the number of source pixels per logical pixel.
	SetSourceScale(scale float64)
}

// Interpolation selects image resampling.
type Interpolation uint8

const (
	// InterpolationSmooth blends neighboring pixels.
	InterpolationSmooth Interpolation = iota
	// InterpolationNearest uses the nearest pixel, which keeps pixel art sharp.
	InterpolationNearest
)

// LinkOpenPolicy controls what happens when a link is activated.
type LinkOpenPolicy uint8

const (
	// LinkOpenNone leaves opening the URL to OnClick handlers.
	LinkOpenNone LinkOpenPolicy = iota
	// LinkOpenPlatform opens the URL with the platform's default handler
	// after OnClick handlers run.
	LinkOpenPlatform
)

// Link is a native hyperlink control. OnClick handlers receive activations;
// opening the URL is left to the application.
type Link interface {
	Widget
	// Text returns the text of the link.
	Text() string
	// SetText changes the text of the link.
	SetText(text string)
	// URL returns the target of the link.
	URL() string
	// SetURL changes the target of the link.
	SetURL(url string)
	// OnClick runs handler when the user activates the link.
	OnClick(handler ClickHandler) Unsubscribe
	// IsVisited reports whether the link shows the visited style.
	IsVisited() bool
	// SetVisited shows the visited style or the normal style.
	SetVisited(visited bool)
	// OpenPolicy returns what the link does when the user activates it.
	OpenPolicy() LinkOpenPolicy
	// SetOpenPolicy changes what the link does when the user activates it.
	SetOpenPolicy(policy LinkOpenPolicy)
	// Icon returns the image of the link.
	Icon() IconSource
	// SetIcon changes the image of the link.
	SetIcon(icon IconSource)
}

// TabPlacement places a tab view's tabs.
type TabPlacement uint8

const (
	// TabPlacementTop puts the tabs above the pages.
	TabPlacementTop TabPlacement = iota
	// TabPlacementBottom puts the tabs below the pages.
	TabPlacementBottom
	// TabPlacementLeft puts the tabs at the left of the pages.
	TabPlacementLeft
	// TabPlacementRight puts the tabs at the right of the pages.
	TabPlacementRight
)

// TabOverflowPolicy controls tabs that do not fit.
type TabOverflowPolicy uint8

const (
	// TabOverflowMenu puts the tabs that do not fit in a menu.
	TabOverflowMenu TabOverflowPolicy = iota
	// TabOverflowScroll lets the user scroll the tabs.
	TabOverflowScroll
)

// TabItem describes one page in a native tab view.
type TabItem struct {
	// Title is the label of the tab.
	Title string
	// Content is the page of the tab.
	Content Widget
}

// TabsOptions describes a native tab view.
type TabsOptions struct {
	// Items are the pages in order.
	Items []TabItem
	// SelectedIndex is the index of the initially visible page.
	SelectedIndex int
}

// Validate checks that every tab has content and the selection is valid.
func (options TabsOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: tabs require at least one item")
	}
	if options.SelectedIndex < 0 || options.SelectedIndex >= len(options.Items) {
		return errors.New("ui: tab selection is out of range")
	}
	for i, item := range options.Items {
		if item.Content == nil {
			return fmt.Errorf("ui: tab %d has no content", i)
		}
	}
	return nil
}

// Tabs is a native tab view containing one visible page at a time.
type Tabs interface {
	Widget
	// Items returns the pages.
	Items() []TabItem
	// SelectedIndex returns the index of the visible page.
	SelectedIndex() int
	// SetSelectedIndex shows the page at index.
	SetSelectedIndex(index int) error
	// OnChange runs handler after the user selects a tab.
	OnChange(handler SelectionChangeHandler) Unsubscribe
	// Placement returns the side of the tabs.
	Placement() TabPlacement
	// SetPlacement changes the side of the tabs.
	SetPlacement(placement TabPlacement)
	// OverflowPolicy returns what the view does with tabs that do not fit.
	OverflowPolicy() TabOverflowPolicy
	// SetOverflowPolicy changes what the view does with tabs that do not fit.
	SetOverflowPolicy(policy TabOverflowPolicy)
}

// ScrollbarPolicy controls scroll bar visibility.
type ScrollbarPolicy uint8

const (
	// ScrollbarsAutomatic shows the scroll bars as the platform decides.
	ScrollbarsAutomatic ScrollbarPolicy = iota
	// ScrollbarsAlways always shows the scroll bars.
	ScrollbarsAlways
	// ScrollbarsNever never shows the scroll bars.
	ScrollbarsNever
)

// ScrollBehavior controls programmatic scrolling.
type ScrollBehavior uint8

const (
	// ScrollInstant moves to the target immediately.
	ScrollInstant ScrollBehavior = iota
	// ScrollSmooth moves to the target with a short animation.
	ScrollSmooth
)

// OverscrollPolicy controls elastic scrolling past the content edges.
type OverscrollPolicy uint8

const (
	// OverscrollAutomatic lets the content move past its edges and spring back, as
	// the platform decides.
	OverscrollAutomatic OverscrollPolicy = iota
	// OverscrollDisabled stops the content at its edges.
	OverscrollDisabled
)

// ScrollAxes controls which directions a scroll view can move through its
// content. The zero value enables vertical scrolling.
type ScrollAxes uint8

const (
	// ScrollVertical scrolls up and down.
	ScrollVertical ScrollAxes = iota
	// ScrollHorizontal scrolls left and right.
	ScrollHorizontal
	// ScrollBoth scrolls in all directions.
	ScrollBoth
)

// ScrollViewOptions describes a native scrolling viewport.
type ScrollViewOptions struct {
	// Content is the widget to scroll.
	Content Widget
	// Axes are the directions in which the view scrolls.
	Axes ScrollAxes
}

// Validate checks that the scroll view has content and recognized axes.
func (options ScrollViewOptions) Validate() error {
	if options.Content == nil {
		return errors.New("ui: scroll view content is required")
	}
	if options.Axes > ScrollBoth {
		return fmt.Errorf("ui: invalid scroll axes %d", options.Axes)
	}
	return nil
}

// ScrollView displays content through a native scrolling viewport. ScrollOffset
// is measured in logical pixels from the content's top-left corner.
type ScrollView interface {
	Widget
	// Content returns the widget that the view scrolls.
	Content() Widget
	// ScrollOffset returns the position of the visible area.
	ScrollOffset() Point
	// SetScrollOffset scrolls to offset. ScrollBehavior controls the animation.
	SetScrollOffset(offset Point)
	// ScrollbarPolicy returns when the scroll bars show.
	ScrollbarPolicy() ScrollbarPolicy
	// SetScrollbarPolicy changes when the scroll bars show.
	SetScrollbarPolicy(policy ScrollbarPolicy)
	// ContentInsets returns the space around the content.
	ContentInsets() Insets
	// SetContentInsets changes the space around the content.
	SetContentInsets(insets Insets)
	// ScrollBehavior returns how programmatic scrolls move.
	ScrollBehavior() ScrollBehavior
	// SetScrollBehavior changes how programmatic scrolls move.
	SetScrollBehavior(behavior ScrollBehavior)
	// OverscrollPolicy returns whether the content can move past its edges.
	OverscrollPolicy() OverscrollPolicy
	// SetOverscrollPolicy changes whether the content can move past its edges.
	SetOverscrollPolicy(policy OverscrollPolicy)
	// Zoom is the content magnification; one is unscaled.
	Zoom() float64
	// SetZoom changes the magnification. It returns an error when zoom is not
	// positive and finite.
	SetZoom(zoom float64) error
	// ViewportSize is the visible content area in logical pixels.
	ViewportSize() Size
	// ScrollToWidget scrolls until widget, a descendant of Content, is visible.
	ScrollToWidget(widget Widget) error
}

// FrameRequest is a display-synchronized request to draw a frame.
type FrameRequest struct {
	// Time is when the frame will be presented.
	Time time.Time
	// Interval is the display refresh interval.
	Interval time.Duration
}

// FrameRequestHandler receives display-synchronized frame requests.
type FrameRequestHandler func(request FrameRequest)

// OcclusionHandler receives whether a surface is fully hidden.
type OcclusionHandler func(occluded bool)

// SurfaceEventKind identifies a GPU surface change.
type SurfaceEventKind uint8

const (
	// SurfaceResized reports a new framebuffer size.
	SurfaceResized SurfaceEventKind = iota
	// SurfaceScaleChanged reports a new backing scale factor.
	SurfaceScaleChanged
	// SurfaceColorSpaceChanged reports a new display color space.
	SurfaceColorSpaceChanged
	// SurfaceLost reports that the surface must be recreated.
	SurfaceLost
)

// SurfaceEvent reports a change to a canvas's GPU surface.
type SurfaceEvent struct {
	// Kind is the change.
	Kind SurfaceEventKind
	// FramebufferSize is the size of the framebuffer in physical pixels.
	FramebufferSize Size
	// ScaleFactor is the number of physical pixels in one logical pixel.
	ScaleFactor float64
	// ColorSpace is the color space of the surface.
	ColorSpace ColorSpace
}

// SurfaceHandler receives GPU surface changes.
type SurfaceHandler func(event SurfaceEvent)

// Canvas is a native control backed by a GPU-presentable surface. It can be
// placed beside ordinary controls in the same window. RequestRedraw marks it
// dirty; TakeRedrawRequest reports and clears that state. Its pointer input is
// its own, separate from the window's: see PointerInput.
type Canvas interface {
	Widget
	Drawable
	PointerInput
	KeyInput
	// RequestRedraw marks the canvas as needing a new frame.
	RequestRedraw()
	// TakeRedrawRequest reports whether a redraw request is pending, and clears
	// it.
	TakeRedrawRequest() bool

	// CapturePointer routes all pointer input to the canvas until
	// ReleasePointer, even outside its bounds.
	CapturePointer() error
	// ReleasePointer ends a pointer capture.
	ReleasePointer()
	// HasPointerCapture reports whether the canvas captures the pointer.
	HasPointerCapture() bool

	// OnFrameRequest runs handler when the display is ready for a new frame.
	OnFrameRequest(handler FrameRequestHandler) Unsubscribe
	// OnOcclusionChange runs handler when the canvas becomes fully hidden or
	// visible again.
	OnOcclusionChange(handler OcclusionHandler) Unsubscribe
	// OnSurfaceChange runs handler after the GPU surface changes.
	OnSurfaceChange(handler SurfaceHandler) Unsubscribe
	// ColorSpace returns the color space of the surface.
	ColorSpace() ColorSpace
}

// SectionOptions describes a titled, collapsible native container.
type SectionOptions struct {
	// Title is the text of the header.
	Title string
	// Content is the widget that the section shows when expanded.
	Content Widget
	// Expanded shows the content initially.
	Expanded bool
}

// Validate checks that the section has content.
func (options SectionOptions) Validate() error {
	if options.Content == nil {
		return errors.New("ui: section content is required")
	}
	return nil
}

// Section is a titled container whose content can be expanded or collapsed.
// Multiple sections in a column form an accordion.
type Section interface {
	Widget
	// Title returns the text of the header.
	Title() string
	// SetTitle changes the text of the header.
	SetTitle(title string)
	// Content returns the widget in the section.
	Content() Widget
	// IsExpanded reports whether the content shows.
	IsExpanded() bool
	// SetExpanded shows or hides the content.
	SetExpanded(expanded bool)
	// OnChange runs handler after the user expands or collapses the section.
	OnChange(handler ExpansionChangeHandler) Unsubscribe
	// Subtitle returns the secondary text of the header.
	Subtitle() string
	// SetSubtitle changes the secondary text of the header.
	SetSubtitle(subtitle string)
	// Icon returns the image of the header.
	Icon() IconSource
	// SetIcon changes the image of the header.
	SetIcon(icon IconSource)
	// Badge returns the badge text of the header.
	Badge() string
	// SetBadge changes the badge text. An empty text removes the badge.
	SetBadge(badge string)
	// IsAnimationEnabled reports whether expansion animates.
	IsAnimationEnabled() bool
	// SetAnimationEnabled starts or stops the expansion animation.
	SetAnimationEnabled(enabled bool)
}
