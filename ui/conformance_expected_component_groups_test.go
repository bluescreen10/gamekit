package ui_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func checkExpectedInfoBar(r *recorder, window ui.Window) {
	info, err := window.CreateInfoBar(ui.InfoBarOptions{
		Severity: ui.SeverityWarning,
		Title:    "Renderer restarted",
		Detail:   "The GPU device changed.",
		Actions:  []ui.Action{{ID: "details", Text: "Details", Enabled: true}},
		Closable: true,
		Live:     ui.LivePolite,
	})
	r.check("info_bar/create", func() error {
		return err
	})
	if info == nil {
		return
	}
	defer info.Close()
	r.check("info_bar/state", func() error {
		if info.Severity() != ui.SeverityWarning || info.Title() != "Renderer restarted" {
			return errors.New("info bar did not preserve its initial state")
		}
		info.SetVisible(false)
		if info.IsVisible() {
			return errors.New("info bar remained visible")
		}
		return nil
	})
}

func checkExpectedDataComponents(r *recorder, window ui.Window) {
	list, listErr := window.CreateListView(ui.ListViewOptions{
		Items: []ui.ListItem{
			{ID: "one", Text: "One"},
			{ID: "two", Text: "Two"},
		},
		Selection: ui.SelectionSingle,
	})
	checkExpectedCreation(r, "list_view", list, listErr)

	table, tableErr := window.CreateTableView(ui.TableViewOptions{
		Columns: []ui.TableColumn{{ID: "name", Title: "Name", Width: 160}},
		Rows:    []ui.TableRow{{ID: "one", Values: map[string]string{"name": "One"}}},
	})
	checkExpectedCreation(r, "table_view", table, tableErr)

	tree, treeErr := window.CreateTreeView(ui.TreeViewOptions{
		Roots: []ui.TreeNode{{ID: "root", Text: "Root", HasChildren: true}},
	})
	checkExpectedCreation(r, "tree_view", tree, treeErr)

	collection, collectionErr := window.CreateCollectionView(ui.CollectionViewOptions{
		Items: []ui.CollectionItem{{ID: "one", Text: "One"}},
	})
	checkExpectedCreation(r, "collection_view", collection, collectionErr)

	navigation, navigationErr := window.CreateNavigationList(ui.NavigationListOptions{
		Sections: []ui.NavigationSection{{
			ID:    "main",
			Title: "Main",
			Items: []ui.NavigationItem{{ID: "home", Text: "Home"}},
		}},
		SelectedID: "home",
	})
	checkExpectedCreation(r, "navigation_list", navigation, navigationErr)

	breadcrumbs, breadcrumbsErr := window.CreateBreadcrumbs(ui.BreadcrumbsOptions{
		Items: []ui.BreadcrumbItem{{ID: "root", Text: "Root"}, {ID: "child", Text: "Child"}},
	})
	checkExpectedCreation(r, "breadcrumbs", breadcrumbs, breadcrumbsErr)

	pagination, paginationErr := window.CreatePagination(ui.PaginationOptions{
		PageCount:    10,
		SelectedPage: 2,
		Compact:      true,
	})
	checkExpectedCreation(r, "pagination", pagination, paginationErr)

	palette, paletteErr := window.CreateCommandPalette(ui.CommandPaletteOptions{
		Commands: []ui.Command{{ID: "open", Text: "Open", Enabled: true}},
	})
	checkExpectedCreation(r, "command_palette", palette, paletteErr)

	editor, editorErr := window.CreateTextField()
	if editorErr == nil {
		inspector, inspectorErr := window.CreatePropertyInspector(ui.PropertyInspectorOptions{
			Groups: []ui.PropertyGroup{{
				ID:    "transform",
				Title: "Transform",
				Rows: []ui.PropertyRow{{
					ID:     "name",
					Label:  "Name",
					Editor: editor,
				}},
			}},
		})
		checkExpectedCreation(r, "property_inspector", inspector, inspectorErr)
	}
}

