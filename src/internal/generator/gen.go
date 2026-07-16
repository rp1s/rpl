package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"rpl/internal/fsutil"
	"rpl/internal/generator/parser/ast"
	tokpkg "rpl/internal/generator/parser/lexer/token"
	targetpkg "rpl/internal/generator/target"
	"rpl/internal/plugins"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"rpl/pkg/sdk"
	"sort"
	"strings"
	"unicode"
)

const (
	defaultOutputDir     = "models"
	defaultTypesDirName  = "types"
	defaultTypesPkgName  = "types"
	defaultTypesFileName = "types.gen.go"
	defaultTypesKey      = "__types__"
	defaultTypesImport   = "typespkg"
	baseModelBlockName   = "model"
	baseModelBlockOrder  = 0
	runtimeBlockOrderMin = 100
	manifestFileName     = ".rpl-generated.json"
)

type Generator struct {
	File              *ast.File
	SourcePath        string
	OutputDirOverride string
	runtimeNamespaces map[string]map[string]struct{}
	runtimeAttrSpecs  map[string][]sdk.AttrSpec
}

type modelLayout struct {
	RootOutputDir   string
	RootPackageName string
	ModelDirName    string
	ModelDir        string
	ModelPackage    string
	ModelFileName   string
	ModelImportPath string
	MainRelative    string
	FacadeFileName  string
	FacadeRelative  string
}

type outputManifest struct {
	Models map[string][]string `json:"models"`
}

func New(file *ast.File, sourcePath string, outputDirOverride ...string) *Generator {
	gen := &Generator{
		File:       file,
		SourcePath: strings.TrimSpace(sourcePath),
	}
	if len(outputDirOverride) > 0 {
		gen.OutputDirOverride = strings.TrimSpace(outputDirOverride[0])
	}

	return gen
}

func (gen *Generator) resolveModelLayout(renderer targetpkg.Renderer, outputRoot string, model *ast.ModelAST) modelLayout {
	relative := targetpkg.ResolveModelLayout(renderer, model.Name)
	rootPackageName := gen.rootPackageName(renderer, outputRoot)
	layout := modelLayout{
		RootOutputDir:   outputRoot,
		RootPackageName: rootPackageName,
		ModelDirName:    relative.ModelDirName,
		ModelDir:        outputRoot,
		ModelPackage:    relative.ModelPackage,
		ModelFileName:   relative.ModelFileName,
		MainRelative:    relative.MainRelative,
		FacadeFileName:  relative.FacadeFileName,
		FacadeRelative:  relative.FacadeRelative,
	}
	if layout.ModelDirName != "" {
		layout.ModelDir = filepath.Join(outputRoot, layout.ModelDirName)
	}

	layout.ModelImportPath = gen.goPackagePath(layout.ModelDir, layout.ModelPackage)
	return layout
}

func (gen *Generator) rootPackageName(renderer targetpkg.Renderer, outputRoot string) string {
	fallback := renderer.PackageName()
	if structured, ok := renderer.(targetpkg.StructuredRenderer); ok {
		fallback = structured.RootPackageName()
	}

	base := strings.TrimSpace(filepath.Base(filepath.Clean(outputRoot)))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return fallback
	}

	name := strings.Trim(strings.TrimSpace(sdk.SnakeCase(base)), "_")
	if name == "" {
		return fallback
	}

	for _, char := range name {
		if unicode.IsDigit(char) {
			return "pkg_" + name
		}
		break
	}

	return name
}

// Run is the main orchestration path: it prepares compiler state, asks each
// runtime for extra artifacts, writes model/facade files, and prunes stale output.
func (gen *Generator) Run() error {
	targetLang, renderer, models, outputDir, err := gen.prepare()
	if err != nil {
		return err
	}
	types := gen.File.Types()
	if len(models) == 0 && len(types) == 0 {
		return nil
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf(localize.Text("создание папки генерации %q: %w", "create generation directory %q: %w"), outputDir, err)
	}
	if err := gen.analyzeAttrs(renderer, targetLang, outputDir); err != nil {
		return err
	}

	manifest, err := gen.loadManifest(outputDir)
	if err != nil {
		return err
	}

	imports := gen.fileImports()
	runtimes := gen.runtimeRefs()
	if err := gen.ensureRuntimeNamespaces(runtimes); err != nil {
		return err
	}
	modelNames := gen.modelNames()
	currentModels := make(map[string]struct{}, len(modelNames))
	for _, name := range modelNames {
		currentModels[name] = struct{}{}
	}
	if len(types) > 0 {
		currentModels[defaultTypesKey] = struct{}{}
		if err := gen.writeTypes(renderer, outputDir, manifest); err != nil {
			return err
		}
	}

	for _, model := range models {
		if model == nil {
			continue
		}

		layout := gen.resolveModelLayout(renderer, outputDir, model)
		if err := os.MkdirAll(layout.ModelDir, 0o755); err != nil {
			return fmt.Errorf(localize.Text("создание папки генерации %q: %w", "create generation directory %q: %w"), layout.ModelDir, err)
		}

		builder := sdk.NewCodeBuilder()
		for _, item := range renderer.UsedImports(gen.File, model) {
			builder.AddImport(item.Path, item.Alias)
		}
		for _, item := range gen.modelImports(renderer, layout, model) {
			builder.AddImport(item.Path, item.Alias)
		}
		builder.AddOrderedBlock(baseModelBlockName, renderer.BaseModelCode(gen.File, model), baseModelBlockOrder)

		if model.GeneratedFrom != "" {
			response := builder.Response()
			body, err := gen.renderModelPackageFile(renderer, layout.ModelPackage, response)
			if err != nil {
				return fmt.Errorf(localize.Text("рендеринг файла для target %q: %w", "render file for target %q: %w"), targetLang, err)
			}

			outputPath := filepath.Join(layout.ModelDir, layout.ModelFileName)
			if err := fsutil.WriteFile(outputPath, body, 0o644); err != nil {
				return fmt.Errorf(localize.Text("запись сгенерированного файла %q: %w", "write generated file %q: %w"), outputPath, err)
			}

			facadeFiles, err := gen.writeFacadeFile(renderer, layout, model, response)
			if err != nil {
				return err
			}
			allFiles := append(prefixGeneratedFiles(layout.ModelDirName, response.Files), facadeFiles...)
			if err := gen.writeExtraFiles(outputDir, allFiles); err != nil {
				return err
			}
			if err := gen.replaceModelFiles(outputDir, manifest, model.Name, layout.MainRelative, allFiles); err != nil {
				return err
			}
			continue
		}

		for _, runtimeRef := range runtimes {
			request, ok := gen.generateRequest(
				model,
				imports,
				runtimes,
				modelNames,
				gen.runtimeModelNames(runtimeRef),
				runtimeRef,
				targetLang,
				layout.ModelDir,
				layout.ModelPackage,
				generatedFileStem(layout.ModelDirName),
			)
			if !ok {
				continue
			}

			if err := plugins.EnsureAvailableAt(gen.SourcePath, runtimeRef.Name, runtimeRef.Author); err != nil {
				return err
			}

			response, err := plugins.GenerateModel(runtimeRef.Name, runtimeRef.Author, request)
			if err != nil {
				return err
			}

			for _, block := range response.Blocks {
				block.Order += runtimeBlockOrderMin
			}
			builder.AddResponse(response)
		}

		response := builder.Response()
		body, err := gen.renderModelPackageFile(renderer, layout.ModelPackage, response)
		if err != nil {
			return fmt.Errorf(localize.Text("рендеринг файла для target %q: %w", "render file for target %q: %w"), targetLang, err)
		}

		outputPath := filepath.Join(layout.ModelDir, layout.ModelFileName)
		if err := fsutil.WriteFile(outputPath, body, 0o644); err != nil {
			return fmt.Errorf(localize.Text("запись сгенерированного файла %q: %w", "write generated file %q: %w"), outputPath, err)
		}

		facadeFiles, err := gen.writeFacadeFile(renderer, layout, model, response)
		if err != nil {
			return err
		}
		allFiles := append(prefixGeneratedFiles(layout.ModelDirName, response.Files), facadeFiles...)
		if err := gen.writeExtraFiles(outputDir, allFiles); err != nil {
			return err
		}
		if err := gen.replaceModelFiles(outputDir, manifest, model.Name, layout.MainRelative, allFiles); err != nil {
			return err
		}
	}

	if err := gen.cleanupStaleOutputs(outputDir, manifest, currentModels); err != nil {
		return err
	}
	if err := gen.saveManifest(outputDir, manifest); err != nil {
		return err
	}

	return nil
}

