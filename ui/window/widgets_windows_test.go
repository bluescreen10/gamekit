//go:build windows && cgo

package window_test

import (
	"image"
	"runtime"
	"syscall"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/window"
)

const (
	winLeftButtonDown = 0x0201
	winLeftButtonUp   = 0x0202
	winMouseWheel     = 0x020A
)

var postMessage = syscall.NewLazyDLL("user32.dll").NewProc("PostMessageW")

func TestWin32CanvasAndWindowPointerRouting(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	backend := &window.Backend{}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "Win32 pointer routing test",
		Size:   ui.Size{Width: 320, Height: 240},
		Hidden: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer win.Close()

	canvas, err := win.CreateCanvas()
	if err != nil {
		t.Fatal(err)
	}
	if err := win.SetContent(canvas); err != nil {
		t.Fatal(err)
	}

	var windowButtons []ui.PointerButtonEvent
	var canvasButtons []ui.PointerButtonEvent
	var windowScroll []ui.ScrollEvent
	var canvasScroll []ui.ScrollEvent
	win.OnPointerButton(func(event ui.PointerButtonEvent) {
		windowButtons = append(windowButtons, event)
	})
	canvas.OnPointerButton(func(event ui.PointerButtonEvent) {
		canvasButtons = append(canvasButtons, event)
	})
	win.OnScroll(func(event ui.ScrollEvent) {
		windowScroll = append(windowScroll, event)
	})
	canvas.OnScroll(func(event ui.ScrollEvent) {
		canvasScroll = append(canvasScroll, event)
	})

	handle := canvas.NativeSurface().Handle
	postWindowMessage(t, handle, winLeftButtonDown, 0, 0)
	postWindowMessage(t, handle, winLeftButtonUp, 0, 0)
	postWindowMessage(t, handle, winMouseWheel, uintptr(uint32(120)<<16), 0)
	win.PollEvents()

	if len(windowButtons) != 2 || len(canvasButtons) != 2 {
		t.Fatalf("button events = (window %d, canvas %d), want (2, 2)",
			len(windowButtons), len(canvasButtons))
	}
	if windowButtons[0].Action != pointer.Pressed || windowButtons[1].Action != pointer.Released ||
		canvasButtons[0].Action != pointer.Pressed || canvasButtons[1].Action != pointer.Released {
		t.Fatalf("button events = (window %#v, canvas %#v), want pressed then released",
			windowButtons, canvasButtons)
	}
	if win.PointerButtonState(pointer.ButtonLeft) != pointer.Released ||
		canvas.PointerButtonState(pointer.ButtonLeft) != pointer.Released {
		t.Fatal("left button remained pressed after its release")
	}
	if len(windowScroll) != 1 || len(canvasScroll) != 1 {
		t.Fatalf("scroll events = (window %d, canvas %d), want (1, 1)",
			len(windowScroll), len(canvasScroll))
	}
	if x, y := win.ScrollOffset(); x != 0 || y != 1 {
		t.Fatalf("window ScrollOffset() = (%v, %v), want (0, 1)", x, y)
	}
	if x, y := canvas.ScrollOffset(); x != 0 || y != 1 {
		t.Fatalf("canvas ScrollOffset() = (%v, %v), want (0, 1)", x, y)
	}
}

func postWindowMessage(t *testing.T, window uintptr, message uint32, wparam, lparam uintptr) {
	t.Helper()
	result, _, callErr := postMessage.Call(window, uintptr(message), wparam, lparam)
	if result == 0 {
		t.Fatalf("PostMessageW(%#x) failed: %v", message, callErr)
	}
}

