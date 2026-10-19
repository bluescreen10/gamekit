//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

func (b *Backend) initCommands() error {
	ci := vkCommandPoolCreateInfo{
		sType:            vkStructureTypeCommandPoolCreateInfo,
		flags:            vkCommandPoolCreateResetCommandBufferBit,
		queueFamilyIndex: cUint32(b.queueFamily),
	}
	var pool vkCommandPool
	r := vkCreateCommandPool(b.device, &ci, nil, &pool)
	b.cmdPool = pool
	if r != vkSuccess {
		return fmt.Errorf("vulkan: command pool creation failed (%d)", int(r))
	}
	return nil
}

func (b *Backend) destroyCommands() {
	if b.cmdPool != nil {
		vkDestroyCommandPool(b.device, b.cmdPool, nil)
	}
}

// cmdBuffer is the Vulkan CommandBuffer: a primary command buffer that has the
// bindless heap bound and the shared pipeline layout for push constants.
type cmdBuffer struct {
	backend *Backend
	cb      vkCommandBuffer
	// inRenderPass is set between BeginRenderPass and EndRenderPass, where no image
	// can be initialized: a barrier inside dynamic rendering may not change layouts.
	inRenderPass bool
}

// Begin allocates a transient command buffer, begins recording, and binds the
// global bindless descriptor set.
func (b *Backend) Begin() gpu.CommandBuffer {
	c := &cmdBuffer{backend: b}
	var cb vkCommandBuffer
	ai := vkCommandBufferAllocateInfo{
		sType:              vkStructureTypeCommandBufferAllocateInfo,
		commandPool:        b.cmdPool,
		level:              vkCommandBufferLevelPrimary,
		commandBufferCount: 1,
	}
	if r := vkAllocateCommandBuffers(b.device, &ai, &cb); r != vkSuccess {
		panic(fmt.Sprintf("vulkan: allocate command buffer failed (%d)", int(r)))
	}
	c.cb = cb
	bi := vkCommandBufferBeginInfo{
		sType: vkStructureTypeCommandBufferBeginInfo,
		flags: vkCommandBufferUsageOneTimeSubmitBit,
	}
	if r := vkBeginCommandBuffer(c.cb, &bi); r != vkSuccess {
		panic(fmt.Sprintf("vulkan: begin command buffer failed (%d)", int(r)))
	}
	// Bind the global bindless set at both bind points for the whole recording.
	sets := &b.scratch.sets
	sets[0] = b.descSet
	vkCmdBindDescriptorSets(c.cb, vkPipelineBindPointGraphics, b.pipelineLayout, 0, 1, &sets[0], 0, nil)
	vkCmdBindDescriptorSets(c.cb, vkPipelineBindPointCompute, b.pipelineLayout, 0, 1, &sets[0], 0, nil)
	c.initializeImages()
	return c
}

// Submit ends recording and submits, returning a fence for the completion.
func (b *Backend) Submit(cmd gpu.CommandBuffer) gpu.Fence {
	c := cmd.(*cmdBuffer)
	vkEndCommandBuffer(c.cb)
	fi := vkFenceCreateInfo{sType: vkStructureTypeFenceCreateInfo}
	var nativeFence vkFence
	if r := vkCreateFence(b.device, &fi, nil, &nativeFence); r != vkSuccess {
		panic(fmt.Sprintf("vulkan: create fence failed (%d)", int(r)))
	}
	if r := b.submit(c.cb, nil, nil, nativeFence); r != vkSuccess {
		panic(fmt.Sprintf("vulkan: submit failed (%d)", int(r)))
	}
	f := &fence{reference: reference{backend: b}, cb: c.cb, native: nativeFence, pendingIndex: len(b.fences)}
	b.fences = append(b.fences, f)
	return f
}

// Wait blocks until the fence signals, then frees the command buffer + fence.
func (b *Backend) Wait(f gpu.Fence) {
	resourceOf[*fence](b, f).destroy()
}