func (gen *Generator) AnalyzeOnly() error {
	targetLang, renderer, _, outputDir, err := gen.prepare()
	if err != nil {
		return err
	}
	if renderer == nil || targetLang == "" {
		return nil
	}

	if err := gen.ensureRuntimeNamespaces(gen.runtimeRefs()); err != nil {
		return err
	}

	return gen.analyzeAttrs(renderer, targetLang, outputDir)
}

func (gen *Generator) prepare() (string, targetpkg.Renderer, []*ast.ModelAST, string, error) {
	if gen == nil || gen.File == nil {
		return "", nil, nil, "", nil
	}

	targetLang := targetpkg.NormalizeLanguage(gen.File.TargetLang())
	renderer, ok := targetpkg.Lookup(targetLang)
	if !ok {
		return "", nil, nil, "", rplerr.Newf(localize.Text("неподдерживаемый target language %q", "unsupported target language %q"), targetLang).
			WithHint(localize.Text("Сейчас поддерживается `golang`. Пример: `target(lang: golang)`.", "Right now `golang` is supported. Example: `target(lang: golang)`."))
	}

	models := gen.File.Models()
	if len(models) == 0 && len(gen.File.Types()) == 0 {
		return targetLang, renderer, models, "", nil
	}

	outputDir, err := gen.outputDir()
	if err != nil {
		return "", nil, nil, "", err
	}

	return targetLang, renderer, models, outputDir, nil
}

func (gen *Generator) outputDir() (string, error) {
	if strings.TrimSpace(gen.OutputDirOverride) != "" {
		return gen.OutputDirOverride, nil
	}

	if gen.SourcePath == "" {
		return defaultOutputDir, nil
	}

	sourceDir := filepath.Dir(gen.SourcePath)
	if sourceDir == "." || sourceDir == "" {
		return defaultOutputDir, nil
	}

	return filepath.Join(sourceDir, defaultOutputDir), nil
}

