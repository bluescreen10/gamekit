package ui_test

import (
	"errors"
	"fmt"

	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

func checkLayout(r *recorder, window ui.Window) {
	first, firstErr := window.CreateLabel("First")
	second, secondErr := window.CreateButton("Second")
	if firstErr != nil || secondErr != nil {
		r.check("layout/create_children", func() error {
			return errors.Join(firstErr, secondErr)
		})
		return
	}
	spec := ui.Row(
		ui.Item(first).MinimumSize(ui.Size{Width: 80}),
		ui.Space(),
		ui.Item(second).Grow(1).Align(ui.AlignCenter),
	).WithPadding(ui.Insets{Top: 3, Right: 5, Bottom: 7, Left: 11}).WithGap(13)
	var layout ui.Layout
	r.check("layout/create", func() error {
		var err error
		layout, err = window.CreateLayout(spec)
		return err
	})
	if layout == nil {
		first.Close()
		second.Close()
		return
	}
	checkWidget(r, "layout", layout)
	r.check("layout/spec/initial", func() error {
		got := layout.Spec()
		if got.Direction != ui.Horizontal || got.Padding != spec.Padding || got.Gap != 13 || len(got.Items) != 3 {
			return fmt.Errorf("Spec = %#v, want row with three items, padding, and gap", got)
		}
		if got.Items[0].Widget != first || !got.Items[1].IsSpacer || got.Items[2].Widget != second {
			return errors.New("Spec did not preserve widgets and spacer")
		}
		return nil
	})
	r.check("layout/children", func() error {
		return wantSliceEqual(layout.Children(), []ui.Widget{first, second})
	})
	r.check("layout/resolution", func() error {
		layout.SetBounds(ui.Rect{Size: ui.Size{Width: 420, Height: 120}})
		if first.Bounds().Size.Width < 80 {
			return fmt.Errorf("first width = %d, want at least 80", first.Bounds().Size.Width)
		}
		if first.Bounds().Position.X != 11 {
			return fmt.Errorf("first x = %d, want left padding 11", first.Bounds().Position.X)
		}
		if second.Bounds().Position.X <= first.Bounds().Position.X {
			return errors.New("row did not position second child after first")
		}
		return nil
	})
	r.check("layout/spec/set", func() error {
		next := ui.Column(ui.Item(second), ui.Item(first)).WithGap(4)
		if err := layout.SetSpec(next); err != nil {
			return err
		}
		got := layout.Spec()
		if got.Direction != ui.Vertical || got.Gap != 4 {
			return fmt.Errorf("Spec after SetSpec = %#v", got)
		}
		return wantSliceEqual(layout.Children(), []ui.Widget{second, first})
	})
	r.check("layout/spec/reject_duplicate", func() error {
		if err := layout.SetSpec(ui.Row(ui.Item(first), ui.Item(first))); err == nil {
			return errors.New("SetSpec accepted the same child twice")
		}
		return nil
	})
	closeWidget(r, "layout", layout)
}

func checkLabel(r *recorder, window ui.Window) {
	label, err := window.CreateLabel("Initial label")
	r.check("label/create", func() error {
		return err
	})
	if label == nil {
		return
	}
	checkWidget(r, "label", label)
	r.check("label/text/initial", func() error {
		return wantEqual(label.Text(), "Initial label")
	})
	r.check("label/text/set", func() error {
		label.SetText("Updated label")
		return wantEqual(label.Text(), "Updated label")
	})
	closeWidget(r, "label", label)
}

func checkButton(r *recorder, window ui.Window) {
	button, err := window.CreateButton("Run")
	r.check("button/create", func() error {
		return err
	})
	if button == nil {
		return
	}
	checkWidget(r, "button", button)
	r.check("button/text/initial", func() error {
		return wantEqual(button.Text(), "Run")
	})
	r.check("button/text/set", func() error {
		button.SetText("Stop")
		return wantEqual(button.Text(), "Stop")
	})
	r.check("button/callback/subscription", func() error {
		calls := 0
		unsubscribe := button.OnClick(func() {
			calls++
		})
		button.SetText("No activation")
		if calls != 0 {
			return errors.New("programmatic text change invoked OnClick")
		}
		unsubscribe()
		unsubscribe()
		return nil
	})
	closeWidget(r, "button", button)
}

func checkTextField(r *recorder, window ui.Window) {
	field, err := window.CreateTextField()
	r.check("text_field/create", func() error {
		return err
	})
	if field == nil {
		return
	}
	checkWidget(r, "text_field", field)
	r.check("text_field/text/initial", func() error {
		return wantEqual(field.Text(), "")
	})
	r.check("text_field/text/set_without_event", func() error {
		calls := 0
		unsubscribe := field.OnChange(func(string) {
			calls++
		})
		field.SetText("Ada")
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetText invoked OnChange %d times", calls)
		}
		return wantEqual(field.Text(), "Ada")
	})
	r.check("text_field/placeholder", func() error {
		field.SetPlaceholder("Name")
		return wantEqual(field.Placeholder(), "Name")
	})
	closeWidget(r, "text_field", field)
}

