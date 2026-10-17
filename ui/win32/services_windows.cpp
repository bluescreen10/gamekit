//go:build windows && cgo

#include "win32_internal.h"

#include <commdlg.h>
#include <shellapi.h>
#include <shlobj.h>
#include <shlwapi.h>

#include <algorithm>

/* ---- Clipboard ----

   The clipboard is the Win32 clipboard. Text, files, and images use the
   standard formats; other MIME types are registered formats with the type
   as the name. */

static bool gkOpenClipboard() {
    for (int attempt = 0; attempt < 20; attempt++) {
        if (OpenClipboard(nullptr)) {
            return true;
        }
        Sleep(5);
    }
    return false;
}

static UINT gkPNGFormat() {
    static UINT format = RegisterClipboardFormatW(L"PNG");
    return format;
}

extern "C" long gkWin32ClipboardChangeCount(void) {
    return (long)GetClipboardSequenceNumber();
}

extern "C" void gkWin32ClipboardClear(void) {
    if (gkOpenClipboard()) {
        EmptyClipboard();
        CloseClipboard();
    }
}

static HGLOBAL gkGlobal(const void *data, size_t size) {
    HGLOBAL memory = GlobalAlloc(GMEM_MOVEABLE, size ? size : 1);
    if (memory) {
        void *target = GlobalLock(memory);
        if (size) {
            memcpy(target, data, size);
        }
        GlobalUnlock(memory);
    }
    return memory;
}

extern "C" int gkWin32ClipboardWrite(const char *text, int hasText, const char *files, size_t filesSize,
                                     const unsigned char *pixels, int width, int height, const char *types,
                                     size_t typesSize, const unsigned char *data, const size_t *sizes,
                                     int count) {
    std::vector<unsigned char> png;
    if (pixels && width > 0 && height > 0) {
        png = gkEncodePNG(pixels, width, height);
    }
    if (!gkOpenClipboard()) {
        return 0;
    }
    EmptyClipboard();
    if (hasText) {
        std::wstring wide = gkWide(text);
        SetClipboardData(CF_UNICODETEXT, gkGlobal(wide.c_str(), (wide.size() + 1) * sizeof(wchar_t)));
    }
    auto paths = gkSplit(files, filesSize);
    if (!paths.empty()) {
        std::wstring list;
        for (auto const &path : paths) {
            list += gkWide(path.c_str());
            list.push_back(L'\0');
        }
        list.push_back(L'\0');
        std::vector<unsigned char> drop(sizeof(DROPFILES) + list.size() * sizeof(wchar_t));
        DROPFILES *header = reinterpret_cast<DROPFILES *>(drop.data());
        header->pFiles = sizeof(DROPFILES);
        header->fWide = TRUE;
        memcpy(drop.data() + sizeof(DROPFILES), list.data(), list.size() * sizeof(wchar_t));
        SetClipboardData(CF_HDROP, gkGlobal(drop.data(), drop.size()));
    }
    if (pixels && width > 0 && height > 0) {
        /* A bottom-up 32-bit DIB with straight alpha, which applications
           that do not read PNG understand. */
        std::vector<unsigned char> dib(sizeof(BITMAPV5HEADER) + (size_t)width * height * 4);
        BITMAPV5HEADER *header = reinterpret_cast<BITMAPV5HEADER *>(dib.data());
        header->bV5Size = sizeof(BITMAPV5HEADER);
        header->bV5Width = width;
        header->bV5Height = height;
        header->bV5Planes = 1;
        header->bV5BitCount = 32;
        header->bV5Compression = BI_BITFIELDS;
        header->bV5RedMask = 0x00ff0000;
        header->bV5GreenMask = 0x0000ff00;
        header->bV5BlueMask = 0x000000ff;
        header->bV5AlphaMask = 0xff000000;
        header->bV5CSType = LCS_sRGB;
        header->bV5SizeImage = (DWORD)((size_t)width * height * 4);
        unsigned char *out = dib.data() + sizeof(BITMAPV5HEADER);
        for (int y = 0; y < height; y++) {
            const unsigned char *row = pixels + (size_t)(height - 1 - y) * width * 4;
            for (int x = 0; x < width; x++) {
                out[((size_t)y * width + x) * 4] = row[x * 4 + 2];
                out[((size_t)y * width + x) * 4 + 1] = row[x * 4 + 1];
                out[((size_t)y * width + x) * 4 + 2] = row[x * 4];
                out[((size_t)y * width + x) * 4 + 3] = row[x * 4 + 3];
            }
        }
        SetClipboardData(CF_DIBV5, gkGlobal(dib.data(), dib.size()));
        if (!png.empty()) {
            SetClipboardData(gkPNGFormat(), gkGlobal(png.data(), png.size()));
        }
    }
    auto names = gkSplit(types, typesSize);
    size_t offset = 0;
    for (int i = 0; i < count && i < (int)names.size(); i++) {
        UINT format = RegisterClipboardFormatW(gkWide(names[i].c_str()).c_str());
        if (format) {
            SetClipboardData(format, gkGlobal(data + offset, sizes[i]));
        }
        offset += sizes[i];
    }
    CloseClipboard();
    return 1;
}