func (gen *Generator) fileImports() []sdk.ImportRef {
	imports := make([]sdk.ImportRef, 0)
	seen := make(map[string]struct{})
	for _, importNode := range gen.File.Imports() {
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

func (gen *Generator) modelImports(renderer targetpkg.Renderer, currentLayout modelLayout, model *ast.ModelAST) []sdk.ImportRef {
	if gen == nil || gen.File == nil || renderer == nil || model == nil {
		return nil
	}

	seen := make(map[string]sdk.ImportRef)
	if len(gen.modelTypeAliases(model)) > 0 {
		gen.addTypesImport(seen, currentLayout.ModelDir, currentLayout.RootOutputDir)
	}
	for _, field := range model.Fields {
		if field == nil {
			continue
		}
		gen.addModelImport(seen, renderer, currentLayout, model, field.Type)
		for _, method := range field.Methods {
			for _, param := range method.Params {
				gen.addModelImport(seen, renderer, currentLayout, model, param.Type)
			}
			for _, result := range method.Returns {
				gen.addModelImport(seen, renderer, currentLayout, model, result)
			}
		}
	}
	for _, method := range model.Methods {
		for _, param := range method.Params {
			gen.addModelImport(seen, renderer, currentLayout, model, param.Type)
		}
		for _, result := range method.Returns {
			gen.addModelImport(seen, renderer, currentLayout, model, result)
		}
	}

	imports := make([]sdk.ImportRef, 0, len(seen))
	for _, item := range seen {
		imports = append(imports, item)
	}

	sort.Slice(imports, func(i int, j int) bool {
		if imports[i].Path == imports[j].Path {
			return imports[i].Alias < imports[j].Alias
		}
		return imports[i].Path < imports[j].Path
	})
	return imports
}

func (gen *Generator) addModelImport(seen map[string]sdk.ImportRef, renderer targetpkg.Renderer, currentLayout modelLayout, currentModel *ast.ModelAST, typeRef ast.TypeRef) {
	if gen == nil || gen.File == nil || currentModel == nil {
		return
	}
	typeRef = gen.resolveModelFieldTypeRef(typeRef)
	if len(typeRef.Name.Parts) != 1 {
		return
	}

	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" || name == currentModel.Name {
		return
	}

	referencedModel, ok := gen.File.FindModel(name)
	if !ok || referencedModel == nil {
		return
	}

	layout := gen.resolveModelLayout(renderer, currentLayout.RootOutputDir, referencedModel)
	importPath := strings.TrimSpace(layout.ModelImportPath)
	if importPath == "" {
		relative, err := filepath.Rel(currentLayout.ModelDir, layout.ModelDir)
		if err != nil {
			return
		}
		importPath = filepath.ToSlash(relative)
		if importPath == "." {
			return
		}
		if !strings.HasPrefix(importPath, ".") {
			importPath = "./" + importPath
		}
	}

	item := sdk.ImportRef{
		Alias: layout.ModelPackage,
		Path:  importPath,
	}
	key := item.Path + "|" + item.Alias
	seen[key] = item
}

func (gen *Generator) runtimeRefs() []sdk.RuntimeRef {
	items := make([]sdk.RuntimeRef, 0)
	seen := make(map[string]struct{})
	for _, runtimeBlock := range gen.File.Runtimes() {
		if runtimeBlock == nil {
			continue
		}

		for i := range runtimeBlock.Specs {
			spec := runtimeBlock.Specs[i]
			key := spec.Identifier()
			if _, exists := seen[key]; exists {
				continue
			}

			seen[key] = struct{}{}
			items = append(items, sdk.RuntimeRef{
				Name:   spec.Name,
				Author: spec.Author,
			})
		}
	}

	return items
}

func (gen *Generator) modelNames() []string {
	models := gen.File.Models()
	items := make([]string, 0, len(models))
	seen := make(map[string]struct{})
	for _, model := range models {
		if model == nil {
			continue
		}

		name := strings.TrimSpace(model.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}

		seen[name] = struct{}{}
		items = append(items, name)
	}

	sort.Strings(items)
	return items
}

func (gen *Generator) modelTypeAliases(model *ast.ModelAST) []*ast.TypeAliasAST {
	if gen == nil || gen.File == nil || model == nil {
		return nil
	}

	seen := make(map[string]*ast.TypeAliasAST)
	appendType := func(typeRef ast.TypeRef) {
		resolved := gen.resolveModelFieldTypeRef(typeRef)
		if len(resolved.Name.Parts) != 1 {
			return
		}
		name := strings.TrimSpace(resolved.Name.String())
		if name == "" {
			return
		}
		item, ok := gen.File.FindType(name)
		if !ok || item == nil {
			return
		}
		seen[name] = item
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

	items := make([]*ast.TypeAliasAST, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func (gen *Generator) addTypesImport(seen map[string]sdk.ImportRef, currentDir string, outputRoot string) {
	typeDir := filepath.Join(outputRoot, defaultTypesDirName)
	importPath := gen.goPackagePath(typeDir, defaultTypesPkgName)
	if importPath == "" {
		relative, err := filepath.Rel(currentDir, typeDir)
		if err != nil {
			return
		}
		importPath = filepath.ToSlash(relative)
		if importPath == "." {
			return
		}
		if !strings.HasPrefix(importPath, ".") {
			importPath = "./" + importPath
		}
	}

	item := sdk.ImportRef{
		Alias: defaultTypesImport,
		Path:  importPath,
	}
	seen[item.Path+"|"+item.Alias] = item
}

func (gen *Generator) runtimeModelNames(runtimeRef sdk.RuntimeRef) []string {
	models := gen.File.Models()
	items := make([]string, 0, len(models))
	seen := make(map[string]struct{})
	for _, model := range models {
		if model == nil {
			continue
		}

		schema := gen.modelSchema(model, runtimeRef)
		if !schema.HasRuntimeAttrs() {
			continue
		}

		name := strings.TrimSpace(model.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}

		seen[name] = struct{}{}
		items = append(items, name)
	}

	sort.Strings(items)
	return items
}

// generateRequest assembles the full per-runtime view of the schema so attrs
// can generate code without re-resolving file, model, and import context.
func (gen *Generator) generateRequest(model *ast.ModelAST, imports []sdk.ImportRef, runtimes []sdk.RuntimeRef, modelNames []string, runtimeModelNames []string, runtimeRef sdk.RuntimeRef, targetLang string, outputDir string, packageName string, modelFileStem string) (sdk.GenerateRequest, bool) {
	modelSchema := gen.modelSchema(model, runtimeRef)
	if !modelSchema.HasRuntimeAttrs() {
		return sdk.GenerateRequest{}, false
	}
	specs := gen.runtimeAttrSpecsFor(runtimeRef)
	modelSchema = sdk.NormalizeModelRuntimeAttrs(modelSchema, specs)
	allModels := sdk.NormalizeModelsRuntimeAttrs(gen.allModelSchemas(runtimeRef), specs)

	return sdk.GenerateRequest{
		File: sdk.FileContext{
			SourcePath:    gen.SourcePath,
			ProjectRoot:   gen.projectRoot(outputDir),
			TargetLang:    targetLang,
			OutputDir:     filepath.ToSlash(outputDir),
			PackageName:   strings.TrimSpace(packageName),
			GoPackagePath: gen.goPackagePath(outputDir, packageName),
			ModelFileStem: strings.TrimSpace(modelFileStem),
			Imports:       imports,
			Runtimes:      runtimes,
			Models:        modelNames,
			RuntimeModels: runtimeModelNames,
			AllModels:     allModels,
			Syntax:        gen.syntaxSnapshot(runtimes, imports),
			Resolved:      gen.resolvedSnapshot(targetLang, runtimes, imports),
		},
		Runtime: runtimeRef,
		Model:   modelSchema,
	}, true
}

func (gen *Generator) projectRoot(outputDir string) string {
	if _, root, ok := findGoModule(outputDir); ok {
		return filepath.ToSlash(root)
	}
	if strings.TrimSpace(gen.SourcePath) == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Dir(gen.SourcePath))
}

func (gen *Generator) goPackagePath(outputDir string, packageName string) string {
	modulePath, moduleRoot, ok := findGoModule(outputDir)
	if !ok {
		return ""
	}

	rel, err := filepath.Rel(moduleRoot, outputDir)
	if err != nil {
		return ""
	}

	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return modulePath
	}

	return path.Join(modulePath, rel)
}

func findGoModule(startDir string) (string, string, bool) {
	if strings.TrimSpace(startDir) == "" {
		return "", "", false
	}

	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", false
	}

	for {
		goModPath := filepath.Join(current, "go.mod")
		body, err := os.ReadFile(goModPath)
		if err == nil {
			if modulePath := parseModulePath(string(body)); modulePath != "" {
				return modulePath, current, true
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", "", false
		}
		current = parent
	}
}

func parseModulePath(goMod string) string {
	for _, line := range strings.Split(goMod, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return strings.TrimSpace(strings.Trim(parts[1], `"`))
			}
		}
	}

	return ""
}

