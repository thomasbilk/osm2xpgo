package reader

import (
	"testing"

	"github.com/paulmach/osm"
)

func TestCloneObject_WayDeepCopy(t *testing.T) {
	orig := &osm.Way{
		ID: 1,
		Tags: osm.Tags{{Key: "highway", Value: "residential"}},
		Nodes: []osm.WayNode{{ID: 10, Lat: 43.7, Lon: 7.4}},
	}

	cloned, ok := cloneObject(orig).(*osm.Way)
	if !ok {
		t.Fatalf("expected *osm.Way clone")
	}

	orig.Tags[0].Value = "motorway"
	orig.Nodes[0].Lat = 0

	if cloned.Tags[0].Value != "residential" {
		t.Fatalf("clone tags mutated: got %q", cloned.Tags[0].Value)
	}
	if cloned.Nodes[0].Lat != 43.7 {
		t.Fatalf("clone nodes mutated: got %f", cloned.Nodes[0].Lat)
	}
}

func TestCloneObject_RelationDeepCopyMembers(t *testing.T) {
	orig := &osm.Relation{
		ID: 2,
		Members: []osm.Member{{
			Type:  osm.TypeWay,
			Ref:   11,
			Role:  "outer",
			Nodes: []osm.WayNode{{ID: 100, Lat: 43.71, Lon: 7.41}},
		}},
	}

	cloned, ok := cloneObject(orig).(*osm.Relation)
	if !ok {
		t.Fatalf("expected *osm.Relation clone")
	}

	orig.Members[0].Nodes[0].Lon = 0
	if cloned.Members[0].Nodes[0].Lon != 7.41 {
		t.Fatalf("clone member nodes mutated: got %f", cloned.Members[0].Nodes[0].Lon)
	}
}
