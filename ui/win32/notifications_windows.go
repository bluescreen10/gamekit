package win32

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// Notifications are Windows toast notifications, which the backend reaches
// through the ABI of the Windows Runtime. An application without a package
// identity needs an application user model ID that Windows knows, so the
// backend registers one for the executable under
// HKEY_CURRENT_USER\Software\Classes\AppUserModelId and gives it to the
// process. A notification with a delivery time waits in the process, like on
// the other backends, so its activation reaches the application.

const notificationGroup = "gamekit"

var (
	notificationReceiver eventReceiver
	// scheduled holds the timers of the notifications that wait for their
	// delivery time.
	scheduled = map[string]uintptr{}
	// groups records the thread of each notification, which is its group in
	// the notification center.
	groups = map[string]string{}

	appIDValue       string
	toastManager     comObject
	toastNotifier    comObject
	toastHistory     comObject
	toastAttempted   bool
	historyAttempted bool
)

// appID returns the application user model ID of the process, and registers
// one when the process has none.
func appID() string {
	if appIDValue != "" {
		return appIDValue
	}
	var current uintptr
	if win32GetCurrentProcessExplicitAppUserModelID(&current).succeeded() && current != 0 {
		appIDValue = goStringAt(current)
		win32CoTaskMemFree(current)
		return appIDValue
	}
	executable, _ := os.Executable()
	name := strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
	var id strings.Builder
	id.WriteString("GameKit.")
	for _, c := range name {
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '.' {
			id.WriteRune(c)
		} else {
			id.WriteByte('_')
		}
	}
	appIDValue = id.String()
	var key uintptr
	if win32RegCreateKeyEx(hkeyCurrentUser, `Software\Classes\AppUserModelId\`+appIDValue, 0, keySetValue,
		&key) == errorSuccess {
		units := utf16.Encode([]rune(name + "\x00"))
		win32RegSetValueEx(key, "DisplayName", regSz, unsafe.Slice((*byte)(unsafe.Pointer(&units[0])), len(units)*2))
		win32RegCloseKey(key)
	}
	win32SetCurrentProcessExplicitAppUserModelID(appIDValue)
	return appIDValue
}

func notifier() comObject {
	if !toastAttempted {
		toastAttempted = true
		toastManager = activationFactory("Windows.UI.Notifications.ToastNotificationManager",
			iidToastNotificationManagerStatics)
		if toastManager != 0 {
			id := win32WindowsCreateString(appID())
			win32ToastNotificationManagerStaticsCreateToastNotifierWithId(toastManager, id, &toastNotifier)
			win32WindowsDeleteString(id)
		}
	}
	return toastNotifier
}

func history() comObject {
	if !historyAttempted && notifier() != 0 {
		historyAttempted = true
		if manager := query(toastManager, iidToastNotificationManagerStatics2); manager != 0 {
			win32ToastNotificationManagerStatics2GetHistory(manager, &toastHistory)
			win32UnknownRelease(manager)
		}
	}
	return toastHistory
}

func notificationsAvailable() bool {
	return notifier() != 0
}

// setNotificationReceiver sets the receiver of the activations of
// notifications.
func setNotificationReceiver(receiver eventReceiver) {
	notificationReceiver = receiver
}

// toastsEnabled reports whether the user allows notifications for the whole
// system.
func toastsEnabled() bool {
	enabled := uint32(1)
	size := uint32(unsafe.Sizeof(enabled))
	win32RegGetValue(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\PushNotifications`,
		"ToastEnabled", rrfRtRegDword, nil, unsafe.Pointer(&enabled), &size)
	return enabled != 0
}

// notificationAuthorization returns the authorization of the
// notifications of the application. Windows does not ask the user; it allows
// the notifications of an application until the user turns them off.
func notificationAuthorization() ui.NotificationAuthorization {
	toasts := notifier()
	if toasts == 0 {
		return ui.NotificationDenied
	}
	var setting int32
	result := win32ToastNotifierGetSetting(toasts, &setting)
	if result == errorNotFoundResult {
		// Windows lists an application in its notification settings after its
		// first notification, so before that the setting is not found and the
		// default applies.
		if toastsEnabled() {
			return ui.NotificationAuthorized
		}
		return ui.NotificationDenied
	}
	// Setting 0 is Enabled.
	if result.succeeded() && setting == 0 {
		return ui.NotificationAuthorized
	}
	return ui.NotificationDenied
}

func requestNotificationAuthorization() ui.NotificationAuthorization {
	return notificationAuthorization()
}

