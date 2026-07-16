package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"rpl/pkg/error/localize"
	"strings"
)

// Start launches the attr binary and binds its stdio so higher-level helpers
// can talk to it with one JSON request and one JSON response.
func Start(binary Binary, workDir string) (*Process, error) {
	cmd := exec.Command(binary.ExecPath)
	if trimmed := resolveProcessWorkDir(workDir); trimmed != "" {
		cmd.Dir = trimmed
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf(localize.Text("получение stdin для attr %q: %w", "get stdin for attr %q: %w"), binary.Manifest.Name, err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf(localize.Text("получение stdout для attr %q: %w", "get stdout for attr %q: %w"), binary.Manifest.Name, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(localize.Text("запуск attr %q: %w", "start attr %q: %w"), binary.Manifest.Name, err)
	}

	return &Process{
		Binary: binary,
		Cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
	}, nil
}

func Request(name string, author string, message any) (json.RawMessage, error) {
	return RequestAt(detectRequestWorkDir(message), name, author, message)
}

// RequestAt resolves the concrete attr binary against the caller's project
// context before running a single request/response exchange.
func RequestAt(basePath string, name string, author string, message any) (json.RawMessage, error) {
	binary, err := FindConfiguredAt(basePath, name, author)
	if err != nil {
		return nil, err
	}

	return RunBinary(*binary, message)
}

func RequestObject(name string, author string, message any) (map[string]any, error) {
	return RequestObjectAt(detectRequestWorkDir(message), name, author, message)
}

func RequestObjectAt(basePath string, name string, author string, message any) (map[string]any, error) {
	response, err := RequestAt(basePath, name, author, message)
	if err != nil {
		return nil, err
	}

	var object map[string]any
	if err := json.Unmarshal(response, &object); err != nil {
		return nil, fmt.Errorf(localize.Text("разбор объектного ответа attr %q автора %q: %w", "decode object response from attr %q by author %q: %w"), name, author, err)
	}

	return object, nil
}

// RunBinary owns the full lifecycle of a one-shot attr process so callers do
// not leak subprocesses when a request fails midway through.
func RunBinary(binary Binary, message any) (json.RawMessage, error) {
	process, err := Start(binary, detectRequestWorkDir(message))
	if err != nil {
		return nil, err
	}
	defer process.Stop()

	return process.Send(message)
}

// Send performs the wire protocol used by attrs: encode one JSON message into
// stdin, then decode one JSON payload from stdout.
func (process *Process) Send(message any) (json.RawMessage, error) {
	if err := process.validate(); err != nil {
		return nil, err
	}

	encoder := json.NewEncoder(process.Stdin)
	if err := encoder.Encode(message); err != nil {
		return nil, fmt.Errorf(localize.Text("отправка сообщения attr %q: %w", "send message to attr %q: %w"), process.Binary.Manifest.Name, err)
	}

	var response json.RawMessage
	decoder := json.NewDecoder(process.Stdout)
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf(localize.Text("чтение ответа attr %q: %w", "read response from attr %q: %w"), process.Binary.Manifest.Name, err)
	}

	return response, nil
}

func (process *Process) SendObject(message any) (map[string]any, error) {
	response, err := process.Send(message)
	if err != nil {
		return nil, err
	}

	var object map[string]any
	if err := json.Unmarshal(response, &object); err != nil {
		return nil, fmt.Errorf(localize.Text("разбор объектного ответа attr %q: %w", "decode object response from attr %q: %w"), process.Binary.Manifest.Name, err)
	}

	return object, nil
}

// Stop closes pipes first and then kills the subprocess if it is still alive,
// which keeps short-lived plugin executions from accumulating zombies.
func (process *Process) Stop() error {
	if process == nil || process.Cmd == nil {
		return nil
	}

	if process.Stdin != nil {
		_ = process.Stdin.Close()
	}
	if process.Stdout != nil {
		_ = process.Stdout.Close()
	}
	if process.Cmd.Process == nil {
		return nil
	}
	if process.Cmd.ProcessState != nil && process.Cmd.ProcessState.Exited() {
		return nil
	}

	if err := process.Cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf(localize.Text("остановка attr %q: %w", "stop attr %q: %w"), process.Binary.Manifest.Name, err)
	}

	_ = process.Cmd.Wait()
	return nil
}

func (process *Process) validate() error {
	if process == nil {
		return errors.New(localize.Text("запущенный attr отсутствует", "running attr is nil"))
	}
	if process.Stdin == nil || process.Stdout == nil {
		return errors.New(localize.Text("каналы attr не инициализированы", "attr pipes are not initialized"))
	}

	return nil
}

func RunAllConfigured() ([]*Process, error) {
	items, err := ListConfigured()
	if err != nil {
		return nil, err
	}

	running := make([]*Process, 0, len(items))
	for _, item := range items {
		process, err := Start(item, "")
		if err != nil {
			return nil, err
		}

		running = append(running, process)
	}

	return running, nil
}

// detectRequestWorkDir extracts the most specific project path available from
// the request payload so plugin discovery follows the caller's schema context.
func detectRequestWorkDir(message any) string {
	payload, ok := message.(map[string]any)
	if !ok {
		return ""
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		return ""
	}

	file, ok := data["file"].(map[string]any)
	if !ok {
		return ""
	}

	if projectRoot, ok := file["project_root"].(string); ok && strings.TrimSpace(projectRoot) != "" {
		return projectRoot
	}

	if sourcePath, ok := file["source_path"].(string); ok && strings.TrimSpace(sourcePath) != "" {
		return filepath.Dir(sourcePath)
	}

	return ""
}

func resolveProcessWorkDir(workDir string) string {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		return ""
	}

	info, err := os.Stat(trimmed)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return trimmed
	}
	return filepath.Dir(trimmed)
}
