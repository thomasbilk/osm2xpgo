package dump

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// captureStdout captures stdout output from a function call.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintPROP(t *testing.T) {
	// Build PROP payload: key1\x00val1\x00key2\x00val2\x00
	payload := []byte("sim/west\x00-180\x00sim/east\x00180\x00")

	output := captureStdout(func() {
		printPROP(payload, "  ")
	})

	expected := "  sim/west = -180\n  sim/east = 180\n"
	if output != expected {
		t.Errorf("printPROP output mismatch:\ngot:  %q\nwant: %q", output, expected)
	}
}

func TestPrintDefinitions(t *testing.T) {
	payload := []byte("terrain_Water\x00terrain_Ground\x00")

	output := captureStdout(func() {
		printDefinitions(payload, "  ")
	})

	expected := "  [0] terrain_Water\n  [1] terrain_Ground\n"
	if output != expected {
		t.Errorf("printDefinitions output mismatch:\ngot:  %q\nwant: %q", output, expected)
	}
}

func TestPrintPointPool(t *testing.T) {
	// planes=3, count=40 (little-endian)
	payload := make([]byte, 5)
	payload[0] = 3
	binary.LittleEndian.PutUint32(payload[1:5], 40)

	output := captureStdout(func() {
		printPointPool(payload, "  ")
	})

	expected := "  3 planes × 40 points\n"
	if output != expected {
		t.Errorf("printPointPool output mismatch:\ngot:  %q\nwant: %q", output, expected)
	}
}

func TestPrintCMDS_PoolSelect(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(dsf.CmdPoolSelect)
	binary.Write(&buf, binary.LittleEndian, uint16(5))

	output := captureStdout(func() {
		printCMDS(buf.Bytes(), "  ")
	})

	if !strings.Contains(output, "POOL_SELECT pool=5") {
		t.Errorf("expected POOL_SELECT pool=5, got: %q", output)
	}
}

func TestPrintCMDS_SetDefinition(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(dsf.CmdSetDefinition)
	binary.Write(&buf, binary.LittleEndian, uint16(3))

	output := captureStdout(func() {
		printCMDS(buf.Bytes(), "  ")
	})

	if !strings.Contains(output, "SET_DEFINITION def=3") {
		t.Errorf("expected SET_DEFINITION def=3, got: %q", output)
	}
}

func TestPrintCMDS_Polygon(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(dsf.CmdPolygon)
	binary.Write(&buf, binary.LittleEndian, uint16(0))  // param
	binary.Write(&buf, binary.LittleEndian, uint16(3))  // count
	binary.Write(&buf, binary.LittleEndian, uint16(10)) // index 0
	binary.Write(&buf, binary.LittleEndian, uint16(11)) // index 1
	binary.Write(&buf, binary.LittleEndian, uint16(12)) // index 2

	output := captureStdout(func() {
		printCMDS(buf.Bytes(), "  ")
	})

	if !strings.Contains(output, "POLYGON param=0 count=3 indices=[10 11 12]") {
		t.Errorf("expected POLYGON output, got: %q", output)
	}
}

func TestPrintCMDS_NetworkChain(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(dsf.CmdNetworkChain)
	buf.WriteByte(2)                                    // count
	binary.Write(&buf, binary.LittleEndian, uint16(7))  // index 0
	binary.Write(&buf, binary.LittleEndian, uint16(8))  // index 1

	output := captureStdout(func() {
		printCMDS(buf.Bytes(), "  ")
	})

	if !strings.Contains(output, "NETWORK_CHAIN count=2 indices=[7 8]") {
		t.Errorf("expected NETWORK_CHAIN output, got: %q", output)
	}
}

func TestPrintCMDS_PolygonWinding(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(dsf.CmdPolygonWinding)

	output := captureStdout(func() {
		printCMDS(buf.Bytes(), "  ")
	})

	if !strings.Contains(output, "POLYGON_WINDING") {
		t.Errorf("expected POLYGON_WINDING, got: %q", output)
	}
}

func TestPrintCMDS_TerrainPatch(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(dsf.CmdTerrainPatch)

	output := captureStdout(func() {
		printCMDS(buf.Bytes(), "  ")
	})

	if !strings.Contains(output, "TERRAIN_PATCH") {
		t.Errorf("expected TERRAIN_PATCH, got: %q", output)
	}
}

