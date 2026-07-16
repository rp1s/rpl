package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func generateSQLHelpers(prefix string, dialect sqlDialect, fields []sqlFieldMeta) string {
	parts := make([]string, 0)

	if sqlNeedsJSON(fields) {
		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sMarshalJSON(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(body), nil
}`, prefix),
			"%sMarshalJSON сериализует значение в JSON-строку для SQL хранения.",
			"%sMarshalJSON serializes a value into JSON text for SQL storage.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sDecodeJSON(value any, target any) error {
	body, err := %sRawBytes(value)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}

	return json.Unmarshal(body, target)
}`, prefix, prefix),
			"%sDecodeJSON восстанавливает JSON-значение из сырого SQL-поля.",
			"%sDecodeJSON restores JSON data from a raw SQL field.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sRawBytes(value any) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	default:
		return nil, fmt.Errorf("unsupported raw sql value %%T", value)
	}
}`, prefix),
			"%sRawBytes приводит сырое SQL-значение к байтам.",
			"%sRawBytes converts a raw SQL value into bytes.",
			prefix,
		))
	}

	if sqlNeedsArrayHelpers(fields) {
		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sDecodeStringArray(value any) ([]string, error) {
	return %sParseArrayItems(value)
}`, prefix, prefix),
			"%sDecodeStringArray разбирает SQL array в срез строк.",
			"%sDecodeStringArray parses an SQL array into a string slice.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sParseArrayItems(value any) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return %sParseArrayText(string(typed))
	case string:
		return %sParseArrayText(typed)
	default:
		return nil, fmt.Errorf("unsupported sql array value %%T", value)
	}
}`, prefix, prefix, prefix),
			"%sParseArrayItems нормализует сырое SQL array-значение в список элементов.",
			"%sParseArrayItems normalizes a raw SQL array value into individual items.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sParseArrayText(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var items []string
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			return nil, err
		}
		return items, nil
	}
	if raw == "{}" {
		return []string{}, nil
	}
	if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return nil, fmt.Errorf("invalid sql array text %%q", raw)
	}

	input := raw[1:len(raw)-1]
	if strings.TrimSpace(input) == "" {
		return []string{}, nil
	}

	items := make([]string, 0)
	var builder strings.Builder
	inQuotes := false
	escaped := false
	for _, item := range input {
		switch {
		case escaped:
			builder.WriteRune(item)
			escaped = false
		case item == '\\':
			escaped = true
		case item == '"':
			inQuotes = !inQuotes
		case item == ',' && !inQuotes:
			items = append(items, builder.String())
			builder.Reset()
		default:
			builder.WriteRune(item)
		}
	}

	items = append(items, builder.String())
	for index := range items {
		if strings.EqualFold(items[index], "NULL") {
			items[index] = ""
		}
	}

	return items, nil
}`, prefix),
			"%sParseArrayText разбирает текстовое представление SQL array.",
			"%sParseArrayText parses the text representation of an SQL array.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sEncodeStringArray(values []string) string {
