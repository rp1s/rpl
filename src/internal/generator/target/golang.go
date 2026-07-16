package target

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/format"
	goparser "go/parser"
	goprinter "go/printer"
	gotoken "go/token"
	"path"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/version"
	"rpl/pkg/sdk"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultGeneratedPkg = "models"
const localTypesImportAlias = "typespkg"

type Golang struct{}

func init() {
	Register(Golang{})
}

func (Golang) Name() string {
	return DefaultLanguage
}

func (Golang) PackageName() string {
	return defaultGeneratedPkg
}

func (Golang) RootPackageName() string {
	return defaultGeneratedPkg
}

func (Golang) ModelDirName(modelName string) string {
	name := strings.TrimSpace(modelName)
	if name == "" {
		name = "model"
	}

	return sdk.SnakeCase(name)
}

func (renderer Golang) ModelPackageName(modelName string) string {
	name := strings.ReplaceAll(renderer.ModelDirName(modelName), "_", "")
	if strings.TrimSpace(name) == "" {
		return "model"
	}
	return name
}

func (Golang) ModelFileName(string) string {
	return "model.gen.go"
}

func (Golang) FacadeFileName(modelName string) string {
	return ""
}

func (Golang) GeneratedFileName(modelName string) string {
	return Golang{}.ModelFileName(modelName)
}

func (Golang) BaseModelCode(file *ast.File, model *ast.ModelAST) string {
	var builder strings.Builder
	defaultsCode := modelDefaultsCode(model)
	methodContractsCode := modelMethodContractsCode(file, model)
	localTypeAliasesCode := modelLocalTypeAliasesCode(file, model)
	modelComment := modelDocComment(model)

	if model != nil && strings.TrimSpace(model.GeneratedFrom) != "" {
		builder.WriteString(sdk.DocComment(
			"%s хранит поля группы, автоматически полученные из модели %s.",
			"%s stores the group fields automatically derived from model %s.",
			model.Name,
			model.GeneratedFrom,
		))
		builder.WriteString("\n")
	} else if model != nil && strings.TrimSpace(modelComment) != "" {
		builder.WriteString(sdk.DocComment(
			modelComment,
			modelComment,
		))
		builder.WriteString("\n")
	} else if model != nil {
		builder.WriteString(sdk.DocComment(
			"%s описывает сгенерированную модель данных.",
			"%s describes the generated data model.",
			model.Name,
		))
		builder.WriteString("\n")
	}

	if localTypeAliasesCode != "" {
		builder.WriteString(localTypeAliasesCode)
		builder.WriteString("\n\n")
	}

	builder.WriteString("type ")
	builder.WriteString(model.Name)
	builder.WriteString(" struct {\n")
	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		if fieldComment := fieldDocComment(field); fieldComment != "" {
			builder.WriteString("\t")
			builder.WriteString(sdk.DocComment(fieldComment, fieldComment))
			builder.WriteString("\n")
		}

		builder.WriteString("\t")
		builder.WriteString(field.Name)
		builder.WriteString(" ")
		builder.WriteString(goType(file, model, field.Type))
		builder.WriteString("\n")
	}
	builder.WriteString("}")

	if methodContractsCode != "" {
		builder.WriteString("\n\n")
		builder.WriteString(methodContractsCode)
	}

	if defaultsCode != "" {
		builder.WriteString("\n\n")
		builder.WriteString(defaultsCode)
	}

	return builder.String()
}

func (Golang) UsedImports(file *ast.File, model *ast.ModelAST) []sdk.ImportRef {
	used := make(map[string]sdk.ImportRef)
	importIndex := make(map[string]sdk.ImportRef)
	for _, item := range fileImports(file) {
		importIndex[item.Alias] = item
	}

	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		for _, alias := range importAliasesFromExpr(field.Default, importIndex) {
			item := importIndex[alias]
			used[item.Alias+"|"+item.Path] = item
		}

		resolvedType := resolveModelFieldTypeRef(file, field.Type)
		if len(resolvedType.Name.Parts) < 2 {
			continue
		}

		prefix := resolvedType.Name.Parts[0]
		if item, ok := importIndex[prefix]; ok {
			used[item.Alias+"|"+item.Path] = item
		}
	}

	for _, method := range model.Methods {
		collectMethodTypeImports(used, importIndex, file, method)
	}
	for _, field := range model.Fields {
		if field == nil {
			continue
		}
		for _, method := range field.Methods {
			collectMethodTypeImports(used, importIndex, file, method)
		}
	}
	if len(model.Methods) > 0 {
		used["|sync"] = sdk.ImportRef{Path: "sync"}
	}

	items := make([]sdk.ImportRef, 0, len(used))
	for _, item := range used {
		items = append(items, item)
	}

	sort.Slice(items, func(i int, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Alias < items[j].Alias
		}

		return items[i].Path < items[j].Path
	})

	return items
}

func collectMethodTypeImports(used map[string]sdk.ImportRef, importIndex map[string]sdk.ImportRef, file *ast.File, method ast.FieldMethodAST) {
	for _, param := range method.Params {
		collectTypeRefImport(used, importIndex, file, param.Type)
	}
	for _, result := range method.Returns {
		collectTypeRefImport(used, importIndex, file, result)
	}
}

func collectTypeRefImport(used map[string]sdk.ImportRef, importIndex map[string]sdk.ImportRef, file *ast.File, typeRef ast.TypeRef) {
	resolvedType := resolveModelFieldTypeRef(file, typeRef)
	if len(resolvedType.Name.Parts) < 2 {
		return
	}

	prefix := resolvedType.Name.Parts[0]
	if item, ok := importIndex[prefix]; ok {
		used[item.Alias+"|"+item.Path] = item
	}
}

func modelMethodContractsCode(file *ast.File, model *ast.ModelAST) string {
	if model == nil {
		return ""
	}

	blocks := make([]string, 0)

	if len(model.Methods) > 0 {
		blocks = append(blocks, renderModelMethodSupportBlock(file, model, model.Methods))
	}

	for _, field := range model.Fields {
		if field == nil || len(field.Methods) == 0 {
			continue
		}
		blocks = append(blocks, renderFieldMethodInterfaceBlock(file, model, field, field.Methods))
	}

	items := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		items = append(items, block)
	}

	return strings.Join(items, "\n\n")
}

