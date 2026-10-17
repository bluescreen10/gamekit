//go:build windows && cgo

#include "win32_internal.h"

#include <math.h>
#include <shellapi.h>

HMENU gkMenuNative(void *pointer, std::vector<std::string> &ids);
void gkMenuInvoke(void *pointer, std::string const &identifier);

/* A system tray item is an icon in the notification area of the taskbar,
   which Shell_NotifyIcon manages through a hidden window. The
   notification area shows only icons, so an item with a
   title and no icon shows the first letters of the title as its icon, and
   an item that wants attention shows a dot on its icon. */

static const UINT gkTrayMessage = WM_APP + 0x47;

struct GKTray : GKObject {
    HWND hwnd = nullptr;
    UINT id = 0;
    HICON icon = nullptr;
    std::wstring glyph;
    std::vector<unsigned char> pixels;
    int width = 0, height = 0;
    std::wstring title;
    std::wstring tooltip;
    int visible = 1;
    int attention = 0;
    int added = 0;
    GKObject *menu = nullptr;
    ~GKTray() override;
};

static LRESULT CALLBACK gkTrayProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);

static ATOM gkTrayClass() {
    static ATOM atom;
    if (!atom) {
        WNDCLASSEXW info = {};
        info.cbSize = sizeof(info);
        info.lpfnWndProc = gkTrayProc;
        info.hInstance = GetModuleHandleW(nullptr);
        info.lpszClassName = L"GameKitTray";
        atom = RegisterClassExW(&info);
    }
    return atom;
}

/* gkTrayIcon draws the icon of an item: the glyph, the image, or the
   initials of the title, with a dot when the item wants attention. */
