package ui_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"reflect"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

// checkBackend exercises every UI attribute and state. It must run on the
// platform UI thread.
func checkBackend(backend ui.Backend) conformanceReport {
	var r recorder
	if backend == nil {
		r.check("backend/non_nil", func() error {
			return errors.New("backend is nil")
		})
		return r.report()
	}

	checkWindowValidation(&r, backend)

	var window ui.Window
	r.check("window/create", func() error {
		var err error
		window, err = backend.CreateWindow(ui.WindowOptions{
			Title:     "GameKit conformance",
			Size:      ui.Size{Width: 720, Height: 540},
			Resizable: true,
			Hidden:    true,
			Theme:     ui.ThemeSystem,
		})
		if err != nil {
			return err
		}
		if window == nil {
			return errors.New("CreateWindow returned a nil window without an error")
		}
		return nil
	})
	if window == nil {
		return r.report()
	}
	defer window.Close()

	checkWindow(&r, backend, window)
	checkExpectedWindow(&r, backend, window)
	checkExpectedInput(&r, window)
	checkWindowOptions(&r, backend)
	checkWindowClose(&r, backend)
	checkWindowContent(&r, backend)
	checkWidgetCreationValidation(&r, window)
	checkWidgetOwnership(&r, backend)
	checkWidgets(&r, window)
	checkExpectedWidgetContract(&r, window)
	checkExpectedComponents(&r, window)
	checkExpectedDialogsAndServices(&r, window)
	checkDialogs(&r, window)
	return r.report()
}

func checkWindowValidation(r *recorder, backend ui.Backend) {
	tests := []struct {
		name    string
		options ui.WindowOptions
	}{
		{name: "zero_size", options: ui.WindowOptions{}},
		{name: "negative_width", options: ui.WindowOptions{Size: ui.Size{Width: -1, Height: 100}}},
		{name: "invalid_kind", options: ui.WindowOptions{Size: ui.Size{Width: 100, Height: 100}, Kind: ui.WindowKind(255)}},
		{name: "invalid_theme", options: ui.WindowOptions{Size: ui.Size{Width: 100, Height: 100}, Theme: ui.Theme(255)}},
		{name: "utility_without_owner", options: ui.WindowOptions{Size: ui.Size{Width: 100, Height: 100}, Kind: ui.WindowUtility}},
	}
	for _, test := range tests {
		test := test
		r.check("window/validation/"+test.name, func() error {
			window, err := backend.CreateWindow(test.options)
			if window != nil {
				window.Close()
				return errors.New("invalid options created a window")
			}
			if err == nil {
				return errors.New("invalid options returned no error")
			}
			return nil
		})
	}
}

