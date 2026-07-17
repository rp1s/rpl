package formatter

import (
	"strings"

	"rpl/internal/generator/parser/ast"
)

func Format(code string, sourcePath string) (string, error) {
	if HasLineComments(code) {
		return preserveCommentedSourceWithTargetNormalization(code, sourcePath), nil
	}

	file, err := parseSourceFile(code, sourcePath)
	if err != nil {
		return "", err
	}

	normalizeTargetDirectives(file, sourcePath)
	return Render(file), nil
}

func HasLineComments(code string) bool {
	return strings.Contains(code, "//")
}

func PreserveCommentedSource(code string) string {
	return ensureTrailingNewline(strings.TrimRight(code, "\n"))
}

func ensureTrailingNewline(code string) string {
	if strings.HasSuffix(code, "\n") {
		return code
	}
	return code + "\n"
}

func Render(file *ast.File) string {
	if file == nil {
		return ""
	}

	parts := make([]string, 0)
	for _, node := range file.ASTs {
		if text := renderNode(node); text != "" {
			parts = append(parts, text)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n")) + "\n"
}

func renderNode(node ast.AST) string {
	switch item := node.(type) {
	case *ast.PackageAST:
		return "package " + item.Name
	case *ast.TargetAST:
		return "target(" + renderCallArgs(item.Args, item.NamedArgs) + ")"
	case *ast.ImportAST:
		return renderImports(item)
	case *ast.RuntimesAST:
		return renderRuntimes(item)
	case *ast.TypeAliasAST:
		return renderTypeAlias(item)
	case *ast.ModelAST:
		return renderModel(item)
	case *ast.FieldExtensionAST:
		return renderFieldExtension(item)
	case *ast.FieldMethodsExtensionAST:
		return renderFieldMethodsExtension(item)
	case *ast.ModelMethodsExtensionAST:
		return renderModelMethodsExtension(item)
	default:
		return ""
	}
}

func renderImports(node *ast.ImportAST) string {
	if node == nil {
		return ""
	}
	lines := []string{"import ("}
	for _, spec := range node.Specs {
		line := "\t"
		if strings.TrimSpace(spec.Alias) != "" {
			line += spec.Alias + " "
		}
		line += quote(spec.Path)
		lines = append(lines, line)
	}
	lines = append(lines, ")")
	return strings.Join(lines, "\n")
}

func renderRuntimes(node *ast.RuntimesAST) string {
	if node == nil {
		return ""
	}
	lines := []string{"attrs ("}
	for _, spec := range node.Specs {
		lines = append(lines, "\t"+quote(spec.Identifier()))
	}
	lines = append(lines, ")")
	return strings.Join(lines, "\n")
}

func renderModel(model *ast.ModelAST) string {
	if model == nil {
		return ""
	}

	lines := make([]string, 0)
	for _, attr := range model.Attrs {
		lines = append(lines, renderAttr(attr))
	}
	lines = append(lines, "model "+model.Name+" {")
	for _, field := range model.Fields {
		lines = append(lines, indent(renderField(field)))
	}
	for _, method := range model.Methods {
		lines = append(lines, indent(renderMethod(method)))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func renderTypeAlias(item *ast.TypeAliasAST) string {
	if item == nil {
		return ""
	}

	return "type " + item.Name + " " + renderTypeRef(item.Type)
}

func renderField(field *ast.FieldAST) string {
	if field == nil {
		return ""
	}

	header := field.Name + " " + renderTypeRef(field.Type)
	if field.Default != nil {
		header += " = " + renderExpr(field.Default)
	}

	if len(field.Attrs) == 0 && len(field.Methods) == 0 {
		return header
	}
	if len(field.Attrs) > 0 && !field.AttrsBlock {
		attrs := make([]string, 0, len(field.Attrs))
		for _, attr := range field.Attrs {
			attrs = append(attrs, renderAttr(attr))
		}
		header += " " + strings.Join(attrs, " ")
	}

	lines := []string{header}
	if len(field.Attrs) > 0 && field.AttrsBlock {
		lines = append(lines, "{")
		for _, attr := range field.Attrs {
			lines = append(lines, indent(renderAttr(attr)))
		}
		lines = append(lines, "}")
	}
	if len(field.Methods) > 0 {
		lines = append(lines, "(")
		for _, method := range field.Methods {
			lines = append(lines, indent(renderMethod(method)))
		}
		lines = append(lines, ")")
	}

	return strings.Join(lines, "\n")
}

func renderFieldExtension(node *ast.FieldExtensionAST) string {
	if node == nil {
		return ""
	}
	lines := []string{"field " + node.Model.String() + "." + node.Field.String() + " {"}
	for _, attr := range node.Attrs {
		lines = append(lines, indent(renderAttr(attr)))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func renderFieldMethodsExtension(node *ast.FieldMethodsExtensionAST) string {
	if node == nil {
		return ""
	}
	lines := []string{"func " + node.Model.String() + "." + node.Field.String() + " {"}
	for _, method := range node.Methods {
		lines = append(lines, indent(renderMethod(method)))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func renderModelMethodsExtension(node *ast.ModelMethodsExtensionAST) string {
	if node == nil {
		return ""
	}
	lines := []string{"func " + node.Model.String() + " {"}
	for _, method := range node.Methods {
		lines = append(lines, indent(renderMethod(method)))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func renderMethod(method ast.FieldMethodAST) string {
	line := "func " + method.Name
	if len(method.Params) > 0 {
		params := make([]string, 0, len(method.Params))
		for _, param := range method.Params {
			params = append(params, param.Name+" "+renderTypeRef(param.Type))
		}
		line += " (" + strings.Join(params, ", ") + ")"
	}
	line += " return (" + renderTypeRefs(method.Returns) + ")"
	for _, attr := range method.Attrs {
		line += " " + renderAttr(attr)
	}
	return line
}

func renderAttr(attr ast.Attr) string {
	text := "@" + attr.Identifier()
	return text + "(" + renderCallArgs(attr.Args, attr.NamedArgs) + ")"
}

func renderCallArgs(args []ast.Expr, named []ast.NamedArg) string {
	parts := make([]string, 0, len(args)+len(named))
	for _, arg := range args {
		parts = append(parts, renderExpr(arg))
	}
	for _, item := range named {
		parts = append(parts, item.Name+": "+renderExpr(item.Value))
	}
	return strings.Join(parts, ", ")
}

func renderTypeRef(typeRef ast.TypeRef) string {
	var builder strings.Builder
	if typeRef.IsList {
		builder.WriteString("[]")
	}
	builder.WriteString(typeRef.Name.String())
	if typeRef.Optional {
		builder.WriteString("?")
	}
	return builder.String()
}

func renderTypeRefs(items []ast.TypeRef) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, renderTypeRef(item))
	}
	return strings.Join(parts, ", ")
}

func renderExpr(expr ast.Expr) string {
	switch value := expr.(type) {
	case ast.StringExpr:
		return quote(value.Value)
	case ast.NumberExpr:
		return value.Value
	case ast.BoolExpr:
		if value.Value {
			return "true"
		}
		return "false"
	case ast.NameExpr:
		return value.Name.String()
	case ast.GoExpr:
		return value.Text
	default:
		return ""
	}
}

func quote(text string) string {
	return `"` + strings.ReplaceAll(text, `"`, `\"`) + `"`
}

func indent(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		lines[i] = "\t" + lines[i]
	}
	return strings.Join(lines, "\n")
}
