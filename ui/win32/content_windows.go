package win32

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Icon ----
//
// An icon is a symbol or an image that the backend draws, as Windows has no
// control for one.

type iconViewExtension struct {
	baseExtension
	icon          *iconImage
	width, height int
	tint          int
}

func (i *iconViewExtension) free() {
}

func (i *iconViewExtension) paint(view *nativeView, canvas *painter) {
	if i.icon == nil {
		return
	}
	pal := view.palette()
	color := view.textColorOr(pal.text)
	if tint, ok := semanticColor(pal, i.tint); ok {
		color = tint
	}
	size := min(canvas.width, canvas.height)
	drawIcon(canvas, i.icon, (canvas.width-size)/2, (canvas.height-size)/2, size, color)
}

func (i *iconViewExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	width, height := i.width, i.height
	if width <= 0 {
		width = 16
	}
	if height <= 0 {
		height = 16
	}
	return d2d1SizeF{width: float32(width), height: float32(height)}, true
}

func (win *nativeWindow) newIconView() *nativeView {
	view := createDrawn(win, kindIcon, &iconViewExtension{}, false)
	view.accessibleHidden = true
	return view
}

func (view *nativeView) setIcon(image *iconImage, width int, height int, tint int, name string) {
	if !view.live() || view.kind != kindIcon {
		return
	}
	state := view.ext.(*iconViewExtension)
	state.icon = image
	state.width = width
	state.height = height
	state.tint = tint
	view.accessibleName = name
	view.accessibleHidden = name == ""
	view.invalidate()
}

// ---- Documents ----
//
// Rich text and Markdown are a rich edit control in a text box. Markdown
// becomes RTF, so its links are RTF hyperlink fields, and it has no frame, like
// text.

type markdownLink struct {
	label, url string
}

type documentExtension struct {
	fieldExtension
	markdown bool
	source   string
	// links are the label and the URL of each Markdown link.
	links []markdownLink
}

func (d *documentExtension) paintGDI(view *nativeView, h hwnd, dc hdc, client *rect) bool {
	if !d.markdown {
		return d.fieldExtension.paintGDI(view, h, dc, client)
	}
	fillRectColor(dc, client, backgroundOf(h))
	return true
}

func (d *documentExtension) layout(view *nativeView) {
	if !d.markdown {
		d.fieldExtension.layout(view)
		return
	}
	var client rect
	win32GetClientRect(view.outer, &client)
	win32SetWindowPos(d.edit, 0, 0, 0, client.right, client.bottom, swpNoZOrder|swpNoActivate)
}

func (d *documentExtension) command(view *nativeView, from hwnd, code int) bool {
	if from == d.edit && code == enChange {
		d.hooks.changed(view)
		return true
	}
	return false
}

func documentOf(view *nativeView) *documentExtension {
	if view == nil || (view.kind != kindDocument && view.kind != kindTextArea) {
		return nil
	}
	document, _ := view.ext.(*documentExtension)
	return document
}

// colors gives the rich edit control the background and the default text
// color of the theme.
func (d *documentExtension) colors(view *nativeView) {
	pal := view.palette()
	fill := rgbColor(0xffffff)
	switch {
	case d.markdown:
		fill = backgroundOf(view.outer)
	case pal.dark:
		fill = pal.inputActive
	}
	win32SendMessage(d.edit, emSetBkgndColor, 0, uintptr(fill.colorRef()))
	if d.markdown {
		return
	}
	format := charFormat2{dwMask: cfmColor, crTextColor: view.textColorOr(pal.text).colorRef()}
	format.cbSize = uint32(unsafe.Sizeof(format))
	view.suppress++
	win32SendMessage(d.edit, emSetCharFormat, scfDefault, uintptr(unsafe.Pointer(&format)))
	// Text without a color of its own takes the system color, which the
	// palette replaces.
	if pal.dark {
		win32SendMessage(d.edit, emSetCharFormat, scfAll, uintptr(unsafe.Pointer(&format)))
	}
	view.suppress--
}

func (d *documentExtension) themeChanged(view *nativeView) {
	d.fieldExtension.themeChanged(view)
	d.colors(view)
	if d.markdown && d.source != "" {
		applyMarkdown(view, d)
	}
}

// link returns the URL of the link in a range of a document: the field
// instruction of an RTF hyperlink, the URL of a Markdown link with the text, or
// the text.
func (d *documentExtension) link(chars charRange) string {
	buffer := make([]uint16, max(chars.cpMax-chars.cpMin+1, 1))
	request := textRange{chrg: chars, lpstrText: &buffer[0]}
	length := int(win32SendMessage(d.edit, emGetTextRange, 0, uintptr(unsafe.Pointer(&request))))
	text := string(utf16.Decode(buffer[:max(length, 0)]))
	first, last := strings.IndexByte(text, '"'), strings.LastIndexByte(text, '"')
	if strings.Contains(text, "HYPERLINK") && first >= 0 && last > first {
		return text[first+1 : last]
	}
	for _, link := range d.links {
		if link.label == text {
			return link.url
		}
	}
	return text
}

func (d *documentExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.hwndFrom != d.edit || header.code != enLink {
		return false
	}
	link := (*enLinkInfo)(unsafe.Pointer(header))
	*result = 0
	if link.msg != wmLButtonUp {
		return true
	}
	var selection charRange
	win32SendMessage(d.edit, emExGetSel, 0, uintptr(unsafe.Pointer(&selection)))
	if selection.cpMax != selection.cpMin {
		return true
	}
	url := d.link(link.chrg)
	if view.receiver == nil || !ask(view.receiver, linkRequest{url: url}) {
		openURL(url, false)
	}
	*result = 1
	return true
}

