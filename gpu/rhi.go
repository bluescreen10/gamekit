// Package gpu is gamekit's rendering hardware interface: a thin, bindless,
// GPU-driven abstraction over a modern explicit API. It is modelled on
// Sebastian Aaltonen's "no graphics API" proposal and maps directly onto
// Vulkan 1.3 (the backend targets apiVersion 1.3; the features it relies on —
// buffer_device_address, descriptor_indexing, dynamic_rendering,
// synchronization2 — are core since 1.3, so no extension juggling on 1.4).
// Backends include Vulkan (via KosmicKrisp on macOS) and native Metal 3.
//
// The design is deliberately minimal:
//
//   - Memory is a flat allocator (Alloc) returning buffers that expose a 64-bit
//     device address (BDA) and, for host memory, a persistently-mapped pointer.
//     Shaders read/write data through raw pointers, not descriptor sets.
//   - Textures live in one global bindless heap; a Texture is a 32-bit index a
//     shader uses to sample. There are no per-draw texture bindings.
//   - All per-draw/dispatch data flows through a single 64-bit "root" pointer
//     (a push constant holding the address of a struct in mapped memory).
//   - There are no render passes (dynamic rendering), no input layouts / vertex
//     buffers (shaders load geometry from buffers), no resource-layout
//     transitions, and barriers are queue+stage granularity with no resource
//     lists.
package gpu

import "unsafe"

// ----------------------------------------------------------------------------
// Resources
// ----------------------------------------------------------------------------

// Resource interfaces hide native handles. Nil means no resource; check for nil
// before calling a method. Use each resource only with its creating backend.
// After destruction, only IsValid and repeated destruction calls are permitted.
// IsValid returns false after destruction or after a fence is consumed by a wait.

// Buffer is a GPU allocation. Pass it only to its creating backend.
// A nil Buffer represents no allocation.
type Buffer interface {
	IsValid() bool
	// Addr is the device address that shaders use to access the buffer.
	Addr() uint64
	// Ptr is mapped, coherent CPU memory for MemoryHost, or nil for MemoryDevice.
	Ptr() unsafe.Pointer
	Size() uint64
	// Write copies data at byteOffset. Empty writes do nothing. Other writes panic
	// if the buffer is not mapped or the range exceeds Size.
	Write(data []byte, byteOffset uint64)
}

// Texture is a texture or view. Index is its bindless shader slot.
// A nil Texture represents no texture, including an absent resolve attachment.
type Texture interface {
	IsValid() bool
	Index() uint32
}

// Sampler selects texture sample behavior. Index is its bindless shader slot.
type Sampler interface {
	IsValid() bool
	Index() uint32
}

// Pipeline is a compute or graphics pipeline.
type Pipeline interface {
	IsValid() bool
}

// Swapchain is a presentation chain. AcquireNext returns its backbuffer.
type Swapchain interface {
	IsValid() bool
}

// Surface is a backend-specific presentation surface created from a native
// window or view.
type Surface uintptr

// IsValid reports whether the surface has a native backend handle.
func (s Surface) IsValid() bool {
	return s != 0
}

// SurfaceTarget is a native drawable that a Backend can turn into a Surface.
// UI packages implement this interface without needing to know which GPU API
// will consume the target.
type SurfaceTarget interface {
	NativeSurface() NativeSurface
}

// NativeSurface describes the platform object underlying a SurfaceTarget.
// Display is used by window systems such as X11 and is zero otherwise.
type NativeSurface struct {
	Kind    NativeSurfaceKind
	Handle  uintptr
	Display uintptr
}

// NativeSurfaceKind identifies the native window-system object in a target.
type NativeSurfaceKind uint8

const (
	NativeSurfaceUnknown NativeSurfaceKind = iota
	NativeSurfaceCocoaWindow
	NativeSurfaceCocoaView
	NativeSurfaceWin32Window
	NativeSurfaceXlibWindow
)

// SwapchainDescriptor describes a window's presentation chain.
type SwapchainDescriptor struct {
	Width, Height uint32
	// Format is the backbuffers' format, one of Backend.SwapchainFormats for the
	// surface. An sRGB format lets shaders write linear colour: the hardware encodes it
	// as it stores, and decodes it to blend. FormatUndefined picks the backend's
	// default, an 8-bit unorm format.
	Format Format
	// PresentMode is when a presented frame reaches the display; the zero value waits
	// for its refresh.
	PresentMode PresentMode
}

