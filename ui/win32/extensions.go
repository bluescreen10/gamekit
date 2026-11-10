//go:build windows

package win32

import (
	"errors"
	"math"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Label ----

type labelState struct {
	alignment    ui.TextAlignment
	wrap         ui.WrapMode
	truncation   ui.Truncation
	selectable   bool
	style        ui.TextStyle
	maximumLines int
}

func (l *label) apply() {
	if native := l.nativeView(); native != nil {
		s := l.state
		native.configureLabel(int(s.alignment), s.wrap != ui.WrapNone, int(s.truncation),
			s.selectable, int(s.style), s.maximumLines)
	}
}

func (l *label) TextAlignment() ui.TextAlignment {
	return l.state.alignment
}

func (l *label) SetTextAlignment(alignment ui.TextAlignment) {
	l.state.alignment = alignment
	l.apply()
}

func (l *label) WrapMode() ui.WrapMode {
	return l.state.wrap
}

func (l *label) SetWrapMode(mode ui.WrapMode) {
	l.state.wrap = mode
	l.apply()
}

func (l *label) Truncation() ui.Truncation {
	return l.state.truncation
}

func (l *label) SetTruncation(truncation ui.Truncation) {
	l.state.truncation = truncation
	l.apply()
}

func (l *label) IsSelectable() bool {
	return l.state.selectable
}

func (l *label) SetSelectable(selectable bool) {
	l.state.selectable = selectable
	l.apply()
}

func (l *label) SemanticStyle() ui.TextStyle {
	return l.state.style
}

func (l *label) SetSemanticStyle(style ui.TextStyle) {
	l.state.style = style
	l.apply()
}

func (l *label) MaximumLines() int {
	return l.state.maximumLines
}

func (l *label) SetMaximumLines(lines int) {
	l.state.maximumLines = max(0, lines)
	l.apply()
}

// SetBounds lets a wrapping label measure its height for its width.
func (l *label) SetBounds(bounds ui.Rect) {
	if native := l.nativeView(); native != nil && l.state.wrap != ui.WrapNone {
		native.labelPreferredWidth(bounds.Size.Width)
	}
	l.widgetBase.SetBounds(bounds)
}

// ---- Button ----

type buttonState struct {
	icon      ui.IconSource
	placement ui.IconPlacement
	role      ui.ButtonRole
	shortcut  ui.Shortcut
	busy      bool
	badge     string
}

func (b *button) apply() {
	native := b.nativeView()
	if native == nil {
		return
	}
	icon := newIcon(b.state.icon, 0)
	key, modifiers := keyEquivalent(b.state.shortcut)
	native.configureButton(icon, int(b.state.placement), int(b.state.role), key, modifiers)
	// The icon and its placement change the preferred size.
	b.relayoutParent()
}

func (b *button) Icon() (ui.IconSource, ui.IconPlacement) {
	return b.state.icon, b.state.placement
}

func (b *button) SetIcon(icon ui.IconSource, placement ui.IconPlacement) {
	b.state.icon, b.state.placement = icon, placement
	b.apply()
}

func (b *button) Role() ui.ButtonRole {
	return b.state.role
}

func (b *button) SetRole(role ui.ButtonRole) {
	b.state.role = role
	b.apply()
}

func (b *button) Shortcut() ui.Shortcut {
	return b.state.shortcut
}

func (b *button) SetShortcut(shortcut ui.Shortcut) {
	b.state.shortcut = shortcut
	b.apply()
}

func (b *button) IsBusy() bool {
	return b.state.busy
}

func (b *button) SetBusy(busy bool) {
	b.state.busy = busy
	if native := b.nativeView(); native != nil {
		native.setButtonBusy(busy)
	}
}

func (b *button) Badge() string {
	return b.state.badge
}

func (b *button) SetBadge(badge string) {
	b.state.badge = badge
	if native := b.nativeView(); native != nil {
		native.setButtonBadge(badge)
	}
}

// ---- Text ----

// runeRange converts a rune range in text to UTF-16 offsets, the unit of the
// selection functions of the native layer.
func runeRange(text string, selection ui.TextRange) (start, length int) {
	runes := []rune(text)
	clamp := func(index int) int {
		return max(0, min(index, len(runes)))
	}
	begin, end := clamp(selection.Start), clamp(max(selection.Start, selection.End))
	start = len(utf16.Encode(runes[:begin]))
	length = len(utf16.Encode(runes[begin:end]))
	return start, length
}

func selectionOf(native *nativeView, text string) ui.TextRange {
	start, length := native.textSelection()
	return utf16Range(text, int(start), int(length))
}

type textFieldState struct {
	readOnly      bool
	maximumLength int
	purpose       ui.InputPurpose
	alignment     ui.TextAlignment
	prefix        string
	suffix        string
	spellCheck    bool
	selection     ui.TextRange
	submits       listeners[ui.TextChangeHandler]
}

func (t *textField) apply() {
	native := t.nativeView()
	if native == nil {
		return
	}
	s := &t.state
	native.configureTextField(s.readOnly, s.maximumLength, int(s.purpose),
		int(s.alignment), s.prefix, s.suffix, s.spellCheck)
}

func (t *textField) OnSubmit(handler ui.TextChangeHandler) ui.Unsubscribe {
	return subscribe(&t.state.submits, handler, handler == nil)
}

func (t *textField) IsReadOnly() bool {
	return t.state.readOnly
}

func (t *textField) SetReadOnly(readOnly bool) {
	t.state.readOnly = readOnly
	t.apply()
}

func (t *textField) Selection() ui.TextRange {
	native := t.nativeView()
	if native == nil {
		return ui.TextRange{}
	}
	return selectionOf(native, t.Text())
}

func (t *textField) SetSelection(selection ui.TextRange) {
	if native := t.nativeView(); native != nil {
		start, length := runeRange(t.Text(), selection)
		native.setTextSelection(start, length)
	}
}

func (t *textField) MaximumLength() int {
	return t.state.maximumLength
}

func (t *textField) SetMaximumLength(length int) {
	t.state.maximumLength = max(0, length)
	t.apply()
	t.enforceMaximumLength()
}

func (t *textField) InputPurpose() ui.InputPurpose {
	return t.state.purpose
}

func (t *textField) SetInputPurpose(purpose ui.InputPurpose) {
	t.state.purpose = purpose
	t.apply()
}

func (t *textField) TextAlignment() ui.TextAlignment {
	return t.state.alignment
}

func (t *textField) SetTextAlignment(alignment ui.TextAlignment) {
	t.state.alignment = alignment
	t.apply()
}

func (t *textField) Prefix() string {
	return t.state.prefix
}

func (t *textField) SetPrefix(prefix string) {
	t.state.prefix = prefix
	t.apply()
}

func (t *textField) Suffix() string {
	return t.state.suffix
}

func (t *textField) SetSuffix(suffix string) {
	t.state.suffix = suffix
	t.apply()
}

func (t *textField) IsSpellCheckEnabled() bool {
	return t.state.spellCheck
}

func (t *textField) SetSpellCheckEnabled(enabled bool) {
	t.state.spellCheck = enabled
	t.apply()
}

// enforceMaximumLength truncates text that exceeds the limit. It reports
// whether it changed the text.
func (t *textField) enforceMaximumLength() bool {
	limit := t.state.maximumLength
	if limit <= 0 {
		return false
	}
	text := t.Text()
	if utf8.RuneCountInString(text) <= limit {
		return false
	}
	t.SetText(string([]rune(text)[:limit]))
	return true
}

type textAreaState struct {
	wrap          ui.WrapMode
	maximumLength int
	tabPolicy     ui.TabPolicy
	spellCheck    bool
}

func (t *textArea) apply() {
	if native := t.nativeView(); native != nil {
		s := t.state
		native.configureTextArea(s.wrap != ui.WrapNone, s.maximumLength,
			s.tabPolicy == ui.TabMovesFocus, s.spellCheck)
	}
}

func (t *textArea) WrapMode() ui.WrapMode {
	return t.state.wrap
}

func (t *textArea) SetWrapMode(mode ui.WrapMode) {
	t.state.wrap = mode
	t.apply()
}

func (t *textArea) Selection() ui.TextRange {
	native := t.nativeView()
	if native == nil {
		return ui.TextRange{}
	}
	return selectionOf(native, t.Text())
}

func (t *textArea) SetSelection(selection ui.TextRange) {
	if native := t.nativeView(); native != nil {
		start, length := runeRange(t.Text(), selection)
		native.setTextSelection(start, length)
	}
}

func (t *textArea) ScrollPosition() ui.Point {
	native := t.nativeView()
	if native == nil {
		return ui.Point{}
	}
	x, y := native.scrollPosition()
	return ui.Point{X: int(x), Y: int(y)}
}

func (t *textArea) SetScrollPosition(position ui.Point) {
	if native := t.nativeView(); native != nil {
		native.setScrollPosition(position.X, position.Y, false)
	}
}

func (t *textArea) MaximumLength() int {
	return t.state.maximumLength
}

func (t *textArea) SetMaximumLength(length int) {
	t.state.maximumLength = max(0, length)
	t.apply()
}

func (t *textArea) TabPolicy() ui.TabPolicy {
	return t.state.tabPolicy
}

func (t *textArea) SetTabPolicy(policy ui.TabPolicy) {
	t.state.tabPolicy = policy
	t.apply()
}

func (t *textArea) IsSpellCheckEnabled() bool {
	return t.state.spellCheck
}

func (t *textArea) SetSpellCheckEnabled(enabled bool) {
	t.state.spellCheck = enabled
	t.apply()
}

// ---- Check box ----

type checkBoxState struct {
	allowsMixed bool
	placement   ui.LabelPlacement
	description string
}

func (c *checkBox) apply(state ui.CheckState) {
	native := c.nativeView()
	if native == nil {
		return
	}
	native.configureCheckBox(c.state.allowsMixed, int(state),
		c.state.placement == ui.LabelBeforeControl, c.Text(), c.state.description)
}

func (c *checkBox) State() ui.CheckState {
	native := c.nativeView()
	if native == nil {
		return ui.CheckOff
	}
	return ui.CheckState(native.checkBoxState())
}

func (c *checkBox) SetState(state ui.CheckState) {
	if state == ui.CheckMixed && !c.state.allowsMixed {
		state = ui.CheckOff
	}
	c.apply(state)
}

func (c *checkBox) AllowsMixedState() bool {
	return c.state.allowsMixed
}

func (c *checkBox) SetAllowsMixedState(allow bool) {
	c.state.allowsMixed = allow
	state := c.State()
	if state == ui.CheckMixed && !allow {
		state = ui.CheckOff
	}
	c.apply(state)
}

func (c *checkBox) LabelPlacement() ui.LabelPlacement {
	return c.state.placement
}

func (c *checkBox) SetLabelPlacement(placement ui.LabelPlacement) {
	c.state.placement = placement
	c.apply(c.State())
}

func (c *checkBox) Description() string {
	return c.state.description
}

func (c *checkBox) SetDescription(description string) {
	c.state.description = description
	c.apply(c.State())
}

// ---- Choices ----

func choicesFromItems(items []string) []ui.Choice {
	choices := make([]ui.Choice, len(items))
	for i, item := range items {
		choices[i] = ui.Choice{ID: item, Text: item, Enabled: true}
	}
	return choices
}

func (s *selection) applyChoices(choices []ui.Choice) error {
	if err := ui.ValidateChoices(choices); err != nil {
		return err
	}
	native := s.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	selectedID := ""
	if index := s.SelectedIndex(); index >= 0 && index < len(s.choices) {
		selectedID = s.choices[index].ID
	}
	items := make([]string, len(choices))
	selected := -1
	for i, choice := range choices {
		items[i] = choice.Text
		if choice.ID == selectedID {
			selected = i
		}
	}
	native.setChoices(choices)
	s.items = items
	s.choices = append([]ui.Choice(nil), choices...)
	if selected < 0 && len(choices) > 0 {
		selected = 0
	}
	native.setSelectionIndex(selected)
	return nil
}

func (s *selection) Choices() []ui.Choice {
	return append([]ui.Choice(nil), s.choices...)
}

func (s *selection) SetChoices(choices []ui.Choice) error {
	return s.applyChoices(choices)
}

type radioGroupState struct {
	orientation ui.LayoutDirection
	label       string
}

func (r *radioGroup) applyLayout() {
	if native := r.nativeView(); native != nil {
		native.configureRadioGroup(r.state.orientation == ui.Horizontal, r.state.label)
		r.relayoutParent()
	}
}

func (r *radioGroup) Orientation() ui.LayoutDirection {
	return r.state.orientation
}

func (r *radioGroup) SetOrientation(orientation ui.LayoutDirection) {
	r.state.orientation = orientation
	r.applyLayout()
}

func (r *radioGroup) Label() string {
	return r.state.label
}

func (r *radioGroup) SetLabel(label string) {
	r.state.label = label
	r.applyLayout()
}

type selectState struct {
	placeholder string
	popupWidth  int
	visibleRows int
}

func (s *selectControl) applyPresentation() {
	if native := s.nativeView(); native != nil {
		native.configureSelect(s.state.placeholder, s.state.popupWidth)
	}
}

func (s *selectControl) Placeholder() string {
	return s.state.placeholder
}

func (s *selectControl) SetPlaceholder(placeholder string) {
	s.state.placeholder = placeholder
	s.applyPresentation()
}

func (s *selectControl) PopupWidth() int {
	return s.state.popupWidth
}

func (s *selectControl) SetPopupWidth(width int) {
	s.state.popupWidth = max(0, width)
	s.applyPresentation()
}

// VisibleRowCount returns the number of rows that the pop-up list shows
// before it scrolls. Zero lets Windows choose.
func (s *selectControl) VisibleRowCount() int {
	return s.state.visibleRows
}

// SetVisibleRowCount limits the height of the pop-up list to rows rows. Zero
// lets Windows choose.
func (s *selectControl) SetVisibleRowCount(rows int) {
	s.state.visibleRows = max(0, rows)
	if native := s.nativeView(); native != nil {
		native.setVisibleRows(s.state.visibleRows)
	}
}

// ---- Value controls ----

type sliderState struct {
	orientation ui.LayoutDirection
	inverted    bool
	ticks       ui.SliderTicks
}

func (s *slider) apply() {
	if native := s.nativeView(); native != nil {
		native.configureSlider(s.state.orientation == ui.Vertical,
			s.minimum, s.maximum, s.ticks().Frequency, s.ticks().SnapToTicks)
	}
}

func (s *slider) ticks() ui.SliderTicks {
	ticks := s.state.ticks
	if ticks.Frequency == 0 && s.step > 0 {
		ticks = ui.SliderTicks{Frequency: s.step, SnapToTicks: true}
	}
	return ticks
}

func (s *slider) Orientation() ui.LayoutDirection {
	return s.state.orientation
}

func (s *slider) SetOrientation(orientation ui.LayoutDirection) {
	s.state.orientation = orientation
	s.apply()
}

// IsInverted reports a reversed value direction.
func (s *slider) IsInverted() bool {
	return s.state.inverted
}

// SetInverted reverses the direction in which the value increases. The
// native layer maps the trackbar position, so the value does not change.
func (s *slider) SetInverted(inverted bool) {
	s.state.inverted = inverted
	if native := s.nativeView(); native != nil {
		native.setSliderInverted(inverted)
	}
}

func (s *slider) Ticks() ui.SliderTicks {
	return s.state.ticks
}

func (s *slider) SetTicks(ticks ui.SliderTicks) {
	if math.IsNaN(ticks.Frequency) || ticks.Frequency < 0 {
		ticks.Frequency = 0
	}
	s.state.ticks = ticks
	s.apply()
}

type progressState struct {
	mode        ui.ProgressMode
	orientation ui.LayoutDirection
	text        string
	condition   ui.ProgressState
}

func (p *progressBar) apply() {
	if native := p.nativeView(); native != nil {
		native.configureProgress(p.state.mode == ui.ProgressIndeterminate,
			p.state.orientation == ui.Vertical, p.state.text, int(p.state.condition))
	}
}

func (p *progressBar) Mode() ui.ProgressMode {
	return p.state.mode
}

func (p *progressBar) SetMode(mode ui.ProgressMode) {
	p.state.mode = mode
	p.apply()
}

func (p *progressBar) Orientation() ui.LayoutDirection {
	return p.state.orientation
}

func (p *progressBar) SetOrientation(orientation ui.LayoutDirection) {
	p.state.orientation = orientation
	p.apply()
}

func (p *progressBar) Text() string {
	return p.state.text
}

func (p *progressBar) SetText(text string) {
	p.state.text = text
	p.apply()
	p.announceText(text)
}

func (p *progressBar) State() ui.ProgressState {
	return p.state.condition
}

func (p *progressBar) SetState(state ui.ProgressState) {
	p.state.condition = state
	p.apply()
}

type activityState struct {
	size  ui.ControlSize
	label string
}

func (a *activityIndicator) apply() {
	if native := a.nativeView(); native != nil {
		native.configureActivity(int(a.state.size), a.state.label)
	}
}

func (a *activityIndicator) ControlSize() ui.ControlSize {
	return a.state.size
}

func (a *activityIndicator) SetControlSize(size ui.ControlSize) {
	a.state.size = size
	a.apply()
}

func (a *activityIndicator) Label() string {
	return a.state.label
}

func (a *activityIndicator) SetLabel(label string) {
	a.state.label = label
	a.apply()
}

// ---- Presentation ----

type separatorState struct {
	label        string
	presentation ui.SeparatorPresentation
	thickness    int
}

func (s *separator) apply() {
	if native := s.nativeView(); native != nil {
		native.configureSeparator(s.direction == ui.Vertical, s.state.label,
			s.state.presentation == ui.SeparatorInset, s.Thickness())
		s.relayoutParent()
	}
}

func (s *separator) SetDirection(direction ui.LayoutDirection) {
	s.direction = direction
	s.apply()
}

func (s *separator) Label() string {
	return s.state.label
}

func (s *separator) SetLabel(label string) {
	s.state.label = label
	s.apply()
}

func (s *separator) Presentation() ui.SeparatorPresentation {
	return s.state.presentation
}

func (s *separator) SetPresentation(presentation ui.SeparatorPresentation) {
	s.state.presentation = presentation
	s.apply()
}

func (s *separator) Thickness() int {
	if s.state.thickness <= 0 {
		return 1
	}
	return s.state.thickness
}

func (s *separator) SetThickness(thickness int) {
	s.state.thickness = max(1, thickness)
	s.apply()
}

type imageState struct {
	alignment     ui.Anchor
	interpolation ui.Interpolation
	alternative   string
	scale         float64
}

func (i *imageView) apply() {
	if native := i.nativeView(); native != nil {
		native.configureImage(int(i.state.alignment),
			i.state.interpolation == ui.InterpolationNearest, i.state.alternative, i.SourceScale())
	}
	// The source scale changes the preferred size.
	i.relayoutParent()
}

func (i *imageView) Alignment() ui.Anchor {
	return i.state.alignment
}

func (i *imageView) SetAlignment(alignment ui.Anchor) {
	i.state.alignment = alignment
	i.apply()
}

func (i *imageView) Interpolation() ui.Interpolation {
	return i.state.interpolation
}

func (i *imageView) SetInterpolation(interpolation ui.Interpolation) {
	i.state.interpolation = interpolation
	i.apply()
}

func (i *imageView) AlternativeText() string {
	return i.state.alternative
}

func (i *imageView) SetAlternativeText(text string) {
	i.state.alternative = text
	i.apply()
}

func (i *imageView) SourceScale() float64 {
	if i.state.scale <= 0 {
		return 1
	}
	return i.state.scale
}

func (i *imageView) SetSourceScale(scale float64) {
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
		scale = 1
	}
	i.state.scale = scale
	i.apply()
}

