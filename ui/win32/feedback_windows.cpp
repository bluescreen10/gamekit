//go:build windows && cgo

#include "win32_internal.h"

#include <initguid.h>
#include <oleacc.h>
#include <shlobj.h>
#include <shobjidl.h>
#include <uiautomation.h>

#include <algorithm>

/* Severity: 0 information, 1 success, 2 warning, 3 error. */
static const wchar_t *gkSeverityGlyph(int severity) {
    switch (severity) {
    case 1:
        return L"\xE73E";
    case 2:
        return L"\xE7BA";
    case 3:
        return L"\xEA39";
    default:
        return L"\xE946";
    }
}

static int gkSeverityRole(int severity) {
    switch (severity) {
    case 1:
        return 6;
    case 2:
        return 7;
    case 3:
        return 8;
    default:
        return 9;
    }
}

static GKColor gkSeverityColor(GKPalette const &palette, int severity) {
    switch (severity) {
    case 1:
        return palette.success;
    case 2:
        return palette.caution;
    case 3:
        return palette.critical;
    default:
        return palette.information;
    }
}

static GKColor gkSeverityBackground(GKPalette const &palette, int severity) {
    switch (severity) {
    case 1:
        return palette.successBackground;
    case 2:
        return palette.cautionBackground;
    case 3:
        return palette.criticalBackground;
    default:
        return palette.informationBackground;
    }
}

/* gkGlyphLabel creates a label that shows a glyph of the symbol font. */
static HWND gkGlyphLabel(HWND parent, const wchar_t *glyph, float size) {
    HWND label = gkMakeNative(WC_STATICW, glyph, SS_CENTER | SS_CENTERIMAGE | SS_NOPREFIX, 0, parent);
    GKFont symbol;
    symbol.family = gkSymbolFamily();
    symbol.size = size;
    SendMessageW(label, WM_SETFONT, (WPARAM)gkFontHandle(symbol, GetDpiForWindow(label)), FALSE);
    return label;
}

/* gkActionButtons creates a button for each action. */
static std::vector<HWND> gkActionButtons(HWND parent, GKView *owner, const char *ids, size_t idsSize,
                                         const char *titles, size_t titlesSize, const int *enabled, int count,
                                         std::vector<std::string> *identifiers) {
    std::vector<HWND> buttons;
    auto keys = gkSplit(ids, idsSize);
    auto names = gkSplit(titles, titlesSize);
    for (int i = 0; i < count; i++) {
        std::wstring name = gkWide(i < (int)names.size() ? names[i].c_str() : "");
        HWND button = gkMakeNative(WC_BUTTONW, name.c_str(), BS_PUSHBUTTON | WS_TABSTOP, 0, parent);
        if (owner) {
            SetPropW(button, L"GKView", owner);
        }
        SendMessageW(button, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(button)), FALSE);
        EnableWindow(button, enabled[i] ? TRUE : FALSE);
        buttons.push_back(button);
        identifiers->push_back(i < (int)keys.size() ? keys[i] : "");
    }
    return buttons;
}

static double gkButtonWidth(HWND button) {
    int length = GetWindowTextLengthW(button);
    std::wstring text((size_t)length, L'\0');
    if (length) {
        GetWindowTextW(button, &text[0], length + 1);
    }
    return std::max<double>(72, gkMeasureGDI(text, gkBodyFont()).cx + 24);
}

/* ---- Announcements ----

   An announcement is the text of a hidden label of the window that assistive
   technology reads as a live region. */

static const GUID gkLiveSettingProperty = {0xc12bcd8e, 0x2a8e, 0x4950, {0x8a, 0xe7, 0x36, 0x25, 0x11, 0x1d, 0x58, 0xeb}};

static HWND gkAnnouncer(HWND top) {
    HWND label = static_cast<HWND>(GetPropW(top, L"GKAnnouncer"));
    if (label && IsWindow(label)) {
        return label;
    }
    label = gkMakeNative(WC_STATICW, L"", SS_LEFT | SS_NOPREFIX, 0, top);
    SetWindowPos(label, HWND_BOTTOM, -10000, -10000, 1, 1, SWP_NOACTIVATE);
    SetPropW(top, L"GKAnnouncer", label);
    return label;
}

