// Package converter transforms OSM elements into DSF building blocks.
package converter

import (
	"context"
	"log"
	"math"
	"strconv"

	"github.com/paulmach/osm"
	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// Run starts the converter stage. It reads OSM objects, transforms recognized
// features into DSF building blocks, and sends them on the output channel.
// The output channel is closed on completion. Cancellation is respected via ctx.
func Run(ctx context.Context, in <-chan osm.Object, out chan<- dsf.BuildingBlock) error {
	defer close(out)

	nodeIndex := make(map[osm.NodeID]dsf.Coordinate)
	var pendingWays []*osm.Way
	var pendingRelations []*osm.Relation

	for {
		select {
		case <-ctx.Done():
			// Drain input channel without processing.
			for range in {
			}
			return ctx.Err()
		case obj, ok := <-in:
			if !ok {
				// Input closed — process deferred ways/relations once all nodes are known.
				if err := processDeferredWays(ctx, pendingWays, nodeIndex, out); err != nil {
					return err
				}
				if err := processDeferredRelations(ctx, pendingRelations, nodeIndex, out); err != nil {
					return err
				}
				return nil
			}

			if err := collectObject(obj, nodeIndex, &pendingWays, &pendingRelations); err != nil {
				return err
			}
		}
	}
}

// collectObject records nodes for coordinate resolution and defers ways/relations
// until all nodes have been observed.
func collectObject(obj osm.Object, nodeIndex map[osm.NodeID]dsf.Coordinate, pendingWays *[]*osm.Way, pendingRelations *[]*osm.Relation) error {
	switch o := obj.(type) {
	case *osm.Node:
		nodeIndex[o.ID] = dsf.Coordinate{Lon: o.Lon, Lat: o.Lat}
		return nil
	case *osm.Way:
		*pendingWays = append(*pendingWays, o)
		return nil
	case *osm.Relation:
		*pendingRelations = append(*pendingRelations, o)
		return nil
	default:
		return nil
	}
}

func processDeferredWays(ctx context.Context, ways []*osm.Way, nodeIndex map[osm.NodeID]dsf.Coordinate, out chan<- dsf.BuildingBlock) error {
	for _, way := range ways {
		if err := processWay(ctx, way, nodeIndex, out); err != nil {
			return err
		}
	}
	return nil
}

func processDeferredRelations(ctx context.Context, rels []*osm.Relation, nodeIndex map[osm.NodeID]dsf.Coordinate, out chan<- dsf.BuildingBlock) error {
	for _, rel := range rels {
		if err := processRelation(ctx, rel, nodeIndex, out); err != nil {
			return err
		}
	}
	return nil
}

// processWay handles OSM ways: classifies tags, resolves geometry, and produces building blocks.
func processWay(ctx context.Context, way *osm.Way, nodeIndex map[osm.NodeID]dsf.Coordinate, out chan<- dsf.BuildingBlock) error {
	// Skip ways with fewer than 2 nodes.
	if len(way.Nodes) < 2 {
		log.Printf("converter: skipping way %d: fewer than 2 nodes", way.ID)
		return nil
	}

	tags := way.TagMap()
	classification := ClassifyTags(tags, len(way.Nodes))

	if classification.Skip {
		return nil
	}

	coords, complete := resolveWayNodes(way.Nodes, nodeIndex)
	if !complete {
		log.Printf("converter: skipping way %d: unresolved node references", way.ID)
		return nil
	}

	// For polygons and facades, enforce CCW winding order.
	if classification.Block == dsf.BlockPolygon || classification.Block == dsf.BlockFacade {
		coords = EnsureCCW(coords)
	}

	// Assign tile from resolved first coordinate.
	tile := tileFromCoord(coords[0].Lat, coords[0].Lon)

	block := dsf.BuildingBlock{
		Type:   classification.Block,
		Tile:   tile,
		Coords: coords,
		Tags:   tags,
	}
	populateDefinitionFields(&block, classification, tags)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- block:
	}

	return nil
}

// processRelation handles OSM relations: checks for multipolygon type and assembles windings.
func processRelation(ctx context.Context, rel *osm.Relation, nodeIndex map[osm.NodeID]dsf.Coordinate, out chan<- dsf.BuildingBlock) error {
	tags := rel.TagMap()

	// Only process multipolygon relations.
	if tags["type"] != "multipolygon" {
		return nil
	}

	classification := ClassifyTags(tags, 4) // multipolygon always has enough nodes conceptually

	if classification.Skip {
		return nil
	}

	// Assemble outer and inner rings from relation members.
	var outerRings [][]dsf.Coordinate
	var innerRings [][]dsf.Coordinate

	for _, member := range rel.Members {
		if member.Type != osm.TypeWay {
			continue
		}
		if len(member.Nodes) == 0 {
			continue
		}

		coords, complete := resolveWayNodes(member.Nodes, nodeIndex)
		if !complete {
			continue
		}

		switch member.Role {
		case "outer":
			outerRings = append(outerRings, coords)
		case "inner":
			innerRings = append(innerRings, coords)
		}
	}

	// If no outer rings, skip this relation.
	if len(outerRings) == 0 {
		return nil
	}

	// Assemble multipolygon with correct winding orders.
	windings := AssembleMultipolygon(outerRings, innerRings)

	// Assign tile from first coordinate of first outer ring.
	firstCoord := windings[0][0]
	tile := tileFromCoord(firstCoord.Lat, firstCoord.Lon)

	block := dsf.BuildingBlock{
		Type:     classification.Block,
		Tile:     tile,
		Windings: windings,
		Tags:     tags,
	}
	populateDefinitionFields(&block, classification, tags)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- block:
	}

	return nil
}

