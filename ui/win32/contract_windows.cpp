//go:build windows && cgo

#include "win32_internal.h"

#include <initguid.h>
#include <oleacc.h>
#include <shellapi.h>
#include <shlobj.h>

#include <algorithm>

/* gkWin32GoDrop is implemented in Go; see widget_contract.go. */
extern "C" void gkWin32GoDrop(uintptr_t handle, int operation, double x, double y, char *text, char *files,
                              size_t filesSize, unsigned char *pixels, int width, int height, char *types,
                              size_t typesSize, unsigned char *data, size_t *sizes, int count);
void gkMenuPopupAt(GKObject *menu, HWND owner, POINT screen);

/* ---- Accessibility ----

   Native controls report their role and state to assistive technology
   through Windows. The bridge adds the name and the description that the
   application sets with dynamic annotation, which overrides what the
   control reports. */

static IAccPropServices *gkAccProps() {
    static IAccPropServices *services;
    static bool tried;
    if (!tried) {
        tried = true;
        CoCreateInstance(CLSID_AccPropServices, nullptr, CLSCTX_INPROC_SERVER, IID_IAccPropServices,
                         reinterpret_cast<void **>(&services));
    }
    return services;
}

static void gkAnnotate(HWND hwnd, MSAAPROPID property, std::string const &value) {
    IAccPropServices *services = gkAccProps();
    if (!services || !hwnd) {
        return;
    }
    if (value.empty()) {
        services->ClearHwndProps(hwnd, OBJID_CLIENT, CHILDID_SELF, &property, 1);
    } else {
        std::wstring wide = gkWide(value.c_str());
        services->SetHwndPropStr(hwnd, OBJID_CLIENT, CHILDID_SELF, property, wide.c_str());
    }
}

extern "C" void gkWin32SetAccessibility(void *pointer, const char *identifier, const char *name,
                                        const char *description, const char *value, int required, int hidden,
                                        int invalid, void *labeledBy, void **describedBy, int describedByCount) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    (void)labeledBy;
    view->accessibleID = identifier ? identifier : "";
    view->accessibleName = name ? name : "";
    view->accessibleDescription = description ? description : "";
    view->accessibleValue = value ? value : "";
    view->accessibleRequired = required != 0;
    view->accessibleHidden = hidden != 0;
    view->accessibleInvalid = invalid != 0;
    std::string described = view->accessibleDescription;
    for (int i = 0; i < describedByCount; i++) {
        if (GKView *related = gkView(describedBy[i])) {
            std::string text = !related->accessibleName.empty() ? related->accessibleName : gkUTF8(related->text);
            if (!text.empty()) {
                described += (described.empty() ? "" : " ") + text;
            }
        }
    }
    HWND target = gkViewTarget(view);
    gkAnnotate(target, PROPID_ACC_NAME, view->accessibleName);
    gkAnnotate(target, PROPID_ACC_DESCRIPTION, described);
    if (!view->accessibleValue.empty()) {
        gkAnnotate(target, PROPID_ACC_VALUE, view->accessibleValue);
    }
}

/* ---- Accessibility tree ----

   The tree comes from the views of the window: each view reports the UI
   Automation control type of its control and the name of its component,
   which the Go layer maps to roles. */

static const char *gkControlType(GKView *view) {
    switch (view->kind) {
    case GK_KIND_LABEL:
    case GK_KIND_VALIDATION:
        return "Text";
    case GK_KIND_BUTTON:
    case GK_KIND_TOGGLE:
    case GK_KIND_COLOR:
    case GK_KIND_FONT:
    case GK_KIND_KEY_RECORDER:
        return "Button";
    case GK_KIND_TEXT_FIELD:
    case GK_KIND_TEXT_AREA:
    case GK_KIND_PASSWORD:
    case GK_KIND_SEARCH:
        return "Edit";
    case GK_KIND_NUMBER:
    case GK_KIND_STEPPER:
        return "Spinner";
    case GK_KIND_CHECK_BOX:
    case GK_KIND_SWITCH:
        return "CheckBox";
    case GK_KIND_SELECT:
    case GK_KIND_COMBO:
        return "ComboBox";
    case GK_KIND_SLIDER:
    case GK_KIND_DIAL:
    case GK_KIND_RANGE:
        return "Slider";
    case GK_KIND_PROGRESS:
    case GK_KIND_ACTIVITY:
        return "ProgressBar";
    case GK_KIND_SEPARATOR:
        return "Separator";
    case GK_KIND_IMAGE:
    case GK_KIND_ICON:
    case GK_KIND_VECTOR:
    case GK_KIND_ANIMATION:
        return "Image";
    case GK_KIND_LINK:
        return "Hyperlink";
    case GK_KIND_TABS:
        return "Tab";
    case GK_KIND_TABLE:
        return "Table";
    case GK_KIND_OUTLINE:
        return "Tree";
    case GK_KIND_COLLECTION:
        return "List";
    case GK_KIND_STATUS_BAR:
        return "StatusBar";
    case GK_KIND_COMMAND_BAR:
    case GK_KIND_TOOLBAR:
        return "ToolBar";
    case GK_KIND_DOCUMENT:
        return "Document";
    case GK_KIND_CALENDAR:
        return "Calendar";
    case GK_KIND_DATE:
        return "ComboBox";
    default:
        return "Group";
    }
}

