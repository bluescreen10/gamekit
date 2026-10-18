//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

/*
#cgo darwin  CFLAGS:  -I/usr/local/include
#cgo darwin  LDFLAGS: -L/usr/local/lib -lvulkan -Wl,-rpath,/usr/local/lib
#cgo linux   LDFLAGS: -lvulkan
#cgo windows LDFLAGS: -lvulkan-1

// The per-frame entry points that take pointers are marked noescape and nocallback, so
// the structs they are passed can stay on the Go stack: Vulkan keeps no pointer it is
// given once the call returns, and with no allocation or debug callbacks installed it
// never calls back into Go (which could move the stack). The directives must go if a Go
// callback (a VK_EXT_debug_utils messenger, say) is ever installed. Entry points taking
// no pointers, like the draws, are left alone: nocallback costs a check per call.
#cgo noescape vkAllocateCommandBuffers
#cgo nocallback vkAllocateCommandBuffers
#cgo noescape vkBeginCommandBuffer
#cgo nocallback vkBeginCommandBuffer
#cgo noescape vkCmdBindDescriptorSets
#cgo nocallback vkCmdBindDescriptorSets
#cgo noescape vkCmdBeginRendering
#cgo nocallback vkCmdBeginRendering
#cgo noescape vkCmdPipelineBarrier2
#cgo nocallback vkCmdPipelineBarrier2
#cgo noescape vkCmdSetViewport
#cgo nocallback vkCmdSetViewport
#cgo noescape vkCmdSetScissor
#cgo nocallback vkCmdSetScissor
#cgo noescape vkCmdPushConstants
#cgo nocallback vkCmdPushConstants
#cgo noescape vkCmdCopyBuffer
#cgo nocallback vkCmdCopyBuffer
#cgo noescape vkCmdCopyBufferToImage
#cgo nocallback vkCmdCopyBufferToImage
#cgo noescape vkCmdCopyImageToBuffer
#cgo nocallback vkCmdCopyImageToBuffer
#cgo noescape vkCmdCopyImage
#cgo nocallback vkCmdCopyImage
#cgo noescape vkCreateFence
#cgo nocallback vkCreateFence
#cgo noescape vkQueueSubmit2
#cgo nocallback vkQueueSubmit2
#cgo noescape vkAcquireNextImageKHR
#cgo nocallback vkAcquireNextImageKHR
#cgo noescape vkQueuePresentKHR
#cgo nocallback vkQueuePresentKHR
#cgo noescape vkWaitForFences
#cgo nocallback vkWaitForFences
#cgo noescape vkFreeCommandBuffers
#cgo nocallback vkFreeCommandBuffers
#include <vulkan/vulkan.h>
*/
import "C"

import "unsafe"

// This file and its platform siblings (vk_darwin.go, vk_linux.go, vk_windows.go) are the
// package's only contact with C: the rest of the package calls Vulkan through the
// names declared here, and never imports "C". The wrappers were generated from the
// Vulkan headers' prototypes; add new ones by hand in the same form. They follow one
// naming convention, shared with gpu/metal's objc_darwin.go, so that a C name always
// gives its Go name:
//
//   - A function keeps its C name, unexported as Vulkan's names already are, and takes
//     the same parameters: vkCreateInstance.
//   - A type is its C name with the first letter lowered: VkImageCreateInfo is
//     vkImageCreateInfo. A struct keeps its C field names.
//   - A C scalar or other non-Vulkan type is "c" and its name in Go style: uint32_t is
//     cUint32, float cFloat, size_t cSize.
//   - A constant or enumerator is its C name in Go camel case: each word capitalized,
//     the KHR and EXT suffixes as initialisms, a word holding a digit kept as written,
//     and an underscore kept between two numbers. VK_PRESENT_MODE_FIFO_KHR is
//     vkPresentModeFifoKHR, VK_FORMAT_R8G8B8A8_UNORM vkFormatR8G8B8A8Unorm,
//     VK_API_VERSION_1_3 vkApiVersion1_3.
//
// When cgo bindings replace cgo, these declarations are what changes.

