package writer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

func TestTileOutputPath(t *testing.T) {
	tests := []struct {
		name      string
		outputDir string
		tile      dsf.TileCoord
		want      string
	}{
		{
			name:      "positive lat and lon",
			outputDir: "output",
			tile:      dsf.TileCoord{Lat: 43, Lon: 7},
			want:      filepath.Join("output", "+40+000", "+43+007.dsf"),
		},
		{
			name:      "negative lat and lon",
			outputDir: "output",
			tile:      dsf.TileCoord{Lat: -12, Lon: -3},
			want:      filepath.Join("output", "-20-010", "-12-003.dsf"),
		},
		{
			name:      "zero lat and lon",
			outputDir: "/scenery",
			tile:      dsf.TileCoord{Lat: 0, Lon: 0},
			want:      filepath.Join("/scenery", "+00+000", "+00+000.dsf"),
		},
		{
			name:      "boundary at 10-degree grid",
			outputDir: "out",
			tile:      dsf.TileCoord{Lat: 50, Lon: -80},
			want:      filepath.Join("out", "+50-080", "+50-080.dsf"),
		},
		{
			name:      "large negative coordinates",
			outputDir: "out",
			tile:      dsf.TileCoord{Lat: -33, Lon: -170},
			want:      filepath.Join("out", "-40-170", "-33-170.dsf"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TileOutputPath(tt.outputDir, tt.tile)
			if got != tt.want {
				t.Errorf("TileOutputPath(%q, %+v) = %q, want %q", tt.outputDir, tt.tile, got, tt.want)
			}
		})
	}
}

func TestEnsureTileDir(t *testing.T) {
	t.Run("creates directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		tile := dsf.TileCoord{Lat: 43, Lon: 7}

		err := EnsureTileDir(tmpDir, tile)
		if err != nil {
			t.Fatalf("EnsureTileDir() returned unexpected error: %v", err)
		}

		expectedDir := filepath.Join(tmpDir, "+40+000")
		info, err := os.Stat(expectedDir)
		if err != nil {
			t.Fatalf("expected directory %q does not exist: %v", expectedDir, err)
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", expectedDir)
		}
	})

	t.Run("creates nested directories for negative coords", func(t *testing.T) {
		tmpDir := t.TempDir()
		tile := dsf.TileCoord{Lat: -12, Lon: -3}

		err := EnsureTileDir(tmpDir, tile)
		if err != nil {
			t.Fatalf("EnsureTileDir() returned unexpected error: %v", err)
		}

		expectedDir := filepath.Join(tmpDir, "-20-010")
		info, err := os.Stat(expectedDir)
		if err != nil {
			t.Fatalf("expected directory %q does not exist: %v", expectedDir, err)
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", expectedDir)
		}
	})

	t.Run("idempotent when directory already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		tile := dsf.TileCoord{Lat: 50, Lon: -80}

		// Call twice — should not error on second call
		err := EnsureTileDir(tmpDir, tile)
		if err != nil {
			t.Fatalf("first call: unexpected error: %v", err)
		}
		err = EnsureTileDir(tmpDir, tile)
		if err != nil {
			t.Fatalf("second call: unexpected error: %v", err)
		}
	})

	t.Run("returns descriptive error on permission failure", func(t *testing.T) {
		// Use an invalid path that cannot be created
		invalidBase := filepath.Join(string([]byte{0}), "impossible")
		tile := dsf.TileCoord{Lat: 10, Lon: 20}

		err := EnsureTileDir(invalidBase, tile)
		if err == nil {
			t.Fatal("expected error for invalid path, got nil")
		}
		// Error should contain descriptive information
		errMsg := err.Error()
		if len(errMsg) == 0 {
			t.Error("expected non-empty error message")
		}
	})
}
