package writer

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

// Config holds writer configuration.
type Config struct {
	OutputDir  string
	TileFilter *dsf.TileCoord // Optional: produce only this tile
	BBoxFilter *BoundingBox   // Optional: produce only overlapping tiles
	TextMode   bool           // If true, write DSF text format (.dsf.txt) instead of binary

	// X-Plane rendering level properties (0-5, higher = more detail required).
	// These control the minimum rendering settings required to see objects/facades.
	ObjectRenderLevel int // sim/require_object level (default 1)
	FacadeRenderLevel int // sim/require_facade level (default 1)

	// Exclusion flags: when true, the DSF will contain sim/exclude_* properties
	// that tell X-Plane to remove default autogen in the overlay area.
	ExcludeObj bool // Exclude default objects
	ExcludeFac bool // Exclude default facades
	ExcludeFor bool // Exclude default forests
	ExcludeNet bool // Exclude default road networks
}

// BoundingBox defines a geographic rectangle in WGS84 degrees.
type BoundingBox struct {
	West  float64
	East  float64
	South float64
	North float64
}

// Run starts the writer stage. It collects building blocks from the input channel,
// groups them by tile, applies optional filters, and writes DSF binary files for
// each non-empty tile.
func Run(ctx context.Context, cfg Config, in <-chan dsf.BuildingBlock) error {
	// Collect all building blocks grouped by tile.
	tiles := make(map[dsf.TileCoord][]dsf.BuildingBlock)
	for {
		select {
		case <-ctx.Done():
			// Drain remaining items without processing.
			for range in {
			}
			return ctx.Err()
		case block, ok := <-in:
			if !ok {
				// Input channel closed, proceed to writing.
				goto write
			}
			tiles[block.Tile] = append(tiles[block.Tile], block)
		}
	}

write:
	writtenTiles := make(map[dsf.TileCoord][]dsf.BuildingBlock)

	// Process each tile.
	for tile, blocks := range tiles {
		// Check context cancellation before processing each tile.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Apply tile filter.
		if cfg.TileFilter != nil && tile != *cfg.TileFilter {
			continue
		}

		// Apply bounding box filter.
		if cfg.BBoxFilter != nil && !tileOverlapsBBox(tile, cfg.BBoxFilter) {
			continue
		}

		// Skip empty tiles (should not happen given we only have tiles with blocks,
		// but guard against edge cases after filtering).
		if len(blocks) == 0 {
			continue
		}

		// Write DSF output for this tile (text or binary).
		if cfg.TextMode {
			if err := writeTextDSF(cfg, tile, blocks); err != nil {
				return fmt.Errorf("writing DSF text for tile %+v: %w", tile, err)
			}
		} else {
			data, err := assembleDSF(tile, blocks)
			if err != nil {
				return fmt.Errorf("assembling DSF for tile %+v: %w", tile, err)
			}

			if err := writeTileFile(cfg.OutputDir, tile, data); err != nil {
				return err
			}
		}

		writtenTiles[tile] = blocks
	}

	if err := writeEarthWEDXML(cfg.OutputDir, writtenTiles); err != nil {
		return err
	}

	return nil
}

// tileOverlapsBBox returns true if the tile's 1° area overlaps the bounding box.
func tileOverlapsBBox(tile dsf.TileCoord, bbox *BoundingBox) bool {
	tileWest := float64(tile.Lon)
	tileEast := float64(tile.Lon + 1)
	tileSouth := float64(tile.Lat)
	tileNorth := float64(tile.Lat + 1)

	return tileWest < bbox.East && tileEast > bbox.West &&
		tileSouth < bbox.North && tileNorth > bbox.South
}

