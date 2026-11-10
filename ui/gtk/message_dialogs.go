//go:build linux && cgo

package gtk

import (
	"context"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

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
	if w.nativePointer() == nil {
		return "", ui.ErrClosed
	}
	alert := newAlert(options.Severity, options.Title, options.Message, options.Details,
		[]string{acknowledge.Text}, 0, nil, nil)
	defer alert.close()
	if _, err := w.runAlert(ctx, alert); err != nil {
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
	if w.nativePointer() == nil {
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
	destructive := make([]bool, len(actions))
	for i, action := range actions {
		titles[i] = action.Text
		destructive[i] = action.Destructive
	}
	alert := newAlert(options.Severity, options.Title, options.Message, options.Details, titles, cancel, destructive, nil)
	defer alert.close()
	index, err := w.runAlert(ctx, alert)
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
	if w.nativePointer() == nil {
		return "", ui.ErrClosed
	}
	alert := newAlert(ui.SeverityInformation, options.Title, options.Message, "", []string{"OK", "Cancel"}, 1, nil, &options)
	defer alert.close()
	index, err := w.runAlert(ctx, alert)
	if err != nil {
		return "", err
	}
	if index != 0 {
		return "", ui.ErrCancelled
	}
	return goString(gtkEditableGetText((*gtkEditable)(unsafe.Pointer(alert.entry)))), nil
}

// ShowAbout shows the standard About panel. It returns once the panel is
// visible; the panel is not modal.
func (w *Window) ShowAbout(ctx context.Context, options ui.AboutOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	showNativeAbout(options)
	return nil
}