static std::string gkFormatName(UINT format) {
    wchar_t name[256];
    int length = GetClipboardFormatNameW(format, name, 256);
    return length > 0 ? gkUTF8(name, length) : std::string();
}

/* gkDIBPixels converts a device-independent bitmap to RGBA pixels. */
static bool gkDIBPixels(const unsigned char *data, size_t size, std::vector<unsigned char> &pixels, int *width,
                        int *height) {
    if (size < sizeof(BITMAPINFOHEADER)) {
        return false;
    }
    const BITMAPINFOHEADER *header = reinterpret_cast<const BITMAPINFOHEADER *>(data);
    int w = header->biWidth, h = abs(header->biHeight);
    bool topDown = header->biHeight < 0;
    int bits = header->biBitCount;
    if (w <= 0 || h <= 0 || (bits != 24 && bits != 32)) {
        return false;
    }
    size_t offset = header->biSize;
    if (header->biCompression == BI_BITFIELDS && header->biSize == sizeof(BITMAPINFOHEADER)) {
        offset += 12;
    }
    size_t stride = ((size_t)w * bits / 8 + 3) & ~(size_t)3;
    if (offset + stride * h > size) {
        return false;
    }
    bool alpha = bits == 32 && header->biSize >= sizeof(BITMAPV5HEADER) &&
                 reinterpret_cast<const BITMAPV5HEADER *>(data)->bV5AlphaMask != 0;
    pixels.resize((size_t)w * h * 4);
    for (int y = 0; y < h; y++) {
        const unsigned char *row = data + offset + stride * (topDown ? y : h - 1 - y);
        for (int x = 0; x < w; x++) {
            const unsigned char *p = row + (size_t)x * bits / 8;
            unsigned char *out = pixels.data() + ((size_t)y * w + x) * 4;
            out[0] = p[2];
            out[1] = p[1];
            out[2] = p[0];
            out[3] = alpha ? p[3] : 255;
        }
    }
    *width = w;
    *height = h;
    return true;
}

static bool gkClipboardBytes(UINT format, std::vector<unsigned char> &bytes) {
    HANDLE handle = GetClipboardData(format);
    if (!handle) {
        return false;
    }
    void *data = GlobalLock(handle);
    if (!data) {
        return false;
    }
    size_t size = GlobalSize(handle);
    bytes.assign(static_cast<unsigned char *>(data), static_cast<unsigned char *>(data) + size);
    GlobalUnlock(handle);
    return true;
}

