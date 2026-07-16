package sdk

import "strings"

type ASTLevel string

const (
	ASTLevelSyntax   ASTLevel = "syntax"
	ASTLevelResolved ASTLevel = "resolved"
)

type SyntaxFile struct {
	Target          SyntaxTarget           `json:"target,omitempty"`
	Imports         []ImportRef            `json:"imports,omitempty"`
	Runtimes        []RuntimeRef           `json:"runtimes,omitempty"`
	Models          []SyntaxModel          `json:"models,omitempty"`
	FieldExtensions []SyntaxFieldExtension `json:"field_extensions,omitempty"`
	Nodes           []SyntaxNode           `json:"nodes,omitempty"`
}

type ResolvedFile struct {
	TargetLang string       `json:"target_lang,omitempty"`
	Imports    []ImportRef  `json:"imports,omitempty"`
	Runtimes   []RuntimeRef `json:"runtimes,omitempty"`
	Models     []Model      `json:"models,omitempty"`
}

type SyntaxNode struct {
	ID       string     `json:"id,omitempty"`
	Kind     string     `json:"kind,omitempty"`
	Name     string     `json:"name,omitempty"`
	Model    string     `json:"model,omitempty"`
	Field    string     `json:"field,omitempty"`
	Type     TypeRef    `json:"type,omitempty"`
	Default  *Value     `json:"default,omitempty"`
	Attrs    []Attr     `json:"attrs,omitempty"`
	Methods  []Method   `json:"methods,omitempty"`
	Origin   AttrOrigin `json:"origin,omitempty"`
	Level    ASTLevel   `json:"level,omitempty"`
	ParentID string     `json:"parent_id,omitempty"`
}

type SyntaxTarget struct {
	Args      []Value      `json:"args,omitempty"`
	NamedArgs []NamedValue `json:"named_args,omitempty"`
	Origin    AttrOrigin   `json:"origin,omitempty"`
}

type SyntaxModel struct {
	Name          string        `json:"name,omitempty"`
	Attrs         []Attr        `json:"attrs,omitempty"`
	Fields        []SyntaxField `json:"fields,omitempty"`
	Methods       []Method      `json:"methods,omitempty"`
	GeneratedFrom string        `json:"generated_from,omitempty"`
	GroupName     string        `json:"group_name,omitempty"`
	Origin        AttrOrigin    `json:"origin,omitempty"`
}

type SyntaxField struct {
	Name    string     `json:"name,omitempty"`
	Type    TypeRef    `json:"type,omitempty"`
	Default *Value     `json:"default,omitempty"`
	Attrs   []Attr     `json:"attrs,omitempty"`
	Methods []Method   `json:"methods,omitempty"`
	Origin  AttrOrigin `json:"origin,omitempty"`
}

type SyntaxFieldExtension struct {
	Model  string     `json:"model,omitempty"`
	Field  string     `json:"field,omitempty"`
	Attrs  []Attr     `json:"attrs,omitempty"`
	Origin AttrOrigin `json:"origin,omitempty"`
}

func (ctx FileContext) FindSyntaxModel(name string) (*SyntaxModel, bool) {
	if ctx.Syntax == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}
	for i := range ctx.Syntax.Models {
		if strings.TrimSpace(ctx.Syntax.Models[i].Name) == name {
			return &ctx.Syntax.Models[i], true
		}
	}
	return nil, false
}

func (ctx FileContext) FindResolvedModel(name string) (*Model, bool) {
	if ctx.Resolved == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}
	for i := range ctx.Resolved.Models {
		if strings.TrimSpace(ctx.Resolved.Models[i].Name) == name {
			return &ctx.Resolved.Models[i], true
		}
	}
	return nil, false
}

func (ctx FileContext) WalkSyntaxNodes(fn func(SyntaxNode) bool) {
	if ctx.Syntax == nil || fn == nil {
		return
	}
	for _, node := range ctx.Syntax.Nodes {
		if !fn(node) {
			return
		}
	}
}
