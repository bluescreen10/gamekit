package gpu_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

func computePipeline(t *testing.T, b gpu.Backend, desc gpu.ComputePipelineDescriptor) gpu.Pipeline {
	t.Helper()
	desc.Label = t.Name()
	p := b.CreateComputePipeline(desc)
	t.Cleanup(func() {
		b.DestroyPipeline(p)
	})

	if p == nil || !p.IsValid() {
		t.Fatal("CreateComputePipeline returned an invalid pipeline")
	}

	return p
}

func TestDispatchCopiesPerCallData(t *testing.T) {
	b := testBackend(t)
	p := computePipeline(t, b, gpu.ComputePipelineDescriptor{Shader: payloadComp, WorkgroupSize: [3]uint32{1, 1, 1}})
	// More than 64 calls also tests backends that grow a per-command data arena.
	const calls = 160
	out := allocation(t, b, calls*120, gpu.MemoryHost)
	clear(unsafe.Slice((*byte)(out.Ptr()), out.Size()))
	data := make([]byte, 128)

	cmd := b.Begin()
	cmd.SetPipeline(p)
	for call := range calls {
		binary.NativeEndian.PutUint64(data, out.Addr()+uint64(call*120))
		for word := range 30 {
			binary.NativeEndian.PutUint32(data[8+word*4:], uint32(call*100+word+1))
		}
		cmd.Dispatch(data, 1, 1, 1)
	}
	clear(data)
	b.Wait(b.Submit(cmd))

	got := unsafe.Slice((*uint32)(out.Ptr()), calls*30)
	for call := range calls {
		for word := range 30 {
			want := uint32(call*100 + word + 1)
			if value := got[call*30+word]; value != want {
				t.Fatalf("dispatch %d word %d = %d, want its own copied data %d", call, word, value, want)
			}
		}
	}
}

func TestDispatchAcceptsMaxDataSize(t *testing.T) {
	b := testBackend(t)
	p := computePipeline(t, b, gpu.ComputePipelineDescriptor{Shader: payloadComp, WorkgroupSize: [3]uint32{1, 1, 1}})
	out := allocation(t, b, 120, gpu.MemoryHost)
	data := bytes.Repeat([]byte{0x3c}, int(b.GetMaxDataSize()))
	if len(data) < 128 {
		t.Fatalf("GetMaxDataSize = %d, want at least 128", len(data))
	}
	binary.NativeEndian.PutUint64(data, out.Addr())

	cmd := b.Begin()
	cmd.SetPipeline(p)
	cmd.Dispatch(data, 1, 1, 1)
	clear(data)
	b.Wait(b.Submit(cmd))

	if got := unsafe.Slice((*byte)(out.Ptr()), 120); !bytes.Equal(got, bytes.Repeat([]byte{0x3c}, 120)) {
		t.Fatalf("maximum-sized dispatch data = %v, want 120 bytes of 0x3c", got)
	}
}

func TestComputePipelineSwitch(t *testing.T) {
	b := testBackend(t)
	first := computePipeline(t, b, gpu.ComputePipelineDescriptor{
		Shader: constantsComp, WorkgroupSize: [3]uint32{1, 1, 1}, Constants: map[string]float64{"COUNT": 11},
	})
	second := computePipeline(t, b, gpu.ComputePipelineDescriptor{
		Shader: constantsComp, WorkgroupSize: [3]uint32{1, 1, 1}, Constants: map[string]float64{"COUNT": 23},
	})
	out := allocation(t, b, 48, gpu.MemoryHost)
	cmd := b.Begin()
	for i, p := range []gpu.Pipeline{first, second, first} {
		cmd.SetPipeline(p)
		addr := out.Addr() + uint64(i*16)
		cmd.Dispatch(utils.ToBytes(&addr), 1, 1, 1)
	}
	b.Wait(b.Submit(cmd))

	for i, count := range []uint32{11, 23, 11} {
		got := *(*shaderConstants)(unsafe.Add(out.Ptr(), i*16))
		want := shaderConstants{offset: -3, count: count, scale: 0.5}
		if got != want {
			t.Fatalf("dispatch %d constants = %+v, want %+v", i, got, want)
		}
	}
}

