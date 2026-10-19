//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/internal/specialization"
)

func topology(t gpu.Topology) vkPrimitiveTopology {
	switch t {
	case gpu.TopologyTriangleStrip:
		return vkPrimitiveTopologyTriangleStrip
	case gpu.TopologyLines:
		return vkPrimitiveTopologyLineList
	case gpu.TopologyPoints:
		return vkPrimitiveTopologyPointList
	default:
		return vkPrimitiveTopologyTriangleList
	}
}

func cullMode(m gpu.CullMode) vkCullModeFlags {
	switch m {
	case gpu.CullBack:
		return vkCullModeBackBit
	case gpu.CullFront:
		return vkCullModeFrontBit
	default:
		return vkCullModeNone
	}
}

// shaderModule wraps SPIR-V code in a module, to be destroyed once its pipeline exists.
func (b *Backend) shaderModule(pin *runtime.Pinner, code []byte) (vkShaderModule, vkResult) {
	ci := vkShaderModuleCreateInfo{
		sType:    vkStructureTypeShaderModuleCreateInfo,
		codeSize: cSize(len(code)),
		pCode:    (*cUint32)(unsafe.Pointer(pinned(pin, &code[0]))),
	}
	var m vkShaderModule
	r := vkCreateShaderModule(b.device, &ci, nil, &m)
	return m, r
}

// ComputePipeline compiles a compute pipeline from SPIR-V, sharing the global
// bindless pipeline layout.
func (b *Backend) CreateComputePipeline(desc gpu.ComputePipelineDescriptor) gpu.Pipeline {
	if desc.Entry == "" {
		desc.Entry = "main"
	}
	if size := desc.WorkgroupSize; size[0] == 0 || size[1] == 0 || size[2] == 0 {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q): every WorkgroupSize dimension must be at least 1, not %v", desc.Label, size))
	}
	// Vulkan dispatches with the size in the shader; Metal with the stated one. They
	// must agree, or the pipeline would behave differently on each.
	declared, fixed, err := localSize(desc.Shader, desc.Entry)
	if err != nil {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q): %v", desc.Label, err))
	}
	if fixed && declared != desc.WorkgroupSize {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q): WorkgroupSize %v is not the shader's local size %v",
			desc.Label, desc.WorkgroupSize, declared))
	}
	constants, err := resolveConstants(desc.Constants, desc.Shader)
	if err != nil {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q): %v", desc.Label, err))
	}
	var pin runtime.Pinner
	defer pin.Unpin()
	module, r := b.shaderModule(&pin, desc.Shader)
	if r != vkSuccess {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q) failed (%d)", desc.Label, int(r)))
	}
	defer vkDestroyShaderModule(b.device, module, nil)

	ci := vkComputePipelineCreateInfo{
		sType:  vkStructureTypeComputePipelineCreateInfo,
		layout: b.pipelineLayout,
		stage: vkPipelineShaderStageCreateInfo{
			sType:               vkStructureTypePipelineShaderStageCreateInfo,
			stage:               vkShaderStageComputeBit,
			module:              module,
			pName:               pinnedCString(&pin, desc.Entry),
			pSpecializationInfo: constants.info(&pin),
		},
	}
	var p vkPipeline
	if r := vkCreateComputePipelines(b.device, nil, 1, &ci, nil, &p); r != vkSuccess {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q) failed (%d)", desc.Label, int(r)))
	}
	return b.registerPipeline(p, vkPipelineBindPointCompute)
}

