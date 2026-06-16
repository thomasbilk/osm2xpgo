// Package reader implements the PBF reading stage of the osm2xpgo pipeline.
// It parses OpenStreetMap PBF files and emits osm.Object elements on an output channel.
package reader

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

// Run starts the PBF reader stage. It reads all OSM elements from the input
// file and sends them on the output channel. The channel is closed on
// completion or error. Cancellation is respected via ctx.
func Run(ctx context.Context, inputPath string, out chan<- osm.Object) error {
	defer close(out)

	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("reader: failed to open PBF file %q: %w", inputPath, err)
	}
	defer f.Close()

	scanner := osmpbf.New(ctx, f, runtime.GOMAXPROCS(-1))
	defer scanner.Close()

	for scanner.Scan() {
		obj := cloneObject(scanner.Object())

		select {
		case <-ctx.Done():
			// Cancellation requested: drain remaining elements without processing.
			for scanner.Scan() {
				// discard
			}
			return ctx.Err()
		case out <- obj:
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reader: error reading PBF file %q: %w", inputPath, err)
	}

	return nil
}

func cloneObject(obj osm.Object) osm.Object {
	switch o := obj.(type) {
	case *osm.Node:
		c := *o
		if len(o.Tags) > 0 {
			c.Tags = append(osm.Tags(nil), o.Tags...)
		}
		return &c

	case *osm.Way:
		c := *o
		if len(o.Tags) > 0 {
			c.Tags = append(osm.Tags(nil), o.Tags...)
		}
		if len(o.Nodes) > 0 {
			c.Nodes = append([]osm.WayNode(nil), o.Nodes...)
		}
		return &c

	case *osm.Relation:
		c := *o
		if len(o.Tags) > 0 {
			c.Tags = append(osm.Tags(nil), o.Tags...)
		}
		if len(o.Members) > 0 {
			members := make([]osm.Member, len(o.Members))
			for i := range o.Members {
				members[i] = o.Members[i]
				if len(o.Members[i].Nodes) > 0 {
					members[i].Nodes = append([]osm.WayNode(nil), o.Members[i].Nodes...)
				}
			}
			c.Members = members
		}
		return &c

	default:
		return obj
	}
}
