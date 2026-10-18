//go:build darwin && cgo

package metal

// device is a Metal device that creates resources and command objects. Its methods
// dispatch through the native binding functions in objc_darwin.go.
type device id

func (d device) argumentBuffersSupport() uint {
	return deviceArgumentBuffersSupport(d)
}

func (d device) hasUnifiedMemory() bool {
	return deviceHasUnifiedMemory(d)
}

func (d device) supportsFamily(family uint) bool {
	return deviceSupportsFamily(d, family)
}

func (d device) maxBufferLength() uint {
	return deviceMaxBufferLength(d)
}

func (d device) maxArgumentBufferSamplerCount() uint {
	return deviceMaxArgumentBufferSamplerCount(d)
}

func (d device) newMTL4CommandQueue() queue {
	return deviceNewMTL4CommandQueue(d)
}

func (d device) newSharedEvent() id {
	return deviceNewSharedEvent(d)
}

func (d device) newFence() id {
	return deviceNewFence(d)
}

func (d device) newCommandBuffer() id {
	return deviceNewCommandBuffer(d)
}

func (d device) newCompilerWithDescriptor(descriptor id, err *id) id {
	return deviceNewCompilerWithDescriptor(d, descriptor, err)
}

func (d device) newResidencySet() (residencySet, error) {
	object, err := newWithDescriptor(d.newResidencySetWithDescriptor, "MTLResidencySetDescriptor",
		func(descriptor id) {
			setLabel(descriptor, "gamekit.residency")
		}, "cannot create residency set")
	return residencySet(object), err
}

func (d device) newResidencySetWithDescriptor(descriptor id, err *id) id {
	return deviceNewResidencySetWithDescriptor(d, descriptor, err)
}

func (d device) newCommandAllocatorWithDescriptor(descriptor id, err *id) id {
	return deviceNewCommandAllocatorWithDescriptor(d, descriptor, err)
}

func (d device) newArgumentTableWithDescriptor(descriptor id, err *id) id {
	return deviceNewArgumentTableWithDescriptor(d, descriptor, err)
}

func (d device) newCounterHeapWithDescriptor(descriptor id, err *id) id {
	return deviceNewCounterHeapWithDescriptor(d, descriptor, err)
}

func (d device) newBufferWithLength(length, options uint) id {
	return deviceNewBufferWithLength(d, length, options)
}

func (d device) newTextureWithDescriptor(descriptor id) id {
	return deviceNewTextureWithDescriptor(d, descriptor)
}

func (d device) newSamplerStateWithDescriptor(descriptor id) id {
	return deviceNewSamplerStateWithDescriptor(d, descriptor)
}

func (d device) newDepthStencilStateWithDescriptor(descriptor id) id {
	return deviceNewDepthStencilStateWithDescriptor(d, descriptor)
}

func (d device) newLibraryWithData(data id, err *id) id {
	return deviceNewLibraryWithData(d, data, err)
}

func (d device) newLibraryWithSource(source, options id, err *id) id {
	return deviceNewLibraryWithSource(d, source, options, err)
}

func (d device) sampleTimestamps(cpu, gpu *uint64) {
	deviceSampleTimestamps(d, cpu, gpu)
}
