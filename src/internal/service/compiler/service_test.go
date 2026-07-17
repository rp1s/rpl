package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"rpl/internal/version"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	bundledAttrBuildMu sync.Mutex
	bundledAttrBuilt   = make(map[string]struct{})
)

func TestRunFileGeneratesOneFilePerModel(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

model User {
    Name string
}

model Profile {
    UserID int
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "package user")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "type User struct")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "// Generated at:")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "// RPL version: "+version.Version)
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "// Author: "+version.GeneratedAuthor())
	assertFileContains(t, filepath.Join(projectDir, "models", "profile", "model.gen.go"), "package profile")
	assertFileContains(t, filepath.Join(projectDir, "models", "profile", "model.gen.go"), "type Profile struct")
}

func TestAutoSetImportsAddsAttrsAndGoImports(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:std")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

model User {
    CreatedAt time.Time
    Name string @comment("User name")
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	updated, err := service.AutoSetImportsFile(sourcePath)
	if err != nil {
		t.Fatalf("auto set imports: %v", err)
	}

	if !strings.Contains(updated, `"rpl:std"`) {
		t.Fatalf("expected std attr import, got:\n%s", updated)
	}
	if !strings.Contains(updated, `import (`) || !strings.Contains(updated, `"time"`) {
		t.Fatalf("expected time import, got:\n%s", updated)
	}
}

func TestAutoSetImportsAddsRuntimeOwnerForNamespacedAttrUsage(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:std")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

model User {
    Name string @std.comment("User name")
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	updated, err := service.AutoSetImportsFile(sourcePath)
	if err != nil {
		t.Fatalf("auto set imports: %v", err)
	}

	if !strings.Contains(updated, `"rpl:std"`) {
		t.Fatalf("expected std attr import for namespaced usage, got:\n%s", updated)
	}
}

func TestAutoSetImportsPreservesLineComments(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:std")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

// модель пользователя
model User {
    // имя
    Name string @comment("User name")
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	updated, err := service.AutoSetImportsFile(sourcePath)
	if err != nil {
		t.Fatalf("auto set imports: %v", err)
	}

	if updated != body {
		t.Fatalf("expected auto import to preserve commented source\nwant:\n%s\ngot:\n%s", body, updated)
	}
}

func TestAutoSetImportsRemovesUnusedAttrs(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")
	buildBundledRuntime(t, "rpl:sql")
	buildBundledRuntime(t, "rpl:std")
	buildBundledRuntime(t, "rpl:validate")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc",
    "rpl:sql",
    "rpl:std",
    "rpl:validate"
)

@grpc
model User {
    Name string
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	updated, err := service.AutoSetImportsFile(sourcePath)
	if err != nil {
		t.Fatalf("auto set imports: %v", err)
	}

	if !strings.Contains(updated, `"rpl:grpc"`) {
		t.Fatalf("expected grpc attr to stay, got:\n%s", updated)
	}
	for _, unexpected := range []string{`"rpl:sql"`, `"rpl:std"`, `"rpl:validate"`} {
		if strings.Contains(updated, unexpected) {
			t.Fatalf("did not expect unused attr %s in:\n%s", unexpected, updated)
		}
	}
}

func TestAutoSetImportsRemovesEmptyAttrsBlock(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")
	buildBundledRuntime(t, "rpl:sql")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc",
    "rpl:sql"
)

model User {
    Name string
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	updated, err := service.AutoSetImportsFile(sourcePath)
	if err != nil {
		t.Fatalf("auto set imports: %v", err)
	}

	if strings.Contains(updated, "attrs (") {
		t.Fatalf("did not expect empty attrs block in:\n%s", updated)
	}
}

func TestRunFileWritesToCustomOutputDir(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "src", "main.rpl")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	body := `target(lang: golang)

model User {
    Name string
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	customOutputDir := filepath.Join(projectDir, "user")
	if _, err := service.RunFileTo(sourcePath, customOutputDir); err != nil {
		t.Fatalf("run file to custom output dir: %v", err)
	}

	assertFileContains(t, filepath.Join(customOutputDir, "user", "model.gen.go"), "package user")
	assertFileContains(t, filepath.Join(customOutputDir, "user", "model.gen.go"), "type User struct")
	assertPathNotExists(t, filepath.Join(customOutputDir, "user.gen.go"))
	assertPathNotExists(t, filepath.Join(projectDir, "src", "models", "user", "model.gen.go"))
}

func TestRunFileToUsesOutputDirNameForRootTypesPackage(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/rplcore\n\ngo 1.25.6\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	sourcePath := filepath.Join(projectDir, "src", "main.rpl")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	body := `target(lang: golang)

type EventName string

model Event {
    Name EventName
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	customOutputDir := filepath.Join(projectDir, "core")
	if _, err := service.RunFileTo(sourcePath, customOutputDir); err != nil {
		t.Fatalf("run file to custom output dir: %v", err)
	}

	assertPathNotExists(t, filepath.Join(customOutputDir, "event.gen.go"))
	assertFileContains(t, filepath.Join(customOutputDir, "types.gen.go"), "package core")
	assertFileContains(t, filepath.Join(customOutputDir, "types.gen.go"), "type EventName = typespkg.EventName")
	assertFileContains(t, filepath.Join(customOutputDir, "event", "model.gen.go"), "package event")
	assertFileContains(t, filepath.Join(customOutputDir, "event", "model.gen.go"), "type EventName = typespkg.EventName")

	runGoTest(t, projectDir)
}

func TestRunFileGeneratesSharedTypeAliases(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:validate")

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/rpltypes\n\ngo 1.25.6\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "time"
)

attrs (
    "rpl:validate"
)

type Email string
type Timestamp time.Time

model User {
    Email Email @validate(email: true)
    CreatedAt Timestamp
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "types", "types.gen.go"), "package types")
	assertFileContains(t, filepath.Join(projectDir, "models", "types", "types.gen.go"), "type Email = string")
	assertFileContains(t, filepath.Join(projectDir, "models", "types", "types.gen.go"), "type Timestamp = time.Time")
	assertFileContains(t, filepath.Join(projectDir, "models", "types.gen.go"), "type Email = typespkg.Email")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "type Email = typespkg.Email")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "type User struct")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "CreatedAt Timestamp")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "validation", "validation.gen.go"), "mail.ParseAddress(model.Email)")

	runGoTest(t, projectDir)
}

