package sdk

import (
	"fmt"
	"sort"
	"strings"
)

type AttrValueType string

const (
	AttrValueTypeAny        AttrValueType = "any"
	AttrValueTypeBool       AttrValueType = "bool"
	AttrValueTypeNumber     AttrValueType = "number"
	AttrValueTypeString     AttrValueType = "string"
	AttrValueTypeName       AttrValueType = "name"
	AttrValueTypeStringLike AttrValueType = "string-like"
)

type AttrArgSpec struct {
	Name          string          `json:"name"`
	Types         []AttrValueType `json:"types,omitempty"`
	Help          string          `json:"help,omitempty"`
	Aliases       []string        `json:"aliases,omitempty"`
	ConflictsWith []string        `json:"conflicts_with,omitempty"`
	Positional    bool            `json:"positional,omitempty"`
}

type AttrSnippetSpec struct {
	Label  string `json:"label,omitempty"`
	Insert string `json:"insert,omitempty"`
	Help   string `json:"help,omitempty"`
}

type AttrSpec struct {
	Namespace string            `json:"namespace"`
	Help      string            `json:"help,omitempty"`
	Args      []AttrArgSpec     `json:"args,omitempty"`
	Snippets  []AttrSnippetSpec `json:"snippets,omitempty"`
}

type DescribeAttrsResponse struct {
	Specs []AttrSpec `json:"specs,omitempty"`
}

func (spec AttrSpec) PositionalArgs() []AttrArgSpec {
	if len(spec.Args) == 0 {
		return nil
	}

	items := make([]AttrArgSpec, 0, len(spec.Args))
	for _, item := range spec.Args {
		if item.Positional {
			items = append(items, item)
		}
	}

	return items
}

func (spec AttrSpec) HasPositionalArg() bool {
	return len(spec.PositionalArgs()) > 0
}

func (spec AttrSpec) ShorthandBoolArgName(value Value) (string, bool) {
	if strings.TrimSpace(value.Kind) != "name" {
		return "", false
	}

	arg, ok := spec.FindArg(value.String())
	if !ok || arg.Positional || !attrArgSupportsType(arg, AttrValueTypeBool) {
		return "", false
	}

	return strings.TrimSpace(arg.Name), true
}

func (spec AttrSpec) AllowedArgNames() []string {
	if len(spec.Args) == 0 {
		return nil
	}

	items := make([]string, 0, len(spec.Args))
	for _, item := range spec.Args {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		items = append(items, strings.TrimSpace(item.Name))
	}

	sort.Strings(items)
	return items
}

func (spec AttrSpec) FindArg(name string) (AttrArgSpec, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return AttrArgSpec{}, false
	}

	for _, item := range spec.Args {
		if strings.TrimSpace(item.Name) == name {
			return item, true
		}
		for _, alias := range item.Aliases {
			if strings.TrimSpace(alias) == name {
				return item, true
			}
		}
	}

	return AttrArgSpec{}, false
}

func (spec AttrSpec) HelpText() string {
	args := spec.AllowedArgNames()
	if len(args) == 0 {
		return strings.TrimSpace(spec.Help)
	}

	line := fmt.Sprintf("Allowed args: %s.", strings.Join(args, ", "))
	if strings.TrimSpace(spec.Help) == "" {
		return line
	}

	return strings.TrimSpace(spec.Help) + " " + line
}

