//go:build windows && cgo

#include "win32_internal.h"

#include <algorithm>
#include <map>

/* A menu is a model of items, separators, and submenus. The bridge builds a
   native menu from it for each place that shows it: a context menu, a
   pop-up, or the menu bar of a window. The root menu keeps the state of each
   action, which the items of every place share. */

UINT gkVirtualKey(const char *name);
int gkAcceleratorModifiers();

struct GKMenu;
struct GKMenuBar;

struct GKMenuEntry {
    enum { ITEM, SEPARATOR, SUBMENU } type = ITEM;
    std::string id;
    std::string title;
    std::string key;
    int modifiers = 0;
    std::string tooltip;
    GKIcon *icon = nullptr;
    std::unique_ptr<GKMenu> submenu;
};

struct GKActionState {
    int enabled = 1;
    int checkable = 0;
    int checked = 0;
};

struct GKMenu : GKObject {
    GKMenu *root = nullptr;
    std::string title;
    std::vector<GKMenuEntry> entries;
    std::map<std::string, GKActionState> actions;
    GKMenuBar *inBar = nullptr;
    ~GKMenu() override {
        for (auto &entry : entries) {
            if (entry.icon) {
                gkReleaseObject(entry.icon);
            }
        }
    }
};

struct GKMenuBar : GKObject {
    GKWin *win = nullptr;
    HMENU bar = nullptr;
    int installed = 0;
    std::vector<GKMenu *> menus;
    std::vector<std::string> titles;
    /* The command of item n is ids[n - gkMenuBarFirst]. */
    std::vector<std::pair<GKMenu *, std::string>> ids;
    std::vector<HBITMAP> bitmaps;
    ~GKMenuBar() override;
};

static const UINT gkMenuBarFirst = 0x1000;

static GKMenu *gkMenu(void *pointer) {
    return static_cast<GKMenu *>(pointer);
}

static GKMenu *gkMenuRoot(GKMenu *menu) {
    return menu && menu->root ? menu->root : menu;
}

static void gkMenuSend(GKMenu *root, std::string const &identifier) {
    gkSend(root->handle, GK_EVENT_ACTION, 0, 0, 0, 0, identifier.c_str());
}

/* gkShortcutText returns the text of a shortcut in a menu, such as
   "Ctrl+Shift+K". */
static std::wstring gkShortcutText(std::string const &key, int modifiers) {
    if (key.empty()) {
        return std::wstring();
    }
    std::wstring text;
    if (modifiers & 1) {
        text += L"Ctrl+";
    }
    if (modifiers & 2) {
        text += L"Alt+";
    }
    if (modifiers & 4) {
        text += L"Shift+";
    }
    if (key == "Return") {
        text += L"Enter";
    } else if (key == "Escape") {
        text += L"Esc";
    } else if (key == "Delete") {
        text += L"Del";
    } else {
        text += gkWide(key.c_str());
    }
    return text;
}

static void gkMenuBuild(GKMenu *menu, HMENU target, std::vector<std::pair<GKMenu *, std::string>> &ids, UINT first,
                        std::vector<HBITMAP> &bitmaps) {
    GKMenu *root = gkMenuRoot(menu);
    int size = (int)lround(16 * (GetDpiForSystem() / 96.0));
    for (auto &entry : menu->entries) {
        switch (entry.type) {
        case GKMenuEntry::SEPARATOR:
            AppendMenuW(target, MF_SEPARATOR, 0, nullptr);
            break;
        case GKMenuEntry::SUBMENU: {
            HMENU sub = CreatePopupMenu();
            gkMenuBuild(entry.submenu.get(), sub, ids, first, bitmaps);
            AppendMenuW(target, MF_POPUP, reinterpret_cast<UINT_PTR>(sub), gkWide(entry.title.c_str()).c_str());
            break;
        }
        default: {
            GKActionState &state = root->actions[entry.id];
            ids.emplace_back(root, entry.id);
            std::wstring text = gkWide(entry.title.c_str());
            std::wstring shortcut = gkShortcutText(entry.key, entry.modifiers);
            if (!shortcut.empty()) {
                text += L"\t" + shortcut;
            }
            MENUITEMINFOW info = {};
            info.cbSize = sizeof(info);
            info.fMask = MIIM_ID | MIIM_STRING | MIIM_STATE | MIIM_FTYPE;
            info.fType = MFT_STRING;
            info.wID = first + (UINT)ids.size() - 1;
            info.dwTypeData = const_cast<wchar_t *>(text.c_str());
            info.fState = (state.enabled ? MFS_ENABLED : MFS_DISABLED) |
                          (state.checkable && state.checked ? MFS_CHECKED : MFS_UNCHECKED);
            if (entry.icon) {
                HBITMAP bitmap = gkIconBitmap(entry.icon, size, gkPalette(gkResolvedTheme()).text);
                if (bitmap) {
                    info.fMask |= MIIM_BITMAP;
                    info.hbmpItem = bitmap;
                    bitmaps.push_back(bitmap);
                }
            }
            InsertMenuItemW(target, GetMenuItemCount(target), TRUE, &info);
            break;
        }
        }
    }
}

