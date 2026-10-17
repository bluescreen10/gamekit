//go:build windows && cgo

#include "winrt_windows.h"

#include <shlwapi.h>
#include <shobjidl.h>
#include <stdio.h>

#include <map>

/* Notifications are Windows toast notifications, which the backend reaches
   through the ABI of the Windows Runtime. An application without a package
   identity needs an application user model ID that Windows knows, so the
   backend registers one for the executable under
   HKEY_CURRENT_USER\Software\Classes\AppUserModelId and gives it to the
   process. A notification with a delivery time waits in the process, like
   on the other backends, so its activation reaches the application. */


extern "C" void gkWin32GoNotificationResult(uintptr_t token, int status, char *ids, size_t size, char *message);

static uintptr_t gkNotificationHandle;
static const wchar_t gkNotificationGroup[] = L"gamekit";

struct GKToast;

struct GKToastNotifier : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE Show(GKToast *toast) = 0;
    GK_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE get_Setting(int *setting) = 0;
};

struct GKToastHistory : IInspectable {
    GK_SLOT(0);
    GK_SLOT(1);
    virtual HRESULT STDMETHODCALLTYPE RemoveGroupedTagWithId(HSTRING tag, HSTRING group, HSTRING app) = 0;
};

/* GKVectorView is IVectorView<ToastNotification>. */
struct GKVectorView : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE GetAt(UINT32 index, GKToast **item) = 0;
    virtual HRESULT STDMETHODCALLTYPE get_Size(UINT32 *size) = 0;
};

struct GKToastHistory2 : IInspectable {
    GK_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE GetHistoryWithId(HSTRING app, GKVectorView **toasts) = 0;
};

struct GKToastManager : IInspectable {
    GK_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE CreateToastNotifierWithId(HSTRING app, GKToastNotifier **notifier) = 0;
};

struct GKToastManager2 : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE get_History(GKToastHistory **history) = 0;
};

struct GKToastFactory : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE CreateToastNotification(IInspectable *document, GKToast **toast) = 0;
};

struct GKToast : IInspectable {
    GK_SLOT(0);
    GK_SLOT(1);
    GK_SLOT(2);
    GK_SLOT(3);
    GK_SLOT(4);
    virtual HRESULT STDMETHODCALLTYPE add_Activated(IUnknown *handler, GKRegistration *registration) = 0;
};

struct GKToast2 : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE put_Tag(HSTRING tag) = 0;
    virtual HRESULT STDMETHODCALLTYPE get_Tag(HSTRING *tag) = 0;
    virtual HRESULT STDMETHODCALLTYPE put_Group(HSTRING group) = 0;
};

struct GKToastActivatedArgs : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE get_Arguments(HSTRING *arguments) = 0;
};

struct GKToastActivatedArgs2 : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE get_UserInput(IInspectable **input) = 0;
};

/* GKStringMap is IMap<String, Object>, the user input of an activation. */
struct GKStringMap : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE Lookup(HSTRING key, IInspectable **value) = 0;
    virtual HRESULT STDMETHODCALLTYPE get_Size(UINT32 *size) = 0;
    virtual HRESULT STDMETHODCALLTYPE HasKey(HSTRING key, boolean *found) = 0;
};

struct GKXmlDocumentIO : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE LoadXml(HSTRING xml) = 0;
};

struct GKPropertyValue : IInspectable {
    GK_SLOT(0);
    GK_SLOT(1);
    GK_SLOT(2);
    GK_SLOT(3);
    GK_SLOT(4);
    GK_SLOT(5);
    GK_SLOT(6);
    GK_SLOT(7);
    GK_SLOT(8);
    GK_SLOT(9);
    GK_SLOT(10);
    GK_SLOT(11);
    GK_SLOT(12);
    virtual HRESULT STDMETHODCALLTYPE GetString(HSTRING *value) = 0;
};

