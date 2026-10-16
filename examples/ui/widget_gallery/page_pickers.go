package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func pickerCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Select", "A plain list, choices with descriptions and a disabled choice, and a list with a placeholder.", g.selectDemo),
		g.card("Date and time pickers", "The date picker accepts dates within 30 days of today.", g.dateTimeDemo),
		g.card("Calendars", "Single dates, ranges, and several dates.", g.calendarsDemo),
		g.card("Color picker", "With and without the alpha channel.", g.colorPickerDemo),
		g.card("Font picker", "The preview label uses the selected font.", g.fontPickerDemo),
	}
}

func (g *gallery) selectDemo() ui.Widget {
	filters := []string{"Nearest", "Linear", "Anisotropic 4×", "Anisotropic 16×"}
	plain := g.choice(filters, 1, func(index int) {
		g.log("Texture filter: %s", filters[index])
	})

	described := must(g.window.CreateSelect(ui.SelectOptions{Items: []string{"Vulkan"}, SelectedIndex: 0}))
	g.attempt("Select choices", func() error {
		return described.SetChoices([]ui.Choice{
			{ID: "metal", Text: "Metal", Description: "Apple platforms", Enabled: true},
			{ID: "vulkan", Text: "Vulkan", Description: "Windows and Linux", Enabled: true},
			{ID: "d3d12", Text: "Direct3D 12", Description: "Windows only", Enabled: true},
			{ID: "opengl", Text: "OpenGL", Description: "Deprecated", Enabled: false},
		})
	})
	described.OnChange(func(index int) {
		g.log("Graphics API: %s", described.Choices()[index].Text)
	})

	placeholder := must(g.window.CreateSelect(ui.SelectOptions{
		Items: []string{"Easy", "Normal", "Hard", "Nightmare"}, SelectedIndex: 0,
	}))
	g.attempt("Placeholder", func() error {
		placeholder.SetPlaceholder("Choose a difficulty")
		return nil
	})
	placeholder.OnChange(func(index int) {
		g.log("Difficulty: %d", index)
	})

	many := make([]string, 40)
	for i := range many {
		many[i] = fmt.Sprintf("Level %02d", i+1)
	}
	limited := must(g.window.CreateSelect(ui.SelectOptions{Items: many, SelectedIndex: 0}))
	g.attempt("Visible rows", func() error {
		limited.SetVisibleRowCount(8)
		limited.SetPopupWidth(240)
		return nil
	})
	limited.OnChange(func(index int) {
		g.log("Level: %s", many[index])
	})
	return g.column(
		g.labeled("Plain", plain),
		g.labeled("Choices", described),
		g.labeled("Placeholder", placeholder),
		g.labeled("Eight visible rows", limited),
		g.states(plain, described, placeholder, limited),
	)
}

func (g *gallery) dateTimeDemo() ui.Widget {
	today := time.Now()
	date := must(g.window.CreateDatePicker(ui.DatePickerOptions{
		Value:   today,
		Minimum: today.AddDate(0, 0, -30),
		Maximum: today.AddDate(0, 0, 30),
	}))
	date.OnChange(func(value time.Time) {
		g.log("Date: %s", value.Format("Monday, 2 January 2006"))
	})
	outside := g.button("Set a date next year", func() {
		g.log("SetValue next year: %v", date.SetValue(today.AddDate(1, 0, 0)))
	})

	twelveHour := must(g.window.CreateTimePicker(ui.TimePickerOptions{
		Value: 9*time.Hour + 30*time.Minute,
		Clock: ui.Clock12Hour,
	}))
	twelveHour.OnChange(func(value time.Duration) {
		g.log("12-hour time: %s", formatClock(value))
	})
	twentyFourHour := must(g.window.CreateTimePicker(ui.TimePickerOptions{
		Value:       21*time.Hour + 15*time.Minute + 30*time.Second,
		ShowSeconds: true,
		Clock:       ui.Clock24Hour,
	}))
	twentyFourHour.OnChange(func(value time.Duration) {
		g.log("24-hour time: %s", formatClock(value))
	})
	system := must(g.window.CreateTimePicker(ui.TimePickerOptions{Value: 12 * time.Hour}))
	system.OnChange(func(value time.Duration) {
		g.log("System clock time: %s", formatClock(value))
	})
	return g.column(
		g.labeled("Date", date, outside),
		g.labeled("12-hour clock", twelveHour),
		g.labeled("24-hour with seconds", twentyFourHour),
		g.labeled("System clock", system),
		g.states(date, twelveHour, twentyFourHour, system),
	)
}

