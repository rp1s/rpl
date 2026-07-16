package sdk

import "strings"

func (ctx FileContext) HasModel(name string) bool {
	if ctx.Resolved != nil {
		if _, ok := ctx.FindResolvedModel(name); ok {
			return true
		}
	}
	if ctx.findModelIndex(name) >= 0 {
		return true
	}

	return containsName(ctx.Models, name)
}

func (ctx FileContext) HasRuntimeModel(name string) bool {
	return containsName(ctx.RuntimeModels, name)
}

func (ctx FileContext) FindModel(name string) (*Model, bool) {
	if model, ok := ctx.FindResolvedModel(name); ok {
		return model, true
	}
	index := ctx.findModelIndex(name)
	if index < 0 {
		return nil, false
	}

	return &ctx.AllModels[index], true
}

func (ctx FileContext) findModelIndex(name string) int {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return -1
	}

	for i := range ctx.AllModels {
		if strings.TrimSpace(ctx.AllModels[i].Name) == trimmed {
			return i
		}
	}

	return -1
}

func containsName(items []string, name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}

	for _, item := range items {
		if strings.TrimSpace(item) == trimmed {
			return true
		}
	}

	return false
}
