//go:build windows && cgo

package win32_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/win32"
)

var win32Widgets struct {
	err error

	rootBounds      ui.Rect
	canvasBounds    ui.Rect
	sidebarBounds   ui.Rect
	labelText       string
	buttonText      string
	placeholder     string
	sliderValue     float64
	framebuffer     ui.Size
	windowTarget    gpu.NativeSurface
	canvasTarget    gpu.NativeSurface
	utilityTarget   gpu.NativeSurface
	utilityTitle    string
	utilityKind     ui.WindowKind
	utilityPosition ui.Point
	utilitySize     ui.Size
	sectionTitle    string
	sectionOpen     bool
	expandedSize    ui.Size
	collapsedSize   ui.Size
	darkTheme       ui.Theme
	// borderlessKey records whether borderless normal and utility windows,
	// as games use, can take keyboard focus.
	borderlessKey map[string]bool
}

var win32Window ui.Window

func TestMain(m *testing.M) {
	backend := &win32.Backend{}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "ui widget test",
		Size:   ui.Size{Width: 600, Height: 300},
		Hidden: true,
	})
	if err == nil {
		win32Window = win
		err = buildWin32WidgetTest(backend, win)
	}
	if err == nil {
		err = checkBorderlessKeyWindows(backend, win)
	}
	win32Widgets.err = err
	win32TestWindow, win32TestWindowError = win32.CreateWindow("gamekit test", 320, 200, &win32.WindowOptions{Hidden: true, Resizable: true})

	code := m.Run()
	if win != nil {
		win.Close()
	}
	if win32TestWindow != nil {
		win32TestWindow.Destroy()
	}
	os.Exit(code)
}

// checkBorderlessKeyWindows must run on the main thread, so TestMain calls it.
func checkBorderlessKeyWindows(backend ui.Backend, owner ui.Window) error {
	win32Widgets.borderlessKey = map[string]bool{}
	for _, kind := range []ui.WindowKind{ui.WindowNormal, ui.WindowUtility} {
		options := ui.WindowOptions{Size: ui.Size{Width: 200, Height: 100}, Hidden: true, Borderless: true, Kind: kind}
		name := "normal"
		if kind == ui.WindowUtility {
			options.Owner, name = owner, "utility"
		}
		window, err := backend.CreateWindow(options)
		if err != nil {
			return err
		}
		window.SetBorderless(false)
		window.SetBorderless(true)
		win32Widgets.borderlessKey[name] = win32.CanBecomeKey(window)
		window.Close()
	}
	return nil
}

func TestBorderlessWindowsCanTakeKeyboardFocus(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	for _, kind := range []string{"normal", "utility"} {
		if !win32Widgets.borderlessKey[kind] {
			t.Errorf("borderless %s window cannot become the key window", kind)
		}
	}
}

func TestWin32DialogHonorsCancelledContext(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := win32Window.OpenFile(ctx, ui.OpenFileOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("OpenFile() error = %v, want context.Canceled", err)
	}
}

