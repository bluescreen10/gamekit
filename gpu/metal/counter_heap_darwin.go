//go:build darwin && cgo

package metal

type counterHeap id

func (h counterHeap) release() {
	release(h)
}

func (h counterHeap) invalidateCounterRange(location, length uint) {
	counterHeapInvalidateCounterRange(h, location, length)
}

func (h counterHeap) resolveCounterRange(location, length uint) id {
	return counterHeapResolveCounterRange(h, location, length)
}
