//go:build windows && cgo

#include "win32_internal.h"

#include <d2d1_3.h>
#include <math.h>
#include <richedit.h>
#include <shellapi.h>
#include <shlobj.h>
#include <shlwapi.h>
#include <stdio.h>

/* ---- Icon ----

   An icon is a symbol or an image that the bridge draws, as Windows has no
   control for one. */

struct GKIconView : GKExtension {
    GKIcon *icon = nullptr;
    int width = 0, height = 0;
    int tint = 0;
    ~GKIconView() override {
        if (icon) {
            gkReleaseObject(icon);
        }
    }
    void paint(GKView *view, GKCanvas &canvas) override {
        if (!icon) {
            return;
        }
        GKPalette const &palette = gkViewPalette(view);
        GKColor color = gkViewTextColor(view, palette.text);
        gkSemanticColor(palette, tint, &color);
        float size = canvas.width < canvas.height ? canvas.width : canvas.height;
        gkDrawIcon(canvas, icon, (canvas.width - size) / 2, (canvas.height - size) / 2, size, color);
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF((float)(width > 0 ? width : 16), (float)(height > 0 ? height : 16));
    }
};

extern "C" void *gkWin32IconCreate(uintptr_t window) {
    GKView *view = gkCreateDrawn(window, GK_KIND_ICON, new GKIconView(), 0, 0);
    view->accessibleHidden = 1;
    return view;
}

extern "C" void gkWin32IconSet(void *pointer, void *image, int width, int height, int tint, const char *name) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_ICON) {
        return;
    }
    auto state = static_cast<GKIconView *>(view->ext.get());
    if (image) {
        gkRetainObject(static_cast<GKObject *>(image));
    }
    if (state->icon) {
        gkReleaseObject(state->icon);
    }
    state->icon = static_cast<GKIcon *>(image);
    state->width = width;
    state->height = height;
    state->tint = tint;
    view->accessibleName = name ? name : "";
    view->accessibleHidden = view->accessibleName.empty();
    gkInvalidate(view);
}

/* ---- Documents ----

   Rich text and Markdown are a rich edit control in a text box. Markdown
   becomes RTF, so its links are RTF hyperlink fields, and it has no frame,
   like text. */

struct GKDocument : GKField {
    int markdown = 0;
    std::string source;
    std::vector<std::pair<std::wstring, std::wstring>> links; /* label and URL of each Markdown link */

    bool paintGDI(GKView *view, HWND hwnd, HDC hdc, RECT const &client) override {
        if (!markdown) {
            return GKField::paintGDI(view, hwnd, hdc, client);
        }
        GKColor fill = gkBackgroundOf(hwnd);
        HBRUSH brush = CreateSolidBrush(RGB((int)(fill.r * 255), (int)(fill.g * 255), (int)(fill.b * 255)));
        FillRect(hdc, &client, brush);
        DeleteObject(brush);
        return true;
    }
    void layout(GKView *view) override {
        if (!markdown) {
            GKField::layout(view);
            return;
        }
        RECT client;
        GetClientRect(view->outer, &client);
        SetWindowPos(edit, nullptr, 0, 0, client.right, client.bottom, SWP_NOZORDER | SWP_NOACTIVATE);
    }
    bool command(GKView *view, HWND from, int code) override {
        if (from == edit && code == EN_CHANGE) {
            changed(view);
            return true;
        }
        return false;
    }
    bool notify(GKView *view, NMHDR *header, LRESULT *result) override;
    void themeChanged(GKView *view) override;
    void colors(GKView *view);
};

static GKDocument *gkDocumentOf(GKView *view) {
    if (!view || (view->kind != GK_KIND_DOCUMENT && view->kind != GK_KIND_TEXT_AREA)) {
        return nullptr;
    }
    return dynamic_cast<GKDocument *>(view->ext.get());
}

static COLORREF gkColorRef(GKColor const &color) {
    return RGB((int)lround(color.r * 255), (int)lround(color.g * 255), (int)lround(color.b * 255));
}

/* colors gives the rich edit control the background and the default text
   color of the theme. */
void GKDocument::colors(GKView *view) {
    GKPalette const &palette = gkViewPalette(view);
    GKColor fill = markdown ? gkBackgroundOf(view->outer) : palette.dark ? palette.inputActive : gkRGB(0xffffff);
    SendMessageW(edit, EM_SETBKGNDCOLOR, 0, gkColorRef(fill));
    if (!markdown) {
        CHARFORMAT2W format = {};
        format.cbSize = sizeof(format);
        format.dwMask = CFM_COLOR;
        format.crTextColor = gkColorRef(gkViewTextColor(view, palette.text));
        view->suppress++;
        SendMessageW(edit, EM_SETCHARFORMAT, SCF_DEFAULT, (LPARAM)&format);
        /* Text without a color of its own takes the system color, which
           the palette replaces. */
        if (palette.dark) {
            SendMessageW(edit, EM_SETCHARFORMAT, SCF_ALL, (LPARAM)&format);
        }
        view->suppress--;
    }
}

int gkWin32MarkdownApply(GKView *view, GKDocument *document);

void GKDocument::themeChanged(GKView *view) {
    GKField::themeChanged(view);
    colors(view);
    if (markdown && !source.empty()) {
        gkWin32MarkdownApply(view, this);
    }
}

/* gkDocumentLink returns the URL of the link in a range of a document: the
   field instruction of an RTF hyperlink, the URL of a Markdown link with
   the text, or the text. */
static std::wstring gkDocumentLink(GKDocument *document, CHARRANGE range) {
    std::wstring text(range.cpMax > range.cpMin ? range.cpMax - range.cpMin + 1 : 1, L'\0');
    TEXTRANGEW request = {range, &text[0]};
    LRESULT length = SendMessageW(document->edit, EM_GETTEXTRANGE, 0, (LPARAM)&request);
    text.resize(length > 0 ? (size_t)length : 0);
    size_t first = text.find(L'"');
    size_t last = text.rfind(L'"');
    if (text.find(L"HYPERLINK") != std::wstring::npos && first != std::wstring::npos && last > first) {
        return text.substr(first + 1, last - first - 1);
    }
    for (auto const &link : document->links) {
        if (link.first == text) {
            return link.second;
        }
    }
    return text;
}

bool GKDocument::notify(GKView *view, NMHDR *header, LRESULT *result) {
    if (header->hwndFrom != edit || header->code != EN_LINK) {
        return false;
    }
    ENLINK *link = reinterpret_cast<ENLINK *>(header);
    if (link->msg != WM_LBUTTONUP) {
        *result = 0;
        return true;
    }
    CHARRANGE selection = {};
    SendMessageW(edit, EM_EXGETSEL, 0, (LPARAM)&selection);
    if (selection.cpMax != selection.cpMin) {
        *result = 0;
        return true;
    }
    std::wstring url = gkDocumentLink(this, link->chrg);
    std::string value = gkUTF8(url);
    GKWin32Event event = {};
    event.kind = GK_EVENT_LINK;
    event.text = value.c_str();
    if (!view->handle || !gkQueryEvent(view->handle, &event)) {
        gkOpenURLWide(url, 0);
    }
    *result = 1;
    return true;
}

