package unsupported

import (
	"context"

	"github.com/bluescreen10/gamekit/ui"
)

// Widget implements the planned additions to ui.Widget.
type Widget struct{}

func (Widget) Accessibility() (_ ui.Accessibility) {
	return
}

func (Widget) AccessibilityRole() (_ ui.AccessibilityRole) {
	return
}

func (Widget) Cursor() (_ ui.Cursor) {
	return
}

func (Widget) Focus() (err error) {
	err = Error("ui.Widget.Focus")
	return
}

func (Widget) IsFocused() (_ bool) {
	return
}

func (Widget) Margins() (_ ui.Insets) {
	return
}

func (Widget) MaximumSize() (_ ui.Size) {
	return
}

func (Widget) MinimumSize() (_ ui.Size) {
	return
}

func (Widget) OnBlur(_ ui.FocusHandler) (_ ui.Unsubscribe) {
	Panic("ui.Widget.OnBlur")
	return
}

func (Widget) OnDrop(_ ui.DropHandler) (_ ui.Unsubscribe) {
	Panic("ui.Widget.OnDrop")
	return
}

func (Widget) OnFocus(_ ui.FocusHandler) (_ ui.Unsubscribe) {
	Panic("ui.Widget.OnFocus")
	return
}

func (Widget) OnResize(_ ui.ResizeHandler) (_ ui.Unsubscribe) {
	Panic("ui.Widget.OnResize")
	return
}

func (Widget) SetAccessibility(_ ui.Accessibility) (err error) {
	err = Error("ui.Widget.SetAccessibility")
	return
}

func (Widget) SetContextMenu(_ ui.ContextMenu) (err error) {
	err = Error("ui.Widget.SetContextMenu")
	return
}

func (Widget) SetCursor(_ ui.Cursor) (err error) {
	err = Error("ui.Widget.SetCursor")
	return
}

func (Widget) SetDragSource(_ ui.DragSource) (err error) {
	err = Error("ui.Widget.SetDragSource")
	return
}

func (Widget) SetDropTarget(_ ui.DropTarget) (err error) {
	err = Error("ui.Widget.SetDropTarget")
	return
}

func (Widget) SetMargins(_ ui.Insets) {
	Panic("ui.Widget.SetMargins")
	return
}

func (Widget) SetMaximumSize(_ ui.Size) {
	Panic("ui.Widget.SetMaximumSize")
	return
}

func (Widget) SetMinimumSize(_ ui.Size) {
	Panic("ui.Widget.SetMinimumSize")
	return
}

func (Widget) SetStyle(_ ui.WidgetStyle) (err error) {
	err = Error("ui.Widget.SetStyle")
	return
}

func (Widget) SetTooltip(_ string) {
	Panic("ui.Widget.SetTooltip")
	return
}

func (Widget) Style() (_ ui.WidgetStyle) {
	return
}

func (Widget) Tooltip() (_ string) {
	return
}

// Window implements the planned additions to ui.Window.
type Window struct{}

func (Window) AccessibilityTree() (_ ui.AccessibilityNode) {
	return
}

func (Window) Clipboard() (_ ui.Clipboard) {
	return
}

func (Window) CreateAccordion(_ ui.AccordionOptions) (_ ui.Accordion, err error) {
	err = Error("ui.Window.CreateAccordion")
	return
}

func (Window) CreateAnimatedImage(_ ui.AnimatedImageOptions) (_ ui.AnimatedImage, err error) {
	err = Error("ui.Window.CreateAnimatedImage")
	return
}

func (Window) CreateBadge(_ ui.BadgeOptions) (_ ui.Badge, err error) {
	err = Error("ui.Window.CreateBadge")
	return
}

func (Window) CreateBreadcrumbs(_ ui.BreadcrumbsOptions) (_ ui.Breadcrumbs, err error) {
	err = Error("ui.Window.CreateBreadcrumbs")
	return
}

func (Window) CreateCalendar(_ ui.CalendarOptions) (_ ui.Calendar, err error) {
	err = Error("ui.Window.CreateCalendar")
	return
}

func (Window) CreateCollectionView(_ ui.CollectionViewOptions) (_ ui.CollectionView, err error) {
	err = Error("ui.Window.CreateCollectionView")
	return
}

func (Window) CreateColorPicker(_ ui.ColorPickerOptions) (_ ui.ColorPicker, err error) {
	err = Error("ui.Window.CreateColorPicker")
	return
}