// GraphicsPipeline compiles a graphics pipeline (dynamic rendering), sharing the
// global bindless pipeline layout. It is a minimal one: empty vertex input (geometry is
// pulled via BDA), dynamic viewport/scissor/depth bias, dynamic rendering (formats via
// VkPipelineRenderingCreateInfo, no render pass). Depth is enabled iff the depth
// format is defined.
//
// The fragment shader may be absent, for a pipeline that writes depth and nothing else.
// That is a real configuration rather than a shortcut: a fragment stage bound to a pass
// with no colour attachment still runs, and one bound to a pass that HAS an attachment
// while writing nothing to it produces undefined colour rather than no colour.
func (b *Backend) CreateGraphicsPipeline(d gpu.PipelineDescriptor) gpu.Pipeline {
	entry := d.VertexEntry
	if entry == "" {
		entry = "main"
	}
	constants, err := resolveConstants(d.Constants, d.VertexShader, d.FragmentShader)
	if err != nil {
		panic(fmt.Sprintf("vulkan: CreateGraphicsPipeline(%q): %v", d.Label, err))
	}

	var pin runtime.Pinner
	defer pin.Unpin()
	fail := func(r vkResult) {
		panic(fmt.Sprintf("vulkan: CreateGraphicsPipeline(%q) failed (%d)", d.Label, int(r)))
	}
	vertex, r := b.shaderModule(&pin, d.VertexShader)
	if r != vkSuccess {
		fail(r)
	}
	defer vkDestroyShaderModule(b.device, vertex, nil)
	// Both stages share one set of constants; a stage ignores any it does not declare.
	name := pinnedCString(&pin, entry)
	spec := constants.info(&pin)
	stages := pinned(&pin, &[2]vkPipelineShaderStageCreateInfo{{
		sType:               vkStructureTypePipelineShaderStageCreateInfo,
		stage:               vkShaderStageVertexBit,
		module:              vertex,
		pName:               name,
		pSpecializationInfo: spec,
	}})
	nStages := 1
	if len(d.FragmentShader) > 0 {
		fragment, r := b.shaderModule(&pin, d.FragmentShader)
		if r != vkSuccess {
			fail(r)
		}
		defer vkDestroyShaderModule(b.device, fragment, nil)
		stages[1] = vkPipelineShaderStageCreateInfo{
			sType:               vkStructureTypePipelineShaderStageCreateInfo,
			stage:               vkShaderStageFragmentBit,
			module:              fragment,
			pName:               name,
			pSpecializationInfo: spec,
		}
		nStages = 2
	}

	samples := d.Samples
	if samples < 2 {
		samples = 1
	}
	// One blend state and format per colour attachment; a target the descriptor gives
	// no blend state is opaque.
	var blendsPtr *vkPipelineColorBlendAttachmentState
	var formatsPtr *vkFormat
	if n := len(d.ColorFormats); n > 0 {
		blends := make([]vkPipelineColorBlendAttachmentState, n)
		formats := make([]vkFormat, n)
		for i, f := range d.ColorFormats {
			var blend gpu.BlendState
			if i < len(d.Blend) {
				blend = d.Blend[i]
			}
			blends[i] = blendAttachment(blend)
			formats[i] = formatOf(f)
		}
		blendsPtr = pinned(&pin, &blends[0])
		formatsPtr = pinned(&pin, &formats[0])
	}
	frontFace := vkFrontFace(vkFrontFaceCounterClockwise)
	if d.FrontFaceCW {
		frontFace = vkFrontFaceClockwise
	}
	dynamic := pinned(&pin, &[3]vkDynamicState{vkDynamicStateViewport, vkDynamicStateScissor, vkDynamicStateDepthBias})

	ci := vkGraphicsPipelineCreateInfo{
		sType: vkStructureTypeGraphicsPipelineCreateInfo,
		pNext: unsafe.Pointer(pinned(&pin, &vkPipelineRenderingCreateInfo{ // dynamic rendering
			sType:                   vkStructureTypePipelineRenderingCreateInfo,
			colorAttachmentCount:    cUint32(len(d.ColorFormats)),
			pColorAttachmentFormats: formatsPtr,
			depthAttachmentFormat:   formatOf(d.DepthFormat),
		})),
		stageCount: cUint32(nStages),
		pStages:    &stages[0],
		pVertexInputState: pinned(&pin, &vkPipelineVertexInputStateCreateInfo{
			sType: vkStructureTypePipelineVertexInputStateCreateInfo,
		}),
		pInputAssemblyState: pinned(&pin, &vkPipelineInputAssemblyStateCreateInfo{
			sType:    vkStructureTypePipelineInputAssemblyStateCreateInfo,
			topology: topology(d.Topology),
		}),
		pViewportState: pinned(&pin, &vkPipelineViewportStateCreateInfo{
			sType:         vkStructureTypePipelineViewportStateCreateInfo,
			viewportCount: 1, // dynamic
			scissorCount:  1,
		}),
		pRasterizationState: pinned(&pin, &vkPipelineRasterizationStateCreateInfo{
			sType:       vkStructureTypePipelineRasterizationStateCreateInfo,
			polygonMode: vkPolygonModeFill,
			cullMode:    cullMode(d.CullMode),
			frontFace:   frontFace,
			lineWidth:   1,
			// Depth bias is always enabled and its factors are dynamic (SetDepthBias);
			// a pipeline that never calls it just runs with the all-zero default,
			// equivalent to bias disabled.
			depthBiasEnable: vkTrue,
		}),
		pMultisampleState: pinned(&pin, &vkPipelineMultisampleStateCreateInfo{
			sType:                vkStructureTypePipelineMultisampleStateCreateInfo,
			rasterizationSamples: vkSampleCountFlagBits(samples),
		}),
		pDepthStencilState: pinned(&pin, &vkPipelineDepthStencilStateCreateInfo{
			sType:            vkStructureTypePipelineDepthStencilStateCreateInfo,
			depthTestEnable:  bool32(d.DepthTest),
			depthWriteEnable: bool32(d.DepthWrite),
			depthCompareOp:   compareOp(d.DepthCompare),
		}),
		pColorBlendState: pinned(&pin, &vkPipelineColorBlendStateCreateInfo{
			sType:           vkStructureTypePipelineColorBlendStateCreateInfo,
			attachmentCount: cUint32(len(d.ColorFormats)),
			pAttachments:    blendsPtr,
		}),
		pDynamicState: pinned(&pin, &vkPipelineDynamicStateCreateInfo{
			sType:             vkStructureTypePipelineDynamicStateCreateInfo,
			dynamicStateCount: cUint32(len(dynamic)),
			pDynamicStates:    &dynamic[0],
		}),
		layout: b.pipelineLayout,
	}
	var p vkPipeline
	if r := vkCreateGraphicsPipelines(b.device, nil, 1, &ci, nil, &p); r != vkSuccess {
		fail(r)
	}
	return b.registerPipeline(p, vkPipelineBindPointGraphics)
}