func TestRunFileGeneratesTypeAliasesWithoutModels(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/rpltypesonly\n\ngo 1.25.6\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

type Email string
type Score int64
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "types", "types.gen.go"), "type Email = string")
	assertFileContains(t, filepath.Join(projectDir, "models", "types", "types.gen.go"), "type Score = int64")
	assertFileContains(t, filepath.Join(projectDir, "models", "types.gen.go"), "type Email = typespkg.Email")

	runGoTest(t, projectDir)
}

func TestCheckDoesNotGenerateFiles(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

model User {
    Name string
}
`

	if _, err := service.Check(body, sourcePath); err != nil {
		t.Fatalf("check schema: %v", err)
	}

	assertPathNotExists(t, filepath.Join(projectDir, "models"))
}

func TestRunFileLoadsSiblingFilesFromSamePackage(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	sharedPath := filepath.Join(projectDir, "profile.rpl")

	mainBody := `package user

target(lang: golang)

model User {
    Profile Profile
}
`
	sharedBody := `package user

model Profile {
    Name string
}
`
	if err := os.WriteFile(sourcePath, []byte(mainBody), 0o644); err != nil {
		t.Fatalf("write main source: %v", err)
	}
	if err := os.WriteFile(sharedPath, []byte(sharedBody), 0o644); err != nil {
		t.Fatalf("write shared source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "profile", "model.gen.go"), "type Profile struct")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "Profile profile.Profile")
}

func TestRunFileAcceptsMatchingTargetsAcrossPackageFiles(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	sharedPath := filepath.Join(projectDir, "main2.rpl")

	mainBody := `package user

target(lang: golang)

model User {
    Id int
}
`
	sharedBody := `package user

target(lang: golang)

model User2 {
    Id User.Id
}
`
	if err := os.WriteFile(sourcePath, []byte(mainBody), 0o644); err != nil {
		t.Fatalf("write main source: %v", err)
	}
	if err := os.WriteFile(sharedPath, []byte(sharedBody), 0o644); err != nil {
		t.Fatalf("write sibling source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "user2", "model.gen.go"), "type User2 struct")
	assertFileContains(t, filepath.Join(projectDir, "models", "user2", "model.gen.go"), "Id int")
}

func TestRunFileResolvesImportedTypeThroughModelFieldRef(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	sharedPath := filepath.Join(projectDir, "main2.rpl")

	mainBody := `package user

target(lang: golang)

import (
    uuid "github.com/google/uuid"
)

model User {
    Id uuid.UUID
}
`
	sharedBody := `package user

target(lang: golang)

model User2 {
    Id User.Id
}
`
	if err := os.WriteFile(sourcePath, []byte(mainBody), 0o644); err != nil {
		t.Fatalf("write main source: %v", err)
	}
	if err := os.WriteFile(sharedPath, []byte(sharedBody), 0o644); err != nil {
		t.Fatalf("write sibling source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "user2", "model.gen.go"), `"github.com/google/uuid"`)
	assertFileContains(t, filepath.Join(projectDir, "models", "user2", "model.gen.go"), "Id uuid.UUID")
}

func TestRunFileIgnoresSiblingFilesFromDifferentPackage(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	foreignPath := filepath.Join(projectDir, "admin.rpl")

	mainBody := `package user

target(lang: golang)

model User {
    Name string
}
`
	foreignBody := `package admin

model Admin {
    Email string
}
`
	if err := os.WriteFile(sourcePath, []byte(mainBody), 0o644); err != nil {
		t.Fatalf("write main source: %v", err)
	}
	if err := os.WriteFile(foreignPath, []byte(foreignBody), 0o644); err != nil {
		t.Fatalf("write foreign source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "type User struct")
	assertPathNotExists(t, filepath.Join(projectDir, "models", "admin", "model.gen.go"))
}

func TestAutoSetImportsInsertsImportAfterPackageDirective(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `package user

target(lang: golang)

model User {
    CreatedAt time.Time
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	updated, err := service.AutoSetImportsFile(sourcePath)
	if err != nil {
		t.Fatalf("auto set imports: %v", err)
	}

	want := "package user\n\ntarget(lang: golang)\n\nimport (\n\t\"time\"\n)\n\nmodel User"
	if !strings.Contains(updated, want) {
		t.Fatalf("expected import block after package and target, got:\n%s", updated)
	}
}

func TestRunFileGeneratesStableProtoGoPackage(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/acme/app\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc"
)

@grpc
model User {
    Name string
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "user", "grpc", "user.proto"), `option go_package = "example.com/acme/app/models/user/grpc;grpc";`)
}

func TestRunFileRejectsRecursiveImports(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	first := filepath.Join(projectDir, "first.rpl")
	second := filepath.Join(projectDir, "second.rpl")

	if err := os.WriteFile(first, []byte(`import ("second.rpl")`), 0o644); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := os.WriteFile(second, []byte(`import ("first.rpl")`), 0o644); err != nil {
		t.Fatalf("write second: %v", err)
	}

	_, err := service.RunFile(first)
	if err == nil {
		t.Fatal("expected recursive import error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "рекурсив") && !strings.Contains(strings.ToLower(err.Error()), "recursive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFileLoadsImportedDSLModels(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sharedPath := filepath.Join(projectDir, "shared.rpl")
	mainPath := filepath.Join(projectDir, "main.rpl")

	if err := os.WriteFile(sharedPath, []byte(`model Shared {
    ID int
}`), 0o644); err != nil {
		t.Fatalf("write shared: %v", err)
	}

	mainBody := `target(lang: golang)

import (
    "shared.rpl"
)

model User {
    Shared Shared
}
`
	if err := os.WriteFile(mainPath, []byte(mainBody), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	if _, err := service.RunFile(mainPath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "shared", "model.gen.go"), "package shared")
	assertFileContains(t, filepath.Join(projectDir, "models", "shared", "model.gen.go"), "type Shared struct")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "package user")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "model.gen.go"), "Shared shared.Shared")
}

func TestRunFileRejectsUnsupportedTargetLanguage(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: typescript)

