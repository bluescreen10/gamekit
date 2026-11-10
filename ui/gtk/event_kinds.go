//go:build linux && cgo

package gtk

const (
	gkEventFocus              = 1
	gkEventBlur               = 2
	gkEventDrop               = 3
	gkEventWindowResize       = 4
	gkEventWindowCloseRequest = 5
	gkEventWindowFocus        = 6
	gkEventWindowBlur         = 7
	gkEventWindowTheme        = 8
	gkEventWindowFileDrop     = 9
	gkEventWindowOcclusion    = 10
	gkEventWindowScreen       = 11
	gkEventPointerMove        = 12
	gkEventPointerEnter       = 13
	gkEventPointerLeave       = 14
	gkEventIme                = 15
	gkEventTouch              = 16
	gkEventPen                = 17
	gkEventGesture            = 18
	gkEventDevice             = 19
	gkEventKey                = 20
	gkEventText               = 21
	gkEventFrame              = 22
	gkEventSurface            = 23
	gkEventAction             = 24
	gkEventValue              = 25
	gkEventSelection          = 26
	gkEventTextChange         = 27
	gkEventSubmit             = 28
	gkEventExpand             = 29
	gkEventClose              = 30
	gkEventActivate           = 31
	gkEventSort               = 32
	gkEventLink               = 33
	gkEventNavigate           = 34
	gkEventResize             = 35
	gkEventPreferences        = 36
	gkEventAppActive          = 37
	gkEventAppInactive        = 38
	gkEventAppReopen          = 39
	gkEventAppQuitRequest     = 40
	gkEventNotification       = 41
	gkEventClipboard          = 42
	gkEventTimer              = 43
	gkEventWindowFullscreen   = 44
)
