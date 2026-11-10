package win32

// The native layer of the backend, over the Win32 API and the common controls
// (version 6, with visual styles), which ship with Windows, so an application
// needs nothing else installed. Views are native controls. The few components
// that Windows has no control for are drawn with Direct2D and the visual style
// parts of Windows.

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"unsafe"
)

// iconImage is a glyph of the symbol font or an image of RGBA pixels.
type iconImage struct {
	glyph         string
	pixels        []byte
	width, height int
	pointSize     float64
}

// ---- Common controls ----

// activate activates the activation context of the common controls version 6
// and returns the cookie that deactivate takes:
//
//	defer deactivate(activate())
func activate() uintptr {
	return activateCommonControls()
}

func deactivate(cookie uintptr) {
	win32DeactivateActCtx(0, cookie)
}

// ---- Theme ----

// themeColor is a straight (not premultiplied) RGBA color.
type themeColor struct {
	r, g, b, a float32
}

// rgbColor returns an opaque color from 0xRRGGBB.
func rgbColor(value uint32) themeColor {
	return rgbAlpha(value, 1)
}

func rgbAlpha(value uint32, alpha float32) themeColor {
	return themeColor{
		r: float32((value>>16)&0xff) / 255,
		g: float32((value>>8)&0xff) / 255,
		b: float32(value&0xff) / 255,
		a: alpha,
	}
}

// mix returns above drawn over below.
func mix(below, above themeColor) themeColor {
	a := above.a
	return themeColor{
		r: below.r*(1-a) + above.r*a,
		g: below.g*(1-a) + above.g*a,
		b: below.b*(1-a) + above.b*a,
		a: 1,
	}
}

// colorRef returns a color as a COLORREF.
func (c themeColor) colorRef() uint32 {
	r := uint32(math.Round(float64(c.r) * 255))
	g := uint32(math.Round(float64(c.g) * 255))
	b := uint32(math.Round(float64(c.b) * 255))
	return r | g<<8 | b<<16
}

func (c themeColor) d2d() d2d1ColorF {
	return d2d1ColorF{r: c.r, g: c.g, b: c.b, a: c.a}
}

// systemDark reports whether apps use the dark theme in the system settings.
func systemDark() bool {
	var light uint32 = 1
	size := uint32(4)
	if win32RegGetValue(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		"AppsUseLightTheme", rrfRtRegDword, nil, unsafe.Pointer(&light), &size) != errorSuccess {
		return false
	}
	return light == 0
}

