//go:build darwin && cgo

package metal

type mtl4RenderPassDescriptor id

func (d mtl4RenderPassDescriptor) release() {
	release(d)
}

func newMTL4RenderPassDescriptor() mtl4RenderPassDescriptor {
	return mtl4RenderPassDescriptor(newObject("MTL4RenderPassDescriptor"))
}

// clone returns an owned copy. The caller must release it.
func (d mtl4RenderPassDescriptor) clone() mtl4RenderPassDescriptor {
	return mtl4RenderPassDescriptor(objectCopy(id(d)))
}

func (d mtl4RenderPassDescriptor) colorAttachments() id {
	return renderPassDescriptorColorAttachments(d)
}

func (d mtl4RenderPassDescriptor) depthAttachment() mtlRenderPassDepthAttachmentDescriptor {
	return renderPassDescriptorDepthAttachment(d)
}
