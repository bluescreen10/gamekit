//go:build darwin && cgo

package metal

type mtl4CounterHeapDescriptor id

func (d mtl4CounterHeapDescriptor) release() {
	release(d)
}

func newMTL4CounterHeapDescriptor() mtl4CounterHeapDescriptor {
	return mtl4CounterHeapDescriptor(newObject("MTL4CounterHeapDescriptor"))
}

func (d mtl4CounterHeapDescriptor) setType(v mtl4CounterHeapType) {
	counterHeapDescriptorSetType(d, v)
}

func (d mtl4CounterHeapDescriptor) setCount(v uint) {
	counterHeapDescriptorSetCount(d, v)
}