func checkWindow(r *recorder, backend ui.Backend, window ui.Window) {
	r.check("window/kind", func() error {
		return wantEqual(window.Kind(), ui.WindowNormal)
	})
	r.check("window/title/initial", func() error {
		return wantEqual(window.Title(), "GameKit conformance")
	})
	r.check("window/title/set", func() error {
		window.SetTitle("Conformance renamed")
		return wantEqual(window.Title(), "Conformance renamed")
	})
	r.check("window/theme/initial", func() error {
		return wantEqual(window.Theme(), ui.ThemeSystem)
	})
	for _, theme := range []ui.Theme{ui.ThemeLight, ui.ThemeDark, ui.ThemeSystem} {
		theme := theme
		r.check(fmt.Sprintf("window/theme/set_%d", theme), func() error {
			if err := window.SetTheme(theme); err != nil {
				return err
			}
			return wantEqual(window.Theme(), theme)
		})
	}
	r.check("window/theme/reject_invalid", func() error {
		if err := window.SetTheme(ui.Theme(255)); err == nil {
			return errors.New("SetTheme accepted an invalid theme")
		}
		return wantEqual(window.Theme(), ui.ThemeSystem)
	})
	r.check("window/size/initial", func() error {
		return wantEqual(window.Size(), ui.Size{Width: 720, Height: 540})
	})
	r.check("window/size/set", func() error {
		want := ui.Size{Width: 680, Height: 500}
		if err := window.SetSize(want); err != nil {
			return err
		}
		window.PollEvents()
		return wantEqual(window.Size(), want)
	})
	r.check("window/size/reject_invalid", func() error {
		before := window.Size()
		if err := window.SetSize(ui.Size{}); err == nil {
			return errors.New("SetSize accepted an empty size")
		}
		return wantEqual(window.Size(), before)
	})
	r.check("window/position/set", func() error {
		want := ui.Point{X: 96, Y: 88}
		window.SetPosition(want)
		window.PollEvents()
		return wantEqual(window.Position(), want)
	})
	r.check("window/move_event", func() error {
		var positions []ui.Point
		unsubscribe := window.OnMove(func(position ui.Point) {
			positions = append(positions, position)
		})
		window.SetVisible(true)
		window.PollEvents()
		want := ui.Point{X: 112, Y: 104}
		window.SetPosition(want)
		window.PollEvents()
		window.SetVisible(false)
		unsubscribe()
		unsubscribe()
		if len(positions) == 0 {
			return errors.New("SetPosition produced no move event")
		}
		return wantEqual(positions[len(positions)-1], want)
	})
	r.check("window/visibility/initial_hidden", func() error {
		if window.IsVisible() {
			return errors.New("window created hidden is visible")
		}
		return nil
	})
	r.check("window/visibility/show_hide", func() error {
		window.SetVisible(true)
		window.PollEvents()
		if !window.IsVisible() {
			return errors.New("window remained hidden after SetVisible(true)")
		}
		window.BringToFront()
		window.SetVisible(false)
		window.PollEvents()
		if window.IsVisible() {
			return errors.New("window remained visible after SetVisible(false)")
		}
		return nil
	})
	r.check("window/close_request/state", func() error {
		window.SetShouldClose(false)
		if window.ShouldClose() {
			return errors.New("ShouldClose is true after clearing it")
		}
		window.SetShouldClose(true)
		if !window.ShouldClose() {
			return errors.New("ShouldClose is false after setting it")
		}
		window.SetShouldClose(false)
		return nil
	})
	r.check("window/framebuffer_size", func() error {
		window.SetVisible(true)
		window.PollEvents()
		size := window.FramebufferSize()
		window.SetVisible(false)
		if !size.IsValid() {
			return fmt.Errorf("FramebufferSize = %#v, want positive dimensions", size)
		}
		return nil
	})
	r.check("window/native_surface", func() error {
		return checkSurface(window.NativeSurface())
	})
	r.check("window/input/default_state", func() error {
		if window.KeyState(keyboard.KeyA) != keyboard.KeyReleased {
			return errors.New("unpressed key is not released")
		}
		if window.PointerButtonState(pointer.ButtonLeft) != pointer.Released {
			return errors.New("unpressed pointer button is not released")
		}
		x, y := window.ScrollOffset()
		if x != 0 || y != 0 {
			return fmt.Errorf("initial scroll offset = (%v, %v), want zero", x, y)
		}
		return nil
	})
	for _, mode := range []pointer.Mode{pointer.Hidden, pointer.Disabled, pointer.Normal} {
		mode := mode
		r.check(fmt.Sprintf("window/input/pointer_mode_%d", mode), func() error {
			window.SetPointerMode(mode)
			return wantEqual(window.PointerMode(), mode)
		})
	}
	checkInputSubscriptions(r, window)
	checkUtilityWindow(r, backend, window)
}

func checkInputSubscriptions(r *recorder, window ui.Window) {
	r.check("window/input/subscriptions", func() error {
		unsubscribes := []ui.Unsubscribe{
			window.OnKey(func(ui.KeyEvent) {
			}),
			window.OnText(func(rune) {
			}),
			window.OnPointerButton(func(ui.PointerButtonEvent) {
			}),
			window.OnScroll(func(ui.ScrollEvent) {
			}),
		}
		for _, unsubscribe := range unsubscribes {
			if unsubscribe == nil {
				return errors.New("event subscription returned nil")
			}
			unsubscribe()
			unsubscribe()
		}
		return nil
	})
}

