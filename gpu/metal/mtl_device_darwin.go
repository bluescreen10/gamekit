//go:build darwin && cgo

package metal

// mtlDevice is a Metal device that creates resources and command objects. Its methods
// dispatch through the native binding functions in objc_darwin.go.
type mtlDevice id

func (d mtlDevice) release() {
	release(d)
}

func (d mtlDevice) argumentBuffersSupport() mtlArgumentBuffersTier {
	return deviceArgumentBuffersSupport(d)
}

func (d mtlDevice) hasUnifiedMemory() bool {
	return deviceHasUnifiedMemory(d)
}

func (d mtlDevice) supportsFamily(family mtlGPUFamily) bool {
	return deviceSupportsFamily(d, family)
}

func (d mtlDevice) maxBufferLength() uint {
	return deviceMaxBufferLength(d)
}

func (d mtlDevice) maxArgumentBufferSamplerCount() uint {
	return deviceMaxArgumentBufferSamplerCount(d)
}

func (d mtlDevice) newMTL4CommandQueue() mtl4CommandQueue {
	return deviceNewMTL4CommandQueue(d)
}

func (d mtlDevice) newSharedEvent() mtlSharedEvent {
	return deviceNewSharedEvent(d)
}

func (d mtlDevice) newFence() mtlFence {
	return deviceNewFence(d)
}

func (d mtlDevice) newCommandBuffer() mtl4CommandBuffer {
	return deviceNewCommandBuffer(d)
}

// newCompiler creates a compiler with a default descriptor and reports Go errors.
func (d mtlDevice) newCompiler() (mtl4Compiler, error) {
	return newWithDescriptor(d.newCompilerWithDescriptor, newMTL4CompilerDescriptor, nil, "cannot create Metal 4 compiler")
}

func (d mtlDevice) newCompilerWithDescriptor(descriptor mtl4CompilerDescriptor, err *id) mtl4Compiler {
	return deviceNewCompilerWithDescriptor(d, descriptor, err)
}

// newResidencySet creates and labels the backend's residency set.
func (d mtlDevice) newResidencySet() (mtlResidencySet, error) {
	object, err := newWithDescriptor(d.newResidencySetWithDescriptor, newMTLResidencySetDescriptor,
		func(descriptor mtlResidencySetDescriptor) {
			descriptor.setLabel("gamekit.residency")
		}, "cannot create residency set")

	return mtlResidencySet(object), err
}

func (d mtlDevice) newResidencySetWithDescriptor(descriptor mtlResidencySetDescriptor, err *id) id {
	return deviceNewResidencySetWithDescriptor(d, descriptor, err)
}

// newCommandAllocator creates an allocator with a default descriptor and reports Go errors.
func (d mtlDevice) newCommandAllocator() (mtl4CommandAllocator, error) {
	return newWithDescriptor(d.newCommandAllocatorWithDescriptor, newMTL4CommandAllocatorDescriptor, nil, "cannot create command allocator")
}

func (d mtlDevice) newCommandAllocatorWithDescriptor(descriptor mtl4CommandAllocatorDescriptor, err *id) mtl4CommandAllocator {
	return deviceNewCommandAllocatorWithDescriptor(d, descriptor, err)
}

// newArgumentTable configures the buffer count and reports Go errors.
func (d mtlDevice) newArgumentTable(maxBufferBindCount uint) (mtl4ArgumentTable, error) {
	return newWithDescriptor(d.newArgumentTableWithDescriptor, newMTL4ArgumentTableDescriptor,
		func(descriptor mtl4ArgumentTableDescriptor) {
			descriptor.setMaxBufferBindCount(maxBufferBindCount)
		}, "cannot create argument table")
}

func (d mtlDevice) newArgumentTableWithDescriptor(descriptor mtl4ArgumentTableDescriptor, err *id) mtl4ArgumentTable {
	return deviceNewArgumentTableWithDescriptor(d, descriptor, err)
}

func (d mtlDevice) newCounterHeap(descriptor mtl4CounterHeapDescriptor, err *id) mtl4CounterHeap {
	return deviceNewCounterHeapWithDescriptor(d, descriptor, err)
}

func (d mtlDevice) newBuffer(length uint, options mtlResourceOptions) mtlBuffer {
	return deviceNewBufferWithLength(d, length, options)
}

func (d mtlDevice) newTexture(descriptor mtlTextureDescriptor) mtlTexture {
	return deviceNewTextureWithDescriptor(d, descriptor)
}

func (d mtlDevice) newSamplerState(descriptor mtlSamplerDescriptor) mtlSamplerState {
	return deviceNewSamplerStateWithDescriptor(d, descriptor)
}

func (d mtlDevice) newDepthStencilState(descriptor mtlDepthStencilDescriptor) mtlDepthStencilState {
	return deviceNewDepthStencilStateWithDescriptor(d, descriptor)
}

func (d mtlDevice) newLibraryWithData(data id, err *id) mtlLibrary {
	return deviceNewLibraryWithData(d, data, err)
}

func (d mtlDevice) newLibraryWithSource(source, options id, err *id) mtlLibrary {
	return deviceNewLibraryWithSource(d, source, options, err)
}

func (d mtlDevice) sampleTimestamps(cpu, gpu *uint64) {
	deviceSampleTimestamps(d, cpu, gpu)
}
