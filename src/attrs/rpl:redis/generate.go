package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strconv"
	"strings"
)

func generateRedis(req sdk.GenerateRequest) sdk.GenerateResponse {
	builder := sdk.NewCodeBuilder()

	modelValues := req.Model.ResolvedValues("redis")
	tableName := strings.TrimSpace(modelValues["table"].String())
	if tableName == "" {
		tableName = sdk.SnakeCase(req.Model.Name)
	}

	ttlLiteral := "0"
	if ttl := strings.TrimSpace(modelValues["ttl"].String()); ttl != "" {
		if _, err := strconv.Atoi(ttl); err == nil {
			ttlLiteral = ttl
		}
	}

	fields := req.Model.ActiveFields("redis")
	if len(fields) == 0 {
		return builder.Response()
	}

	keyFields := redisKeyFields(fields)

	hashResponse := generateRedisHashMethod(req.Model.Name, fields)
	builder.AddResponse(hashResponse)

	applyResponse := generateRedisApplyMethod(req.Model.Name, fields)
	builder.AddResponse(applyResponse)

	keyResponse := generateRedisKeyMethod(req.Model.Name, tableName, keyFields, ttlLiteral)
	builder.AddResponse(keyResponse)

	return builder.Response()
}

func redisKeyFields(fields []sdk.Field) []sdk.Field {
	keys := make([]sdk.Field, 0)
	for _, field := range fields {
		if field.ResolvedValues("redis")["unique"].BoolValue() {
			keys = append(keys, field)
		}
	}
	if len(keys) > 0 {
		return keys
	}

	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "ID" || strings.HasSuffix(name, "ID") {
			keys = append(keys, field)
		}
	}
	if len(keys) > 0 {
		return keys
	}

	return []sdk.Field{fields[0]}
}

func generateRedisKeyMethod(modelName string, tableName string, fields []sdk.Field, ttlLiteral string) sdk.GenerateResponse {
	builder := sdk.NewCodeBuilder()
	builder.AddImport("fmt")
	builder.AddImport("strings")
	for _, field := range fields {
		if field.Type.IsTime() && !field.Type.Optional && !field.Type.IsList {
			builder.AddImport("time")
			break
		}
	}

	lines := make([]string, 0, len(fields)+2)
	lines = append(lines, fmt.Sprintf("parts := []string{%q}", tableName))
	for _, field := range fields {
		lines = append(lines, redisKeyPartCode(field))
	}
	lines = append(lines, `return strings.Join(parts, ":")`)

	if strings.TrimSpace(ttlLiteral) != "" && ttlLiteral != "0" {
		builder.AddOrderedBlock("redis.ttl", fmt.Sprintf("const %sRedisTTLSeconds = %s", sdk.LowerCamel(modelName), ttlLiteral), 0)
	}
	builder.AddOrderedBlock("redis.key", sdk.WithDocComment(
		fmt.Sprintf("func (model %s) RedisKey() string {\n\t%s\n}", modelName, strings.Join(lines, "\n\t")),
		"RedisKey строит Redis-ключ для модели %s.",
		"RedisKey builds the Redis key for model %s.",
		modelName,
	), 10)
	return builder.Response()
}

func generateRedisHashMethod(modelName string, fields []sdk.Field) sdk.GenerateResponse {
	builder := sdk.NewCodeBuilder()
	lines := make([]string, 0, len(fields)+2)
	lines = append(lines, "values := map[string]string{}")

	needsJSON := false
	needsFmt := false
	needsStrconv := false
	needsTime := false

	for _, field := range fields {
		code, imports := redisHashFieldCode(field)
		lines = append(lines, code)
		needsJSON = needsJSON || imports["encoding/json"]
		needsFmt = needsFmt || imports["fmt"]
		needsStrconv = needsStrconv || imports["strconv"]
		needsTime = needsTime || imports["time"]
	}
	lines = append(lines, "return values, nil")

	if needsJSON {
		builder.AddImport("encoding/json")
	}
	if needsFmt {
		builder.AddImport("fmt")
	}
	if needsStrconv {
		builder.AddImport("strconv")
	}
	if needsTime {
		builder.AddImport("time")
	}

	builder.AddOrderedBlock("redis.hash", sdk.WithDocComment(
		fmt.Sprintf("func (model %s) RedisHash() (map[string]string, error) {\n\t%s\n}", modelName, strings.Join(lines, "\n\t")),
		"RedisHash сериализует модель %s в строковый hash для Redis.",
		"RedisHash serializes model %s into a string Redis hash.",
		modelName,
	), 40)
	return builder.Response()
}

