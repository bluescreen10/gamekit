//go:build windows && cgo

#include "win32_internal.h"

#include <dwmapi.h>
#include <shellapi.h>

#include <algorithm>

#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

static std::vector<GKWin *> gkWindowList;
static uintptr_t gkAppHandle;
static int gkAppActive = -1;
static GKView *gkCaptureView;
static const wchar_t *gkWindowClass = L"GameKitWindow";

bool gkForwardNotification(UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result);
bool gkMenuCommand(HWND hwnd, int id);
bool gkPopoverOwns(HWND owner);

std::vector<GKWin *> &gkWindows() {
    return gkWindowList;
}

static void gkSetError(char *error, size_t size, const char *message) {
    if (error && size) {
        strncpy(error, message, size - 1);
        error[size - 1] = '\0';
    }
}

static bool gkWinLive(GKWin *win) {
    return win && std::find(gkWindowList.begin(), gkWindowList.end(), win) != gkWindowList.end();
}

GKWin *gkWinFromHandle(uintptr_t window) {
    return reinterpret_cast<GKWin *>(window);
}

GKWin *gkWinOf(HWND hwnd) {
    if (!hwnd) {
        return nullptr;
    }
    HWND root = GetAncestor(hwnd, GA_ROOT);
    for (GKWin *win : gkWindowList) {
        if (win->hwnd == root) {
            return win;
        }
    }
    return nullptr;
}

void gkWinNotify(GKWin *win, int kind, int a, double x, double y, const char *text) {
    if (win && win->handle) {
        gkSend(win->handle, kind, a, 0, x, y, text);
    }
}

double gkWinScale(GKWin *win) {
    if (win && win->hwnd) {
        return gkScaleOf(win->hwnd);
    }
    return GetDpiForSystem() / 96.0;
}

/* ---- Theme ---- */

static int gkRequestedTheme() {
    int theme = 0;
    for (GKWin *win : gkWindowList) {
        if (win->theme) {
            theme = win->theme;
        }
    }
    return theme;
}

int gkResolvedTheme() {
    int theme = gkRequestedTheme();
    if (theme) {
        return theme;
    }
    return gkSystemDark() ? 2 : 1;
}

int gkWinTheme(GKWin *win) {
    if (win && win->theme) {
        return win->theme;
    }
    return gkResolvedTheme();
}

/* gkThemeChildren gives the native controls of a window the visual style of
   its theme and redraws them. */
static BOOL CALLBACK gkThemeChild(HWND child, LPARAM theme) {
    GKView *view = gkViewOf(child);
    if (view && view->outer == child) {
        gkStyleResolve(view);
    }
    if (view && view->ext) {
        view->ext->themeChanged(view);
    }
    wchar_t name[64] = {};
    GetClassNameW(child, name, 64);
    if (wcscmp(name, L"GameKitLayout") && wcscmp(name, L"GameKitControl")) {
        gkApplyThemeTo(child, (int)theme);
    }
    InvalidateRect(child, nullptr, TRUE);
    return TRUE;
}

static void gkWinApplyTheme(GKWin *win) {
    if (!win || !win->hwnd) {
        return;
    }
    int theme = gkWinTheme(win);
    BOOL dark = theme == 2;
    DwmSetWindowAttribute(win->hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &dark, sizeof(dark));
    EnumChildWindows(win->hwnd, gkThemeChild, theme);
    /* The views of the window that are not in it now, such as the content
       of another tab or page, wait in the parking window. */
    for (HWND child = GetWindow(gkParking(), GW_CHILD); child; child = GetWindow(child, GW_HWNDNEXT)) {
        GKView *view = gkViewOf(child);
        if (view && view->window == win) {
            gkThemeChild(child, theme);
            EnumChildWindows(child, gkThemeChild, theme);
        }
    }
    RedrawWindow(win->hwnd, nullptr, nullptr, RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN | RDW_FRAME);
}

static void gkNotifyThemes() {
    for (GKWin *win : gkWindowList) {
        int resolved = gkWinTheme(win);
        if (resolved != win->resolvedTheme) {
            win->resolvedTheme = resolved;
            gkWinApplyTheme(win);
            gkWinNotify(win, GK_EVENT_WINDOW_THEME, resolved, 0, 0, nullptr);
        }
    }
}

void gkApplyThemes() {
    gkNotifyThemes();
}

/* ---- Content area ---- */

static int gkWinChromeTop(GKWin *win) {
    int height = 0;
    for (auto const &bar : win->chrome) {
        if (bar.top && bar.view && bar.view->visible) {
            height += bar.height;
        }
    }
    return height;
}

static int gkWinChromeBottom(GKWin *win) {
    int height = 0;
    for (auto const &bar : win->chrome) {
        if (!bar.top && bar.view && bar.view->visible) {
            height += bar.height;
        }
    }
    return height;
}

static double gkWinChromeHeight(GKWin *win) {
    return gkWinChromeTop(win) + gkWinChromeBottom(win);
}

void gkWinLayout(GKWin *win) {
    if (!win || !win->hwnd || !win->root) {
        return;
    }
    D2D1_SIZE_F client = gkClientSize(win->hwnd);
    double top = 0;
    for (auto const &bar : win->chrome) {
        if (bar.top && bar.view && bar.view->visible) {
            bar.view->x = 0;
            bar.view->y = (int)top;
            bar.view->width = (int)lround(client.width);
            bar.view->height = bar.height;
            gkPlace(bar.view);
            if (bar.view->ext) {
                bar.view->ext->layout(bar.view);
            }
            top += bar.height;
        }
    }
    double bottom = client.height;
    for (auto it = win->chrome.rbegin(); it != win->chrome.rend(); ++it) {
        if (!it->top && it->view && it->view->visible) {
            bottom -= it->height;
            it->view->x = 0;
            it->view->y = (int)lround(bottom);
            it->view->width = (int)lround(client.width);
            it->view->height = it->height;
            gkPlace(it->view);
            if (it->view->ext) {
                it->view->ext->layout(it->view);
            }
        }
    }
    gkPlaceWindow(win->root, 0, top, client.width, bottom - top > 0 ? bottom - top : 0);
}

void gkWinAddChrome(GKWin *win, GKView *view, int top, int height) {
    if (!win || !view) {
        return;
    }
    for (auto &bar : win->chrome) {
        if (bar.view == view) {
            bar.top = top;
            bar.height = height;
            gkWinLayout(win);
            return;
        }
    }
    GKChrome bar;
    bar.view = view;
    bar.top = top;
    bar.height = height;
    win->chrome.push_back(bar);
    gkReparent(view->outer, win->hwnd);
    ShowWindow(view->outer, SW_SHOWNA);
    gkWinLayout(win);
}

void gkWinRemoveChrome(GKWin *win, GKView *view) {
    if (!win) {
        return;
    }
    for (auto it = win->chrome.begin(); it != win->chrome.end(); ++it) {
        if (it->view == view) {
            win->chrome.erase(it);
            if (view->outer && IsWindow(view->outer)) {
                ShowWindow(view->outer, SW_HIDE);
                gkReparent(view->outer, gkParking());
            }
            gkWinLayout(win);
            return;
        }
    }
}

/* gkWinContentOrigin returns the screen position of the content area in
   physical pixels. */
static POINT gkWinContentOrigin(GKWin *win) {
    POINT origin = {0, 0};
    if (!win || !win->hwnd) {
        return origin;
    }
    if (win->root) {
        ClientToScreen(win->root, &origin);
    } else {
        ClientToScreen(win->hwnd, &origin);
    }
    return origin;
}

/* gkWinContentSize returns the size of the content area in logical
   units. */
static void gkWinContentSize(GKWin *win, double *width, double *height) {
    D2D1_SIZE_F size = win->root ? gkClientSize(win->root) : gkClientSize(win->hwnd);
    *width = size.width;
    *height = size.height;
}

/* gkWinPositionValues returns the position of the window frame in logical
   units, relative to the content of the owner for an owned window. */
static void gkWinPositionValues(GKWin *win, int *x, int *y) {
    if (!win || !win->hwnd) {
        *x = *y = 0;
        return;
    }
    if (!win->shown) {
        *x = win->x;
        *y = win->y;
        return;
    }
    RECT frame;
    GetWindowRect(win->hwnd, &frame);
    double scale = gkWinScale(win);
    double px = frame.left, py = frame.top;
    if (win->owner) {
        POINT origin = gkWinContentOrigin(win->owner);
        px -= origin.x;
        py -= origin.y;
    }
    *x = (int)lround(px / scale);
    *y = (int)lround(py / scale);
}

