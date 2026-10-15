package main

import (
	"math"
	"strconv"
	"strings"
)

// maximumDigits limits the number of digits that the user can enter.
const maximumDigits = 15

// operator is a pending binary operation.
type operator rune

const (
	operatorNone     operator = 0
	operatorAdd      operator = '+'
	operatorSubtract operator = '−'
	operatorMultiply operator = '×'
	operatorDivide   operator = '÷'
)

// calculator is an immediate-execution calculator, like a pocket calculator:
// each operator applies the pending operation before it starts the next one.
type calculator struct {
	// display is the text that the calculator shows.
	display string
	// accumulator is the left operand of the pending operation.
	accumulator float64
	pending     operator
	// isWaitingForOperand is set after an operator or equals: the next digit
	// starts a new number.
	isWaitingForOperand bool
	// hasError is set after a division by zero. The next key clears it.
	hasError bool
}

func newCalculator() *calculator {
	return &calculator{display: "0"}
}

// Display returns the text to show.
func (c *calculator) Display() string {
	return c.display
}

// EnterDigit appends digit to the number that the user enters.
func (c *calculator) EnterDigit(digit rune) {
	c.recoverFromError()
	if c.isWaitingForOperand || c.display == "0" {
		c.display = string(digit)
		c.isWaitingForOperand = false
		return
	}
	if countDigits(c.display) >= maximumDigits {
		return
	}
	c.display += string(digit)
}

// EnterDecimalPoint starts the fractional part of the number.
func (c *calculator) EnterDecimalPoint() {
	c.recoverFromError()
	if c.isWaitingForOperand {
		c.display = "0."
		c.isWaitingForOperand = false
		return
	}
	if !strings.Contains(c.display, ".") {
		c.display += "."
	}
}

// Backspace removes the last digit of the number that the user enters.
func (c *calculator) Backspace() {
	if c.hasError || c.isWaitingForOperand {
		return
	}
	c.display = c.display[:len(c.display)-1]
	if c.display == "" || c.display == "-" {
		c.display = "0"
	}
}

// Negate changes the sign of the displayed number.
func (c *calculator) Negate() {
	if c.hasError || c.display == "0" {
		return
	}
	if strings.HasPrefix(c.display, "-") {
		c.display = c.display[1:]
		return
	}
	c.display = "-" + c.display
}

// Percent divides the displayed number by one hundred.
func (c *calculator) Percent() {
	if c.hasError {
		return
	}
	c.show(c.value() / 100)
	c.isWaitingForOperand = true
}

// SetOperator applies the pending operation and makes op the next one.
// Two operators in sequence replace the first.
func (c *calculator) SetOperator(op operator) {
	if c.hasError {
		return
	}
	if c.pending != operatorNone && !c.isWaitingForOperand {
		c.evaluate()
		if c.hasError {
			return
		}
	} else if c.pending == operatorNone {
		c.accumulator = c.value()
	}
	c.pending = op
	c.isWaitingForOperand = true
}

// Equals applies the pending operation.
func (c *calculator) Equals() {
	if c.hasError || c.pending == operatorNone {
		return
	}
	c.evaluate()
	c.pending = operatorNone
	c.isWaitingForOperand = true
}

// Clear resets the calculator.
func (c *calculator) Clear() {
	*c = *newCalculator()
}

// evaluate applies the pending operation to the accumulator and the
// displayed number, and shows the result.
func (c *calculator) evaluate() {
	right := c.value()
	result := c.accumulator
	switch c.pending {
	case operatorAdd:
		result += right
	case operatorSubtract:
		result -= right
	case operatorMultiply:
		result *= right
	case operatorDivide:
		if right == 0 {
			c.showError()
			return
		}
		result /= right
	}
	if math.IsInf(result, 0) || math.IsNaN(result) {
		c.showError()
		return
	}
	c.accumulator = result
	c.show(result)
}

// value returns the displayed number.
func (c *calculator) value() float64 {
	value, err := strconv.ParseFloat(c.display, 64)
	if err != nil {
		return 0
	}
	return value
}

// show formats value with twelve significant digits. This removes the
// binary rounding error in results such as 0.1 + 0.2.
func (c *calculator) show(value float64) {
	if value == 0 {
		// Show "0", not "-0".
		value = 0
	}
	c.display = strconv.FormatFloat(value, 'g', 12, 64)
}

func (c *calculator) showError() {
	*c = *newCalculator()
	c.display = "Error"
	c.hasError = true
	c.isWaitingForOperand = true
}

// recoverFromError clears an error before the user enters a new number.
func (c *calculator) recoverFromError() {
	if c.hasError {
		c.Clear()
	}
}

func countDigits(text string) int {
	count := 0
	for _, char := range text {
		if char >= '0' && char <= '9' {
			count++
		}
	}
	return count
}
