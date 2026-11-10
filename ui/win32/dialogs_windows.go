package win32

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// programName returns the name of the program without its extension.
func programName() string {
	path, err := os.Executable()
	if err != nil {
		path = os.Args[0]
	}
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// ---- Task dialogs ----
//
// An alert is the task dialog of Windows. A task dialog runs its own message
// loop until it closes, so it runs on a thread of its own while the Go layer
// polls the alert and the window keeps its messages flowing.

const firstButton = 100

type nativeAlert struct {
	receiver    eventReceiver
	severity    int
	title       string
	message     string
	details     string
	buttons     []string
	destructive []bool
	cancel      int
	prompt      bool
	secure      bool
	text        string
	placeholder string
	icon        *iconImage
	owner       hwnd
	// running is set while the thread of a task dialog runs, and finished is
	// closed when it ends.
	running  bool
	finished chan struct{}
	dialog   atomic.Uintptr
	response atomic.Int32
	done     atomic.Bool
	// window is the window of a prompt, which is a dialog of the backend.
	window   hwnd
	edit     hwnd
	controls []hwnd
	answer   string
}

func (a *nativeAlert) taskButton(command int) int {
	if command >= firstButton && command < firstButton+len(a.buttons) {
		return command - firstButton
	}
	if a.cancel >= 0 {
		return a.cancel
	}
	return 0
}

// taskAlerts holds the alerts whose task dialogs show, by the identifier that
// their callback receives.
var (
	taskAlertsMu  sync.Mutex
	taskAlerts    = map[uintptr]*nativeAlert{}
	nextTaskAlert uintptr
)

func taskCallback(h hwnd, notification uint32, wparam, lparam, data uintptr) uintptr {
	taskAlertsMu.Lock()
	alert := taskAlerts[data]
	taskAlertsMu.Unlock()
	if alert == nil {
		return sOK
	}
	switch notification {
	case tdnCreated:
		alert.dialog.Store(uintptr(h))
		if !alert.prompt && alert.owner != 0 && win32IsWindow(alert.owner) {
			win32SetForegroundWindow(h)
		}
	case tdnHyperlinkClicked:
		win32ShellExecute(0, "open", goStringAt(lparam), "", "", swShowNormal)
	case tdnDestroyed:
		alert.dialog.Store(0)
	}
	return sOK
}

// runTask shows the task dialog of an alert on the calling thread until it
// closes.
func (a *nativeAlert) runTask() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(a.finished)
	win32CoInitializeEx(0, coinitApartmentThreaded)
	defer win32CoUninitialize()
	defer deactivate(activate())
	taskAlertsMu.Lock()
	nextTaskAlert++
	id := nextTaskAlert
	taskAlerts[id] = a
	taskAlertsMu.Unlock()
	defer func() {
		taskAlertsMu.Lock()
		delete(taskAlerts, id)
		taskAlertsMu.Unlock()
	}()
	config := taskDialogConfig{
		hwndParent:             a.owner,
		hInstance:              instance(),
		dwFlags:                tdfPositionRelativeToWindow | tdfSizeToContent | tdfEnableHyperlinks,
		pszWindowTitle:         programName(),
		pszMainInstruction:     a.title,
		pszContent:             a.message,
		pszExpandedInformation: a.details,
		nDefaultButton:         firstButton,
		pfCallback:             callback("taskDialog", taskCallback),
		lpCallbackData:         id,
	}
	if a.cancel >= 0 {
		config.dwFlags |= tdfAllowDialogCancellation
	}
	var icon hicon
	if a.icon != nil {
		icon = iconHandle(a.icon, int(math.Round(32*scaleOf(a.owner))), paletteFor(resolvedTheme()).text)
		config.dwFlags |= tdfUseHIconMain
		config.mainIcon = uintptr(icon)
	} else {
		switch a.severity {
		case 2:
			config.mainIcon = tdWarningIcon
		case 3:
			config.mainIcon = tdErrorIcon
		default:
			config.mainIcon = tdInformationIcon
		}
	}
	for i, title := range a.buttons {
		config.buttons = append(config.buttons, taskDialogButton{nButtonID: int32(firstButton + i), pszButtonText: title})
	}
	command := int32(idCancel)
	if win32TaskDialogIndirect(&config, &command).failed() {
		command = idCancel
	}
	if icon != 0 {
		win32DestroyIcon(icon)
	}
	a.response.Store(int32(a.taskButton(int(command))))
	a.done.Store(true)
}

