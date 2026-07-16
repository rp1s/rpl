package runtime

import (
	"encoding/json"
	"rpl/internal/plugins"
)

type RunnerManifest = plugins.Manifest
type RuntimeBinary = plugins.Binary
type RunningRuntime = plugins.Process

const (
	defaultRuntimeExecutable = plugins.DefaultExecutableName
	defaultRuntimeManifest   = plugins.DefaultManifestName
)

func RunRuntimes() ([]*RunningRuntime, error) {
	return plugins.RunAllConfigured()
}

func ListConfiguredRuntimes() ([]RuntimeBinary, error) {
	return plugins.ListConfigured()
}

func RunRuntimesFromDir(dir string) ([]*RunningRuntime, error) {
	items, err := plugins.List(dir)
	if err != nil {
		return nil, err
	}

	running := make([]*RunningRuntime, 0, len(items))
	for _, item := range items {
		process, err := plugins.Start(item, "")
		if err != nil {
			return nil, err
		}

		running = append(running, process)
	}

	return running, nil
}

func ListRuntimes(dir string) ([]RuntimeBinary, error) {
	return plugins.List(dir)
}

func RunRuntime(runtime RuntimeBinary) (*RunningRuntime, error) {
	return plugins.Start(runtime, "")
}

func FindRuntime(name string, author string) (*RuntimeBinary, error) {
	return plugins.FindConfigured(name, author)
}

func FindRuntimeInDir(dir string, name string, author string) (*RuntimeBinary, error) {
	return plugins.Find(dir, name, author)
}

func Run(name string, author string, message any) (json.RawMessage, error) {
	return plugins.Request(name, author, message)
}

func RunObject(name string, author string, message any) (map[string]any, error) {
	return plugins.RequestObject(name, author, message)
}

func RunBinary(runtime RuntimeBinary, message any) (json.RawMessage, error) {
	return plugins.RunBinary(runtime, message)
}

func RequestRuntime(name string, author string, message any) (json.RawMessage, error) {
	return plugins.Request(name, author, message)
}

func RequestRuntimeObject(name string, author string, message any) (map[string]any, error) {
	return plugins.RequestObject(name, author, message)
}

func RequestRuntimeBinary(runtime RuntimeBinary, message any) (json.RawMessage, error) {
	return plugins.RunBinary(runtime, message)
}

func LoadManifest(path string) (*RunnerManifest, error) {
	return plugins.LoadManifest(path)
}
