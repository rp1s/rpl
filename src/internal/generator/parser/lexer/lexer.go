package lexer

import (
	"errors"
	"rpl/internal/generator/parser/lexer/token"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"unicode"
	"unicode/utf8"
)

type Lexer struct {
	Input    string
	FilePath string
	Tokens   []token.Token
	Position token.Position
}

func (lexer *Lexer) advance() {
	if lexer.Position.Offset >= len(lexer.Input) {
		return
	}

	char, size := utf8.DecodeRuneInString(lexer.Input[lexer.Position.Offset:])
	if char == '\n' {
		lexer.Position.Line++
		lexer.Position.Column = 1
	} else {
		lexer.Position.Column++
	}
	lexer.Position.Offset += size
}

func (lexer *Lexer) currentRune() (rune, int) {
	if lexer.Position.Offset >= len(lexer.Input) {
		return utf8.RuneError, 0
	}

	return utf8.DecodeRuneInString(lexer.Input[lexer.Position.Offset:])
}

func (lexer *Lexer) peekRune() (rune, int) {
	offset := lexer.Position.Offset
	if offset >= len(lexer.Input) {
		return utf8.RuneError, 0
	}

	_, size := utf8.DecodeRuneInString(lexer.Input[offset:])
	offset += size
	if offset >= len(lexer.Input) {
		return utf8.RuneError, 0
	}

	return utf8.DecodeRuneInString(lexer.Input[offset:])
}

func NewLexer(input string) *Lexer {
	return NewLexerWithPath(input, "")
}

func NewLexerWithPath(input string, filePath string) *Lexer {
	return &Lexer{
		Input:    input,
		FilePath: filePath,
		Tokens:   make([]token.Token, 0),
		Position: token.Position{
			Line:   1,
			Column: 1,
			Offset: 0,
			File:   filePath,
		},
	}
}

