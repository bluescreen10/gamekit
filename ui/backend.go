package ui

import (
	"context"
	"errors"
	"image/color"

	"github.com/bluescreen10/gamekit/gpu"
)

// Theme controls the native appearance used by a window and its controls.
type Theme uint8

const (
	// ThemeSystem follows the appearance that the user selects for the
	// system.
	ThemeSystem Theme = iota

	// ThemeLight uses the light appearance.
	ThemeLight

	// ThemeDark uses the dark appearance.
	ThemeDark
)

// Validate checks that the theme is recognized.
func (theme Theme) Validate() error {
	if theme > ThemeDark {
		return errors.New("ui: invalid theme")
	}
	return nil
}

// WindowKind describes a window's role in the application.
type WindowKind uint8

const (
	// WindowNormal is an independent application window.
	WindowNormal WindowKind = iota

	// WindowUtility is an owned tool window that stays above its owner and does
	// not appear as a separate application window.
	WindowUtility
)

// Validate checks that the window kind is recognized.
func (kind WindowKind) Validate() error {
	if kind > WindowUtility {
		return errors.New("ui: invalid window kind")
	}
	return nil
}

// WindowOptions describes a native application or utility window. Size is the
// content area in logical pixels. Utility windows require an owner.
type WindowOptions struct {
	// Title is the text in the title bar and in the window lists of the
	// system.
	Title string

	// Size is the size of the content area in logical pixels.
	Size Size

	// Kind is the role of the window. The zero value is WindowNormal.
	Kind WindowKind

	// Owner is the window that owns a utility window. It must be nil for a
	// normal window.
	Owner Window

	// Position is relative to the owner's content area for utility windows.
	Position Point

	// Resizable lets the user change the size of the window.
	Resizable bool

	// Hidden creates the window without showing it. Call SetVisible to show
	// it.
	Hidden bool

	// Borderless removes the title bar and the frame.
	Borderless bool

	// Maximized makes the window fill the work area of its screen.
	Maximized bool

	// AlwaysOnTop keeps the window above the normal windows.
	AlwaysOnTop bool

	// TransparentFramebuffer lets the alpha channel of the framebuffer show
	// the content behind the window.
	TransparentFramebuffer bool

	// Theme is the appearance of the window and its controls.
	Theme Theme

	// Fullscreen requests fullscreen on the window's screen. Like
	// SetFullscreen, the request completes asynchronously; a hidden window
	// enters fullscreen when it is first shown.
	Fullscreen bool

	// MinimumSize is the smallest content size. A zero dimension has no
	// limit.
	MinimumSize Size

	// MaximumSize is the largest content size. A zero dimension has no
	// limit.
	MaximumSize Size
}

// Validate checks that the window options describe a usable native window.
func (options WindowOptions) Validate() error {
	if !options.Size.IsValid() {
		return errors.New("ui: window size must be positive")
	}
	if err := options.Kind.Validate(); err != nil {
		return err
	}
	if err := options.Theme.Validate(); err != nil {
		return err
	}
	if options.Kind == WindowUtility && options.Owner == nil {
		return errors.New("ui: utility window requires an owner")
	}
	if options.Kind == WindowNormal && options.Owner != nil {
		return errors.New("ui: normal window cannot have an owner")
	}
	if err := validateSizeConstraints(options.MinimumSize, options.MaximumSize); err != nil {
		return err
	}
	if options.Fullscreen && options.Kind == WindowUtility {
		return errors.New("ui: utility window cannot be fullscreen")
	}
	return nil
}

func validateSizeConstraints(minimum, maximum Size) error {
	if minimum.Width < 0 || minimum.Height < 0 || maximum.Width < 0 || maximum.Height < 0 {
		return errors.New("ui: size constraints must be non-negative")
	}
	if maximum.Width > 0 && maximum.Width < minimum.Width ||
		maximum.Height > 0 && maximum.Height < minimum.Height {
		return errors.New("ui: maximum size is smaller than minimum size")
	}
	return nil
}

// ValidateSizeConstraints checks a minimum and maximum size pair. Zero
// dimensions mean unconstrained.
func ValidateSizeConstraints(minimum, maximum Size) error {
	return validateSizeConstraints(minimum, maximum)
}

