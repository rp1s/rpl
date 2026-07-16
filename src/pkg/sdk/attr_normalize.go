package sdk

import "strings"

func NormalizeAttrsWithSpecs(attrs []Attr, specs []AttrSpec) []Attr {
	if len(attrs) == 0 || len(specs) == 0 {
		return attrs
	}

	normalized := make([]Attr, 0, len(attrs))
	for _, attr := range attrs {
		current := attr
		for _, spec := range specs {
			current = normalizeAttrWithSpec(current, spec)
		}
		normalized = append(normalized, current)
	}

	return normalized
}

func NormalizeModelRuntimeAttrs(model Model, specs []AttrSpec) Model {
	if len(specs) == 0 {
		return model
	}

	model.RuntimeAttrs = NormalizeAttrsWithSpecs(model.RuntimeAttrs, specs)
	if len(model.Methods) > 0 {
		methods := make([]Method, 0, len(model.Methods))
		for _, method := range model.Methods {
			method.RuntimeAttrs = NormalizeAttrsWithSpecs(method.RuntimeAttrs, specs)
			methods = append(methods, method)
		}
		model.Methods = methods
	}
	if len(model.Fields) == 0 {
		return model
	}

	fields := make([]Field, 0, len(model.Fields))
	for _, field := range model.Fields {
		field.RuntimeAttrs = NormalizeAttrsWithSpecs(field.RuntimeAttrs, specs)
		if len(field.Methods) > 0 {
			methods := make([]Method, 0, len(field.Methods))
			for _, method := range field.Methods {
				method.RuntimeAttrs = NormalizeAttrsWithSpecs(method.RuntimeAttrs, specs)
				methods = append(methods, method)
			}
			field.Methods = methods
		}
		fields = append(fields, field)
	}

	model.Fields = fields
	return model
}

func NormalizeModelsRuntimeAttrs(models []Model, specs []AttrSpec) []Model {
	if len(models) == 0 || len(specs) == 0 {
		return models
	}

	normalized := make([]Model, 0, len(models))
	for _, model := range models {
		normalized = append(normalized, NormalizeModelRuntimeAttrs(model, specs))
	}

	return normalized
}

func normalizeAttrWithSpec(attr Attr, spec AttrSpec) Attr {
	if strings.TrimSpace(spec.Namespace) == "" || len(attr.Args) == 0 {
		return attr
	}
	if !attr.Matches(spec.Namespace) || strings.TrimSpace(attr.SubName()) != "" {
		return attr
	}
	if spec.HasPositionalArg() {
		return attr
	}

	generated := make([]NamedValue, 0, len(attr.Args))
	remainingArgs := make([]Value, 0, len(attr.Args))
	for _, arg := range attr.Args {
		name, ok := spec.ShorthandBoolArgName(arg)
		if !ok {
			remainingArgs = append(remainingArgs, arg)
			continue
		}

		generated = append(generated, NamedValue{
			Name:   name,
			Value:  Value{Kind: "bool", Bool: true},
			Origin: attr.Origin,
		})
	}

	if len(generated) == 0 {
		return attr
	}

	normalized := attr
	normalized.Args = remainingArgs
	normalized.NamedArgs = append(generated, append([]NamedValue(nil), attr.NamedArgs...)...)
	return normalized
}