// submit submits cb to the queue, signalling fence. A non-nil acquire is waited on
// before color output, and a non-nil renderDone signalled when cb completes.
func (b *Backend) submit(cb vkCommandBuffer, acquire, renderDone vkSemaphore, fence vkFence) vkResult {
	s := b.scratch
	s.cmdInfo = vkCommandBufferSubmitInfo{sType: vkStructureTypeCommandBufferSubmitInfo, commandBuffer: cb}
	s.submit = vkSubmitInfo2{
		sType:                  vkStructureTypeSubmitInfo2,
		commandBufferInfoCount: 1,
		pCommandBufferInfos:    &s.cmdInfo,
	}
	if acquire != nil {
		s.wait = vkSemaphoreSubmitInfo{
			sType:     vkStructureTypeSemaphoreSubmitInfo,
			semaphore: acquire,
			stageMask: vkPipelineStage2ColorAttachmentOutputBit,
		}
		s.submit.waitSemaphoreInfoCount = 1
		s.submit.pWaitSemaphoreInfos = &s.wait
	}
	if renderDone != nil {
		s.signal = vkSemaphoreSubmitInfo{
			sType:     vkStructureTypeSemaphoreSubmitInfo,
			semaphore: renderDone,
			stageMask: vkPipelineStage2AllCommandsBit,
		}
		s.submit.signalSemaphoreInfoCount = 1
		s.submit.pSignalSemaphoreInfos = &s.signal
	}
	return vkQueueSubmit2(b.queue, 1, &s.submit, fence)
}

// WaitIdle blocks until the device is idle.
func (b *Backend) WaitIdle() {
	vkDeviceWaitIdle(b.device)
	for len(b.fences) > 0 {
		b.fences[len(b.fences)-1].destroy()
	}
}

// --- CommandBuffer ---

func (b *Backend) tex(t gpu.Texture) *texture {
	return resourceOf[*texture](b, t)
}

func (b *Backend) bufRaw(buf gpu.Buffer) vkBuffer {
	return resourceOf[*buffer](b, buf).buf
}

func aspectOf(e *texture) vkImageAspectFlags {
	if e.isDepthFormat {
		return vkImageAspectDepthBit
	}
	return vkImageAspectColorBit
}

// initializeImages moves every texture still in UNDEFINED to GENERAL, where it then
// stays for its whole life. It runs before anything that could be a texture's first use
// outside a render pass — a pass, a dispatch, a copy, a barrier — so a texture created
// while this command buffer records is ready before it is used.
func (c *cmdBuffer) initializeImages() {
	if c.inRenderPass {
		return
	}
	for _, e := range c.backend.uninitialized {
		if e.IsValid() {
			c.imageBarrier(e.img, aspectOf(e), vkPipelineStage2AllCommandsBit, vkAccess2None, vkPipelineStage2AllCommandsBit, vkAccess2MemoryReadBit|vkAccess2MemoryWriteBit,
				vkImageLayoutUndefined, vkImageLayoutGeneral)
		}
	}
	clear(c.backend.uninitialized)
	c.backend.uninitialized = c.backend.uninitialized[:0]
}

// imageBarrier records a barrier moving img from layout old to new. initializeImages
// uses it to move a new image, or a freshly acquired backbuffer, out of UNDEFINED into
// GENERAL, where it stays; its contents are discarded, and the source scope is every
// stage, so the barrier also chains with an acquire semaphore's wait. Present uses it
// to move a backbuffer into PRESENT_SRC.
func (c *cmdBuffer) imageBarrier(img vkImage, aspect vkImageAspectFlags,
	srcStage vkPipelineStageFlags2, srcAccess vkAccessFlags2,
	dstStage vkPipelineStageFlags2, dstAccess vkAccessFlags2,
	old, new vkImageLayout) {
	s := c.backend.scratch
	s.image = vkImageMemoryBarrier2{
		sType:         vkStructureTypeImageMemoryBarrier2,
		srcStageMask:  srcStage,
		srcAccessMask: srcAccess,
		dstStageMask:  dstStage,
		dstAccessMask: dstAccess,
		oldLayout:     old,
		newLayout:     new,
		image:         img,
		subresourceRange: vkImageSubresourceRange{
			aspectMask: aspect,
			levelCount: vkRemainingMipLevels,
			layerCount: vkRemainingArrayLayers,
		},
	}
	s.dependency = vkDependencyInfo{
		sType:                   vkStructureTypeDependencyInfo,
		imageMemoryBarrierCount: 1,
		pImageMemoryBarriers:    &s.image,
	}
	vkCmdPipelineBarrier2(c.cb, &s.dependency)
}

