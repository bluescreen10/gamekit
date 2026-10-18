//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import "github.com/bluescreen10/gamekit/gpu"

// formatOf maps a gpu.Format to a VkFormat. Formats this backend cannot
// translate (see SupportedFormats) return VK_FORMAT_UNDEFINED.
func formatOf(f gpu.Format) vkFormat {
	switch f {
	case gpu.FormatR8Unorm:
		return vkFormatR8Unorm
	case gpu.FormatRG8Unorm:
		return vkFormatR8G8Unorm
	case gpu.FormatRGBA8Unorm:
		return vkFormatR8G8B8A8Unorm
	case gpu.FormatBGRA8Unorm:
		return vkFormatB8G8R8A8Unorm
	case gpu.FormatRGBA8Srgb:
		return vkFormatR8G8B8A8Srgb
	case gpu.FormatBGRA8Srgb:
		return vkFormatB8G8R8A8Srgb
	case gpu.FormatR8Snorm:
		return vkFormatR8Snorm
	case gpu.FormatRG8Snorm:
		return vkFormatR8G8Snorm
	case gpu.FormatRGBA8Snorm:
		return vkFormatR8G8B8A8Snorm
	case gpu.FormatR8Uint:
		return vkFormatR8Uint
	case gpu.FormatRG8Uint:
		return vkFormatR8G8Uint
	case gpu.FormatRGBA8Uint:
		return vkFormatR8G8B8A8Uint
	case gpu.FormatR8Sint:
		return vkFormatR8Sint
	case gpu.FormatRG8Sint:
		return vkFormatR8G8Sint
	case gpu.FormatRGBA8Sint:
		return vkFormatR8G8B8A8Sint
	case gpu.FormatR16Unorm:
		return vkFormatR16Unorm
	case gpu.FormatRG16Unorm:
		return vkFormatR16G16Unorm
	case gpu.FormatRGBA16Unorm:
		return vkFormatR16G16B16A16Unorm
	case gpu.FormatR16Snorm:
		return vkFormatR16Snorm
	case gpu.FormatRG16Snorm:
		return vkFormatR16G16Snorm
	case gpu.FormatRGBA16Snorm:
		return vkFormatR16G16B16A16Snorm
	case gpu.FormatR16Uint:
		return vkFormatR16Uint
	case gpu.FormatRG16Uint:
		return vkFormatR16G16Uint
	case gpu.FormatRGBA16Uint:
		return vkFormatR16G16B16A16Uint
	case gpu.FormatR16Sint:
		return vkFormatR16Sint
	case gpu.FormatRG16Sint:
		return vkFormatR16G16Sint
	case gpu.FormatRGBA16Sint:
		return vkFormatR16G16B16A16Sint
	case gpu.FormatR16F:
		return vkFormatR16Sfloat
	case gpu.FormatRG16F:
		return vkFormatR16G16Sfloat
	case gpu.FormatRGBA16F:
		return vkFormatR16G16B16A16Sfloat
	case gpu.FormatR32Uint:
		return vkFormatR32Uint
	case gpu.FormatRG32Uint:
		return vkFormatR32G32Uint
	case gpu.FormatRGBA32Uint:
		return vkFormatR32G32B32A32Uint
	case gpu.FormatR32Sint:
		return vkFormatR32Sint
	case gpu.FormatRG32Sint:
		return vkFormatR32G32Sint
	case gpu.FormatRGBA32Sint:
		return vkFormatR32G32B32A32Sint
	case gpu.FormatR32F:
		return vkFormatR32Sfloat
	case gpu.FormatRG32F:
		return vkFormatR32G32Sfloat
	case gpu.FormatRGBA32F:
		return vkFormatR32G32B32A32Sfloat
	case gpu.FormatRGB10A2Unorm:
		return vkFormatA2B10G10R10UnormPACK32
	case gpu.FormatRGB10A2Uint:
		return vkFormatA2B10G10R10UintPACK32
	case gpu.FormatRG11B10F:
		return vkFormatB10G11R11UfloatPACK32
	case gpu.FormatRGB9E5F:
		return vkFormatE5B9G9R9UfloatPACK32
	case gpu.FormatDepth16Unorm:
		return vkFormatD16Unorm
	case gpu.FormatDepth32F:
		return vkFormatD32Sfloat
	case gpu.FormatDepth24Stencil8:
		return vkFormatD24UnormS8Uint
	case gpu.FormatDepth32FStencil8:
		return vkFormatD32SfloatS8Uint
	case gpu.FormatStencil8:
		return vkFormatS8Uint
	case gpu.FormatBC1Unorm:
		return vkFormatBC1RgbaUnormBlock
	case gpu.FormatBC1Srgb:
		return vkFormatBC1RgbaSrgbBlock
	case gpu.FormatBC3Unorm:
		return vkFormatBC3UnormBlock
	case gpu.FormatBC3Srgb:
		return vkFormatBC3SrgbBlock
	case gpu.FormatBC4Unorm:
		return vkFormatBC4UnormBlock
	case gpu.FormatBC4Snorm:
		return vkFormatBC4SnormBlock
	case gpu.FormatBC5Unorm:
		return vkFormatBC5UnormBlock
	case gpu.FormatBC5Snorm:
		return vkFormatBC5SnormBlock
	case gpu.FormatBC6HFloat:
		return vkFormatBC6HSfloatBlock
	case gpu.FormatBC6HUFloat:
		return vkFormatBC6HUfloatBlock
	case gpu.FormatBC7Unorm:
		return vkFormatBC7UnormBlock
	case gpu.FormatBC7Srgb:
		return vkFormatBC7SrgbBlock
	case gpu.FormatASTC4x4Unorm:
		return vkFormatAstc4x4UnormBlock
	case gpu.FormatASTC4x4Srgb:
		return vkFormatAstc4x4SrgbBlock
	case gpu.FormatASTC5x5Unorm:
		return vkFormatAstc5x5UnormBlock
	case gpu.FormatASTC5x5Srgb:
		return vkFormatAstc5x5SrgbBlock
	case gpu.FormatASTC6x6Unorm:
		return vkFormatAstc6x6UnormBlock
	case gpu.FormatASTC6x6Srgb:
		return vkFormatAstc6x6SrgbBlock
	case gpu.FormatASTC8x8Unorm:
		return vkFormatAstc8x8UnormBlock
	case gpu.FormatASTC8x8Srgb:
		return vkFormatAstc8x8SrgbBlock
	case gpu.FormatASTC10x10Unorm:
		return vkFormatAstc10x10UnormBlock
	case gpu.FormatASTC10x10Srgb:
		return vkFormatAstc10x10SrgbBlock
	case gpu.FormatASTC12x12Unorm:
		return vkFormatAstc12x12UnormBlock
	case gpu.FormatASTC12x12Srgb:
		return vkFormatAstc12x12SrgbBlock
	default:
		return vkFormatUndefined
	}
}

