package ui

import (
	"context"
	"errors"
	"fmt"
	stdimage "image"
	"time"
)

// ClipboardContent is clipboard data in several representations. Custom keys
// are MIME types; backends map them to native types such as UTIs.
type ClipboardContent struct {
	// Text is the plain text representation.
	Text string

	// Image is the image representation.
	Image stdimage.Image

	// Files are the paths of copied files.
	Files []string

	// Custom maps MIME types to data in that type.
	Custom map[string][]byte
}

// IsEmpty reports whether the content has no representation.
func (content ClipboardContent) IsEmpty() bool {
	return content.Text == "" && content.Image == nil && len(content.Files) == 0 && len(content.Custom) == 0
}

// Clipboard is the system clipboard. Write replaces all representations.
// OnChange reports changes made by any application.
type Clipboard interface {
	// Read returns every representation that the clipboard holds.
	Read() (ClipboardContent, error)

	// Write replaces the content of the clipboard with content.
	Write(content ClipboardContent) error

	// Clear removes the content of the clipboard.
	Clear() error

	// Formats lists the MIME types currently available.
	Formats() ([]string, error)

	// OnChange receives each change to the clipboard. Handlers run while
	// the application polls events.
	OnChange(handler FocusHandler) Unsubscribe
}

// OpenURLOptions controls OpenURL.
type OpenURLOptions struct {
	// Background opens the URL without activating the handling application.
	Background bool
}

// RevealFileOptions controls RevealFile.
type RevealFileOptions struct {
	// Select selects the file instead of only opening its directory.
	Select bool
}

// NotificationAuthorization is the user's permission for notifications.
type NotificationAuthorization uint8

const (
	// NotificationNotDetermined means that the user has not answered the
	// permission request yet.
	NotificationNotDetermined NotificationAuthorization = iota

	// NotificationDenied means that the user does not allow notifications.
	NotificationDenied

	// NotificationAuthorized means that the user allows notifications.
	NotificationAuthorized

	// NotificationProvisional delivers quietly without an explicit prompt.
	NotificationProvisional
)

// NotificationAction is a button shown on a delivered notification.
type NotificationAction struct {
	// ID identifies the action in NotificationActivation.ActionID.
	ID string

	// Text is the label of the button.
	Text string

	// Destructive marks an action that deletes data or cannot be undone.
	Destructive bool

	// TextInput shows a reply field; the reply arrives in
	// NotificationActivation.ReplyText.
	TextInput bool
}

// Notification is a user notification. IDs are stable: sending a notification
// with the ID of a pending or delivered one replaces it. A zero Delivery shows
// it immediately; a future Delivery schedules it.
type Notification struct {
	// ID identifies the notification for Withdraw and in activations.
	ID string

	// Title is the main text of the notification.
	Title string

	// Subtitle is the text below the title.
	Subtitle string

	// Body is the message of the notification.
	Body string

	// Icon is an image that the notification shows as an attachment.
	Icon IconSource

	// Actions are the buttons of the notification.
	Actions []NotificationAction

	// Category groups notifications that share actions and settings.
	Category string

	// ThreadID groups related notifications in the notification center.
	ThreadID string

	// Sound plays the default notification sound.
	Sound bool

	// Delivery is the time to show the notification.
	Delivery time.Time
}

// Validate checks that the notification can be delivered.
func (notification Notification) Validate() error {
	if notification.ID == "" {
		return errors.New("ui: notification requires an ID")
	}
	if notification.Title == "" && notification.Body == "" {
		return errors.New("ui: notification requires a title or body")
	}
	seen := make(map[string]struct{}, len(notification.Actions))
	for i, action := range notification.Actions {
		if action.ID == "" || action.Text == "" {
			return fmt.Errorf("ui: notification action %d requires an ID and text", i)
		}
		if _, exists := seen[action.ID]; exists {
			return fmt.Errorf("ui: duplicate notification action %q", action.ID)
		}
		seen[action.ID] = struct{}{}
	}
	return nil
}

// NotificationActivation reports the user interacting with a notification.
// ActionID is empty when the user activated the notification itself.
type NotificationActivation struct {
	// NotificationID is the ID of the notification.
	NotificationID string

	// ActionID is the ID of the chosen action.
	ActionID string

	// ReplyText is the text that the user entered in a TextInput action.
	ReplyText string
}

// NotificationActivationHandler receives notification interactions.
type NotificationActivationHandler func(activation NotificationActivation)

// Notifications deliver user notifications through the platform notification
// center. On macOS this requires the application to run from a bundle with a
// bundle identifier; otherwise methods return ErrNotSupported.
type Notifications interface {
	// Authorization reports the current permission without prompting.
	Authorization(ctx context.Context) (NotificationAuthorization, error)

	// RequestAuthorization prompts the user when permission is not yet
	// determined and returns the resulting permission.
	RequestAuthorization(ctx context.Context) (NotificationAuthorization, error)

	// Send validates notification and shows or schedules it. It returns an
	// error when the user does not allow notifications.
	Send(ctx context.Context, notification Notification) error

	// Pending returns the IDs of scheduled notifications not yet delivered.
	Pending(ctx context.Context) ([]string, error)

	// Delivered returns the IDs of notifications shown in the notification
	// center.
	Delivered(ctx context.Context) ([]string, error)

	// Withdraw removes pending and delivered notifications by ID.
	Withdraw(ids ...string) error

	// SetBadge sets the application icon badge; an empty string clears it.
	SetBadge(badge string) error

	// OnActivate receives each interaction of the user with a notification
	// of the application.
	OnActivate(handler NotificationActivationHandler) Unsubscribe
}

