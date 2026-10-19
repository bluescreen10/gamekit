package gpu_test

import (
	"fmt"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
)

func TestFormatAspects(t *testing.T) {
	for format := gpu.FormatUndefined; format <= gpu.FormatASTC12x12Srgb; format++ {
		t.Run(fmt.Sprintf("format=%d", format), func(t *testing.T) {
			depth, stencil := false, false
			switch format {
			case gpu.FormatDepth16Unorm, gpu.FormatDepth32F:
				depth = true
			case gpu.FormatDepth24Stencil8, gpu.FormatDepth32FStencil8:
				depth, stencil = true, true
			case gpu.FormatStencil8:
				stencil = true
			}

			if format.IsDepth() != depth || format.IsStencil() != stencil {
				t.Fatalf("IsDepth, IsStencil = %v, %v; want %v, %v", format.IsDepth(), format.IsStencil(), depth, stencil)
			}
		})
	}
}

func TestZeroResourcesAreInvalid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"buffer", (gpu.Buffer{}).IsValid()},
		{"texture", (gpu.Texture{}).IsValid()},
		{"sampler", (gpu.Sampler{}).IsValid()},
		{"pipeline", (gpu.Pipeline{}).IsValid()},
		{"swapchain", (gpu.Swapchain{}).IsValid()},
		{"surface", gpu.Surface(0).IsValid()},
		{"query-pool", (gpu.QueryPool{}).IsValid()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.valid {
				t.Error("zero resource is valid")
			}
		})
	}
}

type surfaceTarget struct {
	native gpu.NativeSurface
}

func (s surfaceTarget) NativeSurface() gpu.NativeSurface {
	return s.native
}

func TestCreateSurfaceRejectsInvalidTargets(t *testing.T) {
	b := testBackend(t)
	for _, tc := range []struct {
		name   string
		target gpu.SurfaceTarget
	}{
		{"nil", nil},
		{"unknown", surfaceTarget{}},
		{"invalid-kind", surfaceTarget{native: gpu.NativeSurface{Kind: gpu.NativeSurfaceKind(255)}}},
		{"closed-cocoa-window", surfaceTarget{native: gpu.NativeSurface{Kind: gpu.NativeSurfaceCocoaWindow}}},
		{"closed-cocoa-view", surfaceTarget{native: gpu.NativeSurface{Kind: gpu.NativeSurfaceCocoaView}}},
		{"closed-win32-window", surfaceTarget{native: gpu.NativeSurface{Kind: gpu.NativeSurfaceWin32Window}}},
		{"closed-xlib-window", surfaceTarget{native: gpu.NativeSurface{Kind: gpu.NativeSurfaceXlibWindow}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			surface, err := b.CreateSurface(tc.target)
			if err == nil || surface.IsValid() {
				t.Fatalf("CreateSurface = %v, %v; want an invalid surface and an error", surface, err)
			}
		})
	}
}