// PresentMode is when a presented frame reaches the display.
type PresentMode uint8

const (
	// PresentVSync shows each frame at the display's next refresh, never tearing. A
	// frame that finishes early waits, which also paces rendering to the refresh rate.
	PresentVSync PresentMode = iota
	// PresentImmediate shows each frame as soon as it is presented, without waiting for
	// a refresh: frames run as fast as the GPU can draw them, and may tear. Where the
	// display cannot show a frame mid-refresh, the backend falls back to
	// PresentMailbox, and failing that, to PresentVSync.
	PresentImmediate
	// PresentMailbox shows, at each refresh, the newest frame finished by then, never
	// tearing: frames run as fast as the GPU can draw them, and those a newer one
	// overtakes before a refresh are never shown. Where the backend has no such mode —
	// Metal has none — it falls back to PresentVSync.
	PresentMailbox
)

// Fence is a submission completion token. A nil Fence needs no wait.
type Fence interface {
	IsValid() bool
}

// QueryPool is a set of GPU timestamp queries (for profiling pass/frame time).
// CreateTimestampPool returns nil when timestamps are unavailable.
type QueryPool interface {
	IsValid() bool
}

// ----------------------------------------------------------------------------
// Enums
// ----------------------------------------------------------------------------

// MemoryType selects where an allocation lives and whether it is CPU-mapped.
type MemoryType uint8

const (
	// MemoryHost is CPU-visible, coherent, persistently mapped (ReBAR-style):
	// the buffer exposes both Addr and Ptr. Ideal for per-frame upload data.
	MemoryHost MemoryType = iota
	// MemoryDevice is GPU-only: Addr but no Ptr. Populate via CopyBuffer.
	MemoryDevice
)

// Format is a texel/attachment format. This is the full catalog the RHI knows
// about across every platform it targets; no single Backend supports all of
// them; call Backend.SupportedFormats to find out what a given backend accepts.
type Format uint16

const (
	FormatUndefined Format = iota

	// 8-bit per channel, unsigned normalized.
	FormatR8Unorm
	FormatRG8Unorm
	FormatRGBA8Unorm
	FormatBGRA8Unorm
	FormatRGBA8Srgb
	FormatBGRA8Srgb

	// 8-bit per channel, signed normalized.
	FormatR8Snorm
	FormatRG8Snorm
	FormatRGBA8Snorm

	// 8-bit per channel, integer.
	FormatR8Uint
	FormatRG8Uint
	FormatRGBA8Uint
	FormatR8Sint
	FormatRG8Sint
	FormatRGBA8Sint

	// 16-bit per channel, unsigned/signed normalized.
	FormatR16Unorm
	FormatRG16Unorm
	FormatRGBA16Unorm
	FormatR16Snorm
	FormatRG16Snorm
	FormatRGBA16Snorm

	// 16-bit per channel, integer.
	FormatR16Uint
	FormatRG16Uint
	FormatRGBA16Uint
	FormatR16Sint
	FormatRG16Sint
	FormatRGBA16Sint

	// 16-bit per channel, float.
	FormatR16F
	FormatRG16F
	FormatRGBA16F

	// 32-bit per channel, integer.
	FormatR32Uint
	FormatRG32Uint
	FormatRGBA32Uint
	FormatR32Sint
	FormatRG32Sint
	FormatRGBA32Sint

	// 32-bit per channel, float.
	FormatR32F
	FormatRG32F
	FormatRGBA32F

	// Packed.
	FormatRGB10A2Unorm
	FormatRGB10A2Uint
	FormatRG11B10F
	FormatRGB9E5F

	// Depth / stencil.
	FormatDepth16Unorm
	FormatDepth32F
	FormatDepth24Stencil8
	FormatDepth32FStencil8
	FormatStencil8

	// Desktop block compression (BC / DirectX Texture Compression).
	FormatBC1Unorm
	FormatBC1Srgb
	FormatBC3Unorm
	FormatBC3Srgb
	FormatBC4Unorm
	FormatBC4Snorm
	FormatBC5Unorm
	FormatBC5Snorm
	FormatBC6HFloat
	FormatBC6HUFloat
	FormatBC7Unorm
	FormatBC7Srgb

	// Mobile block compression: ETC2/EAC.
	FormatETC2RGB8Unorm
	FormatETC2RGB8Srgb
	FormatETC2RGBA8Unorm
	FormatETC2RGBA8Srgb
	FormatEACR11Unorm
	FormatEACRG11Unorm

	// Mobile block compression: ASTC LDR.
	FormatASTC4x4Unorm
	FormatASTC4x4Srgb
	FormatASTC5x5Unorm
	FormatASTC5x5Srgb
	FormatASTC6x6Unorm
	FormatASTC6x6Srgb
	FormatASTC8x8Unorm
	FormatASTC8x8Srgb
	FormatASTC10x10Unorm
	FormatASTC10x10Srgb
	FormatASTC12x12Unorm
	FormatASTC12x12Srgb
)

