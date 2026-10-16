//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"slices"
	"strings"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// iconArray holds retained native icons for one call.
type iconArray struct {
	pointer unsafe.Pointer
	icons   []unsafe.Pointer
}

func newIconArray(sources []ui.IconSource, size float64) *iconArray {
	array := &iconArray{pointer: C.malloc(C.size_t(max(1, len(sources))) * C.size_t(unsafe.Sizeof(uintptr(0))))}
	array.icons = unsafe.Slice((*unsafe.Pointer)(array.pointer), max(1, len(sources)))[:len(sources)]
	for i, source := range sources {
		array.icons[i] = nativeIcon(source, size)
	}
	return array
}

func (a *iconArray) native() *unsafe.Pointer {
	return (*unsafe.Pointer)(a.pointer)
}

func (a *iconArray) free() {
	for _, icon := range a.icons {
		C.gkGTKRelease(icon)
	}
	C.free(a.pointer)
}

// imageIcons converts images to icon sources.
func imageIcons(count int, image func(int) ui.IconSource) []ui.IconSource {
	sources := make([]ui.IconSource, count)
	for i := range sources {
		sources[i] = image(i)
	}
	return sources
}

func readIDs(read func(*C.size_t) *C.char) []string {
	var size C.size_t
	buffer := read(&size)
	defer C.free(unsafe.Pointer(buffer))
	return splitNul(unsafe.Pointer(buffer), size)
}

// ---- Table and list ----

// table is a GtkListView or a GtkColumnView shared by list views, table views, and the
// command palette.
type table struct {
	*widgetBase
	columns   []ui.TableColumn
	rowIDs    []string
	selection ui.SelectionMode
	sort      ui.SortDescriptor
	selects   listeners[ui.SelectionIDsHandler]
	activates listeners[ui.ActionHandler]
	sorts     listeners[func(ui.SortDescriptor)]
}

func (w *Window) newTable(list bool, columns []ui.TableColumn, selection ui.SelectionMode, owner gtkReceiver) (*table, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(columns))
	titles := make([]string, len(columns))
	widths := make([]C.int, len(columns))
	sortable := make([]C.int, len(columns))
	for i, column := range columns {
		ids[i], titles[i] = column.ID, column.Title
		widths[i], sortable[i] = C.int(column.Width), boolInt(column.Sortable)
	}
	idBuffer, titleBuffer := joinNul(ids), joinNul(titles)
	result := &table{columns: append([]ui.TableColumn(nil), columns...), selection: selection}
	if owner == nil {
		owner = result
	}
	native := C.gkGTKTableCreate(window, boolInt(list), bytesPointer(idBuffer), C.size_t(len(idBuffer)),
		bytesPointer(titleBuffer), C.size_t(len(titleBuffer)), &widths[0], &sortable[0], C.int(selection))
	role := ui.RoleTable
	if list {
		role = ui.RoleList
	}
	if result.widgetBase, err = w.newComponent(native, role, owner); err != nil {
		return nil, err
	}
	return result, nil
}

// setRows replaces the table rows; cells are row-major by column.
func (t *table) setRows(ids []string, cells []string, details []string, icons []ui.IconSource) error {
	native := t.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	selected := t.SelectedIDs()
	idBuffer, cellBuffer, detailBuffer := joinNul(ids), joinNul(cells), joinNul(details)
	var iconPointer *unsafe.Pointer
	if icons != nil {
		array := newIconArray(icons, 16)
		defer array.free()
		iconPointer = array.native()
	}
	C.gkGTKTableSetRows(native, bytesPointer(idBuffer), C.size_t(len(idBuffer)), bytesPointer(cellBuffer),
		C.size_t(len(cellBuffer)), bytesPointer(detailBuffer), C.size_t(len(detailBuffer)), iconPointer,
		C.int(max(1, len(t.columns))))
	t.rowIDs = append([]string(nil), ids...)
	kept := slices.DeleteFunc(selected, func(id string) bool {
		return !slices.Contains(ids, id)
	})
	t.selectIDs(kept)
	return nil
}

func (t *table) SelectedIDs() []string {
	native := t.nativePointer()
	if native == nil {
		return nil
	}
	return readIDs(func(size *C.size_t) *C.char {
		return C.gkGTKTableSelection(native, size)
	})
}

