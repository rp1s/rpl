package sdk

import (
	"path/filepath"
	"rpl/pkg/sdk/decls"
	"strings"
)

func AddGeneratedClaims(builder *AnalyzeBuilder, response GenerateResponse) {
	AddGeneratedClaimsInScope(builder, response, "")
}

func AddGeneratedClaimsInScope(builder *AnalyzeBuilder, response GenerateResponse, scope string) {
	if builder == nil {
		return
	}

	for _, file := range response.Files {
		if strings.TrimSpace(file.Path) == "" {
			continue
		}
		builder.AddClaim("file", file.Path, scope)
		if file.Delete || strings.ToLower(filepath.Ext(file.Path)) != ".go" {
			continue
		}
		for _, name := range decls.GoTopLevelNames(file.Content) {
			builder.AddClaim("identifier", name, scope)
		}
	}

	parts := make([]string, 0, len(response.Blocks))
	for _, block := range response.Blocks {
		code := strings.TrimSpace(block.Code)
		if code == "" {
			continue
		}
		parts = append(parts, code)
	}
	for _, name := range decls.GoTopLevelNames(strings.Join(parts, "\n\n")) {
		builder.AddClaim("identifier", name, scope)
	}
}

func GoTopLevelDeclNames(code string) []string {
	return decls.GoTopLevelNames(code)
}