func (Window) CreateCommandBar(_ ui.CommandBarOptions) (_ ui.CommandBar, err error) {
	err = Error("ui.Window.CreateCommandBar")
	return
}

func (Window) CreateCommandPalette(_ ui.CommandPaletteOptions) (_ ui.CommandPalette, err error) {
	err = Error("ui.Window.CreateCommandPalette")
	return
}

func (Window) CreateContextMenu(_ ui.ContextMenuOptions) (_ ui.ContextMenu, err error) {
	err = Error("ui.Window.CreateContextMenu")
	return
}

func (Window) CreateDatePicker(_ ui.DatePickerOptions) (_ ui.DatePicker, err error) {
	err = Error("ui.Window.CreateDatePicker")
	return
}

func (Window) CreateDial(_ ui.DialOptions) (_ ui.Dial, err error) {
	err = Error("ui.Window.CreateDial")
	return
}

func (Window) CreateDockingLayout(_ ui.DockingLayoutOptions) (_ ui.DockingLayout, err error) {
	err = Error("ui.Window.CreateDockingLayout")
	return
}

func (Window) CreateDrawer(_ ui.DrawerOptions) (_ ui.Drawer, err error) {
	err = Error("ui.Window.CreateDrawer")
	return
}

func (Window) CreateEditableCombo(_ ui.EditableComboOptions) (_ ui.EditableCombo, err error) {
	err = Error("ui.Window.CreateEditableCombo")
	return
}

func (Window) CreateEmptyState(_ ui.EmptyStateOptions) (_ ui.EmptyState, err error) {
	err = Error("ui.Window.CreateEmptyState")
	return
}

func (Window) CreateFlowLayout(_ ui.FlowLayoutOptions) (_ ui.FlowLayout, err error) {
	err = Error("ui.Window.CreateFlowLayout")
	return
}

func (Window) CreateFontPicker(_ ui.FontPickerOptions) (_ ui.FontPicker, err error) {
	err = Error("ui.Window.CreateFontPicker")
	return
}

func (Window) CreateFormLayout(_ ui.FormLayoutOptions) (_ ui.FormLayout, err error) {
	err = Error("ui.Window.CreateFormLayout")
	return
}

func (Window) CreateGridLayout(_ ui.GridLayoutOptions) (_ ui.GridLayout, err error) {
	err = Error("ui.Window.CreateGridLayout")
	return
}

func (Window) CreateGroupBox(_ ui.GroupBoxOptions) (_ ui.GroupBox, err error) {
	err = Error("ui.Window.CreateGroupBox")
	return
}

func (Window) CreateIcon(_ ui.IconOptions) (_ ui.Icon, err error) {
	err = Error("ui.Window.CreateIcon")
	return
}

func (Window) CreateInfoBar(_ ui.InfoBarOptions) (_ ui.InfoBar, err error) {
	err = Error("ui.Window.CreateInfoBar")
	return
}

func (Window) CreateInlineValidation(_ ui.InlineValidationOptions) (_ ui.InlineValidation, err error) {
	err = Error("ui.Window.CreateInlineValidation")
	return
}

func (Window) CreateKeyRecorder(_ ui.KeyRecorderOptions) (_ ui.KeyRecorder, err error) {
	err = Error("ui.Window.CreateKeyRecorder")
	return
}

func (Window) CreateListView(_ ui.ListViewOptions) (_ ui.ListView, err error) {
	err = Error("ui.Window.CreateListView")
	return
}

func (Window) CreateMarkdown(_ ui.MarkdownOptions) (_ ui.Markdown, err error) {
	err = Error("ui.Window.CreateMarkdown")
	return
}

func (Window) CreateMenu(_ ui.MenuOptions) (_ ui.Menu, err error) {
	err = Error("ui.Window.CreateMenu")
	return
}

func (Window) CreateMenuBar(_ ui.MenuBarOptions) (_ ui.MenuBar, err error) {
	err = Error("ui.Window.CreateMenuBar")
	return
}

func (Window) CreateNavigationList(_ ui.NavigationListOptions) (_ ui.NavigationList, err error) {
	err = Error("ui.Window.CreateNavigationList")
	return
}

func (Window) CreateNumberField(_ ui.NumberFieldOptions) (_ ui.NumberField, err error) {
	err = Error("ui.Window.CreateNumberField")
	return
}

