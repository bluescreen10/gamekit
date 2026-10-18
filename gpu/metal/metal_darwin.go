//go:build darwin && cgo

// Package metal implements gpu.Backend directly on Metal 4 (macOS 26+), using
// GPU addresses and Tier 2 argument buffers. See README.md for the shader ABI.
//
// Residency is explicit (no useResources:) and there is no setVertexBytes: equivalent:
// per-draw data goes through an MTL4ArgumentTable populated with raw GPU addresses
// instead.
//
// Every submission is fully synchronized (Wait follows Submit, and Present waits), so
// there is never more than one command buffer in flight. That keeps several things
// simple: the root-struct scratch buffer and the argument table are reused across
// frames without double-buffering, because by the time a frame's recording starts the
// previous frame's GPU work has completed.
package metal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/internal/specialization"
)

// Resource kinds.
const (
	kindBuffer = iota + 1
	kindTexture
	kindSampler
	kindGraphics
	kindCompute
	kindSwapchain
	kindQuery
)

// Bindless table sizes: texture and sampler slot 0 is never handed out.
const (
	textureSlots = 65536
	samplerSlots = 2048
)

// resource is one handle's native object and what the backend needs to know of it.
type resource struct {
	object id
	kind   int
	// slot is the bindless table slot, or 0.
	slot uint32

	// Buffers.
	size    uint64
	addr    uint64
	ptr     unsafe.Pointer
	private bool

	// Textures.
	width, height, depth  uint64
	mips, layers, samples uint64
	textureType           uint
	format                uint
	memoryless            bool

	// Pipelines: the depth-stencil state, fixed-function state, and the bytes the
	// shader's root argument occupies, from reflection. MSL rounds a struct's size up to
	// its alignment (16 if it holds a vec4 or mat4), so a caller packing the same fields
	// more tightly hands the argument table a short root and the shader reads past it.
	// Zero when the shader takes no root argument.
	depthStencil id
	topology     uint
	cull         uint
	winding      uint
	depthWrite   bool
	group        [3]uint
	rootSize     uint64

	// Swapchains: the acquired drawable and its texture's handle.
	drawable        id
	drawableTexture gpu.Handle

	// Timestamp pools.
	written            map[uint32]bool
	overrides          map[uint32]uint64
	cpuStart, gpuStart uint64
	count              uint64
}

// Backend owns one device and serial queue. Like command recording, resource
// creation/destruction must be externally serialized. Wait before freeing resources.
type Backend struct {
	// users is how many Inits have no Destroy yet: the device lives while any do.
	users int

	device       device
	queue        queue
	compiler     compiler
	residencySet residencySet
	event, fence id
	// The command allocator, command buffer and argument table are created once and
	// reused every frame rather than recreated each Begin. That reuse is exactly what
	// MTL4's allocator model exists to make cheap — recreating them per frame was
	// measured to cost roughly what the classic bridge's whole encode phase costs. It
	// is safe because every submission is fully synchronized: by the time the next
	// Begin resets the allocator, the previous frame's GPU work has completed, which is
	// the allocator's documented precondition for reset.
	allocator allocator
	argTable  argumentTable
	buffer    commandBuffer

	// The bindless tables: texture and sampler resource IDs, indexed by slot.
	textureTable, samplerTable id
	textureIDs                 []uint64
	samplerIDs                 []uint64
	freeTextures, freeSamplers []uint32

	// rootArena holds draw and dispatch data (see rootSlot), reused across frames.
	rootArena     id
	rootArenaPtr  unsafe.Pointer
	rootArenaAddr uint64
	rootArenaLen  uint64

	maxBufferLength uint64
	apple1          bool

	resources map[uint64]*resource
	next      uint64
	open      *command

	// eventValue is signaled by the queue after each submission; fences maps a
	// submission's fence handle to it. inFlight is set from submit to its completion,
	// and released holds the objects released meanwhile, which the GPU may still use.
	eventValue uint64
	fences     map[uint64]uint64
	inFlight   int
	released   []id
	// retired holds root arenas outgrown while recording, which the GPU may still read.
	retired  []id
	feedback chan feedback
	// early holds feedback that arrived for a submission nobody waits on yet.
	early map[uint64]feedback

	// swapchainFormats is each swapchain's backbuffer format, as created.
	swapchainMu      sync.Mutex
	swapchainFormats map[gpu.Handle]gpu.Format

	// transients holds the textures created with gpu.TextureTransient, which a render
	// pass may only clear or not care about, and never store — checked on every device,
	// memoryless or not, so that misuse fails alike everywhere.
	transientMu sync.Mutex
	transients  map[gpu.Handle]bool
}

var _ gpu.Backend = (*Backend)(nil)
var _ gpu.CommandBuffer = (*command)(nil)

// ShaderFormat identifies the native shader format to renderer integrations.
func (*Backend) ShaderFormat() string {
	return "metal"
}