// assembleDSF builds the complete DSF binary for a tile from its building blocks.
func assembleDSF(tile dsf.TileCoord, blocks []dsf.BuildingBlock) ([]byte, error) {
	// Build HEAD atom with PROP sub-atom (including overlay property).
	headAtom := buildHEAD(tile)

	// Build DEFN atom with definition sub-atoms.
	defnAtom, defMap := buildDEFN(blocks)

	// Build GEOD atom with point pools.
	geodAtom, poolCoordMap, pool32CoordMap, pool16Idx, pool32Idx := buildGEOD(tile, blocks)

	// Build CMDS atom with command opcodes.
	cmdsAtom := buildCMDS(blocks, defMap, poolCoordMap, pool32CoordMap, pool16Idx, pool32Idx)

	// Assemble cookie + version + atoms.
	var buf []byte

	// Cookie (8 bytes).
	buf = append(buf, dsf.Cookie[:]...)

	// Version (uint32 LE).
	versionBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(versionBytes, dsf.DSFVersion)
	buf = append(buf, versionBytes...)

	// Atoms in order: HEAD, DEFN, GEOD, CMDS.
	buf = append(buf, headAtom...)
	buf = append(buf, defnAtom...)
	buf = append(buf, geodAtom...)
	buf = append(buf, cmdsAtom...)

	// Append MD5 footer.
	buf = AppendMD5Footer(buf)

	return buf, nil
}

// buildHEAD constructs the HEAD atom containing a PROP sub-atom with tile boundary
// properties and the overlay flag.
func buildHEAD(tile dsf.TileCoord) []byte {
	west := strconv.Itoa(tile.Lon)
	east := strconv.Itoa(tile.Lon + 1)
	south := strconv.Itoa(tile.Lat)
	north := strconv.Itoa(tile.Lat + 1)

	// PROP string table: null-terminated key-value pairs.
	// sim/overlay=1 marks this as an overlay DSF that layers on top of base scenery.
	propPayload := "sim/west\x00" + west + "\x00" +
		"sim/east\x00" + east + "\x00" +
		"sim/south\x00" + south + "\x00" +
		"sim/north\x00" + north + "\x00" +
		"sim/overlay\x001\x00"

	propAtom := EncodeAtom(dsf.AtomPROP, []byte(propPayload))
	return BuildAtom(dsf.AtomHEAD, propAtom)
}

// defMapping holds the index mapping for a definition type.
type defMapping struct {
	netw map[string]uint16 // DefPath → index for network defs
	poly map[string]uint16 // DefPath → index for polygon defs (includes facades)
	objt map[string]uint16 // DefPath → index for object defs
	tert map[string]uint16 // DefPath → index for terrain defs
}

// buildDEFN constructs the DEFN atom with definition sub-atoms and returns the
// definition index mapping.
func buildDEFN(blocks []dsf.BuildingBlock) ([]byte, defMapping) {
	dm := defMapping{
		netw: make(map[string]uint16),
		poly: make(map[string]uint16),
		objt: make(map[string]uint16),
		tert: make(map[string]uint16),
	}

	// Ordered slices for deterministic output.
	var netwDefs []string
	var polyDefs []string
	var objtDefs []string

	for _, b := range blocks {
		switch b.Type {
		case dsf.BlockVector:
			if _, exists := dm.netw[b.DefPath]; !exists {
				dm.netw[b.DefPath] = uint16(len(netwDefs))
				netwDefs = append(netwDefs, b.DefPath)
			}
		case dsf.BlockPolygon, dsf.BlockFacade:
			// Both polygons and facades use the POLY definition table in DSF.
			if _, exists := dm.poly[b.DefPath]; !exists {
				dm.poly[b.DefPath] = uint16(len(polyDefs))
				polyDefs = append(polyDefs, b.DefPath)
			}
		case dsf.BlockObject:
			if _, exists := dm.objt[b.DefPath]; !exists {
				dm.objt[b.DefPath] = uint16(len(objtDefs))
				objtDefs = append(objtDefs, b.DefPath)
			}
		}
	}

	// Encode definition atoms as null-terminated string tables.
	tertAtom := EncodeAtom(dsf.AtomTERT, []byte{0}) // Empty terrain defs (null byte placeholder)
	objtAtom := EncodeAtom(dsf.AtomOBJT, encodeDefTable(objtDefs))
	polyAtom := EncodeAtom(dsf.AtomPOLY, encodeDefTable(polyDefs))
	netwAtom := EncodeAtom(dsf.AtomNETW, encodeDefTable(netwDefs))
	demnAtom := EncodeAtom(dsf.AtomDEMN, []byte{0}) // Empty raster defs (null byte placeholder)

	return BuildAtom(dsf.AtomDEFN, tertAtom, objtAtom, polyAtom, netwAtom, demnAtom), dm
}