// accentShade returns a shade of the accent color from the accent palette of
// the system: 0 the lightest through 6 the darkest, 3 the accent itself.
func accentShade(shade int, fallback uint32) themeColor {
	var palette [32]byte
	size := uint32(len(palette))
	if win32RegGetValue(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`,
		"AccentPalette", rrfRtRegBinary, nil, unsafe.Pointer(&palette[0]), &size) == errorSuccess && size >= 28 {
		entry := palette[shade*4:]
		return rgbColor(uint32(entry[0])<<16 | uint32(entry[1])<<8 | uint32(entry[2]))
	}
	return rgbColor(fallback)
}

// accentColor returns the accent color of the system.
func accentColor() themeColor {
	return accentShade(3, 0x0078d4)
}

// palette holds the colors of the Windows 11 design for the light or dark
// theme.
type palette struct {
	window                themeColor // the background of a window
	layer                 themeColor // a raised surface, such as a card
	layerStroke           themeColor // the border of a raised surface
	text                  themeColor // primary text
	textSecondary         themeColor // secondary text
	textTertiary          themeColor // placeholders
	textDisabled          themeColor // disabled text
	textOnAccent          themeColor // text on the accent color
	control               themeColor // the fill of a control at rest
	controlHover          themeColor // the fill of a control under the pointer
	controlPressed        themeColor // the fill of a pressed control
	controlDisabled       themeColor
	controlStroke         themeColor // the border of a control
	controlStrokeBottom   themeColor // the lower edge of a control's border
	controlStrong         themeColor // the outline of an unchecked box or a track
	controlStrongDisabled themeColor
	inputActive           themeColor // the fill of a focused text box
	subtle                themeColor // the fill of a subtle control under the pointer
	subtlePressed         themeColor
	accent                themeColor
	accentHover           themeColor
	accentPressed         themeColor
	accentDisabled        themeColor
	accentText            themeColor // text in the accent color, such as a link
	divider               themeColor
	focus                 themeColor // the outer ring of the focus rectangle
	success               themeColor // the severity colors
	caution               themeColor
	critical              themeColor
	information           themeColor
	successBackground     themeColor
	cautionBackground     themeColor
	criticalBackground    themeColor
	informationBackground themeColor
	selection             themeColor // the background of selected list items
	flyout                themeColor // the background of a popup
	dark                  bool
}

func makePalette(dark bool) *palette {
	if !dark {
		return &palette{
			window:                rgbColor(0xffffff),
			layer:                 rgbColor(0xf9f9f9),
			layerStroke:           rgbColor(0xe5e5e5),
			text:                  rgbColor(0x000000),
			textSecondary:         rgbColor(0x5d5d5d),
			textTertiary:          rgbColor(0x8a8a8a),
			textDisabled:          rgbColor(0xa0a0a0),
			textOnAccent:          rgbColor(0xffffff),
			control:               rgbColor(0xfdfdfd),
			controlHover:          rgbColor(0xe5f1fb),
			controlPressed:        rgbColor(0xcce4f7),
			controlDisabled:       rgbColor(0xf4f4f4),
			controlStroke:         rgbColor(0xadadad),
			controlStrokeBottom:   rgbColor(0xadadad),
			controlStrong:         rgbColor(0x333333),
			controlStrongDisabled: rgbColor(0xbfbfbf),
			inputActive:           rgbColor(0xffffff),
			subtle:                rgbColor(0xe5f3ff),
			subtlePressed:         rgbColor(0xcce8ff),
			accent:                accentShade(4, 0x005fb8),
			accentHover:           accentShade(3, 0x0078d4),
			accentPressed:         accentShade(5, 0x003e92),
			accentDisabled:        rgbColor(0xbfbfbf),
			accentText:            rgbColor(0x0066cc),
			divider:               rgbColor(0xe0e0e0),
			focus:                 rgbColor(0x000000),
			success:               rgbColor(0x0f7b0f),
			caution:               rgbColor(0x9d5d00),
			critical:              rgbColor(0xc42b1c),
			information:           accentShade(4, 0x005fb8),
			successBackground:     rgbColor(0xdff6dd),
			cautionBackground:     rgbColor(0xfff4ce),
			criticalBackground:    rgbColor(0xfde7e9),
			informationBackground: rgbColor(0xebf3fb),
			selection:             rgbColor(0xcce8ff),
			flyout:                rgbColor(0xf9f9f9),
		}
	}
	return &palette{
		window:                rgbColor(0x202020),
		layer:                 rgbColor(0x2b2b2b),
		layerStroke:           rgbColor(0x3a3a3a),
		text:                  rgbColor(0xffffff),
		textSecondary:         rgbColor(0xc5c5c5),
		textTertiary:          rgbColor(0x8b8b8b),
		textDisabled:          rgbColor(0x6d6d6d),
		textOnAccent:          rgbColor(0x000000),
		control:               rgbColor(0x333333),
		controlHover:          rgbColor(0x3c3c3c),
		controlPressed:        rgbColor(0x2a2a2a),
		controlDisabled:       rgbColor(0x2a2a2a),
		controlStroke:         rgbColor(0x4a4a4a),
		controlStrokeBottom:   rgbColor(0x4a4a4a),
		controlStrong:         rgbColor(0x9a9a9a),
		controlStrongDisabled: rgbColor(0x4a4a4a),
		inputActive:           rgbColor(0x1c1c1c),
		subtle:                rgbColor(0x353535),
		subtlePressed:         rgbColor(0x2e2e2e),
		accent:                accentShade(1, 0x60cdff),
		accentHover:           accentShade(2, 0x4cc2ff),
		accentPressed:         accentShade(2, 0x4cc2ff),
		accentDisabled:        rgbColor(0x4a4a4a),
		accentText:            accentShade(0, 0x99ebff),
		divider:               rgbColor(0x3d3d3d),
		focus:                 rgbColor(0xffffff),
		success:               rgbColor(0x6ccb5f),
		caution:               rgbColor(0xfce100),
		critical:              rgbColor(0xff99a4),
		information:           accentShade(1, 0x60cdff),
		successBackground:     rgbColor(0x393d1b),
		cautionBackground:     rgbColor(0x433519),
		criticalBackground:    rgbColor(0x442726),
		informationBackground: rgbColor(0x2b2b2b),
		selection:             rgbColor(0x3a3a3a),
		flyout:                rgbColor(0x2c2c2c),
		dark:                  true,
	}
}

var (
	lightPalette, darkPalette *palette
)

// paletteFor returns the palette of a theme: 1 light, 2 dark.
func paletteFor(theme int) *palette {
	if lightPalette == nil {
		lightPalette = makePalette(false)
		darkPalette = makePalette(true)
	}
	if theme == 2 {
		return darkPalette
	}
	return lightPalette
}

// semanticColor returns the color of a ui.SemanticColor, if it has one.
func semanticColor(p *palette, color int) (themeColor, bool) {
	switch color {
	case 1:
		return p.text, true
	case 2:
		return p.textSecondary, true
	case 3:
		return p.textTertiary, true
	case 4:
		return p.layer, true
	case 5:
		return p.accent, true
	case 6:
		return p.success, true
	case 7:
		return p.caution, true
	case 8:
		return p.critical, true
	case 9:
		return p.accentText, true
	default:
		return themeColor{}, false
	}
}

// The dark mode of the controls of Windows is reached through functions that
// uxtheme.dll exports by ordinal.
const appModeAllowDark = 1

var uxThemeInitialized bool

func initializeUxTheme() {
	if uxThemeInitialized {
		return
	}
	uxThemeInitialized = true
	if procSetPreferredAppMode.find() != 0 {
		win32SetPreferredAppMode(appModeAllowDark)
	}
	if procFlushMenuThemes.find() != 0 {
		win32FlushMenuThemes()
	}
}

func allowDark(h hwnd, dark bool) {
	if procAllowDarkModeForWindow.find() != 0 {
		win32AllowDarkModeForWindow(h, dark)
	}
}

// applyThemeTo gives a native control the visual style of a theme. lightClass
// is the visual style of the light theme, or "" for the default.
func applyThemeTo(h hwnd, theme int, lightClass string) {
	if h == 0 {
		return
	}
	name := className(h)
	dark := theme == 2
	allowDark(h, dark)
	var style string
	switch name {
	case wcEdit, wcComboBox, "RICHEDIT50W":
		if dark {
			style = "DarkMode_CFD"
		} else {
			style = lightClass
		}
	case wcListView, wcTreeView:
		if dark {
			style = "DarkMode_Explorer"
		} else {
			style = "Explorer"
		}
	default:
		if dark {
			style = "DarkMode_Explorer"
		} else {
			style = lightClass
		}
	}
	win32SetWindowTheme(h, style, "")
	win32SendMessage(h, wmThemeChanged, 0, 0)
}

// className returns the name of the window class of a window.
func className(h hwnd) string {
	var name [64]uint16
	win32GetClassName(h, &name[0], int32(len(name)))
	return goString(name[:])
}

// ---- Fonts ----

// fontSpec describes the font of text in logical units (DIPs).
type fontSpec struct {
	family string
	size   float32
	weight int
	italic bool
}

var messageFont *fontSpec

func systemMessageFont() fontSpec {
	if messageFont != nil {
		return *messageFont
	}
	font := fontSpec{weight: 400}
	var metrics nonClientMetrics
	metrics.cbSize = uint32(unsafe.Sizeof(metrics))
	if win32SystemParametersInfoForDpi(spiGetNonClientMetrics, metrics.cbSize, unsafe.Pointer(&metrics), 0, 96) {
		font.family = goString(metrics.lfMessageFont.lfFaceName[:])
		font.size = float32(abs(int(metrics.lfMessageFont.lfHeight)))
	}
	if font.family == "" {
		font.family = "Segoe UI"
	}
	if font.size <= 0 {
		font.size = 12
	}
	messageFont = &font
	return font
}

// bodyFont returns the font of body text.
func bodyFont() fontSpec {
	return systemMessageFont()
}

// typeRamp returns a font of the Windows type ramp: 0 body, 1 caption, 2 body
// strong, 3 subtitle, 4 title, 5 title large, 6 display, 7 icon.
func typeRamp(style int) fontSpec {
	font := systemMessageFont()
	switch style {
	case 1:
		font.size--
	case 2:
		font.weight = 600
	case 3:
		font.size = font.size * 4 / 3
		font.weight = 600
	case 4:
		font.size = font.size * 5 / 3
		font.weight = 600
	case 5:
		font.size = font.size * 7 / 3
		font.weight = 600
	case 6:
		font.size = font.size * 10 / 3
		font.weight = 600
	case 7:
		font.family = symbolFamily()
	}
	return font
}

var symbolFontFamily string

// symbolFamily returns the family of the symbol font.
func symbolFamily() string {
	if symbolFontFamily == "" {
		if fontFamilyInstalled("Segoe Fluent Icons") {
			symbolFontFamily = "Segoe Fluent Icons"
		} else {
			symbolFontFamily = "Segoe MDL2 Assets"
		}
	}
	return symbolFontFamily
}

type gdiFontKey struct {
	font fontSpec
	dpi  uint32
}

var gdiFonts = map[gdiFontKey]hfont{}

// fontHandle returns a GDI font for a native control at dpi. The cache owns
// the font.
func fontHandle(font fontSpec, dpi uint32) hfont {
	key := gdiFontKey{font, dpi}
	if handle, ok := gdiFonts[key]; ok {
		return handle
	}
	handle := win32CreateFont(-int32(math.Round(float64(font.size)*float64(dpi)/96)), 0, 0, 0, int32(font.weight),
		font.italic, false, false, defaultCharset, outDefaultPrecis, clipDefaultPrecis, cleartypeQuality,
		defaultPitch, font.family)
	gdiFonts[key] = handle
	return handle
}

// fontFamilyInstalled reports whether a font family is installed.
func fontFamilyInstalled(family string) bool {
	factory := dwriteFactory()
	if factory == 0 {
		return false
	}
	var collection comObject
	if win32DWriteFactoryGetSystemFontCollection(factory, &collection, false).failed() || collection == 0 {
		return false
	}
	defer win32UnknownRelease(collection)
	var index uint32
	var exists int32
	win32DWriteFontCollectionFindFamilyName(collection, family, &index, &exists)
	return exists != 0
}

// ---- Direct2D ----

var (
	d2dFactoryObject    comObject
	dwriteFactoryObject comObject
	textFormats         = map[fontSpec]comObject{}
)

func d2dFactory() comObject {
	if d2dFactoryObject == 0 {
		win32D2D1CreateFactory(d2d1FactoryTypeSingleThreaded, iidD2D1Factory, nil, &d2dFactoryObject)
	}
	return d2dFactoryObject
}

func dwriteFactory() comObject {
	if dwriteFactoryObject == 0 {
		win32DWriteCreateFactory(dwriteFactoryTypeShared, iidDWriteFactory, &dwriteFactoryObject)
	}
	return dwriteFactoryObject
}

// textFormat returns the DirectWrite text format of a font, which the cache
// owns.
func textFormat(font fontSpec) comObject {
	if format, ok := textFormats[font]; ok {
		return format
	}
	var format comObject
	if factory := dwriteFactory(); factory != 0 {
		style := uint32(dwriteFontStyleNormal)
		if font.italic {
			style = dwriteFontStyleItalic
		}
		win32DWriteFactoryCreateTextFormat(factory, font.family, 0, uint32(font.weight), style, dwriteFontStretchNormal,
			font.size, "", &format)
	}
	textFormats[font] = format
	return format
}

// painter draws with Direct2D on the device context of a window during
// WM_PAINT, in logical units.
type painter struct {
	target comObject
	brush  comObject
	// width and height are the size of the window in logical units.
	width, height float32
	// scale is the physical pixels in a logical unit.
	scale float32
}

func (p *painter) color(value themeColor) {
	if p.brush != 0 {
		color := value.d2d()
		win32D2D1SolidColorBrushSetColor(p.brush, &color)
	}
}

func (p *painter) fillRect(x, y, w, h float32, value themeColor) {
	p.color(value)
	win32D2D1RenderTargetFillRectangle(p.target, &d2d1RectF{left: x, top: y, right: x + w, bottom: y + h}, p.brush)
}

func (p *painter) fillRound(x, y, w, h, radius float32, value themeColor) {
	p.color(value)
	rounded := d2d1RoundedRect{
		rect:    d2d1RectF{left: x, top: y, right: x + w, bottom: y + h},
		radiusX: radius,
		radiusY: radius,
	}
	win32D2D1RenderTargetFillRoundedRectangle(p.target, &rounded, p.brush)
}

func (p *painter) strokeRound(x, y, w, h, radius float32, value themeColor, thickness float32) {
	p.color(value)
	inset := thickness / 2
	rounded := d2d1RoundedRect{
		rect:    d2d1RectF{left: x + inset, top: y + inset, right: x + w - inset, bottom: y + h - inset},
		radiusX: radius,
		radiusY: radius,
	}
	win32D2D1RenderTargetDrawRoundedRectangle(p.target, &rounded, p.brush, thickness, 0)
}

func (p *painter) strokeEllipse(cx, cy, rx, ry float32, value themeColor, thickness float32) {
	p.color(value)
	ellipse := d2d1Ellipse{point: d2d1Point2F{x: cx, y: cy}, radiusX: rx, radiusY: ry}
	win32D2D1RenderTargetDrawEllipse(p.target, &ellipse, p.brush, thickness, 0)
}

func (p *painter) line(x1, y1, x2, y2 float32, value themeColor, thickness float32) {
	p.color(value)
	win32D2D1RenderTargetDrawLine(p.target, d2d1Point2F{x: x1, y: y1}, d2d1Point2F{x: x2, y: y2}, p.brush, thickness, 0)
}

func (p *painter) arc(cx, cy, radius, start, sweep float32, value themeColor, thickness float32) {
	if sweep <= 0 {
		return
	}
	if sweep >= 2*math.Pi-0.001 {
		p.strokeEllipse(cx, cy, radius, radius, value, thickness)
		return
	}
	var path comObject
	if win32D2D1FactoryCreatePathGeometry(d2dFactory(), &path).failed() || path == 0 {
		return
	}
	defer win32UnknownRelease(path)
	var sink comObject
	win32D2D1PathGeometryOpen(path, &sink)
	begin := d2d1Point2F{
		x: cx + float32(math.Cos(float64(start)))*radius,
		y: cy + float32(math.Sin(float64(start)))*radius,
	}
	win32D2D1SimplifiedGeometrySinkBeginFigure(sink, begin, d2d1FigureBeginHollow)
	end := start + sweep
	arcSize := uint32(d2d1ArcSizeSmall)
	if sweep > math.Pi {
		arcSize = d2d1ArcSizeLarge
	}
	segment := d2d1ArcSegment{
		point: d2d1Point2F{
			x: cx + float32(math.Cos(float64(end)))*radius,
			y: cy + float32(math.Sin(float64(end)))*radius,
		},
		size:           d2d1SizeF{width: radius, height: radius},
		sweepDirection: d2d1SweepDirectionClockwise,
		arcSize:        arcSize,
	}
	win32D2D1GeometrySinkAddArc(sink, &segment)
	win32D2D1SimplifiedGeometrySinkEndFigure(sink, d2d1FigureEndOpen)
	win32D2D1SimplifiedGeometrySinkClose(sink)
	win32UnknownRelease(sink)
	p.color(value)
	var round comObject
	properties := d2d1StrokeStyleProperties{startCap: d2d1CapStyleRound, endCap: d2d1CapStyleRound, miterLimit: 10}
	win32D2D1FactoryCreateStrokeStyle(d2dFactory(), &properties, nil, 0, &round)
	win32D2D1RenderTargetDrawGeometry(p.target, path, p.brush, thickness, round)
	win32UnknownRelease(round)
}

// text draws text in a box. align: 0 leading, 1 center, 2 trailing. valign:
// 0 top, 1 center, 2 bottom.
func (p *painter) text(value string, font fontSpec, x, y, w, h float32, textColor themeColor, align, valign int,
	wrap, trimming bool) {
	format := textFormat(font)
	if format == 0 || value == "" {
		return
	}
	layout := createTextLayout(value, format, max(w, 1), max(h, 1))
	if layout == 0 {
		return
	}
	defer win32UnknownRelease(layout)
	switch align {
	case 1:
		win32DWriteTextFormatSetTextAlignment(layout, dwriteTextAlignmentCenter)
	case 2:
		win32DWriteTextFormatSetTextAlignment(layout, dwriteTextAlignmentTrailing)
	default:
		win32DWriteTextFormatSetTextAlignment(layout, dwriteTextAlignmentLeading)
	}
	switch valign {
	case 1:
		win32DWriteTextFormatSetParagraphAlignment(layout, dwriteParagraphAlignmentCenter)
	case 2:
		win32DWriteTextFormatSetParagraphAlignment(layout, dwriteParagraphAlignmentFar)
	default:
		win32DWriteTextFormatSetParagraphAlignment(layout, dwriteParagraphAlignmentNear)
	}
	if wrap {
		win32DWriteTextFormatSetWordWrapping(layout, dwriteWordWrappingWrap)
	} else {
		win32DWriteTextFormatSetWordWrapping(layout, dwriteWordWrappingNoWrap)
	}
	if trimming {
		trim := dwriteTrimming{granularity: dwriteTrimmingGranularityCharacter}
		var ellipsis comObject
		win32DWriteFactoryCreateEllipsisTrimmingSign(dwriteFactory(), format, &ellipsis)
		win32DWriteTextFormatSetTrimming(layout, &trim, ellipsis)
		win32UnknownRelease(ellipsis)
	}
	p.color(textColor)
	win32D2D1RenderTargetDrawTextLayout(p.target, d2d1Point2F{x: x, y: y}, layout, p.brush, d2d1DrawTextOptionsClip)
}

// centeredText draws one line of text centered in a box by the ink of its
// glyphs, for short text such as a count in a badge.
func (p *painter) centeredText(value string, font fontSpec, x, y, w, h float32, textColor themeColor) {
	format := textFormat(font)
	if format == 0 || value == "" {
		return
	}
	layout := createTextLayout(value, format, max(w, 1), max(h, 1))
	if layout == 0 {
		return
	}
	defer win32UnknownRelease(layout)
	win32DWriteTextFormatSetTextAlignment(layout, dwriteTextAlignmentCenter)
	win32DWriteTextFormatSetParagraphAlignment(layout, dwriteParagraphAlignmentCenter)
	win32DWriteTextFormatSetWordWrapping(layout, dwriteWordWrappingNoWrap)
	// The line of a font has room above the glyphs for accents, so the glyphs
	// are centered by their ink instead of their line.
	var overhang dwriteOverhangMetrics
	var shift float32
	if win32DWriteTextLayoutGetOverhangMetrics(layout, &overhang).succeeded() {
		shift = (overhang.top - overhang.bottom) / 2
	}
	p.color(textColor)
	win32D2D1RenderTargetDrawTextLayout(p.target, d2d1Point2F{x: x, y: y + shift}, layout, p.brush, d2d1DrawTextOptionsNone)
}

// glyph draws one glyph of the symbol font centered in a box.
func (p *painter) glyph(value string, size, x, y, w, h float32, glyphColor themeColor) {
	font := fontSpec{family: symbolFamily(), size: size, weight: 400}
	p.text(value, font, x, y, w, h, glyphColor, 1, 1, false, false)
}

func (p *painter) bitmap(image comObject, x, y, w, h, opacity float32, nearest bool) {
	if image == 0 {
		return
	}
	mode := uint32(d2d1BitmapInterpolationModeLinear)
	if nearest {
		mode = d2d1BitmapInterpolationModeNearestNeighbor
	}
	win32D2D1RenderTargetDrawBitmap(p.target, image, &d2d1RectF{left: x, top: y, right: x + w, bottom: y + h}, opacity, mode, nil)
}

// control draws the fill and border of a standard control.
func (p *painter) control(x, y, w, h, radius float32, fill themeColor, pal *palette, accent bool) {
	p.fillRound(x, y, w, h, radius, fill)
	if !accent {
		p.strokeRound(x, y, w, h, radius, pal.controlStroke, 1)
	}
}

// focusRect draws the focus rectangle around a box.
func (p *painter) focusRect(x, y, w, h, radius float32, pal *palette) {
	p.strokeRound(x-3, y-3, w+6, h+6, radius+3, pal.focus, 2)
}

func (p *painter) clip(x, y, w, h float32) {
	win32D2D1RenderTargetPushAxisAlignedClip(p.target, &d2d1RectF{left: x, top: y, right: x + w, bottom: y + h},
		d2d1AntialiasModeAliased)
}

func (p *painter) unclip() {
	win32D2D1RenderTargetPopAxisAlignedClip(p.target)
}

// createTextLayout returns a DirectWrite layout of text, or 0.
func createTextLayout(text string, format comObject, width, height float32) comObject {
	var layout comObject
	if win32DWriteFactoryCreateTextLayout(dwriteFactory(), text, format, width, height, &layout).failed() {
		return 0
	}
	return layout
}

// measureKey names a measurement of text: the layout code measures the same
// text in the same font many times, so measurements are kept.
type measureKey struct {
	text  string
	font  fontSpec
	width float32
	flags uint32
	lines int
}

// measureCache keeps measurements. It forgets everything when it grows large,
// which is simpler than tracking use and rare in practice.
type measureCache[T any] map[measureKey]T

func (c *measureCache[T]) store(key measureKey, value T) {
	if len(*c) > 20000 {
		*c = measureCache[T]{}
	}
	(*c)[key] = value
}

var textMeasures = measureCache[d2d1SizeF]{}

// measureText measures text in logical units. width is the width at which it
// wraps, or 0 for no wrapping.
func measureText(text string, font fontSpec, width float32, maximumLines int) d2d1SizeF {
	key := measureKey{text: text, font: font, width: width, lines: maximumLines}
	if size, ok := textMeasures[key]; ok {
		return size
	}
	size := measureTextUncached(text, font, width, maximumLines)
	textMeasures.store(key, size)
	return size
}

func measureTextUncached(value string, font fontSpec, width float32, maximumLines int) d2d1SizeF {
	format := textFormat(font)
	if format == 0 {
		return d2d1SizeF{}
	}
	text := value
	if text == "" {
		text = " "
	}
	layoutWidth := width
	if layoutWidth <= 0 {
		layoutWidth = 100000
	}
	layout := createTextLayout(text, format, layoutWidth, 100000)
	if layout == 0 {
		return d2d1SizeF{}
	}
	defer win32UnknownRelease(layout)
	if width > 0 {
		win32DWriteTextFormatSetWordWrapping(layout, dwriteWordWrappingWrap)
	} else {
		win32DWriteTextFormatSetWordWrapping(layout, dwriteWordWrappingNoWrap)
	}
	var metrics dwriteTextMetrics
	win32DWriteTextLayoutGetMetrics(layout, &metrics)
	height := metrics.height
	if maximumLines > 0 && metrics.lineCount > uint32(maximumLines) {
		height = metrics.height * float32(maximumLines) / float32(metrics.lineCount)
	}
	if value == "" {
		return d2d1SizeF{height: height}
	}
	return d2d1SizeF{width: metrics.widthIncludingTrailingWhitespace, height: height}
}

// bitmapFromRGBA creates a Direct2D bitmap of RGBA pixels for a render target.
func bitmapFromRGBA(target comObject, pixels []byte, width, height, stride int) comObject {
	if target == 0 || len(pixels) == 0 || width <= 0 || height <= 0 {
		return 0
	}
	bgra := make([]byte, width*height*4)
	for y := range height {
		row := pixels[y*stride:]
		for x := range width {
			p := row[x*4:]
			q := bgra[(y*width+x)*4:]
			a := uint32(p[3])
			q[0] = byte(uint32(p[2]) * a / 255)
			q[1] = byte(uint32(p[1]) * a / 255)
			q[2] = byte(uint32(p[0]) * a / 255)
			q[3] = byte(a)
		}
	}
	properties := d2d1BitmapProperties{
		pixelFormat: d2d1PixelFormat{format: dxgiFormatB8G8R8A8Unorm, alphaMode: d2d1AlphaModePremultiplied},
	}
	var bitmap comObject
	win32D2D1RenderTargetCreateBitmap(target, uint32(width), uint32(height), unsafe.Pointer(&bgra[0]), uint32(width*4),
		&properties, &bitmap)
	return bitmap
}

var dcRenderTarget comObject

// renderTarget returns the render target that paints on device contexts.
// reset recreates it after Direct2D loses it.
func renderTarget(reset bool) comObject {
	if reset && dcRenderTarget != 0 {
		win32UnknownRelease(dcRenderTarget)
		dcRenderTarget = 0
	}
	if dcRenderTarget == 0 && d2dFactory() != 0 {
		properties := d2d1RenderTargetProperties{
			typ:         d2d1RenderTargetTypeDefault,
			pixelFormat: d2d1PixelFormat{format: dxgiFormatB8G8R8A8Unorm, alphaMode: d2d1AlphaModePremultiplied},
			usage:       d2d1RenderTargetUsageGDICompatible,
		}
		win32D2D1FactoryCreateDCRenderTarget(d2dFactory(), &properties, &dcRenderTarget)
	}
	return dcRenderTarget
}

// paintView paints a drawn view during WM_PAINT.
func paintView(view *nativeView, h hwnd) {
	var paint paintStruct
	dc := win32BeginPaint(h, &paint)
	defer win32EndPaint(h, &paint)
	var client rect
	win32GetClientRect(h, &client)
	if view != nil && view.ext != nil {
		var buffer hdc
		painting := win32BeginBufferedPaint(dc, &client, bpbfCompatibleBitmap, nil, &buffer)
		target := dc
		if painting != 0 {
			target = buffer
		}
		fillRectColor(target, &client, backgroundOf(h))
		painted := view.ext.paintGDI(view, h, target, &client)
		if painting != 0 {
			win32EndBufferedPaint(painting, painted)
		}
		if painted {
			return
		}
	}
	// A view inside a native control, such as the spinner of a busy button,
	// draws over the face of the control.
	overNative := false
	if parent := win32GetParent(h); parent != 0 {
		name := className(parent)
		overNative = len(name) < 7 || name[:7] != "GameKit"
	}
	// The view is drawn off the screen and then shown at once, so that an
	// animation, such as a spinner over a button, does not flicker.
	var buffer hdc
	var painting uintptr
	if client.right > 0 && client.bottom > 0 {
		painting = win32BeginBufferedPaint(dc, &client, bpbfCompatibleBitmap, nil, &buffer)
	}
	surface := dc
	if painting != 0 {
		surface = buffer
	}
	if overNative {
		win32DrawThemeParentBackground(h, surface, &client)
	}
	target := renderTarget(false)
	if target != 0 && client.right > 0 && client.bottom > 0 {
		dpi := float32(win32GetDpiForWindow(h))
		win32D2D1RenderTargetSetDpi(target, dpi, dpi)
		if win32D2D1DCRenderTargetBindDC(target, surface, &client).succeeded() {
			canvas := painter{target: target, scale: dpi / 96}
			canvas.width = float32(client.right) / canvas.scale
			canvas.height = float32(client.bottom) / canvas.scale
			black := d2d1ColorF{a: 1}
			win32D2D1RenderTargetCreateSolidColorBrush(target, &black, nil, &canvas.brush)
			win32D2D1RenderTargetBeginDraw(target)
			if overNative {
				win32D2D1RenderTargetSetTextAntialiasMode(target, d2d1TextAntialiasModeGrayscale)
			} else {
				win32D2D1RenderTargetSetTextAntialiasMode(target, d2d1TextAntialiasModeCleartype)
				background := backgroundOf(h).d2d()
				win32D2D1RenderTargetClear(target, &background)
			}
			if view != nil && view.ext != nil {
				view.ext.paint(view, &canvas)
			}
			if win32D2D1RenderTargetEndDraw(target, nil, nil) == d2dErrRecreateTarget {
				renderTarget(true)
			}
			win32UnknownRelease(canvas.brush)
		}
	}
	if painting != 0 {
		win32EndBufferedPaint(painting, true)
	}
}

// paintCanvas draws with Direct2D on a device context of a window, over the
// background of the window.
func paintCanvas(h hwnd, dc hdc, area *rect, draw func(*painter)) bool {
	target := renderTarget(false)
	if target == 0 || area.right <= area.left || area.bottom <= area.top {
		return false
	}
	dpi := float32(win32GetDpiForWindow(h))
	win32D2D1RenderTargetSetDpi(target, dpi, dpi)
	if win32D2D1DCRenderTargetBindDC(target, dc, area).failed() {
		return false
	}
	canvas := painter{target: target, scale: dpi / 96}
	canvas.width = float32(area.right-area.left) / canvas.scale
	canvas.height = float32(area.bottom-area.top) / canvas.scale
	black := d2d1ColorF{a: 1}
	win32D2D1RenderTargetCreateSolidColorBrush(target, &black, nil, &canvas.brush)
	win32D2D1RenderTargetBeginDraw(target)
	win32D2D1RenderTargetSetTextAntialiasMode(target, d2d1TextAntialiasModeCleartype)
	background := backgroundOf(h).d2d()
	win32D2D1RenderTargetClear(target, &background)
	draw(&canvas)
	result := win32D2D1RenderTargetEndDraw(target, nil, nil)
	if result == d2dErrRecreateTarget {
		renderTarget(true)
	}
	win32UnknownRelease(canvas.brush)
	return result.succeeded()
}

// fillRectColor fills a rectangle of a device context with a color.
func fillRectColor(dc hdc, area *rect, color themeColor) {
	brush := win32CreateSolidBrush(color.colorRef())
	win32FillRect(dc, area, brush)
	win32DeleteObject(hgdiobj(brush))
}

// ---- Images ----
//
// The Windows Imaging Component encodes and decodes images.

var wicFactoryObject comObject

func wicFactory() comObject {
	if wicFactoryObject == 0 {
		win32CoCreateInstance(clsidWICImagingFactory, 0, clsctxInprocServer, iidWICImagingFactory, &wicFactoryObject)
	}
	return wicFactoryObject
}

// encodePNG encodes RGBA pixels as PNG.
func encodePNG(pixels []byte, width, height int) []byte {
	factory := wicFactory()
	if factory == 0 || width <= 0 || height <= 0 || len(pixels) < width*height*4 {
		return nil
	}
	var stream comObject
	if win32CreateStreamOnHGlobal(0, true, &stream).failed() {
		return nil
	}
	defer win32UnknownRelease(stream)
	var encoder comObject
	if win32WICImagingFactoryCreateEncoder(factory, guidContainerFormatPng, nil, &encoder).failed() {
		return nil
	}
	defer win32UnknownRelease(encoder)
	if win32WICBitmapEncoderInitialize(encoder, stream, wicBitmapEncoderNoCache).failed() {
		return nil
	}
	var frame comObject
	if win32WICBitmapEncoderCreateNewFrame(encoder, &frame, nil).failed() {
		return nil
	}
	defer win32UnknownRelease(frame)
	if win32WICBitmapFrameEncodeInitialize(frame, 0).failed() ||
		win32WICBitmapFrameEncodeSetSize(frame, uint32(width), uint32(height)).failed() {
		return nil
	}
	// The encoder can change the pixel format to a format that it supports.
	// For BGRA, the red and blue bytes change places.
	format := *guidWICPixelFormat32bppRGBA
	win32WICBitmapFrameEncodeSetPixelFormat(frame, &format)
	data := append([]byte(nil), pixels[:width*height*4]...)
	if format == *guidWICPixelFormat32bppBGRA {
		for i := 0; i < len(data); i += 4 {
			data[i], data[i+2] = data[i+2], data[i]
		}
	}
	if win32WICBitmapFrameEncodeWritePixels(frame, uint32(height), uint32(width*4), data).failed() ||
		win32WICBitmapFrameEncodeCommit(frame).failed() || win32WICBitmapEncoderCommit(encoder).failed() {
		return nil
	}
	var memory hglobal
	win32GetHGlobalFromStream(stream, &memory)
	var stat statStg
	win32StreamStat(stream, &stat, statflagNoName)
	bytes := win32GlobalLock(memory)
	defer win32GlobalUnlock(memory)
	return goBytesAt(bytes, int(stat.cbSize))
}

// decodeImage decodes an image in a format of Windows to RGBA pixels.
func decodeImage(data []byte) (pixels []byte, width, height int, ok bool) {
	factory := wicFactory()
	if factory == 0 || len(data) == 0 {
		return nil, 0, 0, false
	}
	// The stream reads the data in place. The data stays alive until the
	// stream is released, which is before this deferred call.
	defer runtime.KeepAlive(data)
	var stream comObject
	if win32WICImagingFactoryCreateStream(factory, &stream).failed() {
		return nil, 0, 0, false
	}
	defer win32UnknownRelease(stream)
	if win32WICStreamInitializeFromMemory(stream, data).failed() {
		return nil, 0, 0, false
	}
	var decoder comObject
	if win32WICImagingFactoryCreateDecoderFromStream(factory, stream, nil, wicDecodeMetadataCacheOnDemand,
		&decoder).failed() {
		return nil, 0, 0, false
	}
	defer win32UnknownRelease(decoder)
	var frame comObject
	if win32WICBitmapDecoderGetFrame(decoder, 0, &frame).failed() {
		return nil, 0, 0, false
	}
	defer win32UnknownRelease(frame)
	var converter comObject
	if win32WICImagingFactoryCreateFormatConverter(factory, &converter).failed() {
		return nil, 0, 0, false
	}
	defer win32UnknownRelease(converter)
	if win32WICFormatConverterInitialize(converter, frame, guidWICPixelFormat32bppRGBA, wicBitmapDitherTypeNone, 0, 0,
		wicBitmapPaletteTypeCustom).failed() {
		return nil, 0, 0, false
	}
	var w, h uint32
	if win32WICBitmapSourceGetSize(converter, &w, &h).failed() || w == 0 || h == 0 {
		return nil, 0, 0, false
	}
	pixels = make([]byte, int(w)*int(h)*4)
	if win32WICBitmapSourceCopyPixels(converter, nil, w*4, pixels).failed() {
		return nil, 0, 0, false
	}
	return pixels, int(w), int(h), true
}

// ---- Views ----

const (
	kindGeneric = iota
	kindLayout
	kindLabel
	kindButton
	kindTextField
	kindTextArea
	kindCheckBox
	kindRadioGroup
	kindSelect
	kindSlider
	kindProgress
	kindActivity
	kindSeparator
	kindImage
	kindLink
	kindTabs
	kindScroll
	kindCanvas
	kindSection
	kindToggle
	kindSwitch
	kindPassword
	kindSearch
	kindNumber
	kindCombo
	kindSegmented
	kindStepper
	kindDial
	kindRange
	kindDate
	kindCalendar
	kindColor
	kindFont
	kindKeyRecorder
	kindClip
	kindGroupBox
	kindSplit
	kindTable
	kindOutline
	kindCollection
	kindBreadcrumbs
	kindPagination
	kindStatusBar
	kindCommandBar
	kindInfoBar
	kindBadge
	kindEmptyState
	kindValidation
	kindIcon
	kindDocument
	kindVector
	kindAnimation
	kindVideo
	kindWeb
	kindToolbar
)

// viewStyle is the style that the Go layer sets on a view.
type viewStyle struct {
	family                       string
	size                         float64
	weight                       int
	italic                       bool
	hasForeground, hasBackground bool
	foreground, background       themeColor
	// foregroundRole and backgroundRole are the ui.SemanticColor of the
	// foreground and the background, which the colors follow when the theme
	// changes, or 0.
	foregroundRole, backgroundRole int
	cornerRadius                   float64
}

// extension holds the state and the behavior of a kind of view. A view that the
// backend draws has an extension that paints it and handles its input. Embed
// baseExtension for the methods an extension does not need.
type extension interface {
	// paintGDI draws the view with GDI and the visual styles of Windows. It
	// returns false to draw with Direct2D through paint.
	paintGDI(view *nativeView, h hwnd, dc hdc, client *rect) bool
	// paint draws the view.
	paint(view *nativeView, canvas *painter)
	// message handles a message of the view's window. It returns true when
	// it handled the message, with the result in result.
	message(view *nativeView, h hwnd, message uint32, wparam, lparam uintptr, result *uintptr) bool
	// command handles a WM_COMMAND notification from the view's control.
	command(view *nativeView, from hwnd, code int) bool
	// notify handles a WM_NOTIFY notification from the view's control.
	notify(view *nativeView, header *nmhdr, result *uintptr) bool
	// scroll handles WM_HSCROLL or WM_VSCROLL from a trackbar or an up-down
	// control of the view.
	scroll(view *nativeView, from hwnd, code int) bool
	// preferred returns the preferred size of the view in logical units, and
	// false to use the default.
	preferred(view *nativeView) (d2d1SizeF, bool)
	// layout positions the windows inside the view after it changes size.
	layout(view *nativeView)
	// themeChanged updates the view for a new theme.
	themeChanged(view *nativeView)
	// free releases what the extension holds when its view goes away.
	free()
}

// baseExtension implements extension with methods that do nothing.
type baseExtension struct{}

func (baseExtension) paintGDI(*nativeView, hwnd, hdc, *rect) bool {
	return false
}

func (baseExtension) paint(*nativeView, *painter) {
}

func (baseExtension) message(*nativeView, hwnd, uint32, uintptr, uintptr, *uintptr) bool {
	return false
}

func (baseExtension) command(*nativeView, hwnd, int) bool {
	return false
}

func (baseExtension) notify(*nativeView, *nmhdr, *uintptr) bool {
	return false
}

func (baseExtension) scroll(*nativeView, hwnd, int) bool {
	return false
}

func (baseExtension) preferred(*nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{}, false
}

func (baseExtension) layout(*nativeView) {
}

func (baseExtension) themeChanged(*nativeView) {
}

func (baseExtension) free() {
}

// nativeView is a view: a native control or a window that the backend draws.
type nativeView struct {
	receiver eventReceiver
	id       uintptr
	// outer is the window that the parent layout positions.
	outer hwnd
	// control is the window that takes focus and input, if not outer.
	control hwnd
	// body is the layout that holds the children of a container.
	body   hwnd
	window *nativeWindow
	// owner is the view that a part of another view, such as the header of a
	// section, belongs to; focus and accessibility report the owner.
	owner    *nativeView
	kind     int
	suppress int
	// The frame in logical units, relative to the parent layout.
	x, y, width, height int
	// placed is set once the Go layer has given the view a frame.
	placed  bool
	visible bool
	enabled bool
	hover   bool
	pressed bool
	focused bool
	// keyboardFocus is focus that the keyboard moved, which shows a focus
	// rectangle.
	keyboardFocus         bool
	text                  string
	placeholder           string
	url                   string
	accessibleID          string
	accessibleName        string
	accessibleDescription string
	accessibleValue       string
	accessibleHidden      bool
	accessibleRequired    bool
	accessibleInvalid     bool
	tooltip               string
	cursor                int
	checked               bool
	selected              int
	alignment             int
	readOnly              bool
	running               bool
	value                 float64
	minimum, maximum      float64
	step                  float64
	axes                  int
	canvasButtons         int
	frames                bool
	preferredWidth        int
	scaling               int
	nearest               bool
	sourceScale           float64
	vertical              bool
	inverted              bool
	animated              bool
	style                 viewStyle
	// font is the GDI font of the view's native control at the current DPI.
	// The font cache owns it.
	font hfont
	// background is a brush of the color behind the view, for the
	// WM_CTLCOLOR messages of its control.
	background hbrush
	ext        extension
	// contextMenu is the menu that the view shows on a secondary click.
	contextMenu *nativeMenu
}

var (
	views      = map[uintptr]*nativeView{}
	nextViewID uintptr
)

// The user data of a window holds the identifier of its view. Every message
// reads it, and unlike a window property it does not go into the kernel.

// setViewOf makes a window belong to a view.
func setViewOf(h hwnd, view *nativeView) {
	win32SetWindowLongPtr(h, gwlpUserData, view.id)
}

// clearViewOf makes a window belong to no view.
func clearViewOf(h hwnd) {
	win32SetWindowLongPtr(h, gwlpUserData, 0)
}

// viewOf returns the live view of a window, without its ancestors.
func viewOf(h hwnd) *nativeView {
	if h == 0 {
		return nil
	}
	return views[win32GetWindowLongPtr(h, gwlpUserData)]
}

// live reports whether the view has not been destroyed.
func (v *nativeView) live() bool {
	return v != nil && views[v.id] == v
}

// destroy frees the view and its windows. The views inside it stay; see below.
func (v *nativeView) destroy() {
	if !v.live() {
		return
	}
	releaseCapture(v)
	if section := sectionOf(v); section != nil && section.headerView != nil {
		section.headerView.destroy()
		section.headerView = nil
	}
	delete(views, v.id)
	forgetDragSource(v)
	forgetDropTarget(v)
	forgetLabel(v)
	forgetAccelerator(v)
	forgetButton(v)
	forgetTextArea(v)
	forgetCheckBox(v)
	forgetProgress(v)
	v.contextMenu = nil
	if v.background != 0 {
		win32DeleteObject(hgdiobj(v.background))
		v.background = 0
	}
	for _, h := range []hwnd{v.outer, v.control, v.body} {
		if h != 0 && win32IsWindow(h) && win32GetWindowLongPtr(h, gwlpUserData) == v.id {
			clearViewOf(h)
		}
	}
	if v.ext != nil {
		v.ext.free()
		v.ext = nil
	}
	// The views inside a container belong to the Go layer, which closes them
	// itself; they move to the parking window so that destroying the
	// container does not destroy their windows.
	for _, container := range []hwnd{v.body, v.outer} {
		if container == 0 || !win32IsWindow(container) {
			continue
		}
		for child := win32GetWindow(container, gwChild); child != 0; {
			next := win32GetWindow(child, gwHwndNext)
			if inner := viewOf(child); inner != nil && inner != v {
				win32ShowWindow(child, swHide)
				win32SetParent(child, parking())
			}
			child = next
		}
	}
	if v.outer != 0 && win32IsWindow(v.outer) {
		win32DestroyWindow(v.outer)
	}
}

// viewFromDescendant returns the view of a window or its nearest ancestor.
func viewFromDescendant(h hwnd) *nativeView {
	for current := h; current != 0; current = win32GetParent(current) {
		if view := viewOf(current); view != nil {
			if view.owner.live() {
				return view.owner
			}
			return view
		}
		if win32GetWindowLongPtr(current, gwlStyle)&wsChild == 0 {
			break
		}
	}
	return nil
}

// newView creates a view for a window and registers it.
func newView(outer, control hwnd, kind int, window *nativeWindow) *nativeView {
	nextViewID++
	view := &nativeView{
		id:          nextViewID,
		outer:       outer,
		kind:        kind,
		window:      window,
		visible:     true,
		enabled:     true,
		selected:    -1,
		maximum:     1,
		sourceScale: 1,
		animated:    true,
	}
	if control != outer {
		view.control = control
	}
	views[view.id] = view
	// A new view is in its window, at the origin of the content and with no
	// size, until the Go layer gives it a parent and a frame.
	if outer != 0 && view.window != nil && view.window.root != 0 && win32GetParent(outer) == parking() {
		win32SetParent(outer, view.window.root)
	}
	if outer != 0 {
		setViewOf(outer, view)
	}
	if view.control != 0 {
		setViewOf(view.control, view)
	}
	return view
}

// target returns the window of a view that takes focus and input.
func (v *nativeView) target() hwnd {
	if v == nil {
		return 0
	}
	if v.control != 0 {
		return v.control
	}
	return v.outer
}

// theme returns the theme of the window that holds a view.
func (v *nativeView) theme() int {
	if v == nil {
		return resolvedTheme()
	}
	var win *nativeWindow
	if v.outer != 0 {
		win = windowOf(v.outer)
	}
	if win == nil {
		win = v.window
	}
	return win.themeOf()
}

func (v *nativeView) palette() *palette {
	return paletteFor(v.theme())
}

// scale returns the physical pixels in one logical unit for a view.
func (v *nativeView) scale() float64 {
	if v == nil || v.outer == 0 {
		return 1
	}
	return scaleOf(v.outer)
}

// fontOr returns the font of a view: its style over a default.
func (v *nativeView) fontOr(fallback fontSpec) fontSpec {
	font := fallback
	if v != nil {
		if v.style.family != "" {
			font.family = v.style.family
		}
		if v.style.size > 0 {
			font.size = float32(v.style.size)
		}
		if v.style.weight > 0 {
			font.weight = v.style.weight
		}
		if v.style.italic {
			font.italic = true
		}
	}
	return font
}

// textColorOr returns the text color of a view: its style over a default.
func (v *nativeView) textColorOr(fallback themeColor) themeColor {
	if v != nil && v.style.hasForeground {
		return v.style.foreground
	}
	return fallback
}

// invalidate redraws a view.
func (v *nativeView) invalidate() {
	if v == nil {
		return
	}
	for _, h := range []hwnd{v.outer, v.control} {
		if h != 0 {
			win32InvalidateRect(h, nil, true)
		}
	}
}

// applyFont gives the native controls of a view the view's font at the DPI of
// its window.
func (v *nativeView) applyFont() {
	target := v.target()
	if target == 0 {
		return
	}
	dpi := win32GetDpiForWindow(target)
	if dpi == 0 {
		dpi = 96
	}
	v.font = fontHandle(v.fontOr(bodyFont()), dpi)
	win32SendMessage(target, wmSetFont, uintptr(v.font), 1)
}

var ctlColorBrushes = map[uint32]hbrush{}

// textRoleProperty holds the text color of a part of a component: a
// ui.SemanticColor, or textRoleInverted for text on an inverted background.
const (
	textRoleProperty = "GKTextRole"
	textRoleInverted = 10
)

// setTextRole gives a native label the text color of a ui.SemanticColor, or
// textRoleInverted.
func setTextRole(control hwnd, role int) {
	win32SetProp(control, textRoleProperty, uintptr(role))
}

// ctlColor answers a WM_CTLCOLOR message for a control.
func ctlColor(control hwnd, dc hdc, message uint32) uintptr {
	view := viewFromDescendant(control)
	theme := resolvedTheme()
	if view != nil {
		theme = view.theme()
	}
	pal := paletteFor(theme)
	back := backgroundOf(control)
	text := pal.text
	if message == wmCtlColorEdit || message == wmCtlColorListBox {
		back = rgbColor(0xffffff)
		if pal.dark {
			back = pal.inputActive
		}
	}
	if message == wmCtlColorStatic && win32GetWindowLongPtr(control, gwlStyle)&esReadOnly != 0 && className(control) == wcEdit {
		back = rgbColor(0xf9f9f9)
		if pal.dark {
			back = pal.inputActive
		}
	}
	if view != nil && view.style.hasBackground {
		back = mix(back, view.style.background)
	}
	if view != nil {
		fallback := text
		if !view.enabled {
			fallback = pal.textDisabled
		}
		text = view.textColorOr(fallback)
	}
	// A part of a component can have a text color of its own.
	if role := int(win32GetProp(control, textRoleProperty)); role != 0 {
		if role == textRoleInverted {
			text = rgbColor(0xffffff)
			if pal.dark {
				text = rgbColor(0x000000)
			}
		} else if color, ok := semanticColor(pal, role); ok {
			text = color
		}
	}
	win32SetTextColor(dc, text.colorRef())
	win32SetBkColor(dc, back.colorRef())
	win32SetBkMode(dc, transparent)
	if view != nil {
		if view.background != 0 {
			win32DeleteObject(hgdiobj(view.background))
		}
		view.background = win32CreateSolidBrush(back.colorRef())
		return uintptr(view.background)
	}
	key := back.colorRef()
	brush, ok := ctlColorBrushes[key]
	if !ok {
		brush = win32CreateSolidBrush(key)
		ctlColorBrushes[key] = brush
	}
	return uintptr(brush)
}

// report sends an event of the user to the receiver of the view. It sends
// nothing while the view applies a change of the Go layer.
func (v *nativeView) report(event any) {
	if v != nil && v.suppress == 0 {
		send(v.receiver, event)
	}
}

// ---- Subclassing ----

func subclassProc(h hwnd, message uint32, wparam, lparam, id, data uintptr) uintptr {
	view := viewOf(h)
	if view != nil && view.ext != nil {
		var result uintptr
		if view.ext.message(view, h, message, wparam, lparam, &result) {
			return result
		}
	}
	if message == wmNCDestroy {
		win32RemoveWindowSubclass(h, subclassCallback(), 1)
	}
	return win32DefSubclassProc(h, message, wparam, lparam)
}

func subclassCallback() uintptr {
	return callback("subclass", func(h hwnd, message uint32, wparam, lparam, id, data uintptr) uintptr {
		return subclassProc(h, message, wparam, lparam, id, data)
	})
}

// subclass routes messages of a native control to its view's extension and
// reports its focus, so the window input and the focus of the view work.
func subclass(h hwnd) {
	if h != 0 && procSetWindowSubclass.find() != 0 {
		win32SetWindowSubclass(h, subclassCallback(), 1, 0)
	}
}

// ---- Layout ----
//
// A layout is a window that holds child windows at the frames that the Go
// layout code sets, in logical units. It paints the background that its
// children draw over: the color of the window, or the color that a container
// gives it.

type frameRecord struct {
	x, y, width, height float64
}

type layoutData struct {
	hasBackground    bool
	background       themeColor
	originX, originY float64
	zoom             float64
	onResize         func(width, height float64)
}

const (
	layoutClass   = "GameKitLayout"
	controlClass  = "GameKitControl"
	messagesClass = "GameKitMessages"
)

var (
	layouts      = map[hwnd]*layoutData{}
	frames       = map[hwnd]*frameRecord{}
	frameInserts int
)

func layoutDataOf(h hwnd) *layoutData {
	if h == 0 {
		return nil
	}
	return layouts[h]
}

func frameOf(h hwnd) *frameRecord {
	return frames[h]
}

// frameFor returns the frame record of a window, creating it. A window's
// record goes away with the window.
func frameFor(h hwnd) *frameRecord {
	frameInserts++
	if frameInserts%256 == 0 {
		for window := range frames {
			if !win32IsWindow(window) {
				delete(frames, window)
			}
		}
	}
	frame := frames[h]
	if frame == nil {
		frame = &frameRecord{}
		frames[h] = frame
	}
	return frame
}

// scaleOf returns the physical pixels in one logical unit for a window.
func scaleOf(h hwnd) float64 {
	var dpi uint32
	if h != 0 {
		dpi = win32GetDpiForWindow(h)
	}
	if dpi == 0 {
		return 1
	}
	return float64(dpi) / 96
}

// clientSize returns the client size of a window in logical units.
func clientSize(h hwnd) d2d1SizeF {
	var client rect
	win32GetClientRect(h, &client)
	scale := scaleOf(h)
	return d2d1SizeF{width: float32(float64(client.right) / scale), height: float32(float64(client.bottom) / scale)}
}

// backgroundOf returns the color behind a window: the background of the
// nearest layout or container that sets one, or the window color.
func backgroundOf(h hwnd) themeColor {
	for current := h; current != 0; current = win32GetParent(current) {
		if data := layoutDataOf(current); data != nil && data.hasBackground {
			return data.background
		}
		if win32GetWindowLongPtr(current, gwlStyle)&wsChild == 0 {
			win := windowOf(current)
			if win == nil {
				return paletteFor(resolvedTheme()).window
			}
			return paletteFor(win.themeOf()).window
		}
	}
	return paletteFor(resolvedTheme()).window
}

// While a layout pass runs, windows move without drawing, and each window that
// holds them draws once at the end of the pass.
var batchDepth int

// batchMove is a window that moved during a pass, with the area that it left
// in its parent.
type batchMove struct {
	window hwnd
	parent hwnd
	before rect
}

var batchMoves []batchMove

func beginBatch() {
	batchDepth++
}

func endBatch() {
	if batchDepth == 0 {
		return
	}
	batchDepth--
	if batchDepth > 0 {
		return
	}
	moves := batchMoves
	batchMoves = nil
	for _, move := range moves {
		if win32IsWindow(move.parent) {
			win32InvalidateRect(move.parent, &move.before, true)
		}
		if win32IsWindow(move.window) {
			win32RedrawWindow(move.window, nil, 0, rdwInvalidate|rdwErase|rdwAllChildren|rdwFrame)
		}
	}
}

func placeInParent(h hwnd, frame frameRecord) {
	parent := win32GetParent(h)
	data := layoutDataOf(parent)
	zoom, ox, oy := 1.0, 0.0, 0.0
	if data != nil {
		zoom, ox, oy = data.zoom, data.originX, data.originY
	}
	scaleWindow := parent
	if scaleWindow == 0 {
		scaleWindow = h
	}
	scale := scaleOf(scaleWindow)
	x := int32(math.Round((frame.x*zoom - ox) * scale))
	y := int32(math.Round((frame.y*zoom - oy) * scale))
	w := int32(math.Round(frame.width * zoom * scale))
	ht := int32(math.Round(frame.height * zoom * scale))
	flags := uint32(swpNoZOrder | swpNoActivate | swpNoOwnerZOrder)
	if batchDepth > 0 && parent != 0 && win32IsWindowVisible(h) {
		flags |= swpNoRedraw
		var before rect
		win32GetWindowRect(h, &before)
		mapWindowRect(0, parent, &before)
		found := false
		for _, move := range batchMoves {
			if move.window == h {
				found = true
				break
			}
		}
		if !found {
			batchMoves = append(batchMoves, batchMove{window: h, parent: parent, before: before})
		}
	}
	win32SetWindowPos(h, 0, x, y, max(w, 0), max(ht, 0), flags)
}

// placeWindow moves a window to a frame in logical units, relative to a layout.
func placeWindow(h hwnd, x, y, width, height float64) {
	if h == 0 {
		return
	}
	frame := frameFor(h)
	frame.x = x
	frame.y = y
	frame.width = max(width, 0)
	frame.height = max(height, 0)
	placeInParent(h, *frame)
}

// place moves a view's window to its frame in its parent.
func (v *nativeView) place() {
	if v != nil && v.outer != 0 {
		placeWindow(v.outer, float64(v.x), float64(v.y), float64(v.width), float64(v.height))
	}
}

// replaceChildren moves the children of a layout to their frames after its
// origin, zoom, or DPI changes.
func replaceViewChildren(layout hwnd) {
	for child := win32GetWindow(layout, gwChild); child != 0; child = win32GetWindow(child, gwHwndNext) {
		if frame := frameOf(child); frame != nil {
			placeInParent(child, *frame)
		}
	}
}

// setLayoutZoom scales the frames of a layout's children.
func setLayoutZoom(layout hwnd, zoom float64) {
	data := layoutDataOf(layout)
	if data == nil {
		return
	}
	if zoom <= 0 {
		zoom = 1
	}
	if data.zoom == zoom {
		return
	}
	data.zoom = zoom
	replaceViewChildren(layout)
	win32InvalidateRect(layout, nil, true)
}

// setLayoutBackground sets the background color of a layout, or removes it
// with nil so that the layout shows the background of its parent.
func setLayoutBackground(layout hwnd, color *themeColor) {
	if data := layoutDataOf(layout); data != nil {
		data.hasBackground = color != nil
		if color != nil {
			data.background = *color
		}
		win32InvalidateRect(layout, nil, true)
	}
}

// layoutOnResize sets a function that runs when the layout changes size, with
// its size in logical units.
func layoutOnResize(layout hwnd, handler func(width, height float64)) {
	if data := layoutDataOf(layout); data != nil {
		data.onResize = handler
	}
}

// reparent moves a window into a new parent.
func reparent(child, parent hwnd) {
	if child == 0 || parent == 0 || win32GetParent(child) == parent {
		return
	}
	win32SetParent(child, parent)
	if frame := frameOf(child); frame != nil {
		placeInParent(child, *frame)
	}
}

// forwardNotification routes a notification that a control sends its parent
// to the view of the control.
func forwardNotification(message uint32, wparam, lparam uintptr, result *uintptr) bool {
	switch message {
	case wmCommand:
		from := hwnd(lparam)
		var view *nativeView
		if from != 0 {
			view = viewFromDescendant(from)
		}
		if view != nil && view.ext != nil && view.ext.command(view, from, int(hiword(wparam))) {
			*result = 0
			return true
		}
		return false
	case wmNotify:
		header := (*nmhdr)(memoryAt(lparam))
		if header == nil {
			return false
		}
		view := viewFromDescendant(header.hwndFrom)
		if view != nil && view.ext != nil {
			var value uintptr
			if view.ext.notify(view, header, &value) {
				*result = value
				return true
			}
		}
		return false
	case wmHScroll, wmVScroll:
		from := hwnd(lparam)
		var view *nativeView
		if from != 0 {
			view = viewFromDescendant(from)
		}
		if view != nil && view.ext != nil && view.ext.scroll(view, from, int(loword(wparam))) {
			*result = 0
			return true
		}
		return false
	case wmCtlColorStatic, wmCtlColorBtn, wmCtlColorEdit, wmCtlColorListBox:
		*result = ctlColor(hwnd(lparam), hdc(wparam), message)
		return true
	case wmDrawItem:
		item := (*drawItemStruct)(memoryAt(lparam))
		if item == nil {
			return false
		}
		view := viewFromDescendant(item.hwndItem)
		if view != nil && view.ext != nil {
			var value uintptr
			if view.ext.message(view, item.hwndItem, wmDrawItem, wparam, lparam, &value) {
				*result = 1
				return true
			}
		}
		return false
	default:
		return false
	}
}

func fillBackground(h hwnd, dc hdc) {
	var client rect
	win32GetClientRect(h, &client)
	fillRectColor(dc, &client, backgroundOf(h))
}

func layoutProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	var result uintptr
	if forwardNotification(message, wparam, lparam, &result) {
		return result
	}
	if view := viewOf(h); view != nil && view.ext != nil && view.body != h {
		if view.ext.message(view, h, message, wparam, lparam, &result) {
			return result
		}
	}
	switch message {
	case wmNCCreate:
		layouts[h] = &layoutData{zoom: 1}
	case wmNCDestroy:
		delete(layouts, h)
	case wmEraseBkgnd:
		fillBackground(h, hdc(wparam))
		return 1
	case wmPrintClient:
		fillBackground(h, hdc(wparam))
		return 0
	case wmPaint:
		var paint paintStruct
		win32BeginPaint(h, &paint)
		win32EndPaint(h, &paint)
		return 0
	case wmSize:
		if data := layoutDataOf(h); data != nil && data.onResize != nil {
			scale := scaleOf(h)
			handler := data.onResize
			handler(float64(loword(lparam))/scale, float64(hiword(lparam))/scale)
		}
	case wmDpiChangedAfterParent:
		replaceViewChildren(h)
	}
	return win32DefWindowProc(h, message, wparam, lparam)
}

// newLayout creates a layout window in parent, or in the parking window.
func newLayout(parent hwnd) hwnd {
	defer deactivate(activate())
	if parent == 0 {
		parent = parking()
	}
	return win32CreateWindowEx(wsExControlParent, layoutClass, "", wsChild|wsVisible|wsClipChildren|wsClipSiblings,
		0, 0, 0, 0, parent, 0, instance(), nil)
}

// ---- Drawn controls ----

func controlProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	var result uintptr
	if forwardNotification(message, wparam, lparam, &result) {
		return result
	}
	view := viewOf(h)
	if view != nil && view.ext != nil && view.ext.message(view, h, message, wparam, lparam, &result) {
		return result
	}
	switch message {
	case wmEraseBkgnd:
		return 1
	case wmPrintClient:
		// A transparent child, such as a flat toolbar, draws over the face of
		// the view.
		var client rect
		win32GetClientRect(h, &client)
		fillBackground(h, hdc(wparam))
		if view != nil && view.ext != nil {
			view.ext.paintGDI(view, h, hdc(wparam), &client)
		}
		return 0
	case wmPaint:
		paintView(view, h)
		return 0
	case wmMouseMove:
		if view != nil && !view.hover {
			view.hover = true
			trackMouseLeave(h)
			win32InvalidateRect(h, nil, false)
		}
	case wmMouseLeave:
		if view != nil {
			view.hover = false
			view.pressed = false
			win32InvalidateRect(h, nil, false)
		}
	case wmLButtonDown:
		if view != nil && view.enabled && win32GetWindowLongPtr(h, gwlStyle)&wsTabStop != 0 {
			win32SetFocus(h)
		}
	case wmSetFocus, wmKillFocus:
		if view != nil {
			view.focused = message == wmSetFocus
			if !view.focused {
				view.keyboardFocus = false
			}
			win32InvalidateRect(h, nil, false)
		}
		return 0
	case wmGetDlgCode:
		return dlgcWantArrows
	}
	return win32DefWindowProc(h, message, wparam, lparam)
}

// trackMouseLeave asks Windows for WM_MOUSELEAVE when the pointer leaves a
// window.
func trackMouseLeave(h hwnd) {
	track := trackMouseEventInfo{dwFlags: tmeLeave, hwndTrack: h}
	track.cbSize = uint32(unsafe.Sizeof(track))
	win32TrackMouseEvent(&track)
}

// createDrawn creates a view of the backend's control class with ext, in the
// parking window until the Go layer gives it a parent.
func createDrawn(window *nativeWindow, kind int, ext extension, focusable bool) *nativeView {
	cookie := activate()
	style := uint32(wsChild | wsVisible | wsClipChildren | wsClipSiblings)
	if focusable {
		style |= wsTabStop
	}
	h := win32CreateWindowEx(0, controlClass, "", style, 0, 0, 0, 0, parking(), 0, instance(), nil)
	deactivate(cookie)
	view := newView(h, 0, kind, window)
	view.ext = ext
	return view
}

// adopt registers a native window as a view.
func adoptView(window *nativeWindow, outer, control hwnd, kind int) *nativeView {
	view := newView(outer, control, kind, window)
	if control != 0 && control != outer {
		subclass(control)
	} else if outer != 0 {
		if name := className(outer); name != layoutClass && name != controlClass {
			subclass(outer)
		}
	}
	view.applyFont()
	theme := view.theme()
	if control != 0 && control != outer {
		applyThemeTo(control, theme, "")
	} else {
		applyThemeTo(outer, theme, "")
	}
	return view
}

// container returns the layout that holds the children of a view.
func (v *nativeView) container() hwnd {
	if v == nil {
		return 0
	}
	if v.body != 0 {
		return v.body
	}
	return v.outer
}

type gdiSize struct {
	cx, cy int
}

var gdiMeasures = measureCache[gdiSize]{}

var measureDC hdc

// measureGDI measures text with the font of a native control, in logical
// units. width is the width at which the text wraps, or 0.
func measureGDI(text string, font fontSpec, width int, flags uint32) gdiSize {
	key := measureKey{text: text, font: font, width: float32(width), flags: flags}
	if size, ok := gdiMeasures[key]; ok {
		return size
	}
	// A memory device context measures text at 96 DPI without the cost of
	// getting the screen's.
	if measureDC == 0 {
		measureDC = win32CreateCompatibleDC(0)
	}
	previous := win32SelectObject(measureDC, hgdiobj(fontHandle(font, 96)))
	area := rect{right: 100000}
	if width > 0 {
		area.right = int32(width)
	}
	value := text
	if value == "" {
		value = " "
	}
	format := uint32(dtCalcRect | dtNoPrefix | flags)
	if width > 0 {
		format |= dtWordBreak
	} else {
		format |= dtSingleLine
	}
	win32DrawText(measureDC, value, &area, format)
	win32SelectObject(measureDC, previous)
	size := gdiSize{cx: int(area.right - area.left), cy: int(area.bottom - area.top)}
	if text == "" {
		size.cx = 0
	}
	gdiMeasures.store(key, size)
	return size
}

// makeNative creates a native control in parent, or in the parking window.
func makeNative(class, text string, style, exStyle uint32, parent hwnd) hwnd {
	defer deactivate(activate())
	if parent == 0 {
		parent = parking()
	}
	return win32CreateWindowEx(exStyle, class, text, wsChild|wsVisible|style, 0, 0, 0, 0, parent, 0, instance(), nil)
}

// replaceOuter replaces the window of a view with another window at the same
// place, for a change that a native control cannot make in place.
func (v *nativeView) replaceOuter(replacement hwnd) {
	old := v.outer
	if old == 0 || replacement == 0 {
		return
	}
	win32SetParent(replacement, win32GetParent(old))
	if frame := frameOf(old); frame != nil {
		record := *frame
		*frameFor(replacement) = record
		placeInParent(replacement, record)
	}
	win32SetWindowPos(replacement, old, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	if win32IsWindowVisible(old) || win32GetWindowLongPtr(old, gwlStyle)&wsVisible != 0 {
		win32ShowWindow(replacement, swShowNA)
	} else {
		win32ShowWindow(replacement, swHide)
	}
	win32EnableWindow(replacement, win32IsWindowEnabled(old))
	clearViewOf(old)
	setViewOf(replacement, v)
	v.outer = replacement
	if win32GetFocus() == old {
		win32SetFocus(replacement)
	}
	win32DestroyWindow(old)
}

// ---- Parking ----

var parkingWindow hwnd

// parking returns the hidden window that holds views without a parent.
func parking() hwnd {
	if parkingWindow == 0 || !win32IsWindow(parkingWindow) {
		cookie := activate()
		parkingWindow = win32CreateWindowEx(wsExToolWindow|wsExControlParent, layoutClass, "GameKit",
			wsPopup|wsClipChildren, 0, 0, 0, 0, 0, 0, instance(), nil)
		deactivate(cookie)
	}
	return parkingWindow
}

// ---- Timers and work ----

const workMessage = wmApp + 1

var (
	timers                = map[uintptr]func() bool{}
	nextTimer     uintptr = 1000
	messageWindow hwnd
)

func messageProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case wmTimer:
		tick, ok := timers[wparam]
		if !ok {
			win32KillTimer(h, wparam)
			return 0
		}
		if !tick() {
			win32KillTimer(h, wparam)
			delete(timers, wparam)
		}
		return 0
	case workMessage:
		runPending()
		return 0
	case wmSettingChange, wmThemeChanged, wmSysColorChange:
		applyThemes()
	}
	return win32DefWindowProc(h, message, wparam, lparam)
}

func messages() hwnd {
	if messageWindow == 0 {
		// A top-level window, not a message-only window, so it receives the
		// broadcast of setting changes.
		messageWindow = win32CreateWindowEx(wsExToolWindow, messagesClass, "GameKit", wsPopup, 0, 0, 0, 0, 0, 0,
			instance(), nil)
	}
	return messageWindow
}

// startTimer runs a function on the UI thread after a delay, repeating while it
// returns true. It returns an identifier for cancelTimer.
func startTimer(milliseconds uint32, tick func() bool) uintptr {
	id := nextTimer
	nextTimer++
	timers[id] = tick
	win32SetTimer(messages(), id, max(milliseconds, 1), 0)
	return id
}

func cancelTimer(id uintptr) {
	if id == 0 {
		return
	}
	win32KillTimer(messages(), id)
	delete(timers, id)
}

var (
	pendingMu sync.Mutex
	pending   []func()
)

// later runs work on the UI thread after the current message. It may be called
// from any goroutine.
func later(work func()) {
	pendingMu.Lock()
	pending = append(pending, work)
	pendingMu.Unlock()
	if messageWindow != 0 {
		win32PostMessage(messageWindow, workMessage, 0, 0)
	}
}

func runPending() {
	pendingMu.Lock()
	work := pending
	pending = nil
	pendingMu.Unlock()
	for _, run := range work {
		run()
	}
}

// ---- Message loop ----

func pumpDrain() int {
	var message msg
	count := 0
	for ; count < 4096 && win32PeekMessage(&message, 0, 0, 0, pmRemove); count++ {
		if message.message == wmQuit {
			// A quit request is reported through the application events; the Go
			// layer decides when the process ends.
			continue
		}
		if promptKey(&message) || windowInput(&message) {
			continue
		}
		win32TranslateMessage(&message)
		win32DispatchMessage(&message)
	}
	return count
}

// pumpOnce processes the pending messages without waiting.
func pumpOnce() {
	pumpDrain()
	runPending()
}

// pump processes messages until done returns true or timeout seconds pass.
func pump(done func() bool, timeout float64) {
	deadline := win32GetTickCount64() + uint64(timeout*1000)
	for !done() && win32GetTickCount64() < deadline {
		pumpOnce()
		if done() {
			break
		}
		win32MsgWaitForMultipleObjectsEx(0, nil, 2, qsAllInput, mwmoInputAvailable)
	}
}

// ---- Initialization ----

// instance returns the module of the program.
func instance() hinstance {
	return win32GetModuleHandle("")
}

var initialized bool

// initialize prepares the calling thread for the user interface.
func initialize() error {
	if initialized {
		return nil
	}
	win32SetProcessDpiAwarenessContext(dpiAwarenessContextPerMonitorAwareV2)
	if result := win32OleInitialize(0); result.failed() && result != rpcEChangedMode {
		return fmt.Errorf("ui/win32: OleInitialize failed (%v)", result)
	}
	cookie := activate()
	defer deactivate(cookie)
	if procInitCommonControlsEx.find() == 0 || procSetWindowSubclass.find() == 0 {
		return fmt.Errorf("ui/win32: the common controls of Windows are not available")
	}
	controls := initCommonControlsExInfo{
		dwICC: iccWin95Classes | iccDateClasses | iccUserexClasses | iccCoolClasses | iccLinkClass |
			iccPagescrollerClass | iccStandardClasses | iccTabClasses | iccBarClasses | iccProgressClass |
			iccUpdownClass | iccHotkeyClass | iccListviewClasses | iccTreeviewClasses,
	}
	controls.dwSize = uint32(unsafe.Sizeof(controls))
	win32InitCommonControlsEx(&controls)
	loadLibraryDefault("msftedit.dll")
	win32BufferedPaintInit()
	initializeUxTheme()
	uiThread = win32GetCurrentThreadID()

	arrow := win32LoadCursor(0, idcArrow)
	layout := wndClassEx{
		lpfnWndProc: callback("layout", layoutProc),
		hInstance:   instance(),
		hCursor:     arrow,
	}
	registerClass(&layout, layoutClass)
	control := wndClassEx{
		style:       csHRedraw | csVRedraw | csDblClks,
		lpfnWndProc: callback("control", controlProc),
		hInstance:   instance(),
		hCursor:     arrow,
	}
	registerClass(&control, controlClass)
	messageClass := wndClassEx{
		lpfnWndProc: callback("messages", messageProc),
		hInstance:   instance(),
	}
	registerClass(&messageClass, messagesClass)
	messages()

	initialized = true
	return nil
}

// registerClass registers a window class with a name.
func registerClass(class *wndClassEx, name string) {
	class.cbSize = uint32(unsafe.Sizeof(*class))
	class.lpszClassName = wide(name)
	win32RegisterClassEx(class)
}

// ---- Keys ----

// Modifier bits reported alongside key and button events. They match GLFW.
const (
	modShift    = 1 << 0
	modControl  = 1 << 1
	modAlt      = 1 << 2
	modSuper    = 1 << 3
	modCapsLock = 1 << 4
	modNumLock  = 1 << 5
)

func keyDown(key int32) bool {
	return win32GetKeyState(key)&0x8000 != 0
}

func keyToggled(key int32) bool {
	return win32GetKeyState(key)&1 != 0
}

// modifiers returns the modifier keys that are down.
func modifiers() int {
	mods := 0
	if keyDown(vkShift) {
		mods |= modShift
	}
	if keyDown(vkControl) {
		mods |= modControl
	}
	if keyDown(vkMenu) {
		mods |= modAlt
	}
	if keyDown(vkLWin) || keyDown(vkRWin) {
		mods |= modSuper
	}
	if keyToggled(vkCapital) {
		mods |= modCapsLock
	}
	if keyToggled(vkNumLock) {
		mods |= modNumLock
	}
	return mods
}

// virtualKeys maps the virtual keys that are not letters, digits, function
// keys, or the numeric keypad to GLFW keys.
var virtualKeys = map[uint32]int{
	vkSpace:     32,
	vkOEM7:      39,
	vkOEMComma:  44,
	vkOEMMinus:  45,
	vkOEMPeriod: 46,
	vkOEM2:      47,
	vkOEM1:      59,
	vkOEMPlus:   61,
	vkOEM4:      91,
	vkOEM5:      92,
	vkOEM6:      93,
	vkOEM3:      96,
	vkOEM102:    162,
	vkEscape:    256,
	vkTab:       258,
	vkBack:      259,
	vkInsert:    260,
	vkDelete:    261,
	vkRight:     262,
	vkLeft:      263,
	vkDown:      264,
	vkUp:        265,
	vkPrior:     266,
	vkNext:      267,
	vkHome:      268,
	vkEnd:       269,
	vkCapital:   280,
	vkScroll:    281,
	vkNumLock:   282,
	vkSnapshot:  283,
	vkPause:     284,
	vkDecimal:   330,
	vkDivide:    331,
	vkMultiply:  332,
	vkSubtract:  333,
	vkAdd:       334,
	vkLShift:    340,
	vkLControl:  341,
	vkLMenu:     342,
	vkLWin:      343,
	vkRShift:    344,
	vkRControl:  345,
	vkRMenu:     346,
	vkRWin:      347,
	vkApps:      348,
}

// keyFromVirtualKey returns the GLFW key of a virtual key, or -1.
func keyFromVirtualKey(value, details uintptr) int {
	key := uint32(value)
	extended := details&(1<<24) != 0
	switch key {
	case vkShift:
		scan := uint32((details >> 16) & 0xff)
		key = win32MapVirtualKey(scan, mapvkVscToVkEx)
	case vkControl:
		key = vkLControl
		if extended {
			key = vkRControl
		}
	case vkMenu:
		key = vkLMenu
		if extended {
			key = vkRMenu
		}
	}
	switch {
	case key >= 'A' && key <= 'Z', key >= '0' && key <= '9':
		return int(key)
	case key >= vkF1 && key <= vkF24:
		return 290 + int(key-vkF1)
	case key >= vkNumpad0 && key <= vkNumpad9:
		return 320 + int(key-vkNumpad0)
	case key == vkReturn:
		if extended {
			return 335
		}
		return 257
	}
	if glfw, ok := virtualKeys[key]; ok {
		return glfw
	}
	return -1
}

func loword(value uintptr) uint16 {
	return uint16(value)
}

func hiword(value uintptr) uint16 {
	return uint16(value >> 16)
}

// signedLoword and signedHiword return the coordinates of an LPARAM.
func signedLoword(value uintptr) int {
	return int(int16(uint16(value)))
}

func signedHiword(value uintptr) int {
	return int(int16(uint16(value >> 16)))
}

// mapWindowRect converts a rectangle from the coordinates of one window to
// another; 0 is the screen.
func mapWindowRect(from, to hwnd, r *rect) {
	win32MapWindowPoints(from, to, (*point)(unsafe.Pointer(r)), 2)
}
