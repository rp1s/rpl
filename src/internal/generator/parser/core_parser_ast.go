package parser

import (
	"fmt"
	"path/filepath"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer/token"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (parser *Parser) parseAST() (ast.AST, *Err.Error) {
	switch parser.current().Type {
	case token.IMPORT:
		return parser.parseImportAST()
	case token.PACKAGE:
		return parser.parsePackageAST()
	case token.RUNTIMES:
		return parser.parseRuntimesAST()
	case token.TARGET:
		return parser.parseTargetAST()
	case token.FIELD:
		return parser.parseFieldExtensionAST()
	case token.TYPE:
		return parser.parseTypeAliasAST()
	case token.IDENT:
		if strings.EqualFold(parser.current().Text, "func") {
			return parser.parseMethodsExtensionAST()
		}
		return nil, parser.newError(parser.current(), fmt.Sprintf(localize.Text("неожиданный токен: %s", "unexpected token %s"), describeToken(parser.current())))
	case token.DOG, token.MODEL:
		return parser.parseModelAST()
	default:
		tok := parser.current()
		return nil, parser.newError(tok, fmt.Sprintf(localize.Text("неожиданный токен: %s", "unexpected token %s"), describeToken(tok)))
	}
}

func (parser *Parser) parsePackageAST() (ast.AST, *Err.Error) {
	start, err := parser.expect(token.PACKAGE)
	if err != nil {
		return nil, err
	}

	nameTok, err := parser.expect(token.IDENT)
	if err != nil {
		return nil, err
	}

	return &ast.PackageAST{
		Position: start.Position,
		Name:     nameTok.Text,
	}, nil
}

func (parser *Parser) parseTypeAliasAST() (ast.AST, *Err.Error) {
	start, err := parser.expect(token.TYPE)
	if err != nil {
		return nil, err
	}

	nameTok, err := parser.expect(token.IDENT)
	if err != nil {
		return nil, err
	}

	typeRef, err := parser.parseTypeRef()
	if err != nil {
		return nil, err
	}

	return &ast.TypeAliasAST{
		Position: start.Position,
		Name:     nameTok.Text,
		Type:     typeRef,
	}, nil
}

func (parser *Parser) parseMethodsExtensionAST() (ast.AST, *Err.Error) {
	start, err := parser.expectIdentText("func")
	if err != nil {
		return nil, err
	}

	path, err := parser.parseName()
	if err != nil {
		return nil, err
	}

	if len(path.Parts) == 0 {
		return nil, parser.newError(start, localize.Text("ожидалось имя модели или Model.Field", "expected model name or Model.Field")).
			WithHint(localize.Text("Пример: `func User { func String return (string) }`.", "Example: `func User { func String return (string) }`."))
	}

	if _, err := parser.expect(token.LBRACE); err != nil {
		return nil, err
	}

	methods := make([]ast.FieldMethodAST, 0)
	for parser.current().Type != token.RBRACE {
		method, err := parser.parseFieldMethod()
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}

	if _, err := parser.expect(token.RBRACE); err != nil {
		return nil, err
	}

	if len(path.Parts) == 1 {
		return &ast.ModelMethodsExtensionAST{
			Position: start.Position,
			Model:    ast.Name{Parts: append([]string(nil), path.Parts...)},
			Methods:  methods,
		}, nil
	}

	modelName := ast.Name{Parts: append([]string(nil), path.Parts[:len(path.Parts)-1]...)}
	fieldName := ast.Name{Parts: append([]string(nil), path.Parts[len(path.Parts)-1:]...)}

	return &ast.FieldMethodsExtensionAST{
		Position: start.Position,
		Model:    modelName,
		Field:    fieldName,
		Methods:  methods,
	}, nil
}

func (parser *Parser) parseModelAST() (ast.AST, *Err.Error) {
	attrs := make([]ast.Attr, 0)
	for parser.current().Type == token.DOG {
		attr, err := parser.parseAttr()
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attr)
	}

	start, err := parser.expect(token.MODEL)
	if err != nil {
		return nil, err
	}
	nameTok, err := parser.expect(token.IDENT)
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(token.LBRACE); err != nil {
		return nil, err
	}

	fields := make([]*ast.FieldAST, 0)
	methods := make([]ast.FieldMethodAST, 0)
	for parser.current().Type != token.RBRACE {
		if parser.current().Type == token.IDENT && strings.EqualFold(parser.current().Text, "func") {
			method, err := parser.parseFieldMethod()
			if err != nil {
				return nil, err
			}
			methods = append(methods, method)
			continue
		}

		field, err := parser.parseFieldAST()
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}

	if _, err := parser.expect(token.RBRACE); err != nil {
		return nil, err
	}

	return &ast.ModelAST{
		Position: start.Position,
		Name:     nameTok.Text,
		Attrs:    attrs,
		Fields:   fields,
		Methods:  methods,
	}, nil
}

