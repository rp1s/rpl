package target

import (
	"strings"

	"rpl/internal/generator/parser/ast"
	rootsdk "rpl/pkg/sdk"
	schemapkg "rpl/pkg/sdk/schema"
	targetsdk "rpl/pkg/sdk/target"
)

// RegisterSDK bridges a schema-first SDK renderer into the compiler target registry.
func RegisterSDK(renderer targetsdk.Renderer) {
	if renderer == nil {
		return
	}

	Register(sdkRendererAdapter{renderer: renderer})
}

type sdkRendererAdapter struct {
	renderer targetsdk.Renderer
}

func (adapter sdkRendererAdapter) Name() string {
	return adapter.renderer.Name()
}

func (adapter sdkRendererAdapter) PackageName() string {
	return adapter.renderer.DefaultRootPackage()
}

func (adapter sdkRendererAdapter) GeneratedFileName(modelName string) string {
	return strings.TrimSpace(adapter.layout(modelName).ModelFileName)
}

func (adapter sdkRendererAdapter) BaseModelCode(file *ast.File, model *ast.ModelAST) string {
	return adapter.renderer.BaseModelCode(targetModelRequest(file, model))
}

func (adapter sdkRendererAdapter) UsedImports(file *ast.File, model *ast.ModelAST) []rootsdk.ImportRef {
	return adapter.renderer.UsedImports(targetModelRequest(file, model))
}

func (adapter sdkRendererAdapter) RenderFile(response rootsdk.GenerateResponse) ([]byte, error) {
	return adapter.renderer.RenderPackageFile(adapter.PackageName(), response)
}

func (adapter sdkRendererAdapter) RootPackageName() string {
	return adapter.renderer.DefaultRootPackage()
}

func (adapter sdkRendererAdapter) ModelDirName(modelName string) string {
	return strings.TrimSpace(adapter.layout(modelName).ModelDirName)
}

func (adapter sdkRendererAdapter) ModelPackageName(modelName string) string {
	return strings.TrimSpace(adapter.layout(modelName).ModelPackage)
}

func (adapter sdkRendererAdapter) ModelFileName(modelName string) string {
	return strings.TrimSpace(adapter.layout(modelName).ModelFileName)
}

func (adapter sdkRendererAdapter) FacadeFileName(modelName string) string {
	return strings.TrimSpace(adapter.layout(modelName).FacadeFileName)
}

func (adapter sdkRendererAdapter) FacadeImports(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string) []rootsdk.ImportRef {
	return adapter.renderer.FacadeImports(targetFacadeRequest(file, model, modelImportPath, modelPackageName, nil))
}

func (adapter sdkRendererAdapter) FacadeCode(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string) string {
	return adapter.renderer.FacadeCode(targetFacadeRequest(file, model, modelImportPath, modelPackageName, nil))
}

func (adapter sdkRendererAdapter) RenderPackageFile(packageName string, response rootsdk.GenerateResponse) ([]byte, error) {
	return adapter.renderer.RenderPackageFile(packageName, response)
}

func (adapter sdkRendererAdapter) FacadeImportsWithFiles(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []rootsdk.GeneratedFile) []rootsdk.ImportRef {
	return adapter.renderer.FacadeImports(targetFacadeRequest(file, model, modelImportPath, modelPackageName, generatedFiles))
}

func (adapter sdkRendererAdapter) FacadeCodeWithFiles(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []rootsdk.GeneratedFile) string {
	return adapter.renderer.FacadeCode(targetFacadeRequest(file, model, modelImportPath, modelPackageName, generatedFiles))
}

func (adapter sdkRendererAdapter) layout(modelName string) targetsdk.Layout {
	layout := adapter.renderer.ModelLayout(modelName)
	if strings.TrimSpace(layout.RootPackageName) == "" {
		layout.RootPackageName = adapter.renderer.DefaultRootPackage()
	}
	return layout
}

func targetModelRequest(file *ast.File, model *ast.ModelAST) targetsdk.ModelRequest {
	sdkFile := targetFile(file)
	sdkModel := targetModel(file, model)
	return targetsdk.ModelRequest{
		File:  sdkFile,
		Model: sdkModel,
	}
}

func targetFacadeRequest(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []rootsdk.GeneratedFile) targetsdk.FacadeRequest {
	return targetsdk.FacadeRequest{
		File:            targetFile(file),
		Model:           targetModel(file, model),
		ModelImportPath: strings.TrimSpace(modelImportPath),
		ModelPackage:    strings.TrimSpace(modelPackageName),
		GeneratedFiles:  append([]rootsdk.GeneratedFile(nil), generatedFiles...),
	}
}