model User {
    Name string
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected unsupported target language error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unsupported") && !strings.Contains(strings.ToLower(err.Error()), "неподдерж") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFileSwitchingToFFITargetRemovesStaleHostModel(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	sourcePath := filepath.Join(projectDir, "main.rpl")

	golangSchema := `target(lang: golang)

model Calculator {
    Value int64
}
`
	if err := os.WriteFile(sourcePath, []byte(golangSchema), 0o644); err != nil {
		t.Fatalf("write Go target schema: %v", err)
	}
	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("generate Go target: %v", err)
	}
	hostModel := filepath.Join(projectDir, "models", "calculator", "model.gen.go")
	if _, err := os.Stat(hostModel); err != nil {
		t.Fatalf("expected initial host model: %v", err)
	}

	ffiSchema := strings.Replace(golangSchema, "target(lang: golang)", "target(lang: ffi)", 1)
	if err := os.WriteFile(sourcePath, []byte(ffiSchema), 0o644); err != nil {
		t.Fatalf("write FFI target schema: %v", err)
	}
	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("generate FFI target: %v", err)
	}
	if _, err := os.Stat(hostModel); !os.IsNotExist(err) {
		t.Fatalf("stale host model was not removed: %v", err)
	}
}

func TestRunFileUsesModelSpecificHelpersInModelsPackage(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")
	buildBundledRuntime(t, "rpl:validate")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc",
    "rpl:validate"
)

@grpc
model Profile {
    City string
}

@grpc
model User {
    Name string @validate(min: 1, max: 10)
    Phone string @validate(phone: true)
    Password string @validate(hash: "password")
    Profile Profile
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	modelPath := filepath.Join(projectDir, "models", "user", "model.gen.go")
	facadePath := filepath.Join(projectDir, "models", "user.gen.go")
	validatePath := filepath.Join(projectDir, "models", "user", "validation", "validation.gen.go")
	protoPath := filepath.Join(projectDir, "models", "user", "grpc", "user.proto")
	protoGoPath := filepath.Join(projectDir, "models", "user", "grpc", "user.pb.go")
	serverPath := filepath.Join(projectDir, "models", "user", "grpc", "server.gen.go")
	clientPath := filepath.Join(projectDir, "models", "user", "grpc", "client.gen.go")

	assertFileContains(t, modelPath, "package user")
	assertFileContains(t, modelPath, "// User описывает сгенерированную модель данных.")
	assertFileContains(t, modelPath, "profile.Profile")
	assertFileContains(t, validatePath, "package validation")
	assertFileContains(t, validatePath, "func Errors(model modelpkg.User) []error")
	assertFileContains(t, validatePath, "var userPhonePattern = regexp.MustCompile")
	assertFileContains(t, validatePath, "func userLooksHashed(value string) bool")
	assertFileNotContains(t, validatePath, "var phonePattern =")
	assertFileNotContains(t, validatePath, "func looksHashed(")

	assertFileContains(t, protoPath, "message UserMessage")
	assertFileContains(t, protoPath, "message UserProfileMessage")
	assertFileContains(t, protoPath, "service UserService")
	assertFileContains(t, protoPath, "rpc Put (UserMessage) returns (UserMessage);")
	assertFileContains(t, protoPath, "rpc List (UserListRequest) returns (UserListResponse);")
	assertFileNotContains(t, protoPath, "rpc Apply (UserMessage) returns (UserMessage);")
	assertFileContains(t, protoGoPath, "package grpc")
	assertFileContains(t, serverPath, "type UserService interface")
	assertFileContains(t, serverPath, "Put(ctx context.Context, user modelpkg.User) (modelpkg.User, error)")
	assertFileContains(t, serverPath, "List(ctx context.Context) ([]modelpkg.User, error)")
	assertFileContains(t, serverPath, "func RegisterUserGRPC(registrar grpc.ServiceRegistrar, service UserService)")
	assertFileContains(t, clientPath, "func NewUserGRPCClient(conn grpc.ClientConnInterface) UserService")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) Put(ctx context.Context, user modelpkg.User) (modelpkg.User, error)")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) List(ctx context.Context) ([]modelpkg.User, error)")
	assertFileNotContains(t, clientPath, "type UserGRPCClient interface")
	assertFileNotContains(t, facadePath, "grpcpkg")
	assertFileNotContains(t, facadePath, "RegisterUserGRPC")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "grpc", "user_grpc.pb.go"), "type UserServiceClient interface")
}

func TestRunFileAcceptsAttrsKeyword(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:validate"
)

model User {
    Name string @validate(min: 1, max: 10)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "user", "validation", "validation.gen.go"), "func Validate(model modelpkg.User) error")
}

func TestRunFileGeneratesModelMethodContracts(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "time"
)

model Profile {
    Name string
}

model Position {
    X int64
    Y int64
    Z int64? (
        func Lift(delta int64) return (Position)
    )

    func Is2d return (bool)
    func Touch(at time.Time) return (Profile, error)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	modelPath := filepath.Join(projectDir, "models", "position", "model.gen.go")
	assertFileContains(t, modelPath, "type Position struct")
	assertFileContains(t, modelPath, "type PositionMethods interface")
	assertFileContains(t, modelPath, "Is2d(model *Position) bool")
	assertFileContains(t, modelPath, "Touch(model *Position, at time.Time) (profile.Profile, error)")
	assertFileContains(t, modelPath, "func SetPositionMethods(methods PositionMethods)")
	assertFileContains(t, modelPath, "func (model *Position) Is2d() bool")
	assertFileContains(t, modelPath, "return methods.Is2d(model)")
	assertFileContains(t, modelPath, "type PositionZMethods interface")
	assertFileContains(t, modelPath, "Lift(delta int64) Position")
}

func TestRunFileSupportsBoolAttrShorthand(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:validate"
)

model User {
    Phone string @validate(phone)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	validatePath := filepath.Join(projectDir, "models", "user", "validation", "validation.gen.go")
	assertFileContains(t, validatePath, "var userPhonePattern = regexp.MustCompile")
	assertFileContains(t, validatePath, "Phone must be phone")
}

func TestRunFileRejectsValidateAndSQLAttrAnalysisErrors(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:validate")
	buildBundledRuntime(t, "rpl:sql")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:validate",
    "rpl:sql"
)

model User {
    Name int @validate(email: true)
    Age int @validate(min: "x")
    UpdatedAt string @sql(updatedAt: true)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected attr analysis error")
	}

	text := strings.ToLower(err.Error())
	if !strings.Contains(text, "validate(email") {
		t.Fatalf("expected validate type error, got: %v", err)
	}
	if !strings.Contains(text, "validate argument") && !strings.Contains(text, "аргумент validate") {
		t.Fatalf("expected validate argument type error, got: %v", err)
	}
	if !strings.Contains(text, "updatedat") {
		t.Fatalf("expected sql updatedAt error, got: %v", err)
	}
}

func TestRunFileRejectsGRPCAttrAnalysisErrors(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "github.com/gorilla/websocket"
)

attrs (
    "rpl:grpc"
)

@grpc
model A {
    B B
}

