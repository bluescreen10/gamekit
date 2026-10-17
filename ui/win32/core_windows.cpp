//go:build windows && cgo

#include "win32_internal.h"

#include <dwmapi.h>
#include <ole2.h>
#include <shellapi.h>
#include <uxtheme.h>

#include <algorithm>
#include <map>
#include <set>

/* ---- Objects ---- */

void gkRetainObject(GKObject *object) {
    if (object) {
        object->refs++;
    }
}

void gkReleaseObject(GKObject *object) {
    if (object && --object->refs <= 0) {
        delete object;
    }
}

extern "C" void gkWin32Retain(void *object) {
    gkRetainObject(static_cast<GKObject *>(object));
}

extern "C" void gkWin32Release(void *object) {
    gkReleaseObject(static_cast<GKObject *>(object));
}

extern "C" void gkWin32Free(void *pointer) {
    free(pointer);
}

extern "C" void gkWin32SetHandle(void *object, uintptr_t handle) {
    if (object) {
        static_cast<GKObject *>(object)->handle = handle;
    }
}

extern "C" uintptr_t gkWin32Handle(void *object) {
    return object ? static_cast<GKObject *>(object)->handle : 0;
}

/* ---- Strings ---- */

std::wstring gkWide(const char *text) {
    if (!text || !*text) {
        return std::wstring();
    }
    int count = MultiByteToWideChar(CP_UTF8, 0, text, -1, nullptr, 0);
    std::wstring wide((size_t)(count > 0 ? count - 1 : 0), L'\0');
    if (count > 1) {
        MultiByteToWideChar(CP_UTF8, 0, text, -1, &wide[0], count);
    }
    return wide;
}

std::string gkUTF8(const wchar_t *text, int length) {
    if (!text || length == 0) {
        return std::string();
    }
    int count = WideCharToMultiByte(CP_UTF8, 0, text, length, nullptr, 0, nullptr, nullptr);
    if (count <= 0) {
        return std::string();
    }
    std::string result((size_t)count, '\0');
    WideCharToMultiByte(CP_UTF8, 0, text, length, &result[0], count, nullptr, nullptr);
    if (length < 0 && !result.empty() && result.back() == '\0') {
        result.pop_back();
    }
    return result;
}

std::string gkUTF8(std::wstring const &text) {
    return gkUTF8(text.c_str(), (int)text.size());
}

char *gkStrdup(const char *text) {
    const char *value = text ? text : "";
    size_t length = strlen(value);
    char *copy = static_cast<char *>(malloc(length + 1));
    if (copy) {
        memcpy(copy, value, length + 1);
    }
    return copy;
}

char *gkStrdupW(std::wstring const &text) {
    return gkStrdup(gkUTF8(text).c_str());
}

std::vector<std::string> gkSplit(const char *buffer, size_t size) {
    std::vector<std::string> values;
    if (!buffer || size == 0) {
        return values;
    }
    size_t start = 0;
    for (size_t i = 0; i < size; i++) {
        if (buffer[i] == '\0') {
            values.emplace_back(buffer + start, i - start);
            start = i + 1;
        }
    }
    if (start < size) {
        values.emplace_back(buffer + start, size - start);
    }
    return values;
}

char *gkJoin(std::vector<std::string> const &values, size_t *size) {
    size_t total = 0;
    for (auto const &value : values) {
        total += value.size() + 1;
    }
    char *buffer = static_cast<char *>(malloc(total ? total : 1));
    size_t offset = 0;
    for (auto const &value : values) {
        memcpy(buffer + offset, value.data(), value.size());
        offset += value.size();
        buffer[offset++] = '\0';
    }
    if (size) {
        *size = total;
    }
    return buffer;
}

/* ---- Common controls ---- */

static HANDLE gkActivationContext = INVALID_HANDLE_VALUE;

/* gkCreateActivationContext creates an activation context from the manifest
   of the common controls version 6 that shell32.dll carries as resource
   124, so the bridge gets the controls with visual styles without a
   manifest in the program. */
static void gkCreateActivationContext() {
    if (gkActivationContext != INVALID_HANDLE_VALUE) {
        return;
    }
    wchar_t system[MAX_PATH];
    GetSystemDirectoryW(system, MAX_PATH);
    ACTCTXW context = {};
    context.cbSize = sizeof(context);
    context.dwFlags = ACTCTX_FLAG_RESOURCE_NAME_VALID | ACTCTX_FLAG_ASSEMBLY_DIRECTORY_VALID;
    context.lpSource = L"shell32.dll";
    context.lpResourceName = MAKEINTRESOURCEW(124);
    context.lpAssemblyDirectory = system;
    gkActivationContext = CreateActCtxW(&context);
}

GKActivation::GKActivation() {
    gkCreateActivationContext();
    if (gkActivationContext != INVALID_HANDLE_VALUE) {
        ActivateActCtx(gkActivationContext, &cookie);
    }
}

GKActivation::~GKActivation() {
    if (cookie) {
        DeactivateActCtx(0, cookie);
    }
}

GKComctl const &gkComctl() {
    static GKComctl table;
    static bool loaded;
    if (!loaded) {
        loaded = true;
        GKActivation activation;
        HMODULE module = LoadLibraryW(L"comctl32.dll");
        if (module) {
#define GK_LOAD(name) table.name = reinterpret_cast<decltype(table.name)>(GetProcAddress(module, #name))
            GK_LOAD(InitCommonControlsEx);
            GK_LOAD(SetWindowSubclass);
            GK_LOAD(RemoveWindowSubclass);
            GK_LOAD(DefSubclassProc);
            GK_LOAD(TaskDialogIndirect);
            GK_LOAD(ImageList_Create);
            GK_LOAD(ImageList_Add);
            GK_LOAD(ImageList_ReplaceIcon);
            GK_LOAD(ImageList_Destroy);
            GK_LOAD(ImageList_RemoveAll);
            GK_LOAD(LoadIconMetric);
#undef GK_LOAD
        }
    }
    return table;
}

/* ---- Theme ---- */

GKColor gkRGB(unsigned int rgb, float alpha) {
    GKColor color;
    color.r = ((rgb >> 16) & 0xff) / 255.0f;
    color.g = ((rgb >> 8) & 0xff) / 255.0f;
    color.b = (rgb & 0xff) / 255.0f;
    color.a = alpha;
    return color;
}

GKColor gkMix(GKColor below, GKColor above) {
    GKColor color;
    float a = above.a;
    color.r = below.r * (1 - a) + above.r * a;
    color.g = below.g * (1 - a) + above.g * a;
    color.b = below.b * (1 - a) + above.b * a;
    color.a = 1;
    return color;
}

static COLORREF gkColorRef(GKColor const &color) {
    return RGB((int)lroundf(color.r * 255), (int)lroundf(color.g * 255), (int)lroundf(color.b * 255));
}

int gkSystemDark() {
    DWORD light = 1, size = sizeof(light);
    if (RegGetValueW(HKEY_CURRENT_USER, L"Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize",
                     L"AppsUseLightTheme", RRF_RT_REG_DWORD, nullptr, &light, &size) != ERROR_SUCCESS) {
        return 0;
    }
    return light == 0;
}

/* gkAccentShade returns a shade of the accent color from the accent palette
   of the system: 0 the lightest through 6 the darkest, 3 the accent
   itself. */
static GKColor gkAccentShade(int shade, unsigned int fallback) {
    BYTE palette[32];
    DWORD size = sizeof(palette);
    if (RegGetValueW(HKEY_CURRENT_USER, L"Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Accent",
                     L"AccentPalette", RRF_RT_REG_BINARY, nullptr, palette, &size) == ERROR_SUCCESS &&
        size >= 28) {
        BYTE *entry = palette + shade * 4;
        return gkRGB(((unsigned int)entry[0] << 16) | ((unsigned int)entry[1] << 8) | entry[2]);
    }
    return gkRGB(fallback);
}

GKColor gkAccentColor() {
    return gkAccentShade(3, 0x0078d4);
}

static GKPalette gkMakePalette(int dark) {
    GKPalette p;
    p.dark = dark;
    if (!dark) {
        p.window = gkRGB(GetSysColor(COLOR_WINDOW) == RGB(255, 255, 255) ? 0xffffff : 0xffffff);
        p.layer = gkRGB(0xf9f9f9);
        p.layerStroke = gkRGB(0xe5e5e5);
        p.text = gkRGB(0x000000);
        p.textSecondary = gkRGB(0x5d5d5d);
        p.textTertiary = gkRGB(0x8a8a8a);
        p.textDisabled = gkRGB(0xa0a0a0);
        p.textOnAccent = gkRGB(0xffffff);
        p.control = gkRGB(0xfdfdfd);
        p.controlHover = gkRGB(0xe5f1fb);
        p.controlPressed = gkRGB(0xcce4f7);
        p.controlDisabled = gkRGB(0xf4f4f4);
        p.controlStroke = gkRGB(0xadadad);
        p.controlStrokeBottom = gkRGB(0xadadad);
        p.controlStrong = gkRGB(0x333333);
        p.controlStrongDisabled = gkRGB(0xbfbfbf);
        p.inputActive = gkRGB(0xffffff);
        p.subtle = gkRGB(0xe5f3ff);
        p.subtlePressed = gkRGB(0xcce8ff);
        p.accent = gkAccentShade(4, 0x005fb8);
        p.accentHover = gkAccentShade(3, 0x0078d4);
        p.accentPressed = gkAccentShade(5, 0x003e92);
        p.accentDisabled = gkRGB(0xbfbfbf);
        p.accentText = gkRGB(0x0066cc);
        p.divider = gkRGB(0xe0e0e0);
        p.focus = gkRGB(0x000000);
        p.success = gkRGB(0x0f7b0f);
        p.caution = gkRGB(0x9d5d00);
        p.critical = gkRGB(0xc42b1c);
        p.information = gkAccentShade(4, 0x005fb8);
        p.successBackground = gkRGB(0xdff6dd);
        p.cautionBackground = gkRGB(0xfff4ce);
        p.criticalBackground = gkRGB(0xfde7e9);
        p.informationBackground = gkRGB(0xebf3fb);
        p.selection = gkRGB(0xcce8ff);
        p.flyout = gkRGB(0xf9f9f9);
    } else {
        p.window = gkRGB(0x202020);
        p.layer = gkRGB(0x2b2b2b);
        p.layerStroke = gkRGB(0x3a3a3a);
        p.text = gkRGB(0xffffff);
        p.textSecondary = gkRGB(0xc5c5c5);
        p.textTertiary = gkRGB(0x8b8b8b);
        p.textDisabled = gkRGB(0x6d6d6d);
        p.textOnAccent = gkRGB(0x000000);
        p.control = gkRGB(0x333333);
        p.controlHover = gkRGB(0x3c3c3c);
        p.controlPressed = gkRGB(0x2a2a2a);
        p.controlDisabled = gkRGB(0x2a2a2a);
        p.controlStroke = gkRGB(0x4a4a4a);
        p.controlStrokeBottom = gkRGB(0x4a4a4a);
        p.controlStrong = gkRGB(0x9a9a9a);
        p.controlStrongDisabled = gkRGB(0x4a4a4a);
        p.inputActive = gkRGB(0x1c1c1c);
        p.subtle = gkRGB(0x353535);
        p.subtlePressed = gkRGB(0x2e2e2e);
        p.accent = gkAccentShade(1, 0x60cdff);
        p.accentHover = gkAccentShade(2, 0x4cc2ff);
        p.accentPressed = gkAccentShade(2, 0x4cc2ff);
        p.accentDisabled = gkRGB(0x4a4a4a);
        p.accentText = gkAccentShade(0, 0x99ebff);
        p.divider = gkRGB(0x3d3d3d);
        p.focus = gkRGB(0xffffff);
        p.success = gkRGB(0x6ccb5f);
        p.caution = gkRGB(0xfce100);
        p.critical = gkRGB(0xff99a4);
        p.information = gkAccentShade(1, 0x60cdff);
        p.successBackground = gkRGB(0x393d1b);
        p.cautionBackground = gkRGB(0x433519);
        p.criticalBackground = gkRGB(0x442726);
        p.informationBackground = gkRGB(0x2b2b2b);
        p.selection = gkRGB(0x3a3a3a);
        p.flyout = gkRGB(0x2c2c2c);
    }
    return p;
}