func (gen *Generator) allModelSchemas(runtimeRef sdk.RuntimeRef) []sdk.Model {
	models := gen.File.Models()
	items := make([]sdk.Model, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}

		items = append(items, gen.modelSchema(model, runtimeRef))
	}

	return items
}

func (gen *Generator) resolvedSnapshot(targetLang string, runtimes []sdk.RuntimeRef, imports []sdk.ImportRef) *sdk.ResolvedFile {
	if gen == nil || gen.File == nil {
		return nil
	}

	return &sdk.ResolvedFile{
		TargetLang: targetLang,
		Imports:    append([]sdk.ImportRef(nil), imports...),
		Runtimes:   append([]sdk.RuntimeRef(nil), runtimes...),
		Models:     gen.allModelSchemas(sdk.RuntimeRef{}),
	}
}

func (gen *Generator) syntaxSnapshot(runtimes []sdk.RuntimeRef, imports []sdk.ImportRef) *sdk.SyntaxFile {
	if gen == nil || gen.File == nil {
		return nil
	}

	snapshot := &sdk.SyntaxFile{
		Imports:         append([]sdk.ImportRef(nil), imports...),
		Runtimes:        append([]sdk.RuntimeRef(nil), runtimes...),
		Models:          make([]sdk.SyntaxModel, 0),
		FieldExtensions: make([]sdk.SyntaxFieldExtension, 0),
		Nodes:           make([]sdk.SyntaxNode, 0),
	}

	if targetNode, ok := gen.File.Target(); ok && targetNode != nil {
		snapshot.Target = sdk.SyntaxTarget{
			Args:      convertValues(targetNode.Args),
			NamedArgs: convertNamedValues(targetNode.NamedArgs, targetNode.Position, "", sdk.AttrOriginScopeUnknown),
			Origin:    convertOrigin(targetNode.Position, "", sdk.AttrOriginScopeUnknown),
		}
		snapshot.Nodes = append(snapshot.Nodes, sdk.SyntaxNode{
			ID:     "target",
			Kind:   "target",
			Name:   "target",
			Origin: snapshot.Target.Origin,
			Level:  sdk.ASTLevelSyntax,
		})
	}

	for _, model := range gen.File.Models() {
		if model == nil {
			continue
		}

		syntaxModel := sdk.SyntaxModel{
			Name:          model.Name,
			Attrs:         convertAttrs(model.Attrs, sdk.AttrOriginScopeModel),
			Methods:       gen.convertMethods(model.Methods, sdk.RuntimeRef{}, nil, false),
			Fields:        make([]sdk.SyntaxField, 0, len(model.Fields)),
			GeneratedFrom: model.GeneratedFrom,
			GroupName:     model.GroupName,
			Origin:        convertOrigin(model.Position, "", sdk.AttrOriginScopeModel),
		}
		modelNodeID := "model:" + model.Name
		snapshot.Nodes = append(snapshot.Nodes, sdk.SyntaxNode{
			ID:     modelNodeID,
			Kind:   "model",
			Name:   model.Name,
			Attrs:  syntaxModel.Attrs,
			Origin: syntaxModel.Origin,
			Level:  sdk.ASTLevelSyntax,
		})
		for _, method := range syntaxModel.Methods {
			snapshot.Nodes = append(snapshot.Nodes, sdk.SyntaxNode{
				ID:       modelNodeID + ".method:" + method.Name,
				Kind:     "method",
				Name:     method.Name,
				Model:    model.Name,
				Attrs:    method.Attrs,
				Methods:  []sdk.Method{method},
				Origin:   sdk.AttrOrigin{},
				Level:    sdk.ASTLevelSyntax,
				ParentID: modelNodeID,
			})
		}

		for _, field := range model.Fields {
			if field == nil {
				continue
			}

			methods := gen.convertMethods(field.Methods, sdk.RuntimeRef{}, nil, false)
			syntaxField := sdk.SyntaxField{
				Name:    field.Name,
				Type:    gen.convertTypeRef(field.Type),
				Default: convertOptionalValue(field.Default),
				Attrs:   convertAttrs(field.Attrs, sdk.AttrOriginScopeField),
				Methods: methods,
				Origin:  convertOrigin(field.Position, "", sdk.AttrOriginScopeField),
			}
			syntaxModel.Fields = append(syntaxModel.Fields, syntaxField)
			fieldNodeID := modelNodeID + ".field:" + field.Name
			snapshot.Nodes = append(snapshot.Nodes, sdk.SyntaxNode{
				ID:       fieldNodeID,
				Kind:     "field",
				Name:     field.Name,
				Model:    model.Name,
				Type:     syntaxField.Type,
				Default:  syntaxField.Default,
				Attrs:    syntaxField.Attrs,
				Methods:  methods,
				Origin:   syntaxField.Origin,
				Level:    sdk.ASTLevelSyntax,
				ParentID: modelNodeID,
			})

			for _, method := range methods {
				snapshot.Nodes = append(snapshot.Nodes, sdk.SyntaxNode{
					ID:       fieldNodeID + ".method:" + method.Name,
					Kind:     "method",
					Name:     method.Name,
					Model:    model.Name,
					Field:    field.Name,
					Attrs:    method.Attrs,
					Methods:  []sdk.Method{method},
					Origin:   sdk.AttrOrigin{},
					Level:    sdk.ASTLevelSyntax,
					ParentID: fieldNodeID,
				})
			}
		}

		snapshot.Models = append(snapshot.Models, syntaxModel)
	}

	for _, extension := range gen.File.FieldExtensions() {
		if extension == nil {
			continue
		}
		item := sdk.SyntaxFieldExtension{
			Model:  extension.Model.String(),
			Field:  extension.Field.String(),
			Attrs:  convertAttrs(extension.Attrs, sdk.AttrOriginScopeFieldExtension),
			Origin: convertOrigin(extension.Position, "", sdk.AttrOriginScopeFieldExtension),
		}
		snapshot.FieldExtensions = append(snapshot.FieldExtensions, item)
		snapshot.Nodes = append(snapshot.Nodes, sdk.SyntaxNode{
			ID:     "field:" + item.Model + "." + item.Field,
			Kind:   "field_extension",
			Model:  item.Model,
			Field:  item.Field,
			Attrs:  item.Attrs,
			Origin: item.Origin,
			Level:  sdk.ASTLevelSyntax,
		})
	}

	return snapshot
}

