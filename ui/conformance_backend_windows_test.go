//go:build windows && cgo

package ui_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/bluescreen10/gamekit/ui/win32"
)

var backendReport conformanceReport

func TestMain(m *testing.M) {
	runtime.LockOSThread()
	backendReport = checkBackend(&win32.Backend{})
	code := m.Run()
	runtime.UnlockOSThread()
	os.Exit(code)
}

func TestCurrentBackendConformance(t *testing.T) {
	t.Log(backendReport.summary())
	backendReport.assert(t)
}
