//go:build windows && cgo

package win32

/*
#include "win32_bridge.h"
*/
import "C"

import (
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// symbolGlyphs maps backend-neutral icon names to glyphs of the Segoe Fluent
// Icons font, which Windows uses for its own symbols. Segoe MDL2 Assets has the
// same code points on Windows 10.
var symbolGlyphs = map[string]rune{
	"game-controller": 0xE7FC,
	"gear":            0xE713,
	"settings":        0xE713,
	"search":          0xE721,
	"close":           0xE711,
	"add":             0xE710,
	"remove":          0xE738,
	"delete":          0xE74D,
	"trash":           0xE74D,
	"edit":            0xE70F,
	"info":            0xE946,
	"warning":         0xE7BA,
	"error":           0xE783,
	"success":         0xE73E,
	"help":            0xE897,
	"home":            0xE80F,
	"user":            0xE77B,
	"refresh":         0xE72C,
	"download":        0xE896,
	"upload":          0xE898,
	"share":           0xE72D,
	"link":            0xE71B,
	"open":            0xE8E5,
	"save":            0xE74E,
	"copy":            0xE8C8,
	"paste":           0xE77F,
	"cut":             0xE8C6,
	"play":            0xE768,
	"pause":           0xE769,
	"stop":            0xE71A,
	"menu":            0xE700,
	"more":            0xE712,
	"visible":         0xE7B3,
	"hidden":          0xED1A,
	"lock":            0xE72E,
	"unlock":          0xE785,
	"notification":    0xEA8F,
	"calendar":        0xE787,
	"clock":           0xE823,
	"image":           0xE91B,
	"document":        0xE8A5,
	"arrow-down":      0xE74B,
	"arrow-up":        0xE74A,
	"arrow-left":      0xE72B,
	"arrow-right":     0xE72A,
	"folder":          0xE8B7,
	"star":            0xE734,
	"heart":           0xEB51,
}

// nativeIcon returns a referenced icon for source, or nil when the source is
// empty or names a symbol that has no glyph. pointSize scales symbols and
// images; zero keeps their natural size.
func nativeIcon(source ui.IconSource, pointSize float64) unsafe.Pointer {
	if source.IsZero() {
		return nil
	}
	symbol := cString("")
	if glyph, ok := symbolGlyphs[source.SystemName]; ok {
		freeCString(symbol)
		symbol = cString(string(glyph))
	}
	defer freeCString(symbol)
	var pixels []byte
	var width, height int
	if source.Image != nil && !source.Image.Bounds().Empty() {
		pixels, width, height = rgbaPixels(source.Image)
	}
	return C.gkWin32Icon(symbol, (*C.uchar)(unsafe.Pointer(bytesPointer(pixels))),
		C.int(width), C.int(height), C.double(pointSize))
}

// Modifier bits of a keyboard accelerator. They match the
// Windows.System.VirtualKeyModifiers values.
const (
	acceleratorControl = 1
	acceleratorAlt     = 2
	acceleratorShift   = 4
)

// keyEquivalent converts a shortcut to the name of a Windows virtual key,
// such as "Up", "F5", or "K", and a set of modifier bits.
func keyEquivalent(shortcut ui.Shortcut) (string, int) {
	if shortcut.IsZero() {
		return "", 0
	}
	modifiers := 0
	if shortcut.Command || shortcut.Control {
		modifiers |= acceleratorControl
	}
	if shortcut.Alt {
		modifiers |= acceleratorAlt
	}
	if shortcut.Shift {
		modifiers |= acceleratorShift
	}
	if len(shortcut.Key) >= 2 && shortcut.Key[0] == 'F' && isNumber(shortcut.Key[1:]) {
		return shortcut.Key, modifiers
	}
	if utf8.RuneCountInString(shortcut.Key) == 1 {
		r, _ := utf8.DecodeRuneInString(shortcut.Key)
		return string(unicode.ToUpper(r)), modifiers
	}
	return shortcut.Key, modifiers
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
	"Return": "Enter", "Enter": "Enter", "Escape": "Esc", "Tab": "Tab", "Space": "Space",
	"Home": "Home", "End": "End", "PageUp": "Page Up", "PageDown": "Page Down",
}

// shortcutText formats a shortcut as Windows shows it, such as
// "Ctrl+Shift+K".
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
