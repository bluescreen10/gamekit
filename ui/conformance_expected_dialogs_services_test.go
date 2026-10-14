package ui_test

import (
	"context"
	"errors"
	"image"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func checkExpectedDialogsAndServices(r *recorder, window ui.Window) {
	checkExpectedDialogs(r, window)
	checkExpectedClipboard(r, window)
	checkExpectedPlatformServices(r, window)
}

func checkExpectedDialogs(r *recorder, window ui.Window) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r.check("dialogs/alert/pre_cancelled", func() error {
		_, err := window.ShowAlert(ctx, ui.AlertOptions{
			Title:       "Device lost",
			Message:     "The renderer stopped.",
			Details:     "The GPU device is no longer available.",
			Severity:    ui.SeverityError,
			Acknowledge: ui.DialogAction{ID: "ok", Text: "OK"},
		})
		return wantError(err, context.Canceled)
	})

	r.check("dialogs/confirmation/pre_cancelled", func() error {
		_, err := window.ShowConfirmation(ctx, ui.ConfirmationOptions{
			Title:         "Delete scene?",
			Message:       "This action cannot be undone.",
			Actions:       []ui.DialogAction{{ID: "delete", Text: "Delete", Destructive: true}},
			DefaultAction: "delete",
			CancelAction:  "cancel",
		})
		return wantError(err, context.Canceled)
	})

	r.check("dialogs/prompt/pre_cancelled", func() error {
		_, err := window.ShowPrompt(ctx, ui.PromptOptions{
			Title:       "Rename scene",
			Message:     "Enter a new name.",
			Text:        "Scene",
			Placeholder: "Name",
		})
		return wantError(err, context.Canceled)
	})

	r.check("dialogs/about/pre_cancelled", func() error {
		err := window.ShowAbout(ctx, ui.AboutOptions{
			Name:      "GameKit",
			Version:   "1.0",
			Copyright: "Copyright 2026",
			Website:   "https://example.com",
		})
		return wantError(err, context.Canceled)
	})

	r.check("dialogs/file/complete_options", func() error {
		_, err := window.OpenFile(ctx, ui.OpenFileOptions{
			Title:           "Open",
			Directory:       ".",
			Filename:        "scene.gltf",
			Filters:         []ui.FileFilter{{Name: "Scenes", Patterns: []string{"*.gltf", "*.glb"}}},
			SelectedFilter:  0,
			ShowHiddenFiles: true,
			SelectPackages:  false,
		})
		return wantError(err, context.Canceled)
	})

	r.check("dialogs/save/complete_options", func() error {
		_, err := window.SaveFile(ctx, ui.SaveFileOptions{
			Title:            "Save",
			Filename:         "scene.gltf",
			Filters:          []ui.FileFilter{{Name: "glTF", Patterns: []string{"*.gltf"}}},
			SelectedFilter:   0,
			DefaultExtension: "gltf",
			ConfirmOverwrite: true,
			ShowHiddenFiles:  true,
		})
		return wantError(err, context.Canceled)
	})
}

func checkExpectedClipboard(r *recorder, window ui.Window) {
	r.check("services/clipboard", func() error {
		clipboard := window.Clipboard()
		original, err := clipboard.Read()
		if err != nil {
			return err
		}
		defer clipboard.Write(original)
		if err := clipboard.Write(ui.ClipboardContent{
			Text:  "GameKit",
			Image: image.NewRGBA(image.Rect(0, 0, 2, 2)),
			Files: []string{"scene.gltf"},
			Custom: map[string][]byte{
				"application/x-gamekit": {1, 2, 3},
			},
		}); err != nil {
			return err
		}
		content, err := clipboard.Read()
		if err != nil {
			return err
		}
		if content.Text != "GameKit" || len(content.Files) != 1 {
			return errors.New("clipboard did not preserve its content")
		}
		return clipboard.Clear()
	})
}

func checkExpectedPlatformServices(r *recorder, window ui.Window) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r.check("services/subscriptions", func() error {
		unsubscribes := []ui.Unsubscribe{
			window.OnActivated(func() {
			}),
			window.OnDeactivated(func() {
			}),
			window.OnReopen(func() {
			}),
			window.OnQuitRequest(func() bool {
				return false
			}),
			window.Clipboard().OnChange(func() {
			}),
			window.Notifications().OnActivate(func(ui.NotificationActivation) {
			}),
		}
		for _, unsubscribe := range unsubscribes {
			if unsubscribe == nil {
				return errors.New("service subscription returned nil")
			}
			unsubscribe()
		}
		return nil
	})

	r.check("services/open_url", func() error {
		return wantError(window.OpenURL(ctx, "https://example.com", ui.OpenURLOptions{}), context.Canceled)
	})
	r.check("services/reveal_file", func() error {
		return wantError(window.RevealFile(ctx, "scene.gltf", ui.RevealFileOptions{Select: true}), context.Canceled)
	})
	r.check("services/notification", func() error {
		return wantError(window.Notifications().Send(ctx, ui.Notification{
			ID:       "render-complete",
			Title:    "Render complete",
			Body:     "The image is ready.",
			Actions:  []ui.NotificationAction{{ID: "show", Text: "Show"}},
			Delivery: time.Now(),
		}), context.Canceled)
	})
}

func checkExpectedPlatformIntegrations(r *recorder, window ui.Window) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r.check("services/system_tray", func() error {
		tray, err := window.CreateSystemTray(ui.SystemTrayOptions{
			Icon:    ui.IconSource{SystemName: "game-controller"},
			Tooltip: "GameKit",
		})
		if err != nil {
			return err
		}
		tray.Close()
		tray.Close()
		return nil
	})

	r.check("services/recent_documents", func() error {
		recent := window.RecentDocuments()
		if err := recent.Add("scene.gltf"); err != nil {
			return err
		}
		if err := recent.Remove("scene.gltf"); err != nil {
			return err
		}
		return nil
	})

	r.check("services/file_associations", func() error {
		_, err := window.FileAssociations().DefaultApplication(".gltf")
		return err
	})

	r.check("services/printing/pre_cancelled", func() error {
		_, err := window.Print(ctx, ui.PrintOptions{
			Title: "Scene",
			PageCount: func() int {
				return 1
			},
			RenderPage: func(int, ui.PrintCanvas) error {
				return nil
			},
		})
		return wantError(err, context.Canceled)
	})

	r.check("services/share/pre_cancelled", func() error {
		_, err := window.Share(ctx, ui.ShareOptions{
			Items: []ui.ShareItem{{Text: "GameKit", URL: "https://example.com"}},
		})
		return wantError(err, context.Canceled)
	})
}