func (t *table) selectIDs(ids []string) {
	if native := t.nativePointer(); native != nil {
		buffer := joinNul(ids)
		C.gkGTKTableSelect(native, bytesPointer(buffer), C.size_t(len(buffer)))
	}
}

func (t *table) SetSelectedIDs(ids []string) error {
	if err := ui.ValidateSelectedIDs(t.rowIDs, ids, t.selection); err != nil {
		return err
	}
	if t.nativePointer() == nil {
		return ui.ErrClosed
	}
	t.selectIDs(ids)
	return nil
}

func (t *table) ScrollToItem(id string) error {
	native := t.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	value := cString(id)
	defer freeCString(value)
	if C.gkGTKTableScrollTo(native, value) == 0 {
		return errors.New("ui: item does not exist")
	}
	return nil
}

func (t *table) OnSelectionChange(handler ui.SelectionIDsHandler) ui.Unsubscribe {
	return subscribe(&t.selects, handler, handler == nil)
}

func (t *table) OnActivate(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&t.activates, handler, handler == nil)
}

func (t *table) gtkEvent(event gtkEvent) {
	switch event.kind {
	case C.GK_EVENT_SELECTION:
		ids := t.SelectedIDs()
		each(&t.selects, func(handler ui.SelectionIDsHandler) {
			handler(append([]string(nil), ids...))
		})
	case C.GK_EVENT_ACTIVATE:
		id := event.text
		each(&t.activates, func(handler ui.ActionHandler) {
			handler(id)
		})
	case C.GK_EVENT_SORT:
		t.sort = ui.SortDescriptor{ColumnID: event.text, Ascending: event.a != 0}
		sort := t.sort
		each(&t.sorts, func(handler func(ui.SortDescriptor)) {
			handler(sort)
		})
	}
}

func (t *table) PreferredSize() ui.Size {
	return ui.Size{Width: 320, Height: 200}
}

type listView struct {
	*table
	items []ui.ListItem
}

