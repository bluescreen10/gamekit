//go:build darwin && cgo

package metal

import (
	"fmt"
	"time"

	"github.com/bluescreen10/gamekit/gpu"
)

// The stages a fence may name for the two encoder types this backend uses: the
// all-stages mask includes bits an encoder type cannot produce or consume, which
// validation rejects.
const (
	renderStages  = mtlStageVertex | mtlStageFragment | mtlStageTile | mtlStageObject | mtlStageMesh
	computeStages = mtlStageDispatch | mtlStageBlit | mtlStageAccelerationStructure
)

// rootSlotSize is how far apart draws' root data sits in the root arena.
const rootSlotSize = 256

// query is a timestamp to write into a pool, deferred to the next real operation.
type query struct {
	pool  *queryPool
	index uint32
	stage gpu.Stage
}

// attachment is one attachment of the open render pass, as ending it needs.
type attachment struct {
	texture *texture
	store   mtlStoreAction
}

// command records into the backend's one command buffer (see Backend.allocator).
//
// MTL4 has no automatic hazard tracking between encoders at all — not even between
// two compute encoders opened and closed back to back in the same command buffer. One
// fence, updated when any encoder closes and waited on when any encoder opens, orders
// every encoder in a command buffer against the one before it. Coarse (every encoder
// waits on everything before it, not just what it actually depends on), but correct,
// and nothing is pipelined within a frame, so the cost is small.
type command struct {
	backend   *Backend
	submitted bool

	// render and compute are the open encoders, at most one of them. compute also
	// records copies: MTL4 has no separate blit encoder. computeTable says the
	// argument table is bound to it.
	render        mtl4RenderCommandEncoder
	compute       mtl4ComputeCommandEncoder
	computeTable  bool
	pass          mtl4RenderPassDescriptor
	colors        []attachment
	depth         *attachment
	readOnlyDepth bool

	pipeline *pipeline
	// graphics and kernel are the pipelines bound to the open encoders, so a draw or
	// dispatch sets only what changed.
	graphics, kernel *pipeline

	viewport    [6]float64 // x, y, width, height, znear, zfar
	scissor     [4]uint    // x, y, width, height
	hasViewport bool
	hasScissor  bool

	// pending holds queries waiting for the next operation; starts and ends those
	// resolved from the command buffer's GPU start and end when it completes.
	pending, starts, ends []query

	// rootCursor is the next free offset in the root arena.
	rootCursor uint64
}

func (b *Backend) Begin() gpu.CommandBuffer {
	// The device's allocator, buffer and argument table are reused rather than created
	// fresh (see Backend.allocator), which makes concurrent recording impossible: a
	// second Begin before the first Submits or Presents would reset an allocator still
	// backing an open recording. If that changes, the fix is a small pool of
	// allocator/buffer/table triples, one per recording in flight.
	check(b.open == nil, "a command buffer from this device is already open")

	b.allocator.reset()
	b.buffer.beginCommandBufferWithAllocator(b.allocator)

	b.open = &command{backend: b}

	return b.open
}

func (b *Backend) cmd(c gpu.CommandBuffer) *command {
	v, ok := c.(*command)
	if !ok || v.backend != b || v.submitted {
		panic("metal: invalid or submitted command buffer")
	}

	return v
}

func (c *command) ready() {
	if c.submitted {
		panic("metal: command buffer already submitted")
	}
}

func (c *command) endCompute() {
	if c.compute == 0 {
		return
	}

	c.compute.updateFence(c.backend.fence, computeStages)
	c.compute.endEncoding()

	c.compute.release()

	c.compute, c.computeTable, c.kernel = 0, false, nil
}

// outside ends any compute encoder, for an operation that needs none open.
func (c *command) outside() {
	check(c.render == 0, "operation must be outside a render pass")

	c.endCompute()
}

// computeEncoder opens a compute encoder, unless one is open.
func (c *command) computeEncoder() mtl4ComputeCommandEncoder {
	if c.compute == 0 {
		pool := pushPool()

		c.compute = c.backend.buffer.computeCommandEncoder()
		retain(id(c.compute))
		pool.drain()

		check(c.compute != 0, "cannot create compute encoder")

		c.compute.waitForFence(c.backend.fence, computeStages)
	}

	return c.compute
}

