package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bluescreen10/gamekit/ui"
)

func collectionCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("List views", "Single and multiple selection. Double-click or press Return to activate an item.", g.listViewsDemo),
		g.card("Table view", "Click a column header to sort. The rows are sorted by the application.", g.tableViewDemo),
		g.card("Tree view", "Folders load their children when you expand them.", g.treeViewDemo),
		g.card("Collection view", "A grid of thumbnails with multiple selection.", g.collectionViewDemo),
		g.card("Property inspector", "Grouped editors, like the inspector of a scene editor.", g.propertyInspectorDemo),
	}
}

func navigationCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Navigation list", "The sidebar of this window is a navigation list. This one has badges.", g.navigationListDemo),
		g.card("Breadcrumbs", "Click a parent to go back to it.", g.breadcrumbsDemo),
		g.card("Pagination", "Full and compact presentations.", g.paginationDemo),
		g.card("Command palette", "Type to filter the commands. Press Return to run one.", g.commandPaletteDemo),
	}
}

func (g *gallery) listViewsDemo() ui.Widget {
	icons := []string{"document", "image", "folder", "star"}
	items := make([]ui.ListItem, 40)
	for i := range items {
		items[i] = ui.ListItem{
			ID:     fmt.Sprintf("item-%02d", i+1),
			Text:   fmt.Sprintf("Item %d", i+1),
			Detail: fmt.Sprintf("Detail text for item %d", i+1),
			Icon:   ui.IconSource{SystemName: icons[i%len(icons)]},
		}
	}
	single := must(g.window.CreateListView(ui.ListViewOptions{Items: items, Selection: ui.SelectionSingle, SelectedIDs: []string{"item-01"}}))
	single.SetMinimumSize(ui.Size{Height: 200})
	single.OnSelectionChange(func(ids []string) {
		g.log("Single list: [%s]", strings.Join(ids, ", "))
	})
	single.OnActivate(func(id string) {
		g.log("Activated %s", id)
	})

	multiple := must(g.window.CreateListView(ui.ListViewOptions{Items: items[:12], Selection: ui.SelectionMultiple}))
	multiple.SetMinimumSize(ui.Size{Height: 200})
	multiple.OnSelectionChange(func(ids []string) {
		g.log("Multiple list: %d selected", len(ids))
	})

	last := g.button("Scroll to the last item", func() {
		g.attempt("Scroll", func() error {
			return single.ScrollToItem("item-40")
		})
		g.attempt("Select", func() error {
			return single.SetSelectedIDs([]string{"item-40"})
		})
	})
	remove := g.button("Remove selected", func() {
		selected := multiple.SelectedIDs()
		kept := slices.DeleteFunc(multiple.Items(), func(item ui.ListItem) bool {
			return slices.Contains(selected, item.ID)
		})
		if err := multiple.SetItems(kept); err != nil {
			g.log("List: %v", err)
		}
	})
	lists := g.layout(ui.Row(
		ui.Item(g.column(g.label("Single selection"), single)).Grow(1),
		ui.Item(g.column(g.label("Multiple selection"), multiple)).Grow(1),
	).WithGap(16))
	return g.column(lists, g.row(last, remove, g.enabledToggle(single, multiple)))
}

// asset is a row of the table demo.
type asset struct {
	name string
	kind string
	size int
}

func (g *gallery) tableViewDemo() ui.Widget {
	assets := []asset{
		{"hero.png", "Texture", 2048}, {"theme.ogg", "Audio", 5120}, {"level1.map", "Map", 768},
		{"player.glb", "Model", 3400}, {"ui.atlas", "Texture", 1024}, {"jump.wav", "Audio", 96},
		{"boss.glb", "Model", 8800}, {"intro.mp4", "Video", 40960}, {"font.ttf", "Font", 320},
	}
	toRows := func() []ui.TableRow {
		rows := make([]ui.TableRow, len(assets))
		for i, entry := range assets {
			rows[i] = ui.TableRow{ID: entry.name, Values: map[string]string{
				"name": entry.name,
				"kind": entry.kind,
				"size": strconv.Itoa(entry.size) + " KB",
			}}
		}
		return rows
	}
	table := must(g.window.CreateTableView(ui.TableViewOptions{
		Columns: []ui.TableColumn{
			{ID: "name", Title: "Name", Width: 200, Sortable: true},
			{ID: "kind", Title: "Type", Width: 120, Sortable: true},
			{ID: "size", Title: "Size", Width: 100, Sortable: true},
		},
		Rows:      toRows(),
		Selection: ui.SelectionMultiple,
	}))
	table.SetMinimumSize(ui.Size{Height: 220})
	table.OnSortChange(func(sort ui.SortDescriptor) {
		slices.SortStableFunc(assets, func(a, b asset) int {
			order := 0
			switch sort.ColumnID {
			case "name":
				order = strings.Compare(a.name, b.name)
			case "kind":
				order = strings.Compare(a.kind, b.kind)
			case "size":
				order = a.size - b.size
			}
			if !sort.Ascending {
				order = -order
			}
			return order
		})
		if err := table.SetRows(toRows()); err != nil {
			g.log("Table: %v", err)
		}
		g.log("Sorted by %s, ascending %t", sort.ColumnID, sort.Ascending)
	})
	table.OnSelectionChange(func(ids []string) {
		g.log("Table selection: [%s]", strings.Join(ids, ", "))
	})
	table.OnActivate(func(id string) {
		g.log("Opened %s", id)
	})
	return g.column(table, g.row(g.enabledToggle(table)))
}