// CreateListView implements ui.Window.CreateListView.
func (w *Window) CreateListView(options ui.ListViewOptions) (ui.ListView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &listView{}
	created, err := w.newTable(true, []ui.TableColumn{{ID: "text"}}, options.Selection, nil)
	if err != nil {
		return nil, err
	}
	result.table = created
	if err := result.SetItems(options.Items); err != nil {
		result.Close()
		return nil, err
	}
	if err := result.SetSelectedIDs(options.SelectedIDs); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (l *listView) Items() []ui.ListItem {
	return append([]ui.ListItem(nil), l.items...)
}

func (l *listView) SetItems(items []ui.ListItem) error {
	if err := (ui.ListViewOptions{Items: items}).Validate(); err != nil {
		return err
	}
	ids := make([]string, len(items))
	texts := make([]string, len(items))
	details := make([]string, len(items))
	for i, item := range items {
		ids[i], texts[i], details[i] = item.ID, item.Text, item.Detail
	}
	icons := imageIcons(len(items), func(i int) ui.IconSource {
		return items[i].Icon
	})
	if err := l.setRows(ids, texts, details, icons); err != nil {
		return err
	}
	l.items = append([]ui.ListItem(nil), items...)
	return nil
}

type tableView struct {
	*table
	rows []ui.TableRow
}

// CreateTableView implements ui.Window.CreateTableView.
func (w *Window) CreateTableView(options ui.TableViewOptions) (ui.TableView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &tableView{}
	created, err := w.newTable(false, options.Columns, options.Selection, nil)
	if err != nil {
		return nil, err
	}
	result.table = created
	if err := result.SetRows(options.Rows); err != nil {
		result.Close()
		return nil, err
	}
	if err := result.SetSelectedIDs(options.SelectedIDs); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (t *tableView) Columns() []ui.TableColumn {
	return append([]ui.TableColumn(nil), t.columns...)
}
func (t *tableView) Rows() []ui.TableRow {
	return append([]ui.TableRow(nil), t.rows...)
}

func (t *tableView) SetRows(rows []ui.TableRow) error {
	if err := (ui.TableViewOptions{Columns: t.columns, Rows: rows}).Validate(); err != nil {
		return err
	}
	ids := make([]string, len(rows))
	cells := make([]string, 0, len(rows)*len(t.columns))
	for i, row := range rows {
		ids[i] = row.ID
		for _, column := range t.columns {
			cells = append(cells, row.Values[column.ID])
		}
	}
	if err := t.setRows(ids, cells, nil, nil); err != nil {
		return err
	}
	t.rows = append([]ui.TableRow(nil), rows...)
	return nil
}

func (t *tableView) Sort() ui.SortDescriptor {
	return t.sort
}
func (t *tableView) OnSortChange(handler func(ui.SortDescriptor)) ui.Unsubscribe {
	return subscribe(&t.sorts, handler, handler == nil)
}
func (t *tableView) PreferredSize() ui.Size {
	width := 0
	for _, column := range t.columns {
		width += max(column.Width, 80)
	}
	return ui.Size{Width: max(width, 200), Height: 200}
}

// ---- Outline ----

type outlineNode struct {
	depth       int
	hasChildren bool
	group       bool
	id, text    string
	badge       string
	icon        ui.IconSource
}

type outline struct {
	*widgetBase
	selection ui.SelectionMode
	known     []string
	expands   listeners[ui.ActionHandler]
	selects   listeners[ui.SelectionIDsHandler]
}

func (w *Window) newOutline(navigation bool, selection ui.SelectionMode, role ui.AccessibilityRole, owner gtkReceiver) (*outline, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &outline{selection: selection}
	native := C.gkGTKOutlineCreate(window, boolInt(navigation), boolInt(selection == ui.SelectionMultiple))
	if result.widgetBase, err = w.newComponent(native, role, owner); err != nil {
		return nil, err
	}
	return result, nil
}

func (o *outline) setNodes(parent *string, nodes []outlineNode) error {
	native := o.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	depths := make([]C.int, max(1, len(nodes)))
	flags := make([]C.int, max(1, len(nodes)))
	ids := make([]string, len(nodes))
	texts := make([]string, len(nodes))
	badges := make([]string, len(nodes))
	icons := make([]ui.IconSource, len(nodes))
	for i, node := range nodes {
		depths[i] = C.int(node.depth)
		if node.hasChildren {
			flags[i] |= 1
		}
		if node.group {
			flags[i] |= 2
		}
		ids[i], texts[i], badges[i], icons[i] = node.id, node.text, node.badge, node.icon
	}
	idBuffer, textBuffer, badgeBuffer := joinNul(ids), joinNul(texts), joinNul(badges)
	array := newIconArray(icons, 16)
	defer array.free()
	var parentValue *C.char
	if parent != nil {
		parentValue = cString(*parent)
		defer freeCString(parentValue)
	}
	C.gkGTKOutlineSetNodes(native, parentValue, &depths[0], &flags[0], bytesPointer(idBuffer), C.size_t(len(idBuffer)),
		bytesPointer(textBuffer), C.size_t(len(textBuffer)), bytesPointer(badgeBuffer), C.size_t(len(badgeBuffer)),
		array.native(), C.int(len(nodes)))
	return nil
}

func (o *outline) SelectedIDs() []string {
	native := o.nativePointer()
	if native == nil {
		return nil
	}
	return readIDs(func(size *C.size_t) *C.char {
		return C.gkGTKOutlineSelection(native, size)
	})
}

func (o *outline) SetSelectedIDs(ids []string) error {
	if err := ui.ValidateSelectedIDs(o.known, ids, o.selection); err != nil {
		return err
	}
	native := o.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	buffer := joinNul(ids)
	C.gkGTKOutlineSelect(native, bytesPointer(buffer), C.size_t(len(buffer)))
	return nil
}

func (o *outline) OnSelectionChange(handler ui.SelectionIDsHandler) ui.Unsubscribe {
	return subscribe(&o.selects, handler, handler == nil)
}

func (o *outline) gtkEvent(event gtkEvent) {
	switch event.kind {
	case C.GK_EVENT_SELECTION:
		ids := o.SelectedIDs()
		each(&o.selects, func(handler ui.SelectionIDsHandler) {
			handler(append([]string(nil), ids...))
		})
	case C.GK_EVENT_EXPAND:
		id := event.text
		each(&o.expands, func(handler ui.ActionHandler) {
			handler(id)
		})
	}
}

func (o *outline) PreferredSize() ui.Size {
	return ui.Size{Width: 240, Height: 300}
}

type treeView struct {
	*outline
	roots []ui.TreeNode
}

func flattenTree(nodes []ui.TreeNode, depth int, out *[]outlineNode, ids *[]string) error {
	for _, node := range nodes {
		if node.ID == "" {
			return errors.New("ui: tree node has no ID")
		}
		if slices.Contains(*ids, node.ID) {
			return errors.New("ui: duplicate tree node ID " + node.ID)
		}
		*ids = append(*ids, node.ID)
		*out = append(*out, outlineNode{
			depth: depth, id: node.ID, text: node.Text, icon: node.Icon,
			hasChildren: node.HasChildren || len(node.Children) > 0,
		})
		if err := flattenTree(node.Children, depth+1, out, ids); err != nil {
			return err
		}
	}
	return nil
}

// CreateTreeView implements ui.Window.CreateTreeView.
func (w *Window) CreateTreeView(options ui.TreeViewOptions) (ui.TreeView, error) {
	result := &treeView{}
	created, err := w.newOutline(false, options.Selection, ui.RoleTree, result)
	if err != nil {
		return nil, err
	}
	result.outline = created
	if err := result.SetRoots(options.Roots); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (t *treeView) gtkEvent(event gtkEvent) {
	t.outline.gtkEvent(event)
}

func (t *treeView) Roots() []ui.TreeNode {
	return append([]ui.TreeNode(nil), t.roots...)
}

func (t *treeView) SetRoots(roots []ui.TreeNode) error {
	var nodes []outlineNode
	var ids []string
	if err := flattenTree(roots, 0, &nodes, &ids); err != nil {
		return err
	}
	if err := t.setNodes(nil, nodes); err != nil {
		return err
	}
	t.roots = append([]ui.TreeNode(nil), roots...)
	t.known = ids
	return nil
}

// replaceChildren returns roots with the children of id replaced.
func replaceChildren(nodes []ui.TreeNode, id string, children []ui.TreeNode) ([]ui.TreeNode, bool) {
	result := append([]ui.TreeNode(nil), nodes...)
	for i := range result {
		if result[i].ID == id {
			result[i].Children = append([]ui.TreeNode(nil), children...)
			result[i].HasChildren = len(children) > 0
			return result, true
		}
		if updated, found := replaceChildren(result[i].Children, id, children); found {
			result[i].Children = updated
			return result, true
		}
	}
	return result, false
}

func (t *treeView) SetChildren(id string, children []ui.TreeNode) error {
	roots, found := replaceChildren(t.roots, id, children)
	if !found {
		return errors.New("ui: tree node does not exist")
	}
	var nodes []outlineNode
	var ids []string
	if err := flattenTree(roots, 0, &nodes, &ids); err != nil {
		return err
	}
	var subtree []outlineNode
	var subtreeIDs []string
	if err := flattenTree(children, 0, &subtree, &subtreeIDs); err != nil {
		return err
	}
	if err := t.setNodes(&id, subtree); err != nil {
		return err
	}
	t.roots = roots
	t.known = ids
	return nil
}

func (t *treeView) IsExpanded(id string) bool {
	native := t.nativePointer()
	if native == nil {
		return false
	}
	value := cString(id)
	defer freeCString(value)
	return C.gkGTKOutlineExpanded(native, value) != 0
}

func (t *treeView) SetExpanded(id string, expanded bool) error {
	native := t.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	value := cString(id)
	defer freeCString(value)
	if C.gkGTKOutlineSetExpanded(native, value, boolInt(expanded)) == 0 {
		return errors.New("ui: tree node does not exist")
	}
	return nil
}

func (t *treeView) OnExpand(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&t.expands, handler, handler == nil)
}

type navigationList struct {
	*outline
	sections []ui.NavigationSection
	changes  listeners[ui.ActionHandler]
}

// CreateNavigationList implements ui.Window.CreateNavigationList.
func (w *Window) CreateNavigationList(options ui.NavigationListOptions) (ui.NavigationList, error) {
	result := &navigationList{}
	created, err := w.newOutline(true, ui.SelectionSingle, ui.RoleNavigation, result)
	if err != nil {
		return nil, err
	}
	result.outline = created
	if err := result.SetSections(options.Sections); err != nil {
		result.Close()
		return nil, err
	}
	if options.SelectedID != "" {
		if err := result.SetSelectedID(options.SelectedID); err != nil {
			result.Close()
			return nil, err
		}
	}
	return result, nil
}

func (n *navigationList) gtkEvent(event gtkEvent) {
	n.outline.gtkEvent(event)
	if event.kind == C.GK_EVENT_SELECTION {
		id := n.SelectedID()
		each(&n.changes, func(handler ui.ActionHandler) {
			handler(id)
		})
	}
}

func (n *navigationList) Sections() []ui.NavigationSection {
	return append([]ui.NavigationSection(nil), n.sections...)
}

func (n *navigationList) SetSections(sections []ui.NavigationSection) error {
	var nodes []outlineNode
	var ids []string
	for _, section := range sections {
		if section.Title != "" {
			nodes = append(nodes, outlineNode{id: "section:" + section.ID, text: section.Title, group: true, hasChildren: true})
		}
		depth := 0
		if section.Title != "" {
			depth = 1
		}
		for _, item := range section.Items {
			if item.ID == "" || slices.Contains(ids, item.ID) {
				return errors.New("ui: navigation items need unique IDs")
			}
			ids = append(ids, item.ID)
			nodes = append(nodes, outlineNode{depth: depth, id: item.ID, text: item.Text, icon: item.Icon, badge: item.Badge})
		}
	}
	if err := n.setNodes(nil, nodes); err != nil {
		return err
	}
	n.sections = append([]ui.NavigationSection(nil), sections...)
	n.known = ids
	return nil
}

func (n *navigationList) SelectedID() string {
	if ids := n.SelectedIDs(); len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func (n *navigationList) SetSelectedID(id string) error {
	if id == "" {
		return n.SetSelectedIDs(nil)
	}
	return n.SetSelectedIDs([]string{id})
}

func (n *navigationList) OnChange(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&n.changes, handler, handler == nil)
}

// ---- Collection view ----

type collectionView struct {
	*widgetBase
	items     []ui.CollectionItem
	selection ui.SelectionMode
	selects   listeners[ui.SelectionIDsHandler]
	activates listeners[ui.ActionHandler]
}

// CreateCollectionView implements ui.Window.CreateCollectionView.
func (w *Window) CreateCollectionView(options ui.CollectionViewOptions) (ui.CollectionView, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &collectionView{selection: options.Selection}
	native := C.gkGTKCollectionCreate(window, C.int(options.ItemSize.Width), C.int(options.ItemSize.Height), C.int(options.Selection))
	if result.widgetBase, err = w.newComponent(native, ui.RoleGrid, result); err != nil {
		return nil, err
	}
	if err := result.SetItems(options.Items); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (c *collectionView) Items() []ui.CollectionItem {
	return append([]ui.CollectionItem(nil), c.items...)
}

func (c *collectionView) ids(items []ui.CollectionItem) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func (c *collectionView) SetItems(items []ui.CollectionItem) error {
	ids := c.ids(items)
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return errors.New("ui: collection items need unique IDs")
		}
		seen[id] = true
	}
	native := c.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = item.Text
	}
	icons := imageIcons(len(items), func(i int) ui.IconSource {
		return ui.IconSource{Image: items[i].Image}
	})
	array := newIconArray(icons, 0)
	defer array.free()
	idBuffer, textBuffer := joinNul(ids), joinNul(texts)
	C.gkGTKCollectionSetItems(native, bytesPointer(idBuffer), C.size_t(len(idBuffer)), bytesPointer(textBuffer),
		C.size_t(len(textBuffer)), array.native(), C.int(len(items)))
	c.items = append([]ui.CollectionItem(nil), items...)
	return nil
}

