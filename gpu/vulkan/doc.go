// Package vulkan is the Vulkan 1.3 backend for gamekit's gpu package. It uses thin, targeted
// cgo bindings (not a full Vulkan binding) — only the entry points the bindless,
// GPU-driven gpu needs.
//
// On macOS the backend is opt-in: it is compiled only with -tags gamekit_vulkan, and
// then runs on the KosmicKrisp ICD from the Vulkan SDK. Without the tag, importing
// this package registers nothing and Metal is the only backend, so building for macOS
// needs no Vulkan SDK.
package vulkan
