//go:build linux && cgo

package gtk_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

func checkGTKClipboard(window ui.Window) error {
	clipboard := window.Clipboard()
	original, err := clipboard.Read()
	if err != nil {
		return err
	}
	defer clipboard.Write(original)
	picture := image.NewNRGBA(image.Rect(2, 3, 4, 5))
	picture.SetNRGBA(2, 3, color.NRGBA{R: 255, G: 64, B: 32, A: 128})
	picture.SetNRGBA(3, 4, color.NRGBA{R: 40, G: 255, B: 120, A: 255})
	data := bytes.Repeat([]byte{0, 1, 2, 255}, 70000) // Multiple async stream chunks.
	content := ui.ClipboardContent{
		Text:  "GTK clipboard π\nsecond line",
		Files: []string{"first file.txt", "second.txt"},
		Image: picture,
		Custom: map[string][]byte{
			"application/x-gamekit-test":  data,
			"application/x-gamekit-empty": {},
		},
	}
	if err := clipboard.Write(content); err != nil {
		return err
	}
	read, err := clipboard.Read()
	if err != nil {
		return err
	}
	if read.Text != content.Text || len(read.Files) != 2 {
		return fmt.Errorf("clipboard text/files = %q, %v", read.Text, read.Files)
	}
	for i, path := range content.Files {
		absolute, _ := filepath.Abs(path)
		if read.Files[i] != absolute {
			return fmt.Errorf("clipboard file = %q, want %q", read.Files[i], absolute)
		}
	}
	if read.Image == nil || read.Image.Bounds().Size() != image.Pt(2, 2) {
		return errors.New("clipboard image dimensions were lost")
	}
	pixel := color.NRGBAModel.Convert(read.Image.At(0, 0)).(color.NRGBA)
	if pixel != picture.NRGBAAt(2, 3) {
		return fmt.Errorf("clipboard pixel = %v, want %v", pixel, picture.NRGBAAt(2, 3))
	}
	for mime, expected := range content.Custom {
		value, ok := read.Custom[mime]
		if !ok || !bytes.Equal(value, expected) {
			return fmt.Errorf("clipboard %q = %d bytes, want %d", mime, len(value), len(expected))
		}
	}
	formats, err := clipboard.Formats()
	if err != nil {
		return err
	}
	for _, mime := range []string{"text/plain", "text/uri-list", "application/x-gamekit-test", "application/x-gamekit-empty"} {
		if !slices.Contains(formats, mime) {
			return fmt.Errorf("clipboard formats missing %q: %v", mime, formats)
		}
	}
	if err := clipboard.Clear(); err != nil {
		return err
	}
	read, err = clipboard.Read()
	if err != nil || !read.IsEmpty() {
		return fmt.Errorf("clipboard clear returned %+v, %v", read, err)
	}
	return nil
}

func checkGTKMessageDialogs(window ui.Window) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	respond := gtk.ScheduleNativeResponse("Go confirmation", "Delete", "", false)
	action, err := window.ShowConfirmation(ctx, ui.ConfirmationOptions{
		Title: "Go confirmation", Message: "Choose a GTK action.", Details: "Expanded details",
		Actions:       []ui.DialogAction{{ID: "keep", Text: "Keep"}, {ID: "delete", Text: "Delete", Destructive: true}},
		DefaultAction: "keep", CancelAction: "cancel", Severity: ui.SeverityWarning,
	})
	respond()
	if err != nil || action != "delete" {
		return fmt.Errorf("confirmation = %q, %v", action, err)
	}
	for _, secure := range []bool{false, true} {
		respond = gtk.ScheduleNativeResponse("Go prompt", "OK", "edited π text", false)
		text, err := window.ShowPrompt(ctx, ui.PromptOptions{Title: "Go prompt", Text: "initial", Placeholder: "Enter text", Secure: secure})
		respond()
		if err != nil || text != "edited π text" {
			return fmt.Errorf("prompt (secure=%v) = %q, %v", secure, text, err)
		}
	}
	respond = gtk.ScheduleNativeResponse("Go prompt cancel", "", "", true)
	_, err = window.ShowPrompt(ctx, ui.PromptOptions{Title: "Go prompt cancel"})
	respond()
	if !errors.Is(err, ui.ErrCancelled) {
		return fmt.Errorf("closed prompt = %v, want cancelled", err)
	}
	deadline, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	_, err = window.ShowAlert(deadline, ui.AlertOptions{Title: "Go alert timeout", Message: "Closes when context expires"})
	if !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("alert timeout = %v", err)
	}
	return nil
}

