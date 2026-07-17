package parser

import (
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer/token"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (parser *Parser) parseFieldAST() (*ast.FieldAST, *Err.Error) {
	nameTok, err := parser.expect(token.IDENT)
	if err != nil {
		return nil, err
	}

	typeRef, err := parser.parseTypeRef()
	if err != nil {
		return nil, err
	}

	field := &ast.FieldAST{
		Position: nameTok.Position,
		Name:     nameTok.Text,
		Type:     typeRef,
		Attrs:    make([]ast.Attr, 0),
		Methods:  make([]ast.FieldMethodAST, 0),
	}

	if parser.match(token.ASSIGN) {
		value, err := parser.parseFieldDefaultExpr()
		if err != nil {
			return nil, err
		}
		field.Default = value
	}

	parsedBlock := false
	parsedMethods := false
	for {
		switch parser.current().Type {
		case token.DOG:
			attr, err := parser.parseAttr()
			if err != nil {
				return nil, err
			}
			field.Attrs = append(field.Attrs, attr)
		case token.LBRACE:
			if parsedBlock {
				return nil, parser.newError(
					parser.current(),
					localize.Text("дублирующийся блок атрибутов поля", "duplicate field attribute block"),
				).WithHint(localize.Text("У поля может быть только один `{ ... }` блок атрибутов. Объедините атрибуты в один блок.", "A field can only have one `{ ... }` attribute block. Merge the attributes into a single block."))
			}

			attrs, err := parser.parseFieldAttrBlock()
			if err != nil {
				return nil, err
			}
			field.Attrs = append(field.Attrs, attrs...)
			field.AttrsBlock = true
			parsedBlock = true
		case token.LPAREN:
			if parsedMethods {
				return nil, parser.newError(
					parser.current(),
					localize.Text("дублирующийся блок методов поля", "duplicate field methods block"),
				).WithHint(localize.Text("У поля может быть только один `( ... )` блок методов. Объедините описания методов в один блок.", "A field can only have one `( ... )` methods block. Merge the methods into a single block."))
			}

			methods, err := parser.parseFieldMethodBlock()
			if err != nil {
				return nil, err
			}
			field.Methods = append(field.Methods, methods...)
			parsedMethods = true
		default:
			return field, nil
		}
	}
}

func (parser *Parser) parseTypeRef() (ast.TypeRef, *Err.Error) {
	typeRef := ast.TypeRef{}

	if parser.match(token.LBRACKET) {
		if _, err := parser.expect(token.RBRACKET); err != nil {
			return ast.TypeRef{}, err
		}
		typeRef.IsList = true
	}

	name, err := parser.parseName()
	if err != nil {
		return ast.TypeRef{}, err
	}
	typeRef.Name = name
	typeRef.Optional = parser.match(token.QUESTION)

	return typeRef, nil
}

func (parser *Parser) parseAttr() (ast.Attr, *Err.Error) {
	start, err := parser.expect(token.DOG)
	if err != nil {
		return ast.Attr{}, err
	}
	fullName, err := parser.parseName()
	if err != nil {
		return ast.Attr{}, err
	}

	packageName, attrName := splitAttrName(fullName)
	packageName, attrName = normalizeAttrNames(packageName, attrName)

	attr := ast.Attr{
		Position:  start.Position,
		Package:   packageName,
		Name:      attrName,
		Args:      make([]ast.Expr, 0),
		NamedArgs: make([]ast.NamedArg, 0),
	}

	if !parser.match(token.LPAREN) {
		return attr, nil
	}

	args, namedArgs, err := parser.parseCallArgs(token.RPAREN)
	if err != nil {
		return ast.Attr{}, err
	}
	attr.Args = args
	attr.NamedArgs = namedArgs

	if _, err := parser.expect(token.RPAREN); err != nil {
		return ast.Attr{}, err
	}

	return attr, nil
}

func (parser *Parser) parseFieldAttrBlock() ([]ast.Attr, *Err.Error) {
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

	return attrs, nil
}

func (parser *Parser) parseFieldMethodBlock() ([]ast.FieldMethodAST, *Err.Error) {
	if _, err := parser.expect(token.LPAREN); err != nil {
		return nil, err
	}

	methods := make([]ast.FieldMethodAST, 0)
	for parser.current().Type != token.RPAREN {
		method, err := parser.parseFieldMethod()
		if err != nil {
			return nil, err
		}

		methods = append(methods, method)
	}

	if _, err := parser.expect(token.RPAREN); err != nil {
		return nil, err
	}

	return methods, nil
}

// parseFieldMethod reads a DSL method declaration attached to a field.
// We keep the grammar intentionally narrow: only signatures and attrs live
// here, no executable body. That makes the generated bridge predictable.
func (parser *Parser) parseFieldMethod() (ast.FieldMethodAST, *Err.Error) {
	start, err := parser.expectIdentText("func")
	if err != nil {
		return ast.FieldMethodAST{}, err
	}

	nameTok, err := parser.expect(token.IDENT)
	if err != nil {
		return ast.FieldMethodAST{}, err
	}

	params := make([]ast.FieldMethodParamAST, 0)
	if parser.current().Type == token.LPAREN {
		params, err = parser.parseFieldMethodParams()
		if err != nil {
			return ast.FieldMethodAST{}, err
		}
	}

	if _, err := parser.expectOneOf("return", "returns"); err != nil {
		return ast.FieldMethodAST{}, err
	}

	returns, err := parser.parseFieldMethodReturns()
	if err != nil {
		return ast.FieldMethodAST{}, err
	}

	attrs := make([]ast.Attr, 0)
	for parser.current().Type == token.DOG {
		attr, err := parser.parseAttr()
		if err != nil {
			return ast.FieldMethodAST{}, err
		}
		attrs = append(attrs, attr)
	}

	return ast.FieldMethodAST{
		Position: start.Position,
		Name:     nameTok.Text,
		Params:   params,
		Returns:  returns,
		Attrs:    attrs,
	}, nil
}

