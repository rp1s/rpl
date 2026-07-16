package compiler

import (
	goparser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"rpl/internal/formatter"
	"rpl/internal/fsutil"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/plugins"
	rplerr "rpl/pkg/error"
	"sort"
	"strings"
)

func (service *Service) AutoSetImportsFile(path string) (string, error) {
	absolutePath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}

	body, err := os.ReadFile(absolutePath)
	if err != nil {
		return "", err
	}

	return service.AutoSetImports(string(body), absolutePath)
}

func (service *Service) AutoSetImports(code string, sourcePath string) (string, error) {
	if formatter.HasLineComments(code) {
		return formatter.PreserveCommentedSource(code), nil
	}

	file, absolutePath, err := service.loadCodeWithoutFinalize(code, sourcePath)
	if err != nil {
		return "", err
	}

	attrSpecs, err := discoverAttrNamespaces(absolutePath)
	if err != nil {
		return "", err
	}

	usedNamespaces := collectUsedAttrNamespaces(file)
	requiredOwners := collectRequiredAttrOwners(usedNamespaces, attrSpecs)
	if len(requiredOwners) > 0 {
		ensureAttrBlock(file)
		for _, owner := range requiredOwners {
			ensureRuntimeSpec(file, owner)
		}
	}
	pruneUnusedRuntimeSpecs(file, requiredOwners)

	importCandidates, err := collectImportCandidates(file, absolutePath)
	if err != nil {
		return "", err
	}
	if len(importCandidates) > 0 {
		ensureImportBlock(file)
		for _, item := range importCandidates {
			ensureImportSpec(file, item)
		}
	}

	return formatter.Render(file), nil
}

func (service *Service) loadCodeWithoutFinalize(code string, sourcePath string) (*ast.File, string, error) {
	trimmedPath := strings.TrimSpace(sourcePath)
	if trimmedPath == "" {
		file, err := service.parseRaw(code, "")
		if err != nil {
			return nil, "", err
		}
		return file, "", nil
	}

	absolutePath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return nil, "", err
	}

	file, err := service.parseRaw(code, absolutePath)
	if err != nil {
		return nil, "", err
	}

	return file, absolutePath, nil
}

type attrOwner struct {
	Name   string
	Author string
}

func discoverAttrNamespaces(sourcePath string) (map[string]attrOwner, error) {
	items, err := plugins.ListConfiguredAt(sourcePath)
	if err != nil {
		return nil, err
	}

	result := make(map[string]attrOwner)
	ambiguous := make(map[string]struct{})

	for _, item := range items {
		owner := attrOwner{Name: item.Manifest.Name, Author: item.Manifest.Author}
		registerAttrNamespace(result, ambiguous, strings.TrimSpace(item.Manifest.Name), owner)

		description, err := plugins.DescribeAttrsAt(sourcePath, item.Manifest.Name, item.Manifest.Author)
		if err != nil {
			return nil, err
		}

		for _, spec := range description.Specs {
			registerAttrNamespace(result, ambiguous, strings.TrimSpace(spec.Namespace), owner)
		}
	}

	return result, nil
}

func registerAttrNamespace(result map[string]attrOwner, ambiguous map[string]struct{}, namespace string, owner attrOwner) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return
	}
	if _, ok := ambiguous[namespace]; ok {
		return
	}
	current, exists := result[namespace]
	if exists && (current.Name != owner.Name || current.Author != owner.Author) {
		delete(result, namespace)
		ambiguous[namespace] = struct{}{}
		return
	}
	result[namespace] = owner
}

func collectUsedAttrNamespaces(file *ast.File) []string {
	if file == nil {
		return nil
	}

	seen := make(map[string]struct{})
	appendAttrs := func(items []ast.Attr) {
		for _, attr := range items {
			namespace := attrNamespace(attr)
			if namespace == "" {
				continue
			}
			seen[namespace] = struct{}{}
		}
	}

	for _, model := range file.Models() {
		if model == nil {
			continue
		}
		appendAttrs(model.Attrs)
		for _, method := range model.Methods {
			appendAttrs(method.Attrs)
		}
		for _, field := range model.Fields {
			if field == nil {
				continue
			}
			appendAttrs(field.Attrs)
			for _, method := range field.Methods {
				appendAttrs(method.Attrs)
			}
		}
	}
	for _, field := range file.FieldExtensions() {
		if field != nil {
			appendAttrs(field.Attrs)
		}
	}
	for _, methods := range file.FieldMethodExtensions() {
		if methods == nil {
			continue
		}
		for _, method := range methods.Methods {
			appendAttrs(method.Attrs)
		}
	}
	for _, methods := range file.ModelMethodExtensions() {
		if methods == nil {
			continue
		}
		for _, method := range methods.Methods {
			appendAttrs(method.Attrs)
		}
	}

	items := make([]string, 0, len(seen))
	for namespace := range seen {
		items = append(items, namespace)
	}
	sort.Strings(items)
	return items
}

