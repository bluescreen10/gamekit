//go:build windows && cgo

#include "win32_internal.h"

#include <shellapi.h>

#include <algorithm>

void gkRadioGroupSetChoices(GKView *view, std::vector<std::string> const &titles, const int *enabled);
void gkRadioGroupConfigure(GKView *view, int horizontal, const char *label);
void gkRadioGroupPreferred(GKView *view, int *width, int *height);
HWND gkSelectCombo(GKView *view);
void gkSelectConfigure(GKView *view, const char *placeholder, int popupWidth);
void gkSelectSetVisibleRows(GKView *view, int rows);
void gkSliderConfigure(GKView *view, int vertical, double minimum, double maximum, double frequency, int snap);
void gkSliderSetInverted(GKView *view, int inverted);
void gkActivitySetSize(GKView *view, int size);
void gkSeparatorConfigure(GKView *view, int vertical, const char *label, int inset, int thickness);
void gkScrollGetOffset(GKView *view, double *x, double *y);
void gkScrollSetOffset(GKView *view, double x, double y);
void gkScrollViewport(GKView *view, double *width, double *height);
void gkScrollSetZoom(GKView *view, double zoom);
double gkScrollGetZoom(GKView *view);
void gkScrollSetPolicy(GKView *view, int policy);
void gkScrollSetInsets(GKView *view, int top, int right, int bottom, int left);
std::wstring gkLinkMarkup(std::wstring const &text);
GKView *gkActivityCreateIn(HWND parent);

/* ---- Labels ---- */

struct GKLabelState {
    int alignment = 0;
    int wrap = 0;
    int truncation = 3;
    int selectable = 0;
    int style = 0;
    int maximumLines = 0;
};

static std::unordered_map<GKView *, GKLabelState> &gkLabelStates() {
    static std::unordered_map<GKView *, GKLabelState> states;
    return states;
}

static GKFont gkLabelFont(GKView *view, GKLabelState const &state) {
    GKFont font = gkTypeRamp(state.style == 4 ? 0 : state.style);
    if (state.style == 4) {
        font.family = gkFontFamilyInstalled("Cascadia Mono") ? L"Cascadia Mono" : L"Consolas";
    }
    return gkViewFont(view, font);
}

/* gkLabelShown returns the text that a label shows: its text shortened at
   the start or in the middle, which a static control cannot do itself, so
   that it fits the width of the label. */
static std::wstring gkLabelShown(GKView *view, GKLabelState const &state) {
    std::wstring const &text = view->text;
    if ((state.truncation != 1 && state.truncation != 2) || view->width <= 0) {
        return text;
    }
    GKFont font = gkLabelFont(view, state);
    /* Wrapped text fits in its lines: the maximum number of lines, or the
       lines that the height of the label holds. */
    int line = gkMeasureGDI(L"Ag", font).cy;
    int lines = state.maximumLines > 0 ? state.maximumLines : std::max(1, view->height / std::max(1, line));
    if (state.wrap && view->height > 0) {
        lines = std::min(lines, std::max(1, view->height / std::max(1, line)));
    }
    auto fits = [&](std::wstring const &value) {
        if (!state.wrap) {
            return gkMeasureGDI(value, font).cx <= view->width;
        }
        return gkMeasureGDI(value, font, view->width).cy <= line * lines;
    };
    if (fits(text)) {
        return text;
    }
    size_t low = 0, high = text.size();
    std::wstring best = L"…";
    while (low <= high && high > 0) {
        size_t keep = (low + high) / 2;
        std::wstring candidate;
        if (state.truncation == 1) {
            candidate = L"…" + text.substr(text.size() - keep);
        } else {
            size_t head = (keep + 1) / 2;
            candidate = text.substr(0, head) + L"…" + text.substr(text.size() - (keep - head));
        }
        if (fits(candidate)) {
            best = candidate;
            low = keep + 1;
        } else {
            if (keep == 0) {
                break;
            }
            high = keep - 1;
        }
    }
    return best;
}

static void gkLabelApply(GKView *view) {
    GKLabelState &state = gkLabelStates()[view];
    std::wstring shown = gkLabelShown(view, state);
    if (state.selectable) {
        std::wstring converted;
        for (wchar_t c : shown) {
            if (c == L'\n') {
                converted += L"\r\n";
            } else if (c != L'\r') {
                converted += c;
            }
        }
        SetWindowTextW(view->outer, converted.c_str());
    } else {
        SetWindowTextW(view->outer, shown.c_str());
    }
}

void gkLabelSetText(GKView *view, std::wstring const &text) {
    view->text = text;
    gkLabelApply(view);
}

struct GKLabelLayout : GKExtension {
    D2D1_SIZE_F preferred(GKView *view) override {
        GKLabelState const &state = gkLabelStates()[view];
        GKFont font = gkLabelFont(view, state);
        int line = gkMeasureGDI(L"Ag", font).cy;
        if (state.wrap && view->preferredWidth > 0) {
            SIZE size = gkMeasureGDI(view->text, font, view->preferredWidth);
            if (state.maximumLines > 0 && size.cy > line * state.maximumLines) {
                size.cy = line * state.maximumLines;
            }
            return D2D1::SizeF((float)size.cx, (float)std::max<LONG>(size.cy, line));
        }
        if (state.wrap) {
            std::wstring text = view->text;
            int lines = 1 + (int)std::count(text.begin(), text.end(), L'\n');
            SIZE size = gkMeasureGDI(text, font, 100000);
            (void)lines;
            return D2D1::SizeF((float)size.cx, (float)std::max<LONG>(size.cy, line));
        }
        SIZE size = gkMeasureGDI(view->text, font);
        return D2D1::SizeF((float)size.cx, (float)line);
    }
    void layout(GKView *view) override {
        GKLabelState const &state = gkLabelStates()[view];
        if (state.truncation == 1 || state.truncation == 2) {
            gkLabelApply(view);
        }
    }
    void themeChanged(GKView *view) override {
        GKLabelState const &state = gkLabelStates()[view];
        view->font = gkFontHandle(gkLabelFont(view, state), GetDpiForWindow(view->outer));
        SendMessageW(view->outer, WM_SETFONT, (WPARAM)view->font, TRUE);
    }
};

GKExtension *gkLabelExtension() {
    return new GKLabelLayout();
}