// IsDepth reports whether f carries a depth aspect.
func (f Format) IsDepth() bool {
	switch f {
	case FormatDepth16Unorm, FormatDepth32F, FormatDepth24Stencil8, FormatDepth32FStencil8:
		return true
	default:
		return false
	}
}

// IsStencil reports whether f carries a stencil aspect.
func (f Format) IsStencil() bool {
	switch f {
	case FormatDepth24Stencil8, FormatDepth32FStencil8, FormatStencil8:
		return true
	default:
		return false
	}
}

// TextureUsage is a bitmask of intended uses.
type TextureUsage uint16

const (
	TextureSampled      TextureUsage = 1 << iota // sampled in shaders (bindless heap)
	TextureStorage                               // read/write storage image
	TextureRenderTarget                          // color attachment
	TextureDepth                                 // depth/stencil attachment
	TextureTransfer                              // copy src/dst
	// TextureTransient marks an attachment that lives only within a render pass: each
	// pass using it clears it or does not care what it held (LoadClear or LoadDontCare),
	// and keeps nothing of it (StoreDontCare) — though a multisampled one may resolve
	// as the pass ends. It cannot be sampled, written by shaders or copied, and no
	// Barrier may split a pass it is bound to. In return a tile-based GPU never gives
	// it memory: Metal makes it memoryless on Apple GPUs, Vulkan backs it with lazily
	// allocated memory where the device has some. Elsewhere it is an ordinary
	// attachment that the same rules apply to.
	TextureTransient
)

// TextureKind selects the view dimension.
type TextureKind uint8

const (
	Texture2D TextureKind = iota
	Texture2DArray
	TextureCube
	TextureCubeArray
	Texture3D
)

// Stage is a coarse pipeline stage for barriers (synchronization2). Combine with OR.
type Stage uint32

const (
	StageNone        Stage = 0
	StageIndirect    Stage = 1 << iota // indirect command consumption
	StageVertex                        // vertex shading + attribute/index fetch
	StageFragment                      // fragment shading
	StageColorOutput                   // color attachment writes
	StageDepth                         // depth/stencil test+write
	StageCompute                       // compute shading
	StageTransfer                      // copies/blits
	StageAll         Stage = 0xFFFFFFFF
)

// BarrierFlags carry the rare extra hazards that stage granularity can't express.
type BarrierFlags uint32

const (
	// HazardDescriptors: descriptor-heap contents written this batch must be
	// visible to subsequent shader access (Aaltonen's only hazard flag).
	HazardDescriptors BarrierFlags = 1 << iota
)

// Topology is the primitive assembly mode.
// IndexType is the width of the entries in an index buffer.
//
// 8-bit indices are deliberately absent. Vulkan can do them (VK_KHR_index_type_uint8)
// but Metal's MTLIndexType has only 16- and 32-bit, so exposing them would mean an RHI
// value that cannot be implemented on every backend.
type IndexType uint8

const (
	// IndexUint32 is first so it is the zero value: it is what every existing index
	// buffer uses, so a forgotten field defaults to reading them correctly rather
	// than misinterpreting 32-bit data as 16-bit.
	IndexUint32 IndexType = iota
	IndexUint16
)

// Size is the width of one index in bytes.
func (t IndexType) Size() uint32 {
	if t == IndexUint16 {
		return 2
	}
	return 4
}

type Topology uint8

const (
	TopologyTriangles Topology = iota
	TopologyTriangleStrip
	TopologyLines
	TopologyPoints
)

