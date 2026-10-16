//go:build linux && cgo

package gtk_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

var gtkWidgets struct {
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

var gtkWindow ui.Window

func TestMain(m *testing.M) {
	backend := &gtk.Backend{}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "ui widget test",
		Size:   ui.Size{Width: 600, Height: 300},
		Hidden: true,
	})
	if err == nil {
		gtkWindow = win
		err = buildGTKWidgetTest(backend, win)
	}
	if err == nil {
		err = checkBorderlessKeyWindows(backend, win)
	}
	gtkWidgets.err = err
	gtkTestWindow, gtkTestWindowError = gtk.CreateWindow("gamekit test", 320, 200, &gtk.WindowOptions{Hidden: true, Resizable: true})

	code := m.Run()
	if win != nil {
		win.Close()
	}
	if gtkTestWindow != nil {
		gtkTestWindow.Destroy()
	}
	os.Exit(code)
}

// checkBorderlessKeyWindows must run on the main thread, so TestMain calls it.
func checkBorderlessKeyWindows(backend ui.Backend, owner ui.Window) error {
	gtkWidgets.borderlessKey = map[string]bool{}
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
		gtkWidgets.borderlessKey[name] = gtk.CanBecomeKey(window)
		window.Close()
	}
	return nil
}

func TestBorderlessWindowsCanTakeKeyboardFocus(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	for _, kind := range []string{"normal", "utility"} {
		if !gtkWidgets.borderlessKey[kind] {
			t.Errorf("borderless %s window cannot become the key window", kind)
		}
	}
}

func TestGTKDialogHonorsCancelledContext(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := gtkWindow.OpenFile(ctx, ui.OpenFileOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("OpenFile() error = %v, want context.Canceled", err)
	}
}

func buildGTKWidgetTest(backend ui.Backend, win ui.Window) error {
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

	gtkWidgets.rootBounds = root.Bounds()
	gtkWidgets.canvasBounds = canvas.Bounds()
	gtkWidgets.sidebarBounds = sidebar.Bounds()
	gtkWidgets.labelText = label.Text()
	gtkWidgets.buttonText = button.Text()
	gtkWidgets.placeholder = field.Placeholder()
	gtkWidgets.sliderValue = slider.Value()
	gtkWidgets.framebuffer = canvas.FramebufferSize()
	gtkWidgets.windowTarget = win.NativeSurface()
	gtkWidgets.canvasTarget = canvas.NativeSurface()

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
	gtkWidgets.utilityTitle = utility.Title()
	gtkWidgets.utilityKind = utility.Kind()
	gtkWidgets.utilityPosition = utility.Position()
	gtkWidgets.utilitySize = utility.Size()
	gtkWidgets.utilityTarget = utility.NativeSurface()
	gtkWidgets.sectionTitle = section.Title()
	gtkWidgets.sectionOpen = section.IsExpanded()
	gtkWidgets.expandedSize = section.PreferredSize()
	section.SetExpanded(false)
	gtkWidgets.collapsedSize = section.PreferredSize()
	section.SetExpanded(true)
	if err := win.SetTheme(ui.ThemeDark); err != nil {
		return err
	}
	gtkWidgets.darkTheme = win.Theme()
	if err := win.SetTheme(ui.ThemeSystem); err != nil {
		return err
	}
	return testExpandedGTKWidgets(win)
}

func testExpandedGTKWidgets(win ui.Window) error {
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
		return fmt.Errorf("GTK scroll view offset = %v, want 64", scroll.ScrollOffset())
	}
	if area.Text() != "Notes" || !check.IsChecked() || radio.SelectedIndex() != 1 ||
		selectControl.SelectedIndex() != 0 || progress.Value() != 0.5 || !activity.IsRunning() {
		return errors.New("expanded GTK controls did not preserve their initial values")
	}
	if link.URL() != "https://example.com" || tabs.SelectedIndex() != 0 {
		return errors.New("GTK link or tabs did not preserve their initial value")
	}
	activity.SetRunning(false)
	if activity.IsRunning() {
		return errors.New("GTK activity indicator remained running")
	}
	if err := tabs.SetSelectedIndex(1); err != nil || tabs.SelectedIndex() != 1 {
		return fmt.Errorf("GTK tabs selection = %d: %w", tabs.SelectedIndex(), err)
	}
	return nil
}