extern "C" void gkWin32ClipboardRead(char **text, int *hasText, char **files, size_t *filesSize,
                                     unsigned char **pixels, int *width, int *height, char **types,
                                     size_t *typesSize, unsigned char **data, size_t **sizes, int *count) {
    *text = nullptr;
    *hasText = 0;
    *files = nullptr;
    *filesSize = 0;
    *pixels = nullptr;
    *width = *height = 0;
    *types = nullptr;
    *typesSize = 0;
    *data = nullptr;
    *sizes = nullptr;
    *count = 0;
    if (!gkOpenClipboard()) {
        return;
    }
    std::vector<unsigned char> bytes;
    if (IsClipboardFormatAvailable(CF_UNICODETEXT) && gkClipboardBytes(CF_UNICODETEXT, bytes)) {
        const wchar_t *wide = reinterpret_cast<const wchar_t *>(bytes.data());
        size_t length = wcsnlen(wide, bytes.size() / sizeof(wchar_t));
        *text = gkStrdup(gkUTF8(wide, (int)length).c_str());
        *hasText = 1;
    }
    if (IsClipboardFormatAvailable(CF_HDROP)) {
        HDROP drop = static_cast<HDROP>(GetClipboardData(CF_HDROP));
        if (drop) {
            std::vector<std::string> paths;
            UINT total = DragQueryFileW(drop, 0xFFFFFFFF, nullptr, 0);
            for (UINT i = 0; i < total; i++) {
                UINT length = DragQueryFileW(drop, i, nullptr, 0);
                std::wstring path(length + 1, L'\0');
                DragQueryFileW(drop, i, path.data(), length + 1);
                path.resize(length);
                paths.push_back(gkUTF8(path.c_str()));
            }
            if (!paths.empty()) {
                *files = gkJoin(paths, filesSize);
            }
        }
    }
    std::vector<unsigned char> image;
    int imageWidth = 0, imageHeight = 0;
    std::vector<unsigned char> encoded;
    std::vector<unsigned char> dib;
    bool havePNG = IsClipboardFormatAvailable(gkPNGFormat()) && gkClipboardBytes(gkPNGFormat(), encoded);
    bool haveDIB = !havePNG && (IsClipboardFormatAvailable(CF_DIBV5) ? gkClipboardBytes(CF_DIBV5, dib)
                                                                      : IsClipboardFormatAvailable(CF_DIB) &&
                                                                            gkClipboardBytes(CF_DIB, dib));
    std::vector<std::string> names;
    std::vector<unsigned char> custom;
    std::vector<size_t> customSizes;
    for (UINT format = EnumClipboardFormats(0); format; format = EnumClipboardFormats(format)) {
        if (format < 0xC000) {
            continue;
        }
        std::string name = gkFormatName(format);
        if (name.find('/') == std::string::npos) {
            continue;
        }
        std::vector<unsigned char> content;
        if (gkClipboardBytes(format, content)) {
            names.push_back(name);
            customSizes.push_back(content.size());
            custom.insert(custom.end(), content.begin(), content.end());
        }
    }
    CloseClipboard();
    if (havePNG) {
        gkDecodeImage(encoded.data(), encoded.size(), image, &imageWidth, &imageHeight);
    } else if (haveDIB) {
        gkDIBPixels(dib.data(), dib.size(), image, &imageWidth, &imageHeight);
    }
    if (!image.empty()) {
        *pixels = static_cast<unsigned char *>(malloc(image.size()));
        memcpy(*pixels, image.data(), image.size());
        *width = imageWidth;
        *height = imageHeight;
    }
    if (!names.empty()) {
        *types = gkJoin(names, typesSize);
        *data = static_cast<unsigned char *>(malloc(custom.size() ? custom.size() : 1));
        memcpy(*data, custom.data(), custom.size());
        *sizes = static_cast<size_t *>(malloc(customSizes.size() * sizeof(size_t)));
        memcpy(*sizes, customSizes.data(), customSizes.size() * sizeof(size_t));
        *count = (int)names.size();
    }
}

