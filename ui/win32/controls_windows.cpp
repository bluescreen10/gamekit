//go:build windows && cgo

#include "win32_internal.h"

#include <commdlg.h>
#include <shldisp.h>
#include <shobjidl.h>
#include <time.h>
#include <uxtheme.h>
#include <vssym32.h>

#include <algorithm>

extern "C" void gkWin32ButtonConfigure(void *pointer, void *icon, int placement, int role, const char *key,
                                       int modifiers);
GKView *gkSelectCreate(uintptr_t window, int kind, DWORD style, uintptr_t action);
HWND gkSelectCombo(GKView *view);

extern "C" void gkWin32Disconnect(void *pointer) {
    GKView *view = gkView(pointer);
    if (view) {
        view->handle = 0;
    }
}

static double gkClampValue(double value, double minimum, double maximum) {
    return value < minimum ? minimum : value > maximum ? maximum : value;
}

/* gkViewNative creates a view for a native control. */
static GKView *gkViewNative(uintptr_t window, const wchar_t *className, const wchar_t *text, DWORD style, int kind,
                            GKExtension *ext) {
    HWND hwnd = gkMakeNative(className, text, style, 0);
    if (!hwnd) {
        delete ext;
        return nullptr;
    }
    GKView *view = gkAdopt(window, hwnd, nullptr, kind, 0);
    view->ext.reset(ext);
    return view;
}

/* ---- Toggle button ---- */

struct GKToggle : GKExtension {
    bool command(GKView *view, HWND from, int code) override {
        (void)from;
        if (code == BN_CLICKED) {
            gkReport(view, GK_EVENT_VALUE, 0, 0, nullptr);
            return true;
        }
        return false;
    }
    void themeChanged(GKView *view) override {
        gkButtonThemeChanged(view);
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        return gkButtonPreferred(view);
    }
};

extern "C" void *gkWin32ToggleButtonCreate(uintptr_t window, const char *text, void *icon, int checked, int toolbar) {
    (void)toolbar;
    std::wstring value = gkWide(text);
    GKView *view = gkViewNative(window, WC_BUTTONW, value.c_str(), BS_AUTOCHECKBOX | BS_PUSHLIKE | WS_TABSTOP,
                                GK_KIND_TOGGLE, new GKToggle());
    if (!view) {
        return nullptr;
    }
    view->text = value;
    SendMessageW(view->outer, BM_SETCHECK, checked ? BST_CHECKED : BST_UNCHECKED, 0);
    if (icon) {
        gkWin32ButtonConfigure(view, icon, 0, 0, nullptr, 0);
    }
    return view;
}

/* ---- Switch ----

   Windows has no switch control in its classic controls, so a switch is a
   check box, which assistive technology reports as a switch. */

struct GKSwitch : GKExtension {
    std::string on, off;
    bool command(GKView *view, HWND from, int code) override {
        (void)from;
        if (code == BN_CLICKED) {
            view->accessibleValue = SendMessageW(view->outer, BM_GETCHECK, 0, 0) == BST_CHECKED ? on : off;
            InvalidateRect(view->outer, nullptr, FALSE);
            gkReport(view, GK_EVENT_VALUE, 0, 0, nullptr);
            return true;
        }
        return false;
    }
    /* The track of the switch, in logical units, for its control size:
       regular, mini, small, or large. */
    int size = 0;
    float trackWidth = 40, trackHeight = 20, gap = 10;
    void setSize(int value) {
        size = value;
        static const float widths[] = {40, 28, 34, 48}, heights[] = {20, 14, 17, 24};
        int index = size >= 0 && size < 4 ? size : 0;
        trackWidth = widths[index];
        trackHeight = heights[index];
        gap = index == 1 ? 6 : index == 2 ? 8 : 10;
    }
    GKFont font(GKView *view) {
        GKFont base = gkBodyFont();
        static const float sizes[] = {0, 11, 12, 16};
        int index = size >= 0 && size < 4 ? size : 0;
        if (sizes[index] > 0) {
            base.size = sizes[index];
        }
        return gkViewFont(view, base);
    }
    std::wstring label(GKView *view) {
        if (!view->text.empty()) {
            return view->text;
        }
        bool checked = SendMessageW(view->outer, BM_GETCHECK, 0, 0) == BST_CHECKED;
        return gkWide((checked ? on : off).c_str());
    }
    /* notify draws the switch over the check box, which keeps the keyboard
       behavior, the focus, and the accessibility of a check box: Windows
       has no switch among its common controls. */
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->code != NM_CUSTOMDRAW || header->hwndFrom != view->outer) {
            return false;
        }
        NMCUSTOMDRAW *draw = reinterpret_cast<NMCUSTOMDRAW *>(header);
        if (draw->dwDrawStage != CDDS_PREPAINT) {
            *result = CDRF_DODEFAULT;
            return true;
        }
        GKPalette const &palette = gkViewPalette(view);
        bool checked = SendMessageW(view->outer, BM_GETCHECK, 0, 0) == BST_CHECKED;
        bool enabled = IsWindowEnabled(view->outer) != 0;
        bool hot = (draw->uItemState & CDIS_HOT) != 0;
        bool pressed = (draw->uItemState & CDIS_SELECTED) != 0;
        bool focused = (draw->uItemState & CDIS_FOCUS) != 0 &&
                       !(SendMessageW(view->outer, WM_QUERYUISTATE, 0, 0) & UISF_HIDEFOCUS);
        std::wstring text = label(view);
        GKFont textFont = font(view);
        gkPaintCanvas(view->outer, draw->hdc, draw->rc, [&](GKCanvas &canvas) {
            float y = (canvas.height - trackHeight) / 2, x = 3;
            float radius = trackHeight / 2;
            if (checked) {
                GKColor fill = !enabled ? palette.accentDisabled
                               : pressed ? palette.accentPressed
                               : hot     ? palette.accentHover
                                         : palette.accent;
                canvas.fillRound(x, y, trackWidth, trackHeight, radius, fill);
            } else {
                canvas.fillRound(x, y, trackWidth, trackHeight, radius,
                                 hot && enabled ? palette.controlHover : palette.control);
                canvas.strokeRound(x + 0.5f, y + 0.5f, trackWidth - 1, trackHeight - 1, radius - 0.5f,
                                   enabled ? palette.controlStrong : palette.controlStrongDisabled);
            }
            /* The thumb grows under the pointer and stretches while pressed,
               as in Windows. */
            float unit = trackHeight / 20;
            float thumb = (pressed || hot ? 14 : 12) * unit;
            float stretch = pressed ? 3 * unit : 0;
            float cy = y + trackHeight / 2;
            float cx = checked ? x + trackWidth - trackHeight / 2 - stretch / 2 : x + trackHeight / 2 + stretch / 2;
            GKColor knob = checked ? palette.textOnAccent
                           : enabled ? palette.textSecondary
                                     : palette.textDisabled;
            canvas.fillRound(cx - thumb / 2 - stretch / 2, cy - thumb / 2, thumb + stretch, thumb, thumb / 2, knob);
            if (focused) {
                canvas.strokeRound(x - 2, y - 2, trackWidth + 4, trackHeight + 4, radius + 2, palette.focus, 1.5f);
            }
            if (!text.empty()) {
                float tx = x + trackWidth + gap;
                canvas.text(text, textFont, tx, 0, canvas.width - tx, canvas.height,
                            enabled ? gkViewTextColor(view, palette.text) : palette.textDisabled, 0, 1);
            }
        });
        *result = CDRF_SKIPDEFAULT;
        return true;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        GKFont textFont = font(view);
        std::wstring text = view->text.empty() ? gkWide((on.size() > off.size() ? on : off).c_str()) : view->text;
        float width = trackWidth + 6;
        if (!text.empty()) {
            width += gap + (float)gkMeasureGDI(text, textFont).cx + 2;
        }
        float height = std::max(trackHeight + 6, (float)gkMeasureGDI(L"Ag", textFont).cy + 4);
        return D2D1::SizeF(width, height);
    }
};

extern "C" void *gkWin32SwitchCreate(uintptr_t window, const char *text, const char *onLabel, const char *offLabel,
                                     int checked, int size) {
    std::wstring value = gkWide(text);
    GKSwitch *state = new GKSwitch();
    state->setSize(size);
    state->on = onLabel ? onLabel : "";
    state->off = offLabel ? offLabel : "";
    GKView *view = gkViewNative(window, WC_BUTTONW, value.c_str(), BS_AUTOCHECKBOX | WS_TABSTOP, GK_KIND_SWITCH, state);
    if (!view) {
        return nullptr;
    }
    view->text = value;
    SendMessageW(view->outer, BM_SETCHECK, checked ? BST_CHECKED : BST_UNCHECKED, 0);
    view->accessibleValue = checked ? state->on : state->off;
    return view;
}

extern "C" int gkWin32SwitchChecked(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_SWITCH && SendMessageW(view->outer, BM_GETCHECK, 0, 0) == BST_CHECKED;
}

extern "C" void gkWin32SwitchSet(void *pointer, const char *text, int checked) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SWITCH) {
        return;
    }
    view->text = gkWide(text);
    SetWindowTextW(view->outer, view->text.c_str());
    view->suppress++;
    SendMessageW(view->outer, BM_SETCHECK, checked ? BST_CHECKED : BST_UNCHECKED, 0);
    view->suppress--;
    GKSwitch *state = static_cast<GKSwitch *>(view->ext.get());
    view->accessibleValue = checked ? state->on : state->off;
}

/* ---- Password field ---- */

static const wchar_t gkPasswordCharacter = 0x25cf;

/* GKPassword is a password text box with a button that shows the
   password while it is pressed down. */
