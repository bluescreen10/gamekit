//go:build darwin && cgo

package metal

type argumentTableDescriptor id

func (d argumentTableDescriptor) release() {
	release(d)
}

func newArgumentTableDescriptor() argumentTableDescriptor {
	return argumentTableDescriptor(newObject("MTL4ArgumentTableDescriptor"))
}

func (d argumentTableDescriptor) setMaxBufferBindCount(n uint) {
	argumentTableDescriptorSetMaxBufferBindCount(d, n)
}
