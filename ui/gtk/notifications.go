//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"runtime/cgo"
	"sync"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// notifications delivers user notifications with the freedesktop
// notification service on the session bus. Without the service, the calls
// report ErrNotSupported after they validate their arguments.
type notifications struct {
	window *Window
}

// notificationCenter receives activations for every window.
type notificationCenter struct{}

var notificationsOnce sync.Once

func attachNotifications() {
	notificationsOnce.Do(func() {
		C.gkGTKNotificationsAttach(C.uintptr_t(cgo.NewHandle(&notificationCenter{})))
	})
}

func (*notificationCenter) gtkEvent(event gtkEvent) {
	if event.kind != C.GK_EVENT_NOTIFICATION {
		return
	}
	activation := ui.NotificationActivation{NotificationID: event.text, ActionID: event.text2}
	if event.object != nil {
		activation.ReplyText = C.GoString((*C.char)(event.object))
	}
	for _, window := range liveWindows() {
		each(&window.events.notifications, func(handler ui.NotificationActivationHandler) {
			handler(activation)
		})
	}
}

// Notifications returns the notification service.
func (w *Window) Notifications() ui.Notifications {
	attachNotifications()
	return notifications{window: w}
}

// errNotBundled wraps ErrNotSupported when no notification service runs.
var errNotBundled = errors.Join(ui.ErrNotSupported,
	errors.New("ui/gtk: notifications require the freedesktop notification service"))

// notificationResult is the completion of an asynchronous notification call.
type notificationResult struct {
	status int
	ids    []string
	err    error
}

//export gkGTKGoNotificationResult
func gkGTKGoNotificationResult(token C.uintptr_t, status C.int, ids *C.char, size C.size_t, message *C.char) {
	result := notificationResult{status: int(status), ids: splitNul(unsafe.Pointer(ids), size)}
	if ids != nil {
		C.free(unsafe.Pointer(ids))
	}
	if message != nil {
		result.err = errors.New("ui/gtk: " + C.GoString(message))
		C.free(unsafe.Pointer(message))
	} else if status < 0 {
		result.err = errors.New("ui/gtk: the notification center reported an error")
	}
	cgo.Handle(token).Value().(chan notificationResult) <- result
}

// await starts an asynchronous call and waits for its result or ctx.
func await(ctx context.Context, start func(token C.uintptr_t)) (notificationResult, error) {
	if err := ctx.Err(); err != nil {
		return notificationResult{}, err
	}
	if C.gkGTKNotificationsAvailable() == 0 {
		return notificationResult{}, errNotBundled
	}
	results := make(chan notificationResult, 1)
	handle := cgo.NewHandle(results)
	start(C.uintptr_t(handle))
	select {
	case result := <-results:
		handle.Delete()
		return result, result.err
	case <-ctx.Done():
		// The completion still arrives later; the buffered channel keeps it
		// from blocking, and the handle must stay valid until then.
		go func() {
			<-results
			handle.Delete()
		}()
		return notificationResult{}, ctx.Err()
	}
}

func (notifications) Authorization(ctx context.Context) (ui.NotificationAuthorization, error) {
	result, err := await(ctx, func(token C.uintptr_t) {
		C.gkGTKNotificationAuthorization(token)
	})
	return ui.NotificationAuthorization(result.status), err
}

func (notifications) RequestAuthorization(ctx context.Context) (ui.NotificationAuthorization, error) {
	result, err := await(ctx, func(token C.uintptr_t) {
		C.gkGTKNotificationRequestAuthorization(token)
	})
	return ui.NotificationAuthorization(result.status), err
}

func (notifications) Send(ctx context.Context, notification ui.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := notification.Validate(); err != nil {
		return err
	}
	attachment := ""
	if notification.Icon.Image != nil && !notification.Icon.Image.Bounds().Empty() {
		path, err := writeNotificationImage(notification)
		if err != nil {
			return err
		}
		attachment = path
	}
	category := notification.Category
	if category == "" && len(notification.Actions) > 0 {
		category = "gamekit." + notification.ID
	}
	ids := make([]string, len(notification.Actions))
	titles := make([]string, len(notification.Actions))
	flags := make([]C.int, max(1, len(notification.Actions)))
	for i, action := range notification.Actions {
		ids[i], titles[i] = action.ID, action.Text
		if action.Destructive {
			flags[i] |= 1
		}
		if action.TextInput {
			flags[i] |= 2
		}
	}
	delay := 0.0
	if !notification.Delivery.IsZero() {
		delay = time.Until(notification.Delivery).Seconds()
	}
	_, err := await(ctx, func(token C.uintptr_t) {
		identifier := cString(notification.ID)
		title := cString(notification.Title)
		subtitle := cString(notification.Subtitle)
		body := cString(notification.Body)
		categoryValue := cString(category)
		thread := cString(notification.ThreadID)
		attachmentValue := cString(attachment)
		defer freeCString(identifier)
		defer freeCString(title)
		defer freeCString(subtitle)
		defer freeCString(body)
		defer freeCString(categoryValue)
		defer freeCString(thread)
		defer freeCString(attachmentValue)
		idBuffer, titleBuffer := joinNul(ids), joinNul(titles)
		C.gkGTKNotificationSend(token, identifier, title, subtitle, body, categoryValue, thread,
			boolInt(notification.Sound), attachmentValue, C.double(delay),
			bytesPointer(idBuffer), C.size_t(len(idBuffer)), bytesPointer(titleBuffer), C.size_t(len(titleBuffer)),
			&flags[0], C.int(len(notification.Actions)))
	})
	return err
}

// writeNotificationImage stores the icon where the notification center can
// copy it as an attachment.
func writeNotificationImage(notification ui.Notification) (string, error) {
	dir := filepath.Join(os.TempDir(), "gamekit-notifications")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "icon-*.png")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := png.Encode(file, notification.Icon.Image); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func (notifications) Pending(ctx context.Context) ([]string, error) {
	result, err := await(ctx, func(token C.uintptr_t) {
		C.gkGTKNotificationPending(token)
	})
	return result.ids, err
}

func (notifications) Delivered(ctx context.Context) ([]string, error) {
	result, err := await(ctx, func(token C.uintptr_t) {
		C.gkGTKNotificationDelivered(token)
	})
	return result.ids, err
}

func (notifications) Withdraw(ids ...string) error {
	if C.gkGTKNotificationsAvailable() == 0 {
		return errNotBundled
	}
	buffer := joinNul(ids)
	C.gkGTKNotificationWithdraw(bytesPointer(buffer), C.size_t(len(buffer)))
	return nil
}

// SetBadge sets the count of the application's launcher entry with the Unity
// launcher API, which docks such as Dash to Dock show. A badge that is not
// a number hides the count.
func (notifications) SetBadge(badge string) error {
	value := cString(badge)
	defer freeCString(value)
	C.gkGTKSetBadge(value)
	return nil
}

func (n notifications) OnActivate(handler ui.NotificationActivationHandler) ui.Unsubscribe {
	return subscribe(&n.window.events.notifications, handler, handler == nil)
}
