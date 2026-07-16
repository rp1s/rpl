package attrs

import rootsdk "rpl/pkg/sdk"

const (
	AttrOriginScopeUnknown        = rootsdk.AttrOriginScopeUnknown
	AttrOriginScopeModel          = rootsdk.AttrOriginScopeModel
	AttrOriginScopeField          = rootsdk.AttrOriginScopeField
	AttrOriginScopeFieldExtension = rootsdk.AttrOriginScopeFieldExtension
	AttrOriginScopeMethod         = rootsdk.AttrOriginScopeMethod
)

const (
	AttrValueTypeAny        = rootsdk.AttrValueTypeAny
	AttrValueTypeBool       = rootsdk.AttrValueTypeBool
	AttrValueTypeNumber     = rootsdk.AttrValueTypeNumber
	AttrValueTypeString     = rootsdk.AttrValueTypeString
	AttrValueTypeName       = rootsdk.AttrValueTypeName
	AttrValueTypeStringLike = rootsdk.AttrValueTypeStringLike
)

type AttrOrigin = rootsdk.AttrOrigin
type DiagnosticLoc = rootsdk.DiagnosticLoc
type ResolvedValue = rootsdk.ResolvedValue
type AttrConflict = rootsdk.AttrConflict
type ResolvedAttr = rootsdk.ResolvedAttr
type Value = rootsdk.Value
type NamedValue = rootsdk.NamedValue
type Attr = rootsdk.Attr
type TypeRef = rootsdk.TypeRef
type MethodParam = rootsdk.MethodParam
type Method = rootsdk.Method
type Field = rootsdk.Field
type Model = rootsdk.Model

type AttrValueType = rootsdk.AttrValueType
type AttrArgSpec = rootsdk.AttrArgSpec
type AttrSnippetSpec = rootsdk.AttrSnippetSpec
type AttrSpec = rootsdk.AttrSpec
type DescribeAttrsResponse = rootsdk.DescribeAttrsResponse

func NormalizeAttrsWithSpecs(items []Attr, specs []AttrSpec) []Attr {
	return rootsdk.NormalizeAttrsWithSpecs(items, specs)
}

func NormalizeModelRuntimeAttrs(model Model, specs []AttrSpec) Model {
	return rootsdk.NormalizeModelRuntimeAttrs(model, specs)
}

func NormalizeModelsRuntimeAttrs(models []Model, specs []AttrSpec) []Model {
	return rootsdk.NormalizeModelsRuntimeAttrs(models, specs)
}

func ResolveAttrs(items []Attr, identifier string) (ResolvedAttr, bool) {
	return rootsdk.ResolveAttrs(items, identifier)
}
