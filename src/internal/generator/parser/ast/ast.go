package ast

import "rpl/internal/generator/parser/lexer/token"

type File struct {
	ASTs []AST
}

type AST interface {
	astNode()
}

type PackageAST struct {
	Position token.Position
	Name     string
}

func (*PackageAST) astNode() {}

type TargetAST struct {
	Position  token.Position
	Args      []Expr
	NamedArgs []NamedArg
}

func (*TargetAST) astNode() {}

type ModelAST struct {
	Position      token.Position
	Name          string
	Attrs         []Attr
	Fields        []*FieldAST
	Methods       []FieldMethodAST
	GeneratedFrom string
	GroupName     string
}

func (*ModelAST) astNode() {}

type TypeAliasAST struct {
	Position token.Position
	Name     string
	Type     TypeRef
}

func (*TypeAliasAST) astNode() {}

type FieldAST struct {
	Position token.Position
	Name     string
	Type     TypeRef
	Default  Expr
	Attrs    []Attr
	Methods  []FieldMethodAST
}

type FieldExtensionAST struct {
	Position token.Position
	Model    Name
	Field    Name
	Attrs    []Attr
}

func (*FieldExtensionAST) astNode() {}

type FieldMethodsExtensionAST struct {
	Position token.Position
	Model    Name
	Field    Name
	Methods  []FieldMethodAST
}

func (*FieldMethodsExtensionAST) astNode() {}

type ModelMethodsExtensionAST struct {
	Position token.Position
	Model    Name
	Methods  []FieldMethodAST
}

func (*ModelMethodsExtensionAST) astNode() {}

type TypeRef struct {
	Name     Name
	IsList   bool
	Optional bool
}

type FieldMethodAST struct {
	Position token.Position
	Name     string
	Params   []FieldMethodParamAST
	Returns  []TypeRef
	Attrs    []Attr
}

type FieldMethodParamAST struct {
	Position token.Position
	Name     string
	Type     TypeRef
}

type NamedArg struct {
	Position token.Position
	Name     string
	Value    Expr
	Origin   string
}

type Attr struct {
	Position  token.Position
	Package   Name
	Name      Name
	Args      []Expr
	NamedArgs []NamedArg
	Origin    string
}

type Name struct {
	Parts []string
}

func (*FieldAST) sectionItemNode() {}

type Expr interface {
	exprNode()
}

type RuntimesAST struct {
	Position token.Position
	Specs    []RuntimeSpec
}

func (*RuntimesAST) astNode() {}

type RuntimeSpec struct {
	Name   string
	Author string
}

type ModelsGroup struct {
	Attr   string
	Models []*ModelAST
}

type ImportAST struct {
	Position token.Position
	Specs    []ImportSpec
}

func (*ImportAST) astNode() {}

type ImportSpec struct {
	Position token.Position
	Alias    string
	Path     string
}

type FieldAttrs struct {
	Field *FieldAST
	Attrs []Attr
}
