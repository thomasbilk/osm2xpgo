package writer

import (
	"encoding/binary"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

func TestEncodePoolSelect(t *testing.T) {
	result := EncodePoolSelect(42)

	if len(result) != 3 {
		t.Fatalf("expected length 3, got %d", len(result))
	}
	if result[0] != dsf.CmdPoolSelect {
		t.Errorf("expected opcode %d, got %d", dsf.CmdPoolSelect, result[0])
	}
	idx := binary.LittleEndian.Uint16(result[1:3])
	if idx != 42 {
		t.Errorf("expected pool index 42, got %d", idx)
	}
}

func TestEncodePoolSelect_Zero(t *testing.T) {
	result := EncodePoolSelect(0)

	if result[0] != dsf.CmdPoolSelect {
		t.Errorf("expected opcode %d, got %d", dsf.CmdPoolSelect, result[0])
	}
	idx := binary.LittleEndian.Uint16(result[1:3])
	if idx != 0 {
		t.Errorf("expected pool index 0, got %d", idx)
	}
}

func TestEncodePoolSelect_MaxValue(t *testing.T) {
	result := EncodePoolSelect(0xFFFF)

	idx := binary.LittleEndian.Uint16(result[1:3])
	if idx != 0xFFFF {
		t.Errorf("expected pool index 65535, got %d", idx)
	}
}

func TestEncodeSetDefinition(t *testing.T) {
	result := EncodeSetDefinition(7)

	if len(result) != 3 {
		t.Fatalf("expected length 3, got %d", len(result))
	}
	if result[0] != dsf.CmdSetDefinition {
		t.Errorf("expected opcode %d, got %d", dsf.CmdSetDefinition, result[0])
	}
	idx := binary.LittleEndian.Uint16(result[1:3])
	if idx != 7 {
		t.Errorf("expected definition index 7, got %d", idx)
	}
}

func TestEncodeSetDefinition_LittleEndian(t *testing.T) {
	// 0x0102 in little-endian: [0x02, 0x01]
	result := EncodeSetDefinition(0x0102)

	if result[1] != 0x02 || result[2] != 0x01 {
		t.Errorf("expected little-endian [0x02 0x01], got [0x%02X 0x%02X]", result[1], result[2])
	}
}

func TestEncodeSetRoadSubType(t *testing.T) {
	result := EncodeSetRoadSubType(3)

	if len(result) != 2 {
		t.Fatalf("expected length 2, got %d", len(result))
	}
	if result[0] != dsf.CmdSetRoadSubType {
		t.Errorf("expected opcode %d, got %d", dsf.CmdSetRoadSubType, result[0])
	}
	if result[1] != 3 {
		t.Errorf("expected subtype 3, got %d", result[1])
	}
}

func TestEncodeNetworkChain(t *testing.T) {
	indices := []uint16{10, 20, 30}
	result := EncodeNetworkChain(indices)

	// opcode(1) + count(1) + 3×uint16(6) = 8 bytes
	if len(result) != 8 {
		t.Fatalf("expected length 8, got %d", len(result))
	}
	if result[0] != dsf.CmdNetworkChain {
		t.Errorf("expected opcode %d, got %d", dsf.CmdNetworkChain, result[0])
	}
	if result[1] != 3 {
		t.Errorf("expected count 3, got %d", result[1])
	}

	for i, expected := range indices {
		got := binary.LittleEndian.Uint16(result[2+i*2 : 4+i*2])
		if got != expected {
			t.Errorf("index %d: expected %d, got %d", i, expected, got)
		}
	}
}

func TestEncodeNetworkChain_Empty(t *testing.T) {
	result := EncodeNetworkChain(nil)

	if len(result) != 2 {
		t.Fatalf("expected length 2, got %d", len(result))
	}
	if result[0] != dsf.CmdNetworkChain {
		t.Errorf("expected opcode %d, got %d", dsf.CmdNetworkChain, result[0])
	}
	if result[1] != 0 {
		t.Errorf("expected count 0, got %d", result[1])
	}
}

func TestEncodeNetworkChain_SingleIndex(t *testing.T) {
	result := EncodeNetworkChain([]uint16{500})

	if len(result) != 4 {
		t.Fatalf("expected length 4, got %d", len(result))
	}
	if result[1] != 1 {
		t.Errorf("expected count 1, got %d", result[1])
	}
	idx := binary.LittleEndian.Uint16(result[2:4])
	if idx != 500 {
		t.Errorf("expected index 500, got %d", idx)
	}
}

func TestEncodePolygon(t *testing.T) {
	indices := []uint16{5, 6, 7, 8}
	result := EncodePolygon(100, indices)

	// opcode(1) + param(2) + count(2) + 4×uint16(8) = 13 bytes
	if len(result) != 13 {
		t.Fatalf("expected length 13, got %d", len(result))
	}
	if result[0] != dsf.CmdPolygon {
		t.Errorf("expected opcode %d, got %d", dsf.CmdPolygon, result[0])
	}

	param := binary.LittleEndian.Uint16(result[1:3])
	if param != 100 {
		t.Errorf("expected param 100, got %d", param)
	}

	count := binary.LittleEndian.Uint16(result[3:5])
	if count != 4 {
		t.Errorf("expected count 4, got %d", count)
	}

	for i, expected := range indices {
		got := binary.LittleEndian.Uint16(result[5+i*2 : 7+i*2])
		if got != expected {
			t.Errorf("index %d: expected %d, got %d", i, expected, got)
		}
	}
}

func TestEncodePolygon_EmptyIndices(t *testing.T) {
	result := EncodePolygon(0, nil)

	// opcode(1) + param(2) + count(2) = 5 bytes
	if len(result) != 5 {
		t.Fatalf("expected length 5, got %d", len(result))
	}
	count := binary.LittleEndian.Uint16(result[3:5])
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestEncodePolygon_LittleEndianParam(t *testing.T) {
	// param = 0x1234 in little-endian: [0x34, 0x12]
	result := EncodePolygon(0x1234, []uint16{1})

	if result[1] != 0x34 || result[2] != 0x12 {
		t.Errorf("expected param LE [0x34 0x12], got [0x%02X 0x%02X]", result[1], result[2])
	}
}

func TestEncodePolygonWinding(t *testing.T) {
	result := EncodePolygonWinding()

	if len(result) != 1 {
		t.Fatalf("expected length 1, got %d", len(result))
	}
	if result[0] != dsf.CmdPolygonWinding {
		t.Errorf("expected opcode %d, got %d", dsf.CmdPolygonWinding, result[0])
	}
}
