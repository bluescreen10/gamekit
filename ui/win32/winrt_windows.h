/* winrt_windows.h declares helpers for the ABI of the Windows Runtime, which
   the backend reaches without C++/WinRT. The headers of MinGW for the
   Windows Runtime do not compile as C++, so the files that use it declare
   the interfaces they call, with a slot for each method that they do not
   call, in the order of the vtable. */

#ifndef GK_WINRT_WINDOWS_H
#define GK_WINRT_WINDOWS_H

#include "win32_internal.h"

#include <inspectable.h>
#include <roapi.h>
#include <winstring.h>

#include <functional>

#define GK_SLOT(n) virtual HRESULT STDMETHODCALLTYPE slot##n() = 0

/* GKHString owns a Windows Runtime string. */
struct GKHString {
    HSTRING value = nullptr;
    GKHString() = default;
    explicit GKHString(std::wstring const &text) {
        WindowsCreateString(text.c_str(), (UINT32)text.size(), &value);
    }
    GKHString(GKHString const &) = delete;
    GKHString &operator=(GKHString const &) = delete;
    ~GKHString() {
        if (value) {
            WindowsDeleteString(value);
        }
    }
    operator HSTRING() const {
        return value;
    }
    std::wstring text() const {
        UINT32 length = 0;
        const wchar_t *buffer = WindowsGetStringRawBuffer(value, &length);
        return std::wstring(buffer ? buffer : L"", length);
    }
};

template <typename T> static inline void gkRelease(T *&object) {
    if (object) {
        object->Release();
        object = nullptr;
    }
}

/* GKRegistration is EventRegistrationToken. */
struct GKRegistration {
    INT64 value;
};

/* GKEventHandler is a TypedEventHandler of the Windows Runtime with the
   interface ID id. Its events can arrive on any thread. */
struct GKEventHandler : IUnknown {
    LONG references = 1;
    IID id;
    std::function<void(IInspectable *, IInspectable *)> work;

    GKEventHandler(IID const &id, std::function<void(IInspectable *, IInspectable *)> work)
        : id(id), work(std::move(work)) {}
    virtual ~GKEventHandler() = default;
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
        if (IsEqualIID(riid, IID_IUnknown) || IsEqualIID(riid, IID_IAgileObject) || IsEqualIID(riid, id)) {
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
    virtual HRESULT STDMETHODCALLTYPE Invoke(IInspectable *sender, IInspectable *args) {
        work(sender, args);
        return S_OK;
    }
};

/* GKAsyncInfo is IAsyncInfo. */
struct GKAsyncInfo : IInspectable {
    GK_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE get_Status(int *status) = 0;
};

/* GKAsyncOperation is IAsyncOperation<T> for an object T. */
struct GKAsyncOperation : IInspectable {
    GK_SLOT(0);
    GK_SLOT(1);
    virtual HRESULT STDMETHODCALLTYPE GetResults(IInspectable **result) = 0;
};

/* gkAwait waits for an asynchronous operation, processing messages, and
   returns its result. It releases the operation. */
IInspectable *gkAwait(GKAsyncOperation *operation, double timeout);

#endif
