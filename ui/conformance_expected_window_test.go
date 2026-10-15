package ui_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func checkExpectedWindow(r *recorder, backend ui.Backend, window ui.Window) {
	checkExpectedWindowState(r, window)
	checkExpectedWindowEvents(r, window)
	checkExpectedSystemPreferences(r, window)
	checkExpectedFullscreen(r, backend)
}

func checkExpectedWindowState(r *recorder, window ui.Window) {
	r.check("window/complete/size_constraints", func() error {
		minimum := ui.Size{Width: 320, Height: 200}
		maximum := ui.Size{Width: 1920, Height: 1080}
		if err := window.SetMinimumSize(minimum); err != nil {
			return err
		}
		if err := window.SetMaximumSize(maximum); err != nil {
			return err
		}
		if window.MinimumSize() != minimum || window.MaximumSize() != maximum {
			return fmt.Errorf("size constraints = (%#v, %#v), want (%#v, %#v)", window.MinimumSize(), window.MaximumSize(), minimum, maximum)
		}
		return nil
	})

	r.check("window/complete/runtime_flags", func() error {
		window.SetResizable(false)
		window.SetBorderless(true)
		window.SetAlwaysOnTop(true)
		if window.IsResizable() || !window.IsBorderless() || !window.IsAlwaysOnTop() {
			return errors.New("runtime window flags did not preserve their values")
		}
		window.SetResizable(true)
		window.SetBorderless(false)
		window.SetAlwaysOnTop(false)
		return nil
	})

	r.check("window/complete/minimize_maximize_restore", func() error {
		if err := window.Minimize(); err != nil {
			return err
		}
		window.PollEvents()
		if !window.IsMinimized() {
			return errors.New("window did not enter the minimized state")
		}
		if err := window.Maximize(); err != nil {
			return err
		}
		window.PollEvents()
		if !window.IsMaximized() {
			return errors.New("window did not enter the maximized state")
		}
		if err := window.Restore(); err != nil {
			return err
		}
		window.PollEvents()
		if window.IsMinimized() || window.IsMaximized() || window.IsFullscreen() {
			return errors.New("Restore did not restore the normal state")
		}
		return nil
	})

	r.check("window/complete/screen", func() error {
		screen := window.Screen()
		if !screen.Bounds.Size.IsValid() || !screen.WorkArea.Size.IsValid() {
			return fmt.Errorf("invalid screen bounds: %#v", screen)
		}
		if screen.ScaleFactor <= 0 || screen.DPI <= 0 || screen.RefreshRate <= 0 {
			return fmt.Errorf("invalid screen metrics: %#v", screen)
		}
		return nil
	})
}

func checkExpectedWindowEvents(r *recorder, window ui.Window) {
	r.check("window/complete/event_subscriptions", func() error {
		unsubscribes := []ui.Unsubscribe{
			window.OnResize(func(ui.Size) {
			}),
			window.OnCloseRequest(func() bool {
				return false
			}),
			window.OnFocus(func() {
			}),
			window.OnBlur(func() {
			}),
			window.OnThemeChange(func(ui.Theme) {
			}),
			window.OnFileDrop(func([]string) {
			}),
		}
		for _, unsubscribe := range unsubscribes {
			if unsubscribe == nil {
				return errors.New("window event subscription returned nil")
			}
			unsubscribe()
			unsubscribe()
		}
		return nil
	})
}

func checkExpectedSystemPreferences(r *recorder, window ui.Window) {
	r.check("theme/system_preferences", func() error {
		preferences := window.SystemPreferences()
		if preferences.ResolvedTheme != ui.ThemeLight && preferences.ResolvedTheme != ui.ThemeDark {
			return fmt.Errorf("resolved theme = %v", preferences.ResolvedTheme)
		}
		if preferences.TextScale <= 0 {
			return fmt.Errorf("text scale = %v", preferences.TextScale)
		}
		_ = preferences.HighContrast
		_ = preferences.ReducedMotion
		_ = preferences.ReducedTransparency
		_ = preferences.AccentColor
		_ = preferences.Locale
		_ = preferences.RightToLeft
		return nil
	})

	r.check("theme/system_preferences_event", func() error {
		unsubscribe := window.OnSystemPreferencesChange(func(ui.SystemPreferences) {
		})
		if unsubscribe == nil {
			return errors.New("system-preferences subscription returned nil")
		}
		unsubscribe()
		unsubscribe()
		return nil
	})
}

// fullscreenTimeout bounds animated fullscreen transitions.
const fullscreenTimeout = 5 * time.Second