// Screen describes the display that shows a window. Bounds and WorkArea use
// global logical coordinates; WorkArea excludes menu bars, docks, and task
// bars. RefreshRate is in hertz.
type Screen struct {
	// Name is the name of the display that the system shows to the user.
	Name string

	// Bounds is the full area of the display.
	Bounds Rect

	// WorkArea is the part of Bounds that windows can use.
	WorkArea Rect

	// ScaleFactor is the number of physical pixels in one logical pixel.
	ScaleFactor float64

	// DPI is the number of physical pixels in one inch.
	DPI float64

	// RefreshRate is the number of display refreshes in one second.
	RefreshRate float64
}

// SystemPreferences are user-level appearance and accessibility settings.
// ResolvedTheme is always ThemeLight or ThemeDark. TextScale is one for the
// default text size. Locale is a BCP 47 tag.
type SystemPreferences struct {
	// ResolvedTheme is the appearance that the system uses now.
	ResolvedTheme Theme

	// TextScale is the size of text relative to the default size.
	TextScale float64

	// HighContrast is set when the user asks for more contrast.
	HighContrast bool

	// ReducedMotion is set when the user asks for less animation.
	ReducedMotion bool

	// ReducedTransparency is set when the user asks for opaque surfaces.
	ReducedTransparency bool

	// AccentColor is the color that the user selects for controls.
	AccentColor color.RGBA

	// Locale is the language and region of the user.
	Locale string

	// RightToLeft is set when the user's language reads from right to left.
	RightToLeft bool
}

// CloseRequestHandler decides whether a user close request proceeds. Returning
// false keeps the window open.
type CloseRequestHandler func() bool

// FullscreenHandler receives a window's fullscreen state after it changes.
type FullscreenHandler func(fullscreen bool)

// ThemeChangeHandler receives the newly resolved theme.
type ThemeChangeHandler func(theme Theme)

// FileDropHandler receives paths dropped onto a window.
type FileDropHandler func(paths []string)

// SystemPreferencesHandler receives changed system preferences.
type SystemPreferencesHandler func(preferences SystemPreferences)

// Backend creates native windows.
type Backend interface {
	// CreateWindow validates options and opens a native window. It must be
	// called from the platform UI thread.
	CreateWindow(options WindowOptions) (Window, error)
}

// Drawable is a native UI object that can be presented by a GPU backend.
// FramebufferSize is measured in physical pixels, unlike widget geometry.
type Drawable interface {
	gpu.SurfaceTarget

	// FramebufferSize returns the size of the drawable area in physical
	// pixels.
	FramebufferSize() Size
}

