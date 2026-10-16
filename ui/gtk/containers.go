//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
#include "widget_bridge.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"slices"
	"sync"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// adopt validates child for container and moves its view into into.
func adopt(container nativeWidget, child ui.Widget, into unsafe.Pointer) (nativeWidget, error) {
	native, err := validateContainerChild(container, child)
	if err != nil {
		return nil, err
	}
	attach(container, native, into)
	return native, nil
}

// adoptAll adopts children, rejecting duplicates. It validates every child
// before it moves any, so an error leaves all children unchanged.
func adoptAll(container nativeWidget, children []ui.Widget, into unsafe.Pointer) ([]nativeWidget, error) {
	natives, err := validateContainerChildren(container, children)
	if err != nil {
		return nil, err
	}
	for _, native := range natives {
		attach(container, native, into)
	}
	return natives, nil
}

// validateContainerChildren checks that every child can move into container
// and that no child occurs twice.
func validateContainerChildren(container nativeWidget, children []ui.Widget) ([]nativeWidget, error) {
	seen := make(map[*widgetBase]struct{}, len(children))
	result := make([]nativeWidget, 0, len(children))
	for _, child := range children {
		if child == nil {
			return nil, errors.New("ui: container child is nil")
		}
		native, err := validateContainerChild(container, child)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[native.base()]; exists {
			return nil, errors.New("ui/gtk: container contains the same child more than once")
		}
		seen[native.base()] = struct{}{}
		result = append(result, native)
	}
	return result, nil
}

// attach moves the view of a validated child into into.
func attach(container nativeWidget, child nativeWidget, into unsafe.Pointer) {
	child.base().parent = container
	C.gkGTKAddChild(into, child.base().nativePointer())
}

// orphan returns a child to the window when its container lets it go.
func orphan(child nativeWidget) {
	base := child.base()
	base.parent = nil
	if view := base.nativePointer(); view != nil && base.window != nil {
		C.gkGTKUIAttachToWindow(base.window.nativeWindowHandle(), view)
		base.SetVisible(false)
	}
}

// frameContainer is a container whose children Go positions with frames.
type frameContainer struct {
	*widgetBase
	mu          sync.RWMutex
	children    []nativeWidget
	arrange     func(size ui.Size)
	preferred   func() ui.Size
	arrangement arrangement
}

func (w *Window) newFrameContainer(role ui.AccessibilityRole, owner gtkReceiver) (*frameContainer, error) {
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &frameContainer{}
	if owner == nil {
		owner = result
	}
	result.widgetBase, err = w.newComponent(C.gkGTKClipViewCreate(window), role, owner)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (f *frameContainer) gtkEvent(gtkEvent) {
	// A frame sends no native events.
}

func (f *frameContainer) base() *widgetBase {
	return f.widgetBase
}

func (f *frameContainer) setChildren(children []ui.Widget) error {
	adopted, err := adoptAll(f, children, f.nativePointer())
	if err != nil {
		return err
	}
	f.mu.Lock()
	previous := f.children
	f.children = adopted
	f.mu.Unlock()
	for _, child := range previous {
		if !containsWidget(adopted, child) {
			orphan(child)
		}
	}
	for _, child := range adopted {
		child.SetVisible(true)
	}
	f.relayout()
	return nil
}

func (f *frameContainer) Children() []ui.Widget {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]ui.Widget, len(f.children))
	for i, child := range f.children {
		result[i] = child
	}
	return result
}

func (f *frameContainer) Close() {
	if f == nil {
		return
	}
	f.mu.Lock()
	children := f.children
	f.children = nil
	f.mu.Unlock()
	for _, child := range children {
		child.Close()
	}
	f.widgetBase.Close()
}

// SetBounds places the container and arranges its children. It does not
// reapply the parent layout, because the parent layout calls it.
func (f *frameContainer) SetBounds(bounds ui.Rect) {
	f.widgetBase.SetBounds(bounds)
	f.arrangeChildren()
}

// relayout arranges the children after the content changes, and reapplies
// the parent layout because the preferred size can change with the content.
func (f *frameContainer) relayout() {
	f.arrangeChildren()
	f.relayoutParent()
}

func (f *frameContainer) arrangeChildren() {
	if f.arrange == nil {
		return
	}
	f.arrangement.run(func() {
		f.arrange(f.Bounds().Size)
	})
}

func (f *frameContainer) PreferredSize() ui.Size {
	if f.preferred != nil {
		return f.preferred()
	}
	return ui.Size{}
}

// preferredOf returns a widget's preferred size including its size hints.
func preferredOf(widget ui.Widget) ui.Size {
	size, _ := ui.Column(ui.Item(widget)).PreferredSize()
	return size
}

// ---- Accordion ----

type accordion struct {
	*frameContainer
	expansion ui.AccordionExpansion
	sections  []ui.AccordionSection
	natives   []*section
	changes   listeners[ui.SelectionIDsHandler]
}

// CreateAccordion implements ui.Window.CreateAccordion.
func (w *Window) CreateAccordion(options ui.AccordionOptions) (ui.Accordion, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &accordion{expansion: options.Expansion, sections: append([]ui.AccordionSection(nil), options.Sections...)}
	container, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	children := make([]ui.Widget, 0, len(options.Sections))
	for _, item := range options.Sections {
		created, err := w.CreateSection(ui.SectionOptions{Title: item.Title, Content: item.Content, Expanded: item.Expanded})
		if err != nil {
			for _, existing := range result.natives {
				existing.Close()
			}
			container.Close()
			return nil, err
		}
		native := created.(*section)
		id := item.ID
		native.OnChange(func(expanded bool) {
			result.sectionChanged(id, expanded)
		})
		result.natives = append(result.natives, native)
		children = append(children, native)
	}
	if err := container.setChildren(children); err != nil {
		container.Close()
		return nil, err
	}
	container.arrange = result.arrange
	container.preferred = result.preferred
	return result, nil
}