// encodeDefTable encodes a list of definition paths as a null-terminated string table.
func encodeDefTable(defs []string) []byte {
	if len(defs) == 0 {
		return []byte{0} // Empty table with null terminator.
	}
	var payload []byte
	for _, d := range defs {
		payload = append(payload, []byte(d)...)
		payload = append(payload, 0)
	}
	return payload
}

// buildGEOD constructs the GEOD atom with POOL/SCAL (16-bit) and PO32/SC32 (32-bit) sub-atoms.
// Returns the GEOD atom bytes, coordinate-to-index mappings, and the actual
// pool indices used for 16-bit and 32-bit pools in CMDS pool-select commands.
func buildGEOD(tile dsf.TileCoord, blocks []dsf.BuildingBlock) ([]byte, map[int][]uint16, map[int][]uint16, uint16, uint16) {
	// Separate coordinates by pool type:
	// - Polygons/Facades/Objects → 16-bit POOL
	// - Vectors → 32-bit PO32 (roads need higher precision + junction ID plane)
	var poolCoords []dsf.Coordinate
	var pool32Coords []dsf.Coordinate

	// Track which block indices map to which pool index ranges.
	poolCoordMap := make(map[int][]uint16)
	pool32CoordMap := make(map[int][]uint16)

	for i, b := range blocks {
		switch b.Type {
		case dsf.BlockVector:
			startIdx := uint16(len(pool32Coords))
			coords := b.Coords
			indices := make([]uint16, len(coords))
			for j := range coords {
				indices[j] = startIdx + uint16(j)
			}
			pool32CoordMap[i] = indices
			pool32Coords = append(pool32Coords, coords...)

		case dsf.BlockPolygon, dsf.BlockFacade:
			if len(b.Windings) > 0 {
				// Multipolygon with windings.
				var allIndices []uint16
				for _, winding := range b.Windings {
					startIdx := uint16(len(poolCoords))
					for j := range winding {
						allIndices = append(allIndices, startIdx+uint16(j))
					}
					poolCoords = append(poolCoords, winding...)
				}
				poolCoordMap[i] = allIndices
			} else {
				// Simple polygon/facade with Coords.
				startIdx := uint16(len(poolCoords))
				indices := make([]uint16, len(b.Coords))
				for j := range b.Coords {
					indices[j] = startIdx + uint16(j)
				}
				poolCoordMap[i] = indices
				poolCoords = append(poolCoords, b.Coords...)
			}

		case dsf.BlockObject:
			startIdx := uint16(len(poolCoords))
			indices := make([]uint16, len(b.Coords))
			for j := range b.Coords {
				indices[j] = startIdx + uint16(j)
			}
			poolCoordMap[i] = indices
			poolCoords = append(poolCoords, b.Coords...)
		}
	}

	var children [][]byte
	var nextPoolIdx uint16
	const invalidPoolIdx uint16 = 0xFFFF
	pool16Idx := invalidPoolIdx
	pool32Idx := invalidPoolIdx

	// Encode 16-bit point pool if we have polygon/facade/object coordinates.
	if len(poolCoords) > 0 {
		pool16Idx = nextPoolIdx
		nextPoolIdx++
		poolAtom, scalAtom := EncodePointPool(tile, poolCoords)
		children = append(children, poolAtom, scalAtom)
	}

	// Encode 32-bit point pool if we have vector coordinates.
	if len(pool32Coords) > 0 {
		pool32Idx = nextPoolIdx
		nextPoolIdx++
		po32Atom, sc32Atom := EncodePointPool32(tile, pool32Coords)
		children = append(children, po32Atom, sc32Atom)
	}

	geodAtom := BuildAtom(dsf.AtomGEOD, children...)
	return geodAtom, poolCoordMap, pool32CoordMap, pool16Idx, pool32Idx
}