// waitTask waits for the thread of a task dialog to end, keeping the messages
// of the window flowing: the thread can send messages to the window.
func (a *nativeAlert) waitTask() {
	if !a.running {
		return
	}
	for {
		select {
		case <-a.finished:
			a.running = false
			return
		default:
		}
		pumpOnce()
		win32MsgWaitForMultipleObjectsEx(0, nil, 10, qsAllInput, mwmoInputAvailable)
	}
}

func (a *nativeAlert) destroy() {
	if a.running {
		if open := hwnd(a.dialog.Load()); open != 0 {
			win32PostMessage(open, tdmClickButton, idCancel, 0)
		}
		a.waitTask()
	}
	if a.window != 0 && win32IsWindow(a.window) {
		delete(prompts, a.window)
		win32DestroyWindow(a.window)
	}
	if a.owner != 0 && win32IsWindow(a.owner) {
		win32EnableWindow(a.owner, true)
	}
}

// ---- Prompts ----
//
// A prompt asks for a line of text, which a task dialog cannot do, so it is a
// dialog of the backend in the style of the task dialog.

const promptClass = "GameKitPrompt"

var (
	prompts               = map[hwnd]*nativeAlert{}
	promptClassRegistered bool
)

func promptProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	alert := prompts[h]
	switch message {
	case wmCommand:
		if alert != nil && hiword(wparam) == bnClicked {
			id := int(loword(wparam))
			if id == idOK {
				id = firstButton
			}
			if id >= firstButton && id < firstButton+len(alert.buttons) {
				alert.response.Store(int32(id - firstButton))
				alert.done.Store(true)
				return 0
			}
			if id == idCancel && alert.cancel >= 0 {
				alert.response.Store(int32(alert.cancel))
				alert.done.Store(true)
				return 0
			}
		}
	case wmClose:
		if alert != nil && !alert.done.Load() {
			alert.response.Store(int32(max(alert.cancel, 0)))
			alert.done.Store(true)
		}
		return 0
	case wmCtlColorStatic, wmCtlColorEdit, wmCtlColorBtn:
		return ctlColor(hwnd(lparam), hdc(wparam), message)
	case wmEraseBkgnd:
		var client rect
		win32GetClientRect(h, &client)
		pal := paletteFor(resolvedTheme())
		fillRectColor(hdc(wparam), &client, pal.window)
		// The buttons sit on the gray band of a task dialog.
		band := rgbColor(0xf0f0f0)
		if pal.dark {
			band = rgbColor(0x2b2b2b)
		}
		bottom := client
		bottom.top = client.bottom - int32(math.Round(42*scaleOf(h)))
		fillRectColor(hdc(wparam), &bottom, band)
		return 1
	}
	return win32DefWindowProc(h, message, wparam, lparam)
}