func buildWin32WidgetTest(backend ui.Backend, win ui.Window) error {
	label, err := win.CreateLabel("Material")
	if err != nil {
		return err
	}
	button, err := win.CreateButton("Reset")
	if err != nil {
		return err
	}
	field, err := win.CreateTextField()
	if err != nil {
		return err
	}
	field.SetPlaceholder("Name")
	slider, err := win.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 4, Value: 1, Step: 0.25})
	if err != nil {
		return err
	}
	canvas, err := win.CreateCanvas()
	if err != nil {
		return err
	}

	sidebar, err := win.CreateLayout(
		ui.Column(
			ui.Item(label),
			ui.Item(button),
			ui.Item(field),
			ui.Item(slider),
			ui.Space(),
		).WithPadding(ui.UniformInsets(16)).WithGap(12),
	)
	if err != nil {
		return err
	}
	root, err := win.CreateLayout(ui.Row(
		ui.Item(canvas).Grow(2),
		ui.Item(sidebar).Grow(1),
	))
	if err != nil {
		return err
	}
	if err := win.SetContent(root); err != nil {
		return err
	}

	win32Widgets.rootBounds = root.Bounds()
	win32Widgets.canvasBounds = canvas.Bounds()
	win32Widgets.sidebarBounds = sidebar.Bounds()
	win32Widgets.labelText = label.Text()
	win32Widgets.buttonText = button.Text()
	win32Widgets.placeholder = field.Placeholder()
	win32Widgets.sliderValue = slider.Value()
	win32Widgets.framebuffer = canvas.FramebufferSize()
	win32Widgets.windowTarget = win.NativeSurface()
	win32Widgets.canvasTarget = canvas.NativeSurface()

	utility, err := backend.CreateWindow(ui.WindowOptions{
		Title:    "Inspector",
		Size:     ui.Size{Width: 240, Height: 180},
		Kind:     ui.WindowUtility,
		Owner:    win,
		Position: ui.Point{X: 24, Y: 20},
		Hidden:   true,
	})
	if err != nil {
		return err
	}
	panelLabel, err := utility.CreateLabel("Exposure")
	if err != nil {
		return err
	}
	sectionContent, err := utility.CreateLayout(ui.Column(ui.Item(panelLabel)).WithPadding(ui.UniformInsets(8)))
	if err != nil {
		return err
	}
	section, err := utility.CreateSection(ui.SectionOptions{
		Title:    "Rendering",
		Content:  sectionContent,
		Expanded: true,
	})
	if err != nil {
		return err
	}
	panelContent, err := utility.CreateLayout(ui.Column(ui.Item(section), ui.Space()))
	if err != nil {
		return err
	}
	if err := utility.SetContent(panelContent); err != nil {
		return err
	}
	win32Widgets.utilityTitle = utility.Title()
	win32Widgets.utilityKind = utility.Kind()
	win32Widgets.utilityPosition = utility.Position()
	win32Widgets.utilitySize = utility.Size()
	win32Widgets.utilityTarget = utility.NativeSurface()
	win32Widgets.sectionTitle = section.Title()
	win32Widgets.sectionOpen = section.IsExpanded()
	win32Widgets.expandedSize = section.PreferredSize()
	section.SetExpanded(false)
	win32Widgets.collapsedSize = section.PreferredSize()
	section.SetExpanded(true)
	if err := win.SetTheme(ui.ThemeDark); err != nil {
		return err
	}
	win32Widgets.darkTheme = win.Theme()
	if err := win.SetTheme(ui.ThemeSystem); err != nil {
		return err
	}
	return testExpandedWin32Widgets(win)
}

func testExpandedWin32Widgets(win ui.Window) error {
	area, err := win.CreateTextArea(ui.TextAreaOptions{Text: "Notes", Wrap: true})
	if err != nil {
		return err
	}
	check, err := win.CreateCheckBox("Enabled", true)
	if err != nil {
		return err
	}
	radio, err := win.CreateRadioGroup(ui.RadioGroupOptions{Items: []string{"A", "B"}, SelectedIndex: 1})
	if err != nil {
		return err
	}
	selectControl, err := win.CreateSelect(ui.SelectOptions{Items: []string{"One", "Two"}, SelectedIndex: 0})
	if err != nil {
		return err
	}
	progress, err := win.CreateProgressBar(0.5)
	if err != nil {
		return err
	}
	activity, err := win.CreateActivityIndicator(true)
	if err != nil {
		return err
	}
	separator, err := win.CreateSeparator(ui.Horizontal)
	if err != nil {
		return err
	}
	imageView, err := win.CreateImage(ui.ImageOptions{Source: image.NewRGBA(image.Rect(0, 0, 8, 8))})
	if err != nil {
		return err
	}
	link, err := win.CreateLink("GameKit", "https://example.com")
	if err != nil {
		return err
	}
	page, err := win.CreateLayout(ui.Column(
		ui.Item(area), ui.Item(check), ui.Item(radio), ui.Item(selectControl),
		ui.Item(progress), ui.Item(activity), ui.Item(separator), ui.Item(imageView), ui.Item(link),
	))
	if err != nil {
		return err
	}
	secondLabel, err := win.CreateLabel("Second")
	if err != nil {
		return err
	}
	second, err := win.CreateLayout(ui.Column(ui.Item(secondLabel)))
	if err != nil {
		return err
	}
	tabs, err := win.CreateTabs(ui.TabsOptions{Items: []ui.TabItem{
		{Title: "New", Content: page},
		{Title: "Second", Content: second},
	}, SelectedIndex: 0})
	if err != nil {
		return err
	}
	defer tabs.Close()
	tabs.SetBounds(ui.Rect{Size: ui.Size{Width: 500, Height: 280}})
	scrollLabel, err := win.CreateLabel("Scrollable")
	if err != nil {
		return err
	}
	scrollContent, err := win.CreateLayout(ui.Column(
		ui.Item(scrollLabel).MinimumSize(ui.Size{Height: 500}),
	))
	if err != nil {
		return err
	}
	scroll, err := win.CreateScrollView(ui.ScrollViewOptions{Content: scrollContent})
	if err != nil {
		return err
	}
	defer scroll.Close()
	scroll.SetBounds(ui.Rect{Size: ui.Size{Width: 200, Height: 100}})
	scroll.SetScrollOffset(ui.Point{Y: 64})
	if scroll.Content() != scrollContent || scroll.ScrollOffset().Y != 64 {
		return fmt.Errorf("Win32 scroll view offset = %v, want 64", scroll.ScrollOffset())
	}
	if area.Text() != "Notes" || !check.IsChecked() || radio.SelectedIndex() != 1 ||
		selectControl.SelectedIndex() != 0 || progress.Value() != 0.5 || !activity.IsRunning() {
		return errors.New("expanded Win32 controls did not preserve their initial values")
	}
	if link.URL() != "https://example.com" || tabs.SelectedIndex() != 0 {
		return errors.New("Win32 link or tabs did not preserve their initial value")
	}
	activity.SetRunning(false)
	if activity.IsRunning() {
		return errors.New("Win32 activity indicator remained running")
	}
	if err := tabs.SetSelectedIndex(1); err != nil || tabs.SelectedIndex() != 1 {
		return fmt.Errorf("Win32 tabs selection = %d: %w", tabs.SelectedIndex(), err)
	}
	return nil
}