static const IID gkToastManagerID = {0x50ac103f, 0xd235, 0x4598, {0xbb, 0xef, 0x98, 0xfe, 0x4d, 0x1a, 0x3a, 0xd4}};
static const IID gkToastManager2ID = {0x7ab93c52, 0x0e48, 0x4750, {0xba, 0x9d, 0x1a, 0x41, 0x13, 0x98, 0x18, 0x47}};
static const IID gkToastFactoryID = {0x04124b20, 0x82c6, 0x4229, {0xb1, 0x09, 0xfd, 0x9e, 0xd4, 0x66, 0x2b, 0x53}};
static const IID gkToast2ID = {0x9dfb9fd1, 0x143a, 0x490e, {0x90, 0xbf, 0xb9, 0xfb, 0xa7, 0x13, 0x2d, 0xe7}};
static const IID gkToastHistory2ID = {0x3bc3d253, 0x2f31, 0x4092, {0x91, 0x29, 0x8a, 0xd5, 0xab, 0xf0, 0x67, 0xda}};
static const IID gkToastActivatedArgsID = {0xe3bf92f3, 0xc197, 0x436f, {0x82, 0x65, 0x06, 0x25, 0x82, 0x4f, 0x8d, 0xac}};
static const IID gkToastActivatedArgs2ID = {0xab7da512, 0xcc61, 0x568e, {0x81, 0xbe, 0x30, 0x4a, 0xc3, 0x10, 0x38, 0xfa}};
static const IID gkXmlDocumentIOID = {0x6cd0e74e, 0xee65, 0x4489, {0x9e, 0xbf, 0xca, 0x43, 0xe8, 0x7b, 0xa6, 0x37}};
static const IID gkPropertyValueID = {0x4bd682dd, 0x7554, 0x40e9, {0x9a, 0x9b, 0x82, 0x65, 0x4e, 0xde, 0x7e, 0x62}};
static const IID gkStringMapID = {0x1b0d3570, 0x0877, 0x5ec2, {0x8a, 0x2c, 0x3b, 0x95, 0x39, 0x50, 0x6a, 0xca}};
/* ITypedEventHandler<ToastNotification, Object>. */
static const IID gkActivatedHandlerID = {0xab54de2d, 0x97d9, 0x5528, {0xb6, 0xad, 0x10, 0x5a, 0xfe, 0x15, 0x65, 0x30}};

static std::wstring gkExecutableName() {
    wchar_t path[MAX_PATH];
    GetModuleFileNameW(nullptr, path, MAX_PATH);
    std::wstring name = PathFindFileNameW(path);
    size_t dot = name.rfind(L'.');
    return dot == std::wstring::npos ? name : name.substr(0, dot);
}

/* gkAppID returns the application user model ID of the process, and
   registers one when the process has none. */
std::wstring gkAppID() {
    static std::wstring id;
    if (!id.empty()) {
        return id;
    }
    PWSTR current = nullptr;
    if (SUCCEEDED(GetCurrentProcessExplicitAppUserModelID(&current)) && current) {
        id = current;
        CoTaskMemFree(current);
        return id;
    }
    std::wstring name = gkExecutableName();
    id = L"GameKit.";
    for (wchar_t c : name) {
        id += iswalnum(c) || c == L'.' ? c : L'_';
    }
    std::wstring key = L"Software\\Classes\\AppUserModelId\\" + id;
    HKEY handle = nullptr;
    if (RegCreateKeyExW(HKEY_CURRENT_USER, key.c_str(), 0, nullptr, 0, KEY_SET_VALUE, nullptr, &handle, nullptr) ==
        ERROR_SUCCESS) {
        RegSetValueExW(handle, L"DisplayName", 0, REG_SZ, reinterpret_cast<const BYTE *>(name.c_str()),
                       (DWORD)((name.size() + 1) * sizeof(wchar_t)));
        RegCloseKey(handle);
    }
    SetCurrentProcessExplicitAppUserModelID(id.c_str());
    return id;
}

static GKToastManager *gkToastManager() {
    static GKToastManager *manager;
    if (!manager) {
        GKHString name(L"Windows.UI.Notifications.ToastNotificationManager");
        RoGetActivationFactory(name, gkToastManagerID, reinterpret_cast<void **>(&manager));
    }
    return manager;
}