func modelLocalTypeAliasesCode(file *ast.File, model *ast.ModelAST) string {
	if file == nil || model == nil {
		return ""
	}

	seen := make(map[string]struct{})
	appendType := func(typeRef ast.TypeRef) {
		resolved := resolveModelFieldTypeRef(file, typeRef)
		if len(resolved.Name.Parts) != 1 {
			return
		}

		name := strings.TrimSpace(resolved.Name.String())
		if name == "" {
			return
		}
		if _, ok := file.FindType(name); !ok {
			return
		}
		seen[name] = struct{}{}
	}

	for _, field := range model.Fields {
		if field == nil {
			continue
		}
		appendType(field.Type)
		for _, method := range field.Methods {
			for _, param := range method.Params {
				appendType(param.Type)
			}
			for _, result := range method.Returns {
				appendType(result)
			}
		}
	}
	for _, method := range model.Methods {
		for _, param := range method.Params {
			appendType(param.Type)
		}
		for _, result := range method.Returns {
			appendType(result)
		}
	}

	if len(seen) == 0 {
		return ""
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, sdk.WithDocComment(
			fmt.Sprintf("type %s = %s.%s", name, localTypesImportAlias, name),
			"%s переэкспортирует shared RPL type alias `%s` внутри model package.",
			"%s re-exports the shared RPL type alias `%s` inside the model package.",
			name,
			name,
		))
	}

	return strings.Join(lines, "\n\n")
}

func renderModelMethodSupportBlock(file *ast.File, model *ast.ModelAST, methods []ast.FieldMethodAST) string {
	if len(methods) == 0 {
		return ""
	}

	interfaceName := model.Name + "Methods"
	setterName := "Set" + model.Name + "Methods"
	holderName := sdk.LowerCamel(model.Name) + "MethodsImpl"
	mutexName := sdk.LowerCamel(model.Name) + "MethodsMu"
	lookupName := sdk.LowerCamel(model.Name) + "Methods"

	var builder strings.Builder
	builder.WriteString(sdk.DocComment(
		"%s описывает реализацию model methods для %s, объявленных в RPL-схеме.",
		"%s describes the implementation of model methods for %s declared in the RPL schema.",
		interfaceName,
		model.Name,
	))
	builder.WriteString("\n")
	builder.WriteString("type ")
	builder.WriteString(interfaceName)
	builder.WriteString(" interface {\n")
	for _, method := range methods {
		builder.WriteString("\t")
		builder.WriteString(goDelegatedMethodSignature(file, model, method))
		builder.WriteString("\n")
	}
	builder.WriteString("}")

	builder.WriteString("\n\n")
	builder.WriteString("var (\n")
	builder.WriteString("\t")
	builder.WriteString(mutexName)
	builder.WriteString(" sync.RWMutex\n")
	builder.WriteString("\t")
	builder.WriteString(holderName)
	builder.WriteString(" ")
	builder.WriteString(interfaceName)
	builder.WriteString("\n")
	builder.WriteString(")")

	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func %s(methods %s) {\n\t%s.Lock()\n\tdefer %s.Unlock()\n\t%s = methods\n}", setterName, interfaceName, mutexName, mutexName, holderName),
		"%s регистрирует реализацию методов модели %s.",
		"%s registers the implementation of %s model methods.",
		setterName,
		model.Name,
	))

	builder.WriteString("\n\n")
	builder.WriteString(fmt.Sprintf("func %s() %s {\n\t%s.RLock()\n\tdefer %s.RUnlock()\n\treturn %s\n}", lookupName, interfaceName, mutexName, mutexName, holderName))

	for _, method := range methods {
		builder.WriteString("\n\n")
		builder.WriteString(renderDelegatedModelMethod(file, model, lookupName, interfaceName, setterName, method))
	}

	return builder.String()
}

func renderFieldMethodInterfaceBlock(file *ast.File, model *ast.ModelAST, field *ast.FieldAST, methods []ast.FieldMethodAST) string {
	if field == nil || len(methods) == 0 {
		return ""
	}

	interfaceName := model.Name + field.Name + "Methods"
	targetName := model.Name + "." + field.Name

	var builder strings.Builder
	builder.WriteString(sdk.DocComment(
		"%s перечисляет field methods для %s, объявленные в RPL-схеме.",
		"%s lists the field methods for %s declared in the RPL schema.",
		interfaceName,
		targetName,
	))
	builder.WriteString("\n")
	builder.WriteString("type ")
	builder.WriteString(interfaceName)
	builder.WriteString(" interface {\n")
	for _, method := range methods {
		builder.WriteString("\t")
		builder.WriteString(goMethodSignature(file, model, method))
		builder.WriteString("\n")
	}
	builder.WriteString("}")
	return builder.String()
}

func goMethodSignature(file *ast.File, currentModel *ast.ModelAST, method ast.FieldMethodAST) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(method.Name))
	builder.WriteString(goMethodParamList(file, currentModel, method.Params))
	builder.WriteString(goMethodResultList(file, currentModel, method.Returns))
	return builder.String()
}

func goDelegatedMethodSignature(file *ast.File, currentModel *ast.ModelAST, method ast.FieldMethodAST) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(method.Name))
	builder.WriteString("(")
	builder.WriteString("model *")
	builder.WriteString(strings.TrimSpace(currentModel.Name))
	for _, item := range goMethodParams(file, currentModel, method.Params) {
		builder.WriteString(", ")
		builder.WriteString(item)
	}
	builder.WriteString(")")
	builder.WriteString(goMethodResultList(file, currentModel, method.Returns))
	return builder.String()
}

func renderDelegatedModelMethod(file *ast.File, model *ast.ModelAST, lookupName string, interfaceName string, setterName string, method ast.FieldMethodAST) string {
	var builder strings.Builder
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func (model *%s) %s%s%s {\n\tmethods := %s()\n\tif methods == nil {\n\t\tpanic(%q)\n\t}\n\t%s\n}",
			model.Name,
			strings.TrimSpace(method.Name),
			goMethodParamList(file, model, method.Params),
			goMethodResultList(file, model, method.Returns),
			lookupName,
			interfaceName+" is not configured; call "+setterName,
			goDelegatedMethodBody(method),
		),
		"%s вызывает зарегистрированную реализацию метода %s для модели %s.",
		"%s calls the registered implementation of %s for model %s.",
		strings.TrimSpace(method.Name),
		strings.TrimSpace(method.Name),
		model.Name,
	))
	return builder.String()
}

func goDelegatedMethodBody(method ast.FieldMethodAST) string {
	callArgs := make([]string, 0, len(method.Params)+1)
	callArgs = append(callArgs, "model")
	for _, param := range method.Params {
		callArgs = append(callArgs, strings.TrimSpace(param.Name))
	}
	call := "methods." + strings.TrimSpace(method.Name) + "(" + strings.Join(callArgs, ", ") + ")"
	if len(method.Returns) == 0 {
		return call
	}
	return "return " + call
}

func goMethodParamList(file *ast.File, currentModel *ast.ModelAST, params []ast.FieldMethodParamAST) string {
	return "(" + strings.Join(goMethodParams(file, currentModel, params), ", ") + ")"
}

func goMethodParams(file *ast.File, currentModel *ast.ModelAST, params []ast.FieldMethodParamAST) []string {
	items := make([]string, 0, len(params))
	for _, param := range params {
		items = append(items, strings.TrimSpace(param.Name)+" "+goType(file, currentModel, param.Type))
	}
	return items
}

