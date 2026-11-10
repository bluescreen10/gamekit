//go:build linux && cgo

package gtk

import (
	"fmt"
	"slices"
	"strconv"
	"unsafe"
)

type nativeDataRow struct {
	object                        *gObject
	id, detail                    string
	cells                         []string
	icon                          *gdkPaintable
	children                      *gListStore
	parent                        *nativeDataRow
	hasChildren, group, requested bool
}

func (row *nativeDataRow) releaseNative() {
	if row.icon != nil {
		gObjectUnref(unsafe.Pointer(row.icon))
	}
	if row.children != nil {
		gObjectUnref(unsafe.Pointer(row.children))
	}
}
func newDataRow(id string, icon *gdkPaintable) *nativeDataRow {
	row := &nativeDataRow{object: gObjectNewWithProperties(gObjectGetType(), 0, nil, nil), id: id, icon: icon}
	if icon != nil {
		gObjectRef(unsafe.Pointer(icon))
	}
	gObjectSetQdataFull(row.object, componentStateKey, newNativeCallback(row), gtkDestroyCallback())
	return row
}
func dataRow(object unsafe.Pointer) *nativeDataRow { return componentState[nativeDataRow](object) }

type nativeDataView struct {
	widget, list        *gtkWidget
	rows                *gListStore
	selection           *gtkSelectionModel
	tree                *gtkTreeListModel
	mode, selectionMode int
	columns             []string
	nodes               map[string]*nativeDataRow
	suppress            bool
}