// CullMode selects face culling.
type CullMode uint8

const (
	CullNone CullMode = iota
	CullBack
	CullFront
)

// CompareOp is a depth/stencil comparison.
type CompareOp uint8

const (
	CompareNever CompareOp = iota
	CompareLess
	CompareEqual
	CompareLessEqual
	CompareGreater
	CompareNotEqual
	CompareGreaterEqual
	CompareAlways
)

// LoadOp / StoreOp control attachment load/store in dynamic rendering.
type LoadOp uint8

const (
	LoadDontCare LoadOp = iota
	LoadClear
	LoadKeep
)

type StoreOp uint8

const (
	StoreDontCare StoreOp = iota
	StoreKeep
)

// ----------------------------------------------------------------------------
// Descriptors
// ----------------------------------------------------------------------------

// TextureDescriptor describes a texture to create. It is registered into the bindless
// heap when it has TextureSampled usage; Texture.Index returns the heap slot.
type TextureDescriptor struct {
	Kind          TextureKind
	Width, Height uint32
	Depth         uint32 // 3D depth; else 1
	Layers        uint32 // array layers / cube faces; else 1
	Mips          uint32 // 0 => 1
	Format        Format
	Usage         TextureUsage
	// Samples is how many samples each pixel holds (MSAA); 0 and 1 both mean one. A
	// multisampled texture is a 2D render target or depth buffer with one mip. It cannot
	// be filtered or copied. A render pass can resolve it into a single-sample texture
	// (see ColorAttachment.ResolveTexture); or, with TextureSampled usage, a shader reads
	// its samples one at a time, by texelFetch through the heap declared as
	// texture2DMS (texture2d_ms on Metal) — at the same index as any other texture.
	Samples uint8
	Label   string
}

// SamplerDescriptor describes a bindless sampler.
type SamplerDescriptor struct {
	MinLinear, MagLinear, MipLinear bool
	AddressU, AddressV, AddressW    AddressMode
	Compare                         CompareOp // non-CompareNever => comparison sampler (shadows)
	MaxAnisotropy                   uint8
	Label                           string
}

type AddressMode uint8

const (
	AddressClamp AddressMode = iota
	AddressRepeat
	AddressMirror
)

// BlendState is optional per-target blending (kept minimal, separate from PSO
// permutations per the design).
type BlendState struct {
	Enable                      bool
	SrcColor, DstColor, ColorOp BlendFactorOp
	SrcAlpha, DstAlpha, AlphaOp BlendFactorOp
	WriteMask                   uint8 // bit0..3 = RGBA; 0 => all
}

type BlendFactorOp struct {
	Src, Dst BlendFactor
	Op       BlendOp
}

type BlendFactor uint8

const (
	BlendZero BlendFactor = iota
	BlendOne
	BlendSrcAlpha
	BlendOneMinusSrcAlpha
	BlendDstAlpha
	BlendOneMinusDstAlpha
)

type BlendOp uint8

const (
	BlendAdd BlendOp = iota
	BlendSubtract
	BlendReverseSubtract
	BlendMin
	BlendMax
)

// PipelineDescriptor is the minimal graphics pipeline description: shaders + the
// formats/topology that actually affect codegen. Viewport, scissor, blend
// constants and stencil ref are dynamic; per-draw data comes via the root pointer.
// Shader bytes are backend-specific: Vulkan accepts SPIR-V; Metal accepts MSL,
// metallib, or a precompiled artifact containing local threadgroup metadata.
type PipelineDescriptor struct {
	VertexShader []byte
	// FragmentShader may be nil, for a pipeline that writes depth and nothing else — a
	// shadow map or a depth prepass. That is not the same as binding one that outputs
	// nothing: a fragment stage still runs per fragment, and against a pass that has a
	// colour attachment it writes an UNDEFINED value rather than leaving it alone.
	FragmentShader []byte
	VertexEntry    string // empty => backend default (Vulkan: "main", Metal: "main0")
	FragmentEntry  string // empty => backend default (Vulkan: "main", Metal: "main0")

	Topology     Topology
	ColorFormats []Format // dynamic-rendering color attachment formats
	DepthFormat  Format   // FormatUndefined => no depth
	// Samples must match the sample count of the attachments the pipeline draws into;
	// 0 and 1 both mean one.
	Samples uint8

	CullMode     CullMode
	FrontFaceCW  bool // front face is clockwise (vs the default counter-clockwise)
	DepthTest    bool
	DepthWrite   bool
	DepthCompare CompareOp
	Blend        []BlendState // per color target; nil => opaque

	// Constants sets the shaders' specialization constants, as for a compute pipeline.
	// It specializes both stages: a stage ignores any constant it does not declare.
	Constants map[string]float64

	Label string
}