func TestPrintCMDS_MultipleCommands(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(dsf.CmdPoolSelect)
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	buf.WriteByte(dsf.CmdSetDefinition)
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	buf.WriteByte(dsf.CmdPolygonWinding)

	output := captureStdout(func() {
		printCMDS(buf.Bytes(), "  ")
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}
	if !strings.Contains(lines[0], "POOL_SELECT pool=0") {
		t.Errorf("line 0: expected POOL_SELECT, got: %q", lines[0])
	}
	if !strings.Contains(lines[1], "SET_DEFINITION def=1") {
		t.Errorf("line 1: expected SET_DEFINITION, got: %q", lines[1])
	}
	if !strings.Contains(lines[2], "POLYGON_WINDING") {
		t.Errorf("line 2: expected POLYGON_WINDING, got: %q", lines[2])
	}
}

func TestPrintAtom_Indentation(t *testing.T) {
	atom := ParsedAtom{
		ID:   dsf.AtomPROP,
		Size: 18,
		Children: nil,
		Payload: []byte("k\x00v\x00"),
	}

	// Level 0 — no indentation
	output := captureStdout(func() {
		printAtom(atom, 0)
	})
	if !strings.HasPrefix(output, "PROP (18 bytes)") {
		t.Errorf("level 0: expected no indent, got: %q", output)
	}

	// Level 2 — 4 spaces indentation
	output = captureStdout(func() {
		printAtom(atom, 2)
	})
	if !strings.HasPrefix(output, "    PROP (18 bytes)") {
		t.Errorf("level 2: expected 4 spaces indent, got: %q", output)
	}
}

func TestPrintAtom_ContainerWithChildren(t *testing.T) {
	child := ParsedAtom{
		ID:      dsf.AtomPROP,
		Size:    16,
		Payload: []byte("a\x00b\x00"),
	}
	parent := ParsedAtom{
		ID:       dsf.AtomHEAD,
		Size:     24,
		Children: []ParsedAtom{child},
	}

	output := captureStdout(func() {
		printAtom(parent, 0)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), output)
	}
	// HEAD at level 0
	if !strings.HasPrefix(lines[0], "HEAD (24 bytes)") {
		t.Errorf("line 0: %q", lines[0])
	}
	// PROP at level 1 (2 spaces)
	if !strings.HasPrefix(lines[1], "  PROP (16 bytes)") {
		t.Errorf("line 1: %q", lines[1])
	}
	// key-value at level 2 (4 spaces)
	if !strings.HasPrefix(lines[2], "    a = b") {
		t.Errorf("line 2: %q", lines[2])
	}
}

// buildMinimalDSF creates a minimal valid DSF binary with one PROP atom.
func buildMinimalDSF(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write(dsf.Cookie[:])
	binary.Write(&buf, binary.LittleEndian, uint32(1)) // version

	// PROP atom with a key-value pair
	propPayload := []byte("sim/west\x00-180\x00")
	propSize := uint32(8 + len(propPayload))
	binary.Write(&buf, binary.LittleEndian, dsf.AtomPROP)
	binary.Write(&buf, binary.LittleEndian, propSize)
	buf.Write(propPayload)

	// MD5 footer (16 zero bytes for test purposes)
	buf.Write(make([]byte, 16))
	return buf.Bytes()
}

func TestDump_Integration(t *testing.T) {
	data := buildMinimalDSF(t)

	// Write to temp file
	tmpFile := t.TempDir() + "/test.dsf"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(func() {
		err := Dump(tmpFile)
		if err != nil {
			t.Errorf("Dump returned error: %v", err)
		}
	})

	if !strings.Contains(output, "DSF Version: 1") {
		t.Errorf("expected version header, got: %q", output)
	}
	if !strings.Contains(output, "PROP") {
		t.Errorf("expected PROP atom, got: %q", output)
	}
	if !strings.Contains(output, "sim/west = -180") {
		t.Errorf("expected PROP key-value, got: %q", output)
	}
}

func TestDump_InvalidFile(t *testing.T) {
	tmpFile := t.TempDir() + "/bad.dsf"
	if err := os.WriteFile(tmpFile, []byte("short"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Dump(tmpFile)
	if err == nil {
		t.Error("expected error for invalid file, got nil")
	}
}

func TestSplitNullTerminated(t *testing.T) {
	tests := []struct {
		input    []byte
		expected []string
	}{
		{[]byte("a\x00b\x00"), []string{"a", "b"}},
		{[]byte("hello\x00"), []string{"hello"}},
		{[]byte("\x00\x00"), nil},
		{[]byte("one\x00two\x00three\x00"), []string{"one", "two", "three"}},
		{[]byte{}, nil},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			got := splitNullTerminated(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("len mismatch: got %v, want %v", got, tc.expected)
			}
			for j := range got {
				if got[j] != tc.expected[j] {
					t.Errorf("index %d: got %q, want %q", j, got[j], tc.expected[j])
				}
			}
		})
	}
}

func TestReadUint16Slice(t *testing.T) {
	data := make([]byte, 6)
	binary.LittleEndian.PutUint16(data[0:2], 100)
	binary.LittleEndian.PutUint16(data[2:4], 200)
	binary.LittleEndian.PutUint16(data[4:6], 300)

	result := readUint16Slice(data, 3)
	expected := []uint16{100, 200, 300}
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("index %d: got %d, want %d", i, result[i], expected[i])
		}
	}
}