func (parser *Parser) parseTargetAST() (ast.AST, *Err.Error) {
	start, err := parser.expect(token.TARGET)
	if err != nil {
		return nil, err
	}

	if _, err := parser.expect(token.LPAREN); err != nil {
		return nil, err
	}

	args, namedArgs, err := parser.parseCallArgs(token.RPAREN)
	if err != nil {
		return nil, err
	}

	if _, err := parser.expect(token.RPAREN); err != nil {
		return nil, err
	}

	return &ast.TargetAST{
		Position:  start.Position,
		Args:      args,
		NamedArgs: namedArgs,
	}, nil
}

func (parser *Parser) parseFieldExtensionAST() (ast.AST, *Err.Error) {
	start, err := parser.expect(token.FIELD)
	if err != nil {
		return nil, err
	}

	path, err := parser.parseName()
	if err != nil {
		return nil, err
	}

	if len(path.Parts) < 2 {
		return nil, parser.newError(start, localize.Text("ожидалось имя в формате Model.Field", "expected name in Model.Field format")).
			WithHint(localize.Text("Пример: `field User.Name { @sql(index: true) }`.", "Example: `field User.Name { @sql(index: true) }`."))
	}

	modelName := ast.Name{Parts: append([]string(nil), path.Parts[:len(path.Parts)-1]...)}
	fieldName := ast.Name{Parts: append([]string(nil), path.Parts[len(path.Parts)-1:]...)}

	if _, err := parser.expect(token.LBRACE); err != nil {
		return nil, err
	}

	attrs := make([]ast.Attr, 0)
	for parser.current().Type != token.RBRACE {
		attr, err := parser.parseAttr()
		if err != nil {
			return nil, err
		}

		attrs = append(attrs, attr)
	}

	if _, err := parser.expect(token.RBRACE); err != nil {
		return nil, err
	}

	return &ast.FieldExtensionAST{
		Position: start.Position,
		Model:    modelName,
		Field:    fieldName,
		Attrs:    attrs,
	}, nil
}

func (parser *Parser) parseImportAST() (ast.AST, *Err.Error) {
	start, err := parser.expect(token.IMPORT)
	if err != nil {
		return nil, err
	}

	if _, err := parser.expect(token.LPAREN); err != nil {
		return nil, err
	}

	specs := make([]ast.ImportSpec, 0)
	for parser.current().Type != token.RPAREN {
		spec, err := parser.parseImportSpec()
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)

		if parser.match(token.COMMA) {
			continue
		}
	}

	if _, err := parser.expect(token.RPAREN); err != nil {
		return nil, err
	}

	return &ast.ImportAST{
		Position: start.Position,
		Specs:    specs,
	}, nil
}

func (parser *Parser) parseRuntimesAST() (ast.AST, *Err.Error) {
	start, err := parser.expect(token.RUNTIMES)
	if err != nil {
		return nil, err
	}

	if _, err := parser.expect(token.LPAREN); err != nil {
		return nil, err
	}

	specs := make([]ast.RuntimeSpec, 0)
	for parser.current().Type != token.RPAREN {
		spec, err := parser.parseRuntimeSpec()
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)

		if parser.match(token.COMMA) {
			continue
		}
	}

	if _, err := parser.expect(token.RPAREN); err != nil {
		return nil, err
	}

	return &ast.RuntimesAST{
		Position: start.Position,
		Specs:    specs,
	}, nil
}

func (parser *Parser) parseImportSpec() (ast.ImportSpec, *Err.Error) {
	start := parser.current()
	alias := ""

	if parser.current().Type == token.IDENT && parser.peek(1).Type == token.STRING {
		aliasTok := parser.advance()
		alias = aliasTok.Text
		start = aliasTok
	}

	pathTok, err := parser.expect(token.STRING)
	if err != nil {
		return ast.ImportSpec{}, err
	}

	if alias == "" {
		alias = filepath.Base(pathTok.Text)
	}

	return ast.ImportSpec{
		Position: start.Position,
		Alias:    alias,
		Path:     pathTok.Text,
	}, nil
}

func (parser *Parser) parseRuntimeSpec() (ast.RuntimeSpec, *Err.Error) {
	if parser.current().Type == token.STRING {
		nameTok := parser.advance()
		return parseRuntimeSpecValue(nameTok.Text), nil
	}

	name, err := parser.parseName()
	if err != nil {
		return ast.RuntimeSpec{}, err
	}

	return parseRuntimeSpecValue(name.String()), nil
}

func parseRuntimeSpecValue(value string) ast.RuntimeSpec {
	value = strings.TrimSpace(value)
	author, name, ok := strings.Cut(value, ":")
	if !ok {
		return ast.RuntimeSpec{Name: value}
	}

	author = strings.TrimSpace(author)
	name = strings.TrimSpace(name)
	if author == "" || name == "" {
		return ast.RuntimeSpec{Name: value}
	}

	return ast.RuntimeSpec{
		Name:   name,
		Author: author,
	}
}
