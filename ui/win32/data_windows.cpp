//go:build windows && cgo

#include "win32_internal.h"

#include <uxtheme.h>

#include <algorithm>

std::wstring gkLinkMarkup(std::wstring const &text);

static void gkDataReport(GKView *view, int kind, int a, const char *text) {
    if (view && !view->suppress) {
        gkSend(view->handle, kind, a, 0, 0, 0, text);
    }
}

static COLORREF gkColorRef(GKColor const &color) {
    return RGB((int)lroundf(color.r * 255), (int)lroundf(color.g * 255), (int)lroundf(color.b * 255));
}

/* gkImageList makes an image list of icons at a size in logical units. */
static HIMAGELIST gkImageList(HWND owner, void **icons, int count, int width, int height, GKColor const &color) {
    bool any = false;
    for (int i = 0; i < count && icons; i++) {
        any |= icons[i] != nullptr;
    }
    if (!any) {
        return nullptr;
    }
    double scale = gkScaleOf(owner);
    int w = (int)lround(width * scale), h = (int)lround(height * scale);
    int side = std::min(w, h);
    HIMAGELIST list = gkComctl().ImageList_Create(w, h, ILC_COLOR32, count, 1);
    for (int i = 0; i < count; i++) {
        std::vector<unsigned char> pixels =
            icons[i] ? gkIconPixels(static_cast<GKIcon *>(icons[i]), side, color) : std::vector<unsigned char>();
        BITMAPINFO info = {};
        info.bmiHeader.biSize = sizeof(info.bmiHeader);
        info.bmiHeader.biWidth = w;
        info.bmiHeader.biHeight = -h;
        info.bmiHeader.biPlanes = 1;
        info.bmiHeader.biBitCount = 32;
        void *bits = nullptr;
        HDC screen = GetDC(nullptr);
        HBITMAP bitmap = CreateDIBSection(screen, &info, DIB_RGB_COLORS, &bits, nullptr, 0);
        ReleaseDC(nullptr, screen);
        if (!bitmap) {
            continue;
        }
        unsigned char *out = static_cast<unsigned char *>(bits);
        memset(out, 0, (size_t)w * h * 4);
        int ox = (w - side) / 2, oy = (h - side) / 2;
        for (int y = 0; y < side && !pixels.empty(); y++) {
            for (int x = 0; x < side; x++) {
                const unsigned char *p = &pixels[((size_t)y * side + x) * 4];
                unsigned char *q = &out[((size_t)(y + oy) * w + (x + ox)) * 4];
                unsigned int a = p[3];
                q[0] = (unsigned char)(p[2] * a / 255);
                q[1] = (unsigned char)(p[1] * a / 255);
                q[2] = (unsigned char)(p[0] * a / 255);
                q[3] = (unsigned char)a;
            }
        }
        gkComctl().ImageList_Add(list, bitmap, nullptr);
        DeleteObject(bitmap);
    }
    return list;
}

static void gkListColors(HWND list, GKPalette const &palette) {
    SendMessageW(list, LVM_SETBKCOLOR, 0, (LPARAM)gkColorRef(palette.dark ? palette.layer : palette.window));
    SendMessageW(list, LVM_SETTEXTBKCOLOR, 0, (LPARAM)gkColorRef(palette.dark ? palette.layer : palette.window));
    SendMessageW(list, LVM_SETTEXTCOLOR, 0, (LPARAM)gkColorRef(palette.text));
    HWND header = (HWND)SendMessageW(list, LVM_GETHEADER, 0, 0);
    if (header) {
        SetWindowTheme(header, palette.dark ? L"DarkMode_ItemsView" : L"ItemsView", nullptr);
    }
}

/* ---- Tables and lists ---- */

struct GKTable : GKExtension {
    int isList = 0;
    int selection = 1;
    std::vector<std::string> columns;
    std::vector<std::string> ids;
    std::vector<std::wstring> details;
    std::string sortColumn;
    int ascending = 1;
    HIMAGELIST images = nullptr;
    ~GKTable() override {
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
    }
    void showSort(GKView *view) {
        HWND header = (HWND)SendMessageW(view->outer, LVM_GETHEADER, 0, 0);
        for (size_t i = 0; i < columns.size(); i++) {
            HDITEMW item = {};
            item.mask = HDI_FORMAT;
            SendMessageW(header, HDM_GETITEMW, i, (LPARAM)&item);
            item.fmt &= ~(HDF_SORTUP | HDF_SORTDOWN);
            if (columns[i] == sortColumn && !sortColumn.empty()) {
                item.fmt |= ascending ? HDF_SORTUP : HDF_SORTDOWN;
            }
            SendMessageW(header, HDM_SETITEMW, i, (LPARAM)&item);
        }
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        switch (header->code) {
        case LVN_ITEMCHANGING:
            if (selection == 0) {
                NMLISTVIEW *change = reinterpret_cast<NMLISTVIEW *>(header);
                if ((change->uNewState & LVIS_SELECTED) && !(change->uOldState & LVIS_SELECTED)) {
                    *result = TRUE;
                    return true;
                }
            }
            return false;
        case LVN_ITEMCHANGED: {
            NMLISTVIEW *change = reinterpret_cast<NMLISTVIEW *>(header);
            if ((change->uChanged & LVIF_STATE) &&
                ((change->uNewState ^ change->uOldState) & LVIS_SELECTED)) {
                gkDataReport(view, GK_EVENT_SELECTION, 0, nullptr);
            }
            *result = 0;
            return true;
        }
        case LVN_ITEMACTIVATE: {
            NMITEMACTIVATE *activate = reinterpret_cast<NMITEMACTIVATE *>(header);
            int index = activate->iItem;
            if (index < 0) {
                index = (int)SendMessageW(view->outer, LVM_GETNEXTITEM, (WPARAM)-1, LVNI_FOCUSED);
            }
            if (index >= 0 && index < (int)ids.size()) {
                gkSend(view->handle, GK_EVENT_ACTIVATE, 0, 0, 0, 0, ids[index].c_str());
            }
            *result = 0;
            return true;
        }
        case LVN_COLUMNCLICK: {
            NMLISTVIEW *click = reinterpret_cast<NMLISTVIEW *>(header);
            int column = click->iSubItem;
            if (isList || column < 0 || column >= (int)columns.size() || !sortable[column]) {
                return false;
            }
            if (sortColumn == columns[column]) {
                ascending = !ascending;
            } else {
                sortColumn = columns[column];
                ascending = 1;
            }
            showSort(view);
            gkDataReport(view, GK_EVENT_SORT, ascending, sortColumn.c_str());
            *result = 0;
            return true;
        }
        case NM_CUSTOMDRAW: {
            if (!isList) {
                return false;
            }
            /* The detail of a row in a list is secondary text. */
            NMLVCUSTOMDRAW *draw = reinterpret_cast<NMLVCUSTOMDRAW *>(header);
            if (draw->nmcd.dwDrawStage == CDDS_PREPAINT) {
                *result = CDRF_NOTIFYITEMDRAW;
                return true;
            }
            if (draw->nmcd.dwDrawStage == CDDS_ITEMPREPAINT) {
                *result = CDRF_NOTIFYSUBITEMDRAW;
                return true;
            }
            if (draw->nmcd.dwDrawStage == (CDDS_ITEMPREPAINT | CDDS_SUBITEM)) {
                GKPalette const &palette = gkViewPalette(view);
                draw->clrText = gkColorRef(draw->iSubItem == 1 ? palette.textSecondary : palette.text);
                *result = CDRF_NEWFONT;
                return true;
            }
            return false;
        }
        default:
            return false;
        }
    }
    std::vector<int> sortable;
    void themeChanged(GKView *view) override {
        gkApplyThemeTo(view->outer, gkViewTheme(view));
        gkListColors(view->outer, gkViewPalette(view));
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(320, 200);
    }
};