static GKToastNotifier *gkNotifier() {
    static GKToastNotifier *notifier;
    if (!notifier && gkToastManager()) {
        GKHString id(gkAppID());
        gkToastManager()->CreateToastNotifierWithId(id, &notifier);
    }
    return notifier;
}

static GKToastHistory *gkHistory() {
    static GKToastHistory *history;
    GKToastManager2 *manager = nullptr;
    if (!history && gkToastManager() &&
        SUCCEEDED(gkToastManager()->QueryInterface(gkToastManager2ID,
                                                   reinterpret_cast<void **>(&manager)))) {
        manager->get_History(&history);
        manager->Release();
    }
    return history;
}

extern "C" int gkWin32NotificationsAvailable(void) {
    return gkNotifier() != nullptr;
}

extern "C" void gkWin32NotificationsAttach(uintptr_t handle) {
    gkNotificationHandle = handle;
}

/* gkToastsEnabled reports whether the user allows notifications for the
   whole system. */
static bool gkToastsEnabled() {
    DWORD enabled = 1, size = sizeof(enabled);
    RegGetValueW(HKEY_CURRENT_USER, L"Software\\Microsoft\\Windows\\CurrentVersion\\PushNotifications",
                 L"ToastEnabled", RRF_RT_REG_DWORD, nullptr, &enabled, &size);
    return enabled != 0;
}

static int gkAuthorization() {
    /* NotificationDenied is 1 and NotificationAuthorized is 2. Windows does
       not ask the user; it allows the notifications of an application until
       the user turns them off. */
    GKToastNotifier *notifier = gkNotifier();
    if (!notifier) {
        return 1;
    }
    int setting = 0;
    HRESULT result = notifier->get_Setting(&setting);
    if (result == HRESULT_FROM_WIN32(ERROR_NOT_FOUND)) {
        /* Windows lists an application in its notification settings after
           its first notification, so before that the setting is not found
           and the default applies. */
        return gkToastsEnabled() ? 2 : 1;
    }
    return SUCCEEDED(result) && setting == 0 /* Enabled */ ? 2 : 1;
}

extern "C" void gkWin32NotificationAuthorization(uintptr_t token) {
    gkWin32GoNotificationResult(token, gkAuthorization(), nullptr, 0, nullptr);
}

extern "C" void gkWin32NotificationRequestAuthorization(uintptr_t token) {
    gkWin32GoNotificationResult(token, gkAuthorization(), nullptr, 0, nullptr);
}

static std::wstring gkXML(const char *text) {
    std::wstring value = gkWide(text), out;
    for (wchar_t c : value) {
        switch (c) {
        case L'&':
            out += L"&amp;";
            break;
        case L'<':
            out += L"&lt;";
            break;
        case L'>':
            out += L"&gt;";
            break;
        case L'"':
            out += L"&quot;";
            break;
        default:
            out += c;
        }
    }
    return out;
}

struct GKToastRequest {
    std::string identifier;
    std::wstring xml;
};

static std::map<std::string, UINT_PTR> &gkScheduled() {
    static std::map<std::string, UINT_PTR> timers;
    return timers;
}

/* gkGroups records the thread of each notification, which is its group in
   the notification center. */
static std::map<std::string, std::wstring> &gkGroups() {
    static std::map<std::string, std::wstring> groups;
    return groups;
}

static std::wstring gkGroupOf(std::string const &identifier) {
    auto found = gkGroups().find(identifier);
    return found != gkGroups().end() ? found->second : std::wstring(gkNotificationGroup);
}

static std::wstring gkTag(std::string const &identifier) {
    /* A tag has at most 64 characters. */
    return gkWide(identifier.c_str()).substr(0, 64);
}

