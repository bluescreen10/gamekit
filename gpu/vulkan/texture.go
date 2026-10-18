//go:build !darwin || gamekit_vulkan

package vulkan

import (
	"fmt"

	"github.com/bluescreen10/gamekit/gpu"
)

// textureEntry is the backend record for a Texture handle.
//
// It records no layout. With VK_KHR_unified_image_layouts every image lives in
// VK_IMAGE_LAYOUT_GENERAL, once it has left UNDEFINED (see Backend.uninitialized), and
// ordering its uses is the caller's, through gpu.CommandBuffer.Barrier.
type textureEntry struct {
	img    vkImage
	mem    vkDeviceMemory
	view   vkImageView
	format vkFormat
	// width, height and depth are the base level's extent; depth is 1 but for a 3D
	// texture.
	width, height, depth uint32
	isDepthFormat        bool
	// writable says the image has storage usage, so a view of one of its mips can be
	// written as a storage image.
	writable bool
	// transient says the image was created with gpu.TextureTransient: a pass may only
	// clear it or not care what it held, and never stores it.
	transient bool
	// swapchain-owned backbuffers set owned=false so Destroy skips image/mem.
	owned bool
}

func viewType(k gpu.TextureKind) vkImageViewType {
	switch k {
	case gpu.Texture2DArray:
		return vkImageViewType2DArray
	case gpu.TextureCube:
		return vkImageViewTypeCube
	case gpu.TextureCubeArray:
		return vkImageViewTypeCubeArray
	case gpu.Texture3D:
		return vkImageViewType3D
	default:
		return vkImageViewType2D
	}
}

func imageUsage(u gpu.TextureUsage, depth bool) vkImageUsageFlags {
	var f vkImageUsageFlags
	if u&gpu.TextureSampled != 0 {
		f |= vkImageUsageSampledBit
	}
	if u&gpu.TextureStorage != 0 {
		f |= vkImageUsageStorageBit
	}
	if u&gpu.TextureRenderTarget != 0 {
		f |= vkImageUsageColorAttachmentBit
	}
	if u&gpu.TextureDepth != 0 || depth {
		f |= vkImageUsageDepthStencilAttachmentBit
	}
	if u&gpu.TextureTransfer != 0 {
		f |= vkImageUsageTransferSrcBit | vkImageUsageTransferDstBit
	}
	if u&gpu.TextureTransient != 0 {
		if u&(gpu.TextureSampled|gpu.TextureStorage|gpu.TextureTransfer) != 0 {
			panic("vulkan: a transient texture cannot be sampled, written by shaders or copied")
		}
		f |= vkImageUsageTransientAttachmentBit
	}
	return f
}

// CreateTexture creates an image (+ memory + default view) and, when
// TextureSampled/TextureStorage, registers a view into the bindless heap
// (Texture.Index is the heap slot).
func (b *Backend) CreateTexture(d gpu.TextureDescriptor) gpu.Texture {
	layers := d.Layers
	if layers == 0 || d.Kind == gpu.Texture3D {
		layers = 1
	}
	depth := d.Depth
	if depth == 0 {
		depth = 1
	}
	mips := d.Mips
	if mips == 0 {
		mips = 1
	}
	isDepth := d.Format.IsDepth()
	aspect := vkImageAspectFlags(vkImageAspectColorBit)
	if isDepth {
		aspect = vkImageAspectDepthBit
	}

	img, mem, view, r := b.createImage(formatOf(d.Format), d.Width, d.Height, depth, layers, mips, uint32(d.Samples),
		imageUsage(d.Usage, isDepth), aspect, viewType(d.Kind))
	if r != vkSuccess {
		panic(fmt.Sprintf("vulkan: CreateTexture(%dx%d, %q) failed (%d)", d.Width, d.Height, d.Label, int(r)))
	}

	h := b.nextID.Add(1)
	b.textures[h] = &textureEntry{
		img: img, mem: mem, view: view, format: formatOf(d.Format),
		width: d.Width, height: d.Height, depth: depth, isDepthFormat: isDepth,
		writable: d.Usage&gpu.TextureStorage != 0, transient: d.Usage&gpu.TextureTransient != 0, owned: true,
	}
	b.uninitialized = append(b.uninitialized, h)

	// One index for both arrays: a shader samples gTextures[i] and writes gImages[i],
	// with the same i, which is also how Metal's single texture table works.
	tex := gpu.Texture{H: gpu.Handle(h)}
	if d.Usage&(gpu.TextureSampled|gpu.TextureStorage) != 0 {
		tex.Index = b.nextTextureIndex(d.Usage)
	}
	if d.Usage&gpu.TextureSampled != 0 {
		b.writeSampledImage(tex.Index, view)
	}
	if d.Usage&gpu.TextureStorage != 0 {
		b.writeStorageImage(tex.Index, view)
	}
	return tex
}

