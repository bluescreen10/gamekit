package ui_test

import (
	"errors"
	"fmt"
	"image/color"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func checkExpectedComponents(r *recorder, window ui.Window) {
	checkExpectedCurrentComponentExtensions(r, window)
	checkExpectedToggleButton(r, window)
	checkExpectedSwitch(r, window)
	checkExpectedPasswordField(r, window)
	checkExpectedSearchField(r, window)
	checkExpectedNumberField(r, window)
	checkExpectedEditableCombo(r, window)
	checkExpectedSegmentedControl(r, window)
	checkExpectedStepper(r, window)
	checkExpectedRangeSlider(r, window)
	checkExpectedDateAndTime(r, window)
	checkExpectedColorAndFont(r, window)
	checkExpectedCalendarKeyRecorderAndDial(r, window)
	checkExpectedInfoBar(r, window)
	checkExpectedDataComponents(r, window)
	checkExpectedContainers(r, window)
	checkExpectedCommandComponents(r, window)
	checkExpectedFeedbackComponents(r, window)
	checkExpectedContentComponents(r, window)
	checkExpectedPlatformIntegrations(r, window)
}

func checkExpectedToggleButton(r *recorder, window ui.Window) {
	control, err := window.CreateToggleButton(ui.ToggleButtonOptions{
		Text:         "Snap",
		Checked:      true,
		GroupID:      "tools",
		Presentation: ui.ToggleButtonToolbar,
	})
	r.check("toggle_button/create", func() error {
		return err
	})
	if control == nil {
		return
	}
	defer control.Close()
	r.check("toggle_button/attributes", func() error {
		if control.Text() != "Snap" || !control.IsChecked() || control.GroupID() != "tools" {
			return errors.New("toggle button did not preserve its initial attributes")
		}
		control.SetText("Grid")
		control.SetChecked(false)
		if control.Text() != "Grid" || control.IsChecked() {
			return errors.New("toggle button setters did not update its state")
		}
		return nil
	})
}

func checkExpectedSwitch(r *recorder, window ui.Window) {
	control, err := window.CreateSwitch(ui.SwitchOptions{
		Text:     "Lighting",
		OnLabel:  "On",
		OffLabel: "Off",
		Checked:  true,
		Size:     ui.ControlSizeRegular,
	})
	r.check("switch/create", func() error {
		return err
	})
	if control == nil {
		return
	}
	defer control.Close()
	r.check("switch/attributes", func() error {
		if control.Text() != "Lighting" || !control.IsChecked() {
			return errors.New("switch did not preserve its initial state")
		}
		control.SetChecked(false)
		if control.IsChecked() {
			return errors.New("switch remained checked")
		}
		return nil
	})
}

func checkExpectedPasswordField(r *recorder, window ui.Window) {
	field, err := window.CreatePasswordField(ui.PasswordFieldOptions{
		Placeholder: "Password",
		Reveal:      ui.PasswordRevealButton,
		AllowCopy:   false,
	})
	r.check("password_field/create", func() error {
		return err
	})
	if field == nil {
		return
	}
	defer field.Close()
	r.check("password_field/state", func() error {
		field.SetText("secret")
		if field.Text() != "secret" || field.IsRevealed() {
			return errors.New("password field has an invalid text or reveal state")
		}
		if err := field.SetRevealed(true); err != nil {
			return err
		}
		if !field.IsRevealed() {
			return errors.New("password field did not reveal its text")
		}
		return nil
	})
}

func checkExpectedSearchField(r *recorder, window ui.Window) {
	field, err := window.CreateSearchField(ui.SearchFieldOptions{
		Text:        "native widgets",
		Placeholder: "Search",
		Suggestions: []string{"native", "widgets"},
	})
	r.check("search_field/create", func() error {
		return err
	})
	if field == nil {
		return
	}
	defer field.Close()
	r.check("search_field/state", func() error {
		if field.Text() != "native widgets" || field.Placeholder() != "Search" {
			return errors.New("search field did not preserve its initial state")
		}
		field.Clear()
		return wantEqual(field.Text(), "")
	})
}

func checkExpectedNumberField(r *recorder, window ui.Window) {
	field, err := window.CreateNumberField(ui.NumberFieldOptions{
		Minimum:  0,
		Maximum:  100,
		Value:    25,
		Step:     0.5,
		Decimals: 1,
	})
	r.check("number_field/create", func() error {
		return err
	})
	if field == nil {
		return
	}
	defer field.Close()
	r.check("number_field/state", func() error {
		if field.Minimum() != 0 || field.Maximum() != 100 || field.Step() != 0.5 {
			return errors.New("number field did not preserve its range")
		}
		if err := field.SetValue(50.5); err != nil {
			return err
		}
		return wantEqual(field.Value(), 50.5)
	})
}

func checkExpectedEditableCombo(r *recorder, window ui.Window) {
	combo, err := window.CreateEditableCombo(ui.EditableComboOptions{
		Text:          "Custom",
		Items:         []string{"One", "Two"},
		AllowFreeText: true,
	})
	r.check("editable_combo/create", func() error {
		return err
	})
	if combo == nil {
		return
	}
	defer combo.Close()
	r.check("editable_combo/state", func() error {
		combo.SetText("Two")
		if combo.Text() != "Two" || combo.SelectedIndex() != 1 {
			return errors.New("editable combo did not select the matching item")
		}
		combo.SetText("Other")
		return wantEqual(combo.SelectedIndex(), -1)
	})
}

func checkExpectedSegmentedControl(r *recorder, window ui.Window) {
	control, err := window.CreateSegmentedControl(ui.SegmentedControlOptions{
		Items: []ui.Segment{
			{ID: "left", Text: "Left", Enabled: true},
			{ID: "right", Text: "Right", Enabled: true},
		},
		SelectedIDs: []string{"left"},
		Mode:        ui.SelectionSingle,
	})
	r.check("segmented_control/create", func() error {
		return err
	})
	if control == nil {
		return
	}
	defer control.Close()
	r.check("segmented_control/selection", func() error {
		if err := control.SetSelectedIDs([]string{"right"}); err != nil {
			return err
		}
		selected := control.SelectedIDs()
		if len(selected) != 1 || selected[0] != "right" {
			return fmt.Errorf("SelectedIDs() = %#v", selected)
		}
		return nil
	})
}

func checkExpectedStepper(r *recorder, window ui.Window) {
	control, err := window.CreateStepper(ui.StepperOptions{
		Minimum: 0,
		Maximum: 10,
		Value:   5,
		Step:    1,
		Wrap:    true,
	})
	r.check("stepper/create", func() error {
		return err
	})
	if control == nil {
		return
	}
	defer control.Close()
	r.check("stepper/state", func() error {
		if err := control.SetValue(10); err != nil {
			return err
		}
		control.Increment()
		return wantEqual(control.Value(), 0.0)
	})
}

func checkExpectedRangeSlider(r *recorder, window ui.Window) {
	control, err := window.CreateRangeSlider(ui.RangeSliderOptions{
		Minimum:    0,
		Maximum:    100,
		LowerValue: 20,
		UpperValue: 80,
		Step:       5,
	})
	r.check("range_slider/create", func() error {
		return err
	})
	if control == nil {
		return
	}
	defer control.Close()
	r.check("range_slider/state", func() error {
		if err := control.SetValues(30, 70); err != nil {
			return err
		}
		if control.LowerValue() != 30 || control.UpperValue() != 70 {
			return errors.New("range slider values were not preserved")
		}
		return nil
	})
}

func checkExpectedDateAndTime(r *recorder, window ui.Window) {
	date := time.Date(2026, time.October, 14, 0, 0, 0, 0, time.UTC)
	datePicker, dateErr := window.CreateDatePicker(ui.DatePickerOptions{Value: date})
	r.check("date_picker/create", func() error {
		return dateErr
	})
	if datePicker != nil {
		defer datePicker.Close()
		r.check("date_picker/value", func() error {
			return wantEqual(datePicker.Value(), date)
		})
	}

	timePicker, timeErr := window.CreateTimePicker(ui.TimePickerOptions{
		Value:       15*time.Hour + 30*time.Minute,
		ShowSeconds: true,
		Clock:       ui.Clock24Hour,
	})
	r.check("time_picker/create", func() error {
		return timeErr
	})
	if timePicker != nil {
		defer timePicker.Close()
		r.check("time_picker/value", func() error {
			return wantEqual(timePicker.Value(), 15*time.Hour+30*time.Minute)
		})
	}
}

func checkExpectedColorAndFont(r *recorder, window ui.Window) {
	colorPicker, colorErr := window.CreateColorPicker(ui.ColorPickerOptions{
		Color:      color.RGBA{R: 20, G: 40, B: 60, A: 128},
		AllowAlpha: true,
	})
	r.check("color_picker/create", func() error {
		return colorErr
	})
	if colorPicker != nil {
		defer colorPicker.Close()
	}

	fontPicker, fontErr := window.CreateFontPicker(ui.FontPickerOptions{
		Font: ui.Font{Family: "System", Size: 14},
	})
	r.check("font_picker/create", func() error {
		return fontErr
	})
	if fontPicker != nil {
		defer fontPicker.Close()
	}
}

func checkExpectedCalendarKeyRecorderAndDial(r *recorder, window ui.Window) {
	date := time.Date(2026, time.October, 14, 0, 0, 0, 0, time.UTC)
	calendar, calendarErr := window.CreateCalendar(ui.CalendarOptions{
		VisibleMonth:  date,
		SelectedDates: []time.Time{date},
		Selection:     ui.CalendarRange,
	})
	checkExpectedCreation(r, "calendar", calendar, calendarErr)
	if calendar != nil {
		r.check("calendar/visible_month", func() error {
			if err := skipOnDarwin("NSDatePicker always shows the month of its date"); err != nil {
				return err
			}
			month := time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC)
			calendar.SetVisibleMonth(month)
			if !calendar.VisibleMonth().Equal(month) {
				return fmt.Errorf("VisibleMonth() = %v, want %v", calendar.VisibleMonth(), month)
			}
			if dates := calendar.SelectedDates(); len(dates) == 0 || !dates[0].Equal(date) {
				return fmt.Errorf("SetVisibleMonth changed the selection to %v", dates)
			}
			return nil
		})
	}
	r.check("calendar/multiple_selection", func() error {
		if err := skipOnDarwin("NSDatePicker selects one date or one range, not disjoint dates"); err != nil {
			return err
		}
		dates := []time.Time{date, date.AddDate(0, 0, 3)}
		multiple, err := window.CreateCalendar(ui.CalendarOptions{SelectedDates: dates, Selection: ui.CalendarMultiple})
		if err != nil {
			return err
		}
		defer multiple.Close()
		got := multiple.SelectedDates()
		if len(got) != len(dates) || !got[0].Equal(dates[0]) || !got[1].Equal(dates[1]) {
			return fmt.Errorf("SelectedDates() = %v, want %v", got, dates)
		}
		return nil
	})

	recorder, recorderErr := window.CreateKeyRecorder(ui.KeyRecorderOptions{
		Shortcut: ui.Shortcut{Key: "K", Command: true, Shift: true},
	})
	checkExpectedCreation(r, "key_recorder", recorder, recorderErr)

	dial, dialErr := window.CreateDial(ui.DialOptions{
		Minimum: 0,
		Maximum: 360,
		Value:   90,
		Step:    1,
		Wrap:    true,
	})
	checkExpectedCreation(r, "dial", dial, dialErr)
}