/* gkMenuInvoke performs an action of a menu as a click on its item does. */
void gkMenuInvoke(void *pointer, std::string const &identifier) {
    GKMenu *root = gkMenuRoot(gkMenu(pointer));
    if (!root) {
        return;
    }
    GKActionState &state = root->actions[identifier];
    if (!state.enabled) {
        return;
    }
    if (state.checkable) {
        state.checked = !state.checked;
    }
    gkMenuSend(root, identifier);
}

/* gkMenuNative builds a pop-up menu. Command n is ids[n - 1]. */
HMENU gkMenuNative(void *pointer, std::vector<std::string> &ids) {
    GKMenu *menu = gkMenu(pointer);
    if (!menu) {
        return nullptr;
    }
    HMENU popup = CreatePopupMenu();
    std::vector<std::pair<GKMenu *, std::string>> entries;
    std::vector<HBITMAP> bitmaps;
    gkMenuBuild(menu, popup, entries, 1, bitmaps);
    for (auto const &entry : entries) {
        ids.push_back(entry.second);
    }
    return popup;
}

void gkMenuPopupAt(GKObject *object, HWND owner, POINT screen) {
    GKMenu *menu = static_cast<GKMenu *>(object);
    if (!menu) {
        return;
    }
    HMENU popup = CreatePopupMenu();
    std::vector<std::pair<GKMenu *, std::string>> ids;
    std::vector<HBITMAP> bitmaps;
    gkMenuBuild(menu, popup, ids, 1, bitmaps);
    HWND top = owner ? GetAncestor(owner, GA_ROOT) : nullptr;
    UINT command = (UINT)TrackPopupMenuEx(popup, TPM_RETURNCMD | TPM_RIGHTBUTTON | TPM_NONOTIFY, screen.x, screen.y,
                                          top ? top : owner, nullptr);
    DestroyMenu(popup);
    for (HBITMAP bitmap : bitmaps) {
        DeleteObject(bitmap);
    }
    if (command >= 1 && command <= ids.size()) {
        gkMenuInvoke(ids[command - 1].first, ids[command - 1].second);
    }
}

static void gkMenuBarRebuild(GKMenuBar *bar);

static void gkMenuChanged(GKMenu *menu) {
    GKMenu *root = gkMenuRoot(menu);
    if (root && root->inBar) {
        gkMenuBarRebuild(root->inBar);
    }
}

extern "C" void *gkWin32MenuCreate(uintptr_t handle, const char *title) {
    GKMenu *menu = new GKMenu();
    menu->handle = handle;
    menu->title = title ? title : "";
    return menu;
}

extern "C" void gkWin32MenuSetHandle(void *pointer, uintptr_t handle) {
    GKMenu *menu = gkMenuRoot(gkMenu(pointer));
    if (menu) {
        menu->handle = handle;
    }
}

extern "C" void gkWin32MenuClear(void *pointer) {
    GKMenu *menu = gkMenu(pointer);
    if (!menu) {
        return;
    }
    for (auto &entry : menu->entries) {
        if (entry.icon) {
            gkReleaseObject(entry.icon);
        }
    }
    menu->entries.clear();
    if (!menu->root) {
        /* A root menu drops its actions; submenus are rebuilt with it. */
        menu->actions.clear();
    }
    gkMenuChanged(menu);
}

extern "C" void gkWin32MenuAddItem(void *pointer, const char *actionID, const char *title, int enabled, int checked,
                                   const char *key, int modifiers, void *icon, const char *tooltip) {
    GKMenu *menu = gkMenu(pointer);
    if (!menu) {
        return;
    }
    GKMenu *root = gkMenuRoot(menu);
    GKMenuEntry entry;
    entry.id = actionID ? actionID : "";
    entry.title = title ? title : "";
    entry.key = key ? key : "";
    entry.modifiers = modifiers;
    entry.tooltip = tooltip ? tooltip : "";
    if (icon) {
        entry.icon = static_cast<GKIcon *>(icon);
        gkRetainObject(entry.icon);
    }
    GKActionState &state = root->actions[entry.id];
    state.enabled = enabled != 0;
    if (checked) {
        state.checkable = 1;
        state.checked = 1;
    }
    menu->entries.push_back(std::move(entry));
    gkMenuChanged(menu);
}