func goMethodResultList(file *ast.File, currentModel *ast.ModelAST, returns []ast.TypeRef) string {
	switch len(returns) {
	case 0:
		return ""
	case 1:
		return " " + goType(file, currentModel, returns[0])
	default:
		items := make([]string, 0, len(returns))
		for _, result := range returns {
			items = append(items, goType(file, currentModel, result))
		}
		return " (" + strings.Join(items, ", ") + ")"
	}
}

func (Golang) RenderFile(response sdk.GenerateResponse) ([]byte, error) {
	return Golang{}.RenderPackageFile(defaultGeneratedPkg, response)
}

// RenderPackageFile wraps generated blocks into a compilable Go file and falls
// back to the unformatted source when `go/format` cannot parse it yet.
func (Golang) RenderPackageFile(packageName string, response sdk.GenerateResponse) ([]byte, error) {
	var builder strings.Builder

	builder.WriteString("// Code generated by RPL. DO NOT EDIT.\n")
	builder.WriteString("// Generated at: ")
	builder.WriteString(time.Now().Format(time.RFC3339))
	builder.WriteString("\n")
	builder.WriteString("// RPL version: ")
	builder.WriteString(version.Version)
	builder.WriteString("\n")
	builder.WriteString("// Author: ")
	builder.WriteString(version.GeneratedAuthor())
	builder.WriteString("\n")
	builder.WriteString("package ")
	if strings.TrimSpace(packageName) == "" {
		packageName = defaultGeneratedPkg
	}
	builder.WriteString(packageName)
	builder.WriteString("\n\n")

	if len(response.Imports) > 0 {
		builder.WriteString("import (\n")
		for _, item := range response.Imports {
			if strings.TrimSpace(item.Alias) != "" {
				builder.WriteString("\t")
				builder.WriteString(item.Alias)
				builder.WriteString(" ")
			} else {
				builder.WriteString("\t")
			}
			builder.WriteString(fmt.Sprintf("%q", item.Path))
			builder.WriteString("\n")
		}
		builder.WriteString(")\n\n")
	}

	for i, block := range response.Blocks {
		builder.WriteString(strings.TrimSpace(block.Code))
		builder.WriteString("\n")
		if i != len(response.Blocks)-1 {
			builder.WriteString("\n")
		}
	}

	source := builder.String()
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return []byte(source), nil
	}

	return formatted, nil
}

func (renderer Golang) FacadeImports(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string) []sdk.ImportRef {
	return renderer.facadeImports(file, model, modelImportPath, modelPackageName, nil)
}

func (renderer Golang) FacadeImportsWithFiles(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []sdk.GeneratedFile) []sdk.ImportRef {
	return renderer.facadeImports(file, model, modelImportPath, modelPackageName, generatedFiles)
}

func (renderer Golang) facadeImports(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []sdk.GeneratedFile) []sdk.ImportRef {
	if model == nil || strings.TrimSpace(modelImportPath) == "" || strings.TrimSpace(modelPackageName) == "" {
		return nil
	}

	modelAlias := facadeModelImportAlias(file, modelPackageName)
	imports := []sdk.ImportRef{{
		Alias: modelAlias,
		Path:  modelImportPath,
	}}

	if facadeUsesValidation(model) {
		imports = append(imports, sdk.ImportRef{
			Alias: "validationpkg",
			Path:  joinImportPath(modelImportPath, "validation"),
		})
	}

	seen := make(map[string]sdk.ImportRef)
	for _, item := range imports {
		key := strings.TrimSpace(item.Alias) + "|" + strings.TrimSpace(item.Path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = item
	}

	items := make([]sdk.ImportRef, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Alias < items[j].Alias
		}
		return items[i].Path < items[j].Path
	})
	return items
}

// facadeModelImportAlias picks an import alias for the model subpackage that
// cannot collide with user imports or generated helper package aliases.
func facadeModelImportAlias(file *ast.File, modelPackageName string) string {
	baseAlias := strings.TrimSpace(modelPackageName)
	if baseAlias == "" {
		baseAlias = "modelpkg"
	}

	used := map[string]struct{}{
		"grpcpkg":       {},
		"validationpkg": {},
	}
	for _, item := range fileImports(file) {
		alias := strings.TrimSpace(item.Alias)
		if alias == "" {
			parts := strings.Split(strings.TrimSpace(item.Path), "/")
			if len(parts) > 0 {
				alias = parts[len(parts)-1]
			}
		}
		if alias == "" || alias == "_" || alias == "." {
			continue
		}
		used[alias] = struct{}{}
	}

	if _, exists := used[baseAlias]; !exists {
		return baseAlias
	}

	for _, candidate := range []string{"modelpkg", baseAlias + "pkg", baseAlias + "model"} {
		if candidate == "" {
			continue
		}
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}

	for index := 2; ; index++ {
		candidate := fmt.Sprintf("modelpkg%d", index)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func (renderer Golang) FacadeCode(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string) string {
	return renderer.facadeCode(file, model, modelImportPath, modelPackageName, nil)
}

func (renderer Golang) FacadeCodeWithFiles(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []sdk.GeneratedFile) string {
	return renderer.facadeCode(file, model, modelImportPath, modelPackageName, generatedFiles)
}

// facadeCode builds the root-facing API that wraps the per-model subpackage and
// stitches in optional validation and gRPC helpers when those runtimes are active.
func (renderer Golang) facadeCode(file *ast.File, model *ast.ModelAST, modelImportPath string, modelPackageName string, generatedFiles []sdk.GeneratedFile) string {
	if model == nil {
		return ""
	}
	if strings.TrimSpace(modelImportPath) == "" || strings.TrimSpace(modelPackageName) == "" {
		return ""
	}

	importAlias := facadeModelImportAlias(file, modelPackageName)
	var builder strings.Builder

	builder.WriteString(sdk.DocComment(
		"%s оборачивает модель %s из подпакета и сохраняет удобный корневой API.",
		"%s wraps model %s from the subpackage and keeps the root API convenient.",
		model.Name,
		model.Name,
	))
	builder.WriteString("\n")
	builder.WriteString("type ")
	builder.WriteString(model.Name)
	builder.WriteString(" struct {\n\t")
	builder.WriteString(importAlias)
	builder.WriteString(".")
	builder.WriteString(model.Name)
	builder.WriteString("\n}")

	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func Wrap%s(value %s.%s) %s {\n\treturn %s{%s: value}\n}", model.Name, importAlias, model.Name, model.Name, model.Name, model.Name),
		"Wrap%s поднимает модель %s из подпакета в корневой facade-тип.",
		"Wrap%s lifts model %s from the subpackage into the root facade type.",
		model.Name,
		model.Name,
	))

	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func (model %s) Underlying() %s.%s {\n\treturn model.%s\n}", model.Name, importAlias, model.Name, model.Name),
		"Underlying возвращает исходную модель %s из подпакета.",
		"Underlying returns the original %s model from the subpackage.",
		model.Name,
	))

	builder.WriteString("\n\n")
	if modelHasDefaults(model) {
		builder.WriteString(sdk.WithDocComment(
			fmt.Sprintf("func Default%s() %s {\n\treturn Wrap%s(%s.Default%s())\n}", model.Name, model.Name, model.Name, importAlias, model.Name),
			"Default%s возвращает модель %s со значениями по умолчанию из подпакета модели.",
			"Default%s returns model %s with default values from the model subpackage.",
			model.Name,
			model.Name,
		))
	} else {
		builder.WriteString(sdk.WithDocComment(
			fmt.Sprintf("func Default%s() %s {\n\treturn Wrap%s(%s.%s{})\n}", model.Name, model.Name, model.Name, importAlias, model.Name),
			"Default%s возвращает zero-value модель %s, когда дополнительные значения по умолчанию не заданы.",
			"Default%s returns the zero-value %s model when no additional defaults are defined.",
			model.Name,
			model.Name,
		))
	}

	if facadeUsesValidation(model) {
		builder.WriteString("\n\n")
		builder.WriteString(sdk.WithDocComment(
			fmt.Sprintf("func (model %s) ValidationErrors() []error {\n\treturn validationpkg.Errors(model.%s)\n}", model.Name, model.Name),
			"ValidationErrors возвращает список ошибок валидации модели %s через validation-подпакет.",
			"ValidationErrors returns the list of validation errors for model %s through the validation subpackage.",
			model.Name,
		))
		builder.WriteString("\n\n")
		builder.WriteString(sdk.WithDocComment(
			fmt.Sprintf("func (model %s) Validate() error {\n\treturn validationpkg.Validate(model.%s)\n}", model.Name, model.Name),
			"Validate проверяет модель %s через validation-подпакет.",
			"Validate validates model %s through the validation subpackage.",
			model.Name,
		))
	}

	return builder.String()
}

