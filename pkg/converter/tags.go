// Package converter transforms OSM elements into DSF building blocks.
package converter

import "github.com/thomasbilk/osm2xpgo/pkg/dsf"

// PolySubType distinguishes polygon sub-classifications.
type PolySubType uint8

const (
	// PolyGeneric represents a generic polygon (building, multipolygon).
	PolyGeneric PolySubType = iota
	// PolyForest represents a forest/woodland polygon.
	PolyForest
)

// Classification holds the result of OSM tag classification.
type Classification struct {
	Block   dsf.BlockType // The DSF block type to produce.
	SubType PolySubType   // Polygon sub-classification (only meaningful for BlockPolygon).
	Skip    bool          // If true, the element should be skipped entirely.
}

// ClassifyTags inspects OSM tags and a node count to determine what DSF
// building block type the element maps to. The nodeCount parameter is the
// number of nodes in the way (for polygons, this includes the closing node).
//
// Classification rules (evaluated in priority order):
//   - highway=* → BlockVector
//   - building=* (≥4 nodes) → BlockPolygon (generic)
//   - natural=wood (≥4 nodes) → BlockPolygon (forest)
//   - landuse=forest (≥4 nodes) → BlockPolygon (forest)
//   - type=multipolygon → BlockPolygon (generic)
//   - None of the above → Skip
func ClassifyTags(tags map[string]string, nodeCount int) Classification {
	// Highway roads/paths → vector network segment
	if _, ok := tags["highway"]; ok {
		return Classification{Block: dsf.BlockVector, SubType: PolyGeneric, Skip: false}
	}

	// Buildings → polygon (requires ≥4 nodes: 3 distinct + closing)
	if _, ok := tags["building"]; ok {
		if nodeCount >= 4 {
			return Classification{Block: dsf.BlockPolygon, SubType: PolyGeneric, Skip: false}
		}
		// Not enough nodes for a valid polygon, skip.
		return Classification{Skip: true}
	}

	// Forest/woodland → polygon with forest sub-type
	if tags["natural"] == "wood" {
		if nodeCount >= 4 {
			return Classification{Block: dsf.BlockPolygon, SubType: PolyForest, Skip: false}
		}
		return Classification{Skip: true}
	}
	if tags["landuse"] == "forest" {
		if nodeCount >= 4 {
			return Classification{Block: dsf.BlockPolygon, SubType: PolyForest, Skip: false}
		}
		return Classification{Skip: true}
	}

	// Multipolygon relations → polygon
	if tags["type"] == "multipolygon" {
		return Classification{Block: dsf.BlockPolygon, SubType: PolyGeneric, Skip: false}
	}

	// No recognized tags → skip
	return Classification{Skip: true}
}
