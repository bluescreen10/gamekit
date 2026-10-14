package ui_test

import (
	"errors"
	"fmt"

	"github.com/bluescreen10/gamekit/ui"
)

func checkExpectedCurrentComponentExtensions(r *recorder, window ui.Window) {
	checkExpectedLabelExtensions(r, window)
	checkExpectedButtonExtensions(r, window)
	checkExpectedTextExtensions(r, window)
	checkExpectedChoiceExtensions(r, window)
	checkExpectedValueExtensions(r, window)
	checkExpectedPresentationExtensions(r, window)
	checkExpectedContainerExtensions(r, window)
}

func checkExpectedLabelExtensions(r *recorder, window ui.Window) {
	label, err := window.CreateLabel("A long label")
	r.check("label/complete/create", func() error {
		return err
	})
	if label == nil {
		return
	}
	defer label.Close()
	r.check("label/complete/state", func() error {
		label.SetTextAlignment(ui.TextAlignCenter)
		label.SetWrapMode(ui.WrapWords)
		label.SetTruncation(ui.TruncateTail)
		label.SetSelectable(true)
		label.SetSemanticStyle(ui.TextStyleHeading)
		label.SetMaximumLines(3)
		if label.TextAlignment() != ui.TextAlignCenter || label.WrapMode() != ui.WrapWords {
			return errors.New("label alignment or wrapping was not preserved")
		}
		if !label.IsSelectable() || label.MaximumLines() != 3 {
			return errors.New("label selection or line limit was not preserved")
		}
		return nil
	})
}

func checkExpectedButtonExtensions(r *recorder, window ui.Window) {
	button, err := window.CreateButton("Delete")
	r.check("button/complete/create", func() error {
		return err
	})
	if button == nil {
		return
	}
	defer button.Close()
	r.check("button/complete/state", func() error {
		button.SetIcon(ui.IconSource{SystemName: "trash"}, ui.IconBeforeText)
		button.SetRole(ui.ButtonDestructive)
		button.SetShortcut(ui.Shortcut{Key: "Backspace", Command: true})
		button.SetBusy(true)
		button.SetBadge("3")
		if button.Role() != ui.ButtonDestructive || !button.IsBusy() || button.Badge() != "3" {
			return errors.New("button role, busy state, or badge was not preserved")
		}
		return nil
	})
}

func checkExpectedTextExtensions(r *recorder, window ui.Window) {
	field, err := window.CreateTextField()
	r.check("text_field/complete/create", func() error {
		return err
	})
	if field != nil {
		defer field.Close()
		r.check("text_field/complete/state", func() error {
			field.SetText("GameKit")
			field.SetReadOnly(true)
			field.SetSelection(ui.TextRange{Start: 0, End: 4})
			field.SetMaximumLength(64)
			field.SetInputPurpose(ui.InputPurposeText)
			field.SetTextAlignment(ui.TextAlignStart)
			field.SetPrefix("Name: ")
			field.SetSuffix(" *")
			field.SetSpellCheckEnabled(true)
			if !field.IsReadOnly() || field.Selection() != (ui.TextRange{Start: 0, End: 4}) {
				return errors.New("text field read-only or selection state was not preserved")
			}
			if field.MaximumLength() != 64 || !field.IsSpellCheckEnabled() {
				return errors.New("text field length or spell-check state was not preserved")
			}
			return nil
		})
	}

	area, err := window.CreateTextArea(ui.TextAreaOptions{Text: "First\nSecond", Wrap: true})
	r.check("text_area/complete/create", func() error {
		return err
	})
	if area != nil {
		defer area.Close()
		r.check("text_area/complete/state", func() error {
			area.SetWrapMode(ui.WrapWords)
			area.SetSelection(ui.TextRange{Start: 0, End: 5})
			area.SetScrollPosition(ui.Point{X: 0, Y: 20})
			area.SetMaximumLength(1024)
			area.SetTabPolicy(ui.TabInsert)
			area.SetSpellCheckEnabled(true)
			if area.WrapMode() != ui.WrapWords || area.Selection().End != 5 {
				return errors.New("text area wrap or selection state was not preserved")
			}
			return nil
		})
	}
}