// Window is a native window and the owner of everything that it creates:
// widgets, menus, toolbars, popovers, toasts, and dialogs. Close closes these
// objects too. Closing an owner also closes its utility windows. A utility
// window's native close control hides it and sets ShouldClose; clear that
// request before showing the window again. Close must be safe to call
// repeatedly. PollEvents and methods that create or display native UI must be
// called from the platform UI thread. On macOS that is the program's main
// thread.
//
// The dialog methods return context.Canceled or context.DeadlineExceeded
// without showing UI when ctx is already done, and ErrCancelled when the user
// dismisses the dialog.
//
// The application methods, such as Clipboard and OnQuitRequest, reach
// platform services through the window. Application events go to the
// subscribers of every window.
type Window interface {
	Drawable
	Input

	// Close releases the window and every object that it created. The system
	// tray item is an exception: it stays until its own Close.
	Close()

	// Kind returns the role of the window.
	Kind() WindowKind

	// Title returns the text in the title bar.
	Title() string

	// SetTitle changes the text in the title bar.
	SetTitle(title string)

	// Theme returns the appearance that the window requests.
	Theme() Theme

	// SetTheme changes the appearance of the window and its controls.
	SetTheme(theme Theme) error

	// Size returns the size of the content area in logical pixels.
	Size() Size

	// SetSize changes the size of the content area. The minimum and maximum
	// sizes limit the result.
	SetSize(size Size) error

	// Position returns the top-left corner of the window in global logical
	// coordinates.
	Position() Point

	// SetPosition moves the top-left corner of the window.
	SetPosition(position Point)

	// IsVisible reports whether the window is shown.
	IsVisible() bool

	// SetVisible shows or hides the window.
	SetVisible(visible bool)

	// BringToFront puts a visible window above the other windows of the
	// application. It does not change the keyboard focus.
	BringToFront()

	// OnMove receives the new position after the window moves.
	OnMove(handler MoveHandler) Unsubscribe

	// MinimumSize returns the smallest content size. A zero dimension has no
	// limit.
	MinimumSize() Size

	// SetMinimumSize changes the smallest content size. It returns an error
	// when size is larger than the maximum size.
	SetMinimumSize(size Size) error

	// MaximumSize returns the largest content size. A zero dimension has no
	// limit.
	MaximumSize() Size

	// SetMaximumSize changes the largest content size. It returns an error
	// when size is smaller than the minimum size.
	SetMaximumSize(size Size) error

	// IsResizable reports whether the user can change the size of the
	// window.
	IsResizable() bool

	// SetResizable lets the user change the size of the window, or stops
	// the user.
	SetResizable(resizable bool)

	// IsBorderless reports whether the window has no title bar and no
	// frame.
	IsBorderless() bool

	// SetBorderless removes or adds the title bar and the frame.
	SetBorderless(borderless bool)

	// IsAlwaysOnTop reports whether the window stays above the normal
	// windows.
	IsAlwaysOnTop() bool

	// SetAlwaysOnTop keeps the window above the normal windows, or lets
	// other windows cover it.
	SetAlwaysOnTop(alwaysOnTop bool)

	// Minimize hides the window in the Dock or the task bar. It returns an
	// error when the platform refuses, for example for a fullscreen window.
	Minimize() error

	// Maximize makes the window fill the work area of its screen.
	Maximize() error

	// Restore leaves the minimized, maximized, and fullscreen states.
	Restore() error

	// IsMinimized reports whether the window is minimized.
	IsMinimized() bool

	// IsMaximized reports whether the window fills the work area.
	IsMaximized() bool

	// IsFullscreen reports the current state, which changes only when a
	// transition completes.
	IsFullscreen() bool

	// SetFullscreen requests the platform's fullscreen mode. The transition
	// may be animated and completes asynchronously: watch OnFullscreenChange
	// or IsFullscreen while polling events. A request made while the window is
	// hidden is applied when the window is shown. It returns ErrUnavailable
	// when the platform cannot change fullscreen state now, such as while the
	// display is locked or asleep. A failed transition leaves the state as it
	// was and sends no event.
	SetFullscreen(fullscreen bool) error

	// OnFullscreenChange receives the new state after each completed
	// transition, whether requested by the application or the user.
	OnFullscreenChange(handler FullscreenHandler) Unsubscribe

	// Screen returns the display that shows most of the window.
	Screen() Screen

	// OnResize receives content-size changes from the user or the system.
	OnResize(handler ResizeHandler) Unsubscribe

	// OnCloseRequest handlers run when the user asks to close the window. If
	// any handler returns false, ShouldClose is not set.
	OnCloseRequest(handler CloseRequestHandler) Unsubscribe

	// OnFocus receives the window becoming the keyboard focus window.
	OnFocus(handler FocusHandler) Unsubscribe

	// OnBlur receives the window losing keyboard focus.
	OnBlur(handler FocusHandler) Unsubscribe

	// OnThemeChange receives the resolved theme after it changes.
	OnThemeChange(handler ThemeChangeHandler) Unsubscribe

	// OnFileDrop receives the paths of files that the user drops on the
	// window.
	OnFileDrop(handler FileDropHandler) Unsubscribe

	// SystemPreferences returns the current user preferences.
	SystemPreferences() SystemPreferences

	// OnSystemPreferencesChange receives the preferences after one of them
	// changes.
	OnSystemPreferencesChange(handler SystemPreferencesHandler) Unsubscribe

	// AccessibilityTree returns the accessibility elements of the window, as
	// assistive technologies see them.
	AccessibilityTree() AccessibilityNode

	// ShouldClose reports whether the user asked to close the window and no
	// close request handler refused.
	ShouldClose() bool

	// SetShouldClose sets or clears the close request.
	SetShouldClose(close bool)

	// PollEvents processes the pending events of the platform and runs the
	// handlers for them. It does not wait for new events.
	PollEvents()

	// SetContent makes content fill the content area of the window. The
	// previous content stays open but becomes hidden.
	SetContent(content Widget) error

	// CreateLayout creates a layout that arranges its children by spec.
	CreateLayout(spec LayoutSpec) (Layout, error)

	// CreateLabel creates a label that shows text.
	CreateLabel(text string) (Label, error)

	// CreateButton creates a push button with text.
	CreateButton(text string) (Button, error)

	// CreateTextField creates a single-line text editor.
	CreateTextField() (TextField, error)

	// CreateTextArea creates a multi-line text editor.
	CreateTextArea(options TextAreaOptions) (TextArea, error)

	// CreateCheckBox creates a check box with text.
	CreateCheckBox(text string, checked bool) (CheckBox, error)

	// CreateRadioGroup creates a group of mutually exclusive choices.
	CreateRadioGroup(options RadioGroupOptions) (RadioGroup, error)

	// CreateSelect creates a pop-up list of choices.
	CreateSelect(options SelectOptions) (Select, error)

	// CreateSlider creates a slider for a value in a range.
	CreateSlider(options SliderOptions) (Slider, error)

	// CreateProgressBar creates a progress bar. A value from zero to one
	// shows progress. A negative value shows indeterminate progress.
	CreateProgressBar(value float64) (ProgressBar, error)

	// CreateActivityIndicator creates a spinning activity indicator.
	CreateActivityIndicator(running bool) (ActivityIndicator, error)

	// CreateSeparator creates a line that separates other widgets.
	CreateSeparator(direction LayoutDirection) (Separator, error)

	// CreateImage creates a view that shows an image.
	CreateImage(options ImageOptions) (ImageView, error)

	// CreateLink creates a text link to url.
	CreateLink(text, url string) (Link, error)

	// CreateTabs creates a tab view with one page for each item.
	CreateTabs(options TabsOptions) (Tabs, error)

	// CreateScrollView creates a viewport that scrolls its content.
	CreateScrollView(options ScrollViewOptions) (ScrollView, error)

	// CreateCanvas creates a widget that a GPU backend draws into.
	CreateCanvas() (Canvas, error)

	// CreateSection creates a section that the user can expand and collapse.
	CreateSection(options SectionOptions) (Section, error)

	// CreateToggleButton creates a button that stays pressed until the user
	// clicks it again.
	CreateToggleButton(options ToggleButtonOptions) (ToggleButton, error)

	// CreateSwitch creates an on and off switch.
	CreateSwitch(options SwitchOptions) (Switch, error)

	// CreatePasswordField creates a text field that hides its text.
	CreatePasswordField(options PasswordFieldOptions) (PasswordField, error)

	// CreateSearchField creates a text field for search terms.
	CreateSearchField(options SearchFieldOptions) (SearchField, error)

	// CreateNumberField creates a text field for a number in a range.
	CreateNumberField(options NumberFieldOptions) (NumberField, error)

	// CreateEditableCombo creates a text field with a list of suggestions.
	CreateEditableCombo(options EditableComboOptions) (EditableCombo, error)

	// CreateSegmentedControl creates a row of connected buttons.
	CreateSegmentedControl(options SegmentedControlOptions) (SegmentedControl, error)

	// CreateStepper creates a pair of buttons that increase and decrease a
	// value.
	CreateStepper(options StepperOptions) (Stepper, error)

	// CreateRangeSlider creates a slider with two thumbs for a range.
	CreateRangeSlider(options RangeSliderOptions) (RangeSlider, error)

	// CreateDatePicker creates a control for a date.
	CreateDatePicker(options DatePickerOptions) (DatePicker, error)

	// CreateTimePicker creates a control for a time of day.
	CreateTimePicker(options TimePickerOptions) (TimePicker, error)

	// CreateColorPicker creates a control that opens the color panel of the
	// platform.
	CreateColorPicker(options ColorPickerOptions) (ColorPicker, error)

	// CreateFontPicker creates a control that opens the font panel of the
	// platform.
	CreateFontPicker(options FontPickerOptions) (FontPicker, error)

	// CreateCalendar creates a month calendar.
	CreateCalendar(options CalendarOptions) (Calendar, error)

	// CreateKeyRecorder creates a control that records a keyboard shortcut.
	CreateKeyRecorder(options KeyRecorderOptions) (KeyRecorder, error)

	// CreateDial creates a circular control for a value.
	CreateDial(options DialOptions) (Dial, error)

	// CreateAccordion creates a stack of sections that expand and collapse.
	CreateAccordion(options AccordionOptions) (Accordion, error)

	// CreateSplitView creates panes with dividers that the user can drag.
	CreateSplitView(options SplitViewOptions) (SplitView, error)

	// CreateGroupBox creates a titled frame around content.
	CreateGroupBox(options GroupBoxOptions) (GroupBox, error)

	// CreatePopover creates a popover that points to an anchor widget.
	CreatePopover(options PopoverOptions) (Popover, error)

	// CreateDrawer creates a panel that slides from an edge of the window.
	CreateDrawer(options DrawerOptions) (Drawer, error)

	// CreateGridLayout creates a layout of rows and columns.
	CreateGridLayout(options GridLayoutOptions) (GridLayout, error)

	// CreateFlowLayout creates a layout that wraps its children in lines.
	CreateFlowLayout(options FlowLayoutOptions) (FlowLayout, error)

	// CreateOverlayLayout creates a layout that stacks its children on top
	// of each other.
	CreateOverlayLayout(options OverlayLayoutOptions) (OverlayLayout, error)

	// CreateFormLayout creates a layout of labeled fields.
	CreateFormLayout(options FormLayoutOptions) (FormLayout, error)

	// CreateDockingLayout creates a layout of panes in dock areas.
	CreateDockingLayout(options DockingLayoutOptions) (DockingLayout, error)

	// CreateListView creates a list of items.
	CreateListView(options ListViewOptions) (ListView, error)

	// CreateTableView creates a table of rows and columns.
	CreateTableView(options TableViewOptions) (TableView, error)

	// CreateTreeView creates a hierarchy of items that expand and collapse.
	CreateTreeView(options TreeViewOptions) (TreeView, error)

	// CreateCollectionView creates a grid of items.
	CreateCollectionView(options CollectionViewOptions) (CollectionView, error)

	// CreateNavigationList creates a sidebar list of destinations.
	CreateNavigationList(options NavigationListOptions) (NavigationList, error)

	// CreateBreadcrumbs creates a path of links to parent locations.
	CreateBreadcrumbs(options BreadcrumbsOptions) (Breadcrumbs, error)

	// CreatePagination creates a control that selects a page.
	CreatePagination(options PaginationOptions) (Pagination, error)

	// CreateCommandPalette creates a searchable list of commands.
	CreateCommandPalette(options CommandPaletteOptions) (CommandPalette, error)

	// CreatePropertyInspector creates a list of editable properties.
	CreatePropertyInspector(options PropertyInspectorOptions) (PropertyInspector, error)

	// CreateMenu creates a menu that a menu bar or a system tray item can
	// show.
	CreateMenu(options MenuOptions) (Menu, error)

	// CreateMenuBar creates the menu bar of the window. On macOS, the
	// application menu bar shows it while the window has keyboard focus.
	CreateMenuBar(options MenuBarOptions) (MenuBar, error)

	// CreateContextMenu creates a menu that opens at a position in a widget.
	CreateContextMenu(options ContextMenuOptions) (ContextMenu, error)

	// CreateToolbar creates the toolbar of the window.
	CreateToolbar(options ToolbarOptions) (Toolbar, error)

	// CreateStatusBar creates a bar that shows status text.
	CreateStatusBar(options StatusBarOptions) (StatusBar, error)

	// CreateCommandBar creates a bar of action buttons.
	CreateCommandBar(options CommandBarOptions) (CommandBar, error)

	// CreateTitleBarAccessory creates a widget area in the title bar.
	CreateTitleBarAccessory(options TitleBarAccessoryOptions) (TitleBarAccessory, error)

	// CreateInfoBar creates a bar that shows a message and actions.
	CreateInfoBar(options InfoBarOptions) (InfoBar, error)

	// CreateTooltip creates a tooltip that can show at a widget.
	CreateTooltip(options TooltipOptions) (Tooltip, error)

	// CreateToast creates a short message that hides after a time.
	CreateToast(options ToastOptions) (Toast, error)

	// CreateBadge creates a small count or label.
	CreateBadge(options BadgeOptions) (Badge, error)

	// CreateProgressDialog creates a dialog that shows the progress of an
	// operation.
	CreateProgressDialog(options ProgressDialogOptions) (ProgressDialog, error)

	// CreateEmptyState creates a placeholder for a view that has no content.
	CreateEmptyState(options EmptyStateOptions) (EmptyState, error)

	// CreateInlineValidation creates a message about the value of a field.
	CreateInlineValidation(options InlineValidationOptions) (InlineValidation, error)

	// CreateIcon creates a view that shows an icon.
	CreateIcon(options IconOptions) (Icon, error)

	// CreateRichText creates a view of styled text.
	CreateRichText(options RichTextOptions) (RichText, error)

	// CreateMarkdown creates a view that renders Markdown.
	CreateMarkdown(options MarkdownOptions) (Markdown, error)

	// CreateSVG creates a view that renders an SVG document.
	CreateSVG(options SVGOptions) (SVG, error)

	// CreateAnimatedImage creates a view that plays a sequence of frames.
	CreateAnimatedImage(options AnimatedImageOptions) (AnimatedImage, error)

	// CreateVideo creates a video player.
	CreateVideo(options VideoOptions) (Video, error)

	// CreateWebView creates a view that shows web content.
	CreateWebView(options WebViewOptions) (WebView, error)

	// OpenFile shows the open panel and returns the path of the chosen file.
	OpenFile(ctx context.Context, options OpenFileOptions) (string, error)

	// OpenFiles shows the open panel and returns the paths of the chosen
	// files.
	OpenFiles(ctx context.Context, options OpenFileOptions) ([]string, error)

	// SaveFile shows the save panel and returns the chosen path.
	SaveFile(ctx context.Context, options SaveFileOptions) (string, error)

	// SelectDirectory shows the open panel for directories and returns the
	// chosen path.
	SelectDirectory(ctx context.Context, options DirectoryDialogOptions) (string, error)

	// ShowAlert returns the acknowledgement action ID.
	ShowAlert(ctx context.Context, options AlertOptions) (string, error)

	// ShowConfirmation returns the chosen action ID, which is CancelAction
	// when the user cancels.
	ShowConfirmation(ctx context.Context, options ConfirmationOptions) (string, error)

	// ShowPrompt returns the entered text.
	ShowPrompt(ctx context.Context, options PromptOptions) (string, error)

	// ShowAbout shows the about panel of the application.
	ShowAbout(ctx context.Context, options AboutOptions) error

	// Clipboard returns the system clipboard.
	Clipboard() Clipboard

	// Notifications returns the system notifications of the application.
	Notifications() Notifications

	// CreateSystemTray creates an item in the menu bar extras on macOS, or
	// in the notification area on other platforms.
	CreateSystemTray(options SystemTrayOptions) (SystemTray, error)

	// RecentDocuments returns the list of recent documents of the
	// application.
	RecentDocuments() RecentDocuments

	// FileAssociations returns the applications that open file types.
	FileAssociations() FileAssociations

	// OpenURL opens url with its default handler.
	OpenURL(ctx context.Context, url string, options OpenURLOptions) error

	// RevealFile shows path in the platform file manager.
	RevealFile(ctx context.Context, path string, options RevealFileOptions) error

	// Print prints the pages that options renders. It shows the print panel
	// first when options.ShowDialog is set.
	Print(ctx context.Context, options PrintOptions) (PrintResult, error)

	// Share presents the platform share UI and returns the chosen service.
	Share(ctx context.Context, options ShareOptions) (string, error)

	// OnActivated reports the application becoming active.
	OnActivated(handler FocusHandler) Unsubscribe

	// OnDeactivated reports the application becoming inactive.
	OnDeactivated(handler FocusHandler) Unsubscribe

	// OnReopen reports the user re-launching a running application, such as
	// clicking its Dock icon on macOS.
	OnReopen(handler FocusHandler) Unsubscribe

	// OnQuitRequest handlers run when the user asks to quit. If any handler
	// returns false, the application keeps running.
	OnQuitRequest(handler CloseRequestHandler) Unsubscribe
}