extern "C" char *gkWin32ClipboardFormats(size_t *size) {
    *size = 0;
    if (!gkOpenClipboard()) {
        return nullptr;
    }
    std::vector<std::string> names;
    auto add = [&names](std::string const &name) {
        if (std::find(names.begin(), names.end(), name) == names.end()) {
            names.push_back(name);
        }
    };
    for (UINT format = EnumClipboardFormats(0); format; format = EnumClipboardFormats(format)) {
        if (format == CF_UNICODETEXT || format == CF_TEXT || format == CF_OEMTEXT) {
            add("text/plain");
        } else if (format == CF_HDROP) {
            add("text/uri-list");
        } else if (format == CF_DIB || format == CF_DIBV5 || format == CF_BITMAP || format == gkPNGFormat()) {
            add("image/png");
        } else if (format >= 0xC000) {
            std::string name = gkFormatName(format);
            if (name.find('/') != std::string::npos) {
                add(name);
            }
        }
    }
    CloseClipboard();
    return names.empty() ? nullptr : gkJoin(names, size);
}

/* ---- File manager and documents ---- */

extern "C" int gkWin32RevealFile(const char *path, int select) {
    if (!path || !*path) {
        return 0;
    }
    std::wstring wide = gkWide(path);
    for (auto &c : wide) {
        if (c == L'/') {
            c = L'\\';
        }
    }
    if (GetFileAttributesW(wide.c_str()) == INVALID_FILE_ATTRIBUTES) {
        return 0;
    }
    if (select) {
        PIDLIST_ABSOLUTE item = nullptr;
        if (SUCCEEDED(SHParseDisplayName(wide.c_str(), nullptr, &item, 0, nullptr)) && item) {
            HRESULT result = SHOpenFolderAndSelectItems(item, 0, nullptr, 0);
            CoTaskMemFree(item);
            return SUCCEEDED(result);
        }
        return 0;
    }
    std::wstring folder = wide;
    if (!(GetFileAttributesW(wide.c_str()) & FILE_ATTRIBUTE_DIRECTORY)) {
        size_t slash = folder.find_last_of(L"\\");
        folder = slash == std::wstring::npos ? L"." : folder.substr(0, slash);
    }
    return (INT_PTR)ShellExecuteW(nullptr, L"explore", folder.c_str(), nullptr, nullptr, SW_SHOWNORMAL) > 32;
}

/* The recent documents of the application are a list in the local
   application data of the user, named after the executable. Windows also
   records each one in its own recent items. */
static std::wstring gkRecentFile() {
    PWSTR local = nullptr;
    std::wstring folder;
    if (SUCCEEDED(SHGetKnownFolderPath(FOLDERID_LocalAppData, 0, nullptr, &local)) && local) {
        folder = std::wstring(local) + L"\\GameKit\\Recent";
    }
    CoTaskMemFree(local);
    SHCreateDirectoryExW(nullptr, folder.c_str(), nullptr);
    wchar_t path[MAX_PATH];
    GetModuleFileNameW(nullptr, path, MAX_PATH);
    std::wstring name = PathFindFileNameW(path);
    return folder + L"\\" + name + L".txt";
}

static std::vector<std::string> gkRecentRead() {
    std::vector<std::string> paths;
    FILE *file = _wfopen(gkRecentFile().c_str(), L"rb");
    if (!file) {
        return paths;
    }
    std::string line;
    for (int c = fgetc(file); c != EOF; c = fgetc(file)) {
        if (c == '\n') {
            if (!line.empty()) {
                paths.push_back(line);
            }
            line.clear();
        } else {
            line += (char)c;
        }
    }
    if (!line.empty()) {
        paths.push_back(line);
    }
    fclose(file);
    return paths;
}

static void gkRecentWrite(std::vector<std::string> const &paths) {
    FILE *file = _wfopen(gkRecentFile().c_str(), L"wb");
    if (!file) {
        return;
    }
    for (auto const &path : paths) {
        fwrite(path.data(), 1, path.size(), file);
        fputc('\n', file);
    }
    fclose(file);
}