struct GKPassword : GKField {
    HWND reveal = nullptr;
    int revealed = 0;
    void layout(GKView *view) override {
        if (reveal) {
            RECT client;
            GetClientRect(view->outer, &client);
            double scale = gkViewScaleOf(view);
            int width = (int)lround(28 * scale), border = (int)lround(1 * scale);
            SetWindowPos(reveal, nullptr, client.right - border - width, border, width, client.bottom - 2 * border,
                         SWP_NOZORDER | SWP_NOACTIVATE);
            /* The edit control stops before the button. */
            GKField::layout(view);
            RECT edit;
            GetWindowRect(this->edit, &edit);
            MapWindowPoints(nullptr, view->outer, reinterpret_cast<POINT *>(&edit), 2);
            SetWindowPos(this->edit, nullptr, 0, 0, client.right - border - width - edit.left, edit.bottom - edit.top,
                         SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE);
            return;
        }
        GKField::layout(view);
    }
    void show(int visible) {
        SendMessageW(edit, EM_SETPASSWORDCHAR, visible ? 0 : gkPasswordCharacter, 0);
        InvalidateRect(edit, nullptr, TRUE);
    }
    bool command(GKView *view, HWND from, int code) override {
        if (from == reveal && code == BN_CLICKED) {
            int visible = SendMessageW(reveal, TB_ISBUTTONCHECKED, 1, 0) != 0;
            show(visible || revealed);
            return true;
        }
        return GKField::command(view, from, code);
    }
    void icons(GKView *view) {
        GKIcon eye;
        eye.glyph = L"\xE7B3";
        int size = (int)lround(16 * gkViewScaleOf(view));
        HIMAGELIST list = gkComctl().ImageList_Create(size, size, ILC_COLOR32, 1, 0);
        HBITMAP bitmap = gkIconBitmap(&eye, size, gkViewPalette(view).text);
        gkComctl().ImageList_Add(list, bitmap, nullptr);
        DeleteObject(bitmap);
        SendMessageW(reveal, TB_SETIMAGELIST, 0, (LPARAM)list);
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
        images = list;
        SendMessageW(reveal, TB_SETBUTTONSIZE, 0, MAKELPARAM(size + (int)lround(10 * gkViewScaleOf(view)), size));
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->hwndFrom != reveal || header->code != NM_CUSTOMDRAW) {
            return false;
        }
        NMTBCUSTOMDRAW *draw = reinterpret_cast<NMTBCUSTOMDRAW *>(header);
        if (draw->nmcd.dwDrawStage == CDDS_PREERASE) {
            /* The toolbar lies inside the text box, so it has the fill of
               the text box behind its button. */
            GKPalette const &palette = gkViewPalette(view);
            GKColor fill = palette.dark ? palette.inputActive : gkRGB(0xffffff);
            RECT client;
            GetClientRect(reveal, &client);
            HBRUSH brush = CreateSolidBrush(
                RGB((int)lround(fill.r * 255), (int)lround(fill.g * 255), (int)lround(fill.b * 255)));
            FillRect(draw->nmcd.hdc, &client, brush);
            DeleteObject(brush);
            *result = CDRF_SKIPDEFAULT;
            return true;
        }
        *result = CDRF_DODEFAULT;
        return true;
    }
    void themeChanged(GKView *view) override {
        GKField::themeChanged(view);
        if (reveal) {
            gkApplyThemeTo(reveal, gkViewTheme(view));
            icons(view);
        }
    }
    HIMAGELIST images = nullptr;
    ~GKPassword() override {
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
    }
};

extern "C" void *gkWin32PasswordCreate(uintptr_t window, const char *placeholder, int reveal) {
    GKPassword *field = new GKPassword();
    field->report = 1;
    GKView *view = gkFieldCreate(window, GK_KIND_PASSWORD, ES_AUTOHSCROLL | ES_PASSWORD, 0, field);
    SendMessageW(field->edit, EM_SETPASSWORDCHAR, gkPasswordCharacter, 0);
    view->placeholder = gkWide(placeholder);
    SendMessageW(field->edit, EM_SETCUEBANNER, TRUE, (LPARAM)view->placeholder.c_str());
    view->value = reveal;
    if (reveal) {
        /* The reveal button is a flat toolbar button, which has no border
           until the pointer is over it, like the reveal button of Windows. */
        field->reveal = gkMakeNative(TOOLBARCLASSNAMEW, L"",
                                     TBSTYLE_FLAT | TBSTYLE_CUSTOMERASE | TBSTYLE_TOOLTIPS | CCS_NORESIZE |
                                         CCS_NODIVIDER | CCS_NOPARENTALIGN,
                                     0, view->outer);
        SetPropW(field->reveal, L"GKView", view);
        SendMessageW(field->reveal, TB_BUTTONSTRUCTSIZE, sizeof(TBBUTTON), 0);
        field->icons(view);
        TBBUTTON button = {};
        button.iBitmap = 0;
        button.idCommand = 1;
        button.fsState = TBSTATE_ENABLED;
        button.fsStyle = BTNS_CHECK;
        SendMessageW(field->reveal, TB_ADDBUTTONSW, 1, (LPARAM)&button);
        gkApplyThemeTo(field->reveal, gkViewTheme(view));
        field->layout(view);
    }
    return view;
}

extern "C" char *gkWin32PasswordText(void *pointer) {
    GKView *view = gkView(pointer);
    GKField *field = gkFieldOf(view);
    if (!field) {
        return gkStrdup("");
    }
    int length = GetWindowTextLengthW(field->edit);
    std::wstring text((size_t)length, L'\0');
    if (length) {
        GetWindowTextW(field->edit, &text[0], length + 1);
    }
    return gkStrdupW(text);
}

extern "C" void gkWin32PasswordSet(void *pointer, const char *text, const char *placeholder, int revealed) {
    GKView *view = gkView(pointer);
    GKPassword *field = view ? dynamic_cast<GKPassword *>(view->ext.get()) : nullptr;
    if (!field) {
        return;
    }
    std::wstring value = gkWide(text);
    int length = GetWindowTextLengthW(field->edit);
    std::wstring current((size_t)length, L'\0');
    if (length) {
        GetWindowTextW(field->edit, &current[0], length + 1);
    }
    view->suppress++;
    if (current != value) {
        SetWindowTextW(field->edit, value.c_str());
    }
    view->suppress--;
    view->placeholder = gkWide(placeholder);
    SendMessageW(field->edit, EM_SETCUEBANNER, TRUE, (LPARAM)view->placeholder.c_str());
    field->revealed = revealed != 0;
    field->show(revealed);
}

/* ---- Search field ----

   The suggestions of a search field appear in the autocomplete list of
   Windows, which the shell provides for edit controls. */

struct GKSuggestions : IEnumString {
    LONG refs = 1;
    std::vector<std::wstring> items;
    size_t next = 0;
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
        if (riid == IID_IUnknown || riid == IID_IEnumString) {
            *object = static_cast<IEnumString *>(this);
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
    HRESULT STDMETHODCALLTYPE Next(ULONG count, LPOLESTR *out, ULONG *fetched) override {
        ULONG done = 0;
        while (done < count && next < items.size()) {
            size_t bytes = (items[next].size() + 1) * sizeof(wchar_t);
            out[done] = static_cast<LPOLESTR>(CoTaskMemAlloc(bytes));
            memcpy(out[done], items[next].c_str(), bytes);
            done++;
            next++;
        }
        if (fetched) {
            *fetched = done;
        }
        return done == count ? S_OK : S_FALSE;
    }
    HRESULT STDMETHODCALLTYPE Skip(ULONG count) override {
        next += count;
        return next <= items.size() ? S_OK : S_FALSE;
    }
    HRESULT STDMETHODCALLTYPE Reset() override {
        next = 0;
        return S_OK;
    }
    HRESULT STDMETHODCALLTYPE Clone(IEnumString **out) override {
        GKSuggestions *copy = new GKSuggestions();
        copy->items = items;
        copy->next = next;
        *out = copy;
        return S_OK;
    }
};

/* CLSID_AutoComplete is the autocomplete object of the shell. */
static const CLSID gkAutoComplete = {0x00bb2763, 0x6a77, 0x11d0, {0xa5, 0x35, 0x00, 0xc0, 0x4f, 0xd7, 0xd0, 0x62}};

struct GKSearch : GKField {
    GKSuggestions *suggestions = nullptr;
    IAutoComplete2 *complete = nullptr;
    ~GKSearch() override {
        if (complete) {
            complete->Release();
        }
        if (suggestions) {
            suggestions->Release();
        }
    }
};

extern "C" void *gkWin32SearchCreate(uintptr_t window, const char *text, const char *placeholder) {
    GKSearch *field = new GKSearch();
    field->report = 1;
    GKView *view = gkFieldCreate(window, GK_KIND_SEARCH, ES_AUTOHSCROLL, 0, field);
    view->placeholder = gkWide(placeholder);
    SendMessageW(field->edit, EM_SETCUEBANNER, TRUE, (LPARAM)view->placeholder.c_str());
    view->suppress++;
    SetWindowTextW(field->edit, gkWide(text).c_str());
    view->suppress--;
    field->suggestions = new GKSuggestions();
    if (SUCCEEDED(CoCreateInstance(gkAutoComplete, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&field->complete)))) {
        field->complete->Init(field->edit, field->suggestions, nullptr, nullptr);
        field->complete->SetOptions(ACO_AUTOSUGGEST | ACO_UPDOWNKEYDROPSLIST);
    }
    return view;
}

extern "C" void gkWin32SearchSetSuggestions(void *pointer, const char *items, size_t size) {
    GKView *view = gkView(pointer);
    GKSearch *field = view ? dynamic_cast<GKSearch *>(view->ext.get()) : nullptr;
    if (!field) {
        return;
    }
    field->suggestions->items.clear();
    for (auto const &value : gkSplit(items, size)) {
        field->suggestions->items.push_back(gkWide(value.c_str()));
    }
    field->suggestions->next = 0;
}

/* ---- Number field ---- */

