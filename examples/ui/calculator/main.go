// Command calculator is a basic calculator built with native ui widgets.
package main

import (
	"image/color"
	"runtime"
	"time"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui"
	_ "github.com/bluescreen10/gamekit/ui/default"
)

// keyKind selects the colors of a button, like the macOS calculator.
type keyKind uint8

const (
	keyDigit keyKind = iota
	keyFunction
	keyOperator
)

// keyColors are the background colors of the button kinds. All buttons use
// white text.
var keyColors = map[keyKind]color.RGBA{
	keyDigit:    {R: 80, G: 80, B: 82, A: 255},
	keyFunction: {R: 112, G: 112, B: 115, A: 255},
	keyOperator: {R: 255, G: 159, B: 10, A: 255},
}

const (
	keyFontSize     = 28
	keyGap          = 6
	keyRadius       = 16
	displayFontSize = 56
)

// key is one calculator button: its label, its colors, and what it does.
type key struct {
	label string
	kind  keyKind
	press func(c *calculator)
}

// keypad lists the buttons row by row.
var keypad = [][]key{
	{
		{label: "AC", kind: keyFunction, press: (*calculator).Clear},
		{label: "±", kind: keyFunction, press: (*calculator).Negate},
		{label: "%", kind: keyFunction, press: (*calculator).Percent},
		operatorKey(operatorDivide),
	},
	{digitKey('7'), digitKey('8'), digitKey('9'), operatorKey(operatorMultiply)},
	{digitKey('4'), digitKey('5'), digitKey('6'), operatorKey(operatorSubtract)},
	{digitKey('1'), digitKey('2'), digitKey('3'), operatorKey(operatorAdd)},
	{
		digitKey('0'),
		{label: ".", kind: keyDigit, press: (*calculator).EnterDecimalPoint},
		{label: "⌫", kind: keyFunction, press: (*calculator).Backspace},
		{label: "=", kind: keyOperator, press: (*calculator).Equals},
	},
}

func digitKey(digit rune) key {
	return key{
		label: string(digit),
		kind:  keyDigit,
		press: func(c *calculator) {
			c.EnterDigit(digit)
		},
	}
}

func operatorKey(op operator) key {
	return key{
		label: string(op),
		kind:  keyOperator,
		press: func(c *calculator) {
			c.SetOperator(op)
		},
	}
}

func main() {
	// AppKit requires the UI on the main thread.
	runtime.LockOSThread()

	backend, err := ui.New(nil)
	if err != nil {
		panic(err)
	}
	window, err := backend.CreateWindow(ui.WindowOptions{
		Title:       "Calculator",
		Size:        ui.Size{Width: 320, Height: 500},
		MinimumSize: ui.Size{Width: 260, Height: 400},
		Resizable:   true,
		Theme:       ui.ThemeDark,
	})
	if err != nil {
		panic(err)
	}
	defer window.Close()

	if err := buildCalculator(window, newCalculator()); err != nil {
		panic(err)
	}
	for !window.ShouldClose() {
		window.PollEvents()
		time.Sleep(time.Second / 60)
	}
}

// buildCalculator creates the display and the keypad, and connects the
// buttons and the keyboard to c.
func buildCalculator(window ui.Window, c *calculator) error {
	display, err := createDisplay(window)
	if err != nil {
		return err
	}
	display.SetText(c.Display())
	press := func(action func(c *calculator)) {
		action(c)
		display.SetText(c.Display())
	}

	rows := []ui.LayoutItem{ui.Item(display)}
	for _, keys := range keypad {
		row, err := createKeyRow(window, keys, press)
		if err != nil {
			return err
		}
		rows = append(rows, ui.Item(row).Grow(1))
	}
	root, err := window.CreateLayout(ui.Column(rows...).WithPadding(ui.UniformInsets(10)).WithGap(keyGap))
	if err != nil {
		return err
	}
	if err := window.SetContent(root); err != nil {
		return err
	}

	connectKeyboard(window, press)
	return nil
}

func createDisplay(window ui.Window) (ui.Label, error) {
	display, err := window.CreateLabel("0")
	if err != nil {
		return nil, err
	}
	display.SetTextAlignment(ui.TextAlignEnd)
	display.SetTruncation(ui.TruncateHead)
	if err := display.SetStyle(ui.WidgetStyle{Font: ui.Font{Size: displayFontSize, Weight: 300}}); err != nil {
		return nil, err
	}
	// Assistive technologies read each new result.
	if err := display.SetAccessibility(ui.Accessibility{Name: "Result", Live: ui.LivePolite}); err != nil {
		return nil, err
	}
	return display, nil
}

func createKeyRow(window ui.Window, keys []key, press func(func(c *calculator))) (ui.Layout, error) {
	items := make([]ui.LayoutItem, len(keys))
	for i, key := range keys {
		button, err := window.CreateButton(key.label)
		if err != nil {
			return nil, err
		}
		err = button.SetStyle(ui.WidgetStyle{
			Font:            ui.Font{Size: keyFontSize, Weight: ui.FontWeightMedium},
			ForegroundColor: color.White,
			BackgroundColor: keyColors[key.kind],
			CornerRadius:    keyRadius,
		})
		if err != nil {
			return nil, err
		}
		button.OnClick(func() {
			press(key.press)
		})
		items[i] = ui.Item(button).Grow(1)
	}
	return window.CreateLayout(ui.Row(items...).WithGap(keyGap))
}

// connectKeyboard lets the user type digits and operators. Return and "="
// evaluate, Backspace deletes, and Escape clears.
func connectKeyboard(window ui.Window, press func(func(c *calculator))) {
	window.OnText(func(char rune) {
		action := textAction(char)
		if action != nil {
			press(action)
		}
	})
	window.OnKey(func(event ui.KeyEvent) {
		if event.Action == keyboard.KeyReleased {
			return
		}
		switch event.Key {
		case keyboard.KeyEnter, keyboard.KeyKPEnter:
			press((*calculator).Equals)
		case keyboard.KeyBackspace:
			press((*calculator).Backspace)
		case keyboard.KeyEscape:
			press((*calculator).Clear)
		}
	})
}

// textAction returns the calculator action for a typed character, or nil.
func textAction(char rune) func(c *calculator) {
	switch char {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return digitKey(char).press
	case '.', ',':
		return (*calculator).EnterDecimalPoint
	case '+':
		return operatorKey(operatorAdd).press
	case '-':
		return operatorKey(operatorSubtract).press
	case '*', 'x':
		return operatorKey(operatorMultiply).press
	case '/':
		return operatorKey(operatorDivide).press
	case '%':
		return (*calculator).Percent
	case '=':
		return (*calculator).Equals
	}
	return nil
}
