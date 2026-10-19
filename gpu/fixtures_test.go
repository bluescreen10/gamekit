package gpu_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetalFixturesAreCompiledLibraries(t *testing.T) {
	entries, err := metalFixtures.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := metalFixtures.ReadFile("testdata/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(data, []byte("MTLB")) {
				t.Fatal("fixture is not a compiled Metal library")
			}
		})
	}
}

func TestShaderFixturesUseOneGLSLSource(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		name := entry.Name()
		switch filepath.Ext(name) {
		case ".comp", ".vert", ".frag", ".metal", ".metallib":
			t.Errorf("unexpected shader variant %q; keep GLSL sources and generated .spv/.metalbin files", name)
		case ".spv", ".metalbin":
			source := strings.TrimSuffix(name, filepath.Ext(name)) + ".glsl"
			if _, err := os.Stat(filepath.Join("testdata", source)); err != nil {
				t.Errorf("artifact %q has no GLSL source: %v", name, err)
			}
		case ".glsl":
			base := strings.TrimSuffix(name, ".glsl")
			switch filepath.Ext(base) {
			case ".comp", ".vert", ".frag":
				for _, extension := range []string{".spv", ".metalbin"} {
					if _, err := os.Stat(filepath.Join("testdata", base+extension)); err != nil {
						t.Errorf("GLSL source %q has no %s artifact: %v", name, extension, err)
					}
				}
			}
		}
	}
}
