package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func dialogCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("File dialogs", "Open one or several files, save a file, and select a directory.", g.fileDialogsDemo),
		g.card("Alerts", "One alert for each severity, with details.", g.alertsDemo),
		g.card("Confirmations", "Two-button, destructive, and three-button confirmations.", g.confirmationsDemo),
		g.card("Prompts", "Ask for a line of text, or for a secret.", g.promptsDemo),
		g.card("About panel", "The standard panel with the name, version, and credits of the application.", g.aboutDemo),
		g.card("Cancellation", "The dialog closes by itself when its context times out after five seconds.", g.dialogCancellationDemo),
	}
}

// reportDialog logs the result of a dialog. It reports a user cancellation
// separately from other errors.
func (g *gallery) reportDialog(name, result string, err error) {
	switch {
	case errors.Is(err, ui.ErrCancelled):
		g.log("%s: cancelled by the user", name)
	case errors.Is(err, context.DeadlineExceeded):
		g.log("%s: the context timed out", name)
	case err != nil:
		g.log("%s: %v", name, err)
	default:
		g.log("%s: %s", name, result)
	}
}

func (g *gallery) fileDialogsDemo() ui.Widget {
	hidden := false
	filters := []ui.FileFilter{
		{Name: "Images", Patterns: []string{"*.png", "*.jpg", "*.jpeg"}},
		{Name: "Text", Patterns: []string{"*.txt", "*.md"}},
		{Name: "All files", Patterns: []string{"*"}},
	}
	result := g.value("No file chosen")
	report := func(name, value string, err error) {
		g.reportDialog(name, value, err)
		if err == nil {
			result.SetText(value)
		}
	}
	open := g.button("Open file…", func() {
		path, err := g.window.OpenFile(context.Background(), ui.OpenFileOptions{
			Title:           "Choose an image or text file",
			Filters:         filters,
			ShowHiddenFiles: hidden,
		})
		report("Open file", path, err)
	})
	openMany := g.button("Open files…", func() {
		paths, err := g.window.OpenFiles(context.Background(), ui.OpenFileOptions{
			Title:           "Choose files",
			Filters:         filters,
			SelectedFilter:  2,
			ShowHiddenFiles: hidden,
		})
		report("Open files", fmt.Sprintf("%d files: %s", len(paths), strings.Join(paths, ", ")), err)
	})
	save := g.button("Save file…", func() {
		path, err := g.window.SaveFile(context.Background(), ui.SaveFileOptions{
			Title:            "Save the level",
			Filename:         "level1.txt",
			Filters:          filters[1:2],
			DefaultExtension: "txt",
			ConfirmOverwrite: true,
			ShowHiddenFiles:  hidden,
		})
		report("Save file", path, err)
	})
	directory := g.button("Select directory…", func() {
		path, err := g.window.SelectDirectory(context.Background(), ui.DirectoryDialogOptions{Title: "Choose a project folder"})
		report("Select directory", path, err)
	})
	showHidden := g.checkBox("Show hidden files", false, func(checked bool) {
		hidden = checked
	})
	return g.column(g.row(open, openMany, save, directory, showHidden), result)
}

func (g *gallery) alertsDemo() ui.Widget {
	severities := []struct {
		name     string
		severity ui.Severity
		message  string
	}{
		{"Information", ui.SeverityInformation, "The export finished."},
		{"Success", ui.SeveritySuccess, "All tests passed."},
		{"Warning", ui.SeverityWarning, "The disk is almost full."},
		{"Error", ui.SeverityError, "The project file is damaged."},
	}
	buttons := make([]ui.Widget, len(severities))
	for i, entry := range severities {
		buttons[i] = g.button(entry.name, func() {
			id, err := g.window.ShowAlert(context.Background(), ui.AlertOptions{
				Title:       entry.name,
				Message:     entry.message,
				Details:     "The details text gives more information about the alert.",
				Severity:    entry.severity,
				Acknowledge: ui.DialogAction{ID: "ok", Text: "OK"},
			})
			g.reportDialog(entry.name+" alert", "closed with "+id, err)
		})
	}
	return g.row(buttons...)
}

