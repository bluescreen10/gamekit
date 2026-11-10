//go:build linux && cgo

package gtk

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

type nativeTextStyle struct {
	bold, italic, underline, strike, hasColor, noWrap bool
	size, scale                                       float64
	color                                             [4]float64
	family, url                                       string
	margin, below                                     int
}
type nativeDocument struct {
	widget, text *gtkWidget
	buffer       *gtkTextBuffer
	suppress     bool
	styles       map[nativeTextStyle]*gtkTextTag
	tags         map[*gtkTextTag]nativeTextStyle
}

func gkGTKDocumentCreate(window cUintptr, editable, markdown cInt) unsafe.Pointer {
	state := &nativeDocument{widget: gtkScrolledWindowNew(), text: gtkTextViewNew(), styles: make(map[nativeTextStyle]*gtkTextTag), tags: make(map[*gtkTextTag]nativeTextStyle)}
	state.buffer = gtkTextViewGetBuffer((*gtkTextView)(unsafe.Pointer(state.text)))
	scroll := (*gtkScrolledWindow)(unsafe.Pointer(state.widget))
	gtkScrolledWindowSetHasFrame(scroll, nativeBool(markdown == 0))
	gtkScrolledWindowSetPolicy(scroll, gtkPolicyNever, gtkPolicyAutomatic)
	text := (*gtkTextView)(unsafe.Pointer(state.text))
	gtkTextViewSetWrapMode(text, gtkWrapWordChar)
	gtkTextViewSetEditable(text, nativeBool(editable != 0))
	gtkTextViewSetCursorVisible(text, nativeBool(editable != 0))
	gtkTextViewSetLeftMargin(text, 6)
	gtkTextViewSetRightMargin(text, 6)
	gtkTextViewSetTopMargin(text, 6)
	gtkTextViewSetBottomMargin(text, 6)
	gtkScrolledWindowSetChild(scroll, state.text)
	kind := kindTextArea
	if markdown != 0 {
		kind = kindDocument
	}
	registerNativeComponent(uintptr(window), state.widget, state.text, kind)
	storeComponentState(state.widget, state)
	connectNativeSignal(unsafe.Pointer(state.buffer), "changed", func() {
		if !state.suppress {
			sendComponentEvent(state.widget, gtkEvent{kind: gkEventTextChange})
		}
	})
	click := gtkGestureClickNew()
	gtkGestureSingleSetButton((*gtkGestureSingle)(unsafe.Pointer(click)), 1)
	connectNativeArgument(unsafe.Pointer(click), "released", gtkPressedCallback(), func(x, y float64) {
		var start, end gtkTextIter
		if gtkTextBufferGetSelectionBounds(state.buffer, &start, &end) != 0 {
			return
		}
		var bx, by cInt
		gtkTextViewWindowToBufferCoords(text, gtkTextWindowWidget, cInt(x), cInt(y), &bx, &by)
		if gtkTextViewGetIterAtLocation(text, &start, bx, by) == 0 {
			return
		}
		tags := gtkTextIterGetTags(&start)
		defer gSListFree(tags)
		for item := tags; item != nil; item = item.next {
			if style, ok := state.tags[(*gtkTextTag)(item.data)]; ok && style.url != "" {
				if !queryComponent(state.widget, gtkEvent{kind: gkEventLink, text: style.url}) {
					openNativeURL(style.url)
				}
				return
			}
		}
	})
	gtkWidgetAddController(state.text, (*gtkEventController)(unsafe.Pointer(click)))
	return unsafe.Pointer(state.widget)
}
func gkGTKDocumentText(pointer unsafe.Pointer) *cChar {
	state := componentState[nativeDocument](pointer)
	if state == nil {
		return gStrdup(stringPointer(""))
	}
	var start, end gtkTextIter
	gtkTextBufferGetBounds(state.buffer, &start, &end)
	return gtkTextBufferGetText(state.buffer, &start, &end, 0)
}
func gkGTKDocumentSetText(pointer unsafe.Pointer, text *cChar) {
	if state := componentState[nativeDocument](pointer); state != nil {
		state.suppress = true
		gtkTextBufferSetText(state.buffer, text, -1)
		state.suppress = false
	}
}
func gkGTKDocumentSetEditable(pointer unsafe.Pointer, editable cInt) {
	if state := componentState[nativeDocument](pointer); state != nil {
		gtkTextViewSetEditable((*gtkTextView)(unsafe.Pointer(state.text)), nativeBool(editable != 0))
		gtkTextViewSetCursorVisible((*gtkTextView)(unsafe.Pointer(state.text)), nativeBool(editable != 0))
	}
}
func gkGTKDocumentEditable(pointer unsafe.Pointer) cInt {
	if state := componentState[nativeDocument](pointer); state != nil && gtkTextViewGetEditable((*gtkTextView)(unsafe.Pointer(state.text))) != 0 {
		return 1
	}
	return 0
}
func (document *nativeDocument) tag(style nativeTextStyle) *gtkTextTag {
	if tag := document.styles[style]; tag != nil {
		return tag
	}
	tag := gtkTextTagNew(stringPointer(fmt.Sprintf("gamekit-style-%d", len(document.styles))))
	properties := map[string]any{}
	if style.bold {
		properties["weight"] = 700
	}
	if style.italic {
		properties["style"] = int(pangoStyleItalic)
	}
	if style.underline {
		properties["underline"] = 1
	}
	if style.strike {
		properties["strikethrough"] = true
	}
	if style.size > 0 {
		properties["size-points"] = style.size
	}
	if style.family != "" {
		properties["family"] = style.family
	}
	if style.hasColor {
		properties["foreground-rgba"] = gdkRGBA{red: cFloat(style.color[0]), green: cFloat(style.color[1]), blue: cFloat(style.color[2]), alpha: cFloat(style.color[3])}
	}
	if style.scale > 0 {
		properties["scale"] = style.scale
	}
	if style.margin > 0 {
		properties["left-margin"] = style.margin
	}
	if style.below > 0 {
		properties["pixels-below-lines"] = style.below
	}
	if style.noWrap {
		properties["wrap-mode"] = 0
	}
	for name, value := range properties {
		setNativeProperty(unsafe.Pointer(tag), name, value)
	}
	gtkTextTagTableAdd(gtkTextBufferGetTagTable(document.buffer), tag)
	gObjectUnref(unsafe.Pointer(tag))
	document.styles[style] = tag
	document.tags[tag] = style
	return tag
}
func (document *nativeDocument) insert(text string, styles ...nativeTextStyle) {
	if text == "" {
		return
	}
	var end, start gtkTextIter
	gtkTextBufferGetEndIter(document.buffer, &end)
	offset := gtkTextIterGetOffset(&end)
	gtkTextBufferInsert(document.buffer, &end, stringPointer(text), cInt(len(text)))
	gtkTextBufferGetIterAtOffset(document.buffer, &start, offset)
	gtkTextBufferRemoveAllTags(document.buffer, &start, &end)
	for _, style := range styles {
		if style != (nativeTextStyle{}) {
			gtkTextBufferApplyTag(document.buffer, document.tag(style), &start, &end)
		}
	}
}
func mergeTextStyle(target, style nativeTextStyle) nativeTextStyle {
	target.bold = target.bold || style.bold
	target.italic = target.italic || style.italic
	target.underline = target.underline || style.underline
	target.strike = target.strike || style.strike
	if style.size > 0 {
		target.size = style.size
	}
	if style.hasColor {
		target.hasColor, target.color = true, style.color
	}
	if style.family != "" {
		target.family = style.family
	}
	return target
}
func (document *nativeDocument) styleAt(iter *gtkTextIter) nativeTextStyle {
	var style nativeTextStyle
	tags := gtkTextIterGetTags(iter)
	defer gSListFree(tags)
	for item := tags; item != nil; item = item.next {
		style = mergeTextStyle(style, document.tags[(*gtkTextTag)(item.data)])
	}
	return style
}
func rtfEscape(text string) string {
	var out strings.Builder
	for _, r := range text {
		switch r {
		case '\\', '{', '}':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\n':
			out.WriteString("\\par\n")
		case '\t':
			out.WriteString("\\tab ")
		default:
			if r < 0x80 {
				out.WriteRune(r)
			} else {
				for _, code := range utf16.Encode([]rune{r}) {
					fmt.Fprintf(&out, "\\u%d?", int16(code))
				}
			}
		}
	}
	return out.String()
}
func gkGTKDocumentRTF(pointer unsafe.Pointer, size *cSize) *cUchar {
	*size = 0
	document := componentState[nativeDocument](pointer)
	if document == nil {
		return nil
	}
	fonts := []string{"Sans"}
	var colors []string
	index := func(values *[]string, value string) int {
		for i, existing := range *values {
			if existing == value {
				return i
			}
		}
		*values = append(*values, value)
		return len(*values) - 1
	}
	var body strings.Builder
	var iter, end gtkTextIter
	gtkTextBufferGetBounds(document.buffer, &iter, &end)
	for gtkTextIterEqual(&iter, &end) == 0 {
		next := iter
		if gtkTextIterForwardToTagToggle(&next, nil) == 0 {
			next = end
		}
		style := document.styleAt(&iter)
		text := takeString(gtkTextBufferGetText(document.buffer, &iter, &next, 0))
		body.WriteByte('{')
		if style.bold {
			body.WriteString("\\b")
		}
		if style.italic {
			body.WriteString("\\i")
		}
		if style.underline {
			body.WriteString("\\ul")
		}
		if style.strike {
			body.WriteString("\\strike")
		}
		if style.size > 0 {
			fmt.Fprintf(&body, "\\fs%d", int(math.Round(style.size*2)))
		}
		if style.hasColor {
			color := fmt.Sprintf("\\red%d\\green%d\\blue%d;", int(math.Round(style.color[0]*255)), int(math.Round(style.color[1]*255)), int(math.Round(style.color[2]*255)))
			fmt.Fprintf(&body, "\\cf%d", index(&colors, color)+1)
		}
		if style.family != "" {
			fmt.Fprintf(&body, "\\f%d", index(&fonts, style.family))
		}
		body.WriteByte(' ')
		body.WriteString(rtfEscape(text))
		body.WriteByte('}')
		iter = next
	}
	var output strings.Builder
	output.WriteString("{\\rtf1\\ansi\\ansicpg1252\\deff0{\\fonttbl")
	for i, font := range fonts {
		fmt.Fprintf(&output, "{\\f%d %s;}", i, rtfEscape(font))
	}
	output.WriteString("}{\\colortbl;")
	for _, color := range colors {
		output.WriteString(color)
	}
	output.WriteString("}\n")
	output.WriteString(body.String())
	output.WriteString("}\n")
	data := []byte(output.String())
	*size = cSize(len(data))
	memory := gMalloc(gSize(len(data)))
	copy(unsafe.Slice((*byte)(memory), len(data)), data)
	return (*cUchar)(memory)
}

