package target_test

import (
	"fmt"
	"strings"
	"testing"

	targetpkg "rpl/internal/generator/target"
	"rpl/pkg/sdk/codegen"
	schemapkg "rpl/pkg/sdk/schema"
	targetsdk "rpl/pkg/sdk/target"
)

func TestRegisterSDKRendererBridgesSchemaFirstTargets(t *testing.T) {
	targetpkg.RegisterSDK(sdkTestRenderer{})

	renderer, ok := targetpkg.Lookup("sdktest")
	if !ok {
		t.Fatal("expected sdk renderer to be registered")
	}

	file := parseRawFile(t, `package core

target(lang: sdktest)

import (
    "time"
)

type Email string

model User {
    Email Email
    CreatedAt time.Time
}
`)
	model := mustFindModel(t, file, "User")

	layout := targetpkg.ResolveModelLayout(renderer, model.Name)
	if layout.RootPackageName != "sdkroot" {
		t.Fatalf("unexpected root package: %+v", layout)
	}
	if layout.ModelDirName != "user" || layout.ModelPackage != "user" {
		t.Fatalf("unexpected layout: %+v", layout)
	}

	baseCode := renderer.BaseModelCode(file, model)
	for _, want := range []string{
		"type UserDTO struct{}",
		"// package:core",
		"// type:Email=string",
		"// field:Email=Email",
		"// field:CreatedAt=time.Time",
	} {
		if !strings.Contains(baseCode, want) {
			t.Fatalf("expected sdk-backed base model to contain %q, got:\n%s", want, baseCode)
		}
	}

	structured, ok := renderer.(targetpkg.StructuredRenderer)
	if !ok {
		t.Fatal("expected sdk renderer adapter to expose StructuredRenderer")
	}

	facadeCode := structured.FacadeCode(file, model, "example.com/app/sdkroot/user", "user")
	if !strings.Contains(facadeCode, "type UserFacade struct{}") {
		t.Fatalf("unexpected facade code:\n%s", facadeCode)
	}
}

type sdkTestRenderer struct{}

func (sdkTestRenderer) Name() string {
	return "sdktest"
}

func (sdkTestRenderer) DefaultRootPackage() string {
	return "sdkroot"
}

func (sdkTestRenderer) ModelLayout(modelName string) targetsdk.Layout {
	return targetsdk.OnePackagePerModelLayout("sdkroot", modelName, "model.sdk.go", targetsdk.DefaultFacadeFileName(modelName, "sdk.go"))
}

func (sdkTestRenderer) BaseModelCode(req targetsdk.ModelRequest) string {
	lines := []string{
		fmt.Sprintf("type %sDTO struct{}", req.Model.Name),
		fmt.Sprintf("// package:%s", req.File.PackageName),
	}
	for _, item := range req.File.Types {
		lines = append(lines, fmt.Sprintf("// type:%s=%s", item.Name, item.Type.Name))
	}
	for _, field := range req.Model.Fields {
		lines = append(lines, fmt.Sprintf("// field:%s=%s", field.Name, field.Type.Name))
	}
	return strings.Join(lines, "\n")
}

func (sdkTestRenderer) UsedImports(req targetsdk.ModelRequest) []codegen.ImportRef {
	imports := make([]codegen.ImportRef, 0, len(req.File.Imports))
	for _, item := range req.File.Imports {
		imports = append(imports, codegen.ImportRef(item))
	}
	return imports
}

func (sdkTestRenderer) RenderPackageFile(packageName string, response codegen.GenerateResponse) ([]byte, error) {
	return codegen.RenderGoFile(packageName, response)
}

func (sdkTestRenderer) FacadeImports(req targetsdk.FacadeRequest) []codegen.ImportRef {
	if strings.TrimSpace(req.ModelImportPath) == "" {
		return nil
	}
	return []codegen.ImportRef{{Alias: "modelpkg", Path: req.ModelImportPath}}
}

func (sdkTestRenderer) FacadeCode(req targetsdk.FacadeRequest) string {
	if strings.TrimSpace(req.Model.Name) == "" {
		return ""
	}
	return fmt.Sprintf("type %sFacade struct{}", req.Model.Name)
}

var _ targetsdk.Renderer = sdkTestRenderer{}
var _ = schemapkg.TypeKindString
