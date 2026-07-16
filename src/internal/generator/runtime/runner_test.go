package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListRuntimesDoesNotCreateMissingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	runtimesDir := filepath.Join(tempDir, ".rpl", "runtimes")

	items, err := ListRuntimes(runtimesDir)
	if err != nil {
		t.Fatalf("list runtimes: %v", err)
	}

	if len(items) != 0 {
		t.Fatalf("expected no runtimes, got %d", len(items))
	}

	if _, err := os.Stat(runtimesDir); !os.IsNotExist(err) {
		t.Fatalf("expected runtimes dir to be absent, got err=%v", err)
	}
}