func (builder *AnalyzeBuilder) ValidateAttrSpec(attrs []Attr, spec AttrSpec) ResolvedAttr {
	normalizedAttrs := NormalizeAttrsWithSpecs(attrs, []AttrSpec{spec})
	resolved, ok := ResolveAttrs(normalizedAttrs, spec.Namespace)
	if !ok {
		return ResolvedAttr{
			Identifier: strings.TrimSpace(spec.Namespace),
			Namespace:  strings.TrimSpace(spec.Namespace),
			Values:     make(map[string]ResolvedValue),
		}
	}

	allowed := spec.AllowedArgNames()
	positionalArgs := spec.PositionalArgs()
	for _, attr := range resolved.Attrs {
		subName := strings.TrimSpace(attr.SubName())
		if subName != "" {
			argSpec, exists := spec.FindArg(subName)
			if !exists {
				builder.AddDiagnostic(UnknownAttrName(attr, spec.Namespace, subName, allowed, spec.HelpText()))
			} else if len(attr.Args) > 0 && !valueMatchesAny(attr.Args[0], argSpec.Types) {
				builder.AddDiagnostic(WrongArgType(attr, spec.Namespace, argSpec.Name, attr.Args[0], argSpec.Types, argSpec.Help))
			}
		}

		for _, item := range attr.NamedArgs {
			argSpec, exists := spec.FindArg(item.Name)
			if !exists {
				builder.AddDiagnostic(UnknownArg(item, spec.Namespace, item.Name, allowed, spec.HelpText()))
				continue
			}
			if !valueMatchesAny(item.Value, argSpec.Types) {
				builder.AddDiagnostic(WrongArgType(item, spec.Namespace, argSpec.Name, item.Value, argSpec.Types, argSpec.Help))
			}
		}

		for index, value := range attr.Args {
			if len(positionalArgs) == 0 {
				builder.AddDiagnostic(UnknownArg(attr, spec.Namespace, value.String(), allowed, spec.HelpText()))
				continue
			}
			if index >= len(positionalArgs) {
				builder.AddDiagnostic(UnknownArg(attr, spec.Namespace, value.String(), allowed, spec.HelpText()))
				continue
			}

			argSpec := positionalArgs[index]
			if !valueMatchesAny(value, argSpec.Types) {
				builder.AddDiagnostic(WrongArgType(attr, spec.Namespace, argSpec.Name, value, argSpec.Types, argSpec.Help))
			}
		}
	}

	for _, conflict := range resolved.Conflicts {
		builder.AddDiagnostic(ConflictingArgs(spec.Namespace, conflict))
	}

	seen := make(map[string]struct{})
	for _, arg := range spec.Args {
		if _, ok := resolved.Value(arg.Name); !ok {
			continue
		}

		for _, conflictName := range arg.ConflictsWith {
			if _, ok := resolved.Value(conflictName); !ok {
				continue
			}
			keyParts := []string{strings.TrimSpace(arg.Name), strings.TrimSpace(conflictName)}
			sort.Strings(keyParts)
			key := strings.Join(keyParts, "|")
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			builder.AddDiagnostic(Diagnostic{
				Message: fmt.Sprintf(Text("аргументы %s и %s у %s конфликтуют", "arguments %s and %s on %s conflict"), Quote(arg.Name), Quote(conflictName), "@"+spec.Namespace),
				Hint:    Text("Оставьте только один из конфликтующих аргументов.", "Keep only one of the conflicting arguments."),
			})
		}
	}

	return resolved
}

func valueMatchesAny(value Value, kinds []AttrValueType) bool {
	if len(kinds) == 0 {
		return true
	}

	for _, kind := range kinds {
		if valueMatchesType(value, kind) {
			return true
		}
	}

	return false
}

func valueMatchesType(value Value, kind AttrValueType) bool {
	switch kind {
	case AttrValueTypeAny:
		return true
	case AttrValueTypeBool:
		return value.IsBool()
	case AttrValueTypeNumber:
		return value.IsNumber()
	case AttrValueTypeString:
		return value.Kind == "string"
	case AttrValueTypeName:
		return value.Kind == "name"
	case AttrValueTypeStringLike:
		return value.IsStringLike()
	default:
		return false
	}
}

func attrArgSupportsType(arg AttrArgSpec, kind AttrValueType) bool {
	if len(arg.Types) == 0 {
		return true
	}

	for _, item := range arg.Types {
		if item == kind {
			return true
		}
	}

	return false
}

func formatExpectedTypes(items []AttrValueType) string {
	if len(items) == 0 {
		return "any"
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, string(item))
	}

	return strings.Join(parts, ", ")
}
