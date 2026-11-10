//go:build linux && cgo

package gtk

import (
	"fmt"
	"runtime/cgo"
	"sync/atomic"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

var menuSerial atomic.Uint64
var menuDataKey = gQuarkFromString(stringPointer("gamekit-menu"))
var contextMenuDataKey = gQuarkFromString(stringPointer("gamekit-context-menu"))

type nativeMenu struct {
	items       []ui.MenuItem
	definitions map[string]ui.Action
	changes     listeners[func()]
	bar         *nativeMenuBar
	model       *gMenu
	actions     *gSimpleActionGroup
	shortcuts   *gListStore
	prefix      string
	handle      uintptr
	next        int
	names       map[string]string
	popover     *gtkWidget
}

func newNativeMenu(handle uintptr) *nativeMenu {
	menu := &nativeMenu{
		model: gMenuNew(), actions: gSimpleActionGroupNew(),
		shortcuts: gListStoreNew(gtkShortcutGetType()), names: make(map[string]string),
		prefix: fmt.Sprintf("gkm%d", menuSerial.Add(1)), handle: handle,
	}
	gObjectSetQdataFull((*gObject)(unsafe.Pointer(menu.model)), menuDataKey, newNativeCallback(menu), gtkDestroyCallback())
	return menu
}

func menuRecord(model unsafe.Pointer) *nativeMenu {
	if model == nil {
		return nil
	}
	data := gObjectGetQdata((*gObject)(model), menuDataKey)
	if data == nil {
		return nil
	}
	return nativeCallback(data).(*nativeMenu)
}

func (menu *nativeMenu) releaseNative() {
	if menu.popover != nil {
		if gtkWidgetGetParent(menu.popover) != nil {
			gtkWidgetUnparent(menu.popover)
		}
		gObjectUnref(unsafe.Pointer(menu.popover))
	}
	gObjectUnref(unsafe.Pointer(menu.actions))
	gObjectUnref(unsafe.Pointer(menu.shortcuts))
}

func (menu *nativeMenu) clear() {
	gMenuRemoveAll(menu.model)
	for _, name := range menu.names {
		gActionMapRemoveAction((*gActionMap)(unsafe.Pointer(menu.actions)), stringPointer(name))
	}
	clear(menu.names)
	gListStoreRemoveAll(menu.shortcuts)
}

func (menu *nativeMenu) addAction(name, identifier string, checked bool) *gSimpleAction {
	var action *gSimpleAction
	if checked {
		action = gSimpleActionNewStateful(stringPointer(name), nil, gVariantNewBoolean(1))
	} else {
		action = gSimpleActionNew(stringPointer(name), nil)
	}
	gSignalConnectData(unsafe.Pointer(action), stringPointer("activate"), gtkActionCallback(), newNativeCallback(func(_ *gVariant) {
		if state := gActionGetState((*gAction)(unsafe.Pointer(action))); state != nil {
			checked := gBoolean(1)
			if gVariantGetBoolean(state) != 0 {
				checked = 0
			}
			gSimpleActionSetState(action, gVariantNewBoolean(checked))
			gVariantUnref(state)
			for _, changed := range menu.changes.snapshot() {
				changed()
			}
		}
		if menu.handle != 0 {
			if receiver, ok := cgo.Handle(menu.handle).Value().(gtkReceiver); ok {
				receiver.gtkEvent(gtkEvent{kind: gkEventAction, text: identifier})
			}
		}
	}), gtkCallbackFree(), 0)
	gActionMapAddAction((*gActionMap)(unsafe.Pointer(menu.actions)), (*gAction)(unsafe.Pointer(action)))
	gObjectUnref(unsafe.Pointer(action))
	return action
}

func (menu *nativeMenu) appendAction(model *gMenu, action ui.Action) {
	name, exists := menu.names[action.ID]
	if !exists {
		menu.next++
		name = fmt.Sprintf("a%d", menu.next)
		menu.names[action.ID] = name
	}
	native := (*gSimpleAction)(unsafe.Pointer(gActionMapLookupAction((*gActionMap)(unsafe.Pointer(menu.actions)), stringPointer(name))))
	if native == nil || action.Checked && gActionGetStateType((*gAction)(unsafe.Pointer(native))) == nil {
		native = menu.addAction(name, action.ID, action.Checked)
	}
	gSimpleActionSetEnabled(native, nativeBool(action.Enabled))
	detailed := menu.prefix + "." + name
	item := gMenuItemNew(stringPointer(action.Text), stringPointer(detailed))
	defer gObjectUnref(unsafe.Pointer(item))
	if key, _ := keyEquivalent(action.Shortcut); key != "" {
		gMenuItemSetAttributeValue(item, stringPointer("accel"), variantString(key))
		if trigger := gtkShortcutTriggerParseString(stringPointer(key)); trigger != nil {
			shortcut := gtkShortcutNew(trigger, gtkNamedActionNew(stringPointer(detailed)))
			gListStoreAppend(menu.shortcuts, unsafe.Pointer(shortcut))
			gObjectUnref(unsafe.Pointer(shortcut))
		}
	}
	if paintable := nativeIcon(action.Icon, 16); paintable != nil {
		if action.Icon.Image != nil && !action.Icon.Image.Bounds().Empty() {
			gMenuItemSetIcon(item, (*gIcon)(unsafe.Pointer(paintable)))
		} else if name := gtkIconPaintableGetIconName((*gtkIconPaintable)(paintable)); name != nil {
			icon := gThemedIconNew(name)
			gMenuItemSetIcon(item, icon)
			gObjectUnref(unsafe.Pointer(icon))
		}
		gObjectUnref(unsafe.Pointer(paintable))
	}
	gMenuAppendItem(model, item)
}

func (menu *nativeMenu) setActionState(identifier string, enabled, checked int) {
	name, exists := menu.names[identifier]
	if !exists {
		return
	}
	action := (*gSimpleAction)(unsafe.Pointer(gActionMapLookupAction((*gActionMap)(unsafe.Pointer(menu.actions)), stringPointer(name))))
	if action == nil {
		return
	}
	if enabled >= 0 {
		gSimpleActionSetEnabled(action, nativeBool(enabled != 0))
	}
	stateful := gActionGetStateType((*gAction)(unsafe.Pointer(action))) != nil
	if checked > 0 && !stateful {
		wasEnabled := gActionGetEnabled((*gAction)(unsafe.Pointer(action)))
		action = menu.addAction(name, identifier, true)
		gSimpleActionSetEnabled(action, wasEnabled)
	} else if checked >= 0 && stateful {
		gSimpleActionSetState(action, gVariantNewBoolean(nativeBool(checked != 0)))
	}
}

func (menu *nativeMenu) popup(widget *gtkWidget, position ui.Point) {
	if menu.popover != nil && gtkWidgetGetParent(menu.popover) != widget {
		gtkWidgetUnparent(menu.popover)
		gObjectUnref(unsafe.Pointer(menu.popover))
		menu.popover = nil
	}
	if menu.popover == nil {
		menu.popover = gtkPopoverMenuNewFromModel((*gMenuModel)(unsafe.Pointer(menu.model)))
		gObjectRefSink(unsafe.Pointer(menu.popover))
		gtkPopoverSetHasArrow((*gtkPopover)(unsafe.Pointer(menu.popover)), 0)
		gtkWidgetSetHalign(menu.popover, gtkAlignStart)
		gtkWidgetSetParent(menu.popover, widget)
	}
	gtkWidgetInsertActionGroup(widget, stringPointer(menu.prefix), (*gActionGroup)(unsafe.Pointer(menu.actions)))
	rect := gdkRectangle{x: cInt(position.X), y: cInt(position.Y), width: 1, height: 1}
	gtkPopoverSetPointingTo((*gtkPopover)(unsafe.Pointer(menu.popover)), &rect)
	gtkPopoverPopup((*gtkPopover)(unsafe.Pointer(menu.popover)))
}

type nativeContextMenu struct{ model *gMenu }

func (context *nativeContextMenu) releaseNative() {
	if context.model != nil {
		gObjectUnref(unsafe.Pointer(context.model))
	}
}

func attachNativeContextMenu(widget *gtkWidget, model unsafe.Pointer) {
	object := (*gObject)(unsafe.Pointer(widget))
	data := gObjectGetQdata(object, contextMenuDataKey)
	var context *nativeContextMenu
	if data == nil {
		context = &nativeContextMenu{}
		gObjectSetQdataFull(object, contextMenuDataKey, newNativeCallback(context), gtkDestroyCallback())
		gesture := gtkGestureClickNew()
		gtkGestureSingleSetButton((*gtkGestureSingle)(unsafe.Pointer(gesture)), 3)
		gtkEventControllerSetPropagationPhase((*gtkEventController)(unsafe.Pointer(gesture)), gtkPhaseCapture)
		gSignalConnectData(unsafe.Pointer(gesture), stringPointer("pressed"), gtkPressedCallback(), newNativeCallback(func(x, y float64) {
			if menu := menuRecord(unsafe.Pointer(context.model)); menu != nil {
				gtkGestureSetState(gesture, gtkEventSequenceClaimed)
				menu.popup(widget, ui.Point{X: int(x), Y: int(y)})
			}
		}), gtkCallbackFree(), 0)
		gtkWidgetAddController(widget, (*gtkEventController)(unsafe.Pointer(gesture)))
	} else {
		context = nativeCallback(data).(*nativeContextMenu)
	}
	if model != nil {
		gObjectRef(model)
	}
	if context.model != nil {
		gObjectUnref(unsafe.Pointer(context.model))
	}
	context.model = (*gMenu)(model)
}

func nativeBool(value bool) gBoolean {
	if value {
		return 1
	}
	return 0
}
