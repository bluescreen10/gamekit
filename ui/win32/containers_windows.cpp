//go:build windows && cgo

#include "win32_internal.h"

#include <algorithm>

extern "C" void gkWin32AddChild(void *container, void *child) {
    GKView *parent = gkView(container);
    GKView *view = gkView(child);
    if (!parent || !view) {
        return;
    }
    HWND target = gkContainerOf(parent);
    if (target) {
        gkReparent(view->outer, target);
        if (view->visible) {
            ShowWindow(view->outer, SW_SHOWNA);
        }
    }
}

extern "C" void *gkWin32ClipViewCreate(uintptr_t window) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_CLIP, window, 0);
    view->body = host;
    return view;
}

/* ---- Group box ----

   A group box is the group box of Windows: a frame with its title. The
   content and the subtitle are inside the frame. */

static const int gkGroupInset = 9;

struct GKGroupBox : GKExtension {
    HWND frame = nullptr;
    HWND subtitle = nullptr;
    std::wstring subtitleText;
    int titleHeight(GKView *view) {
        return gkMeasureGDI(L"Ag", gkViewFont(view, gkBodyFont())).cy;
    }
    int subtitleHeight(GKView *view, int width) {
        if (subtitleText.empty()) {
            return 0;
        }
        GKFont font = gkTypeRamp(1);
        return gkMeasureGDI(subtitleText, font, width > 0 ? width : 1).cy + 6;
    }
    void layout(GKView *view) override {
        gkPlaceWindow(frame, 0, 0, view->width, view->height);
        int top = titleHeight(view) + 4;
        double width = view->width - 2 * gkGroupInset, height = view->height - top - gkGroupInset;
        gkPlaceWindow(view->body, gkGroupInset, top, width > 0 ? width : 0, height > 0 ? height : 0);
        int sub = subtitleHeight(view, (int)(width > 0 ? width : 0));
        if (subtitle) {
            gkPlaceWindow(subtitle, 0, 0, width > 0 ? width : 0, sub > 6 ? sub - 6 : 0);
            ShowWindow(subtitle, subtitleText.empty() ? SW_HIDE : SW_SHOWNA);
        }
    }
    /* paintDark draws the frame in the dark theme, which the visual styles
       of Windows do not have for group boxes: a line in the color of
       control borders, and the title over it in the text color. */
    void paintDark(GKView *view) {
        PAINTSTRUCT paint;
        HDC hdc = BeginPaint(frame, &paint);
        RECT client;
        GetClientRect(frame, &client);
        GKPalette const &palette = gkViewPalette(view);
        auto ref = [](GKColor const &color) {
            return RGB((int)lround(color.r * 255), (int)lround(color.g * 255), (int)lround(color.b * 255));
        };
        HBRUSH background = CreateSolidBrush(ref(gkBackgroundOf(frame)));
        FillRect(hdc, &client, background);
        HFONT font = (HFONT)SendMessageW(frame, WM_GETFONT, 0, 0);
        HGDIOBJ previous = SelectObject(hdc, font);
        TEXTMETRICW metrics = {};
        GetTextMetricsW(hdc, &metrics);
        int middle = metrics.tmHeight / 2;
        HBRUSH line = CreateSolidBrush(ref(palette.controlStroke));
        RECT box = {client.left, client.top + middle, client.right, client.bottom};
        FrameRect(hdc, &box, line);
        if (!view->text.empty()) {
            int pad = (int)lround(4 * gkScaleOf(frame));
            SIZE size = {};
            GetTextExtentPoint32W(hdc, view->text.c_str(), (int)view->text.size(), &size);
            RECT title = {2 * pad, 0, std::min<LONG>(2 * pad + size.cx + 2 * pad, client.right), metrics.tmHeight};
            FillRect(hdc, &title, background);
            SetBkMode(hdc, TRANSPARENT);
            SetTextColor(hdc, ref(palette.text));
            RECT text = {title.left + pad, 0, title.right - pad, title.bottom};
            DrawTextW(hdc, view->text.c_str(), (int)view->text.size(), &text, DT_LEFT | DT_TOP | DT_SINGLELINE |
                                                                                     DT_NOPREFIX | DT_END_ELLIPSIS);
        }
        DeleteObject(line);
        DeleteObject(background);
        SelectObject(hdc, previous);
        EndPaint(frame, &paint);
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)lparam;
        /* A group box draws only its line and its title. Its parent does not
           paint under it, so it fills its background, or a moved group box
           keeps what was under it before. */
        if (hwnd == frame && message == WM_PAINT && gkViewPalette(view).dark) {
            paintDark(view);
            *result = 0;
            return true;
        }
        if (hwnd == frame && message == WM_ERASEBKGND) {
            RECT client;
            GetClientRect(frame, &client);
            GKColor color = gkBackgroundOf(frame);
            HBRUSH brush = CreateSolidBrush(
                RGB((int)lround(color.r * 255), (int)lround(color.g * 255), (int)lround(color.b * 255)));
            FillRect((HDC)wparam, &client, brush);
            DeleteObject(brush);
            *result = 1;
            return true;
        }
        return false;
    }
    void themeChanged(GKView *view) override {
        SendMessageW(frame, WM_SETFONT, (WPARAM)gkFontHandle(gkViewFont(view, gkBodyFont()), GetDpiForWindow(frame)),
                     TRUE);
        if (subtitle) {
            SendMessageW(subtitle, WM_SETFONT, (WPARAM)gkFontHandle(gkTypeRamp(1), GetDpiForWindow(subtitle)), TRUE);
        }
    }
};

