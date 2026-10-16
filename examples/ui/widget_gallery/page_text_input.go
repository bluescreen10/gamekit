package main

import (
	"fmt"
	"strings"

	"github.com/bluescreen10/gamekit/ui"
)

func textInputCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Text fields", "Placeholder, prefix and suffix, a length limit, alignment, and the read-only state. Press Return to submit.", g.textFieldsDemo),
		g.card("Validation", "The message and the invalid state follow the text as you type.", g.textValidationDemo),
		g.card("Password field", "The reveal button and SetRevealed show the text. Copying is blocked while the text is hidden.", g.passwordFieldDemo),
		g.card("Search field", "Suggestions appear as you type. Press Return to search.", g.searchFieldDemo),
		g.card("Text area", "Wrap modes, the Tab key policy, spell checking, and the read-only state.", g.textAreaDemo),
		g.card("Editable combo box", "Pick a suggestion or type free text.", g.editableComboDemo),
		g.card("Key recorder", "Click the recorder and press a key combination.", g.keyRecorderDemo),
	}
}

func (g *gallery) textFieldsDemo() ui.Widget {
	plain := must(g.window.CreateTextField())
	plain.SetPlaceholder("Type here")
	plain.OnChange(func(text string) {
		g.log("Text field: %q", text)
	})
	plain.OnSubmit(func(text string) {
		g.log("Submitted: %q", text)
	})

	affixed := must(g.window.CreateTextField())
	affixed.SetPlaceholder("example")
	g.attempt("Prefix and suffix", func() error {
		affixed.SetPrefix("https://")
		affixed.SetSuffix(".com")
		affixed.SetInputPurpose(ui.InputPurposeURL)
		return nil
	})
	affixed.OnChange(func(text string) {
		g.log("URL: %s%s%s", affixed.Prefix(), text, affixed.Suffix())
	})

	counter := g.value("0 / 8")
	limited := must(g.window.CreateTextField())
	limited.SetPlaceholder("Eight characters")
	g.attempt("Maximum length", func() error {
		limited.SetMaximumLength(8)
		return nil
	})
	limited.OnChange(func(text string) {
		counter.SetText(fmt.Sprintf("%d / %d", len([]rune(text)), limited.MaximumLength()))
	})

	aligned := must(g.window.CreateTextField())
	aligned.SetText("Aligned text")
	alignment := g.choice([]string{"Start", "Center", "End"}, 0, func(index int) {
		aligned.SetTextAlignment([]ui.TextAlignment{ui.TextAlignStart, ui.TextAlignCenter, ui.TextAlignEnd}[index])
	})

	selectAll := g.button("Select all", func() {
		if err := plain.Focus(); err != nil {
			g.log("Focus: %v", err)
		}
		plain.SetSelection(ui.TextRange{Start: 0, End: len([]rune(plain.Text()))})
		g.log("Selection: %+v", plain.Selection())
	})
	readOnly := g.checkBox("Read-only", false, func(checked bool) {
		for _, field := range []ui.TextField{plain, affixed, limited, aligned} {
			field.SetReadOnly(checked)
		}
		g.log("Read-only: %t", checked)
	})
	spelling := g.checkBox("Spell checking", true, func(checked bool) {
		plain.SetSpellCheckEnabled(checked)
		g.log("Spell checking: %t", plain.IsSpellCheckEnabled())
	})
	return g.column(
		g.labeledWide("Placeholder", plain),
		g.labeledWide("Prefix and suffix", affixed),
		g.labeled("Length limit", fixed(limited, ui.Size{Width: 200}), counter),
		g.labeled("Alignment", fixed(aligned, ui.Size{Width: 240}), alignment),
		g.row(selectAll, readOnly, spelling, g.enabledToggle(plain, affixed, limited, aligned)),
	)
}