func (Window) CreateOverlayLayout(_ ui.OverlayLayoutOptions) (_ ui.OverlayLayout, err error) {
	err = Error("ui.Window.CreateOverlayLayout")
	return
}

func (Window) CreatePagination(_ ui.PaginationOptions) (_ ui.Pagination, err error) {
	err = Error("ui.Window.CreatePagination")
	return
}

func (Window) CreatePasswordField(_ ui.PasswordFieldOptions) (_ ui.PasswordField, err error) {
	err = Error("ui.Window.CreatePasswordField")
	return
}

func (Window) CreatePopover(_ ui.PopoverOptions) (_ ui.Popover, err error) {
	err = Error("ui.Window.CreatePopover")
	return
}

func (Window) CreateProgressDialog(_ ui.ProgressDialogOptions) (_ ui.ProgressDialog, err error) {
	err = Error("ui.Window.CreateProgressDialog")
	return
}

func (Window) CreatePropertyInspector(_ ui.PropertyInspectorOptions) (_ ui.PropertyInspector, err error) {
	err = Error("ui.Window.CreatePropertyInspector")
	return
}

func (Window) CreateRangeSlider(_ ui.RangeSliderOptions) (_ ui.RangeSlider, err error) {
	err = Error("ui.Window.CreateRangeSlider")
	return
}

func (Window) CreateRichText(_ ui.RichTextOptions) (_ ui.RichText, err error) {
	err = Error("ui.Window.CreateRichText")
	return
}

func (Window) CreateSVG(_ ui.SVGOptions) (_ ui.SVG, err error) {
	err = Error("ui.Window.CreateSVG")
	return
}

func (Window) CreateSearchField(_ ui.SearchFieldOptions) (_ ui.SearchField, err error) {
	err = Error("ui.Window.CreateSearchField")
	return
}

func (Window) CreateSegmentedControl(_ ui.SegmentedControlOptions) (_ ui.SegmentedControl, err error) {
	err = Error("ui.Window.CreateSegmentedControl")
	return
}

func (Window) CreateSplitView(_ ui.SplitViewOptions) (_ ui.SplitView, err error) {
	err = Error("ui.Window.CreateSplitView")
	return
}

func (Window) CreateStatusBar(_ ui.StatusBarOptions) (_ ui.StatusBar, err error) {
	err = Error("ui.Window.CreateStatusBar")
	return
}

func (Window) CreateStepper(_ ui.StepperOptions) (_ ui.Stepper, err error) {
	err = Error("ui.Window.CreateStepper")
	return
}

func (Window) CreateSwitch(_ ui.SwitchOptions) (_ ui.Switch, err error) {
	err = Error("ui.Window.CreateSwitch")
	return
}

func (Window) CreateSystemTray(_ ui.SystemTrayOptions) (_ ui.SystemTray, err error) {
	err = Error("ui.Window.CreateSystemTray")
	return
}

func (Window) CreateTableView(_ ui.TableViewOptions) (_ ui.TableView, err error) {
	err = Error("ui.Window.CreateTableView")
	return
}

func (Window) CreateTimePicker(_ ui.TimePickerOptions) (_ ui.TimePicker, err error) {
	err = Error("ui.Window.CreateTimePicker")
	return
}

func (Window) CreateTitleBarAccessory(_ ui.TitleBarAccessoryOptions) (_ ui.TitleBarAccessory, err error) {
	err = Error("ui.Window.CreateTitleBarAccessory")
	return
}

func (Window) CreateToast(_ ui.ToastOptions) (_ ui.Toast, err error) {
	err = Error("ui.Window.CreateToast")
	return
}

func (Window) CreateToggleButton(_ ui.ToggleButtonOptions) (_ ui.ToggleButton, err error) {
	err = Error("ui.Window.CreateToggleButton")
	return
}

func (Window) CreateToolbar(_ ui.ToolbarOptions) (_ ui.Toolbar, err error) {
	err = Error("ui.Window.CreateToolbar")
	return
}

func (Window) CreateTooltip(_ ui.TooltipOptions) (_ ui.Tooltip, err error) {
	err = Error("ui.Window.CreateTooltip")
	return
}

func (Window) CreateTreeView(_ ui.TreeViewOptions) (_ ui.TreeView, err error) {
	err = Error("ui.Window.CreateTreeView")
	return
}

func (Window) CreateVideo(_ ui.VideoOptions) (_ ui.Video, err error) {
	err = Error("ui.Window.CreateVideo")
	return
}

