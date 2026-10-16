package ui_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

const conformanceNotificationID = "gamekit-conformance"

func checkExpectedNotifications(r *recorder, window ui.Window) {
	ctx := context.Background()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	notifications := window.Notifications()

	r.check("notifications/validation", func() error {
		invalid := []ui.Notification{
			{},
			{ID: "empty"},
			{ID: "action", Title: "Title", Actions: []ui.NotificationAction{{ID: "", Text: "Open"}}},
			{ID: "duplicate", Title: "Title", Actions: []ui.NotificationAction{
				{ID: "open", Text: "Open"},
				{ID: "open", Text: "Open again"},
			}},
		}
		for _, notification := range invalid {
			err := notifications.Send(ctx, notification)
			if err == nil {
				return fmt.Errorf("Send(%#v) accepted an invalid notification", notification)
			}
			if errors.Is(err, ui.ErrNotSupported) {
				return fmt.Errorf("Send validated after reporting %v", err)
			}
		}
		return nil
	})

	r.check("notifications/subscription", func() error {
		unsubscribe := notifications.OnActivate(func(ui.NotificationActivation) {
		})
		if unsubscribe == nil {
			return errors.New("OnActivate returned nil")
		}
		unsubscribe()
		unsubscribe()
		return nil
	})

	r.check("notifications/pre_cancelled", func() error {
		if _, err := notifications.Authorization(cancelled); !errors.Is(err, context.Canceled) {
			return fmt.Errorf("Authorization error = %v, want context.Canceled", err)
		}
		if _, err := notifications.Pending(cancelled); !errors.Is(err, context.Canceled) {
			return fmt.Errorf("Pending error = %v, want context.Canceled", err)
		}
		if _, err := notifications.Delivered(cancelled); !errors.Is(err, context.Canceled) {
			return fmt.Errorf("Delivered error = %v, want context.Canceled", err)
		}
		return nil
	})

	var authorization ui.NotificationAuthorization
	r.check("notifications/authorization", func() error {
		var err error
		authorization, err = notifications.Authorization(ctx)
		if err != nil {
			return err
		}
		if authorization > ui.NotificationProvisional {
			return fmt.Errorf("Authorization() = %d, want a known state", authorization)
		}
		return nil
	})

	r.check("notifications/badge", func() error {
		if err := notifications.SetBadge("3"); err != nil {
			return err
		}
		return notifications.SetBadge("")
	})

	authorized := func() error {
		if authorization != ui.NotificationAuthorized && authorization != ui.NotificationProvisional {
			return skipCheck("notification permission is %d; allow notifications for the conformance app to run this check", authorization)
		}
		return nil
	}

	r.check("notifications/schedule_and_withdraw", func() error {
		if err := authorized(); err != nil {
			return err
		}
		if err := notifications.Send(ctx, ui.Notification{
			ID:       conformanceNotificationID,
			Title:    "GameKit conformance",
			Body:     "Scheduled notification",
			Actions:  []ui.NotificationAction{{ID: "show", Text: "Show"}},
			Delivery: time.Now().Add(time.Hour),
		}); err != nil {
			return err
		}
		pending, err := notifications.Pending(ctx)
		if err != nil {
			return err
		}
		if !slices.Contains(pending, conformanceNotificationID) {
			return fmt.Errorf("Pending() = %v, want %q", pending, conformanceNotificationID)
		}
		if err := notifications.Withdraw(conformanceNotificationID); err != nil {
			return err
		}
		pending, err = notifications.Pending(ctx)
		if err != nil {
			return err
		}
		if slices.Contains(pending, conformanceNotificationID) {
			return fmt.Errorf("Pending() = %v after Withdraw", pending)
		}
		return nil
	})

	r.check("notifications/deliver_and_withdraw", func() error {
		if err := authorized(); err != nil {
			return err
		}
		if err := notifications.Send(ctx, ui.Notification{
			ID:    conformanceNotificationID,
			Title: "GameKit conformance",
			Body:  "Delivered notification",
		}); err != nil {
			return err
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			delivered, err := notifications.Delivered(ctx)
			if err != nil {
				return err
			}
			if slices.Contains(delivered, conformanceNotificationID) {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("Delivered() = %v, want %q", delivered, conformanceNotificationID)
			}
			window.PollEvents()
			time.Sleep(50 * time.Millisecond)
		}
		if err := notifications.Withdraw(conformanceNotificationID); err != nil {
			return err
		}
		delivered, err := notifications.Delivered(ctx)
		if err != nil {
			return err
		}
		if slices.Contains(delivered, conformanceNotificationID) {
			return fmt.Errorf("Delivered() = %v after Withdraw", delivered)
		}
		return nil
	})
}

