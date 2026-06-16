// Package writer provides DSF binary file serialization for the osm2xpgo pipeline.
package writer

import "encoding/binary"

// atomHeaderSize is the fixed size of an atom header (4 bytes ID + 4 bytes size).
const atomHeaderSize = 8

// EncodeAtom encodes a single atom with the given ID and payload.
// It returns the complete binary representation: 4-byte little-endian ID,
// 4-byte little-endian total size (inclusive of the 8-byte header), followed
// by the raw payload bytes.
func EncodeAtom(id uint32, payload []byte) []byte {
	totalSize := uint32(atomHeaderSize + len(payload))
	buf := make([]byte, totalSize)
	binary.LittleEndian.PutUint32(buf[0:4], id)
	binary.LittleEndian.PutUint32(buf[4:8], totalSize)
	copy(buf[8:], payload)
	return buf
}

// BuildAtom builds a parent atom whose payload is the concatenation of the
// provided child byte slices. This is useful for nested atoms (e.g., HEAD
// containing PROP).
func BuildAtom(id uint32, children ...[]byte) []byte {
	var payloadSize int
	for _, child := range children {
		payloadSize += len(child)
	}
	payload := make([]byte, 0, payloadSize)
	for _, child := range children {
		payload = append(payload, child...)
	}
	return EncodeAtom(id, payload)
}
