package sdk

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func NewCodeBuilder() *CodeBuilder {
	return &CodeBuilder{
		imports: make(map[string]ImportRef),
		blocks:  make([]CodeBlock, 0),
		files:   make(map[string]GeneratedFile),
	}
}

func (builder *CodeBuilder) AddImport(path string, alias ...string) {
	if builder == nil {
		return
	}

	path = strings.TrimSpace(path)
	if path == "" {
		return
	}

	item := ImportRef{Path: path}
	if len(alias) > 0 {
		item.Alias = strings.TrimSpace(alias[0])
	}
	item = NormalizeImportRef(item)

	key := item.Path + "|" + item.Alias
	builder.imports[key] = item
}

func (builder *CodeBuilder) AddImports(items ...ImportRef) {
	if builder == nil {
		return
	}

	for _, item := range items {
		builder.AddImport(item.Path, item.Alias)
	}
}

func (builder *CodeBuilder) AddBlock(name string, code string) {
	builder.AddOrderedBlock(name, code, len(builder.blocks))
}

func (builder *CodeBuilder) AddOrderedBlock(name string, code string, order int) {
	if builder == nil {
		return
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return
	}

	builder.blocks = append(builder.blocks, CodeBlock{
		Name:  strings.TrimSpace(name),
		Code:  code,
		Order: order,
	})
}

func (builder *CodeBuilder) AddResponse(response GenerateResponse) {
	if builder == nil {
		return
	}

	builder.AddImports(response.Imports...)
	for _, block := range response.Blocks {
		builder.AddOrderedBlock(block.Name, block.Code, block.Order)
	}
	builder.AddFiles(response.Files...)
}

func (builder *CodeBuilder) AddFile(path string, content string) {
	if builder == nil {
		return
	}

	path = normalizeGeneratedPath(path)
	if path == "" || strings.TrimSpace(content) == "" {
		return
	}

	builder.files[path] = GeneratedFile{
		Path:    path,
		Content: content,
	}
}

func (builder *CodeBuilder) DeleteFile(path string) {
	if builder == nil {
		return
	}

	path = normalizeGeneratedPath(path)
	if path == "" {
		return
	}

	builder.files[path] = GeneratedFile{
		Path:   path,
		Delete: true,
	}
}

func (builder *CodeBuilder) AddFiles(items ...GeneratedFile) {
	if builder == nil {
		return
	}

	for _, item := range items {
		if item.Delete {
			builder.DeleteFile(item.Path)
			continue
		}

		builder.AddFile(item.Path, item.Content)
	}
}

func (builder *CodeBuilder) Response() GenerateResponse {
	if builder == nil {
		return GenerateResponse{}
	}

	imports := make([]ImportRef, 0, len(builder.imports))
	for _, item := range builder.imports {
		imports = append(imports, item)
	}

	sort.Slice(imports, func(i int, j int) bool {
		if imports[i].Path == imports[j].Path {
			return imports[i].Alias < imports[j].Alias
		}
		return imports[i].Path < imports[j].Path
	})

	blocks := append([]CodeBlock(nil), builder.blocks...)
	sort.SliceStable(blocks, func(i int, j int) bool {
		if blocks[i].Order == blocks[j].Order {
			return blocks[i].Name < blocks[j].Name
		}
		return blocks[i].Order < blocks[j].Order
	})

	files := make([]GeneratedFile, 0, len(builder.files))
	for _, item := range builder.files {
		files = append(files, item)
	}

	sort.Slice(files, func(i int, j int) bool {
		return files[i].Path < files[j].Path
	})

	return GenerateResponse{
		Imports: imports,
		Blocks:  blocks,
		Files:   files,
	}
}

func NormalizeImportRef(item ImportRef) ImportRef {
	item.Path = strings.TrimSpace(item.Path)
	item.Alias = strings.TrimSpace(item.Alias)
	if item.Path == "" {
		return item
	}

	if item.Alias == path.Base(item.Path) {
		item.Alias = ""
	}

	return item
}

func normalizeGeneratedPath(value string) string {
	value = filepath.ToSlash(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if path.IsAbs(value) {
		return ""
	}

	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return ""
	}

	return cleaned
}
