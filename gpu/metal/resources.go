//go:build darwin && cgo

package metal

import (
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/internal/mapped"
)

// reference keeps registry identity private to this backend. Resource accessors
// read the metadata directly rather than search the registry.
type reference struct {
	backend  *Backend
	value    uint64
	metadata *resource
}

func (r reference) IsValid() bool {
	return r.backend != nil && r.value != 0
}

func (r reference) metalReference() reference {
	return r
}

func (b *Backend) reference(value uint64) reference {
	return reference{backend: b, value: value, metadata: b.resources[value]}
}

func (b *Backend) handle(value any) uint64 {
	if value == nil {
		return 0
	}

	r, ok := value.(interface{ metalReference() reference })
	check(ok && r.metalReference().backend == b, "resource belongs to another backend")

	return r.metalReference().value
}

type buffer struct {
	reference
	mtlBuffer
}

func (b *buffer) Addr() uint64 {
	return b.metadata.addr
}

func (b *buffer) Ptr() unsafe.Pointer {
	return b.metadata.ptr
}

func (b *buffer) Size() uint64 {
	return b.metadata.size
}

func (b *buffer) Write(data []byte, offset uint64) {
	mapped.Write(b.Ptr(), b.Size(), data, offset)
}

type texture struct {
	reference
	mtlTexture
}

func (t *texture) Index() uint32 {
	return t.metadata.slot
}

type sampler struct {
	reference
	mtlSamplerState
}

func (s *sampler) Index() uint32 {
	return s.metadata.slot
}

type pipeline struct{ reference }

type swapchain struct {
	reference
	caMetalLayer
}

type fence struct{ reference }

type queryPool struct {
	reference
	mtl4CounterHeap
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