type linkState struct {
	visited bool
	policy  ui.LinkOpenPolicy
	icon    ui.IconSource
}

func (l *link) apply() {
	if native := l.nativeView(); native != nil {
		icon := newIcon(l.state.icon, 13)
		native.configureLink(l.state.visited, icon)
	}
}

func (l *link) IsVisited() bool {
	return l.state.visited
}

func (l *link) SetVisited(visited bool) {
	l.state.visited = visited
	l.apply()
}

func (l *link) OpenPolicy() ui.LinkOpenPolicy {
	return l.state.policy
}

func (l *link) SetOpenPolicy(policy ui.LinkOpenPolicy) {
	l.state.policy = policy
}

func (l *link) Icon() ui.IconSource {
	return l.state.icon
}

func (l *link) SetIcon(icon ui.IconSource) {
	l.state.icon = icon
	l.apply()
}

// opened runs after OnClick handlers.
func (l *link) opened() {
	if l.state.policy != ui.LinkOpenPlatform {
		return
	}
	if openURL(l.URL(), false) && !l.state.visited {
		l.SetVisited(true)
	}
}

// ---- Containers ----

type tabsState struct {
	placement ui.TabPlacement
	overflow  ui.TabOverflowPolicy
}

func (t *tabs) Placement() ui.TabPlacement {
	return t.state.placement
}

