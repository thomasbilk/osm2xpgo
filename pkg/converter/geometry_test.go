package converter

import (
	"math"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// ccwSquare is a unit square with counterclockwise winding order.
var ccwSquare = []dsf.Coordinate{
	{Lon: 0, Lat: 0},
	{Lon: 1, Lat: 0},
	{Lon: 1, Lat: 1},
	{Lon: 0, Lat: 1},
}

// cwSquare is a unit square with clockwise winding order.
var cwSquare = []dsf.Coordinate{
	{Lon: 0, Lat: 0},
	{Lon: 0, Lat: 1},
	{Lon: 1, Lat: 1},
	{Lon: 1, Lat: 0},
}

func TestSignedArea_CCW(t *testing.T) {
	area := SignedArea(ccwSquare)
	if area <= 0 {
		t.Errorf("expected positive area for CCW square, got %f", area)
	}
	if math.Abs(area-1.0) > 1e-10 {
		t.Errorf("expected area 1.0 for unit square, got %f", area)
	}
}

func TestSignedArea_CW(t *testing.T) {
	area := SignedArea(cwSquare)
	if area >= 0 {
		t.Errorf("expected negative area for CW square, got %f", area)
	}
	if math.Abs(area+1.0) > 1e-10 {
		t.Errorf("expected area -1.0 for CW unit square, got %f", area)
	}
}

func TestSignedArea_Degenerate(t *testing.T) {
	// Fewer than 3 points
	area := SignedArea([]dsf.Coordinate{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 1}})
	if area != 0 {
		t.Errorf("expected 0 area for degenerate ring, got %f", area)
	}

	// Empty
	area = SignedArea(nil)
	if area != 0 {
		t.Errorf("expected 0 area for nil ring, got %f", area)
	}
}

func TestSignedArea_Triangle(t *testing.T) {
	// CCW triangle with area 0.5
	tri := []dsf.Coordinate{
		{Lon: 0, Lat: 0},
		{Lon: 1, Lat: 0},
		{Lon: 0, Lat: 1},
	}
	area := SignedArea(tri)
	if math.Abs(area-0.5) > 1e-10 {
		t.Errorf("expected area 0.5 for CCW triangle, got %f", area)
	}
}

func TestEnsureCCW_AlreadyCCW(t *testing.T) {
	result := EnsureCCW(ccwSquare)
	if SignedArea(result) <= 0 {
		t.Error("expected CCW result")
	}
	// Should return original slice (no copy needed)
	if &result[0] != &ccwSquare[0] {
		t.Error("expected original slice returned for already-CCW input")
	}
}

func TestEnsureCCW_FromCW(t *testing.T) {
	result := EnsureCCW(cwSquare)
	if SignedArea(result) <= 0 {
		t.Error("expected CCW result after reversing CW input")
	}
}

func TestEnsureCW_AlreadyCW(t *testing.T) {
	result := EnsureCW(cwSquare)
	if SignedArea(result) >= 0 {
		t.Error("expected CW result")
	}
	// Should return original slice
	if &result[0] != &cwSquare[0] {
		t.Error("expected original slice returned for already-CW input")
	}
}

func TestEnsureCW_FromCCW(t *testing.T) {
	result := EnsureCW(ccwSquare)
	if SignedArea(result) >= 0 {
		t.Error("expected CW result after reversing CCW input")
	}
}

func TestAssembleMultipolygon_Basic(t *testing.T) {
	outer := [][]dsf.Coordinate{cwSquare} // intentionally CW, should be corrected to CCW
	inner := [][]dsf.Coordinate{ccwSquare} // intentionally CCW, should be corrected to CW

	result := AssembleMultipolygon(outer, inner)

	if len(result) != 2 {
		t.Fatalf("expected 2 windings, got %d", len(result))
	}

	// First ring (outer) must be CCW
	if SignedArea(result[0]) <= 0 {
		t.Error("outer ring should be CCW (positive area)")
	}

	// Second ring (inner) must be CW
	if SignedArea(result[1]) >= 0 {
		t.Error("inner ring should be CW (negative area)")
	}
}

func TestAssembleMultipolygon_MultipleRings(t *testing.T) {
	outer1 := []dsf.Coordinate{
		{Lon: 0, Lat: 0}, {Lon: 10, Lat: 0}, {Lon: 10, Lat: 10}, {Lon: 0, Lat: 10},
	}
	outer2 := []dsf.Coordinate{
		{Lon: 20, Lat: 20}, {Lon: 30, Lat: 20}, {Lon: 30, Lat: 30}, {Lon: 20, Lat: 30},
	}
	inner1 := []dsf.Coordinate{
		{Lon: 2, Lat: 2}, {Lon: 2, Lat: 8}, {Lon: 8, Lat: 8}, {Lon: 8, Lat: 2},
	}

	result := AssembleMultipolygon([][]dsf.Coordinate{outer1, outer2}, [][]dsf.Coordinate{inner1})

	if len(result) != 3 {
		t.Fatalf("expected 3 windings, got %d", len(result))
	}

	// Outers must be CCW
	for i := 0; i < 2; i++ {
		if SignedArea(result[i]) <= 0 {
			t.Errorf("outer ring %d should be CCW", i)
		}
	}

	// Inner must be CW
	if SignedArea(result[2]) >= 0 {
		t.Error("inner ring should be CW")
	}
}

func TestAssembleMultipolygon_EmptyInputs(t *testing.T) {
	result := AssembleMultipolygon(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil inputs, got %d rings", len(result))
	}
}

func TestReverse(t *testing.T) {
	input := []dsf.Coordinate{
		{Lon: 1, Lat: 2},
		{Lon: 3, Lat: 4},
		{Lon: 5, Lat: 6},
	}
	result := reverse(input)

	if len(result) != 3 {
		t.Fatalf("expected length 3, got %d", len(result))
	}
	if result[0].Lon != 5 || result[1].Lon != 3 || result[2].Lon != 1 {
		t.Errorf("unexpected reversed order: %v", result)
	}
	// Original should be unchanged
	if input[0].Lon != 1 {
		t.Error("original slice was mutated")
	}
}
