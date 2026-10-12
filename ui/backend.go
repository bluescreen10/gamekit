package ui

import (
	"context"
	"errors"

	"github.com/bluescreen10/gamekit/gpu"
)

// Theme controls the native appearance used by a window and its controls.
type Theme uint8

const (
	ThemeSystem Theme = iota
	ThemeLight
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
	Title string
	Size  Size
	Kind  WindowKind
	Owner Window

	// Position is relative to the owner's content area for utility windows.
	Position Point

	Resizable              bool
	Hidden                 bool
	Borderless             bool
	Maximized              bool
	AlwaysOnTop            bool
	TransparentFramebuffer bool
	Theme                  Theme
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
	return nil
}

// Backend creates native windows.
type Backend interface {
	CreateWindow(options WindowOptions) (Window, error)
}

// Drawable is a native UI object that can be presented by a GPU backend.
// FramebufferSize is measured in physical pixels, unlike widget geometry.
type Drawable interface {
	gpu.SurfaceTarget
	FramebufferSize() Size
}

// Window is a native window and the owner of its controls and dialogs. Closing
// an owner also closes its utility windows. A utility window's native close
// control hides it and sets ShouldClose; clear that request before showing the
// window again. Close releases the window and must be safe to call repeatedly.
// PollEvents and methods that create or display native UI must be called from
// the platform UI thread. On macOS that is the program's main thread.
type Window interface {
	Drawable
	Input

	Close()
	Kind() WindowKind
	Title() string
	SetTitle(title string)
	Theme() Theme
	SetTheme(theme Theme) error
	Size() Size
	SetSize(size Size) error
	Position() Point
	SetPosition(position Point)
	IsVisible() bool
	SetVisible(visible bool)
	BringToFront()
	OnMove(handler MoveHandler) Unsubscribe

	ShouldClose() bool
	SetShouldClose(close bool)
	PollEvents()

	SetContent(content Widget) error
	CreateLayout(spec LayoutSpec) (Layout, error)
	CreateLabel(text string) (Label, error)
	CreateButton(text string) (Button, error)
	CreateTextField() (TextField, error)
	CreateTextArea(options TextAreaOptions) (TextArea, error)
	CreateCheckBox(text string, checked bool) (CheckBox, error)
	CreateRadioGroup(options RadioGroupOptions) (RadioGroup, error)
	CreateSelect(options SelectOptions) (Select, error)
	CreateSlider(options SliderOptions) (Slider, error)
	CreateProgressBar(value float64) (ProgressBar, error)
	CreateActivityIndicator(running bool) (ActivityIndicator, error)
	CreateSeparator(direction LayoutDirection) (Separator, error)
	CreateImage(options ImageOptions) (ImageView, error)
	CreateLink(text, url string) (Link, error)
	CreateTabs(options TabsOptions) (Tabs, error)
	CreateScrollView(options ScrollViewOptions) (ScrollView, error)
	CreateCanvas() (Canvas, error)
	CreateSection(options SectionOptions) (Section, error)

	OpenFile(ctx context.Context, options OpenFileOptions) (string, error)
	OpenFiles(ctx context.Context, options OpenFileOptions) ([]string, error)
	SaveFile(ctx context.Context, options SaveFileOptions) (string, error)
	SelectDirectory(ctx context.Context, options DirectoryDialogOptions) (string, error)
}
