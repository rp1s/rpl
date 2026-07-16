package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/formatter"
	"rpl/internal/generator"
	"rpl/internal/generator/parser"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

type Service struct{}

type loadedSource struct {
	path string
	file *ast.File
}

func New() *Service {
	return &Service{}
}

func (service *Service) Run(code string) (*ast.File, error) {
	file, err := service.parseAndFinalize(code)
	if err != nil {
		return nil, err
	}

	generator := generator.New(file, "")
	if err := generator.Run(); err != nil {
		return nil, err
	}

	return file, nil
}

func (service *Service) Format(code string, sourcePath string) (string, error) {
	return formatter.Format(code, sourcePath)
}

func (service *Service) Check(code string, sourcePath string) (*ast.File, error) {
	file, absolutePath, err := service.loadCode(code, sourcePath)
	if err != nil {
		return nil, err
	}

	generator := generator.New(file, absolutePath)
	if err := generator.AnalyzeOnly(); err != nil {
		return nil, err
	}

	return file, nil
}

func (service *Service) RunFile(path string) (*ast.File, error) {
	return service.RunFileTo(path, "")
}

func (service *Service) RunFileTo(path string, outputDir string) (*ast.File, error) {
	file, absolutePath, err := service.LoadFile(path)
	if err != nil {
		return nil, err
	}

	generator := generator.New(file, absolutePath, strings.TrimSpace(outputDir))
	if err := generator.Run(); err != nil {
		return nil, err
	}

	return file, nil
}

// LoadFile expands nested `.rpl` imports and sibling files from the same DSL
// package before finalization, so validation sees the same schema the generator
// will use.
func (service *Service) LoadFile(path string) (*ast.File, string, error) {
	absolutePath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, "", fmt.Errorf(localize.Text("определение пути файла %q: %w", "resolve file path %q: %w"), path, err)
	}

	file, err := service.loadUnitRecursive(absolutePath, nil, make(map[string]struct{}), nil)
	if err != nil {
		return nil, "", err
	}

	return service.finalizeLoadedFile(file, absolutePath)
}

func (service *Service) parseAndFinalize(code string) (*ast.File, error) {
	file, err := service.parseRaw(code, "")
	if err != nil {
		return nil, err
	}

	if err := parser.FinalizeFile(file); err != nil {
		return nil, err
	}

	return file, nil
}

func (service *Service) parseRaw(code string, sourcePath string) (*ast.File, error) {
	lex := lexer.NewLexerWithPath(code, sourcePath)
	if err := lex.Run(); err != nil {
		return nil, err
	}

	parser := parser.New(lex)
	return parser.Parse()
}

// loadCode handles both ad-hoc text input and source-backed files. When a real
// file path is present it resolves DSL imports relative to that file first.
func (service *Service) loadCode(code string, sourcePath string) (*ast.File, string, error) {
	trimmedPath := strings.TrimSpace(sourcePath)
	if trimmedPath == "" {
		file, err := service.parseAndFinalize(code)
		if err != nil {
			return nil, "", err
		}

		return file, "", nil
	}

	absolutePath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return nil, "", fmt.Errorf(localize.Text("определение пути файла %q: %w", "resolve file path %q: %w"), trimmedPath, err)
	}

	file, err := service.parseRaw(code, absolutePath)
	if err != nil {
		return nil, "", err
	}

	included := make(map[string]struct{})
	loaded, err := service.loadUnitRecursive(absolutePath, nil, included, file)
	if err != nil {
		return nil, "", err
	}

	return service.finalizeLoadedFile(loaded, absolutePath)
}

func (service *Service) finalizeLoadedFile(file *ast.File, absolutePath string) (*ast.File, string, error) {
	if err := parser.FinalizeFile(file); err != nil {
		return nil, "", err
	}

	return file, absolutePath, nil
}

// loadUnitRecursive walks imported `.rpl` files depth-first while also merging
// sibling files that belong to the same declared DSL package.
func (service *Service) loadUnitRecursive(path string, stack []string, included map[string]struct{}, override *ast.File) (*ast.File, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf(localize.Text("определение пути файла %q: %w", "resolve file path %q: %w"), path, err)
	}

	for _, item := range stack {
		if item == absolutePath {
			chain := append(append([]string(nil), stack...), absolutePath)
			return nil, rplerr.Newf(
				localize.Text("обнаружен рекурсивный импорт: %s", "recursive import detected: %s"),
				strings.Join(chain, " -> "),
			).
				WithLocation(absolutePath, 1, 1).
				WithHint(localize.Text("Разорвите цикл: вынесите общие модели в третий файл и импортируйте их только в одну сторону.", "Break the cycle: move shared models into a third file and import them only one way."))
		}
	}

	if included == nil {
		included = make(map[string]struct{})
	}
	if _, exists := included[absolutePath]; exists && override == nil {
		return &ast.File{ASTs: nil}, nil
	}

	rootFile := override
	if rootFile == nil {
		rootFile, err = service.readAndParseFile(absolutePath)
		if err != nil {
			return nil, err
		}
	}

	unitFiles, err := service.loadPackageFiles(absolutePath, rootFile, included)
	if err != nil {
		return nil, err
	}

	nextStack := append(append([]string(nil), stack...), absolutePath)
	mergedNodes := make([]ast.AST, 0)
	for _, item := range unitFiles {
		resolved, err := service.resolveImports(item.file, item.path, nextStack, included)
		if err != nil {
			return nil, err
		}
		mergedNodes = append(mergedNodes, resolved.ASTs...)
	}

	return &ast.File{ASTs: mergedNodes}, nil
}