type ComputePipelineDescriptor struct {
	Shader []byte
	Entry  string // empty => backend default (Vulkan: "main", Metal: "main0")
	// WorkgroupSize is the number of invocations in one workgroup, in x, y and z: the
	// local_size the shader declares (`layout(local_size_x = 64) in;` in GLSL). Every
	// dimension must be at least 1. Dispatch counts workgroups, not invocations.
	//
	// The pipeline must state it because Metal does not keep it in a compiled kernel:
	// Metal takes the size with each dispatch. Vulkan reads it from the shader, and
	// creating the pipeline panics if the two sizes are different.
	WorkgroupSize [3]uint32
	// Constants sets the shader's specialization constants as the pipeline is created —
	// `layout(constant_id = N) const` in GLSL, `[[function_constant(N)]]` in Metal — much
	// as WebGPU's pipeline constants do. The value is fixed for the pipeline's lifetime, so
	// the compiler folds it and drops the branches it rules out: one shader source serves
	// several pipelines, each paying only for the work it keeps. A constant left out
	// keeps the default its shader declares. Unlike WebGPU, a constant the shader does
	// not declare is ignored rather than an error — so one set of constants can serve every pipeline a pass creates, whether
	// or not each shader uses them.
	//
	// A key is the constant's name in the shader, or its ID in decimal ("3"). A name
	// needs the shader compiled with its debug names, which an optimizing compile (glslc
	// -O) strips; an ID works either way. A value is converted to the type the constant
	// is declared with: a bool is whether it is non-zero, an int or uint must be a whole
	// number in the type's range, and a float is rounded to 32 bits. Creating the
	// pipeline panics for a value its constant's type cannot hold.
	Constants map[string]float64
	Label     string
}

// RenderTargets is the dynamic-rendering attachment set for BeginRenderPass.
type RenderTargets struct {
	Color []ColorAttachment
	Depth *DepthAttachment
}

type ColorAttachment struct {
	Texture Texture
	Load    LoadOp
	Store   StoreOp
	Clear   [4]float32
	// ResolveTexture, when valid, receives the average of each pixel's samples at the end
	// of the pass. Texture must then be multisampled, ResolveTexture a single-sample
	// texture of the same size and format, and Store StoreDontCare: the resolve is the
	// samples' last use. Metal does not reliably resolve samples it also stores — with a
	// second attachment in the pass, an M-series GPU left the resolve texture unwritten
	// in 39 of 40 frames — so no backend accepts it.
	ResolveTexture Texture
}

type DepthAttachment struct {
	Texture Texture
	Load    LoadOp
	Store   StoreOp
	Clear   float32
	// ResolveTexture, when valid, receives one depth per pixel at the end of the pass:
	// sample zero's, the one resolve every device supports. As for a colour attachment,
	// Texture must then be multisampled, ResolveTexture a single-sample texture of the
	// same size and format, and Store StoreDontCare.
	ResolveTexture Texture
	// ReadOnly binds the depth buffer for testing only (no writes), so the same image
	// may be sampled from the bindless heap during the pass. A pipeline used with it
	// must have DepthWrite false.
	ReadOnly bool
}

// ----------------------------------------------------------------------------
// Backend
// ----------------------------------------------------------------------------