struct GKNumber : GKField {
    double minimum = 0, maximum = 100, step = 1;
    int decimals = 0;
    std::wstring format(double value) {
        wchar_t text[64];
        swprintf(text, 64, L"%.*f", decimals, value);
        return text;
    }
    void show(GKView *view) {
        view->suppress++;
        SetWindowTextW(edit, format(view->value).c_str());
        view->suppress--;
    }
    /* commit reads the value from the text, as a number box does when it
       loses the focus or the user presses Return. */
    void commit(GKView *view) {
        int length = GetWindowTextLengthW(edit);
        std::wstring text((size_t)length, L'\0');
        if (length) {
            GetWindowTextW(edit, &text[0], length + 1);
        }
        wchar_t *end = nullptr;
        double value = wcstod(text.c_str(), &end);
        if (end == text.c_str()) {
            show(view);
            return;
        }
        set(view, value);
    }
    void set(GKView *view, double value) {
        double factor = pow(10, decimals);
        value = round(gkClampValue(value, minimum, maximum) * factor) / factor;
        bool changed = value != view->value;
        view->value = value;
        show(view);
        if (changed) {
            gkReport(view, GK_EVENT_VALUE, value, 0, nullptr);
        }
    }
    void changed(GKView *view) override {
        (void)view;
    }
    void submitted(GKView *view) override {
        commit(view);
    }
    void focusLost(GKView *view) override {
        commit(view);
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->hwndFrom == updown && header->code == UDN_DELTAPOS) {
            NMUPDOWN *change = reinterpret_cast<NMUPDOWN *>(header);
            commit(view);
            set(view, view->value - change->iDelta * step);
            *result = 1;
            return true;
        }
        return false;
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        if (hwnd == edit && message == WM_KEYDOWN && (wparam == VK_UP || wparam == VK_DOWN)) {
            commit(view);
            set(view, view->value + (wparam == VK_UP ? step : -step));
            *result = 0;
            return true;
        }
        return GKField::message(view, hwnd, message, wparam, lparam, result);
    }
};

extern "C" void *gkWin32NumberCreate(uintptr_t window, double minimum, double maximum, double value, double step,
                                     int decimals) {
    GKNumber *field = new GKNumber();
    field->report = 1;
    field->minimum = minimum;
    field->maximum = maximum > minimum ? maximum : minimum;
    field->step = step > 0 ? step : 1;
    field->decimals = decimals > 0 ? decimals : 0;
    GKView *view = gkFieldCreate(window, GK_KIND_NUMBER, ES_AUTOHSCROLL | ES_RIGHT, 0, field);
    field->updown = gkMakeNative(UPDOWN_CLASSW, L"", UDS_ARROWKEYS | UDS_HOTTRACK, 0, view->outer);
    SetPropW(field->updown, L"GKView", view);
    SendMessageW(field->updown, UDM_SETRANGE32, (WPARAM)-1000000, 1000000);
    gkApplyThemeTo(field->updown, gkViewTheme(view));
    view->value = gkClampValue(value, field->minimum, field->maximum);
    view->minimum = field->minimum;
    view->maximum = field->maximum;
    field->show(view);
    field->layout(view);
    return view;
}

extern "C" double gkWin32NumberValue(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_NUMBER ? view->value : 0;
}

extern "C" void gkWin32NumberSetValue(void *pointer, double value) {
    GKView *view = gkView(pointer);
    GKNumber *field = view ? dynamic_cast<GKNumber *>(view->ext.get()) : nullptr;
    if (field) {
        double factor = pow(10, field->decimals);
        view->value = round(gkClampValue(value, field->minimum, field->maximum) * factor) / factor;
        field->show(view);
    }
}

/* ---- Editable combo ---- */

static void gkComboFill(HWND combo, const char *items, size_t size) {
    SendMessageW(combo, CB_RESETCONTENT, 0, 0);
    for (auto const &value : gkSplit(items, size)) {
        SendMessageW(combo, CB_ADDSTRING, 0, (LPARAM)gkWide(value.c_str()).c_str());
    }
}

extern "C" void *gkWin32ComboCreate(uintptr_t window, const char *text, const char *items, size_t size) {
    GKView *view = gkSelectCreate(window, GK_KIND_COMBO, CBS_DROPDOWN | CBS_AUTOHSCROLL, 0);
    HWND combo = gkSelectCombo(view);
    view->suppress++;
    gkComboFill(combo, items, size);
    SetWindowTextW(combo, gkWide(text).c_str());
    view->suppress--;
    return view;
}

extern "C" void gkWin32ComboSetItems(void *pointer, const char *items, size_t size) {
    GKView *view = gkView(pointer);
    HWND combo = view && view->kind == GK_KIND_COMBO ? gkSelectCombo(view) : nullptr;
    if (!combo) {
        return;
    }
    int length = GetWindowTextLengthW(combo);
    std::wstring text((size_t)length, L'\0');
    if (length) {
        GetWindowTextW(combo, &text[0], length + 1);
    }
    view->suppress++;
    gkComboFill(combo, items, size);
    SetWindowTextW(combo, text.c_str());
    view->suppress--;
}

/* ---- Segmented control ----

   A segmented control is a row of buttons. Mode 0 is momentary buttons,
   mode 1 selects one segment like radio buttons, and mode 2 selects any
   set of segments like check boxes. */