static void gkWinMoveFrame(GKWin *win, int x, int y) {
    if (!win || !win->hwnd) {
        return;
    }
    double scale = gkWinScale(win);
    double px = x * scale, py = y * scale;
    if (win->owner) {
        POINT origin = gkWinContentOrigin(win->owner);
        px += origin.x;
        py += origin.y;
    }
    SetWindowPos(win->hwnd, nullptr, (int)lround(px), (int)lround(py), 0, 0,
                 SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_NOOWNERZORDER);
}

/* gkWinOuterSize returns the size of the window frame for a content size in
   logical units. */
static SIZE gkWinOuterSize(GKWin *win, double width, double height) {
    double scale = gkWinScale(win);
    RECT rect = {0, 0, (LONG)lround(width * scale), (LONG)lround((height + gkWinChromeHeight(win)) * scale)};
    UINT dpi = GetDpiForWindow(win->hwnd);
    DWORD style = (DWORD)GetWindowLongPtrW(win->hwnd, GWL_STYLE);
    DWORD extended = (DWORD)GetWindowLongPtrW(win->hwnd, GWL_EXSTYLE);
    AdjustWindowRectExForDpi(&rect, style, win->menubar != nullptr, extended, dpi ? dpi : 96);
    SIZE size = {rect.right - rect.left, rect.bottom - rect.top};
    return size;
}

static void gkWinResizeClient(GKWin *win, int width, int height) {
    SIZE outer = gkWinOuterSize(win, width, height);
    SetWindowPos(win->hwnd, nullptr, 0, 0, outer.cx, outer.cy,
                 SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_NOOWNERZORDER);
    /* A menu bar can wrap to more lines at a narrow width, which changes the
       client area, so the size is checked once more. */
    double w, h;
    gkWinContentSize(win, &w, &h);
    if (lround(h) != height && !IsZoomed(win->hwnd) && !win->fullscreen) {
        RECT frame;
        GetWindowRect(win->hwnd, &frame);
        int extra = (int)lround((height - h) * gkWinScale(win));
        SetWindowPos(win->hwnd, nullptr, 0, 0, frame.right - frame.left, frame.bottom - frame.top + extra,
                     SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_NOOWNERZORDER);
    }
}

/* ---- Cursor ---- */

static void gkWinCenter(GKWin *win, double *x, double *y) {
    double width, height;
    gkWinContentSize(win, &width, &height);
    *x = floor(width / 2);
    *y = floor(height / 2);
}

static void gkWinWarpToCenter(GKWin *win) {
    double x, y;
    gkWinCenter(win, &x, &y);
    POINT origin = gkWinContentOrigin(win);
    double scale = gkWinScale(win);
    SetCursorPos(origin.x + (int)lround(x * scale), origin.y + (int)lround(y * scale));
    win->warping = 1;
}

static void gkWinClipCursor(GKWin *win, int clip) {
    if (!clip || !win->hwnd) {
        ClipCursor(nullptr);
        return;
    }
    RECT rect;
    GetClientRect(win->root ? win->root : win->hwnd, &rect);
    MapWindowPoints(win->root ? win->root : win->hwnd, nullptr, reinterpret_cast<POINT *>(&rect), 2);
    ClipCursor(&rect);
}

/* gkWinPointerHidden reports whether the pointer is hidden over a window. */
static bool gkWinPointerHidden(GKWin *win) {
    return win && win->cursorMode != GK_CURSOR_NORMAL && win->active;
}

/* ---- Input ---- */

static void gkWinReleaseButtons(GKWin *win) {
    if (!win->handle) {
        win->buttons = 0;
        return;
    }
    for (int button = 0; button < GK_POINTER_BUTTON_COUNT; button++) {
        if (win->buttons & (1u << button)) {
            gkWin32GoPointerButtonEvent(win->handle, button, GK_POINTER_RELEASED, gkModifiers());
        }
    }
    win->buttons = 0;
}

static int gkWinButton(UINT message, WPARAM wparam, int *pressed) {
    *pressed = message == WM_LBUTTONDOWN || message == WM_RBUTTONDOWN || message == WM_MBUTTONDOWN ||
               message == WM_XBUTTONDOWN || message == WM_LBUTTONDBLCLK || message == WM_RBUTTONDBLCLK ||
               message == WM_MBUTTONDBLCLK || message == WM_XBUTTONDBLCLK;
    switch (message) {
    case WM_LBUTTONDOWN:
    case WM_LBUTTONUP:
    case WM_LBUTTONDBLCLK:
        return GK_POINTER_BUTTON_LEFT;
    case WM_RBUTTONDOWN:
    case WM_RBUTTONUP:
    case WM_RBUTTONDBLCLK:
        return GK_POINTER_BUTTON_RIGHT;
    case WM_MBUTTONDOWN:
    case WM_MBUTTONUP:
    case WM_MBUTTONDBLCLK:
        return GK_POINTER_BUTTON_MIDDLE;
    case WM_XBUTTONDOWN:
    case WM_XBUTTONUP:
    case WM_XBUTTONDBLCLK:
        return GET_XBUTTON_WPARAM(wparam) == XBUTTON1 ? 3 : 4;
    default:
        return -1;
    }
}

/* gkWinPointerPoint returns the position of the pointer in the content of a
   window in logical units. */
static void gkWinPointerPoint(GKWin *win, double *x, double *y) {
    POINT cursor = {0, 0};
    GetCursorPos(&cursor);
    POINT origin = gkWinContentOrigin(win);
    double scale = gkWinScale(win);
    *x = (cursor.x - origin.x) / scale;
    *y = (cursor.y - origin.y) / scale;
}

static void gkWinPointerMoved(GKWin *win, HWND target) {
    double x, y;
    gkWinPointerPoint(win, &x, &y);
    if (!win->inside) {
        win->inside = 1;
        gkWinNotify(win, GK_EVENT_POINTER_ENTER, 0, x, y, nullptr);
    }
    TRACKMOUSEEVENT track = {sizeof(track), TME_LEAVE, target, 0};
    TrackMouseEvent(&track);
    double dx = win->haveLast ? x - win->lastX : 0;
    double dy = win->haveLast ? y - win->lastY : 0;
    if (win->cursorMode == GK_CURSOR_DISABLED && win->active) {
        double cx, cy;
        gkWinCenter(win, &cx, &cy);
        if (win->warping && fabs(x - cx) < 1 && fabs(y - cy) < 1) {
            win->warping = 0;
            return;
        }
        dx = x - cx;
        dy = y - cy;
        win->virtualX += dx;
        win->virtualY += dy;
        gkWinWarpToCenter(win);
        x = win->virtualX;
        y = win->virtualY;
    } else {
        win->lastX = (int)lround(x);
        win->lastY = (int)lround(y);
        win->haveLast = 1;
    }
    if (win->handle) {
        GKWin32Event report = {};
        report.kind = GK_EVENT_POINTER_MOVE;
        report.a = gkModifiers();
        report.x = x;
        report.y = y;
        report.z = dx;
        report.w = dy;
        gkSendEvent(win->handle, &report);
    }
}

/* gkWinLeft reports that the pointer left a window when the window that the
   pointer was over reports it left, unless the pointer is over another
   window of the same top-level window. */
static void gkWinLeft(GKWin *win) {
    POINT cursor;
    GetCursorPos(&cursor);
    HWND under = WindowFromPoint(cursor);
    if (under && GetAncestor(under, GA_ROOT) == win->hwnd) {
        return;
    }
    if (win->inside) {
        win->inside = 0;
        win->haveLast = 0;
        double x, y;
        gkWinPointerPoint(win, &x, &y);
        gkWinNotify(win, GK_EVENT_POINTER_LEAVE, 0, x, y, nullptr);
    }
}

/* gkWinTab moves the focus to the next or previous control. */
static bool gkWinTab(GKWin *win, HWND focus, int backward) {
    HWND next = GetNextDlgTabItem(win->hwnd, focus, backward);
    if (!next || next == focus) {
        return false;
    }
    SetFocus(next);
    if (GKView *view = gkViewFromDescendant(next)) {
        view->keyboardFocus = 1;
        gkInvalidate(view);
    }
    wchar_t name[32] = {};
    GetClassNameW(next, name, 32);
    if (!wcscmp(name, WC_EDITW)) {
        SendMessageW(next, EM_SETSEL, 0, -1);
    }
    return true;
}