// pixelFormats maps every gpu.Format this backend supports to Metal. ETC2/EAC are
// missing because Metal never exposes them, on any platform.
var pixelFormats = map[gpu.Format]uint{
	gpu.FormatR8Unorm: mtlPixelFormatR8Unorm, gpu.FormatRG8Unorm: mtlPixelFormatRG8Unorm,
	gpu.FormatRGBA8Unorm: mtlPixelFormatRGBA8Unorm, gpu.FormatBGRA8Unorm: mtlPixelFormatBGRA8Unorm,
	gpu.FormatRGBA8Srgb: mtlPixelFormatRGBA8Unorm_sRGB, gpu.FormatBGRA8Srgb: mtlPixelFormatBGRA8Unorm_sRGB,
	gpu.FormatR8Snorm: mtlPixelFormatR8Snorm, gpu.FormatRG8Snorm: mtlPixelFormatRG8Snorm,
	gpu.FormatRGBA8Snorm: mtlPixelFormatRGBA8Snorm,
	gpu.FormatR8Uint:     mtlPixelFormatR8Uint, gpu.FormatRG8Uint: mtlPixelFormatRG8Uint,
	gpu.FormatRGBA8Uint: mtlPixelFormatRGBA8Uint,
	gpu.FormatR8Sint:    mtlPixelFormatR8Sint, gpu.FormatRG8Sint: mtlPixelFormatRG8Sint,
	gpu.FormatRGBA8Sint: mtlPixelFormatRGBA8Sint,
	gpu.FormatR16Unorm:  mtlPixelFormatR16Unorm, gpu.FormatRG16Unorm: mtlPixelFormatRG16Unorm,
	gpu.FormatRGBA16Unorm: mtlPixelFormatRGBA16Unorm,
	gpu.FormatR16Snorm:    mtlPixelFormatR16Snorm, gpu.FormatRG16Snorm: mtlPixelFormatRG16Snorm,
	gpu.FormatRGBA16Snorm: mtlPixelFormatRGBA16Snorm,
	gpu.FormatR16Uint:     mtlPixelFormatR16Uint, gpu.FormatRG16Uint: mtlPixelFormatRG16Uint,
	gpu.FormatRGBA16Uint: mtlPixelFormatRGBA16Uint,
	gpu.FormatR16Sint:    mtlPixelFormatR16Sint, gpu.FormatRG16Sint: mtlPixelFormatRG16Sint,
	gpu.FormatRGBA16Sint: mtlPixelFormatRGBA16Sint,
	gpu.FormatR16F:       mtlPixelFormatR16Float, gpu.FormatRG16F: mtlPixelFormatRG16Float,
	gpu.FormatRGBA16F: mtlPixelFormatRGBA16Float,
	gpu.FormatR32Uint: mtlPixelFormatR32Uint, gpu.FormatRG32Uint: mtlPixelFormatRG32Uint,
	gpu.FormatRGBA32Uint: mtlPixelFormatRGBA32Uint,
	gpu.FormatR32Sint:    mtlPixelFormatR32Sint, gpu.FormatRG32Sint: mtlPixelFormatRG32Sint,
	gpu.FormatRGBA32Sint: mtlPixelFormatRGBA32Sint,
	gpu.FormatR32F:       mtlPixelFormatR32Float, gpu.FormatRG32F: mtlPixelFormatRG32Float,
	gpu.FormatRGBA32F:      mtlPixelFormatRGBA32Float,
	gpu.FormatRGB10A2Unorm: mtlPixelFormatRGB10A2Unorm, gpu.FormatRGB10A2Uint: mtlPixelFormatRGB10A2Uint,
	gpu.FormatRG11B10F: mtlPixelFormatRG11B10Float, gpu.FormatRGB9E5F: mtlPixelFormatRGB9E5Float,
	gpu.FormatDepth16Unorm: mtlPixelFormatDepth16Unorm, gpu.FormatDepth32F: mtlPixelFormatDepth32Float,
	gpu.FormatDepth24Stencil8:  mtlPixelFormatDepth24Unorm_Stencil8,
	gpu.FormatDepth32FStencil8: mtlPixelFormatDepth32Float_Stencil8, gpu.FormatStencil8: mtlPixelFormatStencil8,
	gpu.FormatBC1Unorm: mtlPixelFormatBC1_RGBA, gpu.FormatBC1Srgb: mtlPixelFormatBC1_RGBA_sRGB,
	gpu.FormatBC3Unorm: mtlPixelFormatBC3_RGBA, gpu.FormatBC3Srgb: mtlPixelFormatBC3_RGBA_sRGB,
	gpu.FormatBC4Unorm: mtlPixelFormatBC4_RUnorm, gpu.FormatBC4Snorm: mtlPixelFormatBC4_RSnorm,
	gpu.FormatBC5Unorm: mtlPixelFormatBC5_RGUnorm, gpu.FormatBC5Snorm: mtlPixelFormatBC5_RGSnorm,
	gpu.FormatBC6HFloat: mtlPixelFormatBC6H_RGBFloat, gpu.FormatBC6HUFloat: mtlPixelFormatBC6H_RGBUfloat,
	gpu.FormatBC7Unorm: mtlPixelFormatBC7_RGBAUnorm, gpu.FormatBC7Srgb: mtlPixelFormatBC7_RGBAUnorm_sRGB,
	gpu.FormatASTC4x4Unorm: mtlPixelFormatASTC_4x4_LDR, gpu.FormatASTC4x4Srgb: mtlPixelFormatASTC_4x4_sRGB,
	gpu.FormatASTC5x5Unorm: mtlPixelFormatASTC_5x5_LDR, gpu.FormatASTC5x5Srgb: mtlPixelFormatASTC_5x5_sRGB,
	gpu.FormatASTC6x6Unorm: mtlPixelFormatASTC_6x6_LDR, gpu.FormatASTC6x6Srgb: mtlPixelFormatASTC_6x6_sRGB,
	gpu.FormatASTC8x8Unorm: mtlPixelFormatASTC_8x8_LDR, gpu.FormatASTC8x8Srgb: mtlPixelFormatASTC_8x8_sRGB,
	gpu.FormatASTC10x10Unorm: mtlPixelFormatASTC_10x10_LDR, gpu.FormatASTC10x10Srgb: mtlPixelFormatASTC_10x10_sRGB,
	gpu.FormatASTC12x12Unorm: mtlPixelFormatASTC_12x12_LDR, gpu.FormatASTC12x12Srgb: mtlPixelFormatASTC_12x12_sRGB,
}

// supportedFormats is the static set this backend accepts, in catalog order.
var supportedFormats = func() []gpu.Format {
	formats := make([]gpu.Format, 0, len(pixelFormats))
	for f := range pixelFormats {
		formats = append(formats, f)
	}
	slices.Sort(formats)
	return formats
}()

func pixelFormat(f gpu.Format) uint {
	if p, ok := pixelFormats[f]; ok {
		return p
	}
	return mtlPixelFormatInvalid
}

// SupportedFormats reports the Format values this backend can create textures with.
func (b *Backend) SupportedFormats() []gpu.Format {
	return supportedFormats
}

func New() *Backend {
	return &Backend{}
}

func init() {
	gpu.RegisterBackend(New(), "metal", 1)
}

// check panics with message unless condition holds: the backend's errors are panics,
// as gpu.Backend has no error returns past Init.
func check(condition bool, message string) {
	if !condition {
		panic("metal: " + message)
	}
}

func flag(v bool) uint {
	if v {
		return 1
	}
	return 0
}