extern "C" void gkWin32RecentAdd(const char *path) {
    if (!path || !*path) {
        return;
    }
    auto paths = gkRecentRead();
    paths.erase(std::remove(paths.begin(), paths.end(), std::string(path)), paths.end());
    paths.insert(paths.begin(), path);
    if (paths.size() > 100) {
        paths.resize(100);
    }
    gkRecentWrite(paths);
    SHAddToRecentDocs(SHARD_PATHW, gkWide(path).c_str());
}

extern "C" void gkWin32RecentRemove(const char *path) {
    if (!path || !*path) {
        return;
    }
    auto paths = gkRecentRead();
    paths.erase(std::remove(paths.begin(), paths.end(), std::string(path)), paths.end());
    gkRecentWrite(paths);
}

extern "C" char *gkWin32RecentList(size_t *size) {
    *size = 0;
    auto paths = gkRecentRead();
    return paths.empty() ? nullptr : gkJoin(paths, size);
}

extern "C" void gkWin32RecentClear(void) {
    gkRecentWrite({});
}

extern "C" char *gkWin32DefaultApplication(const char *extension) {
    std::wstring value = gkWide(extension);
    if (value.empty()) {
        return gkStrdup("");
    }
    if (value[0] != L'.') {
        value = L"." + value;
    }
    wchar_t name[512];
    DWORD length = 512;
    if (FAILED(AssocQueryStringW(ASSOCF_INIT_IGNOREUNKNOWN, ASSOCSTR_FRIENDLYAPPNAME, value.c_str(), nullptr, name,
                                 &length))) {
        return gkStrdup("");
    }
    return gkStrdup(gkUTF8(name).c_str());
}

/* ---- Printing ----

   Printing uses the Windows print dialog and draws the pages with GDI. The
   Go layer measures pages in points. */

extern "C" int gkWin32GoPrintPageCount(uintptr_t handle);
extern "C" int gkWin32GoPrintPage(uintptr_t handle, int page, double width, double height);

static HDC gkPrintDC;

extern "C" int gkWin32Print(uintptr_t handle, const char *title, int showDialog, void *window) {
    GKWin *win = static_cast<GKWin *>(window);
    PRINTDLGEXW setup = {};
    setup.lStructSize = sizeof(setup);
    setup.hwndOwner = win && win->hwnd && IsWindowVisible(win->hwnd) ? win->hwnd : GetForegroundWindow();
    setup.Flags = PD_RETURNDC | PD_NOPAGENUMS | PD_NOSELECTION | PD_NOCURRENTPAGE | PD_USEDEVMODECOPIESANDCOLLATE;
    if (!showDialog) {
        setup.Flags |= PD_RETURNDEFAULT;
    }
    setup.nStartPage = START_PAGE_GENERAL;
    if (FAILED(PrintDlgExW(&setup)) || !setup.hDC ||
        (showDialog && setup.dwResultAction != PD_RESULT_PRINT)) {
        if (setup.hDC) {
            DeleteDC(setup.hDC);
        }
        if (setup.hDevMode) {
            GlobalFree(setup.hDevMode);
        }
        if (setup.hDevNames) {
            GlobalFree(setup.hDevNames);
        }
        return -1;
    }
    HDC dc = setup.hDC;
    int pages = gkWin32GoPrintPageCount(handle);
    pages = pages > 1 ? pages : 1;
    std::wstring name = title && *title ? gkWide(title) : L"GameKit";
    DOCINFOW document = {};
    document.cbSize = sizeof(document);
    document.lpszDocName = name.c_str();
    int printed = -1;
    if (StartDocW(dc, &document) > 0) {
        double width = GetDeviceCaps(dc, HORZRES) * 72.0 / GetDeviceCaps(dc, LOGPIXELSX);
        double height = GetDeviceCaps(dc, VERTRES) * 72.0 / GetDeviceCaps(dc, LOGPIXELSY);
        printed = pages;
        for (int page = 0; page < pages; page++) {
            StartPage(dc);
            gkPrintDC = dc;
            int ok = gkWin32GoPrintPage(handle, page, width, height);
            gkPrintDC = nullptr;
            EndPage(dc);
            if (!ok) {
                AbortDoc(dc);
                printed = -1;
                break;
            }
        }
        if (printed >= 0) {
            EndDoc(dc);
        }
    }
    DeleteDC(dc);
    if (setup.hDevMode) {
        GlobalFree(setup.hDevMode);
    }
    if (setup.hDevNames) {
        GlobalFree(setup.hDevNames);
    }
    return printed;
}

