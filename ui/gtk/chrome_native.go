//go:build linux && cgo

package gtk

import (
	"errors"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

type nativeMenuBar struct {
	window     *Window
	model      *gMenu
	widget     *gtkWidget
	controller *gtkEventController
	menus      []*nativeMenu
}

// Native windows keep chrome above and below the content layout in one box.
func windowChrome(window *gtkWindow, top bool) *gtkBox {
	overlay := gtkWindowGetChild(window)
	column := gtkOverlayGetChild((*gtkOverlay)(unsafe.Pointer(overlay)))
	if top {
		return (*gtkBox)(unsafe.Pointer(gtkWidgetGetFirstChild(column)))
	}
	return (*gtkBox)(unsafe.Pointer(gtkWidgetGetLastChild(column)))
}

func newNativeMenuBar(window *Window, menus []ui.NamedMenu) (*nativeMenuBar, error) {
	bar := &nativeMenuBar{window: window}
	for _, named := range menus {
		owned, err := ownedMenu(named.Menu)
		if err != nil {
			return nil, err
		}
		record := menuRecord(owned.nativeMenu())
		if record == nil {
			return nil, ui.ErrClosed
		}
		if record.bar != nil {
			return nil, errors.New("ui/gtk: a menu in the menu bar already belongs to another menu")
		}
		for _, previous := range bar.menus {
			if previous == record {
				return nil, errors.New("ui/gtk: menu bar contains a duplicate menu")
			}
		}
		bar.menus = append(bar.menus, record)
	}
	bar.model = gMenuNew()
	models := gListStoreNew(gListModelGetType())
	nativeWindow := window.nativeGTKWindow()
	for i, menu := range bar.menus {
		gMenuAppendSubmenu(bar.model, stringPointer(menus[i].Title), (*gMenuModel)(unsafe.Pointer(menu.model)))
		menu.bar = bar
		gtkWidgetInsertActionGroup((*gtkWidget)(unsafe.Pointer(nativeWindow)), stringPointer(menu.prefix), (*gActionGroup)(unsafe.Pointer(menu.actions)))
		gListStoreAppend(models, unsafe.Pointer(menu.shortcuts))
	}
	bar.widget = gtkPopoverMenuBarNewFromModel((*gMenuModel)(unsafe.Pointer(bar.model)))
	gObjectRefSink(unsafe.Pointer(bar.widget))
	top := windowChrome(nativeWindow, true)
	gtkBoxAppend(top, bar.widget)
	gtkBoxReorderChildAfter(top, bar.widget, nil)
	flattened := gtkFlattenListModelNew((*gListModel)(unsafe.Pointer(models)))
	bar.controller = gtkShortcutControllerNewForModel((*gListModel)(unsafe.Pointer(flattened)))
	gObjectRef(unsafe.Pointer(bar.controller))
	gtkShortcutControllerSetScope((*gtkShortcutController)(unsafe.Pointer(bar.controller)), gtkShortcutScopeGlobal)
	gtkWidgetAddController((*gtkWidget)(unsafe.Pointer(nativeWindow)), bar.controller)
	return bar, nil
}

func (bar *nativeMenuBar) close() {
	if window := bar.window.nativeGTKWindow(); window != nil {
		gtkWidgetRemoveController((*gtkWidget)(unsafe.Pointer(window)), bar.controller)
		for _, menu := range bar.menus {
			gtkWidgetInsertActionGroup((*gtkWidget)(unsafe.Pointer(window)), stringPointer(menu.prefix), nil)
		}
	}
	if parent := gtkWidgetGetParent(bar.widget); parent != nil {
		gtkBoxRemove((*gtkBox)(unsafe.Pointer(parent)), bar.widget)
	}
	for _, menu := range bar.menus {
		menu.bar = nil
	}
	gObjectUnref(unsafe.Pointer(bar.controller))
	gObjectUnref(unsafe.Pointer(bar.widget))
	gObjectUnref(unsafe.Pointer(bar.model))
}

type nativeToolbar struct {
	widget  *gtkWidget
	buttons map[string]*gtkWidget
}

func newNativeToolbar(window *Window, options ui.ToolbarOptions, activate func(string)) (*nativeToolbar, error) {
	actions := make(map[string]ui.Action, len(options.Actions))
	for _, action := range options.Actions {
		actions[action.ID] = action
	}
	for _, item := range options.Items {
		if !item.Separator && !item.Space && !item.FlexibleSpace {
			if _, ok := actions[item.ActionID]; !ok {
				return nil, errors.New("ui: toolbar item names an unknown action " + item.ActionID)
			}
		}
	}
	bar := &nativeToolbar{widget: gtkBoxNew(gtkOrientationHorizontal, 4), buttons: make(map[string]*gtkWidget)}
	gObjectRefSink(unsafe.Pointer(bar.widget))
	gtkWidgetAddCSSClass(bar.widget, stringPointer("toolbar"))
	for _, item := range options.Items {
		var child *gtkWidget
		switch {
		case item.Separator:
			child = gtkSeparatorNew(gtkOrientationVertical)
		case item.Space:
			child = gtkBoxNew(gtkOrientationHorizontal, 0)
			gtkWidgetSetSizeRequest(child, 12, -1)
		case item.FlexibleSpace:
			child = gtkBoxNew(gtkOrientationHorizontal, 0)
			gtkWidgetSetHexpand(child, 1)
		default:
			action := actions[item.ActionID]
			child = gtkButtonNew()
			gtkWidgetAddCSSClass(child, stringPointer("flat"))
			content := gtkBoxNew(gtkOrientationVertical, 2)
			icon := action.Icon
			if icon.IsZero() {
				icon = ui.IconSource{SystemName: "circle"}
			}
			if paintable := nativeIcon(icon, 16); paintable != nil {
				image := gtkImageNewFromPaintable(paintable)
				gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(image)), 16)
				gtkBoxAppend((*gtkBox)(unsafe.Pointer(content)), image)
				gObjectUnref(unsafe.Pointer(paintable))
			}
			if options.ShowsLabels {
				label := gtkLabelNew(stringPointer(action.Text))
				gtkWidgetAddCSSClass(label, stringPointer("caption"))
				gtkBoxAppend((*gtkBox)(unsafe.Pointer(content)), label)
			}
			gtkButtonSetChild((*gtkButton)(unsafe.Pointer(child)), content)
			gtkWidgetSetTooltipText(child, stringPointer(action.Text))
			gtkWidgetSetSensitive(child, nativeBool(action.Enabled))
			connectNativeSignal(unsafe.Pointer(child), "clicked", func() { activate(action.ID) })
			bar.buttons[action.ID] = child
		}
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(bar.widget)), child)
	}
	gtkBoxAppend(windowChrome(window.nativeGTKWindow(), true), bar.widget)
	return bar, nil
}

