//go:build windows && cgo

#include "win32_internal.h"

#include <shobjidl.h>
#include <uxtheme.h>
#include <vssym32.h>

/* ---- Creation helpers ---- */

/* gkNative creates a view for a native control. */
static GKView *gkNative(uintptr_t window, const wchar_t *className, const wchar_t *text, DWORD style, DWORD exStyle,
                        int kind, uintptr_t action) {
    HWND hwnd = gkMakeNative(className, text, style, exStyle);
    if (!hwnd) {
        return nullptr;
    }
    return gkAdopt(window, hwnd, nullptr, kind, action);
}

/* gkHostView creates a view whose window is a layout that holds native
   controls. */
static GKView *gkHostView(uintptr_t window, int kind, GKExtension *ext, uintptr_t action) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, kind, window, action);
    view->ext.reset(ext);
    return view;
}

static double gkClamp(double value, double minimum, double maximum) {
    return value < minimum ? minimum : value > maximum ? maximum : value;
}

/* gkTextHeight returns the height of a line of the body font. */
static int gkTextHeight(GKView *view) {
    return gkMeasureGDI(L"Ag", gkViewFont(view, gkBodyFont())).cy;
}

static std::wstring gkWindowText(HWND hwnd) {
    if (!hwnd) {
        return std::wstring();
    }
    int length = GetWindowTextLengthW(hwnd);
    std::wstring text((size_t)length, L'\0');
    if (length > 0) {
        GetWindowTextW(hwnd, &text[0], length + 1);
    }
    return text;
}

extern "C" uintptr_t gkWin32UIControlHandle(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return 0;
    }
    if (view->kind == GK_KIND_CANVAS) {
        return reinterpret_cast<uintptr_t>(view->outer);
    }
    return reinterpret_cast<uintptr_t>(control);
}

/* ---- Layouts ---- */

extern "C" void *gkWin32UICreateLayout(uintptr_t window) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_LAYOUT, window, 0);
    view->body = host;
    return view;
}

/* ---- Labels ---- */

GKExtension *gkLabelExtension();

extern "C" void *gkWin32UICreateLabel(uintptr_t window, const char *text) {
    std::wstring value = gkWide(text);
    GKView *view = gkNative(window, WC_STATICW, value.c_str(),
                            SS_LEFT | SS_NOPREFIX | SS_CENTERIMAGE | SS_ENDELLIPSIS | SS_NOTIFY, 0, GK_KIND_LABEL,
                            0);
    if (view) {
        view->text = value;
        view->ext.reset(gkLabelExtension());
    }
    return view;
}

/* ---- Buttons ---- */

int gkButtonBusy(GKView *view);
void gkButtonLayout(GKView *view);
void gkButtonSetText(GKView *view, std::wstring const &text);
void gkCheckBoxSetText(GKView *view, std::wstring const &text);

struct GKButton : GKExtension {
    bool command(GKView *view, HWND from, int code) override {
        (void)from;
        if (code == BN_CLICKED) {
            /* A busy button ignores clicks. */
            if (!gkButtonBusy(view)) {
                gkViewAction(view, GK_UI_EVENT_CLICK);
            }
            return true;
        }
        return false;
    }
    void layout(GKView *view) override {
        gkButtonLayout(view);
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        return gkButtonCustomDraw(view, header, result);
    }
    void themeChanged(GKView *view) override {
        gkButtonThemeChanged(view);
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)wparam;
        (void)lparam;
        (void)result;
        /* The badge follows the button. */
        if (hwnd == view->outer && (message == WM_WINDOWPOSCHANGED || message == WM_SHOWWINDOW)) {
            gkButtonLayout(view);
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        return gkButtonPreferred(view);
    }
};

D2D1_SIZE_F gkButtonPreferred(GKView *view) {
    SIZE text = gkMeasureGDI(view->text, gkViewFont(view, gkBodyFont()));
    float line = (float)gkTextHeight(view);
    float width = (float)text.cx + 24;
    float height = line + 8;
    /* A button centers its text in its whole width, so an icon before or
       after the text has the room of the icon on both sides. */
    const float icon = 16, edge = 8, gap = 4;
    switch (gkButtonIconPlacement(view)) {
    case 0:
    case 1:
        width = (float)text.cx + 2 * (edge + icon + gap);
        height = std::max(height, icon + 12);
        break;
    case 2:
    case 3:
        width = std::max((float)text.cx, icon) + 24;
        height = line + icon + gap + 12;
        break;
    case 4:
        width = icon + 24;
        height = std::max(height, icon + 12);
        break;
    default:
        break;
    }
    return D2D1::SizeF(width > 64 || gkButtonIconPlacement(view) == 4 ? width : 64, height);
}

extern "C" void *gkWin32UICreateButton(uintptr_t window, const char *text, uintptr_t handle) {
    std::wstring value = gkWide(text);
    GKView *view = gkNative(window, WC_BUTTONW, value.c_str(), BS_PUSHBUTTON | WS_TABSTOP, 0, GK_KIND_BUTTON, handle);
    if (view) {
        view->text = value;
        view->ext.reset(new GKButton());
    }
    return view;
}

/* ---- Text boxes ---- */

GKField *gkFieldOf(GKView *view) {
    return view ? dynamic_cast<GKField *>(view->ext.get()) : nullptr;
}

static int gkFieldPadding = 6;

int gkTextAreaTabMovesFocus(GKView *view);

void GKField::layout(GKView *view) {
    RECT client;
    GetClientRect(view->outer, &client);
    double scale = gkViewScaleOf(view);
    int pad = (int)lround(gkFieldPadding * scale);
    int border = (int)lround(1 * scale);
    int left = pad, right = client.right - pad;
    GKFont font = gkViewFont(view, gkBodyFont());
    if (prefix && IsWindowVisible(prefix)) {
        int width = (int)lround(gkMeasureGDI(gkWindowText(prefix), font).cx * scale);
        SetWindowPos(prefix, nullptr, left, border, width, client.bottom - 2 * border, SWP_NOZORDER | SWP_NOACTIVATE);
        left += width + (int)lround(4 * scale);
    }
    if (updown) {
        int width = (int)lround(18 * scale);
        SetWindowPos(updown, nullptr, client.right - border - width, border, width, client.bottom - 2 * border,
                     SWP_NOZORDER | SWP_NOACTIVATE);
        right = client.right - border - width - (int)lround(2 * scale);
    }
    if (suffix && IsWindowVisible(suffix)) {
        int width = (int)lround(gkMeasureGDI(gkWindowText(suffix), font).cx * scale);
        right -= width;
        SetWindowPos(suffix, nullptr, right, border, width, client.bottom - 2 * border, SWP_NOZORDER | SWP_NOACTIVATE);
        right -= (int)lround(4 * scale);
    }
    if (multiline) {
        SetWindowPos(edit, nullptr, border + (int)lround(2 * scale), border + (int)lround(2 * scale),
                     client.right - 2 * border - (int)lround(2 * scale),
                     client.bottom - 2 * border - (int)lround(3 * scale), SWP_NOZORDER | SWP_NOACTIVATE);
        return;
    }
    int line = (int)lround(gkMeasureGDI(L"Ag", font).cy * scale);
    int top = (client.bottom - line) / 2;
    SetWindowPos(edit, nullptr, left, top > border ? top : border, right - left > 1 ? right - left : 1, line,
                 SWP_NOZORDER | SWP_NOACTIVATE);
}

static HTHEME gkEditTheme(HWND hwnd) {
    return OpenThemeData(hwnd, L"Edit");
}

bool GKField::paintGDI(GKView *view, HWND hwnd, HDC hdc, RECT const &client) {
    int focused = GetFocus() == edit;
    int theme = gkViewTheme(view);
    int enabled = IsWindowEnabled(edit);
    GKPalette const &palette = gkPalette(theme);
    GKColor fill = palette.dark ? palette.inputActive : gkRGB(0xffffff);
    if (view->readOnly && !multiline) {
        fill = palette.dark ? palette.layer : gkRGB(0xf9f9f9);
    }
    if (view->style.hasBackground) {
        fill = gkMix(fill, view->style.background);
    }
    HBRUSH brush = CreateSolidBrush(RGB((int)(fill.r * 255), (int)(fill.g * 255), (int)(fill.b * 255)));
    FillRect(hdc, &client, brush);
    DeleteObject(brush);
    HTHEME themeData = theme == 2 ? nullptr : gkEditTheme(hwnd);
    if (themeData) {
        int state = !enabled ? EPSN_DISABLED : focused ? EPSN_FOCUSED : hover ? EPSN_HOT : EPSN_NORMAL;
        DrawThemeBackground(themeData, hdc, EP_EDITBORDER_NOSCROLL, state, &client, nullptr);
        CloseThemeData(themeData);
    } else {
        GKColor color = focused ? palette.accent : hover ? palette.controlStrong : palette.controlStroke;
        HBRUSH frame = CreateSolidBrush(RGB((int)(color.r * 255), (int)(color.g * 255), (int)(color.b * 255)));
        FrameRect(hdc, &client, frame);
        DeleteObject(frame);
    }
    return true;
}

static void gkFieldRefresh(GKView *view) {
    if (view && view->outer) {
        InvalidateRect(view->outer, nullptr, FALSE);
    }
}

bool GKField::message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) {
    if (hwnd == edit) {
        switch (message) {
        case WM_SETFOCUS:
            gkFieldRefresh(view);
            break;
        case WM_KILLFOCUS:
            gkFieldRefresh(view);
            focusLost(view);
            break;
        case WM_MOUSEMOVE:
            if (!hover) {
                hover = 1;
                TRACKMOUSEEVENT track = {sizeof(track), TME_LEAVE, edit, 0};
                TrackMouseEvent(&track);
                gkFieldRefresh(view);
            }
            break;
        case WM_MOUSELEAVE: {
            POINT cursor;
            GetCursorPos(&cursor);
            if (WindowFromPoint(cursor) != view->outer) {
                hover = 0;
                gkFieldRefresh(view);
            }
            break;
        }
        case WM_GETDLGCODE:
            if (multiline && lparam && reinterpret_cast<MSG *>(lparam)->wParam == VK_TAB &&
                gkTextAreaTabMovesFocus(view)) {
                *result = gkComctl().DefSubclassProc(hwnd, message, wparam, lparam) & ~(LRESULT)DLGC_WANTALLKEYS &
                          ~(LRESULT)DLGC_WANTTAB;
                return true;
            }
            break;
        case WM_KEYDOWN:
            if (wparam == VK_RETURN && !multiline) {
                submitted(view);
                *result = 0;
                return true;
            }
            break;
        case WM_CHAR:
            if ((wparam == '\r' || wparam == '\n') && !multiline) {
                *result = 0;
                return true;
            }
            if (wparam == 1 && (GetKeyState(VK_CONTROL) & 0x8000)) {
                /* Control+A selects all, which edit controls do not do. */
                SendMessageW(edit, EM_SETSEL, 0, -1);
                *result = 0;
                return true;
            }
            break;
        case WM_PAINT:
            if (multiline && !view->placeholder.empty() && GetWindowTextLengthW(edit) == 0 && GetFocus() != edit) {
                *result = gkComctl().DefSubclassProc(hwnd, message, wparam, lparam);
                HDC hdc = GetDC(edit);
                RECT rect;
                SendMessageW(edit, EM_GETRECT, 0, (LPARAM)&rect);
                HGDIOBJ previous = SelectObject(hdc, view->font);
                GKColor color = gkViewPalette(view).textTertiary;
                SetTextColor(hdc, RGB((int)(color.r * 255), (int)(color.g * 255), (int)(color.b * 255)));
                SetBkMode(hdc, TRANSPARENT);
                DrawTextW(hdc, view->placeholder.c_str(), (int)view->placeholder.size(), &rect,
                          DT_WORDBREAK | DT_NOPREFIX | DT_EDITCONTROL);
                SelectObject(hdc, previous);
                ReleaseDC(edit, hdc);
                return true;
            }
            break;
        default:
            break;
        }
        return false;
    }
    if (hwnd == view->outer) {
        switch (message) {
        case WM_SIZE:
            layout(view);
            return false;
        case WM_LBUTTONDOWN:
            SetFocus(edit);
            *result = 0;
            return true;
        case WM_MOUSEMOVE:
            if (!hover) {
                hover = 1;
                TRACKMOUSEEVENT track = {sizeof(track), TME_LEAVE, view->outer, 0};
                TrackMouseEvent(&track);
                gkFieldRefresh(view);
            }
            return false;
        case WM_MOUSELEAVE: {
            POINT cursor;
            GetCursorPos(&cursor);
            HWND under = WindowFromPoint(cursor);
            if (under != edit) {
                hover = 0;
                gkFieldRefresh(view);
            }
            return false;
        }
        case WM_SETCURSOR:
            SetCursor(LoadCursorW(nullptr, IDC_IBEAM));
            *result = TRUE;
            return true;
        default:
            return false;
        }
    }
    return false;
}

