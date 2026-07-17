package target

import (
	"rpl/internal/generator/parser/ast"
	"rpl/pkg/sdk"
	"strings"
)

// FFI is an artifact-only target. It keeps the normal per-model directory but
// delegates every output file to attrs such as rpl:ffi instead of inventing a
// host-language model.
type FFI struct{}

func init() {
	Register(FFI{})
}

func (FFI) Name() string { return "ffi" }

func (FFI) PackageName() string { return "ffi" }

func (FFI) RootPackageName() string { return "ffi" }

func (FFI) ModelDirName(modelName string) string {
	name := strings.TrimSpace(modelName)
	if name == "" {
		name = "model"
	}
	return sdk.SnakeCase(name)
}

func (FFI) ModelPackageName(string) string { return "ffi" }

func (FFI) ModelFileName(string) string { return "" }

func (FFI) FacadeFileName(string) string { return "" }

func (FFI) GeneratedFileName(string) string { return "" }

func (FFI) BaseModelCode(*ast.File, *ast.ModelAST) string { return "" }

func (FFI) UsedImports(*ast.File, *ast.ModelAST) []sdk.ImportRef { return nil }

func (FFI) RenderFile(sdk.GenerateResponse) ([]byte, error) { return nil, nil }

func (FFI) RenderPackageFile(string, sdk.GenerateResponse) ([]byte, error) { return nil, nil }

func (FFI) FacadeImports(*ast.File, *ast.ModelAST, string, string) []sdk.ImportRef { return nil }

func (FFI) FacadeCode(*ast.File, *ast.ModelAST, string, string) string { return "" }

func (FFI) EmitsHostModel() bool { return false }