func checkTextArea(r *recorder, window ui.Window) {
	area, err := window.CreateTextArea(ui.TextAreaOptions{
		Text:        "Initial notes",
		Placeholder: "Notes",
		ReadOnly:    true,
		Wrap:        true,
	})
	r.check("text_area/create", func() error {
		return err
	})
	if area == nil {
		return
	}
	checkWidget(r, "text_area", area)
	r.check("text_area/initial_attributes", func() error {
		if area.Text() != "Initial notes" || area.Placeholder() != "Notes" || !area.IsReadOnly() {
			return fmt.Errorf("initial attributes = (%q, %q, readOnly %v)", area.Text(), area.Placeholder(), area.IsReadOnly())
		}
		return nil
	})
	r.check("text_area/text/set_without_event", func() error {
		calls := 0
		unsubscribe := area.OnChange(func(string) {
			calls++
		})
		area.SetText("Updated notes")
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetText invoked OnChange %d times", calls)
		}
		return wantEqual(area.Text(), "Updated notes")
	})
	r.check("text_area/placeholder/set", func() error {
		area.SetPlaceholder("Details")
		return wantEqual(area.Placeholder(), "Details")
	})
	r.check("text_area/read_only/set", func() error {
		area.SetReadOnly(false)
		if area.IsReadOnly() {
			return errors.New("text area remained read-only")
		}
		area.SetReadOnly(true)
		if !area.IsReadOnly() {
			return errors.New("text area did not become read-only")
		}
		return nil
	})
	closeWidget(r, "text_area", area)
}

func checkCheckBox(r *recorder, window ui.Window) {
	checkBox, err := window.CreateCheckBox("Enabled", true)
	r.check("check_box/create", func() error {
		return err
	})
	if checkBox == nil {
		return
	}
	checkWidget(r, "check_box", checkBox)
	r.check("check_box/initial_attributes", func() error {
		if checkBox.Text() != "Enabled" || !checkBox.IsChecked() {
			return fmt.Errorf("initial attributes = (%q, checked %v)", checkBox.Text(), checkBox.IsChecked())
		}
		return nil
	})
	r.check("check_box/text/set", func() error {
		checkBox.SetText("Lighting")
		return wantEqual(checkBox.Text(), "Lighting")
	})
	r.check("check_box/checked/set_without_event", func() error {
		calls := 0
		unsubscribe := checkBox.OnChange(func(bool) {
			calls++
		})
		checkBox.SetChecked(false)
		if checkBox.IsChecked() {
			return errors.New("check box remained checked")
		}
		checkBox.SetChecked(true)
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetChecked invoked OnChange %d times", calls)
		}
		if !checkBox.IsChecked() {
			return errors.New("check box did not become checked")
		}
		return nil
	})
	closeWidget(r, "check_box", checkBox)
}

