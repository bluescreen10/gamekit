//go:build darwin && cgo

package metal

type commandAllocatorDescriptor id

func (d commandAllocatorDescriptor) release() {
	release(d)
}

func newCommandAllocatorDescriptor() commandAllocatorDescriptor {
	return commandAllocatorDescriptor(newObject("MTL4CommandAllocatorDescriptor"))
}
