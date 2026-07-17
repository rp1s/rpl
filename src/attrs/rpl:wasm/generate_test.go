package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"rpl/pkg/sdk"
	"strings"
	"testing"
)

func TestGenerateWITLowersRPLTypesAndErrors(t *testing.T) {
	plan := wasmTestPlan()
	body := generateWIT(plan)
	for _, expected := range []string{
		"package example:users@1.0.0;",
		"record user-service",
		"email: option<string>",
		"tags: list<string>",
		"get-user: func(id: u64) -> result<option<user-service>, string>;",
		"%list: func() -> result<list<user-service>, string>;",
		"stats: func() -> tuple<s64, f64>;",
		"world user-plugin",
		"export user-service;",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("generated WIT does not contain %q:\n%s", expected, body)
		}
	}
}

func TestGenerateWASMResponseSelectsGuestAndHost(t *testing.T) {
	response := generateWASMResponse(wasmTestPlan())
	paths := make(map[string]bool)
	for _, file := range response.Files {
		paths[file.Path] = true
	}
	for _, expected := range []string{
		"wasm/user-service/wit/world.wit",
		"wasm/user-service/plugin.toml",
		"wasm/user-service/schema.json",
		"wasm/user-service/guest/rust/src/lib.rs",
		"wasm/user-service/host/rust/src/lib.rs",
	} {
		if !paths[expected] {
			t.Fatalf("missing generated file %q", expected)
		}
	}
}

func TestParseWASMConfigRejectsUnverifiedGoTargets(t *testing.T) {
	if _, err := parseWASMHosts("go"); err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("expected an actionable Go host error, got %v", err)
	}
	if _, _, _, err := parseWASMPackage("Example:Users"); err == nil {
		t.Fatal("expected invalid WIT package to fail")
	}
}

func TestWASMManifestIsDenyByDefault(t *testing.T) {
	body := generateWASMManifest(wasmTestPlan())
	for _, expected := range []string{
		"filesystem = false",
		"network = false",
		"environment = []",
		"max-memory-bytes = 67108864",
		"fuel = 10000000",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("manifest does not contain %q:\n%s", expected, body)
		}
	}
}

func TestGeneratedRustScaffoldsCompile(t *testing.T) {
	if os.Getenv("RPL_WASM_COMPILE_TESTS") != "1" {
		t.Skip("set RPL_WASM_COMPILE_TESTS=1 to compile generated Rust guest and host")
	}
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not installed")
	}
	root := t.TempDir()
	response := generateWASMResponse(wasmTestPlan())
	for _, file := range response.Files {
		marker := "wasm/user-service/"
		if !strings.HasPrefix(file.Path, marker+"guest/rust/") && !strings.HasPrefix(file.Path, marker+"host/rust/") {
			continue
		}
		path := filepath.Join(root, strings.TrimPrefix(file.Path, marker))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, project := range []string{"guest/rust", "host/rust"} {
		command := exec.Command(cargo, "check", "--quiet")
		command.Dir = filepath.Join(root, project)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("compile generated %s: %v\n%s", project, err, output)
		}
	}
}

func wasmTestPlan() *wasmPlan {
	return &wasmPlan{
		Model: sdk.Model{Name: "UserService"}, Package: "example:users@1.0.0", Namespace: "example",
		PackageName: "users", Version: "1.0.0", World: "user-plugin", Interface: "user-service",
		Guest: "rust", Hosts: map[string]bool{"rust": true}, Root: "user-service",
		MemoryBytes: 67108864, TimeoutMS: 1000, Fuel: 10000000,
		Models: []wasmModel{{
			Name: "UserService", WITName: "user-service",
			Fields: []wasmField{
				{Name: "Id", WITName: "id", Type: sdk.TypeRef{Name: "uint64"}},
				{Name: "Name", WITName: "name", Type: sdk.TypeRef{Name: "string"}},
				{Name: "Email", WITName: "email", Type: sdk.TypeRef{Name: "string", Optional: true}},
				{Name: "Tags", WITName: "tags", Type: sdk.TypeRef{Name: "string", IsList: true}},
			},
		}},
		Methods: []wasmMethod{
			{Name: "GetUser", WITName: "get-user", Params: []wasmValue{{Name: "id", WITName: "id", Type: sdk.TypeRef{Name: "uint64"}}}, Returns: []sdk.TypeRef{{Name: "UserService", Optional: true}}, HasError: true},
			{Name: "Stats", WITName: "stats", Returns: []sdk.TypeRef{{Name: "int64"}, {Name: "float64"}}},
			{Name: "List", WITName: "list", Returns: []sdk.TypeRef{{Name: "UserService", IsList: true}}, HasError: true},
		},
	}
}
