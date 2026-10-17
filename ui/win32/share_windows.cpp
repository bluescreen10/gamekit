//go:build windows && cgo

#include "winrt_windows.h"

#include <shlwapi.h>

/* ---- Sharing ----

   Sharing shows the share sheet of Windows for the window of the view,
   through the DataTransferManager of the Windows Runtime. The share sheet
   reports the application that the user chooses; when the user closes the
   sheet without choosing, the window becomes active again. */

struct GKDataTransferManagerInterop : IUnknown {
    virtual HRESULT STDMETHODCALLTYPE GetForWindow(HWND window, REFIID riid, void **manager) = 0;
    virtual HRESULT STDMETHODCALLTYPE ShowShareUIForWindow(HWND window) = 0;
};

struct GKDataTransferManager : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE add_DataRequested(IUnknown *handler, GKRegistration *registration) = 0;
    virtual HRESULT STDMETHODCALLTYPE remove_DataRequested(GKRegistration registration) = 0;
    virtual HRESULT STDMETHODCALLTYPE add_TargetApplicationChosen(IUnknown *handler, GKRegistration *registration) = 0;
    virtual HRESULT STDMETHODCALLTYPE remove_TargetApplicationChosen(GKRegistration registration) = 0;
};

struct GKDataPackagePropertySet : IInspectable {
    GK_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE put_Title(HSTRING title) = 0;
};

struct GKDataPackage : IInspectable {
    GK_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE get_Properties(GKDataPackagePropertySet **properties) = 0;
    GK_SLOT(1);
    GK_SLOT(2);
    GK_SLOT(3);
    GK_SLOT(4);
    GK_SLOT(5);
    GK_SLOT(6);
    GK_SLOT(7);
    GK_SLOT(8);
    virtual HRESULT STDMETHODCALLTYPE SetText(HSTRING text) = 0;
    GK_SLOT(9);
    GK_SLOT(10);
    GK_SLOT(11);
    GK_SLOT(12);
    GK_SLOT(13);
    GK_SLOT(14);
    virtual HRESULT STDMETHODCALLTYPE SetStorageItems(IInspectable *items, boolean readOnly) = 0;
};

struct GKDataPackage2 : IInspectable {
    GK_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE SetWebLink(IInspectable *uri) = 0;
};

struct GKDataRequestDeferral : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE Complete() = 0;
};

struct GKDataRequest : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE get_Data(GKDataPackage **data) = 0;
};

struct GKDataRequestedEventArgs : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE get_Request(GKDataRequest **request) = 0;
};

struct GKTargetApplicationChosenEventArgs : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE get_ApplicationName(HSTRING *name) = 0;
};

struct GKUriFactory : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE CreateUri(HSTRING text, IInspectable **uri) = 0;
};

struct GKStorageFileStatics : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE GetFileFromPathAsync(HSTRING path, GKAsyncOperation **operation) = 0;
};

struct GKStorageFolderStatics : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE GetFolderFromPathAsync(HSTRING path, GKAsyncOperation **operation) = 0;
};

