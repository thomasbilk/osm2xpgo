package writer

import (
	"encoding/binary"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

func TestAssembleDSF_VectorOnly_UsesPool32IndexZero(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	blocks := []dsf.BuildingBlock{
		{
			Type:    dsf.BlockVector,
			Tile:    tile,
			DefPath: "lib/g10/roads.net",
			Coords: []dsf.Coordinate{
				{Lon: 7.1, Lat: 43.1, Ele: 0},
				{Lon: 7.2, Lat: 43.2, Ele: 0},
			},
		},
	}

	data, err := assembleDSF(tile, blocks)
	if err != nil {
		t.Fatalf("assembleDSF returned error: %v", err)
	}

	cmds := topLevelAtomPayload(t, data, dsf.AtomCMDS)
	if len(cmds) < 3 {
		t.Fatalf("CMDS payload too short: %d", len(cmds))
	}
	if cmds[0] != dsf.CmdPoolSelect {
		t.Fatalf("first CMDS opcode = %d, want pool select (%d)", cmds[0], dsf.CmdPoolSelect)
	}
	poolIdx := binary.LittleEndian.Uint16(cmds[1:3])
	if poolIdx != 0 {
		t.Fatalf("vector-only DSF selected pool %d, want 0", poolIdx)
	}
}

func TestAssembleDSF_MixedPools_VectorSelectsPool32IndexOne(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	blocks := []dsf.BuildingBlock{
		{
			Type:    dsf.BlockPolygon,
			Tile:    tile,
			DefPath: "lib/g10/terrain10/forest_tmp.for",
			Coords: []dsf.Coordinate{
				{Lon: 7.01, Lat: 43.01, Ele: 0},
				{Lon: 7.02, Lat: 43.01, Ele: 0},
				{Lon: 7.02, Lat: 43.02, Ele: 0},
			},
		},
		{
			Type:    dsf.BlockVector,
			Tile:    tile,
			DefPath: "lib/g10/roads.net",
			Coords: []dsf.Coordinate{
				{Lon: 7.1, Lat: 43.1, Ele: 0},
				{Lon: 7.2, Lat: 43.2, Ele: 0},
			},
		},
	}

	data, err := assembleDSF(tile, blocks)
	if err != nil {
		t.Fatalf("assembleDSF returned error: %v", err)
	}

	cmds := topLevelAtomPayload(t, data, dsf.AtomCMDS)
	if len(cmds) < 12 {
		t.Fatalf("CMDS payload too short: %d", len(cmds))
	}

	// First block is polygon, so first pool select should be 16-bit pool 0.
	if cmds[0] != dsf.CmdPoolSelect {
		t.Fatalf("first CMDS opcode = %d, want pool select (%d)", cmds[0], dsf.CmdPoolSelect)
	}
	firstPoolIdx := binary.LittleEndian.Uint16(cmds[1:3])
	if firstPoolIdx != 0 {
		t.Fatalf("first pool select index = %d, want 0", firstPoolIdx)
	}

	// Polygon block emits: POOL_SELECT(3) + SET_DEFINITION(3) + POLYGON(11 for 3 indices).
	secondSelectPos := 3 + 3 + 11
	if len(cmds) < secondSelectPos+3 {
		t.Fatalf("CMDS payload too short for second pool select: %d", len(cmds))
	}
	if cmds[secondSelectPos] != dsf.CmdPoolSelect {
		t.Fatalf("opcode at second pool select position = %d, want %d", cmds[secondSelectPos], dsf.CmdPoolSelect)
	}
	secondPoolIdx := binary.LittleEndian.Uint16(cmds[secondSelectPos+1 : secondSelectPos+3])
	if secondPoolIdx != 1 {
		t.Fatalf("second pool select index = %d, want 1", secondPoolIdx)
	}
}

func topLevelAtomPayload(t *testing.T, data []byte, atomID uint32) []byte {
	t.Helper()

	const headerSize = 12
	const md5FooterSize = 16

	if len(data) < headerSize+md5FooterSize {
		t.Fatalf("DSF data too short: %d", len(data))
	}

	pos := headerSize
	limit := len(data) - md5FooterSize
	for pos+8 <= limit {
		id := binary.LittleEndian.Uint32(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if size < 8 || pos+size > limit {
			t.Fatalf("invalid atom size %d at offset %d", size, pos)
		}
		if id == atomID {
			return data[pos+8 : pos+size]
		}
		pos += size
	}

	t.Fatalf("atom 0x%08X not found", atomID)
	return nil
}
