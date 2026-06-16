package writer

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

func TestEncodePointPool_BasicEncoding(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	coords := []dsf.Coordinate{
		{Lon: 7.5, Lat: 43.5, Ele: 100.0},
		{Lon: 7.25, Lat: 43.75, Ele: 200.0},
	}

	poolAtom, scalAtom := EncodePointPool(tile, coords)

	// Verify POOL atom header.
	poolID := binary.LittleEndian.Uint32(poolAtom[0:4])
	if poolID != dsf.AtomPOOL {
		t.Errorf("POOL atom ID = 0x%08X, want 0x%08X", poolID, dsf.AtomPOOL)
	}

	// Verify SCAL atom header.
	scalID := binary.LittleEndian.Uint32(scalAtom[0:4])
	if scalID != dsf.AtomSCAL {
		t.Errorf("SCAL atom ID = 0x%08X, want 0x%08X", scalID, dsf.AtomSCAL)
	}

	// Check POOL payload structure: 3 planes, 2 points.
	poolPayload := poolAtom[8:] // skip 8-byte atom header
	if poolPayload[0] != 3 {
		t.Errorf("planes = %d, want 3", poolPayload[0])
	}
	count := binary.LittleEndian.Uint32(poolPayload[1:5])
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestEncodePointPool_RoundTrip(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	coords := []dsf.Coordinate{
		{Lon: 7.123456, Lat: 43.654321, Ele: 150.0},
		{Lon: 7.999, Lat: 43.001, Ele: 300.0},
		{Lon: 7.0, Lat: 43.0, Ele: 0.0},
	}

	// Use the same scale/offset logic as EncodePointPool to verify round-trip.
	lonOffset := float32(tile.Lon)
	lonScale := float32(1.0 / 65535.0)
	latOffset := float32(tile.Lat)
	latScale := float32(1.0 / 65535.0)
	eleOffset, eleScale := computeElevationScaleOffset(coords)

	for _, c := range coords {
		// Encode then decode longitude.
		encLon := encodeUint16(c.Lon, lonOffset, lonScale)
		decLon := decodeUint16(encLon, lonOffset, lonScale)
		lonErr := math.Abs(decLon-c.Lon) * 111320 // degrees to meters at equator
		if lonErr > 1.0 {
			t.Errorf("Lon round-trip error: %.6f → %d → %.6f (%.2f meters)", c.Lon, encLon, decLon, lonErr)
		}

		// Encode then decode latitude.
		encLat := encodeUint16(c.Lat, latOffset, latScale)
		decLat := decodeUint16(encLat, latOffset, latScale)
		latErr := math.Abs(decLat-c.Lat) * 111320 // degrees to meters
		if latErr > 1.0 {
			t.Errorf("Lat round-trip error: %.6f → %d → %.6f (%.2f meters)", c.Lat, encLat, decLat, latErr)
		}

		// Encode then decode elevation.
		encEle := encodeUint16(c.Ele, eleOffset, eleScale)
		decEle := decodeUint16(encEle, eleOffset, eleScale)
		eleErr := math.Abs(decEle - c.Ele)
		if eleErr > 1.0 {
			t.Errorf("Ele round-trip error: %.2f → %d → %.2f (%.2f meters)", c.Ele, encEle, decEle, eleErr)
		}
	}
}

func TestEncodePointPool32_BasicEncoding(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	coords := []dsf.Coordinate{
		{Lon: 7.5, Lat: 43.5, Ele: 100.0},
	}

	po32Atom, sc32Atom := EncodePointPool32(tile, coords)

	// Verify PO32 atom header.
	po32ID := binary.LittleEndian.Uint32(po32Atom[0:4])
	if po32ID != dsf.AtomPO32 {
		t.Errorf("PO32 atom ID = 0x%08X, want 0x%08X", po32ID, dsf.AtomPO32)
	}

	// Verify SC32 atom header.
	sc32ID := binary.LittleEndian.Uint32(sc32Atom[0:4])
	if sc32ID != dsf.AtomSC32 {
		t.Errorf("SC32 atom ID = 0x%08X, want 0x%08X", sc32ID, dsf.AtomSC32)
	}

	// Check PO32 payload structure: 3 planes, 1 point.
	po32Payload := po32Atom[8:]
	if po32Payload[0] != 3 {
		t.Errorf("planes = %d, want 3", po32Payload[0])
	}
	count := binary.LittleEndian.Uint32(po32Payload[1:5])
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestEncodePointPool32_RoundTrip(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	coords := []dsf.Coordinate{
		{Lon: 7.123456789, Lat: 43.987654321, Ele: 500.5},
		{Lon: 7.0001, Lat: 43.9999, Ele: 10.0},
	}

	lonOffset := float32(tile.Lon)
	lonScale := float32(1.0 / 4294967295.0)
	latOffset := float32(tile.Lat)
	latScale := float32(1.0 / 4294967295.0)
	eleOffset, eleScale := computeElevationScaleOffset32(coords)

	for _, c := range coords {
		// Encode then decode longitude.
		encLon := encodeUint32(c.Lon, lonOffset, lonScale)
		decLon := decodeUint32(encLon, lonOffset, lonScale)
		lonErr := math.Abs(decLon-c.Lon) * 111320
		if lonErr > 0.01 { // uint32 should be sub-millimeter precision
			t.Errorf("Lon round-trip error: %.9f → %d → %.9f (%.6f meters)", c.Lon, encLon, decLon, lonErr)
		}

		// Encode then decode latitude.
		encLat := encodeUint32(c.Lat, latOffset, latScale)
		decLat := decodeUint32(encLat, latOffset, latScale)
		latErr := math.Abs(decLat-c.Lat) * 111320
		if latErr > 0.01 {
			t.Errorf("Lat round-trip error: %.9f → %d → %.9f (%.6f meters)", c.Lat, encLat, decLat, latErr)
		}

		// Encode then decode elevation.
		encEle := encodeUint32(c.Ele, eleOffset, eleScale)
		decEle := decodeUint32(encEle, eleOffset, eleScale)
		eleErr := math.Abs(decEle - c.Ele)
		if eleErr > 0.01 {
			t.Errorf("Ele round-trip error: %.2f → %d → %.2f (%.6f meters)", c.Ele, encEle, decEle, eleErr)
		}
	}
}

func TestEncodePointPool_SCALPayload(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	coords := []dsf.Coordinate{
		{Lon: 7.5, Lat: 43.5, Ele: 100.0},
	}

	_, scalAtom := EncodePointPool(tile, coords)

	// SCAL payload: 3 planes × (4 bytes scale + 4 bytes offset) = 24 bytes
	scalPayload := scalAtom[8:]
	if len(scalPayload) != 24 {
		t.Fatalf("SCAL payload length = %d, want 24", len(scalPayload))
	}

	// Plane 0 (lon): scale = 1/65535, offset = 7.0
	lonScale := math.Float32frombits(binary.LittleEndian.Uint32(scalPayload[0:4]))
	lonOffset := math.Float32frombits(binary.LittleEndian.Uint32(scalPayload[4:8]))
	if math.Abs(float64(lonScale)-1.0/65535.0) > 1e-10 {
		t.Errorf("lon scale = %e, want %e", lonScale, float32(1.0/65535.0))
	}
	if lonOffset != 7.0 {
		t.Errorf("lon offset = %f, want 7.0", lonOffset)
	}

	// Plane 1 (lat): scale = 1/65535, offset = 43.0
	latScale := math.Float32frombits(binary.LittleEndian.Uint32(scalPayload[8:12]))
	latOffset := math.Float32frombits(binary.LittleEndian.Uint32(scalPayload[12:16]))
	if math.Abs(float64(latScale)-1.0/65535.0) > 1e-10 {
		t.Errorf("lat scale = %e, want %e", latScale, float32(1.0/65535.0))
	}
	if latOffset != 43.0 {
		t.Errorf("lat offset = %f, want 43.0", latOffset)
	}
}

func TestEncodePointPool_EmptyCoords(t *testing.T) {
	tile := dsf.TileCoord{Lat: 0, Lon: 0}
	coords := []dsf.Coordinate{}

	poolAtom, scalAtom := EncodePointPool(tile, coords)

	// Should still produce valid atoms.
	poolPayload := poolAtom[8:]
	if poolPayload[0] != 3 {
		t.Errorf("planes = %d, want 3", poolPayload[0])
	}
	count := binary.LittleEndian.Uint32(poolPayload[1:5])
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// SCAL should still have 3 planes of scale/offset.
	scalPayload := scalAtom[8:]
	if len(scalPayload) != 24 {
		t.Errorf("SCAL payload length = %d, want 24", len(scalPayload))
	}
}

func TestEncodePointPool_AtomSizes(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	coords := []dsf.Coordinate{
		{Lon: 7.5, Lat: 43.5, Ele: 100.0},
		{Lon: 7.25, Lat: 43.75, Ele: 200.0},
		{Lon: 7.9, Lat: 43.1, Ele: 50.0},
	}

	poolAtom, scalAtom := EncodePointPool(tile, coords)

	// POOL atom size: 8 (header) + 1 (planes) + 4 (count) + 3*3*2 (data) = 31
	poolSize := binary.LittleEndian.Uint32(poolAtom[4:8])
	expectedPoolSize := uint32(8 + 1 + 4 + 3*3*2)
	if poolSize != expectedPoolSize {
		t.Errorf("POOL atom size = %d, want %d", poolSize, expectedPoolSize)
	}

	// SCAL atom size: 8 (header) + 3*8 (3 planes × 8 bytes each) = 32
	scalSize := binary.LittleEndian.Uint32(scalAtom[4:8])
	expectedScalSize := uint32(8 + 3*8)
	if scalSize != expectedScalSize {
		t.Errorf("SCAL atom size = %d, want %d", scalSize, expectedScalSize)
	}
}

func TestEncodePointPool32_AtomSizes(t *testing.T) {
	tile := dsf.TileCoord{Lat: 43, Lon: 7}
	coords := []dsf.Coordinate{
		{Lon: 7.5, Lat: 43.5, Ele: 100.0},
		{Lon: 7.25, Lat: 43.75, Ele: 200.0},
	}

	po32Atom, sc32Atom := EncodePointPool32(tile, coords)

	// PO32 atom size: 8 (header) + 1 (planes) + 4 (count) + 3*2*4 (data) = 37
	po32Size := binary.LittleEndian.Uint32(po32Atom[4:8])
	expectedPo32Size := uint32(8 + 1 + 4 + 3*2*4)
	if po32Size != expectedPo32Size {
		t.Errorf("PO32 atom size = %d, want %d", po32Size, expectedPo32Size)
	}

	// SC32 atom size: 8 (header) + 3*8 = 32
	sc32Size := binary.LittleEndian.Uint32(sc32Atom[4:8])
	expectedSc32Size := uint32(8 + 3*8)
	if sc32Size != expectedSc32Size {
		t.Errorf("SC32 atom size = %d, want %d", sc32Size, expectedSc32Size)
	}
}

func TestEncodeUint16_Clamping(t *testing.T) {
	// Test that values beyond the range are clamped.
	offset := float32(0.0)
	scale := float32(1.0 / 65535.0)

	// Value at lower bound.
	if v := encodeUint16(0.0, offset, scale); v != 0 {
		t.Errorf("encodeUint16(0.0) = %d, want 0", v)
	}

	// Value at upper bound.
	if v := encodeUint16(1.0, offset, scale); v != 65535 {
		t.Errorf("encodeUint16(1.0) = %d, want 65535", v)
	}

	// Value below range.
	if v := encodeUint16(-0.5, offset, scale); v != 0 {
		t.Errorf("encodeUint16(-0.5) = %d, want 0", v)
	}

	// Value above range.
	if v := encodeUint16(2.0, offset, scale); v != 65535 {
		t.Errorf("encodeUint16(2.0) = %d, want 65535", v)
	}
}

func TestComputeElevationScaleOffset_SingleElevation(t *testing.T) {
	coords := []dsf.Coordinate{
		{Ele: 100.0},
		{Ele: 100.0},
	}
	offset, scale := computeElevationScaleOffset(coords)

	// With identical elevations, range < 1.0, so minimum range of 1.0 is used.
	if offset != 100.0 {
		t.Errorf("offset = %f, want 100.0", offset)
	}
	expectedScale := float32(1.0 / 65535.0)
	if scale != expectedScale {
		t.Errorf("scale = %e, want %e", scale, expectedScale)
	}
}
