//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/strings"
)

func init() {
	// Register a lazily-constructed instance; the renderer calls Init() to actually
	// create the device. Default surface extensions are enabled so a window can be
	// attached later (see defaultInstanceExtensions, which is platform-specific).
	gpu.RegisterBackend(New(defaultInstanceExtensions()...), "vulkan", 2)
}

// Backend implements gpu.Backend (asserted here so signature drift is a compile
// error, not a late failure at the first cross-package use).
var _ gpu.Backend = (*Backend)(nil)

// Backend is the Vulkan implementation of gpu.Backend. Only device init is wired
// so far; memory/bindless/pipelines/swapchain/commands land in sibling files.
type Backend struct {
	// users is how many Inits have no Destroy yet: the device lives while any do.
	users int

	instance vkInstance
	// maxAnisotropy is the device's maxSamplerAnisotropy limit, cached at Init so
	// CreateSampler can clamp to it — callers ask for the quality they want and
	// get whatever the hardware can actually do.
	maxAnisotropy float32
	// maxDataSize is the device's push-constant limit, reported by GetMaxDataSize
	// and used to size the shared pipeline layout's push range.
	maxDataSize    int32
	physicalDevice vkPhysicalDevice
	// properties, memoryProperties and queueFamilies describe physicalDevice, read once
	// at Init.
	properties       vkPhysicalDeviceProperties
	memoryProperties vkPhysicalDeviceMemoryProperties
	queueFamilies    []vkQueueFamilyProperties
	device           vkDevice
	queue            vkQueue
	queueFamily      uint32

	// Bindless heap: one descriptor set (sampled/storage images + samplers) and
	// the pipeline layout every pipeline shares (that set + push-constant root).
	setLayout      vkDescriptorSetLayout
	descPool       vkDescriptorPool
	descSet        vkDescriptorSet
	pipelineLayout vkPipelineLayout

	capSampled, capStorage, capSampler uint32
	// textureNext is the next index in the sampled and storage arrays, which share one
	// index space; samplerNext the next in the sampler array.
	textureNext, samplerNext uint32

	// Transient command recording.
	cmdPool vkCommandPool
	fences  map[uint64]fenceEntry

	// Resource registries, keyed by gpu.Handle (backend-private).
	buffers    map[uint64]bufferEntry
	textures   map[uint64]*textureEntry
	samplers   map[uint64]vkSampler
	pipelines  map[uint64]pipelineEntry
	swapchains map[uint64]*swapchainState
	queryPools map[uint64]vkQueryPool
	activeSwap *swapchainState // set between AcquireNext and Present

	// uninitialized holds the textures still in VK_IMAGE_LAYOUT_UNDEFINED: created, or
	// a backbuffer just acquired. The next command buffer to record a pass, dispatch,
	// copy or barrier moves them to GENERAL first — at the latest the next one begun,
	// which is what covers a texture whose first use is a compute shader writing it.
	uninitialized []uint64

	// nextID hands out backend-private handle ids. Atomic so resources can be
	// created from multiple goroutines without a lock on the counter itself.
	// (The registry maps above are not yet guarded — that's a separate step.)
	nextID atomic.Uint64

	instanceExts []string

	// scratch is pinned by pin from Init to Destroy (see scratch).
	scratch *scratch
	pin     runtime.Pinner
}

// New creates the Vulkan instance/device on the first suitable (preferably
// discrete) GPU, enabling the bindless + dynamic-rendering feature set.
// instanceExtensions are extra instance extensions to enable (e.g. the surface
// extensions a windowing lib requires); pass none for headless use.
// New constructs a lazy Vulkan backend: no device is created until Init(). The
// instance extensions (e.g. surface extensions) are stored for Init to enable.
func New(instanceExtensions ...string) *Backend {
	return &Backend{
		buffers:      map[uint64]bufferEntry{},
		textures:     map[uint64]*textureEntry{},
		pipelines:    map[uint64]pipelineEntry{},
		swapchains:   map[uint64]*swapchainState{},
		instanceExts: instanceExtensions,
	}
}

