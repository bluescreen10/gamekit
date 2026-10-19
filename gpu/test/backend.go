// Package test is an in-process gpu.Backend backed by plain Go memory, for testing
// code that sits on top of the gpu package without a real GPU device or driver.
//
// It is deliberately not a software renderer: draws and dispatches are no-ops, and
// nothing ever gets rasterized or computed. What it does provide, faithfully, is the
// memory model — Alloc/Free hand back real backing storage (MemoryHost buffers get a
// live Ptr, exactly like a real device's persistently-mapped memory), and
// CopyBuffer/CopyBufferToTexture/CopyTextureToBuffer move real bytes between them. That
// covers everything a test needs to exercise buffer/texture lifecycle, resource
// bookkeeping and upload paths — anything that also needs actual rendered pixels needs
// a real backend instead.
//
// It registers itself at the lowest priority (0), so importing this package for its
// side effect never silently replaces a real backend picked by gpu.Instance(nil); a
// caller that wants it explicitly asks by name: gpu.Lookup("test") or
// PIX_GPU_BACKEND=test.
package test

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/bluescreen10/gamekit/gpu"
)

func init() {
	gpu.RegisterBackend(New(), "test", 0)
}

// Backend implements gpu.Backend (asserted here so signature drift is a compile
// error, not a late failure at the first cross-package use).
var _ gpu.Backend = (*Backend)(nil)

// buffer is one Alloc'd allocation's backing storage. Kept regardless of MemoryType so
// CopyBuffer/CopyBufferToTexture/CopyTextureToBuffer can move real bytes even into or
// out of a MemoryDevice buffer, which has no Ptr of its own to expose.
type buffer struct {
	reference
	data   []byte
	mapped bool
}

// texture is one CreateTexture's backing storage: every mip level's bytes,
// concatenated, with offset[m] the byte offset mip m starts at (offset[mips] is the
// total size, for computing the last level's length). Array layers and cube faces are
// folded into each mip's byte range rather than addressed individually — this backend
// only needs to move bytes, not know which layer they belong to.
type texture struct {
	reference
	index  uint32
	format gpu.Format
	data   []byte
	offset []uint64
}

// Backend is the in-memory gpu.Backend. Its zero value is usable.
type Backend struct {
	mu     sync.Mutex
	fences []*fence

	nextTexIndex     uint32
	nextSamplerIndex uint32
	nextTick         uint64
}

// New creates an unconnected Backend. Init is a no-op — there is no device to open —
// but callers should still call it, since gpu.Backend requires it.
func New() *Backend {
	return &Backend{}
}

func (b *Backend) Init() error {
	return nil
}

// ----------------------------------------------------------------------------
// Memory
// ----------------------------------------------------------------------------

func (b *Backend) Alloc(size uint64, mem gpu.MemoryType, label string) gpu.Buffer {
	b.mu.Lock()
	defer b.mu.Unlock()

	buf := &buffer{data: make([]byte, size), mapped: mem == gpu.MemoryHost}
	buf.reference = reference{backend: b}
	return buf
}

func (b *Backend) Free(buf gpu.Buffer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	destroyResource[*buffer](b, buf)
}

// ----------------------------------------------------------------------------
// Bindless resources
// ----------------------------------------------------------------------------

func (b *Backend) CreateTexture(desc gpu.TextureDescriptor) gpu.Texture {
	b.mu.Lock()
	defer b.mu.Unlock()

	texelSize, ok := texelSizes[desc.Format]
	if !ok {
		panic(fmt.Sprintf("gpu/test: CreateTexture: unsupported format %v (block-compressed "+
			"formats aren't supported — this backend only stores raw texel bytes)", desc.Format))
	}

	mips := max(desc.Mips, 1)
	depth := max(desc.Depth, 1)
	layers := max(desc.Layers, 1)

	offset := make([]uint64, mips+1)
	width, height := desc.Width, desc.Height
	for m := range mips {
		levelSize := uint64(max(width, 1)) * uint64(max(height, 1)) * uint64(depth) * uint64(layers) * uint64(texelSize)
		offset[m+1] = offset[m] + levelSize
		width, height = width/2, height/2
	}

	tex := &texture{
		format: desc.Format,
		data:   make([]byte, offset[mips]),
		offset: offset,
	}

	var index uint32
	if desc.Usage&gpu.TextureSampled != 0 {
		b.nextTexIndex++
		index = b.nextTexIndex
	}
	tex.index = index
	tex.reference = reference{backend: b}
	return tex
}