func attrNamespace(attr ast.Attr) string {
	identifier := strings.TrimSpace(attr.Identifier())
	if identifier == "" {
		return ""
	}
	if strings.Contains(identifier, ".") {
		parts := strings.Split(identifier, ".")
		return strings.TrimSpace(parts[0])
	}
	return identifier
}

func ensureAttrBlock(file *ast.File) {
	if file == nil {
		return
	}
	if len(file.Runtimes()) > 0 {
		return
	}

	node := &ast.RuntimesAST{}
	insertIndex := 0
	if _, ok := file.Package(); ok {
		insertIndex = 1
	}
	if _, ok := file.Target(); ok {
		insertIndex++
	}

	file.ASTs = append(file.ASTs, nil)
	copy(file.ASTs[insertIndex+1:], file.ASTs[insertIndex:])
	file.ASTs[insertIndex] = node
}

func ensureRuntimeSpec(file *ast.File, owner attrOwner) {
	if file == nil || strings.TrimSpace(owner.Name) == "" {
		return
	}

	blocks := file.Runtimes()
	if len(blocks) == 0 {
		return
	}

	block := blocks[0]
	for _, spec := range block.Specs {
		if strings.TrimSpace(spec.Name) == strings.TrimSpace(owner.Name) &&
			strings.TrimSpace(spec.Author) == strings.TrimSpace(owner.Author) {
			return
		}
	}

	block.Specs = append(block.Specs, ast.RuntimeSpec{
		Name:   owner.Name,
		Author: owner.Author,
	})
	sort.Slice(block.Specs, func(i, j int) bool {
		return block.Specs[i].Identifier() < block.Specs[j].Identifier()
	})
}

func collectRequiredAttrOwners(namespaces []string, index map[string]attrOwner) []attrOwner {
	if len(namespaces) == 0 || len(index) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	items := make([]attrOwner, 0, len(namespaces))
	for _, namespace := range namespaces {
		owner, ok := index[strings.TrimSpace(namespace)]
		if !ok {
			continue
		}

		key := attrOwnerIdentifier(owner)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		items = append(items, owner)
	}

	sort.Slice(items, func(i, j int) bool {
		return attrOwnerIdentifier(items[i]) < attrOwnerIdentifier(items[j])
	})
	return items
}

func pruneUnusedRuntimeSpecs(file *ast.File, requiredOwners []attrOwner) {
	if file == nil {
		return
	}

	required := make(map[string]struct{}, len(requiredOwners))
	for _, owner := range requiredOwners {
		key := attrOwnerIdentifier(owner)
		if key != "" {
			required[key] = struct{}{}
		}
	}

	filteredNodes := make([]ast.AST, 0, len(file.ASTs))
	for _, node := range file.ASTs {
		block, ok := node.(*ast.RuntimesAST)
		if !ok || block == nil {
			filteredNodes = append(filteredNodes, node)
			continue
		}

		filteredSpecs := make([]ast.RuntimeSpec, 0, len(block.Specs))
		for _, spec := range block.Specs {
			if _, keep := required[spec.Identifier()]; keep {
				filteredSpecs = append(filteredSpecs, spec)
			}
		}

		if len(filteredSpecs) == 0 {
			continue
		}

		block.Specs = filteredSpecs
		filteredNodes = append(filteredNodes, block)
	}

	file.ASTs = filteredNodes
}

func attrOwnerIdentifier(owner attrOwner) string {
	name := strings.TrimSpace(owner.Name)
	author := strings.TrimSpace(owner.Author)
	switch {
	case name == "":
		return ""
	case author == "":
		return name
	default:
		return author + ":" + name
	}
}

func ensureImportBlock(file *ast.File) {
	if file == nil {
		return
	}
	if len(file.Imports()) > 0 {
		return
	}

	node := &ast.ImportAST{}
	insertIndex := 0
	if _, ok := file.Package(); ok {
		insertIndex = 1
	}
	if _, ok := file.Target(); ok {
		insertIndex++
	}
	if len(file.Runtimes()) > 0 {
		insertIndex++
	}

	file.ASTs = append(file.ASTs, nil)
	copy(file.ASTs[insertIndex+1:], file.ASTs[insertIndex:])
	file.ASTs[insertIndex] = node
}

func ensureImportSpec(file *ast.File, spec ast.ImportSpec) {
	if file == nil || strings.TrimSpace(spec.Path) == "" {
		return
	}

	blocks := file.Imports()
	if len(blocks) == 0 {
		return
	}
	block := blocks[0]
	for _, item := range block.Specs {
		if strings.TrimSpace(item.Path) == strings.TrimSpace(spec.Path) {
			return
		}
	}

	block.Specs = append(block.Specs, spec)
	sort.Slice(block.Specs, func(i, j int) bool {
		if block.Specs[i].Path == block.Specs[j].Path {
			return block.Specs[i].Alias < block.Specs[j].Alias
		}
		return block.Specs[i].Path < block.Specs[j].Path
	})
}