func generateRedisApplyMethod(modelName string, fields []sdk.Field) sdk.GenerateResponse {
	builder := sdk.NewCodeBuilder()
	lines := make([]string, 0, len(fields)+1)

	needsJSON := false
	needsFmt := false
	needsStrconv := false
	needsTime := false

	for _, field := range fields {
		defaultCode, defaultImports := redisDefaultCode(field)
		if defaultCode != "" {
			lines = append(lines, defaultCode)
		}
		needsJSON = needsJSON || defaultImports["encoding/json"]
		needsFmt = needsFmt || defaultImports["fmt"]
		needsStrconv = needsStrconv || defaultImports["strconv"]
		needsTime = needsTime || defaultImports["time"]
		code, imports := redisApplyFieldCode(field)
		lines = append(lines, code)
		needsJSON = needsJSON || imports["encoding/json"]
		needsFmt = needsFmt || imports["fmt"]
		needsStrconv = needsStrconv || imports["strconv"]
		needsTime = needsTime || imports["time"]
	}
	lines = append(lines, "return nil")

	if needsJSON {
		builder.AddImport("encoding/json")
	}
	if needsFmt {
		builder.AddImport("fmt")
	}
	if needsStrconv {
		builder.AddImport("strconv")
	}
	if needsTime {
		builder.AddImport("time")
	}

	builder.AddOrderedBlock("redis.apply", sdk.WithDocComment(
		fmt.Sprintf("func (model *%s) ApplyRedisHash(values map[string]string) error {\n\t%s\n}", modelName, strings.Join(lines, "\n\t")),
		"ApplyRedisHash заполняет модель %s данными из Redis hash.",
		"ApplyRedisHash fills model %s from a Redis hash payload.",
		modelName,
	), 50)
	return builder.Response()
}

func redisKeyPartCode(field sdk.Field) string {
	fieldExpr := "model." + field.Name
	if field.Type.Optional && !field.Type.IsList {
		return fmt.Sprintf("if %s != nil {\n\t\tparts = append(parts, fmt.Sprint(*%s))\n\t} else {\n\t\tparts = append(parts, \"\")\n\t}", fieldExpr, fieldExpr)
	}

	if field.Type.IsTime() && !field.Type.IsList {
		return fmt.Sprintf("parts = append(parts, %s.Format(time.RFC3339Nano))", fieldExpr)
	}

	return fmt.Sprintf("parts = append(parts, fmt.Sprint(%s))", fieldExpr)
}

func redisHashFieldCode(field sdk.Field) (string, map[string]bool) {
	imports := map[string]bool{}
	column := redisHashName(field)
	fieldExpr := "model." + field.Name

	switch {
	case field.Type.IsList || (!field.Type.IsScalar() && !field.Type.Optional):
		imports["encoding/json"] = true
		return fmt.Sprintf("if encoded, err := json.Marshal(%s); err != nil {\n\t\treturn nil, err\n\t} else {\n\t\tvalues[%q] = string(encoded)\n\t}", fieldExpr, column), imports
	case field.Type.Optional && !field.Type.IsList:
		return redisOptionalHashCode(field, column), imports
	case field.Type.IsString():
		return fmt.Sprintf("values[%q] = %s", column, fieldExpr), imports
	case field.Type.IsBool():
		imports["strconv"] = true
		return fmt.Sprintf("values[%q] = strconv.FormatBool(%s)", column, fieldExpr), imports
	case field.Type.IsInteger():
		imports["strconv"] = true
		return fmt.Sprintf("values[%q] = strconv.FormatInt(int64(%s), 10)", column, fieldExpr), imports
	case field.Type.IsFloat():
		imports["strconv"] = true
		return fmt.Sprintf("values[%q] = strconv.FormatFloat(float64(%s), 'f', -1, 64)", column, fieldExpr), imports
	case field.Type.IsTime():
		imports["time"] = true
		return fmt.Sprintf("values[%q] = %s.Format(time.RFC3339Nano)", column, fieldExpr), imports
	default:
		imports["encoding/json"] = true
		return fmt.Sprintf("if encoded, err := json.Marshal(%s); err != nil {\n\t\treturn nil, err\n\t} else {\n\t\tvalues[%q] = string(encoded)\n\t}", fieldExpr, column), imports
	}
}