static HICON gkTrayIcon(GKTray *tray) {
    int size = GetSystemMetrics(SM_CXSMICON);
    size = size > 0 ? size : 16;
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
    memset(pixels, 0, (size_t)size * size * 4);
    if (!tray->pixels.empty()) {
        for (int y = 0; y < size; y++) {
            for (int x = 0; x < size; x++) {
                int sx = x * tray->width / size, sy = y * tray->height / size;
                const unsigned char *p = tray->pixels.data() + ((size_t)sy * tray->width + sx) * 4;
                uint32_t a = p[3];
                pixels[y * size + x] = (a << 24) | ((uint32_t)(p[0] * a / 255) << 16) |
                                       ((uint32_t)(p[1] * a / 255) << 8) | (uint32_t)(p[2] * a / 255);
            }
        }
    } else {
        /* The notification area follows the taskbar theme, which is dark
           unless the system uses light mode for apps and the taskbar. */
        DWORD light = 0, length = sizeof(light);
        RegGetValueW(HKEY_CURRENT_USER, L"Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize",
                     L"SystemUsesLightTheme", RRF_RT_REG_DWORD, nullptr, &light, &length);
        COLORREF ink = light ? RGB(0, 0, 0) : RGB(255, 255, 255);
        std::wstring text = tray->glyph;
        const wchar_t *face = gkSymbolFamily().c_str();
        if (text.empty()) {
            face = L"Segoe UI";
            for (wchar_t c : tray->title) {
                if (iswalnum(c) && text.size() < 2) {
                    text += c;
                }
            }
        }
        HDC dc = CreateCompatibleDC(nullptr);
        HGDIOBJ previous = SelectObject(dc, color);
        HFONT font = CreateFontW(-(tray->glyph.empty() ? size * 3 / 4 : size), 0, 0, 0,
                                 tray->glyph.empty() ? FW_BOLD : FW_NORMAL, FALSE, FALSE, FALSE, DEFAULT_CHARSET,
                                 OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, ANTIALIASED_QUALITY, DEFAULT_PITCH, face);
        HGDIOBJ previousFont = SelectObject(dc, font);
        SetBkMode(dc, TRANSPARENT);
        /* The text is drawn white on black, so that its coverage is the
           value of each pixel, and then takes the ink color. */
        SetTextColor(dc, RGB(255, 255, 255));
        RECT area = {0, 0, size, size};
        DrawTextW(dc, text.c_str(), (int)text.size(), &area, DT_CENTER | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX);
        GdiFlush();
        /* GDI text has no alpha; the coverage becomes the alpha of the ink,
           with the color premultiplied. */
        for (int i = 0; i < size * size; i++) {
            uint32_t coverage = pixels[i] & 0xff;
            uint32_t r = GetRValue(ink) * coverage / 255, g = GetGValue(ink) * coverage / 255,
                     b = GetBValue(ink) * coverage / 255;
            pixels[i] = (coverage << 24) | (r << 16) | (g << 8) | b;
        }
        SelectObject(dc, previousFont);
        SelectObject(dc, previous);
        DeleteObject(font);
        DeleteDC(dc);
    }
    if (tray->attention) {
        int radius = size / 4;
        int cx = size - radius - 1, cy = radius + 1;
        for (int y = 0; y < size; y++) {
            for (int x = 0; x < size; x++) {
                double distance = sqrt((x + 0.5 - cx) * (x + 0.5 - cx) + (y + 0.5 - cy) * (y + 0.5 - cy));
                if (distance <= radius) {
                    pixels[y * size + x] = 0xffc42b1c;
                }
            }
        }
    }
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

static void gkTraySync(GKTray *tray) {
    NOTIFYICONDATAW data = {};
    data.cbSize = sizeof(data);
    data.hWnd = tray->hwnd;
    data.uID = tray->id;
    data.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_STATE | NIF_SHOWTIP;
    data.uCallbackMessage = gkTrayMessage;
    if (tray->icon) {
        DestroyIcon(tray->icon);
    }
    tray->icon = gkTrayIcon(tray);
    data.hIcon = tray->icon;
    std::wstring tip = !tray->tooltip.empty() ? tray->tooltip : tray->title;
    wcsncpy(data.szTip, tip.c_str(), sizeof(data.szTip) / sizeof(wchar_t) - 1);
    data.dwStateMask = NIS_HIDDEN;
    data.dwState = tray->visible ? 0 : NIS_HIDDEN;
    if (!tray->added) {
        tray->added = Shell_NotifyIconW(NIM_ADD, &data) != FALSE;
        data.uVersion = NOTIFYICON_VERSION_4;
        Shell_NotifyIconW(NIM_SETVERSION, &data);
    } else {
        Shell_NotifyIconW(NIM_MODIFY, &data);
    }
}

GKTray::~GKTray() {
    if (added) {
        NOTIFYICONDATAW data = {};
        data.cbSize = sizeof(data);
        data.hWnd = hwnd;
        data.uID = id;
        Shell_NotifyIconW(NIM_DELETE, &data);
    }
    if (icon) {
        DestroyIcon(icon);
    }
    if (menu) {
        gkReleaseObject(menu);
    }
    if (hwnd) {
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
        DestroyWindow(hwnd);
    }
}

static void gkTrayMenu(GKTray *tray, int x, int y) {
    if (!tray->menu) {
        return;
    }
    std::vector<std::string> ids;
    HMENU popup = gkMenuNative(tray->menu, ids);
    if (!popup) {
        return;
    }
    /* The menu closes when the user clicks elsewhere only if the window of
       the menu is in the foreground. */
    SetForegroundWindow(tray->hwnd);
    UINT command = TrackPopupMenuEx(popup, TPM_RETURNCMD | TPM_NONOTIFY | TPM_RIGHTBUTTON, x, y, tray->hwnd, nullptr);
    PostMessageW(tray->hwnd, WM_NULL, 0, 0);
    DestroyMenu(popup);
    if (command > 0 && command <= ids.size()) {
        gkMenuInvoke(tray->menu, ids[command - 1]);
    }
}

static LRESULT CALLBACK gkTrayProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    static UINT taskbarCreated = RegisterWindowMessageW(L"TaskbarCreated");
    GKTray *tray = reinterpret_cast<GKTray *>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
    if (tray && message == taskbarCreated) {
        /* Explorer restarted, and the item must be added again. */
        tray->added = 0;
        gkTraySync(tray);
        return 0;
    }
    if (tray && message == gkTrayMessage) {
        int x = (short)LOWORD(wparam), y = (short)HIWORD(wparam);
        switch (LOWORD(lparam)) {
        case NIN_SELECT:
        case NIN_KEYSELECT:
            gkSend(tray->handle, GK_EVENT_ACTIVATE, 0, 0, 0, 0, nullptr);
            return 0;
        case WM_CONTEXTMENU:
            gkTrayMenu(tray, x, y);
            return 0;
        default:
            return 0;
        }
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

extern "C" void *gkWin32TrayCreate(uintptr_t handle) {
    static UINT next;
    auto tray = new GKTray();
    tray->handle = handle;
    tray->id = ++next;
    tray->hwnd = CreateWindowExW(0, reinterpret_cast<LPCWSTR>((uintptr_t)gkTrayClass()), L"", WS_POPUP, 0, 0, 0, 0,
                                 nullptr, nullptr, GetModuleHandleW(nullptr), nullptr);
    SetWindowLongPtrW(tray->hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(tray));
    return tray;
}

extern "C" void gkWin32TrayConfigure(void *pointer, void *icon, int isTemplate, const char *title,
                                     const char *tooltip) {
    (void)isTemplate;
    GKTray *tray = static_cast<GKTray *>(pointer);
    if (!tray) {
        return;
    }
    GKIcon *source = static_cast<GKIcon *>(icon);
    tray->glyph = source ? source->glyph : std::wstring();
    tray->pixels = source ? source->pixels : std::vector<unsigned char>();
    tray->width = source ? source->width : 0;
    tray->height = source ? source->height : 0;
    tray->title = gkWide(title);
    tray->tooltip = gkWide(tooltip);
    gkTraySync(tray);
}

extern "C" void gkWin32TraySetVisible(void *pointer, int visible) {
    GKTray *tray = static_cast<GKTray *>(pointer);
    if (tray) {
        tray->visible = visible != 0;
        gkTraySync(tray);
    }
}

extern "C" void gkWin32TraySetAttention(void *pointer, int attention) {
    GKTray *tray = static_cast<GKTray *>(pointer);
    if (tray) {
        tray->attention = attention != 0;
        gkTraySync(tray);
    }
}

extern "C" void gkWin32TraySetMenu(void *pointer, void *menu) {
    GKTray *tray = static_cast<GKTray *>(pointer);
    if (!tray) {
        return;
    }
    if (menu) {
        gkRetainObject(static_cast<GKObject *>(menu));
    }
    if (tray->menu) {
        gkReleaseObject(tray->menu);
    }
    tray->menu = static_cast<GKObject *>(menu);
}

extern "C" int gkWin32TrayBounds(void *pointer, double *x, double *y, double *width, double *height) {
    GKTray *tray = static_cast<GKTray *>(pointer);
    if (!tray || !tray->added || !tray->visible) {
        return 0;
    }
    NOTIFYICONIDENTIFIER identifier = {};
    identifier.cbSize = sizeof(identifier);
    identifier.hWnd = tray->hwnd;
    identifier.uID = tray->id;
    RECT rect = {};
    if (FAILED(Shell_NotifyIconGetRect(&identifier, &rect))) {
        return 0;
    }
    double scale = GetDpiForSystem() / 96.0;
    *x = rect.left / scale;
    *y = rect.top / scale;
    *width = (rect.right - rect.left) / scale;
    *height = (rect.bottom - rect.top) / scale;
    return 1;
}

extern "C" void gkWin32TrayRemove(void *pointer) {
    GKTray *tray = static_cast<GKTray *>(pointer);
    if (tray) {
        tray->handle = 0;
        gkReleaseObject(tray);
    }
}