extern "C" void *gkWin32DocumentCreate(uintptr_t window, int editable, int markdown) {
    auto document = new GKDocument();
    document->multiline = 1;
    document->report = 1;
    document->markdown = markdown;
    GKView *view = gkCreateDrawn(window, markdown ? GK_KIND_DOCUMENT : GK_KIND_TEXT_AREA, document, 0, 0);
    DWORD style = WS_CHILD | WS_VISIBLE | WS_TABSTOP | WS_VSCROLL | ES_MULTILINE | ES_WANTRETURN | ES_AUTOVSCROLL |
                  ES_NOHIDESEL | (editable ? 0 : ES_READONLY);
    document->edit = gkMakeNative(MSFTEDIT_CLASS, L"", style, 0, view->outer);
    view->control = document->edit;
    view->readOnly = !editable;
    SetPropW(document->edit, L"GKView", view);
    gkSubclass(document->edit);
    gkApplyFont(view);
    gkApplyThemeTo(document->edit, gkViewTheme(view));
    SendMessageW(document->edit, EM_SETEVENTMASK, 0, ENM_CHANGE | ENM_LINK);
    SendMessageW(document->edit, EM_SETTEXTMODE, TM_RICHTEXT | TM_MULTILEVELUNDO | TM_MULTICODEPAGE, 0);
    SendMessageW(document->edit, EM_AUTOURLDETECT, markdown ? 0 : AURL_ENABLEURL, 0);
    /* The rich edit control sends the notifications of the links only to a
       parent that asks for them, which the view's window is. */
    if (markdown) {
        RECT margins = {0, 0, 0, 0};
        SendMessageW(document->edit, EM_SETRECTNP, 0, (LPARAM)&margins);
        SendMessageW(document->edit, EM_SHOWSCROLLBAR, SB_VERT, FALSE);
    } else {
        SendMessageW(document->edit, EM_SETMARGINS, EC_LEFTMARGIN | EC_RIGHTMARGIN, MAKELPARAM(4, 4));
    }
    document->colors(view);
    return view;
}

extern "C" char *gkWin32DocumentText(void *pointer) {
    GKDocument *document = gkDocumentOf(gkView(pointer));
    if (!document) {
        return gkStrdup("");
    }
    GETTEXTLENGTHEX measure = {GTL_NUMCHARS | GTL_PRECISE, 1200};
    LRESULT length = SendMessageW(document->edit, EM_GETTEXTLENGTHEX, (WPARAM)&measure, 0);
    std::wstring text((size_t)(length > 0 ? length : 0) + 1, L'\0');
    GETTEXTEX request = {(DWORD)(text.size() * sizeof(wchar_t)), GT_DEFAULT, 1200, nullptr, nullptr};
    length = SendMessageW(document->edit, EM_GETTEXTEX, (WPARAM)&request, (LPARAM)&text[0]);
    text.resize(length > 0 ? (size_t)length : 0);
    /* The control separates paragraphs with carriage returns. */
    std::wstring value;
    for (size_t i = 0; i < text.size(); i++) {
        if (text[i] == L'\r') {
            value += L'\n';
            if (i + 1 < text.size() && text[i + 1] == L'\n') {
                i++;
            }
        } else {
            value += text[i];
        }
    }
    return gkStrdup(gkUTF8(value).c_str());
}

extern "C" void gkWin32DocumentSetText(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    GKDocument *document = gkDocumentOf(view);
    if (!document) {
        return;
    }
    std::wstring value = gkWide(text);
    SETTEXTEX request = {ST_DEFAULT, 1200};
    view->suppress++;
    SendMessageW(document->edit, EM_SETTEXTEX, (WPARAM)&request, (LPARAM)value.c_str());
    document->colors(view);
    view->suppress--;
}

extern "C" void gkWin32DocumentSetEditable(void *pointer, int editable) {
    GKView *view = gkView(pointer);
    GKDocument *document = gkDocumentOf(view);
    if (document) {
        view->readOnly = !editable;
        SendMessageW(document->edit, EM_SETREADONLY, !editable, 0);
        gkInvalidate(view);
    }
}

extern "C" int gkWin32DocumentEditable(void *pointer) {
    GKDocument *document = gkDocumentOf(gkView(pointer));
    return document && !(GetWindowLongW(document->edit, GWL_STYLE) & ES_READONLY);
}

static DWORD CALLBACK gkStreamOut(DWORD_PTR cookie, LPBYTE buffer, LONG size, LONG *written) {
    reinterpret_cast<std::string *>(cookie)->append(reinterpret_cast<const char *>(buffer), (size_t)size);
    *written = size;
    return 0;
}

extern "C" unsigned char *gkWin32DocumentRTF(void *pointer, size_t *size) {
    *size = 0;
    GKDocument *document = gkDocumentOf(gkView(pointer));
    if (!document) {
        return nullptr;
    }
    std::string rtf;
    EDITSTREAM stream = {(DWORD_PTR)&rtf, 0, gkStreamOut};
    SendMessageW(document->edit, EM_STREAMOUT, SF_RTF, (LPARAM)&stream);
    unsigned char *result = static_cast<unsigned char *>(malloc(rtf.size() ? rtf.size() : 1));
    memcpy(result, rtf.data(), rtf.size());
    *size = rtf.size();
    return result;
}

/* gkSetRTF replaces the content of a document with RTF. */
static bool gkSetRTF(GKView *view, GKDocument *document, std::string const &rtf) {
    SETTEXTEX request = {ST_DEFAULT, CP_ACP};
    view->suppress++;
    LRESULT result = SendMessageW(document->edit, EM_SETTEXTEX, (WPARAM)&request, (LPARAM)rtf.c_str());
    view->suppress--;
    return result != 0;
}

extern "C" int gkWin32DocumentSetRTF(void *pointer, const unsigned char *rtf, size_t size) {
    GKView *view = gkView(pointer);
    GKDocument *document = gkDocumentOf(view);
    if (!document || !rtf || size < 5 || memcmp(rtf, "{\\rtf", 5) != 0) {
        return 0;
    }
    return gkSetRTF(view, document, std::string(reinterpret_cast<const char *>(rtf), size));
}

/* ---- Markdown ----

   The renderer covers CommonMark blocks (headings, paragraphs, block
   quotes, lists, code blocks, and thematic breaks) and inlines (emphasis,
   strong emphasis, code spans, and links), and writes them as RTF. */

