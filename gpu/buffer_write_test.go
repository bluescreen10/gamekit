package gpu_test

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// TestBufferWriteCopiesAtOffset covers the ordinary path: bytes land at the given
// offset in the mapped memory, leaving the rest untouched.
func TestBufferWriteCopiesAtOffset(t *testing.T) {
	b := testBackend(t)

	buf := b.Alloc(8, gpu.MemoryHost, "write-test")
	defer b.Free(buf)

	// Alloc does not zero memory: on lavapipe a fresh buffer held 52 in its last
	// byte. Fill it first so the bytes no Write touches have a known value.
	untouched := byte(0xEE)
	buf.Write(bytes.Repeat([]byte{untouched}, 8), 0)

	buf.Write([]byte{1, 2, 3, 4}, 0)
	buf.Write([]byte{5, 6}, 4)

	got := unsafe.Slice((*byte)(buf.Ptr()), buf.Size())
	want := []byte{1, 2, 3, 4, 5, 6, untouched, untouched}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buffer = %v, want %v", got, want)
		}
	}
}

// TestBufferWriteEmptyIsNoop: writing zero bytes must not panic even against a
// buffer with no mapped pointer, since there is nothing to bounds-check.
func TestBufferWriteEmptyIsNoop(t *testing.T) {
	b := testBackend(t)
	buf := b.Alloc(4, gpu.MemoryDevice, "unmapped empty write")
	defer b.Free(buf)
	buf.Write(nil, 0)
}

// TestBufferWriteOverrunPanics guards the segfault this method exists to replace: a
// write that would run past Size must panic instead of corrupting adjacent memory.
func TestBufferWriteOverrunPanics(t *testing.T) {
	b := testBackend(t)

	buf := b.Alloc(4, gpu.MemoryHost, "overrun-test")
	defer b.Free(buf)

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic writing past the buffer's Size")
		}
	}()
	buf.Write([]byte{1, 2, 3, 4, 5}, 0)
}

// TestBufferWriteOffsetOverrunPanics: an in-bounds length can still overrun once the
// offset is added; both terms of the bound must be checked together, not separately.
func TestBufferWriteOffsetOverrunPanics(t *testing.T) {
	b := testBackend(t)

	buf := b.Alloc(8, gpu.MemoryHost, "offset-overrun-test")
	defer b.Free(buf)

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic when offset+len(data) exceeds Size")
		}
	}()
	buf.Write([]byte{1, 2, 3}, 6)
}

// TestBufferWriteNoPtrPanics: a MemoryDevice buffer has no mapped
// pointer at all, so writing to it must panic rather than dereference nil.
func TestBufferWriteNoPtrPanics(t *testing.T) {
	b := testBackend(t)

	buf := b.Alloc(64, gpu.MemoryDevice, "device-only")
	defer b.Free(buf)
	if buf.Ptr() != nil {
		t.Fatal("MemoryDevice allocation unexpectedly has a mapped pointer")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic writing to a buffer with no mapped pointer")
		}
	}()
	buf.Write([]byte{1}, 0)
}
