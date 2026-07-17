package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"rpl/pkg/sdk"
	"runtime"
	"strings"
	"testing"
)

func TestFFIGeneratesAndCompilesEveryBinding(t *testing.T) {
	plan := ffiTestPlan()
	response := generateFFIResponse(plan)
	files := make(map[string]string, len(response.Files))
	for _, file := range response.Files {
		files[file.Path] = file.Content
		if strings.Contains(file.Content, "%!") {
			t.Fatalf("formatting failure in %s:\n%s", file.Path, file.Content)
		}
	}
	for _, path := range []string{
		"ffi/calculator_service.h",
		"ffi/schema.json",
		"ffi/c/calculator_service_server.c",
		"ffi/c/calculator_service_client.c",
		"ffi/go/client.gen.go",
		"ffi/go/native_cgo.gen.go",
		"ffi/go/native_purego.gen.go",
		"ffi/go/native_purego_unix.gen.go",
		"ffi/go/native_purego_windows.gen.go",
		"ffi/python/calculator_service_ffi.py",
		"ffi/rust/Cargo.toml",
		"ffi/rust/src/lib.rs",
	} {
		if files[path] == "" {
			t.Fatalf("generated file %s is missing", path)
		}
	}
	for _, path := range []string{
		"ffi/go/client.gen.go",
		"ffi/go/native_cgo.gen.go",
		"ffi/go/native_purego.gen.go",
		"ffi/go/native_purego_unix.gen.go",
		"ffi/go/native_purego_windows.gen.go",
	} {
		if _, err := parser.ParseFile(token.NewFileSet(), path, files[path], parser.AllErrors); err != nil {
			t.Fatalf("parse generated %s: %v\n%s", path, err, files[path])
		}
	}

	root := t.TempDir()
	for path, content := range files {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if compiler, err := exec.LookPath("cc"); err == nil {
		for _, source := range []string{"calculator_service_server.c", "calculator_service_client.c"} {
			command := exec.Command(compiler, "-std=c11", "-Wall", "-Werror", "-c", source)
			command.Dir = filepath.Join(root, "ffi", "c")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("compile %s: %v\n%s", source, err, output)
			}
		}
		testGeneratedCClient(t, root, compiler)
		if goBinary, err := exec.LookPath("go"); err == nil && (runtime.GOOS == "darwin" || runtime.GOOS == "linux") {
			testGeneratedCGOClient(t, root, compiler, goBinary)
		}
	}
	if python, err := exec.LookPath("python3"); err == nil {
		command := exec.Command(python, "-m", "py_compile", "calculator_service_ffi.py")
		command.Dir = filepath.Join(root, "ffi", "python")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("compile generated Python: %v\n%s", err, output)
		}
	}
	if cargo, err := exec.LookPath("cargo"); err == nil {
		testGeneratedRustClient(t, root, cargo)
	}
}

func TestParseFFILanguagesSupportsListsAllAndNone(t *testing.T) {
	selected, err := parseFFILanguages("go; python rust", "", ffiClientLanguages)
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"go", "python", "rust"} {
		if !selected[language] {
			t.Fatalf("language %q was not selected: %#v", language, selected)
		}
	}
	selected, err = parseFFILanguages("all", "", ffiClientLanguages)
	if err != nil || len(selected) != len(ffiClientLanguages) {
		t.Fatalf("all = %#v, %v", selected, err)
	}
	selected, err = parseFFILanguages("none", "", ffiClientLanguages)
	if err != nil || len(selected) != 0 {
		t.Fatalf("none = %#v, %v", selected, err)
	}
	if _, err := parseFFILanguages("javascript", "", ffiClientLanguages); err == nil {
		t.Fatal("unsupported FFI language was accepted")
	}
}

