//go:build darwin && cgo

package metal

type mtlRenderPassColorAttachmentDescriptor id

func (d mtlRenderPassColorAttachmentDescriptor) release() {
	release(d)
}

func newMTLRenderPassColorAttachmentDescriptor() mtlRenderPassColorAttachmentDescriptor {
	return mtlRenderPassColorAttachmentDescriptor(newObject("MTLRenderPassColorAttachmentDescriptor"))
}

func (d mtlRenderPassColorAttachmentDescriptor) setClearColor(red, green, blue, alpha float64) {
	renderPassColorAttachmentDescriptorSetClearColor(d, red, green, blue, alpha)
}

func (d mtlRenderPassColorAttachmentDescriptor) setTexture(t mtlTexture) {
	mtlRenderPassAttachmentDescriptor(d).setTexture(t)
}

func (d mtlRenderPassColorAttachmentDescriptor) setResolveTexture(t mtlTexture) {
	mtlRenderPassAttachmentDescriptor(d).setResolveTexture(t)
}

func (d mtlRenderPassColorAttachmentDescriptor) setLoadAction(v uint) {
	mtlRenderPassAttachmentDescriptor(d).setLoadAction(v)
}

func (d mtlRenderPassColorAttachmentDescriptor) setStoreAction(v uint) {
	mtlRenderPassAttachmentDescriptor(d).setStoreAction(v)
}