func TestGTKNativeWidgets(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	if gtkWidgets.labelText != "Material" {
		t.Errorf("label text = %q, want Material", gtkWidgets.labelText)
	}
	if gtkWidgets.buttonText != "Reset" {
		t.Errorf("button text = %q, want Reset", gtkWidgets.buttonText)
	}
	if gtkWidgets.placeholder != "Name" {
		t.Errorf("text field placeholder = %q, want Name", gtkWidgets.placeholder)
	}
	if gtkWidgets.sliderValue != 1 {
		t.Errorf("slider value = %v, want 1", gtkWidgets.sliderValue)
	}
}

func TestGTKSurfaceTargets(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	if gtkWidgets.windowTarget.Kind != gpu.NativeSurfaceXlibWindow || gtkWidgets.windowTarget.Handle == 0 {
		t.Errorf("window target = %#v, want a valid GTK window", gtkWidgets.windowTarget)
	}
	if gtkWidgets.canvasTarget.Kind != gpu.NativeSurfaceXlibWindow || gtkWidgets.canvasTarget.Handle == 0 {
		t.Errorf("canvas target = %#v, want a valid GTK view", gtkWidgets.canvasTarget)
	}
	if gtkWidgets.utilityTarget.Kind != gpu.NativeSurfaceXlibWindow || gtkWidgets.utilityTarget.Handle == 0 {
		t.Errorf("utility target = %#v, want a valid GTK window", gtkWidgets.utilityTarget)
	}
}

func TestGTKUtilityWindow(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	if gtkWidgets.utilityTitle != "Inspector" {
		t.Errorf("utility title = %q, want Inspector", gtkWidgets.utilityTitle)
	}
	if gtkWidgets.utilityKind != ui.WindowUtility {
		t.Errorf("utility kind = %v, want WindowUtility", gtkWidgets.utilityKind)
	}
	if gtkWidgets.utilityPosition != (ui.Point{X: 24, Y: 20}) {
		t.Errorf("utility position = %#v, want 24,20", gtkWidgets.utilityPosition)
	}
	if gtkWidgets.utilitySize != (ui.Size{Width: 240, Height: 180}) {
		t.Errorf("utility size = %#v, want 240x180", gtkWidgets.utilitySize)
	}
}

func TestGTKCollapsibleSection(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	if gtkWidgets.sectionTitle != "Rendering" || !gtkWidgets.sectionOpen {
		t.Fatalf("section = (%q, expanded %v), want (Rendering, true)", gtkWidgets.sectionTitle, gtkWidgets.sectionOpen)
	}
	if gtkWidgets.collapsedSize.Height >= gtkWidgets.expandedSize.Height {
		t.Errorf("collapsed height = %d, want less than expanded height %d", gtkWidgets.collapsedSize.Height, gtkWidgets.expandedSize.Height)
	}
}

func TestGTKWindowTheme(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	if gtkWidgets.darkTheme != ui.ThemeDark {
		t.Errorf("theme = %v, want ThemeDark", gtkWidgets.darkTheme)
	}
}

func TestGTKNativeLayout(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	if gtkWidgets.rootBounds.Size != (ui.Size{Width: 600, Height: 300}) {
		t.Errorf("root size = %#v, want 600 by 300", gtkWidgets.rootBounds.Size)
	}
	if gtkWidgets.canvasBounds.Size.Width != 400 {
		t.Errorf("canvas width = %d, want 400", gtkWidgets.canvasBounds.Size.Width)
	}
	if gtkWidgets.sidebarBounds != (ui.Rect{
		Position: ui.Point{X: 400},
		Size:     ui.Size{Width: 200, Height: 300},
	}) {
		t.Errorf("sidebar bounds = %#v, want right third", gtkWidgets.sidebarBounds)
	}
	if !gtkWidgets.framebuffer.IsValid() {
		t.Errorf("canvas framebuffer size = %#v, want positive dimensions", gtkWidgets.framebuffer)
	}
}
