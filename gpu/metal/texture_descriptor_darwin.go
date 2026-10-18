//go:build darwin && cgo

package metal

type textureDescriptor id

func (d textureDescriptor) release() {
	release(d)
}

func newTextureDescriptor() textureDescriptor {
	return textureDescriptor(newObject("MTLTextureDescriptor"))
}

func (d textureDescriptor) setTextureType(v uint) {
	textureDescriptorSetTextureType(d, v)
}

func (d textureDescriptor) setWidth(v uint) {
	textureDescriptorSetWidth(d, v)
}

func (d textureDescriptor) setHeight(v uint) {
	textureDescriptorSetHeight(d, v)
}

func (d textureDescriptor) setDepth(v uint) {
	textureDescriptorSetDepth(d, v)
}

func (d textureDescriptor) setArrayLength(v uint) {
	textureDescriptorSetArrayLength(d, v)
}

func (d textureDescriptor) setMipmapLevelCount(v uint) {
	textureDescriptorSetMipmapLevelCount(d, v)
}

func (d textureDescriptor) setPixelFormat(v uint) {
	textureDescriptorSetPixelFormat(d, v)
}

func (d textureDescriptor) setSampleCount(v uint) {
	textureDescriptorSetSampleCount(d, v)
}

func (d textureDescriptor) setStorageMode(v uint) {
	textureDescriptorSetStorageMode(d, v)
}

func (d textureDescriptor) setUsage(v uint) {
	textureDescriptorSetUsage(d, v)
}