func (gen *Generator) modelSchema(model *ast.ModelAST, runtimeRef sdk.RuntimeRef) sdk.Model {
	if model == nil {
		return sdk.Model{}
	}

	ownedNamespaces := gen.runtimeOwnedNamespaces(runtimeRef)

	modelSchema := sdk.Model{
		Name:    model.Name,
		Attrs:   convertAttrs(model.Attrs, sdk.AttrOriginScopeModel),
		Methods: gen.convertMethods(model.Methods, runtimeRef, ownedNamespaces, model.GeneratedFrom == ""),
	}
	if model.GeneratedFrom == "" {
		modelSchema.RuntimeAttrs = filterRuntimeAttrs(modelSchema.Attrs, runtimeRef, ownedNamespaces)
	}

	fields := make([]sdk.Field, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		fieldSchema := sdk.Field{
			Name:    field.Name,
			Type:    gen.convertTypeRef(field.Type),
			Attrs:   convertAttrs(field.Attrs, sdk.AttrOriginScopeField),
			Methods: gen.convertMethods(field.Methods, runtimeRef, ownedNamespaces, model.GeneratedFrom == ""),
		}
		if model.GeneratedFrom == "" {
			fieldSchema.RuntimeAttrs = filterRuntimeAttrs(fieldSchema.Attrs, runtimeRef, ownedNamespaces)
		}
		fields = append(fields, fieldSchema)
	}

	modelSchema.Fields = fields
	return modelSchema
}

func (gen *Generator) convertTypeRef(typeRef ast.TypeRef) sdk.TypeRef {
	typeRef = gen.resolveModelFieldTypeRef(typeRef)
	typeRef = gen.resolveTypeAliasTypeRef(typeRef)
	return sdk.TypeRef{
		Name:     typeRef.Name.String(),
		IsList:   typeRef.IsList,
		Optional: typeRef.Optional,
	}
}

func convertAttrs(items []ast.Attr, defaultScope string) []sdk.Attr {
	if len(items) == 0 {
		return nil
	}

	attrs := make([]sdk.Attr, 0, len(items))
	for _, item := range items {
		attr := sdk.Attr{
			Package:    item.Package.String(),
			Name:       item.Name.String(),
			Identifier: item.Identifier(),
			Args:       convertValues(item.Args),
			NamedArgs:  convertNamedValues(item.NamedArgs, item.Position, item.Origin, defaultScope),
			Origin:     convertOrigin(item.Position, item.Origin, defaultScope),
		}
		attrs = append(attrs, attr)
	}

	return attrs
}

func (gen *Generator) convertMethods(items []ast.FieldMethodAST, runtimeRef sdk.RuntimeRef, ownedNamespaces map[string]struct{}, includeRuntime bool) []sdk.Method {
	if len(items) == 0 {
		return nil
	}

	methods := make([]sdk.Method, 0, len(items))
	for _, item := range items {
		method := sdk.Method{
			Name:    item.Name,
			Params:  gen.convertMethodParams(item.Params),
			Returns: gen.convertMethodReturns(item.Returns),
			Attrs:   convertAttrs(item.Attrs, sdk.AttrOriginScopeMethod),
		}
		if includeRuntime {
			method.RuntimeAttrs = filterRuntimeAttrs(method.Attrs, runtimeRef, ownedNamespaces)
		}
		methods = append(methods, method)
	}

	return methods
}

func (gen *Generator) convertMethodParams(items []ast.FieldMethodParamAST) []sdk.MethodParam {
	if len(items) == 0 {
		return nil
	}

	params := make([]sdk.MethodParam, 0, len(items))
	for _, item := range items {
		params = append(params, sdk.MethodParam{
			Name: item.Name,
			Type: gen.convertTypeRef(item.Type),
		})
	}

	return params
}

func (gen *Generator) convertMethodReturns(items []ast.TypeRef) []sdk.TypeRef {
	if len(items) == 0 {
		return nil
	}

	returns := make([]sdk.TypeRef, 0, len(items))
	for _, item := range items {
		returns = append(returns, gen.convertTypeRef(item))
	}

	return returns
}

func (gen *Generator) resolveModelFieldTypeRef(typeRef ast.TypeRef) ast.TypeRef {
	return gen.resolveModelFieldTypeRefSeen(typeRef, make(map[string]struct{}))
}

func (gen *Generator) resolveModelFieldTypeRefSeen(typeRef ast.TypeRef, seen map[string]struct{}) ast.TypeRef {
	if gen == nil || gen.File == nil || len(typeRef.Name.Parts) != 2 {
		return typeRef
	}

	modelName := strings.TrimSpace(typeRef.Name.Parts[0])
	fieldName := strings.TrimSpace(typeRef.Name.Parts[1])
	key := modelName + "." + fieldName
	if _, exists := seen[key]; exists {
		return typeRef
	}
	seen[key] = struct{}{}
	model, ok := gen.File.FindModel(modelName)
	if !ok || model == nil {
		return typeRef
	}

	field, ok := model.FindField(fieldName)
	if !ok || field == nil {
		return typeRef
	}

	resolved := field.Type
	if len(resolved.Name.Parts) == 2 {
		resolved = gen.resolveModelFieldTypeRefSeen(resolved, seen)
	}

	resolved.IsList = resolved.IsList || typeRef.IsList
	resolved.Optional = resolved.Optional || typeRef.Optional
	return resolved
}