static void gkRTFText(std::string &out, std::string const &text) {
    std::wstring wide = gkWide(text.c_str());
    for (wchar_t c : wide) {
        if (c == L'\\' || c == L'{' || c == L'}') {
            out += '\\';
            out += (char)c;
        } else if (c == L'\t') {
            out += "\\tab ";
        } else if (c < 0x80) {
            out += (char)c;
        } else {
            char unit[16];
            snprintf(unit, sizeof(unit), "\\u%d?", (int)(int16_t)c);
            out += unit;
        }
    }
}

static void gkMarkdownInline(std::string &out, std::string const &text) {
    std::string plain;
    auto flush = [&]() {
        gkRTFText(out, plain);
        plain.clear();
    };
    size_t p = 0;
    while (p < text.size()) {
        char c = text[p];
        if (c == '\\' && p + 1 < text.size() && strchr("\\`*_{}[]()#+-.!>", text[p + 1])) {
            plain += text[p + 1];
            p += 2;
            continue;
        }
        if (c == '`') {
            size_t close = text.find('`', p + 1);
            if (close != std::string::npos) {
                flush();
                out += "{\\f1 ";
                gkRTFText(out, text.substr(p + 1, close - p - 1));
                out += "}";
                p = close + 1;
                continue;
            }
        }
        if ((c == '*' || c == '_') && p + 1 < text.size() && text[p + 1] == c) {
            std::string marker(2, c);
            size_t close = text.find(marker, p + 2);
            if (close != std::string::npos && close > p + 2) {
                flush();
                out += "{\\b ";
                gkMarkdownInline(out, text.substr(p + 2, close - p - 2));
                out += "}";
                p = close + 2;
                continue;
            }
        }
        if ((c == '*' || c == '_') && p + 1 < text.size() && text[p + 1] != ' ') {
            size_t close = text.find(c, p + 1);
            if (close != std::string::npos && close > p + 1) {
                flush();
                out += "{\\i ";
                gkMarkdownInline(out, text.substr(p + 1, close - p - 1));
                out += "}";
                p = close + 1;
                continue;
            }
        }
        if (c == '[') {
            size_t labelEnd = text.find("](", p);
            size_t urlEnd = labelEnd != std::string::npos ? text.find(')', labelEnd + 2) : std::string::npos;
            if (labelEnd != std::string::npos && urlEnd != std::string::npos) {
                flush();
                std::string url = text.substr(labelEnd + 2, urlEnd - labelEnd - 2);
                out += "{\\field{\\*\\fldinst{HYPERLINK \"";
                gkRTFText(out, url);
                out += "\"}}{\\fldrslt{\\ul\\cf1 ";
                gkRTFText(out, text.substr(p + 1, labelEnd - p - 1));
                out += "}}}";
                p = urlEnd + 1;
                continue;
            }
        }
        plain += c;
        p++;
    }
    flush();
}

static void gkRTFColor(std::string &out, GKColor const &color) {
    char entry[48];
    snprintf(entry, sizeof(entry), "\\red%d\\green%d\\blue%d;", (int)lround(color.r * 255), (int)lround(color.g * 255),
             (int)lround(color.b * 255));
    out += entry;
}

/* gkMarkdownRTF converts Markdown to RTF in the colors of a palette: 1 is
   links, 2 secondary text, and 3 text. */
static std::string gkMarkdownRTF(const char *source, GKPalette const &palette) {
    std::string out = "{\\rtf1\\ansi\\deff0{\\fonttbl{\\f0 Segoe UI;}{\\f1 Cascadia Mono;}}{\\colortbl;";
    gkRTFColor(out, palette.accentText);
    gkRTFColor(out, palette.textSecondary);
    gkRTFColor(out, palette.text);
    out += "}\\cf3\\fs21 ";
    std::string text = source ? source : "";
    std::vector<std::string> lines;
    size_t start = 0;
    while (start <= text.size()) {
        size_t end = text.find('\n', start);
        lines.push_back(text.substr(start, end == std::string::npos ? std::string::npos : end - start));
        if (end == std::string::npos) {
            break;
        }
        start = end + 1;
    }
    std::string paragraph;
    bool inCode = false;
    auto flush = [&]() {
        if (!paragraph.empty()) {
            out += "\\pard\\cf3 ";
            gkMarkdownInline(out, paragraph);
            out += "\\par\n";
            paragraph.clear();
        }
    };
    for (auto line : lines) {
        while (!line.empty() && (line.back() == '\r' || line.back() == ' ')) {
            line.pop_back();
        }
        if (line.rfind("```", 0) == 0 || line.rfind("~~~", 0) == 0) {
            flush();
            inCode = !inCode;
            continue;
        }
        if (inCode) {
            out += "\\pard\\li360{\\f1 ";
            gkRTFText(out, line);
            out += "}\\par\n";
            continue;
        }
        size_t indent = line.find_first_not_of(' ');
        std::string trimmed = indent == std::string::npos ? "" : line.substr(indent);
        int heading = 0;
        while (heading < (int)trimmed.size() && trimmed[heading] == '#' && heading < 7) {
            heading++;
        }
        bool isHeading = heading > 0 && heading <= 6 && (heading == (int)trimmed.size() || trimmed[heading] == ' ');
        bool isQuote = !trimmed.empty() && trimmed[0] == '>';
        bool isBullet = trimmed.size() > 1 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') &&
                        trimmed[1] == ' ';
        size_t digits = 0;
        while (digits < trimmed.size() && isdigit((unsigned char)trimmed[digits])) {
            digits++;
        }
        bool isNumber = digits > 0 && digits + 1 < trimmed.size() && (trimmed[digits] == '.' || trimmed[digits] == ')') &&
                        trimmed[digits + 1] == ' ';
        bool isRule = trimmed == "---" || trimmed == "***" || trimmed == "___";
        if (trimmed.empty() || isHeading || isQuote || isBullet || isNumber || isRule) {
            flush();
        }
        if (trimmed.empty()) {
            continue;
        }
        if (isHeading) {
            static const int sizes[] = {40, 32, 26, 22, 21, 21};
            char header[48];
            snprintf(header, sizeof(header), "\\pard\\sb120\\sa60{\\b\\fs%d ", sizes[heading - 1]);
            out += header;
            size_t textStart = trimmed.find_first_not_of(' ', heading);
            gkMarkdownInline(out, textStart == std::string::npos ? "" : trimmed.substr(textStart));
            out += "}\\par\n";
        } else if (isQuote) {
            size_t textStart = trimmed.find_first_not_of(' ', 1);
            out += "\\pard\\li480{\\i\\cf2 ";
            gkMarkdownInline(out, textStart == std::string::npos ? "" : trimmed.substr(textStart));
            out += "}\\par\n";
        } else if (isBullet || isNumber) {
            out += "\\pard\\li480\\fi-240 ";
            if (isBullet) {
                out += "\\u8226? ";
            } else {
                gkRTFText(out, trimmed.substr(0, digits + 2));
            }
            gkMarkdownInline(out, trimmed.substr(isBullet ? 2 : digits + 2));
            out += "\\par\n";
        } else if (isRule) {
            out += "\\pard{\\cf2 \\u9472?\\u9472?\\u9472?\\u9472?\\u9472?\\u9472?\\u9472?\\u9472?}\\par\n";
        } else {
            if (!paragraph.empty()) {
                paragraph += ' ';
            }
            paragraph += trimmed;
        }
    }
    flush();
    out += "}";
    return out;
}