func (b *Backend) Init() error {
	if b.device != 0 {
		b.users++
		return nil
	}
	if err := b.init(); err != nil {
		b.destroy()
		return fmt.Errorf("metal: %s", err)
	}
	b.users = 1
	return nil
}

func (b *Backend) init() error {
	pool := pushPool()
	defer pool.drain()
	b.device = mtlCreateSystemDefaultDevice()
	if b.device == 0 {
		return fmt.Errorf("no Metal device")
	}
	if b.device.argumentBuffersSupport() != mtlArgumentBuffersTier2 || !b.device.hasUnifiedMemory() {
		return fmt.Errorf("requires a unified-memory GPU with Tier 2 argument buffers")
	}
	// The paravirtualized GPU of a macOS virtual machine (GitHub's hosted runners)
	// passes the checks above but has no Metal 4: newMTL4CommandQueue returns nil.
	if !b.device.supportsFamily(mtlGPUFamilyMetal4) {
		return fmt.Errorf("requires a Metal 4 GPU")
	}
	b.apple1 = b.device.supportsFamily(mtlGPUFamilyApple1)
	b.maxBufferLength = uint64(b.device.maxBufferLength())
	if b.queue = b.device.newMTL4CommandQueue(); b.queue == 0 {
		return fmt.Errorf("cannot create command queue")
	}
	var err error
	if b.compiler, err = b.device.newCompiler(); err != nil {
		return err
	}
	if b.residencySet, err = b.device.newResidencySet(); err != nil {
		return err
	}
	b.queue.addResidencySet(b.residencySet)
	b.resources = map[uint64]*resource{}
	b.fences = map[uint64]uint64{}
	b.feedback = make(chan feedback, 16)
	if b.event = b.device.newSharedEvent(); b.event == 0 {
		return fmt.Errorf("cannot create shared event")
	}
	if b.fence = b.device.newFence(); b.fence == 0 {
		return fmt.Errorf("cannot create fence")
	}
	if b.allocator, err = b.device.newAllocator(); err != nil {
		return err
	}
	if b.buffer = b.device.newCommandBuffer(); b.buffer == 0 {
		return fmt.Errorf("cannot create command buffer")
	}
	if b.argTable, err = b.device.newArgumentTable(4); err != nil {
		return err
	}
	b.textureTable = b.device.newBufferWithLength(textureSlots*8, mtlResourceStorageModeShared)
	samplers := min(samplerSlots, uint64(b.device.maxArgumentBufferSamplerCount()))
	b.samplerTable = b.device.newBufferWithLength(samplerSlots*8, mtlResourceStorageModeShared)
	if b.textureTable == 0 || b.samplerTable == 0 {
		return fmt.Errorf("cannot allocate argument tables")
	}
	b.residencySet.enrol(b.textureTable)
	b.residencySet.enrol(b.samplerTable)
	b.textureIDs = unsafe.Slice((*uint64)(bufferContents(b.textureTable)), textureSlots)
	b.samplerIDs = unsafe.Slice((*uint64)(bufferContents(b.samplerTable)), samplerSlots)
	textures := bufferGPUAddress(b.textureTable)
	b.argTable.setAddress(textures, 1)
	b.argTable.setAddress(bufferGPUAddress(b.samplerTable), 2)
	// Storage images (Vulkan's binding 1) read the same table as sampled ones: a Metal
	// texture handle does not care how it is accessed, so a texture's one index serves
	// both, as it does on Vulkan.
	b.argTable.setAddress(textures, 3)
	// Slots are handed out lowest first.
	for i := uint32(textureSlots - 1); i >= 1; i-- {
		b.freeTextures = append(b.freeTextures, i)
	}
	for i := uint32(samplers - 1); i >= 1; i-- {
		b.freeSamplers = append(b.freeSamplers, i)
	}
	return nil
}

// newWithDescriptor creates an object with a new, configured descriptor.
// The creation function reports an NSError on failure.
func newWithDescriptor[T, R ~uintptr](create func(descriptor T, err *id) R, descriptor func() T, configure func(T), failure string) (R, error) {
	d := descriptor()
	defer release(d)
	if configure != nil {
		configure(d)
	}
	var nserr id
	x := create(d, &nserr)
	if x == 0 {
		return 0, fmt.Errorf("%s", errorText(nserr, failure))
	}
	return x, nil
}

// Destroy removes a user (see gpu.Backend.Init), and with the last releases the device.
func (b *Backend) Destroy() {
	if b.users > 1 {
		b.users--
		return
	}
	b.users = 0
	if b.device != 0 {
		b.WaitIdle()
		b.destroy()
	}
}

// destroy releases everything the device owns.
func (b *Backend) destroy() {
	pool := pushPool()
	defer pool.drain()
	for _, r := range b.resources {
		release(r.object)
		release(r.depthStencil)
		release(r.drawable)
	}
	b.resources = nil
	b.textureTable.release()
	b.samplerTable.release()
	b.rootArena.release()
	b.residencySet.release()
	b.compiler.release()
	b.event.release()
	b.fence.release()
	b.allocator.release()
	b.buffer.release()
	b.argTable.release()
	b.queue.release()
	b.device.release()
	*b = Backend{}
}

// add registers a native object under a new handle.
func (b *Backend) add(r *resource) uint64 {
	check(r.object != 0, "Metal resource creation failed")
	b.next++
	b.resources[b.next] = r
	return b.next
}

func (b *Backend) resource(h uint64, kind int) *resource {
	r := b.resources[h]
	check(r != nil && (kind == 0 || r.kind == kind), "invalid resource handle or kind")
	return r
}

// releaseLater releases o now, or once the submission in flight completes, as the GPU
// may still be using it.
func (b *Backend) releaseLater(o id) {
	if o == 0 {
		return
	}
	if b.inFlight > 0 {
		b.released = append(b.released, o)
		return
	}
	release(o)
}

func (b *Backend) Alloc(size uint64, mem gpu.MemoryType, label string) gpu.Buffer {
	check(size > 0 && size <= b.maxBufferLength && mem <= gpu.MemoryDevice, "invalid buffer allocation")
	pool := pushPool()
	defer pool.drain()
	options := uint(mtlResourceStorageModeShared)
	if mem == gpu.MemoryDevice {
		options = mtlResourceStorageModePrivate
	}
	r := &resource{kind: kindBuffer, size: size, private: mem == gpu.MemoryDevice}
	r.object = b.device.newBufferWithLength(uint(size), options)
	h := b.add(r)
	setLabel(r.object, label)
	b.residencySet.enrol(r.object)
	r.addr = uint64(bufferGPUAddress(r.object))
	if !r.private {
		r.ptr = bufferContents(r.object)
	}
	return gpu.Buffer{H: gpu.Handle(h), Size: size, Addr: r.addr, Ptr: r.ptr}
}

