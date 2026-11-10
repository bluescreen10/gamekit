//go:build linux && cgo

package gtk

import (
	"fmt"
	"runtime/cgo"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// CheckNativePort runs on GTK's main thread from the external TestMain.
// It checks native state and callbacks that Go-facing getter tests cannot see.
func CheckNativePort() error {
	if err := checkNativeDocuments(); err != nil {
		return err
	}
	if err := checkNativeLayoutAndActions(); err != nil {
		return err
	}
	return checkNativeTrayProtocol()
}
func checkNativeDocuments() error {
	document := gkGTKDocumentCreate(0, 1, 0)
	defer gkGTKUIDestroy(document)
	gkGTKSetAccessibility(document, stringPointer("document"), stringPointer("Rich text"), nil, nil, 0, 0, 1, nil, nil, 0)
	source := []byte(`{\rtf1\ansi\ansicpg1252{\fonttbl{\f0 Sans;}{\f1 Monospace;}}{\colortbl;\red255\green0\blue0;}\f0 plain {\b bold} {\i italic} {\cf1 red} {\f1 mono} \u-10179?\u-8704?}`)
	if gkGTKDocumentSetRTF(document, (*cUchar)(unsafe.Pointer(unsafe.SliceData(source))), cSize(len(source))) == 0 {
		return fmt.Errorf("RTF rejected")
	}
	want := "plain bold italic red mono 😀"
	if text := takeString(gkGTKDocumentText(document)); text != want {
		return fmt.Errorf("RTF text = %q, want %q", text, want)
	}
	state := componentState[nativeDocument](document)
	for _, check := range []struct {
		text  string
		valid func(nativeTextStyle) bool
	}{{"bold", func(style nativeTextStyle) bool { return style.bold && !style.italic }}, {"italic", func(style nativeTextStyle) bool { return style.italic && !style.bold }}, {"red", func(style nativeTextStyle) bool { return style.hasColor && style.color[0] == 1 && style.color[1] == 0 }}, {"mono", func(style nativeTextStyle) bool { return style.family == "Monospace" }}} {
		var iter gtkTextIter
		offset := utf8.RuneCountInString(want[:strings.Index(want, check.text)])
		gtkTextBufferGetIterAtOffset(state.buffer, &iter, cInt(offset))
		if style := state.styleAt(&iter); !check.valid(style) {
			return fmt.Errorf("RTF style for %s = %+v", check.text, style)
		}
	}
	var size cSize
	rtf := gkGTKDocumentRTF(document, &size)
	defer gFree(unsafe.Pointer(rtf))
	if gkGTKDocumentSetRTF(document, rtf, size) == 0 || takeString(gkGTKDocumentText(document)) != want {
		return fmt.Errorf("RTF native tag round trip changed the text")
	}
	markdown := "# Heading\n\n**bold** and [site](https://example.com)\n\n```\ncode\n```"
	if gkGTKMarkdownSet(document, stringPointer(markdown)) == 0 {
		return fmt.Errorf("Markdown rejected")
	}
	text := takeString(gkGTKDocumentText(document))
	if !strings.Contains(text, "Heading") || !strings.Contains(text, "bold and site") || !strings.Contains(text, "code") {
		return fmt.Errorf("Markdown rendered %q", text)
	}
	var iter gtkTextIter
	gtkTextBufferGetIterAtOffset(state.buffer, &iter, cInt(utf8.RuneCountInString(text[:strings.Index(text, "site")])))
	tags := gtkTextIterGetTags(&iter)
	defer gSListFree(tags)
	found := false
	for item := tags; item != nil; item = item.next {
		found = found || state.tags[(*gtkTextTag)(item.data)].url == "https://example.com"
	}
	if !found {
		return fmt.Errorf("Markdown lost native link tag")
	}
	entry := gkGTKUICreateTextField(0, 0)
	defer gkGTKUIDestroy(entry)
	gkGTKUISetText(entry, stringPointer("a😀z"))
	gkGTKSetTextSelection(entry, 1, 2)
	var start, length cInt
	gkGTKTextSelection(entry, &start, &length)
	if start != 1 || length != 2 {
		return fmt.Errorf("UTF-16 selection = %d,%d", start, length)
	}
	return nil
}

type nativePortReceiver struct{ events []int }

func (receiver *nativePortReceiver) nativeEvent(event int) {
	receiver.events = append(receiver.events, event)
}
func checkNativeLayoutAndActions() error {
	receiver := &nativePortReceiver{}
	handle := cgo.NewHandle(receiver)
	defer handle.Delete()
	layout := createNativeLayout(gtkAccessibleRoleGroup)
	gObjectRefSink(unsafe.Pointer(layout))
	defer gObjectUnref(unsafe.Pointer(layout))
	button := gkGTKUICreateButton(0, stringPointer("Native button"), cUintptr(handle))
	defer gkGTKUIDestroy(button)
	gkGTKButtonConfigure(button, nil, 0, 0, nil, 0)
	putNativeLayout(layout, (*gtkWidget)(button))
	moveNativeWidget((*gtkWidget)(button), 12, 18, 160, 42)
	allocated := false
	layoutState(layout).allocate = func(width, height int) { allocated = width == 240 && height == 120 }
	window := gtkWindowNew()
	gObjectRefSink(unsafe.Pointer(window))
	defer func() { gtkWindowDestroy((*gtkWindow)(unsafe.Pointer(window))); gObjectUnref(unsafe.Pointer(window)) }()
	gtkWindowSetChild((*gtkWindow)(unsafe.Pointer(window)), layout)
	gtkWindowPresent((*gtkWindow)(unsafe.Pointer(window)))
	pumpNativeUntil(func() bool { return gtkWidgetGetMapped((*gtkWidget)(button)) != 0 }, time.Second)
	gtkWidgetAllocate(layout, 240, 120, -1, nil)
	var allocation gtkAllocation
	gtkWidgetGetAllocation((*gtkWidget)(button), &allocation)
	if !allocated || allocation.width < 160 {
		return fmt.Errorf("native layout callback = %v, child allocation = %+v", allocated, allocation)
	}
	// Activating GtkWidget is deferred; use its native idle/main-context delivery.
	gtkWidgetActivate((*gtkWidget)(button))
	pumpNativeUntil(func() bool { return len(receiver.events) > 0 }, time.Second)
	if len(receiver.events) != 1 || receiver.events[0] != uiEventClick {
		return fmt.Errorf("native button events = %v", receiver.events)
	}
	gkGTKUIClearAction(button)
	return nil
}
func checkNativeTrayProtocol() error {
	activated := 0
	tray, err := newNativeTray(func() { activated++ })
	if err != nil {
		return err
	}
	defer tray.close()
	tray.configure(ui.IconSource{}, "Port tray", "Native protocol")
	menu := newNativeMenu(0)
	defer gObjectUnref(unsafe.Pointer(menu.model))
	action := ui.Action{ID: "checked", Text: "Checked action", Enabled: true, Checked: true}
	menu.appendAction(menu.model, action)
	menu.items = []ui.MenuItem{{ActionID: action.ID}}
	menu.definitions = map[string]ui.Action{action.ID: action}
	tray.setMenu(menu)
	tray.setMenu(menu) // Reattaching the same model must retain it safely.
	destination := goString(gDBusConnectionGetUniqueName(tray.bus))
	type result struct {
		reply *gVariant
		err   error
	}
	call := func(path, iface, method string, parameters *gVariant) (*gVariant, error) {
		done := make(chan result, 1)
		go func() {
			reply, err := callNativeBus(sessionBus(), destination, path, iface, method, parameters, "", 1500)
			done <- result{reply, err}
		}()
		var outcome result
		received := false
		pumpNativeUntil(func() bool {
			select {
			case outcome = <-done:
				received = true
				return true
			default:
				return false
			}
		}, 2*time.Second)
		if !received {
			outcome = <-done
		}
		return outcome.reply, outcome.err
	}
	reply, err := call(tray.itemPath, "org.freedesktop.DBus.Properties", "Get", variantTuple(variantString("org.kde.StatusNotifierItem"), variantString("Title")))
	if err != nil {
		return err
	}
	boxed := gVariantGetChildValue(reply, 0)
	value := gVariantGetVariant(boxed)
	title := variantStringValue(value)
	gVariantUnref(value)
	gVariantUnref(boxed)
	gVariantUnref(reply)
	if title != "Port tray" {
		return fmt.Errorf("D-Bus tray title = %q", title)
	}
	reply, err = call(tray.menuPath, "com.canonical.dbusmenu", "GetLayout", variantTuple(gVariantNewInt32(0), gVariantNewInt32(-1), variantStrings(nil)))
	if err != nil {
		return err
	}
	root := gVariantGetChildValue(reply, 1)
	children := gVariantGetChildValue(root, 2)
	count := gVariantNChildren(children)
	gVariantUnref(children)
	gVariantUnref(root)
	gVariantUnref(reply)
	if count != 1 {
		return fmt.Errorf("D-Bus menu children = %d", count)
	}
	reply, err = call(tray.itemPath, "org.kde.StatusNotifierItem", "Activate", variantTuple(gVariantNewInt32(0), gVariantNewInt32(0)))
	if err != nil {
		return err
	}
	gVariantUnref(reply)
	if activated != 1 {
		return fmt.Errorf("D-Bus tray activation count = %d", activated)
	}
	reply, err = call(tray.menuPath, "com.canonical.dbusmenu", "Event", variantTuple(gVariantNewInt32(1), variantString("clicked"), gVariantNewVariant(gVariantNewInt32(0)), gVariantNewUint32(0)))
	if err != nil {
		return err
	}
	gVariantUnref(reply)
	native := gActionMapLookupAction((*gActionMap)(unsafe.Pointer(menu.actions)), stringPointer(menu.names[action.ID]))
	checked := gActionGetState(native)
	defer gVariantUnref(checked)
	if gVariantGetBoolean(checked) != 0 {
		return fmt.Errorf("D-Bus menu click did not toggle native action")
	}
	return nil
}