type rtfState struct {
	style             nativeTextStyle
	destination       int
	skip              bool
	unicodeSkip, font int
}

var rtfWindows1252 = []rune{
	'€',
	'\u0081',
	'‚',
	'ƒ',
	'„',
	'…',
	'†',
	'‡',
	'ˆ',
	'‰',
	'Š',
	'‹',
	'Œ',
	'\u008d',
	'Ž',
	'\u008f',
	'\u0090',
	'‘',
	'’',
	'“',
	'”',
	'•',
	'–',
	'—',
	'˜',
	'™',
	'š',
	'›',
	'œ',
	'\u009d',
	'ž',
	'Ÿ',
}

func gkGTKDocumentSetRTF(pointer unsafe.Pointer, rtf *cUchar, size cSize) cInt {
	document := componentState[nativeDocument](pointer)
	if document == nil || rtf == nil {
		return 0
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(rtf)), int(size))
	if !strings.HasPrefix(string(data), "{\\rtf") {
		return 0
	}
	document.suppress = true
	defer func() { document.suppress = false }()
	gtkTextBufferSetText(document.buffer, stringPointer(""), 0)
	stack := []rtfState{{unicodeSkip: 1}}
	fonts := map[int]string{}
	var colors [][4]float64
	color := [4]float64{0, 0, 0, 1}
	var font strings.Builder
	skipChars := 0
	var high rune
	emit := func(r rune) {
		state := stack[len(stack)-1]
		if state.skip {
			return
		}
		if state.destination == 1 {
			if r == ';' {
				fonts[state.font] = strings.TrimSpace(font.String())
				font.Reset()
			} else {
				font.WriteRune(r)
			}
			return
		}
		if state.destination == 2 {
			if r == ';' {
				colors = append(colors, color)
				color = [4]float64{0, 0, 0, 1}
			}
			return
		}
		if r >= 0xd800 && r <= 0xdbff {
			high = r
			return
		}
		if high != 0 {
			if r >= 0xdc00 && r <= 0xdfff {
				r = utf16.DecodeRune(high, r)
			} else {
				document.insert(string(utf8.RuneError), state.style)
			}
			high = 0
		}
		document.insert(string(r), state.style)
	}
	for at := 0; at < len(data); {
		character := data[at]
		at++
		switch character {
		case '{':
			stack = append(stack, stack[len(stack)-1])
			continue
		case '}':
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		case '\n', '\r':
			continue
		case '\\':
			if at >= len(data) {
				continue
			}
			character = data[at]
			at++
			if character == '*' {
				stack[len(stack)-1].skip = true
				continue
			}
			if character == '\'' && at+2 <= len(data) {
				value, err := strconv.ParseUint(string(data[at:at+2]), 16, 8)
				at += 2
				if skipChars > 0 {
					skipChars--
					continue
				}
				if err == nil {
					r := rune(value)
					if r >= 0x80 && r < 0xa0 {
						r = rtfWindows1252[r-0x80]
					}
					emit(r)
				}
				continue
			}
			if character == '\\' || character == '{' || character == '}' {
				if skipChars > 0 {
					skipChars--
				} else {
					emit(rune(character))
				}
				continue
			}
			if character == '~' {
				emit(0xa0)
				continue
			}
			if character < 'A' || character > 'Z' && character < 'a' || character > 'z' {
				continue
			}
			start := at - 1
			for at < len(data) && (data[at] >= 'a' && data[at] <= 'z' || data[at] >= 'A' && data[at] <= 'Z') {
				at++
			}
			word := string(data[start:at])
			start = at
			if at < len(data) && data[at] == '-' {
				at++
			}
			for at < len(data) && data[at] >= '0' && data[at] <= '9' {
				at++
			}
			hasValue := at > start
			value, _ := strconv.Atoi(string(data[start:at]))
			if at < len(data) && data[at] == ' ' {
				at++
			}
			state := &stack[len(stack)-1]
			switch word {
			case "fonttbl":
				state.destination = 1
			case "colortbl":
				state.destination = 2
			case "stylesheet", "info", "pict", "header", "footer", "object":
				state.skip = true
			case "b":
				state.style.bold = !hasValue || value != 0
			case "i":
				state.style.italic = !hasValue || value != 0
			case "ul":
				state.style.underline = !hasValue || value != 0
			case "ulnone":
				state.style.underline = false
			case "strike":
				state.style.strike = !hasValue || value != 0
			case "fs":
				state.style.size = float64(value) / 2
			case "plain":
				state.style = nativeTextStyle{}
			case "f":
				if state.destination == 1 {
					state.font = value
				} else {
					state.style.family = fonts[value]
				}
			case "cf":
				state.style.hasColor = value > 0 && value < len(colors)
				if state.style.hasColor {
					state.style.color = colors[value]
				}
			case "red":
				if state.destination == 2 {
					color[0] = float64(value) / 255
				}
			case "green":
				if state.destination == 2 {
					color[1] = float64(value) / 255
				}
			case "blue":
				if state.destination == 2 {
					color[2] = float64(value) / 255
				}
			case "par", "line":
				emit('\n')
			case "tab":
				emit('\t')
			case "uc":
				state.unicodeSkip = max(0, value)
			case "u":
				emit(rune(uint16(value)))
				skipChars = state.unicodeSkip
			}
			continue
		}
		if skipChars > 0 {
			skipChars--
			continue
		}
		r := rune(character)
		if r >= 0x80 && r < 0xa0 {
			r = rtfWindows1252[r-0x80]
		}
		emit(r)
	}
	return 1
}

