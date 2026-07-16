package ast

import (
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
)

// Resolve применяет top-level field/func блоки к соответствующим моделям и полям
// и убирает сами extension-узлы из итогового AST.
func (file *File) Resolve() error {
	if file == nil {
		return nil
	}

	models := make(map[string]*ModelAST)
	for _, node := range file.ASTs {
		model, ok := node.(*ModelAST)
		if !ok || model == nil {
			continue
		}

		models[model.Name] = model
	}

	resolved := make([]AST, 0, len(file.ASTs))
	for _, node := range file.ASTs {
		modelMethodExtension, modelMethodOK := node.(*ModelMethodsExtensionAST)
		if modelMethodOK {
			if modelMethodExtension == nil {
				continue
			}

			modelName := modelMethodExtension.Model.String()
			model, exists := models[modelName]
			if !exists {
				return Err.Newf(
					localize.Text("модель %q для func-блока не найдена", "model %q for func block was not found"),
					modelName,
				).
					WithLocation(modelMethodExtension.Position.File, modelMethodExtension.Position.Line, modelMethodExtension.Position.Column).
					WithHint(localize.Text("Сначала объявите модель, а потом добавляйте `func Model { ... }` блоки.", "Declare the model first, then add `func Model { ... }` blocks."))
			}

			model.Methods = mergeFieldMethods(model.Methods, modelMethodExtension.Methods)
			continue
		}

		extension, ok := node.(*FieldExtensionAST)
		if !ok {
			methodExtension, methodOK := node.(*FieldMethodsExtensionAST)
			if !methodOK {
				resolved = append(resolved, node)
				continue
			}
			if methodExtension == nil {
				continue
			}

			modelName := methodExtension.Model.String()
			fieldName := methodExtension.Field.String()

			model, exists := models[modelName]
			if !exists {
				return Err.Newf(
					localize.Text("модель %q для func-блока не найдена", "model %q for func block was not found"),
					modelName,
				).
					WithLocation(methodExtension.Position.File, methodExtension.Position.Line, methodExtension.Position.Column).
					WithHint(localize.Text("Сначала объявите модель, а потом добавляйте `func Model.Field { ... }` блоки.", "Declare the model first, then add `func Model.Field { ... }` blocks."))
			}

			field, exists := model.FindField(fieldName)
			if !exists {
				return Err.Newf(
					localize.Text("поле %q модели %q для func-блока не найдено", "field %q of model %q for func block was not found"),
					fieldName,
					modelName,
				).
					WithLocation(methodExtension.Position.File, methodExtension.Position.Line, methodExtension.Position.Column).
					WithHint(localize.Text("Проверьте имя поля и убедитесь, что оно объявлено внутри модели.", "Check the field name and make sure it is declared inside the model."))
			}

			field.Methods = mergeFieldMethods(field.Methods, methodExtension.Methods)
			continue
		}
		if extension == nil {
			continue
		}

		modelName := extension.Model.String()
		fieldName := extension.Field.String()

		model, exists := models[modelName]
		if !exists {
			return Err.Newf(
				localize.Text("модель %q для field-блока не найдена", "model %q for field block was not found"),
				modelName,
			).
				WithLocation(extension.Position.File, extension.Position.Line, extension.Position.Column).
				WithHint(localize.Text("Сначала объявите модель, а потом добавляйте `field Model.Field { ... }` блоки.", "Declare the model first, then add `field Model.Field { ... }` blocks."))
		}

		field, exists := model.FindField(fieldName)
		if !exists {
			return Err.Newf(
				localize.Text("поле %q модели %q для field-блока не найдено", "field %q of model %q for field block was not found"),
				fieldName,
				modelName,
			).
				WithLocation(extension.Position.File, extension.Position.Line, extension.Position.Column).
				WithHint(localize.Text("Проверьте имя поля и убедитесь, что оно объявлено внутри модели.", "Check the field name and make sure it is declared inside the model."))
		}

		markAttrOrigin(extension.Attrs, "field_extension")

		// Inline attrs inside the model are the primary source of truth.
		// Top-level `field Model.Field { ... }` blocks may extend the field,
		// but they must not override values already declared next to the field.
		field.Attrs = mergeAttrs(extension.Attrs, field.Attrs)
	}

	file.ASTs = resolved
	return nil
}