// waitFor polls window events until done reports true or timeout passes.
func waitFor(window ui.Window, timeout time.Duration, done func() bool) bool {
	deadline := time.Now().Add(timeout)
	for !done() {
		if time.Now().After(deadline) {
			return false
		}
		window.PollEvents()
		time.Sleep(10 * time.Millisecond)
	}
	return true
}

func checkExpectedFullscreen(r *recorder, backend ui.Backend) {
	size := ui.Size{Width: 640, Height: 480}
	var window ui.Window
	r.check("window/fullscreen/create", func() error {
		var err error
		window, err = backend.CreateWindow(ui.WindowOptions{
			Title:      "Fullscreen conformance",
			Size:       size,
			Hidden:     true,
			Resizable:  true,
			Fullscreen: true,
		})
		return err
	})
	if window == nil {
		return
	}
	defer window.Close()

	var changes []bool
	unsubscribe := window.OnFullscreenChange(func(fullscreen bool) {
		changes = append(changes, fullscreen)
	})
	defer unsubscribe()
	lastChange := func(want bool) func() bool {
		return func() bool {
			return len(changes) > 0 && changes[len(changes)-1] == want
		}
	}
	// enter requests fullscreen and waits for it. A platform that cannot
	// enter fullscreen now, such as while the display is locked, skips.
	enter := func() error {
		if err := window.SetFullscreen(true); err != nil {
			if errors.Is(err, ui.ErrUnavailable) {
				return skipCheck("fullscreen is unavailable: %v", err)
			}
			return err
		}
		if !waitFor(window, fullscreenTimeout, window.IsFullscreen) {
			return errors.New("window did not enter fullscreen")
		}
		return nil
	}
	requireFullscreen := func() error {
		if !window.IsFullscreen() {
			return skipCheck("the window could not enter fullscreen in an earlier check")
		}
		return nil
	}

	r.check("window/fullscreen/subscription", func() error {
		extra := window.OnFullscreenChange(func(bool) {
		})
		if extra == nil {
			return errors.New("OnFullscreenChange returned nil")
		}
		extra()
		extra()
		return nil
	})

	r.check("window/fullscreen/startup_when_shown", func() error {
		window.SetVisible(true)
		if !waitFor(window, fullscreenTimeout, window.IsFullscreen) {
			// Distinguish an environment that cannot show fullscreen from
			// a backend that ignored the creation option.
			if err := window.SetFullscreen(true); errors.Is(err, ui.ErrUnavailable) {
				return skipCheck("fullscreen is unavailable: %v", err)
			}
			return errors.New("a window created with Fullscreen did not enter fullscreen when shown")
		}
		if !waitFor(window, fullscreenTimeout, lastChange(true)) {
			return fmt.Errorf("OnFullscreenChange reported %v, want a final true", changes)
		}
		return nil
	})

	r.check("window/fullscreen/framebuffer", func() error {
		if err := requireFullscreen(); err != nil {
			return err
		}
		screen := window.Screen()
		framebuffer := window.FramebufferSize()
		want := int(float64(screen.Bounds.Size.Width) * screen.ScaleFactor)
		if framebuffer.Width != want {
			return fmt.Errorf("fullscreen framebuffer width = %d, want the screen's %d", framebuffer.Width, want)
		}
		return nil
	})

	r.check("window/fullscreen/exit_restores_size", func() error {
		if err := requireFullscreen(); err != nil {
			return err
		}
		if err := window.SetFullscreen(false); err != nil {
			return err
		}
		if !waitFor(window, fullscreenTimeout, func() bool {
			return !window.IsFullscreen()
		}) {
			return errors.New("window remained fullscreen")
		}
		if !waitFor(window, fullscreenTimeout, lastChange(false)) {
			return fmt.Errorf("OnFullscreenChange reported %v, want a final false", changes)
		}
		return wantEqual(window.Size(), size)
	})

	r.check("window/fullscreen/runtime", func() error {
		if err := enter(); err != nil {
			return err
		}
		if !window.FramebufferSize().IsValid() {
			return errors.New("fullscreen window has an invalid framebuffer size")
		}
		return nil
	})

	r.check("window/fullscreen/restore", func() error {
		if err := requireFullscreen(); err != nil {
			return err
		}
		if err := window.Restore(); err != nil {
			return err
		}
		if !waitFor(window, fullscreenTimeout, func() bool {
			return !window.IsFullscreen()
		}) {
			return errors.New("Restore did not leave fullscreen")
		}
		return nil
	})
}