func (b *Backend) DestroyTexture(t gpu.Texture) {
	b.mu.Lock()
	defer b.mu.Unlock()
	destroyResource[*texture](b, t)
}

// TextureView hands back another bindless index for the same underlying storage: this
// backend has no real subresource views, only bytes, so a view is indistinguishable
// from the texture it's a view of.
func (b *Backend) TextureView(t gpu.Texture, kind gpu.TextureKind, baseMip, mipCount, baseLayer, layerCount uint32) gpu.Texture {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextTexIndex++
	view := *resourceOf[*texture](b, t)
	view.reference = reference{}
	view.index = b.nextTexIndex
	view.reference = reference{backend: b}
	return &view
}

func (b *Backend) CreateSampler(desc gpu.SamplerDescriptor) gpu.Sampler {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextSamplerIndex++
	s := &sampler{index: b.nextSamplerIndex}
	s.reference = reference{backend: b}
	return s
}

func (b *Backend) DestroySampler(s gpu.Sampler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	destroyResource[*sampler](b, s)
}

// ----------------------------------------------------------------------------
// Pipelines
// ----------------------------------------------------------------------------

func (b *Backend) CreateComputePipeline(d gpu.ComputePipelineDescriptor) gpu.Pipeline {
	if size := d.WorkgroupSize; size[0] == 0 || size[1] == 0 || size[2] == 0 {
		panic(fmt.Sprintf("test: CreateComputePipeline(%q): every WorkgroupSize dimension must be at least 1, not %v", d.Label, size))
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p := &pipeline{}
	p.reference = reference{backend: b}
	return p
}

func (b *Backend) CreateGraphicsPipeline(gpu.PipelineDescriptor) gpu.Pipeline {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := &pipeline{}
	p.reference = reference{backend: b}
	return p
}

func (b *Backend) DestroyPipeline(p gpu.Pipeline) {
	b.mu.Lock()
	defer b.mu.Unlock()
	destroyResource[*pipeline](b, p)
}

// ----------------------------------------------------------------------------
// Presentation
// ----------------------------------------------------------------------------

// swapchainState is a fake swapchain's single backbuffer — there is no compositor to
// present to, so there is nothing to cycle between multiple images for.
type swapchainState struct {
	reference
	width, height uint32
	format        gpu.Format
	presentMode   gpu.PresentMode
	backbuffer    *texture
}

func (b *Backend) createBackbufferLocked(width, height uint32, format gpu.Format) *texture {
	texelSize := texelSizes[format]
	size := uint64(width) * uint64(height) * uint64(texelSize)
	t := &texture{
		format: format,
		data:   make([]byte, size),
		offset: []uint64{0, size},
	}
	t.reference = reference{backend: b}
	return t
}

// surfaceFormats is what this backend's pretend surfaces present.
var surfaceFormats = []gpu.Format{gpu.FormatBGRA8Unorm, gpu.FormatBGRA8Srgb}

func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("gpu/test: nil surface target")
	}
	return 1, nil
}

func (b *Backend) SwapchainFormats(surface gpu.Surface) []gpu.Format {
	return slices.Clone(surfaceFormats)
}

func (b *Backend) CreateSwapchain(surface gpu.Surface, desc gpu.SwapchainDescriptor) (gpu.Swapchain, error) {
	format := desc.Format
	if format == gpu.FormatUndefined {
		format = gpu.FormatBGRA8Unorm
	}
	if !slices.Contains(surfaceFormats, format) {
		return nil, fmt.Errorf("gpu/test: swapchain format %v is not one the surface presents", format)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	s := &swapchainState{
		width:       desc.Width,
		height:      desc.Height,
		format:      format,
		presentMode: desc.PresentMode,
		backbuffer:  b.createBackbufferLocked(desc.Width, desc.Height, format),
	}
	s.reference = reference{backend: b}
	return s, nil
}

func (b *Backend) ResizeSwapchain(sc gpu.Swapchain, width, height uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := resourceOf[*swapchainState](b, sc)
	if st == nil || !st.IsValid() {
		return
	}
	st.backbuffer.reference.destroy()
	st.width, st.height = width, height
	st.backbuffer = b.createBackbufferLocked(width, height, st.format)
}

func (b *Backend) DestroySwapchain(sc gpu.Swapchain) {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := resourceOf[*swapchainState](b, sc)
	if s == nil || !s.IsValid() {
		return
	}

	s.backbuffer.reference.destroy()
	s.reference.destroy()
}

// SetPresentMode records mode; there is no display for it to change anything on.
func (b *Backend) SetPresentMode(sc gpu.Swapchain, mode gpu.PresentMode) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if st := resourceOf[*swapchainState](b, sc); st != nil && st.IsValid() {
		st.presentMode = mode
	}
}

