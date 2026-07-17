package compiler

import (
	"io/fs"
	"os"
	"path/filepath"
	"rpl/internal/config"
	"strings"
	"testing"
)

func TestProjectExamplesGenerateAndCompile(t *testing.T) {
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

	tests := []struct {
		name     string
		attrs    []string
		expected map[string][]string
	}{
		{
			name:  "account-service",
			attrs: []string{"rpl:sql", "rpl:std", "rpl:validate"},
			expected: map[string][]string{
				"generated/account/model.gen.go":                 {"type Account struct"},
				"generated/account/sql/queries.gen.go":           {"func NewStore", "func (store *Store) Create"},
				"generated/account/validation/validation.gen.go": {"func Validate"},
			},
		},
		{
			name:  "session-cache",
			attrs: []string{"rpl:redis", "rpl:std", "rpl:validate"},
			expected: map[string][]string{
				"generated/session/model.gen.go":                 {"func (model Session) RedisKey", "func (model *Session) ApplyRedisHash"},
				"generated/session/validation/validation.gen.go": {"func Validate"},
			},
		},
		{
			name:  "process-service",
			attrs: []string{"rpl:transport", "rpl:validate"},
			expected: map[string][]string{
				"generated/user/model.gen.go":                 {"type User struct"},
				"generated/user/transport/transport.gen.go":   {"type UserTransportService interface", "func (server *UserTransportServer) Serve"},
				"generated/user/transport/http.gen.go":        {"type UserHTTPHandler struct", "func NewUserHTTPClient"},
				"generated/user/transport/unix.gen.go":        {"func ListenUserUnix", "func DialUserUnix"},
				"generated/user/transport/nats.gen.go":        {"type UserNATSBroker interface"},
				"generated/user/transport/kafka.gen.go":       {"type UserKafkaBroker interface"},
				"generated/user/transport/websocket.gen.go":   {"type UserWebSocketConn interface"},
				"generated/user/validation/validation.gen.go": {"func Validate"},
			},
		},
		{
			name:  "grpc-service",
			attrs: []string{"rpl:grpc", "rpl:validate"},
			expected: map[string][]string{
				"generated/user/grpc/user.proto":    {"service UserService", "rpc GetByID"},
				"generated/user/grpc/server.gen.go": {"type UserService interface", "func RegisterUserGRPC"},
				"generated/user/grpc/client.gen.go": {"func NewUserGRPCClient"},
			},
		},
		{
			name:  "ffi-service",
			attrs: []string{"rpl:ffi"},
			expected: map[string][]string{
				"generated/calculator_service/ffi/calculator_service.h":    {"CALCULATOR_FFI_ABI_VERSION", "calculator_ffi_server_call_bytes"},
				"generated/calculator_service/ffi/go/client.gen.go":        {"type NativeABI interface", "func (client *Client) Add"},
				"generated/calculator_service/ffi/go/native_purego.gen.go": {"type PureGoNative struct", "func OpenPureGoFromFactory"},
				"generated/calculator_service/ffi/rust/src/lib.rs":         {"pub trait FFIService", "exported_ffi_server_call_bytes"},
			},
		},
	}

	built := make(map[string]struct{})
	for _, test := range tests {
		for _, identifier := range test.attrs {
			if _, ok := built[identifier]; ok {
				continue
			}
			buildBundledRuntime(t, identifier)
			built[identifier] = struct{}{}
		}

		t.Run(test.name, func(t *testing.T) {
			sourceProject := filepath.Join(repoRoot, "..", "examples", "projects", test.name)
			projectDir := filepath.Join(t.TempDir(), test.name)
			copyProjectExample(t, sourceProject, projectDir)

			sourcePath := filepath.Join(projectDir, "src", "main.rpl")
			outputDir := filepath.Join(projectDir, "generated")
			if _, err := New().RunFileTo(sourcePath, outputDir); err != nil {
				t.Fatalf("generate project: %v", err)
			}

			for relativePath, fragments := range test.expected {
				path := filepath.Join(projectDir, filepath.FromSlash(relativePath))
				for _, fragment := range fragments {
					assertFileContains(t, path, fragment)
				}
			}
			if test.name == "ffi-service" {
				hostModel := filepath.Join(projectDir, "generated", "calculator_service", "model.gen.go")
				if _, err := os.Stat(hostModel); !os.IsNotExist(err) {
					t.Fatalf("artifact-only FFI target generated unexpected host model %s", hostModel)
				}
			}

			runGoTest(t, projectDir)
		})
	}
}

func copyProjectExample(t *testing.T, source string, destination string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(destination, 0o755)
		}
		if entry.IsDir() && (entry.Name() == "generated" || entry.Name() == ".git") {
			return filepath.SkipDir
		}
		if strings.HasPrefix(entry.Name(), ".DS_Store") {
			return nil
		}

		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("copy project example %s: %v", source, err)
	}
}
