//go:build !darwin || !cgo

package ui_test

import "github.com/bluescreen10/gamekit/ui"

// checkPlatformNotifications runs the notification checks in the test
// process. Only macOS needs an application bundle for them.
func checkPlatformNotifications(r *recorder, window ui.Window) {
	checkExpectedNotifications(r, window)
}
