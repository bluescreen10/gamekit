package vulkan

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/bluescreen10/gamekit/gpu/internal/specialization"
)

// SPIR-V opcodes, decorations and execution modes the parsers read.
const (
	opName              = 5
	opEntryPoint        = 15
	opExecutionMode     = 16
	opTypeBool          = 20
	opTypeInt           = 21
	opTypeFloat         = 22
	opSpecConstantTrue  = 48
	opSpecConstantFalse = 49
	opSpecConstant      = 50
	opDecorate          = 71

	decorationSpecID  = 1
	decorationBuiltIn = 11

	builtInWorkgroupSize = 25
	executionModeLocal   = 17 // LocalSize
)

// specConstants lists the specialization constants a SPIR-V module declares: each
// OpSpecConstant* decorated with a SpecId, its type, and its name where the module kept
// its debug names.
func specConstants(code []byte) ([]specialization.Declaration, error) {
	words, err := moduleWords(code)
	if err != nil {
		return nil, err
	}
	names := map[uint32]string{}
	specIDs := map[uint32]uint32{}
	types := map[uint32]specialization.Type{}
	var constants []specialization.Declaration
	err = eachInstruction(words, func(op uint32, operands []uint32) {
		switch op {
		case opName:
			names[operands[0]] = literalString(operands[1:])
		case opDecorate:
			if operands[1] == decorationSpecID {
				specIDs[operands[0]] = operands[2]
			}
		case opTypeBool:
			types[operands[0]] = specialization.Bool
		case opTypeInt:
			types[operands[0]] = specialization.Uint
			if operands[2] != 0 {
				types[operands[0]] = specialization.Int
			}
		case opTypeFloat:
			types[operands[0]] = specialization.Float
		case opSpecConstantTrue, opSpecConstantFalse, opSpecConstant:
			constants = append(constants, specialization.Declaration{ID: operands[1], Type: types[operands[0]]})
		}
	})
	if err != nil {
		return nil, err
	}

	// A spec constant's SpecId and name are decorations of its result id, which the
	// list above held in place of an ID until every decoration had been read.
	var declared []specialization.Declaration
	for _, c := range constants {
		resultID := c.ID
		specID, ok := specIDs[resultID]
		if !ok {
			continue
		}
		declared = append(declared, specialization.Declaration{ID: specID, Name: names[resultID], Type: c.Type})
	}
	return declared, nil
}

// localSize returns the workgroup size the compute entry point named entry declares.
// It reports false when the size is not fixed in the module: when a WorkgroupSize
// built-in sets it, which a shader whose size is a specialization constant uses, or
// when the entry declares none.
func localSize(code []byte, entry string) ([3]uint32, bool, error) {
	words, err := moduleWords(code)
	if err != nil {
		return [3]uint32{}, false, err
	}
	entryIDs := map[uint32]bool{}
	sizes := map[uint32][3]uint32{}
	builtIn := false
	err = eachInstruction(words, func(op uint32, operands []uint32) {
		switch op {
		case opEntryPoint:
			if literalString(operands[2:]) == entry {
				entryIDs[operands[1]] = true
			}
		case opExecutionMode:
			if operands[1] == executionModeLocal && len(operands) >= 5 {
				sizes[operands[0]] = [3]uint32{operands[2], operands[3], operands[4]}
			}
		case opDecorate:
			if operands[1] == decorationBuiltIn && len(operands) >= 3 && operands[2] == builtInWorkgroupSize {
				builtIn = true
			}
		}
	})
	if err != nil || builtIn {
		return [3]uint32{}, false, err
	}
	for id := range entryIDs {
		if size, ok := sizes[id]; ok {
			return size, true, nil
		}
	}
	return [3]uint32{}, false, nil
}

// moduleWords returns a SPIR-V module's words.
func moduleWords(code []byte) ([]uint32, error) {
	if len(code)%4 != 0 || len(code) < 20 {
		return nil, fmt.Errorf("SPIR-V is %d bytes, not a whole module", len(code))
	}
	words := make([]uint32, len(code)/4)
	binary.Read(bytes.NewReader(code), binary.LittleEndian, words)
	return words, nil
}

// eachInstruction calls visit with the opcode and operands of each instruction after
// the module header.
func eachInstruction(words []uint32, visit func(op uint32, operands []uint32)) error {
	for i := 5; i < len(words); {
		count := int(words[i] >> 16)
		if count == 0 || i+count > len(words) {
			return fmt.Errorf("SPIR-V instruction at word %d overruns the module", i)
		}
		visit(words[i]&0xffff, words[i+1:i+count])
		i += count
	}
	return nil
}

// literalString decodes a SPIR-V literal string: UTF-8 packed into words, ending at
// its first NUL.
func literalString(words []uint32) string {
	data := make([]byte, 4*len(words))
	for i, w := range words {
		binary.LittleEndian.PutUint32(data[4*i:], w)
	}
	if end := bytes.IndexByte(data, 0); end >= 0 {
		data = data[:end]
	}
	return string(data)
}
