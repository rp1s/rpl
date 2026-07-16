package sdk

type RuntimeRef struct {
	Name   string `json:"name"`
	Author string `json:"author"`
}

type FileContext struct {
	SourcePath    string        `json:"source_path,omitempty"`
	ProjectRoot   string        `json:"project_root,omitempty"`
	TargetLang    string        `json:"target_lang,omitempty"`
	OutputDir     string        `json:"output_dir,omitempty"`
	PackageName   string        `json:"package_name,omitempty"`
	GoPackagePath string        `json:"go_package_path,omitempty"`
	ModelFileStem string        `json:"model_file_stem,omitempty"`
	Imports       []ImportRef   `json:"imports,omitempty"`
	Runtimes      []RuntimeRef  `json:"runtimes,omitempty"`
	Models        []string      `json:"models,omitempty"`
	RuntimeModels []string      `json:"runtime_models,omitempty"`
	AllModels     []Model       `json:"all_models,omitempty"`
	Syntax        *SyntaxFile   `json:"syntax,omitempty"`
	Resolved      *ResolvedFile `json:"resolved,omitempty"`
}

type ImportRef struct {
	Alias string `json:"alias,omitempty"`
	Path  string `json:"path"`
}

type Value struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
	Bool bool   `json:"bool,omitempty"`
}

type NamedValue struct {
	Name   string     `json:"name"`
	Value  Value      `json:"value"`
	Origin AttrOrigin `json:"origin,omitempty"`
}

type Attr struct {
	Package    string       `json:"package,omitempty"`
	Name       string       `json:"name,omitempty"`
	Identifier string       `json:"identifier,omitempty"`
	Args       []Value      `json:"args,omitempty"`
	NamedArgs  []NamedValue `json:"named_args,omitempty"`
	Origin     AttrOrigin   `json:"origin,omitempty"`
}

type TypeRef struct {
	Name     string `json:"name"`
	IsList   bool   `json:"is_list,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

type MethodParam struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}

type Method struct {
	Name         string        `json:"name"`
	Params       []MethodParam `json:"params,omitempty"`
	Returns      []TypeRef     `json:"returns,omitempty"`
	Attrs        []Attr        `json:"attrs,omitempty"`
	RuntimeAttrs []Attr        `json:"runtime_attrs,omitempty"`
}

type Field struct {
	Name         string   `json:"name"`
	Type         TypeRef  `json:"type"`
	Attrs        []Attr   `json:"attrs,omitempty"`
	RuntimeAttrs []Attr   `json:"runtime_attrs,omitempty"`
	Methods      []Method `json:"methods,omitempty"`
}

type Model struct {
	Name         string   `json:"name"`
	Attrs        []Attr   `json:"attrs,omitempty"`
	RuntimeAttrs []Attr   `json:"runtime_attrs,omitempty"`
	Fields       []Field  `json:"fields,omitempty"`
	Methods      []Method `json:"methods,omitempty"`
}

type GenerateRequest struct {
	File    FileContext `json:"file"`
	Runtime RuntimeRef  `json:"runtime"`
	Model   Model       `json:"model"`
}

type CodeBlock struct {
	Name  string `json:"name,omitempty"`
	Code  string `json:"code"`
	Order int    `json:"order,omitempty"`
}

type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Delete  bool   `json:"delete,omitempty"`
}

type GenerateResponse struct {
	Imports []ImportRef     `json:"imports,omitempty"`
	Blocks  []CodeBlock     `json:"blocks,omitempty"`
	Files   []GeneratedFile `json:"files,omitempty"`
}

type CodeBuilder struct {
	imports map[string]ImportRef
	blocks  []CodeBlock
	files   map[string]GeneratedFile
}