void gkAnnounceText(HWND hwnd, std::wstring const &text) {
    if (!hwnd || text.empty()) {
        return;
    }
    HWND top = GetAncestor(hwnd, GA_ROOT);
    HWND label = gkAnnouncer(top ? top : hwnd);
    SetWindowTextW(label, text.c_str());
    IAccPropServices *services = nullptr;
    if (SUCCEEDED(CoCreateInstance(CLSID_AccPropServices, nullptr, CLSCTX_INPROC_SERVER, IID_IAccPropServices,
                                   reinterpret_cast<void **>(&services)))) {
        VARIANT polite;
        VariantInit(&polite);
        polite.vt = VT_I4;
        polite.lVal = 1;
        services->SetHwndProp(label, OBJID_CLIENT, CHILDID_SELF, gkLiveSettingProperty, polite);
        services->SetHwndPropStr(label, OBJID_CLIENT, CHILDID_SELF, PROPID_ACC_NAME, text.c_str());
        services->Release();
    }
    NotifyWinEvent(EVENT_OBJECT_LIVEREGIONCHANGED, label, OBJID_CLIENT, CHILDID_SELF);
}

extern "C" void gkWin32Announce(void *element, const char *text, int priority) {
    (void)priority;
    GKView *view = gkView(element);
    if (view) {
        gkAnnounceText(view->outer, gkWide(text));
        return;
    }
    GKWin *win = static_cast<GKWin *>(element);
    for (GKWin *live : gkWindows()) {
        if (live == win) {
            gkAnnounceText(win->hwnd, gkWide(text));
        }
    }
}

/* ---- Info bar ----

   An info bar is a band in the color of its severity with an icon, a title,
   a message, its actions, and a close button. */

struct GKInfoBar : GKExtension {
    int severity = 0;
    HWND icon = nullptr, title = nullptr, message = nullptr, close = nullptr;
    std::vector<HWND> buttons;
    std::vector<std::string> ids;
    void colors(GKView *view) {
        GKPalette const &palette = gkViewPalette(view);
        GKColor background = gkSeverityBackground(palette, severity);
        gkLayoutSetBackground(view->outer, &background);
        gkSetTextRole(icon, gkSeverityRole(severity));
        InvalidateRect(view->outer, nullptr, TRUE);
    }
    void layout(GKView *view) override {
        double height = view->height, x = 12;
        gkPlaceWindow(icon, x, 0, 20, std::min(height, 40.0));
        x += 28;
        double right = view->width - 8;
        if (close) {
            right -= 28;
            gkPlaceWindow(close, right, (std::min(height, 40.0) - 28) / 2, 28, 28);
            right -= 8;
        }
        for (auto it = buttons.rbegin(); it != buttons.rend(); ++it) {
            double width = gkButtonWidth(*it);
            right -= width;
            gkPlaceWindow(*it, right, (std::min(height, 40.0) - 26) / 2, width, 26);
            right -= 8;
        }
        GKFont strong = gkTypeRamp(2);
        int titleWidth = gkMeasureGDI(view->text, strong).cx;
        gkPlaceWindow(title, x, 0, std::min<double>(titleWidth + 2, std::max(0.0, right - x)), std::min(height, 40.0));
        x += titleWidth + 12;
        gkPlaceWindow(message, x, 0, std::max(0.0, right - x), std::min(height, 40.0));
    }
    bool command(GKView *view, HWND from, int code) override {
        if (code != BN_CLICKED) {
            return false;
        }
        if (from == close) {
            gkSend(view->handle, GK_EVENT_CLOSE, 0, 0, 0, 0, nullptr);
            return true;
        }
        for (size_t i = 0; i < buttons.size(); i++) {
            if (buttons[i] == from) {
                gkSend(view->handle, GK_EVENT_ACTION, 0, 0, 0, 0, ids[i].c_str());
                return true;
            }
        }
        return false;
    }
    void themeChanged(GKView *view) override {
        colors(view);
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        double width = 48 + gkMeasureGDI(view->text, gkTypeRamp(2)).cx + 12 +
                       gkMeasureGDI(view->placeholder, gkBodyFont()).cx + 12;
        for (HWND button : buttons) {
            width += gkButtonWidth(button) + 8;
        }
        return D2D1::SizeF((float)width + (close ? 36 : 0), 40);
    }
};