static GKGroupBox *gkGroupOf(GKView *view) {
    return view && view->kind == GK_KIND_GROUP_BOX ? static_cast<GKGroupBox *>(view->ext.get()) : nullptr;
}

extern "C" void *gkWin32GroupBoxCreate(uintptr_t window, const char *title, const char *subtitle) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_GROUP_BOX, window, 0);
    GKGroupBox *box = new GKGroupBox();
    view->ext.reset(box);
    view->text = gkWide(title);
    box->frame = gkMakeNative(WC_BUTTONW, view->text.c_str(), BS_GROUPBOX | WS_CLIPSIBLINGS, 0, host);
    view->body = gkLayoutNew(host);
    /* A child window that is created first is at the top of the z-order,
       and the body clips the siblings above it, so the frame goes below
       the body. */
    SetWindowPos(box->frame, HWND_BOTTOM, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    SetPropW(box->frame, L"GKView", view);
    gkSubclass(box->frame);
    box->subtitleText = gkWide(subtitle);
    box->subtitle = gkMakeNative(WC_STATICW, box->subtitleText.c_str(), SS_LEFT | SS_NOPREFIX, 0, view->body);
    gkApplyThemeTo(box->frame, gkViewTheme(view));
    box->themeChanged(view);
    return view;
}

extern "C" void gkWin32GroupBoxSet(void *pointer, const char *title, const char *subtitle) {
    GKView *view = gkView(pointer);
    GKGroupBox *box = gkGroupOf(view);
    if (!box) {
        return;
    }
    view->text = gkWide(title);
    SetWindowTextW(box->frame, view->text.c_str());
    box->subtitleText = gkWide(subtitle);
    SetWindowTextW(box->subtitle, box->subtitleText.c_str());
    box->layout(view);
}

extern "C" void gkWin32GroupBoxChrome(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    GKGroupBox *box = gkGroupOf(view);
    *width = *height = 0;
    if (box) {
        *width = 2 * gkGroupInset;
        *height = box->titleHeight(view) + 4 + gkGroupInset;
    }
}

extern "C" int gkWin32GroupBoxSubtitleHeight(void *pointer, int width) {
    GKView *view = gkView(pointer);
    GKGroupBox *box = gkGroupOf(view);
    return box ? box->subtitleHeight(view, width) : 0;
}

extern "C" void gkWin32GroupBoxBody(void *pointer, int *width, int *height, int *top) {
    GKView *view = gkView(pointer);
    int w = 0, h = 0, t = 0;
    if (gkGroupOf(view)) {
        int chromeWidth, chromeHeight;
        gkWin32GroupBoxChrome(pointer, &chromeWidth, &chromeHeight);
        w = std::max(0, view->width - chromeWidth);
        h = std::max(0, view->height - chromeHeight);
        t = gkWin32GroupBoxSubtitleHeight(pointer, w);
    }
    *width = w;
    *height = h;
    *top = t;
}

/* ---- Split view ----

   The panes of a split view are layouts with a divider between each pair,
   which the user drags. Windows has no split view control, so the bridge
   draws the dividers. */

struct GKSplit;