static GKTable *gkTableOf(GKView *view) {
    return view && view->kind == GK_KIND_TABLE ? static_cast<GKTable *>(view->ext.get()) : nullptr;
}

extern "C" void *gkWin32TableCreate(uintptr_t window, int list, const char *columnIDs, size_t columnIDsSize,
                                    const char *titles, size_t titlesSize, const int *widths, const int *sortable,
                                    int selection) {
    GKTable *table = new GKTable();
    table->isList = list != 0;
    table->selection = selection;
    table->columns = gkSplit(columnIDs, columnIDsSize);
    if (table->columns.empty()) {
        table->columns.push_back("");
    }
    DWORD style = LVS_REPORT | LVS_SHOWSELALWAYS | WS_TABSTOP | (selection == 2 ? 0 : LVS_SINGLESEL);
    if (list) {
        style |= LVS_NOCOLUMNHEADER;
    }
    HWND hwnd = gkMakeNative(WC_LISTVIEWW, L"", style, WS_EX_CLIENTEDGE);
    GKView *view = gkAdopt(window, hwnd, nullptr, GK_KIND_TABLE, 0);
    view->ext.reset(table);
    SendMessageW(hwnd, LVM_SETEXTENDEDLISTVIEWSTYLE, 0,
                 LVS_EX_FULLROWSELECT | LVS_EX_DOUBLEBUFFER | LVS_EX_HEADERDRAGDROP | LVS_EX_INFOTIP);
    auto names = gkSplit(titles, titlesSize);
    int count = list ? 2 : (int)table->columns.size();
    for (int i = 0; i < count; i++) {
        std::wstring name = gkWide(i < (int)names.size() && !list ? names[i].c_str() : "");
        LVCOLUMNW column = {};
        column.mask = LVCF_TEXT | LVCF_WIDTH | LVCF_SUBITEM;
        column.pszText = const_cast<wchar_t *>(name.c_str());
        int width = !list && widths && widths[i] > 0 ? widths[i] : list ? (i == 0 ? 200 : 120) : 120;
        column.cx = (int)lround(width * gkScaleOf(hwnd));
        column.iSubItem = i;
        SendMessageW(hwnd, LVM_INSERTCOLUMNW, i, (LPARAM)&column);
        table->sortable.push_back(!list && sortable && sortable[i]);
    }
    table->themeChanged(view);
    return view;
}

static std::vector<std::string> gkSelectedIDs(HWND list, std::vector<std::string> const &ids) {
    std::vector<std::string> result;
    for (int index = -1; (index = (int)SendMessageW(list, LVM_GETNEXTITEM, (WPARAM)index, LVNI_SELECTED)) >= 0;) {
        if (index < (int)ids.size()) {
            result.push_back(ids[index]);
        }
    }
    return result;
}

static void gkSelectIDs(HWND list, std::vector<std::string> const &ids, std::vector<std::string> const &wanted,
                        bool single) {
    LVITEMW clear = {};
    clear.stateMask = LVIS_SELECTED;
    clear.state = 0;
    SendMessageW(list, LVM_SETITEMSTATE, (WPARAM)-1, (LPARAM)&clear);
    for (size_t i = 0; i < ids.size(); i++) {
        if (std::find(wanted.begin(), wanted.end(), ids[i]) == wanted.end()) {
            continue;
        }
        LVITEMW item = {};
        item.stateMask = LVIS_SELECTED | (single ? LVIS_FOCUSED : 0);
        item.state = item.stateMask;
        SendMessageW(list, LVM_SETITEMSTATE, i, (LPARAM)&item);
        if (single) {
            break;
        }
    }
}

