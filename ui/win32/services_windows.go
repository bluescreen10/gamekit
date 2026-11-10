package win32

import (
	stdimage "image"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Clipboard ----
//
// The clipboard is the Win32 clipboard. Text, files, and images use the
// standard formats; other MIME types are registered formats with the type as
// the name.

func openClipboard() bool {
	for range 20 {
		if win32OpenClipboard(0) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

var pngFormatValue uint32

func pngFormat() uint32 {
	if pngFormatValue == 0 {
		pngFormatValue = win32RegisterClipboardFormat("PNG")
	}
	return pngFormatValue
}

func clipboardChangeCount() int {
	return int(win32GetClipboardSequenceNumber())
}

func clearClipboard() {
	if openClipboard() {
		win32EmptyClipboard()
		win32CloseClipboard()
	}
}

// globalCopy returns movable global memory with a copy of data.
func globalCopy(data []byte) hglobal {
	memory := win32GlobalAlloc(gmemMoveable, uintptr(max(len(data), 1)))
	if memory != 0 && len(data) > 0 {
		copy(unsafe.Slice((*byte)(memoryAt(win32GlobalLock(memory))), len(data)), data)
		win32GlobalUnlock(memory)
	}
	return memory
}

// writeClipboard replaces the content of the clipboard. It reports false when
// another program holds the clipboard.
func writeClipboard(content ui.ClipboardContent) bool {
	var pixels, png []byte
	var width, height int
	if content.Image != nil && !content.Image.Bounds().Empty() {
		pixels, width, height = rgbaPixels(content.Image)
		png = encodePNG(pixels, width, height)
	}
	if !openClipboard() {
		return false
	}
	win32EmptyClipboard()
	if content.Text != "" {
		text := utf16.Encode([]rune(content.Text + "\x00"))
		win32SetClipboardData(cfUnicodeText, globalCopy(unsafe.Slice((*byte)(unsafe.Pointer(&text[0])), len(text)*2)))
	}
	if len(content.Files) > 0 {
		win32SetClipboardData(cfHDrop, globalCopy(fileList(content.Files)))
	}
	if pixels != nil {
		win32SetClipboardData(cfDIBV5, globalCopy(dibV5(pixels, width, height)))
		if len(png) > 0 {
			win32SetClipboardData(pngFormat(), globalCopy(png))
		}
	}
	for mime, data := range content.Custom {
		if format := win32RegisterClipboardFormat(mime); format != 0 {
			win32SetClipboardData(format, globalCopy(data))
		}
	}
	win32CloseClipboard()
	return true
}

// dibV5 returns a bottom-up 32-bit DIB with straight alpha of RGBA pixels,
// which programs that do not read PNG understand.
func dibV5(pixels []byte, width, height int) []byte {
	header := bitmapV5Header{
		bV5Width:       int32(width),
		bV5Height:      int32(height),
		bV5Planes:      1,
		bV5BitCount:    32,
		bV5Compression: biBitfields,
		bV5RedMask:     0x00ff0000,
		bV5GreenMask:   0x0000ff00,
		bV5BlueMask:    0x000000ff,
		bV5AlphaMask:   0xff000000,
		bV5CSType:      lcsSRGB,
		bV5SizeImage:   uint32(width * height * 4),
	}
	header.bV5Size = uint32(unsafe.Sizeof(header))
	size := int(unsafe.Sizeof(header))
	block := make([]byte, size+width*height*4)
	copy(block, unsafe.Slice((*byte)(unsafe.Pointer(&header)), size))
	out := block[size:]
	for y := range height {
		row := pixels[(height-1-y)*width*4:]
		for x := range width {
			at := (y*width + x) * 4
			out[at], out[at+1], out[at+2], out[at+3] = row[x*4+2], row[x*4+1], row[x*4], row[x*4+3]
		}
	}
	return block
}

func formatName(format uint32) string {
	var name [256]uint16
	length := win32GetClipboardFormatName(format, name[:])
	return string(utf16.Decode(name[:max(length, 0)]))
}

// clipboardBytes returns the data of the clipboard in a format.
func clipboardBytes(format uint32) ([]byte, bool) {
	handle := hglobal(win32GetClipboardData(format))
	if handle == 0 {
		return nil, false
	}
	data := win32GlobalLock(handle)
	if data == 0 {
		return nil, false
	}
	bytes := goBytesAt(data, int(win32GlobalSize(handle)))
	win32GlobalUnlock(handle)
	return bytes, true
}

// readClipboard returns the content of the clipboard. An image is the PNG or,
// without one, the DIB on the clipboard. Data in other formats is there by the
// MIME type that names the format.
func readClipboard() ui.ClipboardContent {
	var content ui.ClipboardContent
	if !openClipboard() {
		return content
	}
	if win32IsClipboardFormatAvailable(cfUnicodeText) {
		if bytes, ok := clipboardBytes(cfUnicodeText); ok {
			content.Text = goString(unsafe.Slice((*uint16)(unsafe.Pointer(unsafe.SliceData(bytes))), len(bytes)/2))
		}
	}
	if win32IsClipboardFormatAvailable(cfHDrop) {
		if drop := win32GetClipboardData(cfHDrop); drop != 0 {
			content.Files = filesFromDrop(drop)
		}
	}
	png, havePNG := clipboardBytesOf(pngFormat())
	var dib []byte
	var haveDIB bool
	if !havePNG {
		if dib, haveDIB = clipboardBytesOf(cfDIBV5); !haveDIB {
			dib, haveDIB = clipboardBytesOf(cfDIB)
		}
	}
	for format := win32EnumClipboardFormats(0); format != 0; format = win32EnumClipboardFormats(format) {
		name := formatName(format)
		if format < 0xc000 || !strings.Contains(name, "/") {
			continue
		}
		if data, ok := clipboardBytes(format); ok {
			if content.Custom == nil {
				content.Custom = map[string][]byte{}
			}
			content.Custom[name] = data
		}
	}
	win32CloseClipboard()
	var pixels []byte
	var width, height int
	switch {
	case havePNG:
		pixels, width, height, _ = decodeImage(png)
	case haveDIB:
		pixels, width, height, _ = pixelsFromDIB(dib)
	}
	if pixels != nil {
		image := stdimage.NewNRGBA(stdimage.Rect(0, 0, width, height))
		copy(image.Pix, pixels)
		content.Image = image
	}
	return content
}

// clipboardBytesOf returns the data of the clipboard in a format, if it has
// the format.
func clipboardBytesOf(format uint32) ([]byte, bool) {
	if !win32IsClipboardFormatAvailable(format) {
		return nil, false
	}
	return clipboardBytes(format)
}

func clipboardFormats() []string {
	if !openClipboard() {
		return nil
	}
	var names []string
	add := func(name string) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for format := win32EnumClipboardFormats(0); format != 0; format = win32EnumClipboardFormats(format) {
		switch {
		case format == cfUnicodeText || format == cfText || format == cfOEMText:
			add("text/plain")
		case format == cfHDrop:
			add("text/uri-list")
		case format == cfDIB || format == cfDIBV5 || format == cfBitmap || format == pngFormat():
			add("image/png")
		case format >= 0xc000:
			if name := formatName(format); strings.Contains(name, "/") {
				add(name)
			}
		}
	}
	win32CloseClipboard()
	return names
}

// ---- File manager and documents ----

func revealFile(path string, selectFile bool) bool {
	if path == "" {
		return false
	}
	path = strings.ReplaceAll(path, "/", `\`)
	attributes := win32GetFileAttributes(path)
	if attributes == invalidFileAttributes {
		return false
	}
	if selectFile {
		var item uintptr
		if win32SHParseDisplayName(path, 0, &item, 0, nil).failed() || item == 0 {
			return false
		}
		result := win32SHOpenFolderAndSelectItems(item, 0, nil, 0)
		win32CoTaskMemFree(item)
		return result.succeeded()
	}
	folder := path
	if attributes&fileAttributeDirectory == 0 {
		if slash := strings.LastIndexByte(folder, '\\'); slash >= 0 {
			folder = folder[:slash]
		} else {
			folder = "."
		}
	}
	return win32ShellExecute(0, "explore", folder, "", "", swShowNormal) > 32
}

// recentFile returns the file of the recent documents of the application, a
// list in the local application data of the user, named after the executable.
// Windows also records each one in its own recent items.
func recentFile() string {
	local, _ := os.UserCacheDir()
	folder := filepath.Join(local, "GameKit", "Recent")
	os.MkdirAll(folder, 0o755)
	executable, _ := os.Executable()
	return filepath.Join(folder, filepath.Base(executable)+".txt")
}

func recentRead() []string {
	data, err := os.ReadFile(recentFile())
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

func recentWrite(paths []string) {
	var data strings.Builder
	for _, path := range paths {
		data.WriteString(path)
		data.WriteByte('\n')
	}
	os.WriteFile(recentFile(), []byte(data.String()), 0o644)
}

func addRecentDocument(path string) {
	if path == "" {
		return
	}
	paths := slices.DeleteFunc(recentRead(), func(other string) bool { return other == path })
	paths = append([]string{path}, paths...)
	if len(paths) > 100 {
		paths = paths[:100]
	}
	recentWrite(paths)
	win32SHAddToRecentDocs(shardPathW, path)
}

func removeRecentDocument(path string) {
	if path == "" {
		return
	}
	recentWrite(slices.DeleteFunc(recentRead(), func(other string) bool { return other == path }))
}

func readRecentDocuments() []string {
	return recentRead()
}

func clearRecentDocuments() {
	recentWrite(nil)
}

func defaultApplication(extension string) string {
	if extension == "" {
		return ""
	}
	if extension[0] != '.' {
		extension = "." + extension
	}
	var name [512]uint16
	length := uint32(len(name))
	if win32AssocQueryString(assocfInitIgnoreUnknown, assocstrFriendlyAppName, extension, "", name[:],
		&length).failed() {
		return ""
	}
	return goString(name[:])
}

// ---- Printing ----
//
// Printing uses the print dialog of Windows and draws the pages with GDI. The
// Go layer measures pages in points.

// printDC is the device context of the page being printed.
var printDC hdc

// printPages prints the pages that page draws and returns how many it
// printed, or -1 when the user cancels or printing fails. pageCount returns
// the number of pages, and page draws one in a size in points and reports
// whether it succeeded.
func printPages(title string, showDialog bool, win *nativeWindow, pageCount func() int, page func(index int, width, height float64) bool) int {
	setup := printDlgEx{
		flags:      pdReturnDC | pdNoPageNums | pdNoSelection | pdNoCurrentPage | pdUseDevModeCopiesAndCollate,
		nStartPage: startPageGeneral,
	}
	setup.lStructSize = uint32(unsafe.Sizeof(setup))
	if win != nil && win.hwnd != 0 && win32IsWindowVisible(win.hwnd) {
		setup.hwndOwner = win.hwnd
	} else {
		setup.hwndOwner = win32GetForegroundWindow()
	}
	if !showDialog {
		setup.flags |= pdReturnDefault
	}
	release := func() {
		if setup.hDC != 0 {
			win32DeleteDC(setup.hDC)
		}
		if setup.hDevMode != 0 {
			win32GlobalFree(setup.hDevMode)
		}
		if setup.hDevNames != 0 {
			win32GlobalFree(setup.hDevNames)
		}
	}
	defer release()
	if win32PrintDlgEx(&setup).failed() || setup.hDC == 0 ||
		(showDialog && setup.dwResultAction != pdResultPrint) {
		return -1
	}
	dc := setup.hDC
	pages := max(pageCount(), 1)
	if title == "" {
		title = "GameKit"
	}
	document := docInfo{lpszDocName: wide(title)}
	document.cbSize = int32(unsafe.Sizeof(document))
	if win32StartDoc(dc, &document) <= 0 {
		return -1
	}
	width := float64(win32GetDeviceCaps(dc, horzRes)) * 72 / float64(win32GetDeviceCaps(dc, logPixelsX))
	height := float64(win32GetDeviceCaps(dc, vertRes)) * 72 / float64(win32GetDeviceCaps(dc, logPixelsY))
	for index := range pages {
		win32StartPage(dc)
		printDC = dc
		ok := page(index, width, height)
		printDC = 0
		win32EndPage(dc)
		if !ok {
			win32AbortDoc(dc)
			return -1
		}
	}
	win32EndDoc(dc)
	return pages
}

func printImage(pixels []byte, width int, height int, x float64, y float64, w float64, h float64) {
	if printDC == 0 || len(pixels) < width*height*4 || width <= 0 || height <= 0 {
		return
	}
	// GDI ignores alpha, so the image is composed over white paper.
	bgr := make([]byte, width*height*4)
	for i := range width * height {
		alpha := uint32(pixels[i*4+3])
		for channel := range 3 {
			value := uint32(pixels[i*4+2-channel])
			bgr[i*4+channel] = byte((value*alpha + 255*(255-alpha)) / 255)
		}
		bgr[i*4+3] = 255
	}
	info := bitmapInfo{bmiHeader: bitmapInfoHeader{
		biWidth:       int32(width),
		biHeight:      -int32(height),
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}}
	info.bmiHeader.biSize = uint32(unsafe.Sizeof(info.bmiHeader))
	sx := float64(win32GetDeviceCaps(printDC, logPixelsX)) / 72
	sy := float64(win32GetDeviceCaps(printDC, logPixelsY)) / 72
	win32SetStretchBltMode(printDC, halftone)
	win32StretchDIBits(printDC, int32(math.Round(x*sx)), int32(math.Round(y*sy)), int32(math.Round(w*sx)),
		int32(math.Round(h*sy)), 0, 0, int32(width), int32(height), unsafe.Pointer(&bgr[0]), &info, dibRGBColors,
		srcCopy)
}

// printText draws text on the page and returns false when its font
// family is not installed.
func printText(text string, x float64, y float64, family string, size float64, weight int, italic bool) bool {
	if !fontFamilyInstalled(family) {
		return false
	}
	if printDC == 0 {
		return true
	}
	sx := float64(win32GetDeviceCaps(printDC, logPixelsX)) / 72
	sy := float64(win32GetDeviceCaps(printDC, logPixelsY)) / 72
	face := "Segoe UI"
	switch family {
	case "", "System":
	case "Monospace":
		face = "Consolas"
	default:
		face = family
	}
	if size <= 0 {
		size = 12
	}
	if weight <= 0 {
		weight = fwNormal
	}
	font := win32CreateFont(-int32(math.Round(size*sy)), 0, 0, 0, int32(weight), italic, false, false, defaultCharset,
		outDefaultPrecis, clipDefaultPrecis, cleartypeQuality, defaultPitch, face)
	previous := win32SelectObject(printDC, hgdiobj(font))
	win32SetBkMode(printDC, transparent)
	win32SetTextColor(printDC, 0)
	area := rect{left: int32(math.Round(x * sx)), top: int32(math.Round(y * sy)),
		right: win32GetDeviceCaps(printDC, horzRes), bottom: win32GetDeviceCaps(printDC, vertRes)}
	win32DrawText(printDC, text, &area, dtLeft|dtTop|dtNoClip|dtNoPrefix)
	win32SelectObject(printDC, previous)
	win32DeleteObject(hgdiobj(font))
	return true
}