/* gkWinInput observes a message before it is dispatched. The window gets
   the keyboard and pointer input of all its controls, as one stream, while
   the control under the input still receives it. It returns true when it
   consumed the message. */
bool gkDragTrack(MSG const *message);
bool gkAcceleratorPress(GKWin *win, MSG const *message);
bool gkMenuAccelerator(GKWin *win, MSG const *message);

bool gkWinInput(MSG const *message) {
    UINT kind = message->message;
    if (gkDragTrack(message)) {
        return true;
    }
    bool keyboard = kind >= WM_KEYFIRST && kind <= WM_KEYLAST;
    bool mouse = (kind >= WM_MOUSEFIRST && kind <= WM_MOUSELAST) || kind == WM_MOUSELEAVE;
    if (!keyboard && !mouse) {
        return false;
    }
    GKWin *win = gkWinOf(message->hwnd);
    if (!win || win->destroyed) {
        return false;
    }
    switch (kind) {
    case WM_KEYDOWN:
    case WM_SYSKEYDOWN:
    case WM_KEYUP:
    case WM_SYSKEYUP: {
        if (gkMenuAccelerator(win, message) || gkAcceleratorPress(win, message)) {
            return true;
        }
        int key = gkKeyFromVirtualKey(message->wParam, message->lParam);
        if (key >= 0 && key < GK_KEY_COUNT && win->handle) {
            int action = GK_KEY_PRESSED;
            if (kind == WM_KEYUP || kind == WM_SYSKEYUP) {
                action = GK_KEY_RELEASED;
            } else if (message->lParam & (1l << 30)) {
                action = GK_KEY_REPEAT;
            }
            gkWin32GoKeyEvent(win->handle, key, (int)((message->lParam >> 16) & 0xff), action, gkModifiers());
        }
        if (kind == WM_KEYDOWN && message->wParam == VK_TAB && !(GetKeyState(VK_CONTROL) & 0x8000)) {
            LRESULT code = SendMessageW(message->hwnd, WM_GETDLGCODE, message->wParam, (LPARAM)message);
            if (!(code & (DLGC_WANTTAB | DLGC_WANTALLKEYS))) {
                return gkWinTab(win, message->hwnd, (GetKeyState(VK_SHIFT) & 0x8000) != 0);
            }
        }
        return false;
    }
    case WM_CHAR:
    case WM_SYSCHAR: {
        unsigned int unit = (unsigned int)message->wParam;
        if (unit >= 0xd800 && unit <= 0xdbff) {
            win->highSurrogate = unit;
            return false;
        }
        unsigned int codepoint = unit;
        if (unit >= 0xdc00 && unit <= 0xdfff && win->highSurrogate) {
            codepoint = 0x10000 + ((win->highSurrogate - 0xd800) << 10) + (unit - 0xdc00);
        }
        win->highSurrogate = 0;
        if (codepoint >= 0x20 && codepoint != 0x7f && win->handle && !(GetKeyState(VK_CONTROL) & 0x8000)) {
            gkWin32GoCharEvent(win->handle, codepoint);
        }
        return false;
    }
    case WM_MOUSEMOVE:
        gkWinPointerMoved(win, message->hwnd);
        return false;
    case WM_MOUSELEAVE:
        gkWinLeft(win);
        return false;
    case WM_MOUSEWHEEL:
    case WM_MOUSEHWHEEL: {
        double delta = (double)GET_WHEEL_DELTA_WPARAM(message->wParam) / WHEEL_DELTA;
        double dx = kind == WM_MOUSEHWHEEL ? -delta : 0;
        double dy = kind == WM_MOUSEWHEEL ? delta : 0;
        win->scrollX += dx;
        win->scrollY += dy;
        if (win->handle) {
            gkWin32GoScrollEvent(win->handle, dx, dy);
        }
        /* Windows sends the wheel to the window with the keyboard focus. The
           window under the pointer gets it instead, as on macOS and Linux. */
        POINT point = {GET_X_LPARAM(message->lParam), GET_Y_LPARAM(message->lParam)};
        HWND under = WindowFromPoint(point);
        if (under && under != message->hwnd && GetWindowThreadProcessId(under, nullptr) == GetCurrentThreadId()) {
            SendMessageW(under, kind, message->wParam, message->lParam);
            return true;
        }
        return false;
    }
    default: {
        int pressed = 0;
        int button = gkWinButton(kind, message->wParam, &pressed);
        if (button < 0 || button >= GK_POINTER_BUTTON_COUNT) {
            return false;
        }
        unsigned int mask = 1u << button;
        if (pressed) {
            win->buttons |= mask;
            if (win->handle) {
                gkWin32GoPointerButtonEvent(win->handle, button, GK_POINTER_PRESSED, gkModifiers());
            }
        } else if (win->buttons & mask) {
            win->buttons &= ~mask;
            if (win->handle) {
                gkWin32GoPointerButtonEvent(win->handle, button, GK_POINTER_RELEASED, gkModifiers());
            }
        }
        return false;
    }
    }
}

/* ---- Application ---- */

static void gkWinUpdateApp() {
    int active = 0;
    for (GKWin *win : gkWindowList) {
        if (win->active) {
            active = 1;
        }
    }
    if (active != gkAppActive) {
        gkAppActive = active;
        gkSend(gkAppHandle, active ? GK_EVENT_APP_ACTIVE : GK_EVENT_APP_INACTIVE, 0, 0, 0, 0, nullptr);
    }
}

extern "C" void gkWin32AppAttach(uintptr_t handle) {
    gkAppHandle = handle;
}

/* ---- Window state ---- */

static void gkWinReportSize(GKWin *win) {
    double width, height;
    gkWinContentSize(win, &width, &height);
    int w = (int)lround(width), h = (int)lround(height);
    if (w == win->reportedWidth && h == win->reportedHeight) {
        return;
    }
    win->reportedWidth = w;
    win->reportedHeight = h;
    gkWinNotify(win, GK_EVENT_WINDOW_RESIZE, 0, w, h, nullptr);
}

static void gkWinStateChanged(GKWin *win) {
    int minimized = IsIconic(win->hwnd) != 0 || !IsWindowVisible(win->hwnd);
    if (minimized != win->minimized) {
        win->minimized = minimized;
        gkWinNotify(win, GK_EVENT_WINDOW_OCCLUSION, minimized, 0, 0, nullptr);
    }
}

static void gkWinDropFiles(GKWin *win, HDROP drop) {
    UINT count = DragQueryFileW(drop, 0xffffffff, nullptr, 0);
    std::string paths;
    for (UINT i = 0; i < count; i++) {
        UINT length = DragQueryFileW(drop, i, nullptr, 0);
        std::wstring path(length, L'\0');
        DragQueryFileW(drop, i, &path[0], length + 1);
        if (i) {
            paths += '\x1e';
        }
        paths += gkUTF8(path);
    }
    DragFinish(drop);
    if (count && win->acceptFiles) {
        gkWinNotify(win, GK_EVENT_WINDOW_FILE_DROP, (int)count, 0, 0, paths.c_str());
    }
}

static GKWin *gkWinFromHwnd(HWND hwnd) {
    return reinterpret_cast<GKWin *>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
}

static BOOL CALLBACK gkRefontChild(HWND child, LPARAM data) {
    (void)data;
    GKView *view = gkViewOf(child);
    if (view && (view->outer == child || view->control == child)) {
        gkApplyFont(view);
        if (view->ext) {
            view->ext->themeChanged(view);
        }
    }
    return TRUE;
}

/* ---- Dark menu bar ----

   Windows draws the menu bar in light colors in the dark theme. Like other
   Win32 applications with a dark theme, the window draws the bar and its
   items in dark colors through the messages that the menu bar sends to
   draw itself, which the Windows headers do not declare. */

static const UINT gkUAHDrawMenu = 0x0091;
static const UINT gkUAHDrawMenuItem = 0x0092;

struct GKUAHMenu {
    HMENU menu;
    HDC hdc;
    DWORD flags;
};

struct GKUAHMenuItem {
    int position;
    DWORD metrics[8];
    DWORD popupMetrics[5];
};

struct GKUAHDrawMenuItem {
    DRAWITEMSTRUCT draw;
    GKUAHMenu menu;
    GKUAHMenuItem item;
};

static COLORREF gkColorOf(GKColor const &color) {
    return RGB((int)lround(color.r * 255), (int)lround(color.g * 255), (int)lround(color.b * 255));
}

/* gkMenuBarRect returns the rectangle of the menu bar in window
   coordinates. */