func (t *tabs) SetPlacement(placement ui.TabPlacement) {
	t.state.placement = placement
	if native := t.nativeView(); native != nil {
		native.configureTabs(int(placement))
		t.layoutContent()
	}
}

// OverflowPolicy returns how the tab view shows tabs that do not fit.
func (t *tabs) OverflowPolicy() ui.TabOverflowPolicy {
	return t.state.overflow
}

// SetOverflowPolicy changes how the tab view shows tabs that do not fit.
// Both policies let the user scroll the tabs; TabOverflowMenu also lists
// every tab in a menu that opens from the tab bar.
func (t *tabs) SetOverflowPolicy(policy ui.TabOverflowPolicy) {
	t.state.overflow = policy
	if native := t.nativeView(); native != nil {
		native.setTabOverflow(int(policy))
	}
}

type scrollState struct {
	policy     ui.ScrollbarPolicy
	insets     ui.Insets
	behavior   ui.ScrollBehavior
	overscroll ui.OverscrollPolicy
}

func (s *scrollView) apply() {
	if native := s.nativeView(); native != nil {
		i := s.state.insets
		native.configureScroll(int(s.axes), int(s.state.policy), i.Top, i.Right,
			i.Bottom, i.Left, s.state.overscroll == ui.OverscrollDisabled)
		s.layoutContent()
	}
}

