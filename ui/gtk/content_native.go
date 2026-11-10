//go:build linux && cgo

package gtk

import (
	"net/url"
	"strings"
	"sync"
	"unsafe"
)

var webKitSymbols struct {
	once                                                        sync.Once
	library                                                     *gModule
	newView, loadURI, loadHTML, getURI, settings, scripts, stop unsafe.Pointer
}
var optionalGTK struct {
	once    sync.Once
	library *gModule
	svg     unsafe.Pointer
}

func nativeModuleSymbol(module *gModule, name string) unsafe.Pointer {
	var symbol unsafe.Pointer
	gModuleSymbol(module, stringPointer(name), &symbol)
	return symbol
}
func loadWebKit() bool {
	webKitSymbols.once.Do(func() {
		module := gModuleOpen(stringPointer("libwebkitgtk-6.0.so.4"), 0)
		if module == nil {
			return
		}
		webKitSymbols.library = module
		webKitSymbols.newView = nativeModuleSymbol(module, "webkit_web_view_new")
		webKitSymbols.loadURI = nativeModuleSymbol(module, "webkit_web_view_load_uri")
		webKitSymbols.loadHTML = nativeModuleSymbol(module, "webkit_web_view_load_html")
		webKitSymbols.getURI = nativeModuleSymbol(module, "webkit_web_view_get_uri")
		webKitSymbols.settings = nativeModuleSymbol(module, "webkit_web_view_get_settings")
		webKitSymbols.scripts = nativeModuleSymbol(module, "webkit_settings_set_enable_javascript")
		webKitSymbols.stop = nativeModuleSymbol(module, "webkit_web_view_stop_loading")
	})
	return webKitSymbols.newView != nil && webKitSymbols.loadURI != nil && webKitSymbols.loadHTML != nil && webKitSymbols.getURI != nil
}
func loadOptionalGTK() {
	optionalGTK.once.Do(func() {
		optionalGTK.library = gModuleOpen(stringPointer("libgtk-4.so.1"), 0)
		if optionalGTK.library != nil {
			optionalGTK.svg = nativeModuleSymbol(optionalGTK.library, "gtk_svg_new_from_bytes")
		}
	})
}
func gkGTKWebViewCreate(window cUintptr, scripts cInt) unsafe.Pointer {
	if !loadWebKit() {
		return nil
	}
	web := webkitWebViewNew()
	if web == nil {
		return nil
	}
	if webKitSymbols.settings != nil && webKitSymbols.scripts != nil {
		webkitSettingsSetEnableJavaScript(webkitWebViewGetSettings(web), nativeBool(scripts != 0))
	}
	registerNativeComponent(uintptr(window), (*gtkWidget)(unsafe.Pointer(web)), nil, kindWeb)
	return unsafe.Pointer(web)
}
func gkGTKWebViewLoad(pointer unsafe.Pointer, uri *cChar) cInt {
	value, err := url.Parse(goString(uri))
	if err != nil || value.Scheme == "" {
		return 0
	}
	webkitWebViewLoadURI((*webkitWebView)(pointer), uri)
	return 1
}
func gkGTKWebViewLoadHTML(pointer unsafe.Pointer, html *cChar) {
	webkitWebViewLoadHTML((*webkitWebView)(pointer), html, nil)
}
func gkGTKWebViewURL(pointer unsafe.Pointer) *cChar {
	return gStrdup(webkitWebViewGetURI((*webkitWebView)(pointer)))
}
func gkGTKWebViewRelease(pointer unsafe.Pointer) {
	if webKitSymbols.stop != nil {
		webkitWebViewStopLoading((*webkitWebView)(pointer))
	}
}

type nativeVideo struct{ stream *gtkMediaStream }