%s
}`, prefix, sqlEncodeStringArrayBody(dialect)),
			"%sEncodeStringArray кодирует срез строк в SQL array literal.",
			"%sEncodeStringArray encodes a string slice into an SQL array literal.",
			prefix,
		))
	}

	if sqlNeedsIntArrayHelpers(fields) {
		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sEncodeIntArray(values []int) string {
%s
}`, prefix, sqlEncodeIntArrayBody(dialect)),
			"%sEncodeIntArray кодирует целочисленный срез в SQL array literal.",
			"%sEncodeIntArray encodes an integer slice into an SQL array literal.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sDecodeIntArray(value any) ([]int, error) {
	items, err := %sParseArrayItems(value)
	if err != nil {
		return nil, err
	}
	if items == nil {
		return nil, nil
	}

	result := make([]int, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}

		value, err := strconv.Atoi(item)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}

	return result, nil
}`, prefix, prefix),
			"%sDecodeIntArray декодирует SQL array в срез int.",
			"%sDecodeIntArray decodes an SQL array into an int slice.",
			prefix,
		))
	}

	if sqlNeedsUintArrayHelpers(fields) {
		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sEncodeUintArray(values []uint64) string {
%s
}`, prefix, sqlEncodeUintArrayBody(dialect)),
			"%sEncodeUintArray кодирует срез uint64 в SQL array literal.",
			"%sEncodeUintArray encodes a uint64 slice into an SQL array literal.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sDecodeUintArray(value any) ([]uint64, error) {
	items, err := %sParseArrayItems(value)
	if err != nil {
		return nil, err
	}
	if items == nil {
		return nil, nil
	}

	result := make([]uint64, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}

		value, err := strconv.ParseUint(item, 10, 64)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}

	return result, nil
}`, prefix, prefix),
			"%sDecodeUintArray декодирует SQL array в срез uint64.",
			"%sDecodeUintArray decodes an SQL array into a uint64 slice.",
			prefix,
		))
	}

	if sqlNeedsFloatArrayHelpers(fields) {
		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sEncodeFloatArray(values []float64) string {
%s
}`, prefix, sqlEncodeFloatArrayBody(dialect)),
			"%sEncodeFloatArray кодирует срез float64 в SQL array literal.",
			"%sEncodeFloatArray encodes a float64 slice into an SQL array literal.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sDecodeFloatArray(value any) ([]float64, error) {
	items, err := %sParseArrayItems(value)
	if err != nil {
		return nil, err
	}
	if items == nil {
		return nil, nil
	}

	result := make([]float64, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}

		value, err := strconv.ParseFloat(item, 64)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}

	return result, nil
}`, prefix, prefix),
			"%sDecodeFloatArray декодирует SQL array в срез float64.",
			"%sDecodeFloatArray decodes an SQL array into a float64 slice.",
			prefix,
		))
	}

	if sqlNeedsBoolArrayHelpers(fields) {
		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sEncodeBoolArray(values []bool) string {
%s
}`, prefix, sqlEncodeBoolArrayBody(dialect)),
			"%sEncodeBoolArray кодирует срез bool в SQL array literal.",
			"%sEncodeBoolArray encodes a bool slice into an SQL array literal.",
			prefix,
		))

		parts = append(parts, sqlWithDoc(fmt.Sprintf(`func %sDecodeBoolArray(value any) ([]bool, error) {
	items, err := %sParseArrayItems(value)
	if err != nil {
		return nil, err
	}
	if items == nil {
		return nil, nil
	}

	result := make([]bool, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}

		value, err := strconv.ParseBool(item)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}

	return result, nil
}`, prefix, prefix),
			"%sDecodeBoolArray декодирует SQL array в срез bool.",
			"%sDecodeBoolArray decodes an SQL array into a bool slice.",
			prefix,
		))
	}

	return strings.Join(parts, "\n\n")
}

func sqlEncodeStringArrayBody(dialect sqlDialect) string {
	if dialect == sqlDialectSQLite {
		return `	if len(values) == 0 {
		return "[]"
	}

	body, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}

	return string(body)`
	}

	return `	if len(values) == 0 {
		return "{}"
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		escaped := strings.ReplaceAll(value, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		items = append(items, "\""+escaped+"\"")
	}

	return "{" + strings.Join(items, ",") + "}"`
}

func sqlEncodeIntArrayBody(dialect sqlDialect) string {
	if dialect == sqlDialectSQLite {
		return `	if len(values) == 0 {
		return "[]"
	}

	body, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}

	return string(body)`
	}

	return `	if len(values) == 0 {
		return "{}"
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.Itoa(value))
	}

	return "{" + strings.Join(items, ",") + "}"`
}

func sqlEncodeUintArrayBody(dialect sqlDialect) string {
	if dialect == sqlDialectSQLite {
		return `	if len(values) == 0 {
		return "[]"
	}

	body, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}

	return string(body)`
	}

	return `	if len(values) == 0 {
		return "{}"
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.FormatUint(value, 10))
	}

	return "{" + strings.Join(items, ",") + "}"`
}

func sqlEncodeFloatArrayBody(dialect sqlDialect) string {
	if dialect == sqlDialectSQLite {
		return `	if len(values) == 0 {
		return "[]"
	}

	body, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}

	return string(body)`
	}

	return `	if len(values) == 0 {
		return "{}"
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.FormatFloat(value, 'f', -1, 64))
	}

	return "{" + strings.Join(items, ",") + "}"`
}

