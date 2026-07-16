package ast

import "rpl/internal/generator/parser/lexer/token"

type StringExpr struct {
	Position token.Position
	Value    string
}

func (StringExpr) exprNode() {}

type NumberExpr struct {
	Position token.Position
	Value    string
}

func (NumberExpr) exprNode() {}

type BoolExpr struct {
	Position token.Position
	Value    bool
}

func (BoolExpr) exprNode() {}

type NameExpr struct {
	Position token.Position
	Name     Name
}

func (NameExpr) exprNode() {}

type GoExpr struct {
	Position token.Position
	Text     string
}

func (GoExpr) exprNode() {}