func xmlText(text string) string {
	var out strings.Builder
	for _, c := range text {
		switch c {
		case '&':
			out.WriteString("&amp;")
		case '<':
			out.WriteString("&lt;")
		case '>':
			out.WriteString("&gt;")
		case '"':
			out.WriteString("&quot;")
		default:
			out.WriteRune(c)
		}
	}
	return out.String()
}

func notificationGroupOf(id string) string {
	if group, ok := groups[id]; ok {
		return group
	}
	return notificationGroup
}

// truncate returns the first n UTF-16 units of text.
func truncate(text string, n int) string {
	units := utf16.Encode([]rune(text))
	return string(utf16.Decode(units[:min(len(units), n)]))
}

// tag returns the tag of a notification, which has at most 64 characters.
func tag(id string) string {
	return truncate(id, 64)
}

// argument returns a value of the arguments of an activation, which are
// name=value pairs separated by ampersands, with the values percent-encoded.
func argument(arguments, name string) string {
	for _, part := range strings.Split(arguments, "&") {
		value, found := strings.CutPrefix(part, name+"=")
		if !found {
			continue
		}
		var decoded []byte
		for i := 0; i < len(value); i++ {
			if value[i] == '%' && i+2 < len(value) {
				if b, err := strconv.ParseUint(value[i+1:i+3], 16, 8); err == nil {
					decoded = append(decoded, byte(b))
					i += 2
					continue
				}
			}
			decoded = append(decoded, value[i])
		}
		return string(decoded)
	}
	return ""
}

func encodeArgument(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < 0x80 && (unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) || c == '-' || c == '_' || c == '.') {
			out.WriteByte(c)
		} else {
			fmt.Fprintf(&out, "%%%02X", c)
		}
	}
	return out.String()
}

// toastActivated receives the activation of a toast, on a thread of the
// notification platform, and reports it on the UI thread.
func toastActivated(id string, value comObject) {
	var arguments, reply string
	if args := query(value, iidToastActivatedEventArgs); args != 0 {
		var text hstring
		if win32ToastActivatedEventArgsGetArguments(args, &text).succeeded() {
			arguments = takeString(text)
		}
		win32UnknownRelease(args)
	}
	if args2 := query(value, iidToastActivatedEventArgs2); args2 != 0 {
		var input comObject
		if win32ToastActivatedEventArgs2GetUserInput(args2, &input).succeeded() && input != 0 {
			if values := query(input, iidStringObjectMap); values != 0 {
				key := win32WindowsCreateString("reply")
				var found bool
				var entry comObject
				if win32MapHasKey(values, key, &found).succeeded() && found &&
					win32MapLookup(values, key, &entry).succeeded() && entry != 0 {
					if property := query(entry, iidPropertyValue); property != 0 {
						var text hstring
						if win32PropertyValueGetString(property, &text).succeeded() {
							reply = takeString(text)
						}
						win32UnknownRelease(property)
					}
					win32UnknownRelease(entry)
				}
				win32WindowsDeleteString(key)
				win32UnknownRelease(values)
			}
			win32UnknownRelease(input)
		}
		win32UnknownRelease(args2)
	}
	action := argument(arguments, "action")
	later(func() {
		if notificationReceiver != nil {
			send(notificationReceiver, notificationEvent{activation: ui.NotificationActivation{
				NotificationID: id,
				ActionID:       action,
				ReplyText:      reply,
			}})
		}
	})
}

// toastShow shows a toast with its XML.
func toastShow(id, xml string) hresult {
	toasts := notifier()
	if toasts == 0 {
		return eNotImpl
	}
	documentClass := win32WindowsCreateString("Windows.Data.Xml.Dom.XmlDocument")
	defer win32WindowsDeleteString(documentClass)
	var document comObject
	result := win32RoActivateInstance(documentClass, &document)
	if result.failed() {
		return result
	}
	defer win32UnknownRelease(document)
	io := query(document, iidXmlDocumentIO)
	if io == 0 {
		return eNoInterface
	}
	defer win32UnknownRelease(io)
	text := win32WindowsCreateString(xml)
	result = win32XmlDocumentIOLoadXml(io, text)
	win32WindowsDeleteString(text)
	if result.failed() {
		return result
	}
	factory := activationFactory("Windows.UI.Notifications.ToastNotification", iidToastNotificationFactory)
	if factory == 0 {
		return eNoInterface
	}
	defer win32UnknownRelease(factory)
	var toast comObject
	if result = win32ToastNotificationFactoryCreateToastNotification(factory, document, &toast); result.failed() {
		return result
	}
	defer win32UnknownRelease(toast)
	if toast2 := query(toast, iidToastNotification2); toast2 != 0 {
		tagText, group := win32WindowsCreateString(tag(id)), win32WindowsCreateString(notificationGroupOf(id))
		win32ToastNotification2PutTag(toast2, tagText)
		win32ToastNotification2PutGroup(toast2, group)
		win32WindowsDeleteString(tagText)
		win32WindowsDeleteString(group)
		win32UnknownRelease(toast2)
	}
	handler := newEventHandler(iidToastActivatedHandler, func(_, args comObject) {
		toastActivated(id, args)
	})
	var registration eventRegistrationToken
	win32ToastNotificationAddActivated(toast, handler.com(), &registration)
	win32UnknownRelease(handler.com())
	return win32ToastNotifierShow(toasts, toast)
}

