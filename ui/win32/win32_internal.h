/* Internal declarations of the Win32 bridge. The bridge is C++17 over the
   Win32 API and the common controls (version 6, with visual styles), which
   ship with Windows, so an application needs nothing else installed. Views
   are native controls. The few components that Windows has no control for
   are drawn with Direct2D and the visual style parts of Windows. */

#ifndef GAMEKIT_WIN32_INTERNAL_H
#define GAMEKIT_WIN32_INTERNAL_H

#ifndef UNICODE
#define UNICODE
#endif
#ifndef _UNICODE
#define _UNICODE
#endif
#ifndef NOMINMAX
#define NOMINMAX
#endif
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#ifndef _USE_MATH_DEFINES
#define _USE_MATH_DEFINES
#endif

#include <windows.h>
#include <windowsx.h>
#include <commctrl.h>
#include <d2d1.h>
#include <dwrite.h>
#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <functional>
#include <memory>
#include <string>
#include <unordered_map>
#include <vector>

extern "C" {
#include "win32_bridge.h"
#include "widget_bridge.h"
#include "window_bridge.h"
}

/* ---- Objects ---- */

/* GKObject is the base of every native object that the Go layer holds. The
   object that a create function returns has one reference, which belongs to
   the Go layer. */
struct GKObject {
    int refs = 1;
    uintptr_t handle = 0;
    virtual ~GKObject() = default;
};

void gkRetainObject(GKObject *object);
void gkReleaseObject(GKObject *object);

/* GKIcon is an icon of the Go layer: a glyph of the symbol font or RGBA
   pixels. */
struct GKIcon : GKObject {
    std::wstring glyph;
    std::vector<unsigned char> pixels;
    int width = 0, height = 0;
    double pointSize = 0;
};

/* ---- Strings ---- */

std::wstring gkWide(const char *text);
std::string gkUTF8(const wchar_t *text, int length = -1);
std::string gkUTF8(std::wstring const &text);
/* gkStrdup returns a malloc'd copy of text that the Go layer frees. */
char *gkStrdup(const char *text);
char *gkStrdupW(std::wstring const &text);
/* gkSplit splits a NUL-separated buffer. */
std::vector<std::string> gkSplit(const char *buffer, size_t size);
/* gkJoin joins strings into a malloc'd NUL-separated buffer. */
char *gkJoin(std::vector<std::string> const &values, size_t *size);

/* ---- Theme ----

   GKPalette holds the colors of the Windows 11 design for the light or dark
   theme. Colors are straight (not premultiplied) RGBA. */

struct GKColor {
    float r = 0, g = 0, b = 0, a = 1;
};

GKColor gkRGB(unsigned int rgb, float alpha = 1);
GKColor gkMix(GKColor below, GKColor above);

struct GKPalette {
    GKColor window;         /* the background of a window */
    GKColor layer;          /* a raised surface, such as a card */
    GKColor layerStroke;    /* the border of a raised surface */
    GKColor text;           /* primary text */
    GKColor textSecondary;  /* secondary text */
    GKColor textTertiary;   /* placeholders */
    GKColor textDisabled;   /* disabled text */
    GKColor textOnAccent;   /* text on the accent color */
    GKColor control;        /* the fill of a control at rest */
    GKColor controlHover;   /* the fill of a control under the pointer */
    GKColor controlPressed; /* the fill of a pressed control */
    GKColor controlDisabled;
    GKColor controlStroke;       /* the border of a control */
    GKColor controlStrokeBottom; /* the lower edge of a control's border */
    GKColor controlStrong;       /* the outline of an unchecked box or a track */
    GKColor controlStrongDisabled;
    GKColor inputActive; /* the fill of a focused text box */
    GKColor subtle;      /* the fill of a subtle control under the pointer */
    GKColor subtlePressed;
    GKColor accent;
    GKColor accentHover;
    GKColor accentPressed;
    GKColor accentDisabled;
    GKColor accentText; /* text in the accent color, such as a link */
    GKColor divider;
    GKColor focus;   /* the outer ring of the focus rectangle */
    GKColor success; /* the severity colors */
    GKColor caution;
    GKColor critical;
    GKColor information;
    GKColor successBackground;
    GKColor cautionBackground;
    GKColor criticalBackground;
    GKColor informationBackground;
    GKColor selection; /* the background of selected list items */
    GKColor flyout;    /* the background of a popup */
    int dark = 0;
};