// Init creates the Vulkan instance, device, bindless heap and command pool. Safe to
// call once; a second call is a no-op.
func (b *Backend) Init() error {
	if b.device != nil {
		b.users++
		return nil
	}
	if r := b.createInstance(); r != vkSuccess {
		return fmt.Errorf("vulkan: create instance failed (%d)", int(r))
	}
	if r := b.pickPhysical(); r != vkSuccess {
		vkDestroyInstance(b.instance, nil)
		if r == vkErrorFeatureNotPresent {
			return fmt.Errorf("vulkan: no device supports VK_KHR_unified_image_layouts")
		}
		return fmt.Errorf("vulkan: no physical device (%d)", int(r))
	}
	var properties vkPhysicalDeviceProperties
	vkGetPhysicalDeviceProperties(b.physicalDevice, &properties)
	var memoryProperties vkPhysicalDeviceMemoryProperties
	vkGetPhysicalDeviceMemoryProperties(b.physicalDevice, &memoryProperties)
	b.properties, b.memoryProperties = properties, memoryProperties
	b.queueFamilies = queueFamilies(b.physicalDevice)
	b.queueFamily = b.graphicsQueueFamily()
	if b.queueFamily == noQueueFamily {
		vkDestroyInstance(b.instance, nil)
		return fmt.Errorf("vulkan: no graphics+compute queue family")
	}
	if r := b.createDevice(); r != vkSuccess {
		vkDestroyInstance(b.instance, nil)
		return fmt.Errorf("vulkan: create device failed (%d)", int(r))
	}
	b.maxAnisotropy = float32(b.properties.limits.maxSamplerAnisotropy)
	b.maxDataSize = int32(b.properties.limits.maxPushConstantsSize)
	if err := b.initBindless(); err != nil {
		vkDestroyDevice(b.device, nil)
		vkDestroyInstance(b.instance, nil)
		return err
	}
	if err := b.initCommands(); err != nil {
		b.destroyBindless()
		vkDestroyDevice(b.device, nil)
		vkDestroyInstance(b.instance, nil)
		return err
	}
	b.scratch = new(scratch)
	b.pin.Pin(b.scratch)
	b.users = 1
	return nil
}

// createInstance targets Vulkan 1.3. Optional instance extensions (e.g. surface
// extensions from the windowing lib) are enabled when there are any.
// TODO: make the application name configurable
// TODO: make allocator configurable
// TODO: Add VK_LAYER_KHRONOS_validation &&  VK_EXT_debug_utils, only enabled if is run in debug
// mode (configurable in the Renderer)
func (b *Backend) createInstance() vkResult {
	var pin runtime.Pinner
	defer pin.Unpin()
	name := pinnedCString(&pin, "gamekit")
	ci := vkInstanceCreateInfo{
		sType: vkStructureTypeInstanceCreateInfo,
		pApplicationInfo: pinned(&pin, &vkApplicationInfo{
			sType:            vkStructureTypeApplicationInfo,
			pApplicationName: name,
			pEngineName:      name,
			apiVersion:       vkApiVersion1_3,
		}),
		enabledExtensionCount:   cUint32(len(b.instanceExts)),
		ppEnabledExtensionNames: cStrings(&pin, b.instanceExts),
	}
	var instance vkInstance
	r := vkCreateInstance(&ci, nil, &instance)
	b.instance = instance
	return r
}

