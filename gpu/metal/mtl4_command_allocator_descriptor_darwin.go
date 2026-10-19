//go:build darwin && cgo

package metal

type mtl4CommandAllocatorDescriptor id

func (d mtl4CommandAllocatorDescriptor) release() {
	release(d)
}

func newMTL4CommandAllocatorDescriptor() mtl4CommandAllocatorDescriptor {
	return mtl4CommandAllocatorDescriptor(newObject("MTL4CommandAllocatorDescriptor"))
}