static std::string gkArgument(std::wstring const &arguments, const wchar_t *name) {
    std::wstring key = std::wstring(name) + L"=";
    size_t start = 0;
    while (start <= arguments.size()) {
        size_t end = arguments.find(L'&', start);
        std::wstring part = arguments.substr(start, end == std::wstring::npos ? std::wstring::npos : end - start);
        if (part.rfind(key, 0) == 0) {
            std::wstring value = part.substr(key.size());
            std::wstring decoded;
            for (size_t i = 0; i < value.size(); i++) {
                if (value[i] == L'%' && i + 2 < value.size()) {
                    decoded += (wchar_t)wcstol(value.substr(i + 1, 2).c_str(), nullptr, 16);
                    i += 2;
                } else {
                    decoded += value[i];
                }
            }
            return gkUTF8(decoded.c_str());
        }
        if (end == std::wstring::npos) {
            break;
        }
        start = end + 1;
    }
    return "";
}

static std::wstring gkEncode(std::string const &value) {
    std::wstring out;
    for (unsigned char c : value) {
        if (isalnum(c) || c == '-' || c == '_' || c == '.') {
            out += (wchar_t)c;
        } else {
            wchar_t escaped[4];
            swprintf(escaped, 4, L"%%%02X", c);
            out += escaped;
        }
    }
    return out;
}

/* gkToastActivated receives the activation of a toast, on a thread of the
   notification platform, and reports it on the UI thread. */
static void gkToastActivated(std::string const &identifier, IInspectable *value) {
    std::wstring arguments;
    std::string reply;
    GKToastActivatedArgs *args = nullptr;
    if (value && SUCCEEDED(value->QueryInterface(gkToastActivatedArgsID, reinterpret_cast<void **>(&args)))) {
        GKHString text;
        if (SUCCEEDED(args->get_Arguments(&text.value))) {
            arguments = text.text();
        }
        args->Release();
    }
    GKToastActivatedArgs2 *args2 = nullptr;
    if (value &&
        SUCCEEDED(value->QueryInterface(gkToastActivatedArgs2ID, reinterpret_cast<void **>(&args2)))) {
        IInspectable *input = nullptr;
        GKStringMap *map = nullptr;
        if (SUCCEEDED(args2->get_UserInput(&input)) && input &&
            SUCCEEDED(input->QueryInterface(gkStringMapID, reinterpret_cast<void **>(&map)))) {
            GKHString key(L"reply");
            boolean found = false;
            IInspectable *entry = nullptr;
            GKPropertyValue *property = nullptr;
            if (SUCCEEDED(map->HasKey(key, &found)) && found && SUCCEEDED(map->Lookup(key, &entry)) && entry &&
                SUCCEEDED(entry->QueryInterface(gkPropertyValueID,
                                                reinterpret_cast<void **>(&property)))) {
                GKHString text;
                if (SUCCEEDED(property->GetString(&text.value))) {
                    reply = gkUTF8(text.text());
                }
            }
            gkRelease(property);
            gkRelease(entry);
        }
        gkRelease(map);
        gkRelease(input);
        args2->Release();
    }
    std::string action = gkArgument(arguments, L"action");
    std::string id = identifier;
    gkLater([id, action, reply]() {
        if (!gkNotificationHandle) {
            return;
        }
        GKWin32Event event = {};
        event.kind = GK_EVENT_NOTIFICATION;
        event.text = id.c_str();
        event.text2 = action.c_str();
        event.object = (void *)reply.c_str();
        gkSendEvent(gkNotificationHandle, &event);
    });
}

