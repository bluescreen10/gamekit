//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"fmt"
	"math"
	"slices"

	"github.com/bluescreen10/gamekit/gpu"
)

type swapchain struct {
	reference
	surface  vkSurfaceKHR
	swap     vkSwapchainKHR
	format   vkFormat
	gpuFmt   gpu.Format
	mode     gpu.PresentMode
	w, h     uint32
	images   []*texture // the swapchain owns each backbuffer image
	acquire  []vkSemaphore
	rendered []vkSemaphore
	frame    uint32 // rotating sync index
	curImage uint32 // last acquired image index
}

// SwapchainFormats lists the formats the surface presents in the sRGB colour space,
// restricted to those the RHI can name.
func (b *Backend) SwapchainFormats(surface gpu.Surface) []gpu.Format {
	vs := surfaceFromHandle(surface)
	var n cUint32
	vkGetPhysicalDeviceSurfaceFormatsKHR(b.physicalDevice, vs, &n, nil)
	if n == 0 {
		return nil
	}
	native := make([]vkSurfaceFormatKHR, n)
	vkGetPhysicalDeviceSurfaceFormatsKHR(b.physicalDevice, vs, &n, &native[0])
	var formats []gpu.Format
	for _, f := range native[:n] {
		// Other colour spaces (HDR10, extended linear) are left out until a swapchain
		// can ask for one.
		if f.colorSpace != vkColorSpaceSrgbNonlinearKHR {
			continue
		}
		for _, candidate := range supportedFormats {
			if formatOf(candidate) == f.format && !slices.Contains(formats, candidate) {
				formats = append(formats, candidate)
			}
		}
	}
	return formats
}

// CreateSwapchain wraps a platform VkSurfaceKHR in a swapchain, surfacing
// backbuffers as render-target Textures.
func (b *Backend) CreateSwapchain(surface gpu.Surface, desc gpu.SwapchainDescriptor) (gpu.Swapchain, error) {
	available := b.SwapchainFormats(surface)
	format := desc.Format
	if format == gpu.FormatUndefined {
		format = defaultSwapchainFormat(available)
	}
	if !slices.Contains(available, format) {
		return nil, fmt.Errorf("vulkan: the surface does not present swapchain format %v (it offers %v)", format, available)
	}

	s := &swapchain{surface: surfaceFromHandle(surface), gpuFmt: format, format: formatOf(format), mode: desc.PresentMode}
	b.buildSwapchain(s, desc.Width, desc.Height, nil)

	s.reference = reference{backend: b}
	return s, nil
}

// defaultSwapchainFormat is the format a swapchain gets when its descriptor names none:
// an 8-bit unorm one, in whichever channel order the surface offers.
func defaultSwapchainFormat(available []gpu.Format) gpu.Format {
	for _, f := range []gpu.Format{gpu.FormatBGRA8Unorm, gpu.FormatRGBA8Unorm} {
		if slices.Contains(available, f) {
			return f
		}
	}
	if len(available) > 0 {
		return available[0]
	}
	return gpu.FormatUndefined
}

// presentMode is the present mode a swapchain asking for wanted gets: wanted, if the
// surface offers it; for IMMEDIATE, MAILBOX — which replaces the waiting frame rather
// than tearing — if it offers that instead; and otherwise FIFO, which every surface
// offers.
func (b *Backend) presentMode(surface vkSurfaceKHR, wanted vkPresentModeKHR) vkPresentModeKHR {
	if wanted == vkPresentModeFifoKHR {
		return wanted
	}
	var n cUint32
	vkGetPhysicalDeviceSurfacePresentModesKHR(b.physicalDevice, surface, &n, nil)
	if n == 0 {
		return vkPresentModeFifoKHR
	}
	modes := make([]vkPresentModeKHR, n)
	vkGetPhysicalDeviceSurfacePresentModesKHR(b.physicalDevice, surface, &n, &modes[0])
	switch {
	case slices.Contains(modes[:n], wanted):
		return wanted
	case wanted == vkPresentModeImmediateKHR && slices.Contains(modes[:n], vkPresentModeMailboxKHR):
		return vkPresentModeMailboxKHR
	default:
		return vkPresentModeFifoKHR
	}
}