func (a *nativeAlert) buildPrompt() {
	if !promptClassRegistered {
		promptClassRegistered = true
		class := wndClassEx{
			lpfnWndProc: callback("prompt", promptProc),
			hInstance:   instance(),
			hCursor:     win32LoadCursor(0, idcArrow),
		}
		registerClass(&class, promptClass)
	}
	cookie := activate()
	defer deactivate(cookie)
	a.window = win32CreateWindowEx(wsExDlgModalFrame|wsExControlParent, promptClass, programName(),
		wsPopup|wsCaption|wsSysMenu|wsClipChildren, cwUseDefault, cwUseDefault, 400, 200, a.owner, 0, instance(), nil)
	prompts[a.window] = a
	dark := int32(0)
	if resolvedTheme() == 2 {
		dark = 1
	}
	win32DwmSetWindowAttribute(a.window, dwmwaUseImmersiveDarkMode, unsafe.Pointer(&dark), 4)
	h := a.window
	scale := scaleOf(h)
	dpi := win32GetDpiForWindow(h)
	body := bodyFont()
	heading := typeRamp(3)
	heading.weight = 400
	const width, pad = 360, 12
	px := func(value int) int32 {
		return int32(math.Round(float64(value) * scale))
	}
	y := pad
	var icon hicon
	iconID := uintptr(32516)
	switch a.severity {
	case 3:
		iconID = 32513
	case 2:
		iconID = 32515
	}
	win32LoadIconMetric(0, iconID, limLarge, &icon)
	image := makeNative(wcStatic, "", ssIcon|ssRealSizeControl, 0, h)
	win32SendMessage(image, stmSetIcon, uintptr(icon), 0)
	win32SetWindowPos(image, 0, px(pad), px(y), px(32), px(32), swpNoZOrder)
	textX := pad + 32 + 10
	textWidth := width - textX - pad
	headingSize := measureGDI(a.title, heading, textWidth, 0)
	title := makeNative(wcStatic, a.title, ssLeft|ssNoPrefix, 0, h)
	win32SendMessage(title, wmSetFont, uintptr(fontHandle(heading, dpi)), 0)
	setTextRole(title, 9)
	win32SetWindowPos(title, 0, px(textX), px(y+4), px(textWidth), px(headingSize.cy), swpNoZOrder)
	y += max(36, headingSize.cy+12)
	if a.message != "" {
		messageSize := measureGDI(a.message, body, textWidth, 0)
		label := makeNative(wcStatic, a.message, ssLeft|ssNoPrefix, 0, h)
		win32SendMessage(label, wmSetFont, uintptr(fontHandle(body, dpi)), 0)
		win32SetWindowPos(label, 0, px(textX), px(y), px(textWidth), px(messageSize.cy), swpNoZOrder)
		y += messageSize.cy + 10
	}
	line := measureGDI("Ag", body, 0, 0).cy + 8
	style := uint32(esAutoHScroll | wsTabStop | wsBorder)
	if a.secure {
		style |= esPassword
	}
	a.edit = makeNative(wcEdit, a.text, style, 0, h)
	win32SendMessage(a.edit, wmSetFont, uintptr(fontHandle(body, dpi)), 0)
	win32SendMessage(a.edit, emSetCueBanner, 1, uintptr(unsafe.Pointer(wide(a.placeholder))))
	if a.secure {
		win32SendMessage(a.edit, emSetPasswordChar, passwordCharacter, 0)
	}
	win32SetWindowPos(a.edit, 0, px(textX), px(y), px(textWidth), px(line), swpNoZOrder)
	y += line + 16
	bandTop := y
	x := width - pad
	for i := len(a.buttons) - 1; i >= 0; i-- {
		buttonWidth := max(75, measureGDI(a.buttons[i], body, 0, 0).cx+24)
		x -= buttonWidth
		buttonStyle := uint32(wsTabStop | bsPushButton)
		if i == 0 {
			buttonStyle = wsTabStop | bsDefPushButton
		}
		button := makeNative(wcButton, a.buttons[i], buttonStyle, 0, h)
		win32SetWindowLongPtr(button, gwlpID, uintptr(firstButton+i))
		win32SendMessage(button, wmSetFont, uintptr(fontHandle(body, dpi)), 0)
		win32SetWindowPos(button, 0, px(x), px(bandTop+9), px(buttonWidth), px(24), swpNoZOrder)
		x -= 8
		a.controls = append(a.controls, button)
	}
	height := bandTop + 42
	frame := rect{right: px(width), bottom: px(height)}
	win32AdjustWindowRectExForDpi(&frame, wsPopup|wsCaption|wsSysMenu, false, wsExDlgModalFrame, dpi)
	frameWidth, frameHeight := frame.right-frame.left, frame.bottom-frame.top
	var area rect
	if a.owner != 0 && win32IsWindowVisible(a.owner) {
		win32GetWindowRect(a.owner, &area)
	} else {
		info := monitorInfo{}
		info.cbSize = uint32(unsafe.Sizeof(info))
		win32GetMonitorInfo(win32MonitorFromPoint(point{}, monitorDefaultToPrimary), unsafe.Pointer(&info))
		area = info.rcWork
	}
	win32SetWindowPos(h, 0, area.left+(area.right-area.left-frameWidth)/2, area.top+(area.bottom-area.top-frameHeight)/2,
		frameWidth, frameHeight, swpNoZOrder|swpNoActivate)
	for child := win32GetWindow(h, gwChild); child != 0; child = win32GetWindow(child, gwHwndNext) {
		applyThemeTo(child, resolvedTheme(), "")
	}
}

// promptKey answers Return and Escape in a prompt.
func promptKey(message *msg) bool {
	if message.message != wmKeyDown || (message.wParam != vkReturn && message.wParam != vkEscape &&
		message.wParam != vkTab) {
		return false
	}
	root := win32GetAncestor(message.hwnd, gaRoot)
	if className(root) != promptClass {
		return false
	}
	if message.wParam == vkTab {
		if next := win32GetNextDlgTabItem(root, message.hwnd, keyDown(vkShift)); next != 0 {
			win32SetFocus(next)
		}
		return true
	}
	alert := prompts[root]
	if alert == nil || alert.done.Load() {
		return false
	}
	if message.wParam == vkReturn {
		alert.response.Store(0)
		alert.done.Store(true)
	} else if alert.cancel >= 0 {
		alert.response.Store(int32(alert.cancel))
		alert.done.Store(true)
	}
	return true
}

