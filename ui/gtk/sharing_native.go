//go:build linux && cgo

package gtk

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// Linux offers sharing through a popover of desktop services.
type sharePicker struct {
	popover *gtkPopover
	parent  *gtkWindow
	text    string
	urls    []string
	files   []string
	service string
	done    bool
}

func (w *Window) shareNative(ctx context.Context, options ui.ShareOptions) (string, error) {
	var source *gtkWidget
	if owner, ok := options.Source.(nativeWidget); ok && owner.base().window == w {
		source = (*gtkWidget)(owner.base().nativePointer())
	} else if content, ok := w.content.(nativeWidget); ok {
		source = (*gtkWidget)(content.base().nativePointer())
	}
	if source == nil {
		return "", errors.New("ui/gtk: sharing needs a visible source widget or window content")
	}
	parent := w.nativeGTKWindow()
	if parent == nil || gtkWidgetGetVisible((*gtkWidget)(unsafe.Pointer(parent))) == 0 || gtkWidgetGetMapped(source) == 0 {
		return "", errors.New("ui/gtk: the share picker needs a visible window")
	}
	picker := &sharePicker{parent: parent}
	var texts []string
	for _, item := range options.Items {
		if item.Text != "" {
			texts = append(texts, item.Text)
		}
		if item.URL != "" {
			picker.urls = append(picker.urls, item.URL)
		}
		if item.File != "" {
			picker.files = append(picker.files, absolutePaths([]string{item.File})...)
		}
	}
	picker.text = strings.Join(append(texts, picker.urls...), "\n")
	widget := gtkPopoverNew()
	picker.popover = (*gtkPopover)(unsafe.Pointer(widget))
	gObjectRefSink(unsafe.Pointer(widget))
	defer func() {
		picker.done = true
		gtkPopoverPopdown(picker.popover)
		gtkWidgetUnparent(widget)
		gObjectUnref(unsafe.Pointer(widget))
	}()
	list := gtkBoxNew(gtkOrientationVertical, 0)
	appendButton := func(icon, label string, action func()) {
		button := gtkButtonNew()
		gtkWidgetAddCSSClass(button, stringPointer("flat"))
		row := gtkBoxNew(gtkOrientationHorizontal, 8)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), gtkImageNewFromIconName(stringPointer(icon)))
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), gtkLabelNew(stringPointer(label)))
		gtkButtonSetChild((*gtkButton)(unsafe.Pointer(button)), row)
		connectNativeSignal(unsafe.Pointer(button), "clicked", action)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(list)), button)
	}
	appendButton("edit-copy-symbolic", "Copy", func() {
		writeNativeClipboard(ui.ClipboardContent{Text: picker.text, Files: picker.files})
		picker.finish("Copy")
	})
	if picker.text != "" {
		appendButton("mail-send-symbolic", "Mail", func() {
			service := ""
			if openNativeURL("mailto:?body=" + url.QueryEscape(picker.text)) {
				service = "Mail"
			}
			picker.finish(service)
		})
	}
	if len(picker.files) > 0 || len(picker.urls) > 0 {
		appendButton("document-open-symbolic", "Open With", picker.open)
	}
	gtkPopoverSetChild(picker.popover, list)
	gtkWidgetSetParent(widget, source)
	connectNativeSignal(unsafe.Pointer(widget), "closed", func() { picker.finish("") })
	gtkPopoverPopup(picker.popover)
	for !picker.done {
		runNativePending()
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if !picker.done {
			time.Sleep(time.Millisecond)
		}
	}
	if picker.service == "" {
		return "", ui.ErrCancelled
	}
	return picker.service, nil
}

func (picker *sharePicker) finish(service string) {
	if picker.done {
		return
	}
	picker.done = true
	picker.service = service
	gtkPopoverPopdown(picker.popover)
}

func (picker *sharePicker) open() {
	if len(picker.files) > 0 {
		file := gFileNewForPath(stringPointer(picker.files[0]))
		launcher := gtkFileLauncherNew(file)
		gObjectUnref(unsafe.Pointer(file))
		gtkFileLauncherSetAlwaysAsk(launcher, 1)
		gtkFileLauncherLaunch(launcher, picker.parent, nil, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
			gtkFileLauncherLaunchFinish(launcher, result, nil)
			gObjectUnref(unsafe.Pointer(launcher))
		}))
	} else if len(picker.urls) > 0 {
		launcher := gtkUriLauncherNew(stringPointer(picker.urls[0]))
		gtkUriLauncherLaunch(launcher, picker.parent, nil, gtkAsyncReadyCallback(), asyncNativeCallback(func(_ *gObject, result *gAsyncResult) {
			gtkUriLauncherLaunchFinish(launcher, result, nil)
			gObjectUnref(unsafe.Pointer(launcher))
		}))
	}
	picker.finish("Open With")
}