@grpc
model B {
    A A
}

@grpc
model Socket {
    Conn websocket.Conn
}

@grpc
model Socket2 {
    Conn websocket.Conn? @grpc(mode: "inside") (
        func Close return (string?) @grpc.Inside()
    )
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected grpc analysis error")
	}

	text := strings.ToLower(err.Error())
	if !strings.Contains(text, "recursive") && !strings.Contains(text, "рекурс") {
		t.Fatalf("expected recursive grpc error, got: %v", err)
	}
	if !strings.Contains(text, "cannot serialize field") && !strings.Contains(text, "не умеет сериализовать поле") {
		t.Fatalf("expected grpc serialization error, got: %v", err)
	}
}

func TestRunFileRejectsStorageFieldDomainConflicts(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:sql")
	buildBundledRuntime(t, "rpl:redis")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:sql",
    "rpl:redis"
)

model User {
    Name string @sql(index: true) @redis(unique: true)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected storage conflict error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "storage") {
		t.Fatalf("expected storage claim conflict, got: %v", err)
	}
}

func TestRunFileRejectsInvalidGRPCInsideSignatures(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "github.com/gorilla/websocket"
)

attrs (
    "rpl:grpc"
)

@grpc
model Socket2 {
    Conn websocket.Conn? @grpc(mode: "inside") (
        func Close return (string?) @grpc.Inside()
    )
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected grpc inside signature error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "inside") {
		t.Fatalf("expected grpc inside error, got: %v", err)
	}
}

func TestRunFileGeneratesModelDefaultsFunction(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")
	buildBundledRuntime(t, "rpl:sql")
	buildBundledRuntime(t, "rpl:validate")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "time"
)

attrs (
    "rpl:grpc",
    "rpl:sql",
    "rpl:validate"
)

@grpc
@sql(db: "postgres", table: "user")
model User {
    Name string = "name" {
        @validate(min: 1, max: 10)
    }
    CreatedAt time.Time @sql(default: "now")
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	modelPath := filepath.Join(projectDir, "models", "user.gen.go")
	protoPath := filepath.Join(projectDir, "models", "user.proto")
	assertFileContains(t, modelPath, "func DefaultUser() User")
	assertFileContains(t, modelPath, `Name: "name",`)
	assertFileContains(t, protoPath, "string name = 1;")
	assertFileNotContains(t, protoPath, `= "name"`)
}

func TestRunFileAcceptsGoExpressionDefaultsAndMarksImportsUsed(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "math/bits"
)

model User {
    Size int = bits.UintSize
    Tags []string = []string{"a", "b"}
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	assertFileContains(t, filepath.Join(projectDir, "models", "user.gen.go"), "bits.UintSize")
	assertFileContains(t, filepath.Join(projectDir, "models", "user.gen.go"), `[]string{"a", "b"}`)
}

