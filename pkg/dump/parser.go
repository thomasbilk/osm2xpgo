// Package dump provides a DSF binary parser and pretty-printer for diagnostic inspection.
package dump

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// atomHeaderSize is the fixed size of an atom header (4 bytes ID + 4 bytes size).
const atomHeaderSize = 8

// md5FooterSize is the size of the trailing MD5 checksum.
const md5FooterSize = 16

// cookieSize is the size of the DSF file cookie.
const cookieSize = 8

// versionSize is the size of the DSF version field.
const versionSize = 4

// headerSize is the combined size of cookie + version.
const headerSize = cookieSize + versionSize

// DSFFile represents the parsed structure of a DSF binary file.
type DSFFile struct {
	Version uint32
	Atoms   []ParsedAtom
}

// ParsedAtom represents a single atom in the DSF file's atom tree.
type ParsedAtom struct {
	ID       uint32       // 4-char identifier as little-endian uint32
	Size     uint32       // Total size including the 8-byte header
	Offset   int64        // Byte offset in the file where this atom starts
	Payload  []byte       // Raw payload bytes (for leaf atoms)
	Children []ParsedAtom // Nested atoms (for container atoms)
}

// containerAtoms defines which atom IDs are containers that hold nested sub-atoms.
var containerAtoms = map[uint32]bool{
	dsf.AtomHEAD: true,
	dsf.AtomDEFN: true,
	dsf.AtomGEOD: true,
}

// ParseError describes a structural error encountered while parsing a DSF file.
type ParseError struct {
	Offset  int64  // Byte offset where the error was detected
	Message string // Description of the structural error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("dsf parse error at offset %d: %s", e.Offset, e.Message)
}

// Parse reads a DSF binary file from disk and returns its structured atom tree.
func Parse(filePath string) (*DSFFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read DSF file: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes parses DSF binary data from a byte slice and returns
// its structured atom tree.
func ParseBytes(data []byte) (*DSFFile, error) {
	if len(data) < headerSize+md5FooterSize {
		return nil, &ParseError{
			Offset:  0,
			Message: fmt.Sprintf("file too small: %d bytes, minimum is %d", len(data), headerSize+md5FooterSize),
		}
	}

	// Validate cookie.
	var cookie [cookieSize]byte
	copy(cookie[:], data[:cookieSize])
	if cookie != dsf.Cookie {
		return nil, &ParseError{
			Offset:  0,
			Message: fmt.Sprintf("invalid cookie: got %q, expected %q", string(cookie[:]), string(dsf.Cookie[:])),
		}
	}

	// Read version.
	version := binary.LittleEndian.Uint32(data[cookieSize : cookieSize+versionSize])

	// Parse atoms from after the header to before the MD5 footer.
	atomData := data[headerSize : len(data)-md5FooterSize]
	atoms, err := parseAtoms(atomData, int64(headerSize))
	if err != nil {
		return nil, err
	}

	return &DSFFile{
		Version: version,
		Atoms:   atoms,
	}, nil
}

// parseAtoms parses a sequence of atoms from a byte slice.
// baseOffset is the absolute file offset where this slice starts.
func parseAtoms(data []byte, baseOffset int64) ([]ParsedAtom, error) {
	var atoms []ParsedAtom
	pos := 0

	for pos < len(data) {
		absOffset := baseOffset + int64(pos)

		// Check we have enough bytes for an atom header.
		if pos+atomHeaderSize > len(data) {
			return nil, &ParseError{
				Offset:  absOffset,
				Message: fmt.Sprintf("truncated atom header: only %d bytes remaining, need %d", len(data)-pos, atomHeaderSize),
			}
		}

		id := binary.LittleEndian.Uint32(data[pos : pos+4])
		size := binary.LittleEndian.Uint32(data[pos+4 : pos+8])

		// Validate atom size.
		if size < atomHeaderSize {
			return nil, &ParseError{
				Offset:  absOffset,
				Message: fmt.Sprintf("invalid atom size %d: must be at least %d (header size)", size, atomHeaderSize),
			}
		}

		if pos+int(size) > len(data) {
			return nil, &ParseError{
				Offset:  absOffset,
				Message: fmt.Sprintf("atom size %d exceeds available data (%d bytes remaining)", size, len(data)-pos),
			}
		}

		payloadStart := pos + atomHeaderSize
		payloadEnd := pos + int(size)
		payload := data[payloadStart:payloadEnd]

		atom := ParsedAtom{
			ID:     id,
			Size:   size,
			Offset: absOffset,
		}

		// If this is a container atom, recursively parse its children.
		if containerAtoms[id] {
			children, err := parseAtoms(payload, absOffset+atomHeaderSize)
			if err != nil {
				return nil, err
			}
			atom.Children = children
		} else {
			atom.Payload = payload
		}

		atoms = append(atoms, atom)
		pos += int(size)
	}

	return atoms, nil
}

// IDString converts a uint32 atom ID to its 4-character ASCII string representation.
func IDString(id uint32) string {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], id)
	return string(buf[:])
}
