package ast

import "testing"

func TestResolveFieldExtensionKeepsInlineFieldAttrPriority(t *testing.T) {
	file := &File{
		ASTs: []AST{
			&ModelAST{
				Name: "User",
				Fields: []*FieldAST{
					{
						Name: "Name",
						Attrs: []Attr{
							{
								Package: Name{Parts: []string{"sql"}},
								NamedArgs: []NamedArg{
									{Name: "index", Value: BoolExpr{Value: true}},
									{Name: "unique", Value: BoolExpr{Value: true}},
								},
							},
						},
					},
				},
			},
			&FieldExtensionAST{
				Model: Name{Parts: []string{"User"}},
				Field: Name{Parts: []string{"Name"}},
				Attrs: []Attr{
					{
						Package: Name{Parts: []string{"sql"}},
						NamedArgs: []NamedArg{
							{Name: "index", Value: BoolExpr{Value: false}},
							{Name: "comment", Value: StringExpr{Value: "extra"}},
						},
					},
				},
			},
		},
	}

	if err := file.Resolve(); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	model, ok := file.FindModel("User")
	if !ok {
		t.Fatal("model User not found after resolve")
	}

	field, ok := model.FindField("Name")
	if !ok {
		t.Fatal("field Name not found after resolve")
	}

	attr, ok := field.FindAttr("sql")
	if !ok {
		t.Fatal("sql attr not found after resolve")
	}

	indexExpr, ok := attr.NamedArg("index")
	if !ok {
		t.Fatal("sql.index not found after resolve")
	}
	if ExprString(indexExpr) != "true" {
		t.Fatalf("expected inline sql.index=true to win, got %q", ExprString(indexExpr))
	}

	uniqueExpr, ok := attr.NamedArg("unique")
	if !ok {
		t.Fatal("sql.unique not found after resolve")
	}
	if ExprString(uniqueExpr) != "true" {
		t.Fatalf("expected sql.unique=true to remain, got %q", ExprString(uniqueExpr))
	}

	commentExpr, ok := attr.NamedArg("comment")
	if !ok {
		t.Fatal("expected field extension to add sql.comment")
	}
	if ExprString(commentExpr) != "extra" {
		t.Fatalf("expected sql.comment=extra, got %q", ExprString(commentExpr))
	}
}