extern "C" void *gkWin32InfoBarCreate(uintptr_t window, int closable, const char *ids, size_t idsSize,
                                      const char *titles, size_t titlesSize, const int *enabled, int count) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_INFO_BAR, window, 0);
    GKInfoBar *bar = new GKInfoBar();
    view->ext.reset(bar);
    bar->icon = gkGlyphLabel(host, gkSeverityGlyph(0), 14);
    bar->title = gkMakeNative(WC_STATICW, L"", SS_LEFT | SS_CENTERIMAGE | SS_NOPREFIX | SS_ENDELLIPSIS, 0, host);
    bar->message = gkMakeNative(WC_STATICW, L"", SS_LEFT | SS_CENTERIMAGE | SS_NOPREFIX | SS_ENDELLIPSIS, 0, host);
    SendMessageW(bar->title, WM_SETFONT, (WPARAM)gkFontHandle(gkTypeRamp(2), GetDpiForWindow(host)), FALSE);
    SendMessageW(bar->message, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(host)), FALSE);
    bar->buttons = gkActionButtons(host, view, ids, idsSize, titles, titlesSize, enabled, count, &bar->ids);
    if (closable) {
        bar->close = gkMakeNative(WC_BUTTONW, L"\xE711", BS_PUSHBUTTON | WS_TABSTOP, 0, host);
        SetPropW(bar->close, L"GKView", view);
        GKFont symbol;
        symbol.family = gkSymbolFamily();
        symbol.size = 10;
        SendMessageW(bar->close, WM_SETFONT, (WPARAM)gkFontHandle(symbol, GetDpiForWindow(host)), FALSE);
    }
    bar->colors(view);
    return view;
}

extern "C" void gkWin32InfoBarSet(void *pointer, int severity, const char *title, const char *detail) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_INFO_BAR) {
        return;
    }
    GKInfoBar *bar = static_cast<GKInfoBar *>(view->ext.get());
    bar->severity = severity >= 0 && severity < 4 ? severity : 0;
    view->text = gkWide(title);
    view->placeholder = gkWide(detail);
    SetWindowTextW(bar->icon, gkSeverityGlyph(bar->severity));
    SetWindowTextW(bar->title, view->text.c_str());
    SetWindowTextW(bar->message, view->placeholder.c_str());
    std::string text = std::string(title ? title : "") + (detail && *detail ? " " : "") + (detail ? detail : "");
    if (view->accessibleName.empty() || view->accessibleName == text) {
        view->accessibleName = text;
    }
    bar->colors(view);
    bar->layout(view);
}

/* ---- Tooltip ----

   A tooltip that the application shows is a tracking tooltip of
   Windows. */

struct GKTooltip : GKObject {
    HWND tip = nullptr;
    std::wstring text;
    HWND anchor = nullptr;
    int shown = 0;
    UINT_PTR timer = 0;
    ~GKTooltip() override {
        gkTimerCancel(timer);
        if (tip) {
            DestroyWindow(tip);
        }
    }
};

extern "C" void *gkWin32TooltipCreate(void) {
    GKTooltip *tooltip = new GKTooltip();
    GKActivation activation;
    tooltip->tip = CreateWindowExW(WS_EX_TOPMOST | WS_EX_TRANSPARENT, TOOLTIPS_CLASSW, nullptr,
                                   WS_POPUP | TTS_NOPREFIX | TTS_ALWAYSTIP, CW_USEDEFAULT, CW_USEDEFAULT,
                                   CW_USEDEFAULT, CW_USEDEFAULT, nullptr, nullptr, gkInstance(), nullptr);
    SendMessageW(tooltip->tip, TTM_SETMAXTIPWIDTH, 0, 400);
    TOOLINFOW info = {};
    info.cbSize = sizeof(info);
    info.uFlags = TTF_TRACK | TTF_ABSOLUTE;
    info.uId = 1;
    info.lpszText = const_cast<wchar_t *>(L"");
    SendMessageW(tooltip->tip, TTM_ADDTOOLW, 0, (LPARAM)&info);
    return tooltip;
}

extern "C" void gkWin32TooltipSetText(void *pointer, const char *text) {
    GKTooltip *tooltip = static_cast<GKTooltip *>(pointer);
    if (!tooltip) {
        return;
    }
    tooltip->text = gkWide(text);
    TOOLINFOW info = {};
    info.cbSize = sizeof(info);
    info.uId = 1;
    info.lpszText = const_cast<wchar_t *>(tooltip->text.c_str());
    SendMessageW(tooltip->tip, TTM_UPDATETIPTEXTW, 0, (LPARAM)&info);
}