func checkGTKSharing(window ui.Window) error {
	original, err := window.Clipboard().Read()
	if err != nil {
		return err
	}
	defer window.Clipboard().Write(original)
	window.SetVisible(true)
	defer window.SetVisible(false)
	// Poll native events so GTK maps the source before the picker is opened.
	gtk.PollEvents()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	respond := gtk.ScheduleNativeResponse(window.Title(), "Copy", "", false)
	service, err := window.Share(ctx, ui.ShareOptions{Items: []ui.ShareItem{{Text: "shared text", URL: "https://example.com", File: "shared file.txt"}}})
	respond()
	if err != nil || service != "Copy" {
		return fmt.Errorf("share = %q, %v", service, err)
	}
	read, err := window.Clipboard().Read()
	if err != nil || read.Text != "shared text\nhttps://example.com" || len(read.Files) != 1 {
		return fmt.Errorf("shared clipboard = %+v, %v", read, err)
	}
	return nil
}

func TestGTKNativeServices(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	for _, name := range []string{"clipboard", "message dialogs", "sharing", "notifications", "file dialogs", "Go native port"} {
		t.Run(name, func(t *testing.T) {
			err := gtkWidgets.serviceErrors[name]
			if name == "notifications" && errors.Is(err, ui.ErrNotSupported) {
				t.Skip(err)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func checkGTKNotifications(window ui.Window) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	center := window.Notifications()
	authorization, err := center.Authorization(ctx)
	if errors.Is(err, ui.ErrNotSupported) {
		return err
	}
	if err != nil || authorization != ui.NotificationAuthorized {
		return fmt.Errorf("notification authorization = %v, %v", authorization, err)
	}
	const immediate = "gamekit-go-immediate"
	const scheduled = "gamekit-go-scheduled"
	defer center.Withdraw(immediate, scheduled)
	notification := ui.Notification{ID: immediate, Title: "Go notification", Subtitle: "GTK binding", Body: "Native delivery", Actions: []ui.NotificationAction{{ID: "reply", Text: "Reply", TextInput: true}}}
	if err := center.Send(ctx, notification); err != nil {
		return err
	}
	notification.Title = "Replacement notification"
	if err := center.Send(ctx, notification); err != nil {
		return err
	}
	delivered, err := center.Delivered(ctx)
	if err != nil || !slices.Contains(delivered, immediate) {
		return fmt.Errorf("delivered notifications = %v, %v", delivered, err)
	}
	notification.ID = scheduled
	notification.Delivery = time.Now().Add(time.Minute)
	if err := center.Send(ctx, notification); err != nil {
		return err
	}
	pending, err := center.Pending(ctx)
	if err != nil || !slices.Contains(pending, scheduled) {
		return fmt.Errorf("pending notifications = %v, %v", pending, err)
	}
	if err := center.Withdraw(scheduled); err != nil {
		return err
	}
	pending, err = center.Pending(ctx)
	if err != nil || slices.Contains(pending, scheduled) {
		return fmt.Errorf("withdrawn pending notifications = %v, %v", pending, err)
	}
	notification.Delivery = time.Now().Add(20 * time.Millisecond)
	if err := center.Send(ctx, notification); err != nil {
		return err
	}
	for {
		gtk.PollEvents()
		delivered, err = center.Delivered(ctx)
		if err != nil {
			return err
		}
		if slices.Contains(delivered, scheduled) {
			break
		}
		if ctx.Err() != nil {
			return errors.New("scheduled notification was not delivered")
		}
		time.Sleep(time.Millisecond)
	}
	if err := center.SetBadge("7"); err != nil {
		return err
	}
	return center.SetBadge("")
}

func checkGTKFileDialogs(window ui.Window) error {
	panels := []struct {
		name string
		show func(context.Context) error
	}{
		{"open", func(ctx context.Context) error {
			_, err := window.OpenFile(ctx, ui.OpenFileOptions{Title: "Go open cancellation", Filters: []ui.FileFilter{{Patterns: []string{"*.txt"}}}})
			return err
		}},
		{"open multiple", func(ctx context.Context) error {
			_, err := window.OpenFiles(ctx, ui.OpenFileOptions{Title: "Go multiple cancellation"})
			return err
		}},
		{"select directory", func(ctx context.Context) error {
			_, err := window.SelectDirectory(ctx, ui.DirectoryDialogOptions{Title: "Go folder cancellation"})
			return err
		}},
		{"save", func(ctx context.Context) error {
			_, err := window.SaveFile(ctx, ui.SaveFileOptions{Title: "Go save cancellation", Filename: "scene", DefaultExtension: ".txt"})
			return err
		}},
	}
	for _, panel := range panels {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		err := panel.show(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("%s panel cancellation = %v", panel.name, err)
		}
		// Consume GTK's async cancellation result after the public call returns.
		gtk.PollEvents()
	}
	return nil
}
