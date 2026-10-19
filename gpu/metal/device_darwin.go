//go:build darwin && cgo

package metal

// device is a Metal device that creates resources and command objects. Its methods
// dispatch through the native binding functions in objc_darwin.go.
type device id

func (d device) release() {
	release(d)
}

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

func (d device) newSharedEvent() sharedEvent {
	return deviceNewSharedEvent(d)
}

func (d device) newFence() fence {
	return deviceNewFence(d)
}

func (d device) newCommandBuffer() commandBuffer {
	return deviceNewCommandBuffer(d)
}

func (d device) newCompiler() (compiler, error) {
	return newWithDescriptor(d.newCompilerWithDescriptor, newCompilerDescriptor, nil, "cannot create Metal 4 compiler")
}

func (d device) newCompilerWithDescriptor(descriptor compilerDescriptor, err *id) compiler {
	return deviceNewCompilerWithDescriptor(d, descriptor, err)
}

func (d device) newResidencySet() (residencySet, error) {
	object, err := newWithDescriptor(d.newResidencySetWithDescriptor, newResidencySetDescriptor,
		func(descriptor residencySetDescriptor) {
			descriptor.setLabel("gamekit.residency")
		}, "cannot create residency set")
	return residencySet(object), err
}

func (d device) newResidencySetWithDescriptor(descriptor residencySetDescriptor, err *id) id {
	return deviceNewResidencySetWithDescriptor(d, descriptor, err)
}

func (d device) newAllocator() (allocator, error) {
	return newWithDescriptor(d.newCommandAllocatorWithDescriptor, newCommandAllocatorDescriptor, nil, "cannot create command allocator")
}

func (d device) newCommandAllocatorWithDescriptor(descriptor commandAllocatorDescriptor, err *id) allocator {
	return deviceNewCommandAllocatorWithDescriptor(d, descriptor, err)
}

func (d device) newArgumentTable(maxBufferBindCount uint) (argumentTable, error) {
	return newWithDescriptor(d.newArgumentTableWithDescriptor, newArgumentTableDescriptor,
		func(descriptor argumentTableDescriptor) {
			descriptor.setMaxBufferBindCount(maxBufferBindCount)
		}, "cannot create argument table")
}

func (d device) newArgumentTableWithDescriptor(descriptor argumentTableDescriptor, err *id) argumentTable {
	return deviceNewArgumentTableWithDescriptor(d, descriptor, err)
}

func (d device) newCounterHeapWithDescriptor(descriptor counterHeapDescriptor, err *id) id {
	return deviceNewCounterHeapWithDescriptor(d, descriptor, err)
}

func (d device) newBufferWithLength(length, options uint) buffer {
	return deviceNewBufferWithLength(d, length, options)
}

func (d device) newTextureWithDescriptor(descriptor textureDescriptor) texture {
	return deviceNewTextureWithDescriptor(d, descriptor)
}

func (d device) newSamplerStateWithDescriptor(descriptor samplerDescriptor) sampler {
	return deviceNewSamplerStateWithDescriptor(d, descriptor)
}

func (d device) newDepthStencilStateWithDescriptor(descriptor depthStencilDescriptor) id {
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