func (Window) CreateWebView(_ ui.WebViewOptions) (_ ui.WebView, err error) {
	err = Error("ui.Window.CreateWebView")
	return
}

func (Window) FileAssociations() (_ ui.FileAssociations) {
	return
}

func (Window) IsAlwaysOnTop() (_ bool) {
	return
}

func (Window) IsBorderless() (_ bool) {
	return
}

func (Window) IsFullscreen() (_ bool) {
	return
}

func (Window) IsMaximized() (_ bool) {
	return
}

func (Window) IsMinimized() (_ bool) {
	return
}

func (Window) IsResizable() (_ bool) {
	return
}

func (Window) Maximize() (err error) {
	err = Error("ui.Window.Maximize")
	return
}

func (Window) MaximumSize() (_ ui.Size) {
	return
}

func (Window) Minimize() (err error) {
	err = Error("ui.Window.Minimize")
	return
}

func (Window) MinimumSize() (_ ui.Size) {
	return
}

func (Window) Notifications() (_ ui.Notifications) {
	return
}

func (Window) OnActivated(_ ui.FocusHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnActivated")
	return
}

func (Window) OnBlur(_ ui.FocusHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnBlur")
	return
}

func (Window) OnCloseRequest(_ ui.CloseRequestHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnCloseRequest")
	return
}

func (Window) OnDeactivated(_ ui.FocusHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnDeactivated")
	return
}

func (Window) OnDeviceChange(_ ui.DeviceChangeHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnDeviceChange")
	return
}

func (Window) OnFileDrop(_ ui.FileDropHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnFileDrop")
	return
}

func (Window) OnFullscreenChange(_ ui.FullscreenHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnFullscreenChange")
	return
}

func (Window) OnFocus(_ ui.FocusHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnFocus")
	return
}

func (Window) OnGesture(_ ui.GestureHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnGesture")
	return
}

func (Window) OnIME(_ ui.IMEHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnIME")
	return
}

func (Window) OnPen(_ ui.PenHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnPen")
	return
}

func (Window) OnPointerEnter(_ ui.PointerCrossingHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnPointerEnter")
	return
}

func (Window) OnPointerLeave(_ ui.PointerCrossingHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnPointerLeave")
	return
}

func (Window) OnPointerMove(_ ui.PointerMoveHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnPointerMove")
	return
}

func (Window) OnQuitRequest(_ ui.CloseRequestHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnQuitRequest")
	return
}

func (Window) OnReopen(_ ui.FocusHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnReopen")
	return
}

func (Window) OnResize(_ ui.ResizeHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnResize")
	return
}

func (Window) OnSystemPreferencesChange(_ ui.SystemPreferencesHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnSystemPreferencesChange")
	return
}

func (Window) OnThemeChange(_ ui.ThemeChangeHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnThemeChange")
	return
}

func (Window) OnTouch(_ ui.TouchHandler) (_ ui.Unsubscribe) {
	Panic("ui.Window.OnTouch")
	return
}

func (Window) OpenURL(_ context.Context, _ string, _ ui.OpenURLOptions) (err error) {
	err = Error("ui.Window.OpenURL")
	return
}

func (Window) Print(_ context.Context, _ ui.PrintOptions) (_ ui.PrintResult, err error) {
	err = Error("ui.Window.Print")
	return
}

func (Window) RecentDocuments() (_ ui.RecentDocuments) {
	return
}

func (Window) Restore() (err error) {
	err = Error("ui.Window.Restore")
	return
}

func (Window) RevealFile(_ context.Context, _ string, _ ui.RevealFileOptions) (err error) {
	err = Error("ui.Window.RevealFile")
	return
}

func (Window) Screen() (_ ui.Screen) {
	return
}

func (Window) SetAlwaysOnTop(_ bool) {
	Panic("ui.Window.SetAlwaysOnTop")
	return
}

func (Window) SetBorderless(_ bool) {
	Panic("ui.Window.SetBorderless")
	return
}

func (Window) SetFullscreen(_ bool) (err error) {
	err = Error("ui.Window.SetFullscreen")
	return
}

func (Window) SetMaximumSize(_ ui.Size) (err error) {
	err = Error("ui.Window.SetMaximumSize")
	return
}

func (Window) SetMinimumSize(_ ui.Size) (err error) {
	err = Error("ui.Window.SetMinimumSize")
	return
}

func (Window) SetResizable(_ bool) {
	Panic("ui.Window.SetResizable")
	return
}