static const char *gkComponent(GKView *view) {
    switch (view->kind) {
    case GK_KIND_PASSWORD:
        return "password";
    case GK_KIND_TEXT_AREA:
        return "textarea";
    case GK_KIND_ACTIVITY:
        return "busy";
    case GK_KIND_SECTION:
        return "disclosure";
    case GK_KIND_SCROLL:
        return "scroll";
    case GK_KIND_CANVAS:
        return "canvas";
    case GK_KIND_DATE:
        return "date";
    case GK_KIND_CALENDAR:
        return "calendar";
    case GK_KIND_COLOR:
        return "color";
    case GK_KIND_FONT:
        return "font";
    case GK_KIND_SPLIT:
        return "split";
    case GK_KIND_WEB:
        return "web";
    case GK_KIND_VIDEO:
        return "video";
    case GK_KIND_DOCUMENT:
        return "document";
    case GK_KIND_STATUS_BAR:
    case GK_KIND_BADGE:
        return "status";
    case GK_KIND_SWITCH:
        return "switch";
    case GK_KIND_TOGGLE:
        return "toggle";
    case GK_KIND_SEARCH:
        return "search";
    case GK_KIND_NUMBER:
    case GK_KIND_STEPPER:
        return "spin";
    case GK_KIND_RADIO_GROUP:
    case GK_KIND_SEGMENTED:
        return "radiogroup";
    case GK_KIND_DIAL:
    case GK_KIND_RANGE:
        return "slider";
    case GK_KIND_SEPARATOR:
        return "separator";
    case GK_KIND_TABS:
        return "tablist";
    case GK_KIND_BREADCRUMBS:
    case GK_KIND_PAGINATION:
        return "navigation";
    case GK_KIND_INFO_BAR:
        return "alert";
    case GK_KIND_LAYOUT:
    case GK_KIND_CLIP:
    case GK_KIND_GROUP_BOX:
        return "group";
    default:
        return "";
    }
}

static void gkAppendClean(std::string &out, std::string const &value) {
    for (char c : value) {
        out += c == '\x1e' || c == '\x1f' ? ' ' : c;
    }
}

char *gkWin32UIText(void *control);
std::vector<HWND> const *gkRadioButtons(GKView *view);
std::vector<std::pair<std::string, std::string>> gkSegmentItems(GKView *view);
std::vector<std::pair<std::string, std::string>> gkTabItems(GKView *view);
std::vector<std::pair<std::string, std::string>> gkBreadcrumbItems(GKView *view);
std::vector<std::pair<std::string, std::string>> gkToolbarItems(GKView *view);

static std::string gkViewValueText(GKView *view) {
    if (!view->accessibleValue.empty()) {
        return view->accessibleValue;
    }
    switch (view->kind) {
    case GK_KIND_PASSWORD:
        return "";
    case GK_KIND_TEXT_FIELD:
    case GK_KIND_TEXT_AREA:
    case GK_KIND_SEARCH:
    case GK_KIND_COMBO:
    case GK_KIND_SELECT: {
        char *text = gkWin32UIText(view);
        std::string value = text ? text : "";
        free(text);
        return value;
    }
    case GK_KIND_CHECK_BOX:
    case GK_KIND_TOGGLE:
    case GK_KIND_SWITCH: {
        extern int gkWin32UIChecked(void *control);
        return gkWin32UIChecked(view) ? "1" : "0";
    }
    case GK_KIND_SLIDER:
    case GK_KIND_PROGRESS:
    case GK_KIND_NUMBER:
    case GK_KIND_STEPPER:
    case GK_KIND_DIAL: {
        char text[32];
        snprintf(text, sizeof(text), "%g", view->value);
        return text;
    }
    default:
        return "";
    }
}

static std::string gkViewName(GKView *view) {
    if (!view->accessibleName.empty()) {
        return view->accessibleName;
    }
    switch (view->kind) {
    case GK_KIND_LABEL:
    case GK_KIND_BUTTON:
    case GK_KIND_TOGGLE:
    case GK_KIND_CHECK_BOX:
    case GK_KIND_SWITCH:
    case GK_KIND_LINK:
    case GK_KIND_SECTION:
    case GK_KIND_GROUP_BOX:
    case GK_KIND_STATUS_BAR:
    case GK_KIND_BADGE:
    case GK_KIND_VALIDATION:
        return gkUTF8(view->text);
    default:
        return "";
    }
}

struct GKTreeWriter {
    std::string out;
    int count = 0;
    HWND root = nullptr;
    double scale = 1;
};

/* gkViewItems lists the parts of a view that are items for assistive
   technology, such as the buttons of a radio group, as pairs of a control
   type and a name. */
static std::vector<std::pair<std::string, std::string>> gkViewItems(GKView *view) {
    std::vector<std::pair<std::string, std::string>> items;
    if (auto buttons = gkRadioButtons(view)) {
        for (HWND button : *buttons) {
            int length = GetWindowTextLengthW(button);
            std::wstring text((size_t)length, L'\0');
            if (length) {
                GetWindowTextW(button, &text[0], length + 1);
            }
            items.emplace_back("RadioButton", gkUTF8(text));
        }
    }
    if (view->kind == GK_KIND_SEGMENTED) {
        return gkSegmentItems(view);
    }
    if (view->kind == GK_KIND_TABS) {
        return gkTabItems(view);
    }
    if (view->kind == GK_KIND_BREADCRUMBS) {
        return gkBreadcrumbItems(view);
    }
    if (view->kind == GK_KIND_COMMAND_BAR || view->kind == GK_KIND_TOOLBAR) {
        return gkToolbarItems(view);
    }
    return items;
}

static void gkAppendRecord(GKTreeWriter &writer, int depth, const char *type, const char *component,
                           std::string const &id, std::string const &name, std::string const &value, RECT bounds) {
    writer.count++;
    char number[32];
    snprintf(number, sizeof(number), "%d\x1f", depth);
    writer.out += number;
    gkAppendClean(writer.out, type);
    writer.out += '\x1f';
    gkAppendClean(writer.out, component);
    writer.out += '\x1f';
    gkAppendClean(writer.out, id);
    writer.out += '\x1f';
    gkAppendClean(writer.out, name);
    writer.out += '\x1f';
    gkAppendClean(writer.out, value);
    char geometry[96];
    snprintf(geometry, sizeof(geometry), "\x1f%.0f\x1f%.0f\x1f%.0f\x1f%.0f\x1e", bounds.left / writer.scale,
             bounds.top / writer.scale, (bounds.right - bounds.left) / writer.scale,
             (bounds.bottom - bounds.top) / writer.scale);
    writer.out += geometry;
}

