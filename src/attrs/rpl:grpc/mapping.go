package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func renderRootToMethod(plan *grpcPlan) string {
	lines := renderToBody("model", "message", plan.RootNode)
	body := append([]string{"message := &" + plan.RootNode.MessageName + "{}"}, lines...)
	body = append(body, "return message")

	return sdk.WithDocComment(
		fmt.Sprintf("func ToMessage(model %s) *%s {\n\t%s\n}", grpcModelGoType(plan, plan.RootModel.Name), plan.RootNode.MessageName, strings.Join(body, "\n\t")),
		"ToMessage преобразует модель %s в protobuf-сообщение.",
		"ToMessage converts model %s into its protobuf message.",
		plan.RootModel.Name,
	)
}

func renderRootFromMethod(plan *grpcPlan) string {
	lines := []string{"model := " + grpcModelGoType(plan, plan.RootModel.Name) + "{}", "if message == nil {\n\t\treturn model, nil\n\t}"}
	lines = append(lines, renderFromBody("model", "message", plan.RootNode)...)
	lines = append(lines, "return model, nil")

	return sdk.WithDocComment(
		fmt.Sprintf("func FromMessage(message *%s) (%s, error) {\n\t%s\n}", plan.RootNode.MessageName, grpcModelGoType(plan, plan.RootModel.Name), strings.Join(lines, "\n\t")),
		"FromMessage восстанавливает модель %s из protobuf-сообщения.",
		"FromMessage restores model %s from its protobuf message.",
		plan.RootModel.Name,
	)
}

func renderNestedHelpers(plan *grpcPlan) string {
	if len(plan.Nodes) <= 1 {
		return ""
	}

	parts := make([]string, 0, (len(plan.Nodes)-1)*2)
	for _, node := range plan.Nodes[1:] {
		parts = append(parts, renderToHelper(plan, node))
		parts = append(parts, renderFromHelper(plan, node))
	}

	return strings.Join(parts, "\n\n")
}

func renderToHelper(plan *grpcPlan, node *grpcMessageNode) string {
	lines := renderToBody("value", "message", node)
	body := append([]string{"message := &" + node.MessageName + "{}"}, lines...)
	body = append(body, "return message")

	return sdk.WithDocComment(
		fmt.Sprintf("func %s(value %s) *%s {\n\t%s\n}", node.ToHelperName, grpcModelGoType(plan, node.Model.Name), node.MessageName, strings.Join(body, "\n\t")),
		"%s конвертирует вложенную модель %s в protobuf-сообщение.",
		"%s converts nested model %s into a protobuf message.",
		node.ToHelperName,
		node.Model.Name,
	)
}

func renderFromHelper(plan *grpcPlan, node *grpcMessageNode) string {
	lines := []string{"model := " + grpcModelGoType(plan, node.Model.Name) + "{}", "if message == nil {\n\t\treturn model, nil\n\t}"}
	lines = append(lines, renderFromBody("model", "message", node)...)
	lines = append(lines, "return model, nil")

	return sdk.WithDocComment(
		fmt.Sprintf("func %s(message *%s) (%s, error) {\n\t%s\n}", node.FromHelperName, node.MessageName, grpcModelGoType(plan, node.Model.Name), strings.Join(lines, "\n\t")),
		"%s конвертирует вложенное protobuf-сообщение обратно в модель %s.",
		"%s converts the nested protobuf message back into model %s.",
		node.FromHelperName,
		node.Model.Name,
	)
}

func renderToBody(modelVar string, messageVar string, node *grpcMessageNode) []string {
	lines := make([]string, 0, len(node.Fields))
	for _, field := range node.Fields {
		lines = append(lines, renderToField(modelVar, messageVar, field))
	}

	return lines
}

func renderFromBody(modelVar string, messageVar string, node *grpcMessageNode) []string {
	lines := make([]string, 0, len(node.Fields))
	for _, field := range node.Fields {
		lines = append(lines, renderFromField(modelVar, messageVar, field))
	}

	return lines
}

func renderToField(modelVar string, messageVar string, field grpcFieldBinding) string {
	source := modelVar + "." + field.Field.Name
	target := messageVar + "." + grpcGeneratedFieldName(field.Field.Name)

	if field.Field.Type.IsList {
		itemType := messageSliceElementType(field)
		lines := []string{
			fmt.Sprintf("if len(%s) > 0 {", source),
			fmt.Sprintf("\t\titems := make([]%s, 0, len(%s))", itemType, source),
			fmt.Sprintf("\t\tfor _, item := range %s {", source),
			fmt.Sprintf("\t\t\titems = append(items, %s)", grpcToExpr(field, "item")),
			"\t\t}",
			fmt.Sprintf("\t\t%s = items", target),
			"\t}",
		}
		return strings.Join(lines, "\n\t")
	}

	if field.Field.Type.Optional {
		lines := []string{
			fmt.Sprintf("if %s != nil {", source),
			fmt.Sprintf("\t\t%s = %s", target, grpcToExpr(field, "*"+source)),
			"\t}",
		}
		return strings.Join(lines, "\n\t")
	}

	return fmt.Sprintf("%s = %s", target, grpcToExpr(field, source))
}