func (service *Service) readAndParseFile(path string) (*ast.File, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(localize.Text("чтение файла %q: %w", "read file %q: %w"), path, err)
	}

	return service.parseRaw(string(body), path)
}

func (service *Service) loadPackageFiles(absolutePath string, rootFile *ast.File, included map[string]struct{}) ([]loadedSource, error) {
	if rootFile == nil {
		return nil, nil
	}

	files := make([]loadedSource, 0, 1)
	if _, exists := included[absolutePath]; !exists {
		included[absolutePath] = struct{}{}
	}
	files = append(files, loadedSource{path: absolutePath, file: rootFile})

	packageName := strings.TrimSpace(rootFile.PackageName())
	if packageName == "" {
		return files, nil
	}

	peers, err := service.findPackagePeerPaths(absolutePath, packageName)
	if err != nil {
		return nil, err
	}
	for _, peerPath := range peers {
		if _, exists := included[peerPath]; exists {
			continue
		}

		peerFile, err := service.readAndParseFile(peerPath)
		if err != nil {
			return nil, err
		}

		included[peerPath] = struct{}{}
		files = append(files, loadedSource{path: peerPath, file: peerFile})
	}

	return files, nil
}

func (service *Service) findPackagePeerPaths(rootPath string, packageName string) ([]string, error) {
	dir := filepath.Dir(rootPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf(localize.Text("чтение папки %q: %w", "read directory %q: %w"), dir, err)
	}

	items := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		candidate := filepath.Join(dir, entry.Name())
		if candidate == rootPath {
			continue
		}
		if !isDSLImport(candidate) {
			continue
		}

		body, err := os.ReadFile(candidate)
		if err != nil {
			return nil, fmt.Errorf(localize.Text("чтение файла %q: %w", "read file %q: %w"), candidate, err)
		}
		if probePackageName(string(body)) != packageName {
			continue
		}

		items = append(items, candidate)
	}

	return items, nil
}

// resolveImports replaces DSL import nodes with the imported AST nodes while
// leaving ordinary Go imports in place for the formatter and generator.
func (service *Service) resolveImports(file *ast.File, absolutePath string, stack []string, included map[string]struct{}) (*ast.File, error) {
	if file == nil {
		return &ast.File{ASTs: nil}, nil
	}

	nextStack := append(append([]string(nil), stack...), absolutePath)
	importedNodes := make([]ast.AST, 0)
	currentNodes := make([]ast.AST, 0, len(file.ASTs))

	for _, node := range file.ASTs {
		if _, ok := node.(*ast.PackageAST); ok {
			continue
		}

		importNode, ok := node.(*ast.ImportAST)
		if !ok || importNode == nil {
			currentNodes = append(currentNodes, node)
			continue
		}

		regularSpecs := make([]ast.ImportSpec, 0, len(importNode.Specs))
		for i := range importNode.Specs {
			spec := importNode.Specs[i]
			if !isDSLImport(spec.Path) {
				regularSpecs = append(regularSpecs, spec)
				continue
			}

			importPath := spec.Path
			if !filepath.IsAbs(importPath) {
				importPath = filepath.Join(filepath.Dir(absolutePath), importPath)
			}

			importedFile, err := service.loadUnitRecursive(importPath, nextStack, included, nil)
			if err != nil {
				return nil, err
			}

			importedNodes = append(importedNodes, importedFile.ASTs...)
		}

		if len(regularSpecs) > 0 {
			currentNodes = append(currentNodes, &ast.ImportAST{
				Position: importNode.Position,
				Specs:    regularSpecs,
			})
		}
	}

	mergedNodes := make([]ast.AST, 0, len(importedNodes)+len(currentNodes))
	mergedNodes = append(mergedNodes, importedNodes...)
	mergedNodes = append(mergedNodes, currentNodes...)

	return &ast.File{ASTs: mergedNodes}, nil
}

func isDSLImport(path string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".rpl":
		return true
	default:
		return false
	}
}

func probePackageName(code string) string {
	offset := 0
	for offset < len(code) {
		switch {
		case strings.HasPrefix(code[offset:], "//"):
			offset = skipProbeLine(code, offset)
		case isProbeWhitespace(code[offset]):
			offset++
		default:
			goto scan
		}
	}

scan:
	if !strings.HasPrefix(code[offset:], "package") {
		return ""
	}
	if end := offset + len("package"); end < len(code) {
		next := rune(code[end])
		if next == '_' || next == '-' || (next >= '0' && next <= '9') || (next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z') {
			return ""
		}
		offset = end
	} else {
		offset += len("package")
	}

	for offset < len(code) && isProbeWhitespace(code[offset]) {
		offset++
	}

	start := offset
	for offset < len(code) {
		char := code[offset]
		if char == '_' || (char >= '0' && char <= '9') || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') {
			offset++
			continue
		}
		break
	}
	if start == offset {
		return ""
	}

	return code[start:offset]
}

func skipProbeLine(code string, offset int) int {
	for offset < len(code) && code[offset] != '\n' {
		offset++
	}
	return offset
}

func isProbeWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}