extern "C" void gkWin32TableSetRows(void *pointer, const char *ids, size_t idsSize, const char *cells,
                                    size_t cellsSize, const char *details, size_t detailsSize, void **icons,
                                    int columns) {
    GKView *view = gkView(pointer);
    GKTable *table = gkTableOf(view);
    if (!table) {
        return;
    }
    HWND list = view->outer;
    auto rowIDs = gkSplit(ids, idsSize);
    auto values = gkSplit(cells, cellsSize);
    auto rowDetails = gkSplit(details, detailsSize);
    int count = columns > 0 ? columns : 1;
    std::vector<std::string> selected = gkSelectedIDs(list, table->ids);
    view->suppress++;
    SendMessageW(list, WM_SETREDRAW, FALSE, 0);
    SendMessageW(list, LVM_DELETEALLITEMS, 0, 0);
    table->ids = rowIDs;
    if (table->images) {
        SendMessageW(list, LVM_SETIMAGELIST, LVSIL_SMALL, 0);
        gkComctl().ImageList_Destroy(table->images);
        table->images = nullptr;
    }
    if (table->isList && icons) {
        table->images = gkImageList(list, icons, (int)rowIDs.size(), 16, 16, gkViewPalette(view).text);
        SendMessageW(list, LVM_SETIMAGELIST, LVSIL_SMALL, (LPARAM)table->images);
    }
    for (size_t i = 0; i < rowIDs.size(); i++) {
        std::wstring first = gkWide(i * count < values.size() ? values[i * count].c_str() : "");
        LVITEMW item = {};
        item.mask = LVIF_TEXT | (table->images ? LVIF_IMAGE : 0);
        item.iItem = (int)i;
        item.pszText = const_cast<wchar_t *>(first.c_str());
        item.iImage = (int)i;
        SendMessageW(list, LVM_INSERTITEMW, 0, (LPARAM)&item);
        if (table->isList) {
            std::wstring detail = gkWide(i < rowDetails.size() ? rowDetails[i].c_str() : "");
            LVITEMW sub = {};
            sub.iSubItem = 1;
            sub.pszText = const_cast<wchar_t *>(detail.c_str());
            SendMessageW(list, LVM_SETITEMTEXTW, i, (LPARAM)&sub);
            continue;
        }
        for (int column = 1; column < count && column < (int)table->columns.size(); column++) {
            size_t index = i * count + column;
            std::wstring text = gkWide(index < values.size() ? values[index].c_str() : "");
            LVITEMW sub = {};
            sub.iSubItem = column;
            sub.pszText = const_cast<wchar_t *>(text.c_str());
            SendMessageW(list, LVM_SETITEMTEXTW, i, (LPARAM)&sub);
        }
    }
    if (table->isList) {
        SendMessageW(list, LVM_SETCOLUMNWIDTH, 1, LVSCW_AUTOSIZE_USEHEADER);
    }
    gkSelectIDs(list, table->ids, selected, table->selection != 2);
    SendMessageW(list, WM_SETREDRAW, TRUE, 0);
    InvalidateRect(list, nullptr, TRUE);
    view->suppress--;
}

extern "C" char *gkWin32TableSelection(void *pointer, size_t *size) {
    GKView *view = gkView(pointer);
    GKTable *table = gkTableOf(view);
    *size = 0;
    if (!table) {
        return nullptr;
    }
    std::vector<std::string> ids = gkSelectedIDs(view->outer, table->ids);
    return ids.empty() ? nullptr : gkJoin(ids, size);
}

extern "C" void gkWin32TableSelect(void *pointer, const char *ids, size_t size) {
    GKView *view = gkView(pointer);
    GKTable *table = gkTableOf(view);
    if (!table || table->selection == 0) {
        return;
    }
    view->suppress++;
    gkSelectIDs(view->outer, table->ids, gkSplit(ids, size), table->selection != 2);
    view->suppress--;
}

extern "C" int gkWin32TableScrollTo(void *pointer, const char *identifier) {
    GKView *view = gkView(pointer);
    GKTable *table = gkTableOf(view);
    if (!table) {
        return 0;
    }
    for (size_t i = 0; i < table->ids.size(); i++) {
        if (table->ids[i] == (identifier ? identifier : "")) {
            SendMessageW(view->outer, LVM_ENSUREVISIBLE, i, FALSE);
            return 1;
        }
    }
    return 0;
}

/* ---- Outlines: trees and navigation lists ---- */

struct GKNode {
    std::string id;
    std::wstring text;
    std::wstring badge;
    int hasChildren = 0;
    int group = 0;
    int loaded = 0;
    HTREEITEM item = nullptr;
    GKNode *parent = nullptr;
};

