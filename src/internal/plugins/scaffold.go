package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/config"
	"rpl/internal/fsutil"
	"rpl/pkg/error/localize"
	"strings"
)

type ScaffoldInput struct {
	ProjectRoot string
	Identifier  string
	Description string
	DisplayName string
	AttrFolder  string
}

type ScaffoldResult struct {
	AttrDir       string
	ManifestPath  string
	SourcePath    string
	ReadmePath    string
	CreatedPaths  []string
	ExistingPaths []string
}

// CreateScaffold derives the target attr folder from project config and then
// materializes a minimal runnable plugin with manifest, entrypoint, and docs.
func CreateScaffold(input ScaffoldInput) (ScaffoldResult, error) {
	author, name, err := parseIdentifier(input.Identifier)
	if err != nil {
		return ScaffoldResult{}, err
	}

	projectRoot := strings.TrimSpace(input.ProjectRoot)
	if projectRoot == "" {
		projectRoot = "."
	}

	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return ScaffoldResult{}, fmt.Errorf(localize.Text("определение пути проекта %q: %w", "resolve project path %q: %w"), input.ProjectRoot, err)
	}

	pluginsRoot, err := resolvePluginsRoot(projectRoot)
	if err != nil {
		return ScaffoldResult{}, err
	}

	folderName := strings.TrimSpace(input.AttrFolder)
	if folderName == "" {
		folderName = author + ":" + name
	}

	attrDir := filepath.Join(pluginsRoot, folderName)
	if err := os.MkdirAll(attrDir, 0o755); err != nil {
		return ScaffoldResult{}, fmt.Errorf(localize.Text("создание папки attr %q: %w", "create attr directory %q: %w"), attrDir, err)
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = strings.ToUpper(name[:1]) + name[1:] + " Attr"
	}

	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = fmt.Sprintf("Generates %s-related code for RPL models", name)
	}

	result := ScaffoldResult{
		AttrDir:      attrDir,
		ManifestPath: filepath.Join(attrDir, DefaultManifestName),
		SourcePath:   filepath.Join(attrDir, "main.go"),
		ReadmePath:   filepath.Join(attrDir, "README.md"),
	}
	generatePath := filepath.Join(attrDir, "generate.go")
	analysisPath := filepath.Join(attrDir, "analysis.go")

	if created, err := writeFileIfMissing(result.ManifestPath, manifestTemplate(author, name, displayName, description)); err != nil {
		return ScaffoldResult{}, err
	} else if created {
		result.CreatedPaths = append(result.CreatedPaths, result.ManifestPath)
	} else {
		result.ExistingPaths = append(result.ExistingPaths, result.ManifestPath)
	}

	if created, err := writeFileIfMissing(result.SourcePath, sourceTemplate(author, name)); err != nil {
		return ScaffoldResult{}, err
	} else if created {
		result.CreatedPaths = append(result.CreatedPaths, result.SourcePath)
	} else {
		result.ExistingPaths = append(result.ExistingPaths, result.SourcePath)
	}

	if created, err := writeFileIfMissing(generatePath, generateTemplate(name)); err != nil {
		return ScaffoldResult{}, err
	} else if created {
		result.CreatedPaths = append(result.CreatedPaths, generatePath)
	} else {
		result.ExistingPaths = append(result.ExistingPaths, generatePath)
	}

	if created, err := writeFileIfMissing(analysisPath, analysisTemplate(name)); err != nil {
		return ScaffoldResult{}, err
	} else if created {
		result.CreatedPaths = append(result.CreatedPaths, analysisPath)
	} else {
		result.ExistingPaths = append(result.ExistingPaths, analysisPath)
	}

	if created, err := writeFileIfMissing(result.ReadmePath, readmeTemplate(author, name)); err != nil {
		return ScaffoldResult{}, err
	} else if created {
		result.CreatedPaths = append(result.CreatedPaths, result.ReadmePath)
	} else {
		result.ExistingPaths = append(result.ExistingPaths, result.ReadmePath)
	}

	return result, nil
}

