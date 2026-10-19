//go:build darwin && cgo

package metal

type mtl4CounterHeap id

func (h mtl4CounterHeap) release() {
	release(h)
}

func (h mtl4CounterHeap) invalidateCounterRange(location, length uint) {
	counterHeapInvalidateCounterRange(h, location, length)
}

func (h mtl4CounterHeap) resolveCounterRange(location, length uint) id {
	return counterHeapResolveCounterRange(h, location, length)
}