/* gkPalette returns the palette of a theme: 1 light, 2 dark. */
GKPalette const &gkPalette(int theme);
/* gkSystemDark reports whether apps use the dark theme in the system
   settings. */
int gkSystemDark();
/* gkAccentColor returns the accent color of the system. */
GKColor gkAccentColor();
/* gkSemanticColor returns the color of a ui.SemanticColor, if it has one. */
bool gkSemanticColor(GKPalette const &palette, int color, GKColor *out);

/* ---- Fonts ---- */

/* GKFont describes the font of text in logical units (DIPs). */
struct GKFont {
    std::wstring family;
    float size = 14;
    int weight = 400;
    int italic = 0;
};

/* gkBodyFont returns the font of body text. */
GKFont gkBodyFont();
/* gkTypeRamp returns a font of the Windows type ramp: 0 body, 1 caption,
   2 body strong, 3 subtitle, 4 title, 5 title large, 6 display, 7 icon. */
GKFont gkTypeRamp(int style);
/* gkSymbolFamily returns the family of the symbol font. */
std::wstring const &gkSymbolFamily();
/* gkFontHandle returns a GDI font for a native control at dpi. The bridge
   owns the font. */
HFONT gkFontHandle(GKFont const &font, UINT dpi);
/* gkFontFamilyInstalled reports whether a font family is installed. */
int gkFontFamilyInstalled(const char *family);

/* ---- Direct2D ---- */

ID2D1Factory *gkD2D();
IDWriteFactory *gkDWrite();
IDWriteTextFormat *gkTextFormat(GKFont const &font);

/* GKCanvas draws with Direct2D on the device context of a window during
   WM_PAINT, in logical units. */
struct GKCanvas {
    ID2D1DCRenderTarget *target = nullptr;
    ID2D1SolidColorBrush *brush = nullptr;
    float width = 0, height = 0; /* the size of the window in logical units */
    float scale = 1;             /* physical pixels in a logical unit */
    void color(GKColor const &color);
    void fillRect(float x, float y, float w, float h, GKColor const &color);
    void fillRound(float x, float y, float w, float h, float radius, GKColor const &color);
    void strokeRound(float x, float y, float w, float h, float radius, GKColor const &color, float thickness = 1);
    void fillEllipse(float cx, float cy, float rx, float ry, GKColor const &color);
    void strokeEllipse(float cx, float cy, float rx, float ry, GKColor const &color, float thickness = 1);
    void line(float x1, float y1, float x2, float y2, GKColor const &color, float thickness = 1);
    void arc(float cx, float cy, float radius, float start, float sweep, GKColor const &color, float thickness);
    /* text draws text in a box. align: 0 leading, 1 center, 2 trailing.
       valign: 0 top, 1 center, 2 bottom. */
    void text(std::wstring const &text, GKFont const &font, float x, float y, float w, float h,
              GKColor const &color, int align = 0, int valign = 1, int wrap = 0, int trimming = 1);
    /* centeredText draws one line of text centered in a box by the ink of
       its glyphs, for short text such as a count in a badge. */
    void centeredText(std::wstring const &text, GKFont const &font, float x, float y, float w, float h,
                      GKColor const &color);
    /* glyph draws one glyph of the symbol font centered in a box. */
    void glyph(std::wstring const &glyph, float size, float x, float y, float w, float h, GKColor const &color);
    void bitmap(ID2D1Bitmap *bitmap, float x, float y, float w, float h, float opacity = 1, int nearest = 0);
    /* control draws the fill and border of a standard control. */
    void control(float x, float y, float w, float h, float radius, GKColor const &fill, GKPalette const &palette,
                 int accent);
    /* focusRect draws the focus rectangle around a box. */
    void focusRect(float x, float y, float w, float h, float radius, GKPalette const &palette);
    void clip(float x, float y, float w, float h);
    void unclip();
};

