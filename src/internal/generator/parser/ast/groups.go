package ast

import (
	"fmt"
	"rpl/internal/generator/parser/lexer/token"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
	"unicode"
)

// ExpandGroups turns field-level `@group("req")` markers into real derived
// models such as `UserReq`. The current model keeps its original fields, while
// the derived model gets only the fields that belong to the named group.
func (file *File) ExpandGroups() error {
	if file == nil {
		return nil
	}

	models := file.modelIndex()
	for _, model := range file.Models() {
		if model == nil || strings.TrimSpace(model.GeneratedFrom) != "" {
			continue
		}

		groups := make(map[string][]*FieldAST)
		groupPositions := make(map[string]token.Position)

		for _, field := range model.Fields {
			if field == nil {
				continue
			}

			groupAttr, ok := field.FindAttr("group")
			if !ok {
				continue
			}

			groupValue := strings.TrimSpace(attrFirstString(groupAttr))
			if groupValue == "" {
				return groupError(field.Position, file, localize.Text("атрибут @group(...) требует имя группы", "@group(...) requires a group name")).
					WithHint(localize.Text("Пример: `Name string @group(\"data\")`.", "Example: `Name string @group(\"data\")`."))
			}

			groups[groupValue] = append(groups[groupValue], field)
			if _, exists := groupPositions[groupValue]; !exists {
				groupPositions[groupValue] = field.Position
			}
		}

		for groupValue, fields := range groups {
			groupModelName := groupedModelName(model.Name, groupValue)
			if existing, ok := models[groupModelName]; ok {
				if existing.GeneratedFrom == model.Name && strings.TrimSpace(existing.GroupName) == strings.TrimSpace(groupValue) {
					continue
				}

				return groupError(groupPositions[groupValue], file, fmt.Sprintf(
					localize.Text("авто-группа %q конфликтует с уже существующей моделью %q", "auto-group %q conflicts with the existing model %q"),
					groupModelName,
					existing.Name,
				)).WithHint(localize.Text(
					"Переименуйте существующую модель или используйте другое имя группы.",
					"Rename the existing model or use a different group name.",
				))
			}

			derived := deriveGroupModel(model, groupModelName, groupValue, fields)
			file.ASTs = append(file.ASTs, derived)
			models[groupModelName] = derived
		}
	}

	return nil
}

func (file *File) modelIndex() map[string]*ModelAST {
	index := make(map[string]*ModelAST)
	for _, model := range file.Models() {
		if model == nil {
			continue
		}

		index[strings.TrimSpace(model.Name)] = model
	}

	return index
}

func deriveGroupModel(base *ModelAST, name string, group string, selectedFields []*FieldAST) *ModelAST {
	fields := make([]*FieldAST, 0, len(selectedFields))
	for _, field := range selectedFields {
		if field == nil {
			continue
		}

		cloned := *field
		cloned.Default = field.Default
		cloned.Attrs = removeAttrByIdentifier(cloneAttrs(field.Attrs), "group")
		cloned.Methods = nil
		fields = append(fields, &cloned)
	}

	return &ModelAST{
		Position:      base.Position,
		Name:          name,
		Attrs:         nil,
		Fields:        fields,
		GeneratedFrom: base.Name,
		GroupName:     group,
	}
}

func removeAttrByIdentifier(items []Attr, identifier string) []Attr {
	if len(items) == 0 {
		return nil
	}

	filtered := make([]Attr, 0, len(items))
	for _, item := range items {
		if item.Matches(identifier) {
			continue
		}

		filtered = append(filtered, cloneAttr(item))
	}

	return filtered
}

func attrFirstString(attr *Attr) string {
	if attr == nil {
		return ""
	}
	if len(attr.Args) > 0 {
		return strings.TrimSpace(ExprString(attr.Args[0]))
	}
	if value, ok := attr.NamedArg("name"); ok {
		return strings.TrimSpace(ExprString(value))
	}
	if value, ok := attr.NamedArg("group"); ok {
		return strings.TrimSpace(ExprString(value))
	}

	return ""
}

func groupedModelName(base string, group string) string {
	return strings.TrimSpace(base) + groupSuffix(group)
}

func groupSuffix(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	upperNext := true
	for _, item := range trimmed {
		switch {
		case unicode.IsLetter(item) || unicode.IsDigit(item):
			if upperNext {
				builder.WriteRune(unicode.ToUpper(item))
				upperNext = false
			} else {
				builder.WriteRune(item)
			}
		default:
			upperNext = true
		}
	}

	result := builder.String()
	if result == "" {
		return "Group"
	}

	return result
}

func groupError(position token.Position, file *File, message string) *Err.Error {
	_ = file
	return Err.New(message).
		WithLocation(position.File, position.Line, position.Column).
		WithKind("group")
}