func sqlEncodeBoolArrayBody(dialect sqlDialect) string {
	if dialect == sqlDialectSQLite {
		return `	if len(values) == 0 {
		return "[]"
	}

	body, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}

	return string(body)`
	}

	return `	if len(values) == 0 {
		return "{}"
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.FormatBool(value))
	}

	return "{" + strings.Join(items, ",") + "}"`
}

func sqlFieldSearchable(field sdk.Field, ctx sdk.FileContext) bool {
	if field.Type.IsList || field.Type.IsTime() || field.Type.IsNumeric() || field.Type.IsBool() {
		return false
	}
	if field.Type.IsModel(ctx) {
		return false
	}
	if !field.Type.IsString() {
		return false
	}

	if values := field.ResolvedValues("validate"); strings.TrimSpace(values["hash"].String()) != "" {
		return false
	}

	return true
}

func sqlColumnType(field sdk.Field, ctx sdk.FileContext, dialect sqlDialect) string {
	switch dialect {
	case sqlDialectSQLite:
		return sqlSQLiteColumnType(field, ctx)
	default:
		return sqlPostgresColumnType(field, ctx)
	}
}

func sqlSQLiteColumnType(field sdk.Field, ctx sdk.FileContext) string {
	switch {
	case field.Type.IsList:
		return "TEXT"
	case field.Type.IsString():
		return "TEXT"
	case field.Type.IsBool():
		return "BOOLEAN"
	case field.Type.IsInteger():
		return "INTEGER"
	case field.Type.IsFloat():
		return "REAL"
	case field.Type.IsTime():
		return "TIMESTAMP"
	case field.Type.IsModel(ctx):
		return "TEXT"
	default:
		return "TEXT"
	}
}

func sqlPostgresColumnType(field sdk.Field, ctx sdk.FileContext) string {
	switch {
	case field.Type.IsList:
		return sqlPostgresListType(field, ctx)
	case field.Type.IsString():
		return "TEXT"
	case field.Type.IsBool():
		return "BOOLEAN"
	case field.Type.IsInteger():
		return "BIGINT"
	case field.Type.IsFloat():
		return "DOUBLE PRECISION"
	case field.Type.IsTime():
		return "TIMESTAMP"
	case field.Type.IsModel(ctx):
		return "JSONB"
	default:
		return "JSONB"
	}
}

func sqlPostgresListType(field sdk.Field, ctx sdk.FileContext) string {
	switch field.Type.BaseName() {
	case "string":
		return "TEXT[]"
	case "bool":
		return "BOOLEAN[]"
	case "int", "int8", "int16", "int32", "int64":
		return "BIGINT[]"
	case "uint", "uint8", "uint16", "uint32", "uint64", "byte":
		return "BIGINT[]"
	case "float32", "float64":
		return "DOUBLE PRECISION[]"
	default:
		if field.Type.IsModel(ctx) {
			return "JSONB"
		}
		return "JSONB"
	}
}

func sqlStorageKind(field sdk.Field, ctx sdk.FileContext) string {
	switch {
	case field.Type.IsList && field.Type.IsString():
		return "array_string"
	case field.Type.IsList && field.Type.IsBool():
		return "array_bool"
	case field.Type.IsList && field.Type.IsInteger():
		if strings.HasPrefix(field.Type.BaseName(), "u") || field.Type.BaseName() == "byte" {
			return "array_uint"
		}
		return "array_int"
	case field.Type.IsList && field.Type.IsFloat():
		return "array_float"
	case field.Type.IsList:
		return "json"
	case field.Type.IsModel(ctx):
		return "json"
	default:
		return "scalar"
	}
}

func sqlDefaultLiteral(value string, field sdk.Field) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	if strings.EqualFold(trimmed, "now") && field.Type.IsTime() {
		return "CURRENT_TIMESTAMP"
	}

	if field.Type.IsNumeric() || field.Type.IsBool() {
		return trimmed
	}

	return "'" + strings.ReplaceAll(trimmed, "'", "''") + "'"
}

func sqlPlaceholder(field sqlFieldMeta, index int) string {
	if field.Dialect == sqlDialectSQLite {
		return "?"
	}

	placeholder := fmt.Sprintf("$%d", index)
	if fieldNeedsCast(field) {
		return placeholder + "::" + field.SQLType
	}

	return placeholder
}