type (
	cChar                                          = C.char
	cFloat                                         = C.float
	cInt32                                         = C.int32_t
	cSize                                          = C.size_t
	cUint32                                        = C.uint32_t
	cUint64                                        = C.uint64_t
	vkAccessFlags2                                 = C.VkAccessFlags2
	vkAllocationCallbacks                          = C.VkAllocationCallbacks
	vkApplicationInfo                              = C.VkApplicationInfo
	vkAttachmentLoadOp                             = C.VkAttachmentLoadOp
	vkAttachmentStoreOp                            = C.VkAttachmentStoreOp
	vkBlendFactor                                  = C.VkBlendFactor
	vkBlendOp                                      = C.VkBlendOp
	vkBool32                                       = C.VkBool32
	vkBuffer                                       = C.VkBuffer
	vkBufferCopy                                   = C.VkBufferCopy
	vkBufferCreateInfo                             = C.VkBufferCreateInfo
	vkBufferDeviceAddressInfo                      = C.VkBufferDeviceAddressInfo
	vkBufferImageCopy                              = C.VkBufferImageCopy
	vkColorComponentFlags                          = C.VkColorComponentFlags
	vkCommandBuffer                                = C.VkCommandBuffer
	vkCommandBufferAllocateInfo                    = C.VkCommandBufferAllocateInfo
	vkCommandBufferBeginInfo                       = C.VkCommandBufferBeginInfo
	vkCommandBufferSubmitInfo                      = C.VkCommandBufferSubmitInfo
	vkCommandPool                                  = C.VkCommandPool
	vkCommandPoolCreateInfo                        = C.VkCommandPoolCreateInfo
	vkCompareOp                                    = C.VkCompareOp
	vkComputePipelineCreateInfo                    = C.VkComputePipelineCreateInfo
	vkCopyDescriptorSet                            = C.VkCopyDescriptorSet
	vkCullModeFlags                                = C.VkCullModeFlags
	vkDependencyInfo                               = C.VkDependencyInfo
	vkDescriptorBindingFlags                       = C.VkDescriptorBindingFlags
	vkDescriptorImageInfo                          = C.VkDescriptorImageInfo
	vkDescriptorPool                               = C.VkDescriptorPool
	vkDescriptorPoolCreateInfo                     = C.VkDescriptorPoolCreateInfo
	vkDescriptorPoolSize                           = C.VkDescriptorPoolSize
	vkDescriptorSet                                = C.VkDescriptorSet
	vkDescriptorSetAllocateInfo                    = C.VkDescriptorSetAllocateInfo
	vkDescriptorSetLayout                          = C.VkDescriptorSetLayout
	vkDescriptorSetLayoutBinding                   = C.VkDescriptorSetLayoutBinding
	vkDescriptorSetLayoutBindingFlagsCreateInfo    = C.VkDescriptorSetLayoutBindingFlagsCreateInfo
	vkDescriptorSetLayoutCreateInfo                = C.VkDescriptorSetLayoutCreateInfo
	vkDescriptorType                               = C.VkDescriptorType
	vkDevice                                       = C.VkDevice
	vkDeviceAddress                                = C.VkDeviceAddress
	vkDeviceCreateInfo                             = C.VkDeviceCreateInfo
	vkDeviceMemory                                 = C.VkDeviceMemory
	vkDeviceQueueCreateInfo                        = C.VkDeviceQueueCreateInfo
	vkDeviceSize                                   = C.VkDeviceSize
	vkDynamicState                                 = C.VkDynamicState
	vkExtensionProperties                          = C.VkExtensionProperties
	vkExtent2D                                     = C.VkExtent2D
	vkExtent3D                                     = C.VkExtent3D
	vkFence                                        = C.VkFence
	vkFenceCreateInfo                              = C.VkFenceCreateInfo
	vkFilter                                       = C.VkFilter
	vkFormat                                       = C.VkFormat
	vkFrontFace                                    = C.VkFrontFace
	vkGraphicsPipelineCreateInfo                   = C.VkGraphicsPipelineCreateInfo
	vkImage                                        = C.VkImage
	vkImageAspectFlags                             = C.VkImageAspectFlags
	vkImageCopy                                    = C.VkImageCopy
	vkImageCreateInfo                              = C.VkImageCreateInfo
	vkImageLayout                                  = C.VkImageLayout
	vkImageMemoryBarrier2                          = C.VkImageMemoryBarrier2
	vkImageSubresourceLayers                       = C.VkImageSubresourceLayers
	vkImageSubresourceRange                        = C.VkImageSubresourceRange
	vkImageUsageFlags                              = C.VkImageUsageFlags
	vkImageView                                    = C.VkImageView
	vkImageViewCreateInfo                          = C.VkImageViewCreateInfo
	vkImageViewType                                = C.VkImageViewType
	vkIndexType                                    = C.VkIndexType
	vkInstance                                     = C.VkInstance
	vkInstanceCreateInfo                           = C.VkInstanceCreateInfo
	vkMemoryAllocateFlagsInfo                      = C.VkMemoryAllocateFlagsInfo
	vkMemoryAllocateInfo                           = C.VkMemoryAllocateInfo
	vkMemoryBarrier2                               = C.VkMemoryBarrier2
	vkMemoryMapFlags                               = C.VkMemoryMapFlags
	vkMemoryPropertyFlags                          = C.VkMemoryPropertyFlags
	vkMemoryRequirements                           = C.VkMemoryRequirements
	vkOffset2D                                     = C.VkOffset2D
	vkPhysicalDevice                               = C.VkPhysicalDevice
	vkPhysicalDeviceFeatures2                      = C.VkPhysicalDeviceFeatures2
	vkPhysicalDeviceMemoryProperties               = C.VkPhysicalDeviceMemoryProperties
	vkPhysicalDeviceProperties                     = C.VkPhysicalDeviceProperties
	vkPhysicalDeviceProperties2                    = C.VkPhysicalDeviceProperties2
	vkPhysicalDeviceUnifiedImageLayoutsFeaturesKHR = C.VkPhysicalDeviceUnifiedImageLayoutsFeaturesKHR
	vkPhysicalDeviceVulkan12Features               = C.VkPhysicalDeviceVulkan12Features
	vkPhysicalDeviceVulkan12Properties             = C.VkPhysicalDeviceVulkan12Properties
	vkPhysicalDeviceVulkan13Features               = C.VkPhysicalDeviceVulkan13Features
	vkPipeline                                     = C.VkPipeline
	vkPipelineBindPoint                            = C.VkPipelineBindPoint
	vkPipelineCache                                = C.VkPipelineCache
	vkPipelineColorBlendAttachmentState            = C.VkPipelineColorBlendAttachmentState
	vkPipelineColorBlendStateCreateInfo            = C.VkPipelineColorBlendStateCreateInfo
	vkPipelineDepthStencilStateCreateInfo          = C.VkPipelineDepthStencilStateCreateInfo
	vkPipelineDynamicStateCreateInfo               = C.VkPipelineDynamicStateCreateInfo
	vkPipelineInputAssemblyStateCreateInfo         = C.VkPipelineInputAssemblyStateCreateInfo
	vkPipelineLayout                               = C.VkPipelineLayout
	vkPipelineLayoutCreateInfo                     = C.VkPipelineLayoutCreateInfo
	vkPipelineMultisampleStateCreateInfo           = C.VkPipelineMultisampleStateCreateInfo
	vkPipelineRasterizationStateCreateInfo         = C.VkPipelineRasterizationStateCreateInfo
	vkPipelineRenderingCreateInfo                  = C.VkPipelineRenderingCreateInfo
	vkPipelineShaderStageCreateInfo                = C.VkPipelineShaderStageCreateInfo
	vkPipelineStageFlags2                          = C.VkPipelineStageFlags2
	vkPipelineVertexInputStateCreateInfo           = C.VkPipelineVertexInputStateCreateInfo
	vkPipelineViewportStateCreateInfo              = C.VkPipelineViewportStateCreateInfo
	vkPresentInfoKHR                               = C.VkPresentInfoKHR
	vkPresentModeKHR                               = C.VkPresentModeKHR
	vkPrimitiveTopology                            = C.VkPrimitiveTopology
	vkPushConstantRange                            = C.VkPushConstantRange
	vkQueryPool                                    = C.VkQueryPool
	vkQueryPoolCreateInfo                          = C.VkQueryPoolCreateInfo
	vkQueryResultFlags                             = C.VkQueryResultFlags
	vkQueue                                        = C.VkQueue
	vkQueueFamilyProperties                        = C.VkQueueFamilyProperties
	vkRect2D                                       = C.VkRect2D
	vkRenderingAttachmentInfo                      = C.VkRenderingAttachmentInfo
	vkRenderingInfo                                = C.VkRenderingInfo
	vkResult                                       = C.VkResult
	vkSampleCountFlagBits                          = C.VkSampleCountFlagBits
	vkSampler                                      = C.VkSampler
	vkSamplerAddressMode                           = C.VkSamplerAddressMode
	vkSamplerCreateInfo                            = C.VkSamplerCreateInfo
	vkSamplerMipmapMode                            = C.VkSamplerMipmapMode
	vkSemaphore                                    = C.VkSemaphore
	vkSemaphoreCreateInfo                          = C.VkSemaphoreCreateInfo
	vkSemaphoreSubmitInfo                          = C.VkSemaphoreSubmitInfo
	vkShaderModule                                 = C.VkShaderModule
	vkShaderModuleCreateInfo                       = C.VkShaderModuleCreateInfo
	vkShaderStageFlags                             = C.VkShaderStageFlags
	vkSpecializationInfo                           = C.VkSpecializationInfo
	vkSpecializationMapEntry                       = C.VkSpecializationMapEntry
	vkSubmitInfo2                                  = C.VkSubmitInfo2
	vkSurfaceCapabilitiesKHR                       = C.VkSurfaceCapabilitiesKHR
	vkSurfaceFormatKHR                             = C.VkSurfaceFormatKHR
	vkSurfaceKHR                                   = C.VkSurfaceKHR
	vkSwapchainCreateInfoKHR                       = C.VkSwapchainCreateInfoKHR
	vkSwapchainKHR                                 = C.VkSwapchainKHR
	vkViewport                                     = C.VkViewport
	vkWriteDescriptorSet                           = C.VkWriteDescriptorSet
)

