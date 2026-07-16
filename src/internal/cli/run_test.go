package cli

import (
	"path/filepath"
	"testing"
)

func TestParseRunArgsDefaultOutput(t *testing.T) {
	path, outputDir, err := parseRunArgs([]string{"src/main.rpl"})
	if err != nil {
		t.Fatalf("parse run args: %v", err)
	}

	if path != "src/main.rpl" {
		t.Fatalf("unexpected path: %q", path)
	}
	if outputDir != "" {
		t.Fatalf("expected empty output dir, got %q", outputDir)
	}
}

func TestParseRunArgsWithOutputDir(t *testing.T) {
	path, outputDir, err := parseRunArgs([]string{"src/main.rpl", "out", "user"})
	if err != nil {
		t.Fatalf("parse run args: %v", err)
	}

	if path != "src/main.rpl" {
		t.Fatalf("unexpected path: %q", path)
	}
	if !filepath.IsAbs(outputDir) {
		t.Fatalf("expected absolute output dir, got %q", outputDir)
	}
	if filepath.Base(outputDir) != "user" {
		t.Fatalf("unexpected output dir: %q", outputDir)
	}
}

func TestParseRunArgsRejectsInvalidSyntax(t *testing.T) {
	_, _, err := parseRunArgs([]string{"src/main.rpl", "user"})
	if err == nil {
		t.Fatal("expected invalid syntax error")
	}
}