void gkReport(GKView *view, int kind, double x, double y, const char *text) {
    if (view && !view->suppress) {
        gkSend(view->handle, kind, 0, 0, x, y, text);
    }
}

void GKField::changed(GKView *view) {
    if (view->suppress) {
        return;
    }
    if (report) {
        gkReport(view, GK_EVENT_TEXT_CHANGE, 0, 0, nullptr);
    } else {
        gkViewAction(view, GK_UI_EVENT_TEXT_CHANGE);
    }
}

void GKField::submitted(GKView *view) {
    if (report) {
        gkReport(view, GK_EVENT_SUBMIT, 0, 0, nullptr);
    } else {
        gkViewAction(view, GK_UI_EVENT_SUBMIT);
    }
}

bool GKField::command(GKView *view, HWND from, int code) {
    if (from == edit && code == EN_CHANGE) {
        changed(view);
        return true;
    }
    return false;
}

D2D1_SIZE_F GKField::preferred(GKView *view) {
    if (multiline) {
        return D2D1::SizeF(200, 96);
    }
    float width = 160;
    if (updown) {
        width = 100;
    }
    return D2D1::SizeF(width, (float)(gkTextHeight(view) + 8));
}

void GKField::themeChanged(GKView *view) {
    gkApplyThemeTo(edit, gkViewTheme(view));
    if (updown) {
        gkApplyThemeTo(updown, gkViewTheme(view));
    }
    gkFieldRefresh(view);
}

GKView *gkFieldCreate(uintptr_t window, int kind, DWORD editStyle, uintptr_t action, GKField *field) {
    if (!field) {
        field = new GKField();
    }
    field->multiline = (editStyle & ES_MULTILINE) != 0;
    GKView *view = gkCreateDrawn(window, kind, field, action, 0);
    field->edit = gkMakeNative(WC_EDITW, L"", editStyle | WS_TABSTOP, 0, view->outer);
    view->control = field->edit;
    SetPropW(field->edit, L"GKView", view);
    gkSubclass(field->edit);
    gkApplyFont(view);
    gkApplyThemeTo(field->edit, gkViewTheme(view));
    SendMessageW(field->edit, EM_SETMARGINS, EC_LEFTMARGIN | EC_RIGHTMARGIN, 0);
    return view;
}

extern "C" void *gkWin32UICreateTextField(uintptr_t window, uintptr_t handle) {
    return gkFieldCreate(window, GK_KIND_TEXT_FIELD, ES_AUTOHSCROLL | ES_LEFT, handle);
}

extern "C" void *gkWin32UICreateTextArea(uintptr_t window, const char *text, const char *placeholder, int readOnly,
                                         int wrap, uintptr_t handle) {
    DWORD style = ES_MULTILINE | ES_WANTRETURN | ES_AUTOVSCROLL | WS_VSCROLL;
    if (!wrap) {
        style |= ES_AUTOHSCROLL | WS_HSCROLL;
    }
    if (readOnly) {
        style |= ES_READONLY;
    }
    GKView *view = gkFieldCreate(window, GK_KIND_TEXT_AREA, style, handle);
    GKField *field = gkFieldOf(view);
    view->readOnly = readOnly != 0;
    view->placeholder = gkWide(placeholder);
    view->suppress++;
    std::wstring value = gkWide(text);
    std::wstring converted;
    for (wchar_t c : value) {
        if (c == L'\n') {
            converted += L"\r\n";
        } else if (c != L'\r') {
            converted += c;
        }
    }
    SetWindowTextW(field->edit, converted.c_str());
    view->suppress--;
    return view;
}

/* ---- Check boxes ---- */

