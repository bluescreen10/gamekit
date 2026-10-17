//go:build windows && cgo

#include "win32_internal.h"

#include <algorithm>

/* An icon is a glyph of the symbol font or RGBA pixels. The bridge renders
   either to RGBA pixels at the size that a control needs. */

extern "C" void *gkWin32Icon(const char *symbol, const unsigned char *pixels, int width, int height, double pointSize) {
    if ((!symbol || !*symbol) && (!pixels || width <= 0 || height <= 0)) {
        return nullptr;
    }
    GKIcon *icon = new GKIcon();
    icon->glyph = gkWide(symbol);
    icon->pointSize = pointSize;
    if (pixels && width > 0 && height > 0) {
        icon->pixels.assign(pixels, pixels + (size_t)width * height * 4);
        icon->width = width;
        icon->height = height;
    }
    return icon;
}

/* gkGlyphPixels draws a glyph of the symbol font in a square of size
   pixels with a color. GDI text has no alpha, so the glyph is drawn white on
   black and its coverage becomes the alpha of the color. */
static std::vector<unsigned char> gkGlyphPixels(std::wstring const &glyph, int size, GKColor const &color) {
    std::vector<unsigned char> result((size_t)size * size * 4, 0);
    BITMAPINFO info = {};
    info.bmiHeader.biSize = sizeof(info.bmiHeader);
    info.bmiHeader.biWidth = size;
    info.bmiHeader.biHeight = -size;
    info.bmiHeader.biPlanes = 1;
    info.bmiHeader.biBitCount = 32;
    info.bmiHeader.biCompression = BI_RGB;
    void *bits = nullptr;
    HDC screen = GetDC(nullptr);
    HBITMAP bitmap = CreateDIBSection(screen, &info, DIB_RGB_COLORS, &bits, nullptr, 0);
    ReleaseDC(nullptr, screen);
    if (!bitmap) {
        return result;
    }
    memset(bits, 0, (size_t)size * size * 4);
    HDC dc = CreateCompatibleDC(nullptr);
    HGDIOBJ previous = SelectObject(dc, bitmap);
    HFONT font = CreateFontW(-size, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE, DEFAULT_CHARSET, OUT_DEFAULT_PRECIS,
                             CLIP_DEFAULT_PRECIS, ANTIALIASED_QUALITY, DEFAULT_PITCH, gkSymbolFamily().c_str());
    HGDIOBJ previousFont = SelectObject(dc, font);
    SetBkMode(dc, TRANSPARENT);
    SetTextColor(dc, RGB(255, 255, 255));
    RECT area = {0, 0, size, size};
    DrawTextW(dc, glyph.c_str(), (int)glyph.size(), &area, DT_CENTER | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX);
    GdiFlush();
    const unsigned char *source = static_cast<const unsigned char *>(bits);
    for (int i = 0; i < size * size; i++) {
        unsigned int coverage = source[i * 4 + 1];
        result[i * 4 + 0] = (unsigned char)lroundf(color.r * 255);
        result[i * 4 + 1] = (unsigned char)lroundf(color.g * 255);
        result[i * 4 + 2] = (unsigned char)lroundf(color.b * 255);
        result[i * 4 + 3] = (unsigned char)(coverage * color.a);
    }
    SelectObject(dc, previousFont);
    SelectObject(dc, previous);
    DeleteObject(font);
    DeleteObject(bitmap);
    DeleteDC(dc);
    return result;
}

/* gkScalePixels scales RGBA pixels to fit a square of size pixels, keeping
   their aspect ratio, with bilinear filtering. */
static std::vector<unsigned char> gkScalePixels(GKIcon *icon, int size) {
    std::vector<unsigned char> result((size_t)size * size * 4, 0);
    double scale = std::min((double)size / icon->width, (double)size / icon->height);
    int w = std::max(1, (int)lround(icon->width * scale)), h = std::max(1, (int)lround(icon->height * scale));
    int ox = (size - w) / 2, oy = (size - h) / 2;
    for (int y = 0; y < h; y++) {
        double sy = (y + 0.5) / scale - 0.5;
        int y0 = std::max(0, (int)floor(sy)), y1 = std::min(icon->height - 1, y0 + 1);
        double fy = sy - floor(sy);
        if (sy < 0) {
            fy = 0;
        }
        for (int x = 0; x < w; x++) {
            double sx = (x + 0.5) / scale - 0.5;
            int x0 = std::max(0, (int)floor(sx)), x1 = std::min(icon->width - 1, x0 + 1);
            double fx = sx - floor(sx);
            if (sx < 0) {
                fx = 0;
            }
            unsigned char *out = &result[((size_t)(y + oy) * size + (x + ox)) * 4];
            const unsigned char *p00 = &icon->pixels[((size_t)y0 * icon->width + x0) * 4];
            const unsigned char *p01 = &icon->pixels[((size_t)y0 * icon->width + x1) * 4];
            const unsigned char *p10 = &icon->pixels[((size_t)y1 * icon->width + x0) * 4];
            const unsigned char *p11 = &icon->pixels[((size_t)y1 * icon->width + x1) * 4];
            /* Premultiply while filtering so transparent pixels do not darken
               the edges, then return to straight alpha. */
            double a = (p00[3] * (1 - fx) + p01[3] * fx) * (1 - fy) + (p10[3] * (1 - fx) + p11[3] * fx) * fy;
            for (int c = 0; c < 3; c++) {
                double v = (p00[c] * p00[3] * (1 - fx) + p01[c] * p01[3] * fx) * (1 - fy) +
                           (p10[c] * p10[3] * (1 - fx) + p11[c] * p11[3] * fx) * fy;
                out[c] = (unsigned char)(a > 0 ? std::min(255.0, v / a) : 0);
            }
            out[3] = (unsigned char)lround(a);
        }
    }
    return result;
}

