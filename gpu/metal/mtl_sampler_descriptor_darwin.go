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

func (d mtlSamplerDescriptor) setMinFilter(v mtlSamplerMinMagFilter) {
	samplerDescriptorSetMinFilter(d, v)
}

func (d mtlSamplerDescriptor) setMagFilter(v mtlSamplerMinMagFilter) {
	samplerDescriptorSetMagFilter(d, v)
}

func (d mtlSamplerDescriptor) setMipFilter(v mtlSamplerMipFilter) {
	samplerDescriptorSetMipFilter(d, v)
}

func (d mtlSamplerDescriptor) setSAddressMode(v mtlSamplerAddressMode) {
	samplerDescriptorSetSAddressMode(d, v)
}

func (d mtlSamplerDescriptor) setTAddressMode(v mtlSamplerAddressMode) {
	samplerDescriptorSetTAddressMode(d, v)
}

func (d mtlSamplerDescriptor) setRAddressMode(v mtlSamplerAddressMode) {
	samplerDescriptorSetRAddressMode(d, v)
}

func (d mtlSamplerDescriptor) setCompareFunction(v mtlCompareFunction) {
	samplerDescriptorSetCompareFunction(d, v)
}

func (d mtlSamplerDescriptor) setMaxAnisotropy(v uint) {
	samplerDescriptorSetMaxAnisotropy(d, v)
}

func (d mtlSamplerDescriptor) setSupportArgumentBuffers(v bool) {
	samplerDescriptorSetSupportArgumentBuffers(d, v)
}