static const IID gkInteropID = {0x3a3dcd6c, 0x3eab, 0x43dc, {0xbc, 0xde, 0x45, 0x67, 0x1c, 0xe8, 0x00, 0xc8}};
static const IID gkDataTransferManagerID = {0xa5caee9b, 0x8708, 0x49d1, {0x8d, 0x36, 0x67, 0xd2, 0x5a, 0x8d, 0xa0, 0x0c}};
static const IID gkDataPackage2ID = {0x041c1fe9, 0x2409, 0x45e1, {0xa5, 0x38, 0x4c, 0x53, 0xee, 0xee, 0x04, 0xa7}};
static const IID gkDataRequestedArgsID = {0xcb8ba807, 0x6ac5, 0x43c9, {0x8a, 0xc5, 0x9b, 0xa2, 0x32, 0x16, 0x31, 0x82}};
static const IID gkTargetChosenArgsID = {0xca6fb8ac, 0x2987, 0x4ee3, {0x9c, 0x54, 0xd8, 0xaf, 0xbc, 0xb8, 0x6c, 0x1d}};
static const IID gkUriFactoryID = {0x44a9796f, 0x723e, 0x4fdf, {0xa2, 0x18, 0x03, 0x3e, 0x75, 0xb0, 0xc0, 0x84}};
static const IID gkStorageFileStaticsID = {0x5984c710, 0xdaf2, 0x43c8, {0x8b, 0xb4, 0xa4, 0xd3, 0xea, 0xcf, 0xd0, 0x3f}};
static const IID gkStorageFolderStaticsID = {0x08f327ff, 0x85d5, 0x48b9, {0xae, 0xe9, 0x28, 0x51, 0x1e, 0x33, 0x9f, 0x9f}};
static const IID gkStorageItemID = {0x4207a996, 0xca2f, 0x42f7, {0xbd, 0xe8, 0x8b, 0x10, 0x45, 0x7a, 0x7f, 0x30}};
static const IID gkInspectableID = {0xaf86e2e0, 0xb12d, 0x4c6a, {0x9c, 0x5a, 0xd7, 0xaa, 0x65, 0x10, 0x1e, 0x90}};
static const IID gkAsyncInfoID = {0x00000036, 0x0000, 0x0000, {0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}};
/* IIterable<IStorageItem> and IIterator<IStorageItem>. */
static const IID gkItemsID = {0xbb8b8418, 0x65d1, 0x544b, {0xb0, 0x83, 0x6d, 0x17, 0x2f, 0x56, 0x8c, 0x73}};
static const IID gkItemIteratorID = {0x05b487c2, 0x3830, 0x5d3c, {0x98, 0xda, 0x25, 0xfa, 0x11, 0x54, 0x2d, 0xbd}};
/* TypedEventHandler<DataTransferManager, DataRequestedEventArgs> and
   TypedEventHandler<DataTransferManager, TargetApplicationChosenEventArgs>. */
static const IID gkDataRequestedID = {0xec6f9cc8, 0x46d0, 0x5e0e, {0xb4, 0xd2, 0x7d, 0x77, 0x73, 0xae, 0x37, 0xa0}};
static const IID gkTargetChosenID = {0xc4ac1ba2, 0x7851, 0x5a44, {0xbc, 0x8d, 0x3d, 0x7c, 0x71, 0x3f, 0x1f, 0x41}};

IInspectable *gkAwait(GKAsyncOperation *operation, double timeout) {
    if (!operation) {
        return nullptr;
    }
    GKAsyncInfo *info = nullptr;
    IInspectable *result = nullptr;
    int status = 0;
    if (SUCCEEDED(operation->QueryInterface(gkAsyncInfoID, reinterpret_cast<void **>(&info)))) {
        /* Status 0 is Started and 1 is Completed. */
        gkPump(
            [info, &status]() {
                info->get_Status(&status);
                return status != 0;
            },
            timeout);
        info->Release();
    }
    if (status == 1) {
        operation->GetResults(&result);
    }
    operation->Release();
    return result;
}

static IInspectable *gkFactory(const wchar_t *name, IID const &id) {
    IInspectable *factory = nullptr;
    GKHString runtimeClass(name);
    RoGetActivationFactory(runtimeClass, id, reinterpret_cast<void **>(&factory));
    return factory;
}

/* GKStorageItems is an IIterable<IStorageItem> of the files that the user
   shares, and its own iterator, as a new one for each First. */
struct GKStorageItems : IInspectable {
    LONG references = 1;
    std::vector<IInspectable *> items;
    size_t position = 0;
    bool iterator = false;

