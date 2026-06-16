package writer

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
)

type wedXML struct {
	XMLName xml.Name   `xml:"wed"`
	Version string     `xml:"version,attr"`
	Scenery wedScenery `xml:"scenery"`
}

type wedScenery struct {
	Name  string    `xml:"name,attr"`
	Tiles []wedTile `xml:"tile"`
}

type wedTile struct {
	Lat    int       `xml:"lat,attr"`
	Lon    int       `xml:"lon,attr"`
	Path   string    `xml:"path,attr"`
	Bounds wedBounds `xml:"bounds"`
	Counts wedCounts `xml:"counts"`
}

type wedBounds struct {
	West  string `xml:"west,attr"`
	East  string `xml:"east,attr"`
	South string `xml:"south,attr"`
	North string `xml:"north,attr"`
}

type wedCounts struct {
	Vector  int `xml:"vector,attr"`
	Polygon int `xml:"polygon,attr"`
	Facade  int `xml:"facade,attr"`
	Object  int `xml:"object,attr"`
}

func writeEarthWEDXML(outputDir string, tiles map[dsf.TileCoord][]dsf.BuildingBlock) error {
	tileList := make([]dsf.TileCoord, 0, len(tiles))
	for tile := range tiles {
		tileList = append(tileList, tile)
	}

	sort.Slice(tileList, func(i, j int) bool {
		if tileList[i].Lat != tileList[j].Lat {
			return tileList[i].Lat < tileList[j].Lat
		}
		return tileList[i].Lon < tileList[j].Lon
	})

	wed := wedXML{
		Version: "1",
		Scenery: wedScenery{
			Name:  "osm2xpgo",
			Tiles: make([]wedTile, 0, len(tileList)),
		},
	}

	for _, tile := range tileList {
		blocks := tiles[tile]
		counts := countBlocksByType(blocks)

		wed.Scenery.Tiles = append(wed.Scenery.Tiles, wedTile{
			Lat:  tile.Lat,
			Lon:  tile.Lon,
			Path: filepath.ToSlash(filepath.Join(earthNavDataDir, tile.TilePath())),
			Bounds: wedBounds{
				West:  strconv.Itoa(tile.Lon),
				East:  strconv.Itoa(tile.Lon + 1),
				South: strconv.Itoa(tile.Lat),
				North: strconv.Itoa(tile.Lat + 1),
			},
			Counts: counts,
		})
	}

	payload, err := xml.MarshalIndent(wed, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode earth.wed.xml: %w", err)
	}

	payload = append([]byte(xml.Header), payload...)
	payload = append(payload, '\n')

	path := filepath.Join(outputDir, "earth.wed.xml")
	if err := os.WriteFile(path, payload, 0644); err != nil {
		return fmt.Errorf("failed to write WED file %q: %w", path, err)
	}

	return nil
}

func countBlocksByType(blocks []dsf.BuildingBlock) wedCounts {
	var counts wedCounts
	for _, block := range blocks {
		switch block.Type {
		case dsf.BlockVector:
			counts.Vector++
		case dsf.BlockPolygon:
			counts.Polygon++
		case dsf.BlockFacade:
			counts.Facade++
		case dsf.BlockObject:
			counts.Object++
		}
	}
	return counts
}