func redisOptionalHashCode(field sdk.Field, column string) string {
	fieldExpr := "model." + field.Name

	switch {
	case field.Type.IsString():
		return fmt.Sprintf("if %s != nil {\n\t\tvalues[%q] = *%s\n\t}", fieldExpr, column, fieldExpr)
	case field.Type.IsBool():
		return fmt.Sprintf("if %s != nil {\n\t\tvalues[%q] = strconv.FormatBool(*%s)\n\t}", fieldExpr, column, fieldExpr)
	case field.Type.IsInteger():
		return fmt.Sprintf("if %s != nil {\n\t\tvalues[%q] = strconv.FormatInt(int64(*%s), 10)\n\t}", fieldExpr, column, fieldExpr)
	case field.Type.IsFloat():
		return fmt.Sprintf("if %s != nil {\n\t\tvalues[%q] = strconv.FormatFloat(float64(*%s), 'f', -1, 64)\n\t}", fieldExpr, column, fieldExpr)
	case field.Type.IsTime():
		return fmt.Sprintf("if %s != nil {\n\t\tvalues[%q] = %s.Format(time.RFC3339Nano)\n\t}", fieldExpr, column, fieldExpr)
	default:
		return fmt.Sprintf("if %s != nil {\n\t\tif encoded, err := json.Marshal(*%s); err != nil {\n\t\t\treturn nil, err\n\t\t} else {\n\t\t\tvalues[%q] = string(encoded)\n\t\t}\n\t}", fieldExpr, fieldExpr, column)
	}
}

