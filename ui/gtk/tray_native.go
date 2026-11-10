//go:build linux && cgo

package gtk

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

var traySerial atomic.Uint64

type trayMenuEntry struct {
	action     string
	properties map[string]any
	children   []int
}

type nativeTray struct {
	bus                      *gDBusConnection
	itemPath, menuPath       string
	item, menu               *nativeBusObject
	watch                    cUint
	activate                 func()
	title, tooltip, iconName string
	pixmap                   *gVariant
	visible, attention       bool
	model                    *nativeMenu
	unsubscribe              ui.Unsubscribe
	revision                 uint32
	entries                  []trayMenuEntry
}

func newNativeTray(activate func()) (*nativeTray, error) {
	address := gDBusAddressGetForBusSync(gBusTypeSession, nil, nil)
	if address == nil {
		return nil, errNotBundled
	}
	bus := gDBusConnectionNewForAddressSync(address, gDBusConnectionFlagsAuthenticationClient|gDBusConnectionFlagsMessageBusConnection, nil, nil, nil)
	gFree(unsafe.Pointer(address))
	if bus == nil {
		return nil, errNotBundled
	}
	gDBusConnectionSetExitOnClose(bus, 0)
	serial := traySerial.Add(1)
	tray := &nativeTray{bus: bus, visible: true, activate: activate,
		itemPath: fmt.Sprintf("/StatusNotifierItem/gamekit%d", serial), menuPath: fmt.Sprintf("/MenuBar/gamekit%d", serial)}
	var err error
	tray.item, err = registerNativeBusObject(bus, tray.itemPath, trayItemXML, &nativeBusMethods{
		method: func(method string, _ *gVariant, invocation *gDBusMethodInvocation) {
			if method == "Activate" || method == "SecondaryActivate" {
				tray.activate()
			}
			gDBusMethodInvocationReturnValue(invocation, nil)
		}, property: tray.itemProperty,
	})
	if err == nil {
		tray.menu, err = registerNativeBusObject(bus, tray.menuPath, trayMenuXML, &nativeBusMethods{method: tray.menuCall, property: tray.menuProperty})
	}
	if err != nil {
		tray.close()
		return nil, err
	}
	tray.rebuild()
	tray.watch = gBusWatchNameOnConnection(bus, stringPointer("org.kde.StatusNotifierWatcher"), 0,
		gtkBusAppearedCallback(), gtkBusVanishedCallback(), newNativeCallback(&nativeBusWatch{
			appeared: func() {
				reply, _ := callNativeBus(bus, "org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher", "org.kde.StatusNotifierWatcher",
					"RegisterStatusNotifierItem", variantTuple(variantString(tray.itemPath)), "", 2000)
				if reply != nil {
					gVariantUnref(reply)
				}
			}, vanished: func() {},
		}), gtkDestroyCallback())
	return tray, nil
}

func (tray *nativeTray) close() {
	tray.setMenu(nil)
	if tray.watch != 0 {
		gBusUnwatchName(tray.watch)
	}
	if tray.item != nil {
		tray.item.close()
	}
	if tray.menu != nil {
		tray.menu.close()
	}
	if tray.pixmap != nil {
		gVariantUnref(tray.pixmap)
	}
	gDBusConnectionCloseSync(tray.bus, nil, nil)
	gObjectUnref(unsafe.Pointer(tray.bus))
}

func (tray *nativeTray) configure(icon ui.IconSource, title, tooltip string) {
	tray.title, tray.tooltip, tray.iconName = title, tooltip, ""
	if tray.pixmap != nil {
		gVariantUnref(tray.pixmap)
		tray.pixmap = nil
	}
	if icon.Image != nil && !icon.Image.Bounds().Empty() {
		pixels, width, height := rgbaPixels(icon.Image)
		for i := 0; i < len(pixels); i += 4 {
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = pixels[i+3], pixels[i], pixels[i+1], pixels[i+2]
		}
		tray.pixmap = gVariantRefSink(variantArray("(iiay)", []*gVariant{variantTuple(gVariantNewInt32(cInt(width)), gVariantNewInt32(cInt(height)), variantBytes(pixels))}))
	} else if paintable := nativeIcon(icon, 15); paintable != nil {
		tray.iconName = goString(gtkIconPaintableGetIconName((*gtkIconPaintable)(paintable)))
		gObjectUnref(unsafe.Pointer(paintable))
	}
	for _, signal := range []string{"NewTitle", "NewIcon", "NewToolTip"} {
		tray.signal(signal, nil)
	}
}

