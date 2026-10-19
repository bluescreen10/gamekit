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
	mtl4BlendStateDisabled                  mtl4BlendState                   = C.MTL4BlendStateDisabled
	mtl4BlendStateEnabled                   mtl4BlendState                   = C.MTL4BlendStateEnabled
	mtl4CounterHeapTypeTimestamp            mtl4CounterHeapType              = C.MTL4CounterHeapTypeTimestamp
	mtl4ShaderReflectionBindingInfo         mtl4ShaderReflection             = C.MTL4ShaderReflectionBindingInfo
	mtl4TimestampGranularityPrecise         mtl4TimestampGranularity         = C.MTL4TimestampGranularityPrecise
	mtlArgumentBuffersTier2                 mtlArgumentBuffersTier           = C.MTLArgumentBuffersTier2
	mtlBindingTypeBuffer                    mtlBindingType                   = C.MTLBindingTypeBuffer
	mtlBlendFactorDestinationAlpha          mtlBlendFactor                   = C.MTLBlendFactorDestinationAlpha
	mtlBlendFactorOne                       mtlBlendFactor                   = C.MTLBlendFactorOne
	mtlBlendFactorOneMinusDestinationAlpha  mtlBlendFactor                   = C.MTLBlendFactorOneMinusDestinationAlpha
	mtlBlendFactorOneMinusSourceAlpha       mtlBlendFactor                   = C.MTLBlendFactorOneMinusSourceAlpha
	mtlBlendFactorSourceAlpha               mtlBlendFactor                   = C.MTLBlendFactorSourceAlpha
	mtlBlendFactorZero                      mtlBlendFactor                   = C.MTLBlendFactorZero
	mtlCompareFunctionAlways                mtlCompareFunction               = C.MTLCompareFunctionAlways
	mtlCullModeBack                         mtlCullMode                      = C.MTLCullModeBack
	mtlCullModeFront                        mtlCullMode                      = C.MTLCullModeFront
	mtlCullModeNone                         mtlCullMode                      = C.MTLCullModeNone
	mtlDataTypeBool                         mtlDataType                      = C.MTLDataTypeBool
	mtlDataTypeFloat                        mtlDataType                      = C.MTLDataTypeFloat
	mtlDataTypeInt                          mtlDataType                      = C.MTLDataTypeInt
	mtlDataTypeUInt                         mtlDataType                      = C.MTLDataTypeUInt
	mtlGPUFamilyApple1                      mtlGPUFamily                     = C.MTLGPUFamilyApple1
	mtlGPUFamilyMetal4                      mtlGPUFamily                     = C.MTLGPUFamilyMetal4
	mtlIndexTypeUInt16                      mtlIndexType                     = C.MTLIndexTypeUInt16
	mtlIndexTypeUInt32                      mtlIndexType                     = C.MTLIndexTypeUInt32
	mtlLanguageVersion3_0                   mtlLanguageVersion               = C.MTLLanguageVersion3_0
	mtlLoadActionClear                      mtlLoadAction                    = C.MTLLoadActionClear
	mtlLoadActionDontCare                   mtlLoadAction                    = C.MTLLoadActionDontCare
	mtlLoadActionLoad                       mtlLoadAction                    = C.MTLLoadActionLoad
	mtlMultisampleDepthResolveFilterSample0 mtlMultisampleDepthResolveFilter = C.MTLMultisampleDepthResolveFilterSample0
	mtlPrimitiveTypeLine                    mtlPrimitiveType                 = C.MTLPrimitiveTypeLine
	mtlPrimitiveTypePoint                   mtlPrimitiveType                 = C.MTLPrimitiveTypePoint
	mtlPrimitiveTypeTriangle                mtlPrimitiveType                 = C.MTLPrimitiveTypeTriangle
	mtlPrimitiveTypeTriangleStrip           mtlPrimitiveType                 = C.MTLPrimitiveTypeTriangleStrip
	mtlRenderStageFragment                  mtlRenderStages                  = C.MTLRenderStageFragment
	mtlRenderStageVertex                    mtlRenderStages                  = C.MTLRenderStageVertex
	mtlResourceStorageModePrivate           mtlResourceOptions               = C.MTLResourceStorageModePrivate
	mtlResourceStorageModeShared            mtlResourceOptions               = C.MTLResourceStorageModeShared
	mtlSamplerAddressModeClampToEdge        mtlSamplerAddressMode            = C.MTLSamplerAddressModeClampToEdge
	mtlSamplerAddressModeMirrorRepeat       mtlSamplerAddressMode            = C.MTLSamplerAddressModeMirrorRepeat
	mtlSamplerAddressModeRepeat             mtlSamplerAddressMode            = C.MTLSamplerAddressModeRepeat
	mtlSamplerMinMagFilterLinear            mtlSamplerMinMagFilter           = C.MTLSamplerMinMagFilterLinear
	mtlSamplerMinMagFilterNearest           mtlSamplerMinMagFilter           = C.MTLSamplerMinMagFilterNearest
	mtlSamplerMipFilterLinear               mtlSamplerMipFilter              = C.MTLSamplerMipFilterLinear
	mtlSamplerMipFilterNearest              mtlSamplerMipFilter              = C.MTLSamplerMipFilterNearest
	mtlStageAccelerationStructure           mtlStages                        = C.MTLStageAccelerationStructure
	mtlStageBlit                            mtlStages                        = C.MTLStageBlit
	mtlStageDispatch                        mtlStages                        = C.MTLStageDispatch
	mtlStageFragment                        mtlStages                        = C.MTLStageFragment
	mtlStageMesh                            mtlStages                        = C.MTLStageMesh
	mtlStageObject                          mtlStages                        = C.MTLStageObject
	mtlStageTile                            mtlStages                        = C.MTLStageTile
	mtlStageVertex                          mtlStages                        = C.MTLStageVertex
	mtlStorageModeMemoryless                mtlStorageMode                   = C.MTLStorageModeMemoryless
	mtlStorageModePrivate                   mtlStorageMode                   = C.MTLStorageModePrivate
	mtlStoreActionDontCare                  mtlStoreAction                   = C.MTLStoreActionDontCare
	mtlStoreActionMultisampleResolve        mtlStoreAction                   = C.MTLStoreActionMultisampleResolve
	mtlStoreActionStore                     mtlStoreAction                   = C.MTLStoreActionStore
	mtlStoreActionUnknown                   mtlStoreAction                   = C.MTLStoreActionUnknown
	mtlTextureType2D                        mtlTextureType                   = C.MTLTextureType2D
	mtlTextureType2DArray                   mtlTextureType                   = C.MTLTextureType2DArray
	mtlTextureType2DMultisample             mtlTextureType                   = C.MTLTextureType2DMultisample
	mtlTextureType3D                        mtlTextureType                   = C.MTLTextureType3D
	mtlTextureTypeCube                      mtlTextureType                   = C.MTLTextureTypeCube
	mtlTextureTypeCubeArray                 mtlTextureType                   = C.MTLTextureTypeCubeArray
	mtlTextureUsagePixelFormatView          mtlTextureUsage                  = C.MTLTextureUsagePixelFormatView
	mtlTextureUsageRenderTarget             mtlTextureUsage                  = C.MTLTextureUsageRenderTarget
	mtlTextureUsageShaderRead               mtlTextureUsage                  = C.MTLTextureUsageShaderRead
	mtlTextureUsageShaderWrite              mtlTextureUsage                  = C.MTLTextureUsageShaderWrite
	mtlWindingClockwise                     mtlWinding                       = C.MTLWindingClockwise
	mtlWindingCounterClockwise              mtlWinding                       = C.MTLWindingCounterClockwise
	nsUTF8StringEncoding                                                     = C.NSUTF8StringEncoding

	mtlPixelFormatInvalid               mtlPixelFormat = C.MTLPixelFormatInvalid
	mtlPixelFormatR8Unorm               mtlPixelFormat = C.MTLPixelFormatR8Unorm
	mtlPixelFormatRG8Unorm              mtlPixelFormat = C.MTLPixelFormatRG8Unorm
	mtlPixelFormatRGBA8Unorm            mtlPixelFormat = C.MTLPixelFormatRGBA8Unorm
	mtlPixelFormatBGRA8Unorm            mtlPixelFormat = C.MTLPixelFormatBGRA8Unorm
	mtlPixelFormatRGBA8Unorm_sRGB       mtlPixelFormat = C.MTLPixelFormatRGBA8Unorm_sRGB
	mtlPixelFormatBGRA8Unorm_sRGB       mtlPixelFormat = C.MTLPixelFormatBGRA8Unorm_sRGB
	mtlPixelFormatR8Snorm               mtlPixelFormat = C.MTLPixelFormatR8Snorm
	mtlPixelFormatRG8Snorm              mtlPixelFormat = C.MTLPixelFormatRG8Snorm
	mtlPixelFormatRGBA8Snorm            mtlPixelFormat = C.MTLPixelFormatRGBA8Snorm
	mtlPixelFormatR8Uint                mtlPixelFormat = C.MTLPixelFormatR8Uint
	mtlPixelFormatRG8Uint               mtlPixelFormat = C.MTLPixelFormatRG8Uint
	mtlPixelFormatRGBA8Uint             mtlPixelFormat = C.MTLPixelFormatRGBA8Uint
	mtlPixelFormatR8Sint                mtlPixelFormat = C.MTLPixelFormatR8Sint
	mtlPixelFormatRG8Sint               mtlPixelFormat = C.MTLPixelFormatRG8Sint
	mtlPixelFormatRGBA8Sint             mtlPixelFormat = C.MTLPixelFormatRGBA8Sint
	mtlPixelFormatR16Unorm              mtlPixelFormat = C.MTLPixelFormatR16Unorm
	mtlPixelFormatRG16Unorm             mtlPixelFormat = C.MTLPixelFormatRG16Unorm
	mtlPixelFormatRGBA16Unorm           mtlPixelFormat = C.MTLPixelFormatRGBA16Unorm
	mtlPixelFormatR16Snorm              mtlPixelFormat = C.MTLPixelFormatR16Snorm
	mtlPixelFormatRG16Snorm             mtlPixelFormat = C.MTLPixelFormatRG16Snorm
	mtlPixelFormatRGBA16Snorm           mtlPixelFormat = C.MTLPixelFormatRGBA16Snorm
	mtlPixelFormatR16Uint               mtlPixelFormat = C.MTLPixelFormatR16Uint
	mtlPixelFormatRG16Uint              mtlPixelFormat = C.MTLPixelFormatRG16Uint
	mtlPixelFormatRGBA16Uint            mtlPixelFormat = C.MTLPixelFormatRGBA16Uint
	mtlPixelFormatR16Sint               mtlPixelFormat = C.MTLPixelFormatR16Sint
	mtlPixelFormatRG16Sint              mtlPixelFormat = C.MTLPixelFormatRG16Sint
	mtlPixelFormatRGBA16Sint            mtlPixelFormat = C.MTLPixelFormatRGBA16Sint
	mtlPixelFormatR16Float              mtlPixelFormat = C.MTLPixelFormatR16Float
	mtlPixelFormatRG16Float             mtlPixelFormat = C.MTLPixelFormatRG16Float
	mtlPixelFormatRGBA16Float           mtlPixelFormat = C.MTLPixelFormatRGBA16Float
	mtlPixelFormatR32Uint               mtlPixelFormat = C.MTLPixelFormatR32Uint
	mtlPixelFormatRG32Uint              mtlPixelFormat = C.MTLPixelFormatRG32Uint
	mtlPixelFormatRGBA32Uint            mtlPixelFormat = C.MTLPixelFormatRGBA32Uint
	mtlPixelFormatR32Sint               mtlPixelFormat = C.MTLPixelFormatR32Sint
	mtlPixelFormatRG32Sint              mtlPixelFormat = C.MTLPixelFormatRG32Sint
	mtlPixelFormatRGBA32Sint            mtlPixelFormat = C.MTLPixelFormatRGBA32Sint
	mtlPixelFormatR32Float              mtlPixelFormat = C.MTLPixelFormatR32Float
	mtlPixelFormatRG32Float             mtlPixelFormat = C.MTLPixelFormatRG32Float
	mtlPixelFormatRGBA32Float           mtlPixelFormat = C.MTLPixelFormatRGBA32Float
	mtlPixelFormatRGB10A2Unorm          mtlPixelFormat = C.MTLPixelFormatRGB10A2Unorm
	mtlPixelFormatRGB10A2Uint           mtlPixelFormat = C.MTLPixelFormatRGB10A2Uint
	mtlPixelFormatRG11B10Float          mtlPixelFormat = C.MTLPixelFormatRG11B10Float
	mtlPixelFormatRGB9E5Float           mtlPixelFormat = C.MTLPixelFormatRGB9E5Float
	mtlPixelFormatDepth16Unorm          mtlPixelFormat = C.MTLPixelFormatDepth16Unorm
	mtlPixelFormatDepth32Float          mtlPixelFormat = C.MTLPixelFormatDepth32Float
	mtlPixelFormatDepth24Unorm_Stencil8 mtlPixelFormat = C.MTLPixelFormatDepth24Unorm_Stencil8
	mtlPixelFormatDepth32Float_Stencil8 mtlPixelFormat = C.MTLPixelFormatDepth32Float_Stencil8
	mtlPixelFormatStencil8              mtlPixelFormat = C.MTLPixelFormatStencil8
	mtlPixelFormatBC1_RGBA              mtlPixelFormat = C.MTLPixelFormatBC1_RGBA
	mtlPixelFormatBC1_RGBA_sRGB         mtlPixelFormat = C.MTLPixelFormatBC1_RGBA_sRGB
	mtlPixelFormatBC3_RGBA              mtlPixelFormat = C.MTLPixelFormatBC3_RGBA
	mtlPixelFormatBC3_RGBA_sRGB         mtlPixelFormat = C.MTLPixelFormatBC3_RGBA_sRGB
	mtlPixelFormatBC4_RUnorm            mtlPixelFormat = C.MTLPixelFormatBC4_RUnorm
	mtlPixelFormatBC4_RSnorm            mtlPixelFormat = C.MTLPixelFormatBC4_RSnorm
	mtlPixelFormatBC5_RGUnorm           mtlPixelFormat = C.MTLPixelFormatBC5_RGUnorm
	mtlPixelFormatBC5_RGSnorm           mtlPixelFormat = C.MTLPixelFormatBC5_RGSnorm
	mtlPixelFormatBC6H_RGBFloat         mtlPixelFormat = C.MTLPixelFormatBC6H_RGBFloat
	mtlPixelFormatBC6H_RGBUfloat        mtlPixelFormat = C.MTLPixelFormatBC6H_RGBUfloat
	mtlPixelFormatBC7_RGBAUnorm         mtlPixelFormat = C.MTLPixelFormatBC7_RGBAUnorm
	mtlPixelFormatBC7_RGBAUnorm_sRGB    mtlPixelFormat = C.MTLPixelFormatBC7_RGBAUnorm_sRGB
	mtlPixelFormatASTC_4x4_LDR          mtlPixelFormat = C.MTLPixelFormatASTC_4x4_LDR
	mtlPixelFormatASTC_4x4_sRGB         mtlPixelFormat = C.MTLPixelFormatASTC_4x4_sRGB
	mtlPixelFormatASTC_5x5_LDR          mtlPixelFormat = C.MTLPixelFormatASTC_5x5_LDR
	mtlPixelFormatASTC_5x5_sRGB         mtlPixelFormat = C.MTLPixelFormatASTC_5x5_sRGB
	mtlPixelFormatASTC_6x6_LDR          mtlPixelFormat = C.MTLPixelFormatASTC_6x6_LDR
	mtlPixelFormatASTC_6x6_sRGB         mtlPixelFormat = C.MTLPixelFormatASTC_6x6_sRGB
	mtlPixelFormatASTC_8x8_LDR          mtlPixelFormat = C.MTLPixelFormatASTC_8x8_LDR
	mtlPixelFormatASTC_8x8_sRGB         mtlPixelFormat = C.MTLPixelFormatASTC_8x8_sRGB
	mtlPixelFormatASTC_10x10_LDR        mtlPixelFormat = C.MTLPixelFormatASTC_10x10_LDR
	mtlPixelFormatASTC_10x10_sRGB       mtlPixelFormat = C.MTLPixelFormatASTC_10x10_sRGB
	mtlPixelFormatASTC_12x12_LDR        mtlPixelFormat = C.MTLPixelFormatASTC_12x12_LDR
	mtlPixelFormatASTC_12x12_sRGB       mtlPixelFormat = C.MTLPixelFormatASTC_12x12_sRGB
)