std::vector<unsigned char> gkIconPixels(GKIcon *icon, int size, GKColor const &color) {
    if (!icon || size <= 0) {
        return std::vector<unsigned char>();
    }
    if (!icon->pixels.empty() && icon->width > 0 && icon->height > 0) {
        return gkScalePixels(icon, size);
    }
    return gkGlyphPixels(icon->glyph, size, color);
}

HICON gkIconHandle(GKIcon *icon, int size, GKColor const &color) {
    std::vector<unsigned char> pixels = gkIconPixels(icon, size, color);
    if (pixels.empty()) {
        return nullptr;
    }
    BITMAPV5HEADER header = {};
    header.bV5Size = sizeof(header);
    header.bV5Width = size;
    header.bV5Height = -size;
    header.bV5Planes = 1;
    header.bV5BitCount = 32;
    header.bV5Compression = BI_BITFIELDS;
    header.bV5RedMask = 0x00ff0000;
    header.bV5GreenMask = 0x0000ff00;
    header.bV5BlueMask = 0x000000ff;
    header.bV5AlphaMask = 0xff000000;
    void *bits = nullptr;
    HDC screen = GetDC(nullptr);
    HBITMAP color32 = CreateDIBSection(screen, reinterpret_cast<BITMAPINFO *>(&header), DIB_RGB_COLORS, &bits,
                                       nullptr, 0);
    ReleaseDC(nullptr, screen);
    if (!color32) {
        return nullptr;
    }
    unsigned char *out = static_cast<unsigned char *>(bits);
    for (int i = 0; i < size * size; i++) {
        out[i * 4 + 0] = pixels[i * 4 + 2];
        out[i * 4 + 1] = pixels[i * 4 + 1];
        out[i * 4 + 2] = pixels[i * 4 + 0];
        out[i * 4 + 3] = pixels[i * 4 + 3];
    }
    HBITMAP mask = CreateBitmap(size, size, 1, 1, nullptr);
    ICONINFO info = {};
    info.fIcon = TRUE;
    info.hbmColor = color32;
    info.hbmMask = mask;
    HICON handle = CreateIconIndirect(&info);
    DeleteObject(color32);
    DeleteObject(mask);
    return handle;
}

/* gkIconBitmap returns a 32-bit bitmap with premultiplied alpha, which menus
   and image lists draw with transparency. */
HBITMAP gkIconBitmap(GKIcon *icon, int size, GKColor const &color) {
    std::vector<unsigned char> pixels = gkIconPixels(icon, size, color);
    if (pixels.empty()) {
        return nullptr;
    }
    BITMAPINFO info = {};
    info.bmiHeader.biSize = sizeof(info.bmiHeader);
    info.bmiHeader.biWidth = size;
    info.bmiHeader.biHeight = -size;
    info.bmiHeader.biPlanes = 1;
    info.bmiHeader.biBitCount = 32;
    info.bmiHeader.biCompression = BI_RGB;
    void *bits = nullptr;
    HDC screen = GetDC(nullptr);
    HBITMAP bitmap = CreateDIBSection(screen, &info, DIB_RGB_COLORS, &bits, nullptr, 0);
    ReleaseDC(nullptr, screen);
    if (!bitmap) {
        return nullptr;
    }
    unsigned char *out = static_cast<unsigned char *>(bits);
    for (int i = 0; i < size * size; i++) {
        unsigned int a = pixels[i * 4 + 3];
        out[i * 4 + 0] = (unsigned char)(pixels[i * 4 + 2] * a / 255);
        out[i * 4 + 1] = (unsigned char)(pixels[i * 4 + 1] * a / 255);
        out[i * 4 + 2] = (unsigned char)(pixels[i * 4 + 0] * a / 255);
        out[i * 4 + 3] = (unsigned char)a;
    }
    return bitmap;
}

void gkDrawIcon(GKCanvas &canvas, GKIcon *icon, float x, float y, float size, GKColor const &color) {
    if (!icon) {
        return;
    }
    if (icon->pixels.empty()) {
        canvas.glyph(icon->glyph, size, x, y, size, size, color);
        return;
    }
    ID2D1Bitmap *bitmap = gkBitmapFromRGBA(canvas.target, icon->pixels.data(), icon->width, icon->height,
                                           icon->width * 4);
    if (bitmap) {
        double scale = std::min((double)size / icon->width, (double)size / icon->height);
        float w = (float)(icon->width * scale), h = (float)(icon->height * scale);
        canvas.bitmap(bitmap, x + (size - w) / 2, y + (size - h) / 2, w, h);
        bitmap->Release();
    }
}
