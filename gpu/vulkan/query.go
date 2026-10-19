//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// CreateTimestampPool allocates a pool of count timestamp query slots.
func (b *Backend) CreateTimestampPool(count uint32) gpu.QueryPool {
	if count == 0 {
		return nil
	}

	ci := vkQueryPoolCreateInfo{
		sType:      vkStructureTypeQueryPoolCreateInfo,
		queryType:  vkQueryTypeTimestamp,
		queryCount: cUint32(count),
	}
	var pool vkQueryPool
	if r := vkCreateQueryPool(b.device, &ci, nil, &pool); r != vkSuccess {
		return nil
	}
	object := &queryPool{native: pool}
	object.reference = reference{backend: b}
	return object
}

// DestroyTimestampPool releases a timestamp pool.
func (b *Backend) DestroyTimestampPool(p gpu.QueryPool) {
	resourceOf[*queryPool](b, p).destroy()
}

// ReadTimestamps blocks until the pool's count results are available and returns
// them as raw ticks (convert deltas with TimestampPeriod). Returns nil if the pool
// has no valid results yet.
func (b *Backend) ReadTimestamps(p gpu.QueryPool, count uint32) []uint64 {
	if p == nil || count == 0 {
		return nil
	}

	pool := resourceOf[*queryPool](b, p)
	if !pool.IsValid() {
		return nil
	}
	out := make([]uint64, count)
	r := vkGetQueryPoolResults(b.device, pool.native, 0, cUint32(count), cSize(count*8), unsafe.Pointer(&out[0]), 8,
		vkQueryResult64Bit|vkQueryResultWaitBit)
	if r != vkSuccess {
		return nil
	}
	return out
}

// TimestampPeriod returns nanoseconds per timestamp tick for this device.
func (b *Backend) TimestampPeriod() float64 {
	return float64(b.properties.limits.timestampPeriod) // nanoseconds per tick
}

// TimestampValidBits returns how many bits of the graphics/compute queue's
// timestamps are meaningful (0 = timestamps unsupported on that queue).
func (b *Backend) TimestampValidBits() uint32 {
	if int(b.queueFamily) >= len(b.queueFamilies) {
		return 0
	}
	return uint32(b.queueFamilies[b.queueFamily].timestampValidBits)
}

func (c *cmdBuffer) ResetTimestamps(p gpu.QueryPool, count uint32) {
	if pool := resourceOf[*queryPool](c.backend, p); pool != nil && pool.IsValid() {
		vkCmdResetQueryPool(c.cb, pool.native, 0, cUint32(count))
	}
}

func (c *cmdBuffer) WriteTimestamp(p gpu.QueryPool, index uint32, at gpu.Stage) {
	pool := resourceOf[*queryPool](c.backend, p)
	if pool == nil || !pool.IsValid() {
		return
	}
	stage, _ := stageAccess(at)
	if stage == 0 {
		// StageNone → the earliest point, for a frame-start timestamp.
		stage = vkPipelineStage2TopOfPipeBit
	}
	vkCmdWriteTimestamp2(c.cb, stage, pool.native, cUint32(index))
}