static void gkTooltipOpen(GKTooltip *tooltip, int edge) {
    if (!tooltip->anchor || !IsWindow(tooltip->anchor) || !IsWindowVisible(tooltip->anchor)) {
        return;
    }
    RECT anchor;
    GetWindowRect(tooltip->anchor, &anchor);
    TOOLINFOW info = {};
    info.cbSize = sizeof(info);
    info.uId = 1;
    SendMessageW(tooltip->tip, TTM_TRACKACTIVATE, TRUE, (LPARAM)&info);
    RECT tip;
    GetWindowRect(tooltip->tip, &tip);
    int width = tip.right - tip.left, height = tip.bottom - tip.top, gap = 4;
    int x = (anchor.left + anchor.right - width) / 2, y = anchor.bottom + gap;
    switch (edge) {
    case 0:
        y = anchor.top - height - gap;
        break;
    case 2:
        x = anchor.left - width - gap;
        y = (anchor.top + anchor.bottom - height) / 2;
        break;
    case 3:
        x = anchor.right + gap;
        y = (anchor.top + anchor.bottom - height) / 2;
        break;
    default:
        break;
    }
    SendMessageW(tooltip->tip, TTM_TRACKPOSITION, 0, MAKELPARAM(x, y));
    tooltip->shown = 1;
}

extern "C" int gkWin32TooltipShow(void *pointer, void *anchorPointer, int edge, double delay) {
    GKTooltip *tooltip = static_cast<GKTooltip *>(pointer);
    GKView *anchor = gkView(anchorPointer);
    GKWin *win = anchor ? gkWinOf(anchor->outer) : nullptr;
    if (!tooltip || !anchor || !win || !IsWindowVisible(win->hwnd) || !IsWindowVisible(anchor->outer)) {
        return 0;
    }
    gkTimerCancel(tooltip->timer);
    tooltip->timer = 0;
    tooltip->anchor = anchor->outer;
    if (delay <= 0) {
        gkTooltipOpen(tooltip, edge);
        return 1;
    }
    tooltip->shown = 1;
    tooltip->timer = gkTimer((UINT)lround(delay * 1000), [tooltip, edge]() {
        tooltip->timer = 0;
        gkTooltipOpen(tooltip, edge);
        return false;
    });
    return 1;
}

extern "C" void gkWin32TooltipHide(void *pointer) {
    GKTooltip *tooltip = static_cast<GKTooltip *>(pointer);
    if (!tooltip) {
        return;
    }
    gkTimerCancel(tooltip->timer);
    tooltip->timer = 0;
    TOOLINFOW info = {};
    info.cbSize = sizeof(info);
    info.uId = 1;
    SendMessageW(tooltip->tip, TTM_TRACKACTIVATE, FALSE, (LPARAM)&info);
    tooltip->shown = 0;
}

extern "C" int gkWin32TooltipShown(void *pointer) {
    GKTooltip *tooltip = static_cast<GKTooltip *>(pointer);
    return tooltip && tooltip->shown;
}

/* ---- Toast ----

   A toast is a message that floats above the bottom of the content of a
   window for a while: a dark band with the message and its actions, in
   the style of the notifications of Windows. */

struct GKToast : GKObject {
    GKView *view = nullptr;
    HWND message = nullptr;
    std::vector<HWND> buttons;
    std::vector<std::string> ids;
    double timeout = 0;
    UINT_PTR timer = 0;
    GKWin *win = nullptr;
    ~GKToast() override;
};

static void gkToastDismiss(GKToast *toast) {
    gkTimerCancel(toast->timer);
    toast->timer = 0;
    if (toast->view && toast->win) {
        ShowWindow(toast->view->outer, SW_HIDE);
        gkReparent(toast->view->outer, gkParking());
    }
    toast->win = nullptr;
}

struct GKToastBand : GKExtension {
    GKToast *toast = nullptr;
    void layout(GKView *view) override {
        double x = 14, height = view->height;
        double width = gkMeasureGDI(view->text, gkBodyFont()).cx + 2;
        gkPlaceWindow(toast->message, x, 0, width, height);
        x += width + 12;
        for (HWND button : toast->buttons) {
            double w = gkButtonWidth(button);
            gkPlaceWindow(button, x, (height - 26) / 2, w, 26);
            x += w + 8;
        }
    }
    double width(GKView *view) {
        double total = 28 + gkMeasureGDI(view->text, gkBodyFont()).cx + 2;
        for (HWND button : toast->buttons) {
            total += gkButtonWidth(button) + 8 + 4;
        }
        return std::min(total, 560.0);
    }
    bool command(GKView *view, HWND from, int code) override {
        (void)view;
        if (code != BN_CLICKED) {
            return false;
        }
        for (size_t i = 0; i < toast->buttons.size(); i++) {
            if (toast->buttons[i] == from) {
                std::string id = toast->ids[i];
                gkSend(toast->handle, GK_EVENT_ACTION, 0, 0, 0, 0, id.c_str());
                gkToastDismiss(toast);
                return true;
            }
        }
        return false;
    }
};

