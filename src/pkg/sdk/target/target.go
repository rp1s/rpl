package target

import (
	"strings"

	"rpl/pkg/sdk/codegen"
	"rpl/pkg/sdk/schema"
)

// TypeAlias describes a top-level RPL type declaration such as `type Email string`.
type TypeAlias struct {
	Name string
	Type schema.TypeRef
}

// File captures the resolved file-level data that a language target needs.
type File struct {
	Language    string
	PackageName string
	Imports     []schema.ImportRef
	Models      []schema.Model
	Types       []TypeAlias
}

// Layout describes how one model should be written to disk for a target.
type Layout struct {
	RootPackageName string
	ModelDirName    string
	ModelPackage    string
	ModelFileName   string
	FacadeFileName  string
}

// ModelRequest is the schema-first input for rendering a model file.
type ModelRequest struct {
	File  File
	Model schema.Model
}

// FacadeRequest is the schema-first input for rendering an optional root facade.
type FacadeRequest struct {
	File            File
	Model           schema.Model
	ModelImportPath string
	ModelPackage    string
	GeneratedFiles  []codegen.GeneratedFile
}

// Renderer is the SDK contract for authoring additional language targets.
// It intentionally works with public schema/codegen types instead of compiler internals.
type Renderer interface {
	Name() string
	DefaultRootPackage() string
	ModelLayout(modelName string) Layout
	BaseModelCode(req ModelRequest) string
	UsedImports(req ModelRequest) []codegen.ImportRef
	RenderPackageFile(packageName string, response codegen.GenerateResponse) ([]byte, error)
	FacadeImports(req FacadeRequest) []codegen.ImportRef
	FacadeCode(req FacadeRequest) string
}

func (file File) FindModel(name string) (*schema.Model, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}

	for i := range file.Models {
		if strings.TrimSpace(file.Models[i].Name) == name {
			return &file.Models[i], true
		}
	}

	return nil, false
}

func (file File) FindType(name string) (*TypeAlias, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}

	for i := range file.Types {
		if strings.TrimSpace(file.Types[i].Name) == name {
			return &file.Types[i], true
		}
	}

	return nil, false
}

// OnePackagePerModelLayout returns the common "root/<model>/model.ext + root/<model>.ext" layout.
func OnePackagePerModelLayout(rootPackageName string, modelName string, modelFileName string, facadeFileName string) Layout {
	return Layout{
		RootPackageName: NormalizePackageName(rootPackageName, "models"),
		ModelDirName:    DefaultModelDir(modelName),
		ModelPackage:    DefaultModelPackage(modelName),
		ModelFileName:   strings.TrimSpace(modelFileName),
		FacadeFileName:  strings.TrimSpace(facadeFileName),
	}
}

func DefaultModelDir(modelName string) string {
	name := strings.Trim(strings.TrimSpace(schema.SnakeCase(modelName)), "_")
	if name == "" {
		return "model"
	}
	return name
}

func DefaultModelPackage(modelName string) string {
	name := strings.ReplaceAll(DefaultModelDir(modelName), "_", "")
	if strings.TrimSpace(name) == "" {
		return "model"
	}
	return name
}

func DefaultFacadeFileName(modelName string, ext string) string {
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	name := DefaultModelDir(modelName)
	if ext == "" {
		return name
	}
	return name + ext
}

func NormalizePackageName(name string, fallback string) string {
	name = strings.Trim(strings.TrimSpace(schema.SnakeCase(name)), "_")
	if name == "" {
		name = strings.Trim(strings.TrimSpace(schema.SnakeCase(fallback)), "_")
	}
	if name == "" {
		return "models"
	}
	return name
}
