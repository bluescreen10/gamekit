//go:build windows && cgo

#include "win32_internal.h"

#include <uxtheme.h>

#include <algorithm>

/* ---- Toolbar controls ----

   A toolbar and a command bar are toolbar controls of Windows with a button
   for each action. A flexible space is a separator whose width takes the
   room that the buttons leave. */

struct GKToolItem {
    int kind = 0; /* 0 action, 1 separator, 2 space, 3 flexible space */
    std::string id;
    std::wstring label;
    int enabled = 1;
    int image = -1;
};

struct GKToolbarExt : GKExtension {
    HWND bar = nullptr;
    std::vector<GKToolItem> items;
    HIMAGELIST images = nullptr;
    int showsLabels = 1;
    int overflow = 0; /* hides the buttons that do not fit behind a button */
    uintptr_t target = 0;
    int hidden = 0;
    std::vector<GKIcon *> icons;
    LONG fullWidth = 0;
    /* labelsBelow shows labels below the icons, as the toolbars of windows
       do on other platforms; a command bar shows them beside the icons. */
    int labelsBelow = 0;
    bool below() {
        return labelsBelow && showsLabels;
    }
    ~GKToolbarExt() override {
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
        for (GKIcon *icon : icons) {
            if (icon) {
                gkReleaseObject(icon);
            }
        }
    }
    /* metrics gives the buttons the room of the command bars of Windows:
       padding around the icon and the label, and a gap between buttons. */
    void metrics(GKView *view) {
        double scale = gkViewScaleOf(view);
        TBMETRICS metrics = {};
        metrics.cbSize = sizeof(metrics);
        metrics.dwMask = TBMF_PAD | TBMF_BUTTONSPACING;
        metrics.cxPad = (int)lround(12 * scale);
        metrics.cyPad = (int)lround(10 * scale);
        metrics.cxButtonSpacing = (int)lround((labelsBelow ? 8 : 4) * scale);
        metrics.cyButtonSpacing = 0;
        SendMessageW(bar, TB_SETMETRICS, 0, (LPARAM)&metrics);
        SendMessageW(bar, TB_SETPADDING, 0, MAKELPARAM(metrics.cxPad, metrics.cyPad));
    }
    int commandOf(size_t index) {
        return 100 + (int)index;
    }
    void build(GKView *view, void **source) {
        std::vector<GKIcon *> next(items.size(), nullptr);
        for (size_t i = 0; i < items.size() && source; i++) {
            next[i] = static_cast<GKIcon *>(source[i]);
            if (next[i]) {
                gkRetainObject(next[i]);
            }
        }
        for (GKIcon *icon : icons) {
            if (icon) {
                gkReleaseObject(icon);
            }
        }
        icons = std::move(next);
        SendMessageW(bar, WM_SETREDRAW, FALSE, 0);
        LONG_PTR style = SendMessageW(bar, TB_GETSTYLE, 0, 0);
        SendMessageW(bar, TB_SETSTYLE, 0, below() ? (style & ~(LONG_PTR)TBSTYLE_LIST) : (style | TBSTYLE_LIST));
        metrics(view);
        while (SendMessageW(bar, TB_BUTTONCOUNT, 0, 0) > 0) {
            SendMessageW(bar, TB_DELETEBUTTON, 0, 0);
        }
        if (images) {
            SendMessageW(bar, TB_SETIMAGELIST, 0, 0);
            gkComctl().ImageList_Destroy(images);
            images = nullptr;
        }
        int side = (int)lround(16 * gkViewScaleOf(view));
        bool anyIcon = false;
        for (size_t i = 0; i < items.size() && i < icons.size(); i++) {
            anyIcon |= icons[i] != nullptr;
        }
        if (anyIcon) {
            images = gkComctl().ImageList_Create(side, side, ILC_COLOR32, (int)items.size(), 1);
            for (size_t i = 0; i < items.size() && i < icons.size(); i++) {
                if (icons[i]) {
                    HBITMAP bitmap = gkIconBitmap(icons[i], side, gkViewPalette(view).text);
                    items[i].image = gkComctl().ImageList_Add(images, bitmap, nullptr);
                    DeleteObject(bitmap);
                }
            }
            SendMessageW(bar, TB_SETIMAGELIST, 0, (LPARAM)images);
        } else {
            SendMessageW(bar, TB_SETBITMAPSIZE, 0, MAKELPARAM(0, side));
        }
        std::vector<TBBUTTON> buttons;
        for (size_t i = 0; i < items.size(); i++) {
            TBBUTTON button = {};
            button.idCommand = commandOf(i);
            if (items[i].kind == 0) {
                button.iBitmap = items[i].image >= 0 ? items[i].image : I_IMAGENONE;
                button.fsState = items[i].enabled ? TBSTATE_ENABLED : 0;
                button.fsStyle = BTNS_BUTTON | BTNS_AUTOSIZE |
                                 ((showsLabels || items[i].image < 0) && !below() ? BTNS_SHOWTEXT : 0);
                button.iString = (INT_PTR)items[i].label.c_str();
            } else {
                /* A space is a separator whose line the toolbar paints
                   over; see message. */
                button.iBitmap = items[i].kind == 1 ? 6 : items[i].kind == 2 ? (int)lround(12 * gkViewScaleOf(view)) : 0;
                button.fsStyle = BTNS_SEP;
            }
            buttons.push_back(button);
        }
        if (!buttons.empty()) {
            SendMessageW(bar, TB_ADDBUTTONSW, buttons.size(), (LPARAM)buttons.data());
        }
        SendMessageW(bar, TB_AUTOSIZE, 0, 0);
        /* The full width, with every button shown, is the preferred width;
           fit later hides the buttons that do not fit. */
        SIZE size = {};
        SendMessageW(bar, TB_GETMAXSIZE, 0, (LPARAM)&size);
        fullWidth = size.cx;
        SendMessageW(bar, WM_SETREDRAW, TRUE, 0);
        InvalidateRect(bar, nullptr, TRUE);
    }
    int buttonHeight() {
        LRESULT size = SendMessageW(bar, TB_GETBUTTONSIZE, 0, 0);
        return HIWORD(size);
    }
    /* fit sets the width of the flexible spaces and hides the buttons that
       do not fit, in logical units. */
    void fit(GKView *view) {
        double scale = gkViewScaleOf(view);
        RECT client;
        GetClientRect(bar, &client);
        if (client.right <= 0) {
            return;
        }
        /* The buttons take their width and the spacing between them; the
           flexible spaces share what is left. */
        int flexible = 0;
        for (size_t i = 0; i < items.size(); i++) {
            SendMessageW(bar, TB_HIDEBUTTON, commandOf(i), FALSE);
            if (items[i].kind == 3) {
                flexible++;
                TBBUTTONINFOW info = {};
                info.cbSize = sizeof(info);
                info.dwMask = TBIF_SIZE;
                info.cx = 0;
                SendMessageW(bar, TB_SETBUTTONINFOW, commandOf(i), (LPARAM)&info);
            }
        }
        int fixed = 0;
        if (!items.empty()) {
            RECT last = {};
            SendMessageW(bar, TB_GETITEMRECT, items.size() - 1, (LPARAM)&last);
            fixed = last.right;
        }
        /* The overflow button takes room only when the buttons do not
           fit. */
        int reserve = overflow && fixed > client.right ? (int)lround(28 * scale) : 0;
        int extra = std::max(0, (int)client.right - reserve - fixed);
        auto spread = [&](int room) {
            for (size_t i = 0; i < items.size(); i++) {
                if (items[i].kind == 3) {
                    TBBUTTONINFOW info = {};
                    info.cbSize = sizeof(info);
                    info.dwMask = TBIF_SIZE;
                    info.cx = (WORD)(flexible ? std::max(0, room) / flexible : 0);
                    SendMessageW(bar, TB_SETBUTTONINFOW, commandOf(i), (LPARAM)&info);
                }
            }
        };
        spread(extra);
        /* A separator can take more room than its width; the spaces give
           back what the last button passes the end. */
        if (flexible && !items.empty()) {
            /* The spaces take the difference between where the last button
               ends and the end of the bar. */
            for (int pass = 0; pass < 2; pass++) {
                RECT last = {};
                SendMessageW(bar, TB_GETITEMRECT, items.size() - 1, (LPARAM)&last);
                int difference = (int)(client.right - reserve) - last.right;
                if (difference == 0) {
                    break;
                }
                extra = std::max(0, extra + difference);
                spread(extra);
            }
        }
        hidden = 0;
        if (!overflow) {
            return;
        }
        int limit = client.right - reserve;
        for (size_t i = 0; i < items.size(); i++) {
            RECT rect = {};
            SendMessageW(bar, TB_GETITEMRECT, i, (LPARAM)&rect);
            if (rect.right > limit) {
                SendMessageW(bar, TB_HIDEBUTTON, commandOf(i), TRUE);
                hidden++;
            }
        }
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        if (hwnd == bar && message == WM_PAINT) {
            /* A space or a flexible space is blank: the line that the
               toolbar draws for a separator is painted over. */
            *result = gkComctl().DefSubclassProc(hwnd, message, wparam, lparam);
            HDC hdc = GetDC(bar);
            GKColor color = gkBackgroundOf(bar);
            HBRUSH brush = CreateSolidBrush(
                RGB((int)lround(color.r * 255), (int)lround(color.g * 255), (int)lround(color.b * 255)));
            for (size_t i = 0; i < items.size(); i++) {
                if (items[i].kind == 2 || items[i].kind == 3) {
                    RECT rect = {};
                    if (SendMessageW(bar, TB_GETITEMRECT, i, (LPARAM)&rect)) {
                        FillRect(hdc, &rect, brush);
                    }
                }
            }
            DeleteObject(brush);
            ReleaseDC(bar, hdc);
            return true;
        }
        if (message == WM_COMMAND && (HWND)lparam == bar) {
            size_t index = (size_t)LOWORD(wparam) - 100;
            if (index < items.size()) {
                gkSend(target, GK_EVENT_ACTION, 0, 0, 0, 0, items[index].id.c_str());
            }
            *result = 0;
            return true;
        }
        if (message == WM_SIZE && hwnd == view->outer) {
            layout(view);
        }
        return false;
    }
    void layout(GKView *view) override {
        RECT client;
        GetClientRect(view->outer, &client);
        int height = buttonHeight();
        /* A window toolbar keeps a margin from the edges of the window. */
        int inset = labelsBelow ? (int)lround(4 * gkViewScaleOf(view)) : 0;
        SetWindowPos(bar, nullptr, inset, std::max(0, (int)(client.bottom - height) / 2), client.right - 2 * inset,
                     height, SWP_NOZORDER | SWP_NOACTIVATE);
        fit(view);
        if (HWND more = (HWND)GetPropW(view->outer, L"GKMore")) {
            double scale = gkViewScaleOf(view);
            int side = std::min(height, (int)lround(28 * scale));
            SetWindowPos(more, HWND_TOP, client.right - (int)lround(26 * scale), (client.bottom - side) / 2,
                         (int)lround(24 * scale), side, SWP_NOACTIVATE);
            ShowWindow(more, hidden ? SW_SHOWNA : SW_HIDE);
        }
    }
    void themeChanged(GKView *view) override {
        gkApplyThemeTo(bar, gkViewTheme(view));
        SendMessageW(bar, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(bar)), TRUE);
        /* The glyphs of the icons take the text color of the theme. */
        if (!items.empty()) {
            std::vector<void *> current(icons.begin(), icons.end());
            build(view, current.data());
            layout(view);
        }
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        double scale = gkViewScaleOf(view);
        return D2D1::SizeF((float)(fullWidth / scale + (overflow ? 28 : 0)), (float)(buttonHeight() / scale + 4));
    }
};

