package sdk

type DocsSection struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Order int    `json:"order,omitempty"`
}

type DocsRequest struct {
	File    FileContext `json:"file"`
	Runtime RuntimeRef  `json:"runtime"`
	Model   Model       `json:"model,omitempty"`
}

type DocsResponse struct {
	Sections []DocsSection `json:"sections,omitempty"`
}