func (b *Backend) Free(v gpu.Buffer) {
	b.release(v.H)
}

// release frees a handle: its bindless slot, its residency, and its object.
func (b *Backend) release(h gpu.Handle) {
	r := b.resources[uint64(h)]
	if r == nil {
		return
	}
	if r.slot != 0 {
		if r.kind == kindSampler {
			b.samplerIDs[r.slot] = 0
			b.freeSamplers = append(b.freeSamplers, r.slot)
		} else {
			b.textureIDs[r.slot] = 0
			b.freeTextures = append(b.freeTextures, r.slot)
		}
	}
	if r.kind == kindBuffer || r.kind == kindTexture && !r.memoryless {
		b.residencySet.unenrol(r.object)
	}
	b.releaseLater(r.object)
	b.releaseLater(r.depthStencil)
	delete(b.resources, uint64(h))
}

// slot hands out a bindless slot for a texture or sampler and records its resource ID.
func (b *Backend) slot(r *resource) uint32 {
	free, ids := &b.freeTextures, b.textureIDs
	if r.kind == kindSampler {
		free, ids = &b.freeSamplers, b.samplerIDs
	}
	check(len(*free) > 0, "bindless heap exhausted")
	r.slot = (*free)[len(*free)-1]
	*free = (*free)[:len(*free)-1]
	if r.kind == kindSampler {
		ids[r.slot] = samplerStateGPUResourceID(r.object)
	} else {
		ids[r.slot] = textureGPUResourceID(r.object)
	}
	return r.slot
}

var textureTypes = [...]uint{
	gpu.Texture2D: mtlTextureType2D, gpu.Texture2DArray: mtlTextureType2DArray,
	gpu.TextureCube: mtlTextureTypeCube, gpu.TextureCubeArray: mtlTextureTypeCubeArray,
	gpu.Texture3D: mtlTextureType3D,
}

func textureType(k gpu.TextureKind) uint {
	check(int(k) < len(textureTypes), "unknown texture kind")
	return textureTypes[k]
}

func (b *Backend) CreateTexture(d gpu.TextureDescriptor) gpu.Texture {
	check(d.Width > 0 && d.Height > 0, "texture dimensions must be positive")
	r := &resource{
		kind: kindTexture, textureType: textureType(d.Kind), format: pixelFormat(d.Format),
		width: uint64(d.Width), height: uint64(d.Height), depth: uint64(max(d.Depth, 1)),
		layers: uint64(max(d.Layers, 1)), mips: uint64(max(d.Mips, 1)), samples: uint64(max(d.Samples, 1)),
	}
	arrayLength := r.layers
	switch d.Kind {
	case gpu.TextureCube, gpu.TextureCubeArray:
		check(d.Layers >= 6 && d.Layers%6 == 0, "cube layers must be a multiple of six")
		arrayLength = r.layers / 6
	case gpu.Texture2D, gpu.Texture3D:
		arrayLength, r.layers = 1, 1
	}
	if r.samples > 1 {
		check(d.Kind == gpu.Texture2D && r.mips == 1, "MSAA requires a 2D texture with one mip")
		r.textureType = mtlTextureType2DMultisample
	}
	usage := uint(mtlTextureUsagePixelFormatView)
	if d.Usage&gpu.TextureSampled != 0 {
		usage |= mtlTextureUsageShaderRead
	}
	if d.Usage&gpu.TextureStorage != 0 {
		usage |= mtlTextureUsageShaderRead | mtlTextureUsageShaderWrite
	}
	if d.Usage&(gpu.TextureRenderTarget|gpu.TextureDepth) != 0 {
		usage |= mtlTextureUsageRenderTarget
	}
	storage := uint(mtlStorageModePrivate)
	// gpu.TextureTransient: in tile memory alone, on the GPUs that have it.
	if d.Usage&gpu.TextureTransient != 0 {
		check(d.Usage&(gpu.TextureSampled|gpu.TextureStorage|gpu.TextureTransfer) == 0,
			"a transient texture cannot be sampled, written by shaders or copied")
		usage = mtlTextureUsageRenderTarget
		if b.apple1 {
			storage = mtlStorageModeMemoryless
			r.memoryless = true
		}
	}

	pool := pushPool()
	defer pool.drain()
	desc := newTextureDescriptor()
	defer desc.release()
	desc.setTextureType(uint(r.textureType))
	desc.setWidth(uint(r.width))
	desc.setHeight(uint(r.height))
	desc.setDepth(uint(r.depth))
	desc.setArrayLength(uint(arrayLength))
	desc.setMipmapLevelCount(uint(r.mips))
	desc.setPixelFormat(uint(r.format))
	desc.setSampleCount(uint(r.samples))
	desc.setStorageMode(storage)
	desc.setUsage(usage)
	r.object = b.device.newTextureWithDescriptor(desc)
	h := b.add(r)
	setLabel(r.object, d.Label)
	// Residency sets do not take memoryless textures, which have no memory to make
	// resident.
	if !r.memoryless {
		b.residencySet.enrol(r.object)
	}
	var index uint32
	if d.Usage&(gpu.TextureSampled|gpu.TextureStorage) != 0 {
		index = b.slot(r)
	}
	if d.Usage&gpu.TextureTransient != 0 {
		b.transientMu.Lock()
		if b.transients == nil {
			b.transients = make(map[gpu.Handle]bool)
		}
		b.transients[gpu.Handle(h)] = true
		b.transientMu.Unlock()
	}
	return gpu.Texture{H: gpu.Handle(h), Index: index}
}

// isTransient reports whether t was created with gpu.TextureTransient.
func (b *Backend) isTransient(t gpu.Texture) bool {
	b.transientMu.Lock()
	defer b.transientMu.Unlock()
	return b.transients[t.H]
}