func b2u(v bool) C.gk_u {
	if v {
		return 1
	}

	return 0
}

// C functions.

func mtlCreateSystemDefaultDevice() mtlDevice {
	return mtlDevice(C.gk_device())
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

func deviceArgumentBuffersSupport(d mtlDevice) mtlArgumentBuffersTier {
	return mtlArgumentBuffersTier(C.gk_u_(o(d), selArgumentBuffersSupport))
}
func deviceHasUnifiedMemory(d mtlDevice) bool {
	return C.gk_b(o(d), selHasUnifiedMemory) != 0
}
func deviceSupportsFamily(d mtlDevice, family mtlGPUFamily) bool {
	return C.gk_b_u(o(d), selSupportsFamily, C.gk_u(family)) != 0
}
func deviceMaxBufferLength(d mtlDevice) uint {
	return uint(C.gk_u_(o(d), selMaxBufferLength))
}
func deviceMaxArgumentBufferSamplerCount(d mtlDevice) uint {
	return uint(C.gk_u_(o(d), selMaxArgumentBufferSamplerCount))
}
func deviceNewMTL4CommandQueue(d mtlDevice) mtl4CommandQueue {
	return mtl4CommandQueue(C.gk_o(o(d), selNewMTL4CommandQueue))
}
func deviceNewSharedEvent(d mtlDevice) mtlSharedEvent {
	return mtlSharedEvent(C.gk_o(o(d), selNewSharedEvent))
}
func deviceNewFence(d mtlDevice) mtlFence {
	return mtlFence(C.gk_o(o(d), selNewFence))
}
func deviceNewCommandBuffer(d mtlDevice) mtl4CommandBuffer {
	return mtl4CommandBuffer(C.gk_o(o(d), selNewCommandBuffer))
}
func deviceNewCompilerWithDescriptor(d mtlDevice, desc mtl4CompilerDescriptor, err *id) mtl4Compiler {
	return mtl4Compiler(C.gk_o_op(o(d), selNewCompilerWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewResidencySetWithDescriptor(d mtlDevice, desc mtlResidencySetDescriptor, err *id) id {
	return id(C.gk_o_op(o(d), selNewResidencySetWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewCommandAllocatorWithDescriptor(d mtlDevice, desc mtl4CommandAllocatorDescriptor, err *id) mtl4CommandAllocator {
	return mtl4CommandAllocator(C.gk_o_op(o(d), selNewCommandAllocatorWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewArgumentTableWithDescriptor(d mtlDevice, desc mtl4ArgumentTableDescriptor, err *id) mtl4ArgumentTable {
	return mtl4ArgumentTable(C.gk_o_op(o(d), selNewArgumentTableWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewCounterHeapWithDescriptor(d mtlDevice, desc mtl4CounterHeapDescriptor, err *id) mtl4CounterHeap {
	return mtl4CounterHeap(C.gk_o_op(o(d), selNewCounterHeapWithDescriptor, o(desc), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewBufferWithLength(d mtlDevice, length uint, options mtlResourceOptions) mtlBuffer {
	return mtlBuffer(C.gk_o_uu(o(d), selNewBufferWithLength, C.gk_u(length), C.gk_u(options)))
}
func deviceNewTextureWithDescriptor(d mtlDevice, desc mtlTextureDescriptor) mtlTexture {
	return mtlTexture(C.gk_o_o(o(d), selNewTextureWithDescriptor, o(desc)))
}
func deviceNewSamplerStateWithDescriptor(d mtlDevice, desc mtlSamplerDescriptor) mtlSamplerState {
	return mtlSamplerState(C.gk_o_o(o(d), selNewSamplerStateWithDescriptor, o(desc)))
}
func deviceNewDepthStencilStateWithDescriptor(d mtlDevice, desc mtlDepthStencilDescriptor) mtlDepthStencilState {
	return mtlDepthStencilState(C.gk_o_o(o(d), selNewDepthStencilStateWithDescriptor, o(desc)))
}
func deviceNewLibraryWithData(d mtlDevice, data id, err *id) mtlLibrary {
	return mtlLibrary(C.gk_o_op(o(d), selNewLibraryWithData, o(data), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceNewLibraryWithSource(d mtlDevice, source, options id, err *id) mtlLibrary {
	return mtlLibrary(C.gk_o_oop(o(d), selNewLibraryWithSource, o(source), o(options), (*C.gk_obj)(unsafe.Pointer(err))))
}
func deviceSampleTimestamps(d mtlDevice, cpu, gpu *uint64) {
	C.gk_v_pp(o(d), selSampleTimestamps, unsafe.Pointer(cpu), unsafe.Pointer(gpu))
}

// MTL4CommandQueue, MTLResidencySet, MTLSharedEvent.

func commandQueueAddResidencySet(q mtl4CommandQueue, set mtlResidencySet) {
	C.gk_v_o(o(q), selAddResidencySet, o(set))
}
func commandQueueCommit(q mtl4CommandQueue, buffers *mtl4CommandBuffer, count uint, options id) {
	C.gk_v_puo(o(q), selCommitCountOptions, (*C.gk_obj)(unsafe.Pointer(buffers)), C.gk_u(count), o(options))
}
func commandQueueSignalDrawable(q mtl4CommandQueue, drawable mtlDrawable) {
	C.gk_v_o(o(q), selSignalDrawable, o(drawable))
}
func commandQueueSignalEvent(q mtl4CommandQueue, event mtlSharedEvent, value uint64) {
	C.gk_v_ou(o(q), selSignalEventValue, o(event), C.gk_u(value))
}
func residencySetAddAllocation(s mtlResidencySet, allocation id) {
	C.gk_v_o(o(s), selAddAllocation, o(allocation))
}
func residencySetRemoveAllocation(s mtlResidencySet, allocation id) {
	C.gk_v_o(o(s), selRemoveAllocation, o(allocation))
}
func residencySetCommit(s mtlResidencySet) {
	C.gk_v(o(s), selCommit)
}
func residencySetRequestResidency(s mtlResidencySet) {
	C.gk_v(o(s), selRequestResidency)
}
func sharedEventWaitUntilSignaledValue(e mtlSharedEvent, value uint64, timeoutMS uint) bool {
	return C.gk_b_uu(o(e), selWaitUntilSignaledValue, C.gk_u(value), C.gk_u(timeoutMS)) != 0
}

// MTL4ArgumentTable and its descriptor.

func argumentTableDescriptorSetMaxBufferBindCount(d mtl4ArgumentTableDescriptor, n uint) {
	C.gk_v_u(o(d), selSetMaxBufferBindCount, C.gk_u(n))
}
func argumentTableSetAddress(t mtl4ArgumentTable, address uint64, index uint) {
	C.gk_v_uu(o(t), selSetAddressAtIndex, C.gk_u(address), C.gk_u(index))
}

// MTLBuffer, MTLTexture, MTLSamplerState.

func bufferContents(b mtlBuffer) unsafe.Pointer {
	return C.gk_p(o(b), selContents)
}
func bufferGPUAddress(b mtlBuffer) uint64 {
	return uint64(C.gk_u_(o(b), selGPUAddress))
}
func textureGPUResourceID(t mtlTexture) uint64 {
	return uint64(C.gk_rid(o(t), selGPUResourceID))
}
func samplerStateGPUResourceID(s mtlSamplerState) uint64 {
	return uint64(C.gk_rid(o(s), selGPUResourceID))
}
func textureWidth(t mtlTexture) uint {
	return uint(C.gk_u_(o(t), selWidth))
}
func textureHeight(t mtlTexture) uint {
	return uint(C.gk_u_(o(t), selHeight))
}
func texturePixelFormat(t mtlTexture) mtlPixelFormat {
	return mtlPixelFormat(C.gk_u_(o(t), selPixelFormat))
}
func textureNewTextureViewWithPixelFormat(t mtlTexture, format mtlPixelFormat, textureType mtlTextureType, firstLevel, levels, firstSlice, slices uint) mtlTexture {
	return mtlTexture(C.gk_o_uurr(o(t), selNewTextureViewWithPixelFormat, C.gk_u(format), C.gk_u(textureType),
		C.NSRange{location: C.NSUInteger(firstLevel), length: C.NSUInteger(levels)},
		C.NSRange{location: C.NSUInteger(firstSlice), length: C.NSUInteger(slices)}))
}

// MTLTextureDescriptor.

func textureDescriptorSetTextureType(d mtlTextureDescriptor, v mtlTextureType) {
	C.gk_v_u(o(d), selSetTextureType, C.gk_u(v))
}
func textureDescriptorSetWidth(d mtlTextureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetWidth, C.gk_u(v))
}
func textureDescriptorSetHeight(d mtlTextureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetHeight, C.gk_u(v))
}
func textureDescriptorSetDepth(d mtlTextureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetDepth, C.gk_u(v))
}
func textureDescriptorSetArrayLength(d mtlTextureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetArrayLength, C.gk_u(v))
}
func textureDescriptorSetMipmapLevelCount(d mtlTextureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetMipmapLevelCount, C.gk_u(v))
}
func textureDescriptorSetPixelFormat(d mtlTextureDescriptor, v mtlPixelFormat) {
	C.gk_v_u(o(d), selSetPixelFormat, C.gk_u(v))
}
func textureDescriptorSetSampleCount(d mtlTextureDescriptor, v uint) {
	C.gk_v_u(o(d), selSetSampleCount, C.gk_u(v))
}
func textureDescriptorSetStorageMode(d mtlTextureDescriptor, v mtlStorageMode) {
	C.gk_v_u(o(d), selSetStorageMode, C.gk_u(v))
}
func textureDescriptorSetUsage(d mtlTextureDescriptor, v mtlTextureUsage) {
	C.gk_v_u(o(d), selSetUsage, C.gk_u(v))
}

// MTLSamplerDescriptor.

func samplerDescriptorSetMinFilter(d mtlSamplerDescriptor, v mtlSamplerMinMagFilter) {
	C.gk_v_u(o(d), selSetMinFilter, C.gk_u(v))
}
func samplerDescriptorSetMagFilter(d mtlSamplerDescriptor, v mtlSamplerMinMagFilter) {
	C.gk_v_u(o(d), selSetMagFilter, C.gk_u(v))
}
func samplerDescriptorSetMipFilter(d mtlSamplerDescriptor, v mtlSamplerMipFilter) {
	C.gk_v_u(o(d), selSetMipFilter, C.gk_u(v))
}
func samplerDescriptorSetSAddressMode(d mtlSamplerDescriptor, v mtlSamplerAddressMode) {
	C.gk_v_u(o(d), selSetSAddressMode, C.gk_u(v))
}
func samplerDescriptorSetTAddressMode(d mtlSamplerDescriptor, v mtlSamplerAddressMode) {
	C.gk_v_u(o(d), selSetTAddressMode, C.gk_u(v))
}
func samplerDescriptorSetRAddressMode(d mtlSamplerDescriptor, v mtlSamplerAddressMode) {
	C.gk_v_u(o(d), selSetRAddressMode, C.gk_u(v))
}
func samplerDescriptorSetCompareFunction(d mtlSamplerDescriptor, v mtlCompareFunction) {
	C.gk_v_u(o(d), selSetCompareFunction, C.gk_u(v))
}
func samplerDescriptorSetMaxAnisotropy(d mtlSamplerDescriptor, v uint) {
	C.gk_v_u(o(d), selSetMaxAnisotropy, C.gk_u(v))
}
func samplerDescriptorSetSupportArgumentBuffers(d mtlSamplerDescriptor, v bool) {
	C.gk_v_u(o(d), selSetSupportArgumentBuffers, b2u(v))
}

// Shaders: MTLCompileOptions, MTLLibrary, MTLFunction, MTLFunctionConstant, and the
// MTL4 function descriptors.

func compileOptionsSetLanguageVersion(c id, v mtlLanguageVersion) {
	C.gk_v_u(o(c), selSetLanguageVersion, C.gk_u(v))
}
func libraryNewFunctionWithName(l mtlLibrary, name id) id {
	return id(C.gk_o_o(o(l), selNewFunctionWithName, o(name)))
}
func functionFunctionConstantsDictionary(f id) id {
	return id(C.gk_o(o(f), selFunctionConstantsDictionary))
}
func functionConstantType(c id) mtlDataType {
	return mtlDataType(C.gk_u_(o(c), selType))
}
func functionConstantIndex(c id) uint {
	return uint(C.gk_u_(o(c), selIndex))
}
func functionConstantName(c id) id {
	return id(C.gk_o(o(c), selName))
}
func libraryFunctionDescriptorSetLibrary(d mtl4LibraryFunctionDescriptor, l mtlLibrary) {
	C.gk_v_o(o(d), selSetLibrary, o(l))
}
func libraryFunctionDescriptorSetName(d mtl4LibraryFunctionDescriptor, name id) {
	C.gk_v_o(o(d), selSetName, o(name))
}
func functionConstantValuesSetConstantValue(v id, value unsafe.Pointer, dataType mtlDataType, index uint) {
	C.gk_v_puu(o(v), selSetConstantValue, value, C.gk_u(dataType), C.gk_u(index))
}
func specializedFunctionDescriptorSetFunctionDescriptor(d mtl4SpecializedFunctionDescriptor, f mtl4LibraryFunctionDescriptor) {
	C.gk_v_o(o(d), selSetFunctionDescriptor, o(f))
}
func specializedFunctionDescriptorSetConstantValues(d mtl4SpecializedFunctionDescriptor, v id) {
	C.gk_v_o(o(d), selSetConstantValues, o(v))
}

// Pipelines: MTL4PipelineOptions, the pipeline descriptors, MTL4Compiler, the pipeline
// states and their reflection.

func pipelineOptionsSetShaderReflection(p id, v mtl4ShaderReflection) {
	C.gk_v_u(o(p), selSetShaderReflection, C.gk_u(v))
}
func computePipelineDescriptorSetComputeFunctionDescriptor(d mtl4ComputePipelineDescriptor, f mtl4SpecializedFunctionDescriptor) {
	C.gk_v_o(o(d), selSetComputeFunctionDescriptor, o(f))
}
func computePipelineDescriptorSetOptions(d mtl4ComputePipelineDescriptor, p id) {
	C.gk_v_o(o(d), selSetOptions, o(p))
}
func renderPipelineDescriptorSetVertexFunctionDescriptor(d mtl4RenderPipelineDescriptor, f mtl4SpecializedFunctionDescriptor) {
	C.gk_v_o(o(d), selSetVertexFunctionDescriptor, o(f))
}
func renderPipelineDescriptorSetFragmentFunctionDescriptor(d mtl4RenderPipelineDescriptor, f mtl4SpecializedFunctionDescriptor) {
	C.gk_v_o(o(d), selSetFragmentFunctionDescriptor, o(f))
}
func renderPipelineDescriptorSetRasterSampleCount(d mtl4RenderPipelineDescriptor, v uint) {
	C.gk_v_u(o(d), selSetRasterSampleCount, C.gk_u(v))
}
func renderPipelineDescriptorSetOptions(d mtl4RenderPipelineDescriptor, p id) {
	C.gk_v_o(o(d), selSetOptions, o(p))
}
func renderPipelineDescriptorColorAttachments(d mtl4RenderPipelineDescriptor) id {
	return id(C.gk_o(o(d), selColorAttachments))
}
func renderPipelineColorAttachmentDescriptorSetPixelFormat(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlPixelFormat) {
	C.gk_v_u(o(a), selSetPixelFormat, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetWriteMask(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlColorWriteMask) {
	C.gk_v_u(o(a), selSetWriteMask, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetBlendingState(a mtl4RenderPipelineColorAttachmentDescriptor, v mtl4BlendState) {
	C.gk_v_u(o(a), selSetBlendingState, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetSourceRGBBlendFactor(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlBlendFactor) {
	C.gk_v_u(o(a), selSetSourceRGBBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetDestinationRGBBlendFactor(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlBlendFactor) {
	C.gk_v_u(o(a), selSetDestinationRGBBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetRGBBlendOperation(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlBlendOperation) {
	C.gk_v_u(o(a), selSetRgbBlendOperation, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetSourceAlphaBlendFactor(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlBlendFactor) {
	C.gk_v_u(o(a), selSetSourceAlphaBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetDestinationAlphaBlendFactor(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlBlendFactor) {
	C.gk_v_u(o(a), selSetDestinationAlphaBlendFactor, C.gk_u(v))
}
func renderPipelineColorAttachmentDescriptorSetAlphaBlendOperation(a mtl4RenderPipelineColorAttachmentDescriptor, v mtlBlendOperation) {
	C.gk_v_u(o(a), selSetAlphaBlendOperation, C.gk_u(v))
}
func compilerNewComputePipelineStateWithDescriptor(c mtl4Compiler, d mtl4ComputePipelineDescriptor, options id, err *id) mtlComputePipelineState {
	return mtlComputePipelineState(C.gk_o_oop(o(c), selNewComputePipelineStateWithDescriptor, o(d), o(options), (*C.gk_obj)(unsafe.Pointer(err))))
}
func compilerNewRenderPipelineStateWithDescriptor(c mtl4Compiler, d mtl4RenderPipelineDescriptor, options id, err *id) mtlRenderPipelineState {
	return mtlRenderPipelineState(C.gk_o_oop(o(c), selNewRenderPipelineStateWithDescriptor, o(d), o(options), (*C.gk_obj)(unsafe.Pointer(err))))
}
func computePipelineStateMaxTotalThreadsPerThreadgroup(p mtlComputePipelineState) uint {
	return uint(C.gk_u_(o(p), selMaxTotalThreadsPerThreadgroup))
}
func computePipelineStateReflection(p mtlComputePipelineState) id {
	return id(C.gk_o(o(p), selReflection))
}
func renderPipelineStateReflection(p mtlRenderPipelineState) id {
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
func bindingType(b id) mtlBindingType {
	return mtlBindingType(C.gk_u_(o(b), selType))
}
func bindingIsUsed(b id) bool {
	return C.gk_b(o(b), selIsUsed) != 0
}
func bufferBindingBufferDataSize(b id) uint {
	return uint(C.gk_u_(o(b), selBufferDataSize))
}
func depthStencilDescriptorSetDepthCompareFunction(d mtlDepthStencilDescriptor, v mtlCompareFunction) {
	C.gk_v_u(o(d), selSetDepthCompareFunction, C.gk_u(v))
}
func depthStencilDescriptorSetDepthWriteEnabled(d mtlDepthStencilDescriptor, v bool) {
	C.gk_v_u(o(d), selSetDepthWriteEnabled, b2u(v))
}

// CAMetalLayer, CAMetalDrawable, NSView, NSWindow.

func metalLayerLayer() caMetalLayer {
	return caMetalLayer(C.gk_o(o(objcGetClass("CAMetalLayer")), selLayer))
}
func metalLayerSetDevice(l caMetalLayer, d mtlDevice) {
	C.gk_v_o(o(l), selSetDevice, o(d))
}
func metalLayerSetPixelFormat(l caMetalLayer, v mtlPixelFormat) {
	C.gk_v_u(o(l), selSetPixelFormat, C.gk_u(v))
}
func metalLayerSetFramebufferOnly(l caMetalLayer, v bool) {
	C.gk_v_u(o(l), selSetFramebufferOnly, b2u(v))
}
func metalLayerSetDisplaySyncEnabled(l caMetalLayer, v bool) {
	C.gk_v_u(o(l), selSetDisplaySyncEnabled, b2u(v))
}
func metalLayerSetContentsScale(l caMetalLayer, v float64) {
	C.gk_v_d(o(l), selSetContentsScale, C.double(v))
}
func metalLayerSetDrawableSize(l caMetalLayer, width, height float64) {
	C.gk_v_cgsize(o(l), selSetDrawableSize, C.CGSize{width: C.CGFloat(width), height: C.CGFloat(height)})
}
func metalLayerDrawableSize(l caMetalLayer) (width, height float64) {
	s := C.gk_size(o(l), selDrawableSize)

	return float64(s.width), float64(s.height)
}
func metalLayerNextDrawable(l caMetalLayer) mtlDrawable {
	return mtlDrawable(C.gk_o(o(l), selNextDrawable))
}
func drawableTexture(d mtlDrawable) mtlTexture {
	return mtlTexture(C.gk_o(o(d), selTexture))
}
func drawablePresent(d mtlDrawable) {
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

func counterHeapDescriptorSetType(d mtl4CounterHeapDescriptor, v mtl4CounterHeapType) {
	C.gk_v_u(o(d), selSetType, C.gk_u(v))
}
func counterHeapDescriptorSetCount(d mtl4CounterHeapDescriptor, v uint) {
	C.gk_v_u(o(d), selSetCount, C.gk_u(v))
}
func counterHeapInvalidateCounterRange(h mtl4CounterHeap, location, length uint) {
	C.gk_v_r(o(h), selInvalidateCounterRange, C.NSRange{location: C.NSUInteger(location), length: C.NSUInteger(length)})
}
func counterHeapResolveCounterRange(h mtl4CounterHeap, location, length uint) id {
	return id(C.gk_o_r(o(h), selResolveCounterRange, C.NSRange{location: C.NSUInteger(location), length: C.NSUInteger(length)}))
}

// Recording: MTL4CommandAllocator, MTL4CommandBuffer, the render pass descriptors.

func commandAllocatorReset(a mtl4CommandAllocator) {
	C.gk_v(o(a), selReset)
}
func commandBufferBeginCommandBufferWithAllocator(b mtl4CommandBuffer, a mtl4CommandAllocator) {
	C.gk_v_o(o(b), selBeginCommandBufferWithAllocator, o(a))
}
func commandBufferEndCommandBuffer(b mtl4CommandBuffer) {
	C.gk_v(o(b), selEndCommandBuffer)
}
func commandBufferComputeCommandEncoder(b mtl4CommandBuffer) mtl4ComputeCommandEncoder {
	return mtl4ComputeCommandEncoder(C.gk_o(o(b), selComputeCommandEncoder))
}
func commandBufferRenderCommandEncoderWithDescriptor(b mtl4CommandBuffer, d mtl4RenderPassDescriptor) mtl4RenderCommandEncoder {
	return mtl4RenderCommandEncoder(C.gk_o_o(o(b), selRenderCommandEncoderWithDescriptor, o(d)))
}
func renderPassDescriptorColorAttachments(d mtl4RenderPassDescriptor) id {
	return id(C.gk_o(o(d), selColorAttachments))
}
func renderPassDescriptorDepthAttachment(d mtl4RenderPassDescriptor) mtlRenderPassDepthAttachmentDescriptor {
	return mtlRenderPassDepthAttachmentDescriptor(C.gk_o(o(d), selDepthAttachment))
}
func renderPassAttachmentDescriptorSetTexture(a mtlRenderPassAttachmentDescriptor, t mtlTexture) {
	C.gk_v_o(o(a), selSetTexture, o(t))
}
func renderPassAttachmentDescriptorSetResolveTexture(a mtlRenderPassAttachmentDescriptor, t mtlTexture) {
	C.gk_v_o(o(a), selSetResolveTexture, o(t))
}
func renderPassAttachmentDescriptorSetLoadAction(a mtlRenderPassAttachmentDescriptor, v mtlLoadAction) {
	C.gk_v_u(o(a), selSetLoadAction, C.gk_u(v))
}
func renderPassAttachmentDescriptorSetStoreAction(a mtlRenderPassAttachmentDescriptor, v mtlStoreAction) {
	C.gk_v_u(o(a), selSetStoreAction, C.gk_u(v))
}
func renderPassColorAttachmentDescriptorSetClearColor(a mtlRenderPassColorAttachmentDescriptor, red, green, blue, alpha float64) {
	C.gk_v_clear(o(a), selSetClearColor, C.MTLClearColor{
		red: C.double(red), green: C.double(green), blue: C.double(blue), alpha: C.double(alpha),
	})
}
func renderPassDepthAttachmentDescriptorSetClearDepth(a mtlRenderPassDepthAttachmentDescriptor, v float64) {
	C.gk_v_d(o(a), selSetClearDepth, C.double(v))
}
func renderPassDepthAttachmentDescriptorSetDepthResolveFilter(a mtlRenderPassDepthAttachmentDescriptor, v mtlMultisampleDepthResolveFilter) {
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
func commandEncoderUpdateFence(e id, fence mtlFence, stages mtlStages) {
	C.gk_v_ou(o(e), selUpdateFenceAfterEncoderStages, o(fence), C.gk_u(stages))
}
func commandEncoderWaitForFence(e id, fence mtlFence, stages mtlStages) {
	C.gk_v_ou(o(e), selWaitForFenceBeforeEncoderStages, o(fence), C.gk_u(stages))
}

func computeCommandEncoderSetArgumentTable(e mtl4ComputeCommandEncoder, t mtl4ArgumentTable) {
	C.gk_v_o(o(e), selSetArgumentTable, o(t))
}
func computeCommandEncoderSetComputePipelineState(e mtl4ComputeCommandEncoder, p mtlComputePipelineState) {
	C.gk_v_o(o(e), selSetComputePipelineState, o(p))
}
func computeCommandEncoderDispatchThreadgroups(e mtl4ComputeCommandEncoder, groups, threads [3]uint) {
	C.gk_v_ss(o(e), selDispatchThreadgroupsThreadsPerThreadgroup, mtlSize(groups), mtlSize(threads))
}
func computeCommandEncoderDispatchThreadgroupsWithIndirectBuffer(e mtl4ComputeCommandEncoder, address uint64, threads [3]uint) {
	C.gk_v_us(o(e), selDispatchThreadgroupsWithIndirectBuffer, C.uint64_t(address), mtlSize(threads))
}
func computeCommandEncoderCopyFromBufferToBuffer(e mtl4ComputeCommandEncoder, src mtlBuffer, srcOffset uint, dst mtlBuffer, dstOffset, size uint) {
	C.gk_v_ououu(o(e), selCopyFromBufferToBuffer, o(src), C.gk_u(srcOffset), o(dst), C.gk_u(dstOffset), C.gk_u(size))
}
func computeCommandEncoderCopyFromBufferToTexture(e mtl4ComputeCommandEncoder, src mtlBuffer, srcOffset, bytesPerRow, bytesPerImage uint, size [3]uint,
	dst mtlTexture, slice, level uint, origin [3]uint) {
	C.gk_v_ouuusouuz(o(e), selCopyFromBufferToTexture, o(src), C.gk_u(srcOffset), C.gk_u(bytesPerRow),
		C.gk_u(bytesPerImage), mtlSize(size), o(dst), C.gk_u(slice), C.gk_u(level), mtlOrigin(origin))
}
func computeCommandEncoderCopyFromTextureToBuffer(e mtl4ComputeCommandEncoder, src mtlTexture, slice, level uint, origin, size [3]uint,
	dst mtlBuffer, dstOffset, bytesPerRow, bytesPerImage uint) {
	C.gk_v_ouuzsouuu(o(e), selCopyFromTextureToBuffer, o(src), C.gk_u(slice), C.gk_u(level), mtlOrigin(origin),
		mtlSize(size), o(dst), C.gk_u(dstOffset), C.gk_u(bytesPerRow), C.gk_u(bytesPerImage))
}
func computeCommandEncoderCopyFromTextureToTexture(e mtl4ComputeCommandEncoder, src mtlTexture, srcSlice, srcLevel uint, srcOrigin, size [3]uint,
	dst mtlTexture, dstSlice, dstLevel uint, dstOrigin [3]uint) {
	C.gk_v_ouuzsouuz(o(e), selCopyFromTextureToTexture, o(src), C.gk_u(srcSlice), C.gk_u(srcLevel),
		mtlOrigin(srcOrigin), mtlSize(size), o(dst), C.gk_u(dstSlice), C.gk_u(dstLevel), mtlOrigin(dstOrigin))
}
func computeCommandEncoderWriteTimestampWithGranularity(e mtl4ComputeCommandEncoder, granularity mtl4TimestampGranularity, heap mtl4CounterHeap, index uint) {
	C.gk_v_uou(o(e), selWriteTimestampWithGranularityIntoHeap, C.gk_u(granularity), o(heap), C.gk_u(index))
}

func renderCommandEncoderSetArgumentTable(e mtl4RenderCommandEncoder, t mtl4ArgumentTable, stages mtlRenderStages) {
	C.gk_v_ou(o(e), selSetArgumentTableAtStages, o(t), C.gk_u(stages))
}
func renderCommandEncoderSetViewport(e mtl4RenderCommandEncoder, x, y, width, height, znear, zfar float64) {
	C.gk_v_viewport(o(e), selSetViewport, C.MTLViewport{originX: C.double(x), originY: C.double(y),
		width: C.double(width), height: C.double(height), znear: C.double(znear), zfar: C.double(zfar)})
}
func renderCommandEncoderSetScissorRect(e mtl4RenderCommandEncoder, x, y, width, height uint) {
	C.gk_v_scissor(o(e), selSetScissorRect, C.MTLScissorRect{x: C.NSUInteger(x), y: C.NSUInteger(y),
		width: C.NSUInteger(width), height: C.NSUInteger(height)})
}
func renderCommandEncoderSetColorStoreAction(e mtl4RenderCommandEncoder, action mtlStoreAction, index uint) {
	C.gk_v_uu(o(e), selSetColorStoreActionAtIndex, C.gk_u(action), C.gk_u(index))
}
func renderCommandEncoderSetDepthStoreAction(e mtl4RenderCommandEncoder, action mtlStoreAction) {
	C.gk_v_u(o(e), selSetDepthStoreAction, C.gk_u(action))
}
func renderCommandEncoderSetRenderPipelineState(e mtl4RenderCommandEncoder, p mtlRenderPipelineState) {
	C.gk_v_o(o(e), selSetRenderPipelineState, o(p))
}
func renderCommandEncoderSetDepthStencilState(e mtl4RenderCommandEncoder, s mtlDepthStencilState) {
	C.gk_v_o(o(e), selSetDepthStencilState, o(s))
}
func renderCommandEncoderSetCullMode(e mtl4RenderCommandEncoder, v mtlCullMode) {
	C.gk_v_u(o(e), selSetCullMode, C.gk_u(v))
}
func renderCommandEncoderSetFrontFacingWinding(e mtl4RenderCommandEncoder, v mtlWinding) {
	C.gk_v_u(o(e), selSetFrontFacingWinding, C.gk_u(v))
}
func renderCommandEncoderSetDepthBias(e mtl4RenderCommandEncoder, bias, slope, clamp float32) {
	C.gk_v_fff(o(e), selSetDepthBiasSlopeScaleClamp, C.float(bias), C.float(slope), C.float(clamp))
}
func renderCommandEncoderDrawPrimitives(e mtl4RenderCommandEncoder, primitive mtlPrimitiveType, vertexStart, vertexCount, instanceCount, baseInstance uint) {
	C.gk_v_uuuuu(o(e), selDrawPrimitives, C.gk_u(primitive), C.gk_u(vertexStart), C.gk_u(vertexCount),
		C.gk_u(instanceCount), C.gk_u(baseInstance))
}
func renderCommandEncoderDrawIndexedPrimitivesIndexCount(e mtl4RenderCommandEncoder, primitive mtlPrimitiveType, indexCount uint, indexType mtlIndexType, indexBuffer uint64, indexBufferLength, instanceCount uint, baseVertex int, baseInstance uint) {
	C.gk_v_uuuuuuuu(o(e), selDrawIndexedPrimitivesIndexCount, C.gk_u(primitive), C.gk_u(indexCount),
		C.gk_u(indexType), C.gk_u(indexBuffer), C.gk_u(indexBufferLength), C.gk_u(instanceCount),
		C.NSInteger(baseVertex), C.gk_u(baseInstance))
}
func renderCommandEncoderDrawIndexedPrimitivesIndirectBuffer(e mtl4RenderCommandEncoder, primitive mtlPrimitiveType, indexType mtlIndexType, indexBuffer uint64, indexBufferLength uint, indirectBuffer uint64) {
	C.gk_v_uuuuu(o(e), selDrawIndexedPrimitivesIndirectBuffer, C.gk_u(primitive), C.gk_u(indexType),
		C.gk_u(indexBuffer), C.gk_u(indexBufferLength), C.gk_u(indirectBuffer))
}
func renderCommandEncoderWriteTimestampWithGranularity(e mtl4RenderCommandEncoder, granularity mtl4TimestampGranularity, stage mtlRenderStages, heap mtl4CounterHeap, index uint) {
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
