//go:build windows && cgo

#include "win32_internal.h"

#include <algorithm>

/* ---- Tabs ----

   Tabs at the top or the bottom are the tab control of Windows. The tab
   control shows its tabs only at the top or the bottom with the visual
   styles of Windows, so tabs at the left or the right are a list beside
   the pages. A page is the content view of a tab in the content area,
   which shows the page of the selected tab. */

struct GKTabs : GKExtension {
    int placement = 0;
    int overflow = 1;
    int selected = -1;
    std::vector<std::wstring> titles;
    std::vector<GKView *> pages;
    HWND strip = nullptr;   /* the tab control or the list */
    HWND content = nullptr; /* the layout that holds the pages */
    HWND menu = nullptr;    /* the button that lists every tab */
    int vertical() {
        return placement == 2 || placement == 3;
    }
    /* stripWidth is the width of a list of tabs at the side. */
    int stripWidth(GKView *view) {
        GKFont font = gkViewFont(view, gkBodyFont());
        int widest = 80;
        for (auto const &title : titles) {
            widest = std::max<int>(widest, gkMeasureGDI(title, font).cx + 24);
        }
        return std::min(widest, 240);
    }
    /* contentFrame returns the frame of the pages in logical units. */
    void contentFrame(GKView *view, double *x, double *y, double *width, double *height) {
        double scale = gkViewScaleOf(view);
        if (vertical()) {
            double side = stripWidth(view);
            *x = placement == 2 ? side : 0;
            *y = 0;
            *width = std::max(0.0, view->width - side);
            *height = view->height;
            return;
        }
        RECT rect = {0, 0, (LONG)lround(view->width * scale), (LONG)lround(view->height * scale)};
        SendMessageW(strip, TCM_ADJUSTRECT, FALSE, (LPARAM)&rect);
        *x = rect.left / scale;
        *y = rect.top / scale;
        *width = std::max(0.0, (rect.right - rect.left) / scale);
        *height = std::max(0.0, (rect.bottom - rect.top) / scale);
    }
    void layout(GKView *view) override {
        double menuSize = menu ? 24 : 0;
        if (vertical()) {
            double side = stripWidth(view);
            double x = placement == 2 ? 0 : view->width - side;
            if (menu) {
                gkPlaceWindow(menu, x, 0, side, menuSize);
            }
            gkPlaceWindow(strip, x, menuSize, side, std::max(0.0, view->height - menuSize));
        } else {
            gkPlaceWindow(strip, 0, 0, view->width, view->height);
            if (menu) {
                RECT item = {};
                double scale = gkViewScaleOf(view);
                double height = TabCtrl_GetItemRect(strip, 0, &item) ? (item.bottom - item.top) / scale : menuSize;
                double y = placement == 1 ? view->height - height - 2 : 2;
                gkPlaceWindow(menu, view->width - menuSize - 2, y, menuSize, height - 2);
                SetWindowPos(menu, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
            }
        }
        double x, y, width, height;
        contentFrame(view, &x, &y, &width, &height);
        gkPlaceWindow(content, x, y, width, height);
        SetWindowPos(content, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    }
    void showPage() {
        for (size_t i = 0; i < pages.size(); i++) {
            if (pages[i] && gkView(pages[i])) {
                ShowWindow(pages[i]->outer, (int)i == selected ? SW_SHOWNA : SW_HIDE);
            }
        }
    }
    void select(GKView *view, int index, bool report) {
        if (index == selected) {
            return;
        }
        selected = index;
        view->suppress++;
        if (vertical()) {
            SendMessageW(strip, LB_SETCURSEL, (WPARAM)index, 0);
        } else {
            SendMessageW(strip, TCM_SETCURSEL, (WPARAM)index, 0);
        }
        view->suppress--;
        showPage();
        if (report) {
            gkViewAction(view, GK_UI_EVENT_TAB_CHANGE);
        }
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->hwndFrom == strip && header->code == TCN_SELCHANGE) {
            select(view, (int)SendMessageW(strip, TCM_GETCURSEL, 0, 0), !view->suppress);
            *result = 0;
            return true;
        }
        return false;
    }
    bool command(GKView *view, HWND from, int code) override {
        if (from == strip && code == LBN_SELCHANGE) {
            select(view, (int)SendMessageW(strip, LB_GETCURSEL, 0, 0), !view->suppress);
            return true;
        }
        if (from == menu && code == BN_CLICKED) {
            HMENU popup = CreatePopupMenu();
            for (size_t i = 0; i < titles.size(); i++) {
                AppendMenuW(popup, MF_STRING | ((int)i == selected ? MF_CHECKED : 0), i + 1, titles[i].c_str());
            }
            RECT rect;
            GetWindowRect(menu, &rect);
            UINT command = (UINT)TrackPopupMenuEx(popup, TPM_RETURNCMD | TPM_RIGHTALIGN, rect.right, rect.bottom,
                                                  GetAncestor(menu, GA_ROOT), nullptr);
            DestroyMenu(popup);
            if (command >= 1 && command <= titles.size()) {
                select(view, (int)command - 1, true);
            }
            return true;
        }
        return false;
    }
    void themeChanged(GKView *view) override {
        SendMessageW(strip, WM_SETFONT, (WPARAM)gkFontHandle(gkViewFont(view, gkBodyFont()), GetDpiForWindow(strip)),
                     TRUE);
        gkApplyThemeTo(strip, gkViewTheme(view));
    }
};

static GKTabs *gkTabsOf(GKView *view) {
    return view && view->kind == GK_KIND_TABS ? static_cast<GKTabs *>(view->ext.get()) : nullptr;
}

/* gkTabsBuild makes the strip of the tabs for their placement and overflow
   policy. */
static void gkTabsBuild(GKView *view) {
    GKTabs *tabs = gkTabsOf(view);
    if (tabs->strip) {
        DestroyWindow(tabs->strip);
    }
    if (tabs->menu) {
        DestroyWindow(tabs->menu);
        tabs->menu = nullptr;
    }
    if (tabs->vertical()) {
        tabs->strip = gkMakeNative(WC_LISTBOXW, L"", LBS_NOTIFY | LBS_NOINTEGRALHEIGHT | WS_VSCROLL | WS_TABSTOP,
                                   WS_EX_CLIENTEDGE, view->outer);
        for (auto const &title : tabs->titles) {
            SendMessageW(tabs->strip, LB_ADDSTRING, 0, (LPARAM)title.c_str());
        }
        SendMessageW(tabs->strip, LB_SETCURSEL, (WPARAM)tabs->selected, 0);
    } else {
        tabs->strip = gkMakeNative(WC_TABCONTROLW, L"",
                                   WS_TABSTOP | WS_CLIPSIBLINGS | (tabs->placement == 1 ? TCS_BOTTOM : 0), 0,
                                   view->outer);
        for (size_t i = 0; i < tabs->titles.size(); i++) {
            TCITEMW item = {};
            item.mask = TCIF_TEXT;
            item.pszText = const_cast<wchar_t *>(tabs->titles[i].c_str());
            SendMessageW(tabs->strip, TCM_INSERTITEMW, i, (LPARAM)&item);
        }
        SendMessageW(tabs->strip, TCM_SETCURSEL, (WPARAM)tabs->selected, 0);
    }
    SetPropW(tabs->strip, L"GKView", view);
    gkSubclass(tabs->strip);
    if (tabs->overflow == 0) {
        tabs->menu = gkMakeNative(WC_BUTTONW, L"\xE70D", BS_PUSHBUTTON, 0, view->outer);
        SetPropW(tabs->menu, L"GKView", view);
        GKFont symbol;
        symbol.family = gkSymbolFamily();
        symbol.size = 10;
        SendMessageW(tabs->menu, WM_SETFONT, (WPARAM)gkFontHandle(symbol, GetDpiForWindow(tabs->menu)), FALSE);
        gkApplyThemeTo(tabs->menu, gkViewTheme(view));
    }
    view->control = tabs->strip;
    tabs->themeChanged(view);
    SetWindowPos(tabs->strip, HWND_BOTTOM, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    tabs->layout(view);
}

extern "C" void *gkWin32UICreateTabs(uintptr_t window, uintptr_t handle) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_TABS, window, handle);
    GKTabs *tabs = new GKTabs();
    view->ext.reset(tabs);
    tabs->content = gkLayoutNew(host);
    view->body = tabs->content;
    gkTabsBuild(view);
    return view;
}