func (view *nativeDataView) releaseNative() {
	gObjectUnref(unsafe.Pointer(view.selection))
	if view.tree != nil {
		gObjectUnref(unsafe.Pointer(view.tree))
	}
	gObjectUnref(unsafe.Pointer(view.rows))
}
func newDataSelection(model *gListModel, mode int) *gtkSelectionModel {
	gObjectRef(unsafe.Pointer(model))
	switch mode {
	case 0:
		return (*gtkSelectionModel)(unsafe.Pointer(gtkNoSelectionNew(model)))
	case 2:
		return (*gtkSelectionModel)(unsafe.Pointer(gtkMultiSelectionNew(model)))
	default:
		single := gtkSingleSelectionNew(model)
		gtkSingleSelectionSetAutoselect(single, 0)
		gtkSingleSelectionSetCanUnselect(single, 1)
		gtkSingleSelectionSetSelected(single, ^cUint(0))
		return (*gtkSelectionModel)(unsafe.Pointer(single))
	}
}
func (view *nativeDataView) report(event gtkEvent) {
	if !view.suppress {
		sendComponentEvent(view.widget, event)
	}
}
func connectNativeArgument(instance unsafe.Pointer, signal string, callback gCallback, value any) {
	gSignalConnectData(instance, stringPointer(signal), callback, newNativeCallback(value), gtkCallbackFree(), 0)
}
func newDataFactory(setup, bind func(*gtkListItem)) *gtkListItemFactory {
	factory := gtkSignalListItemFactoryNew()
	connectNativeArgument(unsafe.Pointer(factory), "setup", gtkListItemCallback(), setup)
	connectNativeArgument(unsafe.Pointer(factory), "bind", gtkListItemCallback(), bind)
	return factory
}
func nativeEqualSort() cInt { return 0 }
func (view *nativeDataView) finish(window uintptr, control *gtkWidget, kind int, framed bool) {
	view.list = control
	view.widget = gtkScrolledWindowNew()
	scroll := (*gtkScrolledWindow)(unsafe.Pointer(view.widget))
	gtkScrolledWindowSetHasFrame(scroll, nativeBool(framed))
	gtkScrolledWindowSetPolicy(scroll, gtkPolicyNever, gtkPolicyAutomatic)
	gtkScrolledWindowSetChild(scroll, control)
	registerNativeComponent(window, view.widget, control, kind)
	storeComponentState(view.widget, view)
	connectNativeArgument(unsafe.Pointer(view.selection), "selection-changed", gtkSelectionSignalCallback(), func() { view.report(gtkEvent{kind: gkEventSelection}) })
}
func (view *nativeDataView) rowAt(position cUint) *nativeDataRow {
	if view.tree != nil {
		row := gtkTreeListModelGetRow(view.tree, position)
		if row == nil {
			return nil
		}
		object := gtkTreeListRowGetItem(row)
		data := dataRow(object)
		gObjectUnref(object)
		gObjectUnref(unsafe.Pointer(row))
		return data
	}
	object := gListModelGetItem((*gListModel)(unsafe.Pointer(view.rows)), position)
	if object == nil {
		return nil
	}
	data := dataRow(object)
	gObjectUnref(object)
	return data
}
func (view *nativeDataView) total() cUint {
	if view.tree != nil {
		return gListModelGetNItems((*gListModel)(unsafe.Pointer(view.tree)))
	}
	return gListModelGetNItems((*gListModel)(unsafe.Pointer(view.rows)))
}
func (view *nativeDataView) selectedIDs() []string {
	var ids []string
	for i := cUint(0); i < view.total(); i++ {
		if gtkSelectionModelIsSelected(view.selection, i) != 0 {
			if row := view.rowAt(i); row != nil {
				ids = append(ids, row.id)
			}
		}
	}
	return ids
}
func packNativeIDs(ids []string, size *cSize) *cChar {
	buffer := joinNul(ids)
	*size = cSize(len(buffer))
	if len(buffer) == 0 {
		return nil
	}
	pointer := gMalloc(gSize(len(buffer)))
	copy(unsafe.Slice((*byte)(pointer), len(buffer)), buffer)
	return (*cChar)(pointer)
}
func (view *nativeDataView) selectIDs(ids []string) {
	view.suppress = true
	defer func() { view.suppress = false }()
	if view.tree != nil {
		for _, id := range ids {
			row, _ := view.reveal(id)
			if row != nil {
				gObjectUnref(unsafe.Pointer(row))
			}
		}
	}
	positions := gtkBitsetNewEmpty()
	defer gtkBitsetUnref(positions)
	for i := cUint(0); i < view.total(); i++ {
		row := view.rowAt(i)
		if row != nil && slices.Contains(ids, row.id) {
			gtkBitsetAdd(positions, i)
			if view.selectionMode == 1 {
				break
			}
		}
	}
	if view.selectionMode == 1 {
		position := ^cUint(0)
		for i := cUint(0); i < view.total(); i++ {
			row := view.rowAt(i)
			if row != nil && slices.Contains(ids, row.id) {
				position = i
				break
			}
		}
		gtkSingleSelectionSetSelected((*gtkSingleSelection)(unsafe.Pointer(view.selection)), position)
	} else if view.selectionMode == 2 {
		mask := gtkBitsetNewRange(0, view.total())
		gtkSelectionModelSetSelection(view.selection, positions, mask)
		gtkBitsetUnref(mask)
	}
}
func (view *nativeDataView) listFactory(column int) *gtkListItemFactory {
	return newDataFactory(func(item *gtkListItem) {
		box := gtkBoxNew(gtkOrientationHorizontal, 6)
		image := gtkImageNew()
		gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(image)), 16)
		text := gtkLabelNew(stringPointer(""))
		gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(text)), 0)
		gtkLabelSetEllipsize((*gtkLabel)(unsafe.Pointer(text)), pangoEllipsizeEnd)
		detail := gtkLabelNew(stringPointer(""))
		gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(detail)), 0)
		gtkWidgetAddCSSClass(detail, stringPointer("dim-label"))
		gtkWidgetSetHexpand(detail, 1)
		for _, child := range []*gtkWidget{image, text, detail} {
			gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), child)
		}
		gtkWidgetSetVisible(image, nativeBool(view.mode == 1))
		gtkWidgetSetVisible(detail, nativeBool(view.mode == 1))
		gtkListItemSetChild(item, box)
	}, func(item *gtkListItem) {
		row := dataRow(gtkListItemGetItem(item))
		if row == nil {
			return
		}
		box := gtkListItemGetChild(item)
		image := gtkWidgetGetFirstChild(box)
		text := gtkWidgetGetNextSibling(image)
		detail := gtkWidgetGetNextSibling(text)
		value := ""
		if column < len(row.cells) {
			value = row.cells[column]
		}
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(text)), stringPointer(value))
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(detail)), stringPointer(row.detail))
		gtkImageSetFromPaintable((*gtkImage)(unsafe.Pointer(image)), row.icon)
		gtkWidgetSetVisible(detail, nativeBool(row.detail != ""))
	})
}
func gkGTKTableCreate(window cUintptr, list cInt, columnIDs *cChar, columnIDsSize cSize, titles *cChar, titlesSize cSize, widths, sortable *cInt, selection cInt) unsafe.Pointer {
	view := &nativeDataView{rows: gListStoreNew(gObjectGetType()), columns: splitNul(unsafe.Pointer(columnIDs), columnIDsSize), selectionMode: int(selection)}
	view.selection = newDataSelection((*gListModel)(unsafe.Pointer(view.rows)), int(selection))
	gObjectRef(unsafe.Pointer(view.selection))
	var control *gtkWidget
	if list != 0 {
		view.mode = 1
		control = gtkListViewNew(view.selection, view.listFactory(0))
		gtkListViewSetSingleClickActivate((*gtkListView)(unsafe.Pointer(control)), 0)
	} else {
		control = gtkColumnViewNew(view.selection)
		columns := (*gtkColumnView)(unsafe.Pointer(control))
		gtkColumnViewSetShowRowSeparators(columns, 0)
		gtkColumnViewSetReorderable(columns, 0)
		names := splitNul(unsafe.Pointer(titles), titlesSize)
		sizes, sorts := unsafe.Slice(widths, len(view.columns)), unsafe.Slice(sortable, len(view.columns))
		for i, id := range view.columns {
			title := ""
			if i < len(names) {
				title = names[i]
			}
			column := gtkColumnViewColumnNew(stringPointer(title), view.listFactory(i))
			gtkColumnViewColumnSetId(column, stringPointer(id))
			gtkColumnViewColumnSetResizable(column, 1)
			if sizes[i] > 0 {
				gtkColumnViewColumnSetFixedWidth(column, sizes[i])
			} else {
				gtkColumnViewColumnSetExpand(column, 1)
			}
			if sorts[i] != 0 {
				sorter := gtkCustomSorterNew(gtkEqualSortCallback(), nil, nil)
				gtkColumnViewColumnSetSorter(column, (*gtkSorter)(unsafe.Pointer(sorter)))
				gObjectUnref(unsafe.Pointer(sorter))
			}
			gtkColumnViewAppendColumn(columns, column)
			gObjectUnref(unsafe.Pointer(column))
		}
		sorter := gtkColumnViewGetSorter(columns)
		connectNativeArgument(unsafe.Pointer(sorter), "changed", gtkIntSignalCallback(), func() {
			column := gtkColumnViewSorterGetPrimarySortColumn((*gtkColumnViewSorter)(unsafe.Pointer(sorter)))
			if column != nil {
				ascending := 0
				if gtkColumnViewSorterGetPrimarySortOrder((*gtkColumnViewSorter)(unsafe.Pointer(sorter))) == gtkSortAscending {
					ascending = 1
				}
				view.report(gtkEvent{kind: gkEventSort, a: ascending, text: goString(gtkColumnViewColumnGetId(column))})
			}
		})
	}
	view.finish(uintptr(window), control, kindTable, true)
	connectNativeArgument(unsafe.Pointer(control), "activate", gtkUIntSignalCallback(), func(position uint32) {
		if row := view.rowAt(cUint(position)); row != nil {
			view.report(gtkEvent{kind: gkEventActivate, text: row.id})
		}
	})
	return unsafe.Pointer(view.widget)
}
func gkGTKTableSetRows(pointer unsafe.Pointer, ids *cChar, idsSize cSize, cells *cChar, cellsSize cSize, details *cChar, detailsSize cSize, icons *unsafe.Pointer, columns cInt) {
	view := componentState[nativeDataView](pointer)
	if view == nil {
		return
	}
	identifiers, values, descriptions := splitNul(unsafe.Pointer(ids), idsSize), splitNul(unsafe.Pointer(cells), cellsSize), splitNul(unsafe.Pointer(details), detailsSize)
	paintables := borrowedPaintables(icons, len(identifiers))
	objects := make([]unsafe.Pointer, len(identifiers))
	for i, id := range identifiers {
		row := newDataRow(id, paintables[i])
		for column := 0; column < max(1, int(columns)); column++ {
			value := ""
			if offset := i*max(1, int(columns)) + column; offset < len(values) {
				value = values[offset]
			}
			row.cells = append(row.cells, value)
		}
		if i < len(descriptions) {
			row.detail = descriptions[i]
		}
		objects[i] = unsafe.Pointer(row.object)
	}
	view.suppress = true
	gListStoreSplice(view.rows, 0, gListModelGetNItems((*gListModel)(unsafe.Pointer(view.rows))), unsafe.SliceData(objects), cUint(len(objects)))
	view.suppress = false
	for _, object := range objects {
		gObjectUnref(object)
	}
}
func gkGTKTableSelection(pointer unsafe.Pointer, size *cSize) *cChar {
	view := componentState[nativeDataView](pointer)
	if view == nil {
		*size = 0
		return nil
	}
	return packNativeIDs(view.selectedIDs(), size)
}
func gkGTKTableSelect(pointer unsafe.Pointer, ids *cChar, size cSize) {
	if view := componentState[nativeDataView](pointer); view != nil {
		view.selectIDs(splitNul(unsafe.Pointer(ids), size))
	}
}
func gkGTKTableScrollTo(pointer unsafe.Pointer, id *cChar) cInt {
	view := componentState[nativeDataView](pointer)
	if view == nil {
		return 0
	}
	for i := cUint(0); i < view.total(); i++ {
		row := view.rowAt(i)
		if row != nil && row.id == goString(id) {
			if view.mode == 1 {
				gtkListViewScrollTo((*gtkListView)(unsafe.Pointer(view.list)), i, 0, nil)
			} else {
				gtkColumnViewScrollTo((*gtkColumnView)(unsafe.Pointer(view.list)), i, nil, 0, nil)
			}
			return 1
		}
	}
	return 0
}