// rootSlot copies data into the root arena and returns its GPU address. Each call
// claims a fresh 256-byte slot, so draws recorded earlier in the same command buffer
// keep their own root even though nothing has been submitted yet.
func (c *command) rootSlot(data []byte) uint64 {
	b := c.backend
	off := c.rootCursor

	if need := off + uint64(len(data)); b.rootArena == 0 || need > b.rootArenaLen {
		// Grow by doubling. Draws already recorded still read the old arena, so it stays
		// resident and alive until the GPU is done with it.
		size := max(2*b.rootArenaLen, need, 16384)
		arena := b.device.newBuffer(uint(size), mtlResourceStorageModeShared)
		check(arena != 0, "cannot allocate root scratch buffer")

		if b.rootArena != 0 {
			b.retired = append(b.retired, b.rootArena)
		}

		b.rootArena, b.rootArenaLen = arena, size
		b.rootArenaPtr = arena.contents()
		b.rootArenaAddr = arena.gpuAddress()
		b.residencySet.enrol(id(arena))

		// The new arena starts empty: draws already recorded keep the old one's slots.
		off = 0
	}

	copy(unsafeBytes(b.rootArenaPtr, off, len(data)), data)
	c.rootCursor = off + rootSlotSize

	return b.rootArenaAddr + off
}

// bindings writes the draw or dispatch data into the root arena and binds its address
// at argument-table slot 0. The data belongs to the call rather than to the encoder, so
// a draw that supplies none binds nothing rather than silently inheriting the previous
// draw's.
func (c *command) bindings(data []byte) {
	if len(data) == 0 {
		return
	}

	check(len(data) <= 4096, "draw/dispatch data exceeds the root arena's per-slot budget")

	// Catch a root argument the shader would read past. MSL rounds a struct's size up to
	// its alignment, so a caller that packs the same fields to 8-byte alignment comes up
	// short — silently, until something perturbs what follows the buffer.
	if uint64(len(data)) < c.pipeline.rootSize {
		panic(fmt.Sprintf("metal: draw/dispatch data is %d bytes but the shader's root argument is %d; pad it to a multiple of 16",
			len(data), c.pipeline.rootSize))
	}

	c.backend.argTable.setAddress(c.rootSlot(data), 0)
}

func (c *command) draw(data []byte) {
	p := c.pipeline
	check(c.render != 0 && p != nil && p.render != 0, "draw requires a graphics pipeline and render pass")
	check(!c.readOnlyDepth || !p.depthWrite, "depth-writing pipeline in read-only depth pass")

	if c.graphics != p {
		c.render.setRenderPipelineState(p.render)
		c.render.setDepthStencilState(p.depthStencil)
		c.render.setCullMode(p.cull)
		c.render.setFrontFacingWinding(p.winding)
		c.graphics = p
	}

	c.bindings(data)
}

func (c *command) dispatch(data []byte) {
	p := c.pipeline
	check(c.render == 0 && p != nil && p.compute != 0, "dispatch requires a compute pipeline outside a render pass")

	e := c.computeEncoder()
	if !c.computeTable {
		e.setArgumentTable(c.backend.argTable)
		c.computeTable = true
	}

	if c.kernel != p {
		e.setComputePipelineState(p.compute)
		c.kernel = p
	}

	c.bindings(data)
}

// Metal 4 writes a timestamp directly into whatever encoder is open, at the point
// requested — no render-pass split needed. Not every RHI stage maps precisely yet.
// Fragment is the legal "all rendering work has completed" stage and matches the
// precision of sampling at an encoder's end.
//
// The granularity must be Precise. Relaxed lets Metal take the timestamp at the
// encoder's boundary instead, which collapses every timestamp inside one compute
// encoder onto it: timing consecutive dispatches gave the first all their time and the
// rest none, or an end before its start (TestTimestampsAroundEachDispatchMeasureIt).
func (c *command) sampleQuery(q query) {
	switch {
	case c.render != 0:
		c.render.writeTimestampWithGranularity(mtl4TimestampGranularityPrecise, mtlRenderStageFragment, q.pool.mtl4CounterHeap, uint(q.index))
	case c.compute != 0:
		c.compute.writeTimestampWithGranularity(mtl4TimestampGranularityPrecise, q.pool.mtl4CounterHeap, uint(q.index))
	default:
		// No encoder open: a throwaway compute encoder places the timestamp at this exact
		// point in the command buffer's GPU timeline.
		e := c.computeEncoder()
		e.writeTimestampWithGranularity(mtl4TimestampGranularityPrecise, q.pool.mtl4CounterHeap, uint(q.index))
		c.endCompute()
	}
}