struct GKSegments : GKExtension {
    int mode = 0;
    int clicked = -1;
    std::vector<HWND> buttons;
    std::vector<std::wstring> titles;
    std::vector<HIMAGELIST> images;
    std::vector<GKIcon *> icons;
    ~GKSegments() override {
        for (HIMAGELIST list : images) {
            if (list) {
                gkComctl().ImageList_Destroy(list);
            }
        }
        for (GKIcon *icon : icons) {
            if (icon) {
                gkReleaseObject(icon);
            }
        }
    }
    /* applyIcon gives a segment its icon in the text color of the theme,
       with the padding of a button from the leading border. */
    void applyIcon(GKView *view, size_t i) {
        if (images[i]) {
            gkComctl().ImageList_Destroy(images[i]);
            images[i] = nullptr;
        }
        if (!icons[i]) {
            return;
        }
        HWND button = buttons[i];
        double scale = gkScaleOf(button);
        int side = (int)lround(16 * scale);
        images[i] = gkComctl().ImageList_Create(side, side, ILC_COLOR32, 1, 0);
        HBITMAP bitmap = gkIconBitmap(icons[i], side, gkViewPalette(view).text);
        gkComctl().ImageList_Add(images[i], bitmap, nullptr);
        DeleteObject(bitmap);
        BUTTON_IMAGELIST list = {};
        list.himl = images[i];
        if (titles[i].empty()) {
            list.uAlign = BUTTON_IMAGELIST_ALIGN_CENTER;
        } else {
            list.uAlign = BUTTON_IMAGELIST_ALIGN_LEFT;
            list.margin = {(int)lround(8 * scale), 0, (int)lround(2 * scale), 0};
        }
        SendMessageW(button, BCM_SETIMAGELIST, 0, (LPARAM)&list);
        InvalidateRect(button, nullptr, TRUE);
    }
    void themeChanged(GKView *view) override {
        for (size_t i = 0; i < buttons.size(); i++) {
            gkApplyThemeTo(buttons[i], gkViewTheme(view));
            applyIcon(view, i);
        }
    }
    void layout(GKView *view) override {
        int count = (int)buttons.size();
        if (count == 0) {
            return;
        }
        double width = (double)view->width / count;
        for (int i = 0; i < count; i++) {
            gkPlaceWindow(buttons[i], floor(i * width), 0, floor((i + 1) * width) - floor(i * width), view->height);
        }
    }
    bool command(GKView *view, HWND from, int code) override {
        if (code != BN_CLICKED) {
            return false;
        }
        for (size_t i = 0; i < buttons.size(); i++) {
            if (buttons[i] != from) {
                continue;
            }
            clicked = (int)i;
            if (mode == 1) {
                for (size_t other = 0; other < buttons.size(); other++) {
                    SendMessageW(buttons[other], BM_SETCHECK, other == i ? BST_CHECKED : BST_UNCHECKED, 0);
                }
            }
            gkReport(view, GK_EVENT_SELECTION, 0, 0, nullptr);
            return true;
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        GKFont font = gkViewFont(view, gkBodyFont());
        float widest = 0;
        for (size_t i = 0; i < titles.size(); i++) {
            /* A button centers its text in its whole width, so an icon
               before the text has its room on both sides. */
            float text = (float)gkMeasureGDI(titles[i], font).cx;
            float width = !icons[i] ? text + 24 : titles[i].empty() ? 16 + 24 : text + 2 * (8 + 16 + 4);
            widest = std::max(widest, width);
        }
        widest = std::max(widest, 40.0f);
        return D2D1::SizeF(widest * buttons.size(), (float)(gkMeasureGDI(L"Ag", font).cy + 8));
    }
};

static GKSegments *gkSegmentsOf(GKView *view) {
    return view && view->kind == GK_KIND_SEGMENTED ? static_cast<GKSegments *>(view->ext.get()) : nullptr;
}

extern "C" void *gkWin32SegmentedCreate(uintptr_t window, const char *labels, size_t size, void **icons,
                                        const int *enabled, int count, int mode) {
    HWND host = gkLayoutNew(gkParking());
    GKView *view = gkViewNew(host, nullptr, GK_KIND_SEGMENTED, window, 0);
    GKSegments *segments = new GKSegments();
    segments->mode = mode;
    view->ext.reset(segments);
    auto titles = gkSplit(labels, size);
    for (int i = 0; i < count; i++) {
        std::wstring title = i < (int)titles.size() ? gkWide(titles[i].c_str()) : std::wstring();
        DWORD style = WS_TABSTOP | (i == 0 ? WS_GROUP : 0);
        style |= mode == 0 ? BS_PUSHBUTTON : mode == 1 ? (BS_RADIOBUTTON | BS_PUSHLIKE) : (BS_AUTOCHECKBOX | BS_PUSHLIKE);
        HWND button = gkMakeNative(WC_BUTTONW, title.c_str(), style, 0, host);
        SetPropW(button, L"GKView", view);
        SendMessageW(button, WM_SETFONT, (WPARAM)gkFontHandle(gkBodyFont(), GetDpiForWindow(button)), FALSE);
        gkApplyThemeTo(button, gkViewTheme(view));
        EnableWindow(button, enabled[i] ? TRUE : FALSE);
        GKIcon *icon = static_cast<GKIcon *>(icons[i]);
        if (icon) {
            gkRetainObject(icon);
        }
        segments->buttons.push_back(button);
        segments->titles.push_back(title);
        segments->images.push_back(nullptr);
        segments->icons.push_back(icon);
        segments->applyIcon(view, segments->buttons.size() - 1);
    }
    return view;
}

std::vector<std::pair<std::string, std::string>> gkSegmentItems(GKView *view) {
    std::vector<std::pair<std::string, std::string>> items;
    if (GKSegments *segments = gkSegmentsOf(view)) {
        for (auto const &title : segments->titles) {
            items.emplace_back(segments->mode == 0 ? "Button" : "RadioButton", gkUTF8(title));
        }
    }
    return items;
}

extern "C" int gkWin32SegmentSelected(void *pointer, int index) {
    GKSegments *segments = gkSegmentsOf(gkView(pointer));
    if (!segments || index < 0 || index >= (int)segments->buttons.size() || segments->mode == 0) {
        return 0;
    }
    return SendMessageW(segments->buttons[index], BM_GETCHECK, 0, 0) == BST_CHECKED;
}

extern "C" int gkWin32SegmentClicked(void *pointer) {
    GKSegments *segments = gkSegmentsOf(gkView(pointer));
    return segments ? segments->clicked : -1;
}

extern "C" void gkWin32SegmentSetSelected(void *pointer, int index, int selected) {
    GKView *view = gkView(pointer);
    GKSegments *segments = gkSegmentsOf(view);
    if (!segments || index < 0 || index >= (int)segments->buttons.size() || segments->mode == 0) {
        return;
    }
    view->suppress++;
    SendMessageW(segments->buttons[index], BM_SETCHECK, selected ? BST_CHECKED : BST_UNCHECKED, 0);
    view->suppress--;
}

/* ---- Value controls ----

   A stepper, a dial, and a range slider keep their values in GKValue. */

struct GKValue : GKExtension {
    double minimum = 0, maximum = 1, value = 0, upper = 0, step = 0;
    int wraps = 0;
    double snap(double next) {
        if (step > 0) {
            next = minimum + round((next - minimum) / step) * step;
        }
        return gkClampValue(next, minimum, maximum);
    }
    void report(GKView *view) {
        view->value = value;
        char text[96];
        if (view->kind == GK_KIND_RANGE) {
            snprintf(text, sizeof(text), "%g to %g", value, upper);
        } else {
            snprintf(text, sizeof(text), "%g", value);
        }
        view->accessibleValue = text;
        InvalidateRect(view->outer, nullptr, FALSE);
        gkReport(view, GK_EVENT_VALUE, value, upper, nullptr);
    }
};

static GKValue *gkValueOf(GKView *view) {
    return view ? dynamic_cast<GKValue *>(view->ext.get()) : nullptr;
}

/* GKStepper is an up-down control without a text box. */
struct GKStepper : GKValue {
    void stepBy(GKView *view, double direction) {
        double amount = step > 0 ? step : 1;
        double next = value + direction * amount;
        if (next > maximum) {
            next = wraps ? minimum : maximum;
        } else if (next < minimum) {
            next = wraps ? maximum : minimum;
        }
        if (next != value) {
            value = next;
            report(view);
        }
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->code == UDN_DELTAPOS) {
            NMUPDOWN *change = reinterpret_cast<NMUPDOWN *>(header);
            stepBy(view, change->iDelta > 0 ? -1 : 1);
            *result = 1;
            return true;
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(18, (float)(gkMeasureGDI(L"Ag", gkBodyFont()).cy + 8));
    }
};

extern "C" void *gkWin32StepperCreate(uintptr_t window, double minimum, double maximum, double value, double step,
                                      int wraps) {
    GKStepper *state = new GKStepper();
    state->minimum = minimum;
    state->maximum = maximum > minimum ? maximum : minimum;
    state->value = gkClampValue(value, state->minimum, state->maximum);
    state->step = step;
    state->wraps = wraps != 0;
    GKView *view = gkViewNative(window, UPDOWN_CLASSW, L"", UDS_ARROWKEYS | UDS_HOTTRACK | WS_TABSTOP,
                                GK_KIND_STEPPER, state);
    if (view) {
        SendMessageW(view->outer, UDM_SETRANGE32, (WPARAM)-1000000, 1000000);
        view->value = state->value;
    }
    return view;
}

/* GKDial is a knob that Windows has no control for. A dial is at its
   minimum at the lower left and turns clockwise through 300 degrees to the
   lower right. A dial that wraps is at its minimum at the top and turns
   through a full circle. */
struct GKDial : GKValue {
    double start = 120 * M_PI / 180;
    double sweep = 300 * M_PI / 180;
    int dragging = 0;
    void paint(GKView *view, GKCanvas &canvas) override {
        GKPalette const &palette = gkViewPalette(view);
        float radius = std::min(canvas.width, canvas.height) / 2 - 3;
        if (radius <= 0) {
            return;
        }
        float cx = canvas.width / 2, cy = canvas.height / 2;
        canvas.strokeEllipse(cx, cy, radius, radius, palette.controlStroke, 2);
        double span = maximum - minimum;
        double fraction = span > 0 ? (value - minimum) / span : 0;
        double angle = start + fraction * sweep;
        GKColor accent = view->enabled ? palette.accent : palette.accentDisabled;
        canvas.arc(cx, cy, radius, (float)start, (float)(fraction * sweep), accent, 2);
        canvas.line(cx, cy, cx + (float)cos(angle) * radius * 0.8f, cy + (float)sin(angle) * radius * 0.8f,
                    view->enabled ? palette.text : palette.textDisabled, 2.5f);
        if (view->focused && view->keyboardFocus) {
            canvas.strokeEllipse(cx, cy, radius + 2, radius + 2, palette.focus, 1.5f);
        }
    }
    void point(GKView *view, int x, int y) {
        double scale = gkViewScaleOf(view);
        D2D1_SIZE_F size = gkClientSize(view->outer);
        double angle = atan2(y / scale - size.height / 2, x / scale - size.width / 2);
        double offset = fmod(angle - start + 4 * M_PI, 2 * M_PI);
        if (offset > sweep) {
            offset = offset - sweep < (2 * M_PI - sweep) / 2 ? sweep : 0;
        }
        double next = snap(minimum + offset / sweep * (maximum - minimum));
        if (wraps && next >= maximum) {
            /* The top of a full circle is the minimum. */
            next = minimum;
        }
        if (next != value) {
            value = next;
            report(view);
        }
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)hwnd;
        switch (message) {
        case WM_LBUTTONDOWN:
            SetFocus(view->outer);
            SetCapture(view->outer);
            dragging = 1;
            point(view, GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam));
            *result = 0;
            return true;
        case WM_MOUSEMOVE:
            if (dragging) {
                point(view, GET_X_LPARAM(lparam), GET_Y_LPARAM(lparam));
            }
            return false;
        case WM_LBUTTONUP:
        case WM_CAPTURECHANGED:
            if (dragging) {
                dragging = 0;
                if (message == WM_LBUTTONUP) {
                    ReleaseCapture();
                }
            }
            return false;
        case WM_KEYDOWN: {
            double amount = step > 0 ? step : (maximum - minimum) / 100;
            double next = value;
            if (wparam == VK_RIGHT || wparam == VK_UP) {
                next += amount;
            } else if (wparam == VK_LEFT || wparam == VK_DOWN) {
                next -= amount;
            } else if (wparam == VK_HOME) {
                next = minimum;
            } else if (wparam == VK_END) {
                next = maximum;
            } else {
                return false;
            }
            if (wraps && next > maximum) {
                next = minimum + (next - maximum);
            } else if (wraps && next < minimum) {
                next = maximum - (minimum - next);
            }
            next = snap(next);
            if (next != value) {
                value = next;
                report(view);
            }
            *result = 0;
            return true;
        }
        default:
            return false;
        }
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(48, 48);
    }
};

extern "C" void *gkWin32DialCreate(uintptr_t window, double minimum, double maximum, double value, double step,
                                   int wraps) {
    GKDial *dial = new GKDial();
    dial->minimum = minimum;
    dial->maximum = maximum > minimum ? maximum : minimum;
    dial->value = gkClampValue(value, dial->minimum, dial->maximum);
    dial->step = step;
    dial->wraps = wraps != 0;
    if (dial->wraps) {
        dial->start = -M_PI / 2;
        dial->sweep = 2 * M_PI;
    }
    GKView *view = gkCreateDrawn(window, GK_KIND_DIAL, dial, 0, 1);
    view->value = dial->value;
    return view;
}

extern "C" double gkWin32ControlValue(void *pointer) {
    GKValue *state = gkValueOf(gkView(pointer));
    return state ? state->value : 0;
}

extern "C" void gkWin32ControlSetValue(void *pointer, double value) {
    GKView *view = gkView(pointer);
    GKValue *state = gkValueOf(view);
    if (!state) {
        return;
    }
    state->value = gkClampValue(value, state->minimum, state->maximum);
    view->value = state->value;
    InvalidateRect(view->outer, nullptr, FALSE);
}

/* GKRange is a slider with two thumbs, which Windows has no control for.
   It draws the track and the thumbs of the trackbar of Windows. */