extern "C" void gkWin32MenuAddSeparator(void *pointer) {
    GKMenu *menu = gkMenu(pointer);
    if (menu) {
        GKMenuEntry entry;
        entry.type = GKMenuEntry::SEPARATOR;
        menu->entries.push_back(std::move(entry));
        gkMenuChanged(menu);
    }
}

extern "C" void *gkWin32MenuAddSubmenu(void *pointer, const char *title) {
    GKMenu *menu = gkMenu(pointer);
    if (!menu) {
        return nullptr;
    }
    GKMenuEntry entry;
    entry.type = GKMenuEntry::SUBMENU;
    entry.title = title ? title : "";
    entry.submenu.reset(new GKMenu());
    entry.submenu->root = gkMenuRoot(menu);
    entry.submenu->title = entry.title;
    GKMenu *submenu = entry.submenu.get();
    menu->entries.push_back(std::move(entry));
    gkMenuChanged(menu);
    /* The parent menu owns the submenu. */
    return submenu;
}

extern "C" void gkWin32MenuSetActionState(void *pointer, const char *actionID, int enabled, int checked) {
    GKMenu *root = gkMenuRoot(gkMenu(pointer));
    if (!root) {
        return;
    }
    auto found = root->actions.find(actionID ? actionID : "");
    if (found == root->actions.end()) {
        return;
    }
    GKActionState &state = found->second;
    if (enabled >= 0) {
        state.enabled = enabled != 0;
    }
    if (checked > 0) {
        state.checkable = 1;
    }
    if (checked >= 0 && state.checkable) {
        state.checked = checked != 0;
    }
    gkMenuChanged(root);
}

extern "C" void gkWin32MenuPopup(void *pointer, void *viewPointer, double x, double y) {
    GKView *view = gkView(viewPointer);
    if (!view || !view->outer) {
        return;
    }
    double scale = gkViewScaleOf(view);
    POINT point = {(LONG)lround(x * scale), (LONG)lround(y * scale)};
    ClientToScreen(view->outer, &point);
    gkMenuPopupAt(static_cast<GKObject *>(pointer), view->outer, point);
}

/* ---- Menu bar ---- */

GKMenuBar::~GKMenuBar() {
    for (GKMenu *menu : menus) {
        menu->inBar = nullptr;
        gkReleaseObject(menu);
    }
    if (bar) {
        DestroyMenu(bar);
    }
    for (HBITMAP bitmap : bitmaps) {
        DeleteObject(bitmap);
    }
}

static std::vector<GKMenuBar *> &gkMenuBars() {
    static std::vector<GKMenuBar *> bars;
    return bars;
}

/* gkMenuBarSet shows a menu on a window, or none, and keeps the size of the
   content of the window. */
static void gkMenuBarSet(GKWin *win, HMENU menu) {
    int width = 0, height = 0;
    gkWin32WindowSize(win, &width, &height);
    win->menubar = menu;
    if (!win->fullscreen) {
        SetMenu(win->hwnd, menu);
        if (!IsZoomed(win->hwnd) && width > 0 && height > 0) {
            extern void gkWin32WindowSetSize(void *window, int width, int height);
            gkWin32WindowSetSize(win, width, height);
        }
    }
    DrawMenuBar(win->hwnd);
}

static void gkMenuBarRebuild(GKMenuBar *bar) {
    HMENU fresh = CreateMenu();
    std::vector<std::pair<GKMenu *, std::string>> ids;
    std::vector<HBITMAP> bitmaps;
    for (size_t i = 0; i < bar->menus.size(); i++) {
        HMENU popup = CreatePopupMenu();
        gkMenuBuild(bar->menus[i], popup, ids, gkMenuBarFirst, bitmaps);
        std::wstring title = gkWide(i < bar->titles.size() ? bar->titles[i].c_str() : "");
        AppendMenuW(fresh, MF_POPUP, reinterpret_cast<UINT_PTR>(popup), title.c_str());
    }
    HMENU old = bar->bar;
    bar->bar = fresh;
    bar->ids = ids;
    for (HBITMAP bitmap : bar->bitmaps) {
        DeleteObject(bitmap);
    }
    bar->bitmaps = bitmaps;
    if (bar->installed && bar->win && !bar->win->destroyed) {
        bar->win->menubar = fresh;
        if (!bar->win->fullscreen) {
            SetMenu(bar->win->hwnd, fresh);
        }
        DrawMenuBar(bar->win->hwnd);
    }
    if (old) {
        DestroyMenu(old);
    }
}

