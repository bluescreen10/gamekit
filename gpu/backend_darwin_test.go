//go:build darwin && cgo

package gpu_test

// Register Metal, and Vulkan too when built with -tags gamekit_vulkan;
// GAMEKIT_GPU_BACKEND selects the conformance backend.
import (
	_ "github.com/bluescreen10/gamekit/gpu/metal"
	_ "github.com/bluescreen10/gamekit/gpu/vulkan"
)