func checkUtilityWindow(r *recorder, backend ui.Backend, owner ui.Window) {
	r.check("utility/reject_normal_window_with_owner", func() error {
		window, err := backend.CreateWindow(ui.WindowOptions{
			Title: "Invalid owner",
			Size:  ui.Size{Width: 100, Height: 100},
			Owner: owner,
			Kind:  ui.WindowNormal,
		})
		if window != nil {
			window.Close()
			return errors.New("normal window accepted an owner")
		}
		if err == nil {
			return errors.New("normal window with an owner returned no error")
		}
		return nil
	})
	var utility ui.Window
	r.check("utility/create", func() error {
		owner.SetVisible(true)
		owner.PollEvents()
		var err error
		utility, err = backend.CreateWindow(ui.WindowOptions{
			Title:    "Inspector",
			Size:     ui.Size{Width: 260, Height: 180},
			Kind:     ui.WindowUtility,
			Owner:    owner,
			Position: ui.Point{X: 24, Y: 32},
			Hidden:   true,
			Theme:    ui.ThemeDark,
		})
		return err
	})
	if utility == nil {
		return
	}
	defer utility.Close()
	defer owner.SetVisible(false)
	r.check("utility/attributes", func() error {
		if utility.Kind() != ui.WindowUtility || utility.Title() != "Inspector" {
			return fmt.Errorf("utility identity = (%v, %q)", utility.Kind(), utility.Title())
		}
		if utility.Size() != (ui.Size{Width: 260, Height: 180}) {
			return fmt.Errorf("utility size = %#v", utility.Size())
		}
		if utility.Position() != (ui.Point{X: 24, Y: 32}) {
			return fmt.Errorf("utility position = %#v", utility.Position())
		}
		return wantEqual(utility.Theme(), ui.ThemeDark)
	})
	r.check("utility/native_surface", func() error {
		return checkSurface(utility.NativeSurface())
	})
	r.check("utility/visibility", func() error {
		utility.SetVisible(true)
		utility.PollEvents()
		if !utility.IsVisible() {
			return errors.New("utility did not become visible")
		}
		utility.SetVisible(false)
		utility.PollEvents()
		if utility.IsVisible() {
			return errors.New("utility did not become hidden")
		}
		return nil
	})
}

func checkWindowOptions(r *recorder, backend ui.Backend) {
	tests := []struct {
		name    string
		options ui.WindowOptions
	}{
		{name: "fixed_size", options: ui.WindowOptions{}},
		{name: "resizable", options: ui.WindowOptions{Resizable: true}},
		{name: "borderless", options: ui.WindowOptions{Borderless: true}},
		{name: "maximized", options: ui.WindowOptions{Maximized: true}},
		{name: "always_on_top", options: ui.WindowOptions{AlwaysOnTop: true}},
		{name: "transparent_framebuffer", options: ui.WindowOptions{TransparentFramebuffer: true}},
	}
	for _, test := range tests {
		test := test
		r.check("window/options/"+test.name, func() error {
			test.options.Title = test.name
			test.options.Size = ui.Size{Width: 240, Height: 160}
			test.options.Hidden = true
			window, err := backend.CreateWindow(test.options)
			if err != nil {
				return err
			}
			defer window.Close()
			return checkSurface(window.NativeSurface())
		})
	}
}

func checkWindowClose(r *recorder, backend ui.Backend) {
	r.check("window/close_idempotent", func() error {
		window, err := backend.CreateWindow(ui.WindowOptions{
			Title:  "Close conformance",
			Size:   ui.Size{Width: 160, Height: 100},
			Hidden: true,
		})
		if err != nil {
			return err
		}
		window.Close()
		window.Close()
		return nil
	})
}

func checkWindowContent(r *recorder, backend ui.Backend) {
	var window ui.Window
	r.check("window/content/create_host", func() error {
		var err error
		window, err = backend.CreateWindow(ui.WindowOptions{
			Title:  "Content conformance",
			Size:   ui.Size{Width: 320, Height: 200},
			Hidden: true,
		})
		return err
	})
	if window == nil {
		return
	}
	defer window.Close()

	first, firstErr := window.CreateLabel("First content")
	second, secondErr := window.CreateLabel("Second content")
	if firstErr != nil || secondErr != nil {
		r.check("window/content/create_children", func() error {
			return errors.Join(firstErr, secondErr)
		})
		return
	}
	r.check("window/content/set", func() error {
		if err := window.SetContent(first); err != nil {
			return err
		}
		window.PollEvents()
		return wantEqual(first.Bounds(), ui.Rect{Size: window.Size()})
	})
	r.check("window/content/replace", func() error {
		if err := window.SetContent(second); err != nil {
			return err
		}
		window.PollEvents()
		if first.IsVisible() {
			return errors.New("replaced content remained visible")
		}
		if !second.IsVisible() {
			return errors.New("replacement content is hidden")
		}
		return wantEqual(second.Bounds(), ui.Rect{Size: window.Size()})
	})

	other, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "Foreign content",
		Size:   ui.Size{Width: 160, Height: 100},
		Hidden: true,
	})
	if err != nil {
		r.check("window/content/create_foreign_window", func() error {
			return err
		})
		return
	}
	defer other.Close()
	foreign, err := other.CreateLabel("Foreign")
	if err != nil {
		r.check("window/content/create_foreign_widget", func() error {
			return err
		})
		return
	}
	r.check("window/content/reject_foreign_widget", func() error {
		if err := window.SetContent(foreign); err == nil {
			return errors.New("SetContent accepted a widget from another window")
		}
		return nil
	})
	closeWidget(r, "window/content/replaced", first)
	closeWidget(r, "window/content/replacement", second)
	closeWidget(r, "window/content/foreign", foreign)
}

