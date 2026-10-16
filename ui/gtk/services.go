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
	"fmt"
	stdimage "image"
	"os"
	"path/filepath"
	"runtime/cgo"
	"sync"
	"time"
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
	clipboardCount C.long = -1
)

// Clipboard returns the system clipboard.
func (w *Window) Clipboard() ui.Clipboard {
	return clipboard{window: w}
}

func (clipboard) Read() (ui.ClipboardContent, error) {
	var text, files, types *C.char
	var hasText, width, height, count C.int
	var filesSize, typesSize C.size_t
	var pixels, data *C.uchar
	var sizes *C.size_t
	C.gkGTKClipboardRead(&text, &hasText, &files, &filesSize, &pixels, &width, &height,
		&types, &typesSize, &data, &sizes, &count)
	defer C.free(unsafe.Pointer(files))
	defer C.free(unsafe.Pointer(pixels))
	defer C.free(unsafe.Pointer(types))
	defer C.free(unsafe.Pointer(data))
	defer C.free(unsafe.Pointer(sizes))
	content := ui.ClipboardContent{
		Text:  goStringFree(text),
		Files: splitNul(unsafe.Pointer(files), filesSize),
	}
	if pixels != nil && width > 0 && height > 0 {
		image := stdimage.NewNRGBA(stdimage.Rect(0, 0, int(width), int(height)))
		copy(image.Pix, unsafe.Slice((*byte)(unsafe.Pointer(pixels)), int(width)*int(height)*4))
		content.Image = image
	}
	if count > 0 {
		mimes := splitNul(unsafe.Pointer(types), typesSize)
		lengths := unsafe.Slice(sizes, int(count))
		content.Custom = make(map[string][]byte, len(mimes))
		offset := 0
		for i, mime := range mimes {
			if i >= len(lengths) {
				break
			}
			length := int(lengths[i])
			content.Custom[mime] = C.GoBytes(unsafe.Add(unsafe.Pointer(data), offset), C.int(length))
			offset += length
		}
	}
	return content, nil
}

