package plugins

import (
	"os"
	"path/filepath"
	"rpl/internal/config"
	"testing"
)

func TestFindConfiguredAtResolvesProjectRelativeAttrsFromSourcePath(t *testing.T) {
	projectDir := t.TempDir()
	attrDir := filepath.Join(projectDir, ".rpl", "attrs", "rpl:demo")
	if err := os.MkdirAll(attrDir, 0o755); err != nil {
		t.Fatalf("mkdir attr dir: %v", err)
	}

	manifest := `<?xml version="1.0" encoding="UTF-8"?>
<attr>
  <name>demo</name>
  <author>rpl</author>
  <version>1.0.0</version>
  <entry>attr</entry>
  <sdkVersion>2</sdkVersion>
</attr>
`
	if err := os.WriteFile(filepath.Join(attrDir, "manifest.xml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(attrDir, "attr"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write attr binary: %v", err)
	}

	sourcePath := filepath.Join(projectDir, "src", "main.rpl")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("model User {}"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	outsideDir := t.TempDir()
	if err := os.Chdir(outsideDir); err != nil {
		t.Fatalf("chdir outside project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	item, err := FindConfiguredAt(sourcePath, "demo", "rpl")
	if err != nil {
		t.Fatalf("find configured attr: %v", err)
	}
	if got, want := item.ManifestPath, filepath.Join(attrDir, "manifest.xml"); got != want {
		t.Fatalf("manifest path = %q, want %q", got, want)
	}
	if got, want := item.ExecPath, filepath.Join(attrDir, "attr"); got != want {
		t.Fatalf("exec path = %q, want %q", got, want)
	}
}

func TestConfiguredSearchPathsIncludeBundledAttrs(t *testing.T) {
	projectDir := t.TempDir()
	sourcePath := filepath.Join(projectDir, "src", "main.rpl")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("model User {}"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	paths, err := configuredSearchPathsForBase(sourcePath)
	if err != nil {
		t.Fatalf("configured search paths: %v", err)
	}
	want := config.BundledRuntimesPathFromExecutable()
	if want == "" {
		t.Skip("test executable path is unavailable")
	}

	for _, path := range paths {
		if filepath.Clean(path) == filepath.Clean(want) {
			return
		}
	}
	t.Fatalf("bundled attrs path %q is missing from %v", want, paths)
}