// Backend is a device implementation (Vulkan or Metal). All
// creation is immediate; there are no descriptor sets or pipeline layouts to
// manage — resources are reached through addresses and the bindless heap.
// Callers own resource destruction. Wait for GPU completion, then destroy each
// resource before its final Backend.Destroy call. Destroy does not release caller-owned resources.
type Backend interface {
	// Init obtains the device, its queues and the bindless heap. A backend is one
	// instance per process (see Instance), shared by everything that uses it: each Init
	// adds a user, which a Destroy removes, and the device lives until the last one is
	// gone. Only the first Init does any work.
	Init() error
	// SupportedFormats lists the Format values this backend can create textures
	// with (a subset of the full Format catalog above). Query it before choosing
	// a format for an asset pipeline or render target that must run on every
	// backend.
	SupportedFormats() []Format
	// Memory.
	Alloc(size uint64, mem MemoryType, label string) Buffer
	Free(Buffer)

	// Bindless resources.
	CreateTexture(TextureDescriptor) Texture
	DestroyTexture(Texture)
	// TextureView registers an additional view (e.g. a single array layer / mip) into
	// the heap and returns its index. It is sampled; a view of exactly one mip of a
	// texture with TextureStorage usage can be written too, at the same index — which
	// is how a compute shader writes one level of a mip chain.
	TextureView(t Texture, kind TextureKind, baseMip, mipCount, baseLayer, layerCount uint32) Texture
	CreateSampler(SamplerDescriptor) Sampler
	DestroySampler(Sampler)

	// Pipelines.
	CreateComputePipeline(ComputePipelineDescriptor) Pipeline
	CreateGraphicsPipeline(PipelineDescriptor) Pipeline
	DestroyPipeline(Pipeline)

	// Presentation. A Surface is created from a native SurfaceTarget, then used
	// to query formats and create a swapchain.
	CreateSurface(target SurfaceTarget) (Surface, error)
	// SwapchainFormats lists the backbuffer formats the surface can present, for
	// SwapchainDescriptor.Format.
	SwapchainFormats(surface Surface) []Format
	// CreateSwapchain fails when desc.Format is not one of SwapchainFormats.
	CreateSwapchain(surface Surface, desc SwapchainDescriptor) (Swapchain, error)
	// DestroySwapchain releases the swapchain and its backbuffers. Accepts nil.
	// Wait for GPU completion before calling it. Do not destroy backbuffers separately.
	DestroySwapchain(Swapchain)
	// ResizeSwapchain recreates the backbuffers at a new size, keeping the rest of the
	// swapchain's descriptor.
	ResizeSwapchain(sc Swapchain, width, height uint32)
	// SetPresentMode changes when the swapchain's frames reach the display, keeping the
	// rest of its descriptor. It waits for the GPU to go idle first.
	SetPresentMode(sc Swapchain, mode PresentMode)
	// AcquireNext returns the next backbuffer as a render-target Texture plus a
	// fence that signals when it's safe to reuse.
	// The swapchain owns the returned texture. Present, resize, or destruction may invalidate it.
	AcquireNext(sc Swapchain) (Texture, Fence)
	// Present ends+submits the recorded command list (synced to the acquire) and
	// presents the backbuffer. Use this instead of Submit for on-screen frames.
	Present(sc Swapchain, cmd CommandBuffer)
	SwapchainFormat(sc Swapchain) Format

	// Command recording (transient per frame).
	Begin() CommandBuffer
	Submit(cl CommandBuffer) Fence

	// Synchronization.
	Wait(Fence)
	WaitIdle()

	// GPU timing. CreateTimestampPool makes a pool of count timestamp slots;
	// ReadTimestamps reads them back (call after the writes have completed, e.g.
	// after Present/WaitIdle). Convert a tick delta to nanoseconds with
	// TimestampPeriod. Record writes with CommandBuffer.WriteTimestamp.
	CreateTimestampPool(count uint32) QueryPool
	DestroyTimestampPool(QueryPool)
	ReadTimestamps(pool QueryPool, count uint32) []uint64
	TimestampPeriod() float64 // nanoseconds per timestamp tick

	// GetMaxDataSize is the largest `data` a draw or dispatch accepts, in bytes.
	//
	// It varies widely by driver rather than by hardware — the same Apple GPU reports
	// 4096 through MoltenVK and 256 through KosmicKrisp — so it must be queried, not
	// assumed. Anything that does not fit goes in a buffer with its address passed in
	// data instead.
	GetMaxDataSize() int32

	Destroy()
}