// prepareWork places the pending queries just before the operation about to be
// recorded, when it is known which encoder is open. A leading StageNone query is
// resolved from the command buffer's GPU start when it completes instead, and queries
// with no following work from its GPU end (see submit).
func (c *command) prepareWork() {
	for _, q := range c.pending {
		if q.stage == gpu.StageNone {
			c.starts = append(c.starts, q)
		} else {
			c.sampleQuery(q)
		}
	}

	c.pending = c.pending[:0]
}

func loadAction(op gpu.LoadOp) mtlLoadAction {
	switch op {
	case gpu.LoadClear:
		return mtlLoadActionClear
	case gpu.LoadKeep:
		return mtlLoadActionLoad
	default:
		return mtlLoadActionDontCare
	}
}

// storeAction is what ending a pass does with an attachment: keep it or not, or average
// its samples into resolve. Resolving never keeps the samples too (see
// gpu.ColorAttachment.ResolveTexture). It is applied only when the pass ends (see
// EndRenderPass); an encoder split off mid-pass stores plainly instead.
func storeAction(store gpu.StoreOp, resolve bool) mtlStoreAction {
	if resolve {
		check(store == gpu.StoreDontCare, "a resolved attachment cannot also store its samples")
		return mtlStoreActionMultisampleResolve
	}

	if store == gpu.StoreKeep {
		return mtlStoreActionStore
	}

	return mtlStoreActionDontCare
}

func (c *command) BeginRenderPass(r gpu.RenderTargets) {
	c.ready()
	check(len(r.Color) <= 8, "at most 8 color targets")
	check(c.render == 0, "nested render pass")

	b := c.backend
	for _, x := range r.Color {
		b.checkTransientAttachment(x.Texture, x.Load, x.Store)
	}

	if r.Depth != nil {
		b.checkTransientAttachment(r.Depth.Texture, r.Depth.Load, r.Depth.Store)
	}

	c.endCompute()

	pool := pushPool()
	defer pool.drain()

	c.pass = newMTL4RenderPassDescriptor()

	colors := c.pass.colorAttachments()
	c.colors = c.colors[:0]
	for i, x := range r.Color {
		t := resourceOf[*texture](b, x.Texture)
		a := mtlRenderPassColorAttachmentDescriptor(arrayObjectAtIndexedSubscript(colors, uint(i)))
		a.setTexture(t.mtlTexture)
		a.setLoadAction(loadAction(x.Load))

		resolve := x.ResolveTexture != nil && x.ResolveTexture.IsValid()
		if resolve {
			a.setResolveTexture(resourceOf[*texture](b, x.ResolveTexture).mtlTexture)
		}

		store := storeAction(x.Store, resolve)
		a.setStoreAction(store)
		a.setClearColor(float64(x.Clear[0]), float64(x.Clear[1]), float64(x.Clear[2]), float64(x.Clear[3]))

		c.colors = append(c.colors, attachment{texture: t, store: store})
	}

	c.depth = nil
	if x := r.Depth; x != nil {
		t := resourceOf[*texture](b, x.Texture)
		a := c.pass.depthAttachment()
		a.setTexture(t.mtlTexture)
		a.setLoadAction(loadAction(x.Load))
		resolve := x.ResolveTexture != nil && x.ResolveTexture.IsValid()
		if resolve {
			a.setResolveTexture(resourceOf[*texture](b, x.ResolveTexture).mtlTexture)
			a.setDepthResolveFilter(mtlMultisampleDepthResolveFilterSample0)
		}
		store := storeAction(x.Store, resolve)
		a.setStoreAction(store)
		a.setClearDepth(float64(x.Clear))

		c.depth = &attachment{texture: t, store: store}
		c.readOnlyDepth = x.ReadOnly
	}

	c.prepareWork()
	c.resumeRender(false)
}

// checkTransientAttachment panics if t is transient and a pass would load or store it
// (see gpu.TextureTransient).
func (b *Backend) checkTransientAttachment(t gpu.Texture, load gpu.LoadOp, store gpu.StoreOp) {
	if !resourceOf[*texture](b, t).transient {
		return
	}

	if load == gpu.LoadKeep || store == gpu.StoreKeep {
		panic("metal: BeginRenderPass: a transient attachment can only be cleared or not cared about, and never stored")
	}
}