extern "C" int gkWin32UITabsAdd(void *pointer, const char *title, void *content) {
    GKView *view = gkView(pointer);
    GKTabs *tabs = gkTabsOf(view);
    GKView *page = gkView(content);
    if (!tabs || !page) {
        return 0;
    }
    std::wstring text = gkWide(title);
    tabs->titles.push_back(text);
    tabs->pages.push_back(page);
    gkReparent(page->outer, tabs->content);
    view->suppress++;
    if (tabs->vertical()) {
        SendMessageW(tabs->strip, LB_ADDSTRING, 0, (LPARAM)text.c_str());
    } else {
        TCITEMW item = {};
        item.mask = TCIF_TEXT;
        item.pszText = const_cast<wchar_t *>(text.c_str());
        SendMessageW(tabs->strip, TCM_INSERTITEMW, tabs->titles.size() - 1, (LPARAM)&item);
    }
    view->suppress--;
    if (tabs->selected < 0) {
        tabs->select(view, 0, false);
    }
    tabs->showPage();
    tabs->layout(view);
    return 1;
}

extern "C" int gkWin32UITabsSelected(void *pointer) {
    GKTabs *tabs = gkTabsOf(gkView(pointer));
    return tabs ? tabs->selected : -1;
}

extern "C" void gkWin32UITabsSetSelected(void *pointer, int index) {
    GKView *view = gkView(pointer);
    GKTabs *tabs = gkTabsOf(view);
    if (tabs && index >= 0 && index < (int)tabs->titles.size()) {
        tabs->select(view, index, false);
    }
}

extern "C" void gkWin32UITabsContentSize(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    GKTabs *tabs = gkTabsOf(view);
    *width = *height = 0;
    if (!tabs) {
        return;
    }
    double x, y, w, h;
    tabs->contentFrame(view, &x, &y, &w, &h);
    *width = (int)floor(w);
    *height = (int)floor(h);
}

extern "C" void gkWin32TabsConfigure(void *pointer, int placement) {
    GKView *view = gkView(pointer);
    GKTabs *tabs = gkTabsOf(view);
    if (!tabs || tabs->placement == placement) {
        return;
    }
    tabs->placement = placement;
    gkTabsBuild(view);
}

extern "C" void gkWin32TabsSetOverflow(void *pointer, int policy) {
    GKView *view = gkView(pointer);
    GKTabs *tabs = gkTabsOf(view);
    if (!tabs || tabs->overflow == policy) {
        return;
    }
    tabs->overflow = policy;
    gkTabsBuild(view);
}

std::vector<std::pair<std::string, std::string>> gkTabItems(GKView *view) {
    std::vector<std::pair<std::string, std::string>> items;
    if (GKTabs *tabs = gkTabsOf(view)) {
        for (auto const &title : tabs->titles) {
            items.emplace_back("TabItem", gkUTF8(title));
        }
    }
    return items;
}