struct GKCheckBox : GKExtension {
    bool command(GKView *view, HWND from, int code) override {
        (void)from;
        if (code == BN_CLICKED) {
            if (!view->suppress) {
                gkViewAction(view, GK_UI_EVENT_TOGGLE_CHANGE);
            }
            return true;
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        GKFont font = gkViewFont(view, gkBodyFont());
        SIZE text = gkMeasureGDI(view->text, font);
        int height = gkTextHeight(view);
        float width = (float)(text.cx + 22), total = (float)(height > 17 ? height + 4 : 21);
        /* A description is a second line below the label. */
        std::wstring description = gkCheckBoxDescription(view);
        if (!description.empty()) {
            width = std::max(width, (float)gkMeasureGDI(description, font).cx + 22);
            total += (float)gkMeasureGDI(description, font).cy + 2;
        }
        return D2D1::SizeF(width, total);
    }
    void layout(GKView *view) override {
        /* A check box with its label before the box draws the box at its
           trailing edge, so it keeps its own width for the box to follow
           the label. */
        if (gkCheckBoxLabelBefore(view) && view->width > 0) {
            D2D1_SIZE_F size = preferred(view);
            gkPlaceWindow(view->outer, view->x, view->y, std::min<double>(view->width, size.width + 4), view->height);
        }
    }
};

extern "C" void *gkWin32UICreateCheckBox(uintptr_t window, const char *text, int checked, uintptr_t handle) {
    std::wstring value = gkWide(text);
    GKView *view = gkNative(window, WC_BUTTONW, value.c_str(), BS_AUTOCHECKBOX | WS_TABSTOP, 0, GK_KIND_CHECK_BOX,
                            handle);
    if (view) {
        view->text = value;
        view->ext.reset(new GKCheckBox());
        SendMessageW(view->outer, BM_SETCHECK, checked ? BST_CHECKED : BST_UNCHECKED, 0);
    }
    return view;
}

/* ---- Radio groups ---- */

/* GKRadioGroup holds a radio button for each choice in a layout. */
struct GKRadioGroup : GKExtension {
    std::vector<HWND> buttons;
    int horizontal = 0;
    std::wstring label;
    HWND title = nullptr;
    bool command(GKView *view, HWND from, int code) override {
        if (code != BN_CLICKED) {
            return false;
        }
        for (size_t i = 0; i < buttons.size(); i++) {
            if (buttons[i] == from) {
                if (view->selected != (int)i) {
                    view->selected = (int)i;
                    for (size_t j = 0; j < buttons.size(); j++) {
                        SendMessageW(buttons[j], BM_SETCHECK, j == i ? BST_CHECKED : BST_UNCHECKED, 0);
                    }
                    if (!view->suppress) {
                        gkViewAction(view, GK_UI_EVENT_SELECTION_CHANGE);
                    }
                }
                return true;
            }
        }
        return false;
    }
    void layout(GKView *view) override;
    D2D1_SIZE_F preferred(GKView *view) override;
};

static SIZE gkRadioSize(GKView *view, HWND button) {
    SIZE text = gkMeasureGDI(gkWindowText(button), gkViewFont(view, gkBodyFont()));
    int height = gkTextHeight(view);
    SIZE size = {text.cx + 22, height > 17 ? height + 4 : 21};
    return size;
}

void GKRadioGroup::layout(GKView *view) {
    double x = 0, y = 0;
    if (title && IsWindowVisible(title)) {
        int height = gkTextHeight(view);
        gkPlaceWindow(title, 0, 0, view->width, height);
        y = height + 4;
    }
    for (HWND button : buttons) {
        SIZE size = gkRadioSize(view, button);
        if (horizontal) {
            gkPlaceWindow(button, x, y, size.cx, size.cy);
            x += size.cx + 12;
        } else {
            gkPlaceWindow(button, 0, y, view->width > size.cx ? view->width : size.cx, size.cy);
            y += size.cy + 4;
        }
    }
}

D2D1_SIZE_F GKRadioGroup::preferred(GKView *view) {
    double width = 0, height = 0;
    if (title && IsWindowVisible(title)) {
        SIZE size = gkMeasureGDI(label, gkViewFont(view, gkBodyFont()));
        width = size.cx;
        height = size.cy + 4;
    }
    double rowWidth = 0, rowHeight = 0;
    for (size_t i = 0; i < buttons.size(); i++) {
        SIZE size = gkRadioSize(view, buttons[i]);
        if (horizontal) {
            rowWidth += size.cx + (i ? 12 : 0);
            rowHeight = size.cy > rowHeight ? size.cy : rowHeight;
        } else {
            width = size.cx > width ? size.cx : width;
            height += size.cy + (i ? 4 : 0);
        }
    }
    if (horizontal) {
        width = rowWidth > width ? rowWidth : width;
        height += rowHeight;
    }
    return D2D1::SizeF((float)width, (float)height);
}

extern "C" void *gkWin32UICreateRadioGroup(uintptr_t window, uintptr_t handle) {
    GKView *view = gkHostView(window, GK_KIND_RADIO_GROUP, new GKRadioGroup(), handle);
    view->selected = -1;
    return view;
}

void gkRadioGroupSetChoices(GKView *view, std::vector<std::string> const &titles, const int *enabled) {
    auto group = static_cast<GKRadioGroup *>(view->ext.get());
    for (HWND button : group->buttons) {
        DestroyWindow(button);
    }
    group->buttons.clear();
    for (size_t i = 0; i < titles.size(); i++) {
        HWND button = gkMakeNative(WC_BUTTONW, gkWide(titles[i].c_str()).c_str(),
                                   BS_RADIOBUTTON | WS_TABSTOP | (i == 0 ? WS_GROUP : 0), 0, view->outer);
        SetPropW(button, L"GKView", view);
        SendMessageW(button, WM_SETFONT, (WPARAM)gkFontHandle(gkViewFont(view, gkBodyFont()), GetDpiForWindow(button)),
                     FALSE);
        gkApplyThemeTo(button, gkViewTheme(view));
        EnableWindow(button, (!enabled || enabled[i]) && view->enabled);
        group->buttons.push_back(button);
    }
    if (view->selected >= (int)group->buttons.size()) {
        view->selected = -1;
    }
    for (size_t i = 0; i < group->buttons.size(); i++) {
        SendMessageW(group->buttons[i], BM_SETCHECK, (int)i == view->selected ? BST_CHECKED : BST_UNCHECKED, 0);
    }
    group->layout(view);
}

void gkRadioGroupConfigure(GKView *view, int horizontal, const char *label) {
    auto group = static_cast<GKRadioGroup *>(view->ext.get());
    group->horizontal = horizontal != 0;
    group->label = gkWide(label);
    if (!group->title) {
        group->title = gkMakeNative(WC_STATICW, L"", SS_LEFT | SS_NOPREFIX, 0, view->outer);
        SendMessageW(group->title, WM_SETFONT,
                     (WPARAM)gkFontHandle(gkViewFont(view, gkBodyFont()), GetDpiForWindow(group->title)), FALSE);
    }
    SetWindowTextW(group->title, group->label.c_str());
    ShowWindow(group->title, group->label.empty() ? SW_HIDE : SW_SHOWNA);
    group->layout(view);
}

void gkRadioGroupPreferred(GKView *view, int *width, int *height) {
    D2D1_SIZE_F size = static_cast<GKRadioGroup *>(view->ext.get())->preferred(view);
    *width = (int)ceilf(size.width);
    *height = (int)ceilf(size.height);
}

std::vector<HWND> const *gkRadioButtons(GKView *view) {
    if (!view || view->kind != GK_KIND_RADIO_GROUP) {
        return nullptr;
    }
    return &static_cast<GKRadioGroup *>(view->ext.get())->buttons;
}

/* ---- Selects ---- */

/* GKSelect is a drop-down list in a layout, which centers it vertically,
   because the height of a combo box window includes its list. */
struct GKSelect : GKExtension {
    HWND combo = nullptr;
    int visibleRows = 12;
    int popupWidth = 0;
    std::wstring placeholder;
    bool command(GKView *view, HWND from, int code) override {
        if (from != combo) {
            return false;
        }
        if (view->kind == GK_KIND_COMBO) {
            /* An editable combo box reports its text. */
            if (code == CBN_SELCHANGE) {
                int index = (int)SendMessageW(combo, CB_GETCURSEL, 0, 0);
                int length = index >= 0 ? (int)SendMessageW(combo, CB_GETLBTEXTLEN, index, 0) : 0;
                std::wstring item((size_t)(length > 0 ? length : 0), L'\0');
                if (length > 0) {
                    SendMessageW(combo, CB_GETLBTEXT, index, (LPARAM)&item[0]);
                }
                std::string text = gkUTF8(item);
                gkReport(view, GK_EVENT_TEXT_CHANGE, 0, 0, text.c_str());
            } else if (code == CBN_EDITCHANGE) {
                gkReport(view, GK_EVENT_TEXT_CHANGE, 0, 0, nullptr);
            } else if (code == CBN_KILLFOCUS) {
                gkReport(view, GK_EVENT_SUBMIT, 0, 0, nullptr);
            }
            return true;
        }
        if (code == CBN_SELCHANGE) {
            view->selected = (int)SendMessageW(combo, CB_GETCURSEL, 0, 0);
            if (!view->suppress) {
                gkViewAction(view, GK_UI_EVENT_SELECTION_CHANGE);
            }
            return true;
        }
        return false;
    }
    void layout(GKView *view) override {
        RECT client;
        GetClientRect(view->outer, &client);
        int height = (int)SendMessageW(combo, CB_GETITEMHEIGHT, (WPARAM)-1, 0) + 8;
        double scale = gkViewScaleOf(view);
        int list = (int)lround(visibleRows * (gkTextHeight(view) + 4) * scale);
        int top = (client.bottom - height) / 2;
        SetWindowPos(combo, nullptr, 0, top > 0 ? top : 0, client.right, height + list, SWP_NOZORDER | SWP_NOACTIVATE);
        if (popupWidth > 0) {
            SendMessageW(combo, CB_SETDROPPEDWIDTH, (WPARAM)lround(popupWidth * scale), 0);
        }
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)lparam;
        if (hwnd == view->outer && message == WM_SIZE) {
            layout(view);
        }
        if (hwnd != view->outer && hwnd != combo && message == WM_KEYDOWN && wparam == VK_RETURN &&
            view->kind == GK_KIND_COMBO) {
            gkReport(view, GK_EVENT_SUBMIT, 0, 0, nullptr);
            *result = 0;
            return true;
        }
        if (hwnd != view->outer && hwnd != combo && message == WM_CHAR && wparam == L'\r') {
            *result = 0;
            return true;
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        GKFont font = gkViewFont(view, gkBodyFont());
        int width = gkMeasureGDI(placeholder, font).cx;
        int count = (int)SendMessageW(combo, CB_GETCOUNT, 0, 0);
        for (int i = 0; i < count; i++) {
            int length = (int)SendMessageW(combo, CB_GETLBTEXTLEN, i, 0);
            std::wstring item((size_t)(length > 0 ? length : 0), L'\0');
            if (length > 0) {
                SendMessageW(combo, CB_GETLBTEXT, i, (LPARAM)&item[0]);
            }
            int itemWidth = gkMeasureGDI(item, font).cx;
            width = itemWidth > width ? itemWidth : width;
        }
        return D2D1::SizeF((float)(width + 34 > 96 ? width + 34 : 96), (float)(gkTextHeight(view) + 8));
    }
    void themeChanged(GKView *view) override {
        gkApplyThemeTo(combo, gkViewTheme(view));
    }
};

GKView *gkSelectCreate(uintptr_t window, int kind, DWORD style, uintptr_t action) {
    GKSelect *select = new GKSelect();
    GKView *view = gkHostView(window, kind, select, action);
    select->combo = gkMakeNative(WC_COMBOBOXW, L"", style | WS_TABSTOP | WS_VSCROLL, 0, view->outer);
    view->control = select->combo;
    SetPropW(select->combo, L"GKView", view);
    gkSubclass(select->combo);
    COMBOBOXINFO info = {sizeof(info)};
    if (GetComboBoxInfo(select->combo, &info) && info.hwndItem) {
        SetPropW(info.hwndItem, L"GKView", view);
        gkSubclass(info.hwndItem);
    }
    gkApplyFont(view);
    gkApplyThemeTo(select->combo, gkViewTheme(view));
    view->selected = -1;
    return view;
}

HWND gkSelectCombo(GKView *view) {
    GKSelect *select = view ? dynamic_cast<GKSelect *>(view->ext.get()) : nullptr;
    return select ? select->combo : nullptr;
}

void gkSelectConfigure(GKView *view, const char *placeholder, int popupWidth) {
    GKSelect *select = dynamic_cast<GKSelect *>(view->ext.get());
    if (!select) {
        return;
    }
    select->placeholder = gkWide(placeholder);
    select->popupWidth = popupWidth;
    SendMessageW(select->combo, CB_SETCUEBANNER, 0, (LPARAM)select->placeholder.c_str());
    select->layout(view);
}

void gkSelectSetVisibleRows(GKView *view, int rows) {
    GKSelect *select = dynamic_cast<GKSelect *>(view->ext.get());
    if (select) {
        select->visibleRows = rows > 0 ? rows : 12;
        SendMessageW(select->combo, CB_SETMINVISIBLE, select->visibleRows, 0);
        select->layout(view);
    }
}

extern "C" void *gkWin32UICreateSelect(uintptr_t window, uintptr_t handle) {
    return gkSelectCreate(window, GK_KIND_SELECT, CBS_DROPDOWNLIST, handle);
}

extern "C" void gkWin32UISelectionAddItem(void *control, const char *text) {
    GKView *view = gkView(control);
    HWND combo = gkSelectCombo(view);
    if (combo) {
        SendMessageW(combo, CB_ADDSTRING, 0, (LPARAM)gkWide(text).c_str());
    }
}

extern "C" int gkWin32UISelectionIndex(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return -1;
    }
    if (HWND combo = gkSelectCombo(view)) {
        return (int)SendMessageW(combo, CB_GETCURSEL, 0, 0);
    }
    return view->selected;
}

extern "C" void gkWin32UISetSelectionIndex(void *control, int index) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->suppress++;
    view->selected = index;
    if (HWND combo = gkSelectCombo(view)) {
        SendMessageW(combo, CB_SETCURSEL, (WPARAM)index, 0);
    } else if (auto buttons = gkRadioButtons(view)) {
        for (size_t i = 0; i < buttons->size(); i++) {
            SendMessageW((*buttons)[i], BM_SETCHECK, (int)i == index ? BST_CHECKED : BST_UNCHECKED, 0);
        }
    }
    view->suppress--;
}

/* ---- Sliders ---- */

/* GKSlider is a trackbar. The trackbar holds integer positions, so a value
   is a position times the step, or one of 10000 positions without a
   step. */
struct GKSlider : GKExtension {
    int positions = 10000;
    double frequency = 0;
    int snap = 0;
    int position(GKView *view, double value) {
        double span = view->maximum - view->minimum;
        if (span <= 0) {
            return 0;
        }
        double fraction = (value - view->minimum) / span;
        if (view->inverted) {
            fraction = 1 - fraction;
        }
        if (view->vertical) {
            fraction = 1 - fraction;
        }
        return (int)lround(fraction * positions);
    }
    double value(GKView *view, int position) {
        double fraction = positions > 0 ? (double)position / positions : 0;
        if (view->vertical) {
            fraction = 1 - fraction;
        }
        if (view->inverted) {
            fraction = 1 - fraction;
        }
        double result = view->minimum + fraction * (view->maximum - view->minimum);
        if (view->step > 0) {
            result = view->minimum + round((result - view->minimum) / view->step) * view->step;
        }
        return gkClamp(result, view->minimum, view->maximum);
    }
    void configure(GKView *view) {
        double span = view->maximum - view->minimum;
        positions = view->step > 0 && span > 0 ? (int)lround(span / view->step) : 10000;
        if (positions <= 0) {
            positions = 1;
        }
        HWND bar = view->outer;
        SendMessageW(bar, TBM_SETRANGEMIN, FALSE, 0);
        SendMessageW(bar, TBM_SETRANGEMAX, FALSE, positions);
        SendMessageW(bar, TBM_SETLINESIZE, 0, view->step > 0 ? 1 : positions / 100);
        SendMessageW(bar, TBM_SETPAGESIZE, 0, positions / 10 > 0 ? positions / 10 : 1);
        SendMessageW(bar, TBM_CLEARTICS, TRUE, 0);
        if (frequency > 0 && span > 0) {
            int ticks = (int)lround(span / frequency);
            if (ticks > 0 && ticks <= 200) {
                LONG_PTR style = GetWindowLongPtrW(bar, GWL_STYLE);
                SetWindowLongPtrW(bar, GWL_STYLE, (style & ~TBS_NOTICKS) | TBS_AUTOTICKS);
                SendMessageW(bar, TBM_SETTICFREQ, (WPARAM)lround(positions / (double)ticks), 0);
            }
        } else {
            LONG_PTR style = GetWindowLongPtrW(bar, GWL_STYLE);
            SetWindowLongPtrW(bar, GWL_STYLE, (style & ~TBS_AUTOTICKS) | TBS_NOTICKS);
        }
        SendMessageW(bar, TBM_SETPOS, TRUE, position(view, view->value));
        InvalidateRect(bar, nullptr, TRUE);
    }
    bool scroll(GKView *view, HWND from, int code) override {
        (void)from;
        if (code == TB_ENDTRACK) {
            return true;
        }
        int pos = (int)SendMessageW(view->outer, TBM_GETPOS, 0, 0);
        double next = value(view, pos);
        if (snap && frequency > 0) {
            next = view->minimum + round((next - view->minimum) / frequency) * frequency;
            next = gkClamp(next, view->minimum, view->maximum);
            SendMessageW(view->outer, TBM_SETPOS, TRUE, position(view, next));
        }
        if (next != view->value) {
            view->value = next;
            if (!view->suppress) {
                gkViewAction(view, GK_UI_EVENT_VALUE_CHANGE);
            }
        }
        return true;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        float thickness = frequency > 0 ? 32.0f : 26.0f;
        return view->vertical ? D2D1::SizeF(thickness, 160) : D2D1::SizeF(160, thickness);
    }
};