func (g *gallery) textValidationDemo() ui.Widget {
	email := must(g.window.CreateTextField())
	email.SetPlaceholder("name@example.com")
	g.attempt("Input purpose", func() error {
		email.SetInputPurpose(ui.InputPurposeEmail)
		return nil
	})
	message := must(g.window.CreateInlineValidation(ui.InlineValidationOptions{
		Severity: ui.SeverityInformation,
		Text:     "Enter an e-mail address.",
	}))
	validate := func(text string) {
		severity, detail, invalid := ui.SeveritySuccess, "The address looks valid.", false
		switch {
		case text == "":
			severity, detail = ui.SeverityInformation, "Enter an e-mail address."
		case !strings.Contains(text, "@"):
			severity, detail, invalid = ui.SeverityError, "The address needs an @ sign.", true
		case !strings.Contains(text[strings.Index(text, "@"):], "."):
			severity, detail = ui.SeverityWarning, "The domain has no dot. Is it complete?"
		}
		message.SetSeverity(severity)
		message.SetText(detail)
		g.attempt("Invalid state", func() error {
			return email.SetAccessibility(ui.Accessibility{
				Name:        "E-mail address",
				IsRequired:  true,
				IsInvalid:   invalid,
				DescribedBy: []ui.Widget{message},
			})
		})
	}
	email.OnChange(func(text string) {
		g.attempt("Validate", func() error {
			validate(text)
			return nil
		})
	})
	return g.column(g.labeledWide("E-mail", email), message)
}

func (g *gallery) passwordFieldDemo() ui.Widget {
	password := must(g.window.CreatePasswordField(ui.PasswordFieldOptions{
		Placeholder: "Password",
		Reveal:      ui.PasswordRevealButton,
		AllowCopy:   false,
	}))
	strength := g.value("Strength: empty")
	password.OnChange(func(text string) {
		strength.SetText("Strength: " + passwordStrength(text))
	})
	password.OnSubmit(func(text string) {
		g.log("Password submitted (%d characters)", len([]rune(text)))
	})
	reveal := g.checkBox("Reveal", false, func(checked bool) {
		if err := password.SetRevealed(checked); err != nil {
			g.log("Reveal: %v", err)
			return
		}
		g.log("Password revealed: %t", password.IsRevealed())
	})

	hidden := must(g.window.CreatePasswordField(ui.PasswordFieldOptions{
		Placeholder: "No reveal button",
		Reveal:      ui.PasswordRevealNone,
		AllowCopy:   true,
	}))
	return g.column(
		g.labeled("With reveal button", fixed(password, ui.Size{Width: 260}), strength),
		g.labeled("Without", fixed(hidden, ui.Size{Width: 260})),
		g.row(reveal, g.enabledToggle(password, hidden)),
	)
}

func passwordStrength(password string) string {
	switch length := len([]rune(password)); {
	case length == 0:
		return "empty"
	case length < 8:
		return "weak"
	case length < 14:
		return "fair"
	default:
		return "strong"
	}
}

func (g *gallery) searchFieldDemo() ui.Widget {
	fruits := []string{"Apple", "Apricot", "Banana", "Blackberry", "Blueberry", "Cherry", "Grape", "Mango", "Melon", "Peach"}
	search := must(g.window.CreateSearchField(ui.SearchFieldOptions{
		Placeholder: "Search fruit",
		Suggestions: fruits,
	}))
	results := g.value(fmt.Sprintf("%d matches", len(fruits)))
	search.OnChange(func(text string) {
		matches := []string{}
		for _, fruit := range fruits {
			if strings.Contains(strings.ToLower(fruit), strings.ToLower(text)) {
				matches = append(matches, fruit)
			}
		}
		search.SetSuggestions(matches)
		results.SetText(fmt.Sprintf("%d matches", len(matches)))
	})
	search.OnSubmit(func(text string) {
		g.log("Search: %q", text)
	})
	clear := g.button("Clear", func() {
		search.Clear()
		g.log("Search cleared: %q", search.Text())
	})
	return g.column(
		g.labeled("Search", fixed(search, ui.Size{Width: 280}), results),
		g.row(clear, g.enabledToggle(search)),
	)
}