type facadeGRPCMethod struct {
	Field  *ast.FieldAST
	Method ast.FieldMethodAST
}

type generatedFacadeGRPCParam struct {
	Name string
	Type goast.Expr
}

type generatedFacadeGRPCMethod struct {
	Name    string
	Params  []generatedFacadeGRPCParam
	Results []goast.Expr
	Imports []sdk.ImportRef
}

func joinImportPath(base string, child string) string {
	base = strings.TrimSpace(base)
	child = strings.TrimSpace(child)
	if base == "" {
		return child
	}
	if child == "" {
		return base
	}
	if strings.HasPrefix(base, "./") || strings.HasPrefix(base, "../") {
		return strings.TrimRight(base, "/") + "/" + child
	}
	return path.Join(base, child)
}

func facadeUsesAttr(model *ast.ModelAST, name string) bool {
	if model == nil {
		return false
	}
	for _, attr := range model.Attrs {
		if attr.Matches(name) {
			return true
		}
	}
	return false
}

func facadeUsesValidation(model *ast.ModelAST) bool {
	if model == nil || strings.TrimSpace(model.GeneratedFrom) != "" {
		return false
	}
	if facadeUsesAttr(model, "validate") {
		return true
	}
	for _, field := range model.Fields {
		if field == nil {
			continue
		}
		for _, attr := range field.Attrs {
			if attr.Matches("validate") {
				return true
			}
		}
	}
	return false
}

func facadeUsesGRPC(model *ast.ModelAST) bool {
	return facadeUsesAttr(model, "grpc")
}

func facadeGRPCMethods(model *ast.ModelAST) []facadeGRPCMethod {
	if model == nil {
		return nil
	}

	items := make([]facadeGRPCMethod, 0)
	for _, field := range model.Fields {
		if field == nil {
			continue
		}
		for _, method := range field.Methods {
			items = append(items, facadeGRPCMethod{
				Field:  field,
				Method: method,
			})
		}
	}
	return items
}

func facadeGeneratedGRPCMethods(model *ast.ModelAST, generatedFiles []sdk.GeneratedFile) []generatedFacadeGRPCMethod {
	if model == nil || len(generatedFiles) == 0 {
		return nil
	}

	body, ok := generatedFacadeClientFile(generatedFiles)
	if !ok {
		return nil
	}

	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, "grpc/client.gen.go", body, 0)
	if err != nil {
		return nil
	}

	imports := parsedImportIndex(file)
	methodIndex := explicitFacadeGRPCMethodIndex(model)
	items := make([]generatedFacadeGRPCMethod, 0)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*goast.GenDecl)
		if !ok || genDecl.Tok != gotoken.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*goast.TypeSpec)
			if !ok || typeSpec.Name == nil || typeSpec.Name.Name != model.Name+"GRPCClient" {
				continue
			}
			iface, ok := typeSpec.Type.(*goast.InterfaceType)
			if !ok || iface.Methods == nil {
				return items
			}
			for _, method := range iface.Methods.List {
				if len(method.Names) != 1 {
					continue
				}
				name := strings.TrimSpace(method.Names[0].Name)
				if name == "" {
					continue
				}
				if _, skip := methodIndex[name]; skip {
					continue
				}
				funcType, ok := method.Type.(*goast.FuncType)
				if !ok {
					continue
				}
				items = append(items, generatedFacadeGRPCMethod{
					Name:    name,
					Params:  parsedMethodParams(funcType),
					Results: parsedMethodResults(funcType),
					Imports: collectedMethodImports(funcType, imports),
				})
			}
			return items
		}
	}

	return items
}

func generatedFacadeClientFile(generatedFiles []sdk.GeneratedFile) (string, bool) {
	for _, item := range generatedFiles {
		path := strings.TrimSpace(filepathToSlash(item.Path))
		if path == "grpc/client.gen.go" {
			return item.Content, true
		}
	}
	return "", false
}

func generatedFacadeServerFile(generatedFiles []sdk.GeneratedFile) (string, bool) {
	for _, item := range generatedFiles {
		path := strings.TrimSpace(filepathToSlash(item.Path))
		if path == "grpc/server.gen.go" {
			return item.Content, true
		}
	}
	return "", false
}

func facadeHasGRPCInsideHandler(model *ast.ModelAST, generatedFiles []sdk.GeneratedFile) bool {
	if model == nil {
		return false
	}
	if body, ok := generatedFacadeServerFile(generatedFiles); ok {
		return strings.Contains(body, "type "+model.Name+"GRPCInsideHandler interface")
	}
	return len(facadeGRPCMethods(model)) > 0
}

func filepathToSlash(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
}