func (bar *nativeToolbar) close() {
	if parent := gtkWidgetGetParent(bar.widget); parent != nil {
		gtkBoxRemove((*gtkBox)(unsafe.Pointer(parent)), bar.widget)
	}
	gObjectUnref(unsafe.Pointer(bar.widget))
}

type nativeCommandBar struct {
	widget, overflow, list *gtkWidget
	buttons, items         []*gtkWidget
	width, height          int
}

func (bar *nativeCommandBar) preferredNative() (int, int) { return bar.width, bar.height }
func gkGTKStatusBarCreate(window cUintptr, text *cChar) unsafe.Pointer {
	bar := boxWithRole(gtkOrientationHorizontal, 0, gtkAccessibleRoleStatus)
	gtkWidgetAddCSSClass(bar, stringPointer("gk-status-bar"))
	setComponentCSS("box.gk-status-bar", "border-top: 1px solid alpha(currentColor, 0.15); padding: 2px 8px; min-height: 18px;")
	label := gtkLabelNew(text)
	gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(label)), 0)
	gtkLabelSetEllipsize((*gtkLabel)(unsafe.Pointer(label)), pangoEllipsizeEnd)
	gtkWidgetSetHexpand(label, 1)
	gtkWidgetAddCSSClass(label, stringPointer("dim-label"))
	gtkWidgetAddCSSClass(label, stringPointer("caption"))
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(bar)), label)
	registerNativeComponent(uintptr(window), bar, label, kindStatusBar)
	return unsafe.Pointer(bar)
}
func gkGTKStatusBarSet(pointer unsafe.Pointer, text *cChar) {
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(componentControl((*gtkWidget)(pointer)))), text)
}
func gkGTKCommandBarCreate(window cUintptr) unsafe.Pointer {
	bar := &nativeCommandBar{widget: newComponentLayoutWithRole(gtkAccessibleRoleToolbar), overflow: gtkMenuButtonNew(), list: gtkBoxNew(gtkOrientationVertical, 0)}
	gtkMenuButtonSetIconName((*gtkMenuButton)(unsafe.Pointer(bar.overflow)), stringPointer("view-more-symbolic"))
	gtkWidgetAddCSSClass(bar.overflow, stringPointer("flat"))
	setAccessibleText(bar.overflow, gtkAccessiblePropertyLabel, "More")
	popover := gtkPopoverNew()
	gtkPopoverSetChild((*gtkPopover)(unsafe.Pointer(popover)), bar.list)
	gtkMenuButtonSetPopover((*gtkMenuButton)(unsafe.Pointer(bar.overflow)), popover)
	gtkWidgetSetParent(bar.overflow, bar.widget)
	moveComponent(bar.overflow, 0, 0, 0, 0)
	registerNativeComponent(uintptr(window), bar.widget, nil, kindCommandBar)
	storeComponentState(bar.widget, bar)
	onComponentAllocate(bar.widget, bar.arrange)
	return unsafe.Pointer(bar.widget)
}
func (bar *nativeCommandBar) arrange(width, height int) {
	overflowWidth, overflowHeight := preferredWidgetSize(bar.overflow)
	x := 0
	overflowing := false
	for i, button := range bar.buttons {
		w, h := preferredWidgetSize(button)
		reserve := 0
		if i < len(bar.buttons)-1 {
			reserve = overflowWidth + 4
		}
		if overflowing || x+w+reserve > width {
			overflowing = true
			moveComponent(button, 0, 0, 0, 0)
			gtkWidgetSetVisible(bar.items[i], 1)
			continue
		}
		gtkWidgetSetVisible(bar.items[i], 0)
		moveComponent(button, x, (height-h)/2, w, h)
		x += w + 4
	}
	if overflowing {
		moveComponent(bar.overflow, min(x, width-overflowWidth), (height-overflowHeight)/2, overflowWidth, overflowHeight)
	} else {
		moveComponent(bar.overflow, 0, 0, 0, 0)
	}
}
func gkGTKCommandBarSetActions(pointer unsafe.Pointer, ids *cChar, idsSize cSize, titles *cChar, titlesSize cSize, icons *unsafe.Pointer, enabled *cInt, count, compact cInt) {
	bar := componentState[nativeCommandBar](pointer)
	if bar == nil {
		return
	}
	for _, button := range bar.buttons {
		gtkWidgetUnparent(button)
	}
	for _, item := range bar.items {
		gtkBoxRemove((*gtkBox)(unsafe.Pointer(bar.list)), item)
	}
	bar.buttons, bar.items = nil, nil
	identifiers, names := splitNul(unsafe.Pointer(ids), idsSize), splitNul(unsafe.Pointer(titles), titlesSize)
	paintables := unsafe.Slice((**gdkPaintable)(unsafe.Pointer(icons)), int(count))
	flags := unsafe.Slice(enabled, int(count))
	bar.width, bar.height = 0, 0
	for i := 0; i < int(count); i++ {
		identifier, title := identifiers[i], names[i]
		button := func(flat bool) *gtkWidget {
			button := gtkButtonNew()
			if flat {
				gtkWidgetAddCSSClass(button, stringPointer("flat"))
			}
			row := gtkBoxNew(gtkOrientationHorizontal, 6)
			if paintables[i] != nil {
				image := gtkImageNewFromPaintable(paintables[i])
				gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(image)), 16)
				gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), image)
			}
			if compact == 0 || paintables[i] == nil || flat {
				label := gtkLabelNew(stringPointer(title))
				gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(label)), 0)
				gtkBoxAppend((*gtkBox)(unsafe.Pointer(row)), label)
			}
			gtkButtonSetChild((*gtkButton)(unsafe.Pointer(button)), row)
			gtkWidgetSetTooltipText(button, stringPointer(title))
			setAccessibleText(button, gtkAccessiblePropertyLabel, title)
			gtkWidgetSetSensitive(button, nativeBool(flags[i] != 0))
			connectNativeSignal(unsafe.Pointer(button), "clicked", func() {
				if flat {
					gtkMenuButtonPopdown((*gtkMenuButton)(unsafe.Pointer(bar.overflow)))
				}
				sendComponentEvent(bar.widget, gtkEvent{kind: gkEventAction, text: identifier})
			})
			return button
		}
		direct, item := button(false), button(true)
		gtkWidgetInsertBefore(direct, bar.widget, bar.overflow)
		moveComponent(direct, 0, 0, 0, 0)
		bar.buttons = append(bar.buttons, direct)
		gtkWidgetSetVisible(item, 0)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(bar.list)), item)
		bar.items = append(bar.items, item)
		w, h := preferredWidgetSize(direct)
		bar.width += w + 4
		bar.height = max(bar.height, h)
	}
	bar.width = max(0, bar.width-4)
	gtkWidgetQueueAllocate(bar.widget)
}
