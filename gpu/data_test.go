package gpu_test

import "testing"

// TestMaxDataSizeIsUsable: every backend must report a limit big enough to be worth
// having. The Vulkan spec floor is 128 bytes, so anything below that means the query
// failed rather than found a small device.
func TestMaxDataSizeIsUsable(t *testing.T) {
	b := testBackend(t)
	got := b.GetMaxDataSize()
	if got < 128 {
		t.Fatalf("GetMaxDataSize = %d, below Vulkan's guaranteed 128-byte minimum — "+
			"the limit was probably not queried", got)
	}
	t.Logf("GetMaxDataSize = %d", got)
}