func checkExpectedSystemTray(r *recorder, window ui.Window) {
	r.check("system_tray/validation", func() error {
		tray, err := window.CreateSystemTray(ui.SystemTrayOptions{Tooltip: "No icon or title"})
		if tray != nil {
			tray.Close()
		}
		if err == nil {
			return errors.New("CreateSystemTray accepted an item without an icon or title")
		}
		if errors.Is(err, ui.ErrNotSupported) {
			return fmt.Errorf("CreateSystemTray validated after reporting %v", err)
		}
		return nil
	})

	menu, menuErr := window.CreateMenu(ui.MenuOptions{
		Actions: []ui.Action{
			{ID: "show", Text: "Show Window", Enabled: true},
			{ID: "quit", Text: "Quit", Enabled: true, Shortcut: ui.Shortcut{Key: "Q", Command: true}},
		},
		Items: []ui.MenuItem{{ActionID: "show"}, {Separator: true}, {ActionID: "quit"}},
	})
	r.check("system_tray/create_menu", func() error {
		return menuErr
	})
	if menu != nil {
		defer menu.Close()
	}

	tray, err := window.CreateSystemTray(ui.SystemTrayOptions{
		Icon:    ui.IconSource{SystemName: "game-controller"},
		Title:   "GK",
		Tooltip: "GameKit",
	})
	r.check("system_tray/create", func() error {
		return err
	})
	if tray == nil {
		return
	}
	defer tray.Close()

	r.check("system_tray/initial_state", func() error {
		if tray.Icon().SystemName != "game-controller" || tray.Title() != "GK" || tray.Tooltip() != "GameKit" {
			return fmt.Errorf("tray state = (%#v, %q, %q)", tray.Icon(), tray.Title(), tray.Tooltip())
		}
		if !tray.IsVisible() || tray.IsAttention() || tray.Menu() != nil {
			return errors.New("new tray item must be visible, without attention, and without a menu")
		}
		return nil
	})

	r.check("system_tray/setters", func() error {
		tray.SetTitle("Ready")
		tray.SetTooltip("GameKit is ready")
		if err := tray.SetIcon(ui.IconSource{Image: testImage()}); err != nil {
			return err
		}
		if tray.Title() != "Ready" || tray.Tooltip() != "GameKit is ready" || tray.Icon().Image == nil {
			return errors.New("tray setters did not update the item")
		}
		// A titled item may drop its icon.
		if err := tray.SetIcon(ui.IconSource{}); err != nil {
			return err
		}
		if !tray.Icon().IsZero() {
			return errors.New("SetIcon(IconSource{}) did not remove the icon")
		}
		return tray.SetIcon(ui.IconSource{SystemName: "game-controller"})
	})

	r.check("system_tray/visibility_and_attention", func() error {
		tray.SetVisible(false)
		if tray.IsVisible() {
			return errors.New("tray item remained visible")
		}
		tray.SetVisible(true)
		tray.SetAttention(true)
		if !tray.IsVisible() || !tray.IsAttention() {
			return errors.New("tray item did not become visible with attention")
		}
		tray.SetAttention(false)
		return nil
	})

	r.check("system_tray/menu", func() error {
		if menu == nil {
			return errors.New("menu was not created")
		}
		if err := tray.SetMenu(menu); err != nil {
			return err
		}
		if tray.Menu() != menu {
			return errors.New("Menu did not return the assigned menu")
		}
		if err := tray.SetMenu(nil); err != nil {
			return err
		}
		if tray.Menu() != nil {
			return errors.New("SetMenu(nil) did not remove the menu")
		}
		return nil
	})

	r.check("system_tray/bounds", func() error {
		if skip := skipOnLinux("StatusNotifierItem hosts do not report where they show an item"); skip != nil {
			if _, err := tray.Bounds(); !errors.Is(err, ui.ErrNotSupported) {
				return fmt.Errorf("Bounds() error = %v, want ui.ErrNotSupported", err)
			}
			return skip
		}
		var bounds ui.Rect
		var err error
		for i := 0; i < 100; i++ {
			window.PollEvents()
			if bounds, err = tray.Bounds(); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err != nil {
			return err
		}
		if !bounds.Size.IsValid() {
			return fmt.Errorf("Bounds() = %#v, want a positive size", bounds)
		}
		return nil
	})

	r.check("system_tray/subscription", func() error {
		unsubscribe := tray.OnActivate(func() {
		})
		if unsubscribe == nil {
			return errors.New("OnActivate returned nil")
		}
		unsubscribe()
		unsubscribe()
		return nil
	})

	r.check("system_tray/close", func() error {
		tray.Close()
		tray.Close()
		if err := tray.SetIcon(ui.IconSource{SystemName: "gear"}); !errors.Is(err, ui.ErrClosed) {
			return fmt.Errorf("SetIcon after Close error = %v, want ErrClosed", err)
		}
		return nil
	})
}