func (tray *nativeTray) signal(signal string, parameters *gVariant) {
	gDBusConnectionEmitSignal(tray.bus, nil, stringPointer(tray.itemPath), stringPointer("org.kde.StatusNotifierItem"), stringPointer(signal), parameters, nil)
}
func (tray *nativeTray) status() string {
	if !tray.visible {
		return "Passive"
	}
	if tray.attention {
		return "NeedsAttention"
	}
	return "Active"
}
func (tray *nativeTray) setVisible(visible bool) {
	tray.visible = visible
	tray.signal("NewStatus", variantTuple(variantString(tray.status())))
}
func (tray *nativeTray) pixmaps() *gVariant {
	if tray.pixmap != nil {
		return gVariantRef(tray.pixmap)
	}
	return variantArray("(iiay)", nil)
}
func (tray *nativeTray) itemProperty(name string) *gVariant {
	switch name {
	case "Category":
		return variantString("ApplicationStatus")
	case "Id":
		name := goString(gGetPrgname())
		if name == "" {
			name = "gamekit"
		}
		return variantString(name)
	case "Title":
		return variantString(tray.title)
	case "Status":
		return variantString(tray.status())
	case "WindowId":
		return gVariantNewInt32(0)
	case "IconName", "AttentionIconName":
		return variantString(tray.iconName)
	case "IconPixmap":
		return tray.pixmaps()
	case "ToolTip":
		return variantTuple(variantString(tray.iconName), variantArray("(iiay)", nil), variantString(tray.tooltip), variantString(""))
	case "ItemIsMenu":
		return gVariantNewBoolean(0)
	case "Menu":
		return gVariantNewObjectPath(stringPointer(tray.menuPath))
	}
	return nil
}
func (tray *nativeTray) menuProperty(name string) *gVariant {
	switch name {
	case "Version":
		return gVariantNewUint32(3)
	case "TextDirection":
		if gtkWidgetGetDefaultDirection() == gtkTextDirRTL {
			return variantString("rtl")
		}
		return variantString("ltr")
	case "Status":
		return variantString("normal")
	default:
		return variantStrings(nil)
	}
}