func (g *gallery) textAreaDemo() ui.Widget {
	area := must(g.window.CreateTextArea(ui.TextAreaOptions{
		Text: "A native multiline text area. This first line is long enough to show how each wrap mode " +
			"treats text that does not fit in the width of the view.\n\nPress Tab to move the focus or to insert a tab.",
		Placeholder: "Write something",
		Wrap:        true,
	}))
	tall(area, 160)
	count := g.value(fmt.Sprintf("%d characters", len([]rune(area.Text()))))
	area.OnChange(func(text string) {
		count.SetText(fmt.Sprintf("%d characters", len([]rune(text))))
	})

	wrap := g.choice([]string{"Wrap words", "Wrap characters", "No wrapping"}, 0, func(index int) {
		area.SetWrapMode([]ui.WrapMode{ui.WrapWords, ui.WrapCharacters, ui.WrapNone}[index])
	})
	tabs := g.choice([]string{"Tab moves focus", "Tab inserts a tab"}, 0, func(index int) {
		area.SetTabPolicy([]ui.TabPolicy{ui.TabMovesFocus, ui.TabInsert}[index])
	})
	limit := g.choice([]string{"No length limit", "Limit to 200"}, 0, func(index int) {
		area.SetMaximumLength([]int{0, 200}[index])
	})
	readOnly := g.checkBox("Read-only", false, func(checked bool) {
		area.SetReadOnly(checked)
	})
	spelling := g.checkBox("Spell checking", true, func(checked bool) {
		area.SetSpellCheckEnabled(checked)
	})
	top := g.button("Scroll to top", func() {
		area.SetScrollPosition(ui.Point{})
	})
	return g.column(
		area,
		g.row(wrap, tabs, limit, count),
		g.row(readOnly, spelling, top, g.enabledToggle(area)),
	)
}

func (g *gallery) editableComboDemo() ui.Widget {
	free := must(g.window.CreateEditableCombo(ui.EditableComboOptions{
		Text:          "Medium",
		Items:         []string{"Small", "Medium", "Large"},
		AllowFreeText: true,
	}))
	free.OnChange(func(text string) {
		g.log("Size: %q (item %d)", text, free.SelectedIndex())
	})
	strict := must(g.window.CreateEditableCombo(ui.EditableComboOptions{
		Text:  "Arial",
		Items: []string{"Arial", "Helvetica", "Menlo", "Times"},
	}))
	strict.OnChange(func(text string) {
		g.log("Font: %q (item %d)", text, strict.SelectedIndex())
	})
	return g.column(
		g.labeled("Free text", fixed(free, ui.Size{Width: 220})),
		g.labeled("Items only", fixed(strict, ui.Size{Width: 220})),
		g.states(free, strict),
	)
}

func (g *gallery) keyRecorderDemo() ui.Widget {
	recorder := must(g.window.CreateKeyRecorder(ui.KeyRecorderOptions{
		Shortcut: ui.Shortcut{Key: "P", Command: true, Shift: true},
	}))
	current := g.value(describeShortcut(recorder.Shortcut()))
	recorder.OnChange(func(shortcut ui.Shortcut) {
		current.SetText(describeShortcut(shortcut))
		g.log("Recorded shortcut: %s", describeShortcut(shortcut))
	})
	reset := g.button("Reset", func() {
		recorder.SetShortcut(ui.Shortcut{Key: "P", Command: true, Shift: true})
		current.SetText(describeShortcut(recorder.Shortcut()))
	})
	clear := g.button("Clear", func() {
		recorder.SetShortcut(ui.Shortcut{})
		current.SetText(describeShortcut(recorder.Shortcut()))
	})
	return g.column(
		g.labeled("Shortcut", fixed(recorder, ui.Size{Width: 200}), current),
		g.row(reset, clear, g.enabledToggle(recorder)),
	)
}
