package main

import (
	"fmt"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func feedbackCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Info bars", "Every severity. The last bar has actions and a close button.", g.infoBarsDemo),
		g.card("Inline validation", "Messages that sit next to a field.", g.inlineValidationDemo),
		g.card("Badges", "Counts with a maximum, text badges, and severities.", g.badgesDemo),
		g.card("Toasts and tooltips", "A toast hides after its timeout. A tooltip shows at its anchor after a delay.", g.toastsDemo),
		g.card("Progress dialog", "A simulated five-second operation that you can cancel.", g.progressDialogDemo),
		g.card("Empty state", "A placeholder for a view that has no content, with actions.", g.emptyStateDemo),
	}
}

func (g *gallery) infoBarsDemo() ui.Widget {
	severities := []struct {
		severity ui.Severity
		title    string
		detail   string
	}{
		{ui.SeverityInformation, "Information", "A new version of the asset pipeline is available."},
		{ui.SeveritySuccess, "Success", "All 128 shaders compiled."},
		{ui.SeverityWarning, "Warning", "Three textures are larger than 4096 pixels."},
	}
	bars := make([]ui.Widget, 0, len(severities)+1)
	for _, entry := range severities {
		bars = append(bars, g.optional(func() ui.Widget {
			return must(g.window.CreateInfoBar(ui.InfoBarOptions{
				Severity: entry.severity,
				Title:    entry.title,
				Detail:   entry.detail,
			}))
		}))
	}
	actionable := must(g.window.CreateInfoBar(ui.InfoBarOptions{
		Severity: ui.SeverityError,
		Title:    "Build failed",
		Detail:   "main.go:42: undefined: renderer",
		Actions: []ui.Action{
			{ID: "retry", Text: "Retry", Enabled: true},
			{ID: "log", Text: "Show log", Enabled: true},
		},
		Closable: true,
		Live:     ui.LiveAssertive,
	}))
	actionable.OnAction(func(id string) {
		g.log("Info bar action: %s", id)
	})
	actionable.OnClose(func() {
		actionable.SetVisible(false)
		g.log("Info bar closed")
	})
	bars = append(bars, actionable)

	restore := g.button("Show closed bar", func() {
		actionable.SetVisible(true)
	})
	cycle := g.button("Change severity", func() {
		next := (actionable.Severity() + 1) % 4
		actionable.SetSeverity(next)
		actionable.SetTitle(fmt.Sprintf("Severity %d", next))
		actionable.SetDetail("SetSeverity, SetTitle, and SetDetail changed this bar.")
	})
	return g.column(append(bars, g.row(restore, cycle))...)
}

func (g *gallery) inlineValidationDemo() ui.Widget {
	messages := []struct {
		severity ui.Severity
		text     string
	}{
		{ui.SeverityInformation, "Use 3 to 16 characters."},
		{ui.SeveritySuccess, "This name is available."},
		{ui.SeverityWarning, "This name is close to an existing one."},
		{ui.SeverityError, "This name is taken."},
	}
	widgets := make([]ui.Widget, len(messages))
	for i, entry := range messages {
		widgets[i] = must(g.window.CreateInlineValidation(ui.InlineValidationOptions{
			Severity: entry.severity,
			Text:     entry.text,
		}))
	}
	return g.column(widgets...)
}

func (g *gallery) badgesDemo() ui.Widget {
	counted := must(g.window.CreateBadge(ui.BadgeOptions{Count: 7, Maximum: 99, AccessibleValue: "7 unread messages"}))
	display := g.value(counted.DisplayText())
	setCount := func(count int) {
		counted.SetCount(max(0, count))
		display.SetText(fmt.Sprintf("Count %d shows %q", counted.Count(), counted.DisplayText()))
	}
	setCount(7)
	add := g.button("+1", func() {
		setCount(counted.Count() + 1)
	})
	overflow := g.button("Set 150", func() {
		setCount(150)
	})
	clear := g.button("Set 0", func() {
		setCount(0)
	})

	severities := []struct {
		text     string
		severity ui.Severity
	}{
		{"Info", ui.SeverityInformation},
		{"New", ui.SeveritySuccess},
		{"Beta", ui.SeverityWarning},
		{"Failed", ui.SeverityError},
	}
	texts := make([]ui.Widget, len(severities))
	for i, entry := range severities {
		texts[i] = g.optional(func() ui.Widget {
			return must(g.window.CreateBadge(ui.BadgeOptions{Text: entry.text, Severity: entry.severity}))
		})
	}
	return g.column(
		g.labeled("Count", counted, add, overflow, clear, display),
		g.labeled("Text and severity", texts...),
	)
}