func parsedImportIndex(file *goast.File) map[string]sdk.ImportRef {
	items := make(map[string]sdk.ImportRef)
	if file == nil {
		return items
	}

	for _, item := range file.Imports {
		if item == nil || item.Path == nil {
			continue
		}
		path := strings.Trim(item.Path.Value, `"`)
		if path == "" {
			continue
		}
		lookupAlias := ""
		importAlias := ""
		if item.Name != nil {
			lookupAlias = strings.TrimSpace(item.Name.Name)
			importAlias = lookupAlias
		}
		if lookupAlias == "" {
			parts := strings.Split(path, "/")
			lookupAlias = parts[len(parts)-1]
			importAlias = ""
		}
		if lookupAlias == "." || lookupAlias == "_" || lookupAlias == "" {
			continue
		}
		items[lookupAlias] = sdk.ImportRef{Alias: importAlias, Path: path}
	}

	return items
}

func explicitFacadeGRPCMethodIndex(model *ast.ModelAST) map[string]struct{} {
	items := map[string]struct{}{"Apply": {}}
	for _, method := range facadeGRPCMethods(model) {
		items[facadeGRPCBridgeName(method.Field, method.Method)] = struct{}{}
	}
	return items
}

func parsedMethodParams(funcType *goast.FuncType) []generatedFacadeGRPCParam {
	if funcType == nil || funcType.Params == nil {
		return nil
	}

	items := make([]generatedFacadeGRPCParam, 0)
	index := 1
	for _, field := range funcType.Params.List {
		if field == nil || field.Type == nil {
			continue
		}
		if len(field.Names) == 0 {
			items = append(items, generatedFacadeGRPCParam{
				Name: fmt.Sprintf("param%d", index),
				Type: field.Type,
			})
			index++
			continue
		}
		for _, name := range field.Names {
			paramName := fmt.Sprintf("param%d", index)
			if name != nil && strings.TrimSpace(name.Name) != "" {
				paramName = strings.TrimSpace(name.Name)
			}
			items = append(items, generatedFacadeGRPCParam{
				Name: paramName,
				Type: field.Type,
			})
			index++
		}
	}

	return items
}

func parsedMethodResults(funcType *goast.FuncType) []goast.Expr {
	if funcType == nil || funcType.Results == nil {
		return nil
	}

	items := make([]goast.Expr, 0)
	for _, field := range funcType.Results.List {
		if field == nil || field.Type == nil {
			continue
		}
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			items = append(items, field.Type)
		}
	}
	return items
}

func collectedMethodImports(funcType *goast.FuncType, importIndex map[string]sdk.ImportRef) []sdk.ImportRef {
	seen := make(map[string]sdk.ImportRef)
	collectExprImports := func(expr goast.Expr) {}
	collectExprImports = func(expr goast.Expr) {
		goast.Inspect(expr, func(node goast.Node) bool {
			selector, ok := node.(*goast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*goast.Ident)
			if !ok {
				return true
			}
			item, ok := importIndex[ident.Name]
			if !ok {
				return true
			}
			key := item.Alias + "|" + item.Path
			seen[key] = item
			return true
		})
	}

	if funcType != nil && funcType.Params != nil {
		for _, field := range funcType.Params.List {
			if field != nil {
				collectExprImports(field.Type)
			}
		}
	}
	if funcType != nil && funcType.Results != nil {
		for _, field := range funcType.Results.List {
			if field != nil {
				collectExprImports(field.Type)
			}
		}
	}

	items := make([]sdk.ImportRef, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Alias < items[j].Alias
		}
		return items[i].Path < items[j].Path
	})
	return items
}

func facadeGeneratedMethodImports(model *ast.ModelAST, generatedFiles []sdk.GeneratedFile) []sdk.ImportRef {
	methods := facadeGeneratedGRPCMethods(model, generatedFiles)
	seen := make(map[string]sdk.ImportRef)
	for _, method := range methods {
		for _, item := range method.Imports {
			key := item.Alias + "|" + item.Path
			seen[key] = item
		}
	}

	items := make([]sdk.ImportRef, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Alias < items[j].Alias
		}
		return items[i].Path < items[j].Path
	})
	return items
}

func facadeMethodImports(file *ast.File, model *ast.ModelAST) []sdk.ImportRef {
	if file == nil || model == nil {
		return nil
	}

	index := make(map[string]sdk.ImportRef)
	for _, item := range fileImports(file) {
		index[item.Alias] = item
	}

	seen := make(map[string]sdk.ImportRef)
	for _, method := range facadeGRPCMethods(model) {
		for _, param := range method.Method.Params {
			addFacadeTypeImports(seen, index, resolveFacadeTypeRef(file, model, param.Type))
		}
		for _, result := range method.Method.Returns {
			addFacadeTypeImports(seen, index, resolveFacadeTypeRef(file, model, result))
		}
	}

	items := make([]sdk.ImportRef, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Alias < items[j].Alias
		}
		return items[i].Path < items[j].Path
	})
	return items
}

func addFacadeTypeImports(seen map[string]sdk.ImportRef, index map[string]sdk.ImportRef, typeRef ast.TypeRef) {
	if len(typeRef.Name.Parts) < 2 {
		return
	}
	item, ok := index[typeRef.Name.Parts[0]]
	if !ok {
		return
	}
	key := item.Alias + "|" + item.Path
	seen[key] = item
}

func resolveModelFieldTypeRef(file *ast.File, typeRef ast.TypeRef) ast.TypeRef {
	return resolveModelFieldTypeRefSeen(file, typeRef, make(map[string]struct{}))
}

// resolveModelFieldTypeRefSeen expands `Model.Field` references until the
// generator reaches a concrete type, while preserving outer list/optional flags.
func resolveModelFieldTypeRefSeen(file *ast.File, typeRef ast.TypeRef, seen map[string]struct{}) ast.TypeRef {
	if file == nil || len(typeRef.Name.Parts) != 2 {
		return typeRef
	}

	modelName := strings.TrimSpace(typeRef.Name.Parts[0])
	fieldName := strings.TrimSpace(typeRef.Name.Parts[1])
	if modelName == "" || fieldName == "" {
		return typeRef
	}
	key := modelName + "." + fieldName
	if _, exists := seen[key]; exists {
		return typeRef
	}
	seen[key] = struct{}{}

	referencedModel, ok := file.FindModel(modelName)
	if !ok || referencedModel == nil {
		return typeRef
	}
	for _, field := range referencedModel.Fields {
		if field == nil || strings.TrimSpace(field.Name) != fieldName {
			continue
		}
		resolved := field.Type
		if len(resolved.Name.Parts) == 2 {
			resolved = resolveModelFieldTypeRefSeen(file, resolved, seen)
		}
		resolved.IsList = resolved.IsList || typeRef.IsList
		resolved.Optional = resolved.Optional || typeRef.Optional
		return resolved
	}
	return typeRef
}

func resolveFacadeTypeRef(file *ast.File, currentModel *ast.ModelAST, typeRef ast.TypeRef) ast.TypeRef {
	_ = currentModel
	return resolveModelFieldTypeRef(file, typeRef)
}

