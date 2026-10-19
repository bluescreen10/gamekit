package test

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
	testReference() *reference
}

func (r *reference) IsValid() bool {
	return r.backend != nil && !r.destroyed
}

func (r *reference) testReference() *reference {
	return r
}

func (r *reference) destroy() {
	if r == nil || !r.IsValid() {
		return
	}

	r.destroyed = true
}

func resourceOf[T owned](b *Backend, value any) T {
	if value == nil {
		var zero T
		return zero
	}

	r, ok := value.(T)
	if !ok || r.testReference().backend != b {
		panic("gpu/test: resource belongs to another backend or has the wrong type")
	}

	return r
}

func destroyResource[T owned](b *Backend, value any) {
	if value == nil {
		return
	}

	resourceOf[T](b, value).testReference().destroy()
}

func (b *buffer) Addr() uint64 {
	return uint64(uintptr(unsafe.Pointer(unsafe.SliceData(b.data))))
}

func (b *buffer) Ptr() unsafe.Pointer {
	if !b.mapped || len(b.data) == 0 {
		return nil
	}

	return unsafe.Pointer(&b.data[0])
}

func (b *buffer) Size() uint64 {
	return uint64(len(b.data))
}

func (b *buffer) Write(data []byte, offset uint64) {
	mapped.Write(b.Ptr(), b.Size(), data, offset)
}

func (t *texture) Index() uint32 {
	return t.index
}

type sampler struct {
	reference
	index uint32
}

func (s *sampler) Index() uint32 {
	return s.index
}

type pipeline struct{ reference }
type fence struct {
	reference
	pendingIndex int
}

func (f *fence) destroy() {
	if f == nil || !f.IsValid() {
		return
	}

	f.reference.destroy()
	b := f.backend
	last := len(b.fences) - 1
	b.fences[f.pendingIndex] = b.fences[last]
	b.fences[f.pendingIndex].pendingIndex = f.pendingIndex
	b.fences[last] = nil
	b.fences = b.fences[:last]
}

type queryPool struct {
	reference
	slots []uint64
}

var (
	_ gpu.Buffer    = (*buffer)(nil)
	_ gpu.Texture   = (*texture)(nil)
	_ gpu.Sampler   = (*sampler)(nil)
	_ gpu.Pipeline  = (*pipeline)(nil)
	_ gpu.Swapchain = (*swapchainState)(nil)
	_ gpu.Fence     = (*fence)(nil)
	_ gpu.QueryPool = (*queryPool)(nil)
)
