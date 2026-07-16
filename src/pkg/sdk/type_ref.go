package sdk

import "strings"

type TypeKind string

const (
	TypeKindUnknown TypeKind = "unknown"
	TypeKindString  TypeKind = "string"
	TypeKindBool    TypeKind = "bool"
	TypeKindInteger TypeKind = "integer"
	TypeKindFloat   TypeKind = "float"
	TypeKindTime    TypeKind = "time"
	TypeKindError   TypeKind = "error"
	TypeKindBytes   TypeKind = "bytes"
	TypeKindList    TypeKind = "list"
	TypeKindNamed   TypeKind = "named"
)

func (typeRef TypeRef) Kind() TypeKind {
	switch {
	case typeRef.IsBytes():
		return TypeKindBytes
	case typeRef.IsList:
		return TypeKindList
	case typeRef.IsString():
		return TypeKindString
	case typeRef.IsBool():
		return TypeKindBool
	case typeRef.IsInteger():
		return TypeKindInteger
	case typeRef.IsFloat():
		return TypeKindFloat
	case typeRef.IsTime():
		return TypeKindTime
	case typeRef.IsError():
		return TypeKindError
	case strings.TrimSpace(typeRef.Name) != "":
		return TypeKindNamed
	default:
		return TypeKindUnknown
	}
}

func (typeRef TypeRef) ElementType() TypeRef {
	if !typeRef.IsList {
		return typeRef
	}

	typeRef.IsList = false
	return typeRef
}

func (typeRef TypeRef) NullableBase() TypeRef {
	typeRef.Optional = false
	return typeRef
}

func (typeRef TypeRef) RefModel(ctx FileContext) (*Model, bool) {
	return ctx.FindModel(typeRef.BaseName())
}

func (typeRef TypeRef) IsModel(ctx FileContext) bool {
	_, ok := typeRef.RefModel(ctx)
	return ok
}

func (typeRef TypeRef) IsExternal(ctx FileContext) bool {
	if strings.TrimSpace(typeRef.Name) == "" {
		return false
	}
	if typeRef.IsScalar() || typeRef.IsError() || typeRef.IsModel(ctx) {
		return false
	}

	return strings.Contains(strings.TrimSpace(typeRef.Name), ".")
}

func (field Field) RefModel(ctx FileContext) (*Model, bool) {
	return field.Type.RefModel(ctx)
}
