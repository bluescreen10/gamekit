//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

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

func (r reference) vulkanReference() reference {
	return r
}

func (b *Backend) handle(value any) uint64 {
	if value == nil {
		return 0
	}

	r, ok := value.(interface{ vulkanReference() reference })
	if !ok || r.vulkanReference().backend != b {
		panic("vulkan: resource belongs to another backend")
	}

	return r.vulkanReference().value
}

type buffer struct {
	reference
	bufferEntry
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

type texture struct {
	reference
	*textureEntry
	index uint32
}

func (t *texture) Index() uint32 {
	return t.index
}

type sampler struct {
	reference
	native vkSampler
	index  uint32
}

func (s *sampler) Index() uint32 {
	return s.index
}

type pipeline struct{ reference }
type swapchain struct{ reference }
type fence struct{ reference }
type queryPool struct{ reference }

var (
	_ gpu.Buffer    = (*buffer)(nil)
	_ gpu.Texture   = (*texture)(nil)
	_ gpu.Sampler   = (*sampler)(nil)
	_ gpu.Pipeline  = (*pipeline)(nil)
	_ gpu.Swapchain = (*swapchain)(nil)
	_ gpu.Fence     = (*fence)(nil)
	_ gpu.QueryPool = (*queryPool)(nil)
)