func facadeGoType(file *ast.File, currentModel *ast.ModelAST, typeRef ast.TypeRef) string {
	typeRef = resolveFacadeTypeRef(file, currentModel, typeRef)
	name := typeRef.Name.String()
	if name == "" {
		name = "any"
	}
	if len(typeRef.Name.Parts) == 1 {
		if _, ok := file.FindModel(typeRef.BaseName()); ok {
			name = typeRef.BaseName()
		}
	}
	if typeRef.IsList {
		return "[]" + name
	}
	if typeRef.Optional {
		return "*" + name
	}
	return name
}

func facadeZeroValue(file *ast.File, currentModel *ast.ModelAST, typeRef ast.TypeRef) string {
	typeRef = resolveFacadeTypeRef(file, currentModel, typeRef)
	switch {
	case typeRef.Optional, typeRef.IsList:
		return "nil"
	case typeRef.Name.String() == "[]byte":
		return "nil"
	case typeRef.Name.String() == "error":
		return "nil"
	case typeRef.Name.String() == "string":
		return `""`
	case typeRef.Name.String() == "bool":
		return "false"
	case typeRef.Name.String() == "float32", typeRef.Name.String() == "float64":
		return "0"
	case strings.HasPrefix(typeRef.Name.String(), "int"), strings.HasPrefix(typeRef.Name.String(), "uint"), typeRef.Name.String() == "byte":
		return "0"
	case typeRef.Name.String() == "time.Time":
		return "time.Time{}"
	default:
		if _, ok := file.FindModel(typeRef.BaseName()); ok {
			return typeRef.BaseName() + "{}"
		}
		return typeRef.BaseName() + "{}"
	}
}

func facadeWrapResult(file *ast.File, currentModel *ast.ModelAST, typeRef ast.TypeRef, source string) string {
	typeRef = resolveFacadeTypeRef(file, currentModel, typeRef)
	if _, ok := file.FindModel(typeRef.BaseName()); ok {
		if typeRef.Optional {
			return source
		}
		return "Wrap" + typeRef.BaseName() + "(" + source + ")"
	}
	return source
}

func facadeUnwrapArg(file *ast.File, currentModel *ast.ModelAST, typeRef ast.TypeRef, source string) string {
	typeRef = resolveFacadeTypeRef(file, currentModel, typeRef)
	if _, ok := file.FindModel(typeRef.BaseName()); ok {
		if typeRef.Optional {
			return source
		}
		return source + "." + typeRef.BaseName()
	}
	return source
}

func facadeGRPCBridgeName(field *ast.FieldAST, method ast.FieldMethodAST) string {
	if field == nil {
		return method.Name
	}
	if method.Name == field.Name && len(method.Params) == 0 {
		return "Get" + field.Name
	}
	return field.Name + method.Name
}

func facadeClientReturnTypes(file *ast.File, model *ast.ModelAST, method ast.FieldMethodAST) []ast.TypeRef {
	items := make([]ast.TypeRef, 0, len(method.Returns)+1)
	items = append(items, method.Returns...)
	if len(method.Returns) == 0 || method.Returns[len(method.Returns)-1].Name.String() != "error" {
		items = append(items, ast.TypeRef{Name: ast.Name{Parts: []string{"error"}}})
	}
	return items
}

func facadeGRPCClientCode(file *ast.File, model *ast.ModelAST, extraMethods []generatedFacadeGRPCMethod) string {
	methods := facadeGRPCMethods(model)
	clientName := model.Name + "GRPCClient"
	implName := sdk.LowerCamel(model.Name) + "GRPCRootClient"

	interfaceLines := []string{
		fmt.Sprintf("\tApply(ctx context.Context, model %s) (%s, error)", model.Name, model.Name),
	}
	for _, item := range methods {
		params := make([]string, 0, len(item.Method.Params)+1)
		params = append(params, "ctx context.Context")
		for index, param := range item.Method.Params {
			name := strings.TrimSpace(param.Name)
			if name == "" {
				name = fmt.Sprintf("param%d", index+1)
			}
			params = append(params, name+" "+facadeGoType(file, model, param.Type))
		}

		returns := make([]string, 0, len(facadeClientReturnTypes(file, model, item.Method)))
		for _, result := range facadeClientReturnTypes(file, model, item.Method) {
			returns = append(returns, facadeGoType(file, model, result))
		}
		signature := returns[0]
		if len(returns) > 1 {
			signature = "(" + strings.Join(returns, ", ") + ")"
		}
		interfaceLines = append(interfaceLines, fmt.Sprintf("\t%s(%s) %s", facadeGRPCBridgeName(item.Field, item.Method), strings.Join(params, ", "), signature))
	}
	for _, item := range extraMethods {
		interfaceLines = append(interfaceLines, "\t"+generatedFacadeMethodSignature(item))
	}

	parts := []string{
		fmt.Sprintf("type %s interface {\n%s\n}", clientName, strings.Join(interfaceLines, "\n")),
		fmt.Sprintf("type %s struct {\n\tclient grpcpkg.%sGRPCClient\n}", implName, model.Name),
		sdk.WithDocComment(
			fmt.Sprintf("func New%s(conn grpc.ClientConnInterface) %s {\n\treturn Wrap%s(grpcpkg.New%s(conn))\n}", clientName, clientName, clientName, clientName),
			"New%s создает корневой gRPC-клиент модели %s с backward-compatible API.",
			"New%s creates the root gRPC client for model %s with a backward-compatible API.",
			clientName,
			model.Name,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func Wrap%s(client grpcpkg.%sGRPCClient) %s {\n\treturn &%s{client: client}\n}", clientName, model.Name, clientName, implName),
			"Wrap%s оборачивает grpc-подпакетный клиент модели %s в корневой facade API.",
			"Wrap%s wraps the grpc subpackage client for model %s into the root facade API.",
			clientName,
			model.Name,
		),
		fmt.Sprintf("var _ %s = (*%s)(nil)", clientName, implName),
		sdk.WithDocComment(
			fmt.Sprintf("func (client *%s) Apply(ctx context.Context, model %s) (%s, error) {\n\tif client == nil || client.client == nil {\n\t\treturn %s{}, fmt.Errorf(%q)\n\t}\n\tvalue, err := client.client.Apply(ctx, model.%s)\n\tif err != nil {\n\t\treturn %s{}, err\n\t}\n\treturn Wrap%s(value), nil\n}", implName, model.Name, model.Name, model.Name, "grpc client is nil", model.Name, model.Name, model.Name),
			"Apply отправляет модель %s через корневой gRPC facade и возвращает ее же в корневом типе.",
			"Apply sends model %s through the root gRPC facade and returns it in the root type.",
			model.Name,
		),
	}

	for _, item := range methods {
		parts = append(parts, facadeGRPCClientMethodCode(file, model, implName, item))
	}
	for _, item := range extraMethods {
		parts = append(parts, generatedFacadeGRPCClientMethodCode(model, implName, item))
	}

	return strings.Join(parts, "\n\n")
}

