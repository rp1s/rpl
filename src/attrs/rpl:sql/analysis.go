package main

import (
	"fmt"
	"regexp"
	"rpl/pkg/sdk"
	"strconv"
	"strings"
)

var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var sqlModelSpec = sdk.AttrSpec{
	Namespace: "sql",
	Help:      sdk.Text("На уровне модели sql настраивает диалект и имя таблицы.", "At model level sql configures the dialect and table name."),
	Args: []sdk.AttrArgSpec{
		{Name: "db", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "table", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@sql", Insert: "@sql", Help: sdk.Text("Базовый SQL-атрибут.", "Base SQL attr.")},
	},
}

var sqlFieldSpec = sdk.AttrSpec{
	Namespace: "sql",
	Help:      sdk.Text("На уровне поля sql настраивает имя колонки, ключи, default, индексы, updatedAt и ignore.", "At field level sql configures the column name, keys, default, indexes, updatedAt, and ignore."),
	Args: []sdk.AttrArgSpec{
		{Name: "column", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "default", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "index", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "primaryKey", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "unique", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "updatedAt", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "ignore", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool, sdk.AttrValueTypeStringLike}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@sql", Insert: "@sql", Help: sdk.Text("Базовый SQL-атрибут поля.", "Base SQL field attr.")},
	},
}

func analyzeSQL(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()

	modelResolved := builder.ValidateAttrSpec(req.Model.RuntimeAttrs, sqlModelSpec)
	modelValues := modelResolved.ValueMap()
	validateSQLDialect(builder, modelRuntimeAttr(req.Model, "sql"), modelValues)
	validateSQLIdentifier(builder, modelRuntimeAttr(req.Model, "sql"), "table", modelValues["table"].String())

	columns := make(map[string]string)
	for _, field := range req.Model.Fields {
		analyzeSQLField(builder, field)
		if !field.IgnoredBy("sql") && !field.Type.IsExternal(req.File) {
			column := resolvedSQLColumnName(field)
			if previous, exists := columns[column]; exists {
				builder.AddDiagnostic(sdk.DiagnosticAt(
					fieldRuntimeAttr(field, "sql"),
					fmt.Sprintf(sdk.Text("sql-колонка %q используется полями %q и %q", "SQL column %q is used by fields %q and %q"), column, previous, field.Name),
					sdk.Text("Задайте уникальное имя через `@sql(column: \"...\")`.", "Choose a unique name with `@sql(column: \"...\")`."),
				))
			} else {
				columns[column] = field.Name
			}
		}
		for _, method := range field.Methods {
			for _, attr := range method.RuntimeAttrs {
				builder.AddDiagnostic(sdk.DiagnosticAt(
					attr,
					fmt.Sprintf(sdk.Text("attr %q нельзя использовать на методе поля %q", "attr %q cannot be used on field method %q"), attr.Identifier, method.Name),
					sdk.Text("SQL attrs описывают модель и поля хранения, а не методы поля.", "SQL attrs describe storage metadata on models and fields, not field methods."),
				))
			}
		}
	}

	for _, field := range req.Model.ActiveFields("sql") {
		builder.AddClaim("field.domain", "storage", req.Model.Name+"."+field.Name)
	}

	generated, err := generateSQL(req)
	if err != nil {
		return sdk.AnalyzeResponse{}, err
	}
	sdk.AddGeneratedClaimsInScope(builder, generated, packageScope(req.File, "sql"))
	return builder.Response(), nil
}

func validateSQLDialect(builder *sdk.AnalyzeBuilder, attr sdk.Attr, values map[string]sdk.Value) {
	if builder == nil {
		return
	}

	raw := strings.TrimSpace(values["db"].String())
	if raw == "" {
		return
	}

	switch strings.ToLower(raw) {
	case string(sqlDialectPostgres), "postgresql", string(sqlDialectSQLite), "sqlite3":
		return
	default:
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(sdk.Text("неподдерживаемый sql db %q", "unsupported sql db %q"), raw),
			sdk.Text("Используйте `postgres` или `sqlite`.", "Use `postgres` or `sqlite`."),
		))
	}
}