func checkWidgetCreationValidation(r *recorder, window ui.Window) {
	tests := []struct {
		name   string
		create func() (ui.Widget, error)
	}{
		{name: "radio_group_empty", create: func() (ui.Widget, error) {
			return window.CreateRadioGroup(ui.RadioGroupOptions{})
		}},
		{name: "radio_group_selection", create: func() (ui.Widget, error) {
			return window.CreateRadioGroup(ui.RadioGroupOptions{Items: []string{"one"}, SelectedIndex: 1})
		}},
		{name: "select_empty", create: func() (ui.Widget, error) {
			return window.CreateSelect(ui.SelectOptions{})
		}},
		{name: "select_selection", create: func() (ui.Widget, error) {
			return window.CreateSelect(ui.SelectOptions{Items: []string{"one"}, SelectedIndex: -1})
		}},
		{name: "slider_range", create: func() (ui.Widget, error) {
			return window.CreateSlider(ui.SliderOptions{Minimum: 1, Maximum: 1, Value: 1})
		}},
		{name: "slider_value", create: func() (ui.Widget, error) {
			return window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 2})
		}},
		{name: "slider_step", create: func() (ui.Widget, error) {
			return window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Step: -1})
		}},
		{name: "progress_below_range", create: func() (ui.Widget, error) {
			return window.CreateProgressBar(-0.1)
		}},
		{name: "progress_above_range", create: func() (ui.Widget, error) {
			return window.CreateProgressBar(1.1)
		}},
		{name: "separator_direction", create: func() (ui.Widget, error) {
			return window.CreateSeparator(ui.LayoutDirection(255))
		}},
		{name: "image_nil", create: func() (ui.Widget, error) {
			return window.CreateImage(ui.ImageOptions{})
		}},
		{name: "image_empty", create: func() (ui.Widget, error) {
			return window.CreateImage(ui.ImageOptions{Source: image.NewRGBA(image.Rectangle{})})
		}},
		{name: "image_scaling", create: func() (ui.Widget, error) {
			return window.CreateImage(ui.ImageOptions{Source: testImage(), Scaling: ui.ImageScaling(255)})
		}},
		{name: "tabs_empty", create: func() (ui.Widget, error) {
			return window.CreateTabs(ui.TabsOptions{})
		}},
		{name: "scroll_without_content", create: func() (ui.Widget, error) {
			return window.CreateScrollView(ui.ScrollViewOptions{})
		}},
		{name: "section_without_content", create: func() (ui.Widget, error) {
			return window.CreateSection(ui.SectionOptions{})
		}},
	}
	for _, test := range tests {
		test := test
		r.check("widget/validation/"+test.name, func() error {
			widget, err := test.create()
			if widget != nil {
				widget.Close()
				return errors.New("invalid options created a widget")
			}
			if err == nil {
				return errors.New("invalid options returned no error")
			}
			return nil
		})
	}
}

