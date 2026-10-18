//go:build darwin && cgo

package metal

type allocator id

func (a allocator) release() {
	release(a)
}

func (a allocator) reset() {
	commandAllocatorReset(a)
}
