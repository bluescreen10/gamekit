//go:build linux && cgo

package gtk

import (
	"errors"
	"image"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

var clipboardChanges atomic.Int64
var watchedClipboard *gdkClipboard

func nativeClipboard() *gdkClipboard {
	display := gdkDisplayGetDefault()
	if display == nil {
		return nil
	}
	clipboard := gdkDisplayGetClipboard(display)
	if clipboard != watchedClipboard {
		watchedClipboard = clipboard
		connectNativeSignal(unsafe.Pointer(clipboard), "changed", func() { clipboardChanges.Add(1) })
	}
	return clipboard
}

func nativeClipboardChangeCount() int64 {
	// Window event processing delivers changed signals before polling. Pumping
	// the main context here can interfere with widgets under construction.
	nativeClipboard()
	return clipboardChanges.Load()
}

func writeNativeClipboard(content ui.ClipboardContent) error {
	clipboard := nativeClipboard()
	if clipboard == nil {
		return errors.New("ui/gtk: the clipboard is unavailable")
	}
	provider := nativeContentProvider(content)
	if provider != nil {
		defer gObjectUnref(unsafe.Pointer(provider))
	}
	if gdkClipboardSetContent(clipboard, provider) == 0 {
		return errors.New("ui/gtk: the clipboard rejected the content")
	}
	runNativePending()
	return nil
}

func nativeClipboardFormats() []string {
	clipboard := nativeClipboard()
	if clipboard == nil {
		return nil
	}
	formats := gdkClipboardGetFormats(clipboard)
	names := nativeMimeTypes(formats)
	if gdkContentFormatsContainGType(formats, gTypeString) != 0 && !slices.Contains(names, "text/plain") {
		names = append(names, "text/plain")
	}
	if gdkContentFormatsContainGType(formats, gdkFileListGetType()) != 0 && !slices.Contains(names, "text/uri-list") {
		names = append(names, "text/uri-list")
	}
	return names
}

func nativeMimeTypes(formats *gdkContentFormats) []string {
	var count gSize
	mimes := gdkContentFormatsGetMimeTypes(formats, &count)
	names := make([]string, int(count))
	for i, mime := range unsafe.Slice(mimes, int(count)) {
		names[i] = goString(mime)
	}
	return names
}

func standardClipboardMime(mime string) bool {
	if strings.HasPrefix(mime, "text/plain") || strings.HasPrefix(mime, "image/") {
		return true
	}
	switch mime {
	case "UTF8_STRING", "TEXT", "STRING", "COMPOUND_TEXT", "text/uri-list",
		"application/vnd.portal.filetransfer", "application/vnd.portal.files", "x-special/gnome-copied-files":
		return true
	}
	return false
}

type clipboardRead struct {
	content     ui.ClipboardContent
	pending     int
	finished    bool
	cancellable *gCancellable
}

func readNativeClipboard() ui.ClipboardContent {
	clipboard := nativeClipboard()
	if clipboard == nil {
		return ui.ClipboardContent{}
	}
	read := &clipboardRead{cancellable: gCancellableNew()}
	defer gObjectUnref(unsafe.Pointer(read.cancellable))
	formats := gdkClipboardGetFormats(clipboard)
	hasType := func(kind gType, mimes ...string) bool {
		if gdkContentFormatsContainGType(formats, kind) != 0 {
			return true
		}
		for _, mime := range mimes {
			if gdkContentFormatsContainMimeType(formats, stringPointer(mime)) != 0 {
				return true
			}
		}
		return false
	}
	if hasType(gTypeString, "text/plain;charset=utf-8", "text/plain") {
		read.pending++
		gdkClipboardReadTextAsync(clipboard, read.cancellable, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
			text := takeString(gdkClipboardReadTextFinish(clipboard, result, nil))
			if !read.finished {
				read.content.Text = text
			}
			read.pending--
		}))
	}
	if hasType(gdkFileListGetType(), "text/uri-list") {
		read.pending++
		gdkClipboardReadValueAsync(clipboard, gdkFileListGetType(), 0, read.cancellable, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
			value := gdkClipboardReadValueFinish(clipboard, result, nil)
			if value != nil && !read.finished {
				files := gdkFileListGetFiles((*gdkFileList)(gValueGetBoxed(value)))
				for item := files; item != nil; item = item.next {
					if path := gFileGetPath((*gFile)(item.data)); path != nil {
						read.content.Files = append(read.content.Files, takeString(path))
					}
				}
				gSListFree(files)
			}
			read.pending--
		}))
	}
	if hasType(gdkTextureGetType(), "image/png") {
		read.pending++
		gdkClipboardReadTextureAsync(clipboard, read.cancellable, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
			texture := gdkClipboardReadTextureFinish(clipboard, result, nil)
			if texture != nil {
				if !read.finished {
					width, height := int(gdkTextureGetWidth(texture)), int(gdkTextureGetHeight(texture))
					pixels := image.NewNRGBA(image.Rect(0, 0, width, height))
					downloader := gdkTextureDownloaderNew(texture)
					gdkTextureDownloaderSetFormat(downloader, gdkMemoryR8G8B8A8)
					gdkTextureDownloaderDownloadInto(downloader, (*cUchar)(unsafe.Pointer(unsafe.SliceData(pixels.Pix))), gSize(pixels.Stride))
					gdkTextureDownloaderFree(downloader)
					read.content.Image = pixels
				}
				gObjectUnref(unsafe.Pointer(texture))
			}
			read.pending--
		}))
	}
	for _, mime := range nativeMimeTypes(formats) {
		if standardClipboardMime(mime) {
			continue
		}
		read.pending++
		mimes, free := nativeStringVector([]string{mime})
		gdkClipboardReadAsync(clipboard, mimes, 0, read.cancellable, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
			stream := gdkClipboardReadFinish(clipboard, result, nil, nil)
			if stream == nil || read.finished {
				if stream != nil {
					gObjectUnref(unsafe.Pointer(stream))
				}
				read.pending--
				return
			}
			read.readStream(stream, mime, nil)
		}))
		free()
	}
	deadline := time.Now().Add(2 * time.Second)
	for read.pending > 0 && time.Now().Before(deadline) {
		if gMainContextIteration(nil, 0) == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	// Outstanding callbacks still own read through their handles. They must
	// finish their native requests but cannot mutate content after we return.
	read.finished = true
	gCancellableCancel(read.cancellable)
	return read.content
}

