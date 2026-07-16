package sdk

type AttrCapabilities struct {
	AnalyzeModel  bool `json:"analyze_model,omitempty"`
	AnalyzeFile   bool `json:"analyze_file,omitempty"`
	GenerateModel bool `json:"generate_model,omitempty"`
	GenerateFile  bool `json:"generate_file,omitempty"`
	DocsModel     bool `json:"docs_model,omitempty"`
	DocsFile      bool `json:"docs_file,omitempty"`
	DescribeAttrs bool `json:"describe_attrs,omitempty"`
}

type DescribeCapabilitiesResponse struct {
	Name         string           `json:"name,omitempty"`
	Author       string           `json:"author,omitempty"`
	Capabilities AttrCapabilities `json:"capabilities,omitempty"`
}