func TestComputeBarrierOrdersBufferWrites(t *testing.T) {
	b := testBackend(t)
	p := computePipeline(t, b, gpu.ComputePipelineDescriptor{Shader: fillIndirect, WorkgroupSize: [3]uint32{1, 1, 1}})
	args := allocation(t, b, 20, gpu.MemoryHost)
	clear(unsafe.Slice((*byte)(args.Ptr()), args.Size()))
	root := allocation(t, b, 8, gpu.MemoryHost)
	*(*uint64)(root.Ptr()) = args.Addr()
	readback := allocation(t, b, 20, gpu.MemoryHost)

	cmd := b.Begin()
	cmd.SetPipeline(p)
	addr := root.Addr()
	for i := range 4 {
		if i != 0 {
			cmd.Barrier(gpu.StageCompute, gpu.StageCompute, 0)
		}
		cmd.Dispatch(utils.ToBytes(&addr), 1, 1, 1)
	}
	cmd.Barrier(gpu.StageCompute, gpu.StageTransfer, 0)
	cmd.CopyBuffer(readback, args, 0, 0, 20)
	b.Wait(b.Submit(cmd))

	got := *(*drawIndexedIndirect)(readback.Ptr())
	want := drawIndexedIndirect{indexCount: 3, instanceCount: 4}
	if got != want {
		t.Fatalf("compute -> compute -> transfer = %+v, want %+v", got, want)
	}
}

func TestSubmissionCompletion(t *testing.T) {
	b := testBackend(t)
	for _, completion := range []string{"fence", "idle"} {
		t.Run(completion, func(t *testing.T) {
			upload := allocation(t, b, 64, gpu.MemoryHost)
			device := allocation(t, b, 64, gpu.MemoryDevice)
			readback := allocation(t, b, 64, gpu.MemoryHost)
			for round := range 3 {
				want := bytes.Repeat([]byte{byte(round + 41)}, 64)
				upload.Write(want, 0)
				first := b.Begin()
				first.CopyBuffer(device, upload, 0, 0, 64)
				fence := b.Submit(first)
				if completion == "fence" {
					b.Wait(fence)
				} else {
					b.WaitIdle()
					b.Wait(fence)
				}

				second := b.Begin()
				second.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
				second.CopyBuffer(readback, device, 0, 0, 64)
				b.Wait(b.Submit(second))
				if got := unsafe.Slice((*byte)(readback.Ptr()), 64); !bytes.Equal(got, want) {
					t.Fatalf("round %d readback = %v, want %v", round, got, want)
				}
			}
		})
	}
}

func TestEmptySubmission(t *testing.T) {
	b := testBackend(t)
	b.Wait(b.Submit(b.Begin()))
	b.WaitIdle()
}

func TestTimestampPoolReuse(t *testing.T) {
	b := testBackend(t)
	pool := b.CreateTimestampPool(4)
	if pool == nil || !pool.IsValid() {
		t.Skip("device does not support timestamp pools")
	}
	defer b.DestroyTimestampPool(pool)
	period := b.TimestampPeriod()
	if period <= 0 || math.IsNaN(period) || math.IsInf(period, 0) {
		t.Fatalf("TimestampPeriod = %v, want a finite positive nanoseconds-per-tick scale", period)
	}
	if bits, ok := b.(interface{ TimestampValidBits() uint32 }); ok && bits.TimestampValidBits() == 0 {
		t.Skip("queue does not support timestamps")
	}

	src := allocation(t, b, 4096, gpu.MemoryHost)
	dst := allocation(t, b, 4096, gpu.MemoryDevice)
	src.Write(bytes.Repeat([]byte{37}, 4096), 0)
	for round := range 3 {
		t.Run(fmt.Sprintf("round=%d", round), func(t *testing.T) {
			cmd := b.Begin()
			cmd.ResetTimestamps(pool, 4)
			cmd.WriteTimestamp(pool, 0, gpu.StageNone)
			for i := range uint32(3) {
				cmd.CopyBuffer(dst, src, uint64(i*1024), uint64(i*1024), 1024)
				cmd.WriteTimestamp(pool, i+1, gpu.StageAll)
			}
			b.Wait(b.Submit(cmd))
			values := b.ReadTimestamps(pool, 4)
			if len(values) != 4 {
				t.Fatalf("ReadTimestamps returned %d values, want 4", len(values))
			}
			for i, value := range values {
				if value == 0 {
					t.Errorf("timestamp %d is missing", i)
				}
				if i > 0 && value < values[i-1] {
					t.Errorf("timestamps are not ordered: %v", values)
				}
			}
		})
	}
}
