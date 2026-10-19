//go:build cgo && (linux || windows || gamekit_vulkan)

package defaultgpu

import _ "github.com/bluescreen10/gamekit/gpu/vulkan"