func checkWidgetOwnership(r *recorder, backend ui.Backend) {
	var window ui.Window
	r.check("widget/ownership/create_host", func() error {
		var err error
		window, err = backend.CreateWindow(ui.WindowOptions{
			Title:  "Ownership conformance",
			Size:   ui.Size{Width: 240, Height: 160},
			Hidden: true,
		})
		return err
	})
	if window == nil {
		return
	}
	defer window.Close()

	r.check("widget/ownership/set_content_rejects_nil", func() error {
		if err := window.SetContent(nil); err == nil {
			return errors.New("SetContent accepted nil")
		}
		return nil
	})

	r.check("widget/ownership/set_content_rejects_parented", func() error {
		label, err := window.CreateLabel("Parented")
		if err != nil {
			return err
		}
		layout, err := window.CreateLayout(ui.Row(ui.Item(label)))
		if err != nil {
			label.Close()
			return err
		}
		defer layout.Close()
		if err := window.SetContent(label); err == nil {
			return errors.New("SetContent accepted a layout child")
		}
		return nil
	})

	r.check("widget/ownership/layout_rejects_window_content", func() error {
		label, err := window.CreateLabel("Root")
		if err != nil {
			return err
		}
		defer label.Close()
		if err := window.SetContent(label); err != nil {
			return err
		}
		layout, err := window.CreateLayout(ui.Row(ui.Item(label)))
		if layout != nil {
			layout.Close()
		}
		if err == nil {
			return errors.New("CreateLayout accepted the window content")
		}
		return nil
	})

	r.check("widget/ownership/tabs_reject_duplicate_content", func() error {
		label, err := window.CreateLabel("Duplicate")
		if err != nil {
			return err
		}
		defer label.Close()
		tabs, err := window.CreateTabs(ui.TabsOptions{
			Items: []ui.TabItem{
				{Title: "First", Content: label},
				{Title: "Second", Content: label},
			},
			SelectedIndex: 0,
		})
		if tabs != nil {
			tabs.Close()
		}
		if err == nil {
			return errors.New("CreateTabs accepted duplicate content")
		}
		return nil
	})

	other, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "Foreign ownership",
		Size:   ui.Size{Width: 160, Height: 100},
		Hidden: true,
	})
	if err != nil {
		r.check("widget/ownership/create_foreign_window", func() error {
			return err
		})
		return
	}
	defer other.Close()

	foreign, err := other.CreateLabel("Foreign")
	if err != nil {
		r.check("widget/ownership/create_foreign_widget", func() error {
			return err
		})
		return
	}
	defer foreign.Close()
	checkForeignWidgetOwnership(r, window, foreign)
}

func checkForeignWidgetOwnership(r *recorder, window ui.Window, foreign ui.Widget) {
	tests := []struct {
		name   string
		create func() (ui.Widget, error)
	}{
		{name: "layout", create: func() (ui.Widget, error) {
			return window.CreateLayout(ui.Row(ui.Item(foreign)))
		}},
		{name: "tabs", create: func() (ui.Widget, error) {
			return window.CreateTabs(ui.TabsOptions{
				Items:         []ui.TabItem{{Title: "Foreign", Content: foreign}},
				SelectedIndex: 0,
			})
		}},
		{name: "scroll_view", create: func() (ui.Widget, error) {
			return window.CreateScrollView(ui.ScrollViewOptions{Content: foreign})
		}},
		{name: "section", create: func() (ui.Widget, error) {
			return window.CreateSection(ui.SectionOptions{Title: "Foreign", Content: foreign})
		}},
	}
	for _, test := range tests {
		test := test
		r.check("widget/ownership/"+test.name+"_rejects_foreign", func() error {
			widget, err := test.create()
			if widget != nil {
				widget.Close()
			}
			if err == nil {
				return errors.New("container accepted a widget from another window")
			}
			return nil
		})
	}
}

func checkDialogs(r *recorder, window ui.Window) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	r.check("dialogs/open_file/pre_cancelled", func() error {
		_, err := window.OpenFile(cancelled, ui.OpenFileOptions{
			Title:     "Open image",
			Directory: ".",
			Filename:  "image.png",
			Filters: []ui.FileFilter{
				{Name: "Images", Patterns: []string{"*.png", "*.jpg"}},
			},
		})
		return wantError(err, context.Canceled)
	})
	r.check("dialogs/open_files/pre_cancelled", func() error {
		_, err := window.OpenFiles(cancelled, ui.OpenFileOptions{
			Title: "Open images",
			Filters: []ui.FileFilter{
				{Name: "PNG", Patterns: []string{"*.png"}},
				{Name: "JPEG", Patterns: []string{"*.jpg", "*.jpeg"}},
			},
		})
		return wantError(err, context.Canceled)
	})
	r.check("dialogs/save_file/pre_cancelled", func() error {
		_, err := window.SaveFile(cancelled, ui.SaveFileOptions{
			Title:            "Save image",
			Directory:        ".",
			Filename:         "image.png",
			Filters:          []ui.FileFilter{{Name: "PNG", Patterns: []string{"*.png"}}},
			ConfirmOverwrite: true,
		})
		return wantError(err, context.Canceled)
	})
	r.check("dialogs/select_directory/pre_cancelled", func() error {
		_, err := window.SelectDirectory(cancelled, ui.DirectoryDialogOptions{
			Title:     "Select output directory",
			Directory: ".",
		})
		return wantError(err, context.Canceled)
	})
}

