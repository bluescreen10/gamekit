package win32

import (
	"math"
	"unicode"
	"unicode/utf16"
	"unsafe"
)

// A system tray item is an icon in the notification area of the taskbar,
// which Shell_NotifyIcon manages through a hidden window. The notification
// area shows only icons, so an item with a title and no icon shows the first
// letters of the title as its icon, and an item that wants attention shows a
// dot on its icon.

const (
	trayMessage = wmApp + 0x47
	trayClass   = "GameKitTray"
)

type nativeTray struct {
	receiver eventReceiver
	hwnd     hwnd
	id       uint32
	// image is the icon that the application gives, and icon is the icon
	// that the notification area shows, drawn from it.
	image     *iconImage
	icon      hicon
	title     string
	tooltip   string
	visible   bool
	attention bool
	added     bool
	menu      *nativeMenu
}

var (
	trays               = map[hwnd]*nativeTray{}
	trayClassRegistered bool
	taskbarCreated      uint32
	nextTrayID          uint32
)

// drawIcon draws the icon of an item: the glyph, the image, or the initials
// of the title, with a dot when the item wants attention.
func (t *nativeTray) drawIcon() hicon {
	size := int(win32GetSystemMetrics(smCxSmIcon))
	if size <= 0 {
		size = 16
	}
	header := bitmapV5Header{
		bV5Width:       int32(size),
		bV5Height:      -int32(size),
		bV5Planes:      1,
		bV5BitCount:    32,
		bV5Compression: biBitfields,
		bV5RedMask:     0x00ff0000,
		bV5GreenMask:   0x0000ff00,
		bV5BlueMask:    0x000000ff,
		bV5AlphaMask:   0xff000000,
	}
	header.bV5Size = uint32(unsafe.Sizeof(header))
	var bits uintptr
	screen := win32GetDC(0)
	color := win32CreateDIBSection(screen, unsafe.Pointer(&header), dibRGBColors, &bits, 0, 0)
	win32ReleaseDC(0, screen)
	if color == 0 {
		return 0
	}
	pixels := unsafe.Slice((*uint32)(memoryAt(bits)), size*size)
	clear(pixels)
	var glyph string
	if t.image != nil {
		glyph = t.image.glyph
	}
	if t.image != nil && len(t.image.pixels) > 0 {
		image := t.image
		for y := range size {
			for x := range size {
				sx, sy := x*image.width/size, y*image.height/size
				p := image.pixels[(sy*image.width+sx)*4:]
				a := uint32(p[3])
				pixels[y*size+x] = a<<24 | uint32(p[0])*a/255<<16 | uint32(p[1])*a/255<<8 | uint32(p[2])*a/255
			}
		}
	} else {
		// The notification area follows the taskbar theme, which is dark
		// unless the system uses light mode for apps and the taskbar.
		var light uint32
		length := uint32(unsafe.Sizeof(light))
		win32RegGetValue(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
			"SystemUsesLightTheme", rrfRtRegDword, nil, unsafe.Pointer(&light), &length)
		var ink uint32 = 255
		if light != 0 {
			ink = 0
		}
		text, face := glyph, symbolFamily()
		height, weight := size, int32(fwNormal)
		if text == "" {
			face, height, weight = "Segoe UI", size*3/4, fwBold
			var initials []rune
			for _, c := range t.title {
				if (unicode.IsLetter(c) || unicode.IsDigit(c)) && len(utf16.Encode(initials)) < 2 {
					initials = append(initials, c)
				}
			}
			text = string(initials)
		}
		dc := win32CreateCompatibleDC(0)
		previous := win32SelectObject(dc, hgdiobj(color))
		font := win32CreateFont(-int32(height), 0, 0, 0, weight, false, false, false, defaultCharset,
			outDefaultPrecis, clipDefaultPrecis, antialiasedQuality, defaultPitch, face)
		previousFont := win32SelectObject(dc, hgdiobj(font))
		win32SetBkMode(dc, transparent)
		// The text is drawn white on black, so that its coverage is the value
		// of each pixel, and then takes the ink color.
		win32SetTextColor(dc, 0x00ffffff)
		area := rect{right: int32(size), bottom: int32(size)}
		win32DrawText(dc, text, &area, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
		win32GdiFlush()
		// GDI text has no alpha; the coverage becomes the alpha of the ink,
		// with the color premultiplied.
		for i, pixel := range pixels {
			coverage := pixel & 0xff
			value := ink * coverage / 255
			pixels[i] = coverage<<24 | value<<16 | value<<8 | value
		}
		win32SelectObject(dc, previousFont)
		win32SelectObject(dc, previous)
		win32DeleteObject(hgdiobj(font))
		win32DeleteDC(dc)
	}
	if t.attention {
		radius := size / 4
		cx, cy := float64(size-radius-1), float64(radius+1)
		for y := range size {
			for x := range size {
				if math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) <= float64(radius) {
					pixels[y*size+x] = 0xffc42b1c
				}
			}
		}
	}
	mask := win32CreateBitmap(int32(size), int32(size), 1, 1, nil)
	info := iconInfo{fIcon: 1, hbmColor: color, hbmMask: mask}
	icon := win32CreateIconIndirect(&info)
	win32DeleteObject(hgdiobj(color))
	win32DeleteObject(hgdiobj(mask))
	return icon
}