static LRESULT CALLBACK gkMoreProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, UINT_PTR id, DWORD_PTR data);

/* gkToolbarView makes the view of a toolbar control. */
static GKView *gkToolbarView(uintptr_t window, int kind, uintptr_t target, int overflow) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, kind, window, 0);
    GKToolbarExt *toolbar = new GKToolbarExt();
    toolbar->target = target;
    toolbar->overflow = overflow;
    view->ext.reset(toolbar);
    toolbar->bar = gkMakeNative(TOOLBARCLASSNAMEW, L"",
                                TBSTYLE_FLAT | TBSTYLE_LIST | TBSTYLE_TOOLTIPS | CCS_NODIVIDER | CCS_NORESIZE |
                                    CCS_NOPARENTALIGN | WS_TABSTOP,
                                0, host);
    SendMessageW(toolbar->bar, TB_BUTTONSTRUCTSIZE, sizeof(TBBUTTON), 0);
    SendMessageW(toolbar->bar, TB_SETEXTENDEDSTYLE, 0, TBSTYLE_EX_MIXEDBUTTONS | TBSTYLE_EX_DOUBLEBUFFER);
    SetPropW(toolbar->bar, L"GKView", view);
    gkSubclass(toolbar->bar);
    view->control = toolbar->bar;
    if (overflow) {
        HWND more = gkMakeNative(WC_BUTTONW, L"\xE712", BS_PUSHBUTTON, 0, host);
        GKFont symbol;
        symbol.family = gkSymbolFamily();
        symbol.size = 12;
        SendMessageW(more, WM_SETFONT, (WPARAM)gkFontHandle(symbol, GetDpiForWindow(more)), FALSE);
        SetPropW(host, L"GKMore", more);
        gkComctl().SetWindowSubclass(more, gkMoreProc, 4, reinterpret_cast<DWORD_PTR>(view));
        ShowWindow(more, SW_HIDE);
    }
    toolbar->themeChanged(view);
    return view;
}

