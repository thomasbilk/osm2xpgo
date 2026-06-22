package writer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// writeTextDSF writes a DSF text file (.dsf.txt) for the given tile and its building blocks.
// This format is human-readable and can be compiled to binary DSF using X-Plane's DSFTool.
func writeTextDSF(cfg Config, tile dsf.TileCoord, blocks []dsf.BuildingBlock) error {
	path := TileOutputPath(cfg.OutputDir, tile)
	path += ".txt" // Append .txt to produce e.g. +43+007.dsf.txt

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tile directory %q: %w", dir, err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create DSF text file %q: %w", path, err)
	}
	defer f.Close()

	// Write DSF text header.
	fmt.Fprintln(f, "I")
	fmt.Fprintln(f, "800")
	fmt.Fprintln(f, "DSF2TEXT")
	fmt.Fprintln(f)

	// Properties.
	fmt.Fprintln(f, "PROPERTY sim/planet earth")
	fmt.Fprintln(f, "PROPERTY sim/overlay 1")
	fmt.Fprintln(f, "PROPERTY sim/creation_agent osm2xpgo")

	// Render level requirements.
	objLevel := cfg.ObjectRenderLevel
	if objLevel <= 0 {
		objLevel = 1
	}
	facLevel := cfg.FacadeRenderLevel
	if facLevel <= 0 {
		facLevel = 1
	}
	fmt.Fprintf(f, "PROPERTY sim/require_object %d/0\n", objLevel)
	fmt.Fprintf(f, "PROPERTY sim/require_facade %d/0\n", facLevel)

	// Exclusion zones: tell X-Plane to remove default autogen in this tile area.
	west := fmt.Sprintf("%d.000000000", tile.Lon)
	south := fmt.Sprintf("%d.000000000", tile.Lat)
	east := fmt.Sprintf("%d.000000000", tile.Lon+1)
	north := fmt.Sprintf("%d.000000000", tile.Lat+1)
	excludeBox := west + "/" + south + "/" + east + "/" + north

	if cfg.ExcludeObj {
		fmt.Fprintf(f, "PROPERTY sim/exclude_obj %s\n", excludeBox)
	}
	if cfg.ExcludeFac {
		fmt.Fprintf(f, "PROPERTY sim/exclude_fac %s\n", excludeBox)
	}
	if cfg.ExcludeFor {
		fmt.Fprintf(f, "PROPERTY sim/exclude_for %s\n", excludeBox)
	}
	if cfg.ExcludeNet {
		fmt.Fprintf(f, "PROPERTY sim/exclude_net %s\n", excludeBox)
	}
	if cfg.ExcludePol {
		fmt.Fprintf(f, "PROPERTY sim/exclude_pol %s\n", excludeBox)
	}
	if cfg.ExcludeLin {
		fmt.Fprintf(f, "PROPERTY sim/exclude_lin %s\n", excludeBox)
	}
	if cfg.ExcludeStr {
		fmt.Fprintf(f, "PROPERTY sim/exclude_str %s\n", excludeBox)
	}
	if cfg.ExcludeBch {
		fmt.Fprintf(f, "PROPERTY sim/exclude_bch %s\n", excludeBox)
	}

	fmt.Fprintf(f, "PROPERTY sim/west %d\n", tile.Lon)
	fmt.Fprintf(f, "PROPERTY sim/east %d\n", tile.Lon+1)
	fmt.Fprintf(f, "PROPERTY sim/south %d\n", tile.Lat)
	fmt.Fprintf(f, "PROPERTY sim/north %d\n", tile.Lat+1)
	fmt.Fprintln(f)

	// Collect definitions in order (same logic as buildDEFN).
	var netwDefs []string
	var polyDefs []string
	var objtDefs []string
	netwIndex := make(map[string]int)
	polyIndex := make(map[string]int)
	objtIndex := make(map[string]int)

	for _, b := range blocks {
		switch b.Type {
		case dsf.BlockVector:
			if _, exists := netwIndex[b.DefPath]; !exists {
				netwIndex[b.DefPath] = len(netwDefs)
				netwDefs = append(netwDefs, b.DefPath)
			}
		case dsf.BlockPolygon, dsf.BlockFacade:
			if _, exists := polyIndex[b.DefPath]; !exists {
				polyIndex[b.DefPath] = len(polyDefs)
				polyDefs = append(polyDefs, b.DefPath)
			}
		case dsf.BlockObject:
			if _, exists := objtIndex[b.DefPath]; !exists {
				objtIndex[b.DefPath] = len(objtDefs)
				objtDefs = append(objtDefs, b.DefPath)
			}
		}
	}

	// Write definitions.
	for _, d := range polyDefs {
		fmt.Fprintf(f, "POLYGON_DEF %s\n", d)
	}
	for _, d := range objtDefs {
		fmt.Fprintf(f, "OBJECT_DEF %s\n", d)
	}
	for _, d := range netwDefs {
		fmt.Fprintf(f, "NETWORK_DEF %s\n", d)
	}
	if len(polyDefs) > 0 || len(objtDefs) > 0 || len(netwDefs) > 0 {
		fmt.Fprintln(f)
	}

	// Write features.
	for _, b := range blocks {
		switch b.Type {
		case dsf.BlockObject:
			idx := objtIndex[b.DefPath]
			if len(b.Coords) > 0 {
				c := b.Coords[0]
				fmt.Fprintf(f, "OBJECT %d %s %s 0.00\n", idx, formatCoord(c.Lon), formatCoord(c.Lat))
			}

		case dsf.BlockPolygon, dsf.BlockFacade:
			idx := polyIndex[b.DefPath]
			if len(b.Windings) > 0 {
				// Multipolygon with windings.
				fmt.Fprintf(f, "BEGIN_POLYGON %d %d 2\n", idx, b.Param)
				for _, winding := range b.Windings {
					fmt.Fprintln(f, "BEGIN_WINDING")
					writeTextWinding(f, winding)
					fmt.Fprintln(f, "END_WINDING")
				}
				fmt.Fprintln(f, "END_POLYGON")
			} else if len(b.Coords) > 0 {
				// Simple polygon/facade.
				fmt.Fprintf(f, "BEGIN_POLYGON %d %d 2\n", idx, b.Param)
				fmt.Fprintln(f, "BEGIN_WINDING")
				writeTextWinding(f, b.Coords)
				fmt.Fprintln(f, "END_WINDING")
				fmt.Fprintln(f, "END_POLYGON")
			}

		case dsf.BlockVector:
			writeTextSegment(f, b)
		}
	}

	return nil
}