extern "C" void gkWin32LabelConfigure(void *pointer, int alignment, int wrap, int truncation, int selectable,
                                      int style, int maximumLines) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_LABEL) {
        return;
    }
    GKLabelState &state = gkLabelStates()[view];
    int wasSelectable = state.selectable;
    state.alignment = alignment;
    state.wrap = wrap != 0;
    state.truncation = truncation;
    state.selectable = selectable != 0;
    state.style = style;
    state.maximumLines = maximumLines;
    if (!dynamic_cast<GKLabelLayout *>(view->ext.get())) {
        view->ext.reset(new GKLabelLayout());
    }
    if (state.selectable) {
        /* A static control cannot select its text, so a selectable label is
           a read-only edit control without a border. */
        DWORD style = ES_READONLY | ES_MULTILINE | WS_TABSTOP | (alignment == 1   ? ES_CENTER
                                                                 : alignment == 2 ? ES_RIGHT
                                                                                  : ES_LEFT);
        if (!state.wrap) {
            style |= ES_AUTOHSCROLL;
        }
        HWND edit = gkMakeNative(WC_EDITW, L"", style, 0);
        gkReplaceOuter(view, edit);
        gkSubclass(edit);
        gkApplyThemeTo(edit, gkViewTheme(view));
    } else {
        DWORD style = SS_NOPREFIX | SS_NOTIFY | (alignment == 1 ? SS_CENTER : alignment == 2 ? SS_RIGHT : SS_LEFT);
        if (!state.wrap) {
            style |= SS_CENTERIMAGE;
            if (truncation == 3) {
                style |= SS_ENDELLIPSIS;
            }
        } else if (truncation == 3) {
            style |= SS_EDITCONTROL | SS_ENDELLIPSIS;
        }
        if (wasSelectable) {
            HWND label = gkMakeNative(WC_STATICW, L"", style, 0);
            gkReplaceOuter(view, label);
            gkSubclass(label);
        } else {
            LONG_PTR current = GetWindowLongPtrW(view->outer, GWL_STYLE);
            current &= ~(SS_TYPEMASK | SS_CENTERIMAGE | SS_ELLIPSISMASK | SS_EDITCONTROL);
            SetWindowLongPtrW(view->outer, GWL_STYLE, current | style);
        }
    }
    view->ext->themeChanged(view);
    gkLabelApply(view);
    InvalidateRect(view->outer, nullptr, TRUE);
}

extern "C" void gkWin32LabelPreferredWidth(void *pointer, int width) {
    GKView *view = gkView(pointer);
    if (view) {
        view->preferredWidth = width > 0 ? width : 0;
    }
}

void gkForgetLabel(GKView *view) {
    gkLabelStates().erase(view);
}

/* ---- Accelerators ----

   A button can have a shortcut key. The window looks for a shortcut before
   it sends a key to the control with the focus. */

struct GKAccelerator {
    UINT key = 0;
    int modifiers = 0;
};

static std::unordered_map<GKView *, GKAccelerator> &gkAccelerators() {
    static std::unordered_map<GKView *, GKAccelerator> accelerators;
    return accelerators;
}

/* gkVirtualKey parses the key name of a shortcut that the Go layer makes. */
UINT gkVirtualKey(const char *name) {
    if (!name || !*name) {
        return 0;
    }
    static const struct {
        const char *name;
        UINT key;
    } names[] = {
        {"Up", VK_UP},         {"Down", VK_DOWN},          {"Left", VK_LEFT},          {"Right", VK_RIGHT},
        {"Delete", VK_DELETE}, {"Home", VK_HOME},          {"End", VK_END},            {"PageUp", VK_PRIOR},
        {"PageDown", VK_NEXT}, {"Backspace", VK_BACK},     {"Return", VK_RETURN},      {"Enter", VK_RETURN},
        {"Escape", VK_ESCAPE}, {"Tab", VK_TAB},            {"Space", VK_SPACE},        {",", VK_OEM_COMMA},
        {".", VK_OEM_PERIOD},  {"-", VK_OEM_MINUS},        {"=", VK_OEM_PLUS},         {"+", VK_OEM_PLUS},
        {";", VK_OEM_1},       {"/", VK_OEM_2},            {"`", VK_OEM_3},            {"[", VK_OEM_4},
        {"\\", VK_OEM_5},      {"]", VK_OEM_6},            {"'", VK_OEM_7},
    };
    for (auto const &entry : names) {
        if (strcmp(entry.name, name) == 0) {
            return entry.key;
        }
    }
    if (name[0] == 'F' && name[1] >= '1' && name[1] <= '9') {
        int number = atoi(name + 1);
        if (number >= 1 && number <= 24) {
            return VK_F1 + number - 1;
        }
    }
    if (!name[1] && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= '0' && name[0] <= '9'))) {
        return (UINT)name[0];
    }
    return 0;
}

/* gkAcceleratorModifiers returns the shortcut modifier bits that are down:
   Control 1, Alt 2, Shift 4. */
int gkAcceleratorModifiers() {
    int modifiers = 0;
    if (GetKeyState(VK_CONTROL) & 0x8000) {
        modifiers |= 1;
    }
    if (GetKeyState(VK_MENU) & 0x8000) {
        modifiers |= 2;
    }
    if (GetKeyState(VK_SHIFT) & 0x8000) {
        modifiers |= 4;
    }
    return modifiers;
}

/* gkAcceleratorPress clicks the button of a window whose shortcut is the
   key that the message presses. */
bool gkAcceleratorPress(GKWin *win, MSG const *message) {
    if (message->message != WM_KEYDOWN && message->message != WM_SYSKEYDOWN) {
        return false;
    }
    UINT key = (UINT)message->wParam;
    int modifiers = gkAcceleratorModifiers();
    for (auto const &entry : gkAccelerators()) {
        GKView *view = entry.first;
        if (entry.second.key != key || entry.second.modifiers != modifiers || !gkView(view)) {
            continue;
        }
        HWND target = gkViewTarget(view);
        if (gkWinOf(target) != win || !IsWindowVisible(target) || !IsWindowEnabled(target)) {
            continue;
        }
        if (key == VK_RETURN || key == VK_ESCAPE) {
            /* A control that handles Return or Escape itself keeps it. */
            LRESULT code = SendMessageW(message->hwnd, WM_GETDLGCODE, message->wParam, (LPARAM)message);
            wchar_t name[32] = {};
            GetClassNameW(message->hwnd, name, 32);
            bool multiline = !wcscmp(name, WC_EDITW) && (GetWindowLongPtrW(message->hwnd, GWL_STYLE) & ES_MULTILINE);
            if ((code & DLGC_WANTALLKEYS) || multiline || !wcscmp(name, WC_COMBOBOXW)) {
                continue;
            }
        }
        SendMessageW(target, BM_CLICK, 0, 0);
        return true;
    }
    return false;
}

void gkForgetAccelerator(GKView *view) {
    gkAccelerators().erase(view);
}

/* ---- Buttons ---- */

/* GKButtonParts holds what a button shows besides its text: an icon, a
   badge, and a spinner while it is busy. */
struct GKButtonParts {
    GKIcon *icon = nullptr;
    int placement = 0;
    HIMAGELIST images = nullptr;
    GKView *spinner = nullptr;
    GKView *badge = nullptr;
    std::wstring badgeText;
    int busy = 0;
    ~GKButtonParts() {
        if (icon) {
            gkReleaseObject(icon);
        }
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
    }
};