// supportsUnifiedImageLayouts reports whether pd offers VK_KHR_unified_image_layouts
// with its unifiedImageLayouts feature: every image lives in VK_IMAGE_LAYOUT_GENERAL
// for its whole life, which is what lets the gpu API have no layouts at all.
func supportsUnifiedImageLayouts(pd vkPhysicalDevice) bool {
	var n cUint32
	vkEnumerateDeviceExtensionProperties(pd, nil, &n, nil)
	if n == 0 {
		return false
	}
	exts := make([]vkExtensionProperties, n)
	vkEnumerateDeviceExtensionProperties(pd, nil, &n, &exts[0])
	found := false
	for i := range exts[:n] {
		if strings.GoString((*byte)(unsafe.Pointer(&exts[i].extensionName[0]))) == vkKhrUnifiedImageLayoutsExtensionName {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	var pin runtime.Pinner
	defer pin.Unpin()
	unified := pinned(&pin, &vkPhysicalDeviceUnifiedImageLayoutsFeaturesKHR{
		sType: vkStructureTypePhysicalDeviceUnifiedImageLayoutsFeaturesKHR,
	})
	f2 := vkPhysicalDeviceFeatures2{
		sType: vkStructureTypePhysicalDeviceFeatures2,
		pNext: unsafe.Pointer(unified),
	}
	vkGetPhysicalDeviceFeatures2(pd, &f2)
	return unified.unifiedImageLayouts == vkTrue
}

// pickPhysical chooses a device supporting unified image layouts, preferring a
// discrete GPU. VK_ERROR_FEATURE_NOT_PRESENT means devices exist but none supports them.
// TODO: allow the user to define what device they want
func (b *Backend) pickPhysical() vkResult {
	var n cUint32
	vkEnumeratePhysicalDevices(b.instance, &n, nil)
	if n == 0 {
		return vkErrorInitializationFailed
	}
	devs := make([]vkPhysicalDevice, n)
	vkEnumeratePhysicalDevices(b.instance, &n, &devs[0])

	var chosen vkPhysicalDevice
	for _, d := range devs[:n] {
		if !supportsUnifiedImageLayouts(d) {
			continue
		}
		var p vkPhysicalDeviceProperties
		vkGetPhysicalDeviceProperties(d, &p)
		discrete := p.deviceType == vkPhysicalDeviceTypeDiscreteGpu
		if chosen == nil || discrete {
			chosen = d
		}
		if discrete {
			break
		}
	}
	if chosen == nil {
		return vkErrorFeatureNotPresent
	}
	b.physicalDevice = chosen
	return vkSuccess
}

// queueFamilies returns pd's queue family properties.
// TODO: switch to vkGetPhysicalDeviceQueueFamilyProperties2
func queueFamilies(pd vkPhysicalDevice) []vkQueueFamilyProperties {
	var n cUint32
	vkGetPhysicalDeviceQueueFamilyProperties(pd, &n, nil)
	if n == 0 {
		return nil
	}
	q := make([]vkQueueFamilyProperties, n)
	vkGetPhysicalDeviceQueueFamilyProperties(pd, &n, &q[0])
	return q[:n]
}

// graphicsQueueFamily returns a family index with graphics+compute, or noQueueFamily.
func (b *Backend) graphicsQueueFamily() uint32 {
	const want = vkQueueGraphicsBit | vkQueueComputeBit
	for i, q := range b.queueFamilies {
		if q.queueFlags&want == want {
			return uint32(i)
		}
	}
	return noQueueFamily
}

// createDevice enables the 1.3 feature chain the gpu relies on: buffer device
// address, descriptor indexing (bindless heaps), dynamic rendering, sync2 — and
// unified image layouts, which pickPhysical has already checked for.
// TODO: when we move to multi-threading use dedicated transfer queues
func (b *Backend) createDevice() vkResult {
	var pin runtime.Pinner
	defer pin.Unpin()

	unified := pinned(&pin, &vkPhysicalDeviceUnifiedImageLayoutsFeaturesKHR{
		sType:               vkStructureTypePhysicalDeviceUnifiedImageLayoutsFeaturesKHR,
		unifiedImageLayouts: vkTrue,
	})
	f13 := pinned(&pin, &vkPhysicalDeviceVulkan13Features{
		sType:            vkStructureTypePhysicalDeviceVulkan1_3Features,
		pNext:            unsafe.Pointer(unified),
		dynamicRendering: vkTrue,
		synchronization2: vkTrue,
	})
	f12 := pinned(&pin, &vkPhysicalDeviceVulkan12Features{
		sType:                                    vkStructureTypePhysicalDeviceVulkan1_2Features,
		pNext:                                    unsafe.Pointer(f13),
		bufferDeviceAddress:                      vkTrue,
		descriptorIndexing:                       vkTrue,
		runtimeDescriptorArray:                   vkTrue,
		descriptorBindingPartiallyBound:          vkTrue,
		descriptorBindingVariableDescriptorCount: vkTrue,
		descriptorBindingSampledImageUpdateAfterBind: vkTrue,
		descriptorBindingStorageImageUpdateAfterBind: vkTrue,
		descriptorBindingUpdateUnusedWhilePending:    vkTrue,
		shaderSampledImageArrayNonUniformIndexing:    vkTrue,
		shaderStorageImageArrayNonUniformIndexing:    vkTrue,
		scalarBlockLayout:                            vkTrue, // GL_EXT_scalar_block_layout in the GLSL
		timelineSemaphore:                            vkTrue,
	})
	f2 := pinned(&pin, &vkPhysicalDeviceFeatures2{
		sType: vkStructureTypePhysicalDeviceFeatures2,
		pNext: unsafe.Pointer(f12),
	})
	// Base features: drawIndirectFirstInstance lets each indirect command set its
	// own firstInstance, so one multi-draw-indirect call can cover many geometries
	// (gl_InstanceIndex directly indexes the compacted visible buffer).
	f2.features.drawIndirectFirstInstance = vkTrue
	f2.features.multiDrawIndirect = vkTrue
	// Anisotropic filtering is a base feature and must be requested explicitly,
	// or creating a sampler with anisotropyEnable is invalid.
	f2.features.samplerAnisotropy = vkTrue
	// 64-bit integers in shaders. A device address held in a buffer is a
	// PhysicalStorageBuffer pointer and needs nothing extra, but the same address
	// held in PUSH CONSTANTS is loaded as a plain 64-bit scalar, which makes the
	// compiler emit the Int64 capability. Without this, every shader with an address
	// in its root fails to create — silently, as a blank frame rather than an error.
	f2.features.shaderInt64 = vkTrue
	// Storage images are declared without a format qualifier, so one array serves every
	// format; writing through one needs this. Reads go through sampling instead.
	f2.features.shaderStorageImageWriteWithoutFormat = vkTrue

	extensions := []string{vkKhrSwapchainExtensionName, vkKhrUnifiedImageLayoutsExtensionName}
	ci := vkDeviceCreateInfo{
		sType:                vkStructureTypeDeviceCreateInfo,
		pNext:                unsafe.Pointer(f2), // features2 chain (pEnabledFeatures must be NULL when using it)
		queueCreateInfoCount: 1,
		pQueueCreateInfos: pinned(&pin, &vkDeviceQueueCreateInfo{
			sType:            vkStructureTypeDeviceQueueCreateInfo,
			queueFamilyIndex: cUint32(b.queueFamily),
			queueCount:       1,
			pQueuePriorities: pinned(&pin, new(cFloat(1))),
		}),
		enabledExtensionCount:   cUint32(len(extensions)),
		ppEnabledExtensionNames: cStrings(&pin, extensions),
	}
	var device vkDevice
	if r := vkCreateDevice(b.physicalDevice, &ci, nil, &device); r != vkSuccess {
		return r
	}
	var queue vkQueue
	vkGetDeviceQueue(device, cUint32(b.queueFamily), 0, &queue)
	b.device, b.queue = device, queue
	return vkSuccess
}

// Destroy removes a user (see gpu.Backend.Init), and with the last releases the device
// and instance.
func (b *Backend) Destroy() {
	if b.users > 1 {
		b.users--
		return
	}
	b.users = 0
	if b.device != nil {
		vkDeviceWaitIdle(b.device)
		for _, p := range b.pipelines {
			vkDestroyPipeline(b.device, p.pipe, nil)
		}
		for _, qp := range b.queryPools {
			vkDestroyQueryPool(b.device, qp, nil)
		}
		// The backend is a shared singleton (gpu.Instance); a later renderer re-inits
		// the device. Reset the handle registries so they don't retain dead handles
		// from this device (a subsequent Destroy would double-free them).
		b.pipelines = map[uint64]pipelineEntry{}
		b.queryPools = map[uint64]vkQueryPool{}
		b.destroyAllSwapchains()
		b.destroyAllTextures()
		b.destroyAllBuffers()
		b.destroyCommands()
		b.destroyBindless()
		vkDestroyDevice(b.device, nil)
		b.device = nil
	}
	if b.instance != nil {
		vkDestroyInstance(b.instance, nil)
		b.instance = nil
	}
	b.pin.Unpin()
	b.scratch = nil
}

// InstanceHandle returns the VkInstance as a uintptr for native surface creation.
func (b *Backend) InstanceHandle() uintptr {
	return uintptr(unsafe.Pointer(b.instance))
}

// DeviceName returns the selected GPU's name (diagnostics).
func (b *Backend) DeviceName() string {
	return strings.GoString((*byte)(unsafe.Pointer(&b.properties.deviceName[0])))
}

// APIVersion returns the device's supported Vulkan version as (major, minor, patch).
func (b *Backend) APIVersion() (uint32, uint32, uint32) {
	v := uint32(b.properties.apiVersion)
	return (v >> 22) & 0x7F, (v >> 12) & 0x3FF, v & 0xFFF
}

// GetMaxDataSize is the largest draw/dispatch data this device accepts, in bytes.
func (b *Backend) GetMaxDataSize() int32 {
	return b.maxDataSize
}