// pauseRender and resumeRender split the render encoder, which is how a barrier or a
// timestamp lands inside a pass on a tile-based GPU. The paused encoder stores its
// attachments and the resumed one loads them, and the dynamic state carries over.
func (c *command) pauseRender() {
	// A memoryless attachment cannot be stored for the resumed encoder to load.
	for _, a := range c.colors {
		check(!a.texture.memoryless, "a barrier cannot split a render pass with a transient attachment")
	}
	check(c.depth == nil || !c.depth.texture.memoryless, "a barrier cannot split a render pass with a transient attachment")

	for i := range c.colors {
		c.render.setColorStoreAction(mtlStoreActionStore, uint(i))
	}
	if c.depth != nil {
		c.render.setDepthStoreAction(mtlStoreActionStore)
	}

	c.render.updateFence(c.backend.fence, renderStages)
	c.render.endEncoding()

	c.render.release()

	c.render, c.graphics = 0, nil
}

func (c *command) resumeRender(load bool) {
	pool := pushPool()
	defer pool.drain()

	d := c.pass.clone()
	defer d.release()

	colors := d.colorAttachments()
	for i := range c.colors {
		a := mtlRenderPassColorAttachmentDescriptor(arrayObjectAtIndexedSubscript(colors, uint(i)))
		if load {
			a.setLoadAction(mtlLoadActionLoad)
		}

		a.setStoreAction(mtlStoreActionUnknown)
	}

	if c.depth != nil {
		a := d.depthAttachment()
		if load {
			a.setLoadAction(mtlLoadActionLoad)
		}
		a.setStoreAction(mtlStoreActionUnknown)
	}

	c.render = c.backend.buffer.renderCommandEncoder(d)
	retain(id(c.render))
	check(c.render != 0, "cannot resume render pass")

	c.render.waitForFence(c.backend.fence, renderStages)
	c.render.setArgumentTable(c.backend.argTable, mtlRenderStageVertex|mtlRenderStageFragment)

	if c.hasViewport {
		c.render.setViewport(c.viewport[0], c.viewport[1], c.viewport[2], c.viewport[3], c.viewport[4], c.viewport[5])
	}

	if c.hasScissor {
		c.render.setScissorRect(c.scissor[0], c.scissor[1], c.scissor[2], c.scissor[3])
	}
}

func (c *command) EndRenderPass() {
	c.ready()
	check(c.render != 0, "no active render pass")

	for i, a := range c.colors {
		c.render.setColorStoreAction(a.store, uint(i))
	}
	if c.depth != nil {
		c.render.setDepthStoreAction(c.depth.store)
	}

	c.render.updateFence(c.backend.fence, renderStages)
	c.render.endEncoding()

	c.render.release()
	c.pass.release()

	c.render, c.pass, c.graphics = 0, 0, nil
	c.colors, c.depth = c.colors[:0], nil
	c.hasViewport, c.hasScissor, c.readOnlyDepth = false, false, false
}

func (c *command) SetPipeline(p gpu.Pipeline) {
	c.ready()

	c.pipeline = resourceOf[*pipeline](c.backend, p)
}

func (c *command) SetViewport(x, y, w, h, min, max float32) {
	c.ready()
	check(c.render != 0, "viewport outside render pass")

	c.viewport = [6]float64{float64(x), float64(y), float64(w), float64(h), float64(min), float64(max)}
	c.hasViewport = true

	c.render.setViewport(c.viewport[0], c.viewport[1], c.viewport[2], c.viewport[3], c.viewport[4], c.viewport[5])
}

func (c *command) SetScissor(x, y, w, h int32) {
	c.ready()
	if x < 0 || y < 0 || w < 0 || h < 0 {
		panic("metal: negative scissor")
	}

	check(c.render != 0, "scissor outside render pass")

	c.scissor = [4]uint{uint(x), uint(y), uint(w), uint(h)}
	c.hasScissor = true

	c.render.setScissorRect(c.scissor[0], c.scissor[1], c.scissor[2], c.scissor[3])
}

func (c *command) SetDepthBias(bias, slope, clamp float32) {
	c.ready()
	check(c.render != 0, "depth bias outside render pass")

	c.render.setDepthBias(float32(bias), float32(slope), float32(clamp))
}

func (c *command) Draw(data []byte, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	c.ready()
	c.prepareWork()
	c.draw(data)

	c.render.drawPrimitives(c.pipeline.topology, uint(firstVertex), uint(vertexCount), uint(instanceCount), uint(firstInstance))
}