static std::unordered_map<GKView *, GKButtonParts> &gkButtonParts() {
    static std::unordered_map<GKView *, GKButtonParts> parts;
    return parts;
}

static void gkButtonApplyIcon(GKView *view) {
    auto found = gkButtonParts().find(view);
    if (found == gkButtonParts().end()) {
        return;
    }
    GKButtonParts &parts = found->second;
    HWND button = gkViewTarget(view);
    if (parts.images) {
        gkComctl().ImageList_Destroy(parts.images);
        parts.images = nullptr;
    }
    BUTTON_IMAGELIST list = {};
    list.himl = nullptr;
    if (parts.icon) {
        int size = (int)lround(16 * gkViewScaleOf(view));
        parts.images = gkComctl().ImageList_Create(size, size, ILC_COLOR32, 1, 0);
        HBITMAP bitmap = gkIconBitmap(parts.icon, size, gkViewPalette(view).text);
        gkComctl().ImageList_Add(parts.images, bitmap, nullptr);
        DeleteObject(bitmap);
        list.himl = parts.images;
        static const UINT alignments[] = {BUTTON_IMAGELIST_ALIGN_LEFT, BUTTON_IMAGELIST_ALIGN_RIGHT,
                                          BUTTON_IMAGELIST_ALIGN_TOP, BUTTON_IMAGELIST_ALIGN_BOTTOM,
                                          BUTTON_IMAGELIST_ALIGN_CENTER};
        list.uAlign = alignments[parts.placement >= 0 && parts.placement < 5 ? parts.placement : 0];
        /* The icon keeps the padding of the button from the border that it
           is next to, and a gap from the label. */
        int edge = (int)lround(8 * gkViewScaleOf(view)), gap = (int)lround(2 * gkViewScaleOf(view));
        switch (parts.placement) {
        case 0:
            list.margin = {edge, 0, gap, 0};
            break;
        case 1:
            list.margin = {gap, 0, edge, 0};
            break;
        case 2:
            list.margin = {0, edge / 2, 0, gap};
            break;
        case 3:
            list.margin = {0, gap, 0, edge / 2};
            break;
        default:
            list.margin = {0, 0, 0, 0};
            break;
        }
    }
    SendMessageW(button, BCM_SETIMAGELIST, 0, (LPARAM)&list);
    bool iconOnly = parts.icon && parts.placement == 4;
    SetWindowTextW(button, iconOnly || parts.busy ? L"" : view->text.c_str());
    InvalidateRect(button, nullptr, TRUE);
}

static float gkLuminance(GKColor const &color) {
    return 0.2126f * color.r + 0.7152f * color.g + 0.0722f * color.b;
}

/* gkShade moves a color toward white or black, away from its own
   lightness, by amount. */
static GKColor gkShade(GKColor color, float amount) {
    float target = gkLuminance(color) < 0.5f ? 1.0f : 0.0f;
    color.r += (target - color.r) * amount;
    color.g += (target - color.g) * amount;
    color.b += (target - color.b) * amount;
    return color;
}

bool gkButtonCustomDraw(GKView *view, NMHDR *header, LRESULT *result) {
    if (header->code != NM_CUSTOMDRAW || header->hwndFrom != view->outer ||
        (!view->style.hasBackground && view->style.cornerRadius <= 0)) {
        return false;
    }
    NMCUSTOMDRAW *draw = reinterpret_cast<NMCUSTOMDRAW *>(header);
    if (draw->dwDrawStage != CDDS_PREPAINT) {
        *result = CDRF_DODEFAULT;
        return true;
    }
    GKPalette const &palette = gkViewPalette(view);
    bool enabled = IsWindowEnabled(view->outer) != 0;
    bool pressed = (draw->uItemState & CDIS_SELECTED) != 0;
    bool hot = (draw->uItemState & CDIS_HOT) != 0;
    bool focused = (draw->uItemState & CDIS_FOCUS) != 0 &&
                   !(SendMessageW(view->outer, WM_QUERYUISTATE, 0, 0) & UISF_HIDEFOCUS);
    GKColor fill = view->style.hasBackground ? view->style.background : palette.control;
    if (pressed) {
        fill = gkShade(fill, 0.2f);
    } else if (hot) {
        fill = gkShade(fill, 0.1f);
    }
    GKColor ink = gkViewTextColor(view, palette.text);
    GKFont font = gkViewFont(view, gkBodyFont());
    GKIcon *icon = nullptr;
    auto found = gkButtonParts().find(view);
    bool busy = false;
    if (found != gkButtonParts().end()) {
        icon = found->second.icon;
        busy = found->second.busy != 0;
    }
    std::wstring text = view->text;
    gkPaintCanvas(view->outer, draw->hdc, draw->rc, [&](GKCanvas &canvas) {
        float radius = (float)view->style.cornerRadius;
        float opacity = enabled ? 1.0f : 0.5f;
        GKColor face = fill;
        face.a *= opacity;
        canvas.fillRound(0, 0, canvas.width, canvas.height, radius, face);
        if (!view->style.hasBackground) {
            canvas.strokeRound(0.5f, 0.5f, canvas.width - 1, canvas.height - 1, radius, palette.controlStroke);
        }
        if (focused) {
            canvas.strokeRound(1.5f, 1.5f, canvas.width - 3, canvas.height - 3, radius > 1.5f ? radius - 1.5f : 0,
                               palette.focus, 2);
        }
        if (busy) {
            return;
        }
        GKColor color = ink;
        color.a *= opacity;
        float iconSize = icon ? 16.0f : 0;
        float gap = icon && !text.empty() ? 6.0f : 0;
        float textWidth = text.empty() ? 0 : (float)gkMeasureGDI(text, font).cx + 2;
        float total = iconSize + gap + textWidth;
        float x = (canvas.width - total) / 2;
        if (icon) {
            gkDrawIcon(canvas, icon, x, (canvas.height - iconSize) / 2, iconSize, color);
        }
        if (!text.empty()) {
            canvas.text(text, font, x + iconSize + gap, 0, textWidth, canvas.height, color, 1, 1);
        }
    });
    *result = CDRF_SKIPDEFAULT;
    return true;
}

void gkButtonThemeChanged(GKView *view) {
    auto found = gkButtonParts().find(view);
    if (found == gkButtonParts().end()) {
        return;
    }
    /* The glyph of the icon takes the text color of the theme. */
    if (found->second.icon) {
        gkButtonApplyIcon(view);
    }
    if (found->second.badge) {
        InvalidateRect(found->second.badge->outer, nullptr, FALSE);
    }
}

int gkButtonIconPlacement(GKView *view) {
    auto found = gkButtonParts().find(view);
    if (found == gkButtonParts().end() || !found->second.icon) {
        return -1;
    }
    return found->second.placement;
}

