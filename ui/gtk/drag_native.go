//go:build linux && cgo

package gtk

import (
	"image"
	"runtime/cgo"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

func nativeDragActions(operations int) gdkDragAction {
	var actions gdkDragAction
	if operations&1 != 0 {
		actions |= gdkActionCopy
	}
	if operations&2 != 0 {
		actions |= gdkActionMove
	}
	if operations&4 != 0 {
		actions |= gdkActionLink
	}
	return actions
}
func nativeDropOperation(actions gdkDragAction) (gdkDragAction, ui.DragOperation) {
	for _, choice := range []struct {
		native    gdkDragAction
		operation ui.DragOperation
	}{{gdkActionCopy, ui.DragCopy}, {gdkActionMove, ui.DragMove}, {gdkActionLink, ui.DragLink}} {
		if actions&choice.native != 0 {
			return choice.native, choice.operation
		}
	}
	return 0, 0
}
func gkGTKSetDragSource(pointer unsafe.Pointer, operations cInt, text, files *cChar, filesSize cSize, pixels *cUchar, width, height cInt, customTypes *cChar, customTypesSize cSize, customData *cUchar, customSizes *cSize, customCount cInt) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	if view.dragSource != nil {
		gtkWidgetRemoveController(view.outer, view.dragSource)
		view.dragSource = nil
	}
	if operations == 0 {
		return
	}
	content := ui.ClipboardContent{Text: goString(text), Files: splitNul(unsafe.Pointer(files), filesSize)}
	if pixels != nil && width > 0 && height > 0 {
		img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
		copy(img.Pix, unsafe.Slice((*byte)(unsafe.Pointer(pixels)), len(img.Pix)))
		content.Image = img
	}
	types := splitNul(unsafe.Pointer(customTypes), customTypesSize)
	sizes := unsafe.Slice(customSizes, int(customCount))
	content.Custom = make(map[string][]byte)
	offset := 0
	for i, mime := range types {
		if i >= len(sizes) {
			break
		}
		length := int(sizes[i])
		content.Custom[mime] = append([]byte(nil), unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(customData), offset)), length)...)
		offset += length
	}
	controller := gtkDragSourceNew()
	gtkDragSourceSetActions(controller, nativeDragActions(int(operations)))
	connectNativeArgument(unsafe.Pointer(controller), "prepare", gtkDragPrepareCallback(), func(x, y float64) *gdkContentProvider { return nativeContentProvider(content) })
	connectNativeArgument(unsafe.Pointer(controller), "drag-begin", gtkObjectSignalCallback(), func(_ unsafe.Pointer) {
		paintable := gtkWidgetPaintableNew(view.outer)
		gtkDragSourceSetIcon(controller, paintable, 0, 0)
		gObjectUnref(unsafe.Pointer(paintable))
	})
	view.dragSource = (*gtkEventController)(unsafe.Pointer(controller))
	gtkEventControllerSetPropagationPhase(view.dragSource, gtkPhaseCapture)
	gtkWidgetAddController(view.outer, view.dragSource)
}

type nativeDropRead struct {
	view    *nativeView
	drop    *gdkDrop
	action  gdkDragAction
	event   ui.DropEvent
	pending int
}