func (s *scrollView) ScrollbarPolicy() ui.ScrollbarPolicy {
	return s.state.policy
}

func (s *scrollView) SetScrollbarPolicy(policy ui.ScrollbarPolicy) {
	s.state.policy = policy
	s.apply()
}

func (s *scrollView) ContentInsets() ui.Insets {
	return s.state.insets
}

func (s *scrollView) SetContentInsets(insets ui.Insets) {
	s.state.insets = insets
	s.apply()
}

func (s *scrollView) ScrollBehavior() ui.ScrollBehavior {
	return s.state.behavior
}

func (s *scrollView) SetScrollBehavior(behavior ui.ScrollBehavior) {
	s.state.behavior = behavior
}

func (s *scrollView) OverscrollPolicy() ui.OverscrollPolicy {
	return s.state.overscroll
}

func (s *scrollView) SetOverscrollPolicy(policy ui.OverscrollPolicy) {
	s.state.overscroll = policy
	s.apply()
}

func (s *scrollView) Zoom() float64 {
	native := s.nativeView()
	if native == nil {
		return 1
	}
	return float64(native.zoomFactor())
}

func (s *scrollView) SetZoom(zoom float64) error {
	if math.IsNaN(zoom) || math.IsInf(zoom, 0) || zoom <= 0 {
		return errors.New("ui: zoom must be positive and finite")
	}
	native := s.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.setZoom(zoom)
	return nil
}

