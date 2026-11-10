//go:build linux && cgo

package gtk

import (
	"context"
	"strings"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

type filePanelMode uint8

const (
	openFilePanel filePanelMode = iota
	openFilesPanel
	selectFolderPanel
	saveFilePanel
)

func newFileDialog(title, directory, filename string, filters []ui.FileFilter) *gtkFileDialog {
	dialog := gtkFileDialogNew()
	gtkFileDialogSetModal(dialog, 1)
	if title != "" {
		gtkFileDialogSetTitle(dialog, stringPointer(title))
	}
	if directory != "" {
		folder := gFileNewForPath(stringPointer(directory))
		gtkFileDialogSetInitialFolder(dialog, folder)
		gObjectUnref(unsafe.Pointer(folder))
	}
	if filename != "" {
		gtkFileDialogSetInitialName(dialog, stringPointer(filename))
	}
	if extensions := fileExtensions(filters); extensions != "" {
		filter := gtkFileFilterNew()
		var patterns []string
		for _, extension := range strings.Split(extensions, ",") {
			extension = strings.TrimPrefix(strings.TrimSpace(extension), ".")
			if extension != "" {
				gtkFileFilterAddSuffix(filter, stringPointer(extension))
				patterns = append(patterns, "*."+extension)
			}
		}
		gtkFileFilterSetName(filter, stringPointer(strings.Join(patterns, ", ")))
		store := gListStoreNew(gtkFileFilterGetType())
		gListStoreAppend(store, unsafe.Pointer(filter))
		gtkFileDialogSetFilters(dialog, (*gListModel)(unsafe.Pointer(store)))
		gtkFileDialogSetDefaultFilter(dialog, filter)
		gObjectUnref(unsafe.Pointer(store))
		gObjectUnref(unsafe.Pointer(filter))
	}
	return dialog
}

func (w *Window) runFilePanel(ctx context.Context, dialog *gtkFileDialog, mode filePanelMode) ([]string, error) {
	defer gObjectUnref(unsafe.Pointer(dialog))
	cancellable := gCancellableNew()
	defer gObjectUnref(unsafe.Pointer(cancellable))
	parent := w.nativeGTKWindow()
	if parent != nil && gtkWidgetGetVisible((*gtkWidget)(unsafe.Pointer(parent))) == 0 {
		parent = nil
	}
	var paths []string
	var done bool
	callback := asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
		var file *gFile
		var files *gListModel
		switch mode {
		case openFilePanel:
			file = gtkFileDialogOpenFinish(dialog, result, nil)
		case openFilesPanel:
			files = gtkFileDialogOpenMultipleFinish(dialog, result, nil)
		case selectFolderPanel:
			file = gtkFileDialogSelectFolderFinish(dialog, result, nil)
		case saveFilePanel:
			file = gtkFileDialogSaveFinish(dialog, result, nil)
		}
		appendFile := func(file *gFile) {
			if path := gFileGetPath(file); path != nil {
				paths = append(paths, takeString(path))
			}
			gObjectUnref(unsafe.Pointer(file))
		}
		if files != nil {
			for i := cUint(0); i < gListModelGetNItems(files); i++ {
				appendFile((*gFile)(gListModelGetItem(files, i)))
			}
			gObjectUnref(unsafe.Pointer(files))
		}
		if file != nil {
			appendFile(file)
		}
		done = true
	})
	switch mode {
	case openFilePanel:
		gtkFileDialogOpen(dialog, parent, cancellable, gtkAsyncReadyCallback(), callback)
	case openFilesPanel:
		gtkFileDialogOpenMultiple(dialog, parent, cancellable, gtkAsyncReadyCallback(), callback)
	case selectFolderPanel:
		gtkFileDialogSelectFolder(dialog, parent, cancellable, gtkAsyncReadyCallback(), callback)
	case saveFilePanel:
		gtkFileDialogSave(dialog, parent, cancellable, gtkAsyncReadyCallback(), callback)
	}
	for !done {
		if err := ctx.Err(); err != nil {
			// GTK's async task retains the dialog and callback until its finish
			// function consumes the cancelled result in a later event iteration.
			gCancellableCancel(cancellable)
			return nil, err
		}
		if gMainContextIteration(nil, 0) == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	if len(paths) == 0 {
		return nil, ui.ErrCancelled
	}
	return paths, nil
}
