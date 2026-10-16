package main

import (
	"fmt"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func valueCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Sliders", "Continuous, stepped with tick marks, inverted, and vertical sliders.", g.slidersDemo),
		g.card("Range slider", "Drag either thumb. The lower value cannot pass the upper value.", g.rangeSliderDemo),
		g.card("Number field and stepper", "The field and the stepper share one value. The second stepper wraps around.", g.numberFieldDemo),
		g.card("Dial", "A circular control that wraps from the maximum to the minimum.", g.dialDemo),
		g.card("Progress bars", "Determinate and indeterminate modes, the paused, warning, and error states, and a vertical bar.", g.progressBarsDemo),
		g.card("Activity indicators", "Every control size, with an optional label.", g.activityIndicatorsDemo),
	}
}

func (g *gallery) slidersDemo() ui.Widget {
	continuousValue := g.value("50%")
	continuous := must(g.window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 0.5}))
	continuous.OnChange(func(value float64) {
		continuousValue.SetText(fmt.Sprintf("%.0f%%", value*100))
	})

	steppedValue := g.value("4")
	stepped := must(g.window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 10, Value: 4, Step: 1}))
	g.attempt("Slider ticks", func() error {
		stepped.SetTicks(ui.SliderTicks{Frequency: 1, ShowLabels: true, SnapToTicks: true})
		return nil
	})
	stepped.OnChange(func(value float64) {
		steppedValue.SetText(fmt.Sprintf("%.0f", value))
	})

	invertedValue := g.value("25")
	inverted := must(g.window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 100, Value: 25, Step: 5}))
	g.attempt("Inverted slider", func() error {
		inverted.SetInverted(true)
		return nil
	})
	inverted.OnChange(func(value float64) {
		invertedValue.SetText(fmt.Sprintf("%.0f", value))
	})

	verticalValue := g.value("0.30")
	vertical := g.optional(func() ui.Widget {
		slider := must(g.window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 0.3}))
		slider.SetOrientation(ui.Vertical)
		slider.SetMinimumSize(ui.Size{Height: 140})
		slider.OnChange(func(value float64) {
			verticalValue.SetText(fmt.Sprintf("%.2f", value))
		})
		return slider
	})

	reset := g.button("Set all to minimum", func() {
		for _, slider := range []ui.Slider{continuous, stepped, inverted} {
			if err := slider.SetValue(slider.Minimum()); err != nil {
				g.log("Slider: %v", err)
			}
		}
		g.log("Sliders set to minimum; SetValue does not run OnChange")
	})
	outOfRange := g.button("Set out of range", func() {
		err := continuous.SetValue(2)
		g.log("SetValue(2) on a 0–1 slider: %v", err)
	})

	sliders := g.column(
		g.layout(ui.Row(ui.Item(g.label("Continuous")).MinimumSize(ui.Size{Width: 150}), ui.Item(continuous).Grow(1), ui.Item(continuousValue).MinimumSize(ui.Size{Width: 48})).WithGap(8)),
		g.layout(ui.Row(ui.Item(g.label("Stepped, ticks")).MinimumSize(ui.Size{Width: 150}), ui.Item(stepped).Grow(1), ui.Item(steppedValue).MinimumSize(ui.Size{Width: 48})).WithGap(8)),
		g.layout(ui.Row(ui.Item(g.label("Inverted")).MinimumSize(ui.Size{Width: 150}), ui.Item(inverted).Grow(1), ui.Item(invertedValue).MinimumSize(ui.Size{Width: 48})).WithGap(8)),
		g.row(reset, outOfRange, g.enabledToggle(continuous, stepped, inverted, vertical)),
	)
	return g.layout(ui.Row(
		ui.Item(sliders).Grow(1),
		ui.Item(g.column(vertical, verticalValue)).MinimumSize(ui.Size{Width: 70}),
	).WithGap(16))
}

func (g *gallery) rangeSliderDemo() ui.Widget {
	values := g.value("$200 – $800")
	slider := must(g.window.CreateRangeSlider(ui.RangeSliderOptions{
		Minimum: 0, Maximum: 1000, LowerValue: 200, UpperValue: 800, Step: 50,
	}))
	slider.OnChange(func(lower, upper float64) {
		values.SetText(fmt.Sprintf("$%.0f – $%.0f", lower, upper))
	})
	full := g.button("Full range", func() {
		if err := slider.SetValues(0, 1000); err != nil {
			g.log("Range: %v", err)
		}
		values.SetText(fmt.Sprintf("$%.0f – $%.0f", slider.LowerValue(), slider.UpperValue()))
	})
	crossed := g.button("Set crossed values", func() {
		g.log("SetValues(900, 100): %v", slider.SetValues(900, 100))
	})
	return g.column(
		g.layout(ui.Row(ui.Item(slider).Grow(1), ui.Item(values).MinimumSize(ui.Size{Width: 110})).WithGap(8)),
		g.row(full, crossed, g.enabledToggle(slider)),
	)
}