/* gkMoreProc shows the actions that do not fit in a menu. */
static LRESULT CALLBACK gkMoreProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, UINT_PTR id,
                                   DWORD_PTR data) {
    (void)id;
    if (message == WM_LBUTTONUP) {
        GKView *view = gkView(reinterpret_cast<void *>(data));
        GKToolbarExt *toolbar = view ? static_cast<GKToolbarExt *>(view->ext.get()) : nullptr;
        if (toolbar) {
            HMENU popup = CreatePopupMenu();
            for (size_t i = 0; i < toolbar->items.size(); i++) {
                TBBUTTONINFOW info = {};
                info.cbSize = sizeof(info);
                info.dwMask = TBIF_STATE;
                SendMessageW(toolbar->bar, TB_GETBUTTONINFOW, toolbar->commandOf(i), (LPARAM)&info);
                if (toolbar->items[i].kind == 0 && (info.fsState & TBSTATE_HIDDEN)) {
                    AppendMenuW(popup, MF_STRING | (toolbar->items[i].enabled ? 0 : MF_GRAYED), i + 1,
                                toolbar->items[i].label.c_str());
                }
            }
            RECT rect;
            GetWindowRect(hwnd, &rect);
            UINT command = (UINT)TrackPopupMenuEx(popup, TPM_RETURNCMD | TPM_RIGHTALIGN, rect.right, rect.bottom,
                                                  GetAncestor(hwnd, GA_ROOT), nullptr);
            DestroyMenu(popup);
            if (command >= 1 && command <= toolbar->items.size()) {
                gkSend(toolbar->target, GK_EVENT_ACTION, 0, 0, 0, 0, toolbar->items[command - 1].id.c_str());
            }
        }
    }
    if (message == WM_NCDESTROY) {
        gkComctl().RemoveWindowSubclass(hwnd, gkMoreProc, 4);
    }
    return gkComctl().DefSubclassProc(hwnd, message, wparam, lparam);
}