// resolvePluginsRoot reads the project config if it exists and resolves the
// attrs directory against the project root. That keeps scaffolding aligned
// with the actual discovery path instead of hardcoding another folder.
func resolvePluginsRoot(projectRoot string) (string, error) {
	configPath := filepath.Join(projectRoot, config.DefaultPath)
	cfg, err := config.LoadOrDefault(configPath)
	if err != nil {
		return "", err
	}

	dir := strings.TrimSpace(cfg.Runtimes.Directory)
	if dir == "" {
		dir = ".rpl/attrs"
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(projectRoot, dir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf(localize.Text("создание папки attrs %q: %w", "create attrs directory %q: %w"), dir, err)
	}

	return dir, nil
}

func parseIdentifier(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	author, name, ok := strings.Cut(trimmed, ":")
	if !ok || strings.TrimSpace(author) == "" || strings.TrimSpace(name) == "" {
		return "", "", errors.New(localize.Text("идентификатор attr должен быть в формате author:name", "attr identifier must use author:name format"))
	}

	return strings.TrimSpace(author), strings.TrimSpace(name), nil
}

func writeFileIfMissing(path string, body string) (bool, error) {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf(localize.Text("чтение пути %q: %w", "stat path %q: %w"), path, err)
	}

	if err := fsutil.WriteFile(path, []byte(body), 0o644); err != nil {
		return false, fmt.Errorf(localize.Text("запись файла %q: %w", "write file %q: %w"), path, err)
	}

	return true, nil
}