/* gkMeasureText measures text in logical units. width is the width at which
   it wraps, or 0 for no wrapping. */
D2D1_SIZE_F gkMeasureText(std::wstring const &text, GKFont const &font, float width = 0, int maximumLines = 0);

/* gkBitmapFromRGBA creates a Direct2D bitmap for a render target. */
ID2D1Bitmap *gkBitmapFromRGBA(ID2D1RenderTarget *target, const unsigned char *pixels, int width, int height,
                              int stride);

/* ---- Windows ---- */

struct GKView;

/* GKChrome is a bar above or below the content of a window: a toolbar, a
   status bar, or an info bar. */
struct GKChrome {
    GKView *view = nullptr;
    int top = 1;
    int height = 0;
};

struct GKWin : GKObject {
    HWND hwnd = nullptr;
    HWND root = nullptr; /* the layout that holds the content */
    GKWin *owner = nullptr;
    GKView *focus = nullptr;
    HMENU menubar = nullptr;
    std::vector<GKChrome> chrome;
    int shouldClose = 0;
    int utility = 0;
    int borderless = 0;
    int resizable = 1;
    int alwaysOnTop = 0;
    int transparent = 0;
    int havePosition = 0;
    int x = 0, y = 0;
    int minWidth = 0, minHeight = 0, maxWidth = 0, maxHeight = 0;
    int cursorMode = GK_CURSOR_NORMAL;
    double virtualX = 0, virtualY = 0;
    int lastX = 0, lastY = 0;
    int haveLast = 0;
    int warping = 0;
    double scrollX = 0, scrollY = 0;
    int fullscreen = 0;
    RECT windowedRect = {};
    LONG_PTR windowedStyle = 0;
    int shown = 0;
    /* pendingFullscreen is a fullscreen request of a window that has not
       been shown, which it enters when it is shown. */
    int pendingFullscreen = 0;
    int acceptFiles = 0;
    int theme = 0;
    int resolvedTheme = 0;
    int destroyed = 0;
    int active = 0;
    int tracking = 0; /* the window tracks the pointer leaving it */
    int inside = 0;
    unsigned int buttons = 0;
    unsigned int highSurrogate = 0;
    int minimized = 0;
    int reportedWidth = -1, reportedHeight = -1;
    UINT dpi = 96;
};

GKWin *gkWinFromHandle(uintptr_t window);
/* gkWinOf returns the window that holds hwnd. */
GKWin *gkWinOf(HWND hwnd);
void gkWinNotify(GKWin *win, int kind, int a, double x, double y, const char *text);
/* gkWinScale returns the number of physical pixels in one logical unit. */
double gkWinScale(GKWin *win);
/* gkWinTheme returns the theme of a window: 1 light, 2 dark. */
int gkWinTheme(GKWin *win);
/* gkWinLayout positions the chrome and the content of a window. */
void gkWinLayout(GKWin *win);
/* gkWinAddChrome puts a view above (top != 0) or below the content. */
void gkWinAddChrome(GKWin *win, GKView *view, int top, int height);
void gkWinRemoveChrome(GKWin *win, GKView *view);
std::vector<GKWin *> &gkWindows();
/* gkPump processes messages until done returns true or timeout seconds
   pass. */
void gkPump(std::function<bool()> const &done, double timeout);
/* gkPumpOnce processes the pending messages without waiting. */
void gkPumpOnce();
int gkResolvedTheme();
void gkApplyThemes();
/* gkInitialize prepares the calling thread for the user interface. */
bool gkInitialize(char *error, size_t errorSize);
/* gkInstance returns the module of the program. */
HINSTANCE gkInstance();

