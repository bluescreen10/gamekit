//go:build darwin && cgo

package metal

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=26.0
#cgo LDFLAGS: -framework Metal -framework Foundation -framework QuartzCore -framework Cocoa
#include "objc.h"
*/
import "C"

import (
	"runtime"
	"runtime/cgo"
	"unsafe"

	"github.com/bluescreen10/gamekit/strings"
)

// This file is the package's only contact with C: the rest of the package sends
// Objective-C messages and calls C through the functions declared here, and never
// imports "C". It follows one naming convention, so that a C name always gives its Go
// name:
//
//   - A message is named for its receiver and its selector: the receiver's type with
//     its framework prefix (MTL, MTL4, CA, NS) dropped and the first letter lowered,
//     then the selector's first part with the first letter raised. [layer
//     setContentsScale:s] is metalLayerSetContentsScale(layer, s), and [device
//     newBufferWithLength:n options:o] is deviceNewBufferWithLength(device, n, o). When a
//     receiver takes several messages that share a first part, each adds the part that
//     tells it apart: computeCommandEncoderCopyFromBufferToTexture. Messages that many
//     unrelated types take are named for object (objectRelease, objectSetLabel), class
//     methods for the class (metalLayerLayer is [CAMetalLayer layer]), and the
//     collections for array, dictionary, string and data.
//   - A C function is its name in Go camel case, with its framework prefix lowered:
//     MTLCreateSystemDefaultDevice is mtlCreateSystemDefaultDevice, objc_getClass
//     objcGetClass.
//   - A constant or enumerator is its C name with its framework prefix lowered:
//     MTLPixelFormatRGBA8Unorm is mtlPixelFormatRGBA8Unorm, MTL4BlendStateEnabled
//     mtl4BlendStateEnabled.
//   - Names use Go's initialisms (GPU, ID, RGB, UTF8): gpuResourceID is GPUResourceID.
//   - Parameters and results are Go types: an object is an id, an NSUInteger or enum a
//     uint, a BOOL a bool, and a struct its fields.
//
// Underneath, every message goes through a typed objc_msgSend shim in objc.h, one per
// method signature. When cgo bindings replace cgo, these declarations are what changes.

// id is an Objective-C object. It is never Go memory, and the package retains and
// releases it as Objective-C without ARC would.
type id uintptr

// sel is a registered selector.
type sel = C.gk_sel

func o[T ~uintptr](x T) C.gk_obj {
	return C.gk_obj(x)
}