    ~GKStorageItems() {
        for (IInspectable *item : items) {
            item->Release();
        }
    }
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
        if (IsEqualIID(riid, IID_IUnknown) || IsEqualIID(riid, gkInspectableID) || IsEqualIID(riid, IID_IAgileObject) ||
            IsEqualIID(riid, iterator ? gkItemIteratorID : gkItemsID)) {
            *object = this;
            AddRef();
            return S_OK;
        }
        *object = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override {
        return InterlockedIncrement(&references);
    }
    ULONG STDMETHODCALLTYPE Release() override {
        LONG count = InterlockedDecrement(&references);
        if (!count) {
            delete this;
        }
        return count;
    }
    HRESULT STDMETHODCALLTYPE GetIids(ULONG *count, IID **ids) override {
        *ids = static_cast<IID *>(CoTaskMemAlloc(sizeof(IID)));
        if (!*ids) {
            return E_OUTOFMEMORY;
        }
        **ids = iterator ? gkItemIteratorID : gkItemsID;
        *count = 1;
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE GetRuntimeClassName(HSTRING *name) override {
        *name = nullptr;
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE GetTrustLevel(TrustLevel *level) override {
        *level = BaseTrust;
        return S_OK;
    }
    /* IIterable::First, or IIterator::get_Current. */
    virtual HRESULT STDMETHODCALLTYPE First(void **result) {
        if (iterator) {
            if (position >= items.size()) {
                *result = nullptr;
                return E_BOUNDS;
            }
            return items[position]->QueryInterface(gkStorageItemID, result);
        }
        auto next = new GKStorageItems();
        next->iterator = true;
        for (IInspectable *item : items) {
            item->AddRef();
            next->items.push_back(item);
        }
        *result = next;
        return S_OK;
    }
    /* IIterator::get_HasCurrent. */
    virtual HRESULT STDMETHODCALLTYPE HasCurrent(boolean *result) {
        *result = position < items.size();
        return S_OK;
    }
    /* IIterator::MoveNext. */
    virtual HRESULT STDMETHODCALLTYPE MoveNext(boolean *result) {
        if (position < items.size()) {
            position++;
        }
        *result = position < items.size();
        return S_OK;
    }
    /* IIterator::GetMany. */
    virtual HRESULT STDMETHODCALLTYPE GetMany(UINT32 capacity, void **values, UINT32 *count) {
        *count = 0;
        while (*count < capacity && position < items.size()) {
            items[position]->QueryInterface(gkStorageItemID, &values[*count]);
            (*count)++;
            position++;
        }
        return S_OK;
    }
};

/* gkStorageItem returns the file or the folder at a path. */
static IInspectable *gkStorageItem(std::wstring const &path) {
    GKHString text(path);
    DWORD attributes = GetFileAttributesW(path.c_str());
    if (attributes == INVALID_FILE_ATTRIBUTES) {
        return nullptr;
    }
    GKAsyncOperation *operation = nullptr;
    if (attributes & FILE_ATTRIBUTE_DIRECTORY) {
        auto statics = reinterpret_cast<GKStorageFolderStatics *>(
            gkFactory(L"Windows.Storage.StorageFolder", gkStorageFolderStaticsID));
        if (statics) {
            statics->GetFolderFromPathAsync(text, &operation);
            statics->Release();
        }
    } else {
        auto statics =
            reinterpret_cast<GKStorageFileStatics *>(gkFactory(L"Windows.Storage.StorageFile", gkStorageFileStaticsID));
        if (statics) {
            statics->GetFileFromPathAsync(text, &operation);
            statics->Release();
        }
    }
    return gkAwait(operation, 5);
}

struct GKShare {
    HWND hwnd = nullptr;
    GKDataTransferManager *manager = nullptr;
    GKRegistration requested = {}, chosen = {};
    std::vector<std::wstring> texts, urls;
    GKStorageItems *files = nullptr;
    std::string service;
    volatile LONG done = 0;
    int left = 0;
    ULONGLONG started = 0;

    ~GKShare() {
        if (manager) {
            manager->remove_DataRequested(requested);
            manager->remove_TargetApplicationChosen(chosen);
            manager->Release();
        }
        gkRelease(files);
    }
};

/* gkShareData fills the data package of the share sheet. */
static void gkShareData(GKShare *share, IInspectable *value) {
    GKDataRequestedEventArgs *args = nullptr;
    GKDataRequest *request = nullptr;
    GKDataPackage *data = nullptr;
    if (!value || FAILED(value->QueryInterface(gkDataRequestedArgsID, reinterpret_cast<void **>(&args))) ||
        FAILED(args->get_Request(&request)) || FAILED(request->get_Data(&data))) {
        gkRelease(request);
        gkRelease(args);
        return;
    }
    std::wstring title = !share->texts.empty()  ? share->texts[0]
                         : !share->urls.empty() ? share->urls[0]
                                                : L"Share";
    GKDataPackagePropertySet *properties = nullptr;
    if (SUCCEEDED(data->get_Properties(&properties))) {
        GKHString text(title.substr(0, 120));
        properties->put_Title(text);
        properties->Release();
    }
    std::wstring text;
    for (auto const &value : share->texts) {
        text += (text.empty() ? L"" : L"\n") + value;
    }
    if (!text.empty()) {
        GKHString string(text);
        data->SetText(string);
    }
    if (!share->urls.empty()) {
        auto factory = reinterpret_cast<GKUriFactory *>(gkFactory(L"Windows.Foundation.Uri", gkUriFactoryID));
        GKDataPackage2 *data2 = nullptr;
        IInspectable *uri = nullptr;
        GKHString string(share->urls[0]);
        if (factory && SUCCEEDED(factory->CreateUri(string, &uri)) &&
            SUCCEEDED(data->QueryInterface(gkDataPackage2ID, reinterpret_cast<void **>(&data2)))) {
            data2->SetWebLink(uri);
        }
        gkRelease(data2);
        gkRelease(uri);
        gkRelease(factory);
    }
    if (share->files && !share->files->items.empty()) {
        data->SetStorageItems(share->files, true);
    }
    data->Release();
    request->Release();
    args->Release();
}

extern "C" void *gkWin32ShareBegin(const char *texts, size_t textsSize, const char *urls, size_t urlsSize,
                                   const char *files, size_t filesSize, void *viewPointer) {
    GKView *view = gkView(viewPointer);
    GKWin *win = view ? gkWinOf(view->outer) : nullptr;
    if (!win || !IsWindowVisible(win->hwnd)) {
        return nullptr;
    }
    auto share = new GKShare();
    share->hwnd = win->hwnd;
    for (auto const &value : gkSplit(texts, textsSize)) {
        share->texts.push_back(gkWide(value.c_str()));
    }
    for (auto const &value : gkSplit(urls, urlsSize)) {
        share->urls.push_back(gkWide(value.c_str()));
    }
    share->files = new GKStorageItems();
    for (auto const &value : gkSplit(files, filesSize)) {
        if (IInspectable *item = gkStorageItem(gkWide(value.c_str()))) {
            share->files->items.push_back(item);
        }
    }
    auto interop = reinterpret_cast<GKDataTransferManagerInterop *>(
        gkFactory(L"Windows.ApplicationModel.DataTransfer.DataTransferManager", gkInteropID));
    if (!interop || FAILED(interop->GetForWindow(share->hwnd, gkDataTransferManagerID,
                                                 reinterpret_cast<void **>(&share->manager))) ||
        !share->manager) {
        gkRelease(interop);
        delete share;
        return nullptr;
    }
    /* The events arrive on the UI thread, while gkWin32ShareStep pumps. */
    auto requested = new GKEventHandler(gkDataRequestedID,
                                        [share](IInspectable *, IInspectable *args) { gkShareData(share, args); });
    share->manager->add_DataRequested(requested, &share->requested);
    requested->Release();
    auto chosen = new GKEventHandler(gkTargetChosenID, [share](IInspectable *, IInspectable *value) {
        GKTargetApplicationChosenEventArgs *args = nullptr;
        if (value && SUCCEEDED(value->QueryInterface(gkTargetChosenArgsID, reinterpret_cast<void **>(&args)))) {
            GKHString name;
            args->get_ApplicationName(&name.value);
            share->service = gkUTF8(name.text());
            args->Release();
        }
        InterlockedExchange(&share->done, 1);
    });
    share->manager->add_TargetApplicationChosen(chosen, &share->chosen);
    chosen->Release();
    HRESULT result = interop->ShowShareUIForWindow(share->hwnd);
    interop->Release();
    if (FAILED(result)) {
        delete share;
        return nullptr;
    }
    share->started = GetTickCount64();
    return share;
}

extern "C" int gkWin32ShareStep(void *pointer, char **service) {
    GKShare *share = static_cast<GKShare *>(pointer);
    gkPumpOnce();
    if (!share->done) {
        HWND foreground = GetForegroundWindow();
        if (foreground != share->hwnd) {
            share->left = 1;
        } else if (share->left && GetTickCount64() - share->started > 500) {
            share->done = 1;
        }
    }
    if (!share->done) {
        return 0;
    }
    *service = gkStrdup(share->service.c_str());
    return 1;
}

extern "C" void gkWin32ShareEnd(void *pointer) {
    delete static_cast<GKShare *>(pointer);
}