func (gen *Generator) resolveTypeAliasTypeRef(typeRef ast.TypeRef) ast.TypeRef {
	return gen.resolveTypeAliasTypeRefSeen(typeRef, make(map[string]struct{}))
}

func (gen *Generator) resolveTypeAliasTypeRefSeen(typeRef ast.TypeRef, seen map[string]struct{}) ast.TypeRef {
	if gen == nil || gen.File == nil {
		return typeRef
	}

	typeRef = gen.resolveModelFieldTypeRef(typeRef)
	if len(typeRef.Name.Parts) != 1 {
		return typeRef
	}

	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" {
		return typeRef
	}
	if _, exists := seen[name]; exists {
		return typeRef
	}

	item, ok := gen.File.FindType(name)
	if !ok || item == nil {
		return typeRef
	}

	seen[name] = struct{}{}
	resolved := gen.resolveTypeAliasTypeRefSeen(item.Type, seen)
	resolved.IsList = resolved.IsList || typeRef.IsList
	resolved.Optional = resolved.Optional || typeRef.Optional
	return resolved
}

func convertValues(items []ast.Expr) []sdk.Value {
	if len(items) == 0 {
		return nil
	}

	values := make([]sdk.Value, 0, len(items))
	for _, item := range items {
		values = append(values, convertValue(item))
	}

	return values
}

func convertNamedValues(items []ast.NamedArg, fallbackPosition tokpkg.Position, fallbackOrigin string, defaultScope string) []sdk.NamedValue {
	if len(items) == 0 {
		return nil
	}

	values := make([]sdk.NamedValue, 0, len(items))
	for _, item := range items {
		values = append(values, sdk.NamedValue{
			Name:  item.Name,
			Value: convertValue(item.Value),
			Origin: convertOrigin(
				coalescePosition(item.Position, fallbackPosition),
				item.Origin,
				coalesceOrigin(item.Origin, fallbackOrigin, defaultScope),
			),
		})
	}

	return values
}

func convertOrigin(position tokpkg.Position, explicitOrigin string, defaultScope string) sdk.AttrOrigin {
	scope := coalesceOrigin(explicitOrigin, defaultScope)
	return sdk.AttrOrigin{
		Scope:  scope,
		Path:   strings.TrimSpace(position.File),
		Line:   position.Line,
		Column: position.Column,
	}
}

func coalescePosition(position tokpkg.Position, fallback tokpkg.Position) tokpkg.Position {
	if strings.TrimSpace(position.File) != "" || position.Line > 0 || position.Column > 0 {
		return position
	}
	return fallback
}

func coalesceOrigin(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return sdk.AttrOriginScopeUnknown
}

func convertValue(expr ast.Expr) sdk.Value {
	switch value := expr.(type) {
	case ast.StringExpr:
		return sdk.Value{Kind: "string", Text: value.Value}
	case ast.NumberExpr:
		return sdk.Value{Kind: "number", Text: value.Value}
	case ast.BoolExpr:
		return sdk.Value{Kind: "bool", Bool: value.Value}
	case ast.NameExpr:
		return sdk.Value{Kind: "name", Text: value.Name.String()}
	case ast.GoExpr:
		return sdk.Value{Kind: "go", Text: value.Text}
	default:
		return sdk.Value{Kind: "unknown"}
	}
}

func convertOptionalValue(expr ast.Expr) *sdk.Value {
	if expr == nil {
		return nil
	}
	value := convertValue(expr)
	return &value
}

func filterRuntimeAttrs(items []sdk.Attr, runtimeRef sdk.RuntimeRef, ownedNamespaces map[string]struct{}) []sdk.Attr {
	if len(items) == 0 || strings.TrimSpace(runtimeRef.Name) == "" {
		return nil
	}

	filtered := make([]sdk.Attr, 0)
	for _, item := range items {
		if attrBelongsToRuntime(item, runtimeRef, ownedNamespaces) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func attrBelongsToRuntime(attr sdk.Attr, runtimeRef sdk.RuntimeRef, ownedNamespaces map[string]struct{}) bool {
	runtimeName := strings.TrimSpace(runtimeRef.Name)
	if runtimeName == "" {
		return false
	}

	if len(ownedNamespaces) > 0 {
		if _, ok := ownedNamespaces[strings.TrimSpace(attr.Identifier)]; ok {
			return true
		}
		if _, ok := ownedNamespaces[strings.TrimSpace(attr.Namespace())]; ok {
			return true
		}
		if _, ok := ownedNamespaces[strings.TrimSpace(attr.Name)]; ok {
			return true
		}
	}

	if attr.Package == runtimeName || attr.Name == runtimeName || attr.Identifier == runtimeName {
		return true
	}

	return strings.HasPrefix(attr.Package, runtimeName+".") || strings.HasPrefix(attr.Identifier, runtimeName+".")
}

func (gen *Generator) ensureRuntimeNamespaces(runtimes []sdk.RuntimeRef) error {
	if gen == nil {
		return nil
	}
	if gen.runtimeNamespaces == nil {
		gen.runtimeNamespaces = make(map[string]map[string]struct{})
	}
	if gen.runtimeAttrSpecs == nil {
		gen.runtimeAttrSpecs = make(map[string][]sdk.AttrSpec)
	}

	for _, runtimeRef := range runtimes {
		key := runtimeRefKey(runtimeRef)
		if _, ok := gen.runtimeNamespaces[key]; ok {
			continue
		}

		if err := plugins.EnsureAvailableAt(gen.SourcePath, runtimeRef.Name, runtimeRef.Author); err != nil {
			return err
		}

		namespaces := make(map[string]struct{})
		description, err := plugins.DescribeAttrsAt(gen.SourcePath, runtimeRef.Name, runtimeRef.Author)
		if err != nil {
			return err
		}

		for _, spec := range description.Specs {
			namespace := strings.TrimSpace(spec.Namespace)
			if namespace == "" {
				continue
			}
			namespaces[namespace] = struct{}{}
		}

		gen.runtimeNamespaces[key] = namespaces
		gen.runtimeAttrSpecs[key] = append([]sdk.AttrSpec(nil), description.Specs...)
	}

	return nil
}

func (gen *Generator) runtimeOwnedNamespaces(runtimeRef sdk.RuntimeRef) map[string]struct{} {
	if gen == nil || gen.runtimeNamespaces == nil {
		return nil
	}

	return gen.runtimeNamespaces[runtimeRefKey(runtimeRef)]
}

func (gen *Generator) runtimeAttrSpecsFor(runtimeRef sdk.RuntimeRef) []sdk.AttrSpec {
	if gen == nil || gen.runtimeAttrSpecs == nil {
		return nil
	}

	return gen.runtimeAttrSpecs[runtimeRefKey(runtimeRef)]
}

func runtimeRefKey(runtimeRef sdk.RuntimeRef) string {
	return strings.TrimSpace(runtimeRef.Author) + ":" + strings.TrimSpace(runtimeRef.Name)
}

func (gen *Generator) loadManifest(outputDir string) (*outputManifest, error) {
	path := filepath.Join(outputDir, manifestFileName)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &outputManifest{Models: make(map[string][]string)}, nil
		}

		return nil, fmt.Errorf(localize.Text("чтение манифеста генерации %q: %w", "read generation manifest %q: %w"), path, err)
	}

	var manifest outputManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf(localize.Text("разбор манифеста генерации %q: %w", "parse generation manifest %q: %w"), path, err)
	}
	if manifest.Models == nil {
		manifest.Models = make(map[string][]string)
	}

	return &manifest, nil
}

