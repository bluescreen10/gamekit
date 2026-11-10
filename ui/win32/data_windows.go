package win32

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// imageList makes an image list of icons at a size in logical units, or
// returns 0 when there are no icons.
func imageList(owner hwnd, icons []*iconImage, width, height int, color themeColor) himagelist {
	if !slices.ContainsFunc(icons, func(icon *iconImage) bool { return icon != nil }) {
		return 0
	}
	scale := scaleOf(owner)
	w, h := int(math.Round(float64(width)*scale)), int(math.Round(float64(height)*scale))
	side := min(w, h)
	list := win32ImageListCreate(int32(w), int32(h), ilcColor32, int32(len(icons)), 1)
	for _, icon := range icons {
		pixels := iconPixels(icon, side, color)
		info := bitmapInfo{bmiHeader: bitmapInfoHeader{biWidth: int32(w), biHeight: -int32(h), biPlanes: 1, biBitCount: 32}}
		info.bmiHeader.biSize = uint32(unsafe.Sizeof(info.bmiHeader))
		var bits uintptr
		screen := win32GetDC(0)
		bitmap := win32CreateDIBSection(screen, unsafe.Pointer(&info), dibRGBColors, &bits, 0, 0)
		win32ReleaseDC(0, screen)
		if bitmap == 0 {
			continue
		}
		out := unsafe.Slice((*byte)(memoryAt(bits)), w*h*4)
		clear(out)
		ox, oy := (w-side)/2, (h-side)/2
		for y := 0; y < side && pixels != nil; y++ {
			for x := range side {
				p := pixels[(y*side+x)*4:]
				q := out[((y+oy)*w+(x+ox))*4:]
				a := uint32(p[3])
				q[0] = byte(uint32(p[2]) * a / 255)
				q[1] = byte(uint32(p[1]) * a / 255)
				q[2] = byte(uint32(p[0]) * a / 255)
				q[3] = byte(a)
			}
		}
		win32ImageListAdd(list, bitmap, 0)
		win32DeleteObject(hgdiobj(bitmap))
	}
	return list
}

func listColors(list hwnd, pal *palette) {
	background := pal.window
	if pal.dark {
		background = pal.layer
	}
	win32SendMessage(list, lvmSetBkColor, 0, uintptr(background.colorRef()))
	win32SendMessage(list, lvmSetTextBkColor, 0, uintptr(background.colorRef()))
	win32SendMessage(list, lvmSetTextColor, 0, uintptr(pal.text.colorRef()))
	if header := hwnd(win32SendMessage(list, lvmGetHeader, 0, 0)); header != 0 {
		theme := "ItemsView"
		if pal.dark {
			theme = "DarkMode_ItemsView"
		}
		win32SetWindowTheme(header, theme, "")
	}
}

// ---- Tables and lists ----

// tableRow is a row of a table or an item of a list: the text of each column,
// or the text, detail, and icon of an item.
type tableRow struct {
	id     string
	cells  []string
	detail string
	icon   *iconImage
}

type tableExtension struct {
	baseExtension
	isList    bool
	selection ui.SelectionMode
	columns   []ui.TableColumn
	// ids are the identifiers of the rows, in order.
	ids        []string
	sortColumn string
	ascending  bool
	images     himagelist
}

func (t *tableExtension) free() {
	if t.images != 0 {
		win32ImageListDestroy(t.images)
	}
}

func (t *tableExtension) showSort(view *nativeView) {
	header := hwnd(win32SendMessage(view.outer, lvmGetHeader, 0, 0))
	for i, column := range t.columns {
		item := hdItem{mask: hdiFormat}
		win32SendMessage(header, hdmGetItem, uintptr(i), uintptr(unsafe.Pointer(&item)))
		item.fmt &^= hdfSortUp | hdfSortDown
		if column.ID == t.sortColumn && t.sortColumn != "" {
			if t.ascending {
				item.fmt |= hdfSortUp
			} else {
				item.fmt |= hdfSortDown
			}
		}
		win32SendMessage(header, hdmSetItem, uintptr(i), uintptr(unsafe.Pointer(&item)))
	}
}

// selectionChanged reports whether a list view notification changes the
// selection of an item.
func selectionChanged(change *nmListView) bool {
	return change.uChanged&lvifState != 0 && (change.uNewState^change.uOldState)&lvisSelected != 0
}

// blocksSelection reports whether a list view notification selects an item.
func selects(change *nmListView) bool {
	return change.uNewState&lvisSelected != 0 && change.uOldState&lvisSelected == 0
}