func (a *accordion) arrange(size ui.Size) {
	y := 0
	for _, child := range a.natives {
		height := child.PreferredSize().Height
		child.SetBounds(ui.Rect{Position: ui.Point{Y: y}, Size: ui.Size{Width: size.Width, Height: height}})
		y += height
	}
}

func (a *accordion) preferred() ui.Size {
	size := ui.Size{}
	for _, child := range a.natives {
		preferred := child.PreferredSize()
		size.Width = max(size.Width, preferred.Width)
		size.Height += preferred.Height
	}
	return size
}

func (a *accordion) sectionChanged(id string, expanded bool) {
	if expanded && a.expansion == ui.AccordionSingle {
		for i, item := range a.sections {
			if item.ID != id && a.natives[i].IsExpanded() {
				a.natives[i].SetExpanded(false)
			}
		}
	}
	a.relayout()
	ids := a.ExpandedIDs()
	each(&a.changes, func(handler ui.SelectionIDsHandler) {
		handler(append([]string(nil), ids...))
	})
}

func (a *accordion) Sections() []ui.AccordionSection {
	sections := append([]ui.AccordionSection(nil), a.sections...)
	for i := range sections {
		sections[i].Expanded = a.natives[i].IsExpanded()
	}
	return sections
}

func (a *accordion) ExpandedIDs() []string {
	var ids []string
	for i, item := range a.sections {
		if a.natives[i].IsExpanded() {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func (a *accordion) SetExpanded(id string, expanded bool) error {
	for i, item := range a.sections {
		if item.ID != id {
			continue
		}
		if expanded && a.expansion == ui.AccordionSingle {
			for j := range a.natives {
				if j != i {
					a.natives[j].SetExpanded(false)
				}
			}
		}
		a.natives[i].SetExpanded(expanded)
		a.relayout()
		return nil
	}
	return errors.New("ui: accordion section does not exist")
}

func (a *accordion) OnChange(handler ui.SelectionIDsHandler) ui.Unsubscribe {
	return subscribe(&a.changes, handler, handler == nil)
}

// ---- Group box ----

type groupBox struct {
	*widgetBase
	title, subtitle string
	content         nativeWidget
	contentTop      int
}

// CreateGroupBox implements ui.Window.CreateGroupBox.
func (w *Window) CreateGroupBox(options ui.GroupBoxOptions) (ui.GroupBox, error) {
	if options.Content == nil {
		return nil, errors.New("ui: group box content is required")
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	title, subtitle := cString(options.Title), cString(options.Subtitle)
	defer freeCString(title)
	defer freeCString(subtitle)
	result := &groupBox{title: options.Title, subtitle: options.Subtitle}
	if result.widgetBase, err = w.newComponent(C.gkGTKGroupBoxCreate(window, title, subtitle), ui.RoleGroup, result); err != nil {
		return nil, err
	}
	if result.content, err = adopt(result, options.Content, result.nativePointer()); err != nil {
		result.widgetBase.Close()
		return nil, err
	}
	return result, nil
}

func (g *groupBox) gtkEvent(gtkEvent) {
	// A group box sends no native events.
}
func (g *groupBox) Title() string {
	return g.title
}
func (g *groupBox) SetTitle(title string) {
	g.title = title
	g.apply()
}
func (g *groupBox) Subtitle() string {
	return g.subtitle
}
func (g *groupBox) SetSubtitle(text string) {
	g.subtitle = text
	g.apply()
}
func (g *groupBox) Content() ui.Widget {
	return g.content
}
func (g *groupBox) apply() {
	if native := g.nativePointer(); native != nil {
		title, subtitle := cString(g.title), cString(g.subtitle)
		defer freeCString(title)
		defer freeCString(subtitle)
		C.gkGTKGroupBoxSet(native, title, subtitle)
		g.relayout()
	}
}
func (g *groupBox) SetBounds(bounds ui.Rect) {
	previousTop := g.contentTop
	g.widgetBase.SetBounds(bounds)
	g.relayout()
	if g.contentTop != previousTop {
		// The subtitle wraps differently at the new width, which changes the
		// preferred height.
		g.relayoutParent()
	}
}
func (g *groupBox) relayout() {
	native := g.nativePointer()
	if native == nil || g.content == nil {
		return
	}
	var width, height, top C.int
	C.gkGTKGroupBoxBody(native, &width, &height, &top)
	g.contentTop = int(top)
	g.content.SetBounds(ui.Rect{
		Position: ui.Point{Y: int(top)},
		Size:     ui.Size{Width: int(width), Height: max(0, int(height-top))},
	})
}
func (g *groupBox) PreferredSize() ui.Size {
	native := g.nativePointer()
	if native == nil || g.content == nil {
		return ui.Size{}
	}
	var chromeWidth, chromeHeight, width, height, top C.int
	C.gkGTKGroupBoxChrome(native, &chromeWidth, &chromeHeight)
	C.gkGTKGroupBoxBody(native, &width, &height, &top)
	content := preferredOf(g.content)
	// Before the first layout the box has no width, and a subtitle measured
	// at that width wraps after every word. Measure it at the width that the
	// content asks for instead.
	top = C.gkGTKGroupBoxSubtitleHeight(native, C.int(max(int(width), content.Width)))
	return ui.Size{Width: content.Width + int(chromeWidth), Height: content.Height + int(chromeHeight) + int(top)}
}
func (g *groupBox) Close() {
	if g.content != nil {
		g.content.Close()
		g.content = nil
	}
	g.widgetBase.Close()
}

// ---- Split view ----

type splitView struct {
	*widgetBase
	direction ui.LayoutDirection
	panes     []ui.SplitPane
	children  []nativeWidget
	pending   []int
	changes   listeners[func([]int)]
}

// CreateSplitView implements ui.Window.CreateSplitView.
func (w *Window) CreateSplitView(options ui.SplitViewOptions) (ui.SplitView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	window, err := w.windowHandle()
	if err != nil {
		return nil, err
	}
	result := &splitView{
		direction: options.Direction,
		panes:     append([]ui.SplitPane(nil), options.Panes...),
		pending:   append([]int(nil), options.Dividers...),
	}
	native := C.gkGTKSplitViewCreate(window, boolInt(options.Direction == ui.Horizontal), C.int(len(options.Panes)))
	if result.widgetBase, err = w.newComponent(native, ui.RoleSplitView, result); err != nil {
		return nil, err
	}
	contents := make([]ui.Widget, len(options.Panes))
	for i, pane := range options.Panes {
		contents[i] = pane.Content
	}
	children, err := validateContainerChildren(result, contents)
	if err != nil {
		// Close only the split view: the caller still owns the panes.
		result.widgetBase.Close()
		return nil, err
	}
	for i, pane := range options.Panes {
		attach(result, children[i], C.gkGTKSplitViewPane(native, C.int(i)))
		C.gkGTKSplitViewSetPane(native, C.int(i), C.double(pane.Minimum), boolInt(pane.Collapsible))
	}
	result.children = children
	return result, nil
}

func (s *splitView) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_RESIZE {
		s.layoutPanes()
		positions := s.Dividers()
		each(&s.changes, func(handler func([]int)) {
			handler(append([]int(nil), positions...))
		})
	}
}

func (s *splitView) SetBounds(bounds ui.Rect) {
	s.widgetBase.SetBounds(bounds)
	if len(s.pending) > 0 && bounds.Size.IsValid() {
		pending := s.pending
		s.pending = nil
		_ = s.SetDividers(pending)
	}
	s.layoutPanes()
}

func (s *splitView) relayout() {
	s.layoutPanes()
}

func (s *splitView) layoutPanes() {
	native := s.nativePointer()
	if native == nil {
		return
	}
	for i, child := range s.children {
		var x, y, width, height C.int
		C.gkGTKSplitViewPaneFrame(native, C.int(i), &x, &y, &width, &height)
		child.SetBounds(ui.Rect{Size: ui.Size{Width: int(width), Height: int(height)}})
	}
}

func (s *splitView) PreferredSize() ui.Size {
	size := ui.Size{}
	for i, child := range s.children {
		preferred := preferredOf(child)
		if s.direction == ui.Horizontal {
			size.Width += max(preferred.Width, s.panes[i].Minimum) + 1
			size.Height = max(size.Height, preferred.Height)
		} else {
			size.Height += max(preferred.Height, s.panes[i].Minimum) + 1
			size.Width = max(size.Width, preferred.Width)
		}
	}
	return size
}

func (s *splitView) Panes() []ui.SplitPane {
	return append([]ui.SplitPane(nil), s.panes...)
}

func (s *splitView) Dividers() []int {
	native := s.nativePointer()
	if native == nil {
		return nil
	}
	positions := make([]int, max(0, len(s.panes)-1))
	for i := range positions {
		positions[i] = int(C.gkGTKSplitViewDivider(native, C.int(i)))
	}
	return positions
}

func (s *splitView) SetDividers(positions []int) error {
	if len(positions) != len(s.panes)-1 {
		return errors.New("ui: split view needs one divider position per adjacent pane pair")
	}
	if !slices.IsSorted(positions) {
		return errors.New("ui: split view divider positions must be ascending")
	}
	native := s.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	if !s.Bounds().Size.IsValid() {
		s.pending = append([]int(nil), positions...)
		return nil
	}
	for i, position := range positions {
		C.gkGTKSplitViewSetDivider(native, C.int(i), C.double(position))
	}
	s.layoutPanes()
	return nil
}

func (s *splitView) OnChange(handler func(positions []int)) ui.Unsubscribe {
	return subscribe(&s.changes, handler, handler == nil)
}

func (s *splitView) Close() {
	for _, child := range s.children {
		child.Close()
	}
	s.children = nil
	s.widgetBase.Close()
}

// ---- Popover ----

type popover struct {
	window  *Window
	native  unsafe.Pointer
	handle  cgo.Handle
	body    *frameContainer
	anchor  nativeWidget
	content nativeWidget
	edge    ui.Edge
	hides   listeners[ui.FocusHandler]
}

// CreatePopover implements ui.Window.CreatePopover.
func (w *Window) CreatePopover(options ui.PopoverOptions) (ui.Popover, error) {
	if options.Anchor == nil || options.Content == nil {
		return nil, errors.New("ui: popover requires an anchor and content")
	}
	anchor, ok := options.Anchor.(nativeWidget)
	if !ok || anchor.base().window != w {
		return nil, errors.New("ui/gtk: popover anchor belongs to another backend or window")
	}
	result := &popover{window: w, anchor: anchor, edge: options.PreferredEdge}
	result.handle = cgo.NewHandle(result)
	result.native = C.gkGTKPopoverCreate(C.uintptr_t(result.handle), boolInt(options.Transient))
	body := &frameContainer{}
	body.widgetBase = newWidgetBase(w, C.gkGTKPopoverBody(result.native), ui.RoleDialog)
	C.gkGTKRetain(body.nativePointer())
	body.contract.owner = body
	if err := body.setChildren([]ui.Widget{options.Content}); err != nil {
		body.Close()
		C.gkGTKPopoverRelease(result.native)
		result.handle.Delete()
		return nil, err
	}
	result.body = body
	result.content = body.children[0]
	body.arrange = func(size ui.Size) {
		result.content.SetBounds(ui.Rect{Size: size})
	}
	w.adoptResource(result)
	return result, nil
}

func (p *popover) gtkEvent(event gtkEvent) {
	if event.kind == C.GK_EVENT_CLOSE {
		each(&p.hides, func(handler ui.FocusHandler) {
			handler()
		})
	}
}

func (p *popover) Close() {
	if p.native == nil {
		return
	}
	native := p.native
	p.native = nil
	if p.body != nil {
		p.body.Close()
	}
	C.gkGTKPopoverRelease(native)
	p.handle.Delete()
	p.window.releaseResource(p)
}

func (p *popover) Show() error {
	if p.native == nil {
		return ui.ErrClosed
	}
	view := p.anchor.base().nativePointer()
	if view == nil {
		return ui.ErrClosed
	}
	size := preferredOf(p.content)
	size.Width, size.Height = max(size.Width, 1), max(size.Height, 1)
	if C.gkGTKPopoverShow(p.native, view, C.int(p.edge), C.int(size.Width), C.int(size.Height)) == 0 {
		return errors.New("ui/gtk: a popover's anchor must be in a visible window")
	}
	p.body.SetBounds(ui.Rect{Size: size})
	return nil
}

func (p *popover) Hide() {
	if p.native != nil {
		C.gkGTKPopoverHide(p.native)
	}
}

func (p *popover) IsShown() bool {
	return p.native != nil && C.gkGTKPopoverShown(p.native) != 0
}

func (p *popover) Content() ui.Widget {
	return p.content
}

func (p *popover) OnHide(handler ui.FocusHandler) ui.Unsubscribe {
	return subscribe(&p.hides, handler, handler == nil)
}

// ---- Drawer ----

type drawer struct {
	*frameContainer
	edge    ui.Edge
	width   int
	open    bool
	content nativeWidget
	changes listeners[ui.ExpansionChangeHandler]
}

// CreateDrawer implements ui.Window.CreateDrawer.
func (w *Window) CreateDrawer(options ui.DrawerOptions) (ui.Drawer, error) {
	if options.Content == nil {
		return nil, errors.New("ui: drawer content is required")
	}
	if options.Width < 0 {
		return nil, errors.New("ui: drawer width must be non-negative")
	}
	result := &drawer{edge: options.Edge, width: options.Width, open: options.Open}
	container, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	if err := container.setChildren([]ui.Widget{options.Content}); err != nil {
		container.Close()
		return nil, err
	}
	result.content = container.children[0]
	container.arrange = result.arrange
	container.preferred = result.preferred
	result.content.SetVisible(result.open)
	return result, nil
}

func (d *drawer) extent() int {
	if d.width > 0 {
		return d.width
	}
	preferred := preferredOf(d.content)
	if d.edge == ui.EdgeTop || d.edge == ui.EdgeBottom {
		return preferred.Height
	}
	return preferred.Width
}

func (d *drawer) arrange(size ui.Size) {
	if !d.open {
		return
	}
	extent := d.extent()
	bounds := ui.Rect{Size: size}
	switch d.edge {
	case ui.EdgeLeft:
		bounds.Size.Width = min(size.Width, extent)
	case ui.EdgeRight:
		bounds.Size.Width = min(size.Width, extent)
		bounds.Position.X = size.Width - bounds.Size.Width
	case ui.EdgeTop:
		bounds.Size.Height = min(size.Height, extent)
	case ui.EdgeBottom:
		bounds.Size.Height = min(size.Height, extent)
		bounds.Position.Y = size.Height - bounds.Size.Height
	}
	d.content.SetBounds(bounds)
}

func (d *drawer) preferred() ui.Size {
	if !d.open {
		return ui.Size{}
	}
	preferred := preferredOf(d.content)
	if d.edge == ui.EdgeTop || d.edge == ui.EdgeBottom {
		return ui.Size{Width: preferred.Width, Height: d.extent()}
	}
	return ui.Size{Width: d.extent(), Height: preferred.Height}
}

func (d *drawer) Content() ui.Widget {
	return d.content
}
func (d *drawer) IsOpen() bool {
	return d.open
}
func (d *drawer) SetOpen(open bool) {
	if d.open == open {
		return
	}
	d.open = open
	d.content.SetVisible(open)
	d.relayout()
	each(&d.changes, func(handler ui.ExpansionChangeHandler) {
		handler(open)
	})
}
func (d *drawer) OnChange(handler ui.ExpansionChangeHandler) ui.Unsubscribe {
	return subscribe(&d.changes, handler, handler == nil)
}

// ---- Grid, flow, and overlay layouts ----

type gridLayout struct {
	*frameContainer
	options ui.GridLayoutOptions
}

// CreateGridLayout implements ui.Window.CreateGridLayout.
func (w *Window) CreateGridLayout(options ui.GridLayoutOptions) (ui.GridLayout, error) {
	result := &gridLayout{}
	container, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	container.arrange = result.arrange
	container.preferred = result.preferred
	if err := result.SetOptions(options); err != nil {
		container.Close()
		return nil, err
	}
	return result, nil
}

func (g *gridLayout) Options() ui.GridLayoutOptions {
	options := g.options
	options.Columns = append([]ui.GridTrack(nil), options.Columns...)
	options.Rows = append([]ui.GridTrack(nil), options.Rows...)
	options.Items = append([]ui.GridItem(nil), options.Items...)
	return options
}

func (g *gridLayout) SetOptions(options ui.GridLayoutOptions) error {
	if err := options.Validate(); err != nil {
		return err
	}
	children := make([]ui.Widget, len(options.Items))
	for i, item := range options.Items {
		children[i] = item.Content
	}
	g.options = options
	return g.setChildren(children)
}

// tracks resolves track sizes for available space along one axis.
func resolveTracks(tracks []ui.GridTrack, preferred []int, gap, available int) []int {
	sizes := make([]int, len(tracks))
	used := gap * max(0, len(tracks)-1)
	var weight float32
	for i, track := range tracks {
		switch {
		case track.Size > 0:
			sizes[i] = track.Size
		case track.Grow > 0:
			weight += track.Grow
			continue
		default:
			sizes[i] = preferred[i]
		}
		used += sizes[i]
	}
	if remaining := available - used; remaining > 0 && weight > 0 {
		for i, track := range tracks {
			if track.Size == 0 && track.Grow > 0 {
				sizes[i] = int(float32(remaining) * track.Grow / weight)
			}
		}
	}
	return sizes
}

func (g *gridLayout) trackPreferences() (columns, rows []int) {
	columns = make([]int, len(g.options.Columns))
	rows = make([]int, len(g.options.Rows))
	for _, item := range g.options.Items {
		size := preferredOf(item.Content)
		if max(1, item.ColumnSpan) == 1 {
			columns[item.Column] = max(columns[item.Column], size.Width)
		}
		if max(1, item.RowSpan) == 1 {
			rows[item.Row] = max(rows[item.Row], size.Height)
		}
	}
	return columns, rows
}

func (g *gridLayout) arrange(size ui.Size) {
	columnPreferred, rowPreferred := g.trackPreferences()
	padding := g.options.Padding
	inner := ui.Size{Width: size.Width - padding.Left - padding.Right, Height: size.Height - padding.Top - padding.Bottom}
	columns := resolveTracks(g.options.Columns, columnPreferred, g.options.ColumnGap, inner.Width)
	rows := resolveTracks(g.options.Rows, rowPreferred, g.options.RowGap, inner.Height)
	offsets := func(sizes []int, gap, start int) []int {
		result := make([]int, len(sizes)+1)
		result[0] = start
		for i, size := range sizes {
			result[i+1] = result[i] + size + gap
		}
		return result
	}
	x := offsets(columns, g.options.ColumnGap, padding.Left)
	y := offsets(rows, g.options.RowGap, padding.Top)
	for _, item := range g.options.Items {
		columnEnd := item.Column + max(1, item.ColumnSpan)
		rowEnd := item.Row + max(1, item.RowSpan)
		cell := ui.Rect{
			Position: ui.Point{X: x[item.Column], Y: y[item.Row]},
			Size:     ui.Size{Width: x[columnEnd] - x[item.Column] - g.options.ColumnGap, Height: y[rowEnd] - y[item.Row] - g.options.RowGap},
		}
		item.Content.SetBounds(alignIn(cell, preferredOf(item.Content), item.Alignment))
	}
}

// alignIn places a preferred size within cell, or fills it when stretched.
func alignIn(cell ui.Rect, preferred ui.Size, alignment ui.Alignment) ui.Rect {
	if alignment == ui.AlignStretch {
		return cell
	}
	size := ui.Size{Width: min(cell.Size.Width, preferred.Width), Height: min(cell.Size.Height, preferred.Height)}
	position := cell.Position
	switch alignment {
	case ui.AlignCenter:
		position.X += (cell.Size.Width - size.Width) / 2
		position.Y += (cell.Size.Height - size.Height) / 2
	case ui.AlignEnd:
		position.X += cell.Size.Width - size.Width
		position.Y += cell.Size.Height - size.Height
	}
	return ui.Rect{Position: position, Size: size}
}

func (g *gridLayout) preferred() ui.Size {
	columnPreferred, rowPreferred := g.trackPreferences()
	columns := resolveTracks(g.options.Columns, columnPreferred, g.options.ColumnGap, 0)
	rows := resolveTracks(g.options.Rows, rowPreferred, g.options.RowGap, 0)
	sum := func(sizes []int, gap int) int {
		total := gap * max(0, len(sizes)-1)
		for _, size := range sizes {
			total += size
		}
		return total
	}
	padding := g.options.Padding
	return ui.Size{
		Width:  sum(columns, g.options.ColumnGap) + padding.Left + padding.Right,
		Height: sum(rows, g.options.RowGap) + padding.Top + padding.Bottom,
	}
}

type flowLayout struct {
	*frameContainer
	options ui.FlowLayoutOptions
}

// CreateFlowLayout implements ui.Window.CreateFlowLayout.
func (w *Window) CreateFlowLayout(options ui.FlowLayoutOptions) (ui.FlowLayout, error) {
	if options.RowGap < 0 || options.ColumnGap < 0 {
		return nil, errors.New("ui: flow gaps must be non-negative")
	}
	result := &flowLayout{options: options}
	container, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	container.arrange = func(size ui.Size) {
		result.flow(size, true)
	}
	container.preferred = func() ui.Size {
		return result.flow(ui.Size{}, false)
	}
	if err := container.setChildren(options.Children); err != nil {
		container.Close()
		return nil, err
	}
	return result, nil
}

func (f *flowLayout) SetChildren(children []ui.Widget) error {
	f.options.Children = append([]ui.Widget(nil), children...)
	return f.setChildren(children)
}

// flow places children in lines along the flow direction. With place false it
// only measures; an empty size measures without wrapping.
func (f *flowLayout) flow(size ui.Size, place bool) ui.Size {
	horizontal := f.options.Direction == ui.Horizontal
	limit := size.Width
	mainGap, crossGap := f.options.ColumnGap, f.options.RowGap
	if !horizontal {
		limit = size.Height
		mainGap, crossGap = f.options.RowGap, f.options.ColumnGap
	}
	main, cross, line, extent := 0, 0, 0, 0
	for _, child := range f.frameContainer.children {
		preferred := preferredOf(child)
		length, thickness := preferred.Width, preferred.Height
		if !horizontal {
			length, thickness = preferred.Height, preferred.Width
		}
		if f.options.Wrap && limit > 0 && main > 0 && main+length > limit {
			cross += line + crossGap
			main, line = 0, 0
		}
		if place {
			rect := ui.Rect{Position: ui.Point{X: main, Y: cross}, Size: preferred}
			if !horizontal {
				rect.Position = ui.Point{X: cross, Y: main}
			}
			child.SetBounds(rect)
		}
		main += length + mainGap
		extent = max(extent, main-mainGap)
		line = max(line, thickness)
	}
	total := cross + line
	if horizontal {
		return ui.Size{Width: extent, Height: total}
	}
	return ui.Size{Width: total, Height: extent}
}

type overlayLayout struct {
	*frameContainer
	items []ui.OverlayItem
}

// CreateOverlayLayout implements ui.Window.CreateOverlayLayout.
func (w *Window) CreateOverlayLayout(options ui.OverlayLayoutOptions) (ui.OverlayLayout, error) {
	result := &overlayLayout{}
	container, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	container.arrange = result.arrange
	container.preferred = result.preferred
	if err := result.SetItems(options.Items); err != nil {
		container.Close()
		return nil, err
	}
	return result, nil
}

func (o *overlayLayout) Items() []ui.OverlayItem {
	return append([]ui.OverlayItem(nil), o.items...)
}

func (o *overlayLayout) SetItems(items []ui.OverlayItem) error {
	children := make([]ui.Widget, len(items))
	for i, item := range items {
		children[i] = item.Content
	}
	previous := o.items
	o.items = append([]ui.OverlayItem(nil), items...)
	if err := o.setChildren(children); err != nil {
		o.items = previous
		return err
	}
	// Later items are drawn on top.
	for _, child := range o.frameContainer.children {
		C.gkGTKUIBringToFront(child.base().nativePointer())
	}
	return nil
}

// anchorFractions gives the position of each ui.Anchor along each axis.
var anchorFractions = [...][2]int{{1, 1}, {0, 0}, {1, 0}, {2, 0}, {0, 1}, {2, 1}, {0, 2}, {1, 2}, {2, 2}}

func (o *overlayLayout) arrange(size ui.Size) {
	for _, item := range o.items {
		preferred := preferredOf(item.Content)
		preferred.Width, preferred.Height = min(preferred.Width, size.Width), min(preferred.Height, size.Height)
		fraction := anchorFractions[0]
		if int(item.Anchor) < len(anchorFractions) {
			fraction = anchorFractions[item.Anchor]
		}
		position := ui.Point{
			X: (size.Width-preferred.Width)*fraction[0]/2 + item.Offset.X,
			Y: (size.Height-preferred.Height)*fraction[1]/2 + item.Offset.Y,
		}
		item.Content.SetBounds(ui.Rect{Position: position, Size: preferred})
	}
}

func (o *overlayLayout) preferred() ui.Size {
	size := ui.Size{}
	for _, item := range o.items {
		preferred := preferredOf(item.Content)
		size.Width = max(size.Width, preferred.Width+abs(item.Offset.X))
		size.Height = max(size.Height, preferred.Height+abs(item.Offset.Y))
	}
	return size
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// ---- Form layout ----

type formRow struct {
	row    ui.FormRow
	label  ui.Label
	help   ui.Label
	errorL ui.Label
}

type formLayout struct {
	*frameContainer
	window *Window
	rows   []formRow
}

const formGap = 8

// CreateFormLayout implements ui.Window.CreateFormLayout.
func (w *Window) CreateFormLayout(options ui.FormLayoutOptions) (ui.FormLayout, error) {
	result := &formLayout{window: w}
	container, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	container.arrange = result.arrange
	container.preferred = result.preferred
	if err := result.SetRows(options.Rows); err != nil {
		container.Close()
		return nil, err
	}
	return result, nil
}

func (f *formLayout) Rows() []ui.FormRow {
	rows := make([]ui.FormRow, len(f.rows))
	for i, row := range f.rows {
		rows[i] = row.row
	}
	return rows
}

func (f *formLayout) SetRows(rows []ui.FormRow) error {
	if err := (ui.FormLayoutOptions{Rows: rows}).Validate(); err != nil {
		return err
	}
	built := make([]formRow, 0, len(rows))
	var children []ui.Widget
	cleanup := func() {
		for _, row := range built {
			row.label.Close()
			row.help.Close()
			row.errorL.Close()
		}
	}
	for _, row := range rows {
		text := row.Label
		if row.Required {
			text += " *"
		}
		label, err := f.window.CreateLabel(text)
		if err != nil {
			cleanup()
			return err
		}
		label.SetTextAlignment(ui.TextAlignEnd)
		help, _ := f.window.CreateLabel(row.Help)
		help.SetSemanticStyle(ui.TextStyleCaption)
		help.SetWrapMode(ui.WrapWords)
		help.SetStyle(ui.WidgetStyle{Foreground: ui.SemanticColorSecondary})
		problem, _ := f.window.CreateLabel("")
		problem.SetSemanticStyle(ui.TextStyleCaption)
		problem.SetStyle(ui.WidgetStyle{Foreground: ui.SemanticColorError})
		built = append(built, formRow{row: row, label: label, help: help, errorL: problem})
		children = append(children, label, row.Editor, help, problem)
	}
	previous := f.rows
	if err := f.setChildren(children); err != nil {
		cleanup()
		return err
	}
	f.rows = built
	for _, row := range previous {
		row.label.Close()
		row.help.Close()
		row.errorL.Close()
	}
	for _, row := range built {
		metadata := row.row.Editor.Accessibility()
		metadata.LabeledBy = []ui.Widget{row.label}
		metadata.IsRequired = row.row.Required
		if row.row.Help != "" {
			metadata.DescribedBy = []ui.Widget{row.help}
		}
		_ = row.row.Editor.SetAccessibility(metadata)
		row.help.SetVisible(row.row.Help != "")
		row.errorL.SetVisible(false)
	}
	f.relayout()
	return nil
}

func (f *formLayout) SetError(id, message string) error {
	for _, row := range f.rows {
		if row.row.ID != id {
			continue
		}
		row.errorL.SetText(message)
		row.errorL.SetVisible(message != "")
		metadata := row.row.Editor.Accessibility()
		metadata.IsInvalid = message != ""
		metadata.DescribedBy = nil
		if row.row.Help != "" {
			metadata.DescribedBy = append(metadata.DescribedBy, row.help)
		}
		if message != "" {
			metadata.DescribedBy = append(metadata.DescribedBy, row.errorL)
		}
		_ = row.row.Editor.SetAccessibility(metadata)
		f.relayout()
		return nil
	}
	return errors.New("ui: form row does not exist")
}

func (f *formLayout) labelWidth() int {
	width := 0
	for _, row := range f.rows {
		width = max(width, row.label.PreferredSize().Width)
	}
	return width
}

func (f *formLayout) arrange(size ui.Size) {
	labelWidth := f.labelWidth()
	editorX := labelWidth + formGap
	editorWidth := max(0, size.Width-editorX)
	y := 0
	for _, row := range f.rows {
		editor := preferredOf(row.row.Editor)
		label := row.label.PreferredSize()
		height := max(editor.Height, label.Height)
		row.label.SetBounds(ui.Rect{Position: ui.Point{Y: y + (height-label.Height)/2}, Size: ui.Size{Width: labelWidth, Height: label.Height}})
		row.row.Editor.SetBounds(ui.Rect{Position: ui.Point{X: editorX, Y: y + (height-editor.Height)/2}, Size: ui.Size{Width: editorWidth, Height: editor.Height}})
		y += height
		for _, note := range []ui.Label{row.help, row.errorL} {
			if note.IsVisible() {
				noteHeight := note.PreferredSize().Height
				note.SetBounds(ui.Rect{Position: ui.Point{X: editorX, Y: y + 2}, Size: ui.Size{Width: editorWidth, Height: noteHeight}})
				y += noteHeight + 2
			}
		}
		y += formGap
	}
}

func (f *formLayout) preferred() ui.Size {
	labelWidth := f.labelWidth()
	size := ui.Size{}
	for _, row := range f.rows {
		editor := preferredOf(row.row.Editor)
		size.Width = max(size.Width, labelWidth+formGap+editor.Width)
		size.Height += max(editor.Height, row.label.PreferredSize().Height) + formGap
		for _, note := range []ui.Label{row.help, row.errorL} {
			if note.IsVisible() {
				size.Height += note.PreferredSize().Height + 2
			}
		}
	}
	return size
}

// ---- Docking layout ----

// dockingLayout arranges panes in left, right, top, bottom, and center areas.
// Panes move between areas programmatically; GTK has no docking framework,
// so the user cannot drag them between areas.
type dockingLayout struct {
	*frameContainer
	window  *Window
	panes   []ui.DockPane
	headers []ui.Label
	hidden  map[string]bool
}

const dockHeaderHeight = 22

// CreateDockingLayout implements ui.Window.CreateDockingLayout. GTK has no
// docking framework: the application moves panes, and the user cannot drag
// them.
func (w *Window) CreateDockingLayout(options ui.DockingLayoutOptions) (ui.DockingLayout, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &dockingLayout{window: w, panes: append([]ui.DockPane(nil), options.Panes...), hidden: map[string]bool{}}
	container, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	result.frameContainer = container
	children := make([]ui.Widget, 0, len(options.Panes)*2)
	for _, pane := range options.Panes {
		header, err := w.CreateLabel(pane.Title)
		if err != nil {
			container.Close()
			return nil, err
		}
		header.SetSemanticStyle(ui.TextStyleHeading)
		result.headers = append(result.headers, header)
		children = append(children, header, pane.Content)
	}
	if err := container.setChildren(children); err != nil {
		for _, header := range result.headers {
			header.Close()
		}
		container.Close()
		return nil, err
	}
	container.arrange = result.arrange
	container.preferred = func() ui.Size {
		return ui.Size{Width: 640, Height: 400}
	}
	return result, nil
}

func (d *dockingLayout) Panes() []ui.DockPane {
	return append([]ui.DockPane(nil), d.panes...)
}

func (d *dockingLayout) MovePane(id string, area ui.DockArea) error {
	if area > ui.DockBottom {
		return errors.New("ui: invalid dock area")
	}
	for i := range d.panes {
		if d.panes[i].ID == id {
			d.panes[i].Area = area
			d.relayout()
			return nil
		}
	}
	return errors.New("ui: dock pane does not exist")
}

func (d *dockingLayout) SetPaneVisible(id string, visible bool) error {
	for i, pane := range d.panes {
		if pane.ID == id {
			d.hidden[id] = !visible
			d.headers[i].SetVisible(visible)
			pane.Content.SetVisible(visible)
			d.relayout()
			return nil
		}
	}
	return errors.New("ui: dock pane does not exist")
}

func (d *dockingLayout) arrange(size ui.Size) {
	areas := map[ui.DockArea][]int{}
	for i, pane := range d.panes {
		if !d.hidden[pane.ID] {
			areas[pane.Area] = append(areas[pane.Area], i)
		}
	}
	side := min(max(size.Width/4, 160), 320)
	edge := min(max(size.Height/4, 120), 240)
	left, right, top, bottom := 0, 0, 0, 0
	if len(areas[ui.DockLeft]) > 0 {
		left = side
	}
	if len(areas[ui.DockRight]) > 0 {
		right = side
	}
	if len(areas[ui.DockTop]) > 0 {
		top = edge
	}
	if len(areas[ui.DockBottom]) > 0 {
		bottom = edge
	}
	middle := max(0, size.Width-left-right)
	regions := map[ui.DockArea]ui.Rect{
		ui.DockLeft:   {Size: ui.Size{Width: left, Height: size.Height}},
		ui.DockRight:  {Position: ui.Point{X: size.Width - right}, Size: ui.Size{Width: right, Height: size.Height}},
		ui.DockTop:    {Position: ui.Point{X: left}, Size: ui.Size{Width: middle, Height: top}},
		ui.DockBottom: {Position: ui.Point{X: left, Y: size.Height - bottom}, Size: ui.Size{Width: middle, Height: bottom}},
		ui.DockCenter: {Position: ui.Point{X: left, Y: top}, Size: ui.Size{Width: middle, Height: max(0, size.Height-top-bottom)}},
	}
	for area, indexes := range areas {
		region := regions[area]
		share := region.Size.Height / len(indexes)
		for n, index := range indexes {
			y := region.Position.Y + n*share
			d.headers[index].SetBounds(ui.Rect{Position: ui.Point{X: region.Position.X + 6, Y: y}, Size: ui.Size{Width: max(0, region.Size.Width-12), Height: dockHeaderHeight}})
			d.panes[index].Content.SetBounds(ui.Rect{
				Position: ui.Point{X: region.Position.X, Y: y + dockHeaderHeight},
				Size:     ui.Size{Width: region.Size.Width, Height: max(0, share-dockHeaderHeight)},
			})
		}
	}
}

// ---- Property inspector ----

type propertyInspector struct {
	*accordion
	groups []ui.PropertyGroup
}

// CreatePropertyInspector implements ui.Window.CreatePropertyInspector.
func (w *Window) CreatePropertyInspector(options ui.PropertyInspectorOptions) (ui.PropertyInspector, error) {
	sections := make([]ui.AccordionSection, 0, len(options.Groups))
	var forms []ui.FormLayout
	closeForms := func() {
		for _, form := range forms {
			form.Close()
		}
	}
	for _, group := range options.Groups {
		rows := make([]ui.FormRow, len(group.Rows))
		for i, row := range group.Rows {
			rows[i] = ui.FormRow{ID: row.ID, Label: row.Label, Editor: row.Editor}
		}
		form, err := w.CreateFormLayout(ui.FormLayoutOptions{Rows: rows})
		if err != nil {
			closeForms()
			return nil, err
		}
		forms = append(forms, form)
		sections = append(sections, ui.AccordionSection{ID: group.ID, Title: group.Title, Content: form, Expanded: true})
	}
	if len(sections) == 0 {
		return nil, errors.New("ui: property inspector requires at least one group")
	}
	created, err := w.CreateAccordion(ui.AccordionOptions{Sections: sections, Expansion: ui.AccordionMultiple})
	if err != nil {
		closeForms()
		return nil, err
	}
	return &propertyInspector{accordion: created.(*accordion), groups: append([]ui.PropertyGroup(nil), options.Groups...)}, nil
}

func (p *propertyInspector) Groups() []ui.PropertyGroup {
	return append([]ui.PropertyGroup(nil), p.groups...)
}

func (p *propertyInspector) SetGroupExpanded(id string, expanded bool) error {
	return p.SetExpanded(id, expanded)
}

// ---- Title bar accessory ----

type titleBarAccessory struct {
	window     *Window
	body       *frameContainer
	content    nativeWidget
	controller unsafe.Pointer
}

// CreateTitleBarAccessory implements ui.Window.CreateTitleBarAccessory.
func (w *Window) CreateTitleBarAccessory(options ui.TitleBarAccessoryOptions) (ui.TitleBarAccessory, error) {
	if options.Content == nil {
		return nil, errors.New("ui: title bar accessory content is required")
	}
	window := w.nativeWindowPointer()
	if window == nil {
		return nil, ui.ErrClosed
	}
	body, err := w.newFrameContainer(ui.RoleGroup, nil)
	if err != nil {
		return nil, err
	}
	if err := body.setChildren([]ui.Widget{options.Content}); err != nil {
		body.Close()
		return nil, err
	}
	content := body.children[0]
	body.arrange = func(size ui.Size) {
		content.SetBounds(ui.Rect{Size: size})
	}
	preferred := preferredOf(content)
	alignment := 0
	switch options.Alignment {
	case ui.AlignStart:
		alignment = 1
	case ui.AlignEnd:
		alignment = 3
	}
	height := preferred.Height
	if alignment != 0 {
		height = max(height, 28)
	}
	var controller unsafe.Pointer
	if C.gkGTKTitleAccessoryAdd(window, body.nativePointer(), C.int(alignment), C.int(preferred.Width),
		C.int(height), &controller) == 0 {
		body.Close()
		return nil, errors.New("ui/gtk: title bar accessories need a titled window")
	}
	body.SetBounds(ui.Rect{Size: ui.Size{Width: preferred.Width, Height: height}})
	return &titleBarAccessory{window: w, body: body, content: content, controller: controller}, nil
}

func (t *titleBarAccessory) Content() ui.Widget {
	return t.content
}

func (t *titleBarAccessory) Close() {
	if t.controller == nil {
		return
	}
	controller := t.controller
	t.controller = nil
	C.gkGTKTitleAccessoryRemove(controller)
	t.body.Close()
}

var (
	_ ui.Accordion         = (*accordion)(nil)
	_ ui.GroupBox          = (*groupBox)(nil)
	_ ui.SplitView         = (*splitView)(nil)
	_ ui.Popover           = (*popover)(nil)
	_ ui.Drawer            = (*drawer)(nil)
	_ ui.GridLayout        = (*gridLayout)(nil)
	_ ui.FlowLayout        = (*flowLayout)(nil)
	_ ui.OverlayLayout     = (*overlayLayout)(nil)
	_ ui.FormLayout        = (*formLayout)(nil)
	_ ui.DockingLayout     = (*dockingLayout)(nil)
	_ ui.PropertyInspector = (*propertyInspector)(nil)
)