GKPalette const &gkPalette(int theme) {
    static GKPalette light, dark;
    static bool made;
    if (!made) {
        made = true;
        light = gkMakePalette(0);
        dark = gkMakePalette(1);
    }
    return theme == 2 ? dark : light;
}

bool gkSemanticColor(GKPalette const &palette, int color, GKColor *out) {
    switch (color) {
    case 1:
        *out = palette.text;
        return true;
    case 2:
        *out = palette.textSecondary;
        return true;
    case 3:
        *out = palette.textTertiary;
        return true;
    case 4:
        *out = palette.layer;
        return true;
    case 5:
        *out = palette.accent;
        return true;
    case 6:
        *out = palette.success;
        return true;
    case 7:
        *out = palette.caution;
        return true;
    case 8:
        *out = palette.critical;
        return true;
    case 9:
        *out = palette.accentText;
        return true;
    default:
        return false;
    }
}

/* The dark mode of the controls of Windows is reached through functions
   that uxtheme.dll exports by ordinal. */
enum { GK_APP_MODE_DEFAULT = 0, GK_APP_MODE_ALLOW_DARK = 1 };

static void gkUxThemeInitialize() {
    static bool done;
    if (done) {
        return;
    }
    done = true;
    HMODULE uxtheme = LoadLibraryExW(L"uxtheme.dll", nullptr, LOAD_LIBRARY_SEARCH_SYSTEM32);
    if (!uxtheme) {
        return;
    }
    using SetPreferredAppMode = int(WINAPI *)(int);
    using FlushMenuThemes = void(WINAPI *)();
    auto setMode = reinterpret_cast<SetPreferredAppMode>(GetProcAddress(uxtheme, MAKEINTRESOURCEA(135)));
    auto flush = reinterpret_cast<FlushMenuThemes>(GetProcAddress(uxtheme, MAKEINTRESOURCEA(136)));
    if (setMode) {
        setMode(GK_APP_MODE_ALLOW_DARK);
    }
    if (flush) {
        flush();
    }
}

static void gkAllowDark(HWND hwnd, int dark) {
    static auto allow = []() {
        HMODULE uxtheme = GetModuleHandleW(L"uxtheme.dll");
        using AllowDarkModeForWindow = BOOL(WINAPI *)(HWND, BOOL);
        return uxtheme ? reinterpret_cast<AllowDarkModeForWindow>(GetProcAddress(uxtheme, MAKEINTRESOURCEA(133)))
                       : nullptr;
    }();
    if (allow) {
        allow(hwnd, dark ? TRUE : FALSE);
    }
}

void gkApplyThemeTo(HWND hwnd, int theme, const wchar_t *lightClass) {
    if (!hwnd) {
        return;
    }
    wchar_t name[64] = {};
    GetClassNameW(hwnd, name, 64);
    int dark = theme == 2;
    gkAllowDark(hwnd, dark);
    const wchar_t *style = nullptr;
    if (!wcscmp(name, WC_EDITW) || !wcscmp(name, WC_COMBOBOXW) || !wcscmp(name, L"RICHEDIT50W")) {
        style = dark ? L"DarkMode_CFD" : (lightClass ? lightClass : L"");
    } else if (!wcscmp(name, WC_LISTVIEWW) || !wcscmp(name, WC_TREEVIEWW)) {
        style = dark ? L"DarkMode_Explorer" : L"Explorer";
    } else {
        style = dark ? L"DarkMode_Explorer" : (lightClass ? lightClass : L"");
    }
    SetWindowTheme(hwnd, style && *style ? style : nullptr, nullptr);
    SendMessageW(hwnd, WM_THEMECHANGED, 0, 0);
}

/* ---- Fonts ---- */

static GKFont gkMessageFont() {
    static GKFont font;
    static bool made;
    if (!made) {
        made = true;
        NONCLIENTMETRICSW metrics = {};
        metrics.cbSize = sizeof(metrics);
        if (SystemParametersInfoForDpi(SPI_GETNONCLIENTMETRICS, sizeof(metrics), &metrics, 0, 96)) {
            font.family = metrics.lfMessageFont.lfFaceName;
            font.size = (float)abs(metrics.lfMessageFont.lfHeight);
        }
        if (font.family.empty()) {
            font.family = L"Segoe UI";
        }
        if (font.size <= 0) {
            font.size = 12;
        }
    }
    return font;
}

GKFont gkBodyFont() {
    return gkMessageFont();
}

GKFont gkTypeRamp(int style) {
    GKFont font = gkMessageFont();
    switch (style) {
    case 1:
        font.size = font.size - 1;
        break;
    case 2:
        font.weight = 600;
        break;
    case 3:
        font.size = font.size * 4 / 3;
        font.weight = 600;
        break;
    case 4:
        font.size = font.size * 5 / 3;
        font.weight = 600;
        break;
    case 5:
        font.size = font.size * 7 / 3;
        font.weight = 600;
        break;
    case 6:
        font.size = font.size * 10 / 3;
        font.weight = 600;
        break;
    case 7:
        font.family = gkSymbolFamily();
        break;
    default:
        break;
    }
    return font;
}

std::wstring const &gkSymbolFamily() {
    static std::wstring family = gkFontFamilyInstalled("Segoe Fluent Icons") ? L"Segoe Fluent Icons"
                                                                              : L"Segoe MDL2 Assets";
    return family;
}

HFONT gkFontHandle(GKFont const &font, UINT dpi) {
    static std::map<std::wstring, HFONT> fonts;
    wchar_t key[256];
    swprintf(key, 256, L"%ls|%.2f|%d|%d|%u", font.family.c_str(), font.size, font.weight, font.italic, dpi);
    auto found = fonts.find(key);
    if (found != fonts.end()) {
        return found->second;
    }
    HFONT handle = CreateFontW(-(int)lroundf(font.size * dpi / 96.0f), 0, 0, 0, font.weight, font.italic, FALSE,
                               FALSE, DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY,
                               DEFAULT_PITCH, font.family.c_str());
    fonts[key] = handle;
    return handle;
}


int gkFontFamilyInstalled(const char *family) {
    IDWriteFontCollection *collection = nullptr;
    if (!gkDWrite() || FAILED(gkDWrite()->GetSystemFontCollection(&collection, FALSE)) || !collection) {
        return 0;
    }
    UINT32 index = 0;
    BOOL exists = FALSE;
    collection->FindFamilyName(gkWide(family).c_str(), &index, &exists);
    collection->Release();
    return exists ? 1 : 0;
}

/* ---- Direct2D ---- */

ID2D1Factory *gkD2D() {
    static ID2D1Factory *factory;
    if (!factory) {
        D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED, __uuidof(ID2D1Factory), nullptr,
                          reinterpret_cast<void **>(&factory));
    }
    return factory;
}

IDWriteFactory *gkDWrite() {
    static IDWriteFactory *factory;
    if (!factory) {
        DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED, __uuidof(IDWriteFactory),
                            reinterpret_cast<IUnknown **>(&factory));
    }
    return factory;
}

IDWriteTextFormat *gkTextFormat(GKFont const &font) {
    static std::map<std::wstring, IDWriteTextFormat *> formats;
    wchar_t key[256];
    swprintf(key, 256, L"%ls|%.2f|%d|%d", font.family.c_str(), font.size, font.weight, font.italic);
    auto found = formats.find(key);
    if (found != formats.end()) {
        return found->second;
    }
    IDWriteTextFormat *format = nullptr;
    if (gkDWrite()) {
        gkDWrite()->CreateTextFormat(font.family.c_str(), nullptr, (DWRITE_FONT_WEIGHT)font.weight,
                                     font.italic ? DWRITE_FONT_STYLE_ITALIC : DWRITE_FONT_STYLE_NORMAL,
                                     DWRITE_FONT_STRETCH_NORMAL, font.size, L"", &format);
    }
    formats[key] = format;
    return format;
}

static D2D1_COLOR_F gkD2DColor(GKColor const &color) {
    return D2D1::ColorF(color.r, color.g, color.b, color.a);
}