GKSlider *gkSliderOf(GKView *view) {
    return view ? dynamic_cast<GKSlider *>(view->ext.get()) : nullptr;
}

void gkSliderConfigure(GKView *view, int vertical, double minimum, double maximum, double frequency, int snap) {
    GKSlider *slider = gkSliderOf(view);
    if (!slider) {
        return;
    }
    view->suppress++;
    view->vertical = vertical != 0;
    view->minimum = minimum;
    view->maximum = maximum > minimum ? maximum : minimum;
    view->value = gkClamp(view->value, view->minimum, view->maximum);
    slider->frequency = frequency;
    slider->snap = snap != 0;
    LONG_PTR style = GetWindowLongPtrW(view->outer, GWL_STYLE);
    style = vertical ? ((style & ~TBS_HORZ) | TBS_VERT) : ((style & ~TBS_VERT) | TBS_HORZ);
    SetWindowLongPtrW(view->outer, GWL_STYLE, style);
    slider->configure(view);
    view->suppress--;
}

void gkSliderSetInverted(GKView *view, int inverted) {
    GKSlider *slider = gkSliderOf(view);
    if (slider) {
        view->inverted = inverted != 0;
        view->suppress++;
        slider->configure(view);
        view->suppress--;
    }
}

extern "C" void *gkWin32UICreateSlider(uintptr_t window, double minimum, double maximum, double value, double step,
                                       uintptr_t handle) {
    GKView *view = gkNative(window, TRACKBAR_CLASSW, L"", TBS_HORZ | TBS_NOTICKS | TBS_BOTH | WS_TABSTOP, 0,
                            GK_KIND_SLIDER, handle);
    if (!view) {
        return nullptr;
    }
    GKSlider *slider = new GKSlider();
    view->ext.reset(slider);
    view->minimum = minimum;
    view->maximum = maximum > minimum ? maximum : minimum;
    view->step = step > 0 ? step : 0;
    view->value = gkClamp(value, view->minimum, view->maximum);
    view->suppress++;
    slider->configure(view);
    view->suppress--;
    return view;
}

extern "C" double gkWin32UISliderValue(void *control) {
    GKView *view = gkView(control);
    return view ? view->value : 0;
}

extern "C" void gkWin32UISetSliderValue(void *control, double value) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->value = gkClamp(value, view->minimum, view->maximum);
    if (GKSlider *slider = gkSliderOf(view)) {
        view->suppress++;
        SendMessageW(view->outer, TBM_SETPOS, TRUE, slider->position(view, view->value));
        view->suppress--;
    }
}

/* ---- Progress ---- */

struct GKProgress : GKExtension {
    int indeterminate = 0;
    D2D1_SIZE_F preferred(GKView *view) override {
        return view->vertical ? D2D1::SizeF(6, 160) : D2D1::SizeF(160, 6);
    }
};

extern "C" void *gkWin32UICreateProgressBar(uintptr_t window, double value) {
    GKView *view = gkNative(window, PROGRESS_CLASSW, L"", 0, 0, GK_KIND_PROGRESS, 0);
    if (!view) {
        return nullptr;
    }
    view->ext.reset(new GKProgress());
    SendMessageW(view->outer, PBM_SETRANGE32, 0, 10000);
    view->value = gkClamp(value, 0, 1);
    SendMessageW(view->outer, PBM_SETPOS, (WPARAM)lround(view->value * 10000), 0);
    return view;
}

extern "C" double gkWin32UIProgressValue(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_PROGRESS ? view->value : 0;
}

extern "C" void gkWin32UISetProgressValue(void *control, double value) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_PROGRESS) {
        view->value = gkClamp(value, 0, 1);
        HWND bar = gkViewTarget(view);
        SendMessageW(bar, PBM_SETPOS, (WPARAM)lround(view->value * 10000), 0);
    }
}

/* ---- Activity indicators ----

   Windows has no control for an activity indicator, so the bridge draws a
   rotating arc like the one of Windows. */

struct GKActivity : GKExtension {
    UINT_PTR timer = 0;
    float angle = 0;
    int size = 1;
    ~GKActivity() override {
        gkTimerCancel(timer);
    }
    void paint(GKView *view, GKCanvas &canvas) override {
        if (!view->running) {
            return;
        }
        float diameter = canvas.width < canvas.height ? canvas.width : canvas.height;
        float thickness = diameter / 10 > 1.5f ? diameter / 10 : 1.5f;
        float radius = diameter / 2 - thickness;
        float cx = canvas.width / 2, cy = canvas.height / 2;
        float sweep = (float)(M_PI * (0.6 + 0.4 * sin(angle * 0.7)));
        canvas.arc(cx, cy, radius, angle, sweep, gkViewPalette(view).accent, thickness);
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        float side = size == 0 ? 16.0f : size == 2 ? 32.0f : 20.0f;
        return D2D1::SizeF(side, side);
    }
};

void gkActivityUpdate(GKView *view) {
    auto activity = static_cast<GKActivity *>(view->ext.get());
    if (view->running && !activity->timer) {
        activity->timer = gkTimer(16, [view]() {
            if (!gkView(view)) {
                return false;
            }
            auto activity = static_cast<GKActivity *>(view->ext.get());
            activity->angle += 0.12f;
            if (IsWindowVisible(view->outer)) {
                InvalidateRect(view->outer, nullptr, FALSE);
            }
            return true;
        });
    } else if (!view->running && activity->timer) {
        gkTimerCancel(activity->timer);
        activity->timer = 0;
    }
    InvalidateRect(view->outer, nullptr, FALSE);
}

void gkActivitySetSize(GKView *view, int size) {
    if (view && view->kind == GK_KIND_ACTIVITY) {
        static_cast<GKActivity *>(view->ext.get())->size = size;
    }
}

GKView *gkActivityCreateIn(HWND parent) {
    GKView *view = gkCreateDrawn(0, GK_KIND_ACTIVITY, new GKActivity(), 0, 0);
    static_cast<GKActivity *>(view->ext.get())->size = 0;
    gkReparent(view->outer, parent);
    EnableWindow(view->outer, FALSE);
    return view;
}

extern "C" void *gkWin32UICreateActivityIndicator(uintptr_t window, int running) {
    GKView *view = gkCreateDrawn(window, GK_KIND_ACTIVITY, new GKActivity(), 0, 0);
    view->running = running != 0;
    gkActivityUpdate(view);
    return view;
}

extern "C" int gkWin32UIActivityRunning(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_ACTIVITY && view->running;
}

extern "C" void gkWin32UISetActivityRunning(void *control, int running) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_ACTIVITY) {
        view->running = running != 0;
        gkActivityUpdate(view);
    }
}

/* ---- Separators ---- */

struct GKSeparator : GKExtension {
    std::wstring label;
    int inset = 0;
    int thickness = 1;
    void paint(GKView *view, GKCanvas &canvas) override {
        GKPalette const &palette = gkViewPalette(view);
        float t = (float)thickness;
        if (label.empty()) {
            if (view->vertical) {
                float x = floorf(canvas.width / 2);
                canvas.fillRect(x, (float)inset, t, canvas.height - 2 * inset, palette.divider);
            } else {
                float y = floorf(canvas.height / 2);
                canvas.fillRect((float)inset, y, canvas.width - 2 * inset, t, palette.divider);
            }
            return;
        }
        GKFont font = gkTypeRamp(1);
        D2D1_SIZE_F text = gkMeasureText(label, font);
        if (view->vertical) {
            float x = floorf(canvas.width / 2);
            float middle = canvas.height / 2;
            canvas.fillRect(x, (float)inset, t, middle - text.height / 2 - 4 - inset, palette.divider);
            canvas.fillRect(x, middle + text.height / 2 + 4, t, middle - text.height / 2 - 4 - inset, palette.divider);
            canvas.text(label, font, 0, middle - text.height / 2, canvas.width, text.height, palette.textSecondary, 1);
        } else {
            float y = floorf(canvas.height / 2);
            float middle = canvas.width / 2;
            canvas.fillRect((float)inset, y, middle - text.width / 2 - 6 - inset, t, palette.divider);
            canvas.fillRect(middle + text.width / 2 + 6, y, middle - text.width / 2 - 6 - inset, t, palette.divider);
            canvas.text(label, font, middle - text.width / 2 - 1, 0, text.width + 2, canvas.height,
                        palette.textSecondary, 1);
        }
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        float t = (float)thickness;
        if (!label.empty()) {
            D2D1_SIZE_F text = gkMeasureText(label, gkTypeRamp(1));
            return view->vertical ? D2D1::SizeF(text.width, text.height + 24) : D2D1::SizeF(text.width + 24, text.height);
        }
        return view->vertical ? D2D1::SizeF(t, 0) : D2D1::SizeF(0, t);
    }
};

void gkSeparatorConfigure(GKView *view, int vertical, const char *label, int inset, int thickness) {
    if (!view || view->kind != GK_KIND_SEPARATOR) {
        return;
    }
    auto separator = static_cast<GKSeparator *>(view->ext.get());
    view->vertical = vertical != 0;
    separator->label = gkWide(label);
    separator->inset = inset > 0 ? inset : 0;
    separator->thickness = thickness > 0 ? thickness : 1;
    InvalidateRect(view->outer, nullptr, TRUE);
}

std::wstring gkSeparatorLabel(GKView *view) {
    return view && view->kind == GK_KIND_SEPARATOR ? static_cast<GKSeparator *>(view->ext.get())->label
                                                   : std::wstring();
}

extern "C" void *gkWin32UICreateSeparator(uintptr_t window, int direction) {
    GKView *view = gkCreateDrawn(window, GK_KIND_SEPARATOR, new GKSeparator(), 0, 0);
    view->vertical = direction == 1;
    return view;
}

/* ---- Images ---- */

struct GKImage : GKExtension {
    std::vector<unsigned char> pixels;
    int width = 0, height = 0;
    ID2D1Bitmap *bitmap = nullptr;
    ID2D1RenderTarget *owner = nullptr;
    ~GKImage() override {
        if (bitmap) {
            bitmap->Release();
        }
    }
    void paint(GKView *view, GKCanvas &canvas) override {
        if (pixels.empty() || width <= 0 || height <= 0) {
            return;
        }
        if (!bitmap || owner != canvas.target) {
            if (bitmap) {
                bitmap->Release();
            }
            bitmap = gkBitmapFromRGBA(canvas.target, pixels.data(), width, height, width * 4);
            owner = canvas.target;
        }
        double sourceScale = view->sourceScale > 0 ? view->sourceScale : 1;
        double sourceWidth = width / sourceScale, sourceHeight = height / sourceScale;
        double w = canvas.width, h = canvas.height, x = 0, y = 0;
        if (view->scaling != 2) {
            double sx = canvas.width / sourceWidth, sy = canvas.height / sourceHeight;
            double scale = view->scaling == 1 ? (sx > sy ? sx : sy) : (sx < sy ? sx : sy);
            w = sourceWidth * scale;
            h = sourceHeight * scale;
            /* Anchor order matches ui.Anchor: center, then clockwise from
               top-left. */
            static const double horizontal[] = {0.5, 0, 0.5, 1, 0, 1, 0, 0.5, 1};
            static const double vertical[] = {0.5, 0, 0, 0, 0.5, 0.5, 1, 1, 1};
            int anchor = view->alignment >= 0 && view->alignment < 9 ? view->alignment : 0;
            x = (canvas.width - w) * horizontal[anchor];
            y = (canvas.height - h) * vertical[anchor];
        }
        canvas.clip(0, 0, canvas.width, canvas.height);
        canvas.bitmap(bitmap, (float)x, (float)y, (float)w, (float)h, 1, view->nearest);
        canvas.unclip();
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        double scale = view->sourceScale > 0 ? view->sourceScale : 1;
        return D2D1::SizeF((float)(width / scale), (float)(height / scale));
    }
};

