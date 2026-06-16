package writer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

func TestWriteEarthWEDXML(t *testing.T) {
	tmpDir := t.TempDir()
	tiles := map[dsf.TileCoord][]dsf.BuildingBlock{
		{Lat: 44, Lon: 8}: {
			{Type: dsf.BlockFacade},
			{Type: dsf.BlockPolygon},
		},
		{Lat: 43, Lon: 7}: {
			{Type: dsf.BlockVector},
			{Type: dsf.BlockVector},
			{Type: dsf.BlockObject},
		},
	}

	if err := writeEarthWEDXML(tmpDir, tiles); err != nil {
		t.Fatalf("writeEarthWEDXML returned error: %v", err)
	}

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "earth.wed.xml"))
	if err != nil {
		t.Fatalf("reading earth.wed.xml failed: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, `path="Earth nav data/+40+000/+43+007.dsf"`) {
		t.Fatalf("earth.wed.xml missing first tile path: %s", content)
	}
	if !strings.Contains(content, `path="Earth nav data/+40+000/+44+008.dsf"`) {
		t.Fatalf("earth.wed.xml missing second tile path: %s", content)
	}
	if !strings.Contains(content, `counts vector="2" polygon="0" facade="0" object="1"`) {
		t.Fatalf("earth.wed.xml missing expected counts for +43+007 tile: %s", content)
	}
	if !strings.Contains(content, `counts vector="0" polygon="1" facade="1" object="0"`) {
		t.Fatalf("earth.wed.xml missing expected counts for +44+008 tile: %s", content)
	}

	idx43 := strings.Index(content, `path="Earth nav data/+40+000/+43+007.dsf"`)
	idx44 := strings.Index(content, `path="Earth nav data/+40+000/+44+008.dsf"`)
	if idx43 == -1 || idx44 == -1 || idx43 >= idx44 {
		t.Fatalf("expected tiles sorted by coordinates in earth.wed.xml: %s", content)
	}
}
