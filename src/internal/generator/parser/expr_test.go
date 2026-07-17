package parser

import (
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer"
	"testing"
)

func TestParseExprKeepsQuotedBooleansAsStrings(t *testing.T) {
	lex := lexer.NewLexer(`model Feature {
	Enabled bool @cache(default: "true", strict: true)
}`)
	if err := lex.Run(); err != nil {
		t.Fatalf("lex: %v", err)
	}
	file, err := New(lex).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	model, ok := file.FindModel("Feature")
	if !ok || len(model.Fields) != 1 || len(model.Fields[0].Attrs) != 1 {
		t.Fatalf("unexpected parsed model: %#v", model)
	}
	attr := model.Fields[0].Attrs[0]
	defaultValue, ok := attr.NamedArg("default")
	if !ok {
		t.Fatal("default argument not found")
	}
	if value, ok := defaultValue.(ast.StringExpr); !ok || value.Value != "true" {
		t.Fatalf("quoted true parsed as %#v, want StringExpr", defaultValue)
	}
	strictValue, ok := attr.NamedArg("strict")
	if !ok {
		t.Fatal("strict argument not found")
	}
	if value, ok := strictValue.(ast.BoolExpr); !ok || !value.Value {
		t.Fatalf("unquoted true parsed as %#v, want BoolExpr(true)", strictValue)
	}
}

func TestParseExprAcceptsSignedNumbers(t *testing.T) {
	lex := lexer.NewLexer(`target(lang: golang)
attrs ("rpl:mongodb")
model Product {
    Price float64 @mongodb(indexGroup: "price", indexOrder: -1)
}`)
	if err := lex.Run(); err != nil {
		t.Fatalf("lex: %v", err)
	}
	file, err := New(lex).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	model, ok := file.FindModel("Product")
	if !ok || len(model.Fields) != 1 || len(model.Fields[0].Attrs) != 1 {
		t.Fatalf("unexpected parsed model: %#v", model)
	}
	value, ok := model.Fields[0].Attrs[0].NamedArg("indexOrder")
	if !ok {
		t.Fatal("indexOrder argument not found")
	}
	number, ok := value.(ast.NumberExpr)
	if !ok || number.Value != "-1" {
		t.Fatalf("indexOrder = %#v, want NumberExpr(-1)", value)
	}
}