func (b *Backend) TextureView(t gpu.Texture, k gpu.TextureKind, firstMip, mipCount, firstLayer, layerCount uint32) gpu.Texture {
	src := b.resource(uint64(t.H), kindTexture)
	check(mipCount > 0 && layerCount > 0 && uint64(firstMip+mipCount) <= src.mips, "invalid texture view range")
	pool := pushPool()
	defer pool.drain()
	v := *src
	v.textureType = textureType(k)
	v.mips, v.layers = uint64(mipCount), uint64(layerCount)
	// A view's base level is the source's firstMip.
	v.width, v.height, v.depth = max(src.width>>firstMip, 1), max(src.height>>firstMip, 1), max(src.depth>>firstMip, 1)
	v.slot = 0
	v.object = textureNewTextureViewWithPixelFormat(src.object, src.format, v.textureType, uint(firstMip), uint(mipCount), uint(firstLayer), uint(layerCount))
	h := b.add(&v)
	if !v.memoryless {
		b.residencySet.enrol(v.object)
	}
	return gpu.Texture{H: gpu.Handle(h), Index: b.slot(&v)}
}

func (b *Backend) DestroyTexture(t gpu.Texture) {
	b.transientMu.Lock()
	delete(b.transients, t.H)
	b.transientMu.Unlock()
	b.release(t.H)
}

var addressModes = [...]uint{
	gpu.AddressClamp: mtlSamplerAddressModeClampToEdge, gpu.AddressRepeat: mtlSamplerAddressModeRepeat,
	gpu.AddressMirror: mtlSamplerAddressModeMirrorRepeat,
}

func (b *Backend) CreateSampler(d gpu.SamplerDescriptor) gpu.Sampler {
	check(int(d.AddressU) < len(addressModes) && int(d.AddressV) < len(addressModes) &&
		int(d.AddressW) < len(addressModes) && d.Compare < 8, "invalid sampler descriptor")
	filter := func(linear bool) uint {
		if linear {
			return mtlSamplerMinMagFilterLinear
		}
		return mtlSamplerMinMagFilterNearest
	}
	pool := pushPool()
	defer pool.drain()
	desc := newSamplerDescriptor()
	defer desc.release()
	desc.setMinFilter(filter(d.MinLinear))
	desc.setMagFilter(filter(d.MagLinear))
	mip := uint(mtlSamplerMipFilterNearest)
	if d.MipLinear {
		mip = mtlSamplerMipFilterLinear
	}
	desc.setMipFilter(mip)
	desc.setSAddressMode(addressModes[d.AddressU])
	desc.setTAddressMode(addressModes[d.AddressV])
	desc.setRAddressMode(addressModes[d.AddressW])
	desc.setCompareFunction(uint(d.Compare))
	desc.setMaxAnisotropy(uint(max(d.MaxAnisotropy, 1)))
	desc.setSupportArgumentBuffers(true)
	setLabel(id(desc), d.Label)
	r := &resource{kind: kindSampler, object: b.device.newSamplerStateWithDescriptor(desc)}
	h := b.add(r)
	return gpu.Sampler{H: gpu.Handle(h), Index: b.slot(r)}
}

func (b *Backend) DestroySampler(s gpu.Sampler) {
	b.release(s.H)
}

func entry(s string) string {
	if s == "" || s == "main" {
		return "main0"
	}
	return s
}

// shaderLibrary loads a metallib, or compiles MSL source. The library is autoreleased.
func (b *Backend) shaderLibrary(code []byte) id {
	check(len(code) > 0, "empty shader")
	var library, nserr id
	if bytes.HasPrefix(code, []byte("MTLB")) {
		data := dispatchDataCreate(unsafe.Pointer(&code[0]), len(code))
		library = b.device.newLibraryWithData(data, &nserr)
		release(data)
	} else {
		check(len(code) < 4 || binary.LittleEndian.Uint32(code) != 0x07230203,
			"SPIR-V is not supported; supply MSL source or a metallib")
		source := nsString(string(code))
		check(source != 0, "shader is not UTF-8 MSL or a metallib")
		options := newObject("MTLCompileOptions")
		compileOptionsSetLanguageVersion(options, mtlLanguageVersion3_0)
		library = b.device.newLibraryWithSource(source, options, &nserr)
		release(options)
		release(source)
	}
	check(library != 0, errorText(nserr, "shader compilation failed"))
	return objectAutorelease(library)
}

// libraryFunction names an entry of a shader for MTL4's descriptor-based pipeline
// creation. It is autoreleased.
func (b *Backend) libraryFunction(code []byte, entryName string) libraryFunctionDescriptor {
	fd := newLibraryFunctionDescriptor()
	fd.setLibrary(b.shaderLibrary(code))
	name := nsString(entry(entryName))
	fd.setName(name)
	release(name)
	return libraryFunctionDescriptor(objectAutorelease(id(fd)))
}

// constant is one resolved function constant.
type constant struct {
	index uint32
	kind  specialization.Type
	bits  uint32
}

var constantTypes = [...]uint{
	specialization.Bool: mtlDataTypeBool, specialization.Int: mtlDataTypeInt,
	specialization.Uint: mtlDataTypeUInt, specialization.Float: mtlDataTypeFloat,
}

// specializedFunction fixes a function's constants (see gpu.Constant). A function
// constant left unset keeps the default its shader declares. It specializes even with
// no constants: Metal refuses to build a pipeline from a function that declares
// function constants and was never specialized, however every one of them has a
// default. It is autoreleased.
func specializedFunction(function libraryFunctionDescriptor, constants []constant) specializedFunctionDescriptor {
	values := newObject("MTLFunctionConstantValues")
	for _, c := range constants {
		check(int(c.kind) < len(constantTypes), "invalid constant type")
		// A Metal bool is one byte; the other types take all 32 bits.
		value := [4]byte{}
		if c.kind == specialization.Bool {
			value[0] = byte(flag(c.bits != 0))
		} else {
			binary.NativeEndian.PutUint32(value[:], c.bits)
		}
		functionConstantValuesSetConstantValue(values, unsafe.Pointer(&value[0]), constantTypes[c.kind], uint(c.index))
	}
	fd := newSpecializedFunctionDescriptor()
	fd.setFunctionDescriptor(function)
	fd.setConstantValues(values)
	release(values)
	return specializedFunctionDescriptor(objectAutorelease(id(fd)))
}