func (Window) Share(_ context.Context, _ ui.ShareOptions) (_ string, err error) {
	err = Error("ui.Window.Share")
	return
}

func (Window) ShowAbout(_ context.Context, _ ui.AboutOptions) (err error) {
	err = Error("ui.Window.ShowAbout")
	return
}

func (Window) ShowAlert(_ context.Context, _ ui.AlertOptions) (_ string, err error) {
	err = Error("ui.Window.ShowAlert")
	return
}

func (Window) ShowConfirmation(_ context.Context, _ ui.ConfirmationOptions) (_ string, err error) {
	err = Error("ui.Window.ShowConfirmation")
	return
}

func (Window) ShowPrompt(_ context.Context, _ ui.PromptOptions) (_ string, err error) {
	err = Error("ui.Window.ShowPrompt")
	return
}

func (Window) SystemPreferences() (_ ui.SystemPreferences) {
	return
}

// Label implements the planned additions to ui.Label.
type Label struct{}

func (Label) IsSelectable() (_ bool) {
	return
}

func (Label) MaximumLines() (_ int) {
	return
}

func (Label) SemanticStyle() (_ ui.TextStyle) {
	return
}

func (Label) SetMaximumLines(_ int) {
	Panic("ui.Label.SetMaximumLines")
	return
}

func (Label) SetSelectable(_ bool) {
	Panic("ui.Label.SetSelectable")
	return
}

func (Label) SetSemanticStyle(_ ui.TextStyle) {
	Panic("ui.Label.SetSemanticStyle")
	return
}

func (Label) SetTextAlignment(_ ui.TextAlignment) {
	Panic("ui.Label.SetTextAlignment")
	return
}

func (Label) SetTruncation(_ ui.Truncation) {
	Panic("ui.Label.SetTruncation")
	return
}

func (Label) SetWrapMode(_ ui.WrapMode) {
	Panic("ui.Label.SetWrapMode")
	return
}

func (Label) TextAlignment() (_ ui.TextAlignment) {
	return
}

func (Label) Truncation() (_ ui.Truncation) {
	return
}

func (Label) WrapMode() (_ ui.WrapMode) {
	return
}

// Button implements the planned additions to ui.Button.
type Button struct{}

func (Button) Badge() (_ string) {
	return
}

func (Button) Icon() (_ ui.IconSource, _ ui.IconPlacement) {
	return
}

func (Button) IsBusy() (_ bool) {
	return
}

func (Button) Role() (_ ui.ButtonRole) {
	return
}

func (Button) SetBadge(_ string) {
	Panic("ui.Button.SetBadge")
	return
}

func (Button) SetBusy(_ bool) {
	Panic("ui.Button.SetBusy")
	return
}

func (Button) SetIcon(_ ui.IconSource, _ ui.IconPlacement) {
	Panic("ui.Button.SetIcon")
	return
}

func (Button) SetRole(_ ui.ButtonRole) {
	Panic("ui.Button.SetRole")
	return
}

func (Button) SetShortcut(_ ui.Shortcut) {
	Panic("ui.Button.SetShortcut")
	return
}

func (Button) Shortcut() (_ ui.Shortcut) {
	return
}

// TextField implements the planned additions to ui.TextField.
type TextField struct{}

func (TextField) InputPurpose() (_ ui.InputPurpose) {
	return
}

func (TextField) IsReadOnly() (_ bool) {
	return
}

func (TextField) IsSpellCheckEnabled() (_ bool) {
	return
}

func (TextField) MaximumLength() (_ int) {
	return
}

func (TextField) OnSubmit(_ ui.TextChangeHandler) (_ ui.Unsubscribe) {
	Panic("ui.TextField.OnSubmit")
	return
}

func (TextField) Prefix() (_ string) {
	return
}

func (TextField) Selection() (_ ui.TextRange) {
	return
}

func (TextField) SetInputPurpose(_ ui.InputPurpose) {
	Panic("ui.TextField.SetInputPurpose")
	return
}

func (TextField) SetMaximumLength(_ int) {
	Panic("ui.TextField.SetMaximumLength")
	return
}

func (TextField) SetPrefix(_ string) {
	Panic("ui.TextField.SetPrefix")
	return
}

func (TextField) SetReadOnly(_ bool) {
	Panic("ui.TextField.SetReadOnly")
	return
}

func (TextField) SetSelection(_ ui.TextRange) {
	Panic("ui.TextField.SetSelection")
	return
}

