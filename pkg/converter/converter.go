// Package converter transforms OSM elements into DSF building blocks.
package converter

import (
	"context"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/paulmach/osm"
	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// Run starts the converter stage. It reads OSM objects, transforms recognized
// features into DSF building blocks, and sends them on the output channel.
// The output channel is closed on completion. Cancellation is respected via ctx.
func Run(ctx context.Context, in <-chan osm.Object, out chan<- dsf.BuildingBlock) error {
	defer close(out)

	nodeIndex := make(map[osm.NodeID]dsf.Coordinate)
	wayIndex := make(map[osm.WayID]*osm.Way)
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
				if err := processDeferredRelations(ctx, pendingRelations, nodeIndex, wayIndex, out); err != nil {
					return err
				}
				return nil
			}

			if err := collectObject(obj, nodeIndex, &pendingWays, &pendingRelations, wayIndex); err != nil {
				return err
			}
		}
	}
}

// collectObject records nodes for coordinate resolution and defers ways/relations
// until all nodes have been observed.
func collectObject(obj osm.Object, nodeIndex map[osm.NodeID]dsf.Coordinate, pendingWays *[]*osm.Way, pendingRelations *[]*osm.Relation, wayIndex map[osm.WayID]*osm.Way) error {
	switch o := obj.(type) {
	case *osm.Node:
		nodeIndex[o.ID] = dsf.Coordinate{Lon: o.Lon, Lat: o.Lat}
		return nil
	case *osm.Way:
		*pendingWays = append(*pendingWays, o)
		wayIndex[o.ID] = o
		return nil
	case *osm.Relation:
		*pendingRelations = append(*pendingRelations, o)
		return nil
	default:
		return nil
	}
}

func processDeferredWays(ctx context.Context, ways []*osm.Way, nodeIndex map[osm.NodeID]dsf.Coordinate, out chan<- dsf.BuildingBlock) error {
	// Build a node usage count across all vector ways to identify junctions.
	// A junction is any node referenced by more than one vector way.
	nodeCount := make(map[osm.NodeID]int)
	var vectorWays []*osm.Way
	var otherWays []*osm.Way

	for _, way := range ways {
		tags := way.TagMap()
		classification := ClassifyTags(tags, len(way.Nodes))
		if classification.Skip {
			continue
		}
		if classification.Block == dsf.BlockVector {
			vectorWays = append(vectorWays, way)
			for _, n := range way.Nodes {
				nodeCount[n.ID]++
			}
		} else {
			otherWays = append(otherWays, way)
		}
	}

	// Process vector ways with junction splitting.
	for _, way := range vectorWays {
		if err := processVectorWay(ctx, way, nodeIndex, nodeCount, out); err != nil {
			return err
		}
	}

	// Process non-vector ways normally.
	for _, way := range otherWays {
		if err := processWay(ctx, way, nodeIndex, out); err != nil {
			return err
		}
	}
	return nil
}

func processDeferredRelations(ctx context.Context, rels []*osm.Relation, nodeIndex map[osm.NodeID]dsf.Coordinate, wayIndex map[osm.WayID]*osm.Way, out chan<- dsf.BuildingBlock) error {
	for _, rel := range rels {
		if err := processRelation(ctx, rel, nodeIndex, wayIndex, out); err != nil {
			return err
		}
	}
	return nil
}

// processVectorWay handles a road/path way by splitting it at junction nodes.
// A junction is any interior node that is shared by more than one vector way.
// This produces proper network topology where segments connect at shared nodes.
func processVectorWay(ctx context.Context, way *osm.Way, nodeIndex map[osm.NodeID]dsf.Coordinate, nodeCount map[osm.NodeID]int, out chan<- dsf.BuildingBlock) error {
	if len(way.Nodes) < 2 {
		return nil
	}

	tags := way.TagMap()
	classification := ClassifyTags(tags, len(way.Nodes))

	coords, complete := resolveWayNodes(way.Nodes, nodeIndex)
	if !complete {
		log.Printf("converter: skipping way %d: unresolved node references", way.ID)
		return nil
	}

	// Find split points: interior nodes (not first/last) shared by multiple ways.
	var splitIndices []int
	for i := 1; i < len(way.Nodes)-1; i++ {
		if nodeCount[way.Nodes[i].ID] > 1 {
			splitIndices = append(splitIndices, i)
		}
	}

	// If no junctions, emit as a single segment (same as before).
	if len(splitIndices) == 0 {
		return emitVectorBlock(ctx, coords, tags, classification, out)
	}

	// Split at junction nodes, including the junction node in both segments.
	start := 0
	for _, splitIdx := range splitIndices {
		segment := coords[start : splitIdx+1]
		if len(segment) >= 2 {
			if err := emitVectorBlock(ctx, segment, tags, classification, out); err != nil {
				return err
			}
		}
		start = splitIdx
	}
	// Final segment from last split to end.
	segment := coords[start:]
	if len(segment) >= 2 {
		if err := emitVectorBlock(ctx, segment, tags, classification, out); err != nil {
			return err
		}
	}

	return nil
}