func TestParseFFIClientsTreatsGoAsCGOAndPureGoAsExplicitMode(t *testing.T) {
	clients, goModes, err := parseFFIClients("go,python", "")
	if err != nil {
		t.Fatal(err)
	}
	if !clients["go"] || !clients["python"] {
		t.Fatalf("clients were not selected: %#v", clients)
	}
	if !goModes["cgo"] || goModes["purego"] {
		t.Fatalf("go should select only cgo by default: %#v", goModes)
	}

	clients, goModes, err = parseFFIClients("go:purego,c", "")
	if err != nil {
		t.Fatal(err)
	}
	if !clients["go"] || !clients["c"] {
		t.Fatalf("clients were not selected: %#v", clients)
	}
	if goModes["cgo"] || !goModes["purego"] {
		t.Fatalf("go:purego should select only purego: %#v", goModes)
	}

	_, goModes, err = parseFFIClients("go:all", "")
	if err != nil {
		t.Fatal(err)
	}
	if !goModes["cgo"] || !goModes["purego"] {
		t.Fatalf("go:all should select both Go modes: %#v", goModes)
	}
}

func TestFFIGoClientModesControlGeneratedNativeFiles(t *testing.T) {
	plan := ffiTestPlan()
	plan.Clients = map[string]bool{"go": true}
	plan.GoClientModes = map[string]bool{"cgo": true}
	response := generateFFIResponse(plan)
	files := make(map[string]bool)
	for _, file := range response.Files {
		files[file.Path] = true
	}
	if !files["ffi/go/client.gen.go"] || !files["ffi/go/native_cgo.gen.go"] {
		t.Fatalf("cgo Go client files are missing: %#v", files)
	}
	if files["ffi/go/native_purego.gen.go"] || files["ffi/go/native_purego_unix.gen.go"] || files["ffi/go/native_purego_windows.gen.go"] {
		t.Fatalf("purego files should require go:purego: %#v", files)
	}

	plan.GoClientModes = map[string]bool{"purego": true}
	response = generateFFIResponse(plan)
	files = make(map[string]bool)
	for _, file := range response.Files {
		files[file.Path] = true
	}
	if !files["ffi/go/client.gen.go"] || !files["ffi/go/native_purego.gen.go"] || !files["ffi/go/native_purego_unix.gen.go"] || !files["ffi/go/native_purego_windows.gen.go"] {
		t.Fatalf("purego Go client files are missing: %#v", files)
	}
	if files["ffi/go/native_cgo.gen.go"] {
		t.Fatalf("cgo file should require go or go:cgo: %#v", files)
	}
}