func (win *nativeWindow) newDocument(editable bool, markdown bool) *nativeView {
	document := &documentExtension{markdown: markdown}
	document.hooks = document
	document.multiline = true
	kind := kindTextArea
	if markdown {
		kind = kindDocument
	}
	view := createDrawn(win, kind, document, false)
	style := uint32(wsChild | wsVisible | wsTabStop | wsVScroll | esMultiline | esWantReturn | esAutoVScroll |
		esNoHideSel)
	if !editable {
		style |= esReadOnly
	}
	document.edit = makeNative(msftEditClass, "", style, 0, view.outer)
	view.control = document.edit
	view.readOnly = !editable
	setViewOf(document.edit, view)
	subclass(document.edit)
	view.applyFont()
	applyThemeTo(document.edit, view.theme(), "")
	win32SendMessage(document.edit, emSetEventMask, 0, enmChange|enmLink)
	win32SendMessage(document.edit, emSetTextMode, tmRichText|tmMultiLevelUndo|tmMultiCodePage, 0)
	var detect uintptr
	if !markdown {
		detect = aurlEnableURL
	}
	win32SendMessage(document.edit, emAutoURLDetect, detect, 0)
	// The rich edit control sends the notifications of the links only to a
	// parent that asks for them, which the view's win is.
	if markdown {
		var margins rect
		win32SendMessage(document.edit, emSetRectNP, 0, uintptr(unsafe.Pointer(&margins)))
		win32SendMessage(document.edit, emShowScrollBar, sbVert, 0)
	} else {
		win32SendMessage(document.edit, emSetMargins, ecLeftMargin|ecRightMargin, makeWParam(4, 4))
	}
	document.colors(view)
	return view
}

