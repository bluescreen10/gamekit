//go:build darwin && cgo

package metal

type renderPassDepthAttachmentDescriptor id

func (d renderPassDepthAttachmentDescriptor) release() {
	release(d)
}

func newRenderPassDepthAttachmentDescriptor() renderPassDepthAttachmentDescriptor {
	return renderPassDepthAttachmentDescriptor(newObject("MTLRenderPassDepthAttachmentDescriptor"))
}

func (d renderPassDepthAttachmentDescriptor) setClearDepth(v float64) {
	renderPassDepthAttachmentDescriptorSetClearDepth(d, v)
}

func (d renderPassDepthAttachmentDescriptor) setDepthResolveFilter(v uint) {
	renderPassDepthAttachmentDescriptorSetDepthResolveFilter(d, v)
}

func (d renderPassDepthAttachmentDescriptor) setTexture(t id) {
	renderPassAttachmentDescriptor(d).setTexture(t)
}

func (d renderPassDepthAttachmentDescriptor) setResolveTexture(t id) {
	renderPassAttachmentDescriptor(d).setResolveTexture(t)
}

func (d renderPassDepthAttachmentDescriptor) setLoadAction(v uint) {
	renderPassAttachmentDescriptor(d).setLoadAction(v)
}

func (d renderPassDepthAttachmentDescriptor) setStoreAction(v uint) {
	renderPassAttachmentDescriptor(d).setStoreAction(v)
}