static RECT gkMenuBarRect(HWND hwnd) {
    MENUBARINFO info = {sizeof(info)};
    RECT bar = {};
    if (!GetMenuBarInfo(hwnd, OBJID_MENU, 0, &info)) {
        return bar;
    }
    RECT window;
    GetWindowRect(hwnd, &window);
    bar = info.rcBar;
    OffsetRect(&bar, -window.left, -window.top);
    return bar;
}

/* gkDarkMenuBarLine paints over the light line that Windows draws below
   the menu bar. */
static void gkDarkMenuBarLine(HWND hwnd, GKPalette const &palette) {
    RECT bar = gkMenuBarRect(hwnd);
    if (bar.bottom <= bar.top) {
        return;
    }
    RECT line = {bar.left, bar.bottom, bar.right, bar.bottom + 1};
    HDC hdc = GetWindowDC(hwnd);
    HBRUSH brush = CreateSolidBrush(gkColorOf(palette.window));
    FillRect(hdc, &line, brush);
    DeleteObject(brush);
    ReleaseDC(hwnd, hdc);
}

static bool gkDarkMenuBar(GKWin *win, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) {
    if (!win || !win->menubar || gkWinTheme(win) != 2) {
        return false;
    }
    GKPalette const &palette = gkPalette(2);
    switch (message) {
    case gkUAHDrawMenu: {
        GKUAHMenu *menu = reinterpret_cast<GKUAHMenu *>(lparam);
        RECT bar = gkMenuBarRect(hwnd);
        HBRUSH brush = CreateSolidBrush(gkColorOf(palette.window));
        FillRect(menu->hdc, &bar, brush);
        DeleteObject(brush);
        *result = TRUE;
        return true;
    }
    case gkUAHDrawMenuItem: {
        GKUAHDrawMenuItem *item = reinterpret_cast<GKUAHDrawMenuItem *>(lparam);
        wchar_t text[256] = {};
        MENUITEMINFOW info = {sizeof(info)};
        info.fMask = MIIM_STRING;
        info.dwTypeData = text;
        info.cch = 255;
        GetMenuItemInfoW(item->menu.menu, item->item.position, TRUE, &info);
        UINT state = item->draw.itemState;
        GKColor fill = palette.window;
        if (state & (ODS_SELECTED | ODS_HOTLIGHT)) {
            fill = (state & ODS_SELECTED) ? palette.subtlePressed : palette.subtle;
        }
        HBRUSH brush = CreateSolidBrush(gkColorOf(fill));
        FillRect(item->menu.hdc, &item->draw.rcItem, brush);
        DeleteObject(brush);
        bool disabled = (state & (ODS_INACTIVE | ODS_DISABLED | ODS_GRAYED)) != 0;
        SetBkMode(item->menu.hdc, TRANSPARENT);
        SetTextColor(item->menu.hdc, gkColorOf(disabled ? palette.textDisabled : palette.text));
        UINT flags = DT_CENTER | DT_SINGLELINE | DT_VCENTER;
        if (state & ODS_NOACCEL) {
            flags |= DT_HIDEPREFIX;
        }
        DrawTextW(item->menu.hdc, text, -1, &item->draw.rcItem, flags);
        *result = TRUE;
        return true;
    }
    case WM_NCPAINT:
    case WM_NCACTIVATE:
        *result = DefWindowProcW(hwnd, message, wparam, lparam);
        gkDarkMenuBarLine(hwnd, palette);
        return true;
    default:
        return false;
    }
}

static LRESULT CALLBACK gkWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    GKWin *win = gkWinFromHwnd(hwnd);
    LRESULT result = 0;
    if (message != WM_DRAWITEM && gkForwardNotification(message, wparam, lparam, &result)) {
        return result;
    }
    if (message != WM_NCACTIVATE && gkDarkMenuBar(win, hwnd, message, wparam, lparam, &result)) {
        return result;
    }
    switch (message) {
    case WM_NCCREATE: {
        CREATESTRUCTW *create = reinterpret_cast<CREATESTRUCTW *>(lparam);
        win = static_cast<GKWin *>(create->lpCreateParams);
        win->hwnd = hwnd;
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(win));
        EnableNonClientDpiScaling(hwnd);
        break;
    }
    case WM_CLOSE:
        if (win) {
            if (win->handle) {
                GKWin32Event event = {};
                event.kind = GK_EVENT_WINDOW_CLOSE_REQUEST;
                if (!gkQueryEvent(win->handle, &event)) {
                    return 0;
                }
            }
            win->shouldClose = 1;
            if (win->utility) {
                ShowWindow(hwnd, SW_HIDE);
            }
        }
        return 0;
    case WM_ERASEBKGND: {
        RECT client;
        GetClientRect(hwnd, &client);
        GKColor color = gkPalette(gkWinTheme(win)).window;
        HBRUSH brush = CreateSolidBrush(RGB((int)(color.r * 255), (int)(color.g * 255), (int)(color.b * 255)));
        FillRect((HDC)wparam, &client, brush);
        DeleteObject(brush);
        return 1;
    }
    case WM_SIZE:
        if (win) {
            gkWinLayout(win);
            if (wparam != SIZE_MINIMIZED) {
                gkWinReportSize(win);
            }
            gkWinStateChanged(win);
        }
        break;
    case WM_SHOWWINDOW:
        if (win && !wparam) {
            gkLater([win]() {
                if (gkWinLive(win)) {
                    gkWinStateChanged(win);
                }
            });
        }
        break;
    case WM_GETMINMAXINFO:
        if (win && win->hwnd) {
            MINMAXINFO *info = reinterpret_cast<MINMAXINFO *>(lparam);
            if (win->minWidth > 0 || win->minHeight > 0) {
                SIZE size = gkWinOuterSize(win, win->minWidth, win->minHeight);
                info->ptMinTrackSize.x = win->minWidth > 0 ? size.cx : info->ptMinTrackSize.x;
                info->ptMinTrackSize.y = win->minHeight > 0 ? size.cy : info->ptMinTrackSize.y;
            }
            if ((win->maxWidth > 0 || win->maxHeight > 0) && !win->fullscreen) {
                SIZE size = gkWinOuterSize(win, win->maxWidth > 0 ? win->maxWidth : 100000,
                                           win->maxHeight > 0 ? win->maxHeight : 100000);
                if (win->maxWidth > 0) {
                    info->ptMaxTrackSize.x = size.cx;
                }
                if (win->maxHeight > 0) {
                    info->ptMaxTrackSize.y = size.cy;
                }
            }
            return 0;
        }
        break;
    case WM_ACTIVATE:
        if (win) {
            int active = LOWORD(wparam) != WA_INACTIVE;
            if (active != win->active) {
                win->active = active;
                if (!active) {
                    gkWinReleaseButtons(win);
                    if (win->cursorMode == GK_CURSOR_DISABLED) {
                        gkWinClipCursor(win, 0);
                    }
                } else if (win->cursorMode == GK_CURSOR_DISABLED) {
                    gkWinClipCursor(win, 1);
                }
                gkWinNotify(win, active ? GK_EVENT_WINDOW_FOCUS : GK_EVENT_WINDOW_BLUR, 0, 0, 0, nullptr);
                gkWinUpdateApp();
            }
        }
        break;
    case WM_NCACTIVATE:
        /* The window keeps the look of the active window while a popover
           that it owns is active. */
    {
        bool kept = !wparam && gkPopoverOwns(hwnd);
        if (kept) {
            wparam = TRUE;
        }
        if (gkDarkMenuBar(win, hwnd, message, wparam, lparam, &result)) {
            return result;
        }
        if (kept) {
            return DefWindowProcW(hwnd, message, wparam, lparam);
        }
        break;
    }
    case WM_SETFOCUS:
        if (win && win->focus && gkView(win->focus)) {
            SetFocus(gkViewTarget(win->focus));
            return 0;
        }
        break;
    case WM_SETCURSOR:
        if (win && gkWinPointerHidden(win) && LOWORD(lparam) == HTCLIENT) {
            SetCursor(nullptr);
            return TRUE;
        }
        break;
    case WM_DPICHANGED: {
        RECT *suggested = reinterpret_cast<RECT *>(lparam);
        SetWindowPos(hwnd, nullptr, suggested->left, suggested->top, suggested->right - suggested->left,
                     suggested->bottom - suggested->top, SWP_NOZORDER | SWP_NOACTIVATE);
        EnumChildWindows(hwnd, gkRefontChild, 0);
        if (win) {
            gkWinLayout(win);
            gkWinNotify(win, GK_EVENT_WINDOW_SCREEN, 0, 0, 0, nullptr);
        }
        return 0;
    }
    case WM_DISPLAYCHANGE:
        if (win) {
            gkWinNotify(win, GK_EVENT_WINDOW_SCREEN, 0, 0, 0, nullptr);
        }
        break;
    case WM_SETTINGCHANGE:
    case WM_THEMECHANGED:
        gkApplyThemes();
        break;
    case WM_DROPFILES:
        if (win) {
            gkWinDropFiles(win, (HDROP)wparam);
        }
        return 0;
    case WM_COMMAND:
        if (lparam == 0 && HIWORD(wparam) == 0 && gkMenuCommand(hwnd, LOWORD(wparam))) {
            return 0;
        }
        break;
    case WM_QUERYENDSESSION:
        if (gkAppHandle) {
            GKWin32Event event = {};
            event.kind = GK_EVENT_APP_QUIT_REQUEST;
            if (!gkQueryEvent(gkAppHandle, &event)) {
                ShutdownBlockReasonCreate(hwnd, L"The application has unsaved changes.");
                return FALSE;
            }
            ShutdownBlockReasonDestroy(hwnd);
        }
        return TRUE;
    case WM_CANCELMODE:
        if (win) {
            gkWinReleaseButtons(win);
        }
        break;
    case WM_NCDESTROY:
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
        break;
    default:
        break;
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