// buildCMDS constructs the CMDS atom with command opcodes for all building blocks.
func buildCMDS(blocks []dsf.BuildingBlock, dm defMapping, poolCoordMap, pool32CoordMap map[int][]uint16, pool16Idx, pool32Idx uint16) []byte {
	var payload []byte

	// Track current state to minimize redundant commands.
	var currentPool int = -1 // -1 means no pool selected yet
	const invalidPoolIdx uint16 = 0xFFFF

	for i, b := range blocks {
		switch b.Type {
		case dsf.BlockVector:
			if pool32Idx == invalidPoolIdx {
				continue
			}
			// Select 32-bit pool if not already selected.
			if currentPool != 1 {
				payload = append(payload, EncodePoolSelect(pool32Idx)...)
				currentPool = 1
			}

			// Set definition.
			defIdx := dm.netw[b.DefPath]
			payload = append(payload, EncodeSetDefinition(defIdx)...)

			// Set road subtype.
			payload = append(payload, EncodeSetRoadSubType(b.SubType)...)

			// Encode network chain with point pool indices.
			indices := pool32CoordMap[i]
			payload = append(payload, EncodeNetworkChain(indices)...)

		case dsf.BlockPolygon, dsf.BlockFacade:
			if pool16Idx == invalidPoolIdx {
				continue
			}
			// Select 16-bit pool if not already selected.
			if currentPool != 0 {
				payload = append(payload, EncodePoolSelect(pool16Idx)...)
				currentPool = 0
			}

			// Set definition (both polygons and facades use the POLY table).
			defIdx := dm.poly[b.DefPath]
			payload = append(payload, EncodeSetDefinition(defIdx)...)

			if len(b.Windings) > 0 {
				// Multipolygon: emit polygon with first winding, then winding commands for rest.
				offset := 0
				firstWindingLen := len(b.Windings[0])
				indices := poolCoordMap[i]

				// First winding as main polygon.
				payload = append(payload, EncodePolygon(b.Param, indices[offset:offset+firstWindingLen])...)
				offset += firstWindingLen

				// Additional windings.
				for w := 1; w < len(b.Windings); w++ {
					payload = append(payload, EncodePolygonWinding()...)
					windingLen := len(b.Windings[w])
					payload = append(payload, EncodePolygon(b.Param, indices[offset:offset+windingLen])...)
					offset += windingLen
				}
			} else {
				// Simple polygon or facade.
				indices := poolCoordMap[i]
				payload = append(payload, EncodePolygon(b.Param, indices)...)
			}

		case dsf.BlockObject:
			if pool16Idx == invalidPoolIdx {
				continue
			}
			// Select 16-bit pool if not already selected.
			if currentPool != 0 {
				payload = append(payload, EncodePoolSelect(pool16Idx)...)
				currentPool = 0
			}

			// Set definition.
			defIdx := dm.objt[b.DefPath]
			payload = append(payload, EncodeSetDefinition(defIdx)...)

			// Place object at first coordinate.
			indices := poolCoordMap[i]
			if len(indices) > 0 {
				buf := make([]byte, 3)
				buf[0] = dsf.CmdObject
				binary.LittleEndian.PutUint16(buf[1:3], indices[0])
				payload = append(payload, buf...)
			}
		}
	}

	return EncodeAtom(dsf.AtomCMDS, payload)
}

// writeTileFile creates the directory and writes DSF data to the tile output path.
func writeTileFile(outputDir string, tile dsf.TileCoord, data []byte) error {
	if err := EnsureTileDir(outputDir, tile); err != nil {
		return err
	}

	path := TileOutputPath(outputDir, tile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write DSF file %q: %w", path, err)
	}

	return nil
}
