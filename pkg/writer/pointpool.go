package writer

import (
	"encoding/binary"
	"math"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// EncodePointPool encodes a slice of coordinates into a 16-bit point pool (POOL atom)
// and its corresponding scale/offset (SCAL atom). Coordinates are encoded as 3 planes:
// longitude, latitude, and elevation.
//
// The encoding formula (computed in float64 precision):
//
//	encoded_uint16 = uint16((value - float64(offset)) / float64(scale))
//
// Scale and offset for lon/lat use the tile's integer degree as offset and
// 1.0/65535.0 as scale (mapping one full degree to the uint16 range).
// Elevation uses a computed range from the input data.
func EncodePointPool(tile dsf.TileCoord, coords []dsf.Coordinate) (poolAtom []byte, scalAtom []byte) {
	const planes uint8 = 3
	count := uint32(len(coords))

	// Compute scale/offset in float64, store as float32.
	lonOffset := float32(tile.Lon)
	lonScale := float32(1.0 / 65535.0)
	latOffset := float32(tile.Lat)
	latScale := float32(1.0 / 65535.0)

	// Compute elevation range for scaling.
	eleOffset, eleScale := computeElevationScaleOffset(coords)

	scale := []float32{lonScale, latScale, eleScale}
	offset := []float32{lonOffset, latOffset, eleOffset}

	// Encode coordinates as uint16 values.
	data := make([][]uint16, planes)
	for p := range data {
		data[p] = make([]uint16, count)
	}

	for i, c := range coords {
		data[0][i] = encodeUint16(c.Lon, offset[0], scale[0])
		data[1][i] = encodeUint16(c.Lat, offset[1], scale[1])
		data[2][i] = encodeUint16(c.Ele, offset[2], scale[2])
	}

	// Build POOL payload: uint8 planes, uint32 count, then [plane][point] uint16 values.
	poolPayload := encodePoolPayload(planes, count, data)
	poolAtom = EncodeAtom(dsf.AtomPOOL, poolPayload)

	// Build SCAL payload: for each plane, float32 scale then float32 offset.
	scalPayload := encodeSCALPayload(scale, offset)
	scalAtom = EncodeAtom(dsf.AtomSCAL, scalPayload)

	return poolAtom, scalAtom
}

// EncodePointPool32 encodes a slice of coordinates into a 32-bit point pool (PO32 atom)
// and its corresponding scale/offset (SC32 atom). Used for vector network coordinates
// requiring higher precision.
//
// The encoding formula is the same as EncodePointPool but uses uint32 range (4294967295).
func EncodePointPool32(tile dsf.TileCoord, coords []dsf.Coordinate) (po32Atom []byte, sc32Atom []byte) {
	const planes uint8 = 3
	count := uint32(len(coords))

	// Compute scale/offset in float64, store as float32.
	lonOffset := float32(tile.Lon)
	lonScale := float32(1.0 / 4294967295.0)
	latOffset := float32(tile.Lat)
	latScale := float32(1.0 / 4294967295.0)

	// Compute elevation range for scaling.
	eleOffset, eleScale := computeElevationScaleOffset32(coords)

	scale := []float32{lonScale, latScale, eleScale}
	offset := []float32{lonOffset, latOffset, eleOffset}

	// Encode coordinates as uint32 values.
	data := make([][]uint32, planes)
	for p := range data {
		data[p] = make([]uint32, count)
	}

	for i, c := range coords {
		data[0][i] = encodeUint32(c.Lon, offset[0], scale[0])
		data[1][i] = encodeUint32(c.Lat, offset[1], scale[1])
		data[2][i] = encodeUint32(c.Ele, offset[2], scale[2])
	}

	// Build PO32 payload: uint8 planes, uint32 count, then [plane][point] uint32 values.
	po32Payload := encodePool32Payload(planes, count, data)
	po32Atom = EncodeAtom(dsf.AtomPO32, po32Payload)

	// Build SC32 payload: for each plane, float32 scale then float32 offset.
	sc32Payload := encodeSCALPayload(scale, offset)
	sc32Atom = EncodeAtom(dsf.AtomSC32, sc32Payload)

	return po32Atom, sc32Atom
}

// encodeUint16 encodes a float64 value to uint16 using the DSF formula.
// Computation is done in float64 precision before converting to uint16.
func encodeUint16(value float64, offset float32, scale float32) uint16 {
	encoded := (value - float64(offset)) / float64(scale)
	// Clamp to valid uint16 range.
	if encoded < 0 {
		return 0
	}
	if encoded > 65535 {
		return 65535
	}
	return uint16(math.Round(encoded))
}

// encodeUint32 encodes a float64 value to uint32 using the DSF formula.
// Computation is done in float64 precision before converting to uint32.
func encodeUint32(value float64, offset float32, scale float32) uint32 {
	encoded := (value - float64(offset)) / float64(scale)
	// Clamp to valid uint32 range.
	if encoded < 0 {
		return 0
	}
	if encoded > 4294967295 {
		return 4294967295
	}
	return uint32(math.Round(encoded))
}

// decodeUint16 decodes a uint16 encoded value back to float64 using the DSF formula.
func decodeUint16(encoded uint16, offset float32, scale float32) float64 {
	return float64(encoded)*float64(scale) + float64(offset)
}

// decodeUint32 decodes a uint32 encoded value back to float64 using the DSF formula.
func decodeUint32(encoded uint32, offset float32, scale float32) float64 {
	return float64(encoded)*float64(scale) + float64(offset)
}

// computeElevationScaleOffset computes the scale and offset for the elevation plane
// based on the min/max elevation in the coordinate slice. For uint16 encoding.
func computeElevationScaleOffset(coords []dsf.Coordinate) (offset float32, scale float32) {
	if len(coords) == 0 {
		return 0, 1.0 / 65535.0
	}

	minEle := coords[0].Ele
	maxEle := coords[0].Ele
	for _, c := range coords[1:] {
		if c.Ele < minEle {
			minEle = c.Ele
		}
		if c.Ele > maxEle {
			maxEle = c.Ele
		}
	}

	offset = float32(minEle)
	eleRange := maxEle - minEle
	if eleRange < 1.0 {
		// Minimum range of 1 meter to avoid division by zero.
		eleRange = 1.0
	}
	scale = float32(eleRange / 65535.0)
	return offset, scale
}

// computeElevationScaleOffset32 computes the scale and offset for the elevation plane
// based on the min/max elevation in the coordinate slice. For uint32 encoding.
func computeElevationScaleOffset32(coords []dsf.Coordinate) (offset float32, scale float32) {
	if len(coords) == 0 {
		return 0, 1.0 / 4294967295.0
	}

	minEle := coords[0].Ele
	maxEle := coords[0].Ele
	for _, c := range coords[1:] {
		if c.Ele < minEle {
			minEle = c.Ele
		}
		if c.Ele > maxEle {
			maxEle = c.Ele
		}
	}

	offset = float32(minEle)
	eleRange := maxEle - minEle
	if eleRange < 1.0 {
		eleRange = 1.0
	}
	scale = float32(eleRange / 4294967295.0)
	return offset, scale
}

// encodePoolPayload builds the binary payload for a POOL atom.
// Layout: uint8 planes, uint32 count (LE), then for each plane, for each point: uint16 (LE).
func encodePoolPayload(planes uint8, count uint32, data [][]uint16) []byte {
	// Header: 1 byte (planes) + 4 bytes (count) = 5 bytes
	// Data: planes * count * 2 bytes
	size := 5 + int(planes)*int(count)*2
	buf := make([]byte, size)

	buf[0] = planes
	binary.LittleEndian.PutUint32(buf[1:5], count)

	offset := 5
	for p := 0; p < int(planes); p++ {
		for i := 0; i < int(count); i++ {
			binary.LittleEndian.PutUint16(buf[offset:offset+2], data[p][i])
			offset += 2
		}
	}
	return buf
}

// encodePool32Payload builds the binary payload for a PO32 atom.
// Layout: uint8 planes, uint32 count (LE), then for each plane, for each point: uint32 (LE).
func encodePool32Payload(planes uint8, count uint32, data [][]uint32) []byte {
	// Header: 1 byte (planes) + 4 bytes (count) = 5 bytes
	// Data: planes * count * 4 bytes
	size := 5 + int(planes)*int(count)*4
	buf := make([]byte, size)

	buf[0] = planes
	binary.LittleEndian.PutUint32(buf[1:5], count)

	offset := 5
	for p := 0; p < int(planes); p++ {
		for i := 0; i < int(count); i++ {
			binary.LittleEndian.PutUint32(buf[offset:offset+4], data[p][i])
			offset += 4
		}
	}
	return buf
}

// encodeSCALPayload builds the binary payload for a SCAL or SC32 atom.
// Layout: for each plane, float32 scale (LE), float32 offset (LE).
func encodeSCALPayload(scale []float32, offset []float32) []byte {
	planes := len(scale)
	buf := make([]byte, planes*8) // 4 bytes scale + 4 bytes offset per plane
	for p := 0; p < planes; p++ {
		binary.LittleEndian.PutUint32(buf[p*8:p*8+4], math.Float32bits(scale[p]))
		binary.LittleEndian.PutUint32(buf[p*8+4:p*8+8], math.Float32bits(offset[p]))
	}
	return buf
}
