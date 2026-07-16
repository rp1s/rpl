package parser

import (
	goparser "go/parser"
	"strconv"
	"strings"
	"unicode"

	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer/token"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
)

func (parser *Parser) parseFieldDefaultExpr() (ast.Expr, *Err.Error) {
	start := parser.current()
	if start.Type == token.EOF {
		return nil, parser.newUnexpectedExpressionError(start)
	}

	switch start.Type {
	case token.RBRACE, token.DOG:
		return nil, parser.newUnexpectedExpressionError(start)
	}

	endOffset, err := parser.findFieldDefaultExprEnd()
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(parser.Lexer.Input[start.Position.Offset:endOffset])
	if raw == "" {
		return nil, parser.newUnexpectedExpressionError(start)
	}

	if _, parseErr := goparser.ParseExpr(raw); parseErr != nil {
		return nil, parser.newError(start, localize.Text("некорректное Go-выражение в default", "invalid Go expression in default")).
			WithDetail(parseErr.Error()).
			WithHint(localize.Text("После `=` можно писать любое валидное Go-выражение.", "After `=` you can write any valid Go expression."))
	}

	parser.advanceToOffset(endOffset)
	return compactFieldDefaultExpr(raw, start.Position), nil
}

func (parser *Parser) findFieldDefaultExprEnd() (int, *Err.Error) {
	if parser == nil || parser.Lexer == nil {
		return 0, nil
	}

	startOffset := parser.current().Position.Offset
	bestEnd := -1
	braceDepth := 0
	bracketDepth := 0
	parenDepth := 0

scan:
	for idx := parser.Position; idx < len(parser.Lexer.Tokens); idx++ {
		tok := parser.Lexer.Tokens[idx]
		if tok.Type == token.EOF {
			bestEnd = bestFieldDefaultEnd(bestEnd, parser.fieldDefaultCandidateEnd(startOffset, len(parser.Lexer.Input)))
			break
		}

		if idx > parser.Position && braceDepth == 0 && bracketDepth == 0 && parenDepth == 0 {
			prev := parser.Lexer.Tokens[idx-1]
			if isFieldDefaultHardBoundary(prev, tok) {
				bestEnd = bestFieldDefaultEnd(bestEnd, parser.fieldDefaultCandidateEnd(startOffset, tok.Position.Offset))
				break
			}
		}

		switch tok.Type {
		case token.LBRACE:
			braceDepth++
		case token.RBRACE:
			if braceDepth == 0 {
				bestEnd = bestFieldDefaultEnd(bestEnd, parser.fieldDefaultCandidateEnd(startOffset, tok.Position.Offset))
				break scan
			}
			braceDepth--
		case token.LBRACKET:
			bracketDepth++
		case token.RBRACKET:
			if bracketDepth > 0 {
				bracketDepth--
			}
		case token.LPAREN:
			parenDepth++
		case token.RPAREN:
			if parenDepth > 0 {
				parenDepth--
			}
		}

		bestEnd = bestFieldDefaultEnd(bestEnd, parser.fieldDefaultCandidateEnd(startOffset, parser.nextTokenOffset(idx)))
	}

	if bestEnd >= 0 {
		return bestEnd, nil
	}

	rawEnd := parser.findFieldDefaultBoundaryEnd()
	raw := strings.TrimSpace(parser.Lexer.Input[startOffset:rawEnd])
	if raw == "" {
		return 0, parser.newUnexpectedExpressionError(parser.current())
	}

	if _, parseErr := goparser.ParseExpr(raw); parseErr != nil {
		return 0, parser.newError(parser.current(), localize.Text("некорректное Go-выражение в default", "invalid Go expression in default")).
			WithDetail(parseErr.Error()).
			WithHint(localize.Text("После `=` можно писать любое валидное Go-выражение.", "After `=` you can write any valid Go expression."))
	}

	return 0, parser.newUnexpectedExpressionError(parser.current())
}

func bestFieldDefaultEnd(current int, candidate int) int {
	if candidate > current {
		return candidate
	}
	return current
}

func (parser *Parser) fieldDefaultCandidateEnd(startOffset int, endOffset int) int {
	if parser == nil || parser.Lexer == nil || endOffset <= startOffset {
		return -1
	}

	raw := strings.TrimSpace(parser.Lexer.Input[startOffset:endOffset])
	if raw == "" {
		return -1
	}
	if _, err := goparser.ParseExpr(raw); err != nil {
		return -1
	}
	return endOffset
}