static void gkAppendWindow(GKTreeWriter &writer, HWND hwnd, int depth);

static void gkAppendChildren(GKTreeWriter &writer, HWND parent, int depth) {
    std::vector<HWND> children;
    for (HWND child = GetWindow(parent, GW_CHILD); child; child = GetWindow(child, GW_HWNDNEXT)) {
        children.push_back(child);
    }
    /* Reading order: top to bottom, then leading to trailing. */
    std::sort(children.begin(), children.end(), [](HWND a, HWND b) {
        RECT ra, rb;
        GetWindowRect(a, &ra);
        GetWindowRect(b, &rb);
        return ra.top != rb.top ? ra.top < rb.top : ra.left < rb.left;
    });
    for (HWND child : children) {
        gkAppendWindow(writer, child, depth);
    }
}

static void gkAppendWindow(GKTreeWriter &writer, HWND hwnd, int depth) {
    if (depth > 64 || writer.count > 20000) {
        return;
    }
    GKView *view = gkViewOf(hwnd);
    if (view && view->owner) {
        return;
    }
    bool visible = (GetWindowLongPtrW(hwnd, GWL_STYLE) & WS_VISIBLE) != 0;
    if (!visible) {
        return;
    }
    if (!view || view->outer != hwnd) {
        /* A layout or part without a view of its own: its children belong
           to the nearest view. */
        if (!view) {
            gkAppendChildren(writer, hwnd, depth);
        }
        return;
    }
    if (view->accessibleHidden) {
        return;
    }
    RECT bounds;
    GetWindowRect(hwnd, &bounds);
    MapWindowPoints(nullptr, writer.root, reinterpret_cast<POINT *>(&bounds), 2);
    gkAppendRecord(writer, depth, gkControlType(view), gkComponent(view), view->accessibleID, gkViewName(view),
                   gkViewValueText(view), bounds);
    for (auto const &item : gkViewItems(view)) {
        RECT empty = {bounds.left, bounds.top, bounds.left, bounds.top};
        gkAppendRecord(writer, depth + 1, item.first.c_str(), "", "", item.second, "", empty);
    }
    /* The children of a container are in its body, or are its own child
       windows, such as the panes of a split view. The parts of a native
       control have no views, so they add nothing. */
    gkAppendChildren(writer, view->body ? view->body : hwnd, depth + 1);
}

extern "C" char *gkWin32AccessibilityTree(void *pointer, size_t *size) {
    GKWin *win = static_cast<GKWin *>(pointer);
    *size = 0;
    if (!win || !win->hwnd) {
        return nullptr;
    }
    GKTreeWriter writer;
    writer.root = win->root;
    writer.scale = gkWinScale(win);
    int length = GetWindowTextLengthW(win->hwnd);
    std::wstring title((size_t)length, L'\0');
    if (length) {
        GetWindowTextW(win->hwnd, &title[0], length + 1);
    }
    int width = 0, height = 0;
    gkWin32WindowSize(win, &width, &height);
    writer.out += "0\x1fWindow\x1f\x1f\x1f";
    gkAppendClean(writer.out, gkUTF8(title));
    char geometry[96];
    snprintf(geometry, sizeof(geometry), "\x1f\x1f" "0\x1f" "0\x1f%d\x1f%d\x1e", width, height);
    writer.out += geometry;
    for (auto const &bar : win->chrome) {
        if (bar.view && bar.top) {
            gkAppendWindow(writer, bar.view->outer, 1);
        }
    }
    gkAppendChildren(writer, win->root, 1);
    for (auto const &bar : win->chrome) {
        if (bar.view && !bar.top) {
            gkAppendWindow(writer, bar.view->outer, 1);
        }
    }
    *size = writer.out.size();
    char *result = static_cast<char *>(malloc(writer.out.size() + 1));
    memcpy(result, writer.out.c_str(), writer.out.size() + 1);
    return result;
}

/* ---- Focus ---- */

static bool gkFocusable(HWND hwnd) {
    LONG_PTR style = GetWindowLongPtrW(hwnd, GWL_STYLE);
    return (style & WS_TABSTOP) && IsWindowEnabled(hwnd) && IsWindowVisible(hwnd);
}

static HWND gkFirstFocusable(HWND parent) {
    for (HWND child = GetWindow(parent, GW_CHILD); child; child = GetWindow(child, GW_HWNDNEXT)) {
        if (gkFocusable(child)) {
            return child;
        }
        if (HWND inner = gkFirstFocusable(child)) {
            return inner;
        }
    }
    return nullptr;
}

extern "C" int gkWin32Focus(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view) {
        return 0;
    }
    HWND target = gkViewTarget(view);
    if (target && !(GetWindowLongPtrW(target, GWL_STYLE) & WS_TABSTOP)) {
        target = gkFirstFocusable(view->outer);
    }
    if (!target || !IsWindowEnabled(target)) {
        return 0;
    }
    HWND root = GetAncestor(target, GA_ROOT);
    if (!IsWindowVisible(root) || !IsWindowVisible(target)) {
        return 0;
    }
    SetFocus(target);
    return GetFocus() == target;
}

extern "C" int gkWin32IsFocused(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view) {
        return 0;
    }
    HWND focus = GetFocus();
    return focus && (focus == view->outer || IsChild(view->outer, focus));
}

extern "C" void *gkWin32ViewWindow(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view) {
        return nullptr;
    }
    GKWin *win = gkWinOf(view->outer);
    return win ? win : view->window;
}

extern "C" int gkWin32HandleDragEvent(void *event) {
    (void)event;
    return 0;
}