// pipelineOptions asks for binding reflection, which rootBytes reads. It is
// autoreleased.
func pipelineOptions() id {
	o := newObject("MTL4PipelineOptions")
	pipelineOptionsSetShaderReflection(o, mtl4ShaderReflectionBindingInfo)
	return objectAutorelease(o)
}

// rootBytes reports how many bytes the shader expects at buffer(0) — the RHI's root
// argument — or 0 if it binds nothing there.
func rootBytes(bindings id) uint64 {
	for i := range arrayCount(bindings) {
		x := arrayObjectAtIndex(bindings, i)
		if bindingIndex(x) == 0 && bindingType(x) == mtlBindingTypeBuffer && bindingIsUsed(x) {
			return uint64(bufferBindingBufferDataSize(x))
		}
	}
	return 0
}

func (b *Backend) CreateComputePipeline(d gpu.ComputePipelineDescriptor) gpu.Pipeline {
	code, group := d.Shader, d.WorkgroupSize
	if group[0] == 0 || group[1] == 0 || group[2] == 0 {
		panic(fmt.Sprintf("metal: CreateComputePipeline(%q): every WorkgroupSize dimension must be at least 1, not %v", d.Label, group))
	}
	constants, err := b.resolveConstants(d.Constants, stageShader{code: code, entry: d.Entry})
	if err != nil {
		panic(fmt.Sprintf("metal: CreateComputePipeline(%q): %v", d.Label, err))
	}
	pool := pushPool()
	defer pool.drain()
	desc := newComputePipelineDescriptor()
	defer desc.release()
	setLabel(id(desc), d.Label)
	desc.setComputeFunctionDescriptor(specializedFunction(b.libraryFunction(code, d.Entry), constants))
	desc.setOptions(pipelineOptions())
	var nserr id
	ps := b.compiler.newComputePipelineStateWithDescriptor(desc, 0, &nserr)
	check(ps != 0, errorText(nserr, "compute pipeline creation failed"))
	r := &resource{kind: kindCompute, object: ps}
	r.group = [3]uint{uint(group[0]), uint(group[1]), uint(group[2])}
	if uint64(group[0])*uint64(group[1])*uint64(group[2]) > uint64(computePipelineStateMaxTotalThreadsPerThreadgroup(ps)) {
		release(ps)
		panic("metal: workgroup too large")
	}
	r.rootSize = rootBytes(computePipelineReflectionBindings(computePipelineStateReflection(ps)))
	return gpu.Pipeline{H: gpu.Handle(b.add(r))}
}

var blendFactors = [...]uint{
	gpu.BlendZero: mtlBlendFactorZero, gpu.BlendOne: mtlBlendFactorOne,
	gpu.BlendSrcAlpha: mtlBlendFactorSourceAlpha, gpu.BlendOneMinusSrcAlpha: mtlBlendFactorOneMinusSourceAlpha,
	gpu.BlendDstAlpha: mtlBlendFactorDestinationAlpha, gpu.BlendOneMinusDstAlpha: mtlBlendFactorOneMinusDestinationAlpha,
}

var topologies = [...]uint{
	gpu.TopologyTriangles: mtlPrimitiveTypeTriangle, gpu.TopologyTriangleStrip: mtlPrimitiveTypeTriangleStrip,
	gpu.TopologyLines: mtlPrimitiveTypeLine, gpu.TopologyPoints: mtlPrimitiveTypePoint,
}

var cullModes = [...]uint{
	gpu.CullNone: mtlCullModeNone, gpu.CullBack: mtlCullModeBack, gpu.CullFront: mtlCullModeFront,
}

func (b *Backend) CreateGraphicsPipeline(d gpu.PipelineDescriptor) gpu.Pipeline {
	vertex, fragment := d.VertexShader, d.FragmentShader
	check(len(d.ColorFormats) <= 8, "at most 8 color targets")
	check(int(d.Topology) < len(topologies) && int(d.CullMode) < len(cullModes), "invalid topology or cull mode")
	constants, err := b.resolveConstants(d.Constants,
		stageShader{code: vertex, entry: d.VertexEntry},
		stageShader{code: fragment, entry: d.FragmentEntry})
	if err != nil {
		panic(fmt.Sprintf("metal: CreateGraphicsPipeline(%q): %v", d.Label, err))
	}

	pool := pushPool()
	defer pool.drain()
	desc := newRenderPipelineDescriptor()
	defer desc.release()
	setLabel(id(desc), d.Label)
	// Both stages share one set of constants, as on Vulkan; a stage ignores any it does
	// not declare.
	desc.setVertexFunctionDescriptor(specializedFunction(b.libraryFunction(vertex, d.VertexEntry), constants))
	if len(fragment) > 0 {
		desc.setFragmentFunctionDescriptor(specializedFunction(b.libraryFunction(fragment, d.FragmentEntry), constants))
	}
	desc.setRasterSampleCount(uint(max(d.Samples, 1)))
	desc.setOptions(pipelineOptions())
	attachments := desc.colorAttachments()
	for i, f := range d.ColorFormats {
		t := renderPipelineColorAttachmentDescriptor(arrayObjectAtIndexedSubscript(attachments, uint(i)))
		t.setPixelFormat(uint(pixelFormat(f)))
		var blend gpu.BlendState
		if i < len(d.Blend) {
			blend = d.Blend[i]
		}
		// The RHI's mask has R at bit 0, Metal's at bit 3.
		mask := uint(blend.WriteMask & 15)
		if mask == 0 {
			mask = 15
		}
		t.setWriteMask((mask&1)<<3 | (mask&2)<<1 | (mask&4)>>1 | (mask&8)>>3)
		if !blend.Enable {
			t.setBlendingState(mtl4BlendStateDisabled)
			continue
		}
		for _, f := range []gpu.BlendFactor{blend.ColorOp.Src, blend.ColorOp.Dst, blend.AlphaOp.Src, blend.AlphaOp.Dst} {
			check(int(f) < len(blendFactors), "invalid blend factor")
		}
		t.setBlendingState(mtl4BlendStateEnabled)
		t.setSourceRGBBlendFactor(blendFactors[blend.ColorOp.Src])
		t.setDestinationRGBBlendFactor(blendFactors[blend.ColorOp.Dst])
		t.setRGBBlendOperation(uint(blend.ColorOp.Op))
		t.setSourceAlphaBlendFactor(blendFactors[blend.AlphaOp.Src])
		t.setDestinationAlphaBlendFactor(blendFactors[blend.AlphaOp.Dst])
		t.setAlphaBlendOperation(uint(blend.AlphaOp.Op))
	}
	// Metal 4 pipeline state is no longer tied to a depth/stencil attachment format; it
	// lives purely on the render pass descriptor. The descriptor's depth fields still
	// select the depth-stencil state's compare and write behavior below.
	var nserr id
	ps := b.compiler.newRenderPipelineStateWithDescriptor(desc, 0, &nserr)
	check(ps != 0, errorText(nserr, "render pipeline creation failed"))
	r := &resource{kind: kindGraphics, object: ps, depthWrite: d.DepthWrite}
	// bindings() pushes the root to both stages, so the larger expectation governs.
	reflection := computePipelineStateReflection(ps)
	r.rootSize = max(rootBytes(renderPipelineReflectionVertexBindings(reflection)), rootBytes(renderPipelineReflectionFragmentBindings(reflection)))
	depth := newDepthStencilDescriptor()
	compare := uint(mtlCompareFunctionAlways)
	if d.DepthTest {
		compare = uint(d.DepthCompare)
	}
	depth.setDepthCompareFunction(compare)
	depth.setDepthWriteEnabled(d.DepthWrite)
	r.depthStencil = b.device.newDepthStencilStateWithDescriptor(depth)
	depth.release()
	r.topology = topologies[d.Topology]
	r.cull = cullModes[d.CullMode]
	r.winding = mtlWindingCounterClockwise
	if d.FrontFaceCW {
		r.winding = mtlWindingClockwise
	}
	return gpu.Pipeline{H: gpu.Handle(b.add(r))}
}