struct GKOutline : GKExtension {
    int navigation = 0;
    int shown = 0;
    int multiple = 0;
    std::vector<std::unique_ptr<GKNode>> nodes;
    std::vector<GKNode *> selected;
    HIMAGELIST images = nullptr;
    std::vector<GKIcon *> icons;
    ~GKOutline() override {
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
        for (GKIcon *icon : icons) {
            gkReleaseObject(icon);
        }
    }
    GKNode *nodeOf(HWND tree, HTREEITEM item) {
        if (!item) {
            return nullptr;
        }
        TVITEMW info = {};
        info.mask = TVIF_PARAM | TVIF_HANDLE;
        info.hItem = item;
        TreeView_GetItem(tree, &info);
        GKNode *node = reinterpret_cast<GKNode *>(info.lParam);
        for (auto const &record : nodes) {
            if (record.get() == node) {
                return node;
            }
        }
        return nullptr;
    }
    GKNode *find(const char *identifier) {
        for (auto const &node : nodes) {
            if (node->item && node->id == (identifier ? identifier : "")) {
                return node.get();
            }
        }
        return nullptr;
    }
    void mark(HWND tree, GKNode *node, bool on) {
        TVITEMW info = {};
        info.mask = TVIF_STATE | TVIF_HANDLE;
        info.hItem = node->item;
        info.stateMask = TVIS_SELECTED;
        info.state = on ? TVIS_SELECTED : 0;
        TreeView_SetItem(tree, &info);
    }
    void reportSelection(GKView *view) {
        gkDataReport(view, GK_EVENT_SELECTION, 0, nullptr);
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        HWND tree = view->outer;
        switch (header->code) {
        case TVN_SELCHANGINGW: {
            NMTREEVIEWW *change = reinterpret_cast<NMTREEVIEWW *>(header);
            GKNode *node = nodeOf(tree, change->itemNew.hItem);
            /* A group heading is not selectable. */
            if (node && node->group && !view->suppress) {
                *result = TRUE;
                return true;
            }
            return false;
        }
        case TVN_SELCHANGEDW: {
            NMTREEVIEWW *change = reinterpret_cast<NMTREEVIEWW *>(header);
            GKNode *node = nodeOf(tree, change->itemNew.hItem);
            if (!view->suppress) {
                bool add = multiple && (GetKeyState(VK_CONTROL) & 0x8000);
                if (!add) {
                    for (GKNode *other : selected) {
                        if (other != node && other->item) {
                            mark(tree, other, false);
                        }
                    }
                    selected.clear();
                }
                if (node) {
                    auto found = std::find(selected.begin(), selected.end(), node);
                    if (found == selected.end()) {
                        selected.push_back(node);
                    } else if (add) {
                        selected.erase(found);
                    }
                }
                for (GKNode *other : selected) {
                    mark(tree, other, true);
                }
                reportSelection(view);
            }
            *result = 0;
            return true;
        }
        case TVN_ITEMEXPANDINGW: {
            NMTREEVIEWW *change = reinterpret_cast<NMTREEVIEWW *>(header);
            GKNode *node = nodeOf(tree, change->itemNew.hItem);
            if (node && change->action == TVE_EXPAND && node->hasChildren && !node->loaded &&
                !TreeView_GetChild(tree, node->item)) {
                gkDataReport(view, GK_EVENT_EXPAND, 1, node->id.c_str());
            }
            *result = 0;
            return true;
        }
        case NM_CUSTOMDRAW: {
            NMTVCUSTOMDRAW *draw = reinterpret_cast<NMTVCUSTOMDRAW *>(header);
            GKPalette const &palette = gkViewPalette(view);
            if (draw->nmcd.dwDrawStage == CDDS_PREPAINT) {
                *result = CDRF_NOTIFYITEMDRAW;
                return true;
            }
            GKNode *node = nodeOf(tree, (HTREEITEM)draw->nmcd.dwItemSpec);
            if (draw->nmcd.dwDrawStage == CDDS_ITEMPREPAINT) {
                if (node && node->group && navigation) {
                    /* The heading of a group sits at the bottom of its row,
                       so the room of the row above it separates the group
                       from the previous one. */
                    HDC hdc = draw->nmcd.hdc;
                    RECT row = draw->nmcd.rc;
                    HBRUSH background = CreateSolidBrush(TreeView_GetBkColor(tree));
                    FillRect(hdc, &row, background);
                    DeleteObject(background);
                    RECT label = {};
                    TreeView_GetItemRect(tree, (HTREEITEM)draw->nmcd.dwItemSpec, &label, TRUE);
                    GKFont font = gkBodyFont();
                    font.weight = 600;
                    HGDIOBJ previous = SelectObject(hdc, gkFontHandle(font, GetDpiForWindow(tree)));
                    SetBkMode(hdc, TRANSPARENT);
                    SetTextColor(hdc, gkColorRef(palette.textSecondary));
                    RECT text = {label.left, row.top, row.right, row.bottom - (int)lround(2 * gkScaleOf(tree))};
                    DrawTextW(hdc, node->text.c_str(), (int)node->text.size(), &text,
                              DT_LEFT | DT_BOTTOM | DT_SINGLELINE | DT_NOPREFIX | DT_END_ELLIPSIS);
                    SelectObject(hdc, previous);
                    *result = CDRF_SKIPDEFAULT;
                    return true;
                }
                if (node && node->group) {
                    draw->clrText = gkColorRef(palette.textSecondary);
                }
                *result = CDRF_NOTIFYPOSTPAINT | CDRF_NEWFONT;
                return true;
            }
            if (draw->nmcd.dwDrawStage == CDDS_ITEMPOSTPAINT && node && !node->badge.empty()) {
                /* A badge is secondary text at the end of the row. */
                RECT row = draw->nmcd.rc;
                RECT client;
                GetClientRect(tree, &client);
                row.right = client.right - (int)lround(8 * gkScaleOf(tree));
                SetBkMode(draw->nmcd.hdc, TRANSPARENT);
                SetTextColor(draw->nmcd.hdc, gkColorRef(palette.textSecondary));
                DrawTextW(draw->nmcd.hdc, node->badge.c_str(), (int)node->badge.size(), &row,
                          DT_RIGHT | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX);
                *result = CDRF_DODEFAULT;
                return true;
            }
            return false;
        }
        default:
            return false;
        }
    }
    void layout(GKView *view) override {
        /* A navigation list shows its top, with the selected item, once it
           has a size: a selection made before scrolls the item to the top. */
        if (!navigation || view->height <= 0 || shown) {
            return;
        }
        shown = 1;
        HWND tree = view->outer;
        if (HTREEITEM root = TreeView_GetRoot(tree)) {
            TreeView_SelectSetFirstVisible(tree, root);
        }
        if (HTREEITEM selection = TreeView_GetSelection(tree)) {
            TreeView_EnsureVisible(tree, selection);
        }
    }
    void themeChanged(GKView *view) override {
        GKPalette const &palette = gkViewPalette(view);
        gkApplyThemeTo(view->outer, gkViewTheme(view));
        TreeView_SetBkColor(view->outer, gkColorRef(navigation && !palette.dark ? palette.window
                                                    : palette.dark                ? palette.layer
                                                                                  : palette.window));
        TreeView_SetTextColor(view->outer, gkColorRef(palette.text));
        /* The glyphs of the icons take the text color of the theme. */
        if (!icons.empty()) {
            std::vector<void *> all(icons.begin(), icons.end());
            HIMAGELIST previous = images;
            images = gkImageList(view->outer, all.data(), (int)all.size(), 16, 16, palette.text);
            TreeView_SetImageList(view->outer, images, TVSIL_NORMAL);
            if (previous) {
                gkComctl().ImageList_Destroy(previous);
            }
        }
        InvalidateRect(view->outer, nullptr, TRUE);
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(200, 240);
    }
};

static GKOutline *gkOutlineOf(GKView *view) {
    return view && view->kind == GK_KIND_OUTLINE ? static_cast<GKOutline *>(view->ext.get()) : nullptr;
}

