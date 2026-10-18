//go:build darwin && cgo

package metal

type renderPassAttachmentDescriptor id

func (d renderPassAttachmentDescriptor) release() {
	release(d)
}

func newRenderPassAttachmentDescriptor() renderPassAttachmentDescriptor {
	return renderPassAttachmentDescriptor(newObject("MTLRenderPassAttachmentDescriptor"))
}

func (d renderPassAttachmentDescriptor) setTexture(t id) {
	renderPassAttachmentDescriptorSetTexture(d, t)
}

func (d renderPassAttachmentDescriptor) setResolveTexture(t id) {
	renderPassAttachmentDescriptorSetResolveTexture(d, t)
}

func (d renderPassAttachmentDescriptor) setLoadAction(v uint) {
	renderPassAttachmentDescriptorSetLoadAction(d, v)
}

func (d renderPassAttachmentDescriptor) setStoreAction(v uint) {
	renderPassAttachmentDescriptorSetStoreAction(d, v)
}
