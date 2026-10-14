package ui_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

type registryBackend struct {
	name string
}

func (*registryBackend) CreateWindow(ui.WindowOptions) (ui.Window, error) {
	return nil, nil
}

func TestBackendRegistration(t *testing.T) {
	ui.RegisterBackend("registry-one", func() ui.Backend {
		return &registryBackend{name: "one"}
	})
	selected, err := ui.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.(*registryBackend).name != "one" {
		t.Fatalf("only registered backend name = %q, want one", selected.(*registryBackend).name)
	}

	ui.RegisterBackend("registry-two", func() ui.Backend {
		return &registryBackend{name: "two"}
	})
	if _, err := ui.New(nil); err == nil || !strings.Contains(err.Error(), "multiple backends") {
		t.Fatalf("New(nil) error = %v, want multiple-backends error", err)
	}

	selected, err = ui.New(&ui.Config{Backend: "registry-one"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.(*registryBackend).name != "one" {
		t.Fatalf("selected backend name = %q, want one", selected.(*registryBackend).name)
	}

	ui.SetDefaultBackend("registry-two")
	selected, err = ui.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.(*registryBackend).name != "two" {
		t.Fatalf("default backend name = %q, want two", selected.(*registryBackend).name)
	}

	names := ui.RegisteredBackends()
	if !slices.Contains(names, "registry-one") || !slices.Contains(names, "registry-two") {
		t.Fatalf("RegisteredBackends() = %v, want registry-one and registry-two", names)
	}

	assertPanics(t, func() {
		ui.RegisterBackend("registry-one", func() ui.Backend {
			return &registryBackend{}
		})
	})
}

func TestNewRejectsUnknownBackend(t *testing.T) {
	_, err := ui.New(&ui.Config{Backend: "registry-missing"})
	if err == nil || !strings.Contains(err.Error(), `backend "registry-missing" is not registered`) {
		t.Fatalf("New() error = %v, want missing-backend error", err)
	}
}

func assertPanics(t *testing.T, action func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("action did not panic")
		}
	}()

	action()
}