void GKCanvas::color(GKColor const &value) {
    if (brush) {
        brush->SetColor(gkD2DColor(value));
    }
}

void GKCanvas::fillRect(float x, float y, float w, float h, GKColor const &value) {
    color(value);
    target->FillRectangle(D2D1::RectF(x, y, x + w, y + h), brush);
}

void GKCanvas::fillRound(float x, float y, float w, float h, float radius, GKColor const &value) {
    color(value);
    target->FillRoundedRectangle(D2D1::RoundedRect(D2D1::RectF(x, y, x + w, y + h), radius, radius), brush);
}

void GKCanvas::strokeRound(float x, float y, float w, float h, float radius, GKColor const &value, float thickness) {
    color(value);
    float inset = thickness / 2;
    target->DrawRoundedRectangle(
        D2D1::RoundedRect(D2D1::RectF(x + inset, y + inset, x + w - inset, y + h - inset), radius, radius), brush,
        thickness);
}

void GKCanvas::fillEllipse(float cx, float cy, float rx, float ry, GKColor const &value) {
    color(value);
    target->FillEllipse(D2D1::Ellipse(D2D1::Point2F(cx, cy), rx, ry), brush);
}

void GKCanvas::strokeEllipse(float cx, float cy, float rx, float ry, GKColor const &value, float thickness) {
    color(value);
    target->DrawEllipse(D2D1::Ellipse(D2D1::Point2F(cx, cy), rx, ry), brush, thickness);
}

void GKCanvas::line(float x1, float y1, float x2, float y2, GKColor const &value, float thickness) {
    color(value);
    target->DrawLine(D2D1::Point2F(x1, y1), D2D1::Point2F(x2, y2), brush, thickness);
}

void GKCanvas::arc(float cx, float cy, float radius, float start, float sweep, GKColor const &value,
                   float thickness) {
    if (sweep <= 0) {
        return;
    }
    if (sweep >= 2 * M_PI - 0.001) {
        strokeEllipse(cx, cy, radius, radius, value, thickness);
        return;
    }
    ID2D1PathGeometry *path = nullptr;
    if (FAILED(gkD2D()->CreatePathGeometry(&path)) || !path) {
        return;
    }
    ID2D1GeometrySink *sink = nullptr;
    path->Open(&sink);
    sink->BeginFigure(D2D1::Point2F(cx + cosf(start) * radius, cy + sinf(start) * radius), D2D1_FIGURE_BEGIN_HOLLOW);
    float end = start + sweep;
    sink->AddArc(D2D1::ArcSegment(D2D1::Point2F(cx + cosf(end) * radius, cy + sinf(end) * radius),
                                  D2D1::SizeF(radius, radius), 0, D2D1_SWEEP_DIRECTION_CLOCKWISE,
                                  sweep > M_PI ? D2D1_ARC_SIZE_LARGE : D2D1_ARC_SIZE_SMALL));
    sink->EndFigure(D2D1_FIGURE_END_OPEN);
    sink->Close();
    sink->Release();
    color(value);
    ID2D1StrokeStyle *round = nullptr;
    D2D1_STROKE_STYLE_PROPERTIES properties = D2D1::StrokeStyleProperties(D2D1_CAP_STYLE_ROUND, D2D1_CAP_STYLE_ROUND);
    gkD2D()->CreateStrokeStyle(properties, nullptr, 0, &round);
    target->DrawGeometry(path, brush, thickness, round);
    if (round) {
        round->Release();
    }
    path->Release();
}

void GKCanvas::text(std::wstring const &value, GKFont const &font, float x, float y, float w, float h,
                    GKColor const &textColor, int align, int valign, int wrap, int trimming) {
    IDWriteTextFormat *format = gkTextFormat(font);
    if (!format || value.empty()) {
        return;
    }
    IDWriteTextLayout *layout = nullptr;
    if (FAILED(gkDWrite()->CreateTextLayout(value.c_str(), (UINT32)value.size(), format, w > 0 ? w : 1,
                                            h > 0 ? h : 1, &layout)) ||
        !layout) {
        return;
    }
    layout->SetTextAlignment(align == 1   ? DWRITE_TEXT_ALIGNMENT_CENTER
                             : align == 2 ? DWRITE_TEXT_ALIGNMENT_TRAILING
                                          : DWRITE_TEXT_ALIGNMENT_LEADING);
    layout->SetParagraphAlignment(valign == 1   ? DWRITE_PARAGRAPH_ALIGNMENT_CENTER
                                  : valign == 2 ? DWRITE_PARAGRAPH_ALIGNMENT_FAR
                                                : DWRITE_PARAGRAPH_ALIGNMENT_NEAR);
    layout->SetWordWrapping(wrap ? DWRITE_WORD_WRAPPING_WRAP : DWRITE_WORD_WRAPPING_NO_WRAP);
    if (trimming) {
        DWRITE_TRIMMING trim = {DWRITE_TRIMMING_GRANULARITY_CHARACTER, 0, 0};
        IDWriteInlineObject *ellipsis = nullptr;
        gkDWrite()->CreateEllipsisTrimmingSign(format, &ellipsis);
        layout->SetTrimming(&trim, ellipsis);
        if (ellipsis) {
            ellipsis->Release();
        }
    }
    color(textColor);
    target->DrawTextLayout(D2D1::Point2F(x, y), layout, brush, D2D1_DRAW_TEXT_OPTIONS_CLIP);
    layout->Release();
}

void GKCanvas::centeredText(std::wstring const &value, GKFont const &font, float x, float y, float w, float h,
                            GKColor const &textColor) {
    IDWriteTextFormat *format = gkTextFormat(font);
    if (!format || value.empty()) {
        return;
    }
    IDWriteTextLayout *layout = nullptr;
    if (FAILED(gkDWrite()->CreateTextLayout(value.c_str(), (UINT32)value.size(), format, w > 0 ? w : 1,
                                            h > 0 ? h : 1, &layout)) ||
        !layout) {
        return;
    }
    layout->SetTextAlignment(DWRITE_TEXT_ALIGNMENT_CENTER);
    layout->SetParagraphAlignment(DWRITE_PARAGRAPH_ALIGNMENT_CENTER);
    layout->SetWordWrapping(DWRITE_WORD_WRAPPING_NO_WRAP);
    /* The line of a font has room above the glyphs for accents, so the
       glyphs are centered by their ink instead of their line. */
    DWRITE_OVERHANG_METRICS overhang = {};
    float shift = 0;
    if (SUCCEEDED(layout->GetOverhangMetrics(&overhang))) {
        shift = (overhang.top - overhang.bottom) / 2;
    }
    color(textColor);
    target->DrawTextLayout(D2D1::Point2F(x, y + shift), layout, brush, D2D1_DRAW_TEXT_OPTIONS_NONE);
    layout->Release();
}

void GKCanvas::glyph(std::wstring const &value, float size, float x, float y, float w, float h,
                     GKColor const &glyphColor) {
    GKFont font;
    font.family = gkSymbolFamily();
    font.size = size;
    text(value, font, x, y, w, h, glyphColor, 1, 1, 0, 0);
}

void GKCanvas::bitmap(ID2D1Bitmap *image, float x, float y, float w, float h, float opacity, int nearest) {
    if (!image) {
        return;
    }
    target->DrawBitmap(image, D2D1::RectF(x, y, x + w, y + h), opacity,
                       nearest ? D2D1_BITMAP_INTERPOLATION_MODE_NEAREST_NEIGHBOR
                               : D2D1_BITMAP_INTERPOLATION_MODE_LINEAR);
}

void GKCanvas::control(float x, float y, float w, float h, float radius, GKColor const &fill,
                       GKPalette const &palette, int accent) {
    fillRound(x, y, w, h, radius, fill);
    if (!accent) {
        strokeRound(x, y, w, h, radius, palette.controlStroke);
    }
}

void GKCanvas::focusRect(float x, float y, float w, float h, float radius, GKPalette const &palette) {
    strokeRound(x - 3, y - 3, w + 6, h + 6, radius + 3, palette.focus, 2);
}

void GKCanvas::clip(float x, float y, float w, float h) {
    target->PushAxisAlignedClip(D2D1::RectF(x, y, x + w, y + h), D2D1_ANTIALIAS_MODE_ALIASED);
}

void GKCanvas::unclip() {
    target->PopAxisAlignedClip();
}

/* gkMeasureKey names a measurement of text: the layout code measures the
   same text in the same font many times, so measurements are kept. */
static std::wstring gkMeasureKey(std::wstring const &text, GKFont const &font, float width, UINT flags,
                                 int lines) {
    wchar_t prefix[160];
    swprintf(prefix, 160, L"%ls|%.2f|%d|%d|%.1f|%u|%d|", font.family.c_str(), font.size, font.weight, font.italic,
             width, flags, lines);
    return prefix + text;
}

template <typename T> static T *gkMeasureFind(std::unordered_map<std::wstring, T> &cache, std::wstring const &key) {
    auto found = cache.find(key);
    return found == cache.end() ? nullptr : &found->second;
}

template <typename T> static void gkMeasureStore(std::unordered_map<std::wstring, T> &cache, std::wstring key,
                                                 T const &value) {
    /* The cache forgets everything when it grows large, which is simpler
       than tracking use and rare in practice. */
    if (cache.size() > 20000) {
        cache.clear();
    }
    cache.emplace(std::move(key), value);
}

static D2D1_SIZE_F gkMeasureTextUncached(std::wstring const &value, GKFont const &font, float width,
                                         int maximumLines);

D2D1_SIZE_F gkMeasureText(std::wstring const &value, GKFont const &font, float width, int maximumLines) {
    static std::unordered_map<std::wstring, D2D1_SIZE_F> cache;
    std::wstring key = gkMeasureKey(value, font, width, 0, maximumLines);
    if (D2D1_SIZE_F *found = gkMeasureFind(cache, key)) {
        return *found;
    }
    D2D1_SIZE_F size = gkMeasureTextUncached(value, font, width, maximumLines);
    gkMeasureStore(cache, std::move(key), size);
    return size;
}

