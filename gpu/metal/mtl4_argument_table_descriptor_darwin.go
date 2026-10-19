//go:build darwin && cgo

package metal

type mtl4ArgumentTableDescriptor id

func (d mtl4ArgumentTableDescriptor) release() {
	release(d)
}

func newMTL4ArgumentTableDescriptor() mtl4ArgumentTableDescriptor {
	return mtl4ArgumentTableDescriptor(newObject("MTL4ArgumentTableDescriptor"))
}

func (d mtl4ArgumentTableDescriptor) setMaxBufferBindCount(n uint) {
	argumentTableDescriptorSetMaxBufferBindCount(d, n)
}
