// Package dsf provides shared DSF data types and constants used across
// the osm2xpgo pipeline stages.
package dsf

import (
	"fmt"
	"path/filepath"
)

// BlockType identifies the category of a DSF building block.
type BlockType uint8

const (
	BlockVector  BlockType = iota // Road/path network segment
	BlockPolygon                  // Forest, draped polygon (fields, etc.)
	BlockObject                   // Placed 3D object
	BlockFacade                   // Extruded building facade
)

// TileCoord represents the south-west corner of a 1×1 degree tile
// in integer degrees of latitude and longitude.
type TileCoord struct {
	Lat int // South-west corner latitude (integer degrees)
	Lon int // South-west corner longitude (integer degrees)
}

// TilePath returns the filesystem path for a tile relative to the output root.
// The path follows X-Plane's convention: a 10-degree grid directory containing
// tile-named DSF files.
// Example: TileCoord{Lat: 43, Lon: 7} → "+40+000/+43+007.dsf"
// Example: TileCoord{Lat: -12, Lon: -3} → "-20-010/-12-003.dsf"
func (t TileCoord) TilePath() string {
	gridLat := floorDiv(t.Lat, 10) * 10
	gridLon := floorDiv(t.Lon, 10) * 10
	dir := fmt.Sprintf("%+03d%+04d", gridLat, gridLon)
	file := fmt.Sprintf("%+03d%+04d.dsf", t.Lat, t.Lon)
	return filepath.Join(dir, file)
}

// floorDiv performs integer division that rounds towards negative infinity,
// matching the mathematical floor function. Go's built-in integer division
// truncates towards zero, which gives incorrect results for negative dividends.
func floorDiv(a, b int) int {
	d := a / b
	if (a^b) < 0 && d*b != a {
		d--
	}
	return d
}

// Coordinate represents a WGS84 geographic position with optional elevation.
type Coordinate struct {
	Lon float64 // WGS84 longitude in degrees
	Lat float64 // WGS84 latitude in degrees
	Ele float64 // Elevation in meters above mean sea level
}

// BuildingBlock is the pipeline element sent from the Converter to the Writer.
// It represents a single DSF feature (vector, polygon, facade, or object) assigned to
// a specific tile.
type BuildingBlock struct {
	Type     BlockType         // Vector, Polygon, Facade, or Object
	Tile     TileCoord         // Assigned 1×1 degree tile
	Coords   []Coordinate      // WGS84 coordinates (lon, lat, optional elevation)
	Windings [][]Coordinate    // For multipolygons: outer + inner windings
	DefIndex int               // Index into definition table
	DefPath  string            // Definition file path (e.g., "lib/g10/roads.net")
	SubType  uint8             // Road subtype for vectors
	Param    uint16            // Polygon parameter: density (0-255) for forests, heading for .pol, height for facades
	Tags     map[string]string // Original OSM tags for metadata
}