// Constants.
const (
	mtl4BlendStateDisabled                  = C.MTL4BlendStateDisabled
	mtl4BlendStateEnabled                   = C.MTL4BlendStateEnabled
	mtl4CounterHeapTypeTimestamp            = C.MTL4CounterHeapTypeTimestamp
	mtl4ShaderReflectionBindingInfo         = C.MTL4ShaderReflectionBindingInfo
	mtl4TimestampGranularityPrecise         = C.MTL4TimestampGranularityPrecise
	mtlArgumentBuffersTier2                 = C.MTLArgumentBuffersTier2
	mtlBindingTypeBuffer                    = C.MTLBindingTypeBuffer
	mtlBlendFactorDestinationAlpha          = C.MTLBlendFactorDestinationAlpha
	mtlBlendFactorOne                       = C.MTLBlendFactorOne
	mtlBlendFactorOneMinusDestinationAlpha  = C.MTLBlendFactorOneMinusDestinationAlpha
	mtlBlendFactorOneMinusSourceAlpha       = C.MTLBlendFactorOneMinusSourceAlpha
	mtlBlendFactorSourceAlpha               = C.MTLBlendFactorSourceAlpha
	mtlBlendFactorZero                      = C.MTLBlendFactorZero
	mtlCompareFunctionAlways                = C.MTLCompareFunctionAlways
	mtlCullModeBack                         = C.MTLCullModeBack
	mtlCullModeFront                        = C.MTLCullModeFront
	mtlCullModeNone                         = C.MTLCullModeNone
	mtlDataTypeBool                         = C.MTLDataTypeBool
	mtlDataTypeFloat                        = C.MTLDataTypeFloat
	mtlDataTypeInt                          = C.MTLDataTypeInt
	mtlDataTypeUInt                         = C.MTLDataTypeUInt
	mtlGPUFamilyApple1                      = C.MTLGPUFamilyApple1
	mtlGPUFamilyMetal4                      = C.MTLGPUFamilyMetal4
	mtlIndexTypeUInt16                      = C.MTLIndexTypeUInt16
	mtlIndexTypeUInt32                      = C.MTLIndexTypeUInt32
	mtlLanguageVersion3_0                   = C.MTLLanguageVersion3_0
	mtlLoadActionClear                      = C.MTLLoadActionClear
	mtlLoadActionDontCare                   = C.MTLLoadActionDontCare
	mtlLoadActionLoad                       = C.MTLLoadActionLoad
	mtlMultisampleDepthResolveFilterSample0 = C.MTLMultisampleDepthResolveFilterSample0
	mtlPrimitiveTypeLine                    = C.MTLPrimitiveTypeLine
	mtlPrimitiveTypePoint                   = C.MTLPrimitiveTypePoint
	mtlPrimitiveTypeTriangle                = C.MTLPrimitiveTypeTriangle
	mtlPrimitiveTypeTriangleStrip           = C.MTLPrimitiveTypeTriangleStrip
	mtlRenderStageFragment                  = C.MTLRenderStageFragment
	mtlRenderStageVertex                    = C.MTLRenderStageVertex
	mtlResourceStorageModePrivate           = C.MTLResourceStorageModePrivate
	mtlResourceStorageModeShared            = C.MTLResourceStorageModeShared
	mtlSamplerAddressModeClampToEdge        = C.MTLSamplerAddressModeClampToEdge
	mtlSamplerAddressModeMirrorRepeat       = C.MTLSamplerAddressModeMirrorRepeat
	mtlSamplerAddressModeRepeat             = C.MTLSamplerAddressModeRepeat
	mtlSamplerMinMagFilterLinear            = C.MTLSamplerMinMagFilterLinear
	mtlSamplerMinMagFilterNearest           = C.MTLSamplerMinMagFilterNearest
	mtlSamplerMipFilterLinear               = C.MTLSamplerMipFilterLinear
	mtlSamplerMipFilterNearest              = C.MTLSamplerMipFilterNearest
	mtlStageAccelerationStructure           = C.MTLStageAccelerationStructure
	mtlStageBlit                            = C.MTLStageBlit
	mtlStageDispatch                        = C.MTLStageDispatch
	mtlStageFragment                        = C.MTLStageFragment
	mtlStageMesh                            = C.MTLStageMesh
	mtlStageObject                          = C.MTLStageObject
	mtlStageTile                            = C.MTLStageTile
	mtlStageVertex                          = C.MTLStageVertex
	mtlStorageModeMemoryless                = C.MTLStorageModeMemoryless
	mtlStorageModePrivate                   = C.MTLStorageModePrivate
	mtlStoreActionDontCare                  = C.MTLStoreActionDontCare
	mtlStoreActionMultisampleResolve        = C.MTLStoreActionMultisampleResolve
	mtlStoreActionStore                     = C.MTLStoreActionStore
	mtlStoreActionUnknown                   = C.MTLStoreActionUnknown
	mtlTextureType2D                        = C.MTLTextureType2D
	mtlTextureType2DArray                   = C.MTLTextureType2DArray
	mtlTextureType2DMultisample             = C.MTLTextureType2DMultisample
	mtlTextureType3D                        = C.MTLTextureType3D
	mtlTextureTypeCube                      = C.MTLTextureTypeCube
	mtlTextureTypeCubeArray                 = C.MTLTextureTypeCubeArray
	mtlTextureUsagePixelFormatView          = C.MTLTextureUsagePixelFormatView
	mtlTextureUsageRenderTarget             = C.MTLTextureUsageRenderTarget
	mtlTextureUsageShaderRead               = C.MTLTextureUsageShaderRead
	mtlTextureUsageShaderWrite              = C.MTLTextureUsageShaderWrite
	mtlWindingClockwise                     = C.MTLWindingClockwise
	mtlWindingCounterClockwise              = C.MTLWindingCounterClockwise
	nsUTF8StringEncoding                    = C.NSUTF8StringEncoding

	mtlPixelFormatInvalid               = C.MTLPixelFormatInvalid
	mtlPixelFormatR8Unorm               = C.MTLPixelFormatR8Unorm
	mtlPixelFormatRG8Unorm              = C.MTLPixelFormatRG8Unorm
	mtlPixelFormatRGBA8Unorm            = C.MTLPixelFormatRGBA8Unorm
	mtlPixelFormatBGRA8Unorm            = C.MTLPixelFormatBGRA8Unorm
	mtlPixelFormatRGBA8Unorm_sRGB       = C.MTLPixelFormatRGBA8Unorm_sRGB
	mtlPixelFormatBGRA8Unorm_sRGB       = C.MTLPixelFormatBGRA8Unorm_sRGB
	mtlPixelFormatR8Snorm               = C.MTLPixelFormatR8Snorm
	mtlPixelFormatRG8Snorm              = C.MTLPixelFormatRG8Snorm
	mtlPixelFormatRGBA8Snorm            = C.MTLPixelFormatRGBA8Snorm
	mtlPixelFormatR8Uint                = C.MTLPixelFormatR8Uint
	mtlPixelFormatRG8Uint               = C.MTLPixelFormatRG8Uint
	mtlPixelFormatRGBA8Uint             = C.MTLPixelFormatRGBA8Uint
	mtlPixelFormatR8Sint                = C.MTLPixelFormatR8Sint
	mtlPixelFormatRG8Sint               = C.MTLPixelFormatRG8Sint
	mtlPixelFormatRGBA8Sint             = C.MTLPixelFormatRGBA8Sint
	mtlPixelFormatR16Unorm              = C.MTLPixelFormatR16Unorm
	mtlPixelFormatRG16Unorm             = C.MTLPixelFormatRG16Unorm
	mtlPixelFormatRGBA16Unorm           = C.MTLPixelFormatRGBA16Unorm
	mtlPixelFormatR16Snorm              = C.MTLPixelFormatR16Snorm
	mtlPixelFormatRG16Snorm             = C.MTLPixelFormatRG16Snorm
	mtlPixelFormatRGBA16Snorm           = C.MTLPixelFormatRGBA16Snorm
	mtlPixelFormatR16Uint               = C.MTLPixelFormatR16Uint
	mtlPixelFormatRG16Uint              = C.MTLPixelFormatRG16Uint
	mtlPixelFormatRGBA16Uint            = C.MTLPixelFormatRGBA16Uint
	mtlPixelFormatR16Sint               = C.MTLPixelFormatR16Sint
	mtlPixelFormatRG16Sint              = C.MTLPixelFormatRG16Sint
	mtlPixelFormatRGBA16Sint            = C.MTLPixelFormatRGBA16Sint
	mtlPixelFormatR16Float              = C.MTLPixelFormatR16Float
	mtlPixelFormatRG16Float             = C.MTLPixelFormatRG16Float
	mtlPixelFormatRGBA16Float           = C.MTLPixelFormatRGBA16Float
	mtlPixelFormatR32Uint               = C.MTLPixelFormatR32Uint
	mtlPixelFormatRG32Uint              = C.MTLPixelFormatRG32Uint
	mtlPixelFormatRGBA32Uint            = C.MTLPixelFormatRGBA32Uint
	mtlPixelFormatR32Sint               = C.MTLPixelFormatR32Sint
	mtlPixelFormatRG32Sint              = C.MTLPixelFormatRG32Sint
	mtlPixelFormatRGBA32Sint            = C.MTLPixelFormatRGBA32Sint
	mtlPixelFormatR32Float              = C.MTLPixelFormatR32Float
	mtlPixelFormatRG32Float             = C.MTLPixelFormatRG32Float
	mtlPixelFormatRGBA32Float           = C.MTLPixelFormatRGBA32Float
	mtlPixelFormatRGB10A2Unorm          = C.MTLPixelFormatRGB10A2Unorm
	mtlPixelFormatRGB10A2Uint           = C.MTLPixelFormatRGB10A2Uint
	mtlPixelFormatRG11B10Float          = C.MTLPixelFormatRG11B10Float
	mtlPixelFormatRGB9E5Float           = C.MTLPixelFormatRGB9E5Float
	mtlPixelFormatDepth16Unorm          = C.MTLPixelFormatDepth16Unorm
	mtlPixelFormatDepth32Float          = C.MTLPixelFormatDepth32Float
	mtlPixelFormatDepth24Unorm_Stencil8 = C.MTLPixelFormatDepth24Unorm_Stencil8
	mtlPixelFormatDepth32Float_Stencil8 = C.MTLPixelFormatDepth32Float_Stencil8
	mtlPixelFormatStencil8              = C.MTLPixelFormatStencil8
	mtlPixelFormatBC1_RGBA              = C.MTLPixelFormatBC1_RGBA
	mtlPixelFormatBC1_RGBA_sRGB         = C.MTLPixelFormatBC1_RGBA_sRGB
	mtlPixelFormatBC3_RGBA              = C.MTLPixelFormatBC3_RGBA
	mtlPixelFormatBC3_RGBA_sRGB         = C.MTLPixelFormatBC3_RGBA_sRGB
	mtlPixelFormatBC4_RUnorm            = C.MTLPixelFormatBC4_RUnorm
	mtlPixelFormatBC4_RSnorm            = C.MTLPixelFormatBC4_RSnorm
	mtlPixelFormatBC5_RGUnorm           = C.MTLPixelFormatBC5_RGUnorm
	mtlPixelFormatBC5_RGSnorm           = C.MTLPixelFormatBC5_RGSnorm
	mtlPixelFormatBC6H_RGBFloat         = C.MTLPixelFormatBC6H_RGBFloat
	mtlPixelFormatBC6H_RGBUfloat        = C.MTLPixelFormatBC6H_RGBUfloat
	mtlPixelFormatBC7_RGBAUnorm         = C.MTLPixelFormatBC7_RGBAUnorm
	mtlPixelFormatBC7_RGBAUnorm_sRGB    = C.MTLPixelFormatBC7_RGBAUnorm_sRGB
	mtlPixelFormatASTC_4x4_LDR          = C.MTLPixelFormatASTC_4x4_LDR
	mtlPixelFormatASTC_4x4_sRGB         = C.MTLPixelFormatASTC_4x4_sRGB
	mtlPixelFormatASTC_5x5_LDR          = C.MTLPixelFormatASTC_5x5_LDR
	mtlPixelFormatASTC_5x5_sRGB         = C.MTLPixelFormatASTC_5x5_sRGB
	mtlPixelFormatASTC_6x6_LDR          = C.MTLPixelFormatASTC_6x6_LDR
	mtlPixelFormatASTC_6x6_sRGB         = C.MTLPixelFormatASTC_6x6_sRGB
	mtlPixelFormatASTC_8x8_LDR          = C.MTLPixelFormatASTC_8x8_LDR
	mtlPixelFormatASTC_8x8_sRGB         = C.MTLPixelFormatASTC_8x8_sRGB
	mtlPixelFormatASTC_10x10_LDR        = C.MTLPixelFormatASTC_10x10_LDR
	mtlPixelFormatASTC_10x10_sRGB       = C.MTLPixelFormatASTC_10x10_sRGB
	mtlPixelFormatASTC_12x12_LDR        = C.MTLPixelFormatASTC_12x12_LDR
	mtlPixelFormatASTC_12x12_sRGB       = C.MTLPixelFormatASTC_12x12_sRGB
)

func b2u(v bool) C.gk_u {
	if v {
		return 1
	}
	return 0
}

// C functions.

func mtlCreateSystemDefaultDevice() device {
	return device(C.gk_device())
}

func objcGetClass(name string) id {
	return id(uintptr(unsafe.Pointer(C.objc_getClass((*C.char)(unsafe.Pointer(strings.CString(name)))))))
}