GKToast::~GKToast() {
    gkToastDismiss(this);
    if (view) {
        gkReleaseObject(view);
    }
}

extern "C" void *gkWin32ToastCreate(uintptr_t handle, const char *message, const char *ids, size_t idsSize,
                                    const char *titles, size_t titlesSize, const int *enabled, int count,
                                    double timeout) {
    GKToast *toast = new GKToast();
    toast->handle = handle;
    toast->timeout = timeout;
    HWND host = gkLayoutNew(gkParking());
    toast->view = gkViewNew(host, nullptr, GK_KIND_GENERIC, 0, 0);
    GKToastBand *band = new GKToastBand();
    band->toast = toast;
    toast->view->ext.reset(band);
    toast->view->text = gkWide(message);
    toast->message = gkMakeNative(WC_STATICW, toast->view->text.c_str(), SS_LEFT | SS_CENTERIMAGE | SS_NOPREFIX, 0,
                                  host);
    gkSetTextRole(toast->message, 10);
    SendMessageW(toast->message, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(host)), FALSE);
    toast->buttons = gkActionButtons(host, toast->view, ids, idsSize, titles, titlesSize, enabled, count, &toast->ids);
    return toast;
}

extern "C" int gkWin32ToastShow(void *pointer, void *window) {
    GKToast *toast = static_cast<GKToast *>(pointer);
    GKWin *win = static_cast<GKWin *>(window);
    if (!toast || !win || !win->root) {
        return 0;
    }
    gkToastDismiss(toast);
    GKPalette const &palette = gkPalette(gkWinTheme(win));
    GKColor band = palette.dark ? gkRGB(0xf3f3f3) : gkRGB(0x2b2b2b);
    gkLayoutSetBackground(toast->view->outer, &band);
    gkReparent(toast->view->outer, win->root);
    D2D1_SIZE_F content = gkClientSize(win->root);
    GKToastBand *data = static_cast<GKToastBand *>(toast->view->ext.get());
    double width = data->width(toast->view), height = 40;
    toast->view->x = (int)lround((content.width - width) / 2);
    toast->view->y = (int)lround(content.height - height - 20);
    toast->view->width = (int)lround(width);
    toast->view->height = (int)height;
    gkPlace(toast->view);
    data->layout(toast->view);
    /* The band has rounded corners. */
    double scale = gkScaleOf(win->root);
    int radius = (int)lround(8 * scale);
    HRGN region = CreateRoundRectRgn(0, 0, (int)lround(width * scale) + 1, (int)lround(height * scale) + 1, radius,
                                     radius);
    SetWindowRgn(toast->view->outer, region, TRUE);
    SetWindowPos(toast->view->outer, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_SHOWWINDOW);
    toast->win = win;
    if (toast->timeout > 0) {
        toast->timer = gkTimer((UINT)lround(toast->timeout * 1000), [toast]() {
            toast->timer = 0;
            gkToastDismiss(toast);
            return false;
        });
    }
    gkAnnounceText(win->hwnd, toast->view->text);
    return 1;
}

extern "C" void gkWin32ToastDismiss(void *pointer) {
    GKToast *toast = static_cast<GKToast *>(pointer);
    if (toast) {
        gkToastDismiss(toast);
    }
}

extern "C" int gkWin32ToastShown(void *pointer) {
    GKToast *toast = static_cast<GKToast *>(pointer);
    return toast && toast->win != nullptr;
}

extern "C" void gkWin32ToastRelease(void *pointer) {
    GKToast *toast = static_cast<GKToast *>(pointer);
    if (!toast) {
        return;
    }
    toast->handle = 0;
    gkReleaseObject(toast);
}

/* ---- Badge ----

   A badge is a count or a short text in a pill in the color of its
   severity, which Windows has no control for. */

