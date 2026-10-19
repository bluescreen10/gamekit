package gpu_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// allocation checks the public buffer contract and frees the buffer after the test.
// Tests must wait for their submissions before they return.
func allocation(t *testing.T, b gpu.Backend, size uint64, memory gpu.MemoryType) gpu.Buffer {
	t.Helper()
	buf := b.Alloc(size, memory, t.Name())
	t.Cleanup(func() {
		b.Free(buf)
	})

	if !buf.IsValid() || buf.Addr == 0 || buf.Size != size {
		t.Fatalf("Alloc(%d, %v) = %+v; want a valid buffer, a device address, and Size %d", size, memory, buf, size)
	}
	if (buf.Ptr != nil) != (memory == gpu.MemoryHost) {
		t.Fatalf("Alloc(%d, %v).Ptr = %p; only host buffers must have a mapped pointer", size, memory, buf.Ptr)
	}

	return buf
}

func TestBufferAllocation(t *testing.T) {
	b := testBackend(t)
	for _, memory := range []gpu.MemoryType{gpu.MemoryHost, gpu.MemoryDevice} {
		for _, size := range []uint64{4, 64, 260, 4096} {
			t.Run(fmt.Sprintf("memory=%d/size=%d", memory, size), func(t *testing.T) {
				allocation(t, b, size, memory)
			})
		}
	}
}

func TestBufferCopyRanges(t *testing.T) {
	b := testBackend(t)
	for _, size := range []uint64{4, 12, 252, 4096} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			src := allocation(t, b, size+16, gpu.MemoryHost)
			device := allocation(t, b, size+32, gpu.MemoryDevice)
			dst := allocation(t, b, size+40, gpu.MemoryHost)
			source := make([]byte, src.Size)
			for i := range source {
				source[i] = byte(i*37 + 11)
			}
			src.Write(source, 0)
			want := bytes.Repeat([]byte{0xa5}, int(dst.Size))
			dst.Write(want, 0)
			copy(want[20:20+size], source[8:8+size])

			cmd := b.Begin()
			cmd.CopyBuffer(device, src, 16, 8, size)
			cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
			cmd.CopyBuffer(dst, device, 20, 16, size)
			b.Wait(b.Submit(cmd))

			got := unsafe.Slice((*byte)(dst.Ptr), dst.Size)
			if !bytes.Equal(got, want) {
				t.Fatalf("copy through device memory = %v, want %v (including unchanged guards)", got, want)
			}
		})
	}
}

func TestBufferCopyWithinAllocation(t *testing.T) {
	b := testBackend(t)
	buf := allocation(t, b, 64, gpu.MemoryHost)
	want := make([]byte, buf.Size)
	for i := range want {
		want[i] = byte(i + 1)
	}
	buf.Write(want, 0)
	copy(want[40:56], want[4:20])

	cmd := b.Begin()
	cmd.CopyBuffer(buf, buf, 40, 4, 16)
	b.Wait(b.Submit(cmd))

	if got := unsafe.Slice((*byte)(buf.Ptr), buf.Size); !bytes.Equal(got, want) {
		t.Fatalf("non-overlapping copy within one buffer = %v, want %v", got, want)
	}
}

func TestBufferWriteBounds(t *testing.T) {
	b := testBackend(t)
	buf := allocation(t, b, 8, gpu.MemoryHost)
	for _, offset := range []uint64{7, 8, 9, math.MaxUint64} {
		t.Run(fmt.Sprintf("offset=%d", offset), func(t *testing.T) {
			buf.Write(bytes.Repeat([]byte{0xa5}, 8), 0)
			if offset == 7 {
				buf.Write([]byte{42}, offset)
				if got := *(*byte)(unsafe.Add(buf.Ptr, 7)); got != 42 {
					t.Fatalf("last byte = %d, want 42", got)
				}
				return
			}

			mustPanic(t, func() {
				buf.Write([]byte{42}, offset)
			})
			if got := unsafe.Slice((*byte)(buf.Ptr), 8); !bytes.Equal(got, bytes.Repeat([]byte{0xa5}, 8)) {
				t.Fatalf("failed Write changed memory to %v", got)
			}
		})
	}
}

func mustPanic(t *testing.T, call func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("call did not panic")
		}
	}()

	call()
}

func TestResourcesRemainIndependentAfterFree(t *testing.T) {
	b := testBackend(t)
	const count = 12
	var survivors []gpu.Buffer
	for i := range count {
		buf := b.Alloc(64, gpu.MemoryHost, "temporary")
		buf.Write(bytes.Repeat([]byte{byte(i + 1)}, 64), 0)
		if i%2 == 0 {
			b.Free(buf)
		} else {
			survivors = append(survivors, buf)
			t.Cleanup(func() {
				b.Free(buf)
			})
		}
	}

	for range count {
		buf := allocation(t, b, 64, gpu.MemoryHost)
		buf.Write(bytes.Repeat([]byte{0xff}, 64), 0)
	}

	readback := allocation(t, b, 64*uint64(len(survivors)), gpu.MemoryHost)
	cmd := b.Begin()
	for i, buf := range survivors {
		cmd.CopyBuffer(readback, buf, uint64(i*64), 0, 64)
	}
	b.Wait(b.Submit(cmd))

	for i := range survivors {
		got := unsafe.Slice((*byte)(unsafe.Add(readback.Ptr, i*64)), 64)
		want := bytes.Repeat([]byte{byte(2*i + 2)}, 64)
		if !bytes.Equal(got, want) {
			t.Fatalf("surviving buffer %d = %v, want %v", i, got, want)
		}
	}
}
