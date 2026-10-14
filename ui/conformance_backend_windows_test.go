//go:build windows && cgo

package ui_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/bluescreen10/gamekit/ui/winui"
)

var backendReport conformanceReport

func TestMain(m *testing.M) {
	runtime.LockOSThread()
	backendReport = checkBackend(&winui.Backend{})
	code := m.Run()
	runtime.UnlockOSThread()
	os.Exit(code)
}

func TestCurrentBackendConformance(t *testing.T) {
	t.Log(backendReport.summary())
	backendReport.assert(t)
}
