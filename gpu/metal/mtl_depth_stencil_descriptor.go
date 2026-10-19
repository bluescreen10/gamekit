//go:build darwin && cgo

package metal

type mtlDepthStencilDescriptor id

func (d mtlDepthStencilDescriptor) release() {
	release(d)
}

func newMTLDepthStencilDescriptor() mtlDepthStencilDescriptor {
	return mtlDepthStencilDescriptor(newObject("MTLDepthStencilDescriptor"))
}

func (d mtlDepthStencilDescriptor) setDepthCompareFunction(v mtlCompareFunction) {
	depthStencilDescriptorSetDepthCompareFunction(d, v)
}

func (d mtlDepthStencilDescriptor) setDepthWriteEnabled(v bool) {
	depthStencilDescriptorSetDepthWriteEnabled(d, v)
}