func checkRadioGroup(r *recorder, window ui.Window) {
	wantItems := []string{"Low", "Medium", "High"}
	group, err := window.CreateRadioGroup(ui.RadioGroupOptions{Items: wantItems, SelectedIndex: 1})
	r.check("radio_group/create", func() error {
		return err
	})
	if group == nil {
		return
	}
	checkWidget(r, "radio_group", group)
	r.check("radio_group/items", func() error {
		items := group.Items()
		if err := wantSliceEqual(items, wantItems); err != nil {
			return err
		}
		items[0] = "mutated"
		return wantSliceEqual(group.Items(), wantItems)
	})
	checkSelection(r, "radio_group", group)
	closeWidget(r, "radio_group", group)
}

func checkSelect(r *recorder, window ui.Window) {
	wantItems := []string{"Metal", "Vulkan", "Software"}
	selectControl, err := window.CreateSelect(ui.SelectOptions{Items: wantItems, SelectedIndex: 0})
	r.check("select/create", func() error {
		return err
	})
	if selectControl == nil {
		return
	}
	checkWidget(r, "select", selectControl)
	r.check("select/items", func() error {
		items := selectControl.Items()
		if err := wantSliceEqual(items, wantItems); err != nil {
			return err
		}
		items[0] = "mutated"
		return wantSliceEqual(selectControl.Items(), wantItems)
	})
	checkSelection(r, "select", selectControl)
	closeWidget(r, "select", selectControl)
}

type selectionControl interface {
	SelectedIndex() int
	SetSelectedIndex(index int) error
	OnChange(handler ui.SelectionChangeHandler) ui.Unsubscribe
}

func checkSelection(r *recorder, name string, selection selectionControl) {
	r.check(name+"/selection/initial", func() error {
		if name == "radio_group" {
			return wantEqual(selection.SelectedIndex(), 1)
		}
		return wantEqual(selection.SelectedIndex(), 0)
	})
	r.check(name+"/selection/set_without_event", func() error {
		calls := 0
		unsubscribe := selection.OnChange(func(int) {
			calls++
		})
		if err := selection.SetSelectedIndex(2); err != nil {
			return err
		}
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetSelectedIndex invoked OnChange %d times", calls)
		}
		return wantEqual(selection.SelectedIndex(), 2)
	})
	r.check(name+"/selection/reject_invalid", func() error {
		before := selection.SelectedIndex()
		if err := selection.SetSelectedIndex(-1); err == nil {
			return errors.New("SetSelectedIndex accepted -1")
		}
		if err := selection.SetSelectedIndex(99); err == nil {
			return errors.New("SetSelectedIndex accepted an out-of-range index")
		}
		return wantEqual(selection.SelectedIndex(), before)
	})
}

func checkSlider(r *recorder, window ui.Window) {
	slider, err := window.CreateSlider(ui.SliderOptions{Minimum: -2, Maximum: 4, Value: 1, Step: 0.25})
	r.check("slider/create", func() error {
		return err
	})
	if slider == nil {
		return
	}
	checkWidget(r, "slider", slider)
	r.check("slider/value/initial", func() error {
		return wantEqual(slider.Value(), 1.0)
	})
	r.check("slider/value/set_without_event", func() error {
		calls := 0
		unsubscribe := slider.OnChange(func(float64) {
			calls++
		})
		slider.SetValue(2.5)
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetValue invoked OnChange %d times", calls)
		}
		return wantEqual(slider.Value(), 2.5)
	})
	closeWidget(r, "slider", slider)
}

func checkProgressBar(r *recorder, window ui.Window) {
	progress, err := window.CreateProgressBar(0.25)
	r.check("progress_bar/create", func() error {
		return err
	})
	if progress == nil {
		return
	}
	checkWidget(r, "progress_bar", progress)
	r.check("progress_bar/value/initial", func() error {
		return wantEqual(progress.Value(), 0.25)
	})
	for _, value := range []float64{0, 0.5, 1} {
		value := value
		r.check(fmt.Sprintf("progress_bar/value/set_%g", value), func() error {
			progress.SetValue(value)
			return wantEqual(progress.Value(), value)
		})
	}
	closeWidget(r, "progress_bar", progress)
}

