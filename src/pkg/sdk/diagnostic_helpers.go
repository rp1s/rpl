package sdk

import (
	"fmt"
	"strings"
)

func DiagnosticAt(node DiagnosticLoc, message string, hint ...string) Diagnostic {
	item := Diagnostic{
		Message: strings.TrimSpace(message),
	}
	if len(hint) > 0 {
		item.Hint = strings.TrimSpace(hint[0])
	}
	if node == nil {
		return item
	}

	origin := node.DiagnosticOrigin()
	item.Path = strings.TrimSpace(origin.Path)
	item.Line = origin.Line
	item.Column = origin.Column
	return item
}

func UnknownAttrName(attr Attr, namespace string, name string, allowed []string, help string) Diagnostic {
	hint := help
	if strings.TrimSpace(hint) == "" && len(allowed) > 0 {
		hint = fmt.Sprintf(Text("Разрешены: %s.", "Allowed values are: %s."), strings.Join(allowed, ", "))
	}

	return DiagnosticAt(
		attr,
		fmt.Sprintf(Text("неизвестное имя %s-attr %q", "unknown %s attr name %q"), strings.TrimSpace(namespace), strings.TrimSpace(name)),
		hint,
	)
}

func UnknownArg(node DiagnosticLoc, namespace string, name string, allowed []string, help string) Diagnostic {
	hint := help
	if strings.TrimSpace(hint) == "" && len(allowed) > 0 {
		hint = fmt.Sprintf(Text("Разрешены: %s.", "Allowed values are: %s."), strings.Join(allowed, ", "))
	}

	return DiagnosticAt(
		node,
		fmt.Sprintf(Text("неизвестный аргумент %s %q", "unknown %s argument %q"), strings.TrimSpace(namespace), strings.TrimSpace(name)),
		hint,
	)
}

func WrongArgType(node DiagnosticLoc, namespace string, name string, value Value, expected []AttrValueType, help string) Diagnostic {
	hint := help
	if strings.TrimSpace(hint) == "" {
		hint = fmt.Sprintf(
			Text("Ожидается тип: %s.", "Expected type: %s."),
			formatExpectedTypes(expected),
		)
	}

	return DiagnosticAt(
		node,
		fmt.Sprintf(
			Text("аргумент %s %q имеет неверный тип %q", "%s argument %q has invalid type %q"),
			strings.TrimSpace(namespace),
			strings.TrimSpace(name),
			value.TypeName(),
		),
		hint,
	)
}

func IncompatibleAttrType(node DiagnosticLoc, namespace string, name string, typeRef TypeRef, hint string) Diagnostic {
	return DiagnosticAt(
		node,
		fmt.Sprintf(
			Text("%s(%s: ...) несовместим с типом поля %q", "%s(%s: ...) is incompatible with field type %q"),
			strings.TrimSpace(namespace),
			strings.TrimSpace(name),
			strings.TrimSpace(typeRef.Name),
		),
		hint,
	)
}

func ConflictingArgs(namespace string, conflict AttrConflict) Diagnostic {
	hint := Text("Оставьте только одно значение для этого аргумента.", "Keep only one value for this argument.")
	if !conflict.PreviousOrigin.IsZero() || !conflict.CurrentOrigin.IsZero() {
		hint = Text("У аргумента есть несколько значений из разных мест объявления; последнее значение побеждает, но это стоит явно почистить.", "This argument is declared with multiple values from different origins; the last value wins, but it should be cleaned up explicitly.")
	}

	return Diagnostic{
		Message: fmt.Sprintf(
			Text("конфликт аргумента %s %q: %q и %q", "conflicting %s argument %q: %q and %q"),
			strings.TrimSpace(namespace),
			strings.TrimSpace(conflict.Name),
			conflict.Previous.String(),
			conflict.Current.String(),
		),
		Hint: hint,
	}
}