extern "C" void *gkWin32OutlineCreate(uintptr_t window, int navigation, int multiple) {
    GKOutline *outline = new GKOutline();
    outline->navigation = navigation != 0;
    outline->multiple = multiple != 0;
    DWORD style = WS_TABSTOP | TVS_SHOWSELALWAYS | TVS_HASBUTTONS | TVS_TRACKSELECT;
    if (navigation) {
        style |= TVS_FULLROWSELECT | TVS_NOHSCROLL;
    } else {
        style |= TVS_LINESATROOT;
    }
    HWND hwnd = gkMakeNative(WC_TREEVIEWW, L"", style, navigation ? 0 : WS_EX_CLIENTEDGE);
    GKView *view = gkAdopt(window, hwnd, nullptr, GK_KIND_OUTLINE, 0);
    view->ext.reset(outline);
    SendMessageW(hwnd, TVM_SETEXTENDEDSTYLE, TVS_EX_DOUBLEBUFFER | TVS_EX_FADEINOUTEXPANDOS,
                 TVS_EX_DOUBLEBUFFER | TVS_EX_FADEINOUTEXPANDOS);
    double rowHeight = (gkMeasureGDI(L"Ag", gkBodyFont()).cy + (navigation ? 12 : 6)) * gkScaleOf(hwnd);
    SendMessageW(hwnd, TVM_SETITEMHEIGHT, (WPARAM)lround(rowHeight), 0);
    outline->themeChanged(view);
    return view;
}

static void gkOutlineForget(GKOutline *outline, HWND tree, HTREEITEM item) {
    for (HTREEITEM child = TreeView_GetChild(tree, item); child; child = TreeView_GetNextSibling(tree, child)) {
        gkOutlineForget(outline, tree, child);
    }
    if (GKNode *node = outline->nodeOf(tree, item)) {
        node->item = nullptr;
        outline->selected.erase(std::remove(outline->selected.begin(), outline->selected.end(), node),
                                outline->selected.end());
    }
}

extern "C" void gkWin32OutlineSetNodes(void *pointer, const char *parent, const int *depths, const int *flags,
                                       const char *ids, size_t idsSize, const char *texts, size_t textsSize,
                                       const char *badges, size_t badgesSize, void **icons, int count) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutlineOf(view);
    if (!outline) {
        return;
    }
    HWND tree = view->outer;
    auto nodeIDs = gkSplit(ids, idsSize);
    auto nodeTexts = gkSplit(texts, textsSize);
    auto nodeBadges = gkSplit(badges, badgesSize);
    GKNode *owner = nullptr;
    HTREEITEM target = TVI_ROOT;
    if (parent) {
        owner = outline->find(parent);
        if (!owner) {
            return;
        }
        target = owner->item;
    }
    view->suppress++;
    SendMessageW(tree, WM_SETREDRAW, FALSE, 0);
    /* The children that the nodes replace go away with their records. */
    std::vector<HTREEITEM> old;
    HTREEITEM first = parent ? TreeView_GetChild(tree, target) : TreeView_GetRoot(tree);
    for (HTREEITEM child = first; child; child = TreeView_GetNextSibling(tree, child)) {
        old.push_back(child);
    }
    for (HTREEITEM child : old) {
        gkOutlineForget(outline, tree, child);
        TreeView_DeleteItem(tree, child);
    }
    /* Each icon has a place in the image list of the tree. */
    std::vector<int> imageIndex((size_t)count, -1);
    if (icons) {
        for (int i = 0; i < count; i++) {
            if (icons[i]) {
                GKIcon *icon = static_cast<GKIcon *>(icons[i]);
                gkRetainObject(icon);
                outline->icons.push_back(icon);
                imageIndex[i] = (int)outline->icons.size() - 1;
            }
        }
        if (outline->images) {
            gkComctl().ImageList_Destroy(outline->images);
        }
        std::vector<void *> all(outline->icons.begin(), outline->icons.end());
        outline->images = gkImageList(tree, all.data(), (int)all.size(), 16, 16, gkViewPalette(view).text);
        TreeView_SetImageList(tree, outline->images, TVSIL_NORMAL);
    }
    std::vector<HTREEITEM> stack;
    std::vector<GKNode *> records;
    for (int i = 0; i < count; i++) {
        auto record = std::make_unique<GKNode>();
        record->id = i < (int)nodeIDs.size() ? nodeIDs[i] : "";
        record->text = gkWide(i < (int)nodeTexts.size() ? nodeTexts[i].c_str() : "");
        record->badge = gkWide(i < (int)nodeBadges.size() ? nodeBadges[i].c_str() : "");
        record->hasChildren = (flags[i] & 1) != 0;
        record->group = (flags[i] & 2) != 0;
        while ((int)stack.size() > depths[i]) {
            stack.pop_back();
            records.pop_back();
        }
        TVINSERTSTRUCTW insert = {};
        insert.hParent = stack.empty() ? target : stack.back();
        insert.hInsertAfter = TVI_LAST;
        insert.item.mask = TVIF_TEXT | TVIF_PARAM | TVIF_CHILDREN | TVIF_STATE;
        insert.item.pszText = const_cast<wchar_t *>(record->text.c_str());
        insert.item.lParam = reinterpret_cast<LPARAM>(record.get());
        insert.item.cChildren = record->hasChildren ? 1 : 0;
        insert.item.stateMask = TVIS_BOLD;
        insert.item.state = record->group ? TVIS_BOLD : 0;
        int image = outline->images ? (imageIndex[i] >= 0 ? imageIndex[i] : I_IMAGENONE) : 0;
        if (outline->images) {
            insert.item.mask |= TVIF_IMAGE | TVIF_SELECTEDIMAGE;
            insert.item.iImage = image;
            insert.item.iSelectedImage = image;
        }
        record->parent = records.empty() ? owner : records.back();
        record->item = TreeView_InsertItem(tree, &insert);
        if (record->parent) {
            record->parent->loaded = 1;
            TVITEMW info = {};
            info.mask = TVIF_CHILDREN | TVIF_HANDLE;
            info.hItem = record->parent->item;
            info.cChildren = 1;
            TreeView_SetItem(tree, &info);
        }
        stack.push_back(record->item);
        records.push_back(record.get());
        outline->nodes.push_back(std::move(record));
    }
    if (outline->navigation) {
        /* A group of a navigation list is always open. */
        for (auto const &node : outline->nodes) {
            if (node->item && node->group) {
                TreeView_Expand(tree, node->item, TVE_EXPAND);
            }
        }
    }
    if (owner) {
        owner->hasChildren = count > 0;
        owner->loaded = 1;
        TVITEMW info = {};
        info.mask = TVIF_CHILDREN | TVIF_HANDLE;
        info.hItem = owner->item;
        info.cChildren = count > 0 ? 1 : 0;
        TreeView_SetItem(tree, &info);
        if (count > 0 && (TreeView_GetItemState(tree, owner->item, TVIS_EXPANDED) & TVIS_EXPANDED) == 0) {
            /* The node that asked for its children opens with them. */
            TreeView_Expand(tree, owner->item, TVE_EXPAND);
        }
    }
    outline->nodes.erase(std::remove_if(outline->nodes.begin(), outline->nodes.end(),
                                        [](std::unique_ptr<GKNode> const &node) { return !node->item; }),
                         outline->nodes.end());
    SendMessageW(tree, WM_SETREDRAW, TRUE, 0);
    InvalidateRect(tree, nullptr, TRUE);
    view->suppress--;
}