func (g *gallery) toastsDemo() ui.Widget {
	toast := must(g.window.CreateToast(ui.ToastOptions{
		Message: "Level saved.",
		Actions: []ui.Action{{ID: "undo", Text: "Undo", Enabled: true}},
		Timeout: 4 * time.Second,
	}))
	toast.OnAction(func(id string) {
		g.log("Toast action: %s", id)
	})
	show := g.button("Show toast", func() {
		if err := toast.Show(); err != nil {
			g.log("Toast: %v", err)
			return
		}
		g.log("Toast shown: %t", toast.IsShown())
	})
	dismiss := g.button("Dismiss", func() {
		toast.Dismiss()
		g.log("Toast shown: %t", toast.IsShown())
	})

	tooltip := must(g.window.CreateTooltip(ui.TooltipOptions{
		Text:      "A tooltip that the application shows.",
		Delay:     300 * time.Millisecond,
		Placement: ui.EdgeBottom,
	}))
	var anchor ui.Button
	anchor = g.button("Show tooltip here", func() {
		if tooltip.IsShown() {
			tooltip.Hide()
			return
		}
		if err := tooltip.Show(anchor); err != nil {
			g.log("Tooltip: %v", err)
		}
	})
	hover := g.label("Hover over this label for its widget tooltip.")
	hover.SetTooltip("Widget.SetTooltip sets this text.")
	return g.column(
		g.labeled("Toast", show, dismiss),
		g.labeled("Tooltip", anchor, hover),
	)
}

func (g *gallery) progressDialogDemo() ui.Widget {
	const duration = 5 * time.Second
	start := g.button("Start operation", func() {
		dialog, err := g.window.CreateProgressDialog(ui.ProgressDialogOptions{
			Title:       "Importing assets",
			Message:     "Preparing…",
			Cancellable: true,
		})
		if err != nil {
			g.log("Progress dialog: %v", err)
			return
		}
		cancelled := false
		dialog.OnCancel(func() {
			cancelled = true
		})
		if err := dialog.Show(); err != nil {
			dialog.Close()
			g.log("Progress dialog: %v", err)
			return
		}
		started := time.Now()
		g.every(func(now time.Time) bool {
			progress := min(1, float64(now.Sub(started))/float64(duration))
			if cancelled {
				dialog.Close()
				g.log("Import cancelled at %.0f%%", progress*100)
				return false
			}
			dialog.SetValue(progress)
			dialog.SetMessage(fmt.Sprintf("Importing file %d of 50…", int(progress*49)+1))
			if progress < 1 {
				return true
			}
			dialog.Close()
			g.log("Import finished")
			return false
		})
	})
	return g.row(start)
}

func (g *gallery) emptyStateDemo() ui.Widget {
	empty := must(g.window.CreateEmptyState(ui.EmptyStateOptions{
		Icon:   ui.IconSource{SystemName: "folder"},
		Title:  "No projects yet",
		Detail: "Create a project or open one from disk.",
		Actions: []ui.Action{
			{ID: "new", Text: "New project", Enabled: true},
			{ID: "open", Text: "Open…", Enabled: true},
		},
	}))
	empty.SetMinimumSize(ui.Size{Height: 200})
	empty.OnAction(func(id string) {
		g.log("Empty state action: %s", id)
	})
	rename := g.button("Change text", func() {
		empty.SetTitle("Nothing matches your search")
		empty.SetDetail("Try a different search term.")
	})
	return g.column(empty, g.row(rename))
}
