//go:build darwin && cgo

package metal

type mtlSamplerDescriptor id

func (d mtlSamplerDescriptor) release() {
	release(d)
}

func (d mtlSamplerDescriptor) setLabel(label string) {
	setLabel(d, label)
}

func newMTLSamplerDescriptor() mtlSamplerDescriptor {
	return mtlSamplerDescriptor(newObject("MTLSamplerDescriptor"))
}

func (d mtlSamplerDescriptor) setMinFilter(v uint) {
	samplerDescriptorSetMinFilter(d, v)
}

func (d mtlSamplerDescriptor) setMagFilter(v uint) {
	samplerDescriptorSetMagFilter(d, v)
}

func (d mtlSamplerDescriptor) setMipFilter(v uint) {
	samplerDescriptorSetMipFilter(d, v)
}

func (d mtlSamplerDescriptor) setSAddressMode(v uint) {
	samplerDescriptorSetSAddressMode(d, v)
}

func (d mtlSamplerDescriptor) setTAddressMode(v uint) {
	samplerDescriptorSetTAddressMode(d, v)
}

func (d mtlSamplerDescriptor) setRAddressMode(v uint) {
	samplerDescriptorSetRAddressMode(d, v)
}

func (d mtlSamplerDescriptor) setCompareFunction(v uint) {
	samplerDescriptorSetCompareFunction(d, v)
}

func (d mtlSamplerDescriptor) setMaxAnisotropy(v uint) {
	samplerDescriptorSetMaxAnisotropy(d, v)
}

func (d mtlSamplerDescriptor) setSupportArgumentBuffers(v bool) {
	samplerDescriptorSetSupportArgumentBuffers(d, v)
}