func (t *tableExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	switch header.code {
	case lvnItemChanging:
		if t.selection == 0 && selects((*nmListView)(unsafe.Pointer(header))) {
			*result = 1
			return true
		}
		return false
	case lvnItemChanged:
		if selectionChanged((*nmListView)(unsafe.Pointer(header))) {
			view.report(changeEvent{})
		}
		*result = 0
		return true
	case lvnItemActivate:
		activate := (*nmItemActivate)(unsafe.Pointer(header))
		index := int(activate.iItem)
		if index < 0 {
			index = int(int32(win32SendMessage(view.outer, lvmGetNextItem, ^uintptr(0), lvniFocused)))
		}
		if index >= 0 && index < len(t.ids) {
			send(view.receiver, activateEvent{id: t.ids[index]})
		}
		*result = 0
		return true
	case lvnColumnClick:
		column := int((*nmListView)(unsafe.Pointer(header)).iSubItem)
		if t.isList || column < 0 || column >= len(t.columns) || !t.columns[column].Sortable {
			return false
		}
		if t.sortColumn == t.columns[column].ID {
			t.ascending = !t.ascending
		} else {
			t.sortColumn = t.columns[column].ID
			t.ascending = true
		}
		t.showSort(view)
		view.report(sortEvent{column: t.sortColumn, ascending: t.ascending})
		*result = 0
		return true
	case nmCustomDraw:
		if !t.isList {
			return false
		}
		// The detail of a row in a list is secondary text.
		draw := (*nmLVCustomDraw)(unsafe.Pointer(header))
		switch draw.nmcd.dwDrawStage {
		case cddsPrePaint:
			*result = cdrfNotifyItemDraw
			return true
		case cddsItemPrePaint:
			*result = cdrfNotifySubItemDraw
			return true
		case cddsItemPrePaint | cddsSubItem:
			pal := view.palette()
			if draw.iSubItem == 1 {
				draw.clrText = pal.textSecondary.colorRef()
			} else {
				draw.clrText = pal.text.colorRef()
			}
			*result = cdrfNewFont
			return true
		}
	}
	return false
}

func (t *tableExtension) themeChanged(view *nativeView) {
	applyThemeTo(view.outer, view.theme(), "")
	listColors(view.outer, view.palette())
}

func (t *tableExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: 320, height: 200}, true
}

func tableOf(view *nativeView) *tableExtension {
	if view == nil || view.kind != kindTable {
		return nil
	}
	table, _ := view.ext.(*tableExtension)
	return table
}

// newTable creates a table with columns, or a list, which shows the text and
// the detail of its items.
func (win *nativeWindow) newTable(list bool, columns []ui.TableColumn, selection ui.SelectionMode) *nativeView {
	table := &tableExtension{isList: list, selection: selection, columns: columns, ascending: true}
	if list {
		table.columns = []ui.TableColumn{{}, {}}
	}
	style := uint32(lvsReport | lvsShowSelAlways | wsTabStop)
	if selection != ui.SelectionMultiple {
		style |= lvsSingleSel
	}
	if list {
		style |= lvsNoColumnHeader
	}
	h := makeNative(wcListView, "", style, wsExClientEdge, 0)
	view := adoptView(win, h, 0, kindTable)
	view.ext = table
	win32SendMessage(h, lvmSetExtendedListViewStyle, 0,
		lvsExFullRowSelect|lvsExDoubleBuffer|lvsExHeaderDragDrop|lvsExInfoTip)
	for i, column := range table.columns {
		width := 120
		switch {
		case column.Width > 0:
			width = column.Width
		case list && i == 0:
			width = 200
		}
		header := lvColumn{mask: lvcfText | lvcfWidth | lvcfSubItem, pszText: wide(column.Title),
			cx: int32(math.Round(float64(width) * scaleOf(h))), iSubItem: int32(i)}
		win32SendMessage(h, lvmInsertColumn, uintptr(i), uintptr(unsafe.Pointer(&header)))
	}
	table.themeChanged(view)
	return view
}

// selectedIDs returns the identifiers of the selected items of a list view.
func selectedIDs(list hwnd, ids []string) []string {
	var result []string
	for index := -1; ; {
		index = int(int32(win32SendMessage(list, lvmGetNextItem, uintptr(index), lvniSelected)))
		if index < 0 {
			break
		}
		if index < len(ids) {
			result = append(result, ids[index])
		}
	}
	return result
}

func selectIDs(list hwnd, ids, wanted []string, single bool) {
	clearing := lvItem{stateMask: lvisSelected}
	win32SendMessage(list, lvmSetItemState, ^uintptr(0), uintptr(unsafe.Pointer(&clearing)))
	for i, id := range ids {
		if !slices.Contains(wanted, id) {
			continue
		}
		item := lvItem{stateMask: lvisSelected}
		if single {
			item.stateMask |= lvisFocused
		}
		item.state = item.stateMask
		win32SendMessage(list, lvmSetItemState, uintptr(i), uintptr(unsafe.Pointer(&item)))
		if single {
			break
		}
	}
}

func setItemText(list hwnd, index, column int, text string) {
	sub := lvItem{iSubItem: int32(column), pszText: wide(text)}
	win32SendMessage(list, lvmSetItemText, uintptr(index), uintptr(unsafe.Pointer(&sub)))
}

func at(values []string, i int) string {
	if i >= 0 && i < len(values) {
		return values[i]
	}
	return ""
}