/* ---- Creation ---- */

static DWORD gkWinStyle(GKWin *win) {
    if (win->fullscreen || win->borderless) {
        return WS_POPUP | WS_CLIPCHILDREN | (win->borderless && !win->fullscreen ? WS_MINIMIZEBOX : 0);
    }
    DWORD style = WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_CLIPCHILDREN;
    if (!win->utility) {
        style |= WS_MINIMIZEBOX;
    }
    if (win->resizable) {
        style |= WS_THICKFRAME | (win->utility ? 0 : WS_MAXIMIZEBOX);
    }
    return style;
}

static DWORD gkWinExStyle(GKWin *win) {
    DWORD style = WS_EX_CONTROLPARENT;
    if (win->alwaysOnTop) {
        style |= WS_EX_TOPMOST;
    }
    if (win->utility) {
        style |= WS_EX_TOOLWINDOW;
    }
    if (win->transparent) {
        style |= WS_EX_LAYERED;
    }
    if (win->acceptFiles) {
        style |= WS_EX_ACCEPTFILES;
    }
    return style;
}

static void gkRegisterWindowClass() {
    static bool registered;
    if (registered) {
        return;
    }
    registered = true;
    WNDCLASSEXW cls = {};
    cls.cbSize = sizeof(cls);
    cls.style = CS_DBLCLKS;
    cls.lpfnWndProc = gkWindowProc;
    cls.hInstance = gkInstance();
    cls.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    cls.hIcon = LoadIconW(GetModuleHandleW(nullptr), MAKEINTRESOURCEW(1));
    if (!cls.hIcon) {
        cls.hIcon = LoadIconW(nullptr, IDI_APPLICATION);
    }
    cls.lpszClassName = gkWindowClass;
    RegisterClassExW(&cls);
}

extern "C" void gkWin32WindowSetVisible(void *pointer, int visible);
extern "C" int gkWin32WindowSetFullscreen(void *pointer, int fullscreen);

extern "C" void *gkWin32WindowCreate(const char *title, int width, int height, uint32_t flags, void *owner, int x,
                                     int y, char *error, size_t errorSize) {
    if (!gkInitialize(error, errorSize)) {
        return nullptr;
    }
    gkRegisterWindowClass();
    GKActivation activation;
    GKWin *win = new GKWin();
    win->owner = static_cast<GKWin *>(owner);
    win->utility = (flags & GK_WINDOW_UTILITY) != 0;
    win->resizable = (flags & GK_WINDOW_RESIZABLE) != 0;
    win->borderless = (flags & GK_WINDOW_BORDERLESS) != 0;
    win->alwaysOnTop = (flags & GK_WINDOW_ALWAYS_ON_TOP) != 0;
    win->transparent = (flags & GK_WINDOW_TRANSPARENT) != 0;
    win->x = x;
    win->y = y;
    win->havePosition = owner != nullptr;
    std::wstring wideTitle = gkWide(title);
    HWND hwnd = CreateWindowExW(gkWinExStyle(win), gkWindowClass, wideTitle.c_str(), gkWinStyle(win), CW_USEDEFAULT,
                                CW_USEDEFAULT, 400, 300, win->owner ? win->owner->hwnd : nullptr, nullptr, gkInstance(),
                                win);
    if (!hwnd) {
        gkSetError(error, errorSize, "ui/win32: Windows could not create a window");
        delete win;
        return nullptr;
    }
    if (win->transparent) {
        SetLayeredWindowAttributes(hwnd, 0, 255, LWA_ALPHA);
    }
    win->root = gkLayoutNew(hwnd);
    gkWindowList.push_back(win);
    win->resolvedTheme = gkResolvedTheme();
    gkWinApplyTheme(win);
    gkWinResizeClient(win, width > 1 ? width : 1, height > 1 ? height : 1);
    if (win->owner) {
        gkWinMoveFrame(win, x, y);
    }
    gkWinLayout(win);
    if (flags & GK_WINDOW_MAXIMIZED) {
        win->fullscreen = 0;
        ShowWindow(hwnd, (flags & GK_WINDOW_HIDDEN) ? SW_HIDE : SW_SHOWMAXIMIZED);
        if (!(flags & GK_WINDOW_HIDDEN)) {
            win->shown = 1;
        } else {
            WINDOWPLACEMENT placement = {sizeof(placement)};
            GetWindowPlacement(hwnd, &placement);
            placement.showCmd = SW_HIDE;
            placement.flags |= WPF_RESTORETOMAXIMIZED;
            SetWindowPlacement(hwnd, &placement);
        }
    }
    if (!(flags & GK_WINDOW_HIDDEN) && !win->shown) {
        gkWin32WindowSetVisible(win, 1);
    }
    return win;
}

extern "C" void gkWin32WindowDestroy(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!gkWinLive(win)) {
        return;
    }
    win->destroyed = 1;
    win->handle = 0;
    if (gkCaptureView && gkWinOf(gkCaptureView->outer) == win) {
        gkCaptureView = nullptr;
    }
    if (win->cursorMode == GK_CURSOR_DISABLED) {
        ClipCursor(nullptr);
    }
    gkWindowList.erase(std::find(gkWindowList.begin(), gkWindowList.end(), win));
    win->focus = nullptr;
    for (auto &bar : win->chrome) {
        if (bar.view && bar.view->outer && IsWindow(bar.view->outer)) {
            ShowWindow(bar.view->outer, SW_HIDE);
            SetParent(bar.view->outer, gkParking());
        }
    }
    win->chrome.clear();
    /* The views of the window belong to the Go layer, which releases them;
       they move to the parking window so destroying the window does not
       destroy their windows. */
    for (HWND child = GetWindow(win->root, GW_CHILD); child;) {
        HWND next = GetWindow(child, GW_HWNDNEXT);
        if (gkViewOf(child)) {
            ShowWindow(child, SW_HIDE);
            SetParent(child, gkParking());
        }
        child = next;
    }
    if (win->menubar) {
        SetMenu(win->hwnd, nullptr);
    }
    if (win->owner && IsWindowVisible(win->hwnd) && GetActiveWindow() == win->hwnd) {
        SetActiveWindow(win->owner->hwnd);
    }
    DestroyWindow(win->hwnd);
    delete win;
    gkWinUpdateApp();
    gkNotifyThemes();
}

/* ---- Polling ---- */

extern "C" void gkWin32WindowPoll(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    gkPumpOnce();
    if (gkWinLive(win) && win->shown) {
        gkWinStateChanged(win);
        if (win->handle && IsWindowVisible(win->hwnd)) {
            int x, y;
            gkWinPositionValues(win, &x, &y);
            if (!win->havePosition || x != win->x || y != win->y) {
                int moved = win->havePosition;
                win->x = x;
                win->y = y;
                win->havePosition = 1;
                if (moved) {
                    gkWin32GoWindowMove(win->handle, x, y);
                }
            }
        }
    }
    gkWin32AfterPoll();
}

extern "C" int gkWin32WindowShouldClose(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    return !win || win->shouldClose;
}

extern "C" void gkWin32WindowSetShouldClose(void *pointer, int close) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (win) {
        win->shouldClose = close != 0;
    }
}

