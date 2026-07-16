package analysis

import (
	"io"
	rootsdk "rpl/pkg/sdk"
)

type Diagnostic = rootsdk.Diagnostic
type Claim = rootsdk.Claim
type AnalyzeResponse = rootsdk.AnalyzeResponse
type AnalyzeBuilder = rootsdk.AnalyzeBuilder
type DiagnosticError = rootsdk.DiagnosticError
type DiagnosticLoc = rootsdk.DiagnosticLoc
type Attr = rootsdk.Attr
type Value = rootsdk.Value
type TypeRef = rootsdk.TypeRef
type AttrValueType = rootsdk.AttrValueType
type AttrConflict = rootsdk.AttrConflict
type GenerateResponse = rootsdk.GenerateResponse

func NewAnalyzeBuilder() *AnalyzeBuilder {
	return rootsdk.NewAnalyzeBuilder()
}

func DiagnosticAt(node DiagnosticLoc, message string, hint ...string) Diagnostic {
	return rootsdk.DiagnosticAt(node, message, hint...)
}

func UnknownAttrName(attr Attr, namespace string, name string, allowed []string, help string) Diagnostic {
	return rootsdk.UnknownAttrName(attr, namespace, name, allowed, help)
}

func UnknownArg(node DiagnosticLoc, namespace string, name string, allowed []string, help string) Diagnostic {
	return rootsdk.UnknownArg(node, namespace, name, allowed, help)
}

func WrongArgType(node DiagnosticLoc, namespace string, name string, value Value, expected []AttrValueType, help string) Diagnostic {
	return rootsdk.WrongArgType(node, namespace, name, value, expected, help)
}

func IncompatibleAttrType(node DiagnosticLoc, namespace string, name string, typeRef TypeRef, hint string) Diagnostic {
	return rootsdk.IncompatibleAttrType(node, namespace, name, typeRef, hint)
}

func ConflictingArgs(namespace string, conflict AttrConflict) Diagnostic {
	return rootsdk.ConflictingArgs(namespace, conflict)
}

func AddGeneratedClaims(builder *AnalyzeBuilder, response GenerateResponse) {
	rootsdk.AddGeneratedClaims(builder, response)
}

func AddGeneratedClaimsInScope(builder *AnalyzeBuilder, response GenerateResponse, scope string) {
	rootsdk.AddGeneratedClaimsInScope(builder, response, scope)
}

func GoTopLevelDeclNames(code string) []string {
	return rootsdk.GoTopLevelDeclNames(code)
}

func NewError(text string) *DiagnosticError {
	return rootsdk.NewError(text)
}

func NewErrorf(format string, args ...any) *DiagnosticError {
	return rootsdk.NewErrorf(format, args...)
}

func PrintError(writer io.Writer, err error) {
	rootsdk.PrintError(writer, err)
}

func Text(primary string, fallback string) string {
	return rootsdk.Text(primary, fallback)
}