func (view *nativeView) documentText() string {
	document := documentOf(view)
	if document == nil {
		return ""
	}
	measure := getTextLengthEx{flags: gtlNumChars | gtlPrecise, codepage: 1200}
	length := int(int32(win32SendMessage(document.edit, emGetTextLengthEx, uintptr(unsafe.Pointer(&measure)), 0)))
	buffer := make([]uint16, max(length, 0)+1)
	request := getTextEx{cb: uint32(len(buffer) * 2), flags: gtDefault, codepage: 1200}
	length = int(int32(win32SendMessage(document.edit, emGetTextEx, uintptr(unsafe.Pointer(&request)),
		uintptr(unsafe.Pointer(&buffer[0])))))
	text := string(utf16.Decode(buffer[:max(length, 0)]))
	// The control separates paragraphs with carriage returns.
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

func (view *nativeView) setDocumentText(text string) {
	document := documentOf(view)
	if document == nil {
		return
	}
	request := setTextEx{flags: stDefault, codepage: 1200}
	view.suppress++
	win32SendMessage(document.edit, emSetTextEx, uintptr(unsafe.Pointer(&request)), uintptr(unsafe.Pointer(wide(text))))
	document.colors(view)
	view.suppress--
}

func (view *nativeView) setDocumentEditable(editable bool) {
	if document := documentOf(view); document != nil {
		view.readOnly = !editable
		win32SendMessage(document.edit, emSetReadOnly, boolArg(!editable), 0)
		view.invalidate()
	}
}

func (view *nativeView) isDocumentEditable() bool {
	document := documentOf(view)
	return document != nil && win32GetWindowLongPtr(document.edit, gwlStyle)&esReadOnly == 0
}

// streams holds the buffers that EM_STREAMOUT writes, by the cookie that the
// callback receives.
var (
	streams    = map[uintptr]*[]byte{}
	nextStream uintptr
)

func streamOut(cookie, buffer, size, written uintptr) uintptr {
	if out := streams[cookie]; out != nil {
		*out = append(*out, unsafe.Slice((*byte)(memoryAt(buffer)), int(int32(size)))...)
	}
	*(*int32)(memoryAt(written)) = int32(size)
	return 0
}

func (view *nativeView) documentRTF() []byte {
	document := documentOf(view)
	if document == nil {
		return nil
	}
	var rtf []byte
	nextStream++
	cookie := nextStream
	streams[cookie] = &rtf
	defer delete(streams, cookie)
	stream := editStream{dwCookie: packedPointer(cookie), pfnCallback: packedPointer(callback("streamOut", streamOut))}
	win32SendMessage(document.edit, emStreamOut, sfRTF, uintptr(unsafe.Pointer(&stream)))
	return rtf
}

// setRTF replaces the content of a document with RTF.
func setRTF(view *nativeView, document *documentExtension, rtf []byte) bool {
	request := setTextEx{flags: stDefault, codepage: cpACP}
	text := append(append([]byte(nil), rtf...), 0)
	view.suppress++
	result := win32SendMessage(document.edit, emSetTextEx, uintptr(unsafe.Pointer(&request)),
		uintptr(unsafe.Pointer(&text[0])))
	view.suppress--
	return result != 0
}

func (view *nativeView) setDocumentRTF(rtf []byte) bool {
	document := documentOf(view)
	if document == nil || len(rtf) < 5 || string(rtf[:5]) != `{\rtf` {
		return false
	}
	return setRTF(view, document, rtf)
}

// ---- Markdown ----
//
// The renderer covers CommonMark blocks (headings, paragraphs, block quotes,
// lists, code blocks, and thematic breaks) and inlines (emphasis, strong
// emphasis, code spans, and links), and writes them as RTF.

func rtfText(out *strings.Builder, text string) {
	for _, unit := range utf16.Encode([]rune(text)) {
		switch {
		case unit == '\\' || unit == '{' || unit == '}':
			out.WriteByte('\\')
			out.WriteByte(byte(unit))
		case unit == '\t':
			out.WriteString(`\tab `)
		case unit < 0x80:
			out.WriteByte(byte(unit))
		default:
			fmt.Fprintf(out, `\u%d?`, int16(unit))
		}
	}
}

func markdownInline(out *strings.Builder, text string) {
	var plain strings.Builder
	flush := func() {
		rtfText(out, plain.String())
		plain.Reset()
	}
	for p := 0; p < len(text); {
		c := text[p]
		if c == '\\' && p+1 < len(text) && strings.IndexByte("\\`*_{}[]()#+-.!>", text[p+1]) >= 0 {
			plain.WriteByte(text[p+1])
			p += 2
			continue
		}
		if c == '`' {
			if end := strings.IndexByte(text[p+1:], '`'); end >= 0 {
				flush()
				out.WriteString(`{\f1 `)
				rtfText(out, text[p+1:p+1+end])
				out.WriteString("}")
				p += end + 2
				continue
			}
		}
		if (c == '*' || c == '_') && p+1 < len(text) && text[p+1] == c {
			marker := string([]byte{c, c})
			if end := strings.Index(text[p+2:], marker); end > 0 {
				flush()
				out.WriteString(`{\b `)
				markdownInline(out, text[p+2:p+2+end])
				out.WriteString("}")
				p += end + 4
				continue
			}
		}
		if (c == '*' || c == '_') && p+1 < len(text) && text[p+1] != ' ' {
			if end := strings.IndexByte(text[p+1:], c); end > 0 {
				flush()
				out.WriteString(`{\i `)
				markdownInline(out, text[p+1:p+1+end])
				out.WriteString("}")
				p += end + 2
				continue
			}
		}
		if c == '[' {
			if labelEnd := strings.Index(text[p:], "]("); labelEnd >= 0 {
				labelEnd += p
				if urlEnd := strings.IndexByte(text[labelEnd+2:], ')'); urlEnd >= 0 {
					urlEnd += labelEnd + 2
					flush()
					out.WriteString(`{\field{\*\fldinst{HYPERLINK "`)
					rtfText(out, text[labelEnd+2:urlEnd])
					out.WriteString(`"}}{\fldrslt{\ul\cf1 `)
					rtfText(out, text[p+1:labelEnd])
					out.WriteString("}}}")
					p = urlEnd + 1
					continue
				}
			}
		}
		plain.WriteByte(c)
		p++
	}
	flush()
}

func rtfColor(out *strings.Builder, color themeColor) {
	fmt.Fprintf(out, `\red%d\green%d\blue%d;`, int(math.Round(float64(color.r)*255)),
		int(math.Round(float64(color.g)*255)), int(math.Round(float64(color.b)*255)))
}

// markdownRTF converts Markdown to RTF in the colors of a palette: 1 is links,
// 2 secondary text, and 3 text.
func markdownRTF(source string, pal *palette) string {
	var out strings.Builder
	out.WriteString(`{\rtf1\ansi\deff0{\fonttbl{\f0 Segoe UI;}{\f1 Cascadia Mono;}}{\colortbl;`)
	rtfColor(&out, pal.accentText)
	rtfColor(&out, pal.textSecondary)
	rtfColor(&out, pal.text)
	out.WriteString(`}\cf3\fs21 `)
	var paragraph string
	inCode := false
	flush := func() {
		if paragraph != "" {
			out.WriteString(`\pard\cf3 `)
			markdownInline(&out, paragraph)
			out.WriteString("\\par\n")
			paragraph = ""
		}
	}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimRight(line, "\r ")
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			flush()
			inCode = !inCode
			continue
		}
		if inCode {
			out.WriteString(`\pard\li360{\f1 `)
			rtfText(&out, line)
			out.WriteString("}\\par\n")
			continue
		}
		trimmed := strings.TrimLeft(line, " ")
		heading := 0
		for heading < len(trimmed) && trimmed[heading] == '#' && heading < 7 {
			heading++
		}
		isHeading := heading > 0 && heading <= 6 && (heading == len(trimmed) || trimmed[heading] == ' ')
		isQuote := trimmed != "" && trimmed[0] == '>'
		isBullet := len(trimmed) > 1 && strings.IndexByte("-*+", trimmed[0]) >= 0 && trimmed[1] == ' '
		digits := 0
		for digits < len(trimmed) && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
			digits++
		}
		isNumber := digits > 0 && digits+1 < len(trimmed) && (trimmed[digits] == '.' || trimmed[digits] == ')') &&
			trimmed[digits+1] == ' '
		isRule := trimmed == "---" || trimmed == "***" || trimmed == "___"
		if trimmed == "" || isHeading || isQuote || isBullet || isNumber || isRule {
			flush()
		}
		switch {
		case trimmed == "":
		case isHeading:
			sizes := []int{40, 32, 26, 22, 21, 21}
			fmt.Fprintf(&out, `\pard\sb120\sa60{\b\fs%d `, sizes[heading-1])
			markdownInline(&out, strings.TrimLeft(trimmed[heading:], " "))
			out.WriteString("}\\par\n")
		case isQuote:
			out.WriteString(`\pard\li480{\i\cf2 `)
			markdownInline(&out, strings.TrimLeft(trimmed[1:], " "))
			out.WriteString("}\\par\n")
		case isBullet || isNumber:
			out.WriteString(`\pard\li480\fi-240 `)
			if isBullet {
				out.WriteString(`\u8226? `)
				markdownInline(&out, trimmed[2:])
			} else {
				rtfText(&out, trimmed[:digits+2])
				markdownInline(&out, trimmed[digits+2:])
			}
			out.WriteString("\\par\n")
		case isRule:
			out.WriteString(`\pard{\cf2 \u9472?\u9472?\u9472?\u9472?\u9472?\u9472?\u9472?\u9472?}\par` + "\n")
		default:
			if paragraph != "" {
				paragraph += " "
			}
			paragraph += trimmed
		}
	}
	flush()
	out.WriteString("}")
	return out.String()
}