func checkActivityIndicator(r *recorder, window ui.Window) {
	activity, err := window.CreateActivityIndicator(true)
	r.check("activity_indicator/create", func() error {
		return err
	})
	if activity == nil {
		return
	}
	checkWidget(r, "activity_indicator", activity)
	r.check("activity_indicator/running/initial", func() error {
		if !activity.IsRunning() {
			return errors.New("activity indicator is not initially running")
		}
		return nil
	})
	r.check("activity_indicator/running/set", func() error {
		activity.SetRunning(false)
		if activity.IsRunning() {
			return errors.New("activity indicator remained running")
		}
		activity.SetRunning(true)
		if !activity.IsRunning() {
			return errors.New("activity indicator did not start")
		}
		return nil
	})
	closeWidget(r, "activity_indicator", activity)
}

func checkSeparator(r *recorder, window ui.Window) {
	for _, direction := range []ui.LayoutDirection{ui.Horizontal, ui.Vertical} {
		direction := direction
		name := fmt.Sprintf("separator/direction_%d", direction)
		separator, err := window.CreateSeparator(direction)
		r.check(name+"/create", func() error {
			return err
		})
		if separator == nil {
			continue
		}
		checkWidget(r, name, separator)
		r.check(name+"/value", func() error {
			return wantEqual(separator.Direction(), direction)
		})
		closeWidget(r, name, separator)
	}
}

func checkImage(r *recorder, window ui.Window) {
	view, err := window.CreateImage(ui.ImageOptions{Source: testImage(), Scaling: ui.ImageScaleFit})
	r.check("image/create", func() error {
		return err
	})
	if view == nil {
		return
	}
	checkWidget(r, "image", view)
	r.check("image/scaling/initial", func() error {
		return wantEqual(view.Scaling(), ui.ImageScaleFit)
	})
	for _, scaling := range []ui.ImageScaling{ui.ImageScaleFill, ui.ImageScaleStretch, ui.ImageScaleFit} {
		scaling := scaling
		r.check(fmt.Sprintf("image/scaling/set_%d", scaling), func() error {
			if err := view.SetScaling(scaling); err != nil {
				return err
			}
			return wantEqual(view.Scaling(), scaling)
		})
	}
	r.check("image/scaling/reject_invalid", func() error {
		before := view.Scaling()
		if err := view.SetScaling(ui.ImageScaling(255)); err == nil {
			return errors.New("SetScaling accepted an invalid value")
		}
		return wantEqual(view.Scaling(), before)
	})
	r.check("image/source/replace", func() error {
		return view.SetImage(testImage())
	})
	r.check("image/source/reject_nil", func() error {
		if err := view.SetImage(nil); err == nil {
			return errors.New("SetImage accepted nil")
		}
		return nil
	})
	closeWidget(r, "image", view)
}

func checkLink(r *recorder, window ui.Window) {
	link, err := window.CreateLink("GameKit", "https://example.com/gamekit")
	r.check("link/create", func() error {
		return err
	})
	if link == nil {
		return
	}
	checkWidget(r, "link", link)
	r.check("link/initial_attributes", func() error {
		if link.Text() != "GameKit" || link.URL() != "https://example.com/gamekit" {
			return fmt.Errorf("initial attributes = (%q, %q)", link.Text(), link.URL())
		}
		return nil
	})
	r.check("link/text/set", func() error {
		link.SetText("Documentation")
		return wantEqual(link.Text(), "Documentation")
	})
	r.check("link/url/set", func() error {
		link.SetURL("https://example.com/docs")
		return wantEqual(link.URL(), "https://example.com/docs")
	})
	r.check("link/callback/subscription", func() error {
		calls := 0
		unsubscribe := link.OnClick(func() {
			calls++
		})
		link.SetURL("https://example.com/no-activation")
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetURL invoked OnClick %d times", calls)
		}
		return nil
	})
	closeWidget(r, "link", link)
}