/* ---- Geometry ---- */

extern "C" void gkWin32WindowSize(void *pointer, int *width, int *height) {
    GKWin *win = static_cast<GKWin *>(pointer);
    double w = 0, h = 0;
    if (win) {
        gkWinContentSize(win, &w, &h);
    }
    *width = (int)lround(w);
    *height = (int)lround(h);
}

extern "C" void gkWin32WindowFramebufferSize(void *pointer, int *width, int *height) {
    GKWin *win = static_cast<GKWin *>(pointer);
    RECT client = {};
    if (win && win->root) {
        GetClientRect(win->root, &client);
    }
    *width = client.right;
    *height = client.bottom;
}

extern "C" void gkWin32WindowSetSize(void *pointer, int width, int height) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return;
    }
    width = width > 1 ? width : 1;
    height = height > 1 ? height : 1;
    if (win->minWidth > 0 && width < win->minWidth) {
        width = win->minWidth;
    }
    if (win->minHeight > 0 && height < win->minHeight) {
        height = win->minHeight;
    }
    if (win->maxWidth > 0 && width > win->maxWidth) {
        width = win->maxWidth;
    }
    if (win->maxHeight > 0 && height > win->maxHeight) {
        height = win->maxHeight;
    }
    if (win->fullscreen) {
        win->windowedRect.right = win->windowedRect.left + (LONG)lround(width * gkWinScale(win));
        win->windowedRect.bottom = win->windowedRect.top + (LONG)lround(height * gkWinScale(win));
        return;
    }
    if (IsZoomed(win->hwnd)) {
        ShowWindow(win->hwnd, SW_RESTORE);
    }
    gkWinResizeClient(win, width, height);
}

extern "C" void gkWin32WindowPosition(void *pointer, int *x, int *y) {
    gkWinPositionValues(static_cast<GKWin *>(pointer), x, y);
}

extern "C" void gkWin32WindowSetPosition(void *pointer, int x, int y) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return;
    }
    int beforeX, beforeY;
    gkWinPositionValues(win, &beforeX, &beforeY);
    win->havePosition = 1;
    gkWinMoveFrame(win, x, y);
    if (!win->shown) {
        win->x = x;
        win->y = y;
        return;
    }
    int afterX, afterY;
    gkWinPositionValues(win, &afterX, &afterY);
    win->x = afterX;
    win->y = afterY;
    if ((afterX != beforeX || afterY != beforeY) && win->handle) {
        gkWin32GoWindowMove(win->handle, afterX, afterY);
    }
}

/* ---- Visibility ---- */

extern "C" int gkWin32WindowVisible(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    return win && win->hwnd && IsWindowVisible(win->hwnd);
}

extern "C" void gkWin32WindowSetVisible(void *pointer, int visible) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win || !win->hwnd) {
        return;
    }
    if (visible) {
        int first = !win->shown;
        win->shown = 1;
        if (first && win->owner) {
            gkWinMoveFrame(win, win->x, win->y);
        }
        WINDOWPLACEMENT placement = {sizeof(placement)};
        GetWindowPlacement(win->hwnd, &placement);
        int command = (placement.flags & WPF_RESTORETOMAXIMIZED) && !IsWindowVisible(win->hwnd) ? SW_SHOWMAXIMIZED
                      : win->utility                                                            ? SW_SHOWNOACTIVATE
                                                                                                : SW_SHOW;
        if (IsIconic(win->hwnd)) {
            command = SW_RESTORE;
        }
        ShowWindow(win->hwnd, command);
        if (!win->utility) {
            SetForegroundWindow(win->hwnd);
        }
        if (win->pendingFullscreen) {
            win->pendingFullscreen = 0;
            gkWin32WindowSetFullscreen(win, 1);
        }
        UpdateWindow(win->hwnd);
        gkWinLayout(win);
        gkWinReportSize(win);
    } else {
        ShowWindow(win->hwnd, SW_HIDE);
    }
    gkWinStateChanged(win);
}

extern "C" void gkWin32WindowBringToFront(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win || !IsWindowVisible(win->hwnd)) {
        return;
    }
    SetWindowPos(win->hwnd, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE);
    SetForegroundWindow(win->hwnd);
}

extern "C" void gkWin32WindowSetTitle(void *pointer, const char *title) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (win && win->hwnd) {
        SetWindowTextW(win->hwnd, gkWide(title).c_str());
    }
}

extern "C" void gkWin32WindowSetTheme(void *pointer, int theme) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return;
    }
    win->theme = theme;
    gkNotifyThemes();
}

/* ---- Native handles ---- */

extern "C" uintptr_t gkWin32WindowNativeHandle(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    return win ? reinterpret_cast<uintptr_t>(win->hwnd) : 0;
}

extern "C" uintptr_t gkWin32WindowNativeDisplayHandle(void *pointer) {
    (void)pointer;
    return reinterpret_cast<uintptr_t>(GetModuleHandleW(nullptr));
}

extern "C" void *gkWin32WindowNativePointer(void *pointer) {
    return pointer;
}

extern "C" void *gkWin32WindowNativeDisplay(void *pointer) {
    (void)pointer;
    return GetModuleHandleW(nullptr);
}

/* ---- Pointer ---- */

extern "C" void gkWin32WindowCursorPosition(void *pointer, double *x, double *y) {
    GKWin *win = static_cast<GKWin *>(pointer);
    double px = 0, py = 0;
    if (win && win->cursorMode == GK_CURSOR_DISABLED) {
        px = win->virtualX;
        py = win->virtualY;
    } else if (win && win->hwnd) {
        gkWinPointerPoint(win, &px, &py);
    }
    if (x) {
        *x = px;
    }
    if (y) {
        *y = py;
    }
}

extern "C" void gkWin32WindowSetHandle(void *pointer, uintptr_t handle) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (win) {
        win->handle = handle;
    }
}

extern "C" void gkWin32WindowGetScroll(void *pointer, double *x, double *y) {
    GKWin *win = static_cast<GKWin *>(pointer);
    *x = win ? win->scrollX : 0;
    *y = win ? win->scrollY : 0;
}

static void gkWinApplyCursorMode(GKWin *win, int previous) {
    if (previous == GK_CURSOR_DISABLED) {
        gkWinClipCursor(win, 0);
    }
    POINT cursor;
    GetCursorPos(&cursor);
    SetCursorPos(cursor.x, cursor.y);
    if (win->cursorMode == GK_CURSOR_DISABLED && IsWindowVisible(win->hwnd)) {
        gkWinClipCursor(win, win->active);
        gkWinWarpToCenter(win);
    }
}

static DWORD gkUIThreadId() {
    static DWORD thread = GetCurrentThreadId();
    return thread;
}

extern "C" void gkWin32WindowSetCursorMode(void *pointer, int mode) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win || win->cursorMode == mode) {
        return;
    }
    int previous = win->cursorMode;
    if (mode == GK_CURSOR_DISABLED) {
        double x = 0, y = 0;
        gkWin32WindowCursorPosition(win, &x, &y);
        win->virtualX = x;
        win->virtualY = y;
    }
    win->cursorMode = mode;
    if (GetCurrentThreadId() == gkUIThreadId()) {
        gkWinApplyCursorMode(win, previous);
    } else {
        gkLater([win, previous]() {
            if (gkWinLive(win)) {
                gkWinApplyCursorMode(win, previous);
            }
        });
    }
}

extern "C" int gkWin32WindowGetCursorMode(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    return win ? win->cursorMode : GK_CURSOR_NORMAL;
}

/* ---- Extended window contract ---- */

extern "C" void gkWin32WindowAttach(void *pointer, uintptr_t handle) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (win) {
        win->handle = handle;
    }
}

extern "C" void gkWin32WindowDetach(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (win) {
        win->handle = 0;
    }
}

extern "C" void gkWin32WindowNotify(void *window, int kind, int a, double x, double y, const char *text) {
    gkWinNotify(static_cast<GKWin *>(window), kind, a, x, y, text);
}

extern "C" int gkWin32WindowAllowClose(void *window) {
    GKWin *win = static_cast<GKWin *>(window);
    if (!win || !win->handle) {
        return 1;
    }
    GKWin32Event event = {};
    event.kind = GK_EVENT_WINDOW_CLOSE_REQUEST;
    return gkQueryEvent(win->handle, &event);
}

extern "C" void gkWin32WindowDidResize(void *window) {
    (void)window;
}