extern "C" char *gkWin32OutlineSelection(void *pointer, size_t *size) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutlineOf(view);
    *size = 0;
    if (!outline) {
        return nullptr;
    }
    std::vector<std::string> ids;
    if (outline->multiple) {
        for (auto const &node : outline->nodes) {
            if (node->item && std::find(outline->selected.begin(), outline->selected.end(), node.get()) !=
                                  outline->selected.end()) {
                ids.push_back(node->id);
            }
        }
    } else if (GKNode *node = outline->nodeOf(view->outer, TreeView_GetSelection(view->outer))) {
        ids.push_back(node->id);
    }
    return ids.empty() ? nullptr : gkJoin(ids, size);
}

static void gkOutlineReveal(HWND tree, HTREEITEM item) {
    for (HTREEITEM parent = TreeView_GetParent(tree, item); parent; parent = TreeView_GetParent(tree, parent)) {
        TreeView_Expand(tree, parent, TVE_EXPAND);
    }
}

extern "C" void gkWin32OutlineSelect(void *pointer, const char *ids, size_t size) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutlineOf(view);
    if (!outline) {
        return;
    }
    HWND tree = view->outer;
    auto wanted = gkSplit(ids, size);
    view->suppress++;
    for (GKNode *node : outline->selected) {
        if (node->item) {
            outline->mark(tree, node, false);
        }
    }
    outline->selected.clear();
    GKNode *first = nullptr;
    for (auto const &identifier : wanted) {
        if (GKNode *node = outline->find(identifier.c_str())) {
            gkOutlineReveal(tree, node->item);
            if (!first) {
                first = node;
            }
            outline->selected.push_back(node);
            if (!outline->multiple) {
                break;
            }
        }
    }
    TreeView_SelectItem(tree, first ? first->item : nullptr);
    if (outline->navigation && first && first->parent && first->parent->group && first->parent->item) {
        /* The heading of the group stays in view with the item. */
        TreeView_EnsureVisible(tree, first->parent->item);
        TreeView_EnsureVisible(tree, first->item);
    }
    for (GKNode *node : outline->selected) {
        outline->mark(tree, node, true);
    }
    view->suppress--;
}

extern "C" int gkWin32OutlineExpanded(void *pointer, const char *identifier) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutlineOf(view);
    GKNode *node = outline ? outline->find(identifier) : nullptr;
    return node && (TreeView_GetItemState(view->outer, node->item, TVIS_EXPANDED) & TVIS_EXPANDED) != 0;
}

extern "C" int gkWin32OutlineSetExpanded(void *pointer, const char *identifier, int expanded) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutlineOf(view);
    GKNode *node = outline ? outline->find(identifier) : nullptr;
    if (!node) {
        return 0;
    }
    if (expanded) {
        gkOutlineReveal(view->outer, node->item);
    }
    TreeView_Expand(view->outer, node->item, expanded ? TVE_EXPAND : TVE_COLLAPSE);
    if (expanded && node->hasChildren && !TreeView_GetChild(view->outer, node->item)) {
        /* A node without its children yet keeps its state until the
           children arrive. */
        TVITEMW info = {};
        info.mask = TVIF_STATE | TVIF_HANDLE;
        info.hItem = node->item;
        info.stateMask = TVIS_EXPANDED;
        info.state = TVIS_EXPANDED;
        TreeView_SetItem(view->outer, &info);
    }
    return 1;
}

/* ---- Collection view ---- */

struct GKCollection : GKExtension {
    std::vector<std::string> ids;
    int width = 96, height = 96;
    int selection = 1;
    HIMAGELIST images = nullptr;
    ~GKCollection() override {
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->code == LVN_ITEMCHANGING && selection == 0) {
            NMLISTVIEW *change = reinterpret_cast<NMLISTVIEW *>(header);
            if ((change->uNewState & LVIS_SELECTED) && !(change->uOldState & LVIS_SELECTED)) {
                *result = TRUE;
                return true;
            }
        }
        if (header->code == LVN_ITEMCHANGED) {
            NMLISTVIEW *change = reinterpret_cast<NMLISTVIEW *>(header);
            if ((change->uChanged & LVIF_STATE) && ((change->uNewState ^ change->uOldState) & LVIS_SELECTED)) {
                gkDataReport(view, GK_EVENT_SELECTION, 0, nullptr);
            }
            *result = 0;
            return true;
        }
        if (header->code == LVN_ITEMACTIVATE) {
            NMITEMACTIVATE *activate = reinterpret_cast<NMITEMACTIVATE *>(header);
            if (activate->iItem >= 0 && activate->iItem < (int)ids.size()) {
                gkSend(view->handle, GK_EVENT_ACTIVATE, 0, 0, 0, 0, ids[activate->iItem].c_str());
            }
            *result = 0;
            return true;
        }
        return false;
    }
    void themeChanged(GKView *view) override {
        gkApplyThemeTo(view->outer, gkViewTheme(view));
        gkListColors(view->outer, gkViewPalette(view));
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF((float)width * 3 + 24, (float)height * 2 + 24);
    }
};

static GKCollection *gkCollectionOf(GKView *view) {
    return view && view->kind == GK_KIND_COLLECTION ? static_cast<GKCollection *>(view->ext.get()) : nullptr;
}