func (view *nativeView) setRows(rows []tableRow) {
	table := tableOf(view)
	if table == nil {
		return
	}
	list := view.outer
	selected := selectedIDs(list, table.ids)
	view.suppress++
	win32SendMessage(list, wmSetRedraw, 0, 0)
	win32SendMessage(list, lvmDeleteAllItems, 0, 0)
	table.ids = make([]string, len(rows))
	icons := make([]*iconImage, len(rows))
	for i, row := range rows {
		table.ids[i] = row.id
		icons[i] = row.icon
	}
	if table.images != 0 {
		win32SendMessage(list, lvmSetImageList, lvsilSmall, 0)
		win32ImageListDestroy(table.images)
		table.images = 0
	}
	if table.isList {
		table.images = imageList(list, icons, 16, 16, view.palette().text)
		win32SendMessage(list, lvmSetImageList, lvsilSmall, uintptr(table.images))
	}
	for i, row := range rows {
		item := lvItem{mask: lvifText, iItem: int32(i), pszText: wide(at(row.cells, 0)), iImage: int32(i)}
		if table.images != 0 {
			item.mask |= lvifImage
		}
		win32SendMessage(list, lvmInsertItem, 0, uintptr(unsafe.Pointer(&item)))
		if table.isList {
			setItemText(list, i, 1, row.detail)
			continue
		}
		for column := 1; column < len(row.cells) && column < len(table.columns); column++ {
			setItemText(list, i, column, row.cells[column])
		}
	}
	if table.isList {
		win32SendMessage(list, lvmSetColumnWidth, 1, signExtend(lvscwAutoSizeUseHeader))
	}
	selectIDs(list, table.ids, selected, table.selection != ui.SelectionMultiple)
	win32SendMessage(list, wmSetRedraw, 1, 0)
	win32InvalidateRect(list, nil, true)
	view.suppress--
}

func (view *nativeView) selectedRows() []string {
	if table := tableOf(view); table != nil {
		return selectedIDs(view.outer, table.ids)
	}
	return nil
}

func (view *nativeView) selectRows(ids []string) {
	table := tableOf(view)
	if table == nil || table.selection == 0 {
		return
	}
	view.suppress++
	selectIDs(view.outer, table.ids, ids, table.selection != ui.SelectionMultiple)
	view.suppress--
}

func (view *nativeView) scrollToRow(identifier string) bool {
	table := tableOf(view)
	if table == nil {
		return false
	}
	index := slices.Index(table.ids, identifier)
	if index < 0 {
		return false
	}
	win32SendMessage(view.outer, lvmEnsureVisible, uintptr(index), 0)
	return true
}

// ---- Outlines: trees and navigation lists ----

type nativeOutlineNode struct {
	key         uintptr
	id          string
	text        string
	badge       string
	hasChildren bool
	group       bool
	loaded      bool
	item        uintptr
	parent      *nativeOutlineNode
}

type outlineExtension struct {
	baseExtension
	navigation bool
	shown      bool
	multiple   bool
	nodes      []*nativeOutlineNode
	selected   []*nativeOutlineNode
	images     himagelist
	icons      []*iconImage
	nextKey    uintptr
}

func (o *outlineExtension) free() {
	if o.images != 0 {
		win32ImageListDestroy(o.images)
	}
}

// nodeOf returns the node of a tree item: its lParam is the key of the node.
func (o *outlineExtension) nodeOf(tree hwnd, item uintptr) *nativeOutlineNode {
	if item == 0 {
		return nil
	}
	info := tvItem{mask: tvifParam | tvifHandle, hItem: item}
	win32SendMessage(tree, tvmGetItem, 0, uintptr(unsafe.Pointer(&info)))
	for _, node := range o.nodes {
		if node.key == info.lParam {
			return node
		}
	}
	return nil
}

func (o *outlineExtension) find(identifier string) *nativeOutlineNode {
	for _, node := range o.nodes {
		if node.item != 0 && node.id == identifier {
			return node
		}
	}
	return nil
}

func (o *outlineExtension) mark(tree hwnd, node *nativeOutlineNode, on bool) {
	info := tvItem{mask: tvifState | tvifHandle, hItem: node.item, stateMask: tvisSelected}
	if on {
		info.state = tvisSelected
	}
	win32SendMessage(tree, tvmSetItem, 0, uintptr(unsafe.Pointer(&info)))
}

func treeNext(tree hwnd, flag, item uintptr) uintptr {
	return win32SendMessage(tree, tvmGetNextItem, flag, item)
}

func treeExpand(tree hwnd, item uintptr, action uintptr) {
	win32SendMessage(tree, tvmExpand, action, item)
}

func (o *outlineExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	tree := view.outer
	switch header.code {
	case tvnSelChanging:
		change := (*nmTreeView)(unsafe.Pointer(header))
		// A group heading is not selectable.
		if node := o.nodeOf(tree, change.itemNew.hItem); node != nil && node.group && view.suppress == 0 {
			*result = 1
			return true
		}
		return false
	case tvnSelChanged:
		change := (*nmTreeView)(unsafe.Pointer(header))
		node := o.nodeOf(tree, change.itemNew.hItem)
		if view.suppress == 0 {
			add := o.multiple && keyDown(vkControl)
			if !add {
				for _, other := range o.selected {
					if other != node && other.item != 0 {
						o.mark(tree, other, false)
					}
				}
				o.selected = nil
			}
			if node != nil {
				if found := slices.Index(o.selected, node); found < 0 {
					o.selected = append(o.selected, node)
				} else if add {
					o.selected = slices.Delete(o.selected, found, found+1)
				}
			}
			for _, other := range o.selected {
				o.mark(tree, other, true)
			}
			view.report(changeEvent{})
		}
		*result = 0
		return true
	case tvnItemExpanding:
		change := (*nmTreeView)(unsafe.Pointer(header))
		node := o.nodeOf(tree, change.itemNew.hItem)
		if node != nil && change.action == tveExpand && node.hasChildren && !node.loaded &&
			treeNext(tree, tvgnChild, node.item) == 0 {
			view.report(expandEvent{id: node.id, expanded: true})
		}
		*result = 0
		return true
	case nmCustomDraw:
		return o.customDraw(view, (*nmTVCustomDraw)(unsafe.Pointer(header)), result)
	}
	return false
}