func TestRunFileRejectsInvalidGoExpressionDefault(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

model User {
    Age int = func(
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected invalid Go default expression error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "go") && !strings.Contains(strings.ToLower(err.Error()), "default") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFileRejectsModelFieldReferenceDefault(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

model User {
    Id int
}

model User2 {
    Id int = User.Id
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected model field reference default error")
	}
	if !strings.Contains(err.Error(), "User.Id") {
		t.Fatalf("expected model field reference to be mentioned, got: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "default") {
		t.Fatalf("expected default-related error, got: %v", err)
	}
}

func TestRunFileRejectsIncompatibleFieldDefault(t *testing.T) {
	service := New()
	projectDir := t.TempDir()

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

model User {
    Age int = "name"
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected incompatible default error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "default") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFileExpandsGroupedModelsAndReusesThem(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:std")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:std",
    "rpl:sql",
    "rpl:validate"
)

@sql(db: "postgres", table: "users")
model User {
    Name string @validate(min: 1, max: 10) @group("req")
    Age int @group("req")
}

model AuditLog {
    Actor UserReq
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	groupPath := filepath.Join(projectDir, "models", "user_req", "model.gen.go")
	assertFileContains(t, groupPath, "type UserReq struct")
	assertFileContains(t, groupPath, "// UserReq хранит поля группы, автоматически полученные из модели User.")
	assertFileContains(t, groupPath, "Name string")
	assertFileContains(t, groupPath, "Age  int")
	assertFileNotContains(t, groupPath, "SQLTableName")
	assertFileNotContains(t, groupPath, "Validate() error")

	assertFileContains(t, filepath.Join(projectDir, "models", "audit_log", "model.gen.go"), "userreq.UserReq")
}

func TestRunFileGeneratesStdMetadataHelpers(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:std")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:std"
)

@comment("Основная модель пользователя")
model User {
    Name string @group("data")
    Secret string @ignore("grpc", "sql") (
        func Mask return (string)
    )
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	modelPath := filepath.Join(projectDir, "models", "user", "model.gen.go")
	metaPath := filepath.Join(projectDir, "models", "user", "meta.gen.go")
	assertFileContains(t, modelPath, "// Основная модель пользователя")
	assertFileContains(t, metaPath, "type UserStdFieldMeta struct")
	assertFileContains(t, metaPath, "type UserStdMethodMeta struct")
	assertFileContains(t, metaPath, "func (model User) StdComment() string")
	assertFileContains(t, metaPath, `return "Основная модель пользователя"`)
	assertFileContains(t, metaPath, "func (model User) StdFields() []UserStdFieldMeta")
	assertFileContains(t, metaPath, `"data"`)
	assertFileContains(t, metaPath, `IgnoredBy: []string{"grpc", "sql"}`)
	assertFileContains(t, metaPath, `Name: "Mask"`)
	assertFileContains(t, metaPath, "func (model User) StdField(name string) (UserStdFieldMeta, bool)")
}

func TestRunFileGeneratesFieldCommentDoc(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:std")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:std"
)

model User {
    Name string @comment("value")
    Age int
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	modelPath := filepath.Join(projectDir, "models", "user.gen.go")
	assertFileContains(t, modelPath, "// value\n\tName string")
}

func TestRunFileGeneratesGRPCInsideMethods(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/acme/app\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "github.com/gorilla/websocket"
)

attrs (
    "rpl:grpc"
)

@grpc
model WebSocketConnection {
    Name string (
        func Name return (string) @grpc.Inside()
    )

    Connection websocket.Conn? @grpc.Inside()
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	modelPath := filepath.Join(projectDir, "models", "web_socket_connection", "model.gen.go")
	facadePath := filepath.Join(projectDir, "models", "web_socket_connection.gen.go")
	serverPath := filepath.Join(projectDir, "models", "web_socket_connection", "grpc", "server.gen.go")
	clientPath := filepath.Join(projectDir, "models", "web_socket_connection", "grpc", "client.gen.go")
	protoPath := filepath.Join(projectDir, "models", "web_socket_connection", "grpc", "web_socket_connection.proto")

	assertFileContains(t, modelPath, "package websocketconnection")
	assertFileNotContains(t, facadePath, "GRPCClient")
	assertFileNotContains(t, facadePath, "RegisterWebSocketConnectionGRPC")
	assertFileContains(t, modelPath, "type WebSocketConnection struct")
	assertFileContains(t, serverPath, "type WebSocketConnectionService interface")
	assertFileContains(t, serverPath, "Put(ctx context.Context, webSocketConnection modelpkg.WebSocketConnection) (modelpkg.WebSocketConnection, error)")
	assertFileContains(t, serverPath, "List(ctx context.Context) ([]modelpkg.WebSocketConnection, error)")
	assertFileContains(t, serverPath, "ConnectionReadMessage(ctx context.Context, webSocketConnection modelpkg.WebSocketConnection) (int, []byte, error)")
	assertFileContains(t, serverPath, "ConnectionSetReadDeadline(ctx context.Context, webSocketConnection modelpkg.WebSocketConnection, t time.Time) error")
	assertFileContains(t, serverPath, "type WebSocketConnectionGRPCServer struct")
	assertFileContains(t, serverPath, "// RegisterWebSocketConnectionGRPC регистрирует gRPC сервис модели WebSocketConnection в переданном registrar.")
	assertFileContains(t, serverPath, "func NewWebSocketConnectionGRPCServer(service WebSocketConnectionService) *WebSocketConnectionGRPCServer")
	assertFileContains(t, serverPath, "func RegisterWebSocketConnectionGRPC(registrar grpc.ServiceRegistrar, service WebSocketConnectionService)")
	assertFileContains(t, clientPath, "// NewWebSocketConnectionGRPCClient создает typed gRPC клиент для сервиса модели WebSocketConnection.")
	assertFileContains(t, clientPath, "func NewWebSocketConnectionGRPCClient(conn grpc.ClientConnInterface) WebSocketConnectionService")
	assertFileContains(t, clientPath, "func WrapWebSocketConnectionGRPCClient(client WebSocketConnectionServiceClient) WebSocketConnectionService")
	assertFileContains(t, clientPath, "func (client *webSocketConnectionGRPCClient) ConnectionReadMessage(ctx context.Context, webSocketConnection modelpkg.WebSocketConnection) (int, []byte, error)")
	assertFileContains(t, clientPath, "response, err := client.client.ConnectionReadMessage(ctx, request)")
	assertFileContains(t, clientPath, `return 0, nil, fmt.Errorf("grpc client is nil")`)
	assertFileNotContains(t, serverPath, "InsideHandler")
	assertFileNotContains(t, serverPath, "grpc inside field is nil")
	assertFileNotContains(t, protoPath, "rpc Create (CreateWebSocketConnectionRequest) returns (CreateWebSocketConnectionResponse);")
	assertFileContains(t, protoPath, "rpc Put (WebSocketConnectionMessage) returns (WebSocketConnectionMessage);")
	assertFileContains(t, protoPath, "rpc List (WebSocketConnectionListRequest) returns (WebSocketConnectionListResponse);")
	assertFileNotContains(t, protoPath, "rpc Apply (WebSocketConnectionMessage) returns (WebSocketConnectionMessage);")
	assertFileContains(t, protoPath, "rpc GetName (WebSocketConnectionGetNameRequest) returns (WebSocketConnectionGetNameResponse);")
	assertFileContains(t, protoPath, "rpc ConnectionWriteMessage (WebSocketConnectionConnectionWriteMessageRequest) returns (WebSocketConnectionConnectionWriteMessageResponse);")
	assertFileContains(t, protoPath, "rpc ConnectionSetReadDeadline (WebSocketConnectionConnectionSetReadDeadlineRequest) returns (WebSocketConnectionConnectionSetReadDeadlineResponse);")
	assertFileContains(t, protoPath, "WebSocketConnectionMessage web_socket_connection = 1;")
	assertFileContains(t, protoPath, "bytes data = 3;")
	assertFileContains(t, protoPath, "int64 result = 1;")
	assertFileContains(t, filepath.Join(projectDir, "models", "web_socket_connection", "grpc", "web_socket_connection_grpc.pb.go"), "type WebSocketConnectionServiceClient interface")
}

func TestCheckAllowsGRPCInsideInspectionWithoutGoModule(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "github.com/gorilla/websocket"
)

attrs (
    "rpl:grpc"
)

@grpc
model WebSocketConnection {
    Connection websocket.Conn? @grpc.Inside()
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.Check(body, sourcePath); err != nil {
		t.Fatalf("check file without go.mod: %v", err)
	}
}

func TestRunFileGeneratesCustomGRPCFieldMethods(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")
	buildBundledRuntime(t, "rpl:validate")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:validate",
    "rpl:grpc"
)

@grpc
model User {
    Name string = "igey" {
        @validate(min: 1, max: 32)
    } (
        func Ping return (User.Name)
    )
}

field User.Name {
    @validate(min: 2, max: 1)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	modelPath := filepath.Join(projectDir, "models", "user", "model.gen.go")
	serverPath := filepath.Join(projectDir, "models", "user", "grpc", "server.gen.go")
	clientPath := filepath.Join(projectDir, "models", "user", "grpc", "client.gen.go")
	protoPath := filepath.Join(projectDir, "models", "user", "grpc", "user.proto")

	assertFileContains(t, protoPath, "service UserService")
	assertFileContains(t, protoPath, "rpc Put (UserMessage) returns (UserMessage);")
	assertFileContains(t, protoPath, "rpc List (UserListRequest) returns (UserListResponse);")
	assertFileNotContains(t, protoPath, "rpc Apply (UserMessage) returns (UserMessage);")
	assertFileContains(t, protoPath, "rpc NamePing (UserNamePingRequest) returns (UserNamePingResponse);")
	assertFileContains(t, protoPath, "message UserNamePingRequest {}")
	assertFileNotContains(t, protoPath, "UserMessage user = 1;")
	assertFileContains(t, protoPath, "string result = 1;")

	assertFileContains(t, modelPath, "package user")
	assertFileContains(t, serverPath, "type UserService interface {")
	assertFileContains(t, serverPath, "NamePing(ctx context.Context) (string, error)")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) NamePing(ctx context.Context) (string, error)")
	assertFileContains(t, serverPath, "func RegisterUserGRPC(registrar grpc.ServiceRegistrar, service UserService)")
	assertFileNotContains(t, serverPath, "func (model *User) RegisterGRPC(registrar grpc.ServiceRegistrar)")
	assertFileNotContains(t, serverPath, "HandleApplyGRPC")
	assertFileNotContains(t, serverPath, "InsideHandler")
	assertFileContains(t, clientPath, "response, err := client.client.NamePing(ctx, request)")
	assertFileContains(t, filepath.Join(projectDir, "models", "user", "grpc", "user_grpc.pb.go"), "type UserServiceClient interface")
}

func TestRunFileGeneratesGRPCIDSubjectMethods(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc"
)

@grpc
model User {
    Id int
    {
        @grpc.id()
    }
    (
        func String return (string) @grpc.Model()
    )
    Name string
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	protoPath := filepath.Join(projectDir, "models", "user", "grpc", "user.proto")
	serverPath := filepath.Join(projectDir, "models", "user", "grpc", "server.gen.go")
	clientPath := filepath.Join(projectDir, "models", "user", "grpc", "client.gen.go")

	assertFileContains(t, protoPath, "rpc Put (UserMessage) returns (UserMessage);")
	assertFileContains(t, protoPath, "rpc GetByID (UserGetByIDRequest) returns (UserMessage);")
	assertFileContains(t, protoPath, "rpc Delete (UserDeleteRequest) returns (UserDeleteResponse);")
	assertFileContains(t, protoPath, "rpc List (UserListRequest) returns (UserListResponse);")
	assertFileContains(t, protoPath, "rpc IdString (UserIdStringRequest) returns (UserIdStringResponse);")
	assertFileContains(t, protoPath, "message UserIdStringRequest {")
	assertFileContains(t, protoPath, "int64 id = 1;")
	assertFileContains(t, serverPath, "GetByID(ctx context.Context, id int) (modelpkg.User, error)")
	assertFileContains(t, serverPath, "Delete(ctx context.Context, id int) error")
	assertFileContains(t, serverPath, "IdString(ctx context.Context, id int) (string, error)")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) GetByID(ctx context.Context, id int) (modelpkg.User, error)")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) Delete(ctx context.Context, id int) error")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) IdString(ctx context.Context, id int) (string, error)")
}

