package converter

import (
	"context"
	"testing"

	"github.com/paulmach/osm"
	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
	"github.com/thomasbilk/osm2xpgo/pkg/reader"
)

func TestResolveWayNodes_UsesNodeIndex(t *testing.T) {
	nodes := []osm.WayNode{{ID: 100}, {ID: 101}}
	nodeIndex := map[osm.NodeID]dsf.Coordinate{
		100: {Lon: 7.42, Lat: 43.73},
		101: {Lon: 7.43, Lat: 43.74},
	}

	coords, complete := resolveWayNodes(nodes, nodeIndex)
	if !complete {
		t.Fatal("expected complete=true when all nodes are resolved")
	}
	if len(coords) != 2 {
		t.Fatalf("expected 2 coordinates, got %d", len(coords))
	}
	if coords[0].Lon != 7.42 || coords[0].Lat != 43.73 {
		t.Fatalf("unexpected first coordinate: %+v", coords[0])
	}
}

func TestResolveWayNodes_IncompleteWhenMissingAndZero(t *testing.T) {
	nodes := []osm.WayNode{{ID: 100}, {ID: 101, Lat: 0, Lon: 0}}
	nodeIndex := map[osm.NodeID]dsf.Coordinate{
		100: {Lon: 7.42, Lat: 43.73},
	}

	_, complete := resolveWayNodes(nodes, nodeIndex)
	if complete {
		t.Fatal("expected complete=false when a node is unresolved with zero coords")
	}
}

func TestPopulateDefinitionFields_Vector(t *testing.T) {
	b := dsf.BuildingBlock{Type: dsf.BlockVector}
	class := Classification{Block: dsf.BlockVector}
	tags := map[string]string{"highway": "primary"}

	populateDefinitionFields(&b, class, tags)

	if b.DefPath == "" {
		t.Fatal("expected non-empty DefPath for vector")
	}
	if b.DefPath != "roads/highway.net" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.SubType != 2 {
		t.Fatalf("unexpected SubType: %d", b.SubType)
	}
}

func TestPopulateDefinitionFields_PolygonBuildingLevels(t *testing.T) {
	b := dsf.BuildingBlock{Type: dsf.BlockPolygon}
	class := Classification{Block: dsf.BlockPolygon, SubType: PolyGeneric}
	tags := map[string]string{"building": "yes", "building:levels": "5"}

	populateDefinitionFields(&b, class, tags)

	if b.DefPath != "polygons/building.pol" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.Param != 15 {
		t.Fatalf("unexpected Param: %d", b.Param)
	}
}

func TestPopulateDefinitionFields_PolygonForest(t *testing.T) {
	b := dsf.BuildingBlock{Type: dsf.BlockPolygon}
	class := Classification{Block: dsf.BlockPolygon, SubType: PolyForest}
	tags := map[string]string{"landuse": "forest"}

	populateDefinitionFields(&b, class, tags)

	if b.DefPath != "polygons/forest.pol" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.Param != 0 {
		t.Fatalf("unexpected Param: %d", b.Param)
	}
}

func TestResolveWayNodes_MonacoSample(t *testing.T) {
	const pbfPath = "../../monaco-260615.osm.pbf"
	out := make(chan osm.Object, 1024)
	errCh := make(chan error, 1)

	go func() {
		errCh <- reader.Run(context.Background(), pbfPath, out)
	}()

	nodeIndex := make(map[osm.NodeID]dsf.Coordinate)
	var firstWay *osm.Way

	for obj := range out {
		switch o := obj.(type) {
		case *osm.Node:
			nodeIndex[o.ID] = dsf.Coordinate{Lon: o.Lon, Lat: o.Lat}
		case *osm.Way:
			if firstWay == nil {
				firstWay = o
			}
		}
	}

	if err := <-errCh; err != nil {
		t.Fatalf("reader.Run failed: %v", err)
	}
	if firstWay == nil {
		t.Fatal("expected at least one way in sample")
	}
	if len(firstWay.Nodes) == 0 {
		t.Fatal("first way has no nodes")
	}

	coords, complete := resolveWayNodes(firstWay.Nodes, nodeIndex)
	if !complete {
		t.Fatal("expected first Monaco way to resolve all nodes")
	}
	if coords[0].Lat == 0 && coords[0].Lon == 0 {
		t.Fatalf("expected non-zero first coordinate, got %+v", coords[0])
	}
}