/* ---- Views ---- */

enum {
    GK_KIND_GENERIC = 0,
    GK_KIND_LAYOUT,
    GK_KIND_LABEL,
    GK_KIND_BUTTON,
    GK_KIND_TEXT_FIELD,
    GK_KIND_TEXT_AREA,
    GK_KIND_CHECK_BOX,
    GK_KIND_RADIO_GROUP,
    GK_KIND_SELECT,
    GK_KIND_SLIDER,
    GK_KIND_PROGRESS,
    GK_KIND_ACTIVITY,
    GK_KIND_SEPARATOR,
    GK_KIND_IMAGE,
    GK_KIND_LINK,
    GK_KIND_TABS,
    GK_KIND_SCROLL,
    GK_KIND_CANVAS,
    GK_KIND_SECTION,
    GK_KIND_TOGGLE,
    GK_KIND_SWITCH,
    GK_KIND_PASSWORD,
    GK_KIND_SEARCH,
    GK_KIND_NUMBER,
    GK_KIND_COMBO,
    GK_KIND_SEGMENTED,
    GK_KIND_STEPPER,
    GK_KIND_DIAL,
    GK_KIND_RANGE,
    GK_KIND_DATE,
    GK_KIND_CALENDAR,
    GK_KIND_COLOR,
    GK_KIND_FONT,
    GK_KIND_KEY_RECORDER,
    GK_KIND_CLIP,
    GK_KIND_GROUP_BOX,
    GK_KIND_SPLIT,
    GK_KIND_TABLE,
    GK_KIND_OUTLINE,
    GK_KIND_COLLECTION,
    GK_KIND_BREADCRUMBS,
    GK_KIND_PAGINATION,
    GK_KIND_STATUS_BAR,
    GK_KIND_COMMAND_BAR,
    GK_KIND_INFO_BAR,
    GK_KIND_BADGE,
    GK_KIND_EMPTY_STATE,
    GK_KIND_VALIDATION,
    GK_KIND_ICON,
    GK_KIND_DOCUMENT,
    GK_KIND_VECTOR,
    GK_KIND_ANIMATION,
    GK_KIND_VIDEO,
    GK_KIND_WEB,
    GK_KIND_POPOVER_BODY,
    GK_KIND_TOOLBAR,
};

/* GKStyle is the style that the Go layer sets on a view. */
struct GKStyle {
    std::wstring family;
    double size = 0;
    int weight = 0;
    int italic = 0;
    int hasForeground = 0, hasBackground = 0;
    GKColor foreground, background;
    /* The ui.SemanticColor of the foreground and the background, which the
       colors follow when the theme changes, or 0. */
    int foregroundRole = 0, backgroundRole = 0;
    double cornerRadius = 0;
};

/* gkStyleResolve applies the colors of a view's style in the theme of its
   window. */
void gkStyleResolve(GKView *view);

/* GKExtension holds the state and the behavior of a kind of view. A view
   that the bridge draws has an extension that paints it and handles its
   input. */