/* gkButtonLayout places the spinner and the badge of a button. */
void gkButtonLayout(GKView *view) {
    auto found = gkButtonParts().find(view);
    if (found == gkButtonParts().end()) {
        return;
    }
    GKButtonParts &parts = found->second;
    if (parts.spinner) {
        double side = 16;
        gkPlaceWindow(parts.spinner->outer, (view->width - side) / 2, (view->height - side) / 2, side, side);
        ShowWindow(parts.spinner->outer, parts.busy ? SW_SHOWNA : SW_HIDE);
    }
    if (parts.badge) {
        /* The badge sits on the top-right corner of the button, over its
           edge, so it is a sibling above the button, which follows it. */
        HWND button = view->outer;
        HWND badge = parts.badge->outer;
        double scale = gkViewScaleOf(view);
        D2D1_SIZE_F text = gkMeasureText(parts.badgeText, gkTypeRamp(1));
        int height = (int)lround(16 * scale);
        int width = (int)lround(std::max<double>(16, text.width + 10) * scale);
        /* The badge goes in the nearest enclosing layout with room above
           the button, so that the layout does not clip it. */
        auto isLayout = [](HWND hwnd) {
            wchar_t name[32] = {};
            GetClassNameW(hwnd, name, 32);
            return wcscmp(name, L"GameKitLayout") == 0;
        };
        HWND parent = GetParent(button);
        RECT frame;
        for (;;) {
            GetWindowRect(button, &frame);
            MapWindowPoints(nullptr, parent, reinterpret_cast<POINT *>(&frame), 2);
            HWND above = GetParent(parent);
            if (frame.top - height / 2 >= 0 || !above || !isLayout(above) || !isLayout(parent) ||
                GetParent(above) == nullptr) {
                break;
            }
            parent = above;
        }
        if (GetParent(badge) != parent) {
            SetParent(badge, parent);
        }
        RECT client;
        GetClientRect(parent, &client);
        int x = std::min<int>(frame.right - width * 2 / 3, client.right - width);
        /* The top edge of the button runs through the middle of the badge. */
        int y = std::max<int>(frame.top - height / 2, 0);
        bool show = !parts.badgeText.empty() && IsWindowVisible(button);
        SetWindowPos(badge, HWND_TOP, x, y, width, height,
                     SWP_NOACTIVATE | (show ? SWP_SHOWWINDOW : SWP_HIDEWINDOW));
        SetWindowRgn(badge, CreateRoundRectRgn(0, 0, width + 1, height + 1, height, height), TRUE);
    }
}

/* GKBadgePainter draws the count badge on the corner of a button, which
   Windows has no control for. */
struct GKBadgePainter : GKExtension {
    GKView *button = nullptr;
    void paint(GKView *view, GKCanvas &canvas) override {
        (void)view;
        auto found = gkButtonParts().find(button);
        if (found == gkButtonParts().end()) {
            return;
        }
        GKPalette const &palette = gkViewPalette(button);
        (void)palette;
        /* Windows draws count badges in the same red in both themes. */
        canvas.fillRound(0, 0, canvas.width, canvas.height, canvas.height / 2, gkRGB(0xc42b1c));
        GKFont font = gkTypeRamp(1);
        font.weight = 700;
        canvas.centeredText(found->second.badgeText, font, 0, 0, canvas.width, canvas.height, gkRGB(0xffffff));
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)view;
        (void)hwnd;
        (void)wparam;
        (void)lparam;
        /* The pointer reaches the button under the badge. */
        if (message == WM_NCHITTEST) {
            *result = HTTRANSPARENT;
            return true;
        }
        return false;
    }
};

extern "C" void gkWin32ButtonConfigure(void *pointer, void *icon, int placement, int role, const char *key,
                                       int modifiers) {
    GKView *view = gkView(pointer);
    if (!view || (view->kind != GK_KIND_BUTTON && view->kind != GK_KIND_TOGGLE)) {
        return;
    }
    GKButtonParts &parts = gkButtonParts()[view];
    if (parts.icon) {
        gkReleaseObject(parts.icon);
    }
    parts.icon = static_cast<GKIcon *>(icon);
    if (parts.icon) {
        gkRetainObject(parts.icon);
    }
    parts.placement = placement;
    gkButtonApplyIcon(view);
    HWND button = gkViewTarget(view);
    if (view->kind == GK_KIND_BUTTON) {
        LONG_PTR style = GetWindowLongPtrW(button, GWL_STYLE) & ~(LONG_PTR)(BS_DEFPUSHBUTTON | BS_PUSHBUTTON);
        SetWindowLongPtrW(button, GWL_STYLE, style | (role == 1 ? BS_DEFPUSHBUTTON : BS_PUSHBUTTON));
        InvalidateRect(button, nullptr, TRUE);
    }
    GKAccelerator accelerator;
    if (key && *key) {
        accelerator.key = gkVirtualKey(key);
        accelerator.modifiers = modifiers;
    } else if (role == 1) {
        accelerator.key = VK_RETURN;
    } else if (role == 2) {
        accelerator.key = VK_ESCAPE;
    }
    if (accelerator.key) {
        gkAccelerators()[view] = accelerator;
    } else {
        gkAccelerators().erase(view);
    }
}

extern "C" void gkWin32ButtonSetBusy(void *pointer, int busy) {
    GKView *view = gkView(pointer);
    if (!view || (view->kind != GK_KIND_BUTTON && view->kind != GK_KIND_TOGGLE)) {
        return;
    }
    GKButtonParts &parts = gkButtonParts()[view];
    parts.busy = busy != 0;
    if (parts.busy && !parts.spinner) {
        parts.spinner = gkActivityCreateIn(gkViewTarget(view));
        parts.spinner->owner = view;
        /* The button does not draw over its spinner. */
        SetWindowLongPtrW(view->outer, GWL_STYLE, GetWindowLongPtrW(view->outer, GWL_STYLE) | WS_CLIPCHILDREN);
    }
    if (parts.spinner) {
        extern void gkWin32UISetActivityRunning(void *control, int running);
        gkWin32UISetActivityRunning(parts.spinner, parts.busy);
    }
    gkButtonApplyIcon(view);
    gkButtonLayout(view);
    view->accessibleValue = parts.busy ? "busy" : "";
}

int gkButtonBusy(GKView *view) {
    auto found = gkButtonParts().find(view);
    return found != gkButtonParts().end() && found->second.busy;
}