func TestRunFileGeneratesGRPCClassicModelMethods(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc"
)

@grpc
model User {
    Id int
    {
        @grpc.id()
    }
    Name string
    func String return (string)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	protoPath := filepath.Join(projectDir, "models", "user", "grpc", "user.proto")
	serverPath := filepath.Join(projectDir, "models", "user", "grpc", "server.gen.go")
	clientPath := filepath.Join(projectDir, "models", "user", "grpc", "client.gen.go")

	assertFileContains(t, protoPath, "rpc String (UserStringRequest) returns (UserStringResponse);")
	assertFileContains(t, protoPath, "message UserStringRequest {}")
	assertFileContains(t, serverPath, "String(ctx context.Context) (string, error)")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) String(ctx context.Context) (string, error)")
}

func TestRunFileGeneratesGRPCModelMethods(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc"
)

@grpc.Model()
@grpc(subject: "id")
model User {
    Id int
    {
        @grpc.id()
    }
    Name string
    func String return (string)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	protoPath := filepath.Join(projectDir, "models", "user", "grpc", "user.proto")
	serverPath := filepath.Join(projectDir, "models", "user", "grpc", "server.gen.go")
	clientPath := filepath.Join(projectDir, "models", "user", "grpc", "client.gen.go")

	assertFileContains(t, protoPath, "rpc String (UserStringRequest) returns (UserStringResponse);")
	assertFileContains(t, protoPath, "message UserStringRequest {")
	assertFileContains(t, protoPath, "int64 id = 1;")
	assertFileNotContains(t, protoPath, "rpc IdString (UserIdStringRequest) returns (UserIdStringResponse);")
	assertFileContains(t, serverPath, "String(ctx context.Context, id int) (string, error)")
	assertFileContains(t, clientPath, "func (client *userGRPCClient) String(ctx context.Context, id int) (string, error)")
}

func TestRunFileGeneratesTransportOSBinShell(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:transport")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:transport"
)

@transport(os.bin)
model User {
    Id int
    {
        @transport.id()
    }
    Name string
    func String return (string)
    func Label return (string) @transport.Model()
    func Hidden return (string) @transport(ignore: true)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	transportPath := filepath.Join(projectDir, "models", "user", "transport", "transport.gen.go")

	assertFileContains(t, transportPath, "package transport")
	assertFileContains(t, transportPath, "type UserTransportService interface {")
	assertFileContains(t, transportPath, "Put(ctx context.Context, user modelpkg.User) (modelpkg.User, error)")
	assertFileContains(t, transportPath, "List(ctx context.Context) ([]modelpkg.User, error)")
	assertFileContains(t, transportPath, "GetByID(ctx context.Context, id int) (modelpkg.User, error)")
	assertFileContains(t, transportPath, "Delete(ctx context.Context, id int) error")
	assertFileContains(t, transportPath, "String(ctx context.Context) (string, error)")
	assertFileContains(t, transportPath, "Label(ctx context.Context, id int) (string, error)")
	assertFileNotContains(t, transportPath, "Hidden(ctx context.Context")
	assertFileContains(t, transportPath, "func RunUserTransportServer(service UserTransportService) error")
	assertFileContains(t, transportPath, "return NewUserTransportServer(service).Serve(os.Stdin, os.Stdout)")
	assertFileContains(t, transportPath, "func NewUserTransportClient(command string, args ...string) (*UserTransportClient, error)")
	assertFileContains(t, transportPath, "cmd := exec.Command(command, args...)")
	assertFileContains(t, transportPath, "encoder: json.NewEncoder(stdin)")
	assertFileContains(t, transportPath, "decoder: json.NewDecoder(stdout)")
	assertFileContains(t, transportPath, "`json:\"method\"`")
}

func TestRunFileRejectsGRPCSubjectWithoutModelBinding(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

attrs (
    "rpl:grpc"
)

@grpc
model User {
    func String return (string) @grpc(subject: "id")
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := service.RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected grpc subject binding error")
	}
	text := strings.ToLower(err.Error())
	if !strings.Contains(text, "grpc.model") && !strings.Contains(text, "@grpc.model()") && !strings.Contains(text, "inside") {
		t.Fatalf("expected grpc.Model hint, got: %v", err)
	}
}