/* gkMarkdownLinks records the label and the URL of each link of Markdown,
   to find the URL of a link that the user clicks. */
static void gkMarkdownLinks(std::string const &text, std::vector<std::pair<std::wstring, std::wstring>> &links) {
    size_t p = 0;
    while ((p = text.find('[', p)) != std::string::npos) {
        size_t labelEnd = text.find("](", p);
        size_t urlEnd = labelEnd != std::string::npos ? text.find(')', labelEnd + 2) : std::string::npos;
        if (labelEnd == std::string::npos || urlEnd == std::string::npos) {
            break;
        }
        links.emplace_back(gkWide(text.substr(p + 1, labelEnd - p - 1).c_str()),
                           gkWide(text.substr(labelEnd + 2, urlEnd - labelEnd - 2).c_str()));
        p = urlEnd + 1;
    }
}

int gkWin32MarkdownApply(GKView *view, GKDocument *document) {
    document->links.clear();
    gkMarkdownLinks(document->source, document->links);
    std::string rtf = gkMarkdownRTF(document->source.c_str(), gkViewPalette(view));
    return gkSetRTF(view, document, rtf);
}

extern "C" int gkWin32MarkdownSet(void *pointer, const char *source) {
    GKView *view = gkView(pointer);
    GKDocument *document = gkDocumentOf(view);
    if (!document) {
        return 0;
    }
    if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, source ? source : "", -1, nullptr, 0) == 0) {
        return 0;
    }
    document->source = source ? source : "";
    return gkWin32MarkdownApply(view, document);
}

/* ---- SVG ----

   An SVG image is drawn with the SVG support of Direct2D. A raster image
   is drawn like a picture. */

/* gkSVGLength reads a length attribute of the root element of an SVG
   document, in CSS pixels. */
static double gkSVGLength(std::string const &svg, const char *name) {
    size_t root = svg.find("<svg");
    if (root == std::string::npos) {
        return 0;
    }
    size_t end = svg.find('>', root);
    std::string tag = svg.substr(root, end == std::string::npos ? std::string::npos : end - root);
    std::string key = std::string(" ") + name + "=";
    size_t at = tag.find(key);
    if (at == std::string::npos) {
        return 0;
    }
    at += key.size();
    char quote = at < tag.size() ? tag[at] : 0;
    if (quote != '"' && quote != '\'') {
        return 0;
    }
    double value = atof(tag.c_str() + at + 1);
    std::string unit = tag.substr(at + 1, 8);
    if (unit.find("pt") != std::string::npos) {
        value *= 96.0 / 72.0;
    } else if (unit.find('%') != std::string::npos) {
        value = 0;
    }
    return value;
}

static void gkSVGSize(std::string const &svg, double *width, double *height) {
    *width = gkSVGLength(svg, "width");
    *height = gkSVGLength(svg, "height");
    if (*width > 0 && *height > 0) {
        return;
    }
    size_t root = svg.find("<svg");
    size_t box = root == std::string::npos ? std::string::npos : svg.find("viewBox=", root);
    if (box != std::string::npos) {
        double x = 0, y = 0, w = 0, h = 0;
        if (sscanf(svg.c_str() + box + 9, "%lf%*[ ,]%lf%*[ ,]%lf%*[ ,]%lf", &x, &y, &w, &h) == 4) {
            *width = *width > 0 ? *width : w;
            *height = *height > 0 ? *height : h;
        }
    }
}


struct GKVector : GKExtension {
    std::string svg;
    std::vector<unsigned char> pixels;
    double width = 0, height = 0;
    ID2D1SvgDocument *document = nullptr;
    ID2D1Bitmap *bitmap = nullptr;
    ID2D1RenderTarget *owner = nullptr;
    ~GKVector() override {
        reset();
    }
    void reset() {
        if (document) {
            document->Release();
            document = nullptr;
        }
        if (bitmap) {
            bitmap->Release();
            bitmap = nullptr;
        }
        owner = nullptr;
    }
    void paint(GKView *view, GKCanvas &canvas) override {
        (void)view;
        if (width <= 0 || height <= 0) {
            return;
        }
        double scale = canvas.width / width < canvas.height / height ? canvas.width / width : canvas.height / height;
        float w = (float)(width * scale), h = (float)(height * scale);
        float x = (canvas.width - w) / 2, y = (canvas.height - h) / 2;
        if (owner != canvas.target) {
            reset();
            owner = canvas.target;
        }
        if (!pixels.empty()) {
            if (!bitmap) {
                bitmap = gkBitmapFromRGBA(canvas.target, pixels.data(), (int)width, (int)height, (int)width * 4);
            }
            if (bitmap) {
                canvas.bitmap(bitmap, x, y, w, h);
            }
            return;
        }
        ID2D1DeviceContext5 *context = nullptr;
        if (FAILED(canvas.target->QueryInterface(__uuidof(ID2D1DeviceContext5), reinterpret_cast<void **>(&context)))) {
            return;
        }
        if (!document) {
            IStream *stream = SHCreateMemStream(reinterpret_cast<const BYTE *>(svg.data()), (UINT)svg.size());
            if (stream) {
                context->CreateSvgDocument(stream, D2D1::SizeF((float)width, (float)height), &document);
                stream->Release();
            }
        }
        if (document) {
            D2D1_MATRIX_3X2_F previous;
            context->GetTransform(&previous);
            context->SetTransform(D2D1::Matrix3x2F::Scale((float)scale, (float)scale) *
                                  D2D1::Matrix3x2F::Translation(x, y) * previous);
            context->DrawSvgDocument(document);
            context->SetTransform(previous);
        }
        context->Release();
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF((float)width, (float)height);
    }
};

extern "C" void *gkWin32VectorCreate(uintptr_t window) {
    return gkCreateDrawn(window, GK_KIND_VECTOR, new GKVector(), 0, 0);
}