func checkExpectedChoiceExtensions(r *recorder, window ui.Window) {
	checkBox, err := window.CreateCheckBox("Mixed", false)
	r.check("check_box/complete/create", func() error {
		return err
	})
	if checkBox != nil {
		defer checkBox.Close()
		r.check("check_box/complete/state", func() error {
			checkBox.SetAllowsMixedState(true)
			checkBox.SetState(ui.CheckMixed)
			checkBox.SetLabelPlacement(ui.LabelAfterControl)
			checkBox.SetDescription("Optional value")
			if checkBox.State() != ui.CheckMixed || !checkBox.AllowsMixedState() {
				return errors.New("check box mixed state was not preserved")
			}
			return nil
		})
	}

	group, err := window.CreateRadioGroup(ui.RadioGroupOptions{
		Items:         []string{"One", "Two"},
		SelectedIndex: 0,
	})
	r.check("radio_group/complete/create", func() error {
		return err
	})
	if group != nil {
		defer group.Close()
		r.check("radio_group/complete/state", func() error {
			group.SetOrientation(ui.Vertical)
			group.SetLabel("Quality")
			group.SetChoices([]ui.Choice{{ID: "low", Text: "Low", Enabled: true}})
			return wantEqual(group.Choices()[0].ID, "low")
		})
	}

	selectControl, err := window.CreateSelect(ui.SelectOptions{
		Items:         []string{"One", "Two"},
		SelectedIndex: 0,
	})
	r.check("select/complete/create", func() error {
		return err
	})
	if selectControl != nil {
		defer selectControl.Close()
		r.check("select/complete/state", func() error {
			selectControl.SetPlaceholder("Choose")
			selectControl.SetChoices([]ui.Choice{{ID: "one", Text: "One", Enabled: true}})
			selectControl.SetPopupWidth(240)
			selectControl.SetVisibleRowCount(8)
			if selectControl.Placeholder() != "Choose" || selectControl.PopupWidth() != 240 {
				return errors.New("select placeholder or popup width was not preserved")
			}
			return nil
		})
	}
}

func checkExpectedValueExtensions(r *recorder, window ui.Window) {
	slider, err := window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 100, Value: 50, Step: 5})
	r.check("slider/complete/create", func() error {
		return err
	})
	if slider != nil {
		defer slider.Close()
		r.check("slider/complete/state", func() error {
			if slider.Minimum() != 0 || slider.Maximum() != 100 || slider.Step() != 5 {
				return errors.New("slider range was not preserved")
			}
			slider.SetOrientation(ui.Vertical)
			slider.SetInverted(true)
			slider.SetTicks(ui.SliderTicks{Frequency: 10, ShowLabels: true})
			if slider.Orientation() != ui.Vertical || !slider.IsInverted() {
				return errors.New("slider orientation or direction was not preserved")
			}
			return slider.SetValue(75)
		})
	}

	progress, err := window.CreateProgressBar(0.25)
	r.check("progress_bar/complete/create", func() error {
		return err
	})
	if progress != nil {
		defer progress.Close()
		r.check("progress_bar/complete/state", func() error {
			progress.SetMode(ui.ProgressIndeterminate)
			progress.SetOrientation(ui.Vertical)
			progress.SetText("Loading")
			progress.SetState(ui.ProgressWarning)
			if progress.Mode() != ui.ProgressIndeterminate || progress.State() != ui.ProgressWarning {
				return errors.New("progress mode or state was not preserved")
			}
			return nil
		})
	}
}

func checkExpectedPresentationExtensions(r *recorder, window ui.Window) {
	activity, err := window.CreateActivityIndicator(true)
	r.check("activity_indicator/complete/create", func() error {
		return err
	})
	if activity != nil {
		defer activity.Close()
		activity.SetControlSize(ui.ControlSizeLarge)
		activity.SetLabel("Loading")
	}

	separator, err := window.CreateSeparator(ui.Horizontal)
	r.check("separator/complete/create", func() error {
		return err
	})
	if separator != nil {
		defer separator.Close()
		separator.SetDirection(ui.Vertical)
		separator.SetLabel("Advanced")
		separator.SetPresentation(ui.SeparatorInset)
		separator.SetThickness(2)
	}

	imageView, err := window.CreateImage(ui.ImageOptions{Source: testImage(), Scaling: ui.ImageScaleFit})
	r.check("image/complete/create", func() error {
		return err
	})
	if imageView != nil {
		defer imageView.Close()
		r.check("image/complete/state", func() error {
			imageView.SetAlignment(ui.AnchorCenter)
			imageView.SetInterpolation(ui.InterpolationNearest)
			imageView.SetAlternativeText("Red pixel")
			imageView.SetSourceScale(2)
			if imageView.AlternativeText() != "Red pixel" || imageView.SourceScale() != 2 {
				return errors.New("image alternative text or source scale was not preserved")
			}
			return nil
		})
	}

	link, err := window.CreateLink("Docs", "https://example.com")
	r.check("link/complete/create", func() error {
		return err
	})
	if link != nil {
		defer link.Close()
		link.SetVisited(true)
		link.SetOpenPolicy(ui.LinkOpenPlatform)
		link.SetTooltip("Open documentation")
		link.SetIcon(ui.IconSource{SystemName: "link"})
	}
}