static D2D1_SIZE_F gkMeasureTextUncached(std::wstring const &value, GKFont const &font, float width,
                                         int maximumLines) {
    IDWriteTextFormat *format = gkTextFormat(font);
    if (!format) {
        return D2D1::SizeF(0, 0);
    }
    std::wstring text = value.empty() ? std::wstring(L" ") : value;
    IDWriteTextLayout *layout = nullptr;
    if (FAILED(gkDWrite()->CreateTextLayout(text.c_str(), (UINT32)text.size(), format, width > 0 ? width : 100000,
                                            100000, &layout)) ||
        !layout) {
        return D2D1::SizeF(0, 0);
    }
    layout->SetWordWrapping(width > 0 ? DWRITE_WORD_WRAPPING_WRAP : DWRITE_WORD_WRAPPING_NO_WRAP);
    DWRITE_TEXT_METRICS metrics = {};
    layout->GetMetrics(&metrics);
    float height = metrics.height;
    if (maximumLines > 0 && metrics.lineCount > (UINT32)maximumLines) {
        height = metrics.height * maximumLines / metrics.lineCount;
    }
    layout->Release();
    return D2D1::SizeF(value.empty() ? 0 : metrics.widthIncludingTrailingWhitespace, height);
}

ID2D1Bitmap *gkBitmapFromRGBA(ID2D1RenderTarget *target, const unsigned char *pixels, int width, int height,
                              int stride) {
    if (!target || !pixels || width <= 0 || height <= 0) {
        return nullptr;
    }
    std::vector<unsigned char> bgra((size_t)width * height * 4);
    for (int y = 0; y < height; y++) {
        const unsigned char *row = pixels + (size_t)y * stride;
        for (int x = 0; x < width; x++) {
            const unsigned char *p = row + x * 4;
            unsigned char *q = bgra.data() + ((size_t)y * width + x) * 4;
            unsigned int a = p[3];
            q[0] = (unsigned char)(p[2] * a / 255);
            q[1] = (unsigned char)(p[1] * a / 255);
            q[2] = (unsigned char)(p[0] * a / 255);
            q[3] = (unsigned char)a;
        }
    }
    D2D1_BITMAP_PROPERTIES properties =
        D2D1::BitmapProperties(D2D1::PixelFormat(DXGI_FORMAT_B8G8R8A8_UNORM, D2D1_ALPHA_MODE_PREMULTIPLIED));
    ID2D1Bitmap *bitmap = nullptr;
    target->CreateBitmap(D2D1::SizeU(width, height), bgra.data(), width * 4, properties, &bitmap);
    return bitmap;
}

/* gkRenderTarget returns the render target that paints on device
   contexts. */
static ID2D1DCRenderTarget *gkRenderTarget(bool reset = false) {
    static ID2D1DCRenderTarget *target;
    if (reset && target) {
        target->Release();
        target = nullptr;
    }
    if (!target && gkD2D()) {
        D2D1_RENDER_TARGET_PROPERTIES properties = D2D1::RenderTargetProperties(
            D2D1_RENDER_TARGET_TYPE_DEFAULT,
            D2D1::PixelFormat(DXGI_FORMAT_B8G8R8A8_UNORM, D2D1_ALPHA_MODE_PREMULTIPLIED), 0, 0,
            D2D1_RENDER_TARGET_USAGE_GDI_COMPATIBLE);
        gkD2D()->CreateDCRenderTarget(&properties, &target);
    }
    return target;
}

void gkPaintView(GKView *view, HWND hwnd) {
    PAINTSTRUCT paint;
    HDC hdc = BeginPaint(hwnd, &paint);
    RECT client;
    GetClientRect(hwnd, &client);
    if (view && view->ext) {
        HDC buffer = nullptr;
        HPAINTBUFFER painting = BeginBufferedPaint(hdc, &client, BPBF_COMPATIBLEBITMAP, nullptr, &buffer);
        HDC target = painting ? buffer : hdc;
        HBRUSH background = CreateSolidBrush(gkColorRef(gkBackgroundOf(hwnd)));
        FillRect(target, &client, background);
        DeleteObject(background);
        bool painted = view->ext->paintGDI(view, hwnd, target, client);
        if (painting) {
            EndBufferedPaint(painting, painted ? TRUE : FALSE);
        }
        if (painted) {
            EndPaint(hwnd, &paint);
            return;
        }
    }
    /* A view inside a native control, such as the spinner of a busy button,
       draws over the face of the control. */
    bool overNative = false;
    if (HWND parent = GetParent(hwnd)) {
        wchar_t name[32] = {};
        GetClassNameW(parent, name, 32);
        overNative = wcsncmp(name, L"GameKit", 7) != 0;
    }
    /* The view is drawn off the screen and then shown at once, so that an
       animation, such as a spinner over a button, does not flicker. */
    HDC buffer = nullptr;
    HPAINTBUFFER painting =
        client.right > 0 && client.bottom > 0
            ? BeginBufferedPaint(hdc, &client, BPBF_COMPATIBLEBITMAP, nullptr, &buffer)
            : nullptr;
    HDC surface = painting ? buffer : hdc;
    if (overNative) {
        DrawThemeParentBackground(hwnd, surface, &client);
    }
    ID2D1DCRenderTarget *target = gkRenderTarget();
    if (target && client.right > 0 && client.bottom > 0) {
        float dpi = (float)GetDpiForWindow(hwnd);
        target->SetDpi(dpi, dpi);
        if (SUCCEEDED(target->BindDC(surface, &client))) {
            GKCanvas canvas;
            canvas.target = target;
            canvas.scale = dpi / 96.0f;
            canvas.width = client.right / canvas.scale;
            canvas.height = client.bottom / canvas.scale;
            target->CreateSolidColorBrush(D2D1::ColorF(0, 0, 0, 1), &canvas.brush);
            target->BeginDraw();
            target->SetTextAntialiasMode(overNative ? D2D1_TEXT_ANTIALIAS_MODE_GRAYSCALE
                                                    : D2D1_TEXT_ANTIALIAS_MODE_CLEARTYPE);
            if (!overNative) {
                target->Clear(gkD2DColor(gkBackgroundOf(hwnd)));
            }
            if (view && view->ext) {
                view->ext->paint(view, canvas);
            }
            if (target->EndDraw() == D2DERR_RECREATE_TARGET) {
                gkRenderTarget(true);
            }
            if (canvas.brush) {
                canvas.brush->Release();
            }
        }
    }
    if (painting) {
        EndBufferedPaint(painting, TRUE);
    }
    EndPaint(hwnd, &paint);
}

bool gkPaintCanvas(HWND hwnd, HDC hdc, RECT const &rect, std::function<void(GKCanvas &)> const &draw) {
    ID2D1DCRenderTarget *target = gkRenderTarget();
    if (!target || rect.right <= rect.left || rect.bottom <= rect.top) {
        return false;
    }
    float dpi = (float)GetDpiForWindow(hwnd);
    target->SetDpi(dpi, dpi);
    if (FAILED(target->BindDC(hdc, &rect))) {
        return false;
    }
    GKCanvas canvas;
    canvas.target = target;
    canvas.scale = dpi / 96.0f;
    canvas.width = (rect.right - rect.left) / canvas.scale;
    canvas.height = (rect.bottom - rect.top) / canvas.scale;
    target->CreateSolidColorBrush(D2D1::ColorF(0, 0, 0, 1), &canvas.brush);
    target->BeginDraw();
    target->SetTextAntialiasMode(D2D1_TEXT_ANTIALIAS_MODE_CLEARTYPE);
    target->Clear(gkD2DColor(gkBackgroundOf(hwnd)));
    draw(canvas);
    HRESULT result = target->EndDraw();
    if (result == D2DERR_RECREATE_TARGET) {
        gkRenderTarget(true);
    }
    if (canvas.brush) {
        canvas.brush->Release();
    }
    return SUCCEEDED(result);
}

/* ---- Images ----

   The Windows Imaging Component encodes and decodes images. */

#include <wincodec.h>

static IWICImagingFactory *gkWIC() {
    static IWICImagingFactory *factory;
    if (!factory) {
        CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER, IID_IWICImagingFactory,
                         reinterpret_cast<void **>(&factory));
    }
    return factory;
}

std::vector<unsigned char> gkEncodePNG(const unsigned char *pixels, int width, int height) {
    std::vector<unsigned char> result;
    IWICImagingFactory *factory = gkWIC();
    if (!factory || !pixels || width <= 0 || height <= 0) {
        return result;
    }
    IStream *stream = nullptr;
    if (FAILED(CreateStreamOnHGlobal(nullptr, TRUE, &stream))) {
        return result;
    }
    IWICBitmapEncoder *encoder = nullptr;
    IWICBitmapFrameEncode *frame = nullptr;
    bool ok = SUCCEEDED(factory->CreateEncoder(GUID_ContainerFormatPng, nullptr, &encoder)) &&
              SUCCEEDED(encoder->Initialize(stream, WICBitmapEncoderNoCache)) &&
              SUCCEEDED(encoder->CreateNewFrame(&frame, nullptr)) && SUCCEEDED(frame->Initialize(nullptr)) &&
              SUCCEEDED(frame->SetSize(width, height));
    if (ok) {
        WICPixelFormatGUID format = GUID_WICPixelFormat32bppRGBA;
        frame->SetPixelFormat(&format);
        std::vector<unsigned char> data(pixels, pixels + (size_t)width * height * 4);
        if (IsEqualGUID(format, GUID_WICPixelFormat32bppBGRA)) {
            for (size_t i = 0; i < data.size(); i += 4) {
                std::swap(data[i], data[i + 2]);
            }
        }
        ok = SUCCEEDED(frame->WritePixels(height, width * 4, (UINT)data.size(), data.data())) &&
             SUCCEEDED(frame->Commit()) && SUCCEEDED(encoder->Commit());
    }
    if (ok) {
        HGLOBAL memory = nullptr;
        GetHGlobalFromStream(stream, &memory);
        STATSTG stat = {};
        stream->Stat(&stat, STATFLAG_NONAME);
        const unsigned char *bytes = static_cast<const unsigned char *>(GlobalLock(memory));
        result.assign(bytes, bytes + (size_t)stat.cbSize.QuadPart);
        GlobalUnlock(memory);
    }
    if (frame) {
        frame->Release();
    }
    if (encoder) {
        encoder->Release();
    }
    stream->Release();
    return result;
}