func (o *outlineExtension) customDraw(view *nativeView, draw *nmTVCustomDraw, result *uintptr) bool {
	tree := view.outer
	pal := view.palette()
	if draw.nmcd.dwDrawStage == cddsPrePaint {
		*result = cdrfNotifyItemDraw
		return true
	}
	node := o.nodeOf(tree, draw.nmcd.dwItemSpec)
	if draw.nmcd.dwDrawStage == cddsItemPrePaint {
		if node != nil && node.group && o.navigation {
			// The heading of a group sits at the bottom of its row, so the room
			// of the row above it separates the group from the previous one.
			dc := draw.nmcd.hdc
			row := draw.nmcd.rc
			background := win32CreateSolidBrush(uint32(win32SendMessage(tree, tvmGetBkColor, 0, 0)))
			win32FillRect(dc, &row, background)
			win32DeleteObject(hgdiobj(background))
			label := rect{}
			*(*uintptr)(unsafe.Pointer(&label)) = draw.nmcd.dwItemSpec
			win32SendMessage(tree, tvmGetItemRect, 1, uintptr(unsafe.Pointer(&label)))
			font := bodyFont()
			font.weight = 600
			previous := win32SelectObject(dc, hgdiobj(fontHandle(font, win32GetDpiForWindow(tree))))
			win32SetBkMode(dc, transparent)
			win32SetTextColor(dc, pal.textSecondary.colorRef())
			area := rect{left: label.left, top: row.top, right: row.right,
				bottom: row.bottom - int32(math.Round(2*scaleOf(tree)))}
			win32DrawText(dc, node.text, &area, dtLeft|dtBottom|dtSingleLine|dtNoPrefix|dtEndEllipsis)
			win32SelectObject(dc, previous)
			*result = cdrfSkipDefault
			return true
		}
		if node != nil && node.group {
			draw.clrText = pal.textSecondary.colorRef()
		}
		*result = cdrfNotifyPostPaint | cdrfNewFont
		return true
	}
	if draw.nmcd.dwDrawStage == cddsItemPostPaint && node != nil && node.badge != "" {
		// A badge is secondary text at the end of the row.
		row := draw.nmcd.rc
		var client rect
		win32GetClientRect(tree, &client)
		row.right = client.right - int32(math.Round(8*scaleOf(tree)))
		win32SetBkMode(draw.nmcd.hdc, transparent)
		win32SetTextColor(draw.nmcd.hdc, pal.textSecondary.colorRef())
		win32DrawText(draw.nmcd.hdc, node.badge, &row, dtRight|dtVCenter|dtSingleLine|dtNoPrefix)
		*result = cdrfDoDefault
		return true
	}
	return false
}

func (o *outlineExtension) layout(view *nativeView) {
	// A navigation list shows its top, with the selected item, once it has a
	// size: a selection made before scrolls the item to the top.
	if !o.navigation || view.height <= 0 || o.shown {
		return
	}
	o.shown = true
	tree := view.outer
	if root := treeNext(tree, tvgnRoot, 0); root != 0 {
		win32SendMessage(tree, tvmSelectItem, tvgnFirstVisible, root)
	}
	if selection := treeNext(tree, tvgnCaret, 0); selection != 0 {
		win32SendMessage(tree, tvmEnsureVisible, 0, selection)
	}
}

func (o *outlineExtension) themeChanged(view *nativeView) {
	pal := view.palette()
	applyThemeTo(view.outer, view.theme(), "")
	background := pal.window
	if pal.dark {
		background = pal.layer
	}
	win32SendMessage(view.outer, tvmSetBkColor, 0, uintptr(background.colorRef()))
	win32SendMessage(view.outer, tvmSetTextColor, 0, uintptr(pal.text.colorRef()))
	// The glyphs of the icons take the text color of the theme.
	if len(o.icons) > 0 {
		previous := o.images
		o.images = imageList(view.outer, o.icons, 16, 16, pal.text)
		win32SendMessage(view.outer, tvmSetImageList, tvsilNormal, uintptr(o.images))
		if previous != 0 {
			win32ImageListDestroy(previous)
		}
	}
	win32InvalidateRect(view.outer, nil, true)
}

func (o *outlineExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: 200, height: 240}, true
}

func outlineOf(view *nativeView) *outlineExtension {
	if view == nil || view.kind != kindOutline {
		return nil
	}
	outline, _ := view.ext.(*outlineExtension)
	return outline
}