// toastXML returns the XML of the toast of a notification. Windows has no
// notification categories; the actions of a notification come with it.
func toastXML(n ui.Notification, attachment string) string {
	var xml strings.Builder
	id := encodeArgument(n.ID)
	xml.WriteString(`<toast launch="id=` + id + `"><visual><binding template="ToastGeneric">`)
	xml.WriteString("<text>" + xmlText(n.Title) + "</text>")
	if n.Subtitle != "" {
		xml.WriteString("<text>" + xmlText(n.Subtitle) + "</text>")
	}
	if n.Body != "" {
		xml.WriteString("<text>" + xmlText(n.Body) + "</text>")
	}
	if attachment != "" {
		path := strings.ReplaceAll(attachment, `\`, "/")
		xml.WriteString(`<image placement="appLogoOverride" src="file:///` + xmlText(path) + `"/>`)
	}
	xml.WriteString("</binding></visual>")
	if len(n.Actions) > 0 {
		xml.WriteString("<actions>")
		for _, action := range n.Actions {
			if action.TextInput {
				xml.WriteString(`<input id="reply" type="text" placeHolderContent="` + xmlText(action.Text) + `"/>`)
				break
			}
		}
		for _, action := range n.Actions[:min(len(n.Actions), 5)] {
			xml.WriteString(`<action content="` + xmlText(action.Text) + `" arguments="id=` + id + `&amp;action=` +
				encodeArgument(action.ID) + `"`)
			if action.TextInput {
				xml.WriteString(` hint-inputId="reply"`)
			}
			if action.Destructive {
				xml.WriteString(` hint-buttonStyle="Critical"`)
			}
			xml.WriteString("/>")
		}
		xml.WriteString("</actions>")
	}
	if n.Sound {
		xml.WriteString(`<audio src="ms-winsoundevent:Notification.Default"/>`)
	} else {
		xml.WriteString(`<audio silent="true"/>`)
	}
	xml.WriteString("</toast>")
	return xml.String()
}

// sendNotification sends a notification, or schedules it for its delivery
// time. The thread of a notification groups it in the notification center.
// Windows has no notification categories; the actions of a notification come
// with it. attachment is the path of the image of the notification.
func sendNotification(n ui.Notification, attachment string) error {
	group := notificationGroup
	if n.ThreadID != "" {
		group = truncate(n.ThreadID, 64)
	}
	groups[n.ID] = group
	xml := toastXML(n, attachment)
	if delay := time.Until(n.Delivery); !n.Delivery.IsZero() && delay > 0 {
		if timer, ok := scheduled[n.ID]; ok {
			cancelTimer(timer)
			delete(scheduled, n.ID)
		}
		id := n.ID
		scheduled[id] = startTimer(uint32(delay.Milliseconds()), func() bool {
			delete(scheduled, id)
			toastShow(id, xml)
			return false
		})
		return nil
	}
	if result := toastShow(n.ID, xml); result.failed() {
		return fmt.Errorf("the notification could not be sent (0x%08x)", uint32(result))
	}
	return nil
}

// pendingNotifications returns the notifications that wait for their
// delivery time.
func pendingNotifications() []string {
	ids := make([]string, 0, len(scheduled))
	for id := range scheduled {
		ids = append(ids, id)
	}
	return ids
}

// deliveredNotifications returns the notifications in the notification
// center. Every notification in the history of the application ID is one that
// the application sent.
func deliveredNotifications() []string {
	var ids []string
	recorded := history()
	if recorded == 0 {
		return nil
	}
	history2 := query(recorded, iidToastNotificationHistory2)
	if history2 == 0 {
		return nil
	}
	defer win32UnknownRelease(history2)
	app := win32WindowsCreateString(appID())
	defer win32WindowsDeleteString(app)
	var toasts comObject
	if win32ToastNotificationHistory2GetHistoryWithId(history2, app, &toasts).failed() || toasts == 0 {
		return nil
	}
	defer win32UnknownRelease(toasts)
	var count uint32
	win32VectorViewGetSize(toasts, &count)
	for i := range count {
		var toast comObject
		if win32VectorViewGetAt(toasts, i, &toast).failed() || toast == 0 {
			continue
		}
		if toast2 := query(toast, iidToastNotification2); toast2 != 0 {
			var text hstring
			if win32ToastNotification2GetTag(toast2, &text).succeeded() {
				ids = append(ids, takeString(text))
			}
			win32UnknownRelease(toast2)
		}
		win32UnknownRelease(toast)
	}
	return ids
}