const (
	vkApiVersion1_3                                             = C.VK_API_VERSION_1_3
	vkAttachmentLoadOpClear                                     = C.VK_ATTACHMENT_LOAD_OP_CLEAR
	vkAttachmentLoadOpDontCare                                  = C.VK_ATTACHMENT_LOAD_OP_DONT_CARE
	vkAttachmentLoadOpLoad                                      = C.VK_ATTACHMENT_LOAD_OP_LOAD
	vkAttachmentStoreOpDontCare                                 = C.VK_ATTACHMENT_STORE_OP_DONT_CARE
	vkAttachmentStoreOpStore                                    = C.VK_ATTACHMENT_STORE_OP_STORE
	vkBlendFactorDstAlpha                                       = C.VK_BLEND_FACTOR_DST_ALPHA
	vkBlendFactorOne                                            = C.VK_BLEND_FACTOR_ONE
	vkBlendFactorOneMinusDstAlpha                               = C.VK_BLEND_FACTOR_ONE_MINUS_DST_ALPHA
	vkBlendFactorOneMinusSrcAlpha                               = C.VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA
	vkBlendFactorSrcAlpha                                       = C.VK_BLEND_FACTOR_SRC_ALPHA
	vkBlendFactorZero                                           = C.VK_BLEND_FACTOR_ZERO
	vkBlendOpAdd                                                = C.VK_BLEND_OP_ADD
	vkBlendOpMax                                                = C.VK_BLEND_OP_MAX
	vkBlendOpMin                                                = C.VK_BLEND_OP_MIN
	vkBlendOpReverseSubtract                                    = C.VK_BLEND_OP_REVERSE_SUBTRACT
	vkBlendOpSubtract                                           = C.VK_BLEND_OP_SUBTRACT
	vkBufferUsageIndexBufferBit                                 = C.VK_BUFFER_USAGE_INDEX_BUFFER_BIT
	vkBufferUsageIndirectBufferBit                              = C.VK_BUFFER_USAGE_INDIRECT_BUFFER_BIT
	vkBufferUsageShaderDeviceAddressBit                         = C.VK_BUFFER_USAGE_SHADER_DEVICE_ADDRESS_BIT
	vkBufferUsageStorageBufferBit                               = C.VK_BUFFER_USAGE_STORAGE_BUFFER_BIT
	vkBufferUsageTransferDstBit                                 = C.VK_BUFFER_USAGE_TRANSFER_DST_BIT
	vkBufferUsageTransferSrcBit                                 = C.VK_BUFFER_USAGE_TRANSFER_SRC_BIT
	vkBufferUsageUniformBufferBit                               = C.VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT
	vkColorComponentABit                                        = C.VK_COLOR_COMPONENT_A_BIT
	vkColorComponentBBit                                        = C.VK_COLOR_COMPONENT_B_BIT
	vkColorComponentGBit                                        = C.VK_COLOR_COMPONENT_G_BIT
	vkColorComponentRBit                                        = C.VK_COLOR_COMPONENT_R_BIT
	vkColorSpaceSrgbNonlinearKHR                                = C.VK_COLOR_SPACE_SRGB_NONLINEAR_KHR
	vkCommandBufferLevelPrimary                                 = C.VK_COMMAND_BUFFER_LEVEL_PRIMARY
	vkCommandBufferUsageOneTimeSubmitBit                        = C.VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT
	vkCommandPoolCreateResetCommandBufferBit                    = C.VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT
	vkCompareOpAlways                                           = C.VK_COMPARE_OP_ALWAYS
	vkCompareOpEqual                                            = C.VK_COMPARE_OP_EQUAL
	vkCompareOpGreater                                          = C.VK_COMPARE_OP_GREATER
	vkCompareOpGreaterOrEqual                                   = C.VK_COMPARE_OP_GREATER_OR_EQUAL
	vkCompareOpLess                                             = C.VK_COMPARE_OP_LESS
	vkCompareOpLessOrEqual                                      = C.VK_COMPARE_OP_LESS_OR_EQUAL
	vkCompareOpNever                                            = C.VK_COMPARE_OP_NEVER
	vkCompareOpNotEqual                                         = C.VK_COMPARE_OP_NOT_EQUAL
	vkCompositeAlphaOpaqueBitKHR                                = C.VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR
	vkCullModeBackBit                                           = C.VK_CULL_MODE_BACK_BIT
	vkCullModeFrontBit                                          = C.VK_CULL_MODE_FRONT_BIT
	vkCullModeNone                                              = C.VK_CULL_MODE_NONE
	vkDescriptorBindingPartiallyBoundBit                        = C.VK_DESCRIPTOR_BINDING_PARTIALLY_BOUND_BIT
	vkDescriptorBindingUpdateAfterBindBit                       = C.VK_DESCRIPTOR_BINDING_UPDATE_AFTER_BIND_BIT
	vkDescriptorPoolCreateUpdateAfterBindBit                    = C.VK_DESCRIPTOR_POOL_CREATE_UPDATE_AFTER_BIND_BIT
	vkDescriptorSetLayoutCreateUpdateAfterBindPoolBit           = C.VK_DESCRIPTOR_SET_LAYOUT_CREATE_UPDATE_AFTER_BIND_POOL_BIT
	vkDescriptorTypeSampledImage                                = C.VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE
	vkDescriptorTypeSampler                                     = C.VK_DESCRIPTOR_TYPE_SAMPLER
	vkDescriptorTypeStorageImage                                = C.VK_DESCRIPTOR_TYPE_STORAGE_IMAGE
	vkDynamicStateDepthBias                                     = C.VK_DYNAMIC_STATE_DEPTH_BIAS
	vkDynamicStateScissor                                       = C.VK_DYNAMIC_STATE_SCISSOR
	vkDynamicStateViewport                                      = C.VK_DYNAMIC_STATE_VIEWPORT
	vkErrorFeatureNotPresent                                    = C.VK_ERROR_FEATURE_NOT_PRESENT
	vkErrorInitializationFailed                                 = C.VK_ERROR_INITIALIZATION_FAILED
	vkErrorOutOfDeviceMemory                                    = C.VK_ERROR_OUT_OF_DEVICE_MEMORY
	vkFalse                                                     = C.VK_FALSE
	vkFilterLinear                                              = C.VK_FILTER_LINEAR
	vkFilterNearest                                             = C.VK_FILTER_NEAREST
	vkFormatA2B10G10R10UintPACK32                               = C.VK_FORMAT_A2B10G10R10_UINT_PACK32
	vkFormatA2B10G10R10UnormPACK32                              = C.VK_FORMAT_A2B10G10R10_UNORM_PACK32
	vkFormatAstc10x10SrgbBlock                                  = C.VK_FORMAT_ASTC_10x10_SRGB_BLOCK
	vkFormatAstc10x10UnormBlock                                 = C.VK_FORMAT_ASTC_10x10_UNORM_BLOCK
	vkFormatAstc12x12SrgbBlock                                  = C.VK_FORMAT_ASTC_12x12_SRGB_BLOCK
	vkFormatAstc12x12UnormBlock                                 = C.VK_FORMAT_ASTC_12x12_UNORM_BLOCK
	vkFormatAstc4x4SrgbBlock                                    = C.VK_FORMAT_ASTC_4x4_SRGB_BLOCK
	vkFormatAstc4x4UnormBlock                                   = C.VK_FORMAT_ASTC_4x4_UNORM_BLOCK
	vkFormatAstc5x5SrgbBlock                                    = C.VK_FORMAT_ASTC_5x5_SRGB_BLOCK
	vkFormatAstc5x5UnormBlock                                   = C.VK_FORMAT_ASTC_5x5_UNORM_BLOCK
	vkFormatAstc6x6SrgbBlock                                    = C.VK_FORMAT_ASTC_6x6_SRGB_BLOCK
	vkFormatAstc6x6UnormBlock                                   = C.VK_FORMAT_ASTC_6x6_UNORM_BLOCK
	vkFormatAstc8x8SrgbBlock                                    = C.VK_FORMAT_ASTC_8x8_SRGB_BLOCK
	vkFormatAstc8x8UnormBlock                                   = C.VK_FORMAT_ASTC_8x8_UNORM_BLOCK
	vkFormatB10G11R11UfloatPACK32                               = C.VK_FORMAT_B10G11R11_UFLOAT_PACK32
	vkFormatB8G8R8A8Srgb                                        = C.VK_FORMAT_B8G8R8A8_SRGB
	vkFormatB8G8R8A8Unorm                                       = C.VK_FORMAT_B8G8R8A8_UNORM
	vkFormatBC1RgbaSrgbBlock                                    = C.VK_FORMAT_BC1_RGBA_SRGB_BLOCK
	vkFormatBC1RgbaUnormBlock                                   = C.VK_FORMAT_BC1_RGBA_UNORM_BLOCK
	vkFormatBC3SrgbBlock                                        = C.VK_FORMAT_BC3_SRGB_BLOCK
	vkFormatBC3UnormBlock                                       = C.VK_FORMAT_BC3_UNORM_BLOCK
	vkFormatBC4SnormBlock                                       = C.VK_FORMAT_BC4_SNORM_BLOCK
	vkFormatBC4UnormBlock                                       = C.VK_FORMAT_BC4_UNORM_BLOCK
	vkFormatBC5SnormBlock                                       = C.VK_FORMAT_BC5_SNORM_BLOCK
	vkFormatBC5UnormBlock                                       = C.VK_FORMAT_BC5_UNORM_BLOCK
	vkFormatBC6HSfloatBlock                                     = C.VK_FORMAT_BC6H_SFLOAT_BLOCK
	vkFormatBC6HUfloatBlock                                     = C.VK_FORMAT_BC6H_UFLOAT_BLOCK
	vkFormatBC7SrgbBlock                                        = C.VK_FORMAT_BC7_SRGB_BLOCK
	vkFormatBC7UnormBlock                                       = C.VK_FORMAT_BC7_UNORM_BLOCK
	vkFormatD16Unorm                                            = C.VK_FORMAT_D16_UNORM
	vkFormatD24UnormS8Uint                                      = C.VK_FORMAT_D24_UNORM_S8_UINT
	vkFormatD32Sfloat                                           = C.VK_FORMAT_D32_SFLOAT
	vkFormatD32SfloatS8Uint                                     = C.VK_FORMAT_D32_SFLOAT_S8_UINT
	vkFormatE5B9G9R9UfloatPACK32                                = C.VK_FORMAT_E5B9G9R9_UFLOAT_PACK32
	vkFormatR16G16B16A16Sfloat                                  = C.VK_FORMAT_R16G16B16A16_SFLOAT
	vkFormatR16G16B16A16Sint                                    = C.VK_FORMAT_R16G16B16A16_SINT
	vkFormatR16G16B16A16Snorm                                   = C.VK_FORMAT_R16G16B16A16_SNORM
	vkFormatR16G16B16A16Uint                                    = C.VK_FORMAT_R16G16B16A16_UINT
	vkFormatR16G16B16A16Unorm                                   = C.VK_FORMAT_R16G16B16A16_UNORM
	vkFormatR16G16Sfloat                                        = C.VK_FORMAT_R16G16_SFLOAT
	vkFormatR16G16Sint                                          = C.VK_FORMAT_R16G16_SINT
	vkFormatR16G16Snorm                                         = C.VK_FORMAT_R16G16_SNORM
	vkFormatR16G16Uint                                          = C.VK_FORMAT_R16G16_UINT
	vkFormatR16G16Unorm                                         = C.VK_FORMAT_R16G16_UNORM
	vkFormatR16Sfloat                                           = C.VK_FORMAT_R16_SFLOAT
	vkFormatR16Sint                                             = C.VK_FORMAT_R16_SINT
	vkFormatR16Snorm                                            = C.VK_FORMAT_R16_SNORM
	vkFormatR16Uint                                             = C.VK_FORMAT_R16_UINT
	vkFormatR16Unorm                                            = C.VK_FORMAT_R16_UNORM
	vkFormatR32G32B32A32Sfloat                                  = C.VK_FORMAT_R32G32B32A32_SFLOAT
	vkFormatR32G32B32A32Sint                                    = C.VK_FORMAT_R32G32B32A32_SINT
	vkFormatR32G32B32A32Uint                                    = C.VK_FORMAT_R32G32B32A32_UINT
	vkFormatR32G32Sfloat                                        = C.VK_FORMAT_R32G32_SFLOAT
	vkFormatR32G32Sint                                          = C.VK_FORMAT_R32G32_SINT
	vkFormatR32G32Uint                                          = C.VK_FORMAT_R32G32_UINT
	vkFormatR32Sfloat                                           = C.VK_FORMAT_R32_SFLOAT
	vkFormatR32Sint                                             = C.VK_FORMAT_R32_SINT
	vkFormatR32Uint                                             = C.VK_FORMAT_R32_UINT
	vkFormatR8G8B8A8Sint                                        = C.VK_FORMAT_R8G8B8A8_SINT
	vkFormatR8G8B8A8Snorm                                       = C.VK_FORMAT_R8G8B8A8_SNORM
	vkFormatR8G8B8A8Srgb                                        = C.VK_FORMAT_R8G8B8A8_SRGB
	vkFormatR8G8B8A8Uint                                        = C.VK_FORMAT_R8G8B8A8_UINT
	vkFormatR8G8B8A8Unorm                                       = C.VK_FORMAT_R8G8B8A8_UNORM
	vkFormatR8G8Sint                                            = C.VK_FORMAT_R8G8_SINT
	vkFormatR8G8Snorm                                           = C.VK_FORMAT_R8G8_SNORM
	vkFormatR8G8Uint                                            = C.VK_FORMAT_R8G8_UINT
	vkFormatR8G8Unorm                                           = C.VK_FORMAT_R8G8_UNORM
	vkFormatR8Sint                                              = C.VK_FORMAT_R8_SINT
	vkFormatR8Snorm                                             = C.VK_FORMAT_R8_SNORM
	vkFormatR8Uint                                              = C.VK_FORMAT_R8_UINT
	vkFormatR8Unorm                                             = C.VK_FORMAT_R8_UNORM
	vkFormatS8Uint                                              = C.VK_FORMAT_S8_UINT
	vkFormatUndefined                                           = C.VK_FORMAT_UNDEFINED
	vkFrontFaceClockwise                                        = C.VK_FRONT_FACE_CLOCKWISE
	vkFrontFaceCounterClockwise                                 = C.VK_FRONT_FACE_COUNTER_CLOCKWISE
	vkImageAspectColorBit                                       = C.VK_IMAGE_ASPECT_COLOR_BIT
	vkImageAspectDepthBit                                       = C.VK_IMAGE_ASPECT_DEPTH_BIT
	vkImageCreateCubeCompatibleBit                              = C.VK_IMAGE_CREATE_CUBE_COMPATIBLE_BIT
	vkImageLayoutGeneral                                        = C.VK_IMAGE_LAYOUT_GENERAL
	vkImageLayoutPresentSrcKHR                                  = C.VK_IMAGE_LAYOUT_PRESENT_SRC_KHR
	vkImageLayoutUndefined                                      = C.VK_IMAGE_LAYOUT_UNDEFINED
	vkImageTilingOptimal                                        = C.VK_IMAGE_TILING_OPTIMAL
	vkImageType2D                                               = C.VK_IMAGE_TYPE_2D
	vkImageType3D                                               = C.VK_IMAGE_TYPE_3D
	vkImageUsageColorAttachmentBit                              = C.VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT
	vkImageUsageDepthStencilAttachmentBit                       = C.VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT
	vkImageUsageSampledBit                                      = C.VK_IMAGE_USAGE_SAMPLED_BIT
	vkImageUsageStorageBit                                      = C.VK_IMAGE_USAGE_STORAGE_BIT
	vkImageUsageTransferDstBit                                  = C.VK_IMAGE_USAGE_TRANSFER_DST_BIT
	vkImageUsageTransferSrcBit                                  = C.VK_IMAGE_USAGE_TRANSFER_SRC_BIT
	vkImageUsageTransientAttachmentBit                          = C.VK_IMAGE_USAGE_TRANSIENT_ATTACHMENT_BIT
	vkImageViewType2D                                           = C.VK_IMAGE_VIEW_TYPE_2D
	vkImageViewType2DArray                                      = C.VK_IMAGE_VIEW_TYPE_2D_ARRAY
	vkImageViewType3D                                           = C.VK_IMAGE_VIEW_TYPE_3D
	vkImageViewTypeCube                                         = C.VK_IMAGE_VIEW_TYPE_CUBE
	vkImageViewTypeCubeArray                                    = C.VK_IMAGE_VIEW_TYPE_CUBE_ARRAY
	vkIndexTypeUINT16                                           = C.VK_INDEX_TYPE_UINT16
	vkIndexTypeUINT32                                           = C.VK_INDEX_TYPE_UINT32
	vkLodClampNone                                              = C.VK_LOD_CLAMP_NONE
	vkMaxPhysicalDeviceNameSize                                 = C.VK_MAX_PHYSICAL_DEVICE_NAME_SIZE
	vkMemoryAllocateDeviceAddressBit                            = C.VK_MEMORY_ALLOCATE_DEVICE_ADDRESS_BIT
	vkMemoryPropertyDeviceLocalBit                              = C.VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT
	vkMemoryPropertyHostCoherentBit                             = C.VK_MEMORY_PROPERTY_HOST_COHERENT_BIT
	vkMemoryPropertyHostVisibleBit                              = C.VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT
	vkMemoryPropertyLazilyAllocatedBit                          = C.VK_MEMORY_PROPERTY_LAZILY_ALLOCATED_BIT
	vkPhysicalDeviceTypeDiscreteGpu                             = C.VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU
	vkPipelineBindPointCompute                                  = C.VK_PIPELINE_BIND_POINT_COMPUTE
	vkPipelineBindPointGraphics                                 = C.VK_PIPELINE_BIND_POINT_GRAPHICS
	vkPolygonModeFill                                           = C.VK_POLYGON_MODE_FILL
	vkPresentModeFifoKHR                                        = C.VK_PRESENT_MODE_FIFO_KHR
	vkPresentModeImmediateKHR                                   = C.VK_PRESENT_MODE_IMMEDIATE_KHR
	vkPresentModeMailboxKHR                                     = C.VK_PRESENT_MODE_MAILBOX_KHR
	vkPrimitiveTopologyLineList                                 = C.VK_PRIMITIVE_TOPOLOGY_LINE_LIST
	vkPrimitiveTopologyPointList                                = C.VK_PRIMITIVE_TOPOLOGY_POINT_LIST
	vkPrimitiveTopologyTriangleList                             = C.VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST
	vkPrimitiveTopologyTriangleStrip                            = C.VK_PRIMITIVE_TOPOLOGY_TRIANGLE_STRIP
	vkQueryResult64Bit                                          = C.VK_QUERY_RESULT_64_BIT
	vkQueryResultWaitBit                                        = C.VK_QUERY_RESULT_WAIT_BIT
	vkQueryTypeTimestamp                                        = C.VK_QUERY_TYPE_TIMESTAMP
	vkQueueComputeBit                                           = C.VK_QUEUE_COMPUTE_BIT
	vkQueueGraphicsBit                                          = C.VK_QUEUE_GRAPHICS_BIT
	vkRemainingArrayLayers                                      = C.VK_REMAINING_ARRAY_LAYERS
	vkRemainingMipLevels                                        = C.VK_REMAINING_MIP_LEVELS
	vkResolveModeAverageBit                                     = C.VK_RESOLVE_MODE_AVERAGE_BIT
	vkResolveModeSampleZeroBit                                  = C.VK_RESOLVE_MODE_SAMPLE_ZERO_BIT
	vkSamplerAddressModeClampToEdge                             = C.VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE
	vkSamplerAddressModeMirroredRepeat                          = C.VK_SAMPLER_ADDRESS_MODE_MIRRORED_REPEAT
	vkSamplerAddressModeRepeat                                  = C.VK_SAMPLER_ADDRESS_MODE_REPEAT
	vkSamplerMipmapModeLinear                                   = C.VK_SAMPLER_MIPMAP_MODE_LINEAR
	vkSamplerMipmapModeNearest                                  = C.VK_SAMPLER_MIPMAP_MODE_NEAREST
	vkShaderStageAll                                            = C.VK_SHADER_STAGE_ALL
	vkShaderStageComputeBit                                     = C.VK_SHADER_STAGE_COMPUTE_BIT
	vkShaderStageFragmentBit                                    = C.VK_SHADER_STAGE_FRAGMENT_BIT
	vkShaderStageVertexBit                                      = C.VK_SHADER_STAGE_VERTEX_BIT
	vkSharingModeExclusive                                      = C.VK_SHARING_MODE_EXCLUSIVE
	vkStructureTypeApplicationInfo                              = C.VK_STRUCTURE_TYPE_APPLICATION_INFO
	vkStructureTypeBufferCreateInfo                             = C.VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO
	vkStructureTypeBufferDeviceAddressInfo                      = C.VK_STRUCTURE_TYPE_BUFFER_DEVICE_ADDRESS_INFO
	vkStructureTypeCommandBufferAllocateInfo                    = C.VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO
	vkStructureTypeCommandBufferBeginInfo                       = C.VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO
	vkStructureTypeCommandBufferSubmitInfo                      = C.VK_STRUCTURE_TYPE_COMMAND_BUFFER_SUBMIT_INFO
	vkStructureTypeCommandPoolCreateInfo                        = C.VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO
	vkStructureTypeComputePipelineCreateInfo                    = C.VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO
	vkStructureTypeDependencyInfo                               = C.VK_STRUCTURE_TYPE_DEPENDENCY_INFO
	vkStructureTypeDescriptorPoolCreateInfo                     = C.VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO
	vkStructureTypeDescriptorSetAllocateInfo                    = C.VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO
	vkStructureTypeDescriptorSetLayoutBindingFlagsCreateInfo    = C.VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_BINDING_FLAGS_CREATE_INFO
	vkStructureTypeDescriptorSetLayoutCreateInfo                = C.VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO
	vkStructureTypeDeviceCreateInfo                             = C.VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO
	vkStructureTypeDeviceQueueCreateInfo                        = C.VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO
	vkStructureTypeFenceCreateInfo                              = C.VK_STRUCTURE_TYPE_FENCE_CREATE_INFO
	vkStructureTypeGraphicsPipelineCreateInfo                   = C.VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO
	vkStructureTypeImageCreateInfo                              = C.VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO
	vkStructureTypeImageMemoryBarrier2                          = C.VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER_2
	vkStructureTypeImageViewCreateInfo                          = C.VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO
	vkStructureTypeInstanceCreateInfo                           = C.VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO
	vkStructureTypeMemoryAllocateFlagsInfo                      = C.VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_FLAGS_INFO
	vkStructureTypeMemoryAllocateInfo                           = C.VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO
	vkStructureTypeMemoryBarrier2                               = C.VK_STRUCTURE_TYPE_MEMORY_BARRIER_2
	vkStructureTypePhysicalDeviceFeatures2                      = C.VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_FEATURES_2
	vkStructureTypePhysicalDeviceProperties2                    = C.VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_PROPERTIES_2
	vkStructureTypePhysicalDeviceUnifiedImageLayoutsFeaturesKHR = C.VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_UNIFIED_IMAGE_LAYOUTS_FEATURES_KHR
	vkStructureTypePhysicalDeviceVulkan1_2Features              = C.VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_1_2_FEATURES
	vkStructureTypePhysicalDeviceVulkan1_2Properties            = C.VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_1_2_PROPERTIES
	vkStructureTypePhysicalDeviceVulkan1_3Features              = C.VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_1_3_FEATURES
	vkStructureTypePipelineColorBlendStateCreateInfo            = C.VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO
	vkStructureTypePipelineDepthStencilStateCreateInfo          = C.VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO
	vkStructureTypePipelineDynamicStateCreateInfo               = C.VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO
	vkStructureTypePipelineInputAssemblyStateCreateInfo         = C.VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO
	vkStructureTypePipelineLayoutCreateInfo                     = C.VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO
	vkStructureTypePipelineMultisampleStateCreateInfo           = C.VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO
	vkStructureTypePipelineRasterizationStateCreateInfo         = C.VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO
	vkStructureTypePipelineRenderingCreateInfo                  = C.VK_STRUCTURE_TYPE_PIPELINE_RENDERING_CREATE_INFO
	vkStructureTypePipelineShaderStageCreateInfo                = C.VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO
	vkStructureTypePipelineVertexInputStateCreateInfo           = C.VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO
	vkStructureTypePipelineViewportStateCreateInfo              = C.VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO
	vkStructureTypePresentInfoKHR                               = C.VK_STRUCTURE_TYPE_PRESENT_INFO_KHR
	vkStructureTypeQueryPoolCreateInfo                          = C.VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO
	vkStructureTypeRenderingAttachmentInfo                      = C.VK_STRUCTURE_TYPE_RENDERING_ATTACHMENT_INFO
	vkStructureTypeRenderingInfo                                = C.VK_STRUCTURE_TYPE_RENDERING_INFO
	vkStructureTypeSamplerCreateInfo                            = C.VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO
	vkStructureTypeSemaphoreCreateInfo                          = C.VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO
	vkStructureTypeSemaphoreSubmitInfo                          = C.VK_STRUCTURE_TYPE_SEMAPHORE_SUBMIT_INFO
	vkStructureTypeShaderModuleCreateInfo                       = C.VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO
	vkStructureTypeSubmitInfo2                                  = C.VK_STRUCTURE_TYPE_SUBMIT_INFO_2
	vkStructureTypeSwapchainCreateInfoKHR                       = C.VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR
	vkStructureTypeWriteDescriptorSet                           = C.VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET
	vkSuccess                                                   = C.VK_SUCCESS
	vkTrue                                                      = C.VK_TRUE
	vkWholeSize                                                 = C.VK_WHOLE_SIZE
)

