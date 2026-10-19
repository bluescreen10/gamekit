package gpu_test

// Backend-agnostic RHI conformance tests. They exercise the gpu.Backend interface
// (buffers, pipelines, dynamic rendering, compute→indirect, bindless textures) and
// so validate ANY backend, not just Vulkan. The backend is obtained from the
// registry and the suite skips when none is registered/initializable — so
// `go test ./...` passes on every platform, running the tests only where a backend
// exists. gpu/default registers the platform backend through one shared import.

import (
	"embed"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	_ "github.com/bluescreen10/gamekit/gpu/default"
)

//go:generate go run ./testdata/generate.go -dir ./testdata

// Test shaders are fixtures under gpu/testdata, embedded here (a _test.go file) so
// they never enter the production shaders package. Metal fixtures are compiled
// libraries generated from the corresponding shader sources.
var (
	//go:embed testdata/*.metalbin
	metalFixtures embed.FS
	//go:embed testdata/triangle.vert.spv
	triangleVert []byte
	//go:embed testdata/triangle.frag.spv
	triangleFrag []byte
	//go:embed testdata/mesh.vert.spv
	meshVert []byte
	//go:embed testdata/mesh.frag.spv
	meshFrag []byte
	//go:embed testdata/instanced.comp.spv
	instancedComp []byte
	//go:embed testdata/instanced.vert.spv
	instancedVert []byte
	//go:embed testdata/fill_indirect.comp.spv
	fillIndirect []byte
	//go:embed testdata/textured.vert.spv
	texturedVert []byte
	//go:embed testdata/textured.frag.spv
	texturedFrag []byte
	//go:embed testdata/storage_write.comp.spv
	storageWrite []byte
	//go:embed testdata/storage_write3d.comp.spv
	storageWrite3D []byte
	//go:embed testdata/sampled3d.frag.spv
	sampled3DFrag []byte
	//go:embed testdata/read_samples.comp.spv
	readSamples []byte
	//go:embed testdata/tint.frag.spv
	tintFrag []byte
	//go:embed testdata/constants.comp.spv
	constantsComp []byte
	//go:embed testdata/constants.vert.spv
	constantsVert []byte
	//go:embed testdata/constants.frag.spv
	constantsFrag []byte
	//go:embed testdata/payload.comp.spv
	payloadComp []byte
	//go:embed testdata/shadow.frag.spv
	shadowFrag []byte
	//go:embed testdata/attachments.frag.spv
	attachmentsFrag []byte
)

var (
	backendOnce sync.Once
	backendInst gpu.Backend
	backendErr  error
)

// testBackend returns the registered gpu backend, initialized once and shared
// across the conformance tests. Automatic selection skips if no device exists.
// GAMEKIT_GPU_BACKEND selects an implementation explicitly and fails on init errors.
func testBackend(t *testing.T) gpu.Backend {
	t.Helper()
	name := os.Getenv("GAMEKIT_GPU_BACKEND")
	if !gpu.HasBackend() {
		if name != "" {
			t.Fatalf("requested backend %q is not registered", name)
		}

		t.Skip("no gpu backend registered for this platform")
	}

	backendOnce.Do(func() {
		if name == "" {
			backendInst = gpu.Instance(nil)
		} else {
			var ok bool
			backendInst, ok = gpu.Lookup(name)
			if !ok {
				backendErr = fmt.Errorf("backend %q is not registered", name)
				return
			}
		}

		if metal, ok := gpu.Lookup("metal"); ok && backendInst == metal {
			variant := func(name string) []byte {
				data, err := metalFixtures.ReadFile("testdata/" + name + ".metalbin")
				if err != nil {
					panic(err)
				}
				return data
			}
			triangleVert = variant("triangle.vert")
			triangleFrag = variant("triangle.frag")
			meshVert = variant("mesh.vert")
			meshFrag = variant("mesh.frag")
			instancedVert = variant("instanced.vert")
			instancedComp = variant("instanced.comp")
			fillIndirect = variant("fill_indirect.comp")
			texturedVert = variant("textured.vert")
			texturedFrag = variant("textured.frag")
			storageWrite = variant("storage_write.comp")
			storageWrite3D = variant("storage_write3d.comp")
			sampled3DFrag = variant("sampled3d.frag")
			readSamples = variant("read_samples.comp")
			tintFrag = variant("tint.frag")
			constantsComp = variant("constants.comp")
			constantsVert = variant("constants.vert")
			constantsFrag = variant("constants.frag")
			payloadComp = variant("payload.comp")
			shadowFrag = variant("shadow.frag")
			attachmentsFrag = variant("attachments.frag")
		}
		backendErr = backendInst.Init()
	})

	if backendErr != nil {
		if name != "" {
			t.Fatalf("requested backend init failed: %v", backendErr)
		}
		t.Skipf("gpu backend init failed (no device?): %v", backendErr)
	}

	return backendInst
}

func TestMain(m *testing.M) {
	code := m.Run()
	if backendInst != nil && backendErr == nil {
		backendInst.WaitIdle()
		backendInst.Destroy()
	}

	os.Exit(code)
}
