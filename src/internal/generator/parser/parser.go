package parser

import (
	"rpl/internal/generator/parser/analyzer"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer"
	"rpl/internal/generator/parser/lexer/token"
	"rpl/internal/generator/parser/validation"
)

type Parser struct {
	Lexer    *lexer.Lexer
	File     *ast.File
	Position int
}

func New(lexer *lexer.Lexer) *Parser {
	return &Parser{
		Lexer:    lexer,
		Position: 0,
		File:     &ast.File{ASTs: make([]ast.AST, 0)},
	}
}

func (parser *Parser) Parse() (*ast.File, error) {
	for parser.current().Type != token.EOF {
		node, err := parser.parseAST()
		if err != nil {
			return nil, err
		}
		parser.File.ASTs = append(parser.File.ASTs, node)
	}

	return parser.File, nil
}

func FinalizeFile(file *ast.File) error {
	if file == nil {
		return nil
	}

	if err := validation.ValidationAST(file); err != nil {
		return err
	}

	if err := analyzer.AnalyzeRaw(file); err != nil {
		return err
	}

	if err := file.Resolve(); err != nil {
		return err
	}

	if err := file.ExpandGroups(); err != nil {
		return err
	}

	return analyzer.Analyze(file)
}

func (parser *Parser) Run() (*ast.File, error) {
	file, err := parser.Parse()
	if err != nil {
		return nil, err
	}

	if err := FinalizeFile(file); err != nil {
		return nil, err
	}

	return file, nil
}