func TestRunFileRemovesStaleGRPCSidecarFiles(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:grpc")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	firstBody := `target(lang: golang)

import (
    "github.com/gorilla/websocket"
)

attrs (
    "rpl:grpc"
)

@grpc
model WebSocketConnection {
    Connection websocket.Conn? @grpc.Inside()
}
`
	if err := os.WriteFile(sourcePath, []byte(firstBody), 0o644); err != nil {
		t.Fatalf("write first source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("first run: %v", err)
	}

	grpcSidecarPath := filepath.Join(projectDir, "models", "web_socket_connection", "grpc", "web_socket_connection_grpc.pb.go")
	assertFileContains(t, grpcSidecarPath, "type WebSocketConnectionServiceClient interface")

	secondBody := `target(lang: golang)

model WebSocketConnection {
    ID int
}
`
	if err := os.WriteFile(sourcePath, []byte(secondBody), 0o644); err != nil {
		t.Fatalf("write second source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("second run: %v", err)
	}

	assertPathNotExists(t, grpcSidecarPath)
	assertPathNotExists(t, filepath.Join(projectDir, "models", "web_socket_connection", "grpc", "web_socket_connection.proto"))
}

func TestRunFileGeneratesSQLCRUDHelpers(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:sql")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "time"
)

attrs (
    "rpl:sql"
)

@sql(db: "postgres", table: "users")
model User {
	ID int {
		@sql(index: true)
		@sql(primaryKey: true)
	}
    Email string @sql(unique: true)
    Name string @sql(index: true)
    CreatedAt time.Time @sql(default: "now")
    UpdatedAt time.Time @sql(default: "now", updatedAt: true)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	schemaPath := filepath.Join(projectDir, "models", "user", "sql", "schema.gen.go")
	scanPath := filepath.Join(projectDir, "models", "user", "sql", "scan.gen.go")
	queriesPath := filepath.Join(projectDir, "models", "user", "sql", "queries.gen.go")
	assertFileContains(t, schemaPath, "type Executor interface")
	assertFileContains(t, schemaPath, "var userSQLCreateStatements = []string{")
	assertFileContains(t, schemaPath, `CREATE TABLE IF NOT EXISTS \"users\"`)
	assertFileNotContains(t, schemaPath, `\\n`)
	assertFileContains(t, schemaPath, `ColumnID`)
	assertFileContains(t, schemaPath, `\"id\" BIGINT NOT NULL PRIMARY KEY`)
	assertFileContains(t, scanPath, "func userSQLScan(scanner interface{")
	assertFileContains(t, queriesPath, "// SQLInit создает таблицу и вспомогательные индексы для модели User.")
	assertFileContains(t, queriesPath, "func Init(ctx context.Context, db Executor) error")
	assertFileContains(t, queriesPath, "// SQLCreate вставляет текущую модель User в базу данных.")
	assertFileContains(t, queriesPath, "func Create(ctx context.Context, db Executor, model modelpkg.User) error")
	assertFileContains(t, queriesPath, "// SQLGet находит одну модель User по фильтрам.")
	assertFileContains(t, queriesPath, "func Get(ctx context.Context, db Executor, filters map[string]any) (modelpkg.User, error)")
	assertFileContains(t, queriesPath, "func Update(ctx context.Context, db Executor, model modelpkg.User, filters map[string]any) error")
	assertFileContains(t, queriesPath, "func Upsert(ctx context.Context, db Executor, model modelpkg.User) error")
	assertFileContains(t, queriesPath, "func Delete(ctx context.Context, db Executor, filters map[string]any) error")
	assertFileContains(t, queriesPath, "func List(ctx context.Context, db Executor, limit int, offset int) ([]modelpkg.User, error)")
	assertFileContains(t, queriesPath, "func Search(ctx context.Context, db Executor, term string, limit int, offset int) ([]modelpkg.User, error)")
	assertFileContains(t, queriesPath, "func NewStore(db Executor) *Store")
	assertFileContains(t, queriesPath, "func (store *Store) Upsert(ctx context.Context, model modelpkg.User) error")
}

func TestRunFileGeneratesSQLiteSQLCRUDHelpers(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:sql")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "time"
)

attrs (
    "rpl:sql"
)

model Profile {
    Theme string
}

@sql(db: "sqlite", table: "players")
model Player {
	    ID int @sql(primaryKey: true)
    Name string @sql(index: true)
    Tags []string
    Profile Profile
    CreatedAt time.Time @sql(default: "now")
    UpdatedAt time.Time @sql(default: "now", updatedAt: true)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	schemaPath := filepath.Join(projectDir, "models", "player", "sql", "schema.gen.go")
	scanPath := filepath.Join(projectDir, "models", "player", "sql", "scan.gen.go")
	queriesPath := filepath.Join(projectDir, "models", "player", "sql", "queries.gen.go")

	assertFileContains(t, schemaPath, `CREATE TABLE IF NOT EXISTS \"players\"`)
	assertFileNotContains(t, schemaPath, `\\n`)
	assertFileContains(t, schemaPath, `\"tags\" TEXT NOT NULL`)
	assertFileContains(t, schemaPath, `\"profile\" TEXT NOT NULL`)
	assertFileContains(t, schemaPath, "VALUES (?, ?, ?, ?, ?, ?);")
	assertFileContains(t, schemaPath, `ON CONFLICT (\"id\")`)

	assertFileContains(t, scanPath, "func playerSQLPlaceholder(columnName string, index int) string")
	assertFileContains(t, scanPath, `return "?"`)
	assertFileContains(t, scanPath, `return "[]"`)
	assertFileContains(t, scanPath, "body, err := json.Marshal(values)")

	assertFileContains(t, queriesPath, `query += " LIMIT " + playerSQLPlaceholder("", len(args))`)
	assertFileContains(t, queriesPath, `query += " LIMIT -1"`)
	assertFileContains(t, queriesPath, `query += " OFFSET " + playerSQLPlaceholder("", len(args))`)
	assertFileContains(t, queriesPath, `fmt.Sprintf("LOWER(%s) LIKE LOWER(%s)", column, playerSQLPlaceholder(column, len(args)))`)
	assertFileContains(t, queriesPath, `return playerSQLQueryMany(ctx, db, query, args...)`)
	assertFileNotContains(t, queriesPath, `%!s(MISSING)`)
}