func gkGTKOutlineCreate(window cUintptr, navigation, multiple cInt) unsafe.Pointer {
	view := &nativeDataView{rows: gListStoreNew(gObjectGetType()), mode: 3, selectionMode: 1, nodes: make(map[string]*nativeDataRow)}
	if multiple != 0 {
		view.selectionMode = 2
	}
	gObjectRef(unsafe.Pointer(view.rows))
	view.tree = gtkTreeListModelNew((*gListModel)(unsafe.Pointer(view.rows)), 0, nativeBool(navigation != 0), gtkTreeChildrenCallback(), newNativeCallback(func(object unsafe.Pointer) *gListModel {
		row := dataRow(object)
		if row == nil || !row.hasChildren && gListModelGetNItems((*gListModel)(unsafe.Pointer(row.children))) == 0 {
			return nil
		}
		if row.hasChildren && !row.requested && gListModelGetNItems((*gListModel)(unsafe.Pointer(row.children))) == 0 {
			row.requested = true
			view.report(gtkEvent{kind: gkEventExpand, a: 1, text: row.id})
		}
		gObjectRef(unsafe.Pointer(row.children))
		return (*gListModel)(unsafe.Pointer(row.children))
	}), gtkDestroyCallback())
	view.selection = newDataSelection((*gListModel)(unsafe.Pointer(view.tree)), view.selectionMode)
	factory := newDataFactory(func(item *gtkListItem) {
		expander := gtkTreeExpanderNew()
		box := gtkBoxNew(gtkOrientationHorizontal, 6)
		image := gtkImageNew()
		gtkImageSetPixelSize((*gtkImage)(unsafe.Pointer(image)), 16)
		text := gtkLabelNew(stringPointer(""))
		gtkLabelSetXalign((*gtkLabel)(unsafe.Pointer(text)), 0)
		gtkLabelSetEllipsize((*gtkLabel)(unsafe.Pointer(text)), pangoEllipsizeEnd)
		gtkWidgetSetHexpand(text, 1)
		badge := gtkLabelNew(stringPointer(""))
		gtkWidgetAddCSSClass(badge, stringPointer("dim-label"))
		for _, child := range []*gtkWidget{image, text, badge} {
			gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), child)
		}
		gtkTreeExpanderSetChild((*gtkTreeExpander)(unsafe.Pointer(expander)), box)
		gtkListItemSetChild(item, expander)
	}, func(item *gtkListItem) {
		treeRow := (*gtkTreeListRow)(gtkListItemGetItem(item))
		object := gtkTreeListRowGetItem(treeRow)
		row := dataRow(object)
		defer gObjectUnref(object)
		if row == nil {
			return
		}
		expander := (*gtkTreeExpander)(unsafe.Pointer(gtkListItemGetChild(item)))
		gtkTreeExpanderSetListRow(expander, treeRow)
		box := gtkTreeExpanderGetChild(expander)
		image := gtkWidgetGetFirstChild(box)
		text := gtkWidgetGetNextSibling(image)
		badge := gtkWidgetGetNextSibling(text)
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(text)), stringPointer(row.cells[0]))
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(badge)), stringPointer(row.detail))
		gtkWidgetSetVisible(badge, nativeBool(row.detail != ""))
		gtkImageSetFromPaintable((*gtkImage)(unsafe.Pointer(image)), row.icon)
		gtkWidgetSetVisible(image, nativeBool(row.icon != nil && !row.group))
		for _, class := range []string{"heading", "dim-label"} {
			if row.group {
				gtkWidgetAddCSSClass(text, stringPointer(class))
			} else {
				gtkWidgetRemoveCSSClass(text, stringPointer(class))
			}
		}
		gtkListItemSetSelectable(item, nativeBool(!row.group))
		gtkTreeExpanderSetHideExpander(expander, nativeBool(navigation != 0 && row.group))
	})
	gObjectRef(unsafe.Pointer(view.selection))
	list := gtkListViewNew(view.selection, factory)
	if navigation != 0 {
		gtkWidgetAddCSSClass(list, stringPointer("navigation-sidebar"))
	}
	view.finish(uintptr(window), list, kindOutline, navigation == 0)
	return unsafe.Pointer(view.widget)
}
func gkGTKOutlineSetNodes(pointer unsafe.Pointer, parent *cChar, depths, flags *cInt, ids *cChar, idsSize cSize, texts *cChar, textsSize cSize, badges *cChar, badgesSize cSize, icons *unsafe.Pointer, count cInt) {
	view := componentState[nativeDataView](pointer)
	if view == nil {
		return
	}
	target := view.rows
	var owner *nativeDataRow
	if parent != nil {
		owner = view.nodes[goString(parent)]
		if owner == nil {
			return
		}
		target = owner.children
		owner.requested = true
		owner.hasChildren = count > 0
	}
	for id, row := range view.nodes {
		if owner == nil {
			delete(view.nodes, id)
			continue
		}
		for node := row.parent; node != nil; node = node.parent {
			if node == owner {
				delete(view.nodes, id)
				break
			}
		}
	}
	identifiers, labels, details := splitNul(unsafe.Pointer(ids), idsSize), splitNul(unsafe.Pointer(texts), textsSize), splitNul(unsafe.Pointer(badges), badgesSize)
	levels, bits := unsafe.Slice(depths, int(count)), unsafe.Slice(flags, int(count))
	paintables := borrowedPaintables(icons, int(count))
	var roots []unsafe.Pointer
	var stack []*nativeDataRow
	for i := 0; i < int(count); i++ {
		row := newDataRow(identifiers[i], paintables[i])
		row.cells = []string{labels[i]}
		row.detail = details[i]
		row.children = gListStoreNew(gObjectGetType())
		row.hasChildren, row.group = bits[i]&1 != 0, bits[i]&2 != 0
		for len(stack) > int(levels[i]) {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			row.parent = owner
			roots = append(roots, unsafe.Pointer(row.object))
		} else {
			row.parent = stack[len(stack)-1]
			gListStoreAppend(row.parent.children, unsafe.Pointer(row.object))
			gObjectUnref(unsafe.Pointer(row.object))
		}
		view.nodes[row.id] = row
		stack = append(stack, row)
	}
	view.suppress = true
	gListStoreSplice(target, 0, gListModelGetNItems((*gListModel)(unsafe.Pointer(target))), unsafe.SliceData(roots), cUint(len(roots)))
	view.suppress = false
	for _, root := range roots {
		gObjectUnref(root)
	}
}
func (view *nativeDataView) findTreeRow(target *nativeDataRow) (*gtkTreeListRow, cUint) {
	for i := cUint(0); i < view.total(); i++ {
		row := gtkTreeListModelGetRow(view.tree, i)
		object := gtkTreeListRowGetItem(row)
		match := dataRow(object) == target
		gObjectUnref(object)
		if match {
			return row, i
		}
		gObjectUnref(unsafe.Pointer(row))
	}
	return nil, 0
}
func (view *nativeDataView) reveal(id string) (*gtkTreeListRow, cUint) {
	target := view.nodes[id]
	if target == nil {
		return nil, 0
	}
	var path []*nativeDataRow
	for parent := target.parent; parent != nil; parent = parent.parent {
		path = append(path, parent)
	}
	for i := len(path) - 1; i >= 0; i-- {
		row, _ := view.findTreeRow(path[i])
		if row != nil {
			gtkTreeListRowSetExpanded(row, 1)
			gObjectUnref(unsafe.Pointer(row))
		}
	}
	return view.findTreeRow(target)
}
func gkGTKOutlineSelection(pointer unsafe.Pointer, size *cSize) *cChar {
	return gkGTKTableSelection(pointer, size)
}
func gkGTKOutlineSelect(pointer unsafe.Pointer, ids *cChar, size cSize) {
	gkGTKTableSelect(pointer, ids, size)
}
func gkGTKOutlineExpanded(pointer unsafe.Pointer, id *cChar) cInt {
	view := componentState[nativeDataView](pointer)
	if view == nil {
		return 0
	}
	row, _ := view.findTreeRow(view.nodes[goString(id)])
	if row == nil {
		return 0
	}
	defer gObjectUnref(unsafe.Pointer(row))
	if gtkTreeListRowGetExpanded(row) != 0 {
		return 1
	}
	return 0
}
func gkGTKOutlineSetExpanded(pointer unsafe.Pointer, id *cChar, expanded cInt) cInt {
	view := componentState[nativeDataView](pointer)
	if view == nil || view.nodes[goString(id)] == nil {
		return 0
	}
	row, _ := view.reveal(goString(id))
	if row != nil {
		gtkTreeListRowSetExpanded(row, nativeBool(expanded != 0))
		gObjectUnref(unsafe.Pointer(row))
	}
	return 1
}
func gkGTKCollectionCreate(window cUintptr, width, height, selection cInt) unsafe.Pointer {
	view := &nativeDataView{rows: gListStoreNew(gObjectGetType()), mode: 4, selectionMode: int(selection)}
	view.selection = newDataSelection((*gListModel)(unsafe.Pointer(view.rows)), int(selection))
	if width <= 0 {
		width = 96
	}
	if height <= 0 {
		height = 96
	}
	factory := newDataFactory(func(item *gtkListItem) {
		box := gtkBoxNew(gtkOrientationVertical, 2)
		gtkWidgetSetSizeRequest(box, width, height)
		picture := gtkPictureNew()
		gtkPictureSetContentFit((*gtkPicture)(unsafe.Pointer(picture)), gtkContentFitContain)
		gtkWidgetSetVexpand(picture, 1)
		text := gtkLabelNew(stringPointer(""))
		gtkLabelSetEllipsize((*gtkLabel)(unsafe.Pointer(text)), pangoEllipsizeEnd)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), picture)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(box)), text)
		gtkListItemSetChild(item, box)
	}, func(item *gtkListItem) {
		row := dataRow(gtkListItemGetItem(item))
		if row == nil {
			return
		}
		picture := gtkWidgetGetFirstChild(gtkListItemGetChild(item))
		text := gtkWidgetGetNextSibling(picture)
		gtkLabelSetText((*gtkLabel)(unsafe.Pointer(text)), stringPointer(row.cells[0]))
		gtkPictureSetPaintable((*gtkPicture)(unsafe.Pointer(picture)), row.icon)
	})
	gObjectRef(unsafe.Pointer(view.selection))
	grid := gtkGridViewNew(view.selection, factory)
	gtkGridViewSetMaxColumns((*gtkGridView)(unsafe.Pointer(grid)), 64)
	view.finish(uintptr(window), grid, kindCollection, false)
	return unsafe.Pointer(view.widget)
}
func gkGTKCollectionSetItems(pointer unsafe.Pointer, ids *cChar, idsSize cSize, texts *cChar, textsSize cSize, icons *unsafe.Pointer, count cInt) {
	gkGTKTableSetRows(pointer, ids, idsSize, texts, textsSize, nil, 0, icons, 1)
}
func gkGTKCollectionSelection(pointer unsafe.Pointer, size *cSize) *cChar {
	return gkGTKTableSelection(pointer, size)
}
func gkGTKCollectionSelect(pointer unsafe.Pointer, ids *cChar, size cSize) {
	gkGTKTableSelect(pointer, ids, size)
}

