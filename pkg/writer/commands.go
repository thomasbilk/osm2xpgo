package writer

import (
	"encoding/binary"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// EncodePoolSelect encodes a CmdPoolSelect command that sets the active point pool.
// Format: opcode(1) + pool_index(uint16 LE)
func EncodePoolSelect(poolIdx uint16) []byte {
	buf := make([]byte, 3)
	buf[0] = dsf.CmdPoolSelect
	binary.LittleEndian.PutUint16(buf[1:3], poolIdx)
	return buf
}

// EncodeSetDefinition encodes a CmdSetDefinition command that sets the active
// terrain, object, polygon, or network definition index.
// Format: opcode(1) + definition_index(uint16 LE)
func EncodeSetDefinition(defIdx uint16) []byte {
	buf := make([]byte, 3)
	buf[0] = dsf.CmdSetDefinition
	binary.LittleEndian.PutUint16(buf[1:3], defIdx)
	return buf
}

// EncodeSetRoadSubType encodes a CmdSetRoadSubType command that sets the road sub-type
// for subsequent network chain commands.
// Format: opcode(1) + subtype(uint8)
func EncodeSetRoadSubType(subtype uint8) []byte {
	return []byte{dsf.CmdSetRoadSubType, subtype}
}

// EncodeNetworkChain encodes a CmdNetworkChain command representing a sequence of
// point pool indices forming a road/path network segment.
// Format: opcode(1) + count(uint8) + indices(count × uint16 LE)
func EncodeNetworkChain(indices []uint16) []byte {
	count := len(indices)
	buf := make([]byte, 2+count*2)
	buf[0] = dsf.CmdNetworkChain
	buf[1] = uint8(count)
	for i, idx := range indices {
		binary.LittleEndian.PutUint16(buf[2+i*2:4+i*2], idx)
	}
	return buf
}

// EncodePolygon encodes a CmdPolygon command representing a polygon with a single winding.
// The param value encodes polygon-specific data (e.g., height or density).
// Format: opcode(1) + param(uint16 LE) + count(uint16 LE) + indices(count × uint16 LE)
func EncodePolygon(param uint16, indices []uint16) []byte {
	count := len(indices)
	buf := make([]byte, 5+count*2)
	buf[0] = dsf.CmdPolygon
	binary.LittleEndian.PutUint16(buf[1:3], param)
	binary.LittleEndian.PutUint16(buf[3:5], uint16(count))
	for i, idx := range indices {
		binary.LittleEndian.PutUint16(buf[5+i*2:7+i*2], idx)
	}
	return buf
}

// EncodePolygonWinding encodes a CmdPolygonWinding command that signals the start
// of a new winding within the current polygon. This command has no parameters.
// Format: opcode(1)
func EncodePolygonWinding() []byte {
	return []byte{dsf.CmdPolygonWinding}
}
