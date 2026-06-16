package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeriveOutputDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"monaco-260615.osm.pbf", "monaco-260615"},
		{"path/to/file.osm.pbf", "file"},
		{"simple.pbf", "simple"},
		{"no-extension", "no-extension"},
		{"multiple.dots.in.name.osm.pbf", "multiple.dots.in.name"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := deriveOutputDir(tt.input)
			if got != tt.want {
				t.Errorf("deriveOutputDir(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateFileReadable(t *testing.T) {
	// Create a temporary file for testing.
	tmp := t.TempDir()
	validFile := filepath.Join(tmp, "test.pbf")
	if err := os.WriteFile(validFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid file", func(t *testing.T) {
		if err := validateFileReadable(validFile); err != nil {
			t.Errorf("expected no error for valid file, got: %v", err)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := validateFileReadable(filepath.Join(tmp, "nope.pbf"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		err := validateFileReadable(tmp)
		if err == nil {
			t.Error("expected error when path is a directory")
		}
	})
}

func TestRunNoArgs(t *testing.T) {
	code := run([]string{})
	if code != 1 {
		t.Errorf("run([]) = %d, want 1", code)
	}
}

func TestRunDumpNoFile(t *testing.T) {
	code := run([]string{"--dump"})
	if code != 1 {
		t.Errorf("run([--dump]) = %d, want 1", code)
	}
}

func TestRunNonexistentInput(t *testing.T) {
	code := run([]string{"does-not-exist.osm.pbf"})
	if code != 1 {
		t.Errorf("run with nonexistent file = %d, want 1", code)
	}
}

func TestRunConvertModeInvalidPBF(t *testing.T) {
	// Create a temporary file with invalid PBF content.
	// The pipeline will attempt to parse it and fail, returning non-zero.
	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "test.osm.pbf")
	if err := os.WriteFile(inputFile, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	code := run([]string{inputFile})
	if code != 1 {
		t.Errorf("run with invalid PBF file = %d, want 1", code)
	}
}

func TestRunConvertModeWithOutputDirInvalidPBF(t *testing.T) {
	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "test.osm.pbf")
	if err := os.WriteFile(inputFile, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmp, "custom-output")
	code := run([]string{inputFile, outputDir})
	if code != 1 {
		t.Errorf("run with invalid PBF and output dir = %d, want 1", code)
	}
}

func TestRunDumpNonexistentFile(t *testing.T) {
	code := run([]string{"--dump", "nonexistent.dsf"})
	if code != 1 {
		t.Errorf("run --dump nonexistent = %d, want 1", code)
	}
}

func TestRunMonacoWritesExpectedGeoTile(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.Join("..", "..", "monaco-260615.osm.pbf")
	code := run([]string{input, tmp})
	if code != 0 {
		t.Fatalf("run(monaco) = %d, want 0", code)
	}

	var foundDSF string
	err := filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".dsf") {
			foundDSF = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking output dir: %v", err)
	}
	if foundDSF == "" {
		t.Fatal("expected at least one DSF output file")
	}
	if !strings.Contains(filepath.ToSlash(foundDSF), "/Earth nav data/") {
		t.Fatalf("expected DSF file under Earth nav data, got %q", foundDSF)
	}

	wedPath := filepath.Join(tmp, "earth.wed.xml")
	if _, err := os.Stat(wedPath); err != nil {
		t.Fatalf("expected earth.wed.xml to be generated: %v", err)
	}

	rel, err := filepath.Rel(tmp, foundDSF)
	if err != nil {
		t.Fatalf("filepath.Rel failed: %v", err)
	}
	if strings.Contains(rel, "+00+000") {
		t.Fatalf("expected non-zero geo tile for Monaco input, got %q", rel)
	}
}