bool gkDecodeImage(const unsigned char *data, size_t size, std::vector<unsigned char> &pixels, int *width,
                   int *height) {
    IWICImagingFactory *factory = gkWIC();
    if (!factory || !data || !size) {
        return false;
    }
    IWICStream *stream = nullptr;
    IWICBitmapDecoder *decoder = nullptr;
    IWICBitmapFrameDecode *frame = nullptr;
    IWICFormatConverter *converter = nullptr;
    bool ok = SUCCEEDED(factory->CreateStream(&stream)) &&
              SUCCEEDED(stream->InitializeFromMemory(const_cast<BYTE *>(data), (DWORD)size)) &&
              SUCCEEDED(factory->CreateDecoderFromStream(stream, nullptr, WICDecodeMetadataCacheOnDemand, &decoder)) &&
              SUCCEEDED(decoder->GetFrame(0, &frame)) && SUCCEEDED(factory->CreateFormatConverter(&converter)) &&
              SUCCEEDED(converter->Initialize(frame, GUID_WICPixelFormat32bppRGBA, WICBitmapDitherTypeNone, nullptr, 0,
                                              WICBitmapPaletteTypeCustom));
    UINT w = 0, h = 0;
    if (ok && SUCCEEDED(converter->GetSize(&w, &h)) && w > 0 && h > 0) {
        pixels.resize((size_t)w * h * 4);
        ok = SUCCEEDED(converter->CopyPixels(nullptr, w * 4, (UINT)pixels.size(), pixels.data()));
        *width = (int)w;
        *height = (int)h;
    } else {
        ok = false;
    }
    if (converter) {
        converter->Release();
    }
    if (frame) {
        frame->Release();
    }
    if (decoder) {
        decoder->Release();
    }
    if (stream) {
        stream->Release();
    }
    return ok;
}

/* ---- Views ---- */

static std::set<GKView *> &gkLiveViews() {
    static std::set<GKView *> views;
    return views;
}

static const wchar_t *gkViewProperty = L"GKView";

void gkForgetDragSource(GKView *view);
void gkForgetDropTarget(GKView *view);
void gkForgetLabel(GKView *view);
void gkForgetAccelerator(GKView *view);
void gkForgetButton(GKView *view);
void gkForgetTextArea(GKView *view);
void gkForgetCheckBox(GKView *view);
void gkForgetProgress(GKView *view);

GKView::~GKView() {
    gkLiveViews().erase(this);
    gkForgetDragSource(this);
    gkForgetDropTarget(this);
    gkForgetLabel(this);
    gkForgetAccelerator(this);
    gkForgetButton(this);
    gkForgetTextArea(this);
    gkForgetCheckBox(this);
    gkForgetProgress(this);
    if (contextMenu) {
        gkReleaseObject(contextMenu);
    }
    if (background) {
        DeleteObject(background);
    }
    HWND windows[] = {outer, control, body};
    for (HWND hwnd : windows) {
        if (hwnd && IsWindow(hwnd) && GetPropW(hwnd, gkViewProperty) == this) {
            RemovePropW(hwnd, gkViewProperty);
        }
    }
    ext.reset();
    /* The views inside a container belong to the Go layer, which closes them
       itself; they move to the parking window so that destroying the
       container does not destroy their windows. */
    HWND containers[] = {body, outer};
    for (HWND container : containers) {
        if (!container || !IsWindow(container)) {
            continue;
        }
        for (HWND child = GetWindow(container, GW_CHILD); child;) {
            HWND next = GetWindow(child, GW_HWNDNEXT);
            GKView *inner = gkViewOf(child);
            if (inner && inner != this && gkLiveViews().count(inner)) {
                ShowWindow(child, SW_HIDE);
                SetParent(child, gkParking());
            }
            child = next;
        }
    }
    if (outer && IsWindow(outer)) {
        DestroyWindow(outer);
    }
}

GKView *gkView(void *object) {
    GKView *view = static_cast<GKView *>(object);
    return view && gkLiveViews().count(view) ? view : nullptr;
}

GKView *gkViewOf(HWND hwnd) {
    return hwnd ? static_cast<GKView *>(GetPropW(hwnd, gkViewProperty)) : nullptr;
}

GKView *gkViewFromDescendant(HWND hwnd) {
    for (HWND current = hwnd; current; current = GetParent(current)) {
        if (GKView *view = gkViewOf(current)) {
            if (gkLiveViews().count(view)) {
                return view->owner && gkLiveViews().count(view->owner) ? view->owner : view;
            }
        }
        if (!(GetWindowLongPtrW(current, GWL_STYLE) & WS_CHILD)) {
            break;
        }
    }
    return nullptr;
}

GKView *gkViewNew(HWND outer, HWND control, int kind, uintptr_t window, uintptr_t action) {
    GKView *view = new GKView();
    view->outer = outer;
    view->control = control && control != outer ? control : nullptr;
    view->kind = kind;
    view->action = action;
    view->window = gkWinFromHandle(window);
    gkLiveViews().insert(view);
    /* A new view is in its window, at the origin of the content and with no
       size, until the Go layer gives it a parent and a frame. */
    if (outer && view->window && view->window->root && GetParent(outer) == gkParking()) {
        SetParent(outer, view->window->root);
    }
    if (outer) {
        SetPropW(outer, gkViewProperty, view);
    }
    if (view->control) {
        SetPropW(view->control, gkViewProperty, view);
    }
    return view;
}

HWND gkViewTarget(GKView *view) {
    if (!view) {
        return nullptr;
    }
    return view->control ? view->control : view->outer;
}

int gkViewTheme(GKView *view) {
    if (!view) {
        return gkResolvedTheme();
    }
    GKWin *win = view->outer ? gkWinOf(view->outer) : nullptr;
    return gkWinTheme(win ? win : view->window);
}

GKPalette const &gkViewPalette(GKView *view) {
    return gkPalette(gkViewTheme(view));
}

double gkViewScaleOf(GKView *view) {
    if (!view || !view->outer) {
        return 1;
    }
    return gkScaleOf(view->outer);
}

GKFont gkViewFont(GKView *view, GKFont const &fallback) {
    GKFont font = fallback;
    if (view) {
        if (!view->style.family.empty()) {
            font.family = view->style.family;
        }
        if (view->style.size > 0) {
            font.size = (float)view->style.size;
        }
        if (view->style.weight > 0) {
            font.weight = view->style.weight;
        }
        if (view->style.italic) {
            font.italic = 1;
        }
    }
    return font;
}

GKColor gkViewTextColor(GKView *view, GKColor const &fallback) {
    return view && view->style.hasForeground ? view->style.foreground : fallback;
}

void gkInvalidate(GKView *view) {
    if (!view) {
        return;
    }
    HWND windows[] = {view->outer, view->control};
    for (HWND hwnd : windows) {
        if (hwnd) {
            InvalidateRect(hwnd, nullptr, TRUE);
        }
    }
}

void gkApplyFont(GKView *view) {
    if (!view) {
        return;
    }
    HWND target = gkViewTarget(view);
    if (!target) {
        return;
    }
    UINT dpi = GetDpiForWindow(target);
    view->font = gkFontHandle(gkViewFont(view, gkBodyFont()), dpi ? dpi : 96);
    SendMessageW(target, WM_SETFONT, (WPARAM)view->font, TRUE);
}

LRESULT gkCtlColor(HWND control, HDC hdc, UINT message) {
    GKView *view = gkViewFromDescendant(control);
    int theme = view ? gkViewTheme(view) : gkResolvedTheme();
    GKPalette const &palette = gkPalette(theme);
    GKColor back = gkBackgroundOf(control);
    GKColor text = palette.text;
    bool input = message == WM_CTLCOLOREDIT || message == WM_CTLCOLORLISTBOX;
    if (input) {
        back = palette.dark ? palette.inputActive : gkRGB(0xffffff);
    }
    if (message == WM_CTLCOLORSTATIC && GetWindowLongPtrW(control, GWL_STYLE) & ES_READONLY) {
        wchar_t name[32] = {};
        GetClassNameW(control, name, 32);
        if (!wcscmp(name, WC_EDITW)) {
            back = palette.dark ? palette.inputActive : gkRGB(0xf9f9f9);
        }
    }
    if (view && view->style.hasBackground) {
        back = gkMix(back, view->style.background);
    }
    if (view) {
        text = gkViewTextColor(view, view->enabled ? text : palette.textDisabled);
    }
    /* A part of a component can have a text color of its own. */
    if (int role = (int)(INT_PTR)GetPropW(control, L"GKTextRole")) {
        GKColor color;
        if (role == 10) {
            text = palette.dark ? gkRGB(0x000000) : gkRGB(0xffffff);
        } else if (gkSemanticColor(palette, role, &color)) {
            text = color;
        }
    }
    SetTextColor(hdc, gkColorRef(text));
    SetBkColor(hdc, gkColorRef(back));
    SetBkMode(hdc, TRANSPARENT);
    if (view) {
        if (view->background) {
            DeleteObject(view->background);
        }
        view->background = CreateSolidBrush(gkColorRef(back));
        return (LRESULT)view->background;
    }
    static std::map<COLORREF, HBRUSH> brushes;
    COLORREF key = gkColorRef(back);
    auto found = brushes.find(key);
    if (found == brushes.end()) {
        found = brushes.emplace(key, CreateSolidBrush(key)).first;
    }
    return (LRESULT)found->second;
}

void gkSend(uintptr_t handle, int kind, int a, int b, double x, double y, const char *text) {
    if (!handle) {
        return;
    }
    GKWin32Event event = {};
    event.kind = kind;
    event.a = a;
    event.b = b;
    event.x = x;
    event.y = y;
    event.text = text;
    gkWin32GoEvent(handle, &event);
}

void gkSendEvent(uintptr_t handle, GKWin32Event *event) {
    if (handle) {
        gkWin32GoEvent(handle, event);
    }
}

int gkQueryEvent(uintptr_t handle, GKWin32Event *event) {
    return handle ? gkWin32GoQuery(handle, event) : 0;
}

void gkViewSend(GKView *view, int kind, int a, int b, double x, double y, const char *text) {
    if (view) {
        gkSend(view->handle, kind, a, b, x, y, text);
    }
}