func fieldNeedsCast(field sqlFieldMeta) bool {
	if field.Dialect != sqlDialectPostgres {
		return false
	}
	return strings.HasSuffix(field.SQLType, "[]") || field.SQLType == "JSONB"
}

func sqlNeedsJSON(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		if field.StorageKind == "json" {
			return true
		}
	}

	return false
}

func sqlNeedsArrayHelpers(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		if strings.HasPrefix(field.StorageKind, "array_") {
			return true
		}
	}

	return false
}

func sqlNeedsIntArrayHelpers(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		if field.StorageKind == "array_int" {
			return true
		}
	}

	return false
}

func sqlNeedsUintArrayHelpers(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		if field.StorageKind == "array_uint" {
			return true
		}
	}

	return false
}

func sqlNeedsFloatArrayHelpers(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		if field.StorageKind == "array_float" {
			return true
		}
	}

	return false
}

func sqlNeedsBoolArrayHelpers(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		if field.StorageKind == "array_bool" {
			return true
		}
	}

	return false
}

func sqlNeedsStrconv(fields []sqlFieldMeta) bool {
	return sqlNeedsIntArrayHelpers(fields) || sqlNeedsUintArrayHelpers(fields) || sqlNeedsFloatArrayHelpers(fields) || sqlNeedsBoolArrayHelpers(fields)
}

func sqlNeedsTouchNow(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		if (field.Default == "CURRENT_TIMESTAMP" || field.UpdatedAt) && field.Type.IsTime() {
			return true
		}
	}

	return false
}

func sqlScanVarDecl(field sqlFieldMeta) []string {
	name := sdk.LowerCamel(field.Field)
	switch field.StorageKind {
	case "array_string", "array_int", "array_uint", "array_float", "array_bool", "json":
		return []string{"var " + name + "Raw any"}
	}

	switch {
	case field.Type.IsString():
		if field.Nullable {
			return []string{"var " + name + "Value stdsql.NullString"}
		}
		return []string{"var " + name + "Value string"}
	case field.Type.IsBool():
		if field.Nullable {
			return []string{"var " + name + "Value stdsql.NullBool"}
		}
		return []string{"var " + name + "Value bool"}
	case field.Type.IsInteger():
		if field.Nullable {
			return []string{"var " + name + "Value stdsql.NullInt64"}
		}
		return []string{"var " + name + "Value int64"}
	case field.Type.IsFloat():
		if field.Nullable {
			return []string{"var " + name + "Value stdsql.NullFloat64"}
		}
		return []string{"var " + name + "Value float64"}
	case field.Type.IsTime():
		return []string{"var " + name + "Value stdsql.NullTime"}
	default:
		return []string{"var " + name + "Raw any"}
	}
}

func sqlScanVarRef(field sqlFieldMeta) string {
	name := sdk.LowerCamel(field.Field)
	switch field.StorageKind {
	case "array_string", "array_int", "array_uint", "array_float", "array_bool", "json":
		return "&" + name + "Raw"
	}

	switch {
	case field.Type.IsString(), field.Type.IsBool(), field.Type.IsInteger(), field.Type.IsFloat(), field.Type.IsTime():
		return "&" + name + "Value"
	default:
		return "&" + name + "Raw"
	}
}

