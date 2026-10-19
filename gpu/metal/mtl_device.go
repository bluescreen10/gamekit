//go:build darwin && cgo

package metal

import "errors"

// mtlDevice is a Metal device that creates resources and command objects. Its methods
// dispatch through the native binding functions in objc.go.
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
	return newWithDescriptor(d.newCompilerWithDescriptor, newMTL4CompilerDescriptor, nil)
}

func (d mtlDevice) newCompilerWithDescriptor(descriptor mtl4CompilerDescriptor) (mtl4Compiler, error) {
	var nativeError id
	object := deviceNewCompilerWithDescriptor(d, descriptor, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "cannot create Metal 4 compiler"))
	}

	return object, nil
}

// newResidencySet creates and labels the backend's residency set.
func (d mtlDevice) newResidencySet() (mtlResidencySet, error) {
	return newWithDescriptor(d.newResidencySetWithDescriptor, newMTLResidencySetDescriptor,
		func(descriptor mtlResidencySetDescriptor) {
			descriptor.setLabel("gamekit.residency")
		})
}

func (d mtlDevice) newResidencySetWithDescriptor(descriptor mtlResidencySetDescriptor) (mtlResidencySet, error) {
	var nativeError id
	object := mtlResidencySet(deviceNewResidencySetWithDescriptor(d, descriptor, &nativeError))
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "cannot create residency set"))
	}

	return object, nil
}

// newCommandAllocator creates an allocator with a default descriptor and reports Go errors.
func (d mtlDevice) newCommandAllocator() (mtl4CommandAllocator, error) {
	return newWithDescriptor(d.newCommandAllocatorWithDescriptor, newMTL4CommandAllocatorDescriptor, nil)
}

func (d mtlDevice) newCommandAllocatorWithDescriptor(descriptor mtl4CommandAllocatorDescriptor) (mtl4CommandAllocator, error) {
	var nativeError id
	object := deviceNewCommandAllocatorWithDescriptor(d, descriptor, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "cannot create command allocator"))
	}

	return object, nil
}

// newArgumentTable configures the buffer count and reports Go errors.
func (d mtlDevice) newArgumentTable(maxBufferBindCount uint) (mtl4ArgumentTable, error) {
	return newWithDescriptor(d.newArgumentTableWithDescriptor, newMTL4ArgumentTableDescriptor,
		func(descriptor mtl4ArgumentTableDescriptor) {
			descriptor.setMaxBufferBindCount(maxBufferBindCount)
		})
}

func (d mtlDevice) newArgumentTableWithDescriptor(descriptor mtl4ArgumentTableDescriptor) (mtl4ArgumentTable, error) {
	var nativeError id
	object := deviceNewArgumentTableWithDescriptor(d, descriptor, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "cannot create argument table"))
	}

	return object, nil
}

func (d mtlDevice) newCounterHeap(descriptor mtl4CounterHeapDescriptor) (mtl4CounterHeap, error) {
	var nativeError id
	object := deviceNewCounterHeapWithDescriptor(d, descriptor, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "cannot create counter heap"))
	}

	return object, nil
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

func (d mtlDevice) newLibraryWithData(data id) (mtlLibrary, error) {
	var nativeError id
	object := deviceNewLibraryWithData(d, data, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "shader compilation failed"))
	}

	return object, nil
}

func (d mtlDevice) newLibraryWithSource(source, options id) (mtlLibrary, error) {
	var nativeError id
	object := deviceNewLibraryWithSource(d, source, options, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "shader compilation failed"))
	}

	return object, nil
}

func (d mtlDevice) sampleTimestamps(cpu, gpu *uint64) {
	deviceSampleTimestamps(d, cpu, gpu)
}
