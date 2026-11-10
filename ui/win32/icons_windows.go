package win32

import (
	"math"
	"unsafe"
)

// An icon is a glyph of the symbol font or RGBA pixels. The backend renders
// either to RGBA pixels at the size that a control needs.

// dibSection creates a top-down 32-bit DIB section of size pixels square and
// returns its bits.
func dibSection(size int) (hbitmap, []byte) {
	info := bitmapInfo{bmiHeader: bitmapInfoHeader{
		biWidth:       int32(size),
		biHeight:      -int32(size),
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}}
	info.bmiHeader.biSize = uint32(unsafe.Sizeof(info.bmiHeader))
	var bits uintptr
	screen := win32GetDC(0)
	bitmap := win32CreateDIBSection(screen, unsafe.Pointer(&info), dibRGBColors, &bits, 0, 0)
	win32ReleaseDC(0, screen)
	if bitmap == 0 {
		return 0, nil
	}
	return bitmap, unsafe.Slice((*byte)(memoryAt(bits)), size*size*4)
}

// glyphPixels draws a glyph of the symbol font in a square of size pixels
// with a color. GDI text has no alpha, so the glyph is drawn white on black and
// its coverage becomes the alpha of the color.
func glyphPixels(glyph string, size int, color themeColor) []byte {
	result := make([]byte, size*size*4)
	bitmap, bits := dibSection(size)
	if bitmap == 0 {
		return result
	}
	clear(bits)
	dc := win32CreateCompatibleDC(0)
	previous := win32SelectObject(dc, hgdiobj(bitmap))
	font := win32CreateFont(-int32(size), 0, 0, 0, fwNormal, false, false, false, defaultCharset, outDefaultPrecis,
		clipDefaultPrecis, antialiasedQuality, defaultPitch, symbolFamily())
	previousFont := win32SelectObject(dc, hgdiobj(font))
	win32SetBkMode(dc, transparent)
	win32SetTextColor(dc, 0xffffff)
	area := rect{right: int32(size), bottom: int32(size)}
	win32DrawText(dc, glyph, &area, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
	win32GdiFlush()
	r := byte(math.Round(float64(color.r) * 255))
	g := byte(math.Round(float64(color.g) * 255))
	b := byte(math.Round(float64(color.b) * 255))
	for i := range size * size {
		coverage := float32(bits[i*4+1])
		result[i*4+0] = r
		result[i*4+1] = g
		result[i*4+2] = b
		result[i*4+3] = byte(coverage * color.a)
	}
	win32SelectObject(dc, previousFont)
	win32SelectObject(dc, previous)
	win32DeleteObject(hgdiobj(font))
	win32DeleteObject(hgdiobj(bitmap))
	win32DeleteDC(dc)
	return result
}

// scalePixels scales the pixels of an icon to fit a square of size pixels,
// keeping their aspect ratio, with bilinear filtering.
func scalePixels(icon *iconImage, size int) []byte {
	result := make([]byte, size*size*4)
	scale := min(float64(size)/float64(icon.width), float64(size)/float64(icon.height))
	w := max(1, int(math.Round(float64(icon.width)*scale)))
	h := max(1, int(math.Round(float64(icon.height)*scale)))
	ox, oy := (size-w)/2, (size-h)/2
	pixel := func(x, y int) []byte {
		return icon.pixels[(y*icon.width+x)*4:]
	}
	for y := range h {
		sy := (float64(y)+0.5)/scale - 0.5
		y0 := max(0, int(math.Floor(sy)))
		y1 := min(icon.height-1, y0+1)
		fy := sy - math.Floor(sy)
		if sy < 0 {
			fy = 0
		}
		for x := range w {
			sx := (float64(x)+0.5)/scale - 0.5
			x0 := max(0, int(math.Floor(sx)))
			x1 := min(icon.width-1, x0+1)
			fx := sx - math.Floor(sx)
			if sx < 0 {
				fx = 0
			}
			out := result[((y+oy)*size+(x+ox))*4:]
			p00, p01, p10, p11 := pixel(x0, y0), pixel(x1, y0), pixel(x0, y1), pixel(x1, y1)
			// Premultiply while filtering so transparent pixels do not darken
			// the edges, then return to straight alpha.
			a := (float64(p00[3])*(1-fx)+float64(p01[3])*fx)*(1-fy) + (float64(p10[3])*(1-fx)+float64(p11[3])*fx)*fy
			for c := range 3 {
				v := (float64(p00[c])*float64(p00[3])*(1-fx)+float64(p01[c])*float64(p01[3])*fx)*(1-fy) +
					(float64(p10[c])*float64(p10[3])*(1-fx)+float64(p11[c])*float64(p11[3])*fx)*fy
				if a > 0 {
					out[c] = byte(min(255, v/a))
				} else {
					out[c] = 0
				}
			}
			out[3] = byte(math.Round(a))
		}
	}
	return result
}

// iconPixels returns the pixels of an icon at a size in physical pixels, with
// the color of a glyph.
func iconPixels(icon *iconImage, size int, color themeColor) []byte {
	if icon == nil || size <= 0 {
		return nil
	}
	if len(icon.pixels) > 0 && icon.width > 0 && icon.height > 0 {
		return scalePixels(icon, size)
	}
	return glyphPixels(icon.glyph, size, color)
}

// iconHandle creates an HICON from an icon.
func iconHandle(icon *iconImage, size int, color themeColor) hicon {
	pixels := iconPixels(icon, size, color)
	if pixels == nil {
		return 0
	}
	header := bitmapV5Header{
		bV5Width:       int32(size),
		bV5Height:      -int32(size),
		bV5Planes:      1,
		bV5BitCount:    32,
		bV5Compression: biBitfields,
		bV5RedMask:     0x00ff0000,
		bV5GreenMask:   0x0000ff00,
		bV5BlueMask:    0x000000ff,
		bV5AlphaMask:   0xff000000,
	}
	header.bV5Size = uint32(unsafe.Sizeof(header))
	var address uintptr
	screen := win32GetDC(0)
	color32 := win32CreateDIBSection(screen, unsafe.Pointer(&header), dibRGBColors, &address, 0, 0)
	win32ReleaseDC(0, screen)
	if color32 == 0 {
		return 0
	}
	out := unsafe.Slice((*byte)(memoryAt(address)), size*size*4)
	for i := range size * size {
		out[i*4+0] = pixels[i*4+2]
		out[i*4+1] = pixels[i*4+1]
		out[i*4+2] = pixels[i*4+0]
		out[i*4+3] = pixels[i*4+3]
	}
	mask := win32CreateBitmap(int32(size), int32(size), 1, 1, nil)
	info := iconInfo{fIcon: 1, hbmColor: color32, hbmMask: mask}
	handle := win32CreateIconIndirect(&info)
	win32DeleteObject(hgdiobj(color32))
	win32DeleteObject(hgdiobj(mask))
	return handle
}

// iconBitmap returns a 32-bit bitmap of an icon with premultiplied alpha,
// which menus and image lists draw with transparency.
func iconBitmap(icon *iconImage, size int, color themeColor) hbitmap {
	pixels := iconPixels(icon, size, color)
	if pixels == nil {
		return 0
	}
	bitmap, out := dibSection(size)
	if bitmap == 0 {
		return 0
	}
	for i := range size * size {
		a := uint32(pixels[i*4+3])
		out[i*4+0] = byte(uint32(pixels[i*4+2]) * a / 255)
		out[i*4+1] = byte(uint32(pixels[i*4+1]) * a / 255)
		out[i*4+2] = byte(uint32(pixels[i*4+0]) * a / 255)
		out[i*4+3] = byte(a)
	}
	return bitmap
}

// drawIcon draws an icon in a box.
func drawIcon(canvas *painter, icon *iconImage, x, y, size float32, color themeColor) {
	if icon == nil {
		return
	}
	if len(icon.pixels) == 0 {
		canvas.glyph(icon.glyph, size, x, y, size, size, color)
		return
	}
	bitmap := bitmapFromRGBA(canvas.target, icon.pixels, icon.width, icon.height, icon.width*4)
	if bitmap == 0 {
		return
	}
	scale := min(float64(size)/float64(icon.width), float64(size)/float64(icon.height))
	w, h := float32(float64(icon.width)*scale), float32(float64(icon.height)*scale)
	canvas.bitmap(bitmap, x+(size-w)/2, y+(size-h)/2, w, h, 1, false)
	win32UnknownRelease(bitmap)
}
