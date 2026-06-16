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

	// For polygons, enforce CCW winding order.
	if classification.Block == dsf.BlockPolygon {
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
		// Classify non-multipolygon relations as well — they might match other tags.
		// However, per the spec, only multipolygon relations produce building blocks.
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
		block.DefPath = vectorDefPath(tags)
		block.SubType = vectorSubType(tags)

	case dsf.BlockPolygon:
		block.DefPath = polygonDefPath(classification, tags)
		block.Param = polygonParam(tags)

	case dsf.BlockObject:
		block.DefPath = "objects/default.obj"
	}
}

func vectorDefPath(tags map[string]string) string {
	switch tags["highway"] {
	case "motorway", "trunk":
		return "roads/highway_major.net"
	case "primary", "secondary", "tertiary":
		return "roads/highway.net"
	case "residential", "service", "living_street":
		return "roads/street.net"
	case "footway", "path", "cycleway", "track":
		return "roads/path.net"
	default:
		return "roads/default.net"
	}
}

func vectorSubType(tags map[string]string) uint8 {
	switch tags["highway"] {
	case "motorway", "trunk":
		return 3
	case "primary", "secondary", "tertiary":
		return 2
	case "residential", "service", "living_street":
		return 1
	case "footway", "path", "cycleway", "track":
		return 4
	default:
		return 0
	}
}

func polygonDefPath(classification Classification, tags map[string]string) string {
	if classification.SubType == PolyForest {
		return "polygons/forest.pol"
	}

	if _, ok := tags["building"]; ok {
		return "polygons/building.pol"
	}

	return "polygons/area.pol"
}

func polygonParam(tags map[string]string) uint16 {
	levelsStr, ok := tags["building:levels"]
	if !ok {
		return 0
	}
	levels, err := strconv.Atoi(levelsStr)
	if err != nil || levels < 0 {
		return 0
	}
	height := levels * 3
	if height > int(^uint16(0)) {
		return ^uint16(0)
	}
	return uint16(height)
}