extern "C" void gkWin32WindowDidChangeKey(void *window, int key) {
    gkWinNotify(static_cast<GKWin *>(window), key ? GK_EVENT_WINDOW_FOCUS : GK_EVENT_WINDOW_BLUR, 0, 0, 0, nullptr);
}

extern "C" void gkWin32WindowAcceptFiles(void *window, int accept) {
    GKWin *win = static_cast<GKWin *>(window);
    if (win && win->hwnd) {
        win->acceptFiles = accept != 0;
        DragAcceptFiles(win->hwnd, win->acceptFiles ? TRUE : FALSE);
    }
}

extern "C" void gkWin32WindowDidShow(void *window) {
    (void)window;
}

extern "C" int gkWin32WindowCanBecomeKey(void *window) {
    GKWin *win = static_cast<GKWin *>(window);
    return win && win->hwnd && IsWindowEnabled(win->hwnd);
}

extern "C" void gkWin32WindowFullscreenPhase(void *window, int phase) {
    (void)window;
    (void)phase;
}

extern "C" void gkWin32WindowSetSizeLimits(void *pointer, int minWidth, int minHeight, int maxWidth, int maxHeight) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return;
    }
    win->minWidth = minWidth > 0 ? minWidth : 0;
    win->minHeight = minHeight > 0 ? minHeight : 0;
    win->maxWidth = maxWidth > 0 ? maxWidth : 0;
    win->maxHeight = maxHeight > 0 ? maxHeight : 0;
    int width, height;
    gkWin32WindowSize(win, &width, &height);
    int clampedWidth = width, clampedHeight = height;
    if (win->minWidth > 0 && clampedWidth < win->minWidth) {
        clampedWidth = win->minWidth;
    }
    if (win->minHeight > 0 && clampedHeight < win->minHeight) {
        clampedHeight = win->minHeight;
    }
    if (win->maxWidth > 0 && clampedWidth > win->maxWidth) {
        clampedWidth = win->maxWidth;
    }
    if (win->maxHeight > 0 && clampedHeight > win->maxHeight) {
        clampedHeight = win->maxHeight;
    }
    if (clampedWidth != width || clampedHeight != height) {
        gkWin32WindowSetSize(win, clampedWidth, clampedHeight);
    }
}

/* gkWinRestyle applies the style of the window after a setting changes,
   keeping its content size. */
static void gkWinRestyle(GKWin *win) {
    int width, height;
    gkWin32WindowSize(win, &width, &height);
    LONG_PTR visible = GetWindowLongPtrW(win->hwnd, GWL_STYLE) & WS_VISIBLE;
    SetWindowLongPtrW(win->hwnd, GWL_STYLE, (LONG_PTR)gkWinStyle(win) | visible);
    SetWindowLongPtrW(win->hwnd, GWL_EXSTYLE, (LONG_PTR)gkWinExStyle(win));
    SetWindowPos(win->hwnd, win->alwaysOnTop ? HWND_TOPMOST : HWND_NOTOPMOST, 0, 0, 0, 0,
                 SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_FRAMECHANGED);
    if (!win->fullscreen && !IsZoomed(win->hwnd)) {
        gkWinResizeClient(win, width, height);
    }
}

extern "C" void gkWin32WindowSetStyle(void *pointer, int resizable, int borderless) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return;
    }
    win->resizable = resizable != 0;
    win->borderless = borderless != 0;
    gkWinRestyle(win);
}

extern "C" void gkWin32WindowSetAlwaysOnTop(void *pointer, int alwaysOnTop) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return;
    }
    win->alwaysOnTop = alwaysOnTop != 0;
    SetWindowPos(win->hwnd, win->alwaysOnTop ? HWND_TOPMOST : HWND_NOTOPMOST, 0, 0, 0, 0,
                 SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
}

extern "C" int gkWin32WindowIsMinimized(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    return win && win->shown && IsIconic(win->hwnd);
}

extern "C" int gkWin32WindowMinimize(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return 0;
    }
    if (IsIconic(win->hwnd)) {
        return 1;
    }
    if (win->fullscreen) {
        return 0;
    }
    win->shown = 1;
    ShowWindow(win->hwnd, SW_MINIMIZE);
    gkWinStateChanged(win);
    return IsIconic(win->hwnd) != 0;
}

extern "C" int gkWin32WindowIsMaximized(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win || win->fullscreen) {
        return 0;
    }
    if (!IsWindowVisible(win->hwnd)) {
        WINDOWPLACEMENT placement = {sizeof(placement)};
        GetWindowPlacement(win->hwnd, &placement);
        return (placement.flags & WPF_RESTORETOMAXIMIZED) != 0;
    }
    return IsZoomed(win->hwnd) != 0;
}

extern "C" int gkWin32WindowMaximize(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win || win->fullscreen) {
        return 0;
    }
    if (!win->shown || !IsWindowVisible(win->hwnd)) {
        WINDOWPLACEMENT placement = {sizeof(placement)};
        GetWindowPlacement(win->hwnd, &placement);
        placement.flags |= WPF_RESTORETOMAXIMIZED;
        placement.showCmd = IsWindowVisible(win->hwnd) ? SW_SHOWMAXIMIZED : SW_HIDE;
        SetWindowPlacement(win->hwnd, &placement);
        return 1;
    }
    ShowWindow(win->hwnd, SW_MAXIMIZE);
    gkWinStateChanged(win);
    return 1;
}

extern "C" int gkWin32WindowSetFullscreen(void *pointer, int fullscreen);

extern "C" int gkWin32WindowRestore(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win) {
        return 0;
    }
    if (win->fullscreen) {
        gkWin32WindowSetFullscreen(win, 0);
    }
    WINDOWPLACEMENT placement = {sizeof(placement)};
    GetWindowPlacement(win->hwnd, &placement);
    if (!IsWindowVisible(win->hwnd) && (placement.flags & WPF_RESTORETOMAXIMIZED)) {
        placement.flags &= ~WPF_RESTORETOMAXIMIZED;
        SetWindowPlacement(win->hwnd, &placement);
    }
    if (IsIconic(win->hwnd) || IsZoomed(win->hwnd)) {
        ShowWindow(win->hwnd, SW_RESTORE);
        if (IsZoomed(win->hwnd)) {
            ShowWindow(win->hwnd, SW_RESTORE);
        }
    }
    gkWinStateChanged(win);
    return !IsIconic(win->hwnd);
}

extern "C" int gkWin32WindowIsFullscreen(void *pointer) {
    GKWin *win = static_cast<GKWin *>(pointer);
    return win && win->fullscreen;
}

extern "C" int gkWin32WindowSetFullscreen(void *pointer, int fullscreen) {
    GKWin *win = static_cast<GKWin *>(pointer);
    if (!win || !win->hwnd) {
        return 0;
    }
    fullscreen = fullscreen != 0;
    if (!win->shown && !IsWindowVisible(win->hwnd)) {
        win->pendingFullscreen = fullscreen && !win->fullscreen;
        if (!fullscreen && win->fullscreen) {
            win->fullscreen = 0;
        } else {
            return 1;
        }
    }
    if (fullscreen == win->fullscreen) {
        return 1;
    }
    if (fullscreen) {
        if (IsZoomed(win->hwnd)) {
            ShowWindow(win->hwnd, SW_RESTORE);
        }
        GetWindowRect(win->hwnd, &win->windowedRect);
        win->fullscreen = 1;
        HMONITOR monitor = MonitorFromWindow(win->hwnd, MONITOR_DEFAULTTONEAREST);
        MONITORINFO info = {sizeof(info)};
        GetMonitorInfoW(monitor, &info);
        LONG_PTR visible = GetWindowLongPtrW(win->hwnd, GWL_STYLE) & WS_VISIBLE;
        SetWindowLongPtrW(win->hwnd, GWL_STYLE, (LONG_PTR)gkWinStyle(win) | visible);
        if (win->menubar) {
            SetMenu(win->hwnd, nullptr);
        }
        SetWindowPos(win->hwnd, HWND_TOP, info.rcMonitor.left, info.rcMonitor.top,
                     info.rcMonitor.right - info.rcMonitor.left, info.rcMonitor.bottom - info.rcMonitor.top,
                     SWP_FRAMECHANGED | SWP_NOOWNERZORDER);
        if (win->shown && !IsWindowVisible(win->hwnd)) {
            ShowWindow(win->hwnd, SW_SHOW);
        }
    } else {
        win->fullscreen = 0;
        LONG_PTR visible = GetWindowLongPtrW(win->hwnd, GWL_STYLE) & WS_VISIBLE;
        SetWindowLongPtrW(win->hwnd, GWL_STYLE, (LONG_PTR)gkWinStyle(win) | visible);
        if (win->menubar) {
            SetMenu(win->hwnd, win->menubar);
        }
        RECT rect = win->windowedRect;
        SetWindowPos(win->hwnd, win->alwaysOnTop ? HWND_TOPMOST : HWND_NOTOPMOST, rect.left, rect.top,
                     rect.right - rect.left, rect.bottom - rect.top, SWP_FRAMECHANGED | SWP_NOOWNERZORDER);
    }
    gkWinLayout(win);
    gkWinReportSize(win);
    gkWinNotify(win, GK_EVENT_WINDOW_FULLSCREEN, fullscreen, 0, 0, nullptr);
    return 1;
}