// createImage creates an image with dedicated device-local memory and a default view.
func (b *Backend) createImage(format vkFormat, w, h, depth, layers, mips, samples uint32,
	usage vkImageUsageFlags, aspect vkImageAspectFlags, kind vkImageViewType,
) (img vkImage, mem vkDeviceMemory, view vkImageView, r vkResult) {
	ci := vkImageCreateInfo{
		sType:         vkStructureTypeImageCreateInfo,
		imageType:     vkImageType2D,
		format:        format,
		extent:        vkExtent3D{width: cUint32(w), height: cUint32(h), depth: cUint32(depth)},
		mipLevels:     cUint32(mips),
		arrayLayers:   cUint32(layers),
		samples:       vkSampleCountFlagBits(max(samples, 1)),
		tiling:        vkImageTilingOptimal,
		usage:         usage,
		sharingMode:   vkSharingModeExclusive,
		initialLayout: vkImageLayoutUndefined,
	}
	switch kind {
	case vkImageViewType3D:
		ci.imageType = vkImageType3D
	case vkImageViewTypeCube, vkImageViewTypeCubeArray:
		ci.flags |= vkImageCreateCubeCompatibleBit
	}
	if r = vkCreateImage(b.device, &ci, nil, &img); r != vkSuccess {
		return img, mem, view, r
	}

	var req vkMemoryRequirements
	vkGetImageMemoryRequirements(b.device, img, &req)
	// A transient attachment takes lazily allocated memory where the device has it —
	// tile-based GPUs, which then never commit any — and device-local memory otherwise.
	typeIndex := noMemoryType
	if usage&vkImageUsageTransientAttachmentBit != 0 {
		typeIndex = b.memoryType(req.memoryTypeBits, vkMemoryPropertyLazilyAllocatedBit)
	}
	if typeIndex == noMemoryType {
		typeIndex = b.memoryType(req.memoryTypeBits, vkMemoryPropertyDeviceLocalBit)
	}
	if typeIndex == noMemoryType {
		vkDestroyImage(b.device, img, nil)
		return img, mem, view, vkErrorOutOfDeviceMemory
	}
	ai := vkMemoryAllocateInfo{
		sType:           vkStructureTypeMemoryAllocateInfo,
		allocationSize:  req.size,
		memoryTypeIndex: cUint32(typeIndex),
	}
	if r = vkAllocateMemory(b.device, &ai, nil, &mem); r != vkSuccess {
		vkDestroyImage(b.device, img, nil)
		return img, mem, view, r
	}
	if r = vkBindImageMemory(b.device, img, mem, 0); r != vkSuccess {
		vkFreeMemory(b.device, mem, nil)
		vkDestroyImage(b.device, img, nil)
		return img, mem, view, r
	}
	if view, r = b.createView(img, format, aspect, kind, 0, mips, 0, layers); r != vkSuccess {
		vkFreeMemory(b.device, mem, nil)
		vkDestroyImage(b.device, img, nil)
	}
	return img, mem, view, r
}

