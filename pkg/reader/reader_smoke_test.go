package reader_test

import (
	"context"
	"errors"
	"testing"

	"github.com/paulmach/osm"
	"github.com/thomasbilk/osm2xpgo/pkg/reader"
)

func TestRun_MonacoPBF(t *testing.T) {
	const pbfPath = "../../monaco-260615.osm.pbf"

	out := make(chan osm.Object, 1024)
	errCh := make(chan error, 1)

	go func() {
		errCh <- reader.Run(context.Background(), pbfPath, out)
	}()

	count := 0
	for range out {
		count++
	}

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if count == 0 {
		t.Fatal("expected at least one OSM element, got 0")
	}
	t.Logf("Read %d OSM elements from %s", count, pbfPath)
}

func TestRun_MissingFile(t *testing.T) {
	out := make(chan osm.Object, 1024)

	err := reader.Run(context.Background(), "/nonexistent/path.osm.pbf", out)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	// The error should contain the file path
	if got := err.Error(); !contains(got, "/nonexistent/path.osm.pbf") {
		t.Errorf("error should contain file path, got: %s", got)
	}

	// Channel should be closed
	_, ok := <-out
	if ok {
		t.Error("expected channel to be closed after error")
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	const pbfPath = "../../monaco-260615.osm.pbf"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan osm.Object, 1024)
	errCh := make(chan error, 1)

	go func() {
		errCh <- reader.Run(ctx, pbfPath, out)
	}()

	// Read a few elements then cancel
	elemCount := 0
	for range out {
		elemCount++
		if elemCount >= 10 {
			cancel()
			break
		}
	}

	// Drain remaining elements from the channel
	for range out {
	}

	err := <-errCh
	// After cancellation, Run may return nil, context.Canceled, or a wrapped
	// context.Canceled error — all are acceptable.
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected nil or context.Canceled, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