func (win *nativeWindow) newOutline(navigation bool, multiple bool) *nativeView {
	outline := &outlineExtension{navigation: navigation, multiple: multiple}
	style := uint32(wsTabStop | tvsShowSelAlways | tvsHasButtons | tvsTrackSelect)
	var exStyle uint32
	if navigation {
		style |= tvsFullRowSelect | tvsNoHScroll
	} else {
		style |= tvsLinesAtRoot
		exStyle = wsExClientEdge
	}
	h := makeNative(wcTreeView, "", style, exStyle, 0)
	view := adoptView(win, h, 0, kindOutline)
	view.ext = outline
	win32SendMessage(h, tvmSetExtendedStyle, tvsExDoubleBuffer|tvsExFadeInOutExpandos,
		tvsExDoubleBuffer|tvsExFadeInOutExpandos)
	padding := 6
	if navigation {
		padding = 12
	}
	rowHeight := float64(measureGDI("Ag", bodyFont(), 0, 0).cy+padding) * scaleOf(h)
	win32SendMessage(h, tvmSetItemHeight, uintptr(math.Round(rowHeight)), 0)
	outline.themeChanged(view)
	return view
}

func (o *outlineExtension) forget(tree hwnd, item uintptr) {
	for child := treeNext(tree, tvgnChild, item); child != 0; child = treeNext(tree, tvgnNext, child) {
		o.forget(tree, child)
	}
	if node := o.nodeOf(tree, item); node != nil {
		node.item = 0
		o.selected = slices.DeleteFunc(o.selected, func(other *nativeOutlineNode) bool { return other == node })
	}
}

func (o *outlineExtension) setChildren(tree hwnd, item uintptr, children bool) {
	info := tvItem{mask: tvifChildren | tvifHandle, hItem: item}
	if children {
		info.cChildren = 1
	}
	win32SendMessage(tree, tvmSetItem, 0, uintptr(unsafe.Pointer(&info)))
}

// setOutlineNodes replaces the children of a node, or the roots for
// parent nil, with nodes in depth-first order. Flag 1 marks a node with
// children, flag 2 a group.
func (view *nativeView) setOutlineNodes(parent *string, nodes []outlineNode) {
	outline := outlineOf(view)
	if outline == nil {
		return
	}
	tree := view.outer
	var owner *nativeOutlineNode
	target := tviRoot
	if parent != nil {
		if owner = outline.find(*parent); owner == nil {
			return
		}
		target = owner.item
	}
	view.suppress++
	win32SendMessage(tree, wmSetRedraw, 0, 0)
	// The children that the nodes replace go away with their records.
	var old []uintptr
	first := treeNext(tree, tvgnRoot, 0)
	if parent != nil {
		first = treeNext(tree, tvgnChild, target)
	}
	for child := first; child != 0; child = treeNext(tree, tvgnNext, child) {
		old = append(old, child)
	}
	for _, child := range old {
		outline.forget(tree, child)
		win32SendMessage(tree, tvmDeleteItem, 0, child)
	}
	// Each icon has a place in the image list of the tree.
	imageIndex := make([]int, len(nodes))
	for i, node := range nodes {
		imageIndex[i] = -1
		if icon := newIcon(node.icon, 16); icon != nil {
			outline.icons = append(outline.icons, icon)
			imageIndex[i] = len(outline.icons) - 1
		}
	}
	if outline.images != 0 {
		win32ImageListDestroy(outline.images)
	}
	outline.images = imageList(tree, outline.icons, 16, 16, view.palette().text)
	win32SendMessage(tree, tvmSetImageList, tvsilNormal, uintptr(outline.images))
	var stack []uintptr
	var records []*nativeOutlineNode
	for i, node := range nodes {
		outline.nextKey++
		record := &nativeOutlineNode{
			key:         outline.nextKey,
			id:          node.id,
			text:        node.text,
			badge:       node.badge,
			hasChildren: node.hasChildren,
			group:       node.group,
		}
		for len(stack) > node.depth {
			stack = stack[:len(stack)-1]
			records = records[:len(records)-1]
		}
		insert := tvInsertStruct{hParent: target, hInsertAfter: tviLast}
		if len(stack) > 0 {
			insert.hParent = stack[len(stack)-1]
		}
		insert.item.mask = tvifText | tvifParam | tvifChildren | tvifState
		insert.item.pszText = wide(record.text)
		insert.item.lParam = record.key
		if record.hasChildren {
			insert.item.cChildren = 1
		}
		insert.item.stateMask = tvisBold
		if record.group {
			insert.item.state = tvisBold
		}
		if outline.images != 0 {
			image := int32(iImageNone)
			if imageIndex[i] >= 0 {
				image = int32(imageIndex[i])
			}
			insert.item.mask |= tvifImage | tvifSelectedImage
			insert.item.iImage = image
			insert.item.iSelectedImage = image
		}
		record.parent = owner
		if len(records) > 0 {
			record.parent = records[len(records)-1]
		}
		record.item = win32SendMessage(tree, tvmInsertItem, 0, uintptr(unsafe.Pointer(&insert)))
		if record.parent != nil {
			record.parent.loaded = true
			outline.setChildren(tree, record.parent.item, true)
		}
		stack = append(stack, record.item)
		records = append(records, record)
		outline.nodes = append(outline.nodes, record)
	}
	if outline.navigation {
		// A group of a navigation list is always open.
		for _, node := range outline.nodes {
			if node.item != 0 && node.group {
				treeExpand(tree, node.item, tveExpand)
			}
		}
	}
	if owner != nil {
		owner.hasChildren = len(nodes) > 0
		owner.loaded = true
		outline.setChildren(tree, owner.item, len(nodes) > 0)
		state := win32SendMessage(tree, tvmGetItemState, owner.item, tvisExpanded)
		if len(nodes) > 0 && state&tvisExpanded == 0 {
			// The node that asked for its children opens with them.
			treeExpand(tree, owner.item, tveExpand)
		}
	}
	outline.nodes = slices.DeleteFunc(outline.nodes, func(node *nativeOutlineNode) bool { return node.item == 0 })
	win32SendMessage(tree, wmSetRedraw, 1, 0)
	win32InvalidateRect(tree, nil, true)
	view.suppress--
}