struct GKRange : GKValue {
    int dragging = -1;
    int focusedKnob = 0;
    static const int inset = 8;
    double position(double width, double v) {
        double span = maximum - minimum;
        double fraction = span > 0 ? (v - minimum) / span : 0;
        double track = width - inset * 2;
        return inset + fraction * (track > 0 ? track : 0);
    }
    double valueAt(double width, double x) {
        double track = width - inset * 2;
        track = track > 1 ? track : 1;
        double offset = gkClampValue(x - inset, 0, track);
        return snap(minimum + offset / track * (maximum - minimum));
    }
    bool paintGDI(GKView *view, HWND hwnd, HDC hdc, RECT const &client) override {
        double scale = gkViewScaleOf(view);
        double width = client.right / scale;
        int middle = client.bottom / 2;
        int lower = (int)lround(position(width, value) * scale), higher = (int)lround(position(width, upper) * scale);
        int pad = (int)lround(inset * scale);
        RECT track = {pad, middle - (int)lround(2 * scale), client.right - pad, middle + (int)lround(2 * scale)};
        int themeIndex = gkViewTheme(view);
        GKPalette const &palette = gkPalette(themeIndex);
        HTHEME theme = themeIndex == 2 ? nullptr : OpenThemeData(hwnd, L"Trackbar");
        auto fill = [&](RECT const &rect, GKColor const &color) {
            HBRUSH brush = CreateSolidBrush(RGB((int)(color.r * 255), (int)(color.g * 255), (int)(color.b * 255)));
            FillRect(hdc, &rect, brush);
            DeleteObject(brush);
        };
        if (theme) {
            DrawThemeBackground(theme, hdc, TKP_TRACK, TRS_NORMAL, &track, nullptr);
        } else {
            fill(track, palette.controlStrong);
        }
        RECT span = {lower, track.top + 1, higher, track.bottom - 1};
        fill(span, view->enabled ? palette.accent : palette.accentDisabled);
        int thumbWidth = (int)lround(11 * scale), thumbHeight = (int)lround(20 * scale);
        for (int knob = 0; knob < 2; knob++) {
            int x = knob == 0 ? lower : higher;
            RECT thumb = {x - thumbWidth / 2, middle - thumbHeight / 2, x - thumbWidth / 2 + thumbWidth,
                          middle - thumbHeight / 2 + thumbHeight};
            int state = !view->enabled                ? TUS_DISABLED
                        : dragging == knob            ? TUS_PRESSED
                        : view->focused && focusedKnob == knob ? TUS_FOCUSED
                                                               : TUS_NORMAL;
            if (theme) {
                DrawThemeBackground(theme, hdc, TKP_THUMB, state, &thumb, nullptr);
            } else {
                fill(thumb, state == TUS_PRESSED ? palette.accentPressed : palette.accent);
            }
            if (view->focused && focusedKnob == knob && view->keyboardFocus) {
                RECT focus = thumb;
                InflateRect(&focus, (int)lround(2 * scale), (int)lround(2 * scale));
                DrawFocusRect(hdc, &focus);
            }
        }
        if (theme) {
            CloseThemeData(theme);
        }
        return true;
    }
    void move(GKView *view, int x) {
        double scale = gkViewScaleOf(view);
        double width = gkClientSize(view->outer).width;
        double next = valueAt(width, x / scale);
        if (dragging == 0) {
            next = std::min(next, upper);
            if (next == value) {
                return;
            }
            value = next;
        } else {
            next = std::max(next, value);
            if (next == upper) {
                return;
            }
            upper = next;
        }
        report(view);
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)hwnd;
        switch (message) {
        case WM_LBUTTONDOWN: {
            SetFocus(view->outer);
            SetCapture(view->outer);
            double scale = gkViewScaleOf(view);
            double width = gkClientSize(view->outer).width;
            double x = GET_X_LPARAM(lparam) / scale;
            double lowerX = position(width, value), upperX = position(width, upper);
            dragging = fabs(x - lowerX) <= fabs(x - upperX) && !(x > upperX && lowerX == upperX) ? 0 : 1;
            focusedKnob = dragging;
            move(view, GET_X_LPARAM(lparam));
            InvalidateRect(view->outer, nullptr, FALSE);
            *result = 0;
            return true;
        }
        case WM_MOUSEMOVE:
            if (dragging >= 0) {
                move(view, GET_X_LPARAM(lparam));
            }
            return false;
        case WM_LBUTTONUP:
        case WM_CAPTURECHANGED:
            if (dragging >= 0) {
                dragging = -1;
                if (message == WM_LBUTTONUP) {
                    ReleaseCapture();
                }
                InvalidateRect(view->outer, nullptr, FALSE);
            }
            return false;
        case WM_GETDLGCODE:
            *result = DLGC_WANTARROWS;
            return true;
        case WM_KEYDOWN: {
            double amount = step > 0 ? step : (maximum - minimum) / 100;
            if (wparam == VK_TAB) {
                return false;
            }
            double delta = wparam == VK_RIGHT || wparam == VK_UP ? amount
                           : wparam == VK_LEFT || wparam == VK_DOWN ? -amount
                                                                    : 0;
            if (wparam == VK_SPACE) {
                focusedKnob = 1 - focusedKnob;
                InvalidateRect(view->outer, nullptr, FALSE);
                *result = 0;
                return true;
            }
            if (delta == 0) {
                return false;
            }
            if (focusedKnob == 0) {
                value = std::min(snap(value + delta), upper);
            } else {
                upper = std::max(snap(upper + delta), value);
            }
            report(view);
            *result = 0;
            return true;
        }
        default:
            return false;
        }
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(160, 26);
    }
};

extern "C" void *gkWin32RangeSliderCreate(uintptr_t window, double minimum, double maximum, double lower,
                                          double upper, double step) {
    GKRange *range = new GKRange();
    range->minimum = minimum;
    range->maximum = maximum > minimum ? maximum : minimum;
    range->step = step;
    range->value = gkClampValue(lower, range->minimum, range->maximum);
    range->upper = gkClampValue(upper, range->value, range->maximum);
    return gkCreateDrawn(window, GK_KIND_RANGE, range, 0, 1);
}

extern "C" void gkWin32RangeSliderValues(void *pointer, double *lower, double *upper) {
    GKValue *state = gkValueOf(gkView(pointer));
    *lower = state ? state->value : 0;
    *upper = state ? state->upper : 0;
}

extern "C" void gkWin32RangeSliderSet(void *pointer, double lower, double upper) {
    GKView *view = gkView(pointer);
    GKValue *state = gkValueOf(view);
    if (!state) {
        return;
    }
    state->value = lower;
    state->upper = upper;
    InvalidateRect(view->outer, nullptr, FALSE);
}

/* ---- Dates and times ----

   Dates are exchanged as seconds since 1970 in UTC, and pickers show them as
   the same calendar date, so a date never shifts with the time zone. */

enum {
    GK_ELEMENTS_TIME = 0x000c,
    GK_ELEMENTS_SECONDS = 0x0002,
    GK_ELEMENTS_DATE = 0x00e0,
};

static SYSTEMTIME gkSystemDate(double seconds) {
    time_t value = (time_t)floor(seconds);
    struct tm utc = {};
    gmtime_s(&utc, &value);
    SYSTEMTIME date = {};
    date.wYear = (WORD)(utc.tm_year + 1900);
    date.wMonth = (WORD)(utc.tm_mon + 1);
    date.wDay = (WORD)utc.tm_mday;
    date.wDayOfWeek = (WORD)utc.tm_wday;
    date.wHour = 12;
    return date;
}

static double gkSecondsOf(SYSTEMTIME const &date) {
    struct tm utc = {};
    utc.tm_year = date.wYear - 1900;
    utc.tm_mon = date.wMonth - 1;
    utc.tm_mday = date.wDay;
    return (double)_mkgmtime(&utc);
}

static int gkDayNumber(double seconds) {
    return (int)floor(seconds / 86400);
}

static int gkLocaleUses12Hour() {
    wchar_t format[80] = {};
    GetLocaleInfoEx(LOCALE_NAME_USER_DEFAULT, LOCALE_STIMEFORMAT, format, 80);
    return wcschr(format, L'h') != nullptr;
}

struct GKDate : GKExtension {
    int elements = 0;
    double value = 0;
    double minimum = 0, maximum = 0;
    int clock = 0;
    int calendar = 0;
    double clamp(double v) {
        if (minimum != 0 && v < minimum) {
            v = minimum;
        }
        if (maximum != 0 && v > maximum) {
            v = maximum;
        }
        return v;
    }
    int timeOnly() {
        return !(elements & GK_ELEMENTS_DATE);
    }
    void show(GKView *view) {
        view->suppress++;
        if (timeOnly()) {
            int seconds = (int)fmod(fmod(floor(value), 86400) + 86400, 86400);
            SYSTEMTIME time = {};
            GetLocalTime(&time);
            time.wHour = (WORD)(seconds / 3600);
            time.wMinute = (WORD)((seconds / 60) % 60);
            time.wSecond = (WORD)(seconds % 60);
            time.wMilliseconds = 0;
            SendMessageW(view->outer, DTM_SETSYSTEMTIME, GDT_VALID, (LPARAM)&time);
        } else if (calendar) {
            SYSTEMTIME date = gkSystemDate(value);
            SendMessageW(view->outer, MCM_SETCURSEL, 0, (LPARAM)&date);
        } else {
            SYSTEMTIME date = gkSystemDate(value);
            SendMessageW(view->outer, DTM_SETSYSTEMTIME, GDT_VALID, (LPARAM)&date);
        }
        view->suppress--;
    }
    void changed(GKView *view, SYSTEMTIME const &time) {
        if (view->suppress) {
            return;
        }
        double next;
        if (timeOnly()) {
            next = time.wHour * 3600 + time.wMinute * 60 + ((elements & GK_ELEMENTS_SECONDS) ? time.wSecond : 0);
        } else {
            next = clamp(gkSecondsOf(time));
        }
        value = next;
        gkReport(view, GK_EVENT_VALUE, next, 0, nullptr);
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->code == DTN_DATETIMECHANGE) {
            NMDATETIMECHANGE *change = reinterpret_cast<NMDATETIMECHANGE *>(header);
            if (change->dwFlags == GDT_VALID) {
                changed(view, change->st);
            }
            *result = 0;
            return true;
        }
        if (header->code == MCN_SELCHANGE) {
            NMSELCHANGE *change = reinterpret_cast<NMSELCHANGE *>(header);
            changed(view, change->stSelStart);
            *result = 0;
            return true;
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        if (calendar) {
            RECT rect = {};
            SendMessageW(view->outer, MCM_GETMINREQRECT, 0, (LPARAM)&rect);
            double scale = gkViewScaleOf(view);
            return D2D1::SizeF((float)(rect.right / scale), (float)(rect.bottom / scale));
        }
        return D2D1::SizeF(timeOnly() ? 110.0f : 130.0f, (float)(gkMeasureGDI(L"Ag", gkBodyFont()).cy + 8));
    }
};

