package token

import (
	"fmt"
	"rpl/pkg/error/localize"
)

type TokenType string

type Position struct {
	Line   int
	Column int
	Offset int
	File   string
}

func (position *Position) String() string {
	return localize.Text(
		fmt.Sprintf("строка %d, столбец %d", position.Line, position.Column),
		fmt.Sprintf("line %d, column %d", position.Line, position.Column),
	)
}

type Token struct {
	Type     TokenType
	Text     string
	Position Position
}

func (token *Token) String() string {
	if token.Text == "" {
		return fmt.Sprintf("%s в %s", DescribeType(token.Type), token.Position.String())
	}

	return fmt.Sprintf("%s(%q) в %s", DescribeType(token.Type), token.Text, token.Position.String())
}

func DescribeType(tt TokenType) string {
	switch tt {
	case EOF:
		return localize.Text("конец файла", "end of file")
	case IDENT:
		return localize.Text("идентификатор", "identifier")
	case STRING:
		return localize.Text("строка", "string")
	case NUMBER:
		return localize.Text("число", "number")
	case SYMBOL:
		return localize.Text("символ", "symbol")
	case IMPORT:
		return localize.Text("ключевое слово import", "keyword import")
	case PACKAGE:
		return localize.Text("ключевое слово package", "keyword package")
	case RUNTIMES:
		return localize.Text("ключевое слово attrs", "keyword attrs")
	case TARGET:
		return localize.Text("ключевое слово target", "keyword target")
	case FIELD:
		return localize.Text("ключевое слово field", "keyword field")
	case MODEL:
		return localize.Text("ключевое слово model", "keyword model")
	case TYPE:
		return localize.Text("ключевое слово type", "keyword type")
	default:
		return string(tt)
	}
}

const (
	EOF   TokenType = "EOF"
	IDENT TokenType = "IDENT"

	STRING TokenType = "STRING"
	NUMBER TokenType = "NUMBER"
	SYMBOL TokenType = "SYMBOL"

	DOG      TokenType = "@"
	DOT      TokenType = "."
	COMMA    TokenType = ","
	COLON    TokenType = ":"
	ASSIGN   TokenType = "="
	QUESTION TokenType = "?"

	LBRACE   TokenType = "{"
	RBRACE   TokenType = "}"
	LPAREN   TokenType = "("
	RPAREN   TokenType = ")"
	LBRACKET TokenType = "["
	RBRACKET TokenType = "]"

	IMPORT   TokenType = "IMPORT"
	PACKAGE  TokenType = "PACKAGE"
	RUNTIMES TokenType = "RUNTIMES"
	TARGET   TokenType = "TARGET"
	FIELD    TokenType = "FIELD"
	MODEL    TokenType = "MODEL"
	TYPE     TokenType = "TYPE"
)