func (video *nativeVideo) releaseNative() {
	gtkMediaStreamPause(video.stream)
	gObjectUnref(unsafe.Pointer(video.stream))
}
func gkGTKVideoCreate(window cUintptr, source *cChar, controls, autoplay, muted cInt) unsafe.Pointer {
	var file *gFile
	if strings.Contains(goString(source), "://") {
		file = gFileNewForURI(source)
	} else {
		file = gFileNewForPath(source)
	}
	stream := gtkMediaFileNewForFile(file)
	gObjectUnref(unsafe.Pointer(file))
	gtkMediaStreamSetMuted(stream, nativeBool(muted != 0))
	var widget *gtkWidget
	if controls != 0 {
		widget = gtkVideoNew()
		gtkVideoSetMediaStream((*gtkVideo)(unsafe.Pointer(widget)), stream)
	} else {
		widget = gtkPictureNewForPaintable((*gdkPaintable)(unsafe.Pointer(stream)))
		gtkPictureSetContentFit((*gtkPicture)(unsafe.Pointer(widget)), gtkContentFitContain)
	}
	registerNativeComponent(uintptr(window), widget, nil, kindVideo)
	storeComponentState(widget, &nativeVideo{stream: stream})
	if autoplay != 0 {
		gtkMediaStreamPlay(stream)
	}
	return unsafe.Pointer(widget)
}
func gkGTKVideoPlay(pointer unsafe.Pointer, play cInt) {
	if video := componentState[nativeVideo](pointer); video != nil {
		if play != 0 {
			gtkMediaStreamPlay(video.stream)
		} else {
			gtkMediaStreamPause(video.stream)
		}
	}
}
func gkGTKVideoPlaying(pointer unsafe.Pointer) cInt {
	if video := componentState[nativeVideo](pointer); video != nil && gtkMediaStreamGetPlaying(video.stream) != 0 {
		return 1
	}
	return 0
}
func gkGTKVideoPosition(pointer unsafe.Pointer) cDouble {
	if video := componentState[nativeVideo](pointer); video != nil {
		return cDouble(gtkMediaStreamGetTimestamp(video.stream)) / 1e6
	}
	return 0
}
func gkGTKVideoDuration(pointer unsafe.Pointer) cDouble {
	if video := componentState[nativeVideo](pointer); video != nil {
		return cDouble(gtkMediaStreamGetDuration(video.stream)) / 1e6
	}
	return 0
}
func gkGTKVideoSeek(pointer unsafe.Pointer, seconds cDouble) {
	if video := componentState[nativeVideo](pointer); video != nil && gtkMediaStreamIsSeekable(video.stream) != 0 {
		gtkMediaStreamSeek(video.stream, cInt64(max(0, seconds)*1e6))
	}
}
func gkGTKVideoRelease(pointer unsafe.Pointer) { gkGTKVideoPlay(pointer, 0) }

func gkGTKIconCreate(window cUintptr) unsafe.Pointer {
	widget := widgetWithRole(gtkImageGetType(), gtkAccessibleRoleImg)
	registerNativeComponent(uintptr(window), widget, nil, kindIcon)
	return unsafe.Pointer(widget)
}
func gkGTKIconSet(pointer, image unsafe.Pointer, width, height, tint cInt, name *cChar) {
	widget := (*gtkWidget)(pointer)
	gtkImageSetFromPaintable((*gtkImage)(pointer), (*gdkPaintable)(image))
	size := max(width, height)
	if size <= 0 {
		size = 16
	}
	gtkImageSetPixelSize((*gtkImage)(pointer), size)
	selector := "image." + componentCSSClass(widget)
	rule := ""
	if color := semanticCSSColor(int(tint)); color != "" {
		rule = "color: " + color + ";"
	}
	setComponentCSS(selector, rule)
	if goString(name) != "" {
		setAccessibleText(widget, gtkAccessiblePropertyLabel, goString(name))
		setAccessibleBoolState(widget, gtkAccessibleStateHidden, false)
	} else {
		setAccessibleBoolState(widget, gtkAccessibleStateHidden, true)
	}
}
func gkGTKVectorCreate(window cUintptr) unsafe.Pointer {
	widget := widgetWithRole(gtkPictureGetType(), gtkAccessibleRoleImg)
	gtkPictureSetContentFit((*gtkPicture)(unsafe.Pointer(widget)), gtkContentFitContain)
	registerNativeComponent(uintptr(window), widget, nil, kindVector)
	return unsafe.Pointer(widget)
}
func gkGTKVectorSet(pointer unsafe.Pointer, data *cUchar, size cSize, label *cChar, width, height *cDouble) cInt {
	if data == nil || size == 0 {
		return 0
	}
	bytes := gBytesNew(unsafe.Pointer(data), gSize(size))
	defer gBytesUnref(bytes)
	var paintable *gdkPaintable
	loadOptionalGTK()
	if optionalGTK.svg != nil {
		paintable = gtkSvgNewFromBytes(bytes)
		if paintable != nil && gdkPaintableGetIntrinsicWidth(paintable) <= 0 {
			gObjectUnref(unsafe.Pointer(paintable))
			paintable = nil
		}
	}
	if paintable == nil {
		var failure *gError
		paintable = (*gdkPaintable)(gdkTextureNewFromBytes(bytes, &failure))
		if failure != nil {
			gErrorFree(failure)
		}
	}
	if paintable == nil {
		return 0
	}
	defer gObjectUnref(unsafe.Pointer(paintable))
	gtkPictureSetPaintable((*gtkPicture)(pointer), paintable)
	*width, *height = cDouble(gdkPaintableGetIntrinsicWidth(paintable)), cDouble(gdkPaintableGetIntrinsicHeight(paintable))
	if goString(label) != "" {
		setAccessibleText((*gtkWidget)(pointer), gtkAccessiblePropertyLabel, goString(label))
	}
	return 1
}

