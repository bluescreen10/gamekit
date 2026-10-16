package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func serviceCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Clipboard", "Copy and paste text and images, and list the formats on the clipboard.", g.clipboardDemo),
		g.card("Notifications", "System notifications need permission. On macOS they also need an application bundle.", g.notificationsDemo),
		g.card("System tray", "An item in the menu bar extras or the notification area, with a menu.", g.systemTrayDemo),
		g.card("Recent documents", "The list of recent documents that the platform shows for this application.", g.recentDocumentsDemo),
		g.card("URLs, files, and associations", "Open a URL, reveal a file in the file manager, and look up the default application of a file type.", g.openRevealDemo),
		g.card("Print and share", "Print two generated pages, and share text and a URL.", g.printShareDemo),
	}
}

func (g *gallery) clipboardDemo() ui.Widget {
	clipboard := g.window.Clipboard()
	text := must(g.window.CreateTextField())
	text.SetText("Copied from the GameKit gallery")
	contents := g.value("Clipboard: not read")
	contents.SetWrapMode(ui.WrapWords)
	clipboard.OnChange(func() {
		g.log("Clipboard changed")
	})
	copyText := g.button("Copy text", func() {
		if err := clipboard.Write(ui.ClipboardContent{Text: text.Text()}); err != nil {
			g.log("Copy: %v", err)
		}
	})
	copyImage := g.button("Copy image", func() {
		if err := clipboard.Write(ui.ClipboardContent{Image: swatchImage(64, 0.1)}); err != nil {
			g.log("Copy image: %v", err)
		}
	})
	paste := g.button("Paste", func() {
		content, err := clipboard.Read()
		if err != nil {
			g.log("Paste: %v", err)
			return
		}
		parts := []string{}
		if content.Text != "" {
			parts = append(parts, fmt.Sprintf("text %q", content.Text))
		}
		if content.Image != nil {
			size := content.Image.Bounds().Size()
			parts = append(parts, fmt.Sprintf("image %d × %d", size.X, size.Y))
		}
		if len(content.Files) > 0 {
			parts = append(parts, "files "+strings.Join(content.Files, ", "))
		}
		if content.IsEmpty() {
			parts = append(parts, "empty")
		}
		contents.SetText("Clipboard: " + strings.Join(parts, "; "))
	})
	formats := g.button("Formats", func() {
		list, err := clipboard.Formats()
		if err != nil {
			g.log("Formats: %v", err)
			return
		}
		contents.SetText("Formats: " + strings.Join(list, ", "))
	})
	clear := g.button("Clear", func() {
		if err := clipboard.Clear(); err != nil {
			g.log("Clear: %v", err)
		}
	})
	return g.column(g.labeledWide("Text", text), g.row(copyText, copyImage, paste, formats, clear), contents)
}

func (g *gallery) notificationsDemo() ui.Widget {
	notifications := g.window.Notifications()
	status := g.value("Authorization: unknown")
	describe := func(authorization ui.NotificationAuthorization) string {
		return []string{"not determined", "denied", "authorized", "provisional"}[authorization]
	}
	check := func() {
		authorization, err := notifications.Authorization(context.Background())
		if err != nil {
			status.SetText("Authorization: " + err.Error())
			return
		}
		status.SetText("Authorization: " + describe(authorization))
	}
	check()
	status.SetWrapMode(ui.WrapWords)
	notifications.OnActivate(func(activation ui.NotificationActivation) {
		g.log("Notification %s activated with action %q, reply %q",
			activation.NotificationID, activation.ActionID, activation.ReplyText)
	})

	request := g.button("Request permission", func() {
		authorization, err := notifications.RequestAuthorization(context.Background())
		if err != nil {
			g.log("Notification permission: %v", err)
			return
		}
		status.SetText("Authorization: " + describe(authorization))
	})
	sent := 0
	send := func(notification ui.Notification) {
		sent++
		notification.ID = fmt.Sprintf("gallery-%d", sent)
		if err := notifications.Send(context.Background(), notification); err != nil {
			g.log("Send notification: %v", err)
			return
		}
		g.log("Sent notification %s", notification.ID)
	}
	simple := g.button("Send", func() {
		send(ui.Notification{Title: "Build finished", Body: "The project built in 4.2 seconds.", Sound: true})
	})
	actions := g.button("Send with actions", func() {
		send(ui.Notification{
			Title:    "New message",
			Subtitle: "From the level designer",
			Body:     "Can you look at the boss arena?",
			Category: "message",
			ThreadID: "designer",
			Actions: []ui.NotificationAction{
				{ID: "reply", Text: "Reply", TextInput: true},
				{ID: "dismiss", Text: "Dismiss", Destructive: true},
			},
		})
	})
	later := g.button("Send in 10 seconds", func() {
		send(ui.Notification{Title: "Reminder", Body: "Ten seconds have passed.", Delivery: time.Now().Add(10 * time.Second)})
	})
	list := g.button("List", func() {
		pending, pendingErr := notifications.Pending(context.Background())
		delivered, deliveredErr := notifications.Delivered(context.Background())
		g.log("Pending %v (%v), delivered %v (%v)", pending, pendingErr, delivered, deliveredErr)
	})
	withdraw := g.button("Withdraw all", func() {
		ids := []string{}
		for i := 1; i <= sent; i++ {
			ids = append(ids, fmt.Sprintf("gallery-%d", i))
		}
		if err := notifications.Withdraw(ids...); err != nil {
			g.log("Withdraw: %v", err)
		}
	})
	badge := g.checkBox("Application badge", false, func(checked bool) {
		text := ""
		if checked {
			text = "5"
		}
		if err := notifications.SetBadge(text); err != nil {
			g.log("Badge: %v", err)
		}
	})
	return g.column(status, g.row(request, simple, actions, later), g.row(list, withdraw, badge))
}

