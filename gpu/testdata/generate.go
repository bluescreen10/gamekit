//go:build ignore

// Generate the Vulkan and Metal fixtures from the same GLSL sources.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func main() {
	dir := flag.String("dir", ".", "shader source directory")
	flag.Parse()

	if err := generate(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(dir string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("shader generation requires macOS and Apple's Metal toolchain")
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	sources, err := filepath.Glob(filepath.Join(dir, "*.glsl"))
	if err != nil {
		return err
	}
	workdir, err := os.MkdirTemp("", "gamekit-gpu-shaders-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workdir)

	var artifacts []string
	for _, source := range sources {
		base := strings.TrimSuffix(filepath.Base(source), ".glsl")
		switch filepath.Ext(base) {
		case ".vert", ".frag", ".comp":
		default:
			continue // Includes such as common.glsl are not shader entry points.
		}

		fmt.Printf("generate %s.glsl\n", base)
		if err := buildShader(base, dir, workdir); err != nil {
			return fmt.Errorf("%s: %w", base, err)
		}
		artifacts = append(artifacts, base+".spv", base+".metalbin")
	}
	if len(artifacts) == 0 {
		return fmt.Errorf("no shader sources found in %s", dir)
	}

	// Publish only after every shader compiles. A compiler failure leaves the old
	// fixtures intact. MSL and the translation's binding-remapped SPIR-V stay temporary.
	for _, name := range artifacts {
		data, err := os.ReadFile(filepath.Join(workdir, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			return err
		}
	}

	return nil
}

func buildShader(base, dir, workdir string) error {
	stage := strings.TrimPrefix(filepath.Ext(base), ".")
	spirv := filepath.Join(workdir, base+".spv")
	if err := run(dir, "glslangValidator", "-V", "--target-env", "vulkan1.3", "-S", stage, "-o", spirv, base+".glsl"); err != nil {
		return err
	}
	if err := run(dir, "spirv-val", "--target-env", "vulkan1.3", "--scalar-block-layout", spirv); err != nil {
		return err
	}

	data, err := os.ReadFile(spirv)
	if err != nil {
		return err
	}
	data, err = metalBindings(data)
	if err != nil {
		return err
	}
	metalSPIRV := filepath.Join(workdir, base+".metal.spv")
	if err := os.WriteFile(metalSPIRV, data, 0644); err != nil {
		return err
	}

	msl := filepath.Join(workdir, base+".metal")
	args := []string{
		metalSPIRV, "--msl", "--msl-version", "30000", "--msl-argument-buffers",
		"--msl-argument-buffer-tier", "1", "--msl-device-argument-buffer", "1",
		"--msl-device-argument-buffer", "2", "--msl-decoration-binding", "--output", msl,
	}
	if stage == "vert" {
		args = append(args, "--flip-vert-y")
	}
	if err := run(dir, "spirv-cross", args...); err != nil {
		return err
	}

	data, err = os.ReadFile(msl)
	if err != nil {
		return err
	}
	// SPIRV-Cross reserves buffer slots for descriptor sets before the push constant.
	// The RHI always supplies the push constant at Metal buffer 0 instead.
	root := regexp.MustCompile(`(constant \w+& \w+ \[\[buffer\()\d+(\)\]\])`)
	data = root.ReplaceAll(data, []byte(`${1}0${2}`))
	if err := os.WriteFile(msl, data, 0644); err != nil {
		return err
	}

	output := filepath.Join(workdir, base+".metalbin")
	if err := run(dir, "xcrun", "-sdk", "macosx", "metal", "-std=metal3.0", "-mmacosx-version-min=26.0", msl, "-o", output); err != nil {
		return err
	}
	data, err = os.ReadFile(output)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(data, []byte("MTLB")) {
		return fmt.Errorf("Metal compiler did not produce a compiled library")
	}

	return nil
}

// metalBindings maps Vulkan's sampled/storage/sampler bindings to Metal's two
// argument tables. It changes only descriptor decorations, not shader instructions.
// The Vulkan artifact retains the original decorations.
func metalBindings(data []byte) ([]byte, error) {
	const (
		magic             = 0x07230203
		opDecorate        = 71
		bindingDecoration = 33
		setDecoration     = 34
	)
	if len(data) < 20 || len(data)%4 != 0 || binary.LittleEndian.Uint32(data) != magic {
		return nil, fmt.Errorf("invalid SPIR-V header")
	}

	type decorations struct {
		set, binding int // Byte offsets of the decoration values; zero means absent.
	}
	resources := make(map[uint32]decorations)
	for offset := 20; offset < len(data); {
		instruction := binary.LittleEndian.Uint32(data[offset:])
		size := int(instruction>>16) * 4
		if size == 0 || size > len(data)-offset {
			return nil, fmt.Errorf("invalid SPIR-V instruction at byte %d", offset)
		}
		if instruction&0xffff == opDecorate && size == 16 {
			id := binary.LittleEndian.Uint32(data[offset+4:])
			r := resources[id]
			switch binary.LittleEndian.Uint32(data[offset+8:]) {
			case bindingDecoration:
				r.binding = offset + 12
			case setDecoration:
				r.set = offset + 12
			}
			resources[id] = r
		}
		offset += size
	}

	data = bytes.Clone(data)
	for _, resource := range resources {
		if resource.set == 0 && resource.binding == 0 {
			continue
		}
		if resource.set == 0 || resource.binding == 0 || binary.LittleEndian.Uint32(data[resource.set:]) != 0 {
			return nil, fmt.Errorf("expected a resource in descriptor set 0 with a binding")
		}

		var set uint32
		switch binding := binary.LittleEndian.Uint32(data[resource.binding:]); binding {
		case 0, 1: // Sampled and storage textures share Metal's texture table.
			set = 1
		case 2:
			set = 2
		default:
			return nil, fmt.Errorf("unsupported descriptor binding %d", binding)
		}
		binary.LittleEndian.PutUint32(data[resource.set:], set)
		binary.LittleEndian.PutUint32(data[resource.binding:], 0)
	}

	return data, nil
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w\n%s", name, err, output)
	}

	return nil
}
