//go:build !darwin || gamekit_vulkan

package vulkan

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// Desired heap capacities (clamped to device update-after-bind limits at init).
// Sampled and storage images share one index space — a texture that is both has one
// index, written into both arrays — so the storage array is as large as the sampled.
const (
	wantSampledImages = 16384
	wantStorageImages = wantSampledImages
	wantSamplers      = 512
)

// Bindless heap binding slots (set 0). One global set, shared by every pipeline.
const (
	bindSampled = 0 // texture2D[]  (sampled images)
	bindStorage = 1 // image2D[]    (storage images)
	bindSampler = 2 // sampler[]
)

// initBindless builds the one descriptor set layout (3 update-after-bind arrays), pool,
// persistent set, and the pipeline layout every pipeline shares (that set + a
// push-constant holding the 64-bit root pointer). Capacities are clamped to the
// device's update-after-bind limits, and the clamped values bound the slot allocators.
// TODO: make wantedSampledImages, wantStorageImages, and wantSamplers configurable
func (b *Backend) initBindless() error {
	if r := b.createBindlessHeap(); r != vkSuccess {
		return fmt.Errorf("vulkan: bindless heap creation failed (%d)", int(r))
	}
	b.samplers = map[uint64]vkSampler{}
	return nil
}

func (b *Backend) createBindlessHeap() vkResult {
	var pin runtime.Pinner
	defer pin.Unpin()

	p12 := pinned(&pin, &vkPhysicalDeviceVulkan12Properties{
		sType: vkStructureTypePhysicalDeviceVulkan1_2Properties,
	})
	p2 := vkPhysicalDeviceProperties2{
		sType: vkStructureTypePhysicalDeviceProperties2,
		pNext: unsafe.Pointer(p12),
	}
	vkGetPhysicalDeviceProperties2(b.physicalDevice, &p2)
	b.capSampled = min(wantSampledImages, uint32(p12.maxPerStageDescriptorUpdateAfterBindSampledImages),
		uint32(p12.maxDescriptorSetUpdateAfterBindSampledImages))
	b.capStorage = min(wantStorageImages, uint32(p12.maxPerStageDescriptorUpdateAfterBindStorageImages),
		uint32(p12.maxDescriptorSetUpdateAfterBindStorageImages))
	b.capSampler = min(wantSamplers, uint32(p12.maxPerStageDescriptorUpdateAfterBindSamplers),
		uint32(p12.maxDescriptorSetUpdateAfterBindSamplers))

	binds := pinned(&pin, &[3]vkDescriptorSetLayoutBinding{
		{binding: bindSampled, descriptorType: vkDescriptorTypeSampledImage, descriptorCount: cUint32(b.capSampled), stageFlags: vkShaderStageAll},
		{binding: bindStorage, descriptorType: vkDescriptorTypeStorageImage, descriptorCount: cUint32(b.capStorage), stageFlags: vkShaderStageAll},
		{binding: bindSampler, descriptorType: vkDescriptorTypeSampler, descriptorCount: cUint32(b.capSampler), stageFlags: vkShaderStageAll},
	})
	const bindingFlags = vkDescriptorBindingUpdateAfterBindBit | vkDescriptorBindingPartiallyBoundBit
	flags := pinned(&pin, &[3]vkDescriptorBindingFlags{bindingFlags, bindingFlags, bindingFlags})
	lci := vkDescriptorSetLayoutCreateInfo{
		sType: vkStructureTypeDescriptorSetLayoutCreateInfo,
		pNext: unsafe.Pointer(pinned(&pin, &vkDescriptorSetLayoutBindingFlagsCreateInfo{
			sType:         vkStructureTypeDescriptorSetLayoutBindingFlagsCreateInfo,
			bindingCount:  3,
			pBindingFlags: &flags[0],
		})),
		flags:        vkDescriptorSetLayoutCreateUpdateAfterBindPoolBit,
		bindingCount: 3,
		pBindings:    &binds[0],
	}
	var setLayout vkDescriptorSetLayout
	if r := vkCreateDescriptorSetLayout(b.device, &lci, nil, &setLayout); r != vkSuccess {
		return r
	}
	b.setLayout = setLayout

	sizes := pinned(&pin, &[3]vkDescriptorPoolSize{
		{_type: vkDescriptorTypeSampledImage, descriptorCount: cUint32(b.capSampled)},
		{_type: vkDescriptorTypeStorageImage, descriptorCount: cUint32(b.capStorage)},
		{_type: vkDescriptorTypeSampler, descriptorCount: cUint32(b.capSampler)},
	})
	pci := vkDescriptorPoolCreateInfo{
		sType:         vkStructureTypeDescriptorPoolCreateInfo,
		flags:         vkDescriptorPoolCreateUpdateAfterBindBit,
		maxSets:       1,
		poolSizeCount: 3,
		pPoolSizes:    &sizes[0],
	}
	var descPool vkDescriptorPool
	r := vkCreateDescriptorPool(b.device, &pci, nil, &descPool)
	b.descPool = descPool
	if r != vkSuccess {
		vkDestroyDescriptorSetLayout(b.device, b.setLayout, nil)
		return r
	}

	layouts := pinned(&pin, &[1]vkDescriptorSetLayout{b.setLayout})
	sai := vkDescriptorSetAllocateInfo{
		sType:              vkStructureTypeDescriptorSetAllocateInfo,
		descriptorPool:     b.descPool,
		descriptorSetCount: 1,
		pSetLayouts:        &layouts[0],
	}
	var descSet vkDescriptorSet
	r = vkAllocateDescriptorSets(b.device, &sai, &descSet)
	b.descSet = descSet
	if r != vkSuccess {
		vkDestroyDescriptorPool(b.device, b.descPool, nil)
		vkDestroyDescriptorSetLayout(b.device, b.setLayout, nil)
		return r
	}

	// Every pipeline shares this layout, with a push constant range sized to what the
	// device actually permits rather than a fixed guess. It was 128 bytes, which is
	// smaller than some roots callers legitimately want to pass — and the limit is a
	// driver policy decision, not a hardware one (the same Apple GPU reports 4096
	// through MoltenVK and 256 through KosmicKrisp), so it has to be queried.
	plci := vkPipelineLayoutCreateInfo{
		sType:                  vkStructureTypePipelineLayoutCreateInfo,
		setLayoutCount:         1,
		pSetLayouts:            &layouts[0],
		pushConstantRangeCount: 1,
		pPushConstantRanges: pinned(&pin, &vkPushConstantRange{
			stageFlags: vkShaderStageAll,
			size:       b.properties.limits.maxPushConstantsSize,
		}),
	}
	var pipelineLayout vkPipelineLayout
	r = vkCreatePipelineLayout(b.device, &plci, nil, &pipelineLayout)
	b.pipelineLayout = pipelineLayout
	if r != vkSuccess {
		vkDestroyDescriptorPool(b.device, b.descPool, nil) // frees the allocated set too
		vkDestroyDescriptorSetLayout(b.device, b.setLayout, nil)
		return r
	}
	return vkSuccess
}