func (gen *Generator) saveManifest(outputDir string, manifest *outputManifest) error {
	if manifest == nil {
		return nil
	}

	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf(localize.Text("сериализация манифеста генерации: %w", "serialize generation manifest: %w"), err)
	}

	path := filepath.Join(outputDir, manifestFileName)
	if err := fsutil.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf(localize.Text("запись манифеста генерации %q: %w", "write generation manifest %q: %w"), path, err)
	}

	return nil
}

// cleanupStaleOutputs removes files that belonged to older generations but are
// no longer produced by the current model set. This keeps `models/` free from
// stale sidecar artifacts such as obsolete `.proto` or `_grpc.pb.go` files.
func (gen *Generator) cleanupStaleOutputs(outputDir string, manifest *outputManifest, currentModels map[string]struct{}) error {
	if manifest == nil || len(manifest.Models) == 0 {
		return nil
	}

	for modelName, paths := range manifest.Models {
		if _, exists := currentModels[modelName]; exists {
			continue
		}

		for _, relativePath := range paths {
			if err := removeGeneratedFile(outputDir, relativePath); err != nil {
				return err
			}
		}
		delete(manifest.Models, modelName)
	}

	for modelName, paths := range manifest.Models {
		seen := make(map[string]struct{}, len(paths))
		filtered := make([]string, 0, len(paths))
		for _, relativePath := range paths {
			relativePath = normalizeOutputPath(relativePath)
			if relativePath == "" {
				continue
			}
			if _, exists := seen[relativePath]; exists {
				continue
			}
			seen[relativePath] = struct{}{}
			filtered = append(filtered, relativePath)
		}
		sort.Strings(filtered)
		manifest.Models[modelName] = filtered
	}

	return nil
}

func removeGeneratedFile(outputDir string, relativePath string) error {
	relativePath = normalizeOutputPath(relativePath)
	if relativePath == "" {
		return nil
	}

	outputPath := filepath.Join(outputDir, filepath.FromSlash(relativePath))
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(localize.Text("удаление устаревшего файла %q: %w", "remove stale generated file %q: %w"), outputPath, err)
	}

	return nil
}

func (gen *Generator) replaceModelFiles(outputDir string, manifest *outputManifest, modelName string, mainFile string, files []sdk.GeneratedFile) error {
	if manifest == nil {
		return nil
	}

	current := collectModelFiles(mainFile, files)
	previous := append([]string(nil), manifest.Models[modelName]...)
	currentSet := make(map[string]struct{}, len(current))
	for _, item := range current {
		currentSet[item] = struct{}{}
	}

	for _, item := range previous {
		if _, exists := currentSet[item]; exists {
			continue
		}
		if err := removeGeneratedFile(outputDir, item); err != nil {
			return err
		}
	}

	manifest.Models[modelName] = current
	return nil
}

func normalizeOutputPath(relativePath string) string {
	trimmed := strings.TrimSpace(relativePath)
	trimmed = strings.TrimPrefix(trimmed, "./")
	return filepath.ToSlash(trimmed)
}

func generatedFileStem(fileName string) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return ""
	}

	if strings.HasSuffix(name, ".gen.go") {
		return strings.TrimSuffix(name, ".gen.go")
	}

	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}

	return strings.TrimSuffix(name, ext)
}

func (gen *Generator) renderModelPackageFile(renderer targetpkg.Renderer, packageName string, response sdk.GenerateResponse) ([]byte, error) {
	if structured, ok := renderer.(targetpkg.StructuredRenderer); ok {
		return structured.RenderPackageFile(packageName, response)
	}
	return renderer.RenderFile(response)
}

func (gen *Generator) writeTypes(renderer targetpkg.Renderer, outputDir string, manifest *outputManifest) error {
	files, err := gen.renderTypeFiles(renderer, outputDir)
	if err != nil {
		return err
	}
	if err := gen.writeExtraFiles(outputDir, files); err != nil {
		return err
	}
	return gen.replaceModelFiles(outputDir, manifest, defaultTypesKey, "", files)
}

