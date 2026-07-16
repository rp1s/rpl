package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"rpl/pkg/error/localize"
	"sort"
	"strings"
	"time"
)

const attrDevRebuildEnv = "RPL_ATTR_DEV"

// ensureLocalBinaryUpToDate recompiles a folder-based attr when its Go sources
// are newer than the executable (or when the executable is missing).
//
// In regular project usage we prefer an existing binary as-is, because local
// app copies of attrs should stay runnable even when their source tree no
// longer has access to the original SDK module path. Rebuild-on-edit is kept
// as an explicit dev mode behind RPL_ATTR_DEV=1.
func ensureLocalBinaryUpToDate(dir string, entryName string) error {
	sourceFiles, latestSourceTime, err := localSourceFiles(dir)
	if err != nil {
		return err
	}
	if len(sourceFiles) == 0 {
		return nil
	}

	execPath := filepath.Join(dir, entryName)
	info, err := os.Stat(execPath)
	switch {
	case err == nil:
		if !attrDevRebuildEnabled() {
			return nil
		}
		if !latestSourceTime.After(info.ModTime()) {
			return nil
		}
	case os.IsNotExist(err):
		// Build below.
	default:
		return fmt.Errorf(localize.Text("чтение исполняемого файла attr %q: %w", "read attr executable %q: %w"), execPath, err)
	}

	args := []string{"build", "-o", entryName}
	for _, path := range sourceFiles {
		args = append(args, filepath.Base(path))
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			localize.Text("сборка attr %q: %w\n%s", "build attr %q: %w\n%s"),
			dir,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

func attrDevRebuildEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(attrDevRebuildEnv)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func localSourceFiles(dir string) ([]string, time.Time, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf(localize.Text("поиск исходников attr в %q: %w", "find attr sources in %q: %w"), dir, err)
	}
	sort.Strings(entries)

	var latest time.Time
	files := make([]string, 0, len(entries))
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf(localize.Text("чтение исходника attr %q: %w", "read attr source %q: %w"), path, err)
		}
		if info.IsDir() {
			continue
		}

		files = append(files, path)
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}

	return files, latest, nil
}