extern "C" void gkWin32ButtonSetBadge(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    if (!view || (view->kind != GK_KIND_BUTTON && view->kind != GK_KIND_TOGGLE)) {
        return;
    }
    GKButtonParts &parts = gkButtonParts()[view];
    parts.badgeText = gkWide(text);
    if (!parts.badge && !parts.badgeText.empty()) {
        GKBadgePainter *painter = new GKBadgePainter();
        painter->button = view;
        parts.badge = gkCreateDrawn(0, GK_KIND_GENERIC, painter, 0, 0);
        parts.badge->owner = view;
        /* The button does not draw over the badge, a sibling above it. */
        SetWindowLongPtrW(view->outer, GWL_STYLE, GetWindowLongPtrW(view->outer, GWL_STYLE) | WS_CLIPSIBLINGS);
    }
    if (parts.badge) {
        InvalidateRect(parts.badge->outer, nullptr, FALSE);
    }
    gkButtonLayout(view);
    view->accessibleDescription = gkUTF8(parts.badgeText);
}

std::wstring gkButtonBadge(GKView *view) {
    auto found = gkButtonParts().find(view);
    return found == gkButtonParts().end() ? std::wstring() : found->second.badgeText;
}

void gkButtonSetText(GKView *view, std::wstring const &text) {
    view->text = text;
    if (gkButtonParts().count(view)) {
        gkButtonApplyIcon(view);
    } else {
        SetWindowTextW(gkViewTarget(view), text.c_str());
    }
}

void gkForgetButton(GKView *view) {
    auto found = gkButtonParts().find(view);
    if (found == gkButtonParts().end()) {
        return;
    }
    if (found->second.spinner) {
        gkReleaseObject(found->second.spinner);
    }
    if (found->second.badge) {
        gkReleaseObject(found->second.badge);
    }
    gkButtonParts().erase(found);
}

/* ---- Text ---- */

/* gkTextControl returns the edit control of a view. */
HWND gkTextControl(GKView *view) {
    if (!view) {
        return nullptr;
    }
    if (GKField *field = gkFieldOf(view)) {
        return field->edit;
    }
    if (HWND combo = gkSelectCombo(view)) {
        COMBOBOXINFO info = {sizeof(info)};
        if (GetComboBoxInfo(combo, &info) && info.hwndItem) {
            return info.hwndItem;
        }
    }
    wchar_t name[32] = {};
    GetClassNameW(gkViewTarget(view), name, 32);
    if (!wcscmp(name, WC_EDITW) || !wcscmp(name, L"RICHEDIT50W")) {
        return gkViewTarget(view);
    }
    return nullptr;
}

static HWND gkAffixLabel(GKView *view, GKField *field, int suffix) {
    HWND &label = suffix ? field->suffix : field->prefix;
    if (!label) {
        label = gkMakeNative(WC_STATICW, L"", SS_NOPREFIX | SS_CENTERIMAGE | (suffix ? SS_RIGHT : SS_LEFT), 0,
                             view->outer);
        SendMessageW(label, WM_SETFONT, (WPARAM)view->font, FALSE);
    }
    return label;
}

extern "C" void gkWin32TextFieldConfigure(void *pointer, int readOnly, int maximumLength, int purpose, int alignment,
                                          const char *prefix, const char *suffix, int spellCheck) {
    (void)purpose;
    (void)spellCheck;
    GKView *view = gkView(pointer);
    GKField *field = gkFieldOf(view);
    if (!field) {
        return;
    }
    view->readOnly = readOnly != 0;
    SendMessageW(field->edit, EM_SETREADONLY, readOnly ? TRUE : FALSE, 0);
    SendMessageW(field->edit, EM_SETLIMITTEXT, maximumLength > 0 ? maximumLength : 0, 0);
    LONG_PTR style = GetWindowLongPtrW(field->edit, GWL_STYLE) & ~(LONG_PTR)(ES_LEFT | ES_CENTER | ES_RIGHT);
    style |= alignment == 1 ? ES_CENTER : alignment == 2 ? ES_RIGHT : ES_LEFT;
    SetWindowLongPtrW(field->edit, GWL_STYLE, style);
    std::wstring before = gkWide(prefix), after = gkWide(suffix);
    if (!before.empty() || field->prefix) {
        HWND label = gkAffixLabel(view, field, 0);
        SetWindowTextW(label, before.c_str());
        ShowWindow(label, before.empty() ? SW_HIDE : SW_SHOWNA);
    }
    if (!after.empty() || field->suffix) {
        HWND label = gkAffixLabel(view, field, 1);
        SetWindowTextW(label, after.c_str());
        ShowWindow(label, after.empty() ? SW_HIDE : SW_SHOWNA);
    }
    field->layout(view);
    InvalidateRect(view->outer, nullptr, TRUE);
}

extern "C" void gkWin32TextSelection(void *pointer, int *start, int *length) {
    *start = 0;
    *length = 0;
    HWND edit = gkTextControl(gkView(pointer));
    if (!edit) {
        return;
    }
    DWORD from = 0, to = 0;
    SendMessageW(edit, EM_GETSEL, (WPARAM)&from, (LPARAM)&to);
    /* An edit control counts a line break as two code units, the Go layer
       as one. */
    int textLength = GetWindowTextLengthW(edit);
    std::wstring text((size_t)textLength, L'\0');
    if (textLength) {
        GetWindowTextW(edit, &text[0], textLength + 1);
    }
    auto logical = [&text](DWORD position) {
        int result = 0;
        for (DWORD i = 0; i < position && i < text.size(); i++) {
            if (text[i] != L'\r') {
                result++;
            }
        }
        return result;
    };
    *start = logical(from);
    *length = logical(to) - *start;
}

extern "C" void gkWin32SetTextSelection(void *pointer, int start, int length) {
    HWND edit = gkTextControl(gkView(pointer));
    if (!edit) {
        return;
    }
    int size = GetWindowTextLengthW(edit);
    std::wstring text((size_t)size, L'\0');
    if (size) {
        GetWindowTextW(edit, &text[0], size + 1);
    }
    auto physical = [&text](int position) {
        int count = 0;
        size_t i = 0;
        for (; i < text.size() && count < position; i++) {
            if (text[i] != L'\r') {
                count++;
            }
        }
        return (int)i;
    };
    int logicalSize = (int)std::count_if(text.begin(), text.end(), [](wchar_t c) { return c != L'\r'; });
    start = start < 0 ? 0 : start > logicalSize ? logicalSize : start;
    length = length < 0 ? 0 : start + length > logicalSize ? logicalSize - start : length;
    SendMessageW(edit, EM_SETSEL, physical(start), physical(start + length));
    SendMessageW(edit, EM_SCROLLCARET, 0, 0);
}

/* GKTextAreaState holds the options of a text area that the edit control
   does not keep. */
struct GKTextAreaState {
    int wrap = 1;
    int tabMovesFocus = 1;
};

static std::unordered_map<GKView *, GKTextAreaState> &gkTextAreaStates() {
    static std::unordered_map<GKView *, GKTextAreaState> states;
    return states;
}