func checkExpectedContainers(r *recorder, window ui.Window) {
	first, firstErr := window.CreateLabel("First")
	second, secondErr := window.CreateLabel("Second")
	if firstErr != nil || secondErr != nil {
		r.check("containers/create_children", func() error {
			return errors.Join(firstErr, secondErr)
		})
		return
	}

	split, splitErr := window.CreateSplitView(ui.SplitViewOptions{
		Direction: ui.Horizontal,
		Panes: []ui.SplitPane{
			{Content: first, Minimum: 100},
			{Content: second, Minimum: 100},
		},
		Dividers: []int{240},
	})
	checkExpectedCreation(r, "split_view", split, splitErr)

	groupContent, groupContentErr := window.CreateLabel("Group content")
	if groupContentErr == nil {
		group, groupErr := window.CreateGroupBox(ui.GroupBoxOptions{
			Title:    "Rendering",
			Subtitle: "Quality settings",
			Content:  groupContent,
		})
		checkExpectedCreation(r, "group_box", group, groupErr)
	}

	popoverContent, popoverContentErr := window.CreateLabel("Popover")
	if popoverContentErr == nil {
		popover, popoverErr := window.CreatePopover(ui.PopoverOptions{
			Anchor:        first,
			Content:       popoverContent,
			PreferredEdge: ui.EdgeBottom,
		})
		checkExpectedCreation(r, "popover", popover, popoverErr)
	}

	drawerContent, drawerContentErr := window.CreateLabel("Drawer")
	if drawerContentErr == nil {
		drawer, drawerErr := window.CreateDrawer(ui.DrawerOptions{
			Edge:    ui.EdgeRight,
			Content: drawerContent,
			Open:    true,
			Width:   280,
		})
		checkExpectedCreation(r, "drawer", drawer, drawerErr)
	}

	gridChild, gridChildErr := window.CreateLabel("Grid")
	if gridChildErr == nil {
		grid, gridErr := window.CreateGridLayout(ui.GridLayoutOptions{
			Columns:   []ui.GridTrack{{Grow: 1}, {Size: 200}},
			Rows:      []ui.GridTrack{{Size: 40}},
			Items:     []ui.GridItem{{Content: gridChild, Column: 0, Row: 0, ColumnSpan: 2}},
			RowGap:    8,
			ColumnGap: 12,
		})
		checkExpectedCreation(r, "grid_layout", grid, gridErr)
	}

	flowChild, flowChildErr := window.CreateLabel("Flow")
	if flowChildErr == nil {
		flow, flowErr := window.CreateFlowLayout(ui.FlowLayoutOptions{
			Direction: ui.Horizontal,
			Wrap:      true,
			RowGap:    8,
			ColumnGap: 12,
			Children:  []ui.Widget{flowChild},
		})
		checkExpectedCreation(r, "flow_layout", flow, flowErr)
	}

	overlayChild, overlayChildErr := window.CreateLabel("Overlay")
	if overlayChildErr == nil {
		overlay, overlayErr := window.CreateOverlayLayout(ui.OverlayLayoutOptions{
			Items: []ui.OverlayItem{{Content: overlayChild, Anchor: ui.AnchorCenter}},
		})
		checkExpectedCreation(r, "overlay_layout", overlay, overlayErr)
	}

	formEditor, formEditorErr := window.CreateTextField()
	if formEditorErr == nil {
		form, formErr := window.CreateFormLayout(ui.FormLayoutOptions{
			Rows: []ui.FormRow{{ID: "name", Label: "Name", Editor: formEditor, Required: true}},
		})
		checkExpectedCreation(r, "form_layout", form, formErr)
	}

	dockContent, dockContentErr := window.CreateLabel("Dock")
	if dockContentErr == nil {
		docking, dockingErr := window.CreateDockingLayout(ui.DockingLayoutOptions{
			Panes: []ui.DockPane{{ID: "inspector", Title: "Inspector", Content: dockContent, Area: ui.DockRight}},
		})
		checkExpectedCreation(r, "docking_layout", docking, dockingErr)
	}
}

func checkExpectedCommandComponents(r *recorder, window ui.Window) {
	actions := []ui.Action{
		{ID: "open", Text: "Open", Enabled: true, Shortcut: ui.Shortcut{Key: "O", Command: true}},
		{ID: "save", Text: "Save", Enabled: true, Shortcut: ui.Shortcut{Key: "S", Command: true}},
	}
	menu, menuErr := window.CreateMenu(ui.MenuOptions{
		Items: []ui.MenuItem{
			{ActionID: "open"},
			{Separator: true},
			{ActionID: "save"},
		},
	})
	// The menu bar uses the menu, so it is created while the menu is open
	// and closed before it.
	menuBar, menuBarErr := window.CreateMenuBar(ui.MenuBarOptions{
		Actions: actions,
		Menus:   []ui.NamedMenu{{Title: "File", Menu: menu}},
	})
	checkExpectedCreation(r, "menu_bar", menuBar, menuBarErr)
	checkExpectedCreation(r, "menu", menu, menuErr)

	toolbar, toolbarErr := window.CreateToolbar(ui.ToolbarOptions{
		Actions: actions,
		Items: []ui.ToolbarItem{
			{ActionID: "open"},
			{FlexibleSpace: true},
			{ActionID: "save"},
		},
	})
	checkExpectedCreation(r, "toolbar", toolbar, toolbarErr)

	status, statusErr := window.CreateStatusBar(ui.StatusBarOptions{Text: "Ready"})
	checkExpectedCreation(r, "status_bar", status, statusErr)

	commandBar, commandBarErr := window.CreateCommandBar(ui.CommandBarOptions{
		Actions: actions,
		Compact: true,
	})
	checkExpectedCreation(r, "command_bar", commandBar, commandBarErr)

	contextMenu, contextMenuErr := window.CreateContextMenu(ui.ContextMenuOptions{
		Actions: actions,
		Items:   []ui.MenuItem{{ActionID: "open"}},
	})
	checkExpectedCreation(r, "context_menu", contextMenu, contextMenuErr)

	titleContent, titleContentErr := window.CreateLabel("Scene")
	if titleContentErr == nil {
		accessory, accessoryErr := window.CreateTitleBarAccessory(ui.TitleBarAccessoryOptions{
			Content:   titleContent,
			Alignment: ui.AlignEnd,
		})
		checkExpectedCreation(r, "title_bar_accessory", accessory, accessoryErr)
	}
}

