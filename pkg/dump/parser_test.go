package dump

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// buildDSF constructs a minimal valid DSF byte slice from atoms.
func buildDSF(version uint32, atoms ...[]byte) []byte {
	var buf []byte
	// Cookie
	buf = append(buf, dsf.Cookie[:]...)
	// Version
	var vBuf [4]byte
	binary.LittleEndian.PutUint32(vBuf[:], version)
	buf = append(buf, vBuf[:]...)
	// Atoms
	for _, a := range atoms {
		buf = append(buf, a...)
	}
	// MD5 footer (16 zero bytes for testing)
	buf = append(buf, make([]byte, 16)...)
	return buf
}

// makeAtom builds a raw atom with the given ID and payload.
func makeAtom(id uint32, payload []byte) []byte {
	size := uint32(8 + len(payload))
	buf := make([]byte, size)
	binary.LittleEndian.PutUint32(buf[0:4], id)
	binary.LittleEndian.PutUint32(buf[4:8], size)
	copy(buf[8:], payload)
	return buf
}

func TestParseBytes_ValidMinimalFile(t *testing.T) {
	// A DSF file with a single CMDS leaf atom (empty payload).
	cmds := makeAtom(dsf.AtomCMDS, nil)
	data := buildDSF(1, cmds)

	f, err := parseResult(t, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Version != 1 {
		t.Errorf("version = %d, want 1", f.Version)
	}
	if len(f.Atoms) != 1 {
		t.Fatalf("atom count = %d, want 1", len(f.Atoms))
	}
	if f.Atoms[0].ID != dsf.AtomCMDS {
		t.Errorf("atom ID = 0x%08X, want 0x%08X", f.Atoms[0].ID, dsf.AtomCMDS)
	}
	if f.Atoms[0].Size != 8 {
		t.Errorf("atom size = %d, want 8", f.Atoms[0].Size)
	}
	if f.Atoms[0].Offset != int64(headerSize) {
		t.Errorf("atom offset = %d, want %d", f.Atoms[0].Offset, headerSize)
	}
}

func TestParseBytes_NestedAtoms(t *testing.T) {
	// HEAD containing a PROP sub-atom with some payload.
	propPayload := []byte("sim/west\x00-180\x00")
	prop := makeAtom(dsf.AtomPROP, propPayload)
	head := makeAtom(dsf.AtomHEAD, prop)
	data := buildDSF(1, head)

	f, err := parseResult(t, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Atoms) != 1 {
		t.Fatalf("top-level atom count = %d, want 1", len(f.Atoms))
	}
	headAtom := f.Atoms[0]
	if headAtom.ID != dsf.AtomHEAD {
		t.Errorf("top-level atom ID = 0x%08X, want HEAD", headAtom.ID)
	}
	if len(headAtom.Children) != 1 {
		t.Fatalf("HEAD children count = %d, want 1", len(headAtom.Children))
	}
	propAtom := headAtom.Children[0]
	if propAtom.ID != dsf.AtomPROP {
		t.Errorf("child atom ID = 0x%08X, want PROP", propAtom.ID)
	}
	if string(propAtom.Payload) != string(propPayload) {
		t.Errorf("PROP payload = %q, want %q", propAtom.Payload, propPayload)
	}
}

func TestParseBytes_MultipleTopLevelAtoms(t *testing.T) {
	// HEAD + DEFN + GEOD + CMDS (typical DSF structure).
	head := makeAtom(dsf.AtomHEAD, makeAtom(dsf.AtomPROP, []byte("key\x00val\x00")))
	defn := makeAtom(dsf.AtomDEFN, makeAtom(dsf.AtomTERT, []byte("terrain.ter\x00")))
	geod := makeAtom(dsf.AtomGEOD, makeAtom(dsf.AtomPOOL, []byte{0x01, 0x02}))
	cmds := makeAtom(dsf.AtomCMDS, []byte{0x05, 0x00, 0x01})
	data := buildDSF(1, head, defn, geod, cmds)

	f, err := parseResult(t, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Atoms) != 4 {
		t.Fatalf("top-level atom count = %d, want 4", len(f.Atoms))
	}
	// CMDS should be a leaf.
	cmdsAtom := f.Atoms[3]
	if cmdsAtom.ID != dsf.AtomCMDS {
		t.Errorf("4th atom ID = 0x%08X, want CMDS", cmdsAtom.ID)
	}
	if len(cmdsAtom.Children) != 0 {
		t.Errorf("CMDS should be a leaf, got %d children", len(cmdsAtom.Children))
	}
	if len(cmdsAtom.Payload) != 3 {
		t.Errorf("CMDS payload len = %d, want 3", len(cmdsAtom.Payload))
	}
}

func TestParseBytes_InvalidCookie(t *testing.T) {
	data := buildDSF(1, makeAtom(dsf.AtomCMDS, nil))
	// Corrupt the cookie.
	data[0] = 'Z'

	_, err := ParseBytes(data)
	if err == nil {
		t.Fatal("expected error for invalid cookie")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
	if pe.Offset != 0 {
		t.Errorf("error offset = %d, want 0", pe.Offset)
	}
}

func TestParseBytes_FileTooSmall(t *testing.T) {
	// Less than header + MD5 footer.
	data := []byte("XPLNEDSF")

	_, err := ParseBytes(data)
	if err == nil {
		t.Fatal("expected error for too-small file")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseBytes_TruncatedAtomHeader(t *testing.T) {
	// A file where the atom area has only 4 bytes (incomplete header).
	var buf []byte
	buf = append(buf, dsf.Cookie[:]...)
	var vBuf [4]byte
	binary.LittleEndian.PutUint32(vBuf[:], 1)
	buf = append(buf, vBuf[:]...)
	buf = append(buf, []byte{0x01, 0x02, 0x03, 0x04}...) // incomplete atom header
	buf = append(buf, make([]byte, 16)...)                // MD5 footer

	_, err := ParseBytes(buf)
	if err == nil {
		t.Fatal("expected error for truncated atom header")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseBytes_AtomSizeExceedsData(t *testing.T) {
	// An atom that claims to be larger than remaining data.
	var atomBuf [8]byte
	binary.LittleEndian.PutUint32(atomBuf[0:4], dsf.AtomCMDS)
	binary.LittleEndian.PutUint32(atomBuf[4:8], 1000) // way too big

	var buf []byte
	buf = append(buf, dsf.Cookie[:]...)
	var vBuf [4]byte
	binary.LittleEndian.PutUint32(vBuf[:], 1)
	buf = append(buf, vBuf[:]...)
	buf = append(buf, atomBuf[:]...)
	buf = append(buf, make([]byte, 16)...) // MD5 footer

	_, err := ParseBytes(buf)
	if err == nil {
		t.Fatal("expected error for atom size exceeding data")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseBytes_AtomSizeTooSmall(t *testing.T) {
	// An atom with size < 8 (header size).
	var atomBuf [8]byte
	binary.LittleEndian.PutUint32(atomBuf[0:4], dsf.AtomCMDS)
	binary.LittleEndian.PutUint32(atomBuf[4:8], 4) // less than header

	var buf []byte
	buf = append(buf, dsf.Cookie[:]...)
	var vBuf [4]byte
	binary.LittleEndian.PutUint32(vBuf[:], 1)
	buf = append(buf, vBuf[:]...)
	buf = append(buf, atomBuf[:]...)
	buf = append(buf, make([]byte, 16)...) // MD5 footer

	_, err := ParseBytes(buf)
	if err == nil {
		t.Fatal("expected error for atom size too small")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseBytes_Offsets(t *testing.T) {
	// Verify byte offsets are tracked correctly.
	cmds1 := makeAtom(dsf.AtomCMDS, []byte{0xAA, 0xBB})
	cmds2 := makeAtom(dsf.AtomCMDS, []byte{0xCC})
	data := buildDSF(1, cmds1, cmds2)

	f, err := parseResult(t, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Atoms) != 2 {
		t.Fatalf("atom count = %d, want 2", len(f.Atoms))
	}
	// First atom at offset 12 (8 cookie + 4 version).
	if f.Atoms[0].Offset != 12 {
		t.Errorf("first atom offset = %d, want 12", f.Atoms[0].Offset)
	}
	// Second atom at offset 12 + 10 = 22.
	expectedSecondOffset := int64(12 + len(cmds1))
	if f.Atoms[1].Offset != expectedSecondOffset {
		t.Errorf("second atom offset = %d, want %d", f.Atoms[1].Offset, expectedSecondOffset)
	}
}

func TestIDString(t *testing.T) {
	tests := []struct {
		id   uint32
		want string
	}{
		{dsf.AtomHEAD, "HEAD"},
		{dsf.AtomPROP, "PROP"},
		{dsf.AtomDEFN, "DEFN"},
		{dsf.AtomCMDS, "CMDS"},
		{dsf.AtomGEOD, "GEOD"},
	}
	for _, tt := range tests {
		got := IDString(tt.id)
		if got != tt.want {
			t.Errorf("IDString(0x%08X) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestParseBytes_Version(t *testing.T) {
	cmds := makeAtom(dsf.AtomCMDS, nil)
	data := buildDSF(42, cmds)

	f, err := parseResult(t, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Version != 42 {
		t.Errorf("version = %d, want 42", f.Version)
	}
}

func parseResult(t *testing.T, data []byte) (*DSFFile, error) {
	t.Helper()
	return ParseBytes(data)
}