struct GKBadge : GKExtension {
    int severity = 0;
    void paint(GKView *view, GKCanvas &canvas) override {
        GKPalette const &palette = gkViewPalette(view);
        GKFont font = gkTypeRamp(1);
        font.weight = 700;
        D2D1_SIZE_F text = gkMeasureText(view->text, font);
        float height = std::min(canvas.height, text.height + 6);
        float width = std::min(canvas.width, std::max(height, text.width + 18));
        float x = (canvas.width - width) / 2, y = (canvas.height - height) / 2;
        canvas.fillRound(x, y, width, height, height / 2, gkSeverityColor(palette, severity));
        canvas.centeredText(view->text, font, x, y, width, height,
                            palette.dark ? gkRGB(0x000000) : gkRGB(0xffffff));
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        GKFont font = gkTypeRamp(1);
        font.weight = 700;
        D2D1_SIZE_F text = gkMeasureText(view->text, font);
        float height = text.height + 6;
        return D2D1::SizeF(std::max(height, text.width + 18), height);
    }
};

extern "C" void *gkWin32BadgeCreate(uintptr_t window) {
    return gkCreateDrawn(window, GK_KIND_BADGE, new GKBadge(), 0, 0);
}

extern "C" void gkWin32BadgeSet(void *pointer, const char *text, int severity, const char *accessible) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_BADGE) {
        return;
    }
    static_cast<GKBadge *>(view->ext.get())->severity = severity;
    view->text = gkWide(text);
    const char *value = accessible && *accessible ? accessible : text ? text : "";
    view->accessibleValue = value;
    if (view->accessibleName.empty()) {
        view->accessibleName = value;
    }
    InvalidateRect(view->outer, nullptr, FALSE);
}

/* IGKProgressDialog is the progress dialog interface of the shell, which
   the headers of MinGW leave out. */
struct IGKProgressDialog : IUnknown {
    virtual HRESULT STDMETHODCALLTYPE StartProgressDialog(HWND parent, IUnknown *modeless, DWORD flags,
                                                          LPCVOID reserved) = 0;
    virtual HRESULT STDMETHODCALLTYPE StopProgressDialog() = 0;
    virtual HRESULT STDMETHODCALLTYPE SetTitle(PCWSTR title) = 0;
    virtual HRESULT STDMETHODCALLTYPE SetAnimation(HINSTANCE instance, UINT resource) = 0;
    virtual BOOL STDMETHODCALLTYPE HasUserCancelled() = 0;
    virtual HRESULT STDMETHODCALLTYPE SetProgress(DWORD completed, DWORD total) = 0;
    virtual HRESULT STDMETHODCALLTYPE SetProgress64(ULONGLONG completed, ULONGLONG total) = 0;
    virtual HRESULT STDMETHODCALLTYPE SetLine(DWORD line, PCWSTR text, BOOL compact, LPCVOID reserved) = 0;
    virtual HRESULT STDMETHODCALLTYPE SetCancelMsg(PCWSTR message, LPCVOID reserved) = 0;
    virtual HRESULT STDMETHODCALLTYPE Timer(DWORD action, LPCVOID reserved) = 0;
};

static const CLSID gkProgressDialogClass = {0xf8383852, 0xfcd3, 0x11d1, {0xa6, 0xb9, 0x00, 0x60, 0x97, 0xdf, 0x5b, 0xd4}};
static const IID gkProgressDialogInterface = {0xebbc7c04, 0x315e, 0x11d2, {0xb6, 0x2f, 0x00, 0x60, 0x97, 0xdf, 0x5b, 0xd4}};

enum {
    GK_PROGDLG_NOTIME = 0x04,
    GK_PROGDLG_NOMINIMIZE = 0x08,
    GK_PROGDLG_NOCANCEL = 0x40,
};

/* ---- Progress dialog ----

   A progress dialog is the progress dialog of the shell of Windows, which
   runs without blocking the application. */

struct GKProgressDialog : GKObject {
    IGKProgressDialog *dialog = nullptr;
    int shown = 0;
    int cancellable = 0;
    int reported = 0;
    UINT_PTR timer = 0;
    std::wstring title;
    ~GKProgressDialog() override {
        gkTimerCancel(timer);
        if (dialog) {
            if (shown) {
                dialog->StopProgressDialog();
            }
            dialog->Release();
        }
    }
};

extern "C" void *gkWin32ProgressDialogCreate(uintptr_t handle, const char *title, int cancellable) {
    GKProgressDialog *progress = new GKProgressDialog();
    progress->handle = handle;
    progress->cancellable = cancellable != 0;
    progress->title = gkWide(title);
    if (FAILED(CoCreateInstance(gkProgressDialogClass, nullptr, CLSCTX_INPROC_SERVER, gkProgressDialogInterface,
                                reinterpret_cast<void **>(&progress->dialog)))) {
        delete progress;
        return nullptr;
    }
    progress->dialog->SetTitle(progress->title.c_str());
    if (cancellable) {
        progress->dialog->SetCancelMsg(L"Cancelling…", nullptr);
    }
    return progress;
}