// createView makes a view over a subresource range of img.
func (b *Backend) createView(img vkImage, format vkFormat, aspect vkImageAspectFlags, kind vkImageViewType,
	baseMip, mipCount, baseLayer, layerCount uint32,
) (vkImageView, vkResult) {
	ci := vkImageViewCreateInfo{
		sType:    vkStructureTypeImageViewCreateInfo,
		image:    img,
		viewType: kind,
		format:   format,
		subresourceRange: vkImageSubresourceRange{
			aspectMask:     aspect,
			baseMipLevel:   cUint32(baseMip),
			levelCount:     cUint32(mipCount),
			baseArrayLayer: cUint32(baseLayer),
			layerCount:     cUint32(layerCount),
		},
	}
	var view vkImageView
	r := vkCreateImageView(b.device, &ci, nil, &view)
	return view, r
}

func (b *Backend) destroyImage(e *textureEntry) {
	vkDestroyImageView(b.device, e.view, nil)
	vkFreeMemory(b.device, e.mem, nil)
	vkDestroyImage(b.device, e.img, nil)
}

// nextTextureIndex hands out the next index in the texture arrays, panicking when it
// falls outside an array the texture is written into.
func (b *Backend) nextTextureIndex(usage gpu.TextureUsage) uint32 {
	index := b.textureNext
	if usage&gpu.TextureSampled != 0 && index >= b.capSampled {
		panic(fmt.Sprintf("vulkan: sampled-image heap is full (%d)", b.capSampled))
	}
	if usage&gpu.TextureStorage != 0 && index >= b.capStorage {
		panic(fmt.Sprintf("vulkan: storage-image heap is full (%d)", b.capStorage))
	}
	b.textureNext++
	return index
}

// TextureView registers an additional sampled view over a subresource range of an
// existing texture (e.g. a single array layer or mip) into the bindless heap and
// returns a Texture whose Index is the new heap slot. The returned handle owns only
// the view (owned=false), so DestroyTexture frees the view but not the image.
func (b *Backend) TextureView(t gpu.Texture, kind gpu.TextureKind, baseMip, mipCount, baseLayer, layerCount uint32) gpu.Texture {
	src, ok := b.textures[uint64(t.H)]
	if !ok {
		panic("vulkan: TextureView of unknown texture")
	}
	aspect := vkImageAspectFlags(vkImageAspectColorBit)
	if src.isDepthFormat {
		aspect = vkImageAspectDepthBit
	}
	if mipCount == 0 {
		mipCount = 1
	}
	if layerCount == 0 {
		layerCount = 1
	}
	view, r := b.createView(src.img, src.format, aspect, viewType(kind), baseMip, mipCount, baseLayer, layerCount)
	if r != vkSuccess {
		panic(fmt.Sprintf("vulkan: TextureView failed (%d)", int(r)))
	}

	h := b.nextID.Add(1)
	b.textures[h] = &textureEntry{
		view: view, format: src.format, width: src.width, height: src.height, depth: src.depth,
		isDepthFormat: src.isDepthFormat, owned: false,
	}
	// A storage descriptor names a single mip, so only a one-mip view of a writable
	// image can be written; it gets the same index in both arrays, as a texture does.
	usage := gpu.TextureSampled
	if src.writable && mipCount == 1 {
		usage |= gpu.TextureStorage
	}
	tex := gpu.Texture{H: gpu.Handle(h), Index: b.nextTextureIndex(usage)}
	b.writeSampledImage(tex.Index, view)
	if usage&gpu.TextureStorage != 0 {
		b.writeStorageImage(tex.Index, view)
	}
	return tex
}

// DestroyTexture releases a texture (heap slots are not reclaimed yet).
func (b *Backend) DestroyTexture(t gpu.Texture) {
	e, ok := b.textures[uint64(t.H)]
	if !ok {
		return
	}
	if e.owned {
		b.destroyImage(e)
	} else {
		vkDestroyImageView(b.device, e.view, nil)
	}
	delete(b.textures, uint64(t.H))
}

// destroyAllTextures releases every live texture (shutdown).
func (b *Backend) destroyAllTextures() {
	for k, e := range b.textures {
		if e.owned {
			b.destroyImage(e)
		} else {
			vkDestroyImageView(b.device, e.view, nil)
		}
		delete(b.textures, k)
	}
}