func gkGTKBreadcrumbsCreate(window cUintptr) unsafe.Pointer {
	box := boxWithRole(gtkOrientationHorizontal, 2, gtkAccessibleRoleNavigation)
	setAccessibleText(box, gtkAccessiblePropertyLabel, "Breadcrumbs")
	registerNativeComponent(uintptr(window), box, nil, kindBreadcrumbs)
	return unsafe.Pointer(box)
}
func clearNativeBox(box *gtkWidget) {
	for child := gtkWidgetGetFirstChild(box); child != nil; child = gtkWidgetGetFirstChild(box) {
		gtkBoxRemove((*gtkBox)(unsafe.Pointer(box)), child)
	}
}
func gkGTKBreadcrumbsSetItems(pointer unsafe.Pointer, titles *cChar, size cSize) {
	box := (*gtkWidget)(pointer)
	clearNativeBox(box)
	names := splitNul(unsafe.Pointer(titles), size)
	for i, title := range names {
		if i > 0 {
			separator := gtkImageNewFromIconName(stringPointer("go-next-symbolic"))
			gtkWidgetAddCSSClass(separator, stringPointer("dim-label"))
			gtkBoxAppend((*gtkBox)(pointer), separator)
		}
		button := gtkButtonNewWithLabel(stringPointer(title))
		gtkWidgetAddCSSClass(button, stringPointer("flat"))
		if i == len(names)-1 {
			setAccessibleBoolState(button, gtkAccessibleStateSelected, true)
		}
		connectNativeSignal(unsafe.Pointer(button), "clicked", func() { sendComponentEvent(box, gtkEvent{kind: gkEventActivate, a: i}) })
		gtkBoxAppend((*gtkBox)(pointer), button)
	}
}