func TestRunFileGeneratesMongoDBFiles(t *testing.T) {
	service := New()
	projectDir := t.TempDir()
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:mongodb")

	sourcePath := filepath.Join(projectDir, "main.rpl")
	body := `target(lang: golang)

import (
    "time"
)

attrs (
    "rpl:mongodb"
)

model Profile {
    Theme string
}

@mongodb(db: "mongodb", collection: "users")
model User {
    ID string @mongodb(objectId: true, unique: true)
    Email string @mongodb(unique: true, index: true, search: true)
    Username string @mongodb(index: true, search: true, sort: true)
    Profile Profile
    CreatedAt time.Time @mongodb(default: "now", sort: true)
    UpdatedAt time.Time @mongodb(default: "now", updatedAt: true, sort: true)
}
`
	if err := os.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if _, err := service.RunFile(sourcePath); err != nil {
		t.Fatalf("run file: %v", err)
	}

	schemaPath := filepath.Join(projectDir, "models", "user", "mongodb", "schema.gen.go")
	bsonPath := filepath.Join(projectDir, "models", "user", "mongodb", "bson.gen.go")
	queriesPath := filepath.Join(projectDir, "models", "user", "mongodb", "queries.gen.go")

	assertFileContains(t, schemaPath, `const userMongoCollectionName = "users"`)
	assertFileContains(t, schemaPath, `func userMongoIndexes() []mongo.IndexModel`)
	assertFileContains(t, schemaPath, `Value: "text"`)

	assertFileContains(t, bsonPath, "type userMongoDocument struct")
	assertFileContains(t, bsonPath, "func userMongoDocumentMap(model modelpkg.User) (bson.M, error)")
	assertFileContains(t, bsonPath, "`bson:\"_id,omitempty\"`")
	assertFileContains(t, bsonPath, "primitive.ObjectIDFromHex")
	assertFileContains(t, bsonPath, "func userMongoSearchFilter(term string) bson.M")

	assertFileContains(t, queriesPath, "func userMongoInsertOne(ctx context.Context, db *mongo.Database, model modelpkg.User) error")
	assertFileContains(t, queriesPath, "func userMongoFindByID(ctx context.Context, db *mongo.Database, id any) (modelpkg.User, error)")
	assertFileContains(t, queriesPath, "func userMongoUpdateManyFields(ctx context.Context, db *mongo.Database, filters map[string]any, updates map[string]any, upsert bool) error")
	assertFileContains(t, queriesPath, "func userMongoWatch(ctx context.Context, db *mongo.Database, pipeline mongo.Pipeline) (*mongo.ChangeStream, error)")
}

func useRepoRootAsWorkingDir(t *testing.T) {
	t.Helper()
	repoRoot := currentRepoRoot(t)
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("change working directory to repo root: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(currentDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}

func buildBundledRuntime(t *testing.T, identifier string) {
	t.Helper()

	repoRoot := currentRepoRoot(t)
	candidates := []string{
		filepath.Join(repoRoot, "attrs", identifier),
		filepath.Join(repoRoot, ".rpl", "attrs", identifier),
		filepath.Join(repoRoot, ".rpl", "runtimes", identifier),
		filepath.Join(repoRoot, "src", "attrs", identifier),
		filepath.Join(repoRoot, "src", ".rpl", "attrs", identifier),
		filepath.Join(repoRoot, "src", ".rpl", "runtimes", identifier),
	}
	runtimeDir := ""
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			runtimeDir = candidate
			break
		}
	}
	if runtimeDir == "" {
		t.Fatalf("attr %s directory not found in any known bundled location", identifier)
	}
	cacheDir := filepath.Join(t.TempDir(), "go-build")

	bundledAttrBuildMu.Lock()
	if _, ok := bundledAttrBuilt[identifier]; ok {
		bundledAttrBuildMu.Unlock()
		return
	}
	bundledAttrBuildMu.Unlock()

	entries, err := filepath.Glob(filepath.Join(runtimeDir, "*.go"))
	if err != nil {
		t.Fatalf("list attr files %s: %v", identifier, err)
	}
	if len(entries) == 0 {
		t.Fatalf("attr %s does not contain Go files", identifier)
	}

	args := []string{"build", "-o", "attr"}
	for _, item := range entries {
		args = append(args, filepath.Base(item))
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = runtimeDir
	cmd.Env = append(os.Environ(), "GOCACHE="+cacheDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build attr %s: %v\n%s", identifier, err, string(output))
	}

	bundledAttrBuildMu.Lock()
	bundledAttrBuilt[identifier] = struct{}{}
	bundledAttrBuildMu.Unlock()
}

func currentRepoRoot(t *testing.T) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filePath), "..", "..", ".."))
}

func runGoTest(t *testing.T, dir string) {
	t.Helper()

	cacheDir := filepath.Join(t.TempDir(), "go-build")
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE="+cacheDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test %s: %v\n%s", dir, err, string(output))
	}
}

func assertFileContains(t *testing.T, path string, want string) {
	t.Helper()

	path = resolveGeneratedTestPath(path)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %q: %v", path, err)
	}

	if !strings.Contains(string(body), want) {
		t.Fatalf("generated file %q does not contain %q\n%s", path, want, string(body))
	}
}

func assertFileNotContains(t *testing.T, path string, want string) {
	t.Helper()

	path = resolveGeneratedTestPath(path)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %q: %v", path, err)
	}

	if strings.Contains(string(body), want) {
		t.Fatalf("generated file %q unexpectedly contains %q\n%s", path, want, string(body))
	}
}

func assertPathNotExists(t *testing.T, path string) {
	t.Helper()

	path = resolveGeneratedTestPath(path)
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected path %q to be absent", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat path %q: %v", path, err)
	}
}

func resolveGeneratedTestPath(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if filepath.Base(dir) != "models" {
		return path
	}

	switch {
	case strings.HasSuffix(base, ".gen.go"):
		stem := strings.TrimSuffix(base, ".gen.go")
		return filepath.Join(dir, stem, "model.gen.go")
	case strings.HasSuffix(base, "_grpc.pb.go"):
		stem := strings.TrimSuffix(base, "_grpc.pb.go")
		return filepath.Join(dir, stem, "grpc", base)
	case strings.HasSuffix(base, ".pb.go"):
		stem := strings.TrimSuffix(base, ".pb.go")
		return filepath.Join(dir, stem, "grpc", base)
	case strings.HasSuffix(base, ".proto"):
		stem := strings.TrimSuffix(base, ".proto")
		return filepath.Join(dir, stem, "grpc", base)
	default:
		return path
	}
}