func checkTabs(r *recorder, window ui.Window) {
	firstLabel, firstErr := window.CreateLabel("First page")
	secondLabel, secondErr := window.CreateLabel("Second page")
	if firstErr != nil || secondErr != nil {
		r.check("tabs/create_pages", func() error {
			return errors.Join(firstErr, secondErr)
		})
		return
	}
	first, firstErr := window.CreateLayout(ui.Column(ui.Item(firstLabel)))
	second, secondErr := window.CreateLayout(ui.Column(ui.Item(secondLabel)))
	if firstErr != nil || secondErr != nil {
		r.check("tabs/create_pages", func() error {
			return errors.Join(firstErr, secondErr)
		})
		return
	}
	wantItems := []ui.TabItem{{Title: "One", Content: first}, {Title: "Two", Content: second}}
	tabs, err := window.CreateTabs(ui.TabsOptions{Items: wantItems, SelectedIndex: 0})
	r.check("tabs/create", func() error {
		return err
	})
	if tabs == nil {
		return
	}
	checkWidget(r, "tabs", tabs)
	r.check("tabs/items", func() error {
		items := tabs.Items()
		if len(items) != 2 || items[0].Title != "One" || items[0].Content != first ||
			items[1].Title != "Two" || items[1].Content != second {
			return fmt.Errorf("Items = %#v", items)
		}
		items[0].Title = "mutated"
		if tabs.Items()[0].Title != "One" {
			return errors.New("Items returned mutable backend state")
		}
		return nil
	})
	r.check("tabs/selection/initial", func() error {
		return wantEqual(tabs.SelectedIndex(), 0)
	})
	r.check("tabs/selection/set_without_event", func() error {
		calls := 0
		unsubscribe := tabs.OnChange(func(int) {
			calls++
		})
		if err := tabs.SetSelectedIndex(1); err != nil {
			return err
		}
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetSelectedIndex invoked OnChange %d times", calls)
		}
		return wantEqual(tabs.SelectedIndex(), 1)
	})
	r.check("tabs/selection/reject_invalid", func() error {
		before := tabs.SelectedIndex()
		if err := tabs.SetSelectedIndex(99); err == nil {
			return errors.New("SetSelectedIndex accepted an out-of-range tab")
		}
		return wantEqual(tabs.SelectedIndex(), before)
	})
	closeWidget(r, "tabs", tabs)
}

func checkScrollView(r *recorder, window ui.Window) {
	for _, axes := range []ui.ScrollAxes{ui.ScrollVertical, ui.ScrollHorizontal, ui.ScrollBoth} {
		axes := axes
		name := fmt.Sprintf("scroll_view/axes_%d", axes)
		label, err := window.CreateLabel("Scrollable content")
		if err != nil {
			r.check(name+"/create_content", func() error {
				return err
			})
			continue
		}
		content, err := window.CreateLayout(ui.Column(
			ui.Item(label).MinimumSize(ui.Size{Width: 500, Height: 500}),
		))
		if err != nil {
			r.check(name+"/create_content", func() error {
				return err
			})
			continue
		}
		scroll, err := window.CreateScrollView(ui.ScrollViewOptions{Content: content, Axes: axes})
		r.check(name+"/create", func() error {
			return err
		})
		if scroll == nil {
			continue
		}
		checkWidget(r, name, scroll)
		r.check(name+"/content", func() error {
			if scroll.Content() != content {
				return errors.New("Content did not return the supplied widget")
			}
			return nil
		})
		r.check(name+"/offset", func() error {
			scroll.SetBounds(ui.Rect{Size: ui.Size{Width: 120, Height: 100}})
			want := ui.Point{X: 31, Y: 47}
			scroll.SetScrollOffset(want)
			got := scroll.ScrollOffset()
			switch axes {
			case ui.ScrollVertical:
				want.X = 0
			case ui.ScrollHorizontal:
				want.Y = 0
			}
			return wantEqual(got, want)
		})
		closeWidget(r, name, scroll)
	}
}