func (g *gallery) systemTrayDemo() ui.Widget {
	var tray ui.SystemTray
	create := g.button("Add tray item", func() {
		if tray != nil {
			return
		}
		menu, err := g.window.CreateMenu(ui.MenuOptions{
			Actions: []ui.Action{
				{ID: "show", Text: "Show Gallery", Enabled: true},
				{ID: "pause", Text: "Pause Sync", Enabled: true},
				{ID: "quit", Text: "Quit", Enabled: true},
			},
			Items: []ui.MenuItem{{ActionID: "show"}, {ActionID: "pause"}, {Separator: true}, {ActionID: "quit"}},
		})
		if err != nil {
			g.log("Tray menu: %v", err)
			return
		}
		menu.OnAction(func(id string) {
			g.log("Tray menu action: %s", id)
			switch id {
			case "show":
				g.window.BringToFront()
			case "quit":
				g.window.SetShouldClose(true)
			}
		})
		next, err := g.window.CreateSystemTray(ui.SystemTrayOptions{
			Icon:    ui.IconSource{SystemName: "game-controller"},
			Title:   "GameKit",
			Tooltip: "GameKit widget gallery",
			Menu:    menu,
		})
		if err != nil {
			menu.Close()
			g.log("System tray: %v", err)
			return
		}
		next.OnActivate(func() {
			g.log("Tray item activated")
		})
		tray = next
		g.log("Tray item added")
	})
	remove := g.button("Remove", func() {
		if tray != nil {
			tray.Close()
			tray = nil
		}
	})
	attention := g.checkBox("Request attention", false, func(checked bool) {
		if tray != nil {
			tray.SetAttention(checked)
		}
	})
	title := g.checkBox("Show title", true, func(checked bool) {
		if tray == nil {
			return
		}
		if checked {
			tray.SetTitle("GameKit")
			return
		}
		tray.SetTitle("")
	})
	bounds := g.button("Report bounds", func() {
		if tray == nil {
			return
		}
		rect, err := tray.Bounds()
		g.log("Tray bounds: %+v (%v), visible %t", rect, err, tray.IsVisible())
	})
	return g.row(create, remove, attention, title, bounds)
}

func (g *gallery) recentDocumentsDemo() ui.Widget {
	recent := g.window.RecentDocuments()
	list := g.value("Not read")
	list.SetWrapMode(ui.WrapCharacters)
	refresh := func() {
		paths, err := recent.List()
		if err != nil {
			list.SetText(err.Error())
			return
		}
		if len(paths) == 0 {
			list.SetText("No recent documents")
			return
		}
		list.SetText(strings.Join(paths, "\n"))
	}
	add := g.button("Add a file…", func() {
		path, err := g.window.OpenFile(context.Background(), ui.OpenFileOptions{Title: "Add a recent document"})
		if err != nil {
			g.reportDialog("Add recent document", path, err)
			return
		}
		if err := recent.Add(path); err != nil {
			g.log("Add recent document: %v", err)
		}
		refresh()
	})
	clear := g.button("Clear", func() {
		if err := recent.Clear(); err != nil {
			g.log("Clear recent documents: %v", err)
		}
		refresh()
	})
	return g.column(g.row(add, clear, g.button("Refresh", refresh)), list)
}

