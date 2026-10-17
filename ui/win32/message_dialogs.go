//go:build windows && cgo

package win32

/*
#include "win32_bridge.h"
*/
import "C"

import (
	"context"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// runAlert shows a native alert and returns the zero-based index of the
// chosen button. A context that ends while the alert is visible closes it.
func (w *Window) runAlert(ctx context.Context, alert unsafe.Pointer) (int, error) {
	defer C.gkWin32Release(alert)
	modal := C.gkWin32ModalBegin(alert, w.nativeWindowPointer())
	for {
		index := int(C.gkWin32ModalStep(modal))
		if index >= 0 {
			C.gkWin32ModalEnd(modal, 0)
			return index, nil
		}
		if err := ctx.Err(); err != nil {
			C.gkWin32ModalEnd(modal, 1)
			return -1, err
		}
		time.Sleep(time.Millisecond)
	}
}

func newAlert(severity ui.Severity, title, message, details string, buttons []string, cancel int,
	destructive int, prompt *ui.PromptOptions) unsafe.Pointer {
	titleValue := cString(title)
	messageValue := cString(message)
	detailsValue := cString(details)
	defer freeCString(titleValue)
	defer freeCString(messageValue)
	defer freeCString(detailsValue)
	buttonBuffer := joinNul(buttons)
	var text, placeholder *C.char
	var isPrompt, secure C.int
	if prompt != nil {
		isPrompt = 1
		secure = boolInt(prompt.Secure)
		text = cString(prompt.Text)
		placeholder = cString(prompt.Placeholder)
		defer freeCString(text)
		defer freeCString(placeholder)
	}
	return C.gkWin32AlertCreate(C.int(severity), titleValue, messageValue, detailsValue,
		bytesPointer(buttonBuffer), C.size_t(len(buttonBuffer)), C.int(cancel), C.int(destructive),
		isPrompt, secure, text, placeholder)
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
	if w.nativePointer() == nil {
		return "", ui.ErrClosed
	}
	alert := newAlert(options.Severity, options.Title, options.Message, options.Details,
		[]string{acknowledge.Text}, 0, 0, nil)
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
	destructive := 0
	for i, action := range actions {
		titles[i] = action.Text
		if action.Destructive {
			destructive |= 1 << i
		}
	}
	alert := newAlert(options.Severity, options.Title, options.Message, options.Details, titles, cancel, destructive, nil)
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
	alert := newAlert(ui.SeverityInformation, options.Title, options.Message, "", []string{"OK", "Cancel"}, 1, 0, &options)
	C.gkWin32Retain(alert)
	defer C.gkWin32Release(alert)
	index, err := w.runAlert(ctx, alert)
	if err != nil {
		return "", err
	}
	if index != 0 {
		return "", ui.ErrCancelled
	}
	return goStringFree(C.gkWin32AlertText(alert)), nil
}

// ShowAbout shows the standard About panel. It returns once the panel is
// visible; the panel is not modal.
func (w *Window) ShowAbout(ctx context.Context, options ui.AboutOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name := cString(options.Name)
	version := cString(options.Version)
	copyright := cString(options.Copyright)
	website := cString(options.Website)
	credits := cString(options.Credits)
	defer freeCString(name)
	defer freeCString(version)
	defer freeCString(copyright)
	defer freeCString(website)
	defer freeCString(credits)
	icon := nativeIcon(options.Icon, 64)
	defer C.gkWin32Release(icon)
	C.gkWin32ShowAbout(name, version, copyright, website, credits, icon)
	return nil
}
