//go:build darwin && cgo

package metal

type mtlRenderPassAttachmentDescriptor id

func (d mtlRenderPassAttachmentDescriptor) release() {
	release(d)
}

func newMTLRenderPassAttachmentDescriptor() mtlRenderPassAttachmentDescriptor {
	return mtlRenderPassAttachmentDescriptor(newObject("MTLRenderPassAttachmentDescriptor"))
}

func (d mtlRenderPassAttachmentDescriptor) setTexture(t mtlTexture) {
	renderPassAttachmentDescriptorSetTexture(d, t)
}

func (d mtlRenderPassAttachmentDescriptor) setResolveTexture(t mtlTexture) {
	renderPassAttachmentDescriptorSetResolveTexture(d, t)
}

func (d mtlRenderPassAttachmentDescriptor) setLoadAction(v uint) {
	renderPassAttachmentDescriptorSetLoadAction(d, v)
}

func (d mtlRenderPassAttachmentDescriptor) setStoreAction(v uint) {
	renderPassAttachmentDescriptorSetStoreAction(d, v)
}
