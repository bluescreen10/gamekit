package test

import (
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/internal/mapped"
)

type reference struct {
	backend *Backend
	value   uint64
}

func (r reference) IsValid() bool {
	return r.backend != nil && r.value != 0
}

func (r reference) testReference() reference {
	return r
}

func (b *Backend) handle(value any) uint64 {
	if value == nil {
		return 0
	}

	r, ok := value.(interface{ testReference() reference })
	if !ok || r.testReference().backend != b {
		panic("gpu/test: resource belongs to another backend")
	}

	return r.testReference().value
}

func (b *buffer) Addr() uint64 {
	return b.value
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
type fence struct{ reference }
type queryPool struct{ reference }

var (
	_ gpu.Buffer    = (*buffer)(nil)
	_ gpu.Texture   = (*texture)(nil)
	_ gpu.Sampler   = (*sampler)(nil)
	_ gpu.Pipeline  = (*pipeline)(nil)
	_ gpu.Swapchain = (*swapchainState)(nil)
	_ gpu.Fence     = (*fence)(nil)
	_ gpu.QueryPool = (*queryPool)(nil)
)
