//go:build windows && cgo

#include "win32_internal.h"

#include <dwmapi.h>
#include <shellapi.h>
#include <shlwapi.h>

#include <algorithm>
#include <atomic>

bool gkForwardNotification(UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result);

/* gkProgramName returns the name of the program without its extension. */
static std::wstring gkProgramName() {
    wchar_t path[MAX_PATH];
    GetModuleFileNameW(nullptr, path, MAX_PATH);
    std::wstring file = PathFindFileNameW(path);
    size_t dot = file.rfind(L'.');
    return file.substr(0, dot);
}

/* ---- Task dialogs ----

   An alert is the task dialog of Windows. A task dialog runs its own
   message loop until it closes, so it runs on a thread of its own while the
   Go layer polls the alert and the window keeps its messages flowing. */

static const int gkFirstButton = 100;

struct GKAlert : GKObject {
    int severity = 0;
    std::wstring title, message, details;
    std::vector<std::wstring> buttons;
    std::vector<int> destructive;
    int cancel = -1;
    int prompt = 0;
    int secure = 0;
    std::wstring text, placeholder;
    std::wstring website;
    GKIcon *icon = nullptr;
    HWND owner = nullptr;
    HANDLE thread = nullptr;
    std::atomic<HWND> dialog{nullptr};
    std::atomic<int> response{-1};
    std::atomic<int> done{0};
    /* The window of a prompt, which is a dialog of the bridge. */
    HWND window = nullptr;
    HWND edit = nullptr;
    std::vector<HWND> controls;
    std::wstring answer;
    ~GKAlert() override;
};

static int gkTaskButton(GKAlert *alert, int command) {
    if (command >= gkFirstButton && command < gkFirstButton + (int)alert->buttons.size()) {
        return command - gkFirstButton;
    }
    return alert->cancel >= 0 ? alert->cancel : 0;
}

static HRESULT CALLBACK gkTaskCallback(HWND hwnd, UINT notification, WPARAM wparam, LPARAM lparam, LONG_PTR data) {
    (void)wparam;
    GKAlert *alert = reinterpret_cast<GKAlert *>(data);
    switch (notification) {
    case TDN_CREATED:
        alert->dialog = hwnd;
        if (alert->prompt == 0 && alert->owner && IsWindow(alert->owner)) {
            SetForegroundWindow(hwnd);
        }
        break;
    case TDN_HYPERLINK_CLICKED:
        ShellExecuteW(nullptr, L"open", reinterpret_cast<LPCWSTR>(lparam), nullptr, nullptr, SW_SHOWNORMAL);
        break;
    case TDN_DESTROYED:
        alert->dialog = nullptr;
        break;
    default:
        break;
    }
    return S_OK;
}

static DWORD WINAPI gkTaskThread(LPVOID parameter) {
    GKAlert *alert = static_cast<GKAlert *>(parameter);
    CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
    GKActivation activation;
    TASKDIALOGCONFIG config = {};
    config.cbSize = sizeof(config);
    config.hwndParent = alert->owner;
    config.hInstance = gkInstance();
    config.dwFlags = TDF_POSITION_RELATIVE_TO_WINDOW | TDF_SIZE_TO_CONTENT | TDF_ENABLE_HYPERLINKS;
    if (alert->cancel >= 0) {
        config.dwFlags |= TDF_ALLOW_DIALOG_CANCELLATION;
    }
    std::wstring windowTitle = gkProgramName();
    config.pszWindowTitle = windowTitle.c_str();
    config.pszMainInstruction = alert->title.c_str();
    config.pszContent = alert->message.empty() ? nullptr : alert->message.c_str();
    config.pszExpandedInformation = alert->details.empty() ? nullptr : alert->details.c_str();
    HICON icon = nullptr;
    if (alert->icon) {
        icon = gkIconHandle(alert->icon, (int)lround(32 * gkScaleOf(alert->owner)), gkPalette(gkResolvedTheme()).text);
        config.dwFlags |= TDF_USE_HICON_MAIN;
        config.hMainIcon = icon;
    } else {
        switch (alert->severity) {
        case 2:
            config.pszMainIcon = TD_WARNING_ICON;
            break;
        case 3:
            config.pszMainIcon = TD_ERROR_ICON;
            break;
        default:
            config.pszMainIcon = TD_INFORMATION_ICON;
            break;
        }
    }
    std::vector<TASKDIALOG_BUTTON> buttons;
    for (size_t i = 0; i < alert->buttons.size(); i++) {
        buttons.push_back(TASKDIALOG_BUTTON{gkFirstButton + (int)i, alert->buttons[i].c_str()});
    }
    config.pButtons = buttons.data();
    config.cButtons = (UINT)buttons.size();
    config.nDefaultButton = gkFirstButton;
    config.pfCallback = gkTaskCallback;
    config.lpCallbackData = reinterpret_cast<LONG_PTR>(alert);
    int command = IDCANCEL;
    if (FAILED(gkComctl().TaskDialogIndirect(&config, &command, nullptr, nullptr))) {
        command = IDCANCEL;
    }
    if (icon) {
        DestroyIcon(icon);
    }
    alert->response = gkTaskButton(alert, command);
    alert->done = 1;
    CoUninitialize();
    return 0;
}

