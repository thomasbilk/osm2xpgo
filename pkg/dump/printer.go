package dump

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// Dump parses a DSF binary file and prints its atom structure to stdout.
func Dump(filePath string) error {
	file, err := Parse(filePath)
	if err != nil {
		return err
	}

	fmt.Printf("DSF Version: %d\n", file.Version)
	for _, atom := range file.Atoms {
		printAtom(atom, 0)
	}
	return nil
}

// printAtom recursively prints an atom and its children with indentation.
func printAtom(atom ParsedAtom, level int) {
	indent := strings.Repeat("  ", level)
	idStr := IDString(atom.ID)
	fmt.Printf("%s%s (%d bytes)\n", indent, idStr, atom.Size)

	// Print payload details for special atoms.
	childIndent := strings.Repeat("  ", level+1)
	switch atom.ID {
	case dsf.AtomPROP:
		printPROP(atom.Payload, childIndent)
	case dsf.AtomTERT, dsf.AtomOBJT, dsf.AtomPOLY, dsf.AtomNETW, dsf.AtomDEMN:
		printDefinitions(atom.Payload, childIndent)
	case dsf.AtomPOOL, dsf.AtomPO32:
		printPointPool(atom.Payload, childIndent)
	case dsf.AtomCMDS:
		printCMDS(atom.Payload, childIndent)
	}

	// Recurse into children for container atoms.
	for _, child := range atom.Children {
		printAtom(child, level+1)
	}
}

// printPROP decodes null-terminated key-value pairs from a PROP payload.
func printPROP(payload []byte, indent string) {
	parts := splitNullTerminated(payload)
	// Key-value pairs come in sequential pairs.
	for i := 0; i+1 < len(parts); i += 2 {
		fmt.Printf("%s%s = %s\n", indent, parts[i], parts[i+1])
	}
}

// printDefinitions decodes a null-terminated string table and prints indexed entries.
func printDefinitions(payload []byte, indent string) {
	parts := splitNullTerminated(payload)
	for i, name := range parts {
		fmt.Printf("%s[%d] %s\n", indent, i, name)
	}
}

// printPointPool decodes the point pool header (planes + count) and prints dimensions.
func printPointPool(payload []byte, indent string) {
	if len(payload) < 5 {
		return
	}
	planes := payload[0]
	count := binary.LittleEndian.Uint32(payload[1:5])
	fmt.Printf("%s%d planes × %d points\n", indent, planes, count)
}