static GKDate *gkDateOf(GKView *view) {
    return view && view->kind == GK_KIND_DATE ? static_cast<GKDate *>(view->ext.get()) : nullptr;
}

extern "C" void *gkWin32DatePickerCreate(uintptr_t window, int style, int elements, double value, double minimum,
                                         double maximum, int range, double interval, int clock) {
    (void)range;
    (void)interval;
    GKDate *date = new GKDate();
    date->elements = elements;
    date->value = value;
    date->minimum = minimum;
    date->maximum = maximum;
    date->clock = clock;
    GKView *view;
    if ((elements & GK_ELEMENTS_DATE) && style == 1) {
        date->calendar = 1;
        view = gkViewNative(window, MONTHCAL_CLASSW, L"", WS_TABSTOP | MCS_NOTODAYCIRCLE, GK_KIND_DATE, date);
    } else if (elements & GK_ELEMENTS_DATE) {
        view = gkViewNative(window, DATETIMEPICK_CLASSW, L"", WS_TABSTOP | DTS_SHORTDATECENTURYFORMAT, GK_KIND_DATE,
                            date);
    } else {
        view = gkViewNative(window, DATETIMEPICK_CLASSW, L"", WS_TABSTOP | DTS_TIMEFORMAT | DTS_UPDOWN, GK_KIND_DATE,
                            date);
        if (view) {
            int twelve = clock == 1 || (clock == 0 && gkLocaleUses12Hour());
            const wchar_t *format = (elements & GK_ELEMENTS_SECONDS) ? (twelve ? L"h:mm:ss tt" : L"HH:mm:ss")
                                                                    : (twelve ? L"h:mm tt" : L"HH:mm");
            SendMessageW(view->outer, DTM_SETFORMATW, 0, (LPARAM)format);
        }
    }
    if (!view) {
        return nullptr;
    }
    if (!date->timeOnly() && (minimum != 0 || maximum != 0)) {
        SYSTEMTIME limits[2] = {gkSystemDate(minimum), gkSystemDate(maximum)};
        DWORD flags = (minimum != 0 ? GDTR_MIN : 0) | (maximum != 0 ? GDTR_MAX : 0);
        SendMessageW(view->outer, date->calendar ? MCM_SETRANGE : DTM_SETRANGE, flags, (LPARAM)limits);
    }
    date->show(view);
    return view;
}

extern "C" void gkWin32DatePickerValue(void *pointer, double *value, double *interval) {
    GKDate *date = gkDateOf(gkView(pointer));
    *value = date ? date->value : 0;
    *interval = 0;
}

extern "C" void gkWin32DatePickerSet(void *pointer, double value, double interval) {
    (void)interval;
    GKView *view = gkView(pointer);
    GKDate *date = gkDateOf(view);
    if (date) {
        date->value = value;
        date->show(view);
    }
}

/* ---- Calendar ----

   The month calendar of Windows selects one date or a range. A set of dates
   is shown in bold, and a click adds or removes a date. */