extern "C" void *gkWin32CollectionCreate(uintptr_t window, int width, int height, int selection) {
    GKCollection *collection = new GKCollection();
    collection->width = width > 0 ? width : 96;
    collection->height = height > 0 ? height : 96;
    collection->selection = selection;
    DWORD style = LVS_ICON | LVS_SHOWSELALWAYS | LVS_AUTOARRANGE | WS_TABSTOP | (selection == 2 ? 0 : LVS_SINGLESEL);
    HWND hwnd = gkMakeNative(WC_LISTVIEWW, L"", style, 0);
    GKView *view = gkAdopt(window, hwnd, nullptr, GK_KIND_COLLECTION, 0);
    view->ext.reset(collection);
    SendMessageW(hwnd, LVM_SETEXTENDEDLISTVIEWSTYLE, 0, LVS_EX_DOUBLEBUFFER);
    double scale = gkScaleOf(hwnd);
    SendMessageW(hwnd, LVM_SETICONSPACING, 0,
                 MAKELPARAM((int)lround((collection->width + 12) * scale), (int)lround((collection->height + 12) * scale)));
    collection->themeChanged(view);
    return view;
}

extern "C" void gkWin32CollectionSetItems(void *pointer, const char *ids, size_t idsSize, const char *texts,
                                          size_t textsSize, void **images, int count) {
    GKView *view = gkView(pointer);
    GKCollection *collection = gkCollectionOf(view);
    if (!collection) {
        return;
    }
    HWND list = view->outer;
    auto itemIDs = gkSplit(ids, idsSize);
    auto itemTexts = gkSplit(texts, textsSize);
    std::vector<std::string> selected = gkSelectedIDs(list, collection->ids);
    view->suppress++;
    SendMessageW(list, WM_SETREDRAW, FALSE, 0);
    SendMessageW(list, LVM_DELETEALLITEMS, 0, 0);
    collection->ids.clear();
    if (collection->images) {
        SendMessageW(list, LVM_SETIMAGELIST, LVSIL_NORMAL, 0);
        gkComctl().ImageList_Destroy(collection->images);
        collection->images = nullptr;
    }
    /* The picture takes the cell above a line of text. */
    int line = gkMeasureGDI(L"Ag", gkBodyFont()).cy;
    int pictureHeight = std::max(16, collection->height - line - 8);
    collection->images = gkImageList(list, images, count, collection->width, pictureHeight, gkViewPalette(view).text);
    if (collection->images) {
        SendMessageW(list, LVM_SETIMAGELIST, LVSIL_NORMAL, (LPARAM)collection->images);
    }
    for (int i = 0; i < count; i++) {
        collection->ids.push_back(i < (int)itemIDs.size() ? itemIDs[i] : "");
        std::wstring text = gkWide(i < (int)itemTexts.size() ? itemTexts[i].c_str() : "");
        LVITEMW item = {};
        item.mask = LVIF_TEXT | (collection->images ? LVIF_IMAGE : 0);
        item.iItem = i;
        item.pszText = const_cast<wchar_t *>(text.c_str());
        item.iImage = images && images[i] ? i : I_IMAGENONE;
        SendMessageW(list, LVM_INSERTITEMW, 0, (LPARAM)&item);
    }
    gkSelectIDs(list, collection->ids, selected, collection->selection != 2);
    SendMessageW(list, WM_SETREDRAW, TRUE, 0);
    InvalidateRect(list, nullptr, TRUE);
    view->suppress--;
}

extern "C" char *gkWin32CollectionSelection(void *pointer, size_t *size) {
    GKView *view = gkView(pointer);
    GKCollection *collection = gkCollectionOf(view);
    *size = 0;
    if (!collection) {
        return nullptr;
    }
    std::vector<std::string> ids = gkSelectedIDs(view->outer, collection->ids);
    return ids.empty() ? nullptr : gkJoin(ids, size);
}

extern "C" void gkWin32CollectionSelect(void *pointer, const char *ids, size_t size) {
    GKView *view = gkView(pointer);
    GKCollection *collection = gkCollectionOf(view);
    if (!collection || collection->selection == 0) {
        return;
    }
    view->suppress++;
    gkSelectIDs(view->outer, collection->ids, gkSplit(ids, size), collection->selection != 2);
    view->suppress--;
}

/* ---- Breadcrumbs ----

   Breadcrumbs are a link control with a link for each item but the last,
   which is the current place. */

struct GKBreadcrumbs : GKExtension {
    std::vector<std::wstring> titles;
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->code == NM_CLICK || header->code == NM_RETURN) {
            NMLINK *link = reinterpret_cast<NMLINK *>(header);
            gkDataReport(view, GK_EVENT_ACTIVATE, link->item.iLink, nullptr);
            *result = 0;
            return true;
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        std::wstring text;
        for (size_t i = 0; i < titles.size(); i++) {
            text += (i ? L"  ›  " : L"") + titles[i];
        }
        SIZE size = gkMeasureGDI(text, gkViewFont(view, gkBodyFont()));
        return D2D1::SizeF((float)size.cx + 4, (float)size.cy + 4);
    }
};

extern "C" void *gkWin32BreadcrumbsCreate(uintptr_t window) {
    HWND hwnd = gkMakeNative(WC_LINK, L"", WS_TABSTOP | LWS_TRANSPARENT | LWS_NOPREFIX, 0);
    GKView *view = gkAdopt(window, hwnd, nullptr, GK_KIND_BREADCRUMBS, 0);
    view->ext.reset(new GKBreadcrumbs());
    view->accessibleName = view->accessibleName.empty() ? "Breadcrumbs" : view->accessibleName;
    return view;
}

extern "C" void gkWin32BreadcrumbsSetItems(void *pointer, const char *titles, size_t size) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_BREADCRUMBS) {
        return;
    }
    GKBreadcrumbs *crumbs = static_cast<GKBreadcrumbs *>(view->ext.get());
    crumbs->titles.clear();
    std::wstring markup;
    auto items = gkSplit(titles, size);
    for (size_t i = 0; i < items.size(); i++) {
        std::wstring title = gkWide(items[i].c_str());
        crumbs->titles.push_back(title);
        if (i) {
            markup += L"  ›  ";
        }
        if (i + 1 < items.size()) {
            wchar_t open[32];
            swprintf(open, 32, L"<a id=\"%u\">", (unsigned)i);
            std::wstring link = gkLinkMarkup(title);
            markup += open + link.substr(3);
        } else {
            for (wchar_t c : title) {
                markup += c == L'<' ? std::wstring(L"&lt;") : c == L'&' ? std::wstring(L"&amp;") : std::wstring(1, c);
            }
        }
    }
    SetWindowTextW(view->outer, markup.c_str());
}

