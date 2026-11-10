//go:build windows

package win32

import (
	"context"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

// OpenFile displays a native open-file panel parented to the window.
func (w *Window) OpenFile(ctx context.Context, options ui.OpenFileOptions) (string, error) {
	paths, err := w.openPanel(ctx, options, false, false)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// OpenFiles displays a native multiple-file panel parented to the window.
func (w *Window) OpenFiles(ctx context.Context, options ui.OpenFileOptions) ([]string, error) {
	return w.openPanel(ctx, options, true, false)
}

// SaveFile displays a native save-file panel parented to the window.
func (w *Window) SaveFile(ctx context.Context, options ui.SaveFileOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	paths, cancelled := w.nativeWindow().showSavePanel(options.Title, options.Directory, options.Filename,
		fileExtensions(options.Filters), options.ShowHiddenFiles, strings.TrimPrefix(options.DefaultExtension, "."))
	paths, err := dialogPaths(paths, cancelled)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// SelectDirectory displays a native directory panel parented to the window.
func (w *Window) SelectDirectory(ctx context.Context, options ui.DirectoryDialogOptions) (string, error) {
	paths, err := w.openPanel(ctx, ui.OpenFileOptions{
		Title:     options.Title,
		Directory: options.Directory,
	}, false, true)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

func (w *Window) openPanel(ctx context.Context, options ui.OpenFileOptions, multiple, directories bool) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Windows has no packages, so SelectPackages has no effect.
	paths, cancelled := w.nativeWindow().showOpenPanel(options.Title, options.Directory,
		options.Filename, fileExtensions(options.Filters), multiple, directories, options.ShowHiddenFiles)
	return dialogPaths(paths, cancelled)
}

func dialogPaths(result []string, cancelled bool) ([]string, error) {
	if cancelled {
		return nil, ui.ErrCancelled
	}
	if result == nil {
		return nil, ui.ErrNotSupported
	}
	paths := make([]string, 0, len(result))
	for _, path := range result {
		if path != "" {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil, ui.ErrCancelled
	}
	return paths, nil
}

func fileExtensions(filters []ui.FileFilter) string {
	extensions := make([]string, 0)
	for _, filter := range filters {
		for _, pattern := range filter.Patterns {
			extension := strings.TrimPrefix(strings.TrimSpace(pattern), "*.")
			if extension != "" && extension != "*" {
				extensions = append(extensions, extension)
			}
		}
	}
	return strings.Join(extensions, ",")
}

// runAlert shows a native alert, destroys it when it closes, and returns the
// zero-based index of the chosen button and the text that a prompt holds. A
// context that ends while the alert is visible closes it.
func (w *Window) runAlert(ctx context.Context, alert *nativeAlert) (index int, answer string, err error) {
	defer alert.destroy()
	alert.beginModal(w.nativeWindow())
	for {
		if index := alert.stepModal(); index >= 0 {
			alert.endModal(false)
			return index, alert.promptText(), nil
		}
		if err := ctx.Err(); err != nil {
			alert.endModal(true)
			return -1, "", err
		}
		time.Sleep(time.Millisecond)
	}
}

func newAlert(severity ui.Severity, title, message, details string, buttons []string, cancel int,
	destructive int, prompt *ui.PromptOptions) *nativeAlert {
	var text, placeholder string
	var isPrompt, secure bool
	if prompt != nil {
		isPrompt = true
		secure = prompt.Secure
		text = prompt.Text
		placeholder = prompt.Placeholder
	}
	return newNativeAlert(int(severity), title, message, details, buttons, cancel, destructive, isPrompt, secure,
		text, placeholder)
}

// ShowAlert shows a modal message and returns the acknowledgement action ID.
func (w *Window) ShowAlert(ctx context.Context, options ui.AlertOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := options.Validate(); err != nil {
		return "", err
	}
	acknowledge := options.Acknowledge
	if acknowledge.Text == "" {
		acknowledge.Text = "OK"
	}
	if acknowledge.ID == "" {
		acknowledge.ID = "ok"
	}
	if w.nativeWindow() == nil {
		return "", ui.ErrClosed
	}
	alert := newAlert(options.Severity, options.Title, options.Message, options.Details,
		[]string{acknowledge.Text}, 0, 0, nil)
	if _, _, err := w.runAlert(ctx, alert); err != nil {
		return "", err
	}
	return acknowledge.ID, nil
}

// ShowConfirmation asks a question and returns the chosen action ID.
func (w *Window) ShowConfirmation(ctx context.Context, options ui.ConfirmationOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := options.Validate(); err != nil {
		return "", err
	}
	if w.nativeWindow() == nil {
		return "", ui.ErrClosed
	}
	// The default action comes first so that Return chooses it.
	actions := make([]ui.DialogAction, 0, len(options.Actions)+1)
	for _, action := range options.Actions {
		if action.ID == options.DefaultAction {
			actions = append(actions, action)
		}
	}
	for _, action := range options.Actions {
		if action.ID != options.DefaultAction {
			actions = append(actions, action)
		}
	}
	cancel := -1
	for i, action := range actions {
		if action.ID == options.CancelAction {
			cancel = i
		}
	}
	if cancel < 0 {
		actions = append(actions, ui.DialogAction{ID: options.CancelAction, Text: "Cancel"})
		cancel = len(actions) - 1
	}
	titles := make([]string, len(actions))
	destructive := 0
	for i, action := range actions {
		titles[i] = action.Text
		if action.Destructive {
			destructive |= 1 << i
		}
	}
	alert := newAlert(options.Severity, options.Title, options.Message, options.Details, titles, cancel, destructive, nil)
	index, _, err := w.runAlert(ctx, alert)
	if err != nil {
		return "", err
	}
	if index < 0 || index >= len(actions) {
		return options.CancelAction, nil
	}
	return actions[index].ID, nil
}

// ShowPrompt asks for one line of text.
func (w *Window) ShowPrompt(ctx context.Context, options ui.PromptOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if w.nativeWindow() == nil {
		return "", ui.ErrClosed
	}
	alert := newAlert(ui.SeverityInformation, options.Title, options.Message, "", []string{"OK", "Cancel"}, 1, 0, &options)
	index, answer, err := w.runAlert(ctx, alert)
	if err != nil {
		return "", err
	}
	if index != 0 {
		return "", ui.ErrCancelled
	}
	return answer, nil
}

// ShowAbout shows the standard About panel. It returns once the panel is
// visible; the panel is not modal.
func (w *Window) ShowAbout(ctx context.Context, options ui.AboutOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	icon := newIcon(options.Icon, 64)
	showAbout(options.Name, options.Version, options.Copyright, options.Website, options.Credits, icon)
	return nil
}