func (g *gallery) treeViewDemo() ui.Widget {
	folder := func(id, text string) ui.TreeNode {
		return ui.TreeNode{ID: id, Text: text, Icon: ui.IconSource{SystemName: "folder"}, HasChildren: true}
	}
	roots := []ui.TreeNode{folder("assets", "assets"), folder("scripts", "scripts"), folder("scenes", "scenes")}
	tree := must(g.window.CreateTreeView(ui.TreeViewOptions{Roots: roots, Selection: ui.SelectionSingle}))
	tree.SetMinimumSize(ui.Size{Height: 220})
	loaded := map[string]bool{}
	load := func(id string) {
		if loaded[id] {
			return
		}
		loaded[id] = true
		children := []ui.TreeNode{}
		if strings.Count(id, "/") < 2 {
			children = append(children, folder(id+"/nested", "nested"))
		}
		for i := 1; i <= 3; i++ {
			name := fmt.Sprintf("file%d.txt", i)
			children = append(children, ui.TreeNode{ID: id + "/" + name, Text: name, Icon: ui.IconSource{SystemName: "document"}})
		}
		if err := tree.SetChildren(id, children); err != nil {
			g.log("Tree: %v", err)
		}
	}
	tree.OnExpand(func(id string) {
		load(id)
		g.log("Expanded %s", id)
	})
	tree.OnSelectionChange(func(ids []string) {
		g.log("Tree selection: [%s]", strings.Join(ids, ", "))
	})
	expand := g.button("Expand assets", func() {
		load("assets")
		g.attempt("Expand", func() error {
			return tree.SetExpanded("assets", true)
		})
		g.log("assets expanded: %t", tree.IsExpanded("assets"))
	})
	collapse := g.button("Collapse assets", func() {
		g.attempt("Collapse", func() error {
			return tree.SetExpanded("assets", false)
		})
	})
	return g.column(tree, g.row(expand, collapse))
}

func (g *gallery) collectionViewDemo() ui.Widget {
	items := make([]ui.CollectionItem, 16)
	for i := range items {
		items[i] = ui.CollectionItem{
			ID:    fmt.Sprintf("swatch-%d", i),
			Text:  fmt.Sprintf("Hue %d°", i*360/len(items)),
			Image: swatchImage(64, float64(i)/float64(len(items))),
		}
	}
	view := must(g.window.CreateCollectionView(ui.CollectionViewOptions{
		Items:     items,
		ItemSize:  ui.Size{Width: 96, Height: 96},
		Selection: ui.SelectionMultiple,
	}))
	view.SetMinimumSize(ui.Size{Height: 240})
	view.OnSelectionChange(func(ids []string) {
		g.log("Collection selection: %d items", len(ids))
	})
	view.OnActivate(func(id string) {
		g.log("Opened %s", id)
	})
	first := g.button("Select first two", func() {
		g.attempt("Select", func() error {
			return view.SetSelectedIDs([]string{"swatch-0", "swatch-1"})
		})
	})
	return g.column(view, g.row(first))
}

func (g *gallery) propertyInspectorDemo() ui.Widget {
	number := func(value float64) ui.Widget {
		return must(g.window.CreateNumberField(ui.NumberFieldOptions{Minimum: -1000, Maximum: 1000, Value: value, Step: 1, Decimals: 1}))
	}
	visible := must(g.window.CreateSwitch(ui.SwitchOptions{Checked: true}))
	tint := g.optional(func() ui.Widget {
		return must(g.window.CreateColorPicker(ui.ColorPickerOptions{}))
	})
	layer := must(g.window.CreateSelect(ui.SelectOptions{Items: []string{"Background", "World", "Foreground", "UI"}, SelectedIndex: 1}))
	inspector := must(g.window.CreatePropertyInspector(ui.PropertyInspectorOptions{Groups: []ui.PropertyGroup{
		{ID: "transform", Title: "Transform", Rows: []ui.PropertyRow{
			{ID: "x", Label: "X", Editor: number(120)},
			{ID: "y", Label: "Y", Editor: number(64)},
			{ID: "rotation", Label: "Rotation", Editor: number(45)},
		}},
		{ID: "appearance", Title: "Appearance", Rows: []ui.PropertyRow{
			{ID: "visible", Label: "Visible", Editor: visible},
			{ID: "tint", Label: "Tint", Editor: tint},
			{ID: "layer", Label: "Layer", Editor: layer},
		}},
	}}))
	collapse := g.checkBox("Transform expanded", true, func(checked bool) {
		if err := inspector.SetGroupExpanded("transform", checked); err != nil {
			g.log("Inspector: %v", err)
		}
	})
	return g.column(inspector, g.row(collapse))
}