func mergeFieldMethods(base []FieldMethodAST, overlay []FieldMethodAST) []FieldMethodAST {
	if len(base) == 0 {
		return cloneFieldMethods(overlay)
	}
	if len(overlay) == 0 {
		return cloneFieldMethods(base)
	}

	merged := cloneFieldMethods(base)
	for _, item := range overlay {
		exists := false
		for _, current := range merged {
			if current.Name == item.Name {
				exists = true
				break
			}
		}
		if !exists {
			merged = append(merged, cloneFieldMethod(item))
		}
	}

	return merged
}

// mergeAttrs merges two attr lists by identifier.
// Values from overlay win over base when both define the same attr/key.
func mergeAttrs(base []Attr, overlay []Attr) []Attr {
	if len(base) == 0 {
		return cloneAttrs(overlay)
	}
	if len(overlay) == 0 {
		return cloneAttrs(base)
	}

	merged := cloneAttrs(base)
	for _, item := range overlay {
		replaced := false
		for i := range merged {
			if merged[i].Identifier() != item.Identifier() {
				continue
			}

			merged[i] = mergeAttr(merged[i], item)
			replaced = true
			break
		}

		if !replaced {
			merged = append(merged, cloneAttr(item))
		}
	}

	return merged
}

func mergeAttr(base Attr, overlay Attr) Attr {
	// Для позиционных аргументов и маркерных атрибутов используем полную замену.
	if len(overlay.NamedArgs) == 0 {
		return cloneAttr(overlay)
	}

	merged := cloneAttr(base)
	if len(overlay.Args) > 0 {
		merged.Args = cloneExprs(overlay.Args)
	}

	merged.NamedArgs = mergeNamedArgs(base.NamedArgs, overlay.NamedArgs)
	return merged
}

func mergeNamedArgs(base []NamedArg, overlay []NamedArg) []NamedArg {
	if len(base) == 0 {
		return cloneNamedArgs(overlay)
	}
	if len(overlay) == 0 {
		return cloneNamedArgs(base)
	}

	merged := cloneNamedArgs(base)
	for _, item := range overlay {
		replaced := false
		for i := range merged {
			if merged[i].Name != item.Name {
				continue
			}

			merged[i] = item
			replaced = true
			break
		}

		if !replaced {
			merged = append(merged, item)
		}
	}

	return merged
}

func cloneAttrs(items []Attr) []Attr {
	if len(items) == 0 {
		return nil
	}

	cloned := make([]Attr, len(items))
	for i := range items {
		cloned[i] = cloneAttr(items[i])
	}

	return cloned
}

func cloneAttr(item Attr) Attr {
	item.Args = cloneExprs(item.Args)
	item.NamedArgs = cloneNamedArgs(item.NamedArgs)
	return item
}

func cloneFieldMethods(items []FieldMethodAST) []FieldMethodAST {
	if len(items) == 0 {
		return nil
	}

	cloned := make([]FieldMethodAST, len(items))
	for i := range items {
		cloned[i] = cloneFieldMethod(items[i])
	}

	return cloned
}

func cloneFieldMethod(item FieldMethodAST) FieldMethodAST {
	item.Params = cloneFieldMethodParams(item.Params)
	item.Returns = cloneTypeRefs(item.Returns)
	item.Attrs = cloneAttrs(item.Attrs)
	return item
}

func cloneFieldMethodParams(items []FieldMethodParamAST) []FieldMethodParamAST {
	if len(items) == 0 {
		return nil
	}

	cloned := make([]FieldMethodParamAST, len(items))
	copy(cloned, items)
	return cloned
}

func cloneTypeRefs(items []TypeRef) []TypeRef {
	if len(items) == 0 {
		return nil
	}

	cloned := make([]TypeRef, len(items))
	copy(cloned, items)
	return cloned
}

func cloneExprs(items []Expr) []Expr {
	if len(items) == 0 {
		return nil
	}

	cloned := make([]Expr, len(items))
	copy(cloned, items)
	return cloned
}

func cloneNamedArgs(items []NamedArg) []NamedArg {
	if len(items) == 0 {
		return nil
	}

	cloned := make([]NamedArg, len(items))
	copy(cloned, items)
	return cloned
}

func markAttrOrigin(items []Attr, origin string) {
	for i := range items {
		if items[i].Origin == "" {
			items[i].Origin = origin
		}
		for j := range items[i].NamedArgs {
			if items[i].NamedArgs[j].Origin == "" {
				items[i].NamedArgs[j].Origin = origin
			}
		}
	}
}