// emitVectorBlock creates and sends a vector building block for a road segment.
func emitVectorBlock(ctx context.Context, coords []dsf.Coordinate, tags map[string]string, classification Classification, out chan<- dsf.BuildingBlock) error {
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

	// Skip buildings that are too small or have invalid geometry.
	if classification.Block == dsf.BlockFacade && shouldSkipBuilding(coords) {
		return nil
	}

	// Assign tile from centroid for polygons/facades (more correct than first coord).
	var tile dsf.TileCoord
	if classification.Block == dsf.BlockPolygon || classification.Block == dsf.BlockFacade {
		c := centroidOfRing(coords)
		tile = tileFromCoord(c.Lat, c.Lon)
	} else {
		tile = tileFromCoord(coords[0].Lat, coords[0].Lon)
	}

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
// It uses the wayIndex to resolve member way geometry and chains unclosed way fragments
// into closed rings.
func processRelation(ctx context.Context, rel *osm.Relation, nodeIndex map[osm.NodeID]dsf.Coordinate, wayIndex map[osm.WayID]*osm.Way, out chan<- dsf.BuildingBlock) error {
	tags := rel.TagMap()

	// Only process multipolygon relations.
	if tags["type"] != "multipolygon" {
		return nil
	}

	classification := ClassifyTags(tags, 4) // multipolygon always has enough nodes conceptually

	if classification.Skip {
		return nil
	}

	// Collect resolved coordinate sequences for outer and inner members.
	var outerFragments [][]dsf.Coordinate
	var innerFragments [][]dsf.Coordinate

	for _, member := range rel.Members {
		if member.Type != osm.TypeWay {
			continue
		}

		// Try to resolve the way geometry.
		var coords []dsf.Coordinate
		var complete bool

		// First try: use member.Nodes if populated (some PBF readers include them).
		if len(member.Nodes) > 0 {
			coords, complete = resolveWayNodes(member.Nodes, nodeIndex)
		}

		// Second try: look up the way in our wayIndex.
		if !complete || len(coords) == 0 {
			if way, ok := wayIndex[osm.WayID(member.Ref)]; ok && len(way.Nodes) > 0 {
				coords, complete = resolveWayNodes(way.Nodes, nodeIndex)
			}
		}

		if !complete || len(coords) < 2 {
			continue
		}

		switch member.Role {
		case "outer":
			outerFragments = append(outerFragments, coords)
		case "inner":
			innerFragments = append(innerFragments, coords)
		default:
			// Treat untagged members as outer (common in OSM data).
			outerFragments = append(outerFragments, coords)
		}
	}

	// Chain fragments into closed rings.
	outerRings := chainRings(outerFragments)
	innerRings := chainRings(innerFragments)

	// If no outer rings could be assembled, skip this relation.
	if len(outerRings) == 0 {
		return nil
	}

	// Assemble multipolygon with correct winding orders.
	windings := AssembleMultipolygon(outerRings, innerRings)

	// Assign tile from centroid of first outer ring for better tile assignment.
	firstCoord := centroidOfRing(windings[0])
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

// chainRings merges a set of potentially unclosed coordinate sequences into
// closed rings by connecting fragments whose endpoints match. This handles the
// common OSM case where a multipolygon ring is split across multiple ways.
func chainRings(fragments [][]dsf.Coordinate) [][]dsf.Coordinate {
	if len(fragments) == 0 {
		return nil
	}

	used := make([]bool, len(fragments))
	var rings [][]dsf.Coordinate

	for i := range fragments {
		if used[i] || len(fragments[i]) == 0 {
			continue
		}

		// Start building a ring from this fragment.
		ring := make([]dsf.Coordinate, len(fragments[i]))
		copy(ring, fragments[i])
		used[i] = true

		// Try to extend the ring until it's closed or no more extensions found.
		for !isRingClosed(ring) {
			extended := false
			tail := ring[len(ring)-1]

			for j := range fragments {
				if used[j] || len(fragments[j]) == 0 {
					continue
				}
				frag := fragments[j]

				// Try connecting tail to the start of this fragment.
				if coordsClose(tail, frag[0]) {
					ring = append(ring, frag[1:]...)
					used[j] = true
					extended = true
					break
				}

				// Try connecting tail to the reversed fragment (end matches our tail).
				if coordsClose(tail, frag[len(frag)-1]) {
					reversed := reverseCoords(frag)
					ring = append(ring, reversed[1:]...)
					used[j] = true
					extended = true
					break
				}
			}

			if !extended {
				break
			}
		}

		// Force-close the ring if it's still not closed but we can't extend further.
		if !isRingClosed(ring) && len(ring) > 2 {
			ring = append(ring, ring[0])
		}

		if len(ring) >= 4 { // Need at least 3 distinct points + closing point.
			rings = append(rings, ring)
		}
	}

	return rings
}

// isRingClosed returns true if the first and last coordinates are the same point.
func isRingClosed(ring []dsf.Coordinate) bool {
	if len(ring) < 2 {
		return false
	}
	return coordsClose(ring[0], ring[len(ring)-1])
}

// coordsClose returns true if two coordinates are within a small tolerance (1e-8 degrees).
func coordsClose(a, b dsf.Coordinate) bool {
	const eps = 1e-8
	dLon := a.Lon - b.Lon
	dLat := a.Lat - b.Lat
	if dLon < 0 {
		dLon = -dLon
	}
	if dLat < 0 {
		dLat = -dLat
	}
	return dLon < eps && dLat < eps
}

// reverseCoords returns a new slice with coordinates in reversed order.
func reverseCoords(coords []dsf.Coordinate) []dsf.Coordinate {
	n := len(coords)
	rev := make([]dsf.Coordinate, n)
	for i, c := range coords {
		rev[n-1-i] = c
	}
	return rev
}

// centroidOfRing computes the centroid of a coordinate ring.
func centroidOfRing(ring []dsf.Coordinate) dsf.Coordinate {
	if len(ring) == 0 {
		return dsf.Coordinate{}
	}
	var sumLon, sumLat float64
	for _, c := range ring {
		sumLon += c.Lon
		sumLat += c.Lat
	}
	n := float64(len(ring))
	return dsf.Coordinate{Lon: sumLon / n, Lat: sumLat / n}
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
		block.DefPath = facadeDefPath(tags, block.Coords)
		block.Param = facadeHeight(tags, block.Coords)

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
// Selection considers building type, estimated height, and footprint area.
func facadeDefPath(tags map[string]string, coords []dsf.Coordinate) string {
	height := estimateBuildingHeight(tags, coords)
	buildingType := tags["building"]

	// Special building types with dedicated facades.
	switch buildingType {
	case "church", "cathedral", "chapel":
		return "lib/g10/autogen/Church_1.fac"
	case "industrial", "warehouse":
		if height > 12 {
			return "lib/g10/autogen/Industrial_2.fac"
		}
		return "lib/g10/autogen/Industrial_1.fac"
	case "commercial", "retail", "office":
		if height > 30 {
			return "lib/g10/autogen/OfficeHi_1.fac"
		}
		if height > 15 {
			return "lib/g10/autogen/OfficeMed_1.fac"
		}
		return "lib/g10/autogen/OfficeLo_1.fac"
	case "apartments":
		if height > 30 {
			return "lib/g10/autogen/ResHi_1.fac"
		}
		if height > 15 {
			return "lib/g10/autogen/ResMed_1.fac"
		}
		return "lib/g10/autogen/ResLo_1.fac"
	}

	// For generic or residential buildings, classify by height.
	if height > 30 {
		return "lib/g10/autogen/ResHi_1.fac"
	}
	if height > 15 {
		return "lib/g10/autogen/ResMed_1.fac"
	}
	return "lib/g10/autogen/ResLo_1.fac"
}

// shouldSkipBuilding returns true if a building should be filtered out based on
// its geometry: too few edges, too small area, etc.
func shouldSkipBuilding(coords []dsf.Coordinate) bool {
	// Strip closing duplicate for edge count.
	pts := coords
	if len(pts) > 1 && pts[0].Lon == pts[len(pts)-1].Lon && pts[0].Lat == pts[len(pts)-1].Lat {
		pts = pts[:len(pts)-1]
	}

	// Minimum 3 distinct edges.
	if len(pts) < 3 {
		return true
	}

	// Maximum 200 edges (overly complex geometry is likely a data issue).
	if len(pts) > 200 {
		return true
	}

	// Minimum area: ~25 m² (very small structures are visual noise).
	areaDeg2 := polygonAreaDeg2(coords)
	if areaDeg2 < 0 {
		areaDeg2 = -areaDeg2
	}
	// At ~45° latitude: 1 degree ≈ 111km, so 1e-9 deg² ≈ 12 m².
	// Use 2e-9 as minimum (~25 m²).
	if areaDeg2 < 2e-9 {
		return true
	}

	return false
}

// estimateBuildingHeight estimates the building height in meters from tags and geometry.
// Priority: explicit height → building:levels → heuristic based on area and type.
func estimateBuildingHeight(tags map[string]string, coords []dsf.Coordinate) int {
	// Try explicit height tag.
	if heightStr, ok := tags["height"]; ok {
		h := parseHeight(heightStr)
		if h > 0 {
			return h
		}
	}

	// Try building:levels.
	if levelsStr, ok := tags["building:levels"]; ok {
		if levels, err := strconv.Atoi(levelsStr); err == nil && levels > 0 {
			return levels * 3 // 3 meters per level.
		}
	}

	// Heuristic based on building type and footprint area.
	buildingType := tags["building"]
	areaDeg2 := polygonAreaDeg2(coords)
	if areaDeg2 < 0 {
		areaDeg2 = -areaDeg2
	}

	switch buildingType {
	case "industrial", "warehouse":
		return 8
	case "church", "cathedral":
		return 20
	case "commercial", "office":
		// Larger footprint → likely taller commercial building.
		if areaDeg2 > 1e-7 {
			return 20
		}
		return 12
	case "apartments":
		if areaDeg2 > 5e-8 {
			return 18
		}
		return 12
	case "house", "detached", "semidetached_house", "terrace":
		return 8
	case "garage", "garages":
		return 4
	case "shed", "hut", "cabin":
		return 4
	}

	// Generic building: use footprint area as a rough proxy.
	// Larger footprint buildings are often taller (commercial/institutional).
	if areaDeg2 > 1e-7 {
		return 15
	}
	if areaDeg2 > 3e-8 {
		return 12
	}
	return 9 // Default: ~3 floors residential.
}

// parseHeight parses a height string that may have units (e.g., "12", "15m", "50ft").
func parseHeight(s string) int {
	s = strings.TrimSpace(s)
	isFeet := false
	if strings.HasSuffix(s, "ft") {
		isFeet = true
		s = strings.TrimSuffix(s, "ft")
		s = strings.TrimSpace(s)
	}
	s = strings.TrimSuffix(s, "m")
	s = strings.TrimSuffix(s, "M")
	s = strings.TrimSpace(s)

	h, err := strconv.ParseFloat(s, 64)
	if err != nil || h <= 0 {
		return 0
	}
	if isFeet {
		h = h * 0.3048
	}
	if h > 200 {
		h = 200
	}
	return int(math.Round(h))
}

// polygonAreaDeg2 computes the signed area of a polygon in degree² using the shoelace formula.
func polygonAreaDeg2(coords []dsf.Coordinate) float64 {
	n := len(coords)
	if n < 3 {
		return 0
	}
	var area float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += coords[i].Lon*coords[j].Lat - coords[j].Lon*coords[i].Lat
	}
	return area * 0.5
}

// facadeHeight returns the height parameter for a building facade in meters.
// The param for facades encodes the building height.
func facadeHeight(tags map[string]string, coords []dsf.Coordinate) uint16 {
	h := estimateBuildingHeight(tags, coords)
	if h <= 0 {
		h = 9
	}
	if h > 200 {
		h = 200
	}
	return uint16(h)
}