/* ---- Screens ---- */

extern "C" void gkWin32WindowScreen(void *pointer, double *values, char **name) {
    GKWin *win = static_cast<GKWin *>(pointer);
    HMONITOR monitor = win && win->hwnd ? MonitorFromWindow(win->hwnd, MONITOR_DEFAULTTONEAREST)
                                        : MonitorFromPoint(POINT{0, 0}, MONITOR_DEFAULTTOPRIMARY);
    MONITORINFOEXW info = {};
    info.cbSize = sizeof(info);
    GetMonitorInfoW(monitor, &info);
    UINT dpiX = 96, dpiY = 96;
    typedef HRESULT(WINAPI * GKGetDpiForMonitor)(HMONITOR, int, UINT *, UINT *);
    HMODULE shcore = LoadLibraryW(L"shcore.dll");
    auto getDpi = shcore ? reinterpret_cast<GKGetDpiForMonitor>(
                               reinterpret_cast<void *>(GetProcAddress(shcore, "GetDpiForMonitor")))
                         : nullptr;
    if (getDpi) {
        getDpi(monitor, 0, &dpiX, &dpiY);
    }
    double scale = dpiX / 96.0;
    DEVMODEW mode = {};
    mode.dmSize = sizeof(mode);
    double refresh = 60;
    if (EnumDisplaySettingsW(info.szDevice, ENUM_CURRENT_SETTINGS, &mode) && mode.dmDisplayFrequency > 1) {
        refresh = mode.dmDisplayFrequency;
    }
    RECT bounds = info.rcMonitor, work = info.rcWork;
    values[0] = bounds.left / scale;
    values[1] = bounds.top / scale;
    values[2] = (bounds.right - bounds.left) / scale;
    values[3] = (bounds.bottom - bounds.top) / scale;
    values[4] = work.left / scale;
    values[5] = work.top / scale;
    values[6] = (work.right - work.left) / scale;
    values[7] = (work.bottom - work.top) / scale;
    values[8] = scale > 0 ? scale : 1;
    values[9] = dpiX;
    values[10] = refresh;
    std::string label = gkUTF8(info.szDevice);
    DISPLAY_DEVICEW device = {};
    device.cb = sizeof(device);
    if (EnumDisplayDevicesW(info.szDevice, 0, &device, 0) && device.DeviceString[0]) {
        label = gkUTF8(device.DeviceString);
    }
    *name = gkStrdup(label.c_str());
}

extern "C" int gkWin32WindowResolvedTheme(void *pointer) {
    return gkWinTheme(static_cast<GKWin *>(pointer));
}

extern "C" void gkWin32SystemPreferences(void *pointer, double *values, char **locale) {
    values[0] = gkWin32WindowResolvedTheme(pointer);
    DWORD textScale = 100, size = sizeof(textScale);
    if (RegGetValueW(HKEY_CURRENT_USER, L"Software\\Microsoft\\Accessibility", L"TextScaleFactor",
                     RRF_RT_REG_DWORD, nullptr, &textScale, &size) != ERROR_SUCCESS) {
        textScale = 100;
    }
    BOOL animations = TRUE;
    SystemParametersInfoW(SPI_GETCLIENTAREAANIMATION, 0, &animations, 0);
    DWORD transparency = 1;
    size = sizeof(transparency);
    if (RegGetValueW(HKEY_CURRENT_USER, L"Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize",
                     L"EnableTransparency", RRF_RT_REG_DWORD, nullptr, &transparency, &size) != ERROR_SUCCESS) {
        transparency = 1;
    }
    HIGHCONTRASTW contrast = {};
    contrast.cbSize = sizeof(contrast);
    int high = SystemParametersInfoW(SPI_GETHIGHCONTRAST, sizeof(contrast), &contrast, 0) &&
               (contrast.dwFlags & HCF_HIGHCONTRASTON);
    values[1] = textScale > 0 ? textScale / 100.0 : 1;
    values[2] = high;
    values[3] = !animations;
    values[4] = !transparency;
    GKColor accent = gkAccentColor();
    values[5] = accent.r;
    values[6] = accent.g;
    values[7] = accent.b;
    values[8] = 1;
    wchar_t name[LOCALE_NAME_MAX_LENGTH] = {};
    if (!GetUserDefaultLocaleName(name, LOCALE_NAME_MAX_LENGTH) || !name[0]) {
        wcscpy(name, L"en-US");
    }
    DWORD layout = 0;
    values[9] = GetLocaleInfoEx(name, LOCALE_IREADINGLAYOUT | LOCALE_RETURN_NUMBER, reinterpret_cast<LPWSTR>(&layout),
                                sizeof(layout) / sizeof(wchar_t)) &&
                layout == 1;
    *locale = gkStrdup(gkUTF8(name).c_str());
}

extern "C" int gkWin32WindowColorSpace(void *window) {
    (void)window;
    return 1;
}

/* ---- Focus tracking ---- */

extern "C" void gkWin32TrackFocus(void *window) {
    GKWin *win = static_cast<GKWin *>(window);
    if (!gkWinLive(win) || win->destroyed) {
        return;
    }
    HWND focus = GetFocus();
    GKView *view = nullptr;
    if (focus && GetAncestor(focus, GA_ROOT) == win->hwnd) {
        view = gkViewFromDescendant(focus);
    } else if (!win->active) {
        /* An inactive window keeps the view that has its focus. */
        return;
    }
    if (view == win->focus) {
        return;
    }
    GKView *previous = win->focus;
    win->focus = view;
    if (previous && gkView(previous)) {
        gkViewSend(previous, GK_EVENT_BLUR, 0, 0, 0, 0, nullptr);
    }
    if (view) {
        gkViewSend(view, GK_EVENT_FOCUS, 0, 0, 0, 0, nullptr);
    }
}

static void gkTrackFocusAll() {
    auto windows = gkWindowList;
    for (GKWin *win : windows) {
        gkWin32TrackFocus(win);
    }
}

extern "C" void gkWin32AfterPoll(void) {
    gkTrackFocusAll();
}

extern "C" void gkWin32ForgetFocus(void *window, uintptr_t handle) {
    GKWin *win = static_cast<GKWin *>(window);
    if (!gkWinLive(win)) {
        return;
    }
    GKView *view = gkView(win->focus);
    if (!view || view->handle == handle || view->handle == 0) {
        win->focus = nullptr;
    }
}

/* ---- Canvas capture and scale ---- */

extern "C" int gkWin32CanvasCapture(void *pointer, int capture) {
    GKView *view = gkView(pointer);
    if (capture) {
        if (!view || !gkWinOf(view->outer)) {
            return 0;
        }
        gkCaptureView = view;
        SetCapture(view->outer);
    } else if (gkCaptureView == view) {
        gkCaptureView = nullptr;
        if (view && GetCapture() == view->outer) {
            ReleaseCapture();
        }
    }
    return 1;
}

extern "C" int gkWin32CanvasHasCapture(void *pointer) {
    return pointer && gkCaptureView == pointer;
}

extern "C" void *gkWin32CaptureTarget(void *event) {
    (void)event;
    return gkCaptureView;
}

extern "C" void gkWin32DeliverCaptured(void *view, void *event) {
    (void)view;
    (void)event;
}

extern "C" void gkWin32ReleaseCaptureFor(void *view) {
    if (gkCaptureView == view) {
        gkCaptureView = nullptr;
    }
}

extern "C" double gkWin32ViewScale(void *pointer) {
    GKView *view = gkView(pointer);
    if (view && view->outer) {
        return gkScaleOf(view->outer);
    }
    return GetDpiForSystem() / 96.0;
}

extern "C" int gkWin32ViewColorSpace(void *view) {
    (void)view;
    return 1;
}