// Synchronization2 flags are 64-bit and declared in vulkan_core.h as static const
// variables, not enumerators, so cgo would reference them as data symbols. Their
// values are fixed by the specification.
const (
	vkPipelineStage2None                     vkPipelineStageFlags2 = 0
	vkPipelineStage2TopOfPipeBit             vkPipelineStageFlags2 = 0x1     // TOP_OF_PIPE
	vkPipelineStage2DrawIndirectBit          vkPipelineStageFlags2 = 0x2     // DRAW_INDIRECT
	vkPipelineStage2VertexShaderBit          vkPipelineStageFlags2 = 0x8     // VERTEX_SHADER
	vkPipelineStage2FragmentShaderBit        vkPipelineStageFlags2 = 0x80    // FRAGMENT_SHADER
	vkPipelineStage2EarlyFragmentTestsBit    vkPipelineStageFlags2 = 0x100   // EARLY_FRAGMENT_TESTS
	vkPipelineStage2LateFragmentTestsBit     vkPipelineStageFlags2 = 0x200   // LATE_FRAGMENT_TESTS
	vkPipelineStage2ColorAttachmentOutputBit vkPipelineStageFlags2 = 0x400   // COLOR_ATTACHMENT_OUTPUT
	vkPipelineStage2ComputeShaderBit         vkPipelineStageFlags2 = 0x800   // COMPUTE_SHADER
	vkPipelineStage2AllTransferBit           vkPipelineStageFlags2 = 0x1000  // ALL_TRANSFER
	vkPipelineStage2BottomOfPipeBit          vkPipelineStageFlags2 = 0x2000  // BOTTOM_OF_PIPE
	vkPipelineStage2AllCommandsBit           vkPipelineStageFlags2 = 0x10000 // ALL_COMMANDS

	vkAccess2None                           vkAccessFlags2 = 0
	vkAccess2IndirectCommandReadBit         vkAccessFlags2 = 0x1     // INDIRECT_COMMAND_READ
	vkAccess2ShaderReadBit                  vkAccessFlags2 = 0x20    // SHADER_READ
	vkAccess2ShaderWriteBit                 vkAccessFlags2 = 0x40    // SHADER_WRITE
	vkAccess2ColorAttachmentReadBit         vkAccessFlags2 = 0x80    // COLOR_ATTACHMENT_READ
	vkAccess2ColorAttachmentWriteBit        vkAccessFlags2 = 0x100   // COLOR_ATTACHMENT_WRITE
	vkAccess2DepthStencilAttachmentReadBit  vkAccessFlags2 = 0x200   // DEPTH_STENCIL_ATTACHMENT_READ
	vkAccess2DepthStencilAttachmentWriteBit vkAccessFlags2 = 0x400   // DEPTH_STENCIL_ATTACHMENT_WRITE
	vkAccess2TransferReadBit                vkAccessFlags2 = 0x800   // TRANSFER_READ
	vkAccess2TransferWriteBit               vkAccessFlags2 = 0x1000  // TRANSFER_WRITE
	vkAccess2MemoryReadBit                  vkAccessFlags2 = 0x8000  // MEMORY_READ
	vkAccess2MemoryWriteBit                 vkAccessFlags2 = 0x10000 // MEMORY_WRITE
)