// CommandBuffer records GPU work. It is transient: acquire with Backend.Begin,
// submit once with Backend.Submit, discard. Recording order is execution order
// on the queue; use Barrier to order dependent stages.
type CommandBuffer interface {
	// Dynamic rendering (no render pass objects).
	BeginRenderPass(RenderTargets)
	EndRenderPass()

	// SetPipeline binds a pipeline; compute vs graphics is inferred from the
	// pipeline's own kind (no separate bind-point call).
	SetPipeline(Pipeline)

	// Dynamic state.
	SetViewport(x, y, width, height, minDepth, maxDepth float32)
	SetScissor(x, y, width, height int32)

	// SetDepthBias offsets fragment depth to fight shadow acne / z-fighting
	// (e.g. shadow-map rendering): depth += bias + slope*maxSlope, clamped to
	// clamp (0 => unclamped). It is dynamic per Draw like viewport/scissor;
	// pass all zero to disable. Only meaningful when the bound pipeline has
	// DepthTest or DepthWrite enabled.
	SetDepthBias(bias, slope, clamp float32)

	// Every draw and dispatch carries its own `data`: the bytes the shader reads as
	// push constants. Use utils.ToBytes to pass a struct.
	//
	// It is a parameter rather than sticky state on purpose. Bound state that
	// persists between draws is a standing invitation to forget it and silently
	// inherit the previous draw's parameters — the failure renders, it just renders
	// wrong. Passing it per call makes that impossible to express.
	//
	// data must not exceed Backend.GetMaxDataSize(). Anything larger belongs in a
	// buffer, with its device address passed in data instead; that is also the right
	// shape for anything a shader only reads conditionally, since an address costs 8
	// bytes whatever it points at.
	//
	// The bytes are copied while the command records, so data need not outlive the
	// call. nil is valid for shaders that read nothing.
	//
	// No vertex/index buffer bindings — shaders load geometry from buffers by
	// address. The index buffer, where used, is still bound because indexing is
	// fixed-function.
	Draw(data []byte, vertexCount, instanceCount, firstVertex, firstInstance uint32)
	DrawIndexed(data []byte, indexBuf Buffer, indexType IndexType, indexCount, instanceCount, firstIndex uint32, vertexOffset int32, firstInstance uint32)
	DrawIndexedIndirect(data []byte, indexBuf Buffer, indexType IndexType, args Buffer, argsOffset uint64, drawCount, stride uint32)

	// Dispatch runs the compute pipeline set last in x by y by z workgroups. Each
	// workgroup has the pipeline's WorkgroupSize invocations, so a dimension runs its
	// count times its size: to cover n items with a size of 64, dispatch (n+63)/64
	// workgroups. The last workgroup can be partly outside the n items, so the shader
	// must check gl_GlobalInvocationID against n.
	Dispatch(data []byte, x, y, z uint32)
	// DispatchIndirect is Dispatch with the workgroup counts read from args at offset,
	// when the GPU runs the command: three uint32s, x, y and z, 12 bytes in all. offset
	// must be a multiple of 4, and the 12 bytes must be inside args. The counts are
	// workgroups, not invocations, so a shader that writes them must divide by the
	// WorkgroupSize of the pipeline that reads them.
	DispatchIndirect(data []byte, args Buffer, offset uint64)

	// Barrier orders producer→consumer stages on the queue (no resource lists). It
	// is the only ordering there is: textures as much as buffers, and nothing is
	// ordered implicitly — not even two render passes onto the same target. Whatever
	// reads or writes what an earlier command wrote needs one in between, such as
	// Barrier(StageColorOutput, StageFragment) before sampling an image just rendered.
	Barrier(src, dst Stage, flags BarrierFlags)

	// GPU timestamps. ResetTimestamps clears a pool's slots (record before writing,
	// outside a rendering scope); WriteTimestamp records the GPU clock into slot
	// index once execution reaches `at`.
	ResetTimestamps(pool QueryPool, count uint32)
	WriteTimestamp(pool QueryPool, index uint32, at Stage)

	// Copies.
	CopyBuffer(dst, src Buffer, dstOffset, srcOffset, size uint64)
	CopyBufferToTexture(dst Texture, mip, layer uint32, src Buffer, srcOffset uint64)
	CopyTextureToBuffer(dst Buffer, src Texture, mip, layer uint32) // readback/screenshot
	// CopyTextureToTexture copies one mip and layer of src into the same mip and layer of
	// dst, which must be of the same size and format, and neither multisampled. Both need
	// TextureTransfer usage; a swapchain image may be dst.
	CopyTextureToTexture(dst, src Texture, mip, layer uint32)
}