// maxColorAttachments bounds a single render pass's color targets (today's largest
// user is the G-buffer fill: albedo/normal/metal-rough + emissive-to-color).
const maxColorAttachments = 4

// BeginRenderPass starts rendering into rt. It orders nothing: a pass using an image
// an earlier command wrote needs a Barrier before it.
func (c *cmdBuffer) BeginRenderPass(rt gpu.RenderTargets) {
	if len(rt.Color) > maxColorAttachments {
		panic("vulkan: BeginRenderPass: too many color attachments")
	}
	c.initializeImages()
	// Up to 4 color attachments (0 for a depth-only pass, e.g. a shadow map; up to 4 for
	// MRT, e.g. the G-buffer fill) and an optional depth attachment. A resolve view
	// receives its multisampled attachment's samples when the pass ends: averaged for
	// colour, sample zero's for depth, the one depth resolve every device supports.
	// Every attachment is in VK_IMAGE_LAYOUT_GENERAL, as every image is.
	s := c.backend.scratch
	var w, h uint32
	for i, ca := range rt.Color {
		e := c.backend.tex(ca.Texture)
		w, h = e.width, e.height
		a := &s.colors[i]
		*a = vkRenderingAttachmentInfo{
			sType:       vkStructureTypeRenderingAttachmentInfo,
			imageView:   e.view,
			imageLayout: vkImageLayoutGeneral,
		}
		if e.transient {
			checkTransientAttachment(ca.Load, ca.Store)
		}
		if ca.ResolveTexture != nil && ca.ResolveTexture.IsValid() {
			if ca.Store != gpu.StoreDontCare {
				panic("vulkan: BeginRenderPass: a resolved color attachment cannot also store its samples")
			}
			a.resolveMode = vkResolveModeAverageBit
			a.resolveImageView = c.backend.tex(ca.ResolveTexture).view
			a.resolveImageLayout = vkImageLayoutGeneral
		}
		a.loadOp, a.storeOp = attachmentOps(ca.Load == gpu.LoadClear, e.transient)
		*(*[4]float32)(unsafe.Pointer(&a.clearValue)) = ca.Clear
	}
	s.rendering = vkRenderingInfo{
		sType:                vkStructureTypeRenderingInfo,
		layerCount:           1,
		colorAttachmentCount: cUint32(len(rt.Color)),
	}
	if len(rt.Color) > 0 {
		s.rendering.pColorAttachments = &s.colors[0]
	}
	if rt.Depth != nil {
		e := c.backend.tex(rt.Depth.Texture)
		w, h = e.width, e.height
		d := &s.depth
		*d = vkRenderingAttachmentInfo{
			sType:       vkStructureTypeRenderingAttachmentInfo,
			imageView:   e.view,
			imageLayout: vkImageLayoutGeneral,
		}
		if e.transient {
			checkTransientAttachment(rt.Depth.Load, rt.Depth.Store)
		}
		if rt.Depth.ResolveTexture != nil && rt.Depth.ResolveTexture.IsValid() {
			if rt.Depth.Store != gpu.StoreDontCare {
				panic("vulkan: BeginRenderPass: a resolved depth attachment cannot also store its samples")
			}
			d.resolveMode = vkResolveModeSampleZeroBit
			d.resolveImageView = c.backend.tex(rt.Depth.ResolveTexture).view
			d.resolveImageLayout = vkImageLayoutGeneral
		}
		d.loadOp, d.storeOp = attachmentOps(rt.Depth.Load == gpu.LoadClear, e.transient)
		// clearValue.depthStencil.depth is the union's first float.
		*(*float32)(unsafe.Pointer(&d.clearValue)) = rt.Depth.Clear
		s.rendering.pDepthAttachment = d
		// A read-only depth attachment needs nothing of its own: in GENERAL the pass may
		// depth-test against the image while also sampling it.
	}
	s.rendering.renderArea.extent = vkExtent2D{width: cUint32(w), height: cUint32(h)}
	vkCmdBeginRendering(c.cb, &s.rendering)
	c.inRenderPass = true
}