type nativeAnimation struct {
	widget           *gtkWidget
	frames           []*gdkPaintable
	durations        []float64
	repeats, playing bool
	index            int
	shown            int64
	tick             cUint
}

func (animation *nativeAnimation) releaseNative() {
	for _, frame := range animation.frames {
		if frame != nil {
			gObjectUnref(unsafe.Pointer(frame))
		}
	}
}
func gkGTKAnimationCreate(window cUintptr, images *unsafe.Pointer, durations *cDouble, count, repeats cInt) unsafe.Pointer {
	widget := widgetWithRole(gtkPictureGetType(), gtkAccessibleRoleImg)
	gtkPictureSetContentFit((*gtkPicture)(unsafe.Pointer(widget)), gtkContentFitContain)
	animation := &nativeAnimation{
		widget:    widget,
		frames:    append([]*gdkPaintable(nil), borrowedPaintables(images, int(count))...),
		durations: make([]float64, int(count)),
		repeats:   repeats != 0,
	}
	for i, duration := range unsafe.Slice(durations, int(count)) {
		animation.durations[i] = float64(duration)
		if animation.frames[i] != nil {
			gObjectRef(unsafe.Pointer(animation.frames[i]))
		}
	}
	if count > 0 {
		gtkPictureSetPaintable((*gtkPicture)(unsafe.Pointer(widget)), animation.frames[0])
	}
	registerNativeComponent(uintptr(window), widget, nil, kindAnimation)
	storeComponentState(widget, animation)
	return unsafe.Pointer(widget)
}
func (animation *nativeAnimation) advance(clock *gdkFrameClock) bool {
	if !animation.playing || len(animation.frames) < 2 {
		animation.tick = 0
		return false
	}
	now := int64(gdkFrameClockGetFrameTime(clock))
	if animation.shown == 0 {
		animation.shown = now
	}
	duration := animation.durations[animation.index]
	if duration <= 0 {
		duration = 0.1
	}
	if now-animation.shown < int64(duration*1e6) {
		return true
	}
	animation.shown = now
	if animation.index+1 >= len(animation.frames) && !animation.repeats {
		animation.playing = false
		animation.tick = 0
		return false
	}
	animation.index = (animation.index + 1) % len(animation.frames)
	gtkPictureSetPaintable((*gtkPicture)(unsafe.Pointer(animation.widget)), animation.frames[animation.index])
	return true
}
func gkGTKAnimationPlay(pointer unsafe.Pointer, play cInt) {
	animation := componentState[nativeAnimation](pointer)
	if animation == nil {
		return
	}
	animation.playing = play != 0 && len(animation.frames) > 1
	if animation.playing && animation.tick == 0 {
		if !animation.repeats && animation.index+1 >= len(animation.frames) {
			animation.index = 0
			gtkPictureSetPaintable((*gtkPicture)(pointer), animation.frames[0])
		}
		animation.shown = 0
		animation.tick = gtkWidgetAddTickCallback(animation.widget, gtkTickCallbackAddress(), newNativeCallback(animation.advance), gtkDestroyCallback())
	}
}
func gkGTKAnimationPlaying(pointer unsafe.Pointer) cInt {
	if animation := componentState[nativeAnimation](pointer); animation != nil && animation.playing {
		return 1
	}
	return 0
}