func (g *gallery) navigationListDemo() ui.Widget {
	list := must(g.window.CreateNavigationList(ui.NavigationListOptions{
		Sections: []ui.NavigationSection{
			{ID: "mail", Title: "Mailboxes", Items: []ui.NavigationItem{
				{ID: "inbox", Text: "Inbox", Icon: ui.IconSource{SystemName: "download"}, Badge: "12"},
				{ID: "sent", Text: "Sent", Icon: ui.IconSource{SystemName: "upload"}},
				{ID: "trash", Text: "Trash", Icon: ui.IconSource{SystemName: "delete"}, Badge: "3"},
			}},
			{ID: "tags", Title: "Tags", Items: []ui.NavigationItem{
				{ID: "starred", Text: "Starred", Icon: ui.IconSource{SystemName: "star"}},
				{ID: "favorites", Text: "Favorites", Icon: ui.IconSource{SystemName: "heart"}},
			}},
		},
		SelectedID: "inbox",
	}))
	fixed(list, ui.Size{Width: 240, Height: 280})
	list.OnChange(func(id string) {
		g.log("Mailbox: %s", id)
	})
	trash := g.button("Select trash", func() {
		g.attempt("Select", func() error {
			return list.SetSelectedID("trash")
		})
	})
	return g.row(list, trash)
}

func (g *gallery) breadcrumbsDemo() ui.Widget {
	path := []ui.BreadcrumbItem{
		{ID: "home", Text: "Home"},
		{ID: "projects", Text: "Projects"},
		{ID: "game", Text: "Space Game"},
		{ID: "levels", Text: "Levels"},
		{ID: "level1", Text: "Level 1"},
	}
	crumbs := must(g.window.CreateBreadcrumbs(ui.BreadcrumbsOptions{Items: path}))
	crumbs.OnActivate(func(id string) {
		items := crumbs.Items()
		for i, item := range items {
			if item.ID == id {
				items = items[:i+1]
				break
			}
		}
		if err := crumbs.SetItems(items); err != nil {
			g.log("Breadcrumbs: %v", err)
		}
		g.log("Navigated to %s", id)
	})
	reset := g.button("Reset path", func() {
		if err := crumbs.SetItems(path); err != nil {
			g.log("Breadcrumbs: %v", err)
		}
	})
	return g.column(crumbs, g.row(reset))
}

func (g *gallery) paginationDemo() ui.Widget {
	full := must(g.window.CreatePagination(ui.PaginationOptions{PageCount: 12, SelectedPage: 0}))
	compact := must(g.window.CreatePagination(ui.PaginationOptions{PageCount: 12, SelectedPage: 0, Compact: true}))
	page := g.value("Page 1 of 12")
	sync := func(index int, other ui.Pagination) {
		g.attempt("Page", func() error {
			return other.SetSelectedPage(index)
		})
		page.SetText(fmt.Sprintf("Page %d of %d", index+1, full.PageCount()))
	}
	full.OnChange(func(index int) {
		sync(index, compact)
	})
	compact.OnChange(func(index int) {
		sync(index, full)
	})
	fewer := g.button("Three pages", func() {
		for _, pagination := range []ui.Pagination{full, compact} {
			g.attempt("Page count", func() error {
				return pagination.SetPageCount(3)
			})
		}
		page.SetText(fmt.Sprintf("Page %d of %d", full.SelectedPage()+1, full.PageCount()))
	})
	return g.column(
		g.labeled("Full", full),
		g.labeled("Compact", compact),
		g.row(page, fewer, g.enabledToggle(full, compact)),
	)
}

func (g *gallery) commandPaletteDemo() ui.Widget {
	palette := must(g.window.CreateCommandPalette(ui.CommandPaletteOptions{
		Placeholder: "Type a command",
		Commands: []ui.Command{
			{ID: "open", Text: "Open file", Detail: "Open a file from disk", Shortcut: ui.Shortcut{Key: "O", Command: true}, Enabled: true},
			{ID: "save", Text: "Save", Detail: "Save the current scene", Shortcut: ui.Shortcut{Key: "S", Command: true}, Enabled: true},
			{ID: "build", Text: "Build project", Detail: "Compile scripts and pack assets", Shortcut: ui.Shortcut{Key: "B", Command: true}, Enabled: true},
			{ID: "run", Text: "Run game", Detail: "Start the game in a new window", Shortcut: ui.Shortcut{Key: "R", Command: true}, Enabled: true},
			{ID: "profile", Text: "Start profiler", Detail: "Not available in this build", Enabled: false},
			{ID: "theme", Text: "Toggle theme", Detail: "Switch between light and dark"},
		},
	}))
	palette.SetMinimumSize(ui.Size{Height: 220})
	palette.OnActivate(func(id string) {
		g.log("Command: %s (query %q)", id, palette.Query())
	})
	query := g.button("Search for “build”", func() {
		palette.SetQuery("build")
	})
	return g.column(palette, g.row(query))
}