extern "C" int gkWin32VectorSet(void *pointer, const unsigned char *data, size_t size, const char *label,
                                double *width, double *height) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_VECTOR || !data || !size) {
        return 0;
    }
    auto vector = static_cast<GKVector *>(view->ext.get());
    std::string text(reinterpret_cast<const char *>(data), size);
    if (text.find("<svg") != std::string::npos) {
        double w = 0, h = 0;
        gkSVGSize(text, &w, &h);
        if (w <= 0 || h <= 0) {
            return 0;
        }
        vector->reset();
        vector->svg = text;
        vector->pixels.clear();
        vector->width = w;
        vector->height = h;
    } else {
        std::vector<unsigned char> pixels;
        int w = 0, h = 0;
        if (!gkDecodeImage(data, size, pixels, &w, &h)) {
            return 0;
        }
        vector->reset();
        vector->svg.clear();
        vector->pixels = std::move(pixels);
        vector->width = w;
        vector->height = h;
    }
    *width = vector->width;
    *height = vector->height;
    view->accessibleName = label ? label : "";
    gkInvalidate(view);
    return 1;
}

/* ---- Animated image ----

   An animated image draws its frames in turn, on a timer. */

struct GKAnimation : GKExtension {
    std::vector<GKIcon *> frames;
    std::vector<double> durations;
    std::vector<ID2D1Bitmap *> bitmaps;
    ID2D1RenderTarget *owner = nullptr;
    int repeats = 0;
    int index = 0;
    int playing = 0;
    UINT_PTR timer = 0;
    ~GKAnimation() override {
        if (timer) {
            gkTimerCancel(timer);
        }
        release();
        for (GKIcon *frame : frames) {
            gkReleaseObject(frame);
        }
    }
    void release() {
        for (ID2D1Bitmap *bitmap : bitmaps) {
            if (bitmap) {
                bitmap->Release();
            }
        }
        bitmaps.assign(frames.size(), nullptr);
    }
    void paint(GKView *view, GKCanvas &canvas) override {
        (void)view;
        if (frames.empty()) {
            return;
        }
        if (owner != canvas.target) {
            release();
            owner = canvas.target;
        }
        GKIcon *frame = frames[index];
        if (frame->pixels.empty() || frame->width <= 0 || frame->height <= 0) {
            return;
        }
        if (!bitmaps[index]) {
            bitmaps[index] = gkBitmapFromRGBA(canvas.target, frame->pixels.data(), frame->width, frame->height,
                                              frame->width * 4);
        }
        double scale = canvas.width / frame->width < canvas.height / frame->height ? canvas.width / frame->width
                                                                                    : canvas.height / frame->height;
        float w = (float)(frame->width * scale), h = (float)(frame->height * scale);
        if (bitmaps[index]) {
            canvas.bitmap(bitmaps[index], (canvas.width - w) / 2, (canvas.height - h) / 2, w, h);
        }
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        if (frames.empty()) {
            return D2D1::SizeF(0, 0);
        }
        return D2D1::SizeF((float)frames[0]->width, (float)frames[0]->height);
    }
};

static void gkAnimationSchedule(GKView *view) {
    auto animation = static_cast<GKAnimation *>(view->ext.get());
    double duration = animation->durations[animation->index] > 0 ? animation->durations[animation->index] : 0.1;
    if (animation->timer) {
        gkTimerCancel(animation->timer);
    }
    animation->timer = gkTimer((UINT)lround(duration * 1000), [view]() {
        if (!gkView(view)) {
            return false;
        }
        auto animation = static_cast<GKAnimation *>(view->ext.get());
        animation->timer = 0;
        if (!animation->playing) {
            return false;
        }
        if (animation->index + 1 >= (int)animation->frames.size() && !animation->repeats) {
            animation->playing = 0;
            return false;
        }
        animation->index = (animation->index + 1) % (int)animation->frames.size();
        gkInvalidate(view);
        gkAnimationSchedule(view);
        return false;
    });
}

extern "C" void *gkWin32AnimationCreate(uintptr_t window, void **images, const double *durations, int count,
                                        int repeats) {
    auto animation = new GKAnimation();
    animation->repeats = repeats != 0;
    for (int i = 0; i < count; i++) {
        GKIcon *frame = static_cast<GKIcon *>(images[i]);
        if (!frame) {
            continue;
        }
        gkRetainObject(frame);
        animation->frames.push_back(frame);
        animation->durations.push_back(durations[i]);
    }
    animation->bitmaps.assign(animation->frames.size(), nullptr);
    return gkCreateDrawn(window, GK_KIND_ANIMATION, animation, 0, 0);
}

extern "C" void gkWin32AnimationPlay(void *pointer, int play) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_ANIMATION) {
        return;
    }
    auto animation = static_cast<GKAnimation *>(view->ext.get());
    int count = (int)animation->frames.size();
    bool was = animation->playing;
    animation->playing = play != 0 && count > 1;
    if (!animation->playing) {
        if (animation->timer) {
            gkTimerCancel(animation->timer);
            animation->timer = 0;
        }
        return;
    }
    if (!was) {
        if (!animation->repeats && animation->index + 1 >= count) {
            animation->index = 0;
            gkInvalidate(view);
        }
        gkAnimationSchedule(view);
    }
}

extern "C" int gkWin32AnimationPlaying(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_ANIMATION && static_cast<GKAnimation *>(view->ext.get())->playing;
}

/* ---- Video ----

   A video plays with MFPlay of Media Foundation in a window of its own.
   Its controls are a native button and a trackbar below the picture. */

#include <mfplay.h>
#include <propvarutil.h>

static const wchar_t gkVideoClass[] = L"GameKitVideo";