func (view *nativeView) selectedOutlineItems() []string {
	outline := outlineOf(view)
	if outline == nil {
		return nil
	}
	var ids []string
	if outline.multiple {
		for _, node := range outline.nodes {
			if node.item != 0 && slices.Contains(outline.selected, node) {
				ids = append(ids, node.id)
			}
		}
	} else if node := outline.nodeOf(view.outer, treeNext(view.outer, tvgnCaret, 0)); node != nil {
		ids = append(ids, node.id)
	}
	return ids
}

func outlineReveal(tree hwnd, item uintptr) {
	for parent := treeNext(tree, tvgnParent, item); parent != 0; parent = treeNext(tree, tvgnParent, parent) {
		treeExpand(tree, parent, tveExpand)
	}
}

func (view *nativeView) selectOutlineItems(ids []string) {
	outline := outlineOf(view)
	if outline == nil {
		return
	}
	tree := view.outer
	view.suppress++
	for _, node := range outline.selected {
		if node.item != 0 {
			outline.mark(tree, node, false)
		}
	}
	outline.selected = nil
	var first *nativeOutlineNode
	for _, identifier := range ids {
		node := outline.find(identifier)
		if node == nil {
			continue
		}
		outlineReveal(tree, node.item)
		if first == nil {
			first = node
		}
		outline.selected = append(outline.selected, node)
		if !outline.multiple {
			break
		}
	}
	var firstItem uintptr
	if first != nil {
		firstItem = first.item
	}
	win32SendMessage(tree, tvmSelectItem, tvgnCaret, firstItem)
	if outline.navigation && first != nil && first.parent != nil && first.parent.group && first.parent.item != 0 {
		// The heading of the group stays in view with the item.
		win32SendMessage(tree, tvmEnsureVisible, 0, first.parent.item)
		win32SendMessage(tree, tvmEnsureVisible, 0, first.item)
	}
	for _, node := range outline.selected {
		outline.mark(tree, node, true)
	}
	view.suppress--
}

func (view *nativeView) isOutlineItemExpanded(identifier string) bool {
	outline := outlineOf(view)
	if outline == nil {
		return false
	}
	node := outline.find(identifier)
	return node != nil && win32SendMessage(view.outer, tvmGetItemState, node.item, tvisExpanded)&tvisExpanded != 0
}

func (view *nativeView) setOutlineItemExpanded(identifier string, expanded bool) bool {
	outline := outlineOf(view)
	if outline == nil {
		return false
	}
	node := outline.find(identifier)
	if node == nil {
		return false
	}
	if expanded {
		outlineReveal(view.outer, node.item)
		treeExpand(view.outer, node.item, tveExpand)
	} else {
		treeExpand(view.outer, node.item, tveCollapse)
	}
	if expanded && node.hasChildren && treeNext(view.outer, tvgnChild, node.item) == 0 {
		// A node without its children yet keeps its state until the children
		// arrive.
		info := tvItem{mask: tvifState | tvifHandle, hItem: node.item, stateMask: tvisExpanded, state: tvisExpanded}
		win32SendMessage(view.outer, tvmSetItem, 0, uintptr(unsafe.Pointer(&info)))
	}
	return true
}

// ---- Collection view ----

type collectionExtension struct {
	baseExtension
	ids           []string
	width, height int
	selection     ui.SelectionMode
	images        himagelist
}

func (c *collectionExtension) free() {
	if c.images != 0 {
		win32ImageListDestroy(c.images)
	}
}

func (c *collectionExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	switch header.code {
	case lvnItemChanging:
		if c.selection == 0 && selects((*nmListView)(unsafe.Pointer(header))) {
			*result = 1
			return true
		}
	case lvnItemChanged:
		if selectionChanged((*nmListView)(unsafe.Pointer(header))) {
			view.report(changeEvent{})
		}
		*result = 0
		return true
	case lvnItemActivate:
		activate := (*nmItemActivate)(unsafe.Pointer(header))
		if activate.iItem >= 0 && int(activate.iItem) < len(c.ids) {
			send(view.receiver, activateEvent{id: c.ids[activate.iItem]})
		}
		*result = 0
		return true
	}
	return false
}

func (c *collectionExtension) themeChanged(view *nativeView) {
	applyThemeTo(view.outer, view.theme(), "")
	listColors(view.outer, view.palette())
}

func (c *collectionExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	return d2d1SizeF{width: float32(c.width*3 + 24), height: float32(c.height*2 + 24)}, true
}

func collectionOf(view *nativeView) *collectionExtension {
	if view == nil || view.kind != kindCollection {
		return nil
	}
	collection, _ := view.ext.(*collectionExtension)
	return collection
}

