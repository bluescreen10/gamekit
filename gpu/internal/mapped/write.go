// Package mapped implements writes to persistently mapped GPU buffers.
package mapped

import (
	"fmt"
	"unsafe"
)

// Write copies data into ptr at offset. Empty writes do not need mapped memory.
func Write(ptr unsafe.Pointer, size uint64, data []byte, offset uint64) {
	if len(data) == 0 {
		return
	}
	if ptr == nil {
		panic("gpu: Buffer.Write: buffer has no mapped pointer")
	}
	if offset > size || uint64(len(data)) > size-offset {
		panic(fmt.Sprintf("gpu: Buffer.Write: write of %d bytes at offset %d overruns a %d-byte buffer", len(data), offset, size))
	}

	copy(unsafe.Slice((*byte)(ptr), size)[offset:], data)
}