/* ---- Tooltips ---- */

/* gkTooltipWindow returns the tooltip control of the application, which
   shows the tooltips of every view. */
static HWND gkTooltipWindow() {
    static HWND tooltip;
    if (!tooltip || !IsWindow(tooltip)) {
        GKActivation activation;
        tooltip = CreateWindowExW(WS_EX_TOPMOST | WS_EX_TRANSPARENT, TOOLTIPS_CLASSW, nullptr,
                                  WS_POPUP | TTS_ALWAYSTIP | TTS_NOPREFIX, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT,
                                  CW_USEDEFAULT, nullptr, nullptr, gkInstance(), nullptr);
        SendMessageW(tooltip, TTM_SETMAXTIPWIDTH, 0, 400);
    }
    return tooltip;
}

static void gkTooltipFor(HWND hwnd, std::wstring const &text) {
    if (!hwnd) {
        return;
    }
    HWND tooltip = gkTooltipWindow();
    TOOLINFOW info = {};
    info.cbSize = sizeof(info);
    info.hwnd = GetParent(hwnd);
    info.uId = (UINT_PTR)hwnd;
    info.uFlags = TTF_IDISHWND | TTF_SUBCLASS;
    SendMessageW(tooltip, TTM_DELTOOLW, 0, (LPARAM)&info);
    if (text.empty()) {
        return;
    }
    info.lpszText = const_cast<wchar_t *>(text.c_str());
    SendMessageW(tooltip, TTM_ADDTOOLW, 0, (LPARAM)&info);
}

extern "C" void gkWin32SetTooltip(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    view->tooltip = gkWide(text);
    gkTooltipFor(view->outer, view->tooltip);
    if (view->control && view->control != view->outer) {
        gkTooltipFor(view->control, view->tooltip);
    }
}

/* ---- Cursors ---- */

void gkSetCursorShape(int cursor) {
    /* Windows has no grab cursors; a hand is the Windows cursor for an
       object that the pointer can take and move. */
    static const LPCWSTR shapes[] = {IDC_ARROW,  IDC_HAND,   IDC_IBEAM, IDC_CROSS, IDC_SIZEALL, IDC_SIZEWE,
                                     IDC_SIZENS, IDC_NO,     IDC_WAIT,  IDC_HAND,  IDC_SIZEALL};
    int index = cursor >= 0 && cursor < (int)(sizeof(shapes) / sizeof(shapes[0])) ? cursor : 0;
    SetCursor(LoadCursorW(nullptr, shapes[index]));
}

/* gkCursorProc sets the cursor of a view over its windows. */
static LRESULT CALLBACK gkCursorProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, UINT_PTR id,
                                     DWORD_PTR data) {
    (void)id;
    if (message == WM_SETCURSOR && LOWORD(lparam) == HTCLIENT) {
        GKView *view = gkView(reinterpret_cast<void *>(data));
        if (view && view->cursor > 0) {
            gkSetCursorShape(view->cursor);
            return TRUE;
        }
    }
    if (message == WM_NCDESTROY) {
        gkComctl().RemoveWindowSubclass(hwnd, gkCursorProc, 2);
    }
    return gkComctl().DefSubclassProc(hwnd, message, wparam, lparam);
}

extern "C" void gkWin32SetCursor(void *pointer, int cursor) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    view->cursor = cursor;
    HWND windows[] = {view->outer, view->control};
    for (HWND hwnd : windows) {
        if (hwnd) {
            gkComctl().SetWindowSubclass(hwnd, gkCursorProc, 2, reinterpret_cast<DWORD_PTR>(view));
        }
    }
}

/* ---- Style ---- */

void gkStyleResolve(GKView *view) {
    GKStyle &style = view->style;
    GKPalette const &palette = gkViewPalette(view);
    if (style.foregroundRole) {
        style.hasForeground = gkSemanticColor(palette, style.foregroundRole, &style.foreground);
    }
    if (style.backgroundRole) {
        style.hasBackground = gkSemanticColor(palette, style.backgroundRole, &style.background);
    }
    if (view->kind == GK_KIND_LAYOUT || view->kind == GK_KIND_CLIP) {
        if (style.hasBackground) {
            GKColor color = gkMix(gkBackgroundOf(GetParent(view->outer)), style.background);
            gkLayoutSetBackground(gkContainerOf(view), &color);
        } else {
            gkLayoutSetBackground(gkContainerOf(view), nullptr);
        }
    }
    if (view->background) {
        DeleteObject(view->background);
        view->background = nullptr;
    }
}

extern "C" int gkWin32SetStyle(void *pointer, const char *family, double size, int weight, int italic, int foreground,
                               int background, const double *foregroundRGBA, const double *backgroundRGBA,
                               double cornerRadius) {
    GKView *view = gkView(pointer);
    if (!view) {
        return 0;
    }
    std::string name = family ? family : "";
    if (!name.empty() && name != "System" && name != "Monospace" && !gkFontFamilyInstalled(family)) {
        return 0;
    }
    GKStyle &style = view->style;
    style.family = name == "Monospace" ? L"Consolas" : name == "System" ? L"" : gkWide(family);
    style.size = size > 0 ? size : 0;
    style.weight = weight > 0 ? weight : 0;
    style.italic = italic != 0;
    style.cornerRadius = cornerRadius > 0 ? cornerRadius : 0;
    style.hasForeground = 0;
    style.foregroundRole = 0;
    if (foregroundRGBA) {
        style.hasForeground = 1;
        style.foreground = GKColor{(float)foregroundRGBA[0], (float)foregroundRGBA[1], (float)foregroundRGBA[2],
                                   (float)foregroundRGBA[3]};
    } else {
        style.foregroundRole = foreground;
    }
    style.hasBackground = 0;
    style.backgroundRole = 0;
    if (backgroundRGBA) {
        style.hasBackground = 1;
        style.background = GKColor{(float)backgroundRGBA[0], (float)backgroundRGBA[1], (float)backgroundRGBA[2],
                                   (float)backgroundRGBA[3]};
    } else {
        style.backgroundRole = background;
    }
    gkStyleResolve(view);
    gkApplyFont(view);
    if (view->ext) {
        view->ext->themeChanged(view);
    }
    gkInvalidate(view);
    return 1;
}