func (TextField) SetSpellCheckEnabled(_ bool) {
	Panic("ui.TextField.SetSpellCheckEnabled")
	return
}

func (TextField) SetSuffix(_ string) {
	Panic("ui.TextField.SetSuffix")
	return
}

func (TextField) SetTextAlignment(_ ui.TextAlignment) {
	Panic("ui.TextField.SetTextAlignment")
	return
}

func (TextField) Suffix() (_ string) {
	return
}

func (TextField) TextAlignment() (_ ui.TextAlignment) {
	return
}

// TextArea implements the planned additions to ui.TextArea.
type TextArea struct{}

func (TextArea) IsSpellCheckEnabled() (_ bool) {
	return
}

func (TextArea) MaximumLength() (_ int) {
	return
}

func (TextArea) ScrollPosition() (_ ui.Point) {
	return
}

func (TextArea) Selection() (_ ui.TextRange) {
	return
}

func (TextArea) SetMaximumLength(_ int) {
	Panic("ui.TextArea.SetMaximumLength")
	return
}

func (TextArea) SetScrollPosition(_ ui.Point) {
	Panic("ui.TextArea.SetScrollPosition")
	return
}

func (TextArea) SetSelection(_ ui.TextRange) {
	Panic("ui.TextArea.SetSelection")
	return
}

func (TextArea) SetSpellCheckEnabled(_ bool) {
	Panic("ui.TextArea.SetSpellCheckEnabled")
	return
}

func (TextArea) SetTabPolicy(_ ui.TabPolicy) {
	Panic("ui.TextArea.SetTabPolicy")
	return
}

func (TextArea) SetWrapMode(_ ui.WrapMode) {
	Panic("ui.TextArea.SetWrapMode")
	return
}

func (TextArea) TabPolicy() (_ ui.TabPolicy) {
	return
}

func (TextArea) WrapMode() (_ ui.WrapMode) {
	return
}

// CheckBox implements the planned additions to ui.CheckBox.
type CheckBox struct{}

func (CheckBox) AllowsMixedState() (_ bool) {
	return
}

func (CheckBox) Description() (_ string) {
	return
}

func (CheckBox) LabelPlacement() (_ ui.LabelPlacement) {
	return
}

func (CheckBox) SetAllowsMixedState(_ bool) {
	Panic("ui.CheckBox.SetAllowsMixedState")
	return
}

func (CheckBox) SetDescription(_ string) {
	Panic("ui.CheckBox.SetDescription")
	return
}

func (CheckBox) SetLabelPlacement(_ ui.LabelPlacement) {
	Panic("ui.CheckBox.SetLabelPlacement")
	return
}

func (CheckBox) SetState(_ ui.CheckState) {
	Panic("ui.CheckBox.SetState")
	return
}

func (CheckBox) State() (_ ui.CheckState) {
	return
}

// RadioGroup implements the planned additions to ui.RadioGroup.
type RadioGroup struct{}

func (RadioGroup) Choices() (_ []ui.Choice) {
	return
}

func (RadioGroup) Label() (_ string) {
	return
}

func (RadioGroup) Orientation() (_ ui.LayoutDirection) {
	return
}

func (RadioGroup) SetChoices(_ []ui.Choice) (err error) {
	err = Error("ui.RadioGroup.SetChoices")
	return
}

func (RadioGroup) SetLabel(_ string) {
	Panic("ui.RadioGroup.SetLabel")
	return
}

func (RadioGroup) SetOrientation(_ ui.LayoutDirection) {
	Panic("ui.RadioGroup.SetOrientation")
	return
}

// Select implements the planned additions to ui.Select.
type Select struct{}

func (Select) Choices() (_ []ui.Choice) {
	return
}

func (Select) Placeholder() (_ string) {
	return
}

func (Select) PopupWidth() (_ int) {
	return
}

func (Select) SetChoices(_ []ui.Choice) (err error) {
	err = Error("ui.Select.SetChoices")
	return
}

func (Select) SetPlaceholder(_ string) {
	Panic("ui.Select.SetPlaceholder")
	return
}

func (Select) SetPopupWidth(_ int) {
	Panic("ui.Select.SetPopupWidth")
	return
}

func (Select) SetVisibleRowCount(_ int) {
	Panic("ui.Select.SetVisibleRowCount")
	return
}

func (Select) VisibleRowCount() (_ int) {
	return
}

