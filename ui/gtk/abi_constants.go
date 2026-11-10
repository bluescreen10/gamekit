//go:build linux && cgo

package gtk

const (
	uiEventClick           = 1
	uiEventSectionChange   = 5
	uiEventSelectionChange = 7
	uiEventSubmit          = 9
	uiEventTabChange       = 8
	uiEventTextChange      = 2
	uiEventToggleChange    = 6
	uiEventValueChange     = 3
	gkWindowAlwaysOnTop    = 16
	gkWindowBorderless     = 4
	gkWindowHidden         = 2
	gkWindowMaximized      = 8
	gkWindowResizable      = 1
	gkWindowTransparent    = 32
	gkWindowUtility        = 64
)