void gkViewAction(GKView *view, int event) {
    if (view && view->action && !view->suppress) {
        gkWin32UIGoEvent(view->action, event);
    }
}

/* ---- Subclassing ---- */

static LRESULT CALLBACK gkSubclassProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, UINT_PTR id,
                                       DWORD_PTR data) {
    (void)id;
    (void)data;
    GKView *view = gkViewOf(hwnd);
    if (view && gkLiveViews().count(view) && view->ext) {
        LRESULT result = 0;
        if (view->ext->message(view, hwnd, message, wparam, lparam, &result)) {
            return result;
        }
    }
    if (message == WM_NCDESTROY) {
        gkComctl().RemoveWindowSubclass(hwnd, gkSubclassProc, 1);
    }
    return gkComctl().DefSubclassProc(hwnd, message, wparam, lparam);
}

void gkSubclass(HWND hwnd) {
    if (hwnd && gkComctl().SetWindowSubclass) {
        gkComctl().SetWindowSubclass(hwnd, gkSubclassProc, 1, 0);
    }
}

/* ---- Layout ---- */

struct GKFrameRecord {
    double x = 0, y = 0, width = 0, height = 0;
};

struct GKLayoutData {
    int hasBackground = 0;
    GKColor background;
    double originX = 0, originY = 0;
    double zoom = 1;
    std::function<void(double, double)> onResize;
};

static const wchar_t *gkLayoutClass = L"GameKitLayout";
static const wchar_t *gkControlClass = L"GameKitControl";

static GKLayoutData *gkLayoutData(HWND hwnd) {
    if (!hwnd) {
        return nullptr;
    }
    wchar_t name[32] = {};
    GetClassNameW(hwnd, name, 32);
    if (wcscmp(name, gkLayoutClass)) {
        return nullptr;
    }
    return reinterpret_cast<GKLayoutData *>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
}

/* gkFrames holds the frames of the windows that a layout positions. A
   window's record goes away with the window. */
static std::unordered_map<HWND, GKFrameRecord> &gkFrames() {
    static std::unordered_map<HWND, GKFrameRecord> frames;
    return frames;
}

static GKFrameRecord *gkFrameOf(HWND hwnd) {
    auto found = gkFrames().find(hwnd);
    return found == gkFrames().end() ? nullptr : &found->second;
}

static GKFrameRecord *gkFrameFor(HWND hwnd) {
    static size_t inserts;
    if (++inserts % 256 == 0) {
        for (auto it = gkFrames().begin(); it != gkFrames().end();) {
            it = IsWindow(it->first) ? std::next(it) : gkFrames().erase(it);
        }
    }
    return &gkFrames()[hwnd];
}

double gkScaleOf(HWND hwnd) {
    UINT dpi = hwnd ? GetDpiForWindow(hwnd) : 0;
    return dpi ? dpi / 96.0 : 1;
}

D2D1_SIZE_F gkClientSize(HWND hwnd) {
    RECT client = {};
    GetClientRect(hwnd, &client);
    double scale = gkScaleOf(hwnd);
    return D2D1::SizeF((float)(client.right / scale), (float)(client.bottom / scale));
}

GKColor gkBackgroundOf(HWND hwnd) {
    for (HWND current = hwnd; current; current = GetParent(current)) {
        if (GKLayoutData *data = gkLayoutData(current)) {
            if (data->hasBackground) {
                return data->background;
            }
        }
        if (!(GetWindowLongPtrW(current, GWL_STYLE) & WS_CHILD)) {
            GKWin *win = gkWinOf(current);
            return gkPalette(win ? gkWinTheme(win) : gkResolvedTheme()).window;
        }
    }
    return gkPalette(gkResolvedTheme()).window;
}

/* While a layout pass runs, windows move without drawing, and each window
   that holds them draws once at the end of the pass. */
static int gkBatchDepth;
/* GKBatchMove is a window that moved during a pass, with the area that it
   left in its parent. */
struct GKBatchMove {
    HWND hwnd;
    HWND parent;
    RECT before;
};
static std::vector<GKBatchMove> &gkBatchMoves() {
    static std::vector<GKBatchMove> moves;
    return moves;
}

extern "C" void gkWin32BatchBegin(void) {
    gkBatchDepth++;
}

extern "C" void gkWin32BatchEnd(void) {
    if (gkBatchDepth > 0 && --gkBatchDepth == 0) {
        auto moves = std::move(gkBatchMoves());
        gkBatchMoves().clear();
        for (auto const &move : moves) {
            if (IsWindow(move.parent)) {
                InvalidateRect(move.parent, &move.before, TRUE);
            }
            if (IsWindow(move.hwnd)) {
                RedrawWindow(move.hwnd, nullptr, nullptr, RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN | RDW_FRAME);
            }
        }
    }
}

static void gkPlaceInParent(HWND hwnd, GKFrameRecord const &frame) {
    HWND parent = GetParent(hwnd);
    GKLayoutData *data = gkLayoutData(parent);
    double zoom = data ? data->zoom : 1;
    double ox = data ? data->originX : 0, oy = data ? data->originY : 0;
    double scale = gkScaleOf(parent ? parent : hwnd);
    int x = (int)lround((frame.x * zoom - ox) * scale);
    int y = (int)lround((frame.y * zoom - oy) * scale);
    int w = (int)lround(frame.width * zoom * scale);
    int h = (int)lround(frame.height * zoom * scale);
    UINT flags = SWP_NOZORDER | SWP_NOACTIVATE | SWP_NOOWNERZORDER;
    if (gkBatchDepth > 0 && parent && IsWindowVisible(hwnd)) {
        flags |= SWP_NOREDRAW;
        RECT before;
        GetWindowRect(hwnd, &before);
        MapWindowPoints(nullptr, parent, reinterpret_cast<POINT *>(&before), 2);
        auto &moves = gkBatchMoves();
        auto found = std::find_if(moves.begin(), moves.end(), [hwnd](GKBatchMove const &move) { return move.hwnd == hwnd; });
        if (found == moves.end()) {
            moves.push_back({hwnd, parent, before});
        }
    }
    SetWindowPos(hwnd, nullptr, x, y, w > 0 ? w : 0, h > 0 ? h : 0, flags);
}

void gkPlaceWindow(HWND hwnd, double x, double y, double width, double height) {
    if (!hwnd) {
        return;
    }
    GKFrameRecord *frame = gkFrameFor(hwnd);
    frame->x = x;
    frame->y = y;
    frame->width = width > 0 ? width : 0;
    frame->height = height > 0 ? height : 0;
    gkPlaceInParent(hwnd, *frame);
}

void gkPlace(GKView *view) {
    if (view && view->outer) {
        gkPlaceWindow(view->outer, view->x, view->y, view->width, view->height);
    }
}

/* gkReplaceChildren moves the children of a layout to their frames after its
   origin, zoom, or DPI changes. */
static void gkReplaceChildren(HWND layout) {
    for (HWND child = GetWindow(layout, GW_CHILD); child; child = GetWindow(child, GW_HWNDNEXT)) {
        if (GKFrameRecord *frame = gkFrameOf(child)) {
            gkPlaceInParent(child, *frame);
        }
    }
}

void gkLayoutSetOrigin(HWND layout, double x, double y) {
    if (GKLayoutData *data = gkLayoutData(layout)) {
        if (data->originX != x || data->originY != y) {
            data->originX = x;
            data->originY = y;
            gkReplaceChildren(layout);
            InvalidateRect(layout, nullptr, TRUE);
        }
    }
}

void gkLayoutSetZoom(HWND layout, double zoom) {
    if (GKLayoutData *data = gkLayoutData(layout)) {
        zoom = zoom > 0 ? zoom : 1;
        if (data->zoom == zoom) {
            return;
        }
        data->zoom = zoom;
        gkReplaceChildren(layout);
        InvalidateRect(layout, nullptr, TRUE);
    }
}

double gkLayoutZoom(HWND layout) {
    GKLayoutData *data = gkLayoutData(layout);
    return data ? data->zoom : 1;
}

void gkLayoutSetBackground(HWND layout, GKColor const *color) {
    if (GKLayoutData *data = gkLayoutData(layout)) {
        data->hasBackground = color != nullptr;
        if (color) {
            data->background = *color;
        }
        InvalidateRect(layout, nullptr, TRUE);
    }
}

void gkLayoutOnResize(HWND layout, std::function<void(double, double)> handler) {
    if (GKLayoutData *data = gkLayoutData(layout)) {
        data->onResize = std::move(handler);
    }
}

void gkReparent(HWND child, HWND parent) {
    if (!child || !parent || GetParent(child) == parent) {
        return;
    }
    SetParent(child, parent);
    if (GKFrameRecord *frame = gkFrameOf(child)) {
        gkPlaceInParent(child, *frame);
    }
}

/* gkForwardToView routes a notification that a control sends its parent to
   the view of the control. */
static bool gkForwardToView(UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) {
    switch (message) {
    case WM_COMMAND: {
        HWND from = (HWND)lparam;
        GKView *view = from ? gkViewFromDescendant(from) : nullptr;
        if (view && view->ext && view->ext->command(view, from, HIWORD(wparam))) {
            *result = 0;
            return true;
        }
        return false;
    }
    case WM_NOTIFY: {
        NMHDR *header = reinterpret_cast<NMHDR *>(lparam);
        GKView *view = header ? gkViewFromDescendant(header->hwndFrom) : nullptr;
        if (view && view->ext) {
            LRESULT value = 0;
            if (view->ext->notify(view, header, &value)) {
                *result = value;
                return true;
            }
        }
        return false;
    }
    case WM_HSCROLL:
    case WM_VSCROLL: {
        HWND from = (HWND)lparam;
        GKView *view = from ? gkViewFromDescendant(from) : nullptr;
        if (view && view->ext && view->ext->scroll(view, from, LOWORD(wparam))) {
            *result = 0;
            return true;
        }
        return false;
    }
    case WM_CTLCOLORSTATIC:
    case WM_CTLCOLORBTN:
    case WM_CTLCOLOREDIT:
    case WM_CTLCOLORLISTBOX:
        *result = gkCtlColor((HWND)lparam, (HDC)wparam, message);
        return true;
    case WM_DRAWITEM: {
        DRAWITEMSTRUCT *item = reinterpret_cast<DRAWITEMSTRUCT *>(lparam);
        GKView *view = item ? gkViewFromDescendant(item->hwndItem) : nullptr;
        if (view && view->ext) {
            LRESULT value = 0;
            if (view->ext->message(view, item->hwndItem, WM_DRAWITEM, wparam, lparam, &value)) {
                *result = TRUE;
                return true;
            }
        }
        return false;
    }
    default:
        return false;
    }
}