func (c *collectionView) SelectedIDs() []string {
	native := c.nativePointer()
	if native == nil {
		return nil
	}
	return readIDs(func(size *C.size_t) *C.char {
		return C.gkGTKCollectionSelection(native, size)
	})
}

func (c *collectionView) SetSelectedIDs(ids []string) error {
	if err := ui.ValidateSelectedIDs(c.ids(c.items), ids, c.selection); err != nil {
		return err
	}
	native := c.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	buffer := joinNul(ids)
	C.gkGTKCollectionSelect(native, bytesPointer(buffer), C.size_t(len(buffer)))
	return nil
}

func (c *collectionView) OnSelectionChange(handler ui.SelectionIDsHandler) ui.Unsubscribe {
	return subscribe(&c.selects, handler, handler == nil)
}

func (c *collectionView) OnActivate(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&c.activates, handler, handler == nil)
}

func (c *collectionView) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_SELECTION {
		ids := c.SelectedIDs()
		each(&c.selects, func(handler ui.SelectionIDsHandler) {
			handler(append([]string(nil), ids...))
		})
	}
}

func (c *collectionView) PreferredSize() ui.Size {
	return ui.Size{Width: 320, Height: 240}
}

// ---- Breadcrumbs ----

type breadcrumbs struct {
	*widgetBase
	items     []ui.BreadcrumbItem
	activates listeners[ui.ActionHandler]
}