func (win *nativeWindow) newCollection(width int, height int, selection ui.SelectionMode) *nativeView {
	collection := &collectionExtension{width: width, height: height, selection: selection}
	if width <= 0 {
		collection.width = 96
	}
	if height <= 0 {
		collection.height = 96
	}
	style := uint32(lvsIcon | lvsShowSelAlways | lvsAutoArrange | wsTabStop)
	if selection != ui.SelectionMultiple {
		style |= lvsSingleSel
	}
	h := makeNative(wcListView, "", style, 0, 0)
	view := adoptView(win, h, 0, kindCollection)
	view.ext = collection
	win32SendMessage(h, lvmSetExtendedListViewStyle, 0, lvsExDoubleBuffer)
	scale := scaleOf(h)
	win32SendMessage(h, lvmSetIconSpacing, 0, makeWParam(int(math.Round(float64(collection.width+12)*scale)),
		int(math.Round(float64(collection.height+12)*scale))))
	collection.themeChanged(view)
	return view
}

func (view *nativeView) setCollectionItems(items []ui.CollectionItem) {
	collection := collectionOf(view)
	if collection == nil {
		return
	}
	list := view.outer
	selected := selectedIDs(list, collection.ids)
	view.suppress++
	win32SendMessage(list, wmSetRedraw, 0, 0)
	win32SendMessage(list, lvmDeleteAllItems, 0, 0)
	collection.ids = nil
	if collection.images != 0 {
		win32SendMessage(list, lvmSetImageList, lvsilNormal, 0)
		win32ImageListDestroy(collection.images)
		collection.images = 0
	}
	// The picture takes the cell above a line of text.
	line := measureGDI("Ag", bodyFont(), 0, 0).cy
	pictureHeight := max(16, collection.height-line-8)
	icons := make([]*iconImage, len(items))
	for i, item := range items {
		icons[i] = newIcon(ui.IconSource{Image: item.Image}, 0)
	}
	collection.images = imageList(list, icons, collection.width, pictureHeight, view.palette().text)
	if collection.images != 0 {
		win32SendMessage(list, lvmSetImageList, lvsilNormal, uintptr(collection.images))
	}
	for i, collectionItem := range items {
		collection.ids = append(collection.ids, collectionItem.ID)
		item := lvItem{mask: lvifText, iItem: int32(i), pszText: wide(collectionItem.Text), iImage: iImageNone}
		if collection.images != 0 {
			item.mask |= lvifImage
		}
		if icons[i] != nil {
			item.iImage = int32(i)
		}
		win32SendMessage(list, lvmInsertItem, 0, uintptr(unsafe.Pointer(&item)))
	}
	selectIDs(list, collection.ids, selected, collection.selection != ui.SelectionMultiple)
	win32SendMessage(list, wmSetRedraw, 1, 0)
	win32InvalidateRect(list, nil, true)
	view.suppress--
}

func (view *nativeView) selectedCollectionItems() []string {
	if collection := collectionOf(view); collection != nil {
		return selectedIDs(view.outer, collection.ids)
	}
	return nil
}

func (view *nativeView) selectCollectionItems(ids []string) {
	collection := collectionOf(view)
	if collection == nil || collection.selection == 0 {
		return
	}
	view.suppress++
	selectIDs(view.outer, collection.ids, ids, collection.selection != ui.SelectionMultiple)
	view.suppress--
}

// ---- Breadcrumbs ----
//
// Breadcrumbs are a link control with a link for each item but the last, which
// is the current place.

const breadcrumbSeparator = "  ›  "

type breadcrumbsExtension struct {
	baseExtension
	titles []string
}

func (b *breadcrumbsExtension) notify(view *nativeView, header *nmhdr, result *uintptr) bool {
	if header.code != nmClick && header.code != nmReturn {
		return false
	}
	link := (*nmLink)(unsafe.Pointer(header))
	view.report(activateEvent{index: int(link.item.iLink)})
	*result = 0
	return true
}

func (b *breadcrumbsExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	size := measureGDI(strings.Join(b.titles, breadcrumbSeparator), view.fontOr(bodyFont()), 0, 0)
	return d2d1SizeF{width: float32(size.cx + 4), height: float32(size.cy + 4)}, true
}

func (win *nativeWindow) newBreadcrumbs() *nativeView {
	h := makeNative(wcLink, "", wsTabStop|lwsTransparent|lwsNoPrefix, 0, 0)
	view := adoptView(win, h, 0, kindBreadcrumbs)
	view.ext = &breadcrumbsExtension{}
	if view.accessibleName == "" {
		view.accessibleName = "Breadcrumbs"
	}
	return view
}

func (view *nativeView) setBreadcrumbs(titles []string) {
	if !view.live() || view.kind != kindBreadcrumbs {
		return
	}
	crumbs := view.ext.(*breadcrumbsExtension)
	crumbs.titles = slices.Clone(titles)
	var markup strings.Builder
	escape := strings.NewReplacer("<", "&lt;", "&", "&amp;")
	for i, title := range titles {
		if i > 0 {
			markup.WriteString(breadcrumbSeparator)
		}
		if i+1 < len(titles) {
			fmt.Fprintf(&markup, `<a id="%d">`, i)
			markup.WriteString(strings.TrimPrefix(linkMarkup(title), "<a>"))
		} else {
			markup.WriteString(escape.Replace(title))
		}
	}
	win32SetWindowText(view.outer, markup.String())
}