// writeDescriptor writes one image or sampler descriptor into the bindless set at
// binding's array element index.
func (b *Backend) writeDescriptor(binding uint32, kind vkDescriptorType, index uint32, image vkDescriptorImageInfo) {
	var pin runtime.Pinner
	defer pin.Unpin()
	w := vkWriteDescriptorSet{
		sType:           vkStructureTypeWriteDescriptorSet,
		dstSet:          b.descSet,
		dstBinding:      cUint32(binding),
		dstArrayElement: cUint32(index),
		descriptorCount: 1,
		descriptorType:  kind,
		pImageInfo:      pinned(&pin, &image),
	}
	vkUpdateDescriptorSets(b.device, 1, &w, 0, nil)
}

// writeSampledImage registers a view into the bindless sampled-image array. The image
// is in VK_IMAGE_LAYOUT_GENERAL whenever the descriptor is used, as every image is —
// including a depth buffer bound as an attachment in the same pass.
func (b *Backend) writeSampledImage(index uint32, view vkImageView) {
	b.writeDescriptor(bindSampled, vkDescriptorTypeSampledImage, index,
		vkDescriptorImageInfo{imageView: view, imageLayout: vkImageLayoutGeneral})
}

func (b *Backend) writeStorageImage(index uint32, view vkImageView) {
	b.writeDescriptor(bindStorage, vkDescriptorTypeStorageImage, index,
		vkDescriptorImageInfo{imageView: view, imageLayout: vkImageLayoutGeneral})
}

