//go:build darwin && cgo

package metal

type renderPassColorAttachmentDescriptor id

func (d renderPassColorAttachmentDescriptor) release() {
	release(d)
}

func newRenderPassColorAttachmentDescriptor() renderPassColorAttachmentDescriptor {
	return renderPassColorAttachmentDescriptor(newObject("MTLRenderPassColorAttachmentDescriptor"))
}

func (d renderPassColorAttachmentDescriptor) setClearColor(red, green, blue, alpha float64) {
	renderPassColorAttachmentDescriptorSetClearColor(d, red, green, blue, alpha)
}

func (d renderPassColorAttachmentDescriptor) setTexture(t id) {
	renderPassAttachmentDescriptor(d).setTexture(t)
}

func (d renderPassColorAttachmentDescriptor) setResolveTexture(t id) {
	renderPassAttachmentDescriptor(d).setResolveTexture(t)
}

func (d renderPassColorAttachmentDescriptor) setLoadAction(v uint) {
	renderPassAttachmentDescriptor(d).setLoadAction(v)
}

func (d renderPassColorAttachmentDescriptor) setStoreAction(v uint) {
	renderPassAttachmentDescriptor(d).setStoreAction(v)
}
