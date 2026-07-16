package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileReplacesDirectoryAtTargetPath(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output.txt")
	if err := os.MkdirAll(filepath.Join(target, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}

	if err := WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file over directory: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected target to become a file")
	}

	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("unexpected body %q", string(body))
	}
}

func TestWriteFileCreatesMissingParents(t *testing.T) {
	target := filepath.Join(t.TempDir(), "one", "two", "file.txt")

	if err := WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("unexpected body %q", string(body))
	}
}