func (s *scrollView) ViewportSize() ui.Size {
	native := s.nativeView()
	if native == nil {
		return ui.Size{}
	}
	width, height := native.scrollViewportSize()
	size := ui.Size{Width: int(width), Height: int(height)}
	if !size.IsValid() {
		bounds := s.Bounds().Size
		size = ui.Size{Width: max(1, bounds.Width), Height: max(1, bounds.Height)}
	}
	return size
}

func (s *scrollView) ScrollToWidget(widget ui.Widget) error {
	target, ok := widget.(nativeWidget)
	if !ok || target.base().window != s.window {
		return errors.New("ui/win32: widget belongs to another backend or window")
	}
	native := s.nativeView()
	view := target.base().nativeView()
	if native == nil || view == nil {
		return ui.ErrClosed
	}
	if !native.scrollToView(view, s.state.behavior == ui.ScrollSmooth) {
		return errors.New("ui/win32: widget is not inside the scroll view")
	}
	return nil
}

type sectionState struct {
	subtitle string
	icon     ui.IconSource
	badge    string
	animated bool
}

func (s *section) apply() {
	if native := s.nativeView(); native != nil {
		icon := newIcon(s.state.icon, 13)
		native.configureSection(s.state.subtitle, icon, s.state.badge)
	}
}

func (s *section) Subtitle() string {
	return s.state.subtitle
}

func (s *section) SetSubtitle(subtitle string) {
	s.state.subtitle = subtitle
	s.apply()
}

func (s *section) Icon() ui.IconSource {
	return s.state.icon
}

func (s *section) SetIcon(icon ui.IconSource) {
	s.state.icon = icon
	s.apply()
}

func (s *section) Badge() string {
	return s.state.badge
}

func (s *section) SetBadge(badge string) {
	s.state.badge = badge
	s.apply()
}

// IsAnimationEnabled reports whether the content slides in when the section
// expands.
func (s *section) IsAnimationEnabled() bool {
	return s.state.animated
}

// SetAnimationEnabled makes the content slide in when the section expands.
// The parent layout changes the size of the section immediately.
func (s *section) SetAnimationEnabled(enabled bool) {
	s.state.animated = enabled
	if native := s.nativeView(); native != nil {
		native.setSectionAnimated(enabled)
	}
}