extern "C" void gkWin32ProgressDialogSet(void *pointer, const char *message, double value) {
    GKProgressDialog *progress = static_cast<GKProgressDialog *>(pointer);
    if (!progress) {
        return;
    }
    progress->dialog->SetLine(1, gkWide(message).c_str(), FALSE, nullptr);
    if (value >= 0) {
        progress->dialog->SetProgress64((ULONGLONG)lround((value < 1 ? value : 1) * 10000), 10000);
    }
}

extern "C" int gkWin32ProgressDialogShow(void *pointer, void *parent) {
    GKProgressDialog *progress = static_cast<GKProgressDialog *>(pointer);
    GKWin *win = static_cast<GKWin *>(parent);
    if (!progress) {
        return 0;
    }
    HWND owner = win && win->hwnd && IsWindowVisible(win->hwnd) ? win->hwnd : nullptr;
    DWORD flags = GK_PROGDLG_NOMINIMIZE | GK_PROGDLG_NOTIME | (progress->cancellable ? 0 : GK_PROGDLG_NOCANCEL);
    if (FAILED(progress->dialog->StartProgressDialog(owner, nullptr, flags, nullptr))) {
        return 0;
    }
    progress->shown = 1;
    if (progress->cancellable) {
        progress->timer = gkTimer(100, [progress]() {
            if (progress->dialog->HasUserCancelled() && !progress->reported) {
                /* The Go layer closes the dialog; a cancel request only
                   reports. */
                progress->reported = 1;
                gkSend(progress->handle, GK_EVENT_CLOSE, 0, 0, 0, 0, nullptr);
            }
            return true;
        });
    }
    return 1;
}

extern "C" void gkWin32ProgressDialogRelease(void *pointer) {
    GKProgressDialog *progress = static_cast<GKProgressDialog *>(pointer);
    if (!progress) {
        return;
    }
    progress->handle = 0;
    gkReleaseObject(progress);
}

/* ---- Empty state ---- */

struct GKEmptyState : GKExtension {
    HWND icon = nullptr;
    HICON iconHandle = nullptr;
    HWND title = nullptr, detail = nullptr;
    std::vector<HWND> buttons;
    std::vector<std::string> ids;
    ~GKEmptyState() override {
        if (iconHandle) {
            DestroyIcon(iconHandle);
        }
    }
    void layout(GKView *view) override {
        double width = std::min<double>(360, view->width);
        GKFont subtitle = gkTypeRamp(3);
        SIZE titleSize = gkMeasureGDI(view->text, subtitle, (int)width);
        SIZE detailSize = view->placeholder.empty() ? SIZE{0, 0} : gkMeasureGDI(view->placeholder, gkBodyFont(), (int)width);
        double total = (icon ? 56 : 0) + titleSize.cy + 6 + detailSize.cy + (buttons.empty() ? 0 : 38);
        double y = std::max(0.0, (view->height - total) / 2), x = (view->width - width) / 2;
        if (icon) {
            gkPlaceWindow(icon, (view->width - 48) / 2, y, 48, 48);
            y += 56;
        }
        gkPlaceWindow(title, x, y, width, titleSize.cy);
        y += titleSize.cy + 6;
        gkPlaceWindow(detail, x, y, width, detailSize.cy);
        y += detailSize.cy + 12;
        double row = 0;
        for (HWND button : buttons) {
            row += gkButtonWidth(button) + 8;
        }
        double bx = (view->width - row + 8) / 2;
        for (HWND button : buttons) {
            double w = gkButtonWidth(button);
            gkPlaceWindow(button, bx, y, w, 26);
            bx += w + 8;
        }
    }
    bool command(GKView *view, HWND from, int code) override {
        if (code != BN_CLICKED) {
            return false;
        }
        for (size_t i = 0; i < buttons.size(); i++) {
            if (buttons[i] == from) {
                gkSend(view->handle, GK_EVENT_ACTION, 0, 0, 0, 0, ids[i].c_str());
                return true;
            }
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        SIZE titleSize = gkMeasureGDI(view->text, gkTypeRamp(3), 360);
        SIZE detailSize = view->placeholder.empty() ? SIZE{0, 0} : gkMeasureGDI(view->placeholder, gkBodyFont(), 360);
        return D2D1::SizeF(360, (float)((icon ? 56 : 0) + titleSize.cy + 6 + detailSize.cy + (buttons.empty() ? 0 : 38)));
    }
};

extern "C" void *gkWin32EmptyStateCreate(uintptr_t window, void *icon, const char *ids, size_t idsSize,
                                         const char *titles, size_t titlesSize, const int *enabled, int count) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_EMPTY_STATE, window, 0);
    GKEmptyState *state = new GKEmptyState();
    view->ext.reset(state);
    if (icon) {
        state->icon = gkMakeNative(WC_STATICW, L"", SS_ICON | SS_CENTERIMAGE | SS_REALSIZECONTROL, 0, host);
        int side = (int)lround(48 * gkScaleOf(host));
        state->iconHandle = gkIconHandle(static_cast<GKIcon *>(icon), side, gkPalette(gkViewTheme(view)).textTertiary);
        SendMessageW(state->icon, STM_SETICON, (WPARAM)state->iconHandle, 0);
    }
    state->title = gkMakeNative(WC_STATICW, L"", SS_CENTER | SS_NOPREFIX, 0, host);
    state->detail = gkMakeNative(WC_STATICW, L"", SS_CENTER | SS_NOPREFIX, 0, host);
    gkSetTextRole(state->detail, 2);
    SendMessageW(state->title, WM_SETFONT, (WPARAM)gkFontHandle(gkTypeRamp(3), GetDpiForWindow(host)), FALSE);
    SendMessageW(state->detail, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(host)), FALSE);
    state->buttons = gkActionButtons(host, view, ids, idsSize, titles, titlesSize, enabled, count, &state->ids);
    return view;
}

