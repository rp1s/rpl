package parser

import (
	"fmt"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer/token"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
)

func (p *Parser) parseExpr() (ast.Expr, *Err.Error) {
	tok := p.current()

	if tok.Type == token.SYMBOL && (tok.Text == "-" || tok.Text == "+") && p.peek(1).Type == token.NUMBER {
		p.advance()
		number := p.current()
		p.advance()
		return ast.NumberExpr{Position: tok.Position, Value: tok.Text + number.Text}, nil
	}

	if tok.Type == token.IDENT && (tok.Text == "true" || tok.Text == "false") {
		p.advance()
		return ast.BoolExpr{Position: tok.Position, Value: tok.Text == "true"}, nil
	}

	switch tok.Type {
	case token.STRING:
		p.advance()
		return ast.StringExpr{Position: tok.Position, Value: tok.Text}, nil
	case token.NUMBER:
		p.advance()
		return ast.NumberExpr{Position: tok.Position, Value: tok.Text}, nil
	case token.IDENT:
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		return ast.NameExpr{Position: tok.Position, Name: name}, nil
	default:
		return nil, p.newUnexpectedExpressionError(tok)
	}
}

func (parser *Parser) newUnexpectedExpressionError(tok token.Token) *Err.Error {
	return parser.newError(
		tok,
		fmt.Sprintf(localize.Text("неожиданный токен в выражении: %s", "unexpected expression token %s"), describeToken(tok)),
	).WithHint(localize.Text(
		"В выражениях здесь обычно используются строки, числа, `true`/`false` или имена вроде `time.Time`.",
		"Expressions here usually use strings, numbers, `true`/`false`, or names like `time.Time`.",
	))
}

func (parser *Parser) parseName() (ast.Name, *Err.Error) {
	first, err := parser.expect(token.IDENT)
	if err != nil {
		return ast.Name{}, err
	}

	parts := []string{first.Text}
	for parser.match(token.DOT) {
		next, err := parser.expect(token.IDENT)
		if err != nil {
			return ast.Name{}, err
		}
		parts = append(parts, next.Text)
	}

	return ast.Name{Parts: parts}, nil
}
