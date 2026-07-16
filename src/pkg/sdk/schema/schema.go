package schema

import rootsdk "rpl/pkg/sdk"

const (
	TypeKindUnknown = rootsdk.TypeKindUnknown
	TypeKindString  = rootsdk.TypeKindString
	TypeKindBool    = rootsdk.TypeKindBool
	TypeKindInteger = rootsdk.TypeKindInteger
	TypeKindFloat   = rootsdk.TypeKindFloat
	TypeKindTime    = rootsdk.TypeKindTime
	TypeKindError   = rootsdk.TypeKindError
	TypeKindBytes   = rootsdk.TypeKindBytes
	TypeKindList    = rootsdk.TypeKindList
	TypeKindNamed   = rootsdk.TypeKindNamed
)

type TypeKind = rootsdk.TypeKind

type RuntimeRef = rootsdk.RuntimeRef
type FileContext = rootsdk.FileContext
type ImportRef = rootsdk.ImportRef
type Value = rootsdk.Value
type NamedValue = rootsdk.NamedValue
type Attr = rootsdk.Attr
type TypeRef = rootsdk.TypeRef
type MethodParam = rootsdk.MethodParam
type Method = rootsdk.Method
type Field = rootsdk.Field
type Model = rootsdk.Model

type ASTLevel = rootsdk.ASTLevel
type SyntaxFile = rootsdk.SyntaxFile
type ResolvedFile = rootsdk.ResolvedFile
type SyntaxNode = rootsdk.SyntaxNode
type SyntaxTarget = rootsdk.SyntaxTarget
type SyntaxModel = rootsdk.SyntaxModel
type SyntaxField = rootsdk.SyntaxField
type SyntaxFieldExtension = rootsdk.SyntaxFieldExtension

func CollectRuntimeValues(attrs []Attr, runtimeName string) map[string]Value {
	return rootsdk.CollectRuntimeValues(attrs, runtimeName)
}

func NormalizeImportRef(item ImportRef) ImportRef {
	return rootsdk.NormalizeImportRef(item)
}

func Quote(text string) string {
	return rootsdk.Quote(text)
}

func Indent(text string, prefix string) string {
	return rootsdk.Indent(text, prefix)
}

func JoinNonEmpty(parts ...string) string {
	return rootsdk.JoinNonEmpty(parts...)
}

func SnakeCase(name string) string {
	return rootsdk.SnakeCase(name)
}

func LowerCamel(name string) string {
	return rootsdk.LowerCamel(name)
}
