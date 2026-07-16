package decls

import (
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"sort"
	"strings"
)

// GoTopLevelNames returns sorted top-level Go declaration names from a code
// fragment that omits the package clause.
func GoTopLevelNames(code string) []string {
	body := strings.TrimSpace(code)
	if body == "" {
		return nil
	}

	fileSet := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fileSet, "generated.go", "package generated\n\n"+body, goparser.SkipObjectResolution)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, decl := range file.Decls {
		switch item := decl.(type) {
		case *ast.FuncDecl:
			if item.Recv != nil {
				continue
			}
			name := strings.TrimSpace(item.Name.Name)
			if !claimableTopLevelName(name) {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		case *ast.GenDecl:
			for _, spec := range item.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					name := strings.TrimSpace(typed.Name.Name)
					if !claimableTopLevelName(name) {
						continue
					}
					if _, exists := seen[name]; exists {
						continue
					}
					seen[name] = struct{}{}
					names = append(names, name)
				case *ast.ValueSpec:
					for _, ident := range typed.Names {
						name := strings.TrimSpace(ident.Name)
						if !claimableTopLevelName(name) {
							continue
						}
						if _, exists := seen[name]; exists {
							continue
						}
						seen[name] = struct{}{}
						names = append(names, name)
					}
				}
			}
		}
	}

	sort.Strings(names)
	return names
}

func claimableTopLevelName(name string) bool {
	switch strings.TrimSpace(name) {
	case "", "_", "init":
		return false
	default:
		return true
	}
}