func targetFile(file *ast.File) targetsdk.File {
	if file == nil {
		return targetsdk.File{}
	}

	models := make([]schemapkg.Model, 0, len(file.Models()))
	for _, model := range file.Models() {
		if model == nil {
			continue
		}
		models = append(models, targetModel(file, model))
	}

	types := make([]targetsdk.TypeAlias, 0, len(file.Types()))
	for _, item := range file.Types() {
		if item == nil {
			continue
		}
		types = append(types, targetsdk.TypeAlias{
			Name: strings.TrimSpace(item.Name),
			Type: targetTypeRef(file, item.Type),
		})
	}

	return targetsdk.File{
		Language:    strings.TrimSpace(file.TargetLang()),
		PackageName: strings.TrimSpace(file.PackageName()),
		Imports:     targetImportRefs(fileImports(file)),
		Models:      models,
		Types:       types,
	}
}

func targetModel(file *ast.File, model *ast.ModelAST) schemapkg.Model {
	if model == nil {
		return schemapkg.Model{}
	}

	fields := make([]schemapkg.Field, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		fields = append(fields, schemapkg.Field{
			Name:    field.Name,
			Type:    targetTypeRef(file, field.Type),
			Attrs:   targetAttrs(field.Attrs),
			Methods: targetMethods(file, field.Methods),
		})
	}

	return schemapkg.Model{
		Name:    model.Name,
		Attrs:   targetAttrs(model.Attrs),
		Fields:  fields,
		Methods: targetMethods(file, model.Methods),
	}
}

func targetMethods(file *ast.File, methods []ast.FieldMethodAST) []schemapkg.Method {
	if len(methods) == 0 {
		return nil
	}

	items := make([]schemapkg.Method, 0, len(methods))
	for _, method := range methods {
		params := make([]schemapkg.MethodParam, 0, len(method.Params))
		for _, param := range method.Params {
			params = append(params, schemapkg.MethodParam{
				Name: param.Name,
				Type: targetTypeRef(file, param.Type),
			})
		}

		returns := make([]schemapkg.TypeRef, 0, len(method.Returns))
		for _, result := range method.Returns {
			returns = append(returns, targetTypeRef(file, result))
		}

		items = append(items, schemapkg.Method{
			Name:    method.Name,
			Params:  params,
			Returns: returns,
			Attrs:   targetAttrs(method.Attrs),
		})
	}

	return items
}

func targetTypeRef(file *ast.File, typeRef ast.TypeRef) schemapkg.TypeRef {
	typeRef = resolveModelFieldTypeRef(file, typeRef)
	return schemapkg.TypeRef{
		Name:     typeRef.Name.String(),
		IsList:   typeRef.IsList,
		Optional: typeRef.Optional,
	}
}

func targetAttrs(items []ast.Attr) []schemapkg.Attr {
	if len(items) == 0 {
		return nil
	}

	attrs := make([]schemapkg.Attr, 0, len(items))
	for _, item := range items {
		attrs = append(attrs, schemapkg.Attr{
			Package:    item.Package.String(),
			Name:       item.Name.String(),
			Identifier: item.Identifier(),
			Args:       targetValues(item.Args),
			NamedArgs:  targetNamedValues(item.NamedArgs),
		})
	}

	return attrs
}

func targetImportRefs(items []rootsdk.ImportRef) []schemapkg.ImportRef {
	if len(items) == 0 {
		return nil
	}

	refs := make([]schemapkg.ImportRef, 0, len(items))
	for _, item := range items {
		refs = append(refs, schemapkg.ImportRef{
			Alias: item.Alias,
			Path:  item.Path,
		})
	}
	return refs
}

func targetValues(items []ast.Expr) []schemapkg.Value {
	if len(items) == 0 {
		return nil
	}

	values := make([]schemapkg.Value, 0, len(items))
	for _, item := range items {
		values = append(values, targetValue(item))
	}
	return values
}

func targetNamedValues(items []ast.NamedArg) []schemapkg.NamedValue {
	if len(items) == 0 {
		return nil
	}

	values := make([]schemapkg.NamedValue, 0, len(items))
	for _, item := range items {
		values = append(values, schemapkg.NamedValue{
			Name:  item.Name,
			Value: targetValue(item.Value),
		})
	}
	return values
}

func targetValue(expr ast.Expr) schemapkg.Value {
	switch value := expr.(type) {
	case ast.StringExpr:
		return schemapkg.Value{Kind: "string", Text: value.Value}
	case ast.NumberExpr:
		return schemapkg.Value{Kind: "number", Text: value.Value}
	case ast.BoolExpr:
		return schemapkg.Value{Kind: "bool", Bool: value.Value}
	case ast.NameExpr:
		return schemapkg.Value{Kind: "name", Text: value.Name.String()}
	case ast.GoExpr:
		return schemapkg.Value{Kind: "go", Text: value.Text}
	default:
		return schemapkg.Value{}
	}
}
