package writer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

const earthNavDataDir = "Earth nav data"

// TileOutputPath returns the full filesystem path for a tile's DSF file
// by joining the output directory with the tile's relative path.
// Example: TileOutputPath("/out", TileCoord{Lat:43, Lon:7}) → "/out/+40+000/+43+007.dsf"
func TileOutputPath(outputDir string, tile dsf.TileCoord) string {
	return filepath.Join(outputDir, earthNavDataDir, tile.TilePath())
}

// EnsureTileDir creates the directory structure required for a tile's DSF file.
// It uses os.MkdirAll with permissions 0755 to create any missing parent directories.
// Returns a descriptive error on failure including the target path.
func EnsureTileDir(outputDir string, tile dsf.TileCoord) error {
	tilePath := TileOutputPath(outputDir, tile)
	dir := filepath.Dir(tilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tile directory %q: %w", dir, err)
	}
	return nil
}