func formatClock(value time.Duration) string {
	hours := int(value.Hours())
	minutes := int(value.Minutes()) % 60
	seconds := int(value.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func (g *gallery) calendarsDemo() ui.Widget {
	today := time.Now()
	modes := []struct {
		title     string
		selection ui.CalendarSelection
		dates     []time.Time
	}{
		{"Single date", ui.CalendarSingle, []time.Time{today}},
		{"Range", ui.CalendarRange, []time.Time{today, today.AddDate(0, 0, 4)}},
		{"Several dates", ui.CalendarMultiple, []time.Time{today, today.AddDate(0, 0, 2), today.AddDate(0, 0, 9)}},
	}
	columns := make([]ui.LayoutItem, len(modes))
	for i, mode := range modes {
		calendar := g.optional(func() ui.Widget {
			calendar := must(g.window.CreateCalendar(ui.CalendarOptions{
				VisibleMonth:  today,
				SelectedDates: mode.dates,
				Selection:     mode.selection,
			}))
			calendar.OnChange(func(dates []time.Time) {
				g.log("%s: %s", mode.title, formatDates(dates))
			})
			return calendar
		})
		columns[i] = ui.Item(g.column(g.label(mode.title), calendar)).Grow(1)
	}
	return g.layout(ui.Row(columns...).WithGap(16))
}

func formatDates(dates []time.Time) string {
	texts := make([]string, len(dates))
	for i, date := range dates {
		texts[i] = date.Format("2 Jan")
	}
	return strings.Join(texts, ", ")
}

func (g *gallery) colorPickerDemo() ui.Widget {
	opaque := must(g.window.CreateColorPicker(ui.ColorPickerOptions{
		Color: color.RGBA{R: 0x2f, G: 0x80, B: 0xed, A: 0xff},
	}))
	opaqueValue := g.value(formatColor(opaque.Color()))
	opaque.OnChange(func(value color.RGBA) {
		opaqueValue.SetText(formatColor(value))
	})

	translucent := must(g.window.CreateColorPicker(ui.ColorPickerOptions{
		Color:      color.RGBA{R: 0xeb, G: 0x57, B: 0x57, A: 0x80},
		AllowAlpha: true,
	}))
	translucentValue := g.value(formatColor(translucent.Color()))
	translucent.OnChange(func(value color.RGBA) {
		translucentValue.SetText(formatColor(value))
	})
	reset := g.button("Reset to green", func() {
		opaque.SetColor(color.RGBA{G: 0xa0, B: 0x40, A: 0xff})
		opaqueValue.SetText(formatColor(opaque.Color()))
	})
	return g.column(
		g.labeled("Opaque", opaque, opaqueValue, reset),
		g.labeled(fmt.Sprintf("Alpha (%t)", translucent.AllowsAlpha()), translucent, translucentValue),
		g.states(opaque, translucent),
	)
}

func formatColor(value color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X%02X", value.R, value.G, value.B, value.A)
}

func (g *gallery) fontPickerDemo() ui.Widget {
	initial := ui.Font{Size: 18, Weight: ui.FontWeightRegular}
	picker := must(g.window.CreateFontPicker(ui.FontPickerOptions{Font: initial}))
	preview := g.label("The quick brown fox jumps over the lazy dog")
	description := g.value(formatFont(initial))
	apply := func(font ui.Font) {
		description.SetText(formatFont(font))
		g.attempt("Preview font", func() error {
			return preview.SetStyle(ui.WidgetStyle{Font: font})
		})
	}
	apply(initial)
	picker.OnChange(func(font ui.Font) {
		apply(font)
		g.log("Font: %s", formatFont(font))
	})
	bold := g.button("Bold italic", func() {
		font := picker.Font()
		font.Weight = ui.FontWeightBold
		font.Italic = true
		if err := picker.SetFont(font); err != nil {
			g.log("Font: %v", err)
			return
		}
		apply(picker.Font())
	})
	return g.column(g.row(picker, description, bold), preview, g.row(g.enabledToggle(picker)))
}

func formatFont(font ui.Font) string {
	style := ""
	if font.Italic {
		style = " italic"
	}
	family := font.Family
	if family == "" {
		family = "System font"
	}
	return fmt.Sprintf("%s %.0f pt, weight %d%s", family, font.Size, font.Weight, style)
}