func (tray *nativeTray) setMenu(model *nativeMenu) {
	if model != nil {
		gObjectRef(unsafe.Pointer(model.model))
	}
	if tray.unsubscribe != nil {
		tray.unsubscribe()
		tray.unsubscribe = nil
	}
	if tray.model != nil {
		gObjectUnref(unsafe.Pointer(tray.model.model))
	}
	tray.model = model
	if model != nil {
		tray.unsubscribe = model.changes.add(tray.changed)
	}
	tray.changed()
}
func (tray *nativeTray) changed() {
	tray.revision++
	tray.rebuild()
	gDBusConnectionEmitSignal(tray.bus, nil, stringPointer(tray.menuPath), stringPointer("com.canonical.dbusmenu"), stringPointer("LayoutUpdated"),
		variantTuple(gVariantNewUint32(cUint32(tray.revision)), gVariantNewInt32(0)), nil)
}
func (tray *nativeTray) rebuild() {
	tray.entries = []trayMenuEntry{{properties: map[string]any{"children-display": "submenu"}}}
	if tray.model == nil {
		return
	}
	var build func([]ui.MenuItem, int)
	build = func(items []ui.MenuItem, parent int) {
		for _, item := range items {
			entry := trayMenuEntry{action: item.ActionID, properties: make(map[string]any)}
			if item.Separator {
				entry.properties["type"] = "separator"
			} else if item.ActionID == "" {
				entry.properties["label"] = strings.ReplaceAll(item.Title, "_", "__")
				entry.properties["children-display"] = "submenu"
			} else {
				action := tray.model.definitions[item.ActionID]
				entry.properties["label"] = strings.ReplaceAll(action.Text, "_", "__")
				name := tray.model.names[item.ActionID]
				native := gActionMapLookupAction((*gActionMap)(unsafe.Pointer(tray.model.actions)), stringPointer(name))
				if native != nil {
					entry.properties["enabled"] = gActionGetEnabled(native) != 0
					if state := gActionGetState(native); state != nil {
						entry.properties["toggle-type"] = "checkmark"
						checked := 0
						if gVariantGetBoolean(state) != 0 {
							checked = 1
						}
						entry.properties["toggle-state"] = checked
						gVariantUnref(state)
					}
				}
			}
			id := len(tray.entries)
			tray.entries = append(tray.entries, entry)
			tray.entries[parent].children = append(tray.entries[parent].children, id)
			if len(item.Items) > 0 {
				build(item.Items, id)
			}
		}
	}
	build(tray.model.items, 0)
}
func trayProperties(properties map[string]any) *gVariant {
	values := make(map[string]*gVariant, len(properties))
	for name, value := range properties {
		switch value := value.(type) {
		case string:
			values[name] = variantString(value)
		case bool:
			values[name] = gVariantNewBoolean(nativeBool(value))
		case int:
			values[name] = gVariantNewInt32(cInt(value))
		}
	}
	return variantDictionary(values)
}
func (tray *nativeTray) layout(id, depth int) *gVariant {
	entry := tray.entries[id]
	var children []*gVariant
	if depth != 0 {
		for _, child := range entry.children {
			children = append(children, gVariantNewVariant(tray.layout(child, max(-1, depth-1))))
		}
	}
	return variantTuple(gVariantNewInt32(cInt(id)), trayProperties(entry.properties), variantArray("v", children))
}
func (tray *nativeTray) activateEntry(id int, kind string) bool {
	if id <= 0 || id >= len(tray.entries) {
		return false
	}
	entry := tray.entries[id]
	if kind == "clicked" && entry.action != "" && tray.model != nil {
		name := tray.model.names[entry.action]
		gActionGroupActivateAction((*gActionGroup)(unsafe.Pointer(tray.model.actions)), stringPointer(name), nil)
	}
	return true
}
func (tray *nativeTray) menuCall(method string, parameters *gVariant, invocation *gDBusMethodInvocation) {
	var reply *gVariant
	switch method {
	case "GetLayout":
		id, depth := variantChildInt(parameters, 0), variantChildInt(parameters, 1)
		if id < 0 || id >= len(tray.entries) {
			id = 0
		}
		reply = variantTuple(gVariantNewUint32(cUint32(tray.revision)), tray.layout(id, depth))
	case "GetGroupProperties":
		ids := gVariantGetChildValue(parameters, 0)
		var properties []*gVariant
		for i := gSize(0); i < gVariantNChildren(ids); i++ {
			id := variantChildInt(ids, int(i))
			if id >= 0 && id < len(tray.entries) {
				properties = append(properties, variantTuple(gVariantNewInt32(cInt(id)), trayProperties(tray.entries[id].properties)))
			}
		}
		gVariantUnref(ids)
		reply = variantTuple(variantArray("(ia{sv})", properties))
	case "GetProperty":
		id, name := variantChildInt(parameters, 0), variantChildString(parameters, 1)
		value := any("")
		if id >= 0 && id < len(tray.entries) {
			if property, ok := tray.entries[id].properties[name]; ok {
				value = property
			}
		}
		var native *gVariant
		switch value := value.(type) {
		case string:
			native = variantString(value)
		case bool:
			native = gVariantNewBoolean(nativeBool(value))
		case int:
			native = gVariantNewInt32(cInt(value))
		}
		reply = variantTuple(gVariantNewVariant(native))
	case "Event":
		tray.activateEntry(variantChildInt(parameters, 0), variantChildString(parameters, 1))
	case "EventGroup":
		events := gVariantGetChildValue(parameters, 0)
		var failures []*gVariant
		for i := gSize(0); i < gVariantNChildren(events); i++ {
			event := gVariantGetChildValue(events, i)
			id := variantChildInt(event, 0)
			if !tray.activateEntry(id, variantChildString(event, 1)) {
				failures = append(failures, gVariantNewInt32(cInt(id)))
			}
			gVariantUnref(event)
		}
		gVariantUnref(events)
		reply = variantTuple(variantArray("i", failures))
	case "AboutToShow":
		reply = variantTuple(gVariantNewBoolean(0))
	case "AboutToShowGroup":
		reply = variantTuple(variantArray("i", nil), variantArray("i", nil))
	default:
		gDBusMethodInvocationReturnDbusError(invocation, stringPointer("org.freedesktop.DBus.Error.UnknownMethod"), stringPointer("Unknown method "+method))
		return
	}
	gDBusMethodInvocationReturnValue(invocation, reply)
}