static HRESULT gkToastShow(GKToastRequest const &request) {
    GKToastNotifier *notifier = gkNotifier();
    if (!notifier) {
        return E_NOTIMPL;
    }
    IInspectable *instance = nullptr;
    GKXmlDocumentIO *io = nullptr;
    GKToastFactory *factory = nullptr;
    GKToast *toast = nullptr;
    GKToast2 *toast2 = nullptr;
    GKHString documentClass(L"Windows.Data.Xml.Dom.XmlDocument");
    GKHString toastClass(L"Windows.UI.Notifications.ToastNotification");
    GKHString xml(request.xml);
    HRESULT result = RoActivateInstance(documentClass, &instance);
    if (SUCCEEDED(result)) {
        result = instance->QueryInterface(gkXmlDocumentIOID, reinterpret_cast<void **>(&io));
    }
    if (SUCCEEDED(result)) {
        result = io->LoadXml(xml);
    }
    if (SUCCEEDED(result)) {
        result = RoGetActivationFactory(toastClass, gkToastFactoryID, reinterpret_cast<void **>(&factory));
    }
    if (SUCCEEDED(result)) {
        result = factory->CreateToastNotification(instance, &toast);
    }
    if (SUCCEEDED(result) &&
        SUCCEEDED(toast->QueryInterface(gkToast2ID, reinterpret_cast<void **>(&toast2)))) {
        GKHString tag(gkTag(request.identifier)), group(gkGroupOf(request.identifier));
        toast2->put_Tag(tag);
        toast2->put_Group(group);
    }
    if (SUCCEEDED(result)) {
        std::string identifier = request.identifier;
        auto handler = new GKEventHandler(gkActivatedHandlerID, [identifier](IInspectable *, IInspectable *value) {
            gkToastActivated(identifier, value);
        });
        GKRegistration registration = {};
        toast->add_Activated(handler, &registration);
        handler->Release();
        result = notifier->Show(toast);
    }
    gkRelease(toast2);
    gkRelease(toast);
    gkRelease(factory);
    gkRelease(io);
    gkRelease(instance);
    return result;
}

extern "C" void gkWin32NotificationSend(uintptr_t token, const char *identifier, const char *title,
                                        const char *subtitle, const char *body, const char *category,
                                        const char *thread, int sound, const char *attachment, double delay,
                                        const char *actionIDs, size_t actionIDsSize, const char *actionTitles,
                                        size_t actionTitlesSize, const int *actionFlags, int actionCount) {
    /* Windows has no notification categories; the actions of a
       notification come with it. The thread groups notifications in the
       notification center. */
    (void)category;
    std::string id = identifier ? identifier : "";
    std::wstring group = thread && *thread ? gkWide(thread).substr(0, 64) : std::wstring(gkNotificationGroup);
    gkGroups()[id] = group;
    std::wstring xml = L"<toast launch=\"id=" + gkEncode(id) + L"\"><visual><binding template=\"ToastGeneric\">";
    xml += L"<text>" + gkXML(title) + L"</text>";
    if (subtitle && *subtitle) {
        xml += L"<text>" + gkXML(subtitle) + L"</text>";
    }
    if (body && *body) {
        xml += L"<text>" + gkXML(body) + L"</text>";
    }
    if (attachment && *attachment) {
        std::wstring path = gkWide(attachment);
        for (auto &c : path) {
            if (c == L'\\') {
                c = L'/';
            }
        }
        xml += L"<image placement=\"appLogoOverride\" src=\"file:///" + gkXML(gkUTF8(path.c_str()).c_str()) + L"\"/>";
    }
    xml += L"</binding></visual>";
    auto ids = gkSplit(actionIDs, actionIDsSize);
    auto titles = gkSplit(actionTitles, actionTitlesSize);
    if (actionCount > 0) {
        xml += L"<actions>";
        bool input = false;
        for (int i = 0; i < actionCount; i++) {
            if ((actionFlags[i] & 2) && !input) {
                xml += L"<input id=\"reply\" type=\"text\" placeHolderContent=\"" +
                       gkXML(i < (int)titles.size() ? titles[i].c_str() : "") + L"\"/>";
                input = true;
            }
        }
        for (int i = 0; i < actionCount && i < 5; i++) {
            std::string actionID = i < (int)ids.size() ? ids[i] : "";
            xml += L"<action content=\"" + gkXML(i < (int)titles.size() ? titles[i].c_str() : "") +
                   L"\" arguments=\"id=" + gkEncode(id) + L"&amp;action=" + gkEncode(actionID) + L"\"";
            if (actionFlags[i] & 2) {
                xml += L" hint-inputId=\"reply\"";
            }
            if (actionFlags[i] & 1) {
                xml += L" hint-buttonStyle=\"Critical\"";
            }
            xml += L"/>";
        }
        xml += L"</actions>";
    }
    xml += sound ? L"<audio src=\"ms-winsoundevent:Notification.Default\"/>" : L"<audio silent=\"true\"/>";
    xml += L"</toast>";
    GKToastRequest request{id, xml};
    if (delay > 0) {
        auto &timers = gkScheduled();
        auto existing = timers.find(id);
        if (existing != timers.end()) {
            gkTimerCancel(existing->second);
            timers.erase(existing);
        }
        timers[id] = gkTimer((UINT)(delay * 1000), [request]() {
            gkScheduled().erase(request.identifier);
            gkToastShow(request);
            return false;
        });
    } else {
        HRESULT result = gkToastShow(request);
        if (FAILED(result)) {
            char message[96];
            snprintf(message, sizeof(message), "the notification could not be sent (0x%08lx)", (unsigned long)result);
            gkWin32GoNotificationResult(token, -1, nullptr, 0, gkStrdup(message));
            return;
        }
    }
    gkWin32GoNotificationResult(token, 0, nullptr, 0, nullptr);
}