func indexType(t gpu.IndexType) mtlIndexType {
	if t.Size() == 2 {
		return mtlIndexTypeUInt16
	}

	return mtlIndexTypeUInt32
}

func (c *command) DrawIndexed(data []byte, buf gpu.Buffer, it gpu.IndexType, indexCount, instanceCount, firstIndex uint32, vertexOffset int32, firstInstance uint32) {
	c.ready()
	c.prepareWork()
	c.draw(data)

	// Metal takes a byte offset here, unlike Vulkan which takes an element index and
	// derives the stride from the bound index type. MTL4 addresses the index buffer
	// directly rather than binding an object, so the offset folds into the address.
	ib := resourceOf[*buffer](c.backend, buf)
	off := uint64(firstIndex) * uint64(it.Size())
	check(off <= ib.size, "index offset out of bounds")

	c.render.drawIndexedPrimitivesIndexCountIndexTypeIndexBufferIndexBufferLengthInstanceCountBaseVertexBaseInstance(c.pipeline.topology, uint(indexCount), indexType(it), ib.addr+off, uint(ib.size-off), uint(instanceCount), int(vertexOffset), uint(firstInstance))
}

func (c *command) DrawIndexedIndirect(data []byte, buf gpu.Buffer, it gpu.IndexType, args gpu.Buffer, offset uint64, drawCount, stride uint32) {
	c.ready()
	c.prepareWork()
	c.draw(data)

	a := resourceOf[*buffer](c.backend, args)
	check(stride >= 20 && stride%4 == 0 && offset%4 == 0, "invalid indirect stride/offset")
	check(drawCount == 0 || offset+uint64(drawCount-1)*uint64(stride)+20 <= a.size, "indirect draw out of bounds")

	ib := resourceOf[*buffer](c.backend, buf)

	// Metal has no multi-draw-indirect, so one draw per command is the only option.
	for i := range uint64(drawCount) {
		c.render.drawIndexedPrimitivesIndexTypeIndexBufferIndexBufferLengthIndirectBuffer(c.pipeline.topology, indexType(it), ib.addr, uint(ib.size), a.addr+offset+i*uint64(stride))
	}
}

func (c *command) Dispatch(data []byte, x, y, z uint32) {
	c.ready()
	c.prepareWork()
	c.dispatch(data)

	c.compute.dispatchThreadgroups([3]uint{uint(x), uint(y), uint(z)}, c.pipeline.group)
}

func (c *command) DispatchIndirect(data []byte, buf gpu.Buffer, offset uint64) {
	c.ready()

	// The workgroup counts are three uint32s, at a multiple of 4 inside the buffer.
	r := resourceOf[*buffer](c.backend, buf)
	check(offset%4 == 0 && offset+12 <= r.size, "indirect dispatch out of bounds")

	c.prepareWork()
	c.dispatch(data)
	c.compute.dispatchThreadgroupsWithIndirectBuffer(r.addr+offset, c.pipeline.group)
}

// Barrier is a full barrier. A render encoder cannot express an all-stage,
// both-direction intra-pass barrier: Metal only permits pre-raster producers for that
// form. So the encoder is split, and fragment and tile writes are stored, fenced, and
// loaded by the resumed encoder.
func (c *command) Barrier(src, dst gpu.Stage, f gpu.BarrierFlags) {
	c.ready()

	if c.render != 0 {
		c.pauseRender()
		c.resumeRender(true)
	} else {
		c.endCompute()
	}
}

func (c *command) CopyBuffer(dst, src gpu.Buffer, dstOffset, srcOffset, size uint64) {
	c.ready()
	c.outside()
	c.prepareWork()

	d, s := resourceOf[*buffer](c.backend, dst), resourceOf[*buffer](c.backend, src)
	check(dstOffset <= d.size && size <= d.size-dstOffset && srcOffset <= s.size && size <= s.size-srcOffset,
		"buffer copy out of bounds")

	c.computeEncoder().copyFromBufferSourceOffsetToBufferDestinationOffsetSize(s.mtlBuffer, uint(srcOffset), d.mtlBuffer, uint(dstOffset), uint(size))
}

