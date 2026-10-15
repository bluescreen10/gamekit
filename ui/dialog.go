package ui

import (
	"errors"
	"fmt"
)

// FileFilter is one selectable group of filename patterns. Patterns use shell
// syntax such as "*.png" and "*.jpg".
type FileFilter struct {
	// Name is the text that the dialog shows for the filter.
	Name string

	// Patterns are the filename patterns that the filter accepts.
	Patterns []string
}

// OpenFileOptions controls an open-file dialog. Directory and Filename are
// optional initial values.
type OpenFileOptions struct {
	// Title is the title of the dialog.
	Title string

	// Directory is the directory that the dialog shows first.
	Directory string

	// Filename is the file that the dialog selects first.
	Filename string

	// Filters limit the files that the user can choose. Empty Filters
	// accept all files.
	Filters []FileFilter

	// SelectedFilter is the index of the initially selected filter.
	SelectedFilter int

	// ShowHiddenFiles shows the files that the file manager hides.
	ShowHiddenFiles bool

	// SelectPackages lets the user choose macOS packages as files instead of
	// browsing into them.
	SelectPackages bool
}

// SaveFileOptions controls a save-file dialog. Directory and Filename are
// optional initial values.
type SaveFileOptions struct {
	// Title is the title of the dialog.
	Title string

	// Directory is the directory that the dialog shows first.
	Directory string

	// Filename is the name that the dialog suggests.
	Filename string

	// Filters are the file types that the user can save as.
	Filters []FileFilter

	// ConfirmOverwrite asks the user before the dialog returns the path of
	// an existing file.
	ConfirmOverwrite bool

	// SelectedFilter is the index of the initially selected filter.
	SelectedFilter int

	// DefaultExtension is appended when the user omits one, without a dot.
	DefaultExtension string

	// ShowHiddenFiles shows the files that the file manager hides.
	ShowHiddenFiles bool
}

// DirectoryDialogOptions controls a directory-selection dialog.
type DirectoryDialogOptions struct {
	// Title is the title of the dialog.
	Title string

	// Directory is the directory that the dialog shows first.
	Directory string
}

// DialogAction is one button in a message dialog.
type DialogAction struct {
	// ID is the value that the dialog returns when the user chooses the
	// button.
	ID string

	// Text is the label of the button.
	Text string

	// Destructive marks an action that deletes data or cannot be undone. The
	// platform can show the button in a warning style.
	Destructive bool
}

// AlertOptions describes a modal message with one acknowledgement button. A
// zero Acknowledge uses the platform's "OK".
type AlertOptions struct {
	// Title is the main text of the alert.
	Title string

	// Message is the text below the title.
	Message string

	// Details is more text that the user can read.
	Details string

	// Severity selects the icon of the alert.
	Severity Severity

	// Acknowledge is the button that closes the alert.
	Acknowledge DialogAction
}

// Validate checks that the alert has a message and a recognized severity.
func (options AlertOptions) Validate() error {
	if options.Title == "" && options.Message == "" {
		return errors.New("ui: alert requires a title or message")
	}
	return options.Severity.Validate()
}

// ConfirmationOptions describes a modal question. CancelAction names the
// action chosen by Escape or closing the dialog; when it is not one of Actions,
// the backend adds a platform "Cancel" button with that ID.
type ConfirmationOptions struct {
	// Title is the main text of the question.
	Title string

	// Message is the text below the title.
	Message string

	// Details is more text that the user can read.
	Details string

	// Severity selects the icon of the dialog.
	Severity Severity

	// Actions are the buttons in order.
	Actions []DialogAction

	// DefaultAction is the ID of the action that Return chooses. It can be
	// empty.
	DefaultAction string

	// CancelAction is the ID of the action that Escape chooses.
	CancelAction string
}

// Validate checks that the confirmation has actions with unique IDs.
func (options ConfirmationOptions) Validate() error {
	if len(options.Actions) == 0 {
		return errors.New("ui: confirmation requires at least one action")
	}
	if options.CancelAction == "" {
		return errors.New("ui: confirmation requires a cancel action")
	}
	seen := make(map[string]struct{}, len(options.Actions))
	for i, action := range options.Actions {
		if action.ID == "" || action.Text == "" {
			return fmt.Errorf("ui: confirmation action %d requires an ID and text", i)
		}
		if _, exists := seen[action.ID]; exists {
			return fmt.Errorf("ui: duplicate confirmation action %q", action.ID)
		}
		seen[action.ID] = struct{}{}
	}
	if _, exists := seen[options.DefaultAction]; options.DefaultAction != "" && !exists {
		return fmt.Errorf("ui: default action %q is not an action", options.DefaultAction)
	}
	return options.Severity.Validate()
}

// PromptOptions describes a modal single-line text request.
type PromptOptions struct {
	// Title is the main text of the dialog.
	Title string

	// Message is the text below the title.
	Message string

	// Text is the initial text in the field.
	Text string

	// Placeholder is the text that the empty field shows.
	Placeholder string

	// Secure hides the entered text, as in a password field.
	Secure bool
}

// AboutOptions describes the application's About panel.
type AboutOptions struct {
	// Name is the name of the application.
	Name string

	// Version is the version text of the application.
	Version string

	// Copyright is the copyright notice.
	Copyright string

	// Website is the URL of the website of the application.
	Website string

	// Credits is the text that names the authors and contributors.
	Credits string

	// Icon is the icon of the application. The zero value uses the
	// application icon.
	Icon IconSource
}