struct GKDivider : GKExtension {
    GKView *split = nullptr;
    int index = 0;
    int dragging = 0;
    double offset = 0;
    void paint(GKView *view, GKCanvas &canvas) override;
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override;
};

static const double gkSplitGrip = 6;

struct GKSplit : GKExtension {
    int vertical = 1;
    int count = 1;
    std::vector<HWND> panes;
    std::vector<GKView *> dividers;
    std::vector<double> positions;
    std::vector<double> minimums;
    std::vector<int> collapsible;
    std::vector<GKView *> records;
    ~GKSplit() override {
        for (GKView *record : records) {
            if (record) {
                record->outer = nullptr;
                gkReleaseObject(record);
            }
        }
        for (GKView *divider : dividers) {
            gkReleaseObject(divider);
        }
    }
    double length(GKView *view) {
        return vertical ? view->width : view->height;
    }
    double cross(GKView *view) {
        return vertical ? view->height : view->width;
    }
    void defaults(GKView *view) {
        double total = length(view);
        for (int i = 0; i < count - 1; i++) {
            if (positions[i] < 0) {
                positions[i] = (total - (count - 1)) * (i + 1.0) / count + i;
            }
        }
    }
    void arrange(GKView *view) {
        defaults(view);
        double total = length(view), across = cross(view);
        for (int i = 0; i < count; i++) {
            double start = i == 0 ? 0 : positions[i - 1] + 1;
            double end = i == count - 1 ? total : positions[i];
            double size = end - start > 0 ? floor(end - start) : 0;
            if (vertical) {
                gkPlaceWindow(panes[i], start, 0, size, across);
            } else {
                gkPlaceWindow(panes[i], 0, start, across, size);
            }
            if (i < count - 1) {
                double grip = positions[i] + 0.5 - gkSplitGrip / 2;
                if (vertical) {
                    gkPlaceWindow(dividers[i]->outer, grip, 0, gkSplitGrip, across);
                } else {
                    gkPlaceWindow(dividers[i]->outer, 0, grip, across, gkSplitGrip);
                }
            }
        }
    }
    void clamp(GKView *view, int index, double position) {
        double lower = (index == 0 ? 0 : positions[index - 1] + 1) + (collapsible[index] ? 0 : minimums[index]);
        double upper = (index + 1 < count - 1 ? positions[index + 1] : length(view)) - 1 -
                       (collapsible[index + 1] ? 0 : minimums[index + 1]);
        positions[index] = position < lower ? lower : position > upper ? upper : position;
    }
    void layout(GKView *view) override {
        arrange(view);
    }
};

static GKSplit *gkSplitOf(GKView *view) {
    return view && view->kind == GK_KIND_SPLIT ? static_cast<GKSplit *>(view->ext.get()) : nullptr;
}

void GKDivider::paint(GKView *view, GKCanvas &canvas) {
    GKSplit *data = gkSplitOf(split);
    if (!data) {
        return;
    }
    GKColor color = view->hover || dragging ? gkViewPalette(split).accent : gkViewPalette(split).divider;
    if (data->vertical) {
        canvas.fillRect(floorf(canvas.width / 2), 0, 1, canvas.height, color);
    } else {
        canvas.fillRect(0, floorf(canvas.height / 2), canvas.width, 1, color);
    }
}