type nativePagination struct {
	widget, previous, next, label, pages *gtkWidget
	count, selected                      int
	compact, updating                    bool
	idle                                 cUint
}

func (state *nativePagination) releaseNative() {
	if state.idle != 0 {
		gSourceRemove(state.idle)
	}
}
func (state *nativePagination) choose(page int) {
	page = max(0, min(state.count-1, page))
	if page == state.selected {
		return
	}
	state.selected = page
	state.update()
	sendComponentEvent(state.widget, gtkEvent{kind: gkEventSelection, a: page})
}
func (state *nativePagination) update() {
	state.updating = true
	defer func() { state.updating = false }()
	gtkWidgetSetSensitive(state.previous, nativeBool(state.selected > 0))
	gtkWidgetSetSensitive(state.next, nativeBool(state.selected+1 < state.count))
	text := fmt.Sprintf("Page %d of %d", state.selected+1, state.count)
	gtkLabelSetText((*gtkLabel)(unsafe.Pointer(state.label)), stringPointer(text))
	gtkWidgetSetVisible(state.label, nativeBool(state.compact))
	gtkWidgetSetVisible(state.pages, nativeBool(!state.compact))
	clearNativeBox(state.pages)
	if !state.compact {
		visible := min(state.count, 7)
		start := max(0, min(state.selected-visible/2, state.count-visible))
		var first *gtkToggleButton
		for i := range visible {
			page := start + i
			button := gtkToggleButtonNewWithLabel(stringPointer(strconv.Itoa(page + 1)))
			toggle := (*gtkToggleButton)(unsafe.Pointer(button))
			if first != nil {
				gtkToggleButtonSetGroup(toggle, first)
			} else {
				first = toggle
			}
			gtkToggleButtonSetActive(toggle, nativeBool(page == state.selected))
			connectNativeSignal(unsafe.Pointer(button), "toggled", func() {
				if state.updating || gtkToggleButtonGetActive(toggle) == 0 {
					return
				}
				if state.idle != 0 {
					gSourceRemove(state.idle)
				}
				state.idle = gIdleAddFull(200, gtkSourceCallback(), newNativeCallback(func() bool { state.idle = 0; state.choose(page); return false }), gtkDestroyCallback())
			})
			gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.pages)), button)
		}
	}
	setAccessibleText(state.widget, gtkAccessiblePropertyValueText, text)
}
func gkGTKPaginationCreate(window cUintptr, count, selected, compact cInt) unsafe.Pointer {
	state := &nativePagination{
		widget:   boxWithRole(gtkOrientationHorizontal, 6, gtkAccessibleRoleGroup),
		previous: gtkButtonNewFromIconName(stringPointer("go-previous-symbolic")),
		next:     gtkButtonNewFromIconName(stringPointer("go-next-symbolic")),
		label:    gtkLabelNew(stringPointer("")),
		pages:    gtkBoxNew(gtkOrientationHorizontal, 0),
		count:    max(1, int(count)),
		compact:  compact != 0,
	}
	state.selected = max(0, min(int(selected), state.count-1))
	setAccessibleText(state.widget, gtkAccessiblePropertyLabel, "Pagination")
	setAccessibleText(state.previous, gtkAccessiblePropertyLabel, "Previous page")
	setAccessibleText(state.next, gtkAccessiblePropertyLabel, "Next page")
	gtkWidgetAddCSSClass(state.pages, stringPointer("linked"))
	middle := gtkCenterBoxNew()
	gtkWidgetSetHexpand(middle, 1)
	center := gtkBoxNew(gtkOrientationHorizontal, 0)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(center)), state.label)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(center)), state.pages)
	gtkCenterBoxSetCenterWidget((*gtkCenterBox)(unsafe.Pointer(middle)), center)
	for _, child := range []*gtkWidget{state.previous, middle, state.next} {
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.widget)), child)
	}
	registerNativeComponent(uintptr(window), state.widget, nil, kindPagination)
	storeComponentState(state.widget, state)
	connectNativeSignal(unsafe.Pointer(state.previous), "clicked", func() { state.choose(state.selected - 1) })
	connectNativeSignal(unsafe.Pointer(state.next), "clicked", func() { state.choose(state.selected + 1) })
	state.update()
	return unsafe.Pointer(state.widget)
}
func gkGTKPaginationSet(pointer unsafe.Pointer, count, selected cInt) {
	if state := componentState[nativePagination](pointer); state != nil {
		state.count = max(1, int(count))
		state.selected = max(0, min(int(selected), state.count-1))
		state.update()
	}
}

func borrowedPaintables(pointer *unsafe.Pointer, count int) []*gdkPaintable {
	if pointer == nil {
		return make([]*gdkPaintable, count)
	}
	return unsafe.Slice((**gdkPaintable)(unsafe.Pointer(pointer)), count)
}