func (g *gallery) openRevealDemo() ui.Widget {
	address := must(g.window.CreateTextField())
	address.SetText("https://github.com/bluescreen10/gamekit")
	open := g.button("Open URL", func() {
		if err := g.window.OpenURL(context.Background(), address.Text(), ui.OpenURLOptions{}); err != nil {
			g.log("Open URL: %v", err)
		}
	})
	background := g.button("Open in background", func() {
		if err := g.window.OpenURL(context.Background(), address.Text(), ui.OpenURLOptions{Background: true}); err != nil {
			g.log("Open URL: %v", err)
		}
	})
	reveal := g.button("Reveal a file…", func() {
		path, err := g.window.OpenFile(context.Background(), ui.OpenFileOptions{Title: "Choose a file to reveal"})
		if err != nil {
			g.reportDialog("Reveal", path, err)
			return
		}
		if err := g.window.RevealFile(context.Background(), path, ui.RevealFileOptions{Select: true}); err != nil {
			g.log("Reveal: %v", err)
		}
	})

	extension := must(g.window.CreateTextField())
	extension.SetText(".txt")
	application := g.value("")
	lookup := func() {
		name, err := g.window.FileAssociations().DefaultApplication(extension.Text())
		if err != nil {
			application.SetText(err.Error())
			return
		}
		application.SetText(name)
	}
	extension.OnSubmit(func(string) {
		lookup()
	})
	return g.column(
		g.labeledWide("URL", address),
		g.row(open, background, reveal),
		g.labeled("Extension", fixed(extension, ui.Size{Width: 100}), g.button("Look up", lookup), application),
	)
}

func (g *gallery) printShareDemo() ui.Widget {
	printDialog := g.button("Print…", func() {
		result, err := g.window.Print(context.Background(), ui.PrintOptions{
			Title: "GameKit gallery",
			PageCount: func() int {
				return 2
			},
			RenderPage: func(page int, canvas ui.PrintCanvas) error {
				size := canvas.Size()
				if err := canvas.DrawText(fmt.Sprintf("GameKit gallery, page %d", page+1), ui.Point{X: 72, Y: 72},
					ui.Font{Size: 24, Weight: ui.FontWeightBold}); err != nil {
					return err
				}
				return canvas.DrawImage(gradientImage(192, 128), ui.Rect{
					Position: ui.Point{X: 72, Y: 120},
					Size:     ui.Size{Width: size.Width - 144, Height: (size.Width - 144) * 2 / 3},
				})
			},
			ShowDialog: true,
		})
		if errors.Is(err, ui.ErrCancelled) {
			g.log("Print cancelled")
			return
		}
		if err != nil {
			g.log("Print: %v", err)
			return
		}
		g.log("Printed %d pages", result.Pages)
	})
	var share ui.Button
	share = g.button("Share…", func() {
		target, err := g.window.Share(context.Background(), ui.ShareOptions{
			Items:  []ui.ShareItem{{Text: "GameKit builds native game tools.", URL: "https://github.com/bluescreen10/gamekit"}},
			Source: share,
		})
		g.reportDialog("Share", "shared with "+target, err)
	})
	return g.row(printDialog, share)
}

func eventLogCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Events", "The status bar shows the newest event.", g.eventLogDemo),
	}
}

func (g *gallery) eventLogDemo() ui.Widget {
	area := must(g.window.CreateTextArea(ui.TextAreaOptions{ReadOnly: true, Text: strings.Join(g.events, "\n")}))
	tall(area, 420)
	g.eventLog = area
	clear := g.button("Clear", func() {
		g.events = nil
		area.SetText("")
	})
	copyLog := g.button("Copy", func() {
		if err := g.window.Clipboard().Write(ui.ClipboardContent{Text: strings.Join(g.events, "\n")}); err != nil {
			g.log("Copy log: %v", err)
		}
	})
	return g.column(area, g.row(clear, copyLog))
}