// markdownLinks records the label and the URL of each link of Markdown, to find
// the URL of a link that the user clicks.
func markdownLinks(text string) []markdownLink {
	var links []markdownLink
	for p := 0; ; {
		start := strings.IndexByte(text[p:], '[')
		if start < 0 {
			break
		}
		start += p
		labelEnd := strings.Index(text[start:], "](")
		if labelEnd < 0 {
			break
		}
		labelEnd += start
		urlEnd := strings.IndexByte(text[labelEnd+2:], ')')
		if urlEnd < 0 {
			break
		}
		urlEnd += labelEnd + 2
		links = append(links, markdownLink{label: text[start+1 : labelEnd], url: text[labelEnd+2 : urlEnd]})
		p = urlEnd + 1
	}
	return links
}

func applyMarkdown(view *nativeView, document *documentExtension) bool {
	document.links = markdownLinks(document.source)
	return setRTF(view, document, []byte(markdownRTF(document.source, view.palette())))
}

func (view *nativeView) setMarkdown(source string) bool {
	document := documentOf(view)
	if document == nil || !utf8.ValidString(source) {
		return false
	}
	document.source = source
	return applyMarkdown(view, document)
}

// ---- SVG ----
//
// An SVG image is drawn with the SVG support of Direct2D. A raster image is
// drawn like a picture.

// svgLength reads a length attribute of the root element of an SVG document,
// in CSS pixels.
func svgLength(svg, name string) float64 {
	root := strings.Index(svg, "<svg")
	if root < 0 {
		return 0
	}
	tag := svg[root:]
	if end := strings.IndexByte(tag, '>'); end >= 0 {
		tag = tag[:end]
	}
	key := " " + name + "="
	at := strings.Index(tag, key)
	if at < 0 {
		return 0
	}
	at += len(key)
	if at >= len(tag) || (tag[at] != '"' && tag[at] != '\'') {
		return 0
	}
	value := leadingFloat(tag[at+1:])
	unit := tag[at+1:]
	if len(unit) > 8 {
		unit = unit[:8]
	}
	switch {
	case strings.Contains(unit, "pt"):
		value *= 96.0 / 72.0
	case strings.Contains(unit, "%"):
		value = 0
	}
	return value
}

// leadingFloat parses the number at the start of text, as atof does.
func leadingFloat(text string) float64 {
	value, _ := leadingNumber(text)
	return value
}

func svgSize(svg string) (float64, float64) {
	width, height := svgLength(svg, "width"), svgLength(svg, "height")
	if width > 0 && height > 0 {
		return width, height
	}
	root := strings.Index(svg, "<svg")
	if root < 0 {
		return width, height
	}
	box := strings.Index(svg[root:], "viewBox=")
	if box < 0 {
		return width, height
	}
	fields := strings.FieldsFunc(svg[root+box+9:], func(r rune) bool { return r == ' ' || r == ',' })
	if len(fields) >= 4 {
		w, errW := strconv.ParseFloat(fields[2], 64)
		h, errH := strconv.ParseFloat(strings.TrimRight(fields[3], `"'/>`), 64)
		if errW == nil && errH == nil {
			if width <= 0 {
				width = w
			}
			if height <= 0 {
				height = h
			}
		}
	}
	return width, height
}

type vectorExtension struct {
	baseExtension
	svg           string
	pixels        []byte
	width, height float64
	document      comObject
	bitmap        comObject
	owner         comObject
}

func (v *vectorExtension) free() {
	v.reset()
}

func (v *vectorExtension) reset() {
	win32UnknownRelease(v.document)
	v.document = 0
	win32UnknownRelease(v.bitmap)
	v.bitmap = 0
	v.owner = 0
}

// multiply returns the product of two transforms, a applied first.
func multiply(a, b d2d1Matrix3x2F) d2d1Matrix3x2F {
	return d2d1Matrix3x2F{
		m11: a.m11*b.m11 + a.m12*b.m21,
		m12: a.m11*b.m12 + a.m12*b.m22,
		m21: a.m21*b.m11 + a.m22*b.m21,
		m22: a.m21*b.m12 + a.m22*b.m22,
		dx:  a.dx*b.m11 + a.dy*b.m21 + b.dx,
		dy:  a.dx*b.m12 + a.dy*b.m22 + b.dy,
	}
}