func manifestTemplate(author string, name string, displayName string, description string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<attr>
  <name>%s</name>
  <author>%s</author>
  <version>0.1.0</version>
  <displayName>%s</displayName>
  <description>%s</description>
  <entry>attr</entry>
  <sdkVersion>2</sdkVersion>
</attr>
`, name, author, displayName, description)
}

func sourceTemplate(author string, name string) string {
	return fmt.Sprintf(`package main

import (
	"os"
	"rpl/pkg/sdk/analysis"
	"rpl/pkg/sdk/runtime"
)

func main() {
	attr := runtime.NewAttr(%q, %q)
	attr.HandlePing()
	attr.HandleDescribeAttrs(attrSpec)
	attr.HandleDescribeCapabilities(runtime.AttrCapabilities{
		AnalyzeModel:  true,
		AnalyzeFile:   true,
		GenerateModel: true,
		GenerateFile:  true,
		DocsModel:     true,
		DocsFile:      true,
		DescribeAttrs: true,
	})
	attr.HandleAnalyzeModel(analyzeModel)
	attr.HandleAnalyzeFile(analyzeFile)
	attr.HandleGenerateModel(generateModel)
	attr.HandleGenerateFile(generateFile)
	attr.HandleDocsModel(docsModel)
	attr.HandleDocsFile(docsFile)

	if err := attr.Run(); err != nil {
		analysis.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
`, name, author)
}

func generateTemplate(name string) string {
	return fmt.Sprintf(`package main

import "rpl/pkg/sdk/codegen"

func generateModel(req codegen.GenerateRequest) (codegen.GenerateResponse, error) {
	builder := codegen.NewCodeBuilder()

	values := req.Model.ResolvedValues(%q)
	_ = values

	// Add generated fragments here. Every block is merged into the final
	// model file together with the core model definition and other attrs.
	// Use builder.AddFile(...) when the attr also needs standalone artifacts
	// such as .proto files, generated SDK code, or sidecar manifests.
	// Полный снимок файла доступен через req.File.Syntax и req.File.Resolved.
	_ = req

	return builder.Response(), nil
}

func generateFile(req codegen.GenerateRequest) (codegen.GenerateResponse, error) {
	builder := codegen.NewCodeBuilder()

	// Используйте этот handler, если attr должен выпустить общий файл для всей схемы:
	// registry, реестр сервисов, README sidecar, общий proto index и т.п.
	_ = req

	return builder.Response(), nil
}
`, name)
}

func analysisTemplate(name string) string {
	return fmt.Sprintf(`package main

import (
	"rpl/pkg/sdk/analysis"
	"rpl/pkg/sdk/attrs"
	"rpl/pkg/sdk/codegen"
	"rpl/pkg/sdk/docs"
)

var attrSpec = attrs.AttrSpec{
	Namespace: %q,
	Help:      "Опишите, что делает attr и какие аргументы он поддерживает.",
	Args: []attrs.AttrArgSpec{
		// {Name: "example", Types: []attrs.AttrValueType{attrs.AttrValueTypeStringLike}, Help: "Пример аргумента."},
	},
}

func analyzeModel(req codegen.GenerateRequest) (analysis.AnalyzeResponse, error) {
	builder := analysis.NewAnalyzeBuilder()

	builder.ValidateAttrSpec(req.Model.RuntimeAttrs, attrSpec)

	// Add diagnostics and claims here. Diagnostics should explain schema
	// problems, while claims should describe generated identifiers/files so
	// RPL can detect collisions before writing output.
	_ = req

	return builder.Response(), nil
}

func analyzeFile(req codegen.GenerateRequest) (analysis.AnalyzeResponse, error) {
	builder := analysis.NewAnalyzeBuilder()

	// Здесь можно валидировать общие ограничения по всему файлу:
	// конфликты имён, правила для нескольких моделей, file-level claims.
	// Для точной source-aware логики используйте req.File.Syntax.
	// Для уже нормализованной схемы используйте req.File.Resolved.
	_ = req

	return builder.Response(), nil
}

func docsModel(req docs.DocsRequest) (docs.DocsResponse, error) {
	return docs.DocsResponse{
		Sections: []docs.DocsSection{
			{
				Title: "Что делает attr",
				Body:  "Заполните это описание тем, что attr генерирует для текущей модели.",
				Order: 10,
			},
		},
	}, nil
}

func docsFile(req docs.DocsRequest) (docs.DocsResponse, error) {
	return docs.DocsResponse{
		Sections: []docs.DocsSection{
			{
				Title: "Общая документация attr",
				Body:  "Заполните этот раздел общей документацией по attr для всего файла схемы.",
				Order: 10,
			},
		},
	}, nil
}
`, name)
}

func readmeTemplate(author string, name string) string {
	return fmt.Sprintf(`# %s:%s

Это каркас attr для RPL.

## Файлы

- manifest.xml: метаданные attr для discovery
- main.go: точка входа и описание capabilities
- generate.go: генерация по модели и по файлу
- analysis.go: диагностика, claims и документация

## Сборка

Из этой папки можно собрать attr так:

`+"```bash\n"+`go build -o attr *.go
`+"```"+`

## Контракт

- Use `+"`runtime.NewAttr`"+` to create the JSON server
- Use `+"`attr.HandleDescribeAttrs(...)`"+` so editors can read help and allowed args from your attr
- Describe attr arguments through `+"`attrs.AttrSpec`"+` instead of hand-written switch chains
- Use `+"`analysis.NewAnalyzeBuilder()`"+` and `+"`builder.ValidateAttrSpec(...)`"+` for diagnostics
- Use `+"`field.ResolvedValues(...)`"+`, `+"`field.ResolvedAttr(...)`"+` and `+"`field.AttrOrigins(...)`"+` when reading attrs with precedence
- Use `+"`field.Type.RefModel(req.File)`"+`, `+"`field.Type.IsModel(req.File)`"+`, `+"`field.Type.ElementType()`"+` and `+"`field.Type.NullableBase()`"+` for type-aware generation
- Use `+"`attr.HandleAnalyzeModel(...)`"+` to validate schema fragments and claim generated names/files
- Use `+"`attr.HandleGenerateModel(...)`"+` to receive model generation requests
- Build code with `+"`codegen.NewCodeBuilder()`"+` and return `+"`codegen.GenerateResponse`"+`
`, author, name)
}