/* ---- Toolbar ---- */

struct GKToolbarObject : GKObject {
    GKWin *win = nullptr;
    GKView *view = nullptr;
    ~GKToolbarObject() override {
        if (view) {
            gkReleaseObject(view);
        }
    }
};

/* Item kinds: 0 action, 1 separator, 2 space, 3 flexible space. */
extern "C" void *gkWin32ToolbarCreate(uintptr_t handle, void *window, const int *kinds, const char *ids,
                                      size_t idsSize, const char *labels, size_t labelsSize, void **icons,
                                      const int *enabled, int count, int showsLabels) {
    GKWin *win = static_cast<GKWin *>(window);
    if (!win) {
        return nullptr;
    }
    GKToolbarObject *object = new GKToolbarObject();
    object->win = win;
    object->handle = handle;
    object->view = gkToolbarView((uintptr_t)win, GK_KIND_TOOLBAR, handle, 1);
    GKToolbarExt *toolbar = static_cast<GKToolbarExt *>(object->view->ext.get());
    toolbar->showsLabels = showsLabels != 0;
    toolbar->labelsBelow = 1;
    auto identifiers = gkSplit(ids, idsSize);
    auto names = gkSplit(labels, labelsSize);
    for (int i = 0; i < count; i++) {
        GKToolItem item;
        item.kind = kinds[i];
        item.id = i < (int)identifiers.size() ? identifiers[i] : "";
        item.label = gkWide((i < (int)names.size() ? names[i] : item.id).c_str());
        item.enabled = enabled[i] != 0;
        toolbar->items.push_back(item);
    }
    toolbar->build(object->view, icons);
    object->view->accessibleName = "Toolbar";
    double scale = gkWinScale(win);
    gkWinAddChrome(win, object->view, 1, (int)lround(toolbar->buttonHeight() / scale) + 6);
    return object;
}