// Slider implements the planned additions to ui.Slider.
type Slider struct{}

func (Slider) IsInverted() (_ bool) {
	return
}

func (Slider) Maximum() (_ float64) {
	return
}

func (Slider) Minimum() (_ float64) {
	return
}

func (Slider) Orientation() (_ ui.LayoutDirection) {
	return
}

func (Slider) SetInverted(_ bool) {
	Panic("ui.Slider.SetInverted")
	return
}

func (Slider) SetOrientation(_ ui.LayoutDirection) {
	Panic("ui.Slider.SetOrientation")
	return
}

func (Slider) SetTicks(_ ui.SliderTicks) {
	Panic("ui.Slider.SetTicks")
	return
}

func (Slider) Step() (_ float64) {
	return
}

func (Slider) Ticks() (_ ui.SliderTicks) {
	return
}

// ProgressBar implements the planned additions to ui.ProgressBar.
type ProgressBar struct{}

func (ProgressBar) Mode() (_ ui.ProgressMode) {
	return
}

func (ProgressBar) Orientation() (_ ui.LayoutDirection) {
	return
}

func (ProgressBar) SetMode(_ ui.ProgressMode) {
	Panic("ui.ProgressBar.SetMode")
	return
}

func (ProgressBar) SetOrientation(_ ui.LayoutDirection) {
	Panic("ui.ProgressBar.SetOrientation")
	return
}

func (ProgressBar) SetState(_ ui.ProgressState) {
	Panic("ui.ProgressBar.SetState")
	return
}

func (ProgressBar) SetText(_ string) {
	Panic("ui.ProgressBar.SetText")
	return
}

func (ProgressBar) State() (_ ui.ProgressState) {
	return
}

func (ProgressBar) Text() (_ string) {
	return
}

// ActivityIndicator implements the planned additions to ui.ActivityIndicator.
type ActivityIndicator struct{}

func (ActivityIndicator) ControlSize() (_ ui.ControlSize) {
	return
}

func (ActivityIndicator) Label() (_ string) {
	return
}

func (ActivityIndicator) SetControlSize(_ ui.ControlSize) {
	Panic("ui.ActivityIndicator.SetControlSize")
	return
}

func (ActivityIndicator) SetLabel(_ string) {
	Panic("ui.ActivityIndicator.SetLabel")
	return
}

// Separator implements the planned additions to ui.Separator.
type Separator struct{}

func (Separator) Label() (_ string) {
	return
}

func (Separator) Presentation() (_ ui.SeparatorPresentation) {
	return
}

func (Separator) SetDirection(_ ui.LayoutDirection) {
	Panic("ui.Separator.SetDirection")
	return
}

func (Separator) SetLabel(_ string) {
	Panic("ui.Separator.SetLabel")
	return
}

func (Separator) SetPresentation(_ ui.SeparatorPresentation) {
	Panic("ui.Separator.SetPresentation")
	return
}

func (Separator) SetThickness(_ int) {
	Panic("ui.Separator.SetThickness")
	return
}

func (Separator) Thickness() (_ int) {
	return
}

// ImageView implements the planned additions to ui.ImageView.
type ImageView struct{}

func (ImageView) Alignment() (_ ui.Anchor) {
	return
}

func (ImageView) AlternativeText() (_ string) {
	return
}

func (ImageView) Interpolation() (_ ui.Interpolation) {
	return
}

func (ImageView) SetAlignment(_ ui.Anchor) {
	Panic("ui.ImageView.SetAlignment")
	return
}

func (ImageView) SetAlternativeText(_ string) {
	Panic("ui.ImageView.SetAlternativeText")
	return
}

func (ImageView) SetInterpolation(_ ui.Interpolation) {
	Panic("ui.ImageView.SetInterpolation")
	return
}

func (ImageView) SetSourceScale(_ float64) {
	Panic("ui.ImageView.SetSourceScale")
	return
}

func (ImageView) SourceScale() (_ float64) {
	return
}

// Link implements the planned additions to ui.Link.
type Link struct{}

func (Link) Icon() (_ ui.IconSource) {
	return
}

func (Link) IsVisited() (_ bool) {
	return
}

func (Link) OpenPolicy() (_ ui.LinkOpenPolicy) {
	return
}

func (Link) SetIcon(_ ui.IconSource) {
	Panic("ui.Link.SetIcon")
	return
}

func (Link) SetOpenPolicy(_ ui.LinkOpenPolicy) {
	Panic("ui.Link.SetOpenPolicy")
	return
}