func facadeGRPCClientMethodCode(file *ast.File, model *ast.ModelAST, clientImplName string, item facadeGRPCMethod) string {
	bridge := facadeGRPCBridgeName(item.Field, item.Method)
	params := make([]string, 0, len(item.Method.Params))
	callArgs := make([]string, 0, len(item.Method.Params))
	for index, param := range item.Method.Params {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			name = fmt.Sprintf("param%d", index+1)
		}
		params = append(params, name+" "+facadeGoType(file, model, param.Type))
		callArgs = append(callArgs, facadeUnwrapArg(file, model, param.Type, name))
	}

	returnTypes := facadeClientReturnTypes(file, model, item.Method)
	returnTypeNames := make([]string, 0, len(returnTypes))
	for _, result := range returnTypes {
		returnTypeNames = append(returnTypeNames, facadeGoType(file, model, result))
	}
	signature := returnTypeNames[0]
	if len(returnTypeNames) > 1 {
		signature = "(" + strings.Join(returnTypeNames, ", ") + ")"
	}

	nonErrorReturns := make([]ast.TypeRef, 0, len(returnTypes))
	for _, result := range returnTypes {
		if result.Name.String() == "error" {
			continue
		}
		nonErrorReturns = append(nonErrorReturns, result)
	}

	call := "client.client." + bridge + "(ctx"
	if len(callArgs) > 0 {
		call += ", " + strings.Join(callArgs, ", ")
	}
	call += ")"

	body := []string{
		"if client == nil || client.client == nil {",
		"\t\treturn " + facadeZeroReturnTuple(file, model, returnTypes, true),
		"\t}",
	}

	if len(nonErrorReturns) == 0 {
		body = append(body, "return "+call)
	} else {
		resultNames := make([]string, 0, len(nonErrorReturns))
		for i := range nonErrorReturns {
			resultNames = append(resultNames, fmt.Sprintf("result%d", i+1))
		}
		body = append(body, strings.Join(append(resultNames, "err"), ", ")+" := "+call)
		body = append(body, "if err != nil {\n\t\treturn "+facadeZeroReturnTuple(file, model, returnTypes, false)+"\n\t}")

		converted := make([]string, 0, len(nonErrorReturns))
		for i, result := range nonErrorReturns {
			converted = append(converted, facadeWrapResult(file, model, result, resultNames[i]))
		}
		converted = append(converted, "nil")
		body = append(body, "return "+strings.Join(converted, ", "))
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) %s(ctx context.Context%s) %s {\n\t%s\n}", clientImplName, bridge, facadeParamSignature(file, model, item.Method.Params), signature, strings.Join(body, "\n\t")),
		"%s вызывает gRPC метод %s через корневой facade-клиент модели %s.",
		"%s calls the gRPC method %s through the root facade client for model %s.",
		bridge,
		bridge,
		model.Name,
	)
}

func facadeParamSignature(file *ast.File, model *ast.ModelAST, params []ast.FieldMethodParamAST) string {
	if len(params) == 0 {
		return ""
	}
	items := make([]string, 0, len(params))
	for index, param := range params {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			name = fmt.Sprintf("param%d", index+1)
		}
		items = append(items, name+" "+facadeGoType(file, model, param.Type))
	}
	return ", " + strings.Join(items, ", ")
}

func facadeZeroReturnTuple(file *ast.File, model *ast.ModelAST, returns []ast.TypeRef, nilClient bool) string {
	items := make([]string, 0, len(returns))
	for _, result := range returns {
		if result.Name.String() == "error" {
			if nilClient {
				items = append(items, `fmt.Errorf("grpc client is nil")`)
			} else {
				items = append(items, "err")
			}
			continue
		}
		items = append(items, facadeZeroValue(file, model, result))
	}
	if len(items) == 0 {
		if nilClient {
			return `fmt.Errorf("grpc client is nil")`
		}
		return "err"
	}
	return strings.Join(items, ", ")
}

func generatedFacadeMethodSignature(method generatedFacadeGRPCMethod) string {
	params := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		params = append(params, param.Name+" "+renderGoExpr(param.Type))
	}

	results := generatedFacadeResultTypes(method)
	switch len(results) {
	case 0:
		return method.Name + "(" + strings.Join(params, ", ") + ")"
	case 1:
		return method.Name + "(" + strings.Join(params, ", ") + ") " + results[0]
	default:
		return method.Name + "(" + strings.Join(params, ", ") + ") (" + strings.Join(results, ", ") + ")"
	}
}

func generatedFacadeResultTypes(method generatedFacadeGRPCMethod) []string {
	items := make([]string, 0, len(method.Results))
	for _, result := range method.Results {
		items = append(items, renderGoExpr(result))
	}
	return items
}

func generatedFacadeGRPCClientMethodCode(model *ast.ModelAST, clientImplName string, method generatedFacadeGRPCMethod) string {
	paramNames := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		paramNames = append(paramNames, param.Name)
	}

	signature := generatedFacadeMethodSignature(method)
	body := []string{
		"if client == nil || client.client == nil {",
		"\t\treturn " + generatedFacadeZeroReturnTuple(method, true),
		"\t}",
	}
	call := "client.client." + method.Name + "(" + strings.Join(paramNames, ", ") + ")"
	if len(method.Results) == 0 {
		body = append(body, call)
	} else {
		body = append(body, "return "+call)
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) %s {\n\t%s\n}", clientImplName, signature, strings.Join(body, "\n\t")),
		"%s вызывает gRPC метод %s через корневой facade-клиент модели %s.",
		"%s calls the gRPC method %s through the root facade client for model %s.",
		method.Name,
		method.Name,
		model.Name,
	)
}

func generatedFacadeZeroReturnTuple(method generatedFacadeGRPCMethod, nilClient bool) string {
	if len(method.Results) == 0 {
		return ""
	}

	items := make([]string, 0, len(method.Results))
	for _, result := range method.Results {
		if isErrorExpr(result) {
			if nilClient {
				items = append(items, `fmt.Errorf("grpc client is nil")`)
			} else {
				items = append(items, "err")
			}
			continue
		}
		items = append(items, generatedFacadeZeroValue(result))
	}
	return strings.Join(items, ", ")
}

func generatedFacadeZeroValue(expr goast.Expr) string {
	switch typed := expr.(type) {
	case *goast.StarExpr, *goast.InterfaceType, *goast.MapType, *goast.FuncType, *goast.ChanType:
		return "nil"
	case *goast.ArrayType:
		if typed.Len == nil {
			return "nil"
		}
		return renderGoExpr(expr) + "{}"
	case *goast.Ident:
		switch typed.Name {
		case "string":
			return `""`
		case "bool":
			return "false"
		case "error", "any":
			return "nil"
		case "float32", "float64":
			return "0"
		}
		if strings.HasPrefix(typed.Name, "int") || strings.HasPrefix(typed.Name, "uint") || typed.Name == "byte" || typed.Name == "rune" {
			return "0"
		}
		return typed.Name + "{}"
	default:
		return renderGoExpr(expr) + "{}"
	}
}