func breadcrumbItems(view *nativeView) []accessibleItem {
	var items []accessibleItem
	if view != nil && view.kind == kindBreadcrumbs {
		for _, title := range view.ext.(*breadcrumbsExtension).titles {
			items = append(items, accessibleItem{"Hyperlink", title})
		}
	}
	return items
}

// ---- Pagination ----

type paginationExtension struct {
	baseExtension
	previous, next, label hwnd
	pages                 []hwnd
	count                 int
	selected              int
	compact               bool
}

func (p *paginationExtension) start() int {
	visible := min(p.count, 7)
	first := min(p.selected-visible/2, p.count-visible)
	return max(first, 0)
}

func (p *paginationExtension) update(view *nativeView) {
	for _, page := range p.pages {
		win32DestroyWindow(page)
	}
	p.pages = nil
	win32EnableWindow(p.previous, p.selected > 0)
	win32EnableWindow(p.next, p.selected+1 < p.count)
	text := fmt.Sprintf("Page %d of %d", p.selected+1, p.count)
	win32SetWindowText(p.label, text)
	if p.compact {
		win32ShowWindow(p.label, swShowNA)
	} else {
		win32ShowWindow(p.label, swHide)
	}
	view.accessibleValue = text
	if !p.compact {
		first := p.start()
		for i := range min(p.count, 7) {
			number := first + i + 1
			button := makeNative(wcButton, fmt.Sprint(number), bsAutoRadioButton|bsPushLike|wsTabStop, 0, view.outer)
			setViewOf(button, view)
			win32SetProp(button, "GKPage", uintptr(number))
			win32SendMessage(button, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(button))), 0)
			win32SendMessage(button, bmSetCheck, buttonCheck(first+i == p.selected), 0)
			applyThemeTo(button, view.theme(), "")
			p.pages = append(p.pages, button)
		}
	}
	p.layout(view)
}

func (p *paginationExtension) choose(view *nativeView, page int) {
	page = max(0, min(page, p.count-1))
	if page == p.selected {
		return
	}
	p.selected = page
	later(func() {
		if view.live() {
			p.update(view)
		}
	})
	view.report(pageChangeEvent{page: page})
}

func (p *paginationExtension) layout(view *nativeView) {
	height := float64(view.height)
	side := min(32, height+8)
	placeWindow(p.previous, 0, 0, side, height)
	placeWindow(p.next, float64(view.width)-side, 0, side, height)
	middle := float64(view.width) - 2*side - 12
	if p.compact {
		placeWindow(p.label, side+6, 0, max(0, middle), height)
		return
	}
	const width = 32
	total := float64(len(p.pages)) * (width + 2)
	x := side + 6 + max(0, (middle-total)/2)
	for _, page := range p.pages {
		placeWindow(page, x, 0, width, height)
		x += width + 2
	}
}

func (p *paginationExtension) command(view *nativeView, from hwnd, code int) bool {
	if code != bnClicked {
		return false
	}
	switch {
	case from == p.previous:
		p.choose(view, p.selected-1)
	case from == p.next:
		p.choose(view, p.selected+1)
	default:
		if page := int(win32GetProp(from, "GKPage")); page != 0 {
			p.choose(view, page-1)
		}
	}
	return true
}

func (p *paginationExtension) preferred(view *nativeView) (d2d1SizeF, bool) {
	height := float32(measureGDI("Ag", bodyFont(), 0, 0).cy + 8)
	middle := float32(min(p.count, 7) * 34)
	if p.compact {
		middle = 100
	}
	return d2d1SizeF{width: 2*32 + 12 + middle, height: height}, true
}

func (win *nativeWindow) newPagination(count int, selected int, compact bool) *nativeView {
	host := newLayout(parking())
	view := newView(host, 0, kindPagination, win)
	pagination := &paginationExtension{compact: compact}
	view.ext = pagination
	symbol := fontSpec{family: symbolFamily(), size: 10, weight: 400}
	pagination.previous = makeNative(wcButton, "", bsPushButton|wsTabStop, 0, host)
	pagination.next = makeNative(wcButton, "", bsPushButton|wsTabStop, 0, host)
	pagination.label = makeNative(wcStatic, "", ssCenter|ssCenterImage|ssNoPrefix, 0, host)
	for _, part := range []hwnd{pagination.previous, pagination.next} {
		setViewOf(part, view)
		win32SendMessage(part, wmSetFont, uintptr(fontHandle(symbol, win32GetDpiForWindow(part))), 0)
		applyThemeTo(part, view.theme(), "")
	}
	win32SendMessage(pagination.label, wmSetFont, uintptr(fontHandle(bodyFont(), win32GetDpiForWindow(host))), 0)
	pagination.count = max(count, 1)
	pagination.selected = max(0, min(selected, pagination.count-1))
	view.accessibleName = "Pagination"
	pagination.update(view)
	return view
}

func (view *nativeView) setPagination(count int, selected int) {
	if !view.live() || view.kind != kindPagination {
		return
	}
	pagination := view.ext.(*paginationExtension)
	pagination.count = max(count, 1)
	pagination.selected = max(0, min(selected, pagination.count-1))
	pagination.update(view)
}
