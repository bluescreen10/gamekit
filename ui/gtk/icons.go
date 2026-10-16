//go:build linux && cgo

package gtk

/*
#include "gtk_bridge.h"
*/
import "C"

import (
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// symbolNames maps backend-neutral icon names to freedesktop icon names where
// they differ. A name can have alternatives, because icon themes do not all
// have the same icons. The current Adwaita theme, for example, has no
// "emblem-ok" icon, and only a full-color "emblem-favorite" icon.
var symbolNames = map[string][]string{
	"game-controller": {"input-gaming"},
	"gear":            {"emblem-system"},
	"settings":        {"emblem-system"},
	"search":          {"system-search"},
	"close":           {"window-close"},
	"add":             {"list-add"},
	"remove":          {"list-remove"},
	"delete":          {"user-trash"},
	"edit":            {"document-edit"},
	"info":            {"dialog-information"},
	"warning":         {"dialog-warning"},
	"error":           {"dialog-error"},
	"success":         {"emblem-ok", "object-select", "emblem-default"},
	"help":            {"help-about"},
	"home":            {"go-home"},
	"user":            {"avatar-default"},
	"refresh":         {"view-refresh"},
	"download":        {"folder-download"},
	"upload":          {"document-send"},
	"share":           {"send-to"},
	"open":            {"document-open"},
	"save":            {"document-save"},
	"copy":            {"edit-copy"},
	"paste":           {"edit-paste"},
	"cut":             {"edit-cut"},
	"play":            {"media-playback-start"},
	"pause":           {"media-playback-pause"},
	"stop":            {"media-playback-stop"},
	"menu":            {"open-menu"},
	"more":            {"view-more"},
	"visible":         {"view-reveal"},
	"hidden":          {"view-conceal"},
	"lock":            {"changes-prevent"},
	"unlock":          {"changes-allow"},
	"notification":    {"preferences-system-notifications"},
	"calendar":        {"x-office-calendar"},
	"clock":           {"preferences-system-time"},
	"image":           {"image-x-generic"},
	"document":        {"text-x-generic"},
	"arrow-down":      {"go-down"},
	"arrow-up":        {"go-up"},
	"arrow-left":      {"go-previous"},
	"arrow-right":     {"go-next"},
	"folder":          {"folder"},
	"star":            {"starred"},
	"heart":           {"emblem-favorite"},
}

// symbolCandidates returns the icon names to try for name, in order. Each name
// comes first with the "-symbolic" suffix, so "go-down" becomes
// "go-down-symbolic", and then without it.
func symbolCandidates(name string) []string {
	names, ok := symbolNames[name]
	if !ok {
		names = []string{name}
	}
	candidates := make([]string, 0, 2*len(names))
	for _, name := range names {
		base := strings.TrimSuffix(name, "-symbolic")
		candidates = append(candidates, base+"-symbolic", base)
	}
	return candidates
}

// nativeIcon returns a referenced GdkPaintable for source, or nil. pointSize
// scales symbols and images; zero keeps their natural size.
func nativeIcon(source ui.IconSource, pointSize float64) unsafe.Pointer {
	if source.IsZero() {
		return nil
	}
	symbol := cString("")
	if source.SystemName != "" {
		freeCString(symbol)
		symbol = cString(strings.Join(symbolCandidates(source.SystemName), "\n"))
	}
	defer freeCString(symbol)
	var pixels []byte
	var width, height int
	if source.Image != nil && !source.Image.Bounds().Empty() {
		pixels, width, height = rgbaPixels(source.Image)
	}
	return C.gkGTKIcon(symbol, (*C.uchar)(unsafe.Pointer(bytesPointer(pixels))),
		C.int(width), C.int(height), C.double(pointSize))
}

// acceleratorKeys maps shortcut key names to GDK key names.
var acceleratorKeys = map[string]string{
	"Up": "Up", "Down": "Down", "Left": "Left", "Right": "Right",
	"Delete": "Delete", "Home": "Home", "End": "End", "PageUp": "Page_Up", "PageDown": "Page_Down",
	"Backspace": "BackSpace", "Return": "Return", "Enter": "KP_Enter", "Escape": "Escape", "Tab": "Tab",
	"Space": "space",
}

// keyEquivalent converts a shortcut to a GTK accelerator string, such as
// "<Control><Shift>k". The second result is always zero; the accelerator
// carries the modifiers.
func keyEquivalent(shortcut ui.Shortcut) (string, int) {
	if shortcut.IsZero() {
		return "", 0
	}
	var accelerator strings.Builder
	if shortcut.Command || shortcut.Control {
		accelerator.WriteString("<Control>")
	}
	if shortcut.Alt {
		accelerator.WriteString("<Alt>")
	}
	if shortcut.Shift {
		accelerator.WriteString("<Shift>")
	}
	if name, ok := acceleratorKeys[shortcut.Key]; ok {
		accelerator.WriteString(name)
		return accelerator.String(), 0
	}
	if len(shortcut.Key) >= 2 && shortcut.Key[0] == 'F' && isNumber(shortcut.Key[1:]) {
		accelerator.WriteString(shortcut.Key)
		return accelerator.String(), 0
	}
	r, _ := utf8.DecodeRuneInString(shortcut.Key)
	accelerator.WriteRune(unicode.ToLower(r))
	return accelerator.String(), 0
}

func isNumber(text string) bool {
	if text == "" {
		return false
	}
	for _, digit := range text {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

var shortcutLabels = map[string]string{
	"Up": "↑", "Down": "↓", "Left": "←", "Right": "→", "Delete": "Delete", "Backspace": "Backspace",
	"Return": "Return", "Enter": "Enter", "Escape": "Esc", "Tab": "Tab", "Space": "Space",
	"Home": "Home", "End": "End", "PageUp": "Page Up", "PageDown": "Page Down",
}

// shortcutText formats a shortcut as GTK shows it, such as "Ctrl+Shift+K".
func shortcutText(shortcut ui.Shortcut) string {
	if shortcut.IsZero() {
		return ""
	}
	var text strings.Builder
	if shortcut.Command || shortcut.Control {
		text.WriteString("Ctrl+")
	}
	if shortcut.Alt {
		text.WriteString("Alt+")
	}
	if shortcut.Shift {
		text.WriteString("Shift+")
	}
	if label, ok := shortcutLabels[shortcut.Key]; ok {
		text.WriteString(label)
	} else {
		text.WriteString(strings.ToUpper(shortcut.Key))
	}
	return text.String()
}