// Extension names, which the headers define as string macros: vkKhrSwapchainExtensionName
// is VK_KHR_SWAPCHAIN_EXTENSION_NAME.
const (
	vkKhrSwapchainExtensionName           = "VK_KHR_swapchain"
	vkKhrUnifiedImageLayoutsExtensionName = "VK_KHR_unified_image_layouts"
)

func vkAcquireNextImageKHR(device vkDevice, swapchain vkSwapchainKHR, timeout cUint64, semaphore vkSemaphore, fence vkFence, pImageIndex *cUint32) vkResult {
	return C.vkAcquireNextImageKHR(device, swapchain, timeout, semaphore, fence, pImageIndex)
}
func vkAllocateCommandBuffers(device vkDevice, pAllocateInfo *vkCommandBufferAllocateInfo, pCommandBuffers *vkCommandBuffer) vkResult {
	return C.vkAllocateCommandBuffers(device, pAllocateInfo, pCommandBuffers)
}
func vkAllocateDescriptorSets(device vkDevice, pAllocateInfo *vkDescriptorSetAllocateInfo, pDescriptorSets *vkDescriptorSet) vkResult {
	return C.vkAllocateDescriptorSets(device, pAllocateInfo, pDescriptorSets)
}
func vkAllocateMemory(device vkDevice, pAllocateInfo *vkMemoryAllocateInfo, pAllocator *vkAllocationCallbacks, pMemory *vkDeviceMemory) vkResult {
	return C.vkAllocateMemory(device, pAllocateInfo, pAllocator, pMemory)
}
func vkBeginCommandBuffer(commandBuffer vkCommandBuffer, pBeginInfo *vkCommandBufferBeginInfo) vkResult {
	return C.vkBeginCommandBuffer(commandBuffer, pBeginInfo)
}
func vkBindBufferMemory(device vkDevice, buffer vkBuffer, memory vkDeviceMemory, memoryOffset vkDeviceSize) vkResult {
	return C.vkBindBufferMemory(device, buffer, memory, memoryOffset)
}
func vkBindImageMemory(device vkDevice, image vkImage, memory vkDeviceMemory, memoryOffset vkDeviceSize) vkResult {
	return C.vkBindImageMemory(device, image, memory, memoryOffset)
}
func vkCmdBeginRendering(commandBuffer vkCommandBuffer, pRenderingInfo *vkRenderingInfo) {
	C.vkCmdBeginRendering(commandBuffer, pRenderingInfo)
}
func vkCmdBindDescriptorSets(commandBuffer vkCommandBuffer, pipelineBindPoint vkPipelineBindPoint, layout vkPipelineLayout, firstSet cUint32, descriptorSetCount cUint32, pDescriptorSets *vkDescriptorSet, dynamicOffsetCount cUint32, pDynamicOffsets *cUint32) {
	C.vkCmdBindDescriptorSets(commandBuffer, pipelineBindPoint, layout, firstSet, descriptorSetCount, pDescriptorSets, dynamicOffsetCount, pDynamicOffsets)
}
func vkCmdBindIndexBuffer(commandBuffer vkCommandBuffer, buffer vkBuffer, offset vkDeviceSize, indexType vkIndexType) {
	C.vkCmdBindIndexBuffer(commandBuffer, buffer, offset, indexType)
}
func vkCmdBindPipeline(commandBuffer vkCommandBuffer, pipelineBindPoint vkPipelineBindPoint, pipeline vkPipeline) {
	C.vkCmdBindPipeline(commandBuffer, pipelineBindPoint, pipeline)
}
func vkCmdCopyBuffer(commandBuffer vkCommandBuffer, srcBuffer vkBuffer, dstBuffer vkBuffer, regionCount cUint32, pRegions *vkBufferCopy) {
	C.vkCmdCopyBuffer(commandBuffer, srcBuffer, dstBuffer, regionCount, pRegions)
}
func vkCmdCopyBufferToImage(commandBuffer vkCommandBuffer, srcBuffer vkBuffer, dstImage vkImage, dstImageLayout vkImageLayout, regionCount cUint32, pRegions *vkBufferImageCopy) {
	C.vkCmdCopyBufferToImage(commandBuffer, srcBuffer, dstImage, dstImageLayout, regionCount, pRegions)
}
func vkCmdCopyImage(commandBuffer vkCommandBuffer, srcImage vkImage, srcImageLayout vkImageLayout, dstImage vkImage, dstImageLayout vkImageLayout, regionCount cUint32, pRegions *vkImageCopy) {
	C.vkCmdCopyImage(commandBuffer, srcImage, srcImageLayout, dstImage, dstImageLayout, regionCount, pRegions)
}
func vkCmdCopyImageToBuffer(commandBuffer vkCommandBuffer, srcImage vkImage, srcImageLayout vkImageLayout, dstBuffer vkBuffer, regionCount cUint32, pRegions *vkBufferImageCopy) {
	C.vkCmdCopyImageToBuffer(commandBuffer, srcImage, srcImageLayout, dstBuffer, regionCount, pRegions)
}
func vkCmdDispatch(commandBuffer vkCommandBuffer, groupCountX cUint32, groupCountY cUint32, groupCountZ cUint32) {
	C.vkCmdDispatch(commandBuffer, groupCountX, groupCountY, groupCountZ)
}
func vkCmdDispatchIndirect(commandBuffer vkCommandBuffer, buffer vkBuffer, offset vkDeviceSize) {
	C.vkCmdDispatchIndirect(commandBuffer, buffer, offset)
}
func vkCmdDraw(commandBuffer vkCommandBuffer, vertexCount cUint32, instanceCount cUint32, firstVertex cUint32, firstInstance cUint32) {
	C.vkCmdDraw(commandBuffer, vertexCount, instanceCount, firstVertex, firstInstance)
}
func vkCmdDrawIndexed(commandBuffer vkCommandBuffer, indexCount cUint32, instanceCount cUint32, firstIndex cUint32, vertexOffset cInt32, firstInstance cUint32) {
	C.vkCmdDrawIndexed(commandBuffer, indexCount, instanceCount, firstIndex, vertexOffset, firstInstance)
}
func vkCmdDrawIndexedIndirect(commandBuffer vkCommandBuffer, buffer vkBuffer, offset vkDeviceSize, drawCount cUint32, stride cUint32) {
	C.vkCmdDrawIndexedIndirect(commandBuffer, buffer, offset, drawCount, stride)
}
func vkCmdEndRendering(commandBuffer vkCommandBuffer) {
	C.vkCmdEndRendering(commandBuffer)
}
func vkCmdPipelineBarrier2(commandBuffer vkCommandBuffer, pDependencyInfo *vkDependencyInfo) {
	C.vkCmdPipelineBarrier2(commandBuffer, pDependencyInfo)
}
func vkCmdPushConstants(commandBuffer vkCommandBuffer, layout vkPipelineLayout, stageFlags vkShaderStageFlags, offset cUint32, size cUint32, pValues unsafe.Pointer) {
	C.vkCmdPushConstants(commandBuffer, layout, stageFlags, offset, size, pValues)
}
func vkCmdResetQueryPool(commandBuffer vkCommandBuffer, queryPool vkQueryPool, firstQuery cUint32, queryCount cUint32) {
	C.vkCmdResetQueryPool(commandBuffer, queryPool, firstQuery, queryCount)
}
func vkCmdSetDepthBias(commandBuffer vkCommandBuffer, depthBiasConstantFactor cFloat, depthBiasClamp cFloat, depthBiasSlopeFactor cFloat) {
	C.vkCmdSetDepthBias(commandBuffer, depthBiasConstantFactor, depthBiasClamp, depthBiasSlopeFactor)
}
func vkCmdSetScissor(commandBuffer vkCommandBuffer, firstScissor cUint32, scissorCount cUint32, pScissors *vkRect2D) {
	C.vkCmdSetScissor(commandBuffer, firstScissor, scissorCount, pScissors)
}
func vkCmdSetViewport(commandBuffer vkCommandBuffer, firstViewport cUint32, viewportCount cUint32, pViewports *vkViewport) {
	C.vkCmdSetViewport(commandBuffer, firstViewport, viewportCount, pViewports)
}
func vkCmdWriteTimestamp2(commandBuffer vkCommandBuffer, stage vkPipelineStageFlags2, queryPool vkQueryPool, query cUint32) {
	C.vkCmdWriteTimestamp2(commandBuffer, stage, queryPool, query)
}
func vkCreateBuffer(device vkDevice, pCreateInfo *vkBufferCreateInfo, pAllocator *vkAllocationCallbacks, pBuffer *vkBuffer) vkResult {
	return C.vkCreateBuffer(device, pCreateInfo, pAllocator, pBuffer)
}
func vkCreateCommandPool(device vkDevice, pCreateInfo *vkCommandPoolCreateInfo, pAllocator *vkAllocationCallbacks, pCommandPool *vkCommandPool) vkResult {
	return C.vkCreateCommandPool(device, pCreateInfo, pAllocator, pCommandPool)
}
func vkCreateComputePipelines(device vkDevice, pipelineCache vkPipelineCache, createInfoCount cUint32, pCreateInfos *vkComputePipelineCreateInfo, pAllocator *vkAllocationCallbacks, pPipelines *vkPipeline) vkResult {
	return C.vkCreateComputePipelines(device, pipelineCache, createInfoCount, pCreateInfos, pAllocator, pPipelines)
}
func vkCreateDescriptorPool(device vkDevice, pCreateInfo *vkDescriptorPoolCreateInfo, pAllocator *vkAllocationCallbacks, pDescriptorPool *vkDescriptorPool) vkResult {
	return C.vkCreateDescriptorPool(device, pCreateInfo, pAllocator, pDescriptorPool)
}
func vkCreateDescriptorSetLayout(device vkDevice, pCreateInfo *vkDescriptorSetLayoutCreateInfo, pAllocator *vkAllocationCallbacks, pSetLayout *vkDescriptorSetLayout) vkResult {
	return C.vkCreateDescriptorSetLayout(device, pCreateInfo, pAllocator, pSetLayout)
}
func vkCreateDevice(physicalDevice vkPhysicalDevice, pCreateInfo *vkDeviceCreateInfo, pAllocator *vkAllocationCallbacks, pDevice *vkDevice) vkResult {
	return C.vkCreateDevice(physicalDevice, pCreateInfo, pAllocator, pDevice)
}
func vkCreateFence(device vkDevice, pCreateInfo *vkFenceCreateInfo, pAllocator *vkAllocationCallbacks, pFence *vkFence) vkResult {
	return C.vkCreateFence(device, pCreateInfo, pAllocator, pFence)
}
func vkCreateGraphicsPipelines(device vkDevice, pipelineCache vkPipelineCache, createInfoCount cUint32, pCreateInfos *vkGraphicsPipelineCreateInfo, pAllocator *vkAllocationCallbacks, pPipelines *vkPipeline) vkResult {
	return C.vkCreateGraphicsPipelines(device, pipelineCache, createInfoCount, pCreateInfos, pAllocator, pPipelines)
}
func vkCreateImage(device vkDevice, pCreateInfo *vkImageCreateInfo, pAllocator *vkAllocationCallbacks, pImage *vkImage) vkResult {
	return C.vkCreateImage(device, pCreateInfo, pAllocator, pImage)
}
func vkCreateImageView(device vkDevice, pCreateInfo *vkImageViewCreateInfo, pAllocator *vkAllocationCallbacks, pView *vkImageView) vkResult {
	return C.vkCreateImageView(device, pCreateInfo, pAllocator, pView)
}
func vkCreateInstance(pCreateInfo *vkInstanceCreateInfo, pAllocator *vkAllocationCallbacks, pInstance *vkInstance) vkResult {
	return C.vkCreateInstance(pCreateInfo, pAllocator, pInstance)
}
func vkCreatePipelineLayout(device vkDevice, pCreateInfo *vkPipelineLayoutCreateInfo, pAllocator *vkAllocationCallbacks, pPipelineLayout *vkPipelineLayout) vkResult {
	return C.vkCreatePipelineLayout(device, pCreateInfo, pAllocator, pPipelineLayout)
}
func vkCreateQueryPool(device vkDevice, pCreateInfo *vkQueryPoolCreateInfo, pAllocator *vkAllocationCallbacks, pQueryPool *vkQueryPool) vkResult {
	return C.vkCreateQueryPool(device, pCreateInfo, pAllocator, pQueryPool)
}
func vkCreateSampler(device vkDevice, pCreateInfo *vkSamplerCreateInfo, pAllocator *vkAllocationCallbacks, pSampler *vkSampler) vkResult {
	return C.vkCreateSampler(device, pCreateInfo, pAllocator, pSampler)
}
func vkCreateSemaphore(device vkDevice, pCreateInfo *vkSemaphoreCreateInfo, pAllocator *vkAllocationCallbacks, pSemaphore *vkSemaphore) vkResult {
	return C.vkCreateSemaphore(device, pCreateInfo, pAllocator, pSemaphore)
}
func vkCreateShaderModule(device vkDevice, pCreateInfo *vkShaderModuleCreateInfo, pAllocator *vkAllocationCallbacks, pShaderModule *vkShaderModule) vkResult {
	return C.vkCreateShaderModule(device, pCreateInfo, pAllocator, pShaderModule)
}
func vkCreateSwapchainKHR(device vkDevice, pCreateInfo *vkSwapchainCreateInfoKHR, pAllocator *vkAllocationCallbacks, pSwapchain *vkSwapchainKHR) vkResult {
	return C.vkCreateSwapchainKHR(device, pCreateInfo, pAllocator, pSwapchain)
}
func vkDestroyBuffer(device vkDevice, buffer vkBuffer, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyBuffer(device, buffer, pAllocator)
}
func vkDestroyCommandPool(device vkDevice, commandPool vkCommandPool, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyCommandPool(device, commandPool, pAllocator)
}
func vkDestroyDescriptorPool(device vkDevice, descriptorPool vkDescriptorPool, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyDescriptorPool(device, descriptorPool, pAllocator)
}
func vkDestroyDescriptorSetLayout(device vkDevice, descriptorSetLayout vkDescriptorSetLayout, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyDescriptorSetLayout(device, descriptorSetLayout, pAllocator)
}
func vkDestroyDevice(device vkDevice, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyDevice(device, pAllocator)
}
func vkDestroyFence(device vkDevice, fence vkFence, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyFence(device, fence, pAllocator)
}
func vkDestroyImage(device vkDevice, image vkImage, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyImage(device, image, pAllocator)
}
func vkDestroyImageView(device vkDevice, imageView vkImageView, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyImageView(device, imageView, pAllocator)
}
func vkDestroyInstance(instance vkInstance, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyInstance(instance, pAllocator)
}
func vkDestroyPipeline(device vkDevice, pipeline vkPipeline, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyPipeline(device, pipeline, pAllocator)
}
func vkDestroyPipelineLayout(device vkDevice, pipelineLayout vkPipelineLayout, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyPipelineLayout(device, pipelineLayout, pAllocator)
}
func vkDestroyQueryPool(device vkDevice, queryPool vkQueryPool, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyQueryPool(device, queryPool, pAllocator)
}
func vkDestroySampler(device vkDevice, sampler vkSampler, pAllocator *vkAllocationCallbacks) {
	C.vkDestroySampler(device, sampler, pAllocator)
}
func vkDestroySemaphore(device vkDevice, semaphore vkSemaphore, pAllocator *vkAllocationCallbacks) {
	C.vkDestroySemaphore(device, semaphore, pAllocator)
}
func vkDestroyShaderModule(device vkDevice, shaderModule vkShaderModule, pAllocator *vkAllocationCallbacks) {
	C.vkDestroyShaderModule(device, shaderModule, pAllocator)
}
func vkDestroySurfaceKHR(instance vkInstance, surface vkSurfaceKHR, pAllocator *vkAllocationCallbacks) {
	C.vkDestroySurfaceKHR(instance, surface, pAllocator)
}
func vkDestroySwapchainKHR(device vkDevice, swapchain vkSwapchainKHR, pAllocator *vkAllocationCallbacks) {
	C.vkDestroySwapchainKHR(device, swapchain, pAllocator)
}
func vkDeviceWaitIdle(device vkDevice) vkResult {
	return C.vkDeviceWaitIdle(device)
}
func vkEndCommandBuffer(commandBuffer vkCommandBuffer) vkResult {
	return C.vkEndCommandBuffer(commandBuffer)
}
func vkEnumerateDeviceExtensionProperties(physicalDevice vkPhysicalDevice, pLayerName *cChar, pPropertyCount *cUint32, pProperties *vkExtensionProperties) vkResult {
	return C.vkEnumerateDeviceExtensionProperties(physicalDevice, pLayerName, pPropertyCount, pProperties)
}
func vkEnumeratePhysicalDevices(instance vkInstance, pPhysicalDeviceCount *cUint32, pPhysicalDevices *vkPhysicalDevice) vkResult {
	return C.vkEnumeratePhysicalDevices(instance, pPhysicalDeviceCount, pPhysicalDevices)
}
func vkFreeCommandBuffers(device vkDevice, commandPool vkCommandPool, commandBufferCount cUint32, pCommandBuffers *vkCommandBuffer) {
	C.vkFreeCommandBuffers(device, commandPool, commandBufferCount, pCommandBuffers)
}
func vkFreeMemory(device vkDevice, memory vkDeviceMemory, pAllocator *vkAllocationCallbacks) {
	C.vkFreeMemory(device, memory, pAllocator)
}
func vkGetBufferDeviceAddress(device vkDevice, pInfo *vkBufferDeviceAddressInfo) vkDeviceAddress {
	return C.vkGetBufferDeviceAddress(device, pInfo)
}
func vkGetBufferMemoryRequirements(device vkDevice, buffer vkBuffer, pMemoryRequirements *vkMemoryRequirements) {
	C.vkGetBufferMemoryRequirements(device, buffer, pMemoryRequirements)
}
func vkGetDeviceQueue(device vkDevice, queueFamilyIndex cUint32, queueIndex cUint32, pQueue *vkQueue) {
	C.vkGetDeviceQueue(device, queueFamilyIndex, queueIndex, pQueue)
}
func vkGetImageMemoryRequirements(device vkDevice, image vkImage, pMemoryRequirements *vkMemoryRequirements) {
	C.vkGetImageMemoryRequirements(device, image, pMemoryRequirements)
}
func vkGetPhysicalDeviceFeatures2(physicalDevice vkPhysicalDevice, pFeatures *vkPhysicalDeviceFeatures2) {
	C.vkGetPhysicalDeviceFeatures2(physicalDevice, pFeatures)
}
func vkGetPhysicalDeviceMemoryProperties(physicalDevice vkPhysicalDevice, pMemoryProperties *vkPhysicalDeviceMemoryProperties) {
	C.vkGetPhysicalDeviceMemoryProperties(physicalDevice, pMemoryProperties)
}
func vkGetPhysicalDeviceProperties(physicalDevice vkPhysicalDevice, pProperties *vkPhysicalDeviceProperties) {
	C.vkGetPhysicalDeviceProperties(physicalDevice, pProperties)
}
func vkGetPhysicalDeviceProperties2(physicalDevice vkPhysicalDevice, pProperties *vkPhysicalDeviceProperties2) {
	C.vkGetPhysicalDeviceProperties2(physicalDevice, pProperties)
}
func vkGetPhysicalDeviceQueueFamilyProperties(physicalDevice vkPhysicalDevice, pQueueFamilyPropertyCount *cUint32, pQueueFamilyProperties *vkQueueFamilyProperties) {
	C.vkGetPhysicalDeviceQueueFamilyProperties(physicalDevice, pQueueFamilyPropertyCount, pQueueFamilyProperties)
}
func vkGetPhysicalDeviceSurfaceCapabilitiesKHR(physicalDevice vkPhysicalDevice, surface vkSurfaceKHR, pSurfaceCapabilities *vkSurfaceCapabilitiesKHR) vkResult {
	return C.vkGetPhysicalDeviceSurfaceCapabilitiesKHR(physicalDevice, surface, pSurfaceCapabilities)
}
func vkGetPhysicalDeviceSurfaceFormatsKHR(physicalDevice vkPhysicalDevice, surface vkSurfaceKHR, pSurfaceFormatCount *cUint32, pSurfaceFormats *vkSurfaceFormatKHR) vkResult {
	return C.vkGetPhysicalDeviceSurfaceFormatsKHR(physicalDevice, surface, pSurfaceFormatCount, pSurfaceFormats)
}
func vkGetPhysicalDeviceSurfacePresentModesKHR(physicalDevice vkPhysicalDevice, surface vkSurfaceKHR, pPresentModeCount *cUint32, pPresentModes *vkPresentModeKHR) vkResult {
	return C.vkGetPhysicalDeviceSurfacePresentModesKHR(physicalDevice, surface, pPresentModeCount, pPresentModes)
}
func vkGetQueryPoolResults(device vkDevice, queryPool vkQueryPool, firstQuery cUint32, queryCount cUint32, dataSize cSize, pData unsafe.Pointer, stride vkDeviceSize, flags vkQueryResultFlags) vkResult {
	return C.vkGetQueryPoolResults(device, queryPool, firstQuery, queryCount, dataSize, pData, stride, flags)
}
func vkGetSwapchainImagesKHR(device vkDevice, swapchain vkSwapchainKHR, pSwapchainImageCount *cUint32, pSwapchainImages *vkImage) vkResult {
	return C.vkGetSwapchainImagesKHR(device, swapchain, pSwapchainImageCount, pSwapchainImages)
}
func vkMapMemory(device vkDevice, memory vkDeviceMemory, offset vkDeviceSize, size vkDeviceSize, flags vkMemoryMapFlags, ppData *unsafe.Pointer) vkResult {
	return C.vkMapMemory(device, memory, offset, size, flags, ppData)
}
func vkQueuePresentKHR(queue vkQueue, pPresentInfo *vkPresentInfoKHR) vkResult {
	return C.vkQueuePresentKHR(queue, pPresentInfo)
}
func vkQueueSubmit2(queue vkQueue, submitCount cUint32, pSubmits *vkSubmitInfo2, fence vkFence) vkResult {
	return C.vkQueueSubmit2(queue, submitCount, pSubmits, fence)
}
func vkQueueWaitIdle(queue vkQueue) vkResult {
	return C.vkQueueWaitIdle(queue)
}
func vkUpdateDescriptorSets(device vkDevice, descriptorWriteCount cUint32, pDescriptorWrites *vkWriteDescriptorSet, descriptorCopyCount cUint32, pDescriptorCopies *vkCopyDescriptorSet) {
	C.vkUpdateDescriptorSets(device, descriptorWriteCount, pDescriptorWrites, descriptorCopyCount, pDescriptorCopies)
}
func vkWaitForFences(device vkDevice, fenceCount cUint32, pFences *vkFence, waitAll vkBool32, timeout cUint64) vkResult {
	return C.vkWaitForFences(device, fenceCount, pFences, waitAll, timeout)
}
