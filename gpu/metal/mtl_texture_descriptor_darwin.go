//go:build darwin && cgo

package metal

type mtlTextureDescriptor id

func (d mtlTextureDescriptor) release() {
	release(d)
}

func newMTLTextureDescriptor() mtlTextureDescriptor {
	return mtlTextureDescriptor(newObject("MTLTextureDescriptor"))
}

func (d mtlTextureDescriptor) setTextureType(v uint) {
	textureDescriptorSetTextureType(d, v)
}

func (d mtlTextureDescriptor) setWidth(v uint) {
	textureDescriptorSetWidth(d, v)
}

func (d mtlTextureDescriptor) setHeight(v uint) {
	textureDescriptorSetHeight(d, v)
}

func (d mtlTextureDescriptor) setDepth(v uint) {
	textureDescriptorSetDepth(d, v)
}

func (d mtlTextureDescriptor) setArrayLength(v uint) {
	textureDescriptorSetArrayLength(d, v)
}

func (d mtlTextureDescriptor) setMipmapLevelCount(v uint) {
	textureDescriptorSetMipmapLevelCount(d, v)
}

func (d mtlTextureDescriptor) setPixelFormat(v uint) {
	textureDescriptorSetPixelFormat(d, v)
}

func (d mtlTextureDescriptor) setSampleCount(v uint) {
	textureDescriptorSetSampleCount(d, v)
}

func (d mtlTextureDescriptor) setStorageMode(v uint) {
	textureDescriptorSetStorageMode(d, v)
}

func (d mtlTextureDescriptor) setUsage(v uint) {
	textureDescriptorSetUsage(d, v)
}