// attachmentOps returns an attachment's load and store ops. A transient attachment is
// never loaded or stored (gpu.TextureTransient): with lazily allocated memory, either
// would make the device give it some.
func attachmentOps(clear, transient bool) (vkAttachmentLoadOp, vkAttachmentStoreOp) {
	load := vkAttachmentLoadOp(vkAttachmentLoadOpLoad)
	switch {
	case clear:
		load = vkAttachmentLoadOpClear
	case transient:
		load = vkAttachmentLoadOpDontCare
	}
	if transient {
		return load, vkAttachmentStoreOpDontCare
	}
	return load, vkAttachmentStoreOpStore
}

// checkTransientAttachment panics if a pass would load or store a transient attachment
// (see gpu.TextureTransient).
func checkTransientAttachment(load gpu.LoadOp, store gpu.StoreOp) {
	if load == gpu.LoadKeep || store == gpu.StoreKeep {
		panic("vulkan: BeginRenderPass: a transient attachment can only be cleared or not cared about, and never stored")
	}
}

func (c *cmdBuffer) EndRenderPass() {
	vkCmdEndRendering(c.cb)
	c.inRenderPass = false
}

// SetPipeline binds a pipeline to its own bind point (graphics or compute),
// recorded when the pipeline was created.
func (c *cmdBuffer) SetPipeline(p gpu.Pipeline) {
	e := resourceOf[*pipeline](c.backend, p)
	vkCmdBindPipeline(c.cb, e.bindPoint, e.pipe)
	if e.bindPoint == vkPipelineBindPointGraphics {
		// Depth bias is always a dynamic pipeline state (see CreateGraphicsPipeline); Vulkan
		// requires it be set at least once before a draw uses it, so bind a
		// no-bias default here and let SetDepthBias override it.
		vkCmdSetDepthBias(c.cb, 0, 0, 0)
	}
}

// push records the draw/dispatch data as push constants. Vulkan requires the size to
// be a multiple of 4, and the copy happens here so the caller's slice need not outlive
// the call.
func (c *cmdBuffer) push(data []byte) {
	if len(data) == 0 {
		return
	}
	size := len(data) &^ 3 // round down to a multiple of 4; Vulkan rejects the rest
	if size == 0 {
		return
	}
	vkCmdPushConstants(c.cb, c.backend.pipelineLayout, vkShaderStageAll, 0, cUint32(size), unsafe.Pointer(&data[0]))
}

func (c *cmdBuffer) SetViewport(x, y, width, height, minDepth, maxDepth float32) {
	vp := vkViewport{x: cFloat(x), y: cFloat(y), width: cFloat(width), height: cFloat(height),
		minDepth: cFloat(minDepth), maxDepth: cFloat(maxDepth)}
	vkCmdSetViewport(c.cb, 0, 1, &vp)
}

func (c *cmdBuffer) SetScissor(x, y, width, height int32) {
	sc := vkRect2D{offset: vkOffset2D{x: cInt32(x), y: cInt32(y)},
		extent: vkExtent2D{width: cUint32(width), height: cUint32(height)}}
	vkCmdSetScissor(c.cb, 0, 1, &sc)
}

func (c *cmdBuffer) SetDepthBias(bias, slope, clamp float32) {
	vkCmdSetDepthBias(c.cb, cFloat(bias), cFloat(clamp), cFloat(slope))
}

func (c *cmdBuffer) Draw(data []byte, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	c.push(data)
	vkCmdDraw(c.cb, cUint32(vertexCount), cUint32(instanceCount), cUint32(firstVertex), cUint32(firstInstance))
}

// indexTypeOf maps the RHI's index width to Vulkan's enum.
func indexTypeOf(t gpu.IndexType) vkIndexType {
	if t == gpu.IndexUint16 {
		return vkIndexTypeUINT16
	}
	return vkIndexTypeUINT32
}