func (g *gallery) numberFieldDemo() ui.Widget {
	field := must(g.window.CreateNumberField(ui.NumberFieldOptions{
		Minimum: 0, Maximum: 10, Value: 2.5, Step: 0.25, Decimals: 2,
	}))
	stepper := must(g.window.CreateStepper(ui.StepperOptions{
		Minimum: 0, Maximum: 10, Value: 2.5, Step: 0.25,
	}))
	field.OnChange(func(value float64) {
		g.attempt("Stepper", func() error {
			return stepper.SetValue(value)
		})
		g.log("Number field: %.2f", value)
	})
	stepper.OnChange(func(value float64) {
		g.attempt("Number field", func() error {
			return field.SetValue(value)
		})
		g.log("Stepper: %.2f", value)
	})

	hour := g.value("Hour 23")
	wrapping := must(g.window.CreateStepper(ui.StepperOptions{
		Minimum: 0, Maximum: 23, Value: 23, Step: 1, Wrap: true,
	}))
	wrapping.OnChange(func(value float64) {
		hour.SetText(fmt.Sprintf("Hour %.0f", value))
	})
	increment := g.button("Increment", func() {
		wrapping.Increment()
		hour.SetText(fmt.Sprintf("Hour %.0f", wrapping.Value()))
	})
	decrement := g.button("Decrement", func() {
		wrapping.Decrement()
		hour.SetText(fmt.Sprintf("Hour %.0f", wrapping.Value()))
	})
	return g.column(
		g.labeled("Shared value", fixed(field, ui.Size{Width: 120}), stepper),
		g.labeled("Wrapping stepper", wrapping, hour, increment, decrement),
		g.states(field, stepper, wrapping),
	)
}

func (g *gallery) dialDemo() ui.Widget {
	angle := g.value("90°")
	dial := must(g.window.CreateDial(ui.DialOptions{
		Minimum: 0, Maximum: 360, Value: 90, Step: 15, Wrap: true,
	}))
	dial.SetMinimumSize(ui.Size{Width: 64, Height: 64})
	dial.OnChange(func(value float64) {
		angle.SetText(fmt.Sprintf("%.0f°", value))
	})
	north := g.button("North", func() {
		g.attempt("Dial", func() error {
			return dial.SetValue(0)
		})
		angle.SetText(fmt.Sprintf("%.0f°", dial.Value()))
	})
	return g.row(dial, angle, north, g.enabledToggle(dial))
}

func (g *gallery) progressBarsDemo() ui.Widget {
	determinate := must(g.window.CreateProgressBar(0.4))
	g.attempt("Progress text", func() error {
		determinate.SetText("40%")
		return nil
	})
	value := must(g.window.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 0.4}))
	value.OnChange(func(progress float64) {
		determinate.SetValue(progress)
		g.attempt("Progress text", func() error {
			determinate.SetText(fmt.Sprintf("%.0f%%", progress*100))
			return nil
		})
	})

	indeterminate := must(g.window.CreateProgressBar(0))
	g.attempt("Progress mode", func() error {
		indeterminate.SetMode(ui.ProgressIndeterminate)
		return nil
	})

	states := []ui.ProgressState{ui.ProgressNormal, ui.ProgressPaused, ui.ProgressWarning, ui.ProgressError}
	state := g.choice([]string{"Normal", "Paused", "Warning", "Error"}, 0, func(index int) {
		determinate.SetState(states[index])
		g.log("Progress state: %d", determinate.State())
	})
	mode := g.choice([]string{"Determinate", "Indeterminate"}, 0, func(index int) {
		determinate.SetMode([]ui.ProgressMode{ui.ProgressDeterminate, ui.ProgressIndeterminate}[index])
	})

	animated := must(g.window.CreateProgressBar(0))
	running := false
	animate := g.checkBox("Animate", false, func(checked bool) {
		running = checked
		if !checked {
			return
		}
		started := time.Now()
		g.every(func(now time.Time) bool {
			elapsed := now.Sub(started).Seconds()
			animated.SetValue(elapsed/4 - float64(int(elapsed/4)))
			return running
		})
	})

	vertical := g.optional(func() ui.Widget {
		bar := must(g.window.CreateProgressBar(0.7))
		bar.SetOrientation(ui.Vertical)
		bar.SetMinimumSize(ui.Size{Width: 24, Height: 120})
		return bar
	})

	bars := g.column(
		g.labeledWide("Determinate", determinate),
		g.labeledWide("Value", value),
		g.labeled("State and mode", state, mode),
		g.labeledWide("Indeterminate", indeterminate),
		g.layout(ui.Row(
			ui.Item(g.label("Animated")).Align(ui.AlignCenter).MinimumSize(ui.Size{Width: 150}),
			ui.Item(animated).Grow(1).Align(ui.AlignCenter),
			ui.Item(animate).Align(ui.AlignCenter),
		).WithGap(8)),
	)
	return g.layout(ui.Row(ui.Item(bars).Grow(1), ui.Item(vertical)).WithGap(16))
}

func (g *gallery) activityIndicatorsDemo() ui.Widget {
	sizes := []ui.ControlSize{ui.ControlSizeMini, ui.ControlSizeSmall, ui.ControlSizeRegular, ui.ControlSizeLarge}
	indicators := make([]ui.ActivityIndicator, len(sizes))
	widgets := make([]ui.Widget, len(sizes))
	for i, size := range sizes {
		indicator := must(g.window.CreateActivityIndicator(true))
		g.attempt("Indicator size", func() error {
			indicator.SetControlSize(size)
			return nil
		})
		indicators[i] = indicator
		widgets[i] = indicator
	}
	labeled := must(g.window.CreateActivityIndicator(true))
	g.attempt("Indicator label", func() error {
		labeled.SetLabel("Loading assets…")
		return nil
	})
	indicators = append(indicators, labeled)

	running := g.checkBox("Running", true, func(checked bool) {
		for _, indicator := range indicators {
			indicator.SetRunning(checked)
		}
		g.log("Activity running: %t", checked)
	})
	return g.column(g.row(widgets...), labeled, g.row(running))
}