func (v *vectorExtension) paint(view *nativeView, canvas *painter) {
	if v.width <= 0 || v.height <= 0 {
		return
	}
	scale := min(float64(canvas.width)/v.width, float64(canvas.height)/v.height)
	w, h := float32(v.width*scale), float32(v.height*scale)
	x, y := (canvas.width-w)/2, (canvas.height-h)/2
	if v.owner != canvas.target {
		v.reset()
		v.owner = canvas.target
	}
	if len(v.pixels) > 0 {
		if v.bitmap == 0 {
			v.bitmap = bitmapFromRGBA(canvas.target, v.pixels, int(v.width), int(v.height), int(v.width)*4)
		}
		canvas.bitmap(v.bitmap, x, y, w, h, 1, false)
		return
	}
	context := query(canvas.target, iidD2D1DeviceContext5)
	if context == 0 {
		return
	}
	defer win32UnknownRelease(context)
	if v.document == 0 {
		data := []byte(v.svg)
		if stream := win32SHCreateMemStream(data); stream != 0 {
			win32D2D1DeviceContext5CreateSvgDocument(context, stream, d2d1SizeF{width: float32(v.width),
				height: float32(v.height)}, &v.document)
			win32UnknownRelease(stream)
		}
	}
	if v.document == 0 {
		return
	}
	var previous d2d1Matrix3x2F
	win32D2D1RenderTargetGetTransform(context, &previous)
	scaled := d2d1Matrix3x2F{m11: float32(scale), m22: float32(scale)}
	moved := d2d1Matrix3x2F{m11: 1, m22: 1, dx: x, dy: y}
	transform := multiply(multiply(scaled, moved), previous)
	win32D2D1RenderTargetSetTransform(context, &transform)
	win32D2D1DeviceContext5DrawSvgDocument(context, v.document)
	win32D2D1RenderTargetSetTransform(context, &previous)
}

func (v *vectorExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: float32(v.width), height: float32(v.height)}, true
}

func (win *nativeWindow) newVectorImage() *nativeView {
	return createDrawn(win, kindVector, &vectorExtension{}, false)
}

// setVectorImage sets an SVG or raster image and returns its size, or false
// when the data is not an image.
func (view *nativeView) setVectorImage(data []byte, label string) (width, height float64, ok bool) {
	if !view.live() || view.kind != kindVector || len(data) == 0 {
		return 0, 0, false
	}
	vector := view.ext.(*vectorExtension)
	text := string(data)
	if strings.Contains(text, "<svg") {
		w, h := svgSize(text)
		if w <= 0 || h <= 0 {
			return 0, 0, false
		}
		vector.reset()
		vector.svg = text
		vector.pixels = nil
		vector.width, vector.height = w, h
	} else {
		pixels, w, h, decoded := decodeImage(data)
		if !decoded {
			return 0, 0, false
		}
		vector.reset()
		vector.svg = ""
		vector.pixels = pixels
		vector.width, vector.height = float64(w), float64(h)
	}
	view.accessibleName = label
	view.invalidate()
	return vector.width, vector.height, true
}

// ---- Animated image ----
//
// An animated image draws its frames in turn, on a timer.

// animationFrame is a frame of an animated image: its pixels, how long it
// shows, and the bitmap of Direct2D for it, made when it first paints.
type animationFrame struct {
	pixels        []byte
	width, height int
	duration      time.Duration
	bitmap        comObject
}

type animationExtension struct {
	baseExtension
	frames []animationFrame
	// owner is the render target of the bitmaps of the frames.
	owner   comObject
	repeats bool
	index   int
	playing bool
	timer   uintptr
}

func (a *animationExtension) free() {
	cancelTimer(a.timer)
	a.releaseBitmaps()
}

func (a *animationExtension) releaseBitmaps() {
	for i := range a.frames {
		win32UnknownRelease(a.frames[i].bitmap)
		a.frames[i].bitmap = 0
	}
}

func (a *animationExtension) paint(view *nativeView, canvas *painter) {
	if len(a.frames) == 0 {
		return
	}
	if a.owner != canvas.target {
		a.releaseBitmaps()
		a.owner = canvas.target
	}
	frame := &a.frames[a.index]
	if frame.bitmap == 0 {
		frame.bitmap = bitmapFromRGBA(canvas.target, frame.pixels, frame.width, frame.height, frame.width*4)
	}
	scale := min(float64(canvas.width)/float64(frame.width), float64(canvas.height)/float64(frame.height))
	w, h := float32(float64(frame.width)*scale), float32(float64(frame.height)*scale)
	canvas.bitmap(frame.bitmap, (canvas.width-w)/2, (canvas.height-h)/2, w, h, 1, false)
}

func (a *animationExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	if len(a.frames) == 0 {
		return d2d1SizeF{}, true
	}
	return d2d1SizeF{width: float32(a.frames[0].width), height: float32(a.frames[0].height)}, true
}

// schedule shows the next frame when the current one has shown for its
// duration. A frame without a duration shows for 100 milliseconds.
func (a *animationExtension) schedule(view *nativeView) {
	duration := a.frames[a.index].duration
	if duration <= 0 {
		duration = 100 * time.Millisecond
	}
	cancelTimer(a.timer)
	a.timer = startTimer(uint32(duration.Milliseconds()), func() bool {
		a.timer = 0
		if !view.live() || !a.playing {
			return false
		}
		if a.index+1 >= len(a.frames) && !a.repeats {
			a.playing = false
			return false
		}
		a.index = (a.index + 1) % len(a.frames)
		view.invalidate()
		a.schedule(view)
		return false
	})
}

// newAnimatedImage creates an animated image. Frames without an image are
// left out.
func (win *nativeWindow) newAnimatedImage(frames []ui.AnimationFrame, repeats bool) *nativeView {
	animation := &animationExtension{repeats: repeats}
	for _, frame := range frames {
		if frame.Image == nil || frame.Image.Bounds().Empty() {
			continue
		}
		pixels, width, height := rgbaPixels(frame.Image)
		animation.frames = append(animation.frames, animationFrame{
			pixels:   pixels,
			width:    width,
			height:   height,
			duration: frame.Duration,
		})
	}
	return createDrawn(win, kindAnimation, animation, false)
}