// CreateBreadcrumbs implements ui.Window.CreateBreadcrumbs.
func (w *Window) CreateBreadcrumbs(options ui.BreadcrumbsOptions) (ui.Breadcrumbs, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &breadcrumbs{}
	if result.widgetBase, err = w.newComponent(C.gkGTKBreadcrumbsCreate(window), ui.RoleNavigation, result); err != nil {
		return nil, err
	}
	if err := result.SetItems(options.Items); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (b *breadcrumbs) Items() []ui.BreadcrumbItem {
	return append([]ui.BreadcrumbItem(nil), b.items...)
}

func (b *breadcrumbs) SetItems(items []ui.BreadcrumbItem) error {
	titles := make([]string, len(items))
	seen := map[string]bool{}
	for i, item := range items {
		if item.ID == "" || seen[item.ID] {
			return errors.New("ui: breadcrumb items need unique IDs")
		}
		seen[item.ID] = true
		titles[i] = item.Text
	}
	native := b.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	buffer := joinNul(titles)
	C.gkGTKBreadcrumbsSetItems(native, bytesPointer(buffer), C.size_t(len(buffer)))
	b.items = append([]ui.BreadcrumbItem(nil), items...)
	return nil
}

func (b *breadcrumbs) OnActivate(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&b.activates, handler, handler == nil)
}

