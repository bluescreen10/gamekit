//go:build windows

package win32

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/bluescreen10/gamekit/ui"
)

// notifications delivers user notifications as Windows toast
// notifications. When Windows cannot create a notifier for the
// application, the calls report ErrNotSupported after they validate their
// arguments.
type notifications struct {
	window *Window
}

// notificationCenter receives activations for every window.
type notificationCenter struct{}

var notificationsOnce sync.Once

func attachNotifications() {
	notificationsOnce.Do(func() {
		setNotificationReceiver(&notificationCenter{})
	})
}

func (*notificationCenter) receiveEvent(event any) {
	notification, ok := event.(notificationEvent)
	if !ok {
		return
	}
	activation := notification.activation
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
	errors.New("ui/win32: Windows cannot send toast notifications for this application"))

// available reports errNotBundled when Windows cannot create a notifier for
// the application, after ctx.
func available(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !notificationsAvailable() {
		return errNotBundled
	}
	return nil
}

func (notifications) Authorization(ctx context.Context) (ui.NotificationAuthorization, error) {
	if err := available(ctx); err != nil {
		return 0, err
	}
	return notificationAuthorization(), nil
}

func (notifications) RequestAuthorization(ctx context.Context) (ui.NotificationAuthorization, error) {
	if err := available(ctx); err != nil {
		return 0, err
	}
	return requestNotificationAuthorization(), nil
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
	if err := available(ctx); err != nil {
		return err
	}
	if err := sendNotification(notification, attachment); err != nil {
		return fmt.Errorf("ui/win32: %w", err)
	}
	return nil
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
	if err := available(ctx); err != nil {
		return nil, err
	}
	return pendingNotifications(), nil
}

func (notifications) Delivered(ctx context.Context) ([]string, error) {
	if err := available(ctx); err != nil {
		return nil, err
	}
	return deliveredNotifications(), nil
}

func (notifications) Withdraw(ids ...string) error {
	if !notificationsAvailable() {
		return errNotBundled
	}
	withdrawNotifications(ids)
	return nil
}

// SetBadge shows badge as an overlay on the taskbar buttons of the
// application's windows. An empty badge removes the overlay.
func (notifications) SetBadge(badge string) error {
	setBadge(badge)
	return nil
}

func (n notifications) OnActivate(handler ui.NotificationActivationHandler) ui.Unsubscribe {
	return subscribe(&n.window.events.notifications, handler, handler == nil)
}
