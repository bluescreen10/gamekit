#version 460
#extension GL_EXT_buffer_reference : require
#extension GL_EXT_scalar_block_layout : require
#extension GL_EXT_nonuniform_qualifier : require
#extension GL_EXT_shader_explicit_arithmetic_types_int64 : require

layout(set = 0, binding = 0) uniform texture2D depthTextures[];
layout(set = 0, binding = 2) uniform sampler comparisonSamplers[];

layout(buffer_reference, scalar) readonly buffer Root {
    uint64_t vertices;
    uint textureIndex;
    uint samplerIndex;
};
layout(push_constant) uniform PC { Root root; } pc;

layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 color;

void main() {
    float visible = texture(sampler2DShadow(
        depthTextures[pc.root.textureIndex], comparisonSamplers[pc.root.samplerIndex]), vec3(uv, 0.5));
    color = vec4(visible, visible, visible, 1);
}
