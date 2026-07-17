package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"rpl/internal/config"
	"rpl/pkg/error/localize"
	"strings"
	"testing"
)

func TestAttrInitCreatesScaffold(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()
	t.Setenv("RPL_SDK_PATH", filepath.Clean(filepath.Join(originalWD, "..", "..")))

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	app := New(config.Default())
	if err := app.Execute([]string{"attr", "init", "rpl:demo"}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	assertPathExists(t, filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "manifest.xml"))
	assertPathExists(t, filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "main.go"))
	assertPathExists(t, filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "generate.go"))
	assertPathExists(t, filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "analysis.go"))
	assertPathExists(t, filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "go.mod"))
	assertPathExists(t, filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "README.md"))

	mainBody, err := os.ReadFile(filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	for _, want := range []string{
		`"rpl/pkg/sdk/runtime"`,
		`"rpl/pkg/sdk/analysis"`,
		`runtime.NewAttr`,
		`analysis.PrintError`,
	} {
		if !strings.Contains(string(mainBody), want) {
			t.Fatalf("main.go does not contain %q\n%s", want, string(mainBody))
		}
	}

	generateBody, err := os.ReadFile(filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "generate.go"))
	if err != nil {
		t.Fatalf("read generate.go: %v", err)
	}
	for _, want := range []string{
		`"rpl/pkg/sdk/codegen"`,
		`func generateModel(req codegen.GenerateRequest)`,
		`codegen.NewCodeBuilder()`,
	} {
		if !strings.Contains(string(generateBody), want) {
			t.Fatalf("generate.go does not contain %q\n%s", want, string(generateBody))
		}
	}

	analysisBody, err := os.ReadFile(filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "analysis.go"))
	if err != nil {
		t.Fatalf("read analysis.go: %v", err)
	}

	goModBody, err := os.ReadFile(filepath.Join(tempDir, ".rpl", "attrs", "rpl:demo", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, want := range []string{"module rpl-attr/rpl/demo", "require rpl v0.0.0", "replace rpl =>"} {
		if !strings.Contains(string(goModBody), want) {
			t.Fatalf("go.mod does not contain %q\n%s", want, string(goModBody))
		}
	}
	for _, want := range []string{
		`"rpl/pkg/sdk/analysis"`,
		`"rpl/pkg/sdk/attrs"`,
		`"rpl/pkg/sdk/codegen"`,
		`"rpl/pkg/sdk/docs"`,
		`var attrSpec = attrs.AttrSpec{`,
		`func analyzeModel(req codegen.GenerateRequest) (analysis.AnalyzeResponse, error)`,
		`func docsModel(req docs.DocsRequest) (docs.DocsResponse, error)`,
	} {
		if !strings.Contains(string(analysisBody), want) {
			t.Fatalf("analysis.go does not contain %q\n%s", want, string(analysisBody))
		}
	}
}

func TestAttrInitGlobalUsesUserConfigDirectory(t *testing.T) {
	globalDir := filepath.Join(t.TempDir(), "global")
	t.Setenv(config.GlobalHomeEnv, globalDir)

	app := New(config.Default())
	if err := app.Execute([]string{"attr", "init", "--global", "acme:audit"}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	assertPathExists(t, filepath.Join(globalDir, "config.xml"))
	assertPathExists(t, filepath.Join(globalDir, "attrs", "acme:audit", "manifest.xml"))
	assertPathExists(t, filepath.Join(globalDir, "attrs", "acme:audit", "main.go"))
	assertPathExists(t, filepath.Join(globalDir, "attrs", "acme:audit", "go.mod"))
}

func TestConfigInitCreatesProjectAndGlobalConfigs(t *testing.T) {
	tempDir := t.TempDir()
	globalDir := filepath.Join(tempDir, "global")
	t.Setenv(config.GlobalHomeEnv, globalDir)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWD) })
	projectDir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	app := New(config.Default())
	if err := app.Execute([]string{"config", "init"}); err != nil {
		t.Fatalf("init project config: %v", err)
	}
	if err := app.Execute([]string{"config", "init", "--global"}); err != nil {
		t.Fatalf("init global config: %v", err)
	}

	assertPathExists(t, filepath.Join(projectDir, ".rpl", "config.xml"))
	assertPathExists(t, filepath.Join(globalDir, "config.xml"))
}

func TestDocsGeneratesProjectReadmeFromDefaultSchema(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	if err := os.MkdirAll(filepath.Join(tempDir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	body := `target(lang: golang)

attrs (
    "rpl:grpc",
    "rpl:validate"
)

@comment("User model")
@grpc
model User {
    Name string = "igey" {
        @validate(min: 1, max: 32)
    } (
        func Ping return (User.Name)
    )
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "src", "main.rpl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	app := New(config.Default())
	if err := app.Execute([]string{"docs"}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	readmePath := filepath.Join(tempDir, "README.md")
	assertPathExists(t, readmePath)

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}

	text := string(content)
	for _, want := range []string{
		"README generated by `rpl docs`.",
		"## Models",
		"### User",
		"User model",
		"`Name string` default `\"igey\"`",
		"`Ping() -> string`",
		"`models/user/model.gen.go`",
		"`models/user/validation/validation.gen.go`",
		"`models/user/grpc/user.proto`",
		"`models/user/grpc/user_grpc.pb.go`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("README does not contain %q\n%s", want, text)
		}
	}
}

func TestRenderHelpShowsSectionsAndExamples(t *testing.T) {
	originalLang := localize.Language
	originalColor := localize.UseColor
	localize.SetLanguage(localize.LangEN)
	localize.UseColor = false
	defer func() {
		localize.SetLanguage(originalLang)
		localize.UseColor = originalColor
	}()

	var buffer bytes.Buffer
	renderHelp(&buffer)
	rendered := buffer.String()

	for _, want := range []string{
		"RPL ",
		"Usage",
		"Project",
		"Schema",
		"Attrs",
		"Tools",
		"Examples",
		"rpl <command> [arguments]",
		"rpl attr init rpl:custom",
		"attr info author:name",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("help output does not contain %q\n%s", want, rendered)
		}
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path %q to exist: %v", path, err)
	}
}