func TestWin32NativeWidgets(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	if win32Widgets.labelText != "Material" {
		t.Errorf("label text = %q, want Material", win32Widgets.labelText)
	}
	if win32Widgets.buttonText != "Reset" {
		t.Errorf("button text = %q, want Reset", win32Widgets.buttonText)
	}
	if win32Widgets.placeholder != "Name" {
		t.Errorf("text field placeholder = %q, want Name", win32Widgets.placeholder)
	}
	if win32Widgets.sliderValue != 1 {
		t.Errorf("slider value = %v, want 1", win32Widgets.sliderValue)
	}
}

func TestWin32SurfaceTargets(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	if win32Widgets.windowTarget.Kind != gpu.NativeSurfaceWin32Window || win32Widgets.windowTarget.Handle == 0 {
		t.Errorf("window target = %#v, want a valid Win32 window", win32Widgets.windowTarget)
	}
	if win32Widgets.canvasTarget.Kind != gpu.NativeSurfaceWin32Window || win32Widgets.canvasTarget.Handle == 0 {
		t.Errorf("canvas target = %#v, want a valid Win32 view", win32Widgets.canvasTarget)
	}
	if win32Widgets.utilityTarget.Kind != gpu.NativeSurfaceWin32Window || win32Widgets.utilityTarget.Handle == 0 {
		t.Errorf("utility target = %#v, want a valid Win32 window", win32Widgets.utilityTarget)
	}
}

func TestWin32UtilityWindow(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	if win32Widgets.utilityTitle != "Inspector" {
		t.Errorf("utility title = %q, want Inspector", win32Widgets.utilityTitle)
	}
	if win32Widgets.utilityKind != ui.WindowUtility {
		t.Errorf("utility kind = %v, want WindowUtility", win32Widgets.utilityKind)
	}
	if win32Widgets.utilityPosition != (ui.Point{X: 24, Y: 20}) {
		t.Errorf("utility position = %#v, want 24,20", win32Widgets.utilityPosition)
	}
	if win32Widgets.utilitySize != (ui.Size{Width: 240, Height: 180}) {
		t.Errorf("utility size = %#v, want 240x180", win32Widgets.utilitySize)
	}
}

func TestWin32CollapsibleSection(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	if win32Widgets.sectionTitle != "Rendering" || !win32Widgets.sectionOpen {
		t.Fatalf("section = (%q, expanded %v), want (Rendering, true)", win32Widgets.sectionTitle, win32Widgets.sectionOpen)
	}
	if win32Widgets.collapsedSize.Height >= win32Widgets.expandedSize.Height {
		t.Errorf("collapsed height = %d, want less than expanded height %d", win32Widgets.collapsedSize.Height, win32Widgets.expandedSize.Height)
	}
}

func TestWin32WindowTheme(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	if win32Widgets.darkTheme != ui.ThemeDark {
		t.Errorf("theme = %v, want ThemeDark", win32Widgets.darkTheme)
	}
}

func TestWin32NativeLayout(t *testing.T) {
	if win32Widgets.err != nil {
		t.Fatal(win32Widgets.err)
	}
	if win32Widgets.rootBounds.Size != (ui.Size{Width: 600, Height: 300}) {
		t.Errorf("root size = %#v, want 600 by 300", win32Widgets.rootBounds.Size)
	}
	if win32Widgets.canvasBounds.Size.Width != 400 {
		t.Errorf("canvas width = %d, want 400", win32Widgets.canvasBounds.Size.Width)
	}
	if win32Widgets.sidebarBounds != (ui.Rect{
		Position: ui.Point{X: 400},
		Size:     ui.Size{Width: 200, Height: 300},
	}) {
		t.Errorf("sidebar bounds = %#v, want right third", win32Widgets.sidebarBounds)
	}
	if !win32Widgets.framebuffer.IsValid() {
		t.Errorf("canvas framebuffer size = %#v, want positive dimensions", win32Widgets.framebuffer)
	}
}