extern "C" void gkWin32NotificationPending(uintptr_t token) {
    std::vector<std::string> ids;
    for (auto const &entry : gkScheduled()) {
        ids.push_back(entry.first);
    }
    size_t size = 0;
    char *buffer = ids.empty() ? nullptr : gkJoin(ids, &size);
    gkWin32GoNotificationResult(token, 0, buffer, size, nullptr);
}

extern "C" void gkWin32NotificationDelivered(uintptr_t token) {
    std::vector<std::string> ids;
    GKToastHistory2 *history = nullptr;
    GKVectorView *toasts = nullptr;
    GKHString id(gkAppID());
    if (gkHistory() &&
        SUCCEEDED(gkHistory()->QueryInterface(gkToastHistory2ID, reinterpret_cast<void **>(&history))) &&
        SUCCEEDED(history->GetHistoryWithId(
            id, &toasts)) &&
        toasts) {
        /* Every notification in the history of the application ID is one
           that the application sent. */
        UINT32 count = 0;
        toasts->get_Size(&count);
        for (UINT32 i = 0; i < count; i++) {
            GKToast *toast = nullptr;
            GKToast2 *toast2 = nullptr;
            if (SUCCEEDED(toasts->GetAt(i, &toast)) &&
                SUCCEEDED(toast->QueryInterface(gkToast2ID, reinterpret_cast<void **>(&toast2)))) {
                GKHString tag;
                toast2->get_Tag(&tag.value);
                ids.push_back(gkUTF8(tag.text()));
            }
            gkRelease(toast2);
            gkRelease(toast);
        }
    }
    gkRelease(toasts);
    gkRelease(history);
    size_t size = 0;
    char *buffer = ids.empty() ? nullptr : gkJoin(ids, &size);
    gkWin32GoNotificationResult(token, 0, buffer, size, nullptr);
}

extern "C" void gkWin32NotificationWithdraw(const char *ids, size_t size) {
    GKHString app(gkAppID());
    for (auto const &identifier : gkSplit(ids, size)) {
        auto found = gkScheduled().find(identifier);
        if (found != gkScheduled().end()) {
            gkTimerCancel(found->second);
            gkScheduled().erase(found);
        }
        if (gkHistory()) {
            GKHString tag(gkTag(identifier)), group(gkGroupOf(identifier));
            gkHistory()->RemoveGroupedTagWithId(tag, group, app);
        }
    }
}

/* ---- Badge ----

   The badge is an overlay icon on the taskbar buttons of the windows, the
   badge that Windows shows for desktop applications. */

static std::string gkBadgeText;