// PresentMode is the mode the swapchain presents in, as created or last set.
func (b *Backend) PresentMode(sc gpu.Swapchain) gpu.PresentMode {
	b.mu.Lock()
	defer b.mu.Unlock()
	return resourceOf[*swapchainState](b, sc).presentMode
}

func (b *Backend) AcquireNext(sc gpu.Swapchain) (gpu.Texture, gpu.Fence) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := resourceOf[*swapchainState](b, sc)
	if st == nil || !st.IsValid() {
		panic("gpu/test: AcquireNext on an unknown swapchain")
	}
	f := &fence{reference: reference{backend: b}, pendingIndex: len(b.fences)}
	b.fences = append(b.fences, f)
	return st.backbuffer, f
}

func (b *Backend) Present(sc gpu.Swapchain, cmd gpu.CommandBuffer) {}

func (b *Backend) SwapchainFormat(sc gpu.Swapchain) gpu.Format {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := resourceOf[*swapchainState](b, sc)
	if st == nil || !st.IsValid() {
		panic("gpu/test: SwapchainFormat on an unknown swapchain")
	}
	return st.format
}

// ----------------------------------------------------------------------------
// Command recording
// ----------------------------------------------------------------------------

// Begin returns a command buffer that applies its copies immediately as they are
// recorded: there is no real queue to defer work onto, so Submit/Wait have nothing
// left to do.
func (b *Backend) Begin() gpu.CommandBuffer {
	return &commandBuffer{backend: b}
}

func (b *Backend) Submit(cl gpu.CommandBuffer) gpu.Fence {
	b.mu.Lock()
	defer b.mu.Unlock()
	f := &fence{reference: reference{backend: b}, pendingIndex: len(b.fences)}
	b.fences = append(b.fences, f)
	return f
}

// Wait retires the token. Commands already ran synchronously when recorded.
func (b *Backend) Wait(f gpu.Fence) {
	b.mu.Lock()
	defer b.mu.Unlock()

	resourceOf[*fence](b, f).destroy()
}

func (b *Backend) WaitIdle() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for len(b.fences) > 0 {
		b.fences[len(b.fences)-1].destroy()
	}
}

// ----------------------------------------------------------------------------
// GPU timing
// ----------------------------------------------------------------------------

func (b *Backend) CreateTimestampPool(count uint32) gpu.QueryPool {
	if count == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	p := &queryPool{slots: make([]uint64, count)}
	p.reference = reference{backend: b}
	return p
}

func (b *Backend) DestroyTimestampPool(p gpu.QueryPool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	destroyResource[*queryPool](b, p)
}

func (b *Backend) ReadTimestamps(pool gpu.QueryPool, count uint32) []uint64 {
	if pool == nil || count == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	p := resourceOf[*queryPool](b, pool)
	if !p.IsValid() {
		return nil
	}
	out := make([]uint64, count)
	copy(out, p.slots)
	return out
}

// TimestampPeriod is 1 nanosecond per tick: WriteTimestamp's fake clock already counts
// in whatever unit makes elapsed-time math trivial to check in a test.
func (b *Backend) TimestampPeriod() float64 {
	return 1
}

// GetMaxDataSize returns a generous limit: nothing in this backend actually reads
// draw/dispatch data, so there is no real ceiling to report.
func (b *Backend) GetMaxDataSize() int32 {
	return 1 << 16
}

func (b *Backend) Destroy() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for len(b.fences) > 0 {
		b.fences[len(b.fences)-1].destroy()
	}
	b.fences = nil
}
