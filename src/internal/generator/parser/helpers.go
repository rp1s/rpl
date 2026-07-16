package parser

import (
	"fmt"
	"rpl/internal/generator/parser/lexer/token"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
)

func (parser *Parser) current() token.Token {
	if len(parser.Lexer.Tokens) == 0 {
		return token.Token{Type: token.EOF}
	}
	if parser.Position >= len(parser.Lexer.Tokens) {
		return parser.Lexer.Tokens[len(parser.Lexer.Tokens)-1]
	}
	return parser.Lexer.Tokens[parser.Position]
}

func (parser *Parser) peek(offset int) token.Token {
	if len(parser.Lexer.Tokens) == 0 {
		return token.Token{Type: token.EOF}
	}
	idx := parser.Position + offset
	if idx >= len(parser.Lexer.Tokens) {
		return parser.Lexer.Tokens[len(parser.Lexer.Tokens)-1]
	}
	return parser.Lexer.Tokens[idx]
}

func (parser *Parser) advance() token.Token {
	tok := parser.current()
	if parser.Position < len(parser.Lexer.Tokens) {
		parser.move()
	}
	return tok
}

func (parser *Parser) match(tt token.TokenType) bool {
	if parser.current().Type != tt {
		return false
	}
	parser.move()
	return true
}

func (parser *Parser) expect(tt token.TokenType) (token.Token, *Err.Error) {
	tok := parser.current()
	if tok.Type != tt {
		return token.Token{}, parser.newExpectedError(tok, tt)
	}
	parser.move()
	return tok, nil
}

func (parser *Parser) newError(tok token.Token, message string) *Err.Error {
	return Err.New(message).
		WithLocation(tok.Position.File, tok.Position.Line, tok.Position.Column).
		WithSource(parser.Lexer.Input).
		WithKind("parse")
}

func (parser *Parser) newExpectedError(tok token.Token, expected token.TokenType) *Err.Error {
	return parser.newError(
		tok,
		fmt.Sprintf(localize.Text("ожидалось %s, получено %s", "expected %s, got %s"), token.DescribeType(expected), describeToken(tok)),
	).WithHint(expectedTokenHint(expected))
}

func describeToken(tok token.Token) string {
	if tok.Text == "" || tok.Text == string(tok.Type) {
		return token.DescribeType(tok.Type)
	}

	return fmt.Sprintf("%s (%q)", token.DescribeType(tok.Type), tok.Text)
}

func expectedTokenHint(expected token.TokenType) string {
	switch expected {
	case token.RPAREN:
		return localize.Text("Похоже, здесь не хватает закрывающей `)`.", "It looks like a closing `)` is missing here.")
	case token.RBRACE:
		return localize.Text("Проверьте, закрыт ли текущий блок фигурной скобкой `}`.", "Check whether the current block is closed with `}`.")
	case token.LBRACE:
		return localize.Text("После объявления блока обычно идёт `{`.", "A block declaration is usually followed by `{`.")
	case token.IDENT:
		return localize.Text("Здесь ожидается имя, например `User`, `Name` или `time.Time`.", "An identifier is expected here, for example `User`, `Name`, or `time.Time`.")
	case token.STRING:
		return localize.Text("Строковые значения пишутся в двойных кавычках, например `\"users\"`.", "String values use double quotes, for example `\"users\"`.")
	case token.COLON:
		return localize.Text("Для именованных аргументов нужен синтаксис `name: value`.", "Named arguments use the `name: value` syntax.")
	default:
		return localize.Text("Проверьте токены рядом: возможно, пропущен разделитель или скобка.", "Check the nearby tokens; a separator or bracket may be missing.")
	}
}

func (parser *Parser) move() {
	if parser.Position < len(parser.Lexer.Tokens)-1 {
		parser.Position++
	} else {
		parser.Position = len(parser.Lexer.Tokens)
	}
}
