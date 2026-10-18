//go:build darwin && cgo

package metal

type renderPassDescriptor id

func (d renderPassDescriptor) release() {
	release(d)
}

func newRenderPassDescriptor() renderPassDescriptor {
	return renderPassDescriptor(newObject("MTL4RenderPassDescriptor"))
}

// clone returns an owned copy. The caller must release it.
func (d renderPassDescriptor) clone() renderPassDescriptor {
	return renderPassDescriptor(objectCopy(id(d)))
}

func (d renderPassDescriptor) colorAttachments() id {
	return renderPassDescriptorColorAttachments(d)
}

func (d renderPassDescriptor) depthAttachment() renderPassDepthAttachmentDescriptor {
	return renderPassDescriptorDepthAttachment(d)
}
