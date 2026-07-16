package catalog

import "strings"

// TypeSpec describes one type suggestion for a specific target language.
type TypeSpec struct {
	Name     string   `json:"name"`
	Insert   string   `json:"insert,omitempty"`
	Category string   `json:"category,omitempty"`
	Help     string   `json:"help,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
}

// StructureSpec describes a larger schema snippet such as a model or alias
// template that can be suggested by editor tooling.
type StructureSpec struct {
	Label    string `json:"label,omitempty"`
	Insert   string `json:"insert,omitempty"`
	Category string `json:"category,omitempty"`
	Help     string `json:"help,omitempty"`
}

// Catalog groups the type and structure suggestions available for one target
// language.
type Catalog struct {
	Lang       string          `json:"lang"`
	Label      string          `json:"label,omitempty"`
	Help       string          `json:"help,omitempty"`
	Types      []TypeSpec      `json:"types,omitempty"`
	Structures []StructureSpec `json:"structures,omitempty"`
}

// DescribeResponse is the JSON payload returned by type-catalog endpoints used
// by editors and other tooling.
type DescribeResponse struct {
	Catalog Catalog `json:"catalog"`
}

func (catalog Catalog) Normalized() Catalog {
	catalog.Lang = strings.TrimSpace(catalog.Lang)
	catalog.Label = strings.TrimSpace(catalog.Label)
	catalog.Help = strings.TrimSpace(catalog.Help)
	catalog.Types = normalizeTypes(catalog.Types)
	catalog.Structures = normalizeStructures(catalog.Structures)
	return catalog
}

func normalizeTypes(items []TypeSpec) []TypeSpec {
	if len(items) == 0 {
		return nil
	}

	normalized := make([]TypeSpec, 0, len(items))
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		item.Insert = strings.TrimSpace(item.Insert)
		item.Category = strings.TrimSpace(item.Category)
		item.Help = strings.TrimSpace(item.Help)
		item.Aliases = normalizeStringList(item.Aliases)
		if item.Name == "" && item.Insert == "" {
			continue
		}
		if item.Insert == "" {
			item.Insert = item.Name
		}
		if item.Name == "" {
			item.Name = item.Insert
		}
		normalized = append(normalized, item)
	}

	return normalized
}

func normalizeStructures(items []StructureSpec) []StructureSpec {
	if len(items) == 0 {
		return nil
	}

	normalized := make([]StructureSpec, 0, len(items))
	for _, item := range items {
		item.Label = strings.TrimSpace(item.Label)
		item.Insert = strings.TrimSpace(item.Insert)
		item.Category = strings.TrimSpace(item.Category)
		item.Help = strings.TrimSpace(item.Help)
		if item.Label == "" && item.Insert == "" {
			continue
		}
		if item.Label == "" {
			item.Label = item.Insert
		}
		normalized = append(normalized, item)
	}

	return normalized
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	return normalized
}