func (c *cmdBuffer) DrawIndexed(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, indexCount, instanceCount, firstIndex uint32, vertexOffset int32, firstInstance uint32) {
	c.push(data)
	vkCmdBindIndexBuffer(c.cb, c.backend.bufRaw(indexBuf), 0, indexTypeOf(indexType))
	vkCmdDrawIndexed(c.cb, cUint32(indexCount), cUint32(instanceCount), cUint32(firstIndex), cInt32(vertexOffset), cUint32(firstInstance))
}

func (c *cmdBuffer) DrawIndexedIndirect(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, args gpu.Buffer, argsOffset uint64, drawCount, stride uint32) {
	c.push(data)
	vkCmdBindIndexBuffer(c.cb, c.backend.bufRaw(indexBuf), 0, indexTypeOf(indexType))
	vkCmdDrawIndexedIndirect(c.cb, c.backend.bufRaw(args), vkDeviceSize(argsOffset), cUint32(drawCount), cUint32(stride))
}

func (c *cmdBuffer) Dispatch(data []byte, x, y, z uint32) {
	c.initializeImages()
	c.push(data)
	vkCmdDispatch(c.cb, cUint32(x), cUint32(y), cUint32(z))
}

func (c *cmdBuffer) DispatchIndirect(data []byte, args gpu.Buffer, offset uint64) {
	// The workgroup counts are three uint32s, at a multiple of 4 inside the buffer.
	if size := resourceOf[*buffer](c.backend, args).size; offset%4 != 0 || offset+12 > size {
		panic(fmt.Sprintf("vulkan: DispatchIndirect: offset %d does not hold 12 aligned bytes in a %d-byte buffer", offset, size))
	}
	c.initializeImages()
	c.push(data)
	vkCmdDispatchIndirect(c.cb, c.backend.bufRaw(args), vkDeviceSize(offset))
}

// Barrier is a global memory barrier. With every image in GENERAL it orders images as
// well as buffers, so it is the only ordering the gpu API needs.
func (c *cmdBuffer) Barrier(src, dst gpu.Stage, flags gpu.BarrierFlags) {
	c.initializeImages()
	ss, sa := stageAccess(src)
	ds, da := stageAccess(dst)
	s := c.backend.scratch
	s.memory = vkMemoryBarrier2{
		sType:         vkStructureTypeMemoryBarrier2,
		srcStageMask:  ss,
		srcAccessMask: sa,
		dstStageMask:  ds,
		dstAccessMask: da,
	}
	s.dependency = vkDependencyInfo{
		sType:              vkStructureTypeDependencyInfo,
		memoryBarrierCount: 1,
		pMemoryBarriers:    &s.memory,
	}
	vkCmdPipelineBarrier2(c.cb, &s.dependency)
}

func (c *cmdBuffer) CopyBuffer(dst, src gpu.Buffer, dstOffset, srcOffset, size uint64) {
	region := vkBufferCopy{srcOffset: vkDeviceSize(srcOffset), dstOffset: vkDeviceSize(dstOffset), size: vkDeviceSize(size)}
	vkCmdCopyBuffer(c.cb, c.backend.bufRaw(src), c.backend.bufRaw(dst), 1, &region)
}

func (c *cmdBuffer) CopyBufferToTexture(dst gpu.Texture, mip, layer uint32, src gpu.Buffer, srcOffset uint64) {
	c.initializeImages()
	e := c.backend.tex(dst)
	w, h, d := mipExtent(e, mip)
	region := bufferImageCopy(srcOffset, w, h, d, mip, layer, aspectOf(e))
	vkCmdCopyBufferToImage(c.cb, c.backend.bufRaw(src), e.img, vkImageLayoutGeneral, 1, &region)
}

func (c *cmdBuffer) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, mip, layer uint32) {
	c.initializeImages()
	e := c.backend.tex(src)
	w, h, d := mipExtent(e, mip)
	region := bufferImageCopy(0, w, h, d, mip, layer, aspectOf(e))
	vkCmdCopyImageToBuffer(c.cb, e.img, vkImageLayoutGeneral, c.backend.bufRaw(dst), 1, &region)
}