func selRegisterName(name string) sel {
	return sel(uintptr(unsafe.Pointer(C.sel_registerName((*C.char)(unsafe.Pointer(strings.CString(name)))))))
}

func objcAutoreleasePoolPush() unsafe.Pointer {
	return C.objc_autoreleasePoolPush()
}
func objcAutoreleasePoolPop(pool unsafe.Pointer) {
	C.objc_autoreleasePoolPop(pool)
}

// dispatchDataCreate copies n bytes at p into new dispatch data, which the caller
// releases.
func dispatchDataCreate(p unsafe.Pointer, n int) id {
	return id(C.gk_dispatch_data(p, C.size_t(n)))
}

func machAbsoluteTime() uint64 {
	return uint64(C.mach_absolute_time())
}

func machTimebaseInfo() (numer, denom uint32) {
	var info C.mach_timebase_info_data_t
	C.mach_timebase_info(&info)
	return uint32(info.numer), uint32(info.denom)
}

func caCurrentMediaTime() float64 {
	return float64(C.CACurrentMediaTime())
}

// The feedback handler of a commit, which objc.h's block hands to Go.
//
//export gkMetalFeedback
func gkMetalFeedback(ctx C.uintptr_t, feedback C.uintptr_t) {
	h := cgo.Handle(ctx)
	handler := h.Value().(func(id))
	h.Delete()
	handler(id(feedback))
}

// Objects in general.

func objectNew(class id) id {
	return id(C.gk_o(o(class), selNew))
}
func objectAlloc(class id) id {
	return id(C.gk_o(o(class), selAlloc))
}
func objectInit(x id) id {
	return id(C.gk_o(o(x), selInit))
}
func objectRelease(x id) {
	C.gk_v(o(x), selRelease)
}
func objectRetain(x id) id {
	return id(C.gk_o(o(x), selRetain))
}
func objectAutorelease(x id) id {
	return id(C.gk_o(o(x), selAutorelease))
}
func objectCopy(x id) id {
	return id(C.gk_o(o(x), selCopy))
}
func objectIsKindOfClass(x, class id) bool {
	return C.gk_b_o(o(x), selIsKindOfClass, o(class)) != 0
}
func objectSetLabel(x, label id) {
	C.gk_v_o(o(x), selSetLabel, o(label))
}

// Foundation.

func stringInitWithBytes(s id, p unsafe.Pointer, n uint, encoding uint) id {
	return id(C.gk_o_puu(o(s), selInitWithBytes, p, C.gk_u(n), C.gk_u(encoding)))
}
func stringUTF8String(s id) unsafe.Pointer {
	return unsafe.Pointer(C.gk_str(o(s), selUTF8String))
}
func errorLocalizedDescription(e id) id {
	return id(C.gk_o(o(e), selLocalizedDescription))
}
func arrayCount(a id) uint {
	return uint(C.gk_u_(o(a), selCount))
}
func arrayObjectAtIndex(a id, i uint) id {
	return id(C.gk_o_u(o(a), selObjectAtIndex, C.gk_u(i)))
}
func arrayObjectAtIndexedSubscript(a id, i uint) id {
	return id(C.gk_o_u(o(a), selObjectAtIndexedSubscript, C.gk_u(i)))
}
func dictionaryAllValues(d id) id {
	return id(C.gk_o(o(d), selAllValues))
}
func dataBytes(d id) unsafe.Pointer {
	return C.gk_p(o(d), selBytes)
}
func dataLength(d id) uint {
	return uint(C.gk_u_(o(d), selLength))
}

// MTLDevice.

