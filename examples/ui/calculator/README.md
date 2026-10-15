# GameKit calculator

This example is a basic calculator built with native `ui` widgets. A column of
layouts holds the display and a keypad of buttons. The keypad grows with the
window.

The window uses the dark theme. Like the macOS calculator, the operator keys
are orange and the number keys are grey, with white text. Each button sets
`WidgetStyle.BackgroundColor`, `ForegroundColor`, and `CornerRadius`, which
make it a flat, rounded button that lightens while you press it.

You can click the buttons or type: digits, `.`, `+`, `-`, `*`, `/`, `%`, and
`=`. Return evaluates, Backspace deletes the last digit, and Escape clears.

The calculator applies each operator when you enter the next one, like a
pocket calculator: `12 + 3 × 2 =` shows 30. Results show twelve significant
digits, so 0.1 + 0.2 shows 0.3.

Run the calculator:

```sh
go run ./examples/ui/calculator
```