extern "C" void *gkWin32UICreateImage(uintptr_t window, int scaling) {
    GKView *view = gkCreateDrawn(window, GK_KIND_IMAGE, new GKImage(), 0, 0);
    view->scaling = scaling;
    return view;
}

extern "C" void gkWin32UIImageSet(void *control, const unsigned char *pixels, int width, int height, int stride) {
    GKView *view = gkView(control);
    if (!view || view->kind != GK_KIND_IMAGE || !pixels || width <= 0 || height <= 0) {
        return;
    }
    auto image = static_cast<GKImage *>(view->ext.get());
    image->width = width;
    image->height = height;
    image->pixels.resize((size_t)width * height * 4);
    for (int y = 0; y < height; y++) {
        memcpy(&image->pixels[(size_t)y * width * 4], pixels + (size_t)y * stride, (size_t)width * 4);
    }
    if (image->bitmap) {
        image->bitmap->Release();
        image->bitmap = nullptr;
    }
    InvalidateRect(view->outer, nullptr, FALSE);
}

extern "C" int gkWin32UIImageScaling(void *control) {
    GKView *view = gkView(control);
    return view ? view->scaling : 0;
}

extern "C" void gkWin32UIImageSetScaling(void *control, int scaling) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_IMAGE) {
        view->scaling = scaling;
        InvalidateRect(view->outer, nullptr, FALSE);
    }
}

/* ---- Links ---- */

std::wstring gkLinkMarkup(std::wstring const &text) {
    std::wstring escaped;
    for (wchar_t c : text) {
        if (c == L'<') {
            escaped += L"&lt;";
        } else if (c == L'>') {
            escaped += L"&gt;";
        } else if (c == L'&') {
            escaped += L"&amp;";
        } else {
            escaped += c;
        }
    }
    return L"<a>" + escaped + L"</a>";
}

struct GKLink : GKExtension {
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->code == NM_CLICK || header->code == NM_RETURN) {
            gkViewAction(view, GK_UI_EVENT_CLICK);
            *result = 0;
            return true;
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        SIZE text = gkMeasureGDI(view->text, gkViewFont(view, gkBodyFont()));
        return D2D1::SizeF((float)text.cx + 2, (float)text.cy + 2);
    }
};

extern "C" void *gkWin32UICreateLink(uintptr_t window, const char *text, const char *url, uintptr_t handle) {
    std::wstring value = gkWide(text);
    GKView *view = gkNative(window, WC_LINK, gkLinkMarkup(value).c_str(), WS_TABSTOP | LWS_TRANSPARENT, 0,
                            GK_KIND_LINK, handle);
    if (view) {
        view->text = value;
        view->url = url ? url : "";
        view->ext.reset(new GKLink());
    }
    return view;
}

extern "C" char *gkWin32UILinkURL(void *control) {
    GKView *view = gkView(control);
    return gkStrdup(view ? view->url.c_str() : "");
}

extern "C" void gkWin32UISetLinkURL(void *control, const char *url) {
    GKView *view = gkView(control);
    if (view) {
        view->url = url ? url : "";
    }
}

/* ---- Scroll views ----

   A scroll view is a layout with scroll bars. Its content is a layout that
   moves under it as it scrolls. */

struct GKScroll : GKExtension {
    double contentWidth = 0, contentHeight = 0;
    double top = 0, right = 0, bottom = 0, left = 0;
    double x = 0, y = 0;
    double zoom = 1;
    int policy = 0;
    int insets[4] = {0, 0, 0, 0};
    /* The wheel glides to a target offset, as scrolling in Windows does. */
    double targetX = 0, targetY = 0;
    UINT_PTR glide = 0;
    ~GKScroll() override {
        stopGlide();
    }
    void stopGlide() {
        if (glide) {
            gkTimerCancel(glide);
            glide = 0;
        }
    }
    void glideTo(GKView *view, double nx, double ny) {
        if (!glide) {
            targetX = x;
            targetY = y;
        }
        targetX = nx;
        targetY = ny;
        double width, height;
        viewport(view, &width, &height);
        double maxX = contentWidth * zoom + left + right - width;
        double maxY = contentHeight * zoom + top + bottom - height;
        targetX = gkClamp(targetX, 0, maxX > 0 ? maxX : 0);
        targetY = gkClamp(targetY, 0, maxY > 0 ? maxY : 0);
        if (glide) {
            return;
        }
        BOOL animations = TRUE;
        SystemParametersInfoW(SPI_GETCLIENTAREAANIMATION, 0, &animations, 0);
        if (!animations) {
            scrollTo(view, targetX, targetY);
            return;
        }
        glide = gkTimer(10, [view]() {
            if (!gkView(view)) {
                return false;
            }
            auto scroll = static_cast<GKScroll *>(view->ext.get());
            double dx = scroll->targetX - scroll->x, dy = scroll->targetY - scroll->y;
            bool done = fabs(dx) < 0.5 && fabs(dy) < 0.5;
            scroll->x = done ? scroll->targetX : scroll->x + dx * 0.3;
            scroll->y = done ? scroll->targetY : scroll->y + dy * 0.3;
            scroll->update(view);
            if (done) {
                scroll->glide = 0;
            }
            return !done;
        });
    }
    void viewport(GKView *view, double *width, double *height) {
        D2D1_SIZE_F size = gkClientSize(view->outer);
        *width = size.width;
        *height = size.height;
    }
    void clampOffset(GKView *view) {
        double width, height;
        viewport(view, &width, &height);
        double maxX = contentWidth * zoom + left + right - width;
        double maxY = contentHeight * zoom + top + bottom - height;
        x = gkClamp(x, 0, maxX > 0 ? maxX : 0);
        y = gkClamp(y, 0, maxY > 0 ? maxY : 0);
    }
    void update(GKView *view) {
        double width, height;
        viewport(view, &width, &height);
        int horizontal = view->axes == 1 || view->axes == 2;
        int vertical = view->axes == 0 || view->axes == 2;
        double scale = gkViewScaleOf(view);
        clampOffset(view);
        SCROLLINFO info = {sizeof(info)};
        info.fMask = SIF_RANGE | SIF_PAGE | SIF_POS | (policy == 2 ? SIF_DISABLENOSCROLL : 0);
        info.nMin = 0;
        info.nMax = horizontal ? (int)lround((contentWidth * zoom + left + right) * scale) - 1 : 0;
        info.nPage = horizontal ? (UINT)lround(width * scale) : 0;
        info.nPos = (int)lround(x * scale);
        if (policy != 1) {
            SetScrollInfo(view->outer, SB_HORZ, &info, TRUE);
        }
        info.nMax = vertical ? (int)lround((contentHeight * zoom + top + bottom) * scale) - 1 : 0;
        info.nPage = vertical ? (UINT)lround(height * scale) : 0;
        info.nPos = (int)lround(y * scale);
        if (policy != 1) {
            SetScrollInfo(view->outer, SB_VERT, &info, TRUE);
        } else {
            ShowScrollBar(view->outer, SB_BOTH, FALSE);
        }
        gkPlaceWindow(view->body, left - x, top - y, contentWidth * zoom, contentHeight * zoom);
        gkLayoutSetZoom(view->body, zoom);
    }
    void scrollTo(GKView *view, double nx, double ny) {
        stopGlide();
        x = nx;
        y = ny;
        update(view);
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)lparam;
        if (hwnd != view->outer) {
            return false;
        }
        switch (message) {
        case WM_SIZE:
            update(view);
            return false;
        case WM_HSCROLL:
        case WM_VSCROLL: {
            if (lparam) {
                return false;
            }
            int bar = message == WM_HSCROLL ? SB_HORZ : SB_VERT;
            SCROLLINFO info = {sizeof(info), SIF_ALL};
            GetScrollInfo(view->outer, bar, &info);
            double scale = gkViewScaleOf(view);
            double position = (bar == SB_HORZ ? x : y) * scale;
            double line = 40 * scale;
            switch (LOWORD(wparam)) {
            case SB_LINEUP:
                position -= line;
                break;
            case SB_LINEDOWN:
                position += line;
                break;
            case SB_PAGEUP:
                position -= info.nPage;
                break;
            case SB_PAGEDOWN:
                position += info.nPage;
                break;
            case SB_THUMBTRACK:
            case SB_THUMBPOSITION:
                position = info.nTrackPos;
                break;
            case SB_TOP:
                position = 0;
                break;
            case SB_BOTTOM:
                position = info.nMax;
                break;
            default:
                break;
            }
            stopGlide();
            if (bar == SB_HORZ) {
                x = position / scale;
            } else {
                y = position / scale;
            }
            update(view);
            *result = 0;
            return true;
        }
        case WM_MOUSEWHEEL:
        case WM_MOUSEHWHEEL: {
            double delta = (double)GET_WHEEL_DELTA_WPARAM(wparam) / WHEEL_DELTA;
            UINT lines = 3;
            SystemParametersInfoW(SPI_GETWHEELSCROLLLINES, 0, &lines, 0);
            double distance = delta * (lines ? lines : 3) * 16;
            bool horizontal = message == WM_MOUSEHWHEEL || view->axes == 1 || (GetKeyState(VK_SHIFT) & 0x8000);
            double nx = glide ? targetX : x, ny = glide ? targetY : y;
            if (horizontal) {
                nx += message == WM_MOUSEHWHEEL ? distance : -distance;
            } else {
                ny -= distance;
            }
            glideTo(view, nx, ny);
            *result = 0;
            return true;
        }
        default:
            return false;
        }
    }
};

GKScroll *gkScrollOf(GKView *view) {
    return view && view->kind == GK_KIND_SCROLL ? static_cast<GKScroll *>(view->ext.get()) : nullptr;
}

void gkScrollGetOffset(GKView *view, double *x, double *y) {
    GKScroll *scroll = gkScrollOf(view);
    *x = scroll ? scroll->x : 0;
    *y = scroll ? scroll->y : 0;
}

void gkScrollSetOffset(GKView *view, double x, double y) {
    GKScroll *scroll = gkScrollOf(view);
    if (scroll) {
        scroll->scrollTo(view, x, y);
    }
}

void gkScrollViewport(GKView *view, double *width, double *height) {
    GKScroll *scroll = gkScrollOf(view);
    if (scroll) {
        scroll->viewport(view, width, height);
    } else {
        *width = *height = 0;
    }
}

void gkScrollSetZoom(GKView *view, double zoom) {
    GKScroll *scroll = gkScrollOf(view);
    if (scroll) {
        scroll->zoom = zoom > 0 ? zoom : 1;
        scroll->update(view);
    }
}

double gkScrollGetZoom(GKView *view) {
    GKScroll *scroll = gkScrollOf(view);
    return scroll ? scroll->zoom : 1;
}

void gkScrollSetInsets(GKView *view, int top, int right, int bottom, int left) {
    GKScroll *scroll = gkScrollOf(view);
    if (scroll) {
        scroll->top = top > 0 ? top : 0;
        scroll->right = right > 0 ? right : 0;
        scroll->bottom = bottom > 0 ? bottom : 0;
        scroll->left = left > 0 ? left : 0;
        scroll->update(view);
    }
}

void gkScrollSetPolicy(GKView *view, int policy) {
    GKScroll *scroll = gkScrollOf(view);
    if (scroll) {
        scroll->policy = policy;
        scroll->update(view);
    }
}