func isErrorExpr(expr goast.Expr) bool {
	ident, ok := expr.(*goast.Ident)
	return ok && ident.Name == "error"
}

func renderGoExpr(expr goast.Expr) string {
	if expr == nil {
		return "any"
	}

	var builder bytes.Buffer
	if err := goprinter.Fprint(&builder, gotoken.NewFileSet(), expr); err != nil {
		return "any"
	}
	return builder.String()
}

func fileImports(file *ast.File) []sdk.ImportRef {
	if file == nil {
		return nil
	}

	imports := make([]sdk.ImportRef, 0)
	seen := make(map[string]struct{})
	for _, importNode := range file.Imports() {
		if importNode == nil {
			continue
		}

		for i := range importNode.Specs {
			spec := importNode.Specs[i]
			key := spec.Alias + "|" + spec.Path
			if _, exists := seen[key]; exists {
				continue
			}

			seen[key] = struct{}{}
			imports = append(imports, sdk.ImportRef{
				Alias: spec.Alias,
				Path:  spec.Path,
			})
		}
	}

	sort.Slice(imports, func(i int, j int) bool {
		if imports[i].Path == imports[j].Path {
			return imports[i].Alias < imports[j].Alias
		}

		return imports[i].Path < imports[j].Path
	})

	return imports
}

func goType(file *ast.File, currentModel *ast.ModelAST, typeRef ast.TypeRef) string {
	typeRef = resolveModelFieldTypeRef(file, typeRef)
	name := typeRef.Name.String()
	if name == "" {
		name = "any"
	}
	if referencedModelAlias(file, currentModel, typeRef) != "" {
		name = referencedModelAlias(file, currentModel, typeRef) + "." + strings.TrimSpace(typeRef.Name.String())
	}

	if typeRef.IsList {
		name = "[]" + name
		return name
	}

	if typeRef.Optional {
		return "*" + name
	}

	return name
}

func referencedModelAlias(file *ast.File, currentModel *ast.ModelAST, typeRef ast.TypeRef) string {
	if file == nil || currentModel == nil {
		return ""
	}
	if len(typeRef.Name.Parts) != 1 {
		return ""
	}

	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" || name == currentModel.Name {
		return ""
	}
	if _, ok := file.FindModel(name); !ok {
		return ""
	}
	return Golang{}.ModelPackageName(name)
}

func modelHasDefaults(model *ast.ModelAST) bool {
	if model == nil {
		return false
	}

	for _, field := range model.Fields {
		if _, ok := goDefaultValueLiteral(field); ok {
			return true
		}
	}

	return false
}

func modelDefaultsCode(model *ast.ModelAST) string {
	if model == nil {
		return ""
	}

	assignments := make([]string, 0)
	for _, field := range model.Fields {
		if field == nil || field.Default == nil {
			continue
		}

		value, ok := goDefaultValueLiteral(field)
		if !ok {
			continue
		}
		assignments = append(assignments, fmt.Sprintf("%s: %s,", field.Name, value))
	}

	if len(assignments) == 0 {
		return ""
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func Default%s() %s {\n\treturn %s{\n\t\t%s\n\t}\n}", model.Name, model.Name, model.Name, strings.Join(assignments, "\n\t\t")),
		"Default%s создает %s со значениями полей по умолчанию.",
		"Default%s creates %s with its generated default field values.",
		model.Name,
		model.Name,
	)
}

func modelDocComment(model *ast.ModelAST) string {
	if model == nil {
		return ""
	}

	for _, attr := range model.Attrs {
		if strings.TrimSpace(attr.Identifier()) != "comment" || len(attr.Args) == 0 {
			continue
		}
		if value, ok := attr.Args[0].(ast.StringExpr); ok {
			return strings.TrimSpace(value.Value)
		}
		return strings.TrimSpace(ast.ExprString(attr.Args[0]))
	}

	return ""
}

func fieldDocComment(field *ast.FieldAST) string {
	if field == nil {
		return ""
	}

	for _, attr := range field.Attrs {
		if strings.TrimSpace(attr.Identifier()) != "comment" || len(attr.Args) == 0 {
			continue
		}
		if value, ok := attr.Args[0].(ast.StringExpr); ok {
			return strings.TrimSpace(value.Value)
		}
		return strings.TrimSpace(ast.ExprString(attr.Args[0]))
	}

	return ""
}

func goDefaultValueLiteral(field *ast.FieldAST) (string, bool) {
	if field == nil || field.Default == nil {
		return "", false
	}

	renderOptional := func(expr string) (string, bool) {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			return "", false
		}
		if expr == "nil" && field.Type.Optional {
			return "nil", true
		}
		if field.Type.Optional {
			return fmt.Sprintf("func() *%s { value := %s; return &value }()", defaultValueGoType(field.Type), expr), true
		}
		return expr, true
	}

	switch value := field.Default.(type) {
	case ast.StringExpr:
		return renderOptional(strconv.Quote(value.Value))
	case ast.BoolExpr:
		literal := "false"
		if value.Value {
			literal = "true"
		}
		return renderOptional(literal)
	case ast.NumberExpr:
		return renderOptional(value.Value)
	case ast.NameExpr:
		name := strings.TrimSpace(value.Name.String())
		if name == "nil" {
			return renderOptional(name)
		}
		if len(value.Name.Parts) < 2 {
			return "", false
		}
		return renderOptional(name)
	case ast.GoExpr:
		return renderOptional(value.Text)
	}

	return "", false
}

func defaultValueGoType(typeRef ast.TypeRef) string {
	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" {
		name = "any"
	}
	if typeRef.IsList {
		name = "[]" + name
	}
	return name
}

func importAliasesFromExpr(expr ast.Expr, importIndex map[string]sdk.ImportRef) []string {
	if expr == nil || len(importIndex) == 0 {
		return nil
	}

	aliases := make(map[string]struct{})
	addAlias := func(alias string) {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			return
		}
		if _, ok := importIndex[alias]; ok {
			aliases[alias] = struct{}{}
		}
	}

	switch value := expr.(type) {
	case ast.NameExpr:
		if len(value.Name.Parts) >= 2 {
			addAlias(value.Name.Parts[0])
		}
	case ast.GoExpr:
		parsed, err := goparser.ParseExpr(value.Text)
		if err != nil {
			return nil
		}
		goast.Inspect(parsed, func(node goast.Node) bool {
			selector, ok := node.(*goast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*goast.Ident)
			if !ok {
				return true
			}
			addAlias(ident.Name)
			return true
		})
	}

	if len(aliases) == 0 {
		return nil
	}

	items := make([]string, 0, len(aliases))
	for alias := range aliases {
		items = append(items, alias)
	}
	sort.Strings(items)
	return items
}