extern "C" void gkWin32ToolbarSetEnabled(void *pointer, const char *identifier, int enabled) {
    GKToolbarObject *object = static_cast<GKToolbarObject *>(pointer);
    if (!object || !object->view) {
        return;
    }
    GKToolbarExt *toolbar = static_cast<GKToolbarExt *>(object->view->ext.get());
    for (size_t i = 0; i < toolbar->items.size(); i++) {
        if (toolbar->items[i].id == (identifier ? identifier : "")) {
            toolbar->items[i].enabled = enabled != 0;
            SendMessageW(toolbar->bar, TB_ENABLEBUTTON, toolbar->commandOf(i), enabled ? TRUE : FALSE);
        }
    }
}

extern "C" void gkWin32ToolbarRelease(void *pointer) {
    GKToolbarObject *object = static_cast<GKToolbarObject *>(pointer);
    if (!object) {
        return;
    }
    object->handle = 0;
    if (std::find(gkWindows().begin(), gkWindows().end(), object->win) != gkWindows().end()) {
        gkWinRemoveChrome(object->win, object->view);
    }
    gkReleaseObject(object);
}

/* ---- Status bar ---- */

struct GKStatusBar : GKExtension {
    D2D1_SIZE_F preferred(GKView *view) override {
        SIZE size = gkMeasureGDI(view->text, gkViewFont(view, gkBodyFont()));
        return D2D1::SizeF((float)size.cx + 16, (float)size.cy + 8);
    }
};

extern "C" void *gkWin32StatusBarCreate(uintptr_t window, const char *text) {
    HWND bar = gkMakeNative(STATUSCLASSNAMEW, L"", CCS_NOPARENTALIGN | CCS_NORESIZE | CCS_NODIVIDER | SBARS_TOOLTIPS, 0);
    GKView *view = gkAdopt(window, bar, nullptr, GK_KIND_STATUS_BAR, 0);
    view->ext.reset(new GKStatusBar());
    view->text = gkWide(text);
    SendMessageW(bar, SB_SETTEXTW, SBT_NOBORDERS, (LPARAM)view->text.c_str());
    return view;
}

extern "C" void gkWin32StatusBarSet(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_STATUS_BAR) {
        view->text = gkWide(text);
        SendMessageW(view->outer, SB_SETTEXTW, SBT_NOBORDERS, (LPARAM)view->text.c_str());
    }
}

/* ---- Command bar ---- */

extern "C" void *gkWin32CommandBarCreate(uintptr_t window) {
    GKView *view = gkToolbarView(window, GK_KIND_COMMAND_BAR, 0, 1);
    return view;
}

extern "C" void gkWin32CommandBarSetActions(void *pointer, const char *ids, size_t idsSize, const char *titles,
                                            size_t titlesSize, void **icons, const int *enabled, int count,
                                            int compact) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_COMMAND_BAR) {
        return;
    }
    GKToolbarExt *toolbar = static_cast<GKToolbarExt *>(view->ext.get());
    toolbar->target = view->handle;
    toolbar->showsLabels = !compact;
    toolbar->items.clear();
    auto identifiers = gkSplit(ids, idsSize);
    auto names = gkSplit(titles, titlesSize);
    for (int i = 0; i < count; i++) {
        GKToolItem item;
        item.id = i < (int)identifiers.size() ? identifiers[i] : "";
        item.label = gkWide(i < (int)names.size() ? names[i].c_str() : "");
        item.enabled = enabled[i] != 0;
        toolbar->items.push_back(item);
    }
    toolbar->build(view, icons);
    toolbar->layout(view);
}

std::vector<std::pair<std::string, std::string>> gkToolbarItems(GKView *view) {
    std::vector<std::pair<std::string, std::string>> items;
    if (view && (view->kind == GK_KIND_COMMAND_BAR || view->kind == GK_KIND_TOOLBAR)) {
        for (auto const &item : static_cast<GKToolbarExt *>(view->ext.get())->items) {
            if (item.kind == 0) {
                items.emplace_back("Button", gkUTF8(item.label));
            }
        }
    }
    return items;
}