struct GKCalendar : GKExtension {
    int selection = 0;
    std::vector<double> dates;
    int anchor = 1;
    void apply(GKView *view) {
        view->suppress++;
        if (selection == 2 && !dates.empty()) {
            SYSTEMTIME range[2] = {gkSystemDate(dates.front()), gkSystemDate(dates.size() > 1 ? dates[1] : dates[0])};
            SendMessageW(view->outer, MCM_SETSELRANGE, 0, (LPARAM)range);
        } else if (selection == 0 && !dates.empty()) {
            SYSTEMTIME date = gkSystemDate(dates[0]);
            SendMessageW(view->outer, MCM_SETCURSEL, 0, (LPARAM)&date);
        }
        InvalidateRect(view->outer, nullptr, TRUE);
        view->suppress--;
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override {
        if (header->code == MCN_GETDAYSTATE && selection == 1) {
            NMDAYSTATE *state = reinterpret_cast<NMDAYSTATE *>(header);
            static MONTHDAYSTATE months[16];
            int count = std::min(state->cDayState, 16);
            SYSTEMTIME month = state->stStart;
            for (int i = 0; i < count; i++) {
                months[i] = 0;
                for (double date : dates) {
                    SYSTEMTIME day = gkSystemDate(date);
                    if (day.wYear == month.wYear && day.wMonth == month.wMonth) {
                        months[i] |= 1u << (day.wDay - 1);
                    }
                }
                month.wMonth++;
                if (month.wMonth > 12) {
                    month.wMonth = 1;
                    month.wYear++;
                }
            }
            state->prgDayState = months;
            *result = 0;
            return true;
        }
        if (header->code == MCN_SELECT && !view->suppress) {
            NMSELCHANGE *change = reinterpret_cast<NMSELCHANGE *>(header);
            double seconds = gkSecondsOf(change->stSelStart);
            if (selection == 0) {
                dates.assign(1, seconds);
            } else if (selection == 2) {
                double last = gkSecondsOf(change->stSelEnd);
                dates.assign(1, seconds);
                if (gkDayNumber(last) != gkDayNumber(seconds)) {
                    dates.push_back(last);
                }
            } else {
                auto found = std::find_if(dates.begin(), dates.end(),
                                          [seconds](double date) { return gkDayNumber(date) == gkDayNumber(seconds); });
                if (found == dates.end()) {
                    dates.push_back(seconds);
                    std::sort(dates.begin(), dates.end());
                } else {
                    dates.erase(found);
                }
                refreshBold(view);
            }
            gkReport(view, GK_EVENT_VALUE, seconds, 0, nullptr);
            *result = 0;
            return true;
        }
        return false;
    }
    void refreshBold(GKView *view) {
        if (selection != 1) {
            return;
        }
        SYSTEMTIME range[2];
        int count = (int)SendMessageW(view->outer, MCM_GETMONTHRANGE, GMR_DAYSTATE, (LPARAM)range);
        std::vector<MONTHDAYSTATE> months((size_t)std::max(count, 1), 0);
        SYSTEMTIME month = range[0];
        for (int i = 0; i < count; i++) {
            for (double date : dates) {
                SYSTEMTIME day = gkSystemDate(date);
                if (day.wYear == month.wYear && day.wMonth == month.wMonth) {
                    months[i] |= 1u << (day.wDay - 1);
                }
            }
            month.wMonth++;
            if (month.wMonth > 12) {
                month.wMonth = 1;
                month.wYear++;
            }
        }
        SendMessageW(view->outer, MCM_SETDAYSTATE, count, (LPARAM)months.data());
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        RECT rect = {};
        SendMessageW(view->outer, MCM_GETMINREQRECT, 0, (LPARAM)&rect);
        double scale = gkViewScaleOf(view);
        return D2D1::SizeF((float)(rect.right / scale), (float)(rect.bottom / scale));
    }
};

static GKCalendar *gkCalendarOf(GKView *view) {
    return view && view->kind == GK_KIND_CALENDAR ? static_cast<GKCalendar *>(view->ext.get()) : nullptr;
}

static double gkMonthStart(double seconds) {
    time_t value = (time_t)floor(seconds);
    struct tm utc = {};
    gmtime_s(&utc, &value);
    utc.tm_mday = 1;
    utc.tm_hour = utc.tm_min = utc.tm_sec = 0;
    return (double)_mkgmtime(&utc);
}

/* gkCalendarVisible returns the first month that a calendar shows. */
static double gkCalendarVisible(GKView *view) {
    SYSTEMTIME range[2] = {};
    SendMessageW(view->outer, MCM_GETMONTHRANGE, GMR_VISIBLE, (LPARAM)range);
    range[0].wDay = 1;
    return gkSecondsOf(range[0]);
}

/* gkCalendarShowMonth turns the pages of a calendar to a month with its
   next and previous buttons, which do not change the selection. */
static void gkCalendarShowMonth(GKView *view, double seconds) {
    double target = gkMonthStart(seconds);
    for (int attempt = 0; attempt < 2400; attempt++) {
        double visible = gkCalendarVisible(view);
        if (gkDayNumber(visible) == gkDayNumber(target)) {
            break;
        }
        MCGRIDINFO info = {};
        info.cbSize = sizeof(info);
        info.dwPart = visible < target ? MCGIP_NEXT : MCGIP_PREV;
        info.dwFlags = MCGIF_RECT;
        if (!SendMessageW(view->outer, MCM_GETCALENDARGRIDINFO, 0, (LPARAM)&info)) {
            break;
        }
        LPARAM point = MAKELPARAM((info.rc.left + info.rc.right) / 2, (info.rc.top + info.rc.bottom) / 2);
        view->suppress++;
        SendMessageW(view->outer, WM_LBUTTONDOWN, MK_LBUTTON, point);
        SendMessageW(view->outer, WM_LBUTTONUP, 0, point);
        view->suppress--;
        if (gkDayNumber(gkCalendarVisible(view)) == gkDayNumber(visible)) {
            break;
        }
    }
}

extern "C" void gkWin32CalendarSetDates(void *pointer, const double *dates, int count);
extern "C" void gkWin32CalendarSetVisibleMonth(void *pointer, double month);

extern "C" void *gkWin32CalendarCreate(uintptr_t window, int selection, const double *dates, int count,
                                       double visible) {
    GKCalendar *state = new GKCalendar();
    state->selection = selection;
    DWORD style = WS_TABSTOP | MCS_NOTODAYCIRCLE | (selection == 2 ? MCS_MULTISELECT : 0) |
                  (selection == 1 ? MCS_DAYSTATE : 0);
    GKView *view = gkViewNative(window, MONTHCAL_CLASSW, L"", style, GK_KIND_CALENDAR, state);
    if (!view) {
        return nullptr;
    }
    if (selection == 2) {
        SendMessageW(view->outer, MCM_SETMAXSELCOUNT, 3660, 0);
    }
    gkWin32CalendarSetDates(view, dates, count);
    if (visible != 0) {
        gkWin32CalendarSetVisibleMonth(view, visible);
    }
    return view;
}

extern "C" int gkWin32CalendarDates(void *pointer, double *dates, int capacity) {
    GKCalendar *state = gkCalendarOf(gkView(pointer));
    if (!state) {
        return 0;
    }
    int count = (int)state->dates.size();
    for (int i = 0; i < count && i < capacity; i++) {
        dates[i] = state->dates[i];
    }
    return count;
}

extern "C" void gkWin32CalendarSetDates(void *pointer, const double *dates, int count) {
    GKView *view = gkView(pointer);
    GKCalendar *state = gkCalendarOf(view);
    if (!state) {
        return;
    }
    state->dates.assign(dates, dates + (count > 0 ? count : 0));
    std::sort(state->dates.begin(), state->dates.end());
    state->anchor = 1;
    state->apply(view);
    state->refreshBold(view);
    if (count > 0) {
        gkCalendarShowMonth(view, dates[0]);
    }
}

extern "C" double gkWin32CalendarVisibleMonth(void *pointer) {
    GKView *view = gkView(pointer);
    return gkCalendarOf(view) ? gkCalendarVisible(view) : 0;
}

extern "C" void gkWin32CalendarSetVisibleMonth(void *pointer, double month) {
    GKView *view = gkView(pointer);
    if (gkCalendarOf(view)) {
        gkCalendarShowMonth(view, month);
        static_cast<GKCalendar *>(view->ext.get())->refreshBold(view);
    }
}

/* ---- Color ----

   A color well is a button that shows the color and opens the color dialog
   of Windows. The dialog has no opacity, so for a color well that allows
   it, the dialog gets an opacity row: a trackbar and its value. */

struct GKOpacity {
    double alpha = 1;
    HWND slider = nullptr;
    HWND value = nullptr;
};

static void gkOpacityShow(GKOpacity *opacity) {
    wchar_t text[16];
    swprintf(text, 16, L"%d%%", (int)lround(opacity->alpha * 100));
    SetWindowTextW(opacity->value, text);
}

static UINT_PTR CALLBACK gkOpacityHook(HWND dialog, UINT message, WPARAM wparam, LPARAM lparam) {
    if (message == WM_INITDIALOG) {
        CHOOSECOLORW *choose = reinterpret_cast<CHOOSECOLORW *>(lparam);
        GKOpacity *opacity = reinterpret_cast<GKOpacity *>(choose->lCustData);
        SetPropW(dialog, L"GKOpacity", opacity);
        HFONT font = (HFONT)SendMessageW(dialog, WM_GETFONT, 0, 0);
        RECT client;
        GetClientRect(dialog, &client);
        /* The row goes below the controls of the dialog, which grows to hold
           it, with the margins of the dialog. */
        RECT units = {7, 4, 40, 14};
        MapDialogRect(dialog, &units);
        int margin = units.left, gap = units.top, labelWidth = units.right, row = units.bottom + gap;
        RECT frame;
        GetWindowRect(dialog, &frame);
        SetWindowPos(dialog, nullptr, 0, 0, frame.right - frame.left, frame.bottom - frame.top + row + gap,
                     SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE);
        int top = client.bottom + gap / 2;
        HWND label = CreateWindowExW(0, WC_STATICW, L"&Opacity:", WS_CHILD | WS_VISIBLE | SS_LEFT | SS_CENTERIMAGE,
                                     margin, top, labelWidth, units.bottom, dialog, nullptr, gkInstance(), nullptr);
        int valueWidth = labelWidth;
        opacity->slider = CreateWindowExW(0, TRACKBAR_CLASSW, L"Opacity",
                                          WS_CHILD | WS_VISIBLE | WS_TABSTOP | TBS_HORZ | TBS_NOTICKS,
                                          margin + labelWidth, top, client.right - 2 * margin - labelWidth - valueWidth,
                                          units.bottom, dialog, nullptr, gkInstance(), nullptr);
        opacity->value = CreateWindowExW(0, WC_STATICW, L"", WS_CHILD | WS_VISIBLE | SS_RIGHT | SS_CENTERIMAGE,
                                         client.right - margin - valueWidth, top, valueWidth, units.bottom, dialog,
                                         nullptr, gkInstance(), nullptr);
        SendMessageW(label, WM_SETFONT, (WPARAM)font, FALSE);
        SendMessageW(opacity->value, WM_SETFONT, (WPARAM)font, FALSE);
        SendMessageW(opacity->slider, TBM_SETRANGE, FALSE, MAKELPARAM(0, 100));
        SendMessageW(opacity->slider, TBM_SETPAGESIZE, 0, 10);
        SendMessageW(opacity->slider, TBM_SETPOS, TRUE, (LPARAM)lround(opacity->alpha * 100));
        gkOpacityShow(opacity);
        return FALSE;
    }
    if (message == WM_HSCROLL) {
        GKOpacity *opacity = static_cast<GKOpacity *>(GetPropW(dialog, L"GKOpacity"));
        if (opacity && (HWND)lparam == opacity->slider) {
            opacity->alpha = SendMessageW(opacity->slider, TBM_GETPOS, 0, 0) / 100.0;
            gkOpacityShow(opacity);
            return TRUE;
        }
    }
    if (message == WM_DESTROY) {
        RemovePropW(dialog, L"GKOpacity");
    }
    (void)wparam;
    return FALSE;
}

struct GKColorWell : GKExtension {
    double rgba[4] = {0, 0, 0, 1};
    int allowsAlpha = 0;
    HIMAGELIST images = nullptr;
    COLORREF custom[16] = {};
    ~GKColorWell() override {
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
    }
    void show(GKView *view) {
        double scale = gkViewScaleOf(view);
        int width = (int)lround(32 * scale), height = (int)lround(14 * scale);
        if (images) {
            gkComctl().ImageList_Destroy(images);
        }
        images = gkComctl().ImageList_Create(width, height, ILC_COLOR32, 1, 0);
        std::vector<unsigned char> pixels((size_t)width * height * 4);
        GKColor border = gkViewPalette(view).controlStrong;
        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                bool edge = x == 0 || y == 0 || x == width - 1 || y == height - 1;
                unsigned char *p = &pixels[((size_t)y * width + x) * 4];
                if (edge) {
                    p[0] = (unsigned char)(border.b * 255);
                    p[1] = (unsigned char)(border.g * 255);
                    p[2] = (unsigned char)(border.r * 255);
                    p[3] = 255;
                } else {
                    /* A translucent color shows over a checkerboard. */
                    double checker = ((x / 4 + y / 4) % 2) ? 0.8 : 1.0;
                    double a = rgba[3];
                    p[0] = (unsigned char)lround((rgba[2] * a + checker * (1 - a)) * 255);
                    p[1] = (unsigned char)lround((rgba[1] * a + checker * (1 - a)) * 255);
                    p[2] = (unsigned char)lround((rgba[0] * a + checker * (1 - a)) * 255);
                    p[3] = 255;
                }
            }
        }
        BITMAPINFO info = {};
        info.bmiHeader.biSize = sizeof(info.bmiHeader);
        info.bmiHeader.biWidth = width;
        info.bmiHeader.biHeight = -height;
        info.bmiHeader.biPlanes = 1;
        info.bmiHeader.biBitCount = 32;
        void *bits = nullptr;
        HDC screen = GetDC(nullptr);
        HBITMAP bitmap = CreateDIBSection(screen, &info, DIB_RGB_COLORS, &bits, nullptr, 0);
        ReleaseDC(nullptr, screen);
        if (bitmap) {
            memcpy(bits, pixels.data(), pixels.size());
            gkComctl().ImageList_Add(images, bitmap, nullptr);
            DeleteObject(bitmap);
        }
        BUTTON_IMAGELIST list = {};
        list.himl = images;
        list.uAlign = BUTTON_IMAGELIST_ALIGN_CENTER;
        SendMessageW(view->outer, BCM_SETIMAGELIST, 0, (LPARAM)&list);
        char text[32];
        snprintf(text, sizeof(text), "#%02X%02X%02X%02X", (int)lround(rgba[0] * 255), (int)lround(rgba[1] * 255),
                 (int)lround(rgba[2] * 255), (int)lround(rgba[3] * 255));
        view->accessibleValue = text;
        InvalidateRect(view->outer, nullptr, TRUE);
    }
    bool command(GKView *view, HWND from, int code) override {
        (void)from;
        if (code != BN_CLICKED) {
            return false;
        }
        CHOOSECOLORW choose = {};
        choose.lStructSize = sizeof(choose);
        choose.hwndOwner = GetAncestor(view->outer, GA_ROOT);
        choose.rgbResult = RGB((int)lround(rgba[0] * 255), (int)lround(rgba[1] * 255), (int)lround(rgba[2] * 255));
        choose.lpCustColors = custom;
        choose.Flags = CC_FULLOPEN | CC_RGBINIT | CC_ANYCOLOR;
        GKOpacity opacity;
        opacity.alpha = rgba[3];
        if (allowsAlpha) {
            choose.Flags |= CC_ENABLEHOOK;
            choose.lpfnHook = gkOpacityHook;
            choose.lCustData = reinterpret_cast<LPARAM>(&opacity);
        }
        GKActivation activation;
        if (ChooseColorW(&choose)) {
            rgba[0] = GetRValue(choose.rgbResult) / 255.0;
            rgba[1] = GetGValue(choose.rgbResult) / 255.0;
            rgba[2] = GetBValue(choose.rgbResult) / 255.0;
            if (allowsAlpha) {
                rgba[3] = opacity.alpha;
            }
            show(view);
            gkReport(view, GK_EVENT_VALUE, 0, 0, nullptr);
        }
        return true;
    }
    void themeChanged(GKView *view) override {
        show(view);
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(56, (float)(gkMeasureGDI(L"Ag", gkBodyFont()).cy + 8));
    }
};