var markdownNumber = regexp.MustCompile(`^([0-9]+[.)] )`)

func (document *nativeDocument) markdownInline(text string, styles ...nativeTextStyle) {
	var plain strings.Builder
	flush := func() { document.insert(plain.String(), styles...); plain.Reset() }
	for len(text) > 0 {
		if len(text) > 1 && text[0] == '\\' && strings.ContainsRune("\\`*_{}[]()#+-.!>", rune(text[1])) {
			plain.WriteByte(text[1])
			text = text[2:]
			continue
		}
		marker := ""
		style := nativeTextStyle{}
		switch {
		case strings.HasPrefix(text, "`"):
			marker = "`"
			style.family = "Monospace"
		case strings.HasPrefix(text, "**"), strings.HasPrefix(text, "__"):
			marker = text[:2]
			style.bold = true
		case strings.HasPrefix(text, "*"), strings.HasPrefix(text, "_"):
			marker = text[:1]
			style.italic = true
		case strings.HasPrefix(text, "["):
			if label := strings.Index(text, "]("); label >= 0 {
				if end := strings.IndexByte(text[label+2:], ')'); end >= 0 {
					flush()
					style = nativeTextStyle{underline: true, hasColor: true, color: [4]float64{0.1, 0.4, 0.85, 1}, url: text[label+2 : label+2+end]}
					document.insert(text[1:label], append(append([]nativeTextStyle(nil), styles...), style)...)
					text = text[label+3+end:]
					continue
				}
			}
		}
		if marker != "" {
			if end := strings.Index(text[len(marker):], marker); end > 0 {
				flush()
				document.insert(text[len(marker):len(marker)+end], append(append([]nativeTextStyle(nil), styles...), style)...)
				text = text[len(marker)*2+end:]
				continue
			}
		}
		plain.WriteByte(text[0])
		text = text[1:]
	}
	flush()
}

