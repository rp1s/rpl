package target

import (
	"path/filepath"
	"rpl/internal/generator/parser/ast"
	"rpl/pkg/sdk"
	"strings"
)

const DefaultLanguage = "golang"

type Renderer interface {
	Name() string
	PackageName() string
	GeneratedFileName(modelName string) string
	BaseModelCode(file *ast.File, model *ast.ModelAST) string
	UsedImports(file *ast.File, model *ast.ModelAST) []sdk.ImportRef
	RenderFile(response sdk.GenerateResponse) ([]byte, error)
}

// StructuredRenderer extends the legacy renderer contract with a layout that is
// friendlier for generated code: one folder per model, optional root facades,
// and explicit per-model package names.
type StructuredRenderer interface {
	Renderer
	RootPackageName() string
	ModelDirName(modelName string) string
	ModelPackageName(modelName string) string
	ModelFileName(modelName string) string
	FacadeFileName(modelName string) string
	FacadeImports(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string) []sdk.ImportRef
	FacadeCode(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string) string
	RenderPackageFile(packageName string, response sdk.GenerateResponse) ([]byte, error)
}

// FacadeFilesRenderer is an optional extension for renderers that need access
// to runtime-generated sidecar files when building the root facade.
type FacadeFilesRenderer interface {
	StructuredRenderer
	FacadeImportsWithFiles(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []sdk.GeneratedFile) []sdk.ImportRef
	FacadeCodeWithFiles(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []sdk.GeneratedFile) string
}

// ModelLayout describes compiler-owned output locations for a generated model.
// Attrs should only emit relative files like "grpc/server.gen.go"; the compiler
// decides the root folder and model directory structure.
type ModelLayout struct {
	RootPackageName string
	ModelDirName    string
	ModelPackage    string
	ModelFileName   string
	MainRelative    string
	FacadeFileName  string
	FacadeRelative  string
}

var registry = make(map[string]Renderer)

func Register(renderer Renderer) {
	if renderer == nil {
		return
	}

	name := NormalizeLanguage(renderer.Name())
	if name == "" {
		return
	}

	registry[name] = renderer
}

func Lookup(name string) (Renderer, bool) {
	renderer, ok := registry[NormalizeLanguage(name)]
	return renderer, ok
}

func NormalizeLanguage(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return DefaultLanguage
	}

	return normalized
}

func ResolveModelLayout(renderer Renderer, modelName string) ModelLayout {
	layout := ModelLayout{
		RootPackageName: renderer.PackageName(),
		ModelPackage:    renderer.PackageName(),
		ModelFileName:   renderer.GeneratedFileName(modelName),
		MainRelative:    normalizeOutputPath(renderer.GeneratedFileName(modelName)),
	}

	structured, ok := renderer.(StructuredRenderer)
	if !ok {
		return layout
	}

	layout.RootPackageName = structured.RootPackageName()
	layout.ModelDirName = structured.ModelDirName(modelName)
	layout.ModelPackage = structured.ModelPackageName(modelName)
	layout.ModelFileName = structured.ModelFileName(modelName)
	layout.MainRelative = normalizeOutputPath(filepath.Join(layout.ModelDirName, layout.ModelFileName))
	layout.FacadeFileName = structured.FacadeFileName(modelName)
	layout.FacadeRelative = normalizeOutputPath(layout.FacadeFileName)
	return layout
}

func normalizeOutputPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	path = strings.TrimPrefix(path, "./")
	return strings.TrimPrefix(path, "/")
}