bool GKDivider::message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) {
    (void)hwnd;
    (void)wparam;
    GKSplit *data = gkSplitOf(split);
    if (!data) {
        return false;
    }
    switch (message) {
    case WM_SETCURSOR:
        SetCursor(LoadCursorW(nullptr, data->vertical ? IDC_SIZEWE : IDC_SIZENS));
        *result = TRUE;
        return true;
    case WM_LBUTTONDOWN: {
        POINT point = {GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
        MapWindowPoints(view->outer, split->outer, &point, 1);
        double scale = gkViewScaleOf(split);
        offset = (data->vertical ? point.x : point.y) / scale - data->positions[index];
        dragging = 1;
        SetCapture(view->outer);
        *result = 0;
        return true;
    }
    case WM_MOUSEMOVE:
        if (dragging) {
            POINT point = {GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
            MapWindowPoints(view->outer, split->outer, &point, 1);
            double scale = gkViewScaleOf(split);
            data->clamp(split, index, (data->vertical ? point.x : point.y) / scale - offset);
            data->arrange(split);
            gkSend(split->handle, GK_EVENT_RESIZE, 0, 0, 0, 0, nullptr);
        }
        return false;
    case WM_LBUTTONUP:
    case WM_CAPTURECHANGED:
        if (dragging) {
            dragging = 0;
            if (message == WM_LBUTTONUP) {
                ReleaseCapture();
            }
            InvalidateRect(view->outer, nullptr, FALSE);
        }
        return false;
    default:
        return false;
    }
}

extern "C" void *gkWin32SplitViewCreate(uintptr_t window, int vertical, int panes) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_SPLIT, window, 0);
    GKSplit *split = new GKSplit();
    split->vertical = vertical != 0;
    split->count = panes > 1 ? panes : 1;
    view->ext.reset(split);
    for (int i = 0; i < split->count; i++) {
        split->panes.push_back(gkLayoutNew(host));
        split->positions.push_back(-1);
        split->minimums.push_back(0);
        split->collapsible.push_back(0);
        split->records.push_back(nullptr);
    }
    for (int i = 0; i < split->count - 1; i++) {
        GKDivider *divider = new GKDivider();
        divider->split = view;
        divider->index = i;
        GKView *record = gkCreateDrawn(0, GK_KIND_GENERIC, divider, 0, 0);
        record->owner = view;
        gkReparent(record->outer, host);
        split->dividers.push_back(record);
    }
    return view;
}

extern "C" void *gkWin32SplitViewPane(void *pointer, int index) {
    GKView *view = gkView(pointer);
    GKSplit *split = gkSplitOf(view);
    if (!split || index < 0 || index >= split->count) {
        return nullptr;
    }
    if (!split->records[index]) {
        GKView *record = gkViewNew(nullptr, nullptr, GK_KIND_CLIP, (uintptr_t)view->window, 0);
        record->outer = split->panes[index];
        record->body = split->panes[index];
        split->records[index] = record;
    }
    return split->records[index];
}

extern "C" void gkWin32SplitViewSetPane(void *pointer, int index, double minimum, int collapsible) {
    GKSplit *split = gkSplitOf(gkView(pointer));
    if (!split || index < 0 || index >= split->count) {
        return;
    }
    split->minimums[index] = minimum > 0 ? minimum : 0;
    split->collapsible[index] = collapsible != 0;
}

extern "C" void gkWin32SplitViewPaneFrame(void *pointer, int index, int *x, int *y, int *width, int *height) {
    GKView *view = gkView(pointer);
    GKSplit *split = gkSplitOf(view);
    *x = *y = *width = *height = 0;
    if (!split || index < 0 || index >= split->count) {
        return;
    }
    split->defaults(view);
    double start = index == 0 ? 0 : split->positions[index - 1] + 1;
    double end = index == split->count - 1 ? split->length(view) : split->positions[index];
    int length = (int)floor(end - start > 0 ? end - start : 0);
    int cross = (int)split->cross(view);
    if (split->vertical) {
        *x = (int)start;
        *width = length;
        *height = cross;
    } else {
        *y = (int)start;
        *width = cross;
        *height = length;
    }
}

extern "C" double gkWin32SplitViewDivider(void *pointer, int index) {
    GKView *view = gkView(pointer);
    GKSplit *split = gkSplitOf(view);
    if (!split || index < 0 || index >= split->count - 1) {
        return 0;
    }
    split->defaults(view);
    return split->positions[index];
}

extern "C" void gkWin32SplitViewSetDivider(void *pointer, int index, double position) {
    GKView *view = gkView(pointer);
    GKSplit *split = gkSplitOf(view);
    if (!split || index < 0 || index >= split->count - 1) {
        return;
    }
    split->defaults(view);
    split->clamp(view, index, position);
    split->arrange(view);
}

/* ---- Popover ----

   A popover is a pop-up window with a shadow next to its anchor. A
   transient popover closes when another window becomes active, and its
   owner keeps the look of the active window while it shows. */

struct GKPopover : GKObject {
    HWND popup = nullptr;
    HWND owner = nullptr;
    GKView *record = nullptr;
    int transient = 1;
    int shown = 0;
    ~GKPopover() override;
};

static const wchar_t *gkPopoverClass = L"GameKitPopover";

static std::vector<GKPopover *> &gkPopovers() {
    static std::vector<GKPopover *> popovers;
    return popovers;
}