struct GKExtension {
    virtual ~GKExtension() = default;
    /* paintGDI draws the view with GDI and the visual styles of Windows. It
       returns false to draw with Direct2D through paint. */
    virtual bool paintGDI(GKView *view, HWND hwnd, HDC hdc, RECT const &client) {
        (void)view;
        (void)hwnd;
        (void)hdc;
        (void)client;
        return false;
    }
    /* paint draws the view. */
    virtual void paint(GKView *view, GKCanvas &canvas) {
        (void)view;
        (void)canvas;
    }
    /* message handles a message of the view's window. It returns true when
       it handled the message, with the result in result. */
    virtual bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) {
        (void)view;
        (void)hwnd;
        (void)message;
        (void)wparam;
        (void)lparam;
        (void)result;
        return false;
    }
    /* command handles a WM_COMMAND notification from the view's control. */
    virtual bool command(GKView *view, HWND from, int code) {
        (void)view;
        (void)from;
        (void)code;
        return false;
    }
    /* notify handles a WM_NOTIFY notification from the view's control. */
    virtual bool notify(GKView *view, NMHDR *header, LRESULT *result) {
        (void)view;
        (void)header;
        (void)result;
        return false;
    }
    /* scroll handles WM_HSCROLL or WM_VSCROLL from a trackbar or an up-down
       control of the view. */
    virtual bool scroll(GKView *view, HWND from, int code) {
        (void)view;
        (void)from;
        (void)code;
        return false;
    }
    /* preferred returns the preferred size of the view in logical units, or
       a negative width to use the default. */
    virtual D2D1_SIZE_F preferred(GKView *view) {
        (void)view;
        return D2D1::SizeF(-1, -1);
    }
    /* layout positions the windows inside the view after it changes size. */
    virtual void layout(GKView *view) {
        (void)view;
    }
    /* themeChanged updates the view for a new theme. */
    virtual void themeChanged(GKView *view) {
        (void)view;
    }
};

struct GKView : GKObject {
    /* outer is the window that the parent layout positions. */
    HWND outer = nullptr;
    /* control is the window that takes focus and input, if not outer. */
    HWND control = nullptr;
    /* body is the layout that holds the children of a container. */
    HWND body = nullptr;
    GKWin *window = nullptr;
    /* owner is the view that a part of another view, such as the header of
       a section, belongs to; focus and accessibility report the owner. */
    GKView *owner = nullptr;
    int kind = GK_KIND_GENERIC;
    uintptr_t action = 0;
    int suppress = 0;
    /* The frame in logical units, relative to the parent layout. */
    int x = 0, y = 0, width = 0, height = 0;
    /* placed is set once the Go layer has given the view a frame. */
    int placed = 0;
    int visible = 1;
    int enabled = 1;
    int hover = 0;
    int pressed = 0;
    int focused = 0;
    int keyboardFocus = 0; /* focus that the keyboard moved, which shows a focus rectangle */
    std::wstring text;
    std::wstring placeholder;
    std::string url;
    std::string accessibleID;
    std::string accessibleName;
    std::string accessibleDescription;
    std::string accessibleValue;
    int accessibleHidden = 0;
    int accessibleRequired = 0;
    int accessibleInvalid = 0;
    std::wstring tooltip;
    int cursor = 0;
    int checked = 0;
    int selected = -1;
    int alignment = 0;
    int readOnly = 0;
    int running = 0;
    double value = 0, minimum = 0, maximum = 1, step = 0;
    int axes = 0;
    int canvasButtons = 0;
    int frames = 0;
    int preferredWidth = 0;
    int scaling = 0;
    int nearest = 0;
    double sourceScale = 1;
    int vertical = 0;
    int inverted = 0;
    int animated = 1;
    GKStyle style;
    /* font is the GDI font of the view's native control at the current
       DPI. The font cache owns it. */
    HFONT font = nullptr;
    /* background is a brush of the color behind the view, for the
       WM_CTLCOLOR messages of its control. */
    HBRUSH background = nullptr;
    std::unique_ptr<GKExtension> ext;
    /* contextMenu is the menu that the view shows on a secondary click; the
       view holds a reference to it. */
    GKObject *contextMenu = nullptr;
    ~GKView() override;
};

/* gkView returns the view that an opaque pointer of the Go layer names, or
   null if it is not a live view. */
GKView *gkView(void *object);
/* gkViewOf returns the view of a window, without its ancestors. */
GKView *gkViewOf(HWND hwnd);
/* gkViewFromDescendant returns the view of hwnd or its nearest ancestor. */
GKView *gkViewFromDescendant(HWND hwnd);
/* gkViewNew creates a view for a window and registers it. */
GKView *gkViewNew(HWND outer, HWND control, int kind, uintptr_t window, uintptr_t action);
/* gkCreate creates a view of the bridge's control class with ext, in the
   parking window of the application until the Go layer gives it a
   parent. */