func (b *breadcrumbs) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_ACTIVATE && event.a >= 0 && event.a < len(b.items) {
		id := b.items[event.a].ID
		each(&b.activates, func(handler ui.ActionHandler) {
			handler(id)
		})
	}
}

// ---- Pagination ----

type pagination struct {
	*widgetBase
	count, selected int
	changes         listeners[ui.SelectionChangeHandler]
}

// CreatePagination implements ui.Window.CreatePagination.
func (w *Window) CreatePagination(options ui.PaginationOptions) (ui.Pagination, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &pagination{count: options.PageCount, selected: options.SelectedPage}
	native := C.gkGTKPaginationCreate(window, C.int(options.PageCount), C.int(options.SelectedPage), boolInt(options.Compact))
	if result.widgetBase, err = w.newComponent(native, ui.RoleNavigation, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *pagination) PageCount() int {
	return p.count
}
func (p *pagination) SelectedPage() int {
	return p.selected
}
func (p *pagination) apply() {
	if native := p.nativePointer(); native != nil {
		C.gkGTKPaginationSet(native, C.int(p.count), C.int(p.selected))
	}
}
func (p *pagination) SetPageCount(count int) error {
	if count <= 0 {
		return errors.New("ui: pagination requires at least one page")
	}
	p.count = count
	p.selected = min(p.selected, count-1)
	p.apply()
	return nil
}
func (p *pagination) SetSelectedPage(page int) error {
	if err := (ui.PaginationOptions{PageCount: p.count, SelectedPage: page}).Validate(); err != nil {
		return err
	}
	p.selected = page
	p.apply()
	return nil
}
func (p *pagination) OnChange(handler ui.SelectionChangeHandler) ui.Unsubscribe {
	return subscribe(&p.changes, handler, handler == nil)
}
func (p *pagination) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_SELECTION {
		p.selected = event.a
		page := p.selected
		each(&p.changes, func(handler ui.SelectionChangeHandler) {
			handler(page)
		})
	}
}

// ---- Command palette ----