func (parser *Parser) nextTokenOffset(index int) int {
	if parser == nil || parser.Lexer == nil {
		return 0
	}
	next := index + 1
	if next >= len(parser.Lexer.Tokens) {
		return len(parser.Lexer.Input)
	}
	return parser.Lexer.Tokens[next].Position.Offset
}

func (parser *Parser) findFieldDefaultBoundaryEnd() int {
	if parser == nil || parser.Lexer == nil {
		return 0
	}

	startOffset := parser.current().Position.Offset
	braceDepth := 0
	bracketDepth := 0
	parenDepth := 0

	for idx := parser.Position; idx < len(parser.Lexer.Tokens); idx++ {
		tok := parser.Lexer.Tokens[idx]
		if tok.Type == token.EOF {
			return len(parser.Lexer.Input)
		}

		if idx > parser.Position && braceDepth == 0 && bracketDepth == 0 && parenDepth == 0 {
			prev := parser.Lexer.Tokens[idx-1]
			if isFieldDefaultHardBoundary(prev, tok) {
				return tok.Position.Offset
			}
		}

		switch tok.Type {
		case token.LBRACE:
			braceDepth++
		case token.RBRACE:
			if braceDepth == 0 {
				return tok.Position.Offset
			}
			braceDepth--
		case token.LBRACKET:
			bracketDepth++
		case token.RBRACKET:
			if bracketDepth > 0 {
				bracketDepth--
			}
		case token.LPAREN:
			parenDepth++
		case token.RPAREN:
			if parenDepth > 0 {
				parenDepth--
			}
		}
	}

	if startOffset < len(parser.Lexer.Input) {
		return len(parser.Lexer.Input)
	}
	return startOffset
}

func (parser *Parser) advanceToOffset(offset int) {
	for parser.current().Type != token.EOF && parser.current().Position.Offset < offset {
		parser.advance()
	}
}

func isFieldDefaultHardBoundary(prev token.Token, current token.Token) bool {
	if current.Type == token.DOG || current.Type == token.RBRACE {
		return true
	}
	if current.Position.Line <= prev.Position.Line {
		return false
	}
	return !canContinueFieldDefaultAfterNewline(prev, current)
}

func canContinueFieldDefaultAfterNewline(prev token.Token, current token.Token) bool {
	if prev.Type == token.SYMBOL || current.Type == token.SYMBOL {
		return true
	}

	switch prev.Type {
	case token.COMMA, token.COLON, token.DOT, token.LPAREN, token.LBRACKET, token.LBRACE:
		return true
	}

	switch current.Type {
	case token.COMMA, token.COLON, token.DOT, token.RPAREN, token.RBRACKET, token.RBRACE:
		return true
	}

	return false
}

func compactFieldDefaultExpr(raw string, position token.Position) ast.Expr {
	if value, ok := parseQuotedStringDefault(raw, position); ok {
		return value
	}
	if raw == "true" || raw == "false" {
		return ast.BoolExpr{Position: position, Value: raw == "true"}
	}
	if isSimpleNumericDefault(raw) {
		return ast.NumberExpr{Position: position, Value: raw}
	}
	if isSimpleNameDefault(raw) {
		return ast.NameExpr{
			Position: position,
			Name:     ast.Name{Parts: strings.Split(raw, ".")},
		}
	}
	return ast.GoExpr{Position: position, Text: raw}
}

func parseQuotedStringDefault(raw string, position token.Position) (ast.Expr, bool) {
	if len(raw) < 2 {
		return nil, false
	}
	if raw[0] != '"' && raw[0] != '`' {
		return nil, false
	}

	value, err := strconv.Unquote(raw)
	if err != nil {
		return nil, false
	}
	return ast.StringExpr{Position: position, Value: value}, true
}

func isSimpleNumericDefault(raw string) bool {
	if raw == "" {
		return false
	}
	dotCount := 0
	for _, char := range raw {
		if char == '.' {
			dotCount++
			if dotCount > 1 {
				return false
			}
			continue
		}
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}

func isSimpleNameDefault(raw string) bool {
	if raw == "" {
		return false
	}
	parts := strings.Split(raw, ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		for index, char := range part {
			if index == 0 {
				if char != '_' && !unicode.IsLetter(char) {
					return false
				}
				continue
			}
			if char != '_' && !unicode.IsLetter(char) && !unicode.IsDigit(char) {
				return false
			}
		}
	}
	return true
}
