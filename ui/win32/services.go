//go:build windows

package win32

import (
	"context"
	"errors"
	"fmt"
	stdimage "image"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Clipboard ----

// clipboard is the general pasteboard. Changes made by any application are
// detected while windows poll events.
type clipboard struct {
	window *Window
}

var (
	clipboardMu    sync.Mutex
	clipboardCount = -1
)

// Clipboard returns the system clipboard.
func (w *Window) Clipboard() ui.Clipboard {
	return clipboard{window: w}
}

func (clipboard) Read() (ui.ClipboardContent, error) {
	return readClipboard(), nil
}

func (clipboard) Write(content ui.ClipboardContent) error {
	content.Files = absolutePaths(content.Files)
	if !writeClipboard(content) {
		return errors.New("ui/win32: the clipboard rejected the content")
	}
	return nil
}

func absolutePaths(paths []string) []string {
	result := make([]string, len(paths))
	for i, path := range paths {
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
		result[i] = path
	}
	return result
}

func (clipboard) Clear() error {
	clearClipboard()
	return nil
}

func (clipboard) Formats() ([]string, error) {
	return clipboardFormats(), nil
}

func (c clipboard) OnChange(handler ui.FocusHandler) ui.Unsubscribe {
	clipboardMu.Lock()
	if clipboardCount < 0 {
		clipboardCount = clipboardChangeCount()
	}
	clipboardMu.Unlock()
	return subscribe(&c.window.events.clipboard, handler, handler == nil)
}

// pollClipboard reports clipboard changes to every window's subscribers.
func pollClipboard() {
	clipboardMu.Lock()
	if clipboardCount < 0 {
		clipboardMu.Unlock()
		return
	}
	count := clipboardChangeCount()
	changed := count != clipboardCount
	clipboardCount = count
	clipboardMu.Unlock()
	if !changed {
		return
	}
	for _, window := range liveWindows() {
		each(&window.events.clipboard, func(handler ui.FocusHandler) {
			handler()
		})
	}
}

// ---- URLs and files ----

// OpenURL opens url with its default handler.
func (w *Window) OpenURL(ctx context.Context, url string, options ui.OpenURLOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !openURL(url, options.Background) {
		return fmt.Errorf("ui/win32: no application could open %q", url)
	}
	return nil
}

// RevealFile shows path in Finder.
func (w *Window) RevealFile(ctx context.Context, path string, options ui.RevealFileOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absolute); err != nil {
		return err
	}
	if !revealFile(absolute, options.Select) {
		return fmt.Errorf("ui/win32: Finder could not reveal %q", absolute)
	}
	return nil
}

// ---- Recent documents and file associations ----

type recentDocuments struct{}

// RecentDocuments returns the application's recent-document list. The list
// is kept in the local application data of the user, and Windows also
// records each document in its recent items.
func (w *Window) RecentDocuments() ui.RecentDocuments {
	return recentDocuments{}
}

func (recentDocuments) Add(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	addRecentDocument(absolute)
	return nil
}

func (recentDocuments) Remove(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	removeRecentDocument(absolute)
	return nil
}

func (recentDocuments) Clear() error {
	clearRecentDocuments()
	return nil
}

func (recentDocuments) List() ([]string, error) {
	return readRecentDocuments(), nil
}

type fileAssociations struct{}

// FileAssociations queries Launch Services.
func (w *Window) FileAssociations() ui.FileAssociations {
	return fileAssociations{}
}

// DefaultApplication returns the name of the default application for
// extension, or an empty string when no application is registered.
func (fileAssociations) DefaultApplication(extension string) (string, error) {
	if extension == "" || extension == "." {
		return "", errors.New("ui: file extension is empty")
	}
	return defaultApplication(extension), nil
}

// ---- Printing ----

type printJob struct {
	options ui.PrintOptions
	err     error
	pages   int
}

// Print renders pages with the application's callbacks and sends them to the
// print system.
func (w *Window) Print(ctx context.Context, options ui.PrintOptions) (ui.PrintResult, error) {
	if err := ctx.Err(); err != nil {
		return ui.PrintResult{}, err
	}
	if err := options.Validate(); err != nil {
		return ui.PrintResult{}, err
	}
	job := &printJob{options: options}
	pages := printPages(options.Title, options.ShowDialog, w.nativeWindow(), job.pageCount, job.page)
	if job.err != nil {
		return ui.PrintResult{}, job.err
	}
	if pages < 0 {
		return ui.PrintResult{}, ui.ErrCancelled
	}
	return ui.PrintResult{Pages: pages}, nil
}

func (job *printJob) pageCount() int {
	job.pages = max(0, job.options.PageCount())
	return job.pages
}

// page draws a page in a size in points and reports whether it succeeded.
func (job *printJob) page(index int, width, height float64) bool {
	if job.err != nil {
		return false
	}
	canvas := printCanvas{size: ui.Size{Width: int(width), Height: int(height)}}
	if err := job.options.RenderPage(index, canvas); err != nil {
		job.err = err
		return false
	}
	return true
}

// printCanvas draws into the current print page.
type printCanvas struct {
	size ui.Size
}

func (p printCanvas) Size() ui.Size {
	return p.size
}

func (p printCanvas) DrawImage(image stdimage.Image, bounds ui.Rect) error {
	if image == nil || image.Bounds().Empty() {
		return errors.New("ui: print image is empty")
	}
	pixels, width, height := rgbaPixels(image)
	printImage(pixels, width, height,
		float64(bounds.Position.X), float64(bounds.Position.Y),
		float64(bounds.Size.Width), float64(bounds.Size.Height))
	return nil
}

func (p printCanvas) DrawText(text string, position ui.Point, font ui.Font) error {
	if err := font.Validate(); err != nil {
		return err
	}
	if !printText(text, float64(position.X), float64(position.Y), font.Family, font.Size,
		int(font.Weight), font.Italic) {
		return errors.New("ui/win32: font family is not installed")
	}
	return nil
}

// ---- Sharing ----

// Share shows a share picker anchored to Source, or to the window content,
// and returns the chosen service name. The picker is the share sheet of
// Windows, and the service name is the name of the application that the user
// chooses. Closing the sheet without a choice returns an empty name.
func (w *Window) Share(ctx context.Context, options ui.ShareOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := options.Validate(); err != nil {
		return "", err
	}
	items := slices.Clone(options.Items)
	for i := range items {
		if items[i].File != "" {
			items[i].File = absolutePaths([]string{items[i].File})[0]
		}
	}
	var view *nativeView
	if owner, ok := options.Source.(nativeWidget); ok && owner.base().window == w {
		view = owner.base().nativeView()
	} else if content, ok := w.content.(nativeWidget); ok {
		view = content.base().nativeView()
	}
	if view == nil {
		return "", errors.New("ui/win32: sharing needs a visible source widget or window content")
	}
	picker := beginShare(view, items)
	if picker == nil {
		return "", errors.New("ui/win32: the share picker needs a visible window")
	}
	defer picker.end()
	for {
		if name, done := picker.step(); done {
			if name == "" {
				return "", ui.ErrCancelled
			}
			return name, nil
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		time.Sleep(time.Millisecond)
	}
}