// supportedFormats is the static set this backend accepts. The engine only ever
// runs Vulkan here through KosmicKrisp on macOS (translated onto Metal, not a
// native Vulkan driver), so support tracks the underlying Apple GPU rather than
// the full Vulkan spec: BC and ASTC compressed formats are available (Apple
// Silicon supports both natively), but ETC2/EAC are not (Metal never exposes
// them, on any platform).
var supportedFormats = []gpu.Format{
	gpu.FormatR8Unorm, gpu.FormatRG8Unorm, gpu.FormatRGBA8Unorm, gpu.FormatBGRA8Unorm,
	gpu.FormatRGBA8Srgb, gpu.FormatBGRA8Srgb,
	gpu.FormatR8Snorm, gpu.FormatRG8Snorm, gpu.FormatRGBA8Snorm,
	gpu.FormatR8Uint, gpu.FormatRG8Uint, gpu.FormatRGBA8Uint,
	gpu.FormatR8Sint, gpu.FormatRG8Sint, gpu.FormatRGBA8Sint,
	gpu.FormatR16Unorm, gpu.FormatRG16Unorm, gpu.FormatRGBA16Unorm,
	gpu.FormatR16Snorm, gpu.FormatRG16Snorm, gpu.FormatRGBA16Snorm,
	gpu.FormatR16Uint, gpu.FormatRG16Uint, gpu.FormatRGBA16Uint,
	gpu.FormatR16Sint, gpu.FormatRG16Sint, gpu.FormatRGBA16Sint,
	gpu.FormatR16F, gpu.FormatRG16F, gpu.FormatRGBA16F,
	gpu.FormatR32Uint, gpu.FormatRG32Uint, gpu.FormatRGBA32Uint,
	gpu.FormatR32Sint, gpu.FormatRG32Sint, gpu.FormatRGBA32Sint,
	gpu.FormatR32F, gpu.FormatRG32F, gpu.FormatRGBA32F,
	gpu.FormatRGB10A2Unorm, gpu.FormatRGB10A2Uint, gpu.FormatRG11B10F, gpu.FormatRGB9E5F,
	gpu.FormatDepth16Unorm, gpu.FormatDepth32F, gpu.FormatDepth24Stencil8, gpu.FormatDepth32FStencil8, gpu.FormatStencil8,
	gpu.FormatBC1Unorm, gpu.FormatBC1Srgb, gpu.FormatBC3Unorm, gpu.FormatBC3Srgb,
	gpu.FormatBC4Unorm, gpu.FormatBC4Snorm, gpu.FormatBC5Unorm, gpu.FormatBC5Snorm,
	gpu.FormatBC6HFloat, gpu.FormatBC6HUFloat, gpu.FormatBC7Unorm, gpu.FormatBC7Srgb,
	gpu.FormatASTC4x4Unorm, gpu.FormatASTC4x4Srgb,
	gpu.FormatASTC5x5Unorm, gpu.FormatASTC5x5Srgb,
	gpu.FormatASTC6x6Unorm, gpu.FormatASTC6x6Srgb,
	gpu.FormatASTC8x8Unorm, gpu.FormatASTC8x8Srgb,
	gpu.FormatASTC10x10Unorm, gpu.FormatASTC10x10Srgb,
	gpu.FormatASTC12x12Unorm, gpu.FormatASTC12x12Srgb,
}

// SupportedFormats reports the Format values this backend can create textures
// with. See supportedFormats for why the set is narrower than the full Vulkan spec.
func (b *Backend) SupportedFormats() []gpu.Format {
	return supportedFormats
}