func (Link) SetVisited(_ bool) {
	Panic("ui.Link.SetVisited")
	return
}

// Tabs implements the planned additions to ui.Tabs.
type Tabs struct{}

func (Tabs) OverflowPolicy() (_ ui.TabOverflowPolicy) {
	return
}

func (Tabs) Placement() (_ ui.TabPlacement) {
	return
}

func (Tabs) SetOverflowPolicy(_ ui.TabOverflowPolicy) {
	Panic("ui.Tabs.SetOverflowPolicy")
	return
}

func (Tabs) SetPlacement(_ ui.TabPlacement) {
	Panic("ui.Tabs.SetPlacement")
	return
}

// ScrollView implements the planned additions to ui.ScrollView.
type ScrollView struct{}

func (ScrollView) ContentInsets() (_ ui.Insets) {
	return
}

func (ScrollView) OverscrollPolicy() (_ ui.OverscrollPolicy) {
	return
}

func (ScrollView) ScrollBehavior() (_ ui.ScrollBehavior) {
	return
}

func (ScrollView) ScrollToWidget(_ ui.Widget) (err error) {
	err = Error("ui.ScrollView.ScrollToWidget")
	return
}

func (ScrollView) ScrollbarPolicy() (_ ui.ScrollbarPolicy) {
	return
}

func (ScrollView) SetContentInsets(_ ui.Insets) {
	Panic("ui.ScrollView.SetContentInsets")
	return
}

func (ScrollView) SetOverscrollPolicy(_ ui.OverscrollPolicy) {
	Panic("ui.ScrollView.SetOverscrollPolicy")
	return
}

func (ScrollView) SetScrollBehavior(_ ui.ScrollBehavior) {
	Panic("ui.ScrollView.SetScrollBehavior")
	return
}

func (ScrollView) SetScrollbarPolicy(_ ui.ScrollbarPolicy) {
	Panic("ui.ScrollView.SetScrollbarPolicy")
	return
}

func (ScrollView) SetZoom(_ float64) (err error) {
	err = Error("ui.ScrollView.SetZoom")
	return
}

func (ScrollView) ViewportSize() (_ ui.Size) {
	return
}

func (ScrollView) Zoom() (_ float64) {
	return
}

// Canvas implements the planned additions to ui.Canvas.
type Canvas struct{}

func (Canvas) CapturePointer() (err error) {
	err = Error("ui.Canvas.CapturePointer")
	return
}

func (Canvas) ColorSpace() (_ ui.ColorSpace) {
	return
}

func (Canvas) HasPointerCapture() (_ bool) {
	return
}

func (Canvas) OnFrameRequest(_ ui.FrameRequestHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnFrameRequest")
	return
}

func (Canvas) OnKey(_ ui.KeyHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnKey")
	return
}

func (Canvas) OnOcclusionChange(_ ui.OcclusionHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnOcclusionChange")
	return
}

func (Canvas) OnPointerEnter(_ ui.PointerCrossingHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnPointerEnter")
	return
}

func (Canvas) OnPointerLeave(_ ui.PointerCrossingHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnPointerLeave")
	return
}

func (Canvas) OnPointerMove(_ ui.PointerMoveHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnPointerMove")
	return
}

func (Canvas) OnSurfaceChange(_ ui.SurfaceHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnSurfaceChange")
	return
}

func (Canvas) OnText(_ ui.TextHandler) (_ ui.Unsubscribe) {
	Panic("ui.Canvas.OnText")
	return
}

func (Canvas) ReleasePointer() {
	Panic("ui.Canvas.ReleasePointer")
	return
}

// Section implements the planned additions to ui.Section.
type Section struct{}

func (Section) Badge() (_ string) {
	return
}

func (Section) Icon() (_ ui.IconSource) {
	return
}

func (Section) IsAnimationEnabled() (_ bool) {
	return
}

func (Section) SetAnimationEnabled(_ bool) {
	Panic("ui.Section.SetAnimationEnabled")
	return
}

func (Section) SetBadge(_ string) {
	Panic("ui.Section.SetBadge")
	return
}

func (Section) SetIcon(_ ui.IconSource) {
	Panic("ui.Section.SetIcon")
	return
}

func (Section) SetSubtitle(_ string) {
	Panic("ui.Section.SetSubtitle")
	return
}

func (Section) Subtitle() (_ string) {
	return
}
