//go:build darwin && cgo

package metal

// Metal enums use int for NSInteger and uint for NSUInteger, as defined by the SDK.
// Option sets use distinct types so unrelated flags cannot be mixed.

type mtl4BlendState int

type mtl4CounterHeapType int

type mtl4ShaderReflection uint

type mtl4TimestampGranularity int

type mtlArgumentBuffersTier uint

type mtlBindingType int

type mtlBlendFactor uint

type mtlBlendOperation uint

type mtlColorWriteMask uint

type mtlCompareFunction uint

type mtlCullMode uint

type mtlDataType uint

type mtlGPUFamily int

type mtlIndexType uint

type mtlLanguageVersion uint

type mtlLoadAction uint

type mtlMultisampleDepthResolveFilter uint

type mtlPrimitiveType uint

type mtlRenderStages uint

type mtlResourceOptions uint

type mtlSamplerAddressMode uint

type mtlSamplerMinMagFilter uint

type mtlSamplerMipFilter uint

type mtlStages uint

type mtlStorageMode uint

type mtlStoreAction uint

type mtlTextureType uint

type mtlTextureUsage uint

type mtlWinding uint

type mtlPixelFormat uint