// tileFromCoord computes the TileCoord for a given latitude and longitude.
// The tile is the 1×1 degree cell whose south-west corner is at (floor(lat), floor(lon)).
func tileFromCoord(lat, lon float64) dsf.TileCoord {
	return dsf.TileCoord{
		Lat: int(math.Floor(lat)),
		Lon: int(math.Floor(lon)),
	}
}

func resolveWayNodes(nodes []osm.WayNode, nodeIndex map[osm.NodeID]dsf.Coordinate) ([]dsf.Coordinate, bool) {
	coords := make([]dsf.Coordinate, len(nodes))
	complete := true

	for i, node := range nodes {
		if c, ok := nodeIndex[node.ID]; ok {
			coords[i] = c
			continue
		}

		coords[i] = dsf.Coordinate{Lon: node.Lon, Lat: node.Lat}
		if node.Lon == 0 && node.Lat == 0 {
			complete = false
		}
	}

	return coords, complete
}

func populateDefinitionFields(block *dsf.BuildingBlock, classification Classification, tags map[string]string) {
	switch classification.Block {
	case dsf.BlockVector:
		block.DefPath = vectorDefPath()
		block.SubType = vectorSubType(tags)

	case dsf.BlockPolygon:
		block.DefPath = polygonDefPath(classification)
		block.Param = polygonParam(classification, tags)

	case dsf.BlockFacade:
		block.DefPath = facadeDefPath(tags)
		block.Param = facadeHeight(tags)

	case dsf.BlockObject:
		block.DefPath = "lib/g10/objects/default.obj"
	}
}

// vectorDefPath returns the X-Plane road network definition path.
// X-Plane uses a single .net file per DSF with subtypes for different road classes.
func vectorDefPath() string {
	// X-Plane's default road network from the built-in library.
	return "lib/g10/roads.net"
}

// vectorSubType maps OSM highway tags to X-Plane road subtypes.
// These subtypes correspond to visual styles defined in the .net file:
//
//	1 = highway/motorway (wide, divided)
//	2 = primary/trunk road
//	3 = secondary/tertiary road
//	4 = residential/local street
//	5 = service/track
//	6 = footway/path/cycleway
func vectorSubType(tags map[string]string) uint8 {
	switch tags["highway"] {
	case "motorway", "motorway_link":
		return 1
	case "trunk", "trunk_link":
		return 2
	case "primary", "primary_link":
		return 3
	case "secondary", "secondary_link":
		return 4
	case "tertiary", "tertiary_link":
		return 5
	case "residential", "living_street", "unclassified":
		return 6
	case "service":
		return 7
	case "track":
		return 8
	case "footway", "path", "cycleway", "pedestrian", "steps":
		return 9
	default:
		return 6 // Default to residential
	}
}

// polygonDefPath returns the definition path for polygon features.
func polygonDefPath(classification Classification) string {
	switch classification.SubType {
	case PolyForest:
		// X-Plane forest definition — uses the default deciduous/mixed forest.
		return "lib/g10/forests/autogen_tree.for"
	case PolyField:
		// Draped polygon for agricultural/grass areas.
		return "lib/g10/terrain10/apt_grass.pol"
	default:
		return "lib/g10/terrain10/apt_grass.pol"
	}
}

// polygonParam returns the polygon parameter value.
// For forests: density 0-255 (we use 200 for good coverage).
// For draped polygons (.pol): texture heading in degrees (0).
func polygonParam(classification Classification, tags map[string]string) uint16 {
	switch classification.SubType {
	case PolyForest:
		// Forest density: 200 out of 255 for realistic coverage.
		// The fill mode is encoded in upper bits: 0=solid fill.
		return 200
	case PolyField:
		// Draped polygon heading: 0 degrees.
		return 0
	default:
		return 0
	}
}

// facadeDefPath returns the facade definition path for buildings.
// X-Plane uses .fac files for extruded buildings.
func facadeDefPath(tags map[string]string) string {
	// Choose facade type based on building characteristics.
	buildingType := tags["building"]
	switch buildingType {
	case "industrial", "warehouse":
		return "lib/g10/autogen/Industrial_1.fac"
	case "commercial", "retail", "office":
		return "lib/g10/autogen/OfficeMed_1.fac"
	case "church", "cathedral", "chapel":
		return "lib/g10/autogen/Church_1.fac"
	case "residential", "apartments", "house", "detached", "terrace":
		return "lib/g10/autogen/ResLo_1.fac"
	default:
		// Generic building facade.
		return "lib/g10/autogen/ResLo_1.fac"
	}
}

// facadeHeight returns the height parameter for a building facade in meters.
// The param for facades encodes the building height.
func facadeHeight(tags map[string]string) uint16 {
	// Try building:levels first.
	if levelsStr, ok := tags["building:levels"]; ok {
		if levels, err := strconv.Atoi(levelsStr); err == nil && levels > 0 {
			height := levels * 3 // Approximate 3 meters per floor.
			if height > 200 {
				height = 200
			}
			return uint16(height)
		}
	}

	// Try explicit height tag.
	if heightStr, ok := tags["height"]; ok {
		if h, err := strconv.ParseFloat(heightStr, 64); err == nil && h > 0 {
			if h > 200 {
				h = 200
			}
			return uint16(h)
		}
	}

	// Default height based on building type.
	switch tags["building"] {
	case "industrial", "warehouse":
		return 8
	case "commercial", "office":
		return 15
	case "apartments":
		return 18
	case "church", "cathedral":
		return 20
	default:
		return 9 // ~3 floors for generic residential
	}
}