// buildSwapchain creates (or recreates from old) s's swapchain in its format, in the
// sRGB colour space, presenting in its mode or the nearest the surface offers, with a
// view and a pair of semaphores per image.
func (b *Backend) buildSwapchain(s *swapchain, width, height uint32, old vkSwapchainKHR) {
	var caps vkSurfaceCapabilitiesKHR
	vkGetPhysicalDeviceSurfaceCapabilitiesKHR(b.physicalDevice, s.surface, &caps)
	extent := caps.currentExtent
	if extent.width == 0xFFFFFFFF {
		extent = vkExtent2D{width: cUint32(width), height: cUint32(height)}
	}
	minImages := caps.minImageCount + 1
	if caps.maxImageCount > 0 && minImages > caps.maxImageCount {
		minImages = caps.maxImageCount
	}
	// TRANSFER_SRC lets the frame be copied back out (screenshots). It is optional:
	// the spec only guarantees COLOR_ATTACHMENT, so ask for it only where the surface
	// reports it, and a driver that refuses simply means no screenshots rather than a
	// failed swapchain.
	usage := vkImageUsageFlags(vkImageUsageColorAttachmentBit | vkImageUsageTransferDstBit)
	if caps.supportedUsageFlags&vkImageUsageTransferSrcBit != 0 {
		usage |= vkImageUsageTransferSrcBit
	}
	ci := vkSwapchainCreateInfoKHR{
		sType:            vkStructureTypeSwapchainCreateInfoKHR,
		surface:          s.surface,
		minImageCount:    minImages,
		imageFormat:      s.format,
		imageColorSpace:  vkColorSpaceSrgbNonlinearKHR,
		imageExtent:      extent,
		imageArrayLayers: 1,
		imageUsage:       usage,
		imageSharingMode: vkSharingModeExclusive,
		preTransform:     caps.currentTransform,
		compositeAlpha:   vkCompositeAlphaOpaqueBitKHR,
		presentMode:      b.presentMode(s.surface, presentModeOf(s.mode)),
		clipped:          vkTrue,
		oldSwapchain:     old,
	}
	var swap vkSwapchainKHR
	if r := vkCreateSwapchainKHR(b.device, &ci, nil, &swap); r != vkSuccess {
		panic(fmt.Sprintf("vulkan: swapchain creation failed (%d)", int(r)))
	}
	s.swap = swap
	s.w, s.h = uint32(extent.width), uint32(extent.height)

	var n cUint32
	vkGetSwapchainImagesKHR(b.device, s.swap, &n, nil)
	imgs := make([]vkImage, n)
	vkGetSwapchainImagesKHR(b.device, s.swap, &n, &imgs[0])

	s.images = make([]*texture, n)
	s.acquire = make([]vkSemaphore, n)
	s.rendered = make([]vkSemaphore, n)
	si := vkSemaphoreCreateInfo{sType: vkStructureTypeSemaphoreCreateInfo}
	for i := range imgs[:n] {
		vci := vkImageViewCreateInfo{
			sType:    vkStructureTypeImageViewCreateInfo,
			image:    imgs[i],
			viewType: vkImageViewType2D,
			format:   s.format,
			subresourceRange: vkImageSubresourceRange{
				aspectMask: vkImageAspectColorBit,
				levelCount: 1,
				layerCount: 1,
			},
		}
		var view vkImageView
		vkCreateImageView(b.device, &vci, nil, &view)
		t := &texture{
			img: imgs[i], view: view, format: s.format,
			width: s.w, height: s.h, owned: false,
		}
		t.reference = reference{backend: b}
		s.images[i] = t
		vkCreateSemaphore(b.device, &si, nil, &s.acquire[i])
		vkCreateSemaphore(b.device, &si, nil, &s.rendered[i])
	}
}

// AcquireNext acquires the next backbuffer and returns it as a render-target
// Texture. The returned Fence is unused (present sync is via internal semaphores;
// CPU throttling is via WaitIdle for now).
func (b *Backend) AcquireNext(sc gpu.Swapchain) (gpu.Texture, gpu.Fence) {
	s := resourceOf[*swapchain](b, sc)
	s.frame = (s.frame + 1) % uint32(len(s.acquire))
	var idx cUint32
	vkAcquireNextImageKHR(b.device, s.swap, math.MaxUint64, s.acquire[s.frame], nil, &idx)
	s.curImage = uint32(idx)
	// Fresh acquire: the presentation engine hands the image back in PRESENT_SRC, and
	// its prior contents are undefined for our purposes, so it starts again from
	// UNDEFINED, like a new image.
	b.uninitialized = append(b.uninitialized, s.images[s.curImage])
	return s.images[s.curImage], nil
}