extern "C" void gkWin32EmptyStateSet(void *pointer, const char *title, const char *detail) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_EMPTY_STATE) {
        return;
    }
    GKEmptyState *state = static_cast<GKEmptyState *>(view->ext.get());
    view->text = gkWide(title);
    view->placeholder = gkWide(detail);
    SetWindowTextW(state->title, view->text.c_str());
    SetWindowTextW(state->detail, view->placeholder.c_str());
    ShowWindow(state->detail, view->placeholder.empty() ? SW_HIDE : SW_SHOWNA);
    if (view->accessibleName.empty() || view->accessibleName == gkUTF8(view->text)) {
        view->accessibleName = gkUTF8(view->text);
    }
    state->layout(view);
}

/* ---- Inline validation ---- */

struct GKValidation : GKExtension {
    HWND icon = nullptr, label = nullptr;
    int severity = 0;
    void layout(GKView *view) override {
        gkPlaceWindow(icon, 0, 1, 16, 16);
        gkPlaceWindow(label, 20, 0, std::max(0, view->width - 20), view->height);
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        int width = view->preferredWidth > 20 ? view->preferredWidth - 20 : 0;
        SIZE size = gkMeasureGDI(view->text, gkTypeRamp(1), width);
        return D2D1::SizeF((float)size.cx + 20, (float)std::max<LONG>(size.cy, 16));
    }
};

extern "C" void *gkWin32InlineValidationCreate(uintptr_t window) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_VALIDATION, window, 0);
    GKValidation *validation = new GKValidation();
    view->ext.reset(validation);
    validation->icon = gkGlyphLabel(host, gkSeverityGlyph(0), 12);
    validation->label = gkMakeNative(WC_STATICW, L"", SS_LEFT | SS_NOPREFIX, 0, host);
    SendMessageW(validation->label, WM_SETFONT, (WPARAM)gkFontHandle(gkTypeRamp(1), GetDpiForWindow(host)), FALSE);
    return view;
}

extern "C" void gkWin32InlineValidationSet(void *pointer, int severity, const char *text) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_VALIDATION) {
        return;
    }
    GKValidation *validation = static_cast<GKValidation *>(view->ext.get());
    validation->severity = severity;
    view->text = gkWide(text);
    SetWindowTextW(validation->icon, gkSeverityGlyph(severity));
    SetWindowTextW(validation->label, view->text.c_str());
    gkSetTextRole(validation->icon, gkSeverityRole(severity));
    gkSetTextRole(validation->label, severity >= 2 ? gkSeverityRole(severity) : 2);
    InvalidateRect(view->outer, nullptr, TRUE);
    gkAnnounceText(view->outer, view->text);
}