extern "C" void *gkWin32MenuBarCreate(void *window, const char *titles, size_t titlesSize, void **menus, int count) {
    GKWin *win = static_cast<GKWin *>(window);
    if (!win) {
        return nullptr;
    }
    for (int i = 0; i < count; i++) {
        if (!menus[i] || gkMenu(menus[i])->inBar) {
            return nullptr;
        }
    }
    GKMenuBar *bar = new GKMenuBar();
    bar->win = win;
    bar->titles = gkSplit(titles, titlesSize);
    for (int i = 0; i < count; i++) {
        GKMenu *menu = gkMenu(menus[i]);
        gkRetainObject(menu);
        menu->inBar = bar;
        bar->menus.push_back(menu);
    }
    gkMenuBarRebuild(bar);
    gkMenuBars().push_back(bar);
    return bar;
}

extern "C" void gkWin32MenuBarInstall(void *pointer) {
    GKMenuBar *bar = static_cast<GKMenuBar *>(pointer);
    if (!bar || !bar->win) {
        return;
    }
    bar->installed = 1;
    gkMenuBarSet(bar->win, bar->bar);
}

extern "C" void gkWin32MenuBarRelease(void *pointer) {
    GKMenuBar *bar = static_cast<GKMenuBar *>(pointer);
    if (!bar) {
        return;
    }
    auto &bars = gkMenuBars();
    bars.erase(std::remove(bars.begin(), bars.end(), bar), bars.end());
    if (bar->installed && bar->win && !bar->win->destroyed && bar->win->menubar == bar->bar) {
        gkMenuBarSet(bar->win, nullptr);
    }
    gkReleaseObject(bar);
}

/* gkMenuCommand performs the item of a menu bar that a WM_COMMAND
   names. */
bool gkMenuCommand(HWND hwnd, int id) {
    for (GKMenuBar *bar : gkMenuBars()) {
        if (!bar->installed || !bar->win || bar->win->hwnd != hwnd) {
            continue;
        }
        UINT index = (UINT)id - gkMenuBarFirst;
        if (id >= (int)gkMenuBarFirst && index < bar->ids.size()) {
            auto entry = bar->ids[index];
            gkMenuInvoke(entry.first, entry.second);
            gkMenuBarRebuild(bar);
            return true;
        }
    }
    return false;
}

static bool gkMenuShortcut(GKMenu *menu, UINT key, int modifiers, std::string *found, GKMenu **owner) {
    for (auto &entry : menu->entries) {
        if (entry.type == GKMenuEntry::SUBMENU) {
            if (gkMenuShortcut(entry.submenu.get(), key, modifiers, found, owner)) {
                return true;
            }
        } else if (entry.type == GKMenuEntry::ITEM && !entry.key.empty() && gkVirtualKey(entry.key.c_str()) == key &&
                   entry.modifiers == modifiers) {
            *found = entry.id;
            *owner = gkMenuRoot(menu);
            return true;
        }
    }
    return false;
}

/* gkMenuAccelerator performs the item of the menu bar of a window whose
   shortcut is the key that a message presses. */
bool gkMenuAccelerator(GKWin *win, MSG const *message) {
    if (message->message != WM_KEYDOWN && message->message != WM_SYSKEYDOWN) {
        return false;
    }
    int modifiers = gkAcceleratorModifiers();
    if (!(modifiers & 3) && !(message->wParam >= VK_F1 && message->wParam <= VK_F24)) {
        /* A shortcut without Control or Alt is typing, except for function
           keys. */
        return false;
    }
    for (GKMenuBar *bar : gkMenuBars()) {
        if (!bar->installed || bar->win != win) {
            continue;
        }
        for (GKMenu *menu : bar->menus) {
            std::string id;
            GKMenu *owner = nullptr;
            if (gkMenuShortcut(menu, (UINT)message->wParam, modifiers, &id, &owner)) {
                gkMenuInvoke(owner, id);
                gkMenuBarRebuild(bar);
                return true;
            }
        }
    }
    return false;
}