extern "C" void *gkWin32ColorWellCreate(uintptr_t window, double r, double g, double b, double a, int allowsAlpha) {
    GKColorWell *well = new GKColorWell();
    well->rgba[0] = r;
    well->rgba[1] = g;
    well->rgba[2] = b;
    well->rgba[3] = allowsAlpha ? a : 1;
    well->allowsAlpha = allowsAlpha != 0;
    GKView *view = gkViewNative(window, WC_BUTTONW, L"", BS_PUSHBUTTON | WS_TABSTOP, GK_KIND_COLOR, well);
    if (view) {
        view->text = L"Color";
        well->show(view);
    }
    return view;
}

extern "C" void gkWin32ColorWellColor(void *pointer, double *rgba) {
    GKView *view = gkView(pointer);
    rgba[0] = rgba[1] = rgba[2] = 0;
    rgba[3] = 1;
    if (!view || view->kind != GK_KIND_COLOR) {
        return;
    }
    GKColorWell *well = static_cast<GKColorWell *>(view->ext.get());
    for (int i = 0; i < 4; i++) {
        rgba[i] = well->rgba[i];
    }
}

extern "C" void gkWin32ColorWellSet(void *pointer, double r, double g, double b, double a) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_COLOR) {
        return;
    }
    GKColorWell *well = static_cast<GKColorWell *>(view->ext.get());
    well->rgba[0] = r;
    well->rgba[1] = g;
    well->rgba[2] = b;
    well->rgba[3] = well->allowsAlpha ? a : 1;
    well->show(view);
}

/* ---- Font ----

   A font picker is a button that shows the font and opens the font dialog
   of Windows. */

struct GKFontPicker : GKExtension {
    std::wstring family;
    int system = 1;
    double size = 12;
    int weight = 400;
    int italic = 0;
    void show(GKView *view) {
        wchar_t text[160];
        swprintf(text, 160, L"%ls %g", system ? L"System" : family.c_str(), size);
        view->text = text;
        SetWindowTextW(view->outer, text);
        view->accessibleValue = gkUTF8(text);
    }
    bool command(GKView *view, HWND from, int code) override {
        (void)from;
        if (code != BN_CLICKED) {
            return false;
        }
        LOGFONTW font = {};
        HDC screen = GetDC(nullptr);
        font.lfHeight = -MulDiv((int)lround(size), GetDeviceCaps(screen, LOGPIXELSY), 72);
        ReleaseDC(nullptr, screen);
        font.lfWeight = weight;
        font.lfItalic = italic ? TRUE : FALSE;
        std::wstring name = system ? gkBodyFont().family : family;
        wcsncpy(font.lfFaceName, name.c_str(), LF_FACESIZE - 1);
        CHOOSEFONTW choose = {};
        choose.lStructSize = sizeof(choose);
        choose.hwndOwner = GetAncestor(view->outer, GA_ROOT);
        choose.lpLogFont = &font;
        choose.Flags = CF_SCREENFONTS | CF_INITTOLOGFONTSTRUCT | CF_NOVERTFONTS | CF_NOSCRIPTSEL;
        if (ChooseFontW(&choose)) {
            family = font.lfFaceName;
            system = 0;
            size = choose.iPointSize / 10.0;
            weight = font.lfWeight ? font.lfWeight : 400;
            italic = font.lfItalic != 0;
            show(view);
            gkReport(view, GK_EVENT_VALUE, 0, 0, nullptr);
        }
        return true;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        SIZE text = gkMeasureGDI(view->text, gkBodyFont());
        return D2D1::SizeF((float)text.cx + 24, (float)(gkMeasureGDI(L"Ag", gkBodyFont()).cy + 8));
    }
};

extern "C" void gkWin32FontPickerSet(void *pointer, const char *family, double size, int weight, int italic);

extern "C" void *gkWin32FontPickerCreate(uintptr_t window, const char *family, double size, int weight, int italic) {
    GKView *view = gkViewNative(window, WC_BUTTONW, L"", BS_PUSHBUTTON | WS_TABSTOP, GK_KIND_FONT, new GKFontPicker());
    if (view) {
        gkWin32FontPickerSet(view, family, size, weight, italic);
    }
    return view;
}

extern "C" void gkWin32FontPickerSet(void *pointer, const char *family, double size, int weight, int italic) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_FONT) {
        return;
    }
    std::string name = family ? family : "";
    if (!name.empty() && name != "System" && name != "Monospace" && !gkFontFamilyInstalled(family)) {
        return;
    }
    GKFontPicker *state = static_cast<GKFontPicker *>(view->ext.get());
    state->system = name.empty() || name == "System";
    state->family = state->system ? gkBodyFont().family : name == "Monospace" ? L"Consolas" : gkWide(family);
    state->size = size > 0 ? size : 12;
    state->weight = weight > 0 ? weight : 400;
    state->italic = italic != 0;
    state->show(view);
}

extern "C" char *gkWin32FontPickerFont(void *pointer, double *size, int *weight, int *italic) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_FONT) {
        *size = 0;
        *weight = 400;
        *italic = 0;
        return gkStrdup("System");
    }
    GKFontPicker *state = static_cast<GKFontPicker *>(view->ext.get());
    *size = state->size;
    *weight = state->weight;
    *italic = state->italic;
    if (state->system) {
        return gkStrdup("System");
    }
    return gkStrdupW(state->family);
}

/* ---- Key recorder ----

   A key recorder is a button that records the next shortcut that the user
   types after clicking it. */

struct GKRecorder : GKExtension {
    std::string display;
    int recording = 0;
    void update(GKView *view) {
        const char *text = recording ? "Type shortcut\xE2\x80\xA6"
                           : !display.empty() ? display.c_str()
                                              : "Record Shortcut";
        view->text = gkWide(text);
        SetWindowTextW(view->outer, view->text.c_str());
        view->accessibleName = view->accessibleName.empty() ? "Shortcut" : view->accessibleName;
        view->accessibleValue = display;
    }
    bool command(GKView *view, HWND from, int code) override {
        (void)from;
        if (code == BN_CLICKED) {
            recording = 1;
            SetFocus(view->outer);
            update(view);
            return true;
        }
        return false;
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)hwnd;
        if (message == WM_KILLFOCUS && recording) {
            recording = 0;
            update(view);
            return false;
        }
        if (message == WM_GETDLGCODE && recording) {
            *result = DLGC_WANTALLKEYS;
            return true;
        }
        if ((message != WM_KEYDOWN && message != WM_SYSKEYDOWN) || !recording) {
            return message == WM_CHAR && recording;
        }
        int key = (int)wparam;
        *result = 0;
        if (key == VK_SHIFT || key == VK_CONTROL || key == VK_MENU || key == VK_LWIN || key == VK_RWIN) {
            return true;
        }
        int mods = gkModifiers();
        recording = 0;
        if (key == VK_ESCAPE && !(mods & (GK_MOD_CONTROL | GK_MOD_ALT))) {
            update(view);
            return true;
        }
        if ((key == VK_BACK || key == VK_DELETE) &&
            !(mods & (GK_MOD_CONTROL | GK_MOD_ALT | GK_MOD_SHIFT | GK_MOD_SUPER))) {
            update(view);
            gkSend(view->handle, GK_EVENT_VALUE, 0, 0, 0, 0, "");
            return true;
        }
        int bits = 0;
        if (mods & GK_MOD_CONTROL) {
            bits |= 1;
        }
        if (mods & GK_MOD_SUPER) {
            bits |= 2;
        }
        if (mods & GK_MOD_ALT) {
            bits |= 4;
        }
        if (mods & GK_MOD_SHIFT) {
            bits |= 8;
        }
        char text[8] = {0};
        UINT character = MapVirtualKeyW((UINT)key, MAPVK_VK_TO_CHAR) & 0x7fffffff;
        if (character >= 0x20) {
            wchar_t wide[2] = {(wchar_t)towupper((wint_t)character), 0};
            WideCharToMultiByte(CP_UTF8, 0, wide, 1, text, sizeof(text) - 1, nullptr, nullptr);
        }
        GKWin32Event report = {};
        report.kind = GK_EVENT_VALUE;
        report.a = bits;
        report.b = gkKeyFromVirtualKey(wparam, lparam);
        report.text = text;
        gkSendEvent(view->handle, &report);
        update(view);
        return true;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        SIZE text = gkMeasureGDI(L"Record Shortcut", gkBodyFont());
        (void)view;
        return D2D1::SizeF((float)text.cx + 24, (float)(gkMeasureGDI(L"Ag", gkBodyFont()).cy + 8));
    }
};

extern "C" void *gkWin32KeyRecorderCreate(uintptr_t window) {
    GKRecorder *recorder = new GKRecorder();
    GKView *view = gkViewNative(window, WC_BUTTONW, L"", BS_PUSHBUTTON | WS_TABSTOP, GK_KIND_KEY_RECORDER, recorder);
    if (view) {
        recorder->update(view);
    }
    return view;
}

extern "C" void gkWin32KeyRecorderSet(void *pointer, const char *display) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_KEY_RECORDER) {
        return;
    }
    GKRecorder *recorder = static_cast<GKRecorder *>(view->ext.get());
    recorder->display = display ? display : "";
    recorder->update(view);
}