func (view *nativeView) playAnimation(play bool) {
	if !view.live() || view.kind != kindAnimation {
		return
	}
	animation := view.ext.(*animationExtension)
	count := len(animation.frames)
	was := animation.playing
	animation.playing = play && count > 1
	if !animation.playing {
		cancelTimer(animation.timer)
		animation.timer = 0
		return
	}
	if !was {
		if !animation.repeats && animation.index+1 >= count {
			animation.index = 0
			view.invalidate()
		}
		animation.schedule(view)
	}
}

func (view *nativeView) isAnimationPlaying() bool {
	return view.live() && view.kind == kindAnimation && view.ext.(*animationExtension).playing
}

// ---- Video ----
//
// A video plays with MFPlay of Media Foundation in a window of its own. Its
// controls are a native button and a trackbar below the picture.

const videoClass = "GameKitVideo"

type videoExtension struct {
	baseExtension
	player   comObject
	picture  hwnd
	button   hwnd
	track    hwnd
	controls bool
	seeking  bool
	timer    uintptr
}

func (v *videoExtension) free() {
	cancelTimer(v.timer)
	v.shutdown()
}

func (v *videoExtension) shutdown() {
	if v.player != 0 {
		win32MFPMediaPlayerShutdown(v.player)
		win32UnknownRelease(v.player)
		v.player = 0
	}
}

func (v *videoExtension) playing() bool {
	var state uint32
	return v.player != 0 && win32MFPMediaPlayerGetState(v.player, &state).succeeded() && state == mfpMediaPlayerStatePlaying
}

// seconds returns the duration or the position of the video.
func (v *videoExtension) seconds(duration bool) float64 {
	if v.player == 0 {
		return 0
	}
	var value propVariant
	var result hresult
	if duration {
		result = win32MFPMediaPlayerGetDuration(v.player, guidNull, &value)
	} else {
		result = win32MFPMediaPlayerGetPosition(v.player, guidNull, &value)
	}
	seconds := 0.0
	if result.succeeded() && value.vt == vtI8 {
		seconds = float64(value.int64Value()) / 1e7
	}
	win32PropVariantClear(&value)
	return seconds
}

func (v *videoExtension) sync() {
	if v.button == 0 {
		return
	}
	glyph := "\uE768"
	if v.playing() {
		glyph = "\uE769"
	}
	if windowText(v.button) != glyph {
		win32SetWindowText(v.button, glyph)
	}
	if !v.seeking {
		var steps uintptr
		if duration := v.seconds(true); duration > 0 {
			steps = uintptr(duration * 10)
		}
		win32SendMessage(v.track, tbmSetRangeMax, 0, steps)
		win32SendMessage(v.track, tbmSetPos, 1, uintptr(v.seconds(false)*10))
	}
}

func (v *videoExtension) layout(view *nativeView) {
	var client rect
	win32GetClientRect(view.outer, &client)
	scale := view.scale()
	var bar int32
	if v.controls {
		bar = int32(math.Round(32 * scale))
	}
	win32SetWindowPos(v.picture, 0, 0, 0, client.right, max(client.bottom-bar, 0), swpNoZOrder|swpNoActivate)
	if v.controls {
		size := bar - int32(math.Round(4*scale))
		top := client.bottom - bar + int32(math.Round(2*scale))
		win32SetWindowPos(v.button, 0, int32(math.Round(4*scale)), top, size, size, swpNoZOrder|swpNoActivate)
		left := size + int32(math.Round(8*scale))
		win32SetWindowPos(v.track, 0, left, top, client.right-left-int32(math.Round(4*scale)), size,
			swpNoZOrder|swpNoActivate)
	}
	if v.player != 0 {
		win32MFPMediaPlayerUpdateVideo(v.player)
	}
}

func (v *videoExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if h == view.outer && message == wmSize {
		v.layout(view)
	}
	return false
}

func (v *videoExtension) command(view *nativeView, from hwnd, code int) bool {
	if from != v.button || code != bnClicked || v.player == 0 {
		return false
	}
	if v.playing() {
		win32MFPMediaPlayerPause(v.player)
	} else {
		win32MFPMediaPlayerPlay(v.player)
	}
	v.sync()
	return true
}

func (v *videoExtension) seek(position int64) {
	value := propVariant{vt: vtI8}
	value.setInt64(position)
	win32MFPMediaPlayerSetPosition(v.player, guidNull, &value)
}

func (v *videoExtension) scroll(view *nativeView, from hwnd, code int) bool {
	if from != v.track {
		return false
	}
	v.seeking = code == tbThumbTrack
	if code != tbThumbTrack && v.player != 0 {
		v.seek(int64(win32SendMessage(v.track, tbmGetPos, 0, 0)) * 1000000)
	}
	return true
}

func (v *videoExtension) themeChanged(view *nativeView) {
	if v.button != 0 {
		applyThemeTo(v.button, view.theme(), "")
		applyThemeTo(v.track, view.theme(), "")
	}
}

func (v *videoExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	height := float32(180)
	if v.controls {
		height = 212
	}
	return d2d1SizeF{width: 320, height: height}, true
}

func videoProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	if message != wmPaint && message != wmEraseBkgnd {
		return win32DefWindowProc(h, message, wparam, lparam)
	}
	var video *videoExtension
	if view := viewFromDescendant(h); view != nil {
		video, _ = view.ext.(*videoExtension)
	}
	var paint paintStruct
	dc := win32BeginPaint(h, &paint)
	if video != nil && video.player != 0 {
		win32MFPMediaPlayerUpdateVideo(video.player)
	} else {
		win32FillRect(dc, &paint.rcPaint, hbrush(win32GetStockObject(blackBrush)))
	}
	win32EndPaint(h, &paint)
	if message == wmEraseBkgnd {
		return 1
	}
	return 0
}