extern "C" void *gkWin32UICreateScrollView(uintptr_t window, int axes) {
    GKScroll *scroll = new GKScroll();
    HWND host = gkLayoutNew(gkParking());
    LONG_PTR style = GetWindowLongPtrW(host, GWL_STYLE);
    int horizontal = axes == 1 || axes == 2;
    int vertical = axes == 0 || axes == 2;
    SetWindowLongPtrW(host, GWL_STYLE, style | (horizontal ? WS_HSCROLL : 0) | (vertical ? WS_VSCROLL : 0));
    GKView *view = gkViewNew(host, nullptr, GK_KIND_SCROLL, window, 0);
    view->ext.reset(scroll);
    view->axes = axes;
    view->body = gkLayoutNew(host);
    gkApplyThemeTo(host, gkViewTheme(view));
    scroll->update(view);
    return view;
}

extern "C" void gkWin32UIScrollViewportSize(void *control, int *width, int *height) {
    GKView *view = gkView(control);
    double w = 0, h = 0;
    if (view) {
        gkScrollViewport(view, &w, &h);
    }
    *width = (int)floor(w);
    *height = (int)floor(h);
}

extern "C" void gkWin32UIScrollSetContentSize(void *control, int width, int height) {
    GKView *view = gkView(control);
    GKScroll *scroll = gkScrollOf(view);
    if (scroll) {
        scroll->contentWidth = width > 0 ? width : 0;
        scroll->contentHeight = height > 0 ? height : 0;
        scroll->update(view);
    }
}

extern "C" void gkWin32UIScrollOffset(void *control, int *x, int *y) {
    GKView *view = gkView(control);
    double ox = 0, oy = 0;
    if (view) {
        gkScrollGetOffset(view, &ox, &oy);
    }
    *x = (int)lround(ox);
    *y = (int)lround(oy);
}

/* ---- Canvas ---- */

struct GKCanvasData : GKExtension {
    double lastX = 0, lastY = 0;
    bool haveLast = false;
    bool inside = false;
    bool shown = false;
    int reportedWidth = -1, reportedHeight = -1;
    double reportedScale = 0;
    UINT_PTR frames = 0;
    unsigned int highSurrogate = 0;
    ~GKCanvasData() override {
        gkTimerCancel(frames);
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override;
};

static GKCanvasData *gkCanvasData(GKView *view) {
    return view && view->kind == GK_KIND_CANVAS ? static_cast<GKCanvasData *>(view->ext.get()) : nullptr;
}

static void gkCanvasSend(GKView *view, int kind, int a, int b, int c, double x, double y, double z, double w) {
    if (!view->action) {
        return;
    }
    GKWin32Event event = {};
    event.kind = kind;
    event.a = a;
    event.b = b;
    event.c = c;
    event.x = x;
    event.y = y;
    event.z = z;
    event.w = w;
    gkSendEvent(view->action, &event);
}

static void gkCanvasReportSurface(GKView *view, int kind) {
    GKCanvasData *data = gkCanvasData(view);
    RECT client = {};
    GetClientRect(view->outer, &client);
    double scale = gkViewScaleOf(view);
    if (data && kind == 0 && client.right == data->reportedWidth && client.bottom == data->reportedHeight &&
        scale == data->reportedScale) {
        return;
    }
    if (data) {
        data->reportedWidth = client.right;
        data->reportedHeight = client.bottom;
        data->reportedScale = scale;
    }
    gkCanvasSend(view, GK_EVENT_SURFACE, kind, 1, 0, client.right, client.bottom, scale, 0);
}

static void gkCanvasButton(GKView *view, int button, int action, int mods) {
    if (!view->action || button < 0 || button >= GK_POINTER_BUTTON_COUNT) {
        return;
    }
    int mask = 1 << button;
    if (action == GK_POINTER_PRESSED) {
        view->canvasButtons |= mask;
    } else {
        if (!(view->canvasButtons & mask)) {
            return;
        }
        view->canvasButtons &= ~mask;
    }
    gkWin32UIGoCanvasPointerButton(view->action, button, action, mods);
}

static void gkCanvasShown(GKView *view) {
    GKCanvasData *data = gkCanvasData(view);
    if (!data) {
        return;
    }
    bool shown = IsWindowVisible(view->outer) && gkWinOf(view->outer) != nullptr;
    if (shown != data->shown) {
        data->shown = shown;
        gkCanvasSend(view, GK_EVENT_WINDOW_OCCLUSION, shown ? 0 : 1, 0, 0, 0, 0, 0, 0);
    }
}

bool GKCanvasData::message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) {
    (void)hwnd;
    switch (message) {
    case WM_ERASEBKGND:
        *result = 1;
        return true;
    case WM_PAINT: {
        PAINTSTRUCT paint;
        BeginPaint(view->outer, &paint);
        EndPaint(view->outer, &paint);
        *result = 0;
        return true;
    }
    case WM_SIZE:
        gkCanvasReportSurface(view, 0);
        return false;
    case WM_SHOWWINDOW:
        gkLater([view]() {
            if (gkView(view)) {
                gkCanvasShown(view);
            }
        });
        return false;
    case WM_DPICHANGED_AFTERPARENT:
        gkCanvasReportSurface(view, 1);
        return false;
    case WM_GETDLGCODE:
        *result = DLGC_WANTALLKEYS | DLGC_WANTCHARS | DLGC_WANTARROWS | DLGC_WANTTAB;
        return true;
    case WM_LBUTTONDOWN:
    case WM_RBUTTONDOWN:
    case WM_MBUTTONDOWN:
    case WM_XBUTTONDOWN:
    case WM_LBUTTONDBLCLK:
    case WM_RBUTTONDBLCLK:
    case WM_MBUTTONDBLCLK:
    case WM_XBUTTONDBLCLK: {
        SetFocus(view->outer);
        SetCapture(view->outer);
        int button = message == WM_LBUTTONDOWN || message == WM_LBUTTONDBLCLK   ? GK_POINTER_BUTTON_LEFT
                     : message == WM_RBUTTONDOWN || message == WM_RBUTTONDBLCLK ? GK_POINTER_BUTTON_RIGHT
                     : message == WM_MBUTTONDOWN || message == WM_MBUTTONDBLCLK ? GK_POINTER_BUTTON_MIDDLE
                     : GET_XBUTTON_WPARAM(wparam) == XBUTTON1                   ? 3
                                                                                : 4;
        gkCanvasButton(view, button, GK_POINTER_PRESSED, gkModifiers());
        *result = message == WM_XBUTTONDOWN || message == WM_XBUTTONDBLCLK ? TRUE : 0;
        return true;
    }
    case WM_LBUTTONUP:
    case WM_RBUTTONUP:
    case WM_MBUTTONUP:
    case WM_XBUTTONUP: {
        int button = message == WM_LBUTTONUP   ? GK_POINTER_BUTTON_LEFT
                     : message == WM_RBUTTONUP ? GK_POINTER_BUTTON_RIGHT
                     : message == WM_MBUTTONUP ? GK_POINTER_BUTTON_MIDDLE
                     : GET_XBUTTON_WPARAM(wparam) == XBUTTON1 ? 3
                                                              : 4;
        gkCanvasButton(view, button, GK_POINTER_RELEASED, gkModifiers());
        if (!view->canvasButtons && GetCapture() == view->outer && !gkWin32CanvasHasCapture(view)) {
            ReleaseCapture();
        }
        *result = message == WM_XBUTTONUP ? TRUE : 0;
        return true;
    }
    case WM_CAPTURECHANGED:
        if ((HWND)lparam != view->outer) {
            for (int button = 0; button < GK_POINTER_BUTTON_COUNT; button++) {
                if ((view->canvasButtons & (1 << button)) && view->action) {
                    gkWin32UIGoCanvasPointerButton(view->action, button, GK_POINTER_RELEASED, 0);
                }
            }
            view->canvasButtons = 0;
        }
        return false;
    case WM_MOUSEMOVE: {
        double scale = gkViewScaleOf(view);
        double x = GET_X_LPARAM(lparam) / scale, y = GET_Y_LPARAM(lparam) / scale;
        if (!inside) {
            inside = true;
            TRACKMOUSEEVENT track = {sizeof(track), TME_LEAVE, view->outer, 0};
            TrackMouseEvent(&track);
            gkCanvasSend(view, GK_EVENT_POINTER_ENTER, 0, 0, 0, x, y, 0, 0);
        }
        double dx = haveLast ? x - lastX : 0, dy = haveLast ? y - lastY : 0;
        lastX = x;
        lastY = y;
        haveLast = true;
        gkCanvasSend(view, GK_EVENT_POINTER_MOVE, gkModifiers(), 0, 0, x, y, dx, dy);
        return false;
    }
    case WM_MOUSELEAVE:
        inside = false;
        haveLast = false;
        gkCanvasSend(view, GK_EVENT_POINTER_LEAVE, 0, 0, 0, lastX, lastY, 0, 0);
        return false;
    case WM_MOUSEWHEEL:
    case WM_MOUSEHWHEEL: {
        double delta = (double)GET_WHEEL_DELTA_WPARAM(wparam) / WHEEL_DELTA;
        if (view->action) {
            if (message == WM_MOUSEHWHEEL) {
                gkWin32UIGoCanvasScroll(view->action, -delta, 0);
            } else {
                gkWin32UIGoCanvasScroll(view->action, 0, delta);
            }
        }
        *result = 0;
        return true;
    }
    case WM_KEYDOWN:
    case WM_SYSKEYDOWN:
    case WM_KEYUP:
    case WM_SYSKEYUP: {
        int key = gkKeyFromVirtualKey(wparam, lparam);
        if (key >= 0) {
            int action = message == WM_KEYUP || message == WM_SYSKEYUP ? GK_KEY_RELEASED
                         : (lparam & (1l << 30))                      ? GK_KEY_REPEAT
                                                                      : GK_KEY_PRESSED;
            gkCanvasSend(view, GK_EVENT_KEY, key, (int)((lparam >> 16) & 0xff), action, gkModifiers(), 0, 0, 0);
        }
        return message == WM_KEYDOWN || message == WM_KEYUP;
    }
    case WM_CHAR: {
        unsigned int unit = (unsigned int)wparam;
        if (unit >= 0xd800 && unit <= 0xdbff) {
            highSurrogate = unit;
            return true;
        }
        unsigned int c = unit;
        if (unit >= 0xdc00 && unit <= 0xdfff && highSurrogate) {
            c = 0x10000 + ((highSurrogate - 0xd800) << 10) + (unit - 0xdc00);
        }
        highSurrogate = 0;
        if (c >= 0x20 && c != 0x7f && !(gkModifiers() & (GK_MOD_CONTROL | GK_MOD_ALT | GK_MOD_SUPER))) {
            gkCanvasSend(view, GK_EVENT_TEXT, (int)c, 0, 0, 0, 0, 0, 0);
        }
        *result = 0;
        return true;
    }
    default:
        return false;
    }
}