func checkCanvas(r *recorder, window ui.Window) {
	canvas, err := window.CreateCanvas()
	r.check("canvas/create", func() error {
		return err
	})
	if canvas == nil {
		return
	}
	checkWidget(r, "canvas", canvas)
	r.check("canvas/resize_event", func() error {
		var sizes []ui.Size
		unsubscribe := canvas.OnResize(func(size ui.Size) {
			sizes = append(sizes, size)
		})
		want := ui.Size{Width: 320, Height: 180}
		canvas.SetBounds(ui.Rect{Size: want})
		unsubscribe()
		unsubscribe()
		if len(sizes) != 1 {
			return fmt.Errorf("resize events = %d, want 1", len(sizes))
		}
		return wantEqual(sizes[0], want)
	})
	r.check("canvas/redraw_request", func() error {
		if canvas.TakeRedrawRequest() {
			return errors.New("new canvas had a redraw request")
		}
		canvas.RequestRedraw()
		if !canvas.TakeRedrawRequest() {
			return errors.New("redraw request was not observable")
		}
		if canvas.TakeRedrawRequest() {
			return errors.New("redraw request was not consumed")
		}
		return nil
	})
	r.check("canvas/native_surface", func() error {
		return checkSurface(canvas.NativeSurface())
	})
	r.check("canvas/framebuffer_size", func() error {
		if !canvas.FramebufferSize().IsValid() {
			return fmt.Errorf("FramebufferSize = %#v, want positive dimensions", canvas.FramebufferSize())
		}
		return nil
	})
	r.check("canvas/input/default_state", func() error {
		if canvas.PointerButtonState(pointer.ButtonLeft) != pointer.Released {
			return errors.New("unpressed pointer button is not released")
		}
		x, y := canvas.ScrollOffset()
		if x != 0 || y != 0 {
			return fmt.Errorf("initial scroll offset = (%v, %v), want zero", x, y)
		}
		return nil
	})
	r.check("canvas/input/subscriptions", func() error {
		button := canvas.OnPointerButton(func(ui.PointerButtonEvent) {
		})
		scroll := canvas.OnScroll(func(ui.ScrollEvent) {
		})
		if button == nil || scroll == nil {
			return errors.New("pointer subscription returned nil")
		}
		button()
		button()
		scroll()
		scroll()
		return nil
	})
	closeWidget(r, "canvas", canvas)
}

func checkSection(r *recorder, window ui.Window) {
	label, err := window.CreateLabel("Section content")
	if err != nil {
		r.check("section/create_content", func() error {
			return err
		})
		return
	}
	content, err := window.CreateLayout(ui.Column(ui.Item(label)))
	if err != nil {
		r.check("section/create_content", func() error {
			return err
		})
		return
	}
	section, err := window.CreateSection(ui.SectionOptions{Title: "Advanced", Content: content, Expanded: true})
	r.check("section/create", func() error {
		return err
	})
	if section == nil {
		return
	}
	checkWidget(r, "section", section)
	r.check("section/initial_attributes", func() error {
		if section.Title() != "Advanced" || section.Content() != content || !section.IsExpanded() {
			return fmt.Errorf("initial attributes = (%q, content %T, expanded %v)", section.Title(), section.Content(), section.IsExpanded())
		}
		return nil
	})
	r.check("section/title/set", func() error {
		section.SetTitle("Rendering")
		return wantEqual(section.Title(), "Rendering")
	})
	r.check("section/expanded/set_without_event", func() error {
		calls := 0
		unsubscribe := section.OnChange(func(bool) {
			calls++
		})
		section.SetExpanded(false)
		if section.IsExpanded() {
			return errors.New("section remained expanded")
		}
		section.SetExpanded(true)
		unsubscribe()
		unsubscribe()
		if calls != 0 {
			return fmt.Errorf("SetExpanded invoked OnChange %d times", calls)
		}
		if !section.IsExpanded() {
			return errors.New("section did not expand")
		}
		return nil
	})
	r.check("section/collapsed_preferred_size", func() error {
		section.SetExpanded(true)
		expanded := section.PreferredSize()
		section.SetExpanded(false)
		collapsed := section.PreferredSize()
		if collapsed.Height >= expanded.Height {
			return fmt.Errorf("collapsed height %d is not less than expanded height %d", collapsed.Height, expanded.Height)
		}
		return nil
	})
	closeWidget(r, "section", section)
}