extern "C" void gkWin32PrintImage(const unsigned char *pixels, int width, int height, double x, double y, double w,
                                  double h) {
    if (!gkPrintDC || !pixels || width <= 0 || height <= 0) {
        return;
    }
    /* GDI ignores alpha, so the image is composed over white paper. */
    std::vector<unsigned char> bgr((size_t)width * height * 4);
    for (size_t i = 0; i < (size_t)width * height; i++) {
        unsigned int alpha = pixels[i * 4 + 3];
        for (int channel = 0; channel < 3; channel++) {
            unsigned int value = pixels[i * 4 + 2 - channel];
            bgr[i * 4 + channel] = (unsigned char)((value * alpha + 255 * (255 - alpha)) / 255);
        }
        bgr[i * 4 + 3] = 255;
    }
    BITMAPINFO info = {};
    info.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    info.bmiHeader.biWidth = width;
    info.bmiHeader.biHeight = -height;
    info.bmiHeader.biPlanes = 1;
    info.bmiHeader.biBitCount = 32;
    info.bmiHeader.biCompression = BI_RGB;
    double sx = GetDeviceCaps(gkPrintDC, LOGPIXELSX) / 72.0, sy = GetDeviceCaps(gkPrintDC, LOGPIXELSY) / 72.0;
    SetStretchBltMode(gkPrintDC, HALFTONE);
    StretchDIBits(gkPrintDC, (int)lround(x * sx), (int)lround(y * sy), (int)lround(w * sx), (int)lround(h * sy), 0, 0,
                  width, height, bgr.data(), &info, DIB_RGB_COLORS, SRCCOPY);
}

extern "C" int gkWin32PrintText(const char *text, double x, double y, const char *family, double size, int weight,
                                int italic) {
    if (!gkFontFamilyInstalled(family)) {
        return 0;
    }
    if (!gkPrintDC) {
        return 1;
    }
    double sx = GetDeviceCaps(gkPrintDC, LOGPIXELSX) / 72.0, sy = GetDeviceCaps(gkPrintDC, LOGPIXELSY) / 72.0;
    std::wstring face = family && *family && strcmp(family, "System") != 0
                            ? (strcmp(family, "Monospace") == 0 ? L"Consolas" : gkWide(family))
                            : L"Segoe UI";
    HFONT font = CreateFontW(-(int)lround((size > 0 ? size : 12) * sy), 0, 0, 0, weight > 0 ? weight : FW_NORMAL,
                             italic ? TRUE : FALSE, FALSE, FALSE, DEFAULT_CHARSET, OUT_DEFAULT_PRECIS,
                             CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY, DEFAULT_PITCH, face.c_str());
    HGDIOBJ previous = SelectObject(gkPrintDC, font);
    SetBkMode(gkPrintDC, TRANSPARENT);
    SetTextColor(gkPrintDC, RGB(0, 0, 0));
    std::wstring wide = gkWide(text);
    RECT area = {(LONG)lround(x * sx), (LONG)lround(y * sy), GetDeviceCaps(gkPrintDC, HORZRES),
                 GetDeviceCaps(gkPrintDC, VERTRES)};
    DrawTextW(gkPrintDC, wide.c_str(), (int)wide.size(), &area, DT_LEFT | DT_TOP | DT_NOCLIP | DT_NOPREFIX);
    SelectObject(gkPrintDC, previous);
    DeleteObject(font);
    return 1;
}

