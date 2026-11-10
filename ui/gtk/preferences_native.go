//go:build linux && cgo

package gtk

import (
	"strings"
	"unsafe"
)

var gtkInitialized bool
var systemDark bool
var appEventHandle uintptr
var appActive = -1
var desktopSettings *gSettings

func nativeProperty(object unsafe.Pointer, name string) (gValue, bool) {
	var value gValue
	if object == nil {
		return value, false
	}
	class := (*gObjectClass)(unsafe.Pointer((*gTypeInstance)(object).g_class))
	property := gObjectClassFindProperty(class, stringPointer(name))
	if property == nil {
		return value, false
	}
	gValueInit(&value, property.value_type)
	gObjectGetProperty((*gObject)(object), stringPointer(name), &value)
	return value, true
}
func nativeBoolProperty(object unsafe.Pointer, name string, fallback bool) bool {
	value, ok := nativeProperty(object, name)
	if !ok {
		return fallback
	}
	defer gValueUnset(&value)
	return gValueGetBoolean(&value) != 0
}
func nativeIntProperty(object unsafe.Pointer, name string, fallback int) int {
	value, ok := nativeProperty(object, name)
	if !ok {
		return fallback
	}
	defer gValueUnset(&value)
	switch gTypeFundamental(value.g_type) {
	case gTypeEnum:
		return int(gValueGetEnum(&value))
	case gTypeUint:
		return int(gValueGetUint(&value))
	case gTypeFlagsKind:
		return int(gValueGetFlags(&value))
	default:
		return int(gValueGetInt(&value))
	}
}
func desktopSetting(name string) string {
	source := gSettingsSchemaSourceGetDefault()
	if source == nil {
		return ""
	}
	schema := gSettingsSchemaSourceLookup(source, stringPointer("org.gnome.desktop.interface"), 1)
	if schema == nil {
		return ""
	}
	defer gSettingsSchemaUnref(schema)
	if gSettingsSchemaHasKey(schema, stringPointer(name)) == 0 {
		return ""
	}
	settings := gSettingsNew(stringPointer("org.gnome.desktop.interface"))
	defer gObjectUnref(unsafe.Pointer(settings))
	return takeString(gSettingsGetString(settings, stringPointer(name)))
}
func requestedNativeTheme() int {
	theme := 0
	for _, win := range nativeWindows {
		if win.theme != 0 {
			theme = win.theme
		}
	}
	return theme
}
func resolvedNativeTheme() int {
	if theme := requestedNativeTheme(); theme != 0 {
		return theme
	}
	dark := systemDark
	if scheme := desktopSetting("color-scheme"); scheme != "" {
		dark = scheme == "prefer-dark"
	}
	if dark {
		return 2
	}
	return 1
}
func applyNativeTheme() {
	settings := gtkSettingsGetDefault()
	if settings == nil {
		return
	}
	setNativeProperty(unsafe.Pointer(settings), "gtk-application-prefer-dark-theme", resolvedNativeTheme() == 2)
	if _, ok := nativePropertyExists(unsafe.Pointer(settings), "gtk-interface-color-scheme"); ok {
		scheme := 1
		switch requestedNativeTheme() {
		case 1:
			scheme = 3
		case 2:
			scheme = 2
		}
		setNativeEnumProperty(unsafe.Pointer(settings), "gtk-interface-color-scheme", scheme)
	}
}
func nativePropertyExists(object unsafe.Pointer, name string) (*gParamSpec, bool) {
	if object == nil {
		return nil, false
	}
	class := (*gObjectClass)(unsafe.Pointer((*gTypeInstance)(object).g_class))
	property := gObjectClassFindProperty(class, stringPointer(name))
	return property, property != nil
}
func setNativeEnumProperty(object unsafe.Pointer, name string, number int) {
	property, ok := nativePropertyExists(object, name)
	if !ok {
		return
	}
	var value gValue
	gValueInit(&value, property.value_type)
	gValueSetEnum(&value, cInt(number))
	gObjectSetProperty((*gObject)(object), stringPointer(name), &value)
	gValueUnset(&value)
}
func notifyNativeThemes() {
	for _, win := range nativeWindows {
		resolved := win.theme
		if resolved == 0 {
			resolved = resolvedNativeTheme()
		}
		if resolved != win.resolvedTheme {
			win.resolvedTheme = resolved
			win.notify(gtkEvent{kind: gkEventWindowTheme, a: resolved})
		}
	}
}
func initializeNativeGTK() bool {
	if gtkInitialized {
		return true
	}
	gdkSetAllowedBackends(stringPointer("x11"))
	if gtkInitCheck() == 0 {
		return false
	}
	settings := gtkSettingsGetDefault()
	if settings != nil {
		systemDark = nativeBoolProperty(unsafe.Pointer(settings), "gtk-application-prefer-dark-theme", false)
		for _, property := range []string{"gtk-enable-animations", "gtk-xft-dpi", "gtk-interface-contrast"} {
			if _, ok := nativePropertyExists(unsafe.Pointer(settings), property); ok {
				connectNativeNotify(unsafe.Pointer(settings), "notify::"+property, func() { notifyNativeThemes(); dispatchNativeEvent(appEventHandle, gtkEvent{kind: gkEventPreferences}) })
			}
		}
	}
	source := gSettingsSchemaSourceGetDefault()
	if source != nil {
		schema := gSettingsSchemaSourceLookup(source, stringPointer("org.gnome.desktop.interface"), 1)
		if schema != nil {
			gSettingsSchemaUnref(schema)
			desktopSettings = gSettingsNew(stringPointer("org.gnome.desktop.interface"))
			connectNativeArgument(unsafe.Pointer(desktopSettings), "changed", gtkStringSignalCallback(), func(key string) {
				if key == "color-scheme" {
					applyNativeTheme()
					notifyNativeThemes()
				}
				dispatchNativeEvent(appEventHandle, gtkEvent{kind: gkEventPreferences})
			})
		}
	}
	if display := gdkDisplayGetDefault(); display != nil {
		if seat := gdkDisplayGetDefaultSeat(display); seat != nil {
			for _, signal := range []struct {
				name      string
				connected bool
			}{{"device-added", true}, {"device-removed", false}} {
				connectNativeArgument(unsafe.Pointer(seat), signal.name, gtkObjectSignalCallback(), func(object unsafe.Pointer) {
					device := (*gdkDevice)(object)
					kind := 1
					switch gdkDeviceGetSource(device) {
					case gdkSourceKeyboard:
						kind = 0
					case gdkSourceTouchscreen:
						kind = 2
					case gdkSourcePen:
						kind = 3
					}
					dispatchNativeEvent(appEventHandle, gtkEvent{kind: gkEventDevice, a: kind, b: int(uintptr(object) & 0x7fffffff), c: int(nativeBool(signal.connected)), text: goString(gdkDeviceGetName(device))})
				})
			}
		}
	}
	gtkInitialized = true
	return true
}
func updateNativeAppActive() {
	active := 0
	for _, win := range nativeWindows {
		if gtkWindowIsActive(win.window()) != 0 {
			active = 1
			break
		}
	}
	if active != appActive {
		appActive = active
		kind := gkEventAppInactive
		if active != 0 {
			kind = gkEventAppActive
		}
		dispatchNativeEvent(appEventHandle, gtkEvent{kind: kind})
	}
}
func gkGTKAppAttach(handle cUintptr) { appEventHandle = uintptr(handle) }
func gkGTKWindowSetTheme(pointer unsafe.Pointer, theme cInt) {
	if win := nativeWin(pointer); win != nil {
		win.theme = int(theme)
		applyNativeTheme()
		notifyNativeThemes()
	}
}
func gkGTKWindowResolvedTheme(pointer unsafe.Pointer) cInt {
	if win := nativeWin(pointer); win != nil && win.theme != 0 {
		return cInt(win.theme)
	}
	return cInt(resolvedNativeTheme())
}
func nativeMonitor(win *nativeWinState) *gdkMonitor {
	display := gdkDisplayGetDefault()
	if display == nil {
		return nil
	}
	if win != nil && gtkWidgetGetMapped(win.widget) != 0 {
		if monitor := gdkDisplayGetMonitorAtSurface(display, win.surface()); monitor != nil {
			return monitor
		}
	}
	monitors := gdkDisplayGetMonitors(display)
	if gListModelGetNItems(monitors) == 0 {
		return nil
	}
	monitor := (*gdkMonitor)(gListModelGetItem(monitors, 0))
	gObjectUnref(unsafe.Pointer(monitor))
	return monitor
}
func gkGTKWindowScreen(pointer unsafe.Pointer, values *cDouble, name **cChar) {
	geometry := gdkRectangle{width: 1, height: 1}
	workarea := geometry
	scale, dpi, refresh := 1.0, 96.0, 60.0
	label := ""
	if monitor := nativeMonitor(nativeWin(pointer)); monitor != nil {
		gdkMonitorGetGeometry(monitor, &geometry)
		workarea = geometry
		if isNative(unsafe.Pointer(monitor), gdkX11MonitorGetType()) {
			gdkX11MonitorGetWorkarea(monitor, &workarea)
		}
		scale = float64(gdkMonitorGetScale(monitor))
		if width := gdkMonitorGetWidthMm(monitor); width > 0 {
			dpi = float64(geometry.width) * scale / (float64(width) / 25.4)
		} else {
			dpi = 96 * scale
		}
		if rate := gdkMonitorGetRefreshRate(monitor); rate > 0 {
			refresh = float64(rate) / 1000
		}
		label = goString(gdkMonitorGetDescription(monitor))
		if label == "" {
			label = goString(gdkMonitorGetConnector(monitor))
		}
	}
	copy(unsafe.Slice(values, 11), []cDouble{
		cDouble(geometry.x),
		cDouble(geometry.y),
		cDouble(geometry.width),
		cDouble(geometry.height),
		cDouble(workarea.x),
		cDouble(workarea.y),
		cDouble(workarea.width),
		cDouble(workarea.height),
		cDouble(scale),
		cDouble(dpi),
		cDouble(refresh),
	})
	*name = gStrdup(stringPointer(label))
}
func gkGTKSystemPreferences(pointer unsafe.Pointer, values *cDouble, locale **cChar) {
	settings := unsafe.Pointer(gtkSettingsGetDefault())
	scale := float64(nativeIntProperty(settings, "gtk-xft-dpi", 0)) / 1024 / 96
	if scale <= 0 {
		scale = 1
	}
	animations := nativeBoolProperty(settings, "gtk-enable-animations", true)
	contrast := nativeIntProperty(settings, "gtk-interface-contrast", 0) == 2
	accents := map[string][3]byte{
		"blue":   {0x35, 0x84, 0xe4},
		"teal":   {0x21, 0x90, 0xa4},
		"green":  {0x3a, 0x94, 0x4a},
		"yellow": {0xc8, 0x88, 0},
		"orange": {0xed, 0x5b, 0},
		"red":    {0xe6, 0x2d, 0x42},
		"pink":   {0xd5, 0x61, 0x99},
		"purple": {0x91, 0x41, 0xac},
		"slate":  {0x6f, 0x83, 0x96},
	}
	accent, ok := accents[desktopSetting("accent-color")]
	if !ok {
		accent = accents["blue"]
	}
	copy(unsafe.Slice(values, 10), []cDouble{
		cDouble(gkGTKWindowResolvedTheme(pointer)),
		cDouble(scale),
		cDouble(nativeBool(contrast)),
		cDouble(nativeBool(!animations)),
		0,
		cDouble(accent[0]) / 255,
		cDouble(accent[1]) / 255,
		cDouble(accent[2]) / 255,
		1,
		cDouble(nativeBool(gtkWidgetGetDefaultDirection() == gtkTextDirRTL)),
	})
	tag := "en"
	if languages := gGetLanguageNames(); languages != nil && *languages != nil {
		tag = goString(*languages)
	}
	if cut := strings.IndexAny(tag, ".@"); cut >= 0 {
		tag = tag[:cut]
	}
	tag = strings.ReplaceAll(tag, "_", "-")
	if tag == "" || tag == "C" || tag == "POSIX" {
		tag = "en"
	}
	*locale = gStrdup(stringPointer(tag))
}

func gkGTKViewColorSpace(pointer unsafe.Pointer) cInt { return 1 }
