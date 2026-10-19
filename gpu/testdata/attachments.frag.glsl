#version 460

layout(location = 0) out vec4 first;
layout(location = 1) out vec4 second;

void main() {
    first = vec4(0.5, 0.25, 0, 0.5);
    second = vec4(0, 0.5, 1, 0.5);
}
