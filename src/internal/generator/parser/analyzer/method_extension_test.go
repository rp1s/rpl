package analyzer_test

import (
	"testing"

	"rpl/internal/generator/parser"
	"rpl/internal/generator/parser/lexer"
)

func TestFinalizeFileAcceptsTopLevelFieldMethodExtension(t *testing.T) {
	code := `model User {
    Name string
}

func User.Name {
    func Ping return (User.Name)
}`

	lex := lexer.NewLexerWithPath(code, "schema.rpl")
	if err := lex.Run(); err != nil {
		t.Fatalf("lex: %v", err)
	}

	file, err := parser.New(lex).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := parser.FinalizeFile(file); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	model, ok := file.FindModel("User")
	if !ok || model == nil {
		t.Fatal("model User not found")
	}
	field, ok := model.FindField("Name")
	if !ok || field == nil {
		t.Fatal("field Name not found")
	}
	if len(field.Methods) != 1 || field.Methods[0].Name != "Ping" {
		t.Fatalf("unexpected methods: %+v", field.Methods)
	}
}

func TestFinalizeFileAcceptsTopLevelModelMethodExtension(t *testing.T) {
	code := `model User {
    Name string
}

func User {
    func String return (string)
}`

	lex := lexer.NewLexerWithPath(code, "schema.rpl")
	if err := lex.Run(); err != nil {
		t.Fatalf("lex: %v", err)
	}

	file, err := parser.New(lex).Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := parser.FinalizeFile(file); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	model, ok := file.FindModel("User")
	if !ok || model == nil {
		t.Fatal("model User not found")
	}
	if len(model.Methods) != 1 || model.Methods[0].Name != "String" {
		t.Fatalf("unexpected model methods: %+v", model.Methods)
	}
}
