//go:build darwin && cgo

package ui_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/cocoa"
)

// conformanceBundleID identifies the temporary application bundle that runs
// the notification checks. macOS gives notifications only to a process with
// a bundle identifier.
const conformanceBundleID = "dev.gamekit.ui.conformance"

// bundleReportVariable names the file where the bundled helper writes its
// results. The test binary runs as the helper when the variable is set.
const bundleReportVariable = "GAMEKIT_UI_BUNDLE_REPORT"

// bundleHelperTimeout limits the time for the helper launch and its checks.
const bundleHelperTimeout = 2 * time.Minute

// runBundleHelper runs the notification checks when the process is the
// bundled helper, and writes the results to the report file. It reports
// whether the process is the helper.
func runBundleHelper() (code int, isHelper bool) {
	path := os.Getenv(bundleReportVariable)
	if path == "" {
		return 0, false
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var r recorder
	window, err := (&cocoa.Backend{}).CreateWindow(ui.WindowOptions{
		Title:  "GameKit notification conformance",
		Size:   ui.Size{Width: 320, Height: 200},
		Hidden: true,
	})
	if err != nil {
		r.check("notifications/bundle/window", func() error {
			return err
		})
	} else {
		checkExpectedNotifications(&r, window)
		window.Close()
	}
	data, err := json.Marshal(r.results)
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "notification helper:", err)
		return 1, true
	}
	return 0, true
}

// checkPlatformNotifications runs the notification checks in a temporary
// application bundle that LaunchServices launches. The other checks do not
// need a bundle, so a launch problem affects only these results.
func checkPlatformNotifications(r *recorder, window ui.Window) {
	results, err := runNotificationsInBundle()
	if err != nil {
		r.check("notifications/bundle", func() error {
			return err
		})
		return
	}
	r.results = append(r.results, results...)
}

func runNotificationsInBundle() ([]conformanceResult, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "gamekit-ui-conformance-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	bundle, err := writeConformanceBundle(dir, executable)
	if err != nil {
		return nil, err
	}
	report := filepath.Join(dir, "report.json")
	output := filepath.Join(dir, "output.txt")
	ctx, cancel := context.WithTimeout(context.Background(), bundleHelperTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "open", "-W", "-n", "-g",
		"--env", bundleReportVariable+"="+report,
		"--stdout", output, "--stderr", output,
		bundle)
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("launch the notification helper: %w%s", err, helperOutput(output))
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return nil, fmt.Errorf("the notification helper wrote no report%s", helperOutput(output))
	}
	var results []conformanceResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("read the notification helper report: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("the notification helper reported no checks")
	}
	return results, nil
}

// helperOutput returns the helper's output for an error message.
func helperOutput(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	return ":\n" + string(data)
}

// writeConformanceBundle writes an application bundle around executable and
// returns the bundle path.
func writeConformanceBundle(dir, executable string) (string, error) {
	bundle := filepath.Join(dir, "GameKitConformance.app")
	contents := filepath.Join(bundle, "Contents")
	macOS := filepath.Join(contents, "MacOS")
	if err := os.MkdirAll(macOS, 0o755); err != nil {
		return "", err
	}
	name := "GameKitConformance"
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key><string>` + conformanceBundleID + `</string>
	<key>CFBundleName</key><string>GameKit Conformance</string>
	<key>CFBundleExecutable</key><string>` + name + `</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>NSPrincipalClass</key><string>NSApplication</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
		return "", err
	}
	if err := copyExecutable(executable, filepath.Join(macOS, name)); err != nil {
		return "", err
	}
	return bundle, nil
}

func copyExecutable(source, target string) error {
	if err := os.Link(source, target); err == nil {
		return nil
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
