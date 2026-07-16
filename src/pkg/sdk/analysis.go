package sdk

import "strings"

type Diagnostic struct {
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

type Claim struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Scope string `json:"scope,omitempty"`
}

type AnalyzeResponse struct {
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Claims      []Claim      `json:"claims,omitempty"`
}

type AnalyzeBuilder struct {
	diagnostics []Diagnostic
	claims      map[string]Claim
}

func NewAnalyzeBuilder() *AnalyzeBuilder {
	return &AnalyzeBuilder{
		diagnostics: make([]Diagnostic, 0),
		claims:      make(map[string]Claim),
	}
}

func (builder *AnalyzeBuilder) AddDiagnostic(item Diagnostic) {
	if builder == nil {
		return
	}

	item.Message = strings.TrimSpace(item.Message)
	item.Hint = strings.TrimSpace(item.Hint)
	item.Detail = strings.TrimSpace(item.Detail)
	item.Path = strings.TrimSpace(item.Path)
	if item.Message == "" {
		return
	}

	builder.diagnostics = append(builder.diagnostics, item)
}

func (builder *AnalyzeBuilder) AddClaim(kind string, name string, scope string) {
	if builder == nil {
		return
	}

	item := Claim{
		Kind:  strings.TrimSpace(kind),
		Name:  strings.TrimSpace(name),
		Scope: strings.TrimSpace(scope),
	}
	if item.Kind == "" || item.Name == "" {
		return
	}

	key := item.Kind + "|" + item.Name + "|" + item.Scope
	builder.claims[key] = item
}

func (builder *AnalyzeBuilder) Response() AnalyzeResponse {
	if builder == nil {
		return AnalyzeResponse{}
	}

	claims := make([]Claim, 0, len(builder.claims))
	for _, item := range builder.claims {
		claims = append(claims, item)
	}

	return AnalyzeResponse{
		Diagnostics: append([]Diagnostic(nil), builder.diagnostics...),
		Claims:      claims,
	}
}