// collectImportCandidates scans field and method signatures for package-style
// type refs and then resolves missing aliases back into concrete Go imports.
func collectImportCandidates(file *ast.File, sourcePath string) ([]ast.ImportSpec, error) {
	if file == nil {
		return nil, nil
	}

	importedAliases := make(map[string]struct{})
	importedPaths := make(map[string]struct{})
	for _, block := range file.Imports() {
		if block == nil {
			continue
		}
		for _, spec := range block.Specs {
			alias := strings.TrimSpace(spec.Alias)
			if alias == "" {
				alias = pathBaseAlias(spec.Path)
			}
			if alias != "" {
				importedAliases[alias] = struct{}{}
			}
			importedPaths[strings.TrimSpace(spec.Path)] = struct{}{}
		}
	}

	neededAliases := make(map[string]struct{})
	appendType := func(typeRef ast.TypeRef) {
		if len(typeRef.Name.Parts) < 2 {
			return
		}
		alias := strings.TrimSpace(typeRef.Name.Parts[0])
		if alias == "" {
			return
		}
		if _, ok := importedAliases[alias]; ok {
			return
		}
		neededAliases[alias] = struct{}{}
	}

	for _, model := range file.Models() {
		if model == nil {
			continue
		}
		for _, method := range model.Methods {
			for _, param := range method.Params {
				appendType(param.Type)
			}
			for _, result := range method.Returns {
				appendType(result)
			}
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
	}
	for _, item := range file.Types() {
		if item == nil {
			continue
		}
		appendType(item.Type)
	}
	for _, methods := range file.FieldMethodExtensions() {
		if methods == nil {
			continue
		}
		for _, method := range methods.Methods {
			for _, param := range method.Params {
				appendType(param.Type)
			}
			for _, result := range method.Returns {
				appendType(result)
			}
		}
	}
	for _, methods := range file.ModelMethodExtensions() {
		if methods == nil {
			continue
		}
		for _, method := range methods.Methods {
			for _, param := range method.Params {
				appendType(param.Type)
			}
			for _, result := range method.Returns {
				appendType(result)
			}
		}
	}

	resolver, err := newGoImportResolver(sourcePath)
	if err != nil {
		return nil, err
	}

	items := make([]ast.ImportSpec, 0)
	for alias := range neededAliases {
		path, ok := resolver.Resolve(alias)
		if !ok {
			continue
		}
		if _, exists := importedPaths[path]; exists {
			continue
		}
		items = append(items, ast.ImportSpec{Path: path})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items, nil
}

type goImportResolver struct {
	aliases map[string]string
}

// newGoImportResolver first mines aliases from the local module tree and only
// later falls back to `go list`, which keeps auto-import deterministic offline.
func newGoImportResolver(sourcePath string) (*goImportResolver, error) {
	resolver := &goImportResolver{aliases: make(map[string]string)}
	projectRoot := nearestGoModuleRoot(sourcePath)
	if projectRoot == "" {
		projectRoot = filepath.Dir(sourcePath)
	}
	if strings.TrimSpace(projectRoot) == "" {
		return resolver, nil
	}

	if err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".rpl" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		fset := token.NewFileSet()
		file, parseErr := goparser.ParseFile(fset, path, nil, goparser.ImportsOnly)
		if parseErr != nil {
			return nil
		}
		for _, item := range file.Imports {
			importPath := strings.Trim(item.Path.Value, `"`)
			if importPath == "" {
				continue
			}
			alias := ""
			if item.Name != nil {
				alias = strings.TrimSpace(item.Name.Name)
			}
			if alias == "" {
				alias = pathBaseAlias(importPath)
			}
			if alias == "" || alias == "_" || alias == "." {
				continue
			}
			if current, ok := resolver.aliases[alias]; ok && current != importPath {
				delete(resolver.aliases, alias)
				continue
			}
			resolver.aliases[alias] = importPath
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return resolver, nil
}

func (resolver *goImportResolver) Resolve(alias string) (string, bool) {
	if resolver == nil {
		return "", false
	}
	alias = strings.TrimSpace(alias)
	path, ok := resolver.aliases[alias]
	if ok {
		return path, true
	}

	output, err := exec.Command("go", "list", "-f", "{{.ImportPath}}|{{.Name}}", alias).CombinedOutput()
	if err != nil {
		return "", false
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 2 {
		return "", false
	}
	if strings.TrimSpace(parts[1]) != alias {
		return "", false
	}

	return strings.TrimSpace(parts[0]), true
}

func nearestGoModuleRoot(sourcePath string) string {
	current := strings.TrimSpace(sourcePath)
	if current == "" {
		return ""
	}
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for current != "" {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return ""
}

func pathBaseAlias(importPath string) string {
	importPath = strings.TrimSpace(importPath)
	if importPath == "" {
		return ""
	}
	base := filepath.Base(importPath)
	return strings.TrimSpace(base)
}

func (service *Service) AutoSetImportsInPlace(path string) error {
	updated, err := service.AutoSetImportsFile(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return rplerr.New("path is required")
	}
	return fsutil.WriteFile(path, []byte(updated), 0o644)
}
