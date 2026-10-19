#version 460
#extension GL_EXT_buffer_reference : require
#extension GL_EXT_scalar_block_layout : require

// Copy all 128 bytes of the portable push-constant range, except the output address.
layout(buffer_reference, scalar) writeonly buffer Output {
    uint words[];
};

layout(push_constant, scalar) uniform PC {
    Output outputBuffer;
    uint words[30];
} pc;

layout(local_size_x = 1) in;
void main() {
    for (uint i = 0; i < 30; i++) {
        pc.outputBuffer.words[i] = pc.words[i];
    }
}