bool gkForwardNotification(UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) {
    return gkForwardToView(message, wparam, lparam, result);
}

static void gkFillBackground(HWND hwnd, HDC hdc) {
    RECT client;
    GetClientRect(hwnd, &client);
    GKColor color = gkBackgroundOf(hwnd);
    HBRUSH brush = CreateSolidBrush(gkColorRef(color));
    FillRect(hdc, &client, brush);
    DeleteObject(brush);
}

static LRESULT CALLBACK gkLayoutProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    LRESULT result = 0;
    if (gkForwardToView(message, wparam, lparam, &result)) {
        return result;
    }
    GKView *view = gkViewOf(hwnd);
    if (view && gkLiveViews().count(view) && view->ext && view->body != hwnd) {
        if (view->ext->message(view, hwnd, message, wparam, lparam, &result)) {
            return result;
        }
    }
    switch (message) {
    case WM_NCCREATE:
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(new GKLayoutData()));
        break;
    case WM_NCDESTROY: {
        delete gkLayoutData(hwnd);
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
        break;
    }
    case WM_ERASEBKGND:
        gkFillBackground(hwnd, (HDC)wparam);
        return 1;
    case WM_PRINTCLIENT:
        gkFillBackground(hwnd, (HDC)wparam);
        return 0;
    case WM_PAINT: {
        PAINTSTRUCT paint;
        BeginPaint(hwnd, &paint);
        EndPaint(hwnd, &paint);
        return 0;
    }
    case WM_SIZE: {
        GKLayoutData *data = gkLayoutData(hwnd);
        if (data && data->onResize) {
            double scale = gkScaleOf(hwnd);
            auto handler = data->onResize;
            handler(LOWORD(lparam) / scale, HIWORD(lparam) / scale);
        }
        break;
    }
    case WM_DPICHANGED_AFTERPARENT:
        gkReplaceChildren(hwnd);
        break;
    default:
        break;
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

HWND gkLayoutNew(HWND parent) {
    GKActivation activation;
    return CreateWindowExW(WS_EX_CONTROLPARENT, gkLayoutClass, L"",
                           WS_CHILD | WS_VISIBLE | WS_CLIPCHILDREN | WS_CLIPSIBLINGS, 0, 0, 0, 0,
                           parent ? parent : gkParking(), nullptr, gkInstance(), nullptr);
}

/* ---- Drawn controls ---- */

