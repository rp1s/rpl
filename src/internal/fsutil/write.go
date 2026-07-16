package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFile replaces a directory at the target path with a regular file and
// ensures the parent directory exists before writing.
func WriteFile(path string, body []byte, perm os.FileMode) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("path is required")
	}

	info, err := os.Stat(trimmed)
	switch {
	case err == nil && info.IsDir():
		if removeErr := os.RemoveAll(trimmed); removeErr != nil {
			return fmt.Errorf("remove directory at file path %q: %w", trimmed, removeErr)
		}
	case err == nil:
	case os.IsNotExist(err):
	default:
		return fmt.Errorf("stat file path %q: %w", trimmed, err)
	}

	parentDir := filepath.Dir(trimmed)
	if parentDir != "." && parentDir != "" {
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return fmt.Errorf("create parent directory for %q: %w", trimmed, err)
		}
	}

	if err := os.WriteFile(trimmed, body, perm); err != nil {
		return fmt.Errorf("write file %q: %w", trimmed, err)
	}

	return nil
}
