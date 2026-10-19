//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"math"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/internal/mapped"
)

type reference struct {
	backend   *Backend
	destroyed bool
}

type owned interface {
	vulkanReference() *reference
}

func (r *reference) IsValid() bool {
	return r.backend != nil && !r.destroyed
}

func (r *reference) vulkanReference() *reference {
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
	if !ok || r.vulkanReference().backend != b {
		panic("vulkan: resource belongs to another backend or has the wrong type")
	}

	return r
}

type buffer struct {
	reference
	buf  vkBuffer
	mem  vkDeviceMemory
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

	b.backend.freeBuffer(b)
}

type texture struct {
	reference
	img                  vkImage
	mem                  vkDeviceMemory
	view                 vkImageView
	format               vkFormat
	width, height, depth uint32
	isDepthFormat        bool
	writable             bool
	transient            bool
	// owned is false for views and swapchain backbuffers, which do not own their image.
	owned bool
	index uint32
}

func (t *texture) Index() uint32 {
	return t.index
}

func (t *texture) destroy() {
	if t == nil || !t.invalidate() {
		return
	}

	b := t.backend
	vkDestroyImageView(b.device, t.view, nil)
	if t.owned {
		vkDestroyImage(b.device, t.img, nil)
		vkFreeMemory(b.device, t.mem, nil)
	}
}

type sampler struct {
	reference
	native vkSampler
	index  uint32
}

func (s *sampler) Index() uint32 {
	return s.index
}

func (s *sampler) destroy() {
	if s == nil || !s.invalidate() {
		return
	}

	vkDestroySampler(s.backend.device, s.native, nil)
}

type pipeline struct {
	reference
	pipe      vkPipeline
	bindPoint vkPipelineBindPoint
}

func (p *pipeline) destroy() {
	if p == nil || !p.invalidate() {
		return
	}

	vkDestroyPipeline(p.backend.device, p.pipe, nil)
}

func (s *swapchain) destroy() {
	if s == nil || !s.invalidate() {
		return
	}

	s.backend.destroySwapchainResources(s)
	vkDestroySurfaceKHR(s.backend.instance, s.surface, nil)
}

type fence struct {
	reference
	native       vkFence
	cb           vkCommandBuffer
	pendingIndex int
}

func (f *fence) destroy() {
	if f == nil || !f.invalidate() {
		return
	}

	b := f.backend
	f.removePending()
	// Pass local handles to C. The fence also contains a Go-owned backend pointer.
	native, cb := f.native, f.cb
	vkWaitForFences(b.device, 1, &native, vkTrue, math.MaxUint64)
	vkDestroyFence(b.device, native, nil)
	vkFreeCommandBuffers(b.device, b.cmdPool, 1, &cb)
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
	native vkQueryPool
}

func (q *queryPool) destroy() {
	if q == nil || !q.invalidate() {
		return
	}

	vkDestroyQueryPool(q.backend.device, q.native, nil)
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