// stageShader is one stage's shader, as resolveConstants reads it.
type stageShader struct {
	code  []byte
	entry string
}

// resolveConstants resolves a descriptor's constants against the function constants
// the stages declare. A constant only one stage declares is passed to both, and the
// other ignores it.
func (b *Backend) resolveConstants(constants map[string]float64, stages ...stageShader) ([]constant, error) {
	if len(constants) == 0 {
		return nil, nil
	}
	var declarations [][]specialization.Declaration
	for _, stage := range stages {
		if len(stage.code) > 0 {
			declarations = append(declarations, b.functionConstants(stage))
		}
	}
	values, err := specialization.Resolve(constants, declarations...)
	if err != nil {
		return nil, err
	}
	var resolved []constant
	for _, v := range values {
		resolved = append(resolved, constant{index: v.ID, kind: v.Type, bits: v.Bits})
	}
	return resolved, nil
}

// functionConstants lists the function constants a stage's entry declares, named as
// its GLSL source names them: SPIRV-Cross gives a constant X the function constant
// X_tmp.
func (b *Backend) functionConstants(stage stageShader) []specialization.Declaration {
	pool := pushPool()
	defer pool.drain()
	name := nsString(entry(stage.entry))
	function := libraryNewFunctionWithName(b.shaderLibrary(stage.code), name)
	release(name)
	check(function != 0, "shader has no such entry point")
	defer release(function)
	values := dictionaryAllValues(functionFunctionConstantsDictionary(function))
	var declarations []specialization.Declaration
	for i := range arrayCount(values) {
		c := arrayObjectAtIndex(values, i)
		var kind specialization.Type
		switch functionConstantType(c) {
		case mtlDataTypeBool:
			kind = specialization.Bool
		case mtlDataTypeInt:
			kind = specialization.Int
		case mtlDataTypeUInt:
			kind = specialization.Uint
		case mtlDataTypeFloat:
			kind = specialization.Float
		default:
			panic(fmt.Sprintf("metal: function constant %s is not a bool, int, uint or float", stringOf(functionConstantName(c))))
		}
		declarations = append(declarations, specialization.Declaration{
			ID:   uint32(functionConstantIndex(c)),
			Name: strings.TrimSuffix(stringOf(functionConstantName(c)), "_tmp"),
			Type: kind,
		})
	}
	return declarations
}

func (b *Backend) DestroyPipeline(p gpu.Pipeline) {
	b.release(p.H)
}

// CreateSwapchain creates a presentation chain for a CAMetalLayer surface.
func (b *Backend) CreateSwapchain(surface gpu.Surface, desc gpu.SwapchainDescriptor) (gpu.Swapchain, error) {
	format := desc.Format
	if format == gpu.FormatUndefined {
		format = gpu.FormatBGRA8Unorm
	}
	if !slices.Contains(layerFormats, format) {
		return gpu.Swapchain{}, fmt.Errorf("metal: swapchain format %v is not one a CAMetalLayer presents", format)
	}
	layer := id(surface)
	check(objectIsKindOfClass(layer, class("CAMetalLayer")), "surface must be a CAMetalLayer")
	metalLayerSetDevice(layer, b.device)
	metalLayerSetPixelFormat(layer, uint(pixelFormat(format)))
	metalLayerSetFramebufferOnly(layer, false)
	metalLayerSetDrawableSize(layer, float64(desc.Width), float64(desc.Height))
	sc := gpu.Swapchain{H: gpu.Handle(b.add(&resource{kind: kindSwapchain, object: retain(layer)}))}
	b.setDisplaySync(sc, desc.PresentMode)

	b.swapchainMu.Lock()
	defer b.swapchainMu.Unlock()
	if b.swapchainFormats == nil {
		b.swapchainFormats = make(map[gpu.Handle]gpu.Format)
	}
	b.swapchainFormats[sc.H] = format
	return sc, nil
}

// layerFormats is what a CAMetalLayer can present, in the RHI's catalog.
var layerFormats = []gpu.Format{gpu.FormatBGRA8Unorm, gpu.FormatBGRA8Srgb, gpu.FormatRGBA16F, gpu.FormatRGB10A2Unorm}

// SwapchainFormats is the same for every surface: whatever a CAMetalLayer presents.
func (b *Backend) SwapchainFormats(surface gpu.Surface) []gpu.Format {
	return slices.Clone(layerFormats)
}