func (lexer *Lexer) Run() error {
	var errs []error

	for {
		tok, err := lexer.next()
		if err != nil {
			errs = append(errs, error(err))
			continue
		}
		lexer.Tokens = append(lexer.Tokens, tok)
		if tok.Type == token.EOF {
			break
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (lexer *Lexer) next() (token.Token, *Err.Error) {
	lexer.skipIgnored()

	if lexer.Position.Offset >= len(lexer.Input) {
		return token.Token{
			Type:     token.EOF,
			Text:     "",
			Position: lexer.Position,
		}, nil
	}

	CurrentPosition := lexer.Position
	Char, _ := lexer.currentRune()

	switch Char {
	case '{':
		lexer.advance()
		return token.Token{Type: token.LBRACE, Text: "{", Position: CurrentPosition}, nil
	case '}':
		lexer.advance()
		return token.Token{Type: token.RBRACE, Text: "}", Position: CurrentPosition}, nil
	case '@':
		lexer.advance()
		return token.Token{Type: token.DOG, Text: "@", Position: CurrentPosition}, nil
	case '.':
		lexer.advance()
		return token.Token{Type: token.DOT, Text: ".", Position: CurrentPosition}, nil
	case ',':
		lexer.advance()
		return token.Token{Type: token.COMMA, Text: ",", Position: CurrentPosition}, nil
	case ':':
		lexer.advance()
		return token.Token{Type: token.COLON, Text: ":", Position: CurrentPosition}, nil
	case '=':
		lexer.advance()
		return token.Token{Type: token.ASSIGN, Text: "=", Position: CurrentPosition}, nil
	case '?':
		lexer.advance()
		return token.Token{Type: token.QUESTION, Text: "?", Position: CurrentPosition}, nil
	case '(':
		lexer.advance()
		return token.Token{Type: token.LPAREN, Text: "(", Position: CurrentPosition}, nil
	case ')':
		lexer.advance()
		return token.Token{Type: token.RPAREN, Text: ")", Position: CurrentPosition}, nil
	case '[':
		lexer.advance()
		return token.Token{Type: token.LBRACKET, Text: "[", Position: CurrentPosition}, nil
	case ']':
		lexer.advance()
		return token.Token{Type: token.RBRACKET, Text: "]", Position: CurrentPosition}, nil
	case '"':
		return lexer.readQuotedString(CurrentPosition, '"')
	case '`':
		return lexer.readQuotedString(CurrentPosition, '`')
	default:
		if isLetter(Char) {
			return lexer.readIdentifier(CurrentPosition)
		}
		if isDigit(Char) {
			return lexer.readNumber(CurrentPosition)
		}
		lexer.advance()
		return token.Token{
			Type:     token.SYMBOL,
			Text:     string(Char),
			Position: CurrentPosition,
		}, nil
	}
}

func (lexer *Lexer) skipIgnored() {
	for lexer.Position.Offset < len(lexer.Input) {
		Char, size := lexer.currentRune()
		if isWhitespace(Char) {
			lexer.advance()
			continue
		}
		if Char == '/' && lexer.Position.Offset+size < len(lexer.Input) && lexer.Input[lexer.Position.Offset+size] == '/' {
			lexer.skipLineComment()
			continue
		}
		break
	}
}

func (lexer *Lexer) skipLineComment() {
	for lexer.Position.Offset < len(lexer.Input) {
		char, _ := lexer.currentRune()
		if char == '\n' {
			return
		}
		lexer.advance()
	}
}

func (lexer *Lexer) readQuotedString(CurrentPosition token.Position, quote rune) (token.Token, *Err.Error) {
	lexer.advance()
	start := lexer.Position.Offset
	for lexer.Position.Offset < len(lexer.Input) {
		char, _ := lexer.currentRune()
		if char == quote {
			break
		}
		if quote != '`' && char == '\\' {
			lexer.advance()
			if lexer.Position.Offset >= len(lexer.Input) {
				break
			}
		}
		lexer.advance()
	}
	if lexer.Position.Offset >= len(lexer.Input) {
		hint := localize.Text("Строка должна заканчиваться закрывающей кавычкой.", "The string literal must end with a closing quote.")
		if quote == '`' {
			hint = localize.Text("Raw-строка должна заканчиваться закрывающей обратной кавычкой '`'.", "A raw string must end with a closing backtick '`'.")
		}
		return token.Token{}, lexer.newError(CurrentPosition, localize.Text("незавершённый строковый литерал", "unterminated string literal")).
			WithHint(hint)
	}
	text := lexer.Input[start:lexer.Position.Offset]
	lexer.advance()
	return token.Token{
		Type:     token.STRING,
		Text:     text,
		Position: CurrentPosition,
	}, nil
}

func (lexer *Lexer) readIdentifier(CurrentPosition token.Position) (token.Token, *Err.Error) {
	start := lexer.Position.Offset
	for lexer.Position.Offset < len(lexer.Input) {
		char, _ := lexer.currentRune()
		if !isLetter(char) && !isDigit(char) {
			break
		}
		lexer.advance()
	}
	text := lexer.Input[start:lexer.Position.Offset]

	var tokType token.TokenType
	switch text {
	case "import":
		tokType = token.IMPORT
	case "package":
		tokType = token.PACKAGE
	case "attrs":
		tokType = token.RUNTIMES
	case "target":
		tokType = token.TARGET
	case "field":
		tokType = token.FIELD
	case "model":
		tokType = token.MODEL
	case "type":
		tokType = token.TYPE
	default:
		tokType = token.IDENT
	}

	return token.Token{
		Type:     tokType,
		Text:     text,
		Position: CurrentPosition,
	}, nil
}

func (lexer *Lexer) readNumber(CurrentPosition token.Position) (token.Token, *Err.Error) {
	start := lexer.Position.Offset
	hasDot := false
	for lexer.Position.Offset < len(lexer.Input) {
		char, _ := lexer.currentRune()
		if isDigit(char) {
			lexer.advance()
			continue
		}

		if char == '.' && !hasDot {
			next, _ := lexer.peekRune()
			if isDigit(next) {
				hasDot = true
				lexer.advance()
				continue
			}
		}

		break
	}
	text := lexer.Input[start:lexer.Position.Offset]
	return token.Token{
		Type:     token.NUMBER,
		Text:     text,
		Position: CurrentPosition,
	}, nil
}

func (lexer *Lexer) newError(position token.Position, message string) *Err.Error {
	return Err.New(message).
		WithLocation(position.File, position.Line, position.Column).
		WithSource(lexer.Input).
		WithKind("syntax")
}

func (lexer *Lexer) unexpectedCharHint(char rune) string {
	switch char {
	case '\'':
		return localize.Text("В DSL строки пишутся в двойных кавычках, например `\"users\"`.", "Strings in this DSL use double quotes, for example `\"users\"`.")
	case ';':
		return localize.Text("Точки с запятой тут не нужны: просто переходите на новую строку.", "Semicolons are not needed here; just move to the next line.")
	case '#':
		return localize.Text("Комментарии начинаются с `//`, а не с `#`.", "Comments start with `//`, not `#`.")
	default:
		return localize.Text("Проверьте синтаксис рядом с этим символом: возможно, здесь опечатка или лишний знак.", "Check the syntax near this character; it may be a typo or an extra symbol.")
	}
}

func isWhitespace(char rune) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}

func isLetter(char rune) bool {
	return char == '_' || unicode.IsLetter(char)
}

func isDigit(char rune) bool {
	return unicode.IsDigit(char)
}
