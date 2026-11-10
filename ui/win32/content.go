//go:build windows

package win32

import (
	"errors"
	"fmt"
	"strings"
	"time"

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
	if result.widgetBase, err = w.newComponent(window.newIconView(), ui.RoleImage, result); err != nil {
		return nil, err
	}
	result.apply()
	return result, nil
}

func (i *icon) receiveEvent(event any) {
	// An icon sends no native events.
}

func (i *icon) apply() {
	native := i.nativeView()
	if native == nil {
		return
	}
	size := i.options.Size
	pointSize := float64(max(size.Width, size.Height))
	image := newIcon(i.Source(), pointSize)
	native.setIcon(image, size.Width, size.Height, int(i.options.Tint), i.options.AccessibilityName)
}

func (i *icon) Source() ui.IconSource {
	return ui.IconSource{SystemName: i.options.SystemName, Image: i.options.Image}
}

func (i *icon) SetSource(source ui.IconSource) error {
	if source.IsZero() {
		return errors.New("ui: icon requires a system name or image")
	}
	if i.nativeView() == nil {
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

func (d *document) receiveEvent(event any) {
	if _, ok := event.(textChangeEvent); ok {
		each(&d.changes, func(handler ui.FocusHandler) {
			handler()
		})
	}
}

// answerQuery reports whether the application handled a link; without
// handlers the text view opens the link itself.
func (d *document) answerQuery(query any) bool {
	link, ok := query.(linkRequest)
	if !ok {
		return false
	}
	handlers := d.links.snapshot()
	for _, handler := range handlers {
		handler(link.url)
	}
	return len(handlers) > 0
}

func (d *document) Text() string {
	native := d.nativeView()
	if native == nil {
		return ""
	}
	return native.documentText()
}

func (d *document) SetText(text string) {
	if native := d.nativeView(); native != nil {
		native.setDocumentText(text)
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
	if result.widgetBase, err = w.newComponent(window.newDocument(options.Editable, false), ui.RoleTextArea, result); err != nil {
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

func (r *richText) RTF() ([]byte, error) {
	native := r.nativeView()
	if native == nil {
		return nil, ui.ErrClosed
	}
	return native.documentRTF(), nil
}

func (r *richText) SetRTF(rtf []byte) error {
	native := r.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	if len(rtf) == 0 || !native.setDocumentRTF(rtf) {
		return errors.New("ui/win32: the data is not valid RTF")
	}
	return nil
}

func (r *richText) IsEditable() bool {
	native := r.nativeView()
	return native != nil && native.isDocumentEditable()
}

func (r *richText) SetEditable(editable bool) {
	if native := r.nativeView(); native != nil {
		native.setDocumentEditable(editable)
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
	if result.widgetBase, err = w.newComponent(window.newDocument(false, true), ui.RoleDocument, result); err != nil {
		return nil, err
	}
	if err := result.SetSource(options.Source); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (m *markdown) Source() string {
	return m.source
}

func (m *markdown) SetSource(source string) error {
	native := m.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	if !native.setMarkdown(source) {
		return errors.New("ui/win32: the Markdown source could not be parsed")
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
	if result.widgetBase, err = w.newComponent(window.newVectorImage(), ui.RoleImage, result); err != nil {
		return nil, err
	}
	if err := result.SetSource(options.Source); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (v *vector) receiveEvent(event any) {
	// A vector image sends no native events.
}

func (v *vector) SetSource(source []byte) error {
	if len(source) == 0 {
		return errors.New("ui: SVG source is empty")
	}
	native := v.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	width, height, ok := native.setVectorImage(source, v.text)
	if !ok {
		return fmt.Errorf("ui/win32: Windows could not decode the SVG: %w", ui.ErrNotSupported)
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
	result := &animatedImage{}
	native := window.newAnimatedImage(options.Frames, options.Repeat)
	if result.widgetBase, err = w.newComponent(native, ui.RoleImage, result); err != nil {
		return nil, err
	}
	// Respect the user's reduced-motion preference until the app plays it.
	if !w.SystemPreferences().ReducedMotion && len(options.Frames) > 1 {
		result.Play()
	}
	return result, nil
}

func (a *animatedImage) receiveEvent(event any) {
	// An animated image sends no native events.
}

func (a *animatedImage) Play() {
	if native := a.nativeView(); native != nil {
		native.playAnimation(true)
	}
}

func (a *animatedImage) Pause() {
	if native := a.nativeView(); native != nil {
		native.playAnimation(false)
	}
}

func (a *animatedImage) IsPlaying() bool {
	native := a.nativeView()
	return native != nil && native.isAnimationPlaying()
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
	result := &video{}
	native := window.newVideo(options.Source, options.ShowControls, options.Autoplay, options.Muted)
	if result.widgetBase, err = w.newComponent(native, ui.RoleVideo, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (v *video) receiveEvent(event any) {
	// A video view sends no native events.
}

func (v *video) Close() {
	if native := v.nativeView(); native != nil {
		native.releaseVideo()
	}
	v.widgetBase.Close()
}

func (v *video) Play() {
	if native := v.nativeView(); native != nil {
		native.playVideo(true)
	}
}

func (v *video) Pause() {
	if native := v.nativeView(); native != nil {
		native.playVideo(false)
	}
}

func (v *video) IsPlaying() bool {
	native := v.nativeView()
	return native != nil && native.isVideoPlaying()
}

func seconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Second))
}

func (v *video) Position() time.Duration {
	native := v.nativeView()
	if native == nil {
		return 0
	}
	return seconds(native.videoPosition())
}

func (v *video) Duration() time.Duration {
	native := v.nativeView()
	if native == nil {
		return 0
	}
	return seconds(native.videoDuration())
}

func (v *video) Seek(position time.Duration) error {
	if position < 0 {
		return errors.New("ui: video position must be non-negative")
	}
	native := v.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.seekVideo(position.Seconds())
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
	if result.widgetBase, err = w.newComponent(window.newWebView(options.AllowScripts), ui.RoleWebView, result); err != nil {
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

// receiveEvent receives no events: WebView2 does not report navigation to
// the backend yet.
func (v *webView) receiveEvent(any) {
}

func (v *webView) Close() {
	if native := v.nativeView(); native != nil {
		native.releaseWebView()
	}
	v.widgetBase.Close()
}

func (v *webView) URL() string {
	native := v.nativeView()
	if native == nil {
		return ""
	}
	return native.webViewURL()
}

func (v *webView) Load(url string) error {
	native := v.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	if !native.loadURL(url) {
		return fmt.Errorf("ui: %q is not an absolute URL", url)
	}
	return nil
}

func (v *webView) LoadHTML(html string) error {
	native := v.nativeView()
	if native == nil {
		return ui.ErrClosed
	}
	native.loadHTML(html)
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
