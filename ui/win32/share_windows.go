package win32

import (
	"strings"
	"sync/atomic"
	"unicode/utf16"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Sharing ----
//
// Sharing shows the share sheet of Windows for the window of the view, through
// the DataTransferManager of the Windows Runtime. The share sheet reports the
// application that the user chooses; when the user closes the sheet without
// choosing, the window becomes active again.

// storageItems is the files that the user shares, as an IIterable of
// IStorageItem, and an IIterator over them.
type storageItems struct {
	items    []comObject
	position int
}

func (s *storageItems) release() {
	for _, item := range s.items {
		win32UnknownRelease(item)
	}
	s.items = nil
}

var storageIterableTable, storageIteratorTable []uintptr

// newStorageIterable returns an IIterable<IStorageItem> of items, which it
// holds a reference to.
func newStorageIterable(items []comObject) *comServer {
	if storageIterableTable == nil {
		storageIterableTable = methodTable(append(inspectableMethods(),
			callback("IIterable.First", func(this, result uintptr) uintptr {
				source, ok := serverObject[*storageItems](this)
				if !ok {
					return uintptr(eNoInterface)
				}
				iterator := newStorageIterator(source.items)
				*(*comObject)(memoryAt(result)) = iterator.com()
				return sOK
			}))...)
	}
	state := &storageItems{items: retained(items)}
	server := newCOMServer(storageIterableTable, []*guid{iidStorageItems, iidInspectable, iidAgileObject}, state)
	server.release = state.release
	return server
}

func newStorageIterator(items []comObject) *comServer {
	if storageIteratorTable == nil {
		current := func(this, result uintptr) uintptr {
			state, ok := serverObject[*storageItems](this)
			out := (*comObject)(memoryAt(result))
			if !ok || state.position >= len(state.items) {
				*out = 0
				return uintptr(eBounds)
			}
			*out = query(state.items[state.position], iidStorageItem)
			return sOK
		}
		hasCurrent := func(this, result uintptr) uintptr {
			state, _ := serverObject[*storageItems](this)
			*(*bool)(memoryAt(result)) = state != nil && state.position < len(state.items)
			return sOK
		}
		moveNext := func(this, result uintptr) uintptr {
			state, _ := serverObject[*storageItems](this)
			if state != nil && state.position < len(state.items) {
				state.position++
			}
			*(*bool)(memoryAt(result)) = state != nil && state.position < len(state.items)
			return sOK
		}
		getMany := func(this, capacity, values, count uintptr) uintptr {
			state, _ := serverObject[*storageItems](this)
			var n uint32
			for state != nil && n < uint32(capacity) && state.position < len(state.items) {
				slot := (*comObject)(unsafe.Add(memoryAt(values), uintptr(n)*unsafe.Sizeof(comObject(0))))
				*slot = query(state.items[state.position], iidStorageItem)
				n++
				state.position++
			}
			*(*uint32)(memoryAt(count)) = n
			return sOK
		}
		storageIteratorTable = methodTable(append(inspectableMethods(),
			callback("IIterator.get_Current", current),
			callback("IIterator.get_HasCurrent", hasCurrent),
			callback("IIterator.MoveNext", moveNext),
			callback("IIterator.GetMany", getMany))...)
	}
	state := &storageItems{items: retained(items)}
	server := newCOMServer(storageIteratorTable, []*guid{iidStorageItemIterator, iidInspectable, iidAgileObject},
		state)
	server.release = state.release
	return server
}

// retained returns a copy of objects with a reference to each.
func retained(objects []comObject) []comObject {
	copied := make([]comObject, len(objects))
	for i, object := range objects {
		win32UnknownAddRef(object)
		copied[i] = object
	}
	return copied
}

// storageItem returns the file or the folder at a path, or 0.
func storageItem(path string) comObject {
	attributes := win32GetFileAttributes(path)
	if attributes == invalidFileAttributes {
		return 0
	}
	text := win32WindowsCreateString(path)
	defer win32WindowsDeleteString(text)
	var operation comObject
	if attributes&fileAttributeDirectory != 0 {
		if statics := activationFactory("Windows.Storage.StorageFolder", iidStorageFolderStatics); statics != 0 {
			win32StorageFolderStaticsGetFolderFromPathAsync(statics, text, &operation)
			win32UnknownRelease(statics)
		}
	} else if statics := activationFactory("Windows.Storage.StorageFile", iidStorageFileStatics); statics != 0 {
		win32StorageFileStaticsGetFileFromPathAsync(statics, text, &operation)
		win32UnknownRelease(statics)
	}
	return awaitOperation(operation, 5)
}

// shareSession is a share sheet that is open.
type shareSession struct {
	hwnd              hwnd
	manager           comObject
	requested, chosen eventRegistrationToken
	texts, urls       []string
	files             []comObject
	service           string
	done              atomic.Bool
	left              bool
	started           uint64
}

func (s *shareSession) close() {
	if s.manager != 0 {
		win32DataTransferManagerRemoveDataRequested(s.manager, s.requested)
		win32DataTransferManagerRemoveTargetApplicationChosen(s.manager, s.chosen)
		win32UnknownRelease(s.manager)
		s.manager = 0
	}
	for _, file := range s.files {
		win32UnknownRelease(file)
	}
	s.files = nil
}

// fill fills the data package of the share sheet.
func (s *shareSession) fill(value comObject) {
	if value == 0 {
		return
	}
	args := query(value, iidDataRequestedEventArgs)
	if args == 0 {
		return
	}
	defer win32UnknownRelease(args)
	var request, data comObject
	if win32DataRequestedEventArgsGetRequest(args, &request).failed() || request == 0 {
		return
	}
	defer win32UnknownRelease(request)
	if win32DataRequestGetData(request, &data).failed() || data == 0 {
		return
	}
	defer win32UnknownRelease(data)
	title := "Share"
	switch {
	case len(s.texts) > 0:
		title = s.texts[0]
	case len(s.urls) > 0:
		title = s.urls[0]
	}
	var properties comObject
	if win32DataPackageGetProperties(data, &properties).succeeded() && properties != 0 {
		units := utf16.Encode([]rune(title))
		text := win32WindowsCreateString(string(utf16.Decode(units[:min(len(units), 120)])))
		win32DataPackagePropertySetPutTitle(properties, text)
		win32WindowsDeleteString(text)
		win32UnknownRelease(properties)
	}
	if text := strings.Join(s.texts, "\n"); text != "" {
		value := win32WindowsCreateString(text)
		win32DataPackageSetText(data, value)
		win32WindowsDeleteString(value)
	}
	if len(s.urls) > 0 {
		factory := activationFactory("Windows.Foundation.Uri", iidUriRuntimeClassFactory)
		text := win32WindowsCreateString(s.urls[0])
		var uri comObject
		if factory != 0 && win32UriRuntimeClassFactoryCreateUri(factory, text, &uri).succeeded() && uri != 0 {
			if data2 := query(data, iidDataPackage2); data2 != 0 {
				win32DataPackage2SetWebLink(data2, uri)
				win32UnknownRelease(data2)
			}
			win32UnknownRelease(uri)
		}
		win32WindowsDeleteString(text)
		win32UnknownRelease(factory)
	}
	if len(s.files) > 0 {
		items := newStorageIterable(s.files)
		win32DataPackageSetStorageItems(data, items.com(), true)
		win32UnknownRelease(items.com())
	}
}

// beginShare shows the share sheet for the window of a view, or returns nil
// when it cannot. The paths of the files must be absolute.
func beginShare(view *nativeView, items []ui.ShareItem) *shareSession {
	var win *nativeWindow
	if view.live() {
		win = windowOf(view.outer)
	}
	if win == nil || !win32IsWindowVisible(win.hwnd) {
		return nil
	}
	share := &shareSession{hwnd: win.hwnd}
	for _, item := range items {
		if item.Text != "" {
			share.texts = append(share.texts, item.Text)
		}
		if item.URL != "" {
			share.urls = append(share.urls, item.URL)
		}
		if item.File == "" {
			continue
		}
		if file := storageItem(item.File); file != 0 {
			share.files = append(share.files, file)
		}
	}
	interop := activationFactory("Windows.ApplicationModel.DataTransfer.DataTransferManager",
		iidDataTransferManagerInterop)
	if interop == 0 {
		share.close()
		return nil
	}
	defer win32UnknownRelease(interop)
	if win32DataTransferManagerInteropGetForWindow(interop, share.hwnd, iidDataTransferManager,
		&share.manager).failed() || share.manager == 0 {
		share.close()
		return nil
	}
	// The events arrive on the UI thread, while step processes messages.
	requested := newEventHandler(iidDataRequestedHandler, func(_, args comObject) {
		share.fill(args)
	})
	win32DataTransferManagerAddDataRequested(share.manager, requested.com(), &share.requested)
	win32UnknownRelease(requested.com())
	chosen := newEventHandler(iidTargetApplicationChosenHandler, func(_, value comObject) {
		if args := query(value, iidTargetApplicationChosenEventArgs); args != 0 {
			var name hstring
			if win32TargetApplicationChosenEventArgsGetApplicationName(args, &name).succeeded() {
				share.service = takeString(name)
			}
			win32UnknownRelease(args)
		}
		share.done.Store(true)
	})
	win32DataTransferManagerAddTargetApplicationChosen(share.manager, chosen.com(), &share.chosen)
	win32UnknownRelease(chosen.com())
	if win32DataTransferManagerInteropShowShareUIForWindow(interop, share.hwnd).failed() {
		share.close()
		return nil
	}
	share.started = win32GetTickCount64()
	return share
}

// step processes messages and returns the service that the user
// chose, and true, when the share sheet is closed.
func (share *shareSession) step() (string, bool) {
	pumpOnce()
	if !share.done.Load() {
		if win32GetForegroundWindow() != share.hwnd {
			share.left = true
		} else if share.left && win32GetTickCount64()-share.started > 500 {
			share.done.Store(true)
		}
	}
	if !share.done.Load() {
		return "", false
	}
	return share.service, true
}

func (share *shareSession) end() {
	share.close()
}