func testGeneratedCClient(t *testing.T, root string, compiler string) {
	t.Helper()
	cDir := filepath.Join(root, "ffi", "c")
	smoke := `#include "../calculator_service.h"
#include <string.h>

static int32_t add(void *context, calculator_ffi_view request, calculator_ffi_buffer *response, calculator_ffi_buffer *failure) {
    (void)context; (void)request; (void)failure;
    const char body[] = "5";
    *response = calculator_ffi_buffer_copy((const uint8_t *)body, sizeof(body) - 1);
    return CALCULATOR_FFI_OK;
}

int main(void) {
    calculator_ffi_service_vtable service = {0};
    service.abi_version = CALCULATOR_FFI_ABI_VERSION;
    service.method_add = add;
    calculator_ffi_server *server = calculator_ffi_server_create(service);
    if (server == NULL) return 10;
    const char request[] = "{\"left\":2,\"right\":3}";
    calculator_ffi_view view = {(const uint8_t *)request, sizeof(request) - 1};
    calculator_ffi_buffer response = {0}, failure = {0};
    int32_t status = calculator_ffi_client_add(server, view, &response, &failure);
    int ok = status == 0 && response.len == 1 && response.data[0] == '5';
    calculator_ffi_buffer_free(response);
    calculator_ffi_buffer_free(failure);
    calculator_ffi_server_destroy(server);
    return ok ? 0 : 20;
}
`
	if err := os.WriteFile(filepath.Join(cDir, "smoke.c"), []byte(smoke), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(cDir, "smoke")
	command := exec.Command(compiler, "-std=c11", "-Wall", "-Werror", "calculator_service_server.c", "calculator_service_client.c", "smoke.c", "-o", binary)
	command.Dir = cDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("link generated C client/server: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("run generated C client/server: %v\n%s", err, output)
	}
}

func testGeneratedCGOClient(t *testing.T, root string, compiler string, goBinary string) {
	t.Helper()
	cDir := filepath.Join(root, "ffi", "c")
	goDir := filepath.Join(root, "ffi", "go")
	service := `#include "../calculator_service.h"

static int32_t add(void *context, calculator_ffi_view request, calculator_ffi_buffer *response, calculator_ffi_buffer *failure) {
    (void)context; (void)request; (void)failure;
    const char body[] = "5";
    *response = calculator_ffi_buffer_copy((const uint8_t *)body, sizeof(body) - 1);
    return CALCULATOR_FFI_OK;
}

calculator_ffi_server *calculator_ffi_server_default(void) {
    calculator_ffi_service_vtable service = {0};
    service.abi_version = CALCULATOR_FFI_ABI_VERSION;
    service.method_add = add;
    return calculator_ffi_server_create(service);
}
`
	if err := os.WriteFile(filepath.Join(cDir, "service.c"), []byte(service), 0o644); err != nil {
		t.Fatal(err)
	}
	library := filepath.Join(cDir, "libcalculator.so")
	args := []string{"-shared", "-fPIC", "calculator_service_server.c", "service.c", "-o", library}
	loaderVariable := "LD_LIBRARY_PATH"
	if runtime.GOOS == "darwin" {
		library = filepath.Join(cDir, "libcalculator.dylib")
		args = []string{"-dynamiclib", "calculator_service_server.c", "service.c", "-o", library}
		loaderVariable = "DYLD_LIBRARY_PATH"
	}
	command := exec.Command(compiler, args...)
	command.Dir = cDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native library for generated Go client: %v\n%s", err, output)
	}
	goModule := "module ffi_smoke\n\ngo 1.22\n\nrequire github.com/ebitengine/purego v0.10.1\n"
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte(goModule), 0o644); err != nil {
		t.Fatal(err)
	}
	command = exec.Command(goBinary, "mod", "download", "github.com/ebitengine/purego")
	command.Dir = goDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("prepare purego test dependency: %v\n%s", err, output)
	}
	factory := `//go:build rpl_ffi_cgo && cgo

package ffigo

/*
#cgo CFLAGS: -I${SRCDIR}/..
#cgo LDFLAGS: -L${SRCDIR}/../c -lcalculator
#include "../calculator_service.h"
extern calculator_ffi_server *calculator_ffi_server_default(void);
*/
import "C"

import "unsafe"

func smokeNative() (*CGONative, func()) {
    handle := C.calculator_ffi_server_default()
    native := NewCGONative(uintptr(unsafe.Pointer(handle)))
    return native, func() { C.calculator_ffi_server_destroy(handle) }
}
`
	if err := os.WriteFile(filepath.Join(goDir, "factory_cgo.go"), []byte(factory), 0o644); err != nil {
		t.Fatal(err)
	}
	goTest := `//go:build rpl_ffi_cgo && cgo

package ffigo

import (
    "context"
    "testing"
)

func TestCGOClientCallsCServer(t *testing.T) {
    native, closeServer := smokeNative()
    defer closeServer()
    client := NewClient(native)
    value, err := client.Add(context.Background(), AddRequest{Left: 2, Right: 3})
    if err != nil { t.Fatal(err) }
    if value != 5 { t.Fatalf("Add() = %d, want 5", value) }
}
`
	if err := os.WriteFile(filepath.Join(goDir, "client_test.go"), []byte(goTest), 0o644); err != nil {
		t.Fatal(err)
	}
	command = exec.Command(goBinary, "test", "-tags", "rpl_ffi_cgo", "./...")
	command.Dir = goDir
	command.Env = append(os.Environ(), "CGO_ENABLED=1", loaderVariable+"="+cDir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run generated Go cgo client against C server: %v\n%s", err, output)
	}
	pureGoTest := fmt.Sprintf(`//go:build rpl_ffi_purego

package ffigo

import (
    "context"
    "testing"
)

func TestPureGoClientCallsCServerWithoutCGO(t *testing.T) {
    native, err := OpenPureGoFromFactory(%q, "")
    if err != nil { t.Fatal(err) }
    defer native.Close()
    client := NewClient(native)
    value, err := client.Add(context.Background(), AddRequest{Left: 2, Right: 3})
    if err != nil { t.Fatal(err) }
    if value != 5 { t.Fatalf("Add() = %%d, want 5", value) }
}
`, library)
	if err := os.WriteFile(filepath.Join(goDir, "purego_test.go"), []byte(pureGoTest), 0o644); err != nil {
		t.Fatal(err)
	}
	command = exec.Command(goBinary, "test", "-tags", "rpl_ffi_purego", "./...")
	command.Dir = goDir
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run generated purego client against C server without cgo: %v\n%s", err, output)
	}
	command = exec.Command(goBinary, "test", "-c", "-tags", "rpl_ffi_purego", "-o", filepath.Join(goDir, "purego_windows.test.exe"))
	command.Dir = goDir
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=windows", "GOARCH=amd64")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("cross-compile generated purego Windows client: %v\n%s", err, output)
	}
}