func (b *Backend) ResizeSwapchain(s gpu.Swapchain, w, h uint32) {
	b.WaitIdle()
	r := b.resource(uint64(s.H), kindSwapchain)
	check(r.drawable == 0, "cannot resize with an acquired drawable")
	metalLayerSetDrawableSize(r.object, float64(w), float64(h))
}

// SetPresentMode switches the layer's display sync. A CAMetalLayer without it presents
// as soon as it can, which is PresentImmediate; it has no mailbox, so PresentMailbox
// keeps display sync, as PresentVSync does.
func (b *Backend) SetPresentMode(s gpu.Swapchain, mode gpu.PresentMode) {
	b.WaitIdle()
	b.setDisplaySync(s, mode)
}

func (b *Backend) setDisplaySync(s gpu.Swapchain, mode gpu.PresentMode) {
	metalLayerSetDisplaySyncEnabled(b.resource(uint64(s.H), kindSwapchain).object, mode != gpu.PresentImmediate)
}

func (b *Backend) SwapchainSize(s gpu.Swapchain) (uint32, uint32) {
	w, h := metalLayerDrawableSize(b.resource(uint64(s.H), kindSwapchain).object)
	return uint32(w), uint32(h)
}

func (b *Backend) SwapchainFormat(s gpu.Swapchain) gpu.Format {
	b.swapchainMu.Lock()
	defer b.swapchainMu.Unlock()
	return b.swapchainFormats[s.H]
}

func (b *Backend) AcquireNext(s gpu.Swapchain) (gpu.Texture, gpu.Fence) {
	r := b.resource(uint64(s.H), kindSwapchain)
	check(r.drawable == 0, "drawable already acquired")
	pool := pushPool()
	defer pool.drain()
	r.drawable = retain(metalLayerNextDrawable(r.object))
	check(r.drawable != 0, "no drawable available (window hidden or zero size)")
	// A drawable is used only as a render attachment; it is never referenced through a
	// bindless argument table. Keeping it out of the residency set means its
	// acquire/release cycle costs nothing beyond this call.
	t := retain(drawableTexture(r.drawable))
	tex := &resource{
		kind: kindTexture, object: t, textureType: mtlTextureType2D, layers: 1, mips: 1, samples: 1,
		width: uint64(textureWidth(t)), height: uint64(textureHeight(t)), depth: 1,
		format: uint(texturePixelFormat(t)),
	}
	b.next++
	b.resources[b.next] = tex
	r.drawableTexture = gpu.Handle(b.next)
	return gpu.Texture{H: r.drawableTexture}, gpu.Fence{}
}

func (b *Backend) Present(s gpu.Swapchain, c gpu.CommandBuffer) {
	v := b.cmd(c)
	r := b.resource(uint64(s.H), kindSwapchain)
	check(r.drawable != 0, "no acquired drawable")
	value := b.submit(v, r.drawable)
	b.releaseLater(b.resources[uint64(r.drawableTexture)].object)
	delete(b.resources, uint64(r.drawableTexture))
	b.releaseLater(r.drawable)
	r.drawable, r.drawableTexture = 0, 0
	// Applications reuse mapped frame data and read counters immediately after Present.
	// Match the synchronous presentation contract of the Vulkan backend.
	b.completed(value)
}

func (b *Backend) CreateTimestampPool(n uint32) gpu.QueryPool {
	if n == 0 {
		return gpu.QueryPool{}
	}
	pool := pushPool()
	defer pool.drain()
	heap, err := newWithDescriptor(b.device.newCounterHeapWithDescriptor, newCounterHeapDescriptor, func(d counterHeapDescriptor) {
		d.setType(mtl4CounterHeapTypeTimestamp)
		d.setCount(uint(n))
	}, "")
	if err != nil {
		return gpu.QueryPool{} // unsupported device: degrade to no timestamps.
	}
	r := &resource{kind: kindQuery, object: heap, count: uint64(n), written: map[uint32]bool{}, overrides: map[uint32]uint64{}}
	var cpu, gpuStart uint64
	b.device.sampleTimestamps(&cpu, &gpuStart)
	r.cpuStart, r.gpuStart = cpu, gpuStart
	return gpu.QueryPool{H: gpu.Handle(b.add(r))}
}

func (b *Backend) DestroyTimestampPool(p gpu.QueryPool) {
	b.release(p.H)
}

// ReadTimestamps returns host nanoseconds: GPU counter values are calibrated against
// the CPU clock sampled when the pool was created and again now.
func (b *Backend) ReadTimestamps(p gpu.QueryPool, n uint32) []uint64 {
	if !p.IsValid() || n == 0 {
		return nil
	}
	r := b.resource(uint64(p.H), kindQuery)
	check(uint64(n) <= r.count, "query read out of bounds")
	pool := pushPool()
	defer pool.drain()
	data := counterHeapResolveCounterRange(r.object, 0, uint(n))
	check(data != 0 && uint64(dataLength(data)) >= uint64(n)*8, "timestamp resolve failed")
	values := unsafe.Slice((*uint64)(dataBytes(data)), n)
	var cpu, gpuNow uint64
	b.device.sampleTimestamps(&cpu, &gpuNow)
	check(gpuNow > r.gpuStart, "GPU timestamp calibration unavailable")
	scale := hostScale()
	period := float64(cpu-r.cpuStart) * scale / float64(gpuNow-r.gpuStart)
	out := make([]uint64, n)
	for i := range out {
		if v, ok := r.overrides[uint32(i)]; ok {
			out[i] = v
		} else if r.written[uint32(i)] && values[i] != 0 {
			out[i] = uint64(float64(r.cpuStart)*scale + (float64(values[i])-float64(r.gpuStart))*period)
		}
	}
	return out
}

// hostScale is nanoseconds per mach absolute time tick.
func hostScale() float64 {
	numer, denom := machTimebaseInfo()
	return float64(numer) / float64(denom)
}

func (b *Backend) TimestampPeriod() float64 {
	return 1
}

// GetMaxDataSize is the largest draw/dispatch data this device accepts, in bytes.
//
// Metal's setBytes: is documented for data under 4 KB; past that Apple directs callers
// to an MTLBuffer instead. There is no queryable device property for it, so the
// documented threshold is the limit.
func (b *Backend) GetMaxDataSize() int32 {
	return 4096
}