func checkExpectedContainerExtensions(r *recorder, window ui.Window) {
	first, firstErr := window.CreateLabel("One")
	second, secondErr := window.CreateLabel("Two")
	if firstErr != nil || secondErr != nil {
		r.check("tabs/complete/create_pages", func() error {
			return errors.Join(firstErr, secondErr)
		})
		return
	}
	tabs, err := window.CreateTabs(ui.TabsOptions{
		Items:         []ui.TabItem{{Title: "One", Content: first}, {Title: "Two", Content: second}},
		SelectedIndex: 0,
	})
	r.check("tabs/complete/create", func() error {
		return err
	})
	if tabs != nil {
		defer tabs.Close()
		r.check("tabs/complete/placement", func() error {
			tabs.SetPlacement(ui.TabPlacementLeft)
			tabs.SetOverflowPolicy(ui.TabOverflowScroll)
			if tabs.Placement() != ui.TabPlacementLeft {
				return fmt.Errorf("Placement() = %v", tabs.Placement())
			}
			return nil
		})
	}

	scrollContent, scrollContentErr := window.CreateLabel("Scrollable")
	if scrollContentErr == nil {
		scroll, scrollErr := window.CreateScrollView(ui.ScrollViewOptions{
			Content: scrollContent,
			Axes:    ui.ScrollBoth,
		})
		r.check("scroll_view/complete/create", func() error {
			return scrollErr
		})
		if scroll != nil {
			defer scroll.Close()
			r.check("scroll_view/complete/state", func() error {
				scroll.SetScrollbarPolicy(ui.ScrollbarsAutomatic)
				scroll.SetContentInsets(ui.UniformInsets(8))
				scroll.SetScrollBehavior(ui.ScrollSmooth)
				scroll.SetOverscrollPolicy(ui.OverscrollDisabled)
				if err := scroll.SetZoom(2); err != nil {
					return err
				}
				if scroll.Zoom() != 2 || !scroll.ViewportSize().IsValid() {
					return errors.New("scroll zoom or viewport size is invalid")
				}
				return scroll.ScrollToWidget(scrollContent)
			})
		}
	}

	canvas, canvasErr := window.CreateCanvas()
	if canvasErr == nil {
		defer canvas.Close()
		r.check("canvas/complete/presentation", func() error {
			frame := canvas.OnFrameRequest(func(ui.FrameRequest) {
			})
			occlusion := canvas.OnOcclusionChange(func(bool) {
			})
			surface := canvas.OnSurfaceChange(func(ui.SurfaceEvent) {
			})
			defer frame()
			defer occlusion()
			defer surface()
			if canvas.ColorSpace() == ui.ColorSpaceUnknown {
				return errors.New("canvas color space is unknown")
			}
			return nil
		})
	}

	sectionContent, sectionContentErr := window.CreateLabel("Content")
	if sectionContentErr == nil {
		section, sectionErr := window.CreateSection(ui.SectionOptions{
			Title: "Section", Content: sectionContent, Expanded: true,
		})
		r.check("section/complete/create", func() error {
			return sectionErr
		})
		if section != nil {
			defer section.Close()
			section.SetSubtitle("Details")
			section.SetIcon(ui.IconSource{SystemName: "gear"})
			section.SetBadge("2")
			section.SetAnimationEnabled(false)
		}
	}

	accordionFirst, firstAccordionErr := window.CreateLabel("First")
	accordionSecond, secondAccordionErr := window.CreateLabel("Second")
	if firstAccordionErr == nil && secondAccordionErr == nil {
		accordion, accordionErr := window.CreateAccordion(ui.AccordionOptions{
			Sections: []ui.AccordionSection{
				{ID: "first", Title: "First", Content: accordionFirst, Expanded: true},
				{ID: "second", Title: "Second", Content: accordionSecond},
			},
			Expansion: ui.AccordionSingle,
		})
		r.check("accordion/create", func() error {
			return accordionErr
		})
		if accordion != nil {
			defer accordion.Close()
		}
	}
}