func (c *cmdBuffer) CopyTextureToTexture(dst, src gpu.Texture, mip, layer uint32) {
	c.initializeImages()
	s, d := c.backend.tex(src), c.backend.tex(dst)
	w, h, depth := mipExtent(s, mip)
	sub := subresourceLayers(mip, layer, aspectOf(s))
	region := vkImageCopy{
		srcSubresource: sub,
		dstSubresource: sub,
		extent:         vkExtent3D{width: cUint32(w), height: cUint32(h), depth: cUint32(depth)},
	}
	vkCmdCopyImage(c.cb, s.img, vkImageLayoutGeneral, d.img, vkImageLayoutGeneral, 1, &region)
}

// subresourceLayers names one layer of one mip.
func subresourceLayers(mip, layer uint32, aspect vkImageAspectFlags) vkImageSubresourceLayers {
	return vkImageSubresourceLayers{
		aspectMask:     aspect,
		mipLevel:       cUint32(mip),
		baseArrayLayer: cUint32(layer),
		layerCount:     1,
	}
}

// bufferImageCopy is a copy between a tightly packed buffer at offset and one layer of
// one mip.
func bufferImageCopy(offset uint64, w, h, d, mip, layer uint32, aspect vkImageAspectFlags) vkBufferImageCopy {
	return vkBufferImageCopy{
		bufferOffset:     vkDeviceSize(offset),
		imageSubresource: subresourceLayers(mip, layer, aspect),
		imageExtent:      vkExtent3D{width: cUint32(w), height: cUint32(h), depth: cUint32(d)},
	}
}

// mipExtent returns a texture's extent at level: each level halves every dimension,
// floored at 1 (so a 4x1 image's level 2 is 1x1, not 1x0). Copying the base extent into
// a smaller level would overrun it. A 3D texture's copy covers its whole depth, as a
// 2D texture's covers one layer.
func mipExtent(e *texture, level uint32) (width, height, depth uint32) {
	return max(e.width>>level, 1), max(e.height>>level, 1), max(e.depth>>level, 1)
}

// stageAccess maps an gpu.Stage bitmask to a coarse (stage, access) pair. Each stage
// carries both the reads and the writes it can make, so the same pair serves as a
// barrier's source and its destination: blending reads the color attachment, a depth
// test reads depth, and a vertex or fragment shader can write storage.
func stageAccess(s gpu.Stage) (vkPipelineStageFlags2, vkAccessFlags2) {
	var stage vkPipelineStageFlags2
	var access vkAccessFlags2
	if s == gpu.StageNone {
		return vkPipelineStage2None, vkAccess2None
	}
	if s == gpu.StageAll {
		return vkPipelineStage2AllCommandsBit, vkAccess2MemoryReadBit | vkAccess2MemoryWriteBit
	}
	if s&gpu.StageIndirect != 0 {
		stage |= vkPipelineStage2DrawIndirectBit
		access |= vkAccess2IndirectCommandReadBit
	}
	if s&gpu.StageVertex != 0 {
		stage |= vkPipelineStage2VertexShaderBit
		access |= vkAccess2ShaderReadBit | vkAccess2ShaderWriteBit
	}
	if s&gpu.StageFragment != 0 {
		stage |= vkPipelineStage2FragmentShaderBit
		access |= vkAccess2ShaderReadBit | vkAccess2ShaderWriteBit
	}
	if s&gpu.StageColorOutput != 0 {
		stage |= vkPipelineStage2ColorAttachmentOutputBit
		access |= vkAccess2ColorAttachmentReadBit | vkAccess2ColorAttachmentWriteBit
	}
	if s&gpu.StageDepth != 0 {
		stage |= vkPipelineStage2EarlyFragmentTestsBit | vkPipelineStage2LateFragmentTestsBit
		access |= vkAccess2DepthStencilAttachmentReadBit | vkAccess2DepthStencilAttachmentWriteBit
	}
	if s&gpu.StageCompute != 0 {
		stage |= vkPipelineStage2ComputeShaderBit
		access |= vkAccess2ShaderReadBit | vkAccess2ShaderWriteBit
	}
	if s&gpu.StageTransfer != 0 {
		stage |= vkPipelineStage2AllTransferBit
		access |= vkAccess2TransferReadBit | vkAccess2TransferWriteBit
	}
	return stage, access
}