GKAlert::~GKAlert() {
    if (thread) {
        if (HWND open = dialog.load()) {
            PostMessageW(open, TDM_CLICK_BUTTON, IDCANCEL, 0);
        }
        /* The thread of the dialog can send messages to the window, so the
           window keeps its messages flowing while it waits. */
        while (MsgWaitForMultipleObjects(1, &thread, FALSE, INFINITE, QS_ALLINPUT) == WAIT_OBJECT_0 + 1) {
            gkPumpOnce();
        }
        CloseHandle(thread);
    }
    if (window && IsWindow(window)) {
        SetWindowLongPtrW(window, GWLP_USERDATA, 0);
        DestroyWindow(window);
    }
    if (owner && IsWindow(owner)) {
        EnableWindow(owner, TRUE);
    }
    if (icon) {
        gkReleaseObject(icon);
    }
}

/* ---- Prompts ----

   A prompt asks for a line of text, which a task dialog cannot do, so it
   is a dialog of the bridge in the style of the task dialog. */

static const wchar_t *gkPromptClass = L"GameKitPrompt";

static LRESULT CALLBACK gkPromptProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    GKAlert *alert = reinterpret_cast<GKAlert *>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
    switch (message) {
    case WM_COMMAND:
        if (alert && HIWORD(wparam) == BN_CLICKED) {
            int id = LOWORD(wparam);
            if (id == IDOK) {
                id = gkFirstButton;
            }
            if (id >= gkFirstButton && id < gkFirstButton + (int)alert->buttons.size()) {
                alert->response = id - gkFirstButton;
                alert->done = 1;
                return 0;
            }
            if (id == IDCANCEL && alert->cancel >= 0) {
                alert->response = alert->cancel;
                alert->done = 1;
                return 0;
            }
        }
        break;
    case WM_CLOSE:
        if (alert && !alert->done) {
            alert->response = alert->cancel >= 0 ? alert->cancel : 0;
            alert->done = 1;
        }
        return 0;
    case WM_CTLCOLORSTATIC:
    case WM_CTLCOLOREDIT:
    case WM_CTLCOLORBTN:
        return gkCtlColor((HWND)lparam, (HDC)wparam, message);
    case WM_ERASEBKGND: {
        RECT client;
        GetClientRect(hwnd, &client);
        GKColor color = gkPalette(gkResolvedTheme()).window;
        HBRUSH brush = CreateSolidBrush(RGB((int)(color.r * 255), (int)(color.g * 255), (int)(color.b * 255)));
        FillRect((HDC)wparam, &client, brush);
        DeleteObject(brush);
        /* The buttons sit on the gray band of a task dialog. */
        GKColor band = gkPalette(gkResolvedTheme()).dark ? gkRGB(0x2b2b2b) : gkRGB(0xf0f0f0);
        RECT bottom = client;
        bottom.top = client.bottom - (int)lround(42 * gkScaleOf(hwnd));
        brush = CreateSolidBrush(RGB((int)(band.r * 255), (int)(band.g * 255), (int)(band.b * 255)));
        FillRect((HDC)wparam, &bottom, brush);
        DeleteObject(brush);
        return 1;
    }
    default:
        break;
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

static void gkPromptBuild(GKAlert *alert) {
    static bool registered;
    if (!registered) {
        registered = true;
        WNDCLASSEXW cls = {};
        cls.cbSize = sizeof(cls);
        cls.lpfnWndProc = gkPromptProc;
        cls.hInstance = gkInstance();
        cls.hCursor = LoadCursorW(nullptr, IDC_ARROW);
        cls.lpszClassName = gkPromptClass;
        RegisterClassExW(&cls);
    }
    GKActivation activation;
    std::wstring windowTitle = gkProgramName();
    alert->window = CreateWindowExW(WS_EX_DLGMODALFRAME | WS_EX_CONTROLPARENT, gkPromptClass, windowTitle.c_str(),
                                    WS_POPUP | WS_CAPTION | WS_SYSMENU | WS_CLIPCHILDREN, CW_USEDEFAULT, CW_USEDEFAULT,
                                    400, 200, alert->owner, nullptr, gkInstance(), nullptr);
    SetWindowLongPtrW(alert->window, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(alert));
    BOOL dark = gkResolvedTheme() == 2;
    DwmSetWindowAttribute(alert->window, 20, &dark, sizeof(dark));
    HWND hwnd = alert->window;
    double scale = gkScaleOf(hwnd);
    UINT dpi = GetDpiForWindow(hwnd);
    GKFont body = gkBodyFont();
    GKFont heading = gkTypeRamp(3);
    heading.weight = 400;
    int width = 360, pad = 12;
    auto px = [scale](double value) { return (int)lround(value * scale); };
    int y = pad;
    HICON icon = nullptr;
    gkComctl().LoadIconMetric(nullptr,
                              alert->severity == 3   ? MAKEINTRESOURCEW(32513)
                              : alert->severity == 2 ? MAKEINTRESOURCEW(32515)
                                                     : MAKEINTRESOURCEW(32516),
                              LIM_LARGE, &icon);
    HWND image = gkMakeNative(WC_STATICW, L"", SS_ICON | SS_REALSIZECONTROL, 0, hwnd);
    SendMessageW(image, STM_SETICON, (WPARAM)icon, 0);
    SetWindowPos(image, nullptr, px(pad), px(y), px(32), px(32), SWP_NOZORDER);
    int textX = pad + 32 + 10, textWidth = width - textX - pad;
    SIZE headingSize = gkMeasureGDI(alert->title, heading, textWidth);
    HWND title = gkMakeNative(WC_STATICW, alert->title.c_str(), SS_LEFT | SS_NOPREFIX, 0, hwnd);
    SendMessageW(title, WM_SETFONT, (WPARAM)gkFontHandle(heading, dpi), FALSE);
    gkSetTextRole(title, 9);
    SetWindowPos(title, nullptr, px(textX), px(y + 4), px(textWidth), px(headingSize.cy), SWP_NOZORDER);
    y += std::max<int>(36, headingSize.cy + 12);
    if (!alert->message.empty()) {
        SIZE messageSize = gkMeasureGDI(alert->message, body, textWidth);
        HWND label = gkMakeNative(WC_STATICW, alert->message.c_str(), SS_LEFT | SS_NOPREFIX, 0, hwnd);
        SendMessageW(label, WM_SETFONT, (WPARAM)gkFontHandle(body, dpi), FALSE);
        SetWindowPos(label, nullptr, px(textX), px(y), px(textWidth), px(messageSize.cy), SWP_NOZORDER);
        y += messageSize.cy + 10;
    }
    int line = gkMeasureGDI(L"Ag", body).cy + 8;
    alert->edit = gkMakeNative(WC_EDITW, alert->text.c_str(),
                               ES_AUTOHSCROLL | WS_TABSTOP | WS_BORDER | (alert->secure ? ES_PASSWORD : 0), 0, hwnd);
    SendMessageW(alert->edit, WM_SETFONT, (WPARAM)gkFontHandle(body, dpi), FALSE);
    SendMessageW(alert->edit, EM_SETCUEBANNER, TRUE, (LPARAM)alert->placeholder.c_str());
    if (alert->secure) {
        SendMessageW(alert->edit, EM_SETPASSWORDCHAR, 0x25cf, 0);
    }
    SetWindowPos(alert->edit, nullptr, px(textX), px(y), px(textWidth), px(line), SWP_NOZORDER);
    y += line + 16;
    int bandTop = y;
    int x = width - pad;
    for (int i = (int)alert->buttons.size() - 1; i >= 0; i--) {
        int buttonWidth = std::max<int>(75, gkMeasureGDI(alert->buttons[i], body).cx + 24);
        x -= buttonWidth;
        HWND button = gkMakeNative(WC_BUTTONW, alert->buttons[i].c_str(),
                                   WS_TABSTOP | (i == 0 ? BS_DEFPUSHBUTTON : BS_PUSHBUTTON), 0, hwnd);
        SetWindowLongPtrW(button, GWLP_ID, gkFirstButton + i);
        SendMessageW(button, WM_SETFONT, (WPARAM)gkFontHandle(body, dpi), FALSE);
        SetWindowPos(button, nullptr, px(x), px(bandTop + 9), px(buttonWidth), px(24), SWP_NOZORDER);
        x -= 8;
        alert->controls.push_back(button);
    }
    int height = bandTop + 42;
    RECT frame = {0, 0, px(width), px(height)};
    AdjustWindowRectExForDpi(&frame, WS_POPUP | WS_CAPTION | WS_SYSMENU, FALSE, WS_EX_DLGMODALFRAME, dpi);
    int frameWidth = frame.right - frame.left, frameHeight = frame.bottom - frame.top;
    RECT area = {};
    if (alert->owner && IsWindowVisible(alert->owner)) {
        GetWindowRect(alert->owner, &area);
    } else {
        MONITORINFO info = {sizeof(info)};
        GetMonitorInfoW(MonitorFromPoint(POINT{0, 0}, MONITOR_DEFAULTTOPRIMARY), &info);
        area = info.rcWork;
    }
    SetWindowPos(hwnd, nullptr, area.left + (area.right - area.left - frameWidth) / 2,
                 area.top + (area.bottom - area.top - frameHeight) / 2, frameWidth, frameHeight,
                 SWP_NOZORDER | SWP_NOACTIVATE);
    for (HWND child = GetWindow(hwnd, GW_CHILD); child; child = GetWindow(child, GW_HWNDNEXT)) {
        gkApplyThemeTo(child, gkResolvedTheme());
    }
}

/* gkPromptKey answers Return and Escape in a prompt. */
bool gkPromptKey(MSG const *message) {
    if (message->message != WM_KEYDOWN ||
        (message->wParam != VK_RETURN && message->wParam != VK_ESCAPE && message->wParam != VK_TAB)) {
        return false;
    }
    HWND root = GetAncestor(message->hwnd, GA_ROOT);
    wchar_t name[32] = {};
    GetClassNameW(root, name, 32);
    if (wcscmp(name, gkPromptClass)) {
        return false;
    }
    if (message->wParam == VK_TAB) {
        HWND next = GetNextDlgTabItem(root, message->hwnd, (GetKeyState(VK_SHIFT) & 0x8000) != 0);
        if (next) {
            SetFocus(next);
        }
        return true;
    }
    GKAlert *alert = reinterpret_cast<GKAlert *>(GetWindowLongPtrW(root, GWLP_USERDATA));
    if (!alert || alert->done) {
        return false;
    }
    if (message->wParam == VK_RETURN) {
        alert->response = 0;
        alert->done = 1;
    } else if (alert->cancel >= 0) {
        alert->response = alert->cancel;
        alert->done = 1;
    }
    return true;
}

/* ---- Alerts ---- */

extern "C" void *gkWin32AlertCreate(int severity, const char *title, const char *message, const char *details,
                                    const char *buttons, size_t buttonsSize, int cancelIndex, int destructiveMask,
                                    int prompt, int secure, const char *text, const char *placeholder) {
    if (!gkInitialize(nullptr, 0)) {
        return nullptr;
    }
    GKAlert *alert = new GKAlert();
    alert->severity = severity;
    alert->title = gkWide(title);
    alert->message = gkWide(message);
    alert->details = gkWide(details);
    for (auto const &button : gkSplit(buttons, buttonsSize)) {
        alert->buttons.push_back(gkWide(button.c_str()));
    }
    for (size_t i = 0; i < alert->buttons.size(); i++) {
        alert->destructive.push_back((destructiveMask >> i) & 1);
    }
    alert->cancel = cancelIndex;
    alert->prompt = prompt != 0;
    alert->secure = secure != 0;
    alert->text = gkWide(text);
    alert->placeholder = gkWide(placeholder);
    return alert;
}

extern "C" void *gkWin32ModalBegin(void *pointer, void *parent) {
    GKAlert *alert = static_cast<GKAlert *>(pointer);
    if (!alert) {
        return nullptr;
    }
    GKWin *win = static_cast<GKWin *>(parent);
    alert->owner = win && win->hwnd && IsWindowVisible(win->hwnd) ? win->hwnd : nullptr;
    alert->done = 0;
    alert->response = -1;
    gkRetainObject(alert);
    if (alert->prompt) {
        gkPromptBuild(alert);
        if (alert->owner) {
            EnableWindow(alert->owner, FALSE);
        }
        ShowWindow(alert->window, SW_SHOW);
        SetForegroundWindow(alert->window);
        SetFocus(alert->edit);
        SendMessageW(alert->edit, EM_SETSEL, 0, -1);
        return alert;
    }
    alert->thread = CreateThread(nullptr, 0, gkTaskThread, alert, 0, nullptr);
    return alert;
}

extern "C" int gkWin32ModalStep(void *modal) {
    GKAlert *alert = static_cast<GKAlert *>(modal);
    if (!alert) {
        return 0;
    }
    gkPumpOnce();
    return alert->done ? alert->response.load() : -1;
}

extern "C" void gkWin32ModalEnd(void *modal, int abort) {
    (void)abort;
    GKAlert *alert = static_cast<GKAlert *>(modal);
    if (!alert) {
        return;
    }
    if (alert->prompt && alert->edit) {
        int length = GetWindowTextLengthW(alert->edit);
        std::wstring text((size_t)length, L'\0');
        if (length) {
            GetWindowTextW(alert->edit, &text[0], length + 1);
        }
        alert->answer = text;
    }
    if (alert->thread) {
        if (HWND open = alert->dialog.load()) {
            PostMessageW(open, TDM_CLICK_BUTTON, alert->cancel >= 0 ? IDCANCEL : gkFirstButton, 0);
        }
        while (MsgWaitForMultipleObjects(1, &alert->thread, FALSE, INFINITE, QS_ALLINPUT) == WAIT_OBJECT_0 + 1) {
            gkPumpOnce();
        }
        CloseHandle(alert->thread);
        alert->thread = nullptr;
    }
    if (alert->owner && IsWindow(alert->owner)) {
        EnableWindow(alert->owner, TRUE);
    }
    if (alert->window && IsWindow(alert->window)) {
        ShowWindow(alert->window, SW_HIDE);
    }
    if (alert->owner && IsWindow(alert->owner)) {
        SetForegroundWindow(alert->owner);
    }
    gkReleaseObject(alert);
}

extern "C" char *gkWin32AlertText(void *pointer) {
    GKAlert *alert = static_cast<GKAlert *>(pointer);
    if (!alert) {
        return gkStrdup("");
    }
    if (alert->edit && IsWindow(alert->edit)) {
        int length = GetWindowTextLengthW(alert->edit);
        std::wstring text((size_t)length, L'\0');
        if (length) {
            GetWindowTextW(alert->edit, &text[0], length + 1);
        }
        return gkStrdupW(text);
    }
    return gkStrdupW(alert->answer);
}

/* ---- About box ----

   The about box is a task dialog that does not block the application. */

static std::vector<GKAlert *> &gkAbouts() {
    static std::vector<GKAlert *> abouts;
    return abouts;
}

extern "C" void gkWin32ShowAbout(const char *name, const char *version, const char *copyright, const char *website,
                                 const char *credits, void *icon) {
    if (!gkInitialize(nullptr, 0)) {
        return;
    }
    /* Finished about boxes go away. */
    auto &abouts = gkAbouts();
    for (auto it = abouts.begin(); it != abouts.end();) {
        if ((*it)->done) {
            gkReleaseObject(*it);
            it = abouts.erase(it);
        } else {
            ++it;
        }
    }
    GKAlert *about = new GKAlert();
    std::wstring program = name && *name ? gkWide(name) : gkProgramName();
    about->title = program;
    std::wstring content;
    if (version && *version) {
        content += L"Version " + gkWide(version);
    }
    if (credits && *credits) {
        content += (content.empty() ? L"" : L"\n\n") + gkWide(credits);
    }
    if (copyright && *copyright) {
        content += (content.empty() ? L"" : L"\n\n") + gkWide(copyright);
    }
    if (website && *website) {
        std::wstring link = gkWide(website);
        content += (content.empty() ? L"" : L"\n\n") + std::wstring(L"<a href=\"") + link + L"\">" + link + L"</a>";
    }
    about->message = content;
    about->buttons.push_back(L"OK");
    about->cancel = 0;
    about->icon = static_cast<GKIcon *>(icon);
    if (about->icon) {
        gkRetainObject(about->icon);
    }
    /* The about box has no owner, so the application keeps working while it
       shows. */
    about->owner = nullptr;
    about->thread = CreateThread(nullptr, 0, gkTaskThread, about, 0, nullptr);
    abouts.push_back(about);
}