// specializationConstants is a pipeline's constants as Vulkan takes them: ids[i]
// takes the 32 bits bits[i]. Vulkan reads every constant as 4 bytes whatever its type,
// so the type is not passed on.
type specializationConstants struct {
	ids  []cUint32
	bits []cUint32
}

// resolveConstants resolves a descriptor's constants against what the shaders declare.
// A constant only one stage declares is passed to both, and the other ignores it.
func resolveConstants(constants map[string]float64, shaders ...[]byte) (specializationConstants, error) {
	var resolved specializationConstants
	if len(constants) == 0 {
		return resolved, nil
	}
	var stages [][]specialization.Declaration
	for _, code := range shaders {
		if len(code) == 0 {
			continue
		}
		declared, err := specConstants(code)
		if err != nil {
			return resolved, err
		}
		stages = append(stages, declared)
	}
	values, err := specialization.Resolve(constants, stages...)
	if err != nil {
		return resolved, err
	}
	for _, v := range values {
		resolved.ids = append(resolved.ids, cUint32(v.ID))
		resolved.bits = append(resolved.bits, cUint32(v.Bits))
	}
	return resolved, nil
}

// info describes the constants to Vulkan, in memory pinned by pin: constant ids[i]
// takes the 32 bits bits[i]. Every constant is 4 bytes, VkBool32 included.
func (s specializationConstants) info(pin *runtime.Pinner) *vkSpecializationInfo {
	info := pinned(pin, &vkSpecializationInfo{})
	if len(s.ids) == 0 {
		return info
	}
	entries := make([]vkSpecializationMapEntry, len(s.ids))
	for i, id := range s.ids {
		entries[i] = vkSpecializationMapEntry{constantID: id, offset: cUint32(i * 4), size: 4}
	}
	info.mapEntryCount = cUint32(len(entries))
	info.pMapEntries = pinned(pin, &entries[0])
	info.dataSize = cSize(len(s.bits) * 4)
	info.pData = unsafe.Pointer(pinned(pin, &s.bits[0]))
	return info
}

// blendAttachment translates a target's blend state. WriteMask 0 means every channel,
// whether or not blending is enabled.
func blendAttachment(b gpu.BlendState) vkPipelineColorBlendAttachmentState {
	mask := vkColorComponentFlags(b.WriteMask & 0xf)
	if mask == 0 {
		mask = vkColorComponentRBit | vkColorComponentGBit | vkColorComponentBBit | vkColorComponentABit
	}
	state := vkPipelineColorBlendAttachmentState{colorWriteMask: mask}
	if !b.Enable {
		return state
	}
	state.blendEnable = vkTrue
	state.srcColorBlendFactor = blendFactor(b.ColorOp.Src)
	state.dstColorBlendFactor = blendFactor(b.ColorOp.Dst)
	state.colorBlendOp = blendOp(b.ColorOp.Op)
	state.srcAlphaBlendFactor = blendFactor(b.AlphaOp.Src)
	state.dstAlphaBlendFactor = blendFactor(b.AlphaOp.Dst)
	state.alphaBlendOp = blendOp(b.AlphaOp.Op)
	return state
}

func blendFactor(f gpu.BlendFactor) vkBlendFactor {
	switch f {
	case gpu.BlendOne:
		return vkBlendFactorOne
	case gpu.BlendSrcAlpha:
		return vkBlendFactorSrcAlpha
	case gpu.BlendOneMinusSrcAlpha:
		return vkBlendFactorOneMinusSrcAlpha
	case gpu.BlendDstAlpha:
		return vkBlendFactorDstAlpha
	case gpu.BlendOneMinusDstAlpha:
		return vkBlendFactorOneMinusDstAlpha
	default:
		return vkBlendFactorZero
	}
}

func blendOp(op gpu.BlendOp) vkBlendOp {
	switch op {
	case gpu.BlendSubtract:
		return vkBlendOpSubtract
	case gpu.BlendReverseSubtract:
		return vkBlendOpReverseSubtract
	case gpu.BlendMin:
		return vkBlendOpMin
	case gpu.BlendMax:
		return vkBlendOpMax
	default:
		return vkBlendOpAdd
	}
}

func (b *Backend) registerPipeline(p vkPipeline, bindPoint vkPipelineBindPoint) gpu.Pipeline {
	object := &pipeline{pipe: p, bindPoint: bindPoint}
	object.reference = reference{backend: b}
	return object
}

// DestroyPipeline releases a pipeline.
func (b *Backend) DestroyPipeline(p gpu.Pipeline) {
	resourceOf[*pipeline](b, p).destroy()
}
