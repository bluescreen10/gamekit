package main

import (
	"github.com/bluescreen10/gamekit/ui"
)

// longText is a paragraph that is long enough to show wrapping and
// truncation.
const longText = "GameKit places native controls with its own layout code, so every backend " +
	"shows the same arrangement with the look of its platform. This paragraph is long enough " +
	"to wrap onto several lines, which shows the wrap modes, the truncation styles, and the line limit."

func textDisplayCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Semantic styles", "Labels in each text style of the platform.", g.semanticStylesDemo),
		g.card("Alignment, wrapping, and truncation", "Change the options to see how the label lays out a long paragraph.", g.labelLayoutDemo),
		g.card("Rich text", "An RTF document. Turn on editing to change it.", g.richTextDemo),
		g.card("Markdown", "Edit the source on the left. Click a link to report it.", g.markdownDemo),
	}
}

func (g *gallery) semanticStylesDemo() ui.Widget {
	styles := []struct {
		text  string
		style ui.TextStyle
	}{
		{"Title style", ui.TextStyleTitle},
		{"Heading style", ui.TextStyleHeading},
		{"Body style, for most text", ui.TextStyleBody},
		{"Caption style, for secondary text", ui.TextStyleCaption},
		{"Monospace style: 0123456789", ui.TextStyleMonospace},
	}
	labels := make([]ui.Widget, len(styles))
	for i, entry := range styles {
		label := g.label(entry.text)
		label.SetSemanticStyle(entry.style)
		labels[i] = label
	}
	selectable := g.label("Selectable text: drag to select and copy me.")
	g.attempt("Selectable", func() error {
		selectable.SetSelectable(true)
		return nil
	})
	disabled := g.label("Disabled label")
	disabled.SetEnabled(false)
	return g.column(append(labels, selectable, disabled)...)
}

func (g *gallery) labelLayoutDemo() ui.Widget {
	paragraph := g.label(longText)
	paragraph.SetWrapMode(ui.WrapWords)
	paragraph.SetMaximumSize(ui.Size{Width: 420})
	// A label reports the height of one line, so give the wrapped paragraph
	// room for its lines.
	paragraph.SetMinimumSize(ui.Size{Height: 110})

	alignment := g.choice([]string{"Start", "Center", "End", "Justified"}, 0, func(index int) {
		paragraph.SetTextAlignment([]ui.TextAlignment{ui.TextAlignStart, ui.TextAlignCenter, ui.TextAlignEnd, ui.TextAlignJustified}[index])
	})
	wrap := g.choice([]string{"Wrap words", "Wrap characters", "No wrapping"}, 0, func(index int) {
		paragraph.SetWrapMode([]ui.WrapMode{ui.WrapWords, ui.WrapCharacters, ui.WrapNone}[index])
	})
	truncation := g.choice([]string{"No truncation", "Truncate head", "Truncate middle", "Truncate tail"}, 0, func(index int) {
		paragraph.SetTruncation([]ui.Truncation{ui.TruncateNone, ui.TruncateHead, ui.TruncateMiddle, ui.TruncateTail}[index])
	})
	lines := g.choice([]string{"Any number of lines", "One line", "Two lines", "Three lines"}, 0, func(index int) {
		paragraph.SetMaximumLines(index)
	})
	return g.column(
		g.row(alignment, wrap),
		g.row(truncation, lines),
		paragraph,
	)
}

func (g *gallery) richTextDemo() ui.Widget {
	rich := must(g.window.CreateRichText(ui.RichTextOptions{RTF: []byte(sampleRTF)}))
	tall(rich, 120)
	rich.OnChange(func() {
		g.log("Rich text changed: %d characters", len([]rune(rich.Text())))
	})
	editable := g.checkBox("Editable", false, func(checked bool) {
		rich.SetEditable(checked)
		g.log("Rich text editable: %t", rich.IsEditable())
	})
	export := g.button("Export RTF", func() {
		data, err := rich.RTF()
		if err != nil {
			g.log("RTF: %v", err)
			return
		}
		g.log("RTF export: %d bytes", len(data))
	})
	plain := g.button("Replace with plain text", func() {
		rich.SetText("Plain text replaces the styled document.")
	})
	restore := g.button("Restore RTF", func() {
		if err := rich.SetRTF([]byte(sampleRTF)); err != nil {
			g.log("RTF: %v", err)
		}
	})
	return g.column(rich, g.row(editable, export, plain, restore))
}

func (g *gallery) markdownDemo() ui.Widget {
	view := must(g.window.CreateMarkdown(ui.MarkdownOptions{Source: sampleMarkdown}))
	view.OnLinkActivate(func(url string) {
		g.log("Markdown link: %s", url)
	})
	source := must(g.window.CreateTextArea(ui.TextAreaOptions{Text: sampleMarkdown}))
	source.OnChange(func(text string) {
		if err := view.SetSource(text); err != nil {
			g.log("Markdown: %v", err)
		}
	})
	return g.layout(ui.Row(
		ui.Item(source).Grow(1).MinimumSize(ui.Size{Height: 260}),
		ui.Item(view).Grow(1).MinimumSize(ui.Size{Height: 260}),
	).WithGap(12))
}