func renderFromField(modelVar string, messageVar string, field grpcFieldBinding) string {
	source := messageVar + "." + grpcGeneratedFieldName(field.Field.Name)
	target := modelVar + "." + field.Field.Name

	if field.Field.Type.IsList {
		switch field.Kind {
		case "model":
			lines := []string{
				fmt.Sprintf("if len(%s) > 0 {", source),
				fmt.Sprintf("\t\titems := make(%s, 0, len(%s))", field.Field.GoType(), source),
				fmt.Sprintf("\t\tfor _, item := range %s {", source),
				"\t\t\tif item == nil {",
				"\t\t\t\tcontinue",
				"\t\t\t}",
				fmt.Sprintf("\t\t\tvalue, err := %s(item)", field.Child.FromHelperName),
				"\t\t\tif err != nil {",
				"\t\t\t\treturn model, err",
				"\t\t\t}",
				"\t\t\titems = append(items, value)",
				"\t\t}",
				fmt.Sprintf("\t\t%s = items", target),
				"\t}",
			}
			return strings.Join(lines, "\n\t")
		case "time":
			lines := []string{
				fmt.Sprintf("if len(%s) > 0 {", source),
				fmt.Sprintf("\t\titems := make(%s, 0, len(%s))", field.Field.GoType(), source),
				fmt.Sprintf("\t\tfor _, item := range %s {", source),
				"\t\t\tif item == nil {",
				"\t\t\t\tcontinue",
				"\t\t\t}",
				"\t\t\titems = append(items, item.AsTime())",
				"\t\t}",
				fmt.Sprintf("\t\t%s = items", target),
				"\t}",
			}
			return strings.Join(lines, "\n\t")
		default:
			lines := []string{
				fmt.Sprintf("if len(%s) > 0 {", source),
				fmt.Sprintf("\t\titems := make(%s, 0, len(%s))", field.Field.GoType(), source),
				fmt.Sprintf("\t\tfor _, item := range %s {", source),
				fmt.Sprintf("\t\t\titems = append(items, %s)", grpcFromExpr(field, "item")),
				"\t\t}",
				fmt.Sprintf("\t\t%s = items", target),
				"\t}",
			}
			return strings.Join(lines, "\n\t")
		}
	}

	if field.Field.Type.Optional {
		switch field.Kind {
		case "model":
			lines := []string{
				fmt.Sprintf("if %s != nil {", source),
				fmt.Sprintf("\t\tvalue, err := %s(%s)", field.Child.FromHelperName, source),
				"\t\tif err != nil {",
				"\t\t\treturn model, err",
				"\t\t}",
				fmt.Sprintf("\t\t%s = &value", target),
				"\t}",
			}
			return strings.Join(lines, "\n\t")
		case "time":
			lines := []string{
				fmt.Sprintf("if %s != nil {", source),
				fmt.Sprintf("\t\tvalue := %s.AsTime()", source),
				fmt.Sprintf("\t\t%s = &value", target),
				"\t}",
			}
			return strings.Join(lines, "\n\t")
		default:
			lines := []string{
				fmt.Sprintf("if %s != nil {", source),
				fmt.Sprintf("\t\tvalue := %s", grpcFromOptionalExpr(field, source)),
				fmt.Sprintf("\t\t%s = &value", target),
				"\t}",
			}
			return strings.Join(lines, "\n\t")
		}
	}

	switch field.Kind {
	case "model":
		lines := []string{
			fmt.Sprintf("if %s != nil {", source),
			fmt.Sprintf("\t\tvalue, err := %s(%s)", field.Child.FromHelperName, source),
			"\t\tif err != nil {",
			"\t\t\treturn model, err",
			"\t\t}",
			fmt.Sprintf("\t\t%s = value", target),
			"\t}",
		}
		return strings.Join(lines, "\n\t")
	case "time":
		lines := []string{
			fmt.Sprintf("if %s != nil {", source),
			fmt.Sprintf("\t\t%s = %s.AsTime()", target, source),
			"\t}",
		}
		return strings.Join(lines, "\n\t")
	default:
		return fmt.Sprintf("%s = %s", target, grpcFromExpr(field, source))
	}
}

func grpcToExpr(field grpcFieldBinding, source string) string {
	switch field.Kind {
	case "string", "bool":
		return source
	case "int":
		return "int64(" + source + ")"
	case "uint":
		return "uint64(" + source + ")"
	case "float":
		return "float64(" + source + ")"
	case "wrapper_string":
		return "wrapperspb.String(" + source + ")"
	case "wrapper_bool":
		return "wrapperspb.Bool(" + source + ")"
	case "wrapper_int":
		return "wrapperspb.Int64(int64(" + source + "))"
	case "wrapper_uint":
		return "wrapperspb.UInt64(uint64(" + source + "))"
	case "wrapper_float":
		return "wrapperspb.Double(float64(" + source + "))"
	case "time":
		return "timestamppb.New(" + source + ")"
	case "model":
		return field.Child.ToHelperName + "(" + source + ")"
	default:
		return source
	}
}

func grpcFromExpr(field grpcFieldBinding, source string) string {
	switch field.Kind {
	case "string", "bool":
		return source
	case "int", "uint", "float":
		return field.Field.Type.BaseName() + "(" + source + ")"
	case "time":
		return source + ".AsTime()"
	default:
		return source
	}
}

func grpcFromOptionalExpr(field grpcFieldBinding, source string) string {
	switch field.Kind {
	case "wrapper_string", "wrapper_bool":
		return source + ".Value"
	case "wrapper_int":
		return field.Field.Type.BaseName() + "(" + source + ".Value)"
	case "wrapper_uint":
		return field.Field.Type.BaseName() + "(" + source + ".Value)"
	case "wrapper_float":
		return field.Field.Type.BaseName() + "(" + source + ".Value)"
	default:
		return source
	}
}

func messageSliceElementType(field grpcFieldBinding) string {
	return field.GoMessageType
}