GKView *gkCreateDrawn(uintptr_t window, int kind, GKExtension *ext, uintptr_t action, int focusable);
/* gkAdopt registers a native window as a view. */
GKView *gkAdopt(uintptr_t window, HWND outer, HWND control, int kind, uintptr_t action);
/* gkViewSend reports a widget-contract event through the view's handle. */
void gkViewSend(GKView *view, int kind, int a, int b, double x, double y, const char *text);
/* gkViewAction reports a GK_UI_EVENT to the view's action handle. */
void gkViewAction(GKView *view, int event);
/* gkViewTarget returns the window of view that takes focus and input. */
HWND gkViewTarget(GKView *view);
/* gkViewTheme returns the theme of the window that holds a view. */
int gkViewTheme(GKView *view);
GKPalette const &gkViewPalette(GKView *view);
/* gkViewScale returns the physical pixels in one logical unit for a view. */
double gkViewScaleOf(GKView *view);
/* gkViewFont returns the font of a view: its style over a default. */
GKFont gkViewFont(GKView *view, GKFont const &fallback);
/* gkViewTextColor returns the text color of a view: its style over a
   default. */
GKColor gkViewTextColor(GKView *view, GKColor const &fallback);
/* gkApplyFont gives the native controls of a view the view's font at the
   DPI of its window. */
void gkApplyFont(GKView *view);
/* gkApplyThemeTo gives a native control the visual style of the theme of
   its window. */
void gkApplyThemeTo(HWND hwnd, int theme, const wchar_t *lightClass = nullptr);
/* gkCtlColor answers a WM_CTLCOLOR message for a control. */
LRESULT gkCtlColor(HWND control, HDC hdc, UINT message);
/* gkSetTextRole gives a native label the text color of a ui.SemanticColor,
   or 10 for text on an inverted background. */
inline void gkSetTextRole(HWND control, int role) {
    SetPropW(control, L"GKTextRole", reinterpret_cast<HANDLE>((INT_PTR)role));
}
/* gkSubclass routes messages of a native control to its view's extension and
   reports its focus, so the window input and the focus of the view work. */
void gkSubclass(HWND hwnd);
/* gkReplaceOuter replaces the window of a view with another window at the
   same place, for a change that a native control cannot make in place. */
void gkReplaceOuter(GKView *view, HWND replacement);
/* gkContainerOf returns the layout that holds the children of a view. */
HWND gkContainerOf(GKView *view);
/* gkMeasureGDI measures text with the font of a native control, in logical
   units. width is the width at which the text wraps, or 0. */
SIZE gkMeasureGDI(std::wstring const &text, GKFont const &font, int width = 0, UINT flags = 0);
/* gkMakeNative creates a native control in the parking window. */
HWND gkMakeNative(const wchar_t *className, const wchar_t *text, DWORD style, DWORD exStyle, HWND parent = nullptr);
/* GKField is a text box: an edit control in a frame that the bridge draws
   with the visual style of Windows, with optional labels before and after
   the text. */
struct GKField : GKExtension {
    HWND edit = nullptr;
    HWND prefix = nullptr;
    HWND suffix = nullptr;
    HWND updown = nullptr;
    int multiline = 0;
    int hover = 0;
    /* report sends the events of a component through the view's handle,
       instead of the actions of a text field. */
    int report = 0;
    /* changed runs after the text changes; submitted after Return. */
    virtual void changed(GKView *view);
    virtual void submitted(GKView *view);
    /* focusLost runs when the edit control loses the focus. */
    virtual void focusLost(GKView *view) {
        (void)view;
    }
    void layout(GKView *view) override;
    bool paintGDI(GKView *view, HWND hwnd, HDC hdc, RECT const &client) override;
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override;
    bool command(GKView *view, HWND from, int code) override;
    D2D1_SIZE_F preferred(GKView *view) override;
    void themeChanged(GKView *view) override;
};
/* gkFieldCreate creates a text box with an edit control of a style. The
   field may be a subclass of GKField. */