// Present ends and submits the recorded command list (waiting on the acquire
// semaphore, signalling render-done) and presents the backbuffer. The caller
// must have recorded rendering into the Texture returned by AcquireNext.
func (b *Backend) Present(sc gpu.Swapchain, cmd gpu.CommandBuffer) {
	s := resourceOf[*swapchain](b, sc)
	c := cmd.(*cmdBuffer)

	// Transition the backbuffer from GENERAL to PRESENT_SRC before ending the buffer —
	// the one layout unified image layouts leave in place, since the presentation
	// engine reads it. Anything may have written the backbuffer last: a draw, a copy.
	e := b.tex(s.images[s.curImage])
	c.imageBarrier(e.img, vkImageAspectColorBit, vkPipelineStage2AllCommandsBit, vkAccess2MemoryWriteBit, vkPipelineStage2BottomOfPipeBit, vkAccess2None,
		vkImageLayoutGeneral, vkImageLayoutPresentSrcKHR)
	vkEndCommandBuffer(c.cb)

	// Submit waiting on the acquire semaphore and signalling render-done, then present
	// once render-done signals.
	if r := b.submit(c.cb, s.acquire[s.frame], s.rendered[s.frame], nil); r == vkSuccess {
		sc := b.scratch
		sc.swapchain = s.swap
		sc.imageIndex = cUint32(s.curImage)
		sc.renderDone = s.rendered[s.frame]
		sc.present = vkPresentInfoKHR{
			sType:              vkStructureTypePresentInfoKHR,
			waitSemaphoreCount: 1,
			pWaitSemaphores:    &sc.renderDone,
			swapchainCount:     1,
			pSwapchains:        &sc.swapchain,
			pImageIndices:      &sc.imageIndex,
		}
		vkQueuePresentKHR(b.queue, &sc.present)
	}

	// Simple throttle (per-frame fences / frames-in-flight come later).
	vkQueueWaitIdle(b.queue)
	cb := c.cb
	vkFreeCommandBuffers(b.device, b.cmdPool, 1, &cb)
}

// SwapchainFormat returns the swapchain's color format.
func (b *Backend) SwapchainFormat(sc gpu.Swapchain) gpu.Format {
	return resourceOf[*swapchain](b, sc).gpuFmt
}

// SwapchainSize returns the swapchain's actual backbuffer extent (which may
// differ from a window's framebuffer size on HiDPI). Use it for the viewport.
func (b *Backend) SwapchainSize(sc gpu.Swapchain) (width, height uint32) {
	s := resourceOf[*swapchain](b, sc)
	return s.w, s.h
}

// ResizeSwapchain recreates the swapchain at a new size.
func (b *Backend) ResizeSwapchain(sc gpu.Swapchain, width, height uint32) {
	s := resourceOf[*swapchain](b, sc)
	vkDeviceWaitIdle(b.device)
	b.destroySwapchainResources(s)
	b.buildSwapchain(s, width, height, s.swap)
}

// presentModeOf is the Vulkan present mode a swapchain presenting in mode asks for.
func presentModeOf(mode gpu.PresentMode) vkPresentModeKHR {
	switch mode {
	case gpu.PresentImmediate:
		return vkPresentModeImmediateKHR
	case gpu.PresentMailbox:
		return vkPresentModeMailboxKHR
	default:
		return vkPresentModeFifoKHR
	}
}

// SetPresentMode recreates the swapchain presenting in mode, at its current size.
func (b *Backend) SetPresentMode(sc gpu.Swapchain, mode gpu.PresentMode) {
	s := resourceOf[*swapchain](b, sc)
	if s.mode == mode {
		return
	}
	vkDeviceWaitIdle(b.device)
	s.mode = mode
	b.destroySwapchainResources(s)
	b.buildSwapchain(s, s.w, s.h, s.swap)
}

func (b *Backend) DestroySwapchain(s gpu.Swapchain) {
	resourceOf[*swapchain](b, s).destroy()
}

func (b *Backend) destroySwapchainResources(s *swapchain) {
	for i := range s.images {
		s.images[i].destroy()
		vkDestroySemaphore(b.device, s.acquire[i], nil)
		vkDestroySemaphore(b.device, s.rendered[i], nil)
	}
	vkDestroySwapchainKHR(b.device, s.swap, nil)
}