func gkGTKMarkdownSet(pointer unsafe.Pointer, source *cChar) cInt {
	document := componentState[nativeDocument](pointer)
	text := goString(source)
	if document == nil || !utf8.ValidString(text) {
		return 0
	}
	document.suppress = true
	defer func() { document.suppress = false }()
	gtkTextBufferSetText(document.buffer, stringPointer(""), 0)
	var paragraph []string
	code, first := false, true
	flush := func() {
		if len(paragraph) > 0 {
			document.markdownInline(strings.Join(paragraph, " "))
			document.insert("\n")
			paragraph = nil
			first = false
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			flush()
			code = !code
			continue
		}
		if code {
			document.insert(line, nativeTextStyle{family: "Monospace", margin: 18, noWrap: true})
			document.insert("\n")
			continue
		}
		line = strings.TrimLeft(line, " ")
		heading := len(line) - len(strings.TrimLeft(line, "#"))
		isHeading := heading > 0 && heading <= 6 && (heading == len(line) || line[heading] == ' ')
		quote := strings.HasPrefix(line, ">")
		bullet := strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ")
		number := markdownNumber.FindString(line)
		rule := line == "---" || line == "***" || line == "___"
		if line == "" || isHeading || quote || bullet || number != "" || rule {
			flush()
		}
		switch {
		case line == "":
			if !first {
				document.insert("\n")
			}
			continue
		case isHeading:
			style := nativeTextStyle{bold: true}
			switch heading {
			case 1:
				style.scale, style.below = 1.8, 6
			case 2:
				style.scale, style.below = 1.5, 4
			case 3:
				style.scale = 1.25
			}
			document.markdownInline(strings.TrimLeft(line[heading:], " "), style)
		case quote:
			document.markdownInline(strings.TrimLeft(line[1:], " "), nativeTextStyle{margin: 24, italic: true, hasColor: true, color: [4]float64{0.5, 0.5, 0.5, 1}})
		case bullet:
			document.insert("• ", nativeTextStyle{margin: 24})
			document.markdownInline(line[2:], nativeTextStyle{margin: 24})
		case number != "":
			document.insert(number, nativeTextStyle{margin: 24})
			document.markdownInline(line[len(number):], nativeTextStyle{margin: 24})
		case rule:
			document.insert("────────")
		default:
			paragraph = append(paragraph, line)
			first = false
			continue
		}
		document.insert("\n")
		first = false
	}
	if len(paragraph) > 0 {
		document.markdownInline(strings.Join(paragraph, " "))
	}
	return 1
}