func testGeneratedRustClient(t *testing.T, root string, cargo string) {
	t.Helper()
	testDir := filepath.Join(root, "ffi", "rust", "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rustSmoke := `use calculator::{exported_ffi_server_destroy, into_raw_server, AddRequest, Client, FFIError, FFIService, StatsRequest, StatsResponse};

struct Service;

impl FFIService for Service {
    fn add(&mut self, request: AddRequest) -> Result<i64, FFIError> {
        Ok(request.left + request.right)
    }

    fn stats(&mut self, _request: StatsRequest) -> Result<StatsResponse, FFIError> {
        Ok(StatsResponse { value1: 3, value2: 1.5 })
    }
}

#[test]
fn generated_rust_client_calls_generated_server() {
    unsafe {
        let handle = into_raw_server(Service);
        let mut client = Client::from_raw(handle).unwrap();
        let value = client.add(&AddRequest { left: 2, right: 3 }).unwrap();
        assert_eq!(value, 5);
        exported_ffi_server_destroy(handle);
    }
}
`
	if err := os.WriteFile(filepath.Join(testDir, "smoke.rs"), []byte(rustSmoke), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(cargo, "test", "--quiet", "--offline")
	command.Dir = filepath.Join(root, "ffi", "rust")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile generated Rust: %v\n%s", err, output)
	}
}

func ffiTestPlan() *ffiPlan {
	return &ffiPlan{
		Model:      sdk.Model{Name: "CalculatorService"},
		Prefix:     "calculator",
		Library:    "calculator",
		ABIVersion: 1,
		Servers:    map[string]bool{"c": true, "rust": true},
		Clients:    map[string]bool{"go": true, "python": true, "c": true, "rust": true},
		GoClientModes: map[string]bool{
			"cgo":    true,
			"purego": true,
		},
		Fields: []ffiField{
			{Name: "ID", WireName: "id", Type: sdk.TypeRef{Name: "int64"}},
			{Name: "Name", WireName: "name", Type: sdk.TypeRef{Name: "string", Optional: true}},
			{Name: "Tags", WireName: "tags", Type: sdk.TypeRef{Name: "string", IsList: true}},
		},
		Methods: []ffiMethod{
			{
				Name: "Add", ExportName: "Add", WireName: "add",
				Params:  []ffiValue{{Name: "left", WireName: "left", Type: sdk.TypeRef{Name: "int64"}}, {Name: "right", WireName: "right", Type: sdk.TypeRef{Name: "int64"}}},
				Returns: []ffiValue{{Name: "value", WireName: "value", Type: sdk.TypeRef{Name: "int64"}}},
			},
			{
				Name: "Stats", ExportName: "Stats", WireName: "stats",
				Returns: []ffiValue{{Name: "value1", WireName: "value1", Type: sdk.TypeRef{Name: "int64"}}, {Name: "value2", WireName: "value2", Type: sdk.TypeRef{Name: "float64"}}},
			},
		},
	}
}