/* ---- Context menus ---- */

/* gkMenuProc shows the context menu of a view. */
static LRESULT CALLBACK gkMenuProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, UINT_PTR id,
                                   DWORD_PTR data) {
    (void)id;
    if (message == WM_CONTEXTMENU) {
        GKView *view = gkView(reinterpret_cast<void *>(data));
        if (view && view->contextMenu) {
            POINT point = {GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam)};
            if (point.x == -1 && point.y == -1) {
                RECT rect;
                GetWindowRect(hwnd, &rect);
                point.x = rect.left + 8;
                point.y = rect.top + 8;
            }
            gkMenuPopupAt(view->contextMenu, hwnd, point);
            return 0;
        }
    }
    if (message == WM_NCDESTROY) {
        gkComctl().RemoveWindowSubclass(hwnd, gkMenuProc, 3);
    }
    return gkComctl().DefSubclassProc(hwnd, message, wparam, lparam);
}

extern "C" void gkWin32SetMenu(void *pointer, void *menu) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    GKObject *object = static_cast<GKObject *>(menu);
    if (object) {
        gkRetainObject(object);
    }
    if (view->contextMenu) {
        gkReleaseObject(view->contextMenu);
    }
    view->contextMenu = object;
    HWND windows[] = {view->outer, view->control};
    for (HWND hwnd : windows) {
        if (hwnd) {
            gkComctl().SetWindowSubclass(hwnd, gkMenuProc, 3, reinterpret_cast<DWORD_PTR>(view));
        }
    }
}

/* ---- Data objects ----

   GKDataObject holds data in clipboard formats for drag and drop. */

struct GKDataItem {
    FORMATETC format;
    HGLOBAL memory;
};