func checkWidgets(r *recorder, window ui.Window) {
	checkLayout(r, window)
	checkLabel(r, window)
	checkButton(r, window)
	checkTextField(r, window)
	checkTextArea(r, window)
	checkCheckBox(r, window)
	checkRadioGroup(r, window)
	checkSelect(r, window)
	checkSlider(r, window)
	checkProgressBar(r, window)
	checkActivityIndicator(r, window)
	checkSeparator(r, window)
	checkImage(r, window)
	checkLink(r, window)
	checkTabs(r, window)
	checkScrollView(r, window)
	checkCanvas(r, window)
	checkSection(r, window)
}

func checkWidget(r *recorder, name string, widget ui.Widget) {
	r.check(name+"/accessibility", func() error {
		metadata := widget.Accessibility()
		metadata.ID = name
		if err := widget.SetAccessibility(metadata); err != nil {
			return err
		}
		if widget.Accessibility().ID != name {
			return errors.New("accessibility ID was not preserved")
		}
		if widget.AccessibilityRole() == ui.RoleUnknown {
			return errors.New("accessibility role is unknown")
		}
		return nil
	})
	r.check(name+"/bounds", func() error {
		want := ui.Rect{Position: ui.Point{X: 7, Y: 9}, Size: ui.Size{Width: 180, Height: 64}}
		widget.SetBounds(want)
		return wantEqual(widget.Bounds(), want)
	})
	r.check(name+"/preferred_size", func() error {
		size := widget.PreferredSize()
		if size.Width < 0 || size.Height < 0 {
			return fmt.Errorf("PreferredSize = %#v, want non-negative dimensions", size)
		}
		return nil
	})
	r.check(name+"/visibility", func() error {
		if !widget.IsVisible() {
			return errors.New("new widget is not visible")
		}
		widget.SetVisible(false)
		if widget.IsVisible() {
			return errors.New("widget remained visible")
		}
		widget.SetVisible(true)
		if !widget.IsVisible() {
			return errors.New("widget remained hidden")
		}
		return nil
	})
	r.check(name+"/enabled", func() error {
		if !widget.IsEnabled() {
			return errors.New("new widget is not enabled")
		}
		widget.SetEnabled(false)
		if widget.IsEnabled() {
			return errors.New("widget remained enabled")
		}
		widget.SetEnabled(true)
		if !widget.IsEnabled() {
			return errors.New("widget remained disabled")
		}
		return nil
	})
}

func closeWidget(r *recorder, name string, widget ui.Widget) {
	r.check(name+"/close_idempotent", func() error {
		widget.Close()
		widget.Close()
		return nil
	})
}

func checkSurface(surface gpu.NativeSurface) error {
	if surface.Kind == gpu.NativeSurfaceUnknown {
		return errors.New("native surface kind is unknown")
	}
	if surface.Handle == 0 {
		return errors.New("native surface handle is zero")
	}
	if surface.Kind == gpu.NativeSurfaceXlibWindow && surface.Display == 0 {
		return errors.New("Xlib surface display is zero")
	}
	return nil
}

func testImage() image.Image {
	result := image.NewRGBA(image.Rect(0, 0, 4, 3))
	result.Set(0, 0, color.RGBA{R: 255, A: 255})
	return result
}

func wantEqual[T comparable](got, want T) error {
	if got != want {
		return fmt.Errorf("got %#v, want %#v", got, want)
	}
	return nil
}

func wantSliceEqual[T comparable](got, want []T) error {
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("got %#v, want %#v", got, want)
	}
	return nil
}

func wantError(got, want error) error {
	if !errors.Is(got, want) {
		return fmt.Errorf("error = %v, want %v", got, want)
	}
	return nil
}