func (b *Backend) destroyBindless() {
	for _, s := range b.samplers {
		vkDestroySampler(b.device, s, nil)
	}
	if b.pipelineLayout != nil {
		vkDestroyPipelineLayout(b.device, b.pipelineLayout, nil)
	}
	if b.descPool != nil {
		vkDestroyDescriptorPool(b.device, b.descPool, nil) // frees the set too
	}
	if b.setLayout != nil {
		vkDestroyDescriptorSetLayout(b.device, b.setLayout, nil)
	}
}

func filter(linear bool) vkFilter {
	if linear {
		return vkFilterLinear
	}
	return vkFilterNearest
}

func mipmapMode(linear bool) vkSamplerMipmapMode {
	if linear {
		return vkSamplerMipmapModeLinear
	}
	return vkSamplerMipmapModeNearest
}

func addressMode(m gpu.AddressMode) vkSamplerAddressMode {
	switch m {
	case gpu.AddressRepeat:
		return vkSamplerAddressModeRepeat
	case gpu.AddressMirror:
		return vkSamplerAddressModeMirroredRepeat
	default:
		return vkSamplerAddressModeClampToEdge
	}
}

// CreateSampler registers a sampler into the bindless heap and returns its index.
func (b *Backend) CreateSampler(d gpu.SamplerDescriptor) gpu.Sampler {
	// Clamp to what the device supports: a sampler asking for more anisotropy than
	// maxSamplerAnisotropy is invalid, and callers shouldn't have to query first.
	aniso := float32(d.MaxAnisotropy)
	if aniso > b.maxAnisotropy {
		aniso = b.maxAnisotropy
	}
	ci := vkSamplerCreateInfo{
		sType:            vkStructureTypeSamplerCreateInfo,
		magFilter:        filter(d.MagLinear),
		minFilter:        filter(d.MinLinear),
		mipmapMode:       mipmapMode(d.MipLinear),
		addressModeU:     addressMode(d.AddressU),
		addressModeV:     addressMode(d.AddressV),
		addressModeW:     addressMode(d.AddressW),
		maxLod:           vkLodClampNone,
		compareEnable:    bool32(d.Compare != gpu.CompareNever),
		compareOp:        compareOp(d.Compare),
		anisotropyEnable: bool32(aniso > 1),
		maxAnisotropy:    cFloat(max(aniso, 1)),
	}
	var s vkSampler
	if r := vkCreateSampler(b.device, &ci, nil, &s); r != vkSuccess {
		panic(fmt.Sprintf("vulkan: CreateSampler failed (%d)", int(r)))
	}
	index := b.samplerNext
	b.samplerNext++
	b.writeDescriptor(bindSampler, vkDescriptorTypeSampler, index, vkDescriptorImageInfo{sampler: s})

	h := b.nextID.Add(1)
	b.samplers[h] = s
	return gpu.Sampler{Index: index, H: gpu.Handle(h)}
}

// DestroySampler releases a sampler (its heap slot is not reclaimed yet).
func (b *Backend) DestroySampler(s gpu.Sampler) {
	vs, ok := b.samplers[uint64(s.H)]
	if !ok {
		return
	}
	vkDestroySampler(b.device, vs, nil)
	delete(b.samplers, uint64(s.H))
}

// compareOp maps an gpu compare op to Vulkan.
func compareOp(op gpu.CompareOp) vkCompareOp {
	switch op {
	case gpu.CompareLess:
		return vkCompareOpLess
	case gpu.CompareEqual:
		return vkCompareOpEqual
	case gpu.CompareLessEqual:
		return vkCompareOpLessOrEqual
	case gpu.CompareGreater:
		return vkCompareOpGreater
	case gpu.CompareNotEqual:
		return vkCompareOpNotEqual
	case gpu.CompareGreaterEqual:
		return vkCompareOpGreaterOrEqual
	case gpu.CompareAlways:
		return vkCompareOpAlways
	default:
		return vkCompareOpNever
	}
}

// HeapCapacities returns the clamped bindless array sizes (diagnostics).
func (b *Backend) HeapCapacities() (sampled, storage, sampler uint32) {
	return b.capSampled, b.capStorage, b.capSampler
}