// mediaPath returns a URL as it is and a file as its full path.
func mediaPath(source string) string {
	if strings.Contains(source, "://") {
		return source
	}
	if path, err := filepath.Abs(source); err == nil {
		return path
	}
	return source
}

var videoClassRegistered bool

func (win *nativeWindow) newVideo(source string, controls bool, autoplay bool, muted bool) *nativeView {
	if !videoClassRegistered {
		videoClassRegistered = true
		class := wndClassEx{lpfnWndProc: callback("video", videoProc), hInstance: instance(),
			hCursor: win32LoadCursor(0, idcArrow)}
		registerClass(&class, videoClass)
	}
	video := &videoExtension{controls: controls}
	view := createDrawn(win, kindVideo, video, false)
	video.picture = win32CreateWindowEx(0, videoClass, "", wsChild|wsVisible|wsClipSiblings, 0, 0, 0, 0, view.outer, 0,
		instance(), nil)
	if controls {
		video.button = makeNative(wcButton, "\uE768", wsChild|wsVisible|wsTabStop|bsPushButton, 0, view.outer)
		video.track = makeNative(trackbarClass, "", wsChild|wsVisible|wsTabStop|tbsNoTicks|tbsBoth, 0, view.outer)
		setViewOf(video.button, view)
		setViewOf(video.track, view)
		subclass(video.button)
		subclass(video.track)
		dpi := win32GetDpiForWindow(view.outer)
		if dpi == 0 {
			dpi = 96
		}
		symbol := fontSpec{family: symbolFamily(), size: 12, weight: 400}
		win32SendMessage(video.button, wmSetFont, uintptr(fontHandle(symbol, dpi)), 1)
		win32SetWindowText(video.button, "\uE768")
		video.themeChanged(view)
	}
	if win32MFPCreateMediaPlayer(mediaPath(source), autoplay, 0, 0, video.picture, &video.player).succeeded() && muted {
		win32MFPMediaPlayerSetMute(video.player, true)
	}
	video.timer = startTimer(250, func() bool {
		if !view.live() {
			return false
		}
		video.sync()
		return true
	})
	return view
}

func videoOf(view *nativeView) *videoExtension {
	if !view.live() || view.kind != kindVideo {
		return nil
	}
	video, _ := view.ext.(*videoExtension)
	return video
}

func (view *nativeView) playVideo(play bool) {
	video := videoOf(view)
	if video == nil || video.player == 0 {
		return
	}
	if play {
		win32MFPMediaPlayerPlay(video.player)
	} else {
		win32MFPMediaPlayerPause(video.player)
	}
	video.sync()
}

func (view *nativeView) isVideoPlaying() bool {
	video := videoOf(view)
	return video != nil && video.playing()
}

func (view *nativeView) videoPosition() float64 {
	if video := videoOf(view); video != nil {
		return video.seconds(false)
	}
	return 0
}

func (view *nativeView) videoDuration() float64 {
	if video := videoOf(view); video != nil {
		return video.seconds(true)
	}
	return 0
}

func (view *nativeView) seekVideo(seconds float64) {
	if video := videoOf(view); video != nil && video.player != 0 {
		video.seek(int64(max(seconds, 0) * 1e7))
	}
}

func (view *nativeView) releaseVideo() {
	if video := videoOf(view); video != nil {
		video.shutdown()
	}
}

// ---- Web view ----
//
// A web view is WebView2 with the WebView2 Runtime that Windows includes. The
// backend loads the runtime from where Windows installs it, as the loader
// library of the WebView2 SDK does, so the program needs no library beside it.
// Its data folder is in the local application data of the user, because the
// folder of the executable can be read-only.

// webCompletionTable is the method table of the completion handlers of
// WebView2, which receive a result and an object.
var webCompletionTable []uintptr

// newWebCompletion returns a completion handler of WebView2 with an interface
// that calls work.
func newWebCompletion(iid *guid, work func(result hresult, object comObject)) *comServer {
	if webCompletionTable == nil {
		webCompletionTable = methodTable(callback("WebView2.Invoke", func(this, result, object uintptr) uintptr {
			if work, ok := serverObject[func(hresult, comObject)](this); ok {
				work(hresult(uint32(result)), comObject(object))
			}
			return sOK
		}))
	}
	return newCOMServer(webCompletionTable, []*guid{iid}, work)
}

var (
	webRuntimeTried  bool
	webRuntimeModule uintptr
)

