//go:build linux && cgo

package gtk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

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
	clipboardCount int64 = -1
)

// Clipboard returns the system clipboard.
func (w *Window) Clipboard() ui.Clipboard {
	return clipboard{window: w}
}

func (clipboard) Read() (ui.ClipboardContent, error) {
	return readNativeClipboard(), nil
}

func (clipboard) Write(content ui.ClipboardContent) error {
	return writeNativeClipboard(content)
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
	if clipboard := nativeClipboard(); clipboard != nil {
		gdkClipboardSetContent(clipboard, nil)
	}
	return nil
}

func (clipboard) Formats() ([]string, error) {
	return nativeClipboardFormats(), nil
}

func (c clipboard) OnChange(handler ui.FocusHandler) ui.Unsubscribe {
	clipboardMu.Lock()
	if clipboardCount < 0 {
		clipboardCount = nativeClipboardChangeCount()
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
	count := nativeClipboardChangeCount()
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
	if !openNativeURL(url) {
		return fmt.Errorf("ui/gtk: no application could open %q", url)
	}
	return nil
}

// RevealFile shows path in the desktop's file manager.
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
	if !revealNativeFile(absolute, options.Select) {
		return fmt.Errorf("ui/gtk: file manager could not reveal %q", absolute)
	}
	return nil
}

// ---- Recent documents and file associations ----

type recentDocuments struct{}

// RecentDocuments returns the application's recent-document list, which GTK
// keeps in the shared recently-used file of the user.
func (w *Window) RecentDocuments() ui.RecentDocuments {
	return recentDocuments{}
}

func (recentDocuments) Add(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	uri := fileURI(absolute)
	defer gFree(unsafe.Pointer(uri))
	gtkRecentManagerAddItem(gtkRecentManagerGetDefault(), uri)
	return nil
}

func (recentDocuments) Remove(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	uri := fileURI(absolute)
	defer gFree(unsafe.Pointer(uri))
	gtkRecentManagerRemoveItem(gtkRecentManagerGetDefault(), uri, nil)
	return nil
}

func (recentDocuments) Clear() error {
	clearRecentDocuments()
	return nil
}

func (recentDocuments) List() ([]string, error) {
	return recentDocumentPaths(), nil
}

type fileAssociations struct{}

// FileAssociations queries the desktop's default application associations.
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
	context *cairoContext
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
	pages := runNativePrint(job, w.nativeGTKWindow())
	if job.err != nil {
		return ui.PrintResult{}, job.err
	}
	if pages < 0 {
		return ui.PrintResult{}, ui.ErrCancelled
	}
	return ui.PrintResult{Pages: pages}, nil
}

// ---- Sharing ----

// Share shows a share picker anchored to Source, or to the window content,
// and returns the chosen service name. Linux has no share sheet, so the
// picker offers the clipboard ("Copy"), e-mail ("Mail"), and the applications
// that open a link or a file ("Open With").
func (w *Window) Share(ctx context.Context, options ui.ShareOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := options.Validate(); err != nil {
		return "", err
	}
	return w.shareNative(ctx, options)
}