struct GKVideo : GKExtension {
    IMFPMediaPlayer *player = nullptr;
    HWND picture = nullptr;
    HWND button = nullptr;
    HWND track = nullptr;
    int controls = 0;
    int seeking = 0;
    UINT_PTR timer = 0;
    ~GKVideo() override {
        if (timer) {
            gkTimerCancel(timer);
        }
        shutdown();
    }
    void shutdown() {
        if (player) {
            player->Shutdown();
            player->Release();
            player = nullptr;
        }
    }
    int playing() {
        MFP_MEDIAPLAYER_STATE state = MFP_MEDIAPLAYER_STATE_EMPTY;
        return player && SUCCEEDED(player->GetState(&state)) && state == MFP_MEDIAPLAYER_STATE_PLAYING;
    }
    double seconds(bool duration) {
        if (!player) {
            return 0;
        }
        PROPVARIANT value;
        PropVariantInit(&value);
        HRESULT result = duration ? player->GetDuration(GUID_NULL, &value) : player->GetPosition(GUID_NULL, &value);
        double seconds = SUCCEEDED(result) && value.vt == VT_I8 ? value.hVal.QuadPart / 1e7 : 0;
        PropVariantClear(&value);
        return seconds;
    }
    void sync() {
        if (!button) {
            return;
        }
        std::wstring glyph = playing() ? L"\xE769" : L"\xE768";
        wchar_t current[4] = {};
        GetWindowTextW(button, current, 4);
        if (glyph != current) {
            SetWindowTextW(button, glyph.c_str());
        }
        if (!seeking) {
            double duration = seconds(true);
            SendMessageW(track, TBM_SETRANGEMAX, FALSE, (LPARAM)(duration > 0 ? (LONG)(duration * 10) : 0));
            SendMessageW(track, TBM_SETPOS, TRUE, (LPARAM)(LONG)(seconds(false) * 10));
        }
    }
    void layout(GKView *view) override {
        RECT client;
        GetClientRect(view->outer, &client);
        double scale = gkViewScaleOf(view);
        int bar = controls ? (int)lround(32 * scale) : 0;
        SetWindowPos(picture, nullptr, 0, 0, client.right, client.bottom - bar > 0 ? client.bottom - bar : 0,
                     SWP_NOZORDER | SWP_NOACTIVATE);
        if (controls) {
            int size = bar - (int)lround(4 * scale);
            int top = client.bottom - bar + (int)lround(2 * scale);
            SetWindowPos(button, nullptr, (int)lround(4 * scale), top, size, size, SWP_NOZORDER | SWP_NOACTIVATE);
            int left = size + (int)lround(8 * scale);
            SetWindowPos(track, nullptr, left, top, client.right - left - (int)lround(4 * scale), size,
                         SWP_NOZORDER | SWP_NOACTIVATE);
        }
        if (player) {
            player->UpdateVideo();
        }
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)wparam;
        (void)lparam;
        (void)result;
        if (hwnd == view->outer && message == WM_SIZE) {
            layout(view);
        }
        return false;
    }
    bool command(GKView *view, HWND from, int code) override {
        (void)view;
        if (from != button || code != BN_CLICKED || !player) {
            return false;
        }
        if (playing()) {
            player->Pause();
        } else {
            player->Play();
        }
        sync();
        return true;
    }
    bool scroll(GKView *view, HWND from, int code) override {
        (void)view;
        if (from != track) {
            return false;
        }
        seeking = code == TB_THUMBTRACK;
        if (code != TB_THUMBTRACK && player) {
            PROPVARIANT value;
            InitPropVariantFromInt64((LONGLONG)SendMessageW(track, TBM_GETPOS, 0, 0) * 1000000, &value);
            player->SetPosition(GUID_NULL, &value);
            PropVariantClear(&value);
        }
        return true;
    }
    void themeChanged(GKView *view) override {
        if (button) {
            gkApplyThemeTo(button, gkViewTheme(view));
            gkApplyThemeTo(track, gkViewTheme(view));
        }
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(320, controls ? 212 : 180);
    }
};

static LRESULT CALLBACK gkVideoProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam) {
    if (message == WM_PAINT || message == WM_ERASEBKGND) {
        GKView *view = gkViewFromDescendant(hwnd);
        auto video = view ? dynamic_cast<GKVideo *>(view->ext.get()) : nullptr;
        PAINTSTRUCT paint;
        HDC hdc = BeginPaint(hwnd, &paint);
        if (video && video->player) {
            video->player->UpdateVideo();
        } else {
            FillRect(hdc, &paint.rcPaint, static_cast<HBRUSH>(GetStockObject(BLACK_BRUSH)));
        }
        EndPaint(hwnd, &paint);
        return message == WM_ERASEBKGND ? 1 : 0;
    }
    return DefWindowProcW(hwnd, message, wparam, lparam);
}

static std::wstring gkMediaPath(const char *source) {
    std::wstring value = gkWide(source);
    if (value.find(L"://") != std::wstring::npos) {
        return value;
    }
    wchar_t full[MAX_PATH * 4];
    DWORD length = GetFullPathNameW(value.c_str(), MAX_PATH * 4, full, nullptr);
    return length ? std::wstring(full, length) : value;
}

extern "C" void *gkWin32VideoCreate(uintptr_t window, const char *source, int controls, int autoplay, int muted) {
    static ATOM atom;
    if (!atom) {
        WNDCLASSEXW info = {};
        info.cbSize = sizeof(info);
        info.lpfnWndProc = gkVideoProc;
        info.hInstance = gkInstance();
        info.hCursor = LoadCursorW(nullptr, IDC_ARROW);
        info.lpszClassName = gkVideoClass;
        atom = RegisterClassExW(&info);
    }
    auto video = new GKVideo();
    video->controls = controls != 0;
    GKView *view = gkCreateDrawn(window, GK_KIND_VIDEO, video, 0, 0);
    video->picture = CreateWindowExW(0, gkVideoClass, L"", WS_CHILD | WS_VISIBLE | WS_CLIPSIBLINGS, 0, 0, 0, 0,
                                     view->outer, nullptr, gkInstance(), nullptr);
    if (controls) {
        video->button = gkMakeNative(WC_BUTTONW, L"\xE768", WS_CHILD | WS_VISIBLE | WS_TABSTOP | BS_PUSHBUTTON, 0,
                                     view->outer);
        video->track = gkMakeNative(TRACKBAR_CLASSW, L"", WS_CHILD | WS_VISIBLE | WS_TABSTOP | TBS_NOTICKS | TBS_BOTH, 0,
                                    view->outer);
        SetPropW(video->button, L"GKView", view);
        SetPropW(video->track, L"GKView", view);
        gkSubclass(video->button);
        gkSubclass(video->track);
        UINT dpi = GetDpiForWindow(view->outer);
        GKFont symbol;
        symbol.family = gkSymbolFamily();
        symbol.size = 12;
        SendMessageW(video->button, WM_SETFONT, (WPARAM)gkFontHandle(symbol, dpi ? dpi : 96), TRUE);
        SetWindowTextW(video->button, L"\xE768");
        video->themeChanged(view);
    }
    std::wstring path = gkMediaPath(source);
    if (SUCCEEDED(MFPCreateMediaPlayer(path.c_str(), autoplay != 0, 0, nullptr, video->picture, &video->player)) &&
        muted) {
        video->player->SetMute(TRUE);
    }
    video->timer = gkTimer(250, [view]() {
        if (!gkView(view)) {
            return false;
        }
        static_cast<GKVideo *>(view->ext.get())->sync();
        return true;
    });
    return view;
}

static GKVideo *gkVideoOf(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_VIDEO ? static_cast<GKVideo *>(view->ext.get()) : nullptr;
}

extern "C" void gkWin32VideoPlay(void *pointer, int play) {
    GKVideo *video = gkVideoOf(pointer);
    if (!video || !video->player) {
        return;
    }
    if (play) {
        video->player->Play();
    } else {
        video->player->Pause();
    }
    video->sync();
}

extern "C" int gkWin32VideoPlaying(void *pointer) {
    GKVideo *video = gkVideoOf(pointer);
    return video && video->playing();
}

