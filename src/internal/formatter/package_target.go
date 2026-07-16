package formatter

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rpl/internal/generator/parser"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer"
)

// normalizeTargetDirectives removes only redundant package-level target
// directives. Conflicting targets are left intact so validation can still show
// the real problem to the user.
func normalizeTargetDirectives(file *ast.File, sourcePath string) {
	remove := targetNodesToRemove(file, sourcePath)
	if len(remove) == 0 {
		return
	}

	filtered := make([]ast.AST, 0, len(file.ASTs))
	for _, node := range file.ASTs {
		targetNode, ok := node.(*ast.TargetAST)
		if ok {
			if _, drop := remove[targetNode]; drop {
				continue
			}
		}

		filtered = append(filtered, node)
	}

	file.ASTs = filtered
}

func preserveCommentedSourceWithTargetNormalization(code string, sourcePath string) string {
	preserved := strings.TrimRight(code, "\n")
	file, err := parseSourceFile(preserved, sourcePath)
	if err != nil {
		return ensureTrailingNewline(preserved)
	}

	remove := targetNodesToRemove(file, sourcePath)
	if len(remove) == 0 {
		return ensureTrailingNewline(preserved)
	}

	linesToRemove := targetLinesToRemove(preserved, file, remove)
	if len(linesToRemove) == 0 {
		return ensureTrailingNewline(preserved)
	}

	return ensureTrailingNewline(removeLines(preserved, linesToRemove))
}

func parseSourceFile(code string, sourcePath string) (*ast.File, error) {
	lex := lexer.NewLexerWithPath(code, sourcePath)
	if err := lex.Run(); err != nil {
		return nil, err
	}

	return parser.New(lex).Parse()
}

func targetNodesToRemove(file *ast.File, sourcePath string) map[*ast.TargetAST]struct{} {
	targets := file.Targets()
	if len(targets) == 0 || !targetsEquivalent(targets) {
		return nil
	}

	keepCurrent := shouldKeepPackageTarget(file, sourcePath)
	if keepCurrent && len(targets) == 1 {
		return nil
	}

	remove := make(map[*ast.TargetAST]struct{})
	if keepCurrent {
		for _, target := range targets[1:] {
			remove[target] = struct{}{}
		}
		return remove
	}

	for _, target := range targets {
		remove[target] = struct{}{}
	}
	return remove
}

func shouldKeepPackageTarget(file *ast.File, sourcePath string) bool {
	if file == nil {
		return true
	}

	packageName := strings.TrimSpace(file.PackageName())
	if packageName == "" || strings.TrimSpace(sourcePath) == "" {
		return true
	}

	targets := file.Targets()
	if len(targets) == 0 {
		return true
	}

	targetLang := normalizeTargetLang(targetLangValue(targets[0]))
	if targetLang == "" {
		return true
	}

	ownerPath, ok := findPackageTargetOwner(sourcePath, file, packageName, targetLang)
	if !ok {
		return true
	}

	currentPath, err := filepath.Abs(strings.TrimSpace(sourcePath))
	if err != nil {
		return true
	}

	return ownerPath == currentPath
}

func findPackageTargetOwner(sourcePath string, currentFile *ast.File, packageName string, targetLang string) (string, bool) {
	currentPath, err := filepath.Abs(strings.TrimSpace(sourcePath))
	if err != nil {
		return "", false
	}

	dir := filepath.Dir(currentPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	candidates := make([]string, 0, len(entries)+1)
	seen := map[string]struct{}{currentPath: {}}
	candidates = append(candidates, currentPath)
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".rpl" {
			continue
		}

		candidate := filepath.Join(dir, entry.Name())
		if _, exists := seen[candidate]; exists {
			continue
		}

		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)

	for _, candidate := range candidates {
		var file *ast.File
		if candidate == currentPath {
			file = currentFile
		} else {
			body, readErr := os.ReadFile(candidate)
			if readErr != nil {
				continue
			}

			file, err = parseSourceFile(string(body), candidate)
		}
		if err != nil || file == nil {
			continue
		}
		if strings.TrimSpace(file.PackageName()) != packageName {
			continue
		}

		targets := file.Targets()
		if len(targets) == 0 || !targetsEquivalent(targets) {
			continue
		}
		if normalizeTargetLang(targetLangValue(targets[0])) != targetLang {
			continue
		}

		return candidate, true
	}

	return "", false
}

func targetsEquivalent(targets []*ast.TargetAST) bool {
	if len(targets) == 0 {
		return true
	}

	baseline := normalizeTargetLang(targetLangValue(targets[0]))
	if baseline == "" {
		return false
	}

	for _, target := range targets[1:] {
		if normalizeTargetLang(targetLangValue(target)) != baseline {
			return false
		}
	}
	return true
}

func targetLangValue(target *ast.TargetAST) string {
	if target == nil {
		return ""
	}

	value, ok := target.NamedArg("lang")
	if !ok {
		return ""
	}

	return ast.ExprString(value)
}

func normalizeTargetLang(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func targetLinesToRemove(code string, file *ast.File, remove map[*ast.TargetAST]struct{}) map[int]struct{} {
	lines := strings.Split(code, "\n")
	lineNumbers := make(map[int]struct{})
	for _, target := range file.Targets() {
		if _, ok := remove[target]; !ok {
			continue
		}

		span := directiveLineSpan(lines, target.Position.Line)
		for _, line := range span {
			lineNumbers[line] = struct{}{}
		}

		startLine := target.Position.Line
		endLine := startLine
		if len(span) > 0 {
			endLine = span[len(span)-1]
		}
		if startLine > 1 && endLine < len(lines) && isBlankLine(lines[startLine-2]) && isBlankLine(lines[endLine]) {
			lineNumbers[endLine+1] = struct{}{}
		}
	}
	return lineNumbers
}

// directiveLineSpan finds the raw line span of a target(...) directive so the
// formatter can drop it from commented files without re-rendering the source.
func directiveLineSpan(lines []string, startLine int) []int {
	if startLine <= 0 || startLine > len(lines) {
		return nil
	}

	span := make([]int, 0, 1)
	balance := 0
	seenOpen := false
	for line := startLine; line <= len(lines); line++ {
		span = append(span, line)

		text := stripLineComment(lines[line-1])
		balance += strings.Count(text, "(")
		balance -= strings.Count(text, ")")
		if strings.Contains(text, "(") {
			seenOpen = true
		}
		if !seenOpen {
			return span
		}
		if balance <= 0 {
			return span
		}
	}

	return span
}

func stripLineComment(line string) string {
	index := strings.Index(line, "//")
	if index < 0 {
		return line
	}
	return line[:index]
}

func removeLines(code string, linesToRemove map[int]struct{}) string {
	lines := strings.Split(code, "\n")
	filtered := make([]string, 0, len(lines))
	for lineNumber, line := range lines {
		if _, drop := linesToRemove[lineNumber+1]; drop {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func isBlankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}