func withdrawNotifications(ids []string) {
	app := win32WindowsCreateString(appID())
	defer win32WindowsDeleteString(app)
	for _, id := range ids {
		if timer, ok := scheduled[id]; ok {
			cancelTimer(timer)
			delete(scheduled, id)
		}
		if recorded := history(); recorded != 0 {
			tagText, group := win32WindowsCreateString(tag(id)), win32WindowsCreateString(notificationGroupOf(id))
			win32ToastNotificationHistoryRemoveGroupedTagWithId(recorded, tagText, group, app)
			win32WindowsDeleteString(tagText)
			win32WindowsDeleteString(group)
		}
	}
}

// ---- Badge ----
//
// The badge is an overlay icon on the taskbar buttons of the windows, the badge
// that Windows shows for desktop applications.

var (
	badgeText        string
	taskbar          comObject
	taskbarAttempted bool
)

// badgeIcon draws a red disc with white text.
func badgeIcon(text string) hicon {
	const size = 32
	header := bitmapV5Header{
		bV5Width:       size,
		bV5Height:      -size,
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
	for y := range size {
		for x := range size {
			distance := math.Hypot(float64(x)+0.5-size/2.0, float64(y)+0.5-size/2.0)
			coverage := 0.0
			switch {
			case distance < size/2.0-1:
				coverage = 1
			case distance < size/2.0:
				coverage = size/2.0 - distance
			}
			alpha := uint32(coverage * 255)
			pixels[y*size+x] = alpha<<24 | 0xc4*alpha/255<<16 | 0x2b*alpha/255<<8 | 0x1c*alpha/255
		}
	}
	dc := win32CreateCompatibleDC(0)
	previous := win32SelectObject(dc, hgdiobj(color))
	height := int32(-19)
	if len(utf16.Encode([]rune(text))) > 2 {
		height = -13
	}
	font := win32CreateFont(height, 0, 0, 0, fwBold, false, false, false, defaultCharset, outDefaultPrecis,
		clipDefaultPrecis, antialiasedQuality, defaultPitch, "Segoe UI")
	previousFont := win32SelectObject(dc, hgdiobj(font))
	win32SetBkMode(dc, transparent)
	win32SetTextColor(dc, 0x00ffffff)
	area := rect{right: size, bottom: size}
	win32GdiFlush()
	before := append([]uint32(nil), pixels...)
	win32DrawText(dc, text, &area, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
	win32GdiFlush()
	// GDI text leaves the alpha channel at zero; pixels it changed are made
	// opaque.
	for i := range pixels {
		if pixels[i] != before[i] {
			pixels[i] |= 0xff000000
		}
	}
	win32SelectObject(dc, previousFont)
	win32SelectObject(dc, previous)
	win32DeleteObject(hgdiobj(font))
	win32DeleteDC(dc)
	mask := win32CreateBitmap(size, size, 1, 1, nil)
	info := iconInfo{fIcon: 1, hbmColor: color, hbmMask: mask}
	icon := win32CreateIconIndirect(&info)
	win32DeleteObject(hgdiobj(color))
	win32DeleteObject(hgdiobj(mask))
	return icon
}

func taskbarList() comObject {
	if !taskbarAttempted {
		taskbarAttempted = true
		if win32CoCreateInstance(clsidTaskbarList, 0, clsctxInprocServer, iidTaskbarList3, &taskbar).failed() {
			taskbar = 0
		} else if win32TaskbarListHrInit(taskbar).failed() {
			win32UnknownRelease(taskbar)
			taskbar = 0
		}
	}
	return taskbar
}

func setBadge(badge string) {
	badgeText = badge
	list := taskbarList()
	if list == 0 {
		return
	}
	var icon hicon
	if badge != "" {
		icon = badgeIcon(badge)
	}
	for h, win := range windowsByHWND {
		if h != 0 && !win.utility {
			win32TaskbarList3SetOverlayIcon(list, h, icon, badge)
		}
	}
	if icon != 0 {
		win32DestroyIcon(icon)
	}
}