func (g *gallery) confirmationsDemo() ui.Widget {
	simple := g.button("Continue?", func() {
		id, err := g.window.ShowConfirmation(context.Background(), ui.ConfirmationOptions{
			Title:   "Continue the import?",
			Message: "The import replaces 12 textures.",
			Actions: []ui.DialogAction{
				{ID: "continue", Text: "Continue"},
				{ID: "cancel", Text: "Cancel"},
			},
			DefaultAction: "continue",
			CancelAction:  "cancel",
		})
		g.reportDialog("Confirmation", "chose "+id, err)
	})
	destructive := g.button("Delete…", func() {
		id, err := g.window.ShowConfirmation(context.Background(), ui.ConfirmationOptions{
			Title:    "Delete “level1”?",
			Message:  "You cannot undo this action.",
			Severity: ui.SeverityWarning,
			Actions: []ui.DialogAction{
				{ID: "delete", Text: "Delete", Destructive: true},
				{ID: "keep", Text: "Keep"},
			},
			DefaultAction: "keep",
			CancelAction:  "keep",
		})
		g.reportDialog("Destructive confirmation", "chose "+id, err)
	})
	three := g.button("Save changes?", func() {
		id, err := g.window.ShowConfirmation(context.Background(), ui.ConfirmationOptions{
			Title:   "Save the changes to “level1”?",
			Message: "Your changes are lost if you do not save them.",
			Details: "Three-button confirmation.",
			Actions: []ui.DialogAction{
				{ID: "save", Text: "Save"},
				{ID: "discard", Text: "Don’t Save", Destructive: true},
				{ID: "cancel", Text: "Cancel"},
			},
			DefaultAction: "save",
			CancelAction:  "cancel",
		})
		g.reportDialog("Three-button confirmation", "chose "+id, err)
	})
	return g.row(simple, destructive, three)
}

func (g *gallery) promptsDemo() ui.Widget {
	text := g.button("Rename…", func() {
		value, err := g.window.ShowPrompt(context.Background(), ui.PromptOptions{
			Title:       "Rename the level",
			Message:     "Enter a new name.",
			Text:        "level1",
			Placeholder: "Level name",
		})
		g.reportDialog("Prompt", fmt.Sprintf("%q", value), err)
	})
	secure := g.button("Enter password…", func() {
		value, err := g.window.ShowPrompt(context.Background(), ui.PromptOptions{
			Title:       "Unlock the vault",
			Message:     "Enter the password of the vault.",
			Placeholder: "Password",
			Secure:      true,
		})
		g.reportDialog("Secure prompt", fmt.Sprintf("%d characters", len([]rune(value))), err)
	})
	return g.row(text, secure)
}

func (g *gallery) aboutDemo() ui.Widget {
	about := g.button("Show about panel", func() {
		g.attempt("About", g.showAbout)
	})
	return g.row(about)
}

// showAbout shows the about panel of the gallery.
func (g *gallery) showAbout() error {
	err := g.window.ShowAbout(context.Background(), ui.AboutOptions{
		Name:      "GameKit Widget Gallery",
		Version:   "1.0",
		Copyright: "© 2026 The GameKit authors",
		Website:   "https://github.com/bluescreen10/gamekit",
		Credits:   "Native widgets from Cocoa, GTK 4, and Win32.",
		Icon:      ui.IconSource{SystemName: "game-controller"},
	})
	if errors.Is(err, ui.ErrCancelled) {
		return nil
	}
	return err
}

func (g *gallery) dialogCancellationDemo() ui.Widget {
	timed := g.button("Alert with timeout", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		id, err := g.window.ShowAlert(ctx, ui.AlertOptions{
			Title:   "This alert closes itself",
			Message: "Wait five seconds, or close it.",
		})
		g.reportDialog("Timed alert", "closed with "+id, err)
	})
	timedOpen := g.button("Open file with timeout", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		path, err := g.window.OpenFile(ctx, ui.OpenFileOptions{Title: "This dialog closes after five seconds"})
		g.reportDialog("Timed open", path, err)
	})
	done := g.button("Already cancelled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := g.window.ShowAlert(ctx, ui.AlertOptions{Title: "Never shown"})
		g.log("Alert with a cancelled context: %v", err)
	})
	return g.row(timed, timedOpen, done)
}
