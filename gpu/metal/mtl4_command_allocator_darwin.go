//go:build darwin && cgo

package metal

type mtl4CommandAllocator id

func (a mtl4CommandAllocator) release() {
	release(a)
}

func (a mtl4CommandAllocator) reset() {
	commandAllocatorReset(a)
}