int gkTextAreaWraps(GKView *view) {
    auto found = gkTextAreaStates().find(view);
    return found == gkTextAreaStates().end() ? 1 : found->second.wrap;
}

int gkTextAreaTabMovesFocus(GKView *view) {
    auto found = gkTextAreaStates().find(view);
    return found == gkTextAreaStates().end() ? 1 : found->second.tabMovesFocus;
}

extern "C" void gkWin32TextAreaConfigure(void *pointer, int wrap, int maximumLength, int tabMovesFocus,
                                         int spellCheck) {
    (void)spellCheck;
    GKView *view = gkView(pointer);
    GKField *field = view && view->kind == GK_KIND_TEXT_AREA ? gkFieldOf(view) : nullptr;
    if (!field) {
        return;
    }
    GKTextAreaState &state = gkTextAreaStates()[view];
    state.tabMovesFocus = tabMovesFocus != 0;
    SendMessageW(field->edit, EM_SETLIMITTEXT, maximumLength > 0 ? maximumLength : 0, 0);
    if ((wrap != 0) != (state.wrap != 0)) {
        /* An edit control wraps or scrolls from its creation, so a change
           makes a new control with the same text and selection. */
        state.wrap = wrap != 0;
        int length = GetWindowTextLengthW(field->edit);
        std::wstring text((size_t)length, L'\0');
        if (length) {
            GetWindowTextW(field->edit, &text[0], length + 1);
        }
        DWORD from = 0, to = 0;
        SendMessageW(field->edit, EM_GETSEL, (WPARAM)&from, (LPARAM)&to);
        DWORD style = ES_MULTILINE | ES_WANTRETURN | ES_AUTOVSCROLL | WS_VSCROLL | WS_TABSTOP;
        if (!wrap) {
            style |= ES_AUTOHSCROLL | WS_HSCROLL;
        }
        if (view->readOnly) {
            style |= ES_READONLY;
        }
        HWND old = field->edit;
        bool focused = GetFocus() == old;
        HWND edit = gkMakeNative(WC_EDITW, L"", style, 0, view->outer);
        SetPropW(edit, L"GKView", view);
        field->edit = edit;
        view->control = edit;
        gkSubclass(edit);
        gkApplyFont(view);
        gkApplyThemeTo(edit, gkViewTheme(view));
        SendMessageW(edit, EM_SETMARGINS, EC_LEFTMARGIN | EC_RIGHTMARGIN, 0);
        SendMessageW(edit, EM_SETLIMITTEXT, maximumLength > 0 ? maximumLength : 0, 0);
        view->suppress++;
        SetWindowTextW(edit, text.c_str());
        view->suppress--;
        SendMessageW(edit, EM_SETSEL, from, to);
        RemovePropW(old, L"GKView");
        DestroyWindow(old);
        if (focused) {
            SetFocus(edit);
        }
        field->layout(view);
    }
}

void gkForgetTextArea(GKView *view) {
    gkTextAreaStates().erase(view);
}

/* ---- Scrolling ---- */

static HWND gkScrollingWindow(GKView *view) {
    if (!view) {
        return nullptr;
    }
    if (HWND edit = gkTextControl(view)) {
        return edit;
    }
    wchar_t name[32] = {};
    GetClassNameW(gkViewTarget(view), name, 32);
    if (!wcscmp(name, WC_LISTVIEWW) || !wcscmp(name, WC_TREEVIEWW) || !wcscmp(name, L"RICHEDIT50W")) {
        return gkViewTarget(view);
    }
    return nullptr;
}

extern "C" void gkWin32ScrollPosition(void *pointer, int *x, int *y) {
    GKView *view = gkView(pointer);
    *x = 0;
    *y = 0;
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_SCROLL) {
        double ox, oy;
        gkScrollGetOffset(view, &ox, &oy);
        *x = (int)floor(ox);
        *y = (int)floor(oy);
        return;
    }
    HWND target = gkScrollingWindow(view);
    if (!target) {
        return;
    }
    double scale = gkViewScaleOf(view);
    SCROLLINFO info = {sizeof(info), SIF_POS};
    if (GetScrollInfo(target, SB_HORZ, &info)) {
        *x = (int)lround(info.nPos / scale);
    }
    info.nPos = 0;
    if (GetScrollInfo(target, SB_VERT, &info)) {
        int line = (int)SendMessageW(target, EM_GETFIRSTVISIBLELINE, 0, 0);
        (void)line;
        *y = (int)lround(info.nPos / scale);
    }
}

extern "C" void gkWin32SetScrollPosition(void *pointer, int x, int y, int animated) {
    (void)animated;
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_SCROLL) {
        gkScrollSetOffset(view, x > 0 ? x : 0, y > 0 ? y : 0);
        return;
    }
    HWND target = gkScrollingWindow(view);
    if (!target) {
        return;
    }
    double scale = gkViewScaleOf(view);
    SCROLLINFO info = {sizeof(info), SIF_ALL};
    wchar_t name[32] = {};
    GetClassNameW(target, name, 32);
    if (!wcscmp(name, WC_LISTVIEWW)) {
        POINT origin = {0, 0};
        SendMessageW(target, LVM_GETORIGIN, 0, (LPARAM)&origin);
        RECT item = {};
        item.left = LVIR_BOUNDS;
        int rowHeight = SendMessageW(target, LVM_GETITEMRECT, 0, (LPARAM)&item) ? item.bottom - item.top : 1;
        SCROLLINFO vertical = {sizeof(vertical), SIF_POS};
        GetScrollInfo(target, SB_VERT, &vertical);
        int currentY = vertical.nPos * (rowHeight > 0 ? rowHeight : 1);
        SendMessageW(target, LVM_SCROLL, (WPARAM)(lround(x * scale) - origin.x),
                     (LPARAM)(lround(y * scale) - currentY));
        return;
    }
    if (GetScrollInfo(target, SB_VERT, &info)) {
        int position = (int)lround(y * scale);
        if (!wcscmp(name, WC_EDITW)) {
            /* An edit control scrolls by lines. */
            int line = gkMeasureGDI(L"Ag", gkViewFont(view, gkBodyFont())).cy;
            int first = (int)SendMessageW(target, EM_GETFIRSTVISIBLELINE, 0, 0);
            SendMessageW(target, EM_LINESCROLL, 0, (LPARAM)(y / std::max(line, 1) - first));
        } else {
            SendMessageW(target, WM_VSCROLL, MAKEWPARAM(SB_THUMBPOSITION, position), 0);
        }
    }
    if (x > 0 && GetScrollInfo(target, SB_HORZ, &info)) {
        SendMessageW(target, WM_HSCROLL, MAKEWPARAM(SB_THUMBPOSITION, (int)lround(x * scale)), 0);
    }
}

/* ---- Check boxes ---- */