func (c *command) copyTexture(image gpu.Texture, mip, layer uint32, transferBuffer gpu.Buffer, offset uint64, upload bool) {
	c.ready()
	c.outside()
	c.prepareWork()

	t := resourceOf[*texture](c.backend, image)
	buf := resourceOf[*buffer](c.backend, transferBuffer)
	check(uint64(mip) < t.mips && t.samples == 1, "invalid copy mip or multisampled texture")
	check(uint64(layer) < t.layers, "copy layer out of bounds")

	var bytes uint64
	switch t.format {
	case mtlPixelFormatR8Unorm:
		bytes = 1
	case mtlPixelFormatRG8Unorm:
		bytes = 2
	case mtlPixelFormatRGBA16Float:
		bytes = 8
	case mtlPixelFormatRGBA32Float:
		bytes = 16
	default:
		bytes = 4
	}

	w, h, d := max(t.width>>mip, 1), max(t.height>>mip, 1), max(t.depth>>mip, 1)
	row := w * bytes
	img := row * h
	check(offset <= buf.size && img*d <= buf.size-offset, "texture copy buffer too small")

	// One row per blit permits tightly packed RHI buffers without Metal row-pitch padding.
	e := c.computeEncoder()
	line := [3]uint{uint(w), 1, 1}
	for z := range d {
		for y := range h {
			at := uint(offset + z*img + y*row)
			origin := [3]uint{0, uint(y), uint(z)}

			if upload {
				e.copyFromBufferSourceOffsetSourceBytesPerRowSourceBytesPerImageSourceSizeToTextureDestinationSliceDestinationLevelDestinationOrigin(buf.mtlBuffer, at, uint(row), uint(row), line, t.mtlTexture, uint(layer), uint(mip), origin)
			} else {
				e.copyFromTextureSourceSliceSourceLevelSourceOriginSourceSizeToBufferDestinationOffsetDestinationBytesPerRowDestinationBytesPerImage(t.mtlTexture, uint(layer), uint(mip), origin, line, buf.mtlBuffer, at, uint(row), uint(row))
			}
		}
	}
}

func (c *command) CopyBufferToTexture(dst gpu.Texture, mip, layer uint32, src gpu.Buffer, offset uint64) {
	c.copyTexture(dst, mip, layer, src, offset, true)
}

func (c *command) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, mip, layer uint32) {
	c.copyTexture(src, mip, layer, dst, 0, false)
}

func (c *command) CopyTextureToTexture(dst, src gpu.Texture, mip, layer uint32) {
	c.ready()
	c.outside()
	c.prepareWork()

	d, s := resourceOf[*texture](c.backend, dst), resourceOf[*texture](c.backend, src)
	check(s.format == d.format && s.width == d.width && s.height == d.height && s.depth == d.depth,
		"texture copy needs textures of one size and format")
	check(uint64(mip) < s.mips && uint64(mip) < d.mips && s.samples == 1 && d.samples == 1,
		"invalid copy mip or multisampled texture")
	check(uint64(layer) < s.layers, "copy layer out of bounds")

	size := [3]uint{uint(max(s.width>>mip, 1)), uint(max(s.height>>mip, 1)), uint(max(s.depth>>mip, 1))}
	var origin [3]uint

	c.computeEncoder().copyFromTextureSourceSliceSourceLevelSourceOriginSourceSizeToTextureDestinationSliceDestinationLevelDestinationOrigin(s.mtlTexture, uint(layer), uint(mip), origin, size, d.mtlTexture, uint(layer), uint(mip), origin)
}

func (c *command) ResetTimestamps(p gpu.QueryPool, n uint32) {
	c.ready()

	if p == nil || !p.IsValid() {
		return
	}

	c.outside()

	r := resourceOf[*queryPool](c.backend, p)
	check(uint64(n) <= r.count, "query reset out of bounds")

	r.mtl4CounterHeap.invalidateCounterRange(0, uint(n))

	for i := range n {
		delete(r.written, i)
		delete(r.overrides, i)
	}
}

func (c *command) WriteTimestamp(p gpu.QueryPool, index uint32, stage gpu.Stage) {
	c.ready()

	if p == nil || !p.IsValid() {
		return
	}

	r := resourceOf[*queryPool](c.backend, p)
	check(uint64(index) < r.count, "query index out of bounds")

	c.pending = append(c.pending, query{pool: r, index: index, stage: stage})
	r.written[index] = true
}

// submission is what a commit's feedback handler needs: the queries it resolves.
type submission struct {
	value        uint64
	starts, ends []query
}