func (read *clipboardRead) readStream(stream *gInputStream, mime string, data []byte) {
	gInputStreamReadBytesAsync(stream, 65536, 0, read.cancellable, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
		chunk := gInputStreamReadBytesFinish(stream, result, nil)
		var size gSize
		if chunk != nil {
			bytes := gBytesGetData(chunk, &size)
			if !read.finished {
				data = append(data, unsafe.Slice((*byte)(bytes), int(size))...)
			}
			gBytesUnref(chunk)
		}
		if size > 0 && !read.finished {
			read.readStream(stream, mime, data)
			return
		}
		if !read.finished {
			if read.content.Custom == nil {
				read.content.Custom = make(map[string][]byte)
			}
			read.content.Custom[mime] = data
		}
		gObjectUnref(unsafe.Pointer(stream))
		read.pending--
	}))
}

func nativeContentProvider(content ui.ClipboardContent) *gdkContentProvider {
	var providers []*gdkContentProvider
	if content.Text != "" {
		var value gValue
		gValueInit(&value, gTypeString)
		gValueSetString(&value, stringPointer(content.Text))
		providers = append(providers, gdkContentProviderNewForValue(&value))
		gValueUnset(&value)
	}
	if len(content.Files) > 0 {
		files := make([]*gFile, len(content.Files))
		for i, path := range absolutePaths(content.Files) {
			files[i] = gFileNewForPath(stringPointer(path))
		}
		list := gdkFileListNewFromArray(unsafe.SliceData(files), gSize(len(files)))
		for _, file := range files {
			gObjectUnref(unsafe.Pointer(file))
		}
		var value gValue
		gValueInit(&value, gdkFileListGetType())
		gValueSetBoxed(&value, unsafe.Pointer(list))
		providers = append(providers, gdkContentProviderNewForValue(&value))
		gValueUnset(&value)
		gBoxedFree(gdkFileListGetType(), unsafe.Pointer(list))
	}
	if content.Image != nil && !content.Image.Bounds().Empty() {
		pixels, width, height := rgbaPixels(content.Image)
		bytes := gBytesNew(unsafe.Pointer(unsafe.SliceData(pixels)), gSize(len(pixels)))
		texture := gdkMemoryTextureNew(cInt(width), cInt(height), gdkMemoryR8G8B8A8, bytes, gSize(width*4))
		gBytesUnref(bytes)
		var value gValue
		gValueInit(&value, gdkTextureGetType())
		gValueSetObject(&value, unsafe.Pointer(texture))
		providers = append(providers, gdkContentProviderNewForValue(&value))
		gValueUnset(&value)
		gObjectUnref(unsafe.Pointer(texture))
	}
	for mime, data := range content.Custom {
		bytes := gBytesNew(unsafe.Pointer(unsafe.SliceData(data)), gSize(len(data)))
		providers = append(providers, gdkContentProviderNewForBytes(stringPointer(mime), bytes))
		gBytesUnref(bytes)
	}
	var provider *gdkContentProvider
	if len(providers) > 0 {
		// The union takes ownership of the individual provider references.
		provider = gdkContentProviderNewUnion(unsafe.SliceData(providers), gSize(len(providers)))
	}
	return provider
}