func checkExpectedFeedbackComponents(r *recorder, window ui.Window) {
	tooltip, tooltipErr := window.CreateTooltip(ui.TooltipOptions{
		Text:      "More information",
		Delay:     500 * time.Millisecond,
		Placement: ui.EdgeBottom,
	})
	checkExpectedCreation(r, "tooltip", tooltip, tooltipErr)

	toast, toastErr := window.CreateToast(ui.ToastOptions{
		Message: "Saved",
		Actions: []ui.Action{{ID: "undo", Text: "Undo", Enabled: true}},
		Timeout: 5 * time.Second,
	})
	checkExpectedCreation(r, "toast", toast, toastErr)

	badge, badgeErr := window.CreateBadge(ui.BadgeOptions{
		Count:           120,
		Maximum:         99,
		Severity:        ui.SeverityInformation,
		AccessibleValue: "120 notifications",
	})
	checkExpectedCreation(r, "badge", badge, badgeErr)

	progress, progressErr := window.CreateProgressDialog(ui.ProgressDialogOptions{
		Title:       "Exporting",
		Message:     "Writing files",
		Value:       0.5,
		Cancellable: true,
	})
	checkExpectedCreation(r, "progress_dialog", progress, progressErr)

	empty, emptyErr := window.CreateEmptyState(ui.EmptyStateOptions{
		Title:   "No scenes",
		Detail:  "Create or open a scene.",
		Actions: []ui.Action{{ID: "create", Text: "Create", Enabled: true}},
	})
	checkExpectedCreation(r, "empty_state", empty, emptyErr)

	validation, validationErr := window.CreateInlineValidation(ui.InlineValidationOptions{
		Severity: ui.SeverityError,
		Text:     "A name is required.",
	})
	checkExpectedCreation(r, "inline_validation", validation, validationErr)
}

func checkExpectedContentComponents(r *recorder, window ui.Window) {
	icon, iconErr := window.CreateIcon(ui.IconOptions{
		SystemName:        "folder",
		Size:              ui.Size{Width: 24, Height: 24},
		AccessibilityName: "Folder",
	})
	checkExpectedCreation(r, "icon", icon, iconErr)

	richText, richTextErr := window.CreateRichText(ui.RichTextOptions{
		Text:     "GameKit",
		Editable: true,
	})
	checkExpectedCreation(r, "rich_text", richText, richTextErr)

	markdown, markdownErr := window.CreateMarkdown(ui.MarkdownOptions{Source: "# GameKit"})
	checkExpectedCreation(r, "markdown", markdown, markdownErr)

	svg, svgErr := window.CreateSVG(ui.SVGOptions{
		Source:            []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`),
		AccessibilityText: "Empty image",
	})
	checkExpectedCreation(r, "svg", svg, svgErr)

	animation, animationErr := window.CreateAnimatedImage(ui.AnimatedImageOptions{
		Frames: []ui.AnimationFrame{{Image: testImage(), Duration: 100 * time.Millisecond}},
		Repeat: true,
	})
	checkExpectedCreation(r, "animated_image", animation, animationErr)

	video, videoErr := window.CreateVideo(ui.VideoOptions{Source: "sample.mp4", ShowControls: true})
	checkExpectedCreation(r, "video", video, videoErr)

	web, webErr := window.CreateWebView(ui.WebViewOptions{
		URL:          "https://example.com",
		AllowScripts: false,
	})
	checkExpectedCreation(r, "web_view", web, webErr)
}

type conformanceObject interface {
	Close()
}

func checkExpectedCreation(r *recorder, name string, object conformanceObject, err error) {
	r.check(name+"/create", func() error {
		if err != nil {
			return err
		}
		if object == nil {
			return fmt.Errorf("Create%s returned nil", name)
		}
		object.Close()
		object.Close()
		return nil
	})
}