func (read *nativeDropRead) complete() {
	read.pending--
	if read.pending > 0 {
		return
	}
	if read.view.handle != 0 {
		if widget, ok := cgo.Handle(read.view.handle).Value().(*widgetBase); ok {
			for _, handler := range widget.contract.drops.snapshot() {
				handler(read.event)
			}
		}
	}
	gdkDropFinish(read.drop, read.action)
	gObjectUnref(unsafe.Pointer(read.drop))
	gObjectUnref(unsafe.Pointer(read.view.outer))
}
func nativeTextureImage(texture *gdkTexture) *image.NRGBA {
	width, height := int(gdkTextureGetWidth(texture)), int(gdkTextureGetHeight(texture))
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	downloader := gdkTextureDownloaderNew(texture)
	gdkTextureDownloaderSetFormat(downloader, gdkMemoryR8G8B8A8)
	gdkTextureDownloaderDownloadInto(downloader, (*cUchar)(unsafe.Pointer(unsafe.SliceData(img.Pix))), gSize(img.Stride))
	gdkTextureDownloaderFree(downloader)
	return img
}
func (read *nativeDropRead) readValue(kind gType) {
	read.pending++
	gdkDropReadValueAsync(read.drop, kind, 0, nil, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
		defer read.complete()
		value := gdkDropReadValueFinish(read.drop, result, nil)
		if value == nil {
			return
		}
		switch kind {
		case gTypeString:
			read.event.Text = goString(gValueGetString(value))
		case gdkFileListGetType():
			files := gdkFileListGetFiles((*gdkFileList)(gValueGetBoxed(value)))
			defer gSListFree(files)
			for item := files; item != nil; item = item.next {
				if path := gFileGetPath((*gFile)(item.data)); path != nil {
					read.event.Files = append(read.event.Files, takeString(path))
				}
			}
		case gdkTextureGetType():
			if texture := (*gdkTexture)(gValueGetObject(value)); texture != nil {
				read.event.Image = nativeTextureImage(texture)
			}
		}
	}))
}
func (read *nativeDropRead) readStream(stream *gInputStream, mime string, data []byte) {
	gInputStreamReadBytesAsync(stream, 65536, 0, nil, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
		chunk := gInputStreamReadBytesFinish(stream, result, nil)
		var size gSize
		if chunk != nil {
			bytes := gBytesGetData(chunk, &size)
			data = append(data, unsafe.Slice((*byte)(bytes), int(size))...)
			gBytesUnref(chunk)
		}
		if size > 0 {
			read.readStream(stream, mime, data)
			return
		}
		read.event.Custom[mime] = data
		gObjectUnref(unsafe.Pointer(stream))
		read.complete()
	}))
}
func (read *nativeDropRead) readCustom(mime string) {
	read.pending++
	mimes, free := nativeStringVector([]string{mime})
	gdkDropReadAsync(read.drop, mimes, 0, nil, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
		free()
		stream := gdkDropReadFinish(read.drop, result, nil, nil)
		if stream == nil {
			read.complete()
			return
		}
		read.readStream(stream, mime, nil)
	}))
}
func performNativeDrop(view *nativeView, drop *gdkDrop, actions gdkDragAction, types []string, x, y float64) bool {
	action, operation := nativeDropOperation(gdkDropGetActions(drop) & actions)
	if action == 0 || view.handle == 0 {
		return false
	}
	gObjectRef(unsafe.Pointer(drop))
	gObjectRef(unsafe.Pointer(view.outer))
	read := &nativeDropRead{
		view:    view,
		drop:    drop,
		action:  action,
		pending: 1,
		event:   ui.DropEvent{Operation: operation, Position: ui.Point{X: int(x), Y: int(y)}, Custom: make(map[string][]byte)},
	}
	formats := gdkDropGetFormats(drop)
	if gdkContentFormatsContainGType(formats, gTypeString) != 0 {
		read.readValue(gTypeString)
	}
	if gdkContentFormatsContainGType(formats, gdkFileListGetType()) != 0 {
		read.readValue(gdkFileListGetType())
	} else if gdkContentFormatsContainGType(formats, gdkTextureGetType()) != 0 {
		read.readValue(gdkTextureGetType())
	}
	for _, mime := range types {
		if mime != "text/plain" && mime != "text/uri-list" && gdkContentFormatsContainMimeType(formats, stringPointer(mime)) != 0 {
			read.readCustom(mime)
		}
	}
	read.complete()
	return true
}
func gkGTKSetDropTarget(pointer unsafe.Pointer, operations cInt, types *cChar, typesSize cSize) {
	view := viewState(pointer)
	if view == nil {
		return
	}
	if view.dropTarget != nil {
		gtkWidgetRemoveController(view.outer, view.dropTarget)
		view.dropTarget = nil
	}
	if operations == 0 {
		return
	}
	actions := nativeDragActions(int(operations))
	names := splitNul(unsafe.Pointer(types), typesSize)
	builder := gdkContentFormatsBuilderNew()
	if len(names) == 0 {
		gdkContentFormatsBuilderAddGType(builder, gTypeString)
		gdkContentFormatsBuilderAddGType(builder, gdkFileListGetType())
	}
	for _, mime := range names {
		if mime == "text/plain" {
			gdkContentFormatsBuilderAddGType(builder, gTypeString)
		} else if mime == "text/uri-list" {
			gdkContentFormatsBuilderAddGType(builder, gdkFileListGetType())
		}
		gdkContentFormatsBuilderAddMimeType(builder, stringPointer(mime))
	}
	controller := gtkDropTargetAsyncNew(gdkContentFormatsBuilderFreeToFormats(builder), actions)
	connectNativeArgument(unsafe.Pointer(controller), "accept", gtkDropAcceptCallback(), func(drop *gdkDrop) bool {
		action, _ := nativeDropOperation(gdkDropGetActions(drop) & actions)
		return action != 0
	})
	for _, signal := range []string{"drag-enter", "drag-motion"} {
		connectNativeArgument(unsafe.Pointer(controller), signal, gtkDropMotionCallback(), func(drop *gdkDrop, x, y float64) gdkDragAction {
			action, _ := nativeDropOperation(gdkDropGetActions(drop) & actions)
			return action
		})
	}
	connectNativeArgument(unsafe.Pointer(controller), "drop", gtkDropPerformCallback(), func(drop *gdkDrop, x, y float64) bool { return performNativeDrop(view, drop, actions, names, x, y) })
	view.dropTarget = (*gtkEventController)(unsafe.Pointer(controller))
	gtkWidgetAddController(view.outer, view.dropTarget)
}
