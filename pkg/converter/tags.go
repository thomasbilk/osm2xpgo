// Package converter transforms OSM elements into DSF building blocks.
package converter

import "github.com/thomasbilk/osm2xpgo/pkg/dsf"

// PolySubType distinguishes polygon sub-classifications.
type PolySubType uint8

const (
	// PolyGeneric represents a generic draped polygon.
	PolyGeneric PolySubType = iota
	// PolyForest represents a forest/woodland polygon (.for file).
	PolyForest
	// PolyField represents agricultural/meadow land (.pol file).
	PolyField
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
//   - highway=* → BlockVector (road network)
//   - building=* (≥4 nodes) → BlockFacade (extruded building)
//   - natural=wood (≥4 nodes) → BlockPolygon (forest)
//   - landuse=forest (≥4 nodes) → BlockPolygon (forest)
//   - landuse=farmland (≥4 nodes) → BlockPolygon (field)
//   - landuse=meadow (≥4 nodes) → BlockPolygon (field)
//   - landuse=grass (≥4 nodes) → BlockPolygon (field)
//   - type=multipolygon → depends on inner tags
//   - None of the above → Skip
func ClassifyTags(tags map[string]string, nodeCount int) Classification {
	// Highway roads/paths → vector network segment.
	if _, ok := tags["highway"]; ok {
		return Classification{Block: dsf.BlockVector, SubType: PolyGeneric, Skip: false}
	}

	// Multipolygon relations → classify by inner tags (checked before area tags
	// because relations don't have a meaningful nodeCount).
	if tags["type"] == "multipolygon" {
		return classifyMultipolygonTags(tags)
	}

	// Buildings → facade (extruded 3D building, requires ≥4 nodes: 3 distinct + closing).
	if _, ok := tags["building"]; ok {
		if nodeCount >= 4 {
			return Classification{Block: dsf.BlockFacade, SubType: PolyGeneric, Skip: false}
		}
		// Not enough nodes for a valid polygon, skip.
		return Classification{Skip: true}
	}

	// Forest/woodland → polygon with forest sub-type (.for file).
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

	// Agricultural fields / meadows → polygon with field sub-type (.pol file).
	switch tags["landuse"] {
	case "farmland", "meadow", "grass", "vineyard", "orchard":
		if nodeCount >= 4 {
			return Classification{Block: dsf.BlockPolygon, SubType: PolyField, Skip: false}
		}
		return Classification{Skip: true}
	}

	// Natural grassland/scrub → field polygon.
	switch tags["natural"] {
	case "grassland", "scrub", "heath":
		if nodeCount >= 4 {
			return Classification{Block: dsf.BlockPolygon, SubType: PolyField, Skip: false}
		}
		return Classification{Skip: true}
	}

	// No recognized tags → skip.
	return Classification{Skip: true}
}

// classifyMultipolygonTags determines the block type for a multipolygon relation
// by inspecting its tags beyond just type=multipolygon.
func classifyMultipolygonTags(tags map[string]string) Classification {
	// Check if it's a building multipolygon.
	if _, ok := tags["building"]; ok {
		return Classification{Block: dsf.BlockFacade, SubType: PolyGeneric, Skip: false}
	}

	// Check for forest.
	if tags["natural"] == "wood" || tags["landuse"] == "forest" {
		return Classification{Block: dsf.BlockPolygon, SubType: PolyForest, Skip: false}
	}

	// Check for fields.
	switch tags["landuse"] {
	case "farmland", "meadow", "grass", "vineyard", "orchard":
		return Classification{Block: dsf.BlockPolygon, SubType: PolyField, Skip: false}
	}
	switch tags["natural"] {
	case "grassland", "scrub", "heath":
		return Classification{Block: dsf.BlockPolygon, SubType: PolyField, Skip: false}
	}

	// Generic multipolygon — skip unless we have another recognized tag.
	return Classification{Skip: true}
}