// printCMDS decodes command opcodes from a CMDS atom payload.
func printCMDS(payload []byte, indent string) {
	pos := 0
	for pos < len(payload) {
		opcode := payload[pos]
		pos++

		switch opcode {
		case dsf.CmdPoolSelect:
			if pos+2 > len(payload) {
				return
			}
			pool := binary.LittleEndian.Uint16(payload[pos : pos+2])
			pos += 2
			fmt.Printf("%sPOOL_SELECT pool=%d\n", indent, pool)

		case dsf.CmdJunctionOffset:
			if pos+2 > len(payload) {
				return
			}
			offset := binary.LittleEndian.Uint16(payload[pos : pos+2])
			pos += 2
			fmt.Printf("%sJUNCTION_OFFSET offset=%d\n", indent, offset)

		case dsf.CmdSetDefinition:
			if pos+2 > len(payload) {
				return
			}
			def := binary.LittleEndian.Uint16(payload[pos : pos+2])
			pos += 2
			fmt.Printf("%sSET_DEFINITION def=%d\n", indent, def)

		case dsf.CmdSetRoadSubType:
			if pos+1 > len(payload) {
				return
			}
			subtype := payload[pos]
			pos++
			fmt.Printf("%sSET_ROAD_SUBTYPE subtype=%d\n", indent, subtype)

		case dsf.CmdObject:
			if pos+2 > len(payload) {
				return
			}
			idx := binary.LittleEndian.Uint16(payload[pos : pos+2])
			pos += 2
			fmt.Printf("%sOBJECT index=%d\n", indent, idx)

		case dsf.CmdObjectRange:
			if pos+4 > len(payload) {
				return
			}
			start := binary.LittleEndian.Uint16(payload[pos : pos+2])
			end := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
			pos += 4
			fmt.Printf("%sOBJECT_RANGE start=%d end=%d\n", indent, start, end)

		case dsf.CmdNetworkChain:
			if pos+1 > len(payload) {
				return
			}
			count := int(payload[pos])
			pos++
			if pos+count*2 > len(payload) {
				return
			}
			indices := readUint16Slice(payload[pos:pos+count*2], count)
			pos += count * 2
			fmt.Printf("%sNETWORK_CHAIN count=%d indices=%v\n", indent, count, indices)

		case dsf.CmdNetworkRange:
			if pos+4 > len(payload) {
				return
			}
			start := binary.LittleEndian.Uint16(payload[pos : pos+2])
			end := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
			pos += 4
			fmt.Printf("%sNETWORK_RANGE start=%d end=%d\n", indent, start, end)

		case dsf.CmdPolygon:
			if pos+4 > len(payload) {
				return
			}
			param := binary.LittleEndian.Uint16(payload[pos : pos+2])
			count := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
			pos += 4
			if pos+int(count)*2 > len(payload) {
				return
			}
			indices := readUint16Slice(payload[pos:pos+int(count)*2], int(count))
			pos += int(count) * 2
			fmt.Printf("%sPOLYGON param=%d count=%d indices=%v\n", indent, param, count, indices)

		case dsf.CmdPolygonRange:
			if pos+6 > len(payload) {
				return
			}
			param := binary.LittleEndian.Uint16(payload[pos : pos+2])
			start := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
			end := binary.LittleEndian.Uint16(payload[pos+4 : pos+6])
			pos += 6
			fmt.Printf("%sPOLYGON_RANGE param=%d start=%d end=%d\n", indent, param, start, end)

		case dsf.CmdTerrainPatch:
			fmt.Printf("%sTERRAIN_PATCH\n", indent)

		case dsf.CmdTerrainPatchC:
			if pos+1 > len(payload) {
				return
			}
			flags := payload[pos]
			pos++
			fmt.Printf("%sTERRAIN_PATCH_C flags=%d\n", indent, flags)

		case dsf.CmdTriangleFan:
			if pos+1 > len(payload) {
				return
			}
			count := int(payload[pos])
			pos++
			if pos+count*2 > len(payload) {
				return
			}
			indices := readUint16Slice(payload[pos:pos+count*2], count)
			pos += count * 2
			fmt.Printf("%sTRIANGLE_FAN count=%d indices=%v\n", indent, count, indices)

		case dsf.CmdTriangleFanC:
			if pos+1 > len(payload) {
				return
			}
			count := int(payload[pos])
			pos++
			if pos+count*4 > len(payload) {
				return
			}
			pairs := make([]string, count)
			for i := 0; i < count; i++ {
				pool := binary.LittleEndian.Uint16(payload[pos : pos+2])
				idx := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
				pairs[i] = fmt.Sprintf("%d:%d", pool, idx)
				pos += 4
			}
			fmt.Printf("%sTRIANGLE_FAN_C count=%d pairs=[%s]\n", indent, count, strings.Join(pairs, " "))

		case dsf.CmdTriangleFanR:
			if pos+4 > len(payload) {
				return
			}
			start := binary.LittleEndian.Uint16(payload[pos : pos+2])
			end := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
			pos += 4
			fmt.Printf("%sTRIANGLE_FAN_R start=%d end=%d\n", indent, start, end)

		case dsf.CmdComment:
			// Skip to next opcode — comments have no defined length in DSF spec,
			// but typically the next byte is a length or we just skip one byte.
			// According to the spec, "skip to next opcode" means it has no params.
			fmt.Printf("%sCOMMENT\n", indent)

		case dsf.CmdPolygonWinding:
			fmt.Printf("%sPOLYGON_WINDING\n", indent)

		default:
			fmt.Printf("%sUNKNOWN opcode=%d\n", indent, opcode)
			return // Cannot continue since we don't know the parameter length
		}
	}
}

// splitNullTerminated splits a byte slice by null bytes and returns non-empty strings.
func splitNullTerminated(data []byte) []string {
	var result []string
	for _, part := range bytes.Split(data, []byte{0}) {
		if len(part) > 0 {
			result = append(result, string(part))
		}
	}
	return result
}

// readUint16Slice reads n little-endian uint16 values from data.
func readUint16Slice(data []byte, n int) []uint16 {
	result := make([]uint16, n)
	for i := 0; i < n; i++ {
		result[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	return result
}