GKView *gkFieldCreate(uintptr_t window, int kind, DWORD editStyle, uintptr_t action, GKField *field = nullptr);
/* gkReport sends a component event through the view's handle. */
void gkReport(GKView *view, int kind, double x, double y, const char *text);
GKField *gkFieldOf(GKView *view);
/* gkInvalidate redraws a view. */
void gkInvalidate(GKView *view);
void gkSend(uintptr_t handle, int kind, int a, int b, double x, double y, const char *text);
void gkSendEvent(uintptr_t handle, GKWin32Event *event);
int gkQueryEvent(uintptr_t handle, GKWin32Event *event);

/* ---- Layout ----

   A layout is a window that holds child windows at the frames that the Go
   layout code sets, in logical units. It paints the background that its
   children draw over: the color of the window, or the color that a
   container gives it. */

/* gkLayoutNew creates a layout window. */
HWND gkLayoutNew(HWND parent);
/* gkLayoutSetBackground sets the background color of a layout, or removes it
   so that the layout shows the background of its parent. */
void gkLayoutSetBackground(HWND layout, GKColor const *color);
/* gkBackgroundOf returns the color behind a window: the background of the
   nearest layout or container that sets one, or the window color. */
GKColor gkBackgroundOf(HWND hwnd);
/* gkLayoutSetScroll sets the scroll offset of a layout's children, in
   logical units. */
void gkLayoutSetOrigin(HWND layout, double x, double y);
/* gkLayoutSetZoom scales the frames of a layout's children. */
void gkLayoutSetZoom(HWND layout, double zoom);
double gkLayoutZoom(HWND layout);
/* gkLayoutOnResize sets a function that runs when the layout changes size,
   with its size in logical units. */
void gkLayoutOnResize(HWND layout, std::function<void(double, double)> handler);
/* gkPlace moves a view's window to its frame in its parent. */
void gkPlace(GKView *view);
/* gkPlaceWindow moves a window to a frame in logical units, relative to a
   layout. */
void gkPlaceWindow(HWND hwnd, double x, double y, double width, double height);
/* gkParking returns the hidden window that holds views without a parent. */
HWND gkParking();
/* gkReparent moves a window into a new parent. */
void gkReparent(HWND child, HWND parent);

/* ---- Common controls ----

   The bridge loads the common controls at run time, after it activates
   version 6 with visual styles, because a program that links them loads
   version 5, which draws them in the classic style. */

struct GKComctl {
    BOOL(WINAPI *InitCommonControlsEx)(const INITCOMMONCONTROLSEX *) = nullptr;
    BOOL(WINAPI *SetWindowSubclass)(HWND, SUBCLASSPROC, UINT_PTR, DWORD_PTR) = nullptr;
    BOOL(WINAPI *RemoveWindowSubclass)(HWND, SUBCLASSPROC, UINT_PTR) = nullptr;
    LRESULT(WINAPI *DefSubclassProc)(HWND, UINT, WPARAM, LPARAM) = nullptr;
    HRESULT(WINAPI *TaskDialogIndirect)(const TASKDIALOGCONFIG *, int *, int *, BOOL *) = nullptr;
    HIMAGELIST(WINAPI *ImageList_Create)(int, int, UINT, int, int) = nullptr;
    int(WINAPI *ImageList_Add)(HIMAGELIST, HBITMAP, HBITMAP) = nullptr;
    int(WINAPI *ImageList_ReplaceIcon)(HIMAGELIST, int, HICON) = nullptr;
    BOOL(WINAPI *ImageList_Destroy)(HIMAGELIST) = nullptr;
    BOOL(WINAPI *ImageList_RemoveAll)(HIMAGELIST) = nullptr;
    HRESULT(WINAPI *LoadIconMetric)(HINSTANCE, PCWSTR, int, HICON *) = nullptr;
};