static LRESULT CALLBACK gkControlProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    LRESULT result = 0;
    if (gkForwardToView(message, wparam, lparam, &result)) {
        return result;
    }
    GKView *view = gkViewOf(hwnd);
    if (view && !gkLiveViews().count(view)) {
        view = nullptr;
    }
    if (view && view->ext && view->ext->message(view, hwnd, message, wparam, lparam, &result)) {
        return result;
    }
    switch (message) {
    case WM_ERASEBKGND:
        return 1;
    case WM_PRINTCLIENT: {
        /* A transparent child, such as a flat toolbar, draws over the face
           of the view. */
        RECT client;
        GetClientRect(hwnd, &client);
        gkFillBackground(hwnd, (HDC)wparam);
        if (view && view->ext) {
            view->ext->paintGDI(view, hwnd, (HDC)wparam, client);
        }
        return 0;
    }
    case WM_PAINT:
        gkPaintView(view, hwnd);
        return 0;
    case WM_MOUSEMOVE:
        if (view && !view->hover) {
            view->hover = 1;
            TRACKMOUSEEVENT track = {sizeof(track), TME_LEAVE, hwnd, 0};
            TrackMouseEvent(&track);
            InvalidateRect(hwnd, nullptr, FALSE);
        }
        break;
    case WM_MOUSELEAVE:
        if (view) {
            view->hover = 0;
            view->pressed = 0;
            InvalidateRect(hwnd, nullptr, FALSE);
        }
        break;
    case WM_LBUTTONDOWN:
        if (view && view->enabled && (GetWindowLongPtrW(hwnd, GWL_STYLE) & WS_TABSTOP)) {
            SetFocus(hwnd);
        }
        break;
    case WM_SETFOCUS:
    case WM_KILLFOCUS:
        if (view) {
            view->focused = message == WM_SETFOCUS;
            if (!view->focused) {
                view->keyboardFocus = 0;
            }
            InvalidateRect(hwnd, nullptr, FALSE);
        }
        return 0;
    case WM_GETDLGCODE:
        return DLGC_WANTARROWS;
    default:
        break;
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

GKView *gkCreateDrawn(uintptr_t window, int kind, GKExtension *ext, uintptr_t action, int focusable) {
    GKActivation activation;
    HWND hwnd = CreateWindowExW(0, gkControlClass, L"",
                                WS_CHILD | WS_VISIBLE | WS_CLIPCHILDREN | WS_CLIPSIBLINGS | (focusable ? WS_TABSTOP : 0),
                                0, 0, 0, 0, gkParking(), nullptr, gkInstance(), nullptr);
    GKView *view = gkViewNew(hwnd, nullptr, kind, window, action);
    view->ext.reset(ext);
    return view;
}

GKView *gkAdopt(uintptr_t window, HWND outer, HWND control, int kind, uintptr_t action) {
    GKView *view = gkViewNew(outer, control, kind, window, action);
    if (control && control != outer) {
        gkSubclass(control);
    } else if (outer) {
        wchar_t name[32] = {};
        GetClassNameW(outer, name, 32);
        if (wcscmp(name, gkLayoutClass) && wcscmp(name, gkControlClass)) {
            gkSubclass(outer);
        }
    }
    gkApplyFont(view);
    int theme = gkViewTheme(view);
    if (control && control != outer) {
        gkApplyThemeTo(control, theme);
    } else {
        gkApplyThemeTo(outer, theme);
    }
    return view;
}

HWND gkContainerOf(GKView *view) {
    if (!view) {
        return nullptr;
    }
    return view->body ? view->body : view->outer;
}

SIZE gkMeasureGDI(std::wstring const &text, GKFont const &font, int width, UINT flags) {
    static std::unordered_map<std::wstring, SIZE> cache;
    std::wstring key = gkMeasureKey(text, font, (float)width, flags, 0);
    if (SIZE *found = gkMeasureFind(cache, key)) {
        return *found;
    }
    /* A memory device context measures text at 96 DPI without the cost of
       getting the screen's. */
    static HDC hdc = CreateCompatibleDC(nullptr);
    HGDIOBJ previous = SelectObject(hdc, gkFontHandle(font, 96));
    RECT rect = {0, 0, width > 0 ? width : 100000, 0};
    std::wstring value = text.empty() ? std::wstring(L" ") : text;
    DrawTextW(hdc, value.c_str(), (int)value.size(), &rect,
              DT_CALCRECT | DT_NOPREFIX | (width > 0 ? DT_WORDBREAK : DT_SINGLELINE) | flags);
    SelectObject(hdc, previous);
    SIZE size = {text.empty() ? 0 : rect.right - rect.left, rect.bottom - rect.top};
    gkMeasureStore(cache, std::move(key), size);
    return size;
}

HWND gkMakeNative(const wchar_t *className, const wchar_t *text, DWORD style, DWORD exStyle, HWND parent) {
    GKActivation activation;
    return CreateWindowExW(exStyle, className, text ? text : L"", WS_CHILD | WS_VISIBLE | style, 0, 0, 0, 0,
                           parent ? parent : gkParking(), nullptr, gkInstance(), nullptr);
}

void gkReplaceOuter(GKView *view, HWND replacement) {
    HWND old = view->outer;
    if (!old || !replacement) {
        return;
    }
    HWND parent = GetParent(old);
    SetParent(replacement, parent);
    if (GKFrameRecord *frame = gkFrameOf(old)) {
        GKFrameRecord copy = *frame;
        *gkFrameFor(replacement) = copy;
        gkPlaceInParent(replacement, copy);
    }
    SetWindowPos(replacement, old, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    ShowWindow(replacement, IsWindowVisible(old) || (GetWindowLongPtrW(old, GWL_STYLE) & WS_VISIBLE) ? SW_SHOWNA
                                                                                                    : SW_HIDE);
    EnableWindow(replacement, IsWindowEnabled(old));
    RemovePropW(old, gkViewProperty);
    SetPropW(replacement, gkViewProperty, view);
    view->outer = replacement;
    if (GetFocus() == old) {
        SetFocus(replacement);
    }
    DestroyWindow(old);
}

/* ---- Parking ---- */

HWND gkParking() {
    static HWND parking;
    if (!parking || !IsWindow(parking)) {
        GKActivation activation;
        parking = CreateWindowExW(WS_EX_TOOLWINDOW | WS_EX_CONTROLPARENT, gkLayoutClass, L"GameKit",
                                  WS_POPUP | WS_CLIPCHILDREN, 0, 0, 0, 0, nullptr, nullptr, gkInstance(), nullptr);
    }
    return parking;
}

/* ---- Timers and work ---- */

static const wchar_t *gkMessageClass = L"GameKitMessages";
static const UINT gkWorkMessage = WM_APP + 1;

static std::map<UINT_PTR, std::function<bool()>> &gkTimers() {
    static std::map<UINT_PTR, std::function<bool()>> timers;
    return timers;
}

static HWND gkMessageWindow();

static LRESULT CALLBACK gkMessageProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    if (message == WM_TIMER) {
        auto found = gkTimers().find(wparam);
        if (found == gkTimers().end()) {
            KillTimer(hwnd, wparam);
            return 0;
        }
        auto tick = found->second;
        if (!tick()) {
            KillTimer(hwnd, wparam);
            gkTimers().erase(wparam);
        }
        return 0;
    }
    if (message == gkWorkMessage) {
        gkRunPending();
        return 0;
    }
    if (message == WM_SETTINGCHANGE || message == WM_THEMECHANGED || message == WM_SYSCOLORCHANGE) {
        gkApplyThemes();
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

static HWND gkMessageWindow() {
    static HWND window;
    if (!window) {
        /* A top-level window, not a message-only window, so it receives the
           broadcast of setting changes. */
        window = CreateWindowExW(WS_EX_TOOLWINDOW, gkMessageClass, L"GameKit", WS_POPUP, 0, 0, 0, 0, nullptr,
                                 nullptr, gkInstance(), nullptr);
    }
    return window;
}

UINT_PTR gkTimer(UINT milliseconds, std::function<bool()> tick) {
    static UINT_PTR next = 1000;
    UINT_PTR id = next++;
    gkTimers()[id] = std::move(tick);
    SetTimer(gkMessageWindow(), id, milliseconds ? milliseconds : 1, nullptr);
    return id;
}

void gkTimerCancel(UINT_PTR id) {
    if (!id) {
        return;
    }
    KillTimer(gkMessageWindow(), id);
    gkTimers().erase(id);
}

static std::vector<std::function<void()>> &gkPending() {
    static std::vector<std::function<void()>> pending;
    return pending;
}

static CRITICAL_SECTION *gkPendingLock() {
    static CRITICAL_SECTION lock;
    static INIT_ONCE once = INIT_ONCE_STATIC_INIT;
    InitOnceExecuteOnce(
        &once,
        [](PINIT_ONCE, PVOID, PVOID *) -> BOOL {
            InitializeCriticalSection(&lock);
            return TRUE;
        },
        nullptr, nullptr);
    return &lock;
}

static DWORD gkUIThread;

void gkLater(std::function<void()> work) {
    EnterCriticalSection(gkPendingLock());
    gkPending().push_back(std::move(work));
    LeaveCriticalSection(gkPendingLock());
    if (gkMessageWindow()) {
        PostMessageW(gkMessageWindow(), gkWorkMessage, 0, 0);
    }
}

void gkRunPending() {
    EnterCriticalSection(gkPendingLock());
    auto pending = std::move(gkPending());
    gkPending().clear();
    LeaveCriticalSection(gkPendingLock());
    for (auto const &work : pending) {
        work();
    }
}

/* ---- Message loop ---- */

bool gkWinInput(MSG const *message);
bool gkPromptKey(MSG const *message);

static int gkPumpDrain() {
    MSG message;
    int count = 0;
    for (; count < 4096 && PeekMessageW(&message, nullptr, 0, 0, PM_REMOVE); count++) {
        if (message.message == WM_QUIT) {
            /* A quit request is reported through the application events; the
               Go layer decides when the process ends. */
            continue;
        }
        if (gkPromptKey(&message) || gkWinInput(&message)) {
            continue;
        }
        TranslateMessage(&message);
        DispatchMessageW(&message);
    }
    return count;
}

void gkPumpOnce() {
    gkPumpDrain();
    gkRunPending();
}

void gkPump(std::function<bool()> const &done, double timeout) {
    ULONGLONG deadline = GetTickCount64() + (ULONGLONG)(timeout * 1000);
    while (!done() && GetTickCount64() < deadline) {
        gkPumpOnce();
        if (done()) {
            break;
        }
        MsgWaitForMultipleObjectsEx(0, nullptr, 2, QS_ALLINPUT, MWMO_INPUTAVAILABLE);
    }
}

/* ---- Initialization ---- */

HINSTANCE gkInstance() {
    return GetModuleHandleW(nullptr);
}

bool gkInitialize(char *error, size_t errorSize) {
    static bool initialized;
    if (initialized) {
        return true;
    }
    SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);
    HRESULT result = OleInitialize(nullptr);
    if (FAILED(result) && result != RPC_E_CHANGED_MODE) {
        if (error && errorSize) {
            snprintf(error, errorSize, "ui/win32: OleInitialize failed (HRESULT 0x%08lx)", (unsigned long)result);
        }
        return false;
    }
    GKActivation activation;
    GKComctl const &comctl = gkComctl();
    if (!comctl.InitCommonControlsEx || !comctl.SetWindowSubclass) {
        if (error && errorSize) {
            snprintf(error, errorSize, "ui/win32: the common controls of Windows are not available");
        }
        return false;
    }
    INITCOMMONCONTROLSEX controls = {sizeof(controls), ICC_WIN95_CLASSES | ICC_DATE_CLASSES | ICC_USEREX_CLASSES |
                                                           ICC_COOL_CLASSES | ICC_LINK_CLASS | ICC_PAGESCROLLER_CLASS |
                                                           ICC_STANDARD_CLASSES | ICC_TAB_CLASSES |
                                                           ICC_BAR_CLASSES | ICC_PROGRESS_CLASS | ICC_UPDOWN_CLASS |
                                                           ICC_HOTKEY_CLASS | ICC_LISTVIEW_CLASSES |
                                                           ICC_TREEVIEW_CLASSES};
    comctl.InitCommonControlsEx(&controls);
    LoadLibraryW(L"msftedit.dll");
    BufferedPaintInit();
    gkUxThemeInitialize();
    gkUIThread = GetCurrentThreadId();

    WNDCLASSEXW layout = {};
    layout.cbSize = sizeof(layout);
    layout.lpfnWndProc = gkLayoutProc;
    layout.hInstance = gkInstance();
    layout.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    layout.lpszClassName = gkLayoutClass;
    RegisterClassExW(&layout);

    WNDCLASSEXW control = {};
    control.cbSize = sizeof(control);
    control.style = CS_HREDRAW | CS_VREDRAW | CS_DBLCLKS;
    control.lpfnWndProc = gkControlProc;
    control.hInstance = gkInstance();
    control.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    control.lpszClassName = gkControlClass;
    RegisterClassExW(&control);

    WNDCLASSEXW messages = {};
    messages.cbSize = sizeof(messages);
    messages.lpfnWndProc = gkMessageProc;
    messages.hInstance = gkInstance();
    messages.lpszClassName = gkMessageClass;
    RegisterClassExW(&messages);
    gkMessageWindow();

    initialized = true;
    return true;
}

/* ---- Keys ---- */

int gkModifiers() {
    int mods = 0;
    if (GetKeyState(VK_SHIFT) & 0x8000) {
        mods |= GK_MOD_SHIFT;
    }
    if (GetKeyState(VK_CONTROL) & 0x8000) {
        mods |= GK_MOD_CONTROL;
    }
    if (GetKeyState(VK_MENU) & 0x8000) {
        mods |= GK_MOD_ALT;
    }
    if ((GetKeyState(VK_LWIN) | GetKeyState(VK_RWIN)) & 0x8000) {
        mods |= GK_MOD_SUPER;
    }
    if (GetKeyState(VK_CAPITAL) & 1) {
        mods |= GK_MOD_CAPS_LOCK;
    }
    if (GetKeyState(VK_NUMLOCK) & 1) {
        mods |= GK_MOD_NUM_LOCK;
    }
    return mods;
}

int gkKeyFromVirtualKey(WPARAM value, LPARAM details) {
    UINT key = (UINT)value;
    int extended = (details & (1l << 24)) != 0;
    if (key == VK_SHIFT) {
        UINT scan = (UINT)((details >> 16) & 0xff);
        key = MapVirtualKeyW(scan, MAPVK_VSC_TO_VK_EX);
    } else if (key == VK_CONTROL) {
        key = extended ? VK_RCONTROL : VK_LCONTROL;
    } else if (key == VK_MENU) {
        key = extended ? VK_RMENU : VK_LMENU;
    }
    if ((key >= 'A' && key <= 'Z') || (key >= '0' && key <= '9')) {
        return (int)key;
    }
    if (key >= VK_F1 && key <= VK_F24) {
        return 290 + (int)(key - VK_F1);
    }
    if (key >= VK_NUMPAD0 && key <= VK_NUMPAD9) {
        return 320 + (int)(key - VK_NUMPAD0);
    }
    switch (key) {
    case VK_SPACE:
        return 32;
    case VK_OEM_7:
        return 39;
    case VK_OEM_COMMA:
        return 44;
    case VK_OEM_MINUS:
        return 45;
    case VK_OEM_PERIOD:
        return 46;
    case VK_OEM_2:
        return 47;
    case VK_OEM_1:
        return 59;
    case VK_OEM_PLUS:
        return 61;
    case VK_OEM_4:
        return 91;
    case VK_OEM_5:
        return 92;
    case VK_OEM_6:
        return 93;
    case VK_OEM_3:
        return 96;
    case VK_OEM_102:
        return 162;
    case VK_ESCAPE:
        return 256;
    case VK_RETURN:
        return extended ? 335 : 257;
    case VK_TAB:
        return 258;
    case VK_BACK:
        return 259;
    case VK_INSERT:
        return 260;
    case VK_DELETE:
        return 261;
    case VK_RIGHT:
        return 262;
    case VK_LEFT:
        return 263;
    case VK_DOWN:
        return 264;
    case VK_UP:
        return 265;
    case VK_PRIOR:
        return 266;
    case VK_NEXT:
        return 267;
    case VK_HOME:
        return 268;
    case VK_END:
        return 269;
    case VK_CAPITAL:
        return 280;
    case VK_SCROLL:
        return 281;
    case VK_NUMLOCK:
        return 282;
    case VK_SNAPSHOT:
        return 283;
    case VK_PAUSE:
        return 284;
    case VK_DECIMAL:
        return 330;
    case VK_DIVIDE:
        return 331;
    case VK_MULTIPLY:
        return 332;
    case VK_SUBTRACT:
        return 333;
    case VK_ADD:
        return 334;
    case VK_LSHIFT:
        return 340;
    case VK_LCONTROL:
        return 341;
    case VK_LMENU:
        return 342;
    case VK_LWIN:
        return 343;
    case VK_RSHIFT:
        return 344;
    case VK_RCONTROL:
        return 345;
    case VK_RMENU:
        return 346;
    case VK_RWIN:
        return 347;
    case VK_APPS:
        return 348;
    default:
        return -1;
    }
}
