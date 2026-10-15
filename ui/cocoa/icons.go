//go:build darwin && cgo

package cocoa

/*
#include "cocoa_bridge.h"
*/
import "C"

import (
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// symbolNames maps backend-neutral icon names to SF Symbols where they differ.
// Other names use their dashes as dots, so "arrow-down" becomes "arrow.down".
var symbolNames = map[string]string{
	"game-controller": "gamecontroller",
	"gear":            "gearshape",
	"settings":        "gearshape",
	"search":          "magnifyingglass",
	"close":           "xmark",
	"add":             "plus",
	"remove":          "minus",
	"delete":          "trash",
	"edit":            "pencil",
	"info":            "info.circle",
	"warning":         "exclamationmark.triangle",
	"error":           "xmark.octagon",
	"success":         "checkmark.circle",
	"help":            "questionmark.circle",
	"home":            "house",
	"user":            "person",
	"refresh":         "arrow.clockwise",
	"download":        "arrow.down.circle",
	"upload":          "arrow.up.circle",
	"share":           "square.and.arrow.up",
	"open":            "folder",
	"save":            "square.and.arrow.down",
	"copy":            "doc.on.doc",
	"paste":           "doc.on.clipboard",
	"cut":             "scissors",
	"play":            "play.fill",
	"pause":           "pause.fill",
	"stop":            "stop.fill",
	"menu":            "line.3.horizontal",
	"more":            "ellipsis",
	"visible":         "eye",
	"hidden":          "eye.slash",
	"lock":            "lock",
	"unlock":          "lock.open",
	"notification":    "bell",
	"calendar":        "calendar",
	"clock":           "clock",
	"image":           "photo",
	"document":        "doc",
}

func symbolName(name string) string {
	if mapped, ok := symbolNames[name]; ok {
		return mapped
	}
	return strings.ReplaceAll(name, "-", ".")
}

// nativeIcon returns a retained NSImage for source, or nil. pointSize scales
// symbols and images; zero keeps their natural size.
func nativeIcon(source ui.IconSource, pointSize float64) unsafe.Pointer {
	if source.IsZero() {
		return nil
	}
	symbol := cString("")
	if source.SystemName != "" {
		freeCString(symbol)
		symbol = cString(symbolName(source.SystemName))
	}
	defer freeCString(symbol)
	var pixels []byte
	var width, height int
	if source.Image != nil && !source.Image.Bounds().Empty() {
		pixels, width, height = rgbaPixels(source.Image)
	}
	return C.gkCocoaIcon(symbol, (*C.uchar)(unsafe.Pointer(bytesPointer(pixels))),
		C.int(width), C.int(height), C.double(pointSize))
}

// AppKit modifier flags.
const (
	modifierShift   = 1 << 17
	modifierControl = 1 << 18
	modifierOption  = 1 << 19
	modifierCommand = 1 << 20
)

var functionKeys = map[string]rune{
	"Up": 0xF700, "Down": 0xF701, "Left": 0xF702, "Right": 0xF703,
	"Delete": 0xF728, "Home": 0xF729, "End": 0xF72B, "PageUp": 0xF72C, "PageDown": 0xF72D,
	"Backspace": 0x08, "Return": '\r', "Enter": '\r', "Escape": 0x1B, "Tab": '\t', "Space": ' ',
}

// keyEquivalent converts a shortcut to an AppKit key equivalent and modifier
// mask.
func keyEquivalent(shortcut ui.Shortcut) (string, int) {
	if shortcut.IsZero() {
		return "", 0
	}
	modifiers := 0
	if shortcut.Command {
		modifiers |= modifierCommand
	}
	if shortcut.Control {
		modifiers |= modifierControl
	}
	if shortcut.Alt {
		modifiers |= modifierOption
	}
	if shortcut.Shift {
		modifiers |= modifierShift
	}
	if r, ok := functionKeys[shortcut.Key]; ok {
		return string(r), modifiers
	}
	if len(shortcut.Key) >= 2 && shortcut.Key[0] == 'F' {
		number := 0
		for _, digit := range shortcut.Key[1:] {
			if digit < '0' || digit > '9' {
				number = 0
				break
			}
			number = number*10 + int(digit-'0')
		}
		if number >= 1 && number <= 35 {
			return string(rune(0xF704 + number - 1)), modifiers
		}
	}
	r, _ := utf8.DecodeRuneInString(shortcut.Key)
	return string(unicode.ToLower(r)), modifiers
}

var shortcutSymbols = map[string]string{
	"Up": "↑", "Down": "↓", "Left": "←", "Right": "→", "Delete": "⌦", "Backspace": "⌫",
	"Return": "↩", "Enter": "⌅", "Escape": "⎋", "Tab": "⇥", "Space": "Space",
	"Home": "↖", "End": "↘", "PageUp": "⇞", "PageDown": "⇟",
}

// shortcutText formats a shortcut with macOS modifier symbols.
func shortcutText(shortcut ui.Shortcut) string {
	if shortcut.IsZero() {
		return ""
	}
	var text strings.Builder
	if shortcut.Control {
		text.WriteString("⌃")
	}
	if shortcut.Alt {
		text.WriteString("⌥")
	}
	if shortcut.Shift {
		text.WriteString("⇧")
	}
	if shortcut.Command {
		text.WriteString("⌘")
	}
	if symbol, ok := shortcutSymbols[shortcut.Key]; ok {
		text.WriteString(symbol)
	} else {
		text.WriteString(strings.ToUpper(shortcut.Key))
	}
	return text.String()
}