std::vector<std::pair<std::string, std::string>> gkBreadcrumbItems(GKView *view) {
    std::vector<std::pair<std::string, std::string>> items;
    if (view && view->kind == GK_KIND_BREADCRUMBS) {
        for (auto const &title : static_cast<GKBreadcrumbs *>(view->ext.get())->titles) {
            items.emplace_back("Hyperlink", gkUTF8(title));
        }
    }
    return items;
}

/* ---- Pagination ---- */

struct GKPagination : GKExtension {
    HWND previous = nullptr, next = nullptr, label = nullptr;
    std::vector<HWND> pages;
    int count = 1;
    int selected = 0;
    int compact = 0;
    int start() {
        int visible = count < 7 ? count : 7;
        int first = selected - visible / 2;
        first = first < count - visible ? first : count - visible;
        return first > 0 ? first : 0;
    }
    void update(GKView *view) {
        for (HWND page : pages) {
            DestroyWindow(page);
        }
        pages.clear();
        EnableWindow(previous, selected > 0);
        EnableWindow(next, selected + 1 < count);
        wchar_t text[64];
        swprintf(text, 64, L"Page %d of %d", selected + 1, count);
        SetWindowTextW(label, text);
        ShowWindow(label, compact ? SW_SHOWNA : SW_HIDE);
        view->accessibleValue = gkUTF8(text);
        if (!compact) {
            int visible = count < 7 ? count : 7;
            int first = start();
            for (int i = 0; i < visible; i++) {
                wchar_t number[16];
                swprintf(number, 16, L"%d", first + i + 1);
                HWND button = gkMakeNative(WC_BUTTONW, number, BS_AUTORADIOBUTTON | BS_PUSHLIKE | WS_TABSTOP, 0,
                                           view->outer);
                SetPropW(button, L"GKView", view);
                SetPropW(button, L"GKPage", reinterpret_cast<HANDLE>((INT_PTR)(first + i + 1)));
                SendMessageW(button, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(button)), FALSE);
                SendMessageW(button, BM_SETCHECK, first + i == selected ? BST_CHECKED : BST_UNCHECKED, 0);
                gkApplyThemeTo(button, gkViewTheme(view));
                pages.push_back(button);
            }
        }
        layout(view);
    }
    void choose(GKView *view, int page) {
        page = page < 0 ? 0 : page > count - 1 ? count - 1 : page;
        if (page == selected) {
            return;
        }
        selected = page;
        gkLater([view]() {
            if (gkView(view)) {
                static_cast<GKPagination *>(view->ext.get())->update(view);
            }
        });
        gkDataReport(view, GK_EVENT_SELECTION, page, nullptr);
    }
    void layout(GKView *view) override {
        double height = view->height, side = std::min<double>(32, height + 8);
        gkPlaceWindow(previous, 0, 0, side, height);
        gkPlaceWindow(next, view->width - side, 0, side, height);
        double middle = view->width - 2 * side - 12;
        if (compact) {
            gkPlaceWindow(label, side + 6, 0, std::max(0.0, middle), height);
            return;
        }
        double width = 32, total = pages.size() * (width + 2);
        double x = side + 6 + std::max(0.0, (middle - total) / 2);
        for (HWND page : pages) {
            gkPlaceWindow(page, x, 0, width, height);
            x += width + 2;
        }
    }
    bool command(GKView *view, HWND from, int code) override {
        if (code != BN_CLICKED) {
            return false;
        }
        if (from == previous) {
            choose(view, selected - 1);
        } else if (from == next) {
            choose(view, selected + 1);
        } else if (INT_PTR page = (INT_PTR)GetPropW(from, L"GKPage")) {
            choose(view, (int)page - 1);
        }
        return true;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        float height = (float)(gkMeasureGDI(L"Ag", gkBodyFont()).cy + 8);
        float middle = compact ? 100.0f : (float)(std::min(count, 7) * 34);
        return D2D1::SizeF(2 * 32 + 12 + middle, height);
    }
};

extern "C" void *gkWin32PaginationCreate(uintptr_t window, int count, int selected, int compact) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_PAGINATION, window, 0);
    GKPagination *pagination = new GKPagination();
    view->ext.reset(pagination);
    GKFont symbol;
    symbol.family = gkSymbolFamily();
    symbol.size = 10;
    pagination->previous = gkMakeNative(WC_BUTTONW, L"\xE76B", BS_PUSHBUTTON | WS_TABSTOP, 0, host);
    pagination->next = gkMakeNative(WC_BUTTONW, L"\xE76C", BS_PUSHBUTTON | WS_TABSTOP, 0, host);
    pagination->label = gkMakeNative(WC_STATICW, L"", SS_CENTER | SS_CENTERIMAGE | SS_NOPREFIX, 0, host);
    for (HWND part : {pagination->previous, pagination->next}) {
        SetPropW(part, L"GKView", view);
        SendMessageW(part, WM_SETFONT, (WPARAM)gkFontHandle(symbol, GetDpiForWindow(part)), FALSE);
        gkApplyThemeTo(part, gkViewTheme(view));
    }
    SendMessageW(pagination->label, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(host)), FALSE);
    pagination->count = count > 1 ? count : 1;
    pagination->selected = selected < 0 ? 0 : selected > pagination->count - 1 ? pagination->count - 1 : selected;
    pagination->compact = compact != 0;
    view->accessibleName = "Pagination";
    pagination->update(view);
    return view;
}

extern "C" void gkWin32PaginationSet(void *pointer, int count, int selected) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_PAGINATION) {
        return;
    }
    GKPagination *pagination = static_cast<GKPagination *>(view->ext.get());
    pagination->count = count > 1 ? count : 1;
    pagination->selected = selected < 0 ? 0 : selected > pagination->count - 1 ? pagination->count - 1 : selected;
    pagination->update(view);
}
