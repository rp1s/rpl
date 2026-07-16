package ast

import "strings"

func (typeRef TypeRef) BaseName() string {
	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" {
		return ""
	}

	parts := strings.Split(name, ".")
	return parts[len(parts)-1]
}

func (typeRef TypeRef) IsString() bool {
	return typeRef.BaseName() == "string"
}

func (typeRef TypeRef) IsBool() bool {
	return typeRef.BaseName() == "bool"
}

func (typeRef TypeRef) IsInteger() bool {
	switch typeRef.BaseName() {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "byte":
		return true
	default:
		return false
	}
}

func (typeRef TypeRef) IsFloat() bool {
	switch typeRef.BaseName() {
	case "float32", "float64":
		return true
	default:
		return false
	}
}

func (typeRef TypeRef) IsNumeric() bool {
	return typeRef.IsInteger() || typeRef.IsFloat()
}

func (typeRef TypeRef) IsError() bool {
	return !typeRef.IsList && !typeRef.Optional && typeRef.BaseName() == "error"
}

func (typeRef TypeRef) IsByte() bool {
	switch typeRef.BaseName() {
	case "byte", "uint8":
		return true
	default:
		return false
	}
}

func (typeRef TypeRef) IsBytes() bool {
	return typeRef.IsList && typeRef.IsByte()
}