// writeTextWinding writes polygon points for a winding, stripping the closing
// duplicate point if present.
func writeTextWinding(f *os.File, coords []dsf.Coordinate) {
	pts := coords
	// Strip closing duplicate (same as first point).
	if len(pts) > 1 && pts[0].Lon == pts[len(pts)-1].Lon && pts[0].Lat == pts[len(pts)-1].Lat {
		pts = pts[:len(pts)-1]
	}
	for _, c := range pts {
		fmt.Fprintf(f, "POLYGON_POINT %s %s\n", formatCoord(c.Lon), formatCoord(c.Lat))
	}
}

// writeTextSegment writes a network segment in DSF text format.
func writeTextSegment(f *os.File, b dsf.BuildingBlock) {
	if len(b.Coords) < 2 {
		return
	}
	first := b.Coords[0]
	last := b.Coords[len(b.Coords)-1]

	fmt.Fprintf(f, "BEGIN_SEGMENT 0 %d 0 %s %s %s\n",
		int(b.SubType), formatCoord(first.Lon), formatCoord(first.Lat), formatCoord(first.Ele))

	for _, c := range b.Coords[1 : len(b.Coords)-1] {
		fmt.Fprintf(f, "SHAPE_POINT %s %s %s\n",
			formatCoord(c.Lon), formatCoord(c.Lat), formatCoord(c.Ele))
	}

	fmt.Fprintf(f, "END_SEGMENT 0 %s %s %s\n",
		formatCoord(last.Lon), formatCoord(last.Lat), formatCoord(last.Ele))
}

// formatCoord formats a coordinate value with 9 decimal places,
// matching X-Plane DSF text conventions.
func formatCoord(v float64) string {
	s := fmt.Sprintf("%.9f", v)
	// Trim trailing zeros for cleaner output, but keep at least one decimal.
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
		// Ensure we always have the decimal portion for coordinates.
		if !strings.Contains(s, ".") {
			s += ".0"
		}
	}
	return s
}