static void gkCanvasFrames(GKView *view) {
    GKCanvasData *data = gkCanvasData(view);
    if (!data) {
        return;
    }
    bool wanted = (view->frames & 1) != 0 && view->action;
    if (wanted && !data->frames) {
        data->frames = gkTimer(1, [view]() {
            if (!gkView(view) || !(view->frames & 1)) {
                return false;
            }
            double period = 1.0 / 60;
            HMONITOR monitor = MonitorFromWindow(view->outer, MONITOR_DEFAULTTONEAREST);
            MONITORINFOEXW info = {};
            info.cbSize = sizeof(info);
            DEVMODEW mode = {};
            mode.dmSize = sizeof(mode);
            if (GetMonitorInfoW(monitor, &info) && EnumDisplaySettingsW(info.szDevice, ENUM_CURRENT_SETTINGS, &mode) &&
                mode.dmDisplayFrequency > 1) {
                period = 1.0 / mode.dmDisplayFrequency;
            }
            gkCanvasSend(view, GK_EVENT_FRAME, 0, 0, 0, 0, period, 0, 0);
            return true;
        });
    } else if (!wanted && data->frames) {
        gkTimerCancel(data->frames);
        data->frames = 0;
    }
}

extern "C" void *gkWin32UICreateCanvas(uintptr_t window, uintptr_t handle) {
    return gkCreateDrawn(window, GK_KIND_CANVAS, new GKCanvasData(), handle, 1);
}

extern "C" void gkWin32UICanvasPointerPosition(void *canvas, double *x, double *y) {
    GKView *view = gkView(canvas);
    double px = 0, py = 0;
    if (view && view->outer) {
        POINT cursor;
        if (GetCursorPos(&cursor)) {
            ScreenToClient(view->outer, &cursor);
            double scale = gkViewScaleOf(view);
            px = cursor.x / scale;
            py = cursor.y / scale;
        }
    }
    *x = px;
    *y = py;
}

extern "C" int gkWin32CanvasStartFrames(void *pointer, int start) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_CANVAS) {
        return 0;
    }
    if (start) {
        view->frames |= 1;
    } else {
        view->frames &= ~1;
    }
    gkCanvasFrames(view);
    return 1;
}

extern "C" void gkWin32CanvasAttach(void *pointer, uintptr_t handle) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_CANVAS) {
        view->action = handle;
        gkCanvasFrames(view);
        gkCanvasShown(view);
    }
}

/* ---- Sections ---- */

static const double gkSectionHeaderHeight = 28;

/* GKSectionHeader draws the header of a section, which Windows has no
   control for: a chevron, an icon, a title, a subtitle, and a badge. */
struct GKSection : GKExtension {
    HWND header = nullptr;
    GKView *headerView = nullptr;
    std::wstring subtitle;
    std::wstring badge;
    GKIcon *icon = nullptr;
    int expanded = 0;
    int headerHover = 0;
    ~GKSection() override {
        if (icon) {
            gkReleaseObject(icon);
        }
    }
    void layout(GKView *view) override {
        gkPlaceWindow(header, 0, 0, view->width, gkSectionHeaderHeight);
        double height = view->height - gkSectionHeaderHeight;
        gkPlaceWindow(view->body, 0, gkSectionHeaderHeight, view->width, height > 0 ? height : 0);
        ShowWindow(view->body, expanded ? SW_SHOWNA : SW_HIDE);
    }
};

/* GKSectionHeaderPainter is the extension of the header window. */
struct GKSectionHeader : GKExtension {
    GKView *section = nullptr;
    void paint(GKView *view, GKCanvas &canvas) override {
        GKSection *data = static_cast<GKSection *>(section->ext.get());
        GKPalette const &palette = gkViewPalette(section);
        if (view->hover || view->pressed) {
            canvas.fillRound(0, 0, canvas.width, canvas.height, 4,
                             view->pressed ? palette.subtlePressed : palette.subtle);
        }
        float x = 6;
        canvas.glyph(data->expanded ? L"\xE70D" : L"\xE76C", 10, x, 0, 12, canvas.height, palette.text);
        x += 18;
        if (data->icon) {
            gkDrawIcon(canvas, data->icon, x, (canvas.height - 16) / 2, 16, palette.text);
            x += 22;
        }
        GKFont font = gkViewFont(section, gkTypeRamp(2));
        D2D1_SIZE_F title = gkMeasureText(section->text, font);
        canvas.text(section->text, font, x, 0, canvas.width - x, canvas.height,
                    gkViewTextColor(section, section->enabled ? palette.text : palette.textDisabled));
        x += title.width + 8;
        if (!data->subtitle.empty()) {
            GKFont caption = gkTypeRamp(1);
            D2D1_SIZE_F size = gkMeasureText(data->subtitle, caption);
            canvas.text(data->subtitle, caption, x, 0, canvas.width - x, canvas.height, palette.textSecondary);
            x += size.width + 8;
        }
        if (!data->badge.empty()) {
            GKFont caption = gkTypeRamp(1);
            D2D1_SIZE_F size = gkMeasureText(data->badge, caption);
            float right = canvas.width - 8 - size.width;
            canvas.text(data->badge, caption, right > x ? right : x, 0, size.width + 2, canvas.height,
                        palette.textSecondary);
        }
        if (view->focused && view->keyboardFocus) {
            canvas.strokeRound(1, 1, canvas.width - 2, canvas.height - 2, 4, palette.focus, 1.5f);
        }
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override;
};

void gkSectionToggle(GKView *view, int expanded, bool report) {
    GKSection *data = static_cast<GKSection *>(view->ext.get());
    data->expanded = expanded != 0;
    data->layout(view);
    InvalidateRect(data->header, nullptr, FALSE);
    if (report) {
        gkViewAction(view, GK_UI_EVENT_SECTION_CHANGE);
    }
}

bool GKSectionHeader::message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam,
                              LRESULT *result) {
    (void)hwnd;
    (void)lparam;
    switch (message) {
    case WM_LBUTTONDOWN:
        view->pressed = 1;
        SetCapture(view->outer);
        InvalidateRect(view->outer, nullptr, FALSE);
        return false;
    case WM_LBUTTONUP:
        if (view->pressed) {
            view->pressed = 0;
            ReleaseCapture();
            InvalidateRect(view->outer, nullptr, FALSE);
            if (view->hover && section->enabled) {
                GKSection *data = static_cast<GKSection *>(section->ext.get());
                gkSectionToggle(section, !data->expanded, true);
            }
        }
        *result = 0;
        return true;
    case WM_KEYDOWN:
        if (wparam == VK_SPACE || wparam == VK_RETURN) {
            GKSection *data = static_cast<GKSection *>(section->ext.get());
            gkSectionToggle(section, !data->expanded, true);
            *result = 0;
            return true;
        }
        if (wparam == VK_LEFT || wparam == VK_RIGHT) {
            GKSection *data = static_cast<GKSection *>(section->ext.get());
            int expand = wparam == VK_RIGHT;
            if (expand != data->expanded) {
                gkSectionToggle(section, expand, true);
            }
            *result = 0;
            return true;
        }
        return false;
    default:
        return false;
    }
}

extern "C" void *gkWin32UICreateSection(uintptr_t window, const char *title, int expanded, uintptr_t handle) {
    GKSection *data = new GKSection();
    data->expanded = expanded != 0;
    GKView *view = gkHostView(window, GK_KIND_SECTION, data, handle);
    view->text = gkWide(title);
    GKSectionHeader *painter = new GKSectionHeader();
    painter->section = view;
    GKView *header = gkCreateDrawn(window, GK_KIND_GENERIC, painter, 0, 1);
    gkReparent(header->outer, view->outer);
    /* The header belongs to the section, which reports its focus and its
       accessibility. */
    header->owner = view;
    data->header = header->outer;
    data->headerView = header;
    view->control = header->outer;
    view->body = gkLayoutNew(view->outer);
    data->layout(view);
    return view;
}

/* gkSectionHeaderView returns the drawn view of a section's header. */
static GKView *gkSectionHeaderView(GKView *view) {
    GKSection *data = view && view->kind == GK_KIND_SECTION ? static_cast<GKSection *>(view->ext.get()) : nullptr;
    return data ? data->headerView : nullptr;
}

extern "C" int gkWin32UISectionExpanded(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_SECTION && static_cast<GKSection *>(view->ext.get())->expanded;
}

extern "C" void gkWin32SectionSetAnimated(void *control, int animated) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_SECTION) {
        view->animated = animated != 0;
    }
}

extern "C" void gkWin32SectionConfigure(void *control, const char *subtitle, void *icon, const char *badge) {
    GKView *view = gkView(control);
    if (!view || view->kind != GK_KIND_SECTION) {
        return;
    }
    GKSection *data = static_cast<GKSection *>(view->ext.get());
    data->subtitle = gkWide(subtitle);
    data->badge = gkWide(badge);
    if (data->icon) {
        gkReleaseObject(data->icon);
    }
    data->icon = static_cast<GKIcon *>(icon);
    if (data->icon) {
        gkRetainObject(data->icon);
    }
    InvalidateRect(data->header, nullptr, FALSE);
}

extern "C" void gkWin32UISectionSetExpanded(void *control, int expanded) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_SECTION) {
        gkSectionToggle(view, expanded, false);
    }
}

/* ---- Generic view functions ---- */

extern "C" void gkWin32UIDestroy(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_CANVAS) {
        gkWin32ReleaseCaptureFor(control);
    }
    if (view->kind == GK_KIND_SECTION) {
        if (GKView *header = gkSectionHeaderView(view)) {
            static_cast<GKSection *>(view->ext.get())->headerView = nullptr;
            gkReleaseObject(header);
        }
    }
    gkReleaseObject(view);
}

extern "C" void gkWin32UIClearAction(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->action = 0;
    if (view->kind == GK_KIND_CANVAS) {
        view->frames = 0;
        gkCanvasFrames(view);
        gkWin32ReleaseCaptureFor(control);
    }
}

extern "C" void gkWin32UIAttachToWindow(uintptr_t window, void *control) {
    GKView *view = gkView(control);
    GKWin *win = gkWinFromHandle(window);
    if (view && win && win->root) {
        gkReparent(view->outer, win->root);
        if (view->visible) {
            ShowWindow(view->outer, SW_SHOWNA);
        }
        if (view->kind == GK_KIND_CANVAS) {
            gkCanvasShown(view);
        }
    }
}

extern "C" void gkWin32UISetParent(void *control, void *parent) {
    GKView *view = gkView(control);
    GKView *container = gkView(parent);
    if (!view || !container) {
        return;
    }
    HWND target = gkContainerOf(container);
    if (target) {
        gkReparent(view->outer, target);
        if (view->visible) {
            ShowWindow(view->outer, SW_SHOWNA);
        }
        if (view->kind == GK_KIND_CANVAS) {
            gkCanvasShown(view);
        }
    }
}

extern "C" void gkWin32UIBringToFront(void *control) {
    GKView *view = gkView(control);
    if (view && view->outer) {
        SetWindowPos(view->outer, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    }
}

extern "C" void gkWin32UISetFrame(void *control, int x, int y, int width, int height) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    width = width > 0 ? width : 0;
    height = height > 0 ? height : 0;
    /* The layout code sets the frames of every view on each pass; moving a
       window to where it already is costs a message round. */
    if (view->placed && view->x == x && view->y == y && view->width == width && view->height == height) {
        return;
    }
    view->placed = 1;
    view->x = x;
    view->y = y;
    view->width = width;
    view->height = height;
    gkPlace(view);
    if (view->ext) {
        view->ext->layout(view);
    }
}

extern "C" void gkWin32UISetVisible(void *control, int visible) {
    GKView *view = gkView(control);
    if (view) {
        view->visible = visible != 0;
        ShowWindow(view->outer, visible ? SW_SHOWNA : SW_HIDE);
        if (view->kind == GK_KIND_CANVAS) {
            gkCanvasShown(view);
        }
    }
}

static BOOL CALLBACK gkEnableChild(HWND child, LPARAM enabled) {
    if (!gkViewOf(child) || gkViewOf(child) == gkViewOf(GetParent(child))) {
        EnableWindow(child, (BOOL)enabled);
        InvalidateRect(child, nullptr, TRUE);
    }
    return TRUE;
}

