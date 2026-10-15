//go:build darwin && cgo

package cocoa

/*
#include "cocoa_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// ---- Icon ----

type icon struct {
	*widgetBase
	options ui.IconOptions
}

// CreateIcon implements ui.Window.CreateIcon.
func (w *Window) CreateIcon(options ui.IconOptions) (ui.Icon, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &icon{options: options}
	if result.widgetBase, err = w.newComponent(C.gkCocoaIconCreate(window), ui.RoleImage, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (i *icon) cocoaEvent(cocoaEvent) {
	// An icon sends no native events.
}

func (i *icon) apply() {
	native := i.nativePointer()
	if native == nil {
		return
	}
	size := i.options.Size
	pointSize := float64(max(size.Width, size.Height))
	image := nativeIcon(i.Source(), pointSize)
	defer C.gkCocoaRelease(image)
	name := cString(i.options.AccessibilityName)
	defer freeCString(name)
	C.gkCocoaIconSet(native, image, C.int(size.Width), C.int(size.Height), C.int(i.options.Tint), name)
}

func (i *icon) Source() ui.IconSource {
	return ui.IconSource{SystemName: i.options.SystemName, Image: i.options.Image}
}

func (i *icon) SetSource(source ui.IconSource) error {
	if source.IsZero() {
		return errors.New("ui: icon requires a system name or image")
	}
	if i.nativePointer() == nil {
		return ui.ErrClosed
	}
	i.options.SystemName, i.options.Image = source.SystemName, source.Image
	i.apply()
	return nil
}

// ---- Rich text ----

type document struct {
	*widgetBase
	changes listeners[ui.FocusHandler]
	links   listeners[func(string)]
}

func (d *document) cocoaEvent(event cocoaEvent) {
	if event.kind == C.GK_EVENT_TEXT_CHANGE {
		each(&d.changes, func(handler ui.FocusHandler) {
			handler()
		})
	}
}

// cocoaQuery reports whether a link click was handled; without handlers the
// text view opens the link itself.
func (d *document) cocoaQuery(event cocoaEvent) bool {
	if event.kind != C.GK_EVENT_LINK {
		return false
	}
	handlers := d.links.snapshot()
	for _, handler := range handlers {
		handler(event.text)
	}
	return len(handlers) > 0
}

func (d *document) Text() string {
	native := d.nativePointer()
	if native == nil {
		return ""
	}
	return goStringFree(C.gkCocoaDocumentText(native))
}

func (d *document) SetText(text string) {
	if native := d.nativePointer(); native != nil {
		value := cString(text)
		defer freeCString(value)
		C.gkCocoaDocumentSetText(native, value)
	}
}

type richText struct{ document }

// CreateRichText implements ui.Window.CreateRichText.
func (w *Window) CreateRichText(options ui.RichTextOptions) (ui.RichText, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &richText{}
	if result.widgetBase, err = w.newComponent(C.gkCocoaDocumentCreate(window, boolInt(options.Editable), 0), ui.RoleTextArea, result); err != nil {
		return nil, err
	}
	result.SetText(options.Text)
	if len(options.RTF) > 0 {
		if err := result.SetRTF(options.RTF); err != nil {
			result.Close()
			return nil, err
		}
	}
	return result, nil
}

func (r *richText) cocoaQuery(event cocoaEvent) bool {
	return r.document.cocoaQuery(event)
}

func (r *richText) RTF() ([]byte, error) {
	native := r.nativePointer()
	if native == nil {
		return nil, ui.ErrClosed
	}
	var size C.size_t
	data := C.gkCocoaDocumentRTF(native, &size)
	defer C.free(unsafe.Pointer(data))
	return C.GoBytes(unsafe.Pointer(data), C.int(size)), nil
}

func (r *richText) SetRTF(rtf []byte) error {
	native := r.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	if len(rtf) == 0 || C.gkCocoaDocumentSetRTF(native, (*C.uchar)(unsafe.Pointer(&rtf[0])), C.size_t(len(rtf))) == 0 {
		return errors.New("ui/cocoa: the data is not valid RTF")
	}
	return nil
}

func (r *richText) IsEditable() bool {
	native := r.nativePointer()
	return native != nil && C.gkCocoaDocumentEditable(native) != 0
}

func (r *richText) SetEditable(editable bool) {
	if native := r.nativePointer(); native != nil {
		C.gkCocoaDocumentSetEditable(native, boolInt(editable))
	}
}

func (r *richText) OnChange(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&r.changes, handler, handler == nil)
}

// ---- Markdown ----

type markdown struct {
	document
	source string
}

// CreateMarkdown implements ui.Window.CreateMarkdown.
func (w *Window) CreateMarkdown(options ui.MarkdownOptions) (ui.Markdown, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &markdown{}
	if result.widgetBase, err = w.newComponent(C.gkCocoaDocumentCreate(window, 0, 1), ui.RoleDocument, result); err != nil {
		return nil, err
	}
	if err := result.SetSource(options.Source); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (m *markdown) cocoaQuery(event cocoaEvent) bool {
	return m.document.cocoaQuery(event)
}

func (m *markdown) Source() string {
	return m.source
}

func (m *markdown) SetSource(source string) error {
	native := m.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	value := cString(source)
	defer freeCString(value)
	if C.gkCocoaMarkdownSet(native, value) == 0 {
		return errors.New("ui/cocoa: the Markdown source could not be parsed")
	}
	m.source = source
	return nil
}

func (m *markdown) OnLinkActivate(handler func(url string)) ui.Unsubscribe {
	return subscribe(&m.links, handler, handler == nil)
}

// ---- SVG ----

type vector struct {
	*widgetBase
	size ui.Size
	text string
}

// CreateSVG implements ui.Window.CreateSVG.
func (w *Window) CreateSVG(options ui.SVGOptions) (ui.SVG, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &vector{text: options.AccessibilityText}
	if result.widgetBase, err = w.newComponent(C.gkCocoaVectorCreate(window), ui.RoleImage, result); err != nil {
		return nil, err
	}
	if err := result.SetSource(options.Source); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (v *vector) cocoaEvent(cocoaEvent) {
	// A vector image sends no native events.
}

func (v *vector) SetSource(source []byte) error {
	if len(source) == 0 {
		return errors.New("ui: SVG source is empty")
	}
	native := v.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	label := cString(v.text)
	defer freeCString(label)
	var width, height C.double
	if C.gkCocoaVectorSet(native, (*C.uchar)(unsafe.Pointer(&source[0])), C.size_t(len(source)), label, &width, &height) == 0 {
		return fmt.Errorf("ui/cocoa: AppKit could not decode the SVG: %w", ui.ErrNotSupported)
	}
	v.size = ui.Size{Width: int(width), Height: int(height)}
	v.relayoutParent()
	return nil
}

func (v *vector) IntrinsicSize() ui.Size {
	return v.size
}

// ---- Animated image ----

type animatedImage struct {
	*widgetBase
}

// CreateAnimatedImage implements ui.Window.CreateAnimatedImage.
func (w *Window) CreateAnimatedImage(options ui.AnimatedImageOptions) (ui.AnimatedImage, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	sources := make([]ui.IconSource, len(options.Frames))
	durations := make([]C.double, len(options.Frames))
	for i, frame := range options.Frames {
		sources[i] = ui.IconSource{Image: frame.Image}
		durations[i] = C.double(frame.Duration.Seconds())
	}
	array := newIconArray(sources, 0)
	defer array.free()
	result := &animatedImage{}
	native := C.gkCocoaAnimationCreate(window, array.native(), &durations[0], C.int(len(options.Frames)), boolInt(options.Repeat))
	if result.widgetBase, err = w.newComponent(native, ui.RoleImage, result); err != nil {
		return nil, err
	}
	// Respect the user's reduced-motion preference until the app plays it.
	if !w.SystemPreferences().ReducedMotion && len(options.Frames) > 1 {
		result.Play()
	}
	return result, nil
}

func (a *animatedImage) cocoaEvent(cocoaEvent) {
	// An animated image sends no native events.
}

func (a *animatedImage) Play() {
	if native := a.nativePointer(); native != nil {
		C.gkCocoaAnimationPlay(native, 1)
	}
}

func (a *animatedImage) Pause() {
	if native := a.nativePointer(); native != nil {
		C.gkCocoaAnimationPlay(native, 0)
	}
}

func (a *animatedImage) IsPlaying() bool {
	native := a.nativePointer()
	return native != nil && C.gkCocoaAnimationPlaying(native) != 0
}

// ---- Video ----

type video struct {
	*widgetBase
}

// CreateVideo implements ui.Window.CreateVideo.
func (w *Window) CreateVideo(options ui.VideoOptions) (ui.Video, error) {
	if strings.TrimSpace(options.Source) == "" {
		return nil, errors.New("ui: video source is required")
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	source := cString(options.Source)
	defer freeCString(source)
	result := &video{}
	native := C.gkCocoaVideoCreate(window, source, boolInt(options.ShowControls), boolInt(options.Autoplay), boolInt(options.Muted))
	if result.widgetBase, err = w.newComponent(native, ui.RoleVideo, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (v *video) cocoaEvent(cocoaEvent) {
	// A video view sends no native events.
}

func (v *video) Close() {
	if native := v.nativePointer(); native != nil {
		C.gkCocoaVideoRelease(native)
	}
	v.widgetBase.Close()
}

func (v *video) Play() {
	if native := v.nativePointer(); native != nil {
		C.gkCocoaVideoPlay(native, 1)
	}
}

func (v *video) Pause() {
	if native := v.nativePointer(); native != nil {
		C.gkCocoaVideoPlay(native, 0)
	}
}

func (v *video) IsPlaying() bool {
	native := v.nativePointer()
	return native != nil && C.gkCocoaVideoPlaying(native) != 0
}

func seconds(value C.double) time.Duration {
	return time.Duration(float64(value) * float64(time.Second))
}

func (v *video) Position() time.Duration {
	native := v.nativePointer()
	if native == nil {
		return 0
	}
	return seconds(C.gkCocoaVideoPosition(native))
}

func (v *video) Duration() time.Duration {
	native := v.nativePointer()
	if native == nil {
		return 0
	}
	return seconds(C.gkCocoaVideoDuration(native))
}

func (v *video) Seek(position time.Duration) error {
	if position < 0 {
		return errors.New("ui: video position must be non-negative")
	}
	native := v.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkCocoaVideoSeek(native, C.double(position.Seconds()))
	return nil
}

// ---- Web view ----

type webView struct {
	*widgetBase
	navigations listeners[func(string)]
}

// CreateWebView implements ui.Window.CreateWebView.
func (w *Window) CreateWebView(options ui.WebViewOptions) (ui.WebView, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &webView{}
	if result.widgetBase, err = w.newComponent(C.gkCocoaWebViewCreate(window, boolInt(options.AllowScripts)), ui.RoleWebView, result); err != nil {
		return nil, err
	}
	switch {
	case options.HTML != "":
		err = result.LoadHTML(options.HTML)
	case options.URL != "":
		err = result.Load(options.URL)
	}
	if err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (v *webView) cocoaEvent(event cocoaEvent) {
	if event.kind == C.GK_EVENT_NAVIGATE {
		url := event.text
		each(&v.navigations, func(handler func(string)) {
			handler(url)
		})
	}
}

func (v *webView) Close() {
	if native := v.nativePointer(); native != nil {
		C.gkCocoaWebViewRelease(native)
	}
	v.widgetBase.Close()
}

func (v *webView) URL() string {
	native := v.nativePointer()
	if native == nil {
		return ""
	}
	return goStringFree(C.gkCocoaWebViewURL(native))
}

func (v *webView) Load(url string) error {
	native := v.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	value := cString(url)
	defer freeCString(value)
	if C.gkCocoaWebViewLoad(native, value) == 0 {
		return fmt.Errorf("ui: %q is not an absolute URL", url)
	}
	return nil
}

func (v *webView) LoadHTML(html string) error {
	native := v.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	value := cString(html)
	defer freeCString(value)
	C.gkCocoaWebViewLoadHTML(native, value)
	return nil
}

func (v *webView) OnNavigate(handler func(url string)) ui.Unsubscribe {
	return subscribe(&v.navigations, handler, handler == nil)
}

var (
	_ ui.Icon          = (*icon)(nil)
	_ ui.RichText      = (*richText)(nil)
	_ ui.Markdown      = (*markdown)(nil)
	_ ui.SVG           = (*vector)(nil)
	_ ui.AnimatedImage = (*animatedImage)(nil)
	_ ui.Video         = (*video)(nil)
	_ ui.WebView       = (*webView)(nil)
)