static void gkPopoverClose(GKPopover *popover, bool report) {
    if (!popover->shown) {
        return;
    }
    popover->shown = 0;
    ShowWindow(popover->popup, SW_HIDE);
    if (popover->owner && IsWindow(popover->owner) && GetForegroundWindow() == nullptr) {
        SetForegroundWindow(popover->owner);
    }
    if (report) {
        gkSend(popover->handle, GK_EVENT_CLOSE, 0, 0, 0, 0, nullptr);
    }
}

GKPopover::~GKPopover() {
    auto &list = gkPopovers();
    list.erase(std::remove(list.begin(), list.end(), this), list.end());
    if (record) {
        record->outer = nullptr;
        gkReleaseObject(record);
    }
    if (popup && IsWindow(popup)) {
        SetWindowLongPtrW(popup, GWLP_USERDATA, 0);
        DestroyWindow(popup);
    }
}

bool gkForwardNotification(UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result);

static LRESULT CALLBACK gkPopoverProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    GKPopover *popover = reinterpret_cast<GKPopover *>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
    LRESULT result = 0;
    if (gkForwardNotification(message, wparam, lparam, &result)) {
        return result;
    }
    switch (message) {
    case WM_ACTIVATE:
        if (popover && LOWORD(wparam) == WA_INACTIVE && popover->transient) {
            HWND next = (HWND)lparam;
            /* A dialog that the popover opens keeps it open. */
            if (!next || GetWindow(next, GW_OWNER) != hwnd) {
                gkLater([popover, hwnd]() {
                    auto &list = gkPopovers();
                    if (std::find(list.begin(), list.end(), popover) != list.end() && popover->popup == hwnd &&
                        GetActiveWindow() != hwnd) {
                        gkPopoverClose(popover, true);
                    }
                });
            }
        }
        break;
    case WM_KEYDOWN:
        if (popover && wparam == VK_ESCAPE) {
            gkPopoverClose(popover, true);
            return 0;
        }
        break;
    case WM_CLOSE:
        if (popover) {
            gkPopoverClose(popover, true);
        }
        return 0;
    case WM_ERASEBKGND: {
        RECT client;
        GetClientRect(hwnd, &client);
        GKColor color = gkPalette(gkResolvedTheme()).flyout;
        HBRUSH brush = CreateSolidBrush(RGB((int)(color.r * 255), (int)(color.g * 255), (int)(color.b * 255)));
        FillRect((HDC)wparam, &client, brush);
        DeleteObject(brush);
        return 1;
    }
    default:
        break;
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

/* gkPopoverOwns reports whether a window owns a popover that shows. */
bool gkPopoverOwns(HWND owner) {
    for (GKPopover *popover : gkPopovers()) {
        if (popover->shown && popover->owner == owner) {
            return true;
        }
    }
    return false;
}

extern "C" void *gkWin32PopoverCreate(uintptr_t handle, int transient) {
    static bool registered;
    if (!registered) {
        registered = true;
        WNDCLASSEXW cls = {};
        cls.cbSize = sizeof(cls);
        cls.style = CS_DROPSHADOW;
        cls.lpfnWndProc = gkPopoverProc;
        cls.hInstance = gkInstance();
        cls.hCursor = LoadCursorW(nullptr, IDC_ARROW);
        cls.lpszClassName = gkPopoverClass;
        RegisterClassExW(&cls);
    }
    GKPopover *popover = new GKPopover();
    popover->handle = handle;
    popover->transient = transient != 0;
    GKActivation activation;
    popover->popup = CreateWindowExW(WS_EX_TOOLWINDOW | WS_EX_CONTROLPARENT, gkPopoverClass, L"",
                                     WS_POPUP | WS_BORDER | WS_CLIPCHILDREN, 0, 0, 1, 1, nullptr, nullptr,
                                     gkInstance(), nullptr);
    SetWindowLongPtrW(popover->popup, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(popover));
    HWND body = gkLayoutNew(popover->popup);
    GKColor flyout = gkPalette(gkResolvedTheme()).flyout;
    gkLayoutSetBackground(body, &flyout);
    popover->record = gkViewNew(nullptr, nullptr, GK_KIND_CLIP, 0, 0);
    popover->record->outer = body;
    popover->record->body = body;
    gkPopovers().push_back(popover);
    return popover;
}

extern "C" void *gkWin32PopoverBody(void *pointer) {
    GKPopover *popover = static_cast<GKPopover *>(pointer);
    return popover ? popover->record : nullptr;
}

extern "C" int gkWin32PopoverShow(void *pointer, void *anchorPointer, int edge, int width, int height) {
    GKPopover *popover = static_cast<GKPopover *>(pointer);
    GKView *anchor = gkView(anchorPointer);
    GKWin *win = anchor ? gkWinOf(anchor->outer) : nullptr;
    if (!popover || !anchor || !win || !IsWindowVisible(win->hwnd) || !IsWindowVisible(anchor->outer)) {
        return 0;
    }
    popover->owner = win->hwnd;
    popover->record->window = win;
    SetWindowLongPtrW(popover->popup, GWLP_HWNDPARENT, reinterpret_cast<LONG_PTR>(win->hwnd));
    width = width > 1 ? width : 1;
    height = height > 1 ? height : 1;
    double scale = gkScaleOf(win->hwnd);
    GKColor flyout = gkPalette(gkWinTheme(win)).flyout;
    gkLayoutSetBackground(popover->record->outer, &flyout);
    RECT anchorRect;
    GetWindowRect(anchor->outer, &anchorRect);
    RECT frame = {0, 0, (LONG)lround(width * scale), (LONG)lround(height * scale)};
    AdjustWindowRectExForDpi(&frame, WS_POPUP | WS_BORDER, FALSE, WS_EX_TOOLWINDOW, GetDpiForWindow(win->hwnd));
    int frameWidth = frame.right - frame.left, frameHeight = frame.bottom - frame.top;
    int gap = (int)lround(4 * scale);
    int anchorWidth = anchorRect.right - anchorRect.left, anchorHeight = anchorRect.bottom - anchorRect.top;
    int x = anchorRect.left + (anchorWidth - frameWidth) / 2, y = anchorRect.bottom + gap;
    switch (edge) {
    case 0:
        y = anchorRect.top - frameHeight - gap;
        break;
    case 2:
        x = anchorRect.left - frameWidth - gap;
        y = anchorRect.top + (anchorHeight - frameHeight) / 2;
        break;
    case 3:
        x = anchorRect.right + gap;
        y = anchorRect.top + (anchorHeight - frameHeight) / 2;
        break;
    default:
        break;
    }
    /* The popover stays on the screen of its anchor. */
    MONITORINFO monitor = {sizeof(monitor)};
    GetMonitorInfoW(MonitorFromWindow(win->hwnd, MONITOR_DEFAULTTONEAREST), &monitor);
    x = std::max<int>(monitor.rcWork.left, std::min<int>(x, monitor.rcWork.right - frameWidth));
    y = std::max<int>(monitor.rcWork.top, std::min<int>(y, monitor.rcWork.bottom - frameHeight));
    SetWindowPos(popover->popup, HWND_TOP, x, y, frameWidth, frameHeight, SWP_NOACTIVATE);
    gkPlaceWindow(popover->record->outer, 0, 0, width, height);
    popover->shown = 1;
    ShowWindow(popover->popup, SW_SHOW);
    SetForegroundWindow(popover->popup);
    return 1;
}

extern "C" void gkWin32PopoverHide(void *pointer) {
    GKPopover *popover = static_cast<GKPopover *>(pointer);
    if (popover) {
        gkPopoverClose(popover, true);
    }
}

extern "C" int gkWin32PopoverShown(void *pointer) {
    GKPopover *popover = static_cast<GKPopover *>(pointer);
    return popover && popover->shown;
}

extern "C" void gkWin32PopoverRelease(void *pointer) {
    GKPopover *popover = static_cast<GKPopover *>(pointer);
    if (!popover) {
        return;
    }
    popover->handle = 0;
    gkReleaseObject(popover);
}

/* ---- Title bar accessories ----

   A window of Windows has no room for controls in its title bar, so
   accessories share a bar at the top of the content, below the title
   bar. */

struct GKAccessoryBar : GKExtension {
    std::vector<std::pair<GKView *, int>> items;
    std::vector<std::pair<int, int>> sizes;
    void layout(GKView *view) override {
        double left = 8, right = view->width - 8;
        double total = 0;
        for (size_t i = 0; i < items.size(); i++) {
            if (items[i].second == 2) {
                total += sizes[i].first + 6;
            }
        }
        double center = (view->width - total) / 2;
        for (size_t i = 0; i < items.size(); i++) {
            GKView *item = items[i].first;
            double w = sizes[i].first, h = sizes[i].second;
            double y = (view->height - h) / 2;
            double x;
            if (items[i].second == 1) {
                x = left;
                left += w + 6;
            } else if (items[i].second == 3) {
                right -= w;
                x = right;
                right -= 6;
            } else {
                x = center;
                center += w + 6;
            }
            gkPlaceWindow(item->outer, x, y, w, h);
        }
    }
};

struct GKAccessory : GKObject {
    GKWin *win = nullptr;
    GKView *bar = nullptr;
    GKView *view = nullptr;
};

static std::unordered_map<GKWin *, GKView *> &gkAccessoryBars() {
    static std::unordered_map<GKWin *, GKView *> bars;
    return bars;
}

static void gkAccessoryBarUpdate(GKWin *win, GKView *bar) {
    GKAccessoryBar *data = static_cast<GKAccessoryBar *>(bar->ext.get());
    int height = 0;
    for (auto const &size : data->sizes) {
        height = std::max(height, size.second);
    }
    if (data->items.empty()) {
        gkWinRemoveChrome(win, bar);
    } else {
        gkWinAddChrome(win, bar, 1, height + 8);
        data->layout(bar);
    }
}

extern "C" int gkWin32TitleAccessoryAdd(void *window, void *viewPointer, int alignment, int width, int height,
                                        void **controller) {
    GKWin *win = static_cast<GKWin *>(window);
    GKView *view = gkView(viewPointer);
    if (!win || !view || win->borderless) {
        return 0;
    }
    GKView *&bar = gkAccessoryBars()[win];
    if (!bar) {
        HWND host = gkLayoutNew(gkParking());
        bar = gkViewNew(host, nullptr, GK_KIND_LAYOUT, (uintptr_t)win, 0);
        bar->ext.reset(new GKAccessoryBar());
    }
    GKAccessoryBar *data = static_cast<GKAccessoryBar *>(bar->ext.get());
    data->items.emplace_back(view, alignment == 1 ? 1 : alignment == 3 ? 3 : 2);
    data->sizes.emplace_back(width > 1 ? width : 1, height > 1 ? height : 1);
    gkReparent(view->outer, bar->outer);
    ShowWindow(view->outer, SW_SHOWNA);
    gkAccessoryBarUpdate(win, bar);
    GKAccessory *accessory = new GKAccessory();
    accessory->win = win;
    accessory->bar = bar;
    accessory->view = view;
    *controller = accessory;
    return 1;
}

/* gkWin32TitleAccessoryLayout places the accessories of the bar again,
   after the Go layer has given one its size. */
extern "C" void gkWin32TitleAccessoryLayout(void *pointer) {
    GKAccessory *accessory = static_cast<GKAccessory *>(pointer);
    if (accessory && accessory->bar && accessory->bar->ext) {
        accessory->bar->ext->layout(accessory->bar);
    }
}

extern "C" void gkWin32TitleAccessoryRemove(void *pointer) {
    GKAccessory *accessory = static_cast<GKAccessory *>(pointer);
    if (!accessory) {
        return;
    }
    if (std::find(gkWindows().begin(), gkWindows().end(), accessory->win) != gkWindows().end()) {
        GKAccessoryBar *data = static_cast<GKAccessoryBar *>(accessory->bar->ext.get());
        for (size_t i = 0; i < data->items.size(); i++) {
            if (data->items[i].first == accessory->view) {
                data->items.erase(data->items.begin() + i);
                data->sizes.erase(data->sizes.begin() + i);
                break;
            }
        }
        if (gkView(accessory->view)) {
            ShowWindow(accessory->view->outer, SW_HIDE);
            gkReparent(accessory->view->outer, accessory->win->root);
        }
        gkAccessoryBarUpdate(accessory->win, accessory->bar);
    }
    gkReleaseObject(accessory);
}

extern "C" void gkWin32ViewSize(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    int w = 0, h = 0;
    if (view && view->outer) {
        D2D1_SIZE_F size = gkClientSize(view->outer);
        w = (int)lround(size.width);
        h = (int)lround(size.height);
        if (w <= 0 || h <= 0) {
            w = view->width;
            h = view->height;
        }
    }
    *width = w;
    *height = h;
}