func (clipboard) Write(content ui.ClipboardContent) error {
	text := cString(content.Text)
	defer freeCString(text)
	files := joinNul(absolutePaths(content.Files))
	var pixels []byte
	var width, height int
	if content.Image != nil && !content.Image.Bounds().Empty() {
		pixels, width, height = rgbaPixels(content.Image)
	}
	types := make([]string, 0, len(content.Custom))
	var data []byte
	var sizes []C.size_t
	for mime, value := range content.Custom {
		types = append(types, mime)
		data = append(data, value...)
		sizes = append(sizes, C.size_t(len(value)))
	}
	typeBuffer := joinNul(types)
	var sizesPointer *C.size_t
	if len(sizes) > 0 {
		sizesPointer = &sizes[0]
	}
	if C.gkGTKClipboardWrite(text, boolInt(content.Text != ""), bytesPointer(files), C.size_t(len(files)),
		(*C.uchar)(unsafe.Pointer(bytesPointer(pixels))), C.int(width), C.int(height),
		bytesPointer(typeBuffer), C.size_t(len(typeBuffer)),
		(*C.uchar)(unsafe.Pointer(bytesPointer(data))), sizesPointer, C.int(len(sizes))) == 0 {
		return errors.New("ui/gtk: the clipboard rejected the content")
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
	C.gkGTKClipboardClear()
	return nil
}

func (clipboard) Formats() ([]string, error) {
	var size C.size_t
	buffer := C.gkGTKClipboardFormats(&size)
	defer C.free(unsafe.Pointer(buffer))
	return splitNul(unsafe.Pointer(buffer), size), nil
}

func (c clipboard) OnChange(handler ui.FocusHandler) ui.Unsubscribe {
	clipboardMu.Lock()
	if clipboardCount < 0 {
		clipboardCount = C.gkGTKClipboardChangeCount()
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
	count := C.gkGTKClipboardChangeCount()
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
	value := cString(url)
	defer freeCString(value)
	if C.gkGTKOpenURL(value, boolInt(options.Background)) == 0 {
		return fmt.Errorf("ui/gtk: no application could open %q", url)
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
	value := cString(absolute)
	defer freeCString(value)
	if C.gkGTKRevealFile(value, boolInt(options.Select)) == 0 {
		return fmt.Errorf("ui/gtk: Finder could not reveal %q", absolute)
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
	value := cString(absolute)
	defer freeCString(value)
	C.gkGTKRecentAdd(value)
	return nil
}

func (recentDocuments) Remove(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	value := cString(absolute)
	defer freeCString(value)
	C.gkGTKRecentRemove(value)
	return nil
}

func (recentDocuments) Clear() error {
	C.gkGTKRecentClear()
	return nil
}

func (recentDocuments) List() ([]string, error) {
	var size C.size_t
	buffer := C.gkGTKRecentList(&size)
	defer C.free(unsafe.Pointer(buffer))
	return splitNul(unsafe.Pointer(buffer), size), nil
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
	value := cString(extension)
	defer freeCString(value)
	return goStringFree(C.gkGTKDefaultApplication(value)), nil
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
	handle := cgo.NewHandle(job)
	defer handle.Delete()
	title := cString(options.Title)
	defer freeCString(title)
	pages := int(C.gkGTKPrint(C.uintptr_t(handle), title, boolInt(options.ShowDialog), w.nativeWindowPointer()))
	if job.err != nil {
		return ui.PrintResult{}, job.err
	}
	if pages < 0 {
		return ui.PrintResult{}, ui.ErrCancelled
	}
	return ui.PrintResult{Pages: pages}, nil
}

//export gkGTKGoPrintPageCount
func gkGTKGoPrintPageCount(handle C.uintptr_t) C.int {
	job := cgo.Handle(handle).Value().(*printJob)
	job.pages = max(0, job.options.PageCount())
	return C.int(job.pages)
}

//export gkGTKGoPrintPage
func gkGTKGoPrintPage(handle C.uintptr_t, page C.int, width, height C.double) C.int {
	job := cgo.Handle(handle).Value().(*printJob)
	if job.err != nil {
		return 0
	}
	canvas := printCanvas{size: ui.Size{Width: int(width), Height: int(height)}}
	if err := job.options.RenderPage(int(page), canvas); err != nil {
		job.err = err
		return 0
	}
	return 1
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
	C.gkGTKPrintImage((*C.uchar)(unsafe.Pointer(&pixels[0])), C.int(width), C.int(height),
		C.double(bounds.Position.X), C.double(bounds.Position.Y),
		C.double(bounds.Size.Width), C.double(bounds.Size.Height))
	return nil
}

func (p printCanvas) DrawText(text string, position ui.Point, font ui.Font) error {
	if err := font.Validate(); err != nil {
		return err
	}
	value := cString(text)
	family := cString(font.Family)
	defer freeCString(value)
	defer freeCString(family)
	if C.gkGTKPrintText(value, C.double(position.X), C.double(position.Y), family, C.double(font.Size),
		C.int(font.Weight), boolInt(font.Italic)) == 0 {
		return errors.New("ui/gtk: font family is not installed")
	}
	return nil
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
	var texts, urls, files []string
	for _, item := range options.Items {
		if item.Text != "" {
			texts = append(texts, item.Text)
		}
		if item.URL != "" {
			urls = append(urls, item.URL)
		}
		if item.File != "" {
			files = append(files, absolutePaths([]string{item.File})...)
		}
	}
	var view unsafe.Pointer
	if owner, ok := options.Source.(nativeWidget); ok && owner.base().window == w {
		view = owner.base().nativePointer()
	} else if content, ok := w.content.(nativeWidget); ok {
		view = content.base().nativePointer()
	}
	if view == nil {
		return "", errors.New("ui/gtk: sharing needs a visible source widget or window content")
	}
	textBuffer, urlBuffer, fileBuffer := joinNul(texts), joinNul(urls), joinNul(files)
	picker := C.gkGTKShareBegin(bytesPointer(textBuffer), C.size_t(len(textBuffer)),
		bytesPointer(urlBuffer), C.size_t(len(urlBuffer)), bytesPointer(fileBuffer), C.size_t(len(fileBuffer)), view)
	if picker == nil {
		return "", errors.New("ui/gtk: the share picker needs a visible window")
	}
	defer C.gkGTKShareEnd(picker)
	for {
		var service *C.char
		if C.gkGTKShareStep(picker, &service) != 0 {
			name := goStringFree(service)
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
