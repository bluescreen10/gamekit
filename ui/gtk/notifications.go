//go:build linux && cgo

package gtk

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"

	"github.com/bluescreen10/gamekit/ui"
)

// notifications uses the freedesktop notification service on the session bus.
type notifications struct {
	window *Window
}

func (w *Window) Notifications() ui.Notifications {
	attachNotifications()
	return notifications{window: w}
}

var errNotBundled = errors.Join(ui.ErrNotSupported,
	errors.New("ui/gtk: notifications require the freedesktop notification service"))

func checkNotifications(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !notificationsAvailable() {
		return errNotBundled
	}
	return nil
}

func (notifications) Authorization(ctx context.Context) (ui.NotificationAuthorization, error) {
	if err := checkNotifications(ctx); err != nil {
		return ui.NotificationNotDetermined, err
	}
	// The desktop notification service has no separate permission model.
	return ui.NotificationAuthorized, nil
}

func (n notifications) RequestAuthorization(ctx context.Context) (ui.NotificationAuthorization, error) {
	return n.Authorization(ctx)
}

func (notifications) Send(ctx context.Context, notification ui.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := notification.Validate(); err != nil {
		return err
	}
	if err := checkNotifications(ctx); err != nil {
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
	return sendNativeNotification(notification, attachment)
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
	if err := checkNotifications(ctx); err != nil {
		return nil, err
	}
	return pendingNotificationIDs(), nil
}

func (notifications) Delivered(ctx context.Context) ([]string, error) {
	if err := checkNotifications(ctx); err != nil {
		return nil, err
	}
	return deliveredNotificationIDs(), nil
}

func (notifications) Withdraw(ids ...string) error {
	if !notificationsAvailable() {
		return errNotBundled
	}
	withdrawNativeNotifications(ids)
	return nil
}

// SetBadge updates the count shown by desktop launchers supporting Unity's API.
func (notifications) SetBadge(badge string) error {
	setNativeBadge(badge)
	return nil
}

func (n notifications) OnActivate(handler ui.NotificationActivationHandler) ui.Unsubscribe {
	return subscribe(&n.window.events.notifications, handler, handler == nil)
}