// SystemTrayOptions describes a status item: a menu-bar extra on macOS, a
// notification-area icon on Windows, and a StatusNotifierItem on Linux. Title
// is shown beside the icon where the platform supports it.
type SystemTrayOptions struct {
	// Icon is the image of the item. A template image adapts to the menu
	// bar appearance on macOS.
	Icon IconSource

	// Title is the text of the item.
	Title string

	// Tooltip is the help text of the item.
	Tooltip string

	// Menu is the menu that a click opens. It can be nil.
	Menu Menu
}

// Validate checks that the status item has something to show.
func (options SystemTrayOptions) Validate() error {
	if options.Icon.IsZero() && options.Title == "" {
		return errors.New("ui: system tray item requires an icon or title")
	}
	return nil
}

// SystemTray is an application status item. When it has a menu, a primary
// click opens the menu; otherwise OnActivate receives the click. Attention
// requests the user's notice, such as a highlighted or blinking icon.
type SystemTray interface {
	// Close removes the item. The window that created the item does not
	// close it.
	Close()

	// Icon returns the image of the item.
	Icon() IconSource

	// SetIcon changes the image of the item.
	SetIcon(icon IconSource) error

	// Title returns the text of the item.
	Title() string

	// SetTitle changes the text of the item.
	SetTitle(title string)

	// Tooltip returns the help text of the item.
	Tooltip() string

	// SetTooltip changes the help text of the item.
	SetTooltip(tooltip string)

	// Menu returns the menu of the item, or nil.
	Menu() Menu

	// SetMenu changes the menu of the item. A nil menu sends clicks to
	// OnActivate.
	SetMenu(menu Menu) error

	// IsVisible reports whether the item is shown.
	IsVisible() bool

	// SetVisible shows or hides the item.
	SetVisible(visible bool)

	// IsAttention reports whether the item asks for the user's notice.
	IsAttention() bool

	// SetAttention starts or stops the request for the user's notice.
	SetAttention(attention bool)

	// Bounds is the item's screen rectangle in global logical coordinates.
	// It returns an error until the system shows the item, which can take
	// some event polls after the item is created.
	Bounds() (Rect, error)

	// OnActivate receives clicks on an item that has no menu.
	OnActivate(handler FocusHandler) Unsubscribe
}

// RecentDocuments is the platform-managed list of recently opened documents.
type RecentDocuments interface {
	// Add puts path at the start of the list.
	Add(path string) error

	// Remove takes path out of the list.
	Remove(path string) error

	// Clear removes all documents from the list.
	Clear() error

	// List returns the paths in the list, the most recent first.
	List() ([]string, error)
}

// FileAssociations queries the platform's document handlers.
type FileAssociations interface {
	// DefaultApplication returns the path of the application that opens
	// files with extension, such as ".gltf".
	DefaultApplication(extension string) (string, error)
}

// PrintCanvas draws one printed page. Coordinates are in points with the
// origin at the top-left corner of the printable area.
type PrintCanvas interface {
	// Size returns the size of the printable area in points.
	Size() Size

	// DrawImage draws img scaled to fill bounds.
	DrawImage(img stdimage.Image, bounds Rect) error

	// DrawText draws text with its top-left corner at position.
	DrawText(text string, position Point, font Font) error
}

// PrintOptions describes a print job. RenderPage receives zero-based page
// indexes.
type PrintOptions struct {
	// Title is the name of the print job.
	Title string

	// PageCount returns the number of pages to print.
	PageCount func() int

	// RenderPage draws one page on canvas.
	RenderPage func(page int, canvas PrintCanvas) error

	// ShowDialog shows the print panel before the job starts.
	ShowDialog bool
}

// Validate checks that the job can paginate and render.
func (options PrintOptions) Validate() error {
	if options.PageCount == nil || options.RenderPage == nil {
		return errors.New("ui: print requires PageCount and RenderPage")
	}
	return nil
}

// PrintResult reports a completed print job.
type PrintResult struct {
	// Pages is the number of pages that the job printed.
	Pages int
}

// ShareItem is one item offered to the share UI.
type ShareItem struct {
	// Text is text to share.
	Text string

	// URL is a URL to share.
	URL string

	// File is the path of a file to share.
	File string

	// Image is an image to share.
	Image stdimage.Image
}

// ShareOptions describes a share request. Source anchors the share UI.
type ShareOptions struct {
	// Items are the things to share.
	Items []ShareItem

	// Source is the widget that the share UI points to. It can be nil.
	Source Widget
}

// Validate checks that something is shared.
func (options ShareOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: share requires at least one item")
	}
	return nil
}