func sqlAssignScanLines(prefix string, field sqlFieldMeta) []string {
	name := sdk.LowerCamel(field.Field)
	target := "model." + field.Field

	switch field.StorageKind {
	case "array_string":
		return []string{fmt.Sprintf("if value, err := %sDecodeStringArray(%sRaw); err != nil {\n\t\treturn err\n\t} else {\n\t\t%s = value\n\t}", prefix, name, target)}
	case "array_int":
		return []string{fmt.Sprintf("if value, err := %sDecodeIntArray(%sRaw); err != nil {\n\t\treturn err\n\t} else {\n\t\t%s = value\n\t}", prefix, name, target)}
	case "array_uint":
		return []string{fmt.Sprintf("if value, err := %sDecodeUintArray(%sRaw); err != nil {\n\t\treturn err\n\t} else {\n\t\titems := make(%s, 0, len(value))\n\t\tfor _, item := range value {\n\t\t\titems = append(items, %s(item))\n\t\t}\n\t\t%s = items\n\t}", prefix, name, field.Type.GoString(), field.Type.BaseName(), target)}
	case "array_float":
		return []string{fmt.Sprintf("if value, err := %sDecodeFloatArray(%sRaw); err != nil {\n\t\treturn err\n\t} else {\n\t\titems := make(%s, 0, len(value))\n\t\tfor _, item := range value {\n\t\t\titems = append(items, %s(item))\n\t\t}\n\t\t%s = items\n\t}", prefix, name, field.Type.GoString(), field.Type.BaseName(), target)}
	case "array_bool":
		return []string{fmt.Sprintf("if value, err := %sDecodeBoolArray(%sRaw); err != nil {\n\t\treturn err\n\t} else {\n\t\t%s = value\n\t}", prefix, name, target)}
	case "json":
		return sqlAssignJSONLines(prefix, field, name, target)
	}

	switch {
	case field.Type.IsString():
		if field.Nullable {
			return []string{fmt.Sprintf("if %sValue.Valid {\n\t\tvalue := %sValue.String\n\t\t%s = &value\n\t} else {\n\t\t%s = nil\n\t}", name, name, target, target)}
		}
		return []string{fmt.Sprintf("%s = %sValue", target, name)}
	case field.Type.IsBool():
		if field.Nullable {
			return []string{fmt.Sprintf("if %sValue.Valid {\n\t\tvalue := %sValue.Bool\n\t\t%s = &value\n\t} else {\n\t\t%s = nil\n\t}", name, name, target, target)}
		}
		return []string{fmt.Sprintf("%s = %sValue", target, name)}
	case field.Type.IsInteger():
		if field.Nullable {
			return []string{fmt.Sprintf("if %sValue.Valid {\n\t\tvalue := %s(%sValue.Int64)\n\t\t%s = &value\n\t} else {\n\t\t%s = nil\n\t}", name, field.Type.BaseName(), name, target, target)}
		}
		return []string{fmt.Sprintf("%s = %s(%sValue)", target, field.Type.BaseName(), name)}
	case field.Type.IsFloat():
		if field.Nullable {
			return []string{fmt.Sprintf("if %sValue.Valid {\n\t\tvalue := %s(%sValue.Float64)\n\t\t%s = &value\n\t} else {\n\t\t%s = nil\n\t}", name, field.Type.BaseName(), name, target, target)}
		}
		return []string{fmt.Sprintf("%s = %s(%sValue)", target, field.Type.BaseName(), name)}
	case field.Type.IsTime():
		if field.Nullable {
			return []string{fmt.Sprintf("if %sValue.Valid {\n\t\tvalue := %sValue.Time\n\t\t%s = &value\n\t} else {\n\t\t%s = nil\n\t}", name, name, target, target)}
		}
		return []string{fmt.Sprintf("if !%sValue.Valid {\n\t\treturn fmt.Errorf(\"required sql time column %%q was NULL\", %q)\n\t}\n\t%s = %sValue.Time", name, field.Column, target, name)}
	default:
		return nil
	}
}

func sqlAssignJSONLines(prefix string, field sqlFieldMeta, tempName string, target string) []string {
	if field.Type.Optional {
		return []string{fmt.Sprintf("if %sRaw == nil {\n\t\t%s = nil\n\t} else {\n\t\tvalue := new(%s)\n\t\tif err := %sDecodeJSON(%sRaw, value); err != nil {\n\t\t\treturn err\n\t\t}\n\t\t%s = value\n\t}", tempName, target, field.Type.BaseName(), prefix, tempName, target)}
	}

	return []string{fmt.Sprintf("if err := %sDecodeJSON(%sRaw, &%s); err != nil {\n\t\treturn err\n\t}", prefix, tempName, target)}
}

