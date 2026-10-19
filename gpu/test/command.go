package test

import "github.com/bluescreen10/gamekit/gpu"

// commandBuffer implements gpu.CommandBuffer (asserted below). Draws, dispatches and
// pipeline/render-pass state are no-ops — see the package doc for why. Only the copy
// methods and GPU-timestamp bookkeeping do real work.
var _ gpu.CommandBuffer = (*commandBuffer)(nil)

type commandBuffer struct {
	backend *Backend
}

func (c *commandBuffer) BeginRenderPass(gpu.RenderTargets) {}
func (c *commandBuffer) EndRenderPass()                    {}

func (c *commandBuffer) SetPipeline(gpu.Pipeline) {}

func (c *commandBuffer) SetViewport(x, y, width, height, minDepth, maxDepth float32) {}
func (c *commandBuffer) SetScissor(x, y, width, height int32)                        {}
func (c *commandBuffer) SetDepthBias(bias, slope, clamp float32)                     {}

func (c *commandBuffer) Draw(data []byte, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
}

func (c *commandBuffer) DrawIndexed(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, indexCount, instanceCount, firstIndex uint32, vertexOffset int32, firstInstance uint32) {
}

func (c *commandBuffer) DrawIndexedIndirect(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, args gpu.Buffer, argsOffset uint64, drawCount, stride uint32) {
}

func (c *commandBuffer) Dispatch(data []byte, x, y, z uint32) {}

func (c *commandBuffer) DispatchIndirect(data []byte, args gpu.Buffer, offset uint64) {}

func (c *commandBuffer) Barrier(src, dst gpu.Stage, flags gpu.BarrierFlags) {}

func (c *commandBuffer) ResetTimestamps(pool gpu.QueryPool, count uint32) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	p := resourceOf[*queryPool](c.backend, pool)
	if p == nil || !p.IsValid() {
		return
	}
	for i := range p.slots {
		if uint32(i) < count {
			p.slots[i] = 0
		}
	}
}

// WriteTimestamp advances a fake, backend-wide tick counter so consecutive writes
// (e.g. one at the start and one at the end of a frame) always read back increasing —
// enough for any real code's "did time pass between these two writes" check.
func (c *commandBuffer) WriteTimestamp(pool gpu.QueryPool, index uint32, at gpu.Stage) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	c.backend.nextTick++
	if p := resourceOf[*queryPool](c.backend, pool); p != nil && p.IsValid() && index < uint32(len(p.slots)) {
		p.slots[index] = c.backend.nextTick
	}
}

func (c *commandBuffer) CopyBuffer(dst, src gpu.Buffer, dstOffset, srcOffset, size uint64) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	d := resourceOf[*buffer](c.backend, dst)
	if d == nil || !d.IsValid() {
		panic("gpu/test: CopyBuffer: dst is not a live buffer")
	}
	s := resourceOf[*buffer](c.backend, src)
	if s == nil || !s.IsValid() {
		panic("gpu/test: CopyBuffer: src is not a live buffer")
	}
	copy(d.data[dstOffset:dstOffset+size], s.data[srcOffset:srcOffset+size])
}

func (c *commandBuffer) CopyBufferToTexture(dst gpu.Texture, mip, layer uint32, src gpu.Buffer, srcOffset uint64) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	t := resourceOf[*texture](c.backend, dst)
	if t == nil || !t.IsValid() {
		panic("gpu/test: CopyBufferToTexture: dst is not a live texture")
	}
	s := resourceOf[*buffer](c.backend, src)
	if s == nil || !s.IsValid() {
		panic("gpu/test: CopyBufferToTexture: src is not a live buffer")
	}
	start, end := t.mipRange(mip)
	copy(t.data[start:end], s.data[srcOffset:srcOffset+(end-start)])
}

func (c *commandBuffer) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, mip, layer uint32) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	d := resourceOf[*buffer](c.backend, dst)
	if d == nil || !d.IsValid() {
		panic("gpu/test: CopyTextureToBuffer: dst is not a live buffer")
	}
	t := resourceOf[*texture](c.backend, src)
	if t == nil || !t.IsValid() {
		panic("gpu/test: CopyTextureToBuffer: src is not a live texture")
	}
	start, end := t.mipRange(mip)
	copy(d.data[:end-start], t.data[start:end])
}

func (c *commandBuffer) CopyTextureToTexture(dst, src gpu.Texture, mip, layer uint32) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	d := resourceOf[*texture](c.backend, dst)
	if d == nil || !d.IsValid() {
		panic("gpu/test: CopyTextureToTexture: dst is not a live texture")
	}
	s := resourceOf[*texture](c.backend, src)
	if s == nil || !s.IsValid() {
		panic("gpu/test: CopyTextureToTexture: src is not a live texture")
	}
	start, end := s.mipRange(mip)
	dstStart, dstEnd := d.mipRange(mip)
	if end-start != dstEnd-dstStart {
		panic("gpu/test: CopyTextureToTexture: src and dst mips differ in size")
	}
	copy(d.data[dstStart:dstEnd], s.data[start:end])
}

// mipRange is the byte range mip m occupies within t.data. Array layers and cube
// faces are folded into it rather than addressed individually — see texture's doc
// comment — so layer is not part of the range.
func (t *texture) mipRange(mip uint32) (start, end uint64) {
	return t.offset[mip], t.offset[mip+1]
}
