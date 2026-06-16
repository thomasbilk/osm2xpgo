package writer

import (
	"encoding/binary"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

func TestEncodeAtom_EmptyPayload(t *testing.T) {
	result := EncodeAtom(dsf.AtomHEAD, nil)

	if len(result) != atomHeaderSize {
		t.Fatalf("expected length %d, got %d", atomHeaderSize, len(result))
	}

	id := binary.LittleEndian.Uint32(result[0:4])
	if id != dsf.AtomHEAD {
		t.Errorf("expected atom ID 0x%08X, got 0x%08X", dsf.AtomHEAD, id)
	}

	size := binary.LittleEndian.Uint32(result[4:8])
	if size != atomHeaderSize {
		t.Errorf("expected size %d, got %d", atomHeaderSize, size)
	}
}

func TestEncodeAtom_WithPayload(t *testing.T) {
	payload := []byte("sim/west\x00-180\x00")
	result := EncodeAtom(dsf.AtomPROP, payload)

	expectedLen := atomHeaderSize + len(payload)
	if len(result) != expectedLen {
		t.Fatalf("expected length %d, got %d", expectedLen, len(result))
	}

	id := binary.LittleEndian.Uint32(result[0:4])
	if id != dsf.AtomPROP {
		t.Errorf("expected atom ID 0x%08X, got 0x%08X", dsf.AtomPROP, id)
	}

	size := binary.LittleEndian.Uint32(result[4:8])
	if size != uint32(expectedLen) {
		t.Errorf("expected size %d, got %d", expectedLen, size)
	}

	// Verify payload is intact
	for i, b := range payload {
		if result[atomHeaderSize+i] != b {
			t.Errorf("payload byte %d: expected 0x%02X, got 0x%02X", i, b, result[atomHeaderSize+i])
		}
	}
}

func TestEncodeAtom_LittleEndianByteOrder(t *testing.T) {
	// AtomHEAD = 0x44414548 → bytes: 0x48, 0x45, 0x41, 0x44 ("HEAD" in ASCII)
	result := EncodeAtom(dsf.AtomHEAD, []byte{0xFF})

	// Verify ID bytes are in little-endian order
	if result[0] != 0x48 || result[1] != 0x45 || result[2] != 0x41 || result[3] != 0x44 {
		t.Errorf("ID not in little-endian order: got [%02X %02X %02X %02X]",
			result[0], result[1], result[2], result[3])
	}

	// Size = 9 (8 header + 1 byte payload) → 0x09 0x00 0x00 0x00
	if result[4] != 0x09 || result[5] != 0x00 || result[6] != 0x00 || result[7] != 0x00 {
		t.Errorf("size not in little-endian order: got [%02X %02X %02X %02X]",
			result[4], result[5], result[6], result[7])
	}
}

func TestBuildAtom_NoChildren(t *testing.T) {
	result := BuildAtom(dsf.AtomDEFN)

	if len(result) != atomHeaderSize {
		t.Fatalf("expected length %d, got %d", atomHeaderSize, len(result))
	}

	size := binary.LittleEndian.Uint32(result[4:8])
	if size != atomHeaderSize {
		t.Errorf("expected size %d, got %d", atomHeaderSize, size)
	}
}

func TestBuildAtom_WithChildren(t *testing.T) {
	// Build a PROP child atom
	propPayload := []byte("sim/west\x00-180\x00")
	propAtom := EncodeAtom(dsf.AtomPROP, propPayload)

	// Build a HEAD parent containing the PROP child
	headAtom := BuildAtom(dsf.AtomHEAD, propAtom)

	expectedLen := atomHeaderSize + len(propAtom)
	if len(headAtom) != expectedLen {
		t.Fatalf("expected length %d, got %d", expectedLen, len(headAtom))
	}

	// Verify HEAD header
	id := binary.LittleEndian.Uint32(headAtom[0:4])
	if id != dsf.AtomHEAD {
		t.Errorf("expected HEAD atom ID, got 0x%08X", id)
	}

	size := binary.LittleEndian.Uint32(headAtom[4:8])
	if size != uint32(expectedLen) {
		t.Errorf("expected size %d, got %d", expectedLen, size)
	}

	// Verify nested PROP atom is intact
	nestedID := binary.LittleEndian.Uint32(headAtom[8:12])
	if nestedID != dsf.AtomPROP {
		t.Errorf("expected nested PROP atom ID, got 0x%08X", nestedID)
	}

	nestedSize := binary.LittleEndian.Uint32(headAtom[12:16])
	expectedNestedSize := uint32(atomHeaderSize + len(propPayload))
	if nestedSize != expectedNestedSize {
		t.Errorf("expected nested size %d, got %d", expectedNestedSize, nestedSize)
	}
}

func TestBuildAtom_MultipleChildren(t *testing.T) {
	// Build a DEFN atom containing TERT and OBJT children
	tertPayload := []byte("terrain_Water\x00")
	tertAtom := EncodeAtom(dsf.AtomTERT, tertPayload)

	objtPayload := []byte("objects/tree.obj\x00")
	objtAtom := EncodeAtom(dsf.AtomOBJT, objtPayload)

	defnAtom := BuildAtom(dsf.AtomDEFN, tertAtom, objtAtom)

	expectedLen := atomHeaderSize + len(tertAtom) + len(objtAtom)
	if len(defnAtom) != expectedLen {
		t.Fatalf("expected length %d, got %d", expectedLen, len(defnAtom))
	}

	// Verify DEFN header
	id := binary.LittleEndian.Uint32(defnAtom[0:4])
	if id != dsf.AtomDEFN {
		t.Errorf("expected DEFN atom ID, got 0x%08X", id)
	}

	size := binary.LittleEndian.Uint32(defnAtom[4:8])
	if size != uint32(expectedLen) {
		t.Errorf("expected size %d, got %d", expectedLen, size)
	}

	// Verify first child (TERT) starts at offset 8
	firstChildID := binary.LittleEndian.Uint32(defnAtom[8:12])
	if firstChildID != dsf.AtomTERT {
		t.Errorf("expected first child TERT, got 0x%08X", firstChildID)
	}

	// Verify second child (OBJT) starts after TERT
	secondOffset := atomHeaderSize + len(tertAtom)
	secondChildID := binary.LittleEndian.Uint32(defnAtom[secondOffset : secondOffset+4])
	if secondChildID != dsf.AtomOBJT {
		t.Errorf("expected second child OBJT, got 0x%08X", secondChildID)
	}
}

func TestEncodeAtom_SizeIncludesHeader(t *testing.T) {
	// Requirement 4.3: total size is inclusive of the 8-byte header
	payload := make([]byte, 100)
	result := EncodeAtom(dsf.AtomCMDS, payload)

	size := binary.LittleEndian.Uint32(result[4:8])
	expectedSize := uint32(atomHeaderSize + 100)
	if size != expectedSize {
		t.Errorf("size should include header: expected %d, got %d", expectedSize, size)
	}
}
