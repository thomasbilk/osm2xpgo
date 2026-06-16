package converter

import (
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

func TestClassifyTags_Highway(t *testing.T) {
	tags := map[string]string{"highway": "residential"}
	got := ClassifyTags(tags, 5)

	if got.Skip {
		t.Fatal("expected highway to not be skipped")
	}
	if got.Block != dsf.BlockVector {
		t.Errorf("expected BlockVector, got %v", got.Block)
	}
}

func TestClassifyTags_HighwayAnyValue(t *testing.T) {
	// Any highway value should produce a vector.
	for _, val := range []string{"motorway", "trunk", "primary", "footway", "path", "service"} {
		tags := map[string]string{"highway": val}
		got := ClassifyTags(tags, 2)
		if got.Skip || got.Block != dsf.BlockVector {
			t.Errorf("highway=%s: expected BlockVector, got skip=%v block=%v", val, got.Skip, got.Block)
		}
	}
}

func TestClassifyTags_Building(t *testing.T) {
	tags := map[string]string{"building": "yes"}
	got := ClassifyTags(tags, 5)

	if got.Skip {
		t.Fatal("expected building with 5 nodes to not be skipped")
	}
	if got.Block != dsf.BlockFacade {
		t.Errorf("expected BlockFacade, got %v", got.Block)
	}
}

func TestClassifyTags_BuildingTooFewNodes(t *testing.T) {
	tags := map[string]string{"building": "yes"}
	got := ClassifyTags(tags, 3)

	if !got.Skip {
		t.Fatal("expected building with 3 nodes to be skipped")
	}
}

func TestClassifyTags_BuildingExactly4Nodes(t *testing.T) {
	tags := map[string]string{"building": "house"}
	got := ClassifyTags(tags, 4)

	if got.Skip {
		t.Fatal("expected building with exactly 4 nodes to not be skipped")
	}
	if got.Block != dsf.BlockFacade {
		t.Errorf("expected BlockFacade, got %v", got.Block)
	}
}

func TestClassifyTags_NaturalWood(t *testing.T) {
	tags := map[string]string{"natural": "wood"}
	got := ClassifyTags(tags, 10)

	if got.Skip {
		t.Fatal("expected natural=wood to not be skipped")
	}
	if got.Block != dsf.BlockPolygon {
		t.Errorf("expected BlockPolygon, got %v", got.Block)
	}
	if got.SubType != PolyForest {
		t.Errorf("expected PolyForest subtype, got %v", got.SubType)
	}
}

func TestClassifyTags_NaturalWoodTooFewNodes(t *testing.T) {
	tags := map[string]string{"natural": "wood"}
	got := ClassifyTags(tags, 3)

	if !got.Skip {
		t.Fatal("expected natural=wood with 3 nodes to be skipped")
	}
}

func TestClassifyTags_LanduseForest(t *testing.T) {
	tags := map[string]string{"landuse": "forest"}
	got := ClassifyTags(tags, 6)

	if got.Skip {
		t.Fatal("expected landuse=forest to not be skipped")
	}
	if got.Block != dsf.BlockPolygon {
		t.Errorf("expected BlockPolygon, got %v", got.Block)
	}
	if got.SubType != PolyForest {
		t.Errorf("expected PolyForest subtype, got %v", got.SubType)
	}
}

func TestClassifyTags_LanduseForestTooFewNodes(t *testing.T) {
	tags := map[string]string{"landuse": "forest"}
	got := ClassifyTags(tags, 2)

	if !got.Skip {
		t.Fatal("expected landuse=forest with 2 nodes to be skipped")
	}
}

func TestClassifyTags_LanduseFarmland(t *testing.T) {
	tags := map[string]string{"landuse": "farmland"}
	got := ClassifyTags(tags, 6)

	if got.Skip {
		t.Fatal("expected landuse=farmland to not be skipped")
	}
	if got.Block != dsf.BlockPolygon {
		t.Errorf("expected BlockPolygon, got %v", got.Block)
	}
	if got.SubType != PolyField {
		t.Errorf("expected PolyField subtype, got %v", got.SubType)
	}
}

func TestClassifyTags_LanduseMeadow(t *testing.T) {
	tags := map[string]string{"landuse": "meadow"}
	got := ClassifyTags(tags, 5)

	if got.Skip {
		t.Fatal("expected landuse=meadow to not be skipped")
	}
	if got.Block != dsf.BlockPolygon {
		t.Errorf("expected BlockPolygon, got %v", got.Block)
	}
	if got.SubType != PolyField {
		t.Errorf("expected PolyField subtype, got %v", got.SubType)
	}
}

func TestClassifyTags_NaturalGrassland(t *testing.T) {
	tags := map[string]string{"natural": "grassland"}
	got := ClassifyTags(tags, 5)

	if got.Skip {
		t.Fatal("expected natural=grassland to not be skipped")
	}
	if got.Block != dsf.BlockPolygon {
		t.Errorf("expected BlockPolygon, got %v", got.Block)
	}
	if got.SubType != PolyField {
		t.Errorf("expected PolyField subtype, got %v", got.SubType)
	}
}

func TestClassifyTags_MultipolygonForest(t *testing.T) {
	tags := map[string]string{"type": "multipolygon", "natural": "wood"}
	got := ClassifyTags(tags, 0)

	if got.Skip {
		t.Fatal("expected multipolygon forest to not be skipped")
	}
	if got.Block != dsf.BlockPolygon {
		t.Errorf("expected BlockPolygon, got %v", got.Block)
	}
	if got.SubType != PolyForest {
		t.Errorf("expected PolyForest subtype, got %v", got.SubType)
	}
}

func TestClassifyTags_MultipolygonBuilding(t *testing.T) {
	tags := map[string]string{"type": "multipolygon", "building": "yes"}
	got := ClassifyTags(tags, 0)

	if got.Skip {
		t.Fatal("expected multipolygon building to not be skipped")
	}
	if got.Block != dsf.BlockFacade {
		t.Errorf("expected BlockFacade, got %v", got.Block)
	}
}

func TestClassifyTags_MultipolygonGeneric(t *testing.T) {
	// A multipolygon without recognized inner tags should be skipped.
	tags := map[string]string{"type": "multipolygon"}
	got := ClassifyTags(tags, 0)

	if !got.Skip {
		t.Fatal("expected generic multipolygon without recognized tags to be skipped")
	}
}

func TestClassifyTags_Unrecognized(t *testing.T) {
	tags := map[string]string{"amenity": "restaurant", "name": "Pizza Place"}
	got := ClassifyTags(tags, 5)

	if !got.Skip {
		t.Fatal("expected unrecognized tags to be skipped")
	}
}

func TestClassifyTags_EmptyTags(t *testing.T) {
	got := ClassifyTags(map[string]string{}, 5)

	if !got.Skip {
		t.Fatal("expected empty tags to be skipped")
	}
}

func TestClassifyTags_NilTags(t *testing.T) {
	got := ClassifyTags(nil, 5)

	if !got.Skip {
		t.Fatal("expected nil tags to be skipped")
	}
}

func TestClassifyTags_HighwayTakesPriorityOverBuilding(t *testing.T) {
	// If an element has both highway and building tags, highway wins.
	tags := map[string]string{"highway": "service", "building": "yes"}
	got := ClassifyTags(tags, 5)

	if got.Skip {
		t.Fatal("expected element to not be skipped")
	}
	if got.Block != dsf.BlockVector {
		t.Errorf("expected BlockVector (highway priority), got %v", got.Block)
	}
}
