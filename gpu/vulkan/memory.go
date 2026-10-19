//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// bufferUsage is every usage the bindless model needs, so a single Alloc serves as
// storage / index / indirect / transfer / uniform and exposes a device address.
const bufferUsage = vkBufferUsageStorageBufferBit | vkBufferUsageUniformBufferBit |
	vkBufferUsageIndexBufferBit | vkBufferUsageIndirectBufferBit |
	vkBufferUsageTransferSrcBit | vkBufferUsageTransferDstBit |
	vkBufferUsageShaderDeviceAddressBit

// Alloc creates a dedicated buffer + memory. MemoryHost is CPU-mapped and
// coherent (Buffer.Ptr set); MemoryDevice is GPU-only (Ptr nil). Both expose a
// device address (Buffer.Addr). The engine suballocates within an Alloc via
// offsets on Addr — the gpu itself does no suballocation.
func (b *Backend) Alloc(size uint64, mem gpu.MemoryType, label string) gpu.Buffer {
	e, r := b.allocBuffer(size, mem == gpu.MemoryHost)
	if r != vkSuccess {
		panic(fmt.Sprintf("vulkan: Alloc(%d bytes, %q) failed (%d)", size, label, int(r)))
	}

	e.reference = reference{backend: b}
	return e
}

// allocBuffer creates a buffer with one dedicated allocation, binds them, maps
// host-visible memory, and returns the device address.
func (b *Backend) allocBuffer(size uint64, hostVisible bool) (e *buffer, r vkResult) {
	e = &buffer{size: size}
	bi := vkBufferCreateInfo{
		sType:       vkStructureTypeBufferCreateInfo,
		size:        vkDeviceSize(size),
		usage:       bufferUsage,
		sharingMode: vkSharingModeExclusive,
	}
	if r = vkCreateBuffer(b.device, &bi, nil, &e.buf); r != vkSuccess {
		return e, r
	}

	var req vkMemoryRequirements
	vkGetBufferMemoryRequirements(b.device, e.buf, &req)
	want := vkMemoryPropertyFlags(vkMemoryPropertyDeviceLocalBit)
	if hostVisible {
		want = vkMemoryPropertyHostVisibleBit | vkMemoryPropertyHostCoherentBit
	}
	typeIndex := b.memoryType(req.memoryTypeBits, want)
	if typeIndex == noMemoryType {
		vkDestroyBuffer(b.device, e.buf, nil)
		return e, vkErrorOutOfDeviceMemory
	}

	var pin runtime.Pinner
	defer pin.Unpin()
	ai := vkMemoryAllocateInfo{
		sType: vkStructureTypeMemoryAllocateInfo,
		pNext: unsafe.Pointer(pinned(&pin, &vkMemoryAllocateFlagsInfo{
			sType: vkStructureTypeMemoryAllocateFlagsInfo,
			flags: vkMemoryAllocateDeviceAddressBit,
		})),
		allocationSize:  req.size,
		memoryTypeIndex: cUint32(typeIndex),
	}
	if r = vkAllocateMemory(b.device, &ai, nil, &e.mem); r != vkSuccess {
		vkDestroyBuffer(b.device, e.buf, nil)
		return e, r
	}
	if r = vkBindBufferMemory(b.device, e.buf, e.mem, 0); r != vkSuccess {
		b.freeBuffer(e)
		return e, r
	}
	if hostVisible {
		if r = vkMapMemory(b.device, e.mem, 0, vkWholeSize, 0, &e.ptr); r != vkSuccess {
			b.freeBuffer(e)
			return e, r
		}
	}

	dai := vkBufferDeviceAddressInfo{sType: vkStructureTypeBufferDeviceAddressInfo, buffer: e.buf}
	e.addr = uint64(vkGetBufferDeviceAddress(b.device, &dai))
	return e, vkSuccess
}

// memoryType returns the first memory type allowed by typeBits that has every
// property in want, or noMemoryType.
func (b *Backend) memoryType(typeBits cUint32, want vkMemoryPropertyFlags) uint32 {
	mp := &b.memoryProperties
	for i := range uint32(mp.memoryTypeCount) {
		if typeBits&(1<<i) != 0 && mp.memoryTypes[i].propertyFlags&want == want {
			return i
		}
	}
	return noMemoryType
}

func (b *Backend) freeBuffer(e *buffer) {
	vkDestroyBuffer(b.device, e.buf, nil)
	vkFreeMemory(b.device, e.mem, nil)
}

// Free destroys the buffer and releases its memory. Repeated calls do nothing.
func (b *Backend) Free(buf gpu.Buffer) {
	resourceOf[*buffer](b, buf).destroy()
}
