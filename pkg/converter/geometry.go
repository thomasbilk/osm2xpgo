// Package converter implements the OSM → DSF building block transformation stage.
package converter

import "github.com/thomasbilk/osm2xpgo/pkg/dsf"

// SignedArea computes the signed area of a polygon ring using the shoelace formula.
// A positive result indicates counterclockwise (CCW) winding order.
// A negative result indicates clockwise (CW) winding order.
// The input coordinates use Lon as X and Lat as Y.
func SignedArea(coords []dsf.Coordinate) float64 {
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

// EnsureCCW returns the coordinates in counterclockwise order.
// If the ring is already CCW (positive signed area), the original slice is returned.
// If the ring is CW (negative signed area), a reversed copy is returned.
func EnsureCCW(coords []dsf.Coordinate) []dsf.Coordinate {
	if SignedArea(coords) >= 0 {
		return coords
	}
	return reverse(coords)
}

// EnsureCW returns the coordinates in clockwise order.
// If the ring is already CW (negative signed area), the original slice is returned.
// If the ring is CCW (positive signed area), a reversed copy is returned.
func EnsureCW(coords []dsf.Coordinate) []dsf.Coordinate {
	if SignedArea(coords) <= 0 {
		return coords
	}
	return reverse(coords)
}

// AssembleMultipolygon takes outer and inner coordinate rings and returns them
// with correct winding order for DSF multipolygons. Outer rings are ensured CCW
// and inner rings are ensured CW. The result slice contains all outer rings
// followed by all inner rings.
func AssembleMultipolygon(outerRings, innerRings [][]dsf.Coordinate) [][]dsf.Coordinate {
	result := make([][]dsf.Coordinate, 0, len(outerRings)+len(innerRings))

	for _, ring := range outerRings {
		result = append(result, EnsureCCW(ring))
	}
	for _, ring := range innerRings {
		result = append(result, EnsureCW(ring))
	}

	return result
}

// reverse returns a new slice with elements in reversed order.
func reverse(coords []dsf.Coordinate) []dsf.Coordinate {
	n := len(coords)
	rev := make([]dsf.Coordinate, n)
	for i, c := range coords {
		rev[n-1-i] = c
	}
	return rev
}