func TestWin32NativeWidgets(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	backend := &window.Backend{}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "Win32 widget test",
		Size:   ui.Size{Width: 600, Height: 300},
		Hidden: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer win.Close()

	label, err := win.CreateLabel("Material")
	if err != nil {
		t.Fatal(err)
	}
	field, err := win.CreateTextField()
	if err != nil {
		t.Fatal(err)
	}
	field.SetPlaceholder("Name")
	slider, err := win.CreateSlider(ui.SliderOptions{
		Minimum: 0,
		Maximum: 4,
		Value:   1,
		Step:    0.25,
	})
	if err != nil {
		t.Fatal(err)
	}
	canvas, err := win.CreateCanvas()
	if err != nil {
		t.Fatal(err)
	}
	sidebar, err := win.CreateLayout(ui.Column(
		ui.Item(label),
		ui.Item(field),
		ui.Item(slider),
		ui.Space(),
	).WithPadding(ui.UniformInsets(16)).WithGap(12))
	if err != nil {
		t.Fatal(err)
	}
	root, err := win.CreateLayout(ui.Row(
		ui.Item(canvas).Grow(2),
		ui.Item(sidebar).Grow(1),
	))
	if err != nil {
		t.Fatal(err)
	}
	area, err := win.CreateTextArea(ui.TextAreaOptions{Text: "Notes", Wrap: true})
	if err != nil {
		t.Fatal(err)
	}
	check, err := win.CreateCheckBox("Enabled", true)
	if err != nil {
		t.Fatal(err)
	}
	radio, err := win.CreateRadioGroup(ui.RadioGroupOptions{Items: []string{"A", "B"}, SelectedIndex: 1})
	if err != nil {
		t.Fatal(err)
	}
	selectControl, err := win.CreateSelect(ui.SelectOptions{Items: []string{"One", "Two"}, SelectedIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	progress, err := win.CreateProgressBar(0.5)
	if err != nil {
		t.Fatal(err)
	}
	activity, err := win.CreateActivityIndicator(true)
	if err != nil {
		t.Fatal(err)
	}
	separator, err := win.CreateSeparator(ui.Horizontal)
	if err != nil {
		t.Fatal(err)
	}
	imageView, err := win.CreateImage(ui.ImageOptions{Source: image.NewRGBA(image.Rect(0, 0, 8, 8))})
	if err != nil {
		t.Fatal(err)
	}
	link, err := win.CreateLink("GameKit", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	newControls, err := win.CreateLayout(ui.Column(
		ui.Item(area), ui.Item(check), ui.Item(radio), ui.Item(selectControl),
		ui.Item(progress), ui.Item(activity), ui.Item(separator), ui.Item(imageView), ui.Item(link),
	))
	if err != nil {
		t.Fatal(err)
	}
	tabs, err := win.CreateTabs(ui.TabsOptions{Items: []ui.TabItem{
		{Title: "Original", Content: root},
		{Title: "New", Content: newControls},
	}, SelectedIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	scrollLabel, err := win.CreateLabel("Scrollable")
	if err != nil {
		t.Fatal(err)
	}
	scrollContent, err := win.CreateLayout(ui.Column(
		ui.Item(scrollLabel).MinimumSize(ui.Size{Height: 500}),
	))
	if err != nil {
		t.Fatal(err)
	}
	scroll, err := win.CreateScrollView(ui.ScrollViewOptions{Content: scrollContent})
	if err != nil {
		t.Fatal(err)
	}
	defer scroll.Close()
	scroll.SetBounds(ui.Rect{Size: ui.Size{Width: 200, Height: 100}})
	scroll.SetScrollOffset(ui.Point{Y: 64})
	if scroll.Content() != scrollContent || scroll.ScrollOffset().Y != 64 {
		t.Fatalf("scroll view = (%T, %v), want its content at offset 64", scroll.Content(), scroll.ScrollOffset())
	}
	if err := win.SetContent(tabs); err != nil {
		t.Fatal(err)
	}

	sectionContent, err := win.CreateLayout(ui.Column(ui.Item(label)))
	if err == nil {
		t.Fatal("CreateLayout accepted a child that already has a parent")
	}
	_ = sectionContent
	utility, err := backend.CreateWindow(ui.WindowOptions{
		Title:    "Inspector",
		Size:     ui.Size{Width: 240, Height: 180},
		Kind:     ui.WindowUtility,
		Owner:    win,
		Position: ui.Point{X: 24, Y: 20},
		Hidden:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	panelLabel, err := utility.CreateLabel("Exposure")
	if err != nil {
		t.Fatal(err)
	}
	sectionBody, err := utility.CreateLayout(ui.Column(ui.Item(panelLabel)))
	if err != nil {
		t.Fatal(err)
	}
	section, err := utility.CreateSection(ui.SectionOptions{
		Title:    "Rendering",
		Content:  sectionBody,
		Expanded: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	panelBody, err := utility.CreateLayout(ui.Column(ui.Item(section), ui.Space()))
	if err != nil {
		t.Fatal(err)
	}
	if err := utility.SetContent(panelBody); err != nil {
		t.Fatal(err)
	}

	if label.Text() != "Material" || field.Placeholder() != "Name" || slider.Value() != 1 {
		t.Fatalf("native values = (%q, %q, %v)", label.Text(), field.Placeholder(), slider.Value())
	}
	if area.Text() != "Notes" || !check.IsChecked() || radio.SelectedIndex() != 1 ||
		selectControl.SelectedIndex() != 0 || progress.Value() != 0.5 || !activity.IsRunning() {
		t.Fatal("expanded native controls did not preserve their initial values")
	}
	if link.URL() != "https://example.com" || tabs.SelectedIndex() != 0 {
		t.Fatal("native link or tabs did not preserve their initial value")
	}
	if err := tabs.SetSelectedIndex(1); err != nil || tabs.SelectedIndex() != 1 {
		t.Fatalf("SetSelectedIndex(1) = %v, selected %d", err, tabs.SelectedIndex())
	}
	if target := canvas.NativeSurface(); target.Kind != gpu.NativeSurfaceWin32Window || target.Handle == 0 {
		t.Fatalf("canvas target = %#v, want a valid child HWND", target)
	}
	if utility.Title() != "Inspector" || utility.Kind() != ui.WindowUtility {
		t.Fatalf("utility = (%q, kind %v)", utility.Title(), utility.Kind())
	}
	if utility.Position() != (ui.Point{X: 24, Y: 20}) {
		t.Fatalf("utility position = %#v, want 24,20", utility.Position())
	}
	if target := utility.NativeSurface(); target.Kind != gpu.NativeSurfaceWin32Window || target.Handle == 0 {
		t.Fatalf("utility target = %#v, want a valid HWND", target)
	}
	if !section.IsExpanded() {
		t.Fatal("section is collapsed, want expanded")
	}
	section.SetExpanded(false)
	if section.IsExpanded() {
		t.Fatal("section remained expanded")
	}
	if err := win.SetTheme(ui.ThemeDark); err != nil {
		t.Fatal(err)
	}
	win.PollEvents()
}