func deviceArgumentBuffersSupport(d device) uint {
	return uint(C.gk_u_(o(d), selArgumentBuffersSupport))
}
func deviceHasUnifiedMemory(d device) bool {
	return C.gk_b(o(d), selHasUnifiedMemory) != 0
}
func deviceSupportsFamily(d device, family uint) bool {
	return C.gk_b_u(o(d), selSupportsFamily, C.gk_u(family)) != 0
}
func deviceMaxBufferLength(d device) uint {
	return uint(C.gk_u_(o(d), selMaxBufferLength))
}
func deviceMaxArgumentBufferSamplerCount(d device) uint {
	return uint(C.gk_u_(o(d), selMaxArgumentBufferSamplerCount))
}
func deviceNewMTL4CommandQueue(d device) queue {
	return queue(C.gk_o(o(d), selNewMTL4CommandQueue))
}
func deviceNewSharedEvent(d device) id {
	return id(C.gk_o(o(d), selNewSharedEvent))
}
func deviceNewFence(d device) id {
	return id(C.gk_o(o(d), selNewFence))
}
func deviceNewCommandBuffer(d device) commandBuffer {
	return commandBuffer(C.gk_o(o(d), selNewCommandBuffer))
}
func deviceNewCompilerWithDescriptor(d device, desc compilerDescriptor, err *id) compiler {
	return compiler(C.gk_o_op(o(d), selNewCompilerWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewResidencySetWithDescriptor(d device, desc residencySetDescriptor, err *id) id {
	return id(C.gk_o_op(o(d), selNewResidencySetWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewCommandAllocatorWithDescriptor(d device, desc commandAllocatorDescriptor, err *id) allocator {
	return allocator(C.gk_o_op(o(d), selNewCommandAllocatorWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewArgumentTableWithDescriptor(d device, desc argumentTableDescriptor, err *id) argumentTable {
	return argumentTable(C.gk_o_op(o(d), selNewArgumentTableWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewCounterHeapWithDescriptor(d device, desc counterHeapDescriptor, err *id) id {
	return id(C.gk_o_op(o(d), selNewCounterHeapWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewBufferWithLength(d device, length, options uint) buffer {
	return buffer(C.gk_o_uu(o(d), selNewBufferWithLength, C.gk_u(length), C.gk_u(options)))
}
func deviceNewTextureWithDescriptor(d device, desc textureDescriptor) texture {
	return texture(C.gk_o_o(o(d), selNewTextureWithDescriptor, o(desc)))
}
func deviceNewSamplerStateWithDescriptor(d device, desc samplerDescriptor) sampler {
	return sampler(C.gk_o_o(o(d), selNewSamplerStateWithDescriptor, o(desc)))
}
func deviceNewDepthStencilStateWithDescriptor(d device, desc depthStencilDescriptor) id {
	return id(C.gk_o_o(o(d), selNewDepthStencilStateWithDescriptor, o(desc)))
}
func deviceNewLibraryWithData(d device, data id, err *id) id {
	return id(C.gk_o_op(o(d), selNewLibraryWithData, o(data), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewLibraryWithSource(d device, source, options id, err *id) id {
	return id(C.gk_o_oop(o(d), selNewLibraryWithSource, o(source), o(options), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceSampleTimestamps(d device, cpu, gpu *uint64) {
	C.gk_v_pp(o(d), selSampleTimestamps, unsafe.Pointer(cpu), unsafe.Pointer(gpu))
}

// MTL4CommandQueue, MTLResidencySet, MTLSharedEvent.

func commandQueueAddResidencySet(q queue, set residencySet) {
	C.gk_v_o(o(q), selAddResidencySet, o(set))
}
func commandQueueCommit(q queue, buffers *commandBuffer, count uint, options id) {
	C.gk_v_puo(o(q), selCommitCountOptions, (*C.gk_obj)(unsafe.Pointer(buffers)), C.gk_u(count), o(options))
}
func commandQueueSignalDrawable(q queue, drawable id) {
	C.gk_v_o(o(q), selSignalDrawable, o(drawable))
}
func commandQueueSignalEvent(q queue, event id, value uint64) {
	C.gk_v_ou(o(q), selSignalEventValue, o(event), C.gk_u(value))
}
func residencySetAddAllocation(s residencySet, allocation id) {
	C.gk_v_o(o(s), selAddAllocation, o(allocation))
}
func residencySetRemoveAllocation(s residencySet, allocation id) {
	C.gk_v_o(o(s), selRemoveAllocation, o(allocation))
}
func residencySetCommit(s residencySet) {
	C.gk_v(o(s), selCommit)
}
func residencySetRequestResidency(s residencySet) {
	C.gk_v(o(s), selRequestResidency)
}
func sharedEventWaitUntilSignaledValue(e id, value uint64, timeoutMS uint) bool {
	return C.gk_b_uu(o(e), selWaitUntilSignaledValue, C.gk_u(value), C.gk_u(timeoutMS)) != 0
}

// MTL4ArgumentTable and its descriptor.

func argumentTableDescriptorSetMaxBufferBindCount(d argumentTableDescriptor, n uint) {
	C.gk_v_u(o(d), selSetMaxBufferBindCount, C.gk_u(n))
}
func argumentTableSetAddress(t argumentTable, address uint64, index uint) {
	C.gk_v_uu(o(t), selSetAddressAtIndex, C.gk_u(address), C.gk_u(index))
}

// MTLBuffer, MTLTexture, MTLSamplerState.

func bufferContents(b buffer) unsafe.Pointer {
	return C.gk_p(o(b), selContents)
}
func bufferGPUAddress(b buffer) uint64 {
	return uint64(C.gk_u_(o(b), selGPUAddress))
}
func textureGPUResourceID(t texture) uint64 {
	return uint64(C.gk_rid(o(t), selGPUResourceID))
}
func samplerStateGPUResourceID(s sampler) uint64 {
	return uint64(C.gk_rid(o(s), selGPUResourceID))
}
func textureWidth(t texture) uint {
	return uint(C.gk_u_(o(t), selWidth))
}
func textureHeight(t texture) uint {
	return uint(C.gk_u_(o(t), selHeight))
}
func texturePixelFormat(t texture) uint {
	return uint(C.gk_u_(o(t), selPixelFormat))
}
func textureNewTextureViewWithPixelFormat(t texture, format, textureType, firstLevel, levels, firstSlice, slices uint) texture {
	return texture(C.gk_o_uurr(o(t), selNewTextureViewWithPixelFormat, C.gk_u(format), C.gk_u(textureType),
		C.NSRange{location: C.NSUInteger(firstLevel), length: C.NSUInteger(levels)},
		C.NSRange{location: C.NSUInteger(firstSlice), length: C.NSUInteger(slices)}))
}

// MTLTextureDescriptor.

func textureDescriptorSetTextureType(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetTextureType, C.gk_u(v))
}
func textureDescriptorSetWidth(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetWidth, C.gk_u(v))
}
func textureDescriptorSetHeight(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetHeight, C.gk_u(v))
}
func textureDescriptorSetDepth(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetDepth, C.gk_u(v))
}
func textureDescriptorSetArrayLength(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetArrayLength, C.gk_u(v))
}
func textureDescriptorSetMipmapLevelCount(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetMipmapLevelCount, C.gk_u(v))
}
func textureDescriptorSetPixelFormat(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetPixelFormat, C.gk_u(v))
}
func textureDescriptorSetSampleCount(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetSampleCount, C.gk_u(v))
}
func textureDescriptorSetStorageMode(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetStorageMode, C.gk_u(v))
}
func textureDescriptorSetUsage(d textureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetUsage, C.gk_u(v))
}

// MTLSamplerDescriptor.

func samplerDescriptorSetMinFilter(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetMinFilter, C.gk_u(v))
}
func samplerDescriptorSetMagFilter(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetMagFilter, C.gk_u(v))
}
func samplerDescriptorSetMipFilter(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetMipFilter, C.gk_u(v))
}
func samplerDescriptorSetSAddressMode(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetSAddressMode, C.gk_u(v))
}
func samplerDescriptorSetTAddressMode(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetTAddressMode, C.gk_u(v))
}
func samplerDescriptorSetRAddressMode(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetRAddressMode, C.gk_u(v))
}
func samplerDescriptorSetCompareFunction(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetCompareFunction, C.gk_u(v))
}
func samplerDescriptorSetMaxAnisotropy(d samplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetMaxAnisotropy, C.gk_u(v))
}
func samplerDescriptorSetSupportArgumentBuffers(d samplerDescriptor, v bool) {
	C.gk_v_u(o(d), selSetSupportArgumentBuffers, b2u(v))
}

// Shaders: MTLCompileOptions, MTLLibrary, MTLFunction, MTLFunctionConstant, and the
// MTL4 function descriptors.

func compileOptionsSetLanguageVersion(c id, v uint) {
	C.gk_v_u(o(c), selSetLanguageVersion, C.gk_u(v))
}
func libraryNewFunctionWithName(l, name id) id {
	return id(C.gk_o_o(o(l), selNewFunctionWithName, o(name)))
}
func functionFunctionConstantsDictionary(f id) id {
	return id(C.gk_o(o(f), selFunctionConstantsDictionary))
}
func functionConstantType(c id) uint {
	return uint(C.gk_u_(o(c), selType))
}
func functionConstantIndex(c id) uint {
	return uint(C.gk_u_(o(c), selIndex))
}
func functionConstantName(c id) id {
	return id(C.gk_o(o(c), selName))
}
func libraryFunctionDescriptorSetLibrary(d libraryFunctionDescriptor, l id) {
	C.gk_v_o(o(d), selSetLibrary, o(l))
}
func libraryFunctionDescriptorSetName(d libraryFunctionDescriptor, name id) {
	C.gk_v_o(o(d), selSetName, o(name))
}
func functionConstantValuesSetConstantValue(v id, value unsafe.Pointer, dataType, index uint) {
	C.gk_v_puu(o(v), selSetConstantValue, value, C.gk_u(dataType), C.gk_u(index))
}
func specializedFunctionDescriptorSetFunctionDescriptor(d specializedFunctionDescriptor, f libraryFunctionDescriptor) {
	C.gk_v_o(o(d), selSetFunctionDescriptor, o(f))
}
func specializedFunctionDescriptorSetConstantValues(d specializedFunctionDescriptor, v id) {
	C.gk_v_o(o(d), selSetConstantValues, o(v))
}

// Pipelines: MTL4PipelineOptions, the pipeline descriptors, MTL4Compiler, the pipeline
// states and their reflection.

func pipelineOptionsSetShaderReflection(p id, v uint) {
	C.gk_v_u(o(p), selSetShaderReflection, C.gk_u(v))
}
func computePipelineDescriptorSetComputeFunctionDescriptor(d computePipelineDescriptor, f specializedFunctionDescriptor) {
	C.gk_v_o(o(d), selSetComputeFunctionDescriptor, o(f))
}
func computePipelineDescriptorSetOptions(d computePipelineDescriptor, p id) {
	C.gk_v_o(o(d), selSetOptions, o(p))
}
func renderPipelineDescriptorSetVertexFunctionDescriptor(d renderPipelineDescriptor, f specializedFunctionDescriptor) {
	C.gk_v_o(o(d), selSetVertexFunctionDescriptor, o(f))
}
func renderPipelineDescriptorSetFragmentFunctionDescriptor(d renderPipelineDescriptor, f specializedFunctionDescriptor) {
	C.gk_v_o(o(d), selSetFragmentFunctionDescriptor, o(f))
}
func renderPipelineDescriptorSetRasterSampleCount(d renderPipelineDescriptor, v uint) {
	C.gk_v_u(o(d), selSetRasterSampleCount, C.gk_u(v))
}
func renderPipelineDescriptorSetOptions(d renderPipelineDescriptor, p id) {
	C.gk_v_o(o(d), selSetOptions, o(p))
}
func renderPipelineDescriptorColorAttachments(d renderPipelineDescriptor) id {
	return id(C.gk_o(o(d), selColorAttachments))
}
func renderPipelineColorAttachmentDescriptorSetPixelFormat(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetPixelFormat, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetWriteMask(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetWriteMask, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetBlendingState(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetBlendingState, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetSourceRGBBlendFactor(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetSourceRGBBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetDestinationRGBBlendFactor(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetDestinationRGBBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetRGBBlendOperation(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetRgbBlendOperation, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetSourceAlphaBlendFactor(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetSourceAlphaBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetDestinationAlphaBlendFactor(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetDestinationAlphaBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetAlphaBlendOperation(a renderPipelineColorAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetAlphaBlendOperation, C.gk_u(v))
}
func compilerNewComputePipelineStateWithDescriptor(c compiler, d computePipelineDescriptor, options id, err *id) id {
	return id(C.gk_o_oop(o(c), selNewComputePipelineStateWithDescriptor, o(d), o(options), (*C.gk_obj)(unsafe.Pointer(err))))
}
func compilerNewRenderPipelineStateWithDescriptor(c compiler, d renderPipelineDescriptor, options id, err *id) id {
	return id(C.gk_o_oop(o(c), selNewRenderPipelineStateWithDescriptor, o(d), o(options), (*C.gk_obj)(unsafe.Pointer(err))))
}
func computePipelineStateMaxTotalThreadsPerThreadgroup(p id) uint {
	return uint(C.gk_u_(o(p), selMaxTotalThreadsPerThreadgroup))
}
func computePipelineStateReflection(p id) id {
	return id(C.gk_o(o(p), selReflection))
}
func renderPipelineStateReflection(p id) id {
	return id(C.gk_o(o(p), selReflection))
}
func computePipelineReflectionBindings(r id) id {
	return id(C.gk_o(o(r), selBindings))
}
func renderPipelineReflectionVertexBindings(r id) id {
	return id(C.gk_o(o(r), selVertexBindings))
}
func renderPipelineReflectionFragmentBindings(r id) id {
	return id(C.gk_o(o(r), selFragmentBindings))
}
func bindingIndex(b id) uint {
	return uint(C.gk_u_(o(b), selIndex))
}
func bindingType(b id) uint {
	return uint(C.gk_u_(o(b), selType))
}
func bindingIsUsed(b id) bool {
	return C.gk_b(o(b), selIsUsed) != 0
}
func bufferBindingBufferDataSize(b id) uint {
	return uint(C.gk_u_(o(b), selBufferDataSize))
}
func depthStencilDescriptorSetDepthCompareFunction(d depthStencilDescriptor, v uint) {
	C.gk_v_u(o(d), selSetDepthCompareFunction, C.gk_u(v))
}
func depthStencilDescriptorSetDepthWriteEnabled(d depthStencilDescriptor, v bool) {
	C.gk_v_u(o(d), selSetDepthWriteEnabled, b2u(v))
}

// CAMetalLayer, CAMetalDrawable, NSView, NSWindow.

func metalLayerLayer() id {
	return id(C.gk_o(o(objcGetClass("CAMetalLayer")), selLayer))
}
func metalLayerSetDevice(l id, d device) {
	C.gk_v_o(o(l), selSetDevice, o(d))
}
func metalLayerSetPixelFormat(l id, v uint) {
	C.gk_v_u(o(l), selSetPixelFormat, C.gk_u(v))
}
func metalLayerSetFramebufferOnly(l id, v bool) {
	C.gk_v_u(o(l), selSetFramebufferOnly, b2u(v))
}
func metalLayerSetDisplaySyncEnabled(l id, v bool) {
	C.gk_v_u(o(l), selSetDisplaySyncEnabled, b2u(v))
}
func metalLayerSetContentsScale(l id, v float64) {
	C.gk_v_d(o(l), selSetContentsScale, C.double(v))
}
func metalLayerSetDrawableSize(l id, width, height float64) {
	C.gk_v_cgsize(o(l), selSetDrawableSize, C.CGSize{width: C.CGFloat(width), height: C.CGFloat(height)})
}
func metalLayerDrawableSize(l id) (width, height float64) {
	s := C.gk_size(o(l), selDrawableSize)
	return float64(s.width), float64(s.height)
}
func metalLayerNextDrawable(l id) id {
	return id(C.gk_o(o(l), selNextDrawable))
}
func drawableTexture(d id) texture {
	return texture(C.gk_o(o(d), selTexture))
}
func drawablePresent(d id) {
	C.gk_v(o(d), selPresent)
}
func viewLayer(v id) id {
	return id(C.gk_o(o(v), selLayer))
}
func viewSetLayer(v, l id) {
	C.gk_v_o(o(v), selSetLayer, o(l))
}
func viewSetWantsLayer(v id, w bool) {
	C.gk_v_u(o(v), selSetWantsLayer, b2u(w))
}
func viewWindow(v id) id {
	return id(C.gk_o(o(v), selWindow))
}
func windowContentView(w id) id {
	return id(C.gk_o(o(w), selContentView))
}
func windowBackingScaleFactor(w id) float64 {
	return float64(C.gk_d(o(w), selBackingScaleFactor))
}
func viewBounds(v id) (x, y, width, height float64) {
	r := C.gk_rect(o(v), selBounds)
	return float64(r.origin.x), float64(r.origin.y), float64(r.size.width), float64(r.size.height)
}

// Timestamps: MTL4CounterHeap and its descriptor.

func counterHeapDescriptorSetType(d counterHeapDescriptor, v uint) {
	C.gk_v_u(o(d), selSetType, C.gk_u(v))
}
func counterHeapDescriptorSetCount(d counterHeapDescriptor, v uint) {
	C.gk_v_u(o(d), selSetCount, C.gk_u(v))
}
func counterHeapInvalidateCounterRange(h id, location, length uint) {
	C.gk_v_r(o(h), selInvalidateCounterRange, C.NSRange{location: C.NSUInteger(location), length: C.NSUInteger(length)})
}
func counterHeapResolveCounterRange(h id, location, length uint) id {
	return id(C.gk_o_r(o(h), selResolveCounterRange, C.NSRange{location: C.NSUInteger(location), length: C.NSUInteger(length)}))
}

// Recording: MTL4CommandAllocator, MTL4CommandBuffer, the render pass descriptors.

func commandAllocatorReset(a allocator) {
	C.gk_v(o(a), selReset)
}
func commandBufferBeginCommandBufferWithAllocator(b commandBuffer, a allocator) {
	C.gk_v_o(o(b), selBeginCommandBufferWithAllocator, o(a))
}
func commandBufferEndCommandBuffer(b commandBuffer) {
	C.gk_v(o(b), selEndCommandBuffer)
}
func commandBufferComputeCommandEncoder(b commandBuffer) computeCommandEncoder {
	return computeCommandEncoder(C.gk_o(o(b), selComputeCommandEncoder))
}
func commandBufferRenderCommandEncoderWithDescriptor(b commandBuffer, d renderPassDescriptor) renderCommandEncoder {
	return renderCommandEncoder(C.gk_o_o(o(b), selRenderCommandEncoderWithDescriptor, o(d)))
}
func renderPassDescriptorColorAttachments(d renderPassDescriptor) id {
	return id(C.gk_o(o(d), selColorAttachments))
}
func renderPassDescriptorDepthAttachment(d renderPassDescriptor) renderPassDepthAttachmentDescriptor {
	return renderPassDepthAttachmentDescriptor(C.gk_o(o(d), selDepthAttachment))
}
func renderPassAttachmentDescriptorSetTexture(a renderPassAttachmentDescriptor, t texture) {
	C.gk_v_o(o(a), selSetTexture, o(t))
}
func renderPassAttachmentDescriptorSetResolveTexture(a renderPassAttachmentDescriptor, t texture) {
	C.gk_v_o(o(a), selSetResolveTexture, o(t))
}
func renderPassAttachmentDescriptorSetLoadAction(a renderPassAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetLoadAction, C.gk_u(v))
}
func renderPassAttachmentDescriptorSetStoreAction(a renderPassAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetStoreAction, C.gk_u(v))
}
func renderPassColorAttachmentDescriptorSetClearColor(a renderPassColorAttachmentDescriptor, red, green, blue, alpha float64) {
	C.gk_v_clear(o(a), selSetClearColor, C.MTLClearColor{
		red: C.double(red), green: C.double(green), blue: C.double(blue), alpha: C.double(alpha),
	})
}
func renderPassDepthAttachmentDescriptorSetClearDepth(a renderPassDepthAttachmentDescriptor, v float64) {
	C.gk_v_d(o(a), selSetClearDepth, C.double(v))
}
func renderPassDepthAttachmentDescriptorSetDepthResolveFilter(a renderPassDepthAttachmentDescriptor, v uint) {
	C.gk_v_u(o(a), selSetDepthResolveFilter, C.gk_u(v))
}

// MTL4CommitOptions and MTL4CommitFeedback.

// commitOptionsAddFeedbackHandler adds handler as the commit's feedback handler. It
// runs once, on a thread Metal owns.
func commitOptionsAddFeedbackHandler(c id, handler func(feedback id)) {
	C.gk_add_feedback_handler(o(c), selAddFeedbackHandler, C.uintptr_t(cgo.NewHandle(handler)))
}
func commitFeedbackError(f id) id {
	return id(C.gk_o(o(f), selError))
}
func commitFeedbackGPUStartTime(f id) float64 {
	return float64(C.gk_d(o(f), selGPUStartTime))
}
func commitFeedbackGPUEndTime(f id) float64 {
	return float64(C.gk_d(o(f), selGPUEndTime))
}

// Encoders. The messages every MTL4 encoder takes are named for commandEncoder.

func commandEncoderEndEncoding(e id) {
	C.gk_v(o(e), selEndEncoding)
}
func commandEncoderUpdateFence(e, fence id, stages uint) {
	C.gk_v_ou(o(e), selUpdateFenceAfterEncoderStages, o(fence), C.gk_u(stages))
}
func commandEncoderWaitForFence(e, fence id, stages uint) {
	C.gk_v_ou(o(e), selWaitForFenceBeforeEncoderStages, o(fence), C.gk_u(stages))
}

func computeCommandEncoderSetArgumentTable(e computeCommandEncoder, t argumentTable) {
	C.gk_v_o(o(e), selSetArgumentTable, o(t))
}
func computeCommandEncoderSetComputePipelineState(e computeCommandEncoder, p id) {
	C.gk_v_o(o(e), selSetComputePipelineState, o(p))
}
func computeCommandEncoderDispatchThreadgroups(e computeCommandEncoder, groups, threads [3]uint) {
	C.gk_v_ss(o(e), selDispatchThreadgroupsThreadsPerThreadgroup, mtlSize(groups), mtlSize(threads))
}
func computeCommandEncoderDispatchThreadgroupsWithIndirectBuffer(e computeCommandEncoder, address uint64, threads [3]uint) {
	C.gk_v_us(o(e), selDispatchThreadgroupsWithIndirectBuffer, C.uint64_t(address), mtlSize(threads))
}
func computeCommandEncoderCopyFromBufferToBuffer(e computeCommandEncoder, src buffer, srcOffset uint, dst buffer, dstOffset, size uint) {
	C.gk_v_ououu(o(e), selCopyFromBufferToBuffer, o(src), C.gk_u(srcOffset), o(dst), C.gk_u(dstOffset), C.gk_u(size))
}
func computeCommandEncoderCopyFromBufferToTexture(e computeCommandEncoder, src buffer, srcOffset, bytesPerRow, bytesPerImage uint, size [3]uint,
	dst texture, slice, level uint, origin [3]uint) {
	C.gk_v_ouuusouuz(o(e), selCopyFromBufferToTexture, o(src), C.gk_u(srcOffset), C.gk_u(bytesPerRow),
		C.gk_u(bytesPerImage), mtlSize(size), o(dst), C.gk_u(slice), C.gk_u(level), mtlOrigin(origin))
}
func computeCommandEncoderCopyFromTextureToBuffer(e computeCommandEncoder, src texture, slice, level uint, origin, size [3]uint,
	dst buffer, dstOffset, bytesPerRow, bytesPerImage uint) {
	C.gk_v_ouuzsouuu(o(e), selCopyFromTextureToBuffer, o(src), C.gk_u(slice), C.gk_u(level), mtlOrigin(origin),
		mtlSize(size), o(dst), C.gk_u(dstOffset), C.gk_u(bytesPerRow), C.gk_u(bytesPerImage))
}
func computeCommandEncoderCopyFromTextureToTexture(e computeCommandEncoder, src texture, srcSlice, srcLevel uint, srcOrigin, size [3]uint,
	dst texture, dstSlice, dstLevel uint, dstOrigin [3]uint) {
	C.gk_v_ouuzsouuz(o(e), selCopyFromTextureToTexture, o(src), C.gk_u(srcSlice), C.gk_u(srcLevel),
		mtlOrigin(srcOrigin), mtlSize(size), o(dst), C.gk_u(dstSlice), C.gk_u(dstLevel), mtlOrigin(dstOrigin))
}
func computeCommandEncoderWriteTimestampWithGranularity(e computeCommandEncoder, granularity uint, heap id, index uint) {
	C.gk_v_uou(o(e), selWriteTimestampWithGranularityIntoHeap, C.gk_u(granularity), o(heap), C.gk_u(index))
}

func renderCommandEncoderSetArgumentTable(e renderCommandEncoder, t argumentTable, stages uint) {
	C.gk_v_ou(o(e), selSetArgumentTableAtStages, o(t), C.gk_u(stages))
}
func renderCommandEncoderSetViewport(e renderCommandEncoder, x, y, width, height, znear, zfar float64) {
	C.gk_v_viewport(o(e), selSetViewport, C.MTLViewport{originX: C.double(x), originY: C.double(y),
		width: C.double(width), height: C.double(height), znear: C.double(znear), zfar: C.double(zfar)})
}
func renderCommandEncoderSetScissorRect(e renderCommandEncoder, x, y, width, height uint) {
	C.gk_v_scissor(o(e), selSetScissorRect, C.MTLScissorRect{x: C.NSUInteger(x), y: C.NSUInteger(y),
		width: C.NSUInteger(width), height: C.NSUInteger(height)})
}
func renderCommandEncoderSetColorStoreAction(e renderCommandEncoder, action, index uint) {
	C.gk_v_uu(o(e), selSetColorStoreActionAtIndex, C.gk_u(action), C.gk_u(index))
}
func renderCommandEncoderSetDepthStoreAction(e renderCommandEncoder, action uint) {
	C.gk_v_u(o(e), selSetDepthStoreAction, C.gk_u(action))
}
func renderCommandEncoderSetRenderPipelineState(e renderCommandEncoder, p id) {
	C.gk_v_o(o(e), selSetRenderPipelineState, o(p))
}
func renderCommandEncoderSetDepthStencilState(e renderCommandEncoder, s id) {
	C.gk_v_o(o(e), selSetDepthStencilState, o(s))
}
func renderCommandEncoderSetCullMode(e renderCommandEncoder, v uint) {
	C.gk_v_u(o(e), selSetCullMode, C.gk_u(v))
}
func renderCommandEncoderSetFrontFacingWinding(e renderCommandEncoder, v uint) {
	C.gk_v_u(o(e), selSetFrontFacingWinding, C.gk_u(v))
}
func renderCommandEncoderSetDepthBias(e renderCommandEncoder, bias, slope, clamp float32) {
	C.gk_v_fff(o(e), selSetDepthBiasSlopeScaleClamp, C.float(bias), C.float(slope), C.float(clamp))
}
func renderCommandEncoderDrawPrimitives(e renderCommandEncoder, primitive, vertexStart, vertexCount, instanceCount, baseInstance uint) {
	C.gk_v_uuuuu(o(e), selDrawPrimitives, C.gk_u(primitive), C.gk_u(vertexStart), C.gk_u(vertexCount),
		C.gk_u(instanceCount), C.gk_u(baseInstance))
}
func renderCommandEncoderDrawIndexedPrimitivesIndexCount(e renderCommandEncoder, primitive, indexCount, indexType uint, indexBuffer uint64,
	indexBufferLength, instanceCount uint, baseVertex int, baseInstance uint) {
	C.gk_v_uuuuuuuu(o(e), selDrawIndexedPrimitivesIndexCount, C.gk_u(primitive), C.gk_u(indexCount),
		C.gk_u(indexType), C.gk_u(indexBuffer), C.gk_u(indexBufferLength), C.gk_u(instanceCount),
		C.NSInteger(baseVertex), C.gk_u(baseInstance))
}
func renderCommandEncoderDrawIndexedPrimitivesIndirectBuffer(e renderCommandEncoder, primitive, indexType uint, indexBuffer uint64,
	indexBufferLength uint, indirectBuffer uint64) {
	C.gk_v_uuuuu(o(e), selDrawIndexedPrimitivesIndirectBuffer, C.gk_u(primitive), C.gk_u(indexType),
		C.gk_u(indexBuffer), C.gk_u(indexBufferLength), C.gk_u(indirectBuffer))
}
func renderCommandEncoderWriteTimestampWithGranularity(e renderCommandEncoder, granularity, stage uint, heap id, index uint) {
	C.gk_v_uuou(o(e), selWriteTimestampWithGranularityAfterStage, C.gk_u(granularity), C.gk_u(stage), o(heap),
		C.gk_u(index))
}

func mtlSize(v [3]uint) C.MTLSize {
	return C.MTLSize{width: C.NSUInteger(v[0]), height: C.NSUInteger(v[1]), depth: C.NSUInteger(v[2])}
}

func mtlOrigin(v [3]uint) C.MTLOrigin {
	return C.MTLOrigin{x: C.NSUInteger(v[0]), y: C.NSUInteger(v[1]), z: C.NSUInteger(v[2])}
}

// autoreleasePool collects the autoreleased objects of a stretch of messages, which
// would otherwise leak: threads Go creates have no pool of their own. It holds the
// goroutine on its thread until drained, as a pool belongs to one thread.
type autoreleasePool struct{ p unsafe.Pointer }

func pushPool() autoreleasePool {
	runtime.LockOSThread()
	return autoreleasePool{objcAutoreleasePoolPush()}
}

func (p autoreleasePool) drain() {
	objcAutoreleasePoolPop(p.p)
	runtime.UnlockOSThread()
}

// Selectors, registered once.
var (
	selAddAllocation                             = selRegisterName("addAllocation:")
	selAddFeedbackHandler                        = selRegisterName("addFeedbackHandler:")
	selAddResidencySet                           = selRegisterName("addResidencySet:")
	selAlloc                                     = selRegisterName("alloc")
	selAllValues                                 = selRegisterName("allValues")
	selArgumentBuffersSupport                    = selRegisterName("argumentBuffersSupport")
	selAutorelease                               = selRegisterName("autorelease")
	selBackingScaleFactor                        = selRegisterName("backingScaleFactor")
	selBeginCommandBufferWithAllocator           = selRegisterName("beginCommandBufferWithAllocator:")
	selBindings                                  = selRegisterName("bindings")
	selBounds                                    = selRegisterName("bounds")
	selBufferDataSize                            = selRegisterName("bufferDataSize")
	selBytes                                     = selRegisterName("bytes")
	selColorAttachments                          = selRegisterName("colorAttachments")
	selCommit                                    = selRegisterName("commit")
	selCommitCountOptions                        = selRegisterName("commit:count:options:")
	selComputeCommandEncoder                     = selRegisterName("computeCommandEncoder")
	selContents                                  = selRegisterName("contents")
	selContentView                               = selRegisterName("contentView")
	selCopy                                      = selRegisterName("copy")
	selCopyFromBufferToBuffer                    = selRegisterName("copyFromBuffer:sourceOffset:toBuffer:destinationOffset:size:")
	selCopyFromBufferToTexture                   = selRegisterName("copyFromBuffer:sourceOffset:sourceBytesPerRow:sourceBytesPerImage:sourceSize:toTexture:destinationSlice:destinationLevel:destinationOrigin:")
	selCopyFromTextureToBuffer                   = selRegisterName("copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:toBuffer:destinationOffset:destinationBytesPerRow:destinationBytesPerImage:")
	selCopyFromTextureToTexture                  = selRegisterName("copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:toTexture:destinationSlice:destinationLevel:destinationOrigin:")
	selCount                                     = selRegisterName("count")
	selDepthAttachment                           = selRegisterName("depthAttachment")
	selDispatchThreadgroupsThreadsPerThreadgroup = selRegisterName("dispatchThreadgroups:threadsPerThreadgroup:")
	selDispatchThreadgroupsWithIndirectBuffer    = selRegisterName("dispatchThreadgroupsWithIndirectBuffer:threadsPerThreadgroup:")
	selDrawableSize                              = selRegisterName("drawableSize")
	selDrawIndexedPrimitivesIndexCount           = selRegisterName("drawIndexedPrimitives:indexCount:indexType:indexBuffer:indexBufferLength:instanceCount:baseVertex:baseInstance:")
	selDrawIndexedPrimitivesIndirectBuffer       = selRegisterName("drawIndexedPrimitives:indexType:indexBuffer:indexBufferLength:indirectBuffer:")
	selDrawPrimitives                            = selRegisterName("drawPrimitives:vertexStart:vertexCount:instanceCount:baseInstance:")
	selEndCommandBuffer                          = selRegisterName("endCommandBuffer")
	selEndEncoding                               = selRegisterName("endEncoding")
	selError                                     = selRegisterName("error")
	selFragmentBindings                          = selRegisterName("fragmentBindings")
	selFunctionConstantsDictionary               = selRegisterName("functionConstantsDictionary")
	selGPUAddress                                = selRegisterName("gpuAddress")
	selGPUEndTime                                = selRegisterName("GPUEndTime")
	selGPUResourceID                             = selRegisterName("gpuResourceID")
	selGPUStartTime                              = selRegisterName("GPUStartTime")
	selHasUnifiedMemory                          = selRegisterName("hasUnifiedMemory")
	selHeight                                    = selRegisterName("height")
	selIndex                                     = selRegisterName("index")
	selInit                                      = selRegisterName("init")
	selInitWithBytes                             = selRegisterName("initWithBytes:length:encoding:")
	selInvalidateCounterRange                    = selRegisterName("invalidateCounterRange:")
	selIsKindOfClass                             = selRegisterName("isKindOfClass:")
	selIsUsed                                    = selRegisterName("isUsed")
	selLayer                                     = selRegisterName("layer")
	selLength                                    = selRegisterName("length")
	selLocalizedDescription                      = selRegisterName("localizedDescription")
	selMaxArgumentBufferSamplerCount             = selRegisterName("maxArgumentBufferSamplerCount")
	selMaxBufferLength                           = selRegisterName("maxBufferLength")
	selMaxTotalThreadsPerThreadgroup             = selRegisterName("maxTotalThreadsPerThreadgroup")
	selName                                      = selRegisterName("name")
	selNew                                       = selRegisterName("new")
	selNewArgumentTableWithDescriptor            = selRegisterName("newArgumentTableWithDescriptor:error:")
	selNewBufferWithLength                       = selRegisterName("newBufferWithLength:options:")
	selNewCommandAllocatorWithDescriptor         = selRegisterName("newCommandAllocatorWithDescriptor:error:")
	selNewCommandBuffer                          = selRegisterName("newCommandBuffer")
	selNewCompilerWithDescriptor                 = selRegisterName("newCompilerWithDescriptor:error:")
	selNewComputePipelineStateWithDescriptor     = selRegisterName("newComputePipelineStateWithDescriptor:compilerTaskOptions:error:")
	selNewCounterHeapWithDescriptor              = selRegisterName("newCounterHeapWithDescriptor:error:")
	selNewDepthStencilStateWithDescriptor        = selRegisterName("newDepthStencilStateWithDescriptor:")
	selNewFence                                  = selRegisterName("newFence")
	selNewFunctionWithName                       = selRegisterName("newFunctionWithName:")
	selNewLibraryWithData                        = selRegisterName("newLibraryWithData:error:")
	selNewLibraryWithSource                      = selRegisterName("newLibraryWithSource:options:error:")
	selNewMTL4CommandQueue                       = selRegisterName("newMTL4CommandQueue")
	selNewRenderPipelineStateWithDescriptor      = selRegisterName("newRenderPipelineStateWithDescriptor:compilerTaskOptions:error:")
	selNewResidencySetWithDescriptor             = selRegisterName("newResidencySetWithDescriptor:error:")
	selNewSamplerStateWithDescriptor             = selRegisterName("newSamplerStateWithDescriptor:")
	selNewSharedEvent                            = selRegisterName("newSharedEvent")
	selNewTextureViewWithPixelFormat             = selRegisterName("newTextureViewWithPixelFormat:textureType:levels:slices:")
	selNewTextureWithDescriptor                  = selRegisterName("newTextureWithDescriptor:")
	selNextDrawable                              = selRegisterName("nextDrawable")
	selObjectAtIndex                             = selRegisterName("objectAtIndex:")
	selObjectAtIndexedSubscript                  = selRegisterName("objectAtIndexedSubscript:")
	selPixelFormat                               = selRegisterName("pixelFormat")
	selPresent                                   = selRegisterName("present")
	selReflection                                = selRegisterName("reflection")
	selRelease                                   = selRegisterName("release")
	selRemoveAllocation                          = selRegisterName("removeAllocation:")
	selRenderCommandEncoderWithDescriptor        = selRegisterName("renderCommandEncoderWithDescriptor:")
	selRequestResidency                          = selRegisterName("requestResidency")
	selReset                                     = selRegisterName("reset")
	selResolveCounterRange                       = selRegisterName("resolveCounterRange:")
	selRetain                                    = selRegisterName("retain")
	selSampleTimestamps                          = selRegisterName("sampleTimestamps:gpuTimestamp:")
	selSetAddressAtIndex                         = selRegisterName("setAddress:atIndex:")
	selSetAlphaBlendOperation                    = selRegisterName("setAlphaBlendOperation:")
	selSetArgumentTable                          = selRegisterName("setArgumentTable:")
	selSetArgumentTableAtStages                  = selRegisterName("setArgumentTable:atStages:")
	selSetArrayLength                            = selRegisterName("setArrayLength:")
	selSetBlendingState                          = selRegisterName("setBlendingState:")
	selSetClearColor                             = selRegisterName("setClearColor:")
	selSetClearDepth                             = selRegisterName("setClearDepth:")
	selSetColorStoreActionAtIndex                = selRegisterName("setColorStoreAction:atIndex:")
	selSetCompareFunction                        = selRegisterName("setCompareFunction:")
	selSetComputeFunctionDescriptor              = selRegisterName("setComputeFunctionDescriptor:")
	selSetComputePipelineState                   = selRegisterName("setComputePipelineState:")
	selSetConstantValue                          = selRegisterName("setConstantValue:type:atIndex:")
	selSetConstantValues                         = selRegisterName("setConstantValues:")
	selSetContentsScale                          = selRegisterName("setContentsScale:")
	selSetCount                                  = selRegisterName("setCount:")
	selSetCullMode                               = selRegisterName("setCullMode:")
	selSetDepth                                  = selRegisterName("setDepth:")
	selSetDepthBiasSlopeScaleClamp               = selRegisterName("setDepthBias:slopeScale:clamp:")
	selSetDepthCompareFunction                   = selRegisterName("setDepthCompareFunction:")
	selSetDepthResolveFilter                     = selRegisterName("setDepthResolveFilter:")
	selSetDepthStencilState                      = selRegisterName("setDepthStencilState:")
	selSetDepthStoreAction                       = selRegisterName("setDepthStoreAction:")
	selSetDepthWriteEnabled                      = selRegisterName("setDepthWriteEnabled:")
	selSetDestinationAlphaBlendFactor            = selRegisterName("setDestinationAlphaBlendFactor:")
	selSetDestinationRGBBlendFactor              = selRegisterName("setDestinationRGBBlendFactor:")
	selSetDevice                                 = selRegisterName("setDevice:")
	selSetDisplaySyncEnabled                     = selRegisterName("setDisplaySyncEnabled:")
	selSetDrawableSize                           = selRegisterName("setDrawableSize:")
	selSetFragmentFunctionDescriptor             = selRegisterName("setFragmentFunctionDescriptor:")
	selSetFramebufferOnly                        = selRegisterName("setFramebufferOnly:")
	selSetFrontFacingWinding                     = selRegisterName("setFrontFacingWinding:")
	selSetFunctionDescriptor                     = selRegisterName("setFunctionDescriptor:")
	selSetHeight                                 = selRegisterName("setHeight:")
	selSetLabel                                  = selRegisterName("setLabel:")
	selSetLanguageVersion                        = selRegisterName("setLanguageVersion:")
	selSetLayer                                  = selRegisterName("setLayer:")
	selSetLibrary                                = selRegisterName("setLibrary:")
	selSetLoadAction                             = selRegisterName("setLoadAction:")
	selSetMagFilter                              = selRegisterName("setMagFilter:")
	selSetMaxAnisotropy                          = selRegisterName("setMaxAnisotropy:")
	selSetMaxBufferBindCount                     = selRegisterName("setMaxBufferBindCount:")
	selSetMinFilter                              = selRegisterName("setMinFilter:")
	selSetMipFilter                              = selRegisterName("setMipFilter:")
	selSetMipmapLevelCount                       = selRegisterName("setMipmapLevelCount:")
	selSetName                                   = selRegisterName("setName:")
	selSetOptions                                = selRegisterName("setOptions:")
	selSetPixelFormat                            = selRegisterName("setPixelFormat:")
	selSetRAddressMode                           = selRegisterName("setRAddressMode:")
	selSetRasterSampleCount                      = selRegisterName("setRasterSampleCount:")
	selSetRenderPipelineState                    = selRegisterName("setRenderPipelineState:")
	selSetResolveTexture                         = selRegisterName("setResolveTexture:")
	selSetRgbBlendOperation                      = selRegisterName("setRgbBlendOperation:")
	selSetSAddressMode                           = selRegisterName("setSAddressMode:")
	selSetSampleCount                            = selRegisterName("setSampleCount:")
	selSetScissorRect                            = selRegisterName("setScissorRect:")
	selSetShaderReflection                       = selRegisterName("setShaderReflection:")
	selSetSourceAlphaBlendFactor                 = selRegisterName("setSourceAlphaBlendFactor:")
	selSetSourceRGBBlendFactor                   = selRegisterName("setSourceRGBBlendFactor:")
	selSetStorageMode                            = selRegisterName("setStorageMode:")
	selSetStoreAction                            = selRegisterName("setStoreAction:")
	selSetSupportArgumentBuffers                 = selRegisterName("setSupportArgumentBuffers:")
	selSetTAddressMode                           = selRegisterName("setTAddressMode:")
	selSetTexture                                = selRegisterName("setTexture:")
	selSetTextureType                            = selRegisterName("setTextureType:")
	selSetType                                   = selRegisterName("setType:")
	selSetUsage                                  = selRegisterName("setUsage:")
	selSetVertexFunctionDescriptor               = selRegisterName("setVertexFunctionDescriptor:")
	selSetViewport                               = selRegisterName("setViewport:")
	selSetWantsLayer                             = selRegisterName("setWantsLayer:")
	selSetWidth                                  = selRegisterName("setWidth:")
	selSetWriteMask                              = selRegisterName("setWriteMask:")
	selSignalDrawable                            = selRegisterName("signalDrawable:")
	selSignalEventValue                          = selRegisterName("signalEvent:value:")
	selSupportsFamily                            = selRegisterName("supportsFamily:")
	selTexture                                   = selRegisterName("texture")
	selType                                      = selRegisterName("type")
	selUpdateFenceAfterEncoderStages             = selRegisterName("updateFence:afterEncoderStages:")
	selUTF8String                                = selRegisterName("UTF8String")
	selVertexBindings                            = selRegisterName("vertexBindings")
	selWaitForFenceBeforeEncoderStages           = selRegisterName("waitForFence:beforeEncoderStages:")
	selWaitUntilSignaledValue                    = selRegisterName("waitUntilSignaledValue:timeoutMS:")
	selWidth                                     = selRegisterName("width")
	selWindow                                    = selRegisterName("window")
	selWriteTimestampWithGranularityAfterStage   = selRegisterName("writeTimestampWithGranularity:afterStage:intoHeap:atIndex:")
	selWriteTimestampWithGranularityIntoHeap     = selRegisterName("writeTimestampWithGranularity:intoHeap:atIndex:")
)