func (t *nativeTray) notifyData() notifyIconData {
	data := notifyIconData{hWnd: t.hwnd, uID: t.id}
	data.cbSize = uint32(unsafe.Sizeof(data))
	return data
}

func (t *nativeTray) sync() {
	data := t.notifyData()
	data.uFlags = nifMessage | nifIcon | nifTip | nifState | nifShowTip
	data.uCallbackMessage = trayMessage
	if t.icon != 0 {
		win32DestroyIcon(t.icon)
	}
	t.icon = t.drawIcon()
	data.hIcon = t.icon
	tip := t.tooltip
	if tip == "" {
		tip = t.title
	}
	tipText := utf16.Encode([]rune(tip))
	copy(data.szTip[:len(data.szTip)-1], tipText)
	data.dwStateMask = nisHidden
	if !t.visible {
		data.dwState = nisHidden
	}
	if !t.added {
		t.added = win32ShellNotifyIcon(nimAdd, &data)
		data.uVersion = notifyIconVersion4
		win32ShellNotifyIcon(nimSetVersion, &data)
	} else {
		win32ShellNotifyIcon(nimModify, &data)
	}
}

func (t *nativeTray) destroy() {
	t.receiver = nil
	if t.added {
		data := t.notifyData()
		win32ShellNotifyIcon(nimDelete, &data)
	}
	if t.icon != 0 {
		win32DestroyIcon(t.icon)
	}
	if t.hwnd != 0 {
		delete(trays, t.hwnd)
		win32DestroyWindow(t.hwnd)
	}
}

func (t *nativeTray) showMenu(x, y int32) {
	menu := t.menu
	if menu == nil {
		return
	}
	popup, commands, bitmaps := menuNative(menu)
	// The menu closes when the user clicks elsewhere only if the window of the
	// menu is in the foreground.
	win32SetForegroundWindow(t.hwnd)
	command := uint32(win32TrackPopupMenuEx(popup, tpmReturnCmd|tpmNoNotify|tpmRightButton, x, y, t.hwnd, nil))
	win32PostMessage(t.hwnd, wmNull, 0, 0)
	win32DestroyMenu(popup)
	for _, bitmap := range bitmaps {
		win32DeleteObject(hgdiobj(bitmap))
	}
	if command >= 1 && int(command) <= len(commands) {
		entry := commands[command-1]
		menuInvoke(entry.root, entry.id)
	}
}

func trayProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	tray := trays[h]
	if tray != nil && taskbarCreated != 0 && message == taskbarCreated {
		// Explorer restarted, and the item must be added again.
		tray.added = false
		tray.sync()
		return 0
	}
	if tray != nil && message == trayMessage {
		x, y := int32(int16(wparam)), int32(int16(wparam>>16))
		switch uint32(lparam & 0xffff) {
		case ninSelect, ninKeySelect:
			send(tray.receiver, activateEvent{})
		case wmContextMenu:
			tray.showMenu(x, y)
		}
		return 0
	}
	return win32DefWindowProc(h, message, wparam, lparam)
}

func newTray(receiver eventReceiver) *nativeTray {
	if !trayClassRegistered {
		trayClassRegistered = true
		taskbarCreated = win32RegisterWindowMessage("TaskbarCreated")
		class := wndClassEx{lpfnWndProc: callback("tray", trayProc), hInstance: instance()}
		registerClass(&class, trayClass)
	}
	nextTrayID++
	tray := &nativeTray{id: nextTrayID, visible: true}
	tray.receiver = receiver
	tray.hwnd = win32CreateWindowEx(0, trayClass, "", wsPopup, 0, 0, 0, 0, 0, 0, instance(), nil)
	trays[tray.hwnd] = tray
	return tray
}

func (tray *nativeTray) configure(icon *iconImage, isTemplate bool, title string, tooltip string) {
	if tray == nil {
		return
	}
	tray.image = icon
	tray.title = title
	tray.tooltip = tooltip
	tray.sync()
}

func (tray *nativeTray) setVisible(visible bool) {
	if tray != nil {
		tray.visible = visible
		tray.sync()
	}
}

func (tray *nativeTray) setAttention(attention bool) {
	if tray != nil {
		tray.attention = attention
		tray.sync()
	}
}

func (tray *nativeTray) setMenu(menu *nativeMenu) {
	if tray != nil {
		tray.menu = menu
	}
}

// bounds returns the bounds of the icon of an item on the screen, in
// points, or false when it is not shown.
func (tray *nativeTray) bounds() (x, y, width, height float64, ok bool) {
	if tray == nil || !tray.added || !tray.visible {
		return 0, 0, 0, 0, false
	}
	identifier := notifyIconIdentifier{hWnd: tray.hwnd, uID: tray.id}
	identifier.cbSize = uint32(unsafe.Sizeof(identifier))
	var area rect
	if win32ShellNotifyIconGetRect(&identifier, &area).failed() {
		return 0, 0, 0, 0, false
	}
	scale := float64(win32GetDpiForSystem()) / 96
	return float64(area.left) / scale, float64(area.top) / scale, float64(area.right-area.left) / scale,
		float64(area.bottom-area.top) / scale, true
}