func redisApplyFieldCode(field sdk.Field) (string, map[string]bool) {
	imports := map[string]bool{}
	column := redisHashName(field)

	switch {
	case field.Type.IsList || (!field.Type.IsScalar() && !field.Type.Optional):
		imports["encoding/json"] = true
		imports["fmt"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tif err := json.Unmarshal([]byte(raw), &model.%s); err != nil {\n\t\t\treturn fmt.Errorf(\"decode redis field %s: %%w\", err)\n\t\t}\n\t}", column, field.Name, column), imports
	case field.Type.Optional && !field.Type.IsList:
		code, extra := redisOptionalApplyCode(field, column)
		for key := range extra {
			imports[key] = true
		}
		return code, imports
	case field.Type.IsString():
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tmodel.%s = raw\n\t}", column, field.Name), imports
	case field.Type.IsBool():
		imports["fmt"] = true
		imports["strconv"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := strconv.ParseBool(raw)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tmodel.%s = parsed\n\t}", column, column, field.Name), imports
	case field.Type.IsInteger():
		imports["fmt"] = true
		imports["strconv"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := strconv.ParseInt(raw, 10, 64)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tmodel.%s = %s(parsed)\n\t}", column, column, field.Name, field.Type.BaseName()), imports
	case field.Type.IsFloat():
		imports["fmt"] = true
		imports["strconv"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := strconv.ParseFloat(raw, 64)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tmodel.%s = %s(parsed)\n\t}", column, column, field.Name, field.Type.BaseName()), imports
	case field.Type.IsTime():
		imports["fmt"] = true
		imports["time"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := time.Parse(time.RFC3339Nano, raw)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tmodel.%s = parsed\n\t}", column, column, field.Name), imports
	default:
		imports["encoding/json"] = true
		imports["fmt"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tif err := json.Unmarshal([]byte(raw), &model.%s); err != nil {\n\t\t\treturn fmt.Errorf(\"decode redis field %s: %%w\", err)\n\t\t}\n\t}", column, field.Name, column), imports
	}
}

func redisDefaultCode(field sdk.Field) (string, map[string]bool) {
	imports := map[string]bool{}
	resolved, ok := field.ResolvedAttr("redis")
	if !ok {
		return "", imports
	}
	value, ok := resolved.Value("default")
	if !ok {
		return "", imports
	}
	raw := strings.TrimSpace(value.String())
	if raw == "" {
		return "", imports
	}

	column := redisHashName(field)
	target := "model." + field.Name
	assignment := ""
	switch {
	case field.Type.IsList || (!field.Type.IsScalar() && !field.Type.Optional):
		imports["encoding/json"] = true
		imports["fmt"] = true
		assignment = fmt.Sprintf("if err := json.Unmarshal([]byte(%q), &%s); err != nil { return fmt.Errorf(\"decode default for redis field %s: %%w\", err) }", raw, target, column)
	case field.Type.IsString():
		assignment = fmt.Sprintf("%s = %q", target, raw)
	case field.Type.IsBool():
		parsed, _ := strconv.ParseBool(raw)
		assignment = fmt.Sprintf("%s = %t", target, parsed)
	case field.Type.IsInteger():
		assignment = fmt.Sprintf("%s = %s(%s)", target, field.Type.BaseName(), raw)
	case field.Type.IsFloat():
		assignment = fmt.Sprintf("%s = %s(%s)", target, field.Type.BaseName(), raw)
	case field.Type.IsTime() && strings.EqualFold(raw, "now"):
		imports["time"] = true
		assignment = fmt.Sprintf("%s = time.Now()", target)
	case field.Type.IsTime():
		imports["fmt"] = true
		imports["time"] = true
		assignment = fmt.Sprintf("parsed, err := time.Parse(time.RFC3339, %q)\n\t\tif err != nil { return fmt.Errorf(\"parse default for redis field %s: %%w\", err) }\n\t\t%s = parsed", raw, column, target)
	default:
		imports["encoding/json"] = true
		imports["fmt"] = true
		assignment = fmt.Sprintf("if err := json.Unmarshal([]byte(%q), &%s); err != nil { return fmt.Errorf(\"decode default for redis field %s: %%w\", err) }", raw, target, column)
	}

	if field.Type.Optional && !field.Type.IsList {
		base := field.Type
		base.Optional = false
		switch {
		case field.Type.IsString():
			assignment = fmt.Sprintf("value := %q\n\t\t%s = &value", raw, target)
		case field.Type.IsBool():
			parsed, _ := strconv.ParseBool(raw)
			assignment = fmt.Sprintf("value := %t\n\t\t%s = &value", parsed, target)
		case field.Type.IsInteger(), field.Type.IsFloat():
			assignment = fmt.Sprintf("value := %s(%s)\n\t\t%s = &value", base.BaseName(), raw, target)
		case field.Type.IsTime() && strings.EqualFold(raw, "now"):
			imports["time"] = true
			assignment = fmt.Sprintf("value := time.Now()\n\t\t%s = &value", target)
		case field.Type.IsTime():
			imports["fmt"] = true
			imports["time"] = true
			assignment = fmt.Sprintf("value, err := time.Parse(time.RFC3339, %q)\n\t\tif err != nil { return fmt.Errorf(\"parse default for redis field %s: %%w\", err) }\n\t\t%s = &value", raw, column, target)
		default:
			imports["encoding/json"] = true
			imports["fmt"] = true
			assignment = fmt.Sprintf("var value %s\n\t\tif err := json.Unmarshal([]byte(%q), &value); err != nil { return fmt.Errorf(\"decode default for redis field %s: %%w\", err) }\n\t\t%s = &value", base.GoString(), raw, column, target)
		}
	}

	return fmt.Sprintf("if _, ok := values[%q]; !ok {\n\t\t%s\n\t}", column, assignment), imports
}

func redisOptionalApplyCode(field sdk.Field, column string) (string, map[string]bool) {
	imports := map[string]bool{}

	switch {
	case field.Type.IsString():
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tvalue := raw\n\t\tmodel.%s = &value\n\t}", column, field.Name), imports
	case field.Type.IsBool():
		imports["fmt"] = true
		imports["strconv"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := strconv.ParseBool(raw)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tvalue := parsed\n\t\tmodel.%s = &value\n\t}", column, column, field.Name), imports
	case field.Type.IsInteger():
		imports["fmt"] = true
		imports["strconv"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := strconv.ParseInt(raw, 10, 64)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tvalue := %s(parsed)\n\t\tmodel.%s = &value\n\t}", column, column, field.Type.BaseName(), field.Name), imports
	case field.Type.IsFloat():
		imports["fmt"] = true
		imports["strconv"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := strconv.ParseFloat(raw, 64)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tvalue := %s(parsed)\n\t\tmodel.%s = &value\n\t}", column, column, field.Type.BaseName(), field.Name), imports
	case field.Type.IsTime():
		imports["fmt"] = true
		imports["time"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tparsed, err := time.Parse(time.RFC3339Nano, raw)\n\t\tif err != nil {\n\t\t\treturn fmt.Errorf(\"parse redis field %s: %%w\", err)\n\t\t}\n\t\tvalue := parsed\n\t\tmodel.%s = &value\n\t}", column, column, field.Name), imports
	default:
		imports["encoding/json"] = true
		imports["fmt"] = true
		return fmt.Sprintf("if raw, ok := values[%q]; ok {\n\t\tvar value %s\n\t\tif err := json.Unmarshal([]byte(raw), &value); err != nil {\n\t\t\treturn fmt.Errorf(\"decode redis field %s: %%w\", err)\n\t\t}\n\t\tmodel.%s = &value\n\t}", column, field.Type.BaseName(), column, field.Name), imports
	}
}
