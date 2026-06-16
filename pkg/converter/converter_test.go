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
	if b.DefPath != "lib/g10/roads.net" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.SubType != 3 {
		t.Fatalf("unexpected SubType for primary: %d (expected 3)", b.SubType)
	}
}

func TestPopulateDefinitionFields_Facade(t *testing.T) {
	b := dsf.BuildingBlock{Type: dsf.BlockFacade}
	class := Classification{Block: dsf.BlockFacade, SubType: PolyGeneric}
	tags := map[string]string{"building": "yes", "building:levels": "5"}

	populateDefinitionFields(&b, class, tags)

	if b.DefPath != "lib/g10/autogen/ResLo_1.fac" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.Param != 15 {
		t.Fatalf("unexpected Param (height): %d (expected 15 for 5 levels)", b.Param)
	}
}

func TestPopulateDefinitionFields_FacadeIndustrial(t *testing.T) {
	b := dsf.BuildingBlock{Type: dsf.BlockFacade}
	class := Classification{Block: dsf.BlockFacade, SubType: PolyGeneric}
	tags := map[string]string{"building": "industrial"}

	populateDefinitionFields(&b, class, tags)

	if b.DefPath != "lib/g10/autogen/Industrial_1.fac" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.Param != 8 {
		t.Fatalf("unexpected default height for industrial: %d (expected 8)", b.Param)
	}
}

func TestPopulateDefinitionFields_PolygonForest(t *testing.T) {
	b := dsf.BuildingBlock{Type: dsf.BlockPolygon}
	class := Classification{Block: dsf.BlockPolygon, SubType: PolyForest}
	tags := map[string]string{"landuse": "forest"}

	populateDefinitionFields(&b, class, tags)

	if b.DefPath != "lib/g10/forests/autogen_tree.for" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.Param != 200 {
		t.Fatalf("unexpected Param (density): %d (expected 200)", b.Param)
	}
}

func TestPopulateDefinitionFields_PolygonField(t *testing.T) {
	b := dsf.BuildingBlock{Type: dsf.BlockPolygon}
	class := Classification{Block: dsf.BlockPolygon, SubType: PolyField}
	tags := map[string]string{"landuse": "farmland"}

	populateDefinitionFields(&b, class, tags)

	if b.DefPath != "lib/g10/terrain10/apt_grass.pol" {
		t.Fatalf("unexpected DefPath: %q", b.DefPath)
	}
	if b.Param != 0 {
		t.Fatalf("unexpected Param (heading): %d (expected 0)", b.Param)
	}
}

func TestVectorSubType_RoadClassification(t *testing.T) {
	tests := []struct {
		highway  string
		expected uint8
	}{
		{"motorway", 1},
		{"motorway_link", 1},
		{"trunk", 2},
		{"primary", 3},
		{"secondary", 4},
		{"tertiary", 5},
		{"residential", 6},
		{"service", 7},
		{"track", 8},
		{"footway", 9},
		{"path", 9},
		{"cycleway", 9},
	}

	for _, tt := range tests {
		tags := map[string]string{"highway": tt.highway}
		got := vectorSubType(tags)
		if got != tt.expected {
			t.Errorf("highway=%s: expected subtype %d, got %d", tt.highway, tt.expected, got)
		}
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