GKComctl const &gkComctl();
/* gkActivation activates the activation context of the common controls
   version 6 for the calling scope. */
struct GKActivation {
    ULONG_PTR cookie = 0;
    GKActivation();
    ~GKActivation();
};

/* ---- Images ---- */

/* gkEncodePNG encodes RGBA pixels as PNG. */
std::vector<unsigned char> gkEncodePNG(const unsigned char *pixels, int width, int height);
/* gkDecodeImage decodes an image in a format of Windows to RGBA pixels. */
bool gkDecodeImage(const unsigned char *data, size_t size, std::vector<unsigned char> &pixels, int *width,
                   int *height);

/* ---- Helpers ---- */

int gkModifiers();
int gkKeyFromVirtualKey(WPARAM key, LPARAM details);
/* gkScaleOf returns the physical pixels in one logical unit for a window. */
double gkScaleOf(HWND hwnd);
/* gkLogicalRect returns the client size of a window in logical units. */
D2D1_SIZE_F gkClientSize(HWND hwnd);
/* gkPaintView paints a drawn view during WM_PAINT. */
void gkPaintView(GKView *view, HWND hwnd);
/* gkCheckBoxDescription returns the description of a check box, and
   gkCheckBoxLabelBefore whether its label is before the box. */
std::wstring gkCheckBoxDescription(GKView *view);
int gkCheckBoxLabelBefore(GKView *view);
/* gkButtonIconPlacement returns the placement of the icon of a button:
   0 before, 1 after, 2 above, 3 below, 4 alone, or -1 without an icon. */
int gkButtonIconPlacement(GKView *view);
/* gkButtonPreferred returns the preferred size of a push button or a toggle
   button with its text and icon. */
D2D1_SIZE_F gkButtonPreferred(GKView *view);
/* gkButtonThemeChanged updates the icon and the badge of a button for a new
   theme. */
void gkButtonThemeChanged(GKView *view);
/* gkPaintCanvas draws with Direct2D on a device context of a window, over
   the background of the window. */
bool gkPaintCanvas(HWND hwnd, HDC hdc, RECT const &rect, std::function<void(GKCanvas &)> const &draw);
/* gkButtonCustomDraw draws a push button whose style sets a background color
   or a corner radius, which the visual style of a button cannot show. */
bool gkButtonCustomDraw(GKView *view, NMHDR *header, LRESULT *result);
/* gkTimer runs a function on the UI thread after a delay, repeating while
   it returns true. It returns an identifier for gkTimerCancel. */
UINT_PTR gkTimer(UINT milliseconds, std::function<bool()> tick);
void gkTimerCancel(UINT_PTR id);
/* gkLater runs work on the UI thread after the current message. */
void gkLater(std::function<void()> work);
void gkRunPending();
/* gkOpenURL opens a URL with the default program. */
int gkOpenURLWide(std::wstring const &url, int background);
/* gkIconBitmap returns the pixels of an icon at a size in physical pixels,
   with the color of a glyph. */
std::vector<unsigned char> gkIconPixels(GKIcon *icon, int size, GKColor const &color);
/* gkIconHandle creates an HICON from an icon. */
HICON gkIconHandle(GKIcon *icon, int size, GKColor const &color);
/* gkIconBitmap returns a 32-bit bitmap of an icon with premultiplied
   alpha. */
HBITMAP gkIconBitmap(GKIcon *icon, int size, GKColor const &color);
/* gkDrawIcon draws an icon in a box. */
void gkDrawIcon(GKCanvas &canvas, GKIcon *icon, float x, float y, float size, GKColor const &color);
/* gkSetCursorShape sets the cursor of the pointer. */
void gkSetCursorShape(int cursor);
/* gkAnnounceText reports text to assistive technology. */
void gkAnnounceText(HWND hwnd, std::wstring const &text);

#endif
