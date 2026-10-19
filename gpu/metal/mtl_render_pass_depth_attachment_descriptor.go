//go:build darwin && cgo

package metal

type mtlRenderPassDepthAttachmentDescriptor id

func (d mtlRenderPassDepthAttachmentDescriptor) release() {
	release(d)
}

func newMTLRenderPassDepthAttachmentDescriptor() mtlRenderPassDepthAttachmentDescriptor {
	return mtlRenderPassDepthAttachmentDescriptor(newObject("MTLRenderPassDepthAttachmentDescriptor"))
}

func (d mtlRenderPassDepthAttachmentDescriptor) setClearDepth(v float64) {
	renderPassDepthAttachmentDescriptorSetClearDepth(d, v)
}

func (d mtlRenderPassDepthAttachmentDescriptor) setDepthResolveFilter(v mtlMultisampleDepthResolveFilter) {
	renderPassDepthAttachmentDescriptorSetDepthResolveFilter(d, v)
}

func (d mtlRenderPassDepthAttachmentDescriptor) setTexture(t mtlTexture) {
	mtlRenderPassAttachmentDescriptor(d).setTexture(t)
}

func (d mtlRenderPassDepthAttachmentDescriptor) setResolveTexture(t mtlTexture) {
	mtlRenderPassAttachmentDescriptor(d).setResolveTexture(t)
}

func (d mtlRenderPassDepthAttachmentDescriptor) setLoadAction(v mtlLoadAction) {
	mtlRenderPassAttachmentDescriptor(d).setLoadAction(v)
}

func (d mtlRenderPassDepthAttachmentDescriptor) setStoreAction(v mtlStoreAction) {
	mtlRenderPassAttachmentDescriptor(d).setStoreAction(v)
}
