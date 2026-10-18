//go:build darwin && cgo

package metal

type depthStencilDescriptor id

func (d depthStencilDescriptor) release() {
	release(d)
}

func newDepthStencilDescriptor() depthStencilDescriptor {
	return depthStencilDescriptor(newObject("MTLDepthStencilDescriptor"))
}

func (d depthStencilDescriptor) setDepthCompareFunction(v uint) {
	depthStencilDescriptorSetDepthCompareFunction(d, v)
}

func (d depthStencilDescriptor) setDepthWriteEnabled(v bool) {
	depthStencilDescriptorSetDepthWriteEnabled(d, v)
}