static HICON gkBadgeIcon(std::string const &text) {
    const int size = 32;
    BITMAPV5HEADER header = {};
    header.bV5Size = sizeof(header);
    header.bV5Width = size;
    header.bV5Height = -size;
    header.bV5Planes = 1;
    header.bV5BitCount = 32;
    header.bV5Compression = BI_BITFIELDS;
    header.bV5RedMask = 0x00ff0000;
    header.bV5GreenMask = 0x0000ff00;
    header.bV5BlueMask = 0x000000ff;
    header.bV5AlphaMask = 0xff000000;
    void *bits = nullptr;
    HDC screen = GetDC(nullptr);
    HBITMAP color = CreateDIBSection(screen, reinterpret_cast<BITMAPINFO *>(&header), DIB_RGB_COLORS, &bits, nullptr, 0);
    ReleaseDC(nullptr, screen);
    if (!color) {
        return nullptr;
    }
    uint32_t *pixels = static_cast<uint32_t *>(bits);
    /* A red disc with white text. */
    for (int y = 0; y < size; y++) {
        for (int x = 0; x < size; x++) {
            double dx = x + 0.5 - size / 2.0, dy = y + 0.5 - size / 2.0;
            double distance = sqrt(dx * dx + dy * dy);
            double coverage = distance < size / 2.0 - 1 ? 1 : distance < size / 2.0 ? size / 2.0 - distance : 0;
            uint32_t alpha = (uint32_t)(coverage * 255);
            pixels[y * size + x] = (alpha << 24) | ((uint32_t)(0xc4 * alpha / 255) << 16) |
                                   ((uint32_t)(0x2b * alpha / 255) << 8) | (uint32_t)(0x1c * alpha / 255);
        }
    }
    HDC dc = CreateCompatibleDC(nullptr);
    HGDIOBJ previous = SelectObject(dc, color);
    HFONT font = CreateFontW(text.size() > 2 ? -13 : -19, 0, 0, 0, FW_BOLD, FALSE, FALSE, FALSE, DEFAULT_CHARSET,
                             OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, ANTIALIASED_QUALITY, DEFAULT_PITCH, L"Segoe UI");
    HGDIOBJ previousFont = SelectObject(dc, font);
    SetBkMode(dc, TRANSPARENT);
    SetTextColor(dc, RGB(255, 255, 255));
    std::wstring wide = gkWide(text.c_str());
    RECT area = {0, 0, size, size};
    GdiFlush();
    std::vector<uint32_t> before(pixels, pixels + size * size);
    DrawTextW(dc, wide.c_str(), (int)wide.size(), &area, DT_CENTER | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX);
    GdiFlush();
    /* GDI text leaves the alpha channel at zero; pixels it changed are made
       opaque. */
    for (int i = 0; i < size * size; i++) {
        if (pixels[i] != before[i]) {
            pixels[i] |= 0xff000000;
        }
    }
    SelectObject(dc, previousFont);
    SelectObject(dc, previous);
    DeleteObject(font);
    DeleteDC(dc);
    HBITMAP mask = CreateBitmap(size, size, 1, 1, nullptr);
    ICONINFO info = {};
    info.fIcon = TRUE;
    info.hbmColor = color;
    info.hbmMask = mask;
    HICON icon = CreateIconIndirect(&info);
    DeleteObject(color);
    DeleteObject(mask);
    return icon;
}

static ITaskbarList3 *gkTaskbar() {
    static ITaskbarList3 *taskbar;
    if (!taskbar) {
        if (FAILED(CoCreateInstance(CLSID_TaskbarList, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&taskbar))) ||
            FAILED(taskbar->HrInit())) {
            taskbar = nullptr;
        }
    }
    return taskbar;
}

extern "C" void gkWin32SetBadge(const char *badge) {
    gkBadgeText = badge ? badge : "";
    ITaskbarList3 *taskbar = gkTaskbar();
    if (!taskbar) {
        return;
    }
    HICON icon = gkBadgeText.empty() ? nullptr : gkBadgeIcon(gkBadgeText);
    std::wstring description = gkWide(gkBadgeText.c_str());
    for (GKWin *win : gkWindows()) {
        if (win->hwnd && !win->utility) {
            taskbar->SetOverlayIcon(win->hwnd, icon, description.c_str());
        }
    }
    if (icon) {
        DestroyIcon(icon);
    }
}

extern "C" char *gkWin32Badge(void) {
    return gkStrdup(gkBadgeText.c_str());
}