extern "C" void gkWin32UISetEnabled(void *control, int enabled) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->enabled = enabled != 0;
    EnableWindow(view->outer, enabled ? TRUE : FALSE);
    if (view->control) {
        EnableWindow(view->control, enabled ? TRUE : FALSE);
    }
    if (view->kind != GK_KIND_LAYOUT && view->kind != GK_KIND_SCROLL) {
        EnumChildWindows(view->outer, gkEnableChild, enabled ? TRUE : FALSE);
    }
    gkInvalidate(view);
}

extern "C" void gkWin32UIPreferredSize(void *control, int *width, int *height) {
    GKView *view = gkView(control);
    int w = 0, h = 0;
    if (view) {
        switch (view->kind) {
        case GK_KIND_TABS:
        case GK_KIND_SCROLL:
            w = 320;
            h = 240;
            break;
        case GK_KIND_LAYOUT:
        case GK_KIND_CANVAS:
            break;
        default:
            if (view->ext) {
                D2D1_SIZE_F size = view->ext->preferred(view);
                if (size.width >= 0) {
                    w = (int)ceilf(size.width);
                    h = (int)ceilf(size.height);
                }
            }
            break;
        }
    }
    *width = w;
    *height = h;
}

extern "C" void gkWin32UIFramebufferSize(void *control, int *width, int *height) {
    GKView *view = gkView(control);
    RECT client = {};
    if (view && view->outer) {
        GetClientRect(view->outer, &client);
    }
    *width = client.right;
    *height = client.bottom;
}

/* ---- Text ---- */

void gkLabelSetText(GKView *view, std::wstring const &text);

extern "C" char *gkWin32UIText(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return gkStrdup("");
    }
    switch (view->kind) {
    case GK_KIND_LABEL:
    case GK_KIND_BUTTON:
    case GK_KIND_LINK:
    case GK_KIND_TOGGLE:
    case GK_KIND_CHECK_BOX:
    case GK_KIND_SECTION:
        return gkStrdupW(view->text);
    default:
        break;
    }
    HWND target = gkViewTarget(view);
    if (GKField *field = gkFieldOf(view)) {
        target = field->edit;
    } else if (HWND combo = gkSelectCombo(view)) {
        target = combo;
    }
    std::wstring text = gkWindowText(target);
    std::wstring result;
    for (wchar_t c : text) {
        if (c != L'\r') {
            result += c;
        }
    }
    return gkStrdupW(result);
}

extern "C" void gkWin32UISetText(void *control, const char *text) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    std::wstring value = gkWide(text);
    view->suppress++;
    switch (view->kind) {
    case GK_KIND_LABEL:
        gkLabelSetText(view, value);
        break;
    case GK_KIND_BUTTON:
    case GK_KIND_TOGGLE:
        gkButtonSetText(view, value);
        break;
    case GK_KIND_CHECK_BOX:
        gkCheckBoxSetText(view, value);
        break;
    case GK_KIND_LINK:
        view->text = value;
        SetWindowTextW(view->outer, gkLinkMarkup(value).c_str());
        break;
    case GK_KIND_SECTION:
        view->text = value;
        gkInvalidate(view);
        break;
    default: {
        HWND target = gkViewTarget(view);
        if (GKField *field = gkFieldOf(view)) {
            target = field->edit;
        } else if (HWND combo = gkSelectCombo(view)) {
            target = combo;
        }
        std::wstring converted;
        for (wchar_t c : value) {
            if (c == L'\n') {
                converted += L"\r\n";
            } else if (c != L'\r') {
                converted += c;
            }
        }
        SetWindowTextW(target, converted.c_str());
        break;
    }
    }
    view->suppress--;
}


extern "C" void gkWin32UISetPlaceholder(void *control, const char *text) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->placeholder = gkWide(text);
    if (GKField *field = gkFieldOf(view)) {
        if (!field->multiline) {
            SendMessageW(field->edit, EM_SETCUEBANNER, TRUE, (LPARAM)view->placeholder.c_str());
        }
        InvalidateRect(field->edit, nullptr, TRUE);
    } else if (HWND combo = gkSelectCombo(view)) {
        SendMessageW(combo, CB_SETCUEBANNER, 0, (LPARAM)view->placeholder.c_str());
    }
}

extern "C" char *gkWin32UIPlaceholder(void *control) {
    GKView *view = gkView(control);
    return gkStrdupW(view ? view->placeholder : std::wstring());
}

/* ---- Values ---- */

extern "C" int gkWin32UIChecked(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return 0;
    }
    if (view->kind == GK_KIND_CHECK_BOX || view->kind == GK_KIND_TOGGLE) {
        return SendMessageW(gkViewTarget(view), BM_GETCHECK, 0, 0) == BST_CHECKED;
    }
    return view->checked;
}

extern "C" void gkWin32UISetChecked(void *control, int checked) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->suppress++;
    view->checked = checked != 0;
    if (view->kind == GK_KIND_CHECK_BOX || view->kind == GK_KIND_TOGGLE) {
        SendMessageW(gkViewTarget(view), BM_SETCHECK, checked ? BST_CHECKED : BST_UNCHECKED, 0);
    } else {
        gkInvalidate(view);
    }
    view->suppress--;
}

extern "C" int gkWin32UIReadOnly(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_TEXT_AREA && view->readOnly;
}

extern "C" void gkWin32UISetReadOnly(void *control, int readOnly) {
    GKView *view = gkView(control);
    GKField *field = view && view->kind == GK_KIND_TEXT_AREA ? gkFieldOf(view) : nullptr;
    if (field) {
        view->readOnly = readOnly != 0;
        SendMessageW(field->edit, EM_SETREADONLY, readOnly ? TRUE : FALSE, 0);
        InvalidateRect(view->outer, nullptr, FALSE);
    }
}

/* ---- File dialogs ---- */

static HWND gkDialogOwner(uintptr_t window) {
    GKWin *win = gkWinFromHandle(window);
    return win && win->hwnd && IsWindowVisible(win->hwnd) ? win->hwnd : nullptr;
}

static void gkDialogFilters(IFileDialog *dialog, const char *extensions) {
    if (!extensions || !*extensions) {
        return;
    }
    std::wstring pattern, name;
    std::string list = extensions;
    size_t start = 0;
    while (start <= list.size()) {
        size_t end = list.find(',', start);
        std::string part = list.substr(start, end == std::string::npos ? std::string::npos : end - start);
        while (!part.empty() && (part.front() == ' ' || part.front() == '.')) {
            part.erase(part.begin());
        }
        while (!part.empty() && part.back() == ' ') {
            part.pop_back();
        }
        if (!part.empty()) {
            if (!pattern.empty()) {
                pattern += L";";
                name += L", ";
            }
            pattern += L"*." + gkWide(part.c_str());
            name += L"*." + gkWide(part.c_str());
        }
        if (end == std::string::npos) {
            break;
        }
        start = end + 1;
    }
    if (pattern.empty()) {
        return;
    }
    COMDLG_FILTERSPEC spec = {name.c_str(), pattern.c_str()};
    dialog->SetFileTypes(1, &spec);
    dialog->SetFileTypeIndex(1);
}

static void gkDialogFolder(IFileDialog *dialog, const char *directory) {
    if (!directory || !*directory) {
        return;
    }
    IShellItem *folder = nullptr;
    if (SUCCEEDED(SHCreateItemFromParsingName(gkWide(directory).c_str(), nullptr, IID_PPV_ARGS(&folder)))) {
        dialog->SetFolder(folder);
        folder->Release();
    }
}

static std::string gkItemPath(IShellItem *item) {
    PWSTR path = nullptr;
    std::string result;
    if (SUCCEEDED(item->GetDisplayName(SIGDN_FILESYSPATH, &path)) && path) {
        result = gkUTF8(path);
        CoTaskMemFree(path);
    }
    return result;
}

extern "C" char *gkWin32UIOpenPanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                                    const char *extensions, int multiple, int directories, int hidden, int packages,
                                    size_t *resultSize, int *cancelled) {
    (void)packages;
    GKActivation activation;
    IFileOpenDialog *dialog = nullptr;
    *cancelled = 1;
    *resultSize = 0;
    if (FAILED(CoCreateInstance(CLSID_FileOpenDialog, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&dialog)))) {
        return nullptr;
    }
    FILEOPENDIALOGOPTIONS options = 0;
    dialog->GetOptions(&options);
    options |= FOS_FORCEFILESYSTEM | FOS_PATHMUSTEXIST;
    if (multiple) {
        options |= FOS_ALLOWMULTISELECT;
    }
    if (directories) {
        options |= FOS_PICKFOLDERS;
    }
    if (hidden) {
        options |= FOS_FORCESHOWHIDDEN;
    }
    dialog->SetOptions(options);
    if (title && *title) {
        dialog->SetTitle(gkWide(title).c_str());
    }
    if (filename && *filename) {
        dialog->SetFileName(gkWide(filename).c_str());
    }
    if (!directories) {
        gkDialogFilters(dialog, extensions);
    }
    gkDialogFolder(dialog, directory);
    char *result = nullptr;
    if (SUCCEEDED(dialog->Show(gkDialogOwner(window)))) {
        IShellItemArray *items = nullptr;
        if (SUCCEEDED(dialog->GetResults(&items)) && items) {
            DWORD count = 0;
            items->GetCount(&count);
            std::vector<std::string> paths;
            for (DWORD i = 0; i < count; i++) {
                IShellItem *item = nullptr;
                if (SUCCEEDED(items->GetItemAt(i, &item)) && item) {
                    std::string path = gkItemPath(item);
                    if (!path.empty()) {
                        paths.push_back(path);
                    }
                    item->Release();
                }
            }
            items->Release();
            *cancelled = 0;
            result = gkJoin(paths, resultSize);
        }
    }
    dialog->Release();
    return result;
}

extern "C" char *gkWin32UISavePanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                                    const char *extensions, int hidden, const char *defaultExtension, size_t *resultSize,
                                    int *cancelled) {
    GKActivation activation;
    IFileSaveDialog *dialog = nullptr;
    *cancelled = 1;
    *resultSize = 0;
    if (FAILED(CoCreateInstance(CLSID_FileSaveDialog, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&dialog)))) {
        return nullptr;
    }
    FILEOPENDIALOGOPTIONS options = 0;
    dialog->GetOptions(&options);
    options |= FOS_FORCEFILESYSTEM | FOS_OVERWRITEPROMPT;
    if (hidden) {
        options |= FOS_FORCESHOWHIDDEN;
    }
    dialog->SetOptions(options);
    if (title && *title) {
        dialog->SetTitle(gkWide(title).c_str());
    }
    if (filename && *filename) {
        dialog->SetFileName(gkWide(filename).c_str());
    }
    if (defaultExtension && *defaultExtension) {
        dialog->SetDefaultExtension(gkWide(defaultExtension[0] == '.' ? defaultExtension + 1 : defaultExtension).c_str());
    }
    gkDialogFilters(dialog, extensions);
    gkDialogFolder(dialog, directory);
    char *result = nullptr;
    if (SUCCEEDED(dialog->Show(gkDialogOwner(window)))) {
        IShellItem *item = nullptr;
        if (SUCCEEDED(dialog->GetResult(&item)) && item) {
            std::vector<std::string> paths;
            std::string path = gkItemPath(item);
            if (!path.empty()) {
                paths.push_back(path);
            }
            item->Release();
            *cancelled = 0;
            result = gkJoin(paths, resultSize);
        }
    }
    dialog->Release();
    return result;
}