// ---- Alerts ----

func newNativeAlert(severity int, title string, message string, details string, buttons []string, cancelIndex int, destructiveMask int, prompt bool, secure bool, text string, placeholder string) *nativeAlert {
	if initialize() != nil {
		return nil
	}
	alert := &nativeAlert{
		severity:    severity,
		title:       title,
		message:     message,
		details:     details,
		buttons:     buttons,
		cancel:      cancelIndex,
		prompt:      prompt,
		secure:      secure,
		text:        text,
		placeholder: placeholder,
	}
	for i := range buttons {
		alert.destructive = append(alert.destructive, (destructiveMask>>i)&1 != 0)
	}
	return alert
}

// startTask runs the task dialog of an alert on a thread of its own.
func (a *nativeAlert) startTask() {
	a.running = true
	a.finished = make(chan struct{})
	go a.runTask()
}

func (alert *nativeAlert) beginModal(win *nativeWindow) {
	if alert == nil {
		return
	}
	alert.owner = 0
	if win != nil && win.hwnd != 0 && win32IsWindowVisible(win.hwnd) {
		alert.owner = win.hwnd
	}
	alert.done.Store(false)
	alert.response.Store(-1)
	if alert.prompt {
		alert.buildPrompt()
		if alert.owner != 0 {
			win32EnableWindow(alert.owner, false)
		}
		win32ShowWindow(alert.window, swShow)
		win32SetForegroundWindow(alert.window)
		win32SetFocus(alert.edit)
		win32SendMessage(alert.edit, emSetSel, 0, ^uintptr(0))
		return
	}
	alert.startTask()
}

// stepModal processes messages and returns the button of an alert that
// closed, or -1.
func (alert *nativeAlert) stepModal() int {
	if alert == nil {
		return 0
	}
	pumpOnce()
	if alert.done.Load() {
		return int(alert.response.Load())
	}
	return -1
}

func (alert *nativeAlert) endModal(abort bool) {
	if alert == nil {
		return
	}
	if alert.prompt && alert.edit != 0 {
		alert.answer = windowText(alert.edit)
	}
	if alert.running {
		if open := hwnd(alert.dialog.Load()); open != 0 {
			button := uintptr(firstButton)
			if alert.cancel >= 0 {
				button = idCancel
			}
			win32PostMessage(open, tdmClickButton, button, 0)
		}
		alert.waitTask()
	}
	if alert.owner != 0 && win32IsWindow(alert.owner) {
		win32EnableWindow(alert.owner, true)
	}
	if alert.window != 0 && win32IsWindow(alert.window) {
		win32ShowWindow(alert.window, swHide)
	}
	if alert.owner != 0 && win32IsWindow(alert.owner) {
		win32SetForegroundWindow(alert.owner)
	}
	alert.destroy()
}

func (alert *nativeAlert) promptText() string {
	if alert == nil {
		return ""
	}
	if alert.edit != 0 && win32IsWindow(alert.edit) {
		return windowText(alert.edit)
	}
	return alert.answer
}

// ---- About box ----
//
// The about box is a task dialog that does not block the application.

var abouts []*nativeAlert

func showAbout(name string, version string, copyright string, website string, credits string, icon *iconImage) {
	if initialize() != nil {
		return
	}
	// Finished about boxes go away.
	abouts = slices.DeleteFunc(abouts, func(about *nativeAlert) bool {
		if about.done.Load() {
			about.waitTask()
			about.destroy()
			return true
		}
		return false
	})
	about := &nativeAlert{title: name, buttons: []string{"OK"}, cancel: 0}
	if about.title == "" {
		about.title = programName()
	}
	var parts []string
	if version != "" {
		parts = append(parts, "Version "+version)
	}
	if credits != "" {
		parts = append(parts, credits)
	}
	if copyright != "" {
		parts = append(parts, copyright)
	}
	if website != "" {
		parts = append(parts, `<a href="`+website+`">`+website+`</a>`)
	}
	about.message = strings.Join(parts, "\n\n")
	// The about box has no owner, so the application keeps working while it
	// shows.
	about.startTask()
	abouts = append(abouts, about)
}