func analyzeSQLField(builder *sdk.AnalyzeBuilder, field sdk.Field) {
	resolved := builder.ValidateAttrSpec(field.RuntimeAttrs, sqlFieldSpec)
	values := resolved.ValueMap()
	attr := fieldRuntimeAttr(field, "sql")

	validateSQLIdentifier(builder, attr, "column", values["column"].String())
	validateSQLDefault(builder, attr, field, values["default"].String())

	if values["primaryKey"].BoolValue() && field.Type.Optional {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(sdk.Text("primary key %q не может быть nullable", "primary key %q cannot be nullable"), field.Name),
			sdk.Text("Уберите `?` у типа primary key.", "Remove `?` from the primary-key type."),
		))
	}

	if values["updatedAt"].BoolValue() && !field.Type.IsTime() {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(sdk.Text("sql(updatedAt: true) можно ставить только на time.Time, а не на %q", "sql(updatedAt: true) can only be used on time.Time, not %q"), field.Type.Name),
			sdk.Text("Обычно это поле выглядит как `UpdatedAt time.Time @sql(default: \"now\", updatedAt: true)`.", "A typical field looks like `UpdatedAt time.Time @sql(default: \"now\", updatedAt: true)`."),
		))
	}

	if field.IgnoredBy("sql") && hasMeaningfulRuntimeConfig(values) {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(sdk.Text("поле %q одновременно игнорирует и настраивает sql", "field %q both ignores and configures sql"), field.Name),
			sdk.Text("Если поле нужно исключить из sql, уберите остальные sql-аргументы.", "If the field should be ignored by sql, remove the rest of the sql arguments."),
		))
	}
}

func validateSQLIdentifier(builder *sdk.AnalyzeBuilder, attr sdk.Attr, name string, raw string) {
	if builder == nil {
		return
	}
	value := strings.TrimSpace(raw)
	if value == "" || sqlIdentifierPattern.MatchString(value) {
		return
	}

	builder.AddDiagnostic(sdk.DiagnosticAt(
		attr,
		fmt.Sprintf(sdk.Text("некорректный SQL identifier %s=%q", "invalid SQL identifier %s=%q"), name, value),
		sdk.Text("Используйте буквы, цифры и `_`; первый символ должен быть буквой или `_`.", "Use letters, digits, and `_`; the first character must be a letter or `_`."),
	))
}

func validateSQLDefault(builder *sdk.AnalyzeBuilder, attr sdk.Attr, field sdk.Field, raw string) {
	if builder == nil {
		return
	}
	value := strings.TrimSpace(raw)
	if value == "" || field.Type.IsTime() || field.Type.IsString() || field.Type.IsList {
		return
	}

	valid := true
	switch {
	case field.Type.IsBool():
		_, err := strconv.ParseBool(value)
		valid = err == nil || value == "0" || value == "1"
	case field.Type.IsInteger():
		_, err := strconv.ParseInt(value, 10, 64)
		if strings.HasPrefix(field.Type.BaseName(), "u") || field.Type.BaseName() == "byte" {
			_, err = strconv.ParseUint(value, 10, 64)
		}
		valid = err == nil
	case field.Type.IsFloat():
		_, err := strconv.ParseFloat(value, 64)
		valid = err == nil
	}
	if valid {
		return
	}

	builder.AddDiagnostic(sdk.DiagnosticAt(
		attr,
		fmt.Sprintf(sdk.Text("default %q несовместим с SQL-полем %q", "default %q is incompatible with SQL field %q"), value, field.Name),
		sdk.Text("Укажите литерал, совместимый с типом поля.", "Use a literal compatible with the field type."),
	))
}

func resolvedSQLColumnName(field sdk.Field) string {
	if value := strings.TrimSpace(field.ResolvedValues("sql")["column"].String()); value != "" {
		return value
	}
	return sdk.SnakeCase(field.Name)
}

func hasMeaningfulRuntimeConfig(values map[string]sdk.Value) bool {
	for name := range values {
		if name != "ignore" {
			return true
		}
	}
	return false
}

func fieldRuntimeAttr(field sdk.Field, name string) sdk.Attr {
	attr, _ := field.ResolvedAttr(name)
	if len(attr.Attrs) > 0 {
		return attr.Attrs[0]
	}
	return sdk.Attr{}
}

func modelRuntimeAttr(model sdk.Model, name string) sdk.Attr {
	attr, _ := model.ResolvedAttr(name)
	if len(attr.Attrs) > 0 {
		return attr.Attrs[0]
	}
	return sdk.Attr{}
}

func packageScope(file sdk.FileContext, parts ...string) string {
	base := strings.TrimSpace(file.GoPackagePath)
	if base == "" {
		base = strings.TrimSpace(file.PackageName)
	}
	items := make([]string, 0, len(parts)+1)
	if base != "" {
		items = append(items, base)
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return strings.Join(items, "/")
}
