//go:build darwin && cgo

package metal

type samplerDescriptor id

func (d samplerDescriptor) release() {
	release(d)
}

func newSamplerDescriptor() samplerDescriptor {
	return samplerDescriptor(newObject("MTLSamplerDescriptor"))
}

func (d samplerDescriptor) setMinFilter(v uint) {
	samplerDescriptorSetMinFilter(d, v)
}

func (d samplerDescriptor) setMagFilter(v uint) {
	samplerDescriptorSetMagFilter(d, v)
}

func (d samplerDescriptor) setMipFilter(v uint) {
	samplerDescriptorSetMipFilter(d, v)
}

func (d samplerDescriptor) setSAddressMode(v uint) {
	samplerDescriptorSetSAddressMode(d, v)
}

func (d samplerDescriptor) setTAddressMode(v uint) {
	samplerDescriptorSetTAddressMode(d, v)
}

func (d samplerDescriptor) setRAddressMode(v uint) {
	samplerDescriptorSetRAddressMode(d, v)
}

func (d samplerDescriptor) setCompareFunction(v uint) {
	samplerDescriptorSetCompareFunction(d, v)
}

func (d samplerDescriptor) setMaxAnisotropy(v uint) {
	samplerDescriptorSetMaxAnisotropy(d, v)
}

func (d samplerDescriptor) setSupportArgumentBuffers(v bool) {
	samplerDescriptorSetSupportArgumentBuffers(d, v)
}