func (gen *Generator) renderTypeFiles(renderer targetpkg.Renderer, outputDir string) ([]sdk.GeneratedFile, error) {
	if gen == nil || gen.File == nil {
		return nil, nil
	}

	types := gen.File.Types()
	if len(types) == 0 {
		return nil, nil
	}

	importIndex := make(map[string]sdk.ImportRef)
	for _, item := range gen.fileImports() {
		importIndex[item.Alias] = item
	}

	typeImports := make(map[string]sdk.ImportRef)
	blocks := make([]sdk.CodeBlock, 0, len(types))
	sorted := append([]*ast.TypeAliasAST(nil), types...)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	for _, item := range sorted {
		if item == nil {
			continue
		}

		typeRef := gen.resolveModelFieldTypeRef(item.Type)
		if len(typeRef.Name.Parts) >= 2 {
			if ref, ok := importIndex[strings.TrimSpace(typeRef.Name.Parts[0])]; ok {
				typeImports[ref.Alias+"|"+ref.Path] = ref
			}
		}

		blocks = append(blocks, sdk.CodeBlock{
			Name:  "type." + item.Name,
			Order: 0,
			Code: sdk.WithDocComment(
				fmt.Sprintf("type %s = %s", item.Name, gen.renderDeclaredTypeRef(typeRef)),
				"%s объявляет generated alias для `%s` из RPL.",
				"%s declares the generated alias for `%s` from RPL.",
				item.Name,
				item.Name,
			),
		})
	}

	typeImportsList := make([]sdk.ImportRef, 0, len(typeImports))
	for _, item := range typeImports {
		typeImportsList = append(typeImportsList, item)
	}
	sort.Slice(typeImportsList, func(i int, j int) bool {
		if typeImportsList[i].Path == typeImportsList[j].Path {
			return typeImportsList[i].Alias < typeImportsList[j].Alias
		}
		return typeImportsList[i].Path < typeImportsList[j].Path
	})

	typeBody, err := gen.renderModelPackageFile(renderer, defaultTypesPkgName, sdk.GenerateResponse{
		Imports: typeImportsList,
		Blocks:  blocks,
	})
	if err != nil {
		return nil, err
	}

	typeImportPath := gen.goPackagePath(filepath.Join(outputDir, defaultTypesDirName), defaultTypesPkgName)
	if strings.TrimSpace(typeImportPath) == "" {
		typeImportPath = "./" + defaultTypesDirName
	}
	rootPackageName := gen.rootPackageName(renderer, outputDir)

	rootBody, err := gen.renderModelPackageFile(renderer, rootPackageName, sdk.GenerateResponse{
		Imports: []sdk.ImportRef{{
			Alias: defaultTypesImport,
			Path:  typeImportPath,
		}},
		Blocks: []sdk.CodeBlock{{
			Name:  "types.facade",
			Order: 0,
			Code:  gen.renderRootTypeAliases(sorted),
		}},
	})
	if err != nil {
		return nil, err
	}

	return []sdk.GeneratedFile{
		{
			Path:    filepath.ToSlash(filepath.Join(defaultTypesDirName, defaultTypesFileName)),
			Content: string(typeBody),
		},
		{
			Path:    defaultTypesFileName,
			Content: string(rootBody),
		},
	}, nil
}

func (gen *Generator) renderDeclaredTypeRef(typeRef ast.TypeRef) string {
	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" {
		name = "any"
	}
	if typeRef.IsList {
		name = "[]" + name
	}
	if typeRef.Optional {
		return "*" + name
	}
	return name
}

func (gen *Generator) renderRootTypeAliases(types []*ast.TypeAliasAST) string {
	lines := make([]string, 0, len(types))
	for _, item := range types {
		if item == nil {
			continue
		}
		lines = append(lines, sdk.WithDocComment(
			fmt.Sprintf("type %s = %s.%s", item.Name, defaultTypesImport, item.Name),
			"%s переэкспортирует shared RPL type alias `%s`.",
			"%s re-exports the shared RPL type alias `%s`.",
			item.Name,
			item.Name,
		))
	}
	return strings.Join(lines, "\n\n")
}

func (gen *Generator) writeFacadeFile(renderer targetpkg.Renderer, layout modelLayout, model *ast.ModelAST, generated sdk.GenerateResponse) ([]sdk.GeneratedFile, error) {
	structured, ok := renderer.(targetpkg.StructuredRenderer)
	if !ok {
		return nil, nil
	}
	if strings.TrimSpace(layout.FacadeFileName) == "" || strings.TrimSpace(layout.ModelImportPath) == "" {
		return nil, nil
	}

	imports := structured.FacadeImports(gen.File, model, layout.ModelImportPath, layout.ModelPackage)
	code := structured.FacadeCode(gen.File, model, layout.ModelImportPath, layout.ModelPackage)
	if withFiles, ok := renderer.(targetpkg.FacadeFilesRenderer); ok {
		imports = withFiles.FacadeImportsWithFiles(gen.File, model, layout.ModelImportPath, layout.ModelPackage, generated.Files)
		code = withFiles.FacadeCodeWithFiles(gen.File, model, layout.ModelImportPath, layout.ModelPackage, generated.Files)
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}

	response := sdk.GenerateResponse{
		Imports: imports,
		Blocks: []sdk.CodeBlock{{
			Name:  "facade",
			Code:  code,
			Order: 0,
		}},
	}
	body, err := structured.RenderPackageFile(layout.RootPackageName, response)
	if err != nil {
		return nil, err
	}

	return []sdk.GeneratedFile{{
		Path:    layout.FacadeRelative,
		Content: string(body),
	}}, nil
}

func prefixGeneratedFiles(prefix string, files []sdk.GeneratedFile) []sdk.GeneratedFile {
	if len(files) == 0 {
		return nil
	}

	prefixed := make([]sdk.GeneratedFile, 0, len(files))
	for _, file := range files {
		item := file
		if strings.TrimSpace(prefix) != "" {
			item.Path = normalizeOutputPath(filepath.Join(prefix, item.Path))
		} else {
			item.Path = normalizeOutputPath(item.Path)
		}
		prefixed = append(prefixed, item)
	}

	return prefixed
}

func (gen *Generator) writeExtraFiles(outputDir string, files []sdk.GeneratedFile) error {
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		relativePath := file.Path
		if relativePath == "" {
			continue
		}

		outputPath := filepath.Join(outputDir, filepath.FromSlash(relativePath))
		if file.Delete {
			if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf(localize.Text("удаление сгенерированного файла %q: %w", "remove generated file %q: %w"), outputPath, err)
			}
			continue
		}

		if err := fsutil.WriteFile(outputPath, []byte(file.Content), 0o644); err != nil {
			return fmt.Errorf(localize.Text("запись сгенерированного файла %q: %w", "write generated file %q: %w"), outputPath, err)
		}
	}

	return nil
}

func collectModelFiles(mainFile string, files []sdk.GeneratedFile) []string {
	items := make([]string, 0, 1+len(files))
	if normalized := normalizeOutputPath(mainFile); normalized != "" {
		items = append(items, normalized)
	}

	seen := make(map[string]struct{}, len(files)+1)
	normalized := make([]string, 0, 1+len(files))
	for _, item := range items {
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	for _, file := range files {
		if file.Delete {
			continue
		}
		item := normalizeOutputPath(file.Path)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	sort.Strings(normalized)
	return normalized
}
