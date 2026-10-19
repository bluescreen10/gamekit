//go:build darwin && cgo

package metal

import (
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/internal/mapped"
)

type reference struct {
	backend   *Backend
	destroyed bool
}

type owned interface {
	metalReference() *reference
}

func (r *reference) IsValid() bool {
	return r.backend != nil && !r.destroyed
}

func (r *reference) metalReference() *reference {
	return r
}

func (r *reference) invalidate() bool {
	if !r.IsValid() {
		return false
	}

	r.destroyed = true
	return true
}

func resourceOf[T owned](b *Backend, value any) T {
	if value == nil {
		var zero T
		return zero
	}

	r, ok := value.(T)
	check(ok && r.metalReference().backend == b, "resource belongs to another backend or has the wrong type")
	return r
}

type buffer struct {
	reference
	mtlBuffer
	size uint64
	addr uint64
	ptr  unsafe.Pointer
}

func (b *buffer) Addr() uint64 {
	return b.addr
}

func (b *buffer) Ptr() unsafe.Pointer {
	return b.ptr
}

func (b *buffer) Size() uint64 {
	return b.size
}

func (b *buffer) Write(data []byte, offset uint64) {
	mapped.Write(b.ptr, b.size, data, offset)
}

func (b *buffer) destroy() {
	if b == nil || !b.invalidate() {
		return
	}

	b.backend.residencySet.unenrol(id(b.mtlBuffer))
	b.backend.releaseLater(id(b.mtlBuffer))
}

type texture struct {
	reference
	mtlTexture
	slot                  uint32
	width, height, depth  uint64
	mips, layers, samples uint64
	textureType           mtlTextureType
	format                mtlPixelFormat
	memoryless            bool
	transient             bool
	resident              bool
}

func (t *texture) Index() uint32 {
	return t.slot
}

func (t *texture) destroy() {
	if t == nil || !t.invalidate() {
		return
	}

	b := t.backend
	if t.slot != 0 {
		b.textureIDs[t.slot] = 0
		b.freeTextures = append(b.freeTextures, t.slot)
	}
	if t.resident {
		b.residencySet.unenrol(id(t.mtlTexture))
	}

	b.releaseLater(id(t.mtlTexture))
}

type sampler struct {
	reference
	mtlSamplerState
	slot uint32
}

func (s *sampler) Index() uint32 {
	return s.slot
}

func (s *sampler) destroy() {
	if s == nil || !s.invalidate() {
		return
	}

	b := s.backend
	if s.slot != 0 {
		b.samplerIDs[s.slot] = 0
		b.freeSamplers = append(b.freeSamplers, s.slot)
	}

	b.releaseLater(id(s.mtlSamplerState))
}

type pipeline struct {
	reference
	render       mtlRenderPipelineState
	compute      mtlComputePipelineState
	depthStencil mtlDepthStencilState
	topology     mtlPrimitiveType
	cull         mtlCullMode
	winding      mtlWinding
	depthWrite   bool
	group        [3]uint
	// rootSize is the reflected size of the shader's root argument at buffer(0).
	rootSize uint64
}

func (p *pipeline) destroy() {
	if p == nil || !p.invalidate() {
		return
	}

	p.backend.releaseLater(id(p.render))
	p.backend.releaseLater(id(p.compute))
	p.backend.releaseLater(id(p.depthStencil))
}

type swapchain struct {
	reference
	caMetalLayer
	format          gpu.Format
	drawable        mtlDrawable
	drawableTexture *texture
}

func (s *swapchain) destroy() {
	if s == nil || !s.invalidate() {
		return
	}

	s.drawableTexture.destroy()
	s.drawable.release()
	s.caMetalLayer.release()
}

type fence struct {
	reference
	value        uint64
	pendingIndex int
}

func (f *fence) destroy() {
	if f == nil || !f.invalidate() {
		return
	}

	f.removePending()
	f.backend.completed(f.value)
}

// removePending removes a completed submission without a resource registry.
func (f *fence) removePending() {
	b := f.backend
	last := len(b.fences) - 1
	b.fences[f.pendingIndex] = b.fences[last]
	b.fences[f.pendingIndex].pendingIndex = f.pendingIndex
	b.fences[last] = nil
	b.fences = b.fences[:last]
}

type queryPool struct {
	reference
	mtl4CounterHeap
	written            map[uint32]bool
	overrides          map[uint32]uint64
	cpuStart, gpuStart uint64
	count              uint64
}

func (q *queryPool) destroy() {
	if q == nil || !q.invalidate() {
		return
	}

	q.backend.releaseLater(id(q.mtl4CounterHeap))
}

var (
	_ gpu.Buffer    = (*buffer)(nil)
	_ gpu.Texture   = (*texture)(nil)
	_ gpu.Sampler   = (*sampler)(nil)
	_ gpu.Pipeline  = (*pipeline)(nil)
	_ gpu.Swapchain = (*swapchain)(nil)
	_ gpu.Fence     = (*fence)(nil)
	_ gpu.QueryPool = (*queryPool)(nil)
)
