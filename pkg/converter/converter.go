// Package converter transforms OSM elements into DSF building blocks.
package converter

import (
	"context"
	"log"
	"math"

	"github.com/paulmach/osm"
	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// Run starts the converter stage. It reads OSM objects, transforms recognized
// features into DSF building blocks, and sends them on the output channel.
// The output channel is closed on completion. Cancellation is respected via ctx.
func Run(ctx context.Context, in <-chan osm.Object, out chan<- dsf.BuildingBlock) error {
	defer close(out)

	for {
		select {
		case <-ctx.Done():
			// Drain input channel without processing.
			for range in {
			}
			return ctx.Err()
		case obj, ok := <-in:
			if !ok {
				// Input channel closed — all objects processed.
				return nil
			}
			if err := processObject(ctx, obj, out); err != nil {
				return err
			}
		}
	}
}

// processObject dispatches an OSM object to the appropriate handler based on type.
func processObject(ctx context.Context, obj osm.Object, out chan<- dsf.BuildingBlock) error {
	switch o := obj.(type) {
	case *osm.Way:
		return processWay(ctx, o, out)
	case *osm.Relation:
		return processRelation(ctx, o, out)
	case *osm.Node:
		// Nodes alone don't produce building blocks in this implementation.
		return nil
	default:
		return nil
	}
}

// processWay handles OSM ways: classifies tags, resolves geometry, and produces building blocks.
func processWay(ctx context.Context, way *osm.Way, out chan<- dsf.BuildingBlock) error {
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

	// Extract coordinates from way nodes.
	coords := make([]dsf.Coordinate, len(way.Nodes))
	for i, node := range way.Nodes {
		coords[i] = dsf.Coordinate{
			Lon: node.Lon,
			Lat: node.Lat,
		}
	}

	// For polygons, enforce CCW winding order.
	if classification.Block == dsf.BlockPolygon {
		coords = EnsureCCW(coords)
	}

	// Assign tile from first node coordinates.
	tile := tileFromCoord(way.Nodes[0].Lat, way.Nodes[0].Lon)

	block := dsf.BuildingBlock{
		Type:   classification.Block,
		Tile:   tile,
		Coords: coords,
		Tags:   tags,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- block:
	}

	return nil
}

// processRelation handles OSM relations: checks for multipolygon type and assembles windings.
func processRelation(ctx context.Context, rel *osm.Relation, out chan<- dsf.BuildingBlock) error {
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

		coords := make([]dsf.Coordinate, len(member.Nodes))
		for i, node := range member.Nodes {
			coords[i] = dsf.Coordinate{
				Lon: node.Lon,
				Lat: node.Lat,
			}
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
