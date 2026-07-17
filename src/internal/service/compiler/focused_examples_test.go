package compiler

import (
	"io/fs"
	"os"
	"path/filepath"
	"rpl/internal/config"
	"strings"
	"testing"
)

func TestFocusedAttrExamplesGenerate(t *testing.T) {
	useRepoRootAsWorkingDir(t)
	repoRoot := currentRepoRoot(t)

	globalDir := filepath.Join(t.TempDir(), "global-rpl")
	t.Setenv(config.GlobalHomeEnv, globalDir)
	globalConfig := config.GlobalDefault()
	globalConfig.Runtimes.Directory = filepath.Join(repoRoot, "attrs")
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatalf("global config path: %v", err)
	}
	if err := config.Save(globalPath, globalConfig); err != nil {
		t.Fatalf("save test global config: %v", err)
	}

	folders := []struct {
		name     string
		attrs    []string
		expected string
	}{
		{name: "01-std", attrs: []string{"rpl:std", "rpl:validate"}, expected: "type "},
		{name: "02-validate", attrs: []string{"rpl:validate"}, expected: "func Validate("},
		{name: "03-sql", attrs: []string{"rpl:sql", "rpl:std", "rpl:validate"}, expected: "func NewStore("},
		{name: "04-redis", attrs: []string{"rpl:redis", "rpl:std"}, expected: "RedisHash()"},
		{name: "05-grpc", attrs: []string{"rpl:grpc", "rpl:std"}, expected: "syntax = \"proto3\""},
		{name: "08-mongodb", attrs: []string{"rpl:mongodb", "rpl:std"}, expected: "CollectionName"},
		{name: "09-transport", attrs: []string{"rpl:transport"}, expected: "TransportService interface"},
		{name: "10-ffi", attrs: []string{"rpl:ffi"}, expected: "FFI_ABI_VERSION"},
	}

	built := make(map[string]struct{})
	for _, folder := range folders {
		for _, identifier := range folder.attrs {
			if _, ok := built[identifier]; ok {
				continue
			}
			buildBundledRuntime(t, identifier)
			built[identifier] = struct{}{}
		}

		paths, err := filepath.Glob(filepath.Join(repoRoot, "..", "examples", folder.name, "*.rpl"))
		if err != nil {
			t.Fatalf("list %s examples: %v", folder.name, err)
		}
		if len(paths) == 0 {
			t.Fatalf("no focused examples found in %s", folder.name)
		}

		for _, sourcePath := range paths {
			sourcePath := sourcePath
			t.Run(folder.name+"/"+filepath.Base(sourcePath), func(t *testing.T) {
				body, err := os.ReadFile(sourcePath)
				if err != nil {
					t.Fatalf("read example: %v", err)
				}

				projectDir := t.TempDir()
				isolatedSource := filepath.Join(projectDir, "main.rpl")
				if err := os.WriteFile(isolatedSource, body, 0o644); err != nil {
					t.Fatalf("copy example: %v", err)
				}

				outputDir := filepath.Join(projectDir, "generated")
				if _, err := New().RunFileTo(isolatedSource, outputDir); err != nil {
					t.Fatalf("generate %s: %v", sourcePath, err)
				}
				if !generatedTreeContains(t, outputDir, folder.expected) {
					t.Fatalf("generated output does not contain %q", folder.expected)
				}
			})
		}
	}
}

func generatedTreeContains(t *testing.T, root string, fragment string) bool {
	t.Helper()
	found := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || found {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found = strings.Contains(string(body), fragment)
		return nil
	})
	if err != nil {
		t.Fatalf("inspect generated output: %v", err)
	}
	return found
}
