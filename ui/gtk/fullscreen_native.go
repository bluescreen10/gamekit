//go:build linux && cgo

package gtk

import (
	"time"
	"unsafe"
)

func (win *nativeWinState) fullscreenChanged(fullscreen bool) {
	if win.fullscreenTimer != 0 {
		gSourceRemove(win.fullscreenTimer)
		win.fullscreenTimer = 0
	}
	if fullscreen == win.fullscreen {
		return
	}
	win.fullscreen = fullscreen
	if !fullscreen {
		win.fullscreenTarget = false
		win.applyHints()
	}
	win.notify(gtkEvent{kind: gkEventWindowFullscreen, a: int(nativeBool(fullscreen))})
}
func (win *nativeWinState) fullscreenSettled(width, height int) bool {
	var geometry gdkRectangle
	if monitor := nativeMonitor(win); monitor != nil {
		gdkMonitorGetGeometry(monitor, &geometry)
	}
	screen := width >= int(geometry.width) && height >= int(geometry.height)-1
	if win.fullscreenState {
		return screen
	}
	if win.restoreTimer != 0 || win.restoreWidth > 0 {
		return width == win.restoreWidth && height == win.restoreHeight
	}
	return !screen
}
func (win *nativeWinState) restoreWindowed() bool {
	width, height := int(gtkWidgetGetWidth(win.root)), int(gtkWidgetGetHeight(win.root))
	win.restoreTries--
	reached := width == win.restoreWidth && height == win.restoreHeight
	if win.destroyed || win.fullscreenState || win.restoreTries <= 0 || reached {
		win.restoreTimer = 0
		if !reached {
			win.restoreWidth, win.restoreHeight = 0, 0
		}
		return false
	}
	gtkWindowSetDefaultSize(win.window(), cInt(win.restoreWidth), cInt(win.restoreHeight+win.chromeHeight()))
	scale := int(gtkWidgetGetScaleFactor(win.widget))
	win.message("_NET_MOVERESIZE_WINDOW", [5]cLong{1 | (1 << 10) | (1 << 11) | (1 << 12), 0, 0, cLong(win.restoreWidth * scale), cLong((win.restoreHeight + win.chromeHeight()) * scale)})
	return true
}
func (win *nativeWinState) pollState() {
	if win.destroyed {
		return
	}
	fullscreen := win.state()&nativeStateFullscreen != 0
	if fullscreen == win.fullscreenState {
		return
	}
	win.fullscreenState = fullscreen
	if !fullscreen && win.windowedWidth > 0 {
		win.restoreWidth, win.restoreHeight = win.windowedWidth, win.windowedHeight
		win.windowedWidth, win.windowedHeight, win.restoreTries = 0, 0, 20
		if win.restoreTimer == 0 {
			win.restoreTimer = gTimeoutAddFull(0, 50, gtkSourceCallback(), newNativeCallback(win.restoreWindowed), gtkDestroyCallback())
		}
	}
	if win.fullscreenSettled(int(gtkWidgetGetWidth(win.root)), int(gtkWidgetGetHeight(win.root))) {
		win.fullscreenChanged(fullscreen)
		return
	}
	if win.fullscreenTimer != 0 {
		gSourceRemove(win.fullscreenTimer)
	}
	win.fullscreenTimer = gTimeoutAddFull(0, 1000, gtkSourceCallback(), newNativeCallback(func() bool { win.fullscreenTimer = 0; win.fullscreenChanged(win.fullscreenState); return false }), gtkDestroyCallback())
}
func (win *nativeWinState) unminimize() {
	if win.state()&nativeStateMinimized == 0 {
		return
	}
	gtkWindowUnminimize(win.window())
	gtkWindowPresent(win.window())
	pumpNativeUntil(func() bool { return win.state()&nativeStateMinimized == 0 }, 2*time.Second)
}
func nativeScreenLocked() bool {
	bus := sessionBus()
	if bus == nil {
		return false
	}
	for _, service := range []struct{ name, path string }{{"org.gnome.ScreenSaver", "/org/gnome/ScreenSaver"}, {"org.freedesktop.ScreenSaver", "/org/freedesktop/ScreenSaver"}} {
		reply := gDBusConnectionCallSync(bus, stringPointer(service.name), stringPointer(service.path), stringPointer(service.name), stringPointer("GetActive"), nil, nil, 1, 1000, nil, nil)
		if reply != nil {
			value := gVariantGetChildValue(reply, 0)
			locked := gVariantGetBoolean(value) != 0
			gVariantUnref(value)
			gVariantUnref(reply)
			if locked {
				return true
			}
		}
	}
	return false
}
func gkGTKWindowIsMinimized(pointer unsafe.Pointer) cInt {
	win := nativeWin(pointer)
	return cInt(nativeBool(win != nil && win.state()&nativeStateMinimized != 0))
}
func gkGTKWindowMinimize(pointer unsafe.Pointer) cInt {
	win := nativeWin(pointer)
	if win == nil {
		return 0
	}
	if win.state()&nativeStateMinimized != 0 {
		return 1
	}
	if win.fullscreen {
		return 0
	}
	if gtkWidgetGetMapped(win.widget) == 0 {
		gtkWindowPresent(win.window())
		pumpNativeUntil(win.mapped, time.Second)
	}
	gtkWindowMinimize(win.window())
	pumpNativeUntil(func() bool { return win.state()&nativeStateMinimized != 0 }, time.Second)
	return gkGTKWindowIsMinimized(pointer)
}
func gkGTKWindowIsMaximized(pointer unsafe.Pointer) cInt {
	win := nativeWin(pointer)
	return cInt(nativeBool(win != nil && !win.fullscreen && win.state()&nativeStateMinimized == 0 && win.state()&nativeStateMaximized != 0))
}
func gkGTKWindowMaximize(pointer unsafe.Pointer) cInt {
	win := nativeWin(pointer)
	if win == nil || win.fullscreen {
		return 0
	}
	win.unminimize()
	gtkWindowMaximize(win.window())
	if gtkWidgetGetMapped(win.widget) != 0 {
		done := func() bool { return win.state()&nativeStateMaximized != 0 }
		pumpNativeUntil(done, time.Second)
		if !done() {
			gtkWindowUnmaximize(win.window())
			gtkWindowMaximize(win.window())
			pumpNativeUntil(done, 2*time.Second)
		}
	}
	return 1
}
func gkGTKWindowRestore(pointer unsafe.Pointer) cInt {
	win := nativeWin(pointer)
	if win == nil {
		return 0
	}
	if win.fullscreen || win.fullscreenTarget {
		win.fullscreenTarget = false
		gtkWindowUnfullscreen(win.window())
	}
	win.unminimize()
	if win.state()&nativeStateMaximized != 0 || gtkWindowIsMaximized(win.window()) != 0 {
		gtkWindowUnmaximize(win.window())
		pumpNativeUntil(func() bool { return win.state()&nativeStateMaximized == 0 }, time.Second)
	}
	return cInt(nativeBool(win.state()&nativeStateMinimized == 0))
}
func gkGTKWindowIsFullscreen(pointer unsafe.Pointer) cInt {
	win := nativeWin(pointer)
	return cInt(nativeBool(win != nil && win.fullscreen))
}
func gkGTKWindowSetFullscreen(pointer unsafe.Pointer, fullscreen cInt) cInt {
	win := nativeWin(pointer)
	if win == nil {
		return 0
	}
	target := fullscreen != 0
	if gtkWidgetGetMapped(win.widget) != 0 && target != win.fullscreen && nativeScreenLocked() {
		return 0
	}
	win.fullscreenTarget = target
	if target && !win.fullscreen && !win.fullscreenState {
		win.windowedWidth, win.windowedHeight = win.size()
	}
	if target {
		win.applyHints()
		gtkWindowFullscreen(win.window())
	} else {
		gtkWindowUnfullscreen(win.window())
	}
	return 1
}