struct GKCheckState {
    int allowsMixed = 0;
    int labelBefore = 0;
    std::wstring description;
};

static std::unordered_map<GKView *, GKCheckState> &gkCheckStates() {
    static std::unordered_map<GKView *, GKCheckState> states;
    return states;
}

extern "C" void gkWin32CheckBoxConfigure(void *pointer, int allowsMixed, int state, int labelBefore,
                                         const char *text, const char *description) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_CHECK_BOX) {
        return;
    }
    GKCheckState &check = gkCheckStates()[view];
    check.allowsMixed = allowsMixed != 0;
    check.labelBefore = labelBefore != 0;
    check.description = gkWide(description);
    HWND box = view->outer;
    LONG_PTR style = GetWindowLongPtrW(box, GWL_STYLE) & ~(LONG_PTR)(BS_TYPEMASK | BS_LEFTTEXT | BS_MULTILINE);
    style |= allowsMixed ? BS_AUTO3STATE : BS_AUTOCHECKBOX;
    if (labelBefore) {
        style |= BS_LEFTTEXT;
    }
    if (!check.description.empty()) {
        style |= BS_MULTILINE;
    }
    SetWindowLongPtrW(box, GWL_STYLE, style);
    view->text = gkWide(text);
    std::wstring shown = view->text;
    if (!check.description.empty()) {
        shown += L"\n" + check.description;
    }
    SetWindowTextW(box, shown.c_str());
    view->suppress++;
    SendMessageW(box, BM_SETCHECK, state == 2 && allowsMixed ? BST_INDETERMINATE : state == 1 ? BST_CHECKED
                                                                                              : BST_UNCHECKED,
                 0);
    view->suppress--;
    view->accessibleDescription = gkUTF8(check.description);
    InvalidateRect(box, nullptr, TRUE);
}

extern "C" int gkWin32CheckBoxState(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_CHECK_BOX) {
        return 0;
    }
    LRESULT state = SendMessageW(view->outer, BM_GETCHECK, 0, 0);
    return state == BST_INDETERMINATE ? 2 : state == BST_CHECKED ? 1 : 0;
}

void gkCheckBoxSetText(GKView *view, std::wstring const &text) {
    view->text = text;
    auto found = gkCheckStates().find(view);
    std::wstring shown = text;
    if (found != gkCheckStates().end() && !found->second.description.empty()) {
        shown += L"\n" + found->second.description;
    }
    SetWindowTextW(view->outer, shown.c_str());
}

int gkCheckBoxLabelBefore(GKView *view) {
    auto found = gkCheckStates().find(view);
    return found != gkCheckStates().end() && found->second.labelBefore;
}

std::wstring gkCheckBoxDescription(GKView *view) {
    auto found = gkCheckStates().find(view);
    return found == gkCheckStates().end() ? std::wstring() : found->second.description;
}

void gkForgetCheckBox(GKView *view) {
    gkCheckStates().erase(view);
}

/* ---- Choices ---- */

extern "C" void gkWin32SetChoices(void *pointer, const char *titles, size_t titlesSize, const int *enabled,
                                  int count) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    std::vector<std::string> values = gkSplit(titles, titlesSize);
    std::vector<int> flags(values.size(), 1);
    for (int i = 0; i < count && i < (int)values.size(); i++) {
        flags[i] = enabled ? enabled[i] : 1;
    }
    view->suppress++;
    if (view->kind == GK_KIND_RADIO_GROUP) {
        gkRadioGroupSetChoices(view, values, flags.data());
    } else if (HWND combo = gkSelectCombo(view)) {
        int selected = (int)SendMessageW(combo, CB_GETCURSEL, 0, 0);
        SendMessageW(combo, CB_RESETCONTENT, 0, 0);
        for (auto const &value : values) {
            SendMessageW(combo, CB_ADDSTRING, 0, (LPARAM)gkWide(value.c_str()).c_str());
        }
        SendMessageW(combo, CB_SETCURSEL, selected < (int)values.size() ? selected : -1, 0);
        view->selected = selected < (int)values.size() ? selected : -1;
    }
    view->suppress--;
}

extern "C" void gkWin32RadioGroupConfigure(void *pointer, int horizontal, const char *label) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_RADIO_GROUP) {
        gkRadioGroupConfigure(view, horizontal, label);
    }
}

extern "C" void gkWin32RadioGroupPreferredSize(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    *width = *height = 0;
    if (view && view->kind == GK_KIND_RADIO_GROUP) {
        gkRadioGroupPreferred(view, width, height);
    }
    *width = *width > 120 ? *width : 120;
    *height = *height > 24 ? *height : 24;
}

extern "C" void gkWin32SelectConfigure(void *pointer, const char *placeholder, int popupWidth) {
    GKView *view = gkView(pointer);
    if (view && gkSelectCombo(view)) {
        view->placeholder = gkWide(placeholder);
        gkSelectConfigure(view, placeholder, popupWidth);
    }
}

extern "C" void gkWin32SelectSetVisibleRows(void *pointer, int rows) {
    GKView *view = gkView(pointer);
    if (view && gkSelectCombo(view)) {
        gkSelectSetVisibleRows(view, rows);
    }
}

/* ---- Value controls ---- */

extern "C" void gkWin32SliderConfigure(void *pointer, int vertical, double minimum, double maximum, double frequency,
                                       int snap) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_SLIDER) {
        gkSliderConfigure(view, vertical, minimum, maximum, frequency, snap);
    }
}

extern "C" void gkWin32SliderSetInverted(void *pointer, int inverted) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_SLIDER) {
        gkSliderSetInverted(view, inverted);
    }
}

struct GKProgressState {
    std::wstring text;
    int state = 0;
};

static std::unordered_map<GKView *, GKProgressState> &gkProgressStates() {
    static std::unordered_map<GKView *, GKProgressState> states;
    return states;
}

extern "C" void gkWin32ProgressConfigure(void *pointer, int indeterminate, int vertical, const char *text,
                                         int state) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_PROGRESS) {
        return;
    }
    HWND bar = view->outer;
    GKProgressState &progress = gkProgressStates()[view];
    progress.text = gkWide(text);
    progress.state = state;
    view->running = indeterminate != 0;
    view->vertical = vertical != 0;
    LONG_PTR style = GetWindowLongPtrW(bar, GWL_STYLE) & ~(LONG_PTR)(PBS_MARQUEE | PBS_VERTICAL);
    if (indeterminate) {
        style |= PBS_MARQUEE;
    }
    if (vertical) {
        style |= PBS_VERTICAL;
    }
    SetWindowLongPtrW(bar, GWL_STYLE, style);
    SendMessageW(bar, PBM_SETMARQUEE, indeterminate ? TRUE : FALSE, 30);
    if (!indeterminate) {
        SendMessageW(bar, PBM_SETPOS, (WPARAM)lround(view->value * 10000), 0);
    }
    static const int states[] = {PBST_NORMAL, PBST_PAUSED, PBST_PAUSED, PBST_ERROR};
    SendMessageW(bar, PBM_SETSTATE, states[state >= 0 && state < 4 ? state : 0], 0);
    view->accessibleValue = gkUTF8(progress.text);
    InvalidateRect(bar, nullptr, TRUE);
}