// webRuntime returns the library of the WebView2 Runtime, or 0 when Windows
// does not have it.
func webRuntime() uintptr {
	if webRuntimeTried {
		return webRuntimeModule
	}
	webRuntimeTried = true
	arch := map[string]string{"arm64": "arm64", "amd64": "x64", "386": "x86"}[runtime.GOARCH]
	const key = `Software\Microsoft\EdgeUpdate\ClientState\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
	places := []struct {
		root uintptr
		view uint32
	}{
		{hkeyLocalMachine, keyWow6432Key},
		{hkeyLocalMachine, keyWow6464Key},
		{hkeyCurrentUser, keyWow6432Key},
		{hkeyCurrentUser, keyWow6464Key},
	}
	for _, place := range places {
		var handle uintptr
		if win32RegOpenKeyEx(place.root, key, 0, keyRead|place.view, &handle) != errorSuccess {
			continue
		}
		var folder [maxPath]uint16
		size := uint32((len(folder) - 1) * 2)
		status := win32RegGetValue(handle, "", "EBWebView", rrfRtRegSz, nil, unsafe.Pointer(&folder[0]), &size)
		win32RegCloseKey(handle)
		if status != errorSuccess || folder[0] == 0 {
			continue
		}
		path := goString(folder[:]) + `\EBWebView\` + arch + `\EmbeddedBrowserWebView.dll`
		if webRuntimeModule = win32LoadLibraryEx(path, loadWithAlteredSearchPath); webRuntimeModule != 0 {
			break
		}
	}
	return webRuntimeModule
}

var (
	webEnvironmentValue  comObject
	webEnvironmentFailed bool
	webWaiting           []func(comObject)
)

// webEnvironment runs work with the environment of the web views, which the
// backend creates once; work receives 0 when there is none.
func webEnvironment(work func(environment comObject)) {
	if webEnvironmentValue != 0 || webEnvironmentFailed {
		work(webEnvironmentValue)
		return
	}
	webWaiting = append(webWaiting, work)
	if len(webWaiting) > 1 {
		return
	}
	finish := func(environment comObject) {
		webEnvironmentValue = environment
		webEnvironmentFailed = environment == 0
		waiting := webWaiting
		webWaiting = nil
		for _, next := range waiting {
			next(environment)
		}
	}
	var create uintptr
	if runtimeModule := webRuntime(); runtimeModule != 0 {
		create = win32GetProcAddress(runtimeModule, "CreateWebViewEnvironmentWithOptionsInternal")
	}
	if create == 0 {
		finish(0)
		return
	}
	var folder string
	if local, err := os.UserCacheDir(); err == nil {
		folder = filepath.Join(local, "GameKit", "WebView2")
	}
	handler := newWebCompletion(iidWebEnvironmentHandler, func(result hresult, object comObject) {
		var environment comObject
		if result.succeeded() && object != 0 {
			win32UnknownAddRef(object)
			environment = object
		}
		finish(environment)
	})
	// The runtime type 0 is the installed runtime.
	result := win32CreateWebViewEnvironment(create, true, 0, folder, handler.com())
	win32UnknownRelease(handler.com())
	if result.failed() && len(webWaiting) > 0 {
		finish(0)
	}
}

type webExtension struct {
	baseExtension
	scripts     bool
	controller  comObject
	core        comObject
	pendingURL  string
	pendingHTML string
	// pending is 1 for a URL and 2 for HTML.
	pending int
}

func (w *webExtension) free() {
	w.close()
}

func (w *webExtension) close() {
	win32UnknownRelease(w.core)
	w.core = 0
	if w.controller != 0 {
		win32WebView2ControllerClose(w.controller)
		win32UnknownRelease(w.controller)
		w.controller = 0
	}
}

func (w *webExtension) bounds(view *nativeView) {
	if w.controller != 0 {
		var client rect
		win32GetClientRect(view.outer, &client)
		win32WebView2ControllerPutBounds(w.controller, &client)
	}
}

func (w *webExtension) flush() {
	if w.core == 0 {
		return
	}
	switch w.pending {
	case 1:
		win32WebView2Navigate(w.core, w.pendingURL)
	case 2:
		win32WebView2NavigateToString(w.core, w.pendingHTML)
	}
	w.pending = 0
}

func (w *webExtension) message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool {
	if h == view.outer && message == wmSize {
		w.bounds(view)
	}
	return false
}

func (w *webExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: 320, height: 240}, true
}

func webOf(view *nativeView) *webExtension {
	if view == nil || view.kind != kindWeb {
		return nil
	}
	web, _ := view.ext.(*webExtension)
	return web
}

func (win *nativeWindow) newWebView(scripts bool) *nativeView {
	web := &webExtension{scripts: scripts}
	view := createDrawn(win, kindWeb, web, false)
	webEnvironment(func(environment comObject) {
		if environment == 0 || !view.live() {
			return
		}
		handler := newWebCompletion(iidWebControllerHandler, func(result hresult, object comObject) {
			if !view.live() || result.failed() || object == 0 {
				return
			}
			win32UnknownAddRef(object)
			web.controller = object
			win32WebView2ControllerGetCoreWebView2(web.controller, &web.core)
			var settings comObject
			if web.core != 0 && win32WebView2GetSettings(web.core, &settings).succeeded() && settings != 0 {
				win32WebView2SettingsPutIsScriptEnabled(settings, web.scripts)
				win32UnknownRelease(settings)
			}
			web.bounds(view)
			win32WebView2ControllerPutIsVisible(web.controller, true)
			web.flush()
		})
		win32WebView2EnvironmentCreateCoreWebView2Controller(environment, view.outer, handler.com())
		win32UnknownRelease(handler.com())
	})
	return view
}

func (view *nativeView) loadURL(url string) bool {
	web := webOf(view)
	if web == nil {
		return false
	}
	parsed := parsedURL{}
	parsed.cbSize = uint32(unsafe.Sizeof(parsed))
	if win32ParseURL(url, &parsed).failed() || parsed.cchProtocol == 0 {
		return false
	}
	web.pendingURL = url
	web.pending = 1
	web.flush()
	return true
}

func (view *nativeView) loadHTML(html string) {
	if web := webOf(view); web != nil {
		web.pendingHTML = html
		web.pending = 2
		web.flush()
	}
}

func (view *nativeView) webViewURL() string {
	web := webOf(view)
	if web == nil {
		return ""
	}
	if web.core == 0 {
		if web.pending == 1 {
			return web.pendingURL
		}
		return ""
	}
	var source uintptr
	if win32WebView2GetSource(web.core, &source).failed() || source == 0 {
		return ""
	}
	defer win32CoTaskMemFree(source)
	return goStringAt(source)
}

func (view *nativeView) releaseWebView() {
	if web := webOf(view); web != nil {
		web.close()
	}
}