func (parser *Parser) parseFieldMethodParams() ([]ast.FieldMethodParamAST, *Err.Error) {
	if _, err := parser.expect(token.LPAREN); err != nil {
		return nil, err
	}

	params := make([]ast.FieldMethodParamAST, 0)
	for parser.current().Type != token.RPAREN {
		nameTok, err := parser.expect(token.IDENT)
		if err != nil {
			return nil, err
		}

		typeRef, err := parser.parseTypeRef()
		if err != nil {
			return nil, err
		}

		params = append(params, ast.FieldMethodParamAST{
			Position: nameTok.Position,
			Name:     nameTok.Text,
			Type:     typeRef,
		})

		if parser.match(token.COMMA) {
			continue
		}
	}

	if _, err := parser.expect(token.RPAREN); err != nil {
		return nil, err
	}

	return params, nil
}

func (parser *Parser) parseFieldMethodReturns() ([]ast.TypeRef, *Err.Error) {
	if _, err := parser.expect(token.LPAREN); err != nil {
		return nil, err
	}

	returns := make([]ast.TypeRef, 0)
	for parser.current().Type != token.RPAREN {
		typeRef, err := parser.parseTypeRef()
		if err != nil {
			return nil, err
		}

		returns = append(returns, typeRef)

		if parser.match(token.COMMA) {
			continue
		}
	}

	if _, err := parser.expect(token.RPAREN); err != nil {
		return nil, err
	}

	return returns, nil
}

func splitAttrName(fullName ast.Name) (ast.Name, ast.Name) {
	if len(fullName.Parts) == 0 {
		return ast.Name{}, ast.Name{}
	}

	if len(fullName.Parts) == 1 {
		name := cloneName(fullName)
		return name, cloneName(name)
	}

	return cloneName(ast.Name{Parts: fullName.Parts[:len(fullName.Parts)-1]}), cloneName(ast.Name{Parts: []string{fullName.Parts[len(fullName.Parts)-1]}})
}

func normalizeAttrNames(packageName, attrName ast.Name) (ast.Name, ast.Name) {
	switch {
	case len(packageName.Parts) == 0 && len(attrName.Parts) > 0:
		packageName = cloneName(attrName)
	case len(attrName.Parts) == 0 && len(packageName.Parts) > 0:
		attrName = cloneName(packageName)
	}

	return cloneName(packageName), cloneName(attrName)
}

func (parser *Parser) expectIdentText(value string) (token.Token, *Err.Error) {
	tok, err := parser.expect(token.IDENT)
	if err != nil {
		return token.Token{}, err
	}
	if tok.Text != value {
		return token.Token{}, parser.newError(tok, fmtKeywordMismatch(value, tok.Text)).
			WithHint(localize.Text("Проверьте ключевые слова вокруг сигнатуры метода поля.", "Check the field method signature keywords around this position."))
	}

	return tok, nil
}

func (parser *Parser) expectOneOf(values ...string) (token.Token, *Err.Error) {
	tok, err := parser.expect(token.IDENT)
	if err != nil {
		return token.Token{}, err
	}

	for _, value := range values {
		if tok.Text == value {
			return tok, nil
		}
	}

	return token.Token{}, parser.newError(tok, fmtKeywordMismatch(strings.Join(values, " or "), tok.Text)).
		WithHint(localize.Text("Здесь ожидается блок возвращаемых типов, например `return (string)`.", "A return type block is expected here, for example `return (string)`."))
}

func fmtKeywordMismatch(expected string, actual string) string {
	return localize.Text(
		"ожидалось ключевое слово "+expected+", получено "+actual,
		"expected keyword "+expected+", got "+actual,
	)
}

func cloneName(name ast.Name) ast.Name {
	if len(name.Parts) == 0 {
		return ast.Name{}
	}

	parts := make([]string, len(name.Parts))
	copy(parts, name.Parts)

	return ast.Name{Parts: parts}
}

func (parser *Parser) parseCallArgs(end token.TokenType) ([]ast.Expr, []ast.NamedArg, *Err.Error) {
	args := make([]ast.Expr, 0)
	namedArgs := make([]ast.NamedArg, 0)
	for parser.current().Type != end {
		if parser.looksLikeNamedArg() {
			start := parser.current()
			name, err := parser.parseName()
			if err != nil {
				return nil, nil, err
			}

			if _, err := parser.expect(token.COLON); err != nil {
				return nil, nil, err
			}

			value, err := parser.parseExpr()
			if err != nil {
				return nil, nil, err
			}

			namedArgs = append(namedArgs, ast.NamedArg{
				Position: start.Position,
				Name:     name.String(),
				Value:    value,
			})
		} else {
			expr, err := parser.parseExpr()
			if err != nil {
				return nil, nil, err
			}
			args = append(args, expr)
		}

		if parser.match(token.COMMA) {
			continue
		}
	}

	return args, namedArgs, nil
}

func (parser *Parser) looksLikeNamedArg() bool {
	if parser.current().Type != token.IDENT {
		return false
	}

	offset := 1
	for parser.peek(offset).Type == token.DOT && parser.peek(offset+1).Type == token.IDENT {
		offset += 2
	}

	return parser.peek(offset).Type == token.COLON
}