extern "C" double gkWin32VideoPosition(void *pointer) {
    GKVideo *video = gkVideoOf(pointer);
    return video ? video->seconds(false) : 0;
}

extern "C" double gkWin32VideoDuration(void *pointer) {
    GKVideo *video = gkVideoOf(pointer);
    return video ? video->seconds(true) : 0;
}

extern "C" void gkWin32VideoSeek(void *pointer, double seconds) {
    GKVideo *video = gkVideoOf(pointer);
    if (!video || !video->player) {
        return;
    }
    PROPVARIANT value;
    InitPropVariantFromInt64((LONGLONG)((seconds > 0 ? seconds : 0) * 1e7), &value);
    video->player->SetPosition(GUID_NULL, &value);
    PropVariantClear(&value);
}

extern "C" void gkWin32VideoRelease(void *pointer) {
    GKVideo *video = gkVideoOf(pointer);
    if (video) {
        video->shutdown();
    }
}

/* ---- Web view ----

   A web view is WebView2 with the WebView2 Runtime that Windows includes.
   The bridge loads the runtime from where Windows installs it, as the
   loader library of the WebView2 SDK does, so the program needs no library
   beside it. Its data folder is in the local application data of the user,
   because the folder of the executable can be read-only. The interfaces
   are declared here with the methods the bridge calls in the places of
   their vtables. */

#define GK_WV_SLOT(n) virtual HRESULT STDMETHODCALLTYPE slot##n() = 0

struct GKWebSettings : IUnknown {
    GK_WV_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE put_IsScriptEnabled(BOOL enabled) = 0;
};

struct GKWebCore : IUnknown {
    virtual HRESULT STDMETHODCALLTYPE get_Settings(GKWebSettings **settings) = 0;
    virtual HRESULT STDMETHODCALLTYPE get_Source(LPWSTR *uri) = 0;
    virtual HRESULT STDMETHODCALLTYPE Navigate(LPCWSTR uri) = 0;
    virtual HRESULT STDMETHODCALLTYPE NavigateToString(LPCWSTR html) = 0;
};

struct GKWebController : IUnknown {
    GK_WV_SLOT(0);
    virtual HRESULT STDMETHODCALLTYPE put_IsVisible(BOOL visible) = 0;
    GK_WV_SLOT(1);
    virtual HRESULT STDMETHODCALLTYPE put_Bounds(RECT bounds) = 0;
    GK_WV_SLOT(2);
    GK_WV_SLOT(3);
    GK_WV_SLOT(4);
    GK_WV_SLOT(5);
    GK_WV_SLOT(6);
    GK_WV_SLOT(7);
    GK_WV_SLOT(8);
    GK_WV_SLOT(9);
    GK_WV_SLOT(10);
    GK_WV_SLOT(11);
    GK_WV_SLOT(12);
    GK_WV_SLOT(13);
    GK_WV_SLOT(14);
    GK_WV_SLOT(15);
    GK_WV_SLOT(16);
    GK_WV_SLOT(17);
    virtual HRESULT STDMETHODCALLTYPE NotifyParentWindowPositionChanged() = 0;
    virtual HRESULT STDMETHODCALLTYPE Close() = 0;
    virtual HRESULT STDMETHODCALLTYPE get_CoreWebView2(GKWebCore **core) = 0;
};

struct GKWebEnvironment : IUnknown {
    virtual HRESULT STDMETHODCALLTYPE CreateCoreWebView2Controller(HWND parent, IUnknown *handler) = 0;
};

static const IID gkEnvironmentHandlerID = {0x4e8a3389, 0xc9d8, 0x4bd2, {0xb6, 0xb5, 0x12, 0x4f, 0xee, 0x6c, 0xc1, 0x4d}};
static const IID gkControllerHandlerID = {0x6c4819f3, 0xc9b7, 0x4260, {0x81, 0x27, 0xc9, 0xf5, 0xbd, 0xe7, 0xf6, 0x8c}};

/* GKWebCompletion is a completion handler of WebView2, which receives a
   result and an object. */
struct GKWebCompletion : IUnknown {
    LONG references = 1;
    IID id;
    std::function<void(HRESULT, IUnknown *)> work;
    GKWebCompletion(IID const &id, std::function<void(HRESULT, IUnknown *)> work) : id(id), work(std::move(work)) {}
    virtual ~GKWebCompletion() = default;
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
        if (IsEqualIID(riid, IID_IUnknown) || IsEqualIID(riid, id)) {
            *object = this;
            AddRef();
            return S_OK;
        }
        *object = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override {
        return InterlockedIncrement(&references);
    }
    ULONG STDMETHODCALLTYPE Release() override {
        LONG count = InterlockedDecrement(&references);
        if (!count) {
            delete this;
        }
        return count;
    }
    virtual HRESULT STDMETHODCALLTYPE Invoke(HRESULT result, IUnknown *object) {
        work(result, object);
        return S_OK;
    }
};

typedef HRESULT(STDMETHODCALLTYPE *GKCreateWebEnvironment)(bool checkRunningInstance, int runtimeType,
                                                            PCWSTR userDataFolder, IUnknown *options,
                                                            IUnknown *handler);

/* gkWebRuntime returns the library of the WebView2 Runtime, or null when
   Windows does not have it. */
static HMODULE gkWebRuntime() {
    static HMODULE module;
    static bool tried;
    if (tried) {
        return module;
    }
    tried = true;
#if defined(__aarch64__) || defined(_M_ARM64)
    const wchar_t *arch = L"arm64";
#elif defined(__x86_64__) || defined(_M_X64)
    const wchar_t *arch = L"x64";
#else
    const wchar_t *arch = L"x86";
#endif
    static const wchar_t key[] =
        L"Software\\Microsoft\\EdgeUpdate\\ClientState\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}";
    struct {
        HKEY root;
        REGSAM view;
    } places[] = {{HKEY_LOCAL_MACHINE, KEY_WOW64_32KEY}, {HKEY_LOCAL_MACHINE, KEY_WOW64_64KEY},
                  {HKEY_CURRENT_USER, KEY_WOW64_32KEY}, {HKEY_CURRENT_USER, KEY_WOW64_64KEY}};
    for (auto const &place : places) {
        HKEY handle = nullptr;
        if (RegOpenKeyExW(place.root, key, 0, KEY_READ | place.view, &handle) != ERROR_SUCCESS) {
            continue;
        }
        wchar_t folder[MAX_PATH] = {};
        DWORD size = sizeof(folder) - sizeof(wchar_t);
        LSTATUS status = RegGetValueW(handle, nullptr, L"EBWebView", RRF_RT_REG_SZ, nullptr, folder, &size);
        RegCloseKey(handle);
        if (status != ERROR_SUCCESS || !folder[0]) {
            continue;
        }
        std::wstring path = std::wstring(folder) + L"\\EBWebView\\" + arch + L"\\EmbeddedBrowserWebView.dll";
        module = LoadLibraryExW(path.c_str(), nullptr, LOAD_WITH_ALTERED_SEARCH_PATH);
        if (module) {
            break;
        }
    }
    return module;
}