// commandPalette is a search field above a list of matching commands.
type commandPalette struct {
	*frameContainer
	search    ui.SearchField
	list      *listView
	commands  []ui.Command
	activates listeners[ui.ActionHandler]
}

// CreateCommandPalette implements ui.Window.CreateCommandPalette.
func (w *Window) CreateCommandPalette(options ui.CommandPaletteOptions) (ui.CommandPalette, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &commandPalette{commands: append([]ui.Command(nil), options.Commands...)}
	container, err := w.newFrameContainer(ui.RoleDialog, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	placeholder := options.Placeholder
	if placeholder == "" {
		placeholder = "Search commands"
	}
	search, err := w.CreateSearchField(ui.SearchFieldOptions{Placeholder: placeholder})
	if err != nil {
		container.Close()
		return nil, err
	}
	list, err := w.CreateListView(ui.ListViewOptions{Selection: ui.SelectionSingle})
	if err != nil {
		search.Close()
		container.Close()
		return nil, err
	}
	result.search, result.list = search, list.(*listView)
	if err := container.setChildren([]ui.Widget{search, list}); err != nil {
		search.Close()
		list.Close()
		container.Close()
		return nil, err
	}
	container.arrange = func(size ui.Size) {
		height := search.PreferredSize().Height
		search.SetBounds(ui.Rect{Size: ui.Size{Width: size.Width, Height: height}})
		list.SetBounds(ui.Rect{Position: ui.Point{Y: height + 6}, Size: ui.Size{Width: size.Width, Height: max(0, size.Height-height-6)}})
	}
	container.preferred = func() ui.Size {
		return ui.Size{Width: 420, Height: 280}
	}
	search.OnChange(func(string) {
		result.filter()
	})
	search.OnSubmit(func(string) {
		result.activateSelection()
	})
	list.OnActivate(func(id string) {
		result.activate(id)
	})
	result.filter()
	return result, nil
}

func (c *commandPalette) Commands() []ui.Command {
	return append([]ui.Command(nil), c.commands...)
}

func (c *commandPalette) SetCommands(commands []ui.Command) error {
	if err := (ui.CommandPaletteOptions{Commands: commands}).Validate(); err != nil {
		return err
	}
	c.commands = append([]ui.Command(nil), commands...)
	c.filter()
	return nil
}

func (c *commandPalette) Query() string {
	return c.search.Text()
}

func (c *commandPalette) SetQuery(query string) {
	c.search.SetText(query)
	c.filter()
}

// filter shows enabled commands whose text contains every word of the query.
func (c *commandPalette) filter() {
	words := strings.Fields(strings.ToLower(c.search.Text()))
	var items []ui.ListItem
	for _, command := range c.commands {
		if !command.Enabled {
			continue
		}
		text := strings.ToLower(command.Text + " " + command.Detail)
		if slices.ContainsFunc(words, func(word string) bool {
			return !strings.Contains(text, word)
		}) {
			continue
		}
		detail := command.Detail
		if shortcut := shortcutText(command.Shortcut); shortcut != "" {
			detail = strings.TrimSpace(detail + "  " + shortcut)
		}
		items = append(items, ui.ListItem{ID: command.ID, Text: command.Text, Detail: detail})
	}
	_ = c.list.SetItems(items)
	if len(items) > 0 {
		_ = c.list.SetSelectedIDs([]string{items[0].ID})
	}
}

func (c *commandPalette) activateSelection() {
	if ids := c.list.SelectedIDs(); len(ids) > 0 {
		c.activate(ids[0])
	}
}

func (c *commandPalette) activate(id string) {
	each(&c.activates, func(handler ui.ActionHandler) {
		handler(id)
	})
}

func (c *commandPalette) OnActivate(handler ui.ActionHandler) ui.Unsubscribe {
	return subscribe(&c.activates, handler, handler == nil)
}

var (
	_ ui.ListView       = (*listView)(nil)
	_ ui.TableView      = (*tableView)(nil)
	_ ui.TreeView       = (*treeView)(nil)
	_ ui.NavigationList = (*navigationList)(nil)
	_ ui.CollectionView = (*collectionView)(nil)
	_ ui.Breadcrumbs    = (*breadcrumbs)(nil)
	_ ui.Pagination     = (*pagination)(nil)
	_ ui.CommandPalette = (*commandPalette)(nil)
)