int gkProgressState(GKView *view) {
    auto found = gkProgressStates().find(view);
    return found == gkProgressStates().end() ? 0 : found->second.state;
}

void gkForgetProgress(GKView *view) {
    gkProgressStates().erase(view);
}

extern "C" void gkWin32ActivityConfigure(void *pointer, int size, const char *label) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_ACTIVITY) {
        return;
    }
    gkActivitySetSize(view, size);
    std::string text = label ? label : "";
    extern void gkWin32SetTooltip(void *pointer, const char *text);
    gkWin32SetTooltip(view, text.c_str());
    if (view->accessibleName.empty() || view->accessibleName == gkUTF8(view->text)) {
        view->text = gkWide(label);
    }
}

extern "C" void gkWin32SeparatorConfigure(void *pointer, int vertical, const char *label, int inset, int thickness) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_SEPARATOR) {
        gkSeparatorConfigure(view, vertical, label, inset ? 16 : 0, thickness);
        view->text = gkWide(label);
    }
}

extern "C" void gkWin32ImageConfigure(void *pointer, int alignment, int nearest, const char *alternative,
                                      double scale) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_IMAGE) {
        return;
    }
    view->alignment = alignment;
    view->nearest = nearest != 0;
    view->sourceScale = scale > 0 ? scale : 1;
    view->text = gkWide(alternative);
    /* An image without a description is decorative. */
    view->accessibleHidden = !(alternative && *alternative) && view->accessibleName.empty();
    InvalidateRect(view->outer, nullptr, FALSE);
}

/* ---- Links ---- */

extern "C" void gkWin32LinkConfigure(void *pointer, int visited, void *icon) {
    (void)icon;
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_LINK) {
        return;
    }
    LITEM item = {};
    item.mask = LIF_ITEMINDEX | LIF_STATE;
    item.iLink = 0;
    item.stateMask = LIS_VISITED;
    item.state = visited ? LIS_VISITED : 0;
    SendMessageW(view->outer, LM_SETITEM, 0, (LPARAM)&item);
    view->accessibleValue = visited ? "visited" : "";
    InvalidateRect(view->outer, nullptr, TRUE);
}

int gkOpenURLWide(std::wstring const &target, int background) {
    if (target.empty()) {
        return 0;
    }
    SHELLEXECUTEINFOW info = {};
    info.cbSize = sizeof(info);
    info.fMask = SEE_MASK_NOASYNC | SEE_MASK_FLAG_NO_UI;
    info.lpVerb = L"open";
    info.lpFile = target.c_str();
    info.nShow = background ? SW_SHOWNOACTIVATE : SW_SHOWNORMAL;
    HWND foreground = background ? GetForegroundWindow() : nullptr;
    if (!ShellExecuteExW(&info)) {
        return 0;
    }
    if (foreground) {
        /* The application that opens the URL may still activate itself, so
           the window that was in front takes the foreground back. */
        SetForegroundWindow(foreground);
        gkLater([foreground]() {
            if (IsWindow(foreground)) {
                SetForegroundWindow(foreground);
            }
        });
    }
    return 1;
}

extern "C" int gkWin32OpenURL(const char *url, int background) {
    return gkOpenURLWide(gkWide(url), background);
}

/* ---- Scroll views ---- */

extern "C" void gkWin32ScrollConfigure(void *pointer, int axes, int policy, int top, int right, int bottom, int left,
                                       int overscroll) {
    (void)overscroll;
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SCROLL) {
        return;
    }
    view->axes = axes;
    int horizontal = axes == 1 || axes == 2;
    int vertical = axes == 0 || axes == 2;
    LONG_PTR style = GetWindowLongPtrW(view->outer, GWL_STYLE) & ~(LONG_PTR)(WS_HSCROLL | WS_VSCROLL);
    if (policy != 2) {
        style |= (horizontal ? WS_HSCROLL : 0) | (vertical ? WS_VSCROLL : 0);
    }
    SetWindowLongPtrW(view->outer, GWL_STYLE, style);
    SetWindowPos(view->outer, nullptr, 0, 0, 0, 0,
                 SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
    gkScrollSetInsets(view, top, right, bottom, left);
    gkScrollSetPolicy(view, policy == 1 ? 2 : policy == 2 ? 1 : 0);
}

extern "C" void gkWin32ScrollSetZoom(void *pointer, double zoom) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_SCROLL) {
        gkScrollSetZoom(view, zoom);
    }
}

extern "C" double gkWin32ScrollZoom(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_SCROLL ? gkScrollGetZoom(view) : 1;
}

extern "C" void gkWin32ScrollViewport(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    *width = 0;
    *height = 0;
    if (view && view->kind == GK_KIND_SCROLL) {
        double w, h;
        gkScrollViewport(view, &w, &h);
        *width = (int)floor(w);
        *height = (int)floor(h);
    }
}

extern "C" int gkWin32ScrollToView(void *scrollPointer, void *viewPointer, int animated) {
    (void)animated;
    GKView *scrollView = gkView(scrollPointer);
    GKView *target = gkView(viewPointer);
    if (!scrollView || !target || scrollView->kind != GK_KIND_SCROLL || !scrollView->body) {
        return 0;
    }
    if (!IsChild(scrollView->body, target->outer)) {
        return 0;
    }
    /* The position of the view in the content of the scroll view, in
       logical units. */
    RECT rect;
    GetWindowRect(target->outer, &rect);
    MapWindowPoints(nullptr, scrollView->body, reinterpret_cast<POINT *>(&rect), 2);
    double scale = gkViewScaleOf(scrollView);
    double x = rect.left / scale, y = rect.top / scale;
    double width = (rect.right - rect.left) / scale, height = (rect.bottom - rect.top) / scale;
    double ox, oy, viewportWidth, viewportHeight;
    gkScrollGetOffset(scrollView, &ox, &oy);
    gkScrollViewport(scrollView, &viewportWidth, &viewportHeight);
    if (x + width > ox + viewportWidth) {
        ox = x + width - viewportWidth;
    }
    if (x < ox) {
        ox = x;
    }
    if (y + height > oy + viewportHeight) {
        oy = y + height - viewportHeight;
    }
    if (y < oy) {
        oy = y;
    }
    gkScrollSetOffset(scrollView, ox, oy);
    return 1;
}

extern "C" void gkWin32TabsConfigure(void *tabs, int placement);
extern "C" void gkWin32TabsSetOverflow(void *tabs, int policy);