static GKWebEnvironment *gkWebEnvironmentValue;
static bool gkWebEnvironmentFailed;
static std::vector<std::function<void(GKWebEnvironment *)>> gkWebWaiting;

/* gkWebEnvironment runs work with the environment of the web views, which
   the bridge creates once; work receives null when there is none. */
static void gkWebEnvironment(std::function<void(GKWebEnvironment *)> work) {
    if (gkWebEnvironmentValue || gkWebEnvironmentFailed) {
        work(gkWebEnvironmentValue);
        return;
    }
    gkWebWaiting.push_back(std::move(work));
    if (gkWebWaiting.size() > 1) {
        return;
    }
    auto finish = [](GKWebEnvironment *environment) {
        gkWebEnvironmentValue = environment;
        gkWebEnvironmentFailed = !environment;
        auto waiting = std::move(gkWebWaiting);
        gkWebWaiting.clear();
        for (auto const &next : waiting) {
            next(environment);
        }
    };
    HMODULE runtime = gkWebRuntime();
    auto create = runtime ? reinterpret_cast<GKCreateWebEnvironment>(
                                reinterpret_cast<void *>(GetProcAddress(runtime, "CreateWebViewEnvironmentWithOptionsInternal")))
                          : nullptr;
    if (!create) {
        finish(nullptr);
        return;
    }
    std::wstring folder;
    PWSTR local = nullptr;
    if (SUCCEEDED(SHGetKnownFolderPath(FOLDERID_LocalAppData, 0, nullptr, &local)) && local) {
        folder = std::wstring(local) + L"\\GameKit\\WebView2";
    }
    CoTaskMemFree(local);
    auto handler = new GKWebCompletion(gkEnvironmentHandlerID, [finish](HRESULT result, IUnknown *object) {
        GKWebEnvironment *environment = nullptr;
        if (SUCCEEDED(result) && object) {
            object->AddRef();
            environment = static_cast<GKWebEnvironment *>(object);
        }
        finish(environment);
    });
    /* The runtime type 0 is the installed runtime. */
    HRESULT result = create(true, 0, folder.empty() ? nullptr : folder.c_str(), nullptr, handler);
    handler->Release();
    if (FAILED(result) && !gkWebWaiting.empty()) {
        finish(nullptr);
    }
}

struct GKWeb : GKExtension {
    int scripts = 1;
    GKWebController *controller = nullptr;
    GKWebCore *core = nullptr;
    std::wstring pendingURL;
    std::wstring pendingHTML;
    int pending = 0; /* 1 for a URL, 2 for HTML */
    ~GKWeb() override {
        close();
    }
    void close() {
        if (core) {
            core->Release();
            core = nullptr;
        }
        if (controller) {
            controller->Close();
            controller->Release();
            controller = nullptr;
        }
    }
    void bounds(GKView *view) {
        if (controller) {
            RECT client;
            GetClientRect(view->outer, &client);
            controller->put_Bounds(client);
        }
    }
    void flush() {
        if (!core) {
            return;
        }
        if (pending == 1) {
            core->Navigate(pendingURL.c_str());
        } else if (pending == 2) {
            core->NavigateToString(pendingHTML.c_str());
        }
        pending = 0;
    }
    bool message(GKView *view, HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam, LRESULT *result) override {
        (void)wparam;
        (void)lparam;
        (void)result;
        if (hwnd == view->outer && message == WM_SIZE) {
            bounds(view);
        }
        return false;
    }
    D2D1_SIZE_F preferred(GKView *view) override {
        (void)view;
        return D2D1::SizeF(320, 240);
    }
};

static GKWeb *gkWebOf(GKView *view) {
    return view && view->kind == GK_KIND_WEB ? static_cast<GKWeb *>(view->ext.get()) : nullptr;
}

extern "C" void *gkWin32WebViewCreate(uintptr_t window, int scripts) {
    auto web = new GKWeb();
    web->scripts = scripts != 0;
    GKView *view = gkCreateDrawn(window, GK_KIND_WEB, web, 0, 0);
    gkWebEnvironment([view](GKWebEnvironment *environment) {
        if (!environment || !gkView(view)) {
            return;
        }
        auto handler = new GKWebCompletion(gkControllerHandlerID, [view](HRESULT result, IUnknown *object) {
            GKWeb *web = gkWebOf(gkView(view));
            if (!web || FAILED(result) || !object) {
                return;
            }
            object->AddRef();
            web->controller = static_cast<GKWebController *>(object);
            web->controller->get_CoreWebView2(&web->core);
            GKWebSettings *settings = nullptr;
            if (web->core && SUCCEEDED(web->core->get_Settings(&settings)) && settings) {
                settings->put_IsScriptEnabled(web->scripts);
                settings->Release();
            }
            web->bounds(view);
            web->controller->put_IsVisible(TRUE);
            web->flush();
        });
        environment->CreateCoreWebView2Controller(view->outer, handler);
        handler->Release();
    });
    return view;
}

extern "C" int gkWin32WebViewLoad(void *pointer, const char *url) {
    GKWeb *web = gkWebOf(gkView(pointer));
    if (!web || !url) {
        return 0;
    }
    std::wstring value = gkWide(url);
    PARSEDURLW parsed = {sizeof(parsed)};
    if (FAILED(ParseURLW(value.c_str(), &parsed)) || parsed.cchProtocol == 0) {
        return 0;
    }
    web->pendingURL = value;
    web->pending = 1;
    web->flush();
    return 1;
}

extern "C" void gkWin32WebViewLoadHTML(void *pointer, const char *html) {
    GKWeb *web = gkWebOf(gkView(pointer));
    if (!web) {
        return;
    }
    web->pendingHTML = gkWide(html);
    web->pending = 2;
    web->flush();
}

extern "C" char *gkWin32WebViewURL(void *pointer) {
    GKWeb *web = gkWebOf(gkView(pointer));
    if (!web) {
        return gkStrdup("");
    }
    if (!web->core) {
        return gkStrdup(web->pending == 1 ? gkUTF8(web->pendingURL).c_str() : "");
    }
    LPWSTR source = nullptr;
    std::string value;
    if (SUCCEEDED(web->core->get_Source(&source)) && source) {
        value = gkUTF8(source);
        CoTaskMemFree(source);
    }
    return gkStrdup(value.c_str());
}

extern "C" void gkWin32WebViewRelease(void *pointer) {
    GKWeb *web = gkWebOf(gkView(pointer));
    if (web) {
        web->close();
    }
}