// feedback is a completed submission's outcome, as its feedback handler reported it.
type feedback struct {
	submission *submission
	err        string
	start, end uint64
}

// submit commits the recording, signalling the device's event with a new value, and
// presents drawable if there is one. It returns the value.
//
// MTL4 command buffers carry no waitUntilCompleted, status or error of their own:
// completion is reported through the shared event the queue signals, and errors and GPU
// times through a commit feedback handler.
func (b *Backend) submit(c *command, drawable mtlDrawable) uint64 {
	c.outside()

	c.ends = append(c.ends, c.pending...)
	c.pending = nil

	b.buffer.endCommandBuffer()

	b.eventValue++
	s := &submission{value: b.eventValue, starts: c.starts, ends: c.ends}

	pool := pushPool()
	defer pool.drain()

	options := newObject("MTL4CommitOptions")
	commitOptionsAddFeedbackHandler(options, func(fb id) {
		b.feedback <- feedbackOf(s, fb)
	})

	buffers := [1]mtl4CommandBuffer{b.buffer}
	b.queue.commit(&buffers[0], 1, options)
	release(options)

	// signalDrawable MUST follow the commit it is meant to cover. The queue is an ordered
	// timeline, so signalling first marks the drawable ready ahead of the work that draws
	// into it, and the present below then shows whatever the frame happened to contain at
	// that moment — torn or half-drawn frames under load.
	if drawable != 0 {
		b.queue.signalDrawable(drawable)
	}
	b.queue.signalEvent(b.event, s.value)

	if drawable != 0 {
		drawable.present()
	}

	b.inFlight++
	c.submitted = true
	b.open = nil

	return s.value
}

// feedbackOf reads a commit's feedback, on the thread Metal reports it from.
func feedbackOf(s *submission, fb id) feedback {
	f := feedback{submission: s}
	if err := commitFeedbackError(fb); err != 0 {
		f.err = errorText(err, "GPU command failed")
	}

	// GPUStartTime and GPUEndTime are host seconds (CACurrentMediaTime's clock).
	// Translate them to mach absolute nanoseconds, so that boundary and counter samples
	// from one query pool compare directly.
	if len(s.starts)+len(s.ends) > 0 {
		hostNow := float64(machAbsoluteTime()) * hostScale()
		origin := hostNow - caCurrentMediaTime()*1e9

		if t := commitFeedbackGPUStartTime(fb); t > 0 {
			f.start = uint64(origin + t*1e9)
		}

		if t := commitFeedbackGPUEndTime(fb); t > 0 {
			f.end = uint64(origin + t*1e9)
		}
	}

	return f
}

// completed waits for the submission that signals value, and for its feedback: the
// event and the feedback handler are independent notifications, and Metal guarantees
// both follow GPU completion, not that one precedes the other.
func (b *Backend) completed(value uint64) {
	check(b.event.waitUntilSignaledValue(value, 30000), "GPU command timed out")

	f, ok := b.early[value]
	delete(b.early, value)

	timeout := time.After(30 * time.Second)
	for !ok {
		select {
		case f = <-b.feedback:
			if f.submission.value != value {
				if b.early == nil {
					b.early = map[uint64]feedback{}
				}

				b.early[f.submission.value] = f
				continue
			}

			ok = true
		case <-timeout:
			panic("metal: GPU completion feedback timed out")
		}
	}

	for _, q := range f.submission.starts {
		q.pool.overrides[q.index] = f.start
	}

	for _, q := range f.submission.ends {
		q.pool.overrides[q.index] = f.end
	}

	if b.inFlight--; b.inFlight == 0 {
		for _, o := range b.released {
			release(o)
		}

		for _, o := range b.retired {
			b.residencySet.unenrol(id(o))
			o.release()
		}

		b.released, b.retired = b.released[:0], b.retired[:0]
	}

	check(f.err == "", f.err)
}

func (b *Backend) Submit(c gpu.CommandBuffer) gpu.Fence {
	value := b.submit(b.cmd(c), 0)

	f := &fence{reference: reference{backend: b}, value: value, pendingIndex: len(b.fences)}
	b.fences = append(b.fences, f)
	return f
}

func (b *Backend) Wait(f gpu.Fence) {
	resourceOf[*fence](b, f).destroy()
}

func (b *Backend) WaitIdle() {
	for len(b.fences) > 0 {
		b.fences[len(b.fences)-1].destroy()
	}
}