struct GKDataObject : IDataObject {
    LONG refs = 1;
    std::vector<GKDataItem> items;
    ~GKDataObject() {
        for (auto &item : items) {
            GlobalFree(item.memory);
        }
    }
    void add(CLIPFORMAT format, const void *data, size_t size) {
        HGLOBAL memory = GlobalAlloc(GMEM_MOVEABLE, size ? size : 1);
        if (!memory) {
            return;
        }
        void *target = GlobalLock(memory);
        if (size) {
            memcpy(target, data, size);
        }
        GlobalUnlock(memory);
        GKDataItem item;
        item.format = {format, nullptr, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
        item.memory = memory;
        items.push_back(item);
    }
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
        if (riid == IID_IUnknown || riid == IID_IDataObject) {
            *object = static_cast<IDataObject *>(this);
            AddRef();
            return S_OK;
        }
        *object = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override {
        return InterlockedIncrement(&refs);
    }
    ULONG STDMETHODCALLTYPE Release() override {
        LONG count = InterlockedDecrement(&refs);
        if (count == 0) {
            delete this;
        }
        return count;
    }
    GKDataItem *find(FORMATETC *format) {
        for (auto &item : items) {
            if (item.format.cfFormat == format->cfFormat && (format->tymed & TYMED_HGLOBAL)) {
                return &item;
            }
        }
        return nullptr;
    }
    HRESULT STDMETHODCALLTYPE GetData(FORMATETC *format, STGMEDIUM *medium) override {
        GKDataItem *item = find(format);
        if (!item) {
            return DV_E_FORMATETC;
        }
        size_t size = GlobalSize(item->memory);
        HGLOBAL copy = GlobalAlloc(GMEM_MOVEABLE, size);
        memcpy(GlobalLock(copy), GlobalLock(item->memory), size);
        GlobalUnlock(copy);
        GlobalUnlock(item->memory);
        medium->tymed = TYMED_HGLOBAL;
        medium->hGlobal = copy;
        medium->pUnkForRelease = nullptr;
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE GetDataHere(FORMATETC *, STGMEDIUM *) override {
        return E_NOTIMPL;
    }
    HRESULT STDMETHODCALLTYPE QueryGetData(FORMATETC *format) override {
        return find(format) ? S_OK : DV_E_FORMATETC;
    }
    HRESULT STDMETHODCALLTYPE GetCanonicalFormatEtc(FORMATETC *, FORMATETC *out) override {
        out->ptd = nullptr;
        return E_NOTIMPL;
    }
    HRESULT STDMETHODCALLTYPE SetData(FORMATETC *, STGMEDIUM *, BOOL) override {
        return E_NOTIMPL;
    }
    HRESULT STDMETHODCALLTYPE EnumFormatEtc(DWORD direction, IEnumFORMATETC **result) override {
        if (direction != DATADIR_GET) {
            return E_NOTIMPL;
        }
        std::vector<FORMATETC> formats;
        for (auto const &item : items) {
            formats.push_back(item.format);
        }
        return SHCreateStdEnumFmtEtc((UINT)formats.size(), formats.data(), result);
    }
    HRESULT STDMETHODCALLTYPE DAdvise(FORMATETC *, DWORD, IAdviseSink *, DWORD *) override {
        return OLE_E_ADVISENOTSUPPORTED;
    }
    HRESULT STDMETHODCALLTYPE DUnadvise(DWORD) override {
        return OLE_E_ADVISENOTSUPPORTED;
    }
    HRESULT STDMETHODCALLTYPE EnumDAdvise(IEnumSTATDATA **) override {
        return OLE_E_ADVISENOTSUPPORTED;
    }
};

/* gkCustomFormat returns the clipboard format for a MIME type. */
CLIPFORMAT gkCustomFormat(std::string const &type) {
    return (CLIPFORMAT)RegisterClipboardFormatW(gkWide(("GameKit " + type).c_str()).c_str());
}

/* gkFileList returns a DROPFILES block for paths. */
std::vector<unsigned char> gkFileList(std::vector<std::string> const &paths) {
    std::wstring list;
    for (auto const &path : paths) {
        list += gkWide(path.c_str());
        list += L'\0';
    }
    list += L'\0';
    std::vector<unsigned char> block(sizeof(DROPFILES) + list.size() * sizeof(wchar_t));
    DROPFILES *files = reinterpret_cast<DROPFILES *>(block.data());
    files->pFiles = sizeof(DROPFILES);
    files->fWide = TRUE;
    memcpy(block.data() + sizeof(DROPFILES), list.data(), list.size() * sizeof(wchar_t));
    return block;
}

/* gkDIB returns a CF_DIB block for RGBA pixels. */
std::vector<unsigned char> gkDIB(const unsigned char *pixels, int width, int height) {
    std::vector<unsigned char> block(sizeof(BITMAPINFOHEADER) + (size_t)width * height * 4);
    BITMAPINFOHEADER *header = reinterpret_cast<BITMAPINFOHEADER *>(block.data());
    header->biSize = sizeof(BITMAPINFOHEADER);
    header->biWidth = width;
    header->biHeight = height;
    header->biPlanes = 1;
    header->biBitCount = 32;
    header->biCompression = BI_RGB;
    unsigned char *out = block.data() + sizeof(BITMAPINFOHEADER);
    for (int y = 0; y < height; y++) {
        const unsigned char *row = pixels + (size_t)(height - 1 - y) * width * 4;
        for (int x = 0; x < width; x++) {
            out[((size_t)y * width + x) * 4 + 0] = row[x * 4 + 2];
            out[((size_t)y * width + x) * 4 + 1] = row[x * 4 + 1];
            out[((size_t)y * width + x) * 4 + 2] = row[x * 4 + 0];
            out[((size_t)y * width + x) * 4 + 3] = row[x * 4 + 3];
        }
    }
    return block;
}

/* gkPixelsFromDIB reads RGBA pixels from a CF_DIB block. */
bool gkPixelsFromDIB(const unsigned char *block, size_t size, std::vector<unsigned char> *pixels, int *width,
                     int *height) {
    if (size < sizeof(BITMAPINFOHEADER)) {
        return false;
    }
    const BITMAPINFOHEADER *header = reinterpret_cast<const BITMAPINFOHEADER *>(block);
    int w = header->biWidth, h = abs(header->biHeight);
    bool bottomUp = header->biHeight > 0;
    int bits = header->biBitCount;
    if (w <= 0 || h <= 0 || (bits != 32 && bits != 24)) {
        return false;
    }
    size_t offset = header->biSize + (header->biCompression == BI_BITFIELDS && header->biSize == 40 ? 12 : 0) +
                    header->biClrUsed * 4;
    size_t stride = ((size_t)w * bits / 8 + 3) & ~(size_t)3;
    if (size < offset + stride * h) {
        return false;
    }
    pixels->assign((size_t)w * h * 4, 0);
    bool hasAlpha = false;
    for (int y = 0; y < h && bits == 32; y++) {
        const unsigned char *row = block + offset + stride * y;
        for (int x = 0; x < w; x++) {
            hasAlpha |= row[x * 4 + 3] != 0;
        }
    }
    for (int y = 0; y < h; y++) {
        const unsigned char *row = block + offset + stride * (bottomUp ? h - 1 - y : y);
        for (int x = 0; x < w; x++) {
            unsigned char *out = &(*pixels)[((size_t)y * w + x) * 4];
            const unsigned char *p = row + x * (bits / 8);
            out[0] = p[2];
            out[1] = p[1];
            out[2] = p[0];
            out[3] = bits == 32 && hasAlpha ? p[3] : 255;
        }
    }
    *width = w;
    *height = h;
    return true;
}

/* ---- Drag sources ---- */

struct GKDragSource {
    int operations = 0;
    std::wstring text;
    std::vector<std::string> files;
    std::vector<unsigned char> pixels;
    int width = 0, height = 0;
    std::vector<std::string> types;
    std::vector<std::vector<unsigned char>> data;
};

static std::unordered_map<GKView *, GKDragSource> &gkDragSources() {
    static std::unordered_map<GKView *, GKDragSource> sources;
    return sources;
}

struct GKDropSource : IDropSource {
    LONG refs = 1;
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
        if (riid == IID_IUnknown || riid == IID_IDropSource) {
            *object = static_cast<IDropSource *>(this);
            AddRef();
            return S_OK;
        }
        *object = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override {
        return InterlockedIncrement(&refs);
    }
    ULONG STDMETHODCALLTYPE Release() override {
        LONG count = InterlockedDecrement(&refs);
        if (count == 0) {
            delete this;
        }
        return count;
    }
    HRESULT STDMETHODCALLTYPE QueryContinueDrag(BOOL escape, DWORD keys) override {
        if (escape) {
            return DRAGDROP_S_CANCEL;
        }
        if (!(keys & (MK_LBUTTON | MK_RBUTTON))) {
            return DRAGDROP_S_DROP;
        }
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE GiveFeedback(DWORD) override {
        return DRAGDROP_S_USEDEFAULTCURSORS;
    }
};

static DWORD gkEffects(int operations) {
    DWORD effects = 0;
    if (operations & 1) {
        effects |= DROPEFFECT_COPY;
    }
    if (operations & 2) {
        effects |= DROPEFFECT_MOVE;
    }
    if (operations & 4) {
        effects |= DROPEFFECT_LINK;
    }
    return effects;
}

static int gkOperation(DWORD effect) {
    if (effect & DROPEFFECT_MOVE) {
        return 2;
    }
    if (effect & DROPEFFECT_COPY) {
        return 1;
    }
    if (effect & DROPEFFECT_LINK) {
        return 4;
    }
    return 0;
}

static void gkStartDrag(GKView *view, GKDragSource const &source) {
    GKDataObject *data = new GKDataObject();
    if (!source.text.empty()) {
        data->add(CF_UNICODETEXT, source.text.c_str(), (source.text.size() + 1) * sizeof(wchar_t));
    }
    if (!source.files.empty()) {
        std::vector<unsigned char> block = gkFileList(source.files);
        data->add(CF_HDROP, block.data(), block.size());
    }
    if (!source.pixels.empty()) {
        std::vector<unsigned char> block = gkDIB(source.pixels.data(), source.width, source.height);
        data->add(CF_DIB, block.data(), block.size());
    }
    for (size_t i = 0; i < source.types.size() && i < source.data.size(); i++) {
        data->add(gkCustomFormat(source.types[i]), source.data[i].data(), source.data[i].size());
    }
    GKDropSource *drop = new GKDropSource();
    DWORD effect = 0;
    DoDragDrop(data, drop, gkEffects(source.operations ? source.operations : 1), &effect);
    drop->Release();
    data->Release();
    (void)view;
}

static GKView *gkDragView;
static POINT gkDragStart;

/* gkDragTrack starts a drag when the pointer moves far enough with the
   button down over a view that has a drag source. It returns true when it
   consumed the message. */
bool gkDragTrack(MSG const *message) {
    if (message->message == WM_LBUTTONDOWN) {
        gkDragView = nullptr;
        GKView *view = gkViewFromDescendant(message->hwnd);
        if (view && gkDragSources().count(view)) {
            gkDragView = view;
            GetCursorPos(&gkDragStart);
        }
        return false;
    }
    if (message->message == WM_LBUTTONUP) {
        gkDragView = nullptr;
        return false;
    }
    if (message->message != WM_MOUSEMOVE || !gkDragView || !(message->wParam & MK_LBUTTON)) {
        return false;
    }
    POINT cursor;
    GetCursorPos(&cursor);
    if (abs(cursor.x - gkDragStart.x) < GetSystemMetrics(SM_CXDRAG) &&
        abs(cursor.y - gkDragStart.y) < GetSystemMetrics(SM_CYDRAG)) {
        return false;
    }
    GKView *view = gkDragView;
    gkDragView = nullptr;
    if (!gkView(view) || !gkDragSources().count(view)) {
        return false;
    }
    GKDragSource source = gkDragSources()[view];
    if (GetCapture()) {
        ReleaseCapture();
    }
    gkStartDrag(view, source);
    return true;
}

extern "C" void gkWin32SetDragSource(void *pointer, int operations, const char *text, const char *files,
                                     size_t filesSize, const unsigned char *pixels, int width, int height,
                                     const char *customTypes, size_t customTypesSize, const unsigned char *customData,
                                     const size_t *customSizes, int customCount) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    if (!operations && (!text || !*text) && !filesSize && !pixels && !customCount) {
        gkDragSources().erase(view);
        return;
    }
    GKDragSource source;
    source.operations = operations;
    source.text = gkWide(text);
    source.files = gkSplit(files, filesSize);
    if (pixels && width > 0 && height > 0) {
        source.pixels.assign(pixels, pixels + (size_t)width * height * 4);
        source.width = width;
        source.height = height;
    }
    source.types = gkSplit(customTypes, customTypesSize);
    size_t offset = 0;
    for (int i = 0; i < customCount; i++) {
        source.data.emplace_back(customData + offset, customData + offset + customSizes[i]);
        offset += customSizes[i];
    }
    gkDragSources()[view] = source;
}

void gkForgetDragSource(GKView *view) {
    gkDragSources().erase(view);
    if (gkDragView == view) {
        gkDragView = nullptr;
    }
}

/* ---- Drop targets ---- */

struct GKDropTarget : IDropTarget {
    LONG refs = 1;
    GKView *view = nullptr;
    int operations = 0;
    std::vector<std::string> types;
    bool accepted = false;
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
        if (riid == IID_IUnknown || riid == IID_IDropTarget) {
            *object = static_cast<IDropTarget *>(this);
            AddRef();
            return S_OK;
        }
        *object = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override {
        return InterlockedIncrement(&refs);
    }
    ULONG STDMETHODCALLTYPE Release() override {
        LONG count = InterlockedDecrement(&refs);
        if (count == 0) {
            delete this;
        }
        return count;
    }
    bool wants(std::string const &type) {
        return types.empty() || std::find(types.begin(), types.end(), type) != types.end();
    }
    bool acceptable(IDataObject *data) {
        FORMATETC text = {CF_UNICODETEXT, nullptr, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
        FORMATETC files = {CF_HDROP, nullptr, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
        FORMATETC image = {CF_DIB, nullptr, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
        if ((wants("text/plain") || wants("public.utf8-plain-text")) && data->QueryGetData(&text) == S_OK) {
            return true;
        }
        if ((wants("text/uri-list") || wants("public.file-url") || wants("files")) &&
            data->QueryGetData(&files) == S_OK) {
            return true;
        }
        if ((wants("image/png") || wants("image")) && data->QueryGetData(&image) == S_OK) {
            return true;
        }
        for (auto const &type : types) {
            FORMATETC custom = {gkCustomFormat(type), nullptr, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
            if (data->QueryGetData(&custom) == S_OK) {
                return true;
            }
        }
        return false;
    }
    DWORD choose(DWORD keys, DWORD allowed) {
        DWORD wanted = gkEffects(operations) & allowed;
        if ((keys & MK_CONTROL) && (wanted & DROPEFFECT_COPY)) {
            return DROPEFFECT_COPY;
        }
        if ((keys & MK_SHIFT) && (wanted & DROPEFFECT_MOVE)) {
            return DROPEFFECT_MOVE;
        }
        if (wanted & DROPEFFECT_COPY) {
            return DROPEFFECT_COPY;
        }
        if (wanted & DROPEFFECT_MOVE) {
            return DROPEFFECT_MOVE;
        }
        return wanted & DROPEFFECT_LINK;
    }
    HRESULT STDMETHODCALLTYPE DragEnter(IDataObject *data, DWORD keys, POINTL, DWORD *effect) override {
        accepted = acceptable(data);
        *effect = accepted ? choose(keys, *effect) : DROPEFFECT_NONE;
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE DragOver(DWORD keys, POINTL, DWORD *effect) override {
        *effect = accepted ? choose(keys, *effect) : DROPEFFECT_NONE;
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE DragLeave() override {
        accepted = false;
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE Drop(IDataObject *data, DWORD keys, POINTL point, DWORD *effect) override;
};

static bool gkReadData(IDataObject *data, CLIPFORMAT format, std::vector<unsigned char> *out) {
    FORMATETC request = {format, nullptr, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
    STGMEDIUM medium = {};
    if (data->GetData(&request, &medium) != S_OK) {
        return false;
    }
    if (medium.tymed == TYMED_HGLOBAL && medium.hGlobal) {
        size_t size = GlobalSize(medium.hGlobal);
        const unsigned char *bytes = static_cast<const unsigned char *>(GlobalLock(medium.hGlobal));
        out->assign(bytes, bytes + size);
        GlobalUnlock(medium.hGlobal);
    }
    ReleaseStgMedium(&medium);
    return true;
}

HRESULT STDMETHODCALLTYPE GKDropTarget::Drop(IDataObject *data, DWORD keys, POINTL point, DWORD *effect) {
    DWORD chosen = accepted ? choose(keys, *effect) : DROPEFFECT_NONE;
    *effect = chosen;
    accepted = false;
    if (!chosen || !gkView(view) || !view->handle) {
        return S_OK;
    }
    POINT client = {point.x, point.y};
    ScreenToClient(view->outer, &client);
    double scale = gkViewScaleOf(view);
    std::string text;
    std::vector<unsigned char> block;
    if (gkReadData(data, CF_UNICODETEXT, &block) && block.size() >= 2) {
        text = gkUTF8(reinterpret_cast<const wchar_t *>(block.data()), -1);
    }
    std::vector<std::string> paths;
    if (gkReadData(data, CF_HDROP, &block) && block.size() >= sizeof(DROPFILES)) {
        HGLOBAL memory = GlobalAlloc(GMEM_MOVEABLE, block.size());
        memcpy(GlobalLock(memory), block.data(), block.size());
        GlobalUnlock(memory);
        HDROP drop = static_cast<HDROP>(memory);
        UINT count = DragQueryFileW(drop, 0xffffffff, nullptr, 0);
        for (UINT i = 0; i < count; i++) {
            UINT length = DragQueryFileW(drop, i, nullptr, 0);
            std::wstring path(length, L'\0');
            DragQueryFileW(drop, i, &path[0], length + 1);
            paths.push_back(gkUTF8(path));
        }
        GlobalFree(memory);
    }
    std::vector<unsigned char> pixels;
    int width = 0, height = 0;
    if (gkReadData(data, CF_DIB, &block)) {
        gkPixelsFromDIB(block.data(), block.size(), &pixels, &width, &height);
    }
    std::vector<std::string> foundTypes;
    std::vector<unsigned char> customData;
    std::vector<size_t> sizes;
    for (auto const &type : types) {
        if (gkReadData(data, gkCustomFormat(type), &block)) {
            foundTypes.push_back(type);
            customData.insert(customData.end(), block.begin(), block.end());
            sizes.push_back(block.size());
        }
    }
    size_t filesSize = 0, typesSize = 0;
    char *files = gkJoin(paths, &filesSize);
    char *typeList = gkJoin(foundTypes, &typesSize);
    gkWin32GoDrop(view->handle, gkOperation(chosen), client.x / scale, client.y / scale,
                  text.empty() ? nullptr : const_cast<char *>(text.c_str()), files, filesSize,
                  pixels.empty() ? nullptr : pixels.data(), width, height, typeList, typesSize,
                  customData.empty() ? nullptr : customData.data(), sizes.empty() ? nullptr : sizes.data(),
                  (int)sizes.size());
    free(files);
    free(typeList);
    return S_OK;
}

static std::unordered_map<GKView *, GKDropTarget *> &gkDropTargets() {
    static std::unordered_map<GKView *, GKDropTarget *> targets;
    return targets;
}

extern "C" void gkWin32SetDropTarget(void *pointer, int operations, const char *types, size_t typesSize) {
    GKView *view = gkView(pointer);
    if (!view || !view->outer) {
        return;
    }
    auto found = gkDropTargets().find(view);
    if (found != gkDropTargets().end()) {
        RevokeDragDrop(view->outer);
        found->second->view = nullptr;
        found->second->Release();
        gkDropTargets().erase(found);
    }
    if (!operations) {
        return;
    }
    GKDropTarget *target = new GKDropTarget();
    target->view = view;
    target->operations = operations;
    target->types = gkSplit(types, typesSize);
    if (SUCCEEDED(RegisterDragDrop(view->outer, target))) {
        gkDropTargets()[view] = target;
    } else {
        target->Release();
    }
}

void gkForgetDropTarget(GKView *view) {
    auto found = gkDropTargets().find(view);
    if (found != gkDropTargets().end()) {
        if (view->outer && IsWindow(view->outer)) {
            RevokeDragDrop(view->outer);
        }
        found->second->view = nullptr;
        found->second->Release();
        gkDropTargets().erase(found);
    }
}
