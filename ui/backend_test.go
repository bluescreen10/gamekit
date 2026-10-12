package ui_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

type windowStub struct {
	ui.Window
}

func TestThemeValidation(t *testing.T) {
	for _, theme := range []ui.Theme{ui.ThemeSystem, ui.ThemeLight, ui.ThemeDark} {
		if err := theme.Validate(); err != nil {
			t.Errorf("Theme(%d).Validate() = %v, want nil", theme, err)
		}
	}
	if err := ui.Theme(255).Validate(); err == nil {
		t.Error("Theme(255).Validate() = nil, want an error")
	}
}

func TestWindowKindValidation(t *testing.T) {
	for _, kind := range []ui.WindowKind{ui.WindowNormal, ui.WindowUtility} {
		if err := kind.Validate(); err != nil {
			t.Errorf("WindowKind(%d).Validate() = %v, want nil", kind, err)
		}
	}
	if err := ui.WindowKind(255).Validate(); err == nil {
		t.Error("WindowKind(255).Validate() = nil, want an error")
	}
}

func TestWindowOptionsValidation(t *testing.T) {
	valid := ui.WindowOptions{Size: ui.Size{Width: 640, Height: 480}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("WindowOptions.Validate() = %v, want nil", err)
	}
	utility := ui.WindowOptions{
		Size:  ui.Size{Width: 320, Height: 240},
		Kind:  ui.WindowUtility,
		Owner: &windowStub{},
	}
	if err := utility.Validate(); err != nil {
		t.Fatalf("utility WindowOptions.Validate() = %v, want nil", err)
	}

	tests := []struct {
		name    string
		options ui.WindowOptions
	}{
		{
			name:    "empty size",
			options: ui.WindowOptions{},
		},
		{
			name: "invalid kind",
			options: ui.WindowOptions{
				Size: ui.Size{Width: 640, Height: 480},
				Kind: ui.WindowKind(255),
			},
		},
		{
			name: "invalid theme",
			options: ui.WindowOptions{
				Size:  ui.Size{Width: 640, Height: 480},
				Theme: ui.Theme(255),
			},
		},
		{
			name: "utility without owner",
			options: ui.WindowOptions{
				Size: ui.Size{Width: 320, Height: 240},
				Kind: ui.WindowUtility,
			},
		},
		{
			name: "normal with owner",
			options: ui.WindowOptions{
				Size:  ui.Size{Width: 640, Height: 480},
				Owner: &windowStub{},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.options.Validate(); err == nil {
				t.Error("WindowOptions.Validate() = nil, want an error")
			}
		})
	}
}
