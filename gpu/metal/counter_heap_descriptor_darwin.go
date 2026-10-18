//go:build darwin && cgo

package metal

type counterHeapDescriptor id

func (d counterHeapDescriptor) release() {
	release(d)
}

func newCounterHeapDescriptor() counterHeapDescriptor {
	return counterHeapDescriptor(newObject("MTL4CounterHeapDescriptor"))
}

func (d counterHeapDescriptor) setType(v uint) {
	counterHeapDescriptorSetType(d, v)
}

func (d counterHeapDescriptor) setCount(v uint) {
	counterHeapDescriptorSetCount(d, v)
}
