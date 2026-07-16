package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureLocalBinaryUpToDatePrefersExistingBinaryOutsideDevMode(t *testing.T) {
	t.Setenv(attrDevRebuildEnv, "")

	dir := t.TempDir()
	execPath := filepath.Join(dir, "attr")
	if err := os.WriteFile(execPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write attr: %v", err)
	}

	// This source is intentionally not buildable. Regular project runs should
	// still use the existing binary instead of trying to rebuild it.
	sourcePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(sourcePath, []byte("package main\n\nthis will not compile\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := ensureLocalBinaryUpToDate(dir, "attr"); err != nil {
		t.Fatalf("ensureLocalBinaryUpToDate should keep existing binary: %v", err)
	}
}

func TestLocalSourceFilesExcludeGoTests(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.go")
	testPath := filepath.Join(dir, "main_test.go")
	if err := os.WriteFile(mainPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main source: %v", err)
	}
	if err := os.WriteFile(testPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	files, _, err := localSourceFiles(dir)
	if err != nil {
		t.Fatalf("localSourceFiles(): %v", err)
	}
	if len(files) != 1 || files[0] != mainPath {
		t.Fatalf("localSourceFiles() = %v, want only %q", files, mainPath)
	}
}