func sqlDriverValueLines(prefix string, field sqlFieldMeta) []string {
	fieldExpr := "model." + field.Field

	switch field.StorageKind {
	case "array_string":
		if field.Type.Optional {
			return []string{fmt.Sprintf("if %s == nil {\n\t\tvalues = append(values, nil)\n\t} else {\n\t\tvalues = append(values, %sEncodeStringArray(%s))\n\t}", fieldExpr, prefix, fieldExpr)}
		}
		return []string{fmt.Sprintf("values = append(values, %sEncodeStringArray(%s))", prefix, fieldExpr)}
	case "array_int":
		if field.Type.Optional {
			return []string{fmt.Sprintf("if %s == nil {\n\t\tvalues = append(values, nil)\n\t} else {\n\t\tvalues = append(values, %sEncodeIntArray(%s))\n\t}", fieldExpr, prefix, fieldExpr)}
		}
		return []string{fmt.Sprintf("values = append(values, %sEncodeIntArray(%s))", prefix, fieldExpr)}
	case "array_uint":
		lines := []string{fmt.Sprintf("items%s := make([]uint64, 0, len(%s))", field.Field, fieldExpr), fmt.Sprintf("for _, item := range %s {\n\t\titems%s = append(items%s, uint64(item))\n\t}", fieldExpr, field.Field, field.Field)}
		if field.Type.Optional {
			lines = append([]string{fmt.Sprintf("if %s == nil {\n\t\tvalues = append(values, nil)\n\t} else {", fieldExpr)}, lines...)
			lines = append(lines, fmt.Sprintf("\t\tvalues = append(values, %sEncodeUintArray(items%s))\n\t}", prefix, field.Field))
			return lines
		}
		lines = append(lines, fmt.Sprintf("values = append(values, %sEncodeUintArray(items%s))", prefix, field.Field))
		return lines
	case "array_float":
		lines := []string{fmt.Sprintf("items%s := make([]float64, 0, len(%s))", field.Field, fieldExpr), fmt.Sprintf("for _, item := range %s {\n\t\titems%s = append(items%s, float64(item))\n\t}", fieldExpr, field.Field, field.Field)}
		if field.Type.Optional {
			lines = append([]string{fmt.Sprintf("if %s == nil {\n\t\tvalues = append(values, nil)\n\t} else {", fieldExpr)}, lines...)
			lines = append(lines, fmt.Sprintf("\t\tvalues = append(values, %sEncodeFloatArray(items%s))\n\t}", prefix, field.Field))
			return lines
		}
		lines = append(lines, fmt.Sprintf("values = append(values, %sEncodeFloatArray(items%s))", prefix, field.Field))
		return lines
	case "array_bool":
		if field.Type.Optional {
			return []string{fmt.Sprintf("if %s == nil {\n\t\tvalues = append(values, nil)\n\t} else {\n\t\tvalues = append(values, %sEncodeBoolArray(%s))\n\t}", fieldExpr, prefix, fieldExpr)}
		}
		return []string{fmt.Sprintf("values = append(values, %sEncodeBoolArray(%s))", prefix, fieldExpr)}
	case "json":
		lines := []string{}
		if field.Type.Optional {
			lines = append(lines, fmt.Sprintf("if %s == nil {\n\t\tvalues = append(values, nil)\n\t} else {", fieldExpr))
			lines = append(lines, fmt.Sprintf("\t\tpayload, err := %sMarshalJSON(%s)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tvalues = append(values, payload)\n\t}", prefix, fieldExpr))
			return lines
		}
		lines = append(lines, fmt.Sprintf("payload%s, err := %sMarshalJSON(%s)", field.Field, prefix, fieldExpr))
		lines = append(lines, "if err != nil {\n\t\treturn nil, err\n\t}")
		lines = append(lines, fmt.Sprintf("values = append(values, payload%s)", field.Field))
		return lines
	}

	if field.Type.IsTime() && (field.Default == "CURRENT_TIMESTAMP" || field.UpdatedAt) {
		if field.Type.Optional {
			return []string{fmt.Sprintf("if %s == nil {\n\t\tvalue := now\n\t\tvalues = append(values, value)\n\t} else {\n\t\tvalues = append(values, *%s)\n\t}", fieldExpr, fieldExpr)}
		}
		return []string{fmt.Sprintf("if %s.IsZero() {\n\t\tvalues = append(values, now)\n\t} else {\n\t\tvalues = append(values, %s)\n\t}", fieldExpr, fieldExpr)}
	}

	if field.Type.Optional && !field.Type.IsList {
		return []string{fmt.Sprintf("if %s != nil {\n\t\tvalues = append(values, *%s)\n\t} else {\n\t\tvalues = append(values, nil)\n\t}", fieldExpr, fieldExpr)}
	}

	return []string{fmt.Sprintf("values = append(values, %s)", fieldExpr)}
}
