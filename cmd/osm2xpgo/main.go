// Package main is the CLI entry point for osm2xpgo.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/paulmach/osm"
	"github.com/thomasbilk/osm2xpgo/pkg/converter"
	"github.com/thomasbilk/osm2xpgo/pkg/dsf"
	"github.com/thomasbilk/osm2xpgo/pkg/dump"
	"github.com/thomasbilk/osm2xpgo/pkg/reader"
	"github.com/thomasbilk/osm2xpgo/pkg/writer"
	"golang.org/x/sync/errgroup"
)

const usage = `Usage: osm2xpgo [--dump <file.dsf>] [--text] <input.osm.pbf> [output_dir]

Options:
  --dump <file>  Parse and display DSF file structure
  --text         Output DSF text format (.dsf.txt) instead of binary
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// Check for --dump mode.
	if len(args) >= 1 && args[0] == "--dump" {
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 1
		}
		filePath := args[1]
		if err := validateFileReadable(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if err := dump.Dump(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	// Convert mode: parse flags and positional arguments.
	textMode := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--text":
			textMode = true
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) < 1 {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}

	inputPath := positional[0]

	// Validate input file exists and is readable.
	if err := validateFileReadable(inputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Determine output directory.
	var outputDir string
	if len(positional) >= 2 {
		outputDir = positional[1]
	} else {
		outputDir = deriveOutputDir(inputPath)
	}

	// Set up context with cancellation for the pipeline.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up SIGINT handler.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Create buffered channels between pipeline stages.
	readerCh := make(chan osm.Object, 1024)
	converterCh := make(chan dsf.BuildingBlock, 1024)

	// Launch pipeline stages via errgroup.
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return reader.Run(gCtx, inputPath, readerCh)
	})

	g.Go(func() error {
		return converter.Run(gCtx, readerCh, converterCh)
	})

	writerCfg := writer.Config{
		OutputDir: outputDir,
		TextMode:  textMode,
	}
	g.Go(func() error {
		return writer.Run(gCtx, writerCfg, converterCh)
	})

	// Wait for pipeline completion or SIGINT in a separate goroutine.
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- g.Wait()
	}()

	select {
	case <-sigCh:
		// SIGINT received: cancel context and wait for graceful shutdown.
		fmt.Fprintln(os.Stderr, "Interrupt received, shutting down...")
		cancel()

		// Give stages up to 10 seconds to wind down.
		timer := time.NewTimer(10 * time.Second)
		select {
		case <-doneCh:
			timer.Stop()
		case <-timer.C:
			fmt.Fprintln(os.Stderr, "Shutdown timed out, forcing exit")
		}

		// Clean up partially written DSF files.
		cleanupDSFFiles(outputDir)
		return 1

	case err := <-doneCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}
}

// deriveOutputDir strips the file extension(s) from the input file's base name
// to produce the output directory name. It handles compound extensions like
// .osm.pbf by stripping known suffixes first, then falling back to the last
// extension.
// Example: "monaco-260615.osm.pbf" → "monaco-260615"
func deriveOutputDir(inputPath string) string {
	base := filepath.Base(inputPath)
	// Handle known compound extensions for OSM files.
	knownSuffixes := []string{".osm.pbf", ".osm.bz2", ".osm.gz"}
	lower := strings.ToLower(base)
	for _, suffix := range knownSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return base[:len(base)-len(suffix)]
		}
	}
	// Fall back to stripping the last extension.
	ext := filepath.Ext(base)
	if ext != "" {
		return strings.TrimSuffix(base, ext)
	}
	return base
}

// validateFileReadable checks that the given path exists and is a readable file.
func validateFileReadable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("cannot access file: %s: %v", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}
	// Attempt to open the file to verify read permission.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("file is not readable: %s: %v", path, err)
	}
	f.Close()
	return nil
}

// cleanupDSFFiles walks the output directory and removes any .dsf files that
// were partially written before cancellation.
func cleanupDSFFiles(dir string) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".dsf") {
			if removeErr := os.Remove(path); removeErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", path, removeErr)
			}
		}
		return nil
	})
}
